# Bug 046: Definitive Long-Term Fix — Plan 5

**Status:** PLANNED (supersedes plans 1-4)
**Date:** 2026-07-23
**Depends on:** Bug 045 (migrator.go architectural fix)
**Affected repos:** bchat server (`/home/chaschel/Documents/go/bchat`), Hugo site (`/home/chaschel/Documents/go/izaakmaine.github.io-main`)

**Revision notes:** Incorporates findings from plan4_review.md, plan3_review.md, plan_review.md (bug 045), and adversarial edge-case analysis. This is the definitive plan — it eliminates the root cause class, adds every layer of defense, and remediates pre-existing migration bugs.

---

## Problem Statement

The same class of bug has recurred across bugs 028, 045, and 046: a manually maintained string constant (`DevVersion`/`Version` in `internal/version/version.go`) serves as the source of truth for which database migrations should run. When this constant drifts behind the migration filesystem, migrations are silently skipped. When the constant is bumped, tests with hardcoded version assertions break.

**The trap:** Fix one side, break the other, repeat.

**Current state:** `DevVersion` was bumped to `"0.34.0"` (plan4 Step 1), but `GetCurrentSchemaVersion()` still reads from the hardcoded constant (bug 045 fix not applied), and `TestGetCurrentSchemaVersion` still asserts `"0.31."`. The test fails with `"0.34.0" does not contain "0.31."`.

---

## Root Cause Analysis

### The Version Flow (What Actually Happens)

```
version.go: DevVersion = "0.34.0"
  → GetCurrentVersion("prod") → "0.34.0"
  → GetMinorVersion("0.34.0") → "0.34"
  → fs.Glob("migration/sqlite/0.34/*.sql") → [] (empty — no 0.34/ directory)
  → Returns "0.34.0" (phantom version with no corresponding migration files)
```

This phantom version `"0.34.0"` is then used as:
1. The upper bound in the `Migrate()` guard (line 96): migrations `<= "0.34.0"` are eligible
2. The recorded version in `migration_history` (line 115)
3. The stored schema version in `WorkspaceBasicSetting` (line 120)

### Why This Causes Silent Failures

When `DevVersion = "0.31.0"` (before the bump):
- `GetCurrentSchemaVersion()` returned `"0.31.4"` (from `0.31/` directory)
- The guard at line 96: `"0.31.4" >= "0.32.2"` → **false**
- Migrations in `0.32/` and `0.33/` silently skipped
- Server starts, SQL queries fail at runtime with missing columns

When `DevVersion = "0.34.0"` (current state):
- `GetCurrentSchemaVersion()` returns `"0.34.0"` (phantom — no `0.34/` dir)
- The guard at line 96: `"0.34.0" >= "0.33.1"` → **true** (migrations apply)
- But `"0.34.0"` is recorded in history — a version with no migration files
- Future `0.34/` migrations will only work when `DevVersion` is bumped past `0.34`

### The Test Breakage

`TestGetCurrentSchemaVersion` at `store/test/migrator_test.go:18`:
```go
require.Contains(t, currentSchemaVersion, "0.31.", "schema version should be 0.31.x")
```
This assertion has been updated at least three times (bugs 028, 045, 046) and will break again at every minor version boundary.

---

## Design Principles

1. **Filesystem is the single source of truth.** Migration directories on disk determine the schema version. No human step required.
2. **Fail loud.** Skipped migrations produce warnings in dev, hard errors in prod.
3. **Defense in depth.** Four layers of detection: runtime auto-derive, build-time validation, test-time assertion, startup-time guard.
4. **No sed, no code generation.** Version is derived at runtime from the embedded FS. No scripts modify `version.go`.
5. **Backward compatible.** `Version`/`DevVersion` constants remain for API metadata, startup banner, and profile display. The migration system simply ignores them.

---

## Robustness Assurance: How We Know This Is a Long-Term Fix

### The Root Cause Test

A band-aid fix addresses symptoms. A robust fix addresses the root cause. The root cause here is: **a human must manually keep a string constant in sync with the filesystem**. Every past migration bug (028, 045, 046) traces to this single point of failure.

**How we verify we're fixing the root cause, not a symptom:**

| Symptom-Fix (Band-Aid) | Root-Cause Fix (This Plan) |
|------------------------|---------------------------|
| Bump `DevVersion` to match latest dir | `GetCurrentSchemaVersion()` scans ALL dirs — no constant to bump |
| Update test assertion to `"0.34."` | Test derives expected version from actual migration directories |
| Add a CI check that `DevVersion == latest dir` | Eliminate the human step entirely — FS is the source of truth |
| Document "remember to bump version" | Four-layer defense makes forgetting impossible to deploy |

**Proof that the root cause is eliminated:** After Step 1, `GetCurrentSchemaVersion()` never calls `version.GetCurrentVersion()`. The `DevVersion`/`Version` constants are completely decoupled from migration execution. A developer can set them to anything — `"0.0.1"`, `"99.99.99"`, `""` — and migrations will still apply correctly based on what's on disk.

### Edge Case Matrix

Every scenario traced through the fixed code:

