# Cybertron Embedding Provider Implementation Plan

This document outlines the plan to implement a native Go embedding provider using Cybertron (github.com/nlpodyssey/cybertron) for zero-dependency local embeddings.

## Overview

**Goal:** Add a `cybertron` embedding provider option that runs entirely in Go without external dependencies (no Python, no separate servers, no CGO).

**Benefits:**
- Single binary deployment
- No external embedding server required
- No API costs
- Works offline
- Fast startup after initial model download

---

## Supported Models

| Model | Dimensions | Size | Quality | Speed |
|-------|------------|------|---------|-------|
| `sentence-transformers/all-MiniLM-L6-v2` | 384 | ~90MB | Good | Fast |
| `sentence-transformers/all-MiniLM-L12-v2` | 384 | ~130MB | Better | Medium |
| `sentence-transformers/all-mpnet-base-v2` | 768 | ~420MB | Best | Slower |
| `sentence-transformers/paraphrase-MiniLM-L6-v2` | 384 | ~90MB | Good | Fast |

**Default:** `all-MiniLM-L6-v2` (best balance of speed and quality)

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `EMBEDDING_PROVIDER` | `local` | Set to `cybertron` to use this provider |
| `EMBEDDING_MODEL` | `sentence-transformers/all-MiniLM-L6-v2` | Hugging Face model name |
| `CYBERTRON_MODELS_DIR` | `~/.cybertron/models` | Directory for downloaded models |
| `CYBERTRON_DOWNLOAD_POLICY` | `missing` | `missing` (auto-download) or `never` |

**Example:**
```bash
EMBEDDING_PROVIDER=cybertron \
EMBEDDING_MODEL=sentence-transformers/all-mpnet-base-v2 \
task run
```

---

## Implementation Plan

### Phase 1: Add Dependency

**File:** `go.mod`

```bash
go get github.com/nlpodyssey/cybertron
```

**Note:** Cybertron has sub-packages, we specifically need:
- `github.com/nlpodyssey/cybertron/pkg/tasks`
- `github.com/nlpodyssey/cybertron/pkg/tasks/textencoding`

---

### Phase 2: Implement CybertronEmbedding

**File:** `server/router/api/v1/agent/embedding.go`

Add new provider implementation:

```go
// ============================================================================
// CYBERTRON EMBEDDING SERVICE (Native Go, Zero Dependencies)
// ============================================================================

import (
    "github.com/nlpodyssey/cybertron/pkg/tasks"
    "github.com/nlpodyssey/cybertron/pkg/tasks/textencoding"
)

// CybertronEmbedding implements EmbeddingService using pure Go transformers.
type CybertronEmbedding struct {
    model     textencoding.Interface
    modelName string
    dimension int
    modelsDir string
}

// NewCybertronEmbedding creates a new Cybertron embedding service.
func NewCybertronEmbedding(config *EmbeddingConfig) (*CybertronEmbedding, error) {
    modelName := config.Model
    if modelName == "" {
        modelName = "sentence-transformers/all-MiniLM-L6-v2"
    }

    modelsDir := os.Getenv("CYBERTRON_MODELS_DIR")
    if modelsDir == "" {
        homeDir, _ := os.UserHomeDir()
        modelsDir = filepath.Join(homeDir, ".cybertron", "models")
    }

    downloadPolicy := os.Getenv("CYBERTRON_DOWNLOAD_POLICY")
    if downloadPolicy == "" {
        downloadPolicy = "missing"
    }

    // Determine dimension based on model
    dimension := getDimensionForModel(modelName)

    // Load model (downloads on first use if policy allows)
    model, err := tasks.Load[textencoding.Interface](&tasks.Config{
        ModelsDir:           modelsDir,
        ModelName:           modelName,
        DownloadPolicy:      tasks.DownloadPolicy(downloadPolicy),
        ConversionPolicy:    tasks.ConversionPolicyIfNeeded,
        ConversionPrecision: tasks.PrecisionF32,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to load Cybertron model %s: %w", modelName, err)
    }

    slog.Info("Cybertron embedding model loaded",
        "model", modelName,
        "dimension", dimension,
        "models_dir", modelsDir,
    )

    return &CybertronEmbedding{
        model:     model,
        modelName: modelName,
        dimension: dimension,
        modelsDir: modelsDir,
    }, nil
}

// Embed generates embeddings using Cybertron.
func (e *CybertronEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    if len(texts) == 0 {
        return [][]float32{}, nil
    }

    embeddings := make([][]float32, len(texts))

    for i, text := range texts {
        result, err := e.model.Encode(ctx, text, int(pooling.MeanPooling))
        if err != nil {
            return nil, fmt.Errorf("failed to encode text %d: %w", i, err)
        }

        // Convert []float64 to []float32
        embedding := make([]float32, len(result.Vector))
        for j, v := range result.Vector {
            embedding[j] = float32(v)
        }
        embeddings[i] = embedding
    }

    return embeddings, nil
}

// Dimension returns the embedding vector dimension.
func (e *CybertronEmbedding) Dimension() int {
    return e.dimension
}

// Provider returns "cybertron".
func (e *CybertronEmbedding) Provider() string {
    return "cybertron"
}

// getDimensionForModel returns the embedding dimension for known models.
func getDimensionForModel(modelName string) int {
    switch modelName {
    case "sentence-transformers/all-mpnet-base-v2":
        return 768
    case "sentence-transformers/all-MiniLM-L6-v2",
         "sentence-transformers/all-MiniLM-L12-v2",
         "sentence-transformers/paraphrase-MiniLM-L6-v2":
        return 384
    default:
        return 384 // Default for unknown models
    }
}
```

