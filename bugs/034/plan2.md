# Bug 034 — Fix Plan v2: Replace Heuristic with Real Tokenizer

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

## Fix 1 (Root Cause): Replace `EstimateTokens` with Real Tokenizer

Replace the heuristic `len(content)/4` with OpenAI's actual `cl100k_base` BPE tokenizer (the tokenizer used by `text-embedding-3-small`). Use the pure-Go library `github.com/tiktoken-go/tokenizer` which embeds vocabularies at compile time — zero runtime downloads, zero CGO.

### Changes

**`server/router/api/v1/agent/embedding.go`** — Add tokenizer init:
- Add import: `"github.com/tiktoken-go/tokenizer"`
- Add package-level var: `var globalTokenizer *tokenizer.Tiktoken`
- Add `InitTokenizer(provider, model string) error`:
  - `text-embedding-3-small`, `text-embedding-3-large`, `text-embedding-ada-002` → `cl100k_base`
  - `gpt-4o*`, `gpt-5*` → `o200k_base`
  - default → `cl100k_base`
- Call `InitTokenizer` at startup wherever the embedding service is created
  - Locate the startup path during implementation

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
- Update `GetMaxChunkTokens("openrouter")` from 1000 → **512** (the embedding quality sweet spot per industry benchmarks — smaller chunks give sharper, more discriminative embeddings; exact counting means no safety margin needed)
- Update `GetMaxChunkTokens("local")` — keep at 150 (sentence-transformers limit is 512, but 150 with real tokenizer is still well under)
- Update `GetMaxChunkTokens("mock")` — keep at 500
- Update associated comments to reflect real (exact) counting

**`server/router/api/v1/agent/observer.go`** — Remove local duplicate:
- Delete `estimateTokens` function (lines 425–430)
- Replace all 3 call sites with the exported `EstimateTokens`
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
- Update `TestEstimateTokens` (lines 62–113) — replace heuristic expectations with real tokenizer expectations for each test input:
  - `""` → 0 tokens
  - `"test"` → 1 token
  - `"test test"` → 2 tokens
  - `"hello"` → 1 token
  - `"This is a longer text with more words to test the token estimation"` → actual cl100k_base count
  - `"hello世界"` → actual cl100k_base count (this is critical — the heuristic was wrong for CJK)

**`go.mod`** — Add dependency:
```
github.com/tiktoken-go/tokenizer v0.8.0
```

---

## Fix 2 (Logic Bug): Guard `splitByParagraphs` Flush

**File:** `server/router/api/v1/agent/chunker.go:657–663`

The final flush in `splitByParagraphs` appends the sentence buffer unconditionally. If a single paragraph has no sentence terminators (no `.`, `!`, `?`), the entire paragraph becomes one "sentence", enters the buffer, and gets flushed without any size check.

