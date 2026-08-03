# Bug 058 — Plan: Local CockroachDB Full E2E Testing

**Date:** 2026-08-03
**Status:** PLAN — Do Not Code Until Approved
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

**Source:** Code inspection of `vectordb_cockroach.go` (build tag `cockroach`), `LANCEdb` implementations.

The `agent_vectors` table:
- Is behind `//go:build cockroach` — only compiled for CockroachDB builds
- Uses `VECTOR(1536)` — a CockroachDB-native type (not standard PostgreSQL)
- Uses `CREATE VECTOR INDEX` — CockroachDB-native syntax (not `USING hnsw`)
- Is created by `Validate()` at service startup, not by migrations
- LanceDB has its own separate implementation

LATEST.sql at `store/migration/cockroach/LATEST.sql` contains 57 tables that are shared infrastructure (tenants, sessions, messages, etc.). `agent_vectors` is RAG storage-layer DDL that differs per backend. Moving it to LATEST.sql would:
- Break SQLite/Postgres builds (VECTOR type doesn't exist)
- Couple storage-layer DDL to schema migrations
- Remove the ability to have backend-specific init logic

**Conclusion:** Runtime creation in `Validate()` is the correct architectural choice. No migration change needed.

---

## Part 2: The Plan

### Scope: 3 Files, 1 New Script, 1 Task Target

This is the minimal path to prove bchat works end-to-end against local CockroachDB.

#### Change 1: `server/router/api/v1/agent/vectordb_cockroach.go` (line 112)

Add `IF NOT EXISTS` to the `CREATE VECTOR INDEX` statement.

**Current:**
```go
CREATE VECTOR INDEX idx_agent_vectors_embedding
ON agent_vectors (embedding)
```

**New:**
```go
CREATE VECTOR INDEX IF NOT EXISTS idx_agent_vectors_embedding
ON agent_vectors (embedding)
```

**Why:** Prevents duplicate-index error on concurrent replica startup. Confirmed supported by CockroachDB v26.1/v26.2 docs and live probe.

**Risk:** Zero. `IF NOT EXISTS` is a standard SQL clause. If the index exists, it's a no-op. If it doesn't, it creates it.

#### Change 2: `scripts/crdb-init.sql` (new file)

```sql
-- Local CockroachDB initialization script
-- Usage: cockroach sql --url "postgresql://root@localhost:26257/bchat?sslmode=disable" < scripts/crdb-init.sql
--
-- These settings are:
--   - Required for vector index support (feature.vector_index.enabled)
--   - Recommended for correct SERIAL behavior (serial_normalization)
--   - Local-only performance tuning (jobs.*, sql.stats.*)

-- Required: enable vector index creation
SET CLUSTER SETTING feature.vector_index.enabled = true;

-- Session-level: ensure SERIAL columns use sequences (not unique_rowid)
SET serial_normalization = 'sql_sequence';

-- Local dev performance tuning (fast GC, disable stats collection)
-- These speed up migration re-runs during development
SET CLUSTER SETTING jobs.registry.interval.gc = '30s';
SET CLUSTER SETTING jobs.retention_time = '15s';
SET CLUSTER SETTING sql.stats.automatic_collection.enabled = false;
```

**Notes:**
- `kv.range_merge.queue_interval` is REMOVED — nonexistent setting in v26.2 (confirmed by live probe returning 42P02)
- `serial_normalization` is a session variable, not a cluster setting — `SET` without `CLUSTER SETTING`
- `jobs.*` and `sql.stats.*` are local-dev-only tuning. They work on local `--insecure` root. They also worked on Basic tier (evidence Task A) but are not required there.

#### Change 3: `Taskfile.yml` — Add `crdb:init` Target

```yaml
crdb:init:
  desc: Apply cluster settings to local CockroachDB (run after crdb:up)
  cmds:
    - |
      echo "=== Applying local CockroachDB cluster settings ==="
      cockroach sql --url "postgresql://root@localhost:26257/bchat?sslmode=disable" \
        < scripts/crdb-init.sql
      echo "=== Cluster settings applied ==="
```

Place this after the existing `crdb:reset` target (around line 294).

---

### Execution Sequence

```bash
# 1. Start local CockroachDB single-node container
task crdb:up

# 2. Wait for healthcheck (~5s), then apply cluster settings
task crdb:init

# 3. Build backend with cockroach build tag
task build:backend:cockroach

# 4. Set environment variables
export COCKROACH_DSN="postgresql://root@localhost:26257/bchat?sslmode=disable"
export BCHAT_ALLOW_DB_RESET=1
export RAG_PIPELINE_ENABLED=true
export LANCEDB_STORAGE_PROVIDER=cockroach
export TICKET_EMBEDDING_ENABLED=true

# 5. Boot the app (applies migrations, creates agent_vectors + vector index)
task crdb:migrate

# 6. Run P1-P6 verification
task crdb:verify

# 7. Full E2E smoke test
export BCHAT_URL=http://localhost:5230
export BCHAT_USER=admin
export BCHAT_PASS=memos
bash scripts/verify-production.sh --keep
```

### Expected Outcomes

| Step | Expected | What Failure Means |
|------|----------|-------------------|
| `crdb:init` | All settings applied | Container not healthy or wrong DSN |
| `crdb:migrate` | 57 tables created | LATEST.sql has CockroachDB-incompatible syntax |
| `crdb:verify` P1-P6 | All PASS | Schema/sequence/vector mismatch |
| `verify-production.sh` [1/7] | healthz 200 | App didn't start or wrong port |
| `verify-production.sh` [2-3/7] | Auth + tenant select | JWT/auth flow broken |
| `verify-production.sh` [4/7] | Tenant onboarded | Agent tenant CRUD broken |
| `verify-production.sh` [5/7] | KB + reindexed | RAG pipeline + embeddings broken |
| `verify-production.sh` [6/7] | RAG search > 0 results | Vector search or embeddings broken |
| `verify-production.sh` [7/7] | Cleanup | Tenant deletion broken |

---

## Part 3: Adversarial Review Prompt

Before implementing, this plan should be challenged with the following questions:

### Q1: Is `IF NOT EXISTS` on `CREATE VECTOR INDEX` actually safe for concurrent startup?

The plan assumes two app replicas starting simultaneously will both execute `CREATE VECTOR INDEX IF NOT EXISTS ...` and the second one will silently succeed (no-op). Is this correct? Or does CockroachDB still attempt a second backfill even with `IF NOT EXISTS`? What does the docs say about idempotency of `CREATE VECTOR INDEX IF NOT EXISTS` on an already-indexed table?

### Q2: Does `Validate()` run on every service boot, or only on first start?

If `Validate()` runs on every boot, the `IF NOT EXISTS` is called on every restart. Is there a performance cost to calling `CREATE VECTOR INDEX IF NOT EXISTS` on an already-indexed table on every boot? Should there be a check-first-execute-only-if-needed pattern instead?

### Q3: Is the `simple_protocol` workaround actually harmless on v26.2.1?

The docs confirm the binary format bug is fixed in v25.3+. Since both local and cloud run v26.2.1, `simple_protocol` is technically unnecessary. But does `simple_protocol` have a measurable performance cost? Does it disable prepared statement caching globally? If so, is there a measurable latency impact on vector search queries that execute frequently?

### Q4: Can the local init SQL script fail silently?

The plan's `crdb:init` task pipes SQL to `cockroach sql`. If one setting fails (e.g., `jobs.registry.interval.gc` is removed in a future version), does `cockroach sql` exit non-zero? Should the task use `|| true` or should each statement be checked individually?

### Q5: Is `serial_normalization = 'sql_sequence'` applied before LATEST.sql executes?

The migrator prepends `SET serial_normalization = 'sql_sequence';` before each migration batch. The init script also sets it. But what about the initial LATEST.sql application in `preMigrate()`? Does the session-level setting persist across the entire `ExecContext` call that runs LATEST.sql? Or could it reset between statements?

### Q6: What happens if `agent_vectors` already exists with data (44 rows on cloud)?

The cloud cluster has 44 rows with 0 embeddings. The `IF NOT EXISTS` on `CREATE TABLE` and `CREATE INDEX` means they'll be no-ops. But the `CREATE VECTOR INDEX IF NOT EXISTS` will also be a no-op (index already exists from the killed migration). Is the index actually valid on the cloud cluster, or was it left in a partially-created state from the killed migration? How do you verify index health?

### Q7: Does `verify-production.sh` work against localhost?

The script uses `curl` with cookie jars. The app runs on `localhost:5230`. Are there any CORS or origin restrictions that would block the curl calls? Does the script assume HTTPS or does it work with HTTP?

---

## Part 4: Deferred Items (Post-Hackathon)

| Item | Rationale |
|------|-----------|
| pgtype codec registration (Option A) | `simple_protocol` works; codec has wiring bug; not a correctness issue |
| `crdb:up:fast` (in-memory mode) | Nice for speed, not blocking E2E |
| Fix-forward migration test fixture | Only needed if migration strategy changes |
| Vector index backfill benchmark (10k+ rows) | Cloud-only concern |
| TLS/SCRAM auth parity | Cloud-only concern |
| Connection pool tuning for Basic tier | Cloud-only concern |

---

## Part 5: Files Summary

| File | Action | Lines Changed |
|------|--------|--------------|
| `server/router/api/v1/agent/vectordb_cockroach.go:112` | Edit: add `IF NOT EXISTS` | 1 line |
| `scripts/crdb-init.sql` | **New file** | ~15 lines |
| `Taskfile.yml:~294` | Edit: add `crdb:init` target | ~8 lines |

**Total: 3 files, ~24 lines of changes.**
