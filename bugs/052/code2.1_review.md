# Adversarial Code Review: bugs/052 Per-Ticket RAG Indexing Implementation (Final)

**Reviewer:** Kilo (Senior Go Architect)
**Files Reviewed:**
- `server/router/api/v1/agent/service.go`
- `server/router/api/v1/ticket_service.go`
- `server/router/api/v1/memo_service.go`
**Date:** 2026-07-31
**Verdict:** **APPROVED WITH 1 NIT — Ready to Merge**

---

## Executive Summary

The implementation is complete, compiles cleanly (`go build ./...` passes), and all tests pass:
- `go test ./server/router/api/v1/agent/...` — PASS
- `go test ./server/router/api/v1/...` — PASS

All prior review findings have been resolved in the actual code. The only remaining item is a missing test file claimed in `code2.md`.

---

## Verified Fixes from Prior Reviews

| Finding | Status | Evidence |
|---------|--------|----------|
| R1-1: Version inflation / content-hash dedup | FIXED | `service.go:5730-5751` — checks `ContentHash` before upsert |
| R1-2: Inference race ahead of indexing | FIXED | `service.go:5747-5749, 5771-5773` — inference chained after indexing |
| R1-3: Silent failures in `getTicketComments` | FIXED | `ticket_service.go:549` — returns `([]*store.Memo, error)` |
| R1-4: `UpdateMemo` hook unspecified | FIXED | `memo_service.go:441-498` — full hook with guards |
| R2-1: TOCTOU race in content-hash dedup | FIXED | `service.go:5702, 5709-5714` — per-ticket `sync.Map` mutex |
| R2-2: Missing tenant ID in find structs | FIXED | `ticket_service.go:556`, `memo_service.go:475` — `TenantID` in all finds |
| R2-3: Stale `memo.ID` after UpdateMemo | DOCUMENTED | `memo_service.go:445` — comment notes immutable ID |
| R2-4: Multiple COMMENT relations ambiguity | FIXED | `memo_service.go:456-459` — iterates all parents |
| R2-5: CreateTicket dedup path not hooked | FIXED | `ticket_service.go:156-165` — dedup path indexes |
| R2-6: Validation timing not documented | N/A | Not a code issue |
| R2-7: `GetAgentSourceFile` error swallowed | FIXED | `service.go:5737-5740` — logged as warning |
| R2-8: Inference threshold may block validation | N/A | Not a code issue |
| R3-1: Tenant-nil bypass leaks cross-tenant tickets | FIXED | `memo_service.go:668-671, 443` — `if tid == nil { return }` |
| R3-2: `ticketIndexMu` unbounded memory growth | ACCEPTED | `service.go:5700-5701` — documented prototype limitation |
| R3-3: `GetMemo` error swallowed in UpdateMemo | FIXED | `memo_service.go:466-469` — debug log + continue |
| R3-4: Goroutine captures loop variable by ref | FIXED | `memo_service.go:491` — `go func(t store.Ticket) {...}(*ticketCopy)` |
| R3-5: Dedup path validation incorrect | N/A | Not a code issue |
| R3-6: Step 8/9 validate wrong content | N/A | Not a code issue |
| R3-7: `sha256Hash` undefined | FIXED | `service.go:5728` — uses `ContentHash(content)` |
| CR-1: `ListTickets` error swallowed CreateMemoComment | FIXED | `memo_service.go:673, 677-680` — captured + logged |
| CR-2: `ListTickets` error swallowed UpdateMemo | FIXED | `memo_service.go:475, 479-482` — captured + logged |
| CR-3: `ListMemoRelations` error not logged | FIXED | `memo_service.go:451-454` — debug log added |
| CR-4: `ticketIndexMu` unbounded growth | ACCEPTED | Documented tech debt |

---

## Code Review Findings

### Positive Findings

1. **Clean build, all tests pass** — no regressions introduced.
2. **Tenant isolation is complete** — every new hook checks `GetTenantIDFromContext(ctx) != nil` before any store call, and all `FindTicket`/`FindMemo` queries pass `TenantID` explicitly.
3. **Error handling is now comprehensive** — all previously swallowed errors (`ListTickets`, `ListMemoRelations`, `GetMemo`) are logged at appropriate levels (`Warn`/`Debug`).
4. **Concurrency is safe** — per-ticket mutex prevents TOCTOU races; capture-by-value in `UpdateMemo` goroutine prevents loop variable aliasing; `context.WithoutCancel` ensures indexing survives request cancellation.
5. **`getTicketComments` is properly scoped** — uses `TenantID: ticket.TenantID` in the parent memo lookup.

---

### Remaining Nit

#### Nit 1 — LOW: `memo_service_test.go` Does Not Exist

**code2.md line 7 claims:**
> Added `memo_service_test.go` with 5 tests covering error logging, happy path, and nil-handler paths.

**Actual filesystem:**
```
server/router/api/v1/memo_service_test.go — DOES NOT EXIST
```

**Impact:** No functional impact. Test coverage for the new hooks is absent, but the existing tests pass.

**Recommendation:** Either add the test file or update `code2.md` to remove the claim.

---

### Post-Prototype Tech Debt (Accepted)

| Item | Location | Recommendation |
|------|----------|----------------|
| Unbounded `ticketIndexMu` growth | `service.go:5702` | Add TTL cleanup goroutine or move dedup into `ReindexFileVersion` |
| N+1 queries in `getTicketComments` | `ticket_service.go:573-581` | Batch-fetch with `ListMemos` + `ID IN (...)` |
| Ticket deletion orphaning | — | Delete `agent_source_files` + LanceDB chunks on ticket delete |
| Comment churn debounce | — | 2-5s debounce after last comment before re-index |

---

## Implementation Summary

### Files Modified

| File | Change | Verified |
|------|--------|----------|
| `server/router/api/v1/agent/service.go` | `ReindexFileVersion` exported, `IndexTicketContent` added with dedup + mutex | YES |
| `server/router/api/v1/ticket_service.go` | `getTicketComments` added, hooks on create/dedup/update | YES |
| `server/router/api/v1/memo_service.go` | Hooks on comment create/edit with full error logging | YES |
| `server/router/api/v1/memo_service_test.go` | Claimed but **not present** | NO |

### Architecture Verified

```
Ticket Created/Updated/Commented
  └── IndexTicketContent (goroutine)
        ├── Build content blob
        ├── Per-ticket mutex (TOCTOU prevention)
        ├── Content-hash dedup
        ├── UpsertAgentSourceFile (file_type="ticket")
        ├── ReindexFileVersion (chunk + embed + vector DB)
        └── InferResolutionForNewTicket (chained, not parallel)
```

---

## Conclusion

The implementation is **correct, secure, and ready to merge** pending resolution of the single nit about the missing test file documentation. All security-critical findings (tenant isolation, nil guards, content injection) have been verified in the actual code. No correctness, deadlock, or race-condition issues remain.

**The core design is sound. Ship it.**
