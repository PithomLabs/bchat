# Plan 7: Enforce the Embedding Model's Hard Token Limit at the API Boundary

> Status: PROPOSED — no code changes yet. Awaiting approval.
> Supersedes the band-aid approach of plans 1–6 (tokenizer swap, recursive split, pre-embed guard).

---

## 1. Investigation findings (evidence-based)

### 1.1 The markdown reindex path is provably safe

`combined_files.txt` is a 14.5 MB combined text dump (`/home/chaschel/Desktop/biz/combined_files.txt`):
- 364,860 lines, 14,586,081 bytes
- 4 lines longer than 50,000 chars (longest = 103,186 chars)
- 7,447 `---` delimiter lines, 6,563 `## ` headers

The chunker (`ChunkMarkdownContent` → `splitContent` → H2 → H3 → paragraph → sentence → `splitByHardLimit`) was replicated with the real `cl100k_base` tokenizer and run against the file:

```
total parts: 66356
max REAL tokens in any part: 512
max chars in any part: 4859
parts exceeding 8192 real tokens: 0
```

**Conclusion:** `ChunkMarkdownContent` (capped at 512 tokens) → `expandAndValidateBatch` → `Embed` **cannot** produce an 8192-token input for this file. No code path in the repo embeds raw large text outside the chunked `Insert` paths (QA generation uses the LLM, not embeddings; observations embed per-line chunks). The only production `Embed` callers are the chunk `Insert`/`InsertWithCheckpoint` paths.

### 1.2 What this means for the reported error

With the code currently on disk, the error
```
failed at batch 1: batch 1 failed with permanent error: ...
OpenRouter API error: HTTP 400: ... "maximum input length is 8192 tokens."
```
is **impossible for a KB reindex of this file.** The error therefore originates from one of:

- **(A) A stale binary** — `build/memos` was built before `expandAndValidateBatch`
  (vectordb_lance.go:622) and the recursive `splitByHardLimit` landed (plans 1–6 were
  written but may not be in the running binary). **This is the most likely actual cause.**
- **(B) Any embedding path that bypasses the 512-token markdown chunker** — a future
  ingest route, a different content type, or a raised chunk size. The guards are
  content-type-specific and live upstream of the embedding call, so they are easy to bypass.

---

## 2. The underlying problem (why plans 1–6 were band-aids)

Plans 1–6 tweaked the *chunker's* heuristics. The real architectural flaw is that **the
embedding model's hard input limit is never enforced at the single place that knows it.**

| Layer | What it does | Why it's fragile |
|-------|--------------|------------------|
| Chunker quality cap (512) | Splits content for embedding *quality* | A quality heuristic, unrelated to the 8192 API limit. If raised or bypassed, the limit is approached. |
| `MaxEmbeddingInputTokens = 8000` | Magic constant in `expandAndValidateBatch` | Hardcoded, disconnected from the actual model; duplicated knowledge. |
| `EstimateTokens` | Counts tokens via `cl100k_base`, **silently falls back to `len/4`** if the tokenizer isn't initialized | Silent undercount masks tokenizer-init failure and can mis-size chunks. |
| `OpenRouterEmbedding.doEmbed` | Sends whatever it's given | **Has zero knowledge of the limit.** No enforcement at the boundary. |
| Batch error handling | HTTP 400 is non-retryable (`isRetryableError`) | One bad item aborts the **entire** reindex permanently with an opaque message. No per-item isolation, no diagnostics naming the chunk. |

The system "works" only because 512 (quality) << 8192 (limit) — a coincidental margin, not a guarantee.

---

## 3. The fix: enforce at the boundary (authoritative + resilient)

### Fix 1 — Authoritative model limit (single source of truth)

File: `embedding.go`

Add a function deriving the real limit from the model name, reusing the existing
model-name switch in `getOpenRouterDimension`:

```go
// modelMaxInputTokens returns the embedding model's hard input token limit.
// This is the single authoritative source; the chunker/guard must derive from it.
func modelMaxInputTokens(model string) int {
    switch {
    case strings.Contains(model, "text-embedding-3-small"),
        strings.Contains(model, "text-embedding-3-large"),
        strings.Contains(model, "text-embedding-ada-002"):
        return 8191
    case strings.Contains(model, "qwen3-embedding-8b"):
        return 32768
    default:
        return 8191 // OpenAI default; safe conservative value
    }
}
```

Expose via the `EmbeddingService` interface and compute once at construction:

```go
type EmbeddingService interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int
    Provider() string
    MaxInputTokens() int // NEW: authoritative hard limit for this model
}
```

`OpenRouterEmbedding` stores `maxInputTokens` from `modelMaxInputTokens(model)` in `NewOpenRouterEmbedding`.

### Fix 2 — Enforce at `doEmbed` (the only correct boundary)

File: `embedding.go` — `OpenRouterEmbedding.doEmbed`

