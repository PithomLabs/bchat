# Bug 034: RAG Reindex Fails — Chunk Exceeds Embedding Model Token Limit

## Problem Statement

Tenant 12 (`bchat`) reindex fails on batch 1 with:
```
OpenRouter API error: HTTP 400: "Invalid 'input[0]': maximum input length is 8192 tokens."
```

The embedding model is `openai/text-embedding-3-small` (8192 token limit).
The chunker targets a max of 1000 estimated tokens per chunk (`GetMaxChunkTokens("openrouter")`).
Despite this 8x safety margin, chunks still exceed the real token limit.

## Root Cause Analysis

### Cause 1: Token estimation inaccuracy

`EstimateTokens` (`chunker.go:110`) uses `len(content) / 4` — a rough chars-per-token heuristic.
For `openai/text-embedding-3-small` (cl100k_base tokenizer):
- English prose: ~3.5–4.5 chars/token (estimation roughly accurate)
- Dense technical content, markdown tables, code-like text: ~2–3 chars/token (estimation **underestimates by 2–3x**)
- Non-English text: ~1–2 chars/token (estimation **underestimates by 4x**)

A chunk estimated at 1000 tokens could实际 be 2000–3000+ real tokens.

### Cause 2: Title prepended at embedding time

At `vectordb_lance.go:624`:
```go
textsToEmbed = append(textsToEmbed, fmt.Sprintf("%s: %s", chunk.Title, chunk.Content))
```
The chunker estimates token count from `chunk.Content` only. The title is prepended only at embedding time, adding 10–50+ extra tokens beyond the estimate.

### Cause 3: Chunk escape in `splitByParagraphs`

When a single paragraph exceeds `maxTokens` but contains no sentence terminators (`.`, `!`, `?`):

1. `splitByParagraphs` calls `splitBySentences` (`chunker.go:617`)
2. `splitBySentences` returns the entire paragraph as one "sentence" (`chunker.go:661–664`)
3. The sentence is added to `sentenceBuffer` but never triggers the flush-on-exceed check (requires `sentenceBuffer.Len() > 0` on the *first* iteration — `chunker.go:627`)
4. The remaining sentence is flushed as one chunk with **no hard-limit split** (`chunker.go:658–664`)

This produces a chunk that can be arbitrarily large — limited only by the size of the H2/H3 section.

### Cause 4: No final guard before embedding

After `addChunkOverlap` (`chunker.go:444`) adds ~200 chars to each chunk, there is no validation pass that checks whether any chunk actually fits within the embedding model's token limit. Oversized chunks are sent directly to the API.

## Configuration Context

| Setting | Value | Source |
|---------|-------|--------|
| Embedding provider | `openrouter` | `.env` |
| Embedding model | `openai/text-embedding-3-small` | `.env` |
| Model token limit | 8192 | OpenRouter API |
| Batch size | 10 | `EMBEDDING_BATCH_SIZE` |
| Chunk max (openrouter) | 1000 estimated tokens | `GetMaxChunkTokens` |
| Chunk min (openrouter) | 200 estimated tokens | `GetMinChunkTokens` |
| Overlap | 50 tokens (~200 chars) | `ChunkOverlapTokens` |
| Tenant 12 API key | Configured (encrypted) | `tenant_config` |

## Fix Plan

### Fix 1: Reduce `GetMaxChunkTokens("openrouter")` to 700 (conservative immediate fix)

**File:** `server/router/api/v1/agent/chunker.go:82`

Change from 1000 → 700.

**Rationale:** With 700 estimated tokens, worst-case 3x tokenizer inflation = 2100 real tokens + title overhead (~50) = ~2150. That's a 3.8x safety margin vs the 8192 limit. This is the simplest fix and can be deployed independently.

**Impact:** Smaller chunks mean more chunks total (roughly 40% more), increasing embedding API calls and storage. For tenant 12 with 3.6M estimated tokens, this goes from ~3600 chunks to ~5100 chunks. Acceptable tradeoff for correctness.

### Fix 2: Add final chunk size guard in `ChunkMarkdownContent`

**File:** `server/router/api/v1/agent/chunker.go` — insert after line 444 (after `addChunkOverlap`)

Add a validation pass that splits any chunk exceeding a safe token limit using `splitByHardLimit`. This is the defensive safety net.

```go
// Final guard: ensure no chunk exceeds embedding model limits
maxEmbedTokens := maxTokens
var guardedChunks []DocumentChunk
for _, chunk := range chunks {
    estTokens := EstimateTokens(chunk.Content)
    if estTokens > maxEmbedTokens {
        // Oversized chunk — split by hard limit
        parts := splitByHardLimit(chunk.Content, maxEmbedTokens)
        for p, part := range parts {
            code := fmt.Sprintf("%s_guard_%d", chunk.Code, p)
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

**Rationale:** Even with conservative chunk sizes, edge cases (binary content, corrupted encoding, unusual formatting) could produce oversized chunks. This guard catches them all.

### Fix 3: Fix `splitByParagraphs` to enforce hard limit on flush

**File:** `server/router/api/v1/agent/chunker.go:658–664`

Current code (flush remaining sentences):
```go
if sentenceBuffer.Len() > 0 {
    chunks = append(chunks, paragraphChunk{
        title:   fmt.Sprintf("%s (Part %d)", title, len(chunks)+1),
        content: strings.TrimSpace(sentenceBuffer.String()),
    })
}
```

Add hard limit check before flushing:
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

**Rationale:** This fixes the root cause where a single sentence exceeding `maxTokens` escapes all splitting logic. The hard limit split is the last resort and should always be applied.

## Files to Modify

| File | Lines | Change |
|------|-------|--------|
| `server/router/api/v1/agent/chunker.go` | 82 | Reduce `GetMaxChunkTokens("openrouter")` from 1000 → 700 |
| `server/router/api/v1/agent/chunker.go` | After 444 | Add final chunk size guard pass |
| `server/router/api/v1/agent/chunker.go` | 658–664 | Add hard limit check in `splitByParagraphs` flush |

## Verification

1. **Unit test:** Add test case in `chunker_test.go` with a long paragraph without sentence terminators (e.g., 5000 chars of comma-separated values) — verify it produces chunks ≤ `maxTokens`
2. **Integration test:** Reindex tenant 12 (`POST /api/v1/agent/bchat/reindex`) — should complete without 400 error
3. **Manual verification:** Query LanceDB after reindex to confirm chunk sizes:
   ```bash
   sqlite3 build/data/memos_dev.db "SELECT length(content), title FROM kb_documents_1536 WHERE tenant_id=12 ORDER BY length(content) DESC LIMIT 5;"
   ```

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| More chunks = more API calls | EMBEDDING_BATCH_SIZE=10 already batches; ~5100 chunks = ~510 batches. At ~2s/batch = ~17 min total. Acceptable. |
| Smaller chunks reduce retrieval quality | 700 tokens is still a substantial context window. Overlap ensures continuity. |
| Hard limit split may cut mid-sentence | `splitByHardLimit` uses rune-based splitting; not ideal but better than API failure. Future improvement: split on sentence boundaries within the hard limit. |
