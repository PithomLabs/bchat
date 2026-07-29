# Code: CockroachDB Vector Store Implementation

**Date:** 2026-07-29
**Status:** Implemented
**Build Tag:** `cockroach`

---

## 1. Implementation Summary

### What Was Built

A CockroachDB-backed vector store for bchat's agentic memory system, enabling semantic ticket resolution via vector similarity search. Tickets are embedded into CockroachDB's native `VECTOR(1536)` type, clustered via topological sort, and surfaced through an escalation endpoint.

### File Inventory

| File | Type | Lines | Purpose |
|------|------|-------|---------|
| `server/router/api/v1/agent/vectordb_cockroach.go` | NEW | ~300 | Full VectorDB interface implementation |
| `server/router/api/v1/agent/vectordb_nocockroach.go` | NEW | ~70 | Build tag stub for non-cockroach builds |
| `server/router/api/v1/agent/ticket_embedder.go` | NEW | ~200 | Cron job: embed tickets, build clusters |
| `server/router/api/v1/agent/ticket_resolution_test.go` | NEW | ~150 | Integration tests |
| `server/router/api/v1/agent/vectordb.go` | MODIFIED | +10 | Factory case, config field |
| `server/router/api/v1/agent/service.go` | MODIFIED | +30 | CockroachDB wiring, cron registration, EscalateTicket |
| `server/router/api/v1/agent/handlers.go` | MODIFIED | +35 | /escalate endpoint, request struct |
| `Dockerfile.ecs` | NEW | ~40 | ECS Fargate build |
| `deploy/ecs/task-definition.json` | NEW | ~80 | ECS task definition |
| `deploy/ccloud/setup.sh` | NEW | ~60 | CockroachDB cluster setup |
| `cmd/seed/seed_demo_tickets.go` | NEW | ~150 | Demo ticket seeding |
| `Taskfile.yml` | MODIFIED | +130 | 12 crdb:* tasks |
| `go.mod` | MODIFIED | +1 | cockroach-go/v2 v2.4.3 |

---

## 2. Architecture

### Component Flow

```
┌─────────────────────────────────────────────────────────┐
│                    NewService()                          │
│  ┌──────────────┐    ┌──────────────────────────────┐   │
│  │ NewVectorDB() │───▶│ CockroachVectorDB (cockroach) │   │
│  └──────────────┘    └──────────┬───────────────────┘   │
│                                 │                        │
│  ┌──────────────┐    ┌──────────▼───────────────────┐   │
│  │ SetDB()      │───▶│ Shared *sql.DB connection pool │   │
│  └──────────────┘    └──────────────────────────────┘   │
│                                 │                        │
│  ┌──────────────┐    ┌──────────▼───────────────────┐   │
│  │ Cron (5min)  │───▶│ processPendingTickets()       │   │
│  └──────────────┘    │   ├─ embedTenantTickets()     │   │
│                      │   └─ buildTicketClusters()    │   │
│                      └──────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│                   HTTP Handlers                          │
│  POST /api/v1/agent/:slug/escalate                      │
│    └─▶ HandleEscalateTicket()                           │
│        └─▶ Service.EscalateTicket()                     │
│            ├─▶ store.CreateTicket()                     │
│            └─▶ vectorDB.Search() (similar tickets)     │
└─────────────────────────────────────────────────────────┘
```

### Build Tag Strategy

```
cockroach build:    vectordb_cockroach.go  (real impl)
non-cockroach:      vectordb_nocockroach.go (stub)
```

Files without the build tag always compile. The factory in `vectordb.go` routes to the correct implementation at runtime based on `VECTOR_DB_PROVIDER`.

### Database Schema

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

CREATE VECTOR INDEX idx_agent_vectors_embedding
ON agent_vectors (embedding vector_ip_ops);
```

**Note:** `CREATE VECTOR INDEX` does NOT support `IF NOT EXISTS`. The code checks for `pgcode 42P07` (duplicate_table) via `pgconn.PgError` and treats it as success.

### Vector Search Query

```sql
SELECT id, title, content, content_type, metadata, source_version, created_at,
       1 - (embedding <=> $1::VECTOR) AS similarity
