# Bug 057 — CockroachDB Deployment Profile: Implementation Documentation

**Date:** 2026-08-02
**Status:** Implemented — steps 1–10, 12 of `pre_code.md` complete; all local gates green (P0, e2e, migration parity, compat scanner, builds). Step 11 (live 2-region cluster deploy + rollback demo) pending environment authentication (§8).
**Plan:** `bugs/057/pre_code.md` (8 pre_code_review findings applied)
**Files Modified:** 11 modified, 12 added (source/test + deploy artifacts + docs)

---

## 1. Implementation Summary

| # | Area | Deliverable | Status |
|---|------|-------------|--------|
| 1 | Driver wiring | `case "cockroach"` in factory (`store/db/db.go:25`), profile DSN check (`internal/profile/profile.go:104-107`), `store/db/postgres/cockroach.go`, `file_env "COCKROACH_DSN"` (`scripts/entrypoint.sh:38`) | DONE |
| 2 | Migration mirror | `store/migration/cockroach/LATEST.sql` (1030 lines): 147 `IF NOT EXISTS`, 19 `::BIGINT` casts, 3 UNIQUE→PRIMARY KEY conversions; 0.35 inert mirror | DONE |
| 3 | Migrator branch | cockroach arm in `preMigrate` + `Migrate` — no Begin/Commit, `SET serial_normalization` prefix, whole-file exec, inlined tolerance strings (`store/migrator.go`, 71+/22-) | DONE |
| 4 | P0 gate | `store/cockroach_p0_test.go` (`//go:build cockroach && integration`) — PASS 2.48s | DONE |
| 5 | Retry wrappers | `execTx` dispatcher (`store/db/postgres/tx.go`), 8 sites converted (`crdb.ExecuteTx` SAVEPOINT protocol on cockroach; BeginTx/Commit elsewhere) | DONE |
| 6 | Validation scripts | `validate-parity.sh` Check 2b (cockroach↔postgres names); new `validate-cockroach-compat.sh` (FORBIDDEN/REVIEW patterns) — both exit 0 | DONE |
| 7 | Taskfile + env fixes | 4× `VECTOR_DB_PROVIDER`→`LANCEDB_STORAGE_PROVIDER` swaps; crdb:up/down/reset/migrate/verify/init/check verbs; `crdb:cluster:create` → `deps: [crdb:init]` | DONE |
| 8 | Fly artifacts | `fly_cockroach.toml`, `Dockerfile.cockroach.fly`, `scripts/fly-cockroach-secrets.sh` | DONE |
| 9 | E2E gate | `store/test/cockroach_migrate_test.go` (TestCockroachMigrateEndToEnd 110.71s PASS, TestCockroachMigrateBootIdempotency PASS) | DONE |
| 10 | `scripts/crdb-deploy.sh` (§4.10) + `scripts/verify-production.sh` (§6.3) + `fly_pg-rollback.toml` (§6.4) | 74-line chain runner (≤100 limit); smoke verified through stage 4/7; stage 5 fails cleanly without fly auth | DONE |
| 11 | `crdb:init` console flow + live deploy/rollback (§5, §6.3, §6.4) | **PENDING LIVE** — console-first cluster creation + ccloud/fly auth not available in this environment | PENDING |
| 12 | README (`### Deployment Profiles`) + `docs/docs_flyio_cockroach_deploy.md` | DONE | DONE |

