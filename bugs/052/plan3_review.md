# Adversarial Review: bugs/052 Per-Ticket RAG Indexing Prototype (Final Revision)

**Reviewer:** Kilo (Senior Go Architect)
**Plan File:** `/home/chaschel/Documents/go/bchat/bugs/052/plan3.md`
**Date:** 2026-07-31
**Verdict:** **CONDITIONAL APPROVAL -- 2 Rework Items, 3 Nits**

---

## Executive Summary

The revised plan successfully addresses all 8 findings from Round 2:

1. **TOCTOU race** → per-ticket `sync.Map` mutex added (Step 2)
2. **Missing tenant ID** → `TenantID` passed in all find structs (Steps 6, 7)
3. **Stale `memo.ID`** → documented as immutable
4. **Multiple COMMENT relations** → iterate all parents (Step 7)
5. **Dedup path not hooked** → hook added (Step 4)
6. **Validation timing** → wait/retry loop added (Step 8)
7. **`GetAgentSourceFile` error swallowed** → logged as warning
8. **Inference threshold** → documented workaround

However, adversarial review surfaces **2 rework items** (security + correctness) and **3 nits** that must be resolved before implementation.

Approval is withheld until the rework items are patched in the plan.

---

## Finding 1 -- HIGH: Tenant-Nil Bypass in Hooks Leaks Cross-Tenant Tickets

**Plan code (Step 6 -- CreateMemoComment hook):**
```go
tenantID := GetTenantIDFromContext(ctx)
...
tickets, _ := s.Store.ListTickets(ctx, &store.FindTicket{
    Description: &descriptionLink,
    TenantID:    tenantID,
})
```

**Plan code (Step 7 -- UpdateMemo hook):**
```go
tenantID := GetTenantIDFromContext(ctx)
...
parentMemo, _ := s.Store.GetMemo(ctx, &store.FindMemo{
    ID:       &parentMemoID,
    TenantID: tenantID,
})
...
tickets, _ := s.Store.ListTickets(ctx, &store.FindTicket{
    Description: &descriptionLink,
    TenantID:    tenantID,
})
```

**Problem:** If `GetTenantIDFromContext(ctx)` returns `nil` (e.g., superuser, unscoped JWT, or multi-tenant user without selection), `FindTicket.TenantID` is `nil`. The SQLite implementation (`store/db/sqlite/ticket.go:71-78`) only adds `tenant_id = ?` when `find.TenantID != nil`. Without that filter, the query matches ticket descriptions across ALL tenants:

```sql
SELECT ... FROM tickets
WHERE description = '/m/abc123'
-- NO tenant_id filter when FindTicket.TenantID is nil
```

**Concrete exploit path:**
1. Superuser edits a comment in Tenant B on memo `/m/shared-uid`.
2. `GetTenantIDFromContext` returns `nil`.
3. `ListTickets` without `TenantID` returns Ticket #99 from Tenant A that also links to `/m/shared-uid`.
4. `IndexTicketContent` indexes Tenant A's ticket with Tenant B's content.

This is a **cross-tenant data write violation**. The read path returns the "wrong" ticket for indexing, and the write path embeds Tenant B data into Tenant A's RAG index.

### Required Fix

Add an explicit nil guard before every store call in the new hooks:

```go
tenantID := GetTenantIDFromContext(ctx)
if tenantID == nil {
    return // skip indexing for unscoped requests
}
```

This matches the existing codebase pattern used throughout `memo_service.go` (e.g., lines 347, 480, 552, 585, 641).

---

## Finding 2 -- MEDIUM: Unbounded Memory Growth from `ticketIndexMu`

**Plan code (Step 2):**
```go
var ticketIndexMu sync.Map

func (s *Service) IndexTicketContent(...) {
    muKey := fmt.Sprintf("%d:%d", tenantID, ticket.ID)
    muVal, _ := ticketIndexMu.LoadOrStore(muKey, &sync.Mutex{})
    ...
}
```