FROM agent_vectors
WHERE tenant_id = $2 AND content_type IN ($3)
ORDER BY embedding <=> $1::VECTOR
LIMIT $4
```

---

## 3. Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `VECTOR_DB_PROVIDER` | Yes | `memory` | Set to `cockroach` for CockroachDB |
| `COCKROACH_DSN` | Yes | — | CockroachDB connection string |
| `RAG_PIPELINE_ENABLED` | Yes | `false` | Must be `true` |
| `TICKET_EMBEDDING_ENABLED` | No | `false` | Enable cron job (every 5 min) |
| `EMBEDDING_PROVIDER` | Yes | `mock` | Set to `openrouter` for production |
| `EMBEDDING_MODEL` | Yes | — | e.g., `openai/text-embedding-3-small` |
| `OPENROUTER_API_KEY` | Yes | — | OpenRouter API key |

---

## 4. Build & Run Commands

```bash
# Build with cockroach tag
task build:backend:cockroach

# Build without cockroach (default)
task build:backend

# Run with CockroachDB
task run:cockroach

# Run with mock embeddings (no API key)
VECTOR_DB_PROVIDER=cockroach COCKROACH_DSN="..." \
  EMBEDDING_PROVIDER=mock TICKET_EMBEDDING_ENABLED=true \
  ./build/memos --mode dev --data build/data

# Seed demo tickets
task run:cockroach:seed

# Run integration tests
task crdb:test

# Docker build
task crdb:docker:build

# Setup CockroachDB cluster
./deploy/ccloud/setup.sh
```

---

## 5. Testing Guide

### 5.1 Unit Tests (No External Dependencies)

```bash
# Run tests with mock embeddings
go test -tags cockroach ./server/router/api/v1/agent/... -run "TestProcessPendingTickets|TestEmbedTenantTickets|TestBuildTicketClusters|TestEscalateTicket" -v

# Expected: All 4 tests PASS (skip real DB operations)
```

### 5.2 Integration Tests (Requires CockroachDB)

```bash
# 1. Provision cluster
./deploy/ccloud/setup.sh

# 2. Set env vars
export COCKROACH_DSN="<connection-url from setup>"
export VECTOR_DB_PROVIDER=cockroach
export RAG_PIPELINE_ENABLED=true
export EMBEDDING_PROVIDER=openrouter
export OPENROUTER_API_KEY=sk-or-v1-xxx
export TICKET_EMBEDDING_ENABLED=true

# 3. Start server
task run:cockroach

# 4. Verify vector index creation
curl http://localhost:8081/api/v1/admin/rag/stats
# Should show "vector_db_provider": "cockroach"

# 5. Seed demo tickets
task run:cockroach:seed

# 6. Wait for cron (5 min) or trigger manually
# 7. Test escalation
curl -X POST http://localhost:8081/api/v1/agent/hackathon-demo/escalate \
  -H "Content-Type: application/json" \
  -d '{"title":"Water damage in bathroom","description":"Customer reports water damage from leaky pipe","priority":"high","tags":["water-damage","emergency"]}'
# Expected: {"ticket_id":1,"status":"created","similar_count":3}
```

### 5.3 Manual Verification

```bash
# Check table exists
ccloud cluster sql hackathon-demo -e "SHOW TABLES LIKE 'agent_vectors';"

# Check vector index
ccloud cluster sql hackathon-demo -e "SHOW INDEXES FROM agent_vectors;"

# Check embedded tickets
ccloud cluster sql hackathon-demo -e "SELECT id, title, content_type FROM agent_vectors WHERE content_type='ticket';"

# Check clusters
ccloud cluster sql hackathon-demo -e "SELECT id, title, content_type FROM agent_vectors WHERE content_type='cluster';"
```

### 5.4 Build Verification

```bash
# Both builds must compile
go build -tags cockroach ./bin/memos/...     # passes
go build ./bin/memos/...                     # passes (non-cockroach)

# Non-cockroach build returns errors on vector ops (expected)
go test ./server/router/api/v1/agent/... -run "TestVectorDBNonCockroach" -v
```

---

## 6. Adversarial Code Review Prompt

Copy and paste this prompt into Claude/Gemini for a thorough code review:

---

**PROMPT:**

```
You are performing an adversarial code review of a CockroachDB vector store implementation for a multi-tenant AI chat agent platform (bchat). This is a hackathon submission for CockroachDB × AWS.

Review the following files against the codebase conventions and CockroachDB best practices:

