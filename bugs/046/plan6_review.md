# Adversarial Plan Review — Bug 046 Plan 6 (Migration Loop Range Check Fix)

**Reviewer:** AI Architect (Senior Go / Systems / Database)  
**Plan under review:** `plan6.md`  
**Review date:** 2026-07-23  
**Status:** ✅ APPROVED (4 blocking nits, 3 non-blocking)

---

## Verdict

**Approved for coding after fixing 4 blocking nits (B1–B4).** The core fix — replacing the exact-version-match skip with a range check against `latestMigrationHistoryVersion` — is materially correct and closes the root-cause class for Bug 046 without introducing regressions. The plan also correctly identifies that Plan 5's previously-applied fixes (`MigrationFS` export, `MIGRATE_SKIP_ERROR`, `appliedVersions` map removal) are already in the codebase and must not be re-done.

However, the two proposed new tests are designed incorrectly and will not compile or will not exercise the failure mode. One additional issue with the transaction/Upsert failure sequence must be documented before shipping.

---

## 🔴 Blocking Nits (Fix Before Coding)

### B1. `TestMigrationLoopSkipsAlreadyApplied` cannot use `NewTestingStore` as the entry point

**Issue:** The plan says:
> 1. Creates a fresh SQLite DB

`NewTestingStore` (at `store/test/store.go:24`) calls `store.Migrate(ctx)` immediately after creation (line 33). The resulting DB will already be at the current FS schema version (`0.33.2`) with `migration_history` containing `["0.33.2"]`. Calling `Migrate()` a second time would be a no-op because `schemaVersion (0.33.2) == latestMigrationHistoryVersion (0.33.2)` — the loop at line 93 would never enter.

To simulate a DB "at version 0.31.3" that has not yet applied `0.32.x` or `0.33.x`, the test must either:
- Create a `*store.Store` from a manually-prepared SQLite file (using `store.New(...)` directly, bypassing `NewTestingStore`), or
- Use `NewTestingStore` to get a fully-migrated DB, then `DELETE FROM migration_history` and `INSERT INTO migration_history (version) VALUES ('0.31.3')`, then call `Migrate()` again.

**Mitigation:** Use option (b) — it's the simplest path and reuses the test harness. Pseudocode:
```go
ts := NewTestingStore(ctx, t)
// Simulate a database that was at 0.31.3 before Plan 5's GetCurrentSchemaVersion fix
// surfaced the dormant skip-logic bug.
_, err := ts.driver.GetDB().ExecContext(ctx,
    "DELETE FROM migration_history; INSERT INTO migration_history (version) VALUES ('0.31.3')")
require.NoError(t, err)
// Re-run migration: should skip 0.14/00, execute 0.32.x + 0.33.x
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
```

### B2. `TestNonIdempotentMigrationsNeverRerun` does not test the skip logic

**Issue:** The plan says:
> 1. Lists all migration files that contain `INSERT INTO ... SELECT ... FROM` patterns (non-idempotent)
> 2. For each, verifies the migration version is ≤ the FS max version

That test validates a tautology: **every** migration file in the FS has a version ≤ the FS max version. The existing `TestAllMigrationFilesCoveredBySchemaVersion` already asserts this at `store/test/migrator_test.go:47-79`. Adding a second test that does a stricter version of the same check provides zero new coverage for the Bug 046 failure mode.

The test must instead verify that a **specific non-idempotent migration file is skipped** when the DB is at a newer version. This is the same setup as B1, but with an ASSERTION that `0.14/00__drop_resource_public_id.sql` did not execute.

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

A stronger assertion: query `sqlite_master` for a temp table that `0.14/00` would have created if executed:
```sql
SELECT name FROM sqlite_master WHERE type='table' AND name='resource_temp'
```
If 0.14/00 ran, `resource_temp` would transiently exist (it starts with `DROP TABLE IF EXISTS resource_temp`). After successful completion of 0.14/00 it's renamed to `resource`, so absence of `resource_temp` alone doesn't prove skip — but combined with `Migrate()` not erroring, the evidence is strong.

### B3. Plan must remove the stale comment block too

**Issue:** The plan says "Replace lines 78-86" but only shows removing the code block:
```go
appliedVersions := make(map[string]bool)
for _, v := range migrationHistoryVersions {
    appliedVersions[v] = true
}
```

The actual lines 78–86 in `migrator.go` include the comment block that introduces the map:
```go
// Build a set of all applied versions for O(1) lookup.
// This covers a theoretical edge case from the pre-Step-1 era (phantom versions
// could create gaps in migration_history). After Step 1, gaps cannot occur because
// GetCurrentSchemaVersion() derives from the FS, not a hardcoded constant.
// This is defensive hardening, not a fix for an active bug.
appliedVersions := make(map[string]bool)
...
```

**Mitigation:** Remove the entire comment block too. After the range-check fix, the "defensive hardening" rationale no longer exists. Leaving the comment would mislead the next reader into thinking the map is still needed.

### B4. The tx.Commit succeed + UpsertMigrationHistory failure crash-loop must be documented

**Issue:** The plan does not address the following real scenario, which becomes more dangerous with a range-check skip:

