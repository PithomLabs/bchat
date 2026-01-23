# Hybrid Search Evaluation Guide

**Status:** Active
**Date:** 2026-01-23
**Related Documents:**
- [DOCS_HYBRID_SEARCH.MD](./DOCS_HYBRID_SEARCH.MD) - Implementation plan
- [DOCS_HYBRID_SEARCH_EXPLAINED.MD](./DOCS_HYBRID_SEARCH_EXPLAINED.MD) - Technical details
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG architecture

---

## Overview

This document describes the evaluation framework for the hybrid search system, covering:

1. **Evaluation Dataset Creation** - Building ground truth for testing
2. **Quality Metrics Comparison** - Measuring retrieval effectiveness
3. **Performance Benchmarking** - Measuring latency and throughput

### What We're Measuring

| Category | Metrics | Purpose |
|----------|---------|---------|
| **Retrieval Quality** | Recall, Precision, MRR, NDCG | Does search find relevant content? |
| **Response Quality** | Grounding, Hallucination rate | Are responses accurate? |
| **Performance** | Latency, Throughput, Memory | Is it fast enough? |

---

## Existing Evaluation Infrastructure

### Test Files

| File | Purpose | Lines |
|------|---------|-------|
| `vectordb_test.go` | BM25/hybrid unit tests | 598 |
| `vectordb_lance_test.go` | LanceDB integration tests | 407 |
| `analysis.go` | Conversation scoring rubric | 287 |
| `simulation.go` | Synthetic conversation generation | ~80 |

### Currently Tracked Metrics

**Search Results:**
```go
type SearchResult struct {
    Chunks       []DocumentChunk
    Scores       []float64       // Combined hybrid scores
    Total        int
    Latency      time.Duration
    SearchMode   string          // "vector", "hybrid", "bm25"
    VectorScores []float64       // Component scores for analysis
    BM25Scores   []float64       // Component scores for analysis
}
```

**Conversation Analysis:**
```go
type AnalysisResult struct {
    Score     int               // 0-100 overall score
    Grade     string            // A/B/C/D/F
    Breakdown AnalysisBreakdown // Per-category scores
    Issues    []AnalysisIssue   // Detected problems
}
```

### Running Existing Tests

```bash
# Unit tests (no dependencies)
go test -v ./server/router/api/v1/agent/...

# Integration tests (requires LanceDB library)
LD_LIBRARY_PATH=./lancedb-go-main/pkg/lib/lib_linux_amd64:$LD_LIBRARY_PATH \
  go test -tags "rag integration" -v ./server/router/api/v1/agent/...
```

---

## Evaluation Dataset Creation

### Directory Structure

```
docs/evaluation/
├── datasets/
│   ├── queries.json              # Test queries with ground truth
│   ├── kb_fixtures/
│   │   ├── restoration_kb.md     # Water restoration KB
│   │   ├── insurance_kb.md       # Insurance Q&A KB
│   │   └── retail_kb.md          # Retail product KB
│   └── sample_conversations/
│       ├── good_conversation.json
│       ├── poor_conversation.json
│       └── edge_cases.json
└── benchmarks/
    ├── baseline_results.json     # Vector-only baseline
    └── hybrid_results.json       # Hybrid search results
```

### Query Dataset Schema

```json
{
  "dataset_version": "1.0",
  "created": "2026-01-23",
  "queries": [
    {
      "query_id": "q001",
      "query_text": "water damage emergency flooding basement",
      "query_type": "semantic",
      "category": "emergency",
      "expected_chunks": ["water_extraction_001", "emergency_response_002"],
      "relevance_labels": {
        "water_extraction_001": 3,
        "emergency_response_002": 3,
        "mold_remediation_003": 1
      },
      "notes": "Should prioritize emergency response content"
    },
    {
      "query_id": "q002",
      "query_text": "IICRC S500 certification requirements",
      "query_type": "keyword",
      "category": "technical",
      "expected_chunks": ["certifications_001"],
      "relevance_labels": {
        "certifications_001": 3,
        "training_002": 2
      },
      "notes": "Exact keyword match critical - tests BM25"
    },
    {
      "query_id": "q003",
      "query_text": "help with water problem at home",
      "query_type": "mixed",
      "category": "ambiguous",
      "expected_chunks": ["water_damage_001", "plumbing_002"],
      "relevance_labels": {
        "water_damage_001": 2,
        "plumbing_002": 2,
        "water_heater_003": 1
      },
      "notes": "Ambiguous query - tests hybrid combination"
    }
  ]
}
```

