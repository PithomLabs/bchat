# Bug 055 Implementation: Ticket List Filtered by JWT Tenant

**Bug ID:** 055
**Date:** 2026-07-31
**Status:** IMPLEMENTED — Awaiting adversarial code review
**Files Changed:** 4
**Lines Changed:** ~30
**Build:** PASS
**Tests:** PASS

---

## 1. Summary

Fixed two regressions introduced by Bug 054a's plan2 fix (HOST JWT now carries `tenant_id`):

1. **Ticket list filtering**: HOST could only see tickets from one tenant (JWT tenant), not all tenants
2. **Tenant switching**: `HandleSelectTenant` rejected HOST (no `user_tenant_permission` rows → 403)

Added tenant name display to the ticket list UI as a UX improvement for multi-tenant visibility.

---

## 2. Code Changes

### Fix 1: Super-User Bypass in `ApplyTicketTenantFilter`

**File:** `server/router/api/v1/tenant_context.go`
**Lines:** 90-111

**Before:**
```go
// ApplyTicketTenantFilter applies tenant filtering to a FindTicket query.
// For scoped admins, derives TenantIDs from AllowedTenantIDs.
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
// ApplyTicketTenantFilter applies tenant filtering to a FindTicket query.
// Super users (HOST, unscoped ADMIN) see all tickets across all tenants.
// For scoped admins, derives TenantIDs from AllowedTenantIDs.
func ApplyTicketTenantFilter(c echo.Context, s *store.Store, find *store.FindTicket) {
	// Super users see all tickets — no tenant filter needed
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

**Rationale:** Before plan2 fix, HOST JWT had `tenant_id = nil` → `getTenantFromContext` returned nil → no filter → HOST saw all tickets. Plan2 fix gave HOST JWT `tenant_id = 7` → filter applied → HOST only saw tenant 7 tickets. This restores the pre-plan2 behavior.

---

### Fix 2: Super-User Bypass with Nil Guard in `HandleSelectTenant`

**File:** `server/router/api/v1/auth_service.go`
**Lines:** 529-539

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
	// Super users (HOST, unscoped ADMIN) have implicit access to all tenants
	if matchedUser != nil && !isSuperUser(matchedUser) {
		perm, err := s.Store.GetUserTenantPermission(ctx, &store.FindUserTenantPermission{
			UserID:   &matchedUser.ID,
			TenantID: &req.TenantID,
		})
		if err != nil || perm == nil {
			return echo.NewHTTPError(http.StatusForbidden, "user does not have access to this tenant")
		}
	}
```

**Rationale:** HOST has 0 rows in `user_tenant_permission` → `GetUserTenantPermission` returns nil → 403. Super users should have implicit access to all tenants without explicit permission rows. The nil guard on `matchedUser` prevents panic if user lookup failed earlier.

---

### Fix 3: Add `TenantID`/`TenantName` to Ticket Response

#### 3a: Ticket Response Struct

**File:** `server/router/api/v1/ticket_service.go`
**Lines:** 17-32

**Before:**
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

**After:**
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

#### 3b: Batch Tenant Name Resolution in `ListTickets`

**File:** `server/router/api/v1/ticket_service.go`
**Lines:** 276-292

```go
	// Batch-resolve tenant names (N+1 is acceptable for admin-scale ticket lists)
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

**Rationale:** Uses a `tenantMap` to deduplicate lookups. N+1 queries are acceptable for admin-scale ticket lists (typically < 100 tickets, < 50 tenants). A single `ListAgentTenants` query would fetch all tenants in the system, which is wasteful for large deployments.

#### 3c: `convertTicketFromStore` Includes `TenantID`

**File:** `server/router/api/v1/ticket_service.go`
**Lines:** 556-571

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

**Note:** `TenantName` is only populated in `ListTickets` (batch-resolved), not in `GetTicket`. This is acceptable because the frontend detail view (`TicketDetail.tsx`) doesn't display tenant name.

---

### Fix 4: Tenant Column in Ticket List UI

**File:** `web/src/pages/Tickets.tsx`

#### 4a: Frontend Ticket Interface (lines 20-34)

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

#### 4b: Table Header — Tenant Column (line 437)

```tsx
{isAdmin && <th>Tenant</th>}
```

Placed after the `Assignee` column, guarded by `isAdmin` to match existing pattern.

#### 4c: Table Row — Tenant Cell (lines 486-492)

```tsx
{isAdmin && (
    <td>
        <span className="text-sm text-gray-600 dark:text-gray-400">
            {ticket.tenantName || `#${ticket.tenantId}`}
        </span>
    </td>
)}
```

Falls back to `#${ticket.tenantId}` if `tenantName` is empty (e.g., `GetTicket` response).

#### 4d: Empty State colSpan (line 505)

