# Code2: Internal Notes + RAG-Based Bug Inference (Post-Review)

**Date:** 2026-07-30
**Status:** Revised — Post-Code-Review
**Bug:** 051
**Revision:** code2.md (incorporates 6 findings from code_review.md)

---

## Changelog: code.md → code2.md

| Finding | Source | Resolution |
|---------|--------|------------|
| HIGH: Import Script Hardcodes `creator_id = 1` | code_review.md #1 | **Fixed** — Query for first available user, fallback to SystemBotID (0) |
| MEDIUM: Context Cancellation in Inference Goroutine | code_review.md #2 | **Fixed** — Use `context.WithoutCancel(ctx)` |
| MEDIUM: code.md Incorrectly Claims Two-Step Create+Update | code_review.md #3 | **Fixed** — Single INSERT with `internal_notes` directly |
| NIT: Postgres ListTickets Dead Code | code_review.md #4 | **Fixed** — Remove redundant lines 58-64 |
| NIT: Inference Content Truncation at 500 Characters | code_review.md #5 | **Fixed** — Increase to 1000 characters |
| NIT: No Integration Test for Goroutine Inference Path | code_review.md #6 | **Documented** — Known limitation, manual verification |

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
| `cmd/import-bugs/main.go` | NEW | ~460 | Bug folder import script (SQLite + Postgres, idempotent) |

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
│  │                  │    │ Detect first available user      │   │
│  │                  │    │ Single INSERT with internal_notes│   │
│  │                  │    │ Skip existing tickets (idempotent)│  │
│  └──────────────────┘    └──────────────────────────────────┘   │
│  ┌──────────────────┐    ┌──────────────────────────────────┐   │
│  │ Phase 2          │───▶│ Future: LLM summary worker pool  │   │
│  │ (not yet)        │    │                                  │   │
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

### User Resolution (FIXED from code.md)

**Problem:** `creator_id = 1` hardcoded — foreign key fails if user ID 1 doesn't exist.

**Solution:** Query for first available user at startup, fallback to `SystemBotID` (0):

```go
func getOrCreateUser(ctx context.Context, db *sql.DB, driver string) (int32, error) {
    // 1. Try to find any existing user
    var userID int32
    query := "SELECT id FROM \"user\" ORDER BY id LIMIT 1"
    err := db.QueryRowContext(ctx, query).Scan(&userID)
    if err == nil {
        return userID, nil
    }
    // 2. Fallback to SystemBotID (0) — may fail FK if user table is empty
    return 0, nil
}
```

### Idempotency

The import script is idempotent — running it twice produces the same result:

```go
// Check if ticket already exists (by title + tenant_id)
func ticketExists(ctx context.Context, db *sql.DB, driver string, title string, tenantID int32) (bool, error) {
    var exists bool
    query := "SELECT EXISTS(SELECT 1 FROM tickets WHERE title = $1 AND tenant_id = $2)"
    err := db.QueryRowContext(ctx, query, title, tenantID).Scan(&exists)
    return exists, err
}
```

**Behavior:**
- First run: Creates ~130 tickets, outputs "Created: 130, Skipped: 0"
- Second run: Skips all existing, outputs "Created: 0, Skipped: 130"
- Interrupted run: Tickets with `internal_notes = "Pending summary..."` are detectable

### SQLite Support

```bash
# Default: uses build/data/memos_dev.db
go run ./cmd/import-bugs/

# Custom path
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

Uses `$N` placeholders automatically.

### Bug Folder Parsing

```
bugs/001/
  ├── plan.md          → type: "plan"
  ├── code.md          → type: "code"
  ├── testing.md       → type: "testing"
  ├── code_review.md   → type: "review"
  ├── summary.md       → type: "summary"
  └── code_signoff.md  → type: "signoff"
