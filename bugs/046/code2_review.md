# Adversarial Code Review — Bug 046 Implementation (Plan 5, Steps 0-5, 21-22)

**Reviewer:** AI Architect (Senior Go / Systems)  
**Implementation Record:** `bugs/046/code2.md`  
**Files reviewed:** `store/migrator.go`, `store/test/migrator_test.go`, `store/migration/sqlite/0.33/01__add_system_secret.sql`  
**Status:** ✅ APPROVED WITH NITS

---

## Verdict

**Approved for merge after addressing 3 nits (1 medium, 2 low).** The implementation correctly addresses all findings from plan5_review.md:

| Review Finding | Resolution | Verified |
|----------------|------------|----------|
| B1 — `migrationFS` unexported | Exported to `MigrationFS` | ✅ `store.MigrationFS` in test, `MigrationFS` in migrator.go |
| B2 — `alreadyApplied` wrong algorithm | Changed to exact map lookup | ✅ Line 114: `appliedVersions[fileSchemaVersion]` |
| B3 — `system_secret` schema mismatch | Matches LATEST.sql exactly | ✅ `encryption_salt`, `key_version`, `CHECK (id = 1)`, epoch `created_at` |
| N1 — No prod escape hatch | `MIGRATE_SKIP_ERROR` env var | ✅ Line 124: `os.Getenv("MIGRATE_SKIP_ERROR") == ""` |
| N3 — Glob ordering | `sort.Strings` added | ✅ Lines 98 and 270 |

The root cause class (version constant as migration source of truth) is eliminated.

---

## Medium Severity

### M1. `TestAllMigrationFilesCoveredBySchemaVersion` Only Tests SQLite

**File:** `store/test/migrator_test.go:56`

```go
filePaths, err := fs.Glob(store.MigrationFS, fmt.Sprintf("migration/sqlite/*/*.sql"))
```

The test hardcodes `migration/sqlite/`. It does not test Postgres migration files. If a Postgres-only migration file has a version higher than `GetCurrentSchemaVersion()` returns, the test silently passes while the Postgres path is broken.

**Risk:** Low today — Postgres has fewer migration files than SQLite (13 dirs vs 32), and the FS-derived schema version is driven by SQLite (which is the primary driver). But if Postgres ever gets a migration directory that SQLite doesn't have (similar to the existing `0.33` divergence), the schema version would be wrong for Postgres.

**Fix:** Add a parallel test that runs against Postgres glob:
```go
func TestAllPostgresMigrationFilesCoveredBySchemaVersion(t *testing.T) {
    // Identical logic but glob: "migration/postgres/*/*.sql"
}
```

Or parameterize the existing test to run for both drivers:
```go
func TestAllMigrationFilesCoveredBySchemaVersion(t *testing.T) {
    for _, driver := range []string{"sqlite", "postgres"} {
        t.Run(driver, func(t *testing.T) {
            ...
        })
    }
}
```

---

## Low Severity

### L1. No Recursion Guard in `getSchemaVersionOfMigrateScript`

**File:** `store/migrator.go:305-308`

```go
if strings.HasSuffix(filePath, LatestSchemaFileName) {
    return s.GetCurrentSchemaVersion()
}
```

If `LATEST.sql` ever enters the `*/*.sql` glob results, this would call `GetCurrentSchemaVersion()` → `getSchemaVersionOfMigrateScript()` → `GetCurrentSchemaVersion()` → infinite recursion.

**Current protection:** The `*/*.sql` pattern does not match files at the base path. This is documented in the comment at lines 258-261. But there's no runtime guard.

**Fix:** Add a recursion guard (defensive, not strictly necessary):

```go
func (s *Store) getSchemaVersionOfMigrateScript(filePath string, depth ...int) (string, error) {
    if len(depth) > 0 && depth[0] > 1 {
        return "", errors.Errorf("max recursion depth exceeded for %s", filePath)
    }
    if strings.HasSuffix(filePath, LatestSchemaFileName) {
        return s.GetCurrentSchemaVersion()
    }
    ...
}
```

Or use a `recover()` or a global flag. But given the current glob structure, this is a theoretical risk — the real fix is the comment that documents why it can't happen. Not blocking.

