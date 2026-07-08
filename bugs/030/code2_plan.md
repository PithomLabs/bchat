# Security Remediation v3 — Consolidated Implementation Plan

**Based on:** 3 adversarial code reviews (Mimo, DeepSeek, Stepfun) of `bugs/030/code.md`  
**Date:** 2026-07-09  
**Status:** 📋 Ready for implementation

---

## Executive Summary

Three independent reviewers identified **24 findings** across the security remediation. After deduplication and validation, **15 actionable items** remain. The plan is organized by severity and phase dependency.

**Critical path:** Fix the 3 items marked 🔴 first — they are regressions or active vulnerabilities.

| Severity | Count | Items |
|----------|-------|-------|
| 🔴 CRITICAL (regression/vuln) | 3 | #1, #2, #3 |
| 🟠 HIGH (defense-in-depth gaps) | 4 | #4, #5, #6, #7 |
| 🟡 MEDIUM (operational/consistency) | 5 | #8, #9, #10, #11, #12 |
| 🔵 LOW (hardening/UX) | 3 | #13, #14, #15 |

---

## 🔴 CRITICAL — Must fix before merge

---

### #1 — `HandleSelectTenant` cookie uses `SameSite=None` (REGRESSION)

**Reviewed by:** Mimo, DeepSeek, Stepfun (unanimous)  
**File:** `server/router/api/v1/auth_service.go:536-548`  
**Impact:** Multi-tenant login flow permits cross-site CSRF attacks.

**Problem:** `buildAccessTokenCookie` (line 307) correctly uses `SameSite=Lax`, but `HandleSelectTenant` constructs its own cookie with old conditional logic:

```go
// CURRENT (BROKEN):
if isHTTPS {
    cookie.SameSite = http.SameSiteNoneMode   // line 544
    cookie.Secure = true
} else {
    cookie.SameSite = http.SameSiteStrictMode  // line 547
}
```

**Fix:** Replace the conditional with unconditional `SameSite=Lax`:

```go
// FIXED:
cookie.SameSite = http.SameSiteLaxMode
if isHTTPS {
    cookie.Secure = true
}
```

**Verification:** Grep for all `SameSite` assignments in `auth_service.go` — all must be `SameSiteLaxMode`.

---

### #2 — `deleteBackingResourceFile` missing containment assertion

**Reviewed by:** DeepSeek (H-002)  
**File:** `server/router/api/v1/memo_resource_service.go:252-283`  
**Impact:** Path traversal on delete path if `resource.Reference` is corrupted in DB.

**Problem:** `SaveResourceBlob` and `GetResourceBlob` both have containment assertions, but `deleteBackingResourceFile` does not. It reconstructs a path from `resource.Reference` and calls `os.Remove` without checking the path is within the data directory.

**Fix:** Add containment assertion before `os.Remove`:

```go
func (s *APIMemoService) deleteBackingResourceFile(ctx context.Context, resource *store.MemoResource) error {
    if resource.Reference == "" {
        return nil
    }

    // Sanitize the reference path
    filename := sanitizeFilename(filepath.Base(resource.Reference))
    p := filepath.Join(s.profile.Data, resource.Reference)

    // Containment assertion
    cleanDataDir := filepath.Clean(s.profile.Data) + string(os.PathSeparator)
    if !strings.HasPrefix(filepath.Clean(p), cleanDataDir) {
        return fmt.Errorf("path traversal detected: %s", p)
    }

    if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
        return fmt.Errorf("failed to delete backing resource file: %w", err)
    }
    return nil
}
```

**Verification:** Verify `sanitizeFilename` is imported and available in `memo_resource_service.go`.

---

### #3 — `ApplyTenantFilter` is dead code for Memos

**Reviewed by:** DeepSeek (H-001)  
**Files:** `server/router/api/v1/tenant_context.go:73-87`, `server/router/api/v1/memo_service.go`  
**Impact:** No SQL-level tenant isolation for scoped admin memo queries.

