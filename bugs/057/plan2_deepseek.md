# Plan: Port Postgres Database Migrations to CockroachDB (v2 — Revised)

**Bug ID:** 057
**Date:** 2026-08-02
**Status:** REVISED — pending implementation
**Source docs:** `plan.md` (v1), `plan2_review.md` (Kilo review), `plan2_review_chatgpt.md` (ChatGPT review)
**Goal:** Seamless developer experience for local development, testing, and deployment with CockroachDB, without creating a permanent maintenance fork.

---

## 0. Adjudication of Prior Reviews

This section records, for every prior finding, whether it is accepted, amended, rejected, or superseded, and the evidence behind that decision. Evidence is either official CockroachDB documentation (files under `bugs/057/` or cited official pages) or direct verification against this repository.

### 0.1 Findings accepted from `plan2_review.md`

| Finding | Verdict | Evidence |
|---------|---------|----------|
| **C-1 Code duplication without abstraction** — BLOCKER | **Accepted, superseded by Section 1.1** (single shared implementation, per ChatGPT's direction + codebase verification) | `store/db/postgres/` contains 25 files; 23 are implementation files. Verified all 23 are portable (see 0.3). A copy-fork guarantees drift. |
| **C-2 Missing files in copy list** — BLOCKER | **Accepted.** The plan's 22-file table omits `bridge_auth.go` and `memo_relation.go`, which exist in `store/db/postgres/` and implement required `store.Driver` methods (`CreateBridgeAuthKey`…, `UpsertMemoRelation`…). | `store/driver.go:40-42, 279-286`; `ls store/db/postgres/` |
| **C-3 No incremental migration strategy** — BLOCKER | **Accepted and strengthened** — the migrator *requires* versioned directories (see new C-new-2). | `store/migrator.go:257-281` |
| **C-4 Explicit Cockroach DSN** — BLOCKER | **Accepted.** No `DATABASE_URL` fallback for the `cockroach` driver. | `internal/profile/profile.go:97-102` shows the footgun pattern exists for postgres. |
| **C-5 SQL audit before copying** — BLOCKER | **Accepted**, and executed in this revision (Section 0.3). The audit is now part of the plan, not a precondition. | See 0.3. |
| **H-1 Pool settings** — HIGH | **Accepted with amendment from ChatGPT**: do not derive from `runtime.NumCPU()` of the app host; the 4× rule refers to **cluster** vCPUs. Use env var with documented default. | `prod_checklist.md:351` ("4 times the number of vCPUs **in the cluster**") |
| **H-2 `sslmode=require` insufficient** — HIGH | **Accepted.** Use `sslmode=verify-full`; CRDB Cloud general connection strings verify against system CA certs (no custom `sslrootcert` needed). | `pgx.md:83,162` (system CA + `verify-full` example); `docker.md:419` (secure mode uses `verify-full`) |
| **H-3 `agent_vectors` race** — HIGH | **Downgraded to MEDIUM per ChatGPT and codebase evidence.** `CREATE TABLE IF NOT EXISTS` + `CREATE INDEX IF NOT EXISTS` are idempotent; the vector index (no `IF NOT EXISTS` support) already handles `42P07`. Runtime creation is *intentional* (build-tag-gated feature, DB-specific `VECTOR` type) — keep it, document it. | `vectordb_cockroach.go:81-140`; plan.md §2.2 note |
| **H-4 Retry semantics** — HIGH | **Accepted** (see Section 4). Verified 8 manual `BeginTx` sites. | `agent.go:772`, `bridge.go:112,356,500,726,818,912,1003` |
| **H-5 Async schema changes** — HIGH | **Amended per ChatGPT: downgrade to MEDIUM — but for the correct reason.** The risk is not asynchrony (v25.x executes most DDL synchronously within the transaction's epoch) but that the migrator runs **all of LATEST.sql inside a single explicit transaction**, which official docs flag for CRDB. | Official v25.x "Known Limitations — Schema changes within transactions": "Most schema changes should not be performed within an explicit transaction with multiple statements… Schema change DDL statements inside a multi-statement transaction can fail while other statements succeed." |
| **H-6 No rollback plan** — HIGH | **Accepted** (Section 7). | — |
| **M-1 `EXTRACT(EPOCH FROM NOW())`** — MEDIUM | **Accepted as verify-first** (ChatGPT amendment). CRDB returns DECIMAL for epoch extraction; verify DEFAULT coercion on a live cluster; cast explicitly only if needed. | Official docs: `extract(epoch from timestamp)` → DECIMAL |
| **M-2 `::BIGINT` casts** — MEDIUM | **Accepted as verify-first.** 5 sites in `store/db/postgres/agent.go` (2605, 2693, 2718, 2776, 2780). `BIGINT` is a documented alias of `INT8`; the explicit cast `EXTRACT(EPOCH FROM NOW())::BIGINT` is expected to work on v25.x — verify in `crdb:verify` (Section 9.2). | Official `INT` docs: BIGINT is an alias for INT8 |
| **M-3 RAG reindex settings** — MEDIUM | **Accepted.** First deployment must index; `FORCE_REINDEX_ON_STARTUP=true` for deploy #1. | `fly_crdb.toml` §6.2 |
| **M-4 Placeholder secrets script** — MEDIUM | **Accepted** (Section 6.3). | — |
| **M-5 `default_query_exec_mode` removal** — MEDIUM | **Upgraded to HIGH** (see C-new-3) — the removal risks breaking migration execution itself. | — |
| **M-6 `unique_rowid()` hotspots** — MEDIUM | **Superseded by C-new-1**: `unique_rowid()` is rejected for a harder reason than hotspots — it breaks `int32` scanning. | See C-new-1 |
| **L-1/L-2/L-3/L-4/L-5** (in-memory store, zone config scope, Docker volume, LLM placeholders, build tags) | **Accepted as documented in Sections 3/6.** Note: `docker-compose.cockroach.yml` already uses a named volume (`bchat_crdb_data`) per `docker.md` recommendation. | `docker.md:53-63` |

### 0.2 Findings from the ChatGPT review (`plan2_review_chatgpt.md`)

| ChatGPT finding | Verdict | Evidence |
|-----------------|---------|----------|
| C-1/C-3/C-4 blockers | **Accepted** (see above). | — |
| Downgrade H-5 (async DDL) | **Accepted in severity, reason corrected** — the documented hazard is *multi-statement transactions containing schema changes*, which is exactly what the migrator does today. | v25.x Known Limitations (see 0.1/H-5) |
| Keep `agent_vectors` runtime-created | **Accepted** — document, don't move to migrations. | 0.1/H-3 |
| Reject `runtime.NumCPU()*4` | **Accepted** — env var + documented default; the 4× rule is about cluster vCPUs. | `prod_checklist.md:351` |
| M-1 verify-first | **Accepted.** | 0.1/M-1 |
| **New BLOCKER: cross-database CI** | **Accepted** (Section 8). Repo already has `scripts/validate-parity.sh`, `scripts/validate-migrations.sh`, `scripts/validate-pg-migrations.sh`, `task crdb:test` — extend these rather than greenfield. | `scripts/` |
| Search-path assumptions | **Accepted as verify item** — default `search_path = public` in both; all queries unqualified. Low risk. | 0.3.9 |
| Isolation assumptions (SERIALIZABLE vs READ COMMITTED) | **Accepted** — CRDB default is SERIALIZABLE; more 40001s; retry wrapper is mandatory, not optional. | `prod_checklist.md:649` ("your application should be engineered to handle transaction retries using client-side retry handling") |
| RETURNING usage audit | **Accepted as verify item** — CRDB supports `RETURNING`; audit confirmed single use pattern in `ClaimPendingEvents`. | `agent.go:2790` |
| Prepared statements benchmark | **Accepted, superseded by C-new-3** — pin the protocol decision to the migration requirement instead. | — |
| **"Do you even need two packages?"** | **Accepted with evidence** — see Section 1.1: one shared implementation. | 0.3 audit |

### 0.3 SQL portability audit (executed as part of this revision — replaces C-5)

Every SQL construct in `store/db/postgres/` was audited against CRDB v25.x capability. Result: **the postgres store implementation is portable; no dialect rewrite is required.** The only behavioral deltas are retries, protocol mode, and migration execution — all addressed in Sections 1/4.

| Construct | Where used | CRDB v25.x support | Action |
|-----------|------------|--------------------|--------|
| `$n` placeholders | all files | Full | none |
| `::BIGINT` cast | `agent.go:2605,2693,2718,2776,2780` | Alias of INT8; cast supported | verify in `crdb:verify` |
| `::BOOLEAN` cast | `memo_filter.go:143,224` | Supported | verify |
| JSONB `->`, `->>`, `@>` | `memo.go:106-115`, `memo_filter.go:143,172` | Supported (no PK/FK/unique on JSONB — schema doesn't use them) | verify |
| `jsonb_build_array` | `memo_filter.go:172` | Supported | verify |
| `ILIKE` | `memo.go:70`, `memo_filter.go:209` | Supported | none |
| `FOR UPDATE` | `bridge.go:371,515` | Supported (v22.2+) | none |
| `FOR UPDATE SKIP LOCKED` | `agent.go:2781` | Supported (v22.2+) | none |
| `EXTRACT(EPOCH FROM NOW())` | LATEST.sql defaults | Supported; returns DECIMAL | verify coercion (M-1) |
| `SERIAL` | LATEST.sql (~50 columns) | Supported; **default mode = `INT8 DEFAULT unique_rowid()`** → 64-bit values → **breaks int32 scans** | **replace with sequence mode** (C-new-1) |
| `ON CONFLICT DO NOTHING` | LATEST.sql seed (line 208) | Supported | none |
| `CREATE TABLE/INDEX IF NOT EXISTS` | LATEST.sql, vectordb | Supported | none |
| `RETURNING` | `agent.go:2790` | Supported | none |
| `CREATE EXTENSION` | not used | NOT supported | n/a (none present) |
| Triggers, ENUM, materialized views | not used | n/a | n/a |
| Multi-statement string Exec | `store/migrator.go:130,184` (whole SQL files) | Simple protocol: yes; extended protocol/prepared statement: **no** | C-new-3 (keep simple protocol / split statements) |

### 0.4 New findings from this revision (missed by BOTH prior reviews)

| ID | Severity | Finding | Evidence |
|----|----------|---------|----------|
| **C-new-1** | CRITICAL | **`unique_rowid()` (and CRDB's default SERIAL mode) generate 64-bit IDs that overflow the codebase's `int32` ID fields.** Every `Scan(&struct.ID int32)` fails at runtime. The v1 plan's central "optimization" (`SERIAL → INT DEFAULT unique_rowid()`) is a hard break, and keeping `SERIAL` unchanged is equally broken because CRDB's default `serial_normalization=rowid` maps `SERIAL` to `INT8 DEFAULT unique_rowid()`. | Official `SERIAL` docs: "`rowid` (default): `SERIAL` implies `DEFAULT unique_rowid()`… values are 64-bit INT8 (48-bit timestamp + 15-bit node id)"; structs: `store/agent.go:12,37,77,98,134,152,171,191`; `store/memo.go:38` |
| **C-new-2** | CRITICAL | **The migrator cannot boot with only `LATEST.sql`.** `GetCurrentSchemaVersion()` globs `migration/{driver}/*/*.sql` and returns an error ("no migration files found") when empty → `preMigrate()` fails → the server never starts. The v1 plan creates zero versioned dirs under `store/migration/cockroach/`. | `store/migrator.go:257-281`, `preMigrate` at 160-207 |
| **C-new-3** | HIGH | **Removing `default_query_exec_mode=simple_protocol` endangers every migration.** The migrator Execs entire multi-statement SQL files in one call. pgx's default exec mode uses the extended protocol / prepared statements, which cannot carry multi-statement strings ("cannot insert multiple commands into a prepared statement"). CRDB additionally documents that prepared statements may error after schema changes. Keep `simple_protocol` for the cockroach driver (CRDB fully supports the simple protocol); treat removal as a measured decision, not a default. | pgx v5 docs (QueryExecMode; extended protocol = single statement per Parse); v25.x Known Limitations: "No online schema changes between executions of prepared statements" |
| **C-new-4** | HIGH | **Local bootstrap is broken as written.** `docker-compose.cockroach.yml` does not set `COCKROACH_DATABASE/USER/PASSWORD`, yet the plan's local DSN assumes `bchat_user:bchat_pass@…/bchat`. Those env vars are honored by the official image only with `start-single-node`. | `docker.md:343-352` ("create a database and user automatically… used only if the data directory is empty"); compose file verified |
| **C-new-5** | HIGH | **Migration transactions must never be retried.** CRDB cannot retry transactions containing schema changes; `crdb.ExecuteTx` must be excluded from migration code paths. The v1 plan's blanket "wrap transaction-heavy methods with `crdb.ExecuteTx`" would poison the migrator if applied mechanically. | v25.x Known Limitations (schema changes within transactions); official transaction-retry docs: client-side retry with 40001/"restart transaction" |

---

## 1. Architecture: One Shared SQL Implementation (replaces v1 §1.1-1.4)

### 1.1 Driver strategy — decision

**Question (per ChatGPT): "Why do PostgreSQL and CockroachDB need separate store implementations at all?"**
**Answer: they do not.** The §0.3 audit proves the SQL in `store/db/postgres/` is portable. The deltas are four:

1. Connection string handling (DSN params, pool sizing) — `postgres.go:22-54`
2. Transient-error detection + retry wrapper — `resilience.go`
3. Transaction execution (8 manual `BeginTx` sites must become retry-capable on CRDB) — Section 4
4. Migration execution mode — Section 5

**Design:** keep the single `store/db/postgres` package (it is the "PostgreSQL wire-protocol" implementation). Add a cockroach variant selected at runtime:

```go
// store/db/postgres/postgres.go
func NewDB(profile *profile.Profile) (store.Driver, error) {
    return newDB(profile, false)
}

func NewCockroachDB(profile *profile.Profile) (store.Driver, error) {
    return newDB(profile, true)
}
```

`DB` gains `isCockroach bool`, used only in: connection setup, `isTransientError`, and the `withTx` helper (Section 4). **No build tags** — the driver is selected at runtime via `MEMOS_DRIVER`, like the existing `postgres` case. This resolves C-1 completely: there is one implementation, one bug-fix path, one set of store methods. No `store/db/cockroach/` directory is created.

### 1.2 Driver factory

`store/db/db.go`:

```go
case "cockroach":
    driver, err = postgres.NewCockroachDB(profile)
```

### 1.3 Profile / DSN resolution

`internal/profile/profile.go` (per C-4):

```go
if p.Driver == "cockroach" && p.DSN == "" {
    p.DSN = os.Getenv("COCKROACH_DSN")
    if p.DSN == "" {
        return errors.New("cockroach driver requires COCKROACH_DSN environment variable")
    }
}
```

- **No `DATABASE_URL` fallback** (C-4). A misconfigured `MEMOS_DRIVER=cockroach` with a Neon `DATABASE_URL` set must fail loudly at startup, never silently connect to Postgres.
- Reject `sslmode=disable` in `prod` mode at startup (defense in depth; H-2).

### 1.4 Connection setup (cockroach variant)

```go
dsn := profile.DSN
// Keep simple protocol: the migrator executes multi-statement SQL files (C-new-3),
// and CRDB documents prepared-statement staleness across schema changes.
if !strings.Contains(dsn, "default_query_exec_mode") {
    sep := "?"; if strings.Contains(dsn, "?") { sep = "&" }
    dsn += sep + "default_query_exec_mode=simple_protocol"
}
db.SetMaxOpenConns(poolMaxOpenConns)   // COCKROACH_MAX_OPEN_CONNS, default 8
db.SetMaxIdleConns(poolMaxOpenConns / 2)
db.SetConnMaxLifetime(5 * time.Minute)
db.SetConnMaxIdleTime(1 * time.Minute)
```

- **Pool sizing (H-1 + ChatGPT amendment):** read `COCKROACH_MAX_OPEN_CONNS` (default **8**, i.e., 4× the 2 vCPUs of a CRDB Cloud Basic cluster — `prod_checklist.md:351`). Do **not** derive from `runtime.NumCPU()` of the Fly machine (app host ≠ cluster).
- `sslmode=verify-full` with system CA is the CRDB Cloud default in the docs' connection strings; keep whatever `COCKROACH_DSN` specifies for local (`disable`) and require a non-`disable` mode in `prod`.

### 1.5 Resilience (cockroach variant)

- `isTransientError`: add `"40001"` (serialization_retry) to the existing set (57P01/08006/08003/08001/08004).
- `isUniqueViolation`: unchanged — CRDB uses the same SQLSTATE `23505`.
- `RunResiliently`: unchanged behavior; note that statement-level retry is only applied to idempotent single statements (existing call sites); 40001 handling happens at transaction level via `withTx` (Section 4).
- **Never wrap migration code in retry** (C-new-5).

---

## 2. Schema Migration for CockroachDB

### 2.1 Migration directory

Create `store/migration/cockroach/` containing:

| File | Purpose |
|------|---------|
| `LATEST.sql` | Full schema, adapted per Section 2.2 (copy of postgres LATEST.sql with PK/ID changes only) |
| `0.35/1__init.sql` | Marker file so `GetCurrentSchemaVersion()` finds versioned dirs (C-new-2); a no-op comment statement |

**Rationale (C-new-2):** the migrator requires `migration/{driver}/*/*.sql` glob matches. Fresh CRDB clusters boot via `preMigrate()` → `LATEST.sql` → `migration_history`; incremental files are only applied when a history entry older than the FS max exists. The marker dir must be kept at the current schema version (0.35 at time of writing) and bumped together with the code version.

**Incremental migration policy (C-3):**
1. Every future migration lands in BOTH `store/migration/postgres/` and `store/migration/cockroach/` (identical SQL unless a CRDB-specific construct is documented).
2. CI enforces parity (Section 8): same file lists, same statement counts.
3. CRDB LATEST.sql is regenerated from postgres LATEST.sql by a documented script (`scripts/gen-crdb-latest.sh`) that applies ONLY the documented diffs in §2.2 — no manual divergence.
4. All migration SQL must avoid CRDB-unsupported constructs: `CREATE EXTENSION`, `CREATE INDEX CONCURRENTLY`, triggers, `ALTER COLUMN TYPE` (documented as restricted/limited on CRDB). CI greps for these (Section 8).

### 2.2 LATEST.sql adaptations (postgres → cockroach)

**PK strategy — sequences, not `unique_rowid()` (C-new-1).**

- CRDB default `SERIAL` mode (`rowid`) produces 64-bit `unique_rowid()` values that overflow the codebase's `int32` ID structs. Therefore the cockroach LATEST.sql must force sequence-backed generation:
  - Add at the top of `LATEST.sql`:
    ```sql
    SET serial_normalization = sql_sequence;
    ```
    (per official SERIAL docs: `sql_sequence` mode creates a regular SQL sequence and `SERIAL` → `INT DEFAULT nextval(<seq>)`; values start small and remain int32-compatible.)
  - Or, if session-variable reliance is undesirable, spell it out per table:
    ```sql
    CREATE SEQUENCE "user_id_seq";
    CREATE TABLE "user" (
      id INT DEFAULT nextval('"user_id_seq"') PRIMARY KEY,
      ...
    );
    ```
  - **Decision:** prefer the `SET serial_normalization = sql_sequence` header (single statement, all ~50 SERIAL columns inherit it). Verify with `SHOW CREATE TABLE "user"` in `crdb:verify` that the default became `nextval(...)`, not `unique_rowid()`.
- **Documented trade-off (M-6, superseded by C-new-1):** sequence-backed IDs serialize nextval calls cluster-wide — a write bottleneck at very high throughput, and the opposite of CRDB's documented preference (`unique_rowid`/UUID scatter writes). Acceptable here because (a) int32 API/struct compatibility is mandatory, (b) this application's write volume is low. Follow-up item: widen ID columns to `INT8` + `unique_rowid()` when store structs move to `int64`.

Other adaptations:

| Postgres | Cockroach | Reason |
|----------|-----------|--------|
| `SERIAL PRIMARY KEY` | unchanged, with `SET serial_normalization = sql_sequence` header | int32-safe values (C-new-1) |
| `BIGINT` columns/defaults | unchanged (`INT`/`INT8` are the same type on CRDB) | alias identity |
| `EXTRACT(EPOCH FROM NOW())` defaults | unchanged; verify DECIMAL→INT assignment coercion in `crdb:verify`; cast `::BIGINT` only if coercion fails (M-1) | CRDB returns DECIMAL |
| `TEXT` columns | unchanged | `TEXT` is an alias of `STRING` |
| `JSONB DEFAULT '{}'` | unchanged | supported |
| `ON CONFLICT DO NOTHING` seed | unchanged | supported |
| `CREATE TABLE "user"` | unchanged | quoted identifier supported |

**Do NOT put `agent_vectors` into LATEST.sql** (H-3/ChatGPT): the table and its `VECTOR(1536)` index are CockroachDB-specific and build-tag-gated; migrations are shared with other drivers. Keep runtime creation in `CockroachVectorDB.Validate()` and document it as intentional.

### 2.3 Migration execution mode (C-new-3, C-new-5, H-5)

The shared migrator (`store/migrator.go`) executes each migration file as **one multi-statement Exec inside one explicit transaction** (`tx.Begin()` → `execute()` → `tx.Commit()`). On CRDB this is the documented anti-pattern. Adaptation, confined to `s.profile.Driver == "cockroach"`:

1. **Split migration files into individual statements** (respecting string literals/comments; add a small splitter in `store/migrator.go` or `store/migration_helper.go`).
2. **Execute each statement as its own implicit transaction** (no wrapping `sql.Tx` for the cockroach driver). This:
   - avoids "DDL inside multi-statement transaction can fail while other statements succeed" (v25.x Known Limitations),
   - avoids the extended-protocol multi-statement limitation (each statement is small enough for any protocol),
   - keeps the existing `execute()` tolerance for "duplicate column/already exists" per statement.
3. **Never wrap migration execution in `crdb.ExecuteTx`** (C-new-5) — schema-change transactions cannot be retried; fail cleanly with the original error and let the operator fix and restart.
4. Keep the existing single-transaction behavior for `sqlite`/`postgres`/`mysql` unchanged.

Note: `migration_history` insert + `updateCurrentSchemaVersion` (workspace setting) after a successful migration remain as-is.

---

## 3. Local Development & Testing

### 3.1 Compose bootstrap fix (C-new-4)

Update `scripts/docker-compose.cockroach.yml` to create the database and user automatically, per official `docker.md`:

```yaml
services:
  cockroach:
    image: cockroachdb/cockroach:v25.2.21
    container_name: bchat-crdb
    restart: unless-stopped
    command: start-single-node --insecure --advertise-addr=localhost
    environment:
      - COCKROACH_DATABASE=bchat
      - COCKROACH_USER=bchat_user
      - COCKROACH_PASSWORD=bchat_pass
    ports:
      - "26257:26257"
      - "8080:8080"
    volumes:
      - bchat_crdb_data:/cockroach/cockroach-data
```

- The env vars are honored because the container runs `start-single-node` and only when the data dir is empty (`docker.md:343-352`); document that `down -v` is required to re-bootstrap.
- The named volume `bchat_crdb_data` already matches `docker.md`'s "store cluster data in Docker volumes" recommendation.
- Verify in `crdb:verify` that password auth over the wire works in insecure mode with these env vars; if not, fall back to `root` with `sslmode=disable` for local only (never prod).

### 3.2 Local workflow

```bash
docker compose -f scripts/docker-compose.cockroach.yml up -d
task build:backend:cockroach          # existing task (keep)
COCKROACH_DSN="postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable" \
MEMOS_DRIVER=cockroach MEMOS_MODE=dev \
RAG_PIPELINE_ENABLED=true VECTOR_DB_PROVIDER=cockroach \
./build/memos --mode dev --data build/data
```

### 3.3 Test cluster settings (from `test_locally.md`)

Apply only for local/CI (documented as NOT for production or benchmarking):

```sql
SET CLUSTER SETTING kv.range_merge.queue_interval = '50ms';
SET CLUSTER SETTING jobs.registry.interval.gc = '30s';
SET CLUSTER SETTING jobs.registry.interval.cancel = '180s';
SET CLUSTER SETTING jobs.retention_time = '15s';
SET CLUSTER SETTING sql.stats.automatic_collection.enabled = false;
ALTER RANGE default CONFIGURE ZONE USING "gc.ttlseconds" = 600;
ALTER DATABASE system CONFIGURE ZONE USING "gc.ttlseconds" = 600;
```

Wrap in a `task crdb:tune:test` and never include in production scripts.

### 3.4 Local verification

```bash
docker exec -it bchat-crdb cockroach sql --insecure --host=localhost -d bchat -e "SHOW TABLES;"
docker exec -it bchat-crdb cockroach sql --insecure --host=localhost -d bchat -e "SELECT * FROM migration_history;"
docker exec -it bchat-crdb cockroach sql --insecure --host=localhost -d bchat -e "SHOW CREATE TABLE \"user\";"   # expect DEFAULT nextval(...), NOT unique_rowid()
```

---

## 4. Transaction Retry Strategy (H-4, C-new-5, isolation)

CRDB default isolation is SERIALIZABLE and signals retry with SQLSTATE **40001** + message starting "restart transaction" (`prod_checklist.md:649`; official transaction-retry docs). Client-side retry is mandatory.

### 4.1 Inventory (verified)

| Site | File:line | Pattern | CRDB action |
|------|-----------|---------|-------------|
| CreateAgentMessages | `agent.go:772` | `BeginTx` + insert loop + Commit | wrap in `withTx` |
| CreateBridgeHandoffReplyIfActive | `bridge.go:356` | `conn.BeginTx` + `SELECT…FOR UPDATE` + INSERT + Commit | wrap in `withTx` |
| CreateBridgeHandoffReplyAndOutboxIfActive | `bridge.go:500` | same pattern | wrap in `withTx` |
| EnsureBridgeExternalSession | `bridge.go:112` | `BeginTx` upsert | wrap in `withTx` |
| (remaining bridge sites) | `bridge.go:726,818,912,1003` | `BeginTx` + Commit | wrap in `withTx` |
| ClaimPendingEvents | `agent.go:2781` (no tx wrapper; single `UPDATE…FOR UPDATE SKIP LOCKED…RETURNING`) | single statement | retryable via `RunResiliently`-style 40001 handling or `withTx`; SKIP LOCKED supported on v25.x |

### 4.2 `withTx` helper (cockroach variant only)

Refactor the 8 `BeginTx` sites to closure form through a helper that, in cockroach mode, uses `crdb.ExecuteTx` (from `github.com/cockroachdb/cockroach-go/v2/crdb`, already in go.mod v2.4.3 — the same library the pgx tutorial uses via `crdbpgxv5`; `pgx.md:315-365`):

```go
// postgres/resilience.go
func (d *DB) withTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
    if !d.isCockroach {
        tx, err := d.db.BeginTx(ctx, nil)
        if err != nil { return err }
        defer tx.Rollback()
        if err := fn(tx); err != nil { return err }
        return tx.Commit()
    }
    return crdb.ExecuteTx(ctx, d.db, nil, fn) // retries SQLSTATE 40001
}
```

- `crdb.ExecuteTx` works with both `*sql.DB` and `*sql.Conn` (both satisfy its executor interface); keep `conn.BeginTx` sites working via the same helper.
- **Constraint:** `fn` must be re-runnable — no side effects outside the transaction (verified: all 8 sites are pure read-modify-write within the tx).
- **Explicitly excluded:** `store/migrator.go` (C-new-5).

---

## 5. Files to Create/Modify (revised)

### Modified (4)

| File | Change |
|------|--------|
| `store/db/db.go` | Add `case "cockroach": driver, err = postgres.NewCockroachDB(profile)` |
| `internal/profile/profile.go` | `COCKROACH_DSN` required; no fallback; reject `sslmode=disable` in prod |
| `store/db/postgres/postgres.go` | `newDB(profile, isCockroach)`; DSN handling, pool sizing via `COCKROACH_MAX_OPEN_CONNS` (default 8) |
| `store/db/postgres/resilience.go` | 40001 in `isTransientError`; add `withTx` helper |
| `store/migrator.go` (or `migration_helper.go`) | Statement-splitting + implicit-transaction execution for `cockroach` driver only |
| `scripts/docker-compose.cockroach.yml` | `COCKROACH_DATABASE/USER/PASSWORD` env (C-new-4) |

### Created (5)

| File | Purpose |
|------|---------|
| `store/migration/cockroach/LATEST.sql` | postgres LATEST.sql + `SET serial_normalization = sql_sequence` header (C-new-1) |
| `store/migration/cockroach/0.35/1__init.sql` | versioned-dir marker for the migrator (C-new-2) |
| `scripts/gen-crdb-latest.sh` | regenerate CRDB LATEST.sql from postgres LATEST.sql (documented diffs only) |
| `fly_crdb.toml` + `Dockerfile.crdb.fly` + `scripts/fly-crdb-secrets.sh` | Section 6 |
| `task crdb:verify` | Section 9.2 verification battery |

**No `store/db/cockroach/` package is created** (Section 1.1). This deletes v1's 22-file copy list and C-2 entirely.

---

## 6. Fly.io Deployment (`fly_crdb.toml`)

### 6.1 `fly_crdb.toml`

As in v1 §4.2 with corrections:

- `RAG_PIPELINE_ENABLED = 'true'`, `VECTOR_DB_PROVIDER = 'cockroach'` unchanged.
- **First deployment only:** `FORCE_REINDEX_ON_STARTUP = 'true'` (M-3); flip to `'false'` after first successful deploy and keep `RAG_STARTUP_REINDEX_DISABLED = 'false'` so future index work runs on startup/reindex.
- `LLM_MODEL`/`LLM_MODEL_REASONING`: remove placeholder `openrouter/free`; set via `fly secrets` and reference with a comment (L-4).
- Keep `cpu_kind='shared'`, `cpus=1`: CRDB Cloud is external, so app-host CPU does not affect pool sizing (Section 1.4).

### 6.2 `Dockerfile.crdb.fly`

As v1 §4.3: `go build -tags "cockroach rag"`. Explicitly state: **no `store/db/cockroach` build tag exists** — the `cockroach` driver is always compiled and selected at runtime by `MEMOS_DRIVER` (L-5). Only `vectordb_cockroach.go` is tag-gated (existing behavior).

### 6.3 `scripts/fly-crdb-secrets.sh` (M-4)

Parameterize — no placeholders survive execution:

```bash
#!/bin/bash
set -euo pipefail
APP_NAME="${1:?usage: $0 <app-name> <cockroach-dsn> <openrouter-key> [s3-bucket]}"
COCKROACH_DSN="${2:?cockroach-dsn required}"
OPENROUTER_API_KEY="${3:?openrouter key required}"
S3_BUCKET="${4:-bchat-lancedb}"
fly -a "$APP_NAME" secrets set \
  COCKROACH_DSN="$COCKROACH_DSN" \
  OPENROUTER_API_KEY="$OPENROUTER_API_KEY" \
  ENCRYPTION_MASTER_KEY="$(uuidgen)" \
  LANCEDB_S3_BUCKET="$S3_BUCKET" \
  LANCEDB_S3_PREFIX="$APP_NAME"
```

The DSN must use `sslmode=verify-full` (H-2); CRDB Cloud general connection strings verify against system CA certs (`pgx.md:83`), so no `sslrootcert` is needed unless the operator's setup requires it.

### 6.4 Rollback and canary (H-6)

1. **Canary:** deploy with `fly deploy -c fly_crdb.toml --build-only` first; then deploy to a staging app (`fly -a bchat-crdb-staging deploy`); run the §9.1 verification battery; only then promote.
2. **Rollback:** `fly deploy -c fly_pg.toml` (existing Postgres config) is the rollback path; `DATABASE_URL` secret is untouched by this work. Since CRDB and Neon are separate deployments with separate secrets, rolling back is a deploy, not a data migration.
3. **No data migration** from Neon to CRDB is in scope for this bug — CRDB is a new deployment; if data migration is later required, use CRDB `BACKUP`/`IMPORT` tooling (out of scope, documented as follow-up).

---

## 7. Risks & Mitigations (revised)

| Risk | Severity | Mitigation |
|------|----------|------------|
| 64-bit CRDB row IDs overflow `int32` structs (C-new-1) | Critical | `SET serial_normalization = sql_sequence` in CRDB LATEST.sql; verify via `SHOW CREATE`; add scan regression test |
| Migrator fails to boot without versioned dirs (C-new-2) | Critical | `migration/cockroach/0.35/1__init.sql` marker + parity CI |
| Migration multi-statement Exec breaks without simple protocol (C-new-3) | High | Keep `default_query_exec_mode=simple_protocol`; statement-level execution for CRDB |
| DDL inside single explicit tx can partially fail at COMMIT (H-5) | High | Statement-per-implicit-transaction execution for cockroach driver |
| 40001 serialization errors on hot paths | High | `withTx` → `crdb.ExecuteTx` for the 8 inventory sites; never for migrations |
| `DATABASE_URL` fallback misconfigures cockroach driver (C-4) | High | `COCKROACH_DSN` required, no fallback, loud startup error |
| Local compose lacks DB/user bootstrap (C-new-4) | High | `COCKROACH_DATABASE/USER/PASSWORD` envs in compose |
| Connection pool exceeds 4× cluster vCPUs (H-1) | Medium | `COCKROACH_MAX_OPEN_CONNS` default 8 for 2-vCPU CRDB Cloud |
| `unique_rowid()` hotspots (M-6) | Low (accepted) | Sequences chosen for int32 compatibility; documented; int64 follow-up |
| `EXTRACT(EPOCH)` DECIMAL→INT coercion (M-1) | Low | verify-first; cast in LATEST.sql only if needed |
| `::BIGINT`/JSONB/ILIKE edge differences | Low | `crdb:verify` battery covers each construct (0.3) |
| First-deploy empty RAG index (M-3) | Medium | `FORCE_REINDEX_ON_STARTUP=true` on deploy #1 |
| Placeholder secrets (M-4) | Medium | parameterized script, `set -euo pipefail`, no defaults for DSN/key |

---

## 8. CI: Cross-Database Enforcement

**Blocker per ChatGPT; extend existing tooling:**

1. **Migration parity:** extend `scripts/validate-migrations.sh` / `validate-parity.sh` to assert:
   - `store/migration/postgres/` and `store/migration/cockroach/` have identical versioned file lists;
   - CRDB LATEST.sql equals `gen-crdb-latest.sh` output (no manual drift);
   - no CRDB-unsupported constructs in either tree (`CREATE EXTENSION`, `CONCURRENTLY`, `CREATE TRIGGER`, `ALTER COLUMN TYPE`).
2. **Cross-database integration:** run the store integration suite (`task crdb:test` extended) against **SQLite, Postgres, and CockroachDB** in CI on every PR — same repository APIs, same test code. Compose profile runs the container from `docker-compose.cockroach.yml` with the §3.3 test settings.
3. **Runtime assertions in CI:**
   - scan regression: insert a row and scan every `int32` ID field;
   - `SHOW CREATE TABLE "user"` shows `nextval` default (not `unique_rowid`);
   - 40001 injection test: force a conflict and assert `withTx` retries (count SQLSTATE 40001 in logs).

---

## 9. Verification

### 9.1 Acceptance checklist

- [ ] `docker compose -f scripts/docker-compose.cockroach.yml up -d` bootstraps DB `bchat` + user `bchat_user`
- [ ] `task build:backend:cockroach` compiles; `MEMOS_DRIVER=cockroach` connects and migrates on first boot
- [ ] `migration_history` populated; `SHOW CREATE TABLE "user"` shows `nextval` (not `unique_rowid`)
- [ ] CRUD works for memos/tickets/users/bridges; no int32 scan errors (C-new-1 regression)
- [ ] 40001 is observed in logs under concurrent writes and handled by retry (injectable in `crdb:test`)
- [ ] `agent_vectors` created by `CockroachVectorDB.Validate()` with `VECTOR(1536)`; vector search returns results
- [ ] `task crdb:verify` passes (below)
- [ ] `fly deploy -c fly_crdb.toml` on staging succeeds; `/healthz` green; prod rollback path `fly_pg.toml` documented and tested
- [ ] No SQLSTATE 40001 storms in prod logs; `COCKROACH_MAX_OPEN_CONNS` respected
- [ ] Postgres/Neon and SQLite paths unchanged (CI green)

### 9.2 `task crdb:verify` battery

```bash
# 1. SERIAL translation
SHOW CREATE TABLE "user";                                  # expect DEFAULT nextval(...)
# 2. EXTRACT(EPOCH) coercion (M-1)
INSERT INTO memo (uid, creator_id, content) VALUES ('v1','1','x'); SELECT created_ts FROM memo;
# 3. ::BIGINT cast sites (M-2) — exercise ClaimPendingEvents
# 4. JSONB/ILIKE paths — run memo_filter unit suite against CRDB
# 5. FOR UPDATE SKIP LOCKED — run concurrent ClaimPendingEvents workers
# 6. 40001 retry — force contention on agent_rate_limits, assert recovery
# 7. int32 scan regression — full CRUD sweep of every int32-ID table
```

---

## 10. Implementation Order

| Step | Task | Est. | Depends |
|------|------|------|---------|
| 1 | `profile.go` + `db.go` + `postgres.go` cockroach variant (DSN, pool, simple protocol) | 1 h | — |
| 2 | `resilience.go`: 40001 + `withTx`; refactor 8 `BeginTx` sites | 2 h | 1 |
| 3 | `migration/cockroach/` LATEST.sql + 0.35 marker + `gen-crdb-latest.sh` | 2 h | — |
| 4 | Migrator statement-level execution for cockroach driver | 2 h | 1 |
| 5 | Compose bootstrap env + `task crdb:verify` + regression tests | 2 h | 1-4 |
| 6 | CI parity + cross-DB suite | 2 h | 3-5 |
| 7 | `fly_crdb.toml`, `Dockerfile.crdb.fly`, `fly-crdb-secrets.sh`, rollback doc | 1 h | 1-5 |
| 8 | Staging deploy + full §9.1 battery | 2 h | 7 |

**Total: ~14 h** (up from v1's 8-12 h — the added effort buys single-implementation maintenance, retry correctness, and CI enforcement).

---

## 11. Non-goals (documented)

- No data migration from Neon to CRDB (new deployment; rollback = redeploy `fly_pg.toml`).
- No `int32`→`int64` ID refactor (follow-up; tracked with the sequence→`unique_rowid` swap).
- No changes to HTTP API contracts, handler layer, or service layer.
- No changes to the SQLite/MySQL/Postgres driver behavior — the cockroach variant is additive (`isCockroach` flag).
- No `CREATE EXTENSION`/pgvector — CRDB native `VECTOR` only (already in `vectordb_cockroach.go`).

---

## 12. Official Reference Index

| Reference | Used for |
|-----------|----------|
| `bugs/057/pgx.md` (official pgx tutorial) | `crdb.ExecuteTx` for every transaction; 40001 client-side retry; `sslmode=verify-full` + system CA connection strings |
| `bugs/057/prod_checklist.md` | Connection pooling ≤ 4× cluster vCPUs; TLS mandatory in prod; client-side retry handling mandatory |
| `bugs/057/test_locally.md` | In-memory/CI cluster settings; `gc.ttlseconds=600`; NOT for production |
| `bugs/057/docker.md` | Docker volumes for data; `COCKROACH_DATABASE/USER/PASSWORD` bootstrap; insecure = non-production |
| `bugs/057/cockroach-demo.md` | Connection string formats (`sslmode=require/verify-full`); in-memory demo semantics |
| Official `SERIAL` docs (stable/v26.2) | `rowid` default ⇒ `INT8 DEFAULT unique_rowid()` (64-bit, 48-bit timestamp + 15-bit node id); `sql_sequence` mode ⇒ sequence-backed values; `ALTER ROLE ALL SET` guidance |
| Official v25.x Known Limitations | Schema changes within transactions ("can fail while other statements succeed"); prepared statements + schema changes; `ALTER COLUMN TYPE` restrictions; JSONB constraint restrictions; vector index limitations |
| Official Transaction Retry Error Reference | SQLSTATE 40001, "restart transaction"; client-side retry with `SAVEPOINT cockroach_restart` protocol |
| pgx v5 docs | QueryExecMode; extended protocol = one statement per Parse; multi-statement requires simple protocol |
