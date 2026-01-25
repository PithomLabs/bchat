# LanceDB Storage Reference

**Status:** Active
**Date:** 2026-01-26
**Related Documents:**
- [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD) - RAG implementation plan
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Environment variables reference
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG pipeline architecture

---

## Overview

This document provides a comprehensive reference for LanceDB storage in the bchat application, including:
- What data is stored and its schema
- Storage provider options (`LANCEDB_STORAGE_PROVIDER` values)
- How to inspect and query stored data
- Directory structure and file formats

---

## Table of Contents

1. [Storage Providers](#storage-providers)
2. [Data Schema](#data-schema)
3. [Content Types Indexed](#content-types-indexed)
4. [Directory Structure](#directory-structure)
5. [Inspecting Data](#inspecting-data)
6. [Configuration Reference](#configuration-reference)

---

## Storage Providers

The `LANCEDB_STORAGE_PROVIDER` environment variable controls where vector embeddings and document chunks are stored.

### Available Values

| Provider | Value | Persistence | Use Case | Build Required |
|----------|-------|-------------|----------|----------------|
| **Memory** | `memory` | None (lost on restart) | Development, testing | No |
| **Local** | `local` | Disk | Single-server production, QA | Yes (`-tags rag`) |
| **S3/Tigrisdata** | `s3` | S3-compatible storage | Multi-server production (fly.io) | Yes (`-tags rag`) |

### Provider Details

#### Memory (`memory`) - Default

In-memory storage for rapid development iteration.

```bash
export LANCEDB_STORAGE_PROVIDER=memory
```

**Characteristics:**
- No persistence (data lost on restart)
- No build flags required (works with standard build)
- Fastest for testing
- Uses `MemoryVectorDB` implementation

**When to use:** Development, unit tests, quick iteration

---

#### Local (`local`)

Persistent storage on the local filesystem.

```bash
export LANCEDB_STORAGE_PROVIDER=local
export LANCEDB_LOCAL_PATH=build/data/lancedb  # Default path
```

**Characteristics:**
- Data persists across restarts
- Requires RAG build (`task build:backend:rag`)
- Uses LanceDB native format (columnar storage)
- Single-server deployment only

**When to use:** QA environments, single-server production, development with persistence

---

#### S3/Tigrisdata (`s3`)

S3-compatible object storage for distributed deployments.

```bash
export LANCEDB_STORAGE_PROVIDER=s3
export LANCEDB_S3_ENDPOINT=fly.storage.tigris.dev
export LANCEDB_S3_BUCKET=your-bucket-name
export LANCEDB_S3_REGION=auto
export AWS_ACCESS_KEY_ID=tid_xxxxx
export AWS_SECRET_ACCESS_KEY=tsec_xxxxx
```

**Characteristics:**
- Data persists in cloud storage
- Requires RAG build (`task build:backend:rag`)
- Supports distributed/replicated deployments
- Higher latency than local storage
- Compatible with Tigrisdata (fly.io native storage)

**When to use:** Production on fly.io, multi-server deployments

---

## Data Schema

The LanceDB table `kb_documents` stores document chunks with embeddings for semantic search.

### Schema Fields

| Field | Type | Nullable | Description |
|-------|------|----------|-------------|
| `id` | String | No | Unique identifier: `{tenantID}:{audience}:{type}:{code}` |
| `tenant_id` | Int32 | No | Multi-tenant isolation key |
| `audience_type` | String | No | Audience filter (e.g., "external", "internal") |
| `content_type` | String | No | Document type (service, faq, rule, etc.) |
| `title` | String | Yes | Human-readable title |
| `content` | String | No | Full text content for search and context |
| `code` | String | Yes | Service code, rule code, or identifier |
| `is_emergency` | Boolean | Yes | Emergency service/protocol flag |
| `is_active` | Boolean | No | Active/inactive status for filtering |
| `priority` | Int32 | Yes | Priority ranking for sorting |
| `embedding` | Float32[384] | No | Vector representation (384 dimensions default) |
| `indexed_at` | Int64 | No | Unix timestamp when indexed |
| `source_version` | Int32 | Yes | Version tracking for change detection |

### ID Format

Document IDs follow a deterministic format for upsert operations:

```
{tenant_id}:{audience_type}:{content_type}:{code}
```

**Examples:**
- `123:external:service:water-damage`
- `123:external:faq:0`
- `456:internal:rule:escalation-policy`

### Embedding Dimensions

| Provider | Model | Dimensions |
|----------|-------|------------|
| Mock | N/A | 384 |
| Local | `all-MiniLM-L6-v2` | 384 |
| OpenRouter | `openai/text-embedding-3-small` | 1536 |
| OpenRouter | `openai/text-embedding-3-large` | 3072 |

---

## Content Types Indexed

Documents are chunked by type, with each chunk representing a single logical unit.

### Content Type Reference

| Type | Source | Chunking Strategy | Fields Used |
|------|--------|-------------------|-------------|
| `service` | KB.MD | One chunk per service | name, description, response_time, is_emergency |
| `faq` | KB.MD | One chunk per Q&A pair | question, answer |
| `exclusion` | KB.MD | One chunk per exclusion | name, description, exceptions |
| `coverage` | KB.MD | One chunk per area | area_name, description, coverage_type |
| `rule` | POLICY.MD | One chunk per rule | rule_name, description, applicability |
| `safety` | KB.MD | One chunk per protocol | protocol_name, steps (marked as emergency) |
| `kb_section` | KB.MD | One chunk per section | section_title, content |
| `intent` | POLICY.MD | One chunk per intent | intent_name, description, examples |

### Content Construction

Content is built from structured data before embedding:

```go
// Service example
"Water Damage Restoration: 24/7 emergency water extraction and drying services. Response time: 1-2 hours. This is an emergency service."

// FAQ example
"Q: Do you offer financing?\nA: Yes, we offer 0% financing for qualified customers on jobs over $1000."

// Rule example
"Escalation Policy: Transfer to human agent when customer expresses frustration three times or requests a supervisor."
```

---

## Directory Structure

### Local Storage Layout

```
build/data/lancedb/
└── kb_documents.lance/           # Main table directory
    ├── data/                     # Data partition files
    │   ├── 0000111110011010...lance
    │   ├── 110001000100010...lance
    │   └── ...
    ├── _indices/                 # Index metadata
    │   └── {uuid}/
    │       ├── page_data.lance
    │       └── page_lookup.lance
    ├── _transactions/            # Transaction logs
    └── _versions/                # Version metadata
```

### File Descriptions

| Directory | Contents | Purpose |
|-----------|----------|---------|
| `data/` | `.lance` partition files | Columnar data storage (Arrow format) |
| `_indices/` | Index structures | Vector (IVF-PQ), BTree, FTS indexes |
| `_transactions/` | Transaction logs | ACID compliance, recovery |
| `_versions/` | Version manifests | Time travel, snapshot management |

### Storage Size

Typical storage consumption per tenant (estimates):

| Content Volume | Approximate Size |
|----------------|-----------------|
| 20 services + 50 FAQs | ~500 KB |
| 50 services + 100 FAQs | ~1.5 MB |
| 100 services + 200 FAQs | ~3 MB |

**Note:** Size depends heavily on embedding dimensions (384 vs 1536).

---

## Inspecting Data

### Method 1: API Stats Endpoint

Get database statistics via the reindex endpoint response:

```bash
curl -X POST http://localhost:5230/api/v1/agent/{slug}/reindex \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json"
```

Response includes chunk counts and timing.

### Method 2: Python LanceDB CLI

Install the LanceDB Python package for direct inspection:

```bash
pip install lancedb pyarrow
```

**Inspect table schema and data:**

```python
import lancedb

# Connect to local database
db = lancedb.connect("build/data/lancedb")

# List tables
print(db.table_names())
# Output: ['kb_documents']

# Open table
table = db.open_table("kb_documents")

# View schema
print(table.schema)

# Count rows
print(f"Total rows: {table.count_rows()}")

# Query all data (returns PyArrow table)
data = table.to_arrow()
print(data.to_pandas())

# Query with filters
results = table.search().where("tenant_id = 1").limit(10).to_list()
for row in results:
    print(f"ID: {row['id']}, Type: {row['content_type']}, Title: {row['title']}")
```

**Search by content type:**

```python
# Get all services for tenant 1
services = table.search() \
    .where("tenant_id = 1 AND content_type = 'service'") \
    .to_list()

for svc in services:
    print(f"- {svc['title']}: {svc['content'][:100]}...")
```

**View statistics:**

```python
import pandas as pd

df = table.to_pandas()

# Chunks by tenant
print("\nChunks by tenant:")
print(df.groupby('tenant_id').size())

# Chunks by content type
print("\nChunks by content type:")
print(df.groupby('content_type').size())

# Chunks by audience
print("\nChunks by audience:")
print(df.groupby('audience_type').size())
```

### Method 3: Go Stats Function

Use the `VectorDB.Stats()` method programmatically:

```go
stats, err := vectorDB.Stats(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Total chunks: %d\n", stats.TotalChunks)
fmt.Printf("Tenant counts: %v\n", stats.TenantCounts)
fmt.Printf("Content type counts: %v\n", stats.ContentCounts)
```

### Method 4: Direct File Inspection

For debugging, inspect the raw directory structure:

```bash
# List table contents
ls -la build/data/lancedb/kb_documents.lance/

# Check data file count and sizes
ls -lah build/data/lancedb/kb_documents.lance/data/

# Total size
du -sh build/data/lancedb/

# Check index status
ls -la build/data/lancedb/kb_documents.lance/_indices/
```

---

## Configuration Reference

### Full Environment Variable List

| Variable | Provider | Default | Description |
|----------|----------|---------|-------------|
| `LANCEDB_STORAGE_PROVIDER` | All | `memory` | Storage backend selection |
| `LANCEDB_LOCAL_PATH` | `local` | `build/data/lancedb` | Directory for local storage |
| `LANCEDB_S3_ENDPOINT` | `s3` | `fly.storage.tigris.dev` | S3 endpoint hostname |
| `LANCEDB_S3_BUCKET` | `s3` | (required) | S3 bucket name |
| `LANCEDB_S3_REGION` | `s3` | `auto` | S3 region |
| `AWS_ACCESS_KEY_ID` | `s3` | (required) | S3 access key |
| `AWS_SECRET_ACCESS_KEY` | `s3` | (required) | S3 secret key |

### Configuration Presets

**Development (No persistence, fast):**
```bash
export RAG_PIPELINE_ENABLED=true
export LANCEDB_STORAGE_PROVIDER=memory
export EMBEDDING_PROVIDER=mock
```

**Development (With persistence):**
```bash
export RAG_PIPELINE_ENABLED=true
export LANCEDB_STORAGE_PROVIDER=local
export LANCEDB_LOCAL_PATH=build/data/lancedb
export EMBEDDING_PROVIDER=mock
```

**QA/Staging:**
```bash
export RAG_PIPELINE_ENABLED=true
export LANCEDB_STORAGE_PROVIDER=local
export LANCEDB_LOCAL_PATH=/data/lancedb
export EMBEDDING_PROVIDER=openrouter
export EMBEDDING_MODEL=openai/text-embedding-3-small
```

**Production (fly.io):**
```bash
export RAG_PIPELINE_ENABLED=true
export LANCEDB_STORAGE_PROVIDER=s3
export LANCEDB_S3_ENDPOINT=fly.storage.tigris.dev
export LANCEDB_S3_BUCKET=myapp-vectors
export LANCEDB_S3_REGION=auto
export AWS_ACCESS_KEY_ID=tid_xxxxx
export AWS_SECRET_ACCESS_KEY=tsec_xxxxx
export EMBEDDING_PROVIDER=openrouter
export EMBEDDING_MODEL=openai/text-embedding-3-small
```

---

## Indexes Created

The following indexes are created automatically on table initialization:

| Index Name | Type | Column(s) | Purpose |
|------------|------|-----------|---------|
| `idx_embedding` | IVF-PQ | `embedding` | Vector similarity search |
| `idx_tenant` | BTree | `tenant_id` | Efficient tenant filtering |
| `idx_content_fts` | FTS | `content` | Full-text BM25 keyword search |
| `idx_title_fts` | FTS | `title` | Title-based keyword search |

### Index Types

- **IVF-PQ (Inverted File with Product Quantization):** Approximate nearest neighbor search for vectors
- **BTree:** Standard B-tree for scalar filtering
- **FTS (Full-Text Search):** BM25-based text search for hybrid retrieval

---

## Search Modes

### Vector-Only Search

Default mode using cosine similarity on embeddings.

```go
query := SearchQuery{
    QueryText:    "water damage restoration",
    TenantID:     1,
    TopK:         5,
    MinScore:     0.3,
}
```

### Hybrid Search

Combines vector similarity with BM25 keyword matching.

```go
query := SearchQuery{
    QueryText:       "water damage restoration",
    TenantID:        1,
    TopK:            5,
    MinScore:        0.3,
    UseHybridSearch: true,
    VectorWeight:    0.7,  // 70% semantic
    TextWeight:      0.3,  // 30% keyword
}
```

**Score Fusion:**
```
final_score = (vector_weight * cosine_similarity) + (text_weight * bm25_score)
```

---

## Reindexing

### Per-Tenant Reindex (API)

```bash
curl -X POST http://localhost:5230/api/v1/agent/{slug}/reindex \
  -H "Authorization: Bearer {token}"
```

### All-Tenant Reindex (Startup)

```bash
export REINDEX_RAG=true
./build/memos --mode dev
```

**Note:** Set `REINDEX_RAG=false` after successful reindex.

### When to Reindex

- After changing embedding provider/model
- After fresh deployment with existing database
- After changing embedding dimensions
- When debugging search quality issues

---

## Troubleshooting

### "LanceDB storage requires RAG build"

Using `local` or `s3` storage without the RAG build tag.

**Solution:**
```bash
# Either use memory storage
export LANCEDB_STORAGE_PROVIDER=memory

# Or rebuild with RAG support
task build:backend:rag
```

### Empty Search Results

1. Check if data is indexed: Use Python inspection
2. Verify tenant ID and audience type match
3. Check `is_active` flag on chunks
4. Lower `MinScore` threshold (default: 0.3)

### High Latency

1. Ensure indexes are created (check `_indices/` directory)
2. For S3 storage, latency is inherently higher
3. Consider local caching or memory storage for development

### Data Not Persisting

1. Verify `LANCEDB_STORAGE_PROVIDER=local` or `s3`
2. Check `LANCEDB_LOCAL_PATH` permissions
3. Ensure RAG build is being used (`task build:backend:rag`)

---

*Document Version: 1.0*
*Created: 2026-01-26*
*Author: Claude Code*
