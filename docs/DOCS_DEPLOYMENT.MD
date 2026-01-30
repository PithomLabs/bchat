# Deploying bchat to fly.io

**Status:** Complete
**Date:** 2026-01-30
**Related Documents:**
- [DOCS_DOCKERFILE.MD](./DOCS_DOCKERFILE.MD) - Docker deployment overview
- [DOCS_UNIFIED_ENV_WORKFLOW.MD](./DOCS_UNIFIED_ENV_WORKFLOW.MD) - Environment variables reference
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Complete environment variable reference

---

## Overview

This document provides step-by-step procedures for deploying bchat to fly.io with full RAG (Retrieval-Augmented Generation) support. The deployment mirrors the local `task run:rag:l12` configuration.

### Architecture: Local vs fly.io

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            LOCAL DEVELOPMENT                                │
│                           (task run:rag:l12)                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────────────┐  │
│  │   Go Binary     │    │  Local LanceDB  │    │     OpenRouter API      │  │
│  │   ./build/memos │───▶│  ./build/data/  │    │  (LLM + L12 Embeddings) │  │
│  └────────┬────────┘    │     lancedb/    │    └─────────────────────────┘  │
│           │             └─────────────────┘                                 │
│           │                                                                 │
│           ▼                                                                 │
│  ┌─────────────────┐                                                        │
│  │     SQLite      │                                                        │
│  │ ./build/data/   │                                                        │
│  │  memos_dev.db   │                                                        │
│  └─────────────────┘                                                        │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                            FLY.IO PRODUCTION                                │
│                         (Dockerfile.ubuntu.fly)                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────────────┐  │
│  │   fly.io VM     │    │  Tigrisdata S3  │    │     OpenRouter API      │  │
│  │  Ubuntu 24.04   │───▶│   (LanceDB)     │    │  (LLM + L12 Embeddings) │  │
│  └────────┬────────┘    └─────────────────┘    └─────────────────────────┘  │
│           │                                                                 │
│           │ Volume Mount                                                    │
│           ▼                                                                 │
│  ┌─────────────────┐                                                        │
│  │     SQLite      │                                                        │
│  │ /var/opt/memos/ │                                                        │
│  │    memos.db     │                                                        │
│  └─────────────────┘                                                        │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Configuration Mapping: Local to fly.io

| Aspect | Local (`task run:rag:l12`) | fly.io |
|--------|---------------------------|--------|
| **RAG_PIPELINE_ENABLED** | `true` | `true` |
| **EMBEDDING_MODEL** | `sentence-transformers/all-minilm-l12-v2` | `sentence-transformers/all-minilm-l12-v2` |
| **EMBEDDING_PROVIDER** | `openrouter` (from .env) | `openrouter` |
| **LANCEDB_STORAGE_PROVIDER** | `local` | `s3` |
| **LANCEDB_LOCAL_PATH** | `./build/data/lancedb` | N/A (uses S3) |
| **LANCEDB_S3_ENDPOINT** | N/A | `fly.storage.tigris.dev` |
| **LANCEDB_S3_BUCKET** | N/A | Set via `fly secrets` |
| **Data directory** | `./build/data` | `/var/opt/memos` (volume) |
| **Secrets** | `.env` file | `fly secrets` |

---

## Prerequisites

### Required Tools