**Key deviations from plan:** (1) P0 — 147 `IF NOT EXISTS` clauses added to the mirror (postgres's tolerance-based re-runs are unsafe under pgx simple protocol, which aborts the batch at first error); (2) P1 — 3 UNIQUE-only tables converted to PRIMARY KEY (Cockroach synthesizes `unique_rowid()` int64 defaults otherwise, breaking the int32 ID invariant); (3) build tag confirmed `-tags "cockroach"` only — the `rag` tag is LanceDB-only and would require CGO for nothing. See §5.

---

## 2. Architecture

### 2.1 Data Flow

```
main.go (viper: profile.DSN from MEMOS_DSN/COCKROACH_DSN, :60, :258-259)
  → internal/profile (driver validation; cockroach requires DSN, :104-107)
  → store/db/db.go factory (:18-27; case "cockroach" :25)
  → store/db/postgres.NewCockroachDB (postgres.go:27-45 pattern; DSN appends
    default_query_exec_mode=simple_protocol)
  → migrator.Migrate (store/migrator.go — cockroach arm, §3.3)
  → vectordb factory (vectordb.go:280-294; case "cockroach" :288)
  → CockroachVectorDB (vectordb_cockroach.go — ZERO changes, §3.5)
```

### 2.2 Build Tag Matrix

| Tag set | Files compiled | Use case |
|---------|----------------|----------|
| (none) | `vectordb_nolance.go` (`!rag`), `vectordb_nocockroach.go` (`!cockroach`) | SQLite/LanceDB-less dev |
| `-tags rag` | `vectordb_lance.go`, `vectordb_pool.go` — **requires CGO + liblancedb_go.so** | Local/S3 LanceDB |
| `-tags cockroach` | `vectordb_cockroach.go` — **pure Go, no CGO** | Cockroach deployment |

Verified: `go build -tags "cockroach"` produces a full standalone binary (~87 MB) with zero CGO. The `rag` tag is NOT needed for the Cockroach path (plan §4.0 correction) — the nolance stub errors only for LanceDB providers, never for `LANCEDB_STORAGE_PROVIDER=cockroach`.

---

## 3. Per-Area Implementation

### 3.1 Driver Wiring

- **`store/db/db.go:25`** — factory `case "cockroach":` → `NewCockroachDB`; default error at :26.
- **`internal/profile/profile.go:104-107`** — mirror of the postgres branch: `--driver=cockroach` with empty DSN → fail-fast `"cockroach driver requires DSN or COCKROACH_DSN environment variable"`.
- **`store/db/postgres/cockroach.go`** (new) — thin constructor cloned from `postgres.NewDB` (`postgres.go:22-54`): pgx driver, `default_query_exec_mode=simple_protocol` appended to DSN (:33 pattern), pool (:42-45), 60s PingContext (:47-51).
- **`scripts/entrypoint.sh:38`** — `file_env "COCKROACH_DSN"` alongside `MEMOS_DSN` (exclusive per-driver; main.go's `SetEnvPrefix("memos")`/`AutomaticEnv` at :258-259 needs no change).

### 3.2 Migration Mirror (`store/migration/cockroach/`)

- **`LATEST.sql`** — byte-identical schema to `store/migration/postgres/LATEST.sql` modulo exactly:
  - **15 casts** — `DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT` where postgres relies on implicit coercion (LATEST.sql lines 4, 17, 18, 43, 44, 80, 81, 98, 116, 126, 127, 138, 217, 236, 751; 4 casts pre-existing at 988, 989, 1009, 1026).
  - **147 `IF NOT EXISTS`** — all 57 `CREATE TABLE` + 90 `CREATE INDEX`/`CREATE UNIQUE INDEX` (P0 deviation, §5).
  - **3 UNIQUE→PRIMARY KEY conversions** — `user_setting(user_id, key)` (:31-35), `memo_organizer(memo_id, user_id)` (:57-62), `memo_relation(memo_id, related_memo_id, type)` (:65-72). Semantically identical (P1 deviation, §5).
- Verified: table-name identity check `^CREATE TABLE (IF NOT EXISTS )?[a-z_"]+` → `TABLES_IDENTICAL`; index-name identity likewise; zero `unique_rowid` in the mirror.
- **`0.35/00__tickets_add_internal_notes.sql`** — inert mirror of the postgres 0.35 file (1-line `ALTER TABLE`); required so version-walk parity holds.
- **Why `IF NOT EXISTS` here:** postgres's LATEST.sql relies on `execute()`'s tolerance strings + pgx simple-protocol batch abort (first error aborts the whole batch, so nothing is half-applied). Cockroach executes the same multi-statement round trip; the plan decided idempotency must be *structural* (explicit clauses) rather than tolerance-based.

### 3.3 Migrator Branch (`store/migrator.go`, 71+/22-)

Both arms share zero new shared code — each is a driver-conditional branch:

- **`preMigrate` (legacy pre-0.31 path):** cockroach branch executes `SET serial_normalization = 'sql_sequence';\n` + whole file content via `s.driver.GetDB().ExecContext(ctx, stmt)` — **no `Begin()`/`Commit()`** (Cockroach does not support DDL in explicit transactions; `autocommit_before_ddl` NOTICE). Tolerance strings inlined in the cockroach arm: `"duplicate column"`, `"already exists"`, `"column already exists"` (Cockroach emits `relation "x" already exists`, 42P07 — matches).
- **`Migrate` (production path):** restructured to a conditional-tx shape — `var tx *sql.Tx`; `BeginTx`/`Commit` only when `driver != "cockroach"`; the cockroach arm runs `ExecContext` with the `SET` prefix and inlined tolerance. Rollback via `defer` when a tx exists.
- Version bookkeeping (`migration_history`, `GetCurrentSchemaVersion` :257-281, version-consistency warn-only :283-298) unchanged — shared for all drivers.

### 3.4 Retry Wrappers (`execTx`, §4.5)

**`store/db/postgres/tx.go`** (new, 35 lines) — single dispatcher:

```go
// Cockroach: crdb.ExecuteTx (SAVEPOINT cockroach_restart protocol) retries
// 40001 internally; other drivers: BeginTx/Commit with defer Rollback.
func execTx(ctx context.Context, d *DB, fn func(tx *sql.Tx) error) error {
    if d.profile.Driver == "cockroach" {
        return crdb.ExecuteTx(ctx, d.db, nil, fn)   // tx.go:24
    }
    tx, err := d.db.BeginTx(ctx, nil)
    // ... Commit; defer Rollback
}
```

Contract: the closure **must not** commit/rollback mid-body (execTx owns the tx lifecycle). All 8 sites converted, each with a mandatory `// retry-safe:` comment justifying why a 40001 retry is safe:

| Site | Guard type | Location |
|------|-----------|----------|
| `createBridgeHandoffAttempt` | status-guarded (active-handoff COUNT check) | bridge.go:112 |
| `CreateBridgeHandoffReplyIfActive` | status-guarded (FOR UPDATE re-read) | bridge.go:354 |
| `CreateBridgeHandoffReplyAndOutboxIfActive` | status-guarded (FOR UPDATE re-read) | bridge.go:488 |
| `CreateBridgeHandoffReply` (post-commit lookup) | — (outside closure) | bridge.go:705 |
| `ClaimPendingBridgeReplyOutbox` | token-guarded (UPDATE ... WHERE claim_token) | bridge.go:790 |
| `CompleteClaimedBridgeReplyOutbox` | token-guarded | bridge.go:876 |
| `FailClaimedBridgeReplyOutbox` | status-guarded (UPDATE ... WHERE status) | bridge.go:959 |
| `CreateAgentMessages` | append-only (inserts never conflict on retry) | agent.go:772 |

Also: the two `d.db.Conn(ctx)` pinned connections were **dropped** (audited — no use outside the tx closure). Precedent: existing 3-attempt loop in `CreateBridgeHandoff` (bridge.go:94-109). Diff: bridge.go 554+/607-, agent.go 28 lines.

### 3.5 Vector Store — Zero Changes

`server/router/api/v1/agent/vectordb_cockroach.go` was verified, not modified: DSN required (:26-31), simple_protocol (:50-58), 42P07/0A000 tolerance (:120-135). Env reads already correct (`LANCEDB_STORAGE_PROVIDER` vectordb.go:121, `COCKROACH_DSN` vectordb.go:131, provider switch :288).

### 3.6 Validation Scripts

- **`scripts/validate-parity.sh`** — extended: `COCKROACH_DIR`/`COCKROACH_DIVERGENCES` (:76, 0.19–0.34 inert-mirror-only reasons); `check_file_list_parity` gains a divergence-list arg (:183-184); **Check 2b** (:273, :279) compares cockroach mirror table/index names vs postgres (postgres tables/indexes as source of truth) — FAIL bumps SCHEMA_ISSUES. Exit 0 on repo.
- **`scripts/validate-cockroach-compat.sh`** (new, executable) — scans `store/migration/cockroach/`:
  - **FORBIDDEN** (exit 1): `CREATE EXTENSION`, `NOTIFY`/`LISTEN`, `pg_advisory_lock` family, `CREATE DOMAIN`, range types (`int4range` etc.), `macaddr`/`MONEY`, `CREATE TRIGGER`, `DEFERRABLE`, `DROP PRIMARY KEY`, `DO $$` blocks.
  - **REVIEW-REQUIRED** (exit 2): `ALTER TYPE`, `UPDATE ... FROM`, `^COPY` — must carry a `-- cockroach-compat:` annotation.
  - Exit 0 clean; fixture-tested (extension → 1, COPY → 2). Exit 0 on repo.

### 3.7 Taskfile + Env Fixes

- **Env swap** `VECTOR_DB_PROVIDER` (vestigial) → `LANCEDB_STORAGE_PROVIDER=cockroach`: Taskfile.yml:241 (`run:cockroach`), :348 (`crdb:docker:run`), `deploy/ccloud/setup.sh:45`, `deploy/ecs/task-definition.json:26`. Repo-wide grep clean (only historical bug docs remain).
- **Verbs:** `crdb:check` (:253, now = LANCEDB check + compat scanner + `deps: [validate:parity]`), `crdb:up` (:277), `crdb:down` (:282), `crdb:reset` (:287), `crdb:migrate` (:293, boot memos with `--driver=cockroach`), `crdb:verify` (:303, runs P0 + migrate e2e), `crdb:init` (:313, console-first 2-region bootstrap).
- **`crdb:cluster:create`** → `deps: [crdb:init]` (Basic is console-only for multi-region; single-region CLI superseded).

### 3.8 Fly Artifacts

| Setting | fly_pg.toml (Neon) | fly_cockroach.toml |
|---------|--------------------|--------------------|
| App | `bchat-pg` | `bchat-crdb` |
| Dockerfile | `Dockerfile.pg.fly` | `Dockerfile.cockroach.fly` |
| `MEMOS_DRIVER` | `postgres` | `cockroach` |
| DSN secret | `DATABASE_URL` | `COCKROACH_DSN` (Fly secret; also `OPENROUTER_API_KEY`, `ENCRYPTION_MASTER_KEY`) |
| `LANCEDB_STORAGE_PROVIDER` | `s3` + Tigris creds | `cockroach` — **no AWS/Tigris secrets at all** |
| Reindex | `RAG_STARTUP_REINDEX_DISABLED='true'` | `FORCE_REINDEX_ON_STARTUP='true'` initial A1 only (then disable) |
| Health check / VM | `/healthz`, 1024mb | identical |

- **`Dockerfile.cockroach.fly`** — clone of Dockerfile.pg.fly, delta: `go build -tags cockroach` (NOT `rag` — §2.2), **no liblancedb_go.so copy, no CGO env, no gcc build deps**; runtime stage identical (Ubuntu 24.04, gosu, non-root memos user, supercronic crontab, widget bundle).
- **`scripts/fly-cockroach-secrets.sh`** — clone of fly-pg-secrets.sh (interactive, `set -e`, `fly auth whoami` guard): sets `COCKROACH_DSN`, `OPENROUTER_API_KEY`, `ENCRYPTION_MASTER_KEY` (auto-generated) on `bchat-crdb`; no S3/AWS steps; validates `sslmode=` presence (CockroachDB Cloud requires `sslmode=require`).

### 3.9 Deploy Chain + Rollback (§4.10, §6.3, §6.4)

- **`scripts/crdb-deploy.sh`** (74 lines — under the 100-line ceiling): `build:backend:cockroach` → `validate-parity.sh` → `validate-cockroach-compat.sh` → optional `--experiments` (P4/P5) → `fly -a bchat-crdb deploy -c fly_cockroach.toml` → `/healthz` poll (15s grace, 24×5s) → `task crdb:verify` → `task verify:production`. Each stage logged to `build/crdb-deploy.log`; re-runnable; any failure exits 1 with a log pointer. Verified locally through stage 4/7; stage 5 fails cleanly without fly auth.
- **`scripts/verify-production.sh`** (§6.3 smoke): healthz → REST signin → `/auth/tenants` + `/auth/select-tenant` (cookie jar) → onboard `verify-<ts>` tenant → KB import (multipart `file`/`file_type=kb`/`audience_type=internal`) → reindex → RAG search (polls up to 60s for ≥1 hit) → destroy test tenant (`--keep` disables). Env-driven: `BCHAT_URL`/`BCHAT_USER`/`BCHAT_PASS`. Bridge handoff cycle is exercised by the store-level suites instead (the widget bridge endpoints require per-tenant HMAC secrets, not admin auth).
- **`fly_pg-rollback.toml`** + `task rollback:postgres` (§6.4): sets `DATABASE_URL` secret, unsets `COCKROACH_DSN`, redeploys `bchat-crdb` with the Neon profile, re-runs `verify:production`. No schema downgrade; re-cutover is a plain forward migration.
- **`task crdb:verify`** extended with the §6.2 seven checks, env-gated on `COCKROACH_DSN` + a host `cockroach` binary (skips gracefully): `SELECT 1`, `version()`=Cockroach, `migration_history`=1 row, `nextval()` defaults in `agent_tenants`, `feature.vector_index.enabled`=true, `agent_vectors` indexed, `/healthz` 200.
- **`task crdb:harden`** (§6.5): allocate Fly egress IP → allowlist `<ip>/32` → remove `0.0.0.0/0` → connectivity verify → auto-revert on breakage.

---

## 4. Verification Evidence (2026-08-02)

| Gate | Command | Result |
|------|---------|--------|
| P0 | `go test -tags "cockroach integration" ./store/ -run TestCockroachP0 -v` | **PASS 2.48s** (SET + whole-file exec; nextval on serial tables; comprehensive information_schema scan for `unique_rowid` → empty; idempotent re-run) |
| E2E | `go test -tags "cockroach integration" ./store/test/ -run "TestCockroachMigrate" -v` | **PASS** — TestCockroachMigrateEndToEnd 110.71s (A1 full LATEST apply; migration_history/version/workspace checks; nextval < MaxInt32 on `agent_tenants_id_seq`; unique_rowid-absence scan; A2 re-run no-op; A3 history-wipe recovery; tenant CRUD int32-safe); TestCockroachMigrateBootIdempotency (42P07 "already exists") |
| Postgres regression | `DRIVER=postgres DSN="postgresql://bchat:bchat@localhost:5432/bchat?sslmode=disable" go test ./store/test/ -v` | **Zero new regressions** — 26 failures on HEAD identical to `git stash` base (`/tmp/opencode/base_failures.txt` vs `head_failures.txt`, diff empty). All pre-existing on master (22 bridge/outbox + 4 auth, e.g. `unused argument: 0` pattern) |
| SQLite suite | `go test ./store/test/` | ok (cached) |
| Agent suite (cockroach tag) | `task crdb:test` | PASS (compiles + green under `-tags "cockroach"`) |
| Deploy chain (local stages) | `bash scripts/crdb-deploy.sh` | stages 1–4 PASS; stage 5 fails cleanly (no fly auth) with log pointer |
| Parity | `bash scripts/validate-parity.sh` | exit 0 (Check 1 PASS, Check 2 PASS, Check 2b PASS) |
| Compat | `bash scripts/validate-cockroach-compat.sh` | exit 0 |
| Build/vet | `go build -tags "cockroach" ./...`, `go vet`, gofmt | clean; standalone 87 MB binary, no CGO; gofmt-unclean files all pre-existing on HEAD (verified) |
| Script syntax | `bash -n` on all new/modified scripts | clean |

**Runtime facts learned:** first full LATEST apply on the compose cluster (v25.2.21) ~100s (background DDL jobs), re-applies ~2s; `DROP SCHEMA public` fails 3F000 → reset uses `string_agg(format('DROP TABLE IF EXISTS %I CASCADE'))`; `ALTER SCHEMA public OWNER TO bchat_user` required for table drops; insecure clusters refuse passwords (28P01) — `bchat_user` is password-less.

---

## 5. Deviations from pre_code.md

| # | Deviation | Rationale | Recorded |
|---|-----------|-----------|----------|
| P0 | 147 `IF NOT EXISTS` clauses in cockroach LATEST.sql (postgres relies on error tolerance) | pgx simple protocol aborts the whole batch at first error — tolerance-based re-runs are fragile; structural idempotency decided. Mirror diff vs postgres is exactly: 15 casts + 147 IF NOT EXISTS + 3 PK conversions | pre_code.md §4.1 step 3 |
| P1 | 3 UNIQUE-only tables → PRIMARY KEY | Cockroach synthesizes `unique_rowid()` int64 defaults for UNIQUE-only tables → breaks int32 ID invariant (`user_setting.rowid`, `memo_organizer.rowid`, `memo_relation.rowid`). Semantically identical constraints | pre_code.md §4.1 step 4 |
| — | Build tag `-tags "cockroach"` only (not `"cockroach rag"`) | `rag` gates LanceDB only; would drag in CGO + liblancedb_go.so for nothing | pre_code.md §4.0 |
| — | Earlier "table names identical" grep was vacuous | `^CREATE TABLE [a-z_]+` stopped matching after IF NOT EXISTS prefix; corrected pattern `^CREATE TABLE (IF NOT EXISTS )?` → TABLES_IDENTICAL | pre_code.md §4.1 step 4 |
| — | Line numbers shifted | bridge.go converted sites moved to :112/:354/:488/:705/:790/:876/:959, agent.go:772 (original plan cited :112-1003) | §3.4 above |
| — | `verify:production` smoke uses admin-API path instead of widget bridge HMAC cycle | Bridge handoff endpoints (`/api/v1/agent/:slug/bridge/*`) require per-tenant HMAC secrets, not admin auth; retry-wrapper-adjacent paths covered by store-level suites instead | §3.9 |

---

## 6. Files Modified

### Added
| File | Lines | Purpose |
|------|-------|---------|
| `store/migration/cockroach/LATEST.sql` | 1030 | Cockroach mirror (147 IF NOT EXISTS, 19 casts, 3 PK conversions) |
| `store/migration/cockroach/0.35/00__tickets_add_internal_notes.sql` | 1 | Inert mirror for version-walk parity |
| `store/db/postgres/cockroach.go` | ~50 | `NewCockroachDB` constructor |
| `store/db/postgres/tx.go` | 35 | `execTx` dispatcher |
| `store/cockroach_p0_test.go` | ~90 | P0 gate (`//go:build cockroach && integration`) |
| `store/test/cockroach_migrate_test.go` | ~150 | E2E + boot idempotency (`//go:build cockroach && integration`) |
| `scripts/validate-cockroach-compat.sh` | ~90 | FORBIDDEN/REVIEW scanner |
| `scripts/fly-cockroach-secrets.sh` | ~150 | Fly secrets setup |
| `scripts/crdb-deploy.sh` | 74 | Deploy chain runner (§4.10; ≤100-line ceiling) |
| `scripts/verify-production.sh` | ~130 | App-first smoke (§6.3) |
| `fly_cockroach.toml` | 54 | Fly app config |
| `fly_pg-rollback.toml` | 54 | Rollback profile (app `bchat-crdb`, Neon backend) |
| `Dockerfile.cockroach.fly` | ~120 | Multi-stage, `-tags cockroach`, no CGO |
| `docs/docs_flyio_cockroach_deploy.md` | ~110 | Deploy + demo runbook |

### Modified
| File | Diff | Change |
|------|------|--------|
| `store/db/postgres/bridge.go` | 554+/607- | 7 sites → `execTx`; 2 Conn pins dropped |
| `store/migrator.go` | 71+/22- | cockroach arms (preMigrate + Migrate) |
| `scripts/validate-parity.sh` | 86+/0- | COCKROACH_DIVERGENCES, Check 2b |
| `Taskfile.yml` | 120+/0- | crdb verbs, deploy/verify/rollback/harden verbs, §6.2 checks in crdb:verify, TODO comment |
| `store/db/postgres/agent.go` | 28+/0- | 1 site → `execTx` |
| `internal/profile/profile.go` | 9+/0- | cockroach driver branch |
| `scripts/entrypoint.sh` | 3+/0- | `file_env "COCKROACH_DSN"` |
| `store/db/db.go` | 2+/0- | factory case |
| `deploy/ccloud/setup.sh` | ~50 | env swap + console-first reframe (§4.8) |
| `deploy/ecs/task-definition.json` | 1+/1- | env swap (doc-only) |
| `README.md` | +45 | `### Deployment Profiles` (diagram, wording, parallel-apps note) |

**Unchanged (explicitly):** `vectordb_cockroach.go`, `main.go`, `go.mod` (cockroach-go/v2 v2.4.3 pre-existing), all Neon/Fly-pg files (`fly_pg.toml`, `Dockerfile.pg.fly`, `fly-pg-secrets.sh`).

---

## 7. Security & Tenant Isolation

- No new handlers or API endpoints were added — no new tenant-scoping surface.
- Tenant isolation in tests: e2e creates a tenant and round-trips it; nothing reads cross-tenant.
- Secrets: `COCKROACH_DSN` is a Fly secret, never baked into images; `.env` only local.
- The 8 tx conversions touch state-machine tables (bridge handoffs, outboxes, messages) — the `// retry-safe:` comments are the contract proving each is safe to retry under 40001.
- No PII exposure: driver error messages are wrapped with context (`fmt.Errorf`), tenant IDs never in messages (pre-existing convention maintained).

---

## 8. Known Limitations / Open Items

| Item | Severity | Status |
|------|----------|--------|
| **Step 11 (live):** `crdb:init` on a real 2-region Basic cluster (console-first), deploy:cockroach, verify:production, rollback:postgres demo | MEDIUM | PENDING — requires user console cluster creation + `ccloud` install/auth + `fly auth login`; all artifacts, tasks, and scripts are in place and chain-verified locally |
| `crdb:harden` egress-IP hardening (~$3.60/mo) | LOW | Ready; run only on explicit approval |
| A1 assumption: greenfield empty-DB start (no in-place upgrade path to Cockroach) | HIGH | By design — documented in plan §10 |
| First-apply latency ~100s (DDL backfill) | LOW | One-time; re-applies ~2s; reindex pre-traffic |
| `feature.vector_index.enabled` default false on v25.2/v25.3 | LOW | Cloud stable (v26.x) defaults true; `crdb:init`/setup.sh set it defensively |
| 26 pre-existing postgres `store/test` failures | LOW | Not caused by this work (identical on master); separate backlog |
| gofmt-unclean files (store/agent.go, store/bridge.go, store/ticket.go, store/user.go, store/rbac.go, store/db/sqlite/rbac.go, 4 store/test files) | LOW | Pre-existing on HEAD (verified via `git show HEAD:...`); untouched by this work |
| §6.3 bridge handoff cycle in `verify:production` smoke | LOW | Implemented as admin-API smoke (tenant→KB→reindex→RAG search); widget bridge HMAC cycle covered by store-level tests instead (endpoints require per-tenant HMAC secrets) |

---

## 9. Rollback Plan

1. Default path untouched: `MEMOS_DRIVER` unset → sqlite/postgres; `LANCEDB_STORAGE_PROVIDER` default `memory` (vectordb.go:121).
2. Neon/Fly-pg files byte-identical (`fly_pg.toml`, `Dockerfile.pg.fly`) — `fly -a bchat-pg deploy` unaffected.
3. Non-cockroach builds compile unchanged (`-tags "cockroach"` is additive).
4. Cockroach-specific files are isolated: `store/db/postgres/cockroach.go`, `tx.go`, migration mirror, tests — all behind build tags or driver conditions.
5. `execTx` on non-cockroach drivers preserves exact BeginTx/Commit/Rollback semantics (verified by the zero-regression diff, §4).

---

## 10. Adversarial Code Review Prompt

Copy and paste this prompt into Claude/Gemini for a thorough code review:

**PROMPT:**

```
You are performing an adversarial code review of a CockroachDB deployment profile
for a multi-tenant AI chat agent platform (bchat, Go + Echo + pgx). The change adds
a first-class `cockroach` driver: schema mirror, migrator branch, transaction retry
wrappers, validation scripts, and Fly.io artifacts. The plan document is
`bugs/057/pre_code.md`; this implementation doc is `bugs/057/code.md`.

Review the implementation critically against the plan, the codebase conventions
(AGENTS.md), and CockroachDB best practices. Verify every claim against source —
do not trust the documentation.

FILES TO REVIEW:
1. store/migration/cockroach/LATEST.sql (1030-line mirror) + 0.35/00__*.sql
2. store/migrator.go (cockroach arms in preMigrate + Migrate)
3. store/db/postgres/tx.go (execTx dispatcher)
4. store/db/postgres/bridge.go + store/db/postgres/agent.go (8 converted sites)
5. store/db/postgres/cockroach.go + store/db/db.go + internal/profile/profile.go + scripts/entrypoint.sh
6. store/cockroach_p0_test.go + store/test/cockroach_migrate_test.go (integration gates)
7. scripts/validate-parity.sh + scripts/validate-cockroach-compat.sh
8. Taskfile.yml (crdb:* verbs, deploy:cockroach) + deploy/ccloud/setup.sh + deploy/ecs/task-definition.json
9. fly_cockroach.toml + fly_pg-rollback.toml + Dockerfile.cockroach.fly + scripts/fly-cockroach-secrets.sh
10. scripts/crdb-deploy.sh (chain runner) + scripts/verify-production.sh (app smoke) — verify the ≤100-line thinness ceiling, stage exit handling, and env-driven behavior
11. server/router/api/v1/agent/vectordb_cockroach.go (verify: claimed UNCHANGED)

CONSTRAINTS TO VERIFY:
- lib/pq is BANNED (AGENTS.md). Only pgx/v5 + cockroach-go/v2 v2.4.3.
- DSN must append default_query_exec_mode=simple_protocol (pgx batch behavior: the
  whole multi-statement batch aborts at the first error).
- Cockroach does NOT support DDL inside explicit transactions; migrations must run
  without Begin/Commit and must prepend SET serial_normalization='sql_sequence'.
- Zero unique_rowid() defaults anywhere in the mirror (int32 ID invariant).
- Mirror must equal postgres modulo exactly: 15 ::BIGINT casts + 147 IF NOT EXISTS +
  3 UNIQUE→PRIMARY KEY conversions (user_setting, memo_organizer, memo_relation).
- execTx closures must never commit/rollback mid-body; every converted site must
  carry a // retry-safe: comment whose justification actually holds under 40001.
- No BeginTx may remain on the cockroach path in bridge.go/agent.go.
- Build tag is -tags "cockroach" only; no CGO, no liblancedb_go.so in Dockerfile.cockroach.fly.
- VECTOR_DB_PROVIDER is vestigial — only LANCEDB_STORAGE_PROVIDER may be set.

REVIEW CHECKLIST:
[C-1] CRITICAL: Any BeginTx/Commit left reachable when Driver == "cockroach"?
[C-2] CRITICAL: Any commit/rollback call inside an execTx closure?
[C-3] CRITICAL: Does the Migrate cockroach arm risk partial application on a
      mid-file error (no tx, tolerance strings)? What is the recovery path?
[C-4] CRITICAL: Any unique_rowid() default in the mirror? Any int64 column where
      postgres uses INTEGER that could overflow int32 (agent_tenants_id_seq nextval)?
[C-5] CRITICAL: Tenant isolation — do the new tests or wiring leak tenant scope,
      hardcode tenant IDs, or expose tenant IDs in error messages?
[C-6] CRITICAL: Are the 8 retry-safe justifications actually correct? Construct the
      interleaving that would make each site's claim false (e.g. two claims racing,
      a completed-claim replay, outbox double-settlement).
[H-1] HIGH: Is the postgres path of execTx byte-equivalent in behavior to the old
      BeginTx/Commit? Any defer-order or error-path regression?
[H-2] HIGH: Idempotency — re-run of the whole-file exec after partial success
      (e.g. 3F000 schema failure, network drop mid-batch). Does migration_history
      protect the re-run, and does the P0/e2e test actually prove it?
[H-3] HIGH: Do the integration tests fail loudly if IF NOT EXISTS is removed or a
      cast is dropped? Are assertions strong (not just "no error")?
[H-4] HIGH: Dockerfile.cockroach.fly — is COCKROACH_DSN the only DSN surface? Any
      baked secret or debug env? Does the non-root user work with entrypoint.sh?
[H-5] HIGH: validate-cockroach-compat.sh — are its patterns sufficient? Could a
      forbidden construct (e.g. CREATE TRIGGER, DEFERRABLE, range type) slip past
      via case sensitivity or whitespace?
[M-1] MEDIUM: parity script Check 2b false-positive/false-negative risk (name
      extraction regex on quoted/IF NOT EXISTS tables, index names).
[M-2] MEDIUM: Are all 147 IF NOT EXISTS clauses present and correctly placed (no
      double-prefix, none missing)? Count independently.
[M-3] MEDIUM: Fly health check (grace 15s) vs ~100s first migration apply — will
      Fly kill the first boot? Is FORCE_REINDEX_ON_STARTUP='true' safe in that window?
[M-4] MEDIUM: Any migration file between 0.35 and LATEST that diverges between
      postgres and cockroach dirs (file-list parity)?
[N-1] NIT: gofmt/vet clean? slog usage consistent? Error wrapping with %w?
[N-2] NIT: Comments accurate (line numbers cited in code.md match source)?

EDGE CASES TO EXAMINE:
- Concurrent boot of two memos instances against the same Cockroach cluster
  (both running Migrate) — no tx means no serialization; is the outcome safe?
- sslmode=require vs verify-full; insecure dev cluster DSNs (28P01 password refusal).
- Vector-index backfill on non-empty tables (SET sql_safe_updates=false) — is any
  code path inserting before index creation, contradicting A1?
- Empty-DB (A1) vs upgrade-from-master-Cockroach scenario — what happens if a user
  has a pre-existing Cockroach DB with old schema?

VERIFICATION COMMANDS (run these):
  go build -tags "cockroach" ./... && go vet ./store/...
  gofmt -l store/ | should be empty
  bash scripts/validate-parity.sh          # expect exit 0
  bash scripts/validate-cockroach-compat.sh # expect exit 0
  # Against a running Cockroach cluster (docker compose, v25.2.21):
  #   SET feature.vector_index.enabled = true (v25.x only)
  go test -tags "cockroach integration" ./store/ -run TestCockroachP0 -v
  go test -tags "cockroach integration" ./store/test/ -run "TestCockroachMigrate" -v
  DRIVER=postgres DSN="postgresql://bchat:bchat@localhost:5432/bchat?sslmode=disable" \
    go test ./store/test/ -v   # failure set must equal the pre-change baseline

OUTPUT FORMAT:
For each finding provide:
- File:line_number
- Severity: CRITICAL/HIGH/MEDIUM/NIT
- Description: what's wrong
- Fix: exact code change

End with a verdict: APPROVE / APPROVE WITH NITS / REQUEST CHANGES, listing any
blocking items. Distinguish findings introduced by this change from pre-existing
codebase issues.
```
