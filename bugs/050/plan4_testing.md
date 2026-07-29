# Plan: Local CockroachDB Testing with Docker (Final)

## Goal

Run CockroachDB locally via Docker to test the vector store integration before provisioning a real CRDB cluster. This validates schema creation, vector indexing, search queries, and cron jobs without cloud costs or network latency.

---

## Prerequisites

| Requirement | Status | Notes |
|-------------|--------|-------|
| Docker installed | Check with `docker --version` | Docker Desktop or Docker Engine |
| ~200 MB disk for image | `cockroachdb/cockroach` is ~180 MB compressed | |
| Ports 26257, 8080 free | SQL (26257) + DB Console (8080) | |
| Go 1.26+ | Already installed | |
| OPENROUTER_API_KEY | Required for embeddings | Can use `mock` provider for testing |

---

## Step 1: Pull CockroachDB Docker Image

```bash
docker pull cockroachdb/cockroach:v25.2.21
```

---

## Step 2: Start Single-Node Cluster

```bash
docker run -d \
  --name bchat-crdb \
  -p 26257:26257 \
  -p 8080:8080 \
  -v bchat_crdb_data:/cockroach/cockroach-data \
  cockroachdb/cockroach:v25.2.21 \
  start-single-node \
  --insecure \
  --advertise-addr=localhost
```

**Key flags:**
- `--insecure` — No TLS, no auth password (local dev only)
- `--advertise-addr=localhost` — Required for Docker networking
- `-v bchat_crdb_data:/cockroach/cockroach-data` — Persistent data across container restarts

**Verify:**
```bash
docker ps | grep bchat-crdb
# DB Console: http://localhost:8080
```

---

## Step 2.5: Enable Vector Indexes

```bash
docker exec -it bchat-crdb cockroach sql --insecure --host=localhost \
  -e "SET CLUSTER SETTING feature.vector_index.enabled = true;"
```

Without this, `CREATE VECTOR INDEX` will fail silently and the code will degrade to brute-force search (caught by `vectordb_cockroach.go:111-127` with a 42P07 warning).

---

## Step 3: Create Database and User

```bash
docker exec -it bchat-crdb cockroach sql --insecure --host=localhost \
  -e "CREATE DATABASE bchat; CREATE USER bchat_user WITH PASSWORD 'bchat_pass'; GRANT ALL ON DATABASE bchat TO bchat_user;"
```

---

## Step 4: Verify Vector Support

```bash
# Check CRDB version (must be v25.2+ for vector indexes)
docker exec -it bchat-crdb cockroach sql --insecure --host=localhost \
  -e "SELECT version();"

# Test vector type support
docker exec -it bchat-crdb cockroach sql --insecure --host=localhost \
  -e "CREATE DATABASE IF NOT EXISTS test; USE test;
      CREATE TABLE vec_test (id INT PRIMARY KEY, embedding VECTOR(3));
      INSERT INTO vec_test VALUES (1, '[1.0, 0.0, 0.0]'), (2, '[0.0, 1.0, 0.0]');
      SELECT id, embedding, 1 - (embedding <=> '[1.0, 0.0, 0.0]'::VECTOR) AS similarity
      FROM vec_test ORDER BY embedding <=> '[1.0, 0.0, 0.0]'::VECTOR;
      DROP TABLE vec_test; DROP DATABASE test;"
```

---

## Step 5: Configure bchat

### Option A: Export env vars directly

```bash
export COCKROACH_DSN="postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable"
export VECTOR_DB_PROVIDER=cockroach
export RAG_PIPELINE_ENABLED=true
export EMBEDDING_PROVIDER=mock
export TICKET_EMBEDDING_ENABLED=true
```

### Option B: Use docker-compose (recommended)

Create `scripts/docker-compose.cockroach.yml`:

```yaml
services:
  cockroach:
    image: cockroachdb/cockroach:v25.2.21
    container_name: bchat-crdb
    restart: unless-stopped
    command: start-single-node --insecure --advertise-addr=localhost
    ports:
      - "26257:26257"
      - "8080:8080"
    volumes:
      - bchat_crdb_data:/cockroach/cockroach-data
    healthcheck:
      test: ["CMD", "cockroach", "node", "status", "--insecure", "--host=localhost", "--port=26257"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  bchat_crdb_data:
```

