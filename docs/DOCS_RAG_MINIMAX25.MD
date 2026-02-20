# RAG Pipeline Drawbacks and Limitations Analysis

**Author:** MiniMax-M2.5 Analysis Engine  
**Date:** 2026-02-20  
**Status:** Comprehensive Technical Assessment  

---

## Executive Summary

This document provides a comprehensive analysis of the bchat RAG (Retrieval-Augmented Generation) pipeline's drawbacks, limitations, challenges, and potential edge cases. Based on extensive review of the codebase and documentation, this analysis identifies critical issues that may impact system reliability, performance, and user experience in production environments.

The RAG pipeline, while providing significant benefits for handling large knowledge bases, has several architectural and implementation limitations that need attention for production deployment.

### Key Findings Summary

| Category | Issue | Severity | Impact | Code Location |
|----------|-------|----------|--------|---------------|
| **Architecture** | Token limit constraints | Critical | High | [`chunker.go:67-71`](server/router/api/v1/agent/chunker.go:67) |
| **Performance** | Embedding provider reliability | High | Medium | [`embedding.go:276-300`](server/router/api/v1/agent/embedding.go:276) |
| **Scalability** | Vector database memory limits | High | Medium | [`vectordb.go:197-210`](server/router/api/v1/agent/vectordb.go:197) |
| **Data Quality** | Hybrid search score normalization | Medium | Medium | [`vectordb.go:729-732`](server/router/api/v1/agent/vectordb.go:729) |
| **Content Processing** | Chunking strategy limitations | Medium | Medium | [`chunker.go:127-261`](server/router/api/v1/agent/chunker.go:127) |
| **Temporal** | Fixed decay parameters | Medium | Low | [`vectordb.go:611-640`](server/router/api/v1/agent/vectordb.go:611) |
| **Memory** | No eviction policy | Medium | Medium | [`vectordb.go:197-210`](server/router/api/v1/agent/vectordb.go:197) |
| **Error Handling** | Limited retry logic | Low | Medium | [`embedding.go:282-284`](server/router/api/v1/agent/embedding.go:282) |

---

## 1. Token Limit Constraints and Chunking Issues

### 1.1 The 30K Token Threshold Problem

**Code Location:** [`chunker.go:67-71`](server/router/api/v1/agent/chunker.go:67)

```go
const (
    DefaultTokenThreshold = 30000 // Threshold for switching to RAG mode
    MinChunkTokens        = 30    // Minimum tokens per chunk
    MaxChunkTokens        = 150   // Default max tokens (for local)
    ChunkOverlapTokens    = 50    // Overlap between chunks for context continuity
)
```

**Critical Issues:**

1. **Arbitrary Threshold**: The 30,000 token threshold is hardcoded and not configurable at runtime
2. **No Graceful Degradation**: Content just above 30K tokens forces RAG mode, potentially degrading quality
3. **Model Dependency**: Different LLMs have different context windows (GPT-4o: 128K, Claude 3.5: 200K)

**Edge Cases:**

- **Edge Case 1**: Content at 29,999 tokens uses long context mode, but adding 2 more tokens switches to RAG mode
- **Edge Case 2**: Multi-tenant scenarios where some tenants have small KBs and others have large KBs
- **Edge Case 3**: Content that grows over time (e.g., accumulating FAQs) may suddenly switch modes

**Potential Problems:**

```go
// chunker.go:112-115
func ShouldUseRAG(kbContent, policyContent string) bool {
    totalTokens := EstimateTokens(kbContent) + EstimateTokens(policyContent)
    return totalTokens >= DefaultTokenThreshold
}
```

The binary decision doesn't account for:
- Content complexity
- Query patterns
- Model capabilities
- User preferences

### 1.2 Token Estimation Inaccuracies

**Code Location:** [`chunker.go:102-109`](server/router/api/v1/agent/chunker.go:102)

```go
// EstimateTokens estimates the token count for a given text.
// Note: This is approximate. Actual tokenization varies by model:
// - GPT-style: ~4 chars/token
// - Sentence-transformers: ~1.9 chars/token (subword)
// We use /4 as a baseline; chunk limits are set conservatively to compensate.
func EstimateTokens(content string) int {
    return len(content) / 4
}
```

**Issues:**