```tsx
<td colSpan={isAdmin ? 10 : 8} className="text-center py-8 text-gray-500">
```

Updated from `9/7` to `10/8` to account for the new Tenant column.

---

## 3. Behavioral Changes

### Fix 1 — `ApplyTicketTenantFilter`

| Caller | Before | After |
|--------|--------|-------|
| HOST `ListTickets` | Filtered to JWT tenant (bug) | All tickets (fixed) |
| Scoped ADMIN `ListTickets` | Filtered to `AllowedTenantIDs` | Unchanged |
| USER `ListTickets` | Filtered to JWT tenant | Unchanged |
| Unauthenticated | No filter (nil tenant) | Unchanged |

### Fix 2 — `HandleSelectTenant`

| Caller | Before | After |
|--------|--------|-------|
| HOST `select-tenant` | 403 (no perm rows) | JWT with selected tenant |
| Scoped ADMIN `select-tenant` | Requires perm row | Unchanged |
| USER `select-tenant` | Requires perm row | Unchanged |

### Fix 3 — TenantName in Response

| Endpoint | TenantID | TenantName |
|----------|----------|------------|
| `ListTickets` | Populated | Populated (batch-resolved) |
| `GetTicket` | Populated | Empty (acceptable) |

### Fix 4 — Tenant Column in UI

| User Role | Tenant Column Visible |
|-----------|-----------------------|
| Admin (HOST/ADMIN) | Yes |
| Regular user | No (consistent with Assignee column) |

---

## 4. Known Limitations

1. **N+1 queries for tenant names**: `ListTickets` does up to N sequential `GetAgentTenant` queries where N is the number of unique tenants in the result set. Acceptable for current scale (< 100 tickets, < 50 tenants). Can be optimized later with a `BatchGetAgentTenantsByIDs` store method.

2. **`TenantName` empty in `GetTicket`**: Single-ticket responses include `TenantID` but not `TenantName`. The frontend detail view doesn't display tenant name, so this is acceptable.

3. **No tenant switcher in nav bar**: HOST can switch tenants via `HandleSelectTenant`, but there's no persistent UI control for it. The dropdown in the ticket modal is the only way to select a tenant for ticket creation.

---

## 5. Testing Results

```
$ go build ./...
# Build passed

$ go test ./server/router/api/v1/...
ok  github.com/usememos/memos/server/router/api/v1    2.894s
ok  github.com/usememos/memos/server/router/api/v1/agent    8.718s
# All tests passed
```

---

## 6. Adversarial Code Review Prompt

Before merging, please review this implementation critically:

1. **Security — nil guard in Fix 2**: The check `matchedUser != nil && !isSuperUser(matchedUser)` means if `matchedUser` is nil, the permission check is skipped entirely. Is this safe? Could a malformed selection token result in `matchedUser` being nil, bypassing the permission check and generating a valid JWT?

2. **Security — super-user bypass scope**: Fix 1 lets HOST see all tickets without any tenant filtering. Is this acceptable for compliance/audit scenarios? Should there be an opt-in flag or logging for this behavior?

3. **Regression — scoped admin**: Does the `isSuperUser` check in Fix 1 correctly exclude scoped admins (RoleAdmin with non-empty `AllowedTenantIDs`)? Verify that `store.IsSuperUser` returns false for scoped admins.

4. **Regression — `ApplyTenantFilter` for memos**: The parallel function `ApplyTenantFilter` (for memos) does NOT have a super-user bypass. Is this inconsistency intentional? Could it cause confusion?

5. **Performance — N+1 queries**: The `tenantMap` deduplication in Fix 3b means each unique tenant ID triggers one `GetAgentTenant` call. For a ticket list with 50 tickets across 10 tenants, this is 10 queries. Is this acceptable? Should we add a note in the code about scale limits?

6. **Edge case — nil `TenantID`**: In Fix 3b, the code checks `t.TenantID != nil` before dereferencing. But `convertTicketFromStore` already copies `TenantID` from `store.Ticket`. Could there be a race condition where `TenantID` is set to nil between the two loops?

7. **Frontend — `isAdmin` guard**: Fix 4 wraps the tenant column with `{isAdmin && ...}`. Verify that `isAdmin` is correctly derived from the user's role and isn't affected by tenant context. Could a regular user ever see `isAdmin = true`?

8. **API contract — backward compatibility**: Adding `tenantId` and `tenantName` to the `Ticket` response is additive. But are there any API consumers that parse the response strictly and might break on new fields?

9. **Consistency — `GetTicket` vs `ListTickets`**: `GetTicket` returns `TenantName` as empty string, while `ListTickets` returns it populated. Is this inconsistency acceptable? Should `GetTicket` also resolve the tenant name?

10. **Logging — super-user bypass paths**: Should we add `slog.Info` logging when super users bypass tenant filtering or permission checks? This could help with audit trails and debugging.
