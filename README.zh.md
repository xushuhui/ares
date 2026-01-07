# Ares

[English](README.md)

一个基于 [chi](https://github.com/go-chi/chi) 构建的轻量级、高性能 Go Web 框架。

## 特性

- **轻量级**：仅有 1 个外部依赖（chi）
- **标准库优先**：使用 Go 标准库（slog、encoding/json）
- **简洁的 API**：清晰直观的处理器签名
- **高性能**：基于 chi 的快速路由器构建
- **优雅关闭**：内置优雅关闭服务器支持
- **中间件**：包含日志、恢复和 CORS 中间件

## 安装

```bash
go get github.com/xushuhui/ares
```

## 快速开始

### 简单使用

```go
package main

import (
    "net/http"
    "github.com/xushuhui/ares"
)

func main() {
    // 创建带默认中间件的应用（logger + recovery）
    app := ares.Default()

    // 定义路由
    app.GET("/hello", func(ctx *ares.Context) error {
        return ctx.JSON(http.StatusOK, map[string]string{
            "message": "Hello, World!",
        })
    })

    // 启动服务器
    app.Run(":8080")
}
```

### 手动配置

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
    // 创建带自定义日志的应用
    customLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    app := ares.New(ares.WithLogger(customLogger))

    // 手动添加中间件以获得完全控制
    app.Use(logger.New(logger.WithLogger(app.Logger())))
    app.Use(recovery.New(recovery.WithLogger(app.Logger())))

    // 定义路由
    app.GET("/hello", func(ctx *ares.Context) error {
        return ctx.JSON(http.StatusOK, map[string]string{
            "message": "Hello, World!",
        })
    })

    // 启动服务器
    app.Run(":8080")
}
```

## 核心概念

### 处理器签名

Ares 使用简单的处理器签名，返回 error 以便更好地处理错误：

```go
func MyHandler(ctx *ares.Context) error {
    // 访问请求数据
    id := ctx.Param("id")
    query := ctx.Query("q")

    // 绑定 JSON 请求体
    var req MyRequest
    if err := ctx.Bind(&req); err != nil {
        return err  // 错误将被自动记录和处理
    }

    // 发送 JSON 响应
    return ctx.JSON(200, map[string]any{
        "id":   id,
        "data": data,
    })
}
```

**错误处理**：如果处理器返回错误：
- 错误会自动记录请求详情
- 如果响应尚未发送，将返回 500 错误响应
- 这允许清晰的错误传播，无需在每个处理器中手动处理错误

**查询参数示例**：

```go
// 简单查询：/search?keyword=go&page=1&enabled=true
func Search(ctx *ares.Context) error {
    keyword := ctx.Query("keyword")                    // "go"
    page := ctx.QueryInt("page", 1)                    // 1
    enabled := ctx.QueryBool("enabled", false)         // true
    price := ctx.QueryFloat("price", 0.0)              // 0.0

    // 使用这些值...
}

// 表单数据
func Login(ctx *ares.Context) error {
    username := ctx.FormValue("username")
    password := ctx.FormValue("password")
}

// 文件上传
func Upload(ctx *ares.Context) error {
    file, err := ctx.FormFile("file")
    if err != nil {
        return err
    }
    return ctx.SaveUploadedFile(file, "./uploads/"+file.Filename)
}
```

### Context 方法

**请求**：
- `ctx.Param(key string)` - 获取 URL 参数
- `ctx.Query(key string)` - 获取查询参数
- `ctx.QueryDefault(key, default string)` - 获取查询参数，带默认值
- `ctx.QueryInt(key string, default int)` - 获取查询参数为 int 类型
- `ctx.QueryBool(key string, default bool)` - 获取查询参数为 bool 类型
- `ctx.QueryFloat(key string, default float64)` - 获取查询参数为 float64 类型
- `ctx.FormValue(key string)` - 获取表单值
- `ctx.Bind(v any)` - 解码 JSON 请求体
- `ctx.Cookie(name string)` - 获取 cookie
- `ctx.GetHeader(key string)` - 获取请求头
- `ctx.FormFile(name string)` - 获取上传的文件
- `ctx.SaveUploadedFile(file, dst string)` - 保存上传的文件
- `ctx.MultipartForm()` - 获取 multipart 表单数据

**响应**：
- `ctx.JSON(code int, v any)` - 发送 JSON 响应
- `ctx.String(code int, s string)` - 发送纯文本响应
- `ctx.Status(code int)` - 仅发送状态码
- `ctx.SetCookie(cookie *http.Cookie)` - 设置 cookie
- `ctx.SetHeader(key, value string)` - 设置响应头
- `ctx.Redirect(code int, url string)` - 重定向到 URL
- `ctx.File(filepath string)` - 发送文件
- `ctx.Attachment(filepath, filename string)` - 发送文件作为下载
- `ctx.Stream(contentType string, reader io.Reader)` - 流式响应

**其他**：
- `ctx.Logger()` - 获取日志实例
- `ctx.Set(key string, value any)` - 在上下文中存储键值对
- `ctx.Get(key string) (any, bool)` - 从上下文中获取值
- `ctx.MustGet(key string) any` - 获取值（如果不存在则 panic）
- `ctx.GetString(key string) string` - 获取字符串值
- `ctx.GetInt(key string) int` - 获取整数值
- `ctx.GetBool(key string) bool` - 获取布尔值

### 路由

```go
app := ares.New()

