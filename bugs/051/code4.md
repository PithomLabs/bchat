# Code4: Internal Notes + RAG-Based Bug Inference (Verified)

**Date:** 2026-07-30
**Status:** Verified — All 4 source fixes applied and tested
**Bug:** 051
**Revision:** code4.md (verified against actual source code)

---

## Changelog: code3.md → code4.md

| Finding | Source | Status | Evidence |
|---------|--------|--------|----------|
| HIGH: Import script hardcodes `creator_id = 1` | code_review.md #1 | **Applied** ✅ | `cmd/import-bugs/main.go:168-185` — `getOrCreateUser()` queries first available user |
| MEDIUM: Context cancellation in inference goroutine | code_review.md #2 | **Applied** ✅ | `server/router/api/v1/ticket_service.go:166` — `context.WithoutCancel(ctx)` |
| MEDIUM: Postgres ListTickets dead code | code_review.md #3 | **Applied** ✅ | `store/db/postgres/ticket.go:57-59` — clean initialization |
| NIT: Inference truncation 500→1000 chars | code_review.md #4 | **Applied** ✅ | `server/router/api/v1/agent/service.go:5632` — `1000` |

---

## 1. Implementation Summary

### What Was Built

Added an `internal_notes` field to tickets with RBAC-controlled visibility, an import pipeline that reads 50 bug folders (001-050) into ~130 tickets (SQLite and Postgres), and a synchronous resolution inference system that searches CockroachDB vector index for similar past tickets when a new ticket is created.

### File Inventory

| File | Type | Lines | Purpose |
|------|------|-------|---------|
| `store/migration/sqlite/0.35/00__tickets_add_internal_notes.sql` | NEW | 1 | SQLite migration: add `internal_notes` column |
| `store/migration/postgres/0.35/00__tickets_add_internal_notes.sql` | NEW | 1 | Postgres migration: add `internal_notes` column |
| `store/migration/sqlite/LATEST.sql` | MODIFIED | +1 | Add `internal_notes` to tickets schema |
| `store/migration/postgres/LATEST.sql` | MODIFIED | +1 | Add `internal_notes` to tickets schema |
| `store/ticket.go` | MODIFIED | +2 | `InternalNotes` on `Ticket` and `UpdateTicket` structs |
| `store/db/sqlite/ticket.go` | MODIFIED | +12 | CRUD with `internal_notes` in INSERT, SELECT, UPDATE, RETURNING |
| `store/db/postgres/ticket.go` | MODIFIED | +8 | CRUD with `internal_notes` using `$N` placeholders; dead code removed |
| `server/router/api/v1/agent/permissions.go` | MODIFIED | +10 | `PermTicketInternalNotes` constant, `HasPermission()` helper, preset updates |
| `server/router/api/v1/ticket_service.go` | MODIFIED | +45 | RBAC filtering, `filterInternalNotes()`, inference trigger with `context.WithoutCancel` |
| `web/src/pages/TicketDetail.tsx` | MODIFIED | +10 | Internal notes display section |
| `server/router/api/v1/agent/ticket_embedder.go` | MODIFIED | +1 | Include `internal_notes` in embedding content |
| `server/router/api/v1/agent/service.go` | MODIFIED | +64 | `InferResolutionForNewTicket()` with `vectorDBMu` lock, 1000-char truncation |
| `cmd/import-bugs/main.go` | NEW | ~470 | Bug folder import script (SQLite + Postgres, idempotent, user-aware) |

---

## 2. Architecture

