# Docker Deployment to fly.io with LanceDB RAG Support

**Status:** Complete
**Date:** 2026-01-26
**Related Documents:**
- [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD) - RAG implementation overview
- [DOCS_CYBERTRON.MD](./DOCS_CYBERTRON.MD) - Cybertron native Go embeddings
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Environment variables reference

---

## Overview

This document describes how to deploy the bchat application to fly.io with full LanceDB RAG (Retrieval-Augmented Generation) pipeline support. The deployment uses a multi-stage Docker build that compiles the Go backend with CGO/LanceDB support and bundles the native shared library.

### Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        fly.io Deployment                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────────────┐  │
│  │   fly.io    │    │  Tigrisdata │    │     OpenRouter API      │  │
│  │   Machine   │───▶│   S3 Bucket │    │   (LLM + Embeddings)    │  │
│  └──────┬──────┘    └─────────────┘    └─────────────────────────┘  │
│         │                                                           │
│         │ Volume Mount                                              │
│         ▼                                                           │
│  ┌─────────────┐                                                    │
│  │   SQLite    │                                                    │
│  │  Database   │                                                    │
│  └─────────────┘                                                    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Prerequisites

### Required Tools

| Tool | Version | Purpose |
|------|---------|---------|
| flyctl | Latest | fly.io CLI |
| Docker | 20+ | Container builds (optional for local testing) |

### Required Accounts

