# Implementation: Real Tokenizer for Chunking Pipeline

## Problem

The RAG chunking pipeline used `len(content) / 4` to estimate token counts. This heuristic systematically **undercounts** dense content (e.g., comma-separated lists, CJK characters, minified code) by a factor of 2–4x. When the undercount reaches ~4x, chunks that are believed to be 2000 "tokens" are actually 8000+ real tokens — exceeding the embedding model's 8192-token limit and causing reindex failures.

## Solution

Replace the `len/4` heuristic with a real `cl100k_base` tokenizer (`github.com/tiktoken-go/tokenizer v0.8.0`). All token estimates throughout the codebase now use exact counts. Chunk sizes are reduced from 1000/200 (max/min) to **512/100** for the `openrouter` provider, targeting the embedding quality sweet spot with exact counting.

## Files Changed

### `server/router/api/v1/agent/embedding.go`
- Added import: `github.com/tiktoken-go/tokenizer`
- Added global: `var globalTokenizer tokenizer.Codec`
- Added function `InitTokenizer(provider, model string)` — initializes the singleton tokenizer based on model name (cl100k_base for text-embedding-* models, o200k_base for gpt-4o/gpt-5). Called once at startup; no-ops on subsequent calls. Falls back silently to `len/4` if the tokenizer fails to initialize.
- `NewEmbeddingConfigFromEnv()` unchanged (config only)

### `server/router/api/v1/agent/chunker.go`
- **`EstimateTokens(content string) int`** — replaces `len/4`. Uses `globalTokenizer.Count()` with `len(content)/4` fallback.
- **`GetMaxChunkTokens("openrouter")`** — `512` (was 1000). Exact counting allows targeting the embedding sweet spot.
- **`GetMinChunkTokens("openrouter")`** — `100` (was 200). Scaled proportionally from 512 max.
- **`ChunkMarkdownContent`** — final guard added at line 450–473 (before `addChunkOverlap`). Catches any oversized chunk that escaped prior splitting (e.g., from `splitByParagraphs` escape hatch) and splits it via `splitByHardLimit`.
- **`splitByParagraphs`** — flush guard added at line 664: if a sentence-buffered chunk still exceeds `maxTokens`, calls `splitByHardLimit` before appending. Same guard at line 692 for the final flush.
- **`splitByHardLimit(text string, maxTokens int) []string`** — rewritten to use binary search on runes with real tokenizer counting. Guarantees each part ≤ `maxTokens`. Handles the edge case where a single rune exceeds `maxTokens`.
- **`ShouldUseRAG`** — uses `EstimateTokens` (no longer `len/4` heuristics).
- **`splitBySentences`** — unchanged.

### `server/router/api/v1/agent/vectordb.go`
- Added `InitTokenizer(config.EmbeddingConfig.Provider, config.EmbeddingConfig.Model)` call at line 249 in `NewVectorDB`.

### `server/router/api/v1/agent/observer.go`
- Removed local `func estimateTokens(s string) int { return len(s) / 4 }`.
- Replaced 3 call sites: lines 198, 216, 541 → `EstimateTokens`.

### `server/router/api/v1/agent/observer_buffer.go`
- Replaced `len(newObservations) / 4` → `EstimateTokens(newObservations)` at line 221.

### `server/router/api/v1/agent/fusion_engine.go`
- Replaced 2 call sites: lines 94, 212 → `EstimateTokens`.

### `server/router/api/v1/agent/service.go`
- Replaced 3 call sites: lines 2399, 2743, 2756 → `EstimateTokens`.

### `server/router/api/v1/agent/processor.go`
- Uses `EstimateTokens` at 5 sites (lines 173, 187, 760, 777, 857, 879). No local copy exists.

### `server/router/api/v1/agent/chunker_test.go`
- 4 new tests (all passing):
  - `TestChunkerNoTerminatorParagraph` — comma-separated content with no sentence boundaries; verifies `splitByHardLimit` + final guard produce ≤ maxTokens (first chunk) or maxTokens+overhead (subsequent).
  - `TestChunkerNoH2Headers` — unstructured prose; verifies `splitByParagraphs` + guard produce correct sizes.
  - `TestChunkerOverlapSafe` — multi-section content; verifies overlap inflation stays within `maxTokens + ChunkOverlapTokens*2`.
  - `TestChunkerGuardCatchesOversized` — large header-section with no sentence terminators; verifies guard catches oversized chunks from the escape hatch.
- All tests call `InitTokenizer("test", "text-embedding-3-small")` before running.

