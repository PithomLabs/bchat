# RAG Workflow Clarification

This document clarifies the relationship between **Format for RAG** and **Rebuild Index** features.

## Overview

The Agent Admin provides two seemingly related but **independent** features:

| Feature | Purpose | Data Flow |
|---------|---------|-----------|
| **Format for RAG** | Preview/format content with configurable options | API → Browser (ephemeral) |
| **Rebuild Index** | Generate embeddings and populate VectorDB | Database → VectorDB (persistent) |

**Important:** These features do NOT share data. The output of "Format for RAG" is NOT used by "Rebuild Index".

---

## Feature Details

### Format for RAG Button

**Purpose:** Preview how content will be processed with various options.

**Workflow:**
1. User selects processing options (checkboxes)
2. User clicks "Format for RAG"
3. API processes content according to options
4. Returns formatted content + chunks + stats to browser
5. User can copy output to clipboard
6. **Content is NOT saved to database**

**Use Cases:**
- Preview formatting before uploading KB/Policy files
- Test different chunking strategies
- Validate content structure
- Export processed content for external use

**Output Structure:**
```json
{
  "content": "# Formatted Knowledge Base\n...",
  "chunks": [
    { "id": "kb_section_0", "title": "...", "content": "...", "tokens": 250 }
  ],
  "stats": {
    "original_tokens": 5000,
    "processed_tokens": 4800,
    "chunks_created": 12,
    "faqs_extracted": 5
  }
}
```

---

### Rebuild Index Button

**Purpose:** Regenerate vector embeddings for RAG search.

**Workflow:**
1. Reads KB and Policy content from database (AgentSourceFiles table)
2. Parses content using heading-based chunking
3. Creates `DocumentChunk` objects with full metadata
4. Generates vector embeddings via embedding service
5. Stores chunks with embeddings in VectorDB (LanceDB)

**Triggers:**
- Manual: User clicks "Rebuild Index" button
- Automatic: After KB/Policy file upload
- Startup: When `REINDEX_RAG=true` environment variable is set

**Requirements:**
- Valid embedding provider configuration
- Content exists in AgentSourceFiles table

---

## Why They're Separate

### Design Intent

**Format for RAG** is a **preview tool**:
- Lets users see how content will be processed
- Allows experimentation with options
- Does NOT modify any data

**Rebuild Index** is an **indexing operation**:
- Reads authoritative content from database
- Creates searchable index for chat
- Requires valid embedding provider

### Data Structure Differences

| Aspect | Format for RAG | Rebuild Index |
|--------|----------------|---------------|
| Chunk Type | `ProcessedChunk` | `DocumentChunk` |
| Has TenantID | No | Yes (required) |
| Has AudienceType | No | Yes (required) |
| Has Embedding | No | Yes (generated) |
| Has SourceVersion | No | Yes (required) |

---

## Typical Workflows

### Workflow A: Standard KB Upload
```
1. User uploads KB.MD file
2. System saves to AgentSourceFiles
3. System auto-calls Rebuild Index
4. VectorDB is populated
5. Chat can use RAG search
```

### Workflow B: Using Format for RAG as Preview
```
1. User has raw content
2. User clicks "Format for RAG" with desired options
3. User reviews output in modal
4. User copies formatted content
5. User uploads as KB.MD file (normal upload)
6. System saves to AgentSourceFiles
7. System auto-calls Rebuild Index
8. VectorDB is populated
```

### Workflow C: Generate then Upload
```
1. User clicks "Generate KB.MD" (LLM-based)
2. User reviews annotated output
3. User copies and saves as file
4. User uploads file (normal upload)
5. System processes as in Workflow A
```

---

## FAQ

### Q: Does Format for RAG update the search index?
**A:** No. Format for RAG only returns processed content to your browser. To update the search index, you must upload the content as a KB/Policy file, then the system will auto-index it.

### Q: Why isn't Format for RAG output saved automatically?
**A:** By design, it's a preview/validation tool. This allows you to experiment with different options without affecting live data.

### Q: What happens if Rebuild Index fails?
**A:** Common causes:
- **Embedding provider error**: Check EMBEDDING_PROVIDER setting (use "mock" for testing)
- **No content in database**: Upload KB/Policy files first
- **API rate limits**: Wait and retry

### Q: Are the chunking algorithms the same?
**A:** Similar but not identical. Both use heading-based splitting, but:
- Format for RAG: Configurable via checkboxes
- Rebuild Index: Fixed strategy (H2 → H3 → paragraphs, 100-800 tokens)

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `EMBEDDING_PROVIDER` | `local` | `local`, `openrouter`, or `mock` |
| `EMBEDDING_MODEL` | varies | Model name (depends on provider) |
| `RAG_PIPELINE_ENABLED` | `false` | Enable/disable RAG features |
| `REINDEX_RAG` | `false` | Rebuild all indexes on startup |

### For Testing Without API
```bash
EMBEDDING_PROVIDER=mock task run
```

---

## Related Documentation

- [DOCS_PROCESSING.MD](./DOCS_PROCESSING.MD) - Processing techniques reference
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG architecture overview
- [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD) - Vector database implementation
