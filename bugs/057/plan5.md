# Bug 057 Plan v5 — CockroachDB Database Support + Seamless Fly.io Deployment (Implementation Plan)

**Status:** Ready for implementation review
**Supersedes:** `plan4.md` (v4), `plan3.md` (v3), `plan2_deepseek.md` (v2), `plan.md` (v1)
**Addressed reviews:** `plan4_review.md` (9.8/10 — this revision), `plan3_review.md` (9.6/10), `plan2_deepseek_review.md` (9.3/10), `plan2_review.md`, `plan2_review_chatgpt.md`, `plan_review.md`, `plan_review_chatgpt.md`
**Prepared:** 2026-08-02
**Scope:** Production migration + runtime support for CockroachDB as a first-class driver alongside SQLite and PostgreSQL, plus the **Deployment Workflow (§22)** that makes deploying to CockroachDB feel identical to the current Neon production workflow. This is a planning document — no code changes.

**Reference:** the Neon production deployment is documented in `docs_flyio_neon_deploy.md` (active) and `docs_deployment.md`. plan4_review's verdict: *"This is now an implementation plan that I would be comfortable approving provided the proof-of-concept experiments (P1–P6) all pass… the largest remaining architectural gap is that deployment is almost absent."* §22 closes that gap.

---

## 0. Evidence classification (unchanged from v4)

| Tag | Meaning | Citation required |
|-----|---------|------------------|
| **REPOSITORY FACT** | Derived from bchat source code | `file:line` |
| **DOCUMENTATION FACT** | Derived from official CockroachDB docs via the CockroachDB MCP | doc name + version/section |
| **FLY FACT** | Derived from official Fly.io docs / verified community reports | URL |
| **INFERENCE** | Logical conclusion from verified facts | must say "This is an inference" |
| **SPECULATION** | Recommendation only | must start with "I recommend" |
| **PROHIBITED** | "Cockroach doesn't support X" unless official docs say so | — |

**Evidence rule:** every non-trivial engineering claim is backed by either (1) repository evidence, (2) official CockroachDB documentation (obtained via the CockroachDB MCP), (3) official Fly.io documentation, or (4) an explicitly labeled experiment that must run before implementation.

**Global assumption A1 (centralized, per plan4_review #2):**
> **A1 — Cockroach deployments start with an empty database.** Every greenfield statement in this document refers to A1. When Cockroach has production data, the greenfield-only options (fresh-database escalation, data disposal) no longer apply. A1 is stated once here and referenced by name elsewhere so it can be removed when it expires.

---

## 1. Background context — interactive Q&A with project lead

### 1.1 Session 1 (pre-v3, 2026-08-02) — seven binding decisions, unchanged

| # | Question | Decision (lead) |
|---|----------|-----------------|
| Q1 | Migration execution strategy | **Whole-file, no transaction wrapper.** One `ExecContext` per file on one connection; no custom SQL splitter; `Begin()` skipped for Cockroach; atomicity loss documented and mitigated (§7) |
| Q2 | `serial_normalization` application | **Inject in migrator code.** Cockroach branch prepends `SET serial_normalization = 'sql_sequence';`; SQL files stay byte-identical to Postgres where possible; PoC via `SHOW CREATE TABLE` (P1, §12) |
| Q3 | Migration layout | **Mirror versioned directories.** `store/migration/cockroach/` = `0.19/`…`0.35/` + `LATEST.sql`, file-for-file with postgres |
| Q4 | Runtime query protocol | **`simple_protocol` everywhere**, identical to the current Postgres driver; split-protocol deferred (§17) |
| Q5 | Runtime transaction retries (40001) | **`crdb.ExecuteTx` retry wrapper on the 8 `BeginTx` sites** (cockroach-only); side-effect audit included (§9) |
| Q6 | Driver construction | **`postgres.NewCockroachDB()`** in the existing postgres package |
| Q7 | Connection configuration | **`COCKROACH_DSN` env var + `--driver=cockroach`**; no fallback to `DATABASE_URL`; fail-fast if missing |

### 1.2 Session 2 (pre-v5, 2026-08-02) — seven deployment decisions, binding

| # | Question | Decision (lead) |
|---|----------|-----------------|
| Q8 | Cluster tier | **CockroachDB Cloud Basic.** Free tier, scales to zero, multi-region on select AWS/GCP regions, 50 allowlist rules, automatic managed backups. Upgrade path to Standard documented (§22.15) |
| Q9 | Networking | **Support both options: `0.0.0.0/0` is the DEFAULT** (Basic clusters ship with it; zero-config bootstrap), and **static Fly egress IP + `/32` allowlist is the supported production hardening path** (`crdb:harden`, §22.10) |
| Q10 | Fly.io app layout | **New app, new files:** `fly_cockroach.toml` + `Dockerfile.cockroach.fly` + new app name (`bchat-crdb`). Production Neon app stays untouched until cutover is proven |
| Q11 | Rollback | **Driver switch + redeploy.** `task rollback:postgres` = set `MEMOS_DRIVER=postgres` + `DATABASE_URL` secret, redeploy same image. Verified safe for the vector store (§22.13) |
| Q12 | Vector storage | **Move vectors into CockroachDB.** The deployment runs the RAG pipeline on CRDB native vectors (`LANCEDB_STORAGE_PROVIDER=cockroach`); LanceDB/S3/Tigris is retired from the Cockroach app. Implementation already exists (`vectordb_cockroach.go`) |
| Q13 | Deploy task | **Full chain, experiments optional:** `task deploy:cockroach` runs build → parity → compat → `fly deploy` → wait healthy → verify `migration_history` → smoke tests. P1–P6 remain manual (they need fault injection) |
| Q14 | Plan5 scope | **Complete standalone document**: v4 content with all plan4_review wording fixes + §22 Deployment Workflow + §23 Decision Log |

---

## 2. Methodology and evidence appendix

### 2.1 Methodology (condensed from v4)

Every CockroachDB behavior claim is a **DOCUMENTATION FACT** retrieved and verified via the CockroachDB MCP over official docs (stable/v26.2, v25.x, v24.x, v23.x) and upstream GitHub issues. Repository claims are **REPOSITORY FACTS** with `file:line`. Where sources conflict (e.g., an outdated blog claiming `SKIP LOCKED` is unsupported), the current official docs win (§2.2 row 2). Fly.io claims are **FLY FACTS** from official docs or verified community threads (§2.3).

### 2.2 Evidence appendix — CockroachDB (v4 rows retained, new rows in bold)

| Topic | Official documentation (via MCP) | Repository evidence | Decision |
|---|---|---|---|
| `SKIP LOCKED` | Supported wait policy, v23.2–v26.2; older blog outdated | `agent.go:2773-2781` single `UPDATE…RETURNING` | Keep; note upstream bug #167582 |
| `serial_normalization` | Session variable; options `rowid`/`virtual_sequence`/`sql_sequence`/`sql_sequence_cached`/`unordered_rowid`; default `rowid` = 64-bit `unique_rowid()` | All IDs `int32`; `SERIAL PRIMARY KEY` everywhere (`LATEST.sql`) | Migrator injects `SET serial_normalization='sql_sequence'` (D2); P1 proves `nextval()` |
| `EXTRACT(EPOCH FROM NOW())` | `extract(timestamp, 'epoch') → float`; no implicit float→int8 assignment | 19 defaults in `LATEST.sql`; `migration_history.go:39-45` relies on default | Explicit `::BIGINT` in Cockroach DDL (§6.2); P3 |
| DDL in explicit transactions | Mostly unsupported; `autocommit_before_ddl` commits before DDL | `migrator.go:91,178` wraps in `Begin()` today | No `Begin()` for Cockroach (§7.2) |
| Batched multi-statement simple protocol | Semicolon-separated strings as one unit support automatic retries | `migrator.go:321-335` whole-file `ExecContext` | Whole-file exec, no splitter |
| Client retry requirement | SQLSTATE 40001 requires app-level retry | `crdb.ExecuteTx` precedent `vectordb_cockroach.go:191`; `isPostgresRetryable` string hack `bridge.go:320` | `crdb.ExecuteTx` on 8 tx sites (§9) |
| Connection pooling | Active connections across all pools ≈ ≤4× cluster vCPUs | `postgres.go:42-48` 10/5/5m/1m | Preserve defaults; tune operationally (§8.3) |
| Vector indexes | No large batch inserts; no `IMPORT INTO` on vector-indexed tables | `vectordb_cockroach.go:191-218` single-row UPSERTs | No change |
| Prepared statements vs schema changes | 0A000 `cached plan must not change result type` | simple protocol everywhere | Note only |
| **Vector type + indexes (new)** | `VECTOR(n)` pgvector-compatible (24.2+; stable docs no longer "preview"); vector indexes (C-SPANN) v25.2+; **requires `SET CLUSTER SETTING feature.vector_index.enabled = true`**; backfill on non-empty table **blocks writes**; operators `<->` `<#>` `<=>` | `vectordb_cockroach.go:82-113` runtime `CREATE TABLE` + `CREATE VECTOR INDEX` | Deployment check in `crdb:verify` (§22.7); P6 |
| **IP allowlisting (new)** | Basic/Standard max **50** rules (Advanced: 20 AWS / 200 GCP+Azure); **Basic and Standard clusters ship with `0.0.0.0/0`**; Advanced is locked down; production checklist: remove `0.0.0.0/0` before prod | Taskfile `crdb:ip:allow` adds `0.0.0.0/0`; `deploy/ccloud/setup.sh` | Default `0.0.0.0/0` (Q9), `crdb:harden` swaps to egress IP (§22.10) |
| **Private connectivity (new)** | AWS PrivateLink / GCP PSC / VPC peering: Advanced all clouds, Standard preview (AWS/GCP), **Basic: none**; exists for non-static app servers | — | **Not usable from Fly.io** (no VPC peering surface — INFERENCE); egress-IP allowlist is the path |
| **Connection string (new)** | `postgresql://user:pass@<cluster>-<id>.<org>.crdb.cloud:26257/defaultdb?sslmode=verify-full`; CA is Let's Encrypt signed; `sslmode=require` works without root cert | Neon flow uses `sslmode=require` (`fly-pg-secrets.sh`) | Cockroach DSN uses `sslmode=verify-full` per prod_checklist (root cert from ccloud; system trust works for Let's Encrypt) |
| **ccloud CLI (new)** | `ccloud quickstart` (cluster+user+conn string); `cluster create basic/standard/advanced`; `cluster networking allowlist create/list/update/delete`; `cluster sql --connection-url`; `cluster backup`; `cluster connection string` | Taskfile `crdb:cluster:create`/`crdb:ip:allow`/`crdb:sql:shell`; `deploy/ccloud/setup.sh` | ccloud is the provisioning API (§22.9) |
| **Managed backups (new)** | Basic: automatic, every 24h, 30-day retention; Standard/Advanced: configurable (5m–24h, retention set once); restore = cluster-level, destination must be wiped, same plan type, cluster unavailable during restore | — | Backups = Cockroach-managed; no code (§22.15) |
| `UPDATE … FROM` | Supported v23.1+; join-output ambiguity warning | legacy `0.24/01__memo_pinned.sql` (inert on Cockroach — §7.4) | Allowed; review-required |
| `ALTER TYPE` | Supported v23.1+; `DROP VALUE` jobs non-cancellable | none | Allowed; review-required |
| `CREATE EXTENSION`, LISTEN/NOTIFY, advisory locks, `CREATE DOMAIN`, ranges/`MACADDR`/`MONEY`, triggers, `DEFERRABLE`, drop PK, PL/pgSQL `DO` blocks | Not supported / partial (v23.1–v26.2 docs) | none in repo (verified) | Forbidden in Cockroach migrations (§10) |
| INT8 OID behavior | Literal expressions report INT8 regardless of `default_int_size` | pgx stdlib + simple protocol scans by value | No change; int32 safety rests on `sql_sequence` |
| Single-node Docker bootstrap | `COCKROACH_DATABASE`/`USER`/`PASSWORD` env vars | `scripts/docker-compose.cockroach.yml` | Reuse for local dev (§22.4) |

