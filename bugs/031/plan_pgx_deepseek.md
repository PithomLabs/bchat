# Plan: Consolidate on pgx for Neon/PgBouncer Compatibility

## Background

The codebase already uses `pgx` (v5.10.0 via `github.com/jackc/pgx/v5/stdlib`) exclusively. There is **zero `lib/pq` usage** anywhere — not in `go.mod`, `go.sum`, imports, or source files. The concern is not eliminating `lib/pq` (it's already gone), but ensuring the `pgx` configuration works correctly with Neon's PgBouncer connection pooling.

## The Problem: PgBouncer Prepared Statement Handling

Neon runs PgBouncer in **transaction pooling mode** in front of every database. The app connects through a pooled endpoint (hostname with `-pooler` suffix) or a direct endpoint.

PgBouncer's `transaction` mode returns connections to the pool after each transaction. If pgx caches a prepared statement (named statement) on one connection and that connection is reassigned, a subsequent query on a different connection will fail with:

```
ERROR: prepared statement "1" does not exist (26000)
```

**Current pgx default mode** (`QueryExecModeCacheStatement`) uses named prepared statements cached across transactions — incompatible with PgBouncer transaction mode.

**Neon's PgBouncer (v1.22+)** does support protocol-level prepared statements, but this feature is opt-in on Neon's side and may not be enabled for all computes.

## Recommended Approach: `default_query_exec_mode=simple_protocol`

### Option A (Recommended): Add DSN parameter to connection string

**File:** `store/db/postgres/postgres.go` (line 26)

Change:
```go
db, err := sql.Open("pgx", profile.DSN)
```
To:
```go
dsn := profile.DSN
if !strings.Contains(dsn, "default_query_exec_mode") {
    if strings.Contains(dsn, "?") {
        dsn += "&default_query_exec_mode=simple_protocol"
    } else {
        dsn += "?default_query_exec_mode=simple_protocol"
    }
}
db, err := sql.Open("pgx", dsn)
```

Add `"strings"` to the import block.

This tells pgx's `ParseConfig()` (which the `stdlib` driver calls internally) to use the simple query protocol. Under simple protocol:
- No prepared statements are used
- Parameters are interpolated client-side with proper escaping
- Queries execute in a single round trip
- Fully compatible with PgBouncer transaction mode

**Caveat:** Simple protocol sends all results as text (not binary). For numeric types and timestamps, there's a ~5-10% overhead in parsing text results. This is negligible for bchat's workload.

### Option B (If simple protocol overhead is a concern): `default_query_exec_mode=exec`

Use:
```
?default_query_exec_mode=exec
```

- Uses the extended protocol (binary encoding)
- One round trip per query (no prepare/cache)
- Pooler-safe because it doesn't rely on cached prepared statements
- Better performance than simple protocol
- Lower risk than Option A for existing code (binary format matches current behavior)

### Option C (Safest for direct + pooled): No change + audit connection strings

If Neon's PgBouncer (v1.22+) supports protocol-level prepared statements, the default `cache_statement` mode works fine — **as long as the app uses the pooled endpoint**. However, if the connection string changes or is misconfigured, the prepared statement error can appear.

## Implementation Steps

### Step 1: Audit connection string usage

Search all places where `profile.DSN` or `os.Getenv("DATABASE_URL")` is set:
- `store/db/postgres/postgres.go:26` — where `sql.Open("pgx", profile.DSN)` is called
- `internal/profile/profile.go:97-101` — where `DATABASE_URL` env var is read as DSN
- `fly_pg.toml` — where `MEMOS_DSN` is set (check if pooled or direct endpoint)

### Step 2: Add `default_query_exec_mode` parameter

In `store/db/postgres/postgres.go`, modify the `sql.Open` call as shown in Option A above. Add `"strings"` to the import block.

### Step 3: Audit `Prepare()` call sites

Search for any `db.PrepareContext()`, `tx.PrepareContext()`, `db.Prepare()`, or `tx.Prepare()` in Postgres-specific code that might bypass the `default_query_exec_mode` setting:

```bash
grep -rn "\.Prepare(" store/db/postgres/
```

If any exist, they need to use the direct connection or be refactored to use `ExecContext`/`QueryContext` with simple protocol.

### Step 4: Add fallback for non-pooled operations

For schema migrations (`preMigrate` in `migrator.go`), Neon recommends using the **direct** (non-pooled) connection. The migration runs `LATEST.sql` inside a `BEGIN`/`COMMIT` block. If the DSN points to the pooled endpoint, migration-level commands like `CREATE TABLE` will work but `SET`/`RESET` will fail.

**Fix:** Ensure `MEMOS_DSN` in `fly_pg.toml` points to the direct endpoint for `preMigrate`, or add logic to use a separate DSN for migrations.

### Step 5: Update `validate-pg-migrations.sh`

The validation script at line 82 uses `psql` to run `LATEST.sql`. Update it to wrap in a transaction to match production behavior:

```bash
psql "$TEST_URL" -c "BEGIN; $(cat "$LATEST_SQL"); COMMIT;"
```

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Simple protocol text encoding breaks Go `Scan` on numeric types | Low | Runtime panic | Use `QueryExecModeExec` instead of `SimpleProtocol` |
| `Prepare()` call sites break under simple protocol | Medium | Silent degradation | Audit all Prepare calls and switch to direct queries |
| Migration uses pooled endpoint, fails on SET/LISTEN | Low for Neon | Migration rollback | Ensure `MEMOS_DSN` uses direct endpoint explicitly |
| Performance regression from text encoding | Low | 5-15% slower queries | Acceptable for bchat workload; revisit if needed |

## Verification

1. Run `go build` to ensure no compile errors
2. Run `scripts/validate-pg-migrations.sh` against a Neon test database
3. Deploy to Fly.io and confirm health check passes (`/healthz`)
4. Execute a chat message via the API to confirm query execution works
5. Check app logs for `prepared statement does not exist` errors — if they appear, the fix is necessary