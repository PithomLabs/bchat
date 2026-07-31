# Adversarial Review: bugs/052 Per-Ticket RAG Indexing Prototype (Revised)

**Reviewer:** Kilo (Senior Go Architect)
**Plan File:** `/home/chaschel/Documents/go/bchat/bugs/052/plan2.md`
**Date:** 2026-07-31
**Verdict:** **CONDITIONAL APPROVAL -- 2 Rework Items, 4 Nits**

---

## Executive Summary

The revised plan successfully addresses all 4 must-fix findings from the first review:

1. **Content-hash dedup** is added to `IndexTicketContent` -- prevents version bloat on no-op reindexes.
2. **Inference chaining** replaces the parallel goroutines -- `InferResolutionForNewTicket` is called inside `IndexTicketContent` after `ReindexFileVersion` succeeds.
3. **`getTicketComments` returns error** -- silent failures are eliminated.
4. **`UpdateMemo` hook is fully specified** -- pseudo-code with guard conditions is present.

However, adversarial review surfaces **2 rework items** (correctness/security) and **4 nits** that must be resolved before implementation.

Approval is withheld until the rework items are patched in the plan.

---

## Finding 1 -- HIGH: TOCTOU Race in Content-Hash Dedup

**Plan code (Step 2):**
```go
existing, _ := s.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{
    TenantID:   &tenantID,
    FileType:   strPtr("ticket"),
    LatestOnly: true,
})
if existing != nil && existing.ContentHash == contentHash {
    // reindex existing version
    ...
    return chunks, nil
}

// Content changed -- upsert creates new version row
sourceFile, err := s.store.UpsertAgentSourceFile(...)
```

**Problem:** The `GetAgentSourceFile` read and the `UpsertAgentSourceFile` write are not atomic. Two concurrent goroutines for the same ticket can both observe `existing == nil` (or an older version), then both call `UpsertAgentSourceFile`, creating two version rows. The per-tenant `reindexFileVersion` mutex serializes the vector-DB insert, but the SQLite upsert happens **outside** the mutex.

**Trigger scenario:**
1. Comment A and Comment B are created simultaneously on Ticket X.
2. Both goroutines compute the same `contentHash`.
3. Both call `GetAgentSourceFile` -> both get `nil` (no ticket source file yet).
4. Both call `UpsertAgentSourceFile` -> SQLite rows version=1 and version=2 are created.
5. Both acquire the mutex, call `ReindexFileVersion` on their respective versions.
6. Vector DB active version ends up at v2, but v1 is orphaned.

**Impact:** For a prototype, the orphaned version is tolerable. But the plan's dedup guarantee is violated.

### Required Fix

Wrap the `GetAgentSourceFile` + `UpsertAgentSourceFile` sequence in a per-ticket critical section. Two options:

**Option A (recommended):** Use a per-ticket mutex in `IndexTicketContent`.

**Option B (simpler):** Move `UpsertAgentSourceFile` inside the existing `reindexFileVersion` per-tenant mutex.

For the prototype, **Option A is the minimum viable fix**. Document the race and its low probability if Option A is rejected.

---

## Finding 2 -- HIGH: Missing Tenant ID in Find Structs (Security)

**Plan code (Step 6 -- CreateMemoComment hook):**
```go
descriptionLink := "/m/" + relatedMemo.UID
tickets, _ := s.Store.ListTickets(ctx, &store.FindTicket{Description: &descriptionLink})
```

**Plan code (Step 7 -- UpdateMemo hook):**
```go
parentMemo, _ := s.Store.GetMemo(ctx, &store.FindMemo{ID: &parentMemoID})
...
tickets, _ := s.Store.ListTickets(ctx, &store.FindTicket{Description: &descriptionLink})
```

**Problem:** Neither `ListTickets` nor `GetMemo` receives an explicit `TenantID` in the find struct. These store methods only filter by tenant ID when the find struct's `TenantID` field is set. They do **not** read tenant ID from the context.

Per the project's tenant-isolation rules:
> Always extract tenant ID from context in handlers
> Apply tenant filter before database queries

The plan's pseudo-code violates this pattern. If a malicious user crafts a memo UID that matches a ticket in another tenant, they could trigger cross-tenant indexing.

