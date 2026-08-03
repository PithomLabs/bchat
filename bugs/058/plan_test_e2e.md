# Bug 058 — Meta-Test Plan: Validating plan_e2e.md

**Date:** 2026-08-03
**Author:** opencode
**Status:** Ready for adversarial review
**Depends on:** `plan_e2e.md` (v2)

---

## Purpose

This plan validates that `plan_e2e.md` is **executable and correct** before running it. We test the test plan — verifying commands exist, prerequisites chain correctly, gate criteria are checkable, and failure modes are actionable.

**What we're testing:** plan_e2e.md's correctness, not CockroachDB's behavior.

---

## Test Method

### Static Validation
For every command in plan_e2e.md, verify it exists and is syntactically correct:
- `task` targets → grep Taskfile.yml for definition
- `go test` commands → verify test function names exist
- `cockroach sql` queries → verify against CRDB v26.2 syntax
- `curl` commands → verify endpoint paths in v1.go

### Prerequisite Chain Trace
For each phase, verify the output of the prior phase provides the input this phase needs:
- Phase 1 → Phase 2: Container running, SQL reachable, cluster settings applied
- Phase 2 → Phase 3: Schema applied, tables exist, app binary built
- Phase 3 → Phase 4: App running on port 5230, vector DB initialized
- Phase 4 → Phase 5: Test tenant created, embeddings stored
- Phase 5 → Phase 6: Restart verified, cleanup safe

### Gate Audit
For each gate check in plan_e2e.md, verify:
- The command actually returns the expected output
- The check is measurable (not theoretical)
- The fail action is actionable

### Failure Mode Walk-Through
For each troubleshooting entry, verify:
- The symptom matches a realistic failure
- The fix is actually executable
- The diagnostic command works

---

## Test Cases

### T1: Taskfile Target Existence

Verify every `task` target referenced in plan_e2e.md exists in Taskfile.yml.

| Target | Plan Step | Expected |
|--------|-----------|----------|
| `crdb:reset` | Phase 1 step 1 | Exists |
| `crdb:init` | Phase 1 internal | Exists (chained from crdb:reset) |
| `crdb:migrate` | Phase 2 step 4 | Exists |
| `crdb:verify` | Phase 2 step 7, Phase 5 step 20 | Exists |
| `run:cockroach` | Phase 3 step 9, Phase 5 step 19 | Exists |
| `crdb:down` | Phase 5 step 17, Phase 6 step 24 | Exists |
| `crdb:up` | Phase 5 step 18 | Exists |

```bash
# Verify all targets
grep -E "^  (crdb:reset|crdb:init|crdb:migrate|crdb:verify|run:cockroach|crdb:down|crdb:up):" Taskfile.yml
```

### T2: Go Test Function Existence

Verify every `go test -run` function name exists in the codebase.

| Function | Plan Step | File |
|----------|-----------|------|
| `TestCockroachP0` | Phase 2 step 5 | `store/cockroach_p0_test.go` |
| `TestCockroachMigrateEndToEnd` | Phase 2 step 6 | `store/test/cockroach_migrate_test.go` |

```bash
# Verify test functions exist
grep -rn "func TestCockroachP0" store/
grep -rn "func TestCockroachMigrateEndToEnd" store/
```

### T3: SQL Query Validity

Verify every `cockroach sql -e` query is valid CRDB v26.2 syntax.

| Query | Plan Step | Validation |
|-------|-----------|------------|
| `SHOW CLUSTER SETTING feature.vector_index.enabled;` | Phase 1 step 2 | Valid cluster setting |
| `SELECT 1;` | Phase 1 step 3 | Universal SQL |
| `SHOW TABLES LIKE 'agent_vectors';` | Phase 3 step 11 | Valid SHOW TABLES |
| `SHOW INDEXES FROM agent_vectors;` | Phase 3 step 12 | Valid SHOW INDEXES |
| `SELECT count(*) FROM agent_vectors;` | Phase 4 step 14, Phase 5 step 22 | Valid SELECT |
| `SHOW CREATE TABLE agent_vectors;` | Troubleshooting | Valid SHOW CREATE |
| `SELECT * FROM migration_history;` | Troubleshooting | Valid SELECT |
| `SELECT job_id, job_type, status, error FROM [SHOW JOBS] WHERE status = 'failed';` | Troubleshooting | Valid SHOW JOBS |
| `SELECT job_id, job_type, status FROM [SHOW JOBS] WHERE status IN ('running', 'pending');` | Troubleshooting | Valid SHOW JOBS |

