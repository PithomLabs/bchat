# Code4: Post-Implementation Review Fixes

**Date:** 2026-07-29
**Source:** code3_imp_review.md
**Status:** Plan only — awaiting implementation

---

## Review Results

| Source | CRITICAL | HIGH | MEDIUM | NIT | Total |
|--------|----------|------|--------|-----|-------|
| code3_imp_review.md | 0 | 1 | 0 | 2 | 3 |

---

## Findings to Fix

### H-1. Search() Ignores QueryEmbedding — Diverges from LanceDB Pattern

**File:** `vectordb_cockroach.go:279-295`
**Severity:** HIGH (latent correctness bug)

**Problem:**
```go
// Current code (line 289):
embeddings, err := v.embedSvc.Embed(ctx, []string{query.QueryText})
// ...
queryEmbedding := embeddings[0]
```

Unconditionally embeds `QueryText` — ignores pre-computed `QueryEmbedding` field. If caller provides `QueryEmbedding` without `QueryText`, embeds empty string → garbage vector. If caller provides both, `QueryEmbedding` silently ignored.

**LanceDB pattern (`vectordb_lance.go:1108-1119`):**
```go
if len(query.QueryEmbedding) > 0 {
    queryEmbedding = query.QueryEmbedding
} else if query.QueryText != "" {
    embeddings, err := db.embedSvc.Embed(...)
    queryEmbedding = embeddings[0]
} else {
    return error
}
```

**Fix:** Match LanceDB pattern:
```go
// Replace lines 289-295:
var queryEmbedding []float32
if len(query.QueryEmbedding) > 0 {
    queryEmbedding = query.QueryEmbedding
} else if query.QueryText != "" {
    embeddings, err := v.embedSvc.Embed(ctx, []string{query.QueryText})
    if err != nil {
        return nil, fmt.Errorf("failed to embed query: %w", err)
    }
    queryEmbedding = embeddings[0]
} else {
    return &SearchResult{
        Chunks:  []DocumentChunk{},
        Scores:  []float64{},
        Total:   0,
        Latency: time.Since(start),
    }, nil
}
```

**Lines to edit:** `vectordb_cockroach.go:289-295`

---

### N-1. MinScore Not Implemented

**File:** `vectordb_cockroach.go:311-345`
**Severity:** NIT (feature gap)

**Problem:** `query.MinScore` field exists on `SearchQuery` (defined at `vectordb.go:167`) but is completely ignored in `CockroachVectorDB.Search()`. All results returned regardless of similarity score. LanceDB (`vectordb_lance.go:1159,1163`) and in-memory (`vectordb.go:589,607`) both filter by MinScore.

**Fix:** Add SQL WHERE clause or post-query filtering:
```go
// Option A: Add to SQL WHERE clause (line 311-318):
sqlQuery := fmt.Sprintf(`
    SELECT id, title, content, content_type, metadata, source_version, created_at,
           1 - (embedding <=> $1::VECTOR) AS similarity
    FROM agent_vectors
    WHERE tenant_id = $2 AND content_type IN (%s)
      AND (1 - (embedding <=> $1::VECTOR)) >= $4
    ORDER BY embedding <=> $1::VECTOR
    LIMIT $3
`, contentTypeFilter)

// Update args (line 321):
rows, err := v.db.QueryContext(ctx, sqlQuery, embeddingJSON, query.TenantID, query.TopK, query.MinScore)
```

**Lines to edit:** `vectordb_cockroach.go:311-321`

---

### N-2. Integration Tests Are Stubs

**File:** `ticket_resolution_test.go`
**Severity:** NIT (zero test coverage)

**Problem:** All 4 tests create `&Service{vectorDB, vectorDBConfig}` without a `store` (nil). None call the methods under test. Would nil-dereference at `ticket_embedder.go:34` (`s.store.ListAgentTenants(...)`) if called.

**Fix:** Two options:

**Option A: Skip with explanation (fast, for hackathon):**
```go
func TestProcessPendingTickets(t *testing.T) {
    t.Skip("Requires real CockroachDB + store — see integration test suite")
}
```

**Option B: Write real tests with mock store (proper, ~50 lines):**
```go
// Use a mock store or testify mock to satisfy the interface
// Call actual methods and assert results
```

**Recommendation:** Option A for hackathon deadline. Option B post-demo.

**Lines to edit:** `ticket_resolution_test.go:36-46, 73-83, 110-120, 147-156`

---

## Implementation Order

| Step | Fix | File | Est. Lines |
|------|-----|------|------------|
| 1 | H-1: Rewrite Search() to match LanceDB pattern | `vectordb_cockroach.go` | +12/-7 |
| 2 | N-1: Add MinScore filtering | `vectordb_cockroach.go` | +3 |
| 3 | N-2: Replace stub tests with t.Skip() | `ticket_resolution_test.go` | +4/-40 |
| 4 | Verify builds | `go build` commands | — |
| 5 | Run tests | `go test` commands | — |

**Total estimated lines changed:** ~20 lines

---

## Build Verification

```bash
# Both builds must compile
go build -tags cockroach ./bin/memos/...
go build ./bin/memos/...

# Run tests
go test -tags cockroach ./server/router/api/v1/agent/... -v
```

---

## Risk Assessment

| Fix | Risk | Mitigation |
|-----|------|------------|
| H-1: QueryEmbedding priority | LOW | Matches established LanceDB pattern; no callers currently use QueryEmbedding |
| N-1: MinScore filtering | LOW | SQL WHERE clause is straightforward; fallback to brute-force if index missing |
| N-2: Test stubs → t.Skip | LOW | No behavior change; just documents the skip reason |
