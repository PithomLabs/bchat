# Plan 037 — Update: evpn Zero Chunks Investigation

**Date:** 2026-07-14

## Completed

| Milestone | Status | File Changes |
|-----------|--------|-------------|
| Plan 036 (SQLite → PG parity) | ✅ | LATEST.sql, 00__tenant_isolation.sql, agent.go |
| Plan 037 initial (plan037) | ✅ | handlers.go, service.go, embedding.go, vectordb_lance.go, server.go |
| Plan 037 plan2 (config clamping) | ✅ | embedding.go, vectordb_lance.go |

### Plan 037 Changes Implemented
- **A1:** `MinEmbeddingBatchSize=10` floor in `GetEmbeddingBatchSize()` (`embedding.go:836-865`)
- **A2:** `getEmbeddingTimeout()` wrapper with `MaxEmbeddingTimeout=5m` (`embedding.go:812-831`)
- **B1:** Throughput projection in `InsertWithCheckpoint` — MaxReindexDuration=30m (`vectordb_lance.go:522-540`)
- **B2:** Circuit breaker `MaxConsecutiveFailures=3` (`vectordb_lance.go:545-550`)
- **C1:** Non-blocking reindex handler returning 202 (`handlers.go:1190-1213`)
- **C2:** Validate() moved after checkpoint creation (`service.go`)
- **C3:** `context.Canceled` → `return false` in `isRetryableError()` (`embedding.go:681-687`)
- **C4:** Per-batch mutex in `InsertWithCheckpoint` (`vectordb_lance.go:498-555`)
- **C5:** Context-aware retry in `processBatchWithRetry` (`vectordb_lance.go:588-591`)
- **D1:** `WriteTimeout=35m`, `IdleTimeout=120s` (`server.go:50-73`)
- **D2:** Per-context env var defaults documented (`DOCS_ENV_VAR.MD`, `.env`)

---

## Current Issue: evpn Tenant Produces Zero Chunks

### Symptom
User clicked "Rebuild Index" for evpn tenant. No output appeared. RAG Stats shows 0 chunks.

### DB State (as of 2026-07-14)

```
Tenant: evpn = ID 13
Source Files: 1 row
  - ID: 41
  - audience: internal
  - type: kb
  - content: 1,453,682 bytes (~1.4MB)
  - version: 1

Checkpoints for tenant 13: NONE
```

### Root Cause Analysis

**The goroutine likely never ran.** Key evidence:

1. **No checkpoint** — `agent_reindex_checkpoints` has zero rows for tenant 13. The `ReindexTenantContentWithResume` function creates a checkpoint as its first action. If no checkpoint exists, the goroutine never reached that point.

2. **Handler flow** — `handlers.go:1190-1213` launches the goroutine with `go func()`. If the HTTP handler returned before the goroutine started (race condition), the goroutine would be orphaned but still run — UNLESS something prevented it from starting.

3. **Frontend polling** — The frontend polls `GET /:slug/reindex/status` every 3s. If no checkpoint exists, `GetReindexStatus` would return `{"exists": false}` or similar, and the frontend shows nothing.

4. **Silent failure at service level** — `service.go:805-807` returns `(0, nil)` with NO log when `allChunks` is empty. If chunking produced zero chunks, this would silently succeed.

### Possible Causes

| # | Hypothesis | Evidence For | Evidence Against |
|---|-----------|-------------|-----------------|
| 1 | Goroutine never started | No checkpoint, no logs | Frontend shows "Reindexing" UI |
| 2 | Source file parsing produced zero chunks | `service.go:805-807` silent return | 1.4MB file should produce many chunks |
| 3 | External KB required but not present | audience=internal, no external files | Internal KB should work |
| 4 | Embedding failures caused silent chunk dropping | `vectordb_lance.go:796-799` silent nil | No checkpoint = never reached embed phase |
| 5 | Context canceled immediately | Client disconnected | Frontend shows UI, not canceled |

### Silent Failure Paths Identified

1. **`service.go:805-807`** — Returns `(0, nil)` when `allChunks` is empty (NO LOG)
2. **`vectordb_lance.go:796-799`** — `processSingleBatch` returns nil when ALL chunks fail to embed (WARN log only)
3. **`handlers.go:1195-1203`** — Goroutine logs to stderr; user may not see output

---

## Adversarial Review Reconciliation

| Reviewer | Stance on Plan2 |
|----------|----------------|
| `plan2_review.md` (Claude) | Config clamping insufficient; replace with throughput projection |
| `plan2_review_stepfun.md` | Agrees with above; proposes synthesis |
| `plan2_review_m1.md` | Agrees; recommends concrete thresholds |
| `plan2_review_deepseek.md` | **Says plan2 is MORE sound.** Config clamping IS root-cause fix. Throughput projection is supplementary. |

**Implemented:** Both config clamping (A1+A2) and throughput projection (B1+B2) per plan2_review_deepseek's synthesis. Budget formula: `totalBatches × perBatch × 1.2`, capped at 30m.

---

## Next Steps

### Immediate (Investigation)
1. Add logging to goroutine launch to confirm it actually starts
2. Add logging to source file loading to confirm files are found
3. Add logging to `ReindexTenantContentWithResume` at each stage

### Short-term (Bug Fix)
1. Fix silent failure at `service.go:805-807` — log when zero chunks
2. Clean up stuck checkpoint for tenant 12 (ID=12, batch 1493/2588)

### Medium-term (Verification)
1. Run evpn reindex with added logging
2. Verify zero-chunk root cause
3. Ensure frontend polling works correctly with no-checkpoint state
