# Plan Review: Internal Notes + RAG-Based Bug Inference

**Reviewer:** Senior Go Architect
**Date:** 2026-07-30
**Plan:** `plan.md` (Bug 051)
**Verdict:** **Rework** — concept is sound, but critical execution gaps prevent implementation.

---

## Summary

The plan correctly identifies a valuable problem (trapped institutional knowledge in bug folders) and proposes a reasonable high-level architecture (internal_notes field + RBAC + import script + inference pipeline). However, **6 critical issues** make this unready for implementation — missing Postgres migration, nonexistent function references, compilation-breaking signature changes, packaging conflicts, unhandled LLM bottlenecks, and missing tenant isolation for vector search.

---

## Critical Issues (Blockers)

### 1. Postgres Migration & Driver Completely Missing

The plan covers SQLite-only: one migration file, one driver file. **No Postgres migration**, **no Postgres LATEST.sql update**, **no Postgres driver changes**.

- `store/db/postgres/ticket.go` exists (257 lines) with identical CRUD methods — all need `internal_notes` columns, bind params, RETURNING clauses, Scan targets.
- `store/migration/postgres/LATEST.sql` exists (1029 lines) — the tickets table (line 641) needs the column.
- No `store/migration/postgres/0.35/00__tickets_add_internal_notes.sql` is created.

Since CockroachDB is Postgres-compatible and the plan's entire vector-search value prop depends on CockroachDB, this omission is a **blocker**. The plan needs full Postgres parity.

### 2. `CheckUserPermission` Does Not Exist

The plan references `h.service.CheckUserPermission(ctx, userID, *tenantID, "ticket:internal_notes")` at three call sites in `ticket_service.go`.

