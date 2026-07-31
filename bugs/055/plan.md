# Bug 055: Ticket List Filtered by JWT Tenant — HOST Cannot See All Tickets

**Created:** 2026-07-31
**Status:** DRAFT — Awaiting adversarial review
**Related:** [Bug 054](../054/) (plan2 fix introduced this regression)
**Severity:** High

---

## 1. Background Context

### 1.1 Problem

After the plan2 fix (Bug 054a), HOST JWT now carries a specific `tenant_id` (auto-selected first tenant = 7/scraper). This caused two regressions:

1. **Ticket list filtering**: `ApplyTicketTenantFilter` filters tickets by JWT tenant, so HOST only sees tickets from tenant 7, not from all tenants across the system.
2. **Tenant switching**: `HandleSelectTenant` checks `user_tenant_permission` rows — HOST has 0 rows → returns 403. HOST cannot switch tenants via the UI dropdown.

### 1.2 Root Cause Analysis

**Root cause 1 — `ApplyTicketTenantFilter` doesn't skip for super users:**

```go
// tenant_context.go:92-106
func ApplyTicketTenantFilter(c echo.Context, s *store.Store, find *store.FindTicket) {
    tenantID := getTenantFromContext(c)
    if tenantID != nil {
        find.TenantID = tenantID  // BUG: Filters by JWT tenant even for super users
        return
    }
    // ... scoped admin handling
}
```

**Before plan2 fix**: HOST JWT had `tenant_id = nil` → `getTenantFromContext(c)` returned nil → no filter → HOST saw all tickets.
**After plan2 fix**: HOST JWT has `tenant_id = 7` → filter applied → HOST only sees tenant 7 tickets.

**Root cause 2 — `HandleSelectTenant` doesn't bypass permission check for super users:**

```go
// auth_service.go:529-536
perm, err := s.Store.GetUserTenantPermission(ctx, &store.FindUserTenantPermission{
    UserID:   &matchedUser.ID,
    TenantID: &req.TenantID,
})
if err != nil || perm == nil {
    return echo.NewHTTPError(http.StatusForbidden, "user does not have access to this tenant")
}
```

HOST has 0 rows in `user_tenant_permission` → always returns 403 → HOST cannot switch tenants.

### 1.3 Current HOST Flow (Broken)

```
HOST signs in (gRPC) -> doSignIn auto-selects tenant 7 -> JWT has tenant_id=7
-> Opens ticket dropdown -> GET /api/v1/agent/tenants works (super user bypass)
-> Selects tenant 12 -> POST /api/v1/auth/select-tenant -> 403 (no permission row)
-> HOST stuck on tenant 7 -> Ticket list filtered to tenant 7 only
-> Ticket #171 (tenant 12) invisible
```

### 1.4 Database State

```
Tickets table (relevant):
+-----+-----------+----------------------------------+
| id  | tenant_id | title                            |
+-----+-----------+----------------------------------+
| 171 |        12 | Ticket Tenant Association Missing|
+-----+-----------+----------------------------------+

HOST user (ibm2100, ID=1): 0 rows in user_tenant_permission
```

### 1.5 Key Code References

| File | Line | Function | Issue |
|------|------|----------|-------|
| `tenant_context.go` | 92 | `ApplyTicketTenantFilter` | Doesn't skip for super users |
| `auth_service.go` | 530 | `HandleSelectTenant` | Permission check fails for HOST |
| `ticket_service.go` | 251 | `ListTickets` | Calls `ApplyTicketTenantFilter` |
| `v1.go` | 201-204 | `ticketGroup` | Only `AuthMiddleware`, no `TenantBindingMiddleware` |
| `v1.go` | 544-547 | `AuthMiddleware` | Sets tenant context from JWT claims |
| `store/user.go` | 70 | `IsSuperUser` | Returns true for HOST |
| `common.go` | 68 | `isSuperUser` | Delegates to `store.IsSuperUser` |

---

## 2. Solution Design

### 2.1 Guiding Principle

Super users (HOST, unscoped ADMIN) should see all tickets across all tenants, matching the pre-plan2 behavior. The ticket list is an internal admin view, not a per-tenant view.

### 2.2 Fix 1: Skip Tenant Filter for Super Users