| # | Scenario | `GetCurrentSchemaVersion()` returns | `Migrate()` guard (line 76) | Migrations applied? | Version recorded | Correct? |
|---|----------|-------------------------------------|-----------------------------|--------------------|--------------------|----------|
| 1 | Fresh DB, no history | `"0.33.1"` (from `0.33/00__fix.sql`) | N/A (preMigrate handles it) | LATEST.sql only | `"0.33.1"` | **Yes** |
| 2 | DB at `"0.31.3"` | `"0.33.1"` | `"0.33.1" > "0.31.3"` → true | 0.32/01, 0.33/00 | `"0.33.1"` | **Yes** |
| 3 | DB at `"0.32.2"` | `"0.33.1"` | `"0.33.1" > "0.32.2"` → true | 0.33/00 | `"0.33.1"` | **Yes** |
| 4 | DB at `"0.33.1"` (current FS) | `"0.33.1"` | `"0.33.1" > "0.33.1"` → **false** | None | No change | **Yes** |
| 5 | DB at `"0.34.0"` (phantom) | `"0.33.1"` | `"0.33.1" > "0.34.0"` → **false** | None | No change | **Yes** |
| 6 | DB at `"0.31.3"` + gap (no 0.32.x in history) | `"0.33.1"` | `"0.33.1" > "0.31.3"` → true | 0.32/01, 0.33/00 | `"0.33.1"` | **Yes** (gap filled) |
| 7 | Future: `0.34/00__feature.sql` added | `"0.34.1"` | `"0.34.1" > latestHistory` → true | All pending up to 0.34.1 | `"0.34.1"` | **Yes** |
| 8 | Future: `0.34/` dir exists but empty | `"0.33.1"` (highest non-empty) | Depends on history | Depends on history | `"0.33.1"` | **Yes** |
| 9 | SQLite driver | Scans `migration/sqlite/*/*.sql` | Same logic | SQLite migrations | SQLite version | **Yes** |
| 10 | Postgres driver | Scans `migration/postgres/*/*.sql` | Same logic | Postgres migrations | Postgres version | **Yes** |
| 11 | Demo mode | N/A (seeds instead of migrating) | N/A | LATEST.sql + seed | N/A | **Yes** |

**Scenarios 4 and 5 are the key wins:** Under the current code, these produce phantom versions. Under the fix, they are clean no-ops.

### What Could Still Go Wrong (and Why It's Acceptable)

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Migration file has syntax error | Low | `execute()` returns error, transaction rolls back, server fails to start | Idempotency tolerance in `execute()` |
| `LATEST.sql` out of sync with incremental migrations | Medium | Fresh DB correct; upgrading DB correct via incremental | `validate-migrations.sh` CI gate |
| Two migration files with same patch number | Very low | Deterministic sort order; naming convention enforced | `create-migration.sh` validates |
| Embedded FS not rebuilt after adding migration | Medium | Binary has old migrations | Documented in guide; `go build` required |
| Postgres migration diverges from SQLite | Medium | Schema drift between drivers | `validate-parity.sh` CI gate |

---

## Implementation Plan

### Phase 1: Core Fix (store/migrator.go)

#### Step 0: Export `migrationFS` to `MigrationFS`

**File:** `store/migrator.go:22`

The embedded migration filesystem is currently unexported:
```go
//go:embed migration
var migrationFS embed.FS
```

Export it so tests in `package teststore` can access it:
```go
//go:embed migration
var MigrationFS embed.FS
```

**Why:** `migrationFS` is read-only by construction (`embed.FS`). Exporting it is safe and allows `store/test/migrator_test.go` (different package) to glob migration directories for dynamic version assertions. Update all references in `migrator.go` from `migrationFS` to `MigrationFS`.

#### Step 1: Rewrite `GetCurrentSchemaVersion()` to Scan All Directories

**File:** `store/migrator.go:223-236`

Replace the current implementation:

```go
func (s *Store) GetCurrentSchemaVersion() (string, error) {
    currentVersion := version.GetCurrentVersion(s.profile.Mode)
    minorVersion := version.GetMinorVersion(currentVersion)
    filePaths, err := fs.Glob(migrationFS, fmt.Sprintf("%s%s/*.sql", s.getMigrationBasePath(), minorVersion))
    if err != nil {
        return "", errors.Wrap(err, "failed to read migration files")
    }
    sort.Strings(filePaths)
    if len(filePaths) == 0 {
        return fmt.Sprintf("%s.0", minorVersion), nil
    }
    return s.getSchemaVersionOfMigrateScript(filePaths[len(filePaths)-1])
}
```

With:

```go
// GetCurrentSchemaVersion scans ALL migration subdirectories in the embedded FS
// to find the highest version file. The filesystem is the single source of truth.
//
// NOTE: This function does NOT call itself recursively. The */*.sql glob does NOT
// match LATEST.sql (which sits at the base path, not inside a subdirectory).
// getSchemaVersionOfMigrateScript() calls GetCurrentSchemaVersion() when it
// encounters LATEST.sql, but that path is never reached from this glob.
func (s *Store) GetCurrentSchemaVersion() (string, error) {
    filePaths, err := fs.Glob(MigrationFS, fmt.Sprintf("%s*/*.sql", s.getMigrationBasePath()))
    if err != nil {
        return "", errors.Wrap(err, "failed to glob migration files")
    }
    if len(filePaths) == 0 {
        return "", errors.Errorf("no migration files found in %s", s.getMigrationBasePath())
    }
    sort.Strings(filePaths) // fs.Glob does not guarantee sorted order

    var maxVersion string
    for _, filePath := range filePaths {
        fileVer, err := s.getSchemaVersionOfMigrateScript(filePath)
        if err != nil {
            continue // skip files that can't be parsed (e.g., LATEST.sql at base path)
        }
        if maxVersion == "" || version.IsVersionGreaterThan(fileVer, maxVersion) {
            maxVersion = fileVer
        }
    }
    if maxVersion == "" {
        return "", errors.Errorf("could not determine schema version from migration files")
    }
    return maxVersion, nil
}
```

**Behavior change:**
- Before: Returns `"0.34.0"` (from hardcoded constant, no `0.34/` dir exists)
- After: Returns `"0.33.1"` (from `0.33/00__fix_max_message_length_default.sql`)

