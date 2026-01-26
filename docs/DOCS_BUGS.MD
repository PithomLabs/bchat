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

## BUG-002: LanceDB Empty Vector Index Creation

**Status:** Resolved
**Date:** 2026-01-27
**Related Documents:**
- [DOCS_LANCE_EMBEDDING.MD](./DOCS_LANCE_EMBEDDING.MD) - Adapted embedding techniques
- [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD) - LanceDB implementation plan

### Error

```
WARN Failed to create vector index error="failed to create index: lance error: Not supported:
Creating empty vector indices with train=False is not yet implemented.
Index 'idx_embedding' for column 'embedding' cannot be created without training."
```

### Root Cause

1. IVF-PQ (Inverted File with Product Quantization) indexes require **training data**
2. The algorithm needs actual vectors to:
   - Learn cluster centroids (IVF part)
   - Build product quantization codebook (PQ part)
3. bchat was creating indexes immediately after table creation (when table is empty)
4. LanceDB cannot create IVF-PQ index on empty table (`train=False` not implemented)

### Previous Behavior

```go
// In ensureTable() - Created indexes on EMPTY table
table, err := db.conn.CreateTable(ctx, lanceTableName, schema)
// ...
if err := db.createIndexes(ctx); err != nil {
    slog.Warn("Failed to create indexes", "error", err)
}
```

### Solution: Deferred Index Creation

**Strategy:**
1. Skip IVF-PQ vector index creation in `createIndexes()` (only create BTree and FTS indexes)
2. Defer vector index creation to first `Insert()` operation via new `ensureVectorIndex()` method
3. Track index state with `hasVectorIndex` field to avoid redundant creation attempts

### Files Modified

| File | Change |
|------|--------|
| `server/router/api/v1/agent/vectordb_lance.go` | Added `hasVectorIndex` field, modified `createIndexes()`, added `ensureVectorIndex()`, call from `Insert()` |

### Code Changes

**1. Added state tracking:**
```go
type LanceVectorDB struct {
    // ...existing fields...
    hasVectorIndex bool // Track if IVF-PQ index has been created
}
```

**2. Modified createIndexes() - Skip IVF-PQ:**
```go
func (db *LanceVectorDB) createIndexes(ctx context.Context) error {
    // Skip IVF-PQ vector index on empty table - requires training data
    // Will be created on first Insert() via ensureVectorIndex()

    // BTree and FTS indexes work on empty tables
    // ...
}
```

**3. Added ensureVectorIndex():**
```go
func (db *LanceVectorDB) ensureVectorIndex(ctx context.Context) error {
    if db.hasVectorIndex {
        return nil
    }

    if err := db.table.CreateIndexWithName(ctx, []string{"embedding"},
        contracts.IndexTypeIvfPq, "idx_embedding"); err != nil {
        if strings.Contains(err.Error(), "already exists") {
            db.hasVectorIndex = true
            return nil
        }
        return fmt.Errorf("failed to create vector index: %w", err)
    }

    db.hasVectorIndex = true
    slog.Info("Created IVF-PQ vector index", "table", lanceTableName)
    return nil
}
```

**4. Called from Insert():**
```go
func (db *LanceVectorDB) Insert(ctx context.Context, chunks []DocumentChunk) error {
    // ...add data to table...

    // Create IVF-PQ vector index now that we have data
    if err := db.ensureVectorIndex(ctx); err != nil {
        slog.Warn("Failed to create vector index after insert", "error", err)
    }
}
```

### Verification

1. Delete existing LanceDB data: `rm -rf build/data/lancedb/`
2. Start server: `task run:rag:cybertron`
3. Check logs - should NOT see "Failed to create vector index" warning on startup
4. Upload KB content and click "Rebuild Index"
5. Check logs - should see "Created IVF-PQ vector index" after insert
6. Verify search works correctly

### Behavior After Fix

1. Table creation no longer attempts IVF-PQ index (no warning)
2. BTree and FTS indexes created on empty table (still work)
3. First `Insert()` operation creates IVF-PQ index after data is added
4. Subsequent inserts skip index creation (already exists)
5. Table recreation (dimension mismatch) resets `hasVectorIndex` flag

### Key Lesson

IVF-PQ indexes are **trained** indexes - they learn cluster centroids and codebooks from actual data. Unlike BTree or FTS indexes, they cannot be created on empty tables. Defer creation to first data insertion.

---

*Document Version: 1.1*
*Created: 2026-01-26*
*Updated: 2026-01-27*
