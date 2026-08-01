# Bug 056: Content Type Mismatch — InferResolutionForNewTicket Search Never Matches Stored Chunks

**Date:** 2026-08-01
**Status:** Ready for review
**Related:** Bug 052 (per-ticket RAG indexing), Bug 055 (Ask Rovo E2E test)

---

## Background

During manual E2E testing of the Ask Rovo feature, tickets #172 and #173 were created on tenant 19 (hackathon-demo). Both tickets were successfully indexed into LanceDB (source files 110, 111 exist with correct content hashes), but `InferResolutionForNewTicket` returned empty results — no AI Suggestion comment, no InternalNotes populated.

Investigation revealed a content type mismatch: `ChunkMarkdownContent` stores chunks with `ContentType: fileType + "_section"` (e.g., `"ticket_section"`), but `InferResolutionForNewTicket` searches for the raw file type (e.g., `"ticket"`). The LanceDB filter `content_type IN ('ticket')` never matches rows with `content_type = 'ticket_section'`.

Further audit revealed **two additional bugs** in the same subsystem.

---

## Bug 1: Ticket Content Type Mismatch (HIGH)

**Impact:** Tickets indexed via `IndexTicketContent` (on-create path) are invisible to inference. Only tickets indexed by the background `ticket_embedder.go` (which stores `ContentType: "ticket"` directly) are searchable.

### Root Cause

| Step | Code | Value |
|------|------|-------|
| `IndexTicketContent` → `ReindexFileVersion` | `service.go:5777` | `fileType = "ticket"` |
| `ChunkMarkdownContent` stores chunk | `chunker.go:389` | `ContentType: fileType + "_section"` → `"ticket_section"` |
| LanceDB row | vectordb_lance.go | `content_type = "ticket_section"` |
| `InferResolutionForNewTicket` Search 1 | `service.go:5618` | `ContentTypes: []string{"ticket"}` → filter: `content_type IN ('ticket')` |
| `EscalateTicket` | `service.go:5579` | `ContentTypes: []string{"ticket"}` → filter: `content_type IN ('ticket')` |
| **Result** | | **Filter never matches** — stored `"ticket_section"` ≠ searched `"ticket"` |

### Fix

Change search queries to match stored content type:

```go
// service.go:5579 (EscalateTicket)
- ContentTypes: []string{"ticket"},
+ ContentTypes: []string{"ticket_section"},

// service.go:5618 (InferResolutionForNewTicket, Search 1)
- ContentTypes: []string{"ticket"},
+ ContentTypes: []string{"ticket_section"},
```

### Verification

After fix, create two tickets with similar content (per trigger052.md / trigger_052.md). The second ticket should show an AI Suggestion comment referencing the first.

---

## Bug 2: Bug History Search Is Dead Code (HIGH)

**Impact:** `InferResolutionForNewTicket` Search 2 (bug history, MinScore=0.5) always returns empty. The `import-bug-rag` tool creates `AgentSourceFile` rows with `file_type="bug"`, but no reindex path processes them.

### Root Cause

The three reindex functions only handle `"kb"` and `"policy"`:

| Function | Lines | Handles `"bug"`? |
|----------|-------|-------------------|
| `ReindexAllContent` | `service.go:664-677` | NO |
| `ReindexTenantContent` | `service.go:767-782` | NO |
| `ReindexTenantContentWithResume` | `service.go:1150-1175` | NO |

Bug source files are created but silently skipped during reindex. No chunks with `ContentType: "bug_section"` are ever inserted into LanceDB.

### Fix

Add `"bug"` handler to all three reindex functions, following the same pattern as `"kb"` and `"policy"`:

**`ReindexAllContent` (service.go:664):**
```go
// After the policy block (~line 677):
if entry, ok := fileMap["bug"]; ok {
    maxChunkTokens := GetMaxChunkTokens(s.profile embeddingProvider)
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

**`ReindexTenantContent` (service.go:767):** Same pattern.

**`ReindexTenantContentWithResume` (service.go:1150):** Same pattern, inside the resume loop.

### Note on Content Type

`ChunkMarkdownContent` with `fileType="bug"` produces `ContentType = "bug_section"`. This matches what `InferResolutionForNewTicket` Search 2 already searches for (`ContentTypes: []string{"bug_section"}`). No search query change needed for this bug.

### Verification

After fix, run full tenant reindex. Check LanceDB for bug_section chunks:
```bash
curl -X POST http://localhost:5230/api/v1/agent/:slug/reindex
curl http://localhost:5230/api/v1/admin/rag/stats
```

---

## Bug 3: Admin RAG Search Content Type Mismatch (LOW)

**Impact:** Admin debug endpoints (`HandleTestRAGSearch`, `HandleTenantRAGSearch`) search for raw file types (`"kb"`, `"policy"`) but stored values are `"kb_section"`, `"policy_section"`. These endpoints always return empty results when filtering by file type.

### Root Cause

| Code Location | Searches For | Stored As | Match? |
|---------------|-------------|-----------|--------|
| `handlers.go:4977` | `[]string{req.FileType}` → `"kb"` | `"kb_section"` | NO |
| `handlers.go:5107` | `[]string{req.FileType}` → `"policy"` | `"policy_section"` | NO |
| `service.go:5031` | `[]string{fileType}` → `"kb"` | `"kb_section"` | NO |

### Fix (Optional — defer to future work)

This is an admin debug tool, not a production path. Fix when touching admin RAG endpoints.

**Option 1:** Append `_section` in `SearchVectorDB`:
```go
// service.go:5031
- queryObj.ContentTypes = []string{fileType}
+ queryObj.ContentTypes = []string{fileType + "_section"}
```

**Option 2:** Accept both forms in `buildFilter` (more resilient):
```go
// vectordb_lance.go:1179
if len(query.ContentTypes) > 0 {
    types := make([]string, 0, len(query.ContentTypes)*2)
    for _, ct := range query.ContentTypes {
        types = append(types, fmt.Sprintf("'%s'", ct))
        types = append(types, fmt.Sprintf("'%s_section'", ct))
    }
    filterParts = append(filterParts, fmt.Sprintf("content_type IN (%s)", strings.Join(types, ", ")))
}
```

---

## ContentType Mismatch Audit — Complete Table

| Stored ContentType | Produced by | Searched as | Searched by | Match? | Impact |
|-------------------|-------------|-------------|-------------|--------|--------|
| `"kb_section"` | ChunkMarkdownContent | `""` (all) | RetrieveContextForQuery | YES | None — main chat path |
| `"kb_section"` | ChunkMarkdownContent | `"kb"` | Admin search | **NO** | Debug tool only |
| `"policy_section"` | ChunkMarkdownContent | `""` (all) | RetrieveContextForQuery | YES | None — main chat path |
| `"policy_section"` | ChunkMarkdownContent | `"policy"` | Admin search | **NO** | Debug tool only |
| `"ticket_section"` | ChunkMarkdownContent | `"ticket"` | InferResolution, Escalate | **NO** | **BUG #1** |
| `"ticket"` | ticket_embedder.go | `"ticket"` | InferResolution, Escalate | YES | Background embedder |
| `"observation"` | observation_indexer.go | `"observation"` | SearchObservations | YES | None |
| `"bug_section"` | **Never produced** | `"bug_section"` | InferResolution Search 2 | **N/A** | **BUG #2** |
| `"cluster"` | ticket_embedder.go | (not searched) | N/A | N/A | Metadata only |

---

## Implementation Plan

### Step 1: Fix Ticket Search (Bug 1)

**File:** `server/router/api/v1/agent/service.go`

| Line | Current | Fixed |
|------|---------|-------|
| 5579 | `ContentTypes: []string{"ticket"},` | `ContentTypes: []string{"ticket_section"},` |
| 5618 | `ContentTypes: []string{"ticket"},` | `ContentTypes: []string{"ticket_section"},` |

### Step 2: Add Bug Reindex (Bug 2)

**File:** `server/router/api/v1/agent/service.go`

Add `"bug"` handler in three functions:
- `ReindexAllContent` (~line 677)
- `ReindexTenantContent` (~line 782)
- `ReindexTenantContentWithResume` (~line 1175)

Pattern (copy from `"kb"` block, change fileType to `"bug"`):
```go
if entry, ok := fileMap["bug"]; ok {
    maxChunkTokens := GetMaxChunkTokens(...)
    chunks := s.chunker.ChunkMarkdownContent(entry.content, tenantID, "internal", "bug", entry.version, maxChunkTokens)
    if len(chunks) > 0 {
        if err := s.vectorDB.Insert(ctx, chunks); err != nil {
            slog.Error("failed to insert bug chunks", "error", err)
        }
    }
}
```

### Step 3: Run Existing Tests

```bash
go test -v -run TestAskRovo ./server/router/api/v1/agent/ -count=1
```

Tests should continue to pass (they use `MemoryVectorDB` with `controlledEmbeddingService`, not LanceDB).

### Step 4: Manual Verification

1. Reindex tenant 19 (hackathon-demo)
2. Create ticket #172 (per trigger052.md) — wait for indexing
3. Create ticket #173 (per trigger_052.md) — should now find ticket #172
4. Check for AI Suggestion comment on ticket #173
5. Check InternalNotes on ticket #173

---

## Adversarial Review Prompt

```
You are a senior Go architect reviewing a bug fix plan for a content type
mismatch in a RAG pipeline. The bug causes InferResolutionForNewTicket to
never find indexed ticket chunks because the search filter uses "ticket"
but chunks are stored as "ticket_section".

Review this plan critically. Focus on:

1. CORRECTNESS: Does changing the search from "ticket" to "ticket_section"
   break anything? Are there other code paths that search for "ticket" and
   expect different behavior?

2. SCOPE: Is the 3-bug scope appropriate? Should Bug 3 (admin search) be
   included or deferred?

3. REINDEX IMPACT: Does adding "bug" to the reindex loop require a full
   tenant reindex? Will existing bug data be picked up automatically?

4. BACKWARD COMPATIBILITY: If some LanceDB data was indexed by the old
   ticket_embedder.go path (ContentType="ticket"), will searching for
   "ticket_section" miss those chunks?

5. CONTENT TYPE CONVENTION: Is the "_section" suffix a good convention?
   Should we standardize it across all content types?

6. TESTING: Are the existing tests sufficient? Do we need new tests for
   the bug reindex path?

7. RISK: What's the worst case if this fix is applied incorrectly?
   Could it break the main chat RAG path?

8. ORDER OF OPERATIONS: Should Bug 1 and Bug 2 be fixed together or
   separately? What's the dependency?
```

---

## Files to Modify

| File | Changes |
|------|---------|
| `server/router/api/v1/agent/service.go` | Fix search ContentTypes (Bug 1), add bug reindex (Bug 2) |

**No new files. No schema changes. No frontend changes.**

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Changing search from "ticket" to "ticket_section" misses old "ticket" chunks | Medium | Old chunks from ticket_embedder.go are still found via empty ContentTypes filter in RetrieveContextForQuery |
| Adding bug reindex creates large number of chunks | Low | Bug content is typically small; retention policy keeps last 5 versions |
| Existing tests break | Low | Tests use MemoryVectorDB, not LanceDB — unaffected |
| Main chat RAG path breaks | None | RetrieveContextForQuery uses empty ContentTypes filter — unaffected |
