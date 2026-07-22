# Plan Review — Bug 045: Database Migrations Silently Skipped

**Reviewer:** Senior Go Architect / Database Expert
**Date:** 2026-07-21
**Verdict:** APPROVE WITH NITS — diagnosis and high-level fix are sound; 2 code defects and 3 refinements must be addressed before implementation.

---

## 1. Diagnosis — ACCURATE

The root cause analysis is correct end-to-end:

- `GetCurrentSchemaVersion()` (`store/migrator.go:223-236`) calls `version.GetCurrentVersion(mode)` → `"0.31.0"`, then `GetMinorVersion("0.31.0")` → `"0.31"`, then globs only `migration/sqlite/0.31/*.sql`.
- The guard at line 96 (`version.IsVersionGreaterOrEqualThan(schemaVersion, fileSchemaVersion)`) fails because `"0.31.0" < "0.32.2"`.
- Both SQLite and Postgres are affected — shared migration logic in `store/migrator.go`.
- The impact scope table is correct: 4 migration files across both drivers silently skipped.
- `LATEST.sql` files are in sync — both already include the missing columns/tables (`transcript_signing_key`, `system_secret`, `max_message_length DEFAULT 2000`).

**No issues found with diagnosis.**

---

## 2. Step 1 (Rewrite `GetCurrentSchemaVersion`) — CRITICAL DEFECT

### 2a. Variable Shadowing Compile Error

The proposed code at `migrator.go:121` shadows the imported `version` package:

```go
version, err := s.getSchemaVersionOfMigrateScript(filePath)
if maxVersion == "" || version.IsVersionGreaterThan(version, maxVersion) {
```

`version` is now a `string`, not the `version` package. `version.IsVersionGreaterThan` will not compile — the method exists on the package, not on a `string` value.

**Fix:** Rename the loop variable to `fileVer`:

```go
fileVer, err := s.getSchemaVersionOfMigrateScript(filePath)
if err != nil {
    continue
}
if maxVersion == "" || version.IsVersionGreaterThan(fileVer, maxVersion) {
    maxVersion = fileVer
}
```

### 2b. LATEST.sql No Infinite Recursion — Safe, But Needs Comment

`getSchemaVersionOfMigrateScript` (line 240-241) special-cases `LATEST.sql` by calling `GetCurrentSchemaVersion()` recursively. Under the new code, the `*/*.sql` glob does NOT match `LATEST.sql` (it sits at the base path, e.g., `migration/sqlite/LATEST.sql`, not inside a `*/*.sql` subdirectory). No infinite recursion.

**Action:** Add a code comment explaining this invariant. Future maintainers will see a recursive call to `GetCurrentSchemaVersion()` from inside `GetCurrentSchemaVersion()` and reasonably assume infinite recursion without the comment.

---

## 3. Step 2 (`validateSchemaVersionConsistency`) — INCOMPLETE

### 3a. `getLatestMigrationDirectory()` Does Not Exist

The proposed code calls `s.getLatestMigrationDirectory()` which is not defined anywhere. This function must be implemented.

**Fix:** Implement it:

```go
func (s *Store) getLatestMigrationDirectory() (string, error) {
    dirs, err := fs.ReadDir(migrationFS, s.getMigrationBasePath())
    if err != nil {
        return "", errors.Wrap(err, "failed to read migration directories")
    }
    var maxDir string
    for _, d := range dirs {
        if d.IsDir() && d.Name() != "." {
            if maxDir == "" || version.IsVersionGreaterThan(d.Name(), maxDir) {
                maxDir = d.Name()
            }
        }
    }
    return maxDir, nil
}
```

### 3b. Warn-Only is Correct for Runtime — But CI Enforcement is Missing

Warn-only at runtime is the right call (the FS is the source of truth; a warning is informational). However, the plan has no mechanism to catch staleness before deployment. See **Section 8** below for the long-term prevention strategy.

---

## 4. Step 3 (Skipped Migration Warning) — CORRECT

The proposed warning at the skip site is correct and would have caught the bug immediately in logs.

