# Plan 4: Defeat the 8192 Embedding Limit (Consolidated)

## Problem

Despite Plan 3's fix (real tokenizer, `splitByHardLimit` binary search, flush guards, final guard before `addChunkOverlap`), a 14MB markdown file still produces:

```
failed at batch 1: batch 1 failed with permanent error: failed to generate embeddings for batch 1:
embedding provider unavailable: OpenRouter API error: HTTP 400:
{ "error": { "message": "Invalid 'input[0]': maximum input length is 8192 tokens.", ... } }
```

## Root Cause (Updated)

The `ChunkMarkdownContent` guard validates `EstimateTokens(chunk.Content)` but the embedding API receives `fmt.Sprintf("%s: %s", chunk.Title, chunk.Content)`. The title adds ~10–50 tokens, and `addChunkOverlap` adds another ~50–200 tokens after the guard. Combined, a chunk near the 512-token boundary can inflate by 50–250 tokens before embedding — still far below 8192.

The **real** issue: there is **zero validation** at the Embed call site (`doEmbed` in `embedding.go:377`). If ANY text exceeds 8192 tokens for any reason (tokenizer not initialized, chunker edge case, data corruption, future code changes), the API returns a permanent 400 error with no diagnostic info. This is a defense-in-depth failure.

## Fix Plan

### Part A — Defense-in-Depth (Fixes 1–6)

#### Fix 1 (PRIMARY): Pre-embedding guard in `processSingleBatch`

**File:** `vectordb_lance.go` — `processSingleBatch` (line 617)

Before calling `Embed()`, expand the batch by splitting any oversized chunks. Each chunk whose `Title + ": " + Content` exceeds `MaxEmbeddingInputTokens` (8000) gets split via `splitByHardLimit` with a content limit adjusted for the title overhead. The expanded chunks are embedded as separate entries.

A helper method `expandAndValidateBatch` does the split. Called at the top of `processSingleBatch` and also in `Insert`.

#### Fix 2: Same guard in `Insert()` (vectordb_lance.go:427–446)

Same `expandAndValidateBatch` call in the `Insert` batch loop.

#### Fix 3: Include title in the chunker guard

**File:** `chunker.go` — Guard at line 452

Change from `EstimateTokens(chunk.Content)` to `EstimateTokens(embedText)` where `embedText = fmt.Sprintf("%s: %s", chunk.Title, chunk.Content)`. Pass an adjusted content limit (`maxTokens - titleOverhead`) to `splitByHardLimit`.

#### Fix 4: Enhanced diagnostic logging

**File:** `chunker.go` — line 452

Log warning when the guard catches an oversized chunk (token count, title, content length).

**File:** `vectordb_lance.go` — `expandAndValidateBatch`

Log error when the pre-embedding guard catches an oversized chunk (same details).

#### Fix 5: Tokenizer verification logging

**File:** `embedding.go` — `InitTokenizer` + `EstimateTokens`

- On init failure: `slog.Error("CRITICAL: ...")` with explicit impact statement
- On init success: test with a known sentence and log `testStringTokens`
- In `EstimateTokens` fallback: one-time `slog.Warn` via `sync.Once`

#### Fix 6: `MaxEmbeddingInputTokens` constant

**File:** `chunker.go` — alongside existing constants (line 72)

```go
MaxEmbeddingInputTokens = 8000 // Safety margin below OpenRouter's 8192 limit
```

---

### Part B — Chunker Simplification

The current chunker has **~561 lines** of procedural split logic with nested fallbacks (H2 → H3 → paragraphs → sentences → hard limit), a fragile `paragraphChunk` intermediate type, and deeply nested `if/else` blocks in `ChunkMarkdownContent` (lines 331–426). Despite all this complexity, for non-markdown content (like your 14MB file), every strategy fails and it falls through to `splitByHardLimit` anyway.

Replace it with a **recursive split** approach:

#### New `splitContent` (recursive)

```go
func splitContent(content string, maxTokens int) []string {
    if content == "" || EstimateTokens(content) <= maxTokens {
        return []string{content}
    }
    for _, strategy := range []func(string) []string{
        splitByHeaders,     // ##, ###, etc.
        splitByBlankLines,  // \n\n
        splitBySentences,   // . ! ?
    } {
        if parts := strategy(content); len(parts) > 1 {
            var result []string
            for _, part := range parts {
                result = append(result, splitContent(part, maxTokens)...)
            }
            return result
        }
    }
    return splitByHardLimit(content, maxTokens)
}
```

