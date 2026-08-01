# Implementation: LanceDB PQ Index Minimum-Rows Warning

**Date:** 2026-08-01  
**Status:** Implemented  
**Files changed:**
- `server/router/api/v1/agent/vectordb_lance.go`
- `server/router/api/v1/agent/vectordb_lance_test.go`

---

## Problem Statement

During ticket RAG indexing, LanceDB logged a non-fatal but noisy warning when inserting small datasets:

```
WARN Failed to create vector index after insert error="failed to create vector index: ... LanceError(Index): Not enough rows to train PQ. Requires 256 rows but only 6 available ... lance-index-0.37.0/src/vector/pq/builder.rs:180:27"
```

The insert succeeded and search remained functional, but the warning polluted logs for normal small-ticket indexing.

---

## Root Cause

**File:** `server/router/api/v1/agent/vectordb_lance.go`

The index creation path:

1. `ensureTable` creates the table and calls `createIndexes`
2. `createIndexes` skips IVF-PQ on empty tables because PQ needs training data
3. After `Insert()` adds rows, `ensureVectorIndex` is called
4. `ensureVectorIndex` unconditionally creates an `IndexTypeIvfPq` index

LanceDB's `lance-index-0.37.0` Rust crate trains **256 PQ centroids** (`2^8`) per sub-vector in `pq/builder.rs:180`. This is a hard minimum for PQ codebook training and is independent of IVF `num_partitions`. Reducing `num_partitions` does not reduce the PQ centroid requirement.

With fewer than 256 rows, `CreateIndexWithName` returns the LanceError shown above. The existing call sites at lines 482 and 598 catch this as a non-fatal warning, so the system stays functional but logs are noisy.

---

## Decision

Rather than switching index types or adding configuration, the minimal-safe fix is to **skip IVF-PQ creation when the table has fewer than 256 rows**. Search continues without an index via sequential scan, which is acceptable for small datasets.

---

## Implementation

### Constant

```go
const minIVFPQIndexRows int64 = 256
```

Added next to `legacyTableName` in `vectordb_lance.go`. This avoids magic numbers in multiple places and keeps the comparison typed correctly against `db.table.Count(ctx)`, which returns `int64`.

### `ensureVectorIndex` guard

```go
func (db *LanceVectorDB) ensureVectorIndex(ctx context.Context) error {
    if db.hasVectorIndex {
        return nil
    }

    count, err := db.table.Count(ctx)
    if err != nil {
        return fmt.Errorf("failed to get table count: %w", err)
    }
    if count < minIVFPQIndexRows {
        slog.Debug("Skipping IVF-PQ index: not enough rows for training",
            "table", db.tableName, "count", count, "required", minIVFPQIndexRows)
        return nil
    }

    if err := db.table.CreateIndexWithName(ctx, []string{"embedding"}, contracts.IndexTypeIvfPq, "idx_embedding"); err != nil {
        if strings.Contains(err.Error(), "already exists") {
            db.hasVectorIndex = true
            return nil
        }
        return fmt.Errorf("failed to create vector index: %w", err)
    }

    db.hasVectorIndex = true
    slog.Info("Created IVF-PQ vector index", "table", db.tableName)
    return nil
}
```

Key behaviors:
- **Success path unchanged** when rows >= 256
- **Small-dataset path** returns `nil` without setting `hasVectorIndex`
- **Count failures** are returned as real errors so callers still warn on table/connection trouble
- `hasVectorIndex` intentionally remains `false` so subsequent `Insert()` calls retry once the threshold is crossed

### Call sites

No changes needed. Both `Insert` (line 482) and `InsertWithCheckpoint` (line 598) already treat `ensureVectorIndex` failure as non-fatal:

```go
if err := db.ensureVectorIndex(ctx); err != nil {
    slog.Warn("Failed to create vector index after insert", "error", err)
}
```

After this change, `ensureVectorIndex` returns `nil` for small datasets, so no warning is logged.

---

## Test

**File:** `server/router/api/v1/agent/vectordb_lance_test.go`

Added `TestLanceVectorDB_Integration_SmallDatasetNoIndex` under the existing `//go:build rag && integration` tag.

It:
1. Creates a LanceDB instance with mock embeddings
2. Inserts 3 chunks, which is below the 256-row threshold
3. Asserts `Insert` succeeds
4. Asserts `Stats` reports 3 chunks
5. Asserts `Search` still returns results without a vector index

Integration tests require native LanceDB libraries (`LD_LIBRARY_PATH`), so this test is correctly gated and not run in standard CI.

---

## Verification

```bash
go build ./server/router/api/v1/agent/...
go vet ./server/router/api/v1/agent/... ./server/router/api/v1/...
go test -v ./server/router/api/v1/agent/ -count=1
go test -v ./v1 ./server/router/api/v1/ -count=1
```

All pass. The integration LanceDB test is gated behind build tags and was not executed.

---

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Index never created if table hovers near 256 rows | Low | Medium | `ensureVectorIndex` retries on every `Insert()`; once threshold is crossed, index creation succeeds |
| `Count()` adds latency | Low | Low | Called under existing `db.mu.Lock()`; single `COUNT(*)` is fast |
| Debug log still noisy in dev | Low | Low | Dev environments can enable `Debug` explicitly; production stays at `Info`/`Warn` |

---

## Out of Scope

1. Evaluating `IndexTypeAuto` or `IndexTypeIvfFlat` for consistently small datasets
2. Configurable threshold via env var
3. Exposing index status via metrics/health endpoint

These are deferred to a follow-up RAG infrastructure plan.
