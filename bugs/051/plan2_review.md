# Plan2 Review: Internal Notes + RAG-Based Bug Inference (Revised)

**Reviewer:** Senior Go Architect
**Date:** 2026-07-30
**Plan:** `plan2.md` (Bug 051 — Post-Adversarial-Revision)
**Verdict:** **Approved with Nits** — All critical issues from `plan.md` are resolved. Remaining items are minor documentation/completeness gaps.

---

## Changelog Verification

| # | Finding (from `plan_review.md`) | Status | Evidence |
|---|-------------------------------|--------|----------|
| 1 | Postgres Migration Missing | **Fixed** ✅ | Both `store/migration/postgres/0.35/00__tickets_add_internal_notes.sql` and `LATEST.sql` update added |
| 2 | `CheckUserPermission` doesn't exist | **Fixed** ✅ | Uses `agent.ResolveEffectivePermissions()` + proposed `HasPermission` helper |
| 3 | `convertTicketFromStore` breaks compilation | **Fixed** ✅ | Single-arg signature preserved; `filterInternalNotes` helper added |
| 4 | `cmd/seed/` package conflict | **Fixed** ✅ | Moved to `cmd/import-bugs/main.go` |
| 5 | LLM dependency during import | **Fixed** ✅ | Async worker pool (5 goroutines, 30s timeout, 2 retries, fallback message) |
| 6 | Cross-tenant vector search isolation | **Fixed** ✅ | `tenant_id = $2` filter in search SQL |
| 7 | `AllPermissions`/`PermissionPresets` not updated | **Fixed** ✅ | Both explicitly updated |
| 8 | RBAC check in ListTickets loop | **Fixed** ✅ | Resolved once before loop |
| 9 | `internalNotes` sensitivity in embeddings | **Documented** ✅ | Trade-off note added |
| 10 | No tests mentioned | **Fixed** ✅ | Comprehensive test plan (unit, integration, import, inference tests) |
| 11 | Postgres migration validation | **Fixed** ✅ | `task validate:schema` and `task validate:parity` added |
| 12 | Import script versioning | **No change** ✅ | Verified: 0.35 is correct |
| 13 | `discovery_context` relationship | **Clarified** ✅ | Complementary fields — system-generated vs human annotations |

---

## New Findings (Nits)

### N1. Agent Service Call Chain Not Specified

The plan shows `go s.inferResolutionForNewTicket(ctx, ticket)` inside `CreateTicket` on `APIV1Service` (ticket_service.go), but `inferResolutionForNewTicket` is a method on `*agent.Service`, not `*APIV1Service`.

`APIV1Service` has `agentHandler *agent.Handler` (v1.go:53), and `Handler` exposes `GetService() *Service` (handlers.go:51).

**Actual call should be:**
```go
go s.agentHandler.GetService().inferResolutionForNewTicket(ctx, ticket)
```

This pattern already exists in the codebase at `memo_service.go:1225`:
```go
aiReply, err := s.agentHandler.GetService().ProcessTicketChat(...)
```

**Action:** Update the trigger point in the plan to use the correct call chain.

### N2. `HasPermission` Helper Not in Implementation Order

The plan proposes creating a public `HasPermission` wrapper (line 309-313) but omits it from the implementation order table (step 6 only covers "Permission constant + presets").

**Action:** Add `HasPermission` helper to step 6, or note it as a sub-task of the permissions.go changes.

### N3. Import Script / CreateTicket `internal_notes` Conflict

Two conflicting statements in the plan:

- Line 403-405: **CreateTicket handler** — `internal_notes defaults to "", not settable via create API`
- Line 444: **Import Phase 1** — `Set internal_notes = "Pending summary..."`

The import script cannot set `internal_notes` during CreateTicket if the handler doesn't accept it. Options:
1. Create ticket (internal_notes = ""), then immediately UpdateTicket → adds an extra API call per ticket
2. Allow CreateTicket to accept internal_notes for seed/import contexts (less secure, needs RBAC guard)

**Action:** Clarify which approach is intended and document the extra update step if option 1.

### N4. Raw SQL vs Existing Search API

Lines 563-571 show raw SQL for the vector search query:
```sql
SELECT ... FROM agent_vectors WHERE tenant_id = $2 ...
```

The existing `CockroachVectorDB.Search()` method (vectordb_cockroach.go:283) already handles `tenant_id` filtering via `SearchQuery.TenantID` and uses parameterized queries. The inference function should call:

```go
result, err := s.vectorDB.Search(ctx, SearchQuery{
    TenantID:     *ticket.TenantID,
    QueryText:    ticket.Title + "\n" + ticket.Description,
    ContentTypes: []string{"ticket"},
    TopK:         5,
    MinScore:     0.7,
})
```

**Action:** Replace the raw SQL example with the `Search()` API call. The raw SQL is misleading — it suggests handwritten queries against `agent_vectors` which bypasses the existing abstraction.

### N5. Import Interrupt Resilience

Phase 2 (LLM summary generation) uses a worker pool but no mechanism handles CLI interruption mid-phase. If the process is killed:
- Tickets exist in the database with `internal_notes = "Pending summary..."`
- No record of which tickets were completed vs pending
- Re-running the import would need to detect and re-process

**Action:** Either (a) document this as a known limitation with a manual cleanup command, or (b) add a checkpoint/tracking mechanism so re-runs skip already-summarized tickets.

---

## Overall Assessment

**Approved with Nits.** All 13 critical issues from the initial review are properly resolved. The architecture is sound, the RBAC design is correct, the async import pattern is well-considered, and the test coverage is appropriate.

The 5 nits above are minor documentation gaps and should not block implementation. Address them during coding:

- **N1, N2** should be fixed in the plan for accuracy.
- **N3, N4, N5** can be resolved during implementation with appropriate design decisions.
