# Changes Documentation

This document tracks planned and implemented changes with timestamps.

---

## 2026-01-27 03:45 - OpenRouter Support for all-MiniLM-L6-v2

**Status:** Implemented
**Priority:** High
**Reason:** Enable GPU-accelerated embeddings via OpenRouter while maintaining compatibility with existing Cybertron embeddings (same 384-dimension vectors).

### Problem

Currently, switching from Cybertron to OpenRouter changes the embedding model:
- **Cybertron:** `sentence-transformers/all-MiniLM-L6-v2` (384 dimensions)
- **OpenRouter:** `openai/text-embedding-3-small` (1536 dimensions)

This causes:
1. Dimension mismatch errors (requires table recreation)
2. Loss of existing embeddings
3. Different semantic space (embeddings not comparable)

### Solution

Add support for `sentence-transformers/all-MiniLM-L6-v2` via OpenRouter, allowing:
- GPU-accelerated embeddings
- Same 384-dimension vectors as Cybertron
- Seamless switching between providers without reindexing

### OpenRouter Model Info

```
Model: sentence-transformers/all-MiniLM-L6-v2
Dimensions: 384
Max Tokens: 512
Provider: OpenRouter
```

### Implementation Plan

#### Step 1: Update Model Detection in embedding.go

```go
// In NewOpenRouterEmbedding - update dimension detection
func NewOpenRouterEmbedding(config *EmbeddingConfig) (*OpenRouterEmbedding, error) {
    // ...

    // Set dimension based on model
    dimension := config.Dimension
    if dimension == 0 {
        switch model {
        case "openai/text-embedding-3-large":
            dimension = 3072
        case "openai/text-embedding-ada-002":
            dimension = 1536
        case "openai/text-embedding-3-small":
            dimension = 1536
        // NEW: Support sentence-transformers models via OpenRouter
        case "sentence-transformers/all-MiniLM-L6-v2",
             "sentence-transformers/all-MiniLM-L12-v2",
             "sentence-transformers/paraphrase-MiniLM-L6-v2":
            dimension = 384
        case "sentence-transformers/all-mpnet-base-v2":
            dimension = 768
        default:
            dimension = 1536 // OpenAI default
        }
    }
    // ...
}
```

#### Step 2: Update Default Model Selection

```go
// In NewEmbeddingConfigFromEnv
case "openrouter":
    // Check if user wants MiniLM compatibility
    model = getEnvOrDefault("EMBEDDING_MODEL", "openai/text-embedding-3-small")
    dimension = getOpenRouterDimension(model)
```

#### Step 3: Add Helper Function

```go
func getOpenRouterDimension(modelName string) int {
    switch modelName {
    case "openai/text-embedding-3-large":
        return 3072
    case "openai/text-embedding-ada-002", "openai/text-embedding-3-small":
        return 1536
    case "sentence-transformers/all-MiniLM-L6-v2",
         "sentence-transformers/all-MiniLM-L12-v2",
         "sentence-transformers/paraphrase-MiniLM-L6-v2":
        return 384
    case "sentence-transformers/all-mpnet-base-v2":
        return 768
    default:
        return 1536
    }
}
```

### Files to Modify

| File | Change |
|------|--------|
| `server/router/api/v1/agent/embedding.go` | Add dimension detection for sentence-transformers models |

### Usage After Implementation

```bash
# Use OpenRouter with MiniLM (GPU-accelerated, 384 dimensions)
EMBEDDING_PROVIDER=openrouter \
EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L6-v2 \
task run:rag

# Seamlessly switch to Cybertron (CPU, same 384 dimensions)
EMBEDDING_PROVIDER=cybertron \
task run:rag:cybertron

# No reindexing needed - same embedding space!
```

### Benefits

1. **GPU acceleration** via OpenRouter infrastructure
2. **Same dimensions** as Cybertron (384) - no table recreation
3. **Semantic compatibility** - can mix providers without reindexing
4. **Cost effective** - MiniLM is cheaper than OpenAI embeddings

### Verification

1. Set `EMBEDDING_PROVIDER=openrouter` and `EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L6-v2`
2. Check logs show `dimension=384`
3. Rebuild index - should complete without dimension mismatch
4. Switch to `EMBEDDING_PROVIDER=cybertron`
5. Query should work without reindexing

---

