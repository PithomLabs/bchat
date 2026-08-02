#!/bin/bash
# =============================================================================
# verify-production.sh — app-first smoke against the deployed bchat instance
# (bugs/057 pre_code.md §6.3). Exercises the full data path: auth -> tenant
# onboarding -> KB import -> RAG reindex -> vector search. Test tenant is
# destroyed on exit (--keep disables). Fails fast with non-zero exit.
#
# Usage:
#   BCHAT_URL=https://bchat-crdb.fly.dev BCHAT_USER=admin BCHAT_PASS=... \
#     bash scripts/verify-production.sh [--keep]
# =============================================================================

set -euo pipefail

URL="${BCHAT_URL:-https://bchat-crdb.fly.dev}"
USER="${BCHAT_USER:?BCHAT_USER required (memos admin username)}"
PASS="${BCHAT_PASS:?BCHAT_PASS required}"
KEEP=0
[[ "${1:-}" == "--keep" ]] && KEEP=1

SLUG="verify-$(date +%s)"
COOKIE_JAR=$(mktemp)
TMP_KB=$(mktemp)
trap 'rm -f "$COOKIE_JAR" "$TMP_KB"' EXIT

pass() { echo -e "  \033[0;32mPASS\033[0m $1"; }
fail() { echo -e "  \033[0;31mFAIL\033[0m $1"; exit 1; }

echo "=== verify:production ($URL, tenant=$SLUG) ==="

# 1. healthz
echo "[1/7] healthz"
curl -fsS -o /dev/null "$URL/healthz" || fail "healthz not 200"
pass "healthz 200"

# 2. signin (REST session cookie)
echo "[2/7] signin"
curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "Content-Type: application/json" \
  -d "{\"password_credentials\":{\"username\":\"$USER\",\"password\":\"$PASS\"}}" \
  "$URL/api/v1/auth/signin" -o /dev/null || fail "signin failed (bad credentials?)"
pass "signin"

# 3. tenant selection (multi-tenant flow: /auth/tenants + /auth/select-tenant)
echo "[3/7] select tenant"
TENANTS=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}" \
  "$URL/api/v1/auth/tenants" || fail "auth/tenants failed")
TOKEN=$(echo "$TENANTS" | grep -o '"selection_token":"[^"]*"' | head -1 | cut -d'"' -f4)
TENANT_ID=$(echo "$TENANTS" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
[[ -n "$TOKEN" && -n "$TENANT_ID" ]] || fail "no selection token / tenant id in response"
curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "Content-Type: application/json" \
  -d "{\"selection_token\":\"$TOKEN\",\"tenant_id\":$TENANT_ID}" \
  "$URL/api/v1/auth/select-tenant" -o /dev/null || fail "select-tenant failed"
pass "tenant selected (id=$TENANT_ID)"

# 4. onboard test tenant
echo "[4/7] onboard $SLUG"
curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -F "tenant_slug=$SLUG" \
  -F "company_name=Verify Production Smoke" \
  -F "vertical=qa" \
  "$URL/api/v1/agent/onboard" -o /dev/null || fail "onboard failed"
pass "tenant onboarded"

# 5. KB import + reindex
echo "[5/7] KB import + reindex"
KB_CONTENT='<!-- @service: verify_service -->
## Verify Service
Automated smoke test service. Emergency response available 24/7.

<!-- @faq: smoke -->
## Is this a smoke test?
Yes, this is an automated deployment verification.
'
for i in $(seq 1 1000); do
  echo "$KB_CONTENT" >> "$TMP_KB"
done
curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -F "audience_type=internal" \
  -F "file_type=kb" \
  -F "file=@$TMP_KB" \
  "$URL/api/v1/agent/$SLUG/import" -o /dev/null || fail "KB import failed"
curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -X POST "$URL/api/v1/agent/$SLUG/reindex" -o /dev/null || fail "reindex failed"
pass "KB imported + reindexed"

# 6. RAG search (vector round-trip)
echo "[6/7] RAG search"
for i in $(seq 1 12); do
  HITS=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -H "Content-Type: application/json" \
    -d '{"query":"smoke test"}' \
    "$URL/api/v1/agent/$SLUG/rag/search" 2>/dev/null || echo "")
  N=$(echo "$HITS" | grep -o '"hits"\|"items"' | head -1 | wc -l)
  [[ "$N" -ge 1 ]] && break
  sleep 5
done
[[ "$N" -ge 1 ]] || fail "RAG search returned no hits"
pass "RAG search round-trip"

# 7. cleanup (destroy default on)
if [[ "$KEEP" == "0" ]]; then
  echo "[7/7] destroy $SLUG"
  curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -X DELETE "$URL/api/v1/agent/$SLUG" -o /dev/null || fail "cleanup failed"
  pass "test tenant destroyed"
else
  echo "[7/7] --keep: leaving tenant $SLUG in place"
fi

echo ""
echo "=== verify:production PASSED ==="