### Relevance Label Scale

| Score | Meaning | Example |
|-------|---------|---------|
| 3 | Highly relevant | Directly answers the query |
| 2 | Relevant | Related and useful |
| 1 | Marginally relevant | Tangentially related |
| 0 | Not relevant | Unrelated content |

### Query Type Categories

| Type | Description | Primary Search Mode |
|------|-------------|---------------------|
| `semantic` | Meaning-based queries | Vector search |
| `keyword` | Technical terms, proper nouns | BM25 search |
| `mixed` | Combination of both | Hybrid search |
| `ambiguous` | Unclear intent | Tests fallback behavior |

### Building a Test Dataset

**Step 1: Extract chunks from test KB**
```bash
# Index test content
curl -X POST http://localhost:8080/api/v1/agent/test-tenant/kb/import \
  -F "file=@docs/evaluation/datasets/kb_fixtures/restoration_kb.md"
```

**Step 2: Query and record results**
```bash
# Get search results with component scores
curl -X POST http://localhost:8080/api/v1/agent/test-tenant/search \
  -H "Content-Type: application/json" \
  -d '{"query": "water damage emergency", "hybrid": true, "top_k": 10}'
```

**Step 3: Manually label relevance**
- Review each result
- Assign relevance score (0-3)
- Record in queries.json

---

## Quality Metrics

### Retrieval Metrics Definitions

#### Recall@K
**What it measures:** Of all relevant documents, how many did we find in top K?

```
Recall@K = |Relevant ∩ Retrieved@K| / |Relevant|
```

**Example:**
- 5 relevant documents exist
- Top 10 results contain 4 of them
- Recall@10 = 4/5 = 0.80

**Target:** > 0.85 for Recall@10

#### Precision@K
**What it measures:** Of the top K results, how many are relevant?

```
Precision@K = |Relevant ∩ Retrieved@K| / K
```

**Example:**
- Top 10 results
- 7 are relevant
- Precision@10 = 7/10 = 0.70

**Target:** > 0.70 for Precision@10

#### MRR (Mean Reciprocal Rank)
**What it measures:** How quickly do we find the first relevant result?

```
MRR = (1/|Q|) × Σ(1/rank_i)
```

**Example:**
- Query 1: First relevant at rank 1 → 1/1 = 1.0
- Query 2: First relevant at rank 3 → 1/3 = 0.33
- Query 3: First relevant at rank 2 → 1/2 = 0.5
- MRR = (1.0 + 0.33 + 0.5) / 3 = 0.61

**Target:** > 0.65

#### NDCG (Normalized Discounted Cumulative Gain)
**What it measures:** Quality of ranking considering graded relevance

```
DCG@K = Σ(rel_i / log2(i+1))
NDCG@K = DCG@K / IDCG@K
```

Where IDCG is the ideal DCG (perfect ranking).

**Example:**
- Results: [rel=3, rel=1, rel=2, rel=0, rel=3]
- DCG = 3/1 + 1/1.58 + 2/2 + 0/2.32 + 3/2.58 = 5.80
- Ideal: [3, 3, 2, 1, 0]
- IDCG = 3/1 + 3/1.58 + 2/2 + 1/2.32 + 0/2.58 = 6.33
- NDCG = 5.80/6.33 = 0.92

**Target:** > 0.80

### Quality Comparison Matrix

Run evaluations across search modes and configurations:

| Configuration | Recall@10 | Precision@10 | MRR | NDCG | Latency |
|---------------|-----------|--------------|-----|------|---------|
| Vector-only | - | - | - | - | - |
| Hybrid (0.7/0.3) | - | - | - | - | - |
| Hybrid (0.5/0.5) | - | - | - | - | - |
| Hybrid (0.3/0.7) | - | - | - | - | - |
| BM25-only | - | - | - | - | - |

