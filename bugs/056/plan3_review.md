# Adversarial Plan Review: Plan 3 — Bug 056 Content Type Mismatch Fix (Final)

**Bug/Task:** Plan for `plan3.md`  
**Reviewer:** Kilo (Senior Go Architect)  
**Date:** 2026-08-01  
**Verdict:** REWORK REQUIRED — plan correctly fixes Bug 3 at the handler locations and adds a dual-type search test, but the new test has a **critical design bug** that will cause it to fail. Additionally, there is no test coverage for the Bug 3 admin search fix.

---

## Executive Summary

Plan 3 correctly addresses all findings from Plan 2:
- Bug 1: Dual-type search `["ticket", "ticket_section"]` in `EscalateTicket` and `InferResolutionForNewTicket`
- Bug 2: `"bug"` handler added to all three reindex functions
- Bug 3: Fixed at correct locations — `handlers.go:4977`, `handlers.go:5107`, and `service.go:5031`
- Tests updated to use `"ticket_section"`
- New dual-type test added: `TestAskRovo_DualTypeSearchFindsOldAndNewChunks`
- Convention documented in `chunker.go`

However, **the dual-type test will fail at runtime** because the ticket query text does not contain any seed keywords, causing the `controlledEmbeddingService` to return a LOW vector that produces cosine similarity 0.0 against the HIGH-vector seed chunks — below the `MinScore: 0.7` threshold.

| # | Finding | Severity | Action |
|---|---------|----------|--------|
| 1 | Dual-type test query produces LOW vector → no results → test fails | **HIGH** | Rework |
| 2 | No test for Bug 3 admin search fix | MEDIUM | Add |
| 3 | `HandleRAGSearch` line number slightly off in Bug 3 table | LOW | Fix |
| 4 | Plan does not verify Bug 2 reindex paths compile/run | LOW | Document |

---

## Finding 1: Dual-Type Test Will Fail — Query Text Lacks Keywords

**Severity:** HIGH (CRITICAL)

The new test `TestAskRovo_DualTypeSearchFindsOldAndNewChunks` creates a ticket with:

```go
Title: "Dual Type Test",
Description: "/m/" + memo.UID,
```

`InferResolutionForNewTicket` builds the query text from the ticket:

```go
queryText := fmt.Sprintf("%s %s", ticket.Title, ticket.Description)
// Result: "Dual Type Test /m/test-memo-..."
```

The test's `setupAskRovoTest` configures the embedding service with seed keywords `["rag", "indexing", "ticket"]`. The `controlledEmbeddingService` returns a **HIGH vector** only when the input text contains at least one seed keyword. The query `"Dual Type Test /m/test-memo-..."` contains **none** of these keywords, so it receives the **LOW vector** `[0, 1.0, 0, ...]`.

The seed chunks contain keywords (`"Per-ticket RAG indexing ticket..."`), so they receive **HIGH vectors** `[1.0, 0, 0, ...]`.

Cosine similarity between LOW and HIGH = **0.0**.

`InferResolutionForNewTicket` uses `MinScore: 0.7` for ticket search. Since 0.0 < 0.7, **all chunks are filtered out**. The result is empty, and the test fails on `require.NotEmpty(t, result)`.

**This is the same class of bug as the original Bug 1** — a mismatch between what the test author assumed the embedding would produce and what it actually produces.

**Required fix:** Make the ticket title or description contain at least one seed keyword. For example:

```go
ticket, err := ts.CreateTicket(ctx, &store.Ticket{
    Title:       "Ticket RAG Indexing Dual Type",  // contains "ticket", "rag", "indexing"
    Description: "/m/" + memo.UID,
    Status:      store.TicketStatusOpen,
    Priority:    store.TicketPriorityMedium,
    CreatorID:   user.ID,
    TenantID:    &tenant.ID,
})
```

With this title, the query text becomes `"Ticket RAG Indexing Dual Type /m/test-memo-..."`, which contains multiple seed keywords. The query receives a HIGH vector, cosine similarity against seed chunks is 1.0, and the test passes.