## 2026-01-27 04:05 - Fix Schema-Based Dimension Detection

**Status:** Implemented
**Priority:** High
**Reason:** Dimension mismatch detection failed when table was empty because it queried data instead of schema.

### Problem

The `getTableEmbeddingDimension()` function queried table data to detect dimensions:
```go
results, err := db.table.SelectWithLimit(ctx, 1, 0)
if len(results) == 0 {
    // Table exists but is empty - no schema conflict possible
    return 0  // BUG: Schema conflict IS possible!
}
```

This caused dimension mismatch errors when:
1. Table was created with dimension A (e.g., 1536)
2. Table was empty or data was deleted
3. User tried to insert with dimension B (e.g., 384)
4. Detection returned 0, skipping table recreation
5. LanceDB threw schema mismatch error

### Solution

Query the table **schema** instead of data. Schema is fixed at table creation and always available:

```go
func (db *LanceVectorDB) getTableEmbeddingDimension(ctx context.Context) int {
    schema, err := db.table.Schema(ctx)
    if err != nil {
        return 0
    }

    for i := 0; i < schema.NumFields(); i++ {
        field := schema.Field(i)
        if field.Name == "embedding" {
            if listType, ok := field.Type.(*arrow.FixedSizeListType); ok {
                return int(listType.Len())
            }
        }
    }
    return 0
}
```

### Files Modified

| File | Change |
|------|--------|
| `server/router/api/v1/agent/vectordb_lance.go` | Rewrite `getTableEmbeddingDimension()` to use schema |

### Verification

1. Create table with OpenAI embeddings (1536 dim): `EMBEDDING_MODEL=openai/text-embedding-3-small task run:rag`
2. Delete all data or start fresh
3. Switch to MiniLM (384 dim): `EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L6-v2 task run:rag`
4. Should see: `Embedding dimension mismatch detected, recreating table`
5. Index should complete successfully

---

## 2026-01-27 05:30 - Q&A Pairs Testing System

**Status:** Implemented
**Priority:** Medium
**Reason:** Evaluate embedding quality by generating Q&A pairs from KB and testing if the RAG system can retrieve the correct answers.

### Problem

No objective way to measure embedding quality:
1. Can't verify if embeddings are capturing semantic meaning
2. Can't compare different embedding providers objectively
3. Can't detect retrieval degradation after changes

### Solution

Create a Q&A pairs testing system that:
1. Uses LLM to generate diverse Q&A pairs from KB content
2. Tests if RAG retrieval finds the expected answer for each question
3. Calculates Recall@K metrics

### Database Schema

```sql
CREATE TABLE IF NOT EXISTS agent_qa_pairs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    question TEXT NOT NULL,
    expected_answer TEXT NOT NULL,
    source_section TEXT,
    source_chunk_id TEXT,
    difficulty TEXT DEFAULT 'medium',
    category TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenant(id) ON DELETE CASCADE
);
```

### API Endpoints

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/:slug/qa-pairs/generate` | `HandleGenerateQAPairs` | Generate Q&A pairs from KB using LLM |
| GET | `/:slug/qa-pairs` | `HandleListQAPairs` | List all Q&A pairs |
| POST | `/:slug/qa-pairs` | `HandleCreateQAPair` | Create single Q&A pair |
| PUT | `/:slug/qa-pairs/:id` | `HandleUpdateQAPair` | Update Q&A pair |
| DELETE | `/:slug/qa-pairs/:id` | `HandleDeleteQAPair` | Delete Q&A pair |
| POST | `/:slug/qa-pairs/:id/test` | `HandleTestQAPair` | Test single pair retrieval |
| POST | `/:slug/qa-pairs/test-all` | `HandleTestAllQAPairs` | Test all pairs, return metrics |

### Service Methods Added

```go
// CallLLMSimple - Simple LLM call for handlers
func (s *Service) CallLLMSimple(ctx context.Context, tenantID int32, systemPrompt, userMessage string) (string, error)

