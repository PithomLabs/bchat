# Unified Environment Variable Workflow

This document describes the secure, unified approach to managing environment variables across local development and production (fly.io) environments.

## Quick Start

### Local Development

```bash
# 1. Copy the template
cp .env.example .env

# 2. Edit .env with your values
nano .env  # or your preferred editor

# 3. Run the server (automatically sources .env)
task run:rag
```

### Production (fly.io)

```bash
# Set secrets (never commit these!)
fly secrets set OPENROUTER_API_KEY=sk-or-v1-xxx
fly secrets set ENCRYPTION_MASTER_KEY=$(uuidgen)

# Deploy
fly deploy
```

---

## Environment Variable Categories

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `OPENROUTER_API_KEY` | OpenRouter API key for LLM chat | `sk-or-v1-xxxxx` |

### Security Variables

| Variable | Description | Notes |
|----------|-------------|-------|
| `ENCRYPTION_MASTER_KEY` | AES-256 encryption key for tenant API keys | Generate with `uuidgen` |

**WARNING:** If you lose `ENCRYPTION_MASTER_KEY`, encrypted tenant API keys cannot be recovered!

### LLM Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LLM_MODEL` | `openai/gpt-4o-mini` | Default chat model |
| `LLM_MODEL_REASONING` | `google/gemini-2.5-pro` | Model for Generate KB/Policy |

### RAG Pipeline

| Variable | Default | Description |
|----------|---------|-------------|
| `RAG_PIPELINE_ENABLED` | `false` | Enable RAG features |
| `EMBEDDING_PROVIDER` | `openrouter` | `openrouter`, `mock`, or `local` |
| `EMBEDDING_MODEL` | `openai/text-embedding-3-small` | Embedding model |
| `LANCEDB_STORAGE_PROVIDER` | `memory` | `memory`, `local`, or `s3` |

### S3/Tigrisdata (Production)

| Variable | Description |
|----------|-------------|
| `AWS_ACCESS_KEY_ID` | Tigrisdata access key (`tid_xxxxx`) |
| `AWS_SECRET_ACCESS_KEY` | Tigrisdata secret (`tsec_xxxxx`) |
| `LANCEDB_S3_BUCKET` | S3 bucket name |
| `LANCEDB_S3_ENDPOINT` | `fly.storage.tigris.dev` |
| `LANCEDB_S3_REGION` | `auto` |

---

## Configuration Priority

Environment variables follow this priority order:

```
1. Tenant Config (Agent Admin UI)  ← Highest priority
         ↓ (if empty)
2. Environment Variable            ← Fallback
         ↓ (if empty)
3. Hardcoded Default               ← Lowest priority
```

This means tenant-specific settings in the Agent Admin always override environment variables.

---

## Local Development

### Using .env File

The recommended approach for local development:

1. **Copy the template:**
   ```bash
   cp .env.example .env
   ```

2. **Edit with your values:**
   ```bash
   # .env (example)
   OPENROUTER_API_KEY=sk-or-v1-your-actual-key
   ENCRYPTION_MASTER_KEY=ced26e6e-91ce-4120-b4c0-68ec9ea25701
   RAG_PIPELINE_ENABLED=true
   ```

3. **Run the server:**
   ```bash
   task run:rag
   ```

The Taskfile automatically sources `.env` if present.

### Available Run Commands

| Command | Description |
|---------|-------------|
| `task run` | Basic server (no RAG) |
| `task run:rag` | Server with RAG (local storage) |
| `task run:rag:l12` | Server with RAG + L12 embeddings |

### Why Not Inline Variables?

**Bad (insecure):**
```bash
OPENROUTER_API_KEY=sk-or-xxx task run:rag
```

Problems:
- Appears in shell history
- Visible in `ps` output
- Easy to accidentally commit

**Good (secure):**
```bash
# Secrets in .env file (git-ignored)
task run:rag
```

---

## Docker Deployment

### Using --env-file

```bash
docker run --env-file .env -p 5230:5230 bchat
```

### Using Docker Secrets (_FILE suffix)

The entrypoint supports secrets from files:

```bash
# Create secrets
echo "sk-or-v1-xxx" > /run/secrets/openrouter_key

# Run with secret file reference
docker run \
  -e OPENROUTER_API_KEY_FILE=/run/secrets/openrouter_key \
  -v /run/secrets:/run/secrets:ro \
  bchat
```