- **Language Dependency**: English-based estimate doesn't work for other languages (Chinese, Japanese, etc.)
- **Model Variation**: Different models have different tokenization schemes
- **No Context Consideration**: Doesn't account for system prompt overhead
- **No Dynamic Adjustment**: Fixed ratio regardless of content type

**Edge Cases:**

- **Edge Case 1**: Chinese content has ~1.5 chars/token, causing underestimation
- **Edge Case 2**: Code content has different token density than prose
- **Edge Case 3**: Mixed language content has unpredictable token counts

### 1.3 Chunking Strategy Limitations

**Code Location:** [`chunker.go:127-261`](server/router/api/v1/agent/chunker.go:127)

```go
func (c *Chunker) ChunkMarkdownContent(
    content string,
    tenantID int32,
    audience string,
    fileType string, // "kb" or "policy"
    sourceVersion int32,
    maxTokens int, // Use GetMaxChunkTokens(embeddingProvider) for this value
) []DocumentChunk {
    // ... implementation
}
```

**Problems Identified:**

- **No Semantic Understanding**: Relies purely on markdown structure (H2/H3 headers)
- **Fixed Size Limits**: Provider-specific limits may not be optimal for all content
- **Context Loss**: 50 token overlap may not preserve sufficient context
- **No Content-Type Awareness**: Treats all content the same regardless of type

**Edge Cases:**

- **Edge Case 1**: Content without headers falls back to paragraph splitting, which may create poor chunks
- **Edge Case 2**: Very long paragraphs (> maxTokens) are split mid-sentence
- **Edge Case 3**: Tables and code blocks are split incorrectly

### 1.4 Hard Limit Splitting Issues

**Code Location:** [`chunker.go`](server/router/api/v1/agent/chunker.go)

The `splitByHardLimit` fallback was implemented to handle massive files (e.g., minified JS), but introduces new issues:

- **Sentence Boundary Loss**: Forcibly chopping content without regard to sentence boundaries
- **Semantic Coherence**: May split related content across chunks
- **No Content Awareness**: Treats all content equally regardless of type

---

## 2. Embedding Provider Reliability and Cost

### 2.1 Provider Dependency Issues

**Code Location:** [`embedding.go:102-115`](server/router/api/v1/agent/embedding.go:102)

```go
func NewEmbeddingService(config *EmbeddingConfig) (EmbeddingService, error) {
    switch config.Provider {
    case "openrouter", "openai":
        return NewOpenRouterEmbedding(config)
    case "mock":
        return NewMockEmbedding(config), nil
    case "local":
        return NewLocalEmbedding(config)
    default:
        // Default to openrouter
        return NewOpenRouterEmbedding(config)
    }
}
```

**Critical Issues:**

1. **Mock Provider Limitations**: Random vectors provide no semantic similarity
2. **Local Provider Complexity**: Requires Python environment and additional process
3. **OpenRouter Dependency**: External API with rate limits and costs
4. **No Fallback Mechanism**: System fails if primary provider unavailable

**Edge Cases:**

- **Edge Case 1**: OpenRouter API timeout during bulk indexing
- **Edge Case 2**: Local embedding server crash mid-operation
- **Edge Case 3**: Mock embeddings used in production by mistake

### 2.2 Retry Logic Limitations

**Code Location:** [`embedding.go:276-300`](server/router/api/v1/agent/embedding.go:276)

```go
func (e *OpenRouterEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    if len(texts) == 0 {
        return [][]float32{}, nil
    }

    var lastErr error
    maxRetries := 10
    baseBackoff := 2 * time.Second
    maxBackoff := 30 * time.Second

    for attempt := 0; attempt < maxRetries; attempt++ {
        // ... retry logic
    }
}
```

**Issues:**

- **Fixed Retry Count**: 10 retries may not be enough for transient failures
- **No Circuit Breaker**: Continues retrying even when service is down
- **No Error Classification**: Retries all errors equally
- **No Timeout Adjustment**: Fixed backoff regardless of error type

**Edge Cases:**

- **Edge Case 1**: Rate limiting (429) vs server error (500) should have different retry strategies
- **Edge Case 2**: Authentication errors (401) should not be retried
- **Edge Case 3**: Context cancellation during retry loop

### 2.3 Dimension Mismatch Issues

**Code Location:** [`embedding.go:78-100`](server/router/api/v1/agent/embedding.go:78)

