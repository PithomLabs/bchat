# Hybrid Search: A Complete Technical Guide

**Purpose:** Comprehensive explanation of how hybrid search works in the bchat AI agent system.
**Audience:** Developers, technical leads, and curious team members.
**Last Updated:** 2026-01-23

---

## Table of Contents

1. [Introduction: Why Hybrid Search?](#1-introduction-why-hybrid-search)
2. [The Two Search Methods](#2-the-two-search-methods)
3. [Score Fusion: Combining the Best of Both](#3-score-fusion-combining-the-best-of-both)
4. [The Complete Data Flow](#4-the-complete-data-flow)
5. [Configuration Guide](#5-configuration-guide)
6. [Testing and Verification](#6-testing-and-verification)
7. [Real-World Examples](#7-real-world-examples)
8. [Technical Reference](#8-technical-reference)
9. [Troubleshooting](#9-troubleshooting)

---

## 1. Introduction: Why Hybrid Search?

### The Library Analogy

Imagine you're in a vast library looking for information about "water damage restoration."

**Approach 1: The Expert Librarian (Vector Search)**
You ask a knowledgeable librarian who *understands* your topic. She doesn't just look for those exact words—she knows that "flood cleanup," "moisture remediation," and "water extraction" are all related concepts. She brings you relevant books even if they don't contain your exact phrase.

*Strength:* Understands meaning, synonyms, and related concepts.
*Weakness:* Might miss documents that use very specific technical terms you need.

**Approach 2: The Card Catalog (Keyword Search)**
You use the card catalog to find every book containing "water" AND "damage" AND "restoration." It's precise—if those words are there, you'll find it.

*Strength:* Exact matching, great for specific terms, product names, technical jargon.
*Weakness:* Misses "flood cleanup" because it doesn't contain your exact words.

**Hybrid Search: Best of Both Worlds**
What if you could use BOTH approaches and combine their results? That's hybrid search. It gives you:
- Semantic understanding (meaning-based results)
- Keyword precision (exact term matches)
- Better overall recall and precision

### Why This Matters for AI Chat Agents

When a customer asks your AI agent "Do you cover water damage from burst pipes?", hybrid search helps retrieve:
- Documents about water damage services (semantic match)
- Documents that specifically mention "burst pipes" (keyword match)
- Policy documents with exact coverage terms (keyword match)

The result: More relevant context for the AI to generate accurate answers.

---

## 2. The Two Search Methods

### 2.1 Vector Search (Semantic Understanding)

#### What Are Embeddings?

Text embeddings convert words and sentences into **numerical vectors** (lists of numbers). Think of it like giving every piece of text a unique "fingerprint" in mathematical space.

```
"Water damage restoration" → [0.12, -0.45, 0.78, ..., 0.33]  (384 numbers)
"Flood cleanup services"   → [0.11, -0.43, 0.76, ..., 0.31]  (384 numbers)
"Pizza delivery"           → [-0.67, 0.23, 0.01, ..., -0.89] (384 numbers)
```

Notice how similar concepts ("water damage" and "flood cleanup") have similar numbers, while unrelated concepts ("pizza delivery") have very different numbers.

#### How Similarity is Calculated

We use **cosine similarity** to measure how "aligned" two vectors are:

```
                    A · B (dot product)
Cosine Similarity = ─────────────────────
                    ||A|| × ||B|| (magnitudes)
```

**Intuition:** Imagine two arrows pointing in 3D space:
- If they point the same direction: similarity = 1.0 (identical)
- If they're perpendicular: similarity = 0.0 (unrelated)
- If they point opposite: similarity = -1.0 (opposite meaning)

#### Code Implementation

**File:** `server/router/api/v1/agent/vectordb.go` (lines 473-492)

```go
func cosineSimilarity(a, b []float32) float64 {
    var dotProduct, normA, normB float64
    for i := 0; i < len(a); i++ {
        dotProduct += float64(a[i]) * float64(b[i])
        normA += float64(a[i]) * float64(a[i])
        normB += float64(b[i]) * float64(b[i])
    }
    return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
```

**Real Example:**
```
Query: "water damage"
Document A: "Water damage restoration services" → Similarity: 0.89
Document B: "Fire damage repair"                → Similarity: 0.45
Document C: "Pizza delivery menu"               → Similarity: 0.02
```

---

### 2.2 BM25 Keyword Search (Exact Matching)

#### The Foundation: TF-IDF

Before understanding BM25, let's understand its foundation:

**Term Frequency (TF):** How often a word appears in a document.
- "water" appears 5 times in a document → Higher TF

**Inverse Document Frequency (IDF):** How rare a word is across all documents.
- "water" appears in 90% of documents → Low IDF (common word)
- "remediation" appears in 5% of documents → High IDF (rare, specific word)

**Key Insight:** Rare words are more important than common words.

#### BM25: The Improved Algorithm

BM25 improves on TF-IDF with two key refinements:

**1. Term Frequency Saturation (k1 parameter)**

Without saturation, a document with "water" 100 times would score 100x higher than one with "water" 1 time. But after a certain point, more repetitions don't really make the document more relevant.

BM25 uses k1=1.2 to create diminishing returns:
```
1 occurrence  → score contribution: 1.0
2 occurrences → score contribution: 1.5 (not 2.0)
5 occurrences → score contribution: 2.0 (not 5.0)
```

**2. Length Normalization (b parameter)**

Longer documents naturally contain more words, giving them an unfair advantage. BM25 uses b=0.75 to normalize by document length.

```
Short doc (50 words) with "water damage" → boosted score
Long doc (5000 words) with "water damage" → normalized score
```

#### The BM25 Formula

```
                          tf × (k1 + 1)
BM25(term) = IDF × ─────────────────────────────────────
                   tf + k1 × (1 - b + b × docLen/avgLen)

Where:
- tf = term frequency in document
- k1 = 1.2 (saturation parameter)
- b = 0.75 (length normalization)
- docLen = document length
- avgLen = average document length across corpus
- IDF = log((N - n + 0.5) / (n + 0.5) + 1)
```

#### Code Implementation

**File:** `server/router/api/v1/agent/vectordb.go` (lines 498-585)

```go
type BM25Scorer struct {
    k1        float64            // 1.2 - Term frequency saturation
    b         float64            // 0.75 - Length normalization
    docs      map[string][]string // Document ID → tokenized words
    docFreq   map[string]int     // Term → document frequency
    avgLen    float64            // Average document length
    totalDocs int
}

func (s *BM25Scorer) Score(query, docID string) float64 {
    // ... tokenize query ...

    for _, term := range queryTokens {
        tf := float64(termFreq[term])

        // IDF calculation
        n := float64(s.docFreq[term])
        idf := math.Log((float64(s.totalDocs)-n+0.5)/(n+0.5) + 1)

        // BM25 term score
        numerator := tf * (s.k1 + 1)
        denominator := tf + s.k1*(1-s.b+s.b*docLen/s.avgLen)
        score += idf * numerator / denominator
    }

    // Normalize to 0-1 range
    return score / (score + 1)
}
```

#### Tokenization

Before matching, text is broken into tokens:

```go
func tokenize(text string) []string {
    text = strings.ToLower(text)
    // Remove punctuation: . , ! ? ; : ( ) [ ] { } " ' - _ / \
    // Split on whitespace
    // Filter words < 2 characters
}
```

**Example:**
```
"Water Damage Restoration!" → ["water", "damage", "restoration"]
```

---

## 3. Score Fusion: Combining the Best of Both

### The Linear Combination Formula

```
hybrid_score = α × vector_score + (1 - α) × bm25_score

Where α = vector_weight (default: 0.7)
```

With default weights:
```
hybrid_score = 0.7 × vector_score + 0.3 × bm25_score
```

### Why 70/30 Split?

The default favors vector search (70%) because:
1. **Semantic understanding** catches more relevant results
2. AI agents need to understand *meaning*, not just keywords
3. Users often paraphrase—"burst pipe" vs "broken water line"

But BM25 (30%) ensures:
1. **Exact terms** are boosted (product names, policy numbers)
2. **Technical jargon** gets matched precisely
3. **Specific queries** get precise results

### Score Normalization: Why It's Critical

Both scores MUST be on the same scale (0-1) for meaningful combination.

**Vector Scores (Cosine Similarity):**
- Already normalized: range [0, 1]
- 1.0 = identical, 0.0 = unrelated

**BM25 Scores:**
- Raw output: range [0, ∞]
- Must be normalized using: `normalized = score / (score + 1)`

```
Raw BM25: 0.5 → Normalized: 0.5 / 1.5 = 0.33
Raw BM25: 1.0 → Normalized: 1.0 / 2.0 = 0.50
Raw BM25: 3.0 → Normalized: 3.0 / 4.0 = 0.75
Raw BM25: 10.0 → Normalized: 10.0 / 11.0 = 0.91
```

This sigmoid-like normalization ensures BM25 scores are comparable to cosine similarity.

### Worked Example

**Query:** "water damage restoration"

**Document:** "Our professional water damage restoration services include water extraction, structural drying, and mold prevention."

```
Step 1: Vector Search
  Query embedding:    [0.12, -0.45, 0.78, ...]
  Document embedding: [0.14, -0.42, 0.75, ...]
  Cosine similarity:  0.92

Step 2: BM25 Search
  Query tokens: ["water", "damage", "restoration"]
  Document tokens: ["our", "professional", "water", "damage", "restoration",
                   "services", "include", "water", "extraction", ...]

  Term frequencies in doc:
    "water": 2, "damage": 1, "restoration": 1

  IDF values (assuming 100 docs, term appears in n docs):
    "water": appears in 60 docs → IDF = 0.51
    "damage": appears in 45 docs → IDF = 0.69
    "restoration": appears in 20 docs → IDF = 1.39

  BM25 raw score: 2.8
  Normalized: 2.8 / 3.8 = 0.74

Step 3: Fusion
  hybrid_score = 0.7 × 0.92 + 0.3 × 0.74
               = 0.644 + 0.222
               = 0.866
```

---

## 4. The Complete Data Flow

```
                              USER QUERY
                                  │
                                  │ "water damage restoration"
                                  │
                    ┌─────────────┴─────────────┐
                    │                           │
                    ▼                           ▼
          ┌─────────────────┐        ┌─────────────────┐
          │  EMBED QUERY    │        │ TOKENIZE QUERY  │
          │  (384-dim vec)  │        │ ["water",       │
          └────────┬────────┘        │  "damage",      │
                   │                 │  "restoration"] │
                   │                 └────────┬────────┘
                   │                          │
         ┌─────────┴─────────┐               │
         │                   │               │
         ▼                   ▼               ▼
    ┌─────────┐    ┌──────────────┐  ┌──────────────┐
    │ BUILD   │    │ VECTOR       │  │ FTS SEARCH   │
    │ FILTER  │───▶│ SEARCH       │  │ (BM25)       │
    │ tenant= │    │ IVF-PQ index │  │ Tantivy idx  │
    │ active= │    │ 2×topK       │  │ 2×topK       │
    └─────────┘    └──────┬───────┘  └──────┬───────┘
                          │                 │
                          │ cosine scores   │ BM25 scores
                          │ [0-1]           │ [0-∞]→[0-1]
                          │                 │
                          └────────┬────────┘
                                   │
                                   ▼
                        ┌──────────────────┐
                        │  MERGE RESULTS   │
                        │  by document ID  │
                        │  ┌────┬────┬────┐│
                        │  │ ID │ V  │ B  ││
                        │  ├────┼────┼────┤│
                        │  │ d1 │0.92│0.74││
                        │  │ d2 │0.85│0.00││
                        │  │ d3 │0.00│0.81││
                        │  └────┴────┴────┘│
                        └────────┬─────────┘
                                 │
                                 ▼
                        ┌──────────────────┐
                        │ LINEAR COMBINE   │
                        │                  │
                        │ d1: 0.7×0.92 +   │
                        │     0.3×0.74     │
                        │   = 0.866        │
                        │                  │
                        │ d2: 0.7×0.85 +   │
                        │     0.3×0.00     │
                        │   = 0.595        │
                        │                  │
                        │ d3: 0.7×0.00 +   │
                        │     0.3×0.81     │
                        │   = 0.243        │
                        └────────┬─────────┘
                                 │
                                 ▼
                        ┌──────────────────┐
                        │ SORT & FILTER    │
                        │ minScore = 0.3   │
                        │ topK = 5         │
                        └────────┬─────────┘
                                 │
                                 ▼
                        ┌──────────────────┐
                        │ FINAL RESULTS    │
                        │ ┌────┬───────────┤
                        │ │Rank│  Score    │
                        │ ├────┼───────────┤
                        │ │ 1  │ d1: 0.866 │
                        │ │ 2  │ d2: 0.595 │
                        │ └────┴───────────┘
                        │ (d3 filtered out)│
                        └──────────────────┘
```

### Key Observations

1. **Candidate Expansion (2×topK):** We fetch twice as many candidates from each search to ensure good coverage before fusion.

2. **Document d1** appears in both searches → gets combined score → ranks highest.

3. **Document d2** only appears in vector search (semantic match) → still included but lower score.

4. **Document d3** only appears in BM25 search (keyword match) → filtered out due to low hybrid score, but would rank higher if weights were adjusted.

---

## 5. Configuration Guide

### Environment Variables

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `HYBRID_SEARCH_ENABLED` | bool | `false` | Master switch for hybrid search |
| `HYBRID_VECTOR_WEIGHT` | float | `0.7` | Weight for semantic similarity |
| `HYBRID_TEXT_WEIGHT` | float | `0.3` | Weight for keyword matching |
| `RAG_PIPELINE_ENABLED` | bool | `false` | Must be `true` for any RAG features |

### Example Configurations

**Basic Hybrid Search:**
```bash
export RAG_PIPELINE_ENABLED=true
export HYBRID_SEARCH_ENABLED=true
```

**Custom Weights (More Keyword Focus):**
```bash
export RAG_PIPELINE_ENABLED=true
export HYBRID_SEARCH_ENABLED=true
export HYBRID_VECTOR_WEIGHT=0.5
export HYBRID_TEXT_WEIGHT=0.5
```

**Maximum Keyword Precision:**
```bash
export HYBRID_SEARCH_ENABLED=true
export HYBRID_VECTOR_WEIGHT=0.3
export HYBRID_TEXT_WEIGHT=0.7
```

### Per-Query Override

The `SearchQuery` struct allows per-query customization:

```go
result, err := db.Search(ctx, SearchQuery{
    QueryText:       "water damage policy WD-2024-001",
    TenantID:        tenantID,
    UseHybridSearch: true,       // Enable hybrid
    VectorWeight:    0.4,        // Less semantic
    TextWeight:      0.6,        // More keyword (policy number!)
    TopK:            5,
    MinScore:        0.3,
})
```

### Weight Tuning Guidelines

| Use Case | Vector | Text | Rationale |
|----------|--------|------|-----------|
| **General Q&A** | 0.7 | 0.3 | Semantic understanding priority |
| **Policy lookup** | 0.4 | 0.6 | Exact policy numbers matter |
| **Technical docs** | 0.5 | 0.5 | Balance jargon + concepts |
| **Product search** | 0.3 | 0.7 | Exact product names critical |

---

## 6. Testing and Verification

### Run Unit Tests

```bash
# All agent tests (includes hybrid search tests)
go test -v ./server/router/api/v1/agent/...

# Expected output:
# --- PASS: TestBM25Scorer_Score_ExactMatch
# --- PASS: TestBM25Scorer_Score_IDFWeighting
# --- PASS: TestBM25Scorer_Score_LengthNormalization
# --- PASS: TestMemoryVectorDB_Search_Hybrid
# --- PASS: TestVectorDBConfig_HybridSearchEnabled
```

### Verify Hybrid Search is Active

Check server startup logs:
```
INFO Hybrid search enabled vector_weight=0.7 text_weight=0.3
```

### Debug with Component Scores

The `SearchResult` includes component scores for analysis:

```go
result, _ := db.Search(ctx, query)

fmt.Printf("Search Mode: %s\n", result.SearchMode)  // "hybrid"

for i, chunk := range result.Chunks {
    fmt.Printf("Result %d: %s\n", i+1, chunk.Title)
    fmt.Printf("  Hybrid Score: %.3f\n", result.Scores[i])
    fmt.Printf("  Vector Score: %.3f\n", result.VectorScores[i])
    fmt.Printf("  BM25 Score:   %.3f\n", result.BM25Scores[i])
}
```

**Example Output:**
```
Search Mode: hybrid

Result 1: Water Damage Restoration Services
  Hybrid Score: 0.866
  Vector Score: 0.920
  BM25 Score:   0.740

Result 2: Emergency Water Extraction
  Hybrid Score: 0.721
  Vector Score: 0.850
  BM25 Score:   0.420
```

### Integration Tests (with LanceDB)

```bash
# Requires LanceDB native library
LD_LIBRARY_PATH=./lancedb-go-main/pkg/lib/lib_linux_amd64:$LD_LIBRARY_PATH \
  go test -tags "rag integration" -v ./server/router/api/v1/agent/...
```

---

## 7. Real-World Examples

### Example 1: General Customer Query

**Query:** "Do you handle flood damage?"

**Vector Search Wins Here:**
- "flood damage" ≈ "water damage" semantically
- Finds documents about water restoration even without exact "flood" keyword

**Results:**
| Rank | Document | Vector | BM25 | Hybrid |
|------|----------|--------|------|--------|
| 1 | Water damage restoration services | 0.88 | 0.00 | 0.616 |
| 2 | Flood cleanup and water extraction | 0.82 | 0.75 | 0.799 |
| 3 | Emergency water response team | 0.79 | 0.00 | 0.553 |

**Note:** Document 2 ranks highest because it matches both semantically AND has "flood" keyword.

### Example 2: Specific Policy Lookup

**Query:** "Coverage policy WD-2024-001"

**BM25 Wins Here:**
- Exact policy number "WD-2024-001" is critical
- Vector search sees it as random characters

**Recommended Config:** `VectorWeight=0.3, TextWeight=0.7`

**Results:**
| Rank | Document | Vector | BM25 | Hybrid |
|------|----------|--------|------|--------|
| 1 | Policy WD-2024-001: Water Damage Coverage | 0.45 | 0.95 | 0.80 |
| 2 | General coverage information | 0.72 | 0.30 | 0.43 |

### Example 3: Technical Service Question

**Query:** "IICRC S500 water damage standards"

**Both Matter:**
- "IICRC S500" is a specific standard code (BM25)
- "water damage standards" is conceptual (Vector)

**With 50/50 Weights:**
| Rank | Document | Vector | BM25 | Hybrid |
|------|----------|--------|------|--------|
| 1 | IICRC S500 Certification Standards | 0.65 | 0.92 | 0.785 |
| 2 | Industry water damage protocols | 0.85 | 0.40 | 0.625 |
| 3 | Our IICRC certified technicians | 0.55 | 0.78 | 0.665 |

---

## 8. Technical Reference

### Key Files and Functions

| File | Function/Struct | Line | Purpose |
|------|-----------------|------|---------|
| `vectordb.go` | `BM25Scorer` | 498-520 | BM25 scoring implementation |
| `vectordb.go` | `Score()` | 542-585 | Calculate BM25 score for query/doc |
| `vectordb.go` | `tokenize()` | 587-612 | Text tokenization |
| `vectordb.go` | `cosineSimilarity()` | 473-492 | Vector similarity |
| `vectordb.go` | `SearchQuery` | 106-110 | Hybrid search parameters |
| `vectordb.go` | `SearchResult` | 119-123 | Results with component scores |
| `vectordb_lance.go` | `hybridSearch()` | 488-594 | LanceDB hybrid implementation |
| `vectordb_lance.go` | `createIndexes()` | 177-201 | FTS index creation |
| `vectordb_lance.go` | `distanceToScore()` | 596-610 | L2 distance → similarity |

### Index Types

| Index | Type | Purpose | Library |
|-------|------|---------|---------|
| `idx_embedding` | IVF-PQ | Vector similarity search | LanceDB |
| `idx_tenant` | BTree | Tenant filtering | LanceDB |
| `idx_content_fts` | FTS | BM25 keyword search on content | Tantivy |
| `idx_title_fts` | FTS | BM25 keyword search on title | Tantivy |

### Performance Characteristics

| Operation | Typical Latency | Notes |
|-----------|-----------------|-------|
| Query embedding | 50-200ms | Depends on embedding provider |
| Vector search (IVF-PQ) | 10-50ms | Scales well with data size |
| FTS search (Tantivy) | 5-20ms | Very fast keyword lookup |
| Score fusion | <1ms | Simple arithmetic |
| **Total hybrid search** | 70-270ms | Dominated by embedding time |

### Memory and Storage

- **Embedding dimension:** 384 (all-MiniLM-L6-v2) or 1536 (OpenAI)
- **Vector storage:** ~1.5KB per document (384-dim × 4 bytes)
- **FTS index:** ~30% of text content size (Tantivy compressed)

---

## 9. Troubleshooting

### "Hybrid search returns same results as vector-only"

**Cause:** FTS index may not be created or is empty.

**Fix:**
1. Check logs for FTS index creation warnings
2. Ensure `RAG_PIPELINE_ENABLED=true`
3. Re-index content: `REINDEX_RAG=true`

### "BM25 scores are always 0"

**Cause:** Query text is empty or all tokens filtered out.

**Fix:**
1. Ensure `QueryText` is set (not just `QueryEmbedding`)
2. Check tokenization—words < 2 chars are filtered

### "Results don't match expected ranking"

**Debug Steps:**
1. Enable component score logging (see Section 6)
2. Check individual Vector vs BM25 scores
3. Adjust weights based on your use case

### "Hybrid search is slow"

**Cause:** Usually the embedding generation, not the search.

**Optimize:**
1. Use local embedding service for lower latency
2. Consider caching common query embeddings
3. Pre-compute embeddings for known query patterns

---

## Related Documentation

- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - All environment variables
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG architecture overview
- [DOCS_HYBRID_SEARCH.MD](./DOCS_HYBRID_SEARCH.MD) - Implementation plan and rollout

---

*Document Version: 1.0 | Created: 2026-01-23*
