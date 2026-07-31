# Bug 054 Plan 2: HOST JWT tenant_id = nil — Prevents Ticket RAG Inference

**Created:** 2026-07-31
**Status:** DRAFT — Awaiting review
**Supersedes:** [plan.md](./plan.md) (rejected per adversarial review)
**Related:** [Bug 052](../052/) | [Bug 053](../053/)
**Scope:** Bug 054a only (HOST JWT). Bug 054b (auto-ticket creation) tracked separately.

---

## 1. Background Context

### 1.1 Problem

Tickets created via the web UI have `tenant_id = NULL` for HOST users, causing:
- RAG indexing skipped (`IndexTicketContent` guarded by `ticket.TenantID != nil`)
- `InferResolutionForNewTicket` returns early → `internal_notes` stays empty
- Tickets orphaned from any tenant context

### 1.2 Investigation Summary

**Root cause:** HOST user (ibm2100, ID=1) gets `nil` `tenant_id` in JWT because:

1. **gRPC `SignIn` (`doSignIn` at `auth_service.go:174-190`)**: Tenant resolution only runs for `RoleUser`. HOST/ADMIN are skipped → JWT has `nil` tenant_id.

2. **REST `HandleSignIn` (`auth_service.go:644`)**: Always passes `nil` for tenant_id.

3. **REST `HandleAuthTenants` (`auth_service.go:406-413`)**: Queries `user_tenant_permission` table → HOST has 0 rows → returns **403** ("user is not associated with any company").

4. **Web UI `PasswordSignInForm.tsx:64-69`**: Only falls back to REST `select-tenant` on gRPC "multiple tenants" error. HOST succeeds with nil tenant → frontend never triggers selection.

**HOST is completely locked out of the tenant-selection flow.**

### 1.3 Database State

```
user_tenant_permission table:
+----+---------+-----------+
| id | user_id | tenant_id |
+----+---------+-----------+
|  2 |       2 |         7 |  (ate -> scraper)
+----+---------+-----------+

HOST user (ibm2100, ID=1): 0 permission rows
```

### 1.4 Root Cause Chain

```
HOST signs in -> doSignIn skips tenant resolution -> JWT has nil tenant_id
-> HandleAuthTenants returns 403 (no perm rows) -> HOST can't select tenant
-> Web UI ticket modal has no tenant dropdown -> POST /api/v1/tickets
-> getTenantFromContext(c) returns nil -> ticket.tenant_id = NULL
-> IndexTicketContent skipped -> InferResolutionForNewTicket returns early
-> internal_notes stays empty
```

---

## 2. Solution Design

### 2.1 Guiding Principle

Minimal 2-file fix in `auth_service.go` only. HOST gets a valid `tenant_id` in JWT on sign-in, consistent with how USER with a single tenant works.

### 2.2 Fix 1: Make `HandleAuthTenants` Handle Super Users

**File:** `server/router/api/v1/auth_service.go`
**Lines:** 406-427
**Severity:** CRITICAL

**What:** Use `isSuperUser` check (HOST or unscoped ADMIN) to return all tenants. Scoped admins and regular users use existing `user_tenant_permission` query.

**Before:**
```go
// Get tenant permissions
perms, err := s.Store.ListUserTenantPermissions(c.Request().Context(), &store.FindUserTenantPermission{UserID: &user.ID})
if err != nil {
    return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tenant permissions")
}
if len(perms) == 0 {
    return echo.NewHTTPError(http.StatusForbidden, "user is not associated with any company")
}

// Build tenant list
tenants := make([]TenantInfo, 0, len(perms))
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
```

**After:**
```go
var tenants []TenantInfo

// Super users (HOST or unscoped ADMIN) see all tenants
// NOTE: If no tenants exist, returns 200 with empty list (not error) — HOST is not "not associated", system is empty
if user.Role == store.RoleHost || (user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0) {
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

**Key change:** Uses `isSuperUser` check from `common.go:68` (equivalent to inline logic at `tenant_binding.go:37`).

### 2.3 Fix 2: Make `doSignIn` Resolve Tenant for HOST/ADMIN

**File:** `server/router/api/v1/auth_service.go`
**Lines:** 174-190
**Severity:** CRITICAL

**What:** Extend tenant resolution to cover HOST/ADMIN. When permission rows exist, auto-select single tenant. When no rows exist, auto-select first tenant from all tenants.

**Behavior change:** HOST JWT now carries a specific `tenant_id` instead of `nil`.

**Intentional behavior inconsistency (documented):**
HOST without explicit permission rows is treated as a super-user with implicit access to the first tenant. HOST with explicit permission rows must select or is auto-selected by count. This preserves backward compatibility for both managed and unmanaged HOST setups.

**Audit of `TenantID == nil` checks:**
- `InferResolutionForNewTicket` — this is the exact bug we're fixing; having a tenant is correct
- `TenantBindingMiddleware` — already handles HOST with or without tenant; slug-based endpoints get tenant from URL
- `handleAutoTicketCreation` — only called for non-superusers, not affected

**Before:**
```go
func (s *APIV1Service) doSignIn(ctx context.Context, user *store.User, tenantID *int32, expireTime time.Time) error {
    // External users MUST have a company association to log in.
    if user.Role == store.RoleUser {
        perms, err := s.Store.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{UserID: &user.ID})
        if err != nil {
            return status.Errorf(codes.Internal, "failed to verify user company association")
        }
        if len(perms) == 0 {
            return status.Errorf(codes.PermissionDenied, "user is not associated with any company")
        }
        // Auto-select single tenant if not already specified
        if tenantID == nil && len(perms) == 1 {
            tenantID = &perms[0].TenantID
        } else if tenantID == nil && len(perms) > 1 {
            return status.Errorf(codes.FailedPrecondition, "multiple tenants found, use /auth/tenants endpoint")
        }
    }

    accessToken, err := GenerateAccessToken(user.Email, user.ID, tenantID, expireTime, []byte(s.Secret))