Supported `_FILE` variables:
- `MEMOS_DSN_FILE`
- `OPENROUTER_API_KEY_FILE`
- `ENCRYPTION_MASTER_KEY_FILE`
- `AWS_ACCESS_KEY_ID_FILE`
- `AWS_SECRET_ACCESS_KEY_FILE`

---

## Production (fly.io)

### Setting Secrets

Use `fly secrets` for sensitive values:

```bash
# Required
fly secrets set OPENROUTER_API_KEY=sk-or-v1-your-key

# For tenant API key encryption
fly secrets set ENCRYPTION_MASTER_KEY=$(uuidgen)

# For S3/Tigrisdata storage
fly secrets set AWS_ACCESS_KEY_ID=tid_xxxxx
fly secrets set AWS_SECRET_ACCESS_KEY=tsec_xxxxx
```

### Viewing Secrets

```bash
# List secret names (values hidden)
fly secrets list

# Output:
# NAME                    DIGEST                  CREATED AT
# OPENROUTER_API_KEY      abc123...               2024-01-01T00:00:00Z
# ENCRYPTION_MASTER_KEY   def456...               2024-01-01T00:00:00Z
```

### Non-Sensitive Variables

Non-sensitive variables go in `fly.toml`:

```toml
[env]
  MEMOS_MODE = "prod"
  MEMOS_PORT = "5230"
  RAG_PIPELINE_ENABLED = "true"
  LANCEDB_STORAGE_PROVIDER = "s3"
  LANCEDB_S3_ENDPOINT = "fly.storage.tigris.dev"
  EMBEDDING_PROVIDER = "openrouter"
```

### Deployment Workflow

```bash
# 1. Set secrets (first time only, or when rotating)
fly secrets set OPENROUTER_API_KEY=sk-or-v1-xxx
fly secrets set ENCRYPTION_MASTER_KEY=$(uuidgen)

# 2. Deploy
fly deploy

# 3. Verify
fly status
fly logs
```

---

## Security Best Practices

### Do

- Use `.env` files for local development
- Use `fly secrets` for production
- Generate unique `ENCRYPTION_MASTER_KEY` per environment
- Rotate API keys periodically
- Use `_FILE` suffix for Docker/K8s secrets

### Don't

- Commit `.env` files to version control
- Use inline environment variables in commands
- Share `ENCRYPTION_MASTER_KEY` across environments
- Log sensitive values (check your logging config)

### Key Rotation

**OpenRouter API Key:**
1. Generate new key at https://openrouter.ai/keys
2. Update via `fly secrets set` or `.env`
3. Old key can be revoked after deployment

**Encryption Master Key:**
⚠️ **WARNING:** Changing this will make existing encrypted tenant API keys unrecoverable!

1. Clear encrypted keys: `DELETE FROM tenant_config WHERE openrouter_api_key_encrypted IS NOT NULL;`
2. Set new key
3. Re-enter tenant API keys via Agent Admin

---

## Troubleshooting

### "Encryption not configured"

**Cause:** `ENCRYPTION_MASTER_KEY` not set

**Fix:**
```bash
# Local
echo "ENCRYPTION_MASTER_KEY=$(uuidgen)" >> .env

# Production
fly secrets set ENCRYPTION_MASTER_KEY=$(uuidgen)
```

### "Invalid API key format"

**Cause:** OpenRouter key doesn't start with `sk-or-`

**Fix:** Verify your key at https://openrouter.ai/keys

### "Both VAR and VAR_FILE are set"

**Cause:** Conflicting environment variable sources

**Fix:** Use either the variable OR the `_FILE` suffix, not both

### Environment not loading

**Cause:** `.env` file not found or malformed

**Fix:**
```bash
# Verify file exists
ls -la .env

# Check for syntax errors (no spaces around =)
cat .env | grep -v "^#" | grep -v "^$"
```

---

## File Reference

| File | Purpose |
|------|---------|
| `.env.example` | Template with all variables documented |
| `.env` | Your local configuration (git-ignored) |
| `fly.toml` | Non-sensitive production config |
| `scripts/entrypoint.sh` | Docker entrypoint with `_FILE` support |
| `Taskfile.yml` | Dev commands that source `.env` |

---

## See Also

- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Complete environment variable reference
- [DOCS_RBAC.MD](./DOCS_RBAC.MD) - Encryption and security details
- [fly.toml](../fly.toml) - Production configuration