// SearchVectorDB - Direct vector search for testing
func (s *Service) SearchVectorDB(ctx context.Context, tenantID int32, audienceType, query string, topK int) (*SearchResult, error)
```

### Files Modified

| File | Change |
|------|--------|
| `store/migration/sqlite/0.25/17__agent_qa_pairs.sql` | New migration |
| `store/agent.go` | `AgentQAPair`, `FindAgentQAPair` types, interface methods |
| `store/driver.go` | Driver interface methods |
| `store/db/sqlite/agent.go` | SQLite CRUD implementation |
| `store/db/mysql/agent.go` | Stub implementations |
| `store/db/postgres/agent.go` | Stub implementations |
| `server/router/api/v1/agent/handlers.go` | Q&A handlers |
| `server/router/api/v1/agent/service.go` | `CallLLMSimple`, `SearchVectorDB` methods |
| `server/router/api/v1/v1.go` | Route registration |

### Test Flow

1. **Generate Pairs**
   ```bash
   POST /api/v1/admin/agent/:slug/qa-pairs/generate
   Body: { "max_pairs": 50, "audience_type": "external" }
   ```

2. **Test All Pairs**
   ```bash
   POST /api/v1/admin/agent/:slug/qa-pairs/test-all
   Response: {
     "total_pairs": 50,
     "found": 42,
     "not_found": 8,
     "recall_at_5": 0.84,
     "avg_score": 0.76
   }
   ```

### Verification

1. Start server: `task run:rag:cybertron`
2. Upload KB content to a tenant
3. Click "Rebuild Index" in Agent Admin
4. Generate Q&A pairs via API or UI
5. Run "Test All" to see Recall@K metrics
6. Compare different embedding providers

---

## 2026-01-27 ~07:00 - Fix Q&A Generation for Large KB Files

**Status:** Planned
**Priority:** High
**Reason:** Q&A generation fails when KB content exceeds LLM context window (131K tokens). User's KB is ~485K tokens.

### Problem

```
Failed to generate Q&A pairs: LLM call failed: LLM request failed:
error, code: 400, message: This endpoint's maximum context length is 131072 tokens.
However, you requested about 484526 tokens (484526 of text input).
```

Current implementation sends **entire KB content** to LLM in one request:

```go
// handlers.go line 4074-4081
kbFile, err := h.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{...})
// kbFile.Content = 484K tokens (TOO LARGE!)