```go
func getOpenRouterDimension(modelName string) int {
    switch modelName {
    // OpenAI models
    case "openai/text-embedding-3-large":
        return 3072
    case "openai/text-embedding-ada-002", "openai/text-embedding-3-small":
        return 1536
    // Qwen models
    case "qwen/qwen3-embedding-8b":
        return 4096
    // ... more cases
    default:
        return 1536 // Default to OpenAI dimension
    }
}
```

**Critical Issues:**

- **Index Incompatibility**: Changing dimensions requires complete re-indexing
- **No Versioning**: Cannot maintain multiple embedding versions
- **No Migration Support**: No automated migration between dimensions
- **Storage Inefficiency**: Higher dimensions waste storage for simple use cases

**Edge Cases:**

- **Edge Case 1**: User switches from OpenAI (1536) to Qwen (4096) without reindexing
- **Edge Case 2**: Unknown model defaults to 1536, which may be wrong
- **Edge Case 3**: Custom models not in the switch statement

### 2.4 Batch Size and Payload Issues

**Code Location:** [`DOCS_RAG_FIX.MD`](docs/DOCS_RAG_FIX.MD)

The RAG embedding reliability fix identified several critical issues:

- **Batch Size**: Originally 32, reduced to 10, then 5 due to timeout issues
- **Chunk Size**: Reduced from 7000 to 2000 to 1000 tokens due to provider limits
- **Payload Size**: Large batches with large chunks created payloads exceeding API limits

**Issues:**

- **Provider Instability**: Qwen model failed consistently with "No successful provider responses"
- **Timeout Issues**: Batch processing hung for >3 minutes or failed entirely
- **Configuration Complexity**: Multiple interrelated parameters need careful tuning

---

## 3. Vector Database Scalability and Performance

### 3.1 Memory Storage Limitations

**Code Location:** [`vectordb.go:197-210`](server/router/api/v1/agent/vectordb.go:197)

```go
// MemoryVectorDB is an in-memory implementation of VectorDB for testing.
type MemoryVectorDB struct {
    chunks   map[string]DocumentChunk // key: chunk ID
    embedSvc EmbeddingService
    mu       sync.RWMutex
}
```

**Issues:**

- **No Persistence**: Data lost on restart
- **Linear Search**: O(n) search complexity with cosine similarity
- **Memory Limits**: Practical limit ~10,000 chunks before performance degrades
- **No Indexing**: No performance optimization for large datasets

**Edge Cases:**

- **Edge Case 1**: Server restart during active chat session loses all context
- **Edge Case 2**: Memory exhaustion with large number of tenants
- **Edge Case 3**: Concurrent read/write operations causing lock contention

### 3.2 Search Performance Issues

**Code Location:** [`vectordb.go:310-469`](server/router/api/v1/agent/vectordb.go:310)

```go
func (db *MemoryVectorDB) Search(ctx context.Context, query SearchQuery) (*SearchResult, error) {
    // ... implementation
    
    // Build BM25 index if hybrid search is enabled
    var bm25Scorer *BM25Scorer
    if query.UseHybridSearch && query.QueryText != "" {
        bm25Scorer = NewBM25Scorer()
        for id, chunk := range db.chunks {
            // Only index chunks that pass filters
            // ... filter logic
            bm25Scorer.AddDocument(id, chunk.Title+" "+chunk.Content)
        }
    }
    
    // ... scoring and ranking
}
```

**Performance Bottlenecks:**

- **Embedding Latency**: External embedding calls add 50-200ms
- **No Caching**: Repeated queries re-embed and re-search
- **Linear Filtering**: Tenant/audience filtering is O(n)
- **No Pagination**: All results loaded into memory

**Edge Cases:**

- **Edge Case 1**: Same query repeated multiple times within short time
- **Edge Case 2**: Large result sets consuming excessive memory
- **Edge Case 3**: Slow embedding calls blocking other operations

### 3.3 LanceDB Storage Issues

**Code Location:** [`vectordb_lance.go:43-116`](server/router/api/v1/agent/vectordb_lance.go:43)

```go
func newLanceVectorDB(config *VectorDBConfig, embedSvc EmbeddingService) (VectorDB, error) {
    // ... connection setup
    
    // Compute table name from embedding dimension
    tableName := getTableNameForDimension(embedSvc.Dimension())
    
    // Migrate legacy table if it exists
    if err := db.migrateLegacyTable(ctx); err != nil {
        slog.Warn("Legacy table migration warning", "error", err)
    }
}
```

