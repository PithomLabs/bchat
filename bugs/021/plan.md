# Implementation Plan: Memo & Ticket Tenant Isolation

**Date:** 2026-07-05
**Bug:** 021
**Scope:** Multi-tenant isolation for memos and tickets, disable public exposure
**Status:** Ready for implementation

---

## Context

The investigation (`docs_public.md`) identified that memos and tickets lack `tenant_id` fields, causing cross-tenant data leakage. PUBLIC memos are globally accessible to anonymous users, and PROTECTED memos leak across tenant boundaries. The bchat platform is not designed to publish memos to the internet.

### Decisions

| Decision | Choice |
|----------|--------|
| Visibility model | Option B — Keep PUBLIC/PROTECTED/PRIVATE, add `tenant_id` filtering |
| Tickets | Included in this plan |
| RSS feeds | Disabled entirely |
| Backward compatibility | NULL `tenant_id` for existing memos (invisible to tenants, visible to creator/super) |
| Escalation memos | Keep Protected + tenant filtering |
| Frontend defaults | Change ticket descriptions from PUBLIC to PROTECTED |

### Key Security Fixes

1. Add `tenant_id` to `memo` and `tickets` tables
2. Filter all memo/ticket queries by tenant
3. Disable RSS endpoints (no public publishing)
4. Fix ticket deduplication to scope by tenant
5. Enforce `DisallowPublicVisibility=true` as default

---

## Sprint 1: Database Migration

**Goal:** Add `tenant_id` column to `memo` and `tickets` tables.

### 1.1 SQLite Migration

**File:** `store/migration/sqlite/0.27/00__memo_ticket_tenant_isolation.sql`

```sql
-- Add tenant_id to memo table
ALTER TABLE memo ADD COLUMN tenant_id INTEGER DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_memo_tenant ON memo(tenant_id);

-- Add tenant_id to tickets table
ALTER TABLE tickets ADD COLUMN tenant_id INTEGER DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_tickets_tenant ON tickets(tenant_id);
```

### 1.2 Postgres Migration

**File:** `store/migration/postgres/0.27/00__memo_ticket_tenant_isolation.sql`

```sql
-- Add tenant_id to memo table
ALTER TABLE memo ADD COLUMN tenant_id INTEGER DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_memo_tenant ON memo(tenant_id);

-- Add tenant_id to tickets table
ALTER TABLE tickets ADD COLUMN tenant_id INTEGER DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_tickets_tenant ON tickets(tenant_id);
```

### 1.3 Update LATEST.sql

**Files:** `store/migration/sqlite/LATEST.sql`, `store/migration/postgres/LATEST.sql`

Add `tenant_id INTEGER DEFAULT NULL` to `memo` and `tickets` table definitions.

### 1.4 Verification

```bash
# Run migration against test database
sqlite3 build/data/memos_dev.db < store/migration/sqlite/0.27/00__memo_ticket_tenant_isolation.sql

# Verify columns exist
sqlite3 build/data/memos_dev.db ".schema memo" | grep tenant_id
sqlite3 build/data/memos_dev.db ".schema tickets" | grep tenant_id
```

---

## Sprint 2: Store Layer

**Goal:** Add `TenantID` to data structs and update all SQL queries to filter by tenant.

### 2.1 Update `store/memo.go`

**Add TenantID to Memo struct:**

```go
type Memo struct {
    ID         int32
    UID        string
    CreatorID  int32
    Content    string
    Visibility Visibility
    Payload    *MemoPayload
    TenantID   *int32  // NEW: nil for legacy memos
}
```

**Add TenantID to FindMemo struct:**

```go
type FindMemo struct {
    ID             *int32
    UID            *string
    CreatorID      *int32
    VisibilityList []Visibility
    // ... existing fields ...
    TenantID       *int32  // NEW: filter by tenant
}
```

**Add TenantID to UpdateMemo struct:**

```go
type UpdateMemo struct {
    ID         int32
    // ... existing fields ...
    TenantID   *int32  // NEW: allow setting tenant on update
}
```

### 2.2 Update `store/ticket.go`

**Add TenantID to Ticket struct:**

```go
type Ticket struct {
    ID          int32
    // ... existing fields ...
    TenantID    *int32  // NEW: nil for legacy tickets
}
```

**Add TenantID to FindTicket struct:**

