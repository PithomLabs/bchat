# Adversarial Review: bugs/052 Per-Ticket RAG Indexing Prototype

**Reviewer:** Kilo (Senior Go Architect)
**Plan File:** `/home/chaschel/Documents/go/bchat/bugs/052/plan.md`
**Date:** 2026-07-31
**Verdict:** **BLOCKED — Rework Required**

---

## Executive Summary

The plan correctly identifies the gap (`InferResolutionForNewTicket` searches `["ticket"]` but no code indexes ticket content), but the implementation steps contain **one critical correctness bug**, **one high-severity race condition**, and **several incomplete/inaccurate specifications** that will cause the prototype to fail or produce misleading results.

Approval is withheld until the findings below are resolved in the plan.

---

## Finding 1 — CRITICAL: Version Inflation on Reindex Failure

**Plan claim:** "Same content re-indexed: Content hash dedup; version still increments (content may differ slightly)"

**Verdict on claim: FALSE.** `UpsertAgentSourceFile` (`store/db/sqlite/agent.go:1117-1144`) executes `SELECT COALESCE(MAX(version), 0) + 1` and inserts a brand-new version row on every call. There is no content_hash guard, no upsert-on-hash logic. Every call to `UpsertAgentSourceFile` increments version, period.

### Impact chain

1. Ticket created -> `IndexTicketContent` calls `UpsertAgentSourceFile` -> SQLite row version=N
2. `ReindexFileVersion` attempts embedding -> API times out -> returns error
3. SQLite now has orphaned version=N (not marked active, not in vector DB)
4. Ticket edited -> `IndexTicketContent` runs again -> `UpsertAgentSourceFile` creates version=N+1
5. Sequence repeats. The SQLite `agent_source_files` table grows unboundedly for this single ticket.

### Recovery path problem

On the NEXT successful `ReindexFileVersion`, the call receives version=N+1. Vector DB gets N+1 and sets active=N+1. Version N is still orphaned in SQLite but never indexed. The `InferResolutionForNewTicket` search will return N+1 chunks, but if version N was supposed to be the "latest" content at some point, there is a gap.

### Required Fix

**Option A (recommended for prototype):** Make `IndexTicketContent` idempotent per content hash. Before `UpsertAgentSourceFile`, query for the latest version; if `content_hash` matches, skip upsert and call `ReindexFileVersion` on the existing version. If it doesn't match, upsert and reindex.

**Option B (acceptable fallback):** Document that `UpsertAgentSourceFile` always creates a new version and that this is a known limitation of the current store API. Accept version bloat as prototype debt.

---

## Finding 2 — HIGH: Race Between Inference and Indexing

**Code evidence:** `ticket_service.go:166` launches `InferResolutionForNewTicket` as `go s.agentHandler.GetService().InferResolutionForNewTicket(...)`. The plan adds `IndexTicketContent` as a second goroutine immediately after.

There is no synchronization between the two goroutines. Possible outcomes:

| Timing | Result |
|--------|--------|
| Indexing finishes first | Search finds the ticket -> suggestions populated |
| Search finishes first | Search returns empty -> `internal_notes` stays empty or stale |
| Concurrent | Undefined; search may see partial/empty vector state |

Because `InferResolutionForNewTicket` is extremely fast (~100ms vector search) compared to embedding (~1-3s), **the race almost always resolves in favor of the search winning**. A newly created ticket will almost never get suggestions on its own creation event.

This means the prototype's validation step ("Check internal_notes has suggestions") will likely FAIL for the newly created ticket that triggers inference. The plan only works if Ticket #120 was pre-indexed and a DIFFERENT ticket is created, which the plan does mention -- but it's a fragile manual setup.

### Required Fix

Make `IndexTicketContent` call `InferResolutionForNewTicket` after it successfully completes indexing, passing the ticket it just indexed. Alternatively, launch `InferResolutionForNewTicket` from within `IndexTicketContent` after the `ReindexFileVersion` call succeeds. Do NOT launch both independently from the ticket-creation handler.

---

## Finding 3 — HIGH: Silent Failures in `getTicketComments`

**Plan code:** Step 2 helper uses `comments, _ := s.getTicketComments(ctx, ticket)` (error swallowed).

If `getMemo`, `ListMemoRelations`, or `GetMemo` fails, the ticket is indexed with `comments=nil` and no error is logged. Users get incomplete RAG output with no indication of failure. This is especially problematic for the UpdateTicket hook where the failure would mean the comment-rich ticket gets indexed with only title+description, silently degrading retrieval quality.

### Required Fix

Return `([]*store.Memo, error)` from the plan's `getTicketComments` helper, propagate the error in both the ticket creation and update goroutines, and log on failure. If comments cannot be fetched, index with title+description only but surface the failure in logs.

---

## Finding 4 — HIGH: `UpdateMemo` Hook Is Unspecified

The plan provides pseudo-code for hooks in `CreateMemoComment` and `ticket_service.go` (create + update), but Step 5 says only:

> "In `UpdateMemo` (after memo content is updated), if the memo is a comment on a ticket, re-index the parent ticket."

No implementation pseudo-code is provided. `UpdateMemo` is a gRPC handler (`memo_service.go:324-457`) with complex update-mask logic spanning many fields. The hook must:

1. Check if `path == "content"` was in the update mask (only content changes need re-indexing)
2. Determine if the updated memo is a comment on a ticket (query `MemoRelation` where `MemoID = memo.ID AND Type = COMMENT`)
3. If so, find the parent ticket (query tickets where `description = '/m/' + parentMemo.UID`)
4. Fetch all comments for the ticket
5. Trigger `IndexTicketContent`

Without explicit pseudo-code, the implementer can place the hook in the wrong location (before update is persisted, before relations are checked, etc.).

### Required Fix

Add Step 5 pseudo-code for `UpdateMemo` showing exactly where to insert the hook and what guard conditions to check (update mask contains `content`, memo is a comment, ticket is found).

---

## Finding 5 — MEDIUM: Comment Re-Index Churn

**Impact:** Each comment creation triggers a full ticket re-index with a new version. 5 rapid comments = 5 embedding API calls for nearly identical content. This is acknowledged as an open question in the plan but not resolved.

**Mitigation:** For the prototype, accept the churn since `reindexFileVersion` serializes per tenant (mutex) and the embedding call is batched with resume (`InsertWithCheckpoint`). Add a debounce window (e.g., 2-5 seconds) after the last comment/edit before triggering `IndexTicketContent`. This can be added post-prototype.

---

## Finding 6 — MEDIUM: No Automated Tests

The plan lists only manual curl/SQLite validation in Section 6. No unit tests for `IndexTicketContent`, `getTicketComments`, or the hook wiring.

For a prototype scoped to a single ticket, this is acceptable **only if** the validation steps are exhaustive. The validation table is reasonable but misses:

- Verify `InferResolutionForNewTicket` waits for indexing (Finding 2)
- Verify version does NOT increment when content is unchanged (Finding 1)
- Verify comment-less tickets index correctly
- Verify ticket update without content change does NOT trigger re-index

### Required Fix

Add a unit test for `IndexTicketContent` that asserts: (a) `UpsertAgentSourceFile` is called, (b) `ReindexFileVersion` is called with the returned version, (c) the content blob contains title + description + comments in the expected format. This can wait for post-prototype but must be tracked.

---

## Finding 7 — LOW: Ticket Deletion Orphaning

The plan does not address what happens when a ticket is deleted. The `agent_source_files` row and vector DB chunks for the ticket persist. For the prototype, this is acceptable (no deletions planned). Document as future work.

---

## Finding 8 — LOW-Medium: Content Injection via `internal_notes`

`InferResolutionForNewTicket` (service.go:5666-5682) writes raw `chunk.Content` directly into `ticket.InternalNotes`. If a user injects strings like `<!-- @script: malicious -->` or other annotation markers into a ticket title/description, those strings get embedded, retrieved, and re-emitted into other tickets' `internal_notes`.

For the prototype, this is low risk since `internal_notes` are likely rendered as plain text/markdown in the admin UI. But if the rendering layer sanitizes poorly, it could lead to XSS. Add a sanitization step before writing to `internal_notes` (the codebase already has `sanitizer.go`). Track as post-prototype.

---

## Finding 9 — LOW: `reindexFileVersion` Unexported Name

The plan says "Rename `reindexFileVersion` -> `ReindexFileVersion`" and update internal callers. The two internal callers are `ReindexTenantContent` (line 688) and `ReindexTenantContentWithResume` (line 1020). Verify there are no other internal references (e.g., in test files) before renaming.

---

## Required Rework Summary

| # | Finding | Must Fix Before Implementation? |
|---|----------|--------------------------------|
| 1 | Version inflation / no content-hash dedup | YES |
| 2 | Inference races ahead of indexing | YES |
| 3 | Silent failures in `getTicketComments` | YES |
| 4 | `UpdateMemo` hook has no pseudo-code | YES |
| 5 | Comment churn not resolved | NO -- track for post-prototype |
| 6 | No unit tests | NO -- acceptable for prototype |
| 7 | Ticket deletion orphaning | NO -- acceptable for prototype |
| 8 | `internal_notes` injection surface | NO -- acceptable for prototype |
| 9 | Rename coverage verification | NO -- easy spot-check |

---

## Minimum Viable Plan Changes

The plan must be updated before implementation proceeds:

1. **Step 2 (`IndexTicketContent`):** Add content-hash check before `UpsertAgentSourceFile` to prevent version bloat on no-op re-indexes. Return error from the helper on fetch failures.

2. **Step 3 + Step 4 (`ticket_service.go`):** Remove the independent `InferResolutionForNewTicket` goroutine from `CreateTicket`. Instead, have `IndexTicketContent` accept a `triggerInference bool` parameter, and after successful `ReindexFileVersion`, call `InferResolutionForNewTicket(ctx, ticket)`. This eliminates the race.

3. **Step 2 helper (`getTicketComments`):** Return `([]*store.Memo, error)`. Log on error. Do not swallow errors silently.

4. **Step 5 (`UpdateMemo`):** Add pseudo-code showing the hook location (after `s.Store.UpdateMemo(ctx, update)` at line 437, before the webhook dispatch at line 452), with guards for `path == "content"` and comment relation check.

---

## Rollback Note

The plan's Section 8 rollback is correct and sufficient: delete `agent_source_files` where `file_type='ticket'`, checkout the three files, restart. No vector DB migration or schema nullification is needed because LanceDB tables are auto-created per dimension.

---

## Recommendation

Do not proceed to implementation until the four "Must Fix" items above are addressed in the plan. The prototype can still use the same three files and same test ticket (#120, tenant 19) -- the rework is in the control flow, not in the infrastructure.