**Issues:**

- **CGO Dependency**: Requires native libraries for LanceDB
- **Rebuild Required**: Must rebuild index after dimension changes
- **Single Server**: No clustering support for horizontal scaling
- **File Corruption**: No corruption detection or recovery

**Edge Cases:**

- **Edge Case 1**: LanceDB native library version mismatch
- **Edge Case 2**: S3 connectivity issues during table operations
- **Edge Case 3**: Concurrent table creation from multiple instances

---

## 4. Hybrid Search Effectiveness

### 4.1 Score Normalization Issues

**Code Location:** [`vectordb.go:729-732`](server/router/api/v1/agent/vectordb.go:729)

```go
// Normalize score to 0-1 range using sigmoid-like function
// This ensures BM25 scores are comparable to cosine similarity scores
normalized := score / (score + 1)
return normalized
```

**Issues:**

- **No Score Calibration**: Scores not calibrated to same scale
- **No Confidence Metrics**: No indication of result quality
- **No Diversity**: No diversity promotion in results
- **No Personalization**: No user-specific scoring

**Edge Cases:**

- **Edge Case 1**: BM25 scores can be much higher than vector scores for exact matches
- **Edge Case 2**: Short queries favor BM25, long queries favor vector
- **Edge Case 3**: Rare terms get high BM25 scores but may be irrelevant

### 4.2 Fixed Weight Parameters

**Code Location:** [`vectordb.go:339-345`](server/router/api/v1/agent/vectordb.go:339)

```go
// Determine weights for hybrid search
vectorWeight := query.VectorWeight
textWeight := query.TextWeight
if query.UseHybridSearch && vectorWeight == 0 && textWeight == 0 {
    vectorWeight = 0.7
    textWeight = 0.3
}
```

**Issues:**

- **Fixed Weights**: No dynamic weight adjustment based on query type
- **No Intent-Aware Search**: Same weights for all intents
- **No Fallback Strategy**: No graceful degradation if one search fails
- **No Learning**: Weights don't adapt based on user feedback

**Edge Cases:**

- **Edge Case 1**: Service code queries need exact match (higher BM25 weight)
- **Edge Case 2**: Conceptual queries need semantic match (higher vector weight)
- **Edge Case 3**: Mixed queries need balanced weights

### 4.3 BM25 Implementation Limitations

**Code Location:** [`vectordb.go:646-665`](server/router/api/v1/agent/vectordb.go:646)

```go
type BM25Scorer struct {
    k1        float64             // Term frequency saturation parameter (default: 1.2)
    b         float64             // Length normalization parameter (default: 0.75)
    docs      map[string][]string // Document ID -> tokenized words
    docFreq   map[string]int      // Term -> number of documents containing term
    avgLen    float64             // Average document length
    totalDocs int                 // Total number of documents
}
```

**Issues:**

- **No Parameter Tuning**: Fixed parameters regardless of content
- **No Language Support**: English-centric tokenization
- **No Stop Words**: No stop word filtering
- **No Stemming**: No linguistic processing

**Edge Cases:**

- **Edge Case 1**: Technical terms with hyphens are split incorrectly
- **Edge Case 2**: Proper nouns not recognized
- **Edge Case 3**: Multi-word phrases not handled

---

## 5. Structured vs Unstructured Content Handling

### 5.1 Content Classification Issues

**Code Location:** [`service.go:1350-1364`](server/router/api/v1/agent/service.go:1350)

```go
// Determine which generation method to use based on tenant's retrieval mode:
// - "rag" mode: Use RAG pipeline with retrieved chunks (for large KBs)
// - "long_context" mode (default): Full KB in system prompt (for small/medium KBs)
// - For unstructured content (no structured KB/Policy), always use RAG mode
useRAG := false
forceRAG := !config.HasStructuredContent && s.UseRAGPipeline()
if forceRAG {
    // No structured content - must use RAG mode since there's nothing to build long_context from
    useRAG = true
    slog.Debug("forcing RAG mode for unstructured content",
        "tenant_slug", config.TenantSlug,
        "session_id", session.ID)
} else if s.UseRAGPipeline() {
    // Check tenant-specific retrieval mode
    tenantConfig, _ := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &config.TenantID})
    if tenantConfig != nil && tenantConfig.RetrievalMode == "rag" {
        useRAG = true
    }
}
```

