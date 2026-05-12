# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Ares is a lightweight, high-performance Go web framework built on top of chi router. It emphasizes standard library usage, minimal dependencies, and clean API design.

**Key Design Principles:**
- Standard library first (uses slog, encoding/json, net/http)
- Only 1 external dependency (chi router)
- No reflection or code generation
- Minimal abstraction over HTTP primitives

## Commands

### Testing
```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for specific package
go test ./middleware/logger
go test ./middleware/recovery

# Run specific test
go test -run TestContextJSON

# Run tests with coverage
go test -cover ./...

# Run a single test with timeout
go test -timeout 3s -run TestServerStartStop
```

### Building
```bash
# Build the project (library - no main package)
go build ./...

# Check for compilation errors
go build -o /dev/null ./...

# Verify code (vet checks for common mistakes)
go vet ./...
```

### Development
```bash
# Tidy dependencies
go mod tidy

# Verify dependencies
go mod verify

# Add new dependency
go get github.com/go-chi/chi/v5
```

## Architecture

### Core Components

**ares.go** - Framework core
- `Ares` struct embeds `chi.Mux` for routing
- `Handler` type: `func(*Context) error` - returns error for clean error handling
- Factory methods: `New()` (bare), `Default()` (with logger + recovery middleware)
- Route registration: `GET()`, `POST()`, `PUT()`, `DELETE()`, `PATCH()`
- `wrapHandler()` converts Ares handlers to http.HandlerFunc, handles errors automatically
- `Group` support for route grouping with shared prefix and middleware

**context.go** - Enhanced request context
- `Context` wraps http.ResponseWriter and http.Request
- Uses `sync.Pool` for object reuse (performance optimization)
- Provides helper methods: `JSON()`, `Bind()`, `Param()`, `Query*()`, `FormValue()`, etc.
- Key-value storage: `Set()`, `Get()`, `MustGet()`, `GetString()`, `GetInt()`, `GetBool()`
- Error storage: `SetError()`, `Error()` for middleware access to handler errors
- Tracks if response was written via `written` field to prevent double writes
- Static file serving: `Static()` for directories, `StaticFile()` for single files
- Streaming support: `Flush()` for SSE/streaming, `Unwrap()` to access underlying ResponseWriter

**server.go** - Server lifecycle management
- `Server` interface: `Start(context.Context)`, `Stop(context.Context)`
- `httpServer` implements graceful shutdown with configurable timeouts
- `NewHTTPServer()` creates server with options (read/write/idle/shutdown timeouts)
- Separation of concerns: Ares handles routing, Server handles lifecycle

**errors/error.go** - HTTP error helpers
- `Error` struct with `Code` and `Message` fields
- Helper functions: `BadRequest()`, `NotFound()`, `InternalError()`, `Forbidden()`, `Unauthorized()`
- Implements standard `error` interface
- Used for returning structured HTTP errors from handlers

### Middleware

Located in `middleware/` directory:

**logger/** - Request logging middleware
- Uses slog for structured logging
- Wraps ResponseWriter to capture status code and bytes written
- Configurable: skip paths, custom fields
- Logs: method, path, status, duration, bytes, IP
- Accesses handler errors from request context via typed key `contextkeys.HandlerError`

**recovery/** - Panic recovery middleware
- Recovers from panics to prevent server crashes
- Logs error with optional stack trace
- Returns JSON error response: `{"error":"Internal Server Error"}`
- Configurable: custom logger, enable/disable stack trace, custom recovery handler

**Extended middleware** (CORS, JWT, rate limiting, etc.) lives in separate `ares-contrib` repository.

### Handler Error Handling

Handlers return `error`. The framework automatically:
1. Logs the error with request details (path, method)
2. Stores error in request context via `contextkeys.HandlerError` for middleware access
3. Stores error in Context struct via `SetError()` for middleware access
4. If response not yet written, sends 500 JSON error response
5. Allows clean error propagation without manual error handling in each handler

### Context Pooling

`Context` objects are pooled using `sync.Pool`:
- `NewContext()` gets from pool
- `release()` returns to pool after request completes
- `release()` clears store entries for reuse; if store grows beyond `maxPooledStoreEntries`, it is dropped to avoid retaining oversized maps
- Reduces GC pressure and allocations

### Route Groups

Groups allow organizing routes with common prefix and middleware:
- Created via `app.Group("/prefix")`
- Support nested groups: `api.Group("/v1")`
- Middleware applied in order, wrapping the handler
- `handle()` method constructs full path and applies group middleware chain
- Middleware applied in reverse order (LIFO) when wrapping handlers

### Context Keys

Uses typed key type for request-context keys to avoid collisions:
- `contextkeys.HandlerError` (in `internal/contextkeys`): stores and retrieves handler errors across core and middleware

## Testing Patterns

Tests use `net/http/httptest` for HTTP testing:
- `httptest.NewRequest()` creates test requests
- `httptest.NewRecorder()` captures responses
- Test both success and error cases
- Test middleware behavior (logging, recovery, custom middleware)
- Test context pooling and reuse

Example test pattern:
```go
app := New()
app.GET("/test", func(ctx *Context) error {
    return ctx.JSON(200, map[string]string{"status": "ok"})
})

req := httptest.NewRequest("GET", "/test", nil)
rr := httptest.NewRecorder()
app.ServeHTTP(rr, req)

// Assert status code, body, etc.
```

## Important Implementation Details

1. **Context lifecycle**: Always call `ctx.release()` after handler completes to return to pool
2. **Error handling**: Handler errors are logged and auto-responded if response not written
3. **Middleware order**: Applied in reverse order when wrapping handlers (LIFO)
4. **Response tracking**: `written` flag prevents double writes and enables error auto-response
5. **Chi integration**: Uses `chi.URLParam()` for route parameters, embeds `chi.Mux` for routing
6. **Graceful shutdown**: `Run()` handles SIGINT/SIGTERM, uses context with timeout for shutdown
7. **Context keys**: Uses typed key values for context keys to avoid collisions (e.g., `contextkeys.HandlerError`)
8. **Error propagation**: Handler errors stored in both request context (via `contextkeys.HandlerError`) and Context struct (via `SetError()`) for middleware access

## Related Repositories

- **ares-contrib**: Extended middleware (CORS, JWT, rate limiting, gzip, secure headers)
- Middleware references in README point to `github.com/xushuhui/ares-contrib/middleware/*`
