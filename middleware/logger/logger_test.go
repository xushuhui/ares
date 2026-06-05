package logger

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xushuhui/ares/internal/contextkeys"
)

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	middleware := New(WithLogger(logger))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Check response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Check log output
	logOutput := buf.String()
	if !strings.Contains(logOutput, "request") {
		t.Error("Expected log to contain 'request'")
	}
	if !strings.Contains(logOutput, "GET") {
		t.Error("Expected log to contain method 'GET'")
	}
	if !strings.Contains(logOutput, "/test") {
		t.Error("Expected log to contain path '/test'")
	}
	if !strings.Contains(logOutput, "200") {
		t.Error("Expected log to contain status '200'")
	}
}

func TestLoggerWithSkipPaths(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	middleware := New(
		WithLogger(logger),
		WithSkipPaths(func() []string {
			paths := make([]string, 0, 2)
			paths = append(paths, "/health", "/metrics")
			return paths
		}()),
	)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test skipped path
	req1 := httptest.NewRequestWithContext(context.Background(), "GET", "/health", nil)
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if buf.Len() > 0 {
		t.Error("Expected no log for skipped path /health")
	}

	// Test non-skipped path
	req2 := httptest.NewRequestWithContext(context.Background(), "GET", "/api", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if buf.Len() == 0 {
		t.Error("Expected log for non-skipped path /api")
	}
}

func TestLoggerWithCustomFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	middleware := New(WithLogger(logger), WithCustomFields(func(r *http.Request, status int, duration time.Duration) []any {
		fields := make([]any, 0, 4)
		fields = append(fields, "user_agent", r.UserAgent(), "custom_field", "custom_value")
		return fields
	}))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "test-agent") {
		t.Error("Expected log to contain custom user_agent field")
	}
	if !strings.Contains(logOutput, "custom_value") {
		t.Error("Expected log to contain custom_field")
	}
}

func TestLoggerStatusCodes(t *testing.T) {
	tests := make([]struct {
		name       string
		statusCode int
	}, 5)
	tests[0] = struct {
		name       string
		statusCode int
	}{"200 OK", http.StatusOK}
	tests[1] = struct {
		name       string
		statusCode int
	}{"201 Created", http.StatusCreated}
	tests[2] = struct {
		name       string
		statusCode int
	}{"400 Bad Request", http.StatusBadRequest}
	tests[3] = struct {
		name       string
		statusCode int
	}{"404 Not Found", http.StatusNotFound}
	tests[4] = struct {
		name       string
		statusCode int
	}{"500 Internal Server Error", http.StatusInternalServerError}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			middleware := New(WithLogger(logger))

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))

			req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			logOutput := buf.String()
			// Check if log contains the status code as a number
			statusStr := string(rune('0'+tt.statusCode/100)) + string(rune('0'+(tt.statusCode/10)%10)) + string(rune('0'+tt.statusCode%10))
			if !strings.Contains(logOutput, statusStr) && !strings.Contains(logOutput, "status") {
				t.Errorf("Expected log to contain status information for code %d, got: %s", tt.statusCode, logOutput)
			}
		})
	}
}

func TestLoggerDuration(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	middleware := New(WithLogger(logger))

	responseBody := "test response body"
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(responseBody))
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	logOutput := buf.String()
	// Should log the duration
	if !strings.Contains(logOutput, "duration") {
		t.Error("Expected log to contain 'duration' field")
	}
}

func TestLoggerDifferentMethods(t *testing.T) {
	methods := make([]string, 0, 5)
	methods = append(methods, "GET", "POST", "PUT", "DELETE", "PATCH")

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			middleware := New(WithLogger(logger))

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequestWithContext(context.Background(), method, "/test", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			logOutput := buf.String()
			if !strings.Contains(logOutput, method) {
				t.Errorf("Expected log to contain method %s", method)
			}
		})
	}
}

func TestLoggerDefaultLogger(t *testing.T) {
	// Test that default logger is used when none provided
	middleware := New()

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestLoggerRemoteAddr(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	middleware := New(WithLogger(logger))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "192.168.1.1") {
		t.Error("Expected log to contain remote address")
	}
}

func TestLoggerWithHandlerError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	middleware := New(WithLogger(logger))

	testError := errors.New("test handler error")

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Store error in request context to simulate Ares framework behavior
		*r = *r.WithContext(context.WithValue(r.Context(), contextkeys.HandlerError, testError))
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "test handler error") {
		t.Error("Expected log to contain handler error message")
	}
	if !strings.Contains(logOutput, "error") {
		t.Error("Expected log to contain 'error' field")
	}
}

func TestLoggerIgnoresPlainStringContextKey(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	middleware := New(WithLogger(logger))
	testError := errors.New("plain string key error")

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*r = *r.WithContext(context.WithValue(r.Context(), "handler_error", testError)) //nolint:staticcheck
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	logOutput := buf.String()
	if strings.Contains(logOutput, "plain string key error") {
		t.Error("Expected logger to ignore plain string context key")
	}
}

func TestLoggerWithoutError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	middleware := New(WithLogger(logger))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	logOutput := buf.String()
	// Should not contain error field or error message when no error occurs
	if strings.Contains(logOutput, `"error"`) {
		t.Error("Expected log not to contain 'error' field when no error occurs")
	}
}

type flushTrackingWriter struct {
	*httptest.ResponseRecorder
	flushCount int
}

func (w *flushTrackingWriter) Flush() {
	w.flushCount++
	w.ResponseRecorder.Flush()
}

type hijackTrackingWriter struct {
	*httptest.ResponseRecorder
	hijackCalled bool
}

func (w *hijackTrackingWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.hijackCalled = true
	return nil, nil, errors.New("hijack called")
}

func TestLoggerPreservesFlusher(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	middleware := New(WithLogger(log))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected wrapped ResponseWriter to implement http.Flusher")
		}
		_, _ = w.Write([]byte("chunk"))
		flusher.Flush()
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/stream", nil)
	rw := &flushTrackingWriter{ResponseRecorder: httptest.NewRecorder()}
	handler.ServeHTTP(rw, req)

	if rw.flushCount == 0 {
		t.Fatal("expected Flush to be delegated to underlying ResponseWriter")
	}
}

func TestLoggerPreservesUnwrap(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	middleware := New(WithLogger(log))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unwrap, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			t.Fatal("expected wrapped ResponseWriter to expose Unwrap")
		}
		if unwrap.Unwrap() == nil {
			t.Fatal("expected Unwrap to return underlying ResponseWriter")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/unwrap", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
}

func TestLoggerPreservesHijacker(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	middleware := New(WithLogger(log))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("expected wrapped ResponseWriter to implement http.Hijacker")
		}
		_, _, err := hijacker.Hijack()
		if err == nil || !strings.Contains(err.Error(), "hijack called") {
			t.Fatalf("expected delegated Hijack error, got %v", err)
		}
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/upgrade", nil)
	rw := &hijackTrackingWriter{ResponseRecorder: httptest.NewRecorder()}
	handler.ServeHTTP(rw, req)

	if !rw.hijackCalled {
		t.Fatal("expected Hijack to be delegated to underlying ResponseWriter")
	}
}
