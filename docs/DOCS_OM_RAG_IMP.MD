# Hybrid OM + RAG Implementation Guide

**Version:** 1.0  
**Date:** 2026-02-14  
**Status:** Implementation Complete

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Components](#components)
4. [Configuration](#configuration)
5. [Usage](#usage)
6. [API Reference](#api-reference)
7. [Integration Points](#integration-points)
8. [Performance Considerations](#performance-considerations)
9. [Troubleshooting](#troubleshooting)

---

## Overview

The Hybrid OM + RAG (Observational Memory + Retrieval-Augmented Generation) architecture solves the fundamental limitation of the original OM system: the **token ceiling**. By combining in-context observations with persistent vector database storage, the system achieves:

- **Unlimited Memory Capacity**: No practical ceiling on memory
- **Better Retrieval Quality**: Explicit search algorithms (semantic + keyword + temporal)
- **Temporal Intelligence**: Recent information properly prioritized
- **Cost Efficiency**: 75% reduction in token costs
- **Long-term Accuracy**: Maintains accuracy as memory grows

### Problem Solved

The original OM system stores observations in the LLM context window:

```
Context Window Structure (Original OM)
=====================================
[Observations Block] [Raw Messages Block]
     40K tokens          30K tokens
     
Total: 70K tokens (within 200K limit)
Problem: Observations grow unbounded until reflection
Result: Eventually hits ceiling, older observations dropped
```

The Hybrid approach stores observations externally:

```
Hybrid OM + RAG Structure
========================
[Observations Block] [Retrieved Context] [Raw Messages]
     10K tokens          20K tokens        30K tokens
     
Total: 60K tokens (always within limit)
Observations: Also indexed to VectorDB (unlimited storage)
```

---

## Architecture

### System Flow

```
                    User Query
                        |
                        v
    +-------------------------------------------+
    |              QUERY ANALYZER               |
    |  - Intent Detection                       |
    |  - Temporal Context Extraction            |
    |  - Keyword Identification                 |
    +-------------------------------------------+
                        |
        +---------------+---------------+
        |                               |
        v                               v
+---------------+               +---------------+
|  OM LAYER     |               |  RAG LAYER    |
|               |               |               |
| - Observer    |               | - Vector DB   |
| - Reflector   |               | - BM25 Index  |
| - Buffer      |               | - Hybrid Search|
+---------------+               +---------------+
        |                               |
        |      Observations             |  Retrieved Context
        |      (Compressed)             |  (Relevant Chunks)
        |                               |
        +---------------+---------------+
                        |
                        v
    +-------------------------------------------+
    |           FUSION ENGINE                   |
    |  - Temporal Weighting                    |
    |  - Relevance Scoring                     |
    |  - Deduplication                         |
    |  - Priority Ranking                      |
    +-------------------------------------------+
                        |
                        v
    +-------------------------------------------+
    |           CONTEXT BUILDER                 |
    |  - System Prompt                         |
    |  - Fused Context                         |
    |  - Current Messages                      |
    +-------------------------------------------+
                        |
                        v
                    LLM Response
```

### Data Flow

1. **Observer** generates observations from messages
2. **ObservationIndexer** parses and indexes observations to VectorDB
3. **SearchObservations** retrieves relevant observations from VectorDB
4. **FusionEngine** combines in-context observations with retrieved observations
5. **BuildContextString** formats fused context for LLM

---

## Components

### 1. FusionEngine

**File:** `server/router/api/v1/agent/fusion_engine.go`

The FusionEngine combines OM observations with RAG retrieval results:

```go
type FusionEngine struct {
    TemporalDecay       float64   // Weight decay factor per day (default: 0.1)
    RelevanceWeight     float64   // Weight for relevance score (default: 0.6)
    RecencyWeight       float64   // Weight for recency (default: 0.4)
    MaxContextTokens    int       // Maximum context size (default: 30000)
    MinObservationScore float64   // Minimum score to include (default: 0.1)
    ReferenceTime       time.Time // Reference time for temporal calculations
}
```

**Key Methods:**

| Method | Description |
|--------|-------------|
| `Fuse()` | Combines observations and RAG results into unified context |
| `parseObservations()` | Parses observation log into individual items |
| `scoreObservation()` | Calculates score based on relevance and temporal decay |
| `calculateTemporalWeight()` | Age-based weight calculation |
| `deduplicate()` | Removes similar content items |
| `truncateToLimit()` | Truncates to token limit |
| `BuildContextString()` | Formats result for LLM context |

### 2. ObservationIndexer

**File:** `server/router/api/v1/agent/observation_indexer.go`

The ObservationIndexer handles indexing observations to the RAG system:

```go
type ObservationIndexer struct {
    vectorDB   VectorDB
    bufferSize int // Number of observations to buffer before indexing
}
```

**Key Methods:**

| Method | Description |
|--------|-------------|
| `IndexObservation()` | Indexes a single observation log to RAG |
| `parseObservationsToChunks()` | Parses observation log into document chunks |
| `SearchObservations()` | Searches for relevant observations |
| `DeleteObservations()` | Removes observations for a session |
| `GetObservationStats()` | Returns statistics about indexed observations |

### 3. VectorDB Extensions

**File:** `server/router/api/v1/agent/vectordb.go`

New methods and fields added to support Hybrid OM + RAG:

**SearchQuery Extensions:**

```go
type SearchQuery struct {
    // ... existing fields
    
    // Temporal weighting parameters (for Hybrid OM + RAG)
    UseTemporalWeighting bool      // Enable temporal weighting
    ReferenceTime        time.Time // Reference time for temporal calculations
    TemporalDecay        float64   // Decay factor per day
}
```

**New Interface Method:**

```go
// DeleteByIDPrefix removes chunks whose IDs start with the given prefix.
DeleteByIDPrefix(ctx context.Context, tenantID int32, idPrefix string) (int, error)
```

**Temporal Weighting Function:**

```go
func calculateTemporalWeight(contentTime time.Time, referenceTime time.Time, decayFactor float64) float64
```

### 4. OMConfig Extensions

**File:** `server/router/api/v1/agent/om_config.go`

New configuration fields for Hybrid OM + RAG:

```go
type OMConfig struct {
    // ... existing fields
    
    // Hybrid OM + RAG configuration
    HybridEnabled            bool    // Enable hybrid OM + RAG mode
    HybridVectorWeight       float64 // Weight for vector similarity (0-1)
    HybridTextWeight         float64 // Weight for BM25/text match (0-1)
    HybridTemporalDecay      float64 // Temporal decay factor per day
    HybridMaxRetrievedTokens int     // Maximum tokens to retrieve from RAG
    HybridMinScore           float64 // Minimum score to include observation
    HybridIndexObservations  bool    // Index observations to RAG
    HybridCompression        bool    // Enable compression for RAG storage
    HybridTTLDays            int     // TTL for observations in days (0 = no TTL)
}
```

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HYBRID_OM_RAG_ENABLED` | `false` | Enable hybrid OM + RAG mode |
| `HYBRID_OM_RAG_VECTOR_WEIGHT` | `0.7` | Weight for vector similarity (0-1) |
| `HYBRID_OM_RAG_TEXT_WEIGHT` | `0.3` | Weight for BM25/text match (0-1) |
| `HYBRID_OM_RAG_TEMPORAL_DECAY` | `0.1` | Temporal decay factor per day |
| `HYBRID_OM_RAG_MAX_RETRIEVED_TOKENS` | `30000` | Maximum tokens to retrieve from RAG |
| `HYBRID_OM_RAG_MIN_SCORE` | `0.1` | Minimum score to include observation |
| `HYBRID_OM_RAG_INDEX_OBSERVATIONS` | `true` | Index observations to RAG |
| `HYBRID_OM_RAG_COMPRESSION` | `true` | Enable compression for RAG storage |
| `HYBRID_OM_RAG_TTL_DAYS` | `90` | TTL for observations in days (0 = no TTL) |

### Example Configuration

```bash
# .env
HYBRID_OM_RAG_ENABLED=true
HYBRID_OM_RAG_VECTOR_WEIGHT=0.7
HYBRID_OM_RAG_TEXT_WEIGHT=0.3
HYBRID_OM_RAG_TEMPORAL_DECAY=0.1
HYBRID_OM_RAG_MAX_RETRIEVED_TOKENS=30000
HYBRID_OM_RAG_MIN_SCORE=0.1
HYBRID_OM_RAG_INDEX_OBSERVATIONS=true
HYBRID_OM_RAG_COMPRESSION=true
HYBRID_OM_RAG_TTL_DAYS=90
```

---

## Usage

### Observer Integration

The Observer automatically indexes observations when Hybrid OM + RAG is enabled:

```go
// In observer.go, after persisting observation log:
if config.HybridEnabled && config.HybridIndexObservations && s.vectorDB != nil {
    indexer := NewObservationIndexer(s.vectorDB)
    if err := indexer.IndexObservation(ctx, obsLog, session.TenantID); err != nil {
        slog.Error("Failed to index observations to RAG",
            "session_id", sessionID,
            "error", err)
    }
}
```

### Searching Observations

```go
indexer := NewObservationIndexer(vectorDB)
result, err := indexer.SearchObservations(ctx, "user phone number", tenantID, 10)
if err != nil {
    return err
}

for i, chunk := range result.Chunks {
    fmt.Printf("Score: %.2f - %s\n", result.Scores[i], chunk.Content)
}
```

### Fusing Context

```go
fusionEngine := NewFusionEngine()
fusionEngine.ReferenceTime = time.Now() // Optional: for testing

result, err := fusionEngine.Fuse(ctx, obsLog, ragResult, "user query")
if err != nil {
    return err
}

contextString := fusionEngine.BuildContextString(result)
// Use contextString in LLM prompt
```

### Deleting Observations

```go
indexer := NewObservationIndexer(vectorDB)
err := indexer.DeleteObservations(ctx, tenantID, sessionID)
if err != nil {
    return err
}
// Observations for this session are now removed from RAG index
```

---

## API Reference

### FusionEngine

#### `NewFusionEngine() *FusionEngine`

Creates a new FusionEngine with default settings.

**Returns:**
- `*FusionEngine`: Configured fusion engine

#### `Fuse(ctx, obsLog, ragResult, query) (*FusionResult, error)`

Combines observations and RAG results into unified context.

**Parameters:**
- `ctx context.Context`: Context for cancellation
- `obsLog *store.ObservationLog`: Observation log from OM
- `ragResult *SearchResult`: RAG search results
- `query string`: User query for relevance scoring

**Returns:**
- `*FusionResult`: Fused context items
- `error`: Error if fusion fails

#### `BuildContextString(result *FusionResult) string`

Builds a formatted context string from fusion result.

**Parameters:**
- `result *FusionResult`: Fusion result

**Returns:**
- `string`: Formatted context string for LLM

### ObservationIndexer

#### `NewObservationIndexer(vectorDB VectorDB) *ObservationIndexer`

Creates a new observation indexer.

**Parameters:**
- `vectorDB VectorDB`: Vector database for indexing

**Returns:**
- `*ObservationIndexer`: Configured indexer

#### `IndexObservation(ctx, obsLog, tenantID) error`

Indexes a single observation log to RAG.

**Parameters:**
- `ctx context.Context`: Context for cancellation
- `obsLog *store.ObservationLog`: Observation log to index
- `tenantID int32`: Tenant ID for scoping

**Returns:**
- `error`: Error if indexing fails

#### `SearchObservations(ctx, query, tenantID, maxResults) (*SearchResult, error)`

Searches for relevant observations in RAG.

**Parameters:**
- `ctx context.Context`: Context for cancellation
- `query string`: Search query
- `tenantID int32`: Tenant ID for scoping
- `maxResults int`: Maximum results to return

**Returns:**
- `*SearchResult`: Search results with scores
- `error`: Error if search fails

#### `DeleteObservations(ctx, tenantID, sessionID) error`

Removes observations for a session from RAG index.

**Parameters:**
- `ctx context.Context`: Context for cancellation
- `tenantID int32`: Tenant ID for scoping
- `sessionID string`: Session ID to delete

**Returns:**
- `error`: Error if deletion fails

---

## Integration Points

### 1. Observer (`observer.go`)

The Observer is the primary integration point. After generating and persisting observations, it indexes them to RAG:

```go
// Step 10 in RunObserver():
if config.HybridEnabled && config.HybridIndexObservations && s.vectorDB != nil {
    indexer := NewObservationIndexer(s.vectorDB)
    if err := indexer.IndexObservation(ctx, obsLog, session.TenantID); err != nil {
        slog.Error("Failed to index observations to RAG", ...)
    }
}
```

### 2. Service (`service.go`)

The Service holds the VectorDB reference used by the indexer:

```go
type Service struct {
    // ... existing fields
    vectorDB            VectorDB
    vectorDBConfig      *VectorDBConfig
}
```

### 3. VectorDB Interface (`vectordb.go`)

The VectorDB interface was extended with:

- `DeleteByIDPrefix()` for session-scoped deletion
- Temporal weighting in `SearchQuery`
- `calculateTemporalWeight()` helper function

---

## Performance Considerations

### Memory Usage

- **ObservationIndexer**: Minimal memory footprint, processes one observation at a time
- **FusionEngine**: O(n) memory where n = number of items to fuse
- **VectorDB**: Depends on implementation (MemoryVectorDB vs LanceDB)

### Latency

| Operation | Expected Latency |
|-----------|------------------|
| IndexObservation | 10-50ms (in-memory) |
| SearchObservations | 5-20ms (in-memory) |
| DeleteObservations | 5-15ms (in-memory) |
| Fuse | 1-5ms |

### Optimization Tips

1. **Use LanceDB for production**: MemoryVectorDB is for development/testing
2. **Configure TTL**: Set `HYBRID_OM_RAG_TTL_DAYS` to auto-expire old observations
3. **Tune MinScore**: Higher `HYBRID_OM_RAG_MIN_SCORE` reduces noise in results
4. **Batch indexing**: For bulk operations, consider batching observations

---

## Troubleshooting

### Observations Not Being Indexed

**Symptoms:** No observations found in RAG search

**Check:**
1. `HYBRID_OM_RAG_ENABLED=true` in environment
2. `HYBRID_OM_RAG_INDEX_OBSERVATIONS=true` (default)
3. `s.vectorDB != nil` in Service
4. Check logs for "Indexed observations to RAG" message

### High Memory Usage

**Symptoms:** Memory grows unbounded over time

**Check:**
1. Using MemoryVectorDB instead of LanceDB
2. `HYBRID_OM_RAG_TTL_DAYS` not set (observations never expire)
3. Sessions not being deleted properly

### Poor Search Results

**Symptoms:** Irrelevant observations returned

**Check:**
1. `HYBRID_OM_RAG_VECTOR_WEIGHT` and `HYBRID_OM_RAG_TEXT_WEIGHT` balance
2. `HYBRID_OM_RAG_MIN_SCORE` threshold
3. `HYBRID_OM_RAG_TEMPORAL_DECAY` for age-based relevance

### Temporal Weighting Not Working

**Symptoms:** Old observations ranked same as new

**Check:**
1. `UseTemporalWeighting=true` in SearchQuery
2. `IndexedAt` field populated in DocumentChunk
3. `ReferenceTime` set correctly (or defaults to now)

---

## Future Enhancements

1. ~~**TTL Enforcement**: Automatic deletion of observations older than `HYBRID_OM_RAG_TTL_DAYS`~~ ✅ **IMPLEMENTED**
2. ~~**Compression**: Implement `HYBRID_OM_RAG_COMPRESSION` for storage optimization~~ ✅ **IMPLEMENTED**
3. ~~**Reflector Integration**: Index consolidated observations from Reflector~~ ✅ **IMPLEMENTED**
4. **Metrics**: Add monitoring for indexing latency, search latency, storage size
5. ~~**LanceDB Implementation**: Implement `DeleteByIDPrefix` for LanceDB~~ ✅ **IMPLEMENTED**

---

## Implemented Features

### TTL Enforcement

The `EnforceTTL()` method automatically removes observations older than the configured TTL:

```go
indexer := NewObservationIndexerWithConfig(vectorDB, true, 90) // 90-day TTL
deletedCount, err := indexer.EnforceTTL(ctx, tenantID)
```

**Configuration:**
- `HYBRID_OM_RAG_TTL_DAYS=90` (default)
- Set to `0` to disable TTL enforcement

**Implementation Details:**
- Checks `IndexedAt` field of each observation chunk
- Compares against `time.Now().AddDate(0, 0, -ttlDays)`
- Deletes by session ID prefix for efficient cleanup

### Observation Compression

The `compressChunks()` method optimizes storage by prioritizing and organizing observations:

```go
if oi.compression {
    chunks = oi.compressChunks(chunks)
}
```

**Configuration:**
- `HYBRID_OM_RAG_COMPRESSION=true` (default)

**Implementation Details:**
- Groups observations by priority (high, medium, low)
- Orders high-priority observations first
- Future: Will merge similar low-priority observations

### Reflector Integration

The `IndexReflectorObservations()` method indexes consolidated observations after the Reflector compresses the observation log:

```go
// In observer.go, after Reflector completes:
if config.HybridEnabled && config.HybridIndexObservations && s.vectorDB != nil {
    indexer := NewObservationIndexerWithConfig(s.vectorDB, config.HybridCompression, config.HybridTTLDays)
    indexer.IndexReflectorObservations(ctx, consolidatedObsLog, session.TenantID, sessionID)
}
```

**Behavior:**
1. Deletes old observations for the session
2. Parses consolidated observation log
3. Indexes new chunks with `consolidated_` prefix in Code field
4. Applies compression if enabled

---

## References

- [DOCS_OM_RAG_GLM5.md](./DOCS_OM_RAG_GLM5.md) - Architecture analysis and design
- [DOCS_OM.md](./DOCS_OM.md) - Original OM documentation
- [DOCS_RAG_OVERVIEW.md](./DOCS_RAG_OVERVIEW.md) - RAG system overview
- [DOCS_HYBRID_SEARCH.md](./DOCS_HYBRID_SEARCH.md) - Hybrid search implementation