# Hybrid Chunking Strategy

## Overview

Replace the brittle annotation-based parsing system with a hybrid approach:

1. **Long Context Mode** (< 30K tokens): Load full KB/Policy directly into system prompt
2. **RAG Mode** (≥ 30K tokens): Use heading-based + paragraph fallback chunking

This eliminates the need for manual annotations (`<!-- @chunk:service -->`) while maintaining effective retrieval for large knowledge bases.

---

## Decision Logic

```
┌─────────────────────────────────────────────────────────────┐
│                    On KB/Policy Upload                       │
├─────────────────────────────────────────────────────────────┤
│  1. Calculate token count of KB + Policy content            │
│  2. Store token count in tenant config                      │
│                                                             │
│  IF total_tokens < 30,000:                                  │
│    → Mark tenant as "long_context" mode                     │
│    → Skip RAG indexing                                      │
│                                                             │
│  ELSE:                                                      │
│    → Mark tenant as "rag" mode                              │
│    → Run heading-based chunker                              │
│    → Index chunks in vector DB                              │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                    On Chat Request                           │
├─────────────────────────────────────────────────────────────┤
│  IF tenant.retrieval_mode == "long_context":                │
│    → Load full KB + Policy into system prompt               │
│    → No RAG retrieval                                       │
│                                                             │
│  ELSE (rag mode):                                           │
│    → Query vector DB for relevant chunks                    │
│    → Build prompt with retrieved context only               │
└─────────────────────────────────────────────────────────────┘
```

---

## Heading-Based Chunker (for RAG mode)

### Algorithm

```
1. Primary split on ## headers (H2 - major sections)
2. Secondary split on ### headers (H3 - subsections) if section > 800 tokens
3. Fallback to paragraph-based split if no headers found
4. Size guardrails: min 100 tokens, max 800 tokens per chunk
5. Overlap: 50 tokens between chunks for context continuity
```

### Pseudocode

```go
func ChunkByHeadings(content string) []Chunk {
    chunks := []Chunk{}

    // Step 1: Split by ## headers
    h2Sections := splitByRegex(content, `(?m)^## `)

    for _, section := range h2Sections {
        title := extractFirstLine(section)
        body := removeFirstLine(section)
        tokens := countTokens(body)

        if tokens <= 800 {
            // Section fits in one chunk
            chunks = append(chunks, Chunk{
                Title:   title,
                Content: body,
                Type:    "section",
            })
        } else {
            // Step 2: Split large sections by ### headers
            h3Sections := splitByRegex(body, `(?m)^### `)

            if len(h3Sections) > 1 {
                for _, subsection := range h3Sections {
                    subTitle := extractFirstLine(subsection)
                    subBody := removeFirstLine(subsection)
                    chunks = append(chunks, Chunk{
                        Title:   title + " > " + subTitle,
                        Content: subBody,
                        Type:    "subsection",
                    })
                }
            } else {
                // Step 3: Fallback to paragraph split
                paragraphs := splitByParagraphs(body)
                chunks = append(chunks, mergeParagraphsIntoChunks(title, paragraphs, 800)...)
            }
        }
    }

    // Step 4: Handle content before first header (intro/preamble)
    preamble := extractPreamble(content)
    if len(preamble) > 0 {
        chunks = prepend(chunks, Chunk{
            Title:   "Introduction",
            Content: preamble,
            Type:    "preamble",
        })
    }

    // Step 5: Apply size guardrails
    chunks = enforceMinMaxSize(chunks, minTokens: 100, maxTokens: 800)

    return chunks
}

func splitByParagraphs(content string) []string {
    // Split on double newlines (blank lines)
    return strings.Split(content, "\n\n")
}

func mergeParagraphsIntoChunks(title string, paragraphs []string, maxTokens int) []Chunk {
    chunks := []Chunk{}
    current := ""

    for _, para := range paragraphs {
        if countTokens(current + para) > maxTokens && current != "" {
            chunks = append(chunks, Chunk{Title: title, Content: current})
            current = para
        } else {
            current += "\n\n" + para
        }
    }

    if current != "" {
        chunks = append(chunks, Chunk{Title: title, Content: current})
    }

    return chunks
}
```

---

## Implementation Plan

### Files to Modify

| File | Changes |
|------|---------|
| `store/rbac.go` | Add `RetrievalMode` and `ContentTokens` to TenantConfig |
| `store/db/sqlite/rbac.go` | Update SQL queries for new fields |
| `store/migration/sqlite/0.25/` | New migration for columns |
| `server/router/api/v1/agent/chunker.go` | Replace with heading-based chunker |
| `server/router/api/v1/agent/service.go` | Add mode detection, update prompt building |
| `server/router/api/v1/agent/handlers.go` | Calculate tokens on upload, set mode |

### Phase 1: Add Retrieval Mode to Tenant Config

**store/rbac.go:**
```go
type TenantConfig struct {
    // ... existing fields ...
    RetrievalMode   string // "long_context" or "rag"
    ContentTokens   int32  // Estimated token count of KB + Policy
}
```

**Migration:**
```sql
ALTER TABLE tenant_config ADD COLUMN retrieval_mode TEXT DEFAULT 'long_context';
ALTER TABLE tenant_config ADD COLUMN content_tokens INTEGER DEFAULT 0;
```

### Phase 2: Token Counting on Upload

**handlers.go - HandleImportSingleFile:**
```go
// After importing KB/Policy, calculate tokens
kbTokens := estimateTokens(kbContent)
policyTokens := estimateTokens(policyContent)
totalTokens := kbTokens + policyTokens

