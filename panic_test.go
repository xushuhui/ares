package ares

import (
	"net/http"
	"net/http/httptest"
	"testing"

)

// TestNilPointerPanic tests that ares recovers from nil pointer panic
func TestNilPointerPanic(t *testing.T) {
	app := Default() // Default includes recovery middleware

	// Handler that causes nil pointer panic
	app.GET("/panic/nil", func(c *Context) error {
		var ptr *string
		_ = *ptr // This will panic
		return nil
	})

	req := httptest.NewRequest("GET", "/panic/nil", nil)
	w := httptest.NewRecorder()

	app.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	if w.Body.String() != `{"error":"Internal Server Error"}` {
		t.Errorf("Expected error response, got: %s", w.Body.String())
	}
}

// TestSliceOutOfBoundsPanic tests slice index out of bounds panic
func TestSliceOutOfBoundsPanic(t *testing.T) {
	app := Default()

	app.GET("/panic/slice", func(c *Context) error {
		slice := []int{1, 2, 3}
		_ = slice[10] // This will panic
		return nil
	})

	req := httptest.NewRequest("GET", "/panic/slice", nil)
	w := httptest.NewRecorder()

	app.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

// TestMapNilPanic tests nil map access panic
func TestMapNilPanic(t *testing.T) {
	app := Default()

	app.GET("/panic/map", func(c *Context) error {
		var m map[string]string
		m["key"] = "value" // This will panic
		return nil
	})

	req := httptest.NewRequest("GET", "/panic/map", nil)
	w := httptest.NewRecorder()

	app.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

// TestNormalHandlerAfterPanic tests that other handlers still work after a panic
func TestNormalHandlerAfterPanic(t *testing.T) {
	app := Default()

	app.GET("/panic", func(c *Context) error {
		panic("test panic")
	})

	app.GET("/normal", func(c *Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// First request panics
	req1 := httptest.NewRequest("GET", "/panic", nil)
	w1 := httptest.NewRecorder()
	app.ServeHTTP(w1, req1)

	if w1.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for panic, got %d", w1.Code)
	}

	// Second request should work normally
	req2 := httptest.NewRequest("GET", "/normal", nil)
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 for normal handler, got %d", w2.Code)
	}
}

// TestWithoutRecoveryMiddleware tests panic without recovery middleware
func TestWithoutRecoveryMiddleware(t *testing.T) {
	app := New() // Without recovery middleware

	app.GET("/panic", func(c *Context) error {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	// This should panic and not be recovered
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic to propagate without recovery middleware")
		}
	}()

	app.ServeHTTP(w, req)
}
