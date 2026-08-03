# Bug 058 — Local E2E Testing Plan v2

**Date:** 2026-08-03
**Author:** opencode
**Status:** Revised per adversarial reviews
**Depends on:** Bug 058 implementation (3-file changes + code review fixes)

---

## Background

### What We Built

Three-file change enabling local CockroachDB E2E testing:

| File | Change |
|------|--------|
| `server/router/api/v1/agent/vectordb_cockroach.go:112` | `CREATE VECTOR INDEX IF NOT EXISTS` (was missing `IF NOT EXISTS`) |
| `scripts/crdb-init.sql` | New file — cluster settings, `set -e`, retry loop, documented `serial_normalization` duplication |
| `Taskfile.yml` | New `crdb:init` target, `--wait` on `crdb:up`/`crdb:reset`, `SHOW JOBS` in `crdb:verify` |

### What We're Proving

Local CockroachDB E2E validates the full stack works **before** touching cloud. If local passes, the application is functionally ready for cloud deployment. Cloud-specific operational differences (managed service limits, networking, resource sizing) remain to be validated.

### Infrastructure

| Component | Details |
|-----------|---------|
| CockroachDB | `cockroachdb/cockroach:v26.2.1` single-node insecure |
| Container | Docker Compose, port 26257 (SQL), 8080 (DB Console) |
| Credentials | `bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable` |
| App binary | `go build -tags "cockroach"` → `build/memos` |
| Vector DB | CockroachDB native `VECTOR(1536)` + `CREATE VECTOR INDEX` |
| LLM/Embeddings | OpenRouter API (`OPENROUTER_API_KEY` required) |
| RAG | `LANCEDB_STORAGE_PROVIDER=cockroach` |

### Key Architecture Decisions

1. **`simple_protocol` workaround (Option B)** — OID 90006 binary encoding bug fixed in v25.3+, but keeping it is zero-risk. `pgx` driver appends `default_query_exec_mode=simple_protocol` to DSN automatically.

2. **`Validate()` lifecycle** — Called during reindex (`service.go:1274` inside `shouldValidateReindex()` guard), NOT on every boot. Reindex triggered by admin API `POST /:slug/reindex` (singleton per-tenant).

3. **`agent_vectors` runtime creation** — Correct by design. CockroachDB-specific (`VECTOR(1536)`, `CREATE VECTOR INDEX`), behind `//go:build cockroach` tag. NOT a shared migration table — lives in `Validate()`, not `LATEST.sql`.

4. **`serial_normalization = 'sql_sequence'`** — Dual-purpose: `crdb-init.sql` covers manual SQL sessions, `migrator.go:213` covers programmatic migrations. Both needed because neither can assume the other runs first.

5. **`CREATE VECTOR INDEX IF NOT EXISTS`** — Confirmed supported in v26.1/v26.2 docs and live probe. SQLSTATE fallback retained as defense-in-depth for concurrent `Validate()` calls.

---

## Phase 1: Infrastructure Startup

**Goal:** Start local CockroachDB, apply cluster settings, verify SQL connectivity.

### Steps

```bash
# 1. Wipe any existing data, start container (waits for healthcheck)
task crdb:reset
```

**What this does internally:**
1. `docker compose down -v` — destroys container and volume
2. `docker compose up -d --wait` — starts container, waits for healthcheck (`cockroach node status` every 5s, 5 retries)
3. `task: crdb:init` — waits for SQL readiness (retry loop: 30 attempts, 2s apart), then applies `scripts/crdb-init.sql`

```bash
# 2. Verify cluster settings
cockroach sql --url "postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable" \
  -e "SHOW CLUSTER SETTING feature.vector_index.enabled;"
```

**Expected:** `true`

```bash
# 3. Verify SQL connectivity (manual session only — migrator uses its own session)
cockroach sql --url "postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable" \
  -e "SELECT 1;"
```

**Expected:** `1`

**Note:** `SHOW serial_normalization` is deliberately omitted from Phase 1 — it's a session variable and only proves the current shell session has the value, not the migrator session. The authoritative proof is `TestCockroachP0` in Phase 2 (checks `nextval()` defaults, no `unique_rowid()`).

### Gate Criteria

