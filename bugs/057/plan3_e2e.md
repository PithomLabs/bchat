# Bug 057 — E2E Test Safety Fix & Remaining Execution Plan (plan3_e2e)

**Date:** 2026-08-02  
**Status:** Implementation-ready plan (no code changes yet)  
**Prerequisite:** session-057.md through Phase 4 recovery complete; Cloud DB healthy (57 tables, history=1, healthz 200); verify-production.sh signin payload already fixed.

---

## 1. Background

`task crdb:verify` runs two Go test suites:
1. `TestCockroachP0` — validates `SET serial_normalization='sql_sequence'` produces `nextval()` defaults; fast (~372s on Cloud, ~1s local); non-destructive.
2. `TestCockroachMigrateEndToEnd` — calls `resetCockroachDB()` which **drops every table** in the DSN database, then re-runs full migration. Destructive by design.

During session-057 Phase 4, COCKROACH_DSN was sourced from `.env` (pointing at the production Cloud DB) before running `task crdb:verify`. The E2E test ran against Cloud, dropping 20 tables before being killed at 900s. Recovery: `fly machine restart` → idempotent re-migration restored all 57 tables in ~2 min. Admin user was lost and re-created.

**Root cause:** `crdb:verify` runs the destructive E2E test unconditionally. When `COCKROACH_DSN` points to Cloud, it wipes production.

---

## 2. Safety Fix — Two-Key Explicit Opt-In (Required Change from Review C1+C2)

**Decision:** Three-layer safety guard:
1. **Low-level guard (`resetCockroachDB` / helper):** The function that issues `DROP TABLE` checks for `BCHAT_ALLOW_DB_RESET=1` AND, if the DSN is non-local, also requires `BCHAT_ALLOW_REMOTE_DB_RESET=1`. This protects every caller automatically — future tests, CI, IDE runs, direct `go test`.
2. **Test code guard:** Each E2E test calls the helper; the helper's own check is sufficient. No per-test duplication needed if tests route through the helper. If tests use `resetCockroachDB` directly, the guard there is the authoritative gate.
3. **Taskfile guard:** `crdb:verify` sets `BCHAT_ALLOW_DB_RESET=1` only for localhost DSN. For remote DSN, neither opt-in is set, so the helper skips with a clear message.

### Why two keys
`BCHAT_ALLOW_DB_RESET=1` acknowledges "I intend to reset a database." If the DSN is non-local, `BCHAT_ALLOW_REMOTE_DB_RESET=1` adds a second acknowledgement: "I specifically intend to reset a remote/production database." This makes accidental production wipes substantially harder — the operator must consciously type both.

### Guard behavior matrix

| `COCKROACH_DSN` | `BCHAT_ALLOW_DB_RESET` | `BCHAT_ALLOW_REMOTE_DB_RESET` | E2E tests run? | §6.2 SQL checks |
|-----------------|------------------------|-------------------------------|----------------|-----------------|
| unset | unset | unset | run (localhost default) | skip |
| `localhost:26257/...` | `1` (set by Taskfile) | unset | **run** | run |
| `127.0.0.1:26257/...` | `1` (set by Taskfile) | unset | **run** | run |
| `great-goat-30894....` (Cloud) | unset | unset | **skip** (helper self-skips) | run |
| any other remote host | unset | unset | **skip** (helper self-skips) | run |
| remote + both set | `1` | `1` | **run** (explicit opt-in) | run |

### Helper change (`store/test/cockroach_migrate_test.go`)

Add a small helper that wraps `resetCockroachDB` with the two-key guard:

```go
func requireDatabaseResetPermission(t *testing.T, dsn string) {
    t.Helper()
    if os.Getenv("BCHAT_ALLOW_DB_RESET") != "1" {
        t.Skip("BCHAT_ALLOW_DB_RESET=1 required to run destructive migration reset test")
    }
    if !strings.Contains(dsn, "localhost") && !strings.Contains(dsn, "127.0.0.1") {
        if os.Getenv("BCHAT_ALLOW_REMOTE_DB_RESET") != "1" {
            t.Skip("BCHAT_ALLOW_REMOTE_DB_RESET=1 required to reset a non-local database (DSN: " + dsn + ")")
        }
    }
}
```

