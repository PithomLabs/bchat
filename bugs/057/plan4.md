# Bug 057 Plan v4 — CockroachDB Database Support (Implementation Plan)

**Status:** Ready for implementation review
**Supersedes:** `plan3.md` (v3), `plan2_deepseek.md` (v2), `plan.md` (v1)
**Addressed reviews:** `plan3_review.md` (9.6/10 — this revision), `plan2_deepseek_review.md` (9.3/10), `plan2_review.md`, `plan2_review_chatgpt.md`, `plan_review.md`, `plan_review_chatgpt.md`
**Prepared:** 2026-08-02
**Scope:** Production migration + runtime support for CockroachDB as a first-class driver alongside SQLite and PostgreSQL. This is a planning document — no code changes.

---

## 0. Evidence classification (adopted from plan2_deepseek_review.md "Rules", retained per plan3_review §1)

| Tag | Meaning | Citation required |
|-----|---------|------------------|
| **REPOSITORY FACT** | Derived from bchat source code | `file:line` |
| **DOCUMENTATION FACT** | Derived from official CockroachDB docs via the CockroachDB MCP | doc name + version/section |
| **INFERENCE** | Logical conclusion from verified facts | must say "This is an inference" |
| **SPECULATION** | Recommendation only | must start with "I recommend" |
| **PROHIBITED** | "Cockroach doesn't support X" unless official docs say so | — |

**Evidence rule:** every non-trivial engineering claim is backed by either (1) repository evidence, (2) official CockroachDB documentation (obtained via the CockroachDB MCP), or (3) an explicitly labeled experiment that must run before implementation. Claims that cannot be supported are removed.

---

## 1. Background context — interactive Q&A with project lead (2026-08-02)

Before writing v3, the lead and the architect held an interactive Q&A session. All seven decisions were confirmed by the lead and remain binding constraints; they are unchanged in this revision.

| # | Question | Decision (lead) |
|---|----------|-----------------|
| Q1 | Migration execution strategy (review Issues 2 & 3: splitter, DDL atomicity) | **Whole-file, no transaction wrapper.** One `ExecContext` per file on one connection; no custom SQL splitter; `Begin()` skipped for Cockroach; atomicity loss documented and mitigated (§7) |
| Q2 | `serial_normalization` application | **Inject in migrator code.** Cockroach branch prepends `SET serial_normalization = 'sql_sequence';` as the first statement of the batch; SQL files stay byte-identical to Postgres where possible; PoC proof via `SHOW CREATE TABLE` (P1, §12) |
| Q3 | Migration layout | **Mirror versioned directories.** `store/migration/cockroach/` = `0.19/`…`0.35/` + `LATEST.sql`, file-for-file with postgres |
| Q4 | Runtime query protocol | **`simple_protocol` everywhere** (migration + runtime), identical to the current Postgres driver; split-protocol deferred (§16) |
| Q5 | Runtime transaction retries (40001) | **`crdb.ExecuteTx` retry wrapper on the 8 `BeginTx` sites** (cockroach-only); side-effect audit included (§9); errors propagate when retries exhausted |
| Q6 | Driver construction | **`postgres.NewCockroachDB()`** in the existing postgres package; `NewSQLDriver(profile)` capability factory deferred (§16) |
| Q7 | Connection configuration | **`COCKROACH_DSN` env var + `--driver=cockroach`**; no fallback to `DATABASE_URL`; fail-fast if missing |

---

## 2. Methodology and evidence appendix

### 2.1 Methodology (condensed)

Every CockroachDB behavior claim in this plan is a **DOCUMENTATION FACT** retrieved and verified via the CockroachDB MCP (`cockroachdb_search_cockroach_db_knowledge_sources`), which performs semantic retrieval over official CockroachDB documentation — including per-version pages (`stable`/v26.2, v25.x, v24.x, v23.x) — and upstream GitHub issues. Repository claims are **REPOSITORY FACTS** with `file:line`. Where the MCP surfaced conflicting sources (e.g., an outdated blog claiming `SKIP LOCKED` is unsupported), the current official docs won and the contradiction is recorded (§2.2, row 2). Historical search queries are intentionally omitted here per plan3_review's "Remove"; the appendix below is the implementer-facing form.

### 2.2 Evidence appendix (topic → official doc → repository evidence → decision)

