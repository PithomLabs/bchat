# Plan Review: Port Postgres Database Migrations to CockroachDB

**Bug ID:** 057
**Review Date:** 2026-08-02
**Reviewer:** Kilo (Senior Go & CockroachDB Architect)
**Verdict:** BLOCKED — Critical issues must be resolved before implementation begins.

---

## 1. Critical Findings (Must Fix Before Implementation)

### C-1: Postgres-Specific Cast Syntax in Go Store Code

**Severity:** CRITICAL
**Location:** `store/db/postgres/agent.go` lines 2605, 2693, 2718, 2776, 2780

The plan states that store methods are "SQL-agnostic" and can be copied verbatim. This is **false**. The `agent.go` file hardcodes Postgres-specific cast syntax:

```go
// line 2605
VALUES ($1, $2, $3, $4, $5, EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT)

// line 2776
SET status = 'processing', claimed_at = EXTRACT(EPOCH FROM NOW())::BIGINT, attempts = attempts + 1
```

**Problem:** CockroachDB does not support `::BIGINT` cast syntax. It uses `::INT` or `CAST(... AS INT)`. Additionally, `EXTRACT(EPOCH FROM NOW())` returns `double precision` in CRDB, not `bigint`.

**Impact:** Runtime SQL errors on every insert/update in `agent.go` and any other file using this pattern.

**Recommended Fix:**
- Search all postgres store files for `::BIGINT`, `::INT`, `::TEXT`, `::BOOLEAN` casts
- Replace with CRDB-compatible syntax or remove unnecessary casts
- For `EXTRACT(EPOCH FROM NOW())` results, use `CAST(EXTRACT(EPOCH FROM NOW()) AS INT)` or rely on implicit cast if column type is INT
- Add a grep-based audit step: `grep -rn '::[A-Z]' store/db/postgres/`

---

### C-2: Missing Files in Copy List

**Severity:** CRITICAL
**Location:** Plan Section 1.1 (table)

The plan states "Copy all `.go` files from `store/db/postgres/` to `store/db/cockroach/`" but the table only lists 21 files. The actual `postgres/` directory contains 25 files (excluding `postgres.go` and test files):

| Missing File | Purpose |
|--------------|---------|
| `bridge_auth.go` | Bridge authentication store methods |
| `memo_relation.go` | Memo relationship store methods |

**Impact:** Compile errors when building the cockroach driver. The `memo_relation.go` file contains `RETURNING` clauses that are used by the agent system.

**Recommended Fix:**
- Update the file table to include all 23 postgres files (excluding `postgres.go` and `memo_filter_test.go`)
- Explicitly state: `memo_filter_test.go` is excluded (test file)

---

### C-3: Code Duplication Without Abstraction

**Severity:** CRITICAL
**Location:** Plan Section 1.4

The plan copies ~22 files from `postgres/` to `cockroach/` with only "package name change" and minor edits. This creates a **maintenance fork**:

- Every bug fix in `postgres/` must be manually duplicated in `cockroach/`
- Every new feature must be implemented twice
- Schema changes in LATEST.sql require updates in two places
- Risk of drift between postgres and cockroach implementations grows over time

**Impact:** Long-term maintainability. Within 2-3 release cycles, the two drivers will diverge, causing subtle bugs.

**Recommended Fix:**
- **Option A (Preferred):** Create a shared `store/db/common/` package with all SQL-agnostic logic. Both `postgres` and `cockroach` packages import and wrap it, providing only driver-specific connection setup and resilience.
- **Option B:** Use Go build tags within a single `store/db/postgres/` package, similar to the `vectordb_cockroach.go` / `vectordb_nocockroach.go` pattern.
- **Option C (Minimum):** Add a CI check that diffs the two packages and fails if they diverge by more than the expected resilience differences.

---

## 2. High Findings (Must Fix Before Implementation)

### H-1: Connection Pool Settings Exceed CRDB Cloud Recommendations

**Severity:** HIGH
**Location:** Plan Section 1.1 (`cockroach.go`)

The plan copies `SetMaxOpenConns(10)` from the Postgres driver. CRDB best practices state:

> active connections ≤ 4x vCPU count

For a Fly.io shared-cpu-1x instance (1 vCPU), the maximum recommended connections is **4**. Setting `MaxOpenConns=10` will cause connection exhaustion, queueing, and degraded performance.

