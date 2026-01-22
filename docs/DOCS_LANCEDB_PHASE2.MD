# LanceDB RAG Implementation - Phase 2: Indexing Pipeline

**Status:** Complete
**Date:** 2026-01-23
**Parent Document:** [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD)
**Previous Phase:** [DOCS_LANCEDB_PHASE1.MD](./DOCS_LANCEDB_PHASE1.MD)

---

## Overview

Phase 2 implemented the full LanceDB storage backend and integrated automatic indexing into the KB/Policy file upload flow. When files are uploaded, content is automatically chunked, embedded, and indexed for RAG retrieval.

---

## Files Modified

### 1. `server/router/api/v1/agent/vectordb.go`

Added complete `LanceVectorDB` implementation using `lancedb-go` library.

**New Components:**

```go
// LanceVectorDB - Production implementation using LanceDB
type LanceVectorDB struct {
    conn     contracts.IConnection
    table    contracts.ITable
    embedSvc EmbeddingService
    config   *VectorDBConfig
    mu       sync.RWMutex
}

// Constants
const (
    lanceTableName     = "kb_documents"
    embeddingDimension = 384  // Default for all-MiniLM-L6-v2
)
```

**Methods Implemented:**

| Method | Description |
|--------|-------------|
| `NewLanceVectorDB(config, embedSvc)` | Create and connect to LanceDB |
| `ensureTable(ctx)` | Open existing or create new table |
| `buildSchema()` | Create Arrow schema for documents |
| `createIndexes(ctx)` | Create vector and BTree indexes |
| `Insert(ctx, chunks)` | Add chunks with auto-embedding |
| `chunksToArrowRecord(chunks)` | Convert to Arrow format |
| `Delete(ctx, tenantID, audienceType)` | Remove chunks by filter |
| `Search(ctx, query)` | Vector similarity search |
| `rowToDocumentChunk(row)` | Parse LanceDB result |
| `Close()` | Release resources |
| `Stats(ctx)` | Get database statistics |

**Schema Definition:**

```go
schema := lancedb.NewSchemaBuilder().
    AddStringField("id", false).
    AddInt32Field("tenant_id", false).
    AddStringField("audience_type", false).
    AddStringField("content_type", false).
    AddStringField("title", true).
    AddStringField("content", false).
    AddStringField("code", true).
    AddBooleanField("is_emergency", true).
    AddBooleanField("is_active", false).
    AddInt32Field("priority", true).
    AddVectorField("embedding", dim, contracts.VectorDataTypeFloat32, false).
    AddInt64Field("indexed_at", false).
    AddInt32Field("source_version", true).
    Build()
```

**Indexes Created:**

| Index | Type | Purpose |
|-------|------|---------|
| `idx_embedding` | IVF-PQ | Vector similarity search |
| `idx_tenant` | BTree | Fast tenant filtering |

---

### 2. `server/router/api/v1/agent/service.go`

Integrated VectorDB and Chunker into the Service struct.

**Changes:**

```go
// Service struct additions
type Service struct {
    // ... existing fields ...
    vectorDB  VectorDB   // NEW: Vector database for RAG
    chunker   *Chunker   // NEW: Document chunker
}

// NewService additions
func NewService(s *store.Store, p *profile.Profile) *Service {
    // ... existing initialization ...

    // Initialize chunker
    svc.chunker = NewChunker()

    // Initialize vector database for RAG pipeline
    vectorDBConfig := NewVectorDBConfigFromEnv()
    vectorDB, err := NewVectorDB(vectorDBConfig)
    if err != nil {
        slog.Error("Failed to initialize vector database", "error", err)
        vectorDB = NewNoOpVectorDB()  // Fallback to no-op
    }
    svc.vectorDB = vectorDB

    return svc
}
```

---

### 3. `server/router/api/v1/agent/handlers.go`

Added automatic indexing after file import.

**New Function:**

