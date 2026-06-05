package ares

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newTestReq(method, path string, body io.Reader) *http.Request {
	return httptest.NewRequestWithContext(context.Background(), method, path, body)
}

func TestContextSetGet(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	w := httptest.NewRecorder()
	r := newTestReq("GET", "/test", nil)
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
	r := newTestReq("GET", "/test", nil)
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
	r := newTestReq("GET", "/test", nil)
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
	r := newTestReq("GET", "/test", nil)
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
	r := newTestReq("GET", "/test", nil)
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
	r := newTestReq("GET", "/test", nil)
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

func TestContextStoreResetStrategy(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	w := httptest.NewRecorder()
	r := newTestReq("GET", "/test", nil)
	ctx := NewContext(w, r, logger)

	for i := 0; i <= maxPooledStoreEntries; i++ {
		ctx.Set(fmt.Sprintf("k%d", i), i)
	}

	beforeStorePtr := reflect.ValueOf(ctx.store).Pointer()
	ctx.release()
	afterStorePtr := reflect.ValueOf(ctx.store).Pointer()

	if beforeStorePtr == afterStorePtr {
		t.Fatalf("Expected oversized store map to be reallocated on release")
	}
	if len(ctx.store) != 0 {
		t.Fatalf("Expected reallocated store map to be empty, got len=%d", len(ctx.store))
	}

	ctx2 := NewContext(w, r, logger)
	defer ctx2.release()
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
		return ctx.JSON(http.StatusOK, makeSingleStringMap("status", "ok"))
	})

	req := newTestReq("GET", "/test", nil)
	rr := httptest.NewRecorder()

	app.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("Handler was not called")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestContextParam(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Create a chi router to test URL parameters
	r := chi.NewRouter()
	r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(w, req, logger)
		defer ctx.release()

		id := ctx.Param("id")
		if id != "123" {
			t.Errorf("Expected id='123', got '%s'", id)
		}
		w.WriteHeader(http.StatusOK)
	})

	req := newTestReq("GET", "/users/123", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
}

func TestContextQuery(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test?name=john&age=30&active=true&score=95.5", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	// Test Query
	if name := ctx.Query("name"); name != "john" {
		t.Errorf("Expected name='john', got '%s'", name)
	}

	if missing := ctx.Query("missing"); missing != "" {
		t.Errorf("Expected empty string for missing query, got '%s'", missing)
	}
}

func TestContextQueryDefault(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test?name=john", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	// Test with existing parameter
	if name := ctx.QueryDefault("name", "default"); name != "john" {
		t.Errorf("Expected name='john', got '%s'", name)
	}

	// Test with missing parameter
	if age := ctx.QueryDefault("age", "25"); age != "25" {
		t.Errorf("Expected default value '25', got '%s'", age)
	}
}

func TestContextQueryInt(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test?age=30&invalid=abc", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	// Test valid int
	if age := ctx.QueryInt("age", 0); age != 30 {
		t.Errorf("Expected age=30, got %d", age)
	}

	// Test invalid int (should return default)
	if invalid := ctx.QueryInt("invalid", 25); invalid != 25 {
		t.Errorf("Expected default value 25 for invalid int, got %d", invalid)
	}

	// Test missing parameter
	if missing := ctx.QueryInt("missing", 50); missing != 50 {
		t.Errorf("Expected default value 50 for missing parameter, got %d", missing)
	}
}

func TestContextQueryBool(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test?active=true&inactive=false&invalid=abc", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	// Test true value
	if active := ctx.QueryBool("active", false); active != true {
		t.Errorf("Expected active=true, got %v", active)
	}

	// Test false value
	if inactive := ctx.QueryBool("inactive", true); inactive != false {
		t.Errorf("Expected inactive=false, got %v", inactive)
	}

	// Test invalid bool (should return default)
	if invalid := ctx.QueryBool("invalid", true); invalid != true {
		t.Errorf("Expected default value true for invalid bool, got %v", invalid)
	}

	// Test missing parameter
	if missing := ctx.QueryBool("missing", true); missing != true {
		t.Errorf("Expected default value true for missing parameter, got %v", missing)
	}
}

func TestContextQueryFloat(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test?score=95.5&invalid=abc", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	// Test valid float
	if score := ctx.QueryFloat("score", 0.0); score != 95.5 {
		t.Errorf("Expected score=95.5, got %f", score)
	}

	// Test invalid float (should return default)
	if invalid := ctx.QueryFloat("invalid", 100.0); invalid != 100.0 {
		t.Errorf("Expected default value 100.0 for invalid float, got %f", invalid)
	}

	// Test missing parameter
	if missing := ctx.QueryFloat("missing", 75.0); missing != 75.0 {
		t.Errorf("Expected default value 75.0 for missing parameter, got %f", missing)
	}
}