```bash
docker compose -f scripts/docker-compose.cockroach.yml up -d

# Enable vector indexes (run once after cluster start)
docker exec -it bchat-crdb cockroach sql --insecure --host=localhost \
  -e "SET CLUSTER SETTING feature.vector_index.enabled = true;"

# Create database + user (run once)
docker exec -it bchat-crdb cockroach sql --insecure --host=localhost \
  -e "CREATE DATABASE bchat; CREATE USER bchat_user WITH PASSWORD 'bchat_pass'; GRANT ALL ON DATABASE bchat TO bchat_user;"

# Run bchat
export COCKROACH_DSN="postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable"
export EMBEDDING_PROVIDER=mock
task run:cockroach
```

---

## Step 6: Build and Run

```bash
task build:backend:cockroach

COCKROACH_DSN="postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable" \
VECTOR_DB_PROVIDER=cockroach \
RAG_PIPELINE_ENABLED=true \
EMBEDDING_PROVIDER=mock \
TICKET_EMBEDDING_ENABLED=true \
./build/memos --mode dev --data build/data
```

---

## Step 7: Verify Schema Was Created

```bash
docker exec -it bchat-crdb cockroach sql --insecure --host=localhost -d bchat \
  -e "SHOW TABLES; SHOW CREATE TABLE agent_vectors;"

docker exec -it bchat-crdb cockroach sql --insecure --host=localhost -d bchat \
  -e "SHOW INDEXES FROM agent_vectors;"
```

---

## Step 7.5: Create Table Manually (If Not Present)

The `agent_vectors` table is NOT auto-created on startup. `Validate()` is only called during reindex (`service.go:1244`), not during `NewService()` or `NewCockroachVectorDB()`. The auto-bootstrap path (`service.go:240-242`) calls `Stats()` which fails with "relation does not exist" and silently skips reindex.

**Option A: Create table manually (recommended for first run)**

```bash
docker exec bchat-crdb cockroach sql --insecure --host=localhost -d bchat \
  -e "CREATE TABLE IF NOT EXISTS agent_vectors (
    id STRING PRIMARY KEY,
    tenant_id INT NOT NULL,
    content_type STRING NOT NULL,
    title STRING,
    content TEXT NOT NULL,
    embedding VECTOR(1536),
    metadata JSONB,
    source_version INT DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT now()
  );"

# Note: vector_ip_ops is NOT supported in CRDB v25.2.21 (returns 0A000).
# The code at vectordb_cockroach.go:112-113 will fail on this, but the error
# is caught gracefully (42P07 fallback). Use default (no opclass) instead:
docker exec bchat-crdb cockroach sql --insecure --host=localhost -d bchat \
  -e "CREATE VECTOR INDEX idx_agent_vectors_embedding ON agent_vectors (embedding);"
```

**Note:** `CREATE VECTOR INDEX` does NOT support `IF NOT EXISTS` (verified `01-schema-design.md:152`). The code catches 42P07 as fallback (`vectordb_cockroach.go:111-127`). Also, `vector_ip_ops` is not implemented in CRDB v25.2.21 — the default `vector_l2_ops` is used instead. This means the code's opclass at `vectordb_cockroach.go:113` will fail, but search still works via brute-force fallback.

**Option B: Trigger reindex via API (requires running bchat + auth cookie)**

```bash
curl -X POST http://localhost:8081/api/v1/agent/hackathon-demo/reindex \
  -H "Cookie: memos.access-token=YOUR_ADMIN_TOKEN"
```

**Note:** If vector DB remains empty after startup, check logs for auto-bootstrap messages. The `Stats()` error is silently swallowed.

---

## Step 8: Run Integration Tests

```bash
# Unit tests — NOTE: tests currently skip CRDB-dependent cases (t.Skip)
# Manual verification required for full integration coverage
go test -tags cockroach ./server/router/api/v1/agent/... -v

# Seed demo data (optional)
# Ensure COCKROACH_DSN is set (from Step 5) or available in .env
COCKROACH_DSN="postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable" \
task run:cockroach:seed
```

---

## Step 9: Manual Search Test

```bash
# Insert test vector via SQL
docker exec -it bchat-crdb cockroach sql --insecure --host=localhost -d bchat \
  -e "INSERT INTO agent_vectors (id, tenant_id, title, content, content_type, embedding)
      VALUES ('test-001', 1, 'Water Extraction', '24/7 emergency water extraction services', 'service',
              '[0.1, 0.2, 0.3]');"

# Query via bchat API (requires admin session cookie)
curl -X POST "http://localhost:8081/api/v1/agent/hackathon-demo/rag/search" \
  -H "Content-Type: application/json" \
  -H "Cookie: memos.access-token=YOUR_ADMIN_TOKEN" \
  -d '{"query": "water damage", "tenant_id": 1, "limit": 5}'
```

