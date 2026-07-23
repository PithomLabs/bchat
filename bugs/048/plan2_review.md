# Plan v2 Review: Bug 048 — Incorporating Review Findings

**Reviewer:** Senior Go Architect  
**Date:** 2026-07-24  
**Status:** Approved with Nits

---

## Verdict

Plan v2 correctly implements all feedback from plan_review.md. The separation of DB errors from not-found, the context key constant, the TODO comment, the expanded test plan, and the PR documentation note are all properly addressed. The plan is clean, focused, and ready. Two new nits surfaced on second pass.

---

## New Nit 1: `tenantContextKey` Should Be Shared with `tenant_helpers.go`

**Files:** `tenant_resolver.go` (new) and `tenant_helpers.go:14,25`

The plan creates `const tenantContextKey = "tenant-id"` in the new middleware file. However, `tenant_helpers.go:14` and `tenant_helpers.go:25` still hardcode `"tenant-id"` directly. Since both files are in the same `agent` package, the constant should be defined once and used in both places.

**Recommendation:** Define the constant in one shared location (e.g., `tenant_helpers.go` or a dedicated `const.go` in the package), then update `tenant_helpers.go` to reference it:

- `c.Get("tenant-id")` on line 14 → `c.Get(tenantContextKey)`

This prevents drift within the same package and keeps the magic string in exactly one place.

**Add to Files to Modify table:**

| File | Change |
|------|--------|
| `server/router/api/v1/agent/tenant_helpers.go` | Replace hardcoded `"tenant-id"` with `tenantContextKey` constant |

---

## New Nit 2: Error Message Capitalization

**File:** `tenant_resolver.go` — error message for DB failure

The plan uses `"Failed to resolve agent"` with a capital F, which is inconsistent with the codebase convention of lowercase error messages in the tenant-binding layer.

| Location | Style |
|----------|-------|
| `tenant_binding.go:26` | `"failed to verify tenant binding"` |
| `tenant_binding.go:57` | `"access denied to this tenant"` |
| `tenant_helpers.go:25` | `"tenant context not set - middleware may not be configured correctly"` |
| `bridge_middleware.go:147` | `"Failed to read request body"` (mixed case elsewhere) |

The `agent` package and `tenant_binding.go` consistently use lowercase. The `v1` auth middleware (`v1.go`) uses title case, but that's a different layer.

**Recommendation:** Use `"failed to resolve agent"` (lowercase f) to match the established pattern in the tenant-binding layer.

---

## Minor Observations

### Test snippet uses magic string instead of constant

The test code snippet at line 96 references `c.Get("tenant-id")` directly. If the constant approach is adopted (Nit 1), the test should use `tenantContextKey` instead:

```go
capturedTenantID = c.Get(tenantContextKey).(int32)
```

### DB error → 500 test case not covered

The expanded test plan adds context-propagation but not a DB-error-returns-500 case. This is acceptable — mocking `store.Store` to inject a DB error would require mocking infrastructure that does not exist in this codebase. If a mock driver is ever added, this test should be revisited.

### No test file path specified

The plan mentions writing tests but does not specify the file path. Should be `server/router/api/v1/agent/tenant_resolver_test.go` for consistency with Go conventions (`_test.go` suffix, same package).

---

## Summary

| Finding | Verdict |
|---------|---------|
| Nit 1: `tenantContextKey` not shared with `tenant_helpers.go` | **New — add to scope** |
| Nit 2: `"Failed"` → `"failed"` capitalization | **New — fix** |
| Minor: Test snippet uses magic string | Self-fixes if Nit 1 is adopted |
| Minor: No DB-error test case | Acceptable limitation |
| Minor: No test file path in plan | Add `tenant_resolver_test.go` |
