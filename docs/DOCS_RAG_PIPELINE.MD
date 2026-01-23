# RAG Pipeline Configuration Guide

**Status:** Active
**Date:** 2026-01-23
**Related Documents:**
- [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD) - RAG implementation overview
- [DOCS_DOCKERFILE.MD](./DOCS_DOCKERFILE.MD) - Docker deployment with RAG

---

## Overview

The RAG (Retrieval-Augmented Generation) pipeline enhances chat responses by retrieving relevant context from indexed KB/Policy content instead of including everything in the system prompt.

### Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        RAG Pipeline Flow                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  1. INDEXING (on file upload)                                       │
│     KB.MD / POLICY.MD                                               │
│           │                                                         │
│           ▼                                                         │
│     ┌──────────┐    ┌──────────────┐    ┌─────────────┐            │
│     │  Chunker │───▶│  Embedding   │───▶│  VectorDB   │            │
│     └──────────┘    │   Service    │    │   Insert    │            │
│                     └──────────────┘    └─────────────┘            │
│                                                                     │
│  2. RETRIEVAL (on chat message)                                     │
│     User Query                                                      │
│           │                                                         │
│           ▼                                                         │
│     ┌──────────────┐    ┌─────────────┐    ┌─────────────┐         │
│     │   Embedding  │───▶│  VectorDB   │───▶│  Top-K      │         │
│     │    Service   │    │   Search    │    │  Results    │         │
│     └──────────────┘    └─────────────┘    └─────────────┘         │
│                                                   │                 │
│                                                   ▼                 │
│                                         ┌─────────────────┐        │
│                                         │ Build Focused   │        │
│                                         │ System Prompt   │        │
│                                         └─────────────────┘        │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Storage Backends

The RAG pipeline supports three storage backends:

| Backend | Rebuild Required | Persistence | Use Case |
|---------|------------------|-------------|----------|
| `memory` | **No** | Lost on restart | Development, Testing |
| `local` | Yes (`-tags rag`) | Disk | Single-server production |
| `s3` | Yes (`-tags rag`) | S3/Tigrisdata | Multi-server production |

### Memory Storage (No Rebuild)

The `MemoryVectorDB` is a pure Go implementation that:
- Uses in-memory hash map storage
- Implements cosine similarity search
- Works with any embedding provider
- **Does NOT require LanceDB native libraries**

### Local/S3 Storage (Requires Rebuild)

The `LanceVectorDB` uses LanceDB native libraries and requires:
- CGO enabled
- LanceDB shared library (`.so`)
- Build with `-tags rag` flag

---

## Quick Start: Run RAG Without Rebuilding

**This is the fastest way to test RAG functionality.**

### Step 1: Set Environment Variables

```bash
export RAG_PIPELINE_ENABLED=true
export LANCEDB_STORAGE_PROVIDER=memory
export EMBEDDING_PROVIDER=mock
```

### Step 2: Start the Server

```bash
# If using existing binary
./build/memos --mode dev

# Or with go run
go run ./bin/memos/main.go --mode dev
```

### Step 3: Upload MD Files

Upload KB.MD, POLICY.MD, and SCRIPT.MD files through the Agent Admin UI. The files will be:
1. Parsed and stored in SQLite
2. Chunked into documents
3. Embedded using mock embeddings (random vectors)
4. Indexed in the in-memory vector database

### Step 4: Test Chat

Send a chat message. The system will:
1. Embed the query
2. Search for relevant chunks
3. Build a focused system prompt
4. Generate a response

### Verification

Check logs for:
```
INFO Using in-memory vector database for testing
INFO Indexed content for RAG tenantID=3 audience=internal chunks=25
INFO Using RAG pipeline for response generation
```

---

## Full RAG Setup: Production with LanceDB

For production use with persistent storage:

### Step 1: Build with RAG Support

```bash
# Download LanceDB native libraries
task setup:lancedb

# Build with RAG tag
task build:backend:rag
```

### Step 2: Configure Storage

**For Local Storage:**
```bash
export RAG_PIPELINE_ENABLED=true
export LANCEDB_STORAGE_PROVIDER=local
export LANCEDB_LOCAL_PATH=build/data/lancedb
export EMBEDDING_PROVIDER=openrouter
export OPENROUTER_API_KEY=sk-or-xxxxx
```

**For S3/Tigrisdata Storage:**
```bash
export RAG_PIPELINE_ENABLED=true
export LANCEDB_STORAGE_PROVIDER=s3
export LANCEDB_S3_ENDPOINT=fly.storage.tigris.dev
export LANCEDB_S3_BUCKET=your-bucket
export LANCEDB_S3_REGION=auto
export AWS_ACCESS_KEY_ID=tid_xxxxx
export AWS_SECRET_ACCESS_KEY=tsec_xxxxx
export EMBEDDING_PROVIDER=openrouter
export OPENROUTER_API_KEY=sk-or-xxxxx
```

### Step 3: Run

```bash
./build/memos --mode prod
```

---

