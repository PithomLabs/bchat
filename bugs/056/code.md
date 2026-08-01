# Bug 056 — Content Type Mismatch Fix: Implementation Documentation

**Date:** 2026-08-01
**Status:** Implemented — all 13 tests pass (including `-race`)
**Plan:** `plan5.md` (APPROVED WITH NITS)
**Files Modified:** 4 source files, 1 test file

---

## 1. Implementation Summary

| Sub-Bug | Severity | Description | Fix |
|---------|----------|-------------|-----|
| Bug 1 | HIGH | `InferResolutionForNewTicket` and `EscalateTicket` search for `"ticket"` but chunks are stored as `"ticket_section"` | Dual-type search `["ticket", "ticket_section"]` |
| Bug 2 | HIGH | All three reindex functions skip `"bug"` source files — dead code | Add `"bug"` handler to `ReindexAllContent`, `ReindexTenantContent`, `ReindexTenantContentWithResume` |
| Bug 3 | MEDIUM | Admin RAG search endpoints pass raw `fileType` as ContentTypes but stored values have `_section` suffix | Apply `fileType + "_section"` dual-type pattern at 3 locations |

---

## 2. Root Cause Analysis

The `_section` convention originates from `ChunkMarkdownContent` at `chunker.go:389`:

```go
ContentType: fileType + "_section",
```

This means:
- `fileType="kb"` → `ContentType="kb_section"`
- `fileType="ticket"` → `ContentType="ticket_section"`
- `fileType="bug"` → `ContentType="bug_section"`

However, search queries throughout the codebase used raw `fileType` values (`"ticket"`, `"kb"`, `"policy"`) as `ContentTypes` filters. The LanceDB filter `content_type IN ('ticket')` never matches rows with `content_type = 'ticket_section'`.

The background embedder (`ticket_embedder.go`) had a different inconsistency — it stored `ContentType: "ticket"` directly, bypassing `ChunkMarkdownContent` entirely.

---

## 3. Bug 1: Ticket Content Type Mismatch

### Search Query Fixes

**`service.go:5579` — `EscalateTicket`:**
```go
// Before:
ContentTypes: []string{"ticket"},

// After:
ContentTypes: []string{"ticket", "ticket_section"},
```

**`service.go:5618` — `InferResolutionForNewTicket`:**
```go
// Before:
ContentTypes: []string{"ticket"},

// After:
ContentTypes: []string{"ticket", "ticket_section"},
```

### Background Embedder Fix

**`ticket_embedder.go:106`:**
```go
// Before:
ContentType: "ticket",

// After:
ContentType: "ticket_section",
```

### Backward Compatibility

- Old embedder chunks (`ContentType: "ticket"`) are still found by the dual-type search
- New embedder chunks (`ContentType: "ticket_section"`) are also found
- After next full reindex, all chunks will use `"ticket_section"`

---

## 4. Bug 2: Bug History Dead Code

### `ReindexAllContent` (~line 677) — Uses `ReindexFileVersion`

```go
if entry, ok := fileMap["bug"]; ok {
    if count, err := s.ReindexFileVersion(tenantCtx, tenant.ID, audience, "bug", entry.version, entry.content, maxChunkTokens); err != nil {
        slog.Warn("failed to reindex bug", "tenantID", tenant.ID, "audience", audience, "error", err)
    } else {
        totalChunks += count
    }
}
```

### `ReindexTenantContent` (~line 782) — Uses `ReindexFileVersion`

```go
if entry, ok := fileMap["bug"]; ok {
    if count, err := s.ReindexFileVersion(ctx, tenantID, audience, "bug", entry.version, entry.content, maxChunkTokens); err != nil {
        slog.Error("failed to reindex bug", "tenantID", tenantID, "audience", audience, "error", err)
        return totalChunks, fmt.Errorf("failed to reindex bug for audience %s: %w", audience, err)
    } else {
        totalChunks += count
    }
}
```

### `ReindexTenantContentWithResume` (~line 1175) — Batch Pattern

