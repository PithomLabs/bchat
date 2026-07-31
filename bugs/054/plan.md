# Bug 054: Ticket Tenant Association Missing — NULL tenant_id Prevents RAG Inference

**Created:** 2026-07-31
**Status:** DRAFT — Awaiting review
**Related:** [Bug 052](../052/) | [Bug 053](../053/)

---

## 1. Background Context

### 1.1 What Happened

The user (ibm2100, HOST role) created ticket #169 via the web UI REST API (`POST /api/v1/tickets`). The ticket was created successfully in SQLite but `internal_notes` remained empty, despite Bug 052's RAG-based `InferResolutionForNewTicket` being fully implemented and approved.

### 1.2 Investigation Summary

I traced the full request flow from authentication through ticket creation to RAG inference. Here is what I found:

#### Authentication Layer — HOST Gets nil tenant_id in JWT

**gRPC `SignIn` path** (the default web UI login):
- `PasswordSignInForm.tsx:60` calls `authServiceClient.signIn()` via gRPC
- `doSignIn()` at `auth_service.go:174-190` has a role check: `if user.Role == store.RoleUser { ... }`
- This tenant resolution block ONLY runs for `RoleUser`. HOST and ADMIN roles are **skipped entirely**
- `tenantID` remains `nil` (the value from the gRPC request)
- JWT is generated at line 192 with `nil` tenant_id: `GenerateAccessToken(user.Email, user.ID, tenantID, ...)`

**REST `HandleSignIn` path**:
- `HandleSignIn` at `auth_service.go:644` always passes `nil` for tenant_id:
  ```go
  accessToken, err := GenerateAccessToken(user.Email, user.ID, nil, expireTime, []byte(s.Secret))
  ```
- This is "by design" per AGENTS.md — REST sign-in creates an unscoped session

**REST `HandleAuthTenants` path** (multi-tenant selection):
- `HandleAuthTenants` at `auth_service.go:406-413` queries `ListUserTenantPermissions` for the user
- HOST user (ibm2100, ID=1) has **ZERO rows** in `user_tenant_permission` table (verified via SQLite query)
- When `len(perms) == 0`, it returns HTTP 403: `"user is not associated with any company"`
- **HOST is completely locked out of the tenant-selection flow**

**Web UI `PasswordSignInForm.tsx:64-69`**:
- The frontend only falls back to REST `select-tenant` flow if gRPC returns "multiple tenants" error
- Since `doSignIn` doesn't error for HOST (it succeeds with nil tenant), the frontend never triggers the REST flow
- Even if it did, `HandleAuthTenants` would return 403

#### Database State

```
user_tenant_permission table:
+----+---------+-----------+
| id | user_id | tenant_id |
+----+---------+-----------+
|  2 |       2 |         7 |  (ate -> scraper)
+----+---------+-----------+

HOST user (ibm2100, ID=1): 0 permission rows
USER ading (ID=3): 0 permission rows
USER ading2 (ID=4): 0 permission rows
```

#### Signup Flow — Never Creates Permission Rows

`HandleSignUp` at `auth_service.go:538-597`:
- Creates the user with `RoleHost` if no HOST exists yet (line 575-576)
- Generates JWT with `nil` tenant_id (line 594)
- **Never creates `user_tenant_permission` rows**

#### Tenant Creation — Never Creates Permission Rows for HOST

`HandleCreateTenant` at `handlers.go:1395-1474`:
- Creates the `AgentTenant` record
- Processes KB/Policy files
- **Never creates `user_tenant_permission` rows for the HOST user**

#### Ticket Creation Layer — No tenant selector

`CreateTicketRequest` at `ticket_service.go:32-40`:
```go
type CreateTicketRequest struct {
    Title       string   `json:"title"`
    Description string   `json:"description"`
    Status      string   `json:"status"`
    Priority    string   `json:"priority"`
    Type        string   `json:"type"`
    Tags        []string `json:"tags"`
    AssigneeID  *int32   `json:"assigneeId"`
}
```
- **No `tenant_id` field** — tenant is derived entirely server-side from JWT

`CreateTicket` handler at `ticket_service.go:83-95`:
```go
ticket := &store.Ticket{
    // ... fields ...
    TenantID: getTenantFromContext(c),  // Returns nil when JWT has no tenant_id
}
```

`Tickets.tsx` (ticket creation modal):
- Form fields: Title, Type, Status, Priority, Assignee, Description
- **No tenant dropdown** — tenant is implicit server-side
- Request payload: `{ title, description, status, priority, type, assigneeId }`

#### Auto-Ticket Creation — Also Missing Tenant

