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

```yaml
  crdb:auth:
    desc: Authenticate with CockroachDB Cloud (opens browser)
    cmds:
      - ccloud auth login

  crdb:auth:check:
    desc: Check current CockroachDB Cloud authentication status
    cmds:
      - ccloud auth whoami

  crdb:cluster:create:
    desc: Create a new CockroachDB cluster on AWS (usage: task crdb:cluster:create NAME=my-cluster REGION=us-east-1)
    cmds:
      - |
        ccloud cluster create \
          --name "{{.NAME}}" \
          --cloud-provider aws \
          --region "{{.REGION | default "us-east-1}}" \
          --nodes 3 \
          -o json

  crdb:cluster:info:
    desc: Get cluster info and connection details (usage: task crdb:cluster:info NAME=my-cluster)
    cmds:
      - ccloud cluster info {{.NAME}} -o json

  crdb:cluster:list:
    desc: List all CockroachDB clusters
    cmds:
      - ccloud cluster list

  crdb:sql:create-user:
    desc: Create a SQL user for bchat (usage: task crdb:sql:create-user NAME=my-cluster USER=bchat PASSWORD=secret)
    cmds:
      - ccloud cluster user create {{.NAME}} {{.USER}} -p '{{.PASSWORD}}'

  crdb:sql:shell:
    desc: Open interactive SQL shell (usage: task crdb:sql:shell NAME=my-cluster USER=bchat PASSWORD=secret)
    cmds:
      - ccloud cluster sql {{.NAME}} -u {{.USER}} -p '{{.PASSWORD}}'

  crdb:sql:url:
    desc: Get connection URL for .env (usage: task crdb:sql:url NAME=my-cluster USER=bchat PASSWORD=secret)
    cmds:
      - ccloud cluster sql {{.NAME}} -u {{.USER}} -p '{{.PASSWORD}}' --connection-url

  crdb:ip:allow:
    desc: Add current IP to allowlist (usage: task crdb:ip:allow NAME=my-cluster)
    cmds:
      - |
        CURRENT_IP=$(curl -s https://checkip.amazonaws.com)
        ccloud cluster networking allowlist create {{.NAME}} "${CURRENT_IP}/32" \
          --sql --ui --name "Developer - $(whoami)"

  crdb:ip:list:
    desc: List IP allowlist entries (usage: task crdb:ip:list NAME=my-cluster)
    cmds:
      - ccloud cluster networking allowlist list {{.NAME}}
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
        go test -tags cockroach -run TestTicketResolution ./server/router/api/v1/agent/ -v -timeout 60s

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
