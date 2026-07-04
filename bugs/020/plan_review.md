**Verdict: APPROVED WITH NITS** — the plan is directionally sound and the diagnosis is accurate, but several items need correction before execution.

---

## Critical Findings (must fix)

### 1. Table count is wrong
**Plan says:** 38+ missing tables.  
**Reality:** 53 SQLite tables vs 19 Postgres tables = **34 missing**, not 38+. The overcount inflates effort estimates and creates noise in tracking.

### 2. Method count framing is misleading
**Plan says:** "90+ methods to verify" in Sprint 3.  
**Reality:** Both drivers have exactly **183 methods**. The real work is implementing **~91 stubs** in `store/db/postgres/agent.go` (which return `errNotImplemented`) plus **9 bridge stubs** in `bridge.go` (which return `ErrBridgeUnsupportedDatabase`). Audit work is minimal; implementation work is the bulk.

### 3. Missing systematic SQL translation strategy
Every query string in the postgres driver needs `?` → `$1, $2, ...` conversion. The plan mentions this translation table but offers no mechanical approach (codemod, sed, or systematic grep-and-replace). Without this, Sprint 3 is error-prone and slow.

### 4. Neon connection string omission
The plan's DSN examples omit `&channel_binding=require`, which Neon recommends for SCRAM-SHA-256 authentication. The `.env.example` and `fly secrets set` examples should include it.

### 5. Baseline migration edge case ignored
The plan creates `0.0/00__baseline.sql` for fresh deploys but doesn't address what happens if someone already deployed postgres with the existing `0.19+` migrations. Running a `0.0` migration on an existing DB would conflict with the migration history.

---

## Nits (address before/during execution)

| # | Issue | Location | Severity |
|---|-------|----------|----------|
| 1 | `context.Background()` used in `PingContext` — should accept a timeout from config or use a bounded context | Sprint 1.3 | Medium |
| 2 | `MaxIdleConns=2` is very conservative for Neon; Neon free tier allows ~20 connections. Consider `MaxIdleConns=5` with `MaxOpenConns=10` | Sprint 1.3 | Low |
| 3 | Plan says `go mod tidy` will remove `lib/pq`, but doesn't explicitly run `go get github.com/lib/pq@none` or verify removal | Sprint 1.1 | Low |
| 4 | The `payload TEXT` → `payload JSONB DEFAULT '{}'` translation is correct, but the plan doesn't verify existing Postgres `LATEST.sql` already does this correctly for the 19 existing tables | Sprint 2.2 | Low |
| 5 | Sprint 5 doesn't mention adding `memo_filter_test.go` equivalent for postgres (sqlite has it, postgres doesn't) | Sprint 5.1 | Medium |
| 6 | The plan says "sqlite3 /var/opt/memos/memos_prod.db" as SQLite verification on Fly.io — this file won't exist if the deploy is Postgres-only | Sprint 6.3 | Low |
| 7 | `SupportsBridgeDelivery()` returns `bool` — the postgres driver currently returns `false`. If Sprint 3 implements bridge methods, this must be updated to `true`, or the interface contract breaks | Sprint 3.2 | Medium |
| 8 | No mention of `channel_binding=require` in `neon_deploy.md` Step 2 | neon_deploy.md | Low |
| 9 | The plan implies all changes are additive, but `pgx/v5/stdlib` with `database/sql` means `sql.Open("pgx", ...)` — verify this driver name is correct (it is: `pgx` via stdlib) | Sprint 1.2 | Low |

---

## Structural Observations

1. **The plan's effort estimate is optimistic.** Sprint 3 alone is 3-5 days if done carefully (91 methods × SQL translation + testing). The 1-2 day total is unrealistic unless this is a skeleton-only implementation.

2. **The existing postgres driver compiles.** `go build ./store/db/postgres/...` passes. This means the "Driver Swap" is the only truly low-risk sprint. Everything else requires deep implementation.

3. **The bridge gap is binary, not incremental.** Either the bridge tables + methods are fully implemented, or `SupportsBridgeDelivery()` stays `false` and the bridge feature is silently disabled on Postgres. The plan doesn't commit to either path.

4. **No test harness.** There's no existing Postgres test suite to run against. Sprint 5 needs a test database target (e.g., a local Postgres container or Neon free-tier project) before any verification is possible.

---

## Recommended Order of Operations

1. Fix table count in Sprint 2 (34, not 38+)
2. Add `channel_binding=require` to all DSN examples
3. Replace `context.Background()` with configurable timeout in Sprint 1.3
4. Add `memo_filter_test.go` port to Sprint 5.1
5. Decide bridge implementation scope before Sprint 3 starts (full or none)
6. Add baseline migration version guard in Sprint 2.3 to skip on existing deployments

**Do not proceed to execution until items 1, 4, and 5 are resolved.**
