# Bug 045: Database Migrations Silently Skipped Due to Hardcoded Version Constant

## Detailed Diagnosis

### Symptom
`GET /api/v1/agent/tenants` returns `500 Internal Server Error` with message `"Failed to list tenants"`. The AgentAdmin UI shows "no tenants" because the error is silently swallowed (no error display in the tenant list view).

### Reproduction
1. Server is started with `task run:rag` (mode=dev, driver=sqlite)
2. Frontend calls `GET /api/v1/agent/tenants` with valid JWT from `memos.access-token` cookie
3. Auth middleware passes (JWT valid, user is HOST, token exists in `user_setting`)
4. `isSuperAdmin(h)` returns `true` (HOST user)
5. `ListAgentTenants` in `store/db/sqlite/agent.go:68-73` executes:
   ```sql
   SELECT id, slug, company_name, guid, widget_key, vertical, is_active,
          processing_options, allowed_domains,
          transcript_signing_key, transcript_signing_key_nonce,
          created_at, updated_at
   FROM agent_tenants
   ```
6. SQLite returns an error: columns `transcript_signing_key` and `transcript_signing_key_nonce` do **not** exist in the table
7. Error propagates to `handlers.go:672` → 500 response

### Why the Columns Are Missing
The columns are defined in migration `store/migration/sqlite/0.32/01__transcript_signing_key.sql`:
```sql
ALTER TABLE agent_tenants ADD COLUMN transcript_signing_key BLOB;
ALTER TABLE agent_tenants ADD COLUMN transcript_signing_key_nonce BLOB;
```

This migration was **never applied** to the database.

### Why the Migration Was Skipped
The migration system uses `GetCurrentSchemaVersion()` (`store/migrator.go:223-236`) to determine which migrations to apply:

```go
func (s *Store) GetCurrentSchemaVersion() (string, error) {
    currentVersion := version.GetCurrentVersion(s.profile.Mode)  // "0.31.0"
    minorVersion := version.GetMinorVersion(currentVersion)      // "0.31"
    filePaths, err := fs.Glob(migrationFS, fmt.Sprintf("%s%s/*.sql", s.getMigrationBasePath(), minorVersion))
    // returns files only from migration/sqlite/0.31/
    ...
    return s.getSchemaVersionOfMigrateScript(filePaths[len(filePaths)-1])  // "0.31.4" (last in 0.31/)
}
```

Then in the `Migrate` loop (`store/migrator.go:91-106`):
```go
for _, filePath := range filePaths {  // globs ALL migration/sqlite/*/*.sql
    fileSchemaVersion, _ := s.getSchemaVersionOfMigrateScript(filePath)  // "0.32.2"
    if version.IsVersionGreaterThan(fileSchemaVersion, latestMigrationHistoryVersion)  // "0.32.2" > "0.31.3" → true
        && version.IsVersionGreaterOrEqualThan(schemaVersion, fileSchemaVersion) {     // "0.31.0" >= "0.32.2" → **false**
        // NEVER REACHED for 0.32/ files
        apply(filePath)
    }
}
```

The guard `schemaVersion >= fileSchemaVersion` uses `schemaVersion = "0.31.0"` (from `DevVersion`). Since `"0.31.0" < "0.32.2"`, all files in `store/migration/sqlite/0.32/` and `store/migration/sqlite/0.33/` are silently skipped.

### Root Cause (Systemic)
The schema version used for migration decisions (`schemaVersion`) is derived from a **hardcoded string constant** in `internal/version/version.go`:

```go
var DevVersion = "0.31.0"
var Version   = "0.31.0"
```

This constant must be manually bumped every time a new migration subdirectory (e.g., `0.32/`, `0.33/`) is added. There is:
- **No automated check** that `DevVersion` / `Version` matches the latest migration directory
- **No warning or error** when migration files are skipped due to the version guard
- **No test** that verifies all migration directories are covered by the current version constant
- **No fail-fast mechanism** — the server starts successfully but SQL queries fail at runtime with cryptic 500 errors