**Issues:**

- **Binary Classification**: Content is either structured or unstructured, no middle ground
- **Forced RAG**: Unstructured content always uses RAG, even for small content
- **No Hybrid Approach**: No combination of approaches
- **No User Control**: No way to override strategy

**Edge Cases:**

- **Edge Case 1**: Content with partial annotations treated as unstructured
- **Edge Case 2**: Small unstructured content forced into RAG mode unnecessarily
- **Edge Case 3**: Large structured content could benefit from RAG but uses long context

### 5.2 RAG Fallback Issues

**Code Location:** [`service.go:2131-2137`](server/router/api/v1/agent/service.go:2131)

```go
if err != nil {
    slog.Warn("RAG retrieval failed, falling back to full context",
        "error", err,
        "session_id", session.ID)
    // Fall back to regular generation
    return s.generateResponse(ctx, config, session, classification, decision)
}
```

**Issues:**

- **Silent Fallback**: User not informed that fallback occurred
- **No Quality Indication**: No way to know if response quality degraded
- **No Retry**: Single failure triggers fallback
- **No Logging**: Limited visibility into fallback frequency

**Edge Cases:**

- **Edge Case 1**: RAG retrieval fails but long context would also fail
- **Edge Case 2**: Intermittent failures causing inconsistent behavior
- **Edge Case 3**: Fallback to long context for unstructured content fails

---

## 6. Temporal Weighting and Observation Indexing

### 6.1 Temporal Weighting Limitations

**Code Location:** [`vectordb.go:611-640`](server/router/api/v1/agent/vectordb.go:611)

```go
func calculateTemporalWeight(contentTime time.Time, referenceTime time.Time, decayFactor float64) float64 {
    if contentTime.IsZero() {
        return 0.5 // Default for unknown times
    }

    // Use current time if reference time is not set
    if referenceTime.IsZero() {
        referenceTime = time.Now()
    }

    // Calculate age in days
    age := referenceTime.Sub(contentTime).Hours() / 24

    // Apply decay
    switch {
    case age < 1:
        return 1.0 // Full weight for today
    case age < 7:
        // Linear decay from 1.0 to 0.3 over a week
        return 1.0 - (age * 0.1)
    case age < 30:
        // Slower decay from 0.3 to 0.1 over a month
        return 0.3 - ((age - 7) * 0.01)
    default:
        // Minimum weight for old content
        return 0.1
    }
}
```

**Issues:**

- **Fixed Decay Parameters**: Hardcoded decay regardless of content type
- **No Seasonality**: No consideration of seasonal patterns
- **No Event Awareness**: No recognition of significant events
- **No User Patterns**: No learning from user behavior

**Edge Cases:**

- **Edge Case 1**: Timezone issues causing incorrect age calculation
- **Edge Case 2**: Backdated content treated as old
- **Edge Case 3**: Seasonal content incorrectly weighted

### 6.2 Observation Indexing Issues

The observation indexing system has several limitations:

- **No Batch Processing**: Single observation indexing
- **No Compression**: Observations stored uncompressed
- **No Deduplication**: Duplicate observations indexed
- **No Quality Control**: No filtering of low-quality observations

---

## 7. Memory Management and Storage Optimization

### 7.1 Memory Usage Issues

**Code Location:** [`vectordb.go`](server/router/api/v1/agent/vectordb.go)

```go
// DocumentChunk memory usage
Embedding: []float32 (384 * 4 bytes = 1.5KB per chunk)
Content: string (variable, often 1-10KB)
Metadata: additional overhead
```

**Memory Problems:**

- **No Memory Limits**: No protection against OOM
- **No Compression**: Data stored uncompressed
- **No Eviction**: No LRU or TTL for old data
- **No Monitoring**: No memory usage tracking

### 7.2 Storage Optimization Issues

- **Content Duplication**: Full content stored with embeddings
- **No Compression**: Text stored uncompressed
- **No Deduplication**: Duplicate content across tenants
- **Inefficient Indexing**: No optimization for sparse vs dense vectors

---

## 8. Error Handling and Reliability

