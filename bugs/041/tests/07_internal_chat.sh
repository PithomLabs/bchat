#!/usr/bin/env bash
# 07_internal_chat.sh — Authenticated chat
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

GROUP="07"

# Re-sign in as admin
auth_signin "$ADMIN_USER" "$ADMIN_PASS" > /dev/null

# 7.1 — Authenticated chat
log_test "$GROUP" "Authenticated chat"
RESP=$(curl_post "/api/v1/agent/$SLUG/chat/int" '{"message":"What services do you offer?"}')
CONTENT=$(echo "$RESP" | jq -r '.message.content' 2>/dev/null) || CONTENT=""
INTENT=$(echo "$RESP" | jq -r '.metadata.intent' 2>/dev/null) || INTENT=""
if [ -n "$CONTENT" ] && [ "$CONTENT" != "null" ]; then
  log_pass "Authenticated chat (intent=$INTENT)"
else
  log_fail "Authenticated chat" "content empty, response: $(echo "$RESP" | head -c 200)"
fi

# 7.2 — Chat without auth (use a fresh cookie jar)
NOAUTH_JAR=$(mktemp)
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -c "$NOAUTH_JAR" -b "$NOAUTH_JAR" \
  -X POST -H "Content-Type: application/json" \
  -d '{"message":"test"}' \
  "$BASE_URL/api/v1/agent/$SLUG/chat/int")
rm -f "$NOAUTH_JAR"
log_test "$GROUP" "No auth"
if [ "$STATUS" = "401" ] || [ "$STATUS" = "403" ]; then
  log_pass "No auth ($STATUS)"
else
  log_fail "No auth" "expected 401/403, got $STATUS"
fi

echo ""
echo "[$GROUP] Done."