### Query-Type Analysis

Evaluate metrics by query type:

| Query Type | Vector-only | Hybrid | BM25-only | Winner |
|------------|-------------|--------|-----------|--------|
| Semantic | - | - | - | - |
| Keyword | - | - | - | - |
| Mixed | - | - | - | - |
| Ambiguous | - | - | - | - |

---

## Performance Benchmarking

### Latency Metrics

| Metric | Description | Target | Measured |
|--------|-------------|--------|----------|
| Query embedding | Text → vector | < 200ms | - |
| Vector search | ANN lookup | < 50ms | - |
| BM25 search | FTS lookup | < 20ms | - |
| Score fusion | Combine & sort | < 1ms | - |
| **Total latency** | End-to-end | < 300ms | - |

### Resource Metrics

| Metric | Description | Target | Measured |
|--------|-------------|--------|----------|
| Index size (vector) | IVF-PQ index | ~4KB/chunk | - |
| Index size (FTS) | Tantivy index | ~2KB/chunk | - |
| Memory usage | Runtime memory | < 500MB baseline | - |
| Throughput | Queries/second | > 10 QPS | - |

### Benchmarking Commands

```bash
# Run Go benchmarks
go test -tags "rag" -bench=. -benchmem ./server/router/api/v1/agent/...

# Expected output format:
# BenchmarkVectorSearch-8     1000    1234567 ns/op    4096 B/op    12 allocs/op
# BenchmarkHybridSearch-8      800    1567890 ns/op    5120 B/op    18 allocs/op
```

### Load Testing

```bash
# Using hey (install: go install github.com/rakyll/hey@latest)
hey -n 1000 -c 10 \
  -m POST \
  -H "Content-Type: application/json" \
  -d '{"query": "water damage", "hybrid": true}' \
  http://localhost:8080/api/v1/agent/test/search
```

### Latency Breakdown Analysis

Enable debug logging to see component latencies:

```go
// In vectordb_lance.go hybridSearch()
slog.Debug("hybrid search timing",
    "embedding_ms", embeddingTime.Milliseconds(),
    "vector_search_ms", vectorTime.Milliseconds(),
    "bm25_search_ms", bm25Time.Milliseconds(),
    "fusion_ms", fusionTime.Milliseconds(),
    "total_ms", totalTime.Milliseconds(),
)
```

---

## Test Scenarios

### Scenario 1: Semantic Understanding

**Query:** "My basement got flooded and there's standing water everywhere"

**Expected:** Vector search should find water damage/extraction content even though exact keywords may not match.

| Search Mode | Expected Rank | Actual Rank | Pass/Fail |
|-------------|---------------|-------------|-----------|
| Vector-only | 1 | - | - |
| Hybrid | 1 | - | - |
| BM25-only | 3+ | - | - |

### Scenario 2: Exact Keyword Match

**Query:** "IICRC S500 water damage restoration standards"

**Expected:** BM25 should boost this due to exact technical terms.

| Search Mode | Expected Rank | Actual Rank | Pass/Fail |
|-------------|---------------|-------------|-----------|
| Vector-only | 2-3 | - | - |
| Hybrid | 1 | - | - |
| BM25-only | 1 | - | - |

### Scenario 3: Ambiguous Query

**Query:** "help with water"

**Expected:** Hybrid should balance semantic (water damage context) with keyword (water mentions).

| Search Mode | Top Result | Relevance | Notes |
|-------------|------------|-----------|-------|
| Vector-only | - | - | - |
| Hybrid | - | - | - |
| BM25-only | - | - | - |

### Scenario 4: Out-of-Domain Query

**Query:** "pizza delivery near me"

**Expected:** All modes should return low-relevance results or empty set.

| Search Mode | Results | Max Score | Expected Behavior |
|-------------|---------|-----------|-------------------|
| All modes | 0 or low | < 0.3 | Correctly identifies irrelevant |

---

## Running Evaluations

### Step-by-Step Process

