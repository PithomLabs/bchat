# Plan 4: Bug 056 — Content Type Mismatch Fix (Final)

**Date:** 2026-08-01
**Status:** Ready for implementation
**Review:** 4 findings — all addressed
**Related:** Bug 052 (per-ticket RAG indexing), Bug 055 (Ask Rovo E2E test)

---

## Background

During manual E2E testing of the Ask Rovo feature, tickets #172 and #173 were created on tenant 19 (hackathon-demo). Both tickets were successfully indexed into LanceDB (source files 110, 111 exist with correct content hashes), but `InferResolutionForNewTicket` returned empty results — no AI Suggestion comment, no InternalNotes populated.

Investigation revealed a content type mismatch: `ChunkMarkdownContent` stores chunks with `ContentType: fileType + "_section"` (e.g., `"ticket_section"`), but `InferResolutionForNewTicket` searches for the raw file type (e.g., `"ticket"`). The LanceDB filter `content_type IN ('ticket')` never matches rows with `content_type = 'ticket_section'`.

Further audit revealed two additional bugs: bug history search is dead code (no reindex path processes `"bug"` source files), and admin RAG search has the same content type mismatch for KB/Policy.

---

## What Changed from Plan 3

| # | Finding | Severity | Fix Applied |
|---|---------|----------|-------------|
| 1 | Dual-type test query produces LOW vector → no results → test fails | **HIGH** | **Fixed** — title changed to `"Ticket RAG Indexing Dual Type"` (contains seed keywords) |
| 2 | No test for Bug 3 admin search fix | MEDIUM | **Added** — `TestSearchVectorDB_DualTypeContentTypes` |
| 3 | `HandleRAGSearch` line number incorrect | LOW | **Fixed** — corrected to `handlers.go:6036` |
| 4 | No verification Bug 2 reindex paths compile | LOW | **Documented** — added to follow-up table |

---

## ContentType Convention

**Standard:** `ChunkMarkdownContent` appends `"_section"` to `fileType` to produce `ContentType`.

| fileType | Stored ContentType | Producer |
|----------|-------------------|----------|
| `"kb"` | `"kb_section"` | ChunkMarkdownContent |
| `"policy"` | `"policy_section"` | ChunkMarkdownContent |
| `"ticket"` | `"ticket_section"` | ChunkMarkdownContent, ticket_embedder.go (after fix) |
| `"bug"` | `"bug_section"` | ChunkMarkdownContent (after fix) |
| `"observation"` | `"observation"` | observation_indexer.go (different convention — deferred) |

All producers MUST follow this convention. Code comment to be added to `chunker.go`.

---

## Bug 1: Ticket Content Type Mismatch (HIGH)

**Root Cause:** `InferResolutionForNewTicket` and `EscalateTicket` search for `"ticket"`, but `IndexTicketContent` → `ChunkMarkdownContent` stores `"ticket_section"`. The background embedder (`ticket_embedder.go`) stores `"ticket"` directly — a different convention.

### Fix

**Step 1: Make search accept both types (backward compatible)**

```go
// service.go:5579 (EscalateTicket)
- ContentTypes: []string{"ticket"},
+ ContentTypes: []string{"ticket", "ticket_section"},

// service.go:5618 (InferResolutionForNewTicket, Search 1)
- ContentTypes: []string{"ticket"},
+ ContentTypes: []string{"ticket", "ticket_section"},
```

**Step 2: Update background embedder to follow convention**

```go
// ticket_embedder.go:106
- ContentType: "ticket",
+ ContentType: "ticket_section",
```

**Step 3: Update tests**

```go
// ticket_rag_inference_test.go — all seed chunks
- ContentType: "ticket",
+ ContentType: "ticket_section",
```

### Backward Compatibility

- Old embedder chunks (`ContentType: "ticket"`) are still found by the dual-type search
- New embedder chunks (`ContentType: "ticket_section"`) are also found
- After next full reindex, all chunks will use `"ticket_section"`

---

## Bug 2: Bug History Search Is Dead Code (HIGH)

