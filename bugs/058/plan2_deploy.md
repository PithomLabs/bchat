# Deployment Plan: Fly.io (Postgres or CockroachDB)

**Date:** 2026-08-03
**Bug:** 058
**Status:** Ready to execute
**Scope:** Postgres or CockroachDB — excludes SQLite (local-only)

---

## Parameters

| Variable | Postgres | CockroachDB | Description |
|----------|----------|-------------|-------------|
| `$FLY_APP` | `bchat-pg` | `bchat-crdb` | Fly.io app name |
| `$DEPLOY_CONFIG` | `fly_pg.toml` | `fly_cockroach.toml` | Fly deploy config |
| `$BUILD_TAG` | (none) | `cockroach` | Go build tag |
| `$SECRETS_SCRIPT` | `fly-pg-secrets.sh` | `fly-cockroach-secrets.sh` | Secrets setup script |
| `$DSN_ENV_VAR` | `DATABASE_URL` | `COCKROACH_DSN` | DSN environment variable |
| `$BACKEND` | `postgres` | `cockroach` | MEMOS_DRIVER value |

---

## Context

- Local E2E testing complete (GO verdict)
- Fly app running, database cluster active
- Migration may be partial (schema incomplete)
- Database password changed — need to update DSN secret
- `ENCRYPTION_MASTER_KEY` must NOT be regenerated — it protects encrypted tenant API keys

---

## Phase 0: Update Secrets (DSN Only)

### Step 0.1: Modify Secrets Script

Add environment variable checks to `scripts/$SECRETS_SCRIPT`:

| Secret | Behavior |
|--------|----------|
| `$DSN_ENV_VAR` | If env var set → use it; else → prompt |
| `OPENROUTER_API_KEY` | If env var set → use it; else → prompt |
| `ENCRYPTION_MASTER_KEY` | If env var set → use it; **else if already on Fly → skip**; else → generate new |

### Step 0.2: Run Script

```bash
# Postgres
DATABASE_URL="postgresql://user:password@host:5432/bchat?sslmode=require" \
  bash scripts/fly-pg-secrets.sh

# CockroachDB
COCKROACH_DSN="postgresql://user:password@host:26257/bchat?sslmode=require" \
  bash scripts/fly-cockroach-secrets.sh
```

**Result:** Only DSN updated. `ENCRYPTION_MASTER_KEY` preserved.

---

## Phase 1: Pre-flight Checks

### Step 1.1: Verify Fly Auth
```bash
fly auth whoami
fly status --app $FLY_APP
```

### Step 1.2: Verify Database Connectivity
```bash
# Postgres
psql "$DATABASE_URL" -c "SELECT 1;"

# CockroachDB
cockroach sql --url "$COCKROACH_DSN" -e "SELECT 1;"
```

### Step 1.3: Verify Binary Builds
```bash
task build:backend:$BACKEND
```

### Step 1.4: Verify Migration Parity
```bash
task validate:parity
bash scripts/validate-cockroach-compat.sh  # CockroachDB only
```

### Step 1.5: Migration State Assessment

```bash
# Check current migration state
# Postgres
psql "$DATABASE_URL" -c "SELECT version, description, created_at FROM migration_history ORDER BY version;"

# CockroachDB
cockroach sql --url "$COCKROACH_DSN" -e \
  "SELECT version, description, created_at FROM migration_history ORDER BY version;"

# Count existing tables
# Postgres
psql "$DATABASE_URL" -c "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';"

# CockroachDB
cockroach sql --url "$COCKROACH_DSN" -e \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';"
```

**Expected:** Migration history shows complete sequence, table count matches expected (57 for CockroachDB).

---

## Phase 2: Deploy

```bash
task deploy:$BACKEND
```

Runs deploy script → build → validate → fly deploy (45m timeout) → healthz poll → P1-P6 verify → smoke test.

---

## Phase 3: Verify

### Step 3.1: Schema Completeness Check

```bash
# Postgres
psql "$DATABASE_URL" -c "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';"

# CockroachDB
cockroach sql --url "$COCKROACH_DSN" -e \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';"
```

**Expected:** 57 tables (or whatever `LATEST.sql` defines).

### Step 3.2: P1-P6 Checks
```bash
task crdb:verify  # CockroachDB only
```

### Step 3.3: Smoke Test
```bash
task verify:production
```

### Step 3.4: Encryption Check
```bash
fly ssh console -a $FLY_APP -C "printenv ENCRYPTION_MASTER_KEY"
```

Confirm key matches expected value.

### Step 3.5: Cleanup (Only After All Checks Pass)

```bash
fly secrets unset FORCE_REINDEX_ON_STARTUP --app $FLY_APP
```

---

## Rollback Plan

### If deployment fails during migration:
1. Do NOT unset `FORCE_REINDEX_ON_STARTUP` — keep it for retry
2. Deploy previous known-good image:
   ```bash
   fly deploy --image <previous-image-tag> --app $FLY_APP
   ```
3. Verify previous version is healthy
4. Investigate logs before retrying

### If app crashes after successful migration:
1. Roll back to previous Docker image (same command as above)
2. Verify previous version is healthy
3. Fix issue, then re-deploy

### If database is corrupted:
```bash
# Postgres
psql "$DATABASE_URL" -c "RESTORE FROM '<backup-location>' WITH into_db = 'bchat';"

# CockroachDB
cockroach sql --url "$COCKROACH_DSN" -e \
  "RESTORE FROM '<backup-location>' WITH into_db = 'bchat';"
```

---

## Adversarial Review Checklist

| # | Question | Risk |
|---|----------|------|
| 1 | Is new DSN password in git history or evidence files? | Exposure |
| 2 | Did we accidentally regenerate `ENCRYPTION_MASTER_KEY`? | Data loss |
| 3 | Will idempotent migration complete missing tables? | Partial deploy |
| 4 | Does `CREATE VECTOR INDEX IF NOT EXISTS` work on existing data? | RAG failure |
| 5 | Can we rollback if something goes wrong? | Recovery |
| 6 | Does skip logic handle all 3 states (env set / already on Fly / new)? | Script bug |
| 7 | Are there encrypted tenant API keys that would become undecryptable? | Data loss |
| 8 | Is 45m deploy timeout sufficient for migration + startup? | Timeout |
| 9 | Does schema completeness check pass before smoke test? | Partial migration |
| 10 | Is `FORCE_REINDEX_ON_STARTUP` unset only after verification passes? | Re-index on retry |

---

## Execution Order

```
Phase 0 → Phase 1 → Phase 2 → Phase 3
   ↓         ↓         ↓         ↓
 Secrets   Pre-flight  Deploy   Verify + Cleanup
```

Total estimated time: 60-90 minutes (mostly deploy + migration).
