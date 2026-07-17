#!/usr/bin/env bash
# 11_cleanup.sh — Remove test data
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

GROUP="11"

# Re-sign in as admin
auth_signin "$ADMIN_USER" "$ADMIN_PASS" > /dev/null

# 11.1 — Delete test tenant
log_test "$GROUP" "Delete tenant"
RESP=$(curl_delete "/api/v1/agent/$SLUG")
SUCCESS=$(echo "$RESP" | jq -r '.success' 2>/dev/null) || SUCCESS="null"
if [ "$SUCCESS" = "true" ]; then
  log_pass "Delete tenant"
else
  log_fail "Delete tenant" "success=$SUCCESS"
fi

# Clean up temp files
rm -f "$SCRIPT_DIR/.widget_key" "$SCRIPT_DIR/.session_id" "$SCRIPT_DIR/.sim_id" "$SCRIPT_DIR/.viewer_user_id"

echo ""
echo "[$GROUP] Done."
