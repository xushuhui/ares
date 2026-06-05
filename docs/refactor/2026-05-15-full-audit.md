# Ares 全量代码审计（2026-05-15）

## 审计范围

```
ares.go
context.go
server.go
errors/error.go
middleware/logger/logger.go
middleware/recovery/recovery.go
internal/contextkeys/contextkeys.go
```

---

## 问题分级

### P0：影响稳定性或行为正确性

#### P0-1 `ares.go:141` — handler 错误码丢失，所有业务错误均返回 500

**问题：**
```go
errorResponse["error"] = err.Error()
ctx.JSON(http.StatusInternalServerError, errorResponse)  // 永远 500
```
`errors.Error` 携带了语义化 HTTP 状态码（400/404/403/401），但 `wrapHandler` 完全忽略，统一返回 500。

**影响：** 调用方收到 `500 {"error":"code=400, message=invalid input"}`，状态码与消息体矛盾；RESTful 语义破坏；客户端无法区分客户端错误和服务端错误。

**建议：**
```go
// 在 wrapHandler 中替换固定 500：
code := http.StatusInternalServerError
var httpErr *errors.Error
if errors.As(err, &httpErr) {
    code = httpErr.Code
}
ctx.JSON(code, map[string]string{"error": httpErr.Message /* or err.Error() */})
```

---

#### P0-2 `context.go:302` — `MustGet` 在生产 handler 中触发 panic

**问题：**
```go
func (c *Context) MustGet(key string) any {
    if value, exists := c.store[key]; exists {
        return value
    }
    panic("key \"" + key + "\" does not exist")
}
```

**影响：** handler 调用 `MustGet` 时 key 不存在（如中间件未执行、配置遗漏），触发 panic → recovery 中间件捕获 → 500 响应，且 panic 日志不含 handler 业务上下文，难以定位。`Get()` 已提供安全访问，`MustGet` 的 panic 语义过重。

**建议：** 移除 `MustGet`，或将 panic 改为返回 `(any, bool)` 并重命名为 `GetOrPanic`（明确告知调用者风险）。框架 API 应优先防御性设计。

---

### P1：影响可维护性或隐性正确性风险

#### P1-1 `ares.go:128` — `*r = *r.WithContext(...)` 就地修改请求

**问题：**
```go
*r = *r.WithContext(context.WithValue(r.Context(), contextkeys.HandlerError, err))
```

**影响：** `r.WithContext()` 返回新的 `*http.Request`，对原指针地址做值覆盖是非常规 Go 模式。若 handler 内部启动了 goroutine 并持有原 `r` 引用，存在数据竞争。logger 中间件通过存储的同一指针读取 context 依赖此副作用，耦合隐性。

**建议：** 改为通过 `ctx.SetError(err)` 传递错误（已实现），并让 logger 读取 `ares.Context.Error()` 而非 `request.Context().Value(contextkeys.HandlerError)`。若保留双通道，加注释说明为什么需要两个途径。

---

#### P1-2 `context.go:91` — `Bind()` 无请求体大小限制

**问题：**
```go
func (c *Context) Bind(v any) error {
    return json.NewDecoder(c.Request.Body).Decode(v)
}
```

**影响：** 恶意客户端发送超大 JSON body（如 1GB），服务进程 OOM。框架不限制大小，用户需自行记得设置，易遗漏。

**建议：**
```go
const defaultMaxBodyBytes = 4 << 20 // 4MB
func (c *Context) Bind(v any) error {
    r := io.LimitReader(c.Request.Body, defaultMaxBodyBytes)
    return json.NewDecoder(r).Decode(v)
}
```
或暴露 `BindWithLimit(v any, maxBytes int64)` 允许覆盖。

---

#### P1-3 `recovery/recovery.go:89` — `w.Write()` 返回值未检查

**问题：**
```go
w.Write([]byte(`{"error":"Internal Server Error"}`))
```

**影响：** 若客户端已断开连接，写入失败静默丢失，也无日志记录。对于恢复路径（panic 之后），应至少 log 写入失败。

