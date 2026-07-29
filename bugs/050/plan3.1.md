# Revised Implementation Plan: bchat × CockroachDB × AWS

## CockroachDB Tools Used (2 required + 1 bonus)

| Tool | Status | Source |
|------|--------|--------|
| **Distributed Vector Indexing** | VERIFIED | `cockroachdb-skills-main/.../01-schema-design.md`: `VECTOR(1536)`, `CREATE VECTOR INDEX ... vector_ip_ops`, `<=>` cosine operator |
| **ccloud CLI / Cloud REST API** | VERIFIED | `cockroachdb-skills-main/.../ccloud-commands.md`: `GET/POST/PATCH /api/v1/clusters`, cluster provisioning, backups |
| **MCP Server** | NOT USED | Skills repo only references "cockroach-cloud MCP server" as a connection method, no implementation exists |

## AWS Services Used (1 required + 2 bonus)

| Service | Purpose |
|---------|---------|
| **Amazon ECS Fargate** | Containerized bchat deployment |
| **Amazon Bedrock** | LLM inference (Claude 3.5 Sonnet) for observer/reflector |
| **Amazon S3** | Tenant document storage (KB, Policy, Script files) |

---

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        AWS Cloud                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                   │
│  │   ECS    │  │  Bedrock │  │    S3    │                   │
│  │ (bchat)  │  │  (LLM)   │  │  (docs)  │                   │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘                   │
│       │              │              │                          │
│  ┌────┴──────────────┴──────────────┴────────────────────┐   │
│  │              CockroachDB Cloud                         │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌────────────┐  │   │
│  │  │ agent_vectors │  │ agent_tickets │  │ agent_obs  │  │   │
│  │  │ (VECTOR col)  │  │ (relational)  │  │ (OM logs)  │  │   │
│  │  └──────────────┘  └──────────────┘  └────────────┘  │   │
│  └───────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
```

---

## Critical Design Context: Vector Search Nuance

### CockroachDB vs Neon/Postgres — Different Vector Implementations

| Feature | CockroachDB | Neon / Standard Postgres |
|---------|-------------|-------------------------|
| **VECTOR type** | Native (built into binary) | Requires `CREATE EXTENSION IF NOT EXISTS vector` |
| **Extension** | ❌ `CREATE EXTENSION` fails — not a Postgres extension | ✅ pgvector extension must be enabled first |
| **Vector index syntax** | `CREATE VECTOR INDEX idx ON t (embedding vector_cosine_ops)` | `CREATE INDEX idx ON t USING hnsw (embedding vector_cosine_ops)` |
| **Index algorithm** | C-SPANN (distributed) | HNSW or IVFFlat (single-node) |
| **Search operator** | `<=>` cosine, `<->` L2, `<#>` inner product | Same operators (pgvector-compatible) |
| **Max dimensions** | 16,383 | 2,000 (HNSW), 4,000 (halfvec) |

### Why This Matters for Migration Strategy

