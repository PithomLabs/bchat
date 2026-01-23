# Hybrid Search Implementation Plan

**Status:** Proposed
**Date:** 2026-01-23
**Parent Document:** [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD)

---

## Table of Contents

1. [Overview](#overview)
2. [Current Architecture](#current-architecture)
3. [LanceDB Hybrid Search Capabilities](#lancedb-hybrid-search-capabilities)
4. [Proposed Architecture](#proposed-architecture)
5. [Implementation Plan](#implementation-plan)
6. [Interface Changes](#interface-changes)
7. [Score Fusion Strategies](#score-fusion-strategies)
8. [Testing Strategy](#testing-strategy)
9. [Performance Considerations](#performance-considerations)
10. [Rollout Plan](#rollout-plan)

---

## Overview

### What is Hybrid Search?

Hybrid search combines two complementary retrieval approaches:

| Approach | Strength | Weakness |
|----------|----------|----------|
| **Vector Search** | Semantic understanding, handles synonyms and paraphrases | May miss exact keyword matches |
| **BM25 (Keyword)** | Exact term matching, handles specific names/codes | No semantic understanding |

**Example:**
- Query: "water damage restoration"
- Vector finds: "flood cleanup services", "moisture remediation"
- BM25 finds: "water damage restoration pricing", "water extraction"
- Hybrid: Returns all relevant results, ranked by combined relevance

### Why Hybrid Search?

1. **Better Recall** - Catches both semantic matches and exact keyword matches
2. **Improved Precision** - Exact terms boost relevance of truly matching documents
3. **Handles Edge Cases** - Service codes, phone numbers, proper nouns
4. **Industry Standard** - Used by Elasticsearch, Pinecone, Weaviate, and now LanceDB

---

## Current Architecture

### Vector Search Flow

```
User Query
    │
    ▼
┌─────────────────────┐
│ Embed Query         │  (OpenRouter/local/mock)
│ 1536-dim vector     │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│ Vector Search       │  (LanceDB IVF-PQ index)
│ L2 distance         │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│ Filter & Rank       │  (tenant, audience, minScore)
└─────────┬───────────┘
          │
          ▼
     Top-K Results
```

### Current Files

| File | Purpose |
|------|---------|
| `vectordb.go` | VectorDB interface, MemoryVectorDB, SearchQuery/Result types |
| `vectordb_lance.go` | LanceVectorDB implementation |
| `chunker.go` | DocumentChunk structure with Title/Content fields |
| `service.go` | RetrieveContextForQuery orchestration |

### Current Limitations

- **Vector-only retrieval** - No keyword matching
- **No FTS index** - Title/Content stored but not indexed for search
- **Single scoring** - Only semantic similarity score returned

---

## LanceDB Hybrid Search Capabilities

LanceDB has **built-in full-text search** powered by Tantivy (Rust-based search engine).

### Key Features

| Feature | API | Description |
|---------|-----|-------------|
| FTS Index | `create_fts_index()` | Creates BM25 index on text columns |
| Hybrid Search | `hybrid_search()` | Combines vector + FTS in single query |
| Reranking | `LinearCombinationReranker` | Adjustable weight between vector and text scores |
| SQL FTS | `fts()` function | SQL syntax for full-text queries with `_score` |

### LanceDB Go API

```go
// Create FTS index on Title and Content columns
table.CreateFTSIndex([]string{"title", "content"})

// Hybrid search with reranking
results := table.HybridSearch(
    queryVector,           // Vector embedding
    queryText,             // Text for BM25
    limit,
    &HybridSearchOptions{
        VectorWeight: 0.7, // 70% vector, 30% BM25
        TextWeight:   0.3,
    },
)
```

### Tantivy BM25 Parameters

LanceDB uses Tantivy's default BM25 parameters:
- `k1 = 1.2` (term frequency saturation)
- `b = 0.75` (document length normalization)

---

## Proposed Architecture

### Hybrid Search Flow

```
User Query
    │
    ├──────────────────────────────┐
    │                              │
    ▼                              ▼
┌─────────────────┐      ┌─────────────────┐
│ Embed Query     │      │ Tokenize Query  │
│ 1536-dim vector │      │ BM25 terms      │
└────────┬────────┘      └────────┬────────┘
         │                        │
         └──────────┬─────────────┘
                    │
                    ▼
         ┌─────────────────────┐
         │ LanceDB HybridSearch│
         │ Vector + FTS        │
         └──────────┬──────────┘
                    │
                    ▼
         ┌─────────────────────┐
         │ LinearCombination   │
         │ Reranker            │
         └──────────┬──────────┘
                    │
                    ▼
         ┌─────────────────────┐
         │ Filter & Threshold  │
         │ (tenant, minScore)  │
         └──────────┬──────────┘
                    │
                    ▼
              Top-K Results
              (with hybrid scores)
```

### Architecture Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| FTS Engine | LanceDB built-in (Tantivy) | No external dependencies, single index |
| Index Columns | Title + Content | Both contain searchable text |
| Default Weight | 0.7 vector / 0.3 BM25 | Vector captures intent, BM25 boosts exact matches |
| Fallback | Vector-only if FTS fails | Graceful degradation |

---

## Implementation Plan

### Phase 1: FTS Index Creation

**Goal:** Add FTS index to existing LanceDB tables.

**Changes:**

1. **vectordb_lance.go** - Add FTS index creation after table creation:

```go
// After creating IVF-PQ index on embedding
// Create FTS index on text fields
err = table.CreateFTSIndex([]string{"title", "content"}, &FTSIndexOptions{
    Tokenizer: "default", // or "raw" for exact matching
})
```

2. **Schema verification** - Ensure Title/Content are properly populated during chunking.

**Validation:**
- [ ] FTS index created successfully
- [ ] No errors on insert operations
- [ ] Index size reasonable

---

### Phase 2: Hybrid Search Method

**Goal:** Implement hybrid search in LanceVectorDB.

**Changes:**

1. **vectordb.go** - Extend SearchQuery:

```go
type SearchQuery struct {
    // Existing fields...
    QueryText       string
    QueryEmbedding  []float32

    // New fields for hybrid search
    UseHybridSearch bool     // Enable hybrid mode
    VectorWeight    float64  // Weight for vector score (0-1)
    TextWeight      float64  // Weight for BM25 score (0-1)
}
```

2. **vectordb.go** - Extend SearchResult:

```go
type SearchResult struct {
    Chunks  []DocumentChunk
    Scores  []float64  // Combined hybrid scores

    // New fields for debugging/analysis
    VectorScores []float64  // Raw vector similarity scores
    BM25Scores   []float64  // Raw BM25 scores
    Total        int
    Latency      time.Duration
}
```

3. **vectordb_lance.go** - Implement hybrid search:

```go
func (db *LanceVectorDB) Search(ctx context.Context, query SearchQuery) (*SearchResult, error) {
    // Get embedding if not provided
    embedding := query.QueryEmbedding
    if len(embedding) == 0 && query.QueryText != "" {
        var err error
        embedding, err = db.embedSvc.Embed(ctx, query.QueryText)
        if err != nil {
            return nil, fmt.Errorf("failed to embed query: %w", err)
        }
    }

    // Determine search mode
    if query.UseHybridSearch {
        return db.hybridSearch(ctx, query, embedding)
    }
    return db.vectorSearch(ctx, query, embedding)
}

func (db *LanceVectorDB) hybridSearch(ctx context.Context, query SearchQuery, embedding []float32) (*SearchResult, error) {
    // Build filter
    filter := buildFilter(query)

    // Set default weights
    vectorWeight := query.VectorWeight
    textWeight := query.TextWeight
    if vectorWeight == 0 && textWeight == 0 {
        vectorWeight = 0.7
        textWeight = 0.3
    }

    // Execute hybrid search
    results, err := db.table.HybridSearch(
        embedding,
        query.QueryText,
        query.TopK,
        &lancedb.HybridSearchOptions{
            Filter:       filter,
            VectorWeight: vectorWeight,
            TextWeight:   textWeight,
        },
    )
    if err != nil {
        // Fallback to vector-only search
        slog.Warn("hybrid search failed, falling back to vector search", "error", err)
        return db.vectorSearch(ctx, query, embedding)
    }

    return convertResults(results), nil
}
```

**Validation:**
- [ ] Hybrid search returns results
- [ ] Scores combine vector + BM25 correctly
- [ ] Fallback works when FTS fails

---

### Phase 3: MemoryVectorDB Hybrid Support

**Goal:** Add BM25 to in-memory implementation for testing.

**Changes:**

1. **vectordb.go** - Add simple BM25 implementation:

```go
// BM25Index for in-memory keyword search
type BM25Index struct {
    documents    map[string][]string  // chunkID -> tokens
    docFreq      map[string]int       // term -> document frequency
    avgDocLength float64
    totalDocs    int
}

func (idx *BM25Index) Score(query string, chunkID string) float64 {
    // BM25 scoring with k1=1.2, b=0.75
}
```

2. **vectordb.go** - Update MemoryVectorDB.Search():

```go
func (m *MemoryVectorDB) Search(ctx context.Context, query SearchQuery) (*SearchResult, error) {
    // ... existing vector search logic ...

    if query.UseHybridSearch {
        // Calculate BM25 scores
        for i, chunk := range candidates {
            bm25Score := m.bm25Index.Score(query.QueryText, chunk.ID)
            vectorScore := scores[i]

            // Combine scores
            hybridScore := query.VectorWeight*vectorScore + query.TextWeight*bm25Score
            scores[i] = hybridScore
        }
    }

    // Re-sort by hybrid score
    // ...
}
```

**Validation:**
- [ ] MemoryVectorDB hybrid search matches LanceDB behavior
- [ ] Unit tests pass for BM25 scoring
- [ ] Consistent results across implementations

---

### Phase 4: Integration & Configuration

**Goal:** Wire hybrid search into chat flow with configuration.

**Changes:**

1. **vectordb.go** - Update RetrieveContextForQuery():

```go
func RetrieveContextForQuery(/* ... */) (*RetrievedContext, error) {
    // ... existing intent detection ...

    searchQuery := SearchQuery{
        QueryText:       query,
        TenantID:        tenantID,
        AudienceType:    audienceType,
        ContentTypes:    contentTypes,
        TopK:            topK,
        MinScore:        0.3,
        ActiveOnly:      true,

        // Enable hybrid search
        UseHybridSearch: true,
        VectorWeight:    0.7,
        TextWeight:      0.3,
    }

    result, err := vectorDB.Search(ctx, searchQuery)
    // ...
}
```

2. **Environment variables** - Add configuration:

```bash
# Hybrid search settings
HYBRID_SEARCH_ENABLED=true      # Enable hybrid search
HYBRID_VECTOR_WEIGHT=0.7        # Vector score weight
HYBRID_TEXT_WEIGHT=0.3          # BM25 score weight
```

**Validation:**
- [ ] Chat responses use hybrid search
- [ ] Configuration changes take effect
- [ ] Logs show hybrid search metrics

---

## Interface Changes

### SearchQuery (Extended)

```go
type SearchQuery struct {
    // Existing
    QueryText       string    // Text to search
    QueryEmbedding  []float32 // Pre-computed embedding
    TenantID        int32
    AudienceType    string
    ContentTypes    []string
    ActiveOnly      bool
    TopK            int
    MinScore        float64

    // New for hybrid search
    UseHybridSearch bool    // Enable hybrid mode (default: false for backward compat)
    VectorWeight    float64 // Weight for vector score (default: 0.7)
    TextWeight      float64 // Weight for BM25 score (default: 0.3)
}
```

### SearchResult (Extended)

```go
type SearchResult struct {
    // Existing
    Chunks  []DocumentChunk
    Scores  []float64       // Combined scores (or vector-only if hybrid disabled)
    Total   int
    Latency time.Duration

    // New for debugging/analysis
    SearchMode   string      // "vector", "hybrid", or "bm25"
    VectorScores []float64   // Raw vector similarity (optional, for analysis)
    BM25Scores   []float64   // Raw BM25 scores (optional, for analysis)
}
```

---

## Score Fusion Strategies

### 1. Linear Combination (Recommended)

```
hybrid_score = α × vector_score + (1-α) × bm25_score
```

Where `α` is the vector weight (default: 0.7).

**Pros:** Simple, interpretable, fast
**Cons:** Requires normalized scores

### 2. Reciprocal Rank Fusion (RRF)

```
RRF_score = Σ (1 / (k + rank_i))
```

Where `k` is a constant (typically 60) and `rank_i` is the rank in each result list.

**Pros:** Works with unnormalized scores, handles different scale
**Cons:** Ignores actual score values

### 3. Intent-Aware Weighting

Adjust weights based on detected intent:

| Intent | Vector Weight | BM25 Weight | Rationale |
|--------|---------------|-------------|-----------|
| `service_inquiry` | 0.8 | 0.2 | Semantic understanding important |
| `pricing_question` | 0.5 | 0.5 | Both keywords and context matter |
| `coverage_question` | 0.6 | 0.4 | Location names need exact match |
| `emergency` | 0.7 | 0.3 | Quick semantic match |
| Default | 0.7 | 0.3 | Balanced approach |

**Implementation:**

```go
func getHybridWeights(intent string) (vectorWeight, textWeight float64) {
    switch intent {
    case "service_inquiry", "booking_request":
        return 0.8, 0.2
    case "pricing_question":
        return 0.5, 0.5
    case "coverage_question":
        return 0.6, 0.4
    default:
        return 0.7, 0.3
    }
}
```

---

## Testing Strategy

### Unit Tests

**File:** `vectordb_test.go`

```go
func TestBM25Scoring(t *testing.T) {
    // Test basic BM25 calculation
    // Test term frequency saturation
    // Test document length normalization
}

func TestHybridScoreCombination(t *testing.T) {
    // Test linear combination
    // Test weight normalization
    // Test edge cases (zero scores)
}

func TestHybridSearch_MemoryVectorDB(t *testing.T) {
    // Test hybrid search returns results
    // Test vector-only mode still works
    // Test BM25-only mode
}
```

### Integration Tests

**File:** `vectordb_integration_test.go`

```go
func TestHybridSearch_LanceDB(t *testing.T) {
    // Test FTS index creation
    // Test hybrid search query execution
    // Test fallback to vector search
    // Test filter combination with hybrid
}

func TestHybridSearch_EndToEnd(t *testing.T) {
    // Test full RAG flow with hybrid search
    // Test response quality improvement
}
```

### Retrieval Quality Metrics

**Metrics to Track:**

| Metric | Description | Target |
|--------|-------------|--------|
| **Recall@5** | % relevant docs in top 5 | > 80% |
| **MRR** | Mean Reciprocal Rank | > 0.7 |
| **Latency** | Search execution time | < 100ms |
| **Exact Match Rate** | Queries with exact keyword match found | > 90% |

**Test Dataset:**

Create evaluation dataset with:
1. Queries with known relevant chunks
2. Mix of semantic queries ("help with water damage")
3. Mix of keyword queries ("service code WD-001")
4. Edge cases (typos, abbreviations)

**Evaluation Script:**

```go
func EvaluateHybridSearch(testCases []TestCase) *EvaluationResult {
    var results EvaluationResult

    for _, tc := range testCases {
        // Search with vector-only
        vectorResults := search(tc.Query, false)

        // Search with hybrid
        hybridResults := search(tc.Query, true)

        // Calculate metrics
        results.VectorRecall += calculateRecall(vectorResults, tc.RelevantIDs)
        results.HybridRecall += calculateRecall(hybridResults, tc.RelevantIDs)
        // ...
    }

    return &results
}
```

### A/B Testing Plan

1. **Control:** Vector-only search (current)
2. **Treatment:** Hybrid search (new)
3. **Metrics:** Response relevance, user satisfaction, escalation rate
4. **Duration:** 2 weeks per tenant
5. **Rollout:** 10% → 50% → 100%

---

## Performance Considerations

### Index Size

| Component | Estimated Size | Notes |
|-----------|----------------|-------|
| Vector Index (IVF-PQ) | ~4KB per chunk | Compressed vectors |
| FTS Index (Tantivy) | ~2KB per chunk | Inverted index |
| **Total per 1000 chunks** | ~6MB | Acceptable |

### Latency Budget

| Operation | Target | Current |
|-----------|--------|---------|
| Query embedding | 50-200ms | 50-200ms |
| Vector search | 10-50ms | 10-50ms |
| BM25 search | 5-20ms | N/A |
| Score fusion | < 1ms | N/A |
| **Total hybrid** | 70-270ms | N/A |

### Memory Usage

- FTS index loaded lazily by Tantivy
- BM25 scores computed on-demand
- No significant memory increase expected

### Scaling Considerations

| Scale | Recommendation |
|-------|----------------|
| < 10K chunks | In-memory works fine |
| 10K-100K chunks | LanceDB local storage |
| > 100K chunks | Consider LanceDB S3 + caching |

---

## Rollout Plan

### Week 1: Infrastructure

- [ ] Add FTS index creation to LanceVectorDB
- [ ] Implement hybrid search method
- [ ] Unit tests for BM25 scoring

### Week 2: Integration

- [ ] Implement MemoryVectorDB hybrid support
- [ ] Add environment configuration
- [ ] Integration tests

### Week 3: Evaluation

- [ ] Create evaluation dataset
- [ ] Run quality metrics comparison
- [ ] Performance benchmarking

### Week 4: Rollout

- [ ] Enable for test tenant
- [ ] Monitor metrics
- [ ] Gradual rollout to production

---

## References

- [LanceDB Full-Text Search](https://lancedb.github.io/lancedb/fts/)
- [Tantivy BM25 Implementation](https://github.com/quickwit-oss/tantivy)
- [Reciprocal Rank Fusion Paper](https://plg.uwaterloo.ca/~gvcormac/cormacksigir09-rrf.pdf)
- [Hybrid Search Best Practices](https://www.pinecone.io/learn/hybrid-search/)

---

*Document Version: 1.0*
*Last Updated: 2026-01-23*
