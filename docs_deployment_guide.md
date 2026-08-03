# bchat Deployment Guide (Fly.io)

**Date:** 2026-08-03
**Version:** 1.0
**Status:** Authoritative — supersedes all other deployment documentation

---

## Superseded Documentation

The following files are superseded by this guide:

| File | Reason |
|------|--------|
| `docs_deployment.md` | General deployment now covered in this guide |
| `docs_flyio_neon_deploy.md` | Postgres deployment now covered in Section 6 |
| `docs_flyio_cockroach_deploy.md` | CockroachDB deployment now covered in Section 6 |
| `docs/DOCS_DATABASE_MIGRATION_GUIDE.md` | Migration validation now covered in Section 7 |
| `docs/docs_fly_app_config.md` | Fly app configuration now covered in Section 5 |
| `docs/docs_fly_deploy.md` | Fly deployment process now covered in Section 8 |
| `bugs/057/plan_deploy.md` | CRDB deployment now covered in this guide |
| `bugs/058/plan_deploy.md` | E2E testing + deployment now covered in this guide |
| `bugs/058/plan2_deploy.md` | General deployment now covered in this guide |
| `bugs/058/plan3_deploy.md` | Final deployment plan now covered in this guide |

---

## Overview

This guide covers deploying bchat to Fly.io with one of three backends:

| Backend | Dockerfile | Build Tag | CGO | LanceDB |
|---------|------------|-----------|-----|---------|
| SQLite | `Dockerfile.fly` | `rag` | Enabled | Included |
| Postgres | `Dockerfile.pg.fly` | `rag` | Enabled | Included |
| CockroachDB | `Dockerfile.cockroach.fly` | `cockroach` | Disabled | Excluded |

**Key Design Decision:** Each backend has its own TOML file and Fly app. No manual editing required.

---

## Prerequisites

| Requirement | SQLite | Postgres | CockroachDB |
|-------------|--------|----------|-------------|
| Fly CLI installed | ✅ | ✅ | ✅ |
| Fly CLI authenticated | ✅ | ✅ | ✅ |
| Fly app created | ✅ | ✅ | ✅ |
| Database provisioned | ❌ (embedded) | ✅ (Neon/Supabase) | ✅ (CockroachDB Cloud) |
| OpenRouter API key | ✅ | ✅ | ✅ |
| ENCRYPTION_MASTER_KEY | Optional | Optional | Optional |

---

## Architecture

### Deployment Flow

```
fly.toml (you edit dockerfile pointer)
    ↓
fly deploy -c fly.toml
    ↓
Dockerfile (determines what goes into image)
    ↓
Docker image (binary + frontend + widget)
    ↓
Fly.io machine (runs app)
```

### What Goes Into the Docker Image

| Component | Included | Notes |
|-----------|----------|-------|
| Frontend (React) | ✅ | Built in Stage 1 |
| Widget (embeddable) | ✅ | Built in Stage 1 |
| Backend binary | ✅ | Built in Stage 2 |
| LanceDB libraries | ⚠️ | Only with `rag` build tag |
| SQLite | ❌ | Not in Docker image |
| Postgres driver | ✅ | pgx/v5 (built into binary) |
| CockroachDB driver | ✅ | pgx/v5 (same as Postgres) |

### Build Tag Behavior

```
-rag    → vectordb_lance.go (LanceDB native) + CGO_ENABLED=1
+rag    → vectordb_lance.go (LanceDB native) + CGO_ENABLED=1
+cockroach → vectordb_cockroach.go (CRDB vectors) + no CGO
```

---

## Configuration

### Backend-Specific TOML Files

Each backend has its own TOML file with hard-earned settings:

| Backend | TOML File | Fly App | Dockerfile |
|---------|-----------|---------|------------|
| Postgres | `fly_pg.toml` | `bchat-pg` | `Dockerfile.pg.fly` |
| CockroachDB | `fly_cockroach.toml` | `bchat-crdb` | `Dockerfile.cockroach.fly` |

**Postgres:** `fly_pg.toml` is the canonical source. `fly.toml` is a legacy alias and should not be used for new deployments.

### CockroachDB Configuration (`fly_cockroach.toml`)

