# Bug 058 — Adversarial Code Review

**Date:** 2026-08-03  
**Reviewer:** Senior Go / CockroachDB Architect  
**Artifacts under review:**
- `server/router/api/v1/agent/vectordb_cockroach.go` (line 112 — `IF NOT EXISTS` + SQLSTATE fallback)
- `scripts/crdb-init.sql` (new file)
- `Taskfile.yml` (`crdb:init`, `crdb:reset`, `crdb:verify`, `crdb:cluster:bootstrap`)
- `docs/docs_flyio_cockroach_deploy.md` (documentation drift)

**Context sources:**
- `bugs/058/plan.md`
- `bugs/058/plan2.md`
- `bugs/058/plan2_review_claude.md`
- `bugs/058/plan2_review_chatgpt.md`
- `bugs/058/plan3.md`
- `bugs/058/code.md`
- Live evidence in `bugs/058/evidence_20260803.md`

---

## Executive Summary

The implementation is **substantially correct** and reflects the iterative review feedback well. The three code changes solve a real operational problem (local CockroachDB E2E testing) with minimal surface area.

However, I am **not approving for merge yet** because there is one **High-severity correctness gap** in the deployment documentation and one **Medium-severity verification gap** in `crdb:verify`. Both are straightforward to fix.

**Verdict:** REQUEST CHANGES — 2 findings, both with targeted fixes.

---

## Finding 1 (High) — Deployment Runbook Points to Wrong Task

**File:** `docs/docs_flyio_cockroach_deploy.md`  
**Line:** 28  
**Severity:** High  
**Type:** Documentation drift / operational correctness

### What the doc says

```text
5. `task crdb:init` prints the same guided flow.
```

### What the code does

`crdb:init` is now the **local cluster-settings init** task:

```yaml
crdb:init:
  desc: Apply cluster settings to local CockroachDB (idempotent, safe to rerun)
  cmds:
    - |
      set -e
      echo "=== Waiting for CockroachDB SQL readiness ==="
      ...
```

The task that prints the guided flow is `crdb:cluster:bootstrap`:

```yaml
crdb:cluster:bootstrap:
  desc: Bootstrap a 2-region Basic cluster (console-first; see bugs/057/pre_code.md §5)
  cmds:
    - |
      echo "Multi-region Basic clusters are created in the Cloud Console ..."
      ...
```

### Impact

Anyone following `docs_flyio_cockroach_deploy.md` step 5 will run the local init task and see:

```
=== Waiting for CockroachDB SQL readiness ===
=== Applying cluster settings ===
=== Cluster settings applied ===
```

Instead of the expected guided cluster-bootstrap flow. This breaks the deploy runbook for cloud setup.

### Fix

Update `docs/docs_flyio_cockroach_deploy.md` line 28 to:

```text
5. `task crdb:cluster:bootstrap` prints the guided flow.
```

And update any other references in the docs if present.

---

## Finding 2 (Medium) — `crdb:verify` Treats Failed Schema Jobs as Warning, Not Failure

**File:** `Taskfile.yml`  
**Lines:** 372–378  
**Severity:** Medium  
**Type:** Verification completeness

### What the code does

```yaml
JOBS=$(run_sql "SELECT count(*) FROM [SHOW JOBS] WHERE status = 'failed' AND job_type IN ('SCHEMA CHANGE', 'NEW SCHEMA CHANGE');" 2>/dev/null | tail -1)
if [ "$JOBS" != "0" ] && [ -n "$JOBS" ]; then
  echo "WARN: $JOBS failed schema job(s) found"
  run_sql "SELECT job_id, job_type, status, error FROM [SHOW JOBS] WHERE status = 'failed' AND job_type IN ('SCHEMA CHANGE', 'NEW SCHEMA CHANGE') LIMIT 5;" 2>/dev/null
else
  echo "OK: no failed schema jobs"
fi
```

### Why this is a problem

`crdb:verify` is the production-facing verification task. A failed schema change job means the schema is in an inconsistent or partially-applied state. Treating this as a WARN while printing "P1-P6 verification complete!" at the end is misleading. The task should **fail** if any schema-change job has failed.

This is especially important for the cloud validation path, where a killed migration left 44 rows with 0 embeddings and a potentially stuck index.

### Fix

Change the WARN branch to FAIL:

```yaml
JOBS=$(run_sql "SELECT count(*) FROM [SHOW JOBS] WHERE status = 'failed' AND job_type IN ('SCHEMA CHANGE', 'NEW SCHEMA CHANGE');" 2>/dev/null | tail -1)
if [ "$JOBS" != "0" ] && [ -n "$JOBS" ]; then
  echo "FAIL: $JOBS failed schema job(s) found"
  run_sql "SELECT job_id, job_type, status, error FROM [SHOW JOBS] WHERE status = 'failed' AND job_type IN ('SCHEMA CHANGE', 'NEW SCHEMA CHANGE') LIMIT 5;" 2>/dev/null
  exit 1
else
  echo "OK: no failed schema jobs"
fi
```

---

## Items Reviewed and Approved

### 1. `vectordb_cockroach.go` — `CREATE VECTOR INDEX IF NOT EXISTS`

**Status:** ✅ Correct

