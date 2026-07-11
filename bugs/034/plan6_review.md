# Plan 6 Review — Adversarial Review

**Reviewer:** opencode
**Date:** 2026-07-11
**Verdict:** **APPROVED WITH NITS** — All critical gaps from Plan 5 addressed; Part B is sound

---

## Part A (Defense-in-Depth) — Approved

### What's good

- **Fix 1 data flow is now concrete.** The ASCII diagram clearly shows how oversized chunks are expanded into multiple `DocumentChunk` entries with unique IDs (`{originalID}_split_{n}`) and copied metadata. The `expandAndValidateBatch` implementation is complete and correct.
- **Fix 3 smartly avoids the semantic trap.** Instead of changing the guard to include the title (which would alter chunking behavior), it keeps the guard checking `chunk.Content` only and relies on `expandAndValidateBatch` for the title+content check. Clean separation of concerns.
- **Fix 6 justification is solid.** 8000 = 8192 − 192, where 192 covers title (~20) + overlap (~50) + subword (~20) + 100 buffer. Also derived as `GetMaxChunkTokens * 15 = 7680` rounded up.
- **Fix 5 `sync.Once` pattern is correct.** One-time warning avoids log spam while still surfacing the issue.

### Nit 1: `expandAndValidateBatch` is a method but uses no receiver fields

The function is defined as `func (db *LanceVectorDB) expandAndValidateBatch(batch []DocumentChunk) []DocumentChunk` but doesn't reference `db` at all. It could be a standalone function (or a method on a different type). This is cosmetic but slightly misleading — it suggests the function needs LanceDB state when it doesn't.

**Suggestion:** Either make it a standalone function or document why it's a method (e.g., "kept as method for future use with tenant-specific limits").

### Nit 2: Arrow record creation with expanded chunks

The plan shows the data flow ends with "Arrow records: 5 rows (one per expanded chunk)." This is correct because `chunksToArrowRecord` iterates over the batch slice, and the expanded batch has the correct number of entries. But the plan should explicitly note that `totalChunks` in the `Insert` loop is recalculated after expansion (or that the loop handles variable batch sizes). Currently the `Insert` loop at line 410 uses `totalChunks := len(chunks)` which is set before the loop — if expansion happens inside the loop, the count is stale.

**Suggestion:** Add a note that `expandAndValidateBatch` is called per-batch (not once upfront), so `totalChunks` doesn't need recalculation. Or call it once upfront and recalculate.

### Nit 3: `splitByHardLimit` receives `contentLimit` not `MaxEmbeddingInputTokens`

In `expandAndValidateBatch`:
```go
contentLimit := MaxEmbeddingInputTokens - titleCost
parts := splitByHardLimit(chunk.Content, contentLimit)
```

This is correct — `splitByHardLimit` splits the **content** (not the full text), so the limit should exclude title overhead. But it's worth a one-line comment since it's easy to misread as "split by the embedding limit."

---

## Part B (Chunker Simplification) — Approved

### What's good

- **All 4 critical gaps from Plan 5 review are addressed:**
  - CG1 (paragraph accumulation): Simplified `splitByParagraphs` retains accumulate-flush logic — paragraphs group until they'd exceed `maxTokens`. This is the key behavioral preservation.
  - CG2 (`mergeSmallChunks` placement): Explicitly stated as "stays unchanged" after the document chunk loop.
  - CG3 (title hierarchy): `splitContent` calls `splitByH2Headers` and `splitByH3Headers` separately (H2 first, then H3 within each H2). The recursion handles the nesting correctly.
  - CG4 (`splitByHeaders` implementation): Kept existing tested functions instead of writing new untested ones.

- **The recursive `splitContent` is clean and correct.** Each strategy is tried in preference order. If a strategy produces multiple parts, each is recursively processed. If no strategy produces multiple parts, `splitByHardLimit` is the fallback.

- **The simplified `splitByParagraphs` is correct.** I traced through the logic:
  - Empty buffer + oversized paragraph → accumulated (not flushed), then recursively split by `splitContent`
  - Non-empty buffer + oversized combined → buffer flushed, new paragraph accumulated
  - Last paragraph → flushed at end, then recursively split
  - This matches the current behavior for all practical cases.