---

## Finding 2: No Test for Bug 3 Admin Search Fix

**Severity:** MEDIUM

The plan adds a dual-type test for Bug 1 (ticket search backward compatibility) but does not add any test for Bug 3 (admin search endpoints). The Bug 3 fix touches three code paths:

1. `handlers.go:4977` — `HandleTestRAGSearch`
2. `handlers.go:5107` — `HandleTenantRAGSearch`
3. `service.go:5031` — `SearchVectorDB`

None of these are covered by the existing `TestAskRovo_*` tests. If a future change breaks the dual-type pattern in any of these locations, no test will catch it.

**Required addition:** Add a test that exercises the admin search path with both old and new content types. The simplest approach is to call `SearchVectorDB` directly:

```go
func TestSearchVectorDB_DualTypeContentTypes(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    // Seed old-format chunk
    svc.vectorDB.Insert(ctx, []DocumentChunk{{
        ID: "old_kb", TenantID: tenant.ID, AudienceType: "external",
        ContentType: "kb", Title: "Old KB",
        Content: "RAG indexing for kb", IsActive: true,
    }})

    // Seed new-format chunk
    svc.vectorDB.Insert(ctx, []DocumentChunk{{
        ID: "new_kb", TenantID: tenant.ID, AudienceType: "external",
        ContentType: "kb_section", Title: "New KB",
        Content: "RAG indexing for kb section", IsActive: true,
    }})

    result, err := svc.SearchVectorDB(ctx, tenant.ID, "external", "kb", "RAG indexing", 5, nil)
    require.NoError(t, err)
    require.NotEmpty(t, result.Chunks)
    // Verify both old and new format chunks are found
    ids := make([]string, len(result.Chunks))
    for i, c := range result.Chunks {
        ids[i] = c.ID
    }
    require.Contains(t, ids, "old_kb")
    require.Contains(t, ids, "new_kb")
}
```

Alternatively, if `SearchVectorDB` is considered internal, test the handlers directly via HTTP requests.

---

## Finding 3: `HandleRAGSearch` Line Number Slightly Off

**Severity:** LOW

The Bug 3 table lists `HandleRAGSearch` at `service.go:6038`. Based on the codebase, the function definition is at `handlers.go:6036`:

```go
// handlers.go:6036
func (h *Handler) HandleRAGSearch(c echo.Context) error {
```

The actual call to `SearchVectorDB` is at `handlers.go:6082`. The plan's line 6038 appears to be inside the function body, not the definition.

**Required fix:** Update the table to reference `handlers.go:6036` for the function definition or `handlers.go:6082` for the `SearchVectorDB` call.

---

## Finding 4: No Verification That Bug 2 Reindex Paths Compile

**Severity:** LOW

The plan adds `"bug"` handlers to three functions: `ReindexAllContent`, `ReindexTenantContent`, and `ReindexTenantContentWithResume`. The code snippets look correct, but the plan does not include a compilation check or a test that exercises these paths.

If the `"bug"` key is missing from `fileMap` or if `entry.content` is empty, the new code might silently skip indexing without error. The existing `"kb"` and `"policy"` handlers have the same pattern, so this is unlikely but worth verifying.

**Recommendation:** Document as follow-up: "Add unit test for `ReindexAllContent` with bug source file to verify chunks are produced."

---

## What Is Correct

| Aspect | Status | Notes |
|--------|--------|-------|
| Bug 1 fix locations | CORRECT | `service.go:5579` and `5618` use dual-type search |
| Bug 1 backward compatibility | CORRECT | Old `"ticket"` and new `"ticket_section"` both matched |
| Bug 2 reindex pattern | CORRECT | Adding `"bug"` handler follows existing pattern |
| Bug 3 fix locations | CORRECT | `handlers.go:4977`, `handlers.go:5107`, `service.go:5031` |
| Convention documentation | CORRECT | `chunker.go` comment added |
| Migration strategy | CORRECT | Dual-type search handles backward compatibility |
| Test updates | CORRECT | Existing tests use `"ticket_section"` consistently |
| Follow-up items | CORRECT | Embedder tests, observation standardization, admin UI |

