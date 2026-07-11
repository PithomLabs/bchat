# Plan 5 Review — Adversarial Review

**Reviewer:** opencode
**Date:** 2026-07-11
**Verdict:** **REWORK** — Part A approved with nits; Part B has critical gaps

---

## Part A (Defense-in-Depth) — Approved with Nits

### What's good

- The core insight is correct: there is zero validation at the Embed call site, and Fix 1 (pre-embedding guard) is the right belt-and-suspenders approach.
- Fix 5 (tokenizer verification logging) is well-designed — the `sync.Once` fallback warning is the right pattern.
- Fix 6 (constant) is clean and appropriate.

### Nit 1: Fix 1 and Fix 2 have an unaddressed expansion problem

The plan says `expandAndValidateBatch` splits oversized chunks via `splitByHardLimit` and embeds them "as separate entries." But this raises two questions the plan doesn't answer:

1. **How do expanded parts map to the LanceDB schema?** Each `DocumentChunk` has one `ID`, one `Embedding`, one `Content`. If a chunk is split into 3 parts, we need 3 separate `DocumentChunk` entries with unique IDs, copied metadata, and one embedding each. The plan doesn't describe how these are constructed.

2. **What happens to batch size?** If the batch starts with 10 chunks and one expands to 3 parts, the embedded batch now has 12 entries but only 10 Arrow records (since the original chunk object is what gets written to LanceDB via `chunksToArrowRecord`). Either:
   - The expanded parts replace the original chunk (losing data), or
   - The batch slice must be mutated in-place (replacing the original chunk with the first part and appending the rest to a separate list that gets written separately).

   The plan should specify which approach and describe the data flow.

### Nit 2: Fix 3 changes guard semantics without documenting the tradeoff

Including the title in the guard means the guard now checks `EstimateTokens(title + ": " + content)` instead of `EstimateTokens(content)`. This is correct for catching embedding-time overflow, but it means:

- A chunk with 512 tokens of content and a 20-token title (532 total) would now be split, even though the content alone is fine.
- The split uses `splitByHardLimit` on the full text (title + content), which means the title is embedded as part of the first chunk's content, not as a prefix. This changes the embedding semantics.

The plan should acknowledge this tradeoff or propose an alternative (e.g., truncate the title separately).

### Nit 3: `maxInputTokens = 8000` is derived but unexplained

The plan says "8000 — safety margin below OpenRouter's 8192 limit." The margin is 192 tokens. Why not 7500 (safer) or 8100 (tighter)? The plan should justify this or derive it from a formula (e.g., `MaxChunkTokens * 15`).

### Nit 4: Fix 4 logging in `expandAndValidateBatch` is good but needs content preview

The plan says "Log error when the pre-embedding guard catches an oversized chunk (same details)." For debugging, the log should include the first 200 characters of the text, not just the token count and content length. The pseudocode in the earlier plan included this; the final plan dropped it.

---

## Part B (Chunker Simplification) — Rework Required

### Critical Gap 1: The recursive `splitContent` loses paragraph accumulation behavior

The current `splitByParagraphs` function (lines 611–733) **accumulates paragraphs** until they exceed `maxTokens`, then flushes. This produces chunks that are near `maxTokens` and groups related content together.

The proposed `splitByBlankLines` just does `strings.Split(content, "\n\n")`. This produces **one chunk per paragraph**, regardless of size. The recursive `splitContent` then checks each paragraph individually.

**Concrete difference:** A document with 10 small paragraphs (100 tokens each) would produce:
- **Current:** 2 chunks (5 paragraphs accumulated per chunk)
- **Proposed:** 10 chunks (one per paragraph)

This increases embedding API calls by 5x and produces smaller, less contextually rich chunks. The plan should either:
- Re-implement paragraph accumulation in `splitContent`, or
- Explain why 10 smaller chunks is acceptable for RAG quality.

### Critical Gap 2: `mergeSmallChunks` placement is unspecified

The plan says "`mergeSmallChunks` stays — it handles the 'group small chunks back together' step." But the new `ChunkMarkdownContent` body is:

```go
parts := splitContent(content, maxTokens)
for i, part := range parts {
    title, body := extractTitleAndBody(part)
    // create DocumentChunk...
}
```

