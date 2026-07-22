# Bug 046: Chat Widget Not Appearing — Hugo Defaults + Migration Automation

**Status:** PLANNED
**Date:** 2026-07-23
**Depends on:** Bug 045 (migrator.go fix for `GetCurrentSchemaVersion()`)
**Affected repos:** bchat server (`/home/chaschel/Documents/go/bchat`), Hugo site (`/home/chaschel/Documents/go/izaakmaine.github.io-main`)

---

## Problem Statement

Two independent root causes prevent the bchat chat widget from appearing on tenant landing pages:

### Root Cause 1: `chatBaseUrl` Hardcoded to localhost in Hugo Content

Every tenant `_index.md` hardcodes `chatBaseUrl: "http://localhost:8081"`. When the Hugo site is deployed to GitHub Pages, the browser tries to fetch `embed.js` from `localhost:8081` — which is the visitor's own machine, not the bchat server. The script fails with `ERR_CONNECTION_REFUSED` and the widget never initializes.

The Hugo template (`layouts/_default/list.html:225`) already has a default:
```go
{{ $chatUrl := .Params.chatBaseUrl | default "https://bchat-pg.fly.dev" }}
```
But this default is never reached because every content file explicitly sets the value.

**Fix:** Remove hardcoded `chatBaseUrl` from all content files. Make the default configurable via `hugo.yaml` site params with a safe localhost fallback.

### Root Cause 2: Migration System Skips Files in Newer Minor-Version Directories

Bug 045 documents this fully. The short version: `GetCurrentSchemaVersion()` only scans the directory matching `DevVersion` (e.g., `0.31/`), so migrations in `0.32/` and `0.33/` are silently skipped. The `agent_tenants` table is missing `transcript_signing_key` and `transcript_signing_key_nonce` columns, causing `ListAgentTenants` to fail with a SQL error that propagates as 404 "Agent not found".

**Fix:** Already planned in bug 045 Steps 1-5. This bug adds the automation layer to prevent recurrence.

---

## Scope

| In Scope | Out of Scope |
|----------|-------------|
| Hugo chatBaseUrl defaults via hugo.yaml | The migrator.go code fix (bug 045) |
| Version bump script (derive from FS) | Widget source code changes |
| `task migrate:new` auto-creation | bchat server feature work |
| Definitive migration guide | Postgres-specific fixes (covered by bug 045) |
| AGENTS.md / old doc deprecation | Content data changes (YAML, images, etc.) |

---

## Detailed Implementation

### Part A: Hugo Chat Widget Fix (Hugo repo)

#### Step 8: Add `bchat.baseUrl` to `hugo.yaml`

**File:** `/home/chaschel/Documents/go/izaakmaine.github.io-main/hugo.yaml`

Add under the existing `params:` block (after line 66):

```yaml
params:
  # ... existing params ...

  bchat:
    # Base URL of the bchat server for the chat widget.
    # Override per-environment via config/ directory or environment variables.
    # Default: localhost for local dev (hugo server), set explicitly for production.
    baseUrl: "http://localhost:8081"
```

**Rationale:** The default is `localhost:8081` because:
- `hugo server` (local dev) always runs against the local bchat server
- Production deployments override this via Hugo's environment config (`config/production/params.yaml`) or CLI flags
- No production URL is hardcoded in the repo — security-conscious default

**For production deployments**, create `config/production/params.yaml`:
```yaml
params:
  bchat:
    baseUrl: "https://bchat.example.com"
```
Or pass at build time:
```bash
hugo --environment production --set params.bchat.baseUrl=https://bchat.example.com
```

#### Step 9: Update Landing Page Template

**File:** `/home/chaschel/Documents/go/izaakmaine.github.io-main/layouts/_default/list.html`

Change line 225 from:
```go
{{ $chatUrl := .Params.chatBaseUrl | default "https://bchat-pg.fly.dev" }}
```

