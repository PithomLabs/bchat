# Adversarial Plan Review: Plan 2 — Bug 056 Content Type Mismatch Fix (Revised)

**Bug/Task:** Plan for `plan2.md`  
**Reviewer:** Kilo (Senior Go Architect)  
**Date:** 2026-08-01  
**Verdict:** REWORK REQUIRED — plan correctly addresses Findings 1–3 from the original review (backward compatibility, test updates, root cause convention), but introduces a new critical error: **Bug 3 fix is at the wrong location and does not actually fix the reported admin endpoints.** Additionally, the plan lacks a test for the dual-type search behavior that is the core of the backward-compatibility claim.

---

## Executive Summary

Plan 2 makes significant improvements over Plan 1:
- **Bug 1 fix is backward compatible** — dual-type search `["ticket", "ticket_section"]` correctly handles old embedder chunks and new reindex chunks
- **Tests are updated** — test seed data uses `"ticket_section"` consistently
- **Root cause is documented** — `_section` convention is documented in `chunker.go`
- **Embedder is updated** — `ticket_embedder.go` stores `"ticket_section"` going forward

However, **Bug 3 is not actually fixed by the proposed changes**:
- `HandleTestRAGSearch` (line 4977) and `HandleTenantRAGSearch` (line 5107) construct `SearchQuery` directly and call `vectorDB.Search` directly
- Neither calls `SearchVectorDB` (line 5031), which is the only location the plan modifies
- The Bug 3 fix therefore has **zero effect** on the reported broken endpoints

| # | Finding | Severity | Action |
|---|---------|----------|--------|
| 1 | Bug 3 fix at wrong location — doesn't fix reported endpoints | **HIGH** | Rework |
| 2 | No test for dual-type search behavior | MEDIUM | Add |
| 3 | `HandleRAGSearch` also affected but not mentioned | MEDIUM | Fix |
| 4 | Plan's Bug 3 description says "admin search" but fix is in `SearchVectorDB` | MEDIUM | Fix |
| 5 | Dual-type pattern introduces SQL `IN` clause — verify no injection risk | LOW | Verify |
| 6 | `observation_indexer.go` convention mismatch acknowledged but not scheduled | LOW | Document |

---

## Finding 1: Bug 3 Fix Is at the Wrong Location

**Severity:** HIGH

The plan proposes fixing Bug 3 at `service.go:5031` (`SearchVectorDB`):

```go
// service.go:5031 (SearchVectorDB)
- queryObj.ContentTypes = []string{fileType}
+ queryObj.ContentTypes = []string{fileType, fileType + "_section"}
```

However, the **reported broken endpoints** (`HandleTestRAGSearch` and `HandleTenantRAGSearch`) do **not** call `SearchVectorDB`. They construct `SearchQuery` directly:

```go
// handlers.go:4977 (HandleTestRAGSearch)
if req.FileType != "" {
    searchQuery.ContentTypes = []string{req.FileType}  // <-- NOT fixed by plan
}

// handlers.go:5107 (HandleTenantRAGSearch)
if req.FileType != "" {
    searchQuery.ContentTypes = []string{req.FileType}  // <-- NOT fixed by plan
}
```

Both handlers then call `h.service.vectorDB.Search(ctx, searchQuery)` directly, bypassing `SearchVectorDB` entirely.

**Impact:** After applying the plan's Bug 3 fix:
- `HandleRAGSearch` (which uses `SearchVectorDB`) would be fixed
- `HandleTestRAGSearch` would **still** search for `"kb"` only and miss `"kb_section"` chunks
- `HandleTenantRAGSearch` would **still** search for `"policy"` only and miss `"policy_section"` chunks

The reported bug is **not fixed**.

**Required rework:** Fix the actual affected locations in `handlers.go`:

```go
// handlers.go:4977 (HandleTestRAGSearch)
- searchQuery.ContentTypes = []string{req.FileType}
+ searchQuery.ContentTypes = []string{req.FileType, req.FileType + "_section"}

// handlers.go:5107 (HandleTenantRAGSearch)
- searchQuery.ContentTypes = []string{req.FileType}
+ searchQuery.ContentTypes = []string{req.FileType, req.FileType + "_section"}
```

The `SearchVectorDB` fix (line 5031) is still valuable for `HandleRAGSearch` and any future callers, but it does not address the reported Bug 3 endpoints.

---

## Finding 2: No Test for Dual-Type Search Behavior

**Severity:** MEDIUM

The core innovation of the Bug 1 fix is backward compatibility: the search must find BOTH old chunks (`ContentType: "ticket"`) and new chunks (`ContentType: "ticket_section"`). The plan updates test data to use `"ticket_section"` but does not add a test that verifies the dual-type search works.

