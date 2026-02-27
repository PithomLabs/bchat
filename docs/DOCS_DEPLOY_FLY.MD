# Fly.io Deployment — Local Volume Variant (`Dockerfile.local.fly`)

**Status:** Live in Production  
**App:** `bchat0534`  
**Region:** `sjc` (San Jose)  
**Last Updated:** 2026-02-27  
**Dockerfile:** `Dockerfile.local.fly`  
**Related:**
- [DOCS_DEPLOYMENT.MD](./DOCS_DEPLOYMENT.MD) — S3/Tigrisdata variant (original deployment doc)
- [DOCS_DOCKERFILE.MD](./DOCS_DOCKERFILE.MD) — Docker implementation details
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) — Full environment variable reference

---

## Overview

This document covers the **local-volume variant** of the Fly.io deployment — where both SQLite and LanceDB data reside on a single persistent Fly volume at `/var/opt/memos`. This is simpler and cheaper than the S3/Tigrisdata variant, and suitable for single-machine deployments.

> [!NOTE]
> The original `DOCS_DEPLOYMENT.MD` describes the **S3/Tigrisdata variant** (`Dockerfile.s3.fly` / `Dockerfile.fly`). This document describes the **local volume variant** (`Dockerfile.local.fly`) which is the current active production deployment.

### Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         FLY.IO (bchat0534.fly.dev)                          │
│                         Dockerfile.local.fly                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────────────┐    ┌──────────────────────────────────────────────┐   │
│  │   memos binary   │    │        Fly Volume: memos_data                │   │
│  │  (Ubuntu 24.04)  │───▶│  /var/opt/memos/                            │   │
│  │  port :5230      │    │    memos.db         ← SQLite                │   │
│  └────────┬─────────┘    │    lancedb/         ← LanceDB vector store  │   │
│           │              └──────────────────────────────────────────────┘   │
│           │                                                                 │
│           │ CGO via liblancedb_go.so                                        │
│           │ (copied to /usr/local/lib/ at build time)                       │
│           │                                                                 │
│           ▼                                                                 │
│  ┌──────────────────────────────────────────────────┐                       │
│  │          OpenRouter API (External)               │                       │
│  │  Embeddings: openai/text-embedding-3-small       │                       │
│  │  LLM:       openai/gpt-oss-120b:free             │                       │
│  └──────────────────────────────────────────────────┘                       │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Local vs Fly.io Configuration Comparison

| Setting | Local (`task run:rag`) | Fly.io (`Dockerfile.local.fly`) |
|---------|------------------------|--------------------------------|
| `LANCEDB_STORAGE_PROVIDER` | `local` | `local` |
| `LANCEDB_LOCAL_PATH` | `./build/data/lancedb` | `/var/opt/memos/lancedb` |
| `EMBEDDING_PROVIDER` | `openrouter` | `openrouter` |
| `EMBEDDING_MODEL` | `openai/text-embedding-3-small` | `openai/text-embedding-3-small` |
| `EMBEDDING_BATCH_SIZE` | from `.env` | `10` |
| `RAG_PIPELINE_ENABLED` | `true` | `true` |
| `OPENROUTER_API_KEY` | `.env` file | `flyctl secrets set` |
| LanceDB natives | `lib/linux_amd64/` (local) | embedded in Docker image |
| Data directory | `./build/data` | `/var/opt/memos` (mounted volume) |

---

## Prerequisites

### Tools

