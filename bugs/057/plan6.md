# Bug 057 Plan v6 — Fly.io-first MVP: CockroachDB as the Flagship Deployment Profile (CockroachDB Hackathon Implementation Plan)

**Status:** Ready for implementation review
**Supersedes:** `plan5.md` (v5), `plan4.md` (v4), `plan3.md` (v3), `plan2_deepseek.md` (v2), `plan.md` (v1)
**Addressed reviews:** `plan5_review.md` (reframe — MVP for a CockroachDB hackathon, not a long-lived production architecture), plus all prior reviews (plan4_review 9.8/10, plan3_review 9.6/10, plan2_deepseek_review 9.3/10)
**Prepared:** 2026-08-02
**Scope:** Smallest implementation that demonstrates clean architecture, seamless Fly.io deployment, Cockroach-native capabilities, and an obvious path to supporting additional SQL databases. Planning document — no code changes.

---

## 0. Mission, assumptions, evidence classification

### 0.1 Mission (per plan5_review)

> Reframe this plan as an MVP implementation plan for a CockroachDB hackathon. Preserve the core architectural decisions (shared PostgreSQL implementation, retry wrapper, Fly.io-first deployment, Taskfile operator API, database portability), but aggressively remove long-term governance, enterprise operational procedures, and documentation whose primary audience is future maintainers rather than hackathon judges. Optimize for the smallest implementation that demonstrates clean architecture, seamless Fly.io deployment, Cockroach-native capabilities, and an obvious path to supporting additional SQL databases in the future.

The story the demo must tell: **bchat is a Fly.io-native application framework where CockroachDB is the flagship deployment profile, and the architecture stays portable to other SQL backends.**

### 0.2 Global assumption A1

> **A1 — Cockroach deployments start with an empty database.** Every greenfield statement in this document refers to A1. When Cockroach has production data, the greenfield-only options no longer apply. A1 is stated once here and referenced by name elsewhere.

### 0.3 Evidence classification

| Tag | Meaning | Citation required |
|-----|---------|------------------|
| **REPOSITORY FACT** | Derived from bchat source code | `file:line` |
| **DOCUMENTATION FACT** | Official CockroachDB docs via the CockroachDB MCP (stable/v26.2, v25.x) | doc name |
| **FLY FACT** | Official Fly.io docs / verified community reports | URL |
| **INFERENCE** | Logical conclusion from verified facts | must say "This is an inference" |
| **SPECULATION** | Recommendation only | must start with "I recommend" |

---

## 1. Fly.io-first architecture (the reframe)

### 1.1 The philosophical shift

plan5 read as *CockroachDB-first*. plan6 inverts the stack:

```
Application (bchat — unchanged)
        ↓
Store Driver (store.Driver interface — unchanged)
        ↓
Database Profile (SQLite | PostgreSQL | CockroachDB | future: TiDB, PlanetScale, …)
        ↓
Fly Deployment (fly_*.toml per profile — new profiles plug in without changing philosophy)
```

A **database profile** bundles four things, and only these four:

| Profile part | SQLite | PostgreSQL (live Neon) | CockroachDB (this bug) |
|---|---|---|---|
| Driver flag | `--driver=sqlite` (default) | `--driver=postgres` | `--driver=cockroach` |
| Migration dir | `store/migration/sqlite/` | `store/migration/postgres/` | `store/migration/cockroach/` |
| DSN source | computed path | `DATABASE_URL` (profile.go:97-102) | `COCKROACH_DSN` (new, D7) |
| Fly config | `fly.toml` | `fly_pg.toml` | `fly_cockroach.toml` (new) |

REPOSITORY FACT: the driver factory (`store/db/db.go`) and `profile.Driver` already carry the first two columns; the shared `store/db/postgres/` package (23 implementation files) already satisfies `store.Driver` for both PostgreSQL and CockroachDB — Cockroach is a **new profile, not new architecture**.

### 1.2 Why this stays maintainable

The shared implementation has **currently four identified divergence seams** (wording deliberately non-constraining): DDL dialect (migration files only), transaction semantics (retry wrapper, §3.4), connection protocol (none today — `simple_protocol` everywhere), and capability availability (vector provider file, existing precedent `vectordb_cockroach.go`). A feature may not enter the shared implementation via conditional-on-driver logic inside individual SQL methods; it must enter via one of these seams.

