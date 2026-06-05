package ares

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	aresErrors "github.com/xushuhui/ares/errors"
	"github.com/xushuhui/ares/internal/contextkeys"
	"github.com/xushuhui/ares/middleware/logger"
	"github.com/xushuhui/ares/middleware/recovery"
)

// Handler defines the handler function signature
type Handler func(*Context) error

// AresOption defines a function that configures Ares instance
type AresOption func(*Ares)

// ServerOptions defines HTTP server configuration
type ServerOptions struct {
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// Option defines a function that configures ServerOptions
type Option func(*ServerOptions)

// WithReadTimeout sets the read timeout
func WithReadTimeout(d time.Duration) Option {
	return func(o *ServerOptions) {
		o.ReadTimeout = d
	}
}

// WithWriteTimeout sets the write timeout
func WithWriteTimeout(d time.Duration) Option {
	return func(o *ServerOptions) {
		o.WriteTimeout = d
	}
}

// WithIdleTimeout sets the idle timeout
func WithIdleTimeout(d time.Duration) Option {
	return func(o *ServerOptions) {
		o.IdleTimeout = d
	}
}

// WithShutdownTimeout sets the graceful shutdown timeout
func WithShutdownTimeout(d time.Duration) Option {
	return func(o *ServerOptions) {
		o.ShutdownTimeout = d
	}
}

// defaultServerOptions returns default server configuration
func defaultServerOptions() ServerOptions {
	return ServerOptions{
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}
}

// Ares is the main framework instance
type Ares struct {
	*chi.Mux
	logger *slog.Logger
}

// Group represents a route group with common prefix and middleware
type Group struct {
	ares        *Ares
	prefix      string
	middlewares []func(http.Handler) http.Handler
}

// WithLogger sets a custom logger for Ares
func WithLogger(logger *slog.Logger) AresOption {
	return func(a *Ares) {
		a.logger = logger
	}
}

// New creates a new Ares instance with optional configuration
func New(opts ...AresOption) *Ares {
	a := &Ares{
		Mux:    chi.NewRouter(),
		logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}

	// Apply options
	for _, opt := range opts {
		opt(a)
	}

	return a
}

// Default creates a new Ares instance with default middleware (logger and recovery)
func Default() *Ares {
	a := New()
	a.Use(logger.New(), recovery.New())
	return a
}

// Logger returns the logger instance
func (a *Ares) Logger() *slog.Logger {
	return a.logger
}

// wrapHandler converts HandlerFunc to http.HandlerFunc
func (a *Ares) wrapHandler(h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := NewContext(w, r, a.logger)
		defer ctx.release() // Return to pool after use

		if err := h(ctx); err != nil {
			// Store error in request context for middleware access (e.g. logger middleware).
			// Mutation of *r is intentional: the logger middleware stores the same pointer
			// before calling next, so mutating the struct lets it see the updated context
			// without requiring ares.Context access.
			*r = *r.WithContext(context.WithValue(r.Context(), contextkeys.HandlerError, err))

			ctx.SetError(err)

			a.logger.Error("handler error",
				"error", err,
				"path", r.URL.Path,
				"method", r.Method,
			)

			if !ctx.written {
				code := http.StatusInternalServerError
				msg := err.Error()
				var httpErr *aresErrors.Error
				if errors.As(err, &httpErr) {
					code = httpErr.Code
					msg = httpErr.Message
				}
				errorResponse := make(map[string]string, 1)
				errorResponse["error"] = msg
				_ = ctx.JSON(code, errorResponse)
			}
		}
	}
}

// GET registers a GET route
func (a *Ares) GET(pattern string, h Handler) {
	a.Get(pattern, a.wrapHandler(h))
}

// POST registers a POST route
func (a *Ares) POST(pattern string, h Handler) {
	a.Post(pattern, a.wrapHandler(h))
}

// PUT registers a PUT route
func (a *Ares) PUT(pattern string, h Handler) {
	a.Put(pattern, a.wrapHandler(h))
}

// DELETE registers a DELETE route
func (a *Ares) DELETE(pattern string, h Handler) {
	a.Delete(pattern, a.wrapHandler(h))
}

// PATCH registers a PATCH route
func (a *Ares) PATCH(pattern string, h Handler) {
	a.Patch(pattern, a.wrapHandler(h))
}

// Static serves files from the given file system root
// Example: app.Static("/static", "./public")
func (a *Ares) Static(pattern string, root string) {
	fs := http.FileServer(http.Dir(root))
	a.Get(pattern+"/*", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = r.URL.Path[len(pattern):]
		fs.ServeHTTP(w, r)
	})
}

// StaticFile serves a single file
// Example: app.StaticFile("/favicon.ico", "./favicon.ico")
func (a *Ares) StaticFile(pattern string, filepath string) {
	a.Get(pattern, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath)
	})
}

// Group creates a new router group with the given path prefix
// Example: api := app.Group("/api/v1")
func (a *Ares) Group(pattern string, middlewares ...func(http.Handler) http.Handler) *Group {
	groupMiddlewares := append([]func(http.Handler) http.Handler(nil), middlewares...)
	return &Group{
		ares:        a,
		prefix:      pattern,
		middlewares: groupMiddlewares,
	}
}

// Run starts the HTTP server with graceful shutdown
// Accepts optional configuration functions
func (a *Ares) Run(addr string, opts ...Option) error {
	// Get options for shutdown timeout
	opt := defaultServerOptions()
	for _, fn := range opts {
		fn(&opt)
	}

	// Create HTTP server
	server := NewHTTPServer(addr, a, a.logger, opts...)

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := server.Start(context.Background()); err != nil {
			errChan <- err
		}
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case <-quit:
		a.logger.Info("shutting down server...")
	case err := <-errChan:
		return err
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), opt.ShutdownTimeout)
	defer cancel()

	return server.Stop(ctx)
}

// Use adds middleware to the group
func (g *Group) Use(middlewares ...func(http.Handler) http.Handler) {
	g.middlewares = append(g.middlewares, middlewares...)
}

// GET registers a GET route in the group
func (g *Group) GET(pattern string, h Handler) {
	g.handle("GET", pattern, h)
}

// POST registers a POST route in the group
func (g *Group) POST(pattern string, h Handler) {
	g.handle("POST", pattern, h)
}

// PUT registers a PUT route in the group
func (g *Group) PUT(pattern string, h Handler) {
	g.handle("PUT", pattern, h)
}

// DELETE registers a DELETE route in the group
func (g *Group) DELETE(pattern string, h Handler) {
	g.handle("DELETE", pattern, h)
}

// PATCH registers a PATCH route in the group
func (g *Group) PATCH(pattern string, h Handler) {
	g.handle("PATCH", pattern, h)
}

// Group creates a sub-group with additional prefix
func (g *Group) Group(pattern string, middlewares ...func(http.Handler) http.Handler) *Group {
	combinedMiddlewares := append([]func(http.Handler) http.Handler(nil), g.middlewares...)
	combinedMiddlewares = append(combinedMiddlewares, middlewares...)
	return &Group{
		ares:        g.ares,
		prefix:      g.prefix + pattern,
		middlewares: combinedMiddlewares,
	}
}

// handle registers a route with the given method
func (g *Group) handle(method, pattern string, h Handler) {
	fullPath := g.prefix + pattern
	wrappedHandler := g.ares.wrapHandler(h)

	// Apply group middlewares in order
	var handler http.Handler = wrappedHandler
	for i := len(g.middlewares) - 1; i >= 0; i-- {
		handler = g.middlewares[i](handler)
	}

	g.ares.Method(method, fullPath, handler)
}
