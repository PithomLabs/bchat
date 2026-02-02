# RAG System Overview

**Version:** 1.0
**Date:** 2026-02-03
**Related Documents:**
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - Configuration and deployment guide
- [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD) - LanceDB implementation details
- [DOCS_HYBRID_SEARCH.MD](./DOCS_HYBRID_SEARCH.MD) - Hybrid search (vector + BM25)

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Content Types](#content-types)
3. [Auto-Detection Mechanism](#auto-detection-mechanism)
4. [Processing Flow](#processing-flow)
5. [Mode Selection](#mode-selection)
6. [Configuration](#configuration)
7. [Behavior Matrix](#behavior-matrix)
8. [Verification](#verification)
9. [Troubleshooting](#troubleshooting)

---

## Architecture Overview

The RAG (Retrieval-Augmented Generation) system enhances chat responses by retrieving relevant context from indexed content instead of including everything in the system prompt.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         RAG System Architecture                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         FILE UPLOAD                                   │   │
│  │                                                                       │   │
│  │   KB.MD / POLICY.MD / SCRIPT.MD / Plain Text                         │   │
│  │            │                                                          │   │
│  │            ▼                                                          │   │
│  │   ┌────────────────┐                                                 │   │
│  │   │  ParseWithResult │──► IsStructured: true/false                   │   │
│  │   │  (parser.go)     │    ParsedCount: N items                       │   │
│  │   └────────────────┘                                                 │   │
│  │            │                                                          │   │
│  │            ▼                                                          │   │
│  │   ┌────────────────┐     ┌────────────────┐                         │   │
│  │   │ Structured?    │─Yes─►│ ChunkKBContent │                         │   │
│  │   │                │      │ ChunkPolicy... │                         │   │
│  │   └────────────────┘      └────────────────┘                         │   │
│  │            │ No                    │                                  │   │
│  │            ▼                       │                                  │   │
│  │   ┌────────────────┐              │                                  │   │
│  │   │ ChunkRawContent │              │                                  │   │
│  │   │ (chunker.go)    │◄─────────────┘                                 │   │
│  │   └────────────────┘                                                 │   │
│  │            │                                                          │   │
│  │            ▼                                                          │   │
│  │   ┌────────────────┐    ┌────────────────┐    ┌────────────────┐    │   │
│  │   │   Embedding    │───►│   VectorDB     │───►│   Indexed      │    │   │
│  │   │   Service      │    │   Insert       │    │   Chunks       │    │   │
│  │   └────────────────┘    └────────────────┘    └────────────────┘    │   │
│  │                                                                       │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         CHAT PROCESSING                               │   │
│  │                                                                       │   │
│  │   User Message                                                        │   │
│  │        │                                                              │   │
│  │        ▼                                                              │   │
│  │   ┌────────────────┐                                                 │   │
│  │   │ HasStructured? │                                                 │   │
│  │   └────────────────┘                                                 │   │
│  │        │                                                              │   │
│  │   ┌────┴────┐                                                        │   │
│  │   │         │                                                        │   │
│  │   No       Yes                                                        │   │
│  │   │         │                                                        │   │
│  │   ▼         ▼                                                        │   │
│  │ [Force   [Check Token                                                │   │
│  │  RAG]     Threshold]                                                 │   │
│  │   │         │                                                        │   │
│  │   │    ┌────┴────┐                                                   │   │
│  │   │    │         │                                                   │   │
│  │   │  >=30K    <30K                                                   │   │
│  │   │    │         │                                                   │   │
│  │   │    ▼         ▼                                                   │   │
│  │   └──►[RAG]   [Long Context]                                         │   │
│  │        │         │                                                   │   │
│  │        ▼         ▼                                                   │   │
│  │   ┌────────────────┐    ┌────────────────┐                          │   │
│  │   │ buildRAGSystem │    │ buildSystem    │                          │   │
│  │   │ Prompt()       │    │ Prompt()       │                          │   │
│  │   │ (retrieved     │    │ (full KB in    │                          │   │
│  │   │  chunks only)  │    │  prompt)       │                          │   │
│  │   └────────────────┘    └────────────────┘                          │   │
│  │                                                                       │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Content Types

### Structured Content

Content with semantic annotations that the parser can extract into typed objects.

**KB.MD Annotations:**
```markdown
<!-- @service: water_extraction, emergency: true -->
### Water Extraction Service
24/7 emergency water removal and drying services.
**Response Time:** Within 2 hours

<!-- @faq: pricing -->
### How much does water damage restoration cost?
Costs vary based on extent of damage. We provide free estimates.

<!-- @exclusion: mold_remediation -->
### Mold Remediation
We do not provide mold remediation services.
**Recommendation:** Contact a certified mold specialist.
```

**POLICY.MD Annotations:**
```markdown
<!-- @identity -->
**Role:** Customer Service Representative
**Tone:** Professional, empathetic

<!-- @intent: schedule_service, category: standard, urgency: 2 -->
### Schedule Service
Customer wants to book an appointment.
**Examples:** "I need someone to come out", "Can you schedule a visit?"

<!-- @rule: no_pricing, priority: 1 -->
### No Pricing Over Chat
Never quote exact prices. Always offer free on-site estimate.
```

**Parsed Result:**
- `IsStructured: true`
- `ParsedCount: 6` (services, FAQs, exclusions, intents, rules)

### Unstructured Content

Plain text or markdown without semantic annotations. Examples:
- Novels or books
- General documentation
- Plain text FAQs
- Company policy documents
- Training materials

**Example:**
```markdown
# Company Handbook

Welcome to Acme Corp. This handbook covers our policies and procedures.

## Work Hours

Regular business hours are 9 AM to 5 PM, Monday through Friday.
Employees may request flexible scheduling through their manager.

## Dress Code

Business casual attire is required in the office. Jeans are permitted
on Fridays. Client-facing meetings require business professional attire.
```

**Parsed Result:**
- `IsStructured: false`
- `ParsedCount: 0` (no annotations found)

---

## Auto-Detection Mechanism

### Detection Logic

The parser determines content structure during import:

```go
// parser.go

const StructuredContentThreshold = 2

// ParseKBWithResult returns parsing metadata
func (p *Parser) ParseKBWithResult(content string, tenantID int32, audienceType string) (*ParseKBResult, error) {
    kb, err := p.ParseKB(content, tenantID, audienceType)
    if err != nil {
        return nil, err
    }

    // Count total parsed items
    parsedCount := len(kb.Services) + len(kb.FAQs) + len(kb.Exclusions) +
        len(kb.Coverage) + len(kb.Safety) + len(kb.Sections)

    return &ParseKBResult{
        KB:           kb,
        IsStructured: parsedCount >= StructuredContentThreshold,
        ParsedCount:  parsedCount,
    }, nil
}
```

### Threshold

- **Threshold:** 2 items minimum
- **Rationale:** A single annotation could be accidental; 2+ indicates intentional structure
- Files with 0-1 parsed items are classified as **unstructured**
- Files with 2+ parsed items are classified as **structured**

### Combined Assessment

```go
// handlers.go:1103
hasStructuredContent := kbResult.IsStructured || policyResult.IsStructured
```

A tenant has structured content if **either** KB.MD or POLICY.MD is structured.

---

## Processing Flow

### File Upload Flow

```
1. User uploads KB.MD or POLICY.MD
                │
                ▼
2. ParseKBWithResult() / ParsePolicyWithResult()
   - Extracts annotations into typed objects
   - Counts parsed items
   - Sets IsStructured flag
                │
                ▼
3. Log parsing results:
   "parsed content structure tenantID=3 kb_structured=false kb_items=0"
                │
                ▼
4. Choose chunking strategy:
   ┌──────────────────────────────┐
   │ HasStructuredContent = true  │
   │ Use ChunkKBContent() /       │
   │ ChunkPolicyContent() for     │
   │ typed chunking               │
   └──────────────────────────────┘
                OR
   ┌──────────────────────────────┐
   │ HasStructuredContent = false │
   │ Use ChunkRawContent() for    │
   │ paragraph/sentence chunking  │
   └──────────────────────────────┘
                │
                ▼
5. Generate embeddings for chunks
                │
                ▼
6. Insert into VectorDB
                │
                ▼
7. Return AudienceInfo with structure flags
```

### Chat Processing Flow

```
1. User sends chat message
                │
                ▼
2. Load AudienceConfig
   - Includes HasStructuredContent flag
                │
                ▼
3. Determine processing mode:
   ┌─────────────────────────────────────────┐
   │ if !config.HasStructuredContent &&      │
   │    s.UseRAGPipeline():                  │
   │    forceRAG = true  // No choice        │
   │ else if totalTokens >= 30000:           │
   │    useRAG = true    // Too large        │
   │ else:                                   │
   │    useRAG = false   // Long context     │
   └─────────────────────────────────────────┘
                │
                ▼
4a. RAG Mode:
    - Embed user query
    - Search VectorDB for relevant chunks
    - Build focused system prompt with retrieved context
    - Generate response

4b. Long Context Mode:
    - Include full KB/Policy in system prompt
    - Generate response
```

---

## Mode Selection

### Mode Decision Matrix

| HasStructuredContent | RAG Enabled | Token Count | Mode Selected |
|---------------------|-------------|-------------|---------------|
| `false` | `true` | Any | **RAG** (forced) |
| `false` | `false` | Any | **Error** (no content) |
| `true` | `true` | >= 30K | **RAG** (size-based) |
| `true` | `true` | < 30K | **Long Context** |
| `true` | `false` | Any | **Long Context** |

### Why Force RAG for Unstructured?

When content is unstructured (no annotations), there's nothing to build a traditional system prompt from. The parsed KB/Policy objects are empty. RAG mode is required because:

1. **No services/intents to enumerate** - Can't build "You provide these services..."
2. **No identity to establish** - Can't set role/tone from parsed identity block
3. **Content only exists as chunks** - Must retrieve to provide context

### Minimal Identity for Unstructured

When building RAG prompt for unstructured content:

```go
// service.go - buildRAGSystemPrompt()
if !config.HasStructuredContent {
    // Minimal identity for unstructured content
    sb.WriteString("You are a helpful assistant that answers questions ")
    sb.WriteString("based on the provided context.\n\n")
}
```

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `RAG_PIPELINE_ENABLED` | `false` | Enable RAG pipeline |
| `LANCEDB_STORAGE_PROVIDER` | `memory` | `memory`, `local`, or `s3` |
| `EMBEDDING_PROVIDER` | `openrouter` | `mock`, `local`, or `openrouter` |

### Structured Content Threshold

```go
// parser.go
const StructuredContentThreshold = 2
```

To change the threshold, modify this constant. Lower values make it easier to classify content as structured.

---

## Behavior Matrix

### Upload Behavior

| Content Type | Annotations | ParsedCount | IsStructured | Chunking Method |
|-------------|-------------|-------------|--------------|-----------------|
| Annotated KB | Yes | 5+ | `true` | `ChunkKBContent()` |
| Plain markdown | No | 0 | `false` | `ChunkRawContent()` |
| Mixed (some annotations) | Partial | 1 | `false` | `ChunkRawContent()` |
| Mixed (enough annotations) | Partial | 3+ | `true` | `ChunkKBContent()` |

### Chat Behavior

| Structured | RAG Enabled | Result |
|-----------|-------------|--------|
| `true` | `true` | Normal RAG or long-context based on size |
| `true` | `false` | Long-context mode with full KB |
| `false` | `true` | Forced RAG with minimal identity |
| `false` | `false` | Error - no usable content |

---

## Verification

### Check Parsing Results

After file upload, check server logs:

```
INFO parsed content structure tenantID=3 audience=internal kb_structured=true kb_items=12 policy_structured=true policy_items=8
```

or for unstructured:

```
INFO parsed content structure tenantID=3 audience=internal kb_structured=false kb_items=0 policy_structured=false policy_items=0
```

### Check RAG Mode Selection

During chat processing:

```
INFO Using RAG pipeline for response generation (forced: no structured content)
```

or:

```
INFO Using RAG pipeline for response generation (content exceeds threshold)
```

or:

```
INFO Using long-context mode for response generation
```

### API Response

The `AudienceInfo` response includes structure flags:

```json
{
  "services_count": 0,
  "intents_count": 0,
  "faqs_count": 0,
  "rules_count": 0,
  "kb_is_structured": false,
  "policy_is_structured": false,
  "has_structured_content": false
}
```

---

## Troubleshooting

### "No usable content" Error

**Cause:** Unstructured content uploaded but RAG pipeline is disabled.

**Solution:** Enable RAG pipeline:
```bash
export RAG_PIPELINE_ENABLED=true
```

### Content Not Being Retrieved

**Cause:** Chunks not indexed properly.

**Verification:**
1. Check indexing logs after upload
2. Verify VectorDB has chunks: `GET /api/v1/agent/:slug/rag/stats`
3. Try manual search: `POST /api/v1/agent/:slug/rag/search`

### Poor Retrieval Quality

**Cause:** Using mock embeddings (random vectors).

**Solution:** Use real embeddings:
```bash
export EMBEDDING_PROVIDER=openrouter
export OPENROUTER_API_KEY=sk-or-xxxxx
```

### Structured Content Not Detected

**Cause:** Annotations not matching expected format.

**Verification:** Check annotation format:
- Must use `<!-- @type: params -->` format
- Must have at least 2 parseable items
- Check for typos in annotation type names

**Valid annotation types:**
- KB: `@service`, `@exclusion`, `@faq`, `@coverage`, `@safety`, `@section`
- Policy: `@identity`, `@intent`, `@rule`, `@thresholds`

---

## Key Files

| File | Purpose |
|------|---------|
| `parser.go` | `ParseKBWithResult()`, `ParsePolicyWithResult()`, `StructuredContentThreshold` |
| `chunker.go` | `ChunkKBContent()`, `ChunkPolicyContent()`, `ChunkRawContent()` |
| `handlers.go` | `importFiles()`, `AudienceInfo` struct |
| `service.go` | `AudienceConfig.HasStructuredContent`, mode selection in `processChat()` |

---

## Use Cases

### Traditional Service Business

- **Content:** Annotated KB.MD with services, FAQs, coverage areas
- **Behavior:** Normal structured processing, RAG or long-context based on size
- **Example:** Water damage restoration company

### Document Q&A

- **Content:** Plain text document (novel, manual, policy document)
- **Behavior:** Forced RAG mode with minimal identity
- **Example:** "Chat with this PDF" style application

### Mixed Content

- **Content:** Some annotations (e.g., identity in POLICY.MD) but mostly prose
- **Behavior:** If >= 2 items parsed, structured; otherwise unstructured
- **Tip:** Add at least 2 annotations to get structured treatment

---

*Document Version: 1.0*
*Created: 2026-02-03*
*Author: Claude Code*