| Topic | Official documentation (via MCP) | Repository evidence | Decision |
|---|---|---|---|
| `SKIP LOCKED` | Supported wait policy for `FOR UPDATE`/`FOR SHARE`, every version v23.2–v26.2. Older blog claiming otherwise is outdated | `agent.go:2773-2781` single `UPDATE…RETURNING` claim | Keep as-is; note upstream bug #167582 (§15) |
| `serial_normalization` | Session variable, `SET`/`SHOW`; options `rowid`/`virtual_sequence`/`sql_sequence`/`sql_sequence_cached`/`unordered_rowid` (+`sql_sequence_cached_node` v24.1+); default `rowid` = 64-bit `unique_rowid()` | All IDs `int32` (`store/agent.go`, `store/driver.go`); `SERIAL PRIMARY KEY` everywhere (`LATEST.sql`) | Migrator injects `SET serial_normalization='sql_sequence'` (D2, §5.2); P1 proves `nextval()` |
| `EXTRACT(EPOCH FROM NOW())` | `extract(timestamp/timestamptz, 'epoch') → float` in all versions v23.1–v26.2; no implicit float→int8 assignment | 19 defaults in `LATEST.sql`; runtime relies on default in `migration_history.go:39-45` | Explicit `::BIGINT` in Cockroach DDL (§6.2); P3 |
| DDL in explicit transactions | Mostly unsupported; `CREATE TABLE`/`CREATE INDEX` allowed; `autocommit_before_ddl` commits before DDL | `migrator.go:91,178` wraps files in `Begin()` today | No `Begin()` for Cockroach (§7.2) |
| Batched multi-statement simple protocol | Semicolon-separated strings as one unit support automatic retries | `migrator.go:321-335` executes whole file as one `ExecContext` | Whole-file exec, no splitter (§7.1) |
| `FOR SHARE`/`NOWAIT` | `FOR SHARE` = compat no-op; `NOWAIT` supported | `FOR UPDATE` at `bridge.go:356,500` | No change |
| Client retry requirement | SQLSTATE 40001 requires app-level retry handling | `crdb.ExecuteTx` at `vectordb_cockroach.go:191` (precedent); `isPostgresRetryable` string hack at `bridge.go:320` | `crdb.ExecuteTx` wrapper on 8 tx sites (§9) |
| Connection pooling | Active connections across all pools ≈ ≤4× cluster vCPUs (production checklist) | `postgres.go:42-48` pool settings 10/5/5m/1m | Preserve defaults initially; tune operationally (§8.3) |
| Vector indexes | No large batch inserts; no `IMPORT INTO` on vector-indexed tables; prefix-column filters only | `vectordb_cockroach.go:191-218` single-row UPSERTs | No change (§8.4) |
| Prepared statements vs schema changes | Error 0A000 `cached plan must not change result type`; avoid `SELECT *` | Runtime uses simple protocol (no server-side prepared statements) | Note only (§15) |
| `UPDATE … FROM` | Supported v23.1+; join-output ambiguity warning | `0.24/01__memo_pinned.sql` uses it (legacy, never runs on Cockroach — §7.4) | Allowed; review-required in drift check |
| `ALTER TYPE` | Supported v23.1+ (ADD/RENAME/DROP VALUE, SET SCHEMA); `DROP VALUE` jobs non-cancellable | none today | Allowed; review-required in drift check (§10) |
| `CREATE EXTENSION` | Not fully supported; uuid-ossp functions available without the extension | none today | Forbidden in Cockroach migrations (§10) |
| LISTEN/NOTIFY | Not implemented (`LISTEN` is a syntax error); not on roadmap | none today | Forbidden (§10) |
| Advisory locks | `pg_advisory_lock()` unknown function; some no-op stubs exist | none today | Forbidden (§10) |
| `CREATE DOMAIN` | Not supported (stable docs) | none today | Forbidden (§10) |
| Range types, `MACADDR`, `MONEY` | Not supported (v25.4 sqlx findings + stable docs) | none today | Forbidden (§10) |
| Triggers | Not supported (v24.2 docs) | none today | Forbidden (§10) |
| `DEFERRABLE` constraints | Not supported (v25.4 syntax error) | none today | Forbidden (§10) |
| Drop primary key | Not supported; drop+add PK in one transaction | none today | Forbidden (§10) |
| PL/pgSQL `DO` blocks | Partial; `DO $$…IF NOT EXISTS` unimplemented (v25.4) | all migration files are plain SQL (verified) | Forbidden (§10) |
| INT8 OID behavior | Literal expressions return INT8 regardless of `default_int_size` (v25.4 sqlx issue) | pgx stdlib + simple protocol scans by value, not OID | No change; int32 safety rests on `sql_sequence` values (§15) |
| Single-node Docker bootstrap | `COCKROACH_DATABASE`/`USER`/`PASSWORD` env vars; init scripts only on empty data dir | `scripts/docker-compose.cockroach.yml` exists | Reuse for local dev (§11) |

---

## 3. Capability matrix (review: "biggest thing still missing" in v2; retained in v4)

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
| `UPDATE … FROM` | yes | yes (v23.1+) | yes (legacy files only) | allowed; drift-check review-required |
| Multi-statement simple-protocol exec | yes | yes (batched statements) | yes (migrations) | whole-file exec, no tx wrapper (§7) |
| DDL in explicit transactions | yes | **no** (mostly) | n/a | no `Begin()` for Cockroach migrations (§7) |
| Atomic whole-file migration | yes | **no** | n/a | documented loss + idempotency mitigations (§7.3) |
| LISTEN/NOTIFY, advisory locks, triggers, `CREATE DOMAIN`, ranges | yes | **no** | no | forbidden by drift policy (§10) |

---

## 4. Why the shared implementation remains maintainable (plan3_review #1)

The v3 plan proved "today's repository can share implementation." This section addresses the reviewer's question: *why does it remain maintainable as Cockroach-specific features evolve?*

**Claim (INFERENCE):** the shared implementation has exactly four bounded divergence seams, and every Cockroach-specific behavior that exists today or is foreseeable lands in one of them:

| Seam | What differs | Where it lives | Evolution path |
|---|---|---|---|
| DDL dialect | `::BIGINT` casts, `SET serial_normalization`, idempotency | `store/migration/cockroach/` files only | new migration file per change; postgres files untouched |
| Transaction semantics | 40001 retry required | retry wrapper around 8 tx sites (cockroach branch) | if Postgres gains retry, wrapper becomes shared |
| Connection protocol | none today (`simple_protocol` everywhere) | DSN builder in `postgres.go` | per-driver DSN options already parameterized |
| Capability availability | vector provider | separate provider file, `vectordb_cockroach.go` (existing precedent) | new providers are new files, never new driver packages |

**Boundary rule:** a feature may *not* enter the shared implementation via conditional-on-driver logic inside individual SQL methods. It must enter via one of the seams above. If a future feature genuinely cannot fit a seam (e.g., Cockroach-only SQL constructs in a hot path), the documented escalation is to introduce a driver-capability interface (e.g., `type Capabilities interface { RequiresRetry() bool }`) consumed by the store layer — the same pattern the agent package already uses for vector providers. This is deliberately NOT introduced now, because no current or foreseeable bchat feature needs it (INFERENCE from the §6 audit: all 50+ SQL constructs used today are portable).