**What this fixes:**
- Migrations in ALL directories are now eligible to run
- The recorded version in `migration_history` reflects the actual highest migration file
- No dependency on `DevVersion`/`Version` constants

**What this does NOT change:**
- `Migrate()` loop (lines 91-106) — already globs `*/*.sql`, no change needed
- `preMigrate()` fresh DB path — still applies `LATEST.sql`, but records the FS-derived version
- `Version`/`DevVersion` constants — still used for API metadata, startup banner, profile display

#### Step 2: Add Skipped Migration Warning (Warn in Dev, Error in Prod)

**File:** `store/migrator.go:91-106`

Change the migration loop from:

```go
for _, filePath := range filePaths {
    fileSchemaVersion, err := s.getSchemaVersionOfMigrateScript(filePath)
    if err != nil {
        return errors.Wrap(err, "failed to get schema version of migrate script")
    }
    if version.IsVersionGreaterThan(fileSchemaVersion, latestMigrationHistoryVersion) && version.IsVersionGreaterOrEqualThan(schemaVersion, fileSchemaVersion) {
        bytes, err := migrationFS.ReadFile(filePath)
        if err != nil {
            return errors.Wrapf(err, "failed to read minor version migration file: %s", filePath)
        }
        stmt := string(bytes)
        if err := s.execute(ctx, tx, stmt); err != nil {
            return errors.Wrapf(err, "migrate error: %s", stmt)
        }
    }
}
```

To:

```go
for _, filePath := range filePaths {
    fileSchemaVersion, err := s.getSchemaVersionOfMigrateScript(filePath)
    if err != nil {
        return errors.Wrap(err, "failed to get schema version of migrate script")
    }
    if version.IsVersionGreaterThan(fileSchemaVersion, latestMigrationHistoryVersion) {
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
            continue
        }
        bytes, err := migrationFS.ReadFile(filePath)
        if err != nil {
            return errors.Wrapf(err, "failed to read minor version migration file: %s", filePath)
        }
        stmt := string(bytes)
        if err := s.execute(ctx, tx, stmt); err != nil {
            return errors.Wrapf(err, "migrate error: %s", stmt)
        }
    }
}
```

**Behavior:**
- Dev mode: Logs `WARN` when a migration is skipped. Developer sees it in logs but server continues.
- Prod mode: Returns a hard error. Server refuses to start with a skipped migration. Impossible to deploy with missing schema.
- **Escape hatch:** Set `MIGRATE_SKIP_ERROR=1` environment variable to downgrade the prod error to a warning. Use only for emergency rollback scenarios. Documented in deployment guide.

```go
if s.profile.Mode == "prod" && os.Getenv("MIGRATE_SKIP_ERROR") == "" {
    return errors.Errorf("%s: file=%s file_version=%s schema_version=%s",
        msg, filePath, fileSchemaVersion, schemaVersion)
}
```

#### Step 3: Add Version Consistency Validation

**File:** `store/migrator.go` (new function, called from `Migrate()`)

```go
func (s *Store) validateSchemaVersionConsistency() error {
    fsVersion, err := s.GetCurrentSchemaVersion()
    if err != nil {
        return errors.Wrap(err, "failed to get FS schema version")
    }
    codeVersion := version.GetCurrentVersion(s.profile.Mode)
    codeMinor := version.GetMinorVersion(codeVersion)
    fsMinor := version.GetMinorVersion(fsVersion)

    if version.IsVersionGreaterThan(fsMinor, codeMinor) {
        slog.Warn("migration FS has directories newer than code version; bump Version/DevVersion",
            "fs_minor", fsMinor, "code_minor", codeMinor,
            "fs_version", fsVersion, "code_version", codeVersion)
    }
    return nil // warn-only at runtime; build-time gate catches this earlier
}
```

Call from `Migrate()` at the top (after `preMigrate()`):

```go
func (s *Store) Migrate(ctx context.Context) error {
    if err := s.preMigrate(ctx); err != nil {
        return errors.Wrap(err, "failed to pre-migrate")
    }

    // Validate version consistency (warn-only; build gate is the real enforcement)
    if err := s.validateSchemaVersionConsistency(); err != nil {
        return errors.Wrap(err, "failed to validate schema version consistency")
    }

    // ... rest of Migrate()
```

---

### Phase 2: Test Fix

#### Step 4: Rewrite `TestGetCurrentSchemaVersion` to Be Dynamic

**File:** `store/test/migrator_test.go`

Replace:

```go
func TestGetCurrentSchemaVersion(t *testing.T) {
    ctx := context.Background()
    ts := NewTestingStore(ctx, t)

    currentSchemaVersion, err := ts.GetCurrentSchemaVersion()
    require.NoError(t, err)
    // Schema version should start with the current minor version (0.31.x).
    // Using Contains to avoid updating test on every patch version bump
    require.Contains(t, currentSchemaVersion, "0.31.", "schema version should be 0.31.x")
}
```

With:

```go
func TestGetCurrentSchemaVersion(t *testing.T) {
    ctx := context.Background()
    ts := NewTestingStore(ctx, t)

    currentSchemaVersion, err := ts.GetCurrentSchemaVersion()
    require.NoError(t, err)
    require.NotEmpty(t, currentSchemaVersion)

    // The schema version must be a valid semver (X.Y.Z)
    parts := strings.Split(currentSchemaVersion, ".")
    require.Len(t, parts, 3, "schema version must have 3 parts: %s", currentSchemaVersion)
    for _, p := range parts {
        _, err := strconv.Atoi(p)
        require.NoError(t, err, "schema version part must be numeric: %s", currentSchemaVersion)
    }

    // The schema version must be >= the latest migration directory
    // This directly tests the guard condition that caused bugs 028, 045, 046
    migrationDirs := getMigrationDirs(t)
    if len(migrationDirs) > 0 {
        latestDir := migrationDirs[len(migrationDirs)-1]
        require.True(t,
            version.IsVersionGreaterOrEqualThan(currentSchemaVersion, latestDir+".0"),
            "schema version %s must be >= latest migration directory %s",
            currentSchemaVersion, latestDir)
    }
}
```

