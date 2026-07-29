# Plan: Local CockroachDB Testing with Docker (Revised)

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
docker pull cockroachdb/cockroach:latest
# Or pin: docker pull cockroachdb/cockroach:v25.2.21
```

---

## Step 2: Start Single-Node Cluster

```bash
docker run -d \
  --name bchat-crdb \
  -p 26257:26257 \
  -p 8080:8080 \
  -v bchat_crdb_data:/cockroach/cockroach-data \
  cockroachdb/cockroach:latest \
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

## Step 8: Run Integration Tests

```bash
# Unit tests — NOTE: tests currently skip CRDB-dependent cases (t.Skip)
# Manual verification required for full integration coverage
go test -tags cockroach ./server/router/api/v1/agent/... -v

# Seed demo data (optional)
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

# Query via bchat API
curl -X POST "http://localhost:8081/api/v1/agent/acme-corp/rag/search" \
  -H "Content-Type: application/json" \
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
| `relation "agent_vectors" does not exist` | Schema not auto-created | Run bchat once (it creates schema on startup via `Validate()`) |
| `vector index not supported` | `feature.vector_index.enabled` not set | Run Step 2.5: `SET CLUSTER SETTING feature.vector_index.enabled = true;` |
| `CREATE VECTOR INDEX` silently degrades | Feature flag missing | Code catches 42P07 at `vectordb_cockroach.go:111-127` — enables brute-force fallback |
| `prepared statement protocol` | pgx issue | DSN auto-appends `default_query_exec_mode=simple_protocol` |
| `sslmode must be disable` | TLS mismatch | Add `?sslmode=disable` to DSN |
| Port conflict | 26257 or 8080 in use | `lsof -i :26257` — kill conflicting process |

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
3. Run `task run:cockroach:seed` with demo data
4. Test full chat flow against local CRDB
5. Provision real CRDB cluster (`deploy/ccloud/setup.sh`)
6. Switch DSN to real cluster
7. Verify vector search works end-to-end

---

## Adversarial Plan Review Prompt

Review `plan2_testing.md` for correctness, completeness, and risks. Check:

1. **Feature Flag:** Is `SET CLUSTER SETTING feature.vector_index.enabled = true` the correct syntax? Does it require a restart? Is it persistent across container restarts?
2. **Docker Image:** Is `cockroachdb/cockroach:v25.2.21` a valid tag? Check Docker Hub.
3. **DSN Format:** Is `postgresql://user:pass@host:26257/db?sslmode=disable` correct for pgx v5 + CRDB insecure mode?
4. **Schema Auto-Creation:** Does bchat auto-create `agent_vectors` table on startup? Check `vectordb_cockroach.go` for `CREATE TABLE IF NOT EXISTS`.
5. **Healthcheck:** Is `cockroach node status --insecure --host=localhost --port=26257` valid inside the container?
6. **Ordering:** Should vector index be enabled before or after DB/user creation?
7. **Missing Steps:** Any prerequisites or steps missing?
8. **Security:** Is `--insecure` safe for local dev? Any Docker networking gotchas?
9. **Data Persistence:** Does `-v bchat_crdb_data:/cockroach/cockroach-data` persist data across container restarts? Does it survive `docker compose down` (without `-v`)?
10. **Alternative:** Should we use the CRDB binary directly (as in `setting-up-local-cluster/SKILL.md`) instead of Docker?

Return findings as a table with columns: #, Category, Severity (CRITICAL/HIGH/MEDIUM/NIT), Finding, Fix.
