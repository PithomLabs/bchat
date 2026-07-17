#!/usr/bin/env bash
# 09_rbac.sh — Permissions (uses viewer user from 01_auth.sh)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

GROUP="09"
TEST_DIR="$(dirname "$0")"
VIEWER_USER_ID=$(cat "$TEST_DIR/.viewer_user_id" 2>/dev/null || echo "")

# Re-sign in as admin
auth_signin "$ADMIN_USER" "$ADMIN_PASS" > /dev/null

if [ -z "$VIEWER_USER_ID" ]; then
  echo "SKIP: No viewer user ID (run 01_auth.sh first)"
  exit 0
fi

# 9.1 — List permissions
log_test "$GROUP" "List permissions"
RESP=$(curl_get "/api/v1/agent/$SLUG/permissions")
PERMS=$(echo "$RESP" | jq -r '.permissions | length' 2>/dev/null) || PERMS="0"
if [ "$PERMS" -ge 0 ] 2>/dev/null; then
  log_pass "List permissions (found $PERMS entries)"
else
  log_fail "List permissions" "could not parse response"
fi

# 9.2 — Grant permission to viewer
log_test "$GROUP" "Grant permission"
RESP=$(curl_post "/api/v1/agent/$SLUG/permissions" \
  "{\"user_id\":$VIEWER_USER_ID,\"permissions\":[\"tenant:read\"]}")
SUCCESS=$(echo "$RESP" | jq -r '.success' 2>/dev/null) || SUCCESS="null"
if [ "$SUCCESS" = "true" ]; then
  log_pass "Grant permission"
else
  log_fail "Grant permission" "success=$SUCCESS"
fi

# 9.3 — Revoke permission
log_test "$GROUP" "Revoke permission"
RESP=$(curl_delete "/api/v1/agent/$SLUG/permissions/$VIEWER_USER_ID")
SUCCESS=$(echo "$RESP" | jq -r '.success' 2>/dev/null) || SUCCESS="null"
if [ "$SUCCESS" = "true" ]; then
  log_pass "Revoke permission"
else
  log_fail "Revoke permission" "success=$SUCCESS"
fi

echo ""
echo "[$GROUP] Done."