Add helper:

```go
func getMigrationDirs(t *testing.T) []string {
    t.Helper()
    entries, err := fs.ReadDir(MigrationFS, fmt.Sprintf("migration/%s", "sqlite"))
    require.NoError(t, err)
    var dirs []string
    for _, e := range entries {
        if e.IsDir() && regexp.MustCompile(`^[0-9]+\.[0-9]+$`).MatchString(e.Name()) {
            dirs = append(dirs, e.Name())
        }
    }
    sort.Strings(dirs)
    return dirs
}
```

Add imports: `"regexp"`, `"sort"`, `"strconv"`, `"io/fs"`, `"fmt"`, `"github.com/usememos/memos/internal/version"`.

**Why this works forever:**
- No hardcoded version string — derives expected version from the actual migration directories
- Tests the guard condition (`schemaVersion >= latestDir + ".0"`) that caused every past migration bug
- Automatically adapts when new migration directories are added

#### Step 5: Add Migration Coverage Test

**File:** `store/test/migrator_test.go` (new test)

```go
func TestAllMigrationFilesCoveredBySchemaVersion(t *testing.T) {
    ctx := context.Background()
    ts := NewTestingStore(ctx, t)

    schemaVersion, err := ts.GetCurrentSchemaVersion()
    require.NoError(t, err)

    // For EVERY migration file in the glob, its version must be <= schemaVersion
    // This directly tests the guard condition at migrator.go:96
    filePaths, err := fs.Glob(MigrationFS, fmt.Sprintf("migration/sqlite/*/*.sql"))
    require.NoError(t, err)

    for _, filePath := range filePaths {
        // Skip LATEST.sql — it's at the base path, not in a subdirectory,
        // so the */*.sql glob won't match it. But guard against future changes.
        if strings.HasSuffix(filePath, "LATEST.sql") {
            continue
        }

        // Parse version from path: migration/sqlite/0.33/00__fix.sql → "0.33.1"
        parts := strings.Split(strings.ReplaceAll(filePath, "\\", "/"), "/")
        require.True(t, len(parts) >= 3, "migration path too short: %s", filePath)
        minorVersion := parts[len(parts)-2] // "0.33"
        patchStr := strings.Split(parts[len(parts)-1], "__")[0] // "00"
        patchNum, err := strconv.Atoi(patchStr)
        require.NoError(t, err, "invalid patch number in %s", filePath)
        fileVersion := fmt.Sprintf("%s.%d", minorVersion, patchNum+1)

        require.True(t,
            version.IsVersionGreaterOrEqualThan(schemaVersion, fileVersion),
            "schema version %s must be >= migration file version %s (from %s)",
            schemaVersion, fileVersion, filePath)
    }
}
```

**What this catches:** If `GetCurrentSchemaVersion()` returns a version lower than any migration file, this test fails. This is the exact condition that caused bugs 028, 045, and 046.

---

### Phase 3: Automation (bchat server repo)

#### Step 6: Create `scripts/bump-version.sh` (Informational, Read-Only)

**File:** `scripts/bump-version.sh` (new)

**Read-only script.** Scans `store/migration/sqlite/` for the highest directory and patch file, computes the version, and prints it. Does NOT modify `version.go`.

Key logic:
- Find highest migration directory (e.g., `0.33/`)
- Find highest patch file number (e.g., `00` from `00__fix_max_message_length_default.sql`)
- Compute version: `0.33.1`
- Print the computed version
- Print the current `DevVersion` from `version.go` (for comparison)
- Exit 0 if they match, exit 1 if they differ (informational, not blocking)

**No sed, no file modification.** The script shows what `GetCurrentSchemaVersion()` would compute at runtime. If the computed version differs from `DevVersion`, the script prints a warning but does not modify files.

#### Step 7: Create `scripts/create-migration.sh` (Templates Only)

**File:** `scripts/create-migration.sh` (new)

Creates SQLite and Postgres migration files with TODO templates. **No auto-generation.** Developer writes SQL for each driver.

Key properties:
- Validates migration name (snake_case)
- Creates SQLite file with comment header and TODO placeholder
- Creates Postgres file with comment header and TODO placeholder
- Calls `bump-version.sh` informationally (prints computed version)
- Supports `--dry-run`

**Why no auto-generation:** SQLite→Postgres translation involves DDL transactionality differences, INSERT OR REPLACE semantics, type affinity, generated columns, and RETURNING clauses. A developer who trusts auto-generated output will deploy broken Postgres migrations.

#### Step 8: Create `docs/TYPE_MAPPING.md`

**File:** `docs/TYPE_MAPPING.md` (new)

Explicit SQLite-Postgres type mapping reference covering:
- Type mapping table (BLOB→BYTEA, SERIAL, BOOLEAN, JSONB, TIMESTAMPTZ, etc.)
- Syntax differences (quoting, INSERT OR IGNORE, timestamp functions, reserved words)
- Migration writing rules for each driver
- Review checklist for manually-written Postgres migrations
- SQL parsing limitations disclaimer

#### Step 9: Create `scripts/validate-parity.sh`

