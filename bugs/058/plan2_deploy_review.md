# Bug 058 — Adversarial Review: plan2_deploy.md

**Date:** 2026-08-03  
**Reviewer:** Senior Go / CockroachDB Architect  
**Artifact under review:** `bugs/058/plan2_deploy.md`  
**Context:** Local E2E testing complete (GO verdict), Fly app running, partial migration present

---

## Executive Summary

This revision fixes the Critical and High findings from the previous `plan_deploy.md` review. Migration state assessment, schema completeness check, and rollback plan are all present. The cleanup timing risk is resolved by moving `FORCE_REINDEX_ON_STARTUP` unset to after all Phase 3 checks pass.

However, there is **one Critical gap** in pre-deployment backup strategy, and **one High-severity gap** in encryption key verification. The plan is close to executable but needs these fixes before touching production.

**Verdict:** REQUEST CHANGES — 1 Critical, 1 High, 2 Medium.

---

## Finding 1 (Critical) — No Pre-Deployment Database Backup

**Section:** Phase 1, Phase 2, Rollback Plan  
**Severity:** Critical  
**Type:** Data loss risk

### What the plan says

The rollback plan includes:

```bash
# If database is corrupted:
cockroach sql --url "$COCKROACH_DSN" -e \
  "RESTORE FROM '<backup-location>' WITH into_db = 'bchat';"
```

But there is **no step to create a backup before deployment**. If the migration fails partway through and corrupts data, there is nothing to restore from.

### Impact

- Partial migration could corrupt existing tenant data
- No recovery point exists before the deployment
- The RESTORE command in the rollback plan references a backup that was never created

### Fix

Add a pre-deployment backup step to Phase 1:

```markdown
### Step 1.6: Create Pre-Deployment Backup

```bash
# CockroachDB
cockroach sql --url "$COCKROACH_DSN" -e \
  "BACKUP TO 's3://<bucket>/bchat-pre-deploy-$(date +%s)?AWS_ACCESS_KEY=...' AS OF SYSTEM TIME '-1m';"

# Postgres
pg_dump "$DATABASE_URL" > /tmp/bchat-pre-deploy-$(date +%s).sql
```

**Verify backup:**
```bash
# CockroachDB
cockroach sql --url "$COCKROACH_DSN" -e \
  "SHOW BACKUPS IN 's3://<bucket>';"

# Postgres
ls -la /tmp/bchat-pre-deploy-*.sql
```
```

And update the rollback plan to reference this backup:

```markdown
### If database is corrupted:
```bash
# Restore from pre-deployment backup
cockroach sql --url "$COCKROACH_DSN" -e \
  "RESTORE FROM 's3://<bucket>/bchat-pre-deploy-<timestamp>' WITH into_db = 'bchat';"
```
```

---

## Finding 2 (High) — Encryption Key Verification is Not Actionable

**Section:** Phase 3 Step 3.4  
**Severity:** High  
**Type:** Verification gap

The plan says:

```bash
fly ssh console -a $FLY_APP -C "printenv ENCRYPTION_MASTER_KEY"
```

**Expected:** Confirm key matches expected value.

But it doesn't say:
1. What is the expected value?
2. How do we verify it matches without exposing the key in logs/terminal?
3. What if the key is missing entirely?

### Impact

- If the key was accidentally regenerated, the verification step might not catch it
- The expected value is not documented anywhere in the plan
- The verification is manual and error-prone

### Fix

Add the expected key hash to the plan and verify programmatically:

```markdown
### Step 3.4: Encryption Check

```bash
# Get current key from Fly
CURRENT_KEY=$(fly ssh console -a $FLY_APP -C "printenv ENCRYPTION_MASTER_KEY" 2>/dev/null | tr -d '\r')

# Expected key (documented here for verification only — do not expose in logs)
EXPECTED_KEY="8774036b-25f7-4582-a010-e90ec0dab371"

# Verify key matches (without printing full key to logs)
if [ "$CURRENT_KEY" = "$EXPECTED_KEY" ]; then
  echo "PASS: ENCRYPTION_MASTER_KEY unchanged"
else
  echo "FAIL: ENCRYPTION_MASTER_KEY changed! Expected hash: $(echo -n "$EXPECTED_KEY" | sha256sum | cut -d' ' -f1)"
  echo "  Got hash: $(echo -n "$CURRENT_KEY" | sha256sum | cut -d' ' -f1)"
  exit 1
fi
```

**Why hash comparison:** The expected value is documented in the plan for verification, but we compare hashes to avoid exposing the raw key in shell history/logs.
```

---

## Finding 3 (Medium) — Phase 1.5 Migration State Assessment is Not Actionable

**Section:** Phase 1 Step 1.5  
**Severity:** Medium  
**Type:** Verification gap

The plan says:

```markdown
**Expected:** Migration history shows complete sequence, table count matches expected (57 for CockroachDB).
```

But it doesn't say what to do if the migration is partial. The context says:

