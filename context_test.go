package ares

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestContextSetGet(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(w, r, logger)
	defer ctx.release()

	// Test Set and Get
	ctx.Set("key1", "value1")
	ctx.Set("key2", 123)
	ctx.Set("key3", true)

	// Test Get with existing keys
	if val, ok := ctx.Get("key1"); !ok || val != "value1" {
		t.Errorf("Expected key1='value1', got %v", val)
	}

	if val, ok := ctx.Get("key2"); !ok || val != 123 {
		t.Errorf("Expected key2=123, got %v", val)
	}

	if val, ok := ctx.Get("key3"); !ok || val != true {
		t.Errorf("Expected key3=true, got %v", val)
	}

	// Test Get with non-existing key
	if _, ok := ctx.Get("nonexistent"); ok {
		t.Error("Expected nonexistent key to return false")
	}
}

func TestContextMustGet(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(w, r, logger)
	defer ctx.release()

	ctx.Set("key", "value")

	// Test MustGet with existing key
	val := ctx.MustGet("key")
	if val != "value" {
		t.Errorf("Expected 'value', got %v", val)
	}

	// Test MustGet with non-existing key (should panic)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected MustGet to panic for non-existing key")
		}
	}()
	ctx.MustGet("nonexistent")
}

func TestContextGetString(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(w, r, logger)
	defer ctx.release()

	ctx.Set("string_key", "hello")
	ctx.Set("int_key", 123)

	// Test GetString with string value
	if val := ctx.GetString("string_key"); val != "hello" {
		t.Errorf("Expected 'hello', got %v", val)
	}

	// Test GetString with non-string value
	if val := ctx.GetString("int_key"); val != "" {
		t.Errorf("Expected empty string for non-string value, got %v", val)
	}

	// Test GetString with non-existing key
	if val := ctx.GetString("nonexistent"); val != "" {
		t.Errorf("Expected empty string for non-existing key, got %v", val)
	}
}

func TestContextGetInt(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(w, r, logger)
	defer ctx.release()

	ctx.Set("int_key", 42)
	ctx.Set("string_key", "hello")

	// Test GetInt with int value
	if val := ctx.GetInt("int_key"); val != 42 {
		t.Errorf("Expected 42, got %v", val)
	}

	// Test GetInt with non-int value
	if val := ctx.GetInt("string_key"); val != 0 {
		t.Errorf("Expected 0 for non-int value, got %v", val)
	}

	// Test GetInt with non-existing key
	if val := ctx.GetInt("nonexistent"); val != 0 {
		t.Errorf("Expected 0 for non-existing key, got %v", val)
	}
}

func TestContextGetBool(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(w, r, logger)
	defer ctx.release()

	ctx.Set("bool_key", true)
	ctx.Set("string_key", "hello")

	// Test GetBool with bool value
	if val := ctx.GetBool("bool_key"); val != true {
		t.Errorf("Expected true, got %v", val)
	}

	// Test GetBool with non-bool value
	if val := ctx.GetBool("string_key"); val != false {
		t.Errorf("Expected false for non-bool value, got %v", val)
	}

	// Test GetBool with non-existing key
	if val := ctx.GetBool("nonexistent"); val != false {
		t.Errorf("Expected false for non-existing key, got %v", val)
	}
}

func TestContextStoreClearing(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(w, r, logger)

	// Set some values
	ctx.Set("key1", "value1")
	ctx.Set("key2", "value2")

	// Release context (should clear store)
	ctx.release()

	// Get context from pool again
	ctx2 := NewContext(w, r, logger)
	defer ctx2.release()

	// Store should be empty
	if _, ok := ctx2.Get("key1"); ok {
		t.Error("Expected store to be cleared after release")
	}
	if _, ok := ctx2.Get("key2"); ok {
		t.Error("Expected store to be cleared after release")
	}
}

func TestContextStoreInHandler(t *testing.T) {
	app := New()

	// Middleware that sets a value
	app.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// We need to extract the Context from the handler
			// This is a simplified test
			next.ServeHTTP(w, r)
		})
	})

	handlerCalled := false
	app.GET("/test", func(ctx *Context) error {
		ctx.Set("user_id", 123)
		ctx.Set("username", "testuser")

		if userID := ctx.GetInt("user_id"); userID != 123 {
			t.Errorf("Expected user_id=123, got %v", userID)
		}

		if username := ctx.GetString("username"); username != "testuser" {
			t.Errorf("Expected username='testuser', got %v", username)
		}

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