**Recommended Fix:**
```go
// Make pool settings configurable based on CRDB Cloud node size
maxOpenConns := 4 // Default for 1 vCPU; scale with CRDB node size
if cpuCount := runtime.NumCPU(); cpuCount > 1 {
    maxOpenConns = cpuCount * 4
}
db.SetMaxOpenConns(maxOpenConns)
```

Or read from env var `CRDB_MAX_OPEN_CONNS` with sensible default.

---

### H-2: `COCKROACH_DSN` / `DATABASE_URL` Fallback Is Dangerous

**Severity:** HIGH
**Location:** Plan Section 1.3 (`internal/profile/profile.go`)

```go
if p.Driver == "cockroach" && p.DSN == "" {
    p.DSN = os.Getenv("COCKROACH_DSN")
    if p.DSN == "" {
        p.DSN = os.Getenv("DATABASE_URL")  // fallback for consistency
    }
}
```

If an operator has `DATABASE_URL` set for Postgres (e.g., from a previous deployment) and accidentally sets `MEMOS_DRIVER=cockroach`, the app will connect to the Postgres database using the cockroach driver. This could cause silent data corruption or schema conflicts.

**Recommended Fix:**
- Remove the `DATABASE_URL` fallback for the `cockroach` driver
- Require explicit `COCKROACH_DSN` to prevent accidental misconfiguration
- Log a clear error if `COCKROACH_DSN` is not set

---

### H-3: `sslmode=require` Is Insufficient for Production

**Severity:** HIGH
**Location:** Plan Section 4.4 (`scripts/fly-crdb-secrets.sh`)

The plan uses `sslmode=require`, which encrypts traffic but does **not** verify the server's certificate. This is vulnerable to man-in-the-middle attacks.

CRDB best practices state:
> TLS certificates required for all production connections

**Recommended Fix:**
```bash
# For CRDB Cloud, download the CA certificate
cockroach cert create-ca --certs-dir=/certs --ca-key=ca.key

# Or use sslmode=verify-full with sslrootcert
COCKROACH_DSN="postgresql://user:password@host:26257/db?sslmode=verify-full&sslrootcert=/path/to/ca.crt"
```

For Fly.io deployment, store the CA cert as a secret or volume mount.

---

### H-4: Race Condition in `agent_vectors` Table Creation

**Severity:** HIGH
**Location:** Plan Section 1.1 (`vectordb_cockroach.go` referenced)

The plan states `agent_vectors` is created at runtime by `CockroachVectorDB.Validate()`. If multiple app instances start simultaneously (e.g., during Fly.io rollout), they will all attempt `CREATE TABLE IF NOT EXISTS agent_vectors`. While `IF NOT EXISTS` prevents errors, concurrent `CREATE INDEX` statements could cause lock contention or schema validation failures.

**Recommended Fix:**
- Wrap table creation in a `crdb.ExecuteTx` with retry logic
- Or add a distributed lock using CRDB's `SELECT ... FOR UPDATE`
- Or create `agent_vectors` in `LATEST.sql` instead of at runtime

---

## 3. Medium Findings (Should Fix Before or During Implementation)

### M-1: `EXTRACT(EPOCH FROM NOW())` Type Mismatch in LATEST.sql

**Severity:** MEDIUM
**Location:** Plan Section 2.2

The plan uses `extract(epoch from now())` for `INT` columns. In CRDB, `extract(epoch from now())` returns `double precision`. Inserting a `double precision` into an `INT` column may fail or cause implicit truncation.

**Recommended Fix:**
```sql
-- Explicitly cast to INT
created_ts INT NOT NULL DEFAULT CAST(EXTRACT(EPOCH FROM NOW()) AS INT)
```

---

### M-2: `unique_rowid()` Write Hotspots Not Addressed

**Severity:** MEDIUM
**Location:** Plan Section 2.2

The plan replaces `SERIAL PRIMARY KEY` with `INT DEFAULT unique_rowid() PRIMARY KEY`. While `unique_rowid()` avoids sequence contention, it generates timestamp-based IDs that cause write hotspots on the same range for high-throughput tables.

**Impact:** Tables with high insert rates (e.g., `agent_messages`, `agent_events`) may experience write contention.

