# Core Reliability Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve runtime safety, API consistency, and error semantics of the core framework without changing public routing ergonomics.

**Architecture:** Keep the current lightweight architecture, but harden core boundaries: typed context keys, explicit error translation layer, safer recovery behavior, and better memory behavior for pooled `Context`. Changes are incremental and verified with focused regression tests before each implementation step.

**Tech Stack:** Go, net/http, chi, slog, standard testing (`go test`)

---

## Problem Statement (Code Issues Found)

1. Untyped string context keys are collision-prone and duplicated across packages.
- Evidence: `ErrorKey` is `const string` in `/Users/xsh/gp/ares/ares.go:18` and duplicated as `errorKey` string in `/Users/xsh/gp/ares/middleware/logger/logger.go:14`.
- Impact: Hidden key collisions with third-party middleware and brittle cross-package coupling.

2. `Server.Start(context.Context)` currently ignores the context.
- Evidence: explicit comment in `/Users/xsh/gp/ares/server.go:45-47`; implementation does not use `ctx`.
- Impact: Misleading API contract; callers assume cancellation support that does not exist.

3. Recovery middleware writes default 500 response without checking whether response is already committed.
- Evidence: unconditional `WriteHeader`/`Write` in `/Users/xsh/gp/ares/middleware/recovery/recovery.go:87-89`.
- Impact: Potential mixed/partial response and noisy "superfluous WriteHeader" behavior after panic in partially-written handlers.

4. `Attachment` default filename leaks full server path.
- Evidence: empty filename falls back to full `filepath` in `/Users/xsh/gp/ares/context.go:213-216`.
- Impact: Information disclosure via `Content-Disposition` header and poor client UX.

5. Pooled `Context.store` map is only key-deleted, capacity is never reset.
- Evidence: `delete` loop only in `/Users/xsh/gp/ares/context.go:53-56`.
- Impact: Large one-off requests can permanently inflate pooled map capacity, increasing memory retention.

6. Handler error response path always emits HTTP 500 and exposes raw error message.
- Evidence: `/Users/xsh/gp/ares/ares.go:143-146` always uses 500 + `err.Error()`.
- Impact: Custom HTTP errors (`errors.Error`) are ignored; internal messages may leak to clients.

---

## Refactor Scope and Constraints

- Preserve current route registration API (`GET/POST/...`) and middleware registration behavior.
- Avoid broad rewrites; prioritize targeted hardening with regression tests.
- Keep changes backward-compatible unless explicitly called out in release notes.

---

### Task 1: Context Key Type Safety and De-duplication

**Files:**
- Modify: `/Users/xsh/gp/ares/ares.go`
- Modify: `/Users/xsh/gp/ares/middleware/logger/logger.go`
- Test: `/Users/xsh/gp/ares/ares_test.go`
- Test: `/Users/xsh/gp/ares/middleware/logger/logger_test.go`

- [ ] **Step 1: Write the failing test**
Add tests asserting that logger reads handler error through shared exported typed key instead of private duplicated string constants.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -timeout 60s ./... -run 'TestLogger.*HandlerError|Test.*ErrorKey'`
Expected: failure because current key typing/sharing is not enforced.

- [ ] **Step 3: Write minimal implementation**
Introduce typed context key in core package and make logger consume it directly.

- [ ] **Step 4: Run tests to verify pass**
Run: `go test -timeout 60s ./... -run 'TestLogger.*HandlerError|Test.*ErrorKey'`
Expected: PASS.

- [ ] **Step 5: Commit**
`git commit -m "refactor(core): unify typed error context key"`

---

### Task 2: Error Response Translation Layer

**Files:**
- Modify: `/Users/xsh/gp/ares/ares.go`
- Modify: `/Users/xsh/gp/ares/errors/error.go` (if needed for helper exposure)
- Test: `/Users/xsh/gp/ares/ares_test.go`

- [ ] **Step 1: Write the failing test**
Add tests for:
- returned `*errors.Error{Code:404,...}` maps to 404 response.
- unknown error returns generic message without leaking internals (configurable decision documented in README).

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -timeout 60s ./... -run 'Test.*HandlerErrorResponse'`
Expected: FAIL because current implementation always returns 500 + raw message.

- [ ] **Step 3: Write minimal implementation**
Add small internal translator function in `ares.go` that derives `(statusCode, clientMessage)` from error type.

- [ ] **Step 4: Run tests to verify pass**
Run: `go test -timeout 60s ./... -run 'Test.*HandlerErrorResponse'`
Expected: PASS.

- [ ] **Step 5: Commit**
`git commit -m "refactor(core): map handler errors to safe http responses"`

---

### Task 3: Recovery Middleware Committed-Response Safety