### 8.1 Error Handling Gaps

**Issues:**

- **No Graceful Degradation**: System fails completely on certain errors
- **Limited Retry Logic**: Fixed retry count doesn't adapt to error type
- **No Circuit Breaker**: Continues attempting failed operations
- **Poor Error Messages**: Users don't understand what went wrong

### 8.2 Logging and Monitoring Gaps

- **No Fallback Visibility**: Limited logging when fallback occurs
- **No Performance Metrics**: No tracking of retrieval latency
- **No Quality Metrics**: No way to measure retrieval effectiveness
- **No Alerting**: No notifications for RAG failures

---

## 9. Multi-Tenant and Scalability Issues

### 9.1 Tenant Isolation Problems

- **No Resource Limits**: Tenants can consume unlimited resources
- **No Performance Isolation**: One tenant's queries affect others
- **No Data Partitioning**: All data in single index
- **No Quota Management**: No usage limits per tenant

### 9.2 Index Management Issues

- **No Incremental Updates**: Full reindex on every change
- **No Background Processing**: Reindexing blocks operations
- **No Progress Tracking**: No visibility into reindex status
- **No Rollback**: Cannot revert failed reindex

---

## 10. Security and Compliance

### 10.1 Security Gaps

- **No Input Validation**: No input sanitization for user queries
- **No Authentication**: No user authentication for RAG access
- **No Authorization**: No access control for content
- **No Audit Trail**: No logging of who accessed what content

### 10.2 Data Privacy Concerns

- **No Data Isolation**: Tenant data potentially mixed
- **No Encryption**: Content stored unencrypted in vector DB
- **No Data Retention**: No automatic cleanup of old data
- **No GDPR Compliance**: No right to deletion support

---

## Recommendations Summary

### Critical Priority

1. **Token Threshold**: Make configurable via environment variable
2. **Token Estimation**: Implement proper tokenizer or use API-based counting
3. **Error Handling**: Add circuit breaker and error classification
4. **Memory Limits**: Implement memory protection mechanisms

### High Priority

1. **Hybrid Search**: Implement intent-aware weight adjustment
2. **Caching**: Add query result caching
3. **Pagination**: Implement result pagination
4. **Fallback Visibility**: Log and report fallback occurrences

### Medium Priority

1. **Multi-tenant Isolation**: Add tenant resource limits
2. **BM25 Improvements**: Add stop word filtering and stemming
3. **Temporal Weighting**: Make decay parameters configurable
4. **Compression**: Implement content compression

### Lower Priority

1. **Personalization**: Add user-specific scoring
2. **Learning**: Implement feedback-based weight adjustment
3. **Audit Trail**: Add comprehensive logging
4. **Alerting**: Add failure notifications

---

## Related Documents

- [DOCS_RAG_OVERVIEW.MD](./DOCS_RAG_OVERVIEW.MD) - RAG system overview
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG pipeline configuration
- [DOCS_RAG_PROCESS.MD](./DOCS_RAG_PROCESS.MD) - RAG processing details
- [DOCS_CHUNKING.MD](./DOCS_CHUNKING.MD) - Chunking strategy
- [DOCS_HYBRID_SEARCH.MD](./DOCS_HYBRID_SEARCH.MD) - Hybrid search implementation
- [DOCS_OM_RAG_IMP.MD](./DOCS_OM_RAG_IMP.MD) - OM + RAG implementation
- [DOCS_RAG_ARCEE.MD](./DOCS_RAG_ARCEE.MD) - Arcee analysis
- [DOCS_RAG_GLM5.MD](./DOCS_RAG_GLM5.MD) - GLM-5 analysis

---

## Conclusion

The bchat RAG pipeline provides a solid foundation for retrieval-augmented generation but requires significant improvements to be production-ready. The identified issues span architecture, performance, reliability, security, and operational concerns. Addressing these issues systematically will result in a robust, scalable, and maintainable RAG system.

The most critical issues that should be addressed immediately are:

1. **Token threshold configurability** - Current hardcoded value prevents adaptation to different use cases
2. **Error handling and fallback visibility** - Silent failures reduce system reliability
3. **Memory management** - No protection against memory exhaustion
4. **Embedding provider reliability** - External dependencies need better resilience

By prioritizing these critical issues, the RAG pipeline can be significantly improved for production deployment.
