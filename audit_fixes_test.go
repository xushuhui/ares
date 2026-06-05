package ares

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	aresErrors "github.com/xushuhui/ares/errors"
	"github.com/xushuhui/ares/middleware/logger"
)

// P0-1: wrapHandler must use errors.Error.Code as HTTP status code.
func TestWrapHandlerUsesErrorsErrorStatusCode(t *testing.T) {
	app := New()
	app.GET("/bad", func(ctx *Context) error {
		return aresErrors.BadRequest("invalid input")
	})
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/bad", nil)
	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// P0-1: error response body must expose Message, not the internal "code=N, message=..." string.
func TestWrapHandlerErrorsErrorMessageBody(t *testing.T) {
	app := New()
	app.GET("/notfound", func(ctx *Context) error {
		return aresErrors.NotFound("resource missing")
	})
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/notfound", nil)
	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if body["error"] != "resource missing" {
		t.Errorf("expected 'resource missing', got %q", body["error"])
	}
	if strings.Contains(body["error"], "code=") {
		t.Errorf("response body must not contain internal code= format, got %q", body["error"])
	}
}

// P0-1: plain errors.New still returns 500.
func TestWrapHandlerPlainErrorStillReturns500(t *testing.T) {
	app := New()
	app.GET("/err", func(ctx *Context) error {
		return io.EOF
	})
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/err", nil)
	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for plain error, got %d", rr.Code)
	}
}

// P1-2: Bind must reject bodies larger than the default limit.
func TestContextBindRejectsOversizedBody(t *testing.T) {
	// Build a JSON body just over 4MB.
	overhead := `{"data":"`
	suffix := `"}`
	padding := strings.Repeat("x", (4<<20)+1)
	bigBody := overhead + padding + suffix

	app := New()
	app.POST("/bind", func(ctx *Context) error {
		var v map[string]any
		return ctx.Bind(&v)
	})
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/bind", strings.NewReader(bigBody))
	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)
	// Bind must return error → handler propagates → 500
	if rr.Code == http.StatusOK {
		t.Error("expected Bind to fail on oversized body, but handler returned 200")
	}
}

// P2-3: logger middleware must record bytes_written in the log entry.
func TestLoggerRecordsBytesWritten(t *testing.T) {
	var buf bytes.Buffer
	lg := slog.New(slog.NewJSONHandler(&buf, nil))

	app := New(WithLogger(lg))
	app.Use(logger.New(logger.WithLogger(lg)))
	app.GET("/hello", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, makeSingleStringMap("k", "v"))
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/hello", nil)
	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "bytes") {
		t.Errorf("expected logger to record bytes_written, log was: %s", logOutput)
	}
}