Without such a test, a future developer could "simplify" the search back to `[]string{"ticket_section"}` and break compatibility with old embedder chunks, and no test would catch it.

**Required addition:** Add a test that seeds both content types and asserts both are found:

```go
func TestAskRovo_DualTypeSearchFindsOldAndNewChunks(t *testing.T) {
    ctx, ts, svc, tenant, user := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    // Seed old-format chunk (from ticket_embedder.go before fix)
    svc.vectorDB.Insert(ctx, []DocumentChunk{{
        ID: "old_ticket", TenantID: tenant.ID, AudienceType: "internal",
        ContentType: "ticket", Title: "Old Format Ticket",
        Content: "Per-ticket RAG indexing ticket", IsActive: true,
    }})

    // Seed new-format chunk (from ChunkMarkdownContent after fix)
    svc.vectorDB.Insert(ctx, []DocumentChunk{{
        ID: "new_ticket", TenantID: tenant.ID, AudienceType: "internal",
        ContentType: "ticket_section", Title: "New Format Ticket",
        Content: "Per-ticket RAG indexing ticket section", IsActive: true,
    }})

    memo := createTestMemo(t, ctx, ts, user, tenant, "Test")
    ticket, err := ts.CreateTicket(ctx, &store.Ticket{
        Title: "Dual Type Test", Description: "/m/" + memo.UID,
        Status: store.TicketStatusOpen, Priority: store.TicketPriorityMedium,
        CreatorID: user.ID, TenantID: &tenant.ID,
    })
    require.NoError(t, err)

    result := svc.InferResolutionForNewTicket(ctx, ticket)
    require.NotEmpty(t, result)
    require.Contains(t, result, "Old Format Ticket")
    require.Contains(t, result, "New Format Ticket")
}
```

---

## Finding 3: `HandleRAGSearch` Also Affected

**Severity:** MEDIUM

`HandleRAGSearch` (line 6038, POST `/api/v1/agent/:slug/rag/search`) calls `SearchVectorDB` at line 6082:

```go
searchResult, err := h.service.SearchVectorDB(ctx, tenant.ID, req.AudienceType, req.FileType, req.Query, req.TopK, req.SourceVersion)
```

If `req.FileType` is `"kb"`, `SearchVectorDB` currently sets `ContentTypes: []string{"kb"}` and misses `"kb_section"` chunks. The plan's fix to `SearchVectorDB` would fix this handler.

However, the original Bug 3 report only mentioned `HandleTestRAGSearch` and `HandleTenantRAGSearch`. The plan should either:
1. Expand Bug 3 scope to include `HandleRAGSearch`, OR
2. Note that `HandleRAGSearch` is also affected and will be fixed by the `SearchVectorDB` change

**Recommendation:** Expand Bug 3 description to include `HandleRAGSearch` as a third affected endpoint. This makes the fix at `SearchVectorDB` meaningful.

---

## Finding 4: Plan's Bug 3 Description Mismatches Fix Location

**Severity:** MEDIUM

The plan's Bug 3 section says:
> **Impact:** Admin debug endpoints (`HandleTestRAGSearch`, `HandleTenantRAGSearch`) search for raw file types...

But the fix is at `SearchVectorDB` (service.go:5031), which is **not used by those endpoints**. This is a documentation/implementation mismatch.

**Required fix:** Either:
1. Update Bug 3 description to mention `HandleRAGSearch` as the endpoint fixed by `SearchVectorDB`, OR
2. Move the fix to the actual affected handlers (`handlers.go:4977` and `handlers.go:5107`)

**Recommendation:** Do both:
- Fix `SearchVectorDB` for `HandleRAGSearch`
- Also fix `HandleTestRAGSearch` and `HandleTenantRAGSearch` directly

---

## Finding 5: Dual-Type SQL Injection Risk

**Severity:** LOW (theoretical)

LanceDB's `buildFilter` generates:
```go
filterParts = append(filterParts, fmt.Sprintf("content_type IN (%s)", strings.Join(types, ", ")))
```

Where `types` is built as:
```go
types[i] = fmt.Sprintf("'%s'", ct)
```

If `ct` contains a single quote (e.g., `"kb' OR '1'='1"`), this would produce invalid SQL. However, `ContentTypes` values are set internally from `fileType` parameters, which are validated elsewhere. The risk is low but worth documenting.

**Recommendation:** No action needed for this fix. Document as a general concern for `buildFilter`.

---

## Finding 6: Observation Indexer Convention Mismatch

**Severity:** LOW

