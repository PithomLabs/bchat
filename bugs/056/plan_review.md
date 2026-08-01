# Adversarial Plan Review: Bug 056 — Content Type Mismatch in RAG Pipeline

**Bug/Task:** Plan for `plan.md`  
**Reviewer:** Kilo (Senior Go Architect)  
**Date:** 2026-08-01  
**Verdict:** REWORK REQUIRED — plan correctly identifies two real bugs but proposes an incomplete fix for Bug 1 that introduces a backward-compatibility regression and would break existing tests. The root cause (inconsistent content type convention) is not addressed.

---

## Executive Summary

The plan correctly identifies two content-type mismatches in the RAG pipeline:

1. **Bug 1 (Ticket search):** `InferResolutionForNewTicket` and `EscalateTicket` search for `"ticket"`, but chunks indexed via `ReindexFileVersion` → `ChunkMarkdownContent` are stored as `"ticket_section"`. This is real and causes inference to return empty for on-create indexed tickets.

2. **Bug 2 (Bug reindex):** The three reindex functions (`ReindexAllContent`, `ReindexTenantContent`, `ReindexTenantContentWithResume`) only handle `"kb"` and `"policy"`, never processing `"bug"` source files. This is real and makes bug history search dead code.

However, the proposed fix for Bug 1 is **incomplete and introduces a regression**: changing the search filter from `"ticket"` to `"ticket_section"` would fix the reindex path but **break the background embedder path** (`ticket_embedder.go`), which stores `ContentType: "ticket"` directly. Additionally, the existing test file `ticket_rag_inference_test.go` uses `ContentType: "ticket"` in its seed data, so the tests would fail after the fix.

The plan does not address the **root cause**: `ChunkMarkdownContent` appends `"_section"` to all file types, but `ticket_embedder.go` stores raw file types without the suffix. This inconsistency will continue to cause bugs.

| # | Finding | Severity | Action |
|---|---------|----------|--------|
| 1 | Bug 1 fix breaks background embedder path (`ticket_embedder.go`) | **HIGH** | Rework |
| 2 | Existing tests use `ContentType: "ticket"` and would fail after fix | **HIGH** | Rework |
| 3 | Root cause (inconsistent `_section` convention) not addressed | HIGH | Rework |
| 4 | Bug 3 (admin search) deferred without a universal fix | MEDIUM | Consider |
| 5 | No migration strategy for existing LanceDB data | MEDIUM | Add |
| 6 | Plan's risk assessment contains incorrect mitigation | MEDIUM | Fix |
| 7 | `buildFilter` function name incorrect in Bug 3 description | LOW | Fix |
| 8 | No test coverage for `ticket_embedder.go` path | LOW | Document |

---

## Finding 1: Bug 1 Fix Breaks Background Embedder Path

**Severity:** HIGH

The plan proposes changing `InferResolutionForNewTicket` and `EscalateTicket` to search for `"ticket_section"`:

```go
// service.go:5579 (EscalateTicket)
- ContentTypes: []string{"ticket"},
+ ContentTypes: []string{"ticket_section"},

// service.go:5618 (InferResolutionForNewTicket, Search 1)
- ContentTypes: []string{"ticket"},
+ ContentTypes: []string{"ticket_section"},
```

However, `ticket_embedder.go` stores chunks with `ContentType: "ticket"` directly (line 106):

```go
chunks[i] = DocumentChunk{
    ID:          fmt.Sprintf("ticket_%d", ticket.ID),
    TenantID:    tenantID,
    ContentType: "ticket",  // <-- raw file type, no "_section" suffix
    Title:       ticket.Title,
    Content:     content,
    IsActive:    true,
    IndexedAt:   time.Now(),
}
```

After the fix, `InferResolutionForNewTicket` would search for `"ticket_section"` and **miss all chunks created by the background embedder**. This is a regression.

**Impact:** The background embedder runs periodically (every 5 minutes via `TICKET_EMBEDDING_ENABLED`). Any tickets it embeds would become invisible to inference and escalation.

**Required rework:** The fix must handle BOTH content types. Options:

**Option A:** Change `ticket_embedder.go` to store `"ticket_section"` for consistency:
```go
// ticket_embedder.go:106
- ContentType: "ticket",
+ ContentType: "ticket_section",
```
This standardizes the convention but requires reindexing existing embedded tickets.

**Option B:** Make the search accept both forms:
```go
// service.go:5618
ContentTypes: []string{"ticket", "ticket_section"},
```
This is backward compatible but doesn't fix the underlying inconsistency.

**Option C:** Change `ChunkMarkdownContent` to NOT append `"_section"` for the `"ticket"` file type:
```go
// chunker.go:389
- ContentType: fileType + "_section",
+ ContentType: func() string { if fileType == "ticket" { return "ticket" }; return fileType + "_section" }(),
```
This matches the embedder but is inconsistent with `"kb_section"` and `"policy_section"`.