```go
// indexContentForRAG indexes parsed KB and Policy content into the vector database.
func (h *Handler) indexContentForRAG(
    ctx context.Context,
    tenantID int32,
    audienceType string,
    kb *ParsedKB,
    policy *ParsedPolicy,
) error {
    chunker := h.service.chunker
    vectorDB := h.service.vectorDB

    if chunker == nil || vectorDB == nil {
        return nil // RAG not enabled
    }

    // Delete existing chunks for this tenant/audience
    if err := vectorDB.Delete(ctx, tenantID, audienceType); err != nil {
        return fmt.Errorf("failed to delete existing chunks: %w", err)
    }

    // Chunk KB content
    var allChunks []DocumentChunk
    if kb != nil {
        kbChunks := chunker.ChunkKBContent(kb, tenantID, audienceType, 1)
        allChunks = append(allChunks, kbChunks...)
    }

    // Chunk Policy content
    if policy != nil {
        policyChunks := chunker.ChunkPolicyContent(policy, tenantID, audienceType, 1)
        allChunks = append(allChunks, policyChunks...)
    }

    if len(allChunks) == 0 {
        return nil
    }

    // Insert chunks (embeddings generated automatically)
    if err := vectorDB.Insert(ctx, allChunks); err != nil {
        return fmt.Errorf("failed to insert chunks: %w", err)
    }

    slog.Info("Indexed content for RAG",
        "tenantID", tenantID,
        "audience", audienceType,
        "chunks", len(allChunks))

    return nil
}
```

**Integration Point:**

```go
// In importFiles() - called after storing source files
func (h *Handler) importFiles(...) (*AudienceInfo, error) {
    // ... existing import logic ...

    // Store source files
    // ... (KB and Policy saved to database) ...

    // NEW: Index content for RAG pipeline
    if err := h.indexContentForRAG(ctx, tenantID, audienceType, parsedKB, parsedPolicy); err != nil {
        slog.Warn("Failed to index content for RAG", "error", err, ...)
        // Don't fail import if indexing fails - RAG is an enhancement
    }

    return &AudienceInfo{...}, nil
}
```

---

### 4. `Taskfile.yml`

Fixed Linux build compatibility.

**Problem:** LanceDB static library (`.a`) was built with BSD ar format (macOS), incompatible with GNU linker on Linux.

**Solution:** Use shared library (`.so`) on Linux:

```yaml
build:backend:rag:
  env:
    CGO_LDFLAGS: >-
      {{if eq .PLATFORM "linux"}}
        -L{{.LANCEDB_LIB_DIR}} -llancedb_go -Wl,-rpath,{{.LANCEDB_LIB_DIR}}
      {{else}}
        {{.LANCEDB_LIB_DIR}}/liblancedb_go.a
      {{end}}
      {{if eq .PLATFORM "darwin"}}
        -framework Security -framework CoreFoundation
      {{end}}

dev:backend:rag:
  env:
    # Same CGO_LDFLAGS as above
    LD_LIBRARY_PATH: "{{.LANCEDB_LIB_DIR}}:${LD_LIBRARY_PATH}"
```

---

## Data Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Phase 2: Indexing Pipeline                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  User uploads KB.MD / POLICY.MD                                     │
│         │                                                           │
│         ▼                                                           │
│  ┌─────────────────┐                                                │
│  │ HandleImport*() │  (handlers.go)                                 │
│  └────────┬────────┘                                                │
│           │                                                         │
│           ▼                                                         │
│  ┌─────────────────┐                                                │
│  │  importFiles()  │  Parse KB & Policy, store to SQLite            │
│  └────────┬────────┘                                                │
│           │                                                         │
│           ▼                                                         │
│  ┌─────────────────────┐                                            │
│  │ indexContentForRAG()│  NEW in Phase 2                            │
│  └────────┬────────────┘                                            │
│           │                                                         │
│     ┌─────┴─────┐                                                   │
│     ▼           ▼                                                   │
│  ┌───────┐  ┌───────────┐                                           │
│  │Chunker│  │ VectorDB  │                                           │
│  └───┬───┘  └─────┬─────┘                                           │
│      │            │                                                 │
│      │  chunks    │  Delete old → Insert new                        │
│      └────────────┤                                                 │
│                   ▼                                                 │
│           ┌──────────────┐                                          │
│           │ LanceVectorDB│                                          │
│           └──────┬───────┘                                          │
│                  │                                                  │
│          ┌───────┴───────┐                                          │
│          ▼               ▼                                          │
│   ┌────────────┐  ┌────────────┐                                    │
│   │  Embedding │  │  LanceDB   │                                    │
│   │  Service   │  │   Table    │                                    │
│   └────────────┘  └────────────┘                                    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Storage Backends

### Local Storage (Development/Testing)

```bash
LANCEDB_STORAGE_PROVIDER=local
LANCEDB_LOCAL_PATH=build/data/lancedb
```