**Problem:** `ApplyTenantFilter` exists but is never called in production code. `memo_service.go` uses `GetTenantIDFromContext(ctx)` (gRPC context), which returns `nil` for admin users. The `TenantIDs` plumbing exists in the store layer but is never wired.

**Fix:** Call `ApplyTenantFilter` from the memo query path. Specifically, update `ListMemos` in `memo_service.go` to use the filter:

```go
// In memo_service.go — wherever ListMemos is called via gRPC:
func (s *APIMemoService) ListMemos(ctx context.Context, request *v1pb.ListMemosRequest) (*v1pb.ListMemosResponse, error) {
    // ... existing code ...

    findMemo := &store.FindMemo{
        // ... existing fields ...
    }

    // ADD: Apply tenant filter for scoped admins
    // Use the store method directly since we're in gRPC context
    user, _ := s.getCurrentUser(ctx)
    if user != nil && user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) > 0 {
        tenantIDs := deriveTenantIDsFromGUIDs(ctx, s.Store, user.AllowedTenantIDs)
        if tenantIDs != nil {
            findMemo.TenantIDs = tenantIDs
        }
    }

    // ... rest of query ...
}
```

**Note:** Need a helper `deriveTenantIDsFromGUIDs` that does the GUID→ID resolution (similar to `deriveTenantIDsForScopedAdmin` but taking a store reference instead of echo context). Alternatively, extract the GUID resolution logic into a shared function.

**Verification:** Add a unit test that creates two tenants, a scoped admin with access to only one, and verifies the scoped admin cannot list memos from the other.

---

## 🟠 HIGH — Defense-in-depth gaps

---

### #4 — `authGroup` routes lack `TenantBindingMiddleware`

**Reviewed by:** Stepfun  
**File:** `server/router/api/v1/v1.go:278-332`  
**Impact:** Scoped admins can access any tenant via `authGroup` handlers using `isAdmin` checks.

**Problem:** `TenantBindingMiddleware` is only on `adminGroup` (line 338). The `authGroup` has no tenant binding. Handlers like `HandleValidateTenant`, `HandleSetLLMConfig`, `HandleListPermissions`, `HandleGrantPermission`, `HandleListSessions`, `HandleGetSession`, `HandleImportScript` use `isAdmin(c)` which returns `true` for scoped admins.

**Fix (Option B from plan2.md):** Add `TenantBindingMiddleware` to `authGroup`:

```go
// server/router/api/v1/v1.go
authGroup := echoServer.Group("/api/v1/agent")
authGroup.Use(s.AuthMiddleware)
authGroup.Use(adminCORS)
authGroup.Use(TenantBindingMiddleware(s.Store))  // ADD THIS
```

**Alternative:** If some `authGroup` routes are intentionally tenant-agnostic (e.g., `HandleValidateTenant` validates ANY slug), then add `isSuperAdmin` checks instead of middleware. But `TenantBindingMiddleware` already handles the no-slug case by skipping the check, so middleware is cleaner.

**Verification:** Test that scoped admin on tenant A gets 403 when calling `HandleSetLLMConfig` with tenant B slug.

---

### #5 — `deleteBackingResourceFile` missing containment assertion

*Covered by #2 above.*

---

### #6 — `sanitizeFilename` null-byte stripping ordering

**Reviewed by:** DeepSeek (M-005)  
**File:** `server/router/api/v1/resource_service.go:304-314`  
**Impact:** Potential bypass on platforms where `filepath.Base` truncates at null bytes.

**Problem:** Current order:
```go
filename = filepath.Base(filename)         // line 307
filename = strings.ReplaceAll(filename, "\x00", "")  // line 309
```

**Fix:** Reverse the order:
```go
filename = strings.ReplaceAll(filename, "\x00", "")  // strip nulls first
filename = filepath.Base(filename)                     // then extract basename
if filename == "." || filename == ".." || filename == "" {
    filename = "unnamed"
}
```

**Verification:** Unit test with `"\x00../../../etc/passwd"` — should return `"passwd"`.

---

### #7 — `isSuperAdmin` duplicates `isSuperUser` logic