### Scope of Impact (What Else Is Broken)
**Both SQLite and Postgres drivers are affected.** The bug is in the shared migration logic (`store/migrator.go`), not in any driver-specific code. Two migration directories are silently skipped for both drivers:

| Driver | File | Purpose |
|--------|------|---------|
| sqlite | `0.32/01__transcript_signing_key.sql` | Adds `transcript_signing_key` and `transcript_signing_key_nonce` columns to `agent_tenants` |
| sqlite | `0.33/00__fix_max_message_length_default.sql` | Fixes `max_message_length` default from 4000 to 2000 in `agent_audiences` |
| postgres | `0.32/01__transcript_signing_key.sql` | Same column addition (type `BYTEA` instead of `BLOB`) |
| postgres | `0.33/00__add_system_secret.sql` | Creates `system_secret` table for encryption salt storage (fresh installs get it from `LATEST.sql`; upgrades miss it) |

Any feature that depends on these columns/values will also be broken (e.g., transcript signing, encryption key management).

---

## Proposed Solution

The underlying problem is that the migration system trusts a manually maintained constant as the source of truth for which migrations should run. The fix must remove this human dependency by deriving the schema version from the migration filesystem itself.

### Design Principles
1. **Source of truth = filesystem**: The migration directories on disk should be the single source of truth for what schema version the code expects
2. **Fail fast**: If the migration system detects an inconsistency, it should error at startup, not at runtime
3. **Defense in depth**: Even with the auto-detection fix, add a validation check that the hardcoded version constant is in sync with the FS

### Step 1: Derive Schema Version from Migration FS

**File: `store/migrator.go`**

Replace `GetCurrentSchemaVersion()` with an implementation that scans **all** migration subdirectories to find the highest version file, rather than deriving from `version.GetCurrentVersion()`:

```go
func (s *Store) GetCurrentSchemaVersion() (string, error) {
    filePaths, err := fs.Glob(migrationFS, fmt.Sprintf("%s*/*.sql", s.getMigrationBasePath()))
    if err != nil {
        return "", errors.Wrap(err, "failed to glob migration files")
    }
    if len(filePaths) == 0 {
        return "", errors.Errorf("no migration files found in %s", s.getMigrationBasePath())
    }

    // Find the file with the highest version across all subdirectories
    var maxVersion string
    for _, filePath := range filePaths {
        version, err := s.getSchemaVersionOfMigrateScript(filePath)
        if err != nil {
            continue
        }
        if maxVersion == "" || version.IsVersionGreaterThan(version, maxVersion) {
            maxVersion = version
        }
    }
    if maxVersion == "" {
        return "", errors.Errorf("could not determine schema version from migration files")
    }
    return maxVersion, nil
}
```

**Implications of this change:**
- The migration system no longer depends on `version.GetCurrentVersion(mode)` → the hardcoded `Version`/`DevVersion` constants cannot cause migration-skip bugs
- Adding a new migration directory (e.g., `0.34/`) automatically makes those migrations eligible to run
- The `preMigrate` path (fresh database) is unaffected — it already applies `LATEST.sql` directly

**Edge case — pre-planned future migrations:** If developers commit migration files for future releases (e.g., `0.34/` when current release is `0.32`), the `GetCurrentSchemaVersion` would return a version from `0.34/`. But the `Migrate` loop already handles this correctly because it iterates ALL migration files and applies only those where:
```go
fileSchemaVersion > latestMigrationHistoryVersion  // newer than what DB has
fileSchemaVersion <= schemaVersion                  // this would also be true for 0.34
```

So if someone pre-plans `0.34/` files but hasn't released `0.32` yet, the `0.34` migrations would be applied prematurely. To prevent this, we add Step 2.

### Step 2: Add Version Validation Guard

**File: `store/migrator.go`**

Even though the FS is the source of truth, we should validate that the `DevVersion`/`Version` constants are in the expected range. This acts as a "belt and suspenders" guard and alerts developers when they forget to bump these constants.

Add a validation function called early in `Migrate`:

```go
func (s *Store) validateSchemaVersionConsistency() error {
    // Get the minor version from the FS (highest migration directory)
    fsMinorVersion := s.getLatestMigrationDirectory()
    // Get the minor version from the code constant
    codeMinorVersion := version.GetMinorVersion(version.GetCurrentVersion(s.profile.Mode))

    if version.IsVersionGreaterThan(fsMinorVersion, codeMinorVersion) {
        slog.Warn("migration FS has directories newer than code version; you may need to bump Version/DevVersion",
            "fs_minor", fsMinorVersion, "code_minor", codeMinorVersion)
    }
    return nil // warn-only; don't block startup
}
```

This ensures developers are alerted when their version constant lags behind the migration FS.

**Important:** This is warn-only, not a hard fail. The auto-detection fix in Step 1 ensures the correct migrations still run regardless. The warning is simply a hint to keep the version constant accurate for API metadata, user-agent strings, etc.

### Step 3: Add Fail-Fast Warning for Skipped Migrations

**File: `store/migrator.go`**

In the migration loop, add a warning when a migration file is skipped because `schemaVersion < fileSchemaVersion`:

```go
if version.IsVersionGreaterThan(fileSchemaVersion, latestMigrationHistoryVersion) {
    if !version.IsVersionGreaterOrEqualThan(schemaVersion, fileSchemaVersion) {
        slog.Warn("migration file skipped: schema version too low",
            "file", filePath,
            "file_version", fileSchemaVersion,
            "schema_version", schemaVersion,
            "latest_applied", latestMigrationHistoryVersion)
        continue
    }
    // apply migration
}
```

This would have caught the bug immediately — the log would show:
```
WARN migration file skipped: schema version too low file=migration/sqlite/0.32/01__transcript_signing_key.sql file_version=0.32.2 schema_version=0.31.0 latest_applied=0.31.3
```

### Step 4: Add Migration Coverage Test

**File: `store/migrator_test.go`** (new or existing)

Add a test that verifies all migration directories have corresponding version coverage:

```go
func TestAllMigrationDirectoriesCovered(t *testing.T) {
    // Scan all migration subdirectories
    // Verify each has corresponding migration history entries that would be applied
    // or at minimum, that GetCurrentSchemaVersion() > the highest migration directory
}
```

Specifically, the test should:
1. List all subdirectories in `migration/sqlite/` and `migration/postgres/`
2. Compute the max migration version for each driver
3. Instantiate `Store` with the driver-specific `getMigrationBasePath()`
4. Call `GetCurrentSchemaVersion()` for each
5. Assert that for each driver, `GetCurrentSchemaVersion() >= max(migration versions)` for that driver

This test would fail when new migration directories are added without the version constant being bumped, serving as a CI gate.

**Important design note:** The current `GetCurrentSchemaVersion()` receives the driver base path internally via `s.getMigrationBasePath()` which reads from `s.profile.Driver`. For the test to exercise both drivers, the function should accept the base path as a parameter (making it testable), or the test should instantiate two Store objects with different driver settings.

### Step 5: Fix the Immediate Bug (Apply Pending Migrations)

After Steps 1-4 are implemented, restart the server. The `Migrate` function will:

**SQLite:**
1. Detect `schemaVersion` from the FS as `0.33.1` (from `0.33/00__fix_max_message_length_default.sql`)
2. Find all migration files between `0.31.3` (latest history) and `0.33.1`
3. Apply `0.32/01__transcript_signing_key.sql` and `0.33/00__fix_max_message_length_default.sql`
4. Upsert migration history to `0.33.1`

**Postgres:**
1. Detect `schemaVersion` from the FS as `0.33.1` (from `0.33/00__add_system_secret.sql`)
2. Find all migration files between the latest Postgres history and `0.33.1`
3. Apply `0.32/01__transcript_signing_key.sql` and `0.33/00__add_system_secret.sql`
4. Upsert migration history to `0.33.1`

No manual SQL execution required. The fix is driver-agnostic because `GetCurrentSchemaVersion` already uses `s.getMigrationBasePath()` which returns `migration/sqlite/` or `migration/postgres/` based on `s.profile.Driver`.

