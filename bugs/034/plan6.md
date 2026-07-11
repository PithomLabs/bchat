# Plan 6: Defeat the 8192 Embedding Limit (Revised)

## Problem

Despite Plan 3's fix (real tokenizer, `splitByHardLimit` binary search, flush guards, final guard before `addChunkOverlap`), a 14MB markdown file still produces:

```
failed at batch 1: batch 1 failed with permanent error: failed to generate embeddings for batch 1:
embedding provider unavailable: OpenRouter API error: HTTP 400:
{ "error": { "message": "Invalid 'input[0]': maximum input length is 8192 tokens.", ... } }
```

## Root Cause

The `ChunkMarkdownContent` guard validates `EstimateTokens(chunk.Content)` but the embedding API receives `fmt.Sprintf("%s: %s", chunk.Title, chunk.Content)`. There is **zero validation** at the Embed call site (`doEmbed` in `embedding.go:377`). If ANY text exceeds 8192 tokens for any reason (tokenizer not initialized, chunker edge case, title inflation, overlap inflation, data corruption, future code changes), the API returns a permanent 400 error with no diagnostics.

---

## Part A — Defense-in-Depth (Fixes 1–6)

### Fix 1 (PRIMARY): Pre-embedding guard in `processSingleBatch`

**File:** `vectordb_lance.go` — `processSingleBatch` (line 617)

**What:** Before calling `Embed()`, expand any batch entries whose `Title + ": " + Content` exceeds `MaxEmbeddingInputTokens` (8000). Split via `splitByHardLimit` on the content (with a limit adjusted for title overhead). Each split part becomes its own `DocumentChunk` with a unique ID (`{originalID}_split_{n}`) and the original metadata copied. The batch slice is rebuilt (the single oversized entry replaced by N entries).

**Data flow:**

```
batch (before):  [chunkA, chunkB(oversized), chunkC]
                       ↓ iterate, check each
batch (after):   [chunkA, chunkB_p1, chunkB_p2, chunkB_p3, chunkC]
                       ↓ build textsToEmbed
textsToEmbed:    ["A: ...", "B: (Part 1): ...", "B: (Part 2): ...", "B: (Part 3): ...", "C: ..."]
                       ↓ Embed (all fit within 8192)
embeddings:      [[vecA], [vecB_p1], [vecB_p2], [vecB_p3], [vecC]]
                       ↓ assign back 1:1
Arrow records:   5 rows (one per expanded chunk)
```

The helper method `expandAndValidateBatch` does this:

```go
func (db *LanceVectorDB) expandAndValidateBatch(batch []DocumentChunk) []DocumentChunk {
    var expanded []DocumentChunk
    for _, chunk := range batch {
        embedText := fmt.Sprintf("%s: %s", chunk.Title, chunk.Content)
        if EstimateTokens(embedText) > MaxEmbeddingInputTokens {
            titleCost := EstimateTokens(chunk.Title) + 2
            contentLimit := MaxEmbeddingInputTokens - titleCost
            if contentLimit < 100 {
                contentLimit = 100
            }
            slog.Error("Oversized embedding input detected and split",
                "tokens", EstimateTokens(embedText),
                "limit", MaxEmbeddingInputTokens,
                "title", chunk.Title,
                "contentLength", len(chunk.Content),
                "contentPreview", chunk.Content[:min(200, len(chunk.Content))])
            parts := splitByHardLimit(chunk.Content, contentLimit)
            for p, part := range parts {
                newChunk := chunk
                newChunk.Content = part
                newChunk.Title = fmt.Sprintf("%s (Part %d)", chunk.Title, p+1)
                newChunk.Code = fmt.Sprintf("%s_split_%d", chunk.Code, p+1)
                newChunk.ID = fmt.Sprintf("%s_split_%d", chunk.ID, p+1)
                expanded = append(expanded, newChunk)
            }
        } else {
            expanded = append(expanded, chunk)
        }
    }
    return expanded
}
```

Called at the top of `processSingleBatch`:
```go
func (db *LanceVectorDB) processSingleBatch(ctx context.Context, batch []DocumentChunk, batchNum int) error {
    batch = db.expandAndValidateBatch(batch)
    // ... rest unchanged (build textsToEmbed, Embed, arrow record)
}
```