**Nit:** The current code at line 96-105 does not `continue` explicitly — the `if` block just does not apply. The proposed change to add `slog.Warn` + `continue` is correct. The only logic inside the loop body is the apply path, so no other code runs for skipped files.

**No issues.**

---

## 5. Step 4 (Migration Coverage Test) — NEEDS REFINEMENT

### 5a. Would Have Caught the Bug — Confirmed

A test asserting `GetCurrentSchemaVersion() >= max(migration versions)` for each driver would fail with `schemaVersion = "0.31.0"` and `max(migration versions) = "0.33.1"`.

### 5b. Testability — Store Constructable

`getMigrationBasePath()` reads `s.profile.Driver`. The test can create two `Store` instances:

```go
store := &Store{profile: &Profile{Driver: "sqlite"}}
storePg := &Store{profile: &Profile{Driver: "postgres"}}
```

This works because `Store` is in the same package.

### 5c. No Flakiness

Migration files are embedded via `//go:embed`. Deterministic across CI runs.

### 5d. Missing: Pairwise Guard Condition Assertion

The test should also verify that for **every** migration file in `*/*.sql`, `getSchemaVersionOfMigrateScript(path) <= GetCurrentSchemaVersion()`. This directly tests the guard condition that caused the bug. The current plan only checks the aggregate max.

**Recommendation:** Add this as a second assertion.

---

## 6. Step 5 (Auto-Apply) — CORRECT

### 6a. Transaction Safety — Confirmed

SQLite DDL (ALTER TABLE, CREATE TABLE, CREATE INDEX) IS transactional within a single SQLite transaction. Postgres DDL is also transactional within a transaction block. If any statement fails, the entire batch rolls back.

### 6b. Idempotency — Confirmed

The `execute` function tolerates "duplicate column" and "already exists" errors. The `UPDATE` in `0.33/00__fix_max_message_length_default.sql` is conditionally idempotent (`WHERE max_message_length IS NULL OR max_message_length = 4000`). Safe.

### 6c. Partial Application — Not Possible

The entire migration batch runs in one transaction. Either all statements commit or all roll back. No partial state.

### 6d. Post-Commit Upsert — Pre-existing Design Choice

`UpsertMigrationHistory` runs after `tx.Commit()` (line 115-118). If this fails, the DB is at the new schema but `migration_history` shows the old version. On next startup, migrations re-run but `execute`'s idempotency handling prevents errors. This is a pre-existing design choice, not introduced by this plan. No change required.

---

## 7. Additional Codebase Findings

### 7a. `normalizedMigrationHistoryList` — Unaffected

Only runs for databases with `latestMinorVersion <= "0.22"` (line 290-292). Databases at `0.31.3` return early. No changes needed.

### 7b. `preMigrate` Path — Unaffected

Fresh databases apply `LATEST.sql` and set version to `GetCurrentSchemaVersion()`. Under the new FS-derived version, this correctly becomes `0.33.1`. No changes needed.

### 7c. `Version`/`DevVersion` Constants — Correctly Preserved

The plan preserves these for API metadata, startup banner (`bin/memos/main.go:283-301`), and profile initialization (`bin/memos/main.go:62`). The migration system simply no longer depends on them. This is the right approach.

### 7d. Version Constant Consumers — Minimal Blast Radius

| File | Usage | Impact of FS-derived schema version |
|------|-------|-------------------------------------|
| `bin/memos/main.go:62` | Profile init + startup banner | None — uses `GetCurrentVersion(mode)` directly |
| `store/migrator.go:224` | Migration engine | **Eliminated** — no longer calls `GetCurrentVersion` |
| `store/test/store.go:105` | Test profile factory | None — uses `GetCurrentVersion(mode)` directly |
| `store/test/migrator_test.go:18` | Asserts `currentSchemaVersion` contains `"0.31."` | **Must update** to not assert a specific version |

### 7e. `store/test/migrator_test.go:18` — Hardcoded Version Assertion

```go
require.Contains(t, currentSchemaVersion, "0.31.", ...)
```

