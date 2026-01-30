# Bug Report: Rebuild Index Timeout Issue

**Date:** 2026-01-30
**Status:** Fixed
**Severity:** High (Feature blocking)

---

## Summary

When running "Rebuild Index" in Agent Admin for large KB content, embedding requests to OpenRouter would timeout after 60 seconds, causing the indexing to fail.

---

## Error Message

```
failed to insert chunks: failed to generate embeddings for batch 1: failed to read response body: context deadline exceeded (Client.Timeout or context cancellation while reading body)
```

**Timeline from logs:**
- 23:35:12 - Batch 1 processing started (100 chunks)
- 23:36:12 - Timeout error (exactly 60 seconds later)

---

## Root Cause

1. **Hardcoded 60-second timeout** was insufficient for large batches
2. **Batch size of 100 chunks** created requests that took >60 seconds
3. **No retry logic** - single timeout failure killed entire reindex
4. **No timeout configurability** via environment variables

**Call chain:**
```
handlers.indexContentForRAG()
  └─> vectorDB.Insert(ctx, allChunks)
      └─> vectordb_lance.LanceVectorDB.Insert()
          └─> db.embedSvc.Embed(ctx, textsToEmbed)
              └─> openRouterEmbedding.Embed()
                  └─> io.ReadAll(resp.Body) ← TIMEOUT HERE
```

---

## Fixes Applied

### 1. Increased Timeout (60s → 180s) and Made Configurable

**File:** `server/router/api/v1/agent/embedding.go`

```go
// BEFORE
client: &http.Client{Timeout: 60 * time.Second},

// AFTER - Configurable via EMBEDDING_TIMEOUT env var
timeout := getEnvDuration("EMBEDDING_TIMEOUT", 180*time.Second)
client: &http.Client{Timeout: timeout},
```

### 2. Reduced Batch Size (100 → 25)

**File:** `server/router/api/v1/agent/vectordb_lance.go`

```go
// BEFORE
const batchSize = 100

// AFTER - Smaller batches = faster individual requests
const batchSize = 25
```

**Impact:** 1220 chunks now requires 49 batches instead of 13, but each batch completes much faster and is less likely to timeout.

### 3. Added Retry Logic with Exponential Backoff

**File:** `server/router/api/v1/agent/embedding.go`

```go
func (e *OpenRouterEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    var lastErr error
    maxRetries := 3

    for attempt := 0; attempt < maxRetries; attempt++ {
        if attempt > 0 {
            backoff := time.Duration(attempt*attempt) * time.Second
            slog.Info("Retrying embedding request", "attempt", attempt+1, "backoff", backoff)
            time.Sleep(backoff)
        }

        embeddings, err := e.doEmbed(ctx, texts)
        if err == nil {
            return embeddings, nil
        }
        lastErr = err

        // Only retry on timeout/network errors, not API errors
        if !isRetryableError(err) {
            return nil, err
        }
    }
    return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}
```

**Retry conditions:**
- Timeout errors
- Deadline exceeded
- Connection refused
- Connection reset
- DNS resolution failures

---

## New Environment Variable

| Variable | Default | Description |
|----------|---------|-------------|
| `EMBEDDING_TIMEOUT` | `180s` | HTTP timeout for embedding API requests |

**Examples:**
```bash
EMBEDDING_TIMEOUT=180s   # 3 minutes (default)
EMBEDDING_TIMEOUT=5m     # 5 minutes
EMBEDDING_TIMEOUT=60s    # 1 minute (original behavior)
```

---

## Files Modified

| File | Changes |
|------|---------|
| `server/router/api/v1/agent/embedding.go` | Configurable timeout, retry logic, `getEnvDuration` helper, `isRetryableError` helper |
| `server/router/api/v1/agent/vectordb_lance.go` | Reduced batch size from 100 to 25 |

---

## Verification

1. **Build verification:** `go build -tags rag ./bin/memos/main.go` - Passes
2. **Run Rebuild Index** on tenant with large KB content
3. **Expected log output:**
   ```
   Starting batched insert totalChunks=1220 batchSize=25
   Processing batch batch=1 totalBatches=49 chunksInBatch=25 progress=25/1220
   Processing batch batch=2 totalBatches=49 chunksInBatch=25 progress=50/1220
   ...
   ```
4. **Test retry logic:** Set `EMBEDDING_TIMEOUT=5s` to force timeout, verify retries occur

---

## Performance Impact

| Metric | Before | After |
|--------|--------|-------|
| Batch size | 100 chunks | 25 chunks |
| Batches for 1220 chunks | 13 | 49 |
| Timeout per request | 60s | 180s |
| Retry attempts | 0 | 3 |
| Single batch time (est.) | 45-90s | 10-25s |

Smaller batches complete faster and are more resilient to transient network issues.

---

## Related Documentation

- `docs/DOCS_ENV_VAR.MD` - Environment variables reference
- `docs/DOCS_RAG_PIPELINE.MD` - RAG pipeline architecture