// HTTP 方法
app.GET("/users", GetUsers)
app.POST("/users", CreateUser)
app.PUT("/users/{id}", UpdateUser)
app.DELETE("/users/{id}", DeleteUser)
app.PATCH("/users/{id}", PatchUser)

// URL 参数
app.GET("/users/{id}", func(ctx *ares.Context) {
    id := ctx.Param("id")
    // ...
})

// 路由组（带中间件）
api := app.Group("/api")
api.Use(authMiddleware)  // 对组内所有路由应用中间件
api.GET("/users", GetUsers)
api.POST("/users", CreateUser)

// 嵌套路由组
v1 := api.Group("/v1")
v1.GET("/status", GetStatus)

// 路由组（使用 chi.Router）
app.Route("/api/v1", func(r chi.Router) {
    r.Get("/status", statusHandler)
    r.Get("/health", healthHandler)
})

// 静态文件
app.Static("/static", "./public")        // 提供目录服务
app.StaticFile("/favicon.ico", "./favicon.ico")  // 提供单个文件
```

#### 路由组

路由组允许你使用公共前缀和中间件组织路由：

```go
// 创建带前缀的路由组
api := app.Group("/api")

// 为路由组添加中间件
api.Use(authMiddleware, loggingMiddleware)

// 该组中的所有路由都将具有 /api 前缀并应用中间件
api.GET("/users", GetUsers)       // -> /api/users
api.POST("/users", CreateUser)    // -> /api/users
api.GET("/users/{id}", GetUser)   // -> /api/users/{id}

// 嵌套路由组
v1 := api.Group("/v1")
v1.GET("/status", GetStatus)      // -> /api/v1/status

v2 := api.Group("/v2")
v2.GET("/status", GetStatusV2)    // -> /api/v2/status
```

#### Context 键值存储

在上下文中存储和检索值，以在中间件和处理器之间传递数据：

```go
// 设置值的中间件
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 认证用户...
        userID := 123

        // 存储在上下文中（需要在 Ares 处理器中包装以访问 Context）
        next.ServeHTTP(w, r)
    })
}

// 在处理器中
app.GET("/profile", func(ctx *ares.Context) error {
    // 存储值
    ctx.Set("user_id", 123)
    ctx.Set("username", "john")
    ctx.Set("is_admin", true)

    // 检索值
    userID := ctx.GetInt("user_id")           // 123
    username := ctx.GetString("username")     // "john"
    isAdmin := ctx.GetBool("is_admin")        // true

    // 使用类型断言的通用 Get
    if val, ok := ctx.Get("user_id"); ok {
        userID := val.(int)
        // ...
    }

    // MustGet 在键不存在时会 panic
    userID := ctx.MustGet("user_id").(int)

    return ctx.JSON(200, map[string]any{
        "user_id": userID,
        "username": username,
    })
})
```

### 中间件

Ares 提供两种类型的中间件：
- **核心中间件**：`github.com/xushuhui/ares/middleware` 中的基础中间件
- **扩展中间件**：`github.com/xushuhui/ares-contrib/middleware` 中的额外中间件

#### 日志中间件

记录 HTTP 请求的方法、路径、状态、持续时间等：

```go
import "github.com/xushuhui/ares/middleware/logger"