This test will break once `GetCurrentSchemaVersion()` returns `"0.33.1"` from the FS. Must be updated to either assert dynamically or assert `>=` a minimum version.

### 7f. Postgres Migration Directories — Confirmed

Both drivers have `0.32/` and `0.33/` directories with the expected files. The fix applies equally.

---

## 8. Long-Term Prevention Strategy

The plan fixes the immediate bug but does not prevent recurrence. The root cause is a **single point of failure**: one manually-maintained constant (`DevVersion`/`Version`) gates all migration execution. When it drifts, migrations silently skip with no detection.

### The Recurrence Scenario

1. Developer adds `store/migration/sqlite/0.34/00__feature.sql`
2. Developer forgets to bump `DevVersion` from `"0.31.0"` to `"0.34.0"`
3. `GetCurrentSchemaVersion()` returns `"0.31.0"` (from constant)
4. All `0.32/`, `0.33/`, `0.34/` migrations silently skipped
5. Server starts successfully, SQL queries fail at runtime

### Layer 1: Eliminate the Bug Class — Auto-derive Version from FS

**Make drift impossible by removing the human step.**

Add a `go generate` code generator that auto-derives `Version`/`DevVersion` from the latest migration directory:

```
//go:generate go run ../../scripts/gen_version.go
```

The generator:
1. Scans `store/migration/{sqlite,postgres}/` for all subdirectories
2. Extracts the max minor version (e.g., `"0.33"`)
3. Reads the latest patch file in that directory
4. Computes the full schema version (e.g., `"0.33.1"`)
5. Rewrites `internal/version/version.go` with `Version = "0.33.1"` and `DevVersion = "0.33.1"`

**Result:** `Version` and `DevVersion` are always in sync with the migration filesystem because they are derived from it. A developer adds a migration directory, runs `go generate ./internal/version/`, and the constants update automatically. If they forget `go generate`, Layer 2 catches it.

### Layer 2: Build-Time Validation Gate — Catch Drift Before Runtime

**Enhance the existing `scripts/validate-migrations.sh`** to also verify version constant staleness. This script is already a dependency of `build:backend` (via `validate:migrations` task in `Taskfile.yml`), so every backend build automatically runs it.

Add to the script:

```bash
# Extract minor version from DevVersion constant
DEV_MINOR=$(grep -oP 'DevVersion\s*=\s*"\K[0-9]+\.[0-9]+' internal/version/version.go)

# Find the latest migration directory
LATEST_DIR=$(ls -d store/migration/sqlite/*/ 2>/dev/null | sort -V | tail -1 | xargs basename)

# Compare
if [ "$(printf '%s\n' "$DEV_MINOR" "$LATEST_DIR" | sort -V | head -1)" != "$LATEST_DIR" ]; then
    echo "ERROR: DevVersion minor ($DEV_MINOR) < latest migration dir ($LATEST_DIR)"
    echo "Run: go generate ./internal/version/"
    exit 1
fi
```

**Result:** `task build:backend` fails if `DevVersion` is behind the migration filesystem. No CI pipeline needed — the existing `task` infrastructure handles it.

### Layer 3: Go Test — Catch Drift at Test Time

Add a test to `store/test/` (runs via `task validate:schema`):

```go
func TestMigrationVersionConsistency(t *testing.T) {
    // For each driver (sqlite, postgres):
    //   1. Glob all migration subdirectories
    //   2. Extract the max minor version
    //   3. Compute the max full version (minor + max patch)
    //   4. Call GetCurrentSchemaVersion()
    //   5. Assert result >= max full version
    //
    // Additionally, for each migration file in the glob:
    //   Assert getSchemaVersionOfMigrateScript(path) <= GetCurrentSchemaVersion()
}
```

**Result:** `task validate:schema` catches drift. The pairwise assertion directly tests the guard condition that caused bug 045.

### Layer 4: Migration Loop Hardening — Fail Loudly in Production

Change the `slog.Warn` in Step 3 to a **hard error** when `s.profile.Mode == "prod"`:

```go
if !version.IsVersionGreaterOrEqualThan(schemaVersion, fileSchemaVersion) {
    msg := "migration file skipped: schema version too low"
    slog.Warn(msg,
        "file", filePath,
        "file_version", fileSchemaVersion,
        "schema_version", schemaVersion,
        "latest_applied", latestMigrationHistoryVersion)
    if s.profile.Mode == "prod" {
        return errors.Errorf("%s: file=%s file_version=%s schema_version=%s",
            msg, filePath, fileSchemaVersion, schemaVersion)
    }
}
```

**Result:** In production, a skipped migration causes a hard startup failure — impossible to miss. In dev mode, the warning surfaces the issue without blocking iteration.

### Layer Summary

| Layer | Mechanism | Catches | When |
|-------|-----------|---------|------|
| 1 | `go generate` auto-derive | Stale constants | Before commit (developer runs `go generate`) |
| 2 | `validate-migrations.sh` enhancement | Stale constants | Every `task build:backend` |
| 3 | Go test `TestMigrationVersionConsistency` | Stale constants + guard condition | Every `task validate:schema` |
| 4 | Hard error in prod migration loop | Skipped migrations | Server startup (last resort) |

### What I'm NOT Recommending

- **Removing `Version`/`DevVersion` constants** — They're used for profile display, startup banner, and test assertions. Not worth the refactor scope for this fix.
- **Adding a CI pipeline** — Out of scope. The existing `task` + `validate:*` infrastructure is sufficient.
- **Making the migration loop always error on skip** — In dev mode, developers may intentionally commit migration files before release. Warn-only in dev is appropriate.

---

## 9. Implementation Order (Revised)

| Step | File(s) | Change | Risk |
|------|---------|--------|------|
| 1 | `store/migrator.go` | Rewrite `GetCurrentSchemaVersion()` to derive from FS | Medium — core migration logic |
| 2 | `store/migrator.go` | Implement `getLatestMigrationDirectory()` + `validateSchemaVersionConsistency()` | Low — warn-only |
| 3 | `store/migrator.go` | Add skipped-migration warning (warn in dev, error in prod) | Low — log/error |
| 4 | `store/test/migrator_test.go` | Fix hardcoded `"0.31."` assertion to dynamic check | Low — test only |
| 5 | `scripts/gen_version.go` (new) | Code generator for `Version`/`DevVersion` from FS | Low — dev tooling |
| 6 | `internal/version/version.go` | Add `//go:generate` directive | Low — metadata only |
| 7 | `scripts/validate-migrations.sh` | Add version constant staleness check | Low — build gate |
| 8 | `store/test/` (new test) | `TestMigrationVersionConsistency` | Low — test only |
| 9 | (restart server) | Pending migrations apply automatically | Low — idempotent |

Steps 1-4 are the immediate fix. Steps 5-8 are the long-term prevention.

---

## 10. Summary of Required Changes Before Implementation

| # | Issue | Severity | Action |
|---|-------|----------|--------|
| 1 | Variable shadowing in Step 1 code | **Critical** | Rename loop var from `version` to `fileVer` |
| 2 | `getLatestMigrationDirectory()` undefined | **High** | Implement the function |
| 3 | No long-term prevention mechanism | **High** | Add Layers 1-4 (Section 8) |
| 4 | `migrator_test.go:18` hardcoded `"0.31."` assertion | **Medium** | Update to dynamic check |
| 5 | Pairwise guard condition not tested | **Medium** | Add assertion in Step 4 test |
| 6 | `LATEST.sql` recursion needs comment | **Low** | Add code comment |
| 7 | UpsertMigrationHistory outside transaction | **Low** | Pre-existing; no change needed |

---

## 11. Final Verdict

**APPROVE WITH NITS.** The plan's diagnosis is excellent, the core approach (derive schema version from FS) is the right fix, and the 5-step implementation order is sound. The variable shadowing defect (Step 1) and missing function (Step 2) must be fixed before coding. The long-term prevention strategy (Section 8) must be included to prevent recurrence.

**Estimated implementation scope:** Steps 1-9, approximately 4-6 hours of focused work across 6 files.