**Problem:** Every ticket ever indexed inserts an entry into `ticketIndexMu`. Entries are never removed. In a long-running server:

- 10,000 tickets indexed → 10,000 mutexes in the map (~1.6 MB)
- 1,000,000 tickets indexed → 1,000,000 mutexes (~160 MB)

For a prototype with one ticket (#120), this is negligible. For production, this becomes a memory leak.

### Required Fix

For the prototype, document the limitation and accept it.

For production, add a cleanup mechanism. Two options:

**Option A (simple):** Use a `sync.Map` with a background cleanup goroutine that removes entries older than N minutes. Since `IndexTicketContent` is async and short-lived (~1-3s), the mutex is only needed during that window.

**Option B (correct):** Move the mutex inside `ReindexFileVersion` by adding an optional content-upsert step there, eliminating the need for a separate per-ticket mutex entirely. This couples the dedup logic to the indexing step but avoids the unbounded map.

---

## Finding 3 -- MEDIUM: `UpdateMemo` Hook Fetches Stale `memo` but Ignores Error

**Plan code (Step 7):**
```go
parentMemo, _ := s.Store.GetMemo(ctx, &store.FindMemo{
    ID:       &parentMemoID,
    TenantID: tenantID,
})
if parentMemo == nil {
    continue
}
```

**Problem:** `GetMemo` error is swallowed (`_`). If the query fails (DB error, tenant isolation mismatch), the hook silently skips re-indexing the parent ticket. This is consistent with the "best-effort" pattern elsewhere in the codebase, but worth noting.

### Nit

The error swallow is acceptable for a "fire-and-forget" background goroutine. Add a debug-level log so failures are visible in trace logging:

```go
parentMemo, err := s.Store.GetMemo(ctx, ...)
if err != nil {
    slog.Debug("failed to load parent memo for comment-edit reindex, skipping", "memo_id", parentMemoID, "error", err)
    continue
}
```

---

## Finding 4 -- MEDIUM: Step 7 goroutine loop captures `ticket` by reference, not value

**Plan code (Step 7):**
```go
for _, rel := range parentRelations {
    ...
    go func(t *store.Ticket) {
        ctx := context.WithoutCancel(ctx)
        ...
        _, idxErr := s.agentHandler.GetService().IndexTicketContent(ctx, *t.TenantID, t, comments, false)
    }(&ticket)
}
```

**Observation:** The `&ticket` argument passes the address of the loop-scoped `ticket` variable. In Go, loop variables are reused across iterations, so `&ticket` would point to the same address in each iteration. If the loop runs concurrently and `ticket` is reassigned, all goroutines would see the last iteration's value.

**Correction:** The plan correctly passes `&ticket` as an argument (`t` parameter in the goroutine). In Go, **function arguments are evaluated at call time**, so `&ticket` is evaluated and passed before the goroutine starts. Each goroutine receives its own pointer to the variable at that moment.

However, this concurrency pattern is fragile. If the caller continues to mutate `ticket` after launching the goroutine, the goroutine sees the mutated value. In the plan's case, `ticket` is not mutated after the goroutine launch, so it's safe.

### Nit

For clarity and to prevent future misuse, capture the ticket by value:

```go
go func(ticketCopy store.Ticket) {
    ctx := context.WithoutCancel(ctx)
    comments, err := s.getTicketComments(ctx, &ticketCopy)
    ...
}(*ticket)
```

This is not a correctness bug but a defensive practice.

---

## Finding 5 -- LOW: Dedup Path Validation Incorrect

**Plan validation (Step 8):**
```
| Dedup path indexes | Create duplicate ticket -> check source file exists | version=1 |
```

**Problem:** "Version=1" assumes the content hash matches the existing source file. If the deduplicated ticket's title/description differ from what was originally indexed (e.g., the existing ticket was updated but the new request reverts to old values), the hash may match version=1, OR it may create a new version if the content differs.

More importantly, the plan doesn't specify HOW to trigger the dedup path. The `CreateTicket` handler deduplicates based on some matching logic (lines 108-155 in ticket_service.go). The plan should document the exact input needed to hit this path.

### Nit

Change validation to:
```
| Dedup path indexes | Create duplicate ticket -> check source file version | Same version as existing (no new row if hash matches) |
```

And add a note: "Trigger dedup by creating a ticket with matching title/description."

---

## Finding 6 -- LOW: Step 8 "Same content no version bump" Validation Order

**Plan Step 8:**
```
# 9. Edit ticket #120 with SAME content, verify NO version increment
curl -X PUT http://localhost:5230/api/v1/tickets/120 \
  -d '{"title":"Bug #002: Repair Frontend Dependency Provenance (updated)"}'

sleep 3

sqlite3 build/data/memos_dev.db \
  "SELECT count(*) FROM agent_source_files WHERE file_type='ticket' AND tenant_id=19"
# Expected: still version=2, not version=3
```

**Problem:** Step 7 edited the title to "Bug #002: Repair Frontend Dependency Provenance (updated)". Step 9 submits the SAME title again. The `agent_source_files` content includes the title, so the content hash WILL change (the title has "(updated)" appended), and a new version WILL be created. The validation expectation "still version=2" is WRONG.

The plan's validation for "same content no version bump" should use the exact same payload as the previous successful edit, not a different payload.

### Nit

Fix Step 9 to use identical content to Step 7:
```bash
# 9. Edit ticket #120 with SAME content as step 7, verify NO version increment
curl -X PUT http://localhost:5230/api/v1/tickets/120 \
  -d '{"title":"Bug #002: Repair Frontend Dependency Provenance (updated)"}'
# Expected: count remains at 2 versions
```

Or better: step 8 should also include a "same content" test after the first edit.

---

## Finding 7 -- NIT: `sha256Hash` Unexplained in Step 2

**Plan Step 2:**
```go
contentHash := sha256Hash(content)
```

**Problem:** The plan doesn't define `sha256Hash`. The codebase has `ContentHash(content string) string` in `parser.go:86`. Either reference the existing function or define the helper explicitly.

### Nit

Change to:
```go
contentHash := agent.ContentHash(content) // parser.go:86, SHA256 hex
```

Or add an import of the `agent` package in `service.go` if not already present.

---

## Review Summary

| # | Finding | Severity | Rework Required? |
|---|---------|----------|------------------|
| 1 | Tenant-nil bypass leaks cross-tenant tickets | HIGH | YES |
| 2 | `ticketIndexMu` unbounded memory growth | MEDIUM | Document |
| 3 | `GetMemo` error swallowed in UpdateMemo hook | MEDIUM | Debug log |
| 4 | Goroutine captures loop variable by reference | MEDIUM | Capture by value |
| 5 | Dedup path validation incorrect | LOW | Nit |
| 6 | Step 8/9 validate wrong content for hash match | LOW | Nit |
| 7 | `sha256Hash` undefined | NIT | Reference or define |
| 8 | (none) | -- | -- |

---

## Minimum Plan Changes

1. **Step 6, Step 7 hooks:** Add `if tenantID == nil { return }` guard after `GetTenantIDFromContext(ctx)`.
2. **Step 2:** Document `ticketIndexMu` memory growth and add a production cleanup TODO.
3. **Step 7:** Add debug log for swallowed `GetMemo` error. Capture `ticket` by value in goroutine.
4. **Step 8:** Fix validation steps 9 to use identical content as step 7. Fix dedup validation expectation.

---

## Rollback Note

The plan's Section 8 rollback remains correct and sufficient.

---

## Recommendation

The plan is **nearly ready**. Finding 1 (tenant-nil bypass) is a security issue that must be fixed. Finding 2 (memory growth) should be documented for prototype scope. The remaining nits are clarifications for the implementation agent.

Proceed to implementation only after Findings 1 and 2 are patched in `plan3.md`.
