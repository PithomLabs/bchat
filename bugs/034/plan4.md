# Plan 4: Defeat the 8192 Embedding Limit (Consolidated)

## Problem

Despite Plan 3's fix (real tokenizer, `splitByHardLimit` binary search, flush guards, final guard before `addChunkOverlap`), a 14MB markdown file still produces:

```
failed at batch 1: batch 1 failed with permanent error: failed to generate embeddings for batch 1:
embedding provider unavailable: OpenRouter API error: HTTP 400:
{ "error": { "message": "Invalid 'input[0]': maximum input length is 8192 tokens.", ... } }
```

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

1. **No defense-in-depth at the Embed call site.** `doEmbed()` at `embedding.go:377` sends `texts` directly to OpenRouter with zero token-count validation. If any text in `textsToEmbed` exceeds 8192 tokens (regardless of reason — uninitialized tokenizer, chunker edge case, title inflation, data corruption), the API returns a permanent 400 error. **This is the primary failure mode.**

2. **Guard does not account for title prepended at embedding time.** The guard at `chunker.go:452` checks `EstimateTokens(chunk.Content)` but the embedding text is `fmt.Sprintf("%s: %s", chunk.Title, chunk.Content)`. Titles are typically short (~10 tokens), so this alone cannot explain 8192+ tokens — but it contributes to the gap between what the guard validates and what the API receives.

3. **`addChunkOverlap` inflates content after the guard.** The guard runs before `addChunkOverlap` (`chunker.go:476`), which prepends ~50 tokens of overlap. The overlap uses `overlapTokens * 4` characters (200 chars), but for dense text without spaces, 200 chars could represent 200+ real tokens rather than the assumed 50. Combined with the title, this creates a larger gap between guard-checked size and actual embedding input.

4. **Tokenizer not initialized by the time chunking runs.** If `InitTokenizer` fails silently (or `NewVectorDB` is called but the tokenizer init errors), `EstimateTokens` falls back to `len/4`. For the 14MB file, this heuristic could produce incorrect sizing. **However**, even with `len/4`, a 2048-byte chunk (which would pass the `512` heuristic guard as "≤512 tokens") cannot reach 8192 real tokens for any plausible text density. This makes tokenizer failure unlikely as the **sole** cause, but it could be a contributing factor combined with other edge cases.

5. **O(n log n) binary search stress on the tokenizer.** `splitByHardLimit` calls `Count()` on progressively larger substrings. For 14MB of text, each call tokenizes millions of runes via `regexp2` (backtracking regex engine). If `Count()` encounters an error or timeout on any call, `EstimateTokens` falls back to `len/4` for that probe. This could lead to incorrect split point selection for a few chunks near the boundary.

6. **`mergeSmallChunks` sum-of-parts undercount.** Line 750–751 uses `EstimateTokens(A) + EstimateTokens(B) ≤ maxTokens` to decide merges, but the actual merged content `EstimateTokens(A + "\n\n" + B)` can be larger (subword boundary effects). The guard catches this, but only for `ChunkMarkdownContent` output.

7. **Observation indexer bypasses the chunker.** `observation_indexer.go` creates chunks via `parseObservationsToChunks` — not through `ChunkMarkdownContent`. These chunks have no guard. If observations from the 14MB file's associated session are being indexed alongside the reindex, they could be oversized. But the error says "batch 1" from `processSingleBatch` (part of `InsertWithCheckpoint`), which is called from `ReindexTenantContentWithResume`, not from the observation indexer (which uses `Insert()` directly). Lower likelihood.

## Fix Plan

### Fix 1 (PRIMARY): Pre-embedding guard in `processSingleBatch` — defense-in-depth

**File:** `vectordb_lance.go` — `processSingleBatch` (line 617)

