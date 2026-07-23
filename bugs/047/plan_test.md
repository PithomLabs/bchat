# Test Failure Remediation Plan — `go test ./...` (2026-07-23)

**Status:** Ready to implement (awaiting go-signal)  
**Scope:** Fix 17 pre-existing test failures (16 bridge/role_template + 1 migration)

---

## Root Cause Analysis

### Issue A: Bridge Middleware Missing Tenant Context (16 tests)

**Error:** `"tenant context not set - middleware may not be configured correctly"`

**Source:** `server/router/api/v1/agent/tenant_helpers.go:25`
```go
func getTenantIDOrFail(c echo.Context) (int32, error) {
    tenantID := getTenantFromContext(c)
    if tenantID == nil {
        return 0, echo.NewHTTPError(400, "tenant context not set - middleware may not be configured correctly")
    }
    return *tenantID, nil
}
```

`getTenantFromContext` (line 13) looks for `c.Get("tenant-id").(int32)`.

#### Why it fails

In production, tenant context is set by:
1. **`TenantBindingMiddleware`** — resolves `:slug` URL param → sets `c.Set("tenant-id", tenant.ID)`
2. **`AuthMiddleware`** — reads JWT claims → sets `c.Set("tenant-id", *claims.TenantID)`

In the failing tests, **neither mechanism is used**:

| Test File | Setup | Missing |
|-----------|-------|---------|
| `bridge_endpoints_test.go` | Routes use `RequireBridgeHMAC(ts, enc)` only | `RequireBridgeHMAC` authenticates HMAC but **never calls `c.Set("tenant-id", tenant.ID)`** |
| `bridge_delivery_test.go` | Same as above + some use handler directly | Same |
| `role_template_handler_test.go` | No middleware; manual echo context | Tests set `c.Set("user-id", ...)` but never `c.Set("tenant-id", ...)` |

**This is a production bug, not just a test bug.** The `bridgeGroup` middleware (`v1.go:302-307`) uses `RequireBridgeHMAC` but NOT `TenantBindingMiddleware`. Bridge handlers would also fail in production.

---

### Issue B: Stale Migration Test Assertion (1 test)

**Test:** `TestMigrationLoopSkipsAlreadyApplied`  
**File:** `store/test/migrator_test.go:123`  
**Error:** `[]string{"0.31.3", "0.34.1"} does not contain "0.33.2"`

The test hardcodes `"0.33.2"` as the expected schema version. However:
- New migrations were added since the test was written (`0.33/02__add_rag_threshold.sql`, `0.34/00__add_user_access_token_lookup.sql`)
- The migration system records only the **final batch target version** (`0.34.1`), not individual file versions
- The comment on line 98 is stale: `// DB is at current FS version, history = ["0.33.2"]` — actual is now `["0.34.1"]`

---

## Implementation Plan

### Fix 1: `RequireBridgeHMAC` — Set Tenant Context (Production Bug Fix)

**File:** `server/router/api/v1/agent/bridge_middleware.go`

**What to change:** After successful HMAC verification (around line 228, before `return next(c)`), add:
```go
c.Set("tenant-id", tenant.ID)
```

The `tenant` variable is already available at this point (looked up at line 53 for validation).

**Impact:** Fixes all 15 bridge tests AND the production bug where bridge handlers cannot resolve tenant context.

**Why this location:** The middleware already has the `tenant` object from the HMAC validation query. Setting it in context is the natural place — consistent with how `TenantBindingMiddleware` and `AuthMiddleware` work.

---

### Fix 2: `role_template_handler_test.go` — Set Tenant Context in Tests

**File:** `server/router/api/v1/agent/role_template_handler_test.go`

**What to change:** Each test that creates an echo context manually needs to add `c.Set("tenant-id", tenant.ID)` after setting `c.Set("user-id", adminUser.ID)`.

**Pattern:**
```go
c.Set("user-id", adminUser.ID)
c.Set("tenant-id", tenant.ID)  // <-- ADD THIS
```

**Affected tests:** `TestRoleTemplateEndpoints` and its subtests.

**Why not apply `TenantBindingMiddleware`:** These tests create echo contexts manually and register routes without middleware. Adding middleware would require reworking the entire test setup. Setting the context key directly is the minimal fix.

---

### Fix 3: `TestMigrationLoopSkipsAlreadyApplied` — Dynamic Version Check

**File:** `store/test/migrator_test.go`

**What to change:** Replace the hardcoded assertion at line 123:
```go
// BEFORE (stale):
require.Contains(t, versions, "0.33.2", "0.33.x migrations should have been applied")

// AFTER (dynamic):
schemaVersion, err := ts.GetCurrentSchemaVersion()
require.NoError(t, err)
require.Contains(t, versions, schemaVersion, "latest migrations should have been applied")
```

Also update the stale comment at line 98:
```go
// BEFORE:
ts := NewTestingStore(ctx, t) // DB is at current FS version, history = ["0.33.2"]

// AFTER:
ts := NewTestingStore(ctx, t) // DB is at current FS version
```

**Why dynamic:** Hardcoded version strings break every time a new migration is added. `GetCurrentSchemaVersion()` reads the actual FS maximum, making the test future-proof.

---

## Files to Modify

| File | Change | Lines |
|------|--------|-------|
| `server/router/api/v1/agent/bridge_middleware.go` | Add `c.Set("tenant-id", tenant.ID)` after HMAC verification | ~1 line |
| `server/router/api/v1/agent/role_template_handler_test.go` | Add `c.Set("tenant-id", tenant.ID)` in test setup | ~3-5 lines |
| `store/test/migrator_test.go` | Dynamic version check + update stale comment | ~3 lines |

---

## Verification Steps

### 1. Targeted Tests
```bash
go test ./server/router/api/v1/agent/ -v -run TestBridge -count=1
go test ./server/router/api/v1/agent/ -v -run TestRoleTemplate -count=1
go test ./store/test/ -v -run TestMigrationLoopSkipsAlreadyApplied -count=1
```

### 2. Full Test Suite
```bash
go test ./... -count=1
```
Expected: All packages pass (or only pre-existing failures unrelated to these fixes).

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| `c.Set("tenant-id")` in bridge middleware changes production behavior | Low | High | Intentional fix — bridge handlers need tenant context |
| Role template test context change breaks other tests | Low | Low | Only adds context key, no existing code depends on its absence |
| Dynamic migration version check is too permissive | Low | Low | `GetCurrentSchemaVersion()` returns the exact FS max |
| Other bridge middleware consumers depend on tenant-id being nil | Very Low | Medium | Search for `getTenantIDOrFail` usage — all handlers expect it to be set |

---

## Definition of Done

- [ ] `bridge_middleware.go` sets `tenant-id` context after HMAC verification
- [ ] `role_template_handler_test.go` sets `tenant-id` in all manual echo contexts
- [ ] `migrator_test.go` uses dynamic version check
- [ ] `go test ./server/router/api/v1/agent/ -count=1` passes
- [ ] `go test ./store/test/ -run TestMigrationLoopSkipsAlreadyApplied -count=1` passes
- [ ] `go test ./... -count=1` passes (or only pre-existing failures)
