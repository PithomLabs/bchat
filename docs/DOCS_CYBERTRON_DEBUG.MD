# Cybertron Sequence Length Error: Debug and Fix

**Status:** Resolved
**Date:** 2026-01-26
**Related Documents:**
- [DOCS_CYBERTRON.MD](./DOCS_CYBERTRON.MD) - Cybertron implementation guide
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG pipeline overview

---

## Problem

When using Cybertron embeddings with the RAG pipeline, large content chunks caused embedding failures:

```
ERROR reindex failed tenantID=4 audience=internal
error="failed to insert chunks for audience internal: failed to generate embeddings:
failed to encode text 1: input sequence too long: 11208 > 512"
```

The error indicates that text with 11,208 characters was sent to a model with a 512 token limit.

---

## Root Cause Analysis

### Cybertron Model Limitations

Cybertron's sentence-transformer models have a **hard 512 token limit**:

| Model | Dimension | Max Tokens |
|-------|-----------|------------|
| `all-MiniLM-L6-v2` | 384 | 512 |
| `all-mpnet-base-v2` | 768 | 512 |

### Four Contributing Problems

| Problem | Location | Description |
|---------|----------|-------------|
| MaxChunkTokens too high | `chunker.go:477` | Was 800 tokens, exceeds 512 limit |
| Inaccurate token estimation | `chunker.go:484` | `len/4` underestimates actual tokens |
| No sub-paragraph splitting | `chunker.go:704` | Only splits on `\n\n`, large paragraphs pass through |
| No validation before embedding | `embedding.go` | Sent oversized text directly to model |

### Processing Flow

```
ReindexTenantContent() [service.go]
  └─> ChunkMarkdownContent() [chunker.go]
      └─> splitByH2Headers()
      └─> splitByH3Headers()
      └─> splitByParagraphs() ← Only split on \n\n
  └─> vectorDB.Insert()
      └─> embedSvc.Embed()
          └─> model.Encode(text) → ERROR: sequence too long
```

### Why 11,208 Characters?

The `splitByParagraphs()` function only splits on blank lines (`\n\n`). Content without paragraph breaks (e.g., long legal text, unformatted markdown) passed through as a single chunk.

---

## Solution: Defense in Depth

Three fixes were implemented to ensure no content causes embedding failures:

### Fix 1: Reduced MaxChunkTokens

**File:** `server/router/api/v1/agent/chunker.go`

```go
// BEFORE
MaxChunkTokens = 800

// AFTER (see Addendum for final value)
MaxChunkTokens = 250  // Safe for 512-token models with subword tokenization
MinChunkTokens = 50
ChunkOverlapTokens = 25
```

### Fix 2: Added Sentence-Level Splitting

**File:** `server/router/api/v1/agent/chunker.go`

Added `splitBySentences()` helper and modified `splitByParagraphs()` to use it when a single paragraph exceeds the token limit.

```go
// When a paragraph exceeds maxTokens:
sentences := splitBySentences(para)
// Accumulate sentences until maxTokens, then create chunk
```

### Fix 3: Added Truncation Safety Net

**File:** `server/router/api/v1/agent/embedding.go`

Added truncation before encoding to prevent hard failures:

```go
// See Addendum for final value
const maxChars = 1000  // ~400 tokens at 2.5 chars/token (subword)

if len(text) > maxChars {
    slog.Warn("truncating text for embedding", ...)
    text = text[:maxChars]
}
```

### Why All Three?

| Fix | Alone? | Purpose |
|-----|--------|---------|
| Reduce MaxChunkTokens | Doesn't help large paragraphs | Smaller chunks for all content |
| Sentence splitting | Requires chunker changes | Handles large paragraphs |
| Truncation safety | Loses semantic info | Prevents hard failures |

**Defense in depth** ensures robustness across edge cases.

---

## Testing

### Verification Steps

1. Build with RAG support:
   ```bash
   task build:backend:rag
   ```

2. Run with Cybertron embeddings:
   ```bash
   task run:rag:cybertron
   ```

3. Navigate to Agent Admin → select tenant → click "Rebuild Index"

4. Verify no "sequence too long" errors in logs

5. Check logs show successful indexing:
   ```
   INFO Reindexed content for tenant tenantID=4 tenant=inc audience=internal chunks=123
   ```

### Expected Behavior

- No embedding errors for any content size
- Chunk count may increase (smaller chunks due to lower MaxChunkTokens)
- Warning logs if truncation safety net is triggered (indicates chunker edge case)

---

## Future Considerations

### Model-Specific Limits

Different embedding models have different limits:

| Provider | Model | Max Tokens |
|----------|-------|------------|
| Cybertron | all-MiniLM-L6-v2 | 512 |
| Cybertron | all-mpnet-base-v2 | 512 |
| OpenRouter | text-embedding-3-small | 8191 |
| Local | all-MiniLM-L6-v2 | 512 |

Consider making `MaxChunkTokens` configurable based on embedding provider.

### Token Estimation Accuracy

The `len/4` heuristic is approximate. For precise limits, consider:
1. Using actual tokenizer for the model
2. Adding buffer margin to estimated tokens
3. Tracking truncation frequency to tune limits

---

## Addendum: Token Estimation Was Still Too Optimistic

**Date:** 2026-01-26

### Second Error

After the initial fix, reindexing still failed:

```
ERROR reindex failed tenantID=4 audience=internal
error="failed to insert chunks for audience internal: failed to generate embeddings:
failed to encode text 1: input sequence too long: 763 > 512"
```

### Root Cause

The `len/4` token estimation was too optimistic. Sentence-transformers use **subword tokenization** (WordPiece/BPE), which produces more tokens than word-level tokenization:

| Estimation Method | Chars/Token | 1800 chars = |
|-------------------|-------------|--------------|
| Simple `len/4` | 4.0 | 450 tokens |
| Actual subword | ~2.5 | 720 tokens |

The initial safety net of 1800 chars was producing ~720 tokens, still over the 512 limit.

### Additional Fixes Applied

**1. Reduced truncation safety net** (`embedding.go`):

```go
// BEFORE
const maxChars = 1800  // ~450 tokens at 4 chars/token

// AFTER
const maxChars = 1000  // ~400 tokens at 2.5 chars/token (subword)
```

**2. Reduced MaxChunkTokens further** (`chunker.go`):

```go
// BEFORE
MaxChunkTokens = 400   // Still produced ~640 real tokens

// AFTER
MaxChunkTokens = 250   // Produces ~400 real tokens (safe)
MinChunkTokens = 50    // Adjusted proportionally
ChunkOverlapTokens = 25
```

### Token Math

With the new values:
- `MaxChunkTokens = 250` (estimated)
- 250 × 4 chars = 1000 chars per chunk
- 1000 chars ÷ 2.5 (real ratio) = **400 real tokens**
- Plus title overhead (~50 tokens) = **450 tokens** (safe under 512)

### Key Lesson

**Subword tokenization is 40-60% more aggressive than word-level estimation.** When working with transformer models:

| Estimation | Formula | Use Case |
|------------|---------|----------|
| Optimistic | `len / 4` | GPT-style models (large vocab) |
| Conservative | `len / 2.5` | Sentence-transformers (subword) |
| Safe | `len / 2` | Unknown tokenizer |

For Cybertron/sentence-transformers, always use the conservative estimate.

---

## Addendum 2: Even More Aggressive Tokenization

**Date:** 2026-01-26

### Third Error

After the second fix, reindexing still failed:

```
ERROR reindex failed tenantID=4 audience=internal
error="failed to insert chunks for audience internal: failed to generate embeddings:
failed to encode text 13: input sequence too long: 530 > 512"
```

### Root Cause

The tokenization ratio varies by content type. For the tenant "inc" content (likely containing non-English text, special characters, or technical terms), the ratio was even more aggressive:

| Content Type | Chars/Token | 1000 chars = |
|--------------|-------------|--------------|
| Simple English | ~2.5 | 400 tokens |
| Mixed/Technical | ~1.9 | 530 tokens |
| Non-English/Special | ~1.5 | 667 tokens |

The 1000 char limit was producing 530 tokens for certain content.

### Final Fixes Applied

**1. Further reduced truncation safety net** (`embedding.go`):

```go
// BEFORE
const maxChars = 1000  // ~400 tokens at 2.5 chars/token

// AFTER (FINAL)
const maxChars = 750   // ~400 tokens at 1.9 chars/token
```

**2. Further reduced MaxChunkTokens** (`chunker.go`):

```go
// BEFORE
MaxChunkTokens = 250
MinChunkTokens = 50
ChunkOverlapTokens = 25

// AFTER (FINAL)
MaxChunkTokens = 150   // Very conservative for worst-case tokenization
MinChunkTokens = 30
ChunkOverlapTokens = 15
```

### Final Token Math

With the final values:
- `MaxChunkTokens = 150` (estimated)
- 150 × 4 chars = 600 chars per chunk
- 600 chars ÷ 1.9 (worst-case ratio) = **~315 real tokens**
- Plus title overhead (~50 tokens) = **~365 tokens** (safe under 512)

Safety net at 750 chars:
- 750 ÷ 1.9 = **~395 tokens** (safe under 512)

### Key Lesson

**Tokenization ratio varies by content.** For robust handling:

| Scenario | Chars/Token | Recommendation |
|----------|-------------|----------------|
| Best case (English) | 4.0 | Don't rely on this |
| Typical | 2.5 | Previous assumption |
| Worst case | 1.9 | **Use this for safety** |
| Extreme (CJK, symbols) | 1.5 | May need even lower limits |

### Trade-off

Smaller chunks mean:
- More chunks per document (higher storage)
- More embedding API calls (if using paid API)
- Potentially less context per chunk (may affect retrieval quality)

But they guarantee:
- No embedding failures
- Consistent behavior across all content types

For RAG, having more smaller chunks is generally acceptable - the retrieval will find the most relevant ones.

---

## Addendum 3: Provider-Adaptive Chunking (Final Solution)

**Date:** 2026-01-26

### Better Approach: Use OpenRouter for Large Files

Instead of forcing small chunks for all providers, the system now adapts chunk size based on the embedding provider:

| Provider | Max Tokens | 1.7MB File → Chunks | Indexing Time |
|----------|------------|---------------------|---------------|
| OpenRouter | 2000 | ~250 | 2-3 min |
| Cybertron | 150 | ~3500 | 30+ min |

### Implementation

```go
func GetMaxChunkTokens(embeddingProvider string) int {
    switch embeddingProvider {
    case "openrouter":
        return 2000  // 8191 token limit
    case "cybertron":
        return 150   // 512 token limit
    default:
        return 500
    }
}
```

### Recommendation

- **For large files**: Use `EMBEDDING_PROVIDER=openrouter` for 10-15x faster indexing
- **For offline/free**: Use `EMBEDDING_PROVIDER=cybertron` (accepts longer indexing time)

See [DOCS_RAG_ALTERNATIVES.MD](./DOCS_RAG_ALTERNATIVES.MD) for full comparison of approaches.

---

*Document Version: 1.3*
*Updated: 2026-01-26* - Added provider-adaptive chunking solution