**The `VECTOR(1536)` column type will FAIL on Neon/standard Postgres without the pgvector extension enabled first.** And `CREATE EXTENSION IF NOT EXISTS vector` will FAIL on CockroachDB (it doesn't support PostgreSQL extensions — VECTOR is native).

**Therefore:** The migration file cannot contain `CREATE TABLE` with `VECTOR(1536)` and be safe for all databases. The schema creation must be handled per-provider in the `Validate()` method.

### Schema Creation Strategy

| Provider | Validate() Actions |
|----------|-------------------|
| **CockroachDB** | `CREATE TABLE ... embedding VECTOR(1536)` (native) → `CREATE VECTOR INDEX IF NOT EXISTS ...` |
| **Neon / Postgres** | `CREATE EXTENSION IF NOT EXISTS vector` → `CREATE TABLE ... embedding VECTOR(1536)` → `CREATE INDEX ... USING hnsw (embedding vector_cosine_ops)` |
| **SQLite** | Skip entirely (different migration path) |

### Impact on Hackathon Submission

This nuance actually **strengthens** the submission narrative:

> "We built a vector store that works across CockroachDB's native distributed vectors AND standard Postgres with pgvector — demonstrating real production flexibility, not just a single-vendor demo."

---

## Component 1: CockroachDB Vector Store

### New File: `vectordb_cockroach.go`

Implements the `VectorDB` interface (`vectordb.go:30-75`) using CockroachDB's native vector support.

**Schema** (created in `Validate()`, NOT in migration — see Component 1b):
```sql
CREATE TABLE IF NOT EXISTS agent_vectors (
    id STRING PRIMARY KEY,
    tenant_id INT NOT NULL,
    content_type STRING NOT NULL,
    title STRING,
    content TEXT NOT NULL,
    embedding VECTOR(1536),  -- CockroachDB native type, no extension needed
    metadata JSONB,
    source_version INT DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_vectors_tenant ON agent_vectors (tenant_id);
```

**Vector index** (created separately per cockroachdb-skills syntax):
```sql
CREATE VECTOR INDEX idx_agent_vectors_embedding ON agent_vectors (embedding vector_cosine_ops);
```

**Note:** The skills repo documents `vector_ip_ops` (inner product). We use `vector_cosine_ops` for cosine distance (`<=>` operator) which matches bchat's existing similarity metric. Both opclasses are confirmed supported in CRDB v26.2 docs.

**Known limitation:** Standalone `CREATE VECTOR INDEX` does not support prefix columns. The `tenant_id` filter uses the B-tree index, then vector search runs on the filtered set. Acceptable at hackathon scale (few thousand vectors). For production, use inline `VECTOR INDEX` in `CREATE TABLE` with `(tenant_id, embedding)` prefix columns.

**Hybrid search note:** Vector-only search for MVP. FTS hybrid search deferred — CRDB uses PostgreSQL `ts_rank` (not BM25), requiring different fusion logic and re-tuned weights from the existing LanceDB BM25 implementation (`vectordb_lance.go:1224-1344`).

**Search query** (from `cockroachdb-skills-main/.../03-query-patterns.md`):
```sql
SELECT id, title, content, content_type, metadata, source_version, created_at,
       1 - (embedding <=> $1::VECTOR) AS similarity
FROM agent_vectors
WHERE tenant_id = $2 AND content_type = ANY($3)
ORDER BY embedding <=> $1::VECTOR
LIMIT $4;
```

**Key design decisions:**
- Single-row inserts (NOT batched) — CRDB docs warn: *"Large batch inserts of VECTOR types can cause performance degradation"*
- Uses `crdb.ExecuteTx` from `github.com/cockroachdb/cockroach-go/v2/crdb` for automatic retry on `SQLSTATE 40001` serialization errors — **verified correct** because bchat uses `database/sql` (`store/db/postgres/postgres.go:36`: `sql.Open("pgx", dsn)`)
- Connection pool: configurable via `COCKROACH_MAX_OPEN_CONNS` (default 5 for CRDB Serverless compatibility)

### Connection Setup

Verified against `store/db/postgres/postgres.go:28-44` and `pgx.md`:

```go
import (
    "database/sql"
    "os"
    "strings"
    "time"

    _ "github.com/jackc/pgx/v5/stdlib"
    "github.com/cockroachdb/cockroach-go/v2/crdb"
)

func newCockroachDB(dsn string) (*sql.DB, error) {
    // CRDB requires simple_protocol to avoid prepared statement issues
    if !strings.Contains(dsn, "default_query_exec_mode") {
        sep := "?"
        if strings.Contains(dsn, "?") {
            sep = "&"
        }
        dsn += sep + "default_query_exec_mode=simple_protocol"
    }

    db, err := sql.Open("pgx", dsn)
    if err != nil {
        return nil, fmt.Errorf("failed to open CockroachDB: %w", err)
    }

    // CRDB Serverless compatibility: limit connections
    db.SetMaxOpenConns(5)
    db.SetMaxIdleConns(2)
    db.SetConnMaxLifetime(5 * time.Minute)
    db.SetConnMaxIdleTime(1 * time.Minute)

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    if err := db.PingContext(ctx); err != nil {
        return nil, fmt.Errorf("failed to ping CockroachDB: %w", err)
    }

    return db, nil
}
```

### ExecuteTx Usage

Verified against `crdb/tx.go:41-53`:

```go
import "github.com/cockroachdb/cockroach-go/v2/crdb"

// Single-row insert with retry on SQLSTATE 40001
err := crdb.ExecuteTx(ctx, v.db, nil, func(tx *sql.Tx) error {
    _, err := tx.ExecContext(ctx, `
        UPSERT INTO agent_vectors (id, tenant_id, content_type, title, content, embedding, metadata, source_version, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `, id, tenantID, contentType, title, content, embeddingJSON, metadataJSON, sourceVersion, time.Now())
    return err
})
if err != nil {
    return fmt.Errorf("failed to insert vector: %w", err)
}
```

### Retry Policy

Verified against `crdb/retry.go:68-136`:

```go
// Custom retry policy for production workloads
ctx := crdb.WithRetryPolicy(ctx, &crdb.LimitBackoffRetryPolicy{
    RetryLimit: 5,
    Delay:      100 * time.Millisecond,
})

err := crdb.ExecuteTx(ctx, v.db, nil, func(tx *sql.Tx) error {
    // transaction logic
})
```

### Error Handling

Verified against `crdb/error.go`:

```go
import "github.com/cockroachdb/cockroach-go/v2/crdb"

var ambiguousErr *crdb.AmbiguousCommitError
if errors.As(err, &ambiguousErr) {
    slog.Warn("Ambiguous commit — may or may not have been applied", "error", err)
}

var maxRetriesErr *crdb.MaxRetriesExceededError
if errors.As(err, &maxRetriesErr) {
    slog.Error("Transaction failed after max retries", "error", err)
}
```

### Validate() — CockroachDB Provider

Creates table with native VECTOR type and CRDB-specific vector index:

```go
func (v *CockroachVectorDB) Validate(ctx context.Context) error {
    // 1. Create table with native VECTOR type (no extension needed)
    _, err := v.db.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS agent_vectors (
            id STRING PRIMARY KEY,
            tenant_id INT NOT NULL,
            content_type STRING NOT NULL,
            title STRING,
            content TEXT NOT NULL,
            embedding VECTOR(1536),
            metadata JSONB,
            source_version INT DEFAULT 1,
            created_at TIMESTAMPTZ DEFAULT now()
        )
    `)
    if err != nil {
        return fmt.Errorf("failed to create agent_vectors table: %w", err)
    }

    // 2. B-tree index for tenant filter
    _, err = v.db.ExecContext(ctx, `
        CREATE INDEX IF NOT EXISTS idx_agent_vectors_tenant ON agent_vectors (tenant_id)
    `)
    if err != nil {
        return fmt.Errorf("failed to create tenant index: %w", err)
    }

    // 3. Vector index (CRDB-specific syntax — NOT pgvector USING hnsw)
    _, err = v.db.ExecContext(ctx, `
        CREATE VECTOR INDEX IF NOT EXISTS idx_agent_vectors_embedding
        ON agent_vectors (embedding vector_cosine_ops)
    `)
    if err != nil {
        slog.Warn("Vector index creation failed (may need feature.vector_index.enabled)",
            "error", err)
        // Non-fatal: table still works without vector index (brute-force search)
    }

    return nil
}
```

### Validate() — Neon/Postgres Provider (Future)

For future Neon vector DB provider. Creates pgvector extension and uses standard HNSW index:

```go
func (v *PgVectorDB) Validate(ctx context.Context) error {
    // 1. Enable pgvector extension (REQUIRED — VECTOR type doesn't exist without it)
    _, err := v.db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS vector`)
    if err != nil {
        return fmt.Errorf("failed to create vector extension: %w", err)
    }

    // 2. Create table (BIGSERIAL for Neon/Postgres, not STRING like CRDB)
    _, err = v.db.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS agent_vectors (
            id BIGSERIAL PRIMARY KEY,
            tenant_id INT NOT NULL,
            content_type TEXT NOT NULL,
            title TEXT,
            content TEXT NOT NULL,
            embedding VECTOR(1536),
            metadata JSONB,
            source_version INT DEFAULT 1,
            created_at TIMESTAMPTZ DEFAULT now()
        )
    `)
    if err != nil {
        return fmt.Errorf("failed to create agent_vectors table: %w", err)
    }

    // 3. B-tree index for tenant filter
    _, err = v.db.ExecContext(ctx, `
        CREATE INDEX IF NOT EXISTS idx_agent_vectors_tenant ON agent_vectors (tenant_id)
    `)
    if err != nil {
        return fmt.Errorf("failed to create tenant index: %w", err)
    }

    // 4. HNSW vector index (pgvector syntax — NOT CRDB CREATE VECTOR INDEX)
    _, err = v.db.ExecContext(ctx, `
        CREATE INDEX IF NOT EXISTS idx_agent_vectors_embedding
        ON agent_vectors USING hnsw (embedding vector_cosine_ops)
    `)
    if err != nil {
        slog.Warn("HNSW index creation failed", "error", err)
        // Non-fatal: table still works without index (brute-force search)
    }

    return nil
}
```

### Full Interface Implementation

The `CockroachVectorDB` must implement all 12 methods from `VectorDB` interface (`vectordb.go:30-75`):

```go
type CockroachVectorDB struct {
    db      *sql.DB
    embedSvc EmbeddingService
    config  *VectorDBConfig
}

