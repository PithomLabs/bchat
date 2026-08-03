# Plan: CockroachDB Deployment to Fly.io

**Date:** 2026-08-03
**Bug:** 058
**App:** `bchat-crdb`
**Status:** Ready to execute

---

## Context

- Local E2E testing complete (GO verdict)
- Fly app `bchat-crdb` running, CRDB cluster active
- Migration may be partial (schema incomplete)
- CockroachDB DSN password changed — need to update `COCKROACH_DSN` secret
- `ENCRYPTION_MASTER_KEY` must NOT be regenerated — it protects encrypted tenant API keys
- **Design decision:** Use `fly_cockroach.toml` for CockroachDB (contains hard-earned settings)

---

## Code Changes Required

### 1. Add backend-specific validation tasks to Taskfile.yml

```yaml
fly:check:sqlite:
  desc: Validate SQLite migrations before deployment
  cmds:
    - ./scripts/validate-db-migrations.sh

fly:check:postgres:
  desc: Validate Postgres migrations before deployment
  cmds:
    - ./scripts/validate-pg-migrations.sh

fly:check:cockroach:
  desc: Validate CockroachDB migrations before deployment
  cmds:
    - ./scripts/validate-cockroach-compat.sh
```

### 2. Update docs_deployment_guide.md

Update CockroachDB section to use `fly:check:cockroach` and `fly_cockroach.toml`.

---

## Deployment Phases

### Phase 1: Pre-flight

| Step | Command | Expected |
|------|---------|----------|
| 1.1 | `fly auth whoami` | Shows authenticated user |
| 1.2 | `fly status --app bchat-crdb` | App exists |
| 1.3 | Verify `fly_cockroach.toml` exists | File present |
| 1.4 | `cockroach sql --url "$COCKROACH_DSN" -e "SELECT 1;"` | CRDB accessible |

### Phase 2: Update Secrets (COCKROACH_DSN Only)

```bash
COCKROACH_DSN="postgresql://bchat_user:<new-password>@great-goat-30894.j77.cockroachlabs.cloud:26257/bchat?sslmode=require" \
  bash scripts/fly-cockroach-secrets.sh
```

| Secret | Action |
|--------|--------|
| `COCKROACH_DSN` | Updates (from env var) |
| `OPENROUTER_API_KEY` | Skips (already on Fly) |
| `ENCRYPTION_MASTER_KEY` | Skips (already on Fly) |

### Phase 3: Validation

```bash
task fly:pre-deploy:cockroach
```

This runs: `fly:check` (with `fly_cockroach.toml`) → `fly:check:cockroach` → success message

### Phase 4: Deploy

```bash
task deploy:cockroach
```

**Chain (crdb-deploy.sh):**
1. `build:backend:cockroach` → Binary with `-tags cockroach`
2. `validate-parity.sh` → Migration parity
3. `validate-cockroach-compat.sh` → CRDB compatibility
4. `fly deploy -c fly_cockroach.toml --wait-timeout 45m`
5. Healthz poll (50m timeout)
6. `crdb:verify` → P1-P6 checks
7. `verify:production` → Smoke test

### Phase 5: Verify

| Step | Check |
|------|-------|
| 5.1 | Schema: 57 tables |
| 5.2 | Vector index: `idx_agent_vectors_embedding` |
| 5.3 | Encryption key: unchanged (8774036b-25f7-4582-a010-e90ec0dab371) |
| 5.4 | Health: 200 OK |

---

## Adversarial Review Prompt

| # | Question | Risk |
|---|----------|------|
| 1 | Does `fly_cockroach.toml` exist and is correct? | Wrong config |
| 2 | Is `COCKROACH_DSN` updated with new password? | Connection failure |
| 3 | Is `ENCRYPTION_MASTER_KEY` preserved? | Tenant data loss |
| 4 | Does `task fly:check:cockroach` pass? | Migration failure |
| 5 | Does `task deploy:cockroach` complete? | Deploy failure |
| 6 | Is schema complete (57 tables)? | Partial deploy |
| 7 | Is vector index present? | RAG failure |
| 8 | Is health endpoint 200? | App down |

---

## Execution Order

```
Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5
   ↓         ↓         ↓         ↓         ↓
Pre-flight  Secrets  Validate   Deploy   Verify
```

Total estimated time: 60-90 minutes (mostly deploy + migration).
