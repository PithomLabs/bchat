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

## Component 1: CockroachDB Vector Store

### New File: `vectordb_cockroach.go`

Implements the `VectorDB` interface (`vectordb.go:30-75`) using CockroachDB's native vector support.

**Schema** (via migration, NOT in Go code — see Component 1b):
```sql
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
);

CREATE INDEX IF NOT EXISTS idx_agent_vectors_tenant ON agent_vectors (tenant_id);
```

**Vector index** (created separately per cockroachdb-skills syntax):
```sql
CREATE VECTOR INDEX idx_agent_vectors_embedding ON agent_vectors (embedding vector_cosine_ops);
```

**Note:** The skills repo documents `vector_ip_ops` (inner product). We use `vector_cosine_ops` for cosine distance (`<=>` operator) which matches bchat's existing similarity metric. If `vector_cosine_ops` is not available, fall back to `vector_ip_ops` and adjust the query operator to `<#>`.

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

### Modified File: `vectordb.go`

Add `"cockroach"` case to `NewVectorDB()` factory (`vectordb.go:274-286`):
```go
case "cockroach":
    slog.Info("Using CockroachDB distributed vector database")
    return NewCockroachVectorDB(config, embedSvc)
```

Add `CockroachDSN` field to `VectorDBConfig` (`vectordb.go:78-107`):
```go
CockroachDSN string // COCKROACH_DSN env var (or reuse MEMOS_DSN)
```

Update `NewVectorDBConfigFromEnv()` (`vectordb.go:110-134`) to read `VECTOR_DB_PROVIDER` and `COCKROACH_DSN`.

### New File: `vectordb_cockroach.go` build tag

```go
//go:build cockroach
```

Create `vectordb_nocockroach.go` with `//go:build !cockroach` stub (same pattern as `vectordb_nolance.go`).

---

## Component 1b: Database Migration

### New File: `store/migration/postgres/035/00__agent_vectors.sql`

```sql
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
);

CREATE INDEX IF NOT EXISTS idx_agent_vectors_tenant ON agent_vectors (tenant_id);
```

**Vector index** — created via CockroachDB cluster setting prerequisite:
```sql
-- Must be set on the CockroachDB cluster before creating vector indexes:
-- SET CLUSTER SETTING feature.vector_index.enabled = true;
-- Then create the index:
CREATE VECTOR INDEX idx_agent_vectors_embedding ON agent_vectors (embedding vector_cosine_ops);
```

**Why migration, not Go DDL:** bchat has a versioned migration system at `store/migration/postgres/` (currently at 0.34). DDL in application code violates this governance pattern.

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

**Cron registration** — use bchat's existing `plugin/cron/`, NOT Supercronic:
```go
// In service startup, register the embedder job
cronJob := cron.New()
cronJob.AddFunc("*/5 * * * *", func() {
    ticketEmbedder.ProcessPending(ctx, tenantID)
})
cronJob.Start()
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

When generating responses for support queries, perform vector search against `agent_vectors` with `content_type: ["kb", "ticket"]`:
```sql
SELECT content, metadata, 1 - (embedding <=> $1) AS similarity
FROM agent_vectors
WHERE tenant_id = $2 AND content_type IN ('kb', 'ticket')
ORDER BY embedding <=> $1
LIMIT 5;
```

Include results in the LLM prompt:
```
## Similar Past Resolved Tickets
- Ticket #104 (Similarity: 0.91): "Bulk CSV import 504 Timeout"
  Resolution: "Increased DB pool limit to 50."
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

ccloud CLI commands for cluster provisioning:
```bash
ccloud cluster create --cloud-provider aws --region us-east-1 --nodes 3
ccloud sql user create bchat_readonly --permissions READ
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
//go:build cockroach

package agent

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

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
COCKROACH_DSN="postgresql://user:pass@cluster.cockroachlabs.cloud:26257/bchat?sslmode=verify-full" \
VECTOR_DB_PROVIDER=cockroach \
go test -tags cockroach -run TestTicketResolution ./server/router/api/v1/agent/ -v -timeout 60s
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
| `vectordb_cockroach.go` | ~250 | CockroachDB vector store implementation |
| `vectordb_nocockroach.go` | ~20 | Build tag stub |
| `ticket_embedder.go` | ~120 | Async ticket embedding pipeline |
| `ticket_resolution_test.go` | ~200 | RAG cross-ticket resolution tests |
| `store/migration/postgres/035/00__agent_vectors.sql` | ~15 | Schema migration |
| `Dockerfile.ecs` | ~60 | AWS ECS Docker build |
| `deploy/ecs/task-definition.json` | ~50 | ECS Fargate task |
| `deploy/ccloud/setup.sh` | ~30 | ccloud CLI provisioning |
| `cmd/seed/seed_demo_tickets.go` | ~100 | Seed data generator |

### Modified Files
| File | Change | Lines |
|------|--------|-------|
| `vectordb.go` | Add "cockroach" case to factory, add `CockroachDSN` to config | ~15 |
| `handlers.go` | Add `/escalate` endpoint | ~30 |
| `service.go` | Add vector search to response generation | ~20 |
| `go.mod` | Add `cockroach-go/v2` dependency | ~2 |

---

## Prerequisites

1. **CockroachDB cluster** with `feature.vector_index.enabled = true`
2. **AWS account** with ECS, Bedrock, S3 access
3. **CockroachDB DSN** (from cluster connection string)
4. **ccloud CLI** installed and authenticated (`ccloud auth login`)

## Verification Commands

```bash
# 1. Verify vector search works
COCKROACH_DSN="..." VECTOR_DB_PROVIDER=cockroach go test -tags cockroach -run TestTicketResolution_BasicCrossReference ./server/router/api/v1/agent/ -v

# 2. Verify tenant isolation
COCKROACH_DSN="..." VECTOR_DB_PROVIDER=cockroach go test -tags cockroach -run TestTicketResolution_TenantIsolation ./server/router/api/v1/agent/ -v

# 3. Full integration test
COCKROACH_DSN="..." VECTOR_DB_PROVIDER=cockroach go test -tags cockroach -run TestTicketResolution_EscalationFlow ./server/router/api/v1/agent/ -v

# 4. Verify Docker image (no LanceDB binaries)
docker build -f Dockerfile.ecs -t bchat:crdb . && docker run --rm bchat:crdb ls -la /usr/local/lib/ | grep -c lancedb
# Expected: 0 (no LanceDB binaries)
```