To:
```go
{{ $chatUrl := .Params.chatBaseUrl | default site.Params.bchat.baseUrl | default "http://localhost:8081" }}
```

**Three-tier fallback:**
1. Page-level `chatBaseUrl` in front matter (highest priority — per-tenant overrides)
2. `site.Params.bchat.baseUrl` from `hugo.yaml` (configurable per environment)
3. Hardcoded `"http://localhost:8081"` (safe default — never produces a broken production URL)

**Security rationale:** The hardcoded fallback is localhost. A misconfigured deployment will fail locally (developer catches it), not in production (where it would expose an internal URL or fail silently).

#### Steps 10-12: Remove Hardcoded `chatBaseUrl` from Content Files

Remove the `chatBaseUrl: "http://localhost:8081"` line from:
- `content/rgresidences/_index.md` (line 5)
- `content/bchat/_index.md`
- `content/evpn/_index.md`

After removal, the template falls through to `site.Params.bchat.baseUrl` (from hugo.yaml), which defaults to `http://localhost:8081` for local dev.

---

### Part B: Migration Automation (bchat server repo)

#### Step 1: Bump Version to 0.34.0

**File:** `internal/version/version.go`

Change:
```go
var Version = "0.31.0"
var DevVersion = "0.31.0"
```

To:
```go
var Version = "0.34.0"
var DevVersion = "0.34.0"
```

**Rationale:** `0.33/` is the latest migration directory. Next development cycle starts at `0.34.0`. This is a documentation convention — the auto-detection fix in bug 045 makes it non-critical, but it keeps the version string honest.

#### Step 2: Create `scripts/bump-version.sh`

**File:** `scripts/bump-version.sh` (new)

```bash
#!/usr/bin/env bash
# bump-version.sh — Derive schema version from migration filesystem and update version.go
#
# Usage:
#   ./scripts/bump-version.sh           # Update version.go
#   ./scripts/bump-version.sh --dry-run # Show what would change without writing
#
# The version is computed as:
#   - Find the highest migration directory (e.g., 0.33/)
#   - Find the highest patch file in that directory (e.g., 00__foo.sql → patch 0)
#   - Result: <minor>.<patch+1> (e.g., 0.33.1)
#
# If no migration directories exist, falls back to 0.1.0.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MIGRATION_BASE="$REPO_ROOT/store/migration/sqlite"
VERSION_FILE="$REPO_ROOT/internal/version/version.go"

DRY_RUN=false
if [[ "${1:-}" == "--dry-run" ]]; then
    DRY_RUN=true
fi

# Find the highest migration directory (sort by version number)
LATEST_DIR=""
for dir in "$MIGRATION_BASE"/*/; do
    dirname="$(basename "$dir")"
    # Skip non-version directories
    if [[ "$dirname" =~ ^[0-9]+\.[0-9]+$ ]]; then
        if [[ -z "$LATEST_DIR" ]] || [[ "$dirname" > "$LATEST_DIR" ]]; then
            LATEST_DIR="$dirname"
        fi
    fi
done

if [[ -z "$LATEST_DIR" ]]; then
    echo "ERROR: No migration directories found in $MIGRATION_BASE"
    exit 1
fi

# Find the highest patch file in the latest directory
MAX_PATCH=-1
for sql_file in "$MIGRATION_BASE/$LATEST_DIR"/*.sql; do
    [[ -f "$sql_file" ]] || continue
    filename="$(basename "$sql_file")"
    # Extract patch number: "00__description.sql" → "00"
    patch_str="${filename%%__*}"
    if [[ "$patch_str" =~ ^[0-9]+$ ]]; then
        patch_num=$((10#$patch_str))  # Force base-10 interpretation
        if (( patch_num > MAX_PATCH )); then
            MAX_PATCH=$patch_num
        fi
    fi
done

# Compute version: if MAX_PATCH is -1 (no files), use .0; otherwise patch+1
if (( MAX_PATCH < 0 )); then
    NEW_VERSION="${LATEST_DIR}.0"
else
    NEW_VERSION="${LATEST_DIR}.$((MAX_PATCH + 1))"
fi

echo "Latest migration directory: $LATEST_DIR"
echo "Highest patch number: $MAX_PATCH"
echo "Derived version: $NEW_VERSION"
echo ""

# Read current version from version.go
CURRENT_VERSION=$(grep -oP 'var DevVersion = "\K[^"]+' "$VERSION_FILE")
echo "Current DevVersion: $CURRENT_VERSION"

if [[ "$CURRENT_VERSION" == "$NEW_VERSION" ]]; then
    echo "Version is already up to date. No changes needed."
    exit 0
fi

if $DRY_RUN; then
    echo ""
    echo "[dry-run] Would update $VERSION_FILE:"
    echo "  DevVersion: \"$CURRENT_VERSION\" → \"$NEW_VERSION\""
    echo "  Version:    \"$CURRENT_VERSION\" → \"$NEW_VERSION\""
    exit 0
fi

# Update version.go
sed -i "s/var Version = \"$CURRENT_VERSION\"/var Version = \"$NEW_VERSION\"/" "$VERSION_FILE"
sed -i "s/var DevVersion = \"$CURRENT_VERSION\"/var DevVersion = \"$NEW_VERSION\"/" "$VERSION_FILE"

echo ""
echo "Updated $VERSION_FILE:"
echo "  Version:    \"$NEW_VERSION\""
echo "  DevVersion: \"$NEW_VERSION\""
```

