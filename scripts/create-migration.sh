#!/usr/bin/env bash
# create-migration.sh — Creates migration file templates for both SQLite and
# Postgres drivers. No auto-generation — developer writes SQL for each driver.
#
# Usage: ./scripts/create-migration.sh <migration_name> [--dry-run]
# Example: ./scripts/create-migration.sh add_widget_config

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SQLITE_DIR="$REPO_ROOT/store/migration/sqlite"
POSTGRES_DIR="$REPO_ROOT/store/migration/postgres"

# Portable version comparison for "0.NN" format (no sort -V needed)
version_gt() {
    local v1=$(echo "$1" | sed 's/^0\.//')
    local v2=$(echo "$2" | sed 's/^0\.//')
    [ "${v1:-0}" -gt "${v2:-0}" ] 2>/dev/null
}

DRY_RUN=false
MIGRATION_NAME=""

# Parse arguments
while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        *)
            if [ -z "$MIGRATION_NAME" ]; then
                MIGRATION_NAME="$1"
            else
                echo "ERROR: Unexpected argument: $1"
                echo "Usage: $0 <migration_name> [--dry-run]"
                exit 1
            fi
            shift
            ;;
    esac
done

if [ -z "$MIGRATION_NAME" ]; then
    echo "ERROR: Migration name is required"
    echo "Usage: $0 <migration_name> [--dry-run]"
    echo "Example: $0 add_widget_config"
    exit 1
fi

# Validate name is snake_case
if ! [[ "$MIGRATION_NAME" =~ ^[a-z][a-z0-9_]*$ ]]; then
    echo "ERROR: Migration name must be snake_case (lowercase letters, digits, underscores)"
    echo "  Got: $MIGRATION_NAME"
    exit 1
fi

# Find the latest migration directory
find_latest_dir() {
    local latest=""
    for dir in "$SQLITE_DIR"/*/; do
        local dirname
        dirname=$(basename "$dir")
        if ! [[ "$dirname" =~ ^0\.[0-9]+$ ]]; then
            continue
        fi
        if [ -z "$latest" ] || version_gt "$dirname" "$latest"; then
            latest="$dirname"
        fi
    done
    echo "$latest"
}

# Find the next patch number in a directory
find_next_patch() {
    local dir="$1"
    local max=0
    for f in "$dir"/*.sql; do
        [ -f "$f" ] || continue
        local basename
        basename=$(basename "$f")
        local patch_num
        patch_num=$(echo "$basename" | grep -oE '^[0-9]+' || echo "0")
        if [ "$patch_num" -gt "$max" ] 2>/dev/null; then
            max="$patch_num"
        fi
    done
    printf "%02d" $((max + 1))
}

# Show version info
"$SCRIPT_DIR/bump-version.sh" 2>/dev/null || true
echo ""

LATEST_DIR=$(find_latest_dir)
if [ -z "$LATEST_DIR" ]; then
    echo "ERROR: No migration directories found in $SQLITE_DIR"
    exit 1
fi

NEXT_PATCH=$(find_next_patch "$SQLITE_DIR/$LATEST_DIR")
TODAY=$(date +%Y-%m-%d)
FILENAME="${NEXT_PATCH}__${MIGRATION_NAME}.sql"

SQLITE_PATH="$SQLITE_DIR/$LATEST_DIR/$FILENAME"
POSTGRES_PATH="$POSTGRES_DIR/$LATEST_DIR/$FILENAME"

echo "Creating migration in $LATEST_DIR:"
echo "  SQLite:  $SQLITE_PATH"
echo "  Postgres: $POSTGRES_PATH"
echo ""

if [ "$DRY_RUN" = true ]; then
    echo "[DRY RUN] Would create:"
    echo "  $SQLITE_PATH"
    echo "  $POSTGRES_PATH"
    exit 0
fi

# Ensure directories exist
mkdir -p "$SQLITE_DIR/$LATEST_DIR"
mkdir -p "$POSTGRES_DIR/$LATEST_DIR"

# Check if files already exist
if [ -f "$SQLITE_PATH" ]; then
    echo "ERROR: SQLite file already exists: $SQLITE_PATH"
    exit 1
fi
if [ -f "$POSTGRES_PATH" ]; then
    echo "ERROR: Postgres file already exists: $POSTGRES_PATH"
    exit 1
fi

# Create SQLite template
cat > "$SQLITE_PATH" << EOF
-- Migration: $MIGRATION_NAME
-- Driver: sqlite
-- Date: $TODAY
--
-- TODO: Write migration SQL here.
-- See docs/TYPE_MAPPING.md for SQLite-Postgres type mapping.

EOF

# Create Postgres template
cat > "$POSTGRES_PATH" << EOF
-- Migration: $MIGRATION_NAME
-- Driver: postgres
-- Date: $TODAY
--
-- TODO: Write migration SQL here.
-- See docs/TYPE_MAPPING.md for SQLite-Postgres type mapping.

EOF

echo "Created:"
echo "  $SQLITE_PATH"
echo "  $POSTGRES_PATH"
echo ""
echo "Next steps:"
echo "  1. Write migration SQL in both files"
echo "  2. Update LATEST.sql for both drivers"
echo "  3. Run: task validate:parity"