**Recommendation:** Option A is the cleanest long-term fix, but requires a migration. For immediate correctness, use Option B and schedule Option A as follow-up.

---

## Finding 2: Existing Tests Would Fail After Fix

**Severity:** HIGH

The test file `ticket_rag_inference_test.go` seeds chunks with `ContentType: "ticket"`:

```go
// ticket_rag_inference_test.go:147-148
svc.vectorDB.Insert(ctx, []DocumentChunk{
    {ID: "seed_ticket_1", TenantID: tenant.ID, AudienceType: "internal",
     ContentType: "ticket", Title: "Per-Ticket RAG Indexing",
```

After changing `InferResolutionForNewTicket` to search for `"ticket_section"`, these tests would fail because:
1. Test data uses `ContentType: "ticket"`
2. Search looks for `ContentType: "ticket_section"`
3. No match → tests fail

The plan claims in Step 3: "Tests should continue to pass (they use MemoryVectorDB with controlledEmbeddingService, not LanceDB)." This is **incorrect** — the tests use `ContentTypes` filtering, which is implemented in `MemoryVectorDB.Search` identically to LanceDB.

**Required fix:** Update all test seed data to use `ContentType: "ticket_section"`:

```go
ContentType: "ticket_section",
```

---

## Finding 3: Root Cause (Inconsistent Convention) Not Addressed

**Severity:** HIGH

The plan fixes the symptom (search query mismatch) but not the root cause. The `"_section"` suffix is applied inconsistently:

| Producer | File Type | Stored ContentType |
|----------|-----------|-------------------|
| `ChunkMarkdownContent` | `"kb"` | `"kb_section"` |
| `ChunkMarkdownContent` | `"policy"` | `"policy_section"` |
| `ChunkMarkdownContent` | `"ticket"` | `"ticket_section"` |
| `ticket_embedder.go` | `"ticket"` | `"ticket"` |
| `observation_indexer.go` | `"observation"` | `"observation"` |

The background embedder and observation indexer store raw file types. Only `ChunkMarkdownContent` appends `"_section"`. This inconsistency is the root cause of Bug 1 and will cause future mismatches.

**Required fix:** Standardize the convention. Either:
1. All producers append `"_section"` (consistent with ChunkMarkdownContent)
2. No producer appends `"_section"` (consistent with ticket_embedder and observation_indexer)
3. Document the convention and enforce it in code

**Recommendation:** Standardize on raw file types WITHOUT `"_section"` suffix, since:
- `ticket_embedder.go` and `observation_indexer.go` already use raw types
- The main chat path (`RetrieveContextForQuery`) already ignores content types entirely
- The `"_section"` suffix adds no semantic value — `ContentType` already implies it's a chunk/section

If standardization is out of scope for this bug, at minimum update `ticket_embedder.go` to use `"ticket_section"` to match the reindex path.

---

## Finding 4: Bug 3 Deferred Without Universal Fix

**Severity:** MEDIUM

The plan defers Bug 3 (admin RAG search mismatch) to future work. This is reasonable for a focused bug fix, but the plan's proposed fixes for Bug 1 and Bug 3 are the **same pattern**: search query doesn't match stored content type.

If the team later fixes Bug 3 using Option 1 from the plan:
```go
// service.go:5031
- queryObj.ContentTypes = []string{fileType}
+ queryObj.ContentTypes = []string{fileType + "_section"}
```

This would ALSO break the background embedder path for admin searches. The fix should be holistic.

**Recommendation:** Either:
1. Fix Bug 3 now with the same solution as Bug 1, OR
2. Document that Bug 3 fix must use the same approach as Bug 1 to avoid inconsistency

---

## Finding 5: No Migration Strategy for Existing LanceDB Data

**Severity:** MEDIUM

If the team standardizes on `"ticket_section"` (Option A from Finding 1), existing LanceDB data from `ticket_embedder.go` has `ContentType: "ticket"`. These chunks would become invisible to inference until reindexed.

The plan mentions manual verification for tenant 19 but doesn't address:
1. How to identify tenants with old-format chunks
2. How to migrate existing data (reindex? delete + reindex?)
3. Whether the background embedder should be updated to use the new format

**Required addition:** Add a migration step:
```go
// After fix: reindex all tenants to convert old "ticket" chunks to "ticket_section"
// Or: run ticket_embedder.go with updated ContentType to overwrite
```

---

## Finding 6: Risk Assessment Contains Incorrect Mitigation

**Severity:** MEDIUM

The plan's risk assessment states:
> | Changing search from "ticket" to "ticket_section" misses old "ticket" chunks | Medium | Old chunks from ticket_embedder.go are still found via empty ContentTypes filter in RetrieveContextForQuery |

