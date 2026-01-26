# Document Processing Techniques

This document describes all processing techniques available for preparing content for RAG (Retrieval-Augmented Generation) indexing.

## Overview

The Agent Admin provides two processing approaches:

| Approach | Button | Description |
|----------|--------|-------------|
| **Rule-Based** | "Format for RAG" | Fast, deterministic, no API calls. Uses configurable checkboxes. |
| **LLM-Based** | "Generate KB.MD" / "Generate POLICY.MD" | Uses Gemini 2.5 Pro to semantically annotate content. |

---

## LLM-Based Processing (Generate Buttons)

### How It Works

1. Raw content is fetched from the database
2. Content is sent to OpenRouter API (Gemini 2.5 Pro)
3. LLM applies semantic annotations based on structured prompt
4. Annotated content is returned for user review

### Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `LLM_MODEL_REASONING` | `google/gemini-2.5-pro` | Model for annotation tasks |
| `OPENROUTER_API_KEY` | (required) | API key for OpenRouter |

### Annotation Types (KB.MD)

| Annotation | Purpose | Format |
|------------|---------|--------|
| `@section` | General content sections | `<!-- @section: code, type: category -->` |
| `@service` | Services/products offered | `<!-- @service: code, emergency: true/false -->` |
| `@faq` | Frequently asked questions | `<!-- @faq: code -->` |
| `@exclusion` | Things NOT offered | `<!-- @exclusion: code, exception: "..." -->` |
| `@coverage` | Geographic/scope areas | `<!-- @coverage: include/exclude -->` |
| `@safety` | Safety protocols | `<!-- @safety: code, triggers: intent1, intent2 -->` |

### Annotation Types (POLICY.MD)

| Annotation | Purpose | Format |
|------------|---------|--------|
| `@identity` | Agent identity definition | `<!-- @identity: code -->` |
| `@intent` | User intent classification | `<!-- @intent: code, category: emergency/standard/meta, urgency: 0-5 -->` |
| `@rule` | Policy rules | `<!-- @rule: code, priority: 1-10 -->` |
| `@thresholds` | Escalation thresholds | `<!-- @thresholds: code -->` |

---

## Rule-Based Processing (Format for RAG)

### Content Extraction Options

| Option | Description | Implementation |
|--------|-------------|----------------|
| **Extract FAQs** | Identifies Q&A patterns | Regex: `Q:.*?\nA:.*?(?=\n\n|$)` and similar |
| **Extract Services** | Identifies service listings | Header + bullet pattern detection |
| **Extract Exclusions** | Identifies "not offered" items | Keywords: "not", "except", "excluding" |
| **Extract Coverage** | Identifies geographic mentions | State codes, city names, zip patterns |
| **Extract Safety** | Identifies safety instructions | Keywords: "emergency", "warning", "danger" |

### Text Normalization Options

| Option | Description | Implementation |
|--------|-------------|----------------|
| **Remove Extra Whitespace** | Collapse multiple spaces/newlines | `strings.TrimSpace()` + regex |
| **Normalize Line Endings** | Convert CRLF to LF | `strings.ReplaceAll("\r\n", "\n")` |
| **Strip HTML Tags** | Remove HTML, keep text | Go html package or regex |
| **Fix Encoding** | Handle UTF-8/ASCII issues | Unicode normalization |
| **Remove Page Numbers** | Strip "Page X" patterns | Regex: `Page \d+ of \d+` |
| **Remove Headers/Footers** | Strip repeated text | Duplicate line detection |

### Structure Splitting Options

| Option | Description | Constants |
|--------|-------------|-----------|
| **Split by H2 Headers** | Create chunks at `## ` | Major section boundaries |
| **Split by H3 Headers** | Further split at `### ` | Subsection boundaries |
| **Split by Paragraphs** | Split at `\n\n` | Fallback for unstructured text |
| **Split by Sentences** | Fine-grained splitting | Period + space detection |
| **Preserve List Items** | Keep lists together | Bullet/number pattern grouping |

### Chunking Controls

