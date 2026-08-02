# Bug 057 — E2E Test Safety Fix & Remaining Execution Plan (plan_e2e)

**Date:** 2026-08-02  
**Status:** Implementation-ready plan (no code changes yet)  
**Prerequisite:** `session-057.md` through Phase 4 recovery complete; Cloud DB healthy (57 tables, history=1, healthz 200); `verify-production.sh` signin payload already fixed.

---

## 1. Background

`task crdb:verify` runs two Go test suites:
1. `TestCockroachP0` — validates `SET serial_normalization='sql_sequence'` produces `nextval()` defaults; fast (~372s on Cloud, ~1s local); non-destructive.
2. `TestCockroachMigrateEndToEnd` — calls `resetCockroachDB()` which **drops every table** in the DSN database, then re-runs full migration. Destructive by design.

During session-057 Phase 4, `COCKROACH_DSN` was sourced from `.env` (pointing at the production Cloud DB) before running `task crdb:verify`. The E2E test ran against Cloud, dropping 20 tables before being killed at 900s. Recovery: `fly machine restart` → idempotent re-migration restored all 57 tables in ~2 min. Admin user was lost and re-created.

**Root cause:** `crdb:verify` runs the destructive E2E test unconditionally. When `COCKROACH_DSN` points to Cloud, it wipes production.

---

## 2. Safety Fix — Option B: Guard E2E Tests (Chosen)

**Decision:** Keep the E2E tests in `task crdb:verify`, but skip them when `COCKROACH_DSN` points to a non-local endpoint. This preserves the convenience of `task crdb:verify` as a one-shot local gate while preventing accidental Cloud wipes.

### Why Option B over Option A (remove)
- **Option A (remove):** Simpler, but loses the one-shot local validation gate. Operators must remember to run the E2E tests separately with local DSN.
- **Option B (guard):** Zero additional complexity. The deploy chain already runs the tests without `COCKROACH_DSN` (defaults to `localhost:26257/bchat?sslmode=disable`), so the guard is a no-op in the normal flow. Manual runs with Cloud DSN get an explicit skip message.

### Guard logic
```yaml
# Taskfile.yml — crdb:verify task
crdb:verify:
  cmds:
    - |
      echo "=== CockroachDB Verification (P1-P6) ==="
      go test -tags "cockroach integration" ./store/ -run TestCockroachP0 -v
      # E2E migrate tests are local-only (they reset the DB). Skip if COCKROACH_DSN
      # points to a non-local endpoint to avoid wiping Cloud.
      if [ -z "${COCKROACH_DSN:-}" ] || echo "$COCKROACH_DSN" | grep -qE "localhost|127\.0\.0\.1"; then
        go test -tags "cockroach integration" ./store/test/ -run "TestCockroachMigrate" -v
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
        curl -fsS -o /dev/null "${BCHAT_URL:-http://localhost:5230}/healthz" 2>/dev/null \
          && echo "OK: /healthz 200" || echo "WARN: /healthz unreachable (set BCHAT_URL)"
      fi
      echo ""
      echo "P1-P6 verification complete!"
```

### Guard behavior matrix

| `COCKROACH_DSN` | E2E tests (`TestCockroachMigrate`) | §6.2 SQL checks |
|-----------------|-----------------------------------|-----------------|
| unset | run against `localhost:26257/bchat` (local compose) | skip |
| `localhost:26257/...` | run | run |
| `127.0.0.1:26257/...` | run | run |
| `great-goat-30894....` (Cloud) | **skip** with warning | run |
| any other remote host | **skip** with warning | run |

---

## 3. Remaining Execution Steps

### Step 1: verify:production (Stage 7)
Run the app-first smoke against the deployed Cloud instance. The signin payload fix is already in place (`scripts/verify-production.sh` line 40 uses `password_credentials` wrapper).

```bash
BCHAT_URL=https://bchat-crdb.fly.dev BCHAT_USER=admin BCHAT_PASS="Memos@2026-admin" \
  bash scripts/verify-production.sh
```

**Expected outcome:** All 7 steps pass (healthz → signin → tenant select → onboard → KB import + reindex → RAG search → cleanup). Test tenant destroyed on exit.

**Use `--keep` for inspection:**
```bash
BCHAT_URL=https://bchat-crdb.fly.dev BCHAT_USER=admin BCHAT_PASS="Memos@2026-admin" \
  bash scripts/verify-production.sh --keep
```

### Step 2: Harden `crdb:verify` (safety fix)
Edit `Taskfile.yml` `crdb:verify` task per §2 above. This is the **only code change** in this plan.

