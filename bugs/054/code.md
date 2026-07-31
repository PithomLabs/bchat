# Code Documentation: Bug 054 — HOST JWT tenant_id = nil

**Bug ID:** 054
**Date:** 2026-07-31
**Status:** Implemented
**Files Modified:** `server/router/api/v1/auth_service.go`

---

## 1. Problem Statement

Tickets created by HOST users had `tenant_id = NULL` in the database. Because RAG indexing and inference are gated on `ticket.TenantID != nil` / `ticket.TenantID == nil`, HOST-created tickets were skipped by:

- `IndexTicketContent` in `ticket_service.go:178`
- `InferResolutionForNewTicket` in `service.go:5596`

This left `internal_notes` empty and orphaned tickets from tenant-scoped RAG.

## 2. Root Cause Chain

```
HOST gRPC sign-in
  -> doSignIn() only resolves tenant for RoleUser
  -> HOST JWT contains nil tenant_id
  -> Web UI falls back to REST /auth/tenants
  -> HandleAuthTenants queries user_tenant_permission
  -> HOST has 0 permission rows
  -> Returns 403: "user is not associated with any company"
  -> HOST is locked out of tenant selection
  -> CreateTicket uses getTenantFromContext(c) -> nil
  -> INSERT INTO tickets (...) tenant_id VALUES (... NULL)
  -> IndexTicketContent skipped
  -> InferResolutionForNewTicket returns early
```

Three defects contribute:

| Defect | Location | Effect |
|--------|----------|--------|
| `doSignIn` skips HOST/ADMIN | `auth_service.go:174-190` | JWT `tenant_id` stays nil |
| `HandleAuthTenants` blocks HOST | `auth_service.go:406-413` | HOST cannot select a tenant |
| HOST has no `user_tenant_permission` rows | signup/tenant creation flows | 403 path never recovers |

## 3. Fixes Applied

### Fix 1 — `HandleAuthTenants`: Super-user bypass

**File:** `server/router/api/v1/auth_service.go`
**Lines:** 428-463

Super users (`HOST` or unscoped `ADMIN`) now see all tenants via `ListAgentTenants`. Scoped admins and regular users continue using the existing `user_tenant_permission` query.

**Before:**
```go
perms, err := s.Store.ListUserTenantPermissions(c.Request().Context(), &store.FindUserTenantPermission{UserID: &user.ID})
if err != nil {
    return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tenant permissions")
}
if len(perms) == 0 {
    return echo.NewHTTPError(http.StatusForbidden, "user is not associated with any company")
}

tenants := make([]TenantInfo, 0, len(perms))
for _, perm := range perms {
    tenant, err := s.Store.GetAgentTenant(c.Request().Context(), &store.FindAgentTenant{ID: &perm.TenantID})
    if err != nil || tenant == nil {
        continue
    }
    tenants = append(tenants, TenantInfo{...})
}
```

**After:**
```go
// Super users (HOST or unscoped ADMIN) see all tenants.
var tenants []TenantInfo
if isSuperUser(user) {
    allTenants, err := s.Store.ListAgentTenants(c.Request().Context(), &store.FindAgentTenant{})
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, "failed to list tenants")
    }
    tenants = make([]TenantInfo, 0, len(allTenants))
    for _, t := range allTenants {
        tenants = append(tenants, TenantInfo{
            ID:   t.ID,
            Name: t.CompanyName,
            Slug: t.Slug,
        })
    }
} else {
    // Regular users and scoped admins: query permission rows
    perms, err := s.Store.ListUserTenantPermissions(c.Request().Context(), &store.FindUserTenantPermission{UserID: &user.ID})
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tenant permissions")
    }
    if len(perms) == 0 {
        return echo.NewHTTPError(http.StatusForbidden, "user is not associated with any company")
    }
    tenants = make([]TenantInfo, 0, len(perms))
    for _, perm := range perms {
        tenant, err := s.Store.GetAgentTenant(c.Request().Context(), &store.FindAgentTenant{ID: &perm.TenantID})
        if err != nil || tenant == nil {
            continue
        }
        tenants = append(tenants, TenantInfo{
            ID:   tenant.ID,
            Name: tenant.CompanyName,
            Slug: tenant.Slug,
        })
    }
}
```

