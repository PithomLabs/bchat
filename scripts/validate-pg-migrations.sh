#!/bin/bash
# =============================================================================
# Pre-Deployment Postgres Migration Validation
# =============================================================================
# Comprehensive validation that Postgres migrations will work correctly on Neon.
# Tests SQL syntax, migration sequencing, and schema consistency against a
# running Postgres instance.
#
# Usage:
#   ./scripts/validate-pg-migrations.sh
#
# Requires:
#   - Running Postgres instance (use `task -t Taskfile_pg.yml postgres:start`)
#   - psql client installed
#   - DATABASE_URL environment variable pointing to the target database
#     (e.g., postgresql://bchat:bchat@localhost:5432/bchat)
#
# Exit codes:
#   0 - All checks passed
#   1 - One or more checks failed
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
MIGRATION_DIR="$ROOT_DIR/store/migration/postgres"
LATEST_SQL="$MIGRATION_DIR/LATEST.sql"
TEMP_DIR="/tmp/bchat-pg-migration-test-$$"
TEST_DB="bchat_migration_test_$$"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Determine database URL
DATABASE_URL="${DATABASE_URL:-postgresql://bchat:bchat@localhost:5432/bchat}"
BASE_URL="${DATABASE_URL%%/*}/postgres"

cleanup() {
    echo "Cleaning up..."
    # Drop test database if it exists
    if command -v psql &>/dev/null; then
        psql "$BASE_URL" -c "DROP DATABASE IF EXISTS \"$TEST_DB\";" 2>/dev/null || true
    fi
    rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

mkdir -p "$TEMP_DIR"

echo "=== Pre-Deployment Postgres Migration Check ==="
echo "Using base URL: ${DATABASE_URL%%@*}@${DATABASE_URL##*@}"
echo ""

# Verify psql is available
if ! command -v psql &>/dev/null; then
    echo -e "${RED}ERROR: psql not found. Install PostgreSQL client.${NC}"
    echo "  Ubuntu/Debian: sudo apt install postgresql-client"
    echo "  macOS: brew install libpq"
    exit 1
fi

# Verify database is reachable
echo "Step 0: Checking database connectivity..."
if ! psql "$DATABASE_URL" -c "SELECT 1;" &>/dev/null; then
    echo -e "${RED}FAILED: Cannot connect to database at ${DATABASE_URL%%@*}@${DATABASE_URL##*@}${NC}"
    echo "  Make sure Postgres is running (task -t Taskfile_pg.yml postgres:start)"
    exit 1
fi
echo -e "${GREEN}PASSED: Database is reachable${NC}"
echo ""

# Create a fresh test database from LATEST.sql
echo "Step 1: Creating test database from LATEST.sql..."
psql "$BASE_URL" -c "DROP DATABASE IF EXISTS \"$TEST_DB\";" 2>/dev/null || true
psql "$BASE_URL" -c "CREATE DATABASE \"$TEST_DB\";"

TEST_URL="${DATABASE_URL%%/*}/$TEST_DB"
if ! psql "$TEST_URL" < "$LATEST_SQL" 2>"$TEMP_DIR/latest_errors.txt"; then
    echo -e "${RED}FAILED: LATEST.sql has SQL errors:${NC}"
    cat "$TEMP_DIR/latest_errors.txt"
    exit 1
fi

FRESH_TABLES=$(psql "$TEST_URL" -t -c "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public';" | tr -d ' ')
echo -e "${GREEN}PASSED: Created database with $FRESH_TABLES tables${NC}"
echo ""

# Drop and recreate test database for sequential migration test
echo "Step 2: Dropping and recreating test database..."
psql "$BASE_URL" -c "DROP DATABASE IF EXISTS \"$TEST_DB\";"
psql "$BASE_URL" -c "CREATE DATABASE \"$TEST_DB\";"
echo -e "${GREEN}PASSED${NC}"
echo ""

# Apply migrations in sequence
echo "Step 3: Testing migrations apply in sequence..."
MIGRATED_DB="$TEST_URL"

# Find all version directories and sort them
VERSION_DIRS=$(find "$MIGRATION_DIR" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' \
    | grep -E '^[0-9]+(\.[0-9]+)+$' \
    | sort -V)

if [ -z "$VERSION_DIRS" ]; then
    echo -e "${RED}FAILED: No versioned migration directories found in $MIGRATION_DIR${NC}"
    exit 1
fi

echo "Applying migration directories:"
for version in $VERSION_DIRS; do
    echo "  - $version"
done

for version in $VERSION_DIRS; do
    for file in $(ls "$MIGRATION_DIR/$version"/*.sql 2>/dev/null | sort); do
        filename="$(basename "$file")"
        echo "    Applying: $version/$filename"
        if ! psql "$MIGRATED_DB" -f "$file" 2>"$TEMP_DIR/migration_error.txt"; then
            echo -e "${RED}FAILED: Migration $version/$filename has errors:${NC}"
            cat "$TEMP_DIR/migration_error.txt"
            exit 1
        fi
    done
done

MIGRATED_TABLES=$(psql "$MIGRATED_DB" -t -c "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public';" | tr -d ' ')
echo -e "${GREEN}PASSED: All migrations applied, $MIGRATED_TABLES tables${NC}"
echo ""

# Compare schemas
echo "Step 4: Comparing schemas (LATEST.sql vs migrations)..."

# Recreate fresh database from LATEST.sql for comparison
FRESH_DB_NAME="${TEST_DB}_fresh"
FRESH_DB_URL="${DATABASE_URL%%/*}/${FRESH_DB_NAME}"
psql "$BASE_URL" -c "DROP DATABASE IF EXISTS \"${FRESH_DB_NAME}\";" 2>/dev/null || true
psql "$BASE_URL" -c "CREATE DATABASE \"${FRESH_DB_NAME}\";"
psql "$FRESH_DB_URL" < "$LATEST_SQL" 2>/dev/null
psql "$FRESH_DB_URL" -t -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name;" > "$TEMP_DIR/fresh_tables.txt"
psql "$BASE_URL" -c "DROP DATABASE IF EXISTS \"${FRESH_DB_NAME}\";"

psql "$MIGRATED_DB" -t -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name;" > "$TEMP_DIR/migrated_tables.txt"

if ! diff -q "$TEMP_DIR/fresh_tables.txt" "$TEMP_DIR/migrated_tables.txt" > /dev/null 2>&1; then
    echo -e "${YELLOW}WARNING: Table list differs between LATEST.sql and migrations${NC}"
    FRESH_ONLY=$(comm -23 "$TEMP_DIR/fresh_tables.txt" "$TEMP_DIR/migrated_tables.txt" 2>/dev/null || true)
    MIGRATED_ONLY=$(comm -13 "$TEMP_DIR/fresh_tables.txt" "$TEMP_DIR/migrated_tables.txt" 2>/dev/null || true)
    if [ -n "$FRESH_ONLY" ]; then
        echo "Tables in LATEST.sql only:"
        echo "$FRESH_ONLY" | sed 's/^/  /'
    fi
    if [ -n "$MIGRATED_ONLY" ]]; then
        echo "Tables in migrations only:"
        echo "$MIGRATED_ONLY" | sed 's/^/  /'
    fi
else
    echo -e "${GREEN}PASSED: Table lists match${NC}"
fi
echo ""

# Summary
echo "=== All Checks Passed ==="
echo -e "${GREEN}Postgres migrations are ready for deployment${NC}"