**Concrete example:** Tenant A has a ticket with description `/m/abc123`. Tenant B also has a memo with UID `abc123`. If a Tenant B user edits a comment that links to `/m/abc123`, the `ListTickets` query without `TenantID` could return Tenant A's ticket.

### Required Fix

Pass `TenantID` explicitly in every find struct used in the new hooks:

```go
// Step 6:
tickets, _ := s.Store.ListTickets(ctx, &store.FindTicket{
    Description: &descriptionLink,
    TenantID:    tenantID, // extract from ctx
})

// Step 7:
parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{
    ID:       &parentMemoID,
    TenantID: tenantID, // extract from ctx
})
...
tickets, _ := s.Store.ListTickets(ctx, &store.FindTicket{
    Description: &descriptionLink,
    TenantID:    tenantID, // extract from ctx
})
```

Where `tenantID` is obtained via `GetTenantIDFromContext(ctx)` (gRPC) or `getTenantFromContext(c)` (HTTP).

---

## Finding 3 -- HIGH: UpdateMemo Hook Uses Stale memo.ID

**Plan code (Step 7):**
```go
if err = s.Store.UpdateMemo(ctx, update); err != nil {
    return nil, status.Errorf(codes.Internal, "failed to update memo")
}

// Re-index parent ticket if this memo is a comment and content changed
if s.agentHandler != nil && update.Content != nil {
    commentType := store.MemoRelationComment
    parentRelations, relErr := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
        MemoID: &memo.ID,  // <-- memo was fetched BEFORE the update
        Type:   &commentType,
    })
```

**Problem:** `memo` is fetched at line 333 (`memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})`). The `UpdateMemo` call at line 437 mutates the row but does not update the local `memo` variable. For ID-based lookups, this is fine -- `memo.ID` is immutable. But `memo.Content` and other fields are stale.

In this specific hook, we only use `memo.ID` for the `ListMemoRelations` query. This is safe because `MemoID` in the relation table is a foreign key to the memo's primary key, which never changes. **Verdict: safe in practice, but fragile.** If the hook ever needs `memo.Content` or other fields, the stale object would cause bugs.

### Nit

Add a comment documenting that `memo.ID` is used intentionally because it is immutable, and do not extend this hook to use other `memo` fields without re-fetching.

---

## Finding 4 -- MEDIUM: Multiple COMMENT Relations Ambiguity

**Plan code (Step 7):**
```go
parentRelations, relErr := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
    MemoID: &memo.ID,
    Type:   &commentType,
})
if relErr == nil && len(parentRelations) > 0 {
    parentMemoID := parentRelations[0].RelatedMemoID
    parentMemo, _ := s.Store.GetMemo(ctx, &store.FindMemo{ID: &parentMemoID})
    // ...
}
```

**Problem:** `ListMemoRelations` with `MemoID` and `Type=COMMENT` returns all relations where this memo is a comment. The plan uses `parentRelations[0]` arbitrarily. In the current data model, a memo should have at most one COMMENT relation (a comment replies to one parent). But the storage layer does not enforce this uniqueness constraint.

If a data corruption or bug creates two COMMENT relations for the same memo, the plan re-indexes only the first parent's ticket, silently ignoring the second.

### Nit

Add a guard: if `len(parentRelations) > 1`, log an error and index all parent tickets. Or, since this is a prototype, document the assumption and skip.

---

## Finding 5 -- MEDIUM: CreateTicket Deduplication Path Not Hooked

**Code evidence:** `ticket_service.go:148` has a deduplication path:
```go
ticket, err = s.Store.UpdateTicket(ctx, update)
// ...
return c.JSON(http.StatusOK, convertTicketFromStore(ticket))  // early return
```

**Problem:** The plan's Step 4 only hooks into the `s.Store.CreateTicket` path (line 158). If a user creates a ticket that matches an existing one, the deduplication path runs `UpdateTicket` and returns early. No indexing occurs for the "created" ticket.

**Impact:** Low for prototype -- deduplication is rare. But it means the first-time indexing is incomplete for deduplicated tickets.

### Nit

Either:
1. Hook into the deduplication path too (add indexing after line 153).
2. Document that deduplicated tickets are not indexed and require a manual reindex.

