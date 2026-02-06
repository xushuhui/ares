package logger

import (
	"log/slog"
	"net/http"
	"os"
	"slices"
	"time"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// errorKey is the context key for retrieving handler errors from request context
const errorKey contextKey = "handler_error"

// Option is logger option.
type Option func(*options)

// options defines the configuration for logger middleware
type options struct {
	// Logger is the slog logger instance
	logger *slog.Logger

	// SkipPaths is a list of paths to skip logging
	skipPaths []string

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
		o.skipPaths = paths
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

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path should be skipped
			if slices.Contains(o.skipPaths, r.URL.Path) {
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
