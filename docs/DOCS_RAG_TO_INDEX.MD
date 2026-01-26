# Integration Analysis: Format for RAG → Rebuild Index

This document analyzes whether Rebuild Index should use the processing options configured in Format for RAG.

## Executive Summary

**Question:** Can Rebuild Index use the choices made when Format for RAG was clicked?

**Answer:** Yes, it makes sense, but requires architectural changes.

**Recommendation:** Implement in phases - start with storing options per tenant, then integrate with Rebuild Index.

---

## Current Architecture

### Two Separate Implementations

| Component | File | Algorithm | Output | Configurable |
|-----------|------|-----------|--------|--------------|
| Format for RAG | `processor.go` | ContentProcessor | `ProcessedChunk` | Yes (20+ options) |
| Rebuild Index | `chunker.go` | ChunkMarkdownContent | `DocumentChunk` | No (hardcoded) |

### Hardcoded Values in chunker.go

```go
const (
    MinChunkTokens     = 100   // Fixed
    MaxChunkTokens     = 800   // Fixed
    ChunkOverlapTokens = 50    // Fixed
)
```

### Configurable Values in processor.go

```go
type ProcessingOptions struct {
    // Chunking Controls
    MaxChunkSize     int  `json:"max_chunk_size"`      // 400-1200
    MinChunkSize     int  `json:"min_chunk_size"`      // 50-200
    ChunkOverlap     int  `json:"chunk_overlap"`       // 0-100
    MergeSmallChunks bool `json:"merge_small_chunks"`

    // Structure Splitting
    SplitH2         bool `json:"split_h2"`
    SplitH3         bool `json:"split_h3"`
    SplitParagraphs bool `json:"split_paragraphs"`
    SplitSentences  bool `json:"split_sentences"`
    PreserveLists   bool `json:"preserve_lists"`

    // Text Normalization
    RemoveWhitespace   bool `json:"remove_whitespace"`
    StripHTML          bool `json:"strip_html"`
    // ... and more
}
```

---

## Why Integration Makes Sense

### 1. Preview-Production Parity

Currently, what you see in Format for RAG is NOT what gets indexed:

```
Format for RAG:
  - User sets MaxChunkSize = 500
  - User sets SplitSentences = true
  - Preview shows 20 chunks

Rebuild Index:
  - Uses hardcoded MaxChunkSize = 800
  - Never splits by sentences
  - Actually creates 12 chunks
```

**Problem:** User cannot predict RAG quality from preview.

### 2. Per-Tenant Optimization

Different content types need different chunking:

| Tenant Type | Optimal Settings |
|-------------|------------------|
| FAQ-heavy | Small chunks (400), preserve Q&A |
| Technical docs | Large chunks (1000), split by headers |
| Legal content | Medium chunks (600), split by paragraphs |
| Support scripts | Small chunks (300), split by sentences |

**Problem:** One-size-fits-all hardcoded values.

### 3. Single Source of Truth

Maintaining two chunking implementations is error-prone:

```
processor.go: splitByH2Headers() - one implementation
chunker.go:   splitByH2Headers() - duplicate implementation
```

**Problem:** Bug fixes must be applied twice.

---

## Implementation Approaches

### Approach A: Store ProcessingOptions Per Tenant

**Schema Change:**
```sql
ALTER TABLE agent_tenant ADD COLUMN processing_options TEXT;
-- Stores JSON: {"max_chunk_size": 800, "split_h2": true, ...}
```

**Workflow:**
```
1. User configures options in Format for RAG
2. User clicks "Save as Default"
3. Options stored in agent_tenant.processing_options
4. Rebuild Index loads options from database
5. Applies options during chunking
```

**Pros:** Simple, backward compatible
**Cons:** Two chunking implementations remain

### Approach B: Unify Chunking Logic

**Refactor:**
1. Make `Chunker` accept `ProcessingOptions`
2. Deprecate `ContentProcessor` chunking
3. Both features use same code path

**Code Change:**
```go
func (c *Chunker) ChunkMarkdownContent(
    content string,
    tenantID int32,
    audience string,
    fileType string,
    sourceVersion int32,
    options *ProcessingOptions,  // NEW: optional, uses defaults if nil
) []DocumentChunk
```

