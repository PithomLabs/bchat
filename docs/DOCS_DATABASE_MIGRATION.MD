# Database Migration System Deep Dive

**Status:** Complete
**Date:** 2026-01-30
**Related Files:**
- `store/migrator.go` - Migration orchestration logic
- `store/migration/sqlite/LATEST.sql` - Full schema for new databases
- `store/migration/sqlite/0.25/*.sql` - Individual migration files

---

## Overview

The bchat application uses a **compile-time embedded migration system**. Understanding this architecture is critical because:

1. **Migration files are embedded into the binary at compile time**
2. **Changes to migration files require rebuilding the application**
3. **LATEST.sql and individual migrations serve different purposes**

---

## Architecture

### File Embedding with `//go:embed`

```go
// store/migrator.go

//go:embed migration
var migrationFS embed.FS

//go:embed seed
var seedFS embed.FS
```

The `//go:embed` directive is a Go compiler feature that:
- Reads the `migration/` directory at **compile time**
- Embeds all files into the compiled binary as a virtual filesystem
- Makes files accessible via `migrationFS.ReadFile(path)`

**Critical Implication:** If you modify any file in `store/migration/`, you **MUST rebuild the binary** for changes to take effect. The running binary contains a snapshot of migration files from when it was compiled.

### Directory Structure

```
store/migration/sqlite/
├── LATEST.sql              # Full schema (used for NEW databases)
└── 0.25/                   # Version-specific migrations
    ├── 00__tickets.sql
    ├── 01__alter_tickets.sql
    ├── ...
    ├── 16__processing_options.sql
    ├── 17__agent_qa_pairs.sql
    ├── 18__agent_transcripts.sql
    └── 19__reasoning_model.sql
```

---

## Migration Flow

### Startup Sequence

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        Application Startup                                   │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         preMigrate()                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  Query: SELECT * FROM migration_history                              │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│              ┌─────────────────────┴─────────────────────┐                  │
│              │                                           │                  │
│              ▼                                           ▼                  │
│   ┌──────────────────────┐                 ┌──────────────────────┐         │
│   │  No history / Error  │                 │   History exists     │         │
│   │  (NEW DATABASE)      │                 │   (EXISTING DB)      │         │
│   └──────────┬───────────┘                 └──────────┬───────────┘         │
│              │                                        │                     │
│              ▼                                        ▼                     │
│   ┌──────────────────────┐                 ┌──────────────────────┐         │
│   │  Apply LATEST.sql    │                 │  Skip to Migrate()   │         │
│   │  (full schema)       │                 │                      │         │
│   └──────────────────────┘                 └──────────────────────┘         │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Migrate()                                          │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  Compare: database_version vs binary_schema_version                  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│              ┌─────────────────────┴─────────────────────┐                  │
│              │                                           │                  │
│              ▼                                           ▼                  │
│   ┌──────────────────────┐                 ┌──────────────────────┐         │
│   │  Binary is NEWER     │                 │  Versions match      │         │
│   └──────────┬───────────┘                 └──────────┬───────────┘         │
│              │                                        │                     │
│              ▼                                        ▼                     │
│   ┌──────────────────────┐                 ┌──────────────────────┐         │
│   │  Apply individual    │                 │  No migration needed │         │
│   │  migration files     │                 │                      │         │
│   │  (NN__*.sql)         │                 │                      │         │
│   └──────────────────────┘                 └──────────────────────┘         │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Two Paths: New vs Existing Database

| Scenario | What Happens | Files Used |
|----------|--------------|------------|
| **New Database** | No `migration_history` table exists | `LATEST.sql` only |
| **Existing Database** | `migration_history` has version records | Individual `NN__*.sql` files |

---

## LATEST.sql vs Individual Migrations

### LATEST.sql

**Purpose:** Provides the complete, current schema for brand new databases.

**When Used:**
- First deployment to a new environment
- After deleting/recreating the database volume
- When `migration_history` table doesn't exist or is empty

