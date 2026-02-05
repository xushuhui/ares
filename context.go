package ares

import (
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/go-chi/chi/v5"
)

// contextPool is a sync.Pool for Context objects
var contextPool = sync.Pool{
	New: func() any {
		return &Context{}
	},
}

// Context wraps http.ResponseWriter and http.Request with additional helper methods
type Context struct {
	http.ResponseWriter
	Request *http.Request
	logger  *slog.Logger
	written bool
	store   map[string]any // Key-value storage for passing data between middleware
	err     error          // Handler error for middleware access
}

// NewContext creates a new Context instance from the pool
func NewContext(w http.ResponseWriter, r *http.Request, logger *slog.Logger) *Context {
	ctx := contextPool.Get().(*Context)
	ctx.ResponseWriter = w
	ctx.Request = r
	ctx.logger = logger
	ctx.written = false
	if ctx.store == nil {
		ctx.store = make(map[string]any)
	}
	return ctx
}

// release returns the Context to the pool
func (c *Context) release() {
	c.ResponseWriter = nil
	c.Request = nil
	c.logger = nil
	c.written = false
	c.err = nil
	// Clear the store map
	for k := range c.store {
		delete(c.store, k)
	}
	contextPool.Put(c)
}

// WriteHeader overrides http.ResponseWriter.WriteHeader to track if response was written
func (c *Context) WriteHeader(code int) {
	if !c.written {
		c.written = true
		c.ResponseWriter.WriteHeader(code)
	}
}

// Write overrides http.ResponseWriter.Write to track if response was written
func (c *Context) Write(b []byte) (int, error) {
	c.written = true
	return c.ResponseWriter.Write(b)
}

// JSON sends a JSON response with the given status code
func (c *Context) JSON(code int, v any) error {
	c.Header().Set("Content-Type", "application/json")
	c.WriteHeader(code)
	return json.NewEncoder(c).Encode(v)
}

// Bind decodes the request body into the provided value (JSON only)
func (c *Context) Bind(v any) error {
	return json.NewDecoder(c.Request.Body).Decode(v)
}

// Param returns the URL parameter from chi router
func (c *Context) Param(key string) string {
	return chi.URLParam(c.Request, key)
}

// Query returns the query parameter from the URL
func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

// QueryDefault returns the query parameter or default value if not present
func (c *Context) QueryDefault(key string, defaultValue string) string {
	if val := c.Request.URL.Query().Get(key); val != "" {
		return val
	}
	return defaultValue
}

// QueryInt returns the query parameter as int with default value
func (c *Context) QueryInt(key string, defaultValue int) int {
	val := c.Request.URL.Query().Get(key)
	if val == "" {
		return defaultValue
	}
	if i, err := strconv.Atoi(val); err == nil {
		return i
	}
	return defaultValue
}

// QueryBool returns the query parameter as bool with default value
func (c *Context) QueryBool(key string, defaultValue bool) bool {
	val := c.Request.URL.Query().Get(key)
	if val == "" {
		return defaultValue
	}
	if b, err := strconv.ParseBool(val); err == nil {
		return b
	}
	return defaultValue
}

// QueryFloat returns the query parameter as float64 with default value
func (c *Context) QueryFloat(key string, defaultValue float64) float64 {
	val := c.Request.URL.Query().Get(key)
	if val == "" {
		return defaultValue
	}
	if f, err := strconv.ParseFloat(val, 64); err == nil {
		return f
	}
	return defaultValue
}

// FormValue returns the form value for the given key
func (c *Context) FormValue(key string) string {
	return c.Request.FormValue(key)
}

// Logger returns the logger instance
func (c *Context) Logger() *slog.Logger {
	return c.logger
}

// Status sends a status code without a body
func (c *Context) Status(code int) {
	c.WriteHeader(code)
}

