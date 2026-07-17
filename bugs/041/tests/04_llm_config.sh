#!/usr/bin/env bash
# 04_llm_config.sh — Get/set LLM config
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

GROUP="04"

# Re-sign in as admin
auth_signin "$ADMIN_USER" "$ADMIN_PASS" > /dev/null

# 4.1 — Get LLM config (may be empty on fresh tenant)
log_test "$GROUP" "Get LLM config"
RESP=$(curl_get "/api/v1/agent/$SLUG/llm-config")
STATUS_CODE=$(echo "$RESP" | jq -r '.tenant_slug' 2>/dev/null) || STATUS_CODE=""
if [ -n "$STATUS_CODE" ] && [ "$STATUS_CODE" != "null" ]; then
  log_pass "Get LLM config (tenant_slug=$STATUS_CODE)"
else
  log_fail "Get LLM config" "response invalid"
fi

# 4.2 — Set LLM config
log_test "$GROUP" "Set LLM config"
RESP=$(curl_put "/api/v1/agent/$SLUG/llm-config" '{
  "llm_model": "openai/gpt-4o-mini",
  "simulation_human_model": "openai/gpt-4o-mini",
  "reasoning_model": "google/gemini-2.5-pro"
}')
MODEL=$(echo "$RESP" | jq -r '.llm_model' 2>/dev/null) || MODEL=""
if [ -n "$MODEL" ] && [ "$MODEL" != "null" ]; then
  log_pass "Set LLM config (model=$MODEL)"
else
  log_fail "Set LLM config" "no model in response"
fi

# 4.3 — Get LLM config after set
log_test "$GROUP" "Get LLM config after set"
RESP=$(curl_get "/api/v1/agent/$SLUG/llm-config")
MODEL=$(echo "$RESP" | jq -r '.llm_model' 2>/dev/null) || MODEL=""
if [ "$MODEL" = "openai/gpt-4o-mini" ]; then
  log_pass "Get LLM config after set (model=$MODEL)"
else
  log_fail "Get LLM config after set" "expected openai/gpt-4o-mini, got '$MODEL'"
fi

echo ""
echo "[$GROUP] Done."
