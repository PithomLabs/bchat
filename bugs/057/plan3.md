# Bug 057 Plan v3 — CockroachDB Database Support (Implementation Plan)

**Status:** Ready for implementation review
**Supersedes:** `plan2_deepseek.md` (v2), `plan.md` (v1)
**Addressed reviews:** `plan2_deepseek_review.md` (9.3/10, not approved), `plan2_review.md`, `plan2_review_chatgpt.md`, `plan_review.md`, `plan_review_chatgpt.md`
**Prepared:** 2026-08-02
**Scope:** Production migration + runtime support for CockroachDB as a first-class driver alongside SQLite and PostgreSQL. No coding in this document — every engineering claim is evidence-backed per the classification system below.

---

## 0. Evidence classification (adopted from plan2_deepseek_review.md "Rules")

Every non-trivial claim in this plan is classified. This suppresses hallucinated compatibility claims:

| Tag | Meaning | Citation required |
|-----|---------|------------------|
| **REPOSITORY FACT** | Derived from bchat source code | `file:line` |
| **DOCUMENTATION FACT** | Derived from official CockroachDB docs via the CockroachDB MCP | doc name + section/version |
| **INFERENCE** | Logical conclusion from verified facts | must say "This is an inference" |
| **SPECULATION** | Recommendation only | must start with "I recommend" |
| **PROHIBITED** | "Cockroach doesn't support X" unless official docs say so | — |

