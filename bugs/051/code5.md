# Code5: Internal Notes + RAG-Based Bug Inference (Final)

**Date:** 2026-07-30
**Status:** Final — All nits addressed
**Bug:** 051
**Revision:** code5.md (post code4_review.md)

---

## Changelog: code4.md → code5.md

| Finding | Source | Status | Evidence |
|---------|--------|--------|----------|
| NIT: Unquoted `user` identifier | code4_review.md #1 | **Applied** ✅ | `cmd/import-bugs/main.go:174` — both SQLite and Postgres use `FROM "user"` |
| NIT: File line count mismatch | code4_review.md #2 | **Fixed** ✅ | Updated to `~490` lines |
| NIT: SystemBotID(0) fallback FK risk | code4_review.md #3 | **Applied** ✅ | `cmd/import-bugs/main.go:177-188` — creates system bot user if none exist |

---

## 1. Implementation Summary

Added an `internal_notes` field to tickets with RBAC-controlled visibility, an import pipeline that reads 50 bug folders (001-050) into ~130 tickets (SQLite and Postgres), and a synchronous resolution inference system that searches CockroachDB vector index for similar past tickets when a new ticket is created.

### File Inventory

| File | Type | Lines | Purpose |
|------|------|-------|---------|
| `store/migration/sqlite/0.35/00__tickets_add_internal_notes.sql` | NEW | 1 | SQLite migration |
| `store/migration/postgres/0.35/00__tickets_add_internal_notes.sql` | NEW | 1 | Postgres migration |
| `store/migration/sqlite/LATEST.sql` | MODIFIED | +1 | `internal_notes` column |
| `store/migration/postgres/LATEST.sql` | MODIFIED | +1 | `internal_notes` column |
| `store/ticket.go` | MODIFIED | +2 | `InternalNotes` fields |
| `store/db/sqlite/ticket.go` | MODIFIED | +12 | CRUD with `internal_notes` |
| `store/db/postgres/ticket.go` | MODIFIED | +8 | CRUD with `internal_notes`, dead code removed |
| `server/router/api/v1/agent/permissions.go` | MODIFIED | +10 | Permission constant + helper + presets |
| `server/router/api/v1/ticket_service.go` | MODIFIED | +45 | RBAC filtering + `context.WithoutCancel` |
| `web/src/pages/TicketDetail.tsx` | MODIFIED | +10 | Internal notes display |
| `server/router/api/v1/agent/ticket_embedder.go` | MODIFIED | +1 | Embedding content includes notes |
| `server/router/api/v1/agent/service.go` | MODIFIED | +64 | Inference with 1000-char truncation |
| `cmd/import-bugs/main.go` | NEW | ~490 | Import script (user-aware, idempotent) |

---

## 2. Architecture

### Component Flow

```
CreateTicket Handler
  → store.CreateTicket()
  → go InferResolutionForNewTicket(context.WithoutCancel(ctx), ticket)
      → vectorDBMu.RLock()
      → vectorDB.Search(SearchQuery{TenantID, TopK:5, MinScore:0.7})
      → Format suggested resolution (1000 chars per match)
      → store.UpdateTicket()
  → filterInternalNotes() per RBAC rules

Import Pipeline
  → getOrCreateUser() → queries first user, creates system bot if empty
  → getOrCreateTenant() → queries or creates hackathon-demo tenant
  → For each bug folder (001-050):
      → Parse .md files by phase type
      → Check ticketExists() for idempotency
      → Single INSERT with internal_notes
```

### RBAC Visibility

Internal notes visible to: superuser, creator, assignee, or `ticket:internal_notes` permission holder. All others see `""`.

---

## 3. Import Pipeline

### User Resolution (Final)

```go
func getOrCreateUser(ctx context.Context, db *sql.DB, driver string) (int32, error) {
    var userID int32
    // Both drivers quote "user" (reserved word)
    query := `SELECT id FROM "user" ORDER BY id LIMIT 1`
    err := db.QueryRowContext(ctx, query).Scan(&userID)
    if err == sql.ErrNoRows {
        // Create system bot user
        createQuery := `INSERT INTO "user" (username, role, nickname, password_hash) VALUES ($1, $2, $3, $4) RETURNING id`
        err = db.QueryRowContext(ctx, createQuery, "system_bot", "ADMIN", "Bot", "").Scan(&userID)
        if err != nil {
            return 0, fmt.Errorf("failed to create system bot user: %w", err)
        }
        return userID, nil
    }
    return userID, nil
}
```

### Idempotency

```go
ticketExists(ctx, db, driver, title, tenantID) // checks title + tenant_id
```

- First run: Creates ~130 tickets
- Second run: Skips all existing
- Both SQLite and Postgres supported with correct placeholder syntax

---

## 4. Build & Run

```bash
go build ./bin/memos/main.go
go build ./cmd/import-bugs/
go test ./store/... -count=1
go test ./server/router/api/v1/agent/... -count=1
go run ./cmd/import-bugs/
```

---

## 5. Known Limitations

| Issue | Severity | Mitigation |
|-------|----------|------------|
| Phase 2 (LLM summaries) not implemented | MEDIUM | Placeholder internal_notes |
| No integration test for inference goroutine | LOW | Manual verification |
| Internal notes in vector embeddings | MEDIUM | Trade-off for search quality |

---

## 6. Adversarial Code Review Prompt

```
Review the Internal Notes + RAG-Based Bug Inference implementation for bchat.

FILES: All 13 files listed in inventory.

CHECKLIST:
[H-1] Does import script use getOrCreateUser (not hardcoded ID)?
[H-2] Does getOrCreateUser create system bot user if empty?
[H-3] Does goroutine use context.WithoutCancel(ctx)?
[H-4] Is vectorDBMu.RLock used in InferResolutionForNewTicket?
[M-1] Is dead code removed from postgres ListTickets?
[M-2] Is truncation at 1000 chars?
[M-3] Are both SQLite and Postgres queries using quoted "user"?
[M-4] Is import idempotent?

INVARIANTS:
1. INV_TICKET_INTERNAL_NOTES_RBAC
2. INV_TICKET_INTERNAL_NOTES_PERSISTENCE
3. INV_VECTOR_SEARCH_TENANT_ISOLATION
4. INV_IMPORT_IDEMPOTENCY
5. INV_RESOLUTION_INference_GRACEFUL_DEGRADATION
6. INV_IMPORT_USER_RESOLUTION
```