---

### Phase 3: Register Provider in Factory

**File:** `server/router/api/v1/agent/embedding.go`

Update `NewEmbeddingService()`:

```go
func NewEmbeddingService(config *EmbeddingConfig) (EmbeddingService, error) {
    switch config.Provider {
    case "openrouter":
        return NewOpenRouterEmbedding(config)
    case "mock":
        return NewMockEmbedding(config), nil
    case "cybertron":
        return NewCybertronEmbedding(config)
    case "local":
        return NewLocalEmbedding(config)
    default:
        // Default to cybertron for zero-dependency experience
        return NewCybertronEmbedding(config)
    }
}
```

---

### Phase 4: Update Configuration

**File:** `server/router/api/v1/agent/embedding.go`

Update `EmbeddingConfig` struct:

```go
type EmbeddingConfig struct {
    Provider         string // "cybertron", "openrouter", "local", "mock"
    Model            string // Model name (e.g., "sentence-transformers/all-MiniLM-L6-v2")
    Dimension        int    // Override dimension (optional)
    OpenRouterAPIKey string // For OpenRouter provider
    LocalEndpoint    string // For local provider
    CybertronDir     string // For cybertron provider - models directory
}
```

Update `NewEmbeddingConfig()`:

```go
func NewEmbeddingConfig() *EmbeddingConfig {
    return &EmbeddingConfig{
        Provider:         getEnvOrDefault("EMBEDDING_PROVIDER", "cybertron"),
        Model:            getEnvOrDefault("EMBEDDING_MODEL", "sentence-transformers/all-MiniLM-L6-v2"),
        Dimension:        getEnvIntOrDefault("EMBEDDING_DIMENSION", 0),
        OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
        LocalEndpoint:    getEnvOrDefault("LOCAL_EMBEDDING_ENDPOINT", "http://localhost:8001/embed"),
        CybertronDir:     os.Getenv("CYBERTRON_MODELS_DIR"),
    }
}
```

---

### Phase 5: Add Batch Processing (Optimization)

For large KB files, process embeddings in batches:

```go
const cybertronBatchSize = 32

// EmbedBatch generates embeddings in batches for better memory management.
func (e *CybertronEmbedding) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
    if len(texts) == 0 {
        return [][]float32{}, nil
    }

    embeddings := make([][]float32, len(texts))

    for i := 0; i < len(texts); i += cybertronBatchSize {
        end := i + cybertronBatchSize
        if end > len(texts) {
            end = len(texts)
        }

        batch := texts[i:end]
        for j, text := range batch {
            result, err := e.model.Encode(ctx, text, int(pooling.MeanPooling))
            if err != nil {
                return nil, fmt.Errorf("failed to encode text %d: %w", i+j, err)
            }

            embedding := make([]float32, len(result.Vector))
            for k, v := range result.Vector {
                embedding[k] = float32(v)
            }
            embeddings[i+j] = embedding
        }

        // Log progress for large batches
        if len(texts) > 100 && end%100 == 0 {
            slog.Debug("Embedding progress", "completed", end, "total", len(texts))
        }
    }

    return embeddings, nil
}
```

---

