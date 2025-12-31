package recovery

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecovery(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	middleware := New(WithLogger(logger))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Check response
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "Internal Server Error") {
		t.Error("Expected error message in response")
	}

	// Check log output
	logOutput := buf.String()
	if !strings.Contains(logOutput, "panic recovered") {
		t.Error("Expected log to contain 'panic recovered'")
	}
	if !strings.Contains(logOutput, "test panic") {
		t.Error("Expected log to contain panic message")
	}
}

func TestRecoveryWithStackTrace(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	middleware := New(WithLogger(logger), WithStackTrace(true))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "stack") {
		t.Error("Expected log to contain stack trace")
	}
	if !strings.Contains(logOutput, "goroutine") {
		t.Error("Expected stack trace to contain 'goroutine'")
	}
}

func TestRecoveryWithoutStackTrace(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	middleware := New(WithLogger(logger), WithStackTrace(false))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	logOutput := buf.String()
	if strings.Contains(logOutput, "goroutine") {
		t.Error("Expected log to NOT contain stack trace")
	}
}

func TestRecoveryWithCustomHandler(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	customHandlerCalled := false
	var capturedError any

	middleware := New(WithLogger(logger), WithRecoveryHandler(func(w http.ResponseWriter, r *http.Request, err any) {
		customHandlerCalled = true
		capturedError = err
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Custom error response"))
	}))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("custom panic")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !customHandlerCalled {
		t.Error("Expected custom recovery handler to be called")
	}

	if capturedError != "custom panic" {
		t.Errorf("Expected captured error 'custom panic', got %v", capturedError)
	}

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "Custom error response") {
		t.Error("Expected custom error response")
	}
}

func TestRecoveryNoPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	middleware := New(WithLogger(logger))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if rr.Body.String() != "success" {
		t.Error("Expected normal response")
	}

	// Should not log anything when no panic
	logOutput := buf.String()
	if strings.Contains(logOutput, "panic") {
		t.Error("Should not log panic when there is none")
	}
}

func TestRecoveryDifferentPanicTypes(t *testing.T) {
	tests := []struct {
		name      string
		panicVal  any
		expectLog string
	}{
		{"String panic", "string error", "string error"},
		{"Error panic", http.ErrAbortHandler, "http: Handler"},
		{"Int panic", 42, "42"},
		{"Nil panic", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			middleware := New(WithLogger(logger))

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic(tt.panicVal)
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusInternalServerError {
				t.Errorf("Expected status 500, got %d", rr.Code)
			}

			if tt.expectLog != "" {
				logOutput := buf.String()
				if !strings.Contains(logOutput, "panic recovered") {
					t.Error("Expected log to contain 'panic recovered'")
				}
			}
		})
	}
}

func TestRecoveryLogFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	middleware := New(WithLogger(logger))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("POST", "/api/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "POST") {
		t.Error("Expected log to contain method")
	}
	if !strings.Contains(logOutput, "/api/test") {
		t.Error("Expected log to contain path")
	}
}

func TestRecoveryDefaultLogger(t *testing.T) {
	// Test that default logger is used when none provided
	middleware := New()

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}
}

func TestRecoveryContentType(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	middleware := New(WithLogger(logger))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}
