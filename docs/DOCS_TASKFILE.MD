# Taskfile Commands Reference

**Status:** Active
**Date:** 2026-01-26
**Related Documents:**
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG pipeline configuration
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Environment variables

---

## Overview

This project uses [Task](https://taskfile.dev/) as the build tool. All commands are defined in `Taskfile.yml`.

### Prerequisites

Install Task:
```bash
# macOS
brew install go-task

# Linux
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin

# Go install
go install github.com/go-task/task/v3/cmd/task@latest
```

---

## Quick Reference

| Command | Description |
|---------|-------------|
| `task build` | Build frontend + backend |
| `task build:rag` | Build with RAG/LanceDB support |
| `task dev:backend` | Run dev server |
| `task dev:backend:rag` | Run dev server with RAG (go run) |
| `task run:rag` | Run compiled binary with RAG + OpenRouter embeddings |
| `task run:rag:l12` | Run with L12 embeddings via OpenRouter (better quality, same 384 dim) |
| `task run:rag:mock` | Run compiled binary with RAG + mock embeddings |
| `task fly:check` | Validate env chain (.env -> Dockerfile -> fly.toml -> fly secrets) |
| `task fly:db-check` | Validate database migrations before fly.io deployment |
| `task fly:pre-deploy` | Run all pre-deployment checks (env + database) |

---

## Setup Commands

### task setup

Install all dependencies (npm packages and Go modules).

```bash
task setup
```

**What it does:**
- Runs `npm install` in the `web/` directory
- Runs `go mod download`

---

### task setup:lancedb

Download LanceDB native libraries required for RAG pipeline.

```bash
task setup:lancedb
```

**What it does:**
- Creates `lib/` and `include/` directories
- Downloads platform-specific LanceDB libraries from GitHub
- Required before building with `-tags rag`

**Platform support:**
- `linux_amd64` - Linux x86_64
- `darwin_amd64` - macOS Intel
- `darwin_arm64` - macOS Apple Silicon

**Note:** This is automatically run as a dependency of `build:backend:rag`.

---

## Build Commands

### task build

Build everything (frontend + backend) without RAG support.

```bash
task build
```

**Output:** `build/memos`

**Equivalent to:**
```bash
task build:frontend
task build:backend
```

---

### task build:frontend

Build frontend assets only.

```bash
task build:frontend
```

**What it does:**
- Runs `npm run release` in `web/` directory
- Outputs to `web/dist/`

---

### task build:backend

Build the Go binary without RAG support.

```bash
task build:backend
```

**Output:** `build/memos`

**Note:** This build does NOT include LanceDB. RAG will use in-memory storage only.

---

### task build:backend:rag

Build the Go binary with LanceDB RAG support for local disk storage.

```bash
task build:backend:rag
```

**Output:** `build/memos` (with RAG/LanceDB support)

**What it does:**
1. Automatically runs `setup:lancedb` if needed
2. Sets CGO flags for LanceDB native library
3. Builds with `-tags rag` flag

**Requirements:**
- CGO enabled (automatic)
- C compiler (gcc/clang)
- LanceDB native libraries (auto-downloaded)

**Environment set during build:**
```bash
CGO_ENABLED=1
CGO_CFLAGS="-I<project>/include"
CGO_LDFLAGS="-L<project>/lib/linux_amd64 -llancedb_go ..."
```

---

### task build:rag

Build everything (frontend + backend) with RAG support.

```bash
task build:rag
```

**Equivalent to:**
```bash
task build:frontend
task build:backend:rag
```

---

## Development Commands

### task dev:frontend

Run the frontend development server with hot reload.

```bash
task dev:frontend
```

**URL:** http://localhost:5173 (Vite default)

---

### task dev:backend

Run the backend development server.

```bash
task dev:backend
```

**URL:** http://localhost:8081

**Equivalent to:**
```bash
go run ./bin/memos/main.go --mode dev
```

---

### task dev:backend:rag

Run the backend development server with RAG pipeline enabled.

```bash
task dev:backend:rag
```

**What it does:**
1. Automatically runs `setup:lancedb` if needed
2. Sets CGO and library path environment
3. Enables RAG pipeline with local embedding provider
4. Runs with `-tags rag` flag

**Environment set automatically:**
```bash
CGO_ENABLED=1
LD_LIBRARY_PATH=<project>/lib/linux_amd64
RAG_PIPELINE_ENABLED=true
EMBEDDING_PROVIDER=local
```

**Note:** Requires local embedding server running (`task embedding:server`).

---

### task run:rag

Run the compiled binary with RAG enabled using OpenRouter embeddings.

```bash
task run:rag
```

**What it does:**
1. Builds the backend with RAG support (if needed)
2. Sets RAG environment variables
3. Runs the compiled binary

**Environment set automatically:**
```bash
RAG_PIPELINE_ENABLED=true
LANCEDB_STORAGE_PROVIDER=local
EMBEDDING_PROVIDER=openrouter
```

**Requirements:**
- `OPENROUTER_API_KEY` environment variable must be set

**Usage:**
```bash
export OPENROUTER_API_KEY=sk-or-v1-xxx
task run:rag
```

---

### task run:rag:mock

Run the compiled binary with RAG enabled using mock embeddings (no API key needed).

```bash
task run:rag:mock
```

**What it does:**
1. Builds the backend with RAG support (if needed)
2. Sets RAG environment variables with mock embeddings
3. Runs the compiled binary

**Environment set automatically:**
```bash
RAG_PIPELINE_ENABLED=true
LANCEDB_STORAGE_PROVIDER=local
EMBEDDING_PROVIDER=mock
```

**Note:** Mock embeddings generate random vectors - useful for testing the pipeline without API costs, but no semantic accuracy.

---

### task run:rag:l12

Run with L12 embeddings for better quality while maintaining same 384 dimensions.

```bash
task run:rag:l12
```

**What it does:**
1. Builds the backend with RAG support (if needed)
2. Sets RAG environment variables with L12 model via OpenRouter
3. Runs the compiled binary

**Environment set automatically:**
```bash
RAG_PIPELINE_ENABLED=true
LANCEDB_STORAGE_PROVIDER=local
EMBEDDING_PROVIDER=openrouter
EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L12-v2
```

**Trade-offs:**
- Better semantic quality than L6 (12 layers vs 6)
- Same 384 dimensions - no reindexing when switching from L6
- Slightly slower than L6
- Requires OpenRouter API key

---

## Utility Commands

### task generate

Run Go code generation.

```bash
task generate
```

**What it does:**
- Runs `go generate ./...`

---

### task fly:check

Validate environment variable consistency across the entire deployment chain before deploying to fly.io.

```bash
task fly:check
```

**What it does:**
Validates the full environment chain: `.env` -> `Dockerfile` -> `fly.toml` -> `fly secrets`

1. **Reads `.env`** (source of truth for local development)
2. **Determines Dockerfile** from `fly.toml [build].dockerfile` setting
3. **Reads Dockerfile ENV declarations** with their default values
4. **Reads fly.toml [env] section** runtime overrides
5. **Reads fly secrets** (if flyctl installed)
6. **Validates:**
   - Non-sensitive `.env` vars exist in Dockerfile (warning if missing)
   - Dockerfile ENV values match fly.toml [env] values (error if mismatch)
   - Sensitive vars (`OPENROUTER_API_KEY`, `ENCRYPTION_MASTER_KEY`) are in fly secrets (error if missing)
   - Sensitive vars are NOT exposed in Dockerfile or fly.toml (security error)

**Context-aware storage checks:**
- When using `Dockerfile.local.fly`: checks `LANCEDB_LOCAL_PATH`, skips S3 vars
- When using `Dockerfile.s3.fly`: checks S3 vars (`LANCEDB_S3_ENDPOINT`, etc.), skips local vars

**Example output:**
```
=== Pre-Deployment Environment Chain Check ===

Step 1: Reading .env (source of truth)...
   Found 8 variables

Step 2: Determining Dockerfile from fly.toml...
   Using: Dockerfile.local.fly
   Storage type: local

Step 3: Reading Dockerfile.local.fly ENV declarations...
   Found 9 ENV declarations

Step 4: Reading fly.toml [env] section...
   Found 9 variables

Step 5: Reading fly secrets...
   Found 2 secrets

=== Validation Results ===

Check: .env -> Dockerfile (non-sensitive vars)
  All non-sensitive .env vars accounted for

Check: Dockerfile -> fly.toml (value consistency)
  All Dockerfile ENV vars match fly.toml

Check: Sensitive vars in fly secrets
  OK: OPENROUTER_API_KEY in fly secrets
  OK: ENCRYPTION_MASTER_KEY in fly secrets

Check: Sensitive vars NOT exposed (security)
  No sensitive vars exposed

=== Summary ===
All checks passed!
```

**Error conditions:**
| Check | Failure Type | Description |
|-------|--------------|-------------|
| .env -> Dockerfile | Warning | Non-sensitive var in .env but not in Dockerfile |
| Dockerfile -> fly.toml | Error | ENV var missing from fly.toml or value mismatch |
| Sensitive -> secrets | Error | `OPENROUTER_API_KEY` or `ENCRYPTION_MASTER_KEY` not in fly secrets |
| Security check | Error | Sensitive var exposed in Dockerfile or fly.toml |

**Note:** Run this before `fly deploy` to catch configuration drift and security issues.

---

### task fly:db-check

Comprehensive database migration validation before fly.io deployment.

```bash
task fly:db-check
```

**What it does:**
1. Runs `validate:migrations` (LATEST.sql sync check)
2. Validates migration file numbering (no gaps, proper sequence)
3. Tests LATEST.sql can create a fresh database (simulates new fly.io deployment)
4. Tests all migrations can apply to an empty database in sequence
5. Compares final schema after migrations vs LATEST.sql schema

**Example output:**
```
=== Pre-Deployment Database Migration Check ===

Step 1: Checking LATEST.sql sync...
PASSED

Step 2: Checking migration file numbering...
PASSED: Found 22 migrations in sequence (00-21)

Step 3: Testing LATEST.sql creates valid database...
PASSED: Created database with 25 tables

Step 4: Testing migrations apply in sequence...
PASSED: All migrations applied, 25 tables

Step 5: Comparing schemas (LATEST.sql vs migrations)...
PASSED: Table lists match

=== All Checks Passed ===
Database migrations are ready for fly.io deployment
```

**Validation checks:**

| Check | Description | Failure Condition |
|-------|-------------|-------------------|
| 1. LATEST.sql sync | Existing validate-migrations.sh | Tables/columns missing from LATEST.sql |
| 2. Migration numbering | Sequential 00, 01, 02... | Gaps or duplicates in numbering |
| 3. LATEST.sql validity | Apply to fresh SQLite DB | SQL syntax errors |
| 4. Migrations apply | Apply all migrations in order | Any migration fails |
| 5. Schema comparison | Compare resulting tables | Table list mismatch (warning only) |

**Note:** Run this before `fly deploy` to catch SQL syntax errors and migration issues.

---

### task fly:pre-deploy

Run all pre-deployment checks (environment variables + database migrations).

```bash
task fly:pre-deploy
```

**What it does:**
1. Runs `fly:check` (environment variable sync)
2. Runs `fly:db-check` (database migration validation)
3. Reports overall status

**Example output:**
```
=== Pre-Deployment Environment Sync Check ===
...
✅ All .env variables are synced to fly.io config

=== Pre-Deployment Database Migration Check ===
...
=== All Checks Passed ===
Database migrations are ready for fly.io deployment

=== All pre-deployment checks passed ===
Safe to run: fly deploy
```

**Recommended workflow:**
```bash
# Before every deployment
task fly:pre-deploy

# If all checks pass
fly deploy
```

---

### task embedding:server

Run the local embedding server (Python/FastAPI).

```bash
task embedding:server
```

**URL:** http://localhost:8001

**Requirements:**
- Python 3.8+
- sentence-transformers
- FastAPI
- uvicorn

**Install dependencies:**
```bash
pip install sentence-transformers fastapi uvicorn
```

**What it does:**
- Creates `scripts/embedding_server.py` if not exists
- Runs FastAPI server with sentence-transformers
- Uses `all-MiniLM-L6-v2` model (384 dimensions)

---

## Common Workflows

### Development (No RAG)

Simple development without RAG features:

```bash
# Terminal 1: Frontend
task dev:frontend

# Terminal 2: Backend
task dev:backend
```

---

### Development with RAG (Local Embeddings)

Full RAG development with local embedding server:

```bash
# Terminal 1: Embedding server
task embedding:server

# Terminal 2: Backend with RAG
task dev:backend:rag

# Terminal 3: Frontend
task dev:frontend
```

---

### Development with RAG (Mock Embeddings)

RAG development without embedding server or API keys:

```bash
# Single command - builds and runs with mock embeddings
task run:rag:mock

# Terminal 2 (optional): Frontend dev server
task dev:frontend
```

**Note:** Mock embeddings generate random vectors - useful for testing the RAG pipeline without API costs, but no semantic accuracy.

---

### Development with RAG (OpenRouter Embeddings) - Recommended

RAG development with production-quality embeddings:

```bash
# Set your API key first
export OPENROUTER_API_KEY=sk-or-v1-xxx

# Single command - builds and runs with OpenRouter embeddings
task run:rag

# Terminal 2 (optional): Frontend dev server
task dev:frontend
```

---

### Production Build

Build for production deployment:

```bash
# Without RAG
task build

# With RAG
task build:rag
```

---

### Re-index RAG After Changing Embedding Provider

When switching embedding providers:

```bash
task build:backend:rag

REINDEX_RAG=true \
EMBEDDING_PROVIDER=openrouter \
OPENROUTER_API_KEY=sk-or-v1-xxx \
LANCEDB_STORAGE_PROVIDER=local \
RAG_PIPELINE_ENABLED=true \
./build/memos --mode dev --data build/data
```

---

## Troubleshooting

### "task: command not found"

Task is not installed. See Prerequisites section above.

---

### CGO build errors

```
cgo: C compiler "gcc" not found
```

Install a C compiler:
```bash
# Ubuntu/Debian
sudo apt install build-essential

# macOS
xcode-select --install
```

---

### LanceDB library not found

```
cannot find -llancedb_go
```

Run setup:
```bash
task setup:lancedb
```

---

### Library path error at runtime

```
error while loading shared libraries: liblancedb_go.so
```

Set library path:
```bash
export LD_LIBRARY_PATH=$PWD/lib/linux_amd64:$LD_LIBRARY_PATH
./build/memos --mode dev
```

Or use `task dev:backend:rag` which sets this automatically.

---

### Embedding server connection refused

```
local embedding request failed: dial tcp 127.0.0.1:8001: connect: connection refused
```

Start the embedding server:
```bash
task embedding:server
```

Or use mock/openrouter embeddings instead:
```bash
EMBEDDING_PROVIDER=mock ./build/memos --mode dev
```

---

## Build Matrix

| Command | Frontend | Backend | RAG | LanceDB |
|---------|----------|---------|-----|---------|
| `task build` | Yes | Yes | No | No |
| `task build:frontend` | Yes | No | - | - |
| `task build:backend` | No | Yes | No | No |
| `task build:backend:rag` | No | Yes | Yes | Yes |
| `task build:rag` | Yes | Yes | Yes | Yes |

---

## Run Matrix

| Command | Builds First | RAG Enabled | Embedding Provider | Dimensions | API Key | Quality |
|---------|--------------|-------------|-------------------|------------|---------|---------|
| `task dev:backend` | No | No | - | - | No | - |
| `task dev:backend:rag` | No | Yes | local | 384 | No (needs server) | Good |
| `task run:rag` | Yes | Yes | openrouter | 1536 | Yes | Good |
| `task run:rag:l12` | Yes | Yes | openrouter (L12) | 384 | Yes | Better |
| `task run:rag:mock` | Yes | Yes | mock | 384 | No | None |

**Recommendations:**
- **Development/Production:** `task run:rag` (OpenRouter, requires API key)
- **Pipeline testing only:** `task run:rag:mock` (no semantic accuracy)

---

*Document Version: 1.5*
*Updated: 2026-01-31* - Added fly:db-check and fly:pre-deploy tasks for comprehensive pre-deployment validation