### Component Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    CreateTicket Handler                          │
│  ┌──────────────────┐    ┌──────────────────────────────────┐   │
│  │ Bind request     │───▶│ store.CreateTicket()             │   │
│  └──────────────────┘    └──────────┬───────────────────────┘   │
│                                     │                            │
│  ┌──────────────────┐    ┌──────────▼───────────────────────┐   │
│  │ go goroutine     │───▶│ InferResolutionForNewTicket()    │   │
│  │ context.Without  │    │   ├─ vectorDB.Search()           │   │
│  │ Cancel(ctx)      │    │   └─ store.UpdateTicket()        │   │
│  └──────────────────┘    └──────────────────────────────────┘   │
│  ┌──────────────────┐    ┌──────────────────────────────────┐   │
│  │ RBAC filter      │───▶│ filterInternalNotes()            │   │
│  │ (per-handler)    │    │   superuser/creator/assignee/perm│   │
│  └──────────────────┘    └──────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    Import Pipeline                               │
│  ┌──────────────────┐    ┌──────────────────────────────────┐   │
│  │ Phase 1          │───▶│ Read bugs/001-050 folders        │   │
│  │ (synchronous)    │    │ Parse .md files by phase type    │   │
│  │                  │    │ getOrCreateUser() → valid FK     │   │
│  │                  │    │ Single INSERT with internal_notes│   │
│  │                  │    │ Skip existing tickets (idempotent)│  │
│  └──────────────────┘    └──────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### RBAC Visibility Rules

Internal notes are visible to:
1. **HOST/ADMIN** (superusers) — all tickets
2. **Ticket creator** — their own tickets
3. **Assigned users** (`assignee_id`) — tickets assigned to them
4. **Users with `ticket:internal_notes` permission** — all tickets in their tenant

All other users see `internalNotes: ""`.

### Permission Check Mechanism

```go
resolvedPerms, err := agent.ResolveEffectivePermissions(ctx, s.Store, tenantID, userID)
hasPerm := agent.HasPermission(resolvedPerms, agent.PermTicketInternalNotes)
```

### Resolution Inference Flow

```
New ticket created
  → go InferResolutionForNewTicket(context.WithoutCancel(ctx), ticket)
    → vectorDBMu.RLock()  // protects against data race during reindex
    → vectorDB.Search(ctx, SearchQuery{
        TenantID:     *ticket.TenantID,
        QueryText:    ticket.Title + "\n" + ticket.Description,
        ContentTypes: []string{"ticket"},
        TopK:         5,
        MinScore:     0.7,
      })
    → Format suggested resolution (1000 char truncation per match)
    → store.UpdateTicket() to set internal_notes
```

---

## 3. Import Pipeline

### Overview

The import script reads 50 bug folders (`bugs/001-050`) and creates tickets in the database. It supports both SQLite and Postgres/CockroachDB with automatic driver detection.

### Driver Detection

```
DATABASE_URL set?  → Use Postgres/CockroachDB (pgx driver, $N placeholders)
COCKROACH_DSN set? → Use Postgres/CockroachDB
MEMOS_DSN set?     → Use Postgres/CockroachDB
None set?          → Fall back to SQLite (modernc.org/sqlite driver, ? placeholders)
```

### User Resolution (Applied)

```go
func getOrCreateUser(ctx context.Context, db *sql.DB, driver string) (int32, error) {
    var userID int32
    var query string
    if driver == "postgres" {
        query = `SELECT id FROM "user" ORDER BY id LIMIT 1`
    } else {
        query = `SELECT id FROM user ORDER BY id LIMIT 1`
    }
    err := db.QueryRowContext(ctx, query).Scan(&userID)
    if err == sql.ErrNoRows {
        return 0, nil  // Fallback to SystemBotID (0)
    } else if err != nil {
        return 0, fmt.Errorf("failed to query user: %w", err)
    }
    return userID, nil
}
```

### Idempotency

```go
func ticketExists(ctx context.Context, db *sql.DB, driver string, title string, tenantID int32) (bool, error) {
    var exists bool
    var query string
    if driver == "postgres" {
        query = `SELECT EXISTS(SELECT 1 FROM tickets WHERE title = $1 AND tenant_id = $2)`
    } else {
        query = `SELECT EXISTS(SELECT 1 FROM tickets WHERE title = ? AND tenant_id = ?)`
    }
    err := db.QueryRowContext(ctx, query, title, tenantID).Scan(&exists)
    return exists, err
}
```

**Behavior:**
- First run: Creates ~130 tickets
- Second run: Skips all existing

### SQLite Support

```bash
go run ./cmd/import-bugs/
SQLITE_PATH=/path/to/memos.db go run ./cmd/import-bugs/
```

SQLite DSN includes pragmas:
```
?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)
```

### Postgres/CockroachDB Support

