# Hybrid Search Implementation - Week 1: Infrastructure

**Status:** ✅ Complete
**Date:** 2026-01-23
**Parent Document:** [DOCS_HYBRID_SEARCH.MD](./DOCS_HYBRID_SEARCH.MD)

---

## Overview

Week 1 focuses on building the infrastructure for hybrid search:

| Task | Description | Effort |
|------|-------------|--------|
| FTS Index Creation | Add Tantivy FTS index to LanceVectorDB | 2-3 hours |
| Hybrid Search Method | Implement hybrid search with score fusion | 4-6 hours |
| Unit Tests | BM25 scoring and hybrid search tests | 3-4 hours |

---

## Task 1: Add FTS Index Creation to LanceVectorDB

### Current State

**File:** `server/router/api/v1/agent/vectordb_lance.go`

Current `createIndexes()` function (lines 176-189):

```go
func (db *LanceVectorDB) createIndexes(ctx context.Context) error {
    // Create vector index for similarity search
    if err := db.table.CreateIndexWithName(ctx, []string{"embedding"}, contracts.IndexTypeIvfPq, "idx_embedding"); err != nil {
        slog.Warn("Failed to create vector index", "error", err)
    }

    // Create BTree index for tenant filtering
    if err := db.table.CreateIndexWithName(ctx, []string{"tenant_id"}, contracts.IndexTypeBTree, "idx_tenant"); err != nil {
        slog.Warn("Failed to create tenant index", "error", err)
    }

    return nil
}
```

### Implementation

**Step 1.1:** Add FTS index creation after existing indexes.

```go
func (db *LanceVectorDB) createIndexes(ctx context.Context) error {
    // Create vector index for similarity search (existing)
    if err := db.table.CreateIndexWithName(ctx, []string{"embedding"}, contracts.IndexTypeIvfPq, "idx_embedding"); err != nil {
        slog.Warn("Failed to create vector index", "error", err)
    }

    // Create BTree index for tenant filtering (existing)
    if err := db.table.CreateIndexWithName(ctx, []string{"tenant_id"}, contracts.IndexTypeBTree, "idx_tenant"); err != nil {
        slog.Warn("Failed to create tenant index", "error", err)
    }

    // NEW: Create FTS index for hybrid search (Tantivy BM25)
    if err := db.createFTSIndex(ctx); err != nil {
        slog.Warn("Failed to create FTS index", "error", err)
    }

    return nil
}

// createFTSIndex creates a full-text search index on title and content columns.
// Uses Tantivy under the hood with BM25 scoring (k1=1.2, b=0.75).
func (db *LanceVectorDB) createFTSIndex(ctx context.Context) error {
    // Create FTS index on title and content columns
    // LanceDB Go API: table.CreateFTSIndex(columns, options)
    err := db.table.CreateFTSIndex(ctx, []string{"title", "content"}, &contracts.FTSIndexOptions{
        Tokenizer: "default",  // Standard tokenizer with lowercase + stemming
        WithPosition: true,    // Enable phrase search
    })
    if err != nil {
        return fmt.Errorf("failed to create FTS index: %w", err)
    }

    slog.Info("Created FTS index on title and content columns")
    return nil
}
```

**Step 1.2:** Add method to check if FTS index exists (for migrations).

```go
// hasFTSIndex checks if the FTS index already exists.
func (db *LanceVectorDB) hasFTSIndex(ctx context.Context) bool {
    // Check table indexes for FTS type
    indexes, err := db.table.ListIndexes(ctx)
    if err != nil {
        return false
    }
    for _, idx := range indexes {
        if idx.Type == contracts.IndexTypeFTS {
            return true
        }
    }
    return false
}
```

**Step 1.3:** Update `ensureTable()` to create FTS index on existing tables.

