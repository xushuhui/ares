# Ares Framework — Refactoring Analysis

## Summary

Overall quality is good: clean interfaces, minimal dependencies, consistent patterns. Four issues need fixing before the codebase is production-safe. The rest are improvements.

---

## Critical

### 1. `maxPooledStoreEntries` undefined — tests won't compile

**File:** `context_test.go:187`

```go
for i := 0; i <= maxPooledStoreEntries; i++ {
```

`maxPooledStoreEntries` does not exist anywhere in the codebase. The entire test package fails to compile.

**Fix:** Either implement the constant + pool shrink logic in `context.go`, or remove `TestContextStoreResetStrategy` if the feature is not intended.

Recommended: implement it. Pools that hold large maps amplify GC pressure. Add to `context.go`:

```go
const maxPooledStoreEntries = 16

func (c *Context) release() {
    c.ResponseWriter = nil
    c.Request = nil
    c.logger = nil
    c.written = false
    c.err = nil
    if len(c.store) > maxPooledStoreEntries {
        // Discard oversized map; allocate fresh on next use
        c.store = nil
    } else {
        for k := range c.store {
            delete(c.store, k)
        }
    }
    contextPool.Put(c)
}
```

Also update `NewContext` to handle `nil` store (already done — `ctx.store == nil` check is present).

---

## High

### 2. Error response ignores error type

**File:** `ares.go:140–144`

```go
if !ctx.written {
    errorResponse := make(map[string]string, 1)
    errorResponse["error"] = err.Error()
    ctx.JSON(http.StatusInternalServerError, errorResponse)
}
```

All handler errors produce `500`. If a handler returns `errors.BadRequest("invalid id")`, the client receives `500` instead of `400`. The `errors` package exists precisely to carry HTTP status codes.

**Fix:**

```go
if !ctx.written {
    if httpErr, ok := err.(*errors.Error); ok {
        ctx.JSON(httpErr.Code, httpErr)
    } else {
        ctx.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }
}
```

Add import `"github.com/xushuhui/ares/errors"` to `ares.go`.

---

## Medium

### 3. Dual error storage is redundant and confusing

**Files:** `ares.go:128–131`, `middleware/logger/logger.go:104–113`

Handler errors are stored in two places:

1. `*r = *r.WithContext(context.WithValue(r.Context(), contextkeys.HandlerError, err))` — for standard http middleware
2. `ctx.SetError(err)` / `ctx.err` — for Ares-aware middleware via `ctx.Error()`

Both storages are populated for every error. The logger reads from (1). `ctx.Error()` returns from (2). Users must know which path their middleware uses.

This is a design decision, not a bug, but it should be documented. Add a comment in `wrapHandler` explaining why both are set.

Alternatively, consolidate: remove the request context mutation and have the logger read `ctx.Error()` instead. This requires the logger to accept or detect the Ares `*Context`.

**Recommendation:** Keep both for now (backward compat with plain `http.Handler` middleware), but add documentation.

### 4. `errors.Error.Error()` leaks internal format to clients

**File:** `errors/error.go:14–16`

```go
func (e *Error) Error() string {
    return "code=" + strconv.Itoa(e.Code) + ", message=" + e.Message
}
```

`Error()` is the Go error string (for logs/internal). The JSON representation via `ctx.JSON(httpErr.Code, httpErr)` will use the struct fields (`code`, `message`), which is correct. No action needed on the JSON path once fix #2 is applied.

However, if code falls through to the non-`errors.Error` branch: `map[string]string{"error": err.Error()}` will expose `"code=400, message=..."` as the error string. Ugly but not harmful.

**Fix:** Change `Error()` to return only the message:

```go
func (e *Error) Error() string {
    return e.Message
}
```

Log the status code separately where needed.

### 5. `formatStack` in recovery middleware is fragile

**File:** `middleware/recovery/recovery.go:99–120`

The stack formatter manually parses `debug.Stack()` output using string heuristics (`!strings.HasPrefix(line, "\t")`, `strings.Contains(line, ".go:")`). The format of `debug.Stack()` is not part of Go's public API and can change.

**Fix:** Use the raw stack string directly, or use `runtime.Callers` + `runtime.CallersFrames` for structured access.

Simplest safe fix:

```go
func (c *Context) release() {
    // ... existing logic
}
```

For the stack, just log the raw bytes:

```go
fields = append(fields, "stack", string(debug.Stack()))
```

Remove `formatStack` entirely.

---

## Low

### 6. Duplicate default logger initialization

**Files:** `ares.go:97`, `middleware/logger/logger.go:125`, `middleware/recovery/recovery.go:60`

Same expression repeated three times:

```go
slog.New(slog.NewJSONHandler(os.Stdout, nil))
```

**Fix:** Export a shared constructor from a `internal/defaultlogger` package, or accept this duplication since each package is independently usable.

### 7. `recovery.go:89` — Write error silently ignored

**File:** `middleware/recovery/recovery.go:89`

```go
w.Write([]byte(`{"error":"Internal Server Error"}`))
```

Return values `(int, error)` not checked. Acceptable in panic recovery context (nothing useful can be done), but add explicit discard:

```go
_, _ = w.Write([]byte(`{"error":"Internal Server Error"}`))
```

### 8. `logger/logger.go` — `Write` wrapper is a no-op

**File:** `middleware/logger/logger.go:64–67`

```go
func (rw *responseWriter) Write(b []byte) (int, error) {
    n, err := rw.ResponseWriter.Write(b)
    return n, err
}
```

This wraps but adds nothing (no byte counting, no state update). Either remove this method (delegation happens automatically via the embedded `http.ResponseWriter`) or add byte tracking if needed for access logs.

### 9. `logger/logger.go` — nil map read guarded unnecessarily

**File:** `middleware/logger/logger.go:137`

```go
if _, ok := o.skipPathSet[r.URL.Path]; ok {
```

Go nil map reads are safe (always return zero value + `false`). The guard `if len(o.skipPaths) > 0` before building `skipPathSet` (lines 127–132) means `skipPathSet` is nil when there are no skip paths. The map lookup on line 137 is safe regardless. No change needed, but the `len` guard could be removed to simplify.

---

## Test Coverage Gaps

| Missing Test | Impact |
|---|---|
| Handler returns `errors.BadRequest` → expect 400 response | High (after fix #2) |
| `Static()` and `StaticFile()` route registration | Medium |
| Concurrent `Context` pool access | Medium |
| Pool store shrink (after fix #1) | High (blocks compilation now) |
| `Group.Use()` middleware ordering | Low |

---

## Refactoring Priority

| # | Issue | File | Severity | Effort |
|---|---|---|---|---|
| 1 | `maxPooledStoreEntries` undefined | `context.go`, `context_test.go` | Critical | Small |
| 2 | Error response ignores type | `ares.go` | High | Small |
| 3 | `errors.Error()` format | `errors/error.go` | Medium | Trivial |
| 4 | `formatStack` fragile | `recovery/recovery.go` | Medium | Small |
| 5 | Dual error storage | `ares.go`, `logger.go` | Medium | Document |
| 6 | Duplicate logger init | Multiple | Low | Small |
| 7 | Ignored Write error | `recovery/recovery.go` | Low | Trivial |
| 8 | No-op Write wrapper | `logger/logger.go` | Low | Trivial |