```toml
app = 'bchat-crdb'
primary_region = 'sjc'

[build]
  dockerfile = 'Dockerfile.cockroach.fly'

[env]
  MEMOS_DRIVER = 'cockroach'
  MEMOS_MODE = 'prod'
  MEMOS_PORT = '5230'
  RAG_PIPELINE_ENABLED = 'true'
  EMBEDDING_PROVIDER = 'openrouter'
  EMBEDDING_MODEL = 'openai/text-embedding-3-small'
  LANCEDB_STORAGE_PROVIDER = 'cockroach'
  # ... other env vars ...

[http_service]
  auto_stop_machines = 'off'  # Prevents machine death during migration
  # ... other settings ...

  [[http_service.checks]]
    grace_period = "60m"  # Long grace for ~25-60 min migration
```

### Secrets (set via fly secrets, NOT in TOML files)

```bash
# CockroachDB
fly secrets set \
  COCKROACH_DSN="postgresql://user:pass@host:26257/bchat?sslmode=require" \
  OPENROUTER_API_KEY="sk-or-v1-..." \
  ENCRYPTION_MASTER_KEY="$(uuidgen)" \
  --app bchat-crdb
```

### Backend-Specific Environment Variables

| Variable | SQLite | Postgres | CockroachDB |
|----------|--------|----------|-------------|
| `MEMOS_DRIVER` | `sqlite` | `postgres` | `cockroach` |
| `MEMOS_DSN` | N/A | ✅ | ❌ |
| `DATABASE_URL` | N/A | ✅ | ❌ |
| `COCKROACH_DSN` | N/A | ❌ | ✅ |
| `LANCEDB_STORAGE_PROVIDER` | `local` | `s3` | `cockroach` |

---

## Secrets Management

### Option A: Automated Script

```bash
# Postgres
DATABASE_URL="postgresql://user:pass@host:5432/bchat?sslmode=require" \
  OPENROUTER_API_KEY="sk-or-v1-..." \
  bash scripts/fly-pg-secrets.sh

# CockroachDB
COCKROACH_DSN="postgresql://user:pass@host:26257/bchat?sslmode=require" \
  OPENROUTER_API_KEY="sk-or-v1-..." \
  bash scripts/fly-cockroach-secrets.sh
```

