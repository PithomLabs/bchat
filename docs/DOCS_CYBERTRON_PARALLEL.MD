# Plan: Parallel Cybertron Embeddings

**Date:** 2026-01-27
**Status:** Implemented
**Related Documents:**
- [DOCS_CYBERTRON.MD](./DOCS_CYBERTRON.MD) - Cybertron implementation

---

## Problem

Cybertron embedding generation is slow because texts are processed sequentially:

```go
// Current implementation - SEQUENTIAL
for i, text := range texts {
    result, err := e.model.Encode(ctx, text, 1)  // ~0.5s per text
    embeddings[i] = result
}
```

**Performance:**
- ~0.5 seconds per embedding on CPU
- 100 chunks = ~50 seconds
- No utilization of multiple CPU cores

---

## Solution: Worker Pool Pattern

Use goroutines with a worker pool to process multiple embeddings concurrently.

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Input: []string                       │
└─────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│                     Job Channel                          │
│              (index, text) tuples                        │
└─────────────────────────────────────────────────────────┘
          │           │           │           │
          ▼           ▼           ▼           ▼
     ┌────────┐  ┌────────┐  ┌────────┐  ┌────────┐
     │Worker 1│  │Worker 2│  │Worker 3│  │Worker N│
     │Encode()│  │Encode()│  │Encode()│  │Encode()│
     └────────┘  └────────┘  └────────┘  └────────┘
          │           │           │           │
          ▼           ▼           ▼           ▼
┌─────────────────────────────────────────────────────────┐
│                   Result Channel                         │
│            (index, embedding) tuples                     │
└─────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│              Output: [][]float32 (ordered)               │
└─────────────────────────────────────────────────────────┘
```

### Expected Speedup

| Workers | Time for 100 chunks | Speedup |
|---------|---------------------|---------|
| 1 (current) | ~50 seconds | 1x |
| 4 | ~12.5 seconds | 4x |
| 8 | ~6.25 seconds | 8x |
| N | ~50/N seconds | Nx |

*Actual speedup depends on CPU cores and memory bandwidth.*

---

## Implementation Plan

### Step 1: Add Configuration

```go
// In EmbeddingConfig
type EmbeddingConfig struct {
    // ... existing fields ...
    CybertronWorkers int // Number of parallel workers (default: runtime.NumCPU())
}

// In NewEmbeddingConfigFromEnv
func NewEmbeddingConfigFromEnv() *EmbeddingConfig {
    workers := runtime.NumCPU()
    if envWorkers := os.Getenv("CYBERTRON_WORKERS"); envWorkers != "" {
        if w, err := strconv.Atoi(envWorkers); err == nil && w > 0 {
            workers = w
        }
    }
    // ...
}
```

### Step 2: Modify CybertronEmbedding Struct

```go
type CybertronEmbedding struct {
    model     textencoding.Interface
    modelName string
    dimension int
    modelsDir string
    workers   int  // NEW: number of parallel workers
}
```

### Step 3: Implement Parallel Embed()

```go
type embedJob struct {
    index int
    text  string
}

type embedResult struct {
    index     int
    embedding []float32
    err       error
}

func (e *CybertronEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    if len(texts) == 0 {
        return [][]float32{}, nil
    }

    // For small batches, use sequential processing (less overhead)
    if len(texts) <= e.workers {
        return e.embedSequential(ctx, texts)
    }

    return e.embedParallel(ctx, texts)
}

func (e *CybertronEmbedding) embedParallel(ctx context.Context, texts []string) ([][]float32, error) {
    const maxChars = 750

    jobs := make(chan embedJob, len(texts))
    results := make(chan embedResult, len(texts))

    // Start workers
    var wg sync.WaitGroup
    for w := 0; w < e.workers; w++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for job := range jobs {
                select {
                case <-ctx.Done():
                    results <- embedResult{index: job.index, err: ctx.Err()}
                    continue
                default:
                }

                text := job.text
                if len(text) > maxChars {
                    text = text[:maxChars]
                }

                result, err := e.model.Encode(ctx, text, 1)
                if err != nil {
                    results <- embedResult{index: job.index, err: err}
                    continue
                }

                // Convert to []float32
                vector := result.Vector.Data().F64()
                embedding := make([]float32, len(vector))
                for j, v := range vector {
                    embedding[j] = float32(v)
                }

                results <- embedResult{index: job.index, embedding: embedding}
            }
        }()
    }

    // Send jobs
    for i, text := range texts {
        jobs <- embedJob{index: i, text: text}
    }
    close(jobs)

    // Wait for workers to finish
    go func() {
        wg.Wait()
        close(results)
    }()

    // Collect results
    embeddings := make([][]float32, len(texts))
    var firstErr error

    for result := range results {
        if result.err != nil && firstErr == nil {
            firstErr = result.err
        }
        if result.embedding != nil {
            embeddings[result.index] = result.embedding
        }
    }

    if firstErr != nil {
        return nil, firstErr
    }

    return embeddings, nil
}