#### Strategy functions

| Function | Splits on | Returns >1 when |
|----------|-----------|-----------------|
| `splitByHeaders` | Any `^##`, `^###`, `^####` etc. | Multiple sections exist |
| `splitByBlankLines` | `\n\n` | Multiple paragraphs exist |
| `splitBySentences` | `. ! ?` | Multiple sentences exist (unchanged) |
| `splitByHardLimit` | Token-count binary search (unchanged) | Content exceeds maxTokens |

#### `ChunkMarkdownContent` body (simplified)

```go
parts := splitContent(content, maxTokens)
for i, part := range parts {
    title, body := extractTitleAndBody(part)
    // create DocumentChunk...
}
```

#### Functions removed

| Function | Lines | Replaced by |
|----------|-------|-------------|
| `splitByH2Headers` | 37 | `splitByHeaders` (unified, ~25 lines) |
| `splitByH3Headers` | 23 | `splitByHeaders` (handles all levels) |
| `splitByParagraphs` | 123 | `splitByBlankLines` (~15 lines) + recursive `splitContent` |
| `paragraphChunk` struct | 4 | No intermediate type needed |

`extractTitleAndBody` stays but simplifies — no H3-specific path needed.

`mergeSmallChunks` stays — it handles the "group small chunks back together" step.

#### Code size impact

| | Before | After |
|--|--------|-------|
| Split logic lines | ~561 | ~268 |
| `ChunkMarkdownContent` body | ~196 | ~40 |
| Intermediate types | `paragraphChunk` | None |
| Headers | H2 + H3 separate | Unified `splitByHeaders` |
| Domain-specific | Markdown | General text |

**Behavioral change:** The H2→H3 title hierarchy (`"H2 > H3"`) is lost — `extractTitleAndBody` returns just `"H3"`. For RAG, content is what matters; titles are metadata. If hierarchy matters later, it can be added back via a parent-title context parameter.

#### Test impact

All 4 existing `TestChunker*` tests should pass unchanged — they test final chunk sizes and guard behavior, not intermediate splitting details.

---

## Implementation Order

| Step | File | Fix | Complexity | Risk |
|------|------|-----|-----------|------|
| 1 | `chunker.go` | Fix 6: Add `MaxEmbeddingInputTokens` constant | Trivial | None |
| 2 | `embedding.go` | Fix 5: Tokenizer logging | Low | None |
| 3 | `chunker.go` | Fix 3+4: Include title in guard + logging | Low | Low |
| 4 | `vectordb_lance.go` | Fix 1+2: `expandAndValidateBatch` in both Insert paths | Medium | Low |
| 5 | `chunker.go` | Part B: Recursive split simplification | High | Medium |
| — | `chunker_test.go` | Verify all 4 tests still pass | — | — |

Steps 1–4 are independent of Step 5 and can be verified before the simplification.

## Files to Modify

| File | Lines | Change |
|------|-------|--------|
| `chunker.go` | 72 (constants) | **Fix 6:** Add `MaxEmbeddingInputTokens = 8000` |
| `chunker.go` | 284–426 | **Part B:** Replace `ChunkMarkdownContent` body + add `splitContent`, `splitByHeaders`, `splitByBlankLines`; remove `splitByH2Headers`, `splitByH3Headers`, `splitByParagraphs`, `paragraphChunk` |
| `chunker.go` | 450–473 | **Fix 3+4:** Include title in guard check + logging |
| `embedding.go` | 32–56 | **Fix 5:** Upgrade failure log, add verification test |
| `embedding.go` | 107–115 | **Fix 5:** One-time fallback warning |
| `vectordb_lance.go` | 616–654 | **Fix 1:** Add `expandAndValidateBatch` helper; call in `processSingleBatch` |
| `vectordb_lance.go` | 410–446 | **Fix 2:** Call `expandAndValidateBatch` in `Insert` batch loop |

## Verification

1. `go test -v -run 'TestChunker' ./server/router/api/v1/agent/` — all 4 tests pass
2. `go test -v -run 'TestEstimateTokens' ./server/router/api/v1/agent/` — passes
3. `go build -tags rag ./server/router/api/v1/agent/` — compiles
4. `go vet -tags rag ./server/router/api/v1/agent/...` — clean
5. Deploy, reindex 14MB file, check logs for "Tokenizer initialized" with test value
6. Verify no "Oversized embedding input" or "Chunk exceeded maxTokens" entries
