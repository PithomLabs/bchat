# Plan 7 Review — Adversarial Review

**Reviewer:** deepseek
**Date:** 2026-07-11
**Verdict:** **APPROVED WITH NITS** — Architecture is sound, but 3 gaps need resolution before implementation

---

## Verdict Summary

| Component | Verdict | Action |
|-----------|---------|--------|
| Section 1 (Investigation) | **Approved** | Correct conclusion re: single-file test |
| Section 2 (Problem diagnosis) | **Approved** | Architectural analysis is correct |
| Fix 1 (modelMaxInputTokens) | **Approved with nit** | 8191 vs 8192; not implemented yet |
| Fix 2 (doEmbed enforcement) | **Approved with gap** | Split-and-average loses normalization; per-item API error handling unclear |
| Fix 3 (replace 8000) | **Approved** | Runtime value > magic constant |
| Fix 4 (EstimateTokens fail-loud) | **Rework needed** | Conflicts with Plan 6 `sync.Once`; needs reconciliation |
| Fix 5 (per-item isolation) | **Rework needed** | Vague on mechanism — Embed sends a batch, gets batch results; no per-item error isolation exists |
| Fix 6 (Diagnostics) | **Approved** | Standard good practice |

---

## What's Good

### 1. The architectural diagnosis is correct (Section 2)

The table in Section 2 correctly identifies the real problem: **the embedding model's hard input limit is never enforced at the single place that knows it.** Every prior fix pushed the guard further upstream (chunker → `expandAndValidateBatch`), but none made `doEmbed` itself enforce the limit. This is the right final boundary.

### 2. `modelMaxInputTokens` follows the right pattern

Mirroring `getOpenRouterDimension` (embedding.go:152) with a model-name switch is the correct approach. It keeps the limit logic co-located with the model detection logic, which is already tested and maintained.

### 3. Step 0 is the strongest recommendation

> "Rebuild and redeploy. Confirm the running binary includes `expandAndValidateBatch` and `splitByHardLimit`."

This is correct and should be the first action. The investigation in Section 1.1 proves the KB reindex path is safe for `combined_files.txt` with the current code. The reported error **is** most likely a stale binary. Step 0 should be performed **before** any Plan 7 code changes — if it fixes the error, the urgency of Plan 7 drops to "nice-to-have hardening."

### 4. Fix 5 (per-item isolation) is the right direction

The "one bad chunk aborts the entire reindex" failure mode is the most painful. Making it degrade gracefully is the highest-impact improvement for operator experience.

---

## Gaps Requiring Rework Before Implementation

### Gap 1: Fix 2 — split-and-average loses vector normalization

**Claim:** "Embed the sub-inputs; average their vectors back into one vector for the original index position."

**Problem:** Averaging sub-vectors does NOT produce the same vector as embedding the full text. Worse, the plan doesn't mention re-normalization after averaging. An un-normalized average of normalized vectors breaks the cosine similarity assumption downstream.

**Concrete example:**
- Full text gets vector V (unit length, ||V|| = 1)
- Split into A and B, embedded separately → vectors V_a, V_b (each unit length)
- Average: V_avg = (V_a + V_b) / 2 → ||V_avg|| < 1 (not unit length)
- Cosine similarity with other vectors will be incorrectly scaled

**Required fix:** Normalize the averaged vector: V_result = V_avg / ||V_avg||. Add this step explicitly.

**Better alternative (deferred):** Store N separate rows with IDs `{chunkID}_p1`, `{chunkID}_p2`, etc. This preserves the model's full precision without approximation. The retrieval side would need a small adjustment (group by prefix after search). The plan already mentions this as an alternative — recommend promoting it to the primary approach and marking split-and-average as the follow-up, since this guard should almost never trigger.

### Gap 2: Fix 2 — per-item API error handling is underspecified

**Claim:** "When an individual embedding input is still rejected by the API... skip that one chunk."

**Problem:** The current `doEmbed` (embedding.go:381) sends a **batch** of texts and gets a **batch** of embeddings back. The OpenRouter API does not provide per-item error reporting in the standard OpenAI-compatible response format. If one text in the batch exceeds the limit, the entire request returns HTTP 400. There is no way to tell WHICH item failed or to recover the successful items.

To implement per-item isolation, you cannot do it at the Embed batch level. You'd need to either:
- **(a)** Embed each chunk individually (one text per API call) — defeats batching, increases latency and cost by ~10-25x
- **(b)** Implement per-item isolation at the `processSingleBatch` level: for each chunk, try embedding individually on failure, skip the bad one — requires embedding service to expose per-item embedding
- **(c)** Parse the error message to identify the bad item (fragile, API-dependent)

**Required action:** The plan must pick a specific mechanism. Option (b) is the most practical: `processSingleBatch` → `expandAndValidateBatch` (defense-in-depth) → `Embed` (batch for performance) → on error, fall back to embedding chunks one-by-one, skipping failures with a log. This avoids the granularity problem entirely.

### Gap 3: Fix 4 — conflicts with Plan 6 (already implemented)

**Claim:** "Remove the silent `len/4` fallback as the default RAG path."

**Problem:** Plan 6 (already deployed) added `fallbackWarnOnce` to `EstimateTokens`:
```go
fallbackWarnOnce.Do(func() {
    slog.Warn("EstimateTokens using len/4 fallback — globalTokenizer not initialized")
})
```
This logs a one-time `Warn`. Plan 7 wants to escalate to `ERROR` or fail.