```

**Classification rules:**
- Filename contains "plan" (not "review") → `plan`
- Filename contains "code" (not "review") → `code`
- Filename contains "testing" (not "review") → `testing`
- Filename contains "review" → `review`
- Filename contains "summary" → `summary`
- Filename contains "signoff" → `signoff`

### Status Determination

- Has `signoff` phase → `CLOSED`
- Otherwise → `IN_PROGRESS`

### Priority Determination

- Content contains "critical"/"urgent"/"p0" → `HIGH`
- More than 15 files → `HIGH`
- More than 5 files → `MEDIUM`
- Otherwise → `LOW`

### Internal Notes Format

```markdown
Bug #001 - 8 files across 3 phases

### plan.md (plan)
### code.md (code)
### testing.md (testing)
```

Each phase section contains extracted key points (headers, bullets, root cause/fix/solution mentions) truncated to 500 characters.

### Data Volume

| Metric | Value |
|--------|-------|
| Bug folders | 50 |
| Empty folders skipped | 1 (bug 007) |
| Avg files per bug | 12.4 |
| **Total tickets created** | **~130** |

---

## 4. Database Schema

### Migration

**SQLite** (`store/migration/sqlite/0.35/00__tickets_add_internal_notes.sql`):
```sql
ALTER TABLE tickets ADD COLUMN internal_notes TEXT DEFAULT '';
```

**Postgres** (`store/migration/postgres/0.35/00__tickets_add_internal_notes.sql`):
```sql
ALTER TABLE tickets ADD COLUMN internal_notes TEXT DEFAULT '';
```

**CockroachDB Compatibility:** Standard SQL, works on all three.

### Store Types

```go
type Ticket struct {
    // ... existing fields ...
    InternalNotes string
}

type UpdateTicket struct {
    // ... existing fields ...
    InternalNotes *string
}
```

### Driver Changes

| Operation | SQLite | Postgres |
|-----------|--------|----------|
| CreateTicket | `?` × 12, bind `InternalNotes` | `$12` for `internal_notes` |
| ListTickets | SELECT + Scan includes `internal_notes` | SELECT + Scan includes `internal_notes` |
| UpdateTicket | Dynamic `SET internal_notes = ?`, RETURNING includes it | Dynamic `SET internal_notes = $N`, RETURNING includes it |

---

## 5. Environment Variables

No new environment variables required. Uses existing:
- `COCKROACH_DSN` / `DATABASE_URL` / `MEMOS_DSN` for Postgres import
- `SQLITE_PATH` for local SQLite import (defaults to `build/data/memos_dev.db`)
- `BUGS_DIR` for import script (defaults to `bugs`)

---

## 6. Build & Run Commands

```bash
# Build
go build ./bin/memos/main.go

# Build import script
go build ./cmd/import-bugs/

# Run tests
go test ./store/... -v
go test ./server/router/api/v1/agent/... -v

# Import bugs (SQLite - local development)
go run ./cmd/import-bugs/

# Import bugs (CockroachDB - production)
export COCKROACH_DSN="postgresql://..."
go run ./cmd/import-bugs/

# Import bugs (custom bugs directory)
BUGS_DIR=/path/to/bugs go run ./cmd/import-bugs/

# Validate schema
task validate:schema
task validate:parity

# Cleanup imported tickets (if needed)
sqlite3 build/data/memos_dev.db "DELETE FROM tickets WHERE type = 'BUG' AND tags LIKE '%imported%';"
```

---

## 7. Testing Guide

### Build Verification

```bash
go build ./bin/memos/main.go                    # must compile
go build ./cmd/import-bugs/                     # must compile
go test ./store/... -count=1                    # must pass
go test ./server/router/api/v1/agent/... -count=1  # must pass
task validate:schema                            # must pass
task validate:parity                            # must pass
```

### Manual Verification

```bash
# 1. Start server (migration runs automatically)
task run

# 2. Import bugs (SQLite)
go run ./cmd/import-bugs/

# 3. Verify tickets created
sqlite3 build/data/memos_dev.db \
  "SELECT id, title, substr(internal_notes, 1, 50) FROM tickets WHERE type='BUG' LIMIT 10;"

