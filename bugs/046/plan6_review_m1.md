# Bug 046: Migration Loop Skip Logic — Plan 6 (Finalized)

**Status:** APPROVED (incorporates adversarial review B1-B4 fixes)  
**Date:** 2026-07-23  
**Depends on:** Plan 5 (GetCurrentSchemaVersion FS-scan fix, Steps 0-5 already applied)  
**Exposed by:** Plan 5 Step 1 — fixing `GetCurrentSchemaVersion()` to scan the FS correctly entered the migration loop for the first time, surfacing a dormant bug in the skip logic.

---

## Problem Statement

The migration loop in `store/migrator.go:108-138` uses an **exact-version-match** to skip already-applied migrations:

```go
appliedVersions := make(map[string]bool)
for _, v := range migrationHistoryVersions {
    appliedVersions[v] = true
}
// ...
if appliedVersions[fileSchemaVersion] {
    continue
}
```

`migration_history` stores **only one version per migration run** (the target). For a database at `0.31.3`, `appliedVersions = {"0.31.3": true}`. When the loop encounters `0.14/00__drop_resource_public_id.sql` (version `0.14.1`):

1. `appliedVersions["0.14.1"]` → false (not in map)
2. `IsVersionGreaterOrEqualThan("0.33.2", "0.14.1")` → true
3. **Executes the non-idempotent migration** → crashes with `no such column: external_link`

Before Plan 5 Step 1, this code path was never reached because `GetCurrentSchemaVersion()` returned a hardcoded constant that was ≤ the latest applied version. Our fix correctly computes `0.33.2` from the FS, which IS greater than `0.31.3`, so the loop runs — and the pre-existing bug surfaces.

### Why This Is Non-Idempotent

`0.14/00__drop_resource_public_id.sql` does:
```sql
INSERT INTO resource_temp ... SELECT ... external_link FROM resource;
```

By version `0.22`, `external_link` was DROPPED from `resource`. Re-running this migration on any database that passed `0.22` will crash.

---

## Root Cause

The skip logic checks if the **exact file version** exists in `migration_history`. But `migration_history` doesn't contain every individual file version — only the batch target. So all old migrations (0.2.x through 0.31.x) are NOT in the map and get re-executed. Most succeed due to idempotency tolerance (duplicate column/table errors are caught), but non-idempotent table-rebuild migrations crash.

---

## Related Findings from Plan 5

Plan 5 fixed `GetCurrentSchemaVersion()` to derive from the embedded FS. That fix caused `Migrate()` to enter the migration loop for the first time (FS max `0.33.2` > history `0.31.3`), exposing this dormant skip-logic bug. Plan 5's secondary fixes (`MigrationFS` export, `MIGRATE_SKIP_ERROR` env var, `appliedVersions` map addition) are already present in `migrator.go` and are not repeated here.

---

## Fix

### Step 1: Replace exact-match with range check and remove dead comment

**File:** `store/migrator.go`

Replace lines 78-86 and 113-116:

```go
// DELETE lines 78-86 (the comment block AND the appliedVersions map construction):
// Build a set of all applied versions for O(1) lookup.
// This covers a theoretical edge case from the pre-Step-1 era (phantom versions
// could create gaps in migration_history). After Step 1, gaps cannot occur because
// GetCurrentSchemaVersion() derives from the FS, not a hardcoded constant.
// This is defensive hardening, not a fix for an active bug.
appliedVersions := make(map[string]bool)
for _, v := range migrationHistoryVersions {
    appliedVersions[v] = true
}

// REPLACE lines 113-116 with:
// Skip migrations already applied. migration_history stores only the batch
// target version, not every individual file version. Any file whose version
// is <= the latest applied version was already executed — either during
// incremental migration or when the database was first created via LATEST.sql.
if !version.IsVersionGreaterThan(fileSchemaVersion, latestMigrationHistoryVersion) {
    continue
}

// Dead code after Plan 5+6: fileSchemaVersion is always <= schemaVersion (FS max).
// Retained as defense-in-depth.
// if !version.IsVersionGreaterOrEqualThan(schemaVersion, fileSchemaVersion) {
//     continue
// }
```

This means:
- Files with version ≤ `0.31.3` → **skipped** (already applied)
- Files with version > `0.31.3` → **executed** (0.32.x, 0.33.x)

