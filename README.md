# Ares

[中文文档](README.zh.md)

A lightweight, high-performance Go web framework built on top of [chi](https://github.com/go-chi/chi).

## Features

- **Lightweight**: Only 1 external dependency (chi)
- **Standard Library First**: Uses Go's standard library (slog, encoding/json)
- **Simple API**: Clean and intuitive handler signature
- **High Performance**: Built on chi's fast router
- **Graceful Shutdown**: Built-in support for graceful server shutdown
- **Middleware**: Logger, Recovery, and CORS middleware included
- **Rich Ecosystem**: Extended middleware and application templates available

## Installation

```bash
go get github.com/xushuhui/ares
```

## Quick Start

### Simple Usage

```go
package main

import (
    "net/http"
    "github.com/xushuhui/ares"
)

func main() {
    // Create app with default middleware (logger + recovery)
    app := ares.Default()

    // Define routes
    app.GET("/hello", func(ctx *ares.Context) error {
        return ctx.JSON(http.StatusOK, map[string]string{
            "message": "Hello, World!",
        })
    })

    // Start server
    app.Run(":8080")
}
```

### Manual Configuration

```go
package main

import (
    "net/http"
    "log/slog"
    "os"
    "github.com/xushuhui/ares"
    "github.com/xushuhui/ares/middleware/logger"
    "github.com/xushuhui/ares/middleware/recovery"
)

func main() {
    // Create app with custom logger
    customLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    app := ares.New(ares.WithLogger(customLogger))

    // Add middleware manually for full control
    app.Use(logger.New(logger.WithLogger(app.Logger())))
    app.Use(recovery.New(recovery.WithLogger(app.Logger())))

    // Define routes
    app.GET("/hello", func(ctx *ares.Context) error {
        return ctx.JSON(http.StatusOK, map[string]string{
            "message": "Hello, World!",
        })
    })

    // Start server
    app.Run(":8080")
}
```

## Core Concepts

### Handler Signature

Ares uses a simple handler signature that returns an error for better error handling:

```go
func MyHandler(ctx *ares.Context) error {
    // Access request data
    id := ctx.Param("id")
    query := ctx.Query("q")

    // Bind JSON request body
    var req MyRequest
    if err := ctx.Bind(&req); err != nil {
        return err  // Error will be logged and handled automatically
    }

    // Send JSON response
    return ctx.JSON(200, map[string]any{
        "id":   id,
        "data": data,
    })
}
```

**Error Handling**: If a handler returns an error:
- The error is automatically logged with request details
- If no response has been sent yet, a 500 error response is returned
- This allows for clean error propagation without manual error handling in each handler

**Query Parameter Examples**:

```go
// Simple query: /search?keyword=go&page=1&enabled=true
func Search(ctx *ares.Context) error {
    keyword := ctx.Query("keyword")                    // "go"
    page := ctx.QueryInt("page", 1)                    // 1
    enabled := ctx.QueryBool("enabled", false)         // true
    price := ctx.QueryFloat("price", 0.0)              // 0.0

    // Use the values...
}

// Form data
func Login(ctx *ares.Context) error {
    username := ctx.FormValue("username")
    password := ctx.FormValue("password")
}

// File upload
func Upload(ctx *ares.Context) error {
    file, err := ctx.FormFile("file")
    if err != nil {
        return err
    }
    return ctx.SaveUploadedFile(file, "./uploads/"+file.Filename)
}
```

### Context Methods

**Request**:
- `ctx.Param(key string)` - Get URL parameter
- `ctx.Query(key string)` - Get query parameter
- `ctx.QueryDefault(key, default string)` - Get query with default value
- `ctx.QueryInt(key string, default int)` - Get query as int
- `ctx.QueryBool(key string, default bool)` - Get query as bool
- `ctx.QueryFloat(key string, default float64)` - Get query as float64
- `ctx.FormValue(key string)` - Get form value
- `ctx.Bind(v any)` - Decode JSON request body
- `ctx.Cookie(name string)` - Get cookie
- `ctx.GetHeader(key string)` - Get request header
- `ctx.FormFile(name string)` - Get uploaded file
- `ctx.SaveUploadedFile(file, dst string)` - Save uploaded file
- `ctx.MultipartForm()` - Get multipart form data

**Response**:
- `ctx.JSON(code int, v any)` - Send JSON response
- `ctx.String(code int, s string)` - Send plain text response
- `ctx.Status(code int)` - Send status code only
- `ctx.SetCookie(cookie *http.Cookie)` - Set cookie
- `ctx.SetHeader(key, value string)` - Set response header
- `ctx.Redirect(code int, url string)` - Redirect to URL
- `ctx.File(filepath string)` - Send file
- `ctx.Attachment(filepath, filename string)` - Send file as download
- `ctx.Stream(contentType string, reader io.Reader)` - Stream response

**Other**:
- `ctx.Logger()` - Get logger instance
- `ctx.Set(key string, value any)` - Store a key-value pair in context
- `ctx.Get(key string) (any, bool)` - Retrieve a value from context
- `ctx.MustGet(key string) any` - Retrieve a value (panics if not found)
- `ctx.GetString(key string) string` - Retrieve a string value
- `ctx.GetInt(key string) int` - Retrieve an int value
- `ctx.GetBool(key string) bool` - Retrieve a bool value

### Routing

```go
app := ares.New()

// HTTP methods
app.GET("/users", GetUsers)
app.POST("/users", CreateUser)
app.PUT("/users/{id}", UpdateUser)
app.DELETE("/users/{id}", DeleteUser)
app.PATCH("/users/{id}", PatchUser)

// URL parameters
app.GET("/users/{id}", func(ctx *ares.Context) {
    id := ctx.Param("id")
    // ...
})

// Route groups with middleware
api := app.Group("/api")
api.Use(authMiddleware)  // Apply middleware to all routes in group
api.GET("/users", GetUsers)
api.POST("/users", CreateUser)

// Nested groups
v1 := api.Group("/v1")
v1.GET("/status", GetStatus)

// Route groups (using chi.Router directly)
app.Route("/api/v1", func(r chi.Router) {
    r.Get("/status", statusHandler)
    r.Get("/health", healthHandler)
})

// Static files
app.Static("/static", "./public")        // Serve directory
app.StaticFile("/favicon.ico", "./favicon.ico")  // Serve single file
```

#### Route Groups

Route groups allow you to organize routes with common prefixes and middleware:

```go
// Create a group with prefix
api := app.Group("/api")

// Add middleware to the group
api.Use(authMiddleware, loggingMiddleware)

// All routes in this group will have /api prefix and middleware applied
api.GET("/users", GetUsers)       // -> /api/users
api.POST("/users", CreateUser)    // -> /api/users
api.GET("/users/{id}", GetUser)   // -> /api/users/{id}

// Nested groups
v1 := api.Group("/v1")
v1.GET("/status", GetStatus)      // -> /api/v1/status

v2 := api.Group("/v2")
v2.GET("/status", GetStatusV2)    // -> /api/v2/status
```

#### Context Key-Value Storage

Store and retrieve values in the context to pass data between middleware and handlers:

```go
// Middleware that sets a value
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Authenticate user...
        userID := 123

        // Store in context (need to wrap in Ares handler to access Context)
        next.ServeHTTP(w, r)
    })
}

// In your handler
app.GET("/profile", func(ctx *ares.Context) error {
    // Store values
    ctx.Set("user_id", 123)
    ctx.Set("username", "john")
    ctx.Set("is_admin", true)

    // Retrieve values
    userID := ctx.GetInt("user_id")           // 123
    username := ctx.GetString("username")     // "john"
    isAdmin := ctx.GetBool("is_admin")        // true

    // Generic Get with type assertion
    if val, ok := ctx.Get("user_id"); ok {
        userID := val.(int)
        // ...
    }

    // MustGet panics if key doesn't exist
    userID := ctx.MustGet("user_id").(int)

    return ctx.JSON(200, map[string]any{
        "user_id": userID,
        "username": username,
    })
})
```

### Middleware

Ares provides two types of middleware:
- **Core Middleware**: Essential middleware in `github.com/xushuhui/ares/middleware`
- **Extended Middleware**: Additional middleware in `github.com/xushuhui/ares-contrib/middleware`

#### Logger Middleware

Logs HTTP requests with method, path, status, duration, and more:

```go
import "github.com/xushuhui/ares/middleware/logger"

// Use default logger
app.Use(logger.New())

// With custom logger
app.Use(logger.New(
    logger.WithLogger(app.Logger()),
    logger.WithSkipPaths([]string{"/health", "/metrics"}),
    logger.WithCustomFields(func(r *http.Request, status int, duration time.Duration) []any {
        return []any{"user_id", r.Header.Get("X-User-ID")}
    }),
))
```

**Options:**
- `WithLogger(logger)` - Set custom slog logger (default: JSON logger to stdout)
- `WithSkipPaths(paths)` - Skip logging for specific paths
- `WithCustomFields(func)` - Add custom fields to log output

#### Recovery Middleware

Recovers from panics and returns a 500 error with JSON response:

```go
import "github.com/xushuhui/ares/middleware/recovery"

// Use default logger
app.Use(recovery.New())

// With custom options
app.Use(recovery.New(
    recovery.WithLogger(app.Logger()),
    recovery.WithStackTrace(true),
    recovery.WithRecoveryHandler(func(w http.ResponseWriter, r *http.Request, err any) {
        // Custom recovery logic
    }),
))
```

**Options:**
- `WithLogger(logger)` - Set custom slog logger (default: JSON logger to stdout)
- `WithStackTrace(bool)` - Enable/disable stack trace logging (default: true)
- `WithRecoveryHandler(func)` - Custom recovery handler

When a panic occurs:
- Logs the error with optional stack trace
- Returns `{"error":"Internal Server Error"}` with HTTP 500
- Prevents the server from crashing

#### Extended Middleware

For additional middleware like CORS, JWT, Rate Limiting, Gzip, etc., see the [ares-contrib package](https://github.com/xushuhui/ares-contrib):

```go
import (
    "github.com/xushuhui/ares-contrib/middleware/cors"
    "github.com/xushuhui/ares-contrib/middleware/jwt"
    "github.com/xushuhui/ares-contrib/middleware/ratelimiter"
    "github.com/xushuhui/ares-contrib/middleware/gzip"
)

// CORS
app.Use(cors.New(
    cors.WithAllowedOrigins([]string{"https://example.com"}),
    cors.WithAllowedMethods([]string{"GET", "POST"}),
))

// JWT Authentication
app.Use(jwt.New([]byte("secret-key"),
    jwt.WithSigningMethod(jwt.SigningMethodHS256),
))

// Rate Limiting
app.Use(ratelimiter.New(
    ratelimiter.WithRate(100),
    ratelimiter.WithBurst(200),
))

// Gzip Compression
app.Use(gzip.New(
    gzip.WithLevel(5),
))

// Secure Headers
app.Use(secure.New(
    secure.WithXFrameOptions("DENY"),
    secure.WithHSTSMaxAge(31536000),
    secure.WithContentSecurityPolicy("default-src 'self'"),
))
```

### Server Configuration

Customize HTTP server timeouts using the functional options pattern:

```go
// Use default timeouts (30s read, 30s write, 60s idle, 10s shutdown)
app.Run(":8080")

// Custom single timeout
app.Run(":8080", ares.WithReadTimeout(15*time.Second))

// Multiple custom timeouts
app.Run(":8080",
    ares.WithReadTimeout(15*time.Second),
    ares.WithWriteTimeout(15*time.Second),
    ares.WithIdleTimeout(30*time.Second),
    ares.WithShutdownTimeout(5*time.Second),
)
```

**Available options**:
- `WithReadTimeout(duration)` - Maximum duration for reading the entire request (default: 30s)
- `WithWriteTimeout(duration)` - Maximum duration before timing out writes of the response (default: 30s)
- `WithIdleTimeout(duration)` - Maximum amount of time to wait for the next request (default: 60s)
- `WithShutdownTimeout(duration)` - Maximum duration for graceful shutdown (default: 10s)


### Application Configuration

Ares supports functional options pattern for application configuration:

```go
import "log/slog"

// Create with default logger
app := ares.New()

// Create with custom logger
customLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
app := ares.New(ares.WithLogger(customLogger))

// Or use Default() with custom logger
app := ares.Default(ares.WithLogger(customLogger))
```

**Available options**:
- `WithLogger(logger)` - Set custom slog logger

**Factory Methods**:
- `ares.New(opts...)` - Create a new Ares instance without middleware
- `ares.Default()` - Create a new Ares instance with default middleware (logger + recovery)

The `Default()` method automatically adds:
- **Logger middleware**: Logs all HTTP requests with method, path, status, duration, and IP
- **Recovery middleware**: Recovers from panics and returns a 500 error response

If you need to customize the logger for a Default instance, create with `New()` and add middleware manually:

```go
customLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
app := ares.New(ares.WithLogger(customLogger))
app.Use(logger.New(logger.WithLogger(app.Logger())))
app.Use(recovery.New(recovery.WithLogger(app.Logger())))
```

### Custom Logger

```go
import "log/slog"

logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

// Option 1: Set logger after creation
app := ares.New()
app.SetLogger(logger)

// Option 2: Use WithLogger option
app := ares.New(ares.WithLogger(logger))
```

### Server Interface

Ares uses a `Server` interface for server implementations, allowing for flexibility and testability:

```go
type Server interface {
    Start(context.Context) error
    Stop(context.Context) error
}
```

The framework provides an HTTP server implementation (`httpServer`) that is used internally by the `Run` method. You can also create custom server implementations:

```go
// Create a custom HTTP server
server := ares.NewHTTPServer(":8080", app, app.Logger(),
    ares.WithReadTimeout(15*time.Second),
)

// Start server manually
go func() {
    if err := server.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
}()

// Stop server with context
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
server.Stop(ctx)
```

**For programmatic control**, use the Server interface directly:

```go
app := ares.New()

// Create server with explicit control
server := ares.NewHTTPServer(":8080", app, app.Logger())

// Start server in a goroutine
go func() {
    if err := server.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
}()

// Stop server from another goroutine or signal handler
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
server.Stop(ctx)
```

This design allows for:
- **Separation of concerns**: Ares handles routing, Server handles lifecycle
- **No circular references**: Clean dependency graph
- **Custom server implementations**: Easy to implement HTTPS, HTTP/2, gRPC
- **Better testability**: Mock servers without affecting Ares
- **Explicit control**: Clear ownership of server lifecycle


## Example

See [examples/basic/main.go](examples/basic/main.go) for a complete example.

Run the example:

```bash
cd examples/basic
go run main.go
```

Test the endpoints:

```bash
# Health check
curl http://localhost:8080/health

# Get user
curl http://localhost:8080/users/123

# Create user
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name":"John","age":30}'

# Test panic recovery
curl http://localhost:8080/panic
```

## Project Structure

```
ares/
├── ares.go              # Framework core
├── context.go           # Enhanced context
├── server.go            # Server interface and HTTP implementation
├── middleware/          # Core middleware
│   ├── logger/          # Request logging
│   │   ├── logger.go
│   │   └── logger_test.go
│   └── recovery/        # Panic recovery
│       ├── recovery.go
│       └── recovery_test.go
└── examples/            # Example applications
    └── basic/
```

**Note**: Extended middleware (CORS, JWT, Rate Limiting, etc.) is now available in the separate [ares-contrib](https://github.com/xushuhui/ares-contrib) repository.

## Design Philosophy

1. **Standard Library First**: Prefer standard library over third-party dependencies
2. **Minimal Abstraction**: Don't hide the underlying HTTP primitives
3. **No Magic**: No reflection, code generation, or hidden behavior
4. **Developer Freedom**: Don't force technology choices (databases, caching, etc.)
5. **Simplicity**: Keep the API surface small and intuitive

## Performance

Ares is designed for high performance:

**Optimizations**:
- **Context Pool**: Uses `sync.Pool` to reuse Context objects, reducing GC pressure
- **Zero Reflection**: Query parameter helpers use direct type conversion, no reflection overhead
- **Efficient Router**: Built on chi's radix tree router with O(log n) lookup
- **Minimal Allocations**: Careful memory management throughout

**Benchmarks** (approximate):
- Simple JSON response: ~65,000 req/s
- With query parsing: ~60,000 req/s
- With JSON binding: ~50,000 req/s

Performance is comparable to Gin while maintaining standard library compatibility.

## Comparison with Other Frameworks

| Feature | Ares | Gin | Echo |
|---------|------|-----|------|
| Dependencies | 1 | Many | Many |
| Standard Library | ✓ | ✗ | ✗ |
| Logger | slog | custom | custom |
| Router | chi | custom | custom |
| Learning Curve | Low | Low | Low |

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Ecosystem

The Ares ecosystem includes additional tools and resources to help you build production-ready applications:

### 📦 [Ares Contrib](https://github.com/xushuhui/ares-contrib)

Extended middleware collection for Ares providing production-ready middleware:

- **Request ID** - Unique request tracking with UUID generation
- **Secure Headers** - Security headers (CSP, HSTS, X-Frame-Options, etc.)
- **CORS** - Cross-Origin Resource Sharing with configurable options
- **JWT** - Token-based authentication with custom claims support
- **Rate Limiter** - Token bucket rate limiting per IP/key
- **GZIP** - Response compression with intelligent exclusions
- **Body Limit** - Request body size limiting to prevent DoS

**Installation:**
```bash
go get github.com/xushuhui/ares-contrib
```

**Usage:**
```go
import (
    "github.com/xushuhui/ares-contrib/middleware/cors"
    "github.com/xushuhui/ares-contrib/middleware/jwt"
    "github.com/xushuhui/ares-contrib/middleware/secure"
)

app.Use(secure.New())
app.Use(cors.New(
    cors.WithAllowedOrigins([]string{"https://example.com"}),
))
api := app.Group("/api", jwt.New([]byte("secret-key")))
```

[→ View Documentation](https://github.com/xushuhui/ares-contrib#readme)

### 🏗️ [Ares Layout](https://github.com/xushuhui/ares-layout)

Production-ready application template demonstrating best practices for structuring Ares applications:

- **Clean Architecture** - 4-layer architecture (Server → Handler → Biz → Data)
- **Type-Safe SQL** - Using sqlc for compile-time safe database queries
- **Redis Caching** - Integrated caching layer for performance
- **Configuration Management** - YAML-based configuration
- **Example CRUD** - Complete user management with MySQL
- **Docker Support** - Containerized development environment

**Features:**
- ✅ Clean Architecture pattern (inspired by go-kratos)
- ✅ RESTful API with proper HTTP semantics
- ✅ Database integration (MySQL + sqlc)
- ✅ Redis caching layer
- ✅ OpenAPI/Swagger documentation
- ✅ Production-ready error handling and logging

**Quick Start:**
```bash
git clone https://github.com/xushuhui/ares-layout.git myproject
cd myproject
docker-compose -f deploy/docker-compose.yml up -d
go run main.go
```

[→ View Repository](https://github.com/xushuhui/ares-layout#readme)

### 🛠️ [Aresctl](https://github.com/xushuhui/aresctl)

Command-line tool for Ares framework development:

- **OpenAPI Generation** - Automatically generate OpenAPI 3.0 specifications from Go code
- **Code Analysis** - Parse route definitions and API structures
- **Convention-based** - Works seamlessly with ares-layout project structure
- **Fast & Lightweight** - Minimal dependencies, quick execution

**Installation:**
```bash
go install github.com/xushuhui/aresctl@latest
```

**Usage:**
```bash
# Generate OpenAPI documentation
aresctl openapi

# View help
aresctl --help
```

**Features:**
- ✅ Automatic OpenAPI 3.0 spec generation
- ✅ Route and handler analysis
- ✅ Request/Response schema extraction
- ✅ Tag-based endpoint grouping
- 🚧 Project scaffolding (coming soon)
- 🚧 Code generation (coming soon)

[→ View Repository](https://github.com/xushuhui/aresctl#readme)

### Additional Resources

- **Examples**: Check the [examples/](examples/) directory for usage examples
- **Documentation**: Visit the [Wiki](https://github.com/xushuhui/ares/wiki) for detailed documentation
- **Issues**: Report bugs or request features on [GitHub Issues](https://github.com/xushuhui/ares/issues)

### Quick Links

| Resource | Description | Link |
|----------|-------------|------|
| **Core Framework** | Lightweight web framework | [github.com/xushuhui/ares](https://github.com/xushuhui/ares) |
| **Extended Middleware** | Production-ready middleware | [github.com/xushuhui/ares-contrib](https://github.com/xushuhui/ares-contrib) |
| **Application Template** | Project structure template | [github.com/xushuhui/ares-layout](https://github.com/xushuhui/ares-layout) |
| **CLI Tool** | Development tools | [github.com/xushuhui/aresctl](https://github.com/xushuhui/aresctl) |
| **Documentation** | Official documentation | [github.com/xushuhui/ares/wiki](https://github.com/xushuhui/ares/wiki) |

---

**Built with ❤️ by the Ares community**
