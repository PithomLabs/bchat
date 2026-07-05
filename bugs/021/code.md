# Code Implementation: Memo & Ticket Tenant Isolation

**Date:** 2026-07-05
**Bug:** 021
**Status:** Implemented

---

## Files Modified

### Database Layer

| File | Change |
|------|--------|
| `store/migration/sqlite/0.27/00__memo_ticket_tenant_isolation.sql` | NEW — adds `tenant_id` to `memo` and `tickets` |
| `store/migration/postgres/0.27/00__memo_ticket_tenant_isolation.sql` | NEW — same for Postgres |
| `store/migration/sqlite/LATEST.sql` | Added `tenant_id INTEGER DEFAULT NULL` to `memo` (line 51) and `tickets` (line 165) tables |
| `store/migration/postgres/LATEST.sql` | Added `tenant_id INTEGER DEFAULT NULL` to `memo` (line 49) and `tickets` (line 701) tables |

### Store Types

| File | Change |
|------|--------|
| `store/memo.go` | Added `TenantID *int32` to `Memo` struct (line 52), `FindMemo` struct (line 78), `UpdateMemo` struct (line 108) |
| `store/ticket.go` | Added `TenantID *int32` to `Ticket` struct (line 36), `FindTicket` struct (line 43) |

### SQLite Driver

| File | Change |
|------|--------|
| `store/db/sqlite/memo.go` | `CreateMemo`: added `tenant_id` to INSERT fields and args (lines 17-28). `ListMemos`: added `tenant_id` WHERE clause (lines 86-88), added `tenant_id` to SELECT fields (line 151) and Scan dests (line 191). `UpdateMemo`: added `tenant_id` to SET clause (lines 242-244) |
| `store/db/sqlite/ticket.go` | `CreateTicket`: added `tenant_id` to INSERT fields and args (lines 17-31). `ListTickets`: added `tenant_id` WHERE clause (lines 72-74), added `tenant_id` to SELECT fields (line 86) and Scan dests (line 112). `UpdateTicket`: added `tenant_id` to RETURNING clause (line 184) and Scan dests (line 200) |
| `store/db/sqlite/memo_filter.go` | Added `"tenant_id"` to valid CEL identifiers (line 61). Added `tenant_id` comparison handling (lines 152-162) |

### Postgres Driver

| File | Change |
|------|--------|
| `store/db/postgres/memo.go` | Same changes as SQLite memo.go — added `tenant_id` to INSERT, SELECT, WHERE, and UPDATE queries |
| `store/db/postgres/ticket.go` | Same changes as SQLite ticket.go — added `tenant_id` to INSERT, SELECT, WHERE, and UPDATE queries. Added `encoding/json` import |
| `store/db/postgres/memo_filter.go` | Same changes as SQLite memo_filter.go — added `tenant_id` to valid CEL identifiers and comparison handling |

### Service Layer

| File | Change |
|------|--------|
| `server/router/api/v1/agent/service.go` | `CreateEscalationTicket` (line 3754): Added `TenantID: &tenantID` to memo creation. Ticket creation (line 3781): Added `TenantID: &tenantID`. `findExistingEscalationTicket` (line 3592): Changed from `FindTicket{Type: &ticketType}` to `FindTicket{Type: &ticketType, TenantID: &tenantID}` — scoped by tenant instead of scanning all tickets |
| `server/router/api/v1/memo_service.go` | `handleTicketAIResponse` (line 1124): Added `TenantID: &tenantID` to AI reply memo creation |

### Frontend

| File | Change |
|------|--------|
| `web/src/pages/Tickets.tsx` | Line 648: Changed `defaultVisibility={Visibility.PUBLIC}` to `defaultVisibility={Visibility.PROTECTED}` |

### RSS

| File | Change |
|------|--------|
| `server/router/rss/rss.go` | `GetExploreRSS` and `GetUserRSS` now return `410 Gone` with error message instead of generating RSS feeds |

---

## Key Code Patterns

### 1. TenantID in Memo Creation (Agent)

```go
// server/router/api/v1/agent/service.go:3749-3755
memo := &store.Memo{
    UID:        memoUID,
    CreatorID:  creatorID,
    Content:    memoContent.String(),
    Visibility: store.Protected,
    TenantID:   &tenantID,  // NEW: tenant-scoped
}
```

### 2. Tenant-Scoped Ticket Dedup

```go
// server/router/api/v1/agent/service.go:3591-3592
// BEFORE: Scanned ALL tickets globally
tickets, err := s.store.ListTickets(ctx, &store.FindTicket{Type: &ticketType})

// AFTER: Scoped to tenant
tickets, err := s.store.ListTickets(ctx, &store.FindTicket{
    Type:     &ticketType,
    TenantID: &tenantID,  // NEW: scope to tenant
})
```

### 3. TenantID in SQL Queries

```sql
-- SQLite: ListMemos WHERE clause
WHERE `memo`.`tenant_id` = ?

-- Postgres: ListMemos WHERE clause
WHERE memo.tenant_id = $N
```

### 4. CEL Filter Support

```go
// store/db/sqlite/memo_filter.go
} else if identifier == "tenant_id" {
    if _, err := ctx.Buffer.WriteString(fmt.Sprintf("`memo`.`tenant_id` %s ?", operator)); err != nil {
        return err
    }
    ctx.Args = append(ctx.Args, valueInt)
}
```

### 5. RSS Disabled

```go
// server/router/rss/rss.go:46-50
func (s *RSSService) GetExploreRSS(c echo.Context) error {
    return c.JSON(http.StatusGone, map[string]string{
        "error": "RSS feeds are disabled for security reasons",
    })
}
```

---

## Migration Instructions

### SQLite

```bash
# Backup database first
sqlite3 build/data/memos_dev.db ".backup 'backup_$(date +%s).db'"

# Run migration
sqlite3 build/data/memos_dev.db < store/migration/sqlite/0.27/00__memo_ticket_tenant_isolation.sql

# Verify
sqlite3 build/data/memos_dev.db ".schema memo" | grep tenant_id
sqlite3 build/data/memos_dev.db ".schema tickets" | grep tenant_id
```

### Postgres

```bash
psql $DATABASE_URL -f store/migration/postgres/0.27/00__memo_ticket_tenant_isolation.sql
```

---

## Verification Checklist

- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] Migration creates `tenant_id` column in `memo` table
- [ ] Migration creates `tenant_id` column in `tickets` table
- [ ] Escalation memos have `tenant_id` set
- [ ] Escalation tickets have `tenant_id` set
- [ ] Ticket dedup scoped by tenant
- [ ] RSS returns 410 Gone
- [ ] Frontend ticket default is PROTECTED
