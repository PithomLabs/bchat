# Plan: Consolidate Postgres under pgx + Harden for Neon

**Author:** hyk3
**Context:** `bugs/031` — follow-up to the adversarial `LATEST.sql` review. Goal: eliminate `lib/pq` and consolidate everything under `pgx`, tuned for Neon Postgres on Fly.io.

---

## Investigation Findings

`lib/pq` is **already entirely absent** from the repository:
- Not in `go.mod` / `go.sum`.
- Not present in `go mod graph` (no transitive dependency).
- No `_ "github.com/lib/pq"` import anywhere.
- No `pq.Array` / `pq.CopyIn` / `pq.StringArray` usage.
- No native `TEXT[]` columns in `store/migration/postgres/LATEST.sql` (arrays stored as JSON `TEXT`).
- No `vendor/` directory.

Postgres is **100% `github.com/jackc/pgx/v5`** today:
- `store/db/postgres/postgres.go:9` → `_ "github.com/jackc/pgx/v5/stdlib"`
- `store/db/postgres/postgres.go:26` → `sql.Open("pgx", profile.DSN)`

**Conclusion:** There is nothing to literally "eliminate" today. The real work is (a) hardening pgx for Neon so it does not exhibit the classic pq-vs-pgx incompatibilities, and (b) adding a guard so `lib/pq` can never re-enter the dependency tree.

### How lib/pq and pgx *would* conflict on Neon (awareness / prevention)

1. **Dual driver stacks** — `lib/pq` registers the sql driver name `"postgres"`; `pgx/v5/stdlib` registers `"pgx"`. If both were imported, `sql.Open("postgres",…)` vs `sql.Open("pgx",…)` would open two independent connection stacks against Neon (different TLS/timeout/pool behavior). Not possible while only `"pgx"` is used.
2. **Multi-statement execution** — `lib/pq` (simple protocol) runs a whole script in one `Exec`; `pgx` defaults to `QueryExecModeExec` (extended protocol), which can reject multiple statements in a single query. The migrator (`store/migrator.go`) runs the entire `LATEST.sql` via one `tx.ExecContext()`. It currently works under pgx (the prior failed deploy reached FK errors, proving multi-statement execution), but this is fragile/driver-dependent.
3. **Neon pgbouncer / serverless proxy** — Neon's `-pooler` endpoint is pgbouncer in *transaction* mode, which supports **only the simple query protocol**. pgx's default extended protocol fails behind it (prepared-statement errors); `lib/pq` (simple-only) works. This is the #1 reason "pq worked but pgx fails" stories exist → pgx must use `default_query_exec_mode=simple_protocol` for Neon.
4. **DSN parsing** — both accept `postgresql://…?sslmode=require` (Neon requires TLS). No pq-specific DSN params are constructed in this repo.
5. **Type mapping** — `pq.StringArray`/`pq.Int64Array` vs pgx codecs. Schema has no native `TEXT[]` columns, so zero conflict.
6. **Bulk COPY** — `pq.CopyIn` vs `pgx.CopyFrom`; no `CopyIn` usage exists.
7. **Serverless resilience** — Neon aggressively kills idle connections; pgx v5 has better error classification (`pgconn.SafeToRetry`) and pool health checks than `lib/pq`. Consolidating on pgx is the stronger choice for Neon.

---

## Decisions (confirmed with user)

- **DSN mode:** Add `default_query_exec_mode=simple_protocol` to the pgx DSN. (Recommended — fixes pgbouncer/Neon-pooled compat and makes multi-statement `LATEST.sql` deterministic.)
- **Scope:** Leave the MySQL driver (`github.com/go-sql-driver/mysql`) as-is; only Postgres/Neon is consolidated to pgx.
- **CI guard:** Add a guard that fails the build if `github.com/lib/pq` re-enters `go.mod`.

---

## Implementation Plan (edits to make in Build mode)

### 1. `store/db/postgres/postgres.go` — force pgx simple protocol for Neon
- Add `"strings"` to the import block.
- In `NewDB`, derive the DSN and append `default_query_exec_mode=simple_protocol` (preserving `?`/`&`, skip if already present), then open with it:
```go
  dsn := profile.DSN
  if !strings.Contains(dsn, "default_query_exec_mode") {
      sep := "?"
      if strings.Contains(dsn, "?") {
          sep = "&"
      }
      dsn += sep + "default_query_exec_mode=simple_protocol"
  }
  db, err := sql.Open("pgx", dsn)
  ```
- Keep existing pool settings (`SetMaxOpenConns(10)`, `SetMaxIdleConns(5)`, `SetConnMaxLifetime(5m)`, `SetConnMaxIdleTime(1m)`).
- **Do NOT** add `pool_*` DSN params — they are no-ops under the `database/sql` stdlib wrapper (effective only via `pgxpool`). Migrating to `pgxpool` is noted as future work, out of scope.
- Effect: deterministic multi-statement `LATEST.sql` execution + compatibility with Neon's pooled (pgbouncer transaction-mode) endpoint.

### 2. `Taskfile.yml` — regression guard
- Add a new task:
  ```yaml
  validate:no-libpq:
    desc: Fail if github.com/lib/pq re-enters the dependency tree
    cmds:
      - '! grep -q "lib/pq" go.mod'
  ```
- Wire it as a dependency of `validate:migrations` (which is already in the `build:backend` chain, so it runs in CI).

### 3. Docs (low priority)
- Note in `AGENTS.md` / `scripts/fly-pg-secrets.sh` that pgx is the sole Postgres driver and the Neon connection URL must not assume `lib/pq` semantics.

---

## Verification (after edits)

- `go mod tidy` → confirm `grep -q "lib/pq" go.mod` is empty.
- `task validate:no-libpq` → passes.
- `go build ./...` → clean.
- `task validate:migrations` / `scripts/validate-pg-migrations.sh` (needs local Postgres) → multi-statement `LATEST.sql` still applies under `simple_protocol`.
- `fly deploy` to Neon → migration succeeds, `/healthz` passes.

---

## Out of Scope
- MySQL driver remains untouched.
- Full migration from `database/sql` + pgx stdlib to `pgxpool` (future enhancement; would enable `pool_*` tuning for Neon idle-kill resilience).
