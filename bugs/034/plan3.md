# Bug 034 — Fix Plan v3: Real Tokenizer + Corrected Guard Placement

## Problem

Tenant 12 (`bchat`) reindex fails on batch 1 with:
```
OpenRouter API error: HTTP 400: "Invalid 'input[0]': maximum input length is 8192 tokens."
```

**Root cause:** `EstimateTokens` (`chunker.go:109`) uses `len(content)/4` — a heuristic that underestimates real tokens by 25–75% for code, markdown tables, URLs, and CJK text. A chunk "estimated" at 1000 tokens can be 2000–4000+ real tokens.

**Three contributing bugs:**

| # | Bug | Location | Type |
|---|---|---|---|
| A | Token estimation is heuristic, not exact | `chunker.go:109`, `observer.go:428` | Root cause |
| B | `splitByParagraphs` flush skips size check | `chunker.go:657–663` | Logic error |
| C | Title prepended at embedding time not counted | `vectordb_lance.go:624` | Accounting oversight |

---

## Configuration Context

| Setting | Value | Source |
|---|---|---|
| Embedding provider | `openrouter` | `.env` |
| Embedding model | `openai/text-embedding-3-small` | `.env` |
| Model token limit | 8192 | OpenRouter API |
| Batch size | 10 | `EMBEDDING_BATCH_SIZE` |
| Chunk max (openrouter) | 512 (was 1000) | `GetMaxChunkTokens` — reduced + exact counting |
| Chunk min (openrouter) | 100 (was 200) | `GetMinChunkTokens` — scaled proportionally |
| Overlap | 50 tokens | `ChunkOverlapTokens` |

---

## Fix 1 (Root Cause): Replace `EstimateTokens` with Real Tokenizer

Replace the heuristic `len(content)/4` with OpenAI's actual `cl100k_base` BPE tokenizer (the tokenizer used by `text-embedding-3-small`). Use the pure-Go library `github.com/tiktoken-go/tokenizer` which embeds vocabularies at compile time — zero runtime downloads, zero CGO.

### Changes

**`server/router/api/v1/agent/embedding.go`** — Add tokenizer init:
- Add import: `"github.com/tiktoken-go/tokenizer"`
- Add package-level var: `var globalTokenizer *tokenizer.Tiktoken`
- Add `InitTokenizer(provider, model string) error` mapping:
  - `text-embedding-3-small`, `text-embedding-3-large`, `text-embedding-ada-002` → `cl100k_base`
  - `gpt-4o*`, `gpt-5*` → `o200k_base`
  - default → `cl100k_base`
- Call `InitTokenizer` at startup wherever the embedding service is created (locate during implementation)

**`server/router/api/v1/agent/chunker.go`** — Replace `EstimateTokens` (line 109):
```go
func EstimateTokens(content string) int {
    if globalTokenizer == nil {
        return len(content) / 4 // fallback
    }
    ids, _, err := globalTokenizer.Encode(content)
    if err != nil {
        return len(content) / 4
    }
    return len(ids)
}
```

- Update comment to reflect real tokenizer usage
- Update `GetMaxChunkTokens("openrouter")` from 1000 → **512** (embedding quality sweet spot per industry benchmarks; exact counting means no safety margin needed)
- Update `GetMinChunkTokens` to scale with max: `maxTokens / 5` (512 → ~102, rounded down to 100)
- Update `GetMaxChunkTokens("local")` — keep at 150 (sentence-transformers limit is 512; conservative enough)
- Update `GetMaxChunkTokens("mock")` — keep at 500
- Update associated comments to reflect real (exact) counting

**`server/router/api/v1/agent/observer.go`** — Remove local duplicate:
- Delete `estimateTokens` function (lines 425–430)
- Replace all 3 call sites with `EstimateTokens`:
  - Line 198: `estimateTokens(updatedLog)` → `EstimateTokens(updatedLog)`
  - Line 216: `estimateTokens(updatedLog)` → `EstimateTokens(updatedLog)`
  - Line 548: `estimateTokens(updatedLog)` → `EstimateTokens(updatedLog)`