**Behavior change:**
- HOST/ADMIN without permission rows: returns HTTP 200 with all tenants instead of 403
- Scoped ADMIN: unchanged — still filtered by `AllowedTenantIDs` through the permission-row path
- USER: unchanged

### Fix 2 — `doSignIn`: Resolve tenant for HOST/ADMIN

**File:** `server/router/api/v1/auth_service.go`
**Lines:** 174-212

Extended the existing USER tenant-resolution block to cover HOST/ADMIN roles.

**Before:**
```go
func (s *APIV1Service) doSignIn(ctx context.Context, user *store.User, tenantID *int32, expireTime time.Time) error {
    if user.Role == store.RoleUser {
        perms, err := s.Store.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{UserID: &user.ID})
        if err != nil {
            return status.Errorf(codes.Internal, "failed to verify user company association")
        }
        if len(perms) == 0 {
            return status.Errorf(codes.PermissionDenied, "user is not associated with any company")
        }
        if tenantID == nil && len(perms) == 1 {
            tenantID = &perms[0].TenantID
        } else if tenantID == nil && len(perms) > 1 {
            return status.Errorf(codes.FailedPrecondition, "multiple tenants found, use /auth/tenants endpoint")
        }
    }
    // ...
}
```

**After:**
```go
func (s *APIV1Service) doSignIn(ctx context.Context, user *store.User, tenantID *int32, expireTime time.Time) error {
    // Resolve tenant for all user roles. HOST/ADMIN get the same tenant-resolution
    // behavior as USER: auto-select when exactly one association exists, prompt
    // for selection when multiple exist. When no explicit association exists,
    // fall back to the first tenant in the system.
    if user.Role == store.RoleHost || user.Role == store.RoleAdmin {
        perms, err := s.Store.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{UserID: &user.ID})
        if err != nil {
            return status.Errorf(codes.Internal, "failed to verify tenant association")
        }
        if len(perms) > 0 {
            if tenantID == nil && len(perms) == 1 {
                tenantID = &perms[0].TenantID
            } else if tenantID == nil && len(perms) > 1 {
                return status.Errorf(codes.FailedPrecondition, "multiple tenants found, use /auth/tenants endpoint")
            }
        } else {
            allTenants, err := s.Store.ListAgentTenants(ctx, &store.FindAgentTenant{})
            if err != nil {
                return status.Errorf(codes.Internal, "failed to list tenants")
            }
            if len(allTenants) > 0 && tenantID == nil {
                tenantID = &allTenants[0].ID
            }
        }
    } else if user.Role == store.RoleUser {
        perms, err := s.Store.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{UserID: &user.ID})
        if err != nil {
            return status.Errorf(codes.Internal, "failed to verify user company association")
        }
        if len(perms) == 0 {
            return status.Errorf(codes.PermissionDenied, "user is not associated with any company")
        }
        if tenantID == nil && len(perms) == 1 {
            tenantID = &perms[0].TenantID
        } else if tenantID == nil && len(perms) > 1 {
            return status.Errorf(codes.FailedPrecondition, "multiple tenants found, use /auth/tenants endpoint")
        }
    }

    accessToken, err := GenerateAccessToken(user.Email, user.ID, tenantID, expireTime, []byte(s.Secret))
    // ...
}
```

**Behavior change:**
- HOST/ADMIN with 1 permission row: JWT carries that tenant (same as USER)
- HOST/ADMIN with multiple permission rows: returns gRPC `"multiple tenants"` error (same as USER)
- HOST/ADMIN with 0 permission rows: auto-selects first `agent_tenants` row
- If the system has zero tenants: `tenantID` stays nil, same as pre-fix behavior
- USER path: unchanged

### Super-user helper already existed

**File:** `server/router/api/v1/common.go:68`

```go
func isSuperUser(user *store.User) bool {
    return store.IsSuperUser(user)
}
```

`store.IsSuperUser` (in `store/user.go:70`) returns true for `RoleHost` or unscoped `RoleAdmin` (`AllowedTenantIDs` empty). Both Fix 1 and Fix 2 rely on this existing helper.

