# RAG Pipeline Reindexing Guide

## Overview

This document explains why RAG content may not be indexed (showing zero chunks) and how to resolve the issue. It covers the indexing pipeline, common failure modes, and step-by-step troubleshooting.

---

## Symptoms

| Symptom | Likely Cause |
|---------|--------------|
| RAG Stats shows 0 chunks for tenant | Content not indexed |
| Search returns "No results found" | RAG disabled or no indexed content |
| "Rebuild Index" returns error | RAG pipeline disabled (NoOpVectorDB) |
| Stats reset after server restart | Using memory storage (volatile) |

---

## Root Cause Analysis

### 1. RAG Pipeline Disabled (Most Common)

The RAG pipeline requires explicit activation via environment variable:

```go
// server/router/api/v1/agent/vectordb.go:71
Enabled: os.Getenv("RAG_PIPELINE_ENABLED") == "true"
```

If `RAG_PIPELINE_ENABLED` is not set to `"true"`, the system uses `NoOpVectorDB`:

```go
// vectordb.go:136-138
if !config.Enabled {
    slog.Info("RAG pipeline disabled, using no-op vector database")
    return NewNoOpVectorDB(), nil  // Stores NOTHING
}
```

**NoOpVectorDB** silently accepts all operations but discards them:
- `Insert()` does nothing
- `Search()` returns empty results
- `Stats()` returns zero counts

### 2. Memory Storage (Data Lost on Restart)

The default storage provider is `memory`, which is volatile:

```go
// vectordb.go:63
StorageProvider: getEnvOrDefault("LANCEDB_STORAGE_PROVIDER", "memory")
```

| Provider | Persistence | Use Case |
|----------|-------------|----------|
| `memory` | None (lost on restart) | Quick testing |
| `local` | Disk (`build/data/lancedb/`) | Development, small deployments |
| `s3` | S3/Tigrisdata | Production |

### 3. Content Uploaded Before RAG Enabled

Indexing only occurs at specific trigger points:

| Trigger | When |
|---------|------|
| KB/Policy Upload | `indexContentForRAG()` in handlers.go:905 |
| Manual Reindex | "Rebuild Index" button or API endpoint |
| Startup Reindex | `REINDEX_RAG=true` environment variable |

If KB/Policy files were uploaded **before** `RAG_PIPELINE_ENABLED=true` was set, they were never indexed.

---

## Code Path Trace

### Indexing Flow (On File Upload)

```
POST /api/v1/agent/:slug/import
         │
         ▼
handlers.go:905 - indexContentForRAG()
         │
         ▼
Check: chunker != nil && vectorDB != nil
         │
         ├─ If NoOpVectorDB → returns nil (silent no-op)
         │
         ▼
vectorDB.Delete(tenantID, audienceType)  // Clear existing
         │
         ▼
chunker.ChunkKBContent() + ChunkPolicyContent()
         │
         ▼
vectorDB.Insert(chunks)  // Generate embeddings & store
         │
         ▼
slog.Info("Indexed content for RAG", chunks=N)
```

### Manual Reindex Flow

```
POST /api/v1/agent/:slug/reindex  OR  "Rebuild Index" button
         │
         ▼
handlers.go:571 - service.ReindexTenantContent()
         │
         ▼
service.go:201 - Check: vectorDB != nil && chunker != nil
         │
         ├─ If nil → return error "RAG pipeline not initialized"
         │
         ▼
service.go:206 - Check: NOT NoOpVectorDB
         │
         ├─ If NoOpVectorDB → return error "RAG pipeline disabled (using NoOpVectorDB)"
         │
         ▼
Load source files from database (agent_source_files)
         │
         ▼
Parse KB + Policy for each audience type
         │
         ▼
vectorDB.Delete() → chunker.Chunk*() → vectorDB.Insert()
         │
         ▼
Return total chunk count
```

---

## Resolution Steps

### Step 1: Verify Current Configuration