This function uses a different pattern: collects all chunks into `allChunks`, then batch-inserts via `InsertWithCheckpoint`. The `"kb"` and `"policy"` handlers chunk directly into `allChunks` — bug follows the same pattern:

```go
if entry, ok := fileMap["bug"]; ok && entry.content != "" {
    slog.Info("reindex: chunking bug file",
        "tenant_id", tenantID,
        "audience", audience,
        "content_length", len(entry.content),
        "version", entry.version,
    )
    bugChunks := s.chunker.ChunkMarkdownContent(entry.content, tenantID, audience, "bug", entry.version, maxChunkTokens)
    slog.Info("reindex: bug chunking produced chunks",
        "tenant_id", tenantID,
        "audience", audience,
        "chunk_count", len(bugChunks),
    )
    allChunks = append(allChunks, bugChunks...)
}
```

### Why Different Patterns

| Function | Pattern | Why |
|----------|---------|-----|
| `ReindexAllContent` | `ReindexFileVersion` | Single-tenant, single-audience — uses mutex, active version, retention |
| `ReindexTenantContent` | `ReindexFileVersion` | Same as above |
| `ReindexTenantContentWithResume` | Batch into `allChunks` | Checkpoint/resume support — active version + retention handled at end (lines 1345-1364) |

---

## 5. Bug 3: Admin Search Content Type Mismatch

### Affected Endpoints

| Endpoint | Code Location | Fix |
|----------|--------------|-----|
| `HandleTestRAGSearch` | `handlers.go:4977` | `req.FileType + "_section"` |
| `HandleTenantRAGSearch` | `handlers.go:5107` | `req.FileType + "_section"` |
| `HandleRAGSearch` | `service.go:5031` | `fileType + "_section"` |

### Implementation

**`handlers.go:4977`:**
```go
// Before:
searchQuery.ContentTypes = []string{req.FileType}

// After:
searchQuery.ContentTypes = []string{req.FileType, req.FileType + "_section"}
```

**`handlers.go:5107`:**
```go
// Before:
searchQuery.ContentTypes = []string{req.FileType}

// After:
searchQuery.ContentTypes = []string{req.FileType, req.FileType + "_section"}
```

**`service.go:5031`:**
```go
// Before:
queryObj.ContentTypes = []string{fileType}

// After:
queryObj.ContentTypes = []string{fileType, fileType + "_section"}
```

---

## 6. Convention Documentation

**`chunker.go` — comment added above `ChunkMarkdownContent`:**
```go
// ContentType convention: fileType + "_section".
// Example: fileType="kb" → ContentType="kb_section"
// All producers of DocumentChunk MUST follow this convention.
// See also: ticket_embedder.go, observation_indexer.go
```

### ContentType Table (Post-Fix)

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

## 7. Test Coverage

### Existing Tests Updated

All 5 occurrences of `ContentType: "ticket"` in seed data changed to `ContentType: "ticket_section"`:
- `TestAskRovo_InferResolutionFromSimilarTickets` (2 chunks)
- `TestAskRovo_TenantIsolation` (1 chunk)
- `TestAskRovo_MinScoreThreshold` (1 chunk)
- `TestAskRovo_TopKLimiting` (5 chunks)

### New Test 1: Dual-Type Search (Bug 1)

```go
func TestAskRovo_DualTypeSearchFindsOldAndNewChunks(t *testing.T) {
    // Seeds old-format chunk (ContentType: "ticket") and new-format chunk
    // (ContentType: "ticket_section"), verifies both are found by
    // InferResolutionForNewTicket after the dual-type fix.
}
```

### New Test 2: Dual-Type Admin Search (Bug 3)

```go
func TestSearchVectorDB_DualTypeContentTypes(t *testing.T) {
    // Seeds old-format KB chunk (ContentType: "kb") and new-format KB chunk
    // (ContentType: "kb_section"), verifies both are found by SearchVectorDB
    // with fileType="kb" after the dual-type fix.
}
```

### Test Design