**Reviewed by:** Stepfun (C1), DeepSeek (M-001)  
**Files:** `server/router/api/v1/agent/handlers.go:2246-2268`, `server/router/api/v1/common.go:70-71`  
**Impact:** Maintenance risk — if super-user definition changes, both must be updated.

**Problem:** Two separate functions with identical logic:
```go
// common.go
func isSuperUser(user *store.User) bool {
    return user.Role == store.RoleHost || (user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0)
}

// handlers.go
func (h *Handler) isSuperAdmin(c echo.Context) bool {
    // ... fetches user, then same check
}
```

**Fix:** Make `isSuperAdmin` delegate to `isSuperUser`:

```go
func (h *Handler) isSuperAdmin(c echo.Context) bool {
    userID, ok := c.Get(getUserIDContextKey()).(int32)
    if !ok {
        return false
    }
    user, err := h.store.GetUser(c.Request().Context(), &store.FindUser{ID: &userID})
    if err != nil || user == nil {
        return false
    }
    return isSuperUser(user)  // delegate to shared function
}
```

**Note:** `isSuperUser` in `common.go` is unexported (lowercase). It's in the same package (`v1`), so `handlers.go` can call it directly. If they were in different packages, would need to export it.

**Verification:** Grep for both functions. Confirm `isSuperAdmin` calls `isSuperUser`.

---

## 🟡 MEDIUM — Operational and consistency issues

---

### #8 — `isSuperAdmin` lacks audit logging

**Reviewed by:** DeepSeek (M-001)  
**File:** `server/router/api/v1/agent/handlers.go:2246-2268`  
**Impact:** No audit trail for scoped admin access denials.

**Fix:** Add `slog.Warn`/`slog.Debug` calls matching `isAdmin`'s pattern:

```go
func (h *Handler) isSuperAdmin(c echo.Context) bool {
    userID, ok := c.Get(getUserIDContextKey()).(int32)
    if !ok {
        slog.Debug("isSuperAdmin: no user ID in context")
        return false
    }
    user, err := h.store.GetUser(c.Request().Context(), &store.FindUser{ID: &userID})
    if err != nil || user == nil {
        slog.Warn("isSuperAdmin: user not found", "user_id", userID, "error", err)
        return false
    }
    isSuper := isSuperUser(user)
    if !isSuper {
        slog.Debug("isSuperAdmin: user is not super admin", "user_id", userID, "role", user.Role, "allowed_tenants", len(user.AllowedTenantIDs))
    }
    return isSuper
}
```

---

### #9 — `HandleDeleteTenant` and `HandleOnboard` use `isAdmin` not `isSuperAdmin`

**Reviewed by:** DeepSeek (M-003)  
**Files:** `server/router/api/v1/agent/handlers.go:1659,1298`  
**Impact:** Scoped admins can delete/onboard tenants they shouldn't access.

**Fix:** Replace `isAdmin` with `isSuperAdmin` on these global tenant management handlers:

```go
// HandleDeleteTenant
if !h.isSuperAdmin(c) {
    return echo.NewHTTPError(http.StatusForbidden, "requires super admin")
}

// HandleOnboard
if !h.isSuperAdmin(c) {
    return echo.NewHTTPError(http.StatusForbidden, "requires super admin")
}
```

**Also check:** `HandleListTenants` (line 660) — should scoped admins list all tenants? If not, use `isSuperAdmin` here too.

---

### #10 — `entrypoint.sh` crashes if `gosu` is missing

**Reviewed by:** DeepSeek (M-004), Stepfun  
**File:** `scripts/entrypoint.sh:46-52`  
**Impact:** Dev workflows running as root without gosu installed crash.

**Fix:** Add `command -v` check:

```bash
if [ "$(id -u)" = '0' ]; then
    chown -R memos:memos /var/opt/memos 2>/dev/null || true
    if command -v gosu >/dev/null 2>&1; then
        exec gosu memos "$@"
    else
        echo "WARNING: gosu not found, running as root" >&2
        exec "$@"
    fi
fi

exec "$@"
```

---

### #11 — Empty `TenantIDs` slice produces SQL `IN ()` syntax error

