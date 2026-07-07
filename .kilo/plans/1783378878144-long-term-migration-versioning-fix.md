# Long-term fix: decouple migration versioning from release version

**Status:** Implementation-ready
**Date:** 2026-07-07
**Context:** The `agent_rate_limits` FK bug (bchat tenant onboarding → "Rate limit check failed")
exposed a systemic migrator flaw. A migration placed in a directory newer than the
app's minor version (`0.30`) is **silently ignored** because
`GetCurrentSchemaVersion()` derives the schema version from `version.GetCurrentVersion()`
(app semver, `0.29.0`) and only inspects that one minor directory. The interim workaround
was to bump `Version` to `0.30.0` — a band-aid that (a) conflates release versioning with
schema versioning and (b) will bite every future migration.

## Underlying problems
1. **Schema (already correctly fixed, keep):** `agent_rate_limits.tenant_id` had a hard FK to
   `agent_tenants`, incompatible with the intentional global sentinel `tenant_id=0`. The FK was
   removed via migration `0.30/00__relax_agent_rate_limits_fk.sql` + `LATEST.sql`. This is the
   *correct* fix, not a band-aid (the band-aid would have been "skip rate-limit when tenantID<=0",
   which disables a feature). No change needed here.
2. **Migrator architecture (the real long-term fix):** Schema version must be derived from the
   **embedded migration files**, not the app release version, so any new migration directory is
   automatically discovered and applied.

## Fix (Problem 2)
In `store/migrator.go`, rewrite `GetCurrentSchemaVersion()` to return the maximum version across
**all** migration files for the active driver (embedded `migrationFS`), instead of only the
app's current minor directory.

```go
func (s *Store) GetCurrentSchemaVersion() (string, error) {
    filePaths, err := fs.Glob(migrationFS, fmt.Sprintf("%s*/*.sql", s.getMigrationBasePath()))
    if err != nil {
        return "", errors.Wrap(err, "failed to read migration files")
    }
    if len(filePaths) == 0 {
        return "", errors.New("no migration files found")
    }
    maxVersion := ""
    for _, fp := range filePaths {
        v, err := s.getSchemaVersionOfMigrateScript(fp)
        if err != nil {
            return "", errors.Wrap(err, "failed to get schema version of migrate script")
        }
        if maxVersion == "" || version.IsVersionGreaterThan(v, maxVersion) {
            maxVersion = v
        }
    }
    return maxVersion, nil
}
```

Why this is safe: `migrationFS` is embedded (`//go:embed migration` at migrator.go:21). The max
version present in the embedded FS is by definition what this binary understands, so the
"DB must not be ahead of the code" invariant still holds. The `Migrate` loop only applies files
with `version > recorded && version <= current`; with `current` = max embedded version, that
reduces to "apply every migration not yet recorded" — the correct behavior.

### Revert the interim `Version` bump
Once the above lands, the app `Version`/`DevVersion` in `internal/version/version.go` should be
reverted `0.30.0` → `0.29.0`. A schema migration is not a release; versioning stays honest and
the migration is still discovered via the embedded max. (If already deployed at 0.30.0, reverting
is harmless — `GetCurrentSchemaVersion` still resolves to `0.30.1` from the embedded `0.30` dir.)

### Convention to prevent future collisions
Always add new migrations to a **new highest** directory (`0.31`, `0.32`, …), never by inserting
files into an already-applied directory. With the fix, a fresh directory's files derive versions
(`0.31.1`, `0.31.2`, …) that are all greater than any previously recorded version, so they always
apply. The `fly:db-check` numbering check (must start at `00`, contiguous) still enforces per-dir
hygiene.

## Files touched
- `store/migrator.go` — `GetCurrentSchemaVersion()` (rewrite as above)
- `internal/version/version.go` — revert `Version`/`DevVersion` `0.30.0` → `0.29.0` (after the fix)
- `store/migration/sqlite/0.30/00__relax_agent_rate_limits_fk.sql` — keep as-is (now auto-discovered)
- `store/migration/sqlite/LATEST.sql` — already FK-less (no change)

## Validation
1. `go build ./store/...` — compiles.
2. `task fly:db-check` — Step 1 (LATEST.sql sync) and Step 2 (numbering) still pass. (Step 4 "0 tables"
   is a pre-existing validator limitation for incremental ALTER migrations and is unrelated.)
3. Deploy to a DB that already recorded `0.29.2`: on boot the `0.30` migration applies automatically
   (no `Version` bump required) and the FK is removed.
4. Regression check: a fresh DB applies `LATEST.sql` (FK-less) and records `0.30.1`; no duplicate apply.

## Out of scope
- The "disable auto RAG reindex on startup" change (separate plan).
- Removing the global `tenant_id=0` sentinel pattern (removing the FK is the correct, minimal fix;
  a separate global rate-limiter refactor is not warranted).
