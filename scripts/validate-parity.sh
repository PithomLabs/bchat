#!/usr/bin/env bash
# validate-parity.sh — Cross-driver parity validator for SQLite and Postgres.
#
# Three checks:
#   1. File-list parity (CI gate): Both drivers must have files for each directory
#   2. Schema parity (best-effort lint): LATEST.sql must produce same logical schema
#   3. Historical divergence: Known divergences are documented and skipped
#
# Exit codes:
#   0 = pass
#   1 = file-list differences (fail)
#   2 = schema differences (warn only)
#   3 = both
#
# Usage: ./scripts/validate-parity.sh [--verbose] [--repo-root DIR]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Parse arguments
VERBOSE=false
while [ $# -gt 0 ]; do
    case "$1" in
        --verbose) VERBOSE=true; shift ;;
        --repo-root) REPO_ROOT="$(cd "$2" && pwd)"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

SQLITE_DIR="$REPO_ROOT/store/migration/sqlite"
POSTGRES_DIR="$REPO_ROOT/store/migration/postgres"
SQLITE_LATEST="$SQLITE_DIR/LATEST.sql"
POSTGRES_LATEST="$POSTGRES_DIR/LATEST.sql"

FILE_LIST_ISSUES=0
SCHEMA_ISSUES=0

# Known historical divergences (version:reason)
# SQLite has directories from 0.2-0.33; Postgres starts at 0.19
# These are pre-existing and should not trigger CI failures
# Portable format — grep-based lookup, no bash 4+ required
KNOWN_DIVERGENCES=$(cat << 'DIVERGENCES'
0.2:SQLite-only early migration
0.3:SQLite-only early migration
0.4:SQLite-only early migration
0.5:SQLite-only early migration
0.6:SQLite-only early migration
0.7:SQLite-only early migration
0.8:SQLite-only early migration
0.9:SQLite-only early migration
0.10:SQLite-only early migration
0.11:SQLite-only early migration
0.12:SQLite-only early migration
0.13:SQLite-only early migration
0.14:SQLite-only early migration
0.15:SQLite-only early migration
0.16:SQLite-only early migration
0.17:SQLite-only early migration
0.18:SQLite-only early migration
0.25:Catch-up migration: SQLite has 36 files, Postgres has 2
0.26:Catch-up migration: SQLite has 7 files, Postgres has 4
0.28:Catch-up migration: SQLite has 3 files, Postgres has 1
0.29:Catch-up migration: SQLite has 1 file, Postgres has 2
0.30:Catch-up migration: SQLite has 2 files, Postgres has 5
0.33:Postgres has system_secret table; SQLite has different migration
DIVERGENCES
)

is_known_divergence() {
    local ver="$1"
    echo "$KNOWN_DIVERGENCES" | grep -q "^${ver}:" 2>/dev/null
}

get_divergence_reason() {
    local ver="$1"
    echo "$KNOWN_DIVERGENCES" | grep "^${ver}:" | head -1 | sed 's/^[^:]*://'
}

echo "=== Cross-Driver Parity Validator ==="
echo ""

# --- Check 1: File-list parity ---
echo "Check 1: File-list parity"

check_file_list_parity() {
    local driver_name="$1"
    local driver_dir="$2"
    local other_dir="$3"

    for dir in "$driver_dir"/*/; do
        local dirname
        dirname=$(basename "$dir")
        if ! [[ "$dirname" =~ ^0\.[0-9]+$ ]]; then
            continue
        fi

        # Check known divergence
        if is_known_divergence "$dirname"; then
            if [ "$VERBOSE" = true ]; then
                echo "  SKIP $dirname (known divergence: $(get_divergence_reason "$dirname"))"
            fi
            continue
        fi

        # Check if corresponding directory exists in other driver
        if [ ! -d "$other_dir/$dirname" ]; then
            echo "  MISSING: $driver_name/$dirname/ (exists in $driver_name but not in other driver)"
            FILE_LIST_ISSUES=1
            continue
        fi

        # Check file counts match
        local driver_count
        driver_count=$(find "$driver_dir/$dirname" -name "*.sql" -type f | wc -l)
        local other_count
        other_count=$(find "$other_dir/$dirname" -name "*.sql" -type f 2>/dev/null | wc -l)

        if [ "$driver_count" -ne "$other_count" ]; then
            echo "  MISMATCH: $dirname/ has $driver_count files in $driver_name, $other_count in other driver"
            if [ "$VERBOSE" = true ]; then
                echo "    $driver_name files:"
                find "$driver_dir/$dirname" -name "*.sql" -type f -exec basename {} \; | sort | sed 's/^/      /'
                echo "    other driver files:"
                find "$other_dir/$dirname" -name "*.sql" -type f -exec basename {} \; 2>/dev/null | sort | sed 's/^/      /'
            fi
            FILE_LIST_ISSUES=1
            continue
        fi

        # Check each file exists in other driver
        for f in "$driver_dir/$dirname"/*.sql; do
            [ -f "$f" ] || continue
            local fname
            fname=$(basename "$f")
            if [ ! -f "$other_dir/$dirname/$fname" ]; then
                echo "  MISSING: $driver_name/$dirname/$fname (not in other driver)"
                FILE_LIST_ISSUES=1
            fi
        done

        if [ "$VERBOSE" = true ] && [ "$FILE_LIST_ISSUES" -eq 0 ]; then
            echo "  OK: $dirname/ ($driver_count files)"
        fi
    done
}

check_file_list_parity "sqlite" "$SQLITE_DIR" "$POSTGRES_DIR"
check_file_list_parity "postgres" "$POSTGRES_DIR" "$SQLITE_DIR"

if [ "$FILE_LIST_ISSUES" -eq 0 ]; then
    echo "  PASS: File lists are in sync"
fi
echo ""

# --- Check 2: Schema parity (best-effort lint) ---
echo "Check 2: Schema parity (best-effort lint)"

if [ ! -f "$SQLITE_LATEST" ]; then
    echo "  ERROR: $SQLITE_LATEST not found"
    exit 1
fi
if [ ! -f "$POSTGRES_LATEST" ]; then
    echo "  ERROR: $POSTGRES_LATEST not found"
    exit 1
fi

# Extract table names from CREATE TABLE statements
extract_tables() {
    local file="$1"
    grep -oE 'CREATE[[:space:]]+TABLE[[:space:]]+(IF[[:space:]]+NOT[[:space:]]+EXISTS[[:space:]]+)?["`'"'"']?(\w+)["`'"'"']?' "$file" \
        | sed -E 's/CREATE[[:space:]]+TABLE[[:space:]]+(IF[[:space:]]+NOT[[:space:]]+EXISTS[[:space:]]+)?["`'"'"']?(\w+)["`'"'"']?.*/\2/i' \
        | tr '[:upper:]' '[:lower:]' \
        | sort -u
}