// line 4089
pairs, err := h.generateQAPairsFromKB(ctx, tenant.ID, kbFile.Content, req.MaxPairs)
```

### Solution: Chunk-Based Q&A Generation

Instead of sending entire KB, use the **already-indexed chunks** from LanceDB:

```
┌─────────────────────────────────────────────────────────────────┐
│                     CURRENT (BROKEN)                             │
│  KB.MD (485K tokens) ──────────► LLM ──────► Q&A Pairs          │
│                          ❌ EXCEEDS LIMIT                        │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                     PROPOSED (FIXED)                             │
│                                                                  │
│  LanceDB Chunks ──► Sample N chunks ──► LLM ──► Q&A Pairs       │
│  (already indexed)   (diverse selection)   ✓ FITS LIMIT         │
│                                                                  │
│  Benefits:                                                       │
│  1. Uses existing chunked content (no re-processing)            │
│  2. Each chunk is already token-bounded                         │
│  3. Can generate pairs that map directly to chunk IDs           │
│  4. Better for retrieval testing (pairs match indexed chunks)   │
└─────────────────────────────────────────────────────────────────┘
```

### Implementation Plan

#### Step 1: Add Chunk Sampling to Service

```go
// service.go - New method
func (s *Service) SampleChunksForQA(ctx context.Context, tenantID int32, audienceType string, maxChunks int) ([]DocumentChunk, error) {
    // Get chunks from LanceDB for this tenant/audience
    // Sample diverse chunks (different sections, file types)
    // Return up to maxChunks chunks
}
```

#### Step 2: Modify generateQAPairsFromKB

```go
// handlers.go - Rename to generateQAPairsFromChunks
func (h *Handler) generateQAPairsFromChunks(ctx context.Context, tenantID int32, chunks []DocumentChunk, pairsPerChunk int) ([]*store.AgentQAPair, error) {
    var allPairs []*store.AgentQAPair

    for _, chunk := range chunks {
        // Generate 2-3 Q&A pairs per chunk
        prompt := buildChunkQAPrompt(chunk.Content, chunk.Heading, pairsPerChunk)

        response, err := h.service.CallLLMSimple(ctx, tenantID, systemPrompt, prompt)
        if err != nil {
            slog.Warn("Failed to generate pairs for chunk", "chunkID", chunk.ID, "error", err)
            continue
        }

        pairs := parseQAPairs(response)
        for _, p := range pairs {
            p.SourceChunkID = chunk.ID  // Link to actual chunk!
            p.SourceSection = chunk.Heading
            allPairs = append(allPairs, p)
        }
    }

    return allPairs, nil
}
```

#### Step 3: Update Handler

```go
// handlers.go - HandleGenerateQAPairs
func (h *Handler) HandleGenerateQAPairs(c echo.Context) error {
    // ... validation ...

    // NEW: Sample chunks instead of full KB
    maxChunks := 25  // ~25 chunks × 512 tokens = ~12K tokens (safe)
    pairsPerChunk := 2  // Generate 2 Q&A pairs per chunk = 50 total

    chunks, err := h.service.SampleChunksForQA(ctx, tenant.ID, req.AudienceType, maxChunks)
    if err != nil || len(chunks) == 0 {
        return echo.NewHTTPError(http.StatusBadRequest, "No indexed chunks found. Run 'Rebuild Index' first.")
    }

    pairs, err := h.generateQAPairsFromChunks(ctx, tenant.ID, chunks, pairsPerChunk)
    // ... rest unchanged ...
}
```

#### Step 4: LLM Prompt for Single Chunk

```go
const chunkQAPrompt = `Generate %d question-answer pairs from this content section.

SECTION: %s
CONTENT:
---
%s
---

Requirements:
1. Questions should be natural queries a user might ask
2. Answers must be directly from the content above
3. Vary difficulty: 1 easy factual, 1 requiring inference

Output JSON only:
{"qa_pairs": [{"question": "...", "answer": "...", "difficulty": "easy|medium", "category": "..."}]}
`
```

### Files to Modify

| File | Change |
|------|--------|
| `server/router/api/v1/agent/service.go` | Add `SampleChunksForQA()` method |
| `server/router/api/v1/agent/handlers.go` | Rewrite `HandleGenerateQAPairs` to use chunks |
| `server/router/api/v1/agent/vectordb.go` | Add `ListChunks()` or `SampleChunks()` to VectorDB interface |

### Configuration Options

```go
type QAGenerationConfig struct {
    MaxChunks      int  // How many chunks to sample (default: 25)
    PairsPerChunk  int  // Q&A pairs per chunk (default: 2)
    MaxTotalPairs  int  // Cap total pairs (default: 50)
    DiverseSample  bool // Sample from different sections (default: true)
}
```

### Alternative Approaches Considered

| Approach | Pros | Cons |
|----------|------|------|
| **Chunk-based (chosen)** | Uses existing index, links to chunks, fits context | Requires LanceDB query |
| Split KB into sections | Simple, no LanceDB needed | Manual splitting, no chunk linking |
| Use middle-out transform | OpenRouter feature, automatic | Provider-specific, less control |
| Streaming/batched calls | Process full KB in parts | Complex, many API calls |

### Verification

1. Run with large KB tenant (Iglesia ni Cristo, ~1.7MB)
2. Click "Generate Pairs"
3. Should complete without token limit error
4. Generated pairs should have `source_chunk_id` populated
5. "Test All" should find high recall (pairs match actual chunks)

### Token Budget Calculation

```
Max context: 131,072 tokens
System prompt: ~200 tokens
Per-chunk prompt overhead: ~100 tokens
Chunk content: ~400 tokens (after chunking limits)
Safety margin: 20%

Safe per-request: (131072 - 200) × 0.8 / (100 + 400) = ~210 chunks max

Conservative: 25 chunks × 2 pairs = 50 Q&A pairs
Aggressive: 100 chunks × 1 pair = 100 Q&A pairs
```

---

## 2026-01-27 ~08:30 - Add MiniLM-L12-v2 Embedding Model Option

**Status:** Planned
**Priority:** Medium
**Reason:** Provide higher quality embeddings option while maintaining same 384 dimensions for compatibility.

### Background

Currently supported embedding models:

| Model | Dimensions | Layers | Quality | Speed |
|-------|------------|--------|---------|-------|
| `all-MiniLM-L6-v2` | 384 | 6 | Good | Fast |
| `all-MiniLM-L12-v2` | 384 | 12 | Better | Slower |
| `all-mpnet-base-v2` | 768 | 12 | Best | Slowest |

**L12 vs L6:** L12 has 12 transformer layers (vs 6), providing better semantic understanding at ~2x compute cost. Same 384 dimensions = no reindexing needed when switching.

### Current State

The model is **already supported** in dimension detection:

```go
// embedding.go:87-90
case "sentence-transformers/all-MiniLM-L6-v2",
    "sentence-transformers/all-MiniLM-L12-v2",  // ← Already here
    "sentence-transformers/paraphrase-MiniLM-L6-v2":
    return 384