**Reviewed by:** Mimo, Stepfun  
**File:** `server/router/api/v1/tenant_context.go:60-63`  
**Impact:** 500 errors instead of 403 when all GUID lookups fail.

**Problem:** Returns `[]int32{}` (empty, not nil). Filter functions check `if tenantIDs != nil`, so empty slice IS applied. SQLite/Postgres drivers may or may not handle empty `IN ()`.

**Fix:** Return `nil` instead of empty slice for consistency:

```go
if len(tenantIDs) == 0 {
    // No valid tenants found — deny all by returning nil
    // This matches the "super users see all" path behavior
    return nil
}
```

**Wait — this changes semantics.** If we return nil, scoped admins with no valid GUIDs would see ALL tenants (nil = no filter). The correct fix is to return a sentinel that means "deny all":

```go
// Option A: Return a special marker
var denyAllTenantIDs = []int32{-1} // no tenant has ID -1

if len(tenantIDs) == 0 {
    return denyAllTenantIDs
}

// Option B: Check length in filter functions
func ApplyTenantFilter(c echo.Context, find *store.FindMemo) {
    tenantIDs := deriveTenantIDsForScopedAdmin(...)
    if tenantIDs != nil && len(tenantIDs) > 0 {
        find.TenantIDs = tenantIDs
    }
    // Empty slice → no filter applied → but this is the bug
}
```

**Recommended:** Option A — return `[]int32{-1}` to ensure no results match. This is explicit and safe.

---

### #12 — Reindex error messages leak internal details

**Reviewed by:** Mimo  
**Files:** `server/router/api/v1/agent/handlers.go:1208-1220, 1249`  
**Impact:** Admin-only endpoints, but C3 fix was supposed to return generic messages in production.

**Fix:** Return generic error messages in production:

```go
func reindexHTTPError(c echo.Context, err error) error {
    slog.Error("reindex failed", "error", err)
    if profile.IsDev() {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }
    return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
}
```

---

## 🔵 LOW — Hardening and UX

---

### #13 — CSRF middleware passes when both `Origin` and `Sec-Fetch-Site` are absent

**Reviewed by:** Mimo  
**File:** `server/router/api/v1/csrf.go:41-46`  
**Impact:** Non-browser clients bypass CSRF when both headers are missing.

**Current behavior:** Missing both headers → allowed through (relies on `SameSite=Lax` as primary defense).  
**Risk:** Low — `SameSite=Lax` prevents cookie sending on cross-site requests. But fail-closed is better posture.

**Fix:** Add a warning log and optionally deny:

```go
// Option A: Log and allow (keep current behavior, document it)
if origin == "" && secFetchSite == "" {
    slog.Debug("CSRF: missing both Origin and Sec-Fetch-Site, relying on SameSite=Lax")
    return next(c)
}

// Option B: Deny (fail-closed, may break some API clients)
if origin == "" && secFetchSite == "" {
    return echo.NewHTTPError(http.StatusForbidden, "CSRF validation failed: missing origin headers")
}
```

**Recommendation:** Option A (log + allow). The widget and API clients may not send these headers. SameSite=Lax provides adequate protection.

---

### #14 — `file_env` silently ignores missing secret files

**Reviewed by:** Stepfun (H5)  
**File:** `scripts/entrypoint.sh:15-19`  
**Impact:** Application starts with empty API key, fails at runtime with unclear error.

**Fix:** Add file existence check:

```bash
file_env() {
    local var="$1"
    local fileVar="${var}_FILE"
    eval local val="\${$var:-}"
    eval local val_fileVar="\${$fileVar:-}"
    if [ -n "$val" ] && [ -n "$val_fileVar" ]; then
        echo >&2 "error: both $var and $fileVar are set (but exclusive)"
        exit 1
    fi
    if [ -n "$fileVar" ]; then
        if [ ! -f "$fileVar" ]; then
            echo >&2 "error: secret file $fileVar does not exist"
            exit 1
        fi
        val="$(cat "$fileVar")"
    fi
    export "$var"="$val"
    unset "$fileVar"
}
```

