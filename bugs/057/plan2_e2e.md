# Bug 057 — E2E Test Safety Fix & Remaining Execution Plan (plan2_e2e)

**Date:** 2026-08-02  
**Status:** Implementation-ready plan (no code changes yet)  
**Prerequisite:** session-057.md through Phase 4 recovery complete; Cloud DB healthy (57 tables, history=1, healthz 200); verify-production.sh signin payload already fixed.

---

## 1. Background

`task crdb:verify` runs two Go test suites:
1. `TestCockroachP0` — validates `SET serial_normalization='sql_sequence'` produces `nextval()` defaults; fast (~372s on Cloud, ~1s local); non-destructive.
2. `TestCockroachMigrateEndToEnd` — calls `resetCockroachDB()` which **drops every table** in the DSN database, then re-runs full migration. Destructive by design.

During session-057 Phase 4, COCKROACH_DSN was sourced from .env (pointing at the production Cloud DB) before running task crdb:verify. The E2E test ran against Cloud, dropping 20 tables before being killed at 900s. Recovery: fly machine restart → idempotent re-migration restored all 57 tables in ~2 min. Admin user was lost and re-created.

**Root cause:** crdb:verify runs the destructive E2E test unconditionally. When COCKROACH_DSN points to Cloud, it wipes production.

---

## 2. Safety Fix — Explicit Opt-In (Required Change from Review C1+C2)

**Decision:** Replace the hostname-based heuristic with a dual guard:
1. **Test code guard:** The E2E test itself checks for BCHAT_ALLOW_DB_RESET=1 env var and skips if not set. This protects against direct go test runs, IDE runs, CI, etc.
2. **Taskfile guard:** crdb:verify sets BCHAT_ALLOW_DB_RESET=1 only when running the E2E tests against localhost. For Cloud DSN, the env var is not set, so the test skips itself.

### Why this over the original hostname-only guard
The original guard (grep -q "localhost|127.0.0.1") is a heuristic. SSH tunnels, port-forwards, VPNs, and Docker mappings can expose production on localhost. An explicit env var opt-in is unambiguous: the operator must consciously acknowledge they are about to reset a database.

### Guard behavior matrix

| COCKROACH_DSN | BCHAT_ALLOW_DB_RESET | E2E tests run? | §6.2 SQL checks |
|---------------|----------------------|----------------|-----------------|
| unset | unset | run (localhost default) | skip |
| localhost:26257/... | 1 (set by Taskfile) | run | run |
| 127.0.0.1:26257/... | 1 (set by Taskfile) | run | run |
| great-goat-30894.... (Cloud) | unset | skip (test self-skips) | run |
| any other remote host | unset | skip (test self-skips) | run |

### Test code change (in store/test/cockroach_migrate_test.go)

At the top of TestCockroachMigrateEndToEnd and TestCockroachBootIdempotency:

```go
if os.Getenv("BCHAT_ALLOW_DB_RESET") != "1" {
    t.Skip("BCHAT_ALLOW_DB_RESET=1 required to run destructive migration reset test")
}
```

### Taskfile change (Taskfile.yml → crdb:verify)