**Key properties:**
- Idempotent: running twice produces the same result
- `--dry-run` for preview
- No external dependencies (pure bash + sed)
- Forces base-10 interpretation of patch numbers (avoids octal issues with leading zeros)

#### Step 3: Create `scripts/create-migration.sh`

**File:** `scripts/create-migration.sh` (new)

```bash
#!/usr/bin/env bash
# create-migration.sh — Create a new migration file and bump version
#
# Usage:
#   ./scripts/create-migration.sh "add_widget_config"
#   ./scripts/create-migration.sh "add_widget_config" --dry-run
#
# Creates:
#   store/migration/sqlite/<next_dir>/<next_patch>__<name>.sql
#   store/migration/postgres/<next_dir>/<next_patch>__<name>.sql
#
# Then calls bump-version.sh to update version.go.

set -euo pipefail

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <migration_name> [--dry-run]"
    echo ""
    echo "Example: $0 \"add_widget_config\""
    echo "  Creates: store/migration/sqlite/0.34/00__add_widget_config.sql"
    echo "           store/migration/postgres/0.34/00__add_widget_config.sql"
    exit 1
fi

MIGRATION_NAME="$1"
DRY_RUN=false
if [[ "${2:-}" == "--dry-run" ]]; then
    DRY_RUN=true
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SQLITE_BASE="$REPO_ROOT/store/migration/sqlite"
POSTGRES_BASE="$REPO_ROOT/store/migration/postgres"

# Validate migration name (snake_case only)
if [[ ! "$MIGRATION_NAME" =~ ^[a-z][a-z0-9_]*$ ]]; then
    echo "ERROR: Migration name must be snake_case (lowercase letters, digits, underscores)"
    echo "  Got: $MIGRATION_NAME"
    exit 1
fi

# Find the latest migration directory
LATEST_DIR=""
for dir in "$SQLITE_BASE"/*/; do
    dirname="$(basename "$dir")"
    if [[ "$dirname" =~ ^[0-9]+\.[0-9]+$ ]]; then
        if [[ -z "$LATEST_DIR" ]] || [[ "$dirname" > "$LATEST_DIR" ]]; then
            LATEST_DIR="$dirname"
        fi
    fi
done

if [[ -z "$LATEST_DIR" ]]; then
    echo "ERROR: No migration directories found in $SQLITE_BASE"
    exit 1
fi

# Find the next patch number in the latest directory
MAX_PATCH=-1
for sql_file in "$SQLITE_BASE/$LATEST_DIR"/*.sql; do
    [[ -f "$sql_file" ]] || continue
    filename="$(basename "$sql_file")"
    patch_str="${filename%%__*}"
    if [[ "$patch_str" =~ ^[0-9]+$ ]]; then
        patch_num=$((10#$patch_str))
        if (( patch_num > MAX_PATCH )); then
            MAX_PATCH=$patch_num
        fi
    fi
done

NEXT_PATCH=$((MAX_PATCH + 1))
PATCH_STR=$(printf "%02d" "$NEXT_PATCH")
FILENAME="${PATCH_STR}__${MIGRATION_NAME}.sql"

echo "Migration directory: $LATEST_DIR"
echo "Next patch number:   $NEXT_PATCH"
echo "Filename:            $FILENAME"
echo ""

SQLITE_PATH="$SQLITE_BASE/$LATEST_DIR/$FILENAME"
POSTGRES_PATH="$POSTGRES_BASE/$LATEST_DIR/$FILENAME"

if $DRY_RUN; then
    echo "[dry-run] Would create:"
    echo "  $SQLITE_PATH"
    echo "  $POSTGRES_PATH"
    echo ""
    echo "[dry-run] Would bump version via bump-version.sh"
    exit 0
fi

# Create directories if they don't exist
mkdir -p "$SQLITE_BASE/$LATEST_DIR"
mkdir -p "$POSTGRES_BASE/$LATEST_DIR"

# Create SQLite migration file
cat > "$SQLITE_PATH" << EOF
-- Migration: $MIGRATION_NAME
-- Version: $LATEST_DIR.$((NEXT_PATCH + 1))
-- Date: $(date +%Y-%m-%d)
--
-- Description: TODO - describe what this migration does
--
-- Rules:
--   - Use IF NOT EXISTS for CREATE TABLE
--   - Use ALTER TABLE ADD COLUMN for new columns
--   - Keep migrations idempotent when possible
--   - Update LATEST.sql to match

EOF

# Create Postgres migration file
cat > "$POSTGRES_PATH" << EOF
-- Migration: $MIGRATION_NAME
-- Version: $LATEST_DIR.$((NEXT_PATCH + 1))
-- Date: $(date +%Y-%m-%d)
--
-- Description: TODO - describe what this migration does
-- NOTE: Use Postgres-specific types (e.g., BYTEA instead of BLOB)

EOF

echo "Created:"
echo "  $SQLITE_PATH"
echo "  $POSTGRES_PATH"
echo ""

# Bump version
"$SCRIPT_DIR/bump-version.sh"
```