// String sends a plain text response
func (c *Context) String(code int, s string) error {
	c.Header().Set("Content-Type", "text/plain")
	c.WriteHeader(code)
	_, err := c.Write([]byte(s))
	return err
}

// Cookie returns the named cookie from the request
func (c *Context) Cookie(name string) (*http.Cookie, error) {
	return c.Request.Cookie(name)
}

// SetCookie adds a Set-Cookie header to the response
func (c *Context) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.ResponseWriter, cookie)
}

// Redirect sends a redirect response to the client
func (c *Context) Redirect(code int, url string) error {
	if code < 300 || code > 308 {
		code = http.StatusFound
	}
	http.Redirect(c.ResponseWriter, c.Request, url, code)
	return nil
}

// GetHeader returns the request header value
func (c *Context) GetHeader(key string) string {
	return c.Request.Header.Get(key)
}

// SetHeader sets a response header
func (c *Context) SetHeader(key, value string) {
	c.Header().Set(key, value)
}

// File sends a file as response
func (c *Context) File(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	http.ServeContent(c.ResponseWriter, c.Request, stat.Name(), stat.ModTime(), file)
	c.written = true
	return nil
}

// Attachment sends a file as an attachment (download)
func (c *Context) Attachment(filepath, filename string) error {
	if filename == "" {
		filename = filepath
	}
	c.SetHeader("Content-Disposition", `attachment; filename="`+filename+`"`)
	return c.File(filepath)
}

// Stream sends a stream response
func (c *Context) Stream(contentType string, reader io.Reader) error {
	c.SetHeader("Content-Type", contentType)
	c.WriteHeader(http.StatusOK)
	_, err := io.Copy(c.ResponseWriter, reader)
	return err
}

// FormFile returns the first file for the provided form key
func (c *Context) FormFile(name string) (*multipart.FileHeader, error) {
	if c.Request.MultipartForm == nil {
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32MB default
			return nil, err
		}
	}
	file, header, err := c.Request.FormFile(name)
	if err != nil {
		return nil, err
	}
	file.Close()
	return header, nil
}

// SaveUploadedFile saves the uploaded file to dst
func (c *Context) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

// MultipartForm returns the parsed multipart form, including file uploads
func (c *Context) MultipartForm() (*multipart.Form, error) {
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32MB default
		return nil, err
	}
	return c.Request.MultipartForm, nil
}

// Set stores a key-value pair in the context
func (c *Context) Set(key string, value any) {
	c.store[key] = value
}

// Get retrieves a value from the context by key
// Returns the value and a boolean indicating if the key exists
func (c *Context) Get(key string) (any, bool) {
	value, exists := c.store[key]
	return value, exists
}

// MustGet retrieves a value from the context by key
// Panics if the key doesn't exist
func (c *Context) MustGet(key string) any {
	if value, exists := c.store[key]; exists {
		return value
	}
	panic("key \"" + key + "\" does not exist")
}

// GetString retrieves a string value from the context
func (c *Context) GetString(key string) string {
	if value, ok := c.store[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

// GetInt retrieves an int value from the context
func (c *Context) GetInt(key string) int {
	if value, ok := c.store[key]; ok {
		if i, ok := value.(int); ok {
			return i
		}
	}
	return 0
}

// GetBool retrieves a bool value from the context
func (c *Context) GetBool(key string) bool {
	if value, ok := c.store[key]; ok {
		if b, ok := value.(bool); ok {
			return b
		}
	}
	return false
}

// SetError sets an error in the context for middleware access
func (c *Context) SetError(err error) {
	c.err = err
}

// Flush sends any buffered content to the client
// Returns false if the underlying ResponseWriter doesn't support flushing
func (c *Context) Flush() {
	if flusher, ok := c.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Error returns the error stored in the context
func (c *Context) Error() error {
	return c.err
}
// Unwrap returns the underlying http.ResponseWriter
func (c *Context) Unwrap() http.ResponseWriter {
	return c.ResponseWriter
}