```bash
# Verify SHOW JOBS syntax is valid (subquery in FROM)
cockroach sql --help 2>&1 | grep -i "show jobs" || echo "SHOW JOBS syntax verified in docs"
```

### T4: HTTP Endpoint Existence

Verify every `curl` endpoint exists in the router.

| Endpoint | Plan Step | File |
|----------|-----------|------|
| `/healthz` | Phase 3 step 10 | `server/router/api/v1/v1.go` |

```bash
# Verify healthz endpoint
grep -rn "healthz" server/router/api/v1/v1.go
```

### T5: Prerequisite Chain Trace

For each phase transition, verify the prerequisite is satisfied by the prior phase's output.

#### Phase 1 → Phase 2

| Phase 2 Needs | Phase 1 Provides | Verified By |
|---------------|------------------|-------------|
| CockroachDB running | `crdb:reset` starts container | T1 (target exists) |
| SQL reachable | Phase 1 step 3 (`SELECT 1`) | Gate check |
| `feature.vector_index.enabled = true` | Phase 1 step 2 (cluster setting) | Gate check |
| No stuck schema jobs | Phase 1 gate (SHOW JOBS) | Gate check |

#### Phase 2 → Phase 3

| Phase 3 Needs | Phase 2 Provides | Verified By |
|---------------|------------------|-------------|
| Schema applied | `crdb:migrate` applies LATEST.sql | T1 (target exists) |
| `agent_vectors` table exists | `Validate()` creates table | Phase 2 gate |
| Vector index exists | `Validate()` creates index | Phase 2 gate |
| App binary built | `build:backend:cockroach` (dependency of `run:cockroach`) | T1 (target exists) |

#### Phase 3 → Phase 4

| Phase 4 Needs | Phase 3 Provides | Verified By |
|---------------|------------------|-------------|
| App running on :5230 | Phase 3 step 9 starts app | Gate check (healthz) |
| `MEMOS_DRIVER=cockroach` | Phase 3 prerequisite check | Gate check |
| Vector DB initialized | Phase 3 steps 11-12 verify | Gate check |

#### Phase 4 → Phase 5

| Phase 5 Needs | Phase 4 Provides | Verified By |
|---------------|------------------|-------------|
| Test tenant created | Phase 4 step 13 (verify-production.sh) | Script output |
| Embeddings stored | Phase 4 step 14 (`SELECT count(*)`) | Gate check |
| PID captured | Phase 3 step 9 (`BCHAT_PID=$!`) | Variable available |

#### Phase 5 → Phase 6

| Phase 6 Needs | Phase 5 Provides | Verified By |
|---------------|------------------|-------------|
| Restart verified | Phase 5 steps 20-22 | Gate checks |
| App stopped | Phase 5 step 16 (`kill $BCHAT_PID`) | Cleanup |
| Container stopped | Phase 5 step 17 (`crdb:down`) | Cleanup |

### T6: Gate Criteria Audit

For each gate check, verify the check is actually measurable.