```bash
export COCKROACH_DSN="postgresql://user:pass@host/db?sslmode=require"
go run ./cmd/import-bugs/
```

### Data Volume

| Metric | Value |
|--------|-------|
| Bug folders | 50 |
| Empty folders skipped | 1 (bug 007) |
| **Total tickets created** | **~130** |

---

## 4. Database Schema

### Migration

```sql
ALTER TABLE tickets ADD COLUMN internal_notes TEXT DEFAULT '';
```

### Store Types

```go
type Ticket struct {
    InternalNotes string
}

type UpdateTicket struct {
    InternalNotes *string
}
```

### Driver Changes

| Operation | SQLite | Postgres |
|-----------|--------|----------|
| CreateTicket | `?` × 12, bind `creatorID` | `$12` for `internal_notes` |
| ListTickets | SELECT + Scan includes `internal_notes` | SELECT + Scan includes `internal_notes` |
| UpdateTicket | Dynamic SET, RETURNING includes it | Dynamic SET, RETURNING includes it |

---

## 5. Build & Run Commands

```bash
go build ./bin/memos/main.go
go build ./cmd/import-bugs/
go test ./store/... -count=1
go test ./server/router/api/v1/agent/... -count=1
go run ./cmd/import-bugs/
task validate:schema
task validate:parity
```

---

## 6. Known Limitations

| Issue | Severity | Mitigation |
|-------|----------|------------|
| Phase 2 (LLM summary generation) not implemented | MEDIUM | Tickets have placeholder internal_notes |
| No integration test for goroutine inference path | LOW | Manual verification |
| Internal notes in vector embeddings | MEDIUM | Trade-off for richer search |
| `getOrCreateUser` may fail if user table empty | LOW | Fallback to SystemBotID (0) |

---

## 7. Adversarial Code Review Prompt

```
You are performing an adversarial code review of an Internal Notes + RAG-Based Bug Inference implementation for a multi-tenant AI chat agent platform (bchat).

FILES TO REVIEW:
1. store/migration/sqlite/0.35/00__tickets_add_internal_notes.sql
2. store/migration/postgres/0.35/00__tickets_add_internal_notes.sql
3. store/migration/sqlite/LATEST.sql
4. store/migration/postgres/LATEST.sql
5. store/ticket.go
6. store/db/sqlite/ticket.go
7. store/db/postgres/ticket.go
8. server/router/api/v1/agent/permissions.go
9. server/router/api/v1/ticket_service.go
10. web/src/pages/TicketDetail.tsx
11. server/router/api/v1/agent/ticket_embedder.go
12. server/router/api/v1/agent/service.go
13. cmd/import-bugs/main.go

CONSTRAINTS:
- getOrCreateUser queries first available user (not hardcoded ID)
- context.WithoutCancel(ctx) used for goroutine
- vectorDBMu.RLock/RUnlock wraps vectorDB access
- Dead code removed from postgres ListTickets
- Truncation at 1000 chars

CHECKLIST:
[C-1] CRITICAL: Does filterInternalNotes check superuser, creator, assignee, AND permission?
[C-2] CRITICAL: Is tenant_id enforced in vector search?
[H-1] HIGH: Does import script use getOrCreateUser (not hardcoded creator_id)?
[H-2] HIGH: Does goroutine use context.WithoutCancel(ctx)?
[H-3] HIGH: Is vectorDBMu.RLock used in InferResolutionForNewTicket?
[M-1] MEDIUM: Is dead code removed from postgres ListTickets?
[M-2] MEDIUM: Is truncation at 1000 chars?
[M-3] MEDIUM: Is import idempotent?
[N-1] NIT: Is frontend internalNotes optional and conditionally rendered?

INVARIANTS:
1. INV_TICKET_INTERNAL_NOTES_RBAC
2. INV_TICKET_INTERNAL_NOTES_PERSISTENCE
3. INV_VECTOR_SEARCH_TENANT_ISOLATION
4. INV_IMPORT_IDEMPOTENCY
5. INV_RESOLUTION_INference_GRACEFUL_DEGRADATION
6. INV_IMPORT_USER_RESOLUTION

OUTPUT: File:line_number, Severity, Description, Fix
```
