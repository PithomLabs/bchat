# Plan: LanceDB PQ Index Minimum-Rows Warning

**Date:** 2026-08-01  
**Status:** Ready for implementation  
**Scope:** LanceDB vector index creation in `server/router/api/v1/agent/vectordb_lance.go`  
**Trigger:** Log noise / non-fatal warning during small RAG indexing

---

## Background Context

### Observed Behavior

```
2026/08/01 07:12:46 INFO Starting batched insert totalChunks=1 batchSize=200 table=kb_documents_1536
2026/08/01 07:12:46 INFO Processing batch batch=1 totalBatches=1 chunksInBatch=1 progress=1/1
2026/08/01 07:12:47 WARN Failed to create vector index after insert error="failed to create vector index: failed to create index: Failed to create index: lance error: LanceError(Index): Not enough rows to train PQ. Requires 256 rows but only 6 available, /home/runner/.cargo/registry/src/index.crates.io-1949cf8c6b5b557f/lance-index-0.37.0/src/vector/pq/builder.rs:180:27"
2026/08/01 07:12:47 INFO Completed batched insert totalChunks=1
```

Ticket RAG indexing inserted 6 chunks and then failed to create the IVF-PQ vector index. The warning is non-fatal: the insert succeeded, and search falls back to sequential scan without an index.

### Root Cause Investigation

**File:** `server/router/api/v1/agent/vectordb_lance.go`

The index creation path:

1. Table creation (`ensureTable`, line 142) calls `createIndexes` (line 181)
2. `createIndexes` (line 224) explicitly skips IVF-PQ because an empty table cannot train PQ
3. After `Insert()` adds data, `ensureVectorIndex` (line 249) is called at line 482
4. `ensureVectorIndex` creates `contracts.IndexTypeIvfPq` (line 255)

**PQ training requirement:** LanceDB's `lance-index-0.37.0` Rust crate trains **256 PQ centroids** (`2^8`) per sub-vector in `builder.rs:180`. This is a hard minimum independent of IVF `num_partitions`. Changing `num_partitions` does not reduce the PQ centroid requirement.

**Current handling:** The warning is logged but not propagated as an error. Search continues to work without the index.

### Can it have fewer than 256 rows?

Not with **IVF-PQ**. Three alternative approaches exist:

| Option | Mechanism | Pros | Cons |
|--------|-----------|------|------|
| **Keep IVF-PQ, skip index when too small** | Current behavior | Safe, no code change | Log noise persists for small datasets |
| **Switch to IVF-Flat** (`IndexTypeIvfFlat`) | No PQ compression; stores full vectors | Works with fewer rows; no PQ training | Higher disk/memory usage; still needs enough rows for IVF partitioning |
| **Use Auto index** (`IndexTypeAuto`) | LanceDB picks based on data size | Adapts automatically | Less control; may still pick IVF-PQ for small data |

**Relevant LanceDB index types available** (`lancedb-go v0.1.2/pkg/contracts`):
- `IndexTypeAuto`
- `IndexTypeIvfPq` (current)
- `IndexTypeIvfFlat`
- `IndexTypeHnswPq`
- `IndexTypeHnswSq`
- `IndexTypeBTree`
- `IndexTypeBitmap`
- `IndexTypeLabelList`
- `IndexTypeFts`

---

## Decision Required

**Question:** How should the plan handle the PQ minimum-rows warning?

**Recommended approach:** Implement a row-count guard before attempting IVF-PQ index creation:

1. Check table row count via `db.table.Count(ctx)`
2. If rows < 256, skip IVF-PQ creation and log at `Debug` level instead of `Warn`
3. If rows >= 256, create IVF-PQ as before
4. Add a follow-up task to evaluate `IndexTypeAuto` or `IndexTypeIvfFlat` for small datasets in a future iteration

**Rationale:**
- Minimal blast radius: only changes the warning behavior, not index semantics
- Avoids log pollution during normal small-ticket indexing
- Defers the broader "which index type for which dataset size" question to a separate design decision
- Maintains correctness: search works without index, just slower

---

## Proposed Implementation

### File: `server/router/api/v1/agent/vectordb_lance.go`

**Change `ensureVectorIndex` to guard on row count:**

```go
func (db *LanceVectorDB) ensureVectorIndex(ctx context.Context) error {
    if db.hasVectorIndex {
        return nil
    }

    // IVF-PQ requires at least 256 rows for PQ codebook training.
    // Skip index creation for small datasets; search falls back to sequential scan.
    count, err := db.table.Count(ctx)
    if err != nil {
        return fmt.Errorf("failed to get table count: %w", err)
    }
    if count < 256 {
        slog.Debug("Skipping IVF-PQ index: not enough rows for training",
            "table", db.tableName, "count", count, "required", 256)
        return nil
    }

    // Create IVF-PQ vector index now that we have data for training
    if err := db.table.CreateIndexWithName(ctx, []string{"embedding"}, contracts.IndexTypeIvfPq, "idx_embedding"); err != nil {
        // Check if index already exists (not an error)
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

**Call sites remain unchanged** (lines 482, 598) — they already handle non-fatal warnings.

---

## Files to Modify

| File | Change |
|------|--------|
| `server/router/api/v1/agent/vectordb_lance.go` | Add row-count guard in `ensureVectorIndex`; downgrade insufficient-data log from `Warn` to `Debug` |

---

## Tests

No new tests required. Existing behavior is preserved:
- Insert succeeds regardless of index creation
- Search works without index (sequential scan)
- Warning becomes a debug log for small datasets

**Verification:**
```bash
go test -v -run TestVectorDB ./server/router/api/v1/agent/ -count=1
go test -v ./server/router/api/v1/agent/ -count=1
```

---

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Index never created if table hovers near 256 rows | Low | Medium | Index is re-attempted on every `Insert()` call after checkpoint; once threshold is crossed, index will be created |
| `Count()` adds latency on small inserts | Low | Low | Count is only called during `ensureVectorIndex`, which already acquires `db.mu.Lock()`; single `COUNT(*)` query is fast |
| Debug log still noisy in dev | Low | Low | Dev environments can set log level to `Debug` explicitly; production stays at `Info`/`Warn` |

---

## Open Questions (Out of Scope for This Plan)

1. Should we evaluate `IndexTypeAuto` or `IndexTypeIvfFlat` for datasets that consistently stay below 256 rows?
2. Should there be a configurable threshold via env var (e.g., `LANCE_MIN_INDEX_ROWS`)?
3. Should we expose index status via metrics/health endpoint?

These are deferred to a follow-up RAG infrastructure plan.