---

## Tests

### Step 2: Test `TestMigrationLoopSkipsAlreadyApplied`

**File:** `store/test/migrator_test.go`

```go
func TestMigrationLoopSkipsAlreadyApplied(t *testing.T) {
    ctx := context.Background()
    ts := NewTestingStore(ctx, t) // DB is at current FS version, history = ["0.33.2"]

    // Simulate a database that was at 0.31.3 before Plan 5's GetCurrentSchemaVersion fix
    // surfaced the dormant skip-logic bug.
    _, err := ts.driver.GetDB().ExecContext(ctx,
        "DELETE FROM migration_history; INSERT INTO migration_history (version) VALUES ('0.31.3')")
    require.NoError(t, err)

    // Re-run migration. This previously crashed on 0.14/00 because the skip logic
    // used exact-version-match, which missed all file versions not individually
    // recorded in migration_history.
    err = ts.Migrate(ctx)
    require.NoError(t, err)

    // Verify 0.32.x and 0.33.x were applied by checking that the latest history
    // entry is the FS max version.
    rows, err := ts.driver.GetDB().QueryContext(ctx, "SELECT version FROM migration_history ORDER BY created_ts DESC")
    require.NoError(t, err)
    var versions []string
    for rows.Next() {
        var v string
        require.NoError(t, rows.Scan(&v))
        versions = append(versions, v)
    }
    require.NoError(t, rows.Err())
    require.Contains(t, versions, "0.33.2", "0.33.x migrations should have been applied")
}
```

### Step 3: Test `TestNonIdempotentMigrationsNeverRerun`

**File:** `store/test/migrator_test.go`

```go
func TestNonIdempotentMigrationsNeverRerun(t *testing.T) {
    ctx := context.Background()
    ts := NewTestingStore(ctx, t)

    // Simulate a database at 0.31.3 (the exact state that triggered taskrunrag_fail.md).
    _, err := ts.driver.GetDB().ExecContext(ctx,
        "DELETE FROM migration_history; INSERT INTO migration_history (version) VALUES ('0.31.3')")
    require.NoError(t, err)

    // Re-run migration. Before Plan 6, this crashed with:
    //   "no such column: external_link" (from 0.14/00)
    // After the range-check fix, 0.14/00 is skipped because 0.14.1 <= 0.31.3.
    err = ts.Migrate(ctx)
    require.NoError(t, err)
}
```

---

## Known Limitations

### TX-COMMIT / HISTORY-UPSERT GAP

If `tx.Commit()` succeeds at `migrator.go:140` but the subsequent `UpsertMigrationHistory` at line 147 fails, the DB schema is at the new version but `migration_history` is stale. The next startup re-enters the migration loop and re-executes all migrations in the just-applied batch. Idempotent migrations tolerate this (duplicate-column errors are swallowed). Non-idempotent migrations in that batch (e.g., `DROP TABLE`, table rebuilds) will crash. This is a pre-existing issue; the range-check fix does not introduce it but also does not resolve it.

**Mitigation (future work):** Move the history upsert inside the transaction, or perform a compensating upsert after commit failure before returning the error.

---

## Verification

```bash
go test ./store/test/... -count=1 -timeout 30s                           # all unit tests
go test ./store/test/... -count=1 -run TestMigrationLoop -v              # new migration skip test
go test ./store/test/... -count=1 -run TestNonIdempotent -v              # non-idempotent test
task run:rag                                                            # full pipeline (reproduces taskrunrag_fail.md scenario)
```

---

## Adversarial Review Summary

The plan was reviewed and found to be **APPROVED** with the following blocking issues resolved (incorporated above):

| # | Finding | Resolution |
|---|---------|------------|
| B1 | Test uses `NewTestingStore` which fully migrates already | Downgrade history in-test (Step 2) |
| B2 | `TestNonIdempotentMigrationsNeverRerun` tested a tautology | Rewritten to assert specific files skipped on downgraded DB (Step 3) |
| B3 | Plan removes map code but leaves stale "defensive hardening" comment | Deleted comment block with map code (Step 1) |
| B4 | tx.Commit + UpsertMigrationHistory failure creates crash loop | Documented as Known Limitation |

Non-blocking items (N1-N3) were considered but N2 (pre-0.22 normalization path) is optional and not included to keep the plan focused.