**File:** `scripts/validate-parity.sh` (new)

Cross-driver parity validator. Three checks:

**Check 1 — File-list parity (CI gate):** Compares migration files in corresponding `sqlite/<dir>/` and `postgres/<dir>/` directories. Every file in `sqlite/<dir>/` must have a corresponding `postgres/<dir>/` file. Missing files fail the build.

**Check 2 — Schema parity (best-effort lint):** Parses both `LATEST.sql` files, extracts table names, column names per table, and index names, then compares them. Warns on differences (does not fail — shell SQL parsing is unreliable).

**Check 3 — Historical divergence documentation:** Lists known divergences and skips those from comparison.

#### Step 10: Create Script Test Fixtures

**File:** `scripts/test/` (new directory)

Test fixtures for automation scripts:
- `version.go.fixture` — known-good `version.go` with specific version values
- `migration-tree/` — fake migration directory tree with known structure
- `run-tests.sh` — test runner that validates each script

**Taskfile task:**
```yaml
  test:scripts:
    desc: Run automation script tests
    cmds:
      - ./scripts/test/run-tests.sh
```

#### Step 11: Add Taskfile Tasks and CI Gating

**File:** `Taskfile.yml`

```yaml
  version:info:
    desc: Show computed version from migration filesystem (informational, read-only)
    cmds:
      - ./scripts/bump-version.sh

  migrate:new:
    desc: Create new migration file templates for both drivers (usage: task migrate:new NAME=add_widget_config)
    cmds:
      - ./scripts/create-migration.sh "{{.NAME}}"

  validate:parity:
    desc: Validate SQLite and Postgres migration parity
    cmds:
      - ./scripts/validate-parity.sh

  test:scripts:
    desc: Run automation script tests
    cmds:
      - ./scripts/test/run-tests.sh
```

**CI gating:** Add `validate:parity` and `test:scripts` as dependencies of `build:backend`:

```yaml
  build:backend:
    desc: Build backend binary
    deps: [validate:migrations, validate:parity, test:scripts]
```

---

### Phase 4: Hugo Site Fix

#### Step 12: Add `bchat.baseUrl` to `hugo.yaml`

**File:** `/home/chaschel/Documents/go/izaakmaine.github.io-main/hugo.yaml`

Add under the existing `params:` block:

```yaml
params:
  # ... existing params ...

  bchat:
    # Base URL of the bchat server for the chat widget.
    # Override per-environment via config/ directory or environment variables.
    # Default: localhost for local dev (hugo server), set explicitly for production.
    baseUrl: "http://localhost:8081"
```

#### Step 13: Update Landing Page Template

**File:** `/home/chaschel/Documents/go/izaakmaine.github.io-main/layouts/_default/list.html`

Change line 225 from:
```go
{{ $chatUrl := .Params.chatBaseUrl | default "https://bchat-pg.fly.dev" }}
```

To:
```go
{{ $chatUrl := or .Params.chatBaseUrl site.Params.bchat.baseUrl "http://localhost:8081" }}
{{ if not site.Params.bchat.baseUrl }}
  {{ warnf "WARNING: bchat.baseUrl not set in site params. Widget will use localhost fallback." }}
{{ end }}
```

**Why `or` instead of `default`:** Hugo's `default` checks for `nil` (undefined). An empty string `""` is defined and non-nil, so `default` will NOT fall through. `or` returns the first non-zero value (non-nil, non-empty, non-false), handling both `nil` and `""` correctly.

#### Step 14-16: Remove Hardcoded `chatBaseUrl` from Content Files

Remove `chatBaseUrl: "http://localhost:8081"` from:
- `content/rgresidences/_index.md`
- `content/bchat/_index.md`
- `content/evpn/_index.md`

#### Step 17: Hugo Deploy CI Gate

Add a grep on Hugo build output to fail production builds missing `bchat.baseUrl`:
```bash
hugo --environment production 2>&1 | tee hugo-build.log
if grep -q "bchat.baseUrl not set" hugo-build.log; then
  echo "FATAL: Production build missing bchat.baseUrl"
  exit 1
fi
```

---

### Phase 5: Documentation

#### Step 18: Update AGENTS.md Migration Section

**File:** `AGENTS.md`

Replace the "Database Migrations" section with a quick reference pointing to the new guide, documenting `task migrate:new` as the primary workflow, and listing validation commands.

#### Step 19: Deprecate Old Migration Doc

**File:** `docs/DOCS_DATABASE_MIGRATION.MD`

Add deprecation notice at the top pointing to `DOCS_DATABASE_MIGRATION_GUIDE.md`.

#### Step 20: Create Definitive Migration Guide

**File:** `docs/DOCS_DATABASE_MIGRATION_GUIDE.md` (new)

Comprehensive guide covering:
- Architecture (directory structure, version detection from FS, migration flow)
- Quick reference (all commands)
- Step-by-step: adding a new migration (`task migrate:new`)
- Rules and conventions
- Cross-driver parity (three levels, type mapping, manual writing)
- **CI gates matrix** (which gate catches which historical bug):

| Historical Bug | CI Gate That Would Catch It |
|----------------|---------------------------|
| 008 — Unique constraint failure | `validate-pg-migrations.sh` |
| 009 — Migration 28 hotfix | `validate-db-migrations.sh` |
| 045 — Version directory skip | `version:info` + `TestAllMigrationFilesCoveredBySchemaVersion` |
| 046 — LATEST.sql drift | `validate:parity` + `validate:migrations.sh` |

- Rollback contract
- Gotchas and known issues
- Testing checklist
- Deployment checklist
- Troubleshooting
- Historical context

---

### Phase 6: Pre-Existing Bug Remediation