**Skip Logic:**
- If env var is set → use it
- If already on Fly → skip (don't overwrite)
- If neither → prompt or generate

### Option B: Manual Commands

```bash
# SQLite (minimal secrets)
fly secrets set ENCRYPTION_MASTER_KEY="$(uuidgen)" --app bchat-app

# Postgres
fly secrets set \
  DATABASE_URL="postgresql://user:pass@host:5432/bchat?sslmode=require" \
  OPENROUTER_API_KEY="sk-or-v1-..." \
  ENCRYPTION_MASTER_KEY="$(uuidgen)" \
  --app bchat-app

# CockroachDB
fly secrets set \
  COCKROACH_DSN="postgresql://user:pass@host:26257/bchat?sslmode=require" \
  OPENROUTER_API_KEY="sk-or-v1-..." \
  ENCRYPTION_MASTER_KEY="$(uuidgen)" \
  --app bchat-app
```

### ENCRYPTION_MASTER_KEY Rules

| Rule | Details |
|------|---------|
| Auto-generate | Only when no key exists on Fly |
| Never overwrite | If key already exists on Fly, skip it |
| Preserve on deploy | `fly-cockroach-secrets.sh` detects existing key |
| Rotation | Requires `ENCRYPTION_MASTER_KEY_BACKUP` for old key |

---

## Pre-deployment Validation

### Backend-Specific Validation Tasks

| Backend | Task | What It Checks |
|---------|------|----------------|
| SQLite | `task fly:check:sqlite` | SQLite migrations |
| Postgres | `task fly:check:postgres` | Postgres migrations |
| CockroachDB | `task fly:check:cockroach` | CockroachDB compatibility |

### Run All Pre-deployment Checks

```bash
# CockroachDB
task fly:pre-deploy:cockroach

# Postgres
task fly:pre-deploy:postgres

# SQLite
task fly:pre-deploy:sqlite
```

Each runs: `fly:check` → `fly:check:<backend>` → success message

---

## Deployment Steps

### Step 1: Set Secrets

```bash
# CockroachDB
COCKROACH_DSN="postgresql://user:pass@host:26257/bchat?sslmode=require" \
  OPENROUTER_API_KEY="sk-or-v1-..." \
  bash scripts/fly-cockroach-secrets.sh

# Postgres
DATABASE_URL="postgresql://user:pass@host:5432/bchat?sslmode=require" \
  OPENROUTER_API_KEY="sk-or-v1-..." \
  bash scripts/fly-pg-secrets.sh
```

### Step 2: Run Pre-deployment Checks

```bash
# CockroachDB
task fly:pre-deploy:cockroach

# Postgres
task fly:pre-deploy:postgres
```

### Step 3: Deploy

```bash
# CockroachDB
task deploy:cockroach

# Postgres
task deploy:postgres

# SQLite
fly deploy -c fly.toml
```

### Step 4: Verify Deployment

```bash
# Health check
curl https://bchat-crdb.fly.dev/healthz

# P1-P6 checks (CockroachDB only)
task crdb:verify

# Smoke test
task verify:production
```

---

## Post-deployment Verification

### Schema Completeness Check

```bash
# SQLite
sqlite3 build/data/memos.db ".tables" | wc -w

# Postgres
psql "$DATABASE_URL" -c "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';"

# CockroachDB
cockroach sql --url "$COCKROACH_DSN" -e \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';"
```

**Expected:** 57 tables (or whatever `LATEST.sql` defines).

### Encryption Key Verification

```bash
# Get current key from Fly
CURRENT_KEY=$(fly ssh console -a bchat-app -C "printenv ENCRYPTION_MASTER_KEY" 2>/dev/null | tr -d '\r')

# Verify key exists (don't expose in logs)
if [ -n "$CURRENT_KEY" ]; then
  echo "PASS: ENCRYPTION_MASTER_KEY is set"
else
  echo "FAIL: ENCRYPTION_MASTER_KEY is missing"
  exit 1
fi
```

### Vector Index Check (CockroachDB)

```bash
cockroach sql --url "$COCKROACH_DSN" -e \
  "SHOW INDEX FROM agent_vectors;" | grep -q "idx_agent_vectors_embedding" && \
  echo "PASS: Vector index exists" || \
  echo "FAIL: Vector index missing"
```

---

## Adversarial Review Prompt

Before executing this deployment, review the following:

| # | Question | Risk |
|---|----------|------|
| 1 | Is the correct TOML file being used for your backend? | Wrong config |
| 2 | Are all secrets set (DSN, OPENROUTER_API_KEY, ENCRYPTION_MASTER_KEY)? | App failure |
| 3 | Is ENCRYPTION_MASTER_KEY preserved (not regenerated)? | Data loss |
| 4 | Does `fly:pre-deploy:<backend>` pass all checks? | Migration failure |
| 5 | Is the schema complete (57 tables)? | Partial deploy |
| 6 | Is the vector index present (CockroachDB)? | RAG failure |
| 7 | Is the encryption key unchanged? | Tenant data loss |
| 8 | Is the health endpoint returning 200? | App down |

---

## Reference

### Files Modified

| File | Changes |
|------|---------|
| `Taskfile.yml` | Added `fly:check:sqlite`, `fly:check:postgres`, `fly:check:cockroach`, `fly:pre-deploy:cockroach`, `fly:pre-deploy:postgres`, `fly:pre-deploy:sqlite` |
| `scripts/fly-cockroach-secrets.sh` | Skip logic for existing keys |
| `scripts/fly-pg-secrets.sh` | Skip logic for existing keys |

### Files Created

| File | Purpose |
|------|---------|
| `docs_deployment_guide.md` | This guide |

### Dockerfile Quick Reference

| Backend | Dockerfile | Build Tag | CGO | LanceDB |
|---------|------------|-----------|-----|---------|
| Postgres | `Dockerfile.pg.fly` | `rag` | Enabled | Included |
| CockroachDB | `Dockerfile.cockroach.fly` | `cockroach` | Disabled | Excluded |

---

## Reviewer Decisions

| Decision | Rationale |
|----------|-----------|
| SQLite removed from guide table | SQLite doesn't deploy to Fly.io easily (no volume support), removed from deployment documentation |
| `fly.toml` kept as legacy alias | Kept for backward compatibility, already documented as legacy |
| `fly:db-check` kept as-is | Not recommended for CRDB, low priority to remove |