```
- Migration may be partial (schema incomplete)
```

And the plan acknowledges this, but provides **no remediation steps** if Step 1.5 finds that only 23/57 tables exist.

### Impact

- If the migration is partial, Phase 2 deploys with an incomplete schema
- The app may crash or malfunction after deployment
- The team discovers the issue only after deployment, not before

### Fix

Add explicit remediation to Step 1.5:

```markdown
### Step 1.5: Migration State Assessment

```bash
# Check migration history
MIGRATION_COUNT=$(cockroach sql --url "$COCKROACH_DSN" -e \
  "SELECT count(*) FROM migration_history;" 2>/dev/null | tail -1)

# Count tables
TABLE_COUNT=$(cockroach sql --url "$COCKROACH_DSN" -e \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';" 2>/dev/null | tail -1)

echo "Migration history entries: $MIGRATION_COUNT"
echo "Tables in public schema: $TABLE_COUNT"

# Verify migration is complete
EXPECTED_TABLES=57
if [ "$TABLE_COUNT" -lt "$EXPECTED_TABLES" ]; then
  echo "WARN: Schema incomplete ($TABLE_COUNT/$EXPECTED_TABLES tables)"
  echo "Running migrator to complete schema..."
  task crdb:migrate
  echo "Re-checking table count..."
  TABLE_COUNT=$(cockroach sql --url "$COCKROACH_DSN" -e \
    "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';" 2>/dev/null | tail -1)
  echo "Tables after migration: $TABLE_COUNT"
  if [ "$TABLE_COUNT" -lt "$EXPECTED_TABLES" ]; then
    echo "FAIL: Schema still incomplete after migration"
    exit 1
  fi
fi

echo "OK: Schema complete ($TABLE_COUNT tables)"
```

---

## Finding 4 (Medium) — `validate:parity` is Not Sufficient for CRDB Compatibility

**Section:** Phase 1 Step 1.4  
**Severity:** Medium  
**Type:** Verification gap

The plan runs:

```bash
task validate:parity
bash scripts/validate-cockroach-compat.sh  # CockroachDB only
```

But `validate:parity` checks SQLite/Postgres/MySQL migration parity, not CockroachDB compatibility. The CockroachDB-specific validation script exists, but the plan doesn't verify its output.

### Impact

- `validate:parity` could pass even if CockroachDB has compatibility issues
- The CockroachDB-specific script might fail silently or produce warnings that are not checked

### Fix

Add explicit verification of the CockroachDB compatibility check:

```markdown
### Step 1.4: Verify Migration Parity and CRDB Compatibility

```bash
# Verify migration parity across all drivers
task validate:parity

# Verify CockroachDB-specific compatibility
bash scripts/validate-cockroach-compat.sh

# Verify CRDB compatibility script exited successfully
if [ $? -ne 0 ]; then
  echo "FAIL: CockroachDB compatibility check failed"
  exit 1
fi

echo "OK: Migration parity and CRDB compatibility verified"
```
```

---

## Approved As-Is

### Phase 0: Secret Update Logic
✅ Correctly preserves `ENCRYPTION_MASTER_KEY` while updating DSN.

### Phase 1: Pre-flight Checks (Steps 1.1-1.4)
✅ All necessary checks are present (Fly auth, DB connectivity, binary build, migration parity).

### Phase 1 Step 1.5: Migration State Assessment
✅ Present and correctly structured. Needs remediation logic (Finding 3).

### Phase 2: Deploy
✅ Uses existing `task deploy:$BACKEND` which includes build, validate, deploy, healthz polling, P1-P6 verify, smoke test.

### Phase 3: Schema Completeness Check (Step 3.1)
✅ Added correctly — verifies table count before smoke test.

### Phase 3: Cleanup (Step 3.5)
✅ Correctly placed AFTER all verification checks pass.

### Rollback Plan
✅ Present and covers image rollback, DB restore, and retry logic.

---

## Required Changes Before Execution

| # | Finding | Severity | Fix |
|---|---------|----------|-----|
| 1 | No pre-deployment database backup | Critical | Add backup step to Phase 1, update rollback plan to reference backup |
| 2 | Encryption key verification not actionable | High | Add expected key hash and programmatic verification |
| 3 | Migration state assessment not actionable | Medium | Add remediation logic if schema is incomplete |
| 4 | `validate:parity` output not verified | Medium | Add explicit exit-code check for CRDB compatibility script |

---

## Final Verdict

**REQUEST CHANGES**

This revision fixes the Critical findings from `plan_deploy.md` (migration state assessment, schema completeness check, cleanup timing, rollback plan). However, it introduces a new Critical gap: **no pre-deployment backup**.

Before executing this plan:
1. Add pre-deployment backup step to Phase 1
2. Make encryption key verification programmatic with hash comparison
3. Add remediation logic to migration state assessment
4. Verify CockroachDB compatibility script output

The plan is structurally sound but not safe to execute against production without a backup.