**Verification:**
```bash
# Local DSN — E2E tests should run (no change in behavior)
COCKROACH_DSN="postgresql://bchat_user@localhost:26257/bchat?sslmode=disable" \
  task crdb:verify

# Cloud DSN — E2E tests should skip, §6.2 SQL checks should run
COCKROACH_DSN="postgresql://bchat_user:<pw>@great-goat-30894.j77.cockroachlabs.cloud:26257/bchat?sslmode=verify-full" \
  task crdb:verify
```

### Step 3: crdb:verify §6.2 SQL checks (Cloud)
Run the SQL checks directly via `cockroach sql` (or via the hardened Taskfile). Verify:

| Check | Expected | SQL |
|--------|----------|-----|
| Connectivity | `SELECT 1` succeeds | `cockroach sql --url "$DSN" -e "SELECT 1;"` |
| Cockroach version | output contains "Cockroach" | `SELECT version();` |
| Migration history | count = 1 | `SELECT count(*) FROM migration_history;` |
| nextval defaults | `SHOW CREATE TABLE agent_tenants` contains `nextval` | `SHOW CREATE TABLE agent_tenants;` |
| Vector index enabled | `true` | `SHOW CLUSTER SETTING feature.vector_index.enabled;` |
| agent_vectors indexed | count > 0 | `SELECT count(*) FROM information_schema.statistics WHERE table_name='agent_vectors';` |

### Step 4: Incident documentation
Add a section to `bugs/057/artifacts/phase4/completion-report.md`:

```markdown
## Incident: E2E Test Against Cloud DSN

**Time:** 2026-08-02 ~12:30 UTC  
**Severity:** Data loss (recovered)  
**Impact:** 20 tables dropped from production Cloud DB (including `migration_history`, `user`). Admin user lost.  
**Root cause:** `task crdb:verify` runs `TestCockroachMigrateEndToEnd` which calls `resetCockroachDB()` (drops all tables). When `COCKROACH_DSN` was sourced from `.env` (Cloud endpoint), the test wiped production.  
**Recovery:** `fly machine restart` → idempotent re-migration restored all 57 tables in ~2 min. Admin user re-created via `/api/v1/auth/signup`.  
**Fix:** Guard E2E tests in `Taskfile.yml` `crdb:verify` to skip when `COCKROACH_DSN` points to a non-local endpoint. Deploy chain behavior unchanged.
```

### Step 5: Phase 5 close-out
After all verification passes:

1. **Flip `FORCE_REINDEX_ON_STARTUP`** in `fly_cockroach.toml`:
   ```toml
   FORCE_REINDEX_ON_STARTUP = 'false'
   ```
   Rationale: dead var, short-circuited by `RAG_STARTUP_REINDEX_DISABLED=true`. Setting to `false` removes confusion.

2. **Flip `auto_stop_machines`** back to `'stop'` in `fly_cockroach.toml`:
   ```toml
   auto_stop_machines = 'stop'
   ```
   Rationale: migration is complete; restore intended cost profile (scale-to-zero on idle). The idempotent migration handles any future restarts.

3. **Redeploy:**
   ```bash
   task deploy:cockroach
   ```

4. **Update documentation:**
   - `bugs/057/pending.md` → mark complete
   - `bugs/057/code.md` → note the verify-production.sh fix and Taskfile safety guard
   - `docs/docs_flyio_cockroach_deploy.md` → note the `auto_stop` flip in Phase 5

---

## 4. Execution Order

| Step | Action | Blocking? |
|------|--------|-----------|
| 1 | `verify:production` (bash script) | Yes — must pass before Phase 5 |
| 2 | Harden `Taskfile.yml` `crdb:verify` (guard E2E tests) | Yes — safety fix before any further manual runs |
| 3 | `crdb:verify` §6.2 SQL checks against Cloud | Yes — validates production schema |
| 4 | Document incident in `artifacts/phase4/completion-report.md` | No — can be done in parallel |
| 5 | Phase 5 close-out (flip FORCE_REINDEX + auto_stop, redeploy) | Yes — final step |

---

## 5. Open Decisions

None pending. The E2E test handling decision (Option B — guard) is recorded here as the chosen approach.

---

## 6. References

| Artifact | Location |
|----------|----------|
| Session transcript | `session-057.md` |
| Deploy plan (Rev 4) | `plan4_deploy.md` |
| Implementation plan | `pre_code.md` |
| Dry-run evidence | `artifacts/dryrun/evidence.md` |
| Phase 2 evidence | `artifacts/phase2/evidence.md` |
| Phase 4 sampler log | `artifacts/phase4/sampler.log` |
| Fly config | `fly_cockroach.toml` |
| Deploy chain | `scripts/crdb-deploy.sh` |
| Smoke test | `scripts/verify-production.sh` |
| Taskfile | `Taskfile.yml` |