# 4. Verify idempotency (second run skips all)
go run ./cmd/import-bugs/
# Output: Created: 0, Skipped: ~130

# 5. Verify RBAC filtering
# Create ticket as customer → internal_notes empty
# View as admin → internal_notes populated

# 6. Verify resolution inference
curl -X POST http://localhost:5230/api/v1/tickets \
  -H "Content-Type: application/json" \
  -d '{"title":"Test ticket","description":"/m/test","status":"OPEN","priority":"MEDIUM","type":"TASK"}'
# Check if internal_notes auto-populated
```

---

## 8. Known Limitations

| Issue | Severity | Mitigation |
|-------|----------|------------|
| Phase 2 (LLM summary generation) not implemented | MEDIUM | Tickets have "Pending summary..." placeholder |
| No integration test for goroutine inference path | LOW | Manual verification covers this |
| `creator_id` depends on at least one user existing | LOW | SystemBotID (0) fallback; fails if user table empty |
| Internal notes included in vector embeddings | MEDIUM | Trade-off: richer search vs RBAC gap at vector level |
| Import script uses database/sql directly | LOW | Simpler for one-shot import; store layer not needed |

---

## 9. Adversarial Code Review Prompt

Copy and paste this prompt into Claude/Gemini for a thorough code review:

---

**PROMPT:**

```
You are performing an adversarial code review of an Internal Notes + RAG-Based Bug Inference implementation for a multi-tenant AI chat agent platform (bchat). This is a hackathon submission for CockroachDB × AWS.

Review the following files against the codebase conventions, RBAC requirements, and CockroachDB best practices:

FILES TO REVIEW:
1. store/migration/sqlite/0.35/00__tickets_add_internal_notes.sql (SQLite migration)
2. store/migration/postgres/0.35/00__tickets_add_internal_notes.sql (Postgres migration)
3. store/migration/sqlite/LATEST.sql (line 171: internal_notes column)
4. store/migration/postgres/LATEST.sql (line 660: internal_notes column)
5. store/ticket.go (Ticket and UpdateTicket structs)
6. store/db/sqlite/ticket.go (CreateTicket, ListTickets, UpdateTicket with internal_notes)
7. store/db/postgres/ticket.go (CreateTicket, ListTickets, UpdateTicket with internal_notes)
8. server/router/api/v1/agent/permissions.go (PermTicketInternalNotes, HasPermission, presets)
9. server/router/api/v1/ticket_service.go (RBAC filtering, filterInternalNotes, inference trigger)
10. web/src/pages/TicketDetail.tsx (internal notes display)
11. server/router/api/v1/agent/ticket_embedder.go (include internal_notes in embedding content)
12. server/router/api/v1/agent/service.go (InferResolutionForNewTicket with vectorDBMu lock)
13. cmd/import-bugs/main.go (bug folder import script)