**Why a separate `store/db/cockroach/` package remains wrong (INFERENCE):** it would duplicate 23 implementation files and fork `store.Driver` conformance.

---

## 2. The CockroachDB profile — what already exists (repo facts)

Remarkably little is new. The repository already contains most of the Cockroach path:

| Asset | Location | Status |
|---|---|---|
| `CockroachVectorDB` (full `VectorDB` interface) | `vectordb_cockroach.go` (build tag `cockroach`) | Complete, runtime schema + C-SPANN index + `crdb.ExecuteTx` upserts + `<=>` search |
| Retry library | `cockroach-go/v2 v2.4.3` (go.mod:13); `crdb.ExecuteTx(ctx, *sql.DB, opts, fn)` (module cache `crdb/tx.go:299`) — matches the repo's `database/sql` usage | Present, with working precedent in the vector store |
| Vector provider switch | `vectordb.go:294` selects `CockroachVectorDB` when provider = `cockroach`; env read at `vectordb.go:121` (`LANCEDB_STORAGE_PROVIDER`) and `:131` (`COCKROACH_DSN`) | Present |
| Shared-pool wiring | `service.go:160-170`: pool shared only when `CockroachDSN == ""` or `== p.DSN`; otherwise dedicated pool | Present |
| Local single-node Docker | `scripts/docker-compose.cockroach.yml` — image `cockroachdb/cockroach:v25.2.21`, `bchat_user`/`bchat_pass`@`localhost:26257/bchat?sslmode=disable`, console :8080, healthcheck | Present |
| Operator scaffolding | Taskfile `crdb:*` targets: `build:cockroach` (228), `run:cockroach` (232), `run:cockroach:seed` (243), `crdb:check` (253), `crdb:db-check` (273), `crdb:cluster:create` (286 — already creates a Basic cluster named `hackathon-demo`), `crdb:cluster:delete` (294), `crdb:sql:shell` (302), `crdb:backup:list` (312), `crdb:ip:allow` (317), `crdb:test` (325), `crdb:docker:build` (333), `crdb:docker:run` (341) | Present; needs env fixes (§3.5) and new deploy/verify verbs (§4) |
| Cloud bootstrap script | `deploy/ccloud/setup.sh` | Present |

### 2.2 Native capabilities this profile uses — lightly (DOCUMENTATION FACT, stable/v26.2)

| Capability | Facts | Used for |
|---|---|---|
| Native `VECTOR(n)` + C-SPANN index | v25.2+; stable since v25.4, current stable v26.2; **requires `SET CLUSTER SETTING feature.vector_index.enabled = true`**; backfill on non-empty tables also needs `sql_safe_updates=false` and **blocks writes** | RAG vector search (existing provider) |
| Transaction retries | SQLSTATE 40001 requires client retry; `crdb.ExecuteTx` is the documented Go pattern | 8 tx sites (§3.4) |
| Online schema changes | Non-blocking background jobs; no table locks; DDL **not** supported in explicit transactions | Migration design (§3.2) + demo (§5) |
| Distributed SQL | Basic tier supports multi-region (select AWS/GCP regions); 2 regions survive zone failure, 3 survive regional failure; geolocation routing | Demo via 2-region cluster (§4.3, §5) |

---

## 3. Implementation — the minimal diff

### 3.1 Migration files: minimal mirror (Q&A decision 1)