// Dimension returns the embedding dimension (1536 for text-embedding-3-small).
func (v *CockroachVectorDB) Dimension() int { return 1536 }

// Close releases the database connection.
func (v *CockroachVectorDB) Close() error { return v.db.Close() }

// Stats returns database statistics.
func (v *CockroachVectorDB) Stats(ctx context.Context) (*VectorDBStats, error) {
    var totalChunks int64
    err := v.db.QueryRowContext(ctx, `SELECT count(*) FROM agent_vectors`).Scan(&totalChunks)
    if err != nil { return nil, err }
    return &VectorDBStats{TotalChunks: totalChunks}, nil
}

// ListChunks returns all chunks for a given tenant.
func (v *CockroachVectorDB) ListChunks(ctx context.Context, tenantID int32) ([]DocumentChunk, error) {
    // Query agent_vectors WHERE tenant_id = $1
    // Scan into DocumentChunk structs
}

// Insert adds or updates chunks (single-row UPSERT via crdb.ExecuteTx).
func (v *CockroachVectorDB) Insert(ctx context.Context, chunks []DocumentChunk) error {
    for _, chunk := range chunks {
        embeddingJSON, _ := json.Marshal(chunk.Embedding)
        err := crdb.ExecuteTx(ctx, v.db, nil, func(tx *sql.Tx) error {
            _, err := tx.ExecContext(ctx, `
                UPSERT INTO agent_vectors (id, tenant_id, content_type, title, content, embedding, metadata, source_version, created_at)
                VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
            `, chunk.ID, chunk.TenantID, chunk.ContentType, chunk.Title, chunk.Content,
                embeddingJSON, "{}", chunk.SourceVersion, time.Now())
            return err
        })
        if err != nil { return fmt.Errorf("failed to insert vector: %w", err) }
    }
    return nil
}

// InsertWithCheckpoint adds chunks with progress tracking.
func (v *CockroachVectorDB) InsertWithCheckpoint(ctx context.Context, chunks []DocumentChunk, opts InsertOptions) error {
    // Similar to Insert but calls opts.ProgressCallback after each chunk
}

// Delete removes chunks matching filter criteria.
func (v *CockroachVectorDB) Delete(ctx context.Context, tenantID int32, audienceType string) error {
    _, err := v.db.ExecContext(ctx, `DELETE FROM agent_vectors WHERE tenant_id = $1 AND content_type = $2`, tenantID, audienceType)
    return err
}

// DeleteByVersion removes chunks for a specific version.
func (v *CockroachVectorDB) DeleteByVersion(ctx context.Context, tenantID int32, audienceType, fileType string, version int32) error {
    _, err := v.db.ExecContext(ctx, `DELETE FROM agent_vectors WHERE tenant_id = $1 AND content_type = $2 AND source_version = $3`, tenantID, fileType, version)
    return err
}

// PurgePreVersionedChunks removes chunks that predate versioning.
func (v *CockroachVectorDB) PurgePreVersionedChunks(ctx context.Context, tenantID int32, audienceType, fileType string) error {
    _, err := v.db.ExecContext(ctx, `DELETE FROM agent_vectors WHERE tenant_id = $1 AND content_type = $2 AND (source_version IS NULL OR source_version <= 1)`, tenantID, fileType)
    return err
}

// DeleteByIDPrefix removes chunks whose IDs start with prefix.
func (v *CockroachVectorDB) DeleteByIDPrefix(ctx context.Context, tenantID int32, idPrefix string) (int, error) {
    result, err := v.db.ExecContext(ctx, `DELETE FROM agent_vectors WHERE tenant_id = $1 AND id LIKE $2 || '%'`, tenantID, idPrefix)
    if err != nil { return 0, err }
    count, _ := result.RowsAffected()
    return int(count), nil
}

// ListIndexedVersions returns distinct source_version values.
func (v *CockroachVectorDB) ListIndexedVersions(ctx context.Context, tenantID int32, audienceType, fileType string) ([]int32, error) {
    rows, err := v.db.QueryContext(ctx, `SELECT DISTINCT source_version FROM agent_vectors WHERE tenant_id = $1 AND content_type = $2 ORDER BY source_version`, tenantID, fileType)
    if err != nil { return nil, err }
    defer rows.Close()
    var versions []int32
    for rows.Next() {
        var v int32
        if err := rows.Scan(&v); err != nil { return nil, err }
        versions = append(versions, v)
    }
    return versions, nil
}

// Search performs vector similarity search.
func (v *CockroachVectorDB) Search(ctx context.Context, query SearchQuery) (*SearchResult, error) {
    // 1. Generate embedding for query text
    embeddings, err := v.embedSvc.Embed(ctx, []string{query.QueryText})
    if err != nil { return nil, err }
    queryEmbedding := embeddings[0]

    // 2. Marshal embedding to JSON for SQL
    embeddingJSON, _ := json.Marshal(queryEmbedding)

    // 3. Build query with filters
    // SELECT id, title, content, content_type, metadata, source_version, created_at,
    //        1 - (embedding <=> $1::VECTOR) AS similarity
    // FROM agent_vectors
    // WHERE tenant_id = $2 AND content_type = ANY($3)
    // ORDER BY embedding <=> $1::VECTOR
    // LIMIT $4;

    // 4. Execute and scan into SearchResult
}
```

### Modified File: `vectordb.go`

Add `"cockroach"` case to `NewVectorDB()` factory (`vectordb.go:274-286`):
```go
case "cockroach":
    slog.Info("Using CockroachDB distributed vector database")
    return NewCockroachVectorDB(config, embedSvc)