FILES TO REVIEW:
1. server/router/api/v1/agent/vectordb_cockroach.go (full VectorDB implementation)
2. server/router/api/v1/agent/vectordb_nocockroach.go (build tag stub)
3. server/router/api/v1/agent/ticket_embedder.go (cron job + clustering)
4. server/router/api/v1/agent/ticket_resolution_test.go (integration tests)
5. server/router/api/v1/agent/vectordb.go (factory modification)
6. server/router/api/v1/agent/service.go (CockroachDB wiring, EscalateTicket)
7. server/router/api/v1/agent/handlers.go (/escalate endpoint)
8. Dockerfile.ecs (ECS build)
9. deploy/ecs/task-definition.json (ECS task definition)
10. deploy/ccloud/setup.sh (CockroachDB setup)
11. cmd/seed/seed_demo_tickets.go (demo seeding)
12. Taskfile.yml (new crdb:* tasks)
13. go.mod (new dependency)

CONSTRAINTS TO VERIFY:
- lib/pq is BANNED (AGENTS.md line 32). Must use pgconn.PgError from pgx/v5/pgconn
- CREATE VECTOR INDEX does NOT support IF NOT EXISTS (use pgcode 42P07 check)
- $1::VECTOR cast uses UPPERCASE (skills repo 03-query-patterns.md:333)
- vector_ip_ops is the only verified opclass (not vector_cosine_ops)
- CockroachDB single-row inserts only (no batched VECTOR inserts)
- crdb.ExecuteTx operates on *sql.DB (correct for bchat's database/sql usage)
- Every API request must be scoped to tenant (ApplyTenantFilter pattern)
- Embedding JSON serialization must work with pgx (test manually)
- DSN comparison may fail for SQLite (SetDB only for Postgres deployments)
- Cron job must iterate tenants via ListAgentTenants (not hardcoded tenant)

REVIEW CHECKLIST:
[C-1] CRITICAL: Does vectordb_cockroach.go use pq.Error anywhere?
[C-2] CRITICAL: Does vectordb_cockroach.go use IF NOT EXISTS on CREATE VECTOR INDEX?
[C-3] CRITICAL: Does service.go or handlers.go expose tenant ID in error messages?
[C-4] CRITICAL: Does the /escalate endpoint skip tenant scoping?
[H-1] HIGH: Does Insert() handle NULL embeddings for pre-embedded chunks?
[H-2] HIGH: Does Search() handle empty query text gracefully?
[H-3] HIGH: Does cron job use context with timeout?
[H-4] HIGH: Does EscalateTicket validate ticket description format?
[M-1] MEDIUM: Does SetDB work for SQLite deployments?
[M-2] MEDIUM: Does vector index creation fail gracefully?
[M-3] MEDIUM: Does the Dockerfile expose correct port?
[M-4] MEDIUM: Does Taskfile use correct env var syntax?
[N-1] NIT: Is import ordering consistent?
[N-2] NIT: Are error messages wrapped with context?
[N-3] NIT: Is slog usage consistent with codebase conventions?

OUTPUT FORMAT:
For each finding, provide:
- File:line_number
- Severity: CRITICAL/HIGH/MEDIUM/NIT
- Description: What's wrong
- Fix: Exact code change

Also verify:
- All 4 integration tests compile and pass
- Both cockroach and non-cockroach builds succeed
- No new dependencies beyond cockroach-go/v2
- Existing VectorDB interface methods are all implemented
```

---

## 7. Known Limitations

| Issue | Severity | Mitigation |
|-------|----------|------------|
| `feature.vector_index.enabled` not verified | HIGH | May need CRDB v25.2+ or preview flag |
| Vector index creation may fail on older CRDB | MEDIUM | Table works without index (brute-force search) |
| Cron job runs every 5 minutes (fixed) | LOW | Could make configurable via env var |
| Embedding JSON serialization untested with pgx | HIGH | Test manually before demo |
| Demo seed script uses placeholder store methods | LOW | Complete after DB provisioning |

---

## 8. Rollback Plan

If CockroachDB integration fails:

1. Set `VECTOR_DB_PROVIDER=memory` (default, in-memory)
2. Remove `TICKET_EMBEDDING_ENABLED=true`
3. Non-cockroach build works without any changes
4. All existing LanceDB functionality preserved