```yaml
crdb:verify:
  cmds:
    - |
      echo "=== CockroachDB Verification (P1-P6) ==="
      go test -tags "cockroach integration" ./store/ -run TestCockroachP0 -v
      if [ -z "${COCKROACH_DSN:-}" ] || echo "$COCKROACH_DSN" | grep -qE "localhost|127\.0\.0\.1"; then
        BCHAT_ALLOW_DB_RESET=1 go test -tags "cockroach integration" ./store/test/ -run "TestCockroachMigrate" -v
      else
        echo "SKIP: E2E migrate tests (resetCockroachDB) — COCKROACH_DSN is non-local (${COCKROACH_DSN:0:40}...)"
      fi
      echo ""
      echo "--- §6.2 checks (env-gated on COCKROACH_DSN + cockroach binary) ---"
      if [ -z "${COCKROACH_DSN:-}" ]; then
        echo "COCKROACH_DSN not set — skipping §6.2 SQL checks"
      elif ! command -v cockroach &>/dev/null; then
        echo "cockroach binary not found — skipping §6.2 SQL checks"
      else
        run_sql() { cockroach sql --url "${COCKROACH_DSN}" -e "$1" 2>/dev/null; }
        run_sql "SELECT 1;" >/dev/null || { echo "FAIL: SELECT 1"; exit 1; }
        echo "OK: SELECT 1"
        DB=$(run_sql "SELECT current_database();" 2>/dev/null | tail -1)
        echo "OK: current_database() = $DB"
        V=$(run_sql "SELECT version();")
        echo "$V" | grep -qi cockroach || { echo "FAIL: version() is not Cockroach"; exit 1; }
        echo "OK: version() = Cockroach"
        H=$(run_sql "SELECT count(*) FROM migration_history;")
        echo "$H" | grep -q "1" || { echo "FAIL: migration_history count != 1"; exit 1; }
        echo "OK: migration_history = 1 row (A1)"
        C=$(run_sql "SHOW CREATE TABLE agent_tenants;")
        echo "$C" | grep -q "nextval" || { echo "FAIL: agent_tenants has no nextval default"; exit 1; }
        echo "OK: nextval() defaults present"
        S=$(run_sql "SHOW CLUSTER SETTING feature.vector_index.enabled;")
        echo "$S" | grep -q "true" || { echo "FAIL: feature.vector_index.enabled != true"; exit 1; }
        echo "OK: vector index enabled"
        I=$(run_sql "SELECT count(*) FROM information_schema.statistics WHERE table_name='agent_vectors';")
        echo "$I" | grep -qv "^0" || { echo "FAIL: agent_vectors has no indexes"; exit 1; }
        echo "OK: agent_vectors indexed"
        if [ -n "${BCHAT_URL:-}" ]; then
          curl -fsS -o /dev/null "$BCHAT_URL/healthz" 2>/dev/null \
            || { echo "FAIL: /healthz not 200 at $BCHAT_URL"; exit 1; }
          echo "OK: /healthz 200"
        else
          echo "WARN: BCHAT_URL not set — skipping /healthz check"
        fi
      fi
      echo ""
      echo "P1-P6 verification complete!"
```

### Safety Principles

1. Production verification must be read-only.
2. Destructive tests require explicit opt-in (BCHAT_ALLOW_DB_RESET=1).
3. Database reset operations must never rely solely on DSN parsing.
4. CI may enable destructive tests automatically in ephemeral databases.
5. Human operators must explicitly acknowledge destructive operations.

---

## 3. Remaining Execution Steps

### Step 1: verify:production (Stage 7)
Run the app-first smoke against the deployed Cloud instance. The signin payload fix is already in place (scripts/verify-production.sh line 40 uses password_credentials wrapper).

```bash
BCHAT_URL=https://bchat-crdb.fly.dev BCHAT_USER=admin BCHAT_PASS="Memos@2026-admin" \
  bash scripts/verify-production.sh
```

Expected: all 7 steps pass (healthz → signin → tenant select → onboard → KB import + reindex → RAG search → cleanup). Test tenant destroyed on exit.

Use --keep for inspection:
```bash
BCHAT_URL=https://bchat-crdb.fly.dev BCHAT_USER=admin BCHAT_PASS="Memos@2026-admin" \
  bash scripts/verify-production.sh --keep
```

### Step 2: Harden crdb:verify (safety fix)
Two changes:

A. Test code guard (store/test/cockroach_migrate_test.go): Add BCHAT_ALLOW_DB_RESET=1 skip check at the top of TestCockroachMigrateEndToEnd and TestCockroachBootIdempotency.

B. Taskfile guard (Taskfile.yml → crdb:verify): Set BCHAT_ALLOW_DB_RESET=1 only when DSN is localhost; add SELECT current_database(); make healthz required when BCHAT_URL is set.