---

## Step 10: Tear Down

```bash
# Stop and remove container (data persists in volume)
docker compose -f scripts/docker-compose.cockroach.yml down

# Stop and destroy all data
docker compose -f scripts/docker-compose.cockroach.yml down -v

# Or manually
docker stop bchat-crdb && docker rm bchat-crdb
docker volume rm bchat_crdb_data
```

---

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| `connection refused` | CRDB not started | `docker ps` — check container is running |
| `relation "agent_vectors" does not exist` | Table not auto-created | Run Step 7.5 (create table manually) or trigger reindex |
| `vector index not supported` | `feature.vector_index.enabled` not set | Run Step 2.5: `SET CLUSTER SETTING feature.vector_index.enabled = true;` |
| `CREATE VECTOR INDEX` silently degrades | Feature flag missing | Code catches 42P07 at `vectordb_cockroach.go:111-127` — enables brute-force fallback |
| `operator class vector_ip_ops is not supported` | CRDB v25.2.21 doesn't implement this opclass | Use default (no opclass): `CREATE VECTOR INDEX idx ON table (col);` — code's opclass at `vectordb_cockroach.go:113` will fail, but search works via brute-force |
| `prepared statement protocol` | pgx issue | DSN auto-appends `default_query_exec_mode=simple_protocol` |
| `sslmode must be disable` | TLS mismatch | Add `?sslmode=disable` to DSN |
| Port conflict | 26257 or 8080 in use | `lsof -i :26257` — kill conflicting process |
| Empty vector DB after startup | `Stats()` silently skips reindex | Check startup logs; run Step 7.5 or trigger reindex manually |

---

## Key Differences from Real CRDB Cluster

| Aspect | Local Docker | Real Cluster |
|--------|-------------|--------------|
| TLS | Disabled (`--insecure`) | Required (`sslmode=verify-full`) |
| Replication | None (single node) | 3x default |
| Resilience | No failover | Automatic range rebalancing |
| Vector index | Same syntax (requires feature flag) | Same syntax |
| Performance | Local only | Network latency |

**Note:** The `--insecure` flag is safe for local development. The real cluster will use TLS + auth, which is a DSN change only.

---

## Next Steps After Local Testing

1. ✅ Local CRDB with Docker works
2. ✅ Vector indexes enabled and verified
3. ✅ Schema created and verified
4. Run `task run:cockroach:seed` with demo data
5. Test full chat flow against local CRDB
6. Provision real CRDB cluster (`deploy/ccloud/setup.sh`)
7. Switch DSN to real cluster
8. Verify vector search works end-to-end

---

## Adversarial Plan Review Prompt

Review `plan4_testing.md` for correctness, completeness, and risks. Check:

1. **Schema DDL:** Is the `CREATE TABLE IF NOT EXISTS agent_vectors` DDL correct? Verify column types match `vectordb_cockroach.go` (especially `VECTOR(1536)` dimension and `JSONB` metadata).
2. **Vector Index DDL:** Is `CREATE VECTOR INDEX idx_agent_vectors_embedding ON agent_vectors (embedding)` correct syntax for CRDB v25.2? The `vector_ip_ops` opclass is NOT supported in v25.2.21 (returns 0A000). Verify against `vectordb_cockroach.go:112-113`.
3. **DSN Format:** Is `postgresql://user:pass@host:26257/db?sslmode=disable` correct for pgx v5 + CRDB insecure mode?
4. **Auth:** Does the RAG search endpoint require admin role? Verify `AuthMiddleware` at `v1.go:382,417`.
5. **Tenant Slug:** Does `hackathon-demo` match what the seed script creates? Verify `cmd/seed/seed_demo_tickets.go`.
6. **Feature Flag Persistence:** Does `SET CLUSTER SETTING` persist across container restarts (via volume)?
7. **Missing Steps:** Any prerequisites or steps missing?
8. **Security:** Is `--insecure` safe for local dev? Any Docker networking gotchas?
9. **Data Persistence:** Does `-v bchat_crdb_data:/cockroach/cockroach-data` persist data across `docker compose down` (without `-v`)?
10. **Alternative:** Should we use the CRDB binary directly (as in `setting-up-local-cluster/SKILL.md`) instead of Docker?

Return findings as a table with columns: #, Category, Severity (CRITICAL/HIGH/MEDIUM/NIT), Finding, Fix.