```go
// In ensureTable(), after opening existing table:
if tableExists {
    table, err := db.conn.OpenTable(ctx, lanceTableName)
    // ...
    db.table = table

    // NEW: Ensure FTS index exists on existing tables
    if !db.hasFTSIndex(ctx) {
        slog.Info("Creating FTS index on existing table")
        if err := db.createFTSIndex(ctx); err != nil {
            slog.Warn("Failed to create FTS index on existing table", "error", err)
        }
    }
}
```

### Verification Checklist

- [ ] FTS index created on new tables without error
- [ ] FTS index created on existing tables (migration path)
- [ ] No performance degradation on Insert operations
- [ ] Index visible in `ListIndexes()` output

---

## Task 2: Implement Hybrid Search Method

### Current State

**File:** `server/router/api/v1/agent/vectordb_lance.go`

Current `Search()` method (lines 375-459) only does vector search:

```go
func (db *LanceVectorDB) Search(ctx context.Context, query SearchQuery) (*SearchResult, error) {
    // ... embedding generation ...
    // ... filter building ...

    // Execute vector search with filter
    results, err := db.table.VectorSearchWithFilter(ctx, "embedding", queryEmbedding, topK, filter)
    // ... result processing ...
}
```

### Implementation

**Step 2.1:** Extend SearchQuery struct in `vectordb.go`.

```go
// SearchQuery represents a search request.
type SearchQuery struct {
    // Existing fields
    QueryText       string    `json:"query_text"`
    QueryEmbedding  []float32 `json:"query_embedding,omitempty"`
    TenantID        int32     `json:"tenant_id"`
    AudienceType    string    `json:"audience_type"`
    ContentTypes    []string  `json:"content_types,omitempty"`
    ActiveOnly      bool      `json:"active_only"`
    TopK            int       `json:"top_k"`
    MinScore        float64   `json:"min_score"`

    // NEW: Hybrid search parameters
    UseHybridSearch bool    `json:"use_hybrid_search"` // Enable hybrid mode
    VectorWeight    float64 `json:"vector_weight"`     // Weight for vector score (0-1, default: 0.7)
    TextWeight      float64 `json:"text_weight"`       // Weight for BM25 score (0-1, default: 0.3)
}
```

**Step 2.2:** Extend SearchResult struct in `vectordb.go`.

```go
// SearchResult represents search results.
type SearchResult struct {
    // Existing fields
    Chunks  []DocumentChunk `json:"chunks"`
    Scores  []float64       `json:"scores"`   // Combined hybrid scores (or vector-only)
    Total   int             `json:"total"`
    Latency time.Duration   `json:"latency"`

    // NEW: Debug/analysis fields
    SearchMode   string    `json:"search_mode"`             // "vector", "hybrid", or "fts"
    VectorScores []float64 `json:"vector_scores,omitempty"` // Raw vector scores
    BM25Scores   []float64 `json:"bm25_scores,omitempty"`   // Raw BM25 scores
}
```

**Step 2.3:** Implement hybrid search in `vectordb_lance.go`.