### L2. `validateSchemaVersionConsistency` Checks Minor Version Only

**File:** `store/migrator.go:293-297`

```go
codeMinor := version.GetMinorVersion(codeVersion)
fsMinor := version.GetMinorVersion(fsVersion)

if version.IsVersionGreaterThan(fsMinor, codeMinor) {
```

This compares `"0.33"` vs `"0.34"` — the minor version only. It does NOT detect when the patch version within the same minor has grown beyond the code version (e.g., FS = `"0.34.50"` but code = `"0.34.0"`). In that case, `IsVersionGreaterThan("0.34", "0.34")` → false, and no warning fires.

**Current protection:** The CI gate (`TestAllMigrationFilesCoveredBySchemaVersion`) catches this — the test would fail because `"0.34.50"` would not be `>=` all file versions.

**Fix:** Optional — add a patch-level check:
```go
if version.IsVersionGreaterThan(fsVersion, codeVersion) {
    slog.Warn("migration FS version is newer than code version",
        "fs_version", fsVersion, "code_version", codeVersion)
}
```

But this would fire in the normal state after a DevVersion bump (FS=`"0.33.1"`, code=`"0.34.0"` → `IsVersionGreaterThan("0.33.1", "0.34.0")` → false, correct). And in the opposite state (FS=`"0.34.1"`, code=`"0.34.0"`) → `IsVersionGreaterThan("0.34.1", "0.34.0")` → true. This would actually fire! Is this desirable? Yes — it means the FS has a migration that the code version doesn't know about. The developer should bump `Version`/`DevVersion`.

But the current code only checks minor version. Setting the threshold to warn at the patch level would produce more warnings, which is better (earlier detection). However, the current design also works — the CI gate catches patch-level issues, and the runtime check is just a best-effort secondary layer. This is a design choice, not a bug.

**Verdict:** Leave as-is. The minor-version check is sufficient for the runtime "you forgot to bump" use case. CI catches the rest.

---

## Confirmed Correct (Select Items from Adversarial Prompt)

| # | Concern | Verification |
|---|---------|-------------|
| 1 | `getSchemaVersionOfMigrateScript` on all glob paths | Correct — parses `minor/patch__desc.sql` → `"minor.patch+1"`. LATEST.sql exclusion handled (base path not in `*/*.sql`). |
| 2 | `sort.Strings` vs version sort | Irrelevant — iteration order doesn't affect `IsVersionGreaterThan` max-finding. Sort is for deterministic output only. |
| 3 | Gap blindness map lookup | Correct — exact match `appliedVersions[fileSchemaVersion]`. Comment at lines 78-82 correctly notes this is defensive hardening for pre-Step-1 era. |
| 4 | `MIGRATE_SKIP_ERROR` performance | `os.Getenv` called once per skipped file (<100 iterations). Fast path (reads cached env block). Not a concern. |
| 5 | `validateSchemaVersionConsistency` timing | Correct — runs after `preMigrate()` (which creates `migration_history` entry), before main migration loop. The warning fires when FS has directories newer than code version. |
| 6 | `system_secret` idempotency | `CREATE TABLE IF NOT EXISTS` is safe. Schema matches LATEST.sql verbatim. Verified against `store/migration/sqlite/LATEST.sql:472-478`. |
| 7 | Race condition on `MigrationFS` | `embed.FS` is immutable — all reads are safe for concurrent goroutines. |
| 8 | Version string semver edge cases | All versions are generated as `"<minor>.<patch+1>"` — strictly numeric, no pre-release suffixes. `semver.Compare` handles this correctly. |

---

## Summary

| ID | Severity | File | Line | Change |
|----|----------|------|------|--------|
| M1 | Medium | `store/test/migrator_test.go` | 56 | Add Postgres migration coverage parallel test |
| L1 | Low | `store/migrator.go` | 305-308 | Guard `LATEST.sql` recursion defensively |
| L2 | Low | `store/migrator.go` | 297 | Consider patch-level check in consistency warning |

**Ship with M1 fixed for completeness; L1 and L2 are optional and can be deferred.**