**Root Cause:** `import-bug-rag` creates `AgentSourceFile` rows with `file_type="bug"`, but all three reindex functions only handle `"kb"` and `"policy"`. Bug content is silently skipped.

### Fix

Add `"bug"` handler to all three reindex functions:

**`ReindexAllContent` (~line 677):**
```go
if entry, ok := fileMap["bug"]; ok {
    maxChunkTokens := GetMaxChunkTokens(...)
    chunks := s.chunker.ChunkMarkdownContent(entry.content, 0, "internal", "bug", entry.version, maxChunkTokens)
    if len(chunks) > 0 {
        if err := s.vectorDB.Insert(ctx, chunks); err != nil {
            slog.Error("failed to insert bug chunks", "error", err)
        } else {
            slog.Info("indexed bug content", "chunks", len(chunks))
        }
    }
}
```

**`ReindexTenantContent` (~line 782):** Same pattern.

**`ReindexTenantContentWithResume` (~line 1175):** Same pattern, inside the resume loop.

### Note

`ChunkMarkdownContent` with `fileType="bug"` produces `ContentType = "bug_section"`. This already matches what `InferResolutionForNewTicket` Search 2 searches for (`ContentTypes: []string{"bug_section"}`). No search query change needed.

---

## Bug 3: Admin RAG Search Content Type Mismatch (MEDIUM)

**Root Cause:** Three admin search endpoints pass raw `fileType` values (`"kb"`, `"policy"`) as ContentTypes, but stored values are `"kb_section"`, `"policy_section"`.

### Affected Endpoints

| Endpoint | Code Location | Calls |
|----------|--------------|-------|
| `HandleTestRAGSearch` | `handlers.go:4977` | `vectorDB.Search` directly |
| `HandleTenantRAGSearch` | `handlers.go:5107` | `vectorDB.Search` directly |
| `HandleRAGSearch` | `handlers.go:6036` | `SearchVectorDB` at `service.go:5031` |

### Fix

Apply dual-type pattern to all three locations:

```go
// handlers.go:4977 (HandleTestRAGSearch)
- searchQuery.ContentTypes = []string{req.FileType}
+ searchQuery.ContentTypes = []string{req.FileType, req.FileType + "_section"}

// handlers.go:5107 (HandleTenantRAGSearch)
- searchQuery.ContentTypes = []string{req.FileType}
+ searchQuery.ContentTypes = []string{req.FileType, req.FileType + "_section"}

// service.go:5031 (SearchVectorDB — fixes HandleRAGSearch)
- queryObj.ContentTypes = []string{fileType}
+ queryObj.ContentTypes = []string{fileType, fileType + "_section"}
```

---

## ContentType Audit — Complete Table (Post-Fix)

| Stored ContentType | Produced by | Searched as | Searched by | Match? |
|-------------------|-------------|-------------|-------------|--------|
| `"kb_section"` | ChunkMarkdownContent | `["kb", "kb_section"]` | SearchVectorDB, HandleTestRAGSearch, HandleTenantRAGSearch | YES |
| `"kb_section"` | ChunkMarkdownContent | `""` (all) | RetrieveContextForQuery | YES |
| `"policy_section"` | ChunkMarkdownContent | `["policy", "policy_section"]` | SearchVectorDB, HandleTestRAGSearch, HandleTenantRAGSearch | YES |
| `"policy_section"` | ChunkMarkdownContent | `""` (all) | RetrieveContextForQuery | YES |
| `"ticket_section"` | ChunkMarkdownContent | `["ticket", "ticket_section"]` | InferResolution, Escalate | YES |
| `"ticket_section"` | ticket_embedder.go (after fix) | `["ticket", "ticket_section"]` | InferResolution, Escalate | YES |
| `"observation"` | observation_indexer.go | `["observation"]` | SearchObservations | YES |
| `"bug_section"` | ChunkMarkdownContent (after fix) | `["bug_section"]` | InferResolution Search 2 | YES |
| `"cluster"` | ticket_embedder.go | (not searched) | N/A | N/A |

---

## Implementation Plan

### Step 1: Fix Search Queries (Bug 1)

**File:** `server/router/api/v1/agent/service.go`