**File:** `server/router/api/v1/tenant_context.go`
**Lines:** 92-96
**Severity:** CRITICAL

**What:** Add `isSuperUser` check at the top of `ApplyTicketTenantFilter` to bypass tenant filtering entirely for super users.

**Before:**
```go
func ApplyTicketTenantFilter(c echo.Context, s *store.Store, find *store.FindTicket) {
    tenantID := getTenantFromContext(c)
    if tenantID != nil {
        find.TenantID = tenantID
        return
    }
    // H2 Part B: For scoped admins, derive filter from AllowedTenantIDs
    user := getUserFromContext(c)
    if user != nil {
        tenantIDs := deriveTenantIDsForScopedAdmin(c.Request().Context(), s, user)
        if tenantIDs != nil {
            find.TenantIDs = tenantIDs
        }
    }
}
```

**After:**
```go
func ApplyTicketTenantFilter(c echo.Context, s *store.Store, find *store.FindTicket) {
    // Super users (HOST, unscoped ADMIN) see all tickets across all tenants
    user := getUserFromContext(c)
    if user != nil && isSuperUser(user) {
        return
    }
    tenantID := getTenantFromContext(c)
    if tenantID != nil {
        find.TenantID = tenantID
        return
    }
    // H2 Part B: For scoped admins, derive filter from AllowedTenantIDs
    if user != nil {
        tenantIDs := deriveTenantIDsForScopedAdmin(c.Request().Context(), s, user)
        if tenantIDs != nil {
            find.TenantIDs = tenantIDs
        }
    }
}
```

**Why this is correct:**
- `isSuperUser` is defined in `common.go:68` and delegates to `store.IsSuperUser` (`store/user.go:70`)
- `store.IsSuperUser` returns true for `RoleHost` OR unscoped `RoleAdmin`
- Scoped admins (with `AllowedTenantIDs`) are NOT super users — they still get filtered
- Pre-plan2 behavior: HOST JWT had nil tenant → no filter → saw all tickets (this restores that)
- `ListTickets` already has `isSuperUser` check at line 245 for `CreatorID` filter — consistent pattern

### 2.3 Fix 2: Bypass Permission Check for Super Users in `HandleSelectTenant`

**File:** `server/router/api/v1/auth_service.go`
**Lines:** 529-536
**Severity:** HIGH

**What:** Skip the `GetUserTenantPermission` check for super users, since they have implicit access to all tenants.

**Before:**
```go
// Verify user has access to the target tenant
perm, err := s.Store.GetUserTenantPermission(ctx, &store.FindUserTenantPermission{
    UserID:   &matchedUser.ID,
    TenantID: &req.TenantID,
})
if err != nil || perm == nil {
    return echo.NewHTTPError(http.StatusForbidden, "user does not have access to this tenant")
}
```

**After:**
```go
// Verify user has access to the target tenant
// Super users have implicit access to all tenants (no permission rows needed)
if !isSuperUser(matchedUser) {
    perm, err := s.Store.GetUserTenantPermission(ctx, &store.FindUserTenantPermission{
        UserID:   &matchedUser.ID,
        TenantID: &req.TenantID,
    })
    if err != nil || perm == nil {
        return echo.NewHTTPError(http.StatusForbidden, "user does not have access to this tenant")
    }
}
```

**Why this is correct:**
- `isSuperUser` check is already used in `HandleAuthTenants` (line 430) for the same purpose
- Consistent with `HandleAuthTenants` super user bypass pattern
- HOST/ADMIN should be able to select any tenant without explicit permission rows
- Non-super users still require permission rows (no regression)

### 2.4 Fix 3: Add Tenant Name to Ticket Response

**File:** `server/router/api/v1/ticket_service.go`
**Lines:** 17-30 (Ticket struct), 536-551 (convertTicketFromStore)
**Severity:** MEDIUM (UX improvement)

**What:** Add `TenantID` and `TenantName` fields to the `Ticket` response struct so the frontend can display which tenant a ticket belongs to.

