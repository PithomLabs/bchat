# Adapted lanceEmbeddings Techniques for bchat

**Date:** 2026-01-27
**Reference:** Analysis from `DOCS_LANCEDB_CHATGPT.MD`

---

## Overview

This document describes the lanceEmbeddings techniques adapted for bchat's RAG pipeline, focusing on:
1. Fallback chunking for unannotated markdown
2. Heading-based splitting algorithm
3. Combined title+content embedding
4. Encoding resilience
5. Data inspection tools
6. Progress streaming

---

## 1. Fallback Chunking Implementation

### Problem
bchat's KB parser requires HTML annotations (`<!-- @section -->`). Unannotated content is silently discarded.

### Solution
Added fallback to `ChunkMarkdownContent()` when no annotations are found.

### Code Location
- `server/router/api/v1/agent/handlers.go` - `indexContentForRAG()` function

### Behavior
| Content Type | Chunking Method |
|--------------|-----------------|
| With annotations | `ChunkKBContent()` (structured) |
| Without annotations | `ChunkMarkdownContent()` (heading-based) |

### Detection Logic
```go
hasKBContent := func(k *ParsedKB) bool {
    return k != nil && (len(k.Services) > 0 || len(k.FAQs) > 0 ||
        len(k.Safety) > 0 || len(k.Sections) > 0 ||
        len(k.Exclusions) > 0 || len(k.Coverage) > 0)
}
```

### Log Output
When fallback is used, you'll see:
```
INFO No KB annotations found, using markdown-based chunking tenantID=3 audience=external
```

---

## 2. Heading-Based Splitting Algorithm

### Technique from lanceEmbeddings
Split markdown by headers instead of requiring annotations.

### Implementation in `ChunkMarkdownContent()` (chunker.go:531-656)

**Splitting Priority:**
1. Split by H2 headers (`## `)
2. If sections too large, split by H3 (`### `)
3. If still too large, split by paragraphs
4. If paragraphs too large, split by sentences
5. Merge undersized chunks to meet minimum threshold

### Configuration
```go
func GetMaxChunkTokens(embeddingProvider string) int {
    switch embeddingProvider {
    case "openrouter": return 2000  // Better coherence
    case "cybertron":  return 150   // Development default
    case "local":      return 150
    case "mock":       return 500
    default:           return 500
    }
}
```

---

## 3. Combined Title+Content Embedding

### Technique from lanceEmbeddings
```python
combined = f"{title} {subtitle} {blog_text}".strip()
embedding = model.encode(combined)
```

### bchat Implementation
When creating chunks, combine title and content for richer semantic embeddings:
```go
// In chunker.go - each chunk includes both title and content
chunk := DocumentChunk{
    Title:   sectionTitle,   // From H2/H3 header
    Content: sectionContent, // Content after header
    // Embedding is generated from Title + Content combined
}
```

---

## 4. Encoding Resilience (Priority 2 from Recommendations)

### Technique from lanceEmbeddings
```python
try:
    html = path.read_text(encoding="utf-8", errors="strict")
except Exception:
    html = path.read_text(encoding="utf-8", errors="replace")
```

### bchat Implementation (handlers.go)
```go
func sanitizeEncoding(content []byte) []byte {
    // Replace invalid UTF-8 sequences
    return bytes.ToValidUTF8(content, []byte("?"))
}

// In file upload handler
content, err := io.ReadAll(file)
if !utf8.Valid(content) {
    slog.Warn("encoding issues detected, replacing invalid characters",
        "file", filename)
    content = sanitizeEncoding(content)
}
```

**Status:** To be implemented

---

## 5. Data Inspection Tools (Priority 1 from Recommendations)

### Technique from lanceEmbeddings
DuckDB SQL pipeline for debugging:
```python
con.execute("SELECT * FROM posts_with_embeddings LIMIT 10")
```

### bchat Implementation - New Debug Endpoint

**Endpoint:** `GET /api/v1/agent/:slug/debug/vectordb`

**Response:**
```json
{
  "totalChunks": 150,
  "byContentType": {
    "service": 20,
    "faq": 45,
    "kb_section": 85
  },
  "byAudience": {
    "internal": 80,
    "external": 70
  },
  "embeddingDimension": 384,
  "sampleChunks": [
    {"id": "3:internal:kb_section:INTRO_1", "title": "Introduction", "contentPreview": "..."}
  ]
}
```

