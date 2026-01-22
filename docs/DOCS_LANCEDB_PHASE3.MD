# LanceDB RAG Implementation - Phase 3: Retrieval Pipeline

**Status:** Complete
**Date:** 2026-01-23
**Parent Document:** [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD)
**Previous Phase:** [DOCS_LANCEDB_PHASE2.MD](./DOCS_LANCEDB_PHASE2.MD)

---

## Overview

Phase 3 integrated RAG retrieval into the chat response generation flow. When RAG is enabled, the system retrieves relevant context from the vector database instead of including the full KB, resulting in more focused and efficient prompts.

---

## Files Modified

### 1. `server/router/api/v1/agent/service.go`

Added RAG pipeline methods and modified chat flow.

**New Methods:**

```go
// UseRAGPipeline determines if RAG pipeline should be used for this request.
// Returns true if vectorDB is initialized and not a no-op.
func (s *Service) UseRAGPipeline() bool {
    if s.vectorDB == nil {
        return false
    }
    _, isNoOp := s.vectorDB.(*NoOpVectorDB)
    return !isNoOp
}

// generateRAGResponse generates a response using RAG retrieval for context.
func (s *Service) generateRAGResponse(
    ctx context.Context,
    config *AudienceConfig,
    session *store.AgentSession,
    classification *Classification,
    decision *PolicyDecision,
    userMessage string,
) (string, error)

// buildRAGSystemPrompt constructs a system prompt using RAG-retrieved context.
func (s *Service) buildRAGSystemPrompt(
    config *AudienceConfig,
    session *store.AgentSession,
    classification *Classification,
    decision *PolicyDecision,
    retrieved *RetrievedContext,
) string
```

**Chat Flow Integration:**

```go
// In processChat() - switched based on RAG availability
if s.UseRAGPipeline() {
    slog.Debug("Using RAG pipeline for response generation")
    response, genErr = s.generateRAGResponse(ctx, config, session, classification, decision, userMessage)
} else {
    // Legacy: Use full KB in system prompt
    response, genErr = s.generateResponse(ctx, config, session, classification, decision, userMessage)
}
```

---

### 2. `server/router/api/v1/agent/vectordb.go`

Refactored to support build tags - common code only.

**Changes:**
- Removed LanceDB-specific imports
- Changed `NewLanceVectorDB` to lowercase `newLanceVectorDB` (factory function)
- Factory function is implemented in build-tag-specific files

---

### 3. `server/router/api/v1/agent/vectordb_lance.go` (NEW)

**Build Tag:** `//go:build rag`

Contains the full LanceDB implementation, only compiled when building with `-tags rag`.

**Contents:**
- LanceDB imports (lancedb-go, arrow)
- `LanceVectorDB` struct
- `newLanceVectorDB()` factory function
- All LanceDB methods (Insert, Delete, Search, etc.)
- Arrow record building utilities

---

### 4. `server/router/api/v1/agent/vectordb_nolance.go` (NEW)

**Build Tag:** `//go:build !rag`

Stub implementation for builds without RAG support.

```go
//go:build !rag

package agent

import "fmt"

// newLanceVectorDB is a stub when building without the rag tag.
func newLanceVectorDB(config *VectorDBConfig, embedSvc EmbeddingService) (VectorDB, error) {
    return nil, fmt.Errorf("LanceDB storage (local/s3) requires RAG build. Use 'task build:backend:rag' or set LANCEDB_STORAGE_PROVIDER=memory")
}
```

---

### 5. `Taskfile.yml`

Updated to include `-tags rag` for RAG builds.

```yaml
build:backend:rag:
  cmds:
    - mkdir -p build
    - go build -tags rag -o build/memos ./bin/memos/main.go

dev:backend:rag:
  cmds:
    - go run -tags rag ./bin/memos/main.go --mode dev
```

---

## Data Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Phase 3: Retrieval Pipeline                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  User sends message                                                 │
│         │                                                           │
│         ▼                                                           │
│  ┌─────────────────┐                                                │
│  │  processChat()  │  (service.go)                                  │
│  └────────┬────────┘                                                │
│           │                                                         │
│           ▼                                                         │
│  ┌─────────────────────┐                                            │
│  │ UseRAGPipeline()?   │                                            │
│  └────────┬────────────┘                                            │
│           │                                                         │
│     ┌─────┴─────┐                                                   │
│     │           │                                                   │
│   YES          NO                                                   │
│     │           │                                                   │
│     ▼           ▼                                                   │
│  ┌───────────────────┐  ┌───────────────────┐                       │
│  │generateRAGResponse│  │ generateResponse  │ (Legacy: full KB)     │
│  └─────────┬─────────┘  └───────────────────┘                       │
│            │                                                        │
│            ▼                                                        │
│  ┌────────────────────────┐                                         │
│  │ RetrieveContextForQuery│  (vectordb.go)                          │
│  └─────────┬──────────────┘                                         │
│            │                                                        │
│            ▼                                                        │
│  ┌────────────────────────┐                                         │
│  │ VectorDB.Search()      │                                         │
│  └─────────┬──────────────┘                                         │
│            │                                                        │
│            ▼                                                        │
│  ┌────────────────────────┐                                         │
│  │ buildRAGSystemPrompt() │  Build prompt with retrieved context    │
│  └─────────┬──────────────┘                                         │
│            │                                                        │
│            ▼                                                        │
│  ┌────────────────────────┐                                         │
│  │    LLM Generation      │                                         │
│  └────────────────────────┘                                         │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## RAG System Prompt Structure

