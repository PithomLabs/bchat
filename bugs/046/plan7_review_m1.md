# Adversarial Plan Review — Bug 046 Plan 7

**Reviewer:** AI Architect (Senior Go / Systems / Database)  
**Plan under review:** `bugs/046/plan7.md`  
**Review date:** 2026-07-23  
**Status:** ⚠️ **NEEDS CORRECTION** (1 blocking issue, 2 non-blocking)

---

## Verdict

Plan 7 claims to incorporate adversarial review findings from Plan 6, but contains a **compilation-breaking error** in the test code. The proposed tests use `ts.driver.GetDB()` but `driver` is a private field on `Store`. Must use `ts.GetDriver().GetDB()` instead. Additionally, there are inconsistencies between the plan's stated line numbers and the actual current code.

---

## 🔴 Blocking Issues

### B1. Test code uses private field `ts.driver.GetDB()`

**Issue:** The plan proposes:
```go
_, err := ts.driver.GetDB().ExecContext(ctx, ...)
```

But `Store.driver` is a private field (lowercase `d`). The correct accessor is `ts.GetDriver().GetDB()` (returns `Driver` interface), and `Driver` has a `GetDB()` method per line 16 of `store/driver.go`.

**Mitigation:** Change all `ts.driver.GetDB()` to `ts.GetDriver().GetDB()` in both test functions.

Additionally, the plan references `ts.driver` which is:
1. Inconsistent with existing test patterns (tests use `ts.GetCurrentSchemaVersion()` etc., not raw DB access)
2. Will cause compilation failure

---

## 🟡 Non-Blocking Issues

### N1. Line number references are outdated

**Issue:** The plan states:
- "Delete lines 78-86"
- "Replace lines 113-116" 
- "Mark line 117"

Current `migrator.go` has the exact version, but the plan doesn't account for the comment block removal being part of the same edit. The actual code to change is:
- Lines 78-86: Remove entire comment block AND `appliedVersions` map
- Lines 113-116: Replace the `appliedVersions[fileSchemaVersion]` check with range check
- Line 117: Add dead-code comment annotation

The plan text is correct but the presentation could be clearer.

### N2. Missing `context` import in test

**Issue:** The test code in Step 2/3 doesn't show the `context` import, though `migrator_test.go` already imports it. This is minor but worth noting for completeness.

---

## ✅ Verified Correct

| # | Concern | Verdict |
|---|---------|---------|
| 1 | Is `!IsVersionGreaterThan(fileSchemaVersion, latestMigrationHistoryVersion)` semantically equivalent to `fileSchemaVersion <= latestMigrationHistoryVersion`? | **Yes.** `IsVersionGreaterThan` returns true only for strict greater-than. |
| 2 | Does the range check handle multiple history entries? | **Yes.** `latestMigrationHistoryVersion` is the max (line 76). Files ≤ max are skipped. |
| 3 | Does the fix retain the `IsVersionGreaterOrEqualThan(schemaVersion, fileSchemaVersion)` guard? | **Yes.** Lines 117-129 will still be in place with dead-code annotation. |
| 4 | Will a fresh database be affected? | **No.** `preMigrate` applies `LATEST.sql` and sets `migration_history` to FS version. Safe. |
| 5 | Does removing the `appliedVersions` map break anything? | **No.** The map was built from `migrationHistoryVersions` (same source as `latestMigrationHistoryVersion`). |

---

## Corrected Test Code

```go
func TestMigrationLoopSkipsAlreadyApplied(t *testing.T) {
    ctx := context.Background()
    ts := NewTestingStore(ctx, t) // DB is at current FS version, history = ["0.33.2"]

    // Simulate a database that was at 0.31.3 before Plan 5's fix surfaced the bug.
    _, err := ts.GetDriver().GetDB().ExecContext(ctx,
        "DELETE FROM migration_history; INSERT INTO migration_history (version) VALUES ('0.31.3')")
    require.NoError(t, err)

    // Re-run migration. This previously crashed on 0.14/00 because the skip logic
    // used exact-version-match, which missed all file versions not individually
    // recorded in migration_history.
    err = ts.Migrate(ctx)
    require.NoError(t, err)

    // Verify 0.32.x and 0.33.x were applied by checking that the latest history
    // entry is the FS max version.
    rows, err := ts.GetDriver().GetDB().QueryContext(ctx, "SELECT version FROM migration_history ORDER BY created_ts DESC")
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

func TestNonIdempotentMigrationsNeverRerun(t *testing.T) {
    ctx := context.Background()
    ts := NewTestingStore(ctx, t)

    // Simulate a database at 0.31.3 (the exact state that triggered taskrunrag_fail.md).
    _, err := ts.GetDriver().GetDB().ExecContext(ctx,
        "DELETE FROM migration_history; INSERT INTO migration_history (version) VALUES ('0.31.3')")
    require.NoError(t, err)

    // Re-run migration. Before the fix, this crashed with:
    //   "no such column: external_link" (from 0.14/00)
    // After the range-check fix, 0.14/00 is skipped because 0.14.1 <= 0.31.3.
    err = ts.Migrate(ctx)
    require.NoError(t, err)
}
```

---

## Summary Table

| # | Finding | Severity | Resolution |
|---|---------|----------|------------|
| B1 | Tests use `ts.driver.GetDB()` (private field) | CRITICAL | Use `ts.GetDriver().GetDB()` instead |
| N1 | Line number references are slightly off | LOW | Minor clarification needed |
| N2 | Missing context import in test snippets | LOW | Already imported in real file |

---

## Approval Conditions

**Do not start coding until B1 is corrected.** The test code must use the public accessor `ts.GetDriver().GetDB()`.