**Ticket struct (before):**
```go
type Ticket struct {
    ID            int32    `json:"id"`
    Title         string   `json:"title"`
    Description   string   `json:"description"`
    Status        string   `json:"status"`
    Priority      string   `json:"priority"`
    CreatorID     int32    `json:"creatorId"`
    AssigneeID    *int32   `json:"assigneeId"`
    CreatedTs     int64    `json:"createdTs"`
    UpdatedTs     int64    `json:"updatedTs"`
    Type          string   `json:"type"`
    Tags          []string `json:"tags"`
    InternalNotes string   `json:"internalNotes"`
}
```

**Ticket struct (after):**
```go
type Ticket struct {
    ID            int32    `json:"id"`
    Title         string   `json:"title"`
    Description   string   `json:"description"`
    Status        string   `json:"status"`
    Priority      string   `json:"priority"`
    CreatorID     int32    `json:"creatorId"`
    AssigneeID    *int32   `json:"assigneeId"`
    CreatedTs     int64    `json:"createdTs"`
    UpdatedTs     int64    `json:"updatedTs"`
    Type          string   `json:"type"`
    Tags          []string `json:"tags"`
    InternalNotes string   `json:"internalNotes"`
    TenantID      *int32   `json:"tenantId"`
    TenantName    string   `json:"tenantName"`
}
```

**`convertTicketFromStore` (after):**
```go
func convertTicketFromStore(ticket *store.Ticket) *Ticket {
    return &Ticket{
        ID:            ticket.ID,
        Title:         ticket.Title,
        Description:   ticket.Description,
        Status:        string(ticket.Status),
        Priority:      string(ticket.Priority),
        CreatorID:     ticket.CreatorID,
        AssigneeID:    ticket.AssigneeID,
        CreatedTs:     ticket.CreatedTs,
        UpdatedTs:     ticket.UpdatedTs,
        Type:          ticket.Type,
        Tags:          ticket.Tags,
        InternalNotes: ticket.InternalNotes,
        TenantID:      ticket.TenantID,
    }
}
```

**`ListTickets` enrichment (after `convertTicketFromStore`):**
```go
// Batch-resolve tenant names for the response
tenantMap := make(map[int32]string)
for _, t := range result {
    if t.TenantID != nil {
        if _, ok := tenantMap[*t.TenantID]; !ok {
            tenant, err := s.Store.GetAgentTenant(ctx, &store.FindAgentTenant{ID: t.TenantID})
            if err == nil && tenant != nil {
                tenantMap[*t.TenantID] = tenant.CompanyName
            }
        }
    }
}
for _, t := range result {
    if t.TenantID != nil {
        t.TenantName = tenantMap[*t.TenantID]
    }
}
```

### 2.5 Fix 4: Frontend — Show Tenant Name in Ticket List

**File:** `web/src/pages/Tickets.tsx`
**Lines:** 20-32 (Ticket interface), 428-436 (table header), 440-489 (table row)
**Severity:** MEDIUM (UX improvement)

**What:** Add `tenantId` and `tenantName` to the frontend `Ticket` interface, and render a "Tenant" column in the ticket list table (visible to admin users).

**Ticket interface (after):**
```typescript
interface Ticket {
    id: number;
    title: string;
    description: string;
    status: string;
    priority: string;
    type?: string;
    creatorId: number;
    assigneeId?: number;
    createdTs: number;
    updatedTs: number;
    tags?: string[];
    tenantId?: number;
    tenantName?: string;
}
```

**Table header (after):**
```tsx
<th>Tenant</th>
```

**Table row (after):**
```tsx
<td>
    <span className="text-sm text-gray-600 dark:text-gray-400">
        {ticket.tenantName || `#${ticket.tenantId}`}
    </span>
