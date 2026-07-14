# Adversarial Review: `bugs/037/plan2.md`

**Reviewer:** Plan mode
**Date:** 2026-07-14
**Verdict:** **REWORK required** — correct diagnosis, flawed treatment

---

## What plan2 gets RIGHT

1. **Diagnosis is accurate.** The root cause is unguarded config allowing unbounded operation duration. `EMBEDDING_BATCH_SIZE=1` + `EMBEDDING_TIMEOUT=10m` produces 183 sequential 10-minute calls — a 30-hour theoretical runtime crammed into a 30-minute context.

2. **Change A1 (batch floor) is sound.** A `MinEmbeddingBatchSize = 10` floor with logged clamping is the right defensive measure. Current code at `embedding.go:821` (`size > 0 && size <= 200`) permits 1, which is pathological.

3. **Change A2 (timeout cap) is necessary.** `getEnvDuration` at `embedding.go:802` has no upper bound. A 10-minute per-call timeout is never correct for an embedding API call.

4. **Plan correctly identifies what NOT to change** — the non-blocking goroutine, checkpoint/resume, frontend polling are all fine.

---

## What plan2 gets WRONG

### Bug 1: `getEnvDuration` is generic — clamping all callers to 5m is wrong

`getEnvDuration` is called for `EMBEDDING_TIMEOUT` at `embedding.go:363` (OpenRouter) and `embedding.go:255` (local). But it's a general utility. If any other env var uses `getEnvDuration` in the future, it inherits the 5m cap silently.

**Fix:** Add a separate `getEnvDurationWithMax(key, default, max)` or clamp only at the embedding call sites, not in the shared utility.

### Bug 2: `computeReindexBudget` references `h.store` but is declared as a standalone function

The plan writes:

```go
func computeReindexBudget(tenantID int32, audienceType string) time.Duration {
    ...
    files, err := h.store.ListAgentSourceFiles(ctx, ...)
```

But `h` is not in scope — this is a package-level function, not a method on `*Handler`. It needs either:
- A method receiver `(h *Handler)`, or
- Explicit parameters `(store Store, ctx context.Context, tenantID int32, audienceType string)`

And `ctx` isn't passed either — the function needs a context for the DB query.

### Bug 3: Budget formula is wrong — over-multiplies by `embedMaxRetries+1`

```go
budget := time.Duration(totalBatches) * perBatch * time.Duration(embedMaxRetries+1)
```

This assumes every batch hits all 10 embedding retries. In practice, retries are rare (only on transient HTTP errors). The formula inflates the budget by ~11x. With `totalBatches=18, perBatch=5m, retries=10`, you get `18 × 5m × 11 = 990 minutes = 16.5 hours`, clamped to 60 minutes.

But the real issue: **the budget is still a wall-clock cap, not a progress-aware mechanism.** If 17/18 batches complete in 2 minutes but batch 18 is slow, the 60-minute budget doesn't help — the operation just runs until the context expires.

### Bug 4: `MaxReindexBudget = 60 minutes` contradicts `WriteTimeout = 35 minutes`

Plan037 set `WriteTimeout = 35m` in `server.go`. Plan2 sets `MaxReindexBudget = 60m`. The Echo server will kill the connection at 35 minutes, but the goroutine's context lives until 60 minutes. This creates a split-brain: the HTTP client sees a timeout, but the goroutine keeps running and writing checkpoints to a dead listener.

The budget should be `WriteTimeout - margin` (e.g., 30 minutes), not an independent 60-minute value.

### Bug 5: `estimateReindexChunkCount` uses `h.store` — same scoping bug as #2

This helper references `h.store` but is declared standalone. Additionally, it queries `ListAgentSourceFiles` which was already called in `ReindexTenantContentWithResume` — this is a duplicate DB query on the request path (the plan acknowledges this but dismisses it as "cheap").

---

## Design Issue: Config clamping is a band-aid, not a fix

The real question: **why does the reindex operation have no self-awareness?**

Plan2 treats the symptom (bad config) instead of the disease (the operation doesn't know when it's going too slow). A well-designed reindex should:

1. **Track throughput** — chunks/minute after each batch
2. **Project completion time** — if projected > ceiling, abort with clear error
3. **Circuit-break on consecutive failures** — if N batches fail, stop (don't burn 30 minutes)
4. **Log progress at operation level** — "batch 5/183, 2m elapsed, projected 20m remaining"

None of these require external config clamping. The operation should be resilient to *any* config, not dependent on config being correct.

---

## Recommended rework

| Change | Verdict | Rework |
|--------|---------|--------|
| A1 (batch floor) | **Approved with nit** | Good. Nit: use `MinEmbeddingBatchSize` as a named constant (plan already does this). |
| A2 (timeout cap) | **Needs rework** | Don't clamp in `getEnvDuration`. Add embedding-specific clamping at call sites only, or create `getEmbeddingTimeout()` wrapper. |
| B1 (work-derived budget) | **Reject — rewrite** | Remove `computeReindexBudget` entirely. Instead, add **throughput-aware budgeting** inside `ReindexTenantContentWithResume` itself. |
| B2 (budget in checkpoint) | **Defer** | Nice-to-have but not blocking. |

## What should replace B1

Add a **progress-aware timeout** inside `InsertWithCheckpoint` (or `ReindexTenantContentWithResume`):

```go
// After each successful batch:
elapsed := time.Since(startTime)
chunksProcessed := batchEnd
rate := float64(chunksProcessed) / elapsed.Minutes() // chunks/minute
projectedTotal := time.Duration(float64(totalChunks) / rate) * time.Minute

if projectedTotal > MaxReindexDuration {
    return fmt.Errorf("reindex too slow: projected %v but cap is %v (rate: %.0f chunks/min)",
        projectedTotal, MaxReindexDuration, rate)
}
```

This is **self-healing** — it works regardless of config, adapts to actual API performance, and fails fast when the operation is going sideways.

Additionally, add a **consecutive failure circuit breaker** in `processBatchWithRetry`:

```go
if consecutiveFailures >= maxConsecutiveFailures {
    return fmt.Errorf("circuit breaker: %d consecutive batch failures, aborting reindex", consecutiveFailures)
}
```

---

## Summary

Plan2's diagnosis is right. Changes A1 and A2 are useful defensive measures. But B1 (the core fix) introduces scoping bugs, a wrong formula, and a timeout contradiction — and most importantly, it still doesn't solve the underlying problem: **the reindex operation has no self-awareness about its own progress.**

The fix should be progress-aware budgeting inside the reindex loop, not external config clamping + a hardcoded ceiling.
