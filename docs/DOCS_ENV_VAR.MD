# Environment Variables Reference

**Status:** Active
**Date:** 2026-01-26
**Related Documents:**
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG pipeline configuration
- [DOCS_TASKFILE.MD](./DOCS_TASKFILE.MD) - Build and run commands
- [DOCS_DOCKERFILE.MD](./DOCS_DOCKERFILE.MD) - Docker deployment

---

## Overview

This document provides a comprehensive reference for all environment variables used by the application. Variables are organized by functional area.

---

## Quick Reference

| Category | Key Variables |
|----------|--------------|
| **Core** | `OPENROUTER_API_KEY`, `LLM_MODEL` |
| **Embedding** | `EMBEDDING_PROVIDER`, `EMBEDDING_MODEL`, `EMBEDDING_TIMEOUT` |
| **RAG** | `RAG_PIPELINE_ENABLED`, `REINDEX_RAG`, `HYBRID_SEARCH_ENABLED` |
| **Observational Memory** | `OM_ENABLED`, `OM_OBSERVER_TOKEN_THRESHOLD`, `OM_TOKEN_THRESHOLD` |
| **Hybrid OM + RAG** | `HYBRID_OM_RAG_ENABLED`, `HYBRID_OM_RAG_TEMPORAL_DECAY`, `HYBRID_OM_RAG_TTL_DAYS` |
| **Storage** | `LANCEDB_STORAGE_PROVIDER`, `LANCEDB_LOCAL_PATH` |

---

## Core Server Configuration

### OPENROUTER_API_KEY

**Required** for LLM functionality.

| Property | Value |
|----------|-------|
| Type | String |
| Default | (none - required) |
| Example | `sk-or-v1-xxxxxxxxxxxx` |

API key for OpenRouter service. Used for both chat completion and embeddings (when using openrouter provider).

```bash
export OPENROUTER_API_KEY=sk-or-v1-your-api-key-here
```

---

### LLM_MODEL

Model identifier for chat completion (LLM responses).

| Property | Value |
|----------|-------|
| Type | String |
| Default | `openai/gpt-4o-mini` |
| Examples | `openai/gpt-4o`, `anthropic/claude-3.5-sonnet` |

```bash
export LLM_MODEL=openai/gpt-4o-mini
```