## Embedding Providers

### Mock Embeddings (Testing)

Generates random 384-dimensional vectors. Useful for testing the pipeline flow without API costs.

```bash
export EMBEDDING_PROVIDER=mock
```

**Pros:** Free, fast, no external dependencies
**Cons:** No semantic similarity (random vectors)

### Local Embeddings (Development)

Uses a local Python server running sentence-transformers.

```bash
# Terminal 1: Start embedding server
task embedding:server
# Runs on http://localhost:8001

# Terminal 2: Configure and run
export EMBEDDING_PROVIDER=local
export LOCAL_EMBEDDING_ENDPOINT=http://localhost:8001/embed
```

**Pros:** Free, high-quality embeddings, no API costs
**Cons:** Requires Python environment, additional process

### OpenRouter Embeddings (Production)

Uses OpenRouter API for production-quality embeddings.

```bash
export EMBEDDING_PROVIDER=openrouter
export EMBEDDING_MODEL=openai/text-embedding-3-small
export OPENROUTER_API_KEY=sk-or-xxxxx
```

**Pros:** Best quality, managed service
**Cons:** API costs (~$0.02/1M tokens)

---

## Environment Variables Reference

### Core Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `RAG_PIPELINE_ENABLED` | `false` | Enable/disable RAG pipeline |

### Storage Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LANCEDB_STORAGE_PROVIDER` | `memory` | Storage backend: `memory`, `local`, `s3` |
| `LANCEDB_LOCAL_PATH` | `build/data/lancedb` | Path for local storage |
| `LANCEDB_S3_ENDPOINT` | `fly.storage.tigris.dev` | S3 endpoint URL |
| `LANCEDB_S3_BUCKET` | (required for s3) | S3 bucket name |
| `LANCEDB_S3_REGION` | `auto` | S3 region |
| `AWS_ACCESS_KEY_ID` | (required for s3) | S3 access key |
| `AWS_SECRET_ACCESS_KEY` | (required for s3) | S3 secret key |

### Embedding Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `EMBEDDING_PROVIDER` | `local` | Provider: `mock`, `local`, `openrouter` |
| `EMBEDDING_MODEL` | `all-MiniLM-L6-v2` | Model name |
| `LOCAL_EMBEDDING_ENDPOINT` | `http://localhost:8001/embed` | Local server URL |
| `OPENROUTER_API_KEY` | (required for openrouter) | OpenRouter API key |

### Optional Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LLM_VERIFIER_ENABLED` | `false` | Enable LLM response verification |
| `REINDEX_RAG` | `false` | Re-index all content on startup |

---

## Re-indexing Without Re-uploading Files

When changing embedding providers or after a fresh deployment, you may need to re-index all content without re-uploading files. Use the `REINDEX_RAG` environment variable:

### Example: Switch from Mock to OpenRouter Embeddings

```bash
# First run with mock embeddings
EMBEDDING_PROVIDER=mock \
LANCEDB_STORAGE_PROVIDER=local \
RAG_PIPELINE_ENABLED=true \
./build/memos --mode dev --data build/data

# Later, switch to OpenRouter and re-index all existing content
REINDEX_RAG=true \
EMBEDDING_PROVIDER=openrouter \
OPENROUTER_API_KEY=sk-or-xxxxx \
LANCEDB_STORAGE_PROVIDER=local \
RAG_PIPELINE_ENABLED=true \
./build/memos --mode dev --data build/data
```

### What REINDEX_RAG Does

When `REINDEX_RAG=true`:
1. Server starts normally
2. After 2 seconds, re-indexing begins in the background
3. For each tenant:
   - Retrieves KB and POLICY content from the database
   - Parses content by audience type (internal/external)
   - Deletes existing vector chunks for that tenant/audience
   - Generates new embeddings with the current provider
   - Inserts new chunks into the vector database
4. Logs progress and completion

### Log Output

```
INFO Starting RAG reindex of all content...
INFO Reindexed content for tenant tenantID=3 tenant=coolair audience=internal chunks=54
INFO Reindexed content for tenant tenantID=2 tenant=carmichael audience=internal chunks=54
INFO Reindexed content for tenant tenantID=2 tenant=carmichael audience=external chunks=54
INFO RAG reindex completed totalChunks=162 tenants=2
```

### When to Use REINDEX_RAG

| Scenario | Use REINDEX_RAG |
|----------|-----------------|
| Changed embedding provider | ✅ Yes |
| Fresh deployment with existing DB | ✅ Yes |
| Changed embedding dimensions | ✅ Yes |
| Uploaded new MD files | ❌ No (auto-indexed) |
| Normal server restart | ❌ No |

### Important Notes

- Reindexing happens asynchronously in the background
- The server is fully functional during reindexing
- Old chunks are deleted before new ones are inserted
- If embedding fails for a tenant, it continues with the next one
- Set `REINDEX_RAG=false` (or remove it) after successful reindex

---

## Configuration Presets

### Development (Fastest)

