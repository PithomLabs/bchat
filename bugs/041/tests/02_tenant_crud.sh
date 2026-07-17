#!/usr/bin/env bash
# 02_tenant_crud.sh — Onboard, config, update (+ capture widget key)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

GROUP="02"
TEST_DIR="$(dirname "$0")"
VIEWER_USER_ID=$(cat "$TEST_DIR/.viewer_user_id" 2>/dev/null || echo "")

# Re-sign in as admin
auth_signin "$ADMIN_USER" "$ADMIN_PASS" > /dev/null

# 2.1 — Onboard tenant with KB/Policy
log_test "$GROUP" "Onboard tenant"
RESP=$(curl -s -b "$COOKIE_FILE" -c "$COOKIE_FILE" \
  -X POST "$BASE_URL/api/v1/agent/onboard" \
  -F "tenant_slug=$SLUG" \
  -F "company_name=E2E Test Corp" \
  -F "vertical=restoration" \
  -F "external_kb_file=@$SCRIPT_DIR/lib/KB.MD" \
  -F "external_policy_file=@$SCRIPT_DIR/lib/POLICY.MD")
STATUS=$(echo "$RESP" | jq -r '.success' 2>/dev/null) || STATUS="null"
if [ "$STATUS" = "true" ]; then
  log_pass "Onboard tenant"
else
  log_fail "Onboard tenant" "success=$STATUS, response: $(echo "$RESP" | head -c 200)"
fi

# 2.2 — Get tenant config (capture widget key)
log_test "$GROUP" "Get config"
RESP=$(curl_get "/api/v1/agent/$SLUG/config")
WIDGET_KEY=$(echo "$RESP" | jq -r '.tenant.widgetKey' 2>/dev/null) || WIDGET_KEY=""
if [ -n "$WIDGET_KEY" ] && [ "$WIDGET_KEY" != "null" ]; then
  log_pass "Get config (widget_key=$WIDGET_KEY)"
else
  log_fail "Get config" "widget_key missing"
  WIDGET_KEY=""
fi
echo "$WIDGET_KEY" > "$TEST_DIR/.widget_key"

# 2.3 — Update tenant + rotate widget key
log_test "$GROUP" "Update tenant"
RESP=$(curl_patch "/api/v1/agent/$SLUG" '{"company_name":"E2E Test Corp Updated"}')
STATUS=$(echo "$RESP" | jq -r '.success' 2>/dev/null) || STATUS="null"
if [ "$STATUS" = "true" ]; then
  log_pass "Update tenant"
else
  log_fail "Update tenant" "success=$STATUS"
fi

echo ""
echo "[$GROUP] Done. Widget key: $WIDGET_KEY"