- **Net reduction is real: ~561 → ~332 lines (~40%).** The removed code (inline sentence fallback, inline hard-limit fallback, `paragraphChunk` type) is genuinely unnecessary when recursion handles it.

### Nit 4: `splitByParagraphs` signature change is unmentioned

The current `splitByParagraphs` returns `[]paragraphChunk` (with `title` and `content` fields). The simplified version returns `[]string`. This is a breaking change to the function signature. The plan should note that:
- `paragraphChunk` struct is removed
- All callers of `splitByParagraphs` are updated (currently only `ChunkMarkdownContent` calls it)
- The title generation moves from `splitByParagraphs` to `extractTitleAndBody` in the main loop

This is correct behavior but should be documented in the "What changes" table.

### Nit 5: Chunk ID format change is undocumented

Current: `%s_section_%d_%d_%d` (e.g., `kb_section_0_1_2`)
Proposed: `%s_chunk_%d` (e.g., `kb_chunk_0`)

This changes the ID format for all chunks. While functionally equivalent (IDs are opaque keys), it means:
- Reindexing after the change will create new chunks with different IDs
- Old chunks won't be deduplicated against new ones (the `Delete` call before reindex handles this)
- If a partial reindex is needed, the ID format mismatch could cause confusion

**Suggestion:** Either preserve the old format or note that reindexing is a full replace (no partial resume).

### Nit 6: `splitContent` calls `splitByParagraphs(content, maxTokens)` — title parameter dropped

The current `splitByParagraphs` takes `(content, title string, maxTokens int)` — the `title` parameter is used for chunk title generation. The simplified version takes `(content string, maxTokens int)` — no title. This is correct because titles are now generated by `extractTitleAndBody` in the main loop. But the plan should note this parameter change.

---

## Test Impact Analysis

The plan claims "all 4 tests pass unchanged." Let me verify:

| Test | Current behavior | Proposed behavior | Match? |
|------|-----------------|-------------------|--------|
| `TestChunkerNoTerminatorParagraph` | No H2 → no H3 → `splitByParagraphs` (one paragraph, no terminators) → guard → `splitByHardLimit` | `splitByH2Headers` (1 part) → `splitByH3Headers` (1 part) → `splitByParagraphs` (1 part) → `splitBySentences` (1 part) → `splitByHardLimit` | ✓ Same result |
| `TestChunkerNoH2Headers` | No H2 → `splitByParagraphs` (200 paragraphs, accumulate-flush) → guard | `splitByH2Headers` (1 part) → `splitByH3Headers` (1 part) → `splitByParagraphs` (accumulate-flush) → guard | ✓ Same result |
| `TestChunkerOverlapSafe` | 3 H2 sections → 3 chunks with overlap | `splitByH2Headers` (3 parts) → each part small enough → 3 chunks with overlap | ✓ Same result |
| `TestChunkerGuardCatchesOversized` | 1 H2, 1 paragraph, 3000 items → guard → `splitByHardLimit` | `splitByH2Headers` (1 part) → `splitByH3Headers` (1 part) → `splitByParagraphs` (1 part) → `splitBySentences` (1 part) → `splitByHardLimit` | ✓ Same result |

All tests should pass. The only difference is the number of recursive calls, which doesn't affect the output.

---

## Summary

| Component | Verdict | Action |
|-----------|---------|--------|
| Fix 1 (expandAndValidateBatch) | **Approved with nits** | Note receiver is unused; add comment on `contentLimit` |
| Fix 2 (Insert guard) | **Approved** | None |
| Fix 3 (keep guard as-is) | **Approved** | None |
| Fix 4 (diagnostic logging) | **Approved** | None |
| Fix 5 (tokenizer logging) | **Approved** | None |
| Fix 6 (constant) | **Approved** | None |
| Part B (recursive split) | **Approved with nits** | Document `splitByParagraphs` signature change, chunk ID format change, title parameter removal |

**Recommendation:** Approve and implement. The plan is thorough, addresses all review feedback, and the Part B simplification is well-justified. The nits are documentation-level — they don't block implementation but should be noted in the code comments. Deploy Part A (Steps 1–4) first if urgent; Part B (Step 5) can follow immediately after.