// 使用默认日志
app.Use(logger.New())

// 使用自定义日志
app.Use(logger.New(
    logger.WithLogger(app.Logger()),
    logger.WithSkipPaths([]string{"/health", "/metrics"}),
    logger.WithCustomFields(func(r *http.Request, status int, duration time.Duration) []any {
        return []any{"user_id", r.Header.Get("X-User-ID")}
    }),
))
```

**选项：**
- `WithLogger(logger)` - 设置自定义 slog 日志（默认：JSON 日志输出到 stdout）
- `WithSkipPaths(paths)` - 跳过特定路径的日志
- `WithCustomFields(func)` - 添加自定义字段到日志输出

#### 恢复中间件

从 panic 中恢复并返回 500 错误和 JSON 响应：

```go
import "github.com/xushuhui/ares/middleware/recovery"

// 使用默认日志
app.Use(recovery.New())

// 使用自定义选项
app.Use(recovery.New(
    recovery.WithLogger(app.Logger()),
    recovery.WithStackTrace(true),
    recovery.WithRecoveryHandler(func(w http.ResponseWriter, r *http.Request, err any) {
        // 自定义恢复逻辑
    }),
))
```

**选项：**
- `WithLogger(logger)` - 设置自定义 slog 日志（默认：JSON 日志输出到 stdout）
- `WithStackTrace(bool)` - 启用/禁用堆栈跟踪日志（默认：true）
- `WithRecoveryHandler(func)` - 自定义恢复处理器

当发生 panic 时：
- 记录错误和可选的堆栈跟踪
- 返回 `{"error":"Internal Server Error"}` 和 HTTP 500
- 防止服务器崩溃

#### 扩展中间件

有关 CORS、JWT、限流、Gzip 等额外中间件，请参阅 [ares-contrib 包](https://github.com/xushuhui/ares-contrib)：

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

// JWT 认证
app.Use(jwt.New([]byte("secret-key"),
    jwt.WithSigningMethod(jwt.SigningMethodHS256),
))

// 限流
app.Use(ratelimiter.New(
    ratelimiter.WithRate(100),
    ratelimiter.WithBurst(200),
))

// Gzip 压缩
app.Use(gzip.New(
    gzip.WithLevel(5),
))

// 安全头
app.Use(secure.New(
    secure.WithXFrameOptions("DENY"),
    secure.WithHSTSMaxAge(31536000),
    secure.WithContentSecurityPolicy("default-src 'self'"),
))
```

### 服务器配置

使用函数选项模式自定义 HTTP 服务器超时：

```go
// 使用默认超时（30s 读取，30s 写入，60s 空闲，10s 关闭）
app.Run(":8080")

// 自定义单个超时
app.Run(":8080", ares.WithReadTimeout(15*time.Second))

// 多个自定义超时
app.Run(":8080",
    ares.WithReadTimeout(15*time.Second),
    ares.WithWriteTimeout(15*time.Second),
    ares.WithIdleTimeout(30*time.Second),
    ares.WithShutdownTimeout(5*time.Second),
)
```

**可用选项**：
- `WithReadTimeout(duration)` - 读取整个请求的最大持续时间（默认：30s）
- `WithWriteTimeout(duration)` - 写入响应超时前的最大持续时间（默认：30s）
- `WithIdleTimeout(duration)` - 等待下一个请求的最大时间（默认：60s）
- `WithShutdownTimeout(duration)` - 优雅关闭的最大持续时间（默认：10s）


### 应用配置

Ares 支持函数选项模式进行应用配置：

```go
import "log/slog"

// 使用默认日志创建
app := ares.New()

// 使用自定义日志创建
customLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
app := ares.New(ares.WithLogger(customLogger))

// 或使用 Default() 配合自定义日志
app := ares.Default(ares.WithLogger(customLogger))
```

**可用选项**：
- `WithLogger(logger)` - 设置自定义 slog 日志

**工厂方法**：
- `ares.New(opts...)` - 创建不带中间件的新 Ares 实例
- `ares.Default()` - 创建带默认中间件（logger + recovery）的新 Ares 实例

`Default()` 方法自动添加：
- **Logger 中间件**：记录所有 HTTP 请求的方法、路径、状态、持续时间和 IP
- **Recovery 中间件**：从 panic 中恢复并返回 500 错误响应

如果需要为 Default 实例自定义日志，请使用 `New()` 创建并手动添加中间件：