- `controlledEmbeddingService`: keyword-based, seed keywords `["rag", "indexing", "ticket"]`
- HIGH vector (v[0]=1.0) for matching texts, LOW (v[1]=1.0) for non-matching
- Cosine similarity is 1.0 (match) or 0.0 (no match) — exercises MinScore threshold
- `t.Setenv` isolation prevents LanceDB/TICKET_EMBEDDING side effects
- No `t.Parallel()` — file-level warning comment, sequential execution is actual safeguard

---

## 8. Files Modified

| File | Lines Changed | Description |
|------|---------------|-------------|
| `server/router/api/v1/agent/service.go` | +30, -2 | Bug 1 (lines 5579, 5618), Bug 2 (3 reindex functions), Bug 3 (line 5031) |
| `server/router/api/v1/agent/handlers.go` | +2, -2 | Bug 3 (lines 4977, 5107) |
| `server/router/api/v1/agent/ticket_embedder.go` | +1, -1 | Bug 1 (line 106) |
| `server/router/api/v1/agent/chunker.go` | +5 | Convention comment |
| `server/router/api/v1/agent/ticket_rag_inference_test.go` | +52, -5 | Updated seed data + 2 new tests |

**No new files. No schema changes. No frontend changes.**

---

## 9. Migration Strategy

**No data migration required.** The dual-type search handles backward compatibility automatically:

- Old embedder chunks (`ContentType: "ticket"`) are found by `["ticket", "ticket_section"]`
- New reindex chunks (`ContentType: "ticket_section"`) are also found
- After full reindex, all chunks will use the `_section` convention

---

## 10. Follow-Up Work

| Item | Description | Priority |
|------|-------------|----------|
| Add HTTP-level tests for `HandleTestRAGSearch` and `HandleTenantRAGSearch` | Verify dual-type ContentTypes fix at handler level — current test only covers `SearchVectorDB` | MEDIUM |
| Verify LanceDB `buildFilter` supports multiple ContentTypes in IN clause | Already works (vectordb_lance.go:1179), but not explicitly tested | LOW |
| Add tests for `ticket_embedder.go` | Verify ContentType consistency with inference path | MEDIUM |
| Add unit test for `ReindexAllContent` with bug source file | Verify bug reindex path produces chunks | MEDIUM |
| Standardize observation indexer | If convention is standardized, update `SearchObservations` to use dual-type pattern | LOW |
| Admin search UI update | Update admin RAG search UI to show content type with `_section` suffix | LOW |

---

## 11. Adversarial Review Prompt

```
You are a senior Go architect reviewing a bug fix implementation for a
content type mismatch in a RAG pipeline. The fix uses dual-type search
(["ticket", "ticket_section"]) for backward compatibility, updates the
embedder convention, adds bug content to the reindex loop, and fixes
admin search at three affected locations.

Review the implementation critically. Focus on:

1. CORRECTNESS: Do all code changes match the plan? Are the patterns
   consistent with existing code? Verify each edit against the source.

2. COMPLETENESS: Are all code locations covered? Are there any remaining
   ContentTypes filters that might have the same mismatch? Search for
   all ContentTypes usage across the codebase.

3. TEST QUALITY: Do tests exercise the right code paths? Are assertions
   sufficient? Any false-positive/negative risks from the
   controlledEmbeddingService design? Will dual-type tests actually
   catch regressions?

4. CONVENTION: Is the "_section" convention clearly documented? Will
   future developers follow it? Are there any producers that bypass
   ChunkMarkdownContent and store raw fileType?

5. MIGRATION: Is backward compatibility correct? Will old and new chunks
   both be found? What happens during a rolling deployment where some
   pods have the fix and others don't?

6. RISK: What's the worst case if this fix is applied incorrectly?
   Any rollback concerns? Could the dual-type search mask a future
   convention change?

7. EDGE CASES: What happens with empty content, nil tenant, concurrent
   reindex, large bug corpus, or mixed old/new chunks in the same query?

8. PERFORMANCE: Does dual-type search add measurable overhead? Any
   concerns with IN clause size in LanceDB SQL filter? Does the
   extra string concatenation in the hot path matter?
```