**Recommended Fix:**
- Use `INT DEFAULT unique_rowid()` for most tables
- For very high-throughput tables, consider `UUID` primary keys (supported by CRDB)
- Document the trade-off in the plan
- Consider `ALTER TABLE ... CONFIGURE ZONE` with specific replica placement for hotspot tables

---

### M-3: No Rollback Plan

**Severity:** MEDIUM
**Location:** Plan Section 4 (entire)

The plan has no rollback strategy if CockroachDB deployment fails after the Postgres deployment is working.

**Recommended Fix:**
- Add a rollback section:
  - How to revert to Postgres if CRDB Cloud is unreachable
  - Fly.io `fly deploy -c fly_pg.toml` as rollback command
  - Data migration strategy if data exists in CRDB
- Add a canary deployment strategy: deploy to staging first, validate, then promote to production

---

### M-4: Placeholder Values in Fly Secrets Script

**Severity:** MEDIUM
**Location:** Plan Section 4.4

```bash
fly -a "$APP_NAME" secrets set \
  COCKROACH_DSN="postgresql://user:password@your-cluster.cockroachlabs.cloud:26257/bchat?sslmode=require"
```

The script contains hardcoded placeholders (`your-cluster.cockroachlabs.cloud`, `your-bucket-name`, `xxx`). An operator running this script will either:
1. Accidentally deploy with placeholder values
2. Manually edit the script, risking typos

**Recommended Fix:**
```bash
#!/bin/bash
set -euo pipefail

APP_NAME="${1:-bchat-crdb}"
COCKROACH_DSN="${2:?Usage: $0 <app-name> <cockroach-dsn>}"
OPENROUTER_API_KEY="${3:?Usage: $0 <app-name> <cockroach-dsn> <openrouter-key>}"
# ... etc

echo "Setting secrets for $APP_NAME..."
fly -a "$APP_NAME" secrets set COCKROACH_DSN="$COCKROACH_DSN"
```

---

### M-5: `default_query_exec_mode` Removal May Break CRDB Cloud

**Severity:** MEDIUM
**Location:** Plan Section 1.1

The plan removes `default_query_exec_mode=simple_protocol` because "not needed for CockroachDB". This was added for Neon PgBouncer compatibility. CRDB Cloud may use a similar connection pooler or proxy.

**Recommended Fix:**
- Test with CRDB Cloud connection string to verify `simple_protocol` is not needed
- If needed, add it conditionally:
  ```go
  if !strings.Contains(dsn, "default_query_exec_mode") {
      dsn += "?default_query_exec_mode=simple_protocol"
  }
  ```
- Document the decision in the plan

---

### M-6: RAG Reindex Settings Conflict

**Severity:** MEDIUM
**Location:** Plan Section 4.2

```toml
RAG_PIPELINE_ENABLED = 'true'
FORCE_REINDEX_ON_STARTUP = 'false'
RAG_STARTUP_REINDEX_DISABLED = 'true'
```

This means:
1. RAG is enabled
2. But startup reindex is disabled
3. And force reindex is disabled

**Result:** On first deployment, the LanceDB index will be empty. The RAG pipeline will run but return no results, causing silent quality degradation.

**Recommended Fix:**
- Set `FORCE_REINDEX_ON_STARTUP=true` for the first deployment only
- Or add a one-time `/api/v1/agent/:slug/reindex` call in the deployment script
- Or set `RAG_STARTUP_REINDEX_DISABLED=false`

---

### M-7: No Incremental Migration Strategy

**Severity:** MEDIUM
**Location:** Plan Section 2

The plan only mentions `LATEST.sql`. It does not address:
- How incremental migrations will be maintained for CRDB
- Whether existing Postgres incremental migrations are compatible with CRDB
- What happens if a future migration uses Postgres-specific syntax

**Recommended Fix:**
- Add a maintenance section: "Future migrations must be tested against both Postgres and CockroachDB"
- Add a CI job that runs migrations against both databases
- Consider a shared migration strategy where possible

---

## 4. Low Findings (Nits — Fix During Implementation)

### L-1: Assumed Taskfile Targets

**Severity:** LOW
**Location:** Plan Section 3.2, 3.5, 7

The plan references `task build:backend:cockroach`, `task run:cockroach`, and `task crdb:test` without verifying they exist or defining them.

