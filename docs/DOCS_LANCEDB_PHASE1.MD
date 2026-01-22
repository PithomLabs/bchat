# LanceDB RAG Implementation - Phase 1: Foundation

**Status:** Complete
**Date:** 2026-01-23
**Parent Document:** [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD)

---

## Overview

Phase 1 established the foundation for the RAG (Retrieval-Augmented Generation) pipeline by setting up LanceDB dependencies, creating the vector database abstraction layer, and implementing document chunking utilities.

---

## Files Created

### 1. `server/router/api/v1/agent/vectordb.go`

Core vector database abstraction and implementations.

**Key Components:**

```go
// VectorDB interface - abstraction for vector database operations
type VectorDB interface {
    Insert(ctx context.Context, chunks []DocumentChunk) error
    Delete(ctx context.Context, tenantID int32, audienceType string) error
    Search(ctx context.Context, query SearchQuery) (*SearchResult, error)
    Close() error
    Stats(ctx context.Context) (*VectorDBStats, error)
}

// VectorDBConfig - configuration from environment
type VectorDBConfig struct {
    StorageProvider string // "memory", "local", or "s3"
    LocalPath       string // For local: "build/data/lancedb/"
    S3Endpoint      string // For S3/Tigrisdata
    S3Bucket        string
    S3Region        string
    S3AccessKey     string
    S3SecretKey     string
    EmbeddingConfig *EmbeddingConfig
    Enabled         bool   // RAG_PIPELINE_ENABLED
}
```

**Implementations:**

| Implementation | Purpose | Use Case |
|----------------|---------|----------|
| `MemoryVectorDB` | In-memory storage with cosine similarity | Testing, development |
| `NoOpVectorDB` | No-op when RAG disabled | Production fallback |

**Helper Functions:**
- `NewVectorDBConfigFromEnv()` - Load config from environment variables
- `NewVectorDB(config)` - Factory function to create appropriate implementation
- `cosineSimilarity(a, b)` - Calculate similarity between vectors
- `RetrieveContextForQuery()` - Intent-aware retrieval helper

---

### 2. `server/router/api/v1/agent/embedding.go`

Embedding service interface supporting multiple providers.

**Interface:**

```go
type EmbeddingService interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int
    Provider() string
}
```

**Implementations:**

| Provider | Class | Dimension | Use Case |
|----------|-------|-----------|----------|
| Local | `LocalEmbedding` | 384 | Testing (HTTP to Python service) |
| OpenRouter | `OpenRouterEmbedding` | 1536 | Production |
| Mock | `MockEmbedding` | 384 | Unit tests |

**Configuration:**

```go
type EmbeddingConfig struct {
    Provider         string // "local", "openrouter", or "mock"
    Model            string // Model name
    Dimension        int    // Vector dimension
    OpenRouterAPIKey string // For OpenRouter provider
    LocalEndpoint    string // For local provider (default: http://localhost:8001/embed)
    BatchSize        int    // Max texts per batch
}
```

---

### 3. `server/router/api/v1/agent/chunker.go`

Document chunking utilities for converting KB/Policy content to vector-indexable chunks.

**Core Types:**

```go
type DocumentChunk struct {
    ID            string    // Unique: tenantID:audience:type:code
    TenantID      int32
    AudienceType  string
    ContentType   string    // service, faq, exclusion, coverage, rule, safety, kb_section
    Title         string
    Content       string
    Code          string
    IsEmergency   bool
    IsActive      bool
    Priority      int32
    SourceVersion int32
    Embedding     []float32 // Populated after embedding
    IndexedAt     time.Time
}
```

**Chunking Methods:**

| Method | Input | Output |
|--------|-------|--------|
| `ChunkKBContent()` | ParsedKB | Services, FAQs, Exclusions, Coverage, Safety, KB Sections |
| `ChunkPolicyContent()` | ParsedPolicy | Rules, Intents |
| `ChunkFromStoreTypes()` | Store types | All chunk types (for existing data) |

