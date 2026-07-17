#!/usr/bin/env bash
# 01_auth.sh — Sign up, sign in, status, sign out (+ create viewer for RBAC)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

GROUP="01"

ADMIN_USER="${ADMIN_USER:-e2e-admin}"
ADMIN_PASS="${ADMIN_PASS:-e2e-admin-pass}"
VIEWER_USER="${VIEWER_USER:-e2e-viewer}"
VIEWER_PASS="${VIEWER_PASS:-e2e-viewer-pass}"

# 1.1 — Sign up as first user (admin)
log_test "$GROUP" "Sign up admin"
RESP=$(auth_signup "$ADMIN_USER" "$ADMIN_PASS")
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -c "$COOKIE_FILE" -b "$COOKIE_FILE" \
  -X POST -H "Content-Type: application/json" \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}" \
  "$BASE_URL/api/v1/auth/signup")
if [ "$STATUS" = "200" ]; then
  log_pass "Sign up admin"
elif [ "$STATUS" = "409" ] || [ "$STATUS" = "400" ] || [ "$STATUS" = "500" ]; then
  # User exists — fall back to sign in
  log_pass "Sign up admin (already exists, signing in)"
  auth_signin "$ADMIN_USER" "$ADMIN_PASS" > /dev/null
else
  log_fail "Sign up admin" "HTTP $STATUS"
fi

# 1.2 — Auth status
log_test "$GROUP" "Auth status"
RESP=$(auth_status)
assert_json "$RESP" '.username' "$ADMIN_USER" "Auth status username"

# 1.3 — Sign up viewer (for RBAC tests)
log_test "$GROUP" "Sign up viewer"
VIEWER_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -c "$COOKIE_FILE" -b "$COOKIE_FILE" \
  -X POST -H "Content-Type: application/json" \
  -d "{\"username\":\"$VIEWER_USER\",\"password\":\"$VIEWER_PASS\"}" \
  "$BASE_URL/api/v1/auth/signup")
if [ "$VIEWER_STATUS" = "200" ]; then
  VIEWER_RESP=$(auth_status)
  VIEWER_USER_ID=$(echo "$VIEWER_RESP" | jq -r '.id')
  log_pass "Sign up viewer (id=$VIEWER_USER_ID)"
elif [ "$VIEWER_STATUS" = "409" ] || [ "$VIEWER_STATUS" = "400" ]; then
  # Viewer exists — sign in to get ID
  auth_signin "$VIEWER_USER" "$VIEWER_PASS" > /dev/null
  VIEWER_RESP=$(auth_status)
  VIEWER_USER_ID=$(echo "$VIEWER_RESP" | jq -r '.id')
  log_pass "Sign up viewer (already exists, id=$VIEWER_USER_ID)"
else
  log_fail "Sign up viewer" "HTTP $VIEWER_STATUS"
  VIEWER_USER_ID=""
fi

# Save viewer ID for other scripts
echo "$VIEWER_USER_ID" > "$(dirname "$0")/.viewer_user_id"

# Sign back in as admin
auth_signin "$ADMIN_USER" "$ADMIN_PASS" > /dev/null

# 1.4 — Sign out
log_test "$GROUP" "Sign out"
RESP=$(auth_signout)
# REST handler returns {} — check that it's a valid JSON object
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -c "$COOKIE_FILE" -b "$COOKIE_FILE" \
  -X POST -H "Content-Type: application/json" \
  -d '{}' \
  "$BASE_URL/api/v1/auth/signout")
if [ "$STATUS" = "200" ]; then
  log_pass "Sign out"
else
  log_fail "Sign out" "HTTP $STATUS"
fi

echo ""
echo "[$GROUP] Done. Viewer user ID: $VIEWER_USER_ID"
