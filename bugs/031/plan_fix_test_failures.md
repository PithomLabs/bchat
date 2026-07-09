# Plan: Fix Postgres Test Failures

**Date**: 2026-07-09
**Source**: Local Postgres test run (`DRIVER=postgres go test -v ./store/test/...`)
**Related**: `fix_gotest_fail.md`, `plan_fix_rate_limit_fk.md`

---

## Failures Summary

| # | Test | Error | Root Cause |
|---|------|-------|------------|
| 1 | `TestTicketStore` | `relation "user" does not exist` | Migration rollback |
| 2 | `TestUserSettingStore` | `relation "user" does not exist` | Migration rollback |
| 3 | `TestUserStore` | `relation "user" does not exist` | Migration rollback |
| 4 | `TestWebhookStore` | `relation "user" does not exist` | Migration rollback |
| 5 | `TestWorkspaceSettingV1Store` | `relation "system_setting" does not exist` | Migration rollback |
| 6 | `TestCreateBridgeHandoffReplyIfActiveDeliveryStatusConstraint` | `invalid input syntax for type bigint` | `time.Time` → BIGINT |

---

## Root Cause A: Migration Rollback (Failures 1-5)

### How it happens

1. First test runs `NewTestingStore()` → calls `resetTestingDB()` → drops some tables
2. Calls `store.Migrate()` → `preMigrate()` finds no migration history
3. `preMigrate()` reads `LATEST.sql` and executes it inside a `sql.Tx`
4. `CREATE TABLE agent_tenants` fails — table still exists from prior run (not in DROP list)
5. Transaction rolls back → **all tables vanish** including `"user"` and `"system_setting"`
6. Subsequent test operations fail with `relation "user" does not exist`

### Why it happens

`resetTestingDB()` at `store/test/store.go:59-75` has a **manual DROP list** that misses `agent_tenants` and many other agent tables:

```go
// Current DROP list (incomplete):
DROP TABLE IF EXISTS migration_history CASCADE;
DROP TABLE IF EXISTS system_setting CASCADE;
DROP TABLE IF EXISTS "user" CASCADE;
DROP TABLE IF EXISTS user_setting CASCADE;
DROP TABLE IF EXISTS webhook CASCADE;
DROP TABLE IF EXISTS memo_organizer CASCADE;
DROP TABLE IF EXISTS memo_relation CASCADE;
DROP TABLE IF EXISTS resource CASCADE;
DROP TABLE IF EXISTS reaction CASCADE;
```

Tables created by `LATEST.sql` but **NOT** in this DROP list:
`agent_tenants`, `agent_audiences`, `agent_services`, `agent_exclusions`, `agent_coverage`, `agent_faqs`, `agent_safety_protocols`, `agent_kb_sections`, `agent_intents`, `agent_rules`, `agent_sessions`, `agent_source_files`, `agent_rate_limits`, `agent_simulation_transcripts`, `agent_tenant_scripts`, `agent_simulations`, `agent_script_analysis`, `agent_analysis_results`, `agent_learning_memory`, `agent_compliance_audits`, `agent_scoring_config`, `agent_qa_pairs`, `agent_transcripts`, `agent_workflows`, `agent_reindex_checkpoints`, `agent_observations`, `agent_messages`, `agent_leads`, `tickets`, `bridge_external_sessions`, `bridge_handoffs`, `bridge_handoff_replies`, `bridge_reply_outbox`, `bridge_auth_keys`, `bridge_auth_nonces`, `notifications`, `system_secret`, `tenant_role_templates`, `user_tenant_permission`, `tenant_config`

### Fix: Replace manual DROP list with schema reset

**File**: `store/test/store.go:59-75`

Replace:
```go
} else if profile.Driver == "postgres" {
    _, err := dbDriver.GetDB().ExecContext(ctx, `
    DROP TABLE IF EXISTS migration_history CASCADE;
    DROP TABLE IF EXISTS system_setting CASCADE;
    ...`)
```

With:
```go
} else if profile.Driver == "postgres" {
    _, err := dbDriver.GetDB().ExecContext(ctx, `
        DROP SCHEMA public CASCADE;
        CREATE SCHEMA public;
    `)
```

**Why this is correct:**
- Drops ALL tables in the schema in one shot — no manual list to maintain
- `validate-pg-migrations.sh` already uses this exact pattern
- The subsequent `Migrate()` call rebuilds everything from `LATEST.sql`
- Future tables are automatically included — no risk of missing them

**Why NOT add `IF NOT EXISTS` to LATEST.sql instead:**
- `IF NOT EXISTS` makes `CREATE TABLE` silently skip when table exists — hides schema drift
- `IF NOT EXISTS` on `CREATE INDEX` is fine, but on `CREATE TABLE` it masks real problems
- The `validate-pg-migrations.sh` script tests LATEST.sql against a **clean** database — adding `IF NOT EXISTS` would not fix the test infrastructure issue

