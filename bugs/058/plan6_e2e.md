# Bug 058 — Meta-Test Plan v6: Validating plan_e2e.md

**Date:** 2026-08-03
**Author:** opencode
**Status:** Final — ready to execute
**Depends on:** `plan_e2e.md` (v2), `plan5_e2e.md`

---

## Revision Summary

Final revision addressing all review findings:

| # | Finding | Severity | Change |
|---|---------|----------|--------|
| 1 | `crdb:verify` split mechanism | High | Added `crdb:verify-vectors` task (P6 only) |
| 2 | T10 log format assumption | Low | Use Approach B (critical SQLSTATE codes) — format-agnostic |
| 3 | Phase 3 P6 check not detailed | Low | Added explicit `task crdb:verify-vectors` step |
| 4 | Signal handling wording | Low | Changed "resolved" to "mitigated" |
| 5 | T10 verification note | Low | Added "capture real log lines before execution" |

---

## Taskfile Change

Add `crdb:verify-vectors` task after `crdb:verify`:

```yaml
crdb:verify-vectors:
    desc: Verify agent_vectors table and vector index (P6 only — run after reindex)
    cmds:
      - |
        echo "=== P6: Vector Index Verification ==="
        if [ -z "${COCKROACH_DSN:-}" ]; then
          echo "COCKROACH_DSN not set — skipping P6 checks"
          exit 1
        elif ! command -v cockroach &>/dev/null; then
          echo "cockroach binary not found — skipping P6 checks"
          exit 1
        fi
        run_sql() { cockroach sql --url "${COCKROACH_DSN}" -e "$1" 2>/dev/null; }
        S=$(run_sql "SHOW CLUSTER SETTING feature.vector_index.enabled;")
        echo "$S" | grep -q "true" || { echo "FAIL: feature.vector_index.enabled != true"; exit 1; }
        echo "OK: vector index enabled"
        I=$(run_sql "SELECT count(*) FROM information_schema.statistics WHERE table_name='agent_vectors';")
        echo "$I" | grep -qv "^0" || { echo "FAIL: agent_vectors has no indexes"; exit 1; }
        echo "OK: agent_vectors indexed"
        echo ""
        echo "P6 verification complete!"
```

---

## Phase Execution

### Phase 1: Infrastructure Startup

```bash
task crdb:reset
# Verify cluster settings
cockroach sql --url "postgresql://bchat_user:bchat_pass@localhost:26257/bchat?sslmode=disable" \
  -e "SHOW CLUSTER SETTING feature.vector_index.enabled;"
```

### Phase 2: Tests + App Startup

```bash
# Go tests (BEFORE app starts)
go test -tags "cockroach integration" ./store/ -run TestCockroachP0 -v
BCHAT_ALLOW_DB_RESET=1 go test -tags "cockroach integration" ./store/test/ -run TestCockroachMigrateEndToEnd -v

# Build binary
task build:backend:cockroach

# Start app in background
task run:cockroach &
BCHAT_PID=$!
trap "kill $BCHAT_PID 2>/dev/null; pkill -f 'build/memos' 2>/dev/null; task crdb:down 2>/dev/null" EXIT INT TERM
sleep 10

# Healthz check
curl -fsS http://localhost:5230/healthz

# P1-P5 verification (P6 deferred — agent_vectors doesn't exist yet)
task crdb:verify
```

### Phase 3: Data Path + P6 Verification

```bash
# Full data path (triggers reindex → creates agent_vectors)
BCHAT_URL=http://localhost:5230 \
BCHAT_USER=<your-email> \
BCHAT_PASS=<your-password> \
bash scripts/verify-production.sh

# P6 verification (now passes — reindex created agent_vectors)
task crdb:verify-vectors
```

### Phase 4: Idempotency Proof

```bash
# Stop app
kill $BCHAT_PID
wait $BCHAT_PID 2>/dev/null

# Stop and restart CockroachDB (preserve volume)
task crdb:down
task crdb:up

# Restart app
task run:cockroach &
BCHAT_PID=$!
trap "kill $BCHAT_PID 2>/dev/null; pkill -f 'build/memos' 2>/dev/null; task crdb:down 2>/dev/null" EXIT INT TERM
sleep 10

# Verify infrastructure
task crdb:verify

# Verify application
BCHAT_URL=http://localhost:5230 \
BCHAT_USER=<your-email> \
BCHAT_PASS=<your-password> \
bash scripts/verify-production.sh

# P6 verification
task crdb:verify-vectors
```

### Phase 5: Cleanup & Gate

```bash
kill $BCHAT_PID
wait $BCHAT_PID 2>/dev/null
task crdb:down
rm -rf build/data
```

---

## Go/No-Go Checklist

| Gate | Status | Notes |
|------|--------|-------|
| Phase 1: Infrastructure | [ ] Pass | |
| Phase 2: Tests | [ ] Pass | |
| Phase 2: App startup | [ ] Pass | |
| Phase 3: Data path | [ ] Pass | |
| Phase 3: P6 verification | [ ] Pass | |
| Phase 4: Idempotency | [ ] Pass | |
| No orphaned processes | [ ] Pass | |