`handleAutoTicketCreation` at `memo_service.go:1163-1173`:
```go
ticket := &store.Ticket{
    Title:       title,
    Description: "/m/" + memo.UID,
    Status:      store.TicketStatusOpen,
    Priority:    priority,
    Type:        ticketType,
    Tags:        tags,
    CreatorID:   user.ID,
    CreatedTs:   time.Now().Unix(),
    UpdatedTs:   time.Now().Unix(),
    // NOTE: TenantID is NOT set — memo.TenantID is available but not used
}
```

#### RAG Inference — Skipped Due to nil TenantID

`IndexTicketContent` at `ticket_service.go:178`:
```go
if s.agentHandler != nil && ticket.TenantID != nil {  // Guard: skips if nil
    go func() {
        _, err := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, nil, true)
    }()
}
```

`InferResolutionForNewTicket` at `service.go:5597-5600`:
```go
func (s *Service) InferResolutionForNewTicket(ctx context.Context, ticket *store.Ticket) {
    if ticket.TenantID == nil {
        return  // Early return — no inference
    }
    // ...
}
```

### 1.3 Root Cause Chain (Complete)

```
+-------------------------------------------------------------------+
| 1. HOST user has 0 rows in user_tenant_permission                  |
|    (signup flow never creates them, tenant creation never adds)    |
|                                                                    |
| 2. doSignIn() skips tenant resolution for RoleUser only             |
|    HOST gets nil tenant_id in JWT                                   |
|                                                                    |
| 3. HandleAuthTenants returns 403 for HOST (no perm rows)           |
|    HOST can't reach select-tenant flow                              |
|                                                                    |
| 4. Web UI ticket modal has no tenant dropdown                       |
|    CreateTicketRequest has no tenant_id field                       |
|                                                                    |
| 5. CreateTicket sets TenantID from JWT context -> nil               |
|    Ticket saved with tenant_id = NULL                               |
|                                                                    |
| 6. IndexTicketContent guarded: ticket.TenantID != nil -> skipped    |
|                                                                    |
| 7. InferResolutionForNewTicket: ticket.TenantID == nil -> return    |
|                                                                    |
| 8. internal_notes stays empty                                       |
+-------------------------------------------------------------------+
```

---

## 2. Solution Design

### 2.1 Guiding Principle

HOST should have **one associated tenant** just like every other user, with a `user_tenant_permission` row. The auth flow should handle HOST the same way it handles USER — no special casing.

### 2.2 Fix 1: Seed HOST Permission Row (Data Migration)

**Files:** `store/migration/sqlite/` and `store/migration/postgres/` (new migration)

**What:** Create a `user_tenant_permission` row for the HOST user (user_id=1) pointing to the first tenant, if none exists.

```sql
-- SQLite
INSERT OR IGNORE INTO user_tenant_permission (user_id, tenant_id, role, source_template_id)
SELECT 1, id, 'admin', NULL
FROM agent_tenants
WHERE id = (SELECT MIN(id) FROM agent_tenants)
AND NOT EXISTS (
    SELECT 1 FROM user_tenant_permission WHERE user_id = 1
);

-- Postgres
INSERT INTO user_tenant_permission (user_id, tenant_id, role, source_template_id)
SELECT 1, id, 'admin', NULL
FROM agent_tenants
WHERE id = (SELECT MIN(id) FROM agent_tenants)
AND NOT EXISTS (
    SELECT 1 FROM user_tenant_permission WHERE user_id = 1
)
ON CONFLICT DO NOTHING;
```

**Why:** HOST needs at least one permission row to pass `HandleAuthTenants` validation. Without this, the fix in step 2 has no data to work with.

### 2.3 Fix 2: Make `HandleAuthTenants` Handle HOST/ADMIN

**File:** `server/router/api/v1/auth_service.go`
**Lines:** 406-427