These bugs exist TODAY and are NOT caused by the version constant issue. They must be fixed separately.

#### Step 21: Fix Gap Blindness in Migration Guard

**File:** `store/migrator.go:91-106`

**Problem:** The guard at line 96 compares `fileSchemaVersion` against only the **maximum** version in `migration_history`, not against all versions. If history has a gap (e.g., `0.31.3` and `0.33.1` but no `0.32.x`), the `0.32` migration is permanently skipped.

**Current guard:**
```go
if version.IsVersionGreaterThan(fileSchemaVersion, latestMigrationHistoryVersion)
   && version.IsVersionGreaterOrEqualThan(schemaVersion, fileSchemaVersion)
```

**Fix:** Check against ALL versions in migration history using a map lookup (O(1)):

```go
// Build a set of all applied versions for O(1) lookup.
// This covers a theoretical edge case from the pre-Step-1 era (phantom versions
// could create gaps in migration_history). After Step 1, gaps cannot occur because
// GetCurrentSchemaVersion() derives from the FS, not a hardcoded constant.
// This is defensive hardening, not a fix for an active bug.
appliedVersions := make(map[string]bool)
for _, v := range migrationHistoryVersions {
    appliedVersions[v] = true
}

for _, filePath := range filePaths {
    fileSchemaVersion, err := s.getSchemaVersionOfMigrateScript(filePath)
    if err != nil {
        return errors.Wrap(err, "failed to get schema version of migrate script")
    }
    // Skip if this exact version was already applied
    if appliedVersions[fileSchemaVersion] {
        continue
    }
    if version.IsVersionGreaterOrEqualThan(schemaVersion, fileSchemaVersion) {
        // apply migration
    }
}
```

**Risk:** Low — defensive hardening with simple map lookup. No behavioral change for well-ordered databases.

#### Step 22: Add Missing SQLite `system_secret` Migration

**File:** `store/migration/sqlite/0.33/01__add_system_secret.sql` (new)

**Problem:** The `system_secret` table is defined in SQLite's `LATEST.sql` (line 472-478) but has NO corresponding incremental migration for SQLite. Fresh SQLite installs get it from `LATEST.sql`, but upgrading SQLite databases from < 0.33 to >= 0.33 will never create this table.

```sql
-- Migration: add_system_secret
-- Driver: sqlite
-- Date: 2026-07-23
-- Bug: 046
--
-- Adds system_secret table for encryption salt storage.
-- Already present in LATEST.sql for fresh installs.
-- Schema must match LATEST.sql exactly (not a simplified version).

CREATE TABLE IF NOT EXISTS system_secret (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    encryption_salt BLOB NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    rotated_at BIGINT
);
```

**Note:** The schema must match LATEST.sql verbatim — `encryption_salt` (not `salt`), `CHECK (id = 1)`, `key_version`, `BIGINT` timestamps, and `rotated_at`. This migration is a no-op for fresh installs (table already exists from LATEST.sql) and a create for incremental upgrades.

**Risk:** Low — additive DDL, idempotent with `IF NOT EXISTS`.

#### Step 23: Audit and Complete Postgres Incremental Migrations

**Problem:** Postgres `LATEST.sql` defines ~30 tables, but only ~15 have incremental migrations. An incrementally-upgraded Postgres database is missing `agent_workflows`, `agent_simulations`, `agent_analysis_results`, `agent_compliance_audits`, `agent_scoring_config`, `agent_qa_pairs`, `agent_reindex_checkpoints`, `agent_rag_active_versions`, and more.

**Approach:**
1. Audit which Postgres tables in `LATEST.sql` have no incremental migration
2. For each, determine if it was introduced via `LATEST.sql` only (fresh installs) or needs incremental support
3. Create migration files for tables that need incremental support
4. Document tables that are "fresh-install only" in the migration guide
5. Add CI check that every table in `LATEST.sql` has a corresponding incremental migration path

**Risk:** High — requires careful analysis of which tables are actually needed for incremental upgrades vs. fresh installs only.

#### Step 24: Document "No Retroactive Migrations" Rule

**File:** `docs/DOCS_DATABASE_MIGRATION_GUIDE.md`

**Problem:** Adding a new file to an older minor version directory (e.g., `0.31/03__foo.sql`) will never be applied to any database that has progressed past 0.31.x in `migration_history`. This is a design limitation.

**Mitigation:** Document in the migration guide:

```markdown
### Never Add Retroactive Migrations

Adding a new file to an existing minor version directory (e.g., adding `03__foo.sql` to `0.31/`) is
NOT supported. The migration guard compares against the maximum version in `migration_history`, so
any file in an older directory will be skipped for databases that have already progressed past that version.

**Rule:** Always create a new minor version directory for new migrations.
```

**Risk:** Low — documentation only.

---

## Implementation Order

