#!/usr/bin/env bash
# run-tests.sh — Test runner for automation scripts.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TEST_DIR="$SCRIPT_DIR/bump-version"
PASS=0
FAIL=0

assert_exit() {
    local desc="$1"
    local expected="$2"
    local actual="$3"
    if [ "$actual" -eq "$expected" ] 2>/dev/null; then
        echo "  PASS: $desc (exit=$actual)"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $desc (expected exit=$expected, got=$actual)"
        FAIL=$((FAIL + 1))
    fi
}

assert_contains() {
    local desc="$1"
    local haystack="$2"
    local needle="$3"
    if echo "$haystack" | grep -q "$needle"; then
        echo "  PASS: $desc"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $desc (output does not contain '$needle')"
        FAIL=$((FAIL + 1))
    fi
}

run_cmd() {
    set +e
    OUTPUT=$("$@" 2>&1)
    EXIT_CODE=$?
    set -e
}

echo "=== Script Tests ==="
echo ""

# Test 1: bump-version.sh — version mismatch is expected (0.31.0 vs 0.33.x)
echo "Test 1: bump-version.sh detects version mismatch"
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT
mkdir -p "$TMPDIR/internal/version"
cp "$TEST_DIR/version.go.fixture" "$TMPDIR/internal/version/version.go"
mkdir -p "$TMPDIR/store/migration/sqlite"
cp -r "$TEST_DIR/migration-tree/"* "$TMPDIR/store/migration/sqlite/"
mkdir -p "$TMPDIR/scripts"
ln -sf "$REPO_ROOT/scripts/bump-version.sh" "$TMPDIR/scripts/bump-version.sh"

run_cmd bash "$TMPDIR/scripts/bump-version.sh"
assert_exit "bump-version.sh exits 1 (version mismatch)" 1 $EXIT_CODE
assert_contains "shows computed version" "$OUTPUT" "0.33.2"
assert_contains "shows current version.go" "$OUTPUT" "0.31.0"
assert_contains "shows MISMATCH" "$OUTPUT" "MISMATCH"
echo ""

# Test 2: create-migration.sh --dry-run
echo "Test 2: create-migration.sh --dry-run"
run_cmd bash "$REPO_ROOT/scripts/create-migration.sh" test_migration --dry-run
assert_exit "exits 0" 0 $EXIT_CODE
assert_contains "shows migration name" "$OUTPUT" "test_migration"
echo ""

# Test 3: create-migration.sh validates name
echo "Test 3: create-migration.sh validates input"
run_cmd bash "$REPO_ROOT/scripts/create-migration.sh" "INVALID NAME!" --dry-run
assert_exit "rejects invalid name" 1 $EXIT_CODE

run_cmd bash "$REPO_ROOT/scripts/create-migration.sh" --dry-run
assert_exit "rejects missing name" 1 $EXIT_CODE

run_cmd bash "$REPO_ROOT/scripts/create-migration.sh" "UPPERCASE" --dry-run
assert_exit "rejects uppercase" 1 $EXIT_CODE
echo ""

# Test 4: validate-parity.sh — passes on known-good pair
echo "Test 4: validate-parity.sh passes on known-good pair"
TMPDIR2=$(mktemp -d)
mkdir -p "$TMPDIR2/store/migration/sqlite/0.99" "$TMPDIR2/store/migration/postgres/0.99"
cp "$TEST_DIR/latest-sqlite.sql.fixture" "$TMPDIR2/store/migration/sqlite/LATEST.sql"
cp "$TEST_DIR/latest-postgres.sql.fixture" "$TMPDIR2/store/migration/postgres/LATEST.sql"
cp "$TEST_DIR/migration-tree/0.33/00__test.sql" "$TMPDIR2/store/migration/sqlite/0.99/00__test.sql"
cp "$TEST_DIR/migration-tree/0.33/00__test.sql" "$TMPDIR2/store/migration/postgres/0.99/00__test.sql"

run_cmd bash "$REPO_ROOT/scripts/validate-parity.sh" --repo-root "$TMPDIR2"
assert_exit "validate-parity.sh passes on known-good pair" 0 $EXIT_CODE
echo ""

# Test 5: validate-parity.sh detects schema drift
echo "Test 5: validate-parity.sh detects schema drift"
cp "$TEST_DIR/latest-postgres-drift.sql.fixture" "$TMPDIR2/store/migration/postgres/LATEST.sql"

run_cmd bash "$REPO_ROOT/scripts/validate-parity.sh" --repo-root "$TMPDIR2"
# Exit code 2 = schema differences only (warn), 3 = both file-list + schema
if [ "$EXIT_CODE" -ge 2 ]; then
    echo "  PASS: validate-parity.sh detects drift (exit=$EXIT_CODE)"
    PASS=$((PASS + 1))
else
    echo "  FAIL: validate-parity.sh should detect drift (exit=$EXIT_CODE)"
    FAIL=$((FAIL + 1))
fi
echo ""

# Test 6: validate-parity.sh detects file-list mismatch
echo "Test 6: validate-parity.sh detects file-list mismatch"
TMPDIR3=$(mktemp -d)
mkdir -p "$TMPDIR3/store/migration/sqlite/0.99" "$TMPDIR3/store/migration/postgres/0.99"
cp "$TEST_DIR/migration-tree/0.33/00__test.sql" "$TMPDIR3/store/migration/sqlite/0.99/00__test.sql"
cp "$TEST_DIR/migration-tree/0.33/01__test2.sql" "$TMPDIR3/store/migration/sqlite/0.99/01__test2.sql"
# Only copy one file to postgres (mismatch)
cp "$TEST_DIR/migration-tree/0.33/00__test.sql" "$TMPDIR3/store/migration/postgres/0.99/00__test.sql"
# Add LATEST.sql files to avoid errors
cp "$TEST_DIR/latest-sqlite.sql.fixture" "$TMPDIR3/store/migration/sqlite/LATEST.sql"
cp "$TEST_DIR/latest-postgres.sql.fixture" "$TMPDIR3/store/migration/postgres/LATEST.sql"

run_cmd bash "$REPO_ROOT/scripts/validate-parity.sh" --repo-root "$TMPDIR3"
# Exit code 1 = file-list differences (fail)
if [ "$EXIT_CODE" -ge 1 ]; then
    echo "  PASS: validate-parity.sh detects file-list mismatch (exit=$EXIT_CODE)"
    PASS=$((PASS + 1))
else
    echo "  FAIL: validate-parity.sh should detect file-list mismatch (exit=$EXIT_CODE)"
    FAIL=$((FAIL + 1))
fi
echo ""

# Summary
echo "=== Results ==="
echo "  Passed: $PASS"
echo "  Failed: $FAIL"
if [ "$FAIL" -gt 0 ]; then
    echo "  STATUS: FAIL"
    exit 1
else
    echo "  STATUS: PASS"
    exit 0
fi