**Contents:** Must contain ALL tables and columns that exist in the current schema.

```sql
-- LATEST.sql should be the UNION of:
-- 1. Base tables (user, memo, resource, etc.)
-- 2. All tables added by migrations 00-19
-- 3. All columns added by ALTER TABLE migrations
```

### Individual Migration Files (NN__description.sql)

**Purpose:** Incremental changes applied to existing databases during upgrades.

**When Used:**
- Upgrading an existing installation
- Binary has newer schema version than database

**Naming Convention:**
```
NN__description.sql
│  │
│  └── Human-readable description
└───── Sequence number (determines order and version)
```

**Version Calculation:**
```
File: 0.25/16__processing_options.sql
      ────  ──
       │     │
       │     └── Patch version = 16
       └──────── Minor version = 0.25

Schema Version = "0.25.17" (patch + 1)
```

---

## The Root Cause of the fly.io Issue

### What Happened

1. **fly.io deployment created a NEW database** (fresh volume)
2. **preMigrate() detected no migration history** → Used `LATEST.sql`
3. **LATEST.sql was outdated** → Missing tables/columns from migrations 07-19
4. **Individual migrations were SKIPPED** → Database already at "latest" version
5. **Result:** `processing_options` column missing, app crashes

### Why LATEST.sql Gets Outdated

When developers add new migrations:

```
✓ Create store/migration/sqlite/0.25/16__processing_options.sql
✗ Forget to update store/migration/sqlite/LATEST.sql
```

The migration works fine for **existing databases** (individual file applied), but fails for **new databases** (LATEST.sql used).

### The Fix

**LATEST.sql must always be kept in sync with all migrations.**

After adding `16__processing_options.sql`:
```sql
-- LATEST.sql must include:
CREATE TABLE agent_tenants (
    ...
    processing_options TEXT,  -- ← Must be added here too!
    ...
);
```

---

## Rebuilding and Deploying

### Why Rebuild is Required

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        Build Process                                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   Source Files                    Compiled Binary                           │
│   ┌─────────────────┐             ┌─────────────────┐                       │
│   │ LATEST.sql      │             │                 │                       │
│   │ 00__tickets.sql │ ──────────▶ │  memos binary   │                       │
│   │ 16__proc....sql │  go:embed   │  (contains all  │                       │
│   │ ...             │             │   SQL files)    │                       │
│   └─────────────────┘             └─────────────────┘                       │
│                                                                             │
│   Changes here                    Must rebuild to                           │
│   are NOT reflected ──────────────include changes                           │
│   until rebuild                                                             │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Deployment Steps After Updating LATEST.sql

```bash
# 1. Update LATEST.sql (done)

# 2. Rebuild and redeploy to fly.io
fly deploy

# The Docker build will:
# - Run 'go build' which embeds updated LATEST.sql
# - Create new image with updated binary
# - Deploy to fly.io
```

### For Existing Databases with Missing Columns

If the database already exists but is missing columns:

**Option A: Fresh Start (delete volume)**
```bash
fly volumes destroy memos_data
fly volumes create memos_data --size 1 --region sjc
fly deploy
```

**Option B: Manual Migration (keep data)**
```bash
fly ssh console
sqlite3 /var/opt/memos/memos.db

-- Run missing migrations manually
ALTER TABLE agent_tenants ADD COLUMN processing_options TEXT;
-- ... other missing statements

.quit
exit
fly apps restart
```

---

## Version Tracking

### How Versions are Determined

**Binary Schema Version:**
```go
func (s *Store) GetCurrentSchemaVersion() (string, error) {
    // 1. Get app version (e.g., "0.25.0")
    currentVersion := version.GetCurrentVersion(s.profile.Mode)

    // 2. Get minor version (e.g., "0.25")
    minorVersion := version.GetMinorVersion(currentVersion)

    // 3. Find highest migration file number
    filePaths, _ := fs.Glob(migrationFS, "migration/sqlite/0.25/*.sql")
    // e.g., ["00__tickets.sql", ..., "19__reasoning_model.sql"]

    // 4. Return version based on highest file
    // "0.25/19__reasoning_model.sql" → "0.25.20"
    return "0.25.20", nil
}
```