1. `migration_history` currently has `["0.31.3"]`.
2. Migration loop runs, applies `0.32.x` and `0.33.x` inside a single transaction.
3. `tx.Commit()` succeeds — the DB is now at `0.33.2`.
4. `UpsertMigrationHistory(ctx, "0.33.2")` at line 147 **fails** (network glitch, disk full, constraint violation).
5. `Migrate()` returns an error. `migration_history` still contains `["0.31.3"]`.
6. Next startup: `schemaVersion (0.33.2) > latestHistory (0.31.3)` → loop re-enters.
7. Range check: `0.32.x > 0.31.3` → execute. `0.33.x > 0.31.3` → execute.
8. `0.32.x` migrations include `ALTER TABLE … ADD COLUMN` (idempotent, tolerated).
9. If any `0.32.x` or `0.33.x` migration contains non-idempotent SQL (e.g., `DROP TABLE`, `INSERT INTO ... SELECT ... FROM`), it **crashes**. This creates a boot-loop: every restart crashes before the app becomes ready.

This is a **pre-existing issue** (exact-match map had the same blind spot — it could only skip exact versions in history, not "already applied in the last batch"). However, the plan's Expected Findings table incorrectly describes this as a "tx commit fails mid-batch" scenario, which is not the same thing.

**Mitigation:** Add this to the plan as a documented known limitation. The correct fix is either:
- Upsert `migration_history` **inside** the transaction before `tx.Commit()`, OR
- After `tx.Commit()` fails to upsert history, re-open a single-statement tx to bump history to the batch target before returning success.

Both require a Step 6 in the plan.

---

## 🟡 Non-Blocking Nits

### N1. `plan5.md` update step is the wrong artifact

**Issue:** Step 5 says:
> Update plan5.md — Add a "Known Issues" section documenting this as an additional finding from Plan 5's implementation.

Plan 5 is already implemented and committed. Retroactively editing an already-implemented plan file creates confusion for future readers who will see a plan file that references code not present at the time the plan was executed.

**Mitigation:** Document the relationship in `plan6.md` itself (under "Related findings from Plan 5") and in `CHANGELOG.md`. Delete Step 5.

### N2. `TestMigrationLoopSkipsAlreadyApplied` should also exercise the `normalizedMigrationHistoryList` path

**Issue:** For pre-0.22 databases, `preMigrate` runs `normalizedMigrationHistoryList` which inserts an additional history row. The range check must work correctly when `migration_history` contains two different minor versions (e.g., `["0.14.1", "0.14.2"]` after normalization of a pre-0.22 DB).

The current Step 2 design simulates a normalized post-0.22 database (`["0.31.3"]`). It doesn't test the pre-0.22 normalization path.

**Mitigation:** Add a sub-test or separate test case:
1. Prepare a DB at version `0.14` (history `["0.14.1"]`).
2. Call `preMigrate()` → `normalizedMigrationHistoryList` inserts `0.14.2`.
3. Verify `migration_history` now has `["0.14.1", "0.14.2"]`.
4. Call `Migrate()` with FS max `0.33.2`.
5. Verify `0.15.x` through `0.33.x` migrations are applied and `0.14/00`, `0.14/01`, `0.15/00` (etc.) are skipped.

### N3. `IsVersionGreaterOrEqualThan` guard at line 117 is now dead code

**Issue:** After the range-check fix:
```go
if !version.IsVersionGreaterThan(fileSchemaVersion, latestMigrationHistoryVersion) {
    continue  // file <= latestHistory
}
// line 117: if !version.IsVersionGreaterOrEqualThan(schemaVersion, fileSchemaVersion)
```

Since `schemaVersion` is the FS max and `fileSchemaVersion` is a file in the FS, `fileSchemaVersion <= schemaVersion` is always true. The guard at line 117 can never trigger after Plan 5's correct `GetCurrentSchemaVersion()`.

**Mitigation:** Leave it in place as a defensive assertion, but add a comment: `// Dead code after Plan 5+6: fileSchemaVersion is always <= schemaVersion (FS max). Retained as defense-in-depth.`

---

## ✅ Confirmed Correct

