# Bug 058 — Implementation Documentation

**Date:** 2026-08-03
**Author:** opencode
**Status:** Implemented, pending adversarial review

---

## Implementation Summary

Three-file change enabling local CockroachDB E2E testing before cloud deployment.

| File | Change | Lines |
|------|--------|-------|
| `server/router/api/v1/agent/vectordb_cockroach.go` | Add `IF NOT EXISTS` to vector index DDL | 112 |
| `scripts/crdb-init.sql` | New file — cluster settings + retry loop | 38 lines |
| `Taskfile.yml` | New `crdb:init` target, update `crdb:reset`/`crdb:up`, enhance `crdb:verify` | ~50 lines changed |

---

## Code Changes

### 1. `server/router/api/v1/agent/vectordb_cockroach.go` (line 112)

**Before:**
```go
_, err = v.db.ExecContext(ctx, `
    CREATE VECTOR INDEX idx_agent_vectors_embedding
    ON agent_vectors (embedding)
`)
```

**After:**
```go
// 3. Vector index (CRDB-specific syntax — NOT pgvector USING hnsw)
// IF NOT EXISTS is supported for VECTOR INDEX in CRDB v26.1+ (docs confirmed).
// vector_ip_ops is NOT supported (CRDB issue #144016) — default to vector_l2_ops
// SQLSTATE fallback kept as defense-in-depth until concurrent startup is verified.
// TODO(post-hackathon): remove SQLSTATE fallback after concurrent startup exercised.
_, err = v.db.ExecContext(ctx, `
    CREATE VECTOR INDEX IF NOT EXISTS idx_agent_vectors_embedding
    ON agent_vectors (embedding)
`)
```

**Rationale:**
- `CREATE VECTOR INDEX IF NOT EXISTS` confirmed supported in v26.1/v26.2 docs
- SQLSTATE fallback (lines 118-130) retained as defense-in-depth — prevents crash on concurrent startup race
- TODO comment prevents dead code from becoming permanent

---

### 2. `scripts/crdb-init.sql` (new file)

Full contents:
```sql
-- =============================================================================
-- Local CockroachDB Initialization Script
-- =============================================================================
-- Usage:
--   cockroach sql --url "postgresql://root@localhost:26257/bchat?sslmode=disable" \
--     < scripts/crdb-init.sql
--
-- Idempotent: Safe to run multiple times. All SET statements are no-ops if
-- already applied. Run after crdb:up, crdb:reset, or container restart.
-- =============================================================================

-- === REQUIRED: Vector index support ===
-- Must be enabled before any CREATE VECTOR INDEX statements.
SET CLUSTER SETTING feature.vector_index.enabled = true;

-- === REQUIRED: Sequence normalization ===
-- Session-level setting. Ensures SERIAL columns use nextval() sequences
-- instead of unique_rowid().
--
-- WHY DUPLICATED WITH migrator.go?
--   crdb-init.sql  → ensures manual cockroach sql sessions get correct behavior
--   migrator.go     → guarantees migration behavior regardless of external setup
-- Both are needed: the migrator can't assume the init script was run, and the
-- init script can't assume the migrator will run before manual SQL sessions.
SET serial_normalization = 'sql_sequence';

-- === DEV-ONLY: Performance tuning for local development ===
-- These speed up migration re-runs by reducing GC and stats overhead.
-- They are NOT required for correctness. Safe to skip on Basic tier.
SET CLUSTER SETTING jobs.registry.interval.gc = '30s';
SET CLUSTER SETTING jobs.retention_time = '15s';
SET CLUSTER SETTING sql.stats.automatic_collection.enabled = false;
```

**Key design decisions:**
- `set -e` not needed here — script is non-interactive SQL, errors surface immediately
- Documented why `serial_normalization` appears in both crdb-init.sql and `migrator.go:213` (reviewer nit)
- Separated required vs dev-only settings with clear comments

---

### 3. `Taskfile.yml`

#### New target: `crdb:init`

```yaml
crdb:init:
    desc: Apply cluster settings to local CockroachDB (idempotent, safe to rerun)
    cmds:
      - |
        set -e
        echo "=== Waiting for CockroachDB SQL readiness ==="
        for i in $(seq 1 30); do
          if cockroach sql --url "postgresql://root@localhost:26257/bchat?sslmode=disable" -e "SELECT 1;" >/dev/null 2>&1; then
            echo "CockroachDB SQL ready (attempt $i)"
            break
          fi
          if [ "$i" -eq 30 ]; then
            echo "ERROR: CockroachDB not ready after 30 attempts"
            exit 1
          fi
          echo "  Waiting for SQL... (attempt $i/30)"
          sleep 2
        done
        echo "=== Applying cluster settings ==="
        cockroach sql --url "postgresql://root@localhost:26257/bchat?sslmode=disable" \
          < scripts/crdb-init.sql
        echo "=== Cluster settings applied ==="
```

**Why retry loop:** `docker compose up -d --wait` waits for healthcheck, but SQL readiness can lag a few seconds after healthcheck passes. The retry loop handles this gracefully with `set -e` for fail-fast (reviewer nit).

