# Plan 037 — Fix Recurring Reindex Stuck Issue (Full Architectural Fix)

## Problem / Context

When triggering "Rebuild Index" for a tenant (e.g., evpn), the reindex appears stuck —
the API request never returns, no progress is visible, and the process may hang
indefinitely. This is a recurring issue documented in `docs/docs_env_default.md`.

The env var root cause (`EMBEDDING_BATCH_SIZE=1` + `EMBEDDING_TIMEOUT=10m`) was identified
previously, but the **underlying architectural issues** cause reindex to appear stuck even
with correct env vars.

### Goal

1. Make reindex non-blocking so the HTTP request returns immediately
2. Ensure progress is visible via the existing polling UI
3. Make all retry/sleep logic context-aware (respect client disconnect)
4. Create checkpoint BEFORE Validate() so resume always works
5. Normalize env var defaults per-context (local vs production)

## Current-state gaps (grounded in code)

| # | Area | Location | Issue |
|---|------|----------|-------|
| 1 | Handler | `handlers.go:1193` | POST reindex blocks synchronously — no server timeout, request hangs indefinitely |
| 2 | Service | `service.go:753-757` | `Validate()` runs BEFORE checkpoint creation — if API is slow, no checkpoint exists, `resume=true` can't help |
| 3 | Embedding | `embedding.go:681-683` | `isRetryableError()` returns `true` for `context.Canceled` — goroutine retries up to 10x after client disconnect |
| 4 | Embedding | `embedding.go:420` | `time.Sleep(backoff)` ignores context — continues sleeping even after cancel |
| 5 | VectorDB | `vectordb_lance.go:498` | `db.mu.Lock()` held for entire batch loop — blocks all LanceDB operations |
| 6 | VectorDB | `vectordb_lance.go:588` | `time.Sleep(delay)` in retry ignores context |
| 7 | Server | `server.go:50` | Echo server created with no `HandlerTimeout` — no safety net |

### What already works (no changes needed)

- Frontend progress UI: progress bar, chunk counts, status label (`AgentAdmin.tsx:1234-1294`)
- `GET /:slug/reindex/status` polling endpoint (`handlers.go:1222-1254`)
- `ReindexCheckpoint` DB model with `current_batch`, `total_batches`, `processed_chunks`
- 3s polling interval in frontend (`AgentAdmin.tsx:240-263`)
- `isRebuilding` flag and status-driven UI updates

## Proposed changes

### Change 1: Make reindex non-blocking

**File:** `server/router/api/v1/agent/handlers.go:1157-1206`

Replace the synchronous call with a background goroutine + immediate 202 response:

```go
func (h *Handler) HandleReindexTenant(c echo.Context) error {
    ctx := c.Request().Context()
    slug := c.Param("slug")

    tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
    if err != nil || tenant == nil {
        return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
    }

    if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermAPIConfig) {
        return echo.NewHTTPError(http.StatusForbidden, "Permission denied")
    }

    tenantConfig, _ := h.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &tenant.ID})
    if tenantConfig != nil && tenantConfig.RetrievalMode == "long_context" {
        return c.JSON(http.StatusOK, map[string]interface{}{
            "success": true, "message": "Skipped - long_context mode", "chunks": 0,
        })
    }

    audienceType := c.QueryParam("audience_type")
    if audienceType == "" {
        audienceType = "all"
    }
    resume := c.QueryParam("resume") == "true"

    // Detached context: survives client disconnect, bounded to 30 minutes
    reindexCtx, reindexCancel := context.WithTimeout(context.Background(), 30*time.Minute)
    go func() {
        defer reindexCancel()
        chunks, reindexErr := h.service.ReindexTenantContentWithResume(reindexCtx, tenant.ID, audienceType, resume)
        if reindexErr != nil {
            slog.Error("reindex failed", "tenantID", tenant.ID, "audience", audienceType, "error", reindexErr)
        } else {
            slog.Info("reindex completed", "tenantID", tenant.ID, "chunks", chunks)
        }
    }()

    return c.JSON(http.StatusAccepted, map[string]interface{}{
        "success":  true,
        "message":  "Reindex started",
        "audience": audienceType,
        "resumed":  resume,
    })
}
```

**Impact:** Frontend already handles this — `handleRebuildIndex` sets `isRebuilding=true`,
the polling `useEffect` activates, and progress updates via `GET /:slug/reindex/status`.

### Change 2: Move Validate() after checkpoint creation

**File:** `server/router/api/v1/agent/service.go:709-970`

Reorder the function so checkpoint is created BEFORE Validate():

```
Current order:
  withTenantEmbeddingAPIKey → Validate() → Fetch files → Chunk → Create checkpoint → Batch embed

New order:
  withTenantEmbeddingAPIKey → Fetch files → Chunk → Create checkpoint → Validate() → Batch embed
```

Move lines 753-757 (Validate block) to after line 857 (after checkpoint creation).

**Why:** If Validate() hangs (preflight embedding call slow), the checkpoint already exists.
On `resume=true`, the checkpoint lookup at lines 728-743 will find it, and
`shouldValidateReindex()` (line 984) will return `false` — skipping validation entirely.