| Tool | Version | Installation |
|------|---------|--------------|
| **flyctl** | Latest | See [Install flyctl](#1-install-flyctl) |
| **Docker** | 20+ | Optional, for local testing |

### Required Accounts

1. **fly.io account** - [Sign up](https://fly.io/app/sign-up)
2. **OpenRouter API key** - [Get API key](https://openrouter.ai/keys)

### Required Files

Ensure these files exist in your project:

```
bchat/
├── Dockerfile.ubuntu.fly      # Ubuntu-based Dockerfile
├── fly.toml                   # fly.io configuration
├── scripts/entrypoint.sh      # Container entrypoint
├── lib/linux_amd64/           # LanceDB native libraries
│   └── liblancedb_go.so
└── include/                   # LanceDB headers
    └── lancedb_go.h
```

---

## First-Time Setup

### 1. Install flyctl

**macOS:**
```bash
brew install flyctl
```

**Linux:**
```bash
curl -L https://fly.io/install.sh | sh
```

**Windows:**
```powershell
powershell -Command "iwr https://fly.io/install.ps1 -useb | iex"
```

### 2. Login to fly.io

```bash
fly auth login
```

This opens a browser for authentication.

### 3. Create the fly.io App

```bash
# From the bchat project root
fly launch --no-deploy
```

When prompted:
- **App name:** Accept default or enter custom name (e.g., `bchat`)
- **Region:** Choose closest region (e.g., `ord` for Chicago)
- **PostgreSQL:** No
- **Redis:** No

This creates the app but doesn't deploy yet.

### 4. Create Tigrisdata Storage Bucket

```bash
fly storage create
```

**Expected output:**
```
Your Tigris project (bchat) was created

Setting the following secrets on bchat:
AWS_ACCESS_KEY_ID: tid_xxxxxxxxxxxxxxxxxxxx
AWS_SECRET_ACCESS_KEY: tsec_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
BUCKET_NAME: bchat-storage-xxxx

Secrets are staged for the first deployment
```

**Save the `BUCKET_NAME`** - you'll need it in the next step.

### 5. Set Required Secrets

```bash
# Set the S3 bucket name (from step 4)
fly secrets set LANCEDB_S3_BUCKET=bchat-storage-xxxx

# Set your OpenRouter API key
fly secrets set OPENROUTER_API_KEY=sk-or-v1-xxxxxxxxxxxxxxxxxxxxx

# Set encryption master key (for tenant API key encryption)
fly secrets set ENCRYPTION_MASTER_KEY=$(uuidgen)
```

### 6. Create Volume for SQLite Database

```bash
# Create a 1GB volume in your primary region
fly volumes create memos_data --size 1 --region ord
```

**Expected output:**
```
        ID: vol_xxxxxxxxxxxx
      Name: memos_data
       App: bchat
    Region: ord
      Zone: xxxx
   Size GB: 1
 Encrypted: true
Created at: 2026-01-30T12:00:00Z
```

### 7. Verify Configuration

```bash
# List all secrets (values are hidden)
fly secrets list
```

**Expected output:**
```
NAME                    DIGEST                          CREATED AT
AWS_ACCESS_KEY_ID       xxxxxxxxxxxxxxxxxxxxxxxx        2026-01-30T12:00:00Z
AWS_SECRET_ACCESS_KEY   xxxxxxxxxxxxxxxxxxxxxxxx        2026-01-30T12:00:00Z
LANCEDB_S3_BUCKET       xxxxxxxxxxxxxxxxxxxxxxxx        2026-01-30T12:00:00Z
OPENROUTER_API_KEY      xxxxxxxxxxxxxxxxxxxxxxxx        2026-01-30T12:00:00Z
ENCRYPTION_MASTER_KEY   xxxxxxxxxxxxxxxxxxxxxxxx        2026-01-30T12:00:00Z
```

---

## Step-by-Step Deployment

### Deploy to fly.io

```bash
fly deploy
```

**First deployment takes longer** as it builds the Docker image (~5-10 minutes).

**Expected build output:**
```
==> Building image
[+] Building...
 => [frontend 1/4] FROM docker.io/library/node:20-alpine
 => [backend 1/7] FROM docker.io/library/golang:1.24
 => [stage-2 1/6] FROM docker.io/library/ubuntu:24.04
...
 => exporting to image
==> Pushing image to fly
...
==> Creating release
--> release v1 created
==> Monitoring deployment
 1 desired, 1 placed, 1 healthy, 0 unhealthy
--> v1 deployed successfully
```

### Verify Deployment

```bash
# Check app status
fly status
```

**Expected output:**
```
App
  Name     = bchat
  Owner    = personal
  Hostname = bchat.fly.dev
  Platform = machines

Machines
ID              STATE   REGION  HEALTH CHECKS
xxxxxxxxxxxx    started ord     1 total, 1 passing
```

```bash
# View logs
fly logs
```

**Expected log messages on successful startup:**
```
INFO Starting bchat server mode=prod port=5230
INFO LanceDB vector database initialized uri=s3://bchat-storage-xxxx/lancedb provider=s3
INFO RAG pipeline enabled embedding_provider=openrouter embedding_model=sentence-transformers/all-minilm-l12-v2
INFO Server listening on :5230
```

### Open the Application

```bash
fly open
```

This opens `https://bchat.fly.dev` in your browser.

---

## Verification Checklist

After deployment, verify each component is working:

### 1. Application Starts

```bash
fly logs | grep "Server listening"
```
**Expected:** `INFO Server listening on :5230`

### 2. LanceDB Initialized with S3

```bash
fly logs | grep "LanceDB"
```
**Expected:** `INFO LanceDB vector database initialized uri=s3://... provider=s3`

### 3. RAG Pipeline Enabled

```bash
fly logs | grep "RAG pipeline"
```
**Expected:** `INFO RAG pipeline enabled`

### 4. Test File Upload and Indexing

1. Open the app in browser
2. Navigate to Agent Admin
3. Upload a KB.MD file
4. Check logs:

```bash
fly logs | grep "Indexed content"
```
**Expected:** `INFO Indexed content for RAG tenantID=X audience=external chunks=N`

### 5. Test Chat Functionality

1. Send a test chat message
2. Check logs:

```bash
fly logs | grep "Using RAG"
```
**Expected:** `INFO Using RAG pipeline for response generation`

### 6. Verify in OpenRouter Dashboard

Check [OpenRouter Activity](https://openrouter.ai/activity) to confirm:
- LLM requests are being made
- Embedding requests show `sentence-transformers/all-minilm-l12-v2`

---

## Common Operations

### View Logs

```bash
# Stream logs (live)
fly logs

# View last 100 lines
fly logs -n 100
```

### SSH into Container

```bash
fly ssh console
```

Useful commands inside container:
```bash
# Check library is loaded
ldd /usr/local/memos/memos | grep lancedb

# Check environment variables
env | grep -E "(LANCEDB|RAG|EMBEDDING)"

# View SQLite database
sqlite3 /var/opt/memos/memos.db ".tables"
```

### Restart Application

```bash
fly apps restart
```

### Scale Resources

```bash
# Increase memory
fly scale memory 1024

# View current resources
fly scale show
```

### Update Secrets

```bash
# Update a secret (triggers redeployment)
fly secrets set OPENROUTER_API_KEY=sk-or-v1-new-key
```

### Redeploy

```bash
# Deploy latest code
fly deploy

# Deploy with build cache cleared
fly deploy --no-cache
```

---

## Troubleshooting

### Error: "error loading shared library: liblancedb_go.so"

**Cause:** The shared library is not in the library path.

**Solution:** Ensure `ldconfig` runs after copying the library in Dockerfile:
```dockerfile
COPY --from=backend /usr/local/lib/lancedb/liblancedb_go.so /usr/local/lib/
RUN ldconfig
```

### Error: "LanceDB storage (local/s3) requires RAG build"

**Cause:** The binary was built without the `-tags rag` flag.

**Solution:** Ensure the build command includes:
```dockerfile
RUN go build -tags rag -ldflags="-s -w" -o memos ./bin/memos/main.go
```

### Error: S3 Connection Failed

**Cause:** Missing or incorrect Tigrisdata credentials.

**Solution:**
```bash
# Verify secrets are set
fly secrets list

# Re-create storage bucket if needed
fly storage create
```

### Error: Volume Not Mounted

**Cause:** Volume doesn't exist or is in a different region.

**Solution:**
```bash
# Check existing volumes
fly volumes list

# Create volume in correct region
fly volumes create memos_data --size 1 --region ord
```

### Error: Out of Memory

**Cause:** Insufficient RAM for LanceDB operations.

**Solution:** Increase VM memory:
```bash
fly scale memory 1024
```

Or update `fly.toml`:
```toml
[[vm]]
  memory = "1gb"
```

### Error: Machine Health Check Failed

**Cause:** Application not responding on port 5230.

**Solution:**
```bash
# Check logs for startup errors
fly logs -n 200

# Verify port configuration
grep MEMOS_PORT fly.toml
```

### Error: glibc Version Mismatch

**Cause:** LanceDB library compiled against different glibc version.

**Solution:** If Ubuntu 24.04 has compatibility issues, fall back to 22.04:
```dockerfile
FROM ubuntu:22.04
```

---

## Rollback Procedures

### Rollback to Previous Release

```bash
# List all releases
fly releases

# Rollback to specific version
fly deploy --image registry.fly.io/bchat:deployment-XXXX
```

### Rollback Secrets

```bash
# Secrets cannot be "rolled back" - you must re-set the old value
fly secrets set OPENROUTER_API_KEY=sk-or-v1-old-key
```

### Disaster Recovery

If you need to recreate everything:

```bash
# 1. Destroy app (WARNING: deletes everything)
fly apps destroy bchat

# 2. Recreate app
fly launch --no-deploy

# 3. Recreate storage
fly storage create

# 4. Set secrets
fly secrets set LANCEDB_S3_BUCKET=new-bucket-name
fly secrets set OPENROUTER_API_KEY=sk-or-v1-xxx
fly secrets set ENCRYPTION_MASTER_KEY=$(uuidgen)

# 5. Create volume
fly volumes create memos_data --size 1 --region ord

# 6. Deploy
fly deploy
```

**Note:** SQLite data on the old volume is lost. Vector indexes in S3 persist if using the same bucket.

---

## Environment Variable Reference

### Set via fly.toml (non-sensitive)

| Variable | Value | Description |
|----------|-------|-------------|
| `MEMOS_MODE` | `prod` | Application mode |
| `MEMOS_PORT` | `5230` | HTTP server port |
| `TZ` | `UTC` | Timezone |
| `RAG_PIPELINE_ENABLED` | `true` | Enable RAG pipeline |
| `LANCEDB_STORAGE_PROVIDER` | `s3` | Use Tigrisdata S3 |
| `LANCEDB_S3_ENDPOINT` | `fly.storage.tigris.dev` | S3 endpoint |
| `LANCEDB_S3_REGION` | `auto` | S3 region |
| `EMBEDDING_PROVIDER` | `openrouter` | Use OpenRouter for embeddings |
| `EMBEDDING_MODEL` | `sentence-transformers/all-minilm-l12-v2` | L12 embedding model |
| `LLM_VERIFIER_ENABLED` | `false` | Optional LLM verifier |

### Set via fly secrets (sensitive)

| Variable | Description | How to Set |
|----------|-------------|------------|
| `OPENROUTER_API_KEY` | OpenRouter API key | `fly secrets set OPENROUTER_API_KEY=sk-or-v1-xxx` |
| `ENCRYPTION_MASTER_KEY` | Tenant API key encryption | `fly secrets set ENCRYPTION_MASTER_KEY=$(uuidgen)` |
| `AWS_ACCESS_KEY_ID` | Tigrisdata access key | Auto-set by `fly storage create` |
| `AWS_SECRET_ACCESS_KEY` | Tigrisdata secret key | Auto-set by `fly storage create` |
| `LANCEDB_S3_BUCKET` | S3 bucket name | `fly secrets set LANCEDB_S3_BUCKET=xxx` |

---

## Cost Estimates

### fly.io Resources

| Resource | Specification | Approximate Cost |
|----------|---------------|------------------|
| Shared CPU VM | 1 CPU, 512MB RAM | ~$5/month |
| Volume (1GB) | SSD storage | ~$0.15/month |
| Tigrisdata S3 | First 5GB free | Free |
| Outbound bandwidth | First 100GB free | Free |

### OpenRouter API

| Model | Usage | Cost |
|-------|-------|------|
| sentence-transformers/all-minilm-l12-v2 | Embeddings | ~$0.02/1M tokens |
| openai/gpt-4o-mini | Chat responses | ~$0.15/1M tokens |

---

## See Also

- [DOCS_DOCKERFILE.MD](./DOCS_DOCKERFILE.MD) - Docker deployment details
- [DOCS_UNIFIED_ENV_WORKFLOW.MD](./DOCS_UNIFIED_ENV_WORKFLOW.MD) - Environment workflow
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Complete environment variable reference
- [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD) - RAG implementation details

---

*Document Version: 1.0*
*Created: 2026-01-30*