**Key properties:**
- Validates migration name (snake_case)
- Creates both SQLite and Postgres files
- Includes a template with rules as comments
- Calls `bump-version.sh` automatically
- `--dry-run` for preview

#### Step 4: Add Taskfile Tasks

**File:** `Taskfile.yml`

Add after the existing tasks:

```yaml
  version:bump:
    desc: Auto-derive version from migration filesystem and update version.go
    cmds:
      - ./scripts/bump-version.sh

  migrate:new:
    desc: Create new migration file and bump version (usage: task migrate:new NAME=add_widget_config)
    cmds:
      - ./scripts/create-migration.sh "{{.NAME}}"
```

**Usage:**
```bash
task migrate:new NAME="add_widget_config"
# Creates: store/migration/sqlite/0.34/00__add_widget_config.sql
#          store/migration/postgres/0.34/00__add_widget_config.sql
# Bumps:   DevVersion to 0.34.1

task version:bump
# Shows current derived version, updates if needed
```

#### Step 5: Update AGENTS.md Migration Section

**File:** `AGENTS.md`

Replace the "Database Migrations" section (lines 526-540) with:

```markdown
### Database Migrations

**Guide:** [docs/DOCS_DATABASE_MIGRATION_GUIDE.md](docs/DOCS_DATABASE_MIGRATION_GUIDE.md) (supersedes this section)

**Quick reference:**

Location: `store/migration/sqlite/` and `store/migration/postgres/`

Create a new migration:
```bash
task migrate:new NAME="my_feature_name"
```

This creates migration files in the latest version directory and auto-bumps the version constant.

Naming: `NN__snake_case_description.sql`

Rules:
- Use `IF NOT EXISTS` for `CREATE TABLE`
- Use `ALTER TABLE ADD COLUMN` for new columns
- Keep migrations idempotent when possible
- Update `LATEST.sql` to match schema changes
- Both SQLite and Postgres files must be created

After adding migrations, verify:
```bash
./scripts/validate-migrations.sh   # LATEST.sql in sync
task validate:schema                 # Go schema tests
```
```

