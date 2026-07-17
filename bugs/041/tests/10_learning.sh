#!/usr/bin/env bash
# 10_learning.sh — Learning memory
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

GROUP="10"

# Re-sign in as admin
auth_signin "$ADMIN_USER" "$ADMIN_PASS" > /dev/null

# 10.1 — Get learning memory
log_test "$GROUP" "Get learning"
RESP=$(curl_get "/api/v1/agent/$SLUG/learning")
if [ -n "$RESP" ] && [ "$RESP" != "null" ]; then
  log_pass "Get learning (got response)"
else
  log_fail "Get learning" "empty response"
fi

# 10.2 — Clear learning memory
log_test "$GROUP" "Clear learning"
RESP=$(curl_delete "/api/v1/agent/$SLUG/learning")
MSG=$(echo "$RESP" | jq -r '.message' 2>/dev/null) || MSG=""
if [ -n "$MSG" ] && [ "$MSG" != "null" ]; then
  log_pass "Clear learning ($MSG)"
else
  log_fail "Clear learning" "no message in response"
fi

echo ""
echo "[$GROUP] Done."