Verification:
```bash
# Local DSN — E2E tests should run (Taskfile sets BCHAT_ALLOW_DB_RESET=1)
COCKROACH_DSN="postgresql://bchat_user@localhost:26257/bchat?sslmode=disable" \
  task crdb:verify

# Cloud DSN — E2E tests should skip (test self-skips), §6.2 SQL checks should run
COCKROACH_DSN="postgresql://bchat_user:<pw>@great-goat-30894.j77.cockroachlabs.cloud:26257/bchat?sslmode=verify-full" \
  BCHAT_URL=https://bchat-crdb.fly.dev \
  task crdb:verify

# Direct go test without opt-in — should skip
COCKROACH_DSN="postgresql://bchat_user@localhost:26257/bchat?sslmode=disable" \
  go test -tags "cockroach integration" ./store/test/ -run "TestCockroachMigrate" -v
# Expected: "SKIP: BCHAT_ALLOW_DB_RESET=1 required..."

# Direct go test with opt-in — should run
BCHAT_ALLOW_DB_RESET=1 \
  go test -tags "cockroach integration" ./store/test/ -run "TestCockroachMigrate" -v
```

### Step 3: crdb:verify §6.2 SQL checks (Cloud)
Run via the hardened Taskfile (Step 2B) or directly:

```bash
DSN="postgresql://bchat_user:<pw>@great-goat-30894.j77.cockroachlabs.cloud:26257/bchat?sslmode=verify-full"
cockroach sql --url "$DSN" -e "SELECT 1;"
cockroach sql --url "$DSN" -e "SELECT current_database();"  # must be "bchat"
cockroach sql --url "$DSN" -e "SELECT version();"  # must contain "Cockroach"
cockroach sql --url "$DSN" -e "SELECT count(*) FROM migration_history;"  # must be 1
cockroach sql --url "$DSN" -e "SHOW CREATE TABLE agent_tenants;"  # must show nextval
cockroach sql --url "$DSN" -e "SHOW CLUSTER SETTING feature.vector_index.enabled;"  # must be true
cockroach sql --url "$DSN" -e "SELECT count(*) FROM information_schema.statistics WHERE table_name='agent_vectors';"  # must be > 0
```

### Step 4: Incident documentation
Add to bugs/057/artifacts/phase4/completion-report.md:

```markdown
## Incident: Destructive Test Executed Against Production Database

**Time:** 2026-08-02 ~12:30 UTC  
**Severity:** Destructive test executed against production database; service recovered via idempotent migration.  
**Impact:** 20 tables dropped from production Cloud DB (including migration_history, user). Admin user lost.  
**Root cause:** task crdb:verify runs TestCockroachMigrateEndToEnd which calls resetCockroachDB() (drops all tables). When COCKROACH_DSN was sourced from .env (Cloud endpoint), the test wiped production.  
**Why recovery succeeded:** The migration is fully idempotent — all DDL uses IF NOT EXISTS, and preMigrate re-runs the full LATEST.sql when migration_history is empty. The 37 surviving tables were skipped; only the 20 missing tables were recreated (~2 min on Cloud).  
**Permanent fix:** 
- Test code: BCHAT_ALLOW_DB_RESET=1 env var guard inside TestCockroachMigrateEndToEnd and TestCockroachBootIdempotency — protects against direct go test runs.
- Taskfile: crdb:verify sets BCHAT_ALLOW_DB_RESET=1 only for localhost DSN; skips with warning for remote DSN.
- Safety principle: destructive tests require explicit opt-in; never rely solely on DSN parsing.
```

### Step 5: Phase 5 close-out
After all verification passes:

1. Flip FORCE_REINDEX_ON_STARTUP in fly_cockroach.toml:
   ```toml
   FORCE_REINDEX_ON_STARTUP = 'false'
   ```
   Rationale: dead var, short-circuited by RAG_STARTUP_REINDEX_DISABLED=true. Setting to false removes confusion.