**1. Prepare Environment**
```bash
# Start server with RAG enabled
RAG_PIPELINE_ENABLED=true HYBRID_SEARCH_ENABLED=true task run:rag
```

**2. Load Test Data**
```bash
# Import evaluation KB
curl -X POST http://localhost:8080/api/v1/agent/eval-tenant/kb/import \
  -F "file=@docs/evaluation/datasets/kb_fixtures/restoration_kb.md"
```

**3. Run Evaluation Script**
```bash
# Example evaluation runner (to be implemented)
go run cmd/evaluate/main.go \
  --dataset docs/evaluation/datasets/queries.json \
  --tenant eval-tenant \
  --modes vector,hybrid,bm25 \
  --output docs/evaluation/benchmarks/results.json
```

**4. Generate Report**
```
Search Mode Comparison (50 queries)
=====================================
                    Vector-Only    Hybrid (0.7/0.3)    BM25-Only
Recall@10           0.72           0.86                0.64
Precision@10        0.58           0.71                0.52
MRR                 0.54           0.68                0.48
NDCG@10             0.61           0.79                0.55
Avg Latency         45ms           67ms                22ms

By Query Type:
                    Vector    Hybrid    BM25
Semantic            0.89      0.91      0.52
Keyword             0.48      0.82      0.88
Mixed               0.71      0.85      0.63
Ambiguous           0.62      0.75      0.45
```

---

## Interpretation Guidelines

### When Vector Search Wins
- Semantic queries with paraphrasing
- Natural language questions
- Conceptually related content

### When BM25 Wins
- Technical terms and acronyms
- Proper nouns and names
- Exact phrase matches

### When Hybrid Wins
- Mixed queries with both semantic and keyword aspects
- Ambiguous queries needing multiple signals
- Queries where both approaches return relevant (but different) content

### Tuning Weight Configuration

| Scenario | Recommended Weights | Rationale |
|----------|---------------------|-----------|
| General Q&A | 0.7 vector / 0.3 BM25 | Semantic understanding primary |
| Technical docs | 0.5 vector / 0.5 BM25 | Balance for technical terms |
| Search logs show keyword patterns | 0.4 vector / 0.6 BM25 | Users search with keywords |

---

## Future Evaluation Capabilities

### Planned Additions

| Capability | Status | Priority |
|------------|--------|----------|
| Hallucination detection | Not started | High |
| Grounding score calculation | Not started | High |
| A/B testing framework | Not started | Medium |
| Regression detection | Not started | Medium |
| CI/CD integration | Not started | Medium |
| Evaluation dashboard | Not started | Low |

### Hallucination Detection (Planned)

```go
type HallucinationAnalysis struct {
    HallucinationRate float64    // % claims not in context
    GroundingScore    float64    // % claims traceable to KB
    UncitedClaims     []string   // Statements without sources
    ConflictingInfo   []Conflict // Contradictions
}
```

### A/B Testing Framework (Planned)

```go
type ExperimentResult struct {
    ControlGroup   string   // "vector-only"
    TreatmentGroup string   // "hybrid"
    Metric         string   // "relevance"
    Improvement    float64  // Relative improvement
    PValue         float64  // Statistical significance
    Significant    bool     // p < 0.05
}
```

---

## Appendix: Metric Calculation Code

### Recall@K
```go
func RecallAtK(retrieved []string, relevant []string, k int) float64 {
    if len(relevant) == 0 {
        return 0
    }
    topK := retrieved
    if len(topK) > k {
        topK = topK[:k]
    }

    relevantSet := make(map[string]bool)
    for _, r := range relevant {
        relevantSet[r] = true
    }

    found := 0
    for _, r := range topK {
        if relevantSet[r] {
            found++
        }
    }

    return float64(found) / float64(len(relevant))
}
```

### MRR
```go
func MRR(queries []QueryResult) float64 {
    sum := 0.0
    for _, q := range queries {
        for rank, doc := range q.Retrieved {
            if q.IsRelevant(doc) {
                sum += 1.0 / float64(rank+1)
                break
            }
        }
    }
    return sum / float64(len(queries))
}
```

---

*Document Version: 1.0*
*Created: 2026-01-23*