Creates directory structure:
```
build/data/lancedb/
└── kb_documents.lance/
    ├── data/
    ├── _transactions/
    └── _versions/
```

### S3/Tigrisdata Storage (Production)

```bash
LANCEDB_STORAGE_PROVIDER=s3
LANCEDB_S3_ENDPOINT=fly.storage.tigris.dev
LANCEDB_S3_BUCKET=your-bucket-name
LANCEDB_S3_REGION=auto
AWS_ACCESS_KEY_ID=tid_xxxxx
AWS_SECRET_ACCESS_KEY=tsec_xxxxx
```

Stores at: `s3://your-bucket-name/lancedb/kb_documents.lance/`

---

## Chunk Types Indexed

| Content Type | Source | Fields Indexed |
|--------------|--------|----------------|
| `service` | KB.MD | Name, description, response time, emergency flag |
| `faq` | KB.MD | Question, answer |
| `exclusion` | KB.MD | Name, description, exception rule, referral |
| `coverage` | KB.MD | Area name, type, state, included flag |
| `safety` | KB.MD | Name, triggers, instructions |
| `kb_section` | KB.MD | Title, content |
| `rule` | POLICY.MD | Name, description, applies to |
| `intent` | POLICY.MD | Name, description, examples, category |

---

## Error Handling

Indexing failures are non-fatal:

```go
if err := h.indexContentForRAG(ctx, tenantID, audienceType, parsedKB, parsedPolicy); err != nil {
    slog.Warn("Failed to index content for RAG", "error", err, ...)
    // Don't fail the import - RAG is an enhancement, not critical path
}
```

This ensures:
- File uploads always succeed (data stored in SQLite)
- RAG indexing is best-effort
- System degrades gracefully if LanceDB unavailable

---

## Verification

### Build Tests

```bash
# RAG-enabled build
task build:backend:rag
# Success: builds binary with LanceDB support

# Standard build (no LanceDB)
task build
# Success: builds binary without LanceDB (RAG disabled by default)
```

### Runtime Verification

When RAG is enabled, logs show:

```
INFO LanceDB vector database initialized uri=build/data/lancedb provider=local
INFO Created new LanceDB table name=kb_documents
```

On file upload:

```
INFO Indexed content for RAG tenantID=1 audience=external chunks=47
```

---

## Next Steps (Phase 3)

Phase 3 will integrate RAG retrieval into the chat response generation:

1. Modify `buildSystemPrompt()` to use retrieved context
2. Add `RetrieveContextForQuery()` call before LLM generation
3. Replace full KB inclusion with top-K retrieved chunks
4. Add relevance scoring and threshold filtering

---

## Appendix: Arrow Record Building

The `chunksToArrowRecord()` function handles the complex conversion from Go structs to Apache Arrow format:

```go
func (db *LanceVectorDB) chunksToArrowRecord(chunks []DocumentChunk) (arrow.Record, error) {
    pool := memory.NewGoAllocator()

    // Build schema matching LanceDB table
    schema := arrow.NewSchema([]arrow.Field{
        {Name: "id", Type: arrow.BinaryTypes.String, Nullable: false},
        {Name: "tenant_id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
        // ... other fields ...
        {Name: "embedding", Type: arrow.FixedSizeListOf(int32(dim), arrow.PrimitiveTypes.Float32), Nullable: false},
    }, nil)

    // Create builders for each column
    idBuilder := array.NewStringBuilder(pool)
    // ... create other builders ...
    embeddingBuilder := array.NewFixedSizeListBuilder(pool, int32(dim), arrow.PrimitiveTypes.Float32)

    // Populate builders from chunks
    for _, chunk := range chunks {
        idBuilder.Append(chunk.ID)
        // ... append other fields ...

        // Handle fixed-size vector embedding
        embeddingBuilder.Append(true)
        valueBuilder := embeddingBuilder.ValueBuilder().(*array.Float32Builder)
        for _, v := range chunk.Embedding {
            valueBuilder.Append(v)
        }
        // Pad if needed
        for i := len(chunk.Embedding); i < dim; i++ {
            valueBuilder.Append(0)
        }
    }

    // Build arrays and create record
    record := array.NewRecord(schema, []arrow.Array{...}, int64(len(chunks)))
    return record, nil
}
```

This ensures proper memory management and type safety when interfacing with the LanceDB Rust core via CGO.