CONSTRAINTS TO VERIFY:
- SQLite uses ? placeholders, Postgres uses $N placeholders
- convertTicketFromStore keeps single-arg signature (no API break)
- filterInternalNotes is called in each handler post-conversion (not in converter)
- ResolveEffectivePermissions + HasPermission used (NOT CheckUserPermission which doesn't exist)
- vectorDBMu.RLock/RUnlock wraps vectorDB access in InferResolutionForNewTicket
- vectorDB.Search() API used (NOT raw SQL against agent_vectors)
- Import script queries for first available user (NOT hardcoded creator_id)
- Import script uses context.WithoutCancel(ctx) for goroutine
- Import script uses modernc.org/sqlite driver (not mattn/go-sqlite3)
- Tenant ID never exposed in error messages
- internal_notes defaults to "" in CreateTicket handler (not settable via API)

REVIEW CHECKLIST:

RBAC:
[C-1] CRITICAL: Does filterInternalNotes correctly check superuser, creator, assignee, AND permission?
[C-2] CRITICAL: Is ResolveEffectivePermissions called with correct tenantID and userID?
[C-3] CRITICAL: Does UpdateTicket handler only allow internal_notes update for superuser or ticket:internal_notes permission?
[C-4] CRITICAL: Is tenant_id enforced in vector search (InferResolutionForNewTicket)?

DATA INTEGRITY:
[H-1] HIGH: Does SQLite CreateTicket bind 12 parameters correctly?
[H-2] HIGH: Does Postgres CreateTicket use $11 for tenant_id and $12 for internal_notes?
[H-3] HIGH: Does ListTickets Scan include internal_notes in both SQLite and Postgres?
[H-4] HIGH: Does UpdateTicket RETURNING clause include internal_notes?
[H-5] HIGH: Does import script query for first available user (not hardcoded ID)?

CONCURRENCY:
[H-6] HIGH: Is vectorDBMu.RLock used before accessing s.vectorDB in InferResolutionForNewTicket?
[H-7] HIGH: Does InferResolutionForNewTicket handle nil TenantID gracefully?
[H-8] HIGH: Does the goroutine use context.WithoutCancel(ctx)?

IMPORT SCRIPT:
[M-1] MEDIUM: Does import script use correct SQLite DSN with pragmas?
[M-2] MEDIUM: Does import script skip empty folders (bug 007)?
[M-3] MEDIUM: Does import script detect existing tickets and skip?
[M-4] MEDIUM: Does import script use single INSERT with internal_notes (not two-step)?
[M-5] MEDIUM: Does import script handle both SQLite and Postgres parameter syntax?

FRONTEND:
[N-1] NIT: Is internalNotes optional in the Ticket TypeScript interface?
[N-2] NIT: Is internal notes section only rendered when non-empty?
[N-3] NIT: Is the yellow background styling consistent with codebase?

GENERAL:
[N-4] NIT: Are error messages wrapped with context?
[N-5] NIT: Is slog usage consistent with codebase conventions?
[N-6] NIT: Is import ordering consistent?

INVARIANTS TO VERIFY:

1. INV_TICKET_INTERNAL_NOTES_RBAC
   Internal notes must be hidden from users who are not superusers, not the creator,
   not assigned, and don't have ticket:internal_notes permission.

2. INV_TICKET_INTERNAL_NOTES_PERSISTENCE
   Internal notes must survive CREATE → READ → UPDATE → READ cycles without data loss.

3. INV_VECTOR_SEARCH_TENANT_ISOLATION
   InferResolutionForNewTicket must only return results from the same tenant.
   Cross-tenant data must never leak through vector search.

4. INV_IMPORT_IDEMPOTENCY
   Running the import script twice must not create duplicate tickets.
   Existing tickets must be detected and skipped.

5. INV_RESOLUTION_INference_GRACEFUL_DEGRADATION
   If CockroachDB is unavailable or no similar tickets found, the new ticket
   must still be created successfully with empty or default internal_notes.

6. INV_IMPORT_USER_RESOLUTION
   Import script must find an existing user or fall back gracefully.
   Hardcoded creator_id must not cause FK violations.

OUTPUT FORMAT:
For each finding, provide:
- File:line_number
- Severity: CRITICAL/HIGH/MEDIUM/NIT
- Description: What's wrong
- Fix: Exact code change

Also verify:
- All 13 files compile without errors
- Both store tests and agent tests pass
- task validate:schema passes
- task validate:parity passes
```

---

## 10. Rollback Plan

If internal notes feature causes issues:

1. **Database:** Column defaults to `""`, no rollback needed.
2. **RBAC:** Remove `PermTicketInternalNotes` from `AllPermissions` and `PermissionPresets`.
3. **Inference:** Remove the `go` goroutine trigger in `CreateTicket` handler.
4. **Import:** Delete imported tickets:
   ```sql
   DELETE FROM tickets WHERE type = 'BUG' AND tags LIKE '%imported%';
   ```
5. **Frontend:** Remove `{ticket.internalNotes && ...}` block from `TicketDetail.tsx`.