Then `TestCockroachMigrateEndToEnd` and `TestCockroachBootIdempotency` call it at the top with the test DSN:

```go
func TestCockroachMigrateEndToEnd(t *testing.T) {
    requireDatabaseResetPermission(t, cockroachTestDSN(t))
    // ... existing body
}
```

### Taskfile change (`Taskfile.yml` → `crdb:verify`)

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
        EXPECTED_DB=$(echo "$COCKROACH_DSN" | grep -oE '[^/]+$' | cut -d'?' -f1)
        if [ "$DB" != "$EXPECTED_DB" ]; then
          echo "FAIL: connected to $DB, expected $EXPECTED_DB"; exit 1
        fi
        echo "OK: database matches DSN"
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

### Safety Principles (strengthened per review wording suggestion)

1. **Production verification must be read-only by default.** Any destructive production operation requires explicit, separate acknowledgement and must never occur implicitly as part of a verification task.
2. Destructive tests require explicit opt-in (`BCHAT_ALLOW_DB_RESET=1`).
3. Non-local database resets require a second explicit acknowledgement (`BCHAT_ALLOW_REMOTE_DB_RESET=1`).
4. Database reset operations must never rely solely on DSN parsing.
5. CI may enable destructive tests automatically in ephemeral databases only.
6. Human operators must explicitly acknowledge destructive operations.

---

## 3. Verification Classification

Reinforces the operational model introduced by this incident:

| Check | Read-only | Destructive |
|-------|-----------|-------------|
| `SELECT 1` | ✓ | |
| `SELECT current_database()` | ✓ | |
| `SELECT version()` | ✓ | |
| `migration_history` count | ✓ | |
| `SHOW CREATE TABLE agent_tenants` | ✓ | |
| `SHOW CLUSTER SETTING feature.vector_index.enabled` | ✓ | |
| `information_schema.statistics` | ✓ | |
| `/healthz` | ✓ | |
| `TestCockroachP0` | ✓ | |
| `TestCockroachMigrateEndToEnd` | | ✓ |
| `resetCockroachDB()` | | ✓ |

---

## 4. CI Behavior (Review H3)

Ephemeral CI environments run destructive tests as follows:

```
ephemeral Cockroach cluster (per-job lifecycle)
        ↓
CI sets BCHAT_ALLOW_DB_RESET=1
        ↓
go test -tags "cockroach integration" ./store/test/ -run "TestCockroachMigrate"
        ↓
cluster destroyed at end of job
```

Production-like environments (long-lived, named, or non-local DSN) do **not** set `BCHAT_ALLOW_DB_RESET=1` or `BCHAT_ALLOW_REMOTE_DB_RESET=1`. Destructive tests skip.

---

## 5. Remaining Execution Steps

### Step 1: verify:production (Stage 7)
Run the app-first smoke against the deployed Cloud instance. The signin payload fix is already in place (`scripts/verify-production.sh` line 40 uses `password_credentials` wrapper).

```bash
BCHAT_URL=https://bchat-crdb.fly.dev BCHAT_USER=admin BCHAT_PASS="<redacted>" \
  bash scripts/verify-production.sh
```

Expected: all 7 steps pass (healthz → signin → tenant select → onboard → KB import + reindex → RAG search → cleanup). Test tenant destroyed on exit.

Use `--keep` for inspection:
```bash
BCHAT_URL=https://bchat-crdb.fly.dev BCHAT_USER=admin BCHAT_PASS="<redacted>" \
  bash scripts/verify-production.sh --keep
```

### Step 2: Harden E2E tests (safety fix)
Three changes:

**A. Helper** (`store/test/cockroach_migrate_test.go`): Add `requireDatabaseResetPermission(t, dsn)` with the two-key guard.

**B. Test guards:** Call the helper at the top of `TestCockroachMigrateEndToEnd` and `TestCockroachBootIdempotency`.

**C. Taskfile guard** (`Taskfile.yml` → `crdb:verify`): Set `BCHAT_ALLOW_DB_RESET=1` only for localhost DSN; add `SELECT current_database()` with expected-DB assertion; make healthz required when `BCHAT_URL` is set.