**What:** After credential validation, check if user is HOST/ADMIN and bypass the `user_tenant_permission` query. Return ALL tenants directly.

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
// HOST/ADMIN bypass — they have access to all tenants
if user.Role == store.RoleHost || user.Role == store.RoleAdmin {
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
    // Existing user_tenant_permission query for USER role
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

### 2.4 Fix 3: Make `doSignIn` Handle HOST/ADMIN

**File:** `server/router/api/v1/auth_service.go`
**Lines:** 174-190

**What:** Extend the tenant resolution logic to cover HOST/ADMIN roles. When HOST has permission rows, auto-select single tenant. When HOST has multiple, return "multiple tenants" error so frontend triggers the selection flow.

**Recommendation:** Extend existing USER logic to HOST/ADMIN. The JWT will have a valid `tenant_id` just like any other user. The user can switch tenants later via `select-tenant` if they have multiple.

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
    // ...
}
```

**After:**
```go
func (s *APIV1Service) doSignIn(ctx context.Context, user *store.User, tenantID *int32, expireTime time.Time) error {
    // All users must have at least one tenant association to log in.
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
    // ...
}
```

### 2.5 Fix 4: Set `TenantID` from Memo in `handleAutoTicketCreation`

**File:** `server/router/api/v1/memo_service.go`
**Lines:** 1163-1173

**What:** The memo struct already has a `TenantID` field. Pass it to the ticket.

**Before:**
```go
ticket := &store.Ticket{
    Title:       title,
    Description: "/m/" + memo.UID,
    Status:      store.TicketStatusOpen,
    Priority:    priority,
    Type:        ticketType,
    Tags:        tags,
    CreatorID:   user.ID,
    CreatedTs:   time.Now().Unix(),
    UpdatedTs:   time.Now().Unix(),
}
```

**After:**
```go
ticket := &store.Ticket{
    Title:       title,
    Description: "/m/" + memo.UID,
    Status:      store.TicketStatusOpen,
    Priority:    priority,
    Type:        ticketType,
    Tags:        tags,
    CreatorID:   user.ID,
    CreatedTs:   time.Now().Unix(),
    UpdatedTs:   time.Now().Unix(),
    TenantID:    memo.TenantID, // Propagate tenant from memo
}
```

### 2.6 Fix 5: Add `tenant_id` to `CreateTicketRequest`

**File:** `server/router/api/v1/ticket_service.go`
**Lines:** 32-40, 83-95

**What:** Add optional `TenantID` field to `CreateTicketRequest`. For HOST/ADMIN users, allow specifying which tenant the ticket belongs to. For regular users, this field is ignored (JWT context is authoritative).

**Before:**
```go
type CreateTicketRequest struct {
    Title       string   `json:"title"`
    Description string   `json:"description"`
    Status      string   `json:"status"`
    Priority    string   `json:"priority"`
    Type        string   `json:"type"`
    Tags        []string `json:"tags"`
    AssigneeID  *int32   `json:"assigneeId"`
}
```

**After:**
```go
type CreateTicketRequest struct {
    Title       string   `json:"title"`
    Description string   `json:"description"`
    Status      string   `json:"status"`
    Priority    string   `json:"priority"`
    Type        string   `json:"type"`
    Tags        []string `json:"tags"`
    AssigneeID  *int32   `json:"assigneeId"`
    TenantID    *int32   `json:"tenantId"` // Optional: for multi-tenant HOST/ADMIN
}
```

**Handler change:**
```go
tenantID := getTenantFromContext(c)
// If JWT has no tenant but request provides one, use it (HOST/ADMIN only)
if tenantID == nil && request.TenantID != nil && isSuperUser(user) {
    // Validate the tenant exists
    tenant, err := s.Store.GetAgentTenant(ctx, &store.FindAgentTenant{ID: request.TenantID})
    if err != nil || tenant == nil {
        return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant_id")
    }
    tenantID = request.TenantID
}
```

### 2.7 Fix 6: Add Tenant Dropdown to Ticket Creation Modal

**File:** `web/src/pages/Tickets.tsx`
**Lines:** 498-673

**What:** Add a tenant `<Select>` dropdown in the ticket creation form, visible only for HOST/ADMIN users.

**Requirements:**
- Show only for users with access to multiple tenants (HOST/ADMIN)
- Default to the currently selected tenant (from JWT/localStorage)
- Send `tenantId` in the request payload
- Add translation key for the dropdown label

**Implementation approach:**
1. Fetch tenant list on component mount (using existing tenant API)
2. Add `tenantId` state variable
3. Add `<Select>` dropdown between Title and Type fields (admin only section)
4. Include `tenantId` in the payload: `{ ...payload, tenantId: selectedTenantId }`

**Files to also update:**
- `web/src/locales/en.json` — add translation key for "Tenant" / "Company"

---

## 3. Implementation Order

| Step | Fix | Description | Dependencies |
|------|-----|-------------|-------------|
| 1 | Fix 1 | Seed HOST permission row (data migration) | None |
| 2 | Fix 2 | `HandleAuthTenants` HOST bypass | Fix 1 (or code-level fallback) |
| 3 | Fix 3 | `doSignIn` HOST/ADMIN tenant resolution | Fix 1 (or code-level fallback) |
| 4 | Fix 4 | `handleAutoTicketCreation` tenant propagation | None |
| 5 | Fix 5 | `CreateTicketRequest.tenantId` backend | None |
| 6 | Fix 6 | UI tenant dropdown | Fix 5 |

Steps 1-3 can be combined: Fix 2+3 are code-level fallbacks that make Fix 1 optional. If we implement Fix 2+3, the migration becomes a nice-to-have for data consistency.

---

## 4. Testing Plan

### 4.1 Auth Flow Tests

| Test Case | Expected Result |
|-----------|----------------|
| HOST gRPC sign-in | JWT contains `tenant_id` (not nil) |
| HOST REST sign-in | JWT contains `tenant_id` (not nil) |
| HOST `POST /api/v1/auth/tenants` | Returns all tenants (not 403) |
| HOST `POST /api/v1/auth/select-tenant` | Returns new JWT with selected `tenant_id` |
| USER gRPC sign-in (single tenant) | JWT contains `tenant_id` (no regression) |
| USER gRPC sign-in (multiple tenants) | Returns "multiple tenants" error (no regression) |
| USER with no perms | Returns 403 (no regression) |

### 4.2 Ticket Creation Tests

| Test Case | Expected Result |
|-----------|----------------|
| Create ticket as HOST via REST | `tenant_id != NULL` in DB |
| Create ticket with explicit `tenantId` | Correct tenant in DB |
| Create ticket as USER | JWT tenant used (request body `tenantId` ignored) |
| Create ticket with invalid `tenantId` | HTTP 400 |
| `handleAutoTicketCreation` | `tenant_id` from memo propagated to ticket |

### 4.3 RAG Inference Tests

| Test Case | Expected Result |
|-----------|----------------|
| Create ticket -> check `internal_notes` | Populated (not empty) |
| Verify `InferResolutionForNewTicket` called | Not skipped |
| Verify `IndexTicketContent` called | Not skipped |
| Verify vector DB has ticket content | Searchable |

### 4.4 Edge Case Tests

| Test Case | Expected Result |
|-----------|----------------|
| HOST with no tenants | Graceful error (no panic) |
| HOST with 1 tenant | Auto-selected, no prompt |
| HOST with multiple tenants | Selection flow triggered |
| Create ticket without auth | HTTP 401 |
| Create ticket as archived user | HTTP 403 |

### 4.5 Manual Verification Script

```bash
# 1. Verify HOST can sign in and get tenant_id in JWT
curl -c cookies.txt -X POST http://localhost:5230/api/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{"username":"ibm2100","password":"..."}'