Check RAG Stats page to see current status:
- **Enabled:** Should be "Yes"
- **Storage Provider:** Should be "local" or "s3" for persistence
- **Embedding Provider:** Should be configured

If "Enabled: No", RAG is disabled.

### Step 2: Set Required Environment Variables

```bash
# REQUIRED: Enable RAG pipeline
export RAG_PIPELINE_ENABLED=true

# RECOMMENDED: Use persistent storage
export LANCEDB_STORAGE_PROVIDER=local

# REQUIRED: Choose embedding provider
export EMBEDDING_PROVIDER=mock  # Options: mock, local, openrouter

# If using OpenRouter embeddings
export OPENROUTER_API_KEY=sk-or-v1-...
```

### Step 3: Restart Server

Using Taskfile (recommended):
```bash
task run:rag        # Starts with RAG enabled
# OR
task run:rag:mock   # Uses mock embeddings (no API key needed)
```

Manual:
```bash
RAG_PIPELINE_ENABLED=true LANCEDB_STORAGE_PROVIDER=local go run ./bin/memos/main.go
```

### Step 4: Reindex Content

**Option A: Via Agent Admin UI**
1. Navigate to Agent Admin
2. Select the tenant (e.g., "Iglesia ni Cristo")
3. Click "Rebuild Index" button
4. Wait for success message with chunk count

**Option B: Via API**
```bash
# Get admin token first (login as admin)
TOKEN="your-auth-token"

# Reindex specific tenant
curl -X POST "http://localhost:8081/api/v1/agent/iglesia-ni-cristo/reindex" \
  -H "Authorization: Bearer $TOKEN"

# Response: {"success":true,"chunks":45,"message":"Successfully reindexed 45 chunks"}
```

**Option C: Reindex All Tenants on Startup**
```bash
RAG_PIPELINE_ENABLED=true REINDEX_RAG=true task run:rag
```

This triggers `ReindexAllContent()` which processes all tenants.

### Step 5: Verify Indexing

1. Go to RAG Stats page
2. Check "Per-Tenant Breakdown" table
3. Verify tenant shows chunk count > 0
4. Test search with a query

---

## Environment Variable Reference

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `RAG_PIPELINE_ENABLED` | `false` | Yes | Must be `"true"` to enable RAG |
| `LANCEDB_STORAGE_PROVIDER` | `memory` | No | Storage: `memory`, `local`, or `s3` |
| `LANCEDB_LOCAL_PATH` | `build/data/lancedb` | No | Path for local storage |
| `EMBEDDING_PROVIDER` | `local` | No | Embedding: `mock`, `local`, or `openrouter` |
| `EMBEDDING_MODEL` | varies | No | Model identifier |
| `REINDEX_RAG` | `false` | No | Reindex all content on startup |
| `HYBRID_SEARCH_ENABLED` | `false` | No | Enable vector + BM25 hybrid search |

---

## Storage Provider Details

### Memory Storage (`LANCEDB_STORAGE_PROVIDER=memory`)

```go
case "memory":
    return NewMemoryVectorDB(embedSvc), nil
```

- **Pros:** Fast, no disk I/O, no setup
- **Cons:** Data lost on restart
- **Use for:** Quick testing, CI/CD

### Local Storage (`LANCEDB_STORAGE_PROVIDER=local`)

```go
case "local":
    return newLanceVectorDB(config, embedSvc)
```

- **Pros:** Persistent, no external dependencies
- **Cons:** Single-machine only
- **Path:** `build/data/lancedb/` (configurable via `LANCEDB_LOCAL_PATH`)
- **Use for:** Development, small deployments

### S3 Storage (`LANCEDB_STORAGE_PROVIDER=s3`)

```go
case "s3":
    return newLanceVectorDB(config, embedSvc)
```

- **Pros:** Scalable, shared across instances
- **Cons:** Requires S3 credentials, network latency
- **Use for:** Production (fly.io with Tigrisdata)