</td>
```

---

## 3. Implementation Order

| Step | Fix | File | Description |
|------|-----|------|-------------|
| 1 | Fix 1 | `tenant_context.go:92-96` | Skip tenant filter for super users in `ApplyTicketTenantFilter` |
| 2 | Fix 2 | `auth_service.go:529-536` | Bypass permission check for super users in `HandleSelectTenant` |
| 3 | Fix 3 | `ticket_service.go:17-30, 258-274, 536-551` | Add `TenantID`/`TenantName` to Ticket response + batch resolve |
| 4 | Fix 4 | `Tickets.tsx:20-32, 428-436, 440-489` | Add tenant column to ticket list UI |

**Total:** ~40 lines changed across 4 files.

---

## 4. What This Plan Does NOT Cover

| Issue | Bug | Why Deferred |
|-------|-----|-------------|
| Migration to seed HOST permission rows | 054a-followup | Code fixes handle nil perms gracefully |
| Auto-ticket tenant propagation | 054b | Covered in plan3.md |
| Host tenant selection persistence | 055-followup | Could add tenant switcher to nav bar |
| Regular user tenant filtering | N/A | Already works correctly |

---

## 5. Testing Plan

### 5.1 Super User Ticket List

| Test Case | Expected Result |
|-----------|----------------|
| HOST views ticket list | Sees tickets from ALL tenants |
| HOST JWT has `tenant_id = 7` | Still sees ticket #171 (tenant 12) |
| HOST sees tenant names | Tenant column shows company names |
| Scoped admin views ticket list | Sees only allowed tenants' tickets (no regression) |
| Regular user views ticket list | Sees only own tickets (no regression) |

### 5.2 Tenant Selection

| Test Case | Expected Result |
|-----------|----------------|
| HOST calls `POST /api/v1/auth/select-tenant` | Returns new JWT with selected tenant |
| HOST switches to tenant 12 | JWT updated with `tenant_id = 12` |
| HOST switches back to tenant 7 | JWT updated with `tenant_id = 7` |
| Regular user calls select-tenant | Requires permission row (no regression) |

### 5.3 Manual Verification Script

```bash
# 1. Verify HOST can list tickets from all tenants
curl -b cookies.txt http://localhost:8081/api/v1/tickets | python3 -m json.tool
# Should show tickets from multiple tenants

# 2. Verify HOST can switch tenants
curl -b cookies.txt -X POST http://localhost:8081/api/v1/auth/select-tenant \
  -H "Content-Type: application/json" \
  -d '{"selection_token":"<token>","tenant_id":12}'
# Should return new JWT with tenant_id=12

# 3. Verify tenant names in ticket list
curl -b cookies.txt http://localhost:8081/api/v1/tickets | python3 -c "
import sys, json
for t in json.load(sys.stdin):
    print(f\"#{t['id']} [{t.get('tenantName', 'N/A')}] {t['title']}\")
"
```

---

## 6. Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|-----------|
| HOST sees all tickets (security) | Low | HOST is instance owner, already has full access |
| `HandleSelectTenant` bypass for super users | Low | Consistent with `HandleAuthTenants` pattern |
| New fields in Ticket response | None | Additive, backward compatible |
| Scoped admin regression | None | `isSuperUser` check excludes scoped admins |

---

## 7. Files to Modify

| File | Fix | Lines Changed |
|------|-----|--------------|
| `server/router/api/v1/tenant_context.go` | Fix 1 | ~5 lines |
| `server/router/api/v1/auth_service.go` | Fix 2 | ~3 lines |
| `server/router/api/v1/ticket_service.go` | Fix 3 | ~20 lines |
| `web/src/pages/Tickets.tsx` | Fix 4 | ~15 lines |

---

## 8. Adversarial Review Prompt

Before implementing, please review this plan critically:

1. **Security**: Does skipping the tenant filter for super users create any data leakage risk? Are there edge cases where a super user should NOT see all tickets?

2. **Regression**: Does Fix 1 or Fix 2 break any existing functionality for non-super users (regular users, scoped admins)?

3. **Edge cases**: What happens if `ApplyTicketTenantFilter` is called when `getUserFromContext` returns nil (e.g., unauthenticated request)? Does the nil check prevent issues?

4. **Performance**: Does batch-resolving tenant names in `ListTickets` add significant latency? Is the `tenantMap` approach efficient enough?

5. **Consistency**: Is the `isSuperUser` check in Fix 1 consistent with how `ApplyTenantFilter` (for memos) handles super users?

6. **Completeness**: Are there other places where `ApplyTicketTenantFilter` is called that might need similar fixes?

7. **Frontend**: Does adding a new column break the table layout? Is the column order optimal for the user?

8. **Alternative approaches**: Would it be better to add a `tenantId` query parameter to `ListTickets` instead of skipping the filter entirely? What are the tradeoffs?