```

**After:**
```go
func (s *APIV1Service) doSignIn(ctx context.Context, user *store.User, tenantID *int32, expireTime time.Time) error {
    // Resolve tenant for all user roles
    if user.Role == store.RoleHost || user.Role == store.RoleAdmin {
        // HOST/ADMIN: check for explicit permission rows first
        perms, err := s.Store.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{UserID: &user.ID})
        if err != nil {
            return status.Errorf(codes.Internal, "failed to verify tenant association")
        }
        if len(perms) > 0 {
            // Has explicit permission rows — use same logic as USER
            if tenantID == nil && len(perms) == 1 {
                tenantID = &perms[0].TenantID
            } else if tenantID == nil && len(perms) > 1 {
                return status.Errorf(codes.FailedPrecondition, "multiple tenants found, use /auth/tenants endpoint")
            }
        } else {
            // No permission rows — fall back to all tenants
            allTenants, err := s.Store.ListAgentTenants(ctx, &store.FindAgentTenant{})
            if err != nil {
                return status.Errorf(codes.Internal, "failed to list tenants")
            }
            if len(allTenants) > 0 {
                if tenantID == nil {
                    tenantID = &allTenants[0].ID // Auto-select first tenant
                }
            }
            // If no tenants exist, tenantID stays nil — acceptable for HOST
        }
    } else if user.Role == store.RoleUser {
        // Existing USER logic (unchanged)
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
```

---

## 3. Implementation Order

| Step | Fix | File | Description |
|------|-----|------|-------------|
| 1 | Fix 1 | `auth_service.go:406-427` | `HandleAuthTenants` super user bypass |
| 2 | Fix 2 | `auth_service.go:174-190` | `doSignIn` HOST/ADMIN tenant resolution |

**Total:** ~30 lines changed in 1 file.

---

## 4. What This Plan Does NOT Cover (Follow-up Tickets)

| Issue | Bug | Why Deferred |
|-------|-----|-------------|
| Migration to seed HOST permission row | 054a-followup | Code fixes handle nil perms gracefully |
| Auto-ticket creation tenant propagation | 054b | Only affects non-superusers, not HOST |
| `CreateTicketRequest.tenantId` field | 054c | Not needed if Fix 2 works |
| UI tenant dropdown | 054d | Depends on confirmed multi-tenant UX |

---

## 5. Testing Plan

### 5.1 Auth Flow Tests

| Test Case | Expected Result |
|-----------|----------------|
| HOST gRPC sign-in | JWT contains `tenant_id` (not nil) |
| HOST `POST /api/v1/auth/tenants` | Returns all tenants (not 403) |
| HOST `POST /api/v1/auth/select-tenant` | Returns new JWT with selected `tenant_id` |
| USER gRPC sign-in (single tenant) | JWT contains `tenant_id` (no regression) |
| USER gRPC sign-in (multiple tenants) | Returns "multiple tenants" error (no regression) |
| USER with no perms | Returns 403 (no regression) |
| Scoped ADMIN | Only sees allowed tenants (no regression) |

### 5.2 Ticket Creation Tests

| Test Case | Expected Result |
|-----------|----------------|
| Create ticket as HOST via REST | `tenant_id != NULL` in DB |
| Create ticket as HOST via UI | `tenant_id != NULL` in DB |
| Create ticket as USER | JWT tenant used (no regression) |

### 5.3 RAG Inference Tests

| Test Case | Expected Result |
|-----------|----------------|
| Create ticket -> check `internal_notes` | Populated (not empty) |
| Verify `InferResolutionForNewTicket` called | Not skipped |
| Verify `IndexTicketContent` called | Not skipped |

### 5.4 Manual Verification Script

```bash
# 1. Verify HOST can sign in and get tenant_id in JWT
curl -c cookies.txt -X POST http://localhost:5230/api/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{"username":"ibm2100","password":"..."}'
# Check JWT contains tenant_id in cookie

# 2. Verify HOST can list tenants
curl -b cookies.txt -X POST http://localhost:5230/api/v1/auth/tenants \
  -H "Content-Type: application/json" \
  -d '{"username":"ibm2100","password":"..."}'
# Should return all tenants, not 403

# 3. Create ticket and verify tenant_id
curl -b cookies.txt -X POST http://localhost:5230/api/v1/tickets \
  -H "Content-Type: application/json" \
  -d '{"title":"Test Ticket","description":"/m/test","status":"OPEN","priority":"MEDIUM","type":"TASK"}'

# 4. Check ticket in DB
sqlite3 build/data/memos_dev.db "SELECT id, tenant_id, internal_notes FROM tickets ORDER BY id DESC LIMIT 1;"
# Should show non-null tenant_id and populated internal_notes
```

---

## 6. Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|-----------|
| HOST JWT now has tenant_id (behavior change) | Medium | Audit confirms no downstream code relies on HOST having nil tenant |
| Auto-selects first tenant for HOST | Low | HOST can switch via select-tenant if needed |
| Scoped admin gets all tenants | None | Fix 1 uses `isSuperUser` check, not blanket `RoleAdmin` |

---

## 7. Files to Modify

| File | Fix | Lines Changed |
|------|-----|--------------|
| `server/router/api/v1/auth_service.go` | Fix 1, Fix 2 | ~30 lines |