The plan correctly notes that `observation_indexer.go` uses `"observation"` (no `_section` suffix) and defers standardization. This is reasonable.

However, if the team later decides to standardize observation to `"observation_section"`, the same dual-type pattern would be needed in `observation_indexer.go:179`:
```go
ContentTypes: []string{"observation"},
```

**Recommendation:** Add to follow-up: "If observation convention is standardized, update `SearchObservations` to use dual-type pattern."

---

## What Is Correct

| Aspect | Status | Notes |
|--------|--------|-------|
| Bug 1 dual-type search | CORRECT | `["ticket", "ticket_section"]` works in both MemoryVectorDB and LanceDB |
| Bug 1 backward compatibility | CORRECT | Old embedder chunks (`"ticket"`) still found |
| Bug 1 embedder update | CORRECT | `ticket_embedder.go` stores `"ticket_section"` going forward |
| Bug 1 test updates | CORRECT | Test seed data uses `"ticket_section"` |
| Bug 2 reindex pattern | CORRECT | Adding `"bug"` handler follows existing `"kb"`/`"policy"` pattern |
| Bug 2 content type | CORRECT | `ChunkMarkdownContent(..., "bug", ...)` produces `"bug_section"` which matches search |
| Convention documentation | CORRECT | Adding comment to `chunker.go` is the right place |
| Migration strategy | CORRECT | Dual-type search handles backward compatibility; no data migration needed |
| Risk assessment | CORRECT | Dual-type overhead is negligible |
| Follow-up items | CORRECT | `ticket_embedder.go` tests, observation standardization, admin UI update |

---

## Behavioral Correctness Check

### Bug 1: Dual-Type Search
| Step | Expected | Actual |
|------|----------|--------|
| Old chunk with `ContentType: "ticket"` | Found by `ContentTypes: ["ticket", "ticket_section"]` | CORRECT — MemoryVectorDB and LanceDB both match either type |
| New chunk with `ContentType: "ticket_section"` | Found by `ContentTypes: ["ticket", "ticket_section"]` | CORRECT |
| Search with empty `ContentTypes` | All chunks found | CORRECT — main chat path unaffected |

### Bug 2: Bug Reindex
| Step | Expected | Actual |
|------|----------|--------|
| `ReindexAllContent` processes `"bug"` file | Chunks with `ContentType: "bug_section"` inserted | CORRECT — follows `"kb"`/`"policy"` pattern |
| `InferResolutionForNewTicket` Search 2 | Finds `"bug_section"` chunks | CORRECT — already searches for `"bug_section"` |

---

## Recommended Rework

### 1. Fix Bug 3 at the Correct Locations (Finding 1)

The Bug 3 fix must be applied to the actual affected handlers, not just `SearchVectorDB`:

```go
// handlers.go:4977 (HandleTestRAGSearch)
if req.FileType != "" {
    searchQuery.ContentTypes = []string{req.FileType, req.FileType + "_section"}
}

// handlers.go:5107 (HandleTenantRAGSearch)
if req.FileType != "" {
    searchQuery.ContentTypes = []string{req.FileType, req.FileType + "_section"}
}
```

Keep the `SearchVectorDB` fix as well (it fixes `HandleRAGSearch`).

### 2. Add Dual-Type Search Test (Finding 2)

Add `TestAskRovo_DualTypeSearchFindsOldAndNewChunks` as described in Finding 2.

### 3. Update Bug 3 Description (Finding 4)

Update Bug 3 to mention all three affected endpoints:
- `HandleTestRAGSearch` — fix in `handlers.go:4977`
- `HandleTenantRAGSearch` — fix in `handlers.go:5107`
- `HandleRAGSearch` — fix in `SearchVectorDB` at `service.go:5031`

### 4. Document Observation Indexer Follow-Up (Finding 6)

Add to follow-up table: "If observation convention is standardized, update `SearchObservations` to use dual-type pattern."

---

## Final Verdict

**REWORK REQUIRED.** Plan 2 correctly addresses the high-severity findings from Plan 1 (backward compatibility, test updates, root cause convention). The Bug 1 and Bug 2 fixes are sound.

However, **Bug 3 is not actually fixed by the proposed changes**. The plan modifies `SearchVectorDB` at `service.go:5031`, but the reported broken endpoints (`HandleTestRAGSearch` and `HandleTenantRAGSearch`) bypass `SearchVectorDB` entirely and construct `SearchQuery` directly in `handlers.go`. The fix must be applied to the correct locations.

Additionally, the plan lacks a test that verifies the dual-type search behavior — the core backward-compatibility claim. Without this test, the fix is fragile.

After addressing these two issues, the plan will be implementation-ready.
