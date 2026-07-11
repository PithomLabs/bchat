# Plan Adversarial Review — Bug 034

Review of `plan.md` for RAG reindex failure: chunk exceeds 8192 token embedding limit.

---

## VERDICT: Fix 2 has a bug that must be corrected before implementation

The root cause analysis (Causes 1–4) is **correct and well-grounded**. Fixes 1 and 3 are **correct as written**. Fix 2 has a **placement bug** that would cause mass unnecessary splitting, plus a minor code nit. Several gaps should be addressed for the coding agent's benefit.

---

## BUG — Fix 2 guard placement causes mass false triggers

**Plan says:** Insert guard **after line 444** (after `addChunkOverlap`).

**Problem:** `addChunkOverlap` (line 784) prepends `"[...] " + ~200 chars + "\n\n"` to every chunk after the first. This inflates estimated tokens from ~700 to ~750. The guard threshold is `maxTokens` (700). Since the guard checks `EstimateTokens(chunk.Content) > maxTokens`, it triggers on **every chunk with overlap** — nearly all chunks.

**Impact:** Instead of a safety net catching rare oversized chunks, the guard becomes a mandatory splitter for the entire output. Every chunk gets fed to `splitByHardLimit`, producing `maxTokens * 3.5` char fragments. Chunk count roughly doubles. Retrieval quality degrades because `splitByHardLimit` cuts on rune boundaries (mid-word, mid-sentence).

**Fix — two options (recommend Option A):**

**Option A (cleanest):** Move the guard **before** `addChunkOverlap`, between the garbage filter (line 441) and `addChunkOverlap` (line 444). This way the guard runs on raw chunks before overlap inflation. Chunks at this point are guaranteed ≤ `maxTokens` by estimate from the normal chunking path, so the guard only triggers on escape-hatch chunks from Cause 3.

**Option B:** Keep the guard after `addChunkOverlap` but raise the threshold:
```go
safeMaxTokens := maxTokens + ChunkOverlapTokens // ~750 for openrouter
```
This accounts for overlap but couples the guard to the overlap constant — fragile if overlap changes.

**Recommended final insertion point (Option A):**
```go
    chunks = cleanChunks   // line 441

    // NEW: Final guard — split any oversized chunks before overlap
    // [guard code here]

    chunks = addChunkOverlap(chunks, ChunkOverlapTokens)  // line 444

    return chunks  // line 446
```

---

## NIT 1 — Pointless variable alias in Fix 2 code

**Plan has:**
```go
maxEmbedTokens := maxTokens
```

**Fix:** Remove the alias. Use `maxTokens` directly throughout the guard loop. The variable adds no clarity and `maxTokens` is already descriptive.

---

## NIT 2 — Plan低估了 Fix 2 chunk count impact

**Plan's Risk Assessment says:** "~5100 chunks. Acceptable tradeoff."

**Issue:** The "~40% more chunks" estimate (3600 → 5100) accounts for Fix 1 only. With the current (buggy) Fix 2 placement after overlap, every chunk gets split by `splitByHardLimit`, potentially doubling the count to ~10,000+.

**After the placement fix (Option A above):** The estimate becomes accurate again. The guard only triggers on true escape-hatch chunks (Cause 3), which are rare. Actual chunk count stays close to ~5100.

---

## GAP 1 — `chunker_test.go` does not exist

**Plan's Verification step 1 says:** "Add test case in `chunker_test.go`"

**Issue:** This file does not exist. `ls server/router/api/v1/agent/chunker_test.go` returns nothing. The coding agent must create it.

**Suggested test file location:** `server/router/api/v1/agent/chunker_test.go`

**Minimum test cases the agent should write:**

1. **No-terminator paragraph** — 5000 chars of comma-separated values, no `.!?`. Verify all output chunks ≤ `maxTokens` by `EstimateTokens`.

2. **Section with no H2 headers** — entire content passed as one section. Verify it's properly split.