**Why a separate `store/db/cockroach/` package remains wrong (INFERENCE):** it would duplicate 23 implementation files and fork `store.Driver` conformance, which is exactly the C-1/C-2 blocker the v2 review identified; the seams above give maintainers a cheaper, testable divergence point.

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
        └── runtime retries ──► crdb.ExecuteTx wrapper around 8 BeginTx sites (cockroach-only)
```

- REPOSITORY FACT: driver factory switch at `store/db/db.go` (`case "sqlite"`, `case "mysql"`, `case "postgres"`, default error); a `case "cockroach"` is added.
- REPOSITORY FACT: `NewDB` at `store/db/postgres/postgres.go:22`; appends `default_query_exec_mode=simple_protocol` (line 33); `MaxOpenConns(10)` (line 42), `MaxIdleConns(5)`, `ConnMaxLifetime(5m)`, `ConnMaxIdleTime(1m)`, 60s ping.

### 5.1 D1 — Whole-file migration execution, no explicit transaction (Q1)
REPOSITORY FACT: `store/migrator.go:321-335` — `execute()` runs the entire file via one `tx.ExecContext(ctx, stmt)`; no splitter exists. DOCUMENTATION FACT: simple-protocol multi-statement strings support automatic retries; most DDL cannot run in explicit transactions and `autocommit_before_ddl` commits before DDL regardless. INFERENCE: `Begin()` adds no atomicity on Cockroach and misleads readers; the Cockroach branch drops it.

### 5.2 D2 — `SET serial_normalization = 'sql_sequence'` injected by the migrator (Q2)
DOCUMENTATION FACT: session variable controlling SERIAL handling in table definitions. REPOSITORY FACT: all IDs are `int32`; default `rowid` mode yields 64-bit `unique_rowid()` values (DOCUMENTATION FACT). INFERENCE: `sql_sequence` is required to keep IDs in int32 range (sequences start at 1); a hard compatibility requirement.

### 5.3 D3 — Mirror versioned migration directories (Q3)
REPOSITORY FACT: base path `migration/{driver}/` (`migrator.go:210`); `GetCurrentSchemaVersion` scans `*/*.sql` (line 257); version parse (line 300); history upsert only after success (lines 142-149, 192-199). INFERENCE: mirrored `store/migration/cockroach/` works with zero migrator-structure changes beyond the Cockroach branch.

### 5.4 D4 — simple_protocol everywhere (Q4)
REPOSITORY FACT: `postgres.go:33` forces simple protocol today. INFERENCE: identical runtime behavior, zero protocol coupling, smallest test surface. The review's split-protocol idea is deferred with a benchmark gate (§16).

### 5.5 D5 — `crdb.ExecuteTx` retry wrapper on 8 tx sites (Q5)
REPOSITORY FACT: `cockroach-go/v2 v2.4.3` in `go.mod`; `ExecuteTx(ctx, *sql.DB, opts, fn)` exists in v2.4.3 (module cache `crdb/tx.go:299`); precedent at `vectordb_cockroach.go:191`; `isPostgresRetryable` string-matching at `bridge.go:320` does not match CRDB 40001 messages. INFERENCE: SQLSTATE-based retry supersedes the string hack on the Cockroach path.

### 5.6 D6 — `postgres.NewCockroachDB()` (Q6)
Same package, shared internals (per §4). No `store/db/cockroach/` directory is created (REPOSITORY FACT: none exists today).

### 5.7 D7 — `COCKROACH_DSN`, no fallback (Q7)
REPOSITORY FACT: `internal/profile/profile.go` carries `Driver`/`DSN`; `bin/memos/main.go` binds them via viper. INFERENCE: explicit DSN prevents cross-wiring; fail-fast at startup. REPOSITORY FACT: existing Cockroach tooling is reused — `scripts/docker-compose.cockroach.yml` (`bchat_user`/`bchat_pass`@`localhost:26257/bchat?sslmode=disable`, console :8080), Taskfile `build:cockroach`/`run:cockroach`/`crdb:check`/`crdb:db-check`/`crdb:test`/`crdb:docker:build`/`crdb:docker:run` (build tag `cockroach`).

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
| `ILIKE` | `memo_filter.go:143,172,209,224` | supported | none |
| `jsonb` + `@>` | `memo_filter.go` | supported | none |
| `::BOOLEAN` / `::BIGINT` casts | `memo.go:106-115`; `agent.go:2605,2693,2718,2776,2780` | supported (explicit casts) | none |
| `UPDATE … FROM` | legacy `0.24/01__memo_pinned.sql` (never runs on Cockroach — §7.4) | supported (v23.1+) | allowed; review-required in drift check |
| `CREATE VECTOR INDEX` | `vectordb_cockroach.go:109` (runtime) | supported; **no `IF NOT EXISTS`** | runtime-only; already handled |
| `INSERT … $6::VECTOR` / `<=>` | `vectordb_cockroach.go:194,323-327` | supported | none |
| DDL in explicit tx | `migrator.go:91,178` | not supported (mostly) | no `Begin()` for Cockroach |
| Forbidden constructs (drift check) | none found in repo (verified) | see §10 | CI gate |

**Implementation step 6.1 (audit gate):** a full statement-by-statement `diff` between every `store/migration/postgres/**/*.sql` file and its Cockroach mirror must be performed in Phase 2. The only anticipated textual differences: `::BIGINT` on the 19 EXTRACT defaults.

---

## 7. Migration execution design

### 7.1 Current behavior (Postgres, unchanged)
REPOSITORY FACT (`store/migrator.go`): `preMigrate` applies `LATEST.sql` inside `Begin()` when no history exists (lines 160-207); `Migrate` applies versioned files `> latestMigrationHistoryVersion` inside `Begin()` (lines 62-150); `execute()` tolerates "duplicate column"/"already exists"/"column already exists" errors (lines 321-335); history upsert only after batch success (lines 142-149).

### 7.2 New behavior (Cockroach)
1. Read files from `migration/cockroach/` (D3).
2. No `Begin()`/`Commit()`.
3. Prepend `SET serial_normalization = 'sql_sequence';` (D2) to the file content, then `db.ExecContext` the batch on one connection.
4. All version logic, skip logic, history upsert, and idempotent tolerance unchanged.

DOCUMENTATION FACT: sending the batch as one unit over the simple protocol allows statement-level automatic retries; DDL executes as individual online schema changes with `autocommit_before_ddl` committing prior statements first.

### 7.3 Atomicity — explicit discussion (review Issue 3, retained)
- **Changed vs Postgres:** failed statement N no longer rolls back statements 1..N-1.
- **Required by Cockroach:** online schema changes do not support all-or-nothing multi-statement DDL; keeping `Begin()` would be cosmetic.
- **Safe for bchat because:**
  1. `migration_history` is written only after full success (`migrator.go:142-149`) → failed boot re-runs the migration.
  2. All DDL is `IF NOT EXISTS` (REPOSITORY FACT: `LATEST.sql`); the only data statement is `ON CONFLICT (tenant_id, code) DO NOTHING` (`LATEST.sql:201-208`). **The LATEST.sql file is fully idempotent** — verified statement-by-statement (REPOSITORY FACT, new in v4).
  3. `execute()` swallows "already exists" class errors (`migrator.go:326-330`).
  4. Fresh Cockroach deployments run `LATEST.sql` only (§7.4), so no partial-history recovery exists in the field at first deploy.

### 7.4 Which files actually execute on Cockroach (new in v4 — repository-derived)
REPOSITORY FACT (`migrator.go`): on a fresh database, `preMigrate` applies `LATEST.sql`, then upserts history = `schemaVersion` (0.35). `Migrate` then applies only versioned files **greater than** the latest history version. Therefore:
- **On a fresh Cockroach deployment: only `LATEST.sql` executes.** The versioned dirs `0.19/`…`0.35/` never execute; they exist solely to satisfy the version machinery (`GetCurrentSchemaVersion` scans `*/*.sql`, `migrator.go:257`) and parity validation.
- **On future upgrades (0.36+): only the new files execute.**
- Consequences (INFERENCE):
  1. The legacy data-backfill statements in 0.19–0.24 (e.g., the random-UUID `UPDATE memo SET resource_name = uuid_in(md5(random()::text || random()::text)::cstring)` at `0.19/00__add_resource_name.sql:3` — **non-idempotent**, would rewrite values on re-run) are **inert on Cockroach**. They are mirrored file-for-file for parity but never run.
  2. The idempotency rule (§7.5) governs only **future** Cockroach migrations (0.36+), where the recovery scenario actually applies.

### 7.5 Recovery after partially applied migration (plan3_review #2)

Scenario: future incremental migration `0.36/00__x.sql` fails at statement 4; statements 1–3 applied; history row NOT written; developer edits the file; redeploy.

**Recovery procedure (ordered):**
1. **Detect:** boot fails with the migration error; `migration_history` has no 0.36 row (REPOSITORY FACT: upsert happens only after success). Logs identify the failing statement.
2. **Assess:** determine which statements applied via `SELECT * FROM migration_history;` plus `SHOW TABLES` / `information_schema` inspection (the `crdb:sql:shell` target).
3. **Two allowed paths:**
   - **Path A — idempotent file (default):** fix the failing statement in the unreleased file, redeploy. All applied statements re-execute safely because the file is idempotent (§7.3 items 2-3). This is the required norm for all Cockroach migrations.
   - **Path B — non-idempotent data transform:** if a transform is not idempotent (e.g., random-value UPDATE), the file **must not be edited in place** after any statement applied. Instead: (a) leave the file as-is, (b) add a NEW file `0.36/01__fix.sql` that compensates or completes the transform, (c) redeploy. Editing applied files is prohibited (see below).
4. **Escalation — fresh deployment only:** because Cockroach has no production data at first deploy (INFERENCE: first deployment is the initial Fly.io rollout), a stuck migration can also be resolved by dropping and re-creating the database, then redeploying. This option is *not* available once production data exists.

**Migration file lifecycle rule (new, enforced by §10 drift policy and code review):**
- Files that have never succeeded (no history row) may be edited freely within the same development cycle.
- Once a file has a history row, **never edit it** — all fixes ship as new files (Path B). This protects both Postgres (whose atomic migrations make editing retroactive and untested) and Cockroach (whose partial application makes editing dangerous).
- Every Cockroach migration must be idempotent: DDL with `IF NOT EXISTS`, inserts with `ON CONFLICT DO NOTHING`, transforms with deterministic, WHERE-guarded values. The 0.19 random-UUID pattern is the canonical counter-example and is banned going forward.

---

## 8. Runtime driver design

### 8.1 Constructor and factory
New: `postgres.NewCockroachDB(profile *profile.Profile) (store.Driver, error)`; new factory case in `store/db/db.go`. REPOSITORY FACT: `store/driver.go` interface needs no changes — the same 23 implementation files already satisfy it.

### 8.2 DSN
`COCKROACH_DSN` env var (D7), no fallback. Reuse the DSN builder (`postgres.go:33` appends `simple_protocol`). Local: `scripts/docker-compose.cockroach.yml` credentials, `sslmode=disable`. Production (Fly.io + CockroachDB Cloud): `postgresql://user:pass@host:26257/bchat?sslmode=verify-full` per `bugs/057/prod_checklist.md`. Fail-fast when `--driver=cockroach` and DSN empty.

### 8.3 Pool settings (plan3_review #9)
- **Decision:** preserve the existing defaults initially — `MaxOpenConns(10)`, `MaxIdleConns(5)`, `ConnMaxLifetime(5m)`, `ConnMaxIdleTime(1m)`, 60s ping (REPOSITORY FACT: `postgres.go:42-48`).
- **Rationale reframed (per review):** these are **operational tuning parameters, not architecture**. DOCUMENTATION FACT: active connections across all pools should not greatly exceed ~4× cluster vCPUs. The correct validation point is the production cluster sizing exercise at deploy time (Phase 7), where `MaxOpenConns` is adjusted via configuration if the Cloud cluster is small. No code-level sizing logic (e.g., `runtime.NumCPU()`) is introduced.

### 8.4 Vector database
REPOSITORY FACT: `vectordb_cockroach.go` already implements the native `VECTOR(1536)` provider (constructor line 27; runtime `CREATE TABLE` + `CREATE VECTOR INDEX` lines 82-113; `crdb.ExecuteTx` upserts lines 191-218; `<=>` search lines 323-327). No changes. Review Issue 5 justification (retained): the schema exists only when the vector capability is compiled in (build tag `cockroach`; env gate `VECTOR_DB_PROVIDER=cockroach` per Taskfile `crdb:check`), so runtime creation is the correct boundary.

---

## 9. Transaction retry design (review Issue 6, extended per plan3_review #3)

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

### 9.2 Side-effect audit (review Issue 6) — retained from v3
All 8 transaction bodies are pure DB operations; external effects (websocket delivery, outbox processing, HTTP responses) occur after `Commit()` in callers. `uuid.NewString()` regenerates harmlessly on retry. A code-review checklist item enforces "no I/O inside tx closures" (§18).

### 9.3 Determinism classification under retry (plan3_review #3, new)

Mechanical fact: a 40001 aborts and rolls back the failed attempt's writes, so **a retried closure never double-writes**. The only variability across attempts is that *reads may observe newer committed state*. Classification per site:

| Site | Classification | Why it is safe |
|---|---|---|
| 1 `createBridgeHandoffAttempt` (112) | **optimistic by design** | `MAX(generation)+1` recomputed on retry; monotonic guard; conflict errors preserved. Generation is only meaningful within the tx |
| 2 `CreateBridgeHandoffReplyIfActive` (356) | **may observe newer state; guard-safe** | re-reads `FOR UPDATE` row; if handoff deactivated meanwhile, returns `ErrBridgeHandoffConflict` — correct outcome |
| 3 `CreateBridgeHandoffReplyAndOutboxIfActive` (500) | **may observe newer state; guard-safe** | same pattern + outbox insert is append-only |
| 4 `ClaimPendingBridgeReplyOutbox` (726) | **may observe newer state; guard-safe** | claim UPDATEs guarded by `status`/`expiry` conditions; rows claimed by others are excluded on re-read |
| 5 `CompleteClaimedBridgeReplyOutbox` (818) | **deterministic under retry** | UPDATE guarded by `claim_token`; idempotent completion branch handles the already-completed case |
| 6 `FailClaimedBridgeReplyOutbox` (912) | **deterministic under retry** | same token-guarded pattern with idempotent fail branch |
| 7 `ClaimBridgeReplyOutboxByOutboxID` (1003) | **may observe newer state; guard-safe** | claim UPDATE guarded by status conditions |
| 8 `CreateAgentMessages` (772) | **deterministic under retry** | append-only INSERTs; failed attempt's inserts were rolled back |

Conclusion: no site produces different *results* across retries in a way that changes observable behavior beyond the intended "claim once" semantics; sites 1–4 may legitimately observe newer state, which is by design. This classification is recorded in a table comment at each wrapper site during implementation.

### 9.4 Wrapper mechanics (cockroach-only)
- Convert the 8 sites to `func(tx *sql.Tx) error` closures run via `crdb.ExecuteTx(ctx, d.db, nil, fn)` (API: module cache `crdb/tx.go:299`, v2.4.3; precedent `vectordb_cockroach.go:191`).
- `crdb.ExecuteTx` detects SQLSTATE 40001/40003/08xxx and re-runs with backoff (DOCUMENTATION FACT: client retry handling; MCP §2.2 row 6).
- Non-retryable errors propagate; `store.ErrBridge*` sentinels untouched.
- `isPostgresRetryable` loop at `bridge.go:95-104` stays for Postgres; the Cockroach path does not reach it.
- `RunResiliently` (`resilience.go`, zero callers — REPOSITORY FACT) stays unused; deferred (§16).

---

## 10. Capability drift policy (plan3_review #4 and #5)

Goal: keep Postgres ↔ Cockroach divergence continuous and documented, not a one-time audit.

### 10.1 Rules
1. **Every new SQL feature** (in migrations or runtime queries) must update the capability matrix (§3) in the same change.
2. **Every new migration file** must pass the portability audit (§6) and the drift check below before merge.
3. **CI rejects undocumented divergence**: a file cannot add a construct from the FORBIDDEN list at all; from the REVIEW-REQUIRED list without an explicit `--verified` annotation in the capability matrix.
4. **Never edit an applied migration file** (§7.5 lifecycle rule).

### 10.2 Drift-check construct list (MCP-verified, §2.2)

**FORBIDDEN in Cockroach migration files (CI fails):**
- `CREATE EXTENSION` (not fully supported)
- `LISTEN` / `NOTIFY` / `UNLISTEN` (not implemented)
- advisory lock functions (`pg_advisory_lock*`) (unsupported; no-op stubs)
- `CREATE DOMAIN` (unsupported)
- range types, `MACADDR`, `MONEY` (unsupported)
- triggers (`CREATE TRIGGER`) (unsupported)
- `DEFERRABLE` / `INITIALLY DEFERRED` constraints (unsupported)
- `DROP PRIMARY KEY` / dropping the primary key constraint (unsupported; use drop+add in one transaction)
- PL/pgSQL blocks (`DO $$`, functions with body syntax) (partial/unimplemented)
- Procedural blocks of any kind — migrations must be plain SQL

**REVIEW-REQUIRED (CI flags for human sign-off, then annotate the matrix):**
- `ALTER TYPE` (supported v23.1+; `DROP VALUE` spawns non-cancellable jobs)
- `UPDATE … FROM` (supported; join-output ambiguity)
- `COPY` (supported generally, but `IMPORT INTO` excluded on vector-indexed tables)
- `SELECT *` in any statement that could become a prepared statement (0A000 risk)
- Non-idempotent data transforms (random values, unguarded UPDATE)

### 10.3 CI implementation (Phase 6)
New `scripts/validate-cockroach-compat.sh`: grep-based scanner over `store/migration/cockroach/**/*.sql` matching the lists above; exit 1 on FORBIDDEN, exit 2 on unannotated REVIEW-REQUIRED. Wired into `Taskfile.yml` (`crdb:check` extension) and the validate-parity flow. Shell-level parsing is explicitly best-effort (mirroring the existing `validate-parity.sh` limitation, line 235); regexes are documented in the script.

---

## 11. Rollback strategy (plan3_review #6, new)

**Property: Bug 057 is additive-only.** No behavior change to the Postgres or SQLite drivers; no postgres migration file changes; `postgres.go` DSN/pool behavior unchanged (REPOSITORY FACT: v3/v4 changes touch only new files plus the cockroach branches in `db.go`, `migrator.go`, and the two tx-site files).

**Rollback scenarios:**
1. **Pre-deploy (build time):** reverting the change = reverting the commit. Nothing else required — the feature is compiled in via the `cockroach` build tag and driver flag; other builds are unaffected.
2. **Post-deploy, runtime bug in Cockroach path:** switch `--driver` back to `postgres` (or `sqlite`) on the same binary and restart. No migration changes are needed because Postgres migrations were never touched. Cockroach data is simply unused.
3. **Cockroach data disposal:** at first deployment there is no production data (INFERENCE: initial rollout is greenfield on CockroachDB Cloud). If the Cockroach rollout is abandoned, the cluster/database is dropped; no export or backfill obligations exist.
4. **Future incremental migrations:** because applied files are never edited (§7.5), rollback of a released schema version means deploying the previous code version; the database stays at its last successful history version (REPOSITORY FACT: history upsert is the commit marker). No downgrade migrations are written — bchat does not ship downgrades today, and Cockroach's online schema changes make `ALTER … DROP` rollbacks a per-incident decision, not an automated path (INFERENCE; noted in §15).

---

## 12. Experiments — VERIFY-FIRST gates with exit criteria (plan3_review #7)

All run on single-node Docker (`scripts/docker-compose.cockroach.yml`) before implementation. Pass criteria are measurable.

| ID | Question | Method | Pass criteria (all must hold) |
|---|---|---|---|
| P1 | Does injected `SET serial_normalization='sql_sequence'` produce `nextval(...)` defaults through the whole-file path? | Run Cockroach `LATEST.sql` (SET-prefixed) via the migrator; `SHOW CREATE TABLE` on ≥3 tables incl. `agent_tenants` and `migration_history` | 100% of SERIAL columns show `nextval('…_seq')` defaults; **0** `unique_rowid()` occurrences; migration_history row written |
| P2 | Does whole-file exec migrate cleanly, and is failure+re-run idempotent? | Full `LATEST.sql` run; delete history row; re-run; then inject a failing statement mid-file and re-run | Re-run after history deletion is a no-op (no errors, no dupes — esp. `tenant_role_templates` stays at 5 rows); failed run writes no history row; corrected re-run completes |
| P3 | Is `EXTRACT(EPOCH FROM NOW())::BIGINT` valid and correct as a default? | `CREATE TABLE t (c BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT); INSERT INTO t DEFAULT VALUES; SELECT c;` | Insert succeeds; `c` = unix epoch seconds ±1; uncast variant fails with a type error (documents why the cast is needed) |
| P4 | Does `crdb.ExecuteTx` retry correctly under contention? | 2–4 concurrent claimers over 1000 outbox rows (reuse claim/complete shapes of §9.1 sites 4–7); force conflicts | **1000 claims, 0 duplicated rows, 0 lost rows**; ≥1 SQLSTATE 40001 observed in logs; every retry eventually succeeds; completion count = 1000 |
| P5 | SKIP LOCKED claim correctness under concurrency | Multiple workers on `ClaimPendingEvents` query (`agent.go:2781`) over 1000 events | Each event claimed exactly once (0 double-claims, 0 lost); workers observe different batches |
| P6 | Vector path regression | Existing `crdb:test` suite (Taskfile `crdb:test`, `-tags cockroach`) with `VECTOR_DB_PROVIDER=cockroach` | All tests green; vector insert/search round-trip latency within 2× of pre-change baseline (informational, not a gate) |

---

## 13. Benchmarks — regression detection only (plan3_review #8, new)

Not optimization; the goal is a documented baseline so future changes can detect regressions. A `crdb:bench` Taskfile target runs the suite on the local Docker cluster and records results in `build/bench/`:

| Benchmark | Metric | Method | Baseline note |
|---|---|---|---|
| Migration runtime | wall time for fresh `LATEST.sql` | `time ./build/memos --driver=cockroach --mode dev` on empty DB | compare vs Postgres on the same host (informational; different engines) |
| CreateAgentMessages TPS | inserts/sec, p99 latency | Go benchmark wrapping `CreateAgentMessages` (site 8) with N=1000 | Postgres vs Cockroach on same host |
| Bridge outbox throughput | claim→complete cycles/sec | benchmark wrapping §9.1 sites 4–7 flow | Postgres vs Cockroach |
| Vector search latency | p99 of `<=>` top-K query | reuse existing vector tests with 1k vectors | Cockroach-only |

Benchmarks are advisory: no perf gate in CI, and no changes to the `simple_protocol` decision result from them without a follow-up decision (deferred per §16).

---

## 14. Adversarial pass — every behavioral change defended (retained from v3, updated)

### 14.1 Migrations: explicit-transaction → autocommit whole-file
1. **Changed:** a migration file is no longer atomic.
2. **Required:** online schema changes do not support multi-statement DDL transactions (MCP row 4).
3. **New failure modes:** partial application on failure → mitigated by idempotent `LATEST.sql` (verified), `execute()` tolerance, history-after-success, and §7.5 recovery procedure.

### 14.2 Migrations: injected `SET serial_normalization`
1. **Changed:** none on Postgres (cockroach-only branch).
2. **Required:** int32 overflow with `unique_rowid()` (D2 evidence chain).
3. **New failure modes:** SET silently not applying → gated by P1; performance: one `nextval` round-trip per insert (negligible at bchat volumes — INFERENCE); `sql_sequence_cached` is the documented alternative if ever needed.

### 14.3 DDL: explicit `::BIGINT` casts
1. **Changed:** Cockroach files differ textually (19 defaults); Postgres uses numeric→bigint assignment cast (rounds), Cockroach uses explicit float→int8 cast (truncates). ≤1s deviation possible at .5s fractional boundaries.
2. **Required:** `extract → float` with no implicit int assignment (MCP row 3); uncast DDL fails at CREATE TABLE.
3. **New failure modes:** none beyond DDL validation, gated by P3; runtime `migration_history.go:39` needs no change (uses the default).

### 14.4 Runtime: retry closures on 8 tx sites
1. **Changed:** 40001 re-executes the closure instead of propagating.
2. **Required:** client-side retry handling is documented as required for 40001 (MCP row 6).
3. **New failure modes:** duplicate side effects → none found (§9.2); behavioral variability on retry → classified per site (§9.3), all safe.

### 14.5 Driver selection/config
1. **Changed:** new driver name + DSN source; no behavior change to other drivers.
2. **Required:** distinct DSN prevents cross-wiring (D7).
3. **New failure modes:** misconfiguration → fail-fast startup error.

### 14.6 Protocol: simple everywhere
1. **Changed:** none vs. today's Postgres driver.
2. **Required:** no — conservative parity choice; split deferred (§16).
3. **New failure modes:** none.

### 14.7 Runtime-created vector schema
1. **Changed:** none — identical to today.
2. **Required:** no — kept with justification (§8.4).
3. **New failure modes:** none.

### 14.8 NEW — migration file lifecycle rule
1. **Changed:** a governance rule (no code change): applied files are immutable.
2. **Required:** Cockroach's partial application makes in-place edits dangerous; Postgres atomicity makes them retroactive.
3. **New failure modes:** forced "compensation file" pattern adds one extra file per fix — accepted cost; prevents silent divergence.

---

## 15. Files to create / modify

### Create
| File | Purpose |
|---|---|
| `store/migration/cockroach/LATEST.sql` | Full schema; mirror of postgres with `::BIGINT` on 19 EXTRACT defaults (§6.2) |
| `store/migration/cockroach/0.19/00__*.sql` … `0.35/*.sql` | File-for-file mirrors (parity; inert on fresh deploy — §7.4) |
| `store/db/postgres/cockroach.go` | `NewCockroachDB` constructor |
| `scripts/validate-cockroach-compat.sh` | Drift-check scanner (§10.3) |

### Modify
| File | Change |
|---|---|
| `store/db/db.go` | `case "cockroach"` in the driver factory |
| `store/migrator.go` | Cockroach branch: no `Begin()`; inject `SET serial_normalization` prefix (§7.2) |
| `store/db/postgres/bridge.go` | `crdb.ExecuteTx` wrappers for sites 112, 356, 500, 726, 818, 912, 1003 (cockroach-only path) |
| `store/db/postgres/agent.go` | `crdb.ExecuteTx` wrapper for site 772 (cockroach-only) |
| `internal/profile/profile.go` | accept/validate `Driver == "cockroach"`; expose `COCKROACH_DSN` |
| `bin/memos/main.go` | bind `COCKROACH_DSN`; fail-fast when driver=cockroach and DSN empty |
| `scripts/validate-parity.sh` | extend file-list parity to postgres↔cockroach; document sqlite 0.2–0.18 legacy dirs as known divergence |
| `Taskfile.yml` | `run:cockroach`/`crdb:*`: pass `--driver=cockroach` + `COCKROACH_DSN`; add `crdb:bench`, migration-drift, and compat-check targets |
| `store/migration_helper.go` | **no change** — SQLite-only helpers must not be ported (REPOSITORY FACT) |

### Unchanged (explicitly)
All 23 `store/db/postgres/*.go` implementation files except the two tx-site files; `store/driver.go`; `store/agent.go`; migrator version logic; `vectordb_cockroach.go`.

---

## 16. Config & environment

| Item | Value |
|---|---|
| Driver flag | `--driver=cockroach` |
| DSN env | `COCKROACH_DSN` (no fallback) |
| Protocol | `simple_protocol` (appended by DSN builder) |
| Local dev | `scripts/docker-compose.cockroach.yml` (exists); single-node bootstrap via `COCKROACH_DATABASE`/`USER`/`PASSWORD` (MCP row 19) |
| Production | Fly.io + CockroachDB Cloud; `sslmode=verify-full`; TLS per `prod_checklist.md` |
| Build tag | `cockroach` (existing Taskfile pattern) |

---

## 17. Out of scope / future work (recorded, not implemented)

- Split protocols (review Issue 4): migrator simple / runtime extended — gated on a benchmark showing material runtime gain (§13).
- `NewSQLDriver(profile)` capability factory (review Issue 7) — naming refactor, defer until a third driver appears.
- Wiring `RunResiliently` (connection-level retry) — zero-caller code today; revisit if Cockroach connection flaps appear in production.
- Capability-interface extraction (§4) — only if a future feature cannot fit the four seams.
- TLS/cert rotation, backups, locality, cluster sizing validation — operational follow-up per `prod_checklist.md`.
- `BIGSERIAL`/int64 ID migration — future schema evolution; int32 space is 2.1B per table (adequate at bchat scale — INFERENCE).
- pgvector provider parity — not used; native `VECTOR` is the only Cockroach vector path.

---

## 18. CI & validation

| Check | Today | After |
|---|---|---|
| File-list parity | `validate-parity.sh` sqlite↔postgres (exit 1) | + postgres↔cockroach |
| Schema parity (best-effort) | sqlite↔postgres (exit 2, warn) | + cockroach; expect identical names modulo `::BIGINT` text |
| Migration drift | `validate-pg-migrations.sh`, `validate-migrations.sh` (LATEST vs cumulative) | parameterized for cockroach |
| **Compat drift (new)** | — | `validate-cockroach-compat.sh` (§10.3): FORBIDDEN → exit 1, REVIEW-REQUIRED → exit 2 |
| Runtime tests | `crdb:test` (`-tags cockroach ./server/router/api/v1/agent/...`) | + store-layer tests: migrate-on-Docker, CRUD smoke, P4/P5 concurrency tests |
| Benchmarks (advisory) | — | `crdb:bench` (§13) |
| Build | `build:cockroach` (tag `cockroach`) | unchanged |

---

## 19. Risks & known limitations

| Risk | Detail | Mitigation |
|---|---|---|
| SKIP LOCKED upstream bug #167582 | Rare: SKIP LOCKED may skip rows whose lock is a stale intent from a committed transaction (MCP row 1) | claims re-scanned each poll cycle; skipped rows picked up next pass (INFERENCE); no code change |
| 0A000 "cached plan must not change result type" | Prepared statements can break across online schema changes (MCP row 10) | simple protocol (no server-side prepared statements in this path); avoid `SELECT *` (drift list) |
| Migration atomicity | files apply statement-by-statement | §7.3 mitigations + §7.5 recovery procedure + P2 gate |
| int32 ID space | bounded at 2.1B/table | `sql_sequence` keeps values in range; INT8 OID behavior irrelevant to pgx stdlib text scanning (MCP row 18); BIGSERIAL upgrade deferred (§17) |
| EXTRACT float→int truncation | ≤1s deviation at .5s boundaries vs Postgres rounding | explicit cast; no sub-second precision dependency (REPOSITORY FACT: all consumers read Unix seconds) |
| Vector index limitations | no `IMPORT INTO`; batch inserts degrade (MCP row 9) | provider avoids both (single-row UPSERTs) |
| Pool sizing | 10 open × 2 pools vs ~4×vCPU guidance | operational tuning at deploy (§8.3) |
| Non-cancellable `ALTER TYPE DROP VALUE` | long-running schema change if used | review-required in drift check; not used today |
| simple_protocol performance | text results | accepted; benchmark gate for split (§13, §17) |

---

## 20. Implementation task list (ordered)

**Phase 1 — Experiments (gate):** P1–P3 on single-node Docker; record results in this file's successor (implementation log).
**Phase 2 — Migration files:** mirror `store/migration/postgres/` → `store/migration/cockroach/`; apply §6.1 audit + `::BIGINT`; run parity + compat scripts.
**Phase 3 — Migrator branch:** no-`Begin` + SET injection; idempotency rule applied to all Cockroach files.
**Phase 4 — Driver:** `cockroach.go`, factory case, profile/main wiring, fail-fast DSN validation.
**Phase 5 — Retries:** convert 8 tx sites to `crdb.ExecuteTx` closures (cockroach-only); add §9.3 classification comments; re-verify with P4.
**Phase 6 — CI/tooling:** `validate-cockroach-compat.sh`, parity extension, drift-script parameterization, Taskfile targets (`crdb:bench`, compat checks).
**Phase 7 — E2E + operational validation:** P4/P5/P6, fresh Cloud deploy, pool sizing check per §8.3, benchmark baselines (§13).

---

## 21. Acceptance criteria

1. `--driver=cockroach` + `COCKROACH_DSN` boots locally and on Cloud; fail-fast without DSN.
2. Fresh Cockroach DB migrates via `LATEST.sql`; `SHOW CREATE TABLE` proves `nextval(...)` defaults (P1).
3. Migration re-run and mid-file failure behave per §7.3/§7.5 (P2); all Cockroach migration files idempotent; applied files immutable (rule documented).
4. Postgres driver behavior unchanged — existing test suite fully green.
5. All 8 tx sites use `crdb.ExecuteTx` on the Cockroach path; no I/O inside closures (§9.2); determinism classification comments present (§9.3).
6. P4: 1000 concurrent claims, 0 duplicates, 0 lost, ≥1 40001 observed; P5: exactly-once event claims.
7. Parity validator + compat drift check pass with cockroach included.
8. Vector path green (`crdb:test` with `VECTOR_DB_PROVIDER=cockroach`); P6 criteria met.
9. Capability matrix (§3) updated for any SQL construct added during implementation; drift policy (§10) enforced in CI.
10. Every claim in the final PR description cites repository evidence or MCP-verified docs per §0; benchmark baselines recorded (§13).