Required S3 variables:
```bash
LANCEDB_S3_ENDPOINT=fly.storage.tigris.dev
LANCEDB_S3_BUCKET=your-bucket
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
```

---

## Troubleshooting Checklist

### Symptom: Zero chunks for all tenants

- [ ] Is `RAG_PIPELINE_ENABLED=true` set?
- [ ] Check server logs for "RAG pipeline disabled"
- [ ] Check RAG Stats "Enabled" status

### Symptom: Chunks reset to zero after restart

- [ ] Is `LANCEDB_STORAGE_PROVIDER` set to `local` or `s3`?
- [ ] Default `memory` storage is volatile

### Symptom: "Rebuild Index" fails with error

- [ ] Error "RAG pipeline not initialized" → Set `RAG_PIPELINE_ENABLED=true`
- [ ] Error "RAG pipeline disabled (using NoOpVectorDB)" → Same fix
- [ ] Restart server after changing env vars

### Symptom: Search returns no results despite chunks > 0

- [ ] Verify searching correct tenant
- [ ] Check audience type (external vs internal)
- [ ] Try broader search terms
- [ ] Check embedding provider is working

### Symptom: Embedding errors during indexing

- [ ] Check `EMBEDDING_PROVIDER` is valid
- [ ] For `openrouter`: verify `OPENROUTER_API_KEY` is set
- [ ] For `local`: verify local embedding server is running
- [ ] Try `EMBEDDING_PROVIDER=mock` for testing

---

## Server Log Messages

### Successful RAG Initialization
```
INFO RAG pipeline enabled, using local storage path=build/data/lancedb
INFO Embedding service initialized provider=mock dimension=384
```

### RAG Disabled (NoOp)
```
INFO RAG pipeline disabled, using no-op vector database
```

### Successful Indexing
```
INFO Indexed content for RAG tenantID=1 audience=external chunks=45
INFO RAG reindex completed for tenant tenantID=1 tenant=iglesia-ni-cristo totalChunks=45
```

### Startup Reindex
```
INFO Starting RAG content reindex on startup
INFO RAG reindex completed totalChunks=120 tenants=3
```

---

## Quick Reference Commands

```bash
# Development with persistent storage
task run:rag

# Development with mock embeddings (no API key)
task run:rag:mock

# Reindex all tenants on startup
RAG_PIPELINE_ENABLED=true REINDEX_RAG=true task run:rag

# Production (fly.io)
fly secrets set RAG_PIPELINE_ENABLED=true
fly secrets set REINDEX_RAG=true  # One-time reindex
fly deploy
fly secrets unset REINDEX_RAG     # Remove after reindex
```

---

## Addendum: File Version Bug (Fixed 2026-01-26)

### Issue

Zero chunks despite properly annotated KB.MD file.

**Example:** Iglesia ni Cristo tenant had an internal KB.MD with 43 annotations (version 4, 20KB), but "Rebuild Index" showed 0 chunks.

### Root Cause

The `ReindexTenantContent()` function in `service.go` was loading ALL file versions instead of just the latest:

```go
// BUG: Missing LatestOnly: true
files, err := s.store.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
    TenantID: &tenantID,
})
```

The query returned files ordered by `version DESC`. The loop then overwrote the content map for each file:

```go
for _, f := range files {
    audienceFiles[f.AudienceType][f.FileType] = f.Content  // Overwrites!
}
```

Result: The **oldest version** (usually raw/unformatted) ended up being processed instead of the latest formatted version.

### Evidence

```
INC internal KB versions:
- Version 4: 20KB, 43 annotations ← Should be used
- Version 3: 1.7MB, 0 annotations
- Version 2: 1.7MB, 0 annotations
- Version 1: 1.7MB, 0 annotations ← Was being used (bug)
```

### Fix

Added `LatestOnly: true` to the `ListAgentSourceFiles` call:

```go
files, err := s.store.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
    TenantID:   &tenantID,
    LatestOnly: true,  // Only get latest version of each file type
})
```

### Verification

