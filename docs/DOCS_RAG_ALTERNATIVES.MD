# RAG Processing Alternatives

**Status:** Active
**Date:** 2026-01-26
**Related Documents:**
- [DOCS_CYBERTRON.MD](./DOCS_CYBERTRON.MD) - Cybertron embedding implementation
- [DOCS_CYBERTRON_DEBUG.MD](./DOCS_CYBERTRON_DEBUG.MD) - Sequence length fixes
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG pipeline overview

---

## Overview

This document explains the provider-adaptive chunk sizing system and alternatives for processing large knowledge base files.

---

## Provider-Adaptive Chunk Sizing

### The Problem

Different embedding providers have different token limits:

| Provider | Model | Max Tokens | Recommended Chunk Size |
|----------|-------|------------|------------------------|
| OpenRouter | text-embedding-3-small | 8,191 | 2,000 tokens |
| Cybertron | all-MiniLM-L6-v2 | 512 | 150 tokens |
| Local | all-MiniLM-L6-v2 | 512 | 150 tokens |

A 1.7MB KB file produces:
- **OpenRouter**: ~250 chunks → 2-3 min indexing
- **Cybertron**: ~3,500 chunks → 30+ min indexing

### The Solution

Chunk size is now automatically configured based on embedding provider:

```go
func GetMaxChunkTokens(embeddingProvider string) int {
    switch embeddingProvider {
    case "openrouter":
        return 2000  // text-embedding-3-small: 8191 token limit
    case "cybertron":
        return 150   // 512 token limit with subword tokenization
    case "local":
        return 150   // Same as Cybertron
    default:
        return 500
    }
}
```

### Usage

The chunk size is automatically selected when you run with different providers:

```bash
# OpenRouter (fast, larger chunks)
EMBEDDING_PROVIDER=openrouter task run:rag

# Cybertron (offline, smaller chunks)
EMBEDDING_PROVIDER=cybertron task run:rag:cybertron
```

Logs will show the configuration:
```
INFO Starting RAG reindex for tenant tenantID=4 embeddingProvider=openrouter maxChunkTokens=2000
```

---

## Comparison of Approaches

### Current: Direct Chunking

```
KB.MD → Chunk by headers → Embed all chunks → Store in LanceDB
```

| Provider | Chunks | Time | Cost | Quality |
|----------|--------|------|------|---------|
| OpenRouter | ~250 | 2-3 min | $0.02 | High detail |
| Cybertron | ~3500 | 30+ min | Free | High detail |

### Alternative: Pre-Summarization

```
KB.MD → LLM Summarization → Chunk summaries → Embed → Store
```

| Method | Chunks | Time | Cost | Quality |
|--------|--------|------|------|---------|
| NotebookLM | ~100 | Manual | Free | Good |
| OpenRouter API | ~200 | 2-5 min | $1-2 | Good |
| Ollama (local) | ~200 | 5-10 min | Free | Medium |

**When to use summarization:**
- Files > 2MB
- Overview queries more common than specific lookups
- Cost per query matters (fewer chunks = faster retrieval)

**When NOT to use summarization:**
- Specific details matter (phone numbers, addresses, exact procedures)
- Legal/compliance content where exact wording matters
- FAQ-style content where Q&A pairs should remain intact

---

## Offline Fallback Strategy

The system supports automatic fallback for offline scenarios:

1. **Primary**: OpenRouter with large chunks (fast)
2. **Fallback**: Cybertron with small chunks (offline)

To enable:
```bash
# Use Cybertron for offline capability
EMBEDDING_PROVIDER=cybertron task run:rag:cybertron
```

---

## Future Enhancements

### Hierarchical Indexing (Not Implemented)

Create multiple index levels for different query types:
- Level 1: Large chunks (2000 tokens) for overview queries
- Level 2: Small chunks (150 tokens) for specific lookups

### Content-Aware Chunking (Not Implemented)

Different chunk strategies per content type:
- FAQs: Preserve Q&A pairs as single chunks
- Procedures: Keep step sequences together
- Coverage areas: One chunk per area

### Hybrid Summarization (Not Implemented)

Store both summaries and originals:
1. Search summaries (fast)
2. Retrieve original sections (full detail)

---

## Configuration Reference

### Environment Variables

| Variable | Values | Default | Description |
|----------|--------|---------|-------------|
| `EMBEDDING_PROVIDER` | `openrouter`, `cybertron`, `local`, `mock` | `cybertron` | Embedding provider |
| `RAG_PIPELINE_ENABLED` | `true`, `false` | `false` | Enable RAG pipeline |

### Chunk Size by Provider

| Provider | Max Tokens | ~Chars | Min Tokens |
|----------|------------|--------|------------|
| openrouter | 2000 | 8000 | 100 |
| cybertron | 150 | 600 | 30 |
| local | 150 | 600 | 30 |
| mock | 500 | 2000 | 50 |

---

## Troubleshooting

### Slow Indexing with Cybertron

**Expected behavior**: Cybertron uses small chunks (150 tokens) due to 512 token limit.

**Solutions:**
1. Use OpenRouter for faster indexing: `EMBEDDING_PROVIDER=openrouter`
2. Accept longer indexing time (one-time per file upload)
3. Consider pre-summarizing very large files

### "Sequence too long" Errors

See [DOCS_CYBERTRON_DEBUG.MD](./DOCS_CYBERTRON_DEBUG.MD) for details.

**Quick fix**: Ensure you're using the latest code with provider-adaptive chunking.

---

*Document Version: 1.0*
*Created: 2026-01-26*