| Line | Current | Fixed |
|------|---------|-------|
| 5579 | `ContentTypes: []string{"ticket"}` | `ContentTypes: []string{"ticket", "ticket_section"}` |
| 5618 | `ContentTypes: []string{"ticket"}` | `ContentTypes: []string{"ticket", "ticket_section"}` |

### Step 2: Update Background Embedder (Bug 1)

**File:** `server/router/api/v1/agent/ticket_embedder.go`

| Line | Current | Fixed |
|------|---------|-------|
| 106 | `ContentType: "ticket"` | `ContentType: "ticket_section"` |

### Step 3: Fix Admin Search (Bug 3)

**File:** `server/router/api/v1/agent/handlers.go`

| Line | Current | Fixed |
|------|---------|-------|
| 4977 | `searchQuery.ContentTypes = []string{req.FileType}` | `searchQuery.ContentTypes = []string{req.FileType, req.FileType + "_section"}` |
| 5107 | `searchQuery.ContentTypes = []string{req.FileType}` | `searchQuery.ContentTypes = []string{req.FileType, req.FileType + "_section"}` |

**File:** `server/router/api/v1/agent/service.go`

| Line | Current | Fixed |
|------|---------|-------|
| 5031 | `queryObj.ContentTypes = []string{fileType}` | `queryObj.ContentTypes = []string{fileType, fileType + "_section"}` |

### Step 4: Add Bug Reindex (Bug 2)

**File:** `server/router/api/v1/agent/service.go`

Add `"bug"` handler in three functions:
- `ReindexAllContent` (~line 677)
- `ReindexTenantContent` (~line 782)
- `ReindexTenantContentWithResume` (~line 1175)

### Step 5: Add Convention Comment

**File:** `server/router/api/v1/agent/chunker.go`

Add comment at `ChunkMarkdownContent` function (~line 333):
```go
// ContentType convention: fileType + "_section".
// Example: fileType="kb" → ContentType="kb_section"
// All producers of DocumentChunk MUST follow this convention.
// See also: ticket_embedder.go, observation_indexer.go
```

### Step 6: Update Tests

**File:** `server/router/api/v1/agent/ticket_rag_inference_test.go`

1. Change all `ContentType: "ticket"` in seed data to `ContentType: "ticket_section"`
2. Add new test: `TestAskRovo_DualTypeSearchFindsOldAndNewChunks`
3. Add new test: `TestSearchVectorDB_DualTypeContentTypes`

### Step 7: Run Tests

```bash
go test -v -run TestAskRovo ./server/router/api/v1/agent/ -count=1
go test -v -race -run TestAskRovo ./server/router/api/v1/agent/ -count=1
go test -v -run TestSearchVectorDB ./server/router/api/v1/agent/ -count=1
```

### Step 8: Manual Verification

1. Reindex tenant 19
2. Create ticket #172 (per trigger052.md)
3. Create ticket #173 (per trigger_052.md)
4. Check for AI Suggestion comment on ticket #173

---