| Check | Expected | Fail Action |
|-------|----------|-------------|
| Container running | `docker ps` shows `bchat-crdb` | `docker compose logs cockroach` for startup errors |
| Healthcheck passing | `docker inspect --format='{{.State.Health.Status}}' bchat-crdb` = `healthy` | Wait 30s, retry |
| SQL connectivity | `SELECT 1` succeeds | Check port 26257 not blocked |
| `feature.vector_index.enabled` | `true` | Re-run `crdb:init` |
| SHOW JOBS — no stuck jobs | 0 running/pending schema changes | Wait 30s, re-check |

---

## Phase 2: Database Migration

**Goal:** Apply full schema (LATEST.sql), verify table structure, indexes, and `nextval()` defaults.

### Steps

```bash
# 4. Boot app against CockroachDB (applies migrations)
task crdb:migrate
```

**What this does internally:**
1. Loads `.env` if present
2. Runs `./build/memos --driver=cockroach --mode dev --data build/data`
3. `migrator.go` detects `driver == "cockroach"`:
   - Skips explicit transaction (CRDB doesn't support DDL in transactions)
   - Prepends `SET serial_normalization = 'sql_sequence'`
   - Executes `LATEST.sql` as single `ExecContext` call
   - Tolerates `"already exists"` errors (idempotent)
4. Process **exits after migration completes** (no HTTP server started — `--mode dev` with no explicit `serve` subcommand)

```bash
# 5. Run P0 gate test
go test -tags "cockroach integration" ./store/ -run TestCockroachP0 -v
```

**What this verifies:**
- `SET serial_normalization = 'sql_sequence'` + entire `LATEST.sql` executes as one batch
- `nextval()` defaults on serial tables (`agent_tenants`, `memo`, `tickets`)
- Zero `unique_rowid()` defaults (int32 ID safety)
- Idempotent re-run succeeds
- Duplicate-table error contains `"already exists"` (migrator tolerance)

```bash
# 6. Run E2E migration test
BCHAT_ALLOW_DB_RESET=1 go test -tags "cockroach integration" ./store/test/ -run TestCockroachMigrateEndToEnd -v
```

**What this verifies:**
- Scenario A1: Fresh database gets full LATEST.sql + migration_history
- Scenario A2: Re-run with history present is a no-op
- Scenario A3: Failed-boot recovery (history wiped, then re-apply)
- Scenario A4: Stable state after corrupt history
- `serial_normalization=sql_sequence` — IDs come from `nextval()` (int32-safe)
- Tenant row round-trips through store layer with int32 ID

```bash
# 7. Run full verification
task crdb:verify
```

**What this verifies (P1-P6 + SHOW JOBS):**
- P1: `SELECT 1` — basic connectivity
- P2: `current_database()` = expected DB name
- P3: `version()` contains "Cockroach"
- P4: `migration_history` = 1 row
- P5: `nextval()` defaults present on `agent_tenants`
- P6: `feature.vector_index.enabled` = true + `agent_vectors` indexed
- SHOW JOBS: No failed OR running/pending schema change jobs

### Gate Criteria

| Check | Expected | Fail Action |
|-------|----------|-------------|
| `crdb:migrate` exits 0 | App starts successfully | Check app logs for migration errors |
| `TestCockroachP0` passes | All assertions pass | Check `serial_normalization` setting |
| `TestCockroachMigrateEndToEnd` passes | All scenarios pass | Check `BCHAT_ALLOW_DB_RESET` env |
| `crdb:verify` exits 0 | All P1-P6 pass, no failed jobs | Check individual check output |
| `agent_vectors` table exists | `SHOW TABLES LIKE 'agent_vectors'` returns row | Check `Validate()` logs |
| Vector index exists | `SHOW INDEXES FROM agent_vectors` returns row | Check `feature.vector_index.enabled` |
| SHOW JOBS — no failed/running jobs | All counts = 0 | Check job error messages |

---

## Phase 3: App Startup

**Goal:** Boot bchat application against CockroachDB, verify HTTP health and vector DB access.

### Prerequisites

**CRITICAL (F1 blocker):** `run:cockroach` loads `.env` via `set -a && . .env && set +a`. If `MEMOS_DRIVER=cockroach` is not in `.env` or the shell environment, the app will silently fall back to SQLite and Phase 3 will pass while testing nothing about CockroachDB.

```bash
# Verify MEMOS_DRIVER is set (run before Phase 3)
grep "MEMOS_DRIVER" .env
# Expected: MEMOS_DRIVER=cockroach
# If missing, add it:
echo "MEMOS_DRIVER=cockroach" >> .env
```

### Steps

```bash
# 8. Ensure environment is configured
# Verify these are in .env or exported:
#   MEMOS_DRIVER=cockroach
#   COCKROACH_DSN=postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable
#   OPENROUTER_API_KEY=sk-or-v1-xxx
#   RAG_PIPELINE_ENABLED=true
#   LANCEDB_STORAGE_PROVIDER=cockroach
#   EMBEDDING_PROVIDER=openrouter
#   TICKET_EMBEDDING_ENABLED=true
```

```bash
# 9. Start app in background with PID capture
task run:cockroach &
BCHAT_PID=$!
echo "bchat PID: $BCHAT_PID"
# Wait for startup
sleep 10
```

```bash
# 10. Verify HTTP health
curl -fsS http://localhost:5230/healthz
```

**Expected:** `OK` or `200` response

```bash
# 11. Verify vector DB exists (schema created, 0 rows before reindex)
cockroach sql --url "postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable" \
  -e "SHOW TABLES LIKE 'agent_vectors';"
```

**Expected:** Returns row (table exists)

```bash
# 12. Verify app can use vector DB (schema-level check)
cockroach sql --url "postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable" \
  -e "SHOW INDEXES FROM agent_vectors;"
```

**Expected:** Returns index rows (B-tree + vector index)

### Gate Criteria

| Check | Expected | Fail Action |
|-------|----------|-------------|
| `MEMOS_DRIVER` set in `.env` | `grep` returns match | Add to `.env` before proceeding |
| App exits 0 (doesn't crash) | HTTP server starts | Check app logs for DSN errors |
| `/healthz` returns 200 | HTTP 200 | Check app logs for migration/vector errors |
| No SQLSTATE errors in logs | Clean startup | Check `simple_protocol` DSN param |
| `agent_vectors` table exists | `SHOW TABLES LIKE 'agent_vectors'` returns row | Check `Validate()` execution |
| Vector index exists | `SHOW INDEXES FROM agent_vectors` returns rows | App can create schema |
| Errors reference CockroachDB, not SQLite | Driver = cockroach | Check `MEMOS_DRIVER` env var |

---

## Phase 4: Full Data Path

**Goal:** Exercise the complete data flow: sign-in → tenant → KB import → reindex → RAG search.

### Steps

```bash
# 13. Run production smoke test against local
BCHAT_URL=http://localhost:5230 \
BCHAT_USER=<your-email> \
BCHAT_PASS=<your-password> \
bash scripts/verify-production.sh
```

**What this does internally:**
1. `healthz` check
2. REST sign-in (session cookie)
3. Multi-tenant flow (`/auth/tenants` + `/auth/select-tenant`)
4. Onboard test tenant (`verify-<ts>`)
5. KB import (uploads test KB content)
6. RAG reindex (triggers `Validate()`, creates vector index, embeds content)
7. RAG search (vector round-trip, retries up to 12 times with 5s delays)
8. Cleanup (destroys test tenant, unless `--keep`)

```bash
# 14. Verify embeddings landed in CockroachDB (direct DB confirmation)
cockroach sql --url "postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable" \
  -e "SELECT count(*) FROM agent_vectors;"
```

**Expected:** ≥1 row (embeddings persisted)

```bash
# 15. Check for OpenRouter-specific errors (distinct from CockroachDB errors)
# If reindex/search fails, check logs for:
#   - "OpenRouter" / "HTTP 429" / "401" → embedding provider issue (not DB)
#   - "SQLSTATE" / "cockroach" / "vector" → CockroachDB issue
```

### Gate Criteria

| Check | Expected | Fail Action |
|-------|----------|-------------|
| Sign-in succeeds | Session cookie returned | Check user credentials |
| Tenant onboarding succeeds | Test tenant created | Check `/api/v1/auth/tenants` |
| KB import succeeds | Content stored in `agent_source_files` | Check upload handler logs |
| RAG reindex succeeds | Chunks embedded in `agent_vectors` | Check embedding provider logs |
| RAG search returns results | ≥1 hit with score > threshold | Check vector search SQL |
| Cleanup succeeds | Test tenant destroyed | Manual cleanup via SQL |
| `agent_vectors` count > 0 | Direct DB confirmation | Check reindex logs |
| Failures distinguish DB vs OpenRouter | Error context clear | Check error messages |

---

## Phase 5: Idempotency Proof

**Goal:** Verify app survives restart without data wipe.

**Why skip `crdb:init`:** Cluster settings (`feature.vector_index.enabled`, `jobs.*`, `sql.stats.*`) are durably stored in the CockroachDB volume and persist across restarts. `serial_normalization` is a session variable and does NOT persist — but this is safe because no new tables are created in Phase 5, and `migrator.go` re-prepends it at every migration execution.

### Steps

```bash
# 16. Stop app (capture PID from Phase 3)
kill $BCHAT_PID
wait $BCHAT_PID 2>/dev/null
```

```bash
# 17. Stop CockroachDB (preserve volume)
task crdb:down
```

```bash
# 18. Start CockroachDB (reuse existing data — cluster settings persist in volume)
task crdb:up
# Note: crdb:init skipped — no new tables being created, cluster settings persist
```

```bash
# 19. Restart app in background
task run:cockroach &
BCHAT_PID=$!
echo "bchat PID: $BCHAT_PID"
sleep 10
```

```bash
# 20. Verify infrastructure survived restart
task crdb:verify
```

```bash
# 21. Verify application survived restart
BCHAT_URL=http://localhost:5230 \
BCHAT_USER=<your-email> \
BCHAT_PASS=<your-password> \
bash scripts/verify-production.sh
```

```bash
# 22. Verify embeddings persisted (direct DB check)
cockroach sql --url "postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable" \
  -e "SELECT count(*) FROM agent_vectors;"
```

**What this proves:**
- Data survives container restart (volume persistence)
- Migrations are idempotent (re-run is no-op)
- Vector index persists (no re-creation needed)
- App can recover from clean shutdown
- Schema + application state consistent after restart

### Gate Criteria

| Check | Expected | Fail Action |
|-------|----------|-------------|
| `crdb:up` starts successfully | Container healthy | Check volume mount |
| `task crdb:verify` passes | Schema intact | Check migration_history count |
| `verify-production.sh` passes | Data persists | Check `agent_vectors` row count |
| `agent_vectors` count unchanged | Same as Phase 4 | Check for duplicate embeddings |
| No duplicate data | Same results as Phase 4 | Check idempotent reindex logic |

---

## Phase 6: Cleanup & Gate Decision

**Goal:** Document go/no-go for cloud deployment.

### Steps

```bash
# 23. Stop app
kill $BCHAT_PID
wait $BCHAT_PID 2>/dev/null
```

```bash
# 24. Stop CockroachDB
task crdb:down
```

```bash
# 25. Clean build data (SQLite data only — CockroachDB data lives in Docker volume)
rm -rf build/data
```

```bash
# 26. Document results
```

### Go/No-Go Checklist

| Gate | Status | Notes |
|------|--------|-------|
| Phase 1: Infrastructure | [ ] Pass | |
| Phase 2: Migration | [ ] Pass | |
| Phase 3: App Startup | [ ] Pass | |
| Phase 4: Data Path | [ ] Pass | |
| Phase 5: Idempotency | [ ] Pass | |
| No SQLSTATE errors in logs | [ ] Pass | |
| Vector round-trip works | [ ] Pass | |
| No `unique_rowid()` defaults | [ ] Pass | |
| SHOW JOBS — no failed/running jobs | [ ] Pass | |
| `agent_vectors` count > 0 after reindex | [ ] Pass | |
| Embeddings distinguish DB vs OpenRouter errors | [ ] Pass | |

### Decision

- **ALL PASS** → Proceed to `plan_cloud.md` (cloud deployment)
- **ANY FAIL** → Fix in local, re-run from failed phase

---

## Troubleshooting

### Common Issues

| Issue | Symptom | Fix |
|-------|---------|-----|
| Container not starting | `docker compose ps` shows `unhealthy` | Check port 26257 not in use: `lsof -i :26257` |
| SQL connection refused | `cockroach sql` fails | Wait 30s for startup, check `docker compose logs cockroach` |
| Migration fails | App crashes on startup | Check `COCKROACH_DSN` env var, verify `serial_normalization` setting |
| Vector index missing | RAG search returns empty | Check `feature.vector_index.enabled = true`, re-run `crdb:init` |
| Embedding errors | `agent_vectors` has 0 rows | Check `OPENROUTER_API_KEY`, verify embedding provider logs |
| OID 90006 error | Binary format encoding fails | Verify `simple_protocol` in DSN (auto-appended by pgx) |
| Duplicate data | Multiple embeddings per chunk | Check idempotent reindex logic, verify `IF NOT EXISTS` on vector index |
| App falls back to SQLite | Phase 3 passes but `agent_vectors` doesn't exist | Check `MEMOS_DRIVER=cockroach` in `.env` |
| Port conflict | Phase 3 can't bind to 5230 | Kill Phase 2 process first, or use different port |

### Useful Commands

```bash
# Check container status
docker compose -f scripts/docker-compose.cockroach.yml ps

# Check app logs
# (app runs in foreground, logs to stdout)

# Direct SQL queries
cockroach sql --url "postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable"

# Check table structure
SHOW CREATE TABLE agent_vectors;

# Check vector index
SHOW INDEXES FROM agent_vectors;

# Check migration history
SELECT * FROM migration_history;

# Check failed jobs
SELECT job_id, job_type, status, error FROM [SHOW JOBS] WHERE status = 'failed';

# Check running jobs (should be 0 after migration)
SELECT job_id, job_type, status FROM [SHOW JOBS] WHERE status IN ('running', 'pending');

# Check vector count
SELECT count(*) FROM agent_vectors;

# Check embedding dimensions
SELECT length(embedding::text) FROM agent_vectors LIMIT 1;

# Verify MEMOS_DRIVER is set
grep "MEMOS_DRIVER" .env
```

---

## Adversarial Review Prompt

```
You are reviewing a revised local E2E testing plan for a CockroachDB-backed Go application
(bchat). The plan validates a 3-file implementation before cloud deployment.

CONTEXT:
- 3-file change: vectordb_cockroach.go (IF NOT EXISTS), crdb-init.sql (cluster settings),
  Taskfile.yml (crdb:init, crdb:reset, crdb:verify)
- Local dev: cockroachdb/cockroach:v26.2.1 single-node insecure
- Target production: CockroachDB Basic (serverless) v26.2.1
- Vector storage: CockroachDB native VECTOR(1536) + CREATE VECTOR INDEX
- simple_protocol workaround kept (OID 90006 bug fixed in v25.3+)
- Validate() called during reindex (admin-triggered singleton), NOT on every boot
- MEMOS_DRIVER env var must be set in .env for run:cockroach to use CockroachDB

REVIEW FOR:

1. COMPLETENESS:
   - Does the plan cover all critical failure modes?
   - Are there missing verification steps?
   - Is the gate criteria sufficient for cloud deployment readiness?
   - Are troubleshooting scenarios comprehensive?

2. CORRECTNESS:
   - Are the expected outputs correct for each check?
   - Are the fail actions appropriate?
   - Is the idempotency proof sufficient?
   - Are there any race conditions in the test sequence?

3. SEQUENCING:
   - Are dependencies between phases correct?
   - Can any phases run in parallel?
   - Is the cleanup step safe (won't destroy needed data)?
   - Are there any blocking steps that could hang?

4. RISK COVERAGE:
   - Does the plan catch the OID 90006 binary format bug?
   - Does the plan verify serial_normalization = sql_sequence?
   - Does the plan verify no unique_rowid() defaults?
   - Does the plan verify vector index creation?
   - Does the plan verify vector round-trip (embed → search)?
   - Does the plan verify MEMOS_DRIVER is set before Phase 3?

5. OPERATIONAL:
   - Are all commands copy-pasteable?
   - Are env vars clearly documented?
   - Are failure modes actionable?
   - Is the troubleshooting section useful?
   - Is the PID capture and cleanup handled correctly?

6. CLOUD READINESS:
   - Does local E2E give confidence for cloud deployment?
   - What gaps remain between local and cloud?
   - Is the go/no-go checklist sufficient?

EXPECTED OUTPUT:
- For each finding: severity (Critical/High/Medium/Low), whether it's a blocker,
  and suggested fix.
- Overall verdict: APPROVE / APPROVE WITH NITS / REQUEST CHANGES
- List any additional tests that should run before cloud deployment
```