```

Add `CockroachDSN` field to `VectorDBConfig` (`vectordb.go:78-107`):
```go
CockroachDSN string // COCKROACH_DSN env var. If empty, reuses existing *sql.DB from store.Driver.GetDB()
```

**Connection pool strategy:** If `COCKROACH_DSN` is empty or matches `MEMOS_DSN`, reuse the existing `*sql.DB` from `store.Driver.GetDB()` (`postgres.go:56`). Only open a separate pool when `COCKROACH_DSN` explicitly differs. This avoids double-pooling and CRDB Serverless connection limits.

Update `NewVectorDBConfigFromEnv()` (`vectordb.go:110-134`) to read `VECTOR_DB_PROVIDER` and `COCKROACH_DSN`.

### New File: `vectordb_cockroach.go` build tag

```go
//go:build cockroach
```

Create `vectordb_nocockroach.go` with `//go:build !cockroach` stub (same pattern as `vectordb_nolance.go`):

```go
//go:build !cockroach

package agent

import (
    "context"
    "fmt"
)

// CockroachVectorDB is a stub when building without the cockroach tag.
type CockroachVectorDB struct{}

func NewCockroachVectorDB(config *VectorDBConfig, embedSvc EmbeddingService) (VectorDB, error) {
    return nil, fmt.Errorf("CockroachDB vector storage requires cockroach build tag. Use 'go build -tags cockroach' or set VECTOR_DB_PROVIDER=memory")
}

// All 12 VectorDB interface methods must be stubbed
func (v *CockroachVectorDB) Insert(_ context.Context, _ []DocumentChunk) error {
    return fmt.Errorf("CockroachDB vector storage unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) InsertWithCheckpoint(_ context.Context, _ []DocumentChunk, _ InsertOptions) error {
    return fmt.Errorf("CockroachDB vector storage unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) Delete(_ context.Context, _ int32, _ string) error {
    return fmt.Errorf("CockroachDB vector storage unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) DeleteByVersion(_ context.Context, _ int32, _ string, _ string, _ int32) error {
    return fmt.Errorf("CockroachDB vector storage unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) PurgePreVersionedChunks(_ context.Context, _ int32, _ string, _ string) error {
    return fmt.Errorf("CockroachDB vector storage unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) DeleteByIDPrefix(_ context.Context, _ int32, _ string) (int, error) {
    return 0, fmt.Errorf("CockroachDB vector storage unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) ListIndexedVersions(_ context.Context, _ int32, _ string, _ string) ([]int32, error) {
    return nil, fmt.Errorf("CockroachDB vector storage unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) Search(_ context.Context, _ SearchQuery) (*SearchResult, error) {
    return nil, fmt.Errorf("CockroachDB vector storage unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) Close() error { return nil }
func (v *CockroachVectorDB) Stats(_ context.Context) (*VectorDBStats, error) {
    return nil, fmt.Errorf("CockroachDB vector storage unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) ListChunks(_ context.Context, _ int32) ([]DocumentChunk, error) {
    return nil, fmt.Errorf("CockroachDB vector storage unavailable (non-cockroach build)")
}
func (v *CockroachVectorDB) Dimension() int { return 0 }
func (v *CockroachVectorDB) Validate(_ context.Context) error {
    return fmt.Errorf("CockroachDB vector storage unavailable (non-cockroach build)")
}
```

---

## Component 1b: Database Migration — REMOVED

### Why Migration Is Removed

The migration file `035/00__agent_vectors.sql` is **not needed**. Schema creation moves entirely to `Validate()` per provider.

**Root cause:** The `VECTOR(1536)` column type has different requirements per database:

| Database | Requirement | Migration Impact |
|----------|-------------|-----------------|
| CockroachDB | Native VECTOR type | `CREATE TABLE ... VECTOR(1536)` works |
| Neon | pgvector extension | `CREATE EXTENSION IF NOT EXISTS vector` required first |
| Standard Postgres | pgvector extension | Same as Neon |
| SQLite | N/A | Different migration path |

**The problem:** A single migration file cannot satisfy all three requirements:
- `CREATE EXTENSION IF NOT EXISTS vector` → **FAILS on CockroachDB** (no extension system)
- `CREATE TABLE ... VECTOR(1536)` without extension → **FAILS on Neon/Postgres**

**The solution:** Each VectorDB provider's `Validate()` method handles its own schema creation:
- CockroachDB `Validate()`: Native VECTOR type + `CREATE VECTOR INDEX`
- Neon/Postgres `Validate()`: `CREATE EXTENSION` + VECTOR type + `USING hnsw` index
- SQLite: Skip entirely

### Backfill Safety

If creating the vector index after inserting data, CockroachDB warns: *"Adding a vector index to a non-empty table can temporarily disrupt workloads."* For the hackathon demo, create the index on an empty table before seeding data.

---

## Component 2: Ticket Embedding Pipeline

### New File: `ticket_embedder.go`

Embeds tickets and their comments into `agent_vectors` for cross-reference search.

**Flow:**
1. Query `agent_tickets` where `updated_ts > last_checkpoint`
2. Format ticket data into searchable chunks:
   ```
   [Ticket #104] Title: Bulk CSV import 504 Timeout
   Description: User experienced 504 Gateway Timeout during CSV import.
   Resolution: Tuned DB pool connection limit to 50 and added batching.
   ```
3. Generate 1536-dim embeddings via `EmbeddingService` (`embedding.go`)
4. Single-row UPSERT into `agent_vectors` with `content_type = "ticket"` and deterministic ID: `fmt.Sprintf("ticket:%d:%d", ticketID, commentID)`