---

## 4. Request/Response Impact

### gRPC Sign-In

**Before:**
```
HOST -> doSignIn(nil tenantID) -> GenerateAccessToken(... nil ...) -> JWT has tenant_id = nil
```

**After (no perms):**
```
HOST -> doSignIn(nil tenantID)
     -> ListUserTenantPermissions -> 0 rows
     -> ListAgentTenants -> first tenant
     -> tenantID = &firstTenant.ID
     -> GenerateAccessToken(... tenantID ...) -> JWT has tenant_id = <first tenant>
```

**After (1 perm):**
```
HOST -> doSignIn(nil tenantID)
     -> ListUserTenantPermissions -> 1 row
     -> tenantID = &perm[0].TenantID
     -> GenerateAccessToken(... tenantID ...) -> JWT has tenant_id = <that tenant>
```

### REST `POST /api/v1/auth/tenants`

**Before:**
```
HOST -> HandleAuthTenants -> ListUserTenantPermissions -> 0 rows -> 403
```

**After:**
```
HOST -> HandleAuthTenants -> isSuperUser == true -> ListAgentTenants -> 200 { tenants: [...] }
```

### REST `POST /api/v1/tickets`

When HOST creates a ticket after these fixes:

**Before:**
```sql
INSERT INTO tickets (...) VALUES (... NULL)
```

**After:**
```sql
INSERT INTO tickets (...) VALUES (... <JWT tenant_id>)
```

This unblocks:
- `IndexTicketContent` guard `ticket.TenantID != nil`
- `InferResolutionForNewTicket` early return removed
- `internal_notes` populated from RAG inference

---

## 5. Files Modified

| File | Changes | Description |
|------|---------|-------------|
| `server/router/api/v1/auth_service.go` | ~40 lines | Fix 1: `HandleAuthTenants` super-user bypass. Fix 2: `doSignIn` HOST/ADMIN tenant resolution |

No other files were modified. No migrations were added. No frontend changes were made.

---

## 6. Verification Plan

### Build

```bash
task build:backend        # Compiles
task build:backend:rag    # Compiles with RAG support
go vet ./server/router/api/v1/
go test ./server/router/api/v1/...
```

### Auth Flow Tests

| Test | Command | Expected |
|------|---------|----------|
| HOST gRPC sign-in | gRPC `SignIn` with HOST credentials | JWT contains non-nil `tenant_id` |
| HOST REST `/auth/tenants` | `POST /api/v1/auth/tenants` with HOST creds | HTTP 200 with tenant list, not 403 |
| HOST `select-tenant` | `POST /api/v1/auth/select-tenant` | JWT with selected `tenant_id` |
| USER single tenant | gRPC `SignIn` | No regression — JWT has `tenant_id` |
| USER multiple tenants | gRPC `SignIn` | No regression — returns `"multiple tenants"` error |
| USER no perms | gRPC `SignIn` | No regression — returns 403 |
| Scoped ADMIN | `POST /api/v1/auth/tenants` | No regression — sees only allowed tenants |

### Ticket Creation Tests

| Test | Expected |
|------|----------|
| HOST creates ticket via REST | `tenant_id != NULL` in DB |
| HOST creates ticket via web UI | `tenant_id != NULL` in DB |
| RAG inference after ticket creation | `internal_notes` populated |
| USER creates ticket | No regression — JWT tenant used |

### Manual Verification

```bash
# 1. Start server
task run:rag

# 2. Sign in as HOST (gRPC via web UI or REST)
curl -c cookies.txt -X POST http://localhost:8081/api/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{"username":"ibm2100","password":"<password>"}'

# 3. Verify HOST can list tenants
curl -b cookies.txt -X POST http://localhost:8081/api/v1/auth/tenants \
  -H "Content-Type: application/json" \
  -d '{"username":"ibm2100","password":"<password>"}'
# Expected: HTTP 200 with tenant list

# 4. Decode JWT and verify tenant_id is present
# JWT is in the cookie; decode headers to confirm tenant_id is not nil

# 5. Create ticket
curl -b cookies.txt -X POST http://localhost:8081/api/v1/tickets \
  -H "Content-Type: application/json" \
  -d '{"title":"Test Ticket","description":"/m/test","status":"OPEN","priority":"MEDIUM","type":"TASK"}'

# 6. Verify ticket tenant_id
sqlite3 build/data/memos_dev.db \
  "SELECT id, tenant_id, internal_notes FROM tickets ORDER BY id DESC LIMIT 1;"
# Expected: tenant_id is not NULL, internal_notes is populated
```