```go
// Search performs vector or hybrid search based on query parameters.
func (db *LanceVectorDB) Search(ctx context.Context, query SearchQuery) (*SearchResult, error) {
    start := time.Now()

    db.mu.RLock()
    defer db.mu.RUnlock()

    // Get or generate query embedding
    queryEmbedding, err := db.getQueryEmbedding(ctx, query)
    if err != nil {
        return nil, err
    }

    // Build filter string
    filter := db.buildFilter(query)

    // Determine search mode
    if query.UseHybridSearch && query.QueryText != "" {
        return db.hybridSearch(ctx, query, queryEmbedding, filter, start)
    }
    return db.vectorSearch(ctx, query, queryEmbedding, filter, start)
}

// hybridSearch performs combined vector + BM25 search using LanceDB's built-in hybrid search.
func (db *LanceVectorDB) hybridSearch(ctx context.Context, query SearchQuery, embedding []float32, filter string, start time.Time) (*SearchResult, error) {
    // Set default weights if not specified
    vectorWeight := query.VectorWeight
    textWeight := query.TextWeight
    if vectorWeight == 0 && textWeight == 0 {
        vectorWeight = 0.7
        textWeight = 0.3
    }

    topK := query.TopK
    if topK <= 0 {
        topK = 10
    }

    // Execute hybrid search using LanceDB API
    // This combines vector search with FTS (BM25) using LinearCombinationReranker
    results, err := db.table.HybridSearch(ctx, &contracts.HybridSearchRequest{
        VectorColumn:  "embedding",
        VectorQuery:   embedding,
        TextColumns:   []string{"title", "content"},
        TextQuery:     query.QueryText,
        Limit:         topK,
        Filter:        filter,
        Reranker: &contracts.LinearCombinationReranker{
            VectorWeight: float32(vectorWeight),
            TextWeight:   float32(textWeight),
        },
    })
    if err != nil {
        // Fallback to vector-only search if hybrid fails
        slog.Warn("Hybrid search failed, falling back to vector search", "error", err)
        return db.vectorSearch(ctx, query, embedding, filter, start)
    }

    // Parse results
    chunks := make([]DocumentChunk, 0, len(results))
    scores := make([]float64, 0, len(results))
    vectorScores := make([]float64, 0, len(results))
    bm25Scores := make([]float64, 0, len(results))

    for _, row := range results {
        chunk := db.rowToDocumentChunk(row)

        // Extract hybrid score
        var score float64 = 1.0
        if s, ok := row["_score"].(float64); ok {
            score = s
        } else if s, ok := row["_score"].(float32); ok {
            score = float64(s)
        }

        // Extract component scores if available
        var vectorScore, bm25Score float64
        if vs, ok := row["_vector_score"].(float64); ok {
            vectorScore = vs
        }
        if bs, ok := row["_text_score"].(float64); ok {
            bm25Score = bs
        }

        if score >= query.MinScore {
            chunks = append(chunks, chunk)
            scores = append(scores, score)
            vectorScores = append(vectorScores, vectorScore)
            bm25Scores = append(bm25Scores, bm25Score)
        }
    }

    return &SearchResult{
        Chunks:       chunks,
        Scores:       scores,
        VectorScores: vectorScores,
        BM25Scores:   bm25Scores,
        Total:        len(chunks),
        Latency:      time.Since(start),
        SearchMode:   "hybrid",
    }, nil
}

// vectorSearch performs vector-only search (existing logic refactored).
func (db *LanceVectorDB) vectorSearch(ctx context.Context, query SearchQuery, embedding []float32, filter string, start time.Time) (*SearchResult, error) {
    topK := query.TopK
    if topK <= 0 {
        topK = 10
    }

    results, err := db.table.VectorSearchWithFilter(ctx, "embedding", embedding, topK, filter)
    if err != nil {
        return nil, fmt.Errorf("vector search failed: %w", err)
    }

    chunks := make([]DocumentChunk, 0, len(results))
    scores := make([]float64, 0, len(results))

    for _, row := range results {
        chunk := db.rowToDocumentChunk(row)

        var score float64 = 1.0
        if dist, ok := row["_distance"].(float64); ok {
            score = 1.0 / (1.0 + dist)
        } else if dist, ok := row["_distance"].(float32); ok {
            score = 1.0 / (1.0 + float64(dist))
        }

        if score >= query.MinScore {
            chunks = append(chunks, chunk)
            scores = append(scores, score)
        }
    }

    return &SearchResult{
        Chunks:     chunks,
        Scores:     scores,
        Total:      len(chunks),
        Latency:    time.Since(start),
        SearchMode: "vector",
    }, nil
}

// Helper methods

func (db *LanceVectorDB) getQueryEmbedding(ctx context.Context, query SearchQuery) ([]float32, error) {
    if len(query.QueryEmbedding) > 0 {
        return query.QueryEmbedding, nil
    }
    if query.QueryText != "" {
        embeddings, err := db.embedSvc.Embed(ctx, []string{query.QueryText})
        if err != nil {
            return nil, fmt.Errorf("failed to embed query: %w", err)
        }
        return embeddings[0], nil
    }
    return nil, fmt.Errorf("query must have either QueryText or QueryEmbedding")
}

func (db *LanceVectorDB) buildFilter(query SearchQuery) string {
    var filterParts []string
    filterParts = append(filterParts, fmt.Sprintf("tenant_id = %d", query.TenantID))

    if query.AudienceType != "" {
        filterParts = append(filterParts, fmt.Sprintf("audience_type = '%s'", query.AudienceType))
    }
    if query.ActiveOnly {
        filterParts = append(filterParts, "is_active = true")
    }
    if len(query.ContentTypes) > 0 {
        types := make([]string, len(query.ContentTypes))
        for i, ct := range query.ContentTypes {
            types[i] = fmt.Sprintf("'%s'", ct)
        }
        filterParts = append(filterParts, fmt.Sprintf("content_type IN (%s)", strings.Join(types, ", ")))
    }

    return strings.Join(filterParts, " AND ")
}
```

