#!/usr/bin/env bash
# init-local-cockroach-db.sh — reset the hermetic bchat_test database on the local CRDB node.
# Safe only for loopback DSNs; the memos binary enforces the same rule at startup (internal/profile).
set -euo pipefail

require() {
  command -v "$1" >/dev/null 2>&1 || { echo "ERROR: $1 not found on PATH" >&2; exit 1; }
}
require cockroach

DSN="${COCKROACH_DSN:-}"
if [ -z "$DSN" ]; then
  echo "ERROR: COCKROACH_DSN not set (copy .env.local.example to .env.local first)" >&2
  exit 1
fi

case "$DSN" in
  *localhost*|*127.0.0.1*|*\[::1\]*)
    ;;
  *)
    echo "ERROR: refusing to reset DB — COCKROACH_DSN host is not loopback: $DSN" >&2
    echo "Local dev runs must target the local CRDB node (localhost:26257), never the cloud cluster." >&2
    exit 1
    ;;
esac

echo "=== Resetting hermetic database bchat_test (drop + create) ==="
cockroach sql --url "postgresql://root@localhost:26257/defaultdb?sslmode=disable" \
  -e "DROP DATABASE IF EXISTS bchat_test; CREATE DATABASE bchat_test;"
echo "=== bchat_test ready ==="
