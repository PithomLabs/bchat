#!/usr/bin/env bash
# 03_files.sh — Import KB/Policy, get content
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

GROUP="03"
TEST_DIR="$(dirname "$0")"

# Re-sign in as admin
auth_signin "$ADMIN_USER" "$ADMIN_PASS" > /dev/null

# 3.1 — Import KB.MD
log_test "$GROUP" "Import KB"
RESP=$(curl -s -b "$COOKIE_FILE" -c "$COOKIE_FILE" \
  -X POST "$BASE_URL/api/v1/agent/$SLUG/import" \
  -F "audience_type=external" \
  -F "file_type=kb" \
  -F "file=@$SCRIPT_DIR/lib/KB.MD")
SUCCESS=$(echo "$RESP" | jq -r '.success' 2>/dev/null) || SUCCESS="null"
if [ "$SUCCESS" = "true" ]; then
  log_pass "Import KB"
else
  log_fail "Import KB" "success=$SUCCESS"
fi

# 3.2 — Import POLICY.MD
log_test "$GROUP" "Import POLICY"
RESP=$(curl -s -b "$COOKIE_FILE" -c "$COOKIE_FILE" \
  -X POST "$BASE_URL/api/v1/agent/$SLUG/import" \
  -F "audience_type=external" \
  -F "file_type=policy" \
  -F "file=@$SCRIPT_DIR/lib/POLICY.MD")
SUCCESS=$(echo "$RESP" | jq -r '.success' 2>/dev/null) || SUCCESS="null"
if [ "$SUCCESS" = "true" ]; then
  log_pass "Import POLICY"
else
  log_fail "Import POLICY" "success=$SUCCESS"
fi

# 3.3 — Import internal KB (needed for internal chat/simulation)
log_test "$GROUP" "Import internal KB"
RESP=$(curl -s -b "$COOKIE_FILE" -c "$COOKIE_FILE" \
  -X POST "$BASE_URL/api/v1/agent/$SLUG/import" \
  -F "audience_type=internal" \
  -F "file_type=kb" \
  -F "file=@$SCRIPT_DIR/lib/KB.MD")
SUCCESS=$(echo "$RESP" | jq -r '.success' 2>/dev/null) || SUCCESS="null"
if [ "$SUCCESS" = "true" ]; then
  log_pass "Import internal KB"
else
  log_fail "Import internal KB" "success=$SUCCESS"
fi

# 3.4 — Import internal POLICY
log_test "$GROUP" "Import internal POLICY"
RESP=$(curl -s -b "$COOKIE_FILE" -c "$COOKIE_FILE" \
  -X POST "$BASE_URL/api/v1/agent/$SLUG/import" \
  -F "audience_type=internal" \
  -F "file_type=policy" \
  -F "file=@$SCRIPT_DIR/lib/POLICY.MD")
SUCCESS=$(echo "$RESP" | jq -r '.success' 2>/dev/null) || SUCCESS="null"
if [ "$SUCCESS" = "true" ]; then
  log_pass "Import internal POLICY"
else
  log_fail "Import internal POLICY" "success=$SUCCESS"
fi

# 3.5 — Get source file content
log_test "$GROUP" "Get source content"
# Note: test uses external audience since that was imported first
RESP=$(curl_get "/api/v1/agent/$SLUG/source-file?audience_type=external&file_type=kb")
CONTENT=$(echo "$RESP" | jq -r '.content' 2>/dev/null) || CONTENT=""
if [ -n "$CONTENT" ] && [ "$CONTENT" != "null" ]; then
  log_pass "Get source content"
else
  log_fail "Get source content" "content empty"
fi

echo ""
echo "[$GROUP] Done."
