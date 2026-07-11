# Plan 4: Defeat the 8192 Embedding Limit (DeepSeek Analysis)

## Problem

Despite Plan 3's fix (real tokenizer, `splitByHardLimit` binary search, flush guards, final guard before `addChunkOverlap`), a 14MB markdown file still produces:

```
failed at batch 1: batch 1 failed with permanent error: failed to generate embeddings for batch 1:
embedding provider unavailable: OpenRouter API error: HTTP 400:
{ "error": { "message": "Invalid 'input[0]': maximum input length is 8192 tokens.", ... } }
```

The chunker guard (`final guard` at `chunker.go:450–473`) correctly limits pre-overlap chunks to `maxTokens` (512). The `addChunkOverlap` adds ~50 tokens. Neither should approach 8192.

## Investigation Summary

### What was confirmed correct

| Component | Status | Evidence |
|-----------|--------|----------|
| `splitByHardLimit` binary search | ✓ Verified | Standalone test: every chunk = exactly 512 tokens |
| `ChunkMarkdownContent` guard | ✓ Correct | Runs before `addChunkOverlap`, iterates ALL chunks, catches oversized |
| `ReindexTenantContentWithResume` → `InsertWithCheckpoint` | ✓ Traced | Uses `ChunkMarkdownContent`, guard applies |
| `InitTokenizer` call site | ✓ Present | Called in `NewVectorDB()` at vectordb.go:249 |
| All reindex paths use chunker | ✓ Confirmed | `ReindexAllContent`, `ReindexTenantContent`, `ReindexTenantContentWithResume` all call `s.chunker.ChunkMarkdownContent` |
| Non-RAG build stub returns error | ✓ Confirmed | `vectordb_nolance.go` returns error for `newPool`, preventing silent fallback |

### What remains unconfirmed (code reading cannot verify)

The error occurs at runtime. The chunker output goes through `ChunkMarkdownContent` → `allChunks` → `InsertWithCheckpoint` → `processSingleBatch` → `Embed()`. At each stage the data is correct in theory, but we cannot prove it without runtime diagnostics.

### Root Cause Hypotheses (ordered by likelihood)

1. **No defense-in-depth at the Embed call site.** `doEmbed()` at `embedding.go:377` sends `texts` directly to OpenRouter with zero token-count validation. If any text in `textsToEmbed` exceeds 8192 tokens (regardless of reason — uninitialized tokenizer, chunker edge case, title inflation, data corruption), the API returns a permanent 400 error.

2. **Tokenizer not initialized by the time chunking runs.** If `InitTokenizer` fails silently (or `NewVectorDB` is called but the tokenizer init errors), `EstimateTokens` falls back to `len/4`. For the 14MB file, this heuristic could produce incorrect sizing. **However**, even with `len/4`, a 2048-byte chunk (which would pass the `512` heuristic guard as "≤512 tokens") cannot reach 8192 real tokens for any plausible text density. This makes hypothesis #2 unlikely as the sole cause.

3. **O(n log n) binary search stress on the tokenizer.** `splitByHardLimit` calls `Count()` on progressively larger substrings. For 14MB of text, each call tokenizes millions of runes via `regexp2` (backtracking regex engine). If `Count()` encounters an error or timeout on any call, `EstimateTokens` falls back to `len/4` for that probe. This could lead to incorrect split point selection.

4. **`mergeSmallChunks` sum-of-parts undercount.** Line 750–751 uses `EstimateTokens(A) + EstimateTokens(B) ≤ maxTokens` to decide merges, but the actual merged content `EstimateTokens(A + "\n\n" + B)` can be larger (subword boundary effects). The guard catches this, but only for `ChunkMarkdownContent` output. If any other code path creates chunks without the guard, oversizing is possible.

5. **Observation indexer bypasses the chunker.** `observation_indexer.go` creates chunks via `parseObservationsToChunks` — not through `ChunkMarkdownContent`. These chunks have no guard. If observations from the 14MB file's associated session are being indexed alongside the reindex, they could be oversized. But the error says "batch 1" from `processSingleBatch` (part of `InsertWithCheckpoint`), which is called from `ReindexTenantContentWithResume`, not from the observation indexer (which uses `Insert()` directly). Lower likelihood.