```go
type FindTicket struct {
    ID       *int32
    // ... existing fields ...
    TenantID *int32  // NEW: filter by tenant
}
```

### 2.3 Update `store/db/sqlite/memo.go`

**CreateMemo — inject tenant_id:**

```go
func (d *DB) CreateMemo(ctx context.Context, memo *store.Memo) (*store.Memo, error) {
    // ... existing code ...
    result, err := d.db.ExecContext(ctx, `
        INSERT INTO memo (uid, creator_id, content, visibility, pinned, payload, tenant_id)
        VALUES (?, ?, ?, ?, ?, ?, ?)`,
        memo.UID, memo.CreatorID, memo.Content, memo.Visibility, memo.Pinned, payload, memo.TenantID,
    )
```

**ListMemos — add tenant filter:**

```go
func (d *DB) ListMemos(ctx context.Context, find *store.FindMemo) ([]*store.Memo, error) {
    // ... existing query building ...

    // NEW: Add tenant filter
    if find.TenantID != nil {
        where, args = append(where, "tenant_id = ?"), append(args, *find.TenantID)
    }

    // ... rest of query ...
}
```

**GetMemo — add tenant check:**

```go
func (d *DB) GetMemo(ctx context.Context, find *store.FindMemo) (*store.Memo, error) {
    // ... existing query ...

    // NEW: Add tenant filter if specified
    if find.TenantID != nil {
        where, args = append(where, "tenant_id = ?"), append(args, *find.TenantID)
    }
```

### 2.4 Update `store/db/sqlite/ticket.go`

**CreateTicket — inject tenant_id:**

```go
func (d *DB) CreateTicket(ctx context.Context, ticket *store.Ticket) (*store.Ticket, error) {
    result, err := d.db.ExecContext(ctx, `
        INSERT INTO tickets (subject, description, type, priority, status, creator_id, tenant_id)
        VALUES (?, ?, ?, ?, ?, ?, ?)`,
        ticket.Subject, ticket.Description, ticket.Type, ticket.Priority, ticket.Status, ticket.CreatorID, ticket.TenantID,
    )
```

**ListTickets — add tenant filter:**

```go
func (d *DB) ListTickets(ctx context.Context, find *store.FindTicket) ([]*store.Ticket, error) {
    // ... existing query building ...

    // NEW: Add tenant filter
    if find.TenantID != nil {
        where, args = append(where, "tenant_id = ?"), append(args, *find.TenantID)
    }
```

### 2.5 Update `store/db/sqlite/memo_filter.go`

Add `tenant_id` to the CEL filter expression support:

```go
// In the filter compilation, add support for "tenant_id" field
case "tenant_id":
    // Compile to SQL: tenant_id = ?
```

### 2.6 Repeat for Postgres

Mirror all SQLite changes in `store/db/postgres/memo.go`, `store/db/postgres/ticket.go`, and `store/db/postgres/memo_filter.go`.

### 2.7 Verification

```bash
go build ./store/...
go vet ./store/...
```

---

## Sprint 3: Service Layer

**Goal:** Inject tenant context into all memo/ticket operations, fix cross-tenant leaks.

### 3.1 Update `server/router/api/v1/memo_service.go`

**CreateMemo — set TenantID from context:**

```go
func (s *APIV1Service) CreateMemo(ctx context.Context, request *v1pb.CreateMemoRequest) (*v1pb.Memo, error) {
    // ... existing code ...

    // NEW: Get tenant from context (set by auth middleware or agent session)
    tenantID := getTenantFromContext(ctx)

    create := &store.Memo{
        UID:        shortuuid.New(),
        CreatorID:  user.ID,
        Content:    request.Memo.Content,
        Visibility: convertVisibilityToStore(request.Memo.Visibility),
        TenantID:   tenantID,  // NEW
    }

    // ... rest of existing code ...
}
```

**ListMemos — filter by tenant:**

```go
func (s *APIV1Service) ListMemos(ctx context.Context, request *v1pb.ListMemosRequest) (*v1pb.ListMemosResponse, error) {
    // ... existing code ...

    // NEW: Add tenant filter
    tenantID := getTenantFromContext(ctx)
    if tenantID != nil {
        memoFind.TenantID = tenantID
    }

    // ... existing visibility filtering logic ...
}
```

**GetMemo — verify tenant access:**