### Rollback Plan
If any step causes issues during migration application:
- The `execute` function tolerates `"duplicate column"` and `"already exists"` errors, making ALTER TABLE statements idempotent
- The entire migration batch runs in a single transaction (line 84), so if any migration fails, the transaction rolls back and the DB remains at `0.31.3`

---

## Implementation Order

| Step | File(s) | Change | Risk |
|------|---------|--------|------|
| 1 | `store/migrator.go` | Rewrite `GetCurrentSchemaVersion()` to derive from FS | Medium — changes core migration logic |
| 2 | `store/migrator.go` | Add `validateSchemaVersionConsistency()` | Low — warn-only |
| 3 | `store/migrator.go` | Add skipped-migration log warning in migration loop | Low — log-only |
| 4 | `store/migrator_test.go` | Add migration coverage test | Low — test only |
| 5 | (restart server) | Pending migrations apply automatically | Low — idempotent, transactional |

## Files Not Changed
- `internal/version/version.go` — `Version` and `DevVersion` constants remain for API metadata; the migration system no longer depends on them
- `store/db/sqlite/agent.go` — no change needed (the SQL query was correct; the columns were just missing)
- `store/db/postgres/agent.go` — no change needed (same query pattern, same missing columns)
- All migration `.sql` files — no change needed (the SQL is correct; it just wasn't being run)

---

## Adversarial Review Prompt

> Review this plan for Bug 045 (database migrations silently skipped due to hardcoded version constant) with an adversarial mindset. Focus on:
>
> 1. **Edge cases**: What happens if the migration filesystem has gaps (e.g., `0.32/` directory but no `0.31/`)? What if the migration FS has files from a future major version? What if `LATEST.sql` is out of sync with the per-version migration files?
> 2. **Security**: Could the auto-detection from FS be subverted (e.g., by a malicious migration file being added)?
> 3. **Concurrency**: The migration runs at startup. If two server instances start simultaneously against the same DB, could Step 5's auto-apply race? (The transaction should prevent this, but verify.)
> 4. **Rollback safety**: Step 5 mentions the transaction handles rollback, but what about DDL statements (ALTER TABLE) that SQLite cannot roll back within a transaction? Confirm that SQLite's DDL-is-transactional behavior applies (it does for ALTER TABLE in WAL mode, but verify).
> 5. **Downside of auto-detection**: Step 1 removes the human gate. Previously, migrations were gated by the release version. Now, any migration file committed to the codebase will be applied on next startup regardless of release readiness. Is this desirable? Should there be an explicit "enable" mechanism for unreleased migrations?
> 6. **DevVersion vs Version in Step 2**: The warn-only check compares `Version`/`DevVersion` to the FS. Is there ever a legitimate case where these should differ (e.g., backporting migrations)? Should the warn-only be a hard error in CI but not in dev mode?
> 7. **Postgres parity**: The fix focuses on SQLite. Does the same bug affect Postgres? Check if `migration/postgres/` has directories beyond the current version.
> 8. **Test quality**: Step 4's proposed test — would it have caught the current bug? Would it be flaky (dependent on FS state in CI)?
>
> Provide specific recommendations to strengthen the plan.

---

## Verification

### SQLite
1. `PRAGMA table_info(agent_tenants)` shows `transcript_signing_key` and `transcript_signing_key_nonce` columns
2. `GET /api/v1/agent/tenants` with valid JWT returns 200 with tenants list
3. `SELECT * FROM migration_history ORDER BY version DESC LIMIT 1;` shows `0.33.1`
4. `SELECT max_message_length FROM agent_audiences;` shows no rows with NULL or 4000

### Postgres
1. `\d agent_tenants` shows `transcript_signing_key` (BYTEA) and `transcript_signing_key_nonce` (BYTEA) columns
2. `GET /api/v1/agent/tenants` with valid JWT returns 200 with tenants list
3. `SELECT * FROM migration_history ORDER BY version DESC LIMIT 1;` shows `0.33.1`
4. `SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'system_secret');` returns `true`