**`server/router/api/v1/agent/observer_buffer.go`**:
- Line 221: `estimateTokens(newObservations)` → `EstimateTokens(newObservations)`

**`server/router/api/v1/agent/fusion_engine.go`**:
- Line 94: `estimateTokens(chunk.Content)` → `EstimateTokens(chunk.Content)`
- Line 212: `estimateTokens(content)` → `EstimateTokens(content)`

**`server/router/api/v1/agent/service.go`**:
- Line 2399: `estimateTokens(session.Messages[i].Content)` → `EstimateTokens(...)`

**`server/router/api/v1/agent/observer_test.go`**:
- Update `TestEstimateTokens` (lines 62–113) — replace heuristic expectations with real tokenizer expectations:
  - `""` → 0 tokens
  - `"test"` → 1 token
  - `"test test"` → 2 tokens
  - `"hello"` → 1 token
  - `"This is a longer text..."` → actual cl100k_base count (not 16)
  - `"hello世界"` → actual cl100k_base count (not 1, this is critical — heuristic was wrong for CJK)

**`go.mod`** — Add dependency:
```
github.com/tiktoken-go/tokenizer v0.8.0
```

### Tokenizer initialization path

`InitTokenizer` must be called before any `EstimateTokens` call. The embedding service creation (`NewEmbeddingService` in `embedding.go:138`) is the natural place since it already has the model name. During implementation, verify the call chain and add the init there.

---

## Fix 2 (Logic Bug): Guard `splitByParagraphs` Flush

**File:** `server/router/api/v1/agent/chunker.go:657–663`

The final flush in `splitByParagraphs` appends the sentence buffer unconditionally. If a single paragraph has no sentence terminators (no `.`, `!`, `?`), the entire paragraph becomes one "sentence", enters the buffer, and gets flushed without any size check.

Current code:
```go
if sentenceBuffer.Len() > 0 {
    chunks = append(chunks, paragraphChunk{
        title:   fmt.Sprintf("%s (Part %d)", title, len(chunks)+1),
        content: strings.TrimSpace(sentenceBuffer.String()),
    })
}
```

Replace with:
```go
if sentenceBuffer.Len() > 0 {
    content := strings.TrimSpace(sentenceBuffer.String())
    if EstimateTokens(content) > maxTokens {
        parts := splitByHardLimit(content, maxTokens)
        for _, part := range parts {
            chunks = append(chunks, paragraphChunk{
                title:   fmt.Sprintf("%s (Part %d)", title, len(chunks)+1),
                content: part,
            })
        }
    } else {
        chunks = append(chunks, paragraphChunk{
            title:   fmt.Sprintf("%s (Part %d)", title, len(chunks)+1),
            content: content,
        })
    }
}
```

Note: `splitByHardLimit` already exists at line 746 — no definition needed.

---

## Fix 3 (Defense in Depth): Final Guard **Before** `addChunkOverlap`

**File:** `server/router/api/v1/agent/chunker.go` — insert **between garbage filter (line 441) and `addChunkOverlap` (line 444)**

**Critical placement decision:** The guard MUST go BEFORE `addChunkOverlap`. `addChunkOverlap` prepends `"[...] " + ~200 chars + "\n\n"` to each chunk (except the first), inflating token count by ~50 tokens. If the guard ran after overlap, it would trigger on nearly every chunk, forcing them all through `splitByHardLimit` (rune-based splitting). Placing the guard before overlap means it only catches true escape-hatch chunks.