### 2.3 Evidence appendix — Fly.io deployment facts (FLY FACT, web-verified 2026-08-02)

| Topic | Source | Fact | Consequence |
|---|---|---|---|
| Default egress | fly.io/docs/networking/egress-ips | By default, outbound IPs from Fly Machines are **unstable and may change** | Cannot allowlist CRDB Cloud by default egress |
| Static egress IPs | fly.io/docs (egress-ips; `fly machine egress-ip allocate`); community.fly.io threads | App-level static egress IPs exist since late 2024 (~$3.60/mo per region; per-machine variant $0.005/hr); survive machine migration; **not shared between machines**; some users report post-allocation connection breakage (IPv4/IPv6 listener nuances) | `crdb:harden` allocates + **verifies** (§22.10) |
| Inbound services | fly.toml in repo | `http_service` with `/healthz` check, `force_https`, ports | Cockroach app reuses the exact shape |
| Secrets | `scripts/fly-pg-secrets.sh` (repo) | `fly secrets set` is the Neon pattern | Same for `COCKROACH_DSN` (§22.11) |

---

## 3. Capability matrix

| Capability | PostgreSQL | CockroachDB | Used by bchat | Abstraction in this port |
|---|---|---|---|---|
| Transaction retry | optional | **mandatory** (40001) | yes | `crdb.ExecuteTx` wrapper on 8 `BeginTx` sites (§9) |
| Vector | pgvector (runtime) | native `VECTOR(n)` (runtime) | yes | runtime-created schema; existing `vectordb_cockroach.go` |
| Vector indexes | pgvector HNSW/IVFFlat | **C-SPANN, v25.2+; needs `feature.vector_index.enabled`** | yes | runtime `CREATE VECTOR INDEX`; verify at deploy (§22.7) |
| SERIAL / identity | sequence | configurable via `serial_normalization` | yes | migrator-injected `SET` (§7.4) + P1 |
| JSONB | yes | yes | yes | shared |
| `FOR UPDATE SKIP LOCKED` | yes | yes (v23.2+) | yes | shared (note #167582) |
| `EXTRACT(EPOCH FROM NOW())` defaults | numeric → assignment cast | **float** (no implicit int cast) | yes | explicit `::BIGINT` in Cockroach DDL (§6.2) |
| `RETURNING`, `ON CONFLICT`, `ILIKE`, `jsonb @>` | yes | yes | yes | shared |
| `UPDATE … FROM` | yes | yes (v23.1+) | legacy only | allowed; drift-check review-required |
| Multi-statement simple-protocol exec | yes | yes (batched) | yes (migrations) | whole-file exec, no tx wrapper |
| DDL in explicit transactions | yes | **no** (mostly) | n/a | no `Begin()` for Cockroach |
| Atomic whole-file migration | yes | **no** | n/a | documented loss + idempotency mitigations (§7.3) |
| LISTEN/NOTIFY, advisory locks, triggers, `CREATE DOMAIN`, ranges | yes | **no** | no | forbidden by drift policy (§10) |
| Managed backups | Neon-side | **Cockroach-managed (Basic: 24h, 30d)** | n/a | console/API; §22.15 |

---

## 4. Why the shared implementation remains maintainable (plan3_review #1, wording per plan4_review #3)

The shared implementation has **currently four identified divergence seams** (not "exactly four" — architecture evolves, and this wording deliberately does not constrain future contributors):

| Seam | What differs | Where it lives | Evolution path |
|---|---|---|---|
| DDL dialect | `::BIGINT` casts, `SET serial_normalization`, idempotency | `store/migration/cockroach/` files only | new migration file per change; postgres files untouched |
| Transaction semantics | 40001 retry required | retry wrapper around 8 tx sites (cockroach branch) | if Postgres gains retry, wrapper becomes shared |
| Connection protocol | none today (`simple_protocol` everywhere) | DSN builder in `postgres.go` | per-driver DSN options already parameterized |
| Capability availability | vector provider | separate provider file, `vectordb_cockroach.go` (existing precedent) | new providers are new files, never new driver packages |

**Boundary rule** (unchanged): a feature may not enter the shared implementation via conditional-on-driver logic inside individual SQL methods; it must enter via one of the seams above, with the documented capability-interface escalation (§17).

**Why a separate `store/db/cockroach/` package remains wrong (INFERENCE):** it would duplicate 23 implementation files and fork `store.Driver` conformance — the C-1/C-2 blocker from the v2 review.

---

## 5. Architecture overview and decisions

```
profile.Driver == "cockroach"
        │
        ├── store/db/db.go factory ──► postgres.NewCockroachDB(profile)   (new, same package as NewDB)
        │                                   │
        │                                   ├── shared DSN builder (postgres.go) — COCKROACH_DSN, simple_protocol
        │                                   ├── shared pool settings (postgres.go) — 10/5/5m/1m + 60s ping
        │                                   └── shared SQL implementation files (23 files, unchanged)
        │
        ├── store/migrator.go ──► migration/cockroach/{0.19…0.35,LATEST.sql}  (mirrors postgres dirs)
        │         └── Cockroach branch: no Begin(); inject SET serial_normalization; whole-file Exec
        │
        ├── runtime retries ──► crdb.ExecuteTx wrapper around 8 BeginTx sites (cockroach-only)
        │
        └── RAG vectors (Q12) ──► LANCEDB_STORAGE_PROVIDER=cockroach → CockroachVectorDB
                    └── shares the app driver pool when DSNs match (service.go:158-166)
```

- REPOSITORY FACT: driver factory switch at `store/db/db.go`; `NewDB` at `store/db/postgres/postgres.go:22`; appends `default_query_exec_mode=simple_protocol` (line 33); `MaxOpenConns(10)` (line 42), `MaxIdleConns(5)`, `ConnMaxLifetime(5m)`, `ConnMaxIdleTime(1m)`, 60s ping.

### 5.1 D1 — Whole-file migration execution, no explicit transaction (Q1)
REPOSITORY FACT: `store/migrator.go:321-335` — `execute()` runs the entire file via one `ExecContext`; no splitter exists. DOCUMENTATION FACT: simple-protocol multi-statement strings support automatic retries; most DDL cannot run in explicit transactions and `autocommit_before_ddl` commits before DDL regardless. INFERENCE: `Begin()` adds no atomicity on Cockroach and misleads readers; the Cockroach branch drops it.

### 5.2 D2 — `SET serial_normalization = 'sql_sequence'` injected by the migrator (Q2; wording per plan4_review "One thing I would still challenge")
> **Given the current repository model types (`int32` IDs), `sql_sequence` is the chosen compatibility strategy.**

DOCUMENTATION FACT: `serial_normalization` is a session variable controlling SERIAL handling; default `rowid` yields 64-bit `unique_rowid()` values. REPOSITORY FACT: all IDs are `int32` (`store/agent.go`, `store/driver.go`). The underlying requirement is **int32 compatibility**, not `sql_sequence` per se: if the repository later migrates to int64 IDs, the strategy changes (recorded in §23 Decision Log). Inference: `sql_sequence` keeps IDs in int32 range (sequences start at 1).

### 5.3 D3 — Mirror versioned migration directories (Q3; justification per plan4_review #1)
**Why mirror the historical migration tree if Cockroach never executes those files?** Three reasons, stated explicitly so future maintainers do not delete them:
1. **Version machinery** — `GetCurrentSchemaVersion` scans `*/*.sql` to compute the schema version (REPOSITORY FACT: `migrator.go:257`); a partial tree yields a wrong version and breaks `preMigrate`.
2. **Parity validation** — `validate-parity.sh` cross-checks file lists across drivers; a missing mirror is a CI failure, not a warning.
3. **Future auditability** — the mirrored tree documents, file-for-file, what bchat's schema evolution looked like, so a future Cockroach 0.36+ migration can be written with full historical context.
REPOSITORY FACT: base path `migration/{driver}/` (`migrator.go:210`); version parse (line 300); history upsert only after success (lines 142-149, 192-199). INFERENCE: mirrored `store/migration/cockroach/` works with zero migrator-structure changes beyond the Cockroach branch.

### 5.4 D4 — simple_protocol everywhere (Q4)
REPOSITORY FACT: `postgres.go:33` forces simple protocol today. INFERENCE: identical runtime behavior, zero protocol coupling. Split-protocol deferred with a benchmark gate (§17).

### 5.5 D5 — `crdb.ExecuteTx` retry wrapper on 8 tx sites (Q5)
REPOSITORY FACT: `cockroach-go/v2 v2.4.3` in `go.mod`; `ExecuteTx(ctx, *sql.DB, opts, fn)` exists (module cache `crdb/tx.go:299`); precedent `vectordb_cockroach.go:191`; `isPostgresRetryable` string-matching at `bridge.go:320` does not match CRDB 40001 messages. INFERENCE: SQLSTATE-based retry supersedes the string hack on the Cockroach path.

### 5.6 D6 — `postgres.NewCockroachDB()` (Q6)
Same package, shared internals (per §4).

### 5.7 D7 — `COCKROACH_DSN`, no fallback (Q7)
REPOSITORY FACT: `internal/profile/profile.go` carries `Driver`/`DSN` (postgres branch reads `DATABASE_URL` at lines 97-101); `bin/memos/main.go` binds them via viper. INFERENCE: explicit DSN prevents cross-wiring; fail-fast at startup. Existing Cockroach tooling reused: `scripts/docker-compose.cockroach.yml`, Taskfile `build:cockroach`/`run:cockroach`/`crdb:*` targets.

---

## 6. SQL portability audit (construct → usage → support → action)

| Construct | Usage in bchat | Cockroach support | Action |
|---|---|---|---|
| `SERIAL PRIMARY KEY` | every table, `LATEST.sql` | supported; mode = session var | inject SET (D2); prove via P1 |
| `EXTRACT(EPOCH FROM NOW())` BIGINT defaults | 19× in `LATEST.sql`; runtime relies on default (`migration_history.go:39-45`) | `extract → float`; no implicit float→int8 assignment | `::BIGINT` in Cockroach DDL; prove via P3 |
| `FOR UPDATE SKIP LOCKED` | `agent.go:2773-2781` (single statement, no tx) | supported (v23.2+) | none (note #167582) |
| `FOR UPDATE` | `bridge.go:356,500` (in tx) | supported | none (retry wrapper handles 40001) |
| `RETURNING` | ~20+ sites | supported | none |
| `ON CONFLICT` | `LATEST.sql:208`; `migration_history.go:39` | supported | none |
| `ILIKE`, `jsonb` + `@>` | `memo_filter.go` | supported | none |
| `::BOOLEAN` / `::BIGINT` casts | `memo.go:106-115`; `agent.go:2605,2693,2718,2776,2780` | supported (explicit casts) | none |
| `UPDATE … FROM` | legacy `0.24/01__memo_pinned.sql` (never runs on Cockroach — §7.4) | supported (v23.1+) | allowed; review-required in drift check |
| `CREATE VECTOR INDEX` | `vectordb_cockroach.go:109` (runtime) | supported; **no `IF NOT EXISTS`**; needs cluster setting | runtime-only; verify at deploy (§22.7) |
| `INSERT … $6::VECTOR` / `<=>` | `vectordb_cockroach.go:194,323-327` | supported | none |
| DDL in explicit tx | `migrator.go:91,178` | not supported (mostly) | no `Begin()` for Cockroach |
| Forbidden constructs (drift check) | none found in repo (verified) | see §10 | CI gate |

**Implementation step 6.1 (audit gate):** a full statement-by-statement `diff` between every `store/migration/postgres/**/*.sql` file and its Cockroach mirror must be performed in Phase 2. The only anticipated textual differences: `::BIGINT` on the 19 EXTRACT defaults.

---

## 7. Migration execution design

### 7.1 Current behavior (Postgres, unchanged)
REPOSITORY FACT (`store/migrator.go`): `preMigrate` applies `LATEST.sql` inside `Begin()` when no history exists (lines 160-207); `Migrate` applies versioned files `> latestMigrationHistoryVersion` inside `Begin()` (lines 62-150); `execute()` tolerates "duplicate column"/"already exists" errors (lines 321-335); history upsert only after batch success (lines 142-149).

### 7.2 New behavior (Cockroach)
1. Read files from `migration/cockroach/` (D3).
2. No `Begin()`/`Commit()`.
3. Prepend `SET serial_normalization = 'sql_sequence';` (D2) to the file content, then `db.ExecContext` the batch on one connection.
4. All version logic, skip logic, history upsert, and idempotent tolerance unchanged.

DOCUMENTATION FACT: sending the batch as one unit over the simple protocol allows statement-level automatic retries; DDL executes as individual online schema changes with `autocommit_before_ddl` committing prior statements first.

### 7.3 Atomicity — explicit discussion (unchanged from v4)
- **Changed vs Postgres:** failed statement N no longer rolls back statements 1..N-1.
- **Required by Cockroach:** online schema changes do not support all-or-nothing multi-statement DDL; keeping `Begin()` would be cosmetic.
- **Safe for bchat because:**
  1. `migration_history` is written only after full success (`migrator.go:142-149`) → failed boot re-runs the migration.
  2. All DDL is `IF NOT EXISTS`; the only data statement is `ON CONFLICT (tenant_id, code) DO NOTHING` (`LATEST.sql:201-208`). **`LATEST.sql` is fully idempotent** — verified statement-by-statement (REPOSITORY FACT).
  3. `execute()` swallows "already exists" class errors (`migrator.go:326-330`).
  4. Under A1, fresh Cockroach deployments run `LATEST.sql` only (§7.4), so no partial-history recovery exists in the field at first deploy.

### 7.4 Which files actually execute on Cockroach (repository-derived)
REPOSITORY FACT (`migrator.go`): on a fresh database, `preMigrate` applies `LATEST.sql`, then upserts history = `schemaVersion` (0.35). `Migrate` then applies only versioned files **greater than** the latest history version. Therefore:
- **Under A1, only `LATEST.sql` executes.** The versioned dirs `0.19/`…`0.35/` never execute; they exist solely for version machinery and parity validation (D3 justification).
- **On future upgrades (0.36+): only the new files execute.**
- Consequences (INFERENCE):
  1. Legacy data-backfill statements in 0.19–0.24 (e.g., the random-UUID `UPDATE` at `0.19/00__add_resource_name.sql:3` — non-idempotent) are **inert on Cockroach**; mirrored file-for-file for parity but never run.
  2. The idempotency rule (§7.5) governs only future Cockroach migrations (0.36+), where recovery actually applies.

### 7.5 Recovery after partially applied migration (unchanged; lifecycle rule enforced)
Scenario: future incremental migration `0.36/00__x.sql` fails at statement 4; statements 1–3 applied; history row NOT written; developer edits the file; redeploy.

**Recovery procedure (ordered):**
1. **Detect:** boot fails with the migration error; `migration_history` has no 0.36 row (REPOSITORY FACT: upsert only after success). Logs identify the failing statement.
2. **Assess:** determine which statements applied via `SELECT * FROM migration_history;` plus `SHOW TABLES` / `information_schema` inspection (`crdb:sql:shell` target).
3. **Two allowed paths:**
   - **Path A — idempotent file (default):** fix the failing statement in the unreleased file, redeploy. All applied statements re-execute safely because the file is idempotent (§7.3 items 2-3). Required norm for all Cockroach migrations.
   - **Path B — non-idempotent data transform:** leave the file as-is; add a NEW file `0.36/01__fix.sql` that compensates or completes the transform; redeploy. Editing applied files is prohibited.
4. **Escalation — A1 only:** because Cockroach has no production data at first deploy, a stuck migration can also be resolved by dropping and re-creating the database, then redeploying. Not available once production data exists.

**Migration file lifecycle rule (enforced by §10 drift policy and code review):**
- Files that have never succeeded (no history row) may be edited freely within the same development cycle.
- Once a file has a history row, **never edit it** — all fixes ship as new files (Path B).
- Every Cockroach migration must be idempotent: DDL with `IF NOT EXISTS`, inserts with `ON CONFLICT DO NOTHING`, transforms with deterministic, WHERE-guarded values. The 0.19 random-UUID pattern is the canonical counter-example and is banned.

---

## 8. Runtime driver design

### 8.1 Constructor and factory
New: `postgres.NewCockroachDB(profile *profile.Profile) (store.Driver, error)`; new factory case in `store/db/db.go`. REPOSITORY FACT: `store/driver.go` interface needs no changes — the same 23 implementation files already satisfy it.

### 8.2 DSN
`COCKROACH_DSN` env var (D7), no fallback. Reuse the DSN builder (`postgres.go:33` appends `simple_protocol`). Local: `scripts/docker-compose.cockroach.yml` credentials, `sslmode=disable`. Production (Fly.io + CockroachDB Cloud): `postgresql://user:pass@host:26257/bchat?sslmode=verify-full` per `bugs/057/prod_checklist.md`; CA cert available via `ccloud quickstart`/Connect dialog (§2.2 connection-string row). Fail-fast when `--driver=cockroach` and DSN empty.

### 8.3 Pool settings
- **Decision:** preserve existing defaults — `MaxOpenConns(10)`, `MaxIdleConns(5)`, `ConnMaxLifetime(5m)`, `ConnMaxIdleTime(1m)`, 60s ping (REPOSITORY FACT: `postgres.go:42-48`).
- **Rationale reframed (per plan3_review #9):** these are **operational tuning parameters, not architecture**. DOCUMENTATION FACT: active connections across all pools should not greatly exceed ~4× cluster vCPUs. The correct validation point is the production cluster sizing exercise at deploy time (Phase 8), where `MaxOpenConns` is adjusted via configuration if the Cloud cluster is small. No code-level sizing logic is introduced.

### 8.4 Vector database (expanded per Q12 — vectors move into CockroachDB)
REPOSITORY FACT: `CockroachVectorDB` (`vectordb_cockroach.go`, build tag `cockroach`) implements the full `VectorDB` interface; selected when `StorageProvider == "cockroach"` (`vectordb.go:294`); `NewVectorDBConfigFromEnv` reads `StorageProvider` from **`LANCEDB_STORAGE_PROVIDER`** (REPOSITORY FACT: `vectordb.go:121`) with `COCKROACH_DSN` (line 131); runtime `CREATE TABLE` + `CREATE VECTOR INDEX` (lines 82-113); `crdb.ExecuteTx` upserts (lines 191-218); `<=>` search (lines 323-327). No changes to the provider.

**Shared-pool wiring (REPOSITORY FACT: `service.go:158-166`):** the pool is shared only when `CockroachDSN == ""` or `CockroachDSN == p.DSN`. In the Cockroach deployment (driver=cockroach, DSN=COCKROACH_DSN) one pool serves app + vectors. On rollback to Neon (driver=postgres, DSN=Neon ≠ COCKROACH_DSN) the vector store opens its own dedicated pool — verified safe (§22.13).

**Env-var discrepancy to fix (REPOSITORY FACT):** the Taskfile and `deploy/ecs/task-definition.json` set `VECTOR_DB_PROVIDER=cockroach`, but the RAG pipeline never reads `VECTOR_DB_PROVIDER` — it reads `LANCEDB_STORAGE_PROVIDER`. The Cockroach deployment config must set `LANCEDB_STORAGE_PROVIDER=cockroach`; `VECTOR_DB_PROVIDER` is vestigial and is cleaned up from Taskfile/ECS docs as part of Phase 6 (§15).

**Cluster setting (DOCUMENTATION FACT):** creating vector indexes requires `SET CLUSTER SETTING feature.vector_index.enabled = true` (v25.2+). Admin-level; applied once via `crdb:init` and verified by `crdb:verify` (§22.7/§22.9).

**Reindex caveats (DOCUMENTATION FACT):** large batch inserts of `VECTOR` degrade performance; creating a vector index on a non-empty table backfills with writes blocked. Mitigation: the provider uses single-row UPSERTs (no change); the deployment's initial reindex runs before production traffic (A1) and with `FORCE_REINDEX_ON_STARTUP` controlled (§22.11).

---

## 9. Transaction retry design (unchanged from v4; P4 extended per plan4_review #6)

### 9.1 Inventory of explicit transactions

| # | Site | Function | Transaction body |
|---|---|---|---|
| 1 | `bridge.go:112` | `createBridgeHandoffAttempt` | UPDATE lock, COUNT, MAX+1, INSERT; post-commit read |
| 2 | `bridge.go:356` | `CreateBridgeHandoffReplyIfActive` | `SELECT … FOR UPDATE`, INSERT, idempotent constraint branch |
| 3 | `bridge.go:500` | `CreateBridgeHandoffReplyAndOutboxIfActive` | `FOR UPDATE`, INSERT reply, INSERT outbox |
| 4 | `bridge.go:726` | `ClaimPendingBridgeReplyOutbox` | batch claim UPDATEs on status/expiry conditions |
| 5 | `bridge.go:818` | `CompleteClaimedBridgeReplyOutbox` | SELECT, UPDATE, idempotent branch |
| 6 | `bridge.go:912` | `FailClaimedBridgeReplyOutbox` | SELECT, UPDATE, idempotent branch |
| 7 | `bridge.go:1003` | `ClaimBridgeReplyOutboxByOutboxID` | SELECT, claim UPDATE |
| 8 | `agent.go:772` | `CreateAgentMessages` | batch INSERT loop |

Plus non-transactional: `agent.go:2773-2781` `ClaimPendingEvents` — single `UPDATE … FOR UPDATE SKIP LOCKED … RETURNING` (server-retried implicit transaction; no wrapper needed).

### 9.2 Side-effect audit
All 8 transaction bodies are pure DB operations; external effects (websocket delivery, outbox processing, HTTP responses) occur after `Commit()` in callers. `uuid.NewString()` regenerates harmlessly on retry. A code-review checklist item enforces "no I/O inside tx closures" (§18).

### 9.3 Determinism classification under retry
Mechanical fact: a 40001 aborts and rolls back the failed attempt's writes, so **a retried closure never double-writes**. The only variability across attempts is that *reads may observe newer committed state*. Classification per site:

| Site | Classification | Why it is safe |
|---|---|---|
| 1 `createBridgeHandoffAttempt` (112) | **optimistic by design** | `MAX(generation)+1` recomputed on retry; monotonic guard; conflict errors preserved |
| 2 `CreateBridgeHandoffReplyIfActive` (356) | **may observe newer state; guard-safe** | re-reads `FOR UPDATE` row; if deactivated meanwhile, returns `ErrBridgeHandoffConflict` — correct |
| 3 `CreateBridgeHandoffReplyAndOutboxIfActive` (500) | **may observe newer state; guard-safe** | same pattern + append-only outbox insert |
| 4 `ClaimPendingBridgeReplyOutbox` (726) | **may observe newer state; guard-safe** | claim UPDATEs guarded by `status`/`expiry`; rows claimed by others excluded on re-read |
| 5 `CompleteClaimedBridgeReplyOutbox` (818) | **deterministic under retry** | UPDATE guarded by `claim_token`; idempotent completion branch |
| 6 `FailClaimedBridgeReplyOutbox` (912) | **deterministic under retry** | same token-guarded pattern with idempotent fail branch |
| 7 `ClaimBridgeReplyOutboxByOutboxID` (1003) | **may observe newer state; guard-safe** | claim UPDATE guarded by status conditions |
| 8 `CreateAgentMessages` (772) | **deterministic under retry** | append-only INSERTs; failed attempt's inserts rolled back |

This classification is recorded in a table comment at each wrapper site during implementation.

### 9.4 Wrapper mechanics (cockroach-only)
- Convert the 8 sites to `func(tx *sql.Tx) error` closures run via `crdb.ExecuteTx(ctx, d.db, nil, fn)` (API: module cache `crdb/tx.go:299`, v2.4.3; precedent `vectordb_cockroach.go:191`).
- `crdb.ExecuteTx` detects SQLSTATE 40001/40003/08xxx and re-runs with backoff (DOCUMENTATION FACT).
- Non-retryable errors propagate; `store.ErrBridge*` sentinels untouched.
- `isPostgresRetryable` loop at `bridge.go:95-104` stays for Postgres; the Cockroach path does not reach it.
- `RunResiliently` (`resilience.go`, zero callers — REPOSITORY FACT) stays unused; deferred (§17).

---

## 10. Capability drift policy (per plan4_review #4: grep scanner is explicitly best-effort)

Goal: keep Postgres ↔ Cockroach divergence continuous and documented, not a one-time audit.

### 10.1 Rules
1. **Every new SQL feature** must update the capability matrix (§3) in the same change.
2. **Every new migration file** must pass the portability audit (§6) and the drift check below before merge.
3. **CI rejects undocumented divergence**: a file cannot add a FORBIDDEN construct at all; a REVIEW-REQUIRED construct requires an explicit `--verified` annotation in the capability matrix.
4. **Never edit an applied migration file** (§7.5 lifecycle rule).

### 10.2 Drift-check construct list (MCP-verified, §2.2)

**FORBIDDEN in Cockroach migration files (CI fails):** `CREATE EXTENSION`; `LISTEN`/`NOTIFY`/`UNLISTEN`; advisory lock functions; `CREATE DOMAIN`; range types/`MACADDR`/`MONEY`; triggers; `DEFERRABLE`/`INITIALLY DEFERRED`; `DROP PRIMARY KEY`; PL/pgSQL blocks (`DO $$`, function bodies) — migrations must be plain SQL.

**REVIEW-REQUIRED (CI flags for human sign-off, then annotate the matrix):** `ALTER TYPE` (supported v23.1+; `DROP VALUE` jobs non-cancellable); `UPDATE … FROM` (join-output ambiguity); `COPY` (`IMPORT INTO` excluded on vector-indexed tables); `SELECT *` in any statement that could become prepared (0A000 risk); non-idempotent data transforms.

### 10.3 CI implementation (Phase 6)
New `scripts/validate-cockroach-compat.sh`: grep-based scanner over `store/migration/cockroach/**/*.sql` matching the lists above; exit 1 on FORBIDDEN, exit 2 on unannotated REVIEW-REQUIRED. Wired into `Taskfile.yml` (`crdb:check` extension) and the validate-parity flow.

**Scope statement (per plan4_review #4):** this scanner is explicitly **best-effort, not a parser**. It catches named constructs via documented regexes and cannot guarantee exhaustiveness (mirroring the existing `validate-parity.sh` limitation, line 235). The authoritative gate is the §6.1 statement-by-statement audit at Phase 2 and code review for every future migration file.

---

## 11. Rollback strategy (per plan4_review #7: "Rollback does not attempt schema downgrade" — stated)

**Property: Bug 057 is additive-only.** No behavior change to Postgres or SQLite drivers; no postgres migration file changes; `postgres.go` DSN/pool behavior unchanged.

**Rollback scenarios:**
1. **Pre-deploy (build time):** reverting the change = reverting the commit. The feature is compiled in via the `cockroach` build tag and driver flag; other builds unaffected.
2. **Post-deploy, runtime bug in Cockroach path:** switch `--driver` back to `postgres` on the same binary and restart (encoded as `task rollback:postgres`, §22.13). No migration changes needed because Postgres migrations were never touched. Cockroach data is simply unused.
3. **Cockroach data disposal:** under A1, no production data exists at first rollout; if abandoned, the cluster/database is dropped — no export or backfill obligations.
4. **Future incremental migrations:** because applied files are never edited (§7.5), rollback of a released schema version means deploying the previous code version; the database stays at its last successful history version.

**Explicit statement (per plan4_review #7):** *Rollback does not attempt schema downgrade.* bchat does not ship downgrade migrations today, and Cockroach's online schema changes make `ALTER … DROP` rollbacks a per-incident decision, not an automated path. Downgrades are out of scope (§17).

---

## 12. Experiments — VERIFY-FIRST gates with exit criteria (P4 extended per plan4_review #6)

All run on single-node Docker (`scripts/docker-compose.cockroach.yml`) before implementation. Pass criteria are measurable.

| ID | Question | Method | Pass criteria (all must hold) |
|---|---|---|---|
| P1 | Does injected `SET serial_normalization='sql_sequence'` produce `nextval(...)` defaults through the whole-file path? | Run Cockroach `LATEST.sql` (SET-prefixed) via the migrator; `SHOW CREATE TABLE` on ≥3 tables incl. `agent_tenants` and `migration_history` | 100% of SERIAL columns show `nextval('…_seq')` defaults; **0** `unique_rowid()` occurrences; migration_history row written |
| P2 | Does whole-file exec migrate cleanly, and is failure+re-run idempotent? | Full `LATEST.sql` run; delete history row; re-run; then inject a failing statement mid-file and re-run | Re-run after history deletion is a no-op (no errors, no dupes — `tenant_role_templates` stays at 5 rows); failed run writes no history row; corrected re-run completes |
| P3 | Is `EXTRACT(EPOCH FROM NOW())::BIGINT` valid and correct as a default? | `CREATE TABLE t (c BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT); INSERT INTO t DEFAULT VALUES; SELECT c;` | Insert succeeds; `c` = unix epoch seconds ±1; uncast variant fails with a type error |
| P4 | Does `crdb.ExecuteTx` retry correctly under contention? | 2–4 concurrent claimers over 1000 outbox rows; force conflicts; **additionally force (a) an explicit transaction abort (kill a tx mid-flight via `CANCEL QUERY`) and (b) a network disconnect (kill the client conn / restart Docker node)** — these exercise different retry paths per plan4_review #6 | **1000 claims, 0 duplicated rows, 0 lost rows**; ≥1 SQLSTATE 40001 observed; every retry eventually succeeds; completion count = 1000; abort and disconnect cases also end with exactly-once claims |
| P5 | SKIP LOCKED claim correctness under concurrency | Multiple workers on `ClaimPendingEvents` (`agent.go:2781`) over 1000 events | Each event claimed exactly once (0 double-claims, 0 lost); workers observe different batches |
| P6 | Vector path regression on the deployment config | Existing `crdb:test` suite with `LANCEDB_STORAGE_PROVIDER=cockroach` + `COCKROACH_DSN`; verify `feature.vector_index.enabled` path: `CREATE VECTOR INDEX` succeeds; search round-trips | All tests green; vector index exists; insert/search round-trip latency within 2× of pre-change baseline (informational, not a gate) |

---

## 13. Benchmarks — regression detection only (threshold per plan4_review #5)

Not optimization; the goal is a documented baseline so future changes can detect regressions. A `crdb:bench` Taskfile target runs the suite on the local Docker cluster and records results in `build/bench/`:

| Benchmark | Metric | Method | Baseline note |
|---|---|---|---|
| Migration runtime | wall time for fresh `LATEST.sql` | `time ./build/memos --driver=cockroach --mode dev` on empty DB | compare vs Postgres on same host (informational) |
| CreateAgentMessages TPS | inserts/sec, p99 latency | Go benchmark wrapping `CreateAgentMessages` (site 8), N=1000 | Postgres vs Cockroach |
| Bridge outbox throughput | claim→complete cycles/sec | benchmark wrapping §9.1 sites 4–7 flow | Postgres vs Cockroach |
| Vector search latency | p99 of `<=>` top-K query | reuse existing vector tests with 1k vectors | Cockroach-only |

**Regression definition (per plan4_review #5):** any subsequent benchmark showing **>20% degradation vs the recorded baseline triggers manual review** — the change is investigated and either accepted with a documented reason or reverted. Benchmarks are advisory: no perf gate in CI, and no changes to the `simple_protocol` decision result from them without a follow-up decision (§17).

---

## 14. Adversarial pass — every behavioral change defended (wording fixes per plan4_review #8)

### 14.1 Migrations: explicit-transaction → autocommit whole-file
1. **Changed:** a migration file is no longer atomic.
2. **Required:** online schema changes do not support multi-statement DDL transactions.
3. **New failure modes:** partial application → mitigated by idempotent `LATEST.sql` (verified), `execute()` tolerance, history-after-success, and §7.5 recovery procedure.

### 14.2 Migrations: injected `SET serial_normalization`
1. **Changed:** none on Postgres (cockroach-only branch).
2. **Required:** **repository scans into `int32`**; `unique_rowid()` produces 64-bit values that cannot be scanned. (Rephrased per plan4_review #8: the issue is the repository's int32 scan, not a claim that `unique_rowid()` is inherently wrong — it is Cockroach's default and perfectly fine for int64 models.)
3. **New failure modes:** SET silently not applying → gated by P1; one `nextval` round-trip per insert (negligible at bchat volumes — INFERENCE); `sql_sequence_cached` is the documented alternative if ever needed.

### 14.3 DDL: explicit `::BIGINT` casts
1. **Changed:** Cockroach files differ textually (19 defaults); Postgres uses numeric→bigint assignment cast (rounds), Cockroach uses explicit float→int8 cast (truncates). ≤1s deviation possible at .5s fractional boundaries.
2. **Required:** `extract → float` with no implicit int assignment; uncast DDL fails at CREATE TABLE.
3. **New failure modes:** none beyond DDL validation, gated by P3; runtime `migration_history.go:39` needs no change (uses the default).

### 14.4 Runtime: retry closures on 8 tx sites
1. **Changed:** 40001 re-executes the closure instead of propagating.
2. **Required:** client-side retry handling is documented as required for 40001.
3. **New failure modes:** duplicate side effects → none found (§9.2); behavioral variability on retry → classified per site (§9.3), all safe.

### 14.5 Driver selection/config
1. **Changed:** new driver name + DSN source; no behavior change to other drivers.
2. **Required:** distinct DSN prevents cross-wiring (D7).
3. **New failure modes:** misconfiguration → fail-fast startup error.

### 14.6 Protocol: simple everywhere
1. **Changed:** none vs. today's Postgres driver.
2. **Required:** no — conservative parity choice; split deferred (§17).
3. **New failure modes:** none.

### 14.7 Runtime-created vector schema + deployment-wide vectors (Q12)
1. **Changed:** none in the provider; the deployed environment now points the RAG pipeline at CRDB (`LANCEDB_STORAGE_PROVIDER=cockroach`) instead of LanceDB S3.
2. **Required:** Q12 (lead decision); provider already supports it; shared-pool wiring verified (`service.go:158-166`).
3. **New failure modes:** `feature.vector_index.enabled` unset → `CREATE VECTOR INDEX` fails → caught by `crdb:verify`/P6 (never silent); vector-index backfill blocking writes → initial reindex happens pre-traffic under A1; batch-insert degradation → provider uses single-row UPSERTs.

### 14.8 Migration file lifecycle rule
1. **Changed:** a governance rule (no code change): applied files are immutable.
2. **Required:** Cockroach's partial application makes in-place edits dangerous; Postgres atomicity makes them retroactive.
3. **New failure modes:** forced "compensation file" pattern adds one extra file per fix — accepted cost; prevents silent divergence.

---

## 15. Files to create / modify (v5 = v4 list + deployment artifacts)

### Create — core (v4, unchanged)
| File | Purpose |
|---|---|
| `store/migration/cockroach/LATEST.sql` | Full schema; mirror of postgres with `::BIGINT` on 19 EXTRACT defaults (§6.2) |
| `store/migration/cockroach/0.19/00__*.sql` … `0.35/*.sql` | File-for-file mirrors (parity; inert on fresh deploy — §7.4) |
| `store/db/postgres/cockroach.go` | `NewCockroachDB` constructor |
| `scripts/validate-cockroach-compat.sh` | Drift-check scanner (§10.3) |

### Create — deployment (new in v5)
| File | Purpose |
|---|---|
| `fly_cockroach.toml` | Fly app config; clone of `fly_pg.toml` with `MEMOS_DRIVER='cockroach'`, no S3/LanceDB env, `app = 'bchat-crdb'` (§22.11) |
| `Dockerfile.cockroach.fly` | Clone of `Dockerfile.pg.fly`; `go build -tags "cockroach rag"`; **drop** `LANCEDB_STORAGE_PROVIDER=s3` defaults (§22.11) |
| `scripts/fly-cockroach-secrets.sh` | Interactive secrets setter, modeled on `fly-pg-secrets.sh` (§22.11) |
| `scripts/crdb-deploy.sh` (or inline Taskfile commands) | Enforces the §22.6 deploy chain |
| `docs_flyio_cockroach_deploy.md` | Deployment guide, parallel to `docs_flyio_neon_deploy.md` (§22) |

### Modify
| File | Change |
|---|---|
| `store/db/db.go` | `case "cockroach"` in the driver factory |
| `store/migrator.go` | Cockroach branch: no `Begin()`; inject `SET serial_normalization` prefix (§7.2) |
| `store/db/postgres/bridge.go` | `crdb.ExecuteTx` wrappers for sites 112, 356, 500, 726, 818, 912, 1003 (cockroach-only) |
| `store/db/postgres/agent.go` | `crdb.ExecuteTx` wrapper for site 772 (cockroach-only) |
| `internal/profile/profile.go` | accept/validate `Driver == "cockroach"`; expose `COCKROACH_DSN` |
| `bin/memos/main.go` | bind `COCKROACH_DSN`; fail-fast when driver=cockroach and DSN empty |
| `scripts/validate-parity.sh` | extend file-list parity to postgres↔cockroach; document sqlite 0.2–0.18 legacy dirs as known divergence |
| `Taskfile.yml` | intent-based `crdb:*`/`deploy:*` targets (§22.3); `crdb:bench`; compat checks; **remove vestigial `VECTOR_DB_PROVIDER` env from `run:cockroach`/`crdb:check`** (§8.4) |
| `scripts/entrypoint.sh` | add `file_env "COCKROACH_DSN"` (required for `COCKROACH_DSN_FILE` support; matches `MEMOS_DSN` pattern) |
| `deploy/ecs/task-definition.json` | document-only: swap `VECTOR_DB_PROVIDER` → `LANCEDB_STORAGE_PROVIDER` (vestigial env cleanup, §8.4) |
| `store/migration_helper.go` | **no change** — SQLite-only helpers must not be ported (REPOSITORY FACT) |

### Unchanged (explicitly)
All 23 `store/db/postgres/*.go` implementation files except the two tx-site files; `store/driver.go`; `store/agent.go`; migrator version logic; `vectordb_cockroach.go`. Neon production files (`fly_pg.toml`, `Dockerfile.pg.fly`, `fly-pg-secrets.sh`) are **not modified** — the Cockroach app is a parallel app (Q10).

---

## 16. Config & environment (extended for deployment)

| Item | Value |
|---|---|
| Driver flag | `--driver=cockroach` |
| DSN env | `COCKROACH_DSN` (no fallback) |
| Protocol | `simple_protocol` (appended by DSN builder) |
| Local dev | `scripts/docker-compose.cockroach.yml` (exists); single-node bootstrap via `COCKROACH_DATABASE`/`USER`/`PASSWORD` |
| Production | Fly.io app `bchat-crdb` + CockroachDB Cloud **Basic**; `postgresql://user:pass@host:26257/bchat?sslmode=verify-full` |
| Vector provider | `LANCEDB_STORAGE_PROVIDER=cockroach` (+ `COCKROACH_DSN`); `RAG_PIPELINE_ENABLED=true` |
| Cluster setting | `feature.vector_index.enabled = true` (set once via `crdb:init`; verified by `crdb:verify`) |
| Build tag | `cockroach` + `rag` in the Fly Dockerfile |
| Backups | Cockroach-managed (Basic: 24h full, 30-day retention) — no code |
| Allowlist | default `0.0.0.0/0` (Basic default); hardened via `crdb:harden` (static Fly egress IP) |

---

## 17. Out of scope / future work (recorded, not implemented)

- Split protocols: migrator simple / runtime extended — gated on a benchmark showing material runtime gain (§13).
- `NewSQLDriver(profile)` capability factory — naming refactor; defer until a third driver appears.
- Wiring `RunResiliently` — zero-caller code today; revisit if Cockroach connection flaps appear.
- Capability-interface extraction (§4) — only if a future feature cannot fit the four seams.
- **Schema downgrade migrations** (per plan4_review #7) — rollback is code-version rollback, never schema downgrade (§11).
- `BIGSERIAL`/int64 ID migration — future schema evolution; int32 space is 2.1B per table (adequate at bchat scale — INFERENCE). When IDs become int64, D2's `sql_sequence` strategy is revisited (§23).
- pgvector provider parity — not used; native `VECTOR` is the only Cockroach vector path.
- CockroachDB Cloud Standard/Advanced tiers, private connectivity (PrivateLink/PSC) — not usable from Fly.io (no VPC peering surface — INFERENCE); documented upgrade path in §22.15.
- Multi-region Cockroach cluster, locality, TLS cert rotation — operational follow-up per `prod_checklist.md`.

---

## 18. CI & validation (extended for deployment)

| Check | Today | After |
|---|---|---|
| File-list parity | `validate-parity.sh` sqlite↔postgres | + postgres↔cockroach |
| Schema parity (best-effort) | sqlite↔postgres | + cockroach; expect identical names modulo `::BIGINT` text |
| Migration drift | `validate-pg-migrations.sh`, `validate-migrations.sh` | parameterized for cockroach |
| **Compat drift (new)** | — | `validate-cockroach-compat.sh` (§10.3) |
| Runtime tests | `crdb:test` (`-tags cockroach ./server/router/api/v1/agent/...`) | + store-layer tests: migrate-on-Docker, CRUD smoke, P4/P5 concurrency tests |
| Benchmarks (advisory) | — | `crdb:bench` (§13) |
| **Deploy chain (new)** | — | `deploy:cockroach` gate (§22.6): parity + compat + build must pass before `fly deploy` |
| Build | `build:cockroach` | unchanged; Fly image builds `-tags "cockroach rag"` |

---

## 19. Risks & known limitations (v4 + deployment rows)

| Risk | Detail | Mitigation |
|---|---|---|
| SKIP LOCKED upstream bug #167582 | SKIP LOCKED may skip rows whose lock is a stale intent from a committed transaction | claims re-scanned each poll cycle; skipped rows picked up next pass (INFERENCE); no code change |
| 0A000 "cached plan must not change result type" | Prepared statements can break across online schema changes | simple protocol (no server-side prepared statements); avoid `SELECT *` (drift list) |
| Migration atomicity | files apply statement-by-statement | §7.3 mitigations + §7.5 recovery + P2 |
| int32 ID space | bounded at 2.1B/table | `sql_sequence` keeps values in range; BIGSERIAL upgrade deferred (§17) |
| EXTRACT float→int truncation | ≤1s deviation at .5s boundaries vs Postgres rounding | explicit cast; no sub-second precision dependency |
| Vector index limitations | no `IMPORT INTO`; batch inserts degrade; backfill on non-empty table blocks writes | provider avoids both; initial reindex pre-traffic under A1; `feature.vector_index.enabled` checked by `crdb:verify` |
| Pool sizing | 10 open × 2 pools vs ~4×vCPU guidance | operational tuning at deploy (§8.3) |
| **Fly static egress IP breakage (new, FLY FACT)** | community reports of lost connectivity after `egress-ip` allocation (IPv4/IPv6 listener nuances) | `crdb:harden` includes a post-allocation connectivity verification step; if breakage occurs, release the IP and fall back to `0.0.0.0/0` temporarily (§22.10) |
| **Basic-tier limits (new)** | 50 allowlist rules; no private connectivity; backups not configurable | adequate for this deployment (INFERENCE); Standard upgrade path documented (§22.15) |
| **Concurrent boot migration (new)** | multiple Fly instances migrate simultaneously | `LATEST.sql` idempotent + `ON CONFLICT` history upsert (REPOSITORY FACT); identical to Neon behavior today |
| Non-cancellable `ALTER TYPE DROP VALUE` | long-running schema change if used | review-required in drift check; not used today |
| simple_protocol performance | text results | accepted; benchmark gate for split (§13, §17) |

---

## 20. Implementation task list (v4 phases + Phase 8 deployment)

**Phase 1 — Experiments (gate):** P1–P4 on single-node Docker; record results in the implementation log.
**Phase 2 — Migration files:** mirror `store/migration/postgres/` → `store/migration/cockroach/`; §6.1 audit + `::BIGINT`; run parity + compat scripts.
**Phase 3 — Migrator branch:** no-`Begin` + SET injection; idempotency rule applied to all Cockroach files.
**Phase 4 — Driver:** `cockroach.go`, factory case, profile/main wiring, fail-fast DSN validation.
**Phase 5 — Retries:** convert 8 tx sites to `crdb.ExecuteTx` closures; add §9.3 classification comments; re-verify with P4.
**Phase 6 — CI/tooling:** `validate-cockroach-compat.sh`, parity extension, Taskfile intent-based targets (`crdb:up`, `crdb:reset`, `crdb:migrate`, `crdb:verify`, `crdb:init`, `crdb:harden`, `deploy:cockroach`, `verify:production`, `rollback:postgres`), vestigial `VECTOR_DB_PROVIDER` cleanup.
**Phase 7 — Vector verification:** P6 on the full deployment config (`LANCEDB_STORAGE_PROVIDER=cockroach`); confirm `feature.vector_index.enabled` flow.
**Phase 8 — Deployment rollout:** create Cloud Basic cluster via `crdb:init`; new Fly app + `fly_cockroach.toml` + `Dockerfile.cockroach.fly`; secrets; P1–P6 re-run against Cloud; `deploy:cockroach`; `verify:production`; `crdb:harden`; benchmarks (§13) recorded; Neon stays live until cutover is proven (Q10).

---

## 21. Acceptance criteria (v4 + deployment)

1. `--driver=cockroach` + `COCKROACH_DSN` boots locally and on Cloud; fail-fast without DSN.
2. Fresh Cockroach DB migrates via `LATEST.sql`; `SHOW CREATE TABLE` proves `nextval(...)` defaults (P1).
3. Migration re-run and mid-file failure behave per §7.3/§7.5 (P2); all Cockroach migration files idempotent; applied files immutable (rule documented).
4. Postgres driver behavior unchanged — existing test suite fully green.
5. All 8 tx sites use `crdb.ExecuteTx` on the Cockroach path; no I/O inside closures (§9.2); determinism classification comments present (§9.3).
6. P4: 1000 concurrent claims, 0 duplicates, 0 lost, ≥1 40001 observed, abort/disconnect cases pass; P5: exactly-once event claims.
7. Parity validator + compat drift check pass with cockroach included.
8. Vector path green with `LANCEDB_STORAGE_PROVIDER=cockroach`; P6 criteria met; `feature.vector_index.enabled` verified.
9. Capability matrix (§3) updated for any SQL construct added during implementation; drift policy (§10) enforced in CI.
10. Every claim in the final PR description cites repository evidence or MCP-verified docs per §0; benchmark baselines recorded (§13).
11. **`task deploy:cockroach` completes end-to-end** (§22.6): build → parity → compat → deploy → health → migration_history → smoke tests; `task verify:production` passes against the Cloud cluster.
12. **`task rollback:postgres` returns the app to Neon** (driver switch + redeploy) with CRDB vectors still reachable via their own pool (verified via smoke tests, §22.13).
13. **`crdb:harden`** swaps `0.0.0.0/0` for the static egress IP `/32` and verifies connectivity; `0.0.0.0/0` remains the documented default (Q9).

---

## 22. Deployment Workflow (plan4_review — "Operations"; NEW in v5)

### 22.1 Design goal — identical operational experience

The lead's success criterion (plan4_review): *"Deploying to CockroachDB should feel almost identical to deploying to Neon."* Neon is live in production today; the Cockroach deployment must match its ergonomics.

| Task | Neon (current, live) | Cockroach (target) |
|---|---|---|
| Build | `task build:rag` (identical binary) | identical |
| Deploy | `fly -a bchat-pg deploy -c fly_pg.toml` | `fly -a bchat-crdb deploy -c fly_cockroach.toml` (wrapped in `task deploy:cockroach`) |
| Migrate | automatic at boot (`main.go:98`) | automatic at boot (same code path) |
| Verify | `/healthz` | `/healthz` + `crdb:verify` (§22.7) |
| Rollback | redeploy previous config | `task rollback:postgres` (driver switch + redeploy) |
| Secrets | `fly secrets set DATABASE_URL=...` | `fly secrets set COCKROACH_DSN=...` |
| Provision DB | Neon console | `ccloud quickstart` / `crdb:init` |

The operator never touches `SET serial_normalization`, migration ordering, or `cockroach sql`. The Taskfile owns that complexity.

### 22.2 Current Neon workflow (documented, REPOSITORY FACT)
```
task build:rag                       # identical binary; driver selected at runtime
   ↓
./scripts/fly-pg-secrets.sh          # DATABASE_URL, OPENROUTER_API_KEY, ENCRYPTION_MASTER_KEY, S3
   ↓
fly -a bchat-pg deploy -c fly_pg.toml
   ↓
entrypoint.sh → memos boot           # profile.Validate(): DSN ← DATABASE_URL
   ↓
storeInstance.Migrate(ctx)           # automatic; main.go:98
   ↓
/healthz green → production live
```

### 22.3 Taskfile as the operator API (plan4_review "public API for operators")

Taskfile is the **public API for operators** — intent-based verbs, not implementation details. Docker compose, `ccloud`, `fly`, and SQL shells are hidden behind it.

| Intent | Task | Internally performs |
|---|---|---|
| Start local Cockroach | `task crdb:up` | `docker compose -f scripts/docker-compose.cockroach.yml up -d`; wait for readiness |
| Stop local Cockroach | `task crdb:down` | compose down |
| Reset local database | `task crdb:reset` | compose down + `rm -rf` data dir + up |
| Apply migrations (local) | `task crdb:migrate` | `./build/memos --driver=cockroach --mode dev` (boot = migrate) or migrator-only invocation |
| Run the application | `task run:cockroach` | existing target, corrected env (`LANCEDB_STORAGE_PROVIDER=cockroach`, not `VECTOR_DB_PROVIDER` — §8.4) |
| Validate compatibility | `task crdb:verify` | §22.7 checks |
| Bootstrap cloud cluster | `task crdb:init` | §22.9 |
| Harden network access | `task crdb:harden` | §22.10 |
| Deploy to Fly | `task deploy:cockroach` | §22.6 |
| Smoke-test production | `task verify:production` | §22.12 |
| Roll back to Neon | `task rollback:postgres` | §22.13 |

**Explicitly NOT in the API:** `cockroach sql` ad-hoc commands, manual `SET` statements, raw `ccloud` invocations, copy-pasted connection strings. Operators may of course use the underlying tools for diagnostics (documented in `docs_flyio_cockroach_deploy.md`).

### 22.4 Local developer workflow
```
task crdb:up            # start single-node Cockroach
   ↓
task crdb:migrate       # migrate (boot applies LATEST.sql)
   ↓
task run:cockroach      # app with COCKROACH_DSN + LANCEDB_STORAGE_PROVIDER=cockroach
```
REPOSITORY FACT: `scripts/docker-compose.cockroach.yml` already provides the container (`bchat_user`/`bchat_pass`@`localhost:26257/bchat?sslmode=disable`, console :8080). The sequence replaces today's manual `docker compose … && cockroach sql …` steps.

### 22.5 Environment management — where does `COCKROACH_DSN` come from?

The bchat convention is a single `.env` for local development (sourced by Taskfile run targets: `set -a && . .env && set +a` — REPOSITORY FACT, Taskfile.yml) and **Fly secrets for production** (the Neon pattern). No new dotfile convention is introduced:

| Environment | Source of `COCKROACH_DSN` | How it is set |
|---|---|---|
| Local dev | `.env` | `COCKROACH_DSN=postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable` (docker-compose credentials) |
| Local CI | env var | task/CI pipeline |
| Production (Fly) | Fly secret | `fly -a bchat-crdb secrets set COCKROACH_DSN="postgresql://user:pass@<cluster>-<id>.<org>.crdb.cloud:26257/bchat?sslmode=verify-full"` (from `ccloud quickstart`/Connect dialog) |
| Secret-file support | `COCKROACH_DSN_FILE` | `scripts/entrypoint.sh` gains `file_env "COCKROACH_DSN"` (§15) |

`DATABASE_URL` and `COCKROACH_DSN` never coexist by design (D7): the driver flag decides which is read, and a driver/DSN mismatch is a fail-fast startup error.

### 22.6 Fresh production deployment (plan4_review "Fresh production deployment")

```
task deploy:cockroach        # single command; full chain, experiments optional
   ├── 1. build              # go build -tags "cockroach rag" (identical binary story)
   ├── 2. validate parity    # validate-parity.sh (sqlite↔postgres↔cockroach)
   ├── 3. validate compat    # validate-cockroach-compat.sh (FORBIDDEN/REVIEW-REQUIRED)
   ├── 4. experiments        # OPTIONAL (skip flag): P1–P6 need fault injection; run locally or on Cloud
   ├── 5. fly deploy         # fly -a bchat-crdb deploy -c fly_cockroach.toml
   ├── 6. wait healthy       # poll https://bchat-crdb.fly.dev/healthz (grace_period 15s, per fly_pg.toml pattern)
   ├── 7. verify migration_history  # §22.7 step 3 — 0.35 row present, exactly one
   └── 8. smoke test         # verify:production (§22.12)
```

The user never thinks about `SET serial_normalization`, migration ordering, or allowlists. Phases 1–3 reuse the existing Neon-era scripts; only the deploy target and verification are new.

### 22.7 Verification — `task crdb:verify`
Checks, in order (all scripted; any failure → exit 1 with the failing check named):
1. **Connection:** `SELECT 1` via pgx against `COCKROACH_DSN` (retry on transient failure).
2. **Version:** `SELECT version()` — confirms a CockroachDB server, not Postgres (guards DSN cross-wiring).
3. **Migration history:** `SELECT version, created_ts FROM migration_history ORDER BY version` — exactly one row = `0.35` (A1); on upgrades, rows are contiguous.
4. **Schema version:** boot-time log line + `SHOW CREATE TABLE agent_tenants` — proves `nextval(...)` defaults (P1 evidence in production).
5. **Vector index:** `SELECT index_name FROM [SHOW INDEX FROM <vector_table>]` (or `SHOW CREATE TABLE` on the vector table) — index exists; `SHOW CLUSTER SETTING feature.vector_index.enabled` = `true`.
6. **Retry plumbing:** log-grep for the §9.4 wrapper initialization; optionally a scripted contention probe (reuses P4 harness).
7. **Health endpoint:** `GET /healthz` returns 200.

This is the production-facing form of P1–P6's pass criteria.

### 22.8 Health verification (post-deploy, per plan4_review "Health verification")
`deploy:cockroach` reports success only after: `GET /healthz` 200 **and** `SELECT version()` confirms Cockroach **and** `migration_history` shows the expected version (§22.6 steps 6–7). Same bar Neon deploys clear today (healthz + logs), plus the two database checks.

### 22.9 Cloud bootstrap — `task crdb:init` (plan4_review "Cloud bootstrap")
```
task crdb:init
   ├── 1. validate env        # ccloud CLI present + authenticated (ccloud auth whoami); COCKROACH_DSN or cluster name provided
   ├── 2. cluster             # create Basic cluster (ccloud cluster create basic <name> <region> --cloud AWS); region nearest Fly primary (sjc) — verify availability via `ccloud cluster region list`; or reuse `ccloud quickstart`
   ├── 3. SQL user            # ccloud cluster user create (or quickstart's user)
   ├── 4. connection string   # ccloud cluster sql --connection-url <name> → set COCKROACH_DSN (local .env or Fly secret)
   ├── 5. test TLS            # connect with sslmode=verify-full; CA cert from quickstart/Connect dialog (Let's Encrypt — system roots suffice)
   ├── 6. ping cluster        # SELECT version() over the DSN
   ├── 7. verify permissions  # SHOW GRANTS / attempt migration-history read; admin SQL user created by default (DOCUMENTATION FACT: new SQL users are created with admin privileges)
   ├── 8. cluster setting     # SET CLUSTER SETTING feature.vector_index.enabled = true (vector indexes, §8.4)
   └── 9. done                # instructions: run crdb:verify, then deploy:cockroach
```
No manual SQL. The existing Taskfile `crdb:cluster:create`/`crdb:sql:shell` targets are subsumed into this flow.

### 22.10 Networking — default open, production hardening (Q9)

**Default (matches Basic tier):** Basic clusters are created with `0.0.0.0/0` (DOCUMENTATION FACT). Zero-config bootstrap; suitable for staging/eval. Protection = SQL user password + TLS only. Documented as acceptable only because the app database holds no public-sensitive data at this stage (INFERENCE) and the hardening step is one command away.

**Production hardening — `task crdb:harden`:**
```
   ├── 1. allocate        # fly -a bchat-crdb ips allocate-egress   (app-level static egress IP; ~$3.60/mo/region; FLY FACT)
   ├── 2. read IPs        # fly ips list -a bchat-crdb (type egress)
   ├── 3. allowlist       # ccloud cluster networking allowlist create <name> <ip>/32 --sql --name fly-<app>   (replace 0.0.0.0/0 entry: allowlist update/delete)
   ├── 4. remove 0.0.0.0/0  # ccloud cluster networking allowlist delete <name> 0.0.0.0/0  (production checklist guidance)
   └── 5. VERIFY          # crdb:verify step 1-2 from the deployed app; if broken, see below
```
**Post-allocation verification is mandatory** (FLY FACT: community reports of lost connectivity after egress-IP allocation, IPv4/IPv6 listener nuances). If connectivity breaks: release the IP (`fly ips release-egress`), restore `0.0.0.0/0`, and raise a Fly support issue — the app itself is unaffected (connections fail fast at pool startup). Allowlist rule limits: Basic/Standard 50 rules (DOCUMENTATION FACT) — ample for one app.

### 22.11 Fly.io integration — new app, new files (Q10)

| Setting | Neon (`fly_pg.toml`) | Cockroach (`fly_cockroach.toml`) |
|---|---|---|
| App name | `bchat-pg` | `bchat-crdb` |
| Dockerfile | `Dockerfile.pg.fly` | `Dockerfile.cockroach.fly` (`go build -tags "cockroach rag"`) |
| `MEMOS_DRIVER` | `'postgres'` | `'cockroach'` |
| DSN secret | `DATABASE_URL` | `COCKROACH_DSN` |
| `[[mounts]]` | none | none (managed DB; vector store now also in CRDB — no LanceDB volume) |
| RAG env | `LANCEDB_STORAGE_PROVIDER=s3` + Tigris creds | `LANCEDB_STORAGE_PROVIDER=cockroach`; **no S3/Tigris env at all** (Q12) |
| Embeddings | `EMBEDDING_PROVIDER=openrouter` | unchanged |
| Reindex | `RAG_STARTUP_REINDEX_DISABLED=true` | `FORCE_REINDEX_ON_STARTUP=true` for the initial A1 reindex only, then disabled (vector-index backfill blocking is safe pre-traffic; §19) |
| Health check | `/healthz`, grace 15s | identical |
| VM | 1024mb shared 1cpu | identical initially; tune per §8.3 after first load |

**Secrets** (`./scripts/fly-cockroach-secrets.sh`, modeled on `fly-pg-secrets.sh`): `COCKROACH_DSN`, `OPENROUTER_API_KEY`, `ENCRYPTION_MASTER_KEY` (auto-generated). **No AWS/Tigris secrets.**

**Migration on boot:** unchanged code path (`main.go:98`). Multiple Fly instances booting concurrently are safe: `LATEST.sql` is fully idempotent and the history upsert is `ON CONFLICT` (REPOSITORY FACT) — the same property Neon relies on today.

### 22.12 Production smoke tests — `task verify:production` (plan4_review "Production smoke tests")

Executed against the deployed app's public API immediately after deploy, proving migrations, retries, vectors, and transactions survived deployment:
```
create tenant  →  POST /api/v1/agent/tenants (admin)
create memo    →  POST /api/v1/memos
vector insert  →  trigger indexing of a KB chunk (tenant KB upload + reindex)
vector search  →  RAG search endpoint; assert ≥1 hit
bridge tx      →  exercise a bridge handoff create/claim/complete cycle (outbox)
delete test data → remove tenant/memo (cleanup)
```
Assertions: each step returns 2xx and the expected payload shape; vector search returns the uploaded chunk. Failure of any step fails `deploy:cockroach`'s success report (§22.6 step 8). A `--destroy` flag (default on) ensures no test residue remains in the production database.

### 22.13 Rollback — `task rollback:postgres` (Q11; plan4_review "Add rollback commands")
```
task rollback:postgres
   ├── 1. fly -a bchat-crdb secrets set DATABASE_URL="<neon-dsn>"     # (Neon secret restored)
   ├── 2. fly -a bchat-crdb secrets unset COCKROACH_DSN
   ├── 3. fly -a bchat-crdb deploy                                    # same image; MEMOS_DRIVER=postgres in fly_cockroach.toml stays? — NO: flip MEMOS_DRIVER via env override or second config; see note
   └── 4. verify:production (against Neon) + crdb:verify skipped (no CRDB DSN)
```
Design note (encoded at implementation time, INFERENCE): the cleanest encoding is a dedicated `fly_pg-rollback.toml` (clone of `fly_pg.toml` with the `bchat-crdb` app name) so `MEMOS_DRIVER=postgres` is declarative, not a manual env override. **Vector behavior on rollback — verified safe:** with `COCKROACH_DSN` unset and driver=postgres, `NewCockroachVectorDB` cannot construct (DSN required, `vectordb_cockroach.go:26-31`); if `COCKROACH_DSN` is *kept*, `service.go:158-166` opens a dedicated CRDB pool for vectors while app data lives on Neon — a legitimate mixed state for transition periods. Both paths are documented; the default rollback unsets `COCKROACH_DSN` and re-runs vectors on Neon's LanceDB… **no** — with `LANCEDB_STORAGE_PROVIDER=cockroach` unset, the fallback provider applies (memory/no-op). The rollback task therefore also resets RAG env to the Neon values. Encoded exactly in the task definition; verified by `verify:production` smoke tests (§22.12) which include the vector search step.

**Rollback does not attempt schema downgrade** (§11): CRDB data remains at its last applied schema; a later re-cutover (re-deploy with `--driver=cockroach`) is a plain forward migration.

### 22.14 Vectors into CockroachDB — what the deployment actually does (Q12)
- `LANCEDB_STORAGE_PROVIDER=cockroach` + `COCKROACH_DSN` + `RAG_PIPELINE_ENABLED=true` → `CockroachVectorDB` (REPOSITORY FACT: `vectordb.go:294`).
- With driver=cockroach, the vector store shares the app's single pool (`service.go:158-166`).
- Native `VECTOR(n)` + `<=>` search + C-SPANN vector index (runtime-created; needs `feature.vector_index.enabled`, §8.4).
- LanceDB/S3/Tigris is retired from this app (no `LANCEDB_STORAGE_PROVIDER=s3`, no AWS secrets).
- The reindex path uses single-row UPSERTs (DOCUMENTATION FACT: batch inserts degrade; provider already avoids them).
- Rollback implications handled in §22.13.

### 22.15 Backup & restore (no code; documented for operators)
- **Managed backups (DOCUMENTATION FACT):** Basic clusters are backed up automatically every 24h with 30-day retention. Backups are stored in a single region selected at cluster creation.
- **Restore:** console (Backup and Restore → Restore) or Cloud API (`POST /api/v1/clusters/{id}/restores`). Destination must be wiped; same plan type; cluster unavailable during restore.
- **Upgrade path:** if RPO/retention requirements tighten, Standard allows 5min–24h frequency and configurable retention (set once). Standard also adds private connectivity (not usable from Fly.io) and provisioned compute — a documented, console-only migration.

### 22.16 Out-of-band operations (documented, not encoded)
Allowlist management (`ccloud cluster networking allowlist *`), backup/restore (console/API), cluster deletion (`ccloud cluster delete`) remain available for incident response; they are documented in `docs_flyio_cockroach_deploy.md` but intentionally NOT part of the operator Taskfile API — they are rare, destructive, or console-native.

---

## 23. Decision Log (plan4_review "Final principal-engineer suggestion")

The table becomes invaluable two years from now when someone asks "why did we do this?"

| Decision | Why | Revisit When |
|---|---|---|
| simple_protocol everywhere | parity with Postgres driver; zero protocol coupling | runtime benchmark exceeds §13 threshold (>20%) |
| `sql_sequence` (D2) | **given int32 model types**, keeps IDs in range | IDs become int64 (then `rowid` or `sql_sequence_cached` are viable) |
| Whole-file migrations, no tx wrapper | Cockroach DDL does not support multi-statement txns | Cockroach transaction semantics change |
| Shared implementation (postgres package) | 23 files + `store.Driver` conformance stay single-source | SQL divergence exceeds the four seams (§4) |
| Mirror versioned dirs (D3) | version machinery + parity validation + auditability | migration tooling changes (e.g., a real migration framework) |
| `COCKROACH_DSN`, no fallback (D7) | prevents cross-wiring; fail-fast | a unified DSN convention emerges |
| **CockroachDB Cloud Basic tier (Q8)** | free, scales to zero, managed backups; bchat traffic is modest | traffic/cost/RPO needs exceed Basic |
| **`0.0.0.0/0` default + `crdb:harden` (Q9)** | Basic ships open; hardening is one command; Fly egress IPs cost ~$3.60/mo | production traffic or compliance appears; egress-IP feature stabilizes |
| **New Fly app + new files (Q10)** | Neon production untouched until cutover proven | cutover to Cockroach is permanent (then files merge) |
| **Rollback = driver switch + redeploy (Q11)** | binary is identical; driver selected at runtime | Cockroach-only schema features make the binary diverge |
| **Vectors into CockroachDB (Q12)** | single data store; provider + shared pool already exist | vector workload outgrows Basic; LanceDB features needed |
| **`deploy:cockroach` full chain (Q13)** | deploys are verifiable end-to-end | CI/CD replaces manual deploys (then the chain moves to CI) |

---

## 24. Final verification summary

plan4_review's remaining issues and their resolution:

| plan4_review item | Resolution in v5 |
|---|---|
| #1 Mirror justification | §5.3 explicit three-part justification |
| #2 Fresh-deployment assumption | Centralized as **Assumption A1** (§0); referenced everywhere |
| #3 "exactly four seams" | "currently four identified seams" (§4) |
| #4 Grep scanner scope | §10.3: "best-effort, not a parser" |
| #5 Benchmark thresholds | §13: >20% vs baseline → manual review |
| #6 Retry experiments | §12 P4: + forced transaction abort + network disconnect |
| #7 Rollback wording | §11: "Rollback does not attempt schema downgrade" |
| #8 int32/unique_rowid wording | §14.2: rephrased as repository int32 scan issue |
| #9 sql_sequence phrasing | §5.2: "Given the current repository model types (int32 IDs)…" |
| Decision Log | §23 |
| **Operations / Deployment Workflow** | **§22 (new)** — Taskfile operator API, local + production workflows, verification, cloud bootstrap, networking, Fly integration, smoke tests, rollback commands, environment management, backup/restore |