**Recommended Fix:**
- Verify these targets exist in `Taskfile.yml`
- If missing, add them:
  ```yaml
  build:backend:cockroach:
      cmds:
          - go build -tags "cockroach" -ldflags="-s -w" -o {{.BUILD_DIR}}/memos ./bin/memos
  run:cockroach:
      cmds:
          - COCKROACH_DSN="postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable" \
            MEMOS_DRIVER=cockroach MEMOS_MODE=dev RAG_PIPELINE_ENABLED=true VECTOR_DB_PROVIDER=cockroach \
            {{.BUILD_DIR}}/memos
  ```

---

### L-2: Placeholder `LLM_MODEL` Values in Production Config

**Severity:** LOW
**Location:** Plan Section 4.2

```toml
LLM_MODEL = "openrouter/free"
LLM_MODEL_REASONING = "openrouter/free"
```

These are placeholder values that will produce broken LLM responses in production.

**Recommended Fix:**
- Add a comment: `# Set via fly secrets or replace with production model`
- Or parameterize: `LLM_MODEL = "${LLM_MODEL}"`

---

### L-3: In-Memory CockroachDB Store Loses Data

**Severity:** LOW
**Location:** Plan Section 3.1

```yaml
# docker-compose.cockroach.yml
environment:
  - cockroach start-single-node --insecure --store=type=mem,size=0.25
```

The `type=mem` store loses all data when the container restarts. This is fine for ephemeral testing but confusing for developers who restart Docker and lose their data.

**Recommended Fix:**
- Use a file-backed store for local development:
  ```yaml
  - cockroach start-single-node --insecure --store=type=fs,path=/var/lib/cockroach
  ```
- Or document clearly that data is ephemeral

---

### L-4: `ALTER RANGE default CONFIGURE ZONE` Affects All Tables

**Severity:** LOW
**Location:** Plan Section 3.4

```sql
ALTER RANGE default CONFIGURE ZONE USING gc.ttlseconds = 600;
```

This changes the default zone configuration for **all** ranges (tables) in the cluster. This could have unintended side effects on system tables or future tables.

**Recommended Fix:**
- Apply zone configuration to specific tables instead:
  ```sql
  ALTER TABLE memo CONFIGURE ZONE USING gc.ttlseconds = 600;
  ALTER TABLE agent_messages CONFIGURE ZONE USING gc.ttlseconds = 600;
  ```
- Or document that this is for testing only and should not be used in production

---

### L-5: `auto_stop_machines='stop'` with `min_machines_running=0`

**Severity:** LOW
**Location:** Plan Section 4.2

```toml
auto_stop_machines = 'stop'
min_machines_running = 0
```

This allows the app to scale to zero. While Fly.io will wake it on request, cold starts could cause:
- Health check failures during wake-up
- Timeout errors for the first request after idle period

**Recommended Fix:**
- Consider `min_machines_running = 1` for production
- Or increase `request_timeout` to account for cold starts
- Or document the cold-start behavior

---

### L-6: Build Tag Strategy Not Explicitly Documented

**Severity:** LOW
**Location:** Plan Section 1.1

The plan does not explicitly state whether the `store/db/cockroach/` package uses `//go:build cockroach` tags. The existing `vectordb_cockroach.go` uses build tags, but the driver package does not.

**Recommended Fix:**
- Explicitly state: "The cockroach driver package does NOT use build tags. It is always compiled."
- Explain why: The driver is selected at runtime via `MEMOS_DRIVER` env var, not at build time.

---

## 5. Missing Information

### MI-1: Postgres-Specific SQL Idioms in Store Methods

**Severity:** CRITICAL (Risk)
**Location:** Plan Section 1.4

The plan claims store methods are "SQL-agnostic" but does not verify this. The grep audit found:

| Pattern | Count | CRDB Support |
|---------|-------|--------------|
| `::BIGINT` cast | 5 | NOT supported |
| `::boolean` cast | 3 | Supported |
| `ILIKE` | 1 | Supported |
| `CASE WHEN` | 3 | Supported |
| `RETURNING` | 30+ | Supported |
| `jsonb_build_array` | 2 | Supported |
| `jsonb->'path'` | 5+ | Supported |
| `jsonb->>'path'` | 3+ | Supported |
| `@>` containment | 1 | Supported |

