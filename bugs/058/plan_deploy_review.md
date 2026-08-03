# Bug 058 — Adversarial Review: plan_deploy.md

**Date:** 2026-08-03  
**Reviewer:** Senior Go / CockroachDB Architect  
**Artifact under review:** `bugs/058/plan_deploy.md`

---

## Executive Summary

The deployment plan is **structurally sound** and correctly prioritizes preserving `ENCRYPTION_MASTER_KEY`. However, it has **one Critical gap** in partial-migration recovery, **one High-severity gap** in rollback planning, and **several Medium nits** that should be addressed before execution.

**Verdict:** REQUEST CHANGES — 1 Critical, 1 High, 3 Medium.

---

## Finding 1 (Critical) — No Recovery Plan for Partial Migration

**Section:** Context  
**Severity:** Critical  
**Type:** Execution blocker

The plan acknowledges the partial migration:

```
- Migration shows 0.35.1 but only 23/57 tables exist (partial migration)
```

But it provides **no recovery steps**. Phase 2 runs `task deploy:cockroach` and hopes the remaining 34 tables apply. If the migration fails partway through, the database is left in a worse state with no documented recovery.

### Impact
- Database left with mixed old/new schema
- `migration_history` could be inconsistent
- App crashes on startup with schema validation errors
- No documented rollback procedure

### Fix

Add a pre-deployment migration assessment step and a rollback section:

```markdown
### Phase 1.5: Migration State Assessment

```bash
# Check current migration state
COCKROACH_DSN="<dsn>" cockroach sql --url "$COCKROACH_DSN" -e \
  "SELECT version, description, created_at FROM migration_history ORDER BY version;"

# Count existing tables
cockroach sql --url "$COCKROACH_DSN" -e \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';"
```

**Expected:** Migration history shows complete sequence, table count matches expected.

## Rollback Plan

If deployment fails:
1. `fly secrets unset MEMOS_DRIVER --app bchat-crdb` (revert to previous driver if needed)
2. `fly deploy --image <previous-image-tag> --app bchat-crdb`
3. Verify previous version is healthy
4. If DB is corrupted, restore from backup:
   ```bash
   cockroach sql --url "$COCKROACH_DSN" -e \
     "RESTORE FROM '<backup-location>' WITH into_db = 'bchat';"
   ```
```

---

## Finding 2 (High) — No Rollback Plan

**Section:** Phase 2, Phase 3  
**Severity:** High  
**Type:** Operational risk

If the deployment fails after Phase 2, there is no documented recovery procedure. The team must improvise under pressure.

### Fix

Add explicit rollback instructions (see Finding 1 for full text). At minimum:

```markdown
## Rollback Plan

### If deployment fails during migration:
1. Do NOT unset `FORCE_REINDEX_ON_STARTUP` — keep it for retry
2. Deploy previous known-good image:
   ```bash
   fly deploy --image <previous-image-tag> --app bchat-crdb
   ```
3. Verify previous version is healthy
4. Investigate logs before retrying

### If app crashes after successful migration:
1. Roll back to previous Docker image (same command as above)
2. Verify previous version is healthy
3. Fix issue, then re-deploy
```

---

## Finding 3 (Medium) — Phase 4 Cleanup Timing Risk

**Section:** Phase 4  
**Severity:** Medium  
**Type:** Operational risk

The plan unsets `FORCE_REINDEX_ON_STARTUP` in Phase 4:

```bash
fly secrets unset FORCE_REINDEX_ON_STARTUP --app bchat-crdb
```

But this happens **after** Phase 3 verification. If Phase 3 fails and the team needs to re-deploy, they must remember to re-set `FORCE_REINDEX_ON_STARTUP`.

### Fix

Move the cleanup to the **end** of Phase 3, after all verification passes:

```markdown
### Phase 3: Verify

| Check | Command |
|-------|---------|
| P1-P6 | `task crdb:verify` |
| Smoke test | `task verify:production` |
| Encryption unchanged | `fly ssh console -a bchat-crdb -C "printenv ENCRYPTION_MASTER_KEY"` → confirm `8774036b-25f7-4582-a010-e90ec0dab371` |
| Reindex complete | `fly ssh console -a bchat-crdb -C "ls -la /data/agent_vectors"` → confirm files exist |

**Only after all checks pass:**
```bash
fly secrets unset FORCE_REINDEX_ON_STARTUP --app bchat-crdb
```
```

---

## Finding 4 (Medium) — Smoke Test May Not Catch Partial Migration

**Section:** Phase 3  
**Severity:** Medium  
**Type:** Verification gap

`task verify:production` exercises the full data path, but if the migration is partial (some tables missing), the app might:
- Crash on startup (missing table)
- Return 500 on sign-in (missing column)
- Fail silently on tenant onboarding (missing constraint)

The smoke test catches this, but only **after** the app is deployed and potentially serving traffic.

### Fix

Add an explicit schema completeness check before the smoke test:

```markdown
### Phase 3: Verify

```bash
# Check schema completeness before smoke test
COCKROACH_DSN="<dsn>" cockroach sql --url "$COCKROACH_DSN" -e \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';"
```

**Expected:** 57 tables (or whatever `LATEST.sql` defines)
```

---

## Finding 5 (Medium) — Phase 1 `validate:parity` Doesn't Check CRDB Schema

**Section:** Phase 1  
**Severity:** Medium  
**Type:** Verification gap

`task validate:parity` validates that SQLite and Postgres migrations are in sync, but it does **not** verify that the CockroachDB database has all required tables. The partial migration (23/57 tables) would not be caught by this check.

### Fix

Add a CockroachDB-specific schema count check to Phase 1:

```markdown
| CRDB schema completeness | `cockroach sql --url "$COCKROACH_DSN" -e "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';"` | Should match expected table count from LATEST.sql |
```

---

## Approved As-Is

### Phase 0: Secret Update Logic
✅ Correct priority: `COCKROACH_DSN` updated, `ENCRYPTION_MASTER_KEY` preserved.

### Phase 1: Pre-flight Checks
✅ All necessary checks are present (Fly auth, CRDB connectivity, binary build, parity, CRDB compat).

### Phase 2: Deploy
✅ Uses existing `task deploy:cockroach` which includes build, validate, deploy, healthz polling, P1-P6 verify, smoke test.

### Phase 3: Encryption Check
✅ Explicitly verifies `ENCRYPTION_MASTER_KEY` is unchanged — critical for tenant API key decryption.

---

## Required Changes Before Execution

| # | Finding | Severity | Fix |
|---|---------|----------|-----|
| 1 | No recovery plan for partial migration | Critical | Add migration state assessment + rollback procedure |
| 2 | No rollback plan | High | Add image rollback and DB restore steps |
| 3 | Phase 4 cleanup timing risk | Medium | Move `FORCE_REINDEX_ON_STARTUP` unset to end of Phase 3 |
| 4 | Smoke test may not catch partial migration | Medium | Add explicit schema completeness check before smoke test |
| 5 | `validate:parity` doesn't check CRDB schema | Medium | Add CRDB table count check to Phase 1 |

---

## Final Verdict

**REQUEST CHANGES**

The deployment plan is correct in its structure and priorities, but it is **not ready to execute** because it acknowledges a partial migration without providing recovery steps.

**Minimum viable fixes before execution:**
1. Add migration state assessment to Phase 1
2. Add rollback procedure (image rollback + DB restore)
3. Move `FORCE_REINDEX_ON_STARTUP` cleanup to after Phase 3 verification passes
4. Add schema completeness check before smoke test
5. Add CRDB-specific table count check to Phase 1 pre-flight

Once these are added, the plan is ready to execute.