func TestContextFormValue(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Create form data
	form := url.Values{}
	form.Add("username", "johndoe")
	form.Add("email", "john@example.com")

	req := newTestReq("POST", "/test", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	// Test existing form values
	if username := ctx.FormValue("username"); username != "johndoe" {
		t.Errorf("Expected username='johndoe', got '%s'", username)
	}

	if email := ctx.FormValue("email"); email != "john@example.com" {
		t.Errorf("Expected email='john@example.com', got '%s'", email)
	}

	// Test missing form value
	if missing := ctx.FormValue("missing"); missing != "" {
		t.Errorf("Expected empty string for missing form value, got '%s'", missing)
	}
}

func TestContextJSON(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	data := make(map[string]any, 3)
	data["name"] = "john"
	data["age"] = 30
	data["active"] = true

	err := ctx.JSON(http.StatusOK, data)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if contentType := w.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	// Verify JSON content
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if result["name"] != "john" {
		t.Errorf("Expected name='john', got '%v'", result["name"])
	}
}

func TestContextString(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	text := "Hello, World!"
	err := ctx.String(http.StatusOK, text)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if contentType := w.Header().Get("Content-Type"); contentType != "text/plain" {
		t.Errorf("Expected Content-Type 'text/plain', got '%s'", contentType)
	}

	if body := w.Body.String(); body != text {
		t.Errorf("Expected body '%s', got '%s'", text, body)
	}
}

func TestContextStatus(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	ctx.Status(http.StatusNoContent)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}

	if body := w.Body.String(); body != "" {
		t.Errorf("Expected empty body, got '%s'", body)
	}
}

