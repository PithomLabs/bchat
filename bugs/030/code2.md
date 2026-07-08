# Security Remediation v3 — Implementation Documentation

**Date:** 2026-07-09
**Based on:** `code2_plan.md` (consolidated from 3 adversarial reviews of v2)
**Status:** ✅ Implemented — 14 items + 4 nit fixes, build passes, tests pass

---

## Files Modified

| File | Items | Change |
|------|-------|--------|
| `server/router/api/v1/auth_service.go` | #1 | `SameSite=None/Strict` → `SameSite=LaxMode` in `HandleSelectTenant` |
| `server/router/api/v1/memo_resource_service.go` | #2, #2-nit | Added containment assertion to `deleteBackingResourceFile`; added nil-profile absolute-path guard |
| `server/router/api/v1/memo_service.go` | #3, #3-nit | Wired `deriveTenantIDsForScopedAdmin` in `ListMemos`; added scoped admin access check in `GetMemo` |
| `server/router/api/v1/v1.go` | #4 | Added `TenantBindingMiddleware` to `authGroup` |
| `server/router/api/v1/resource_service.go` | #6, #15 | Reversed null-byte stripping order; added `sanitizeFilename` on read path |
| `server/router/api/v1/agent/handlers.go` | #7, #8, #9, #12, #9-nit | `isSuperAdmin` delegates to `store.IsSuperUser`; added audit logging; `HandleDeleteTenant`/`Onboard`/`ListTenants` use `isSuperAdmin`; generic reindex errors |
| `server/router/api/v1/common.go` | #7-nit | `isSuperUser` delegates to `store.IsSuperUser` |
| `store/user.go` | #7-nit | Added exported `IsSuperUser` function (single source of truth) |
| `server/router/api/v1/tenant_context.go` | #11, #7-nit | Empty TenantIDs returns `[]int32{-1}` deny-all sentinel; `deriveTenantIDsForScopedAdmin` delegates to `store.IsSuperUser` |
| `server/router/api/v1/csrf.go` | #13 | Added `slog.Debug` audit log for missing Origin+Sec-Fetch-Site |
| `scripts/entrypoint.sh` | #10, #14 | Added `command -v gosu` fallback; `file_env` exits on missing secret file |

---

## Implementation Details

### #1 — HandleSelectTenant SameSite=Lax (CRITICAL)

**File:** `server/router/api/v1/auth_service.go:543`

**Before:**
```go
if isHTTPS {
    cookie.SameSite = http.SameSiteNoneMode
    cookie.Secure = true
} else {
    cookie.SameSite = http.SameSiteStrictMode
}
```

**After:**
```go
cookie.SameSite = http.SameSiteLaxMode
if isHTTPS {
    cookie.Secure = true
}
```

**Impact:** Multi-tenant login flow now uses SameSite=Lax, consistent with `buildAccessTokenCookie`. All `SameSite` assignments in `auth_service.go` are now `Lax`.

---

### #2 — deleteBackingResourceFile Containment Assertion (CRITICAL)

**File:** `server/router/api/v1/memo_resource_service.go:252-272`

**Added:**
```go
// H3: Containment assertion — ensure resolved path stays within data directory
if s.Profile != nil {
    cleanDataDir := filepath.Clean(s.Profile.Data) + string(os.PathSeparator)
    if !strings.HasPrefix(filepath.Clean(p), cleanDataDir) {
        slog.Warn("path traversal detected in delete", slog.String("path", p))
        return
    }
} else if filepath.IsAbs(p) {
    // Refuse to delete absolute paths when profile data dir is unknown
    slog.Warn("absolute path in resource.Reference with nil profile, refusing delete",
        slog.String("path", p),
    )
    return
}
```

**Impact:** Delete path now has the same containment assertion as save and read paths. Nil-profile + absolute-path edge case is handled.

---

### #3 — ApplyTenantFilter Wiring for Memos (CRITICAL)

**File:** `server/router/api/v1/memo_service.go:166-178, 285-303`