```go
    chunks = cleanChunks  // line 441

    // Final guard: ensure no chunk exceeds maxTokens before overlap
    var guardedChunks []DocumentChunk
    for _, chunk := range chunks {
        if EstimateTokens(chunk.Content) > maxTokens {
            parts := splitByHardLimit(chunk.Content, maxTokens)
            for p, part := range parts {
                code := fmt.Sprintf("%s_guard_%d", chunk.Code, p+1)
                guardedChunks = append(guardedChunks, DocumentChunk{
                    ID:            ChunkID(chunk.TenantID, chunk.AudienceType, chunk.ContentType, code),
                    TenantID:      chunk.TenantID,
                    AudienceType:  chunk.AudienceType,
                    ContentType:   chunk.ContentType,
                    Title:         fmt.Sprintf("%s (Part %d)", chunk.Title, p+1),
                    Content:       part,
                    Code:          code,
                    IsActive:      true,
                    SourceVersion: chunk.SourceVersion,
                    IndexedAt:     chunk.IndexedAt,
                })
            }
        } else {
            guardedChunks = append(guardedChunks, chunk)
        }
    }
    chunks = guardedChunks

    chunks = addChunkOverlap(chunks, ChunkOverlapTokens)  // line 444
    return chunks  // line 446
```

With the real tokenizer, `EstimateTokens` returns exact counts, so this guard is mathematically correct — if it says `<= maxTokens`, the chunk will not overflow the embedding API.

---

## How Bug C (Title Prepending) Is Automatically Fixed

At `vectordb_lance.go:624`, the title is prepended at embedding time:
```go
textsToEmbed = append(textsToEmbed, fmt.Sprintf("%s: %s", chunk.Title, chunk.Content))
```

Before this fix: the chunker counted `len(Content)/4`, not `len(Title+Content)/4`. A chunk at 1000 estimated tokens could become ~1050 real tokens at embedding time.

After this fix: `EstimateTokens` reports exact counts. The guard in Fix 3 uses the real token count of `chunk.Content`. Total (`Title + Content`) is bounded by `maxTokens + title_overhead`. With maxTokens=512 and title overhead ~10–50 tokens, total is ~562 tokens — a ~14× safety margin below 8192. **Bug C is eliminated by Fix 1 + Fix 3 together; no separate change needed.**

---

## Files to Modify

| File | Change |
|---|---|
| `go.mod` | Add `github.com/tiktoken-go/tokenizer v0.8.0` |
| `server/router/api/v1/agent/embedding.go` | Add `InitTokenizer()` and global `*tokenizer.Tiktoken` var |
| `server/router/api/v1/agent/chunker.go` | Replace `EstimateTokens` body (line 109); Fix 2 (flush guard, line 657); Fix 3 (post-garbage, pre-overlap guard, line 441); update `GetMaxChunkTokens` to 512 and `GetMinChunkTokens` to 100; update comments |
| `server/router/api/v1/agent/observer.go` | Remove local `estimateTokens` (line 428); replace 3 call sites with `EstimateTokens` |
| `server/router/api/v1/agent/observer_buffer.go` | Replace `estimateTokens` → `EstimateTokens` (line 221) |
| `server/router/api/v1/agent/fusion_engine.go` | Replace `estimateTokens` → `EstimateTokens` (lines 94, 212) |
| `server/router/api/v1/agent/service.go` | Replace `estimateTokens` → `EstimateTokens` (line 2399) |
| `server/router/api/v1/agent/observer_test.go` | Update `TestEstimateTokens` with real tokenizer expectations |
| **`server/router/api/v1/agent/chunker_test.go`** | **CREATE** — see Verification step 1 |

---

## Files NOT Changing

| File | Reason |
|---|---|
| `server/router/api/v1/agent/processor.go` | Already uses `EstimateTokens` (exported) — replaced automatically |
| `server/router/api/v1/agent/handlers.go` | Same — already uses `EstimateTokens` |
| `server/router/api/v1/agent/vectordb_lance.go` | Title prepend is absorbed by safety margin; no change needed |
| `server/router/api/v1/agent/splitByHardLimit` (line 746) | Already exists and works correctly |

---

## Awareness Notes (No Code Change)

### handlers.go:5309 — Second caller with hardcoded 512