**建议：**
```go
if _, err := w.Write([]byte(`{"error":"Internal Server Error"}`)); err != nil {
    o.logger.Error("failed to write recovery response", "error", err)
}
```

---

#### P1-4 `middleware/logger/logger.go:83` — 实现已废弃的 `http.Pusher`

**问题：**
```go
func (rw *responseWriter) Push(target string, opts *http.PushOptions) error {
    pusher, ok := rw.ResponseWriter.(http.Pusher)
    ...
}
```

**影响：** `http.Pusher` 在 Go 1.21 中已标记为废弃（HTTP/2 server push 移除）。持续实现并代理废弃接口增加维护负担，并在 Go 未来版本可能产生编译警告。

**建议：** 删除 `Push()` 方法。

---

### P2：影响可读性或轻微设计问题

#### P2-1 `context.go:246,278` — 魔法数字 `32 << 20` 重复出现且不可配置

```go
// context.go:246
if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32MB default
// context.go:278
if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32MB default
```
提取为 package 级常量 `const defaultMaxMultipartMemory = 32 << 20`，并考虑暴露为 `Context` 配置项。

---

#### P2-2 `ares.go:141` — 原始 `err.Error()` 直接暴露给客户端

```go
errorResponse["error"] = err.Error()
```
`errors.Error.Error()` 返回 `"code=400, message=invalid input"` 这种内部格式字符串，泄漏框架内部表示。客户端收到混乱格式。改为只暴露 `Message` 字段（见 P0-1 建议）。

---

#### P2-3 `logger/logger.go:156` — 声称记录 bytes 实际未记录

CLAUDE.md 文档写 "Logs: method, path, status, duration, bytes, IP"，但 logger 实现未跟踪 `bytesWritten`。`responseWriter` 的 `Write()` 未统计写入字节数。

**建议：** 在 `responseWriter` 加 `bytesWritten int64`，在 `Write()` 中累加，并加入日志字段。或更新文档删掉 `bytes` 字段。

---

#### P2-4 `server.go:47` — `Start(ctx context.Context)` 参数未使用

```go
// The context parameter is currently not used as ListenAndServe blocks until shutdown.
func (s *httpServer) Start(ctx context.Context) error {
```
满足 `Server` 接口但参数无效用。如接口设计意图是未来使用，加 `//nolint:revive` 或用 `_` 命名。若永不使用，可收窄接口，或在 `Start` 中监听 `ctx.Done()` 作为额外关闭信号。

---

## 体量热点

| 文件 | 行数 | 是否需拆分 |
|------|------|----------|
| `context.go` | 354 | 否，职责单一 |
| `middleware/logger/logger.go` | 181 | 否 |
| `ares.go` | 297 | 否，但 Group 逻辑可提取到 `group.go` |
| `middleware/recovery/recovery.go` | 122 | 否 |
| `server.go` | 89 | 否 |
| `errors/error.go` | 45 | 否 |

整体体量小，无超长文件问题。

---

## 建议处理顺序

### 第一阶段：P0（正确性）

1. `ares.go` — `wrapHandler` 读取 `errors.Error.Code`，按语义码响应（P0-1）
2. `context.go` — 评估 `MustGet` 去留，最低要求加文档注释强调 panic 风险（P0-2）

### 第二阶段：P1（可维护性）

3. `context.go` — `Bind()` 加默认 body size limit（P1-2）
4. `recovery/recovery.go` — 检查 `w.Write()` 错误（P1-3）
5. `ares.go` — 替换 `*r = *r.WithContext(...)` 为更显式的错误传递（P1-1）
6. `logger/logger.go` — 删除 `Push()` 方法（P1-4）

### 第三阶段：P2（清理）

7. `context.go` — 提取 multipart 魔法数字为常量
8. `logger/logger.go` — 补全 `bytesWritten` 统计或更新文档
9. `ares.go` — Group 路由注册提取到独立文件 `group.go`