- `IF NOT EXISTS` is supported in CRDB v26.1+ per docs and confirmed by live probe (`evidence_20260803.md` Task A.1).
- SQLSTATE `42P07` fallback retained as defense-in-depth for the race where two replicas attempt index creation simultaneously.
- SQLSTATE `0A000` fallback retained for the case where `feature.vector_index.enabled` is false.
- TODO comment is actionable and scoped to post-hackathon cleanup.
- Build tag `cockroach` ensures no impact on non-CRDB builds.

**One observation (not a blocker):**

`Validate()` is called during reindex (`service.go:1273-1274`), not on every boot. Reindex can be triggered by:
1. `FORCE_REINDEX_ON_STARTUP=true` (env-gated)
2. Auto-bootstrap when `TotalChunks == 0` and source files exist (line 233-257)
3. Admin API `POST /:slug/reindex` (handler line 1208)

Triggers 1 and 2 can fire concurrently across multiple replicas on a fresh deploy. The `IF NOT EXISTS` + SQLSTATE fallback is adequate for this case, but the **concurrent-startup test should remain on the Gate 0 checklist** (plan3.md correctly lists it as optional, not removed). The defense-in-depth argument holds.

### 2. `scripts/crdb-init.sql` — New Init Script

**Status:** ✅ Correct

- All `SET` statements are idempotent by CockroachDB design.
- `kv.range_merge.queue_interval` correctly removed (nonexistent in v26.2, confirmed by `42P02` in live probe).
- `serial_normalization = 'sql_sequence'` correctly uses `SET` without `CLUSTER SETTING` (session variable).
- Required vs dev-only settings are clearly separated with section comments.
- The dual-purpose `serial_normalization` documentation is accurate: `crdb-init.sql` covers manual sessions, `migrator.go` covers programmatic migrations.

### 3. `Taskfile.yml` — `crdb:init`, `crdb:reset`, `crdb:verify`, `crdb:cluster:bootstrap`

**Status:** ✅ Correct (with 1 nit documented in Finding 2)

**`crdb:init`:**
- `set -e` is present — fail-fast is correct.
- Retry loop (30 attempts × 2s = 60s max) correctly handles the gap between Docker healthcheck and SQL readiness.
- The retry loop is in the right place: inside `crdb:init`, not `crdb:up`, because readiness is a property of the database process, not the container.

**`crdb:reset`:**
- Chains `crdb:init` via `task: crdb:init` — correct Taskfile v3 dependency syntax.
- Uses `docker compose up -d --wait` — correct, ensures healthcheck passes before proceeding.
- Ordering is guaranteed sequential (`cmds:` entries run in order).

**`crdb:verify`:**
- SHOW JOBS check is present and correctly filters to schema-change jobs.
- **Issue:** See Finding 2 — WARN should be FAIL.

**`crdb:cluster:bootstrap`:**
- Renamed correctly from old `crdb:init` to avoid name collision.
- No other Taskfile targets or scripts reference the old `crdb:init` name (verified via grep across `.sh` and `.go` files).
- **Issue:** See Finding 1 — docs still reference old `crdb:init`.

---

## Correctness Summary by Dimension

| Dimension | Verdict | Notes |
|-----------|---------|-------|
| **Correctness** | ✅ Approved (pending doc fix) | `IF NOT EXISTS` confirmed by docs + live probe. Init script idempotent. |
| **Race conditions** | ✅ Adequate | `IF NOT EXISTS` + SQLSTATE fallback covers concurrent `Validate()`. Concurrent reindex is possible but handled. |
| **Security** | ✅ Approved | No user input in SQL. `sslmode=disable` is local-only. No credentials in Taskfile. |
| **Idempotency** | ✅ Approved | `crdb:reset` → `crdb:init` chain is safe to re-run. All SET statements are idempotent. |
| **Completeness** | ⚠️ 1 nit | Documentation drift (Finding 1) + `crdb:verify` should FAIL on bad schema jobs (Finding 2). |
| **Regression risk** | ✅ Low | Build tag `cockroach` isolates changes. No non-CRDB builds affected. |

---

## Recommended Gate 0 Sequence

```text
task crdb:reset          # wipe + start (--wait) + init (chained)
    ↓
task build:backend:cockroach
    ↓
task crdb:migrate        # boot app, applies LATEST.sql
    ↓
task crdb:verify         # P1-P6 + SHOW JOBS (with FAIL on failed jobs)
    ↓
verify-production.sh     # full data path
    ↓
restart app
    ↓
verify-production.sh     # idempotency proof
```

Concurrent startup test: **optional** — reindex is triggered by admin API, env var, or auto-bootstrap, not per-replica cron. The `IF NOT EXISTS` + SQLSTATE fallback is sufficient defense-in-depth for the hackathon scope.

---

## Required Changes Before Merge

| # | Finding | File | Fix |
|---|---------|------|-----|
| 1 | High — deploy runbook points to wrong task | `docs/docs_flyio_cockroach_deploy.md:28` | Change `crdb:init` → `crdb:cluster:bootstrap` |
| 2 | Medium — `crdb:verify` warns instead of failing on failed schema jobs | `Taskfile.yml:372-378` | Change `echo "WARN"` → `echo "FAIL"` and add `exit 1` |

---

## Final Verdict

**REQUEST CHANGES**

The implementation is well-engineered and reflects substantial adversarial review. The two findings above are **not architectural** — they are one-line fixes. Once applied, this is ready to merge.

Do not expand scope further. The concurrent-startup test, while valuable, is appropriately marked optional in `plan3.md`. The `simple_protocol` workaround is correctly retained. The `agent_vectors` runtime creation is correctly preserved.