// Determine mode
mode := "long_context"
if totalTokens >= 30000 {
    mode = "rag"
}

// Update tenant config
config.RetrievalMode = mode
config.ContentTokens = int32(totalTokens)
s.store.UpsertTenantConfig(ctx, config)

// If RAG mode, trigger indexing
if mode == "rag" {
    go s.indexContentForRAG(ctx, tenantID, audienceType)
}
```

**Token estimation:**
```go
func estimateTokens(content string) int {
    // Rough estimate: 1 token ≈ 4 characters for English
    // More accurate: use tiktoken or similar
    return len(content) / 4
}
```

### Phase 3: Replace Annotation-Based Chunker

**chunker.go - New ChunkByHeadings function:**

Replace `ChunkKBContent` and `ChunkPolicyContent` with a unified heading-based chunker that works on raw markdown content without requiring parsed structures.

```go
// ChunkMarkdownContent chunks raw markdown using heading-based splitting.
func (c *Chunker) ChunkMarkdownContent(
    content string,
    tenantID int32,
    audience string,
    fileType string, // "kb" or "policy"
    sourceVersion int32,
) []DocumentChunk {
    // Implementation as described in pseudocode above
}
```

### Phase 4: Update Service Layer

**service.go - processChat:**
```go
func (s *Service) processChat(...) {
    // Get retrieval mode from tenant config
    tenantConfig, _ := s.store.GetTenantConfig(ctx, tenantID)

    if tenantConfig.RetrievalMode == "long_context" {
        // Use full context mode
        response, err = s.generateResponse(ctx, config, session, classification, decision)
    } else {
        // Use RAG mode
        response, err = s.generateRAGResponse(ctx, config, session, classification, decision, userMessage)
    }
}
```

**service.go - buildSystemPrompt (long context mode):**

Already loads full KB/Policy content. No changes needed.

**service.go - buildRAGSystemPrompt (rag mode):**

Works with retrieved chunks. No changes needed.

---

## Token Threshold Rationale

**Why 30,000 tokens?**

| Model | Context Window | Safe Working Context |
|-------|----------------|---------------------|
| GPT-4o | 128K | ~100K usable |
| Claude 3.5 | 200K | ~150K usable |
| GPT-4o-mini | 128K | ~100K usable |

With 30K tokens for KB/Policy, we leave ~70K+ for:
- System instructions (~2K)
- Conversation history (~10K for long conversations)
- Response generation headroom (~5K)
- Safety buffer

**Configurable threshold:**
```go
const DefaultTokenThreshold = 30000

// Can be overridden via environment variable
func getTokenThreshold() int {
    if v := os.Getenv("KB_TOKEN_THRESHOLD"); v != "" {
        if n, err := strconv.Atoi(v); err == nil {
            return n
        }
    }
    return DefaultTokenThreshold
}
```

---

## Migration Path

### For Existing Tenants

1. On server startup (one-time migration):
   - Calculate token counts for all existing tenants
   - Set retrieval_mode based on threshold
   - Re-index tenants in RAG mode with new chunker

2. New uploads automatically trigger mode detection

### Backward Compatibility

- Annotation-based KB/Policy files continue to work
- Annotations are simply ignored by heading-based chunker
- No changes required to existing tenant content

---

## Verification

1. **Unit tests:**
   - Test heading-based chunker with various markdown formats
   - Test token counting accuracy
   - Test mode switching logic

2. **Integration tests:**
   - Upload small KB (< 30K) → verify long_context mode
   - Upload large KB (≥ 30K) → verify RAG mode and indexing
   - Chat with both modes → verify responses use correct context

3. **Manual testing:**
   - Upload KB without annotations → verify chunks created correctly
   - Compare response quality between modes
   - Verify no regression for existing annotated KBs

---

## Future Enhancements

1. **Semantic chunking:** Use LLM to identify topic boundaries for even better chunks
2. **Adaptive threshold:** Adjust threshold based on model being used
3. **Chunk quality scoring:** Rate chunks and re-chunk poor quality sections
4. **Incremental indexing:** Only re-index changed sections on update
