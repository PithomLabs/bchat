# Plan 037b — Fix Recurring Reindex Stuck Issue (Underlying Cause: Unguarded Config + Unbounded Operation Budget)

## Problem / Context

Plan 037 shipped the architectural changes (non-blocking reindex via `handlers.go:1192-1210`,
`Validate()` after checkpoint at `service.go:853-859`, context-aware retries). Those fix the
**symptom** — a blocked HTTP request — but NOT the **disease**: the reindex operation's
duration is still entirely dictated by unguarded environment configuration.

Concretely, with `EMBEDDING_BATCH_SIZE=1` (from a `Taskfile.yml` inline override) and
`EMBEDDING_TIMEOUT=10m` (from `.env`):

- `GetEmbeddingBatchSize()` (`embedding.go:819-825`) clamps to ≤200 but **permits a minimum of 1**,
  so batch size 1 survives.
- `getEnvDuration("EMBEDDING_TIMEOUT", 180s)` (`embedding.go:802`, used at `embedding.go:363` and
  `embedding.go:255`) has **no upper bound**, so 10m survives.
- A 1.4MB KB file → ~183 chunks at `RAG_MAX_CHUNK_TOKENS=4096`. With batch size 1 that is 183
  sequential embedding calls, each allowed to wait up to `EMBEDDING_TIMEOUT=10m`.
- The reindex runs in a detached `30*time.Minute` context (`handlers.go:1194`). It does NOT fail
  fast — it runs ~3 batches × 10m ≈ 30 min, then the detached context fires `DeadlineExceeded`,
  `InsertWithCheckpoint` returns `ctx.Err()`, and only 3 of 183 chunks are indexed before the
  checkpoint is persisted as `failed` via the detached 5s context (`service.go:904`).
- To the user: the UI shows `in_progress` for 30 minutes, then `failed`. That reads as "still stuck."

**Root cause:** The reindex cost is unbounded and uncontrolled. A single operator error in env
vars (or a Taskfile override) silently degrades reindex by ~180× with no detection, no fast-fail,
and no circuit breaker. Plan 037 treated the symptom; this plan removes the disease.

### Goal

1. **(A) Clamp embedding config server-side** so pathological values are physically impossible
   and any clamping is logged (visible, not silent).
2. **(B) Derive the reindex operation budget from actual work size** so the operation fails fast
   and predictably when config would make it exceed a sane ceiling — instead of hanging for 30 min
   and dropping most chunks.

## Current-state gaps (grounded in code)

| # | Area | Location | Issue |
|---|------|----------|-------|
| 1 | Embedding | `embedding.go:819-825` | `GetEmbeddingBatchSize()` allows a minimum of 1 — permits pathological single-chunk batches |
| 2 | Embedding | `embedding.go:802` (used at `363`, `255`) | `getEnvDuration("EMBEDDING_TIMEOUT")` has no upper bound — 10m/1h silently accepted |
| 3 | Handler | `handlers.go:1194` | Detached reindex context hardcoded to `30*time.Minute` — a blunt cap unrelated to work size; operation can run the full 30 min and fail having done only a few batches |
| 4 | Service | `service.go:811-812` | `totalBatches` is computed but never used to bound total operation duration |

### Notes on what's already correct (no change needed)

- Non-blocking reindex (goroutine + `202 Accepted`) — `handlers.go:1192-1210`
- `Validate()` runs after checkpoint creation — `service.go:853-859`
- Checkpoint + resume mechanism — `service.go:827-859`, `904`
- Frontend polling UI — `AgentAdmin.tsx`
- `EMBEDDING_BATCH_SIZE` already has a ≤200 ceiling (`embedding.go:821`) — only the floor is missing

## Proposed changes

### Change A1: Floor + warning on `GetEmbeddingBatchSize`

**File:** `server/router/api/v1/agent/embedding.go:819-825`

```go
// GetEmbeddingBatchSize returns the embedding batch size from env or default.
// Controls how many chunks are sent to the embedding API per request.
// Default is 200 (was 10). Larger batches drastically cut the number of
// API calls during reindex — the dominant cost for large KBs. The OpenRouter
// text-embedding-3-small request limit (8191 tokens/input) keeps a 200×~1024
// token batch well within bounds. For Qwen3 (32K context), 40 is safe with
// 800-token chunks.
//
// A floor of MinEmbeddingBatchSize prevents pathological single-chunk batches
// (e.g. EMBEDDING_BATCH_SIZE=1) that make reindex run for hours. (Plan 037b / A)
const MinEmbeddingBatchSize = 10
const MaxEmbeddingBatchSize = 200

func GetEmbeddingBatchSize() int {
	if v := os.Getenv("EMBEDDING_BATCH_SIZE"); v != "" {
		if size, err := strconv.Atoi(v); err == nil {
			if size < MinEmbeddingBatchSize {
				slog.Warn("EMBEDDING_BATCH_SIZE below minimum; clamping",
					"requested", size, "used", MinEmbeddingBatchSize)
				return MinEmbeddingBatchSize
			}
			if size > MaxEmbeddingBatchSize {
				slog.Warn("EMBEDDING_BATCH_SIZE above maximum; clamping",
					"requested", size, "used", MaxEmbeddingBatchSize)
				return MaxEmbeddingBatchSize
			}
			return size
		}
		slog.Warn("Invalid EMBEDDING_BATCH_SIZE; using default", "value", v)
	}
	return 200
}
```