**Available Models (via OpenRouter):**
- `openai/gpt-4o-mini` - Fast, cost-effective (default)
- `openai/gpt-4o` - More capable
- `anthropic/claude-3.5-sonnet` - Anthropic's model
- See [OpenRouter Models](https://openrouter.ai/models) for full list

---

### LLM_MODEL_REASONING

Model identifier for reasoning/generation tasks (e.g., auto-generating annotated KB.MD/POLICY.MD).

| Property | Value |
|----------|-------|
| Type | String |
| Default | `google/gemini-2.5-pro` |
| Examples | `google/gemini-2.5-pro`, `anthropic/claude-3.5-sonnet` |

```bash
export LLM_MODEL_REASONING=google/gemini-2.5-pro
```

**Use Cases:**
- Auto-generating annotated KB.MD from raw content
- Auto-generating annotated POLICY.MD from raw content
- Other tasks requiring longer context or advanced reasoning

**Note:** This model is used for server-side content generation tasks, not for chat responses. It should be a capable model with good instruction following.

---

## Configuration Priority (Tenant vs Environment)

Environment variables provide **global defaults**, but tenant-specific settings in the **Agent Admin page take priority**.

### LLM Model Resolution Order

```
1. Tenant Config (Agent Admin) → Highest priority
         ↓ (if empty)
2. LLM_MODEL env var → Fallback
         ↓ (if empty)
3. Hardcoded default → "openai/gpt-oss-120b:free"
```

**Code reference:** `service.go:390-411`

```go
// 1. Try tenant-specific config (Agent Admin page)
if config != nil && config.LLMModel != "" {
    model = config.LLMModel  // Takes priority
}

// 2. Fallback to environment variable
if model == "" {
    model = s.profile.LLMModel  // LLM_MODEL env var
}
```

### API Key Resolution Order

```
1. Tenant Config (Agent Admin, encrypted) → Highest priority
         ↓ (if empty)
2. OPENROUTER_API_KEY env var → Fallback
```

### Practical Usage

| Scenario | Configuration Approach |
|----------|----------------------|
| Single tenant, simple setup | Set `LLM_MODEL` and `OPENROUTER_API_KEY` env vars |
| Multi-tenant, shared API key | Set `OPENROUTER_API_KEY` env var, leave tenant configs empty |
| Multi-tenant, per-tenant keys | Configure each tenant in Agent Admin page |
| Mixed (some tenants custom) | Set env var defaults, override specific tenants in Agent Admin |

**Example: Global default with tenant override**
```bash
# Environment (global default)
export LLM_MODEL=openai/gpt-4o-mini

# Tenant "premium-corp" in Agent Admin:
# LLM Model: openai/gpt-4o  (overrides env var for this tenant only)
```

---

### ENCRYPTION_MASTER_KEY

Master key for encrypting tenant API keys (AES-256).

| Property | Value |
|----------|-------|
| Type | String (32 bytes, hex or base64) |
| Default | (none - optional) |
| Example | `0123456789abcdef0123456789abcdef` |

```bash
export ENCRYPTION_MASTER_KEY=your-32-byte-key-here
```

**Note:** If not set, tenant API keys are stored unencrypted.

---

## Embedding Configuration

### EMBEDDING_PROVIDER

Selects the embedding service provider.

| Property | Value |
|----------|-------|
| Type | String |
| Default | `openrouter` |
| Options | `openrouter`, `mock`, `local` |

```bash
export EMBEDDING_PROVIDER=openrouter
```

**Provider Comparison:**

| Provider | Dimensions | Cost | Use Case |
|----------|------------|------|----------|
| `openrouter` | 1536 | ~$0.02/1M tokens | **Recommended** - Production quality |
| `mock` | 384 | Free | Testing pipeline flow (no semantic accuracy) |
| `local` | 384 | Free | Development with Python embedding server |

---

### EMBEDDING_MODEL

Model name for generating embeddings.

| Property | Value |
|----------|-------|
| Type | String |
| Default | Varies by provider (see below) |

**Defaults by Provider:**

| Provider | Default Model | Dimensions |
|----------|---------------|------------|
| `openrouter` | `openai/text-embedding-3-small` | 1536 |
| `local` | `all-MiniLM-L6-v2` | 384 |
| `mock` | N/A (random vectors) | 384 |

```bash
# For OpenRouter (recommended)
export EMBEDDING_MODEL=openai/text-embedding-3-small

# For local sentence-transformers Python server
export EMBEDDING_MODEL=all-MiniLM-L6-v2
```

**Available OpenRouter Embedding Models:**
- `openai/text-embedding-3-small` - 1536 dimensions (default, recommended)
- `openai/text-embedding-3-large` - 3072 dimensions (higher quality)
- `openai/text-embedding-ada-002` - 1536 dimensions (legacy)

---

### EMBEDDING_LOCAL_ENDPOINT

URL for local embedding server (when using `local` provider).

| Property | Value |
|----------|-------|
| Type | URL |
| Default | `http://localhost:8001/embed` |

```bash
export EMBEDDING_LOCAL_ENDPOINT=http://localhost:8001/embed
```

**Starting the local embedding server:**
```bash
task embedding:server
```

---

### EMBEDDING_TIMEOUT

HTTP timeout for embedding API requests.

| Property | Value |
|----------|-------|
| Type | Duration string |
| Default | `180s` |
| Examples | `60s`, `180s`, `5m` |

```bash
export EMBEDDING_TIMEOUT=180s
```

**Format:** Accepts Go duration strings (e.g., `60s`, `3m`, `1h30m`).

**Use cases:**
- Increase for large batch operations with slow network
- Decrease for faster failure detection in testing

**Note:** Added in 2026-01-30 to fix timeout issues with large KB content during "Rebuild Index" operations. The previous hardcoded 60s timeout was insufficient for large batches.

---

## RAG Pipeline Configuration

### RAG_PIPELINE_ENABLED

Master switch to enable/disable the RAG pipeline.

| Property | Value |
|----------|-------|
| Type | Boolean |
| Default | `false` |
| Options | `true`, `false` |

```bash
export RAG_PIPELINE_ENABLED=true
```

When enabled:
- KB and Policy content is chunked and indexed
- Chat queries retrieve relevant context
- System prompts include retrieved information

---

### REINDEX_RAG

Triggers re-indexing of all existing content on startup.

| Property | Value |
|----------|-------|
| Type | Boolean |
| Default | `false` |
| Options | `true`, `false` |

```bash
export REINDEX_RAG=true
```

**Use cases:**
- After changing embedding providers
- After fresh deployment with existing database
- After changing embedding dimensions

**Note:** Set back to `false` after successful reindex.

---

### HYBRID_SEARCH_ENABLED

Enables hybrid search combining vector similarity with BM25 keyword matching.

| Property | Value |
|----------|-------|
| Type | Boolean |
| Default | `false` |
| Options | `true`, `false` |

```bash
export HYBRID_SEARCH_ENABLED=true
```

When enabled, RAG queries use both:
- **Vector similarity** - Semantic understanding of query meaning
- **BM25 keyword matching** - Exact term matching for precise results

**Benefits:**
- Better recall for exact keyword matches
- Improved handling of technical terms and proper nouns
- Combines semantic understanding with keyword precision

---

### HYBRID_VECTOR_WEIGHT

Weight for vector similarity in hybrid search score fusion.

| Property | Value |
|----------|-------|
| Type | Float |
| Default | `0.7` |
| Range | `0.0` to `1.0` |

```bash
export HYBRID_VECTOR_WEIGHT=0.7
```

Higher values favor semantic similarity over keyword matching.

---

### HYBRID_TEXT_WEIGHT

Weight for BM25 text matching in hybrid search score fusion.

| Property | Value |
|----------|-------|
| Type | Float |
| Default | `0.3` |
| Range | `0.0` to `1.0` |

```bash
export HYBRID_TEXT_WEIGHT=0.3
```

Higher values favor exact keyword matches over semantic similarity.

**Note:** `HYBRID_VECTOR_WEIGHT` + `HYBRID_TEXT_WEIGHT` should sum to 1.0 for normalized scores.

---

### LLM_VERIFIER_ENABLED

Enables LLM-based response verification (semantic grounding against KB).

| Property | Value |
|----------|-------|
| Type | Boolean |
| Default | `false` |
| Options | `true`, `false` |

```bash
export LLM_VERIFIER_ENABLED=true
```

When enabled, responses are verified against KB content before being sent.

**Note:** This is now the primary semantic verification layer. The regex-based compliance checker was removed in Phase 5B. The output sanitizer (always active) handles concrete placeholder patterns, while the LLM verifier provides semantic grounding checks.

**When to enable:**
- High-stakes deployments needing extra validation
- Debugging response quality issues
- Production environments where accuracy is critical

---

## Observational Memory Configuration

Observational Memory (OM) gives agents long-term memory by compressing conversation history into an observation log. See [DOCS_OM_ANALYSIS.MD](./DOCS_OM_ANALYSIS.MD) for details.

### OM_ENABLED

Enables or disables the Observational Memory system.

| Property | Value |
|----------|-------|
| Type | Boolean |
| Default | `true` |

```bash
export OM_ENABLED=true
```

### OM_MESSAGE_THRESHOLD

> **Deprecated:** Use `OM_OBSERVER_TOKEN_THRESHOLD` instead for token-based triggering.

Number of new messages required before triggering the Observer. Helps reduce API calls by only observing after a batch of messages.

| Property | Value |
|----------|-------|
| Type | Integer |
| Default | `10` |

```bash
export OM_MESSAGE_THRESHOLD=10
```

### OM_OBSERVER_TOKEN_THRESHOLD

Token count threshold for triggering the Observer. When unobserved messages exceed this token count, the Observer runs to compress them into observations. This aligns with Mastra's OM specification for consistent triggering regardless of message verbosity.

| Property | Value |
|----------|-------|
| Type | Integer |
| Default | `30000` |

```bash
export OM_OBSERVER_TOKEN_THRESHOLD=30000
```

### OM_TOKEN_THRESHOLD

Token count threshold for triggering the Reflector to compress the observation log.

| Property | Value |
|----------|-------|
| Type | Integer |
| Default | `2000` |

```bash
export OM_TOKEN_THRESHOLD=2000
```

### OM_RETRY_ATTEMPTS

Number of retry attempts for LLM calls in the Observer/Reflector.

| Property | Value |
|----------|-------|
| Type | Integer |
| Default | `3` |

```bash
export OM_RETRY_ATTEMPTS=3
```

### OM_RETRY_DELAY_MS

Delay in milliseconds between retry attempts.

| Property | Value |
|----------|-------|
| Type | Integer |
| Default | `1000` |

```bash
export OM_RETRY_DELAY_MS=1000
```

---

## Hybrid OM + RAG Configuration

Hybrid OM + RAG combines Observational Memory with Retrieval-Augmented Generation for unlimited memory capacity. See [DOCS_OM_RAG_IMP.MD](./DOCS_OM_RAG_IMP.MD) for implementation details.

### HYBRID_OM_RAG_ENABLED

Enables the Hybrid OM + RAG mode for unlimited memory capacity.

| Property | Value |
|----------|-------|
| Type | Boolean |
| Default | `false` |

```bash
export HYBRID_OM_RAG_ENABLED=true
```

When enabled:
- Observations are indexed to the vector database
- RAG retrieval supplements in-context observations
- Temporal weighting prioritizes recent observations
- Fusion engine combines OM and RAG results

---

### HYBRID_OM_RAG_VECTOR_WEIGHT

Weight for vector similarity in hybrid search score fusion.

| Property | Value |
|----------|-------|
| Type | Float |
| Default | `0.7` |
| Range | `0.0` to `1.0` |

```bash
export HYBRID_OM_RAG_VECTOR_WEIGHT=0.7
```

Higher values favor semantic similarity over keyword matching.

---

### HYBRID_OM_RAG_TEXT_WEIGHT

Weight for BM25 text matching in hybrid search score fusion.

| Property | Value |
|----------|-------|
| Type | Float |
| Default | `0.3` |
| Range | `0.0` to `1.0` |

```bash
export HYBRID_OM_RAG_TEXT_WEIGHT=0.3
```

Higher values favor exact keyword matches over semantic similarity.

**Note:** `HYBRID_OM_RAG_VECTOR_WEIGHT` + `HYBRID_OM_RAG_TEXT_WEIGHT` should sum to 1.0 for normalized scores.

---

### HYBRID_OM_RAG_TEMPORAL_DECAY

Temporal decay factor for age-based relevance scoring.

| Property | Value |
|----------|-------|
| Type | Float |
| Default | `0.1` |
| Range | `0.0` to `1.0` |

```bash
export HYBRID_OM_RAG_TEMPORAL_DECAY=0.1
```

Controls how quickly observation relevance decays with age:
- Day 1: Full weight (1.0)
- Days 1-7: Linear decay from 1.0 to 0.3
- Days 7-30: Slower decay from 0.3 to 0.1
- Day 30+: Minimum weight (0.1)

---

### HYBRID_OM_RAG_MAX_RETRIEVED_TOKENS

Maximum tokens to retrieve from RAG for context building.

| Property | Value |
|----------|-------|
| Type | Integer |
| Default | `30000` |

```bash
export HYBRID_OM_RAG_MAX_RETRIEVED_TOKENS=30000
```

Controls the maximum size of retrieved context. Higher values provide more context but increase token costs.

---

### HYBRID_OM_RAG_MIN_SCORE

Minimum relevance score for including observations in results.

| Property | Value |
|----------|-------|
| Type | Float |
| Default | `0.1` |
| Range | `0.0` to `1.0` |

```bash
export HYBRID_OM_RAG_MIN_SCORE=0.1
```

Observations with scores below this threshold are excluded. Higher values reduce noise but may miss relevant content.

---

### HYBRID_OM_RAG_INDEX_OBSERVATIONS

Enables automatic indexing of observations to the RAG system.

| Property | Value |
|----------|-------|
| Type | Boolean |
| Default | `true` |

```bash
export HYBRID_OM_RAG_INDEX_OBSERVATIONS=true
```

When enabled, the Observer automatically indexes new observations to the vector database after persisting them.

---

### HYBRID_OM_RAG_COMPRESSION

Enables compression for RAG storage.

| Property | Value |
|----------|-------|
| Type | Boolean |
| Default | `true` |

```bash
export HYBRID_OM_RAG_COMPRESSION=true
```

When enabled, observations are compressed before storage to reduce disk usage.

---

### HYBRID_OM_RAG_TTL_DAYS

Time-to-live for observations in days. Observations older than this are eligible for deletion.

| Property | Value |
|----------|-------|
| Type | Integer |
| Default | `90` |

```bash
export HYBRID_OM_RAG_TTL_DAYS=90
```

Set to `0` to disable automatic expiration. Default of 90 days balances memory retention with storage efficiency.

---

## Vector Database (LanceDB) Configuration

### LANCEDB_STORAGE_PROVIDER

Selects the vector database storage backend.

| Property | Value |
|----------|-------|
| Type | String |
| Default | `memory` |
| Options | `memory`, `local`, `s3` |

```bash
export LANCEDB_STORAGE_PROVIDER=local
```

**Storage Comparison:**

| Provider | Persistence | Build Required | Use Case |
|----------|-------------|----------------|----------|
| `memory` | None (lost on restart) | No | Development, testing |
| `local` | Disk | Yes (`-tags rag`) | Single-server production |
| `s3` | S3/Tigrisdata | Yes (`-tags rag`) | Multi-server production |

---

### LANCEDB_LOCAL_PATH

Directory path for local LanceDB storage.

| Property | Value |
|----------|-------|
| Type | Path |
| Default | `build/data/lancedb` |

```bash
export LANCEDB_LOCAL_PATH=/data/lancedb
```

**Note:** Only used when `LANCEDB_STORAGE_PROVIDER=local`.

---

### LANCEDB_S3_ENDPOINT

S3-compatible endpoint URL.

| Property | Value |
|----------|-------|
| Type | URL (hostname only) |
| Default | `fly.storage.tigris.dev` |

```bash
export LANCEDB_S3_ENDPOINT=fly.storage.tigris.dev
```

**Note:** Only used when `LANCEDB_STORAGE_PROVIDER=s3`.

---

### LANCEDB_S3_BUCKET

S3 bucket name for vector storage.

| Property | Value |
|----------|-------|
| Type | String |
| Default | (none - required for s3) |

```bash
export LANCEDB_S3_BUCKET=my-app-vectors
```

---

### LANCEDB_S3_REGION

S3 region identifier.

| Property | Value |
|----------|-------|
| Type | String |
| Default | `auto` |

```bash
export LANCEDB_S3_REGION=auto
```

---

### AWS_ACCESS_KEY_ID

S3 access key for authentication.

| Property | Value |
|----------|-------|
| Type | String |
| Default | (none - required for s3) |

```bash
export AWS_ACCESS_KEY_ID=tid_xxxxxxxxxxxxx
```

---

### AWS_SECRET_ACCESS_KEY

S3 secret key for authentication.

| Property | Value |
|----------|-------|
| Type | String |
| Default | (none - required for s3) |

```bash
export AWS_SECRET_ACCESS_KEY=tsec_xxxxxxxxxxxxx
```

---

## Configuration Presets

### Development with OpenRouter (Recommended)

Production-quality embeddings via API.

```bash
export OPENROUTER_API_KEY=sk-or-v1-your-key
export LLM_MODEL=openai/gpt-4o-mini
export RAG_PIPELINE_ENABLED=true
export LANCEDB_STORAGE_PROVIDER=local
export EMBEDDING_PROVIDER=openrouter
export EMBEDDING_MODEL=openai/text-embedding-3-small
```

Or simply run:
```bash
task run:rag
```

### Development (Mock - Pipeline Testing Only)

No external dependencies, but NO semantic accuracy.

```bash
export OPENROUTER_API_KEY=sk-or-v1-your-key
export LLM_MODEL=openai/gpt-4o-mini
export RAG_PIPELINE_ENABLED=true
export LANCEDB_STORAGE_PROVIDER=memory
export EMBEDDING_PROVIDER=mock
```

### Development (Python Embedding Server)

Real semantic search, requires local Python embedding server.

```bash
export OPENROUTER_API_KEY=sk-or-v1-your-key
export LLM_MODEL=openai/gpt-4o-mini
export RAG_PIPELINE_ENABLED=true
export LANCEDB_STORAGE_PROVIDER=memory
export EMBEDDING_PROVIDER=local
export EMBEDDING_LOCAL_ENDPOINT=http://localhost:8001/embed
```

### Production (Local Storage)

Persistent vectors on disk, requires RAG build.

```bash
export OPENROUTER_API_KEY=sk-or-v1-your-key
export LLM_MODEL=openai/gpt-4o-mini
export RAG_PIPELINE_ENABLED=true
export LANCEDB_STORAGE_PROVIDER=local
export LANCEDB_LOCAL_PATH=/data/lancedb
export EMBEDDING_PROVIDER=openrouter
export EMBEDDING_MODEL=openai/text-embedding-3-small
```

### Production (fly.io with Tigrisdata)

Distributed storage, requires RAG build.

```bash
export OPENROUTER_API_KEY=sk-or-v1-your-key
export LLM_MODEL=openai/gpt-4o-mini
export RAG_PIPELINE_ENABLED=true
export LANCEDB_STORAGE_PROVIDER=s3
export LANCEDB_S3_ENDPOINT=fly.storage.tigris.dev
export LANCEDB_S3_BUCKET=your-bucket-name
export LANCEDB_S3_REGION=auto
export AWS_ACCESS_KEY_ID=tid_xxxxxxxxxxxxx
export AWS_SECRET_ACCESS_KEY=tsec_xxxxxxxxxxxxx
export EMBEDDING_PROVIDER=openrouter
export EMBEDDING_MODEL=openai/text-embedding-3-small
```

---

## Environment File Example

Create a `.env` file for local development:

```bash
# .env (do not commit to version control)

# Core
OPENROUTER_API_KEY=sk-or-v1-your-api-key

# LLM
LLM_MODEL=openai/gpt-4o-mini

# RAG Pipeline
RAG_PIPELINE_ENABLED=true
LANCEDB_STORAGE_PROVIDER=local
LANCEDB_LOCAL_PATH=build/data/lancedb

# Embeddings (OpenRouter recommended)
EMBEDDING_PROVIDER=openrouter
EMBEDDING_MODEL=openai/text-embedding-3-small
```

Load with:
```bash
source .env && ./build/memos --mode dev
```

Or use Taskfile commands which set env vars automatically:
```bash
task run:rag  # Recommended
```

---

## Local vs Production Environment Variables

### IMPORTANT: Sync .env to fly.io Before Deployment

The `.env` file is for **LOCAL DEVELOPMENT ONLY**. Before deploying to fly.io, you must ensure all required variables are reflected in fly.io configuration.

| Environment | How Variables Are Set |
|-------------|----------------------|
| **Local development** | `.env` file (not committed to git) |
| **fly.io (non-sensitive)** | `fly.toml [env]` section |
| **fly.io (sensitive)** | `fly secrets set VAR=value` |

### Pre-Deployment Sync Checklist

Run the sync check task before deploying:

```bash
task fly:check
```

This will:
- List all variables in your local `.env`
- List variables in `fly.toml [env]`
- List `fly secrets`
- Report any `.env` variables missing from fly.io config

### Variable Placement Guide

| Variable Type | Local (.env) | fly.io Location |
|---------------|--------------|-----------------|
| `OPENROUTER_API_KEY` | `.env` | `fly secrets set` |
| `ENCRYPTION_MASTER_KEY` | `.env` | `fly secrets set` |
| `LLM_MODEL` | `.env` | `fly.toml [env]` |
| `EMBEDDING_MODEL` | `.env` | `fly.toml [env]` |
| `EMBEDDING_TIMEOUT` | `.env` | `fly.toml [env]` |
| `RAG_PIPELINE_ENABLED` | `.env` | `fly.toml [env]` |
| `AWS_ACCESS_KEY_ID` | `.env` | `fly secrets set` (auto-set by `fly storage create`) |

### Adding a New Variable Workflow

```bash
# Step 1: Add to .env and test locally
echo "NEW_VAR=value" >> .env
source .env
task run:rag  # Verify locally

# Step 2: Add to fly.io (choose based on sensitivity)

# Option A: Non-sensitive → Edit fly.toml
#   [env]
#     NEW_VAR = "value"

# Option B: Sensitive → Use fly secrets
fly secrets set NEW_VAR=value

# Step 3: Deploy and verify
fly deploy
fly ssh console -C "env | grep NEW_VAR"
```

See [DOCS_DEPLOYMENT.MD](./DOCS_DEPLOYMENT.MD) for complete deployment procedures.

---

## Troubleshooting

### "OPENROUTER_API_KEY not configured"

The API key is missing or empty.

```bash
export OPENROUTER_API_KEY=sk-or-v1-your-key
```

### "OPENROUTER_API_KEY required for OpenRouter embedding provider"

Using `EMBEDDING_PROVIDER=openrouter` but API key is not set.

```bash
export OPENROUTER_API_KEY=sk-or-v1-your-key
```

### "RAG pipeline disabled, using no-op vector database"

RAG is not enabled.

```bash
export RAG_PIPELINE_ENABLED=true
```

### "LanceDB storage requires RAG build"

Using `local` or `s3` storage without the RAG build tag.

Either use memory storage:
```bash
export LANCEDB_STORAGE_PROVIDER=memory
```

Or rebuild with RAG support:
```bash
task build:backend:rag
```

---

*Document Version: 1.5*
*Updated: 2026-02-14* - Added Hybrid OM + RAG configuration variables

## Taskfile Commands & RAG Configuration

When running RAG tasks, it is critical to understand the difference between production/dev running and testing/reindexing.

### `run:rag` vs `run:testrag`

| Command | Purpose | Configuration Source | Reindexes Content? |
|---------|---------|----------------------|--------------------|
| `task run:rag` | Standard development run | **Strictly `.env` file** (unless fixed in Taskfile) | No (uses existing index) |
| `task run:testrag` | Testing & Verification | **Explicitly overrides** env vars | **YES** (Delete & Rebuild) |

**What is "wrong" with `run:rag`?**
- It relies entirely on your local `.env` file.
- If `RAG_PIPELINE_ENABLED=true` is missing from `.env`, it defaults to `false` and RAG is **silently disabled**.
- It does not reindex. If you change embedding models or batch sizes, existing embeddings may be incompatible or cause errors until you manually reindex.

**Why `run:testrag` works reliably:**
- It explicitly sets `RAG_PIPELINE_ENABLED=true`, `EMBEDDING_BATCH_SIZE=10`, and `EMBEDDING_MODEL` inline.
- It sets `REINDEX_RAG=true`, forcing a fresh index build on startup, which validates the entire pipeline.