# Extract index names
extract_indexes() {
    local file="$1"
    grep -oE 'CREATE[[:space:]]+(UNIQUE[[:space:]]+)?INDEX[[:space:]]+(IF[[:space:]]+NOT[[:space:]]+EXISTS[[:space:]]+)?["`'"'"']?(\w+)["`'"'"']?' "$file" \
        | sed -E 's/CREATE[[:space:]]+(UNIQUE[[:space:]]+)?INDEX[[:space:]]+(IF[[:space:]]+NOT[[:space:]]+EXISTS[[:space:]]+)?["`'"'"']?(\w+)["`'"'"']?.*/\3/i' \
        | tr '[:upper:]' '[:lower:]' \
        | sort -u
}

SQLITE_TABLES=$(extract_tables "$SQLITE_LATEST")
POSTGRES_TABLES=$(extract_tables "$POSTGRES_LATEST")

# Compare table lists
MISSING_IN_POSTGRES=$(comm -23 <(echo "$SQLITE_TABLES") <(echo "$POSTGRES_TABLES") || true)
MISSING_IN_SQLITE=$(comm -13 <(echo "$SQLITE_TABLES") <(echo "$POSTGRES_TABLES") || true)

if [ -n "$MISSING_IN_POSTGRES" ]; then
    echo "  Tables in SQLite but NOT in Postgres:"
    echo "$MISSING_IN_POSTGRES" | sed 's/^/    /'
    SCHEMA_ISSUES=1
fi

if [ -n "$MISSING_IN_SQLITE" ]; then
    echo "  Tables in Postgres but NOT in SQLite:"
    echo "$MISSING_IN_SQLITE" | sed 's/^/    /'
    SCHEMA_ISSUES=1
fi

# Compare indexes
SQLITE_INDEXES=$(extract_indexes "$SQLITE_LATEST")
POSTGRES_INDEXES=$(extract_indexes "$POSTGRES_LATEST")

MISSING_IDX_POSTGRES=$(comm -23 <(echo "$SQLITE_INDEXES") <(echo "$POSTGRES_INDEXES") || true)
MISSING_IDX_SQLITE=$(comm -13 <(echo "$SQLITE_INDEXES") <(echo "$POSTGRES_INDEXES") || true)

if [ -n "$MISSING_IDX_POSTGRES" ]; then
    echo "  Indexes in SQLite but NOT in Postgres:"
    echo "$MISSING_IDX_POSTGRES" | sed 's/^/    /'
    SCHEMA_ISSUES=1
fi

if [ -n "$MISSING_IDX_SQLITE" ]; then
    echo "  Indexes in Postgres but NOT in SQLite:"
    echo "$MISSING_IDX_SQLITE" | sed 's/^/    /'
    SCHEMA_ISSUES=1
fi

if [ "$VERBOSE" = true ]; then
    SQLITE_COUNT=$(echo "$SQLITE_TABLES" | grep -c . || true)
    POSTGRES_COUNT=$(echo "$POSTGRES_TABLES" | grep -c . || true)
    echo "  Table counts: SQLite=$SQLITE_COUNT, Postgres=$POSTGRES_COUNT"
fi

if [ "$SCHEMA_ISSUES" -eq 0 ]; then
    echo "  PASS: Schema structure matches (best-effort check)"
else
    echo "  WARN: Schema differences detected (best-effort lint — verify manually)"
    echo "  Note: Shell-level SQL parsing is limited. See docs/TYPE_MAPPING.md for details."
fi
echo ""

# --- Summary ---
echo "=== Summary ==="
if [ "$FILE_LIST_ISSUES" -ne 0 ]; then
    echo "FAIL: File-list parity issues found"
fi
if [ "$SCHEMA_ISSUES" -ne 0 ]; then
    echo "WARN: Schema differences detected (best-effort lint)"
fi
if [ "$FILE_LIST_ISSUES" -eq 0 ] && [ "$SCHEMA_ISSUES" -eq 0 ]; then
    echo "PASS: All checks passed"
fi

EXIT_CODE=$((FILE_LIST_ISSUES * 1 + SCHEMA_ISSUES * 2))
exit $EXIT_CODE
