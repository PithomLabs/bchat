# Code: CockroachDB Reference Deployment Profile — Implementation Plan

**Date:** 2026-08-02
**Status:** Planned (pre-implementation plan; no code changes yet)
**Build Tag:** `cockroach`
**Planning doc:** `plan6.md` (approved, plan6_review 9.95/10) — this file is the executable version
**Convention:** follows `bugs/050/code.md` file-inventory format, but as a **pre-implementation** plan

---

## 1. Objective

Implement the CockroachDB **reference deployment profile** (wording per plan6_review #1 — "reference", not "flagship") for bchat: the smallest diff that makes CockroachDB a first-class database profile alongside SQLite/PostgreSQL, deployable to Fly.io with the Taskfile as the operator API, and demonstrable at a hackathon.

The story: **bchat is a Fly.io-native application framework where the database is a deployment profile, not an architecture.** CockroachDB is the reference profile; the path to future profiles (TiDB, PlanetScale, …) stays open because a profile bundles only four things: driver flag, migration dir, DSN source, Fly config (plan6 §1.1).

**Scope guardrails (do NOT implement):** no governance beyond the decision log, no benchmark gates, no schema downgrade, no Standard/Advanced tier work, no private connectivity, no changes to Neon production files (`fly_pg.toml`, `Dockerfile.pg.fly`, `scripts/fly-pg-secrets.sh`).

---

## 2. Architecture (also lands in README, per plan6_review #3)

### 2.1 The diagram (README addition)

```
                  bchat
                    │
             store.Driver
                    │
        ┌───────────┼────────────┐
        │           │            │
     SQLite     PostgreSQL   Cockroach
        │           │            │
     Local      Fly+Neon     Fly+Cockroach
        │           │            │
       Taskfile operator API (intent, not infrastructure)
```

### 2.2 What Cockroach actually is

- A **database profile**: `--driver=cockroach` + `store/migration/cockroach/` + `COCKROACH_DSN` + `fly_cockroach.toml`. Nothing else.
- The shared `store/db/postgres/` package (23 files) already satisfies `store.Driver` for both PostgreSQL and CockroachDB (REPOSITORY FACT: `store/db/db.go`, `store/driver.go`). Cockroach is a new profile, not new architecture.
- Four divergence seams, and the only places driver-specific logic may live: DDL dialect (migration files), transaction semantics (retry wrapper), connection protocol (none today — `simple_protocol` everywhere), capability availability (vector provider file — existing precedent `vectordb_cockroach.go`). No conditional-on-driver logic inside individual SQL methods.
- **Parallel Fly apps `bchat-pg` / `bchat-crdb` are a migration and demo strategy, not the intended permanent deployment topology** (plan6_review #2) — one paragraph in README + deploy doc.
- Future evolution (plan6_review #3): `task deploy:postgres` / `task deploy:cockroach` become `task deploy PROFILE=postgres` / `deploy PROFILE=cockroach`. Mentioned as future, not implemented.

---

## 3. File Inventory

### 3.1 Create

| File | Type | Purpose |
|------|------|---------|
| `store/migration/cockroach/LATEST.sql` | NEW | The real migration (copy of postgres LATEST.sql + 15 `::BIGINT` casts, §4.1) |
| `store/migration/cockroach/0.35/00__tickets_add_internal_notes.sql` | NEW | Inert mirror; satisfies version machinery (migrator.go:262-264 requires ≥1 versioned dir) |
| `store/db/postgres/cockroach.go` | NEW | `NewCockroachDB(profile)` — same package, shared internals (per §1.2) |
| `scripts/validate-cockroach-compat.sh` | NEW | Grep-based compat scanner for cockroach migration SQL (§4.6) |
| `fly_cockroach.toml` | NEW | Fly app config, clone of fly_pg.toml (app `bchat-crdb`) |
| `Dockerfile.cockroach.fly` | NEW | Multi-stage build, clone of Dockerfile.pg.fly, `go build -tags "cockroach"` (§4.9) |
| `scripts/fly-cockroach-secrets.sh` | NEW | Interactive Fly secret setter, clone of fly-pg-secrets.sh (COCKROACH_DSN instead of DATABASE_URL/S3) |
| `scripts/crdb-deploy.sh` | NEW | Thin deploy/rollback/verify chain runner for `deploy:cockroach` / `rollback:postgres` (§4.10) |
| `docs_flyio_cockroach_deploy.md` | NEW | One-page deploy + demo runbook (regions, backups, diagnostics, hardening) |

### 3.2 Modify

| File | Change |
|------|--------|
| `store/db/db.go` | factory `case "cockroach"` (§4.2) |
| `internal/profile/profile.go` | mirror postgres branch for cockroach + `COCKROACH_DSN` (§4.3) |
| `store/migrator.go` | cockroach branch: no `Begin()`, prepend `SET serial_normalization='sql_sequence'`, whole-file exec (§4.4) |
| `store/db/postgres/bridge.go` | 7 BeginTx sites → `crdb.ExecuteTx` (cockroach path only) (§4.5) |
| `store/db/postgres/agent.go` | 1 BeginTx site → `crdb.ExecuteTx` (cockroach path only) (§4.5) |
| `scripts/entrypoint.sh` | `file_env "COCKROACH_DSN"` after line 36 (§4.2) |
| `scripts/validate-parity.sh` | cockroach dir + KNOWN_DIVERGENCES entries (§4.6) |
| `Taskfile.yml` | env fix (`LANCEDB_STORAGE_PROVIDER`), new crdb:/deploy:/verify:/rollback: verbs (§4.7) |
| `deploy/ccloud/setup.sh` | env var swap at :45 + console-first 2-region flow (§4.8) |
| `deploy/ecs/task-definition.json` | doc-only env swap `VECTOR_DB_PROVIDER` → `LANCEDB_STORAGE_PROVIDER` at :26 |
| `README` | architecture diagram (§2.1) + reference-profile wording + parallel-apps note |
| `go.mod` | **no change** — cockroach-go/v2 v2.4.3 already present (go.mod:13) |

### 3.3 Unchanged (explicitly)

All other `store/db/postgres/*.go` files (23), `store/driver.go`, `store/agent.go`, `vectordb_cockroach.go` (zero changes — verified, §4.0), `vectordb_nocockroach.go`, `bin/memos/main.go` (verified: **no change needed**, §4.3), Neon files (`fly_pg.toml`, `Dockerfile.pg.fly`, `fly-pg-secrets.sh`), `store/migration/postgres/*`, `store/migration/sqlite/*`.

---

## 4. Per-file Implementation Specs

### 4.0 Pre-flight audit (do first, ~1h)

| Item | Check | Verified fact |
|------|-------|---------------|
| CockroachVectorDB | Read `vectordb_cockroach.go` fully | Constructor requires `COCKROACH_DSN` (:26-31); appends `default_query_exec_mode=simple_protocol` (:50-58); runtime schema + vector index creation tolerates `42P07` (already exists) and `0A000` (feature not supported → logs warning, falls back to brute-force) (:120-135); `Dimension()` = 1536; `Close()` = pool close. **No changes required.** |
| Provider switch | `vectordb.go:280-294` | `case "cockroach": return NewCockroachVectorDB(config, embedSvc)` (~:288-290). Env reads: `LANCEDB_STORAGE_PROVIDER` (:121), `COCKROACH_DSN` (:131). **No changes required.** |
| Shared pool | `service.go:160-170` | Pool shared with store pool when `CockroachDSN == ""` or `== p.DSN`; dedicated pool otherwise. **No changes required.** |
| Build tags | all `vectordb_*.go` | `cockroach`/`!cockroach` (cockroach/nocockroach), `rag`/`!rag` (lance/nolance/pool). **The `rag` tag is NOT needed for the Cockroach path** — it only gates LanceDB (local/s3) + per-tenant pool (vectordb_nolance.go stub errors only for those providers). Dockerfile + Taskfile therefore use `-tags "cockroach"` only; no liblancedb_go.so needed. Correction to plan6 §4.9 ("cockroach rag"). |
| Retry precedent | `vectordb_cockroach.go` | Uses `crdb.ExecuteTx` already (per plan6 §2.1). |
| Retryable matcher | `bridge.go:311-323` | `isPostgresConstraint` (:311-318), `isPostgresRetryable` (:320-323, message-based). Postgres-only; cockroach path never reaches it. |
| Existing retry loop | `bridge.go:94-109` | `CreateBridgeHandoff` = 3-attempt loop over `createBridgeHandoffAttempt`, sleeps `(attempt+1)*10ms`, falls back to `store.ErrBridgeHandoffConflict`. Wrapper precedent for §4.5. |

### 4.0.1 P0 gate — prototype the exact `execute()` path FIRST (pre_code_review required #1)

The highest-risk remaining assumption: `SET serial_normalization='sql_sequence';` + the whole ~1030-line multi-statement `LATEST.sql` executed as **one** `ExecContext` through pgx simple protocol on Cockroach. `execute()` (migrator.go:321-335) already works this shape today — whole file content as a single `ExecContext`; multi-statement works because the DSN appends `default_query_exec_mode=simple_protocol` (postgres.go:33). Two unknowns: (a) does Cockroach apply the session SET to the subsequent DDL in one simple-protocol round trip, and (b) do `execute()`'s tolerance strings (`"already exists"`, `"column already exists"`, `"duplicate column"`, migrator.go:326-328) match Cockroach's actual messages (`relation "x" already exists`, 42P07)?

**Spec — `store/cockroach_p0_test.go`, build tag `//go:build cockroach && integration`, run against the compose cluster (v25.2.21; remember `SET feature.vector_index.enabled = true` first, §5):**

1. `sql.Open("pgx", <compose DSN> + "&default_query_exec_mode=simple_protocol")` — byte-identical config to `NewCockroachDB`'s (postgres.go:27-45).
2. Read `store/migration/cockroach/LATEST.sql` from `MigrationFS`, run `tx.ExecContext(ctx, "SET serial_normalization = 'sql_sequence';\n"+content)` — exactly what the §4.4 branch hands to `execute()`.
3. Assert: no error; `SHOW CREATE TABLE` on ≥3 tables (e.g., `agent_tenants`, `memo`, `tickets`) shows `nextval(...)` defaults, zero `unique_rowid()`.
4. Re-run the same `ExecContext` → assert no error (idempotency), and confirm the duplicate-table error message contains "already exists" so `execute()`'s tolerance would swallow it.
5. Leave no residue (drop test-created objects; A1).

This prototype doubles as the P1/P2 evidence harness (§6.1) — keep it as a committed integration test. **Gate (blocking): P0 must pass before the migrator.go branch is written (§8 step 5).** If (a) fails → fallback: split the SET into its own `ExecContext` before the file exec (still minimal, no shared-code change). If (b) fails → extend the tolerance strings in the cockroach arm only (§4.4).

### 4.1 `store/migration/cockroach/LATEST.sql` (generated, ~1030 lines)

1. `cp store/migration/postgres/LATEST.sql store/migration/cockroach/LATEST.sql`.
2. Add `::BIGINT` to the **15 uncast** `EXTRACT(EPOCH FROM NOW())` defaults. REPOSITORY FACT (verified this session): total 19 occurrences; uncast at **lines 4, 17, 18, 43, 44, 80, 81, 98, 116, 126, 127, 138, 217, 236, 751**; already cast at **988, 989, 1009, 1026**. DOCUMENTATION FACT: Cockroach `extract` returns float with no implicit float→int8 assignment — uncast defaults fail `CREATE TABLE`.
   - Mechanical step: `grep -n "EXTRACT(EPOCH FROM NOW())" LATEST.sql | grep -v "::BIGINT"` must return exactly the 15 lines above; edit each.
3. **P0 deviation (2026-08-02, P0 gate):** add `IF NOT EXISTS` to all **57 bare `CREATE TABLE` + 90 bare `CREATE INDEX`/`CREATE UNIQUE INDEX`** statements (147 total). The P0 prototype proved the plan's prior claim "all DDL IF NOT EXISTS" **wrong** — postgres LATEST.sql relies on the migrator's "already exists" error-tolerance, which is unsafe for whole-file re-runs: in pgx simple protocol the first tolerated error **aborts the remaining batch**, so a failed-boot re-run would silently half-apply the schema. With the cockroach file fully idempotent, whole-file re-runs are clean (P2 gate holds). This is the only content deviation from the postgres file beyond the casts: 15 casts + 147 `IF NOT EXISTS` (verified statement-by-statement; diff is exactly those lines).
4. **P1 deviation (2026-08-02, e2e gate):** convert `UNIQUE` → `PRIMARY KEY` on **3 tables** — `user_setting(user_id, key)`, `memo_organizer(memo_id, user_id)`, `memo_relation(memo_id, related_memo_id, type)`. REPOSITORY FACT: those are the only PRIMARY KEY-less tables in the file; Cockroach synthesizes a hidden `rowid INT NOT NULL DEFAULT unique_rowid()` for them (int64, breaks the int32 ID invariant — caught by the information_schema scan in `TestCockroachP0`/`TestCockroachMigrateEndToEnd`). Converting the existing UNIQUE constraint to PRIMARY KEY is semantically identical (same columns, same uniqueness) and removes the synthesized column. OBSERVED FACT: first LATEST.sql apply on the compose cluster takes **~100s** (many background DDL jobs); re-applies are ~2s — document in operator docs.
   - NOTE: the earlier "table names identical" check was vacuous — the grep pattern `^CREATE TABLE [a-z_]+` stopped matching after the `IF NOT EXISTS` prefix was added. The correct check is `^CREATE TABLE (IF NOT EXISTS )?`.
4. `store/migration/cockroach/0.35/00__tickets_add_internal_notes.sql`: content `ALTER TABLE tickets ADD COLUMN internal_notes TEXT DEFAULT '';` — byte-identical to postgres 0.35/ file. Never executes under A1 (preMigrate applies LATEST.sql, then Migrate applies only files > latest history version — migrator.go:160-207, 83-133). REPOSITORY FACT: postgres `0.35/` contains exactly one file → mirror yields identical `GetCurrentSchemaVersion` = **0.35.1** for both drivers.
5. Schema-version consistency: `GetCurrentSchemaVersion` (migrator.go:257-281) errors when no versioned dirs exist (:262-264) — hence the 0.35/ mirror is mandatory, not cosmetic.

### 4.2 `store/db/db.go` + `scripts/entrypoint.sh`

```go
// store/db/db.go — after the postgres case (:23-24)
case "cockroach":
    driver, err = postgres.NewCockroachDB(profile)
```

`scripts/entrypoint.sh`: add `file_env "COCKROACH_DSN"` directly after line 36 (`file_env "MEMOS_DSN"`). Pattern verified: entrypoint.sh:3-27.

### 4.3 `internal/profile/profile.go` — and why main.go is untouched

Mirror the postgres branch (profile.go:97-102), inserted after it:

```go
if p.Driver == "cockroach" && p.DSN == "" {
    p.DSN = os.Getenv("COCKROACH_DSN")
    if p.DSN == "" {
        return errors.New("cockroach driver requires DSN or COCKROACH_DSN environment variable")
    }
}
```

- **Correction to plan6 §3.3:** `bin/memos/main.go` needs **no change**. Verified: `Run()` builds the profile with `DSN: viper.GetString("dsn")` (main.go:60); viper maps env `MEMOS_DSN` via `SetEnvPrefix("memos")` + `AutomaticEnv()` (main.go:258-259) — that channel is shared by all drivers and we do not set MEMOS_DSN on the CRDB deploy. The postgres DSN env (`DATABASE_URL`) is read inside `profile.Validate()`, not viper — the cockroach branch mirrors that exact mechanism (D7: `COCKROACH_DSN` only, no fallback; driver/DSN mismatch = fail-fast at `Validate()`).
- Update the driver doc comment at profile.go:29 ("sqlite, mysql" → "sqlite, mysql, postgres, cockroach").

### 4.4 `store/migrator.go` — cockroach branch (MINIMAL DIFF — pre_code_review required #2)

**Constraint:** no refactoring of shared migration logic. Two isolated `if s.profile.Driver == "cockroach" { … } else { … }` guards in `preMigrate` (:178-189) and `Migrate` (:90-137); `execute()` (:321-335), all version logic, and the sqlite/postgres paths stay byte-identical. This is the highest-risk file in the repo — the smaller the diff, the easier the review.

1. **No `Begin()`/`Commit()`.** Today: `Migrate` begins a tx at :91, `preMigrate` at :179. DOCUMENTATION FACT: Cockroach does not support DDL in explicit transactions (online schema changes run as background jobs; `autocommit_before_ddl` commits prior statements anyway — the tx would be cosmetic at best, an error at worst).
   - `preMigrate`: wrap the existing `Begin → execute → Commit` block (:179-189) in `else`; the cockroach arm runs the P0-verified statement (`SET serial_normalization = 'sql_sequence';\n` + file content) via `s.driver.GetDB().ExecContext(ctx, stmt)` — no tx.
   - `Migrate`: same guard around the per-file loop's `Begin → execute → Commit` (:91-137). Under A1 this arm never executes for cockroach (the inert 0.35/ file version ≤ history version, :107), but it must not `Begin()` if it ever does.
2. **Prepend `SET serial_normalization = 'sql_sequence';`** to the file content read from `MigrationFS` (preMigrate :169, Migrate :125). The SET + whole file is one `ExecContext` (P0-verified, §4.0.1).
   - **Why `sql_sequence`:** all model IDs are `int32` (REPOSITORY FACT: `store/agent.go`, `store/driver.go`); Cockroach's default `unique_rowid()` is 64-bit and overflows int32 scans. `sql_sequence` → `nextval()` keeps IDs in int32 range.
3. **Atomicity (explicit, one paragraph):** failed statement N no longer rolls back 1..N-1. Safe for bchat because (a) history is upserted only after full success (migrator.go:142-149, 192-199) → failed boot re-runs the migration; (b) `LATEST.sql` is fully idempotent (§4.1); (c) the cockroach arm's error tolerance (below) swallows "already exists"-class errors (message match P0-verified, §4.0.1); (d) under A1 only `LATEST.sql` runs at first deploy — no partial-history recovery exists in the field.

**Error tolerance:** the cockroach arm inlines `execute()`'s 4-line tolerance check (migrator.go:326-328) verbatim (`errMsg contains "already exists" | "column already exists" | "duplicate column" → slog.Warn + return nil`), with a comment pointing at `execute()` — deliberate duplication so `execute()` and the shared path stay byte-identical. (If the resulting total diff would be smaller with a tiny extracted helper used by both, choose the extraction — but only then.)

### 4.5 Retry wrappers — 8 sites (`bridge.go` ×7, `agent.go` ×1)

**Conversion pattern (cockroach path only; postgres path unchanged):**

```go
return crdb.ExecuteTx(ctx, d.db, nil, func(tx *sql.Tx) error {
    // ...existing body, tx.QueryRowContext/ExecContext unchanged...
})
```

- `crdb.ExecuteTx(ctx, *sql.DB, opts, fn)` — API verified in the repo's exact dependency cockroach-go v2.4.3 (`crdb/tx.go:299`); precedent `vectordb_cockroach.go`.
- **Two sites use `d.db.Conn(ctx)` + `conn.BeginTx`** (bridge.go:356 `CreateBridgeHandoffReplyIfActive`, :500 `CreateBridgeHandoffReplyAndOutboxIfActive`): `crdb.ExecuteTx` takes `*sql.DB`, so the conversion drops the explicit `Conn()` and passes `d.db` — the closure receives its own `*sql.Tx`. Verify the body doesn't rely on the pinned connection (audit during implementation; these are single-tx bodies, so it does not).

**IMPL status (2026-08-02):** realized as a shared `execTx` dispatcher (`store/db/postgres/tx.go`) instead of per-site inline `crdb.ExecuteTx` — one driver gate (`d.profile.Driver == "cockroach"` → `crdb.ExecuteTx`; else BeginTx/Commit, today's semantics), bodies moved into closures at each site (no duplication), all 8 `// retry-safe:` comments in place. Mid-body `tx.Commit()`/`tx.Rollback()` calls were removed per site (execTx owns commit/rollback — a mid-body commit would break ExecuteInTx's SAVEPOINT protocol). Postgres regression-verified: full `store/test` suite on postgres = 26 failures BEFORE and AFTER (identical set, all pre-existing on master); sqlite suite green. The two `Conn()`-pinned sites dropped the pin per plan (single-tx bodies, audited: no outside-tx conn use).

**Site inventory (BeginTx line → function → retry-determinism classification):**

| # | File:Line | Function | Determinism (plan6 §3.4) |
|---|-----------|----------|---------------------------|
| 1 | bridge.go:112 | `createBridgeHandoffAttempt` | status-guarded (external-session lock + active-count check); safe on re-read |
| 2 | bridge.go:356 | `CreateBridgeHandoffReplyIfActive` | status-guarded (`IfActive`); safe on re-read |
| 3 | bridge.go:500 | `CreateBridgeHandoffReplyAndOutboxIfActive` | status-guarded (`IfActive`); safe on re-read |
| 4 | bridge.go:726 | outbox claim | token/status-guarded; safe on re-read |
| 5 | bridge.go:818 | `CompleteClaimedBridgeReplyOutbox` | token-guarded (claim token); safe on re-read |
| 6 | bridge.go:912 | `FailClaimedBridgeReplyOutbox` | token-guarded; safe on re-read |
| 7 | bridge.go:1003 | `ReclaimExpiredBridgeReplyOutbox` | status-guarded; safe on re-read |
| 8 | agent.go:772 | `CreateAgentMessages` | append-only; safe on re-run |

Classification rule: all 8 bodies are pure DB operations; side effects occur after `Commit()` in callers — a 40001 retry never double-writes (REPOSITORY FACT audit from plan5 §9.2). **MANDATORY per-site comment block** (pre_code_review #2 — reviewer insists, not optional) immediately above each `crdb.ExecuteTx` call:

```go
// retry-safe: <status-guarded | token-guarded | append-only> — <why a 40001 retry cannot double-write>
```

Example: `// retry-safe: token-guarded — claim token filters already-claimed rows on re-read`. `isPostgresRetryable` string matching (bridge.go:320-323) stays for Postgres; the Cockroach path never reaches it. Non-retryable errors propagate; `store.ErrBridge*` sentinels untouched.

### 4.6 Validation scripts

**`scripts/validate-parity.sh`** — extend, don't refactor:
- Add `COCKROACH_DIR="$REPO_ROOT/store/migration/cockroach"` + `COCKROACH_LATEST` beside :32-35.
- Add KNOWN_DIVERGENCES entries (:44-68 block): `0.19:...:0.34` → `cockroach minimal mirror (inert; version machinery only)` and `0.35:postgres applies real migration; cockroach mirror is inert`. All sqlite 0.2-0.18 divergences apply to cockroach implicitly only if file-list parity checks cockroach dir — note: cockroach has only 0.35/, so the parity check loops over dirs of each driver; cockroach's single 0.35/ dir must match postgres 0.35/ file count (1 file each) — this works **only if** the check tolerates cockroach missing 0.19-0.34 (add those as known divergences).
- Schema check (names only, :170-223) compares `POSTGRES_LATEST` vs `SQLITE_LATEST`; the `::BIGINT` text difference is invisible to it (name extraction via grep, :172-176). **Optional:** add a cockroach-vs-postgres name comparison as a third pair (cheap, same helpers).
- Wire into `task validate:parity` deps.

**`scripts/validate-cockroach-compat.sh`** (new, ~50 lines, grep-based, exit 1 on FORBIDDEN / 2 on REVIEW-REQUIRED):
- FORBIDDEN in `store/migration/cockroach/**/*.sql` (MCP-verified unsupported): `CREATE EXTENSION`, `LISTEN`/`NOTIFY`, `pg_advisory_lock`, `CREATE DOMAIN`, range types/`MACADDR`/`MONEY`, `CREATE TRIGGER`, `DEFERRABLE`, `DROP PRIMARY KEY`, PL/pgSQL `DO $$...$$` blocks.
- REVIEW-REQUIRED (unannotated, exit 2): `ALTER TYPE`, `UPDATE ... FROM`, `COPY`.
- Explicitly best-effort, not a parser — authoritative gate is code review + §4.1 audit (plan6 §3.7). Wire into `crdb:check`.

**Strict independence (pre_code_review #3):** compat and parity are separate concerns, separate scripts, **zero shared logic**. `validate-cockroach-compat.sh` must never grow parity checks, and `validate-parity.sh` never compat checks. If consolidation is ever justified, it gets its own file + review — not accretion.

### 4.7 Taskfile.yml — env fixes + operator API

**Env fixes (correctness):** the RAG pipeline reads `LANCEDB_STORAGE_PROVIDER` (vectordb.go:121); `VECTOR_DB_PROVIDER` is vestigial. Fix at:
- `run:cockroach` (:241): `VECTOR_DB_PROVIDER=cockroach` → `LANCEDB_STORAGE_PROVIDER=cockroach`
- `crdb:check` (:263-266): replace `VECTOR_DB_PROVIDER` check with `LANCEDB_STORAGE_PROVIDER`
- `crdb:docker:run` (:348): same swap
- `deploy/ccloud/setup.sh:45` and `deploy/ecs/task-definition.json:26`: same swap (doc-only)

**New verbs (plan6 §4.1, with verified corrections):**

| Task | Performs | Notes |
|------|----------|-------|
| `crdb:up` / `crdb:down` / `crdb:reset` | docker compose up/down/-v on `scripts/docker-compose.cockroach.yml` | image `cockroachdb/cockroach:v25.2.21`, `bchat_user`@localhost:26257/bchat |
| `crdb:migrate` | boot `./build/memos --driver=cockroach --mode dev` (boot = migrate, main.go:97-98) | |
| `crdb:check` | env + DSN + compat scanner + parity | extends existing :253-271 |
| `crdb:verify` | production-facing P1-P6 (§6.2) | new |
| `crdb:init` | console-first 2-region bootstrap (§5) | replaces existing single-region `crdb:cluster:create` semantics (:286-292) — keep task, change flow; document |
| `crdb:test` | `go test -tags "cockroach"` on agent tests + P4/P5 harness (optional `--experiments`) | extends existing :325-331 |
| `deploy:cockroach` | `scripts/crdb-deploy.sh` chain (§4.10) | new |
| `deploy:postgres` | wraps existing Neon flow (`fly -a bchat-pg deploy -c fly_pg.toml`) — proves "same Taskfile" | new |
| `verify:production` | §6.3 | new |
| `rollback:postgres` | §6.4 | new |
| `crdb:harden` | §6.5 | new |

`crdb:cluster:create`'s current body (`ccloud cluster create basic hackathon-demo aws-us-east-1 --cloud AWS --spend-limit 0`, :291) is **single-region only** (verified CLI syntax) — replaced by the console-first flow of `crdb:init`; the raw task stays for dev convenience only.

Leave a TODO comment in Taskfile.yml beside the new `crdb:up`/`crdb:down`/`crdb:reset` tasks (pre_code_review #5 — not for this PR): `# TODO(post-hackathon): profile-parameterized aliases — db:up PROFILE=cockroach, db:reset PROFILE=cockroach (scales to SQLite/TiDB/PlanetScale)`.

### 4.8 `deploy/ccloud/setup.sh`

- Swap `VECTOR_DB_PROVIDER=cockroach` → `LANCEDB_STORAGE_PROVIDER=cockroach` at :45 (already prints export lines).
- Reframe bootstrap comment + echo text: cluster creation is console-first for 2-region (§5); the script handles user/DSN/allowlist after that.

### 4.9 `fly_cockroach.toml` + `Dockerfile.cockroach.fly` (clones)

`fly_cockroach.toml` — clone of fly_pg.toml (54 lines) with these deltas:

| Setting | fly_pg.toml (Neon) | fly_cockroach.toml |
|---|---|---|
| App | `bchat-pg` (:4) | `bchat-crdb` |
| Primary region | `sjc` (:5) | `sjc` (unchanged) |
| Dockerfile | `Dockerfile.pg.fly` (:8) | `Dockerfile.cockroach.fly` |
| `MEMOS_DRIVER` | `postgres` (:11) | `cockroach` |
| DSN secret | `DATABASE_URL` | `COCKROACH_DSN` (Fly secret via `scripts/fly-cockroach-secrets.sh`; also `OPENROUTER_API_KEY`, `ENCRYPTION_MASTER_KEY`) |
| `LANCEDB_STORAGE_PROVIDER` | `s3` (:19) + Tigris creds | `cockroach`; **no AWS/Tigris secrets at all** |
| Reindex | `RAG_STARTUP_REINDEX_DISABLED='true'` (:25) | `FORCE_REINDEX_ON_STARTUP='true'` for initial A1 reindex only (then disable — vector-index backfill blocks writes, safe pre-traffic) |
| Health check | `/healthz` (:44-48) | identical |
| VM | 1024mb shared 1cpu (:50-53) | identical |

`Dockerfile.cockroach.fly` — clone of Dockerfile.pg.fly with **`go build -tags "cockroach"`** (NOT "cockroach rag" — §4.0: rag tag is LanceDB-only and would require CGO + liblancedb_go.so for nothing). Result: no liblancedb_go.so copy step; otherwise keep the multi-stage shape (node:20-alpine frontend → golang backend → Ubuntu 24.04 runtime with gosu/memos user).

`scripts/fly-cockroach-secrets.sh` — clone of fly-pg-secrets.sh (interactive, `set -e`, colorized, checks `fly auth whoami`): sets `COCKROACH_DSN`, `OPENROUTER_API_KEY`, `ENCRYPTION_MASTER_KEY` on app `bchat-crdb`. No S3/AWS vars.

### 4.10 `scripts/crdb-deploy.sh` (deploy/verify/rollback orchestration)

Chain for `deploy:cockroach` (plan6 §4.5):

```
build:backend:cockroach (tags "cockroach")
→ validate-parity.sh (now includes cockroach pair)
→ validate-cockroach-compat.sh
→ (--experiments: P1-P6, optional)
→ fly -a bchat-crdb deploy -c fly_cockroach.toml
→ poll https://bchat-crdb.fly.dev/healthz (grace 15s, fly_pg.toml pattern)
→ crdb:verify steps (§6.2)
→ verify:production (§6.3)
```

Script is stateful-safe: re-runnable; logs each stage to `build/crdb-deploy.log`.

**Thinness ceiling (pre_code_review #6):** hard limit **100 lines**. This script is a chain runner (run stage → check exit → log → next), never a deployment brain — all logic lives in Taskfile / `crdb:verify` / `verify:production`. If implementation exceeds 100 lines, move logic back into Taskfile rather than growing the script.

---

## 5. Cloud bootstrap — `task crdb:init` (VERIFIED REVISION of plan6 §4.3)

**Two verified constraints change the flow (MCP, this session):**
1. `ccloud cluster create basic <name> <region> --cloud <C|A> --spend-limit 0` accepts **exactly one region** — no multi-region option for Basic on the CLI (multi-region region syntax `us-central1:8 us-west2:4` exists only for `ccloud cluster create dedicated`).
2. **You cannot add regions to an existing single-region Basic cluster** ("You cannot currently edit the region configuration for a single-region cluster once it has been created"), and you cannot remove regions. Multi-region must be chosen **at creation**, which is only possible in the **Cloud Console** (Create cluster → Regions → **Add regions**; max 6).

**Therefore `crdb:init` is console-first:**

1. `ccloud auth login` — validate CLI present + authenticated.
2. **Console (manual, guided by the script's output):** create **Basic** cluster on AWS with **2 regions — `us-east-1` (primary) + `us-west-2`**. (Rationale: 2 regions survive zone failure; nearest to Fly `sjc`. Region choice is console-guided — plan6's "regions nearest sjc" intent is preserved; primary region can be changed later via console "Set primary region".) Free at $0 idle.
3. Create SQL user (console dialog) → `ccloud cluster sql --connection-url` (or console copy) for the DSN: `postgresql://user:pass@<cluster>-<id>.<org>.crdb.cloud:26257/bchat?sslmode=verify-full` (DOCUMENTATION FACT; CA is Let's Encrypt — system roots suffice).
4. DSN → `.env` (local) / Fly secret `COCKROACH_DSN` (prod).
5. **Vector setting — verified state:** `feature.vector_index.enabled` is **supported on Basic** (supported-deployments column: Basic/Standard/Advanced/Self-Hosted) and its **default is `true` on stable (v26.x)** — so on a current Cloud Basic cluster this is a no-op; `SET CLUSTER SETTING feature.vector_index.enabled = true;` is required only on v25.2/v25.3 clusters (default `false`) and on the local compose cluster (v25.2.21). `crdb:init` still issues the SET defensively (harmless no-op on v26) and records the result. Note: the local compose cluster needs it; vector index creation on **non-empty** tables additionally needs `SET sql_safe_updates = false` (backfill blocks writes) — irrelevant under A1 (tables empty at schema time; reindex pre-traffic).
6. Ping: `SELECT version()` → must report Cockroach (guards DSN cross-wiring, D7).
7. Output: run `crdb:verify`, then `deploy:cockroach`.

Allowlist: Basic ships with `0.0.0.0/0` (DOCUMENTATION FACT) — zero-config bootstrap; hardening is `crdb:harden` (§6.5).

---

## 6. Verification

### 6.1 Gates P1–P6 (local, on the compose cluster; Q&A decision 4 — P1-P3+P6 mandatory, P4-P5 optional)

| ID | Question | Method | Gate |
|----|----------|--------|------|
| P1 | Does injected `SET serial_normalization='sql_sequence'` produce `nextval()` defaults? | run cockroach LATEST.sql via migrator; `SHOW CREATE TABLE` on ≥3 tables | **Mandatory** — 100% `nextval()`, 0 `unique_rowid()` |
| P2 | Whole-file exec idempotent on failure + re-run? | full run; delete history row; re-run; inject failing statement mid-file | **Mandatory** — no dupes; failed run writes no history |
| P3 | `EXTRACT(...)::BIGINT` valid as a default? | `CREATE TABLE t (c BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT); INSERT; SELECT` | **Mandatory** — epoch ±1s; uncast variant fails |
| P6 | Vector path on the deployment config | `crdb:test` with `LANCEDB_STORAGE_PROVIDER=cockroach`; verify `feature.vector_index.enabled` flow | **Mandatory** — index exists; search round-trip |
| P4 | `crdb.ExecuteTx` retry under contention (1000 claims, 0 dup/lost, ≥1 40001; optional abort/disconnect cases) | concurrency harness on outbox sites | **Optional** (`--experiments`) |
| P5 | SKIP LOCKED exactly-once claims | concurrent workers on `ClaimPendingEvents` (agent.go:2773-2781) | **Optional** |

All run on `docker-compose.cockroach.yml` (v25.2.21 — vector-index capable; `SET feature.vector_index.enabled = true` needed there, §5).

### 6.2 `task crdb:verify` (production-facing P1–P6)

1. `SELECT 1` via pgx over `COCKROACH_DSN`.
2. `SELECT version()` — Cockroach, not Postgres.
3. `migration_history` — exactly one row = `0.35.1` under A1.
4. `SHOW CREATE TABLE agent_tenants` — `nextval(...)` defaults (P1 evidence in production).
5. Vector index exists + `SHOW CLUSTER SETTING feature.vector_index.enabled` = `true`.
6. Log-grep for retry-wrapper initialization.
7. `GET /healthz` 200.

### 6.3 `task verify:production` (app-first smoke)

Against the deployed app's public API: create tenant → create memo → KB upload + reindex (vector insert) → RAG search (≥1 hit) → bridge handoff create/claim/complete cycle (exercises retry wrappers) → delete test data (`--destroy` default on). Any failure fails the deploy report.

### 6.4 `task rollback:postgres` (demo capability, not DR — plan6_review #6)

> Rollback exists primarily as a demo capability proving deployment profile portability.

Driver switch + redeploy, same image: set `DATABASE_URL` (Neon) secret, unset `COCKROACH_DSN`, flip `MEMOS_DRIVER=postgres` (encoded declaratively via a `fly_pg-rollback.toml` clone of `fly_pg.toml` with app `bchat-crdb`), redeploy, `verify:production`. **No schema downgrade** — CRDB data stays at last applied schema; re-cutover is a plain forward migration. Vector behavior verified safe: with driver=postgres and COCKROACH_DSN unset → Neon + memory fallback provider; if COCKROACH_DSN is kept → `service.go:160-170` opens a dedicated CRDB pool for vectors (legitimate mixed state). RAG env resets to Neon values in the task definition.

### 6.5 `task crdb:harden` (networking)

Default `0.0.0.0/0`; hardening = allocate Fly static egress IP (`fly ips allocate-egress`, ~$3.60/mo), allowlist `<ip>/32` via `ccloud cluster networking allowlist create`, remove `0.0.0.0/0`, then **mandatory connectivity verification** (community-reported breakage after allocation; if broken: release IP, restore `0.0.0.0/0`). Allowlist limit 50 rules — ample.

### 6.6 Demo (plan6 §5, with plan6_review #5 applied)

Spine (unchanged, keep exactly): `task db:local` → SQLite works; `task deploy:postgres` → Fly+Neon works (same Taskfile, live today); `task deploy:cockroach` → Fly+CRDB works (one command); `task rollback:postgres` → back to Neon works.

Distributed SQL demo is **app-first**: run the §6.3 flow on the 2-region cluster while stating "this cluster is spanning two regions" — keep SQL to a minimum during the presentation (`SHOW REGIONS` as a supporting screenshot/short clip, not the centerpiece). Other native features: automatic retries (concurrent outbox claims → 40001 in logs), native VECTOR (`SHOW CREATE TABLE agent_vectors` shows `VECTOR(1536)` + C-SPANN index), online schema change (ALTER while query loop runs — zero blocking). Fallback: pre-recorded capture of the same steps.

---

## 7. Implementation Exit Criteria (merge checklist — pre_code_review required #3)

This is what "done" means at merge time. **All boxes must be checked** before the PR is considered complete:

- [x] **Compiles** — `task build:backend:cockroach` (tags `cockroach`) and the default build (`task build`) both green
- [x] **SQLite unchanged** — `task validate:parity` green; git diff empty for `store/migration/sqlite/`
- [x] **PostgreSQL unchanged** — git diff for `store/migration/postgres/` empty; `store/db/postgres/*.go` diff limited to the 7 retry wrappers in `bridge.go` (no other edits) — plus the 1 site in `agent.go` (§4.5 inventory) and new files `cockroach.go`/`tx.go`
- [x] **Cockroach boots** — `task crdb:up` + `task crdb:migrate` boots to server start (main.go:97-98 path)
- [x] **Migrations succeed** — P0 + P1 + P2 + P3 gates green (§6.1); `migration_history` = `0.35.1` under A1 (e2e re-verified 2026-08-02, 109.47s)
- [x] **Retries verified** — `crdb:test` green; mandatory `// retry-safe:` comment present at all 8 wrapper sites; P4 gate optional (skip per Q&A decision 4 — P1-P3+P6 mandatory)
- [ ] **Fly deploy works** — `task deploy:cockroach` end-to-end; `GET /healthz` 200 on `bchat-crdb.fly.dev` — **PENDING LIVE EXECUTION** (no authenticated ccloud/fly in this environment; chain verified through stage 4/7 locally, stage 5 fails cleanly without auth)
- [ ] **Rollback demonstrated** — `task rollback:postgres` succeeds on `bchat-crdb`; `verify:production` green after rollback — **PENDING LIVE EXECUTION** (same reason)
- [x] **README updated** — §2.1 architecture diagram + "reference deployment profile" wording + parallel-apps note present (added under `## Architecture → ### Deployment Profiles`)
- [x] **Zero changes to Neon deployment** — git diff empty for `fly_pg.toml`, `Dockerfile.pg.fly`, `scripts/fly-pg-secrets.sh`
- [x] **Docs written** — `docs/docs_flyio_cockroach_deploy.md` exists and matches the deployed flow

---

## 8. Implementation Order (checklist)

| Step | Work | Gate |
|------|------|------|
| 1 | §4.0 audit (read all cited files; confirm line numbers) + write P0 test (§4.0.1) | audit log in this file updated; P0 test compiles under `-tags "cockroach integration"` |
| 2 | `store/migration/cockroach/` (LATEST.sql + 0.35/ mirror) + §4.1 verification | 15 casts; mirror byte-identical |
| 3 | `store/db/db.go`, `internal/profile/profile.go`, `store/db/postgres/cockroach.go`, `scripts/entrypoint.sh` | `--driver=cockroach` + empty DSN → fail-fast error message |
| 4 | **P0 gate — run the prototype against the compose cluster** (§4.0.1) | SET + whole-file exec passes; nextval on ≥3 tables; clean re-run. **Blocking:** migrator.go stays untouched until green |
| 5 | `store/migrator.go` cockroach branch — minimal diff (§4.4) | P1 via real boot (`crdb:migrate`); nextval on ≥3 tables |
| 6 | 8 retry wrappers (§4.5) with mandatory `// retry-safe:` comments | compile + `crdb:test`; P4 optional |
| 7 | Validation scripts (§4.6) | `task validate:parity` green; scanner exits 1/2 on fixtures |
| 8 | Taskfile verbs + env fixes (§4.7), setup.sh/ECS env swap | `task crdb:up` → `crdb:migrate` → `run:cockroach` works locally |
| 9 | Fly artifacts (§4.9) + thin `crdb-deploy.sh` (§4.10) + secrets script | ✅ image builds (`-tags cockroach`, no CGO); chain verified through stage 4/7; `fly deploy` dry-run pending live auth |
| 10 | `crdb:init` console flow (§5) | ⏳ PENDING LIVE — cluster creation is console-first (user); ccloud not installed/authenticated in this environment |
| 11 | `crdb:verify` + `verify:production` + `rollback:postgres` | ✅ implemented (§6.2 checks env-gated + smoke script); ⏳ full §6.3 smoke + rollback demonstrated pending live deploy |
| 12 | README diagram + wording; `docs_flyio_cockroach_deploy.md` | ✅ written (README `### Deployment Profiles`; `docs/docs_flyio_cockroach_deploy.md`) |

**Done definition:** the §7 exit checklist is fully checked — `task deploy:cockroach` end-to-end on the 2-region Basic cluster, `verify:production` green, `rollback:postgres` demonstrated, README + deploy doc written, Neon files untouched (git diff shows zero changes under `fly_pg*`/`Dockerfile.pg.fly`).

---

## 9. Evidence Sources (verified 2026-08-02)

### REPOSITORY FACTS (this session, file:line)
- Driver factory switch: `store/db/db.go:18-27` (default error :26).
- Postgres profile branch: `internal/profile/profile.go:97-102`; driver comment :29.
- main.go: profile DSN from viper :60; `SetEnvPrefix("memos")`/`AutomaticEnv` :258-259; boot migration :97-98; no cockroach change needed.
- `postgres.NewDB` template: `store/db/postgres/postgres.go:22-54` (simple_protocol :27-34, pool :42-45, 60s PingContext :47-51).
- 8 BeginTx sites: bridge.go:112, 356, 500, 726, 818, 912, 1003; agent.go:772. Retryable matcher bridge.go:311-323; retry loop precedent bridge.go:94-109.
- LATEST.sql: 15 uncast EXTRACT defaults at 4, 17, 18, 43, 44, 80, 81, 98, 116, 126, 127, 138, 217, 236, 751; 4 cast at 988, 989, 1009, 1026; `ON CONFLICT (tenant_id, code)` :208; total 1030 lines.
- Migrator: Migrate :38-158 (Begin :91), preMigrate :160-207 (Begin :179), base path :209-211, GetCurrentSchemaVersion :257-281 (error :262-264), version-consistency warn-only :283-298.
- vectordb.go: `LANCEDB_STORAGE_PROVIDER` :121, `COCKROACH_DSN` :131, provider switch :280-294 (case `cockroach` ~:288-290).
- vectordb_cockroach.go: DSN required :26-31, simple_protocol :50-58, 42P07/0A000 tolerance :120-135 — zero changes.
- validate-parity.sh: dirs :32-33, KNOWN_DIVERGENCES :44-68, schema name compare :170-223.
- Taskfile.yml: `build:backend:cockroach` :222-226 (`-tags "cockroach"` — sufficient), `run:cockroach` :232-241, `crdb:check` :253-271, `crdb:cluster:create` :286-292 (single-region CLI), `crdb:docker:run` :341-354.
- fly_pg.toml: app :4, sjc :5, Dockerfile :8, driver :11, provider :19, reindex :25, health :44-48, VM :50-53.
- entrypoint.sh file_env :3-27, MEMOS_DSN :36.
- 0.35 mirror source: `store/migration/postgres/0.35/00__tickets_add_internal_notes.sql` (1 line, `ALTER TABLE tickets ADD COLUMN internal_notes TEXT DEFAULT '';`).
- Build tags: `cockroach`/`!cockroach`, `rag`/`!rag` across vectordb_*.go; nolance stub proves rag tag gates only LanceDB/pool.

### DOCUMENTATION FACTS (CockroachDB MCP, this session)
- `feature.vector_index.enabled`: supported on Basic/Standard/Advanced/Self-Hosted; default **true** on stable (v26.2) / v26.1 / v25.4, **false** on v25.3 / v25.2; `SET CLUSTER SETTING feature.vector_index.enabled = true;` documented; backfill on non-empty tables needs `SET sql_safe_updates = false` and blocks writes.
- Cockroach does not support DDL in explicit transactions; schema changes run as online background jobs.
- `ccloud cluster create basic <name> <region> --cloud X --spend-limit 0`: exactly one region; no Basic multi-region CLI option (multi-region `r:n r:n` syntax is dedicated/Advanced only).
- Basic multi-region is **console-only at creation** (Add regions; max 6); **cannot add regions to a single-region cluster after creation**; cannot remove regions; primary region editable via console.
- Basic: 2 regions survive zone failure, 3 survive regional failure; geolocation routing; databases inherit cluster regions automatically.
- Basic default allowlist `0.0.0.0/0`; allowlist limit 50 rules; Basic backups every 24h, 30-day retention; CA = Let's Encrypt (system roots suffice).

---

## 10. Open Items / Assumptions

- **A1 (plan6 §0.2):** every greenfield statement assumes empty-database start.
- `serial_normalization` session-setting prefix: **resolved by the P0 gate** (§4.0.1, step 4) before migrator.go is written — if SET + whole-file `ExecContext` fails, fallback per §4.0.1 (split SET into its own exec; no shared-code change).
- Exact function names for bridge.go:726/818/912/1003 sites confirmed by earlier read (`CreateBridgeHandoffReplyIfActive`, `CreateBridgeHandoffReplyAndOutboxIfActive` at :356/:500); re-verify names at :726/:818/:912/:1003 during step 6 (names not load-bearing for the conversion).
- `crdb:cluster:create` raw task retained for dev convenience but superseded by `crdb:init` for the hackathon flow (documented in Taskfile desc).