```go
customLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
app := ares.New(ares.WithLogger(customLogger))
app.Use(logger.New(logger.WithLogger(app.Logger())))
app.Use(recovery.New(recovery.WithLogger(app.Logger())))
```

### 自定义日志

```go
import "log/slog"

logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

// 方式 1：创建后设置日志
app := ares.New()
app.SetLogger(logger)

// 方式 2：使用 WithLogger 选项
app := ares.New(ares.WithLogger(logger))
```

### 服务器接口

Ares 使用 `Server` 接口来实现服务器，允许灵活性和可测试性：

```go
type Server interface {
    Start(context.Context) error
    Stop(context.Context) error
}
```

框架提供了一个 HTTP 服务器实现（`httpServer`），由 `Run` 方法内部使用。您也可以创建自定义服务器实现：

```go
// 创建自定义 HTTP 服务器
server := ares.NewHTTPServer(":8080", app, app.Logger(),
    ares.WithReadTimeout(15*time.Second),
)

// 手动启动服务器
go func() {
    if err := server.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
}()

// 使用上下文停止服务器
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
server.Stop(ctx)
```

**对于程序化控制**，直接使用 Server 接口：

```go
app := ares.New()

// 创建具有显式控制的服务器
server := ares.NewHTTPServer(":8080", app, app.Logger())

// 在 goroutine 中启动服务器
go func() {
    if err := server.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
}()

// 从另一个 goroutine 或信号处理器停止服务器
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
server.Stop(ctx)
```

这种设计允许：
- **关注点分离**：Ares 处理路由，Server 处理生命周期
- **无循环引用**：清晰的依赖关系图
- **自定义服务器实现**：易于实现 HTTPS、HTTP/2、gRPC
- **更好的可测试性**：模拟服务器而不影响 Ares
- **显式控制**：清晰的服务器生命周期所有权


## 示例

查看 [examples/basic/main.go](examples/basic/main.go) 获取完整示例。

运行示例：

```bash
cd examples/basic
go run main.go
```

测试端点：

```bash
# 健康检查
curl http://localhost:8080/health

# 获取用户
curl http://localhost:8080/users/123

# 创建用户
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name":"John","age":30}'

# 测试 panic 恢复
curl http://localhost:8080/panic
```

## 项目结构

```
ares/
├── ares.go              # 框架核心
├── context.go           # 增强的上下文
├── server.go            # 服务器接口和 HTTP 实现
├── middleware/          # 核心中间件
│   ├── logger/          # 请求日志
│   │   ├── logger.go
│   │   └── logger_test.go
│   └── recovery/        # Panic 恢复
│       ├── recovery.go
│       └── recovery_test.go
└── examples/            # 示例应用
    └── basic/
```

**注意**：扩展中间件（CORS、JWT、限流等）现在在独立的 [ares-contrib](https://github.com/xushuhui/ares-contrib) 仓库中提供。

## 设计理念

1. **标准库优先**：优先使用标准库而非第三方依赖
2. **最小抽象**：不隐藏底层 HTTP 原语
3. **无魔法**：无反射、代码生成或隐藏行为
4. **开发者自由**：不强制技术选择（数据库、缓存等）
5. **简洁性**：保持 API 表面小而直观

## 性能

Ares 专为高性能设计：

**优化**：
- **Context 池**：使用 `sync.Pool` 重用 Context 对象，减少 GC 压力
- **零反射**：查询参数辅助函数使用直接类型转换，无反射开销
- **高效路由器**：基于 chi 的基数树路由器，O(log n) 查找
- **最小分配**：全程精心管理内存

**基准测试**（近似值）：
- 简单 JSON 响应：~65,000 req/s
- 带查询解析：~60,000 req/s
- 带 JSON 绑定：~50,000 req/s

性能与 Gin 相当，同时保持标准库兼容性。

## 与其他框架的比较

| 特性 | Ares | Gin | Echo |
|---------|------|-----|------|
| 依赖 | 1 | 多个 | 多个 |
| 标准库 | ✓ | ✗ | ✗ |
| 日志 | slog | 自定义 | 自定义 |
| 路由器 | chi | 自定义 | 自定义 |
| 学习曲线 | 低 | 低 | 低 |

## 许可证

MIT

## 贡献

欢迎贡献！请随时提交 Pull Request。