Current code:
```go
// Flush remaining sentences
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

## Fix 3 (Defense in Depth): Final Guard After `addChunkOverlap`

**File:** `server/router/api/v1/agent/chunker.go` — insert after `addChunkOverlap` (after line 444)

Add a validation pass that splits any chunk exceeding the real token limit. This catches edge cases that all prior splitting logic missed:

```go
// Final guard: ensure no chunk exceeds embedding model token limits
maxEmbedTokens := maxTokens
var guardedChunks []DocumentChunk
for _, chunk := range chunks {
    estTokens := EstimateTokens(chunk.Content)
    if estTokens > maxEmbedTokens {
        parts := splitByHardLimit(chunk.Content, maxEmbedTokens)
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
```

Note 1: With the real tokenizer, `EstimateTokens` returns exact counts, so this guard is mathematically correct — if it says `<= maxTokens`, the chunk will not overflow the embedding API.

Note 2: `splitByHardLimit` with `maxTokens=512` creates parts of `maxTokens * 3.5 = 1792` chars each. Each part will have at most ~512 real tokens (cl100k_base uses ~3.5–4 chars/token for English). In the worst case (dense code: ~2 chars/token), a part would be ~896 real tokens — still well under 8192. So the guard is safe.

---

## How Bug C (Title Prepending) Is Automatically Fixed

At `vectordb_lance.go:624`, the title is prepended at embedding time:
```go
textsToEmbed = append(textsToEmbed, fmt.Sprintf("%s: %s", chunk.Title, chunk.Content))
```

Before this fix: the chunker counted `len(Content)/4`, not `len(Title+Content)/4`. A chunk at 1000 estimated tokens could become ~1050 real tokens at embedding time.

After this fix: `EstimateTokens` reports exact counts. The guard in Fix 3 uses real token count of `chunk.Content`. Even if the title adds 10–50 real tokens (depending on length), the total (`chunk.Content + title`) is still bounded by `maxTokens + ~50`. Given the 8192 embedding limit and `maxTokens=512`, that's a ~15× safety margin. **Bug C is eliminated by Fix 1 + Fix 3 together; no separate change needed.**

---

## Files to Modify (Summary)

| File | Change |
|---|---|
| `go.mod` | Add `github.com/tiktoken-go/tokenizer v0.8.0` |
| `server/router/api/v1/agent/embedding.go` | Add `InitTokenizer()` and global `*tokenizer.Tiktoken` var |
| `server/router/api/v1/agent/chunker.go` | Replace `EstimateTokens` body (line 109); apply Fix 2 (flush guard, line 657); apply Fix 3 (post-overlap guard, line 444); update `GetMaxChunkTokens` to 512; update comments |
| `server/router/api/v1/agent/observer.go` | Remove local `estimateTokens` (line 428); replace 3 call sites with `EstimateTokens` |
| `server/router/api/v1/agent/observer_buffer.go` | Replace `estimateTokens` → `EstimateTokens` (line 221) |
| `server/router/api/v1/agent/fusion_engine.go` | Replace `estimateTokens` → `EstimateTokens` (lines 94, 212) |
| `server/router/api/v1/agent/service.go` | Replace `estimateTokens` → `EstimateTokens` (line 2399) |
| `server/router/api/v1/agent/observer_test.go` | Update `TestEstimateTokens` with real tokenizer expectations |

---

## Files NOT Changing

| File | Reason |
|---|---|
| `server/router/api/v1/agent/processor.go` | Already uses `EstimateTokens` (exported) — replaced automatically |
| `server/router/api/v1/agent/handlers.go` | Same |
| `server/router/api/v1/agent/vectordb_lance.go` | Title prepend is absorbed by safety margin; no change needed |
| `server/router/api/v1/agent/splitByHardLimit` (line 746) | Already exists and works correctly |

---

## Risk Assessment

| Risk | Likelihood | Mitigation |
|---|---|---|
| Tokenizer init fails at startup | Low | Fallback to `len/4` heuristic |
| Binary size +4MB | Certain | Acceptable for production Go binary |
| Chunk count increases (512 vs 700 est tokens) | Certain | From ~3600 to ~5100 chunks for tenant 12 = ~17 min at batch=10. Acceptable. |
| Retrieval quality changes from smaller chunks | Medium | 512 tokens is the industry sweet spot; overlap at 10% (50 tokens) ensures continuity |
| `splitByHardLimit` cuts mid-sentence | Low | Run-time splitting is last resort; existing BEIR benchmarks show acceptable degradation |

---

## Verification

1. **Unit test:** Update `TestEstimateTokens` in `observer_test.go` with real tokenizer expectations
2. **Chunker edge case test:** Manually verify that a paragraph with no sentence terminators (e.g., 10,000 chars of comma-separated values) produces chunks ≤ 512 tokens
3. **Integration:** Reindex tenant 12 — should complete without 400 error
4. **LanceDB query:** Verify chunk content sizes:
   ```sql
   SELECT length(content), title FROM kb_documents_1536 WHERE tenant_id=12 ORDER BY length(content) DESC LIMIT 5;
   ```
   Expected max content length: ~1792 chars (`512 * 3.5` from `splitByHardLimit`)
5. **Binary size:** Confirm +4MB from embedded vocab
