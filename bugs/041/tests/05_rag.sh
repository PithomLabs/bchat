#!/usr/bin/env bash
# 05_rag.sh — Reindex, poll status, search
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

GROUP="05"

# Re-sign in as admin
auth_signin "$ADMIN_USER" "$ADMIN_PASS" > /dev/null

# 5.1 — Trigger reindex
log_test "$GROUP" "Trigger reindex"
RESP=$(curl_post "/api/v1/agent/$SLUG/reindex" '{}')
SUCCESS=$(echo "$RESP" | jq -r '.success' 2>/dev/null) || SUCCESS="null"
if [ "$SUCCESS" = "true" ]; then
  log_pass "Trigger reindex"
else
  log_fail "Trigger reindex" "success=$SUCCESS"
fi

# 5.2 — Poll reindex status (up to 60s)
log_test "$GROUP" "Poll reindex status"
POLL_RESP=$(poll_reindex_status "$SLUG" 30)
POLL_STATUS=$(echo "$POLL_RESP" | jq -r '.status' 2>/dev/null) || POLL_STATUS="unknown"
if [ "$POLL_STATUS" = "completed" ] || [ "$POLL_STATUS" = "idle" ]; then
  log_pass "Reindex status: $POLL_STATUS"
else
  log_fail "Reindex status" "status=$POLL_STATUS"
fi

# 5.3 — RAG search explorer
log_test "$GROUP" "RAG search"
RESP=$(curl_post "/api/v1/agent/$SLUG/rag/search" '{"query":"water extraction","top_k":3}')
RESULTS=$(echo "$RESP" | jq -r '.total_results' 2>/dev/null) || RESULTS="0"
if [ "$RESULTS" -gt 0 ] 2>/dev/null; then
  log_pass "RAG search (found $RESULTS results)"
else
  log_fail "RAG search" "total_results=$RESULTS"
fi

echo ""
echo "[$GROUP] Done."
