# Code Review — Bug 046 Implementation (Plan 5, Steps 0-5, 21-22)

**Date:** 2026-07-23
**Status:** IMPLEMENTED
**Branch:** (uncommitted changes)

---

## Summary

Implemented the core migrator fix (root cause elimination), tests, gap blindness hardening, and missing SQLite migration from plan5.md. This eliminates the hardcoded version constant as the source of truth for migration execution.

## Files Changed

### `store/migrator.go`

| Change | Lines | Description |
|--------|-------|-------------|
| Export `migrationFS` → `MigrationFS` | 22 | Allows test package to access embedded FS for dynamic assertions |
| Rewrite `GetCurrentSchemaVersion()` | 241-273 | Scans ALL `*/*.sql` dirs, returns highest version (was: only scanned `DevVersion` dir) |
| Add `sort.Strings` after glob | 257 | `fs.Glob` doesn't guarantee sorted order — defensive |
| Add `validateSchemaVersionConsistency()` | 275-288 | Warns if FS dirs are newer than `Version`/`DevVersion` constants |
| Add skipped migration warning | 117-128 | WARN in dev, hard error in prod (with `MIGRATE_SKIP_ERROR` escape hatch) |
| Add gap blindness map lookup | 80-85, 113-115 | Checks ALL applied versions via map, not just max — defensive hardening |
| Add `"os"` import | 9 | For `os.Getenv("MIGRATE_SKIP_ERROR")` |

### `store/test/migrator_test.go`

| Change | Description |
|--------|-------------|
| Rewrite `TestGetCurrentSchemaVersion` | Dynamic: asserts version is valid semver and >= latest migration dir (was: hardcoded `"0.31."`) |
| Add `TestAllMigrationFilesCoveredBySchemaVersion` | Asserts every migration file version <= `GetCurrentSchemaVersion()` — directly tests guard condition |
| Add `getMigrationDirs` helper | Reads `MigrationFS` to list migration directories dynamically |
| New imports | `fmt`, `io/fs`, `regexp`, `sort`, `strconv`, `strings`, `version`, `store` |

### `store/migration/sqlite/0.33/01__add_system_secret.sql` (new)

Adds the `system_secret` table to SQLite's incremental migration path. Schema matches `LATEST.sql` exactly:
- `id INTEGER PRIMARY KEY CHECK (id = 1)`
- `encryption_salt BLOB NOT NULL`
- `key_version INTEGER NOT NULL DEFAULT 1`
- `created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now'))`
- `rotated_at BIGINT`

## Test Results

```
TestGetCurrentSchemaVersion                                  PASS
TestAllMigrationFilesCoveredBySchemaVersion                  PASS
TestSchemaValidation                                        PASS
TestMigrationHistoryVersion                                 PASS (reports 0.33.2)
Full store test suite                                       PASS
Binary build                                                PASS
```

## Behavior Changes

| Before | After |
|--------|-------|
| `GetCurrentSchemaVersion()` reads `DevVersion`, scans one dir | Scans ALL dirs, returns highest version |
| `DevVersion = "0.34.0"` → phantom version recorded | `"0.33.2"` recorded (actual highest migration) |
| Test asserts `"0.31."` → breaks every minor bump | Dynamic assertion → never breaks |
| Skipped migrations silent | WARN (dev) / error (prod) with escape hatch |
| Gap in history permanently loses migrations | Map lookup skips already-applied versions |
| `system_secret` missing from SQLite incremental path | Added via `0.33/01__add_system_secret.sql` |

## Adversarial Code Review Prompt

> Review this implementation with an adversarial mindset. Focus on:
>
> 1. **Correctness of `GetCurrentSchemaVersion()`**: The new implementation globs `*/*.sql` and iterates to find the max version. Does `getSchemaVersionOfMigrateScript()` correctly parse all file paths in the glob? What about files at unexpected paths (e.g., nested subdirectories)?
>
> 2. **`sort.Strings` ordering**: The new code sorts file paths alphabetically before iterating. But `getSchemaVersionOfMigrateScript()` computes version from the path (directory name + patch number). Does alphabetical sort of full paths produce the same order as version sort? What if `0.33/00__foo.sql` sorts before `0.32/01__bar.sql`?
>
> 3. **Gap blindness map lookup**: The `appliedVersions` map is built from `migrationHistoryVersions`. But `migrationHistoryVersions` is sorted by `version.SortVersion`. If the map contains `"0.33.1"` but a file computes to `"0.32.2"`, the map lookup `appliedVersions["0.32.2"]` returns false, and the file is applied. Is this correct? What if the file was already applied but the version string differs slightly?
>
> 4. **`MIGRATE_SKIP_ERROR` env var**: The env var is checked via `os.Getenv` on every skipped migration. Is this a performance concern? Should it be read once at startup? What if the env var is set mid-migration (impossible in practice, but worth noting)?
>
> 5. **`validateSchemaVersionConsistency()` timing**: This runs after `preMigrate()` but before the main migration loop. If `preMigrate()` applies `LATEST.sql` and records a version, then `validateSchemaVersionConsistency()` warns about FS vs code mismatch — is this warning accurate? Or does it fire before the migration loop has a chance to update the version?
>
> 6. **`system_secret` migration idempotency**: `CREATE TABLE IF NOT EXISTS` is idempotent. But what if a fresh install already has the table from `LATEST.sql`? The migration runs but is a no-op. Is there any risk of the `IF NOT EXISTS` silently swallowing a schema mismatch (e.g., column added to LATEST.sql but not to this migration)?
>
> 7. **Test coverage**: `TestAllMigrationFilesCoveredBySchemaVersion` only tests SQLite. Should it also test Postgres? The `MigrationFS` is shared — is there a risk that Postgres-only migrations (e.g., `0.33/00__add_system_secret.sql` in postgres/) are not covered?
>
> 8. **`getSchemaVersionOfMigrateScript` recursion**: When the glob encounters a `LATEST.sql` at the base path, `getSchemaVersionOfMigrateScript` calls `GetCurrentSchemaVersion()` recursively. The new `GetCurrentSchemaVersion()` uses `*/*.sql` which shouldn't match `LATEST.sql`. But what if someone moves `LATEST.sql` into a subdirectory? Is there a guard against infinite recursion?
>
> 9. **Race condition on `MigrationFS`**: `MigrationFS` is a global `embed.FS`. Multiple goroutines calling `GetCurrentSchemaVersion()` concurrently — is `fs.Glob` safe for concurrent use? (It should be, since `embed.FS` is immutable, but verify.)
>
> 10. **Version string comparison edge cases**: `version.IsVersionGreaterThan` uses `semver.Compare`. Does it handle pre-release versions (e.g., `0.33.1-beta.1`)? What if a migration file has a non-numeric patch number?
>
> Provide specific findings with file:line references.