| Step | File(s) | Change | Depends on | Risk |
|------|---------|--------|-----------|------|
| 0 | `store/migrator.go` | Export `migrationFS` to `MigrationFS` | — | Low |
| 1 | `store/migrator.go` | Rewrite `GetCurrentSchemaVersion()` to scan all FS dirs | 0 | Medium |
| 2 | `store/migrator.go` | Add skipped migration warning (warn dev, error prod) + `MIGRATE_SKIP_ERROR` env var | 0 | Low |
| 3 | `store/migrator.go` | Add `validateSchemaVersionConsistency()` | 1 | Low |
| 4 | `store/test/migrator_test.go` | Rewrite test to be dynamic (no hardcoded version) | 1 | Low |
| 5 | `store/test/migrator_test.go` | Add `TestAllMigrationFilesCoveredBySchemaVersion` | 1 | Low |
| 6 | `scripts/bump-version.sh` | Informational version script (read-only) | — | Low |
| 7 | `scripts/create-migration.sh` | Migration templates (no auto-gen) | 6 | Low |
| 8 | `docs/TYPE_MAPPING.md` | Type mapping reference | — | Low |
| 9 | `scripts/validate-parity.sh` | Cross-driver parity validator | 8 | Low |
| 10 | `scripts/test/` | Script test fixtures + runner | 6, 7, 9 | Low |
| 11 | `Taskfile.yml` | Add tasks + CI gating | 6, 7, 9, 10 | Low |
| 12 | Hugo `hugo.yaml` | Add bchat.baseUrl param | — | Low |
| 13 | Hugo `layouts/_default/list.html` | 3-tier fallback with `or` | 12 | Low |
| 14-16 | Hugo content files | Remove chatBaseUrl | 13 | Low |
| 17 | Deploy pipeline | Hugo CI gate on missing bchat.baseUrl | 13 | Low |
| 18 | `AGENTS.md` | Update migration section | 20 | Low |
| 19 | `docs/DOCS_DATABASE_MIGRATION.MD` | Deprecation notice | — | Low |
| 20 | `docs/DOCS_DATABASE_MIGRATION_GUIDE.md` | Definitive guide | 6, 7, 8, 9, 10, 11 | Low |
| 21 | `store/migrator.go` | Fix gap blindness — map lookup (Bug A, defensive) | 1 | Low |
| 22 | `store/migration/sqlite/0.33/01__add_system_secret.sql` | Add missing SQLite migration (Bug B) | — | Low |
| 23 | `store/migration/postgres/` | Audit + add missing Postgres migrations (Bug C) | — | High |
| 24 | `docs/DOCS_DATABASE_MIGRATION_GUIDE.md` | Document "no retroactive migrations" rule (Bug D) | 20 | Low |

Steps 1-5 fix the root cause. Steps 6-17 prevent recurrence and fix the widget. Steps 18-20 document the process. Steps 21-24 fix pre-existing bugs.

**Steps 1-5 and 12-17 are independent** — they can be implemented in parallel.

---

## What Changes in Behavior

### Before (Current State)

| Component | Behavior |
|-----------|----------|
| `migrationFS` | Unexported — tests in `package teststore` cannot access it |
| `GetCurrentSchemaVersion()` | Reads `DevVersion` constant, scans only that minor version directory |
| Migration guard | Uses hardcoded version as upper bound — skips newer directories |
| Test | Hardcoded `"0.31."` assertion — breaks at every minor version bump |
| Version recording | Records phantom version (e.g., `"0.34.0"`) with no corresponding migration files |
| Hugo widget | Hardcoded `chatBaseUrl: "http://localhost:8081"` in every content file |
| Gap handling | Gap in migration_history permanently loses migrations |
| Skipped migrations | Silent — no log, no error |

### After (Plan 5)

| Component | Behavior |
|-----------|----------|
| `MigrationFS` | Exported — tests can glob migration directories for dynamic assertions |
| `GetCurrentSchemaVersion()` | Scans ALL `*/*.sql` files, returns highest version found |
| Migration guard | Uses FS-derived version as upper bound — all directories are eligible |
| Test | Dynamic assertion: version >= latest migration directory, valid semver |
| Version recording | Records actual highest migration version (e.g., `"0.33.1"`) |
| Hugo widget | Configurable via `hugo.yaml` params, 3-tier fallback, localhost default |
| Skipped migrations | Logged as WARN (dev) or hard error (prod), with `MIGRATE_SKIP_ERROR` escape hatch |
| Version recording | Records actual highest migration version (e.g., `"0.33.1"`) |
| Hugo widget | Configurable via `hugo.yaml` params, 3-tier fallback, localhost default |
| Skipped migrations | Logged as WARN (dev) or hard error (prod) |
| Version drift | Detected by `validateSchemaVersionConsistency()` (warn), `validate-parity.sh` (CI gate), and `TestAllMigrationFilesCoveredBySchemaVersion` (test) |
| Gap handling | Gap blindness fixed — all applied versions checked, not just max |
| SQLite system_secret | Added to incremental migration path |
| Postgres incremental path | Audited and completed |

---

## Verification

### Immediate (After Steps 1-5)

1. `go test ./store/test/... -run TestGetCurrentSchemaVersion` — passes with dynamic assertion
2. `go test ./store/test/... -run TestAllMigrationFilesCoveredBySchemaVersion` — passes
3. `go test ./store/test/... -run TestMigrationHistoryVersion` — passes
4. `go test ./store/test/... -run TestSchemaValidation` — passes
5. Server startup logs show `"start migration"` → `"end migrate"` with correct version

### After Steps 6-11

6. `task version:info` — prints `"0.33.1"` (computed from FS)
7. `task migrate:new NAME=test_migration` — creates both files with TODO templates
8. `task validate:parity` — passes with current LATEST.sql files
9. `task test:scripts` — all fixtures pass
10. `go build ./bin/memos/` — succeeds

### After Steps 12-17

11. `hugo server` — loads rgresidences page, widget loads from `localhost:8081` (from `hugo.yaml` param)
12. Template falls through correctly when `chatBaseUrl` is omitted from front matter
13. `warnf` fires when `bchat.baseUrl` is not set in site params

### After Steps 21-24 (Pre-Existing Bug Fixes)

14. Database with gap in migration_history (e.g., 0.31.3 + 0.33.1, no 0.32.x) — 0.32 migration applied on next startup
15. SQLite upgrading from < 0.33 — `system_secret` table created
16. Postgres upgrading from < 0.33 — all missing tables created (after Step 23 audit)