## New Test 1: Dual-Type Search (Bug 1)

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

    // Title MUST contain seed keywords for controlledEmbeddingService to return HIGH vector
    memo := createTestMemo(t, ctx, ts, user, tenant, "Test")
    ticket, err := ts.CreateTicket(ctx, &store.Ticket{
        Title: "Ticket RAG Indexing Dual Type", // contains "ticket", "rag", "indexing"
        Description: "/m/" + memo.UID,
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

## New Test 2: Dual-Type Admin Search (Bug 3)

```go
func TestSearchVectorDB_DualTypeContentTypes(t *testing.T) {
    ctx, ts, svc, tenant, _ := setupAskRovoTest(t, []string{"rag", "indexing", "ticket"})

    // Seed old-format KB chunk
    svc.vectorDB.Insert(ctx, []DocumentChunk{{
        ID: "old_kb", TenantID: tenant.ID, AudienceType: "external",
        ContentType: "kb", Title: "Old KB",
        Content: "RAG indexing for kb", IsActive: true,
    }})

    // Seed new-format KB chunk
    svc.vectorDB.Insert(ctx, []DocumentChunk{{
        ID: "new_kb", TenantID: tenant.ID, AudienceType: "external",
        ContentType: "kb_section", Title: "New KB",
        Content: "RAG indexing for kb section", IsActive: true,
    }})

    result, err := svc.SearchVectorDB(ctx, tenant.ID, "external", "kb", "RAG indexing", 5, nil)
    require.NoError(t, err)
    require.NotEmpty(t, result.Chunks)

    ids := make([]string, len(result.Chunks))
    for i, c := range result.Chunks {
        ids[i] = c.ID
    }
    require.Contains(t, ids, "old_kb")
    require.Contains(t, ids, "new_kb")
}
```

---

## Migration Strategy

**Existing LanceDB data:**
- Chunks from `ticket_embedder.go` with `ContentType: "ticket"` — still found by dual-type search
- Chunks from `IndexTicketContent` with `ContentType: "ticket_section"` — found by dual-type search
- After full reindex, all chunks will use `"ticket_section"` (embedder updated)

**No data migration required.** The dual-type search handles backward compatibility automatically.

---

## Files to Modify

| File | Changes |
|------|---------|
| `server/router/api/v1/agent/service.go` | Fix search ContentTypes (Bug 1, Bug 3), add bug reindex (Bug 2) |
| `server/router/api/v1/agent/handlers.go` | Fix admin search ContentTypes (Bug 3) |
| `server/router/api/v1/agent/ticket_embedder.go` | Update ContentType to `"ticket_section"` |
| `server/router/api/v1/agent/chunker.go` | Add convention comment |
| `server/router/api/v1/agent/ticket_rag_inference_test.go` | Update test seed ContentType, add 2 new tests |

**No new files. No schema changes. No frontend changes.**

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Dual-type search adds slight overhead | None | Single额外的字符串比较 — negligible |
| Embedder stores `"ticket_section"` but old chunks have `"ticket"` | None | Dual-type search handles both |
| Bug reindex creates many chunks for large bug corpus | Low | Retention policy keeps last 5 versions |
| Observation indexer uses different convention (`"observation"`) | None | Already searched correctly — not affected |
| Main chat RAG path breaks | None | `RetrieveContextForQuery` uses empty ContentTypes filter — unaffected |

---

## Follow-Up Work

| Item | Description | Priority |
|------|-------------|----------|
| Add tests for `ticket_embedder.go` | Verify ContentType consistency with inference path | MEDIUM |
| Add unit test for `ReindexAllContent` with bug source file | Verify bug reindex path produces chunks | MEDIUM |
| Standardize observation indexer | If convention is standardized, update `SearchObservations` to use dual-type pattern | LOW |
| Admin search UI update | Update admin RAG search UI to show content type with `_section` suffix | LOW |
| Test `HandleTestRAGSearch` and `HandleTenantRAGSearch` handlers directly | Currently only `SearchVectorDB` is tested; handler-level tests would catch handler-specific issues | LOW |

---

## Adversarial Review Prompt

```
You are a senior Go architect reviewing a final bug fix plan for a content
type mismatch in a RAG pipeline. The plan uses dual-type search
(["ticket", "ticket_section"]) for backward compatibility, updates the
embedder to use "ticket_section", adds bug content to the reindex loop,
and fixes admin search at all three affected locations.

The plan includes two new tests:
1. TestAskRovo_DualTypeSearchFindsOldAndNewChunks — verifies backward
   compatibility for ticket search (Bug 1)
2. TestSearchVectorDB_DualTypeContentTypes — verifies admin search
   backward compatibility (Bug 3)

Review this final plan critically. Focus on:

1. TEST CORRECTNESS: Do the new tests correctly verify the dual-type
   behavior? Will they pass with the controlledEmbeddingService?

2. COMPLETENESS: Are all code locations covered? Are there any other
   ContentTypes filters that might have the same mismatch?

3. CONVENTION: Is the "_section" convention clearly documented? Will
   future developers follow it?

4. MIGRATION: Is the "no data migration needed" claim correct?

5. RISK: What's the worst case if this fix is applied incorrectly?
```