### Verification Checklist

- [ ] Hybrid search returns results when `UseHybridSearch=true`
- [ ] Vector-only search still works when `UseHybridSearch=false`
- [ ] Fallback to vector search works when FTS fails
- [ ] Component scores (VectorScores, BM25Scores) populated
- [ ] SearchMode field correctly set to "hybrid" or "vector"
- [ ] MinScore filtering works with hybrid scores

---

## Task 3: Unit Tests for BM25 Scoring

### Test File

**Create:** `server/router/api/v1/agent/vectordb_test.go`

```go
//go:build !rag

package agent

import (
    "context"
    "math"
    "testing"
    "time"
)

// =============================================================================
// BM25 Scoring Tests
// =============================================================================

func TestBM25Score_BasicScoring(t *testing.T) {
    tests := []struct {
        name     string
        query    string
        title    string
        content  string
        wantMin  float64  // Minimum expected score
        wantMax  float64  // Maximum expected score
    }{
        {
            name:    "exact match in title",
            query:   "water damage",
            title:   "Water Damage Restoration",
            content: "We provide professional restoration services.",
            wantMin: 0.5,
            wantMax: 1.0,
        },
        {
            name:    "exact match in content",
            query:   "water damage",
            title:   "Our Services",
            content: "We specialize in water damage repair and restoration.",
            wantMin: 0.3,
            wantMax: 1.0,
        },
        {
            name:    "no match",
            query:   "plumbing repair",
            title:   "Water Damage Restoration",
            content: "We provide professional restoration services.",
            wantMin: 0.0,
            wantMax: 0.1,
        },
        {
            name:    "partial match",
            query:   "water restoration",
            title:   "Water Damage Restoration",
            content: "Professional water damage services.",
            wantMin: 0.4,
            wantMax: 1.0,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create a simple BM25 scorer for testing
            scorer := NewBM25Scorer()
            scorer.AddDocument("doc1", tt.title+" "+tt.content)

            score := scorer.Score(tt.query, "doc1")

            if score < tt.wantMin || score > tt.wantMax {
                t.Errorf("BM25Score() = %v, want between %v and %v", score, tt.wantMin, tt.wantMax)
            }
        })
    }
}

func TestBM25Score_TermFrequencySaturation(t *testing.T) {
    // BM25 should show diminishing returns for repeated terms
    scorer := NewBM25Scorer()

    // Document with term appearing once
    scorer.AddDocument("doc1", "water damage restoration")

    // Document with term appearing multiple times
    scorer.AddDocument("doc2", "water water water damage damage restoration")

    score1 := scorer.Score("water", "doc1")
    score2 := scorer.Score("water", "doc2")

    // Score2 should be higher but not 3x higher (saturation effect)
    if score2 < score1 {
        t.Errorf("Repeated term should have higher score: got %v < %v", score2, score1)
    }
    if score2 > score1*2.0 {
        t.Errorf("BM25 saturation not working: score2 (%v) should not be more than 2x score1 (%v)", score2, score1)
    }
}

func TestBM25Score_DocumentLengthNormalization(t *testing.T) {
    scorer := NewBM25Scorer()

    // Short document with exact match
    scorer.AddDocument("short", "water damage")

    // Long document with same terms buried in text
    scorer.AddDocument("long", "our company provides many services including water damage restoration and many other home improvement services for residential and commercial customers")

    scoreShort := scorer.Score("water damage", "short")
    scoreLong := scorer.Score("water damage", "long")

    // Short document should score higher for exact match due to length normalization
    if scoreShort <= scoreLong {
        t.Errorf("Short document should score higher: short=%v, long=%v", scoreShort, scoreLong)
    }
}

// =============================================================================
// Hybrid Score Combination Tests
// =============================================================================

func TestLinearCombination_Scoring(t *testing.T) {
    tests := []struct {
        name         string
        vectorScore  float64
        bm25Score    float64
        vectorWeight float64
        textWeight   float64
        want         float64
    }{
        {
            name:         "default weights",
            vectorScore:  0.8,
            bm25Score:    0.6,
            vectorWeight: 0.7,
            textWeight:   0.3,
            want:         0.74, // 0.7*0.8 + 0.3*0.6
        },
        {
            name:         "equal weights",
            vectorScore:  0.8,
            bm25Score:    0.6,
            vectorWeight: 0.5,
            textWeight:   0.5,
            want:         0.7, // 0.5*0.8 + 0.5*0.6
        },
        {
            name:         "vector only",
            vectorScore:  0.8,
            bm25Score:    0.6,
            vectorWeight: 1.0,
            textWeight:   0.0,
            want:         0.8,
        },
        {
            name:         "bm25 only",
            vectorScore:  0.8,
            bm25Score:    0.6,
            vectorWeight: 0.0,
            textWeight:   1.0,
            want:         0.6,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := LinearCombination(tt.vectorScore, tt.bm25Score, tt.vectorWeight, tt.textWeight)
            if math.Abs(got-tt.want) > 0.001 {
                t.Errorf("LinearCombination() = %v, want %v", got, tt.want)
            }
        })
    }
}

func TestLinearCombination_EdgeCases(t *testing.T) {
    // Zero scores
    got := LinearCombination(0.0, 0.0, 0.7, 0.3)
    if got != 0.0 {
        t.Errorf("Zero scores should return 0, got %v", got)
    }

    // Perfect scores
    got = LinearCombination(1.0, 1.0, 0.7, 0.3)
    if got != 1.0 {
        t.Errorf("Perfect scores should return 1.0, got %v", got)
    }
}

// =============================================================================
// MemoryVectorDB Hybrid Search Tests
// =============================================================================

func TestMemoryVectorDB_HybridSearch(t *testing.T) {
    ctx := context.Background()

    // Create mock embedding service
    mockEmbed := &MockEmbeddingService{
        dimension: 384,
    }

    // Create memory vector DB
    db := NewMemoryVectorDB(mockEmbed)

    // Insert test documents
    chunks := []DocumentChunk{
        {
            ID:           "1",
            TenantID:     1,
            AudienceType: "external",
            ContentType:  "service",
            Title:        "Water Damage Restoration",
            Content:      "Professional water damage cleanup and restoration services.",
            IsActive:     true,
        },
        {
            ID:           "2",
            TenantID:     1,
            AudienceType: "external",
            ContentType:  "service",
            Title:        "Mold Remediation",
            Content:      "Expert mold removal and prevention services.",
            IsActive:     true,
        },
        {
            ID:           "3",
            TenantID:     1,
            AudienceType: "external",
            ContentType:  "faq",
            Title:        "How long does water damage restoration take?",
            Content:      "Water damage restoration typically takes 3-5 days depending on severity.",
            IsActive:     true,
        },
    }

    err := db.Insert(ctx, chunks)
    if err != nil {
        t.Fatalf("Insert failed: %v", err)
    }

    // Test hybrid search
    result, err := db.Search(ctx, SearchQuery{
        QueryText:       "water damage",
        TenantID:        1,
        AudienceType:    "external",
        TopK:            10,
        MinScore:        0.0,
        UseHybridSearch: true,
        VectorWeight:    0.7,
        TextWeight:      0.3,
    })

    if err != nil {
        t.Fatalf("Search failed: %v", err)
    }

    // Should find documents with "water damage"
    if result.Total == 0 {
        t.Error("Expected at least one result")
    }

    // Verify search mode
    if result.SearchMode != "hybrid" {
        t.Errorf("Expected SearchMode='hybrid', got '%s'", result.SearchMode)
    }

    // Document 1 and 3 should score higher (exact "water damage" match)
    foundWaterDamage := false
    for _, chunk := range result.Chunks {
        if chunk.Title == "Water Damage Restoration" || chunk.Title == "How long does water damage restoration take?" {
            foundWaterDamage = true
            break
        }
    }
    if !foundWaterDamage {
        t.Error("Expected to find 'water damage' related documents in top results")
    }
}

func TestMemoryVectorDB_VectorOnlySearch(t *testing.T) {
    ctx := context.Background()

    mockEmbed := &MockEmbeddingService{dimension: 384}
    db := NewMemoryVectorDB(mockEmbed)

    chunks := []DocumentChunk{
        {
            ID:           "1",
            TenantID:     1,
            AudienceType: "external",
            ContentType:  "service",
            Title:        "Test Service",
            Content:      "Test content",
            IsActive:     true,
        },
    }

    db.Insert(ctx, chunks)

    // Test vector-only search (UseHybridSearch = false)
    result, err := db.Search(ctx, SearchQuery{
        QueryText:       "test",
        TenantID:        1,
        AudienceType:    "external",
        TopK:            10,
        UseHybridSearch: false,
    })

    if err != nil {
        t.Fatalf("Search failed: %v", err)
    }

    if result.SearchMode != "vector" {
        t.Errorf("Expected SearchMode='vector', got '%s'", result.SearchMode)
    }
}

// =============================================================================
// Helper Types for Testing
// =============================================================================

// MockEmbeddingService for testing
type MockEmbeddingService struct {
    dimension int
}

func (m *MockEmbeddingService) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    embeddings := make([][]float32, len(texts))
    for i := range texts {
        embeddings[i] = make([]float32, m.dimension)
        // Generate deterministic pseudo-random embedding based on text hash
        for j := range embeddings[i] {
            embeddings[i][j] = float32(len(texts[i])+j) / 1000.0
        }
    }
    return embeddings, nil
}

func (m *MockEmbeddingService) Dimension() int {
    return m.dimension
}

// =============================================================================
// BM25 Scorer Implementation (for testing MemoryVectorDB)
// =============================================================================

// BM25Scorer implements BM25 scoring for in-memory search.
type BM25Scorer struct {
    k1       float64            // Term frequency saturation (default: 1.2)
    b        float64            // Length normalization (default: 0.75)
    docs     map[string][]string // docID -> tokens
    docFreq  map[string]int      // term -> document frequency
    avgLen   float64
    totalDocs int
}

// NewBM25Scorer creates a new BM25 scorer with default parameters.
func NewBM25Scorer() *BM25Scorer {
    return &BM25Scorer{
        k1:      1.2,
        b:       0.75,
        docs:    make(map[string][]string),
        docFreq: make(map[string]int),
    }
}

// AddDocument adds a document to the index.
func (s *BM25Scorer) AddDocument(id, text string) {
    tokens := tokenize(text)
    s.docs[id] = tokens

    // Update document frequency
    seen := make(map[string]bool)
    for _, token := range tokens {
        if !seen[token] {
            s.docFreq[token]++
            seen[token] = true
        }
    }

    s.totalDocs++

    // Recalculate average length
    totalLen := 0
    for _, doc := range s.docs {
        totalLen += len(doc)
    }
    s.avgLen = float64(totalLen) / float64(s.totalDocs)
}

// Score calculates the BM25 score for a query against a document.
func (s *BM25Scorer) Score(query, docID string) float64 {
    docTokens, ok := s.docs[docID]
    if !ok {
        return 0.0
    }

    queryTokens := tokenize(query)
    docLen := float64(len(docTokens))

    // Count term frequencies in document
    tf := make(map[string]int)
    for _, token := range docTokens {
        tf[token]++
    }

    var score float64
    for _, term := range queryTokens {
        termFreq := float64(tf[term])
        if termFreq == 0 {
            continue
        }

        // IDF calculation
        df := float64(s.docFreq[term])
        idf := math.Log((float64(s.totalDocs)-df+0.5)/(df+0.5) + 1)

        // BM25 term score
        numerator := termFreq * (s.k1 + 1)
        denominator := termFreq + s.k1*(1-s.b+s.b*(docLen/s.avgLen))
        score += idf * (numerator / denominator)
    }

    // Normalize to 0-1 range (approximate)
    maxScore := float64(len(queryTokens)) * 3.0 // rough upper bound
    if maxScore > 0 {
        score = score / maxScore
    }
    if score > 1.0 {
        score = 1.0
    }

    return score
}

// tokenize splits text into lowercase tokens.
func tokenize(text string) []string {
    // Simple tokenization: lowercase, split on non-alpha
    text = strings.ToLower(text)
    words := strings.FieldsFunc(text, func(r rune) bool {
        return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
    })
    return words
}

// LinearCombination combines vector and BM25 scores.
func LinearCombination(vectorScore, bm25Score, vectorWeight, textWeight float64) float64 {
    return vectorWeight*vectorScore + textWeight*bm25Score
}
```