**ListMemos (line 166-178):**
```go
// H-001: For scoped admins, derive TenantIDs filter from AllowedTenantIDs
if currentUser != nil && memoFind.TenantID == nil {
    tenantIDs := deriveTenantIDsForScopedAdmin(ctx, s.Store, currentUser)
    if tenantIDs != nil {
        memoFind.TenantIDs = tenantIDs
    }
}
```

**GetMemo (line 285-303) — new scoped admin check:**
```go
if user != nil && !isSuperUser(user) && tenantID == nil {
    allowedTenantIDs := deriveTenantIDsForScopedAdmin(ctx, s.Store, user)
    if allowedTenantIDs != nil {
        if memo.TenantID == nil {
            return nil, status.Errorf(codes.PermissionDenied, "permission denied")
        }
        allowed := false
        for _, id := range allowedTenantIDs {
            if id == *memo.TenantID {
                allowed = true
                break
            }
        }
        if !allowed {
            return nil, status.Errorf(codes.PermissionDenied, "permission denied")
        }
    }
}
```

**Impact:** Scoped admins are now filtered at the SQL level in `ListMemos` and at the access-check level in `GetMemo`. `ListMemoComments` is covered via its internal `GetMemo` call.

---

### #4 — authGroup + TenantBindingMiddleware (HIGH)

**File:** `server/router/api/v1/v1.go:281`

**Added:**
```go
authGroup := echoServer.Group("/api/v1/agent")
authGroup.Use(s.AuthMiddleware)
authGroup.Use(adminCORS)
authGroup.Use(TenantBindingMiddleware(s.Store))  // NEW
```

**Impact:** All `authGroup` handlers (LLM config, permissions, sessions, scripts, simulations, etc.) are now protected by tenant binding. Scoped admins can only access their assigned tenants on these routes.

---

### #6 — sanitizeFilename Null-Byte Ordering (HIGH)

**File:** `server/router/api/v1/resource_service.go:304-314`

**Before:**
```go
filename = filepath.Base(filename)
filename = strings.ReplaceAll(filename, "\x00", "")
```

**After:**
```go
filename = strings.ReplaceAll(filename, "\x00", "")
filename = filepath.Base(filename)
```

**Impact:** Null bytes are now stripped before `filepath.Base`, preventing platform-specific truncation bypasses.

---

### #7 — isSuperAdmin Single Source of Truth (HIGH)

**Files:** `store/user.go:66-73`, `server/router/api/v1/common.go:66-70`, `server/router/api/v1/agent/handlers.go:2261`, `server/router/api/v1/tenant_context.go:42`

**New `store.IsSuperUser` (canonical definition):**
```go
func IsSuperUser(user *User) bool {
    return user.Role == RoleHost || (user.Role == RoleAdmin && len(user.AllowedTenantIDs) == 0)
}
```

**All callers delegate:**
- `common.go:isSuperUser` → `store.IsSuperUser(user)`
- `handlers.go:isSuperAdmin` → `store.IsSuperUser(user)`
- `tenant_context.go:deriveTenantIDsForScopedAdmin` → `store.IsSuperUser(user)`

**Impact:** Single source of truth for super-user definition. No more maintenance divergence risk.

---

### #8 — isSuperAdmin Audit Logging (MEDIUM)

**File:** `server/router/api/v1/agent/handlers.go:2251-2270`

**Added:**
| Condition | Log | Level |
|-----------|-----|-------|
| No user ID in context | `slog.Debug("isSuperAdmin: no user ID in context")` | Debug |
| User not found | `slog.Warn("isSuperAdmin: user not found", ...)` | Warn |
| User is not super admin | `slog.Debug("isSuperAdmin: user is not super admin", ...)` | Debug |

---

### #9 — HandleDeleteTenant/Onboard/ListTenants Use isSuperAdmin (MEDIUM)

**File:** `server/router/api/v1/agent/handlers.go:1298-1299, 1659-1660, 659-661`

