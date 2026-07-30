# Code Review: Internal Notes + RAG-Based Bug Inference Implementation

**Reviewer:** Senior Go Architect
**Date:** 2026-07-30
**Code:** `code.md` (Bug 051 — Implementation)
**Verdict:** **Approved with Nits** — No critical issues. Implementation matches plan3.md. A few medium/high items and documentation corrections.

---

## Files Reviewed

All 13 files verified against actual implementation at the listed paths.

| File | Status | Lines |
|------|--------|-------|
| `store/migration/sqlite/0.35/00__tickets_add_internal_notes.sql` | ✅ | 1 |
| `store/migration/postgres/0.35/00__tickets_add_internal_notes.sql` | ✅ | 1 |
| `store/migration/sqlite/LATEST.sql` | ✅ | +1 (line 172) |
| `store/migration/postgres/LATEST.sql` | ✅ | +1 (line 661) |
| `store/ticket.go` | ✅ | +2 |
| `store/db/sqlite/ticket.go` | ✅ | +12 |
| `store/db/postgres/ticket.go` | ✅ | +12 (pre-existing dead code noted) |
| `server/router/api/v1/agent/permissions.go` | ✅ | +10 |
| `server/router/api/v1/ticket_service.go` | ✅ | ~+45 |
| `web/src/pages/TicketDetail.tsx` | ✅ | +10 |
| `server/router/api/v1/agent/ticket_embedder.go` | ✅ | +1 |
| `server/router/api/v1/agent/service.go` | ✅ | ~+64 |
| `cmd/import-bugs/main.go` | ✅ | ~300 |

---

## Issues Found

### HIGH: Import Script Hardcodes `creator_id = 1`

**File:** `cmd/import-bugs/main.go:447`
**Severity:** HIGH
**Description:** The `createTicket` function binds hardcoded `1` for `creator_id`:
```go
_, err := db.ExecContext(ctx, query,
    title, description, status, priority,
    1, // system user
    ...
)
```
If no user with ID 1 exists in the database, the foreign key constraint `creator_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE` will cause the INSERT to fail.
**Fix:** Query for an existing user first, or accept a `--creator-id` CLI flag, or use the store layer's CreateTicket which handles this.

---

### MEDIUM: Context Cancellation in Inference Goroutine

**File:** `server/router/api/v1/agent/service.go:5589`
**Severity:** MEDIUM
**Description:** `InferResolutionForNewTicket` receives the HTTP request context:
```go
go s.agentHandler.GetService().InferResolutionForNewTicket(ctx, ticket)
```
The request context is cancelled when the HTTP response is sent. The inference function then calls `vectorDB.Search(ctx, ...)` and `s.store.UpdateTicket(ctx, ...)` with a potentially cancelled context, causing silent failures under load.
**Fix:** Use `context.WithoutCancel(ctx)` (Go 1.21+):
```go
go s.agentHandler.GetService().InferResolutionForNewTicket(context.WithoutCancel(ctx), ticket)
```

---

### MEDIUM: code.md Incorrectly Claims Two-Step Create+Update

**File:** `code.md` (Section 4, line 363)
**Severity:** MEDIUM (documentation)
**Description:** The code.md states the import script "creates ticket then immediately UpdateTicket for internal_notes (two-step)". The actual implementation uses a single `INSERT` with `internal_notes` directly (`main.go:438-443`):
```go
INSERT INTO tickets (..., tenant_id, internal_notes) VALUES (..., ?, ?)
```
The single-step approach is correct and more efficient. The documentation should reflect the actual implementation.

---

### NIT: Postgres ListTickets Dead Code

**File:** `store/db/postgres/ticket.go:55-61`
**Severity:** NIT
**Description:** Pre-existing dead code in the Postgres `ListTickets` function:
```go
where, args := []string{"1=1"}, []interface{}{}
if find.ID != nil {
    where = append(where, "id = $1") // immediately overwritten below
}
where = []string{"1=1"}
args = []interface{}{}
```
The initial append is overwritten three lines later. Not introduced by this feature, but worth cleaning up while modifying this function.

---

### NIT: Inference Content Truncation at 500 Characters

**File:** `server/router/api/v1/agent/service.go:5622`
**Severity:** NIT
**Description:** The inference function truncates matched content at 500 bytes:
```go
if len(content) > 500 {
    content = content[:500] + "..."
}
```
500 characters may cut meaningful resolution details from longer internal notes. Consider 1000 or making this configurable.

---

### NIT: No Integration Test for Goroutine Inference Path

**File:** `code.md` (Test Plan, Section 6)
**Severity:** NIT
**Description:** The test plan covers `filterInternalNotes` and migration but lacks an end-to-end test for the `CreateTicket → goroutine → InferResolutionForNewTicket → Search → UpdateTicket` flow. The goroutine path is exercised only via manual testing.

---

## Verified Correct

| Invariant | Status |
|-----------|--------|
| SQLite CreateTicket: 12 `?` placeholders | ✅ |
| Postgres CreateTicket: `$11` for tenant_id, `$12` for internal_notes | ✅ |
| SQLite ListTickets: SELECT + Scan includes internal_notes | ✅ |
| Postgres ListTickets: SELECT + Scan includes internal_notes | ✅ |
| SQLite UpdateTicket: Dynamic SET includes `internal_notes = ?`, RETURNING includes it | ✅ |
| Postgres UpdateTicket: Dynamic SET includes `internal_notes = $N`, RETURNING includes it | ✅ |
| `convertTicketFromStore` keeps single-arg signature | ✅ |
| `filterInternalNotes` called in each handler post-conversion | ✅ |
| `ResolveEffectivePermissions` + `HasPermission` used (not `CheckUserPermission`) | ✅ |
| `HasPermission` public wrapper delegates to `containsResolvedPermission` | ✅ |
| `AllPermissions` includes `PermTicketInternalNotes` | ✅ |
| `PermissionPresets` updated for analyst/editor/tenant_admin | ✅ |
| `vectorDBMu.RLock/RUnlock` wraps vectorDB access | ✅ |
| `vectorDB.Search()` API used (not raw SQL) | ✅ |
| Import script uses `modernc.org/sqlite` (not `mattn/go-sqlite3`) | ✅ |
| Import script skips empty folders (bug 007) | ✅ |
| Import script detects existing tickets and skips | ✅ |
| SQLite DSN includes foreign_keys + busy_timeout + WAL pragmas | ✅ |
| Frontend `internalNotes?: string` is optional | ✅ |
| Frontend section only renders when non-empty | ✅ |
| Tenant ID not exposed in error messages | ✅ |
| `internal_notes` defaults to `""` in CreateTicket handler | ✅ |
| Ticket embedder includes `InternalNotes` in content | ✅ |
| Inference function handles nil TenantID gracefully | ✅ |
| Inference function handles nil vectorDB gracefully | ✅ |
| Inference function handles empty search results gracefully | ✅ |
