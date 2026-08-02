#!/usr/bin/env bash
# validate-cockroach-compat.sh — CockroachDB compatibility scanner for
# store/migration/cockroach/**/*.sql.
#
# Two categories (bugs/057/pre_code.md §4.6):
#   FORBIDDEN       (exit 1) — MCP-verified unsupported by CockroachDB
#   REVIEW-REQUIRED (exit 2) — supported only with caveats; must carry an
#                              explicit annotation in the SQL to pass
#
# Best-effort grep scanner, NOT a SQL parser. The authoritative gate is code
# review plus the §4.1 statement-by-statement audit. Must never grow parity
# checks (strict independence — see pre_code.md §4.6).
#
# Usage: ./scripts/validate-cockroach-compat.sh [--repo-root DIR]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
if [ $# -ge 2 ] && [ "$1" = "--repo-root" ]; then
    REPO_ROOT="$(cd "$2" && pwd)"
fi

MIG_DIR="$REPO_ROOT/store/migration/cockroach"
if [ ! -d "$MIG_DIR" ]; then
    echo "PASS: no cockroach migration dir (nothing to scan)"
    exit 0
fi

FILES=$(find "$MIG_DIR" -name "*.sql" -type f | sort)
if [ -z "$FILES" ]; then
    echo "PASS: no cockroach migration files"
    exit 0
fi

FORBIDDEN=(
    'CREATE[[:space:]]+EXTENSION'
    '\bNOTIFY\b|\bLISTEN\b'
    'pg_advisory_lock|pg_advisory_unlock|pg_try_advisory_lock'
    'CREATE[[:space:]]+DOMAIN'
    '\b(int4range|int8range|numrange|tsrange|tstzrange|daterange)\b'
    '\bmacaddr8?\b'
    '\bMONEY\b'
    'CREATE[[:space:]]+TRIGGER'
    '\bDEFERRABLE\b'
    'DROP[[:space:]]+PRIMARY[[:space:]]+KEY'
    'DO[[:space:]]+\$\$'
)

# REVIEW-REQUIRED constructs. Annotation format (attached to the statement):
#   -- cockroach-compat: <why this is safe on CockroachDB>
REVIEW_REQUIRED=(
    'ALTER[[:space:]]+TYPE'
    'UPDATE[[:space:]]+[^;]*[[:space:]]+FROM[[:space:]]'
    '^COPY[[:space:]]'
)

contains_any() {
    local file="$1"
    shift
    local pattern
    for pattern in "$@"; do
        if grep -qiE "$pattern" "$file"; then
            return 0
        fi
    done
    return 1
}

FORBIDDEN_HITS=""
REVIEW_HITS=""

for f in $FILES; do
    if contains_any "$f" "${FORBIDDEN[@]}"; then
        FORBIDDEN_HITS="$FORBIDDEN_HITS $f"
    fi
    if contains_any "$f" "${REVIEW_REQUIRED[@]}"; then
        REVIEW_HITS="$REVIEW_HITS $f"
    fi
done

EXIT_CODE=0

if [ -n "$FORBIDDEN_HITS" ]; then
    echo "FAIL: FORBIDDEN constructs found (unsupported by CockroachDB):"
    for f in $FORBIDDEN_HITS; do
        echo "  $f"
        for pattern in "${FORBIDDEN[@]}"; do
            if grep -qiE "$pattern" "$f"; then
                echo "    - $(grep -iE "$pattern" "$f" | head -1 | sed 's/^[[:space:]]*//')"
            fi
        done
    done
    EXIT_CODE=1
fi

if [ -n "$REVIEW_HITS" ]; then
    if [ "$EXIT_CODE" -ne 1 ]; then
        EXIT_CODE=2
    fi
    echo "WARN: REVIEW-REQUIRED constructs (add -- cockroach-compat: annotation):"
    for f in $REVIEW_HITS; do
        echo "  $f"
        for pattern in "${REVIEW_REQUIRED[@]}"; do
            if grep -qiE "$pattern" "$f"; then
                echo "    - $(grep -iE "$pattern" "$f" | head -1 | sed 's/^[[:space:]]*//')"
            fi
        done
    done
fi

if [ "$EXIT_CODE" -eq 0 ]; then
    echo "PASS: no forbidden or unannotated constructs in cockroach migrations"
fi
exit $EXIT_CODE
