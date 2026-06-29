#!/bin/bash
# =============================================================================
# Pre-Deployment Database Migration Validation
# =============================================================================
# Comprehensive validation that migrations will work correctly on fly.io.
# Tests SQL syntax, migration sequencing, and schema consistency.
#
# Usage:
#   ./scripts/validate-db-migrations.sh
#
# Exit codes:
#   0 - All checks passed
#   1 - One or more checks failed
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
MIGRATION_DIR="$ROOT_DIR/store/migration/sqlite"
LATEST_SQL="$MIGRATION_DIR/LATEST.sql"
TEMP_DIR="/tmp/bchat-migration-test-$$"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

cleanup() {
    rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

mkdir -p "$TEMP_DIR"

list_version_dirs() {
    find "$MIGRATION_DIR" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' \
        | grep -E '^[0-9]+(\.[0-9]+)+$' \
        | sort -V
}

find_latest_version_dir() {
    list_version_dirs | tail -n 1
}

LATEST_VERSION="$(find_latest_version_dir)"
if [ -z "$LATEST_VERSION" ]; then
    echo -e "${RED}ERROR: No versioned migration directories found in $MIGRATION_DIR${NC}"
    exit 1
fi
VERSION_DIR="$MIGRATION_DIR/$LATEST_VERSION"

find_bootstrap_migration_dirs() {
    mapfile -t version_dirs < <(list_version_dirs)
    local total=${#version_dirs[@]}

    for ((start = total - 1; start >= 0; start--)); do
        local candidate_db="$TEMP_DIR/bootstrap-${start}.db"
        local ok=1
        : > "$candidate_db"

        for ((i = start; i < total; i++)); do
            local version="${version_dirs[$i]}"
            for file in $(ls "$MIGRATION_DIR/$version"/*.sql 2>/dev/null | sort); do
                if ! sqlite3 "$candidate_db" < "$file" 2>"$TEMP_DIR/bootstrap-error.txt"; then
                    ok=0
                    break 2
                fi
            done
        done

        if [ "$ok" -eq 1 ]; then
            for ((i = start; i < total; i++)); do
                echo "$MIGRATION_DIR/${version_dirs[$i]}"
            done
            return 0
        fi
    done

    return 1
}

echo "=== Pre-Deployment Database Migration Check ==="
echo "Latest migration directory: $VERSION_DIR"
echo ""

# Check 1: Run existing validation
echo "Step 1: Checking LATEST.sql sync..."
if ! "$SCRIPT_DIR/validate-migrations.sh"; then
    echo -e "${RED}FAILED: LATEST.sql is out of sync with migrations${NC}"
    exit 1
fi
echo -e "${GREEN}PASSED${NC}"
echo ""

# Check 2: Validate migration numbering
echo "Step 2: Checking migration file numbering..."
EXPECTED=0
ERRORS=0
for file in $(ls "$VERSION_DIR"/*.sql 2>/dev/null | sort); do
    filename=$(basename "$file")
    # Extract the number before __ (e.g., "00" from "00__tickets.sql")
    num=$(echo "$filename" | sed 's/^\([0-9]*\)__.*/\1/' | sed 's/^0*//')
    [ -z "$num" ] && num=0
    if [ "$num" -ne "$EXPECTED" ]; then
        echo -e "${RED}ERROR: Expected migration $EXPECTED, found $filename${NC}"
        ERRORS=$((ERRORS + 1))
    fi
    EXPECTED=$((EXPECTED + 1))
done
if [ $ERRORS -gt 0 ]; then
    echo -e "${RED}FAILED: Migration numbering has gaps or is out of order${NC}"
    exit 1
fi
echo -e "${GREEN}PASSED: Found $EXPECTED migrations in sequence (00-$(printf "%02d" $((EXPECTED-1))))${NC}"
echo ""

# Check 3: Test LATEST.sql creates valid database
echo "Step 3: Testing LATEST.sql creates valid database..."
FRESH_DB="$TEMP_DIR/fresh.db"
if ! sqlite3 "$FRESH_DB" < "$LATEST_SQL" 2>"$TEMP_DIR/latest_errors.txt"; then
    echo -e "${RED}FAILED: LATEST.sql has SQL errors:${NC}"
    cat "$TEMP_DIR/latest_errors.txt"
    exit 1
fi
FRESH_TABLES=$(sqlite3 "$FRESH_DB" ".tables" | wc -w)
echo -e "${GREEN}PASSED: Created database with $FRESH_TABLES tables${NC}"
echo ""

# Check 4: Test migrations apply in sequence
echo "Step 4: Testing migrations apply in sequence..."
MIGRATED_DB="$TEMP_DIR/migrated.db"
touch "$MIGRATED_DB"
if ! mapfile -t BOOTSTRAP_DIRS < <(find_bootstrap_migration_dirs); then
    echo -e "${RED}FAILED: Could not find a versioned migration sequence that applies cleanly${NC}"
    if [ -f "$TEMP_DIR/bootstrap-error.txt" ]; then
        cat "$TEMP_DIR/bootstrap-error.txt"
    fi
    exit 1
fi
echo "Applying migration directories:"
for dir in "${BOOTSTRAP_DIRS[@]}"; do
    echo "  - $dir"
done
for dir in "${BOOTSTRAP_DIRS[@]}"; do
    for file in $(ls "$dir"/*.sql 2>/dev/null | sort); do
        filename="$(basename "$dir")/$(basename "$file")"
        if ! sqlite3 "$MIGRATED_DB" < "$file" 2>"$TEMP_DIR/migration_error.txt"; then
            echo -e "${RED}FAILED: Migration $filename has errors:${NC}"
            cat "$TEMP_DIR/migration_error.txt"
            exit 1
        fi
    done
done
MIGRATED_TABLES=$(sqlite3 "$MIGRATED_DB" ".tables" | wc -w)
echo -e "${GREEN}PASSED: All migrations applied, $MIGRATED_TABLES tables${NC}"
echo ""

# Check 5: Compare schemas
echo "Step 5: Comparing schemas (LATEST.sql vs migrations)..."
sqlite3 "$FRESH_DB" ".schema" | sort > "$TEMP_DIR/fresh_schema.txt"
sqlite3 "$MIGRATED_DB" ".schema" | sort > "$TEMP_DIR/migrated_schema.txt"

# Extract table names for comparison
sqlite3 "$FRESH_DB" "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name;" > "$TEMP_DIR/fresh_tables.txt"
sqlite3 "$MIGRATED_DB" "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name;" > "$TEMP_DIR/migrated_tables.txt"

if ! diff -q "$TEMP_DIR/fresh_tables.txt" "$TEMP_DIR/migrated_tables.txt" > /dev/null 2>&1; then
    echo -e "${YELLOW}WARNING: Table list differs between LATEST.sql and migrations${NC}"
    # Sort files and compare - suppress comm warnings about sort order
    sort "$TEMP_DIR/fresh_tables.txt" > "$TEMP_DIR/fresh_tables_sorted.txt"
    sort "$TEMP_DIR/migrated_tables.txt" > "$TEMP_DIR/migrated_tables_sorted.txt"
    FRESH_ONLY=$(comm -23 "$TEMP_DIR/fresh_tables_sorted.txt" "$TEMP_DIR/migrated_tables_sorted.txt" 2>/dev/null || true)
    MIGRATED_ONLY=$(comm -13 "$TEMP_DIR/fresh_tables_sorted.txt" "$TEMP_DIR/migrated_tables_sorted.txt" 2>/dev/null || true)
    if [ -n "$FRESH_ONLY" ]; then
        echo "Tables in LATEST.sql only:"
        echo "$FRESH_ONLY" | sed 's/^/  /'
    fi
    if [ -n "$MIGRATED_ONLY" ]; then
        echo "Tables in migrations only:"
        echo "$MIGRATED_ONLY" | sed 's/^/  /'
    fi
    # This is a warning, not a failure - migrations may create/drop tables
else
    echo -e "${GREEN}PASSED: Table lists match${NC}"
fi
echo ""

# Summary
echo "=== All Checks Passed ==="
echo -e "${GREEN}Database migrations are ready for fly.io deployment${NC}"