**Cron registration** — use bchat's existing `plugin/cron/`, registered in `server/router/api/v1/agent/service.go` `NewService()` constructor:
```go
// In NewService() constructor, after service initialization:
func NewService(...) *Service {
    s := &Service{...}

    // Register ticket embedder cron job (CockroachDB only)
    if os.Getenv("VECTOR_DB_PROVIDER") == "cockroach" {
        cronJob := cron.New()
        cronJob.AddFunc("*/5 * * * *", func() {
            s.ticketEmbedder.ProcessPending(context.Background(), tenantID)
        })
        cronJob.Start()
    }

    return s
}
```

**Idempotency:** `INSERT ... ON CONFLICT (id) DO UPDATE` ensures re-indexing overwrites, never duplicates.

---

## Component 3: Chat Widget Escalation

### Modified File: `handlers.go`

Add `/api/v1/agent/:slug/chat/escalate` endpoint:
- Accepts session ID and user message context
- Creates ticket via `ticket_service.go` (`CreateTicket` at `ticket_service.go:57`)
- Returns AI confirmation: *"Your issue has been escalated to ticket #105."*

### Modified File: `service.go`

The CRDB ticket search is **additive** to the existing RAG retrieval. In the response generation flow:
1. First, retrieve KB content via existing `RetrieveContextForQuery()` (`vectordb.go:1048-1098`)
2. Then, retrieve past tickets via CRDB vector search (new code below)
3. Merge both into the LLM prompt

```go
// After existing RetrieveContextForQuery call in service.go:
if v.vectorDB != nil && config.StorageProvider == "cockroach" {
    ticketResults, err := v.vectorDB.Search(ctx, SearchQuery{
        QueryText:    userMessage,
        TenantID:     tenantID,
        ContentTypes: []string{"kb", "ticket"},
        TopK:         5,
    })
    if err == nil && len(ticketResults.Chunks) > 0 {
        sb.WriteString("\n## Similar Past Resolved Tickets\n")
        for _, chunk := range ticketResults.Chunks {
            sb.WriteString(fmt.Sprintf("- %s (Similarity: %.2f): %s\n",
                chunk.Title, chunk.Score, chunk.Content))
        }
    }
}
```

---

## Component 4: AWS Deployment

### New File: `Dockerfile.ecs`

Based on existing `Dockerfile.fly` but with `CGO_ENABLED=0` (no LanceDB CGO):
```dockerfile
FROM golang:1.26 AS backend
ENV CGO_ENABLED=0
RUN go build -tags cockroach -ldflags="-s -w" -o memos ./bin/memos/main.go
```

### New File: `deploy/ecs/task-definition.json`

Fargate task with `COCKROACH_DSN`, `VECTOR_DB_PROVIDER=cockroach`, Bedrock/S3 env vars.

### New File: `deploy/ccloud/setup.sh`

ccloud CLI commands for cluster provisioning (all verified against `ccloud.md`):
```bash
#!/bin/bash
# Create a CockroachDB Basic cluster on AWS
# Source: ccloud.md:47-70 — "ccloud cluster create basic"
ccloud cluster create basic bchat-db us-east-1 --cloud AWS --spend-limit 0

# Wait for cluster to be ready
echo "Waiting for cluster to be ready..."
sleep 60

# Create SQL user
# Source: ccloud.md:303-314 — "ccloud cluster user create"
ccloud cluster user create bchat-db bchat

# Get connection URL
# Source: ccloud.md:316-329 — "ccloud cluster sql --connection-url"
CONNECTION_URL=$(ccloud cluster sql --connection-url bchat-db)
echo "COCKROACH_DSN=${CONNECTION_URL}"

# Allowlist current developer IP
# Source: ccloud.md:807-874 — "ccloud cluster networking allowlist create"
CURRENT_IP=$(curl -s https://checkip.amazonaws.com)
ccloud cluster networking allowlist create bchat-db "${CURRENT_IP}/32" --sql --ui --name "Developer"
```

---

## Component 5: Seed Data & Demo

### New File: `cmd/seed/seed_demo_tickets.go`

Standalone CLI that:
1. Creates 50 realistic tickets (OAuth errors, CSV timeouts, webhook failures, etc.)
2. Generates embeddings for each ticket via Bedrock/OpenRouter
3. Inserts into `agent_vectors` with `content_type = "ticket"`
4. Pre-computes embeddings (no wait time during demo)

---

## RAG-Based Jira Cross-Ticket Resolution Testing

### Test File: `ticket_resolution_test.go`

This is the critical test that proves the hackathon's value proposition: "an agent can resolve a customer's issue by finding similar past tickets in CockroachDB."

#### Test Structure

```go
//go:build cockroach,integration

package agent

import (
    "context"
    "os"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
    if os.Getenv("COCKROACH_DSN") == "" {
        println("SKIP: COCKROACH_DSN not set, skipping integration tests")
        os.Exit(0)
    }
    os.Exit(m.Run())
}

// TestTicketResolution_BasicCrossReference proves the core value:
// A new customer query finds a similar past resolved ticket.
func TestTicketResolution_BasicCrossReference(t *testing.T) {
    // SETUP: Create CockroachVectorDB connection
    // SEED: Insert 10 tickets with known resolutions
    // QUERY: Simulate a customer saying "CSV import timing out"
    // VERIFY: Search returns Ticket #104 (the CSV timeout ticket) with similarity > 0.8
}

// TestTicketResolution_KnowledgeBaseCrossReference proves KB + tickets
// are searched together.
func TestTicketResolution_KnowledgeBaseCrossReference(t *testing.T) {
    // SETUP: Insert KB articles about CSV imports AND past tickets
    // QUERY: "How do I fix CSV import errors?"
    // VERIFY: Results include both KB article AND resolved ticket
}

// TestTicketResolution_TenantIsolation proves cross-tenant data doesn't leak.
func TestTicketResolution_TenantIsolation(t *testing.T) {
    // SETUP: Tenant A has a ticket about "OAuth error"
    //        Tenant B has NO such ticket
    // QUERY: Tenant B searches for "OAuth error"
    // VERIFY: Results do NOT include Tenant A's ticket
}

// TestTicketResolution_DuplicateDetection proves the agent can detect
// when a new issue matches an existing ticket.
func TestTicketResolution_DuplicateDetection(t *testing.T) {
    // SETUP: Insert 3 tickets about the same root cause (different symptoms)
    // QUERY: New customer reports a related symptom
    // VERIFY: Search returns all 3 related tickets
}

// TestTicketResolution_EscalationFlow proves the full widget → ticket → search flow.
func TestTicketResolution_EscalationFlow(t *testing.T) {
    // 1. Create a session via the chat widget
    // 2. User says "My webhook is failing with 429 errors"
    // 3. Agent searches tickets, finds similar past issue
    // 4. Agent responds with resolution from past ticket
    // 5. User clicks "Escalate to Human"
    // 6. Verify ticket created in ticket_service.go
    // 7. Verify new ticket is now searchable for future queries
}
```