**Database Version:**
```sql
SELECT version FROM migration_history ORDER BY created_ts DESC LIMIT 1;
-- Returns: "0.25.20" (or whatever was last recorded)
```

### Migration History Table

```sql
CREATE TABLE migration_history (
    version TEXT NOT NULL PRIMARY KEY,
    created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now'))
);

-- Example data:
-- version    | created_ts
-- -----------|------------
-- 0.25.1     | 1706500000
-- 0.25.10    | 1706600000
-- 0.25.20    | 1706700000  ← Current version
```

---

## Best Practices

### When Adding New Migrations

1. **Create the migration file:**
   ```bash
   # Find next number
   ls store/migration/sqlite/0.25/
   # Create new file
   touch store/migration/sqlite/0.25/20__my_new_feature.sql
   ```

2. **Write the migration SQL:**
   ```sql
   -- 20__my_new_feature.sql
   ALTER TABLE some_table ADD COLUMN new_column TEXT;
   ```

3. **Update LATEST.sql:**
   ```sql
   -- Add the column to the CREATE TABLE statement
   CREATE TABLE some_table (
       ...
       new_column TEXT,  -- ← Add here!
       ...
   );
   ```

4. **Test both paths:**
   - Fresh database (uses LATEST.sql)
   - Existing database (uses migration file)

### Migration File Guidelines

| Do | Don't |
|----|-------|
| Use `IF NOT EXISTS` for CREATE TABLE | Assume table doesn't exist |
| Use `ALTER TABLE ADD COLUMN` for new columns | Recreate entire tables |
| Keep migrations idempotent when possible | Write migrations that fail on re-run |
| Update LATEST.sql immediately | Forget to sync LATEST.sql |

### Testing Migrations Locally

```bash
# Test fresh database
rm -rf build/data/memos_dev.db
task run:rag

# Test existing database migration
# (keep existing build/data/memos_dev.db)
task run:rag
# Check logs for "start migration" → "end migrate"
```

---

## Troubleshooting

### "no such column: X" Error

**Cause:** LATEST.sql missing column that exists in migration files.

**Fix:**
1. Add column to LATEST.sql
2. Rebuild and redeploy
3. For existing DB: run ALTER TABLE manually or recreate volume

### "no migration history found" Error

**Cause:** Database exists but migration_history table is empty/missing.

**Fix:** This is normal for new databases. preMigrate() will apply LATEST.sql.

### Migrations Not Applied on Deploy

**Cause:**
- Binary schema version equals database version
- Or LATEST.sql was used instead of migrations

**Debug:**
```bash
fly ssh console
sqlite3 /var/opt/memos/memos.db
SELECT * FROM migration_history ORDER BY created_ts;
.quit
```

### Schema Mismatch After Deploy

**Cause:** go:embed cached old files (rare, usually IDE cache issue)

**Fix:**
```bash
# Clean build
go clean -cache
task build:backend:rag
# Or for fly.io:
fly deploy --no-cache
```

---

## Summary

| Concept | Description |
|---------|-------------|
| **go:embed** | Embeds SQL files into binary at compile time |
| **LATEST.sql** | Complete schema for new databases |
| **NN__*.sql** | Incremental migrations for existing databases |
| **Rebuild required** | Any migration file change requires rebuilding |
| **Keep in sync** | LATEST.sql must contain all tables/columns |

---

---

## Addendum: Automated LATEST.sql Validation

To prevent LATEST.sql from getting out of sync with migrations, we can implement automated validation.

### Option 1: Build-Time Validation Script (Recommended)