func (e *CybertronEmbedding) embedSequential(ctx context.Context, texts []string) ([][]float32, error) {
    // Original sequential implementation for small batches
    const maxChars = 750
    embeddings := make([][]float32, len(texts))

    for i, text := range texts {
        if len(text) > maxChars {
            text = text[:maxChars]
        }

        result, err := e.model.Encode(ctx, text, 1)
        if err != nil {
            return nil, fmt.Errorf("failed to encode text %d: %w", i, err)
        }

        vector := result.Vector.Data().F64()
        embedding := make([]float32, len(vector))
        for j, v := range vector {
            embedding[j] = float32(v)
        }
        embeddings[i] = embedding
    }

    return embeddings, nil
}
```

### Step 4: Thread Safety Verification

**Critical:** Verify that `e.model.Encode()` is thread-safe.

From Cybertron source analysis:
- The model weights are read-only after loading
- Each `Encode()` call creates its own computation graph
- Should be safe for concurrent use

**If NOT thread-safe**, alternative approach:
- Create a pool of model instances (one per worker)
- Trade memory for parallelism

```go
type CybertronEmbedding struct {
    models    []textencoding.Interface  // Pool of models
    modelPool chan textencoding.Interface
    // ...
}
```

---

## Files to Modify

| File | Change |
|------|--------|
| `server/router/api/v1/agent/embedding.go` | Add parallel processing to CybertronEmbedding |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CYBERTRON_WORKERS` | `runtime.NumCPU()` | Number of parallel embedding workers |

---

## Verification

### Test 1: Performance Benchmark

```bash
# Before (sequential)
time curl -X POST http://localhost:8081/api/v1/agent/test/reindex

# After (parallel with 4 workers)
CYBERTRON_WORKERS=4 task run:rag:cybertron
time curl -X POST http://localhost:8081/api/v1/agent/test/reindex
```

### Test 2: Correctness Verification

```go
// Unit test: parallel results match sequential results
func TestCybertronParallelCorrectness(t *testing.T) {
    texts := []string{"text1", "text2", "text3", ...}

    // Sequential
    e.workers = 1
    seqResults, _ := e.Embed(ctx, texts)

    // Parallel
    e.workers = 4
    parResults, _ := e.Embed(ctx, texts)

    // Compare
    for i := range texts {
        assert.Equal(t, seqResults[i], parResults[i])
    }
}
```

### Test 3: Memory Usage

Monitor memory during reindex to ensure parallel processing doesn't cause OOM:

```bash
# Watch memory while reindexing
watch -n 1 'ps -o rss,vsz,comm -p $(pgrep memos)'
```

---

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Model not thread-safe | Test first; if fails, use model pool |
| Memory exhaustion | Limit workers, add memory monitoring |
| Context cancellation | Handle ctx.Done() in workers |
| Result ordering | Use indexed jobs/results pattern |

---

## Alternative Approaches Considered

### 1. Batch API in Cybertron
- Cybertron doesn't expose batch encoding API
- Would require upstream changes

### 2. Multiple Model Instances
- Load N models, one per worker
- Pro: Guaranteed thread safety
- Con: N × memory usage (~500MB per model)

### 3. External GPU Service
- Use local ollama or text-embeddings-inference
- Pro: Much faster (GPU)
- Con: Additional dependency

---

## Summary

Implement worker pool pattern for parallel Cybertron embeddings:
1. Add `CYBERTRON_WORKERS` configuration
2. Implement parallel `Embed()` with goroutines
3. Maintain result ordering via indexed channels
4. Fall back to sequential for small batches
5. Verify thread safety of Cybertron model

Expected improvement: **4-8x speedup** on multi-core CPUs.
