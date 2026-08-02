# bchat

Multi-tenant AI chat agent platform. Each tenant gets a configurable agent powered by LLMs, with optional RAG for knowledge-grounded responses.

Built on [Memos](https://github.com/usememos/memos) with [OpenRouter](https://openrouter.ai/) for LLM access and [LanceDB](https://lancedb.com/) for vector search.

---

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [Architecture](#architecture)
4. [Configuration](#configuration)
5. [Development](#development)
6. [API Reference](#api-reference)
7. [Deployment](#deployment)
8. [Contributing](#contributing)
9. [Documentation](#documentation)

---

## Overview

bchat is a multi-tenant AI chat agent platform. A single deployment powers unlimited independent AI chat agents — one per business — with no code changes between tenants.

### Key Principles

| Principle | Description |
|-----------|-------------|
| **General-purpose** | No tenant-specific logic in code. All customization via config files. |
| **Configuration-driven** | Agent behavior defined by KB.MD, POLICY.MD, SCRIPT.MD per tenant. |
| **Multi-tenant isolation** | Each tenant has isolated data, configs, and chat sessions. |

### Technology Stack

| Layer | Technology |
|-------|------------|
| Backend | Go 1.25.0, Echo framework |
| Database | SQLite (default), PostgreSQL |
| Frontend | React 18, TypeScript, MobX, Vite |
| UI | Joy UI (@mui/joy) |
| LLM | OpenRouter API |
| Vector DB | LanceDB (optional, for RAG) |
| Embeddings | OpenRouter text-embedding-3-small, local, or mock |

### Business Guide

For business owners and operators (KB/POLICY writing, widget deployment, lead capture), see [`docs/DOCS_HOWTO_BIZ.MD`](docs/DOCS_HOWTO_BIZ.MD).

---

## Quick Start

### Prerequisites

- Go 1.25.0+
- Node.js 18+ (for frontend)
- [Task](https://taskfile.dev/) (task runner)
- OpenRouter API key (`sk-or-v1-...`)

### Local Development (No RAG)

```bash
# Clone and install
git clone <repo-url> && cd bchat
task setup

# Set API key
export OPENROUTER_API_KEY=sk-or-v1-your-key-here

# Build and run
task build
./build/memos --mode dev --data build/data
```

App starts at `http://localhost:8081`. Admin UI at `/agent-admin`.

### With RAG

RAG = agent searches KB.MD for relevant context before answering.

```bash
task build:rag
task run:rag

# Or with mock embeddings (no API key needed for testing)
task run:rag:mock
```

### Development Mode (Hot Reload)

```bash
# Terminal 1: Backend
task dev:backend

# Terminal 2: Frontend
task dev:frontend
```

Frontend: `http://localhost:5173` | Backend API: `http://localhost:8081/api/v1/`

---

## Architecture

### Directory Structure

```
bchat/
├── bin/memos/                    # Application entry point
├── build/                        # Build output
│   └── data/                     # Runtime data (SQLite DB, LanceDB indexes)
├── docs/                         # Documentation
├── server/
│   └── router/api/v1/
│       ├── agent/                # Agent API handlers and services
│       │   ├── handlers.go       # HTTP request handlers
│       │   ├── service.go        # Business logic, LLM integration
│       │   ├── parser.go         # KB/Policy/Script parsing
│       │   ├── vectordb.go       # Vector database interface
│       │   ├── vectordb_lance.go # LanceDB implementation
│       │   ├── embedding.go      # Embedding providers
│       │   ├── chunker.go        # Document chunking
│       │   ├── observer.go       # Observational memory
│       │   ├── simulation.go     # Agent simulation
│       │   ├── analysis.go       # Transcript analysis
│       │   ├── verifier.go       # LLM response verification
│       │   ├── sanitizer.go      # Output sanitization
│       │   └── prompts/          # Prompt templates
│       ├── auth.go               # JWT claims, token generation
│       ├── auth_service.go       # Auth endpoints, tenant selection
│       ├── acl.go                # gRPC auth interceptor
│       ├── tenant_context.go     # Tenant context helpers
│       ├── memo_service.go       # Memo CRUD with tenant scoping
│       ├── ticket_service.go     # Ticket CRUD with tenant scoping
│       └── v1.go                 # Route registration
├── store/
│   ├── agent.go                  # Data types and store interface
│   ├── driver.go                 # Database driver interface
│   ├── db/sqlite/                # SQLite implementation
│   ├── db/postgres/              # PostgreSQL implementation
│   └── migration/sqlite/         # Database migrations
├── web/
│   ├── src/
│   │   ├── pages/                # React page components
│   │   ├── store/v2/             # MobX stores
│   │   └── locales/              # i18n translations
│   └── dist/                     # Built frontend assets
├── widget/                       # Embeddable chat widget
├── plugin/                       # Cron, webhook, storage plugins
└── Taskfile.yml                  # Build and run commands
```

### Request Flow

```
Client Request
     │
     ▼
Echo Router (/api/v1/agent/:slug/...)
     │
     ▼
Agent Handler (handlers.go)
     │
     ├─► Permission Check (RBAC)
     │
     ▼
Agent Service (service.go)
     │
     ├─► Parse KB/Policy/Script
     ├─► [Optional] RAG Search (vectordb.go)
     ├─► Build System Prompt
     └─► Call OpenRouter LLM
     │
     ▼
SSE Response Stream
```

### Tenant Isolation

Every API request is scoped to a single tenant via JWT claims:

```
JWT (tenant_id) → Auth Middleware → Context → Handler → DB Query (WHERE tenant_id = ?)
```

- **gRPC handlers**: Extract tenant from Go context via `GetTenantIDFromContext(ctx)`
- **Echo handlers**: Extract tenant from echo context via `getTenantFromContext(c)`
- **Superusers**: Bypass tenant checks for cross-tenant access

### Deployment Profiles

bchat is a Fly.io-native application framework where the **database is a deployment profile, not an architecture**. A profile bundles exactly four things: a driver flag, a migration directory, a DSN source, and a Fly config.

```
                  bchat
                    │
             store.Driver
                    │
        ┌───────────┼────────────┐
        │           │            │
     SQLite     PostgreSQL   Cockroach
        │           │            │
     Local      Fly+Neon     Fly+Cockroach
        │           │            │
       Taskfile operator API (intent, not infrastructure)
```

- **CockroachDB is the reference profile**: `--driver=cockroach` + `store/migration/cockroach/` + `COCKROACH_DSN` + `fly_cockroach.toml`. Nothing else. The shared `store/db/postgres/` package satisfies `store.Driver` for both PostgreSQL and CockroachDB.
- Four divergence seams — the only places driver-specific logic may live: DDL dialect (migration files), transaction semantics (retry wrapper), connection protocol (none today), capability availability (vector provider). No conditional-on-driver logic inside individual SQL methods.
- **Parallel Fly apps `bchat-pg` / `bchat-crdb` are a migration and demo strategy, not the intended permanent deployment topology.** They prove the same Taskfile drives both profiles (`task deploy:postgres` / `task deploy:cockroach`) so a tenant can move between them without code changes; a permanent deployment picks one profile.
- Future evolution: `task deploy:postgres` / `task deploy:cockroach` become `task deploy PROFILE=postgres` / `deploy PROFILE=cockroach` (not implemented yet).
- See [`docs/docs_flyio_cockroach_deploy.md`](docs/docs_flyio_cockroach_deploy.md) for the CockroachDB deploy + demo runbook.

---

## Configuration

### Required Environment Variables

```bash
OPENROUTER_API_KEY=sk-or-v1-xxx    # OpenRouter API key (required)
```

### LLM Configuration

```bash
LLM_MODEL=openai/gpt-4o-mini       # Default chat model
LLM_MODEL_REASONING=google/gemini-2.5-pro  # Content generation model
```

### RAG Configuration

```bash
RAG_PIPELINE_ENABLED=true          # Enable RAG pipeline
EMBEDDING_PROVIDER=openrouter      # openrouter|local|mock
EMBEDDING_MODEL=text-embedding-3-small
EMBEDDING_BATCH_SIZE=10
EMBEDDING_TIMEOUT=180s
LANCEDB_STORAGE_PROVIDER=local     # local|s3
LANCEDB_LOCAL_PATH=build/data/lancedb
HYBRID_SEARCH_ENABLED=true         # Vector (70%) + BM25 (30%)
```

### Observational Memory

```bash
OM_ENABLED=true                    # Long-term memory
OM_OBSERVER_TOKEN_THRESHOLD=30000
OM_TOKEN_THRESHOLD=2000
```

### Startup Flags

```bash
FORCE_REINDEX_ON_STARTUP=true      # Re-index all content at boot
LLM_VERIFIER_ENABLED=true          # LLM response verification
```

### Configuration Priority

```
1. Tenant Config (Agent Admin UI) → Highest
         ↓ (if empty)
2. Environment Variable → Fallback
         ↓ (if empty)
3. Hardcoded Default → Lowest
```

See [`docs/DOCS_ENV_VAR.MD`](docs/DOCS_ENV_VAR.MD) for complete reference.

---

## Development

### Common Build Tasks

| Command | Description |
|---------|-------------|
| `task setup` | Install all dependencies |
| `task build` | Build frontend + backend |
| `task build:frontend` | Frontend only |
| `task build:backend` | Backend only |
| `task build:rag` | Full build with RAG (LanceDB) |
| `task run` | Run dev server |
| `task run:rag` | Run with RAG enabled |
| `task run:rag:mock` | Run with mock embeddings (no API key) |
| `task dev:backend` | Backend with hot reload |
| `task dev:frontend` | Frontend with hot reload |
| `task validate:schema` | Validate database schema |

### Backend (Go)

**Adding a new API endpoint:**

1. Define types in [`store/agent.go`](store/agent.go)
2. Add interface methods in [`store/driver.go`](store/driver.go)
3. Implement in `store/db/sqlite/agent.go` (+ stubs for postgres/mysql)
4. Add handler in [`server/router/api/v1/agent/handlers.go`](server/router/api/v1/agent/handlers.go)
5. Register route in [`server/router/api/v1/v1.go`](server/router/api/v1/v1.go)

**Adding a new database table:**

1. Create migration in `store/migration/sqlite/NN__description.sql`
2. Add Go types in [`store/agent.go`](store/agent.go)
3. Add interface in [`store/driver.go`](store/driver.go)
4. Implement in `store/db/sqlite/agent.go`
5. Migrations auto-apply on startup

**Code conventions:**

- Use `slog` for logging: `slog.Error("message", "error", err)`
- Return errors with context: `fmt.Errorf("failed to X: %w", err)`
- Pointer receivers for methods
- JSON tags: `snake_case`

### Frontend (React + TypeScript)

**Adding a new feature:**

1. Add types and state to store (e.g., `web/src/store/v2/agentAdmin.ts`)
2. Add fetch methods with `runInAction` for async updates
3. Add UI component in `web/src/pages/`
4. Add translations to `web/src/locales/en.json`

**Code conventions:**

- MobX `makeAutoObservable` for stores
- `runInAction` for all async state updates
- `observer` HOC for reactive components
- Joy UI components from `@mui/joy`

### Database Migrations

Location: `store/migration/sqlite/`

Naming: `NN__snake_case_description.sql`

```sql
CREATE TABLE IF NOT EXISTS my_table (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenant(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_my_table_tenant ON my_table(tenant_id);
```

---

## API Reference

### Public Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/agent/:slug/chat` | Send chat message |
| `GET` | `/api/v1/agent/:slug/chat/stream` | SSE response stream |
| `GET` | `/widget/:slug/embed.js` | JavaScript widget embed |
| `GET` | `/widget/:slug/iframe` | iframe widget embed |

### Auth Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/auth/tenants` | List tenants for multi-tenant user |
| `POST` | `/api/v1/auth/select-tenant` | Select tenant, return JWT with tenant_id |

### Admin Endpoints (Authenticated)

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| `GET/POST` | `/api/v1/agent/tenants` | `tenant:admin` | List/Create tenants |
| `GET/PUT/DELETE` | `/api/v1/agent/:slug` | `tenant:admin` | CRUD tenant |
| `POST` | `/api/v1/agent/:slug/files` | `files:upload` | Upload KB/Policy/Script |
| `POST` | `/api/v1/agent/:slug/reindex` | `api:config` | Rebuild RAG index |
| `POST` | `/api/v1/agent/:slug/simulate` | `chat:test` | Run simulation |
| `GET` | `/api/v1/agent/:slug/simulations` | `chat:test` | List simulations |
| `POST` | `/api/v1/agent/:slug/generate-kb` | `tenant:admin` | Auto-generate KB.MD |
| `POST` | `/api/v1/agent/:slug/rag/search` | `api:config` | Test RAG search |
| `GET` | `/api/v1/agent/:slug/leads` | `tenant:admin` | List leads |
| `GET` | `/api/v1/agent/:slug/leads/export` | `tenant:admin` | Export leads CSV |

### Permission System (RBAC)

| Permission | Description |
|------------|-------------|
| `tenant:admin` | Full tenant management |
| `tenant:read` | View tenant configuration |
| `api:config` | Configure LLM settings, rebuild index |
| `chat:test` | Run simulations, view history |
| `chat:logs` | View real chat session logs |
| `files:upload` | Upload KB/Policy/Script files |

---

## Deployment

### Docker

```bash
docker build -t bchat .
docker run -e OPENROUTER_API_KEY=sk-or-v1-xxx -p 8081:8081 bchat
```

### Fly.io

```bash
export FLY_API_TOKEN=...
fly launch
fly deploy
```

Pre-deployment checks:

```bash
task fly:check        # Validate env chain
task fly:db-check     # Validate migrations
task fly:pre-deploy   # Run all checks
```

See [`docs/DOCS_DEPLOY_FLY.MD`](docs/DOCS_DEPLOY_FLY.MD) for full deployment guide.

---

## Contributing

Contributions welcome! See [`AGENTS.md`](AGENTS.md) for the development guide, code conventions, and patterns for adding new features.

---

## Documentation

### Core

| Document | Description |
|----------|-------------|
| [`docs/DOCS_README.MD`](docs/DOCS_README.MD) | Comprehensive project documentation |
| [`docs/DOCS_ENV_VAR.MD`](docs/DOCS_ENV_VAR.MD) | Environment variables reference |
| [`docs/DOCS_TASKFILE.MD`](docs/DOCS_TASKFILE.MD) | Build commands reference |
| [`AGENTS.md`](AGENTS.md) | AI agent development guide |

### Architecture

| Document | Description |
|----------|-------------|
| [`docs/DOCS_AGENT_ARCHITECTURE.MD`](docs/DOCS_AGENT_ARCHITECTURE.MD) | Configuration-driven design |
| [`docs/DOCS_CHAT_DESIGN.MD`](docs/DOCS_CHAT_DESIGN.MD) | Chat agent design spec |
| [`docs/DOCS_RAG_PIPELINE.MD`](docs/DOCS_RAG_PIPELINE.MD) | RAG pipeline architecture |
| [`docs/DOCS_HYBRID_SEARCH.MD`](docs/DOCS_HYBRID_SEARCH.MD) | Hybrid search (vector + BM25) |

### Features

| Document | Description |
|----------|-------------|
| [`docs/DOCS_SIMULATION.MD`](docs/DOCS_SIMULATION.MD) | Agent simulation feature |
| [`docs/DOCS_WIDGET.MD`](docs/DOCS_WIDGET.MD) | Chat widget integration |
| [`docs/DOCS_HOWTO_BIZ.MD`](docs/DOCS_HOWTO_BIZ.MD) | Business owner guide |

### Operations

| Document | Description |
|----------|-------------|
| [`docs/DOCS_DEPLOY_FLY.MD`](docs/DOCS_DEPLOY_FLY.MD) | Fly.io deployment guide |
| [`docs/DOCS_ROLLOUT.MD`](docs/DOCS_ROLLOUT.MD) | Production rollout guide |
| [`docs/CHANGELOG.MD`](docs/CHANGELOG.MD) | Project changelog |


### License

MIT License

Copyright (c) 2026 Pithom Labs

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.