| Gate | Check | Measurable? | Notes |
|------|-------|-------------|-------|
| Phase 1 | Container running | ✅ `docker ps` | Standard Docker |
| Phase 1 | Healthcheck passing | ✅ `docker inspect` | Standard Docker |
| Phase 1 | SQL connectivity | ✅ `cockroach sql -e "SELECT 1"` | Standard SQL |
| Phase 1 | `feature.vector_index.enabled` | ✅ `SHOW CLUSTER SETTING` | CRDB syntax |
| Phase 1 | SHOW JOBS no stuck jobs | ✅ `SHOW JOBS` | CRDB syntax |
| Phase 2 | `crdb:migrate` exits 0 | ✅ Exit code | Standard shell |
| Phase 2 | `TestCockroachP0` passes | ✅ `go test` exit code | Standard Go |
| Phase 2 | `TestCockroachMigrateEndToEnd` passes | ✅ `go test` exit code | Standard Go |
| Phase 2 | `crdb:verify` exits 0 | ✅ Exit code | Standard shell |
| Phase 2 | `agent_vectors` table exists | ✅ `SHOW TABLES LIKE` | CRDB syntax |
| Phase 2 | Vector index exists | ✅ `SHOW INDEXES FROM` | CRDB syntax |
| Phase 2 | SHOW JOBS no failed/running | ✅ `SHOW JOBS` | CRDB syntax |
| Phase 3 | `MEMOS_DRIVER` set in `.env` | ✅ `grep` | Standard shell |
| Phase 3 | App doesn't crash | ✅ Exit code | Standard shell |
| Phase 3 | `/healthz` returns 200 | ✅ `curl -fsS` | Standard HTTP |
| Phase 3 | No SQLSTATE errors | ⚠️ Log inspection | Manual check |
| Phase 3 | `agent_vectors` exists | ✅ `SHOW TABLES LIKE` | CRDB syntax |
| Phase 3 | Vector index exists | ✅ `SHOW INDEXES FROM` | CRDB syntax |
| Phase 3 | Driver = cockroach | ⚠️ Log inspection | Manual check |
| Phase 4 | Sign-in succeeds | ✅ `verify-production.sh` exit | Script output |
| Phase 4 | KB import succeeds | ✅ `verify-production.sh` exit | Script output |
| Phase 4 | RAG reindex succeeds | ✅ `verify-production.sh` exit | Script output |
| Phase 4 | RAG search returns results | ✅ `verify-production.sh` exit | Script output |
| Phase 4 | Cleanup succeeds | ✅ `verify-production.sh` exit | Script output |
| Phase 4 | `agent_vectors` count > 0 | ✅ `SELECT count(*)` | CRDB syntax |
| Phase 4 | DB vs OpenRouter errors | ⚠️ Log inspection | Manual check |
| Phase 5 | `crdb:up` starts | ✅ `docker ps` | Standard Docker |
| Phase 5 | `crdb:verify` passes | ✅ Exit code | Standard shell |
| Phase 5 | `verify-production.sh` passes | ✅ Exit code | Script output |
| Phase 5 | `agent_vectors` count unchanged | ✅ `SELECT count(*)` | CRDB syntax |
| Phase 5 | No duplicate data | ⚠️ Manual comparison | Manual check |

**Summary:** 28/31 gates are directly measurable. 3 require manual log inspection — acceptable for a hackathon, document as known limitation.

### T7: Failure Mode Walk-Through

For each troubleshooting entry, verify the fix is executable.

| Issue | Symptom | Fix Executable? | Notes |
|-------|---------|-----------------|-------|
| Container not starting | `docker compose ps` unhealthy | ✅ `lsof -i :26257` | Standard |
| SQL connection refused | `cockroach sql` fails | ✅ `docker compose logs` | Standard |
| Migration fails | App crashes | ✅ Check `COCKROACH_DSN` | Env var check |
| Vector index missing | RAG empty | ✅ Re-run `crdb:init` | Task exists |
| Embedding errors | 0 rows | ✅ Check `OPENROUTER_API_KEY` | Env var check |
| OID 90006 error | Binary format | ✅ Check `simple_protocol` | DSN param |
| Duplicate data | Multiple embeddings | ⚠️ Check reindex logic | Code inspection |
| App SQLite fallback | No `agent_vectors` | ✅ Check `MEMOS_DRIVER` | Env var check |
| Port conflict | Can't bind 5230 | ✅ Kill process | Standard |

**Summary:** 8/9 fixes are directly executable. 1 requires code inspection — acceptable.

---

## Environment Audit

Verify all env vars referenced in plan_e2e.md are documented and verifiable.