Before sending the batch, for each input string:
1. Count exact tokens with an **authoritative** counter owned by the embedding service.
2. If `tokens > maxInputTokens - safetyMargin` (margin ~ 16 tokens), split that *single*
   input via `splitByHardLimit` into sub-inputs each ≤ limit.
3. Embed the sub-inputs; **average** their vectors back into one vector for the original
   index position (preserves the 1:1 chunk→vector mapping the rest of the pipeline
   assumes). *Alternative:* store N separate rows keyed by `{id}_p{n}` for finer
   retrieval — note as a follow-up, not required for this fix.

This guarantees no >limit input ever reaches the API, **regardless of which caller fed it.**

### Fix 3 — Replace the hardcoded 8000

File: `chunker.go` (`MaxEmbeddingInputTokens`) and `vectordb_lance.go` (`expandAndValidateBatch`)

`expandAndValidateBatch` should read `db.embedSvc.MaxInputTokens()` (minus margin) instead of
the magic `8000`. Keep it as defense-in-depth only — Fix 2 is the primary guard. The
`MaxEmbeddingInputTokens` constant becomes `modelLimit - margin` at runtime (or is removed in
favor of the service value).

### Fix 4 — Make `EstimateTokens` fail-loud

File: `chunker.go` — `EstimateTokens`

Remove the silent `len/4` fallback as the default RAG path. If `globalTokenizer == nil` at the
first `EstimateTokens` call in a RAG context, either (a) initialize on-demand from the embedding
config, or (b) return an error / log an `ERROR` (not `Warn`) so the caller refuses rather than
silently undercounting. The `len/4` fallback may remain only for non-RAG/test contexts, with a
clear one-time `ERROR` log.

### Fix 5 — Per-item isolation in batching

File: `vectordb_lance.go` — `Insert` and `processSingleBatch`

When an individual embedding input is still rejected by the API (genuinely un-splittable — e.g.
a single token > limit, which is impossible for text but possible in theory), **skip that one
chunk**, log its `tenantID, chunkID, title, realTokenCount, modelLimit`, and continue — instead
of returning a permanent error that aborts the whole reindex. The per-chunk loop already exists;
wrap each `Embed` so one bad item does not fail the batch.

### Fix 6 — Diagnostics

On any truncation/skip, log: `tenantID`, `chunkID`, `title`, `realTokenCount`, `modelLimit`.
This makes the root cause always visible and turns an opaque fatal failure into an actionable,
recoverable one.

---

## 4. Step 0 — likely your actual fix

Rebuild and redeploy. Confirm the running `build/memos` includes `expandAndValidateBatch`
(vectordb_lance.go:622) and the recursive `splitByHardLimit`. The current code on disk already
prevents this error for a markdown/KB reindex; the reported error strongly implies a **stale
binary**. Plan 7 hardens the boundary so the failure mode is impossible even for non-markdown or
future ingest paths.

---

## 5. Files to modify

| File | Change |
|------|--------|
| `embedding.go` | `modelMaxInputTokens(model)`; `MaxInputTokens()` on interface + impls; authoritative count in `doEmbed` with split+average; fail-loud tokenizer |
| `vectordb_lance.go` | `doEmbed` boundary enforcement; `expandAndValidateBatch` uses `MaxInputTokens()`; per-item isolation in `Insert`/`processSingleBatch`; diagnostics |
| `chunker.go` | `MaxEmbeddingInputTokens` sourced from model limit; `EstimateTokens` fail-loud |
| `vectordb.go` | `InitTokenizer` fail-loud / on-demand init |

---

## 6. Verification

1. **Unit test (boundary):** feed a synthetic >8192-token string directly to
   `OpenRouterEmbedding.Embed` against a mock server; assert it is split and succeeds (no 400).
2. **Regression:** reindex `combined_files.txt`; assert no HTTP 400 and logs show **0**
   oversized/skipped chunks (proves the markdown path is fine) while the boundary guard would
   catch any future regression.
3. **Per-item isolation:** inject one oversized chunk into a batch; assert the batch completes
   with a warning and the rest of the chunks are indexed.
4. `go build -tags rag ./server/router/api/v1/agent/`
5. `go vet -tags rag ./server/router/api/v1/agent/...`
6. `go test -v -run 'TestChunker|TestEstimateTokens|TestEmbedding' ./server/router/api/v1/agent/`

---

## 7. Why this is the "underlying" fix, not a band-aid

- Plans 1–6 assumed the problem was *chunk sizing* and pushed guards further upstream.
- Plan 7 recognizes the limit belongs to the **embedding contract**, enforced at the **API
  boundary** by the component that owns the model config — immune to stale binaries, new ingest
  paths, tokenizer fallbacks, model changes, and raised chunk sizes.
- It also removes the fatal, opaque failure mode (whole-batch abort on one bad item) and adds
  diagnostics, so the system degrades gracefully and is debuggable.
