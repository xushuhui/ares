package ares

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
)

// Server defines the interface for server implementations
type Server interface {
	Start(context.Context) error
	Stop(context.Context) error
}

// httpServer implements Server interface for HTTP server
type httpServer struct {
	addr    string
	handler http.Handler
	logger  *slog.Logger
	options ServerOptions
	mu      sync.RWMutex
	srv     *http.Server
}

// NewHTTPServer creates a new HTTP server instance
func NewHTTPServer(addr string, handler http.Handler, logger *slog.Logger, opts ...Option) Server {
	// Start with default options
	options := defaultServerOptions()

	// Apply all option functions
	for _, fn := range opts {
		fn(&options)
	}

	return &httpServer{
		addr:    addr,
		handler: handler,
		logger:  logger,
		options: options,
	}
}

// Start starts the HTTP server and blocks until it is shut down.
// The ctx parameter is accepted to satisfy the Server interface but is not used to
// trigger shutdown; call Stop(ctx) explicitly or use Run() for signal-based shutdown.
func (s *httpServer) Start(ctx context.Context) error {
	srv := &http.Server{
		Addr:         s.addr,
		Handler:      s.handler,
		ReadTimeout:  s.options.ReadTimeout,
		WriteTimeout: s.options.WriteTimeout,
		IdleTimeout:  s.options.IdleTimeout,
	}
	s.mu.Lock()
	s.srv = srv
	s.mu.Unlock()

	s.logger.Info("starting http server", "addr", s.addr)

	// Start server (this blocks until server is shut down)
	// Note: ListenAndServe does not accept a context, it runs until Shutdown is called
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.logger.Error("http server error", "error", err)
		return err
	}

	return nil
}

// Stop gracefully stops the HTTP server
func (s *httpServer) Stop(ctx context.Context) error {
	s.mu.RLock()
	srv := s.srv
	s.mu.RUnlock()
	if srv == nil {
		return nil
	}

	s.logger.Info("stopping http server...")

	if err := srv.Shutdown(ctx); err != nil {
		s.logger.Error("http server forced to shutdown", "error", err)
		return err
	}

	s.logger.Info("http server stopped")
	return nil
}