func TestContextBind(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	user := User{Name: "john", Age: 30}
	jsonData, _ := json.Marshal(user)

	req := newTestReq("POST", "/test", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	var result User
	err := ctx.Bind(&result)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.Name != "john" {
		t.Errorf("Expected name='john', got '%s'", result.Name)
	}

	if result.Age != 30 {
		t.Errorf("Expected age=30, got %d", result.Age)
	}
}

func TestContextBindInvalidJSON(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("POST", "/test", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	var result map[string]any
	err := ctx.Bind(&result)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestContextCookie(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})
	req.AddCookie(&http.Cookie{Name: "user", Value: "john"})
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	// Test existing cookie
	cookie, err := ctx.Cookie("session")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if cookie.Value != "abc123" {
		t.Errorf("Expected session='abc123', got '%s'", cookie.Value)
	}

	// Test non-existing cookie
	_, err = ctx.Cookie("missing")
	if err == nil {
		t.Error("Expected error for missing cookie")
	}
}

func TestContextSetCookie(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	cookie := &http.Cookie{
		Name:     "session",
		Value:    "abc123",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   3600,
	}

	ctx.SetCookie(cookie)

	setCookieHeader := w.Header().Get("Set-Cookie")
	if !strings.Contains(setCookieHeader, "session=abc123") {
		t.Errorf("Expected Set-Cookie header to contain 'session=abc123', got '%s'", setCookieHeader)
	}
}

func TestContextRedirect(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	err := ctx.Redirect(http.StatusFound, "/new-location")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if w.Code != http.StatusFound {
		t.Errorf("Expected status 302, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/new-location" {
		t.Errorf("Expected Location '/new-location', got '%s'", location)
	}
}

func TestContextRedirectInvalidCode(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	// Use invalid redirect code - should default to 302
	err := ctx.Redirect(200, "/new-location")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if w.Code != http.StatusFound {
		t.Errorf("Expected status 302 for invalid redirect code, got %d", w.Code)
	}
}

func TestContextGetSetHeader(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("User-Agent", "TestClient/1.0")
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	// Test GetHeader
	auth := ctx.GetHeader("Authorization")
	if auth != "Bearer token123" {
		t.Errorf("Expected Authorization='Bearer token123', got '%s'", auth)
	}

	userAgent := ctx.GetHeader("User-Agent")
	if userAgent != "TestClient/1.0" {
		t.Errorf("Expected User-Agent='TestClient/1.0', got '%s'", userAgent)
	}

	missing := ctx.GetHeader("Missing-Header")
	if missing != "" {
		t.Errorf("Expected empty string for missing header, got '%s'", missing)
	}

	// Test SetHeader
	ctx.SetHeader("X-Custom-Header", "custom-value")
	ctx.SetHeader("X-API-Version", "1.0")

	if header := w.Header().Get("X-Custom-Header"); header != "custom-value" {
		t.Errorf("Expected X-Custom-Header='custom-value', got '%s'", header)
	}

	if header := w.Header().Get("X-API-Version"); header != "1.0" {
		t.Errorf("Expected X-API-Version='1.0', got '%s'", header)
	}
}

func TestContextFile(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Create a temporary file
	tempFile := filepath.Join(os.TempDir(), "test_file.txt")
	content := "Hello, World!"
	err := os.WriteFile(tempFile, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tempFile) }()

	req := newTestReq("GET", "/file", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	err = ctx.File(tempFile)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if body := w.Body.String(); body != content {
		t.Errorf("Expected body '%s', got '%s'", content, body)
	}
}

func TestContextFileNotFound(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/file", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	err := ctx.File("/non/existent/file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestContextAttachment(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Create a temporary file
	tempFile := filepath.Join(os.TempDir(), "test_attachment.txt")
	content := "Attachment content"
	err := os.WriteFile(tempFile, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tempFile) }()

	req := newTestReq("GET", "/download", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	err = ctx.Attachment(tempFile, "my_file.txt")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentDisposition := w.Header().Get("Content-Disposition")
	_, params, parseErr := mime.ParseMediaType(contentDisposition)
	if parseErr != nil {
		t.Fatalf("Expected parseable Content-Disposition, got error: %v", parseErr)
	}
	if params["filename"] != "my_file.txt" {
		t.Errorf("Expected filename my_file.txt, got '%s'", params["filename"])
	}

	if body := w.Body.String(); body != content {
		t.Errorf("Expected body '%s', got '%s'", content, body)
	}
}

func TestContextAttachmentDefaultFilename(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Create a temporary file
	tempFile := filepath.Join(os.TempDir(), "default_name.txt")
	content := "Default filename test"
	err := os.WriteFile(tempFile, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tempFile) }()

	req := newTestReq("GET", "/download", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	// Use empty filename - should use base name only
	err = ctx.Attachment(tempFile, "")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	contentDisposition := w.Header().Get("Content-Disposition")
	_, params, parseErr := mime.ParseMediaType(contentDisposition)
	if parseErr != nil {
		t.Fatalf("Expected parseable Content-Disposition, got error: %v", parseErr)
	}
	if params["filename"] != "default_name.txt" {
		t.Errorf("Expected base filename default_name.txt, got '%s'", params["filename"])
	}
	if strings.Contains(contentDisposition, string(os.PathSeparator)) {
		t.Errorf("Expected Content-Disposition not to leak path, got '%s'", contentDisposition)
	}
}

func TestContextAttachmentEscapedFilename(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	tempFile := filepath.Join(os.TempDir(), "escape_name.txt")
	content := "Escape filename test"
	err := os.WriteFile(tempFile, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tempFile) }()

	req := newTestReq("GET", "/download", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	err = ctx.Attachment(tempFile, `my"file.txt`)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	contentDisposition := w.Header().Get("Content-Disposition")
	_, params, parseErr := mime.ParseMediaType(contentDisposition)
	if parseErr != nil {
		t.Fatalf("Expected parseable Content-Disposition, got error: %v", parseErr)
	}
	if params["filename"] != `my"file.txt` {
		t.Errorf("Expected filename my\"file.txt, got '%s'", params["filename"])
	}
}

func TestContextStream(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/stream", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	content := "Streaming content"
	reader := strings.NewReader(content)

	err := ctx.Stream("text/plain", reader)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if contentType := w.Header().Get("Content-Type"); contentType != "text/plain" {
		t.Errorf("Expected Content-Type 'text/plain', got '%s'", contentType)
	}

	if body := w.Body.String(); body != content {
		t.Errorf("Expected body '%s', got '%s'", content, body)
	}
}

func TestContextMultipartForm(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Create multipart form data
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("name", "john")
	_ = writer.WriteField("email", "john@example.com")
	_ = writer.Close()

	req := newTestReq("POST", "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	form, err := ctx.MultipartForm()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if name := form.Value["name"][0]; name != "john" {
		t.Errorf("Expected name='john', got '%s'", name)
	}

	if email := form.Value["email"][0]; email != "john@example.com" {
		t.Errorf("Expected email='john@example.com', got '%s'", email)
	}
}

func TestContextFormFile(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Create multipart form with file
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	fileWriter, err := writer.CreateFormFile("upload", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	_, _ = fileWriter.Write([]byte("file content"))

	_ = writer.WriteField("name", "john")
	_ = writer.Close()

	req := newTestReq("POST", "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	fileHeader, err := ctx.FormFile("upload")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if fileHeader.Filename != "test.txt" {
		t.Errorf("Expected filename='test.txt', got '%s'", fileHeader.Filename)
	}

	if fileHeader.Size != 12 { // "file content" is 12 bytes
		t.Errorf("Expected file size=12, got %d", fileHeader.Size)
	}
}

func TestContextFormFileNotFound(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Create form without file
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("name", "john")
	_ = writer.Close()

	req := newTestReq("POST", "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	_, err := ctx.FormFile("missing")
	if err == nil {
		t.Error("Expected error for missing file")
	}
}

func TestContextSaveUploadedFile(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Create multipart form with file
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	fileWriter, err := writer.CreateFormFile("upload", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	content := "file content for saving"
	_, _ = fileWriter.Write([]byte(content))
	_ = writer.Close()

	req := newTestReq("POST", "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	fileHeader, err := ctx.FormFile("upload")
	if err != nil {
		t.Fatalf("Failed to get form file: %v", err)
	}

	tempFile := filepath.Join(os.TempDir(), "saved_file.txt")
	defer func() { _ = os.Remove(tempFile) }()

	err = ctx.SaveUploadedFile(fileHeader, tempFile)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify file was saved correctly
	savedContent, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	if string(savedContent) != content {
		t.Errorf("Expected saved content '%s', got '%s'", content, string(savedContent))
	}
}

func TestContextWriteTracking(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	// Initially should not be written
	if ctx.written {
		t.Error("Expected written=false initially")
	}

	// WriteHeader should set written=true
	ctx.WriteHeader(http.StatusOK)
	if !ctx.written {
		t.Error("Expected written=true after WriteHeader")
	}

	// Multiple WriteHeader calls should not change status
	ctx.WriteHeader(http.StatusBadRequest)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 (first call), got %d", w.Code)
	}
}

func TestContextWriteMethod(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	content := "Hello, World!"
	n, err := ctx.Write([]byte(content))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if n != len(content) {
		t.Errorf("Expected %d bytes written, got %d", len(content), n)
	}

	if !ctx.written {
		t.Error("Expected written=true after Write")
	}

	if body := w.Body.String(); body != content {
		t.Errorf("Expected body '%s', got '%s'", content, body)
	}
}

// Benchmark tests for performance-critical methods
func BenchmarkContextQuery(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test?name=john&age=30&active=true", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := NewContext(w, req, logger)
		_ = ctx.Query("name")
		ctx.release()
	}
}

func BenchmarkContextQueryInt(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test?age=30", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := NewContext(w, req, logger)
		_ = ctx.QueryInt("age", 0)
		ctx.release()
	}
}

func BenchmarkContextJSON(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	req := newTestReq("GET", "/test", nil)

	data := make(map[string]any, 2)
	data["name"] = "john"
	data["age"] = 30

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		ctx := NewContext(w, req, logger)
		_ = ctx.JSON(http.StatusOK, data)
		ctx.release()
	}
}

func TestContextFlush(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	// Test Flush with httptest.ResponseRecorder (which implements http.Flusher)
	ctx.Flush()
}

func TestContextFlushNotSupported(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	req := newTestReq("GET", "/test", nil)
	// Create a custom ResponseWriter that doesn't implement http.Flusher
	w := &nonFlusherResponseWriter{}
	ctx := NewContext(w, req, logger)
	defer ctx.release()

	// Test Flush with non-flusher ResponseWriter
	ctx.Flush()
}

// nonFlusherResponseWriter is a ResponseWriter that doesn't implement http.Flusher
type nonFlusherResponseWriter struct {
	statusCode int
	headers    http.Header
	body       *bytes.Buffer
}

func (w *nonFlusherResponseWriter) Header() http.Header {
	if w.headers == nil {
		w.headers = make(http.Header)
	}
	return w.headers
}

func (w *nonFlusherResponseWriter) Write(data []byte) (int, error) {
	if w.body == nil {
		w.body = &bytes.Buffer{}
	}
	return w.body.Write(data)
}

func (w *nonFlusherResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

type unwrapOnlyResponseWriter struct {
	http.ResponseWriter
}

func (w *unwrapOnlyResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

type flushCounterWriter struct {
	*httptest.ResponseRecorder
	flushCount int
}

func (w *flushCounterWriter) Flush() {
	w.flushCount++
	w.ResponseRecorder.Flush()
}

func TestContextFlushViaUnwrap(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	req := newTestReq("GET", "/test", nil)
	base := &flushCounterWriter{ResponseRecorder: httptest.NewRecorder()}
	wrapped := &unwrapOnlyResponseWriter{ResponseWriter: base}

	ctx := NewContext(wrapped, req, logger)
	defer ctx.release()

	ctx.Flush()

	if base.flushCount == 0 {
		t.Fatal("expected Flush to reach underlying ResponseWriter via Unwrap")
	}
}

func BenchmarkContextFlush(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	req := newTestReq("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		ctx := NewContext(w, req, logger)
		ctx.Flush()
		ctx.release()
	}
}