#### Updated: `crdb:up`

```yaml
crdb:up:
    desc: Start local CockroachDB compose cluster (waits for healthcheck)
    cmds:
      - docker compose -f scripts/docker-compose.cockroach.yml up -d --wait
```

#### Updated: `crdb:reset`

```yaml
crdb:reset:
    desc: Wipe and restart local CockroachDB compose cluster (A1 state)
    cmds:
      - docker compose -f scripts/docker-compose.cockroach.yml down -v
      - docker compose -f scripts/docker-compose.cockroach.yml up -d --wait
      - task: crdb:init
```

**Key:** Uses `task: crdb:init` (Taskfile dependency syntax, not `task crdb:init` shell command — reviewer nit).

#### Enhanced: `crdb:verify` (after P1-P6 checks)

```yaml
JOBS=$(run_sql "SELECT count(*) FROM [SHOW JOBS] WHERE status = 'failed' AND job_type IN ('SCHEMA CHANGE', 'NEW SCHEMA CHANGE');" 2>/dev/null | tail -1)
if [ "$JOBS" != "0" ] && [ -n "$JOBS" ]; then
  echo "WARN: $JOBS failed schema job(s) found"
  run_sql "SELECT job_id, job_type, status, error FROM [SHOW JOBS] WHERE status = 'failed' AND job_type IN ('SCHEMA CHANGE', 'NEW SCHEMA CHANGE') LIMIT 5;" 2>/dev/null
else
  echo "OK: no failed schema jobs"
fi
```

#### Renamed: `crdb:cluster:bootstrap`

Old cloud placeholder `crdb:init` renamed to `crdb:cluster:bootstrap` to avoid conflict with new local `crdb:init`.

---

## Gate 0 Execution Sequence

```bash
task crdb:reset              # wipe + start (waits for healthcheck) + init (chained)
task build:backend:cockroach  # compile with cockroach tag
task crdb:migrate             # apply migrations + validate serial_normalization
task crdb:verify              # P1-P6 + SHOW JOBS check
task crdb:up                  # restart without wipe (reuses existing data)
task crdb:verify              # idempotency proof
```

---

## Adversarial Code Review Prompt

```
You are reviewing a 3-file implementation for CockroachDB local E2E testing in a
Go/React multi-tenant AI chat agent platform (bchat).

CONTEXT:
- Local dev runs cockroachdb/cockroach:v26.2.1 single-node insecure
- Target production is CockroachDB Basic (serverless) v26.2.1
- agent_vectors table uses VECTOR(1536) + CREATE VECTOR INDEX (CRDB-specific)
- simple_protocol workaround kept (Option B) — OID 90006 bug fixed in v25.3+
- Validate() called during reindex (admin-triggered singleton), NOT on every boot

REVIEW FOR:

1. CORRECTNESS:
   - Does CREATE VECTOR INDEX IF NOT EXISTS work in CRDB v26.2? (docs say v26.1+)
   - Is the retry loop (30 attempts, 2s apart) sufficient for container readiness?
   - Does --wait on docker compose up guarantee SQL is accepting connections?
   - Are all crdb-init.sql settings idempotent?

2. RACE CONDITIONS:
   - Can concurrent Validate() calls (multiple replicas) cause vector index errors?
   - Is the SQLSTATE fallback (lines 118-130) necessary alongside IF NOT EXISTS?
   - Could the retry loop exit before SQL is fully ready (e.g., DDL still in progress)?

3. SECURITY:
   - Any SQL injection risk in crdb-init.sql (hardcoded settings, no user input)?
   - Is the cockroach sql URL with sslmode=disable acceptable for local dev?

4. IDEMPOTENCY:
   - Can crdb:reset be run multiple times safely?
   - Does docker compose down -v reliably wipe volumes?
   - Will crdb:init fail if cluster settings already applied?

5. COMPLETENESS:
   - Are all reviewer nits from plan2 addressed (set -e, task dep syntax, SHOW JOBS)?
   - Is the TODO comment on SQLSTATE fallback actionable?
   - Is the serial_normalization duplication documented clearly enough?
   - Does crdb:verify catch failed schema jobs before they block the app?

6. REGRESSION RISK:
   - Do these changes affect non-cockroach builds? (build tag: cockroach only)
   - Could the renamed crdb:cluster:bootstrap break any CI/CD pipelines?
   - Is the Taskfile valid YAML after all edits?

FILES TO REVIEW:
1. server/router/api/v1/agent/vectordb_cockroach.go (line 112)
2. scripts/crdb-init.sql (new file, 38 lines)
3. Taskfile.yml (crdb:init, crdb:reset, crdb:verify, crdb:cluster:bootstrap)

EXPECTED OUTPUT:
- For each finding: severity (Critical/High/Medium/Low), whether it's a blocker,
  and suggested fix.
- Overall verdict: APPROVE / APPROVE WITH NITS / REQUEST CHANGES
```