---

## Root Cause B: `time.Time` → BIGINT Mismatch (Failure 6)

### How it happens

`store/db/postgres/bridge.go` passes raw `time.Time` Go objects as SQL parameters to `BIGINT` columns:

```go
// bridge.go:24 — INSERT into bridge_external_sessions (created_at BIGINT)
result, err := d.db.ExecContext(ctx, `
    INSERT INTO bridge_external_sessions (...)
    VALUES ($1, $2, 'active', $3, $4, $5, $6)
    ON CONFLICT(...) DO NOTHING
`, tenantID, sessionID, now, now, expiresAt, now)
//                       ^^^^^ ^^^^^ ^^^^^^^ ^^^
//                       time.Time objects — pgx serializes as "2026-07-08 23:04:56.400331Z"
//                       Postgres: "invalid input syntax for type bigint"
```

**SQLite works** because it's type-flexible — it accepts timestamp strings in INTEGER columns.
**Postgres fails** because it enforces strict types — `BIGINT` cannot parse `"2026-07-08 23:04:56.400331Z"`.

The **SQLite driver** (`store/db/sqlite/bridge.go:24`) correctly calls `.Unix()`:
```go
`, tenantID, sessionID, now.Unix(), now.Unix(), expiresAt.Unix(), now.Unix())
```

### All affected methods

| Method | File:Line | Params to fix |
|--------|-----------|---------------|
| `EnsureBridgeExternalSession` | `postgres/bridge.go:24` | `now, now, expiresAt, now` → `.Unix()` each |
| `TouchBridgeExternalSession` | `postgres/bridge.go:79` | `now, now, expiresAt` → `.Unix()` each |
| `createBridgeHandoffAttempt` | `postgres/bridge.go:152` | `now, now` → `.Unix()` each |
| `UpdateBridgeHandoffRoutingModeCAS` | `postgres/bridge.go:206` | `now` (2 places) → `.Unix()` each |

### Additional scan issue

`scanBridgeHandoff` at `postgres/bridge.go:252` scans `closed_at` into `int64`:
```go
var createdAt, updatedAt, closedAt int64
```

But `closed_at` is nullable (`BIGINT` without `NOT NULL` in `LATEST.sql:787`). When `closed_at IS NULL`, scanning into plain `int64` fails. The SQLite driver correctly uses `sql.NullInt64`:
```go
var closedAt sql.NullInt64  // sqlite/bridge.go:255
```

### Fix: Convert `time.Time` → `.Unix()` + fix nullable scan

**File**: `store/db/postgres/bridge.go`

1. Lines 24, 79, 152, 206: Add `.Unix()` to all `time.Time` parameters
2. Line 252: Change `closedAt` from `int64` to `sql.NullInt64`
3. Update `scanBridgeHandoff` to handle `closedAt.Valid` / `closedAt.Int64`
4. Update the struct assignment in `FindBridgeHandoff` to use the nullable value

---

## Files Modified

| File | Change |
|------|--------|
| `store/test/store.go:59-75` | Replace manual DROP list with `DROP SCHEMA public CASCADE; CREATE SCHEMA public;` |
| `store/db/postgres/bridge.go:24` | `now, now, expiresAt, now` → `now.Unix(), now.Unix(), expiresAt.Unix(), now.Unix()` |
| `store/db/postgres/bridge.go:79` | `now, now, expiresAt` → `now.Unix(), now.Unix(), expiresAt.Unix()` |
| `store/db/postgres/bridge.go:152` | `now, now` → `now.Unix(), now.Unix()` |
| `store/db/postgres/bridge.go:206` | `now` (2 places) → `now.Unix()` |
| `store/db/postgres/bridge.go:252` | `closedAt int64` → `closedAt sql.NullInt64` + scan/usage updates |

---

## Verification

1. `go test ./...` — 0 failures (SQLite)
2. Start local Postgres: `docker compose -f scripts/docker-compose.postgres.yml up -d`
3. `DRIVER=postgres DSN="postgresql://bchat:bchat@localhost:5432/bchat" go test -v ./store/test/...` — 0 failures
4. `DATABASE_URL="postgresql://bchat:bchat@localhost:5432/bchat" ./scripts/validate-pg-migrations.sh` — passes
5. Stop container: `docker compose -f scripts/docker-compose.postgres.yml down`

---

## Out of Scope

- Adding `IF NOT EXISTS` to `LATEST.sql` (masks schema drift, not the right fix)
- Fixing the `preMigrate` transaction behavior (working as designed — the issue is test cleanup)
- Other Postgres driver files beyond `bridge.go` (no other time.Time → BIGINT issues found)
