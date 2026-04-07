package logger

import (
	"bufio"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"
)

// errorKey is the context key for retrieving handler errors from request context
const errorKey = "handler_error"

// Option is logger option.
type Option func(*options)

// options defines the configuration for logger middleware
type options struct {
	// Logger is the slog logger instance
	logger *slog.Logger

	// SkipPaths is a list of paths to skip logging
	skipPaths   []string
	skipPathSet map[string]struct{}

	// CustomFields is a function to add custom fields to log
	customFields func(*http.Request, int, time.Duration) []any
}

// WithLogger sets the logger instance
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

// WithSkipPaths sets paths to skip logging
func WithSkipPaths(paths []string) Option {
	return func(o *options) {
		o.skipPaths = append([]string(nil), paths...)
	}
}

// WithCustomFields sets a function to add custom fields
func WithCustomFields(f func(*http.Request, int, time.Duration) []any) Option {
	return func(o *options) {
		o.customFields = f
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	request    *http.Request
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	return n, err
}

func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (rw *responseWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := rw.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (rw *responseWriter) ReadFrom(r io.Reader) (int64, error) {
	readerFrom, ok := rw.ResponseWriter.(io.ReaderFrom)
	if ok {
		return readerFrom.ReadFrom(r)
	}
	return io.Copy(rw.ResponseWriter, r)
}

func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// getHandlerError retrieves the error from request context if present
func (rw *responseWriter) getHandlerError() error {
	if rw.request != nil {
		if err := rw.request.Context().Value(errorKey); err != nil {
			if e, ok := err.(error); ok {
				return e
			}
		}
	}
	return nil
}

// New returns a middleware that logs HTTP requests using slog
func New(opts ...Option) func(http.Handler) http.Handler {
	o := &options{}

	for _, opt := range opts {
		opt(o)
	}

	// Use default logger if not provided
	if o.logger == nil {
		o.logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	if len(o.skipPaths) > 0 {
		o.skipPathSet = make(map[string]struct{}, len(o.skipPaths))
		for _, path := range o.skipPaths {
			o.skipPathSet[path] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path should be skipped
			if _, ok := o.skipPathSet[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()

			// Wrap response writer to capture status code
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				request:        r,
			}
			// Process request
			next.ServeHTTP(rw, r)

			// Log request details
			duration := time.Since(start)

			// Build log fields
			fields := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.statusCode,
				"duration", duration.String(),
				"ip", r.RemoteAddr,
			}

			// Add error field if present
			if handlerErr := rw.getHandlerError(); handlerErr != nil {
				fields = append(fields, "error", handlerErr.Error())
			}

			// Add custom fields if provided
			if o.customFields != nil {
				customFields := o.customFields(r, rw.statusCode, duration)
				fields = append(fields, customFields...)
			}

			o.logger.Info("request", fields...)
		})
	}
}