**Recommended Fix:**
- Add a mandatory audit step before copying files:
  ```bash
  grep -rn '::BIGINT\|::INT\|::TEXT\|::UUID\|::JSONB\|::BYTEA\|::TIMESTAMPTZ' store/db/postgres/
  ```
- Document all Postgres-specific idioms found and their CRDB equivalents
- Test each query pattern against local CRDB instance

---

### MI-2: Incremental Migration Compatibility

**Severity:** HIGH (Risk)
**Location:** Plan Section 2

The plan does not address whether existing Postgres incremental migrations (in `store/migration/postgres/0.xx/`) are compatible with CockroachDB.

**Recommended Fix:**
- Audit all existing incremental migrations for Postgres-specific syntax:
  - `SERIAL` → `unique_rowid()`
  - `::BIGINT` casts
  - `CREATE EXTENSION`
  - `CREATE INDEX CONCURRENTLY`
  - `UNLOGGED` tables
- Create a CRDB-specific incremental migration directory if any migrations need changes
- Or document that incremental migrations must be CRDB-compatible going forward

---

## 6. Recommended Implementation Order (Revised)

| Step | Task | Estimated Effort | Dependencies |
|------|------|-----------------|--------------|
| 0 | **Audit Postgres store files for Postgres-specific SQL** | 1 hr | None |
| 0.5 | **Fix `::BIGINT` and other Postgres-specific casts in Go code** | 1-2 hrs | Step 0 |
| 1 | Create `store/db/cockroach/cockroach.go` (driver init) | 30 min | None |
| 2 | Create `store/db/cockroach/resilience.go` (retry logic) | 30 min | None |
| 3 | Copy and adapt all 23 postgres store files to cockroach | 2-3 hrs | Steps 0.5, 1, 2 |
| 4 | Modify `store/db/db.go` to add cockroach case | 5 min | Step 1 |
| 5 | Modify `internal/profile/profile.go` for COCKROACH_DSN (no DATABASE_URL fallback) | 10 min | None |
| 6 | Create `store/migration/cockroach/LATEST.sql` (with explicit casts) | 2-3 hrs | Step 0 |
| 7 | Fix `agent_vectors` race condition (add retry or move to LATEST.sql) | 30 min | None |
| 8 | Test local: `docker compose up` + `task run:cockroach` | 1-2 hrs | Steps 1-7 |
| 9 | Create `fly_crdb.toml` (with sslmode=verify-full) | 30 min | Steps 1-7 |
| 10 | Create `Dockerfile.crdb.fly` | 30 min | Steps 1-7 |
| 11 | Create `scripts/fly-crdb-secrets.sh` (parameterized) | 15 min | Step 9 |
| 12 | Adjust connection pool settings for CRDB Cloud | 15 min | None |
| 13 | Fix RAG reindex settings for first deployment | 10 min | None |
| 14 | Test Fly.io deployment | 1-2 hrs | Steps 8-13 |

**Revised total estimated effort:** 10-14 hours (up from 8-12 due to additional audit and fix steps)

---

## 7. Blocking Summary

The plan is **BLOCKED** until the following critical issues are resolved:

1. **C-1:** Audit and fix `::BIGINT` and other Postgres-specific casts in Go store code
2. **C-2:** Add missing `bridge_auth.go` and `memo_relation.go` to the file list
3. **C-3:** Replace copy-paste strategy with shared abstraction or document drift mitigation

Once these are fixed, the plan is implementation-ready with the following high-priority follow-ups:

4. **H-1:** Adjust connection pool settings for CRDB Cloud
5. **H-2:** Remove `DATABASE_URL` fallback for cockroach driver
6. **H-3:** Use `sslmode=verify-full` for production
7. **H-4:** Fix `agent_vectors` race condition

---

## 8. Approval Criteria

The plan can be marked **APPROVED** when:

- [ ] All files in `store/db/postgres/` (excluding `postgres.go` and test files) are listed in the copy table
- [ ] A SQL audit of all store methods confirms no Postgres-specific idioms remain
- [ ] Connection pool settings are configurable and appropriate for CRDB Cloud
- [ ] `COCKROACH_DSN` is required with no `DATABASE_URL` fallback
- [ ] `sslmode=verify-full` is used in production deployment
- [ ] `agent_vectors` creation is race-safe
- [ ] RAG reindex settings are correct for first deployment

---

*Review complete. Awaiting fixes to critical findings before implementation can proceed.*