Create a script that runs during build to verify LATEST.sql contains all expected tables/columns.

**File: `scripts/validate-migrations.sh`**

```bash
#!/bin/bash
# Validates that LATEST.sql is in sync with migration files
# Run: ./scripts/validate-migrations.sh

set -e

LATEST_SQL="store/migration/sqlite/LATEST.sql"
MIGRATIONS_DIR="store/migration/sqlite/0.25"

echo "Validating LATEST.sql against migrations..."

# Extract all ALTER TABLE ADD COLUMN statements from migrations
MISSING_COLUMNS=()

for migration in "$MIGRATIONS_DIR"/*.sql; do
    # Find ALTER TABLE ... ADD COLUMN statements
    while IFS= read -r line; do
        if [[ "$line" =~ ALTER[[:space:]]+TABLE[[:space:]]+([a-z_]+)[[:space:]]+ADD[[:space:]]+COLUMN[[:space:]]+([a-z_]+) ]]; then
            TABLE="${BASH_REMATCH[1]}"
            COLUMN="${BASH_REMATCH[2]}"

            # Check if column exists in LATEST.sql
            if ! grep -q "$COLUMN" "$LATEST_SQL"; then
                MISSING_COLUMNS+=("$TABLE.$COLUMN (from $(basename $migration))")
            fi
        fi
    done < "$migration"
done

# Extract all CREATE TABLE statements from migrations
for migration in "$MIGRATIONS_DIR"/*.sql; do
    while IFS= read -r line; do
        if [[ "$line" =~ CREATE[[:space:]]+TABLE[[:space:]]+(IF[[:space:]]+NOT[[:space:]]+EXISTS[[:space:]]+)?([a-z_]+) ]]; then
            TABLE="${BASH_REMATCH[2]}"

            # Check if table exists in LATEST.sql
            if ! grep -q "CREATE TABLE.*$TABLE" "$LATEST_SQL"; then
                MISSING_COLUMNS+=("TABLE $TABLE (from $(basename $migration))")
            fi
        fi
    done < "$migration"
done

if [ ${#MISSING_COLUMNS[@]} -gt 0 ]; then
    echo "ERROR: LATEST.sql is missing the following:"
    for item in "${MISSING_COLUMNS[@]}"; do
        echo "  - $item"
    done
    echo ""
    echo "Please update LATEST.sql to include these tables/columns."
    exit 1
fi

echo "✓ LATEST.sql is in sync with all migrations"
```

**Add to Taskfile.yml:**

```yaml
tasks:
  validate:migrations:
    desc: Validate LATEST.sql is in sync with migrations
    cmds:
      - ./scripts/validate-migrations.sh

  build:backend:rag:
    desc: Build the Go binary with LanceDB RAG support
    deps: [setup:lancedb, validate:migrations]  # ← Add validation
    # ... rest of task
```

### Option 2: Go Test for CI/CD

Create a Go test that validates schema consistency.

**File: `store/migration_test.go`**