**Why:** Makes the `EMBEDDING_BATCH_SIZE=1` footgun physically impossible and surfaces the
misconfiguration in logs instead of silently degrading performance.

### Change A2: Clamp `EMBEDDING_TIMEOUT` with a maximum + warning

**File:** `server/router/api/v1/agent/embedding.go:800-810`

Add bounds around `getEnvDuration` for the timeout key, and introduce a shared clamp helper used
by both the OpenRouter and local providers.

```go
// MaxEmbeddingTimeout bounds a single embedding API call. A value larger than
// this (e.g. 10m) lets one slow batch block the whole reindex for minutes and,
// combined with a tiny batch size, makes reindex run for hours. (Plan 037b / A)
const MaxEmbeddingTimeout = 5 * time.Minute

// getEnvDuration returns a duration from an environment variable or default.
// Accepts formats like "180s", "3m", "1h30m".
func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			if d > MaxEmbeddingTimeout {
				slog.Warn("Embedding timeout above maximum; clamping",
					"key", key, "requested", d, "used", MaxEmbeddingTimeout)
				return MaxEmbeddingTimeout
			}
			if d <= 0 {
				slog.Warn("Embedding timeout must be positive; using default",
					"key", key, "value", v, "default", defaultVal)
				return defaultVal
			}
			return d
		}
		slog.Warn("Invalid duration format for env var, using default", "key", key, "value", v, "default", defaultVal)
	}
	return defaultVal
}
```

This single change covers both call sites (`embedding.go:363` OpenRouter, `embedding.go:255`
local) since both use `getEnvDuration("EMBEDDING_TIMEOUT", 180*time.Second)`.

**Why:** An upper bound on per-call timeout prevents one batch from blocking for minutes and stops
the cumulative 30-minute silent failure. The warning makes misconfig visible.

### Change B1: Compute a work-derived operation budget

**File:** `server/router/api/v1/agent/handlers.go:1157-1211`

The detached context duration is currently hardcoded to `30*time.Minute`. Replace it with a budget
derived from the actual work:

```go
// Non-blocking: run reindex in background goroutine with detached context.
// The frontend polls GET /:slug/reindex/status for progress updates.
//
// Operation budget is derived from work size so the operation fails fast and
// predictably when config would make it exceed a sane ceiling, instead of
// hanging for the full detached-context cap. (Plan 037b / B)
const MaxReindexBudget = 60 * time.Minute

budget := computeReindexBudget(audienceType)
reindexCtx, reindexCancel := context.WithTimeout(context.Background(), budget)
go func() {
	defer reindexCancel()
	chunks, reindexErr := h.service.ReindexTenantContentWithResume(reindexCtx, tenant.ID, audienceType, resume)
	if reindexErr != nil {
		slog.Error("reindex failed", "tenantID", tenant.ID, "audience", audienceType, "resume", resume, "error", reindexErr)
	} else {
		slog.Info("reindex completed", "tenantID", tenant.ID, "chunks", chunks)
	}
}()
```

Add the helper (in the same file or `service.go`):

```go
// computeReindexBudget returns a bounded duration for the whole reindex
// operation based on the number of batches and the (clamped) per-batch timeout.
// budget = totalBatches × perBatchTimeout × (MaxRetries+1), capped at MaxReindexBudget.
// This fails fast instead of silently running for the full detached-context cap.
func computeReindexBudget(tenantID int32, audienceType string) time.Duration {
	batchSize := GetEmbeddingBatchSize()
	perBatch := getEnvDuration("EMBEDDING_TIMEOUT", 180*time.Second)
	// Estimate chunk count from source files (upper bound per audience ~ a few thousand).
	// We deliberately use a conservative estimate; the goal is to bound, not to be exact.
	estimatedChunks := estimateReindexChunkCount(tenantID, audienceType)
	totalBatches := (estimatedChunks + batchSize - 1) / batchSize
	if totalBatches < 1 {
		totalBatches = 1
	}
	const embedMaxRetries = 10 // matches OpenRouterEmbedding.Embed maxRetries
	budget := time.Duration(totalBatches) * perBatch * time.Duration(embedMaxRetries+1)
	if budget > MaxReindexBudget {
		slog.Warn("Computed reindex budget exceeds ceiling; capping",
			"estimatedChunks", estimatedChunks, "totalBatches", totalBatches,
			"perBatch", perBatch, "computed", budget, "used", MaxReindexBudget)
		return MaxReindexBudget
	}
	return budget
}
```

