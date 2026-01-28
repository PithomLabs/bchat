# Log Analysis: Duplicate RAG Indexing on KB Upload

**Date:** 2026-01-28
**Issue:** RAG indexing is performed twice when uploading KB files for internal audience

## Summary

When a KB file is uploaded with `retrievalMode=rag` and `audience=internal`, the RAG indexing pipeline runs **twice** - once synchronously during import and once asynchronously immediately after. This doubles processing time and API costs for embeddings.

## Evidence from Logs

### First Indexing (16:33:42 - 16:35:21)
```
16:33:42 INFO No KB annotations found, using markdown-based chunking tenantID=4 audience=internal
16:33:42 INFO Starting batched insert totalChunks=2732 batchSize=100
... (28 batches processed)
16:35:21 INFO Created IVF-PQ vector index table=kb_documents
16:35:21 INFO Completed batched insert totalChunks=2732
16:35:21 INFO Indexed content for RAG tenantID=4 audience=internal chunks=2732
16:35:21 INFO file imported tenant=inc audience=internal fileType=kb totalTokens=2735634 retrievalMode=rag
```

### Second Indexing (16:35:21 - 16:37:08)
```
16:35:21 INFO Starting RAG reindex for tenant tenantID=4 tenant=inc audienceFilter=internal
16:35:21 INFO DEBUG: Found source files for reindex tenantID=4 audienceFilter=internal fileCount=2
16:35:21 INFO DEBUG: About to insert chunks tenantID=4 audience=internal chunkCount=2732
16:35:21 INFO Starting batched insert totalChunks=2732 batchSize=100
... (28 batches processed again)
16:37:08 INFO Completed batched insert totalChunks=2732
16:37:08 INFO Reindexed content for tenant tenantID=4 tenant=inc audience=internal chunks=2732
16:37:08 INFO RAG reindex completed for tenant tenantID=4 tenant=inc totalChunks=2732
```

## Root Cause Analysis

The duplicate indexing occurs due to two separate code paths being triggered during file upload:

### Code Path 1: `indexContentForRAG()` (Synchronous)

**File:** `server/router/api/v1/agent/handlers.go`
**Location:** Line 1014 within `importAudienceData()`

```go
// Index content for RAG pipeline
if err := h.indexContentForRAG(ctx, tenantID, audienceType, parsedKB, parsedPolicy, kbContent, policyContent); err != nil {
    slog.Warn("Failed to index content for RAG", "error", err, "tenantID", tenantID, "audience", audienceType)
    // Don't fail the import if indexing fails - RAG is an enhancement
}
```

This function:
1. Deletes existing chunks for the tenant/audience
2. Chunks the raw content using markdown-based chunking
3. Inserts chunks with embeddings into LanceDB
4. Creates vector index

### Code Path 2: `ReindexTenantContent()` (Asynchronous)

**File:** `server/router/api/v1/agent/handlers.go`
**Location:** Lines 623-631 in `HandleImportFiles()`

```go
// Trigger RAG indexing only for internal audience in RAG mode
// External audience is never indexed (uses long_context retrieval)
if retrievalMode == "rag" && audienceType == "internal" {
    go func() {
        if _, err := h.service.ReindexTenantContent(context.Background(), tenant.ID, "internal"); err != nil {
            slog.Error("background RAG indexing failed", "tenant", slug, "error", err)
        }
    }()
}
```

This background goroutine is triggered **immediately after** `importAudienceData()` returns, which has already performed indexing.

## Execution Timeline

```
t=0     HandleImportFiles() called
t=1     └── importAudienceData() called
t=2         └── indexContentForRAG() called    ← FIRST INDEXING STARTS
t=99s       └── indexContentForRAG() returns   ← FIRST INDEXING ENDS
t=99s   └── importAudienceData() returns
t=99s   ReindexTenantContent() goroutine spawned ← SECOND INDEXING STARTS
t=99s   HandleImportFiles() returns HTTP 200
t=207s  ReindexTenantContent() goroutine completes ← SECOND INDEXING ENDS
```

## Impact

| Metric | Expected | Actual |
|--------|----------|--------|
| Batches processed | 28 | 56 (2x) |
| Chunks indexed | 2,732 | 5,464 (2x) |
| Embedding API calls | ~28 | ~56 (2x) |
| Processing time | ~99 sec | ~207 sec (2x) |

### Cost Impact
- **Embedding API costs doubled** - each chunk requires an embedding API call
- For 2,732 chunks at ~$0.0001/1K tokens, duplicate processing adds unnecessary cost

## Recommended Fix

Remove the redundant background reindex call since `indexContentForRAG()` already handles indexing during import.

**Option A:** Remove background reindex (lines 623-631)

The `indexContentForRAG()` call at line 1014 already handles all indexing. The background reindex was likely added before `indexContentForRAG()` existed or was intended for a different purpose.

```go
// DELETE these lines (623-631):
// Trigger RAG indexing only for internal audience in RAG mode
// External audience is never indexed (uses long_context retrieval)
if retrievalMode == "rag" && audienceType == "internal" {
    go func() {
        if _, err := h.service.ReindexTenantContent(context.Background(), tenant.ID, "internal"); err != nil {
            slog.Error("background RAG indexing failed", "tenant", slug, "error", err)
        }
    }()
}
```

**Option B:** Remove inline indexing, keep only background reindex

If background processing is preferred to avoid blocking the HTTP response:
1. Remove `indexContentForRAG()` call at line 1014
2. Keep the background `ReindexTenantContent()` call

Option A is recommended as it:
- Provides synchronous feedback on indexing success/failure
- Simpler code path
- Already works correctly

## Related Files

- `server/router/api/v1/agent/handlers.go` - Import handlers and indexing calls
- `server/router/api/v1/agent/service.go` - `ReindexTenantContent()` implementation
- `server/router/api/v1/agent/vectordb.go` - LanceDB operations
- `server/router/api/v1/agent/chunker.go` - Document chunking logic