After fix, "Rebuild Index" should show non-zero chunks for tenants with annotated KB.MD files.

---

## Addendum: Embedding Server Connection Refused (Fixed 2026-01-26)

### Issue

"Rebuild Index" fails with connection refused error:

```
Reindex failed: failed to insert chunks for audience internal:
failed to generate embeddings: local embedding request failed:
Post "http://localhost:8001/embed": dial tcp 127.0.0.1:8001: connect: connection refused
```

### Root Cause

`EMBEDDING_PROVIDER` defaults to `"local"`, which requires a local embedding server running at `http://localhost:8001/embed`. If no server is running, the connection is refused.

### Solutions

**Option A: Use Mock Embeddings (Recommended for Testing)**

```bash
# Use hash-based embeddings (no external dependencies)
export EMBEDDING_PROVIDER=mock
task run:rag

# Or use the convenience command:
task run:rag:mock
```

- Deterministic hash-based embeddings
- No external dependencies
- Good for testing RAG pipeline functionality

**Option B: Use OpenRouter Embeddings (Production)**

```bash
export EMBEDDING_PROVIDER=openrouter
export OPENROUTER_API_KEY=sk-or-v1-...
task run:rag
```

- Uses OpenAI text-embedding-3-small via OpenRouter
- Requires API key
- Better semantic search quality

**Option C: Run Local Embedding Server**

```bash
# Start embedding server (Python FastAPI with sentence-transformers)
cd embedding-server && python server.py

# Then run app with local provider
export EMBEDDING_PROVIDER=local
export EMBEDDING_LOCAL_ENDPOINT=http://localhost:8001/embed
task run:rag
```

### Quick Fix

For development/testing, use mock embeddings:

```bash
task run:rag:mock
```

---

## Addendum: Audience Type Filtering (Added 2026-01-26)

### Background

By default, "Rebuild Index" indexes **all** audience types (internal and external) for a tenant. Sometimes you only want to reindex a specific audience.

### API Usage

The reindex endpoint now accepts an optional `audience` query parameter:

```bash
# Reindex internal content only
curl -X POST "http://localhost:8081/api/v1/agent/inc/reindex?audience=internal" \
  -H "Authorization: Bearer $TOKEN"

# Reindex external content only
curl -X POST "http://localhost:8081/api/v1/agent/inc/reindex?audience=external" \
  -H "Authorization: Bearer $TOKEN"

# Reindex all audiences (default behavior)
curl -X POST "http://localhost:8081/api/v1/agent/inc/reindex" \
  -H "Authorization: Bearer $TOKEN"
```

### Response

```json
{
  "success": true,
  "chunks": 45,
  "message": "Successfully reindexed 45 chunks",
  "audience": "internal"
}
```

The `audience` field is only present when filtering by a specific audience type.

### Code Changes

**service.go:**
```go
func (s *Service) ReindexTenantContent(ctx context.Context, tenantID int32, audienceType string) (int, error) {
    // ...
    findParams := &store.FindAgentSourceFile{
        TenantID:   &tenantID,
        LatestOnly: true,
    }

    // Optional: filter by audience type
    if audienceType != "" {
        findParams.AudienceType = &audienceType
    }
    // ...
}
```

**handlers.go:**
```go
func (h *Handler) HandleReindexTenant(c echo.Context) error {
    // ...
    audienceType := c.QueryParam("audience") // "internal" or "external" or "" (all)
    chunks, err := h.service.ReindexTenantContent(ctx, tenant.ID, audienceType)
    // ...
}
```

---

## Related Documentation

- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG architecture overview
- [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD) - LanceDB implementation details
- [DOCS_LANCEDB_STORAGE.MD](./DOCS_LANCEDB_STORAGE.MD) - Storage inspection
- [DOCS_EMBEDDING.MD](./DOCS_EMBEDDING.MD) - Embedding providers
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Environment variables
- [DOCS_TASKFILE.MD](./DOCS_TASKFILE.MD) - Build commands
