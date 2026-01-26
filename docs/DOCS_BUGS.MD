# Bug Documentation

This document tracks bugs encountered during development, their root causes, and fixes.

---

## BUG-001: LanceDB Embedding Dimension Mismatch

**Status:** Resolved
**Date:** 2026-01-26
**Related Documents:**
- [DOCS_CYBERTRON.MD](./DOCS_CYBERTRON.MD) - Cybertron implementation
- [DOCS_CYBERTRON_DEBUG.MD](./DOCS_CYBERTRON_DEBUG.MD) - Sequence length debugging
- [DOCS_RAG_ALTERNATIVES.MD](./DOCS_RAG_ALTERNATIVES.MD) - Provider comparison

### Error

```
ERROR reindex failed tenantID=4 audience=internal
error="failed to insert chunks for audience internal: failed to add records to LanceDB:
failed to add records: Failed to add data to table: lance error: Append with different schema:
`embedding` should have type fixed_size_list:float:1536 but type was fixed_size_list:float:384"
```

### Root Cause

1. LanceDB table was initially created with Cybertron embeddings (384 dimensions)
2. User switched to OpenRouter embeddings (1536 dimensions)
3. LanceDB enforces strict schema consistency - cannot append vectors with different dimensions to an existing table

### Embedding Dimensions by Provider

| Provider | Model | Dimensions |
|----------|-------|------------|
| Cybertron | all-MiniLM-L6-v2 | 384 |
| Cybertron | all-mpnet-base-v2 | 768 |
| OpenRouter | text-embedding-3-small | 1536 |
| OpenRouter | text-embedding-3-large | 3072 |
| Local | all-MiniLM-L6-v2 | 384 |

### Solution

Implemented auto-detection and table recreation in `vectordb_lance.go`:

```go
// In Insert() method, after generating embeddings:

// Check for embedding dimension mismatch (happens when switching embedding providers)
if len(chunks) > 0 && len(chunks[0].Embedding) > 0 {
    existingDim := db.getTableEmbeddingDimension(ctx)
    newDim := len(chunks[0].Embedding)

    if existingDim > 0 && existingDim != newDim {
        slog.Warn("Embedding dimension mismatch detected, recreating table",
            "table", lanceTableName,
            "existingDimension", existingDim,
            "newDimension", newDim,
            "reason", "embedding provider likely changed")

        if err := db.dropAndRecreateTable(ctx); err != nil {
            return fmt.Errorf("failed to recreate table for dimension change: %w", err)
        }
    }
}
```

### Files Modified

| File | Change |
|------|--------|
| `server/router/api/v1/agent/vectordb_lance.go` | Added `getTableEmbeddingDimension()` and `dropAndRecreateTable()` methods |

### Behavior After Fix

1. System detects dimension mismatch before insert
2. Logs warning with old and new dimensions
3. Automatically drops and recreates table with correct schema
4. Proceeds with insert using new dimensions
5. Existing embeddings are lost (expected - they're incompatible anyway)

### Quick Manual Fix

If you need to manually clear the data:

```bash
# Delete existing LanceDB data
rm -rf build/data/lancedb/

# Run with desired provider
EMBEDDING_PROVIDER=openrouter task run:rag
```

### Verification

1. Start with Cybertron: `EMBEDDING_PROVIDER=cybertron task run:rag:cybertron`
2. Click "Rebuild Index" - creates table with 384-dimension schema
3. Stop server
4. Switch to OpenRouter: `EMBEDDING_PROVIDER=openrouter task run:rag`
5. Click "Rebuild Index" - should auto-detect mismatch and recreate
6. Check logs for: `Embedding dimension mismatch detected, recreating table`
7. Verify index completes successfully

### Key Lesson

When supporting multiple embedding providers with different dimensions:
- LanceDB schema is fixed at table creation time
- Changing providers requires schema migration (or table recreation)
- Auto-detection prevents cryptic schema mismatch errors

---

*Document Version: 1.0*
*Created: 2026-01-26*
