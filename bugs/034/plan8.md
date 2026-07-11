# Plan 8: Enforce the Embedding Model's Hard Token Limit at the API Boundary (consolidated)

> Status: **IMPLEMENTED & VERIFIED** (2026-07-11). Both reviews' nits + 3 mandatory reworks resolved.
> Supersedes plans 1–6 as the authoritative boundary; plans 1–6 remain defense-in-depth.

---

## 0. Context (from investigation)

`combined_files.txt` (14.5 MB combined text dump) was reproduced through the chunker with the
real `cl100k_base` tokenizer:

```
total parts: 66356
max REAL tokens in any part: 512
parts exceeding 8192 real tokens: 0
```

The markdown/KB reindex path **cannot** produce an 8192-token input with the code on disk.
The reported error therefore comes from either (a) a **stale binary** (Plans 1–6 not in the
running `build/memos`), or (b) an embedding path that bypasses the 512-token markdown chunker.

The underlying architectural flaw: the embedding model's hard input limit (8192) is **never
enforced at the only place that knows it** — `OpenRouterEmbedding.doEmbed`. It is delegated to
fragile, disconnected heuristics (512 quality cap, magic `8000` constant, silent `len/4`
fallback), and one bad item aborts the entire batch permanently with an opaque message.

---

## 1. Relationship to plans 1–6 (re-framed)

- **Plans 1–6:** defense-in-depth at the *symptom* level (chunker cap + pre-embed
  `expandAndValidateBatch`). Keep them.
- **Plan 8:** authoritative *root-cause* enforcement at the embedding boundary + graceful
  failure. Both layers required.

---

## 2. Changes

### R1 — `modelMaxInputTokens` returns accurate limits
File: `embedding.go`. Reuse the existing model-name switch (mirror `getOpenRouterDimension`).

| Model | Limit |
|-------|-------|
| text-embedding-3-small / -large / ada-002 | `8192` |
| qwen3-embedding-8b | `32768` |
| default | `8192` |

Return the accurate limit (not 8191); the `safetyMargin` provides headroom. The qwen limit is
best-effort; the API is the final authority.

Expose `MaxInputTokens() int` on the `EmbeddingService` interface and compute it once at
construction in `OpenRouterEmbedding` (and `LocalEmbedding`/`MockEmbedding` return
`math.MaxInt32` = unlimited).

### R2 — split-and-average MUST re-normalize (High)
File: `embedding.go` — `doEmbed`. When an input exceeds the limit, split via `splitByHardLimit`,
embed the sub-inputs, average their vectors, and **renormalize**: `V = V_avg / ||V_avg||`.
(N-row storage `{id}_pN` is an alternative for finer retrieval — deferred.)

### R3 — per-item isolation mechanism (High)
File: `vectordb_lance.go` — `processSingleBatch`. The OpenRouter batch API returns HTTP 400 for
the whole batch if any item exceeds the limit (no per-item info). So isolation is at the
`processSingleBatch` level:
1. `batch = expandAndValidateBatch(batch)` (defense-in-depth, now dynamic — R5)
2. `Embed(batch)` (batched for performance)
3. **On failure**, retry embedding chunks **one-by-one**; skip any chunk that still errors,
   logging `tenantID, chunkID, title, realTokenCount, modelLimit`. The rest are indexed.

`InsertWithCheckpoint` is covered because it routes through `processSingleBatch` →
`processBatchWithRetry`.

### R4 — `EstimateTokens` fail-loud, reconciled with Plan 6 (Med)
File: `chunker.go`. Plan 6 added `fallbackWarnOnce` + `slog.Warn`. Resolution:
- Change `slog.Warn` → `slog.Error` (non-breaking; achieves fail-loud).
- Capture the `EmbeddingConfig` package-level in `NewVectorDB` (`vectordb.go`) and let
  `EstimateTokens` **self-heal via `sync.Once`** if the tokenizer was missed, so it never silently
  uses `len/4` in RAG/prod. `len/4` remains only for non-RAG/test contexts with an `ERROR` log.

### R5 — `expandAndValidateBatch` consumes the dynamic limit (Nit)
File: `vectordb_lance.go`. Obtain the limit via type assertion:
`if ore, ok := db.embedSvc.(*OpenRouterEmbedding); ok { limit = ore.MaxInputTokens() }`.
For `LocalEmbedding`/`MockEmbedding` the limit is `math.MaxInt32` (unlimited) → validation skipped.
`MaxEmbeddingInputTokens` in `chunker.go` becomes `modelLimit - safetyMargin` at runtime (or is
removed in favor of the service value).