```bash
RAG_PIPELINE_ENABLED=true
LANCEDB_STORAGE_PROVIDER=memory
EMBEDDING_PROVIDER=mock
```
- No rebuild required
- No external dependencies
- Data lost on restart

### Development (Quality)

```bash
RAG_PIPELINE_ENABLED=true
LANCEDB_STORAGE_PROVIDER=memory
EMBEDDING_PROVIDER=local
LOCAL_EMBEDDING_ENDPOINT=http://localhost:8001/embed
```
- No rebuild required
- Requires local embedding server
- Real semantic search
- Data lost on restart

### Production (fly.io)

```bash
RAG_PIPELINE_ENABLED=true
LANCEDB_STORAGE_PROVIDER=s3
LANCEDB_S3_ENDPOINT=fly.storage.tigris.dev
LANCEDB_S3_BUCKET=your-bucket
LANCEDB_S3_REGION=auto
EMBEDDING_PROVIDER=openrouter
EMBEDDING_MODEL=openai/text-embedding-3-small
```
- Requires RAG build
- Persistent storage
- Production-quality embeddings

---

## Verification Checklist

### 1. Check RAG is Enabled

Look for this log on startup:
```
INFO Using in-memory vector database for testing
```
or
```
INFO LanceDB vector database initialized uri=...
```

If you see this, RAG is **disabled**:
```
INFO RAG pipeline disabled, using no-op vector database
```

### 2. Check Indexing Works

After uploading KB/POLICY files:
```
INFO Indexed content for RAG tenantID=3 audience=internal chunks=25
```

### 3. Check Retrieval Works

When processing a chat message:
```
INFO Using RAG pipeline for response generation
DEBUG Retrieved context for RAG query="..." chunks=5 latency=12ms
```

### 4. Check Vector DB Stats (Programmatic)

```go
stats, _ := vectorDB.Stats(ctx)
fmt.Printf("Total chunks: %d\n", stats.TotalChunks)
fmt.Printf("Tenant counts: %v\n", stats.TenantCounts)
```

---

## Troubleshooting

### "RAG pipeline disabled, using no-op vector database"

**Cause:** `RAG_PIPELINE_ENABLED` is not set to `true`

**Solution:**
```bash
export RAG_PIPELINE_ENABLED=true
```

### "LanceDB storage requires RAG build"

**Cause:** Using `local` or `s3` storage without the RAG build tag

**Solution:** Either:
1. Use memory storage: `LANCEDB_STORAGE_PROVIDER=memory`
2. Rebuild with RAG: `task build:backend:rag`

### "failed to create embedding service"

**Cause:** Embedding provider configuration issue

**Solution:** Check provider settings:
- For mock: No additional config needed
- For local: Ensure `LOCAL_EMBEDDING_ENDPOINT` is correct and server is running
- For openrouter: Ensure `OPENROUTER_API_KEY` is set

### "search failed: query must have either QueryText or QueryEmbedding"

**Cause:** Empty query passed to search

**Solution:** Ensure user message is not empty before RAG retrieval

### No chunks indexed after file upload

**Cause:** Chunker not producing chunks

**Solution:**
1. Verify file content has parseable sections
2. Check logs for parsing errors
3. Ensure KB.MD has proper annotations (`<!-- @service: code -->`)

### Low relevance scores / poor retrieval

**Cause:** Using mock embeddings (random vectors)

**Solution:** Switch to real embeddings:
```bash
export EMBEDDING_PROVIDER=local  # or openrouter
```

---

## Memory vs Persistent Storage

### When to Use Memory Storage

- Local development
- Testing RAG functionality
- Quick demos
- CI/CD testing

### When to Use Persistent Storage

- Production deployments
- Large knowledge bases
- Multi-instance deployments
- When re-indexing on every restart is too slow

### Data Loss Scenarios

| Event | Memory | Local | S3 |
|-------|--------|-------|-----|
| Server restart | Lost | Preserved | Preserved |
| Redeploy | Lost | Lost (unless volume) | Preserved |
| File re-upload | Replaced | Replaced | Replaced |

---

## Performance Considerations

### Memory Storage

- **Insert:** O(n) where n = number of chunks
- **Search:** O(n) linear scan with cosine similarity
- **Memory:** ~1KB per chunk (embedding + metadata)
- **Suitable for:** < 10,000 chunks

### LanceDB Storage

- **Insert:** O(log n) with indexing
- **Search:** O(log n) with IVF-PQ index
- **Disk:** ~1KB per chunk + index overhead
- **Suitable for:** Millions of chunks

---

## Re-indexing Triggers

Content is re-indexed when:

1. **KB.MD uploaded** - All services, FAQs, exclusions, coverage, safety, KB sections
2. **POLICY.MD uploaded** - All rules, intents
3. **Audience changed** - When switching audience types

Re-indexing process:
1. Delete existing chunks for tenant/audience
2. Parse new content
3. Generate embeddings
4. Insert new chunks

---

*Document Version: 1.0*
*Created: 2026-01-23*
*Author: Claude Code*