#### Step 6: Add Deprecation Notice to Old Doc

**File:** `docs/DOCS_DATABASE_MIGRATION.MD`

Add at the very top (before the existing title):

```markdown
> **DEPRECATED:** This document is superseded by [DOCS_DATABASE_MIGRATION_GUIDE.md](./DOCS_DATABASE_MIGRATION_GUIDE.md).
> It is retained for historical reference on the fly.io incident and LATEST.sql validation options.
> For current migration procedures, see the new guide.

---
```

#### Step 7: Create Definitive Migration Guide

**File:** `docs/DOCS_DATABASE_MIGRATION_GUIDE.md` (new)

Contents:

```markdown
# Database Migration Guide

**Status:** Current (supersedes `DOCS_DATABASE_MIGRATION.MD` and `AGENTS.md` migration section)
**Date:** 2026-07-23
**Related bugs:** 008, 009, 015, 045, 046

---

## Overview

The bchat application uses a **compile-time embedded migration system**. Migration SQL files are embedded into the Go binary via `//go:embed` and applied at startup.

Key facts:
- Migration files are embedded at **compile time** — changes require a rebuild
- Two paths: `LATEST.sql` for new databases, `NN__*.sql` for existing databases
- The schema version is derived from the **migration filesystem**, not a hardcoded constant
- Both SQLite and Postgres migrations must be kept in sync

---

## Quick Reference

| Task | Command |
|------|---------|
| Create new migration | `task migrate:new NAME="my_feature"` |
| Bump version from FS | `task version:bump` |
| Validate LATEST.sql sync | `./scripts/validate-migrations.sh` |
| Validate schema (Go tests) | `task validate:schema` |
| Full pre-deploy check | `task fly:pre-deploy` |

---

## Architecture

### Directory Structure

```
store/migration/
├── sqlite/
│   ├── LATEST.sql                    # Full schema for NEW databases
│   ├── 0.25/                         # Version directory (minor version)
│   │   ├── 00__tickets.sql           # Migration file (patch version)
│   │   ├── 01__alter_tickets.sql
│   │   └── ...
│   ├── 0.31/
│   │   ├── 00__agent_integrations.sql
│   │   ├── 01__agent_events.sql
│   │   └── 02__rag_active_versions.sql
│   ├── 0.32/
│   │   └── 01__transcript_signing_key.sql
│   └── 0.33/
│       └── 00__fix_max_message_length_default.sql
└── postgres/
    ├── LATEST.sql
    └── (same structure as sqlite, with Postgres-specific types)
```

### Version Detection

`GetCurrentSchemaVersion()` in `store/migrator.go`:
1. Scans **all** `migration/<driver>/*/*.sql` files
2. Finds the file with the highest version across all directories
3. Computes version as `<directory>.<patch+1>`
4. Example: `0.33/00__fix_max_message_length_default.sql` → `"0.33.1"`

### Migration Flow