### Change 3: Context-aware retry in embedding

**File:** `server/router/api/v1/agent/embedding.go:401-436`

Replace `time.Sleep(backoff)` with context-aware select:

```go
for attempt := 0; attempt < maxRetries; attempt++ {
    if attempt > 0 {
        backoff := baseBackoff * time.Duration(1<<(attempt-1))
        if backoff > maxBackoff {
            backoff = maxBackoff
        }
        slog.Info("Retrying embedding request", "attempt", attempt+1, "backoff", backoff)
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-time.After(backoff):
        }
    }
    embeddings, err := e.doEmbed(ctx, texts)
    if err == nil {
        return embeddings, nil
    }
    lastErr = err
    if !isRetryableError(err) {
        return nil, err
    }
}
```

### Change 4: Don't retry on context.Canceled

**File:** `server/router/api/v1/agent/embedding.go:662-694`

Change `isRetryableError()`:

```go
func isRetryableError(err error) bool {
    if err == nil {
        return false
    }
    if errors.Is(err, ErrEmbeddingProviderMisconfigured) || errors.Is(err, ErrVectorStoreUnavailable) {
        return false
    }

    var httpErr *embeddingHTTPError
    if errors.As(err, &httpErr) {
        switch httpErr.statusCode {
        case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
            return true
        default:
            return false
        }
    }

    // Don't retry on explicit cancel (client disconnected)
    if errors.Is(err, context.Canceled) {
        return false
    }
    // Do retry on timeout (transient)
    if errors.Is(err, context.DeadlineExceeded) {
        return true
    }

    var netErr net.Error
    if errors.As(err, &netErr) {
        return true
    }
    var dnsErr *net.DNSError
    if errors.As(err, &dnsErr) {
        return true
    }

    return false
}
```

### Change 5: Per-batch mutex in InsertWithCheckpoint

**File:** `server/router/api/v1/agent/vectordb_lance.go:491-563`

Release mutex between batches and add context checks:

```go
func (db *LanceVectorDB) InsertWithCheckpoint(ctx context.Context, chunks []DocumentChunk, opts InsertOptions) error {
    if len(chunks) == 0 {
        return nil
    }

    if opts.MaxRetries == 0 {
        opts.MaxRetries = 3
    }
    if opts.RetryDelay == 0 {
        opts.RetryDelay = 5 * time.Second
    }

    batchSize := GetEmbeddingBatchSize()
    totalChunks := len(chunks)
    totalBatches := (totalChunks + batchSize - 1) / batchSize
    startBatch := opts.StartBatch

    slog.Info("Starting batched insert with checkpoint support",
        "totalChunks", totalChunks, "batchSize", batchSize,
        "startBatch", startBatch, "totalBatches", totalBatches)

    for batchNum := startBatch; batchNum < totalBatches; batchNum++ {
        // Check context before each batch
        if ctx.Err() != nil {
            return ctx.Err()
        }

        batchStart := batchNum * batchSize
        batchEnd := batchStart + batchSize
        if batchEnd > totalChunks {
            batchEnd = totalChunks
        }
        batch := chunks[batchStart:batchEnd]

        slog.Info("Processing batch",
            "batch", batchNum+1, "totalBatches", totalBatches,
            "chunksInBatch", len(batch),
            "progress", fmt.Sprintf("%d/%d", batchEnd, totalChunks))

        // Per-batch lock — releases between batches so searches can proceed
        db.mu.Lock()
        err := db.processBatchWithRetry(ctx, batch, batchNum+1, opts.MaxRetries, opts.RetryDelay)
        db.mu.Unlock()

        if err != nil {
            return fmt.Errorf("failed at batch %d: %w", batchNum+1, err)
        }

        if opts.CheckpointFunc != nil {
            if err := opts.CheckpointFunc(batchNum+1, batchEnd, totalBatches, totalChunks, len(batch)); err != nil {
                slog.Warn("Checkpoint callback failed", "batch", batchNum+1, "error", err)
            }
        }
    }

    // Index creation needs exclusive lock
    db.mu.Lock()
    defer db.mu.Unlock()
    if err := db.ensureVectorIndex(ctx); err != nil {
        slog.Warn("Failed to create vector index after insert", "error", err)
    }

    slog.Info("Completed batched insert with checkpoint", "totalChunks", totalChunks)
    return nil
}
```

### Change 6: Context-aware retry in processBatchWithRetry

**File:** `server/router/api/v1/agent/vectordb_lance.go:565-597`

```go
func (db *LanceVectorDB) processBatchWithRetry(ctx context.Context, batch []DocumentChunk, batchNum, maxRetries int, initialDelay time.Duration) error {
    var lastErr error
    delay := initialDelay
    maxDelay := 60 * time.Second

    for attempt := 0; attempt < maxRetries; attempt++ {
        err := db.processSingleBatch(ctx, batch, batchNum)
        if err == nil {
            return nil
        }

        lastErr = err
        if !isRetryableError(err) {
            return fmt.Errorf("batch %d failed with permanent error: %w", batchNum, err)
        }
        if attempt < maxRetries-1 {
            slog.Warn("Batch failed, retrying",
                "batch", batchNum, "attempt", attempt+1,
                "maxRetries", maxRetries, "delay", delay, "error", err)
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(delay):
            }
            delay = delay * 2
            if delay > maxDelay {
                delay = maxDelay
            }
        }
    }

    return fmt.Errorf("batch %d failed after %d retries: %w", batchNum, maxRetries, lastErr)
}
```

