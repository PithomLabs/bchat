# Deploy to Fly.io with Neon Postgres

**Date:** 2026-07-08
**Status:** Active
**Companion to:** [`docs_deployment.md`](docs_deployment.md), [`docs_testing_pg.md`](docs_testing_pg.md)

---

## Overview

This guide covers deploying bchat to Fly.io using Neon serverless Postgres as the database backend. The Postgres driver is fully implemented, the Dockerfile exists, and the Taskfile is ready.

| Component | Status | File |
|-----------|--------|------|
| Postgres driver | ✅ Complete | `store/db/postgres/` (24 files) |
| Dockerfile | ✅ Exists | `Dockerfile.pg.fly` |
| Fly config | ✅ Exists | `fly_pg.toml` |
| Taskfile | ✅ Fixed | `Taskfile_pg.yml` |
| Migration validation | ✅ Fixed | `scripts/validate-pg-migrations.sh` |

---

## Prerequisites

| Requirement | Check command |
|-------------|--------------|
| `fly` CLI installed | `fly --version` |
| Logged into Fly.io | `fly auth whoami` |
| Fly app created | `fly status --app bchat-pg` |
| Neon account | https://console.neon.tech |
| `OPENROUTER_API_KEY` | Already set as Fly secret on existing app |

---

## Default Credentials (Local Postgres)

Used for migration validation only, not for production.

| Field | Value |
|-------|-------|
| Host | `localhost` |
| Port | `5432` |
| Database | `bchat` |
| User | `bchat` |
| Password | `bchat` |
| Full URL | `postgresql://bchat:bchat@localhost:5432/bchat` |

---

## Step 1: Validate Migrations Locally

```bash
# Start local Postgres container
task -t Taskfile_pg.yml postgres:start

# Validate LATEST.sql + all migrations
task -t Taskfile_pg.yml validate:migrations
```

All 4 steps must pass before deploying. See [`docs_testing_pg.md`](docs_testing_pg.md) for details.

---

## Step 2: Create Neon Database

1. Go to https://console.neon.tech
2. Create a new project (region: closest to your Fly.io region)
3. Copy the connection string — it looks like:
   ```
   postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require&channel_binding=require
   ```
4. **Keep `sslmode=require`** — Neon requires it

---

## Step 3: Set Fly Secrets

**Recommended:** Use the automated script to set all secrets in one session:

```bash
./scripts/fly-pg-secrets.sh
```

This will prompt for your Neon connection string, OpenRouter API key, and create Tigrisdata S3 storage automatically.

**Manual alternative** — set secrets individually:

```bash
# Database connection string (Neon)
fly -a bchat-pg secrets set DATABASE_URL="postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require&channel_binding=require"

# LLM API key
fly -a bchat-pg secrets set OPENROUTER_API_KEY="sk-or-v1-xxx"

# Encryption key for tenant API keys (auto-generated)
fly -a bchat-pg secrets set ENCRYPTION_MASTER_KEY="$(uuidgen)"

# LanceDB S3 storage (Tigrisdata)
fly -a bchat-pg secrets set LANCEDB_S3_BUCKET="your-bucket"
```

**Note:** `DATABASE_URL` is read via `os.Getenv()` in `internal/profile/profile.go`, not via viper. It works as a Fly secret without any additional config.

---

## Step 4: Deploy

```bash
fly -a bchat-pg deploy -c fly_pg.toml
```

This builds the Docker image (`Dockerfile.pg.fly`) and deploys to Fly.io.

---

## Step 5: Verify

```bash
# Check logs for DSN output and errors
fly -a bchat-pg logs

# Test health endpoint
curl https://bchat-pg.fly.dev/healthz

# Test agent endpoint
curl https://bchat-pg.fly.dev/api/v1/agent/your-slug/validate
```

---

## Configuration Reference

### fly_pg.toml vs fly.toml

| Setting | `fly.toml` (SQLite) | `fly_pg.toml` (Neon) |
|---------|---------------------|----------------------|
| App name | `bchat0534` | `bchat-pg` |
| `[[mounts]]` | Required (`memos_data`) | **None** — Neon replaces volume |
| `MEMOS_DRIVER` | Not set (defaults to sqlite) | `'postgres'` in `[env]` |
| `DATABASE_URL` | Not set | Set via `fly secrets set` |
| Dockerfile | `Dockerfile.s3.fly` | `Dockerfile.pg.fly` |