### Test Commands

```bash
# Run all tests
go test ./server/router/api/v1/agent/... -v

# Run specific test
go test ./server/router/api/v1/agent/... -v -run TestBM25Score

# Run with coverage
go test ./server/router/api/v1/agent/... -cover

# Run with race detector
go test ./server/router/api/v1/agent/... -race
```

### Verification Checklist

- [ ] All BM25 scoring tests pass
- [ ] Linear combination tests pass
- [ ] MemoryVectorDB hybrid search tests pass
- [ ] No race conditions detected
- [ ] Code coverage > 80% for new code

---

## Summary

### Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `vectordb.go` | Modify | Extend SearchQuery, SearchResult structs; add BM25Scorer |
| `vectordb_lance.go` | Modify | Add FTS index, implement hybridSearch() |
| `vectordb_test.go` | Create | Unit tests for BM25 and hybrid search |

### Dependencies

- No new external dependencies required
- Uses LanceDB's built-in FTS (Tantivy)
- BM25 scorer is pure Go for testing

### Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| LanceDB Go API differs from documentation | Check actual lancedb-go SDK for correct method signatures |
| FTS index creation slow on large tables | Add progress logging, consider async creation |
| Score normalization inconsistent | Test thoroughly with known query/document pairs |

---

## Next Steps

After Week 1 completion:
- **Week 2:** Integration - Wire hybrid search into RetrieveContextForQuery
- **Week 3:** Evaluation - Create test dataset, measure quality metrics
- **Week 4:** Rollout - Enable for test tenant, monitor and iterate

---

*Document Version: 1.0*
*Last Updated: 2026-01-23*