| Option | Default | Range | Description |
|--------|---------|-------|-------------|
| **Max Chunk Size** | 800 tokens | 400-1200 | Maximum tokens per chunk |
| **Min Chunk Size** | 100 tokens | 50-200 | Merge chunks below this |
| **Chunk Overlap** | 50 tokens | 0-100 | Context preservation between chunks |
| **Merge Small Chunks** | On | On/Off | Combine tiny adjacent chunks |

### Metadata Options

| Option | Description |
|--------|-------------|
| **Generate Titles** | Auto-generate titles from first line/header |
| **Add Source Reference** | Include filename in chunk metadata |
| **Preserve Hierarchy** | Create breadcrumb titles (Section > Subsection) |

---

## Token Estimation

The chunker estimates tokens using: `tokens = characters / 4`

This is a rough approximation suitable for English text. Actual tokenization varies by model.

---

## Chunking Algorithm

```
1. If content has H2 headers:
   - Split by ## headers into sections
   - For each section:
     a. If tokens <= MaxChunkSize: create single chunk
     b. Else if has H3 headers: split by ### and recurse
     c. Else: split by paragraphs

2. Apply MinChunkSize filter:
   - Merge adjacent chunks below minimum
   - Maintain MaxChunkSize limit

3. Generate metadata:
   - Extract title from header or first line
   - Build hierarchical title if nested
   - Assign unique chunk IDs
```

---

## API Reference

### Format for RAG Endpoint

```
POST /api/v1/agent/:slug/format-for-rag
Authorization: Bearer <token>
Content-Type: application/json

Request Body:
{
  "content": "raw text content...",
  "file_type": "kb" | "policy",
  "options": {
    "extract_faqs": true,
    "extract_services": true,
    "extract_exclusions": false,
    "extract_coverage": false,
    "extract_safety": false,
    "remove_whitespace": true,
    "strip_html": false,
    "fix_encoding": false,
    "remove_page_numbers": false,
    "split_h2": true,
    "split_h3": true,
    "split_paragraphs": true,
    "split_sentences": false,
    "preserve_lists": true,
    "max_chunk_size": 800,
    "min_chunk_size": 100,
    "chunk_overlap": 50,
    "merge_small_chunks": true,
    "generate_titles": true,
    "add_source_ref": false,
    "preserve_hierarchy": true
  }
}

Response:
{
  "content": "# Formatted Content\n\n## Section 1\n...",
  "chunks": [
    {
      "id": "1:external:kb_section:section_0",
      "title": "Section 1",
      "content": "...",
      "tokens": 245
    }
  ],
  "stats": {
    "original_tokens": 5000,
    "processed_tokens": 4800,
    "chunks_created": 12,
    "faqs_extracted": 5,
    "services_extracted": 3
  }
}
```

### Generate KB.MD Endpoint (existing)

```
POST /api/v1/agent/:slug/generate-kb
Authorization: Bearer <token>

Response:
{
  "content": "# Company Knowledge Base\n\n<!-- @section: ... -->..."
}
```

### Generate POLICY.MD Endpoint (existing)

```
POST /api/v1/agent/:slug/generate-policy
Authorization: Bearer <token>

Response:
{
  "content": "# Company Policy\n\n<!-- @identity: ... -->..."
}
```

---

## Best Practices

### When to Use Each Approach

| Scenario | Recommended Approach |
|----------|---------------------|
| Well-structured markdown | Format for RAG (faster, cheaper) |
| Messy PDF extraction | Format for RAG + Remove whitespace/page numbers |
| Raw unstructured text | Generate KB.MD (LLM annotation) |
| Need semantic annotations | Generate KB.MD / POLICY.MD |
| Cost-sensitive environment | Format for RAG only |
| High-quality requirements | Generate -> then Format for fine-tuning |

### Recommended Default Options

For most documents:
- Remove Extra Whitespace
- Split by H2/H3 Headers
- Split by Paragraphs
- Preserve List Items
- Merge Small Chunks
- Generate Titles
- Preserve Hierarchy
- Max Chunk: 800 tokens
- Min Chunk: 100 tokens

---

## Related Documentation

- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG architecture overview
- [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD) - Vector database implementation
- [DOCS_HYBRID_SEARCH.MD](./DOCS_HYBRID_SEARCH.MD) - Hybrid search (vector + BM25)