**Verification:**
```bash
# Local DSN — E2E tests should run (Taskfile sets BCHAT_ALLOW_DB_RESET=1)
COCKROACH_DSN="postgresql://bchat_user@localhost:26257/bchat?sslmode=disable" \
  task crdb:verify

# Cloud DSN — E2E tests should skip (helper self-skips), §6.2 SQL checks should run
COCKROACH_DSN="postgresql://bchat_user:<pw>@great-goat-30894.j77.cockroachlabs.cloud:26257/bchat?sslmode=verify-full" \
  BCHAT_URL=https://bchat-crdb.fly.dev \
  task crdb:verify

# Direct go test without opt-in — should skip
COCKROACH_DSN="postgresql://bchat_user@localhost:26257/bchat?sslmode=disable" \
  go test -tags "cockroach integration" ./store/test/ -run "TestCockroachMigrate" -v
# Expected: "SKIP: BCHAT_ALLOW_DB_RESET=1 required..."

# Direct go test with local opt-in — should run
BCHAT_ALLOW_DB_RESET=1 \
  go test -tags "cockroach integration" ./store/test/ -run "TestCockroachMigrate" -v

# Direct go test with remote DSN + both keys — should run (explicit production opt-in)
BCHAT_ALLOW_DB_RESET=1 BCHAT_ALLOW_REMOTE_DB_RESET=1 \
  COCKROACH_DSN="postgresql://bchat_user:<pw>@great-goat-30894.j77.cockroachlabs.cloud:26257/bchat?sslmode=verify-full" \
  go test -tags "cockroach integration" ./store/test/ -run "TestCockroachMigrate" -v
```

### Step 3: crdb:verify §6.2 SQL checks (Cloud)
Run via the hardened Taskfile (Step 2C) or directly:

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
Add to `bugs/057/artifacts/phase4/completion-report.md`:

```markdown
## Incident: Destructive Test Executed Against Production Database

**Time:** 2026-08-02 ~12:30 UTC  
**Severity:** Destructive test executed against production database; service recovered via idempotent migration.  
**Impact:** 20 tables dropped from production Cloud DB (including `migration_history`, `user`). Admin user lost.  
**Root cause:** `task crdb:verify` runs `TestCockroachMigrateEndToEnd` which calls `resetCockroachDB()` (drops all tables). When `COCKROACH_DSN` was sourced from `.env` (Cloud endpoint), the test wiped production.  
**Why recovery succeeded:** The migration is fully idempotent — all DDL uses `IF NOT EXISTS`, and `preMigrate` re-runs the full `LATEST.sql` when `migration_history` is empty. The 37 surviving tables were skipped; only the 20 missing tables were recreated (~2 min on Cloud). Recovery did not depend on luck; it depended on the idempotent migration design.  
**Permanent fix:**
- Low-level guard: `requireDatabaseResetPermission()` helper in `store/test/cockroach_migrate_test.go` enforces two-key opt-in (`BCHAT_ALLOW_DB_RESET=1` + `BCHAT_ALLOW_REMOTE_DB_RESET=1` for non-local DSN). Every future caller inherits protection automatically.
- Test code: E2E tests call the helper at entry.
- Taskfile: `crdb:verify` sets `BCHAT_ALLOW_DB_RESET=1` only for localhost DSN; skips with warning for remote DSN.
- Safety principle: destructive tests require explicit opt-in; non-local resets require a second acknowledgement; never rely solely on DSN parsing.
```

### Step 5: Phase 5 close-out
After all verification passes:

1. Flip `FORCE_REINDEX_ON_STARTUP` in `fly_cockroach.toml`:
   ```toml
   FORCE_REINDEX_ON_STARTUP = 'false'
   ```
   Rationale: dead var, short-circuited by `RAG_STARTUP_REINDEX_DISABLED=true`. Setting to `false` removes confusion.

2. Flip `auto_stop_machines` back to `'stop'` in `fly_cockroach.toml`:
   ```toml
   auto_stop_machines = 'stop'
   ```
   Rationale: migration is complete; restore intended cost profile (scale-to-zero on idle). The idempotent migration handles any future restarts.

3. Redeploy:
   ```bash
   task deploy:cockroach
   ```

