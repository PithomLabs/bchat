# Embedding Methods Documentation

## Overview

This document explains the embedding implementation in the bchat RAG pipeline, addressing the question of whether user-selectable embedding methods (Word2Vec, GloVe, BERT) are implemented.

---

## Quick Answer

**Is embedding method selection (Word2Vec, GloVe, BERT) implemented for users to choose after file upload?**

**No.** The current implementation does not provide a user interface for selecting embedding methods. Embedding configuration is done at the **server level** via environment variables, not per-user or per-upload.

| Feature | Status |
|---------|--------|
| Word2Vec | Not implemented |
| GloVe | Not implemented |
| BERT | Partially (via all-MiniLM-L6-v2, a BERT distillate) |
| User selection UI | Not implemented |
| Per-upload selection | Not implemented |

---

## Current Implementation

### Source File
`server/router/api/v1/agent/embedding.go`

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    EmbeddingService Interface               │
├─────────────────────────────────────────────────────────────┤
│  Embed(ctx, texts []string) → ([][]float32, error)          │
│  Dimension() → int                                          │
│  Provider() → string                                        │
└─────────────────────────────────────────────────────────────┘
                              │
         ┌────────────────────┼────────────────────┐
         │                    │                    │
         ▼                    ▼                    ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│  MockEmbedding  │  │ LocalEmbedding  │  │OpenRouterEmbed. │
├─────────────────┤  ├─────────────────┤  ├─────────────────┤
│ Hash-based      │  │ HTTP endpoint   │  │ OpenRouter API  │
│ Dimension: 384  │  │ all-MiniLM-L6-v2│  │ text-embed-3-*  │
│ No API needed   │  │ Dimension: 384  │  │ Dimension: 1536 │
└─────────────────┘  └─────────────────┘  └─────────────────┘
```

### Supported Providers

#### 1. Mock Provider (`EMBEDDING_PROVIDER=mock`)

**Purpose:** Testing the RAG pipeline without external dependencies.

| Attribute | Value |
|-----------|-------|
| Model | N/A (deterministic hash) |
| Dimension | 384 |
| External dependency | None |
| Use case | Unit tests, CI/CD |

**How it works:** Generates deterministic pseudo-random vectors based on text hash using a linear congruential generator. The same text always produces the same embedding, enabling reproducible search results.

```go
// Hash-based deterministic embedding
hash := uint64(0)
for _, c := range text {
    hash = hash*31 + uint64(c)
}
```

#### 2. Local Provider (`EMBEDDING_PROVIDER=local`)

**Purpose:** Development and QA testing with a local embedding server.

| Attribute | Value |
|-----------|-------|
| Model | all-MiniLM-L6-v2 (default) |
| Dimension | 384 |
| External dependency | Local HTTP server |
| Use case | Development, testing |

**Model Details:**
- **all-MiniLM-L6-v2** is a sentence-transformer model based on **BERT architecture**
- Trained via knowledge distillation from larger BERT models
- Optimized for semantic similarity tasks
- 384-dimensional output vectors

**Required setup:**
```bash
# Start local embedding server (Python FastAPI with sentence-transformers)
EMBEDDING_LOCAL_ENDPOINT=http://localhost:8001/embed
```

**Request format:**
```json
{
  "texts": ["How do I file a claim?", "Water damage restoration"],
  "model": "all-MiniLM-L6-v2"
}
```

#### 3. OpenRouter Provider (`EMBEDDING_PROVIDER=openrouter`)

**Purpose:** Production deployment using OpenAI embeddings via OpenRouter.

| Attribute | Value |
|-----------|-------|
| Model | openai/text-embedding-3-small (default) |
| Dimension | 1536 |
| External dependency | OpenRouter API |
| Use case | Production |

**Supported models:**
| Model | Dimension |
|-------|-----------|
| openai/text-embedding-3-small | 1536 |
| openai/text-embedding-3-large | 3072 |
| openai/text-embedding-ada-002 | 1536 |

**Required:**
```bash
OPENROUTER_API_KEY=sk-or-v1-...
```

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `EMBEDDING_PROVIDER` | `local` | Provider: `mock`, `local`, or `openrouter` |
| `EMBEDDING_MODEL` | varies by provider | Model identifier |
| `EMBEDDING_LOCAL_ENDPOINT` | `http://localhost:8001/embed` | Local server URL |
| `OPENROUTER_API_KEY` | (required for openrouter) | API key |

