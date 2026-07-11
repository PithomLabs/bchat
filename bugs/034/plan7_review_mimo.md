# Plan 7 Review — Adversarial Review

**Reviewer:** mimo-v2.5-free
**Date:** 2026-07-11
**Verdict:** **APPROVED WITH NITS** — Strongest plan yet; addresses root cause, not symptoms

---

## What's different about Plan 7

Plans 1–6 treated the problem as "chunks are too big." Plan 7 proves they aren't, and reframes the problem as "the embedding contract has no enforced boundary." This is a fundamentally better framing.

### Section 1: Investigation findings — Compelling

The replicated chunker run against `combined_files.txt` is the strongest evidence in any plan:

```
total parts: 66356
max REAL tokens in any part: 512
parts exceeding 8192 real tokens: 0
```

This **proves** the markdown reindex path cannot produce the reported error. The conclusion that the binary is stale or the error comes from a different code path is well-supported.

**Verified:** `expandAndValidateBatch` already exists at `vectordb_lance.go:622` (from Plan 6 implementation). `MaxEmbeddingInputTokens = 8000` exists at `chunker.go:73`. Plans 1–6 were implemented. The error therefore implies a stale binary or a non-markdown ingest path.

### Section 2: Architectural critique — Accurate

The table diagnosing why plans 1–6 were band-aids is precise:

| Layer | Fragility |
|-------|-----------|
| Chunker quality cap (512) | Quality heuristic, not limit enforcement |
| `MaxEmbeddingInputTokens = 8000` | Magic constant, disconnected from model |
| `EstimateTokens` | Silent `len/4` fallback masks init failure |
| `doEmbed` | Zero knowledge of the limit |
| Batch error handling | One bad item aborts entire reindex |

The observation that "the system works only because 512 << 8192 — a coincidental margin, not a guarantee" is the key insight.

---

## Part-by-part assessment

### Fix 1 (Authoritative model limit) — Approved

Good: Single source of truth derived from model name. Reuses existing model-name switch pattern.

**Nit 1: Why 8191 and not 8192?**

`modelMaxInputTokens` returns `8191` for OpenAI models. The OpenRouter limit is `8192`. Why the off-by-one? If this is a safety margin, it should be documented. If it's a mistake, it should be `8192`. The `safetyMargin` parameter in Fix 2 already provides headroom — the model limit itself should be accurate.

**Suggestion:** Return `8192` and let `doEmbed`'s `safetyMargin` handle the margin. Or document why 8191 is intentional.

### Fix 2 (Enforce at `doEmbed`) — Approved

This is the core fix and it's architecturally correct. The embedding service is the only component that knows the model's actual limit.

**Nit 2: Vector averaging is a semantic change**

The plan says "average their vectors back into one vector for the original index position." This produces a different embedding than embedding the original content as a whole. For a chunk split into 3 parts, the averaged vector is the centroid of 3 local embeddings, not the global embedding of the full text.

This is acceptable as a safety net (the alternative — storing N separate rows — is noted as a follow-up). But it should be documented as a tradeoff: the averaged vector may have different retrieval characteristics than a hypothetical full-text embedding.

**Nit 3: safetyMargin = 16 is unexplained**

Why 16 tokens? The title overhead is ~20 tokens, the overlap is ~50 tokens. If the safety margin is meant to cover title overhead, 16 is too small. If it's meant to cover subword variation, 16 is generous. The plan should justify this number or derive it from the actual overhead sources.

### Fix 3 (Replace hardcoded 8000) — Approved

Good: `expandAndValidateBatch` reads `MaxInputTokens()` from the service instead of the constant. Defense-in-depth becomes dynamic.

**No nits.**

### Fix 4 (Fail-loud `EstimateTokens`) — Approved with one concern

Good: Remove the silent `len/4` fallback for RAG contexts. Fail immediately rather than silently undercounting.

**Nit 4: On-demand initialization requires config access**