#### Test Data

```go
var testTickets = []struct {
    ID          int32
    Title       string
    Description string
    Resolution  string
    Status      string
}{
    {101, "OAuth SSO Redirect Error on Safari", "Users getting redirect loop after SSO login on Safari", "Disabled SameSite=Strict on session cookie", "CLOSED"},
    {102, "Webhook 429 Rate Limit Exceeded", "Third-party webhook returning 429 after 100 requests/min", "Implemented exponential backoff with jitter", "CLOSED"},
    {103, "CSV Import Timeout on Large Files", "504 Gateway Timeout when importing CSV with 5000+ rows", "Tuned DB pool limit to 50 and chunked imports into 500-row batches", "CLOSED"},
    {104, "Dashboard Loading Slow", "Dashboard takes 30s to load with 10K+ records", "Added pagination and server-side filtering", "CLOSED"},
    {105, "Email Notifications Not Sending", "Users not receiving email notifications after signup", "Fixed SMTP connection pool exhaustion, increased max connections to 20", "CLOSED"},
}
```

#### How to Run

```bash
# Requires CockroachDB cluster with vector index enabled
# Tests are skipped if COCKROACH_DSN is not set (TestMain guard)
COCKROACH_DSN="postgresql://user:pass@cluster.cockroachlabs.cloud:26257/bchat?sslmode=verify-full" \
VECTOR_DB_PROVIDER=cockroach \
go test -tags cockroach,integration -run TestTicketResolution ./server/router/api/v1/agent/ -v -timeout 60s
```

#### What This Test Proves to Judges

| Test | Judging Criterion |
|------|------------------|
| `BasicCrossReference` | **Agentic Memory Design** — Agent uses CockroachDB vectors to find past resolutions |
| `KnowledgeBaseCrossReference` | **Technical Implementation** — Hybrid KB + ticket search works |
| `TenantIsolation` | **Production Readiness** — Multi-tenant security is enforced |
| `DuplicateDetection` | **Real-World Impact** — Reduces support ticket volume |
| `EscalationFlow` | **Creativity** — Full widget → ticket → AI resolution pipeline |

---

## Files Summary

### New Files
| File | Lines | Purpose |
|------|-------|---------|
| `vectordb_cockroach.go` | ~320 | CockroachDB vector store + vector index creation in Validate() |
| `vectordb_nocockroach.go` | ~70 | Build tag stub (all 12 VectorDB methods) |
| `ticket_embedder.go` | ~120 | Async ticket embedding pipeline |
| `ticket_resolution_test.go` | ~220 | RAG cross-ticket resolution tests (integration) |
| `Dockerfile.ecs` | ~60 | AWS ECS Docker build (CGO_ENABLED=0) |
| `deploy/ecs/task-definition.json` | ~50 | ECS Fargate task |
| `deploy/ccloud/setup.sh` | ~40 | ccloud CLI provisioning (verified commands) |
| `cmd/seed/seed_demo_tickets.go` | ~100 | Seed data generator |

### Modified Files
| File | Change | Lines |
|------|--------|-------|
| `vectordb.go` | Add "cockroach" case to factory, add `CockroachDSN` to config | ~20 |
| `handlers.go` | Add `/escalate` endpoint | ~30 |
| `service.go` | Add vector search to response generation | ~20 |
| `go.mod` | Add `cockroach-go/v2` dependency | ~2 |

### Removed Files
| File | Reason |
|------|--------|
| `store/migration/postgres/035/00__agent_vectors.sql` | Schema creation moved to Validate() per provider |

---

## Prerequisites

1. **CockroachDB cluster** with `feature.vector_index.enabled = true`
2. **AWS account** with ECS, Bedrock, S3 access
3. **CockroachDB DSN** (from cluster connection string)
4. **ccloud CLI** installed and authenticated (`ccloud auth login`)

## Verification Commands

```bash
# 1. Verify vector search works
COCKROACH_DSN="..." VECTOR_DB_PROVIDER=cockroach go test -tags cockroach,integration -run TestTicketResolution_BasicCrossReference ./server/router/api/v1/agent/ -v

# 2. Verify tenant isolation
COCKROACH_DSN="..." VECTOR_DB_PROVIDER=cockroach go test -tags cockroach,integration -run TestTicketResolution_TenantIsolation ./server/router/api/v1/agent/ -v

# 3. Full integration test
COCKROACH_DSN="..." VECTOR_DB_PROVIDER=cockroach go test -tags cockroach,integration -run TestTicketResolution_EscalationFlow ./server/router/api/v1/agent/ -v

# 4. Verify Docker image (no LanceDB binaries)
docker build -f Dockerfile.ecs -t bchat:crdb . && docker run --rm bchat:crdb ls -la /usr/local/lib/ | grep -c lancedb
# Expected: 0 (no LanceDB binaries)

# 5. Verify existing builds still work (no cockroach tag)
go build ./bin/memos/main.go                    # SQLite (no cockroach tag)
go build -tags cockroach ./bin/memos/main.go    # CockroachDB
go build -tags rag ./bin/memos/main.go          # LanceDB
```

---

## Component 6: Easy Deployment via Taskfile

Mirrors the existing `fly:*` task pattern for CockroachDB + AWS ECS deployment.

### Build Tasks

```yaml
  build:backend:cockroach:
    desc: Build Go binary with CockroachDB vector support (no LanceDB CGO)
    deps: [validate:migrations, validate:parity]
    env:
      CGO_ENABLED: "0"
    cmds:
      - mkdir -p build
      - go build -tags cockroach -ldflags="-s -w" -o build/memos ./bin/memos/main.go

  build:cockroach:
    desc: Build everything for CockroachDB deployment (frontend + backend)
    deps: [build:frontend, build:backend:cockroach]
```