**Before:** All three handlers used `isAdmin(c)`.
**After:** All three use `isSuperAdmin(c)`.

**Impact:** Scoped admins are blocked from deleting tenants, onboarding tenants, and enumerating all tenants.

---

### #10 — entrypoint.sh Gosu Fallback (MEDIUM)

**File:** `scripts/entrypoint.sh:50-56`

**Added:**
```bash
if command -v gosu >/dev/null 2>&1; then
    exec gosu memos "$@"
else
    echo "WARNING: gosu not found, running as root" >&2
    exec "$@"
fi
```

**Impact:** Dev workflows running as root without gosu no longer crash.

---

### #11 — Empty TenantIDs Deny-All Sentinel (MEDIUM)

**File:** `server/router/api/v1/tenant_context.go:62`

**Before:** `return []int32{}` (empty slice, potentially problematic).
**After:** `return []int32{-1}` (sentinel that never matches a real tenant ID).

**Impact:** `tenant_id IN (-1)` returns 0 rows — correct deny-all behavior.

---

### #12 — Reindex Error Message Genericity (MEDIUM)

**File:** `server/router/api/v1/agent/handlers.go:1208-1220, 1249-1252`

**Before:** `message := "Reindex failed: " + err.Error()`
**After:** Generic messages per error type; error details logged server-side via `slog.Error`.

**Impact:** No internal details leaked to clients. Error types still distinguishable via HTTP status codes.

---

### #13 — CSRF Missing-Origin Audit Log (LOW)

**File:** `server/router/api/v1/csrf.go:43-49`

**Added:**
```go
slog.Debug("CSRF: missing both Origin and Sec-Fetch-Site, relying on SameSite=Lax",
    "method", c.Request().Method,
    "path", c.Request().URL.Path,
)
```

---

### #14 — file_env Missing File Check (LOW)

**File:** `scripts/entrypoint.sh:18-21`

**Added:**
```bash
if [ ! -f "$val_fileVar" ]; then
    echo "error: secret file $val_fileVar does not exist" >&2
    exit 1
fi
```

---

### #15 — sanitizeFilename on Read Path (LOW)

**File:** `server/router/api/v1/resource_service.go:412-418`

**Added:**
```go
resourcePath = filepath.Join(filepath.Dir(resourcePath),
    sanitizeFilename(filepath.Base(resourcePath)))
```

**Impact:** Defense-in-depth — read path now sanitizes filenames identical to save path.

---

## Verification Checklist

- [x] Build passes (`go build ./...`)
- [x] Tests pass (`go test ./server/router/api/v1/...`)
- [x] #1: All SameSite in auth_service.go are LaxMode
- [x] #2: Containment assertion on delete path (including nil-profile edge case)
- [x] #3: TenantIDs filter wired in ListMemos; access check in GetMemo
- [x] #4: TenantBindingMiddleware on authGroup
- [x] #6: Null bytes stripped before filepath.Base
- [x] #7: store.IsSuperUser is single source of truth; all callers delegate
- [x] #8: Audit logs in isSuperAdmin
- [x] #9: HandleDeleteTenant, HandleOnboard, HandleListTenants use isSuperAdmin
- [x] #10: Gosu fallback in entrypoint.sh
- [x] #11: Empty TenantIDs returns []int32{-1}
- [x] #12: Generic reindex error messages
- [x] #13: CSRF audit log for missing headers
- [x] #14: file_env checks file existence
- [x] #15: sanitizeFilename on read path

---

## Items Addressed from Reviews

| Source | Finding | Status |
|--------|---------|--------|
| Qwen #2 | deleteBackingResourceFile nil-profile absolute path | ✅ Fixed |
| Qwen #3 | ApplyTenantFilter only in ListMemos, not GetMemo | ✅ Fixed |
| Qwen #7 | isSuperAdmin duplicates isSuperUser logic | ✅ Fixed (store.IsSuperUser) |
| Qwen #9 | HandleListTenants still uses isAdmin | ✅ Fixed (uses isSuperAdmin) |