| Tool | Notes |
|------|-------|
| `flyctl` | [Install](https://fly.io/docs/getting-started/installing-flyctl/) |
| `git` | For source control |

### Accounts

- **fly.io** account — [fly.io/app/sign-up](https://fly.io/app/sign-up)
- **OpenRouter** API key — [openrouter.ai/keys](https://openrouter.ai/keys)

### Required Local Files

These must be present before building. See the [LanceDB build docs](./DOCS_LANCEDB.MD) for how to obtain them:

```
bchat/
├── Dockerfile.local.fly         # Active Dockerfile (local volume variant)
├── fly.toml                     # Fly app configuration
├── scripts/entrypoint.sh        # Container entrypoint
├── lib/linux_amd64/             # LanceDB native libraries (NOT in git, ~542MB)
│   ├── liblancedb_go.a          # Static lib (363MB) – CGO link
│   └── liblancedb_go.so         # Shared lib (179MB) – runtime
├── include/                     # LanceDB C headers
│   └── lancedb.h
└── .dockerignore                # Must allow lib/ and include/ through (see below)
```

> [!IMPORTANT]
> `lib/linux_amd64/` is **not committed to git** (too large). It must be present locally before running `flyctl deploy`. See _Known Issues_ for what happens if it's missing from the Docker build context.

---

## Current `fly.toml`

```toml
app = 'bchat0534'
primary_region = 'sjc'

[build]
  dockerfile = 'Dockerfile.local.fly'

[env]
  EMBEDDING_BATCH_SIZE = '10'
  EMBEDDING_MODEL = 'openai/text-embedding-3-small'
  EMBEDDING_PROVIDER = 'openrouter'
  EMBEDDING_TIMEOUT = '10m'
  LANCEDB_LOCAL_PATH = '/var/opt/memos/lancedb'
  LANCEDB_STORAGE_PROVIDER = 'local'
  LLM_MODEL = 'openai/gpt-oss-120b:free'
  LLM_MODEL_REASONING = 'nvidia/nemotron-3-nano-30b-a3b:free'
  LLM_VERIFIER_ENABLED = 'false'
  MEMOS_MODE = 'prod'
  MEMOS_PORT = '5230'
  RAG_PIPELINE_ENABLED = 'true'
  TZ = 'UTC'

[[mounts]]
  source = 'memos_data'
  destination = '/var/opt/memos'

[http_service]
  internal_port = 5230
  force_https = true
  auto_stop_machines = 'stop'
  auto_start_machines = true
  min_machines_running = 0
  processes = ['app']

  [http_service.concurrency]
    type = 'connections'
    hard_limit = 25
    soft_limit = 20

[[vm]]
  memory = '1024mb'
  cpu_kind = 'shared'
  cpus = 1
  memory_mb = 1024
```

> [!NOTE]
> `auto_stop_machines = 'stop'` and `min_machines_running = 0` means the machine stops when idle, reducing cost. It wakes on first request (cold start ~2–3s).

---

## Required Secrets

Set once; persists across deployments. Setting a new secret triggers an automatic restart.

```bash
# Your OpenRouter API key (REQUIRED — RAG won't initialize without this)
flyctl secrets set OPENROUTER_API_KEY=sk-or-v1-...

# Encryption master key for tenant API key encryption (generate once, keep safe)
flyctl secrets set ENCRYPTION_MASTER_KEY=$(uuidgen)
```

### Verify secrets are in place

```bash
flyctl secrets list
```

**Expected output:**
```
NAME                    DIGEST                  STATUS
ENCRYPTION_MASTER_KEY   7286a5e204ac9781        Deployed
OPENROUTER_API_KEY      xxxxxxxxxxxxxxxx        Deployed
```

> [!CAUTION]
> If `OPENROUTER_API_KEY` is missing, the RAG pipeline silently initializes as `NoOpVectorDB` at startup. The app will run but reindexing will fail with `"RAG pipeline disabled (using NoOpVectorDB)"`. This is a silent failure — check `flyctl logs` for the actual error.

---

## Deploying

### First-Time Setup

```bash
# 1. Login
flyctl auth login

# 2. Create the Fly volume (1GB is enough for most deployments)
flyctl volumes create memos_data --size 1 --region sjc

# 3. Set required secrets
flyctl secrets set OPENROUTER_API_KEY=sk-or-v1-...
flyctl secrets set ENCRYPTION_MASTER_KEY=$(uuidgen)

# 4. Deploy
flyctl deploy
```

### Standard Redeployment

```bash
flyctl deploy
```

The Depot remote builder caches layers aggressively. Most redeploys only rebuild changed layers. Expect:
- **No code change:** ~30s (all cached, just pushes image)
- **Code change only:** ~2–3 min (recompiles Go, skips npm/lib layers)
- **`lib/` or `include/` change:** ~5–8 min (re-sends 542MB lib context + recompiles)
- **First deploy:** ~8–12 min

### Force Rebuild (Clear Depot Cache)

```bash
flyctl deploy --no-cache
```

Use when Depot's cache appears stale or you've updated `go.mod`, `package.json`, or native libraries.

---

## Dockerfile Explained (`Dockerfile.local.fly`)

The current Dockerfile uses three build stages:

```
Stage 1: frontend  (node:20-alpine)
  → npm ci && npm run release
  → Output: /server/router/frontend/dist/  ← NOTE: NOT /frontend-build/dist/

Stage 2: backend  (golang:1.24)
  → Install gcc, libc-dev  (CGO dependencies)
  → COPY lib/linux_amd64/ /usr/local/lib/lancedb/    ← LanceDB natives
  → COPY include/ /usr/local/include/lancedb/         ← LanceDB headers
  → go mod download
  → COPY . .
  → COPY --from=frontend /server/router/frontend/dist ./server/router/frontend/dist
      ↑ Needed for go:embed in server/router/frontend/frontend.go
  → go build -tags rag -ldflags="-s -w" -o memos ./bin/memos/main.go

Stage 3: runtime  (ubuntu:24.04)
  → Install ca-certificates, tzdata
  → COPY --from=backend /usr/local/lib/lancedb/liblancedb_go.so /usr/local/lib/
  → RUN ldconfig                                      ← Registers .so with linker
  → COPY --from=backend /backend-build/memos .
  → COPY scripts/entrypoint.sh .
  → COPY --from=frontend /widget-build/dist ./widget/dist
```

### Why `-tags rag`?

The `rag` build tag gates all LanceDB CGO code. Without it:
- `vectordb_lance.go` is excluded from compilation
- `vectordb_nolance.go` is included instead, which always returns an error
- The app compiles fine but `LANCEDB_STORAGE_PROVIDER=local` would fail at runtime

### Why `go:embed` Requires the Frontend Build First

`server/router/frontend/frontend.go` contains:
```go
//go:embed dist/*
var embeddedFiles embed.FS
```

This embeds the entire `dist/` directory into the Go binary at compile time. The `dist/` folder must exist **in the same directory as `frontend.go`** before `go build` runs. Since `npm run release` outputs to `../server/router/frontend/dist` (relative to `web/`), the Dockerfile must `COPY --from=frontend /server/router/frontend/dist ./server/router/frontend/dist` **before** `go build`.

---

## How the RAG Pipeline Initializes

Understanding this prevents silent failures:

```go
// service.go — NewService()
vectorDBConfig := NewVectorDBConfigFromEnv()
vectorDB, err := NewVectorDB(vectorDBConfig)
if err != nil {
    slog.Error("Failed to initialize vector database", "error", err)
    vectorDB = NewNoOpVectorDB()   // ← SILENT FALLBACK!
}
```

```go
// vectordb.go — NewVectorDB()
func NewVectorDB(config *VectorDBConfig) (VectorDB, error) {
    if !config.Enabled {                   // RAG_PIPELINE_ENABLED != "true"
        return NewNoOpVectorDB(), nil      // ← RAG disabled by env var
    }
    embedSvc, err := NewEmbeddingService(config.EmbeddingConfig)
    if err != nil {
        return nil, err                    // ← OPENROUTER_API_KEY missing → error here
    }
    // ...
    return newLanceVectorDB(config, embedSvc)
}
```

**Failure chain when `OPENROUTER_API_KEY` is missing:**
1. `NewEmbeddingService()` → `"OPENROUTER_API_KEY required for OpenRouter embedding provider"`
2. `NewVectorDB()` → returns error
3. `NewService()` → logs `slog.Error(...)`, falls back to `NoOpVectorDB`
4. App starts normally with RAG disabled
5. User triggers reindex → `"RAG pipeline disabled (using NoOpVectorDB)"`

**To diagnose RAG initialization failures:**
```bash
flyctl logs | grep -E "(Failed to initialize|RAG pipeline|LanceDB|Embedding)"
```

---

## Verifying RAG Is Running

### Check Logs After Deploy

```bash
flyctl logs | grep -E "(LanceDB|RAG|Embedding)"
```

**Success looks like:**
```
INFO Embedding service initialized provider=openrouter dimension=1536
INFO LanceDB vector database initialized uri=/var/opt/memos/lancedb provider=local tableName=kb_documents_1536 dimension=1536
INFO RAG pipeline enabled ...
```

**Failure (NoOp fallback) looks like:**
```
ERROR Failed to initialize vector database error="OPENROUTER_API_KEY required for OpenRouter embedding provider"
INFO RAG pipeline disabled, using no-op vector database
```

### SSH Diagnostics

```bash
flyctl ssh console

# Inside the container:

# Confirm .so is linked correctly
ldd /usr/local/memos/memos | grep lancedb
# Expected: liblancedb_go.so => /usr/local/lib/liblancedb_go.so

# Check env vars are set
env | grep -E "(RAG|LANCEDB|EMBEDDING|OPENROUTER)"

# Verify LanceDB data directory exists
ls -la /var/opt/memos/lancedb/

# Check SQLite is operational
sqlite3 /var/opt/memos/memos.db ".tables"
```

### Test Reindex from UI

1. Open the app → Agent Admin
2. Click **Rebuild Index**
3. **Success:** Shows progress, completes
4. **Failure:** `"RAG pipeline disabled (using NoOpVectorDB)"` → missing secret or init error

---

## Operational Procedures

### View Logs

```bash
# Stream live logs
flyctl logs

# Last N lines
flyctl logs -n 100

# Filter for specific component
flyctl logs | grep "RAG\|LanceDB\|Embedding"
```

### Restart Without Rebuild

```bash
# Applied when only secrets changed
flyctl apps restart
```

> Note: `flyctl secrets set` automatically triggers a restart. You don't need to restart manually after changing secrets.

### Scale Resources

```bash
# Check current allocation
flyctl scale show

# Increase memory if seeing OOM errors during indexing
flyctl scale memory 2048
```

The current `fly.toml` sets `memory = '1024mb'`. LanceDB indexing can peak above 512MB with large datasets.

### View Volume Info

```bash
flyctl volumes list
flyctl volumes show <volume-id>
```

### SSH into Container

```bash
flyctl ssh console
```

---

## Known Issues and Lessons Learned (2026-02-27 Deploy Session)

This section documents every build failure encountered and how they were resolved. This is essential context for agents and engineers doing future deploys.

---

### Issue 1: `lib/linux_amd64/` Not Found in Depot Build Context

**Error:**
```
ERROR [backend  4/10] COPY lib/linux_amd64/ /usr/local/lib/lancedb/
Error: failed to compute cache key: "/lib/linux_amd64": not found
```

**Root Cause:**  
The `.dockerignore` file — auto-generated by `flyctl launch` from `.gitignore` — contained:
```
**/lib
**/linux_amd64
**/*.so
```

These three entries explicitly excluded both the `lib/linux_amd64/` directory and all `.so` files from the Docker build context. Depot's remote builder never received the LanceDB native libraries.

**Symptom pattern:**
- First attempt: transfers 1.45GB (includes `lib/` in context) but still fails
- Second attempt: only transfers 448KB (Depot uses cached empty context, no `lib/`)
- Root cause: Depot's BuildKit caches the build context — once it caches a context without `lib/`, subsequent runs skip re-sending it

**Fix applied to `.dockerignore`:**
```diff
- **/lib
- **/linux_amd64
- **/*.so

+ # NOTE: lib/ and linux_amd64/ are intentionally NOT excluded here.
+ # They contain LanceDB native libraries (.so/.a) required for CGO compilation.
+ !lib/
+ !include/
+ # NOTE: *.so files inside lib/ are needed for LanceDB CGO. Use !lib/ negation above.
```

**Agent Guidance:** When `flyctl launch` auto-generates `.dockerignore`, it includes generic Go/Node ignores that will catch `lib/`, `libs/`, `*.so`, and `*.a` patterns. Always audit `.dockerignore` if your project uses CGO with native shared libraries. Use `!lib/` negation rules to explicitly allow directories.

---

### Issue 2: Wrong Frontend `dist/` Source Path in COPY

**Error:**
```
ERROR [backend  9/10] COPY --from=frontend /frontend-build/dist ./web/dist
Error: failed to compute cache key: "/frontend-build/dist": not found
```

**Root Cause:**  
The Dockerfiles assumed `npm run release` outputs to `/frontend-build/dist`. But `web/package.json` defines:
```json
"release": "vite build --mode release --outDir=../server/router/frontend/dist --emptyOutDir"
```

The `--outDir` is relative to `web/`, so from the Docker `frontend` stage (where `WORKDIR=/frontend-build`), the actual output path is:
- Docker CWD: `/frontend-build/`
- Relative output: `../server/router/frontend/dist`
- Absolute output: `/server/router/frontend/dist`

**Fix applied to all three Dockerfiles:**
```diff
- COPY --from=frontend /frontend-build/dist ./web/dist
+ COPY --from=frontend /server/router/frontend/dist ./server/router/frontend/dist
```

**Note:** This also fixed the destination. See Issue 3 below.

**Files updated:** `Dockerfile.local.fly`, `Dockerfile.fly`, `Dockerfile.s3.fly`

---

### Issue 3: Wrong Frontend `dist/` Destination — `go:embed` Failure

**Error:**
```
ERROR [backend 10/10] RUN go build -tags rag -ldflags="-s -w" -o memos ./bin/memos/main.go
server/router/frontend/frontend.go:19:12: pattern dist/*: no matching files found
```

**Root Cause:**  
`server/router/frontend/frontend.go` uses:
```go
//go:embed dist/*
var embeddedFiles embed.FS
```

The `go:embed` directive resolves paths relative to the **package directory** (`server/router/frontend/`). The `dist/` folder must exist at `server/router/frontend/dist/` for compilation to succeed.

Even after fixing Issue 2's source path, the destination was still wrong:
```dockerfile
# WRONG: copies to /backend-build/web/dist — go:embed can't find it
COPY --from=frontend /server/router/frontend/dist ./web/dist

# CORRECT: copies to /backend-build/server/router/frontend/dist — go:embed finds it
COPY --from=frontend /server/router/frontend/dist ./server/router/frontend/dist
```

**Agent Guidance:** Whenever a Go package uses `//go:embed <subdir>/*`, the `subdir` must be present as an actual directory relative to the `.go` file containing the directive. In multi-stage builds, you must `COPY` the embedded content **before** `go build`, and the destination path must match the package's location in the source tree.

---

### Issue 4: RAG Pipeline Disabled at Runtime — Silent `NoOpVectorDB` Fallback

**Symptom:** Deployment succeeds, app runs, but reindexing returns:
```
Reindex failed: RAG pipeline disabled (using NoOpVectorDB)
```

**Root Cause:**  
`OPENROUTER_API_KEY` was not set as a Fly secret. The embedding service requires it to initialize:

```go
// embedding.go
if config.OpenRouterAPIKey == "" {
    return nil, fmt.Errorf("OPENROUTER_API_KEY required for OpenRouter embedding provider")
}
```

Without it, `NewVectorDB()` returns an error, and `service.go` silently falls back to `NoOpVectorDB` — the app starts normally with no visible error to the user.

**Key observation from `flyctl secrets list`:**
```
NAME                    DIGEST                  STATUS
ENCRYPTION_MASTER_KEY   7286a5e204ac9781        Deployed
```
`OPENROUTER_API_KEY` was absent.

**Fix:**
```bash
flyctl secrets set OPENROUTER_API_KEY=sk-or-v1-...your-key...
```
This triggers an automatic restart. RAG initialized correctly on next startup.

**Agent Guidance:** This is a silent failure by design — the app constructor catches the error and degrades gracefully. To surface it, always check `flyctl logs | grep "Failed to initialize"` after deployment. If RAG is expected but reindex fails, check secrets first.

---

## Troubleshooting Reference

### RAG Is Disabled (NoOpVectorDB)

**Diagnosis:**
```bash
flyctl logs | grep -E "(Failed to initialize|NoOp|RAG pipeline)"
```

**Causes and fixes:**

| Cause | Symptom in Logs | Fix |
|-------|-----------------|-----|
| `OPENROUTER_API_KEY` not set | `"OPENROUTER_API_KEY required..."` | `flyctl secrets set OPENROUTER_API_KEY=...` |
| `RAG_PIPELINE_ENABLED` != `"true"` | `"RAG pipeline disabled..."` | Add/fix in `fly.toml [env]` |
| LanceDB `.so` not found at runtime | `"error loading shared library"` | Rebuild — check `.dockerignore` allows `lib/` |
| Built without `-tags rag` | `"LanceDB storage requires RAG build"` | Check Dockerfile build command |

---

### `lib/linux_amd64/` Not Found in Build

**Symptom:**
```
ERROR [backend N/10] COPY lib/linux_amd64/ /usr/local/lib/lancedb/
"/lib/linux_amd64": not found
```

**Fix:**
Check `.dockerignore` for patterns that exclude `lib/`:
```bash
grep -n "lib\|\.so\|\.a" .dockerignore
```
Remove or negate any pattern that blocks `lib/linux_amd64/`. Add:
```
!lib/
!include/
```

---

### `go:embed` Fails at Build

**Symptom:**
```
pattern dist/*: no matching files found
```

**Fix:** Ensure `COPY --from=frontend` places the dist folder **in the same directory** as the `.go` file with `//go:embed`:

```dockerfile
# frontend.go is at: server/router/frontend/frontend.go
# Therefore dist/ must be at: server/router/frontend/dist/
COPY --from=frontend /server/router/frontend/dist ./server/router/frontend/dist
```

---

### Build Context Too Large / Slow

**Symptom:** `transferring context: 1.45GB` — slow uploads on every deploy.

**Cause:** `web/node_modules/` (450MB) or other large directories not excluded.

**Fix:** Ensure `.dockerignore` excludes unnecessary directories:
```
**/node_modules
**/dist
build/
docs/
mastra-main/
lancedb-go-main/
```

> [!IMPORTANT]
> The current `.dockerignore` excludes `**/dist` globally. This is intentional — the frontend `dist/` is built inside Docker, not sent from the host. The Go `server/router/frontend/dist/` is populated via `COPY --from=frontend` in the Dockerfile.

---

### Shared Library Not Found at Runtime

**Symptom:**
```
./memos: error while loading shared libraries: liblancedb_go.so: cannot open shared object file
```

**Fix:** Ensure the runtime stage copies the `.so` and runs `ldconfig`:
```dockerfile
COPY --from=backend /usr/local/lib/lancedb/liblancedb_go.so /usr/local/lib/
RUN ldconfig
```

Verify inside the container:
```bash
flyctl ssh console
ldconfig -p | grep lancedb
# Expected: liblancedb_go.so (libc6,x86-64) => /usr/local/lib/liblancedb_go.so
```

---

### Volume Not Mounting

**Symptom:** `Error: volume 'memos_data' not found for region 'sjc'`

**Fix:**
```bash
flyctl volumes list
# If missing:
flyctl volumes create memos_data --size 1 --region sjc
```

---

### Out of Memory During Reindex

**Symptom:** Machine crashes mid-reindex; `dmesg` shows OOM.

**Fix:**
```bash
flyctl scale memory 2048
```

Or update `fly.toml`:
```toml
[[vm]]
  memory = '2048mb'
```

---

## Deployment Checklist

### Before `flyctl deploy`

- [ ] `lib/linux_amd64/liblancedb_go.so` exists locally
- [ ] `lib/linux_amd64/liblancedb_go.a` exists locally
- [ ] `include/lancedb.h` exists locally
- [ ] `.dockerignore` has `!lib/` and `!include/` negations
- [ ] `flyctl secrets list` shows `OPENROUTER_API_KEY` and `ENCRYPTION_MASTER_KEY`
- [ ] `fly.toml` `[env]` has `RAG_PIPELINE_ENABLED = 'true'`
- [ ] Volume exists: `flyctl volumes list` shows `memos_data` in `sjc`

### After `flyctl deploy`

- [ ] `flyctl status` shows machine as `started`
- [ ] `flyctl logs | grep "LanceDB"` shows successful init
- [ ] `flyctl logs | grep "Failed to initialize"` returns nothing
- [ ] Reindex from UI succeeds
- [ ] Chat query returns RAG-backed results

---

## Available Dockerfiles

The project has three Fly.io Dockerfiles:

| Dockerfile | LanceDB Storage | Use Case |
|-----------|-----------------|----------|
| `Dockerfile.local.fly` | Local volume (`/var/opt/memos/lancedb`) | **Current prod** — simple, no S3 |
| `Dockerfile.fly` | Tigrisdata S3 | Multi-machine or geo-distributed |
| `Dockerfile.s3.fly` | Tigrisdata S3 | Same as above, slight variant |

To switch to S3 variant, update `fly.toml`:
```toml
[build]
  dockerfile = 'Dockerfile.fly'
```
And set required S3 secrets — see [DOCS_DEPLOYMENT.MD](./DOCS_DEPLOYMENT.MD) for full S3 setup.

---

## `.dockerignore` Reference

The current `.dockerignore` is auto-generated by `flyctl launch` from `.gitignore` entries. Key rules that affect the LanceDB build:

**MUST be absent or negated (will break CGO build if present):**
```
**/lib          # ← Would exclude lib/linux_amd64/
**/linux_amd64  # ← Would exclude lib/linux_amd64/
**/*.so         # ← Would exclude liblancedb_go.so
**/*.a          # ← Would exclude liblancedb_go.a
```

**MUST be present (explicitly allow LanceDB files through):**
```
!lib/
!include/
```

To audit quickly:
```bash
grep -n "lib\|\.so\|\.a\|linux_amd64" .dockerignore
```

---

## See Also

- [DOCS_DEPLOYMENT.MD](./DOCS_DEPLOYMENT.MD) — S3/Tigrisdata deployment variant
- [DOCS_DOCKERFILE.MD](./DOCS_DOCKERFILE.MD) — Dockerfile deep dive
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) — All environment variables
- [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD) — LanceDB RAG implementation

---

*Document Version: 1.0*  
*Created: 2026-02-27 — Initial deployment and troubleshooting session*
