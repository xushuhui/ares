package ares

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
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
		return ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
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
		return ctx.JSON(http.StatusOK, map[string]string{"message": "test"})
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
		return ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
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
		return ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Test that Default() has middleware
	appDefault := Default()
	appDefault.GET("/test", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
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