This is **incorrect**:
1. `RetrieveContextForQuery` is the main chat RAG path, not the inference path
2. The bug is in `InferResolutionForNewTicket` and `EscalateTicket`, not `RetrieveContextForQuery`
3. Changing the search filter in inference/ escalation DOES miss old `"ticket"` chunks from the embedder

The mitigation should be: "Old chunks from ticket_embedder.go will be missed by inference after the fix. Update ticket_embedder.go to store `ticket_section` or make search accept both types."

---

## Finding 7: `buildFilter` Function Name Incorrect

**Severity:** LOW

The plan references `buildFilter` in `vectordb_lance.go:1179` for Bug 3 Option 2. The actual function is `buildFilter` at line 1167, not line 1179. Line 1179 is inside the function body.

**Required fix:** Update reference to `buildFilter` (line 1167), not line 1179.

---

## Finding 8: No Test Coverage for `ticket_embedder.go` Path

**Severity:** LOW

There are no tests for `ticket_embedder.go` (`processPendingTickets`, `embedTenantTickets`, `buildTicketClusters`). The plan doesn't mention adding any.

**Impact:** After fixing Bug 1, the embedder path would be broken (if we don't also update the embedder). Without tests, this regression might not be caught.

**Recommendation:** Document as follow-up: "Add tests for ticket_embedder.go to verify ContentType consistency with inference path."

---

## Recommended Rework

### 1. Fix Bug 1 with Backward Compatibility (Finding 1)

**Recommended approach:** Use Option B (search accepts both types) + Option A (update embedder to use new format).

```go
// service.go:5579 (EscalateTicket)
ContentTypes: []string{"ticket", "ticket_section"},

// service.go:5618 (InferResolutionForNewTicket, Search 1)
ContentTypes: []string{"ticket", "ticket_section"},
```

Then update `ticket_embedder.go`:
```go
// ticket_embedder.go:106
- ContentType: "ticket",
+ ContentType: "ticket_section",
```

This:
- Fixes the immediate bug (reindex path works)
- Maintains backward compatibility (old embedder chunks still found)
- Standardizes the convention going forward

### 2. Update Tests (Finding 2)

Change all `ContentType: "ticket"` in `ticket_rag_inference_test.go` to `ContentType: "ticket_section"`.

### 3. Address Root Cause (Finding 3)

Add a content type convention document or code comment:
```go
// CONTENT TYPE CONVENTION:
// ChunkMarkdownContent appends "_section" to fileType to produce ContentType.
// Example: fileType="kb" → ContentType="kb_section"
// All producers MUST follow this convention.
```

Then update `ticket_embedder.go` to follow the convention.

### 4. Fix Risk Assessment (Finding 6)

Update the mitigation for backward compatibility to accurately describe the embedder path.

### 5. Add Migration Note (Finding 5)

Document that existing LanceDB chunks with `ContentType="ticket"` from the embedder will continue to work with the dual-type search, but should be reindexed to `"ticket_section"` during next maintenance window.

---

## What Is Correct

| Aspect | Status | Notes |
|--------|--------|-------|
| Bug 1 identification | CORRECT | Search for `"ticket"` doesn't match stored `"ticket_section"` |
| Bug 2 identification | CORRECT | Reindex functions skip `"bug"` file type |
| Bug 3 identification | CORRECT | Admin search has same mismatch for `"kb"` and `"policy"` |
| ContentType audit table | CORRECT | Accurate mapping of stored vs searched types |
| Bug 2 fix approach | CORRECT | Adding `"bug"` handler to reindex functions is correct |
| Bug 3 deferral | CORRECT | Admin debug tool is lower priority |

---

## Final Verdict

**REWORK REQUIRED.** The plan correctly identifies two real bugs but proposes an incomplete fix for Bug 1 that introduces a backward-compatibility regression. Three high-severity findings require correction:

1. **Bug 1 fix breaks background embedder path** — `ticket_embedder.go` stores `ContentType: "ticket"` directly, but the plan changes search to `"ticket_section"`. After the fix, chunks created by the background embedder would be invisible to inference and escalation.
2. **Existing tests would fail** — `ticket_rag_inference_test.go` seeds chunks with `ContentType: "ticket"`. Changing the search to `"ticket_section"` would cause these tests to fail.
3. **Root cause not addressed** — `ChunkMarkdownContent` appends `"_section"` to all file types, but `ticket_embedder.go` and `observation_indexer.go` store raw file types. This inconsistency is the root cause and will continue to cause bugs.

**Recommended fix for Bug 1:** Make the search accept both types (`ContentTypes: []string{"ticket", "ticket_section"}`) AND update `ticket_embedder.go` to store `"ticket_section"` for consistency. Update test seed data to use `"ticket_section"`.

After addressing these findings, the plan will be implementation-ready.