3. **Overlap doesn't push chunks over limit** — verify `addChunkOverlap` output ≤ `maxTokens + ChunkOverlapTokens` for all chunks.

4. **Guard catches oversized chunks** — inject a chunk with `Content` artificially set to exceed `maxTokens`. Verify it's split by the guard.

---

## GAP 2 — Second caller at `handlers.go:5309`

**Location:** `server/router/api/v1/agent/handlers.go:5309`

```go
maxChunkTokens := 512 // Standard chunk size
chunks := chunker.ChunkMarkdownContent(kbFile.Content, tenant.ID, req.AudienceType, "kb", 1, maxChunkTokens)
```

This is a hardcoded `512` (not from `GetMaxChunkTokens`), used for Q&A pair generation. The same `splitByParagraphs` escape (Cause 3) applies here too, though at lower risk since 512 tokens is more conservative.

**Action for coding agent:** No code change needed (512 is safe enough), but note this in the plan for awareness. If the Q&A generation path ever hits this error, the same Fix 3 applies.

---

## GAP 3 — `splitBySentences` has a secondary edge case

**Location:** `chunker.go:554`

```go
if i+1 < len(runes) {
```

Text ending with `.` but no trailing space or newline (e.g., `"Item 1. Item 2. Item 3."` at end of string) will NOT split the last sentence because `i+1 == len(runes)` fails the bounds check. The last period-terminated sentence merges into the previous one.

**Impact:** Minor. Can produce a chunk ~1 sentence larger than expected. Not a direct cause of the 8192 error but worth documenting as a known limitation.

**Action:** No fix needed for this bug. Add a comment in the code or a "Known Limitations" section in the plan.

---

## GAP 4 — Title overhead not reflected in guard threshold

The guard checks `EstimateTokens(chunk.Content)`, but the actual embedding text is `fmt.Sprintf("%s: %s", chunk.Title, chunk.Content)` (vectordb_lance.go:624). Title adds ~10–50 tokens.

**Impact:** Already mitigated by Fix 1's conservative 700 target. At 700 estimated tokens + 50 title = 750, with 3x inflation = 2250 actual tokens. Well under 8192.

**Action:** No code change needed. Add a one-line comment in the guard code noting that title overhead is accounted for by the conservative `maxTokens` value.

---

## VERIFICATION — Adjusted checklist for coding agent

After implementing the corrected fixes:

1. **Create** `server/router/api/v1/agent/chunker_test.go` with the 4 test cases above.

2. **Run** `go test ./server/router/api/v1/agent/ -run TestChunk` to verify all tests pass.

3. **Run** `go vet ./server/router/api/v1/agent/` to check for issues.

4. **Reindex tenant 12** (`POST /api/v1/agent/bchat/reindex`) — should complete without 400 error.

5. **Verify chunk sizes** after reindex:
   ```bash
   sqlite3 build/data/memos_dev.db \
     "SELECT length(content), title FROM kb_documents_1536 WHERE tenant_id=12 ORDER BY length(content) DESC LIMIT 5;"
   ```
   All chunks should be ≤ ~2800 chars (700 tokens × 3.5 × 1.2 overlap margin ≈ 2940).

---

## SUMMARY — What the coding agent should change in plan.md

| Item | Current plan | Corrected |
|------|-------------|-----------|
| Fix 2 insertion point | After line 444 (after `addChunkOverlap`) | **Before line 444** (after garbage filter, before `addChunkOverlap`) |
| Fix 2 variable | `maxEmbedTokens := maxTokens` | Remove alias, use `maxTokens` directly |
| Fix 2 code block | As-is (after variable fix) | Move to between `chunks = cleanChunks` and `chunks = addChunkOverlap(...)` |
| Verification step 1 | "Add test case in chunker_test.go" | "Create `chunker_test.go` with 4 test cases (see Gap 1)" |
| Risk Assessment | "~5100 chunks" | Accurate after placement fix; no change needed |
| New section | — | Add "Known Limitations" noting `splitBySentences` trailing-period edge case (Gap 3) |
