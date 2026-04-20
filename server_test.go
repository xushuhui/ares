package ares

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"
)

func TestNewHTTPServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := make([]struct {
		name     string
		addr     string
		handler  http.Handler
		logger   *slog.Logger
		options  []Option
		wantType string
	}, 3)
	tests[0] = struct {
		name     string
		addr     string
		handler  http.Handler
		logger   *slog.Logger
		options  []Option
		wantType string
	}{
		name:     "basic server creation",
		addr:     ":0",
		handler:  handler,
		logger:   logger,
		options:  nil,
		wantType: "*ares.httpServer",
	}
	tests[1] = struct {
		name     string
		addr     string
		handler  http.Handler
		logger   *slog.Logger
		options  []Option
		wantType string
	}{
		name:    "server with custom timeouts",
		addr:    ":0",
		handler: handler,
		logger:  logger,
		options: func() []Option {
			opts := make([]Option, 0, 3)
			opts = append(opts, WithReadTimeout(5*time.Second), WithWriteTimeout(5*time.Second), WithIdleTimeout(10*time.Second))
			return opts
		}(),
		wantType: "*ares.httpServer",
	}
	tests[2] = struct {
		name     string
		addr     string
		handler  http.Handler
		logger   *slog.Logger
		options  []Option
		wantType string
	}{
		name:    "server with all options",
		addr:    ":0",
		handler: handler,
		logger:  logger,
		options: func() []Option {
			opts := make([]Option, 0, 4)
			opts = append(opts, WithReadTimeout(15*time.Second), WithWriteTimeout(15*time.Second), WithIdleTimeout(30*time.Second), WithShutdownTimeout(5*time.Second))
			return opts
		}(),
		wantType: "*ares.httpServer",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewHTTPServer(tt.addr, tt.handler, tt.logger, tt.options...)

			if server == nil {
				t.Fatal("Expected server to be created, got nil")
			}

			// Check that it implements Server interface
			var _ Server = server

			// Check internal type
			if fmt.Sprintf("%T", server) != tt.wantType {
				t.Errorf("Expected server type %s, got %T", tt.wantType, server)
			}
		})
	}
}

func TestHTTPServerStartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Get a free port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal("Failed to get free port:", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	addr := fmt.Sprintf(":%d", port)
	server := NewHTTPServer(addr, handler, logger)

	// Test server start and stop
	var wg sync.WaitGroup
	var startErr error

	// Start server in goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		startErr = server.Start(context.Background())
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test that server is responding
	resp, err := http.Get(fmt.Sprintf("http://localhost%s", addr))
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Stop server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	// Wait for start goroutine to finish
	wg.Wait()

	// Check that start didn't return an error (server closed gracefully)
	if startErr != nil && !errors.Is(startErr, http.ErrServerClosed) {
		t.Errorf("Expected no error or ErrServerClosed, got: %v", startErr)
	}
}

func TestHTTPServerStopBeforeStart(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := NewHTTPServer(":0", handler, logger)

	// Stop server before starting - should not error
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := server.Stop(ctx)
	if err != nil {
		t.Errorf("Expected no error when stopping server before start, got: %v", err)
	}
}

func TestHTTPServerGracefulShutdown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Handler that takes time to respond
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // Simulate work
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("completed"))
	})

	// Get a free port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal("Failed to get free port:", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	addr := fmt.Sprintf(":%d", port)
	server := NewHTTPServer(addr, handler, logger, WithShutdownTimeout(1*time.Second))

	// Start server
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		server.Start(context.Background())
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Start a request that will take time
	var reqWg sync.WaitGroup
	var reqErr error
	reqWg.Add(1)
	go func() {
		defer reqWg.Done()
		resp, err := http.Get(fmt.Sprintf("http://localhost%s", addr))
		if err != nil {
			reqErr = err
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			reqErr = fmt.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	}()

	// Give request time to start
	time.Sleep(50 * time.Millisecond)

	// Stop server - should wait for request to complete
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stopStart := time.Now()
	err = server.Stop(ctx)
	stopDuration := time.Since(stopStart)

	if err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	// Should have taken at least 200ms to allow request to complete
	if stopDuration < 150*time.Millisecond {
		t.Errorf("Server stopped too quickly (%v), should have waited for in-flight request", stopDuration)
	}

	// Wait for request to complete
	reqWg.Wait()
	if reqErr != nil {
		t.Errorf("Request should have completed successfully: %v", reqErr)
	}

	wg.Wait()
}

func TestHTTPServerMultipleStops(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Get a free port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal("Failed to get free port:", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	addr := fmt.Sprintf(":%d", port)
	server := NewHTTPServer(addr, handler, logger)

	// Start server
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		server.Start(context.Background())
	}()

	time.Sleep(100 * time.Millisecond) // Give server time to start

	// Stop server multiple times - should not error
	ctx1, cancel1 := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel1()

	err1 := server.Stop(ctx1)
	if err1 != nil {
		t.Errorf("First stop failed: %v", err1)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel2()

	err2 := server.Stop(ctx2)
	if err2 != nil {
		t.Errorf("Second stop failed: %v", err2)
	}

	wg.Wait()
}

func TestHTTPServerInvalidAddress(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Try to bind to an invalid address
	server := NewHTTPServer("invalid:address:format", handler, logger)

	err := server.Start(context.Background())
	if err == nil {
		t.Error("Expected error for invalid address, got nil")
		server.Stop(context.Background()) // Clean up if somehow it started
	}
}

func TestHTTPServerPortInUse(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Get a free port and immediately bind to it
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal("Failed to get free port:", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	// Don't close the listener - keep it bound

	addr := fmt.Sprintf(":%d", port)
	server := NewHTTPServer(addr, handler, logger)

	// Try to start server on the same port - should fail
	err = server.Start(context.Background())
	if err == nil {
		t.Error("Expected error for port in use, got nil")
		server.Stop(context.Background()) // Clean up if somehow it started
	}

	listener.Close() // Clean up
}

func TestHTTPServerContextCancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second) // Long-running request
		w.WriteHeader(http.StatusOK)
	})

	// Get a free port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal("Failed to get free port:", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	addr := fmt.Sprintf(":%d", port)
	server := NewHTTPServer(addr, handler, logger)

	// Start server
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		server.Start(context.Background())
	}()

	time.Sleep(100 * time.Millisecond) // Give server time to start

	// Try to stop with a cancelled context - should still work but may timeout
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = server.Stop(ctx)
	// Should not fail even with cancelled context, but might timeout quickly
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("Unexpected error with cancelled context: %v", err)
	}

	wg.Wait()
}

// Benchmark server creation
func BenchmarkNewHTTPServer(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server := NewHTTPServer(":0", handler, logger)
		_ = server
	}
}