```go
package store

import (
    "regexp"
    "strings"
    "testing"
)

func TestLatestSQLContainsAllMigrations(t *testing.T) {
    // Read LATEST.sql
    latestSQL, err := migrationFS.ReadFile("migration/sqlite/LATEST.sql")
    if err != nil {
        t.Fatalf("failed to read LATEST.sql: %v", err)
    }
    latestContent := string(latestSQL)

    // Read all migration files
    files, err := migrationFS.ReadDir("migration/sqlite/0.25")
    if err != nil {
        t.Fatalf("failed to read migrations dir: %v", err)
    }

    alterTableRegex := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+ADD\s+COLUMN\s+(\w+)`)
    createTableRegex := regexp.MustCompile(`CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)

    for _, file := range files {
        if file.IsDir() || file.Name() == "LATEST.sql" {
            continue
        }

        content, err := migrationFS.ReadFile("migration/sqlite/0.25/" + file.Name())
        if err != nil {
            t.Errorf("failed to read %s: %v", file.Name(), err)
            continue
        }

        // Check ALTER TABLE ADD COLUMN
        matches := alterTableRegex.FindAllStringSubmatch(string(content), -1)
        for _, match := range matches {
            column := match[2]
            if !strings.Contains(latestContent, column) {
                t.Errorf("LATEST.sql missing column '%s' from %s", column, file.Name())
            }
        }

        // Check CREATE TABLE
        matches = createTableRegex.FindAllStringSubmatch(string(content), -1)
        for _, match := range matches {
            table := match[1]
            if !strings.Contains(latestContent, "CREATE TABLE") ||
               !strings.Contains(latestContent, table) {
                t.Errorf("LATEST.sql missing table '%s' from %s", table, file.Name())
            }
        }
    }
}
```

**Run in CI:**
```bash
go test ./store -run TestLatestSQLContainsAllMigrations
```

### Option 3: Runtime Startup Validation

Add validation during application startup (warns but doesn't block).

**Add to `store/migrator.go`:**

```go
func (s *Store) validateLatestSQL(ctx context.Context) {
    // Only validate in dev mode
    if s.profile.Mode != "dev" {
        return
    }

    // Get all migration files
    filePaths, err := fs.Glob(migrationFS, fmt.Sprintf("%s0.25/*.sql", s.getMigrationBasePath()))
    if err != nil {
        slog.Warn("failed to validate LATEST.sql", "error", err)
        return
    }

    latestPath := s.getMigrationBasePath() + LatestSchemaFileName
    latestContent, err := migrationFS.ReadFile(latestPath)
    if err != nil {
        return
    }
    latest := string(latestContent)

    alterRegex := regexp.MustCompile(`(?i)ALTER\s+TABLE\s+(\w+)\s+ADD\s+COLUMN\s+(\w+)`)

    for _, filePath := range filePaths {
        if strings.HasSuffix(filePath, LatestSchemaFileName) {
            continue
        }

        content, err := migrationFS.ReadFile(filePath)
        if err != nil {
            continue
        }

        matches := alterRegex.FindAllStringSubmatch(string(content), -1)
        for _, match := range matches {
            column := match[2]
            if !strings.Contains(latest, column) {
                slog.Warn("LATEST.sql may be out of sync",
                    "missing_column", column,
                    "migration_file", filePath,
                )
            }
        }
    }
}
```

Call it in `Migrate()`:
```go
func (s *Store) Migrate(ctx context.Context) error {
    s.validateLatestSQL(ctx)  // ← Add at start
    // ... rest of function
}
```

### Option 4: Pre-commit Hook

**File: `.githooks/pre-commit`**

```bash
#!/bin/bash
# Validate migrations before commit

if git diff --cached --name-only | grep -q "store/migration/sqlite/0.25/"; then
    echo "Migration files changed, validating LATEST.sql..."
    ./scripts/validate-migrations.sh
    if [ $? -ne 0 ]; then
        echo "Commit blocked: LATEST.sql validation failed"
        exit 1
    fi
fi
```

**Enable:**
```bash
git config core.hooksPath .githooks
chmod +x .githooks/pre-commit
```

### Recommended Implementation

| Environment | Validation Method |
|-------------|-------------------|
| Local Dev | Pre-commit hook + Taskfile validation |
| CI/CD | Go test (`migration_test.go`) |
| Runtime | Startup warning in dev mode |

### Implementation Priority

1. **Immediate:** Add `scripts/validate-migrations.sh` and integrate with Taskfile
2. **Short-term:** Add Go test for CI/CD
3. **Optional:** Add runtime validation for dev mode warnings

---

## See Also

- [DOCS_DEPLOYMENT.MD](./DOCS_DEPLOYMENT.MD) - fly.io deployment guide
- [CLAUDE.MD](../CLAUDE.MD) - Adding new database tables
- `store/migrator.go` - Migration implementation details