### R6 — `safetyMargin` justified
`margin = 16` tokens. Covers only: (a) discrepancy between the local `cl100k_base` count and the
API's count for the same model (negligible), and (b) the `splitByHardLimit` binary-search
off-by-one. It does NOT need to cover title/overlap, because at `doEmbed` the input is already the
full `Title + ": " + Content` string, and `expandAndValidateBatch` already accounted for title
overhead upstream.

### R7 — `doEmbed` boundary enforcement detail
`doEmbed` expands the batch internally: for each input, if
`EstimateTokens(input) > MaxInputTokens() - safetyMargin`, replace it with N sub-inputs via
`splitByHardLimit`; after `Embed`, collapse N → 1 averaged+renormalized vector, preserving the 1:1
index mapping (OpenRouter returns embeddings by `index`). Guarantees no >limit input reaches the
API regardless of caller.

### R8 — Step 0 first (highest value)
Rebuild/redeploy; confirm the running `build/memos` includes Plan 6 changes:
`strings build/memos | grep expandAndValidateBatch`. If Step 0 resolves the error, still implement
R3+R6 (Fix 5+6) for operator resilience; Fixes 1–4 become completeness hardening.

### R9 — scope note
Investigation is scoped to `combined_files.txt` + KB reindex path. Other RAG ingest routes that
bypass the markdown chunker need separate verification.

---

## 3. Files to modify

| File | Change |
|------|--------|
| `embedding.go` | `modelMaxInputTokens(model)`; `MaxInputTokens()` on interface + all impls; authoritative count + split+average+renormalize in `doEmbed`; fail-loud `EstimateTokens` (Error + on-demand init) |
| `vectordb_lance.go` | `doEmbed` enforcement (R7); `expandAndValidateBatch` uses `MaxInputTokens()` via type assertion (R5); per-item isolation in `processSingleBatch` (R3); diagnostics (tenantID, chunkID, title, realTokenCount, modelLimit) |
| `chunker.go` | `MaxEmbeddingInputTokens` sourced from model limit; `EstimateTokens` fail-loud/on-demand (R4) |
| `vectordb.go` | Capture `EmbeddingConfig` package-level in `NewVectorDB`; `InitTokenizer` fail-loud |

---

## 4. Verification

1. **Boundary test (NEW):** mock HTTP server; feed a synthetic >8192-token string directly to
   `OpenRouterEmbedding.Embed`; assert it is split and succeeds (no 400).
2. **Per-item isolation test (NEW):** mock embed API that 400s on one specific item; assert the
   batch completes with a warning and the other items are indexed.
3. **Tokenizer fail-loud test (NEW):** `EstimateTokens` without init logs `ERROR` (not just WARN).
4. **Regression:** reindex `combined_files.txt`; assert no HTTP 400 and 0 oversized/skipped chunks.
5. `go build -tags rag ./server/router/api/v1/agent/`
6. `go vet -tags rag ./server/router/api/v1/agent/...`
7. `go test -v -run 'TestChunker|TestEstimateTokens|TestEmbedding' ./server/router/api/v1/agent/`

## 5. Implementation notes (2026-07-11)

- `embedding.go`: added `modelMaxInputTokens`, `MaxInputTokens()` on the interface +
  all three impls (`OpenRouterEmbedding` stores `maxInputTokens`; local/mock return
  `math.MaxInt32`), refactored `doEmbed` into boundary enforcement (expand → `doEmbedRaw`
  → collapse+average+renormalize) with `embedSafetyMargin = 16`. Added `SetEstimateTokenizerConfig`
  + `maybeInitTokenizer` for on-demand `EstimateTokens` self-heal.
- `chunker.go`: `EstimateTokens` now self-heals and logs `ERROR` (not `Warn`) on fallback.
- `vectordb.go`: `NewVectorDB` captures config via `SetEstimateTokenizerConfig`.
- `vectordb_lance.go`: `expandAndValidateBatch(batch, limit)` now uses the dynamic
  `embeddingLimit(embedSvc)` (type-asserts `*OpenRouterEmbedding`); added `embedWithIsolation`
  (per-item fault isolation in `processSingleBatch`) + `firstErr` helper.
- Tests added: `TestOpenRouterEmbeddingSplitsOversizedInput` (boundary split + renormalize
  magnitude ≈ 1), `TestEstimateTokensFailLoud`, `TestEstimateTokensSelfHeals`,
  `TestEmbedWithIsolationSkipsFailedItem`; `mockEmbeddingService` gained `MaxInputTokens()`.

### Step 0 still applies
The reported error most likely came from a **stale binary** (Plans 1–6 not in the running
`build/memos`). After rebuilding with these changes, reindex `combined_files.txt`; the markdown
path is already safe (verified: max 512 real tokens/chunk, 0 over 8192), and the boundary guard
now makes any oversized input impossible regardless of caller, with per-item isolation so one bad
chunk can never abort the whole reindex.
