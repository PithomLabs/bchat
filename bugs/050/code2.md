# Code2: Adversarial Review Fixes

**Date:** 2026-07-29
**Source:** code.md §6 adversarial review prompt → verified against codebase
**Status:** Plan only — awaiting implementation

---

## Review Results Summary

| Severity | PASS | FAIL | Total |
|----------|------|------|-------|
| CRITICAL | 4 | 0 | 4 |
| HIGH | 1 | 3 | 4 |
| MEDIUM | 4 | 0 | 4 |
| NIT | 3 | 0 | 3 |
| **Total** | **12** | **3** | **15** |

---

## Findings to Fix (3)

### [H-1] Insert() marshals nil embedding to `"null"`

**File:** `vectordb_cockroach.go:175-194`, `198-224`
**Severity:** HIGH

**Problem:**
```go
embeddingJSON, err := json.Marshal(chunk.Embedding)  // nil → "null"
// ...
VALUES ($1, $2, $3, $4, $5, $6::VECTOR, $7, $8, $9) // null::VECTOR fails
```

If `chunk.Embedding` is nil or empty, `json.Marshal(nil)` produces `"null"`, and `null::VECTOR` fails at the CockroachDB level. The `ticket_embedder.go` always generates embeddings before calling Insert, so this is not triggered in that path, but the Insert API does not defensively handle it.

**Fix:**
Skip embedding column when nil; let the row exist without an embedding (for pre-embedded chunks or metadata-only inserts).

```go
// In Insert() and InsertWithCheckpoint(), replace the inner func:
var embeddingValue interface{} = chunk.Embedding
if len(chunk.Embedding) == 0 {
    embeddingValue = nil  // NULL embedding — row exists but not searchable by vector
}

err = crdb.ExecuteTx(ctx, v.db, nil, func(tx *sql.Tx) error {
    _, err := tx.ExecContext(ctx, `
        UPSERT INTO agent_vectors (id, tenant_id, content_type, title, content, embedding, metadata, source_version, created_at)
        VALUES ($1, $2, $3, $4, $5, $6::VECTOR, $7, $8, $9)
    `, chunk.ID, chunk.TenantID, chunk.ContentType, chunk.Title, chunk.Content,
        embeddingValue, "{}", chunk.SourceVersion, time.Now())
    return err
})
```

**Risk:** Rows with NULL embeddings will not appear in vector search results (CockroachDB returns NULL for `embedding <=> NULL`). This is correct behavior — only embedded rows are searchable.

**Lines to edit:**
- `vectordb_cockroach.go:175-194` (Insert method)
- `vectordb_cockroach.go:198-224` (InsertWithCheckpoint method)

---

### [H-2] Search() embeds empty string without validation

**File:** `vectordb_cockroach.go:273-281`
**Severity:** HIGH

**Problem:**
```go
func (v *CockroachVectorDB) Search(ctx context.Context, query SearchQuery) (*SearchResult, error) {
    start := time.Now()

    // 1. Generate embedding for query text
    embeddings, err := v.embedSvc.Embed(ctx, []string{query.QueryText})  // empty string → ???
```

If `query.QueryText` is empty and `query.QueryEmbedding` is nil, the method calls `Embed("")` which may return a zero-dimension embedding, error, or undefined behavior depending on the provider. The `MemoryVectorDB.Search()` in `vectordb.go` also lacks this validation.

**Fix:**
Add early return guard at the top of Search():

```go
func (v *CockroachVectorDB) Search(ctx context.Context, query SearchQuery) (*SearchResult, error) {
    start := time.Now()

    // Validate: must have either query text or pre-computed embedding
    if query.QueryText == "" && len(query.QueryEmbedding) == 0 {
        return &SearchResult{
            Chunks:  []DocumentChunk{},
            Scores:  []float64{},
            Total:   0,
            Latency: time.Since(start),
        }, nil
    }
    // ... rest of method
```

**Lines to edit:**
- `vectordb_cockroach.go:273-281` (Search method)

---

### [H-4] EscalateTicket has no description length/character validation

**File:** `handlers.go:484-486`, `service.go:5535-5538`
**Severity:** HIGH

**Problem:**
```go
// handlers.go:484-486
if req.Description == "" {
    return echo.NewHTTPError(http.StatusBadRequest, "description is required")
}

// service.go:5535-5538
description := req.Description
if len(description) < 3 || description[:3] != "/m/" {
    description = "/m/" + description
}
```

Only validates non-empty. No length limits, no character restrictions. A 10MB description string would be inserted into the database and embedded, causing performance issues and potential memory exhaustion.

**Fix:**
Add length validation in the handler (before calling service):

```go
// handlers.go, after line 486
if len(req.Description) > 10000 {
    return echo.NewHTTPError(http.StatusBadRequest, "description must be under 10000 characters")
}
```

**Lines to edit:**
- `handlers.go:486` (add length check after empty check)

---

## Findings Verified as PASS (12)

| Item | Verdict | Evidence |
|------|---------|----------|
| [C-1] | PASS | `vectordb_cockroach.go:118` uses `*pgconn.PgError`, comment at line 117 documents ban |
| [C-2] | PASS | `vectordb_cockroach.go:109` documents `IF NOT EXISTS` unsupported; lines 115-127 catch `42P07` |
| [C-3] | PASS | `handlers.go:492` returns sanitized `"Escalation service unavailable"` to client |
| [C-4] | PASS | `handlers.go:469-473` extracts tenant; `service.go:5546` sets `TenantID: &tenant.ID` |
| [H-3] | PASS | `service.go:196-198` uses `context.WithTimeout(context.Background(), 2*time.Minute)` |
| [M-1] | PASS | `service.go:160-162` type-asserts `*CockroachVectorDB` before calling `SetDB` |
| [M-2] | PASS | `vectordb_cockroach.go:115-127` handles `42P07` as success; non-duplicate logged as warning |
| [M-3] | PASS | `Dockerfile.ecs:37` exposes port 8081; healthcheck uses `localhost:8081` |
| [M-4] | PASS | `Taskfile.yml:241` uses inline `VAR=value ./binary` (not `env:` block) |
| [N-1] | PASS | All files use stdlib → third-party → internal grouping |
| [N-2] | PASS | All errors use `fmt.Errorf("...: %w", err)` wrapping |
| [N-3] | PASS | All logging uses `log/slog` with structured key-value pairs |

---

## Implementation Order

| Step | File | Change | Est. Lines |
|------|------|--------|------------|
| 1 | `vectordb_cockroach.go` | Fix nil embedding in `Insert()` and `InsertWithCheckpoint()` | +6 |
| 2 | `vectordb_cockroach.go` | Add empty query guard in `Search()` | +8 |
| 3 | `handlers.go` | Add description length validation in `HandleEscalateTicket()` | +3 |
| 4 | `ticket_resolution_test.go` | Add tests for nil embedding and empty query | +20 |
| 5 | Verify both builds compile | `go build -tags cockroach ./bin/memos/...` and `go build ./bin/memos/...` | — |
| 6 | Run tests | `go test -tags cockroach ./server/router/api/v1/agent/... -v` | — |

---

## Risk Assessment

| Fix | Risk | Mitigation |
|-----|------|------------|
| [H-1] nil embedding | LOW | NULL embeddings are valid SQL; vector search skips NULLs naturally |
| [H-2] empty query | LOW | Early return with empty result; no side effects |
| [H-4] description length | LOW | Hard limit prevents memory exhaustion; 10K chars is generous for ticket descriptions |