```go
func (s *APIV1Service) GetMemo(ctx context.Context, request *v1pb.GetMemoRequest) (*v1pb.Memo, error) {
    // ... existing code ...

    // NEW: Verify tenant access
    tenantID := getTenantFromContext(ctx)
    if tenantID != nil && memo.TenantID != nil && *memo.TenantID != *tenantID {
        return nil, status.Errorf(codes.PermissionDenied, "memo not found in this tenant")
    }

    // ... existing visibility checks ...
}
```

### 3.2 Helper Function: `getTenantFromContext`

```go
func getTenantFromContext(ctx context.Context) *int32 {
    if tenantID, ok := ctx.Value("tenant_id").(*int32); ok {
        return tenantID
    }
    return nil
}
```

### 3.3 Fix Agent Escalation Code

**File:** `server/router/api/v1/agent/service.go`

**CreateEscalationTicket — set TenantID on memo:**

```go
// Line ~3749: Update memo creation
memo := &store.Memo{
    UID:        memoUID,
    CreatorID:  creatorID,
    Content:    memoContent.String(),
    Visibility: store.Protected,
    TenantID:   &tenantID,  // NEW: tenantID is already available as parameter
}
```

**findExistingEscalationTicket — scope by tenant:**

```go
// Line ~3592: Fix ticket deduplication to scope by tenant
func (s *Service) findExistingEscalationTicket(ctx context.Context, tenantID int32, sessionID string) *store.Ticket {
    ticketType := "agent_escalation"

    // FIX: Filter by tenant instead of scanning all tickets
    tickets, err := s.store.ListTickets(ctx, &store.FindTicket{
        Type:     &ticketType,
        TenantID: &tenantID,  // NEW: scope to tenant
    })

    // ... rest of dedup logic ...
}
```

### 3.4 Update Agent Ticket Creation

**CreateEscalationTicket — set TenantID on ticket:**

```go
// Line ~3760: Update ticket creation
ticket := &store.Ticket{
    Subject:     ticketNumber,
    Description: description,
    Type:        "agent_escalation",
    Priority:    priority,
    Status:      "open",
    CreatorID:   creatorID,
    TenantID:    &tenantID,  // NEW
}
```

### 3.5 Update AI Ticket Reply

**AddTicketReply — set TenantID on AI memo:**

```go
// Line ~1124: Update AI memo creation
aiMemo := &store.Memo{
    UID:        shortuuid.New(),
    CreatorID:  store.SystemBotID,
    Content:    aiReply,
    Visibility: store.Protected,
    TenantID:   tenantID,  // NEW: get from ticket's tenant
}
```

### 3.6 Verification

```bash
go build ./server/...
go vet ./server/...
```

---

## Sprint 4: Frontend & RSS

**Goal:** Disable RSS, update frontend defaults, ensure tenant-scoped UI.

### 4.1 Disable RSS Endpoints

**File:** `server/router/rss/rss.go`

Return 410 Gone from RSS handlers:

```go
func GetRSS(c echo.Context) error {
    return c.JSON(410, map[string]string{
        "error": "RSS feeds are disabled for security reasons",
    })
}
```

### 4.2 Update Frontend Ticket Defaults

**File:** `web/src/pages/Tickets.tsx`

**Line 648:** Change default visibility from PUBLIC to PROTECTED:

```tsx
// BEFORE:
defaultVisibility={Visibility.PUBLIC}

// AFTER:
defaultVisibility={Visibility.PROTECTED}
```

### 4.3 Verification

```bash
cd web && npm run build
```

---

## Sprint 5: Testing

**Goal:** Verify tenant isolation, backward compatibility, and cross-tenant prevention.

### 5.1 Unit Tests for Tenant-Scoped Queries

```go
func TestMemoTenantIsolation(t *testing.T) {
    // Create memos for tenant A and tenant B
    // Verify tenant A cannot see tenant B's memos
    // Verify NULL tenant_id memos are only visible to creator/super
}

func TestTicketTenantIsolation(t *testing.T) {
    // Create tickets for tenant A and tenant B
    // Verify tenant A cannot see tenant B's tickets
    // Verify ticket deduplication is scoped by tenant
}
```

### 5.2 Integration Test Checklist