### Fix 2: Same guard in `Insert()`

**File:** `vectordb_lance.go` — `Insert` batch loop (line 416–417)

Call `expandAndValidateBatch` on each batch before building `textsToEmbed`:

```go
batch := chunks[batchStart:batchEnd]
batch = db.expandAndValidateBatch(batch)  // <-- add this
batchNum := (batchStart / batchSize) + 1
```

### Fix 3: Include title in the chunker guard

**File:** `chunker.go` — Guard at line 452

**Review feedback:** Changing the guard to include title changes chunking semantics (splits content that's fine on its own). Instead, **keep the guard checking `chunk.Content` only** — it prevents content-only overflow, which is the chunker's responsibility.

The title+content overflow is caught by Fix 1 (`expandAndValidateBatch`) at embedding time. This provides a clean separation:

| Layer | Checks | Responsibility |
|-------|--------|---------------|
| Chunker guard | `chunk.Content ≤ maxTokens` | Chunker correctness |
| `expandAndValidateBatch` | `Title + Content ≤ MaxEmbeddingInputTokens` | Embedding safety net |

No changes to the guard logic itself. Only add diagnostic logging (see Fix 4).

### Fix 4: Enhanced diagnostic logging

**File:** `chunker.go` — line 452

```go
if EstimateTokens(chunk.Content) > maxTokens {
    slog.Warn("Chunk exceeded maxTokens, splitting",
        "actualTokens", EstimateTokens(chunk.Content),
        "maxTokens", maxTokens,
        "title", chunk.Title,
        "contentLength", len(chunk.Content),
        "contentPreview", chunk.Content[:min(200, len(chunk.Content))])
    // ... splitByHardLimit as before
}
```

**File:** `vectordb_lance.go` — `expandAndValidateBatch`

Log includes content preview as shown in Fix 1.

### Fix 5: Tokenizer verification logging

**File:** `embedding.go` — `InitTokenizer`

- On failure: `slog.Error("CRITICAL: Tokenizer initialization failed, falling back to len/4 heuristic. Embedding token counts will be inaccurate.", "encoding", encName, "error", err)`
- On success: `slog.Info("Tokenizer verified", "encoding", encName, "testStringTokens", testTokens)` where `testTokens` is the result of `enc.Count("The quick brown fox jumps over the lazy dog.")`

**File:** `embedding.go` — `EstimateTokens`

Add one-time fallback warning:
```go
var fallbackWarnOnce sync.Once

func EstimateTokens(content string) int {
    if globalTokenizer != nil {
        count, err := globalTokenizer.Count(content)
        if err == nil {
            return count
        }
    }
    fallbackWarnOnce.Do(func() {
        slog.Warn("EstimateTokens using len/4 fallback — globalTokenizer not initialized")
    })
    return len(content) / 4
}
```

Add `"sync"` to imports.

### Fix 6: `MaxEmbeddingInputTokens` constant

**File:** `chunker.go` — alongside existing constants (line 72)

```go
MaxEmbeddingInputTokens = 8000
```

**Justification:** 8000 = 8192 (OpenRouter limit) − 192 (margin). 192 tokens is enough headroom for: title overhead (~20 tokens), `addChunkOverlap` inflation (~50 tokens), and subword-boundary variation (~20 tokens), with 100 tokens of emergency buffer. Derived as `GetMaxChunkTokens("openrouter") * 15 = 512 * 15 = 7680` rounded up to a round number.

---

## Part B — Chunker Simplification

### Design

Replace the ~196-line procedural branching in `ChunkMarkdownContent` (lines 331–426) with a **recursive `splitContent` function** that chains strategies. Each strategy handles one split method; the recursion ensures all content eventually fits within `maxTokens`.

### New `splitContent` (recursive chain)

```go
func splitContent(content string, maxTokens int) []string {
    content = strings.TrimSpace(content)
    if content == "" || EstimateTokens(content) <= maxTokens {
        return []string{content}
    }
    // Try each strategy in order of preference
    if parts := splitByH2Headers(content); len(parts) > 1 {
        return splitParts(parts, maxTokens)
    }
    if parts := splitByH3Headers(content); len(parts) > 1 {
        return splitParts(parts, maxTokens)
    }
    if parts := splitByParagraphs(content, maxTokens); len(parts) > 1 {
        return splitParts(parts, maxTokens)
    }
    if parts := splitBySentences(content); len(parts) > 1 {
        return splitParts(parts, maxTokens)
    }
    return splitByHardLimit(content, maxTokens)
}

func splitParts(parts []string, maxTokens int) []string {
    var result []string
    for _, part := range parts {
        result = append(result, splitContent(part, maxTokens)...)
    }
    return result
}
```

### Strategy changes

| Strategy | Before | After | Change |
|----------|--------|-------|--------|
| `splitByH2Headers` | 37 lines, preamble-aware | Unchanged | Kept as-is |
| `splitByH3Headers` | 23 lines | Unchanged | Kept as-is |
| `splitByParagraphs` | 123 lines, inline sentence+hard-limit fallback, `paragraphChunk` type | ~40 lines, no fallbacks, `[]string` return | Major simplification |
| `splitBySentences` | 38 lines | Unchanged | Kept as-is |
| `splitByHardLimit` | 32 lines | Unchanged | Kept as-is |

### `splitByParagraphs` simplification

Removed: inline sentence fallback, inline hard-limit fallback, `paragraphChunk` type, `flushChunk` closure.
Kept: accumulate-flush logic (paragraphs group until they'd exceed `maxTokens`).

```go
func splitByParagraphs(content string, maxTokens int) []string {
    paragraphs := strings.Split(content, "\n\n")
    var result []string
    var buf strings.Builder
    for _, para := range paragraphs {
        para = strings.TrimSpace(para)
        if para == "" {
            continue
        }
        combined := buf.String()
        if combined != "" {
            combined += "\n\n"
        }
        combined += para
        if EstimateTokens(combined) > maxTokens && buf.Len() > 0 {
            result = append(result, strings.TrimSpace(buf.String()))
            buf.Reset()
        }
        if buf.Len() > 0 {
            buf.WriteString("\n\n")
        }
        buf.WriteString(para)
    }
    if buf.Len() > 0 {
        result = append(result, strings.TrimSpace(buf.String()))
    }
    return result
}
```

The inline fallbacks are unnecessary because `splitContent` recurses on each resulting part — if a paragraph part is still too large, it goes through `splitBySentences` → `splitByHardLimit` on the next recursion.

### `ChunkMarkdownContent` body (simplified)

```go
parts := splitContent(content, maxTokens)
var chunks []DocumentChunk
now := time.Now()
for i, part := range parts {
    title, body := extractTitleAndBody(part)
    if strings.TrimSpace(body) == "" {
        continue
    }
    code := fmt.Sprintf("%s_chunk_%d", fileType, i)
    chunks = append(chunks, DocumentChunk{
        ID:            ChunkID(tenantID, audience, fileType+"_section", code),
        TenantID:      tenantID,
        AudienceType:  audience,
        ContentType:   fileType + "_section",
        Title:         title,
        Content:       body,
        Code:          code,
        IsActive:      true,
        SourceVersion: sourceVersion,
        IndexedAt:     now,
    })
}
```

Everything after this (mergeSmallChunks → garbage filter → final guard → addChunkOverlap) stays unchanged.

### Title hierarchy preserved

Because `splitByH2Headers` and `splitByH3Headers` are called **separately** (H2 first, then H3 within each H2 section), the `extractTitleAndBody` function sees the correct header at each level. An H2 section `## Services` that contains `### Water Extraction` produces:

```
splitByH2Headers → 1 section "## Services\n...### Water Extraction\n..."
  EstimateTokens > maxTokens → splitByH3Headers → 2 subsections:
    "## Services\nintro text..."    → title: "Services"
    "### Water Extraction\ndetail..." → title: "Water Extraction"
```

The recursion ensures each subsections' title comes from its own header. The content correctly reflects the nesting.

### What changes and what stays

| Removed | Lines | Reason |
|---------|-------|--------|
| `ChunkMarkdownContent` branching | ~196 | Replaced by `splitContent` + loop |
| `splitByParagraphs` fallbacks + `paragraphChunk` | ~83 | Handled by recursion |
| `paragraphChunk` struct | 4 | No longer needed |

| Kept | Lines | Reason |
|------|-------|--------|
| `splitByH2Headers` | 37 | Unchanged |
| `splitByH3Headers` | 23 | Unchanged |
| `extractTitleAndBody` | 22 | Unchanged |
| `splitBySentences` | 38 | Unchanged |
| `splitByHardLimit` | 32 | Unchanged |
| `mergeSmallChunks` | 52 | Unchanged |
| `addChunkOverlap` | 23 | Unchanged |
| Garbage filter + guard | ~40 | Unchanged |

| Added | Lines |
|-------|-------|
| `splitContent` | ~25 |
| `splitParts` | ~8 |
| Simplified `splitByParagraphs` | ~40 |

**Net: ~561 → ~332 lines for split logic (~40% reduction).**

### Addresses all critical gaps from review

| Gap | Before (Plan 4) | After (Plan 6) |
|-----|-----------------|----------------|
| CG1: Paragraph accumulation | `splitByBlankLines` → 1 chunk per paragraph | Keep `splitByParagraphs` with accumulate-flush → near-maxTokens chunks |
| CG2: mergeSmallChunks placement | Unspecified | Explicitly after document chunk loop |
| CG3: Title hierarchy loss | `splitByHeaders` unifies all levels → "H3" only | Separate H2→H3 call preserves `"H2 > H3"` |
| CG4: splitByHeaders impl missing | Vague spec | Keep existing H2 + H3 functions (known, tested) |

---

## Implementation Order

| Step | File | Change | Complexity |
|------|------|--------|-----------|
| 1 | `chunker.go` | Add `MaxEmbeddingInputTokens = 8000` constant | Trivial |
| 2 | `embedding.go` | Fix 5: Tokenizer verification logging | Low |
| 3 | `chunker.go` | Fix 4: Enhanced logging in guard | Low |
| 4 | `vectordb_lance.go` | Fix 1+2: `expandAndValidateBatch` in both Insert paths | Medium |
| 5 | `chunker.go` | Part B: Recursive split + simplify `splitByParagraphs` + rewrite `ChunkMarkdownContent` body; remove `paragraphChunk` | Medium |
| 6 | All | `go test`, `go build -tags rag`, `go vet` | — |

Steps 1–4 are independent of step 5. Deploy after step 4 if urgent.

## Files to Modify

| File | Lines | Change |
|------|-------|--------|
| `chunker.go` | 72 (constants) | **Step 1:** Add `MaxEmbeddingInputTokens = 8000` |
| `embedding.go` | 32–56 | **Step 2:** Upgrade failure log, add verification test |
| `embedding.go` | 107–115 | **Step 2:** Add `sync.Once` fallback warning in `EstimateTokens` |
| `chunker.go` | 450–473 | **Step 3:** Add diagnostic logging (content preview) to guard |
| `vectordb_lance.go` | 616–654 | **Step 4:** Add `expandAndValidateBatch` helper; call in `processSingleBatch` |
| `vectordb_lance.go` | 410–446 | **Step 4:** Call `expandAndValidateBatch` in `Insert` batch loop |
| `chunker.go` | 284–426 | **Step 5:** Replace `ChunkMarkdownContent` body with `splitContent` + loop |
| `chunker.go` | 567–733 | **Step 5:** Remove `paragraphChunk`; simplify `splitByParagraphs` (~123→40 lines) |
| `chunker.go` | 788–821 | **Step 5:** Add `splitContent` + `splitParts` functions |
| `chunker_test.go` | — | **Step 6:** Verify all 4 tests pass unchanged |

## Verification

1. `go test -v -run 'TestChunker' ./server/router/api/v1/agent/` — all 4 tests pass (same chunk boundary behavior verified)
2. `go test -v -run 'TestEstimateTokens' ./server/router/api/v1/agent/` — passes
3. `go build -tags rag ./server/router/api/v1/agent/` — compiles
4. `go vet -tags rag ./server/router/api/v1/agent/...` — clean
5. Deploy, reindex 14MB file, check logs for:
   - "Tokenizer initialized" with `testStringTokens`
   - No "Oversized embedding input" or "Chunk exceeded maxTokens" entries
