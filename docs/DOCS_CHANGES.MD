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

*Add new changes below with timestamp*
