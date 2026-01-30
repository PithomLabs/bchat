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
| `task fly:check` | Check .env vars are synced to fly.toml/secrets before deploy |

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

Check that `.env` variables are synced to fly.io configuration before deployment.

```bash
task fly:check
```

**What it does:**
- Lists all variables in your local `.env`
- Lists variables in `fly.toml [env]` section
- Lists `fly secrets`
- Reports any `.env` variables missing from fly.io config

**Example output:**
```
=== Pre-Deployment Environment Sync Check ===

📁 Local .env variables:
   LLM_MODEL
   OPENROUTER_API_KEY

📄 fly.toml [env] variables:
   EMBEDDING_MODEL
   RAG_PIPELINE_ENABLED

🔐 fly secrets:
   OPENROUTER_API_KEY

=== Sync Status ===
❌ MISSING: LLM_MODEL (not in fly.toml or fly secrets)

⚠️  1 variable(s) missing from fly.io config
```

**Note:** Run this before `fly deploy` to catch missing environment variables.

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

*Document Version: 1.4*
*Updated: 2026-01-31* - Added fly:check task for pre-deployment env sync