```

**Current usage via environment:**
```bash
EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L12-v2 task run:rag
```

### What's Missing

1. **No UI selector** - Users can't choose embedding model in Agent Admin
2. **No Taskfile shortcut** - No `task run:rag:l12` command
3. **No documentation** - Model options not documented

### Implementation Plan

#### Option A: Taskfile Shortcut (Quick Win)

Add new Taskfile command for L12 model:

```yaml
# Taskfile.yml
run:rag:cybertron:l12:
  desc: "Run with Cybertron L12 embeddings (higher quality)"
  cmds:
    - |
      RAG_PIPELINE_ENABLED=true \
      EMBEDDING_PROVIDER=cybertron \
      EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L12-v2 \
      DATA_DIR={{.ROOT_DIR}}/build/data \
      ./build/memos --mode dev
```

#### Option B: Per-Tenant Embedding Config (Full Solution)

Add embedding model selection to Agent Admin UI:

**1. Database Schema**
```sql
-- Add to tenant_config table
ALTER TABLE tenant_config ADD COLUMN embedding_model TEXT;
```

**2. Backend Changes**
```go
// store/agent.go - TenantConfig struct
type TenantConfig struct {
    // ... existing fields
    EmbeddingModel string `json:"embedding_model"` // NEW
}

// service.go - Use tenant's embedding model
func (s *Service) getEmbeddingModel(tenantID int32) string {
    config, _ := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &tenantID})
    if config != nil && config.EmbeddingModel != "" {
        return config.EmbeddingModel
    }
    return s.defaultEmbeddingModel // From env
}
```

**3. Frontend Changes**
```tsx
// AgentAdmin.tsx - Add embedding model selector
<Select
  value={embeddingModel}
  onChange={(_, v) => setEmbeddingModel(v)}
>
  <Option value="sentence-transformers/all-MiniLM-L6-v2">
    MiniLM-L6 (Fast, Good Quality)
  </Option>
  <Option value="sentence-transformers/all-MiniLM-L12-v2">
    MiniLM-L12 (Slower, Better Quality)
  </Option>
  <Option value="sentence-transformers/all-mpnet-base-v2">
    MPNet (Slowest, Best Quality) - 768 dim
  </Option>
</Select>
```

**Note:** Changing embedding model requires "Rebuild Index" to re-embed all content.

### Recommended Approach

**Phase 1 (Immediate):** Add Taskfile shortcut - zero code changes
**Phase 2 (Later):** Add UI selector if per-tenant embedding needed

### Files to Modify

| File | Change |
|------|--------|
| `Taskfile.yml` | Add `run:rag:cybertron:l12` command |
| `docs/DOCS_ENV_VAR.MD` | Document `EMBEDDING_MODEL` options |
| `docs/DOCS_TASKFILE.MD` | Document new task command |

### Verification

```bash
# Test L12 model
task run:rag:cybertron:l12

# Check logs show correct model
# "Loading Cybertron embedding model" model=sentence-transformers/all-MiniLM-L12-v2