**Pros:** Single source of truth, no drift
**Cons:** Larger refactor, risk of regression

### Approach C: Hybrid (Recommended)

**Phase 1:** Store options per tenant (Approach A)
**Phase 2:** Unify chunking logic (Approach B)
**Phase 3:** Add UI to configure defaults

---

## Data Structure Bridge

### Current Gap

```go
// processor.go output
type ProcessedChunk struct {
    ID      string  // "kb_section_0"
    Title   string
    Content string
    Tokens  int
}

// chunker.go output
type DocumentChunk struct {
    ID            string    // "1:external:kb_section:kb_section_0"
    TenantID      int32     // REQUIRED
    AudienceType  string    // REQUIRED
    ContentType   string
    Title         string
    Content       string
    Code          string
    IsEmergency   bool
    IsActive      bool
    Priority      int32
    SourceVersion int32
    Embedding     []float32 // REQUIRED for VectorDB
    IndexedAt     time.Time
}
```

### Bridge Function (needed for Approach A)

```go
func ConvertToDocumentChunks(
    processed []ProcessedChunk,
    tenantID int32,
    audience string,
    fileType string,
    sourceVersion int32,
) []DocumentChunk {
    var result []DocumentChunk
    for _, p := range processed {
        result = append(result, DocumentChunk{
            ID:            ChunkID(tenantID, audience, fileType+"_section", p.ID),
            TenantID:      tenantID,
            AudienceType:  audience,
            ContentType:   fileType + "_section",
            Title:         p.Title,
            Content:       p.Content,
            Code:          p.ID,
            IsActive:      true,
            SourceVersion: sourceVersion,
            IndexedAt:     time.Now(),
        })
    }
    return result
}
```

---

## Recommended Implementation Plan

### Phase 1: Store Options (Low Risk)

1. Add `processing_options` column to `agent_tenant` table
2. Add "Save as Default" button to Format for RAG UI
3. Load saved options when Format for RAG opens
4. No change to Rebuild Index yet

### Phase 2: Use Options in Rebuild Index (Medium Risk)

1. Modify `ReindexTenantContent` to load `ProcessingOptions`
2. Create bridge function to convert chunks
3. Apply options during chunking
4. Keep fallback to defaults if no options saved

### Phase 3: Unify Chunking (Higher Risk)

1. Refactor `Chunker` to accept `ProcessingOptions`
2. Deprecate `ContentProcessor.createChunks()`
3. Full test coverage
4. Migration testing

---

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Breaking existing indexes | Keep defaults identical to current hardcoded values |
| Inconsistent chunk IDs | Use deterministic ID generation |
| Migration complexity | Phase rollout, tenant-by-tenant |
| UI confusion | Clear documentation, tooltips |

---

## Decision Matrix

| Factor | Do Nothing | Approach A | Approach B | Approach C |
|--------|------------|------------|------------|------------|
| Preview-production parity | No | Yes | Yes | Yes |
| Per-tenant optimization | No | Yes | Yes | Yes |
| Code simplicity | Yes | Partial | Yes | Yes |
| Implementation effort | None | Low | Medium | Medium |
| Risk | None | Low | Medium | Low to Medium |
| Backward compatibility | Yes | Yes | Partial | Yes |

**Recommendation:** Approach C (Hybrid) - phased implementation with minimal risk.

---

## Conclusion

**Should Rebuild Index use Format for RAG options?**

**Yes**, for these reasons:
1. Users expect preview to match actual indexing
2. Different content types need different chunking strategies
3. Maintaining two implementations is technical debt

**Implementation path:**
1. Start with storing options per tenant (quick win)
2. Bridge the gap with conversion function
3. Eventually unify the chunking logic

This creates a coherent system where Format for RAG becomes the configuration interface for RAG indexing, not just a preview tool.

---

## Related Documentation

- [DOCS_RAG_CLARIFICATION.MD](./DOCS_RAG_CLARIFICATION.MD) - Current workflow separation
- [DOCS_PROCESSING.MD](./DOCS_PROCESSING.MD) - Processing techniques reference
- [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD) - Vector database implementation