### Run Tasks (Local Development with CockroachDB)

```yaml
  run:cockroach:
    desc: Run locally against CockroachDB (sources .env file)
    deps: [build:backend:cockroach]
    cmds:
      - |
        if [ -f .env ]; then
          echo "Loading environment from .env file..."
          set -a && . .env && set +a
        fi
        VECTOR_DB_PROVIDER=cockroach ./build/memos --mode dev --data {{.ROOT_DIR}}/build/data

  run:cockroach:seed:
    desc: Seed demo tickets into CockroachDB for hackathon demo
    deps: [build:backend:cockroach]
    cmds:
      - |
        if [ -f .env ]; then
          set -a && . .env && set +a
        fi
        go run ./cmd/seed/seed_demo_tickets.go
```

### CockroachDB Cluster Management Tasks

All commands verified against `ccloud.md` (official CockroachDB Cloud CLI documentation).

```yaml
  crdb:auth:
    desc: Authenticate with CockroachDB Cloud (opens browser)
    # Source: ccloud.md:27-31
    cmds:
      - ccloud auth login

  crdb:auth:check:
    desc: Check current CockroachDB Cloud authentication status
    # Source: ccloud.md:280
    cmds:
      - ccloud auth whoami

  crdb:cluster:create:
    desc: Create a new CockroachDB Basic cluster on AWS (usage: task crdb:cluster:create NAME=bchat-db REGION=us-east-1)
    # Source: ccloud.md:47-70 — "ccloud cluster create basic"
    cmds:
      - ccloud cluster create basic {{.NAME}} {{.REGION | default "us-east-1"}} --cloud AWS --spend-limit 0

  crdb:cluster:create:standard:
    desc: Create a CockroachDB Standard cluster on AWS
    # Source: ccloud.md:72-95 — "ccloud cluster create standard"
    cmds:
      - ccloud cluster create standard {{.NAME}} {{.REGION | default "us-east-1"}} --cloud AWS

  crdb:cluster:create:advanced:
    desc: Create a CockroachDB Advanced cluster on AWS
    # Source: ccloud.md:97-126 — "ccloud cluster create dedicated"
    cmds:
      - ccloud cluster create dedicated {{.NAME}} {{.REGION | default "us-east-1"}}:{{.NODES | default 1}} --cloud AWS --vcpus 4 --storage-gib 110

  crdb:cluster:info:
    desc: Get cluster info and connection details (usage: task crdb:cluster:info NAME=bchat-db)
    # Source: ccloud.md:143-184
    cmds:
      - ccloud cluster info {{.NAME}}

  crdb:cluster:list:
    desc: List all CockroachDB clusters in the organization
    # Source: ccloud.md:128-141
    cmds:
      - ccloud cluster list

  crdb:cluster:delete:
    desc: Delete a CockroachDB cluster (usage: task crdb:cluster:delete NAME=bchat-db)
    # Source: ccloud.md:186-202
    cmds:
      - ccloud cluster delete {{.NAME}}

  crdb:sql:create-user:
    desc: Create a SQL user (usage: task crdb:sql:create-user NAME=bchat-db USER=bchat)
    # Source: ccloud.md:303-314 — no --permissions flag exists
    cmds:
      - ccloud cluster user create {{.NAME}} {{.USER}}

  crdb:sql:shell:
    desc: Open interactive SQL shell (usage: task crdb:sql:shell NAME=bchat-db)
    # Source: ccloud.md:204-236 — auto-creates user if none exist
    cmds:
      - ccloud cluster sql {{.NAME}}

  crdb:sql:sso:
    desc: Open SQL shell with SSO authentication (no password needed)
    # Source: ccloud.md:238-278
    cmds:
      - ccloud cluster sql --sso {{.NAME}}

  crdb:sql:url:
    desc: Get connection URL for .env (usage: task crdb:sql:url NAME=bchat-db)
    # Source: ccloud.md:316-329
    cmds:
      - ccloud cluster sql --connection-url {{.NAME}}

  crdb:sql:params:
    desc: Get individual connection parameters (host, port, database)
    # Source: ccloud.md:331-343
    cmds:
      - ccloud cluster sql --connection-params {{.NAME}}

  crdb:ip:allow:
    desc: Add current developer IP to allowlist (usage: task crdb:ip:allow NAME=bchat-db)
    # Source: ccloud.md:807-874
    cmds:
      - |
        CURRENT_IP=$(curl -s https://checkip.amazonaws.com)
        ccloud cluster networking allowlist create {{.NAME}} "${CURRENT_IP}/32" \
          --sql --ui --name "Developer - $(whoami)"

  crdb:ip:allow:ecs:
    desc: Add ECS task outbound IP to allowlist (usage: task crdb:ip:allow:ecs NAME=bchat-db CIDR=10.0.0.0/24)
    # Source: ccloud.md:807-874 — for ECS tasks with NAT gateway
    cmds:
      - ccloud cluster networking allowlist create {{.NAME}} "{{.CIDR}}" --sql --name "ECS tasks"

  crdb:ip:list:
    desc: List IP allowlist entries (usage: task crdb:ip:list NAME=bchat-db)
    # Source: ccloud.md:826-836
    cmds:
      - ccloud cluster networking allowlist list {{.NAME}}

  crdb:ip:delete:
    desc: Remove IP from allowlist (usage: task crdb:ip:delete NAME=bchat-db CIDR=1.1.1.1/32)
    # Source: ccloud.md:863-874
    cmds:
      - ccloud cluster networking allowlist delete {{.NAME}} {{.CIDR}}

  crdb:backup:list:
    desc: List backups for a cluster
    # Source: ccloud.md:384-399
    cmds:
      - ccloud cluster backup list {{.NAME}}

  crdb:backup:config:
    desc: Get backup configuration (frequency, retention)
    # Source: ccloud.md:401-413
    cmds:
      - ccloud cluster backup config get {{.NAME}}

  crdb:db:list:
    desc: List databases in a cluster
    # Source: ccloud.md:345-360
    cmds:
      - ccloud cluster database list {{.NAME}}

  crdb:db:create:
    desc: Create a database in a cluster (usage: task crdb:db:create NAME=bchat-db DB=bchat)
    # Source: ccloud.md:362-371
    cmds:
      - ccloud cluster database create {{.NAME}} {{.DB}}

  crdb:versions:
    desc: List available CockroachDB versions
    # Source: ccloud.md:473-487
    cmds:
      - ccloud cluster versions
```