**Files:**
- Modify: `/Users/xsh/gp/ares/middleware/recovery/recovery.go`
- Test: `/Users/xsh/gp/ares/middleware/recovery/recovery_test.go`

- [ ] **Step 1: Write the failing test**
Add regression test: handler writes headers/body, then panics; recovery must log panic but must not overwrite already-committed response.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -timeout 60s ./middleware/recovery -run TestRecoveryAfterPartialWrite`
Expected: FAIL with current unconditional write path.

- [ ] **Step 3: Write minimal implementation**
Wrap writer with committed-state tracking (or detect via interface) and guard default 500 write when already committed.

- [ ] **Step 4: Run tests to verify pass**
Run: `go test -timeout 60s ./middleware/recovery`
Expected: PASS.

- [ ] **Step 5: Commit**
`git commit -m "fix(recovery): avoid overwriting committed responses"`

---

### Task 4: Attachment Filename Sanitization

**Files:**
- Modify: `/Users/xsh/gp/ares/context.go`
- Test: `/Users/xsh/gp/ares/context_test.go`

- [ ] **Step 1: Write the failing test**
Add tests that empty `filename` uses base name only (not full path), and special characters are safely quoted.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -timeout 60s ./... -run 'TestContextAttachment.*'`
Expected: FAIL on full-path fallback behavior.

- [ ] **Step 3: Write minimal implementation**
Use `path/filepath.Base` for default filename and safe header construction for `Content-Disposition`.

- [ ] **Step 4: Run tests to verify pass**
Run: `go test -timeout 60s ./... -run 'TestContextAttachment.*'`
Expected: PASS.

- [ ] **Step 5: Commit**
`git commit -m "fix(context): sanitize attachment filename default"`

---

### Task 5: Pooled Store Memory Retention Control

**Files:**
- Modify: `/Users/xsh/gp/ares/context.go`
- Test: `/Users/xsh/gp/ares/context_test.go`

- [ ] **Step 1: Write the failing test**
Add test to simulate high-cardinality `store` usage and verify release path resets map when size/capacity threshold exceeded.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -timeout 60s ./... -run TestContextStoreResetStrategy`
Expected: FAIL because current implementation only deletes keys.

- [ ] **Step 3: Write minimal implementation**
Introduce simple heuristic in `release()`:
- small map: delete keys and reuse
- oversized map: reallocate fresh small map

- [ ] **Step 4: Run tests to verify pass**
Run: `go test -timeout 60s ./... -run 'TestContext(Store|Pool).*'`
Expected: PASS.

- [ ] **Step 5: Commit**
`git commit -m "perf(context): cap pooled store memory growth"`

---

### Task 6: Server Lifecycle Contract Clarification

**Files:**
- Modify: `/Users/xsh/gp/ares/server.go`
- Modify: `/Users/xsh/gp/ares/server_test.go`
- Modify: `/Users/xsh/gp/ares/README.md`
- Modify: `/Users/xsh/gp/ares/README.zh.md`

- [ ] **Step 1: Write the failing test**
Add contract test for `Start(ctx)` behavior (either honor cancellation before bind, or explicitly document and enforce non-cancelable start semantics).

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -timeout 60s ./... -run 'TestHTTPServerStart.*Context'`
Expected: FAIL based on chosen contract.

- [ ] **Step 3: Write minimal implementation**
Pick one direction and make code/docs consistent:
- Option A (recommended): honor `ctx.Done()` before and during startup coordination.
- Option B: remove `ctx` from interface in next major version, keep compatibility shim now.

- [ ] **Step 4: Run tests to verify pass**
Run: `go test -timeout 60s ./...`
Expected: PASS.

- [ ] **Step 5: Commit**
`git commit -m "refactor(server): align start lifecycle with context contract"`

---

## Cross-Cutting Verification

- [ ] Run formatting/lint gates used by repo.
- [ ] Run full test suite with timeout guard: `go test -timeout 60s ./...`.
- [ ] Ensure README and README.zh remain behavior-accurate.
- [ ] Add migration notes for any observable behavior change (especially error response semantics).

---

## Suggested Execution Order

1. Task 1 (key safety)
2. Task 2 (error translation)
3. Task 3 (recovery safety)
4. Task 4 (attachment sanitization)
5. Task 5 (pool memory control)
6. Task 6 (server lifecycle contract)

This order minimizes risk: foundation first, behavior changes next, then memory/lifecycle hardening.

---

## Acceptance Criteria

- No regression in existing public routing/middleware APIs.
- All new behavior is protected by tests that fail before implementation.
- Full suite passes under 60s timeout per command.
- Core docs reflect actual runtime behavior.
