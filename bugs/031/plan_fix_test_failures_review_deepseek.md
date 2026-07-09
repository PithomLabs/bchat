# Adversarial Plan Review: Fix Postgres Test Failures

**Reviewer:** DeepSeek V4 Flash
**Target:** `bugs/031/plan_fix_test_failures.md`
**Status:** Plan verified — approved. One factual inaccuracy noted below.

---

## Verification Summary

| Claim | Source | Result |
|-------|--------|--------|
| Manual DROP list misses `agent_tenants` and ~39 other tables | `store/test/store.go:59-75` | ✅ |
| `CREATE TABLE agent_tenants` fails on re-run → tx rollback → all tables lost | `migrator.go:137-162` — `preMigrate()` inside `sql.Tx` | ✅ |
| Postgres `bridge.go:24` passes raw `time.Time` to BIGINT column | `postgres/bridge.go:19-24` — `VALUES ($1, ..., $3, $4, $5, $6)` with `now, now, expiresAt, now` | ✅ |
| Postgres `bridge.go:79` passes raw `time.Time` | `postgres/bridge.go:79` — `SET updated_at = $1, last_seen_at = $2, expires_at = $3` with `now, now, expiresAt` | ✅ |
| Postgres `bridge.go:152` passes raw `time.Time` | `postgres/bridge.go:152` — `VALUES (..., $6, $7)` with `now, now` | ✅ |
| Postgres `bridge.go:206` passes raw `time.Time` (2 occurrences) | `postgres/bridge.go:206` — `now` twice in args list | ✅ |
| SQLite `bridge.go` correctly uses `.Unix()` | `sqlite/bridge.go:25` — `now.Unix(), now.Unix(), expiresAt.Unix(), now.Unix()` | ✅ |
| `closed_at` is nullable (`BIGINT` without `NOT NULL`) | `LATEST.sql:787` — `closed_at BIGINT,` | ✅ |
| `scanBridgeHandoff` scans `closed_at` into plain `int64` | `postgres/bridge.go:252` — `var createdAt, updatedAt, closedAt int64` | ✅ |
| SQLite version uses `sql.NullInt64` for nullable columns | `sqlite/bridge.go:50` — `var expiresAt, lastSeenAt sql.NullInt64` | ✅ |

---

## Findings

### No blockers — plan is correct

Both root causes are correctly identified and the fixes are minimal and correct.

**Root Cause A:** The manual DROP list in `resetTestingDB()` is missing ~39 tables. `DROP SCHEMA public CASCADE; CREATE SCHEMA public;` is the correct fix — used throughout the Postgres ecosystem, drops everything in one shot, no maintenance burden.

**Root Cause B:** Four methods in `postgres/bridge.go` pass raw `time.Time` objects to `BIGINT` columns. Postgres rejects this (`invalid input syntax for type bigint`). The fix (adding `.Unix()` to all four sites + changing `closedAt` from `int64` to `sql.NullInt64`) exactly mirrors the SQLite driver's correct implementation.

### One factual inaccuracy (does not affect correctness)

**Line 77:** "`validate-pg-migrations.sh` already uses this exact pattern"

The script at `scripts/validate-pg-migrations.sh` does NOT use `DROP SCHEMA public CASCADE`. It uses `DROP DATABASE` / `CREATE DATABASE` (lines 78-79, 93-95). The `DROP SCHEMA` pattern is standard and correct, but the script doesn't currently demonstrate it. This does not affect the fix's correctness.

---

## Files to change

| File | Change | Risk |
|------|--------|------|
| `store/test/store.go:59-75` | Replace manual DROP list with `DROP SCHEMA public CASCADE; CREATE SCHEMA public;` | Low — atomic schema reset |
| `store/db/postgres/bridge.go:24` | `now` → `now.Unix()` (4 occurrences on this line) | Low — same pattern as SQLite driver |
| `store/db/postgres/bridge.go:79` | `now` → `now.Unix()` (2 occurrences), `expiresAt` → `expiresAt.Unix()` | Low |
| `store/db/postgres/bridge.go:152` | `now` → `now.Unix()` (2 occurrences) | Low |
| `store/db/postgres/bridge.go:206` | `now` → `now.Unix()` (2 occurrences) | Low |
| `store/db/postgres/bridge.go:252` | `closedAt int64` → `closedAt sql.NullInt64` + adjust `nullableUnixTime` call | Low — exactly mirrors SQLite pattern |

---

## Verdict

**PLAN READY FOR IMPLEMENTATION.** Both root causes are correctly identified, the fixes are correct and minimal, and all claims are verified against source code. The one inaccuracy (validate-pg-migrations.sh pattern) is cosmetic and does not affect the fix.