### Example Configurations

**Development (no external dependencies):**
```bash
EMBEDDING_PROVIDER=mock
RAG_PIPELINE_ENABLED=true
```

**Development with local server:**
```bash
EMBEDDING_PROVIDER=local
EMBEDDING_MODEL=all-MiniLM-L6-v2
EMBEDDING_LOCAL_ENDPOINT=http://localhost:8001/embed
RAG_PIPELINE_ENABLED=true
```

**Production:**
```bash
EMBEDDING_PROVIDER=openrouter
EMBEDDING_MODEL=openai/text-embedding-3-small
OPENROUTER_API_KEY=sk-or-v1-...
RAG_PIPELINE_ENABLED=true
```

---

## Embedding Methods Comparison

### Traditional Methods (NOT Implemented)

| Method | Year | Approach | Limitations |
|--------|------|----------|-------------|
| **Word2Vec** | 2013 | Predict context words | No sentence-level semantics |
| **GloVe** | 2014 | Global word co-occurrence | Static embeddings, no context |

These methods produce **word-level** embeddings and are not well-suited for RAG applications that require semantic understanding of sentences and paragraphs.

### Modern Methods (Implemented)

| Method | Year | Approach | Advantages |
|--------|------|----------|------------|
| **BERT-based** (MiniLM) | 2020 | Transformer encoder | Contextual, semantic similarity |
| **OpenAI Embeddings** | 2022+ | Proprietary transformer | High quality, optimized for retrieval |

The current implementation uses modern transformer-based embeddings which capture:
- Contextual word meanings
- Sentence-level semantics
- Query-document similarity

---

## Why No User Selection?

The current architecture uses server-level configuration for several reasons:

1. **Consistency:** All documents in the vector database must use the same embedding model for search to work correctly. Mixing embeddings from different models would produce meaningless similarity scores.

2. **Simplicity:** Server-level configuration avoids complexity of managing multiple embedding indices per model.

3. **Performance:** A single embedding model can be loaded once and reused, rather than switching between models per request.

4. **Index coherence:** LanceDB stores vectors with a fixed dimension. Changing models would require re-indexing all content.

---

## Future Enhancement: User-Selectable Embeddings

If user-selectable embedding methods are desired, the implementation would require:

### Option A: Per-Tenant Configuration

```go
// In agent_tenant table
type TenantConfig struct {
    EmbeddingProvider string // "local" or "openrouter"
    EmbeddingModel    string // Model identifier
}
```

**Pros:** Each tenant can optimize for their domain
**Cons:** Separate vector indices per tenant, increased storage

### Option B: UI Model Selection

Add to Agent Admin UI:
```
┌──────────────────────────────────────┐
│ Embedding Configuration              │
├──────────────────────────────────────┤
│ Provider: [Local ▼]                  │
│ Model:    [all-MiniLM-L6-v2 ▼]       │
│           [BERT-base]                │
│           [OpenAI text-embedding-3]  │
│                                      │
│ [Rebuild Index]                      │
└──────────────────────────────────────┘
```

**Note:** Changing the model requires full re-indexing of all content.

### Option C: Multi-Model Index

Store multiple embedding vectors per document:
```go
type DocumentChunk struct {
    // ... existing fields
    EmbeddingMiniLM  []float32 // 384-dim
    EmbeddingOpenAI  []float32 // 1536-dim
}
```

**Pros:** Instant model switching, A/B testing
**Cons:** Significantly increased storage and indexing time

---

## Summary

| Question | Answer |
|----------|--------|
| Is Word2Vec implemented? | No |
| Is GloVe implemented? | No |
| Is BERT implemented? | Yes, via all-MiniLM-L6-v2 (BERT distillate) |
| Can users select embedding method? | No, server-level config only |
| What models are available? | MiniLM (local), OpenAI text-embedding-3 (production) |

The current implementation provides production-ready embedding capabilities using modern transformer models. User-selectable embedding methods could be added as a future enhancement if required.

---

## Related Documentation

- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG architecture overview
- [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD) - LanceDB implementation details
- [DOCS_LANCEDB_STORAGE.MD](./DOCS_LANCEDB_STORAGE.MD) - Storage and inspection
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Environment variables reference