**Required action:** Reconcile:
- Option A: Remove the Plan 6 `sync.Once` and replace with an `ERROR` log and explicit error return (breaking change — callers must handle the error)
- Option B: Keep the `sync.Once` but change `slog.Warn` → `slog.Error` (non-breaking, aligns with Plan 7's "fail-loud" goal)
- Option C: Remove `len/4` entirely and return 0 / error on uninitialized tokenizer

Option B is safest: change one line from `slog.Warn` to `slog.Error` and the goal is met without refactoring callers.

---

## Nits

### Nit 1: 8191 vs 8192 — the 1-token margin is undocumented

The `modelMaxInputTokens` function returns `8191` for OpenAI models. The documented limit is `8192` (`max_input` for text-embedding-3-small). Why the -1? If it's a safety margin, document it. If it's a typo, fix to `8192`. The "title separator may add 1 token" would be a valid reason, but needs a comment.

### Nit 2: investigation tests only ONE file (Section 1.1)

The test against `combined_files.txt` proves the KB reindex path is safe **for that specific file**, not universally. The conclusion "cannot produce an 8192-token input for this file" is correct. But "No code path in the repo embeds raw large text outside the chunked Insert paths" is a broader claim that the test doesn't cover. The OM observer path (embeds per-line chunks — each line is trivially under 8192) and the QA generation path (uses LLM, not embeddings) are mentioned but not verified with the test harness.

Add a note: "Investigation scoped to the reported file and KB reindex path. Other RAG ingest routes that skip the markdown chunker would need separate verification."

### Nit 3: `modelMaxInputTokens` doesn't handle `qwen3-embedding-8b`

The model `qwen3-embedding-8b` may or may not be available on OpenRouter. If it uses a different tokenizer (not cl100k_base), the 32768 token count is based on that model's tokenizer, which could encode differently. The plan should note that the token limit is a model-property best effort and the actual limit is enforced by the API.

### Nit 4: Plans 1–6 are called "band-aids" but are actually defense-in-depth

Section 2 says plans 1–6 were "band-aids." While the architectural argument is valid (they don't fix the root cause at the boundary), calling them band-aids understates their value:
- They prevent the error from reaching `doEmbed` in the first place
- They provide diagnostics when something goes wrong
- They are the first line of defense; Plan 7 is the last

Consider rephrasing: "Plans 1–6 addressed the symptom (chunks exceeding the chunker's own limit). Plan 7 addresses the cause (no enforcement at the embedding boundary). Both layers are needed for defense-in-depth."

### Nit 5: `MaxInputTokens()` on the interface is awkward for non-OpenRouter impls

`LocalEmbedding` and `MockEmbedding` don't have a model input token limit. If `MaxInputTokens()` returns 0 or some sentinel, callers like `expandAndValidateBatch` need to handle it. Either:
- Document that 0 = no limit (caller should skip validation)
- Return `math.MaxInt32` for unlimited models
- Keep it off the interface and only add it to `OpenRouterEmbedding`, with `expandAndValidateBatch` checking via type assertion

The type assertion approach is cleaner: `if ore, ok := db.embedSvc.(*OpenRouterEmbedding); ok { limit = ore.MaxInputTokens() }`.

### Nit 6: InsertWithCheckpoint path needs explicit mention

Fix 5 says it modifies `Insert` and `processSingleBatch`. The `InsertWithCheckpoint` path (vectordb_lance.go:476) also calls `processSingleBatch` via `processBatchWithRetry`. Since Fix 5's per-item isolation is in `processSingleBatch`, it's automatically covered. But this should be explicitly noted to avoid confusion.

---

## Implementation Order Recommendation

1. **Step 0 first:** Rebuild and redeploy. Confirm the stale binary hypothesis.
2. **If Step 0 fixes it:** Proceed with Fix 5 (per-item isolation) + Fix 6 (diagnostics) only — these are the high-impact operator experience improvements. Fixes 1–4 become "nice to have" for completeness.
3. **If Step 0 doesn't fix it:** Implement all fixes. Start with Fix 1 + 2 (the core boundary enforcement), then Fix 3 (consume from service), then Fix 4 (fail-loud tokenizer), then Fix 5 + 6.

---

## Test Impact Analysis

| Verification | Status | Notes |
|-------------|--------|-------|
| `TestChunker*` | ✅ No change | Fixes 1–3 don't touch chunker output; Fix 4 is logging-only |
| `TestEstimateTokens` | ⚠️ Minor change if Fix 4 adopted | If `len/4` is removed, tests using mock (no real tokenizer) need InitTokenizer called first |
| New: boundary test | ❌ Missing | Plan 7 Section 6 item 1: requires a mock HTTP server. No test code provided. |
| New: per-item isolation test | ❌ Missing | Plan 7 Section 6 item 3: requires mocking the embed API. No test code provided. |
| `go build -tags rag` | ✅ | No new CGO dependencies |
| `go vet -tags rag` | ✅ | No new vet issues expected |

---

## Summary

| # | Issue | Severity | Must fix before code? |
|---|-------|----------|-----------------------|
| 1 | Split-and-average lacks re-normalization (Fix 2) | **High** | Yes |
| 2 | Per-item isolation mechanism unspecified (Fix 5) | **High** | Yes |
| 3 | Fix 4 conflicts with deployed Plan 6 code | **Medium** | Yes |
| 4 | Interface design for `MaxInputTokens` | **Low** | No (nit) |
| 5 | 8191 vs 8192 margin undocumented | **Low** | No (nit) |
| 6 | "Band-aid" framing understates prior work | **Low** | No (nit) |
| 7 | Investigation scope limited to one file | **Low** | No (nit) |

**Recommendation:** **Approve with 3 mandatory reworks** (gaps 1–3). The architecture is sound and Plan 7 is the right direction. Fix gaps 1–3, adopt nits 4 and 5, then implement with the order recommendation in Section 5.