Where `estimateReindexChunkCount` queries the source files (or reuses the existing
`ListAgentSourceFiles` + a chunk-count estimate). A simpler, dependency-free approximation is to
reuse the already-available estimate from `service.go` by exposing `totalBatches` from
`ReindexTenantContentWithResume` — but to keep the handler self-contained, estimate from the
source-file byte lengths:

```go
func estimateReindexChunkCount(tenantID int32, audienceType string) int {
	// Conservative upper bound: 1 chunk per ~1500 bytes of source content
	// (well above the true rate, so budget is over-estimated = safer).
	files, err := h.store.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
		TenantID: &tenantID, LatestOnly: true, AudienceType: &audienceType,
	})
	if err != nil || len(files) == 0 {
		return 1
	}
	total := 0
	for _, f := range files {
		total += (len(f.Content) + 1499) / 1500
	}
	if total < 1 {
		return 1
	}
	return total
}
```

(If `audienceType == "all"`, the estimate is summed across audiences; the over-estimate is fine
because the budget is capped at `MaxReindexBudget` anyway.)

**Why:** The operation budget is now proportional to real work and config, bounded by a sane
ceiling. A pathological config (batch size 1 + 10m timeout) yields a large budget that is clamped
to 60 min AND, more importantly, the per-batch timeout itself is clamped to 5m (Change A2) — so
each batch can block at most 5m and the operation still finishes or fails fast rather than dropping
180/183 chunks at the 30-min wall.

### Change B2: Surface budget in logs + checkpoint `last_message`

In `computeReindexBudget`, the clamp warning already logs. Additionally, pass the budget into the
service so it can record it in the `in_progress` checkpoint's `last_message` for operator
visibility:

- Extend `InsertOptions` (or the reindex entry) with `Budget time.Duration`.
- In `service.go:860-877` (`checkpointFunc`), include budget in `LastMessage` once.

This is optional but improves diagnosability.

## Files to modify (summary)

| File | Change |
|------|--------|
| `server/router/api/v1/agent/embedding.go:800-810` | Clamp `EMBEDDING_TIMEOUT` to `MaxEmbeddingTimeout` (5m) + warn |
| `server/router/api/v1/agent/embedding.go:819-825` | Floor `EMBEDDING_BATCH_SIZE` to `MinEmbeddingBatchSize` (10) + warn |
| `server/router/api/v1/agent/handlers.go:1157-1211` | Replace hardcoded `30*time.Minute` with `computeReindexBudget(...)` |
| `server/router/api/v1/agent/handlers.go` (new) | Add `computeReindexBudget` + `estimateReindexChunkCount` helpers + `MaxReindexBudget` const |
| `server/router/api/v1/agent/service.go:860-877` | (optional) record budget in `last_message` |

## Verification

### Step 1: Compile check
```bash
go build ./...
```

### Step 2: Config clamping is enforced + logged
```bash
# Temporarily force pathological values, then start and watch logs:
EMBEDDING_BATCH_SIZE=1 EMBEDDING_TIMEOUT=10m task run:rag
# Expect in logs (once):
#   WARN EMBEDDING_BATCH_SIZE below minimum; clamping requested=1 used=10
#   WARN Embedding timeout above maximum; clamping ... requested=10m0s used=5m0s
```

### Step 3: Healthy reindex completes fast
```bash
# Reset to sane defaults, restart:
task run:rag
curl -X POST "http://localhost:8081/api/v1/agent/evpn/reindex?audience_type=internal"
# Poll:
curl "http://localhost:8081/api/v1/agent/evpn/reindex/status?audience_type=internal"
# Expect: in_progress → completed within seconds; chunk count ~180+
```

### Step 4: Pathological config fails fast (not 30-min silent drop)
```bash
# With clamped values from Step 2, trigger reindex on a large file;
# confirm the operation budget is bounded and the checkpoint reflects failure
# quickly (well under the old 30-min wall) if the API is unhealthy.
```

## Risks / notes

| Risk | Mitigation |
|------|------------|
| `estimateReindexChunkCount` over/under-estimates | Budget is over-estimated and hard-capped at `MaxReindexBudget`; over-estimate only makes budget larger, never smaller than needed |
| Lowering `MaxEmbeddingTimeout` to 5m breaks very large single chunks | `expandAndValidateBatch` (`vectordb_lance.go:651`) already splits oversized inputs before embedding; 5m is generous for a single embedding call |
| Floor of 10 on batch size changes behavior for `run:testrag` (used 32) | 32 is within bounds; untouched. Only sub-10 values are raised to 10 |
| Budget computation adds a DB query in handler | Cheap `ListAgentSourceFiles` with `LatestOnly`; acceptable on the request path before spawning the goroutine |

## Status

Plan complete. Ready to implement.
