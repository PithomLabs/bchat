#!/usr/bin/env bash
# bump-version.sh — Informational script that shows the version derived from
# the migration filesystem. Does NOT modify any files.
#
# The version is derived at runtime by GetCurrentSchemaVersion() in migrator.go,
# which scans all directories under store/migration/sqlite/. This script
# replicates that logic in shell for developer convenience.
#
# Usage: ./scripts/bump-version.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MIGRATION_DIR="$REPO_ROOT/store/migration/sqlite"
VERSION_FILE="$REPO_ROOT/internal/version/version.go"

# Portable version comparison for "0.NN" format (no sort -V needed)
version_gt() {
    local v1=$(echo "$1" | sed 's/^0\.//')
    local v2=$(echo "$2" | sed 's/^0\.//')
    [ "${v1:-0}" -gt "${v2:-0}" ] 2>/dev/null
}

# Find the highest migration directory (e.g., "0.33")
find_latest_dir() {
    local latest=""
    for dir in "$MIGRATION_DIR"/*/; do
        local dirname
        dirname=$(basename "$dir")
        # Skip non-version directories
        if ! [[ "$dirname" =~ ^0\.[0-9]+$ ]]; then
            continue
        fi
        if [ -z "$latest" ] || version_gt "$dirname" "$latest"; then
            latest="$dirname"
        fi
    done
    echo "$latest"
}

# Find the highest patch number in a directory (e.g., "01" from "01__add_column.sql")
find_latest_patch() {
    local dir="$1"
    local latest=0
    for f in "$dir"/*.sql; do
        [ -f "$f" ] || continue
        local basename
        basename=$(basename "$f")
        local patch_num
        patch_num=$(echo "$basename" | grep -oE '^[0-9]+' || echo "0")
        if [ "$patch_num" -gt "$latest" ] 2>/dev/null; then
            latest="$patch_num"
        fi
    done
    echo "$latest"
}

# Extract current DevVersion from version.go
get_current_version() {
    sed -n 's/.*var DevVersion = "\([^"]*\)".*/\1/p' "$VERSION_FILE" 2>/dev/null || echo "unknown"
}

main() {
    local latest_dir
    latest_dir=$(find_latest_dir)

    if [ -z "$latest_dir" ]; then
        echo "ERROR: No migration directories found in $MIGRATION_DIR"
        exit 1
    fi

    local minor
    minor=$(echo "$latest_dir" | sed 's/^0\.//')

    local latest_patch
    latest_patch=$(find_latest_patch "$MIGRATION_DIR/$latest_dir")

    # Computed version: 0.<minor>.<patch+1> (next development version)
    local next_patch=$((latest_patch + 1))
    local computed_version="0.${minor}.${next_patch}"

    local current_version
    current_version=$(get_current_version)

    echo "Migration filesystem version info:"
    echo "  Latest directory:    $latest_dir"
    echo "  Latest patch:        $latest_patch"
    echo "  Computed version:    $computed_version (what GetCurrentSchemaVersion returns at runtime)"
    echo "  version.go value:    $current_version (default, overridden at runtime)"
    echo ""

    if [ "$computed_version" = "$current_version" ]; then
        echo "Status: MATCH — computed version matches version.go"
        exit 0
    else
        echo "Status: MISMATCH — computed version differs from version.go"
        echo "  Note: This is informational only. GetCurrentSchemaVersion() uses the computed version at runtime."
        echo "  To update version.go, edit internal/version/version.go manually."
        exit 1
    fi
}

main
