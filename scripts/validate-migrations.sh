#!/bin/bash
# =============================================================================
# Migration Validation Script
# =============================================================================
# Validates that LATEST.sql is in sync with individual migration files.
# Prevents deployment issues where new databases are missing tables/columns.
#
# Usage:
#   ./scripts/validate-migrations.sh
#
# Exit codes:
#   0 - LATEST.sql is in sync
#   1 - LATEST.sql is missing tables or columns
# =============================================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
MIGRATION_ROOT="$PROJECT_ROOT/store/migration/sqlite"
LATEST_SQL="$MIGRATION_ROOT/LATEST.sql"

find_latest_version_dir() {
    find "$MIGRATION_ROOT" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' \
        | grep -E '^[0-9]+(\.[0-9]+)+$' \
        | sort -V \
        | tail -n 1
}

LATEST_VERSION="$(find_latest_version_dir)"
if [ -z "$LATEST_VERSION" ]; then
    echo -e "${RED}ERROR: No versioned migration directories found in $MIGRATION_ROOT${NC}"
    exit 1
fi
MIGRATIONS_DIR="$MIGRATION_ROOT/$LATEST_VERSION"

echo "Validating LATEST.sql against migrations..."
echo "Using migration directory: $MIGRATIONS_DIR"
echo ""

# Check files exist
if [ ! -f "$LATEST_SQL" ]; then
    echo -e "${RED}ERROR: LATEST.sql not found at $LATEST_SQL${NC}"
    exit 1
fi

if [ ! -d "$MIGRATIONS_DIR" ]; then
    echo -e "${RED}ERROR: Migrations directory not found at $MIGRATIONS_DIR${NC}"
    exit 1
fi

# Track issues
MISSING_TABLES=()
MISSING_COLUMNS=()

# Read LATEST.sql content (lowercase for case-insensitive matching)
LATEST_CONTENT=$(cat "$LATEST_SQL" | tr '[:upper:]' '[:lower:]')

# Tables to ignore (temporary tables, renamed tables, etc.)
# These are created during migrations but don't persist
# Patterns: *_new, *_old, *_backup, *_temp, *_tmp
IGNORE_TABLES="_new$|_old$|_backup$|_temp$|_tmp$"

# Process each migration file
for migration in "$MIGRATIONS_DIR"/*.sql; do
    filename=$(basename "$migration")

    # Skip if not a numbered migration file
    if [[ ! "$filename" =~ ^[0-9]+__.+\.sql$ ]]; then
        continue
    fi

    # Read migration content
    content=$(cat "$migration")

    # Check for CREATE TABLE statements
    # Match: CREATE TABLE [IF NOT EXISTS] table_name
    while read -r line; do
        if [[ "$line" =~ CREATE[[:space:]]+TABLE[[:space:]]+(IF[[:space:]]+NOT[[:space:]]+EXISTS[[:space:]]+)?([a-zA-Z_][a-zA-Z0-9_]*)[[:space:]]*\( ]]; then
            table="${BASH_REMATCH[2]}"
            table_lower=$(echo "$table" | tr '[:upper:]' '[:lower:]')

            # Skip ignored tables (temp tables, etc.)
            if [[ "$table_lower" =~ ($IGNORE_TABLES) ]]; then
                continue
            fi

            # Skip if migration also contains DROP TABLE for same table (table recreation)
            if echo "$content" | grep -qi "DROP TABLE.*$table"; then
                continue
            fi

            # Skip if migration renames this table (it's temporary)
            if echo "$content" | grep -qi "RENAME TO.*$table"; then
                continue
            fi

            # Check if table exists in LATEST.sql
            if ! echo "$LATEST_CONTENT" | grep -q "create table.*$table_lower"; then
                MISSING_TABLES+=("$table (from $filename)")
            fi
        fi
    done <<< "$content"

    # Check for ALTER TABLE ADD COLUMN statements
    while read -r line; do
        # Match: ALTER TABLE table_name ADD COLUMN column_name
        if [[ "$line" =~ ALTER[[:space:]]+TABLE[[:space:]]+([a-zA-Z_][a-zA-Z0-9_]*)[[:space:]]+ADD[[:space:]]+COLUMN[[:space:]]+([a-zA-Z_][a-zA-Z0-9_]*) ]]; then
            table="${BASH_REMATCH[1]}"
            column="${BASH_REMATCH[2]}"
            column_lower=$(echo "$column" | tr '[:upper:]' '[:lower:]')

            # Skip if this is adding to a temp table
            if [[ "$table" =~ ($IGNORE_TABLES) ]]; then
                continue
            fi

            # Check if column exists in LATEST.sql
            if ! echo "$LATEST_CONTENT" | grep -q "$column_lower"; then
                MISSING_COLUMNS+=("$table.$column (from $filename)")
            fi
        fi
    done <<< "$content"

done

# Report results
ERRORS=0

if [ ${#MISSING_TABLES[@]} -gt 0 ]; then
    echo -e "${RED}Missing tables in LATEST.sql:${NC}"
    for item in "${MISSING_TABLES[@]}"; do
        echo "  - $item"
    done
    echo ""
    ERRORS=1
fi

if [ ${#MISSING_COLUMNS[@]} -gt 0 ]; then
    echo -e "${RED}Missing columns in LATEST.sql:${NC}"
    for item in "${MISSING_COLUMNS[@]}"; do
        echo "  - $item"
    done
    echo ""
    ERRORS=1
fi

if [ $ERRORS -eq 1 ]; then
    echo -e "${YELLOW}Please update store/migration/sqlite/LATEST.sql to include the missing items.${NC}"
    echo -e "${YELLOW}See docs/DOCS_DATABASE_MIGRATION.MD for details.${NC}"
    exit 1
fi

echo -e "${GREEN}✓ LATEST.sql is in sync with all migrations${NC}"
exit 0
