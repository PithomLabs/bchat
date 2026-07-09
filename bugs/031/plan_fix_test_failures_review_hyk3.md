# Adversarial Plan Review: `bugs/031/plan_fix_test_failures.md`

**Reviewer:** hyk3
**Input:** `bugs/031/plan_fix_test_failures.md` (fixes 6 Postgres test failures: 5 migration-rollback + 1 `time.Time`→BIGINT)
**Goal:** Minimum-viable, self-contained plan a coding agent can execute without another review round.

---

## Verdict

The plan's **diagnosis is correct** and the two fixes are on the right track. However, there are **two required corrections** (one false premise, one missed nullable scan) plus cautions. All four `bridge.go` INSERT methods were verified against source; the `resetTestingDB` DROP-list gap was verified.

---

## Root Cause A — Migration Rollback (Failures 1–5)

**Confirmed correct diagnosis.** `resetTestingDB` (`store/test/store.go:59-75`) drops only 15 tables and **omits `agent_tenants` and all ~40 agent/bridge/ticket tables**. A stale `agent_tenants` survives from a prior run; `LATEST.sql`'s `CREATE TABLE agent_tenants` (no `IF NOT EXISTS`, `LATEST.sql:146`) then fails → the whole `preMigrate` transaction rolls back → `"user"` / `"system_setting"` (created inside that tx) vanish → downstream tests fail with `relation "user" does not exist`. Root cause is sound; `DROP SCHEMA public CASCADE; CREATE SCHEMA public;` is the right fix (maintenance-free, future-proof).

### Corrections / cautions
- **Required (factual):** The claim *"validate-pg-migrations.sh already uses this exact pattern"* is **false**. That script does `DROP DATABASE IF EXISTS` / `CREATE DATABASE` (`validate-pg-migrations.sh:78-79, 94-95`), never `DROP SCHEMA public CASCADE`. Do not cite it as precedent — state instead that this is the standard test-isolation pattern.
- **Caution (MEDIUM):** `DROP SCHEMA public CASCADE` also drops any **extensions installed in `public`** (e.g., `pgcrypto`, `vector`). `LATEST.sql` does not depend on them, so a fresh local test DB is fine, but note the risk if the test DB has extensions.
- **Caution (MEDIUM):** Destructive on whatever DB the DSN points at. The verification DSN is `postgresql://bchat:bchat@localhost:5432/bchat` (the real app DB). Tests already wipe known tables, so this is consistent in intent, but recommend pointing tests at a **disposable** DB and using `DROP SCHEMA IF EXISTS public CASCADE` (add `IF EXISTS` for safety).
- The plan's rejection of "add `IF NOT EXISTS` to LATEST.sql" is correct (it would mask real schema drift).

---

## Root Cause B — `time.Time` → BIGINT (Failure 6)

**All four methods confirmed** passing raw `time.Time` to `BIGINT` columns (verified in `store/db/postgres/bridge.go`):

| Method | Line | Params to fix |
|--------|------|---------------|
| `EnsureBridgeExternalSession` | `:24` | `now, now, expiresAt, now` → `.Unix()` each |
| `TouchBridgeExternalSession` | `:79` | `now, now, expiresAt` → `.Unix()` each |
| `createBridgeHandoffAttempt` | `:152` | `now, now` → `.Unix()` each |
| `UpdateBridgeHandoffRoutingModeCAS` | `:206` | `now` (updated_at) + `now` (closed_at) → `.Unix()` each |

**SQLite parity confirmed:** `sqlite/bridge.go` uses `.Unix()` on inserts (`:25,81,154,208`) and `sql.NullInt64` on scans (`:50,255`). `CreateBridgeHandoffReply.Now` is `int64` (`store/bridge.go:106`), so the reply-insert path (`create.Now`) is already correct — not an issue.

### Required correction 1 (MEDIUM — missed nullable scan)
The plan fixes `closed_at` at `scanBridgeHandoff:252`, but **`FindBridgeExternalSession` (`:48`, `:55`) scans `expires_at`/`last_seen_at` into plain `int64`**, even though those columns are **nullable `BIGINT`** in `LATEST.sql:758-759`. SQLite tolerates NULL→int64 (yields 0); **Postgres errors** (`cannot scan NULL into int64`). `sqlite/bridge.go:50` correctly uses `sql.NullInt64` for these. This is a latent Postgres failure the plan omits.

