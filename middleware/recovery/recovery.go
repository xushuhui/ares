package recovery

import (
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
)

// Option is recovery option.
type Option func(*options)

// options defines the configuration for recovery middleware
type options struct {
	// Logger is the slog logger instance
	logger *slog.Logger

	// EnableStackTrace controls whether to log stack trace
	enableStackTrace bool

	// RecoveryHandler is a custom handler for recovered panics
	recoveryHandler func(http.ResponseWriter, *http.Request, any)
}

// WithLogger sets the logger instance
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

// WithStackTrace enables or disables stack trace logging
func WithStackTrace(enable bool) Option {
	return func(o *options) {
		o.enableStackTrace = enable
	}
}

// WithRecoveryHandler sets a custom recovery handler
func WithRecoveryHandler(handler func(http.ResponseWriter, *http.Request, any)) Option {
	return func(o *options) {
		o.recoveryHandler = handler
	}
}

// New returns a middleware that recovers from panics
func New(opts ...Option) func(http.Handler) http.Handler {
	o := &options{
		enableStackTrace: true, // Enable by default
	}

	for _, opt := range opts {
		opt(o)
	}

	// Use default logger if not provided
	if o.logger == nil {
		o.logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Build log fields
					fields := []any{
						"error", err,
						"path", r.URL.Path,
						"method", r.Method,
					}

					// Add stack trace if enabled
					if o.enableStackTrace {
						fields = append(fields, "stack", string(debug.Stack()))
					}

					// Log the panic
					o.logger.Error("panic recovered", fields...)

					// Use custom recovery handler if provided
					if o.recoveryHandler != nil {
						o.recoveryHandler(w, r, err)
						return
					}

					// Default recovery response
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error":"Internal Server Error"}`))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