**Status:** To be implemented

---

## 6. Reindex Progress Streaming (Priority 3 from Recommendations)

### Technique from lanceEmbeddings
```python
embeddings = model.encode(combined_texts, show_progress_bar=True)
```

### bchat Implementation - SSE Progress Events

**During reindexing, emit progress events:**
```go
// In HandleReindexTenant
for i, batch := range batches {
    embeddings, _ := embedSvc.Embed(ctx, batch)

    // Send SSE progress event
    progress := ReindexProgress{
        Phase:      "embedding",
        Current:    i + 1,
        Total:      len(batches),
        Percentage: float64(i+1) / float64(len(batches)) * 100,
    }
    sendSSE(w, "progress", progress)
}
```

**Status:** To be implemented

---

## 7. Column Alias Handling (Priority 4 from Recommendations)

### Technique from lanceEmbeddings
```python
col_map = {
    "subtitle": "sub_title",
    "sub_title": "sub_title",
    "date": "post_date",
}
```

### bchat Implementation (parser.go)
```go
var headerAliases = map[string]string{
    "services":     "service",
    "service":      "service",
    "faqs":         "faq",
    "questions":    "faq",
    "rules":        "rule",
    "policies":     "rule",
    "safety":       "safety",
    "emergency":    "safety",
}

func normalizeAnnotationType(t string) string {
    t = strings.ToLower(strings.TrimSpace(t))
    if alias, ok := headerAliases[t]; ok {
        return alias
    }
    return t
}
```

**Status:** To be implemented

---

## 8. Pipeline Debug Endpoints (Priority 5 from Recommendations)

### bchat Implementation - Debug Pipeline View

**Endpoint:** `GET /api/v1/agent/:slug/debug/pipeline`

**Response:**
```json
{
  "stages": [
    {
      "name": "raw_input",
      "inputCount": 1,
      "outputCount": 1,
      "sampleData": "# Introduction\nThis is..."
    },
    {
      "name": "parsed",
      "inputCount": 1,
      "outputCount": 0,
      "note": "No annotations found"
    },
    {
      "name": "chunked",
      "inputCount": 1,
      "outputCount": 15,
      "method": "markdown-based",
      "sampleData": [{"title": "Introduction", "tokens": 120}]
    },
    {
      "name": "embedded",
      "inputCount": 15,
      "outputCount": 15,
      "dimension": 384
    }
  ],
  "errors": []
}
```

**Status:** To be implemented

---

## Summary of Adapted Techniques

| Technique | Source | Status | Files |
|-----------|--------|--------|-------|
| Fallback chunking | lanceEmbeddings | **Implemented** | handlers.go |
| Heading-based splitting | lanceEmbeddings | Existing | chunker.go |
| Combined title+content | lanceEmbeddings | Existing | chunker.go |
| Encoding resilience | lanceEmbeddings | To implement | handlers.go |
| Data inspection | lanceEmbeddings | To implement | handlers.go |
| Progress streaming | lanceEmbeddings | To implement | handlers.go |
| Column aliases | lanceEmbeddings | To implement | parser.go |
| Pipeline debug | lanceEmbeddings | To implement | handlers.go |

---

## Testing

### Test 1: Fallback Chunking
```bash
# Upload unannotated KB.MD
curl -X POST http://localhost:8081/api/v1/agent/test/import \
  -F "file_type=kb" \
  -F "audience_type=external" \
  -F "file=@unannotated_kb.md"

# Check logs for: "No KB annotations found, using markdown-based chunking"
```

### Test 2: Verify Chunks Created
After uploading unannotated content, verify that chunks were created by querying the chat endpoint - responses should now include relevant content from the KB.

### Test 3: Regression with Annotations
Upload a properly annotated KB.MD and verify that annotation-based chunking is still used (no "markdown-based chunking" log message).

---

## Related Documentation

- `DOCS_LANCEDB_CHATGPT.MD` - Full comparison analysis of lanceEmbeddings vs bchat
- `DOCS_RAG_PIPELINE.MD` - RAG pipeline architecture
- `DOCS_HYBRID_SEARCH.MD` - Hybrid search implementation