**Why not "LATEST.sql only" (the review's simplification, corrected):** `GetCurrentSchemaVersion` (migrator.go:257-281) globs `migration/{driver}/*/*.sql` — versioned subdirectories only, LATEST.sql excluded — and **errors when none exist** (migrator.go:262-264). A driver directory with only `LATEST.sql` fails at boot. The migrator therefore requires ≥1 versioned directory.

**Chosen shape — minimal mirror:**

```
store/migration/cockroach/
├── LATEST.sql                      ← the real migration (applied on fresh DB, A1)
└── 0.35/00__tickets_add_internal_notes.sql   ← inert; satisfies version machinery
```

REPOSITORY FACT: postgres `0.35/` contains exactly one file. Therefore the mirror's `GetCurrentSchemaVersion` returns **0.35.1 — identical to postgres** (INFERENCE: version = `0.35.(patch+1)` = `0.35.1` for file `00__…`; both drivers compute the same max). The versioned file never executes under A1 (preMigrate applies `LATEST.sql`, then `Migrate` applies only files > latest history version — REPOSITORY FACT, migrator.go:160-207, 83-133).

**`LATEST.sql` content — postgres copy with 15 `::BIGINT` casts:**
- REPOSITORY FACT: 19 `EXTRACT(EPOCH FROM NOW())` defaults in postgres `LATEST.sql`; **15 are uncast** (lines 4, 17, 18, 43, 44, 80, 81, 98, 116, 126, 127, 138, 217, 236, 751), **4 already carry `::BIGINT`** (988, 989, 1009, 1026). DOCUMENTATION FACT: `extract → float` with no implicit float→int8 assignment → uncast defaults fail `CREATE TABLE` on Cockroach. Fix: add `::BIGINT` to the 15 uncast lines only.
- Idempotency is already present: all DDL `IF NOT EXISTS`; the only data statement uses `ON CONFLICT (tenant_id, code) DO NOTHING` (LATEST.sql:208). Verified statement-by-statement (REPOSITORY FACT).

**Parity script extension is cheap:** validate-parity.sh has a grep-based `KNOWN_DIVERGENCES` mechanism (lines 44-68) — add `0.19`…`0.34: cockroach minimal mirror (inert; version machinery only)` entries. Its schema check compares only table/index **names** in LATEST.sql (lines 170-223), so the `::BIGINT` text difference is invisible to it.

### 3.2 Migrator branch (cockroach only)

New behavior in `store/migrator.go`, keyed on `s.profile.Driver == "cockroach"`:
1. Read files from `migration/cockroach/` (base path already keyed by driver, migrator.go:210).
2. **No `Begin()`/`Commit()`** (today: `migrator.go:91` in `Migrate`, `migrator.go:179` in `preMigrate`). DOCUMENTATION FACT: Cockroach does not support DDL in explicit transactions; `autocommit_before_ddl` commits prior statements anyway — `Begin()` would be cosmetic.
3. **Prepend `SET serial_normalization = 'sql_sequence';`** to the file content, then whole-file `ExecContext` on one connection (existing `execute()` path, migrator.go:321-335, tolerates "already exists" class errors).
4. All version logic, skip logic, history upsert (only after success, migrator.go:142-149/192-199), and `ON CONFLICT` history write are unchanged.

**Why `sql_sequence`:** all model IDs are `int32` (REPOSITORY FACT, `store/agent.go`, `store/driver.go`); Cockroach's default `rowid` produces 64-bit `unique_rowid()` values that overflow int32 scans. `sql_sequence` keeps IDs in int32 range via `nextval()`.

**Atomicity — explicit, one paragraph:** failed statement N no longer rolls back 1..N-1. Safe for bchat because (a) history is written only after full success → failed boot re-runs the migration; (b) `LATEST.sql` is fully idempotent (verified, §3.1); (c) `execute()` swallows "already exists" errors; (d) under A1 only `LATEST.sql` runs at first deploy — no partial-history recovery exists in the field.

### 3.3 Driver wiring

| Change | Detail |
|---|---|
| `store/db/postgres/cockroach.go` (new) | `postgres.NewCockroachDB(profile)` — same package, shared internals, per §1.2 |
| `store/db/db.go` | add `case "cockroach"` in the factory (REPOSITORY FACT: existing switch at `store/db/db.go`) |
| `internal/profile/profile.go` | mirror the postgres branch (profile.go:97-102): `if p.Driver == "cockroach" && p.DSN == "" { p.DSN = os.Getenv("COCKROACH_DSN"); … fail-fast if empty }` |
| `bin/memos/main.go` | bind `COCKROACH_DSN`; boot migration already automatic (`main.go:97-98`) |
| `scripts/entrypoint.sh` | add `file_env "COCKROACH_DSN"` (pattern exists: `file_env "MEMOS_DSN"`, entrypoint.sh:3-27,36) |

**D7 — `COCKROACH_DSN`, no fallback to `DATABASE_URL`:** prevents cross-wiring; driver/DSN mismatch is a fail-fast startup error. `DATABASE_URL` and `COCKROACH_DSN` never coexist by design.

### 3.4 Transaction retry wrapper (native feature #1)

REPOSITORY FACT: exactly **8 `BeginTx` sites** — `store/db/postgres/bridge.go:112, 356, 500, 726, 818, 912, 1003` and `store/db/postgres/agent.go:772`. Convert them (cockroach path only) to `crdb.ExecuteTx(ctx, d.db, nil, fn)` closures (API verified in the repo's exact dependency: cockroach-go v2.4.3, `crdb/tx.go:299`; precedent `vectordb_cockroach.go:191`).

- All 8 transaction bodies are pure DB operations; side effects occur after `Commit()` in callers — a 40001 retry never double-writes (REPOSITORY FACT audit in plan5 §9.2, carried here).
- Determinism under retry: 4 sites are token/status-guarded and safe on re-read; 2 are optimistic-by-design (`MAX+1`, monotonic guard); 2 are append-only — classified in a table comment at each wrapper site.
- `isPostgresRetryable` string matching (bridge.go:95-104, 320) stays for Postgres; the Cockroach path never reaches it.
- Non-retryable errors propagate; `store.ErrBridge*` sentinels untouched.

### 3.5 Vector env fix (correctness, small)

REPOSITORY FACT: the Taskfile and `deploy/ecs/task-definition.json` set `VECTOR_DB_PROVIDER=cockroach`, but the RAG pipeline reads **`LANCEDB_STORAGE_PROVIDER`** (vectordb.go:121). `VECTOR_DB_PROVIDER` is vestigial. Fix in Taskfile `run:cockroach` (line 241), `crdb:check` (lines 263-266), `crdb:docker:run` (line 348) and ECS doc: use `LANCEDB_STORAGE_PROVIDER=cockroach`; drop the vestigial var.

### 3.6 Verification gates (Q&A decision 4)

| ID | Question | Method | Gate |
|---|---|---|---|
| P1 | Does injected `SET serial_normalization='sql_sequence'` produce `nextval(...)` defaults? | Run cockroach `LATEST.sql` via migrator; `SHOW CREATE TABLE` on ≥3 tables | **Mandatory** — 100% `nextval()`, 0 `unique_rowid()` |
| P2 | Whole-file exec idempotent on failure + re-run? | Full run; delete history row; re-run; inject failing statement mid-file | **Mandatory** — no dupes; failed run writes no history |
| P3 | `EXTRACT(...)::BIGINT` valid as a default? | `CREATE TABLE t (c BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT); INSERT; SELECT` | **Mandatory** — epoch ±1s; uncast variant fails |
| P6 | Vector path on the deployment config | `crdb:test` with `LANCEDB_STORAGE_PROVIDER=cockroach`; verify `feature.vector_index.enabled` flow | **Mandatory** — index exists; search round-trip |
| P4 | `crdb.ExecuteTx` retry under contention (1000 claims, 0 dup/lost, ≥1 40001; optional abort/disconnect cases) | concurrency harness on outbox sites | **Optional** (`--experiments` flag in deploy chain) |
| P5 | SKIP LOCKED exactly-once claims | concurrent workers on `ClaimPendingEvents` (agent.go:2773-2781) | **Optional** |

All run on the existing single-node Docker cluster (`docker-compose.cockroach.yml`, v25.2.21 — vector-index capable). `crdb:verify` (§4.4) carries the production-facing form of these pass criteria.

### 3.7 Compat scanner (kept, minimal — Q&A decision 3)

New `scripts/validate-cockroach-compat.sh` (~50 lines, grep-based, wired into `crdb:check`): exit 1 on FORBIDDEN constructs in `store/migration/cockroach/**/*.sql` (`CREATE EXTENSION`, `LISTEN/NOTIFY`, advisory locks, `CREATE DOMAIN`, ranges/`MACADDR`/`MONEY`, triggers, `DEFERRABLE`, `DROP PRIMARY KEY`, PL/pgSQL `DO` blocks — all MCP-verified unsupported), exit 2 on unannotated REVIEW-REQUIRED (`ALTER TYPE`, `UPDATE … FROM`, `COPY`). Explicitly **best-effort, not a parser** — the authoritative gate is code review and the §3.1 audit.

### 3.8 Files to create / modify

**Create:** `store/migration/cockroach/LATEST.sql` (+ `0.35/00__tickets_add_internal_notes.sql` mirror), `store/db/postgres/cockroach.go`, `scripts/validate-cockroach-compat.sh`, `fly_cockroach.toml`, `Dockerfile.cockroach.fly`, `scripts/fly-cockroach-secrets.sh`, `docs_flyio_cockroach_deploy.md`.

**Modify:** `store/db/db.go` (factory case), `store/migrator.go` (cockroach branch), `store/db/postgres/bridge.go` + `agent.go` (7+1 retry wrappers), `internal/profile/profile.go` (driver accept + DSN), `bin/memos/main.go` (DSN binding), `scripts/entrypoint.sh` (`file_env "COCKROACH_DSN"`), `scripts/validate-parity.sh` (cockroach pair + known divergences), `Taskfile.yml` (§4 targets + env fix), `deploy/ecs/task-definition.json` (doc-only env swap).

**Unchanged (explicitly):** all other `store/db/postgres/*.go` files, `store/driver.go`, `store/agent.go`, `vectordb_cockroach.go`, Neon production files (`fly_pg.toml`, `Dockerfile.pg.fly`, `fly-pg-secrets.sh`).

---

## 4. Deployment — the Taskfile as the operator API

The reviewer's "genuinely innovative" idea, kept: operators express **intent**, never infrastructure. Docker compose, `ccloud`, `fly`, and SQL shells hide behind Taskfile verbs.

### 4.1 Taskfile operator API

| Intent | Task | Internally performs |
|---|---|---|
| Local dev | `task crdb:up` / `crdb:down` / `crdb:reset` | docker compose up/down/-v on `scripts/docker-compose.cockroach.yml` |
| Local migrate | `task crdb:migrate` | boot `./build/memos --driver=cockroach --mode dev` (boot = migrate, main.go:97-98) |
| Run locally | `task run:cockroach` | existing target, env fixed (§3.5) |
| Validate | `task crdb:check` | env + DSN + compat scanner (§3.7) |
| Verify cluster | `task crdb:verify` | §4.4 checks |
| Provision cloud | `task crdb:init` | §4.3 |
| Deploy | `task deploy:cockroach` | §4.5 chain |
| Deploy Neon (demo) | `task deploy:postgres` | wraps the existing Neon flow (`fly -a bchat-pg deploy -c fly_pg.toml`) — proves "same Taskfile" |
| Smoke prod | `task verify:production` | §4.6 |
| Roll back | `task rollback:postgres` | §4.7 |
| Harden net | `task crdb:harden` | §4.8 |

**Explicitly NOT in the API:** `cockroach sql` ad-hoc commands, manual `SET` statements, raw `ccloud` invocations. Operators may use the underlying tools for diagnostics (documented in `docs_flyio_cockroach_deploy.md`).

### 4.2 Local developer workflow

```
task crdb:up        # single-node Cockroach (v25.2.21)
task crdb:migrate   # boot applies LATEST.sql
task run:cockroach  # app with COCKROACH_DSN + LANCEDB_STORAGE_PROVIDER=cockroach
```

REPOSITORY FACT: all three building blocks exist today (`docker-compose.cockroach.yml`, boot-migration, `run:cockroach`); the sequence replaces manual compose + `cockroach sql` steps.

### 4.3 Cloud bootstrap — `task crdb:init` (Q&A decision 5: 2-region Basic)

1. Validate `ccloud` CLI present + authenticated.
2. **Create 2-region Basic cluster** — regions nearest Fly's `sjc` primary (fly_pg.toml:5): `us-west-2` (primary, closest) + `us-east-1`. DOCUMENTATION FACT: Basic supports multi-region on select AWS regions; 2 regions survive zone failure. Existing `crdb:cluster:create` (Taskfile.yml:286) is parameterized to match (cluster name from env, default `hackathon-demo`). Free at $0 idle; vector-data integration is a marketed Basic capability.
3. SQL user + connection string via `ccloud quickstart`/`ccloud cluster sql --connection-url`.
4. DSN → `.env` (local) / Fly secret (prod): `postgresql://user:pass@<cluster>-<id>.<org>.crdb.cloud:26257/bchat?sslmode=verify-full` (DOCUMENTATION FACT; CA is Let's Encrypt — system roots suffice).
5. `SET CLUSTER SETTING feature.vector_index.enabled = true;` (DOCUMENTATION FACT, §2.2).
6. Ping: `SELECT version()` confirms Cockroach (guards DSN cross-wiring).
7. Output: run `crdb:verify`, then `deploy:cockroach`.

Allowlist: Basic ships with `0.0.0.0/0` (DOCUMENTATION FACT) — zero-config bootstrap. Production hardening is one command away (§4.8).

### 4.4 Verification — `task crdb:verify` (production-facing P1–P6)

1. `SELECT 1` via pgx over `COCKROACH_DSN`.
2. `SELECT version()` — Cockroach, not Postgres.
3. `migration_history` — exactly one row = `0.35.1` under A1.
4. `SHOW CREATE TABLE agent_tenants` — `nextval(...)` defaults (P1 evidence in production).
5. Vector index exists + `SHOW CLUSTER SETTING feature.vector_index.enabled` = `true`.
6. Log-grep for retry-wrapper initialization.
7. `GET /healthz` 200.

### 4.5 Fresh deployment — `task deploy:cockroach` (full chain; experiments optional)

```
build:cockroach  →  validate-parity.sh  →  validate-cockroach-compat.sh
→  (--experiments: P1–P6, optional)  →  fly -a bchat-crdb deploy -c fly_cockroach.toml
→  poll https://bchat-crdb.fly.dev/healthz (grace 15s, fly_pg.toml pattern)
→  crdb:verify steps 1–5  →  verify:production
```

The operator never thinks about `SET serial_normalization`, migration ordering, or allowlists. Neon stays live until cutover is proven (Q10: new app `bchat-crdb`, new files; Neon files untouched).

### 4.6 Production smoke — `task verify:production`

Against the deployed app's public API: create tenant → create memo → KB upload + reindex (vector insert) → RAG search (≥1 hit) → bridge handoff create/claim/complete cycle (exercises retry wrappers) → delete test data (`--destroy` default on). Any failure fails the deploy report.

### 4.7 Rollback — `task rollback:postgres`

Driver switch + redeploy, same image: set `DATABASE_URL` (Neon) secret, unset `COCKROACH_DSN`, flip `MEMOS_DRIVER=postgres` (encoded declaratively via a `fly_pg-rollback.toml` clone of `fly_pg.toml` with app `bchat-crdb`), redeploy, `verify:production`. **Rollback does not attempt schema downgrade** — CRDB data stays at its last applied schema; re-cutover is a plain forward migration. Vector behavior verified safe: with driver=postgres and `COCKROACH_DSN` unset, the app uses Neon + memory fallback provider; if `COCKROACH_DSN` is kept, `service.go:160-170` opens a dedicated CRDB pool for vectors (legitimate mixed state). RAG env resets to Neon values in the task definition.

### 4.8 Networking — default open, hardening on demand

- **Default:** `0.0.0.0/0` (Basic default) — zero-config; protection = SQL password + TLS.
- **`task crdb:harden`:** allocate Fly static egress IP (`fly ips allocate-egress`, ~$3.60/mo — FLY FACT), allowlist `<ip>/32` via `ccloud cluster networking allowlist`, remove `0.0.0.0/0`, then **mandatory connectivity verification** (FLY FACT: community reports of post-allocation breakage; if broken: release IP, restore `0.0.0.0/0`). Allowlist limit 50 rules (DOCUMENTATION FACT) — ample.

### 4.9 Fly integration — `fly_cockroach.toml` (clone of fly_pg.toml)

| Setting | Neon (`fly_pg.toml`) | Cockroach (`fly_cockroach.toml`) |
|---|---|---|
| App | `bchat-pg` (fly_pg.toml:4) | `bchat-crdb` |
| Primary region | `sjc` (:5) | `sjc` (unchanged) |
| Dockerfile | `Dockerfile.pg.fly` (:8) | `Dockerfile.cockroach.fly` — `go build -tags "cockroach rag"` |
| Driver | `MEMOS_DRIVER='postgres'` (:11) | `MEMOS_DRIVER='cockroach'` |
| DSN secret | `DATABASE_URL` | `COCKROACH_DSN` (via `scripts/fly-cockroach-secrets.sh`, modeled on `fly-pg-secrets.sh`; + `OPENROUTER_API_KEY`, `ENCRYPTION_MASTER_KEY`) |
| Vector store | `LANCEDB_STORAGE_PROVIDER='s3'` (:19) + Tigris creds | `LANCEDB_STORAGE_PROVIDER='cockroach'`; **no AWS/Tigris secrets at all** |
| Reindex | `RAG_STARTUP_REINDEX_DISABLED='true'` (:25) | `FORCE_REINDEX_ON_STARTUP='true'` for the initial A1 reindex only, then disabled (backfill blocks writes — safe pre-traffic) |
| Health | `/healthz` check (:44-48) | identical |
| VM | 1024mb shared 1cpu (:50-53) | identical |

Migration on boot: unchanged code path (`main.go:97-98`); concurrent Fly instance boots are safe — `LATEST.sql` idempotent + `ON CONFLICT` history upsert, the same property Neon relies on today.

**Backups (no code):** Cockroach-managed — Basic backs up every 24h, 30-day retention (DOCUMENTATION FACT); restore via console/API. Documented in `docs_flyio_cockroach_deploy.md`.

---

## 5. Demo — the portability story

### 5.1 The script (the strongest story)

```
task db:local             →  SQLite            →  works
task deploy:postgres      →  Fly + Neon        →  works        (same Taskfile, live today)
task deploy:cockroach     →  Fly + CRDB Cloud  →  works        (one command)
task rollback:postgres    →  back to Neon      →  works
```

Same application, same Taskfile, same Fly workflow — the database is a profile, not an architecture. This is the demo's spine; everything else is supporting evidence.

### 5.2 Cockroach-native features — demonstrated lightly (judge Q&A)

| Feature | Live demo | Under the hood |
|---|---|---|
| Automatic retries | Concurrent outbox claims during the bridge smoke test; show the 40001 retry in logs | `crdb.ExecuteTx` wrappers (§3.4), the documented Cockroach pattern |
| Native VECTOR | Upload a KB file → reindex → RAG search returns the chunk; `SHOW CREATE TABLE agent_vectors` shows `embedding VECTOR(1536)` + C-SPANN index | existing `CockroachVectorDB` (§2.1) |
| Distributed SQL | `SHOW REGIONS` on the 2-region cluster; geolocation routing note | Basic multi-region (§4.3) |
| Online schema changes | Issue `ALTER TABLE … ADD COLUMN` while a query loop runs against the app — zero blocking, background job notice | DOCUMENTATION FACT (§2.2) |

Fallback if any live step is risky on demo day: pre-recorded capture of the same steps.

---

## 6. Risks (short) + deferred to post-hackathon

### 6.1 Risks

| Risk | Mitigation |
|---|---|
| SKIP LOCKED upstream bug #167582 (may skip rows with stale intents) | claims re-scanned each poll cycle; picked up next pass (INFERENCE) |
| Migration non-atomicity | idempotent `LATEST.sql` + history-after-success + `execute()` tolerance (§3.2) |
| Vector-index backfill blocks writes | initial reindex pre-traffic under A1; provider uses single-row UPSERTs (REPOSITORY FACT) |
| Fly egress-IP breakage after allocation | `crdb:harden` verifies; fallback to `0.0.0.0/0` (FLY FACT) |
| int32 ID space (2.1B/table) | `sql_sequence` keeps values in range; int64 migration deferred |
| Concurrent boot migration | identical to Neon behavior today (§4.9) |

### 6.2 Deferred (recorded, not implemented — the reviewer's governance cut)

- Capability drift policy + CI enforcement (v5 §10) — the compat scanner (§3.7) is the retained minimal slice; full matrix + drift gates post-hackathon.
- Benchmark governance (v5 §13) — no perf gates in the MVP; `crdb:test` covers correctness.
- Migration lifecycle rules (v5 §7.5) — one line of guidance for future migrations: *Cockroach files must be idempotent; never edit an applied file; fixes ship as new files.*
- Full mirrored migration tree (v5 D3) — replaced by the minimal mirror (§3.1); a full mirror is a trivial mechanical extension if auditability becomes required.
- Split protocols (migrator simple / runtime extended), `NewSQLDriver` capability factory, `RunResiliently` wiring — unchanged from plan5 §17.
- Schema downgrade migrations — out of scope; rollback is code-version rollback, never schema downgrade (§4.7).
- Standard/Advanced tiers, private connectivity (PrivateLink/PSC) — not usable from Fly.io (no VPC peering surface — INFERENCE); upgrade path documented in `docs_flyio_cockroach_deploy.md`.

---

## 7. Decision log (condensed; Q&A decision 2 — kept)

| Decision | Why | Revisit when |
|---|---|---|
| CockroachDB = deployment profile, not architecture | portability is the demo; shared `store/db/postgres/` (23 files) stays single-source | SQL divergence exceeds the four seams (§1.2) |
| Minimal mirror (0.35/ only) | migrator requires ≥1 versioned dir (migrator.go:262-264); identical schemaVersion 0.35.1; zero shared-code change | a real migration framework arrives; full mirror needed |
| `SET serial_normalization='sql_sequence'` | **given int32 model types**, keeps IDs in int32 range | IDs become int64 |
| Whole-file exec, no `Begin()` | Cockroach DDL does not support explicit-txn DDL (DOCUMENTATION FACT) | transaction semantics change |
| `crdb.ExecuteTx` on 8 tx sites (cockroach only) | 40001 requires client retry (DOCUMENTATION FACT); proven API in-repo | Postgres gains retry → wrapper becomes shared |
| `COCKROACH_DSN`, no fallback | prevents cross-wiring; fail-fast (D7) | a unified DSN convention emerges |
| Vectors in CockroachDB (`LANCEDB_STORAGE_PROVIDER=cockroach`) | provider + shared pool already exist; native VECTOR is a demo feature | vector workload outgrows Basic |
| 2-region Basic cluster ($0 idle) | real distributed SQL demo; survives zone failure | traffic/cost/RPO needs exceed Basic |
| New Fly app `bchat-crdb`, Neon untouched | cutover proven before touching live production | cutover permanent → files merge |
| Rollback = driver switch + redeploy | binary identical; driver selected at runtime | Cockroach-only schema features diverge the binary |
| Taskfile as the operator API | the review's "genuinely innovative" piece; intent over infrastructure | CI/CD replaces manual deploys (chain moves to CI) |

---

## 8. Final verification summary (plan5_review asks → plan6 sections)

| plan5_review item | Resolution in v6 |
|---|---|
| Fly.io-first primary principle | §1 reframe: App → Driver → Database Profile → Fly Deployment |
| Cockroach as a deployment profile | §1.1 profile table; §2 inventory of what already exists |
| Demo showcases portability | §5: `deploy:postgres` → `deploy:cockroach` → `rollback:postgres`, same app/Taskfile |
| Remove governance | §6.2 deferred list; only evidence classification + condensed decision log kept (§0.3, §7) |
| Double down on Taskfile | §4.1 operator API; new `deploy:postgres` verb proves the story |
| Native features lightly | §2.2 + §5.2: retries, VECTOR, distributed SQL, online schema changes |
| Reduce migration complexity | §3.1 minimal mirror; "PG historical migrations unchanged; CRDB starts from LATEST.sql; future migrations same pattern" — with the migrator constraint (versioned dir required) documented |
| Keep Taskfile-as-deployment-API | §4 (unchanged, reviewer-approved) |