**Fix:** in `FindBridgeExternalSession`,
```go
var createdAt, updatedAt, expiresAt, lastSeenAt int64
```
→
```go
var createdAt, updatedAt int64
var expiresAt, lastSeenAt sql.NullInt64
```
and lines 66-67:
```go
session.ExpiresAt = nullableUnixTime(expiresAt)
session.LastSeenAt = nullableUnixTime(lastSeenAt)
```
→
```go
session.ExpiresAt = nullableUnixTimeNull(expiresAt)
session.LastSeenAt = nullableUnixTimeNull(lastSeenAt)
```

### Required correction 2 (LOW — precise helper usage)
For `closed_at`, the plan says "use the nullable value" but must be explicit: change `scanBridgeHandoff` line 265
```go
handoff.ClosedAt = nullableUnixTime(closedAt)
```
→
```go
handoff.ClosedAt = nullableUnixTimeNull(closedAt)   // helper already exists at bridge.go:286
```
(`closedAt` becomes `sql.NullInt64`; `nullableUnixTime(int64)` would no longer compile.)

---

## Cross-Plan Dependency (important)

Failure 6's test — `TestCreateBridgeHandoffReplyIfActiveDeliveryStatusConstraint` — is the **same test** as `plan_fix_gotest.md`:
- On **SQLite** (default) it fails on the `delivery_status` CHECK (`'delivered'` now valid) → fixed there by `'delivered'`→`'bogus_status'` + assertion `"constraint"`.
- On **Postgres** it fails on `invalid input syntax for type bigint` (the `time.Time`→BIGINT in `bridge.go`, hit during `createHumanActiveHandoffFixture` setup) → fixed here.

**Both fixes must be applied together**, or the test still fails under one driver. Note this in the hand-off.

---

## Recommended Verification Additions
- After the `bridge.go` fix, run the **full** Postgres suite: `DRIVER=postgres DSN=... go test ./store/...` (not only `./store/test/...`) to catch any other `time.Time`→BIGINT mismatch the plan assumes don't exist.
- Quick audit: `grep -rn "time.Time" store/db/postgres/` cross-checked against `BIGINT` columns, to confirm `bridge.go` is the only offender.

---

## Revised MVP Edits

| File | Change |
|------|--------|
| `store/test/store.go:59-75` | Postgres branch → `DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;` |
| `store/db/postgres/bridge.go:24` | `now, now, expiresAt, now` → `now.Unix(), now.Unix(), expiresAt.Unix(), now.Unix()` |
| `store/db/postgres/bridge.go:79` | `now, now, expiresAt` → `now.Unix(), now.Unix(), expiresAt.Unix()` |
| `store/db/postgres/bridge.go:152` | `now, now` → `now.Unix(), now.Unix()` |
| `store/db/postgres/bridge.go:206` | both `now` → `now.Unix()` (updated_at and closed_at) |
| `store/db/postgres/bridge.go:48,55,66,67` | `expiresAt, lastSeenAt` → `sql.NullInt64`; use `nullableUnixTimeNull(...)` |
| `store/db/postgres/bridge.go:252,265` | `closedAt` → `sql.NullInt64`; line 265 → `nullableUnixTimeNull(closedAt)` |
| `store/test/bridge_test.go` (from `plan_fix_gotest.md`) | `'delivered'`→`'bogus_status'` + assertion `"constraint"` (apply alongside) |

### Verification
1. `go test ./...` — 0 failures (SQLite).
2. `docker compose -f scripts/docker-compose.postgres.yml up -d`
3. `DRIVER=postgres DSN="postgresql://bchat:bchat@localhost:5432/bchat" go test -v ./store/test/...` — 0 failures.
4. `DATABASE_URL="postgresql://bchat:bchat@localhost:5432/bchat" ./scripts/validate-pg-migrations.sh` — passes.
5. `docker compose -f scripts/docker-compose.postgres.yml down`

### Excluded
- Adding `IF NOT EXISTS` to `LATEST.sql` (masks schema drift).
- Fixing `preMigrate` transaction behavior (working as designed; issue is test cleanup).
- Other Postgres driver files — assume none affected, but verify via full Postgres suite run (above).