| # | Concern | Verdict |
|---|---------|---------|
| 1 | Is `!IsVersionGreaterThan(file, latestHistory)` semantically equivalent to `file <= latestHistory`? | **Yes.** `IsVersionGreaterThan` returns true only for strict greater-than. |
| 2 | Does the range check handle multiple history entries (e.g., `["0.14.1", "0.31.3"]`)? | **Yes.** `latestMigrationHistoryVersion` is the max. Files ≤ max are skipped. Files > max are executed. |
| 3 | Does the range check handle `normalizedMigrationHistoryList` inserting an extra entry? | **Yes.** If history becomes `["0.14.1", "0.14.2"]`, latest = `0.14.2`. Files ≤ `0.14.2` skipped, `0.15+` executed. Correct. |
| 4 | Does the fix retain the `IsVersionGreaterOrEqualThan(schemaVersion, fileSchemaVersion)` guard? | **Yes.** It's preserved at line 117. It becomes dead code but is harmless. |
| 5 | Will a fresh database be affected? | **No.** `preMigrate` applies `LATEST.sql` and inserts the FS version into `migration_history`. The loop condition `schemaVersion > latestHistory` is false. Safe. |
| 6 | Will a DB already at `0.33.2` be affected? | **No.** `0.33.2` is not `> 0.33.2`. Loop doesn't run. Safe. |
| 7 | Will a DB at `0.22.1` correctly apply `0.23.x`–`0.33.x`? | **Yes.** `0.23.1 > 0.22.1` → execute. Correct. |
| 8 | Does removing the `appliedVersions` map break anything? | **No.** The map was built from `migrationHistoryVersions` (the same source as `latestMigrationHistoryVersion`). The range check replaces all its semantics. |
| 9 | Is `MigrationFS` already exported? | **Yes.** Line 23: `var MigrationFS embed.FS`. Plan 5 B1 fix is already applied. |
| 10 | Is `MIGRATE_SKIP_ERROR` already in the code? | **Yes.** Lines 124–127. Plan 5 N1 fix is already applied. Tests in `test/migrator_test.go` can use it without a new code change. |

---

## 📋 Missing / Revised Steps

The plan should be restructured as follows:

### Step 1 (Replace Step 1): Replace exact-match with range check and remove dead comment

**File:** `store/migrator.go`

1. Delete lines 78–86 (the comment block AND the `appliedVersions` map construction).
2. Replace lines 113–116 with:

```go
// Skip migrations already applied. migration_history stores only the batch
// target version, not every individual file version. Any file whose version
// is <= the latest applied version was already executed — either during
// incremental migration or when the database was first created via LATEST.sql.
if !version.IsVersionGreaterThan(fileSchemaVersion, latestMigrationHistoryVersion) {
    continue
}
```

### Step 2 (Revised Step 2): Test `TestMigrationLoopSkipsAlreadyApplied`

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

### Step 3 (Revised Step 3): Test `TestNonIdempotentMigrationsNeverRerun`

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

### Step 4: Run full verification

```bash
go test ./store/test/... -count=1 -timeout 30s                           # all unit tests
go test ./store/test/... -count=1 -run TestMigrationLoop -v            # new tests specifically
go test ./store/test/... -count=1 -run TestNonIdempotent -v            # new non-idempotent test
task run:rag                                                            # full pipeline (reproduces taskrunrag_fail.md scenario)
```

### Step 5 (NEW): Document the UpsertMigrationHistory failure crash-loop

Add a "Known Limitations" section to `plan6.md` covering:

> **TX-COMMIT / HISTORY-UPSERT GAP:** If `tx.Commit()` succeeds at `migrator.go:140` but the subsequent `UpsertMigrationHistory` at line 147 fails, the DB schema is at the new version but `migration_history` is stale. The next startup re-enters the migration loop and re-executes all migrations in the just-applied batch. Idempotent migrations tolerate this (duplicate-column errors are swallowed). Non-idempotent migrations in that batch (e.g., `DROP TABLE`, table rebuilds) will crash. This is a pre-existing issue; the range-check fix does not introduce it but also does not resolve it. **Mitigation:** Move the history upsert inside the transaction, or perform a compensating upsert after commit failure before returning the error.

### Step 6 (DELETED): Remove the "Update plan5.md" step

Plan 5 is already executed. Document the relationship in `plan6.md` itself under a "Related findings from Plan 5" section:
> Plan 5 fixed `GetCurrentSchemaVersion()` to derive from the embedded FS. That fix caused `Migrate()` to enter the migration loop for the first time (FS max `0.33.2` > history `0.31.3`), exposing this dormant skip-logic bug. Plan 5's secondary fixes (`MigrationFS` export, `MIGRATE_SKIP_ERROR` env var, `appliedVersions` map addition) are already present in `migrator.go` and are not repeated here.

---

## Summary Table

| # | Finding | Severity | Resolution |
|---|---------|----------|------------|
| B1 | Test uses `NewTestingStore` which fully migrates already | CRITICAL | Downgrade history in-test |
| B2 | `TestNonIdempotentMigrationsNeverRerun` tests a tautology, not the skip logic | CRITICAL | Rewrite to assert specific files are skipped on a downgraded-history DB |
| B3 | Plan removes map code but leaves stale "defensive hardening" comment | CRITICAL | Delete the comment block (lines 78–82) too |
| B4 | tx.Commit + UpsertMigrationHistory failure creates a non-idempotent crash loop | CRITICAL | Add Step 5 documenting the limitation |
| N1 | Step 5 (update plan5.md) is wrong artifact | MEDIUM | Delete Step 5 |
| N2 | Tests don't exercise normalizedMigrationHistoryList path | MEDIUM | Add a sub-test with DB at 0.14 + normalization |
| N3 | `IsVersionGreaterOrEqualThan` at line 117 is dead code after fix | LOW | Add "dead code, retained as defense-in-depth" comment |

---

## Approval Conditions

**Do not start coding until B1–B4 are incorporated into the plan.** Once updated, the plan is fully executable without additional database or architectural decisions.
