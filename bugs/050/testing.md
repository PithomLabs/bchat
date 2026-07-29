# Testing Implementation: Local CockroachDB with Docker

## What Was Done

Set up a local CockroachDB instance via Docker to test the vector store integration before provisioning a real CRDB cluster.

---

## Files Created

| File | Purpose |
|------|---------|
| `scripts/docker-compose.cockroach.yml` | Docker Compose for CRDB v25.2.21 single-node, insecure, persistent volume |

## Files Modified

| File | Change |
|------|--------|
| `bugs/050/plan4_testing.md` | Fixed DDL: `vector_ip_ops` → default opclass; added troubleshooting row; updated review prompt |

---

## Container Status

```bash
$ docker ps --filter name=bchat-crdb
NAMES        STATUS
bchat-crdb   Up (healthy)
```

- **Image:** `cockroachdb/cockroach:v25.2.21`
- **Ports:** 26257 (SQL), 8080 (DB Console)
- **Volume:** `scripts_bchat_crdb_data` (persistent across restarts)
- **Mode:** Insecure (no TLS, no password auth)

---

## What Was Verified

| Check | Result |
|-------|--------|
| CRDB version | `CockroachDB CCL v25.2.21` ✅ |
| Feature flag | `SET CLUSTER SETTING feature.vector_index.enabled = true` ✅ |
| Database created | `bchat` ✅ |
| User created | `bchat_user` (no password — insecure mode) ✅ |
| Vector type support | `VECTOR(3)`, cosine distance `<=>` operator ✅ |
| Table created | `agent_vectors` with correct schema ✅ |
| Tenant index | `idx_agent_vectors_tenant` ✅ |
| Vector index | `idx_agent_vectors_embedding` (default `vector_l2_ops`) ✅ * |

*\* Created via workaround (opclass omitted). Code's `vector_ip_ops` fails — see Key Finding below.*

---

## Key Finding: `vector_ip_ops` Not Supported

**Severity:** HIGH (blocks code's vector index creation)

**Problem:** The code at `vectordb_cockroach.go:112-113` creates a vector index with `vector_ip_ops`:
```sql
CREATE VECTOR INDEX idx_agent_vectors_embedding
ON agent_vectors (embedding vector_ip_ops)
```

CRDB v25.2.21 returns:
```
ERROR: unimplemented: operator class vector_ip_ops is not supported
SQLSTATE: 0A000
HINT: You have attempted to use a feature that is not yet implemented.
See: https://go.crdb.dev/issue-v/144016/v25.2
```

**Workaround:** Use default opclass (no explicit `vector_ip_ops`):
```sql
CREATE VECTOR INDEX idx_agent_vectors_embedding ON agent_vectors (embedding);
```
This creates the index with `vector_l2_ops` (L2/Euclidean distance).

**Impact:**
- The code's `Validate()` at `vectordb_cockroach.go:111-127` will fail when creating the vector index
- The error is caught gracefully (42P07 fallback), but the log shows a warning
- Search still works via brute-force fallback (no vector index acceleration)
- The `<=>` operator works regardless of opclass — only the index optimization is affected

**Code Fix:** Removed `vector_ip_ops` opclass from DDL (now uses default `vector_l2_ops`). Added `0A000` error handler. See `testing3.md` section 1.

---

## Key Finding: Insecure Mode Password Auth

**Severity:** MEDIUM (blocks user creation with password)

**Problem:** `CREATE USER bchat_user WITH PASSWORD 'bchat_pass'` fails in insecure mode:
```
ERROR: setting or updating a password is not supported in insecure mode
SQLSTATE: 28P01
```

**Workaround:** Create user without password:
```sql
CREATE USER bchat_user;
```

**Impact:** DSN uses `bchat_user@localhost` (no password in URL). This is correct for insecure mode.

---

## DSN for Local Testing

```bash
export COCKROACH_DSN="postgresql://bchat_user@localhost:26257/bchat?sslmode=disable"
```

Note: No password in DSN (insecure mode doesn't support password auth).

---

## Commands to Reproduce

```bash
# Start CRDB
docker compose -f scripts/docker-compose.cockroach.yml up -d

# Enable vector indexes
docker exec bchat-crdb cockroach sql --insecure --host=localhost \
  -e "SET CLUSTER SETTING feature.vector_index.enabled = true;"

# Create database and user
docker exec bchat-crdb cockroach sql --insecure --host=localhost \
  -e "CREATE DATABASE bchat; CREATE USER bchat_user; GRANT ALL ON DATABASE bchat TO bchat_user;"

# Create table + indexes
docker exec bchat-crdb cockroach sql --insecure --host=localhost -d bchat -e "
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
  CREATE VECTOR INDEX idx_agent_vectors_embedding ON agent_vectors (embedding);
"

# Verify schema
docker exec bchat-crdb cockroach sql --insecure --host=localhost -d bchat \
  -e "SHOW TABLES; SHOW INDEXES FROM agent_vectors;"

# Stop CRDB
docker compose -f scripts/docker-compose.cockroach.yml down

# Stop and destroy data
docker compose -f scripts/docker-compose.cockroach.yml down -v
```

---

## v26.2 Verification

**Status:** ✅ Verified 2026-07-29

The skills repo (`01-schema-design.md:152`) shows `vector_ip_ops` in all examples — these are **NOT aspirational**. `vector_ip_ops` IS supported in CRDB v26.2.4.

| Version | `vector_ip_ops` Status | Source |
|---------|----------------------|--------|
| v25.2.21 | ❌ Not supported (0A000, issue #144016) | Tested locally |
| v26.2.4 | ✅ Supported | Tested locally 2026-07-29 |

**Test results (v26.2.4):**
- `CREATE VECTOR INDEX ... (embedding vector_ip_ops)` → ✅ Success
- `CREATE VECTOR INDEX ... (embedding)` (default) → ✅ Success
- Cosine distance `<=>` with `vector_ip_ops` index → ✅ Returns correct similarity scores

**Impact on code fix:** Our fix (removing opclass) is still correct:
- Backward compatible with v25.2.21 (which doesn't support `vector_ip_ops`)
- Works in v26.2.4 (defaults to `vector_l2_ops`)
- If targeting v26.2.4+ only, could restore `vector_ip_ops` for explicit opclass

---

## Reconciliation Table Update

After code fix (`vectordb_cockroach.go:113`) and v26.2.4 verification:

| Source | Statement | v25.2.21 | v26.2.4 |
|--------|-----------|----------|---------|
| Skills repo `01-schema-design.md:152` | `CREATE VECTOR INDEX ... (embedding vector_ip_ops)` | ❌ 0A000 | ✅ Works |
| Code before fix (`vectordb_cockroach.go:113`) | `CREATE VECTOR INDEX ... (embedding vector_ip_ops)` | ❌ 0A000 | ✅ Would work |
| Code after fix (`vectordb_cockroach.go:113`) | `CREATE VECTOR INDEX ... (embedding)` | ✅ Works | ✅ Works |
| testing4.md (verified workaround) | `CREATE VECTOR INDEX ... (embedding)` | ✅ Works | ✅ Works |

---

## Adversarial Testing Review Prompt

Review the testing implementation described in `testing.md` for correctness, completeness, and risks. Check:

1. **Container Configuration:** Is `cockroachdb/cockroach:v25.2.21` the correct image? Is `start-single-node --insecure --advertise-addr=localhost` the correct command for local Docker testing?

2. **Feature Flag:** Is `SET CLUSTER SETTING feature.vector_index.enabled = true` the correct syntax? Does it persist across container restarts (via volume)? Does it require a restart?

3. **Schema DDL:** Is the `CREATE TABLE IF NOT EXISTS agent_vectors` DDL correct? Verify column types match `vectordb_cockroach.go:84-95` (especially `VECTOR(1536)` dimension and `JSONB` metadata).

4. **Vector Index:** The `vector_ip_ops` opclass is not supported in CRDB v25.2.21. Is the default `vector_l2_ops` the correct fallback? Does the `<=>` operator work with `vector_l2_ops` for cosine distance queries?

5. **Insecure Mode:** Is `CREATE USER bchat_user` (without password) the correct approach for insecure mode? Is the DSN `postgresql://bchat_user@localhost:26257/bchat?sslmode=disable` correct?

6. **Healthcheck:** Is `cockroach node status --insecure --host=localhost --port=26257` the correct healthcheck command? Does it work inside the container?

7. **Data Persistence:** Does `-v bchat_crdb_data:/cockroach/cockroach-data` persist data across `docker compose down` (without `-v`)? Does it survive container restarts?

8. **Port Conflicts:** Are 26257 and 8080 the correct default ports for CRDB single-node? What happens if they're already in use?

9. **Missing Steps:** Are there any prerequisites or steps missing from the setup?

10. **Alternative:** Should we use the CRDB binary directly (as in `setting-up-local-cluster/SKILL.md`) instead of Docker?

11. **Code Impact:** The code at `vectordb_cockroach.go:112-113` uses `vector_ip_ops`. What happens when this code runs against a CRDB instance with `vector_l2_ops`? Does the error handling at lines 115-127 catch this gracefully?

12. **Security:** Is `--insecure` safe for local development? Any Docker networking gotchas?

Return findings as a table with columns: #, Category, Severity (CRITICAL/HIGH/MEDIUM/NIT), Finding, Fix.
