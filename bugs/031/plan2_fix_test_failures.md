# Plan2: Fix Postgres Test Failures

**Date**: 2026-07-09
**Reviews incorporated**: `plan_fix_test_failures_review_hyk3.md`, `plan_fix_test_failures_review_deepseek.md`
**Cross-plan dependency**: `plan_fix_gotest.md` changes already applied (test 6 requires both)

---

## Failures Summary

| # | Test | Error | Root Cause |
|---|------|-------|------------|
| 1-4 | `TestTicketStore`, `TestUserStore`, `TestUserSettingStore`, `TestWebhookStore` | `relation "user" does not exist` | Migration rollback |
| 5 | `TestWorkspaceSettingV1Store` | `relation "system_setting" does not exist` | Migration rollback |
| 6 | `TestCreateBridgeHandoffReplyIfActiveDeliveryStatusConstraint` | `invalid input syntax for type bigint` | `time.Time` → BIGINT |

---

## Root Cause A: Migration Rollback (Failures 1-5)

`resetTestingDB()` at `store/test/store.go:59-75` has a manual DROP list missing ~40 tables. Stale `agent_tenants` survives from prior run → `preMigrate()` fails to `CREATE TABLE agent_tenants` (no `IF NOT EXISTS`) → transaction rolls back → all tables vanish → `"user"` / `"system_setting"` not found.

**Fix**: Replace manual DROP list with schema reset.

**File**: `store/test/store.go:59-75`

```go
// BEFORE: manual DROP list (missing agent_tenants + ~40 others)
_, err := dbDriver.GetDB().ExecContext(ctx, `
    DROP TABLE IF EXISTS migration_history CASCADE;
    DROP TABLE IF EXISTS system_setting CASCADE;
    DROP TABLE IF EXISTS "user" CASCADE;
    ...`)

// AFTER: atomic schema reset
_, err := dbDriver.GetDB().ExecContext(ctx, `
    DROP SCHEMA IF EXISTS public CASCADE;
    CREATE SCHEMA public;
    `)
```

---

## Root Cause B: `time.Time` → BIGINT (Failure 6)

### B1: INSERT methods passing `time.Time` to BIGINT columns

Four methods pass raw `time.Time` to BIGINT. pgx serializes as `"2026-07-08 23:04:56.400331Z"` — Postgres rejects it.

**File**: `store/db/postgres/bridge.go`

| Method | Line | Current | Fixed |
|--------|------|---------|-------|
| `EnsureBridgeExternalSession` | 24 | `now, now, expiresAt, now` | `now.Unix(), now.Unix(), expiresAt.Unix(), now.Unix()` |
| `TouchBridgeExternalSession` | 79 | `now, now, expiresAt` | `now.Unix(), now.Unix(), expiresAt.Unix()` |
| `createBridgeHandoffAttempt` | 152 | `now, now` | `now.Unix(), now.Unix()` |
| `UpdateBridgeHandoffRoutingModeCAS` | 206 | `now` (2 places) | `now.Unix()` (2 places) |

### B2: Nullable BIGINT scans

**`FindBridgeExternalSession`** (line 48): `expires_at` and `last_seen_at` are nullable BIGINT but scanned into plain `int64`.

```go
// BEFORE (line 48):
var createdAt, updatedAt, expiresAt, lastSeenAt int64

// AFTER:
var createdAt, updatedAt int64
var expiresAt, lastSeenAt sql.NullInt64
```

Lines 66-67:
```go
// BEFORE:
session.ExpiresAt = nullableUnixTime(expiresAt)
session.LastSeenAt = nullableUnixTime(lastSeenAt)

// AFTER:
session.ExpiresAt = nullableUnixTimeNull(expiresAt)
session.LastSeenAt = nullableUnixTimeNull(lastSeenAt)
```

**`scanBridgeHandoff`** (line 252): `closed_at` is nullable BIGINT but scanned into plain `int64`.

```go
// BEFORE (line 252):
var createdAt, updatedAt, closedAt int64

// AFTER:
var createdAt, updatedAt int64
var closedAt sql.NullInt64
```

Line 265:
```go
// BEFORE:
handoff.ClosedAt = nullableUnixTime(closedAt)

// AFTER:
handoff.ClosedAt = nullableUnixTimeNull(closedAt)
```

`nullableUnixTimeNull` already exists at line 286.

---

## Files Modified

| # | File | Lines | Change |
|---|------|-------|--------|
| 1 | `store/test/store.go` | 59-75 | Postgres DROP → `DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;` |
| 2 | `store/db/postgres/bridge.go` | 24 | `.Unix()` on 4 time params |
| 3 | `store/db/postgres/bridge.go` | 79 | `.Unix()` on 3 time params |
| 4 | `store/db/postgres/bridge.go` | 152 | `.Unix()` on 2 time params |
| 5 | `store/db/postgres/bridge.go` | 206 | `.Unix()` on 2 time params |
| 6 | `store/db/postgres/bridge.go` | 48,66,67 | `expiresAt, lastSeenAt` → `sql.NullInt64` + `nullableUnixTimeNull()` |
| 7 | `store/db/postgres/bridge.go` | 252,265 | `closedAt` → `sql.NullInt64` + `nullableUnixTimeNull()` |

---

## Verification

1. `go test ./...` — 0 failures (SQLite)
2. `docker compose -f scripts/docker-compose.postgres.yml up -d`
3. `DRIVER=postgres DSN="postgresql://bchat:bchat@localhost:5432/bchat" go test -v ./store/test/...` — 0 failures
4. `DATABASE_URL="postgresql://bchat:bchat@localhost:5432/bchat" ./scripts/validate-pg-migrations.sh` — passes
5. `docker compose -f scripts/docker-compose.postgres.yml down`