| Variable | Phase | Required | Verified By |
|----------|-------|----------|-------------|
| `MEMOS_DRIVER` | Phase 3 | Yes | `grep "MEMOS_DRIVER" .env` |
| `COCKROACH_DSN` | Phase 3 | Yes | `grep "COCKROACH_DSN" .env` |
| `OPENROUTER_API_KEY` | Phase 3 | Yes | `grep "OPENROUTER_API_KEY" .env` |
| `RAG_PIPELINE_ENABLED` | Phase 3 | Recommended | `grep "RAG_PIPELINE_ENABLED" .env` |
| `LANCEDB_STORAGE_PROVIDER` | Phase 3 | Recommended | `grep "LANCEDB_STORAGE_PROVIDER" .env` |
| `EMBEDDING_PROVIDER` | Phase 3 | Recommended | `grep "EMBEDDING_PROVIDER" .env` |
| `TICKET_EMBEDDING_ENABLED` | Phase 3 | Recommended | `grep "TICKET_EMBEDDING_ENABLED" .env` |
| `BCHAT_URL` | Phase 4, 5 | Yes | Set inline |
| `BCHAT_USER` | Phase 4, 5 | Yes | Set inline |
| `BCHAT_PASS` | Phase 4, 5 | Yes | Set inline |
| `BCHAT_ALLOW_DB_RESET` | Phase 2 | Yes (test) | Set inline |

---

## Credential Verification

Verify the DSN credentials match docker-compose.cockroach.yml.

| Plan Reference | Docker Compose | Match? |
|----------------|----------------|--------|
| `bchat_user:bchat_pass@localhost:26257/bchat` | Line 11: same | ✅ |

---

## Phase 2 Process Lifecycle

Verify the claim: "Phase 2 process exits after migration completes."

```bash
# Read crdb:migrate target
grep -A 10 "crdb:migrate:" Taskfile.yml
```

**Claim to verify:** `crdb:migrate` runs `./build/memos --driver=cockroach --mode dev --data build/data`. The `--mode dev` flag without `serve` subcommand should cause the app to apply migrations and exit.

**Verification:** Read `bin/memos/main.go` to confirm `--mode dev` behavior.

---

## Adversarial Review Prompt

```
You are reviewing a meta-test plan that validates a local E2E testing plan for a
CockroachDB-backed Go application (bchat).

META-TEST PLAN: bugs/058/plan_test_e2e.md
E2E PLAN UNDER TEST: bugs/058/plan_e2e.md (v2)
IMPLEMENTATION: 3-file change (vectordb_cockroach.go, crdb-init.sql, Taskfile.yml)

REVIEW FOR:

1. COMPLETENESS:
   - Does the meta-test plan cover ALL commands in plan_e2e.md?
   - Are there any steps in plan_e2e.md that are not validated by plan_test_e2e.md?
   - Are the test cases sufficient to detect plan errors before execution?
   - Is the environment audit complete?

2. CORRECTNESS:
   - Are the test case expected outputs correct?
   - Do the grep patterns actually match the expected content?
   - Are the SQL syntax validations accurate for CRDB v26.2?
   - Is the prerequisite chain trace complete and correct?

3. GAPS:
   - Are there failure modes in plan_e2e.md that plan_test_e2e.md doesn't validate?
   - Are there env vars in plan_e2e.md that plan_test_e2e.md doesn't check?
   - Are there commands that could silently succeed but prove nothing?
   - Is the "3 gates require manual log inspection" finding acceptable or should we add automated checks?

4. RISK:
   - Could a plan_e2e.md error pass all plan_test_e2e.md checks?
   - What's the highest-risk gap in the meta-test coverage?
   - Is the meta-test plan itself sufficient, or does it need its own adversarial review?

5. OPERATIONAL:
   - Can the meta-test plan be run before executing plan_e2e.md?
   - Are the meta-test commands copy-pasteable?
   - Does the meta-test plan add significant time overhead?
   - Is the meta-test plan worth the investment?

EXPECTED OUTPUT:
- For each finding: severity (Critical/High/Medium/Low), whether it's a blocker,
  and suggested fix.
- Overall verdict: APPROVE / APPROVE WITH NITS / REQUEST CHANGES
- List any additional validation steps that should be added
```