```
Startup
  │
  ├── preMigrate()
  │   ├── No migration_history? → Apply LATEST.sql (new database)
  │   └── Has history? → Skip to Migrate()
  │
  └── Migrate()
      ├── Compare: FS schema version vs database version
      ├── If FS is newer: Apply NN__*.sql files in order (transactional)
      └── Upsert version to migration_history
```

### Two Paths: New vs Existing Database

| Scenario | What Happens | Files Used |
|----------|--------------|------------|
| **New database** | No `migration_history` table | `LATEST.sql` only |
| **Existing database** | `migration_history` has records | Individual `NN__*.sql` files |

---

## Adding Migrations

### Step-by-Step: New Migration

1. **Create the migration file:**
   ```bash
   task migrate:new NAME="add_widget_config"
   ```
   This creates files in both `sqlite/` and `postgres/` directories and bumps the version.

2. **Write the migration SQL:**

   SQLite example:
   ```sql
   -- Migration: add_widget_config
   -- Version: 0.34.1
   -- Date: 2026-07-23
   --
   -- Description: Add widget_config column to agent_tenants for per-tenant widget customization

   ALTER TABLE agent_tenants ADD COLUMN widget_config TEXT;
   ```

   Postgres example (same migration, different types):
   ```sql
   -- Migration: add_widget_config
   -- Version: 0.34.1
   -- Date: 2026-07-23
   --
   -- Description: Add widget_config column to agent_tenants for per-tenant widget customization

   ALTER TABLE agent_tenants ADD COLUMN widget_config TEXT;
   ```

3. **Update `LATEST.sql`:**
   ```sql
   -- Add the column to the CREATE TABLE statement
   CREATE TABLE agent_tenants (
       ...
       widget_config TEXT,  -- ← Add here
       ...
   );
   ```

4. **Validate:**
   ```bash
   ./scripts/validate-migrations.sh   # Check LATEST.sql sync
   task validate:schema                 # Run Go schema tests
   ```

5. **Test locally:**
   ```bash
   # Test existing database (incremental migration)
   task run
   # Check logs for "start migration" → "end migrate"

   # Test fresh database (LATEST.sql path)
   rm -rf build/data/memos_dev.db
   task run
   ```

### Step-by-Step: New Version Directory

When starting a new development cycle:

1. **Create the directory:**
   ```bash
   mkdir -p store/migration/sqlite/0.34
   mkdir -p store/migration/postgres/0.34
   ```

2. **Create the first migration:**
   ```bash
   task migrate:new NAME="my_first_feature"
   ```

3. **The version bumps automatically** to `0.34.1`.

---

## Rules and Conventions

### Migration File Rules

| Do | Don't |
|----|-------|
| Use `IF NOT EXISTS` for `CREATE TABLE` | Assume table doesn't exist |
| Use `ALTER TABLE ADD COLUMN` for new columns | Recreate entire tables (unless required by SQLite limitation) |
| Keep migrations idempotent when possible | Write migrations that fail on re-run |
| Update `LATEST.sql` to match | Forget to sync `LATEST.sql` |
| Create both SQLite and Postgres files | Only create one driver's file |
| Use `ON DELETE CASCADE` for foreign keys | Leave orphaned rows on delete |
| Add indexes for foreign key columns | Forget indexes on frequently queried FKs |

### LATEST.sql Rules

- Must contain **ALL** tables and columns from all migrations
- Must be the **complete, current schema** for new databases
- Must be updated **every time** a migration adds a table or column
- Validated automatically by `scripts/validate-migrations.sh` (build dependency)

### Version Naming

```
Store/migration/sqlite/
├── 0.34/                              ← Minor version (directory)
│   ├── 00__add_widget_config.sql      ← Patch 0 → version 0.34.1
│   ├── 01__add_widget_theme.sql       ← Patch 1 → version 0.34.2
│   └── 02__fix_widget_resize.sql      ← Patch 2 → version 0.34.3
└── 0.35/                              ← Next minor version
    └── 00__rewrite_widget.sql         ← Patch 0 → version 0.35.1
```