| Test | Expected | Status |
|------|----------|--------|
| Create memo with tenant_id | Stored with tenant_id | |
| List memos for tenant A | Only tenant A memos returned | |
| List memos for tenant B | Only tenant B memos returned | |
| Get memo from different tenant | PermissionDenied | |
| Create escalation memo | Tenant_id set from agent context | |
| List escalation tickets | Scoped to requesting tenant | |
| Dedup escalation ticket | Only checks same tenant | |
| RSS endpoint | Returns 410 | |
| Ticket default visibility | PROTECTED | |
| Legacy memos (NULL tenant_id) | Visible to creator + super only | |

### 5.3 Verification

```bash
go test ./store/db/sqlite/... -count=1 -v
go test ./store/db/postgres/... -count=1 -v
go test ./server/router/api/v1/... -count=1 -v
go test ./server/router/api/v1/agent/... -count=1 -v
```

---

## Sprint 6: Deployment & Rollback

**Goal:** Safe deployment with rollback capability.

### 6.1 Pre-Deployment Checklist

- [ ] All tests pass
- [ ] Migration tested on staging database
- [ ] `DisallowPublicVisibility=true` set in workspace settings
- [ ] RSS endpoints confirmed disabled
- [ ] Frontend builds without errors

### 6.2 Deployment Steps

```bash
# 1. Backup database
sqlite3 build/data/memos_dev.db ".backup 'backup_$(date +%s).db'"

# 2. Run migration
task build:backend

# 3. Verify migration
sqlite3 build/data/memos_dev.db ".schema memo" | grep tenant_id
sqlite3 build/data/memos_dev.db ".schema tickets" | grep tenant_id

# 4. Start application
task run
```

### 6.3 Environment Variables

```bash
# Ensure these are set:
DISALLOW_PUBLIC_VISIBILITY=true  # Blocks PUBLIC memo creation
```

### 6.4 Rollback Plan

If issues arise:

```bash
# 1. Stop application

# 2. Remove tenant_id columns (SQLite 3.35.0+)
sqlite3 build/data/memos_dev.db "ALTER TABLE memo DROP COLUMN tenant_id;"
sqlite3 build/data/memos_dev.db "ALTER TABLE tickets DROP COLUMN tenant_id;"

# 3. Revert code changes

# 4. Restart application
```

**Note:** SQLite does not support `DROP COLUMN` before version 3.35.0. If running older SQLite, restore from backup.

### 6.5 Post-Deployment Verification

```bash
# Verify RSS disabled
curl -X GET http://localhost:8081/rss.xml
# Expected: 410

# Verify ticket default visibility
curl -X GET http://localhost:8081/api/v1/tickets
```

---

## Effort Estimates

| Sprint | Effort | Risk |
|--------|--------|------|
| 1: Database migration | 30 min | Low — additive columns |
| 2: Store layer | 2-3 hours | Medium — multiple files |
| 3: Service layer | 2-3 hours | Medium — business logic |
| 4: Frontend & RSS | 1 hour | Low — simple changes |
| 5: Testing | 2-3 hours | Medium — comprehensive coverage |
| 6: Deployment | 30 min | Low — additive migration |

**Total estimated effort: 1 day**

---

## Affected Files

| File | Change Type |
|------|-------------|
| `store/migration/sqlite/0.27/00__memo_ticket_tenant_isolation.sql` | **NEW** |
| `store/migration/postgres/0.27/00__memo_ticket_tenant_isolation.sql` | **NEW** |
| `store/migration/sqlite/LATEST.sql` | Modify |
| `store/migration/postgres/LATEST.sql` | Modify |
| `store/memo.go` | Modify |
| `store/ticket.go` | Modify |
| `store/db/sqlite/memo.go` | Modify |
| `store/db/sqlite/memo_filter.go` | Modify |
| `store/db/sqlite/ticket.go` | Modify |
| `store/db/postgres/memo.go` | Modify |
| `store/db/postgres/memo_filter.go` | Modify |
| `store/db/postgres/ticket.go` | Modify |
| `server/router/api/v1/memo_service.go` | Modify |
| `server/router/api/v1/agent/service.go` | Modify |
| `server/router/rss/rss.go` | Disable |
| `server/router/api/v1/v1.go` | Modify |
| `web/src/pages/Tickets.tsx` | Modify |

---

## Risk Mitigation

1. **Additive migration** — Adding nullable columns is safe, no data loss
2. **NULL backward compat** — Existing memos remain accessible to creators
3. **No breaking changes** — API contracts unchanged, new fields are additive
4. **Rollback safe** — Can drop columns or restore from backup
5. **RSS removal** — Non-critical feature, no API consumers expected