### Full Widget Test (After All Steps + Bug 045 Deployed)

17. `curl http://localhost:8081/widget/rgresidences/embed.js` — returns JS
18. `PRAGMA table_info(agent_tenants)` — shows `transcript_signing_key` columns
19. Widget appears in browser on rgresidences landing page

---

## Four-Layer Defense (Prevents Recurrence)

| Layer | Mechanism | Catches | When |
|-------|-----------|---------|------|
| 1 | `GetCurrentSchemaVersion()` scans all FS dirs | Stale version constant | Runtime (startup) |
| 2 | `validate-parity.sh` + `validate-migrations.sh` | LATEST.sql drift, missing files | Every `task build:backend` |
| 3 | `TestAllMigrationFilesCoveredBySchemaVersion` | Version < any migration file | Every `task validate:schema` |
| 4 | Skipped migration warning/error | Migrations not applied | Server startup (last resort) |

---

## Files Changed Summary

| File | Change | Phase |
|------|--------|-------|
| `store/migrator.go` | Export `MigrationFS`, rewrite `GetCurrentSchemaVersion()`, add warning + `MIGRATE_SKIP_ERROR`, add validation, fix gap blindness | 1, 6 |
| `store/test/migrator_test.go` | Dynamic test, coverage test | 2 |
| `scripts/bump-version.sh` | New — informational version script | 3 |
| `scripts/create-migration.sh` | New — migration templates | 3 |
| `scripts/validate-parity.sh` | New — cross-driver parity | 3 |
| `scripts/test/` | New — script test fixtures | 3 |
| `Taskfile.yml` | Add tasks, CI gating | 3 |
| `docs/TYPE_MAPPING.md` | New — type mapping reference | 3 |
| `docs/DOCS_DATABASE_MIGRATION_GUIDE.md` | New — definitive guide | 5 |
| `docs/DOCS_DATABASE_MIGRATION.MD` | Add deprecation notice | 5 |
| `AGENTS.md` | Update migration section | 5 |
| Hugo `hugo.yaml` | Add `bchat.baseUrl` param | 4 |
| Hugo `layouts/_default/list.html` | 3-tier fallback with `or` | 4 |
| Hugo content files (3) | Remove `chatBaseUrl` | 4 |
| `store/migration/sqlite/0.33/01__add_system_secret.sql` | New — missing SQLite migration | 6 |
| `store/migration/postgres/` | Audit + add missing migrations | 6 |

**Total: 16 files modified or created.**

---

## Adversarial Review Prompt

> Review this plan for Bug 046 (definitive long-term fix) with an adversarial mindset. Focus on:
>
> 1. **Step 1 correctness:** The new `GetCurrentSchemaVersion()` scans `*/*.sql`. Does this glob match `LATEST.sql` at the base path? (It should NOT — `LATEST.sql` is at `migration/sqlite/LATEST.sql`, not `migration/sqlite/X.Y/LATEST.sql`.) Confirm the recursive call in `getSchemaVersionOfMigrateScript` cannot cause infinite recursion.
> 2. **Edge case — empty migration directories:** What if `0.34/` exists but has no `.sql` files? The glob returns empty for that directory, but other directories have files. Is this handled correctly?
> 3. **Edge case — future migrations:** If someone commits `0.34/00__feature.sql` before release, `GetCurrentSchemaVersion()` returns `"0.34.1"`. The `Migrate()` guard applies all migrations up to `0.34.1`. Is this desirable? Or should there be an explicit "enable" mechanism?
> 4. **Step 2 — prod hard error:** In prod mode, a skipped migration returns a hard error. But what if the skip is intentional (e.g., a migration was rolled back)? Should there be an escape hatch?
> 5. **Step 4 — test imports:** The rewritten test needs `regexp`, `sort`, `strconv`, `io/fs`, `fmt`, and the `version` package. Are these already imported in the test package? Check `store/test/store.go` for existing imports.
> 6. **Step 5 — unexported method:** `getSchemaVersionOfMigrateScript` is unexported. The test in Step 5 parses the version from the path manually instead. Is this fragile? Should the method be exported?
> 7. **Step 9 — shell SQL parsing:** The parity validator uses awk/grep to parse SQL. Document the known false positives/negatives. Is this good enough for a CI gate?
> 8. **Step 13 — Hugo `or` vs `default`:** Confirm Hugo's `or` treats empty string `""` as falsy. Test with `chatBaseUrl: ""` in front matter.
> 9. **Concurrency — bump-version.sh:** If two developers run `task migrate:new` simultaneously, they get the same patch number. Is this acceptable for a solo/small-team project?
> 10. **Backward compatibility:** After Step 1, existing databases with `migration_history` showing `"0.34.0"` (phantom version) will have `GetCurrentSchemaVersion()` return `"0.33.1"`. The guard at line 76: `"0.33.1" > "0.34.0"` → false. Migrations won't re-run. Is this correct? What if the database is actually at `0.31.3` (the original phantom)?
> 11. **Step 21 — gap blindness fix:** The proposed fix checks ALL versions in migration_history. But what if the list is very large (thousands of versions)? Is the O(n*m) complexity acceptable? Should we use a more efficient data structure?
> 12. **Step 23 — Postgres audit scope:** How many tables are actually missing from the Postgres incremental path? Is this a 2-hour task or a 2-day task? Should we scope this more precisely before committing?
> 13. **Step 22 — SQLite system_secret idempotency:** The `CREATE TABLE IF NOT EXISTS` is idempotent, but what if the table already exists with a different schema? Should we add a schema version check?
>
> Provide specific recommendations to strengthen the plan.