2. Flip auto_stop_machines back to 'stop' in fly_cockroach.toml:
   ```toml
   auto_stop_machines = 'stop'
   ```
   Rationale: migration is complete; restore intended cost profile (scale-to-zero on idle). The idempotent migration handles any future restarts.

3. Redeploy:
   ```bash
   task deploy:cockroach
   ```

4. Update documentation:
   - bugs/057/pending.md → mark complete
   - bugs/057/code.md → note the verify-production.sh fix and Taskfile safety guard
   - docs/docs_flyio_cockroach_deploy.md → note the auto_stop flip in Phase 5

---

## 4. Execution Order

| Step | Action | Blocking? |
|------|--------|-----------|
| 1 | verify:production (bash script) | Yes — must pass before Phase 5 |
| 2 | Harden E2E tests: add BCHAT_ALLOW_DB_RESET=1 guard to test code + Taskfile | Yes — safety fix before any further manual runs |
| 3 | crdb:verify §6.2 SQL checks against Cloud (via hardened Taskfile) | Yes — validates production schema |
| 4 | Document incident in artifacts/phase4/completion-report.md | No — can be done in parallel |
| 5 | Phase 5 close-out (flip FORCE_REINDEX + auto_stop, redeploy) | Yes — final step |

---

## 5. Changes Summary

| File | Change | Why |
|------|--------|-----|
| store/test/cockroach_migrate_test.go | Add BCHAT_ALLOW_DB_RESET=1 skip guard to TestCockroachMigrateEndToEnd and TestCockroachBootIdempotency | Prevents direct go test runs from wiping any DB without explicit opt-in |
| Taskfile.yml → crdb:verify | Set BCHAT_ALLOW_DB_RESET=1 only for localhost DSN; add SELECT current_database(); make healthz required when BCHAT_URL set | Taskfile-level guard + stronger verification |
| scripts/verify-production.sh | Already fixed (signin payload password_credentials) | Prerequisite from earlier session |
| bugs/057/artifacts/phase4/completion-report.md | Add incident section with recovery explanation | Operational learning |

---

## 6. Review Findings Incorporated

| Finding | Verdict | Where Addressed |
|---------|---------|-----------------|
| C1: Hostname-based safety is a heuristic | **Adopted** — replaced with explicit BCHAT_ALLOW_DB_RESET=1 env var opt-in | §2 |
| C2: Destructive test should declare its intent | **Adopted** — guard in test code itself, not just Taskfile | §2 |
| H2: SELECT 1 does not validate intended database | **Adopted** — add SELECT current_database() to §6.2 checks | Step 3 |
| H3: Recovery documentation | **Adopted** — add "Why recovery succeeded" section to incident doc | Step 4 |
| M2: Health check should be required, not WARN, for Cloud | **Adopted** — fail if BCHAT_URL set and /healthz not 200 | Taskfile change |
| M3: Incident severity wording | **Adopted** — "Destructive test executed against production database; service recovered via idempotent migration" | Step 4 |
| H1: Split verify tasks | **Deferred** — post-hackathon; keep single task for now | Out of scope |
| M1: Verify sequence name in SHOW CREATE TABLE | **Noted** — low value for hackathon; can add later | Out of scope |
| N1: Option B is correct choice | **Confirmed** — Option B with explicit opt-in | §2 |

---

## 7. Open Decisions

None pending. All review findings are either adopted or explicitly deferred.

---

## 8. References

| Artifact | Location |
|----------|----------|
| Session transcript | session-057.md |
| Deploy plan (Rev 4) | plan4_deploy.md |
| Implementation plan | pre_code.md |
| Review of this plan | plan_e2e_review.md |
| Dry-run evidence | artifacts/dryrun/evidence.md |
| Phase 2 evidence | artifacts/phase2/evidence.md |
| Phase 4 sampler log | artifacts/phase4/sampler.log |
| Fly config | fly_cockroach.toml |
| Deploy chain | scripts/crdb-deploy.sh |
| Smoke test | scripts/verify-production.sh |
| Taskfile | Taskfile.yml |
