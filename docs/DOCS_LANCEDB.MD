# LanceDB RAG Implementation Plan

## Executive Summary

This document outlines a comprehensive plan to simplify the chat agent codebase by implementing a Retrieval-Augmented Generation (RAG) pipeline using [LanceDB-Go](https://github.com/lancedb/lancedb-go). The goal is to improve response quality, reduce hallucinations, and dramatically decrease system prompt size through intelligent document retrieval.

---

## Confirmed Design Decisions Summary

| Decision | Choice |
|----------|--------|
| **Embedding Model** | Local (testing/QA) + OpenRouter API (prod) |
| **Index Storage** | Local filesystem (testing) + Tigrisdata S3 (prod on fly.io) |
| **Re-indexing** | On every file upload |
| **Feature Flag** | Global environment variable |
| **Compliance Checker** | REMOVE regex compliance entirely |
| **LLM Verifier** | Keep as OPTIONAL safety net (disabled by default) |
| **SCRIPT.MD** | Full content as system prompt, configurable per tenant |

---

## Table of Contents

1. [Current Architecture Analysis](#1-current-architecture-analysis)
2. [Problems with Current Approach](#2-problems-with-current-approach)
3. [Proposed RAG Architecture](#3-proposed-rag-architecture)
4. [LanceDB Integration Details](#4-lancedb-integration-details)
5. [Document Indexing Strategy](#5-document-indexing-strategy)
6. [Query Pipeline Design](#6-query-pipeline-design)
7. [Simplified System Prompt Structure](#7-simplified-system-prompt-structure)
8. [Implementation Phases](#8-implementation-phases)
9. [Migration Strategy](#9-migration-strategy)
10. [Performance Considerations](#10-performance-considerations)
11. [Risk Assessment](#11-risk-assessment)
12. [File Structure Changes](#12-file-structure-changes)

---

## 1. Current Architecture Analysis

### 1.1 How MD Files Are Currently Used

**KB.MD** (Knowledge Base):
- Parsed using HTML comment annotations (`<!-- @service: code -->`)
- Contains: Services, Exclusions, Coverage Areas, FAQs, Safety Protocols
- **Problem**: Entire KB content concatenated into system prompt (~2000-5000 tokens)

**POLICY.MD** (Agent Policy):
- Contains: Identity, Intents, Rules, Audience Thresholds
- Parsed into structured data for rule evaluation
- **Problem**: All rules included regardless of conversation context

**SCRIPT.MD** (Conversation Flow):
- Contains: Conversation flow structure with phases
- Summary generated and included in prompt
- **Current Usage**: Works well as system prompt guidance

### 1.2 Current System Prompt Structure (~1300 lines of code)

```
SECTION 0: Customer Info Already Provided (context retention)
SECTION 1: Critical Constraints (highest priority)
SECTION 2: Identity (from POLICY.MD)
SECTION 3: Services We Offer (ALL services from KB.MD)       ← PROBLEM
SECTION 4: Services We Don't Provide (ALL exclusions)        ← PROBLEM
SECTION 5: Conversation Flow (from SCRIPT.MD)
SECTION 6: Policies & Rules (evaluated rules only)
SECTION 7: Learned Behaviors
SECTION 8: FAQs (ALL FAQs from KB.MD)                        ← PROBLEM
SECTION 9: Authorized Contact Info
SECTION 10: Response Format
SECTION 11: Critical Reminder
```

### 1.3 Current Chat Flow

```
User Message
    ↓
Load Config → [Cache or Database]
    ↓
Classify Intent → [LLM Call #1]
    ↓
Build Full System Prompt → [All KB content included]
    ↓
Generate Response → [LLM Call #2]
    ↓
Sanitize & Verify → [LLM Call #3 + Regex]
    ↓
Return Response
```

---

## 2. Problems with Current Approach

### 2.1 Token Inefficiency

| Content Type | Typical Size | Relevance per Query |
|-------------|--------------|---------------------|
| All Services | 500-1500 tokens | 10-20% relevant |
| All FAQs | 1000-3000 tokens | 5-15% relevant |
| All Exclusions | 200-500 tokens | 0-10% relevant |
| All Coverage | 300-800 tokens | 5-20% relevant |
| **Total Waste** | **~3000-5000 tokens** | **~80% irrelevant** |

### 2.2 Hallucination Risk

- LLM sees all services → may mention irrelevant ones
- LLM sees all FAQs → may mix up answers
- No semantic grounding → responses based on pattern matching, not retrieval

### 2.3 Scaling Issues

- Prompt size grows linearly with KB size
- 50 services + 100 FAQs = massive prompt
- Context window limits become a concern

### 2.4 Code Complexity

| Component | Lines of Code | Purpose |
|-----------|---------------|---------|
| `buildSystemPrompt()` | ~400 lines | Concatenate everything |
| `parser.go` | ~600 lines | Parse all annotation types |
| `compliance.go` | ~300 lines | Regex verification |
| `verifier.go` | ~200 lines | LLM verification |
| `sanitizer.go` | ~150 lines | Remove hallucinations |
| **Total** | **~1650 lines** | **Workarounds for poor grounding** |

---

## 3. Proposed RAG Architecture

### 3.1 New Architecture Overview

```
User Message
    ↓
Embed Query → [Embedding Model]
    ↓
Hybrid Search → [LanceDB: Vector + Metadata Filter]
    ↓
Retrieve Top-K Documents → [Services, FAQs, Coverage]
    ↓
Build Targeted Prompt → [Only relevant content]
    ↓
Generate Response → [LLM with grounded context]
    ↓
Return Response
```

### 3.2 Key Changes

| Aspect | Before | After |
|--------|--------|-------|
| KB Content | All included | Top-K retrieved |
| System Prompt | 4000-8000 tokens | 1500-2500 tokens |
| Grounding | None | Source citations |
| Verification | 3-layer stack | Built-in retrieval confidence |
| LLM Calls | 3 per message | 2 per message |

### 3.3 Document Flow

```
┌─────────────────────────────────────────────────────────────┐
│                     INDEXING PIPELINE                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  KB.MD ───→ Parse Sections ───→ Chunk ───→ Embed ───→ Index│
│                                                             │
│  POLICY.MD → Parse Rules ─────→ Chunk ───→ Embed ───→ Index│
│                                                             │
│  SCRIPT.MD → Parse Phases ────→ Store as System Prompt      │
│                                                             │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                     QUERY PIPELINE                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  User Query ───→ Embed ───→ Hybrid Search ───→ Top-K Docs   │
│                                     │                       │
│                                     ↓                       │
│                              Build Context                  │
│                                     │                       │
│                                     ↓                       │
│                         [System Prompt + Retrieved Docs]    │
│                                     │                       │
│                                     ↓                       │
│                            LLM Generation                   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## 4. LanceDB Integration Details

### 4.1 Installation Requirements

```bash
# Download native libraries
curl -sSL https://raw.githubusercontent.com/lancedb/lancedb-go/main/scripts/download-artifacts.sh | bash

# Add to go.mod
go get github.com/lancedb/lancedb-go

# Set CGO flags (add to Taskfile or build script)
export CGO_CFLAGS="-I$(pwd)/include"
export CGO_LDFLAGS="$(pwd)/lib/$(go env GOOS)_$(go env GOARCH)/liblancedb_go.a ..."
```

### 4.2 Database Schema Design

```go
// Vector dimension: 384 (sentence-transformers/all-MiniLM-L6-v2)
// or 256 (smaller models for faster inference)

schema := []arrow.Field{
    // Identity
    {Name: "id", Type: arrow.BinaryTypes.String},
    {Name: "tenant_id", Type: arrow.PrimitiveTypes.Int32},
    {Name: "audience_type", Type: arrow.BinaryTypes.String},

    // Content
    {Name: "content", Type: arrow.BinaryTypes.String},
    {Name: "content_type", Type: arrow.BinaryTypes.String}, // service, faq, exclusion, coverage, rule, safety
    {Name: "title", Type: arrow.BinaryTypes.String},

    // Metadata
    {Name: "code", Type: arrow.BinaryTypes.String},         // service code, rule code
    {Name: "is_emergency", Type: arrow.FixedWidthTypes.Boolean},
    {Name: "is_active", Type: arrow.FixedWidthTypes.Boolean},
    {Name: "priority", Type: arrow.PrimitiveTypes.Int32},   // for ranking

    // Vector
    {Name: "embedding", Type: arrow.FixedSizeListOf(384, arrow.PrimitiveTypes.Float32)},

    // Timestamps
    {Name: "indexed_at", Type: arrow.PrimitiveTypes.Int64},
    {Name: "source_version", Type: arrow.PrimitiveTypes.Int32},
}
```

### 4.3 Index Configuration

```go
// Create indexes for hybrid search
table.CreateIndex(ctx, []string{"embedding"}, contracts.IndexTypeIvfPq)  // Vector search
table.CreateIndex(ctx, []string{"content"}, contracts.IndexTypeFts)       // Full-text search
table.CreateIndex(ctx, []string{"tenant_id"}, contracts.IndexTypeBTree)   // Tenant filtering
table.CreateIndex(ctx, []string{"content_type"}, contracts.IndexTypeBitmap) // Type filtering
table.CreateIndex(ctx, []string{"audience_type"}, contracts.IndexTypeBitmap)
```

### 4.4 Connection Management

```go
// server/router/api/v1/agent/vectordb.go

type VectorDB struct {
    conn   contracts.IConnection
    table  contracts.ITable
    embed  EmbeddingService
    config *VectorDBConfig
}

type VectorDBConfig struct {
    // Storage configuration
    StorageProvider string // "local" or "s3"
    LocalPath       string // For local: "build/data/lancedb/"

    // S3/Tigrisdata configuration (for production)
    S3Endpoint  string // "fly.storage.tigris.dev" for Tigrisdata
    S3Bucket    string
    S3Region    string // "auto" for Tigrisdata
    S3AccessKey string
    S3SecretKey string

    // Embedding configuration
    EmbeddingProvider string // "local" or "openrouter"
    EmbeddingModel    string
    OpenRouterAPIKey  string
}

// NewVectorDBConfigFromEnv loads configuration from environment variables
func NewVectorDBConfigFromEnv() *VectorDBConfig {
    return &VectorDBConfig{
        StorageProvider:   getEnv("LANCEDB_STORAGE_PROVIDER", "local"),
        LocalPath:         getEnv("LANCEDB_LOCAL_PATH", "build/data/lancedb"),
        S3Endpoint:        getEnv("LANCEDB_S3_ENDPOINT", "fly.storage.tigris.dev"),
        S3Bucket:          getEnv("LANCEDB_S3_BUCKET", ""),
        S3Region:          getEnv("LANCEDB_S3_REGION", "auto"),
        S3AccessKey:       getEnv("AWS_ACCESS_KEY_ID", ""),
        S3SecretKey:       getEnv("AWS_SECRET_ACCESS_KEY", ""),
        EmbeddingProvider: getEnv("EMBEDDING_PROVIDER", "local"),
        EmbeddingModel:    getEnv("EMBEDDING_MODEL", "all-MiniLM-L6-v2"),
        OpenRouterAPIKey:  getEnv("OPENROUTER_API_KEY", ""),
    }
}

func NewVectorDB(config *VectorDBConfig) (*VectorDB, error) {
    ctx := context.Background()

    var conn contracts.IConnection
    var err error

    switch config.StorageProvider {
    case "s3":
        conn, err = connectS3(ctx, config)
    default:
        conn, err = connectLocal(ctx, config)
    }

    if err != nil {
        return nil, fmt.Errorf("failed to connect to LanceDB: %w", err)
    }

    // Initialize embedding service
    embed, err := NewEmbeddingService(&EmbeddingConfig{
        Provider:         config.EmbeddingProvider,
        Model:            config.EmbeddingModel,
        OpenRouterAPIKey: config.OpenRouterAPIKey,
        Dimension:        384, // Default for MiniLM
    })
    if err != nil {
        return nil, fmt.Errorf("failed to initialize embedding service: %w", err)
    }

    // Open or create table
    table, err := openOrCreateTable(ctx, conn, embed.Dimension())
    if err != nil {
        return nil, err
    }

    return &VectorDB{
        conn:   conn,
        table:  table,
        embed:  embed,
        config: config,
    }, nil
}

// connectLocal connects to local filesystem storage (Testing/QA)
func connectLocal(ctx context.Context, config *VectorDBConfig) (contracts.IConnection, error) {
    return lancedb.Connect(ctx, config.LocalPath, &contracts.ConnectionOptions{
        StorageOptions: &contracts.StorageOptions{
            LocalConfig: &contracts.LocalConfig{
                CreateDirIfNotExists: ptr(true),
            },
        },
    })
}

// connectS3 connects to S3-compatible storage (Tigrisdata on fly.io)
func connectS3(ctx context.Context, config *VectorDBConfig) (contracts.IConnection, error) {
    if config.S3Bucket == "" {
        return nil, fmt.Errorf("LANCEDB_S3_BUCKET is required for S3 storage")
    }

    s3URI := fmt.Sprintf("s3://%s/lancedb", config.S3Bucket)

    s3Config := &contracts.S3Config{
        Endpoint:        &config.S3Endpoint,
        Region:          &config.S3Region,
        AccessKeyID:     &config.S3AccessKey,
        SecretAccessKey: &config.S3SecretKey,
        ForcePathStyle:  ptr(true), // Required for Tigrisdata
    }

    return lancedb.Connect(ctx, s3URI, &contracts.ConnectionOptions{
        StorageOptions: &contracts.StorageOptions{
            S3Config:  s3Config,
            BlockSize: ptr(5 * 1024 * 1024), // 5MB blocks for S3
        },
    })
}

func openOrCreateTable(ctx context.Context, conn contracts.IConnection, dimension int) (contracts.ITable, error) {
    table, err := conn.OpenTable(ctx, "kb_documents")
    if err != nil {
        // Create table if doesn't exist
        schema := buildSchema(dimension)
        table, err = conn.CreateTable(ctx, "kb_documents", schema)
        if err != nil {
            return nil, fmt.Errorf("failed to create table: %w", err)
        }

        // Create indexes
        if err := createIndexes(ctx, table); err != nil {
            slog.Warn("failed to create indexes", "error", err)
        }
    }
    return table, nil
}

func ptr[T any](v T) *T { return &v }
```

#### 4.4.1 Environment Variables Reference

```bash
# ============================================
# Testing/QA Configuration (Local)
# ============================================
LANCEDB_STORAGE_PROVIDER=local
LANCEDB_LOCAL_PATH=build/data/lancedb
EMBEDDING_PROVIDER=local
EMBEDDING_MODEL=all-MiniLM-L6-v2
RAG_PIPELINE_ENABLED=true

# ============================================
# Production Configuration (Tigrisdata on fly.io)
# ============================================
LANCEDB_STORAGE_PROVIDER=s3
LANCEDB_S3_ENDPOINT=fly.storage.tigris.dev
LANCEDB_S3_BUCKET=your-app-lancedb
LANCEDB_S3_REGION=auto
AWS_ACCESS_KEY_ID=tid_xxxxx
AWS_SECRET_ACCESS_KEY=tsec_xxxxx
EMBEDDING_PROVIDER=openrouter
EMBEDDING_MODEL=openai/text-embedding-3-small
OPENROUTER_API_KEY=sk-or-xxxxx
RAG_PIPELINE_ENABLED=true
LLM_VERIFIER_ENABLED=false
```

#### 4.4.2 Tigrisdata Setup on fly.io

```bash
# Create Tigrisdata bucket on fly.io
fly storage create

# Get credentials (automatically sets secrets)
fly storage dashboard

# Or manually set secrets
fly secrets set AWS_ACCESS_KEY_ID=tid_xxxxx
fly secrets set AWS_SECRET_ACCESS_KEY=tsec_xxxxx
fly secrets set LANCEDB_S3_BUCKET=your-bucket-name
```

---

## 5. Document Indexing Strategy

### 5.1 Chunking Strategy

| Document Type | Chunking Method | Typical Chunk Size |
|--------------|-----------------|-------------------|
| Services | One chunk per service | 100-300 tokens |
| FAQs | One chunk per Q&A pair | 50-200 tokens |
| Exclusions | One chunk per exclusion | 50-100 tokens |
| Coverage | One chunk per area/zone | 50-150 tokens |
| Rules | One chunk per rule | 50-150 tokens |
| Safety | One chunk per protocol | 100-200 tokens |

### 5.2 Metadata Extraction

```go
type DocumentChunk struct {
    ID           string
    TenantID     int32
    AudienceType string
    ContentType  string    // service, faq, exclusion, coverage, rule, safety
    Title        string
    Content      string
    Code         string    // service/rule code
    IsEmergency  bool
    IsActive     bool
    Priority     int32
    Embedding    []float32
    SourceVersion int32
}

func ChunkKBDocument(kb *ParsedKB, tenantID int32, audience string) []DocumentChunk {
    var chunks []DocumentChunk

    // Services
    for _, svc := range kb.Services {
        chunks = append(chunks, DocumentChunk{
            ID:          fmt.Sprintf("%d:%s:service:%s", tenantID, audience, svc.Code),
            TenantID:    tenantID,
            AudienceType: audience,
            ContentType: "service",
            Title:       svc.Name,
            Content:     fmt.Sprintf("%s: %s", svc.Name, svc.Description),
            Code:        svc.Code,
            IsEmergency: svc.IsEmergency,
            IsActive:    svc.IsActive,
            Priority:    svc.Priority,
        })
    }

    // FAQs
    for i, faq := range kb.FAQs {
        chunks = append(chunks, DocumentChunk{
            ID:          fmt.Sprintf("%d:%s:faq:%d", tenantID, audience, i),
            TenantID:    tenantID,
            AudienceType: audience,
            ContentType: "faq",
            Title:       faq.Question,
            Content:     fmt.Sprintf("Q: %s\nA: %s", faq.Question, faq.Answer),
            IsActive:    faq.IsActive,
        })
    }

    // ... similar for exclusions, coverage, rules, safety

    return chunks
}
```

### 5.3 Embedding Generation

```go
// server/router/api/v1/agent/embedding.go

// Embedding service interface - allows different providers
type EmbeddingService interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int
    Provider() string
}

// Factory function based on config
func NewEmbeddingService(config *EmbeddingConfig) (EmbeddingService, error) {
    switch config.Provider {
    case "openrouter":
        return NewOpenRouterEmbedding(config)
    case "local":
        return NewLocalEmbedding(config)
    default:
        return NewLocalEmbedding(config) // Default to local
    }
}

// Configuration
type EmbeddingConfig struct {
    Provider         string // "local" or "openrouter"
    Model            string // Model name/path
    Dimension        int    // Vector dimension (384 for MiniLM)
    OpenRouterAPIKey string // For OpenRouter provider
    BatchSize        int    // Max texts per batch (default: 32)
}
```

#### 5.3.1 Local Embedding (Testing/QA)

```go
// Local embedding using sentence-transformers via ONNX runtime
// Recommended: all-MiniLM-L6-v2 (384 dimensions, fast)

import (
    "github.com/pbarker/go-onnxruntime"
    // Or use a simpler approach with HTTP to a local Python service
)

type LocalEmbedding struct {
    model     *onnxruntime.Session
    tokenizer *Tokenizer
    dimension int
}

func NewLocalEmbedding(config *EmbeddingConfig) (*LocalEmbedding, error) {
    // Option 1: ONNX Runtime (pure Go, complex setup)
    // Option 2: HTTP call to local Python FastAPI service (simpler)

    // For simplicity, recommend Option 2 during testing:
    // Run: uvicorn embedding_server:app --port 8001
    return &LocalEmbedding{
        endpoint:  "http://localhost:8001/embed",
        dimension: 384,
    }, nil
}

func (e *LocalEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    // HTTP POST to local embedding service
    req := EmbedRequest{Texts: texts}
    resp, err := http.Post(e.endpoint, "application/json", toJSON(req))
    if err != nil {
        return nil, fmt.Errorf("local embedding failed: %w", err)
    }

    var result EmbedResponse
    json.NewDecoder(resp.Body).Decode(&result)
    return result.Embeddings, nil
}

func (e *LocalEmbedding) Dimension() int { return e.dimension }
func (e *LocalEmbedding) Provider() string { return "local" }
```

**Local Embedding Service (Python):**
```python
# scripts/embedding_server.py
from fastapi import FastAPI
from sentence_transformers import SentenceTransformer
from pydantic import BaseModel

app = FastAPI()
model = SentenceTransformer('all-MiniLM-L6-v2')

class EmbedRequest(BaseModel):
    texts: list[str]

@app.post("/embed")
def embed(req: EmbedRequest):
    embeddings = model.encode(req.texts).tolist()
    return {"embeddings": embeddings}

# Run: uvicorn embedding_server:app --port 8001
```

#### 5.3.2 OpenRouter Embedding (Production)

```go
// OpenRouter embedding implementation for production
// Uses OpenRouter's embedding endpoints

type OpenRouterEmbedding struct {
    apiKey    string
    model     string
    endpoint  string
    dimension int
    client    *http.Client
}

func NewOpenRouterEmbedding(config *EmbeddingConfig) (*OpenRouterEmbedding, error) {
    if config.OpenRouterAPIKey == "" {
        return nil, fmt.Errorf("OPENROUTER_API_KEY required for OpenRouter embedding")
    }

    return &OpenRouterEmbedding{
        apiKey:    config.OpenRouterAPIKey,
        model:     config.Model, // e.g., "openai/text-embedding-3-small"
        endpoint:  "https://openrouter.ai/api/v1/embeddings",
        dimension: config.Dimension, // 1536 for text-embedding-3-small
        client:    &http.Client{Timeout: 30 * time.Second},
    }, nil
}

func (e *OpenRouterEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    reqBody := map[string]interface{}{
        "model": e.model,
        "input": texts,
    }

    jsonBody, _ := json.Marshal(reqBody)
    req, _ := http.NewRequestWithContext(ctx, "POST", e.endpoint, bytes.NewBuffer(jsonBody))
    req.Header.Set("Authorization", "Bearer "+e.apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := e.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("OpenRouter embedding request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("OpenRouter embedding error: %s", string(body))
    }

    var result struct {
        Data []struct {
            Embedding []float32 `json:"embedding"`
        } `json:"data"`
    }
    json.NewDecoder(resp.Body).Decode(&result)

    embeddings := make([][]float32, len(result.Data))
    for i, d := range result.Data {
        embeddings[i] = d.Embedding
    }

    return embeddings, nil
}

func (e *OpenRouterEmbedding) Dimension() int { return e.dimension }
func (e *OpenRouterEmbedding) Provider() string { return "openrouter" }
```

#### 5.3.3 Embedding Model Recommendations

| Provider | Model | Dimensions | Speed | Quality | Cost |
|----------|-------|------------|-------|---------|------|
| Local | all-MiniLM-L6-v2 | 384 | Fast | Good | Free |
| Local | all-mpnet-base-v2 | 768 | Medium | Better | Free |
| OpenRouter | openai/text-embedding-3-small | 1536 | Fast | Best | $0.02/1M tokens |
| OpenRouter | openai/text-embedding-3-large | 3072 | Medium | Best | $0.13/1M tokens |

**Recommendation:**
- Testing/QA: `all-MiniLM-L6-v2` (384 dim) - fast, good quality
- Production: `openai/text-embedding-3-small` (1536 dim) - best quality/cost ratio

### 5.4 Index Trigger on File Upload

```go
// Modify HandleUploadKBFile in handlers.go

func (h *Handler) HandleUploadKBFile(c echo.Context) error {
    // ... existing file upload logic ...

    // Parse KB content
    parsedKB := parseKBContent(content)

    // Chunk and index
    chunks := ChunkKBDocument(parsedKB, tenant.ID, audienceType)

    // Generate embeddings (batch)
    texts := make([]string, len(chunks))
    for i, chunk := range chunks {
        texts[i] = chunk.Content
    }
    embeddings, err := h.vectorDB.embed.Embed(ctx, texts)
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate embeddings")
    }

    // Set embeddings
    for i := range chunks {
        chunks[i].Embedding = embeddings[i]
    }

    // Delete old documents for this tenant/audience
    h.vectorDB.DeleteByTenantAudience(ctx, tenant.ID, audienceType)

    // Insert new chunks
    err = h.vectorDB.InsertChunks(ctx, chunks)
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, "failed to index documents")
    }

    // ... continue with existing logic ...
}
```

---

## 6. Query Pipeline Design

### 6.1 Hybrid Search Function

```go
// server/router/api/v1/agent/retrieval.go

type RetrievalResult struct {
    Chunks     []DocumentChunk
    Scores     []float32
    SearchType string // "vector", "keyword", "hybrid"
}

func (v *VectorDB) HybridSearch(
    ctx context.Context,
    query string,
    tenantID int32,
    audienceType string,
    contentTypes []string, // ["service", "faq"] - filter by type
    topK int,
) (*RetrievalResult, error) {
    // 1. Generate query embedding
    embeddings, err := v.embed.Embed(ctx, []string{query})
    if err != nil {
        return nil, fmt.Errorf("failed to embed query: %w", err)
    }
    queryVector := embeddings[0]

    // 2. Build filter
    filter := fmt.Sprintf("tenant_id = %d AND audience_type = '%s' AND is_active = true",
        tenantID, audienceType)

    if len(contentTypes) > 0 {
        types := strings.Join(contentTypes, "', '")
        filter += fmt.Sprintf(" AND content_type IN ('%s')", types)
    }

    // 3. Execute hybrid search (vector + filter)
    results, err := v.table.VectorSearchWithFilter(ctx, "embedding", queryVector, topK, filter)
    if err != nil {
        return nil, fmt.Errorf("vector search failed: %w", err)
    }

    // 4. Parse results
    var chunks []DocumentChunk
    var scores []float32
    for _, r := range results {
        chunk := parseResultToChunk(r)
        chunks = append(chunks, chunk)
        scores = append(scores, r["_distance"].(float32))
    }

    return &RetrievalResult{
        Chunks:     chunks,
        Scores:     scores,
        SearchType: "hybrid",
    }, nil
}
```

### 6.2 Intent-Aware Retrieval

```go
// Retrieve different content types based on classified intent

func (s *Service) RetrieveContextForIntent(
    ctx context.Context,
    query string,
    intent string,
    tenantID int32,
    audience string,
) (*RetrievedContext, error) {
    var contentTypes []string
    var topK int

    switch intent {
    case "service_inquiry", "booking_request":
        contentTypes = []string{"service", "faq"}
        topK = 5
    case "coverage_question":
        contentTypes = []string{"coverage", "service"}
        topK = 5
    case "pricing_question":
        contentTypes = []string{"faq", "service"}
        topK = 3
    case "emergency":
        contentTypes = []string{"service", "safety"}
        topK = 3
    case "complaint", "escalation":
        contentTypes = []string{"rule", "faq"}
        topK = 3
    default:
        contentTypes = []string{"faq", "service"}
        topK = 4
    }

    result, err := s.vectorDB.HybridSearch(ctx, query, tenantID, audience, contentTypes, topK)
    if err != nil {
        return nil, err
    }

    return &RetrievedContext{
        Services:   filterByType(result.Chunks, "service"),
        FAQs:       filterByType(result.Chunks, "faq"),
        Coverage:   filterByType(result.Chunks, "coverage"),
        Rules:      filterByType(result.Chunks, "rule"),
        Safety:     filterByType(result.Chunks, "safety"),
    }, nil
}
```

### 6.3 Relevance Threshold

```go
const (
    HighRelevanceThreshold   = 0.8  // Include directly
    MediumRelevanceThreshold = 0.6  // Include with lower priority
    LowRelevanceThreshold    = 0.4  // Exclude
)

func FilterByRelevance(chunks []DocumentChunk, scores []float32, threshold float32) []DocumentChunk {
    var filtered []DocumentChunk
    for i, chunk := range chunks {
        // LanceDB returns distance (lower = more similar for L2)
        // Convert to similarity score if needed
        similarity := 1.0 - scores[i]
        if similarity >= threshold {
            filtered = append(filtered, chunk)
        }
    }
    return filtered
}
```

---

## 7. Simplified System Prompt Structure

### 7.1 New Prompt Template

```go
// Dramatically simplified - from 11 sections to 5

const SystemPromptTemplate = `
=== IDENTITY ===
You are {{.Identity.Role}} for {{.CompanyName}}.
Tone: {{.Identity.Tone}}
Brand Voice: {{.Identity.BrandVoice}}

=== CONVERSATION GUIDE ===
{{.ScriptSummary}}

=== CRITICAL CONSTRAINTS ===
1. ONLY discuss services listed in RETRIEVED CONTEXT below
2. ONLY use contact info provided below
3. If uncertain, acknowledge honestly
4. Never invent services, prices, or processes

=== CONTACT INFORMATION ===
Phone: {{.Phone}}
Email: {{.Email}}

=== RETRIEVED CONTEXT (Use ONLY this information) ===
{{range .RetrievedServices}}
SERVICE: {{.Title}}
{{.Content}}
{{end}}

{{range .RetrievedFAQs}}
FAQ: {{.Title}}
{{.Content}}
{{end}}

{{if .RetrievedCoverage}}
COVERAGE:
{{range .RetrievedCoverage}}
- {{.Content}}
{{end}}
{{end}}

=== RESPONSE FORMAT ===
- Plain text only (no markdown)
- Concise but complete
- Natural conversational tone
`
```

### 7.2 Token Savings Estimate

| Section | Before (tokens) | After (tokens) | Savings |
|---------|-----------------|----------------|---------|
| Services | 500-1500 | 100-300 | 70-80% |
| FAQs | 1000-3000 | 150-400 | 85-90% |
| Exclusions | 200-500 | 0 (not needed) | 100% |
| Coverage | 300-800 | 50-150 | 80-85% |
| Rules | 300-600 | 50-150 | 80-85% |
| Boilerplate | 500-800 | 200-300 | 60% |
| **Total** | **3000-7000** | **550-1300** | **~80%** |

### 7.3 SCRIPT.MD as Primary System Prompt

The SCRIPT.MD becomes the primary source for conversation flow guidance:

```go
func BuildSystemPrompt(config *AudienceConfig, retrieved *RetrievedContext) string {
    // SCRIPT.MD content is the conversation guide
    scriptContent := config.Script.FullContent // or Summary

    // Build prompt using template
    data := PromptData{
        Identity:          config.Audience.Identity,
        CompanyName:       config.CompanyName,
        ScriptSummary:     scriptContent,
        Phone:             config.Phone,
        Email:             config.Email,
        RetrievedServices: retrieved.Services,
        RetrievedFAQs:     retrieved.FAQs,
        RetrievedCoverage: retrieved.Coverage,
    }

    var buf bytes.Buffer
    tmpl.Execute(&buf, data)
    return buf.String()
}
```

---

## 8. Implementation Phases

### Phase 1: Foundation (Week 1-2)

**Tasks:**
1. Add LanceDB-Go dependencies and build configuration
2. Create `vectordb.go` with connection management
3. Design and implement schema
4. Create document chunking utilities
5. Add embedding service interface

**Files to Create:**
- `server/router/api/v1/agent/vectordb.go`
- `server/router/api/v1/agent/embedding.go`
- `server/router/api/v1/agent/chunker.go`

**Files to Modify:**
- `Taskfile.yaml` (add CGO flags)
- `go.mod` (add dependencies)

### Phase 2: Indexing Pipeline (Week 2-3)

**Tasks:**
1. Implement indexing on KB/Policy file upload
2. Add version tracking for re-indexing
3. Implement batch embedding generation
4. Add index management endpoints (optional)

**Files to Modify:**
- `server/router/api/v1/agent/handlers.go` (upload handlers)
- `server/router/api/v1/agent/service.go` (add VectorDB dependency)

### Phase 3: Retrieval Pipeline (Week 3-4)

**Tasks:**
1. Implement hybrid search function
2. Add intent-aware retrieval
3. Implement relevance filtering
4. Create context builder

**Files to Create:**
- `server/router/api/v1/agent/retrieval.go`

### Phase 4: Prompt Simplification (Week 4-5)

**Tasks:**
1. Create new prompt template
2. Refactor `buildSystemPrompt()` to use retrieved context
3. Remove redundant sections
4. Test with various KB sizes

**Files to Modify:**
- `server/router/api/v1/agent/service.go` (buildSystemPrompt)

### Phase 5: Cleanup & Optimization (Week 5-6)

**Tasks:**
1. Remove `compliance.go` entirely (regex checks no longer needed)
2. Remove `grounding.go` entirely (replaced by RAG pipeline)
3. Make LLM verifier optional via `LLM_VERIFIER_ENABLED` env var
4. Simplify sanitizer to basic text cleanup only
5. Performance tuning and monitoring

**Files to Remove:**
- `server/router/api/v1/agent/compliance.go` - DELETE
- `server/router/api/v1/agent/grounding.go` - DELETE

**Files to Modify:**
- `server/router/api/v1/agent/verifier.go` - Add enable/disable flag
- `server/router/api/v1/agent/sanitizer.go` - Simplify to basic cleanup

**Verifier Configuration:**
```go
// verifier.go - Make optional
type VerifierConfig struct {
    Enabled bool `env:"LLM_VERIFIER_ENABLED" default:"false"`
}

func (v *Verifier) Verify(ctx context.Context, ...) (*VerifyResult, error) {
    if !v.config.Enabled {
        return &VerifyResult{Passed: true, Skipped: true}, nil
    }
    // ... existing verification logic
}
```

---

## 9. Migration Strategy

### 9.1 Feature Flag Approach

```go
// config.go
type AgentConfig struct {
    UseRAGPipeline bool `json:"use_rag_pipeline"` // Default: false
    // ... other config
}

// service.go
func (s *Service) processChat(ctx context.Context, ...) (*ChatResponse, error) {
    if s.config.UseRAGPipeline {
        return s.processChatRAG(ctx, ...)
    }
    return s.processChatLegacy(ctx, ...)
}
```

### 9.2 Parallel Operation

1. Run both pipelines in parallel during testing
2. Compare responses for quality
3. Monitor token usage and latency
4. Gradually increase RAG traffic

### 9.3 Rollback Plan

- Keep legacy code intact behind feature flag
- Store both indexed and raw KB data
- Ability to switch back instantly

---

## 10. Performance Considerations

### 10.1 Latency Budget

| Operation | Target | Notes |
|-----------|--------|-------|
| Query embedding | <50ms | Local model preferred |
| Vector search | <20ms | With proper indexing |
| Context building | <5ms | Template rendering |
| LLM generation | 500-2000ms | Unchanged |
| **Total overhead** | **<75ms** | Acceptable |

### 10.2 Embedding Service Options

| Option | Latency | Cost | Quality |
|--------|---------|------|---------|
| OpenRouter API | 100-300ms | Per-call | High |
| Local sentence-transformers | 20-50ms | Free | Good |
| Custom fine-tuned | 20-50ms | Training cost | Best |

**Recommendation:** Start with OpenRouter API, migrate to local model if latency is critical.

### 10.3 Index Optimization

```go
// Periodic optimization task
func (v *VectorDB) Optimize(ctx context.Context) error {
    stats, err := v.table.Optimize(ctx)
    if err != nil {
        return err
    }
    slog.Info("LanceDB optimized",
        "compacted_files", stats.CompactedFiles,
        "removed_files", stats.RemovedFiles)
    return nil
}
```

---

## 11. Risk Assessment

### 11.1 Technical Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| CGO build complexity | Medium | High | Docker build, CI/CD testing |
| Embedding latency | Low | Medium | Local model fallback |
| Index corruption | Low | High | Regular backups, version control |
| Relevance quality | Medium | Medium | Threshold tuning, A/B testing |

### 11.2 Business Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Response quality regression | Medium | High | Parallel testing, feature flag |
| Missing critical info | Low | High | Always include emergency/safety |
| Increased infrastructure | Low | Low | LanceDB is embedded, no servers |

---

## 12. File Structure Changes

### 12.1 New Files

```
server/router/api/v1/agent/
├── vectordb.go        # LanceDB connection and operations
├── embedding.go       # Embedding service interface
├── chunker.go         # Document chunking logic
├── retrieval.go       # RAG query pipeline
└── prompt_template.go # New prompt templates

include/                # LanceDB C headers (from download)
└── lancedb.h

lib/                    # LanceDB native libraries (from download)
├── darwin_arm64/
├── darwin_amd64/
├── linux_arm64/
├── linux_amd64/
└── windows_amd64/
```

### 12.2 Modified Files

```
server/router/api/v1/agent/
├── service.go         # Add VectorDB, new processChat flow
├── handlers.go        # Trigger indexing on file upload
├── verifier.go        # Keep as optional safety net (disabled by default)
└── sanitizer.go       # Simplify (basic cleanup only)

store/
├── agent.go           # Add ScriptMode field to AgentTenant

Taskfile.yaml          # Add CGO environment variables
go.mod                 # Add lancedb-go dependency
```

### 12.3 Files to Remove (After Migration)

```
server/router/api/v1/agent/
├── compliance.go      # REMOVE: Regex compliance no longer needed with RAG
└── grounding.go       # REMOVE: Replaced by RAG retrieval pipeline
```

### 12.4 Database Migration

```sql
-- store/migration/sqlite/0.25/10__add_script_mode.sql

-- Add ScriptMode to agent_tenant for configurable SCRIPT.MD handling
ALTER TABLE agent_tenant ADD COLUMN script_mode TEXT DEFAULT 'full';
-- Values: 'full' (default) or 'summary'
```

---

## 13. Success Metrics

### 13.1 Technical Metrics

- **Token reduction**: Target 70-80% reduction in system prompt size
- **Latency**: <100ms additional overhead
- **Index size**: <100MB per tenant
- **Retrieval accuracy**: >90% relevant documents in top-5

### 13.2 Quality Metrics

- **Hallucination rate**: Target <5% (down from ~15%)
- **Response accuracy**: Target >95% factually correct
- **User satisfaction**: Maintain or improve
- **Escalation rate**: Maintain or reduce

---

## 14. Design Decisions (Confirmed)

The following decisions have been confirmed for implementation:

### 14.1 Embedding Model Choice
**Decision:** Support both local and OpenRouter API

| Environment | Embedding Provider | Rationale |
|-------------|-------------------|-----------|
| Testing/QA | Local sentence-transformers | Free, fast iteration, no API costs |
| Production | OpenRouter Embedding API | Higher quality, managed service |

**Configuration:**
```go
// Environment variables
EMBEDDING_PROVIDER=local|openrouter  // Default: local
EMBEDDING_MODEL=all-MiniLM-L6-v2     // For local
OPENROUTER_EMBEDDING_MODEL=...       // For OpenRouter
```

### 14.2 Index Storage Location
**Decision:** Local for testing, Tigrisdata (S3-compatible) for production

| Environment | Storage | Path/URI |
|-------------|---------|----------|
| Testing/QA | Local filesystem | `build/data/lancedb/` |
| Production | Tigrisdata (fly.io) | `s3://bucket-name/lancedb/` |

**Tigrisdata Configuration:**
```go
// Production S3-compatible storage (Tigrisdata on fly.io)
type StorageConfig struct {
    Provider      string // "local" or "s3"
    LocalPath     string // For local: "build/data/lancedb/"
    S3Endpoint    string // For Tigrisdata: "fly.storage.tigris.dev"
    S3Bucket      string
    S3Region      string // "auto" for Tigrisdata
    S3AccessKey   string
    S3SecretKey   string
}

// Environment variables for production
LANCEDB_STORAGE_PROVIDER=s3
LANCEDB_S3_ENDPOINT=fly.storage.tigris.dev
LANCEDB_S3_BUCKET=your-bucket-name
LANCEDB_S3_REGION=auto
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
```

### 14.3 Re-indexing Strategy
**Decision:** Re-index on every file upload

- Simple and predictable behavior
- No background job complexity
- Acceptable latency for KB sizes (<1000 documents)
- ContentHash still used for cache invalidation

```go
// In HandleUploadKBFile
func (h *Handler) HandleUploadKBFile(c echo.Context) error {
    // 1. Save file to database
    // 2. Parse content
    // 3. Delete old index entries for tenant/audience
    // 4. Generate embeddings for all chunks
    // 5. Insert new index entries
    // 6. Invalidate config cache
}
```

### 14.4 Feature Flag Scope
**Decision:** Global flag via environment variable

```go
// Environment variable
RAG_PIPELINE_ENABLED=true|false  // Default: false

// In config
type GlobalConfig struct {
    RAGPipelineEnabled bool `env:"RAG_PIPELINE_ENABLED" default:"false"`
}

// In service
func (s *Service) processChat(ctx context.Context, ...) (*ChatResponse, error) {
    if s.globalConfig.RAGPipelineEnabled {
        return s.processChatRAG(ctx, ...)
    }
    return s.processChatLegacy(ctx, ...)
}
```

### 14.5 Compliance Verification
**Decision:** Remove regex compliance, keep LLM verifier as optional safety net

**Remove entirely:**
- `compliance.go` - Regex-based compliance checker

**Keep as optional:**
- `verifier.go` - LLM-based semantic verifier (disabled by default)

```go
// Environment variable
LLM_VERIFIER_ENABLED=true|false  // Default: false

// Only used when RAG grounding confidence is low
// or for high-stakes conversations (emergencies)
```

**Rationale:**
- RAG grounding eliminates need for post-hoc regex checks
- LLM verifier provides safety net for edge cases
- Reduces LLM calls from 3 to 2 (or 2.5 with optional verifier)

### 14.6 SCRIPT.MD Handling
**Decision:** Full content as system prompt, configurable per tenant

```go
// New field in agent_tenant table
type AgentTenant struct {
    // ... existing fields ...
    ScriptMode string `json:"script_mode"` // "full" | "summary"
}

// Default: "full" for new tenants
// Configurable via Agent Admin UI

// In prompt building
func getScriptContent(config *AudienceConfig) string {
    if config.Tenant.ScriptMode == "summary" {
        return config.Script.Summary
    }
    return config.Script.FullContent // Default: full
}
```

**UI Configuration:**
- Add toggle in Agent Admin: "Script Mode: Full / Summary"
- Full: Include entire SCRIPT.MD content
- Summary: Use condensed version (~20 lines)

---

## 15. Next Steps

Upon approval of this plan:

1. **Phase 1 Kickoff**
   - Set up LanceDB dependencies
   - Create basic vectordb.go
   - Test connection and table creation

2. **Embedding Service Selection**
   - Benchmark OpenRouter vs local options
   - Implement chosen solution

3. **Indexing Pipeline**
   - Implement chunking for KB.MD
   - Test with existing tenant data

4. **Iterative Development**
   - Build retrieval pipeline
   - Simplify system prompt
   - Test and tune

---

## 16. Appendix: OpenRouter Embedding API

OpenRouter provides access to various embedding models. Here's how to use them:

### 16.1 Available Embedding Models via OpenRouter

| Model | Provider | Dimensions | Max Tokens | Price |
|-------|----------|------------|------------|-------|
| `openai/text-embedding-3-small` | OpenAI | 1536 | 8191 | $0.02/1M |
| `openai/text-embedding-3-large` | OpenAI | 3072 | 8191 | $0.13/1M |
| `openai/text-embedding-ada-002` | OpenAI | 1536 | 8191 | $0.10/1M |

### 16.2 API Usage

```bash
# Embedding request via OpenRouter
curl https://openrouter.ai/api/v1/embeddings \
  -H "Authorization: Bearer $OPENROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai/text-embedding-3-small",
    "input": ["Hello world", "How are you?"]
  }'

# Response
{
  "data": [
    {"embedding": [0.1, 0.2, ...], "index": 0},
    {"embedding": [0.3, 0.4, ...], "index": 1}
  ],
  "model": "openai/text-embedding-3-small",
  "usage": {"prompt_tokens": 6, "total_tokens": 6}
}
```

---

---

## 17. Implementation Status

| Phase | Description | Status | Document |
|-------|-------------|--------|----------|
| Phase 1 | Foundation | Complete | [DOCS_LANCEDB_PHASE1.MD](./DOCS_LANCEDB_PHASE1.MD) |
| Phase 2 | Indexing Pipeline | Complete | [DOCS_LANCEDB_PHASE2.MD](./DOCS_LANCEDB_PHASE2.MD) |
| Phase 3 | Retrieval Pipeline | Complete | [DOCS_LANCEDB_PHASE3.MD](./DOCS_LANCEDB_PHASE3.MD) |
| Phase 4 | Prompt Simplification | Complete | [DOCS_LANCEDB_PHASE4.MD](./DOCS_LANCEDB_PHASE4.MD) |
| Phase 5 | Cleanup & Optimization | Complete | [DOCS_LANCEDB_PHASE5.MD](./DOCS_LANCEDB_PHASE5.MD) |

---

*Document Version: 1.4*
*Created: 2026-01-22*
*Updated: 2026-01-23*
*Author: Claude Code*
*Status: All Phases Complete*