The `buildRAGSystemPrompt()` function constructs a focused prompt:

```
## Identity
{Policy identity section}

## Tone Guidelines
{Policy tone section}

## Relevant Context
{Retrieved chunks organized by type: Services, FAQs, Coverage, etc.}

## Conversation Rules
{Policy rules - filtered based on classification}

## Response Guidelines
{Instructions for response format}
```

**Key Differences from Legacy Prompt:**
- Only includes retrieved chunks (not full KB)
- Chunks are pre-filtered by relevance score
- Organized by content type for clarity
- Smaller context window usage

---

## Intent-Aware Retrieval

The `RetrieveContextForQuery()` function adjusts search parameters based on detected intent:

| Intent | Content Types | TopK |
|--------|---------------|------|
| `service_inquiry`, `booking_request` | service, faq | 5 |
| `coverage_question` | coverage, service | 5 |
| `pricing_question` | faq, service | 3 |
| `emergency` | service, safety | 3 |
| `complaint`, `escalation` | rule, faq | 3 |
| (default) | faq, service, kb_section | 5 |

All searches use:
- `MinScore: 0.3` (relevance threshold)
- `ActiveOnly: true` (only active content)
- Tenant and audience filtering

---

## Build Configurations

### Standard Build (No RAG)

```bash
task build
# or
task build:backend
```

- Uses `vectordb_nolance.go` stub
- LanceDB storage returns error
- Memory storage still works (for testing)
- No CGO required

### RAG Build (With LanceDB)

```bash
task build:rag
# or
task build:backend:rag
```

- Uses `vectordb_lance.go` implementation
- Full LanceDB support (local and S3)
- Requires CGO and native library
- `-tags rag` flag passed to compiler

---

## Environment Configuration

```bash
# Enable RAG pipeline (required)
RAG_PIPELINE_ENABLED=true

# Storage provider (memory works with both builds)
LANCEDB_STORAGE_PROVIDER=memory|local|s3

# For local storage (RAG build only)
LANCEDB_LOCAL_PATH=build/data/lancedb

# For S3 storage (RAG build only)
LANCEDB_S3_ENDPOINT=fly.storage.tigris.dev
LANCEDB_S3_BUCKET=your-bucket
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...

# Embedding provider
EMBEDDING_PROVIDER=local|openrouter|mock
```

---

## Verification

### Build Tests

```bash
# Regular build (no RAG)
task build
# Success: builds without LanceDB dependencies

# RAG build (with LanceDB)
task build:backend:rag
# Success: builds with LanceDB support
```

### Runtime Verification

When RAG is enabled and working:

```
INFO Using RAG pipeline for response generation
INFO Retrieved context for RAG query=... chunks=5 latency=...
```

When RAG is disabled or unavailable:

```
INFO RAG pipeline disabled, using no-op vector database
```

---

## Fallback Behavior

The system degrades gracefully:

1. **RAG enabled, VectorDB available**: Uses RAG retrieval
2. **RAG enabled, VectorDB fails**: Falls back to legacy full KB prompt
3. **RAG disabled**: Uses legacy full KB prompt (no VectorDB initialized)
4. **Non-RAG build with local/s3 storage**: Error message, falls back to NoOpVectorDB

---

## Performance Considerations

### Token Usage

| Mode | Typical Prompt Size |
|------|---------------------|
| Legacy (full KB) | 10,000-50,000 tokens |
| RAG (retrieved context) | 1,000-5,000 tokens |

### Latency

| Component | Typical Latency |
|-----------|-----------------|
| Embedding generation | 50-200ms |
| Vector search | 10-50ms |
| LLM generation | 500-2000ms |

RAG adds ~100-300ms for retrieval but saves significantly on LLM tokens.

---

## Next Steps

Potential future enhancements:

1. **Hybrid Search**: Combine vector + BM25 keyword search
2. **Re-ranking**: Use cross-encoder for more accurate ranking
3. **Query Expansion**: Expand user queries for better recall
4. **Caching**: Cache frequent queries and embeddings
5. **Analytics**: Track retrieval quality metrics