# Rebuild index for a tenant
# Compare retrieval quality vs L6
```

### Model Comparison Reference

| Metric | L6 | L12 | MPNet |
|--------|----|----|-------|
| Dimensions | 384 | 384 | 768 |
| Parameters | 22M | 33M | 109M |
| Speed (relative) | 1x | 0.5x | 0.3x |
| STS Benchmark | 0.78 | 0.79 | 0.83 |
| Memory | ~90MB | ~130MB | ~420MB |

---

## 2026-01-27 ~09:00 - Batched Reindex for Large Files

**Status:** Implemented
**Priority:** High
**Reason:** Rebuild Index fails for large KB/POLICY files due to memory issues and API timeouts when processing all chunks at once.

### Problem

The `Insert()` function in `vectordb_lance.go` processed all chunks in a single batch:

```go
// OLD: All chunks processed at once
func (db *LanceVectorDB) Insert(ctx context.Context, chunks []DocumentChunk) error {
    // Generate embeddings for ALL chunks at once
    embeddings, err := db.embedSvc.Embed(ctx, allTexts)  // ← Memory spike + API timeout

    // Build one giant Arrow record
    record, err := db.chunksToArrowRecord(chunks)  // ← Memory spike

    // Single insert
    db.table.Add(ctx, record, nil)
}
```

**Issues with large files:**
1. **Memory spikes** - Building Arrow record for 1000+ chunks at once
2. **API timeouts** - OpenRouter embedding API times out on large batches
3. **No progress visibility** - User sees no feedback during long operations
4. **All-or-nothing** - Single failure loses all progress

### Solution: Batched Processing

Process chunks in batches of 100 with progress logging:

```go
// NEW: Process in batches
func (db *LanceVectorDB) Insert(ctx context.Context, chunks []DocumentChunk) error {
    const batchSize = 100

    for batchStart := 0; batchStart < len(chunks); batchStart += batchSize {
        batch := chunks[batchStart:min(batchStart+batchSize, len(chunks))]

        // 1. Generate embeddings for this batch only
        embeddings, err := db.embedSvc.Embed(ctx, batchTexts)

        // 2. Build Arrow record for this batch
        record, err := db.chunksToArrowRecord(batch)

        // 3. Insert batch
        db.table.Add(ctx, record, nil)
        record.Release()

        // 4. Log progress
        slog.Info("Processing batch",
            "batch", batchNum,
            "totalBatches", totalBatches,
            "progress", fmt.Sprintf("%d/%d", processed, total))
    }
}
```

### Implementation Details

**Batch size:** 100 chunks per batch
- Small enough to avoid memory issues
- Large enough for efficient embedding API calls
- Matches OpenRouter's recommended batch size

**Progress logging:**
```
INFO Processing batch batch=1 totalBatches=15 chunksInBatch=100 progress=100/1487
INFO Processing batch batch=2 totalBatches=15 chunksInBatch=100 progress=200/1487
...
INFO Completed batched insert totalChunks=1487
```

**Dimension mismatch check:** Moved to start of function using first chunk as sample, before processing all batches. This prevents wasted work if table needs recreation.

### Files Modified

| File | Change |
|------|--------|
| `server/router/api/v1/agent/vectordb_lance.go` | Rewrite `Insert()` to use batched processing |

### Key Code Changes

```go
// vectordb_lance.go - Insert function

// Batch size for embedding and insertion (handles large files)
const batchSize = 100

totalChunks := len(chunks)
slog.Info("Starting batched insert", "totalChunks", totalChunks, "batchSize", batchSize)

// Check dimension mismatch BEFORE processing (using first chunk as sample)
// This avoids wasting API calls if table needs recreation

// Process chunks in batches
for batchStart := 0; batchStart < totalChunks; batchStart += batchSize {
    batchEnd := batchStart + batchSize
    if batchEnd > totalChunks {
        batchEnd = totalChunks
    }

    batch := chunks[batchStart:batchEnd]
    batchNum := (batchStart / batchSize) + 1
    totalBatches := (totalChunks + batchSize - 1) / batchSize

    slog.Info("Processing batch",
        "batch", batchNum,
        "totalBatches", totalBatches,
        "chunksInBatch", len(batch),
        "progress", fmt.Sprintf("%d/%d", batchEnd, totalChunks))

    // Generate embeddings for this batch
    // Build Arrow record for this batch
    // Add batch to table
    // Release record memory
}
```

### Benefits

1. **Memory efficient** - Only one batch in memory at a time
2. **API friendly** - Smaller embedding requests, less likely to timeout
3. **Progress visibility** - Server logs show batch progress
4. **Partial success** - Early batches committed even if later fails
5. **Scalable** - Can handle any size KB/POLICY file

### Verification

1. Upload a large KB file (1MB+)
2. Click "Rebuild Index"
3. Check server logs for batch progress messages
4. Index should complete successfully
5. Search should return results from all chunks

### Configuration

Batch size is hardcoded to 100. Future enhancement could make this configurable via environment variable:

```bash
# Future: Configure batch size
LANCEDB_INSERT_BATCH_SIZE=100  # Default
```

---

*Add new changes below with timestamp*
