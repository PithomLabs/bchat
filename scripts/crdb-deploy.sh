#!/bin/bash
# =============================================================================
# crdb-deploy.sh — CockroachDB Fly deploy chain runner (bugs/057 pre_code.md §4.10)
# Thin chain runner: run stage -> check exit -> log -> next. All logic lives in
# Taskfile tasks (crdb:verify, verify:production). Re-runnable — safe to re-run
# after any stage failure (stateful-safe). Logs each stage to build/crdb-deploy.log.
#
# Usage:
#   bash scripts/crdb-deploy.sh            # full chain
#   bash scripts/crdb-deploy.sh --experiments   # + P4/P5 optional gates
# =============================================================================

set -euo pipefail

APP="bchat-crdb"
HEALTH_URL="https://$APP.fly.dev/healthz"
LOG="build/crdb-deploy.log"
mkdir -p build
: > "$LOG"

stage() { echo "=== $1 ===" | tee -a "$LOG"; }
fail()  { echo -e "FAILED: $1\nFull log: $LOG" | tee -a "$LOG"; exit 1; }

EXPERIMENTS=0
if [[ "${1:-}" == "--experiments" ]]; then
  EXPERIMENTS=1
fi

# 1. Build
stage "1/7 build:backend:cockroach"
task build:backend:cockroach >>"$LOG" 2>&1 || fail "build:backend:cockroach"

# 2. Migration parity (now includes cockroach<->postgres pair)
stage "2/7 validate-parity.sh"
bash scripts/validate-parity.sh >>"$LOG" 2>&1 || fail "validate-parity.sh"

# 3. Cockroach migration compatibility scanner
stage "3/7 validate-cockroach-compat.sh"
bash scripts/validate-cockroach-compat.sh >>"$LOG" 2>&1 || fail "validate-cockroach-compat.sh"

# 4. Optional experiments (P4/P5, bugs/057 §6.1)
if [[ "$EXPERIMENTS" == "1" ]]; then
  stage "4/7 experiments (P4/P5)"
  task crdb:test >>"$LOG" 2>&1 || fail "crdb:test (experiments)"
else
  stage "4/7 experiments skipped (--experiments to enable P4/P5)"
fi

# 5. Fly deploy
# Timeout ordering (bugs/057 plan4_deploy.md §6): 
#   fly --wait-timeout 45m (informational) < poll 50m (authoritative) < grace 60m (machine-side bound)
# A wait-timeout expiry mid-migration is EXPECTED (stage-5 is informational only —
# fly abandons waiting, the machine keeps migrating). Stage 6 decides success.
stage "5/7 fly deploy (--wait-timeout 45m; informational)"
if ! fly -a "$APP" deploy -c fly_cockroach.toml --ignorefile .dockerignore.cockroach --wait-timeout 45m 2>&1 | tee -a "$LOG"; then
  echo "--- fly deploy wait timed out (expected mid-migration) — continuing to stage 6 (authoritative poll)" | tee -a "$LOG"
fi

# 6. Healthz poll (authoritative; grace 60m per fly_cockroach.toml http_service.checks)
stage "6/7 healthz poll ($HEALTH_URL)"
sleep 15
OK=0
for i in $(seq 1 600); do
  if curl -fsS -o /dev/null "$HEALTH_URL" 2>/dev/null; then
    echo "--- healthz 200 OK (attempt $i/600)" | tee -a "$LOG"
    OK=1
    break
  fi
  sleep 5
done
[[ "$OK" == "1" ]] || fail "healthz not 200 after ~50m"

# 7. Production-facing verification (bugs/057 §6.2 + §6.3)
stage "7/7 crdb:verify + verify:production"
# Source credentials for verify:production (fly secrets are NOT readable at deploy time)
if [ -f .env ]; then
  set -a && . .env && set +a
fi
task crdb:verify >>"$LOG" 2>&1 || fail "crdb:verify (bugs/057 §6.2)"
task verify:production >>"$LOG" 2>&1 || fail "verify:production (bugs/057 §6.3)"

echo ""
echo "=== DEPLOY COMPLETE — $HEALTH_URL ==="
echo "Full log: $LOG"