---

## Finding 6 -- MEDIUM: Validation Timing Not Documented

**Plan code (Step 8):**
```bash
# 5. Test inference: create a NEW ticket (not #120)
curl -X POST http://localhost:5230/api/v1/tickets ...

# 6. Check internal_notes has suggestions
sqlite3 build/data/memos_dev.db \
  "SELECT substr(internal_notes, 1, 500) FROM tickets ORDER BY id DESC LIMIT 1"
```

**Problem:** `IndexTicketContent` runs in a goroutine. The HTTP response returns before indexing completes. The validator will likely query `internal_notes` before `InferResolutionForNewTicket` writes to it, seeing empty or stale notes.

The plan should specify a wait/retry mechanism for validation.

### Required Fix

Add to Step 8:
```bash
# Wait for indexing to complete (up to 10s)
for i in $(seq 1 10); do
  NOTES=$(sqlite3 build/data/memos_dev.db \
    "SELECT substr(internal_notes, 1, 50) FROM tickets ORDER BY id DESC LIMIT 1")
  if [[ "$NOTES" == *"Suggested Resolution"* ]]; then
    echo "Inference complete"
    break
  fi
  sleep 1
done
```

---

## Finding 7 -- LOW: GetAgentSourceFile Error Swallowed

**Plan code (Step 2):**
```go
existing, _ := s.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{...})
```

**Problem:** If `GetAgentSourceFile` fails (DB error), the plan ignores it and proceeds to `UpsertAgentSourceFile`, creating a new version unnecessarily. The error should at minimum be logged.

### Nit

Change to:
```go
existing, err := s.store.GetAgentSourceFile(ctx, ...)
if err != nil {
    slog.Warn("failed to check existing ticket source file, will upsert new version", "error", err)
}
```

---

## Finding 8 -- LOW: Inference Threshold May Block Validation

`InferResolutionForNewTicket` uses `MinScore: 0.7` for ticket similarity search. If the new ticket created in validation is not semantically similar enough to Ticket #120, the search returns empty and `internal_notes` stays empty. The validation step will fail.

### Nit

For the prototype validation, either:
1. Create a new ticket with deliberately similar wording to #120.
2. Temporarily lower `MinScore` to 0.3 for validation and restore it after.

---

## Rework Summary

| # | Finding | Severity | Rework Required? |
|---|---------|----------|----------|
| 1 | TOCTOU race in content-hash dedup | HIGH | YES |
| 2 | Missing tenant ID in find structs | HIGH | YES |
| 3 | Stale `memo.ID` after UpdateMemo | HIGH | NO -- nit only |
| 4 | Multiple COMMENT relations ambiguity | MEDIUM | NO -- nit only |
| 5 | CreateTicket dedup path not hooked | MEDIUM | NO -- nit only |
| 6 | Validation timing not documented | MEDIUM | YES |
| 7 | `GetAgentSourceFile` error swallowed | LOW | NO -- nit only |
| 8 | Inference threshold may block validation | LOW | NO -- nit only |

---

## Minimum Plan Changes

1. **Step 2 (`IndexTicketContent`):** Add per-ticket mutex or document the TOCTOU race. Swallow `GetAgentSourceFile` error with a warning log.

2. **Step 6 (`CreateMemoComment` hook):** Extract `tenantID` from context and pass it in `FindTicket{Description, TenantID}`.

3. **Step 7 (`UpdateMemo` hook):** Extract `tenantID` from context and pass it in `FindMemo{ID, TenantID}` and `FindTicket{Description, TenantID}`. Document that `memo.ID` is immutable and safe to use.

4. **Step 8 (Validation):** Add a wait/retry loop after ticket creation to allow indexing to complete before checking `internal_notes`.

---

## Rollback Note

The plan's Section 8 rollback remains correct and sufficient.

---

## Recommendation

The plan is close to ready. The two HIGH-severity rework items (TOCTOU race, missing tenant ID) are security/correctness issues that must be fixed. The MEDIUM validation-timing fix is required for the prototype to pass its own validation steps. The remaining 4 nits are acceptable as documented tradeoffs for a prototype.

Proceed to implementation only after the rework items are patched in `plan2.md`.
