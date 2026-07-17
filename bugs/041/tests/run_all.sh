#!/usr/bin/env bash
# run_all.sh — Execute all E2E tests in order, report pass/fail
#
# Usage:
#   ./run_all.sh              Run all tests
#   BASE_URL=http://host:port ./run_all.sh   Custom server URL
#
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

TOTAL=0
PASSED=0
FAILED=0
SKIPPED=0

run_test() {
  local script="$1" label="$2"
  if bash "$SCRIPT_DIR/$script" 2>&1 | tee -a "$SCRIPT_DIR/.test_output"; then
    PASSED=$((PASSED + 1))
  else
    FAILED=$((FAILED + 1))
  fi
}

main() {
  echo "=== bchat E2E Critical Path Tests ==="
  echo "Server: $BASE_URL"
  echo "Tenant: $SLUG"
  echo ""

  # Setup
  setup_common

  # Wait for server
  if ! wait_for_server 30; then
    echo "FATAL: Server not ready at $BASE_URL"
    exit 1
  fi

  # Clean up orphaned tenant from previous failed run
  echo ""
  echo "Cleaning up orphaned tenant..."
  cleanup_tenant "$SLUG" 2>/dev/null || true

  # Run test groups in order
  run_test "01_auth.sh"           "Auth"
  run_test "02_tenant_crud.sh"    "Tenant CRUD"
  run_test "03_files.sh"          "Files"
  run_test "04_llm_config.sh"     "LLM Config"
  run_test "05_rag.sh"            "RAG Pipeline"
  run_test "06_external_chat.sh"  "External Chat"
  run_test "07_internal_chat.sh"  "Internal Chat"
  run_test "08_simulation.sh"     "Simulation"
  run_test "09_rbac.sh"           "RBAC"
  run_test "10_learning.sh"       "Learning Memory"
  run_test "11_cleanup.sh"        "Cleanup"

  # Summary
  echo ""
  echo "=== Results ==="
  TOTAL=$((PASSED + FAILED))
  echo "Total: $TOTAL tests"
  printf "  \033[32mPassed: %d\033[0m\n" "$PASSED"
  if [ "$FAILED" -gt 0 ]; then
    printf "  \033[31mFailed: %d\033[0m\n" "$FAILED"
  else
    printf "  Failed: %d\n" "$FAILED"
  fi

  if [ "$FAILED" -gt 0 ]; then
    echo ""
    echo "=== Failed Test Output ==="
    cat "$SCRIPT_DIR/.test_output" | grep -A2 "FAIL" || true
    exit 1
  fi

  echo ""
  echo "All tests passed!"
  exit 0
}

# Clean up previous output
rm -f "$SCRIPT_DIR/.test_output"
touch "$SCRIPT_DIR/.test_output"

main "$@"