The plan says "initialize on-demand from the embedding config." Currently `EstimateTokens` is a standalone function with no access to the embedding config. To initialize on-demand, it would need:
- A package-level reference to the `EmbeddingConfig`, or
- A method on a struct that holds the config, or
- A `sync.Once` initializer that takes the config at first call

The plan should specify which approach. The simplest is a `sync.Once` + package-level config set during `NewVectorDB`.

### Fix 5 (Per-item isolation) — Approved

Good: Skip one bad chunk instead of aborting the entire reindex. This is the resilience fix.

**Nit 5: "genuinely un-splittable" is theoretical**

The plan says "a single token > limit, which is impossible for text but possible in theory." In practice, Fix 2's split ensures no text exceeds the limit. Per-item isolation is a safety net for hypothetical edge cases. This is fine — defense in depth — but the plan should be clear that Fix 2 should prevent this from ever triggering.

### Fix 6 (Diagnostics) — Approved

Good: Log tenantID, chunkID, title, realTokenCount, modelLimit on any truncation/skip.

**No nits.**

### Step 0 (Rebuild and redeploy) — Approved

Good: Practical advice. The current code on disk already prevents the error for markdown reindex. The stale binary theory is the most likely actual cause.

**Nit 6: Verify the binary includes Plan 6 changes**

The plan says "Confirm the running build/memos includes expandAndValidateBatch." But the code on disk already has it (vectordb_lance.go:622). The user should verify the running binary, not just the source. A `strings build/memos | grep expandAndValidateBatch` would confirm.

---

## Relationship to Plans 1–6

The plan says "Supersedes the band-aid approach of plans 1–6." This is slightly misleading — Plans 1–6 are already implemented and provide defense-in-depth. Plan 7 builds on top of them, not replaces them. The `expandAndValidateBatch` from Plan 6 is kept (Fix 3 makes it dynamic). The correct framing is:

- Plans 1–6: Upstream guards (chunker + pre-embed expansion) — defense-in-depth
- Plan 7: Boundary enforcement (doEmbed) — primary guard

Both layers are needed. The upstream guards reduce the frequency of oversized inputs reaching the boundary. The boundary guard catches anything that slips through.

---

## Test plan assessment

The verification plan is comprehensive:

1. **Unit test (boundary):** Feed synthetic >8192-token string to `OpenRouterEmbedding.Embed` against a mock server. This is the critical test — it proves the boundary guard works.
2. **Regression:** Reindex `combined_files.txt`; assert 0 oversized/skipped chunks. Proves the markdown path is fine.
3. **Per-item isolation:** Inject one oversized chunk; assert batch completes with warning. Proves resilience.
4. **Build/vet/test:** Standard quality gates.

**Nit 7: Add a test for tokenizer fail-loud**

Fix 4 makes `EstimateTokens` fail-loud when the tokenizer isn't initialized. Add a test that calls `EstimateTokens` without initializing the tokenizer and asserts it returns an error or logs an ERROR (not just a WARN).

---

## Summary

| Component | Verdict | Action |
|-----------|---------|--------|
| Investigation findings | **Approved** | None |
| Architectural critique | **Approved** | None |
| Fix 1 (model limit) | **Approved with nits** | Explain 8191 vs 8192 |
| Fix 2 (doEmbed enforcement) | **Approved with nits** | Document vector averaging tradeoff; justify safetyMargin |
| Fix 3 (dynamic limit) | **Approved** | None |
| Fix 4 (fail-loud tokenizer) | **Approved with nits** | Specify on-demand init mechanism |
| Fix 5 (per-item isolation) | **Approved** | None |
| Fix 6 (diagnostics) | **Approved** | None |
| Step 0 (rebuild) | **Approved with nits** | Verify binary, not just source |
| Test plan | **Approved with nits** | Add tokenizer fail-loud test |

**Recommendation:** Approve and implement. This is the right architectural fix. Deploy Step 0 first (likely resolves the immediate error). Then implement Fixes 1–6 in order. The nits are documentation-level and can be addressed during implementation.