There's no call to `mergeSmallChunks` here. The plan should specify:
- Where `mergeSmallChunks` is called (after the loop? before the guard?)
- Whether it operates on `[]string` or `[]DocumentChunk`
- Whether the `minTokens` parameter is still derived from `maxTokens / 5`

### Critical Gap 3: Title hierarchy loss is not justified

The plan acknowledges "The H2→H3 title hierarchy ('H2 > H3') is lost" and says "For RAG, content is what matters; titles are metadata." But:

- Titles are prepended to content at embedding time (`title + ": " + content`). Losing hierarchy means the embedding sees "H3" instead of "H2 > H3", which reduces semantic context.
- The current tests (`TestChunkerOverlapSafe`) produce chunks with titles like "Section One", "Section Two". The recursive approach would produce different titles depending on the `splitByHeaders` implementation.
- If the 14MB file uses nested headers, this could significantly affect retrieval quality.

The plan should either:
- Preserve hierarchy via a parent-title parameter (as it suggests for "later"), or
- Justify why flat titles are acceptable with evidence (e.g., the 14MB file has no nested headers).

### Critical Gap 4: `splitByHeaders` implementation details are missing

The plan says `splitByHeaders` handles "Any `^##`, `^###`, `^####` etc." but doesn't specify:
- Does it split on ALL header levels at once, or hierarchically (H2 first, then H3 within each H2)?
- What regex does it use? The current code uses `strings.HasPrefix(line, "## ")` for H2 and `strings.HasPrefix(line, "### ")` for H3.
- How does it handle content before the first header (preamble)?

The current `splitByH2Headers` preserves preamble content (lines 498–503). The recursive approach needs to handle this too.

### Nit 5: Performance concern with recursive `EstimateTokens`

For a 14MB file, `splitContent` calls `EstimateTokens` at every recursion level. The binary search in `splitByHardLimit` also calls `EstimateTokens` O(n log n) times. If the tokenizer is slow (e.g., the `regexp2` backtracking regex in tiktoken-go), this could be significantly slower than the current approach.

The plan should mention this risk and suggest profiling.

### Nit 6: Test claim "should pass unchanged" needs verification

The plan claims all 4 tests pass unchanged. Let me trace through `TestChunkerGuardCatchesOversized`:

1. Content: `"## Large Section\n" + 3000 comma-separated items`
2. Current flow: `splitByH2Headers` → 1 section → `splitByH3Headers` → 1 subsection → `splitByParagraphs` → 1 paragraph → guard catches → `splitByHardLimit`
3. Proposed flow: `splitByHeaders` →1 part (no split) → `splitByBlankLines` →1 part (no split) → `splitBySentences` →1 part (no sentence terminators) → `splitByHardLimit`

This should produce the same result. But `TestChunkerOverlapSafe` uses 3 H2 sections. The proposed flow would split by headers →3 parts → each part is small enough → return 3 chunks. The current flow does the same. So the test should pass.

However, the test checks `EstimateTokens(chunk.Content)` against `safeLimit`. If the proposed approach produces different chunk boundaries (due to the unified `splitByHeaders` vs separate H2/H3 splitting), the token counts could differ slightly. The plan should add a note about this.

---

## Summary

| Component | Verdict | Action |
|-----------|---------|--------|
| Fix 1 (pre-embedding guard) | **Approved with nits** | Specify expansion data flow |
| Fix 2 (Insert guard) | **Approved** | Same as Fix 1 |
| Fix 3 (title in guard) | **Approved with nits** | Document tradeoff |
| Fix 4 (diagnostic logging) | **Approved** | Add content preview |
| Fix 5 (tokenizer logging) | **Approved** | None |
| Fix 6 (constant) | **Approved** | Justify 8000 |
| Part B (recursive split) | **REWORK** | Address 4 critical gaps |

**Recommendation:** Implement Part A (Fixes 1–6) first. It's self-contained, low-risk, and will likely fix the immediate error. Defer Part B until the root cause is confirmed via diagnostic logs. The simplification is tempting but introduces semantic changes that need careful validation against the 14MB file.