- **fly.io account** - [Sign up](https://fly.io/app/sign-up)
- **OpenRouter API key** - [Get API key](https://openrouter.ai/)

---

## Files Reference

### Dockerfile.fly

**Location:** `Dockerfile.fly` (project root)

Multi-stage Dockerfile with three build stages:

| Stage | Base Image | Purpose |
|-------|------------|---------|
| 1. `frontend` | `node:20-alpine` | Build React frontend |
| 2. `backend` | `golang:1.24` | Compile Go with CGO/LanceDB |
| 3. Runtime | `debian:bookworm-slim` | Minimal production image |

### fly.toml

**Location:** `fly.toml` (project root)

fly.io deployment configuration defining:
- App name and region
- Environment variables
- HTTP service configuration
- VM resources
- Volume mounts

---

## Build Stages Explained

### Stage 1: Frontend Build

```dockerfile
FROM node:20-alpine AS frontend
WORKDIR /frontend-build
COPY web/package*.json ./
RUN npm ci
COPY web/ .
RUN npm run release
```

Builds the React frontend using Node.js. The `npm run release` command creates optimized production assets in `web/dist`.

### Stage 2: Backend Build with LanceDB

```dockerfile
FROM golang:1.24 AS backend

# Install CGO dependencies
RUN apt-get update && apt-get install -y gcc libc-dev

# Copy LanceDB native libraries
COPY lib/linux_amd64/ /usr/local/lib/lancedb/
COPY include/ /usr/local/include/lancedb/

# Build with RAG support
ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-I/usr/local/include/lancedb"
ENV CGO_LDFLAGS="-L/usr/local/lib/lancedb -llancedb_go -Wl,-rpath,/usr/local/lib/lancedb"

RUN go build -tags rag -ldflags="-s -w" -o memos ./bin/memos/main.go
```

Key points:
- Uses `golang:1.24` (Debian-based) for glibc compatibility
- Copies LanceDB shared library (`.so`) and headers
- Sets CGO flags for native library linking
- Uses `-tags rag` to enable LanceDB code paths

### Stage 3: Runtime Image

```dockerfile
FROM debian:bookworm-slim

# Copy LanceDB shared library
COPY --from=backend /usr/local/lib/lancedb/liblancedb_go.so /usr/local/lib/
RUN ldconfig

# Copy application
COPY --from=backend /backend-build/memos .
```

Key points:
- Uses Debian slim (not Alpine) for glibc compatibility
- Copies only the shared library needed at runtime
- Runs `ldconfig` to register the library

---

## Environment Variables

### Application Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MEMOS_MODE` | `prod` | Application mode |
| `MEMOS_PORT` | `5230` | HTTP server port |
| `TZ` | `UTC` | Timezone |

### RAG Pipeline Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `RAG_PIPELINE_ENABLED` | `true` | Enable RAG retrieval pipeline |
| `LANCEDB_STORAGE_PROVIDER` | `s3` | Storage backend (`memory`, `local`, `s3`) |
| `LANCEDB_S3_ENDPOINT` | `fly.storage.tigris.dev` | S3 endpoint for Tigrisdata |
| `LANCEDB_S3_REGION` | `auto` | S3 region |
| `LANCEDB_S3_BUCKET` | (required) | S3 bucket name |
| `EMBEDDING_PROVIDER` | `openrouter` | Embedding provider (`cybertron`, `openrouter`, `local`, `mock`) |
| `EMBEDDING_MODEL` | `openai/text-embedding-3-small` | Embedding model (varies by provider) |
| `CYBERTRON_MODELS_DIR` | `~/.cybertron/models` | Directory for Cybertron model cache |
| `LLM_VERIFIER_ENABLED` | `false` | Enable LLM response verification |

**Embedding Provider Options:**

| Provider | Use Case | Cost | Notes |
|----------|----------|------|-------|
| `cybertron` | Development/Production | Free | Native Go, no external deps, semantically accurate |
| `openrouter` | Production | API costs | Higher quality (1536 dim), requires API key |
| `mock` | Testing only | Free | NO semantic accuracy - pipeline testing only |
| `local` | Development | Free | Requires Python embedding server |

### Secrets (set via fly secrets)

| Variable | Description |
|----------|-------------|
| `OPENROUTER_API_KEY` | OpenRouter API key for LLM and embeddings |
| `AWS_ACCESS_KEY_ID` | Tigrisdata access key |
| `AWS_SECRET_ACCESS_KEY` | Tigrisdata secret key |

---

## Deployment Steps

### 1. Install flyctl

```bash
# macOS
brew install flyctl

# Linux
curl -L https://fly.io/install.sh | sh

# Windows
powershell -Command "iwr https://fly.io/install.ps1 -useb | iex"
```

### 2. Login to fly.io

```bash
fly auth login
```

### 3. Create Tigrisdata Storage

```bash
# Create a Tigrisdata bucket (automatically sets AWS credentials)
fly storage create

# Output will show the bucket name, e.g.:
# Created storage bucket: bchat-storage-abc123
```

### 4. Set Required Secrets

```bash
# Set the bucket name from step 3
fly secrets set LANCEDB_S3_BUCKET=bchat-storage-abc123

# Set your OpenRouter API key
fly secrets set OPENROUTER_API_KEY=sk-or-v1-xxxxxxxxxxxxx
```

### 5. Create Volume for SQLite

```bash
fly volumes create memos_data --size 1 --region ord
```

### 6. Deploy

```bash
fly deploy
```

The first deployment takes longer as it builds the Docker image.

---

## Verification

### Check Deployment Status

```bash
# View app status
fly status

# View logs
fly logs

# Open in browser
fly open
```

### Expected Log Messages

On successful deployment, you should see:

```
INFO LanceDB vector database initialized uri=s3://bucket-name/lancedb provider=s3
INFO RAG pipeline enabled
```

When KB/Policy files are uploaded:

```
INFO Indexed content for RAG tenantID=1 audience=external chunks=47
```

When chat messages are processed:

```
INFO Using RAG pipeline for response generation
INFO Retrieved context for RAG query="..." chunks=5
```

### Verification Checklist

- [ ] App starts without errors: `fly logs`
- [ ] LanceDB initialized: Look for "LanceDB vector database initialized"
- [ ] S3 connection working: Look for "uri=s3://..."
- [ ] File uploads trigger indexing: Upload KB.MD and check for "Indexed content for RAG"
- [ ] RAG retrieval works: Send chat message and look for "Using RAG pipeline"

---

## Troubleshooting

### Common Issues

#### 1. "error loading shared library: liblancedb_go.so"

**Cause:** The shared library is not in the library path.

**Solution:** Ensure `ldconfig` runs after copying the library:
```dockerfile
COPY --from=backend /usr/local/lib/lancedb/liblancedb_go.so /usr/local/lib/
RUN ldconfig
```

#### 2. "LanceDB storage (local/s3) requires RAG build"

**Cause:** The binary was built without the `-tags rag` flag.

**Solution:** Ensure the build command includes:
```dockerfile
RUN go build -tags rag -ldflags="-s -w" -o memos ./bin/memos/main.go
```

#### 3. S3 Connection Errors

**Cause:** Missing or incorrect Tigrisdata credentials.

**Solution:** Verify secrets are set:
```bash
fly secrets list
# Should show: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, LANCEDB_S3_BUCKET
```

#### 4. Volume Mount Issues

**Cause:** Volume not created or in different region.

**Solution:**
```bash
# Check existing volumes
fly volumes list

# Create in correct region
fly volumes create memos_data --size 1 --region ord
```

#### 5. Out of Memory Errors

**Cause:** Insufficient RAM for LanceDB operations.

**Solution:** Increase VM memory in `fly.toml`:
```toml
[[vm]]
  memory = "1gb"  # Increase from 512mb
```

### Debug Commands

```bash
# SSH into running machine
fly ssh console

# Check library is loaded
ldd /usr/local/memos/memos | grep lancedb

# Check environment variables
env | grep -E "(LANCEDB|RAG|EMBEDDING)"

# View SQLite database
sqlite3 /var/opt/memos/memos.db ".tables"
```

---

## Local Development with Docker

### Build Locally

```bash
docker build -f Dockerfile.fly -t bchat:rag .
```

### Run with Cybertron Embeddings (Recommended)

Native Go embeddings, no external dependencies, semantically accurate.

```bash
docker run -p 5230:5230 \
  -v $(pwd)/data:/var/opt/memos \
  -v ~/.cybertron:/root/.cybertron \
  -e LANCEDB_STORAGE_PROVIDER=local \
  -e LANCEDB_LOCAL_PATH=/var/opt/memos/lancedb \
  -e EMBEDDING_PROVIDER=cybertron \
  -e EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L6-v2 \
  -e OPENROUTER_API_KEY=sk-or-xxxxx \
  bchat:rag
```

**Note:** Mount `~/.cybertron` to cache downloaded models between container restarts.

### Run with Mock Embeddings (Pipeline Testing Only)

Fast startup, but NO semantic accuracy.

```bash
docker run -p 5230:5230 \
  -e LANCEDB_STORAGE_PROVIDER=memory \
  -e EMBEDDING_PROVIDER=mock \
  -e OPENROUTER_API_KEY=sk-or-xxxxx \
  bchat:rag
```

### Run with Local Storage (Persistent)

```bash
docker run -p 5230:5230 \
  -v $(pwd)/data:/var/opt/memos \
  -v ~/.cybertron:/root/.cybertron \
  -e LANCEDB_STORAGE_PROVIDER=local \
  -e LANCEDB_LOCAL_PATH=/var/opt/memos/lancedb \
  -e EMBEDDING_PROVIDER=cybertron \
  -e OPENROUTER_API_KEY=sk-or-xxxxx \
  bchat:rag
```

---

## Architecture Notes

### Why Debian Instead of Alpine?

The prebuilt LanceDB native library (`liblancedb_go.so`) is compiled against glibc. Alpine Linux uses musl libc, which is incompatible. Using Debian ensures glibc compatibility.

### Why Shared Library (.so) Instead of Static (.a)?

The static library (`liblancedb_go.a`) was built with BSD ar format (macOS). The GNU linker on Linux cannot read this format, causing link errors. The shared library works across platforms.

### Image Size Considerations

| Component | Size |
|-----------|------|
| Debian slim base | ~80MB |
| LanceDB shared library | ~179MB |
| Go binary | ~50MB |
| Frontend assets | ~5MB |
| **Total** | **~300-350MB** |

### Persistence Strategy

| Data | Storage | Persistence |
|------|---------|-------------|
| Vector indexes | Tigrisdata S3 | Survives redeployments |
| SQLite database | fly.io Volume | Survives redeployments |
| Config cache | In-memory | Lost on restart |

---

## Scaling Considerations

### Horizontal Scaling

fly.io can run multiple machines. For horizontal scaling:

1. Use external PostgreSQL instead of SQLite
2. LanceDB S3 storage is already shared across instances
3. Consider sticky sessions for chat continuity

### Vertical Scaling

Increase VM resources in `fly.toml`:

```toml
[[vm]]
  memory = "1gb"
  cpu_kind = "shared"
  cpus = 2
```

Or use dedicated CPUs:

```toml
[[vm]]
  memory = "2gb"
  cpu_kind = "performance"
  cpus = 2
```

---

## Cost Estimates

### fly.io Pricing (as of 2026)

| Resource | Specification | Approximate Cost |
|----------|---------------|------------------|
| Shared CPU VM | 1 CPU, 512MB RAM | ~$5/month |
| Volume (1GB) | SSD storage | ~$0.15/month |
| Tigrisdata S3 | First 5GB free | Free (under 5GB) |
| Outbound bandwidth | First 100GB free | Free (typical usage) |

### OpenRouter API Costs

| Model | Usage | Cost |
|-------|-------|------|
| text-embedding-3-small | Embeddings | $0.02/1M tokens |
| gpt-4o-mini | Chat responses | ~$0.15/1M tokens |

### Embedding Provider Costs

| Provider | Cost | Notes |
|----------|------|-------|
| `cybertron` | Free | Native Go, ~90MB model download on first use |
| `mock` | Free | No semantic accuracy - testing only |
| `openrouter` | ~$0.02/1M tokens | Higher quality embeddings |

**Recommendation:** Use `cybertron` for development and cost-sensitive production. Use `openrouter` when highest embedding quality is required.

---

*Document Version: 1.1*
*Updated: 2026-01-26* - Added Cybertron embedding provider
