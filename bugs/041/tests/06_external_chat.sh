#!/usr/bin/env bash
# 06_external_chat.sh — Widget chat flow (uses widget key)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

GROUP="06"
TEST_DIR="$(dirname "$0")"
WIDGET_KEY=$(cat "$TEST_DIR/.widget_key" 2>/dev/null || echo "")

if [ -z "$WIDGET_KEY" ]; then
  echo "SKIP: No widget key available (run 02_tenant_crud.sh first)"
  exit 0
fi

# 6.1 — First message (new session)
log_test "$GROUP" "New session"
RESP=$(curl -s -X POST "$BASE_URL/api/v1/agent/$SLUG/chat/ext" \
  -H "Content-Type: application/json" \
  -H "X-Widget-Key: $WIDGET_KEY" \
  -d '{"message":"Hello, I need help with water damage"}')
SESSION_ID=$(echo "$RESP" | jq -r '.session_id' 2>/dev/null) || SESSION_ID=""
CONTENT=$(echo "$RESP" | jq -r '.message.content' 2>/dev/null) || CONTENT=""
if [ -n "$SESSION_ID" ] && [ "$SESSION_ID" != "null" ] && [ -n "$CONTENT" ]; then
  log_pass "New session (id=$SESSION_ID)"
else
  log_fail "New session" "session_id=$SESSION_ID, content_empty=$([ -z "$CONTENT" ] && echo yes || echo no)"
fi
echo "$SESSION_ID" > "$TEST_DIR/.session_id"

# 6.2 — Follow-up message
log_test "$GROUP" "Follow-up"
SESSION_ID=$(cat "$TEST_DIR/.session_id" 2>/dev/null || echo "")
if [ -n "$SESSION_ID" ]; then
  RESP=$(curl -s -X POST "$BASE_URL/api/v1/agent/$SLUG/chat/ext" \
    -H "Content-Type: application/json" \
    -H "X-Widget-Key: $WIDGET_KEY" \
    -d "{\"session_id\":\"$SESSION_ID\",\"message\":\"What is the cost?\"}")
  FSESSION=$(echo "$RESP" | jq -r '.session_id' 2>/dev/null) || FSESSION=""
  FCONTENT=$(echo "$RESP" | jq -r '.message.content' 2>/dev/null) || FCONTENT=""
  if [ "$FSESSION" = "$SESSION_ID" ] && [ -n "$FCONTENT" ]; then
    log_pass "Follow-up"
  else
    log_fail "Follow-up" "session mismatch or empty content"
  fi
else
  log_fail "Follow-up" "no session_id from 6.1"
fi

# 6.3 — Invalid widget key
log_test "$GROUP" "Invalid key"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
  "$BASE_URL/api/v1/agent/$SLUG/chat/ext" \
  -H "Content-Type: application/json" \
  -H "X-Widget-Key: invalid-key-12345" \
  -d '{"message":"test"}')
if [ "$STATUS" = "403" ]; then
  log_pass "Invalid key (403)"
else
  log_fail "Invalid key" "expected 403, got $STATUS"
fi

echo ""
echo "[$GROUP] Done."