### `server/router/api/v1/agent/observer_test.go`
- `TestEstimateTokens` — updated to use real tokenizer (previously `len/4` heuristic). Expected values confirmed against cl100k_base:
  - `""` → 0, `"test"` → 1, `"test test"` → 2, `"hell"` → 1, `"hello"` → 1, longer text → 13, `"hello世界"` → 4.

### `go.mod`
- Added: `github.com/tiktoken-go/tokenizer v0.8.0`

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Global singleton tokenizer** | Tokenizer initialization allocates internal tables; one instance for the process lifetime. Thread-safe reads from `Count()`. |
| **Binary search in splitByHardLimit** | O(log n) per split point vs O(n) for linear scan. Content is rune-iterated once for each binary search probe; total cost is O(k log n) where k is number of split points. |
| **Guard before addChunkOverlap** | Overlap prepends ~200 chars (~50 tokens for prose, ~80 for dense). If the guard ran after overlap, it would need extra tuning. By running before, the guard guarantees raw chunks ≤ maxTokens exactly. |
| **Test tolerance for overlap** | Chunks after the first have overlap prepended, inflating them ~50–80 tokens. The real constraint is the embedding model's 8192 limit, not the 512 quality target. Tests allow 100 tokens overhead for subsequent chunks. |
| **len/4 fallback** | If `tokenizer.Get(encName)` fails (e.g., missing binary blob in edge environment), the system degrades gracefully to the heuristic rather than panicking. |
| **No concurrent init guard** | `InitTokenizer` is called once during startup from `NewVectorDB`. Tests call it serially in `init` or `TestMain`. No mutex needed. |

## Edge Cases Covered

- **Empty content**: `EstimateTokens("")` returns 0. `ChunkMarkdownContent` returns nil for empty/whitespace-only content.
- **Single rune exceeding maxTokens**: `splitByHardLimit` falls back to emitting single runes.
- **No sentence terminators**: Falls through paragraph → sentence → `splitByHardLimit`.
- **No H2/H3 headers**: Treated as single section, processed via `splitByParagraphs`.
- **Script/style boilerplate**: Stripped by `CleanRAGSourceContent` before chunking.
- **Garbage chunks**: Filtered by `IsGarbageChunk` before the final guard.
- **CJK unicode**: cl100k_base handles CJK at ~1 token/char (for `hello世界`, 4 tokens = 1 for "hello" + 3 for "世界").
- **Tokenizer init failure**: Silently falls back to `len/4` with a `slog.Warn`.

---

## Adversarial Code Review Prompt

```
You are performing an adversarial code review of a Go implementation that replaces
a len/4 token heuristic with a real cl100k_base tokenizer for document chunking.

Review the following aspects:

1. **Chunk correctness** — Does `splitByHardLimit` guarantee every returned part is
   ≤ `maxTokens`? Are there any off-by-one errors in the binary search
   (lo/hi bounds, lo-1 split index)?

2. **Tokenizer initialization** — `InitTokenizer` is called once at startup and
   guarded by `if globalTokenizer != nil { return }`. Is there any path where
   it is called with a different model after initialization (and incorrectly
   skipped)? Could tests interfere via shared global state?

3. **Overlap safety** — The final guard runs before `addChunkOverlap`, and overlap
   adds `overlapTokens*4` chars (~50-80 tokens). Can any production scenario
   produce overlap >100 tokens (e.g., CJK-heavy content at 1 char/token)? Should
   the overlap use the real tokenizer instead of `len*4`?

4. **Concurrent safety** — `globalTokenizer` is a package-level var.
   `tokenizer.Codec.Count()` is documented as thread-safe, but is there any race
   between `InitTokenizer` (write) and `EstimateTokens` (read) during startup?

5. **Edge cases** — Examine:
   - Content with only newlines/whitespace
   - Content where every rune exceeds maxTokens
   - Content with null bytes
   - Extremely long single-line content (>1MB)
   - Unicode combining characters (runes that decompose to multiple code points)

6. **Regression check** — `EstimateTokens` replaced `len/4` in 14+ call sites
   across 6 files. Are any callers depending on the old heuristic's specific
   return values (e.g., comparison against a threshold tuned for len/4)?

7. **Observer test expectations** — `TestEstimateTokens` asserts exact token counts
   from cl100k_base. Are these values stable across tiktoken-go versions? Should
   the test use a tolerance range instead?

8. **Processor.go usage** — `processor.go` uses `EstimateTokens` at 6 sites but
   never imports `globalTokenizer` directly. Confirm the call resolves to the
   package-level function (not a local copy).

Return findings as:
- **CRITICAL**: Would cause incorrect behavior or data loss
- **WARNING**: Potential issue under specific conditions
- **INFO**: Style/readability suggestions
```