- `CheckUserPermission` is **not defined anywhere** in the codebase.
- `ticket_service.go` is in package `v1` (not `agent`), so it cannot call `Handler.hasPermission()` which takes `echo.Context`.
- The closest real function is `agent.ResolveEffectivePermissions(ctx, store, tenantID, userID int32)` in `permissions.go:138`. This *is* accessible from `v1` (it's exported, in a subpackage), but the plan must detail this dependency and the import path.
- The `APIV1Service` struct does have an `agentHandler *agent.Handler` field (v1.go:53), but the ticket handlers never use it.

**Action required:** The plan must specify the real permission-checking mechanism — either `agent.ResolveEffectivePermissions` called directly, or a new helper on `APIV1Service`.

### 3. `convertTicketFromStore` Signature Change Breaks Compilation

Changing from `func(*store.Ticket) *Ticket` (single arg) to `func(*store.Ticket, *store.User, bool) *Ticket` (3 args) will break **every existing call site**. The plan does not mention or address this.

Call sites in `ticket_service.go` alone:
- Line ~150 (ListTickets response mapping)
- Line ~162 (ListTickets append)
- Line ~207 (UpdateTicket response)
- Line ~326 (UpdateTicket return)
- Line ~427 (GetTicket return)

**Action required:** Either (a) detail every caller update, or (b) keep the conversion function single-arg and do RBAC filtering in each handler after conversion — e.g., `if !hasPerm { resp.InternalNotes = "" }`.

### 4. `cmd/seed/` Package Conflict

The plan proposes `cmd/seed/import_bugs.go` with `package main + func main()`. But `cmd/seed/seed_demo_tickets.go` *already* has `package main + func main()`. Two `main()` functions in the same package will **not compile**.

**Action required:** Either:
- Create a separate command directory: `cmd/import-bugs/main.go`
- Use build tags: `//go:build import_bugs`
- Extend the existing seed script with a flag: `go run ./cmd/seed --import-bugs`

### 5. LLM Dependency During Import

The import script generates LLM-summarized `internal_notes` for ~130 tickets. The plan assumes synchronous LLM calls but provides:
- No batch strategy (130 sequential LLM calls could take 30+ minutes)
- No timeout handling
- No fallback for LLM unavailability
- No retry logic

If the LLM is unavailable mid-import, 60+ tickets may be created with empty `internal_notes`, polluting the vector index. The fallback section only addresses CockroachDB, not LLM failures.

**Action required:** Design a batch/async approach — e.g., create tickets with placeholder `internal_notes`, then enqueue a background job for LLM enrichment. Or at minimum: parallelize with worker pool, add timeout + retry, and provide a meaningful fallback.

### 6. Cross-Tenant Vector Search Isolation

The inference pipeline (`inferResolutionForNewTicket`) searches CockroachDB for similar tickets and copies their `internal_notes` into the new ticket's `internal_notes`. There is **no tenant filter** on the vector search.

This means:
- A ticket created in tenant A could surface internal notes from tenant B's bugs.
- Sensitive cross-tenant information would be embedded in the new ticket's `internal_notes`.

The current `embedTenantTickets` function does filter by `TenantID`, but the plan's inference function does not show a tenant-scoped query.

**Action required:** The vector search must filter by `tenant_id` at query time. Additionally, consider whether internal_notes should be included in the embedding content at all (it makes semantic search more useful but increases sensitivity).

---

## Nits (Minor Issues)

### 7. `AllPermissions` and `PermissionPresets` Not Updated

The plan adds `PermTicketInternalNotes = "ticket:internal_notes"` but does not mention:
- Adding it to `AllPermissions` slice (`permissions.go:24`). Without this, `ValidatePermissions` will reject it.
- Adding it to `PermissionPresets` map (`permissions.go:40`). The plan says Analyst/Editor/Tenant Admin get it, but the code won't be updated.

### 8. RBAC Check Called Inside ListTickets Loop

The plan pseudocode calls `CheckUserPermission` inside the ListTickets loop. Since the user and tenant don't change per iteration, this should be resolved **once before the loop**. The per-iteration pattern is fine for `convertTicketFromStore` but the permission check itself is wasteful.

### 9. `internalNotes` in Embedding Content Creates Sensitivity Concern

The plan enhances the ticket embedder to include `internalNotes` in the embedding content. This makes internal notes searchable via vector search. Since the vector DB doesn't enforce RBAC at search time, any user who can query the vector index could discover internal notes content through semantic search. This should at minimum be documented as a trade-off.

### 10. No Tests Mentioned

The verification plan includes `go test ./...` commands but doesn't mention writing specific tests:
- Unit tests for RBAC logic in `convertTicketFromStore`
- Integration test for the migration (up and down)
- Test for the import script's parser (malformed .md files, empty folders, etc.)
- Test for the inference fallback path

### 11. No Postgres Migration Validation

The plan lists `task validate:schema` in references but doesn't mention updating Postgres migration files or running validation. With the new Postgres version directory and updated `LATEST.sql`, the validation step is important.

### 12. Import Script Versioning

The plan hardcodes migration `0.35`. The latest migration version in both SQLite and Postgres should be verified before deciding the version number. (Currently SQLite has versions up to 0.34, so 0.35 is likely correct, but Postgres also needs parity.)

### 13. `discovery_context` Relationship

The tickets table already has a `discovery_context TEXT` field. The plan should clarify the relationship between `discovery_context` and `internal_notes` — are they complementary (one for system-generated discovery, one for human notes) or is `internal_notes` replacing it?

---

## Recommendations

### If Reworking (Preferred Approach)

1. **Add Postgres parity**: Create `store/migration/postgres/0.35/00__tickets_add_internal_notes.sql`, update Postgres LATEST.sql, and add `internal_notes` to all Postgres driver CRUD methods.

2. **Fix permission check mechanism**: Replace `CheckUserPermission` references with the actual API — either `agent.ResolveEffectivePermissions(ctx, s.Store, tenantID, userID)` or a thin wrapper on `APIV1Service`.

3. **Preserve `convertTicketFromStore` signature**: Either keep it single-arg and filter in each handler, or use an options pattern (`func(*store.Ticket, ...Option)`) to avoid breaking existing callers.

4. **Fix import script packaging**: Move to `cmd/import-bugs/main.go`.

5. **Design async import flow**: Create tickets first, enqueue LLM enrichment as background job. Add worker pool, timeout, retry, and fallback to `"No summary generated"`.

6. **Add tenant isolation to vector search**: Ensure `inferResolutionForNewTicket` filters CockroachDB queries by `ticket.TenantID`.

### Quick Fixes (For Nits)

7. Update `AllPermissions` and `PermissionPresets` in `permissions.go`.
8. Move permission check outside ListTickets loop.
9. Add test coverage for RBAC, migration, parser.
10. Clarify `discovery_context` vs `internal_notes` relationship.

---

## Open Questions for Author

1. Should `internalNotes` be excluded from ticket embedding content to prevent sensitivity leakage via vector search?
2. Is the import script a one-time migration or a repeatable tool? (Affects error handling strategy.)
3. Should `internal_notes` support markdown formatting, or is it plaintext? (Affects frontend rendering.)
4. Does `discovery_context` serve the same purpose as `internal_notes`? Should they be merged?

---

## References Checked

| Reference | Status |
|-----------|--------|
| `store/ticket.go` | Verified — no `InternalNotes` field |
| `store/db/sqlite/ticket.go` | Verified — 11 columns, no internal_notes |
| `store/db/postgres/ticket.go` | **Exists** — plan omits this file |
| `store/migration/sqlite/LATEST.sql` | Verified — no internal_notes column |
| `store/migration/postgres/LATEST.sql` | **Exists** — plan omits this file |
| `server/router/api/v1/ticket_service.go` | Verified — no `CheckUserPermission`, no `InternalNotes` |
| `server/router/api/v1/agent/permissions.go` | Verified — `AllPermissions` and `PermissionPresets` need updates |
| `server/router/api/v1/agent/ticket_embedder.go` | Verified — enhanced embedding is straightforward |
| `server/router/api/v1/agent/handlers.go` | Verified — `hasPermission` exists but takes `echo.Context` |
| `web/src/pages/TicketDetail.tsx` | Verified — no `internalNotes` field |
| `cmd/seed/seed_demo_tickets.go` | Verified — `package main` with `func main()` conflict |
| `APIV1Service` (v1.go:32) | Verified — has `agentHandler *agent.Handler` field |