### Env Var Flow (Production)

```
fly secrets set DATABASE_URL="postgresql://..."
    ↓
Docker container starts, entrypoint.sh runs
    ↓
bin/memos/main.go → viper reads MEMOS_DRIVER=postgres from env
    ↓
profile.Validate(): p.DSN == "" → p.DSN = os.Getenv("DATABASE_URL")
    ↓
store/db/db.go: switch "postgres" → postgres.NewDB(profile)
    ↓
store/db/postgres/postgres.go: sql.Open("pgx", profile.DSN)
    ↓
pgx/v5 handles sslmode=require natively for Neon
```

---

## S3 Configuration Reference

`Dockerfile.pg.fly` bakes in S3 env vars for Tigrisdata. Only the bucket name needs to be set as a Fly secret.

| Variable | Value | Source |
|----------|-------|--------|
| `LANCEDB_STORAGE_PROVIDER` | `s3` | `fly_pg.toml` `[env]` |
| `LANCEDB_S3_ENDPOINT` | `t3.storage.dev` | Baked into Dockerfile |
| `LANCEDB_S3_REGION` | `auto` | Baked into Dockerfile |
| `LANCEDB_S3_FORCE_PATH_STYLE` | `false` | Baked into Dockerfile |
| `LANCEDB_S3_BUCKET` | Your bucket name | Fly secret |
| `AWS_ACCESS_KEY_ID` | Tigrisdata credential | Fly secret (auto by `fly storage create`) |
| `AWS_SECRET_ACCESS_KEY` | Tigrisdata credential | Fly secret (auto by `fly storage create`) |

### Create S3 Storage

```bash
# Create Tigrisdata bucket (auto-generates AWS credentials)
fly storage create --app bchat-pg

# Set bucket name as secret
fly -a bchat-pg secrets set LANCEDB_S3_BUCKET="your-bucket-name"
```

### Local LanceDB Fallback

If `LANCEDB_S3_BUCKET` is not set or S3 is unreachable, LanceDB falls back to local storage at `/var/opt/memos` (created by `Dockerfile.pg.fly`).

---

## Quick Reference

| Action | Command |
|--------|---------|
| Set all secrets (recommended) | `./scripts/fly-pg-secrets.sh` |
| Validate migrations | `task -t Taskfile_pg.yml validate:migrations` |
| Start local Postgres | `task -t Taskfile_pg.yml postgres:start` |
| Stop local Postgres | `task -t Taskfile_pg.yml postgres:stop` |
| Deploy | `fly -a bchat-pg deploy -c fly_pg.toml` |
| Check logs | `fly -a bchat-pg logs` |
| Set secrets (manual) | `fly -a bchat-pg secrets set KEY=VALUE` |

---

## Troubleshooting

### "unknown db driver"
`MEMOS_DRIVER` env var not set. Ensure `fly_pg.toml` has `MEMOS_DRIVER = 'postgres'` in `[env]`.

### "postgres driver requires DSN or DATABASE_URL"
`DATABASE_URL` not set as Fly secret. Run `fly -a bchat-pg secrets set DATABASE_URL="..."`.

### "failed to ping database"
- Check Neon is not paused (free tier autosuspends after ~5 min)
- Verify `sslmode=require` in connection string
- Check network connectivity

### Neon cold start delay
Free tier databases pause after ~5 min inactivity. First connection takes 2-5s. The 60s ping timeout handles this.

### LanceDB S3 upload fails
Check that `LANCEDB_S3_BUCKET`, `AWS_ACCESS_KEY_ID`, and `AWS_SECRET_ACCESS_KEY` are set as Fly secrets. Verify with `fly secrets list --app bchat-pg`.

### S3 bucket not found
Run `fly storage list --app bchat-pg` to see available buckets. If none exist, create one: `fly storage create --app bchat-pg`.

---

*Document Version: 1.1*
*Updated: Added S3 configuration reference and automated secrets script*