---

## 7. Edge Cases

| Case | Behavior |
|------|----------|
| HOST with no tenants in system | `doSignIn`: JWT has nil `tenant_id` (no regression). `HandleAuthTenants`: returns HTTP 200 with empty `tenants: []`. |
| HOST with 1 tenant | Auto-selected on sign-in. `HandleAuthTenants` returns list of 1. |
| HOST with multiple tenants | Returns `"multiple tenants"` error on gRPC sign-in. Frontend should trigger REST `/auth/tenants` flow. |
| Scoped ADMIN with `AllowedTenantIDs` | `HandleAuthTenants` uses permission-row path. No access to unrelated tenants. |
| User with empty `AllowedTenantIDs` | Treated as super user (existing behavior). |
| `select-tenant` after sign-in | JWT carries explicitly selected tenant. `doSignIn` does not override explicit `tenantID`. |

---

## 8. Consumers

**JWT consumer:** `AuthMiddleware` at `v1.go:542`
```go
if claims.TenantID != nil {
    c.Set(getTenantIDContextKey(), *claims.TenantID)
}
```

With this fix, HOST JWT now carries a valid `tenant_id` instead of nil, so downstream context setup works correctly.

**RAG consumers:**
- `IndexTicketContent` (`ticket_service.go:178`) — guard `ticket.TenantID != nil` now passes for HOST-created tickets
- `InferResolutionForNewTicket` (`service.go:5596`) — early return on `nil` no longer triggers

**Web UI consumer:** `PasswordSignInForm.tsx:64-69` — now correctly falls back to REST `select-tenant` flow for HOST because `HandleAuthTenants` no longer returns 403.

---

## 9. Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|-----------|
| HOST JWT now has tenant_id | Medium | Audited downstream `TenantID == nil` checks: `InferResolutionForNewTicket`, `TenantBindingMiddleware`, `handleAutoTicketCreation`. All are correct with non-nil tenant. |
| HOST without perms auto-selects first tenant | Low | HOST can switch tenants via `/auth/tenants` + `select-tenant`. Behavior matches existing USER single-tenant auto-selection. |
| `HandleAuthTenants` returns 200 with empty list | Low | Only occurs when system has zero tenants. 200 with `tenants: []` is more useful than 403 for a super user. |
| Scoped admin isolation | None | Uses existing permission-row path. No change. |

---

## 10. What Changed vs Plan

| Plan item | Status |
|-----------|--------|
| Fix 1: `HandleAuthTenants` super-user bypass | ✅ Implemented as planned |
| Fix 2: `doSignIn` HOST/ADMIN tenant resolution | ✅ Implemented as planned |
| Fix 3: Document behavior change | ✅ Documented in §4 and §7 |
| Fix 1 migration | ❌ Dropped — code fixes handle nil perms gracefully |
| Fix 4-6 (auto-ticket, request field, UI) | ❌ Deferred — out of scope for this bug |

---

## 11. References

- `doSignIn`: `server/router/api/v1/auth_service.go:174`
- `HandleAuthTenants`: `server/router/api/v1/auth_service.go:428-463`
- `isSuperUser`: `server/router/api/v1/common.go:68`
- `store.IsSuperUser`: `store/user.go:70`
- `TenantBindingMiddleware`: `server/router/api/v1/tenant_binding.go:16`
- `CreateTicket`: `server/router/api/v1/ticket_service.go:62`
- `IndexTicketContent`: `ticket_service.go:178`
- `InferResolutionForNewTicket`: `server/router/api/v1/agent/service.go:5596`
- `handleAutoTicketCreation`: `server/router/api/v1/memo_service.go:1123`