### Pre-Deployment Checks

```yaml
  crdb:check:
    desc: Validate CockroachDB environment chain (.env -> config)
    cmds:
      - |
        echo "=== Checking CockroachDB environment ==="
        if [ -z "$COCKROACH_DSN" ]; then
          echo "ERROR: COCKROACH_DSN not set"
          echo "Set it in .env or export COCKROACH_DSN='postgresql://...'"
          exit 1
        fi
        if [ "$VECTOR_DB_PROVIDER" != "cockroach" ]; then
          echo "WARNING: VECTOR_DB_PROVIDER is not 'cockroach' (current: $VECTOR_DB_PROVIDER)"
          echo "Setting VECTOR_DB_PROVIDER=cockroach for this deployment"
          export VECTOR_DB_PROVIDER=cockroach
        fi
        echo "COCKROACH_DSN: set"
        echo "VECTOR_DB_PROVIDER: $VECTOR_DB_PROVIDER"
        echo "=== Environment check passed ==="

  crdb:db-check:
    desc: Validate database migrations against CockroachDB
    cmds:
      - task: validate:migrations
      - task: validate:parity
      - |
        echo "=== CockroachDB migration validation passed ==="

  crdb:pre-deploy:
    desc: Run all pre-deployment checks for CockroachDB + AWS ECS
    cmds:
      - task: crdb:check
      - task: crdb:db-check
      - task: build:cockroach
      - |
        echo ""
        echo "=== All CockroachDB pre-deployment checks passed ==="
        echo "Ready to deploy: task crdb:deploy"
```

### AWS ECS Deployment Tasks

```yaml
  crdb:deploy:
    desc: Build and deploy to AWS ECS Fargate (usage: task crdb:deploy CLUSTER=bchat-ecs SERVICE=bchat)
    deps: [build:cockroach]
    cmds:
      - |
        echo "=== Building Docker image ==="
        docker build -f Dockerfile.ecs -t bchat:crdb .
        echo "=== Pushing to ECR ==="
        AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
        AWS_REGION={{.AWS_REGION | default "us-east-1"}}
        ECR_URI="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/bchat"
        aws ecr get-login-password --region ${AWS_REGION} | docker login --username AWS --password-stdin ${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com
        docker tag bchat:crdb ${ECR_URI}:latest
        docker push ${ECR_URI}:latest
        echo "=== Updating ECS service ==="
        aws ecs update-service \
          --cluster {{.CLUSTER | default "bchat-ecs"}} \
          --service {{.SERVICE | default "bchat"}} \
          --force-new-deployment \
          --region ${AWS_REGION}
        echo "=== Deployment initiated ==="

  crdb:deploy:setup:
    desc: One-time AWS ECS setup (usage: task crdb:deploy:setup CLUSTER=bchat-ecs)
    cmds:
      - |
        AWS_REGION={{.AWS_REGION | default "us-east-1"}}
        echo "=== Creating ECR repository ==="
        aws ecr create-repository --repository-name bchat --region ${AWS_REGION} || true
        echo "=== Creating ECS cluster ==="
        aws ecs create-cluster --cluster-name {{.CLUSTER | default "bchat-ecs"}} --region ${AWS_REGION} || true
        echo "=== Setup complete ==="
```

### Monitoring & Debugging Tasks

```yaml
  crdb:logs:
    desc: Stream ECS logs for bchat (Ctrl+C to exit)
    cmds:
      - |
        AWS_REGION={{.AWS_REGION | default "us-east-1"}}
        aws logs tail /ecs/bchat --follow --region ${AWS_REGION}

  crdb:logs:rag:
    desc: Stream RAG and vector search logs from ECS
    cmds:
      - |
        AWS_REGION={{.AWS_REGION | default "us-east-1"}}
        aws logs tail /ecs/bchat --follow --region ${AWS_REGION} | grep -E "RAG|vector|CockroachDB|embedding|ticket"

  crdb:status:
    desc: Check ECS service status
    cmds:
      - |
        AWS_REGION={{.AWS_REGION | default "us-east-1"}}
        aws ecs describe-services \
          --cluster {{.CLUSTER | default "bchat-ecs"}} \
          --services bchat \
          --region ${AWS_REGION} \
          --query 'services[0].{status:status,desired:desiredCount,running:runningCount,health:healthCheckGracePeriodSeconds}'

  crdb:test:
    desc: Run RAG cross-ticket resolution tests against CockroachDB
    cmds:
      - |
        if [ -f .env ]; then
          set -a && . .env && set +a
        fi
        go test -tags cockroach,integration -run TestTicketResolution ./server/router/api/v1/agent/ -v -timeout 60s

  crdb:sql:query:
    desc: Run a SQL query against CockroachDB (usage: task crdb:sql:query SQL="SELECT count(*) FROM agent_vectors")
    cmds:
      - |
        if [ -f .env ]; then
          set -a && . .env && set +a
        fi
        psql "$COCKROACH_DSN" -c "{{.SQL}}"
```

### Quick Reference: Equivalent Commands

| Action | Fly.io | CockroachDB + AWS |
|--------|--------|-------------------|
| Pre-deploy check | `task fly:pre-deploy` | `task crdb:pre-deploy` |
| Deploy | `fly deploy` | `task crdb:deploy` |
| Stream logs | `fly logs` | `task crdb:logs` |
| SSH to DB | `fly ssh:db` | `task crdb:sql:shell` |
| Validate env | `task fly:check` | `task crdb:check` |
| Validate DB | `task fly:db-check` | `task crdb:db-check` |
| Run with RAG | `task run:rag` | `task run:cockroach` |
| Seed data | (manual) | `task run:cockroach:seed` |
| Run tests | `go test ...` | `task crdb:test` |
| Create cluster | `fly create` | `task crdb:cluster:create` |
| Delete cluster | `fly destroy` | `task crdb:cluster:delete` |
| SQL shell | `fly ssh:db` | `task crdb:sql:shell` |
| List backups | (none) | `task crdb:backup:list` |
| Allowlist IP | (none) | `task crdb:ip:allow` |