Before calling `db.embedSvc.Embed(ctx, textsToEmbed)`, iterate `textsToEmbed` and check each with `EstimateTokens()`. If any exceeds `maxInputTokens` (8000 — inline constant or derived from `GetMaxChunkTokens * 15`), log the chunk details (title, content length, token count, first 200 chars) and split the text using `splitByHardLimit`, generating multiple embeddings per chunk position.

This is the **primary fix** — it catches ANY oversized text regardless of origin (chunker bug, tokenizer failure, data corruption, future code changes).

```go
// Pseudocode — insert before embedSvc.Embed call:
maxInputTokens := 8000 // safety margin below 8192
for i, text := range textsToEmbed {
    tokens := EstimateTokens(text)
    if tokens > maxInputTokens {
        slog.Error("Oversized embedding input detected",
            "index", i, "tokens", tokens, "limit", maxInputTokens,
            "contentLength", len(text),
            "title", extractTitle(text),
            "contentPreview", text[:min(200, len(text))])
        parts := splitByHardLimit(text, maxInputTokens)
        // ... generate embedding for each part, store separately ...
    }
}
```

### Fix 2: Same guard in `Insert()` (vectordb_lance.go:427–446)

The non-checkpoint `Insert` method also calls `Embed()` directly. Apply the same pre-embedding validation before `db.embedSvc.Embed(ctx, textsToEmbed)`.

### Fix 3: Include title in the chunker guard

**File:** `chunker.go` — Guard at line 452

Change from:
```go
if EstimateTokens(chunk.Content) > maxTokens {
```
To:
```go
embedText := fmt.Sprintf("%s: %s", chunk.Title, chunk.Content)
if EstimateTokens(embedText) > maxTokens {
```

This accounts for the title prepended at embedding time, closing the gap between what the guard validates and what the API receives.

### Fix 4: Enhanced diagnostic logging in the guard

**File:** `chunker.go` — line 452

Add detailed logging when the final guard catches an oversized chunk:

```go
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

### Fix 5: Tokenizer initialization verification logging

**File:** `embedding.go` — `InitTokenizer`

- On success: log a verification test sentence result to confirm tokenizer works
- On failure: log `slog.Error("CRITICAL: Failed to initialize tokenizer...")` with the specific error and impact statement

**File:** `embedding.go` — `EstimateTokens`

- Add a one-time `slog.Warn` when the `len/4` fallback is used, so we know immediately if the tokenizer failed at runtime

### Fix 6: Add `maxInputTokens` field

**File:** `vectordb_lance.go` and `vectordb.go`

Add a `maxInputTokens` field derived from `GetMaxChunkTokens` × 15 (safe margin below 8192). Use in the pre-embedding check rather than a magic constant.

### Fix 7 (optimization): Verify fast-path in `splitByHardLimit`

The fast-path at line 797–799 (`if EstimateTokens(remainder) <= maxTokens`) already exists. Verify it triggers correctly for large inputs by adding a log when it does.

## Files to Modify

| File | Lines | Change |
|------|-------|--------|
| `vectordb_lance.go` | 617–638 | **Fix 1:** Pre-embedding guard in `processSingleBatch` with `splitByHardLimit` fallback |
| `vectordb_lance.go` | 427–446 | **Fix 2:** Same guard in `Insert()` |
| `chunker.go` | 450–473 | **Fix 3:** Include title in guard check; **Fix 4:** Enhanced logging |
| `embedding.go` | 32–56, 107–115 | **Fix 5:** Tokenizer init verification + fallback warning |
| `vectordb.go`, `vectordb_lance.go` | — | **Fix 6:** Add `maxInputTokens` field |

## Verification

1. `go test -v -run 'TestChunker' ./server/router/api/v1/agent/` — all existing tests pass
2. `go vet ./server/router/api/v1/agent/...` — no new issues
3. Build with `task build:backend:rag`
4. Deploy and reindex the 14MB file
5. Check logs for "Tokenizer initialized" with verification test result
6. Check logs for any "Oversized embedding input" or "Chunk exceeded maxTokens" entries
7. If error persists, the diagnostic logs will pinpoint the exact source
