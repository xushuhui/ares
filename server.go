package ares

import (
	"context"
	"log/slog"
	"net/http"
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

// Start starts the HTTP server
// The context parameter is currently not used as ListenAndServe blocks until shutdown.
// Shutdown is controlled via the Stop method with its own context.
func (s *httpServer) Start(ctx context.Context) error {
	s.srv = &http.Server{
		Addr:         s.addr,
		Handler:      s.handler,
		ReadTimeout:  s.options.ReadTimeout,
		WriteTimeout: s.options.WriteTimeout,
		IdleTimeout:  s.options.IdleTimeout,
	}

	s.logger.Info("starting http server", "addr", s.addr)

	// Start server (this blocks until server is shut down)
	// Note: ListenAndServe does not accept a context, it runs until Shutdown is called
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.logger.Error("http server error", "error", err)
		return err
	}

	return nil
}

// Stop gracefully stops the HTTP server
func (s *httpServer) Stop(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}

	s.logger.Info("stopping http server...")

	if err := s.srv.Shutdown(ctx); err != nil {
		s.logger.Error("http server forced to shutdown", "error", err)
		return err
	}

	s.logger.Info("http server stopped")
	return nil
}