# 2. Verify HOST can list tenants
curl -b cookies.txt http://localhost:5230/api/v1/auth/tenants \
  -X POST -H "Content-Type: application/json" \
  -d '{"username":"ibm2100","password":"..."}'

# 3. Create ticket and verify tenant_id
curl -b cookies.txt -X POST http://localhost:5230/api/v1/tickets \
  -H "Content-Type: application/json" \
  -d '{"title":"Test Ticket","description":"Test","status":"OPEN","priority":"MEDIUM","type":"TASK"}'

# 4. Check ticket in DB
sqlite3 build/data/memos_dev.db "SELECT id, tenant_id, internal_notes FROM tickets ORDER BY id DESC LIMIT 1;"

# 5. Verify internal_notes populated
# Should show RAG-inferred notes, not empty
```

---

## 5. Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Fix 3 changes gRPC sign-in behavior for HOST | Medium | HOST gets valid tenant_id in JWT — better than nil. If tenant doesn't exist, error is returned. |
| Migration creates permission row for wrong tenant | Low | Migration uses MIN(id) from agent_tenants — always the first tenant |
| UI dropdown breaks for users without multiple tenants | Low | Only shown for HOST/ADMIN with multiple tenants |
| `HandleAuthTenants` returns all tenants for HOST | Low | HOST is super-user — intended behavior |

---

## 6. Files to Modify

| File | Fix | Lines Changed |
|------|-----|--------------|
| `server/router/api/v1/auth_service.go` | Fix 2, Fix 3 | ~30 lines |
| `server/router/api/v1/ticket_service.go` | Fix 5 | ~10 lines |
| `server/router/api/v1/memo_service.go` | Fix 4 | 1 line |
| `web/src/pages/Tickets.tsx` | Fix 6 | ~40 lines |
| `web/src/locales/en.json` | Fix 6 | 1 line |
| `store/migration/sqlite/NN__seed_host_permission.sql` | Fix 1 | ~5 lines |
| `store/migration/postgres/NN__seed_host_permission.sql` | Fix 1 | ~5 lines |

---

## 7. Open Questions

1. **Fix 1 vs code-level fallback:** Should we create a migration to seed HOST permission rows, or rely on the code-level fixes (Fix 2+3) to handle HOST without permission rows? My recommendation: do both — migration for data consistency, code fixes for robustness.

2. **Auto-select tenant for HOST:** When HOST has multiple tenants and signs in via gRPC, should we auto-select the first tenant or always require selection? My recommendation: auto-select first (HOST can switch later via `select-tenant`).

3. **Tenant dropdown default:** Should the dropdown default to the currently selected tenant (from JWT) or the first tenant in the list? My recommendation: default to current tenant.