**Evidence rule (from the review's final prompt):** every non-trivial engineering claim is backed by either (1) repository evidence, (2) official CockroachDB documentation, or (3) an explicitly labeled experiment that must be run before implementation. Claims that cannot be supported are removed.

---

## 1. Background context — interactive Q&A with project lead (2026-08-02)

Before writing this plan, the lead and the architect held an interactive Q&A session. Every question was answered with a recommendation, and the lead made the final decisions. All seven decisions are binding constraints for this plan.

### Q1 — Migration execution strategy (Review Issues 2 & 3: statement splitter, DDL atomicity)
**Question:** How should migrations execute on CockroachDB?
- **Decision (lead): Whole-file, no transaction wrapper.**
- Keep the existing `execute()` behavior — one `ExecContext` with the entire file content on one connection (multi-statement string via simple protocol).
- Skip `db.Begin()` for the Cockroach driver.
- No custom SQL splitter is written (the review explicitly rejected a splitter as notoriously difficult).
- Atomicity loss is explicitly documented and mitigated (see §7.3).

### Q2 — serial_normalization (Review Issue 1: prove nextval() is generated)
**Question:** How should `serial_normalization` be applied for Cockroach SQL files?
- **Decision (lead): Inject in migrator code.**
- The Cockroach migration path prepends `SET serial_normalization = 'sql_sequence';` as the first statement of the batch, on the same connection as the CREATE TABLE statements.
- Cockroach SQL files stay byte-identical to Postgres files where possible, preserving the parity validation script.
- A proof-of-concept experiment (P1, §12) must prove `nextval(...)` defaults via `SHOW CREATE TABLE` after migration, per the review's requirement.

### Q3 — Migration layout
**Question:** What migration layout for `store/migration/cockroach/`?
- **Decision (lead): Mirror versioned directories.**
- `store/migration/cockroach/` contains versioned dirs `0.19/`…`0.35/` plus `LATEST.sql`, mirroring `store/migration/postgres/` file-for-file.
- This works unchanged with the existing migrator, `GetCurrentSchemaVersion`, and the parity validator.

### Q4 — Query protocol for the runtime driver (Review Issue 4: separate migration from runtime protocol)
**Question:** What query protocol should the runtime Cockroach driver use?
- **Decision (lead): `simple_protocol` everywhere.**
- Migration and runtime both use `default_query_exec_mode=simple_protocol`, identical to the current Postgres driver.
- The review's suggestion to split protocols (simple for migrator, extended for runtime) is consciously deferred; it is recorded as future work (§16).

### Q5 — Runtime transaction retries (40001)
**Question:** How should runtime transactions handle Cockroach serialization retries?
- **Decision (lead): 40001 retry wrapper on the 8 explicit-transaction sites.**
- A Cockroach-only retry helper wraps the 8 `BeginTx` sites (inventory in §9), using `crdb.ExecuteTx` from `cockroach-go/v2 v2.4.3` (already a repository dependency).
- The side-effect audit required by the review (Issue 6) is included in §9.2 — all transaction bodies are pure DB operations.
- Errors still propagate to callers when retries are exhausted.

### Q6 — Driver construction (Review Issue 7: naming)
**Question:** How should the Cockroach driver be constructed?
- **Decision (lead): `postgres.NewCockroachDB()`.**
- An exported constructor inside the existing `store/db/postgres` package, sharing internals with `NewDB()`.
- The review's `NewSQLDriver(profile)` capability-factory alternative is recorded as future work (§16).

### Q7 — Connection configuration (v2 plan: explicit COCKROACH_DSN)
**Question:** How is the Cockroach connection configured?
- **Decision (lead): `COCKROACH_DSN` env var + `--driver=cockroach`.**
- No fallback to `DATABASE_URL`. Fly.io sets `COCKROACH_DSN` explicitly.
- `bin/memos/main.go` already reads driver/dsn flags via viper (REPOSITORY FACT: `bin/memos/main.go` flag definitions; `internal/profile/profile.go` carries `Driver` and `DSN` fields).

---

## 2. Methodology — how the CockroachDB MCP backed every claim

The CockroachDB MCP server (`cockroachdb_search_cockroach_db_knowledge_sources`) was used as the authoritative reference for all CockroachDB behavior claims. The MCP performs semantic retrieval over official CockroachDB documentation and knowledge sources, including per-version docs pages (`stable`/v26.2, v25.x, v24.x, v23.x) and upstream GitHub issues.

### 2.1 MCP queries executed and what they returned

| # | Query | Key evidence returned |
|---|-------|------------------------|
| 1 | "Does CockroachDB support SELECT FOR UPDATE with SKIP LOCKED?" | **SKIP LOCKED is a documented wait policy** in the FOR UPDATE/FOR SHARE docs for every version queried (v23.2–v26.2/stable). The older blog result claiming "CockroachDB doesn't currently support this" is **outdated** and was discarded. Also surfaced the upstream correctness bug #167582 (SKIP LOCKED may skip rows with intents from committed transactions) — see §15. |
| 2 | "What does the serial_normalization setting do and how do I set it?" | **`serial_normalization` is a session variable** — modifiable with `SET`, viewable with `SHOW`. Options: `rowid` (default), `virtual_sequence`, `sql_sequence`, `sql_sequence_cached`, `unordered_rowid` (+ `sql_sequence_cached_node` from v24.1). It "specifies the default handling of SERIAL in table definitions." This confirms the injection strategy in Q2. |
| 3 | "What type does EXTRACT(EPOCH FROM TIMESTAMP) return and can it be implicitly assigned to a BIGINT column?" | **`extract(element, timestamp/timestamptz) → float`** in every version (v23.1–v26.2). Confirms the portability action in §6.2: Postgres files use `EXTRACT(EPOCH FROM NOW())` as BIGINT column defaults without casts (19 occurrences, REPOSITORY FACT: `store/migration/postgres/LATEST.sql`); PostgreSQL permits this via assignment cast (numeric→bigint), CockroachDB returns float — the Cockroach DDL must use an explicit `::BIGINT` cast. |
| 4 (prior session) | pgx CRUD tutorial | pgx connection patterns; `crdb.ExecuteTx` retry-handling pattern for PostgreSQL-wire drivers. |
| 5 (prior session) | SERIAL semantics | Default `rowid` mode generates 64-bit `unique_rowid()` values (non-monotonic, gap-heavy); `sql_sequence` mode generates sequential 64-bit values starting at 1. |
| 6 (prior session) | Schema changes in explicit transactions | Most DDL is not supported inside explicit multi-statement transactions; `CREATE TABLE` and `CREATE INDEX` may run inside multi-statement transactions; the `autocommit_before_ddl` session setting commits the enclosing transaction before executing a DDL statement (settable via connection string). |
| 7 (prior session) | Batched statements / simple query protocol | Semicolon-separated statement strings sent as a single unit (e.g., `BEGIN; …; COMMIT;`) support automatic transaction retries — grounds the whole-file execution strategy. |
| 8 (prior session) | Single-node Docker bootstrap | `COCKROACH_DATABASE`, `COCKROACH_USER`, `COCKROACH_PASSWORD` env vars and `docker-entrypoint-initdb.d` scripts; applied only when the data directory is empty. Grounds the local-dev path and the existing `scripts/docker-compose.cockroach.yml`. |
| 9 (prior session) | SELECT FOR UPDATE details | `FOR SHARE`/`FOR KEY SHARE` accepted for Postgres compatibility; `NOWAIT` supported; application-level retry handling still required for SQLSTATE 40001. |
| 10 (prior session) | Connection pooling production guidance | Active connections across all pools should not greatly exceed ~4× the cluster's vCPUs. Grounds §8.4. |
| 11 (prior session) | Vector indexes | Limitations: large batch inserts degrade performance (batching should be avoided), `IMPORT INTO` unsupported on tables with vector indexes, `vector_l1_ops`/`bit_hamming_ops`/`bit_jaccard_ops` not implemented, index acceleration applies only to prefix-column filters. Grounds §15 and the existing `vectordb_cockroach.go`. |
| 12 (prior session) | Prepared statements vs. online schema changes | Prepared statements can break across online schema changes with error `cached plan must not change result type` (SQLSTATE 0A000); avoid `SELECT *` in prepared statements. Grounds §15. |

### 2.2 Reference documents in `bugs/057/`

`pgx.md`, `docker.md`, `cockroach-demo.md`, `prod_checklist.md`, `test_locally.md` were read and are consistent with the MCP findings above. The review's proposed 11-file reference pack (`01-architecture.md` … `11-local-development.md`) is **not created separately** — this plan embeds every citation inline (§6, §7, §8, §9) so the implementing agent never has to chase a second document set.

---

## 3. Capability matrix (review: "biggest thing still missing")

| Capability | PostgreSQL | CockroachDB | Used by bchat | Abstraction in this port |
|---|---|---|---|---|
| Transaction retry | optional | **mandatory** (40001) | yes | `crdb.ExecuteTx` wrapper on the 8 `BeginTx` sites (§9) |
| Vector | pgvector (runtime) | native `VECTOR(n)` (runtime) | yes | runtime-created schema; existing `vectordb_cockroach.go` |
| SERIAL / identity | sequence | configurable via `serial_normalization` | yes | migrator-injected `SET` (§7.4) + experiment P1 |
| JSONB | yes | yes | yes | shared (no changes) |
| `FOR UPDATE SKIP LOCKED` | yes | yes (v23.2+ docs) | yes | shared (no changes; note #167582) |
| `EXTRACT(EPOCH FROM NOW())` defaults | numeric → assignment cast | **float** (no implicit int cast) | yes | explicit `::BIGINT` in Cockroach DDL (§6.2) |
| `RETURNING` | yes | yes | yes | shared |
| `ON CONFLICT DO NOTHING` | yes | yes | yes | shared |
| `ILIKE` | yes | yes | yes | shared |
| Multi-statement simple-protocol exec | yes | yes (batched statements) | yes (migrations) | whole-file exec, no tx wrapper (§7) |
| DDL in explicit transactions | yes | **no** (mostly) | n/a | no `Begin()` for Cockroach migrations (§7) |
| Atomic whole-file migration | yes | **no** | n/a | documented loss + idempotency mitigations (§7.3) |

---

## 4. Architecture overview

```
profile.Driver == "cockroach"
        │
        ├── store/db/db.go factory ──► postgres.NewCockroachDB(profile)   (new, same package as NewDB)
        │                                   │
        │                                   ├── shared DSN builder (postgres.go)   — COCKROACH_DSN, simple_protocol
        │                                   ├── shared pool settings (postgres.go) — 10/5/5m/1m + 60s ping
        │                                   └── shared SQL implementation files (23 files, unchanged)
        │
        ├── store/migrator.go ──► migration/cockroach/{0.19…0.35,LATEST.sql}  (mirrors postgres dirs)
        │         └── Cockroach branch: no Begin(); inject SET serial_normalization; whole-file Exec
        │
        └── runtime retries ──► crdb.ExecuteTx wrapper around 8 BeginTx sites (cockroach-only)
```

- **Single implementation, not a fork.** `postgres.NewDB()` and `postgres.NewCockroachDB()` share every SQL file and helper. This removes the code-duplication blocker (plan2_review C-1) and the missing-files issue (C-2) by construction.
- REPOSITORY FACT: the driver factory switch is `store/db/db.go` (`case "sqlite"`, `case "mysql"`, `case "postgres"`, default error). A `case "cockroach"` is added.
- REPOSITORY FACT: `NewDB` is `store/db/postgres/postgres.go:22`; it appends `default_query_exec_mode=simple_protocol` to the DSN when absent (line 33), sets `MaxOpenConns(10)` (line 42), `MaxIdleConns(5)`, `ConnMaxLifetime(5m)`, `ConnMaxIdleTime(1m)`, and a 60s ping timeout.

---

## 5. Decisions, rationale, evidence

### 5.1 D1 — Whole-file migration execution, no explicit transaction (Q1)
- REPOSITORY FACT: `store/migrator.go:321-335` — `execute()` runs the entire file via one `tx.ExecContext(ctx, stmt)`. No statement splitter exists anywhere in the codebase today.
- DOCUMENTATION FACT: CockroachDB simple query protocol accepts multi-statement strings; batched statements executed as one unit support automatic retries (MCP query 7).
- DOCUMENTATION FACT: most DDL cannot run inside explicit multi-statement transactions; `autocommit_before_ddl` (MCP query 6) commits before DDL regardless. An explicit `Begin()` wrapper therefore provides no atomicity on Cockroach and adds a false promise.
- INFERENCE: Therefore the Cockroach branch drops `Begin()`/`Commit()` and executes the file directly on the pool, preserving the existing whole-file execution shape.
- Alternative rejected: custom statement splitter (review Issue 2; risky with quoted semicolons/dollar-quotes).

### 5.2 D2 — `SET serial_normalization = 'sql_sequence'` injected by the migrator (Q2)
- DOCUMENTATION FACT: `serial_normalization` is a session variable, settable with `SET`, controlling SERIAL handling in table definitions (MCP query 2).
- REPOSITORY FACT: all model IDs are `int32` (`store/agent.go`, `store/memo.go`, `store/driver.go`); Postgres DDL uses `SERIAL PRIMARY KEY` on every table (`store/migration/postgres/LATEST.sql`).
- DOCUMENTATION FACT: default `rowid` mode yields 64-bit `unique_rowid()` values (MCP query 5) that overflow Go `int32`.
- INFERENCE: `sql_sequence` mode is required to keep ID values within `int32` range (sequences start at 1 and increment). This is a hard compatibility requirement, not a preference.
- REPOSITORY FACT: `serial_normalization` is not currently set anywhere in the repository; the Postgres path is unaffected.

### 5.3 D3 — Mirror versioned migration directories (Q3)
- REPOSITORY FACT: `store/migrator.go:210` — base path is `migration/{driver}/`; `GetCurrentSchemaVersion` (line 257) scans `*/*.sql`; file version parsing (line 300); `LATEST.sql` handling (line 34); `preMigrate` (line 160) applies `LATEST.sql` when no history exists; history upsert happens only after successful execution (lines 142-149, 192-199).
- INFERENCE: a mirrored `store/migration/cockroach/` directory tree works with zero migrator-structure changes beyond the Cockroach branch (no `Begin`, SET injection).
- REPOSITORY FACT: postgres has versioned dirs `0.19/`–`0.35/` (25 version dirs) plus `LATEST.sql` — the mirror is file-for-file.

### 5.4 D4 — simple_protocol everywhere (Q4)
- REPOSITORY FACT: `store/db/postgres/postgres.go:33` — the Postgres driver already forces `simple_protocol`.
- INFERENCE: keeping it for Cockroach yields identical runtime behavior, zero migration-protocol coupling, and the smallest test surface. The review's split-protocol idea (Issue 4) is acknowledged and deferred (§16) with a benchmark gate.
- SPECULATION: I recommend measuring first; if runtime latency is a concern later, the extended protocol can be enabled per-connection for the runtime pool only.

### 5.5 D5 — `crdb.ExecuteTx` retry wrapper on 8 tx sites (Q5)
- REPOSITORY FACT: `go.mod` already requires `github.com/cockroachdb/cockroach-go/v2 v2.4.3`.
- REPOSITORY FACT: module cache `crdb/tx.go:299` — `ExecuteTx(ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn func(*sql.Tx) error) error` exists in v2.4.3 (works with `database/sql`).
- REPOSITORY FACT: `server/router/api/v1/agent/vectordb_cockroach.go:191` already uses `crdb.ExecuteTx(ctx, v.db, nil, ...)` over `sql.Open("pgx", dsn)` — the pattern is proven in-repo.
- REPOSITORY FACT: the existing `isPostgresRetryable` helper (`store/db/postgres/bridge.go:320`) matches error *strings* ("deadlock", "lock not available", "could not serialize") and is used by a hand-rolled 3-attempt loop (`bridge.go:95-104`). It does not match Cockroach's 40001 messages ("restart transaction").
- INFERENCE: `crdb.ExecuteTx` (SQLSTATE-based 40001 detection) supersedes the string-matching hack for the Cockroach path; the Postgres path keeps its current behavior untouched.

### 5.6 D6 — `postgres.NewCockroachDB()` (Q6)
- Same package, shared internals (D1). The review's `NewSQLDriver(profile)` naming (Issue 7) is deferred (§16).
- REPOSITORY FACT: no `store/db/cockroach/` directory exists; none will be created.

### 5.7 D7 — `COCKROACH_DSN`, no fallback (Q7)
- REPOSITORY FACT: `internal/profile/profile.go` carries `Driver` and `DSN`; `bin/memos/main.go` binds them from flags/env via viper.
- INFERENCE: explicit `COCKROACH_DSN` prevents accidentally pointing a Cockroach run at a Postgres URL (and vice versa); `--driver=cockroach` + missing `COCKROACH_DSN` must fail fast at startup.
- REPOSITORY FACT: the existing Cockroach tooling already exists and is reused: `scripts/docker-compose.cockroach.yml` (service `cockroach`, `bchat_user`/`bchat_pass`, `localhost:26257/bchat?sslmode=disable`, console :8080) and Taskfile targets `build:cockroach`, `run:cockroach`, `crdb:check`, `crdb:db-check`, `crdb:test`, `crdb:docker:build`, `crdb:docker:run` (build tag `cockroach`).

---

## 6. SQL portability audit (construct → usage → support → action)

Format adopted from plan2_deepseek.md and kept per review §2. Every row cites usage and support.

| Construct | Usage in bchat | Cockroach support | Action |
|---|---|---|---|
| `SERIAL PRIMARY KEY` | every table, `store/migration/postgres/LATEST.sql` | supported; mode = session var | inject `SET serial_normalization='sql_sequence'` (D2); prove via P1 |
| `EXTRACT(EPOCH FROM NOW())` as BIGINT default | 19 occurrences, `store/migration/postgres/LATEST.sql`; also `migration_history.created_ts` (runtime relies on default via `store/db/postgres/migration_history.go:39-45`) | `extract(...) → float`; no implicit float→int8 assignment cast (MCP query 3) | Cockroach DDL uses `EXTRACT(EPOCH FROM NOW())::BIGINT` (explicit cast; deterministic truncation); prove via P3 |
| `FOR UPDATE SKIP LOCKED` | `store/db/postgres/agent.go:2773-2781` (single `UPDATE … RETURNING`, no tx) | supported wait policy (v23.2+ docs, MCP query 1) | none (note #167582, §15) |
| `FOR UPDATE` | `store/db/postgres/bridge.go:356, 500` (in tx) | supported | none (retry wrapper handles 40001) |
| `RETURNING` | ~20+ sites (`bridge.go`, `agent.go`, `memo.go`) | supported | none |
| `ON CONFLICT DO NOTHING` / `ON CONFLICT(version) DO UPDATE` | `LATEST.sql`; `migration_history.go:39` | supported | none |
| `ILIKE` | `store/db/postgres/memo_filter.go:143,172,209,224` | supported | none |
| `jsonb` + `@> jsonb_build_array(...)` | `memo_filter.go` | supported | none |
| `::BOOLEAN` casts | `store/db/postgres/memo.go:106-115` | supported | none |
| `::BIGINT` casts | `store/db/postgres/agent.go:2605,2693,2718,2776,2780` | supported (explicit cast) | none |
| `uuid` columns / `uuid.NewString()` | `bridge.go` (handoff/reply IDs) | supported (`UUID` type) | none; values generated in Go |
| `CHECK`/unique/FK constraints, indexes | `LATEST.sql` | supported | none |
| `CREATE VECTOR INDEX` | `vectordb_cockroach.go:109` (runtime-created) | supported; **no `IF NOT EXISTS`** | runtime-only; already handled in repo |
| `INSERT … $6::VECTOR` / `embedding <=> $1::VECTOR` | `vectordb_cockroach.go:194,323-327` | supported | none (already production-shaped) |
| DDL in explicit tx | `store/migrator.go:91,178` wraps files in `Begin()` | not supported (mostly); `autocommit_before_ddl` | D4 branch: no `Begin()` for cockroach |
| Prepared statements across schema changes | runtime uses `database/sql` + simple protocol (no server-side prepared stmts) | risk 0A000 if extended protocol used | avoid `SELECT *`; note in §15 |

**Implementation step 6.1 (audit gate):** a full `diff` pass comparing every `store/migration/postgres/**/*.sql` statement against the Cockroach compatibility list above must be performed before generating the Cockroach files (task in §17, Phase 2). The only anticipated textual differences: `::BIGINT` on the 19 EXTRACT defaults and any `uuid`-related syntax checked at that time.

---

## 7. Migration execution design

### 7.1 Current behavior (Postgres, unchanged)
REPOSITORY FACT (`store/migrator.go`): `preMigrate` applies `LATEST.sql` inside `Begin()` when no migration history exists (lines 160-207); `Migrate` applies versioned files `> latestMigrationHistoryVersion` inside `Begin()` (lines 62-150); `execute()` tolerates "duplicate column"/"already exists"/"column already exists" errors by warning and continuing (lines 321-335) — migrations are re-runnable; history is upserted only after the batch commits (lines 142-149).

### 7.2 New behavior (Cockroach, driver == "cockroach")
1. Read the same files from `migration/cockroach/` (D3).
2. Do **not** call `Begin()`/`Commit()`.
3. Prepend `SET serial_normalization = 'sql_sequence';` (D2) to the whole-file statement string, then `db.ExecContext` the batch on a single connection.
4. Everything else (version comparison, skip logic, history upsert, idempotent-tolerance in `execute()`) is unchanged.

DOCUMENTATION FACT (MCP query 7): sending the batch as one unit over the simple protocol allows the server to handle statement-level retries automatically; DDL statements execute individually (online schema changes), with `autocommit_before_ddl` ensuring prior statements commit before each DDL.

### 7.3 Atomicity — explicit discussion (review Issue 3)
- **What Postgres behavior is changed:** today a failed statement rolls back all earlier statements in the file. On Cockroach, each statement commits independently; if statement N fails, statements 1..N-1 remain applied.
- **Why this is required by Cockroach:** online schema changes do not support all-or-nothing multi-statement DDL; this is architectural, not a choice we are making.
- **Why it is safe for bchat (mitigations, all already in place):**
  1. `migration_history` is written only after the full batch succeeds (`migrator.go:142-149`), so a failed boot re-runs the migration.
  2. All DDL is idempotent: `IF NOT EXISTS` everywhere in `LATEST.sql` (REPOSITORY FACT), `ON CONFLICT DO NOTHING` for seed data.
  3. `execute()` already swallows "already exists" class errors (REPOSITORY FACT: `migrator.go:326-330`).
  4. New Cockroach deployments start from `LATEST.sql` only (no production Cockroach database exists yet — INFERENCE from `prod_checklist.md`/plan.md, target is a fresh CockroachDB Cloud cluster on Fly.io), so there is no partial-history recovery scenario in the field.

### 7.4 `serial_normalization` proof plan (review Issue 1)
The review requires proof that `SET` → `CREATE TABLE` → persistent `nextval(...)` defaults. Experiment P1 (§12) runs the Cockroach `LATEST.sql` on single-node Docker, then issues `SHOW CREATE TABLE agent_tenants;` and asserts the column default is `nextval('..._seq'::regclass)` — not `unique_rowid()`. This is a **VERIFY-FIRST** gate, not an assumed truth.

---

## 8. Runtime driver design

### 8.1 Constructor and factory
- New: `postgres.NewCockroachDB(profile *profile.Profile) (store.Driver, error)` in the `store/db/postgres` package.
- New case in the factory: `store/db/db.go` — `case "cockroach": return postgres.NewCockroachDB(profile)`.
- REPOSITORY FACT: `store/db/db.go` factory switch is the single selection point; `store/driver.go` defines the `store.Driver` interface the implementation already satisfies (no interface changes needed — same methods, same 23 implementation files).

### 8.2 DSN
- Source: `COCKROACH_DSN` env var (D7). No fallback.
- REPOSITORY FACT: existing DSN builder at `store/db/postgres/postgres.go:33` appends `default_query_exec_mode=simple_protocol` — reused as-is for Cockroach.
- CockroachDB Cloud (Fly.io target): `postgresql://user:pass@host:26257/bchat?sslmode=verify-full` shape per `prod_checklist.md`; local dev uses `scripts/docker-compose.cockroach.yml` credentials (`bchat_user`/`bchat_pass`, `sslmode=disable`).
- Fail-fast: `--driver=cockroach` with empty `COCKROACH_DSN` aborts startup (INFERENCE: silent fallback is the failure mode D7 exists to prevent).

### 8.3 Pool settings
- Reuse Postgres settings: `MaxOpenConns(10)`, `MaxIdleConns(5)`, `ConnMaxLifetime(5m)`, `ConnMaxIdleTime(1m)`, 60s ping timeout (REPOSITORY FACT: `postgres.go:42`+).
- DOCUMENTATION FACT (MCP query 10): active connections across all pools should not greatly exceed ~4× cluster vCPUs. With 10 open per pool and the runtime pool + vector pool both bounded, a small CockroachDB Cloud cluster (2 vCPU) stays within guidance; if the cluster is smaller, lower `MaxOpenConns` via env (configuration, not code).

### 8.4 Vector database
- REPOSITORY FACT: `server/router/api/v1/agent/vectordb_cockroach.go` already implements the native `VECTOR(1536)` provider: `NewCockroachVectorDB` (line 27), runtime `CREATE TABLE` + `CREATE VECTOR INDEX` (lines 82-113), `crdb.ExecuteTx` upserts (lines 191-218), `<=>` cosine search (lines 323-327). No changes planned.
- Review Issue 5 (justify runtime-created schema): the schema exists only because the Cockroach vector provider is a compiled-in capability (build tag `cockroach`, Taskfile `build:cockroach`); when the build lacks it, no vector DDL exists at all — runtime creation is the correct boundary, consistent with the existing `VECTOR_DB_PROVIDER=cockroach` env gate (REPOSITORY FACT: Taskfile `crdb:check`).

---

## 9. Transaction retry design (review Issue 6)

### 9.1 Inventory of explicit transactions

| # | Site | Function | Transaction body | External side effects inside tx? |
|---|---|---|---|---|
| 1 | `bridge.go:112` | `createBridgeHandoffAttempt` | UPDATE lock, COUNT, MAX+1, INSERT; post-commit read | none |
| 2 | `bridge.go:356` | `CreateBridgeHandoffReplyIfActive` | `SELECT … FOR UPDATE`, INSERT, idempotent constraint branch | none |
| 3 | `bridge.go:500` | `CreateBridgeHandoffReplyAndOutboxIfActive` | `FOR UPDATE`, INSERT reply, INSERT outbox | none |
| 4 | `bridge.go:726` | `ClaimPendingBridgeReplyOutbox` | batch claim UPDATEs on status/expiry conditions | none |
| 5 | `bridge.go:818` | `CompleteClaimedBridgeReplyOutbox` | SELECT, UPDATE, idempotent branch | none |
| 6 | `bridge.go:912` | `FailClaimedBridgeReplyOutbox` | SELECT, UPDATE, idempotent branch | none |
| 7 | `bridge.go:1003` | `ClaimBridgeReplyOutboxByOutboxID` | SELECT, claim UPDATE | none |
| 8 | `agent.go:772` | `CreateAgentMessages` | batch INSERT loop | none |

Plus **non-transactional**: `agent.go:2773-2781` `ClaimPendingEvents` — single `UPDATE … FOR UPDATE SKIP LOCKED … RETURNING` statement (REPOSITORY FACT). Single statements execute in implicit transactions that CockroachDB retries automatically; no wrapper needed (INFERENCE, DOCUMENTATION FACT MCP query 1/7).

### 9.2 Side-effect audit (review Issue 6) — result
All 8 transaction bodies are **pure DB operations**: only `QueryRowContext`/`QueryContext`/`ExecContext` against the transaction handle, plus in-memory struct assembly and `uuid.NewString()` for new IDs. External effects (websocket delivery, outbox worker processing, HTTP responses) occur **after** `Commit()` in callers — e.g., `ClaimPendingBridgeReplyOutbox` returns the claimed items and callers deliver them. `uuid.NewString()` regenerates harmlessly on retry (a new unique ID is fine; it is never visible outside the tx result). Conclusion: converting each site to a retry closure introduces **no duplicate side effects**. This audit is recorded as verified, and a code-review check ("no I/O inside tx closures") is added to the acceptance criteria (§18).

### 9.3 Wrapper mechanics (cockroach-only)
- Each of the 8 sites converts to a closure `func(tx *sql.Tx) error` run via `crdb.ExecuteTx(ctx, d.db, nil, fn)` (API at module cache `crdb/tx.go:299`, v2.4.3).
- `crdb.ExecuteTx` detects SQLSTATE 40001/40003/08xxx retryable errors and re-runs the closure with backoff (DOCUMENTATION FACT: transaction-retry pattern in the pgx tutorial, MCP query 4; repository precedent `vectordb_cockroach.go:191`).
- Non-retryable errors propagate unchanged to callers; the existing `store.ErrBridge*` sentinel logic is untouched.
- The string-matching `isPostgresRetryable` loop at `bridge.go:95-104` remains for Postgres; the Cockroach path simply doesn't reach it (the closure retry replaces it).
- `RunResiliently` (`store/db/postgres/resilience.go`, maxRetries 5, backoff 1s, transient codes 57P01/08006/08003/08001/08004, 23505-as-success) has zero callers today (REPOSITORY FACT). **Not activated** in this plan — the lead's Q5 decision was the 40001 wrapper only; wiring `RunResiliently` is future work (§16).

---

## 10. Files to create / modify

### Create
| File | Purpose |
|---|---|
| `store/migration/cockroach/LATEST.sql` | Full schema for fresh Cockroach deployments; mirror of postgres with `::BIGINT` on EXTRACT defaults (§6.2) |
| `store/migration/cockroach/0.19/00__*.sql` … `store/migration/cockroach/0.35/*.sql` | File-for-file mirror of `store/migration/postgres/0.19…0.35` (count: same as postgres; verify with parity script) |
| `store/db/postgres/cockroach.go` | `NewCockroachDB` constructor (shared internals with `postgres.go`) |

### Modify
| File | Change |
|---|---|
| `store/db/db.go` | add `case "cockroach"` to the driver factory |
| `store/migrator.go` | Cockroach branch: skip `Begin()`; inject `SET serial_normalization = 'sql_sequence';` prefix before whole-file exec |
| `store/db/postgres/bridge.go` | wrap 8→7 tx sites (112, 356, 500, 726, 818, 912, 1003) in `crdb.ExecuteTx` closures (cockroach-only path) |
| `store/db/postgres/agent.go` | wrap tx site 772 (CreateAgentMessages) in `crdb.ExecuteTx` closure (cockroach-only) |
| `internal/profile/profile.go` | accept/validate `Driver == "cockroach"`; expose `COCKROACH_DSN` |
| `bin/memos/main.go` | bind `COCKROACH_DSN`; fail fast when driver=cockroach and DSN empty |
| `scripts/validate-parity.sh` | extend file-list parity to postgres↔cockroach (sqlite↔cockroach covered by transitivity; document sqlite's 0.2–0.18 legacy dirs as known divergence) |
| `Taskfile.yml` | `run:cockroach`/`crdb:*` targets: pass `--driver=cockroach` + `COCKROACH_DSN`; add migration CI target |
| `store/migration_helper.go` | **no change** — SQLite-only (`PRAGMA table_info`) helpers must not be ported (REPOSITORY FACT) |

### Unchanged (explicitly)
All 23 `store/db/postgres/*.go` implementation files except the two tx-site files above; `store/driver.go` interface; `store/agent.go` types; `store/migrator.go` version logic; `server/router/api/v1/agent/vectordb_cockroach.go`.

---

## 11. Config & environment

| Item | Value |
|---|---|
| Driver flag | `--driver=cockroach` |
| DSN env | `COCKROACH_DSN` (no fallback to `DATABASE_URL`) |
| Protocol | `default_query_exec_mode=simple_protocol` (appended in DSN builder) |
| Local dev | `scripts/docker-compose.cockroach.yml` (already exists; `bchat_user`/`bchat_pass`@`localhost:26257/bchat`, `sslmode=disable`, console :8080) |
| Single-node Docker bootstrap | `COCKROACH_DATABASE`/`COCKROACH_USER`/`COCKROACH_PASSWORD` env vars or init scripts (DOCUMENTATION FACT, MCP query 8) |
| Production | Fly.io + CockroachDB Cloud; DSN shape per `bugs/057/prod_checklist.md`; TLS via `sslmode=verify-full` |
| Build tag | `cockroach` (existing Taskfile pattern) |

---

## 12. Experiments (VERIFY-FIRST gates, must run before implementation)

| ID | Question | Method | Pass criterion |
|---|---|---|---|
| P1 | Does `SET serial_normalization='sql_sequence'` produce `nextval(...)` defaults through the whole-file path? | Run `LATEST.sql` (prefixed with SET) on single-node Docker via `crdb sql` / app migrator; `SHOW CREATE TABLE` | Defaults show `nextval('…_seq')`, not `unique_rowid()` |
| P2 | Does whole-file multi-statement exec (no `Begin`) migrate cleanly on Cockroach, and is a failed re-run idempotent? | Execute the full Cockroach `LATEST.sql`; delete `migration_history` row; re-run; also inject a failing statement mid-file and verify history-not-written + successful re-run | Migration succeeds; re-run is no-op via `IF NOT EXISTS`; failed run leaves no history |
| P3 | Is `EXTRACT(EPOCH FROM NOW())::BIGINT` accepted as a column default and correct? | `CREATE TABLE t (created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT); INSERT INTO t DEFAULT VALUES; SELECT created_ts;` | Inserts succeed; value equals unix epoch seconds |
| P4 | Does `crdb.ExecuteTx` retry correctly under contention? | Run `ClaimPendingBridgeReplyOutbox`-style claim from two concurrent connections against the same outbox rows; force a 40001 by conflicting writes | Exactly one claimer wins per row; no duplicated rows; 40001 observed in logs and retried |
| P5 | SKIP LOCKED claim still correct after port | Run `ClaimPendingEvents` (`agent.go:2781` query) with multiple concurrent workers on Cockroach | Each event claimed exactly once; no double-processing (guards existing behavior, not a new feature) |
| P6 | Vector path unaffected | Existing `crdb:test` suite (`TestProcessPendingTickets` et al., Taskfile line ~325) | All green with `VECTOR_DB_PROVIDER=cockroach` |

---

## 13. Adversarial pass — every behavioral change defended (review's final prompt)

For each deviation from the PostgreSQL implementation: (1) exact PG behavior changed, (2) why Cockroach requires it, not merely convenient, (3) new failure modes/regressions.

### 13.1 Migrations: explicit-transaction → autocommit whole-file
1. **Changed:** a migration file is no longer atomic; statement N failure leaves 1..N-1 applied.
2. **Required:** Cockroach online schema changes do not support multi-statement DDL transactions; `autocommit_before_ddl` would dissolve the tx anyway (MCP query 6). Keeping `Begin()` would be cosmetic and misleading.
3. **New failure modes:** partial application on failure. Mitigated by idempotent DDL (`IF NOT EXISTS`), duplicate-tolerance in `execute()` (`migrator.go:326-330`), and history-after-success (§7.3). Residual risk: a *non-idempotent* future migration could fail on re-run — mitigated by requiring all Cockroach migrations to be idempotent (rule added to §18 acceptance criteria) and validated by P2.

### 13.2 Migrations: injected `SET serial_normalization`
1. **Changed:** none on Postgres (injection is cockroach-only, inside the migrator branch).
2. **Required:** `int32` IDs overflow with `unique_rowid()` (D2 evidence chain). Without this, every INSERT on Cockroach eventually fails with an out-of-range int32 scan error.
3. **New failure modes:** none beyond the file itself: the SET is a session variable on the executing connection only; it cannot leak into other pools or sessions. Risk that it silently doesn't apply → P1 gates this.
4. Regression risk: `sql_sequence` mode performs one `nextval` round-trip per insert (vs. client-generated rowid). For bchat's write volumes (chat messages, agent events) this is negligible (INFERENCE); if it ever matters, `sql_sequence_cached` is the documented alternative (MCP query 2).

### 13.3 DDL: explicit `::BIGINT` casts on EXTRACT defaults
1. **Changed:** Cockroach files differ textually from Postgres files (19 defaults). Postgres: `numeric → bigint` via assignment cast (rounds); Cockroach: explicit `float → int8` cast (truncates). Both yield whole-second epoch values; a ≤1s difference is theoretically possible at .5s fractional boundaries.
2. **Required:** `extract(...) → float` in Cockroach and no implicit float→int8 assignment (MCP query 3); without the cast the DDL fails at CREATE TABLE time.
3. **New failure modes:** none beyond the DDL validation itself, gated by P3. The runtime query `migration_history.go:39` needs no change because it relies on the column default, which the Cockroach DDL now defines correctly.

### 13.4 Runtime: retry closures on 8 tx sites
1. **Changed:** transaction code becomes a closure; a 40001 causes re-execution instead of error propagation.
2. **Required:** Cockroach documentation requires client-side retry handling for 40001 (MCP query 9); without it, transient conflicts surface as user-visible failures on a multi-instance Fly.io deployment (INFERENCE).
3. **New failure modes:** duplicate side effects on retry — audit in §9.2 found none inside closures. ID generation (`uuid.NewString()`) inside closures is safe. The only behavioral nuance: a retried closure may observe newer data on the second attempt — acceptable for these functions (all are either claim/idempotent-by-design or append-only inserts; INFERENCE).

### 13.5 Driver selection/config
1. **Changed:** a new driver name (`cockroach`) and DSN source (`COCKROACH_DSN`). No behavior change to Postgres/SQLite paths.
2. **Required:** distinct DSN prevents cross-wiring mistakes in production (D7 rationale).
3. **New failure modes:** misconfiguration (driver=cockroach without DSN) → fail-fast startup error.

### 13.6 Protocol: simple everywhere
1. **Changed:** none vs. current Postgres driver (already `simple_protocol`).
2. **Required:** no — this is the conservative choice; the review's split-protocol idea is optional performance work, not a compatibility requirement. Kept as-is for parity; deferred split documented (§16).

### 13.7 Runtime-created vector schema
1. **Changed:** none — behavior identical to today's `vectordb_cockroach.go`.
2. **Required:** no — kept, with the review's requested justification: the schema exists only when the VECTOR capability is compiled in (`cockroach` build tag), making runtime creation the correct boundary.

---

## 14. CI & validation

| Check | Today | After |
|---|---|---|
| File-list parity | `scripts/validate-parity.sh` checks sqlite↔postgres (exit 1) | extends to postgres↔cockroach; sqlite↔cockroach by transitivity with the 0.2–0.18 known-divergence list |
| Schema parity (best-effort lint) | sqlite↔postgres `LATEST.sql` table/index comparison (exit 2, warn) | add cockroach; expect cockroach↔postgres identical except `::BIGINT` cast text (lint compares names only) |
| Migration drift | `scripts/validate-pg-migrations.sh`, `scripts/validate-migrations.sh` (LATEST.sql vs cumulative) | parallel scripts for cockroach, or parameterize driver |
| Runtime tests | `crdb:test` (`go test -tags cockroach ./server/router/api/v1/agent/...`) | add store-layer tests: migrate on Docker Cockroach, CRUD smoke, concurrent claim tests (P4/P5) |
| Build | `build:cockroach` (tag `cockroach`) | unchanged; plus `go build` without tag must still exclude cockroach-only code paths from tests |

---

## 15. Risks & known limitations

| Risk | Detail | Mitigation |
|---|---|---|
| SKIP LOCKED upstream bug #167582 | Under rare conditions, SKIP LOCKED may skip rows whose lock is a stale intent from a committed transaction (GitHub issue surfaced via MCP query 1). Affects `ClaimPendingEvents` claim completeness, not correctness of claiming | Monitor; claims are re-scanned each poll cycle so skipped rows are picked up on the next pass (INFERENCE from poller design). No code change now |
| 0A000 "cached plan must not change result type" | Prepared statements can break across online schema changes | Runtime uses simple protocol (no server-side prepared statements in this path); avoid `SELECT *` (MCP query 12) |
| Migration atomicity | Files apply statement-by-statement | §7.3 mitigations; idempotency rule; P2 gate |
| int32 ID space | `sql_sequence` keeps values in int32 range but the space is 2.1B per table | Acceptable for bchat scale (INFERENCE); note in docs; `BIGSERIAL` upgrade is a future decision, not part of this port |
| EXTRACT float→int truncation | ≤1s deviation possible at .5s boundaries vs. Postgres rounding | Deterministic explicit cast; no business logic depends on sub-second precision (REPOSITORY FACT: all `created_ts`/`updated_ts` consumers read Unix seconds) |
| Vector index limitations | No `IMPORT INTO` on vector-indexed tables; large batch inserts degrade | Existing provider avoids IMPORT; inserts are single-row UPSERTs (`vectordb_cockroach.go:191`) |
| Pool sizing | 10 open connections × 2 pools vs. ~4×vCPU guidance | Configurable; verify against actual cluster size at deploy (prod_checklist.md) |
| simple_protocol performance | Text results, no binary protocol | Accepted; benchmark gate for the split (§16) |

---

## 16. Out of scope / future work (recorded, not implemented)

- Split protocols (review Issue 4): migrator simple / runtime extended — gated on a benchmark showing material runtime gain.
- `NewSQLDriver(profile)` capability factory (review Issue 7) — naming-level refactor, defer until a third driver appears.
- Wiring `RunResiliently` (connection-level retry) — currently zero-caller code; revisit if Cockroach connection flaps appear in production.
- TLS/cert rotation, backups, locality setup for CockroachDB Cloud — operational follow-up per `prod_checklist.md`, not code.
- `BIGSERIAL`/int64 ID migration — future schema evolution.
- pgvector provider parity — not used by bchat; the native `VECTOR` provider is the only Cockroach vector path.

---

## 17. Implementation task list (ordered; no code in this document)

**Phase 1 — Experiments (gate for everything else):** P1, P2, P3 (§12) on single-node Docker.
**Phase 2 — Migration files:** mirror `store/migration/postgres/` → `store/migration/cockroach/` with the §6.1 audit and `::BIGINT` fixes; run parity script; P4/P5/P6.
**Phase 3 — Migrator branch:** cockroach path (no `Begin`, SET injection) in `store/migrator.go`; idempotency rule applied to all Cockroach files.
**Phase 4 — Driver:** `store/db/postgres/cockroach.go` (`NewCockroachDB`), `store/db/db.go` factory case, profile/main.go wiring, fail-fast DSN validation.
**Phase 5 — Retries:** convert the 8 tx sites to `crdb.ExecuteTx` closures (cockroach-only); re-verify with P4.
**Phase 6 — CI/tooling:** extend `validate-parity.sh`, parameterize migration-drift scripts, Taskfile targets, `run:cockroach` with `COCKROACH_DSN`.
**Phase 7 — E2E validation:** fresh-deploy on CockroachDB Cloud (Fly.io) + `crdb:test` + chat smoke test.

---

## 18. Acceptance criteria

1. `--driver=cockroach` + `COCKROACH_DSN` boots on local single-node Docker and on CockroachDB Cloud; fail-fast without DSN.
2. Fresh Cockroach DB migrates via `LATEST.sql`; `SHOW CREATE TABLE` proves `nextval(...)` defaults (P1).
3. Migration re-run and mid-file failure behave per §7.3 (P2); all Cockroach migration files are idempotent.
4. Postgres driver behavior is byte-for-byte unchanged (all existing tests green).
5. All 8 tx sites use `crdb.ExecuteTx` on the cockroach path; audit confirms no I/O inside closures (§9.2) — enforced by code review checklist.
6. `ClaimPendingEvents` SKIP LOCKED claim works under concurrent workers with exactly-once claiming (P5).
7. Parity validator passes with cockroach included; migration-drift scripts pass.
8. Vector path green (`crdb:test` with `VECTOR_DB_PROVIDER=cockroach`).
9. Every claim in the final implementation commit message/PR description cites repo evidence or MCP-verified docs per §0.