### Phase 6: Update Documentation

**File:** `docs/DOCS_ENV_VAR.MD`

Add Cybertron section:

```markdown
### Cybertron (Native Go Embeddings)

| Variable | Default | Description |
|----------|---------|-------------|
| `EMBEDDING_PROVIDER` | `cybertron` | Set to `cybertron` for native Go embeddings |
| `EMBEDDING_MODEL` | `sentence-transformers/all-MiniLM-L6-v2` | Model to use |
| `CYBERTRON_MODELS_DIR` | `~/.cybertron/models` | Model storage directory |
| `CYBERTRON_DOWNLOAD_POLICY` | `missing` | `missing` or `never` |

**Available Models:**
- `sentence-transformers/all-MiniLM-L6-v2` (384 dim, fast, recommended)
- `sentence-transformers/all-MiniLM-L12-v2` (384 dim, medium)
- `sentence-transformers/all-mpnet-base-v2` (768 dim, best quality)
```

---

### Phase 7: Add Taskfile Commands

**File:** `Taskfile.yml`

```yaml
  run:cybertron:
    desc: Run with Cybertron embeddings (native Go, no external server)
    cmds:
      - |
        EMBEDDING_PROVIDER=cybertron \
        EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L6-v2 \
        ./build/memos --mode dev --port 5230 --data {{.DATA_DIR}}
    deps:
      - build

  run:cybertron:quality:
    desc: Run with high-quality Cybertron embeddings (slower, better)
    cmds:
      - |
        EMBEDDING_PROVIDER=cybertron \
        EMBEDDING_MODEL=sentence-transformers/all-mpnet-base-v2 \
        ./build/memos --mode dev --port 5230 --data {{.DATA_DIR}}
    deps:
      - build
```

---

## File Changes Summary

| File | Changes |
|------|---------|
| `go.mod` | Add `github.com/nlpodyssey/cybertron` dependency |
| `server/router/api/v1/agent/embedding.go` | Add `CybertronEmbedding` implementation |
| `docs/DOCS_ENV_VAR.MD` | Document new environment variables |
| `Taskfile.yml` | Add `run:cybertron` commands |

---

## Testing Plan

### 1. Unit Test
```go
func TestCybertronEmbedding(t *testing.T) {
    config := &EmbeddingConfig{
        Provider: "cybertron",
        Model:    "sentence-transformers/all-MiniLM-L6-v2",
    }

    svc, err := NewCybertronEmbedding(config)
    require.NoError(t, err)

    embeddings, err := svc.Embed(context.Background(), []string{
        "water damage restoration",
        "flood cleanup services",
    })
    require.NoError(t, err)
    assert.Len(t, embeddings, 2)
    assert.Len(t, embeddings[0], 384)
}
```

### 2. Integration Test
```bash
# Start server with Cybertron
EMBEDDING_PROVIDER=cybertron task run

# Trigger reindex via UI or API
curl -X POST http://localhost:5230/api/v1/agent/inc/reindex

# Verify chunks were indexed
curl http://localhost:5230/api/v1/admin/rag/stats
```

### 3. Semantic Similarity Test
```bash
# Search should return semantically similar results
curl -X POST http://localhost:5230/api/v1/admin/rag/search \
  -H "Content-Type: application/json" \
  -d '{"query": "water damage", "tenant_id": 4, "limit": 5}'
```

---

## Rollout Plan

1. **Development:** Use `EMBEDDING_PROVIDER=cybertron` for local testing
2. **Staging:** Test with production-like KB content
3. **Production:**
   - Pre-download models to avoid first-request latency
   - Consider `all-mpnet-base-v2` for better quality if latency allows

---

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| First request slow (model download) | Pre-download in Docker build or startup script |
| Memory usage (~500MB per model) | Document requirements, use smaller model if needed |
| Model not supported | Validate model name on startup, fail fast with clear error |
| Cybertron library updates | Pin version in go.mod |

---

## Future Enhancements

1. **Model Preloading:** Download models at build time for faster cold starts
2. **GPU Support:** Cybertron can use GPU if available (needs CGO)
3. **Caching:** Cache embeddings for frequently-queried text
4. **Hybrid:** Fall back to OpenRouter if Cybertron fails

---

## Related Documentation

- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG architecture overview
- [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD) - Vector database implementation
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Environment variables reference
- [Cybertron GitHub](https://github.com/nlpodyssey/cybertron) - Library documentation