**Version = `<directory>.<highest_patch + 1>`**

---

## Gotchas and Known Issues

### 1. Tests Always Use LATEST.sql

Test environments create fresh in-memory databases, which always take the `LATEST.sql` path via `preMigrate()`. The incremental migration path (`Migrate()`) is **never exercised in tests**.

**Implication:** A broken migration file may pass tests but fail on existing databases.

**Mitigation:** Test on an existing database manually before deploying:
```bash
# Keep your existing build/data/memos_dev.db
task run
# Check logs for migration application
```

### 2. SQLite DDL Is Transactional (But With Caveats)

SQLite supports `ALTER TABLE` within transactions, so failed migrations roll back correctly. However:
- `CREATE TABLE` is transactional
- `ALTER TABLE ADD COLUMN` is transactional
- Some operations (like `VACUUM`) are not

The `execute()` function tolerates `"duplicate column"` and `"already exists"` errors for idempotency.

### 3. WAL Mode Visibility

SQLite in WAL mode provides snapshot isolation per-transaction. If the server process starts before a migration is applied by another process, it may not see the new columns until it starts a new transaction. This is generally not an issue because each HTTP request gets its own connection.

### 4. `go:embed` Requires Rebuild

Migration files are embedded at compile time. Any change to `store/migration/` requires rebuilding the binary:
```bash
task build:backend      # Standard build
task build:backend:rag  # With RAG support
```

The build task depends on `validate:migrations`, which checks LATEST.sql sync.

### 5. Dual Version Tracking (Technical Debt)

Versions are stored in two places:
- `migration_history` table (per-version rows)
- `workspace_basic_setting` schema_version field

TODO comments in `migrator.go` acknowledge this should be simplified. If these ever diverge, the migration system could behave unexpectedly. For now, both are updated together.

### 6. `LATEST.sql` Drift Is the #1 Recurring Bug

Every past migration incident (bugs 008, 009, 015, 045, 046) traces back to `LATEST.sql` being out of sync or the version constant not matching the filesystem. The automation in this bug (bump-version.sh, validate-migrations.sh) is designed to prevent this class of bug.

---

## Testing Checklist

### Fresh Database (LATEST.sql Path)
- [ ] Delete `build/data/memos_dev.db`
- [ ] Start server
- [ ] Verify all tables exist: `PRAGMA table_info(agent_tenants);`
- [ ] Verify migration_history shows current version
- [ ] No errors in startup logs

### Existing Database (Incremental Migration Path)
- [ ] Keep existing `build/data/memos_dev.db`
- [ ] Start server
- [ ] Check logs for "start migration" → "end migrate"
- [ ] Verify new columns exist
- [ ] Verify migration_history updated

### Idempotency
- [ ] Start server twice without changes
- [ ] Second start should not re-apply migrations
- [ ] No "column already exists" errors in logs

### Schema Validation
- [ ] `./scripts/validate-migrations.sh` passes
- [ ] `task validate:schema` passes
- [ ] `go test ./store/...` passes

---

## Deployment Checklist

1. [ ] Migration files created (both SQLite and Postgres)
2. [ ] `LATEST.sql` updated to match
3. [ ] `task version:bump` shows correct version
4. [ ] `./scripts/validate-migrations.sh` passes
5. [ ] `task validate:schema` passes
6. [ ] Tested on existing database locally
7. [ ] Tested on fresh database locally
8. [ ] Binary rebuilt (`task build:backend:rag`)
9. [ ] Deployed to staging/production

---

## Troubleshooting

### "no such column: X" Error
**Cause:** Code references a column that doesn't exist. Migrations were skipped.
**Fix:** Check `migration_history` version vs FS version. Apply pending migrations or rebuild with fixed `GetCurrentSchemaVersion()`.

### "no migration history found" Error
**Cause:** Database exists but `migration_history` is empty/missing.
**Fix:** Normal for new databases. `preMigrate()` will apply `LATEST.sql`.