---

### #15 — `sanitizeFilename` not called on read path

**Reviewed by:** DeepSeek (L-002)  
**File:** `server/router/api/v1/resource_service.go:409-439`  
**Impact:** Defense-in-depth gap — containment assertion is the actual safety boundary.

**Assessment:** The containment assertion on the read path (`GetResourceBlob`) provides equivalent protection. Adding `sanitizeFilename` on read is optional defense-in-depth.

**Fix (optional):** Add `sanitizeFilename` to `GetResourceBlob` for consistency:

```go
func (s *APIMemoService) GetResourceBlob(ctx context.Context, resource *store.Resource) (io.ReadCloser, error) {
    // ... existing code ...
    filename := sanitizeFilename(resource.Filename)  // ADD
    // ... rest of path construction ...
}
```

---

## Implementation Order

```
🔴 CRITICAL (do first — regressions/vulns)
    ├── #1: HandleSelectTenant SameSite=Lax        [auth_service.go]
    ├── #2: deleteBackingResourceFile containment    [memo_resource_service.go]
    └── #3: ApplyTenantFilter wiring for Memos      [memo_service.go + tenant_context.go]

🟠 HIGH (defense-in-depth)
    ├── #4: authGroup + TenantBindingMiddleware     [v1.go]
    ├── #6: sanitizeFilename null-byte ordering     [resource_service.go]
    └── #7: isSuperAdmin delegates to isSuperUser   [handlers.go]

🟡 MEDIUM (operational)
    ├── #8: isSuperAdmin audit logging              [handlers.go]
    ├── #9: HandleDeleteTenant/Onboard → isSuperAdmin [handlers.go]
    ├── #10: entrypoint.sh gosu fallback            [entrypoint.sh]
    ├── #11: Empty TenantIDs → deny all             [tenant_context.go]
    └── #12: Reindex error message genericity       [handlers.go]

🔵 LOW (hardening)
    ├── #13: CSRF missing-Origin behavior           [csrf.go]
    ├── #14: file_env missing file check            [entrypoint.sh]
    └── #15: sanitizeFilename on read path          [resource_service.go]
```

---

## Verification Commands

After implementation:

```bash
# Build
go build ./...

# Test
go test ./server/router/api/v1/...

# Lint
golangci-lint run ./server/...

# Manual checks
grep -n "SameSite" server/router/api/v1/auth_service.go          # All should be LaxMode
grep -n "deleteBackingResourceFile" server/router/api/v1/memo_resource_service.go
grep -n "ApplyTenantFilter" server/router/api/v1/memo_service.go  # Should appear
grep -n "TenantBindingMiddleware" server/router/api/v1/v1.go      # Should be on authGroup
grep -n "isSuperAdmin\|isSuperUser" server/router/api/v1/agent/handlers.go
grep -n "sanitizeFilename" server/router/api/v1/resource_service.go
```

---

## Summary of Changes by File

| File | Items | Changes |
|------|-------|---------|
| `server/router/api/v1/auth_service.go` | #1 | `SameSite=None` → `SameSite=Lax` in `HandleSelectTenant` |
| `server/router/api/v1/memo_resource_service.go` | #2 | Add containment assertion to `deleteBackingResourceFile` |
| `server/router/api/v1/memo_service.go` | #3 | Wire `ApplyTenantFilter` or equivalent for scoped admins |
| `server/router/api/v1/v1.go` | #4 | Add `TenantBindingMiddleware` to `authGroup` |
| `server/router/api/v1/resource_service.go` | #6, #15 | Fix null-byte ordering; optionally sanitize on read |
| `server/router/api/v1/agent/handlers.go` | #7, #8, #9, #12 | Delegate to `isSuperUser`; add logging; fix global handlers; generic errors |
| `server/router/api/v1/tenant_context.go` | #11 | Return deny-all sentinel for empty TenantIDs |
| `scripts/entrypoint.sh` | #10, #14 | gosu fallback; file_env existence check |
| `server/router/api/v1/csrf.go` | #13 | Document/log missing-Origin behavior |
