# Bug 058 — Plan v2: Local CockroachDB Full E2E Testing

**Date:** 2026-08-03
**Status:** REVISED — Incorporates adversarial review findings from `plan_review_claude.md` and `plan_review_chatgpt.md`
**Goal:** Test bchat against local single-node CockroachDB before any cloud deployment

---

## Part 1: Background Context & Investigation

### What Previous Plans Claimed (And What Was Wrong)

Three plan iterations (`plan_hermes_20260803_064328.md`, `plan_hermes_20260803_064900.md`, `plan_hermes_20260803_072500.md`) were produced by a previous agent. Each contained claims that do not hold up against CockroachDB documentation or code inspection:

| Claim in Previous Plans | Reality |
|------------------------|---------|
| "CREATE VECTOR INDEX does NOT support IF NOT EXISTS" | **Wrong.** CockroachDB v26.1/v26.2 docs confirm `IF NOT EXISTS` is part of the standard `CREATE INDEX` synopsis for VECTOR INDEX. Live probe (evidence_20260803.md Task A.1) also confirmed it parses and executes. |
| "P0 concurrency race fix — DONE" | **Not in code.** `vectordb_cockroach.go:112` still reads `CREATE VECTOR INDEX idx_agent_vectors_embedding` without `IF NOT EXISTS`. The plan claimed the fix was applied but the file was never modified. |
| "Credential rotation — DONE" | **Not verifiable from code.** Password was exposed in `evidence_20260803.md` and `gemini.md`. Whether rotation actually happened cannot be confirmed from the repository alone. |
| "agent_vectors should move to LATEST.sql" | **Wrong approach.** `agent_vectors` uses CockroachDB-native `VECTOR(1536)` type and `CREATE VECTOR INDEX`. The table is behind `//go:build cockroach` and only exists when `LANCEDB_STORAGE_PROVIDER=cockroach`. Putting it in a shared LATEST.sql would break SQLite/Postgres builds. Runtime creation in `Validate()` is correct by design for a multi-backend system. |
| "simple_protocol must be replaced with pgtype codec (Option A)" | **Premature.** The pgx/v5 binary encoding bug (OID 90006 FormatBinary) was fixed in v25.3+ (PR #148719, #148843). Both local Docker (v26.2.1) and Cloud Basic (v26.2.1) have the fix. `simple_protocol` is technically unnecessary but harmless. Option A's code sample had a wiring bug (`pgxpool.Config.AfterConnect` ignored by `sql.Open`). Option B (keep simple_protocol) is correct for this pass. |
| "kv.range_merge.queue_interval = '50ms'" | **Nonexistent setting.** Live probe returned SQLSTATE 42P02 ("unknown cluster setting"). This setting does not exist in v26.2. It must be removed from all init scripts. |

### CockroachDB Documentation-Grounded Findings (via MCP)

#### Finding 1: `CREATE VECTOR INDEX IF NOT EXISTS` — Supported

**Source:** CockroachDB v26.1 and v26.2 `CREATE INDEX` documentation.

The `CREATE INDEX` synopsis includes `IF NOT EXISTS` for all index types, including VECTOR INDEX:

```
VECTOR INDEX [CONCURRENTLY] [IF NOT EXISTS] index_name ON table_name ...
```

The `IF NOT EXISTS` parameter is documented as:
> Create a new index only if an index of the same name does not already exist; if one does exist, do not return an error.

**Live confirmation:** `evidence_20260803.md` Task A.1 ran `CREATE VECTOR INDEX IF NOT EXISTS idx_probe_test ON agent_vectors (embedding)` against a live v26.2.1 Basic cluster. It succeeded in 36.657s (cold-start latency, not backfill).

**Conclusion:** The SQLSTATE 42P07/0A000 error trap in `vectordb_cockroach.go` is a safe-but-unnecessary workaround. Adding `IF NOT EXISTS` to the DDL is the correct fix.

#### Finding 2: Vector Index on Non-Empty Table — Blocks Writes

**Source:** CockroachDB v25.2+ Known Limitations, issue CRDB-48656 / #144443 (still open).

> Creating a vector index through a backfill disables mutations (INSERT, UPSERT, UPDATE, DELETE) on the table.

To create a vector index on a non-empty table:
```sql
SET sql_safe_updates = false;
CREATE VECTOR INDEX ... ON non_empty_table (embedding);
```

This blocks all writes until backfill completes.

**Impact on bchat:** For the initial boot, `agent_vectors` is created empty and the vector index is created immediately — no backfill, no blocking. This is the correct path. The risk only applies if someone later tries to add a vector index to an already-populated table (future migration edge case).

**Conclusion:** Keep `CREATE VECTOR INDEX IF NOT EXISTS` in `Validate()`. The empty-table path avoids backfill entirely.

#### Finding 3: OID 90006 Binary Format Bug — Fixed in v25.3+, Not v25.2

**Source:** CockroachDB GitHub issues #147844 (original), #148719 (fix merged to master 2025-06-25), #148843 (backport to release-25.3 2025-06-27), #170485 (crash on v25.2.18), #172672 (crash on v25.2.0).

The bug: `DecodeDatum` in `encoding.go` does not handle OID 90006 (`T_pgvector`) in `FormatBinary` mode. Any client using pgx binary parameter binding for VECTOR columns triggers an assertion failure.

**Status by version:**
| Branch | Status |
|--------|--------|
| v25.2.x | **NOT FIXED** — no backport exists |
| v25.3+ | Fixed — PR #148843 |
| v25.4+ | Fixed |
| v26.2.1 | Fixed (has the fix from master) |

**Impact on bchat:** Both local Docker (`cockroachdb/cockroach:v26.2.1`) and Cloud Basic (`v26.2.1`) have the fix. The `simple_protocol` workaround is technically unnecessary on v26.2.1.

**Conclusion:** Keep `simple_protocol` (Option B) for this pass. It's a safe, zero-risk workaround. The proper fix (pgtype codec registration) is a post-hackathon follow-up.

#### Finding 4: `SET CLUSTER SETTING` Requires `admin` or `MODIFYCLUSTERSETTING`

**Source:** CockroachDB v26.2 `SET CLUSTER SETTING` documentation.

> To use the `SET CLUSTER SETTING` statement, a user must have one of the following:
> - Be a member of the `admin` role
> - Have the `MODIFYCLUSTERSETTING` system-level privilege

On local `--insecure` single-node, `root` has admin. All cluster settings work.

On Basic tier, `root` also has admin, but CockroachDB Cloud may restrict certain internal settings. Evidence from `evidence_20260803.md` Task A:

| Setting | Result | Notes |
|---------|--------|-------|
| `feature.vector_index.enabled` | ✅ Succeeded | Required for vector indexes |
| `serial_normalization` | ✅ Succeeded | Session variable, not cluster setting |
| `kv.range_merge.queue_interval` | ❌ 42P02 | **Nonexistent** — setting name doesn't exist in v26.2 |
| `jobs.registry.interval.gc` | ✅ Succeeded | Unexpected but confirmed |
| `jobs.retention_time` | ✅ Succeeded | Unexpected but confirmed |
| `sql.stats.automatic_collection.enabled` | ✅ Succeeded | Unexpected but confirmed |

**Conclusion:** For local init, apply all settings. Remove `kv.range_merge.queue_interval` (nonexistent). The three `jobs.*`/`sql.stats.*` settings are local-dev-only performance tuning — harmless on local, potentially useful on Basic tier too.

#### Finding 5: `agent_vectors` Runtime Creation Is Correct by Design

**Source:** Code inspection of `vectordb_cockroach.go` (build tag `cockroach`), LanceDB implementations.

The `agent_vectors` table:
- Is behind `//go:build cockroach` — only compiled for CockroachDB builds
- Uses `VECTOR(1536)` — a CockroachDB-native type (not standard PostgreSQL)
- Uses `CREATE VECTOR INDEX` — CockroachDB-native syntax (not `USING hnsw`)
- Is created by `Validate()` during reindex operations, not on every boot
- LanceDB has its own separate implementation

LATEST.sql at `store/migration/cockroach/LATEST.sql` contains 57 tables that are shared infrastructure (tenants, sessions, messages, etc.). `agent_vectors` is RAG storage-layer DDL that differs per backend. Moving it to LATEST.sql would:
- Break SQLite/Postgres builds (VECTOR type doesn't exist)
- Couple storage-layer DDL to schema migrations
- Remove the ability to have backend-specific init logic

**Conclusion:** Runtime creation in `Validate()` is the correct architectural choice. No migration change needed.

---

## Part 2: Adversarial Review Findings (Incorporated)

### From `plan_review_claude.md`

| Finding | Valid? | Resolution |
|---------|--------|------------|
| Q5 connection pinning may cause `serial_normalization` to not persist | **Investigated — NOT A BUG.** `migrator.go:212-213` concatenates SET + LATEST.sql into ONE string, passed to ONE `db.ExecContext()` call. Go's `database/sql` checks out ONE connection for the call. Session variable persists. Code comment at line 209: "The SET + whole file is one statement (P0-verified)." |
| Test concurrent `CREATE VECTOR INDEX IF NOT EXISTS` | **Valid.** Add to execution sequence. |
| Explicit exit-code checking in `crdb:init` | **Valid.** Add `set -e` or `$?` check. |
| Q2 `Validate()` lifecycle | **Investigated — called during reindex, not every boot.** `service.go:1274` calls `s.vectorDB.Validate(ctx)` inside `shouldValidateReindex()` guard. |
| Q7 verify-production URL | **Investigated — works with HTTP.** `verify-production.sh:15`: `URL="${BCHAT_URL:-https://...}"` uses env var. |

### From `plan_review_chatgpt.md`

| Finding | Valid? | Resolution |
|---------|--------|------------|
| Nit 1: `crdb:init` should be idempotent | **Valid.** All `SET CLUSTER SETTING` statements are idempotent by design. Explicitly document this. |
| Nit 2: Separate required vs tuning settings | **Valid.** Use clear section comments in crdb-init.sql. |
| Nit 3: Verify vector index health | **Valid.** Add `SHOW INDEX FROM agent_vectors` check to `crdb:verify`. |
| Nit 4: Keep SQLSTATE fallback alongside `IF NOT EXISTS` | **Valid.** Keep the error handling as defense-in-depth. |
| Nit 5: Add restart verification | **Valid.** Add restart → re-verify to execution sequence. |
| `crdb:reset` should invoke `crdb:init` | **Valid.** Makes `crdb:reset` self-contained. |

---

## Part 3: The Plan

### Scope: 3 Files Changed

#### Change 1: `server/router/api/v1/agent/vectordb_cockroach.go` (line 112)

Add `IF NOT EXISTS` to the `CREATE VECTOR INDEX` statement. Keep existing SQLSTATE fallback handling.

**Current (line 112):**
```go
CREATE VECTOR INDEX idx_agent_vectors_embedding
ON agent_vectors (embedding)
```

**New:**
```go
CREATE VECTOR INDEX IF NOT EXISTS idx_agent_vectors_embedding
ON agent_vectors (embedding)
```

**Why:** Prevents duplicate-index error on concurrent replica startup. Confirmed supported by CockroachDB v26.1/v26.2 docs and live probe. Keep the SQLSTATE 42P07/0A000 trap at lines 116-133 as defense-in-depth until concurrent startup has been exercised.

#### Change 2: `scripts/crdb-init.sql` (new file)

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
-- instead of unique_rowid(). Also prepended by migrator.go at runtime,
-- but setting here ensures manual cockroach sql sessions also behave.
SET serial_normalization = 'sql_sequence';

-- === DEV-ONLY: Performance tuning for local development ===
-- These speed up migration re-runs by reducing GC and stats overhead.
-- They are NOT required for correctness. Safe to skip on Basic tier.
SET CLUSTER SETTING jobs.registry.interval.gc = '30s';
SET CLUSTER SETTING jobs.retention_time = '15s';
SET CLUSTER SETTING sql.stats.automatic_collection.enabled = false;
```

**Notes:**
- `kv.range_merge.queue_interval` REMOVED — nonexistent in v26.2 (confirmed by 42P02 from live probe)
- `serial_normalization` is a session variable — `SET` without `CLUSTER SETTING`
- Sections clearly separated: required vs dev-only
- Explicitly documented as idempotent

#### Change 3: `Taskfile.yml` — Add `crdb:init` target, update `crdb:reset`

**Add after `crdb:reset` (around line 294):**

```yaml
  crdb:init:
    desc: Apply cluster settings to local CockroachDB (idempotent, safe to rerun)
    cmds:
      - |
        echo "=== Applying local CockroachDB cluster settings ==="
        cockroach sql --url "postgresql://root@localhost:26257/bchat?sslmode=disable" \
          < scripts/crdb-init.sql
        echo "=== Cluster settings applied ==="
```

**Update `crdb:reset` to chain `crdb:init`:**

```yaml
  crdb:reset:
    desc: Wipe and restart local CockroachDB compose cluster (A1 state)
    cmds:
      - docker compose -f scripts/docker-compose.cockroach.yml down -v
      - docker compose -f scripts/docker-compose.cockroach.yml up -d
      - task: crdb:init
```

---

### Execution Sequence (Gate 0)

```
Gate 0: Full local E2E validation
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

task crdb:reset          # wipe + start + init (chained)
    ↓
task build:backend:cockroach
    ↓
task crdb:migrate        # boot app, applies LATEST.sql
    ↓
task crdb:verify         # P1-P6 schema checks
    ↓
verify-production.sh     # full data path: auth → onboard → KB → reindex → RAG search
    ↓
restart app              # prove startup idempotency
    ↓
verify-production.sh     # re-run after restart
    ↓
concurrent startup test  # start two replicas, verify no vector index errors
```

### Environment Setup

```bash
export COCKROACH_DSN="postgresql://root@localhost:26257/bchat?sslmode=disable"
export BCHAT_ALLOW_DB_RESET=1
export RAG_PIPELINE_ENABLED=true
export LANCEDB_STORAGE_PROVIDER=cockroach
export TICKET_EMBEDDING_ENABLED=true
export BCHAT_URL=http://localhost:5230
export BCHAT_USER=admin
export BCHAT_PASS=memos
```

### Expected Outcomes

| Step | Expected | What Failure Means |
|------|----------|-------------------|
| `crdb:reset` | Container starts + settings applied | Container healthcheck or DSN issue |
| `crdb:migrate` | 57 tables created | LATEST.sql has CockroachDB-incompatible syntax |
| `crdb:verify` P1-P6 | All PASS | Schema/sequence/vector mismatch |
| `verify-production.sh` [1/7] | healthz 200 | App didn't start or wrong port |
| `verify-production.sh` [2-3/7] | Auth + tenant select | JWT/auth flow broken |
| `verify-production.sh` [4/7] | Tenant onboarded | Agent tenant CRUD broken |
| `verify-production.sh` [5/7] | KB + reindexed | RAG pipeline + embeddings broken |
| `verify-production.sh` [6/7] | RAG search > 0 results | Vector search or embeddings broken |
| `verify-production.sh` [7/7] | Cleanup | Tenant deletion broken |
| Restart + re-verify | Same results | Migration not idempotent |
| Concurrent startup | No duplicate-index errors | `IF NOT EXISTS` not working as expected |

---

## Part 4: Remaining Adversarial Questions (Answered)

### Q5: Does `serial_normalization` persist across the migration batch?

**ANSWERED BY CODE INSPECTION.** `migrator.go:212-213`:
```go
stmt := "SET serial_normalization = 'sql_sequence';\n" + string(bytes)
if _, err := s.driver.GetDB().ExecContext(ctx, stmt); err != nil {
```
The SET + entire LATEST.sql is ONE string, ONE `ExecContext` call. Go's `database/sql` checks out ONE connection for the call. Session variable persists. Code comment at line 209: "The SET + whole file is one statement (P0-verified)." **Not a bug.**

### Q2: Does `Validate()` run on every boot?

**ANSWERED BY CODE INSPECTION.** `service.go:1273-1274`:
```go
if shouldValidateReindex(resume, existingCheckpoint) {
    if err := s.vectorDB.Validate(ctx); err != nil {
```
Called during reindex operations, not on every boot. `CREATE VECTOR INDEX IF NOT EXISTS` runs only when reindexing. **Safe.**

### Q7: Does `verify-production.sh` work with HTTP localhost?

**ANSWERED BY CODE INSPECTION.** `verify-production.sh:15`:
```bash
URL="${BCHAT_URL:-https://bchat-crdb.fly.dev}"
```
Uses env var with fallback. When `BCHAT_URL=http://localhost:5230` is set, uses HTTP. **Works.**

---

## Part 5: Deferred Items (Post-Hackathon)

| Item | Rationale |
|------|-----------|
| pgtype codec registration (Option A) | `simple_protocol` works; codec has wiring bug; not a correctness issue |
| `crdb:up:fast` (in-memory mode) | Nice for speed, not blocking E2E |
| Fix-forward migration test fixture | Only needed if migration strategy changes |
| Vector index backfill benchmark (10k+ rows) | Cloud-only concern |
| TLS/SCRAM auth parity | Cloud-only concern |
| Connection pool tuning for Basic tier | Cloud-only concern |

---

## Part 6: Files Summary

| File | Action | Lines Changed |
|------|--------|--------------|
| `server/router/api/v1/agent/vectordb_cockroach.go:112` | Edit: add `IF NOT EXISTS` | 1 line |
| `scripts/crdb-init.sql` | **New file** | ~25 lines |
| `Taskfile.yml:~294` | Edit: add `crdb:init` target, update `crdb:reset` | ~12 lines |

**Total: 3 files, ~38 lines of changes.**

---

## Part 7: Adversarial Review Prompt (For Next Reviewer)

Before implementation, challenge this plan with:

1. **Does `IF NOT EXISTS` on `CREATE VECTOR INDEX` handle true concurrency?** Two replicas starting simultaneously on a fresh empty table — does the second one silently no-op, or does it hit a schema-change lease conflict (retryable error)? Test, don't reason.

2. **Is the `crdb:init` exit code actually checked?** The plan says "check `$?`" but the Taskfile YAML doesn't show the implementation. Make sure the task fails if a required setting fails to apply.

3. **Does `crdb:reset` → `crdb:init` chaining work in Taskfile v3?** Verify that `task: crdb:init` as a dependency of `crdb:reset` runs AFTER the docker compose up, not in parallel.

4. **Is `simple_protocol` truly harmless on v26.2.1?** It disables prepared statement caching. For vector search queries executed frequently, is there a measurable latency cost? Benchmark if critical.

5. **What happens if the vector index backfill is interrupted mid-flight on the cloud cluster?** The 44 rows with 0 embeddings and a potentially stuck/failed index — how do you verify index health before deploying? `SHOW JOBS` filtered to schema-change jobs on `agent_vectors`.