### Migrations Not Applied on Deploy
**Cause:** Binary schema version equals or is less than database version.
**Fix:** Verify `task version:bump` shows correct version. Rebuild and redeploy.

### Schema Mismatch After Deploy
**Cause:** `go:embed` cached old files.
**Fix:** `go clean -cache && task build:backend:rag`

---

## Historical Context

| Bug | Issue | Root Cause |
|-----|-------|-----------|
| 008 | Unique constraint failure on migration 28 | Existing duplicate tickets blocked unique index creation |
| 009 | Migration 28 hotfix | Added CTE deduplication before index creation |
| 015 | Migration history table not found in tests | Tests use fresh in-memory DB (by design) |
| 045 | Migrations silently skipped | `GetCurrentSchemaVersion()` only scanned current minor version directory |
| 046 | Chat widget not appearing + no automation | Hardcoded chatBaseUrl in Hugo + no tooling to prevent version drift |

---

## See Also

- [DOCS_DATABASE_MIGRATION.MD](./DOCS_DATABASE_MIGRATION.MD) — Deprecated, retained for historical reference
- [AGENTS.md](../AGENTS.md) — Migration quick reference (updated to point here)
- `store/migrator.go` — Migration implementation
- `scripts/bump-version.sh` — Version derivation from filesystem
- `scripts/create-migration.sh` — Migration file creation with auto-bump
- `scripts/validate-migrations.sh` — LATEST.sql sync validation
```

---

## Adversarial Review Prompt

> Review this plan for Bug 046 (chat widget not appearing + migration automation) with an adversarial mindset. Focus on:
>
> 1. **Hugo template correctness**: The 3-tier fallback `{{ .Params.chatBaseUrl | default site.Params.bchat.baseUrl | default "http://localhost:8081" }}` — does Hugo's `| default` pipe treat empty strings as falsy? What if someone sets `chatBaseUrl: ""` in front matter? Does it fall through or use the empty string?
> 2. **Security of defaults**: The hardcoded fallback is `http://localhost:8081`. Is this the right security posture? What if a production deployment accidentally omits the hugo.yaml override — the widget would try to load from localhost (fails silently, which is safe but confusing). Should we log a warning?
> 3. **Version bump script robustness**: `bump-version.sh` uses `sed -i` to update `version.go`. What if the file format changes (e.g., someone adds a comment containing the version string)? Is the regex specific enough? Should we use a more structured approach (e.g., Go's `go/ast` package)?
> 4. **Migration script race condition**: If two developers run `task migrate:new` simultaneously, they could get the same patch number. How likely is this in practice? Is the collision window acceptable for a solo/small-team project?
> 5. **Postgres parity**: The `create-migration.sh` creates files in both `sqlite/` and `postgres/` directories. But Postgres migrations may need different SQL (e.g., `BYTEA` vs `BLOB`). The script creates identical files. Should it prompt for differences, or is copy-then-edit the expected workflow?
> 6. **LATEST.sql update is manual**: The plan says "update LATEST.sql" as step 3 in adding a migration. This is the same manual step that caused bugs 008, 009, 015, 045. Should the `create-migration.sh` script auto-update LATEST.sql (at least for simple `ALTER TABLE ADD COLUMN` cases)?
> 7. **Migration guide completeness**: Does the guide cover all the gotchas from past bugs? Is there anything from bugs 008, 009, 015 that should be explicitly called out?
> 8. **Order of operations**: The plan says bug 045's migrator.go fix must be applied for the widget to work. Should the plan explicitly gate Steps 8-12 (Hugo changes) on bug 045 being deployed first? Or can they be deployed independently?
> 9. **Testing the automation**: How do we test that `bump-version.sh` and `create-migration.sh` work correctly? Should there be a test or CI step for the scripts themselves?
>
> Provide specific recommendations to strengthen the plan.
```
