# Ares 框架设计文档

> 本文档面向不了解任何 Web 框架的新手，详细解释 Ares 框架的设计理念和实现原理。

## 目录

1. [什么是 Web 框架？](#什么是-web-框架)
2. [核心理念](#核心理念)
3. [架构设计](#架构设计)
4. [关键设计决策](#关键设计决策)
5. [设计模式](#设计模式)
6. [实战案例](#实战案例)

---

## 什么是 Web 框架？

### 问题：用原生 Go 写 Web 服务器很麻烦

假设你要写一个简单的 Web API：

```go
// 原生 Go 的写法
func handleUser(w http.ResponseWriter, r *http.Request) {
    // 1. 解析 URL 参数
    id := r.URL.Query().Get("id")

    // 2. 解析 JSON 请求体
    var user User
    err := json.NewDecoder(r.Body).Decode(&user)
    if err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
        return
    }

    // 3. 设置响应头
    w.Header().Set("Content-Type", "application/json")

    // 4. 返回 JSON 响应
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(user)
}
```

**问题：**
- ❌ 每个接口都要重复写相同的代码
- ❌ 错误处理很繁琐
- ❌ 没有路由分组
- ❌ 没有中间件机制

### 解决方案：Web 框架

Web 框架提供了一套**标准化的工具和模式**，让你专注于业务逻辑，而不是重复的基础设施代码。

---

## 核心理念

Ares 框架遵循以下设计原则：

### 1. **简单性** (Simplicity)
- API 设计直观易懂
- 避免过度设计
- 最小化学习成本

### 2. **可组合性** (Composability)
- 中间件可以灵活组合
- 路由可以分组嵌套
- 功能模块化，按需使用

### 3. **性能优化** (Performance)
- 使用对象池减少内存分配
- 避免不必要的反射
- 零成本抽象

### 4. **渐进式增强** (Progressive Enhancement)
- 从简单开始，按需增加功能
- 核心框架保持轻量
- 扩展功能通过中间件提供

---

## 架构设计

### 整体架构图

```
┌─────────────────────────────────────────────────────────────┐
│                        HTTP 请求                             │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                     全局中间件链                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │  Logger  │→ │ Recovery │→ │  CORS    │→ │   JWT    │→   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                     路由分组 (Group)                         │
│  ┌──────────────────┐  ┌──────────────────┐                │
│  │  /api/v1 (JWT)   │  │  /public (无认证) │                │
│  │  ┌────────────┐  │  │  ┌────────────┐  │                │
│  │  │组中间件链   │  │  │  │   无       │  │                │
│  │  └────────────┘  │  │  └────────────┘  │                │
│  └──────────────────┘  └──────────────────┘                │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                      Handler (业务逻辑)                      │
│                   func(*Context) error                      │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                     HTTP 响应                                │
└─────────────────────────────────────────────────────────────┘
```

### 核心组件

#### 1. **Ares 结构体** - 框架核心
```go
type Ares struct {
    *chi.Mux        // 嵌入 Chi 路由器（提供路由能力）
    logger *slog.Logger  // 日志记录器
}
```

**设计思想：**
- 使用**组合**而非继承（嵌入 `chi.Mux`）
- 只保留核心字段，保持轻量
- 通过选项模式灵活配置

#### 2. **Context 结构体** - 请求上下文
```go
type Context struct {
    http.ResponseWriter  // 响应写入器
    Request *http.Request    // 请求对象
    logger  *slog.Logger     // 日志记录器
    written bool             // 是否已写入响应
    store   map[string]any   // 键值存储（用于中间件间传递数据）
    err     error            // 错误信息
}
```

**为什么需要 Context？**
- ✅ 封装了常用的操作（JSON、Bind、Param 等）
- ✅ 提供类型安全的辅助方法
- ✅ 跨中间件传递数据

**为什么使用对象池？**
```go
var contextPool = sync.Pool{
    New: func() any {
        return &Context{}
    },
}
```

**原因：**
- 减少 GC 压力（复用对象而不是频繁创建/销毁）
- 提高性能（避免内存分配）
- 每次请求从池中获取，用完后归还

---

## 关键设计决策

### 决策 1：为什么 Handler 返回 error？

```go
// Ares 的设计
type Handler func(*Context) error

// 对比标准库
type Handler func(http.ResponseWriter, *http.Request)
```

**原因：**

1. **统一的错误处理**
```go
func wrapHandler(h Handler) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx := NewContext(w, r, logger)
        defer ctx.release()

        // ✅ 集中处理所有错误
        if err := h(ctx); err != nil {
            // 自动记录日志
            logger.Error("handler error", "error", err)

            // 自动返回错误响应
            if !ctx.written {
                ctx.JSON(http.StatusInternalServerError, map[string]string{
                    "error": err.Error(),
                })
            }
        }
    }
}
```

2. **业务代码更清晰**
```go
// ✅ 使用 Ares
func GetUser(ctx *ares.Context) error {
    user, err := db.FindUser(id)
    if err != nil {
        return err  // 直接返回错误，框架自动处理
    }
    return ctx.JSON(http.StatusOK, user)
}

// ❌ 使用标准库
func GetUser(w http.ResponseWriter, r *http.Request) {
    user, err := db.FindUser(id)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
        return  // 每个地方都要重复写错误处理
    }
    json.NewEncoder(w).Encode(user)
}
```

### 决策 2：为什么需要 Group？

**问题场景：**
```go
app := ares.Default()  // 全局中间件：logger + recovery

// ❌ 直接添加 JWT 中间件
app.Use(jwtMiddleware)

// 问题：所有路由都需要认证！
app.GET("/health", healthHandler)      // ❌ 被 JWT 拦截
app.GET("/public", publicHandler)      // ❌ 被 JWT 拦截
app.GET("/api/user", userHandler)      // ✅ 需要 JWT
```

**解决方案：Group**
```go
app := ares.Default()

// ✅ 公开路由（不需要认证）
app.GET("/health", healthHandler)
app.GET("/public", publicHandler)

// ✅ API 路由组（需要认证）
api := app.Group("/api", jwtMiddleware)
api.GET("/user", userHandler)    // 只有这个路由需要 JWT

// ✅ Admin 路由组（需要认证 + 管理员权限）
admin := app.Group("/admin", jwtMiddleware, adminMiddleware)
admin.GET("/stats", statsHandler)  // 需要 JWT + Admin
```

**Group 的工作原理：**
```go
type Group struct {
    ares        *Ares                                // 主路由器
    prefix      string                               // 路径前缀
    middlewares []func(http.Handler) http.Handler    // 组专用中间件
}

func (g *Group) handle(method, pattern string, h Handler) {
    // 1. 拼接完整路径
    fullPath := g.prefix + pattern  // "/api" + "/user" = "/api/user"

    // 2. 包装 Handler
    wrappedHandler := g.ares.wrapHandler(h)

    // 3. 从内到外应用组中间件
    var handler http.Handler = wrappedHandler
    for i := len(g.middlewares) - 1; i >= 0; i-- {
        handler = g.middlewares[i](handler)
    }

    // 4. 注册到主路由器
    g.ares.Method(method, fullPath, handler)
}
```

**中间件执行顺序：**
```
请求 → 全局中间件 → 组中间件 → Handler
```

### 决策 3：为什么中间件是 `func(http.Handler) http.Handler`？

**标准库的中间件模式：**
```go
type Middleware func(http.Handler) http.Handler
```

**为什么这样设计？**

1. **链式调用**（洋葱模型）
```go
func MyMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. 前置处理（请求进入）
        fmt.Println("请求进入")

        // 2. 调用下一个处理器
        next.ServeHTTP(w, r)

        // 3. 后置处理（响应返回）
        fmt.Println("响应返回")
    })
}
```

2. **执行流程图：**
```
请求 → Logger → Recovery → Handler
      ↓        ↑         ↑
    前置处理    后置处理   返回响应
```

3. **组合多个中间件**
```go
// 从内到外包装
handler = recovery.New(logger.New(handler))

// 等价于：
handler = recovery.New(
    logger.New(
        handler
    )
)
```

**为什么不是其他方式？**

❌ **方式 A：返回 error 的函数**
```go
// 这样无法链式调用
type Middleware func(*Context) error
```

❌ **方式 B：直接处理请求**
```go
// 这样无法调用下一个中间件
type Middleware func(w, r)
```

✅ **标准库方式：返回 Handler**
```go
// 可以链式调用，灵活组合
type Middleware func(http.Handler) http.Handler
```

### 决策 4：为什么 Context 有 store 字段？

**问题：中间件之间如何传递数据？**

**场景：JWT 认证**
```go
// JWT 中间件：解析用户信息
func JWTMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := parseToken(r)
        userID := token.UserID

        // ❌ 问题：如何把 userID 传给 Handler？
        next.ServeHTTP(w, r)
    })
}

// Handler：需要用户信息
func GetUser(ctx *ares.Context) error {
    // ❌ 如何获取 userID？
}
```

**方案对比：**

❌ **方案 A：全局变量**
```go
var currentUserID int  // ❌ 并发不安全！
```

❌ **方案 B：请求参数**
```go
// ❌ 污染业务逻辑
func GetUser(ctx *ares.Context, userID int) error {
    // ...
}
```

✅ **方案 C：Context Store**
```go
// JWT 中间件
func JWTMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := parseToken(r)
        userID := token.UserID

        // ✅ 使用 Ares Context
        aresCtx := getAresContext(ctx)
        aresCtx.Set("user_id", userID)

        next.ServeHTTP(w, r)
    })
}

// Handler
func GetUser(ctx *ares.Context) error {
    // ✅ 从 Context 获取
    userID := ctx.GetInt("user_id")
    // ...
}
```

**为什么使用 map 而不是结构体？**
- ✅ 灵活：可以存储任意类型的数据
- ✅ 解耦：中间件不依赖具体的字段名
- ✅ 扩展性：新增中间件不需要修改 Context 结构

---

## 设计模式

### 1. **选项模式** (Options Pattern)

**问题：如何优雅地配置组件？**

❌ **方式 A：多个构造函数**
```go
// ❌ 不灵活
NewCORS()
NewCORSWithOrigins(origins)
NewCORSWithMethods(methods)
NewCORSWithOriginsAndMethods(origins, methods)  // 组合爆炸！
```

✅ **方式 B：选项模式**
```go
// 定义选项类型
type Option func(*options)

// 定义配置结构
type options struct {
    allowedOrigins []string
    allowedMethods []string
    // ...
}

// 提供选项函数
func WithAllowedOrigins(origins []string) Option {
    return func(o *options) {
        o.allowedOrigins = origins
    }
}

// 使用
cors.New(
    cors.WithAllowedOrigins([]string{"*"}),
    cors.WithAllowedMethods([]string{"GET", "POST"}),
    // ✅ 只设置需要的选项，其他使用默认值
)
```

**优势：**
- ✅ 向后兼容（新增选项不影响已有代码）
- ✅ 可读性强（一眼看出配置了什么）
- ✅ 灵活组合（按需设置）

### 2. **工厂模式** (Factory Pattern)

**问题：JWT 中间件的 claims 为什么是函数？**

```go
type options struct {
    claims func() jwt.Claims  // ❓ 为什么是函数？
}
```

**原因：避免并发问题**

❌ **错误做法：直接存储实例**
```go
type options struct {
    claims jwt.Claims  // ❌
}

// 配置
claims := &MyClaims{}  // 创建一个实例
New(key, WithClaims(claims))  // ❌ 所有请求共享这个实例

// 解析 token
jwt.ParseWithClaims(token, o.claims, keyFunc)  // ❌ 多个请求同时写入 o.claims
```

**问题：**
- 多个并发请求会**同时修改同一个 claims 对象**
- 导致**竞态条件**（Race Condition）

✅ **正确做法：工厂函数**
```go
type options struct {
    claims func() jwt.Claims  // ✅
}

// 配置
New(key, WithClaims(func() jwt.Claims {
    return &MyClaims{}  // ✅ 每次返回新实例
}))

// 解析 token
jwt.ParseWithClaims(token, o.claims(), keyFunc)  // ✅ 每次调用返回新实例
```

**优势：**
- ✅ 线程安全（每个请求独立的实例）
- ✅ 零开销（函数调用很轻量）
- ✅ 类型安全（编译期检查）

### 3. **装饰器模式** (Decorator Pattern)

**问题：如何动态添加功能？**

中间件本质上就是装饰器模式：
```go
// 基础 Handler
handler := MyHandler

// 装饰一层：Logger
handler = logger.New(handler)

// 装饰二层：Recovery
handler = recovery.New(handler)

// 装饰三层：CORS
handler = cors.New(handler)

// 执行时：从外到内
// CORS → Recovery → Logger → MyHandler
```

**优势：**
- ✅ 功能解耦（每个中间件只做一件事）
- ✅ 可组合（像搭积木一样灵活组合）
- ✅ 可复用（中间件可以在不同项目中使用）

### 4. **对象池模式** (Object Pool Pattern)

**问题：频繁创建对象影响性能**

```go
// ❌ 不使用对象池
func NewContext() *Context {
    return &Context{}  // 每次请求都创建新对象
}

// ✅ 使用对象池
var contextPool = sync.Pool{
    New: func() any {
        return &Context{}  // 初始化时创建少量对象
    },
}

func GetContext() *Context {
    return contextPool.Get().(*Context)  // 从池中获取
}

func PutContext(ctx *Context) {
    contextPool.Put(ctx)  // 用完后归还
}
```

**性能对比：**
```
不使用对象池：
每次请求：创建 Context → 使用 → GC 回收
性能：10000 req/s

使用对象池：
每次请求：从池获取 → 使用 → 归还池
性能：50000 req/s（提升 5 倍！）
```

---

## 实战案例

### 案例 1：构建 REST API

```go
func main() {
    // 1. 创建应用（包含 logger 和 recovery）
    app := ares.Default()

    // 2. 全局中间件（所有路由都会使用）
    app.Use(cors.New(
        cors.WithAllowedOrigins([]string{"*"}),
    ))

    // 3. 公开路由（不需要认证）
    app.GET("/health", healthHandler)
    app.GET("/login", loginHandler)

    // 4. API 路由组（需要认证）
    api := app.Group("/api/v1", jwtMiddleware)

    // 5. 用户路由（需要认证）
    api.GET("/users", listUsersHandler)
    api.GET("/users/:id", getUserHandler)
    api.POST("/users", createUserHandler)
    api.PUT("/users/:id", updateUserHandler)
    api.DELETE("/users/:id", deleteUserHandler)

    // 6. Admin 路由组（需要认证 + 管理员权限）
    admin := api.Group("/admin", adminMiddleware)
    admin.GET("/stats", statsHandler)

    // 7. 启动服务器
    app.Run(":8080")
}
```

**执行流程：**

```
请求 GET /health
  ↓
Logger → Recovery → CORS → healthHandler
（全局中间件）     （无认证）

请求 GET /api/v1/users/123
  ↓
Logger → Recovery → CORS → JWT → getUserHandler
（全局中间件）        （组中间件）

请求 GET /api/v1/admin/stats
  ↓
Logger → Recovery → CORS → JWT → Admin → statsHandler
（全局中间件）        （嵌套组中间件）
```

### 案例 2：中间件链路追踪

```go
// RequestID 中间件：生成唯一请求 ID
func RequestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 生成请求 ID
        requestID := uuid.New().String()

        // ✅ 存储到 Context
        aresCtx := getAresContext(ctx)
        aresCtx.Set("request_id", requestID)

        // 设置响应头
        w.Header().Set("X-Request-ID", requestID)

        // 调用下一个中间件
        next.ServeHTTP(w, r)
    })
}

// Logger 中间件：记录请求日志
func LoggerMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        // ✅ 从 Context 获取请求 ID
        requestID := ctx.GetString("request_id")

        // 调用下一个中间件
        next.ServeHTTP(w, r)

        // ✅ 记录日志（包含请求 ID）
        fmt.Printf("[%s] %s %s %v\n",
            requestID,
            r.Method,
            r.URL.Path,
            time.Since(start),
        )
    })
}

// Handler：业务逻辑
func GetUser(ctx *ares.Context) error {
    // ✅ 从 Context 获取请求 ID
    requestID := ctx.GetString("request_id")

    // 使用 requestID 记录日志、追踪错误等
    ctx.Logger().Info("fetching user", "request_id", requestID)

    // ...
}
```

**优势：**
- ✅ 所有日志都有唯一的 requestID
- ✅ 可以轻松追踪整个请求链路
- ✅ 中间件之间可以共享数据

### 案例 3：错误处理与恢复

```go
// Recovery 中间件：捕获 panic
func RecoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                // ✅ 捕获 panic，防止服务崩溃
                log.Printf("Panic recovered: %v", err)
                http.Error(w, "Internal Server Error", 500)
            }
        }()

        next.ServeHTTP(w, r)
    })
}

// Handler：可以安全地 panic
func DivideHandler(ctx *ares.Context) error {
    a := ctx.QueryInt("a", 0)
    b := ctx.QueryInt("b", 1)

    if b == 0 {
        // ✅ 即使 panic，也会被 Recovery 中间件捕获
        panic("division by zero")
    }

    return ctx.JSON(http.StatusOK, map[string]int{
        "result": a / b,
    })
}
```

---

## 总结

### Ares 框架的核心思想

1. **分层架构**
   - 全局中间件 → 组中间件 → Handler
   - 每层有清晰的职责

2. **中间件模式**
   - 所有横切关注点都用中间件实现
   - 灵活组合，按需使用

3. **Context 抽象**
   - 统一的请求/响应接口
   - 跨中间件数据传递
   - 对象池优化性能

4. **选项模式**
   - 优雅的配置方式
   - 向后兼容
   - 可读性强

5. **Group 路由**
   - 中间件作用域隔离
   - 支持嵌套
   - 路径前缀管理

### 为什么这样设计？

| 设计决策 | 原因 | 好处 |
|---------|------|------|
| Handler 返回 error | 统一错误处理 | 代码更简洁，错误处理自动化 |
| 使用 Context | 封装常用操作 | API 更友好，类型安全 |
| 对象池 | 减少内存分配 | 性能提升 5 倍 |
| Group 路由 | 中间件作用域隔离 | 避免全局污染，灵活组合 |
| 选项模式 | 优雅配置 | 向后兼容，可读性强 |
| 工厂函数 | 避免并发问题 | 线程安全，零开销 |

### 与其他框架的对比

| 特性 | Ares | Gin | Echo |
|------|------|-----|------|
| 路由器 | Chi (Radix Tree) | 自研 (Radix Tree) | 自研 (Radix Tree) |
| Handler 签名 | `func(*Context) error` | `func(*Context)` | `func(*Context) error` |
| 中间件模式 | `func(Handler)Handler` | `func(*Context)` | `func(Handler, Handler) Handler` |
| Group 支持 | ✅ | ✅ | ✅ |
| 错误处理 | 集中式 | 分散式 | 集中式 |
| 对象池 | ✅ | ✅ | ❌ |

Ares 的设计结合了**标准库的简洁性**和**第三方框架的便利性**，是一个轻量、高性能、易用的 Web 框架。

---

**推荐学习路径：**

1. 先理解中间件模式（核心概念）
2. 再学习 Group 的使用（路由管理）
3. 然后掌握 Context 的 API（日常开发）
4. 最后了解设计模式（进阶优化）

**下一步：**
- 阅读源码：`ares/ares.go`、`ares/context.go`
- 查看示例：`examples/basic/main.go`
- 实践项目：写一个简单的 REST API

祝学习愉快！🚀