```go
maxChunkTokens := 512 // Standard chunk size
chunks := chunker.ChunkMarkdownContent(kbFile.Content, tenant.ID, req.AudienceType, "kb", 1, maxChunkTokens)
```

This Q&A generation path uses a hardcoded 512 (not from `GetMaxChunkTokens`). The same `splitByParagraphs` escape (Cause B) applies here, though at lower risk since 512 tokens is conservative. If this path ever hits the 8192 error, Fix 2 applies identically.

### splitBySentences trailing-period edge case (chunker.go:554)

Text ending with `.` but no trailing space or newline (e.g., `"Item 1. Item 2. Item 3."` at end of string) will NOT split the last sentence because `i+1 < len(runes)` fails the bounds check. The last period-terminated sentence merges into the previous one. Impact is minor — can produce a chunk ~1 sentence larger than expected. Not a direct cause of the 8192 error. (See Known Limitations.)

---

## Known Limitations

1. **`splitByHardLimit` cuts mid-sentence / mid-word.** When the hard limit is reached, it splits on rune boundaries. This is the last resort — better than API failure. Future improvement: split on sentence boundaries within the hard limit.

2. **`splitBySentences` trailing-period edge case.** Text ending with `.` without a trailing space/newline does not split the final sentence (see Awareness Notes above). Minor impact.

3. **Tokenizer model mismatch.** If the embedding model is changed to one that uses a different tokenizer (e.g., switching from `cl100k_base` to `o200k_base`), `InitTokenizer` must be called again. The plan handles this by mapping model names to encodings.

---

## Risk Assessment

| Risk | Likelihood | Mitigation |
|---|---|---|
| Tokenizer init fails at startup | Low | Fallback to `len/4` heuristic |
| Binary size +4MB (embedded vocab) | Certain | Acceptable for production Go binary |
| Chunk count increases (512 vs 1000 est) | Certain | ~7000 chunks (from ~3600 at 1000). At 10/batch ≈ 700 batches ≈ 23 min. Acceptable. |
| Retrieval quality changes from smaller chunks | Medium | 512 tokens is the industry sweet spot; 10% overlap (50 tokens) ensures continuity |
| `splitByHardLimit` cuts mid-sentence | Low | Last resort; existing benchmarks show acceptable degradation |
| Tokenizer dependency introduces new bugs | Low | Library is well-tested, pure Go, no CGO; fallback path exists |

---

## Verification

1. **Create** `server/router/api/v1/agent/chunker_test.go` with 4 test cases:
   - **No-terminator paragraph:** 5000 chars of comma-separated values, no `.!?`. Verify all output chunks ≤ `maxTokens` by `EstimateTokens`.
   - **Section with no H2 headers:** Entire content passed as one section. Verify it's properly split.
   - **Overlap doesn't push chunks over limit:** Verify `addChunkOverlap` output ≤ `maxTokens + ChunkOverlapTokens` for all chunks.
   - **Guard catches oversized chunks:** Inject a chunk with `Content` artificially exceeding `maxTokens`. Verify it's split by the guard.

2. **Update** `observer_test.go:TestEstimateTokens` with real tokenizer expectations.

3. **Run** `go test ./server/router/api/v1/agent/ -run TestChunk` to verify all chunker tests pass.

4. **Run** `go test ./server/router/api/v1/agent/ -run TestEstimateTokens` to verify tokenizer tests pass.

5. **Run** `go vet ./server/router/api/v1/agent/` to check for issues.

6. **Integration:** Reindex tenant 12 (`POST /api/v1/agent/bchat/reindex`) — should complete without 400 error.

7. **LanceDB query:** Verify chunk content sizes after reindex:
   ```sql
   SELECT length(content), title FROM kb_documents_1536 WHERE tenant_id=12 ORDER BY length(content) DESC LIMIT 5;
   ```
   Expected max `length(content)`: ~1792 chars (`512 * 3.5` from `splitByHardLimit`). All results should be under this.