---

## Behavioral Correctness Check

### Bug 1: Dual-Type Search
| Step | Expected | Actual |
|------|----------|--------|
| Search with `ContentTypes: ["ticket", "ticket_section"]` | Matches both old and new chunks | CORRECT — MemoryVectorDB and LanceDB both match either type |
| Old embedder chunks (`"ticket"`) | Found | CORRECT |
| New reindex chunks (`"ticket_section"`) | Found | CORRECT |
| Main chat path (`RetrieveContextForQuery`) | Unaffected | CORRECT — uses empty ContentTypes |

### Bug 2: Bug Reindex
| Step | Expected | Actual |
|------|----------|--------|
| `ReindexAllContent` processes `"bug"` file | Chunks with `ContentType: "bug_section"` inserted | CORRECT — follows `"kb"`/`"policy"` pattern |
| `InferResolutionForNewTicket` Search 2 | Finds `"bug_section"` chunks | CORRECT — already searches for `"bug_section"` |

### Bug 3: Admin Search
| Step | Expected | Actual |
|------|----------|--------|
| `HandleTestRAGSearch` with `FileType="kb"` | Finds both `"kb"` and `"kb_section"` chunks | CORRECT — dual-type pattern applied |
| `HandleTenantRAGSearch` with `FileType="policy"` | Finds both `"policy"` and `"policy_section"` chunks | CORRECT — dual-type pattern applied |
| `HandleRAGSearch` with `FileType="kb"` | Finds both `"kb"` and `"kb_section"` chunks | CORRECT — `SearchVectorDB` fixed |

---

## Recommended Rework

### 1. Fix Dual-Type Test Query (Finding 1)

Change the ticket title to contain at least one seed keyword:

```go
ticket, err := ts.CreateTicket(ctx, &store.Ticket{
    Title:       "Ticket RAG Indexing Dual Type",  // contains "ticket", "rag", "indexing"
    Description: "/m/" + memo.UID,
    Status:      store.TicketStatusOpen,
    Priority:    store.TicketPriorityMedium,
    CreatorID:   user.ID,
    TenantID:    &tenant.ID,
})
```

This ensures the query text receives a HIGH vector from `controlledEmbeddingService`, producing cosine similarity 1.0 against the seed chunks.

### 2. Add Bug 3 Admin Search Test (Finding 2)

Add `TestSearchVectorDB_DualTypeContentTypes` as described in Finding 2, or test the admin handlers directly via HTTP.

### 3. Fix Line Number (Finding 3)

Update Bug 3 table to reference `handlers.go:6036` or `handlers.go:6082` instead of `service.go:6038`.

### 4. Document Bug 2 Verification (Finding 4)

Add to follow-up: "Add unit test for `ReindexAllContent` with bug source file to verify chunks are produced."

---

## Final Verdict

**REWORK REQUIRED.** Plan 3 correctly fixes all code locations from Plan 2's findings. The Bug 1, Bug 2, and Bug 3 fixes are at the correct locations and use the correct dual-type pattern.

However, **the new dual-type test has a critical design bug**: the ticket title `"Dual Type Test"` does not contain any seed keywords (`"rag"`, `"indexing"`, `"ticket"`), so the query receives a LOW vector from `controlledEmbeddingService`. The seed chunks receive HIGH vectors. Cosine similarity between LOW and HIGH is 0.0, which is below `MinScore: 0.7`. The test will fail with an empty result.

Additionally, **no test covers the Bug 3 admin search fix**. The dual-type pattern is applied to three handler locations, but only the ticket search path has test coverage.

After fixing the dual-type test query and adding Bug 3 test coverage, the plan will be implementation-ready.