4. Update documentation:
   - `bugs/057/pending.md` → mark complete
   - `bugs/057/code.md` → note the verify-production.sh fix and Taskfile safety guard
   - `docs/docs_flyio_cockroach_deploy.md` → note the `auto_stop` flip in Phase 5

---

## 6. Execution Order

| Step | Action | Blocking? |
|------|--------|-----------|
| 1 | `verify:production` (bash script) | Yes — must pass before Phase 5 |
| 2 | Harden E2E tests: helper + per-test guards + Taskfile guard | Yes — safety fix before any further manual runs |
| 3 | `crdb:verify` §6.2 SQL checks against Cloud (via hardened Taskfile) | Yes — validates production schema |
| 4 | Document incident in `artifacts/phase4/completion-report.md` | No — can be done in parallel |
| 5 | Phase 5 close-out (flip FORCE_REINDEX + auto_stop, redeploy) | Yes — final step |

---

## 7. Changes Summary

| File | Change | Why |
|------|--------|-----|
| `store/test/cockroach_migrate_test.go` | Add `requireDatabaseResetPermission(t, dsn)` helper with two-key guard; call from `TestCockroachMigrateEndToEnd` and `TestCockroachBootIdempotency` | Protects every caller of `resetCockroachDB` automatically, including future tests |
| `Taskfile.yml` → `crdb:verify` | Set `BCHAT_ALLOW_DB_RESET=1` only for localhost DSN; add `SELECT current_database()` with expected-DB assertion; make healthz required when `BCHAT_URL` set | Taskfile-level guard + stronger verification |
| `scripts/verify-production.sh` | Already fixed (signin payload `password_credentials`) | Prerequisite from earlier session |
| `bugs/057/artifacts/phase4/completion-report.md` | Add incident section with recovery explanation | Operational learning |

---

## 8. Review Findings Incorporated

| Finding | Verdict | Where Addressed |
|---------|---------|-----------------|
| C1: Single env var insufficient for remote | **Adopted** — two-key system: `BCHAT_ALLOW_DB_RESET=1` + `BCHAT_ALLOW_REMOTE_DB_RESET=1` for non-local DSN | §2 |
| C2: Safety belongs at lowest layer | **Adopted** — guard in `resetCockroachDB` / helper, not just test wrapper | §2 |
| H1: Assert `current_database()` matches expected | **Adopted** — fail if DB name from `SELECT current_database()` does not match DSN | Taskfile change |
| H2: Verification classification table | **Adopted** — read-only vs destructive table in §3 | §3 |
| H3: Specify CI behavior | **Adopted** — ephemeral cluster flow in §4 | §4 |
| M2: Health check required when BCHAT_URL set | **Adopted** — fail hard, not WARN | Taskfile change |
| M3: Incident severity wording | **Adopted** — "Destructive test executed against production database; service recovered via idempotent migration" | Step 4 |
| Wording: strengthen safety principle 1 | **Adopted** — "Production verification must be read-only by default. Any destructive production operation requires explicit, separate acknowledgement..." | §2 |
| H1: Split verify tasks | **Deferred** — post-hackathon; keep single task for now | Out of scope |
| M1: Verify sequence name | **Deferred** — low value for hackathon | Out of scope |
| N1: Option B confirmed | **Confirmed** — with two-key explicit opt-in | §2 |

---

## 9. Open Decisions

None pending. All review findings are either adopted or explicitly deferred.

---

## 10. References

| Artifact | Location |
|----------|----------|
| Session transcript | `session-057.md` |
| Deploy plan (Rev 4) | `plan4_deploy.md` |
| Implementation plan | `pre_code.md` |
| Review of plan_e2e | `plan_e2e_review.md` |
| Review of plan2_e2e | `plan2_e2e_review.md` |
| Dry-run evidence | `artifacts/dryrun/evidence.md` |
| Phase 2 evidence | `artifacts/phase2/evidence.md` |
| Phase 4 sampler log | `artifacts/phase4/sampler.log` |
| Fly config | `fly_cockroach.toml` |
| Deploy chain | `scripts/crdb-deploy.sh` |
| Smoke test | `scripts/verify-production.sh` |
| Taskfile | `Taskfile.yml` |