## Fix Plan

### Fix 1: Pre-embedding guard in `processSingleBatch` (defense-in-depth)

**File:** `vectordb_lance.go` — `processSingleBatch` (line 617)

Before calling `db.embedSvc.Embed(ctx, textsToEmbed)`, iterate `textsToEmbed` and check each with `EstimateTokens()`. If any exceeds `maxInputTokens` (e.g., 8000 — inline constant or `GetMaxChunkTokens * 15`), log the chunk details (title, content length, token count, first 200 chars) and call `splitByHardLimit` to split the text inline, generating multiple embeddings per chunk position.

This is the **primary fix** — it catches ANY oversized text regardless of origin (chunker bug, tokenizer failure, data corruption, future code changes).

```go
// Pseudocode — insert before embedSvc.Embed call:
maxInputTokens := db.maxInputTokens() // e.g., 8000
for i, text := range textsToEmbed {
    tokens := EstimateTokens(text)
    if tokens > maxInputTokens {
        slog.Error("Oversized embedding input detected",
            "index", i,
            "tokens", tokens,
            "limit", maxInputTokens,
            "contentLength", len(text),
            "title", extractTitle(text),
            "contentPreview", text[:min(200, len(text))])
        // Split the text and re-embed
        parts := splitByHardLimit(text, maxInputTokens)
        // ... generate embedding for each part ...
    }
}
```

### Fix 2: Same guard in `Insert()` (vectordb_lance.go:438)

The non-checkpoint `Insert` method also calls `Embed()` directly at line 438. Apply the same pre-embedding validation before `db.embedSvc.Embed(ctx, textsToEmbed)`.

### Fix 3: Diagnostic logging in the guard

Add detailed logging when the final guard in `ChunkMarkdownContent` catches an oversized chunk (line 452). Log `EstimateTokens(chunk.Content)`, `maxTokens`, content length, and title. This confirms whether the guard is actually being triggered.

```go
// At chunker.go:452 — enhance existing log
if EstimateTokens(chunk.Content) > maxTokens {
    slog.Warn("Chunk exceeded maxTokens, splitting",
        "actualTokens", EstimateTokens(chunk.Content),
        "maxTokens", maxTokens,
        "title", chunk.Title,
        "contentLength", len(chunk.Content))
    parts := splitByHardLimit(chunk.Content, maxTokens)
    // ...
}
```

### Fix 4: Fast-path in `splitByHardLimit` for small inputs (optimization)

At the top of `splitByHardLimit`, add a fast-path check: if `EstimateTokens(text) ≤ maxTokens`, return `[]string{text}` immediately. This avoids the binary search setup cost for chunks that already fit.

(Already present at line 797–799. Verify it's triggering correctly.)

### Fix 5: Consider adding a `maxInputTokens` field to `LanceVectorDB` and `MemoryVectorDB`

Derive from `GetMaxChunkTokens` × 15 (safe margin below 8192). Use in the pre-embedding check rather than a magic constant.

### Fix 6 (investigative, optional): Add tokenizer initialization logging

At each reindex call site, log whether `globalTokenizer` is set. This confirms the tokenizer was initialized before chunking.

## Files to Modify

| File | Lines | Change |
|------|-------|--------|
| `vectordb_lance.go` | 617–638 | **Fix 1:** Pre-embedding guard in `processSingleBatch` with `splitByHardLimit` fallback |
| `vectordb_lance.go` | 427–446 | **Fix 2:** Same guard in `Insert()` |
| `chunker.go` | 450–473 | **Fix 3:** Enhanced logging when guard catches oversized chunk |
| `chunker.go` | 791 | **Fix 4:** Verify fast-path at line 797 works for large inputs |
| `vectordb.go` and `vectordb_lance.go` | — | **Fix 5:** Add `maxInputTokens` field |

## Verification

1. Run `go test -v -run 'TestChunker' ./server/router/api/v1/agent/` — all existing tests pass
2. Run `go vet ./server/router/api/v1/agent/...` — no new issues
3. Deploy and reindex the 14MB file
4. If error persists, examine logs for "Oversized embedding input" or "Chunk exceeded maxTokens" entries to determine the actual source