**Content Builders:**

Each content type has a dedicated builder function to create semantic text:
- `buildServiceContent()` - Service name, description, response time, emergency flag
- `buildFAQContent()` - Question and answer
- `buildExclusionContent()` - Name, description, exception rule, referral
- `buildCoverageContent()` - Area name, type, state, included flag
- `buildSafetyContent()` - Name, triggers, instructions
- `buildRuleContent()` - Name, description, applies to
- `buildIntentContent()` - Name, description, examples, category

---

## Build Configuration

### Taskfile.yml Updates

Added LanceDB-specific build tasks:

```yaml
vars:
  LANCEDB_VERSION: v0.1.2
  LANCEDB_LIB_DIR: "{{.ROOT_DIR}}/lib/{{.PLATFORM}}_{{.ARCH}}"

tasks:
  setup:lancedb:
    desc: Download LanceDB native libraries
    cmds:
      - curl -sSL https://raw.githubusercontent.com/lancedb/lancedb-go/main/scripts/download-artifacts.sh | bash -s {{.LANCEDB_VERSION}}
    status:
      - test -f "{{.LANCEDB_LIB_DIR}}/liblancedb_go.a"

  build:backend:rag:
    desc: Build with LanceDB RAG support
    deps: [setup:lancedb]
    env:
      CGO_ENABLED: "1"
      CGO_CFLAGS: "-I{{.ROOT_DIR}}/include"
      CGO_LDFLAGS: "..."  # Platform-specific linking
```

### Dependencies Added

```
go.mod:
  github.com/lancedb/lancedb-go v0.1.2
  github.com/apache/arrow/go/v17 v17.0.0
```

---

## Environment Variables

```bash
# Feature flag
RAG_PIPELINE_ENABLED=true|false  # Default: false

# Storage provider
LANCEDB_STORAGE_PROVIDER=memory|local|s3  # Default: memory
LANCEDB_LOCAL_PATH=build/data/lancedb     # For local storage

# Embedding provider
EMBEDDING_PROVIDER=local|openrouter|mock  # Default: local
EMBEDDING_MODEL=all-MiniLM-L6-v2          # For local
LOCAL_EMBEDDING_ENDPOINT=http://localhost:8001/embed

# OpenRouter (production)
OPENROUTER_API_KEY=sk-or-xxxxx
```

---

## Testing

### Memory Vector DB Test

```go
// Create in-memory DB
config := &VectorDBConfig{
    StorageProvider: "memory",
    Enabled: true,
    EmbeddingConfig: &EmbeddingConfig{
        Provider: "mock",
        Dimension: 384,
    },
}
db, _ := NewVectorDB(config)

// Insert chunks
chunks := []DocumentChunk{
    {ID: "1:ext:service:water", TenantID: 1, ContentType: "service", ...},
}
db.Insert(ctx, chunks)

// Search
result, _ := db.Search(ctx, SearchQuery{
    QueryText: "water damage",
    TenantID: 1,
    TopK: 5,
})
```

### Local Embedding Server

```bash
# Start Python embedding server
task embedding:server

# Or manually:
cd scripts && python embedding_server.py
# Runs on http://localhost:8001
```

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    Phase 1: Foundation                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │
│  │  VectorDB   │    │  Embedding  │    │   Chunker   │     │
│  │  Interface  │    │   Service   │    │             │     │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘     │
│         │                  │                  │             │
│    ┌────┴────┐        ┌────┴────┐        ┌────┴────┐       │
│    │ Memory  │        │  Local  │        │  KB.MD  │       │
│    │  NoOp   │        │OpenRouter│       │POLICY.MD│       │
│    └─────────┘        │  Mock   │        └─────────┘       │
│                       └─────────┘                          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Next Steps (Phase 2)

- Implement actual LanceDB backend (local and S3)
- Trigger indexing on KB/Policy file upload
- Add VectorDB to Service struct
- Integrate with import handlers
