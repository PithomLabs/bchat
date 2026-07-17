#!/usr/bin/env bash
# 08_simulation.sh — AI-vs-AI testing
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

GROUP="08"
TEST_DIR="$(dirname "$0")"

# Re-sign in as admin
auth_signin "$ADMIN_USER" "$ADMIN_PASS" > /dev/null

# 8.1 — Start simulation
log_test "$GROUP" "Start simulation"
RESP=$(curl_post "/api/v1/agent/$SLUG/simulate" '{"initial_prompt":"Customer has water damage in basement, needs emergency help"}')
SIM_ID=$(echo "$RESP" | jq -r '.session_id' 2>/dev/null) || SIM_ID=""
SIM_STATUS=$(echo "$RESP" | jq -r '.status' 2>/dev/null) || SIM_STATUS=""
if [ -n "$SIM_ID" ] && [ "$SIM_ID" != "null" ]; then
  log_pass "Start simulation (id=$SIM_ID, status=$SIM_STATUS)"
else
  log_fail "Start simulation" "session_id=$SIM_ID"
fi
echo "$SIM_ID" > "$TEST_DIR/.sim_id"

# 8.2 — Stream simulation (SSE) — just verify connection works
log_test "$GROUP" "Stream simulation"
SIM_ID=$(cat "$TEST_DIR/.sim_id" 2>/dev/null || echo "")
if [ -n "$SIM_ID" ]; then
  # Use timeout to avoid hanging forever; capture first few events
  STREAM_RESP=$(timeout 15 curl -s -N -b "$COOKIE_FILE" \
    "$BASE_URL/api/v1/agent/$SLUG/simulate/$SIM_ID/stream" 2>/dev/null || true)
  if echo "$STREAM_RESP" | grep -q "event:"; then
    log_pass "Stream simulation (SSE events received)"
  elif [ -n "$STREAM_RESP" ]; then
    log_pass "Stream simulation (got response, may still be running)"
  else
    log_fail "Stream simulation" "no SSE events received"
  fi
else
  log_fail "Stream simulation" "no sim_id from 8.1"
fi

# 8.3 — List simulations (poll until transcript is saved, max 60s)
# Simulations involve multiple LLM calls; the goroutine saves transcript when done
log_test "$GROUP" "List simulations"
SIM_LIST_OK=false
for i in $(seq 1 20); do
  sleep 3
  RESP=$(curl_get "/api/v1/agent/$SLUG/simulations")
  TOTAL=$(echo "$RESP" | jq -r '.total' 2>/dev/null) || TOTAL="0"
  if [ "$TOTAL" -gt 0 ] 2>/dev/null; then
    log_pass "List simulations (found $TOTAL)"
    SIM_LIST_OK=true
    break
  fi
done
if [ "$SIM_LIST_OK" = false ]; then
  log_fail "List simulations" "total=$TOTAL after polling (transcript save may be slow)"
fi

echo ""
echo "[$GROUP] Done."
