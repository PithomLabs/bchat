# Deployment Plan: CockroachDB on Fly.io

**Date:** 2026-08-02
**Bug:** 058
**Status:** Ready to execute

---

## Context

- Local E2E testing complete (GO verdict)
- Fly app `bchat-crdb` running, CRDB cluster active
- Migration shows 0.35.1 but only 23/57 tables exist (partial migration)
- CockroachDB DSN password changed — need to update `COCKROACH_DSN` secret
- `ENCRYPTION_MASTER_KEY` must NOT be regenerated — it protects encrypted tenant API keys

---

## Phase 0: Update Secrets (COCKROACH_DSN Only)

### Step 0.1: Modify Secrets Script

Add environment variable checks to `scripts/fly-cockroach-secrets.sh`:

| Secret | Behavior |
|--------|----------|
| `COCKROACH_DSN` | If env var set → use it; else → prompt |
| `OPENROUTER_API_KEY` | If env var set → use it; else → prompt |
| `ENCRYPTION_MASTER_KEY` | If env var set → use it; **else if already on Fly → skip**; else → generate new |

### Step 0.2: Run Script

```bash
COCKROACH_DSN="postgresql://bchat_user:<new-password>@great-goat-30894.j77.cockroachlabs.cloud:26257/bchat?sslmode=require" \
  bash scripts/fly-cockroach-secrets.sh
```

**Result:** Only `COCKROACH_DSN` updated. `ENCRYPTION_MASTER_KEY` stays `8774036b-25f7-4582-a010-e90ec0dab371`.

---

## Phase 1: Pre-flight Checks

| Check | Command |
|-------|---------|
| Fly auth | `fly auth whoami` |
| CRDB connectivity | `cockroach sql --url "$COCKROACH_DSN" -e "SELECT 1;"` |
| Binary builds | `task build:backend:cockroach` |
| Migration parity | `task validate:parity` |
| CRDB compat | `bash scripts/validate-cockroach-compat.sh` |

---

## Phase 2: Deploy

```bash
task deploy:cockroach
```

Runs `scripts/crdb-deploy.sh` → build → validate → fly deploy (45m timeout) → healthz poll → P1-P6 verify → smoke test.

---

## Phase 3: Verify

| Check | Command |
|-------|---------|
| P1-P6 | `task crdb:verify` |
| Smoke test | `task verify:production` |
| Encryption unchanged | `fly ssh console -a bchat-crdb -C "printenv ENCRYPTION_MASTER_KEY"` → confirm `8774036b-25f7-4582-a010-e90ec0dab371` |

---

## Phase 4: Cleanup

```bash
fly secrets unset FORCE_REINDEX_ON_STARTUP --app bchat-crdb
```

---

## Adversarial Review Checklist

| # | Question | Risk |
|---|----------|------|
| 1 | Is new COCKROACH_DSN password in git history or evidence files? | Exposure |
| 2 | Did we accidentally regenerate `ENCRYPTION_MASTER_KEY`? | Data loss |
| 3 | Will idempotent migration complete missing 34 tables? | Partial deploy |
| 4 | Does `CREATE VECTOR INDEX IF NOT EXISTS` work on existing data? | RAG failure |
| 5 | Can we rollback if something goes wrong? | Recovery |
| 6 | Does skip logic handle all 3 states (env set / already on Fly / new)? | Script bug |
| 7 | Are there encrypted tenant API keys that would become undecryptable? | Data loss |
| 8 | Is 45m deploy timeout sufficient for migration + startup? | Timeout |

---

## Execution Order

```
Phase 0 → Phase 1 → Phase 2 → Phase 3 → Phase 4
   ↓         ↓         ↓         ↓         ↓
 Secrets   Pre-flight  Deploy   Verify   Cleanup
```

Total estimated time: 60-90 minutes (mostly deploy + migration).
