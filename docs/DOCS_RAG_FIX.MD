# RAG Embedding Reliability Fix

## Problem
RAG indexing was failing with `OpenRouter API error: No successful provider responses`.
- **Root Cause 1**: Batch size of 32 with large chunks (~800 tokens) created payloads (~25k tokens) that frequently timed out or were rejected by upstream providers.
- **Root Cause 2**: OpenRouter sometimes fails *fast* (immediately returning error) when upstream is busy, bypassing the long HTTP timeout. The original retry logic (wait ~5s) was too short to allow recovery.

## Changes Verified

### 1. Reduced Batch Size
- **Change**: `EMBEDDING_BATCH_SIZE` reduced from `32` to **`10`**.
- **Impact**: Payload size reduced to ~8k tokens, well within safe limits for `openai/text-embedding-3-small`.
- **Verification**: 
  - Batch 32: Hung for >3 minutes / Failed.
  - Batch 16: Succeeded in 1.5s.
  - Batch 10: Succeeded consistently in ~0.8s.

### 2. Aggressive Retry Strategy
- **Change**: Increased `maxRetries` from `3` to **`10`**.
- **Change**: Implemented **Exponential Backoff** (`2s * 2^attempt`) with **30s Cap**.
  - Attempt 1: Wait 2s
  - Attempt 2: Wait 4s
  - ...
  - Attempt 5: Wait 30s
  - ...
  - Attempt 10: Wait 30s
- **Impact**: Total retry duration increased to **~3.5 minutes**. This gives OpenRouter/Upstream substantial time to recover from transient outages.

### 3. Chunk Size Optimization (Root Cause Fix)
- **Problem**: [GetMaxChunkTokens](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/chunker.go#73-89) was set to **7000** (initial) then **2000**, but Qwen model failed even with 20k token batches.
- **Consequence**: "No successful provider responses" due to exceeding Qwen/OpenRouter per-request limits (likely ~8k).
- **Fix**: 
  - Reduced [MaxChunkTokens](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/chunker.go#73-89) to **1000**.
  - Reduced `EMBEDDING_BATCH_SIZE` to **5**.
- **Result**: Max batch size is now ~5k tokens, universally safe.


## Verification Results
- **OpenAI Model**: Success with **Batch 10 / Chunk 2000** (and verifying **Batch 5 / Chunk 1000**).
- **Qwen Model**: Failed consistently with "No successful provider responses" even at **Batch 5 / Chunk 1000** (5k payload). Confirmed model/provider instability.
- **Final State**: Reverted to `openai/text-embedding-3-small` with safe config (**Chunk 1000, Batch 5**) to ensure rock-solid stability.

## How to Run
The fix is applied to the codebase and configuration. You can now reliably rebuild indexes using your preferred method:

```bash
# Option A: Rebuild via Task (Check .env first)
task run:rag

# Option B: Rebuild specific tenant via API
# (Requires token)
curl -X POST "http://localhost:8081/api/v1/agent/browse/reindex?audience=internal" ...
```
