package ares

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/xushuhui/ares/middleware/logger"
)

func TestNew(t *testing.T) {
	app := New()
	if app == nil {
		t.Fatal("Expected app to be created")
	}
	if app.logger == nil {
		t.Fatal("Expected logger to be initialized")
	}
}

func TestNewWithLogger(t *testing.T) {
	customLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	app := New(WithLogger(customLogger))

	if app.logger != customLogger {
		t.Error("Expected custom logger to be set")
	}
}

func TestDefault(t *testing.T) {
	app := Default()
	if app == nil {
		t.Fatal("Expected app to be created")
	}

	// Test that middleware is applied
	handlerCalled := false
	app.GET("/test", func(ctx *Context) error {
		handlerCalled = true
		return ctx.JSON(http.StatusOK, makeSingleStringMap("status", "ok"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	app.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("Handler was not called")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestDefaultLoggingMiddleware(t *testing.T) {
	app := Default()

	app.GET("/test", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, makeSingleStringMap("message", "test"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "test") {
		t.Error("Expected response to contain 'test'")
	}
}

func TestDefaultRecoveryMiddleware(t *testing.T) {
	app := Default()

	app.GET("/panic", func(ctx *Context) error {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	rr := httptest.NewRecorder()

	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "Internal Server Error") {
		t.Error("Expected error message in response")
	}
}

func TestDefaultMiddlewareOrder(t *testing.T) {
	app := Default()

	// Add a custom middleware after default
	middlewareCalled := false
	app.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			middlewareCalled = true
			next.ServeHTTP(w, r)
		})
	})

	app.GET("/test", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, makeSingleStringMap("status", "ok"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	app.ServeHTTP(rr, req)

	if !middlewareCalled {
		t.Error("Custom middleware was not called")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestNewVsDefault(t *testing.T) {
	// Test that New() doesn't have middleware
	appNew := New()
	appNew.GET("/test", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, makeSingleStringMap("status", "ok"))
	})

	// Test that Default() has middleware
	appDefault := Default()
	appDefault.GET("/test", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, makeSingleStringMap("status", "ok"))
	})

	// Both should work
	req := httptest.NewRequest("GET", "/test", nil)

	rr1 := httptest.NewRecorder()
	appNew.ServeHTTP(rr1, req)
	if rr1.Code != http.StatusOK {
		t.Errorf("New() app: Expected status 200, got %d", rr1.Code)
	}

	rr2 := httptest.NewRecorder()
	appDefault.ServeHTTP(rr2, req)
	if rr2.Code != http.StatusOK {
		t.Errorf("Default() app: Expected status 200, got %d", rr2.Code)
	}
}

func TestLoggerMiddlewareWithHandlerError(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	customLogger := slog.New(slog.NewJSONHandler(&buf, nil))

	// Create app with custom logger
	app := New(WithLogger(customLogger))
	app.Use(logger.New(logger.WithLogger(customLogger)))

	// Add a handler that returns an error
	testError := errors.New("test handler error")
	app.GET("/error", func(ctx *Context) error {
		return testError
	})

	req := httptest.NewRequest("GET", "/error", nil)
	rr := httptest.NewRecorder()

	app.ServeHTTP(rr, req)

	// Check response status
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}

	// Check that error is in response
	if !strings.Contains(rr.Body.String(), "test handler error") {
		t.Error("Expected error message in response body")
	}

	// Check that logger middleware logged the error
	logOutput := buf.String()
	if !strings.Contains(logOutput, "test handler error") {
		t.Error("Expected logger middleware to log the handler error")
	}
	if !strings.Contains(logOutput, "error") {
		t.Error("Expected log to contain 'error' field")
	}
	if !strings.Contains(logOutput, "/error") {
		t.Error("Expected log to contain the request path")
	}
}

func TestLoggerMiddlewareWithoutError(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	customLogger := slog.New(slog.NewJSONHandler(&buf, nil))

	// Create app with custom logger
	app := New(WithLogger(customLogger))
	app.Use(logger.New(logger.WithLogger(customLogger)))

	// Add a handler that succeeds
	app.GET("/success", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, makeSingleStringMap("status", "ok"))
	})

	req := httptest.NewRequest("GET", "/success", nil)
	rr := httptest.NewRecorder()

	app.ServeHTTP(rr, req)

	// Check response status
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Check that logger middleware did not log an error
	logOutput := buf.String()
	if strings.Contains(logOutput, `"error"`) {
		t.Error("Expected log not to contain 'error' field when handler succeeds")
	}
	if !strings.Contains(logOutput, "/success") {
		t.Error("Expected log to contain the request path")
	}
}

func TestLoggerMiddlewareWithHandlerErrorSeparatedLogger(t *testing.T) {
	var appBuf bytes.Buffer
	var middlewareBuf bytes.Buffer

	appLogger := slog.New(slog.NewJSONHandler(&appBuf, nil))
	middlewareLogger := slog.New(slog.NewJSONHandler(&middlewareBuf, nil))

	app := New(WithLogger(appLogger))
	app.Use(logger.New(logger.WithLogger(middlewareLogger)))

	testError := errors.New("separated logger handler error")
	app.GET("/error2", func(ctx *Context) error {
		return testError
	})

	req := httptest.NewRequest("GET", "/error2", nil)
	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}

	middlewareLog := middlewareBuf.String()
	if !strings.Contains(middlewareLog, "/error2") {
		t.Fatal("Expected middleware log to contain request path")
	}
	if !strings.Contains(middlewareLog, `"error"`) {
		t.Fatal("Expected middleware log to contain error field")
	}
	if !strings.Contains(middlewareLog, testError.Error()) {
		t.Fatalf("Expected middleware log to contain handler error, got: %s", middlewareLog)
	}
}

func TestRedirectThenErrorDoesNotAppendErrorBody(t *testing.T) {
	app := New()
	app.GET("/redirect", func(ctx *Context) error {
		_ = ctx.Redirect(http.StatusFound, "/next")
		return errors.New("redirect handler error")
	})

	req := httptest.NewRequest(http.MethodGet, "/redirect", nil)
	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("Expected status 302, got %d", rr.Code)
	}

	if strings.Contains(rr.Body.String(), "redirect handler error") {
		t.Fatalf("Expected redirect response body without appended error json, got: %q", rr.Body.String())
	}
}