### Change 7: Add server HandlerTimeout (safety net)

**File:** `server/server.go:50-73`

Add a timeout to the Echo server as a safety net:

```go
echoServer := echo.New()
// ... existing config ...

// Safety net: 35-minute write timeout (reindex is the longest operation)
echoServer.Server.ReadTimeout = 0
echoServer.Server.WriteTimeout = 35 * time.Minute
echoServer.Server.IdleTimeout = 120 * time.Second
```

### Change 8: Document per-context env var defaults

Add a section to `docs/DOCS_ENV_VAR.MD` and comments in `.env` documenting the rationale
for different values in local vs production.

#### Local Development (`.env`)

```bash
# EMBEDDING_BATCH_SIZE: 200 (max throughput, no rate limits in dev)
# EMBEDDING_TIMEOUT: 180s (generous for slow API, but won't stack with batch=200)
# RAG_MAX_CHUNK_TOKENS: 4096 (large chunks = fewer API calls)
```

#### Production Fly.io (`fly.toml` / Dockerfiles)

```yaml
# EMBEDDING_BATCH_SIZE: 10 (conservative for OpenRouter rate limits + 1024MB VM memory)
# EMBEDDING_TIMEOUT: 10m (large tenants with 10K+ chunks need longer per-batch timeout)
# RAG_MAX_CHUNK_TOKENS: 4096 (same as local)
```

#### Rationale

| Setting | Local | Production | Why different |
|---------|-------|------------|---------------|
| `EMBEDDING_BATCH_SIZE` | 200 | 10 | Local has no rate limits; production shares OpenRouter quota across tenants and has 1024MB memory limit |
| `EMBEDDING_TIMEOUT` | 180s | 10m | Local batches are fast (200 chunks = ~1-2 API calls); production batches are smaller (10 chunks each) but large tenants may have 1000+ batches |
| `RAG_MAX_CHUNK_TOKENS` | 4096 | 4096 | Same — chunk size is a quality/accuracy tradeoff, not environment-dependent |
| `EMBEDDING_PROVIDER` | openrouter | openrouter | Same provider, same model |
| `LANCEDB_STORAGE_PROVIDER` | local | s3 | Local uses filesystem; production uses Tigrisdata S3 on fly.io |

## Files to modify (summary)

| File | Change |
|------|--------|
| `server/server.go:50-73` | Add `WriteTimeout=35min` to Echo server |
| `server/router/api/v1/agent/handlers.go:1157-1206` | Non-blocking reindex (goroutine + 202) |
| `server/router/api/v1/agent/service.go:709-970` | Reorder: checkpoint before Validate() |
| `server/router/api/v1/agent/embedding.go:412-434` | Context-aware retry sleep |
| `server/router/api/v1/agent/embedding.go:681-683` | Don't retry on `context.Canceled` |
| `server/router/api/v1/agent/vectordb_lance.go:491-563` | Per-batch mutex |
| `server/router/api/v1/agent/vectordb_lance.go:565-597` | Context-aware retry sleep |
| `.env` | Add comments documenting batch size rationale |
| `docs/DOCS_ENV_VAR.MD` | Document per-context defaults table |

## Verification

### Step 1: Compile check
```bash
go build ./...
```

### Step 2: Test reindex flow locally
```bash
task run:rag
# POST reindex → should return 202 immediately
curl -X POST http://localhost:5230/api/v1/agent/evpn/reindex?audience_type=internal
# Poll status → should show in_progress → completed
curl http://localhost:5230/api/v1/agent/evpn/reindex/status?audience_type=internal
```

### Step 3: Test resume after interrupt
```bash
# Start reindex, kill server mid-batch, restart, then:
curl -X POST "http://localhost:5230/api/v1/agent/evpn/reindex?resume=true&audience_type=internal"
```

### Step 4: Verify progress UI
- Open Agent Admin, trigger reindex for evpn
- Button shows spinner immediately (202 response)
- Progress bar updates every 3s (polling)
- Chunk counts update (processed/total)
- Status changes: in_progress → completed

## Risks / notes

| Risk | Mitigation |
|------|------------|
| Background goroutine outlives HTTP request | Context bounded to 30min; goroutine checks ctx.Err() each batch |
| Concurrent reindex requests for same tenant | Per-batch mutex + checkpoint table prevents double-work |
| Frontend expects POST response with chunks | Frontend already handles missing chunks (polls for status) |
| `context.Canceled` no longer retries | Intentional: client disconnected means no one is listening |
| Per-batch mutex may reduce write throughput | Trades throughput for responsiveness; searches can proceed between batches |

## Status

Plan complete. **No code changes yet** — awaiting approval to implement.
