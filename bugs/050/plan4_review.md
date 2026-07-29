# Adversarial Review: plan4.md (bchat × CockroachDB × AWS)

**Reviewer Context:** Cross-references `plan4.md` against:
- `cockroachdb-skills-main/` at `/home/chaschel/Desktop/cockroach/cockroachdb-skills-main/` (CockroachDB agent skills repository)
- `ccloud.md` at `/home/chaschel/Desktop/cockroach/ccloud.md` (official ccloud CLI docs)
- bchat codebase at `/home/chaschel/Documents/go/bchat/`
- Prior reviews: `plan2.1_review.md`, `plan3.1_review.md`

---

## What Plan 4 Fixes from Plan 3.1

All 11 findings from the plan3.1 adversarial review are fixed:

| Finding from 3.1 Review | Status | Evidence |
|---|---|---|
| **C-1**: `CREATE VECTOR INDEX IF NOT EXISTS` unsupported | ✅ Uses bare `CREATE VECTOR INDEX` + `42P07` error check | Lines 260-278 |
| **C-2**: Cron captures undefined `tenantID` | ✅ `processPendingTickets()` iterates all tenants via `store.ListAgentTenants()` | Lines 616-653 |
| **H-1**: Factory lacks `*sql.DB` parameter | ✅ `SetDB()` post-construction wiring, mirrors `TenantVectorDBPool.SetStore()` | Lines 336-363 |
| **H-2**: `vector_cosine_ops` unverified in skills repo | ✅ Switched to `vector_ip_ops` (only opclass in skills repo, `01-schema-design.md:152`) | Lines 105-106, 265 |
| **H-3**: `feature.vector_index.enabled` not found | ✅ Removed from prerequisites, replaced with generic version note | Lines 903-906 |
| **H-4**: Embedding JSON serialization untested | ✅ Uses `$1::VECTOR` cast for explicit typing | Lines 185, 403 |
| **M-1**: Seed 50 single-row transactions | ✅ Batch in groups of 10 per transaction | Line 754 |
| **M-2**: `psql` fragility | ✅ Replaced with `ccloud cluster sql` | Lines 1238-1247 |
| **M-3**: LanceDB orphan data | ✅ Provider switching note added | Lines 1268-1271 |
| **M-5**: Spend limit cost | ✅ Warning added on `--spend-limit 0` | Lines 720-721 |
| **M-6**: Migration invisibility | ✅ Note added to `crdb:db-check` task | Lines 1149-1151 |

---

## Verdict: APPROVED WITH NITS

Plan4 is architecturally sound and production-ready for the hackathon. One critical error remains (wrong Postgres error type), plus 3 nits. Fix these and implement.

---

## CRITICAL (Must Fix Before Implementation)

### C-4. `pq.Error` Uses Banned `lib/pq` Package — Must Use `pgconn.PgError`

**Finding:** Lines 268-269 of `Validate()` attempt to check for `pgcode == "42P07"` using `pq.Error`:

```go
// Lines 268-269 (WRONG)
var pgErr *pq.Error
if errors.As(err, &pgErr) && pgErr.Code == "42P07" {
```

This requires importing `github.com/lib/pq`, which is **explicitly banned** by AGENTS.md:
> *"pgx/v5 (sole driver — lib/pq is NOT used and must not be added)"*

**Evidence from codebase:** `store/db/postgres/resilience.go:9` imports `github.com/jackc/pgx/v5/pgconn` and uses the identical pattern at lines 18, 42:
```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) {
    switch pgErr.Code { ... }
}
```

**Severity:** CRITICAL — using `pq.Error` would require adding the banned `lib/pq` dependency. The code won't compile without it, and even if it did, `pq.Error.Code` format may differ from `pgconn.PgError.Code`.

**Fix:** Replace `pq.Error` with `pgconn.PgError` (import `"github.com/jackc/pgx/v5/pgconn"`, already in `go.sum` as transitive dependency of `pgx/v5`). No new dependency needed.

---

## NITS (Fix Before Implementation)

### N-1. `$1::VECTOR` Case Inconsistency with Skills Repo

**Finding:** Lines 185, 403 use `$6::VECTOR` with uppercase `VECTOR`. The skills repo (`03-query-patterns.md:333`) uses lowercase `$1::vector`.

```sql
-- Skills repo pattern:
embedding <=> $1::vector

-- Plan pattern:
$6::VECTOR
```

**Severity:** NIT — CRDB identifiers are case-insensitive. Both forms work. But matching the skills repo convention reduces cognitive load for future readers.

**Fix:** Change `::VECTOR` to `::vector` in both the `UPSERT` (line 185) and any other SQL strings.

---

### N-2. `processPendingTickets()` Can Filter At DB Level

**Finding:** Lines 637-644 fetch ALL tenants then skip inactive ones in Go:

```go
tenants, err := s.store.ListAgentTenants(ctx, &store.FindAgentTenant{})
...
for _, tenant := range tenants {
    if !tenant.IsActive { continue }
```

**Evidence from codebase:** `FindAgentTenant` (`store/agent.go:28-33`) has `IsActive *bool` as a filter field:

```go
type FindAgentTenant struct {
    ID       *int32
    Slug     *string
    GUID     *string
    IsActive *bool
}
```

**Severity:** NIT — currently works but fetches inactive tenants unnecessarily. For small tenant counts this is irrelevant.

**Fix:** Pass `&store.FindAgentTenant{IsActive: &[]bool{true}[0]}` to filter at the DB level.

---

### N-3. DSN Comparison May Fail for SQLite Deployments

**Finding:** Line 357 compares `vectorDBConfig.CockroachDSN == p.DSN` to decide whether to reuse the connection pool:

```go
if vectorDBConfig.CockroachDSN == "" || vectorDBConfig.CockroachDSN == p.DSN {
    cockroachDB.SetDB(s.GetDriver().GetDB())
}
```

When `MEMOS_DRIVER=sqlite`, `p.DSN` is a file path (not a Postgres DSN), so `COCKROACH_DSN` will never match it. The connection reuse falls through and `NewCockroachVectorDB` opens its own pool — correct behavior by accident. But the `SetDB` path is never taken for SQLite deployments even when CRDB DSN is empty, which is the expected case.

**Severity:** NIT — doesn't cause a bug (new pool is opened), but the condition check is misleading.

**Fix:** Add a separate condition or check the driver type. For the hackathon, leave as-is and add a comment clarifying that `SetDB` is only for Postgres-based deployments sharing a connection pool with the main application.

---

## Summary

| Severity | Count | Key Issues |
|----------|-------|-----------|
| CRITICAL | 1 | `pq.Error` uses banned `lib/pq` — must use `pgconn.PgError` from existing `pgx/v5` |
| NIT | 3 | `$1::VECTOR` case, DB-level `IsActive` filter, DSN comparison for SQLite |
| Previously fixed from 3.1 | 11 | All critical, high, and medium findings resolved |

### Pre-Implementation Checklist

1. Replace `pq.Error` → `pgconn.PgError` (line 268-269 in `Validate()`)
2. Normalize `::VECTOR` → `::vector` (lines 185, 403)
3. Pass `IsActive: &{true}` to `FindAgentTenant` in `processPendingTickets()` (line 637)
4. Add comment on DSN comparison SQLite edge case (line 357)
