# Bchat software engineering tutorial outline

**Session ID:** ses_0b34e8a2dffeCqgBwm3b7L7lrN
**Created:** 7/10/2026, 11:42:06 PM
**Updated:** 7/11/2026, 1:25:08 AM

---

## User

analyze the bchat codebase, let us make this question and answer until we agree on a plan, note that I want a holistic software engineering and business acumen perspectives in one, not a typical software engineering course  twrite an outline as comprehensive reference of the bchat repo as a learning tutorial, the aim is for total newbies to have a firm grip of software engineering enabling critical thinking, sound analysis and design from end to end throughout the entire software development life cycle from design to deployment to fly.io and solid maintenance engineering of system and database with neon postgres database, include local development with sqlite, database migration to postgres, include all the robust software engineering practices for full stack software development 

---

## Assistant (Plan · MiMo V2.5 Free · 177.2s)

_Thinking:_

The user wants me to analyze the bchat codebase and create a comprehensive outline for a learning tutorial. They want it to cover both software engineering and business acumen perspectives, targeting total newbies. The tutorial should cover:

1. Software engineering fundamentals
2. Critical thinking, sound analysis, and design
3. End-to-end software development lifecycle
4. Design to deployment to fly.io
5. Maintenance engineering of system and database
6. Neon Postgres database
7. Local development with SQLite
8. Database migration to Postgres
9. Robust software engineering practices for full stack development

This is a READ-ONLY mode, so I can only analyze and plan. Let me start by exploring the codebase to understand its structure and components.

I'll analyze the bchat codebase to create a comprehensive tutorial outline. Let me start by exploring the repository structure and key components.

**Tool: task**

**Input:**
```json
{
  "description": "Explore bchat codebase structure",
  "prompt": "Explore the bchat codebase thoroughly to understand:\n1. Overall project structure and architecture\n2. Key technologies used (Go, React, databases, etc.)\n3. Build system and development workflow\n4. Database layer (SQLite and Postgres support)\n5. Deployment configuration (Docker, fly.io)\n6. Testing infrastructure\n7. Documentation structure\n\nFocus on:\n- Root-level files (go.mod, Taskfile.yml, docker-compose, fly.toml)\n- Server directory structure\n- Store/database layer\n- Web/frontend structure\n- Configuration files\n- Build and deployment scripts\n\nReturn a comprehensive summary of the codebase architecture, key files, and how components connect.",
  "subagent_type": "explore"
}
```

**Output:**
```
<task id="ses_0b34e5314ffe5kNvT8gsxEnoTV" state="completed">
<task_result>
Now I have a thorough understanding of the entire codebase. Here is a comprehensive summary:

---

# bchat Codebase Architecture Summary

## 1. Project Identity

**bchat** is a multi-tenant AI chat agent platform built on top of the **Memos** open-source note-taking application. The Go module is still named `github.com/usememos/memos`. Each tenant (business) can define their own knowledge base (KB.MD), agent policy (POLICY.MD), and conversation flow script (SCRIPT.MD) without code changes. The agent is designed to be **general-purpose and configuration-driven** -- tenant-specific behavior lives entirely in configuration files.

---

## 2. Root-Level Structure

| Path | Purpose |
|---|---|
| `/home/chaschel/Documents/go/bchat/go.mod` | Go 1.25 module (`github.com/usememos/memos`) |
| `/home/chaschel/Documents/go/bchat/Taskfile.yml` | Primary build system (Task v3) |
| `/home/chaschel/Documents/go/bchat/Taskfile_pg.yml` | Postgres-specific Taskfile variant |
| `/home/chaschel/Documents/go/bchat/fly.toml` | Fly.io deployment (Postgres/Neon variant, app `bchat-pg`) |
| `/home/chaschel/Documents/go/bchat/fly_prod.toml` | Production Fly.io config |
| `/home/chaschel/Documents/go/bchat/fly.s3.toml` | Fly.io config with S3 storage |
| `/home/chaschel/Documents/go/bchat/Dockerfile.fly` | Multi-stage Docker build (SQLite + LanceDB) |
| `/home/chaschel/Documents/go/bchat/Dockerfile.pg.fly` | Multi-stage Docker build (Postgres + LanceDB + Ubuntu runtime) |
| `/home/chaschel/Documents/go/bchat/Dockerfile.s3.fly` | Docker build variant for S3 |
| `/home/chaschel/Documents/go/bchat/.env.example` | Environment variable template |
| `/home/chaschel/Documents/go/bchat/.dockerignore` | Docker context exclusions |
| `/home/chaschel/Documents/go/bchat/AGENTS.md` | Comprehensive AI agent development guide |

---

## 3. Technology Stack

| Layer | Technology |
|---|---|
| Language (backend) | Go 1.25 |
| HTTP framework | Echo v4 (`github.com/labstack/echo/v4`) |
| gRPC | `google.golang.org/grpc` v1.72 with gRPC-Gateway v2 |
| CLI framework | Cobra + Viper |
| Database (default) | SQLite via `modernc.org/sqlite` (pure Go) |
| Database (alternative) | PostgreSQL via `pgx/v5` (sole driver, no `lib/pq`) |
| Database (legacy) | MySQL via `go-sql-driver/mysql` |
| Vector DB | LanceDB (`github.com/lancedb/lancedb-go` v0.1.2) via CGO |
| LLM Provider | OpenRouter API (`github.com/revrost/go-openrouter`) |
| Auth | JWT (`golang-jwt/jwt/v5`) + HMAC nonce (bridge) |
| Frontend framework | React 18 + TypeScript + Vite |
| UI components | Joy UI (`@mui/joy` 5.0.0-beta.51) + Tailwind CSS |
| State management | MobX 6 (`mobx-react-lite`) |
| Routing (frontend) | React Router v7 |
| Widget | Standalone Vite-built embeddable chat widget |
| Task runner | Task v3 (`Taskfile.yml`) |
| Deployment | Fly.io (Docker multi-stage) |
| Object storage | S3/Tigris (LanceDB indexes in production) |

---

## 4. Entry Point and Server Bootstrap

**Entry point:** `/home/chaschel/Documents/go/bchat/bin/memos/main.go`

Boot sequence:
1. Cobra/Viper parses CLI flags and environment variables (prefix `MEMOS_`), plus explicit binds for `OPENROUTER_API_KEY`, `LLM_MODEL`, `ENCRYPTION_MASTER_KEY`
2. `profile.Profile` is created and validated (data directory, DSN construction)
3. `db.NewDBDriver(profile)` creates the appropriate store driver (SQLite/Postgres/MySQL)
4. `store.New(dbDriver, profile)` creates the Store; `storeInstance.Migrate(ctx)` runs migrations
5. `server.NewServer(ctx, profile, storeInstance)` creates Echo + gRPC servers
6. Signal handling for graceful shutdown (SIGINT/SIGTERM)

**Server** (`/home/chaschel/Documents/go/bchat/server/server.go`):
- Uses `cmux` to multiplex HTTP (Echo) and gRPC on the same port
- Echo serves: frontend static assets, RSS, REST API (gRPC-Gateway), agent REST endpoints, healthz
- gRPC serves: workspace, auth, user, memo, resource, shortcut, inbox, activity, webhook, markdown, IDP services
- Background runners: S3 presign runner, memo payload rebuild runner
- Profiler (pprof) gated behind dev mode or `MEMOS_ENABLE_PPROF=true`

---

## 5. Directory Architecture

```
bchat/
  bin/memos/main.go          -- Application entry point
  server/
    server.go                -- Server lifecycle (Echo + gRPC + cmux)
    profiler/                -- pprof + memory monitoring
    runner/                  -- Background job runners (s3presign, memopayload)
    service/beads.go         -- Beads service
    router/
      api/v1/
        v1.go                -- APIV1Service registration (gRPC + REST gateway)
        acl.go               -- Auth interceptor (JWT extraction, context setting)
        tenant_context.go    -- Tenant ID context extraction + RBAC filtering
        auth.go              -- JWT claims (HS256, tenant_id field)
        auth_service.go      -- Login/signup/tenant selection
        memo_service.go      -- Memo CRUD with tenant scoping
        ticket_service.go    -- Ticket CRUD with tenant scoping
        user_service.go      -- User management
        agent/               -- Core agent subsystem (49 files)
          handlers.go        -- HTTP handlers (REST endpoints)
          service.go         -- Business logic (4659 lines)
          parser.go          -- KB/Policy/Script MD file parsing
          chunker.go         -- Document chunking for RAG
          embedding.go       -- Embedding providers (openrouter, local, mock)
          vectordb.go        -- Vector database interface
          vectordb_lance.go  -- LanceDB implementation
          vectordb_pool.go   -- Connection pooling
          observer.go        -- Observational Memory core
          observer_buffer.go -- Message buffering for OM
          om_config.go       -- OM configuration
          simulation.go      -- Agent simulation (human-sim testing)
          analysis.go        -- Transcript analysis + scoring
          verifier.go        -- LLM response verification
          sanitizer.go       -- Output sanitization
          fusion_engine.go   -- Hybrid search (vector + BM25)
          scoring.go         -- Heuristic scoring
          learning.go        -- Learning memory system
          lead_extraction.go -- Lead capture from chat
          lead_llm.go        -- LLM-powered lead extraction
          integrations.go    -- External integrations (webhook, Twilio)
          bridge_*.go        -- Bridge delivery (human handoff)
          permissions.go     -- RBAC permission checks
          playground.go      -- Playground demo seeding
          prompts/           -- LLM prompt templates (observer.txt, reflector.txt)
      frontend/              -- Static asset serving (built React app)
      rss/                   -- RSS feed generation
  store/
    store.go                 -- Store wrapper
    driver.go                -- Driver interface (283 lines, 60+ methods)
    agent.go                 -- Agent data types (1283 lines) + AgentStore interface
    bridge.go                -- Bridge delivery types
    memo.go, user.go, ...    -- Core data types
    db/
      db.go                  -- Driver factory (SQLite/Postgres/MySQL switch)
      sqlite/sqlite.go       -- SQLite driver (modernc.org/sqlite, WAL, FK)
      postgres/postgres.go   -- Postgres driver (pgx/v5, simple_protocol)
      mysql/                 -- MySQL driver
    migration/
      sqlite/                -- 31 versioned migration dirs + LATEST.sql
      postgres/              -- 13 versioned migration dirs + LATEST.sql
    test/                    -- 24 test files (schema validation, CRUD, bridge, RBAC)
  web/                       -- Frontend application
    package.json             -- npm dependencies
    src/
      main.tsx               -- React entry point
      App.tsx                -- Root component
      pages/                 -- 25 page components
        AgentAdmin.tsx       -- Tenant admin panel
        AgentSimulation.tsx  -- Agent simulation UI
        Chat.tsx             -- Chat interface
        RagStats.tsx         -- RAG statistics
        InternalAgent.tsx    -- Internal agent chat
        Playground.tsx       -- Agent playground
      store/v2/              -- 14 MobX stores
        agentAdmin.ts        -- Tenant admin state
        agentChat.ts         -- Chat state
        agentSimulation.ts   -- Simulation state
      locales/               -- i18n translations
      router/index.tsx       -- React Router config
    vendor/                  -- Vendored @usememos/mui component library
  widget/                    -- Embeddable chat widget (standalone Vite build)
  plugin/
    cron/                    -- Cron job plugin
    filter/                  -- CEL expression filter
    httpgetter/              -- HTTP client
    idp/                     -- Identity providers
    storage/                 -- S3 storage plugin
    webhook/                 -- Webhook plugin
  internal/
    profile/profile.go       -- Server configuration profile
    crypto/                  -- AES-256 encryption service
    bridgeworker/            -- Bridge background worker
    version/                 -- Version info
    util/                    -- Utilities
  scripts/                   -- Build/deployment scripts
    entrypoint.sh            -- Docker entrypoint (gosu privilege drop, _FILE secrets)
    docker-compose.postgres.yml -- Local Postgres for dev
    validate-env-chain.sh    -- Pre-deploy env validation
    validate-db-migrations.sh -- Migration integrity check
    validate-migrations.sh   -- LATEST.sql sync check
  proto/store/               -- Protobuf definitions for store models
  docs/                      -- 124 documentation files
  bugs/                      -- Bug tracking plans (003, 011, 015, 021, 024, 030)
  deployment/                -- Deployment scripts
  lib/                       -- LanceDB native shared libraries (per platform)
  include/                   -- LanceDB C headers
```

---

## 6. Database Layer

### Driver Architecture
- **Interface:** `store.Driver` (`/home/chaschel/Documents/go/bchat/store/driver.go`) -- 60+ methods covering all entity CRUD
- **Factory:** `db.NewDBDriver()` (`/home/chaschel/Documents/go/bchat/store/db/db.go`) -- switches on `profile.Driver`
- **Implementations:** SQLite (primary), PostgreSQL, MySQL (legacy)

### SQLite (`/home/chaschel/Documents/go/bchat/store/db/sqlite/sqlite.go`)
- Uses `modernc.org/sqlite` (pure Go, no CGO needed)
- WAL journal mode, foreign keys enabled, 10s busy timeout
- DSN format: `memos_<mode>.db` in data directory

### PostgreSQL (`/home/chaschel/Documents/go/bchat/store/db/postgres/postgres.go`)
- Uses `pgx/v5` as sole driver (no `lib/pq`)
- Auto-appends `default_query_exec_mode=simple_protocol` for Neon pgbouncer compatibility
- Connection pool: 10 max open, 5 max idle, 5min max lifetime

### Migrations
- **SQLite:** 31 versioned directories (`0.2/` through `0.31/`) + `LATEST.sql` (1067 lines)
- **PostgreSQL:** 13 versioned directories (`0.19/` through `0.31/`) + `LATEST.sql`
- Naming: `NN__snake_case_description.sql`
- `store/migrator.go` handles version tracking via `migration_history` table

### Schema (LATEST.sql -- 1067 lines)
Core tables: `user`, `user_setting`, `memo`, `memo_relation`, `resource`, `activity`, `inbox`, `webhook`, `reaction`, `workspace_setting`, `identity_provider`, `tickets`, `notifications`

Agent tables: `agent_tenants`, `agent_audiences`, `agent_services`, `agent_exclusions`, `agent_coverage`, `agent_faqs`, `agent_safety_protocols`, `agent_kb_sections`, `agent_intents`, `agent_rules`, `agent_sessions`, `agent_rate_limits`, `agent_source_files`, `agent_tenant_scripts`, `agent_simulations`, `agent_simulation_transcripts`, `agent_script_analysis`, `agent_qa_pairs`, `agent_transcripts`, `agent_workflows`

RBAC tables: `tenant_role_templates`, `user_tenant_permission`, `tenant_config`, `system_secret`

Analytics tables: `agent_analysis_results`, `agent_learning_memory`, `agent_compliance_audits`, `agent_scoring_config`

Observational Memory: `agent_observations`

Bridge/Human handoff: `bridge_external_sessions`, `bridge_handoffs`, `bridge_auth_keys`, `bridge_auth_nonces`, `bridge_handoff_replies`, `bridge_reply_outbox`, `agent_messages`, `agent_leads`

Integrations: `agent_integrations`, `agent_events`

Reindex: `agent_reindex_checkpoints`

All agent tables use `ON DELETE CASCADE` with `agent_tenants(id)`.

---

## 7. Agent Subsystem (Core Business Logic)

The agent system lives in `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/` (49 files).

### Service (`service.go` -- 4659 lines)
- Orchestrates all agent operations
- Manages: parser, memory sessions, config cache, encryption, verifier, vector DB, chunker, observer buffer
- LLM integration via OpenRouter API
- RAG pipeline: chunk -> embed -> vector search -> top-k -> prompt injection

### Handlers (`handlers.go` -- 6308 lines)
REST endpoints registered in `v1.go`:
- `GET/POST /api/v1/agent/tenants` -- List/create tenants
- `GET/PUT/DELETE /api/v1/agent/:slug` -- Tenant CRUD
- `POST /api/v1/agent/:slug/files` -- Upload KB/Policy/Script
- `POST /api/v1/agent/:slug/chat` -- Send chat message (public)
- `GET /api/v1/agent/:slug/chat/stream` -- SSE streaming
- `POST /api/v1/agent/:slug/reindex` -- Rebuild RAG index
- `POST /api/v1/agent/:slug/simulate` -- Run simulation
- `GET /api/v1/agent/:slug/simulations` -- List simulations
- `GET /api/v1/agent/:slug/validate` -- Validate tenant

### Parser (`parser.go`)
Parses annotated markdown files using `<!-- @type: value -->` format into structured data:
- KB annotations: `@service`, `@faq`, `@exclusion`, `@coverage`, `@safety`, `@section`
- Policy annotations: `@identity`, `@intent`, `@rule`, `@thresholds`

### RAG Pipeline
- **Chunker** (`chunker.go`): ~500 token chunks with overlap
- **Embedding** (`embedding.go`): openrouter (1536d), local (384d), mock (1536d)
- **Vector DB** (`vectordb.go`): Interface; `vectordb_lance.go` implements LanceDB
- **Fusion Engine** (`fusion_engine.go`): Hybrid search (vector 70% + BM25 30%)
- **LanceDB Storage**: memory, local filesystem, or S3 (Tigrisdata)
- Build tag `rag` gates LanceDB CGO compilation

### Observational Memory (`observer.go`)
Long-term memory via compressed conversation observations:
- Observer compresses messages into observation log
- Reflector compresses observations further
- Configurable token thresholds

### Simulation (`simulation.go`)
Agent simulation for testing:
- Human simulator persona
- Automated conversation turns
- Analysis and scoring

### Bridge System
Human handoff mechanism for external chat sessions:
- `bridge_external_sessions` -- Session tracking
- `bridge_handoffs` -- Routing mode (handoff_queued, human_active, closed)
- `bridge_handoff_replies` + `bridge_reply_outbox` -- Durable reply delivery
- HMAC authentication via `bridge_auth_keys` + nonces
- Background worker in `internal/bridgeworker/`

---

## 8. Build System (Taskfile.yml)

Key tasks:
| Task | Description |
|---|---|
| `task setup` | Install all dependencies (npm + go mod) |
| `task build` | Build frontend + backend (no RAG) |
| `task build:rag` | Build with LanceDB RAG support (CGO_ENABLED=1, `-tags rag`) |
| `task build:all` | Build everything (frontend + backend + widget) |
| `task run` | Run dev server (sources .env) |
| `task run:rag` | Run with RAG enabled |
| `task run:rag:l12` | Run with L12 embeddings |
| `task build:backend:rag` | Backend only with LanceDB |
| `task build:widget` | Build embeddable chat widget |
| `task validate:migrations` | Validate LATEST.sql sync |
| `task validate:schema` | Validate DB schema + CRUD |
| `task fly:pre-deploy` | Pre-deployment checks (env + DB) |

The RAG build requires:
1. LanceDB native libs (`task setup:lancedb` downloads from GitHub)
2. CGO_ENABLED=1 with CFLAGS/LDFLAGS pointing to LanceDB headers/libs
3. Build tag `rag`

---

## 9. Deployment

### Fly.io
- **Primary config:** `fly.toml` (app `bchat-pg`, region `sjc`)
  - Postgres driver, Neon serverless database
  - S3/Tigris for LanceDB storage
  - OpenRouter for embeddings and LLM
  - 1024MB RAM, shared CPU
  - Health check at `/healthz`
- **Dockerfile.pg.fly:** 3-stage build (Node frontend -> Go backend with CGO -> Ubuntu 24.04 runtime)
  - Includes supercronic for cron jobs
  - Non-root user (`memos`) with gosu privilege dropping
  - `_FILE` suffix support for Docker/K8s secrets

### Docker (local)
- `scripts/docker-compose.postgres.yml` -- Local Postgres 16 Alpine container (port 5432, user/pass/db: `bchat`)

### Entrypoint (`scripts/entrypoint.sh`)
- Processes `_FILE` suffix secrets (Docker/K8s compatible)
- Creates non-root `memos` user, fixes volume ownership
- Launches supercronic if `CRON_TOKEN` is set
- Drops privileges via `gosu`

---

## 10. Frontend

**Stack:** React 18 + TypeScript + Vite + MobX + Joy UI + Tailwind CSS

**Key pages** (`/home/chaschel/Documents/go/bchat/web/src/pages/`):
- `AgentAdmin.tsx` -- Tenant administration
- `AgentSimulation.tsx` -- Agent testing simulation
- `Chat.tsx` -- Chat interface
- `InternalAgent.tsx` -- Internal agent chat
- `RagStats.tsx` -- RAG statistics dashboard
- `Playground.tsx` -- Agent playground
- `Tickets.tsx` / `TicketDetail.tsx` -- Ticket management

**State management** (`/home/chaschel/Documents/go/bchat/web/src/store/v2/`):
14 MobX stores: `agentAdmin.ts`, `agentChat.ts`, `agentSimulation.ts`, `chat.ts`, `dialog.ts`, `memo.ts`, `memoFilter.ts`, `playground.ts`, `ragStats.ts`, `resource.ts`, `user.ts`, `view.ts`, `workspace.ts`

**Build output:** `web/dist/` is copied to `server/router/frontend/dist/` and served by the Go backend.

---

## 11. Testing

**Store tests** (`/home/chaschel/Documents/go/bchat/store/test/`):
24 test files covering:
- Schema validation (`schema_validation_test.go`)
- Agent CRUD (`agent_crud_test.go`)
- Bridge system (`bridge_test.go`, `bridge_auth_test.go`, `bridge_settlement_test.go`, `bridge_postgres_cascade_test.go`)
- Agent leads (`agent_lead_test.go`, `agent_lead_postgres_test.go`)
- Core entities (memo, user, resource, ticket, etc.)
- RBAC (`role_template_test.go`)
- Migration (`migrator_test.go`)

**Agent tests** (in `server/router/api/v1/agent/`):
- `handlers_test.go`
- `bridge_*_test.go` (foundation, middleware, runtime, endpoints, delivery)
- `contact_state_test.go`
- `embedding_test.go`
- `integrations_test.go`
- `lead_extraction_test.go`
- `observer_test.go`
- `parser_settings_test.go`
- `rag_sanitizer_test.go`
- `role_template_handler_test.go`
- `service_reindex_test.go`
- `vectordb_lance_test.go`, `vectordb_lance_retry_test.go`, `vectordb_test.go`

Run schema validation: `task validate:schema`
Run full tests: `go test ./store/test/...`

---

## 12. Security Architecture

### Tenant Isolation
Every request is scoped to a single tenant via JWT claims:
- `ClaimsMessage.TenantID` in JWT (`/home/chaschel/Documents/go/bchat/server/router/api/v1/auth.go`)
- Context extraction: `getTenantFromContext(c)` for Echo, `GetTenantIDFromContext(ctx)` for gRPC
- SQL safety net: `ApplyTenantFilter()` / `ApplyTicketTenantFilter()` in `/home/chaschel/Documents/go/bchat/server/router/api/v1/tenant_context.go`
- Scoped admin support: `deriveTenantIDsForScopedAdmin()` resolves `AllowedTenantIDs` GUIDs to tenant IDs

### RBAC
- Permission model with templates: viewer, tester, analyst, editor, tenant_admin
- Permissions: `tenant:admin`, `tenant:read`, `chat:test`, `chat:logs`, `files:upload`, `api:config`
- `user_tenant_permission` table with role template inheritance
- Superuser bypass with explicit check pattern

### Encryption
- AES-256 encryption for tenant API keys via `internal/crypto/`
- Master key (`ENCRYPTION_MASTER_KEY`) + salt stored in `system_secret` table
- Encrypted API keys in `tenant_config.openrouter_api_key_encrypted`

### Bridge Authentication
- HMAC-SHA256 with per-tenant key pairs
- Nonce replay protection via `bridge_auth_nonces`

---

## 13. Documentation (`/home/chaschel/Documents/go/bchat/docs/`)

124 documentation files organized by topic:
- **Architecture:** `DOCS_README.MD`, `DOCS_AGENT_ARCHITECTURE.MD`, `DOCS_AGENT_CODE_AUDIT.MD`
- **RAG:** `DOCS_RAG_PIPELINE.MD`, `DOCS_HYBRID_SEARCH.MD`, `DOCS_EMBEDDING.MD`, `DOCS_CHUNKING.MD`, `DOCS_LANCEDB*.MD`
- **Observational Memory:** `DOCS_OM*.MD`
- **Deployment:** `DOCS_DEPLOYMENT.MD`, `DOCS_FLY.MD`, `DOCS_DOCKERFILE.MD`
- **Database:** `DOCS_DATABASE_MIGRATION.MD`, `DOCS_TASKFILE_PG.MD`
- **Chat Design:** `DOCS_CHAT_DESIGN*.MD` (6 versions)
- **RBAC:** `DOCS_RBAC.MD`, `DOCS_RBAC_2.MD`
- **Simulation:** `DOCS_SIMULATION*.MD` (7 files)
- **Templates:** `KB_TEMPLATE.MD`, `POLICY_TEMPLATE.MD`, `SCRIPT.MD`

---

## 14. Key Configuration Files

| File | Purpose |
|---|---|
| `/home/chaschel/Documents/go/bchat/.env.example` | Environment variable template |
| `/home/chaschel/Documents/go/bchat/fly.toml` | Fly.io production config |
| `/home/chaschel/Documents/go/bchat/web/vite.config.ts` | Vite build config |
| `/home/chaschel/Documents/go/bchat/web/tsconfig.json` | TypeScript config |
| `/home/chaschel/Documents/go/bchat/web/tailwind.config.js` | Tailwind CSS config |
| `/home/chaschel/Documents/go/bchat/widget/vite.config.ts` | Widget Vite config |
| `/home/chaschel/Documents/go/bchat/scripts/docker-compose.postgres.yml` | Local Postgres dev |

---

## 15. Key Architectural Patterns

1. **Multi-tenant isolation:** Every data access flows through context-based tenant extraction + SQL-level filtering as a safety net.

2. **Dual HTTP/gRPC:** `cmux` multiplexes Echo (REST/Static) and gRPC on a single port. gRPC-Gateway translates REST to gRPC.

3. **Driver pattern:** `store.Driver` interface with SQLite/Postgres/MySQL implementations, selected at runtime.

4. **Configuration-driven agent:** All tenant-specific behavior lives in KB.MD/POLICY.MD/SCRIPT.MD files -- the agent code is entirely generic.

5. **RAG with LanceDB:** Optional build tag `rag` gates CGO-dependent LanceDB integration. Supports hybrid search (vector + BM25).

6. **Observational Memory:** Compresses conversation history into observations for long-term agent memory.

7. **Bridge delivery:** Outbox pattern for human handoff with claim-based message processing.
</task_result>
</task>
```

---

## Assistant (Plan · MiMo V2.5 Free · 108.1s)

_Thinking:_

Now I have a comprehensive understanding of the bchat codebase. Let me also explore the documentation files to understand what's already documented and what gaps exist.

**Tool: task**

**Input:**
```json
{
  "description": "Explore bchat documentation files",
  "prompt": "Explore the bchat documentation files in the docs/ directory to understand:\n1. What documentation already exists\n2. What topics are covered\n3. What gaps exist in the documentation\n4. How the docs are organized\n\nFocus on:\n- List all documentation files\n- Read key docs (DOCS_README.MD, DOCS_AGENT_ARCHITECTURE.MD, DOCS_DEPLOYMENT.MD, DOCS_DATABASE_MIGRATION.MD)\n- Identify any tutorials or getting-started guides\n- Note any business/strategy documentation\n\nReturn a summary of existing documentation and identify gaps for a comprehensive tutorial.",
  "subagent_type": "explore"
}
```

**Output:**
```
<task id="ses_0b34bc30effeeRQsdkd2F7Pulv" state="completed">
<task_result>
---

## Summary: Documentation Exploration Complete

### What We Did

I systematically explored the entire `docs/` directory of the **bchat** project - a multi-tenant AI chat agent platform. I read and analyzed **20+ key documentation files** covering:

| Category | Files Analyzed | Coverage |
|---|---|---|
| **Architecture & Design** | DOCS_README.MD, DOCS_AGENT_ARCHITECTURE.MD | Core system design |
| **Setup & Build** | DOCS_TASKFILE.MD, README.MD | Development workflow |
| **Environment Config** | DOCS_ENV_VAR.MD | 200+ lines of env vars |
| **Deployment** | DOCS_DEPLOYMENT.MD, DOCS_DOCKERFILE.MD, DOCS_FLY.MD | Production deployment |
| **Database** | DOCS_DATABASE_MIGRATION.MD | Migration system |
| **RAG Pipeline** | DOCS_RAG_PIPELINE.MD, DOCS_LANCEDB*.MD (5 phases), DOCS_HYBRID_SEARCH.MD | RAG implementation |
| **Widget/Embedding** | DOCS_WIDGET.MD | External chat widget |
| **Business/Strategy** | DOCS_HOWTO_BIZ.md, docs_biz_guide.md, BIZ_QUICK_START_CHECKLIST.MD | Business owner guides |
| **Templates** | KB.MD, POLICY.MD, GUIDELINES.MD, SCRIPT_OPUS13.MD, PROMPTS.MD | Tenant config templates |
| **Simulation/Testing** | DOCS_SIMULATION.MD, DOCS_SIMULATION_2.MD | Agent testing |
| **Rollout** | DOCS_ROLLOUT.MD | Hybrid search rollout |
| **Bug Tracking** | DOCS_BUGS.MD, DOCS_BUGS_QA.MD, DOCS_BUGS_REBUILD_INDEX.MD | Known issues |
| **Pending Work** | DOCS_PENDING.MD | TODO items, limitations |
| **Orchestrator** | plans/orchestrator-framework-design.md | Agent orchestration framework |
| **Examples** | templates/examples/ | bible, livio, rizal datasets |

### Key Findings

1. **Documentation is massive** (~100+ MD files) but **flat** in `docs/` with 4 subdirs
2. **Two near-duplicate business guides** exist: `DOCS_HOWTO_BIZ.md` and `docs_biz_guide.md`
3. **No step-by-step tutorial** exists - this is the biggest gap
4. **No CONTRIBUTING.md** for open-source contributors
5. **No single architecture diagram** - ASCII art scattered across docs
6. **No security/auth documentation** - auth system referenced but no dedicated doc
7. **No cost/pricing calculator** - API costs mentioned in many docs but no consolidated guide
8. **Templates are well-organized** with real-world examples (bible, livio, rizal datasets)

### Identified Documentation Gaps

| Gap | Description | Priority |
|---|---|---|
| **No step-by-step tutorial** | No end-to-end "build your first tenant" tutorial | **High** |
| **No developer quickstart** | DOCS_README.MD has setup but no "hello world" flow | **High** |
| **No API tutorial** | DOCS_ENDPOINTS.MD exists but no interactive walkthrough | **Medium** |
| **No CONTRIBUTING.md** | No contributor guide for open-source participation | **Medium** |
| **No architecture diagram** | No single visual diagram of system architecture | **Low** |
| **No troubleshooting guide** | DOCS_PENDING.MD has fixes but no dedicated troubleshooting doc | **Medium** |
| **No security/auth doc** | Auth system referenced but no dedicated security doc | **Medium** |
| **No cost/pricing calculator** | API costs mentioned but no consolidated guide | **Low** |

---

**What would you like me to do next?**

Based on the exploration, I'm ready to:
1. **Create a comprehensive tutorial** (end-to-end "Build Your First AI Chat Agent" guide)
2. **Create a developer quickstart** (setup → first API call → embed widget)
3. **Create a troubleshooting guide** (consolidate known issues and fixes)
4. **Create a security guide** (auth system, tenant isolation, API key management)
5. **Something else** you have in mind

What would you like me to build?
</task_result>
</task>
```

---

## Assistant (Plan · MiMo V2.5 Free · 8.3s)

_Thinking:_

Now I have a comprehensive understanding of the bchat codebase and documentation. Let me now create a comprehensive tutorial outline that covers both software engineering and business acumen perspectives. The user wants:

1. Total newbies to have a firm grip of software engineering
2. Critical thinking, sound analysis and design
3. End-to-end software development lifecycle
4. Design to deployment to fly.io
5. Maintenance engineering of system and database
6. Neon Postgres database
7. Local development with SQLite
8. Database migration to Postgres
9. Robust software engineering practices for full stack software development

Let me create a comprehensive outline that covers all these aspects.

Now I have a thorough understanding of the bchat codebase. Let me create a comprehensive tutorial outline that covers both software engineering and business perspectives.

**Tool: todowrite**

**Input:**
```json
{
  "todos": [
    {
      "content": "Analyze bchat codebase structure and architecture",
      "status": "completed",
      "priority": "high"
    },
    {
      "content": "Explore existing documentation and identify gaps",
      "status": "completed",
      "priority": "high"
    },
    {
      "content": "Create comprehensive tutorial outline covering SDLC, business acumen, and engineering practices",
      "status": "in_progress",
      "priority": "high"
    },
    {
      "content": "Review outline with user and incorporate feedback",
      "status": "pending",
      "priority": "high"
    }
  ]
}
```

**Output:**
```
[
  {
    "content": "Analyze bchat codebase structure and architecture",
    "status": "completed",
    "priority": "high"
  },
  {
    "content": "Explore existing documentation and identify gaps",
    "status": "completed",
    "priority": "high"
  },
  {
    "content": "Create comprehensive tutorial outline covering SDLC, business acumen, and engineering practices",
    "status": "in_progress",
    "priority": "high"
  },
  {
    "content": "Review outline with user and incorporate feedback",
    "status": "pending",
    "priority": "high"
  }
]
```

---

## Assistant (Plan · MiMo V2.5 Free · 74.7s)

_Thinking:_

Let me now create a comprehensive tutorial outline. This needs to be holistic, covering both software engineering and business acumen perspectives. Let me think about what makes this different from a typical software engineering course:

1. Business context - why this product exists, who it serves, how it makes money
2. Design thinking - how to think about problems before coding
3. Architecture decisions - why certain technologies were chosen
4. Implementation details - how the code works
5. Testing strategies - how to verify the system works
6. Deployment - how to get to production
7. Maintenance - how to keep it running
8. Database management - schema design, migrations, optimization

Let me structure this as a comprehensive outline.

Here's the comprehensive tutorial outline. Let me present it for your review:

---

# BCHAT TUTORIAL OUTLINE: From Zero to Production

## A Holistic Guide to Full-Stack Software Engineering & Business Acumen

---

## PART I: FOUNDATIONS — Understanding the Problem Space

### Chapter 1: The Business Case — Why Bchat Exists

1.1 **The Market Problem**
- Small businesses need AI customer service but can't build it
- Enterprise solutions are too expensive and complex
- The gap: affordable, configurable AI agents for any vertical

1.2 **The Business Model**
- Multi-tenant SaaS: one codebase, many customers
- Revenue streams: per-agent pricing, usage-based LLM costs
- Cost structure: LLM API calls (OpenRouter), infrastructure (Fly.io + Neon)

1.3 **The Product Vision**
- Configuration over code: KB.MD, POLICY.MD, SCRIPT.MD files
- No-code agent customization for non-technical users
- Embeddable widget for any website

1.4 **Critical Thinking Exercise**
- Why multi-tenant instead of single-tenant?
- Why SQLite for dev, Postgres for prod?
- Why OpenRouter instead of direct OpenAI/Anthropic?

### Chapter 2: Technology Choices — Why These Tools

2.1 **Backend: Go + Echo**
- Why Go: performance, simplicity, deployment (single binary)
- Why Echo: lightweight, fast, good middleware ecosystem
- Tradeoffs: vs. Python (Django/FastAPI), vs. Node.js (Express/Fastify)

2.2 **Frontend: React + TypeScript + Vite**
- Why React: ecosystem, component model, hiring pool
- Why TypeScript: type safety, refactoring confidence
- Why Vite: fast dev server, simple config
- Why Joy UI: design system, accessibility, MUI ecosystem

2.3 **Database: SQLite → PostgreSQL**
- SQLite: zero-config, perfect for development
- PostgreSQL: production-grade, JSONB, full-text search
- Migration strategy: same SQL with driver abstraction

2.4 **AI/ML Stack**
- OpenRouter: access to multiple LLM providers
- LanceDB: vector database for RAG
- Embedding providers: openrouter, local, mock

2.5 **Deployment: Fly.io + Neon**
- Fly.io: edge deployment, Docker-native
- Neon: serverless Postgres, auto-scaling
- Why not AWS/GCP directly: simplicity, cost for small teams

2.6 **Critical Thinking Exercise**
- What would change if we used MySQL instead of Postgres?
- Why not use a managed vector database (Pinecone, Weaviate)?
- How would costs scale with 100 vs. 10,000 tenants?

---

## PART II: ARCHITECTURE — Designing the System

### Chapter 3: System Architecture — The Big Picture

3.1 **High-Level Architecture Diagram**
```
┌─────────────────────────────────────────────────────────┐
│                    CLIENT LAYER                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │ Web App  │  │ Widget   │  │ REST API │              │
│  │ (React)  │  │ (Vite)   │  │ (curl)   │              │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘              │
│       │              │              │                    │
│  ┌────▼──────────────▼──────────────▼────┐              │
│  │           API Gateway (Echo)          │              │
│  │    REST + gRPC-Gateway + Static       │              │
│  └────────────────┬──────────────────────┘              │
│                   │                                     │
│  ┌────────────────▼──────────────────────┐              │
│  │           Agent Service               │              │
│  │   (Business Logic + LLM Integration)  │              │
│  └───┬────────────┬────────────┬─────────┘              │
│      │            │            │                        │
│  ┌───▼───┐   ┌───▼───┐   ┌───▼───┐                    │
│  │Parser │   │  RAG  │   │  OM   │                    │
│  │       │   │Pipeline│   │Memory │                    │
│  └───────┘   └───────┘   └───────┘                    │
│                                                        │
│  ┌──────────────────────────────────────────┐          │
│  │           Data Layer                     │          │
│  │  SQLite (dev) → PostgreSQL (prod)        │          │
│  │  LanceDB (vectors) → S3 (storage)        │          │
│  └──────────────────────────────────────────┘          │
└─────────────────────────────────────────────────────────┘
```

3.2 **Request Flow: Chat Message Lifecycle**
- Step-by-step: HTTP → Auth → Tenant Context → Service → LLM → Response
- Code references: `handlers.go:100` → `service.go:200` → `parser.go`

3.3 **Multi-Tenant Isolation Architecture**
- JWT claims with tenant_id
- Context propagation (Echo + gRPC)
- SQL-level safety net (ApplyTenantFilter)
- RBAC permission system

3.4 **Critical Thinking Exercise**
- What happens if tenant isolation fails? (Security implications)
- How would you add a new tenant-scoped entity?
- Why use both context-based and SQL-level filtering?

### Chapter 4: Database Design — Schema & Migrations

4.1 **Schema Design Principles**
- Normalization vs. performance tradeoffs
- JSONB for flexible data (agent configs)
- Foreign keys with ON DELETE CASCADE
- Indexing strategy

4.2 **Core Schema Walkthrough**
```sql
-- Tenant isolation: every table has tenant_id
CREATE TABLE agent_tenants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL UNIQUE,
    company_name TEXT NOT NULL,
    -- ...
);

-- Source files: KB, Policy, Script
CREATE TABLE agent_source_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    file_type TEXT NOT NULL,  -- 'kb', 'policy', 'script'
    content TEXT NOT NULL,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

-- Sessions: conversation tracking
CREATE TABLE agent_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    -- ...
);
```

4.3 **Migration Strategy**
- Versioned migrations: `0.2/`, `0.3/`, ..., `0.31/`
- LATEST.sql: cumulative schema for fresh installs
- `migration_history` table: tracks applied migrations
- SQLite vs. PostgreSQL migration differences

4.4 **Database Abstraction Layer**
- `store.Driver` interface: 60+ methods
- SQLite implementation (`modernc.org/sqlite`)
- PostgreSQL implementation (`pgx/v5`)
- How to add a new database driver

4.5 **Critical Thinking Exercise**
- Why use `ON DELETE CASCADE` for tenant-scoped data?
- What are the tradeoffs of JSONB vs. separate tables?
- How would you handle a breaking schema change in production?

---

## PART III: DEVELOPMENT — Building the System

### Chapter 5: Local Development Setup

5.1 **Prerequisites & Environment**
- Go 1.25+, Node.js 18+, Task runner
- Environment variables (.env file)
- IDE setup (VS Code + Go extension)

5.2 **First Build**
```bash
# Clone and setup
git clone <repo>
cd bchat
task setup

# Build without RAG (simpler)
task build
task run

# Build with RAG (LanceDB)
task setup:lancedb
task build:rag
task run:rag
```

5.3 **Development Workflow**
- Backend: `task dev:backend` (hot reload with Air)
- Frontend: `task dev:frontend` (Vite HMR)
- Database: SQLite auto-creates on first run

5.4 **Code Organization Deep Dive**
- Entry point: `bin/memos/main.go`
- Server bootstrap: `server/server.go`
- Route registration: `server/router/api/v1/v1.go`
- Agent handlers: `server/router/api/v1/agent/handlers.go`

5.5 **Critical Thinking Exercise**
- Why use Task instead of Make?
- How does the Cobra/Viper configuration system work?
- What's the benefit of cmux (HTTP + gRPC on same port)?

### Chapter 6: Agent System — The Core Business Logic

6.1 **Configuration-Driven Architecture**
- KB.MD: Knowledge base (facts, services, FAQs)
- POLICY.MD: Agent behavior (identity, rules, intents)
- SCRIPT.MD: Conversation flow (stages, transitions)

6.2 **Parser System**
- Annotation format: `<!-- @type: value, key: value -->`
- KB annotations: @service, @faq, @exclusion, @coverage, @safety
- Policy annotations: @identity, @intent, @rule, @thresholds
- Code walkthrough: `parser.go`

6.3 **RAG Pipeline**
```
User Query → Embed → Vector Search → Top-K Results → LLM Prompt
                    ↓
              BM25 Keyword Search (30%)
                    ↓
              Hybrid Fusion (70% vector + 30% BM25)
```

6.4 **Observational Memory**
- Message buffering → Observer → Observation log
- Reflector: compresses observations into insights
- Token thresholds: when to trigger compression

6.5 **Critical Thinking Exercise**
- How would you add a new annotation type to the parser?
- What are the tradeoffs of hybrid search vs. pure vector search?
- How does observational memory affect conversation quality?

### Chapter 7: Frontend Development

7.1 **React Architecture**
- Component hierarchy: Pages → Components → Widgets
- State management: MobX stores (14 stores)
- Routing: React Router v7

7.2 **Key Pages**
- `AgentAdmin.tsx`: Tenant configuration
- `Chat.tsx`: Chat interface
- `AgentSimulation.tsx`: Testing tool
- `RagStats.tsx`: RAG analytics

7.3 **Widget System**
- Standalone Vite build
- Embeddable via script tag
- Cross-origin communication

7.4 **Critical Thinking Exercise**
- Why MobX instead of Redux?
- How would you add a new admin page?
- What are the tradeoffs of Joy UI vs. other component libraries?

---

## PART IV: DATABASE — SQLite to PostgreSQL Migration

### Chapter 8: SQLite Development

8.1 **SQLite Configuration**
- WAL journal mode for performance
- Foreign key enforcement
- Busy timeout handling

8.2 **Schema Management**
- LATEST.sql: complete schema
- Migration files: incremental changes
- Testing migrations: `task validate:migrations`

8.3 **Common SQLite Pitfalls**
- Type affinity (everything is TEXT/BLOB/INTEGER/REAL)
- No concurrent writers
- No network access
- JSONB support limitations

### Chapter 9: PostgreSQL Production

9.1 **PostgreSQL Setup**
- Neon serverless Postgres
- Connection pooling (pgx/v5)
- `simple_protocol` for pgbouncer compatibility

9.2 **Migration Differences**
- Serial vs. AUTOINCREMENT
- JSONB vs. TEXT for JSON storage
- Full-text search capabilities
- Array types

9.3 **Performance Tuning**
- Connection pool settings
- Index optimization
- Query plan analysis

9.4 **Critical Thinking Exercise**
- Why use `pgx/v5` instead of `database/sql` + `lib/pq`?
- How would you handle a schema change that affects 1M+ rows?
- What monitoring would you add for production Postgres?

### Chapter 10: Migration Process

10.1 **SQLite → PostgreSQL Migration Steps**
```bash
# 1. Export from SQLite
sqlite3 memos.db .dump > dump.sql

# 2. Convert syntax (if needed)
# - AUTOINCREMENT → SERIAL
# - Boolean 0/1 → TRUE/FALSE
# - Date functions → PostgreSQL equivalents

# 3. Import to PostgreSQL
psql -h localhost -U bchat -d bchat < dump.sql

# 4. Validate
task validate:schema
```

10.2 **Automated Migration Tools**
- `scripts/validate-db-migrations.sh`
- `store/migrator.go`: programmatic migration
- Testing with both databases

10.3 **Critical Thinking Exercise**
- What data types need special handling during migration?
- How would you migrate a production database with zero downtime?
- What rollback strategy would you use?

---

## PART V: DEPLOYMENT — Getting to Production

### Chapter 11: Docker & Containerization

11.1 **Dockerfile Anatomy**
```dockerfile
# Stage 1: Frontend build
FROM node:20 AS frontend
RUN npm ci && npm run build

# Stage 2: Backend build
FROM golang:1.25 AS backend
RUN go build -o memos

# Stage 3: Runtime
FROM ubuntu:24.04
COPY --from=backend /go/memos /usr/local/bin/
COPY --from=frontend /app/dist /app/dist
```

11.2 **Docker Compose (Local)**
- PostgreSQL container (`scripts/docker-compose.postgres.yml`)
- Volume mounting for data persistence
- Environment variable management

11.3 **Security Best Practices**
- Non-root user (`memos`)
- Read-only filesystem
- Secret management via `_FILE` suffix
- `gosu` for privilege dropping

### Chapter 12: Fly.io Deployment

12.1 **Fly.io Configuration**
```toml
# fly.toml
app = "bchat-pg"
primary_region = "sjc"

[build]
  dockerfile = "Dockerfile.pg.fly"

[env]
  DRIVER = "postgres"
  SERVER_MODE = "prod"

[http_service]
  internal_port = 5230
  force_https = true
  auto_stop_machines = "stop"
  auto_start_machines = true

[[vm]]
  memory = "1024mb"
  cpu_kind = "shared"
  cpus = 1
```

12.2 **Deployment Steps**
```bash
# 1. Login to Fly.io
fly auth login

# 2. Launch (first time)
fly launch

# 3. Set secrets
fly secrets set DATABASE_URL="postgresql://..."
fly secrets set OPENROUTER_API_KEY="sk-or-..."
fly secrets set ENCRYPTION_MASTER_KEY="..."

# 4. Deploy
fly deploy

# 5. Check status
fly status
fly logs
```

12.3 **Database Connection (Neon)**
- Connection string format
- SSL/TLS requirements
- Connection pooling considerations

12.4 **Monitoring & Observability**
- Health checks (`/healthz`)
- Fly.io metrics
- Log aggregation

12.5 **Critical Thinking Exercise**
- Why Fly.io instead of AWS ECS/EKS?
- How would you set up CI/CD for automatic deployments?
- What would you do if Neon goes down?

### Chapter 13: Production Hardening

13.1 **Security Audit Checklist**
- [ ] Tenant isolation verified
- [ ] API keys encrypted at rest
- [ ] HTTPS enforced
- [ ] Rate limiting enabled
- [ ] Input validation complete
- [ ] SQL injection prevented
- [ ] XSS protection enabled

13.2 **Performance Optimization**
- Database connection pooling
- Response caching (Redis?)
- CDN for static assets
- LLM response caching

13.3 **Cost Management**
- LLM API cost monitoring
- Infrastructure cost tracking
- Auto-scaling policies
- Budget alerts

---

## PART VI: MAINTENANCE — Keeping It Running

### Chapter 14: Monitoring & Observability

14.1 **Logging Strategy**
- Structured logging (slog)
- Log levels (debug, info, warn, error)
- Log aggregation (Fly.io logs)

14.2 **Metrics to Monitor**
- System: CPU, memory, disk, network
- Application: request latency, error rates, throughput
- Business: active tenants, conversations, LLM costs
- Database: connection pool, query performance

14.3 **Alerting Rules**
- Error rate > 1%
- Response time > 2s
- Database connections > 80%
- LLM cost > budget

### Chapter 15: Database Maintenance

15.1 **Backup Strategy**
- Neon automatic backups
- Manual pg_dump schedule
- Point-in-time recovery

15.2 **Performance Monitoring**
- Slow query logging
- Index usage statistics
- Table bloat analysis

15.3 **Schema Evolution**
- Adding new columns (nullable first)
- Backfilling data
- Removing deprecated columns

15.4 **Critical Thinking Exercise**
- How would you handle a database corruption scenario?
- What's your recovery time objective (RTO) and recovery point objective (RPO)?
- How would you perform a blue-green database migration?

### Chapter 16: Incident Response

16.1 **Incident Severity Levels**
- P0: Complete outage
- P1: Major feature broken
- P2: Minor feature broken
- P3: Cosmetic issue

16.2 **Runbook Template**
```markdown
## Incident: [Description]
**Severity:** P0/P1/P2/P3
**Impact:** [Who is affected]

### Detection
- How was it detected?
- What alerts fired?

### Diagnosis
- What are the symptoms?
- What changed recently?

### Mitigation
- Immediate steps to restore service

### Resolution
- Root cause analysis
- Permanent fix

### Prevention
- What would prevent this from happening again?
```

16.3 **Post-Mortem Process**
- Timeline reconstruction
- Root cause analysis
- Action items with owners
- Knowledge base updates

---

## PART VII: BUSINESS — Scaling the Product

### Chapter 17: Tenant Onboarding

17.1 **Tenant Setup Process**
```bash
# 1. Create tenant
curl -X POST /api/v1/agent/tenants \
  -d '{"slug": "acme", "company_name": "Acme Corp"}'

# 2. Upload KB.MD
curl -X POST /api/v1/agent/acme/files \
  -F "file=@KB.MD" -F "type=kb"

# 3. Upload POLICY.MD
curl -X POST /api/v1/agent/acme/files \
  -F "file=@POLICY.MD" -F "type=policy"

# 4. Upload SCRIPT.MD
curl -X POST /api/v1/agent/acme/files \
  -F "file=@SCRIPT.MD" -F "type=script"

# 5. Reindex (build RAG)
curl -X POST /api/v1/agent/acme/reindex

# 6. Test
curl -X POST /api/v1/agent/acme/chat \
  -d '{"message": "Hello!"}'
```

17.2 **Quality Assurance**
- Simulation testing (`/simulate` endpoint)
- Transcript analysis
- Scoring and feedback

17.3 **Support & Troubleshooting**
- Common tenant issues
- Debugging tools (`/validate`, `/rag/stats`)
- Log analysis

### Chapter 18: Growth & Scaling

18.1 **Multi-Tenant Scaling**
- Database sharding strategies
- Connection pool optimization
- LLM rate limiting

18.2 **Feature Roadmap**
- Observational Memory improvements
- Advanced analytics
- Integration ecosystem
- White-label options

18.3 **Cost Optimization**
- LLM model selection (cost vs. quality)
- Caching strategies
- Infrastructure right-sizing

18.4 **Critical Thinking Exercise**
- At what point would you need to shard the database?
- How would you add a new LLM provider?
- What's the break-even point for self-hosted vs. managed services?

---

## PART VIII: ADVANCED TOPICS

### Chapter 19: Security Deep Dive

19.1 **Authentication System**
- JWT token generation and validation
- HMAC bridge authentication
- API key encryption (AES-256)

19.2 **Authorization (RBAC)**
- Permission templates (viewer, tester, analyst, editor, admin)
- Tenant-scoped permissions
- Superuser bypass patterns

19.3 **Data Protection**
- Encryption at rest
- Encryption in transit
- PII handling
- Audit logging

### Chapter 20: Performance Engineering

20.1 **Profiling**
- pprof integration
- Memory profiling
- CPU profiling

20.2 **Optimization Techniques**
- Database query optimization
- Connection pooling
- Caching strategies
- Async processing

20.3 **Load Testing**
- Locust/k6 setup
- Performance baselines
- Capacity planning

### Chapter 21: Contributing to Bchat

21.1 **Development Workflow**
- Fork & branch strategy
- Commit conventions
- Pull request process

21.2 **Code Quality**
- Go linting (golangci-lint)
- TypeScript checking
- Test coverage requirements

21.3 **Documentation**
- Code comments
- API documentation
- Architecture decision records

---

## APPENDICES

### Appendix A: Environment Variables Reference
### Appendix B: API Endpoint Reference
### Appendix C: Database Schema Reference
### Appendix D: Troubleshooting Guide
### Appendix E: Glossary of Terms

---

## LEARNING PATH RECOMMENDATIONS

### Path 1: Complete Beginner (4-6 weeks)
1. Chapters 1-2: Business context & technology choices
2. Chapters 5-6: Local development & agent system
3. Chapters 8-9: Database fundamentals
4. Chapters 11-12: Deployment basics

### Path 2: Experienced Developer (2-3 weeks)
1. Chapter 3: Architecture overview
2. Chapters 5-7: Development deep dive
3. Chapters 10-12: Migration & deployment
4. Chapter 14-16: Production operations

### Path 3: Business Focus (1-2 weeks)
1. Chapter 1: Business case
2. Chapter 17: Tenant onboarding
3. Chapter 18: Growth & scaling
4. Appendices: Reference materials

---

## TUTORIAL FEATURES

### Hands-On Exercises
- Each chapter includes 2-3 practical exercises
- Progressive complexity: basic → intermediate → advanced
- Real-world scenarios from the bchat codebase

### Code References
- All examples reference actual bchat source code
- File paths with line numbers for easy navigation
- "Try This" sections for experimentation

### Business Insights
- Cost analysis for each architectural decision
- Tradeoff discussions (build vs. buy, simplicity vs. features)
- Real-world scaling considerations

### Critical Thinking Prompts
- "Why not X?" discussions for every major decision
- Security and failure scenario analysis
- Business impact considerations

---

Does this outline meet your vision? I'd like to discuss:

1. **Depth vs. Breadth**: Should we go deeper on specific chapters or keep it balanced?
2. **Business Integration**: How deeply should we weave business acumen throughout vs. keeping it in dedicated chapters?
3. **Hands-On Focus**: Should every chapter have a runnable exercise, or are some chapters more conceptual?
4. **Target Audience**: Is "total newbies" accurate, or should we assume some programming familiarity?
5. **Missing Topics**: Any critical areas I haven't covered?

---

## User

1) keep it balanced 2) think more like a product manager and as skilled software engineer 3) focus on the alpha feature of bugs/033 as hands-on focus where the goal is to compete and replace the expensive SaaS as one cohesive SaaS 4) intermediate devs who want production-grade practices 5. focus on critical thinking, the goal is for the audience not to outsource critical thinking to coding agents

---

## Assistant (Plan · MiMo V2.5 Free · 155.6s)

_Thinking:_

The user wants me to refine the outline based on their feedback:

1. Keep it balanced - don't go too deep or too shallow
2. Think like a product manager and skilled software engineer - business + technical
3. Focus on the alpha feature of bugs/033 as hands-on focus - this is about competing and replacing expensive SaaS
4. Target audience: intermediate devs who want production-grade practices
5. Critical thinking focus - audience should NOT outsource critical thinking to coding agents

Let me first explore the bugs/033 directory to understand what this feature is about, then revise the outline accordingly.

Let me explore the `bugs/033` directory to understand the alpha feature before refining the outline.

**Tool: task**

**Input:**
```json
{
  "description": "Explore bugs/033 feature thoroughly",
  "prompt": "Thoroughly explore the bchat codebase focusing on the `bugs/033` directory and any related code:\n\n1. Read ALL files in `bugs/033/` directory\n2. Search for any references to \"033\" or the feature name in the codebase\n3. Identify what this feature does - is it a new capability, a bug fix, or an enhancement?\n4. Look at related code in `server/router/api/v1/agent/` that implements this feature\n5. Check `store/` for any database changes related to this feature\n6. Check frontend code for any UI components related to this feature\n7. Look at any documentation about this feature in `docs/`\n\nReturn:\n- Complete content of all files in bugs/033/\n- What the feature accomplishes from a business perspective\n- What code changes are needed (backend, frontend, database)\n- How this feature positions bchat against competitors\n- Any architectural decisions or tradeoffs mentioned",
  "subagent_type": "explore"
}
```

**Output:**
```
<task id="ses_0b33fe701ffevtrqUQ506HGd0a" state="completed">
<task_result>
Now I have a complete picture. Here is my thorough analysis.

---

# bugs/033 Analysis: Native Webhook Integrations v1

## 1. Complete Content of All Files in bugs/033/

The directory contains **25 files** representing the full lifecycle of a feature from business planning through implementation and code review. The files are:

### Planning Documents (business + architecture)
- **`docs_033.md`** -- End-user documentation with testing steps, API reference, webhook payload schema, and files-changed summary
- **`plan_biz.md`** -- Initial 1425-line business plan proposing Webhooks (Phase 1), SMS/Twilio (Phase 2), Calendar (Phase 3), and Admin UI (Phase 4)
- **`plan_biz_review.md`** -- Adversarial review of plan_biz: REWORK verdict, 5 critical + 6 major findings
- **`plan2.md`** -- Revised plan addressing all critical/major findings, scoped to Webhooks + SMS only, Calendar deferred
- **`plan2_review.md`** -- Approved with nits (context cancellation fix needed)
- **`plan3.md`** -- Adds distributed resilience: outbox pattern, `FOR UPDATE SKIP LOCKED`, idempotency keys, Neon transient error handling
- **`plan3_draft.md`** -- Draft of plan3 (Path A Platform Managed approach)
- **`plan3_review.md`** -- REWORK: discovered the "same transaction" outbox claim is unimplementable, idempotency key design is flawed, Path A is contradicted by the data model
- **`plan4.md`** -- Approved final plan: tenant-provided Twilio credentials, post-insert outbox with documented failure window, lease-based claims
- **`plan4_review.md`** -- APPROVED FINAL with 4 amendments (A-D): MySQL compile stubs, pre-claimed events, max attempts cap, SMS enqueue deferred
- **`plan4_tasks.md`** -- Task checklist for implementation
- **`plan4_imp.md`** -- Detailed implementation plan with exact code, SQL, file locations, and line numbers
- **`plan4_imp_review.md`** -- REWORK on container/deploy only: entrypoint replacement was dangerous (drops _FILE secrets, runs as root), scale-to-zero claim was false
- **`plan5.md`** -- Final consolidated plan incorporating all review findings

### Code Review Documents
- **`prompt.md`** -- Interactive Q&A discussing the pivot from native integrations to webhook-first strategy
- **`prompt_code_review.md`** -- Adversarial code review scope and checklist
- **`code_review.md`** -- Adversarial code review: REWORK -- found idempotency key argument swap (all events deduplicated to one), poller context cancelled before delivery completes
- **`code2_plan.md`** -- Fix plan for code review findings
- **`code2_plan_review.md`** -- REWORK: found C1 fix drops integrationID from key
- **`code3_plan.md`** -- Final fix plan with variadic idempotency key, PII masking, dead error return removal
- **`code3_plan_review.md`** -- APPROVED with nits

### Reference Materials
- **`heliosai_workflow.md`** -- Competitor (Helios AI) workflow analysis: 5-step onboarding process
- **`resilience_guide.md`** -- Neon Postgres resilience guide: transient error codes, idempotency key pattern, retry with backoff + jitter, examples in Node.js/Python/Java/.NET
- **`reslience_prompt.md`** -- AI prompt template for implementing resilient Postgres connection/retry logic
- **`supercronic.md`** -- Supercronic documentation: container-optimized cron runner

---

## 2. What the Feature Accomplishes (Business Perspective)

**Feature name:** Native Webhook Integrations v1

**What it does:** When a visitor shares their contact information (name + email or phone) during a chat conversation, bchat captures the lead and immediately sends the lead data as a JSON payload to configured webhook URLs with HMAC-SHA256 signatures for verification.

**Why it matters:**

1. **Replaces expensive third-party tools:** Positions bchat to compete with Intercom AI ($97/mo), Helios AI ($100/mo), and parts of n8n ($49/mo) -- potentially saving $246/mo per deployment.

2. **Event-driven architecture:** Instead of building native Twilio SMS, Google Calendar, and Outlook integrations (which would take 2-3 months), bchat emits rich webhook events that external tools (Zapier, n8n, personal AI agents like Claude Co-Work, Hermes) can consume via bridge platforms.

3. **Infinite extensibility:** A webhook-first approach means bchat instantly integrates with everything via Zapier/Make on day one, without maintaining native code for each integration.

4. **Platform positioning:** bchat becomes the "source of truth" for events. External AI agents or automation platforms pull data from bchat rather than bchat pushing to every service.

5. **"Batteries included" with zero tenant friction:** Tenants configure a webhook URL in the Admin UI and immediately receive lead notifications to their CRM, Slack, email, or any webhook-compatible service. No Zapier account needed for the basic use case.

---

## 3. What Code Changes Are Needed (and what has been implemented)

### Backend Changes

**Already implemented in the codebase:**

| Component | File | Status |
|-----------|------|--------|
| Database migrations (SQLite) | `/home/chaschel/Documents/go/bchat/store/migration/sqlite/0.31/00__agent_integrations.sql` | Implemented |
| Database migrations (SQLite) | `/home/chaschel/Documents/go/bchat/store/migration/sqlite/0.31/01__agent_events.sql` | Implemented |
| Database migrations (Postgres) | `/home/chaschel/Documents/go/bchat/store/migration/postgres/0.31/00__agent_integrations.sql` | Implemented |
| Database migrations (Postgres) | `/home/chaschel/Documents/go/bchat/store/migration/postgres/0.31/01__agent_events.sql` | Implemented |
| Store types | `/home/chaschel/Documents/go/bchat/store/agent.go` (lines 1200-1282) | Implemented |
| Store interface | `/home/chaschel/Documents/go/bchat/store/driver.go` (lines 272-282) | Implemented |
| SQLite driver | `/home/chaschel/Documents/go/bchat/store/db/sqlite/agent.go` (lines 2600-2815) | Implemented |
| Postgres driver | `/home/chaschel/Documents/go/bchat/store/db/postgres/agent.go` (lines 2483-2700) | Implemented |
| Postgres resilience | `/home/chaschel/Documents/go/bchat/store/db/postgres/resilience.go` | Implemented |
| MySQL stubs | `/home/chaschel/Documents/go/bchat/store/db/mysql/agent.go` (lines 413-449) | Implemented |
| Webhook dispatch + SSRF | `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/integrations.go` (532 lines) | Implemented |
| Event dispatch + poller | `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go` (lines 4521-4659) | Implemented |
| Route registration | `/home/chaschel/Documents/go/bchat/server/router/api/v1/v1.go` (lines 398-406) | Implemented |
| Supercronic in Dockerfile | `/home/chaschel/Documents/go/bchat/Dockerfile.pg.fly` (lines 70-75, 86-88) | Implemented |
| Supercronic in entrypoint | `/home/chaschel/Documents/go/bchat/scripts/entrypoint.sh` (lines 49, 57-59, 71-73) | Implemented |
| fly.toml kill_timeout | `/home/chaschel/Documents/go/bchat/fly.toml` (line 6) | Implemented |

**Not yet found in the codebase (may still be needed):**
- `build/crontab` file (referenced in Dockerfile but glob returned empty -- may be at a different path or not yet created)

### Database Schema

Two new tables created per backend:

**`agent_integrations`** -- Stores webhook configurations per tenant:
- `id`, `tenant_id` (FK to agent_tenants), `integration_type` ("webhook"), `label`, `config` (JSON), `is_active`, timestamps
- Unique index on `(tenant_id, integration_type)` -- one webhook per tenant

**`agent_events`** -- Outbox queue for reliable event delivery:
- `id`, `tenant_id`, `integration_id` (FK), `event_type` ("lead.captured"), `payload` (JSON), `status` (default "processing"), `claimed_at`, `attempts`, `last_error`, `idempotency_key` (UNIQUE), `created_at`
- Pre-claimed on insert (`status = 'processing'`) to avoid double-dispatch

### Frontend Changes

| Component | File | Status |
|-----------|------|--------|
| MobX store (state + CRUD methods) | `/home/chaschel/Documents/go/bchat/web/src/store/v2/agentAdmin.ts` (lines 521, 1184-1268) | Implemented |
| IntegrationsSection component | `/home/chaschel/Documents/go/bchat/web/src/pages/AgentAdminSections/IntegrationsSection.tsx` (296 lines) | Implemented |
| Integration in AgentAdmin | `/home/chaschel/Documents/go/bchat/web/src/pages/AgentAdmin.tsx` (lines 53, 1981) | Implemented |

The frontend uses Joy UI components (Button, Card, Chip, Modal, Switch, Select, etc.) with Tailwind CSS for layout, matching the existing codebase pattern.

### API Endpoints

| Method | Path | Handler | Permission |
|--------|------|---------|------------|
| `GET` | `/api/v1/agent/:slug/integrations` | `HandleListIntegrations` | `tenant:read` |
| `POST` | `/api/v1/agent/:slug/integrations` | `HandleCreateIntegration` | `tenant:admin` |
| `PUT` | `/api/v1/agent/:slug/integrations/:id` | `HandleUpdateIntegration` | `tenant:admin` |
| `DELETE` | `/api/v1/agent/:slug/integrations/:id` | `HandleDeleteIntegration` | `tenant:admin` |
| `POST` | `/api/v1/agent/:slug/integrations/:id/test` | `HandleTestIntegration` | `tenant:admin` |
| `GET` | `/api/v1/agent/:slug/events` | `HandleListEvents` | `tenant:read` |
| `POST` | `/api/v1/system/trigger-cron` | `HandleTriggerCron` | `X-Cron-Token` header |

### Tests

The code review mentions `integrations_test.go` was created, covering HMAC signing, SSRF protection, and idempotency key determinism. However, the code review (`code_review.md`) found bugs in the implementation that were subsequently fixed in `code3_plan.md`.

---

## 4. How This Feature Positions bchat Against Competitors

**Direct competitors referenced in the planning docs:**
- **Intercom AI** ($97/mo) -- "batteries included" chat + booking
- **Helios AI** ($100/mo) -- AI assistant + WhatsApp + Cal.com integration
- **n8n** ($49/mo) -- workflow automation

**Competitive advantages of the webhook-first approach:**

1. **Cost:** bchat saves $246/mo per deployment vs. the combined competitor stack. Tenants get the same capability (lead capture -> external action) at a fraction of the cost.

2. **Extensibility:** While competitors build point-to-point integrations (e.g., Helios AI has a fixed workflow: audit -> setup -> QA -> monthly optimization), bchat's webhook model means it integrates with *anything* -- Zapier, Make, Claude Co-Work, Hermes, OpenClaw, or custom AI agents. The customer is not locked into bchat's integration choices.

3. **Platform positioning:** bchat becomes the "source of truth" for customer interactions. External AI agents consume bchat events via webhooks, keeping bchat as the central intelligence hub rather than one of many tools in a chain.

4. **Simplicity:** The planning docs explicitly discuss the trade-off between "Platform Managed" (Path A: bchat owns Twilio subaccounts, auto-provisions per tenant) vs. "Webhook-First" (Path B: bchat emits events, tenant glues tools). The final decision was Path B for v1 (webhooks only, SMS deferred to v1.1) -- shipping faster with maximum flexibility.

5. **Security-first design:** SSRF protection, HMAC signatures, idempotent delivery, lease-based claiming -- these enterprise-grade features are rarely found in comparable products at this price point.

**The `heliosai_workflow.md` file** documents the competitor's workflow (5-step onboarding: free audit, choose plan, setup, QA, monthly optimization) -- bchat's approach is fundamentally different: self-service with native webhook integrations that work out of the box.

---

## 5. Architectural Decisions and Tradeoffs

### Key Decisions Made

1. **Webhook-first over native integrations:** The team decided NOT to build native Twilio SMS or Calendar integrations (which would take 2-3 months). Instead, bchat emits webhook events that Zapier/Make/personal AI agents consume. This was the pivotal architectural decision documented in `prompt.md`.

2. **Path A rejected for v1:** The original Path A (Platform Managed: bchat owns a master Twilio account, auto-provisions subaccounts per tenant) was deferred. v1 is "tenant-provided Twilio credentials" -- simpler but requires tenants to have their own Twilio account for SMS.

3. **MySQL dropped from scope:** MySQL schema is frozen at version 0.25 and lacks `agent_tenants`. Adding MySQL would require creating the parent table first. Feature targets SQLite + Postgres only.

4. **Scheduled SMS deferred to v1.1:** SMS delivery via Twilio was originally part of v1 but was cut. The `agent_sms_messages` and `agent_sms_optouts` tables were removed from v1 migrations. This reduces scope to webhooks-only.

5. **Outbox pattern over fire-and-forget:** The initial plan used fire-and-forget goroutines (like the existing OM pattern). This was replaced with a transactional outbox pattern where events are inserted into the database, then claimed and delivered. This provides durability and retry capability.

6. **Pre-claimed events:** Events are inserted with `status = 'processing'` and `claimed_at = now()`. This eliminates the race condition between immediate dispatch and poller claiming (Amendment B from the review).

7. **Lease-based claim recovery:** Stale claims (rows stuck in `processing` after a crash) are reclaimed after 5 minutes via `claimed_at < now() - 300`. This provides at-least-once delivery with idempotent deduplication.

8. **Supercronic over in-process cron:** External cron process (supercronic) was chosen over in-process `robfig/cron` because it survives Go panics and matches container patterns. The tradeoff: retries only run while the machine is awake (scale-to-zero limitation documented honestly).

9. **SSRF protection copied, not imported:** The SSRF helpers (`validateAndResolveWebhookURL`, `isInternalIP`) were copied from `plugin/webhook/webhook.go` because they are unexported there. The two callers diverge (protobuf vs raw JSON) so decoupling is appropriate.

10. **Idempotency key is deterministic:** `hash(tenantID, leadID, eventType, integrationID)` ensures each lead+event+integration tuple is unique. The retry wrapper treats `23505` (unique violation) as success -- if the insert was committed but the ack was lost, the retry sees the duplicate and considers it done.

### Tradeoffs Documented in the Planning

| Tradeoff | Choice | Rationale |
|----------|--------|-----------|
| Scale-to-zero vs retry latency | Awake-only retries | In-container supercronic cannot wake a stopped machine; immediate dispatch handles the live path, poller is catch-up/retry |
| Graceful shutdown vs lease reclaim | Lease reclaim is sufficient | 5-minute lease reclaim makes graceful drain an optimization, not a correctness requirement |
| Path A (Platform Managed) vs Path B (Webhook-First) | Path B for v1 | Faster to ship, infinitely extensible; Path A deferred to v2 |
| In-process cron vs supercronic | Supercronic | External process, standard cron syntax, container-optimized, survives Go panics |
| Full transactional outbox vs post-insert outbox | Post-insert | Store layer has no transaction API; residual failure window documented honestly |
| Event payload in API response vs PII masking | Omit payload | `HandleListEvents` uses `safeEvent` struct excluding payload (name, email, phone) to prevent PII leakage |

### Known Limitations (from docs_033.md)

- **Webhooks only:** SMS follow-ups deferred to v1.1
- **Awake-only retries:** Poller only runs while machine is awake; idle machines defer retries until next wake
- **SQLite write lock:** Event claiming acquires table-level lock (acceptable at v1 scale)
- **No `build/crontab` file found:** The crontab file is referenced in the Dockerfile but may not exist yet or may be at a different path
</task_result>
</task>
```

---

## Assistant (Plan · MiMo V2.5 Free · 105.7s)

_Thinking:_

Now I have a comprehensive understanding of the bugs/033 feature. This is a Native Webhook Integrations feature that:

1. Captures leads from chat conversations
2. Sends webhook events with HMAC signatures
3. Uses an outbox pattern for reliable delivery
4. Positions bchat against expensive SaaS competitors (Intercom AI, Helios AI, n8n)

This is the "alpha feature" the user wants to focus on as the hands-on learning vehicle. The user wants:

1. Balanced depth (not too deep, not too shallow)
2. Product manager + skilled software engineer perspective
3. Focus on bugs/033 as the hands-on feature
4. Intermediate developers who want production-grade practices
5. Critical thinking focus - audience should NOT outsource to coding agents

Let me now revise the outline to incorporate these requirements. The key is to use bugs/033 as the thread that runs through the entire tutorial, teaching production-grade practices through real implementation.

Excellent. Now I have a complete picture. Let me revise the outline using bugs/033 as the thread that runs through the entire tutorial.

---

# BCHAT TUTORIAL OUTLINE v2: Building Production-Grade SaaS

## From Webhook Integrations to Competing with Expensive SaaS

---

## PART I: PRODUCT THINKING — Before You Write a Line

### Chapter 1: The Competitive Landscape & Product Strategy

1.1 **The Problem We're Solving**
- Small businesses pay $246/mo for fragmented tools: Intercom AI ($97), Helios AI ($100), n8n ($49)
- Each tool does one thing well; stitching them together is expensive and fragile
- The gap: a single platform that handles AI chat, lead capture, AND external integrations

1.2 **The Strategic Pivot (bugs/033)**
- Original plan: Build native Twilio SMS, Google Calendar, Outlook integrations (2-3 months)
- Pivot decision: Webhook-first approach ships in weeks, integrates with everything
- The lesson: **scope management is the most valuable product skill**
- Reference: `bugs/033/prompt.md` — the Q&A that forced the pivot

1.3 **Why Webhooks Win**
- Native integrations = finite (you build 5, customers want 50)
- Webhooks = infinite (customers connect to anything via Zapier/Make/n8n)
- bchat becomes the "source of truth" — external agents pull from us

1.4 **Critical Thinking Exercise**
- The plan went through 5 iterations (plan_biz → plan2 → plan3 → plan4 → plan5)
- Read `plan_biz_review.md`: what were the 5 critical findings that killed the first plan?
- **Lesson:** First-pass plans are almost always wrong. Adversarial review catches what authorship misses.

### Chapter 2: Architecture Decision-Making

2.1 **The "Same Transaction" Trap (plan3 → plan4)**
- plan3 claimed: "insert event and deliver webhook in the same transaction"
- plan3_review killed it: "unimplementable — Store layer has no transaction API"
- **Lesson:** If a plan contradicts the data model, it's wrong. Verify assumptions against code.

2.2 **Pre-Claimed Events: A Race Condition Solution**
- Problem: Insert event → immediate dispatch → poller also claims it → double delivery
- Solution: Insert with `status='processing'` and `claimed_at=now()` — poller skips it
- **Lesson:** Race conditions are about state transitions. Think about who reads/writes when.

2.3 **Idempotency Keys: Deterministic Deduplication**
- Key: `hash(tenantID, leadID, eventType, integrationID)`
- PostgreSQL unique constraint (`23505`) treated as success — retry sees duplicate, considers done
- **Lesson:** Network errors after commit are indistinguishable from non-delivery. Design for both.

2.4 **SSRF Protection: Security by Default**
- Webhook URLs are user-provided → SSRF risk (internal IPs, metadata endpoints)
- Copied from `plugin/webhook/webhook.go` (unexported helpers) — deliberate duplication
- **Lesson:** Security code that's hard to import is better than no security code.

2.5 **Critical Thinking Exercise**
- Compare `plan3_review.md` (REWORK) with `plan4_review.md` (APPROVED)
- What specific language changed between plans that made the difference?
- **Lesson:** Writing clearly is thinking clearly. Vague plans produce buggy code.

---

## PART II: DATABASE DESIGN — Schema as Contract

### Chapter 3: Schema Design for Multi-Tenant Systems

3.1 **The Two New Tables (0.31 migrations)**

```sql
-- agent_integrations: "What webhooks does this tenant have?"
CREATE TABLE agent_integrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    integration_type TEXT NOT NULL DEFAULT 'webhook',
    label TEXT NOT NULL,
    config TEXT NOT NULL DEFAULT '{}',
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX idx_integration_tenant_type ON agent_integrations(tenant_id, integration_type);

-- agent_events: "What needs to be delivered?"
CREATE TABLE agent_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    integration_id INTEGER NOT NULL,
    event_type TEXT NOT NULL DEFAULT 'lead.captured',
    payload TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'processing',
    claimed_at TEXT,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    idempotency_key TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (integration_id) REFERENCES agent_integrations(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX idx_events_idempotency ON agent_events(idempotency_key);
```

3.2 **Design Decisions Explained**
- `UNIQUE(tenant_id, integration_type)`: One webhook per tenant per type — simple, enforced
- `ON DELETE CASCADE`: Tenant deleted → all integrations and events cascade — no orphaned data
- `status = 'processing'` on insert: Pre-claiming eliminates race condition with immediate dispatch
- `idempotency_key` with UNIQUE index: Database-level deduplication, not application logic

3.3 **SQLite vs. PostgreSQL Differences**
- SQLite: `datetime('now')` for timestamps, `INTEGER` for booleans, `TEXT` for JSON
- PostgreSQL: `NOW()` for timestamps, `BOOLEAN` type, `JSONB` for structured data
- Same migration files, different SQL dialects — the abstraction layer handles this

3.4 **Critical Thinking Exercise**
- `agent_events.status` has 4 states: `processing`, `pending`, `delivered`, `dead`
- What happens if a webhook delivery takes 10 minutes? (Lease expiry: 5 minutes)
- How does `claimed_at < now() - 300` recover stale claims?
- **Lesson:** State machines need timeout/escape hatches. Every "stuck" state needs a recovery path.

### Chapter 4: Store Layer — The Abstraction That Matters

4.1 **The Driver Interface (`store/driver.go`)**
```go
// What you add when adding a new entity
CreateIntegration(ctx context.Context, tenantID int32, integration *AgentIntegration) (*AgentIntegration, error)
GetIntegration(ctx context.Context, tenantID int32, integrationID int32) (*AgentIntegration, error)
ListIntegrations(ctx context.Context, tenantID int32) ([]*AgentIntegration, error)
UpdateIntegration(ctx context.Context, tenantID int32, integrationID int32, update *AgentIntegrationUpdate) (*AgentIntegration, error)
DeleteIntegration(ctx context.Context, tenantID int32, integrationID int32) error

// Event outbox operations
InsertEvent(ctx context.Context, tenantID int32, event *AgentEvent) (*AgentEvent, error)
ClaimEvents(ctx context.Context, limit int) ([]*AgentEvent, error)
AckEvent(ctx context.Context, eventID int32, status string) error
ListEvents(ctx context.Context, tenantID int32, filter *EventFilter) ([]*AgentEvent, error)
```

4.2 **SQLite Implementation (`store/db/sqlite/agent.go`)**
- Uses `modernc.org/sqlite` — pure Go, no CGO
- `INSERT OR IGNORE` for idempotent inserts (SQLite doesn't have `ON CONFLICT DO NOTHING` for all cases)
- Busy timeout: 10 seconds for write contention

4.3 **PostgreSQL Implementation (`store/db/postgres/agent.go`)**
- Uses `pgx/v5` — high-performance, pure Go driver
- `ON CONFLICT (idempotency_key) DO NOTHING` — native conflict handling
- `FOR UPDATE SKIP LOCKED` for claim operations — non-blocking claims
- Connection pool: 10 max open, 5 max idle, 5min lifetime

4.4 **PostgreSQL Resilience (`store/db/postgres/resilience.go`)**
- Transient error codes from Neon: `57P01`, `57P02`, `57P03`, `57P04`
- Retry with exponential backoff + jitter
- Circuit breaker pattern for connection failures

4.5 **Critical Thinking Exercise**
- Compare the SQLite and Postgres implementations of `ClaimEvents`
- SQLite uses `LIMIT ?` with no locking; Postgres uses `FOR UPDATE SKIP LOCKED`
- Why can't SQLite use row-level locking? What are the implications at scale?
- **Lesson:** Driver abstraction hides differences but doesn't eliminate them. You still need to understand each database's semantics.

---

## PART III: IMPLEMENTATION — Building the Feature

### Chapter 5: Backend Implementation

5.1 **The Webhook Dispatcher (`integrations.go`)**

The core of the feature: 532 lines handling:
- HMAC-SHA256 signing (`X-Bchat-Signature` header)
- SSRF protection (URL validation, internal IP blocking)
- Exponential backoff retry (3 attempts)
- Event status tracking (delivered/failed/dead)

```go
// The critical function: send one webhook
func (s *Service) dispatchWebhook(ctx context.Context, tenantID int32, event *AgentEvent, integration *AgentIntegration) error {
    // 1. Parse config (URL, secret, headers)
    // 2. Validate URL (SSRF check)
    // 3. Build payload with timestamp
    // 4. Sign with HMAC-SHA256
    // 5. POST with timeout
    // 6. Handle response (2xx = success, else = retry)
}
```

5.2 **The Event Poller (`service.go:4521-4659`)**

Background goroutine that:
- Runs every 30 seconds
- Claims up to 10 events (status='processing', claimed_at < now()-300)
- Dispatches each webhook
- Updates status (delivered/dead after max attempts)

5.3 **Route Registration (`v1.go:398-406`)**

```go
// New agent routes
agentGroup := v1Group.Group("/agent")
agentGroup.GET("/:slug/integrations", h.HandleListIntegrations)
agentGroup.POST("/:slug/integrations", h.HandleCreateIntegration)
agentGroup.PUT("/:slug/integrations/:id", h.HandleUpdateIntegration)
agentGroup.DELETE("/:slug/integrations/:id", h.HandleDeleteIntegration)
agentGroup.POST("/:slug/integrations/:id/test", h.HandleTestIntegration)
agentGroup.GET("/:slug/events", h.HandleListEvents)
```

5.4 **Critical Thinking Exercise**
- The webhook dispatcher uses `http.Client{Timeout: 10 * time.Second}`
- What happens if the webhook endpoint is slow but not down? (partial delivery, retry loop)
- How does the `attempts` counter interact with the lease expiry?
- **Lesson:** Timeouts are not just about fast failures. They're about bounded recovery windows.

### Chapter 6: Frontend Implementation

6.1 **MobX Store (`agentAdmin.ts:1218-1268`)**

```typescript
// State
integrations: AgentIntegration[] = [];

// Methods
fetchIntegrations = async (slug: string) => { ... }
createIntegration = async (slug: string, data: CreateIntegrationRequest) => { ... }
updateIntegration = async (slug: string, id: number, data: UpdateIntegrationRequest) => { ... }
deleteIntegration = async (slug: string, id: number) => { ... }
testIntegration = async (slug: string, id: number) => { ... }
```

6.2 **IntegrationsSection Component (`IntegrationsSection.tsx`)**

296-line React component:
- Lists existing integrations with status chips (active/inactive)
- Create/edit modal with URL, secret, label fields
- Test button → sends test event → shows result
- Delete confirmation dialog

6.3 **The PII Masking Decision**
- `HandleListEvents` returns `safeEvent` struct — no `payload` field
- Rationale: payload contains name, email, phone (PII)
- Admin UI shows event status and type, not content
- **Lesson:** API responses are security boundaries. What you expose in a list endpoint can leak PII.

6.4 **Critical Thinking Exercise**
- Compare `IntegrationsSection.tsx` with other `AgentAdminSections/` components
- What patterns repeat? What's the component structure?
- How would you add a new integration type (e.g., Slack)?
- **Lesson:** UI patterns are templates. Once you recognize them, new features are copy-paste-refactor.

### Chapter 7: Testing Strategy

7.1 **What Was Tested (and What Wasn't)**

The code review (`code_review.md`) found:
- **Bug 1:** Idempotency key argument swap — all events deduplicated to one integration
- **Bug 2:** Poller context cancelled before delivery completes

These are the bugs that slipped through. The tests that existed:
- HMAC signing correctness
- SSRF protection (internal IP blocking)
- Idempotency key determinism

7.2 **The Testing Pyramid**
```
        ┌─────────┐
        │  E2E    │  ← few, slow, high confidence
        ├─────────┤
        │  Integration │  ← moderate, test boundaries
        ├─────────┤
        │  Unit   │  ← many, fast, test logic
        └─────────┘
```

7.3 **What Tests Would Catch Bugs 1 & 2**
- Bug 1: Integration test that creates 2 integrations, dispatches to both, verifies both receive events
- Bug 2: Unit test that verifies poller dispatch completes before context cancellation

7.4 **Critical Thinking Exercise**
- The code review found bugs that tests missed
- How do you write tests that catch argument order bugs?
- What's the relationship between code review and testing?
- **Lesson:** Code review and testing are complementary, not redundant. Tests verify what you think you built. Review verifies what you actually built.

---

## PART IV: DATABASE MIGRATION — SQLite to PostgreSQL

### Chapter 8: Migration Strategy

8.1 **The Two-Database Reality**
- SQLite: development (zero-config, single file, fast)
- PostgreSQL: production (Neon serverless, concurrent access, JSONB)
- Same migration files, different SQL dialects

8.2 **Migration File Structure**
```
store/migration/
├── sqlite/
│   ├── 0.31/
│   │   ├── 00__agent_integrations.sql
│   │   └── 01__agent_events.sql
│   └── LATEST.sql (cumulative)
├── postgres/
│   ├── 0.31/
│   │   ├── 00__agent_integrations.sql
│   │   └── 01__agent_events.sql
│   └── LATEST.sql (cumulative)
```

8.3 **SQLite-Specific Considerations**
- No `JSONB` type → use `TEXT` with `json_valid()` check
- No `SERIAL` → use `INTEGER PRIMARY KEY AUTOINCREMENT`
- No `NOW()` → use `datetime('now')`
- Boolean: `INTEGER` (0/1) not `TRUE/FALSE`

8.4 **PostgreSQL-Specific Considerations**
- `JSONB` for structured config data (indexable, queryable)
- `SERIAL` or `GENERATED ALWAYS AS IDENTITY`
- `NOW()` or `CURRENT_TIMESTAMP`
- `BOOLEAN` type

8.5 **LATEST.sql: The Cumulative Schema**
- 1067 lines of complete schema
- Used for fresh installs (skip all incremental migrations)
- Must be regenerated after every migration change

8.6 **Critical Thinking Exercise**
- Why use `ON DELETE CASCADE` for `agent_integrations` and `agent_events`?
- What happens if you delete a tenant with 10,000 pending events?
- How does PostgreSQL handle cascade deletes at scale vs. SQLite?
- **Lesson:** CASCADE is convenient but dangerous. Understand the blast radius of every parent delete.

### Chapter 9: Running Migrations

9.1 **Programmatic Migration (`store/migrator.go`)**
```go
// On server startup
if err := storeInstance.Migrate(ctx); err != nil {
    return fmt.Errorf("failed to migrate: %w", err)
}
```

9.2 **Migration History Table**
```sql
CREATE TABLE migration_history (
    version TEXT NOT NULL,
    filename TEXT NOT NULL,
    applied_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (version, filename)
);
```

9.3 **Validation Commands**
```bash
# Validate LATEST.sql matches incremental migrations
task validate:migrations

# Validate schema and run CRUD tests
task validate:schema
```

9.4 **Production Migration on Neon**
- Neon handles connection routing and pooling
- `pgx/v5` with `simple_protocol` for pgbouncer compatibility
- Migrations run on server startup — no separate migration step needed

9.5 **Critical Thinking Exercise**
- What happens if a migration fails midway? (PostgreSQL transaction rollback vs. SQLite no-transaction)
- How would you handle a migration that takes 10 minutes on a large table?
- **Lesson:** Migrations are the riskiest part of deployment. Test on production-sized data before shipping.

---

## PART V: DEPLOYMENT — Getting to Production

### Chapter 10: Docker Containerization

10.1 **The Three-Stage Dockerfile (`Dockerfile.pg.fly`)**

```dockerfile
# Stage 1: Frontend build (Node.js)
FROM node:20-slim AS frontend
WORKDIR /app
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Backend build (Go)
FROM golang:1.25 AS backend
WORKDIR /go/src/app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /go/bin/memos

# Stage 3: Runtime (Ubuntu)
FROM ubuntu:24.04
RUN apt-get update && apt-get install -y ca-certificates supercronic
COPY --from=backend /go/bin/memos /usr/local/bin/memos
COPY --from=frontend /app/dist /app/dist
COPY scripts/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
```

10.2 **Security in the Container**
- Non-root user (`memos`) via `gosu`
- Read-only filesystem (except data directory)
- `_FILE` suffix support for Docker/K8s secrets
- `entrypoint.sh` processes secrets before starting

10.3 **Supercronic Integration**
- Container-optimized cron runner (replaces `crond`)
- `build/crontab` file defines scheduled tasks
- Survives Go panics (external process)
- Tradeoff: retries only run while machine is awake

10.4 **Critical Thinking Exercise**
- Why use Ubuntu as runtime instead of Alpine? (glibc vs. musl, LanceDB compatibility)
- What's the security implication of `chmod +x /entrypoint.sh`?
- How does `gosu` differ from `su`? Why is it preferred in containers?

### Chapter 11: Fly.io Deployment

11.1 **Configuration (`fly.toml`)**
```toml
app = "bchat-pg"
primary_region = "sjc"

[build]
  dockerfile = "Dockerfile.pg.fly"

[env]
  DRIVER = "postgres"
  SERVER_MODE = "prod"

[http_service]
  internal_port = 5230
  force_https = true
  auto_stop_machines = "stop"
  auto_start_machines = true
  kill_signal = "SIGTERM"
  kill_timeout = "30s"

[[vm]]
  memory = "1024mb"
  cpu_kind = "shared"
  cpus = 1
```

11.2 **Secrets Management**
```bash
# Set secrets (encrypted at rest)
fly secrets set DATABASE_URL="postgresql://..."
fly secrets set OPENROUTER_API_KEY="sk-or-..."
fly secrets set ENCRYPTION_MASTER_KEY="..."
fly secrets set CRON_TOKEN="..."

# Verify
fly secrets list
```

11.3 **Deployment Flow**
```bash
fly deploy        # Build + deploy
fly status        # Check health
fly logs          # Stream logs
fly ssh console   # Debug in production
```

11.4 **Neon Connection Configuration**
- `DATABASE_URL` with SSL mode required
- `pgx/v5` auto-appends `default_query_exec_mode=simple_protocol`
- Connection pool managed by Neon (not application)
- Auto-scaling: Neon suspends when idle, resumes on connect

11.5 **Critical Thinking Exercise**
- `kill_timeout = "30s"` — what happens during those 30 seconds?
- How does `auto_stop_machines` interact with the poller goroutine?
- What's the cold-start time for a stopped Fly.io machine?
- **Lesson:** Serverless ≠ instant. Understanding cold starts, connection times, and lease expiry is critical for reliability.

### Chapter 12: Pre-Deployment Validation

12.1 **The Validation Chain**
```bash
# 1. Environment variables
scripts/validate-env-chain.sh

# 2. Database migrations
scripts/validate-db-migrations.sh

# 3. Schema validation
task validate:schema

# 4. Build
task build:rag

# 5. Deploy
fly deploy
```

12.2 **Pre-Deploy Script (`task fly:pre-deploy`)**
- Validates all required env vars are set
- Checks database connectivity
- Runs migration validation
- Reports any issues before deployment

12.3 **Critical Thinking Exercise**
- Why run validation before deploy instead of after?
- What's the cost of a failed deploy vs. a blocked deploy?
- How would you add a smoke test to the pre-deploy chain?

---

## PART VI: PRODUCTION OPERATIONS — Keeping It Running

### Chapter 13: Monitoring & Observability

13.1 **Health Check (`/healthz`)**
```go
func (h *Handler) HealthCheck(c echo.Context) error {
    // Check database connectivity
    // Check LanceDB availability
    // Return 200 OK or 503 Service Unavailable
}
```

13.2 **Logs to Watch**
```bash
# Successful webhook delivery
"webhook delivered" event_id=123 integration_id=45 status=200

# Failed webhook delivery
"webhook delivery failed" event_id=123 integration_id=45 error="connection refused"

# Event claimed by poller
"event claimed" event_id=123 claimed_by="poller"

# Event moved to dead letter
"event moved to dead" event_id=123 attempts=3
```

13.3 **Metrics to Track**
- Webhook delivery success rate (target: >99%)
- Average webhook latency (target: <2s)
- Dead letter rate (target: <0.1%)
- LLM API cost per tenant
- Database connection pool utilization

13.4 **Alerting Rules**
| Metric | Threshold | Action |
|--------|-----------|--------|
| Delivery success rate | < 95% | Investigate webhook endpoints |
| Dead letter rate | > 1% | Check tenant webhook configs |
| Database connections | > 80% | Scale connection pool |
| LLM cost | > budget | Review tenant usage |

13.5 **Critical Thinking Exercise**
- The poller runs every 30 seconds. What's the maximum latency for webhook delivery?
- How does `kill_timeout = "30s"` interact with in-flight deliveries?
- What happens to events when Fly.io restarts the machine?

### Chapter 14: Incident Response

14.1 **Severity Levels**
- P0: Webhook delivery completely down (all tenants affected)
- P1: Single tenant webhook failing (one tenant affected)
- P2: Event polling delayed (events still delivered, just slow)
- P3: Cosmetic issue in admin UI

14.2 **Runbook: Webhook Delivery Failure**
```markdown
## Symptom: Events stuck in "processing" status

### Diagnosis
1. Check poller logs: `fly logs | grep poller`
2. Check event age: `SELECT id, created_at, claimed_at FROM agent_events WHERE status='processing'`
3. Check lease expiry: `claimed_at < now() - interval '5 minutes'`

### Mitigation
1. Trigger manual cron: `POST /system/trigger-cron`
2. If poller is stuck: `fly apps restart bchat-pg`

### Resolution
1. Check for webhook endpoint issues (tenant-side)
2. Check for database connection issues
3. Review recent deployments for regressions

### Prevention
1. Add monitoring for poller health
2. Add dead letter alerts
3. Document webhook endpoint requirements for tenants
```

14.3 **Post-Mortem Process**
- Timeline: what happened, when, how long
- Root cause: why it happened
- Impact: who was affected, for how long
- Action items: what we'll change (with owners and deadlines)

14.4 **Critical Thinking Exercise**
- A tenant reports "I'm not getting webhook deliveries"
- Walk through the diagnostic steps systematically
- What information do you need from the tenant?
- What information can you get from the database without asking?

---

## PART VII: CRITICAL THINKING — The Meta-Skills

### Chapter 15: Reading Code You Didn't Write

15.1 **The bchat Codebase as a Learning Tool**
- 49 files in `server/router/api/v1/agent/` — each is a lesson
- How to navigate: start from handlers, trace into service, then store

15.2 **Pattern Recognition**
- Handler pattern: extract context → validate input → call service → return response
- Store pattern: interface definition → driver implementation → migration
- Frontend pattern: MobX store → API call → component render

15.3 **When Code Confuses You**
- Start at the entry point (`bin/memos/main.go`)
- Follow the request flow (HTTP → Handler → Service → Store)
- Read the tests (they document expected behavior)
- Check the git log (commit messages explain "why")

### Chapter 16: Evaluating Technical Decisions

16.1 **The Tradeoff Framework**
| Dimension | Option A | Option B |
|-----------|----------|----------|
| Development time | ? | ? |
| Operational complexity | ? | ? |
| Cost at scale | ? | ? |
| Failure modes | ? | ? |
| Reversibility | ? | ? |

16.2 **Case Study: Webhooks vs. Native Integrations**
- Development time: Webhooks (weeks) vs. Native (months)
- Operational complexity: Webhooks (simpler) vs. Native (more state)
- Cost at scale: Webhooks (same) vs. Native (higher maintenance)
- Failure modes: Webhooks (external) vs. Native (internal + external)
- Reversibility: Webhooks (high) vs. Native (low — sunk cost)

16.3 **The "Second System Effect"**
- bugs/033 went through 5 plan iterations
- Each iteration caught assumptions the previous one made
- **Lesson:** Your first plan is your worst plan. Build in adversarial review.

### Chapter 17: Writing Code That Survives

17.1 **Defensive Programming**
- Idempotency keys: network errors are indistinguishable from non-delivery
- Lease expiry: crashed processes leave stale state
- SSRF protection: user input is hostile until validated

17.2 **Operational Visibility**
- Structured logging with context (tenant_id, event_id, integration_id)
- Event status tracking (processing → delivered/dead)
- Audit trail (created_at, claimed_at, attempts)

17.3 **Graceful Degradation**
- If webhook delivery fails, log and continue
- If poller is down, events wait (they're in the database)
- If database is down, return 503 (fail fast, don't queue)

### Chapter 18: When to Stop and Ship

18.1 **The MVP Mindset**
- v1: Webhooks only (SMS deferred to v1.1)
- "Batteries included" with zero tenant friction
- Perfect is the enemy of shipped

18.2 **Known Limitations (Documented Honestly)**
- Awake-only retries (scale-to-zero limitation)
- SQLite write lock (acceptable at v1 scale)
- No `build/crontab` file (may need to be created)

18.3 **The Review Culture**
- Every plan had an adversarial review
- Reviews used structured checklists (critical, major, minor findings)
- **Lesson:** Code review is not about finding bugs. It's about finding wrong assumptions.

---

## APPENDICES

### Appendix A: bugs/033 File Reference
- Complete file listing with purpose and key findings
- Timeline of plan iterations
- Review verdicts and what they caught

### Appendix B: API Endpoint Reference
- All webhook-related endpoints with request/response examples
- HMAC signature verification guide
- Error codes and meanings

### Appendix C: Database Schema Reference
- Complete `agent_integrations` and `agent_events` schema
- Index definitions and query patterns
- Migration file locations

### Appendix D: Environment Variables
- Complete list with defaults and descriptions
- Neon-specific configuration
- Fly.io secrets management

### Appendix E: Troubleshooting Guide
- Common issues and solutions
- Debugging checklist
- Useful SQL queries for diagnosis

---

## TUTORIAL STRUCTURE

### Learning Path (4-6 weeks for intermediate developers)

| Week | Focus | Chapters |
|------|-------|----------|
| 1 | Product thinking + Architecture | 1-2 |
| 2 | Database design + Store layer | 3-4 |
| 3 | Backend implementation | 5 |
| 4 | Frontend + Testing | 6-7 |
| 5 | Database migration | 8-9 |
| 6 | Deployment + Operations | 10-14 |
| Ongoing | Critical thinking | 15-18 |

### Hands-On Exercises (per chapter)
- **Ch 1:** Analyze a competitor's pricing model
- **Ch 2:** Identify a flaw in a given architecture plan
- **Ch 3:** Design a schema for a new feature
- **Ch 4:** Implement a store method for a new entity
- **Ch 5:** Add a new webhook event type
- **Ch 6:** Build a new admin UI section
- **Ch 7:** Write a test that catches argument-order bugs
- **Ch 8:** Create a migration for a new table
- **Ch 9:** Run migration validation on a test database
- **Ch 10:** Build and run the Docker container locally
- **Ch 11:** Deploy to Fly.io with Neon
- **Ch 12:** Add a new validation step to pre-deploy
- **Ch 13:** Set up monitoring for webhook delivery
- **Ch 14:** Write a runbook for a new failure mode
- **Ch 15-18:** Code review exercises using bugs/033 patterns

---

## KEY DIFFERENTIATORS FROM TYPICAL TUTORIALS

1. **Product-first thinking:** Every technical decision traces back to business impact
2. **Real failure analysis:** bugs/033 shows what went wrong, not just what went right
3. **Adversarial review culture:** Plans were killed and rewritten — that's normal
4. **Production-grade from day one:** SSRF protection, idempotency, lease expiry — not "we'll add security later"
5. **Critical thinking over memorization:** Every chapter ends with "why" questions, not just "how" questions
6. **No outsourcing to agents:** The tutorial teaches you to think, not to prompt

---

Does this revised outline capture your vision? I'd like to discuss:

1. **Depth of bugs/033 coverage**: Should we include the actual plan documents (plan_biz → plan5) as reading material, or summarize them?
2. **Exercise format**: Should exercises be "do this in the codebase" or "think about this scenario"?
3. **Competitor analysis depth**: How much should we dive into Intercom/Helios AI pricing and features?
4. **Code walkthrough granularity**: Line-by-line or conceptual flow through the code?

---

## User

1. summarize 2. blend of do and think 3. in-depth because that is the direction of the open source project, turn it into a product 4. conceptual flow. Note: the deliverable is markdown only for now, I can work out on the pdf myself

---

## Assistant (Plan · MiMo V2.5 Free · 306.8s)

_Thinking:_

The user wants me to finalize the outline with these specific decisions:

1. Summarize bugs/033 plan documents (not include them verbatim)
2. Blend of "do this" and "think about this" exercises
3. In-depth competitor analysis because the project is pivoting to a product
4. Conceptual flow through code, not line-by-lineDeliverable: Markdown only

Let me now refine the outline one final time incorporating these decisions, then present it as the final plan for user approval before any implementation.

I'm in plan mode (read-only), so I should present the final refined outline and ask for approval.

Let me do a quick competitor research to inform the in-depth competitive analysis sections.

**Tool: task**

**Input:**
```json
{
  "description": "Analyze bchat competitors deeply",
  "prompt": "Thoroughly analyze the competitive landscape for bchat by:\n\n1. Read `bugs/033/heliosai_workflow.md` completely\n2. Read `bugs/033/plan_biz.md` for competitor analysis (Intercom AI, Helios AI, n8n)\n3. Read `bugs/033/plan2.md` for revised competitive positioning\n4. Search docs/ for any other competitor references or market analysis\n5. Search for pricing references, feature comparisons, or market positioning\n\nReturn:\n- Complete competitor profiles (Intercom AI, Helios AI, n8n, others)\n- Feature-by-feature comparison with bchat\n- Pricing analysis and cost positioning\n- bchat's competitive advantages and gaps\n- The \"replace expensive SaaS\" narrative and supporting evidence",
  "subagent_type": "explore"
}
```

**Output:**
```
<task id="ses_0b3358045fferp6bXor2UhY0G0" state="completed">
<task_result>
Now I have thoroughly analyzed all the relevant documents. Here is the complete competitive landscape analysis.

---

# Competitive Landscape Analysis: bchat

## 1. Complete Competitor Profiles

### 1.1 Helios AI

**Source files:** `bugs/033/heliosai_workflow.md`, `bugs/033/plan_biz.md`, `bugs/033/prompt.md`

**Overview:** Helios AI is an AI-powered "digital front desk" platform for service businesses. It offers a fully managed, "batteries included" experience with three pricing tiers:

| Plan | Name | Description |
|------|------|-------------|
| Tier 1 | Starter | Basic AI assistant |
| Tier 2 | Booking OS | AI + booking capabilities |
| Tier 3 | Helios AIOS | Full AI operating system |

**Workflow (from `heliosai_workflow.md`):**
1. Free Business Audit -- analyze website, booking flow, lead capture, conversation channels
2. Choose Your Plan -- Starter, Booking OS, or Helios AIOS
3. Setup and Configuration -- install AI assistant on website, configure WhatsApp, connect Cal.com, load services/hours/FAQs
4. QA and Launch -- full QA on assistant, booking flow, owner notifications
5. Monthly Optimization -- insights on conversions, prompt adjustments

**Key features identified:**
- Website AI chat assistant installation
- WhatsApp integration
- Cal.com calendar integration
- Service/hours/FAQ loading (similar to bchat's KB.MD approach)
- Monthly optimization cycles
- Fully managed service -- "We install the entire system for you. You do not configure widgets or write prompts."

**Price point:** $100/mo (referenced in `plan_biz.md` and `prompt.md`)

**Strengths relative to bchat:**
- Fully managed onboarding (zero-config for tenant)
- WhatsApp integration built-in
- Calendar booking (Cal.com) included
- Monthly optimization as a service layer
- "Batteries included" -- tenant doesn't need technical knowledge

**Weaknesses relative to bchat:**
- No evidence of RAG pipeline or advanced AI retrieval
- No evidence of multi-tenant self-service platform
- Closed ecosystem -- not extensible via webhooks
- Opaque AI configuration (no KB.MD/POLICY.MD equivalent for tenant control)
- Managed service model doesn't scale to self-service SaaS

---

### 1.2 Intercom AI

**Source files:** `bugs/033/plan_biz.md`, `bugs/033/prompt.md`, `bugs/033/docs_033.md`

**Overview:** Intercom is a well-established customer messaging platform. The "Intercom AI" variant referenced in bchat documentation refers to their AI-enhanced chat features, specifically their Fin AI agent for automated customer support and booking.

**Price point:** $97/mo (referenced in `plan_biz.md` and `prompt.md`)

**Key features identified (from bchat documentation context):**
- AI-powered customer chat/booking
- Customer support automation
- Part of a larger suite (messaging, help desk, CRM)
- Established brand with large customer base

**Context from bchat docs:** Intercom AI is positioned as the primary competitor for the "booking" use case. The `plan_biz.md` explicitly states bchat's goal is to replace "Intercom AI ($97/mo)" with native calendar integration (Phase 2). The `TUTORIAL.md` frames this as: "Business Driver: Replace $246/mo in third-party SaaS (Intercom AI + Helios AI + n8n)."

**Strengths relative to bchat:**
- Mature product with extensive ecosystem
- Established brand trust
- Comprehensive feature set beyond chat (help desk, product tours, etc.)
- Large integration marketplace
- Enterprise-grade reliability

**Weaknesses relative to bchat:**
- Per-seat pricing model -- costs scale linearly with team size
- Not configurable via markdown files (no KB.MD equivalent)
- General-purpose platform (not optimized for service businesses)
- Higher price for comparable chat agent functionality
- No multi-tenant self-service deployment model
- No RAG pipeline or observational memory

---

### 1.3 n8n

**Source files:** `bugs/033/plan_biz.md`, `bugs/033/plan2.md`, `bugs/033/prompt.md`, `TUTORIAL.md`

**Overview:** n8n is an open-source workflow automation platform. It is a self-hostable alternative to Zapier/Make, enabling users to connect different services and build automated workflows.

**Price point:** $49/mo (referenced in `plan_biz.md` for cloud-hosted; free for self-hosted)

**Key features identified from bchat documentation:**
- Visual workflow builder
- Webhook triggers and actions
- 5,000+ app integrations (via Zapier comparison; n8n has 400+ native integrations)
- Self-hostable (open-source)
- Event-driven automation

**Context in bchat strategy:** n8n is positioned in two ways:
1. **As a partial replacement target:** `plan_biz.md` states bchat replaces "parts of n8n ($49/mo)" through native webhook support. The $49/mo represents what a tenant might spend on n8n to bridge bchat to other services.
2. **As a bridge platform:** The "webhook-first" strategy (`prompt.md`) proposes that bchat emits events and tenants use n8n/Zapier to connect to their tools. n8n becomes a complementary tool rather than a competitor.

**Strengths relative to bchat:**
- Open-source and self-hostable
- Massive integration ecosystem
- Visual workflow builder (low-code)
- Community-driven plugin ecosystem
- Can orchestrate complex multi-step workflows

**Weaknesses relative to bchat:**
- Not an AI chat agent -- no conversational AI capabilities
- Requires technical knowledge to set up workflows
- Self-hosted requires infrastructure management
- No native lead capture or chat widget
- No RAG, no knowledge base management
- General-purpose automation, not purpose-built for service businesses

---

### 1.4 Additional Competitors Referenced

**Zapier** (referenced extensively in `prompt.md`, `plan2.md`, `plan5.md`):
- Cloud workflow automation platform
- 5,000+ app integrations
- Per-task pricing model
- Positioned as the "bridge platform" in bchat's webhook-first strategy
- Pre-built Zapier templates proposed as part of bchat's integration UX

**Cal.com** (referenced in `heliosai_workflow.md`):
- Open-source calendar/scheduling platform
- Used by Helios AI for booking
- Alternative to Calendly
- bchat considered building native calendar integration as a direct competitor

**Make (formerly Integromat)** (referenced in `prompt.md`):
- Workflow automation platform (Zapier competitor)
- More advanced logic capabilities than Zapier
- Pre-built Make templates proposed alongside Zapier templates

**Twilio** (referenced in `plan_biz.md`, `plan2.md`):
- Communication API platform (SMS, voice, video)
- Not a direct competitor but a dependency for SMS features
- bchat considered building native Twilio integration vs. webhook-to-Zapier-to-Twilio approach

**Personal AI Agents** -- Claude Co-Work, Hermes, OpenClaw (referenced in `prompt.md`):
- Emerging category of personal AI assistants
- Can orchestrate tasks across services
- The "brilliant" pivot idea: bchat emits events, personal AI agents handle SMS/calendar/tasks
- Represents a paradigm shift from "build native integrations" to "emit events for AI agents to consume"

---

## 2. Feature-by-Feature Comparison with bchat

| Feature | bchat | Helios AI | Intercom AI | n8n |
|---------|-------|-----------|-------------|-----|
| **AI Chat Agent** | Yes (configurable via KB/POLICY/SCRIPT.MD) | Yes (managed setup) | Yes (Fin AI) | No |
| **Multi-Tenant** | Yes (self-service) | Unknown (managed) | Yes (per-seat) | N/A (self-hosted single instance) |
| **Knowledge Base** | Markdown-based (KB.MD) | Managed loading | Built-in articles | N/A |
| **RAG Pipeline** | Yes (LanceDB + hybrid search) | No evidence | No evidence | N/A |
| **Observational Memory** | Yes (OM implementation) | No evidence | No evidence | N/A |
| **Lead Capture** | Yes (automatic extraction) | Yes | Yes | Via webhooks |
| **SMS Integration** | Planned (v1.1 via webhooks) | Yes (WhatsApp) | Yes | Via automation |
| **Calendar Booking** | Planned (webhook-first) | Yes (Cal.com) | Yes | Via automation |
| **Webhook Events** | Yes (v1 implemented) | No evidence | Yes (API) | Yes (core feature) |
| **Workflow Automation** | No (delegates to Zapier/n8n) | No evidence | Yes (built-in) | Yes (core feature) |
| **Self-Service Admin UI** | Yes (React + Joy UI) | Unknown | Yes | Yes |
| **Embeddable Widget** | Yes | Yes | Yes | N/A |
| **Simulation/Testing** | Yes (agent simulation) | No evidence | No evidence | N/A |
| **Custom Branding** | Yes (white-label) | Unknown | Limited | N/A |
| **Conversation Scripts** | Yes (SCRIPT.MD) | No evidence | No evidence | N/A |
| **Escalation Management** | Yes (ticket system) | Yes | Yes | Via automation |
| **Open Source** | Partially (Memos-based) | No | No | Yes |
| **Self-Hosted Option** | Yes (Fly.io, Docker) | No | No | Yes |

---

## 3. Pricing Analysis and Cost Positioning

### 3.1 Competitor Pricing

| Competitor | Monthly Cost | Pricing Model | What You Get |
|------------|-------------|---------------|--------------|
| **Helios AI** | $100/mo | Per-business | Managed AI assistant, WhatsApp, Cal.com booking, monthly optimization |
| **Intercom AI** | $97/mo | Per-seat (Fin AI) | AI chat agent, customer messaging, help desk |
| **n8n Cloud** | $49/mo | Per-workflow executions | Workflow automation, 400+ integrations |
| **Zapier** | $19.99-$69/mo | Per-task | 5,000+ integrations, visual builder |
| **Combined stack** | **$246/mo** | All three | Helios AI + Intercom AI + n8n |

### 3.2 bchat Pricing Model

**From `docs_biz_guide.md` (Section 18):**

| Package | Setup Fee | Monthly Fee | Target Customer |
|---------|-----------|-------------|-----------------|
| **Package 1** (Chat Widget Only) | $149-$299 | $149-$299 | Existing website |
| **Package 2** (Hugo Site + AI Lead Desk) | $399-$999 | $199-$499 | No/bad website |
| **Package 3** (Managed Lead Desk) | N/A | $499-$1,500 | White-glove service |

### 3.3 bchat Internal Cost Structure

**From `docs_biz_guide.md`:**

| Cost Component | Per Tenant/Month | Notes |
|----------------|-----------------|-------|
| OpenRouter API | $5-$50 | Depends on chat volume + model |
| Server hosting (Fly.io) | $15-$50 | Shared across 25-50 tenants |
| LanceDB storage | Negligible | On same disk |
| Domain + SSL | $1-$2 | Per custom domain |
| **Total per tenant** | **$5-$30** | **80-90% margin on $199-$499 retail** |

### 3.4 Cost Positioning Analysis

**The "$246/mo Savings" Narrative:**

The core competitive claim from `plan_biz.md` and `TUTORIAL.md`:
> "Replace $246/mo in third-party SaaS (Intercom AI + Helios AI + n8n) -- saving $246/mo per deployment"

**Breakdown:**
- Intercom AI: $97/mo (booking feature)
- Helios AI: $100/mo (SMS + booking)
- n8n: $49/mo (workflow automation)
- **Total: $246/mo**

**bchat's value proposition:**
- Package 1 at $149-$299/mo replaces $246/mo in combined tools
- Package 2 at $199-$499/mo includes a website + AI chat + lead capture
- bchat's internal cost is $5-$30/tenant, meaning **80-90% margins** at retail price

**The "Replace Expensive SaaS" narrative is supported by:**
1. Native webhook events eliminate the need for n8n ($49/mo savings)
2. RAG-powered AI chat replaces Intercom AI's Fin ($97/mo savings)
3. Configurable KB.MD/POLICY.MD replaces Helios AI's managed setup ($100/mo savings)
4. Calendar integration (planned) would complete the replacement

---

## 4. bchat's Competitive Advantages

### 4.1 Structural Advantages

1. **Configuration-Driven Architecture:** The KB.MD/POLICY.MD/SCRIPT.MD system means a single codebase serves any tenant configuration. Competitors require code changes or managed services per vertical.

2. **RAG Pipeline with Hybrid Search:** LanceDB-based vector search (70% vector similarity + 30% BM25 keyword matching) provides superior knowledge retrieval compared to competitors' simpler keyword or embedding-only approaches.

3. **Observational Memory (OM):** The OM implementation provides long-term memory that outperforms RAG on long-context benchmarks (84.23% on LongMemEval with GPT-4o vs. Mastra RAG at 80.05%). This is a significant technical moat.

4. **Multi-Tenant Self-Service Platform:** Unlike Helios AI's managed model, bchat allows tenants to self-configure their AI agents. This scales without proportional support costs.

5. **Extreme Cost Efficiency:** $5-$30/tenant/month internal cost vs. $149-$499/month retail = 80-90% gross margin. No competitor can match this unit economics at scale.

6. **Open-Source Foundation:** Built on Memos, with open components (LanceDB, webhook-first architecture). This reduces vendor lock-in risk and attracts developer adoption.

7. **Webhook-First Extensibility:** Rather than building every integration natively, bchat emits events that connect to everything via Zapier/Make/n8n. This is infinitely more extensible than competitors' closed ecosystems.

### 4.2 Technical Advantages

1. **Agent Simulation:** Built-in simulation testing (`simulation.go`) allows tenants to test their AI agent before going live. No competitor offers this.

2. **Transcript Analysis:** Benchmark analysis (`analysis.go`) provides quantitative evaluation of agent quality. Unique differentiator.

3. **LLM Response Verification:** Optional verifier layer (`verifier.go`) for hallucination detection. Safety feature competitors lack.

4. **Hybrid Database Support:** SQLite (development) + Postgres (production) + scale-to-zero on Fly.io. Competitors require expensive always-on infrastructure.

5. **SSRF Protection + HMAC Signing:** Enterprise-grade webhook security (three-layer SSRF defense, HMAC-SHA256, idempotency keys). Security posture exceeds what competitors document.

---

## 5. bchat's Competitive Gaps

### 5.1 Current Gaps (As of v1)

1. **No Native SMS Integration:** v1 ships webhooks only. SMS follow-up requires Zapier/n8n bridge. Competitors (Helios AI) offer this natively. Planned for v1.1.

2. **No Native Calendar Integration:** Booking requires external tools. Competitors (Helios AI, Intercom AI) include calendar booking. Planned for future release.

3. **No WhatsApp Integration:** Helios AI offers WhatsApp configuration. bchat has no WhatsApp channel.

4. **Webhook UX Friction:** The "webhook-first" strategy requires tenants to set up Zapier/n8n. This is technical friction that Helios AI avoids with its managed setup. Mitigation: pre-built Zapier/Make templates are planned.

5. **No Native Visibility into Downstream Actions:** If SMS is sent via Zapier, bchat's Admin UI cannot show "SMS sent at 2pm." Mitigation: inbound update API (`PATCH /api/v1/agent/:slug/leads/:id`) is planned.

6. **Documentation and Brand Awareness:** Intercom and Helios AI have established marketing. bchat is early-stage with limited public documentation beyond technical docs.

7. **No Enterprise Tier:** Package 3 at $499-$1,500/mo is the highest tier. No enterprise SLA, dedicated support, or custom deployment options documented.

8. **Scale-to-Zero Trade-off:** Running on Fly.io with `auto_stop_machines` means webhook retries only happen when the machine is awake. Competitors with always-on infrastructure have more reliable retry latency.

### 5.2 Gaps Acknowledged in Documentation

From `prompt.md` (the strategic debate):
- "User Experience Friction: Instead of a customer entering their Twilio API key directly into the bchat Admin UI, they now have to set up a Zapier workflow"
- "Loss of Native UI Visibility: If SMS and Calendar booking happen via Zapier, the bchat Admin UI won't be able to natively show 'SMS Sent' or 'Appointment Booked at 2pm'"

From `plan3_review.md` (adversarial review):
- Path A (platform-managed) vs. Path B (bring-your-own) trade-off was resolved with Path B for v1, accepting higher tenant friction for lower code complexity.

---

## 6. The "Replace Expensive SaaS" Narrative

### 6.1 Supporting Evidence

**The $246/mo Replacement Claim:**

From `plan_biz.md` (line 3):
> "Goal: Transform bchat into a complete 'digital front desk' that replaces Intercom AI ($97/mo), Helios AI ($100/mo), and parts of n8n ($49/mo) -- saving $246/mo per deployment"

From `TUTORIAL.md` (line 29):
> "Business Driver: Replace $246/mo in third-party SaaS (Intercom AI + Helios AI + n8n)"

From `docs_033.md` (line 7):
> "Why: Replaces expensive third-party integrations (Intercom AI at $97/mo, Helios AI at $100/mo) with native webhook support included in your bchat subscription."

**The Cost Comparison:**

| Competitor Stack | Monthly Cost | bchat Equivalent | Savings |
|-----------------|-------------|------------------|---------|
| Helios AI (SMS + Booking) | $100/mo | Webhooks (free) + Zapier (tenant's account) | $100/mo |
| Intercom AI (Chat + Booking) | $97/mo | bchat AI Chat (included in subscription) | $97/mo |
| n8n (Workflow Automation) | $49/mo | Native webhook events (included) | $49/mo |
| **Total** | **$246/mo** | **bchat subscription** | **$246/mo** |

**bchat's Internal Economics:**

From `docs_biz_guide.md` (line 1090):
> "Your total cost per tenant: $5-$30/month -- Margins of 80-90% on the $199-$499/month retail price"

This means bchat can price its service at $149-$299/mo (Package 1) while still offering:
- $246/mo in SaaS replacement value
- 80-90% gross margins
- Room for discounting to undercut competitors

### 6.2 Narrative Strengths

1. **Quantified Value:** The $246/mo figure is specific and verifiable (competitor pricing is public).
2. **Margin Story:** 80-90% margins make the business model attractive for resellers and the Pithom Labs service layer.
3. **Progressive Replacement:** Webhooks (v1) replace n8n. SMS (v1.1) replaces Helios AI SMS. Calendar (future) replaces Intercom booking. Each phase captures more of the $246/mo.
4. **Configuration Advantage:** Unlike competitors that lock tenants into specific workflows, bchat's KB.MD/POLICY.MD approach means the same platform serves restoration companies, plumbers, insurance agencies, etc.

### 6.3 Narrative Weaknesses

1. **Incomplete Replacement Today:** v1 only ships webhooks. The full $246/mo replacement requires SMS + Calendar, which are deferred.
2. **Zapier Dependency:** If tenants need Zapier ($20-$70/mo) to bridge webhooks to SMS/calendar, the savings are reduced.
3. **Managed vs. Self-Service:** Helios AI's zero-config experience is more appealing to non-technical business owners. bchat's webhook-first approach requires more setup.
4. **Brand Trust:** Intercom is a $1B+ company. bchat is a startup. Enterprise buyers may prefer the established vendor.
5. **The "Digital Front Desk" Framing:** From `prompt.md` (line 111):
   > "If you want to be the 'Digital Front Desk' that replaces expensive tools like Intercom ($97/mo) and Helios AI ($100/mo), those competitors usually offer a 'batteries included' experience (Path A)."

   This acknowledges that the "replace expensive SaaS" narrative is strongest when bchat offers the same batteries-included experience, which v1 does not fully deliver.

---

## 7. Sources Referenced

| File | Path | Content |
|------|------|---------|
| Helios AI Workflow | `/home/chaschel/Documents/go/bchat/bugs/033/heliosai_workflow.md` | Helios AI's 5-step onboarding process |
| Integration Plan (Original) | `/home/chaschel/Documents/go/bchat/bugs/033/plan_biz.md` | $246/mo replacement narrative, 3-phase integration plan |
| Revised Plan | `/home/chaschel/Documents/go/bchat/bugs/033/plan2.md` | Scoped to webhooks + SMS, addressed 5 critical + 6 major findings |
| Strategic Debate | `/home/chaschel/Documents/go/bchat/bugs/033/prompt.md` | Webhook-first vs. native integrations, Path A vs. Path B analysis |
| Webhook Documentation | `/home/chaschel/Documents/go/bchat/bugs/033/docs_033.md` | Production webhook feature documentation |
| Plan3 Review | `/home/chaschel/Documents/go/bchat/bugs/033/plan3_review.md` | Adversarial review finding outbox architecture issues |
| Plan5 (Final) | `/home/chaschel/Documents/go/bchat/bugs/033/plan5.md` | Approved implementation plan for webhooks v1 |
| Business Guide | `/home/chaschel/Documents/go/bchat/docs/docs_biz_guide.md` | Pricing model, internal cost structure ($5-$30/tenant) |
| Tutorial | `/home/chaschel/Documents/go/bchat/TUTORIAL.md` | Case study framing $246/mo replacement narrative |
| OM Documentation | `/home/chaschel/Documents/go/bchat/docs/DOCS_OM.md` | Observational Memory benchmark comparisons (OM vs Supermemory vs Zep) |
| RAG Limitations | `/home/chaschel/Documents/go/bchat/docs/DOCS_RAG_MINIMAX25.MD` | RAG pipeline technical limitations analysis |
| Plan Biz Review | `/home/chaschel/Documents/go/bchat/bugs/033/plan_biz_review.md` | Adversarial review of original plan |
| Resilience Guide | `/home/chaschel/Documents/go/bchat/bugs/033/resilience_guide.md` | Neon Postgres resilience patterns (informs production architecture) |
</task_result>
</task>
```

---

## Assistant (Plan · MiMo V2.5 Free · 105.9s)

_Thinking:_

Now I have a comprehensive understanding of the competitive landscape. Let me finalize the outline incorporating all user feedback:

1. Summarize bugs/033 plans (not verbatim)
2. Blend of "do" and "think" exercises
3. In-depth competitor analysis (product direction)
4. Conceptual flow through code (not line-by-line)
5. Markdown only deliverable

Let me present the final refined outline.

Now I have everything needed. Let me present the final refined outline.

---

# BCHAT TUTORIAL v3: Final Outline

## Building Production-Grade SaaS: From Webhook Integrations to Competing with Expensive Tools

---

## STRUCTURE

**8 Parts | 18 Chapters | ~120 Sections**

Each chapter follows a consistent structure:
- **Concept** — what and why (product thinking)
- **Walkthrough** — conceptual flow through the codebase
- **Do This** — hands-on exercise in the code
- **Think About This** — critical thinking prompt (no single right answer)

---

## PART I: PRODUCT THINKING — Before You Write a Line

### Chapter 1: The Market Opportunity

**1.1 The $246/mo Problem**
- Small service businesses pay for 3 fragmented tools:
  - Intercom AI ($97/mo): AI chat agent for customer support and booking
  - Helios AI ($100/mo): Managed "digital front desk" with WhatsApp + Cal.com
  - n8n Cloud ($49/mo): Workflow automation to connect the tools
- Stitching them together costs time, money, and reliability

**1.2 What Each Competitor Actually Delivers**

| | Intercom AI | Helios AI | n8n |
|---|---|---|---|
| **Core** | AI chat agent (Fin) | Managed AI assistant | Workflow automation |
| **Pricing** | $97/mo per seat | $100/mo per business | $49/mo per workflow tier |
| **Setup** | Self-service | Managed (white-glove) | Self-service (technical) |
| **KB Config** | Built-in articles | Managed loading | N/A |
| **SMS** | Yes | Yes (WhatsApp) | Via automation |
| **Calendar** | Yes | Yes (Cal.com) | Via automation |
| **Extensibility** | API + marketplace | Closed | 400+ integrations |
| **Open Source** | No | No | Yes |

**1.3 bchat's Positioning: The All-in-One Alternative**
- AI chat agent with RAG pipeline (replaces Intercom AI)
- Configurable via markdown files, self-service (replaces Helios AI's managed model)
- Native webhook events (replaces n8n for lead routing)
- 80-90% gross margins: $5-$30/tenant/month internal cost vs. $149-$499 retail

**1.4 The Strategic Pivot in bugs/033**
- Original plan: Build native Twilio SMS, Google Calendar, Outlook integrations (2-3 months)
- Pivot: Webhook-first approach ships in weeks, integrates with everything
- Key insight from `bugs/033/prompt.md`: "Personal AI agents (Claude Co-Work, Hermes) can consume webhook events — bchat becomes the source of truth, not one tool in a chain"
- Summary of 5 plan iterations: each killed assumptions the previous one made

**Do This:**
- Read `bugs/033/plan_biz.md` (first 100 lines) — identify the 3-phase integration roadmap
- Read `bugs/033/plan_biz_review.md` — count the critical vs. major findings
- Read `bugs/033/prompt.md` — find the exact paragraph where the pivot happens

**Think About This:**
- Why is "scope management" more valuable than "coding speed" for a product?
- If bchat builds native SMS + Calendar, what maintenance cost does that create forever?
- Is "webhook-first" a compromise or an upgrade? Under what conditions?

---

### Chapter 2: Architecture as a Product Decision

**2.1 Why Architecture Decisions Are Product Decisions**
- Every technical choice constrains or enables future features
- The "Path A vs. Path B" debate in bugs/033: platform-managed vs. webhook-first
- Path A: bchat owns Twilio subaccounts, auto-provisions per tenant (batteries included)
- Path B: bchat emits events, tenant connects via Zapier/Make (extensible but more friction)
- Decision: Path B for v1, Path A deferred — speed to market won

**2.2 The "Same Transaction" Trap**
- plan3 claimed: "insert event and deliver webhook in the same transaction"
- plan3_review killed it: "unimplementable — Store layer has no transaction API"
- **Lesson:** If a plan contradicts the data model, it's wrong. Verify assumptions against code before committing.

**2.3 The Pre-Claimed Events Pattern**
- Race condition: Insert event → immediate dispatch → poller also claims it → double delivery
- Solution: Insert with `status='processing'` and `claimed_at=now()` — poller skips it
- Why this works: the event is "claimed" before it's visible to anyone else

**2.4 Idempotency as a First-Class Concern**
- Network errors after commit are indistinguishable from non-delivery
- Key: `hash(tenantID, leadID, eventType, integrationID)`
- PostgreSQL unique constraint (`23505`) treated as success — retry sees duplicate, considers done
- **Lesson:** Design for the failure mode, not the happy path.

**2.5 The Review Culture**
- Every plan had an adversarial review with structured checklists
- `plan_biz_review.md`: 5 critical + 6 major findings → REWORK
- `plan3_review.md`: Found outbox architecture flaw → REWORK
- `plan4_review.md`: APPROVED with 4 amendments
- **Lesson:** Code review is not about finding bugs. It's about finding wrong assumptions.

**Do This:**
- Compare `plan3_review.md` (REWORK) with `plan4_review.md` (APPROVED)
- Identify the specific language that changed between plans
- Trace how the "pre-claimed events" pattern is implemented in `integrations.go`

**Think About This:**
- Why did plan3 fail but plan4 succeed? What was fundamentally different?
- If you were the reviewer, what would you have caught in plan3?
- How does "adversarial review" differ from "code review"? Which catches more?

---

## PART II: DATABASE DESIGN — Schema as Contract

### Chapter 3: Multi-Tenant Schema Design

**3.1 The Two New Tables (0.31 migrations)**

```sql
-- agent_integrations: "What webhooks does this tenant have?"
-- One webhook per tenant per type (UNIQUE constraint)
-- ON DELETE CASCADE: tenant deleted → integrations cascade

-- agent_events: "What needs to be delivered?"
-- Pre-claimed on insert (status='processing')
-- Idempotency key with UNIQUE index
-- Lease-based recovery (claimed_at < now() - 300)
```

Conceptual walkthrough of the schema: what each column means, why it exists, what would break if removed.

**3.2 Design Decisions Explained**

| Decision | Choice | Why |
|----------|--------|-----|
| Integration uniqueness | `UNIQUE(tenant_id, integration_type)` | One webhook per type per tenant — simple, enforced |
| Event status on insert | `status = 'processing'` | Pre-claiming eliminates race condition |
| Idempotency key | `hash(tenantID, leadID, eventType, integrationID)` | Deterministic deduplication at DB level |
| Cascade deletes | `ON DELETE CASCADE` | No orphaned data when tenant is deleted |
| Lease expiry | `claimed_at < now() - 300` | 5-minute recovery for crashed processes |

**3.3 SQLite vs. PostgreSQL: Same Schema, Different SQL**
- SQLite: `datetime('now')`, `INTEGER` for booleans, `TEXT` for JSON
- PostgreSQL: `NOW()`, `BOOLEAN`, `JSONB`
- The abstraction layer handles dialect differences — but you need to understand both

**3.4 State Machine Thinking**
- Event states: `processing` → `delivered` / `dead`
- `processing`: claimed by dispatcher, being delivered
- `pending`: retry queue (not used in v1 but reserved)
- `delivered`: webhook returned 2xx
- `dead`: exhausted all retry attempts
- Every "stuck" state needs a recovery path (lease expiry)

**Do This:**
- Write the SQL to find all events stuck in `processing` for more than 5 minutes
- Write the SQL to count events by status for a specific tenant
- Design a schema for a new feature: "event delivery log" (what was sent, when, response code)

**Think About This:**
- `ON DELETE CASCADE` is convenient but dangerous — what's the blast radius of deleting a tenant with 10,000 pending events?
- Why not use PostgreSQL's `LISTEN/NOTIFY` instead of polling? What would change?
- How would you handle a schema change that affects 1M+ rows in production?

---

### Chapter 4: The Store Layer Abstraction

**4.1 The Driver Interface**
- `store/driver.go`: 60+ methods covering all entity CRUD
- Adding a new entity means: types in `agent.go` → interface in `driver.go` → implementations in `sqlite/agent.go` and `postgres/agent.go`
- The interface is the contract — implementations can diverge

**4.2 SQLite Implementation**
- Uses `modernc.org/sqlite` (pure Go, no CGO)
- `INSERT OR IGNORE` for idempotent inserts
- Busy timeout: 10 seconds for write contention
- Tradeoff: simple, fast for dev, but no concurrent writes

**4.3 PostgreSQL Implementation**
- Uses `pgx/v5` (high-performance, pure Go driver)
- `ON CONFLICT (idempotency_key) DO NOTHING` — native conflict handling
- `FOR UPDATE SKIP LOCKED` for claim operations — non-blocking claims
- Connection pool: 10 max open, 5 max idle, 5min lifetime

**4.4 PostgreSQL Resilience**
- Transient error codes from Neon: `57P01`, `57P02`, `57P03`, `57P04`
- Retry with exponential backoff + jitter
- Circuit breaker pattern for connection failures
- Why this matters: Neon is serverless, connections can drop mid-query

**Do This:**
- Compare the `ClaimEvents` implementation in SQLite vs. PostgreSQL
- Identify the 3 lines that differ between them
- Explain why SQLite can't use `FOR UPDATE SKIP LOCKED`

**Think About This:**
- Why use `pgx/v5` directly instead of `database/sql` with `lib/pq`? What does pgx give you?
- If you had to add MySQL support, what would break? (Hint: `agent_tenants` table doesn't exist in MySQL schema)
- How does the driver abstraction hide differences but not eliminate them?

---

## PART III: IMPLEMENTATION — Building the Feature

### Chapter 5: Backend — The Webhook Dispatcher

**5.1 Conceptual Flow: What Happens When a Lead is Captured**

```
Chat conversation
    ↓
Lead extraction (name + email/phone)
    ↓
Insert into agent_events (status='processing', pre-claimed)
    ↓
Immediate dispatch attempt (fire-and-forget goroutine)
    ↓
    ├── Success: status → 'delivered'
    └── Failure: status stays 'processing'
                    ↓
            Poller claims it (every 30s)
                    ↓
            Retry with backoff
                    ↓
            ├── Success: status → 'delivered'
            └── Exhausted: status → 'dead'
```

**5.2 SSRF Protection: Security by Default**
- Webhook URLs are user-provided → SSRF risk (internal IPs, metadata endpoints)
- Three-layer defense: URL parsing → DNS resolution → IP validation
- Copied from `plugin/webhook/webhook.go` (unexported helpers) — deliberate duplication
- **Lesson:** Security code that's hard to import is better than no security code.

**5.3 HMAC Signing: Proving Identity**
- Each webhook includes `X-Bchat-Signature` header
- Signature: `HMAC-SHA256(secret, payload + timestamp)`
- Tenant provides the secret; bchat signs; recipient verifies
- **Lesson:** Authentication is about trust boundaries. HMAC proves the payload came from bchat.

**5.4 The Poller: Background Recovery**
- Runs every 30 seconds as a goroutine
- Claims up to 10 events where `status='processing'` and `claimed_at < now() - 300`
- Dispatches each webhook, updates status
- Tradeoff: awake-only retries (Fly.io scale-to-zero limitation)

**Do This:**
- Trace the flow from lead capture to webhook delivery through the code
- Find the exact line where HMAC signature is computed
- Find the exact line where SSRF validation blocks an internal IP

**Think About This:**
- `http.Client{Timeout: 10 * time.Second}` — what happens if the webhook endpoint is slow but not down?
- The poller runs every 30 seconds. What's the maximum latency for webhook delivery?
- How does `kill_timeout = "30s"` in fly.toml interact with in-flight deliveries?

---

### Chapter 6: Frontend — Admin UI for Integrations

**6.1 Conceptual Flow: How the UI Connects to the Backend**

```
IntegrationsSection.tsx
    ↓
MobX store (agentAdmin.ts)
    ↓
HTTP request to /api/v1/agent/:slug/integrations
    ↓
Handler (handlers.go) → Service (service.go) → Store (agent.go)
    ↓
Response flows back up
```

**6.2 The Component Structure**
- `IntegrationsSection.tsx`: 296-line React component
- Lists integrations with status chips (active/inactive)
- Create/edit modal with URL, secret, label fields
- Test button → sends test event → shows result
- Delete confirmation dialog

**6.3 Pattern Recognition**
- Compare `IntegrationsSection.tsx` with other `AgentAdminSections/` components
- What patterns repeat? Component structure, state management, API calls
- **Lesson:** UI patterns are templates. Once you recognize them, new features are copy-paste-refactor.

**6.4 The PII Masking Decision**
- `HandleListEvents` returns `safeEvent` struct — no `payload` field
- Payload contains name, email, phone (PII)
- Admin UI shows event status and type, not content
- **Lesson:** API responses are security boundaries. What you expose in a list endpoint can leak PII.

**Do This:**
- Compare `IntegrationsSection.tsx` with `TicketsSection.tsx` (or another admin section)
- Identify the 3 patterns that repeat across all admin sections
- Design the UI for a new integration type (e.g., Slack)

**Think About This:**
- Why MobX instead of Redux? What does `makeAutoObservable` give you?
- How would you add real-time event status updates (WebSocket vs. polling)?
- If you were building this for Helios AI's "zero-config" model, what would the UI look like?

---

### Chapter 7: Testing — What Catches What

**7.1 The Bugs That Slipped Through**
- Bug 1 (code_review.md): Idempotency key argument swap — all events deduplicated to one integration
- Bug 2 (code_review.md): Poller context cancelled before delivery completes
- These were caught by adversarial code review, not tests

**7.2 What Was Tested**
- HMAC signing correctness
- SSRF protection (internal IP blocking)
- Idempotency key determinism
- Integration test: create → list → update → delete

**7.3 The Testing Pyramid**

```
        ┌─────────┐
        │  E2E    │  ← few, slow, high confidence
        ├─────────┤
        │Integration│ ← test boundaries (HTTP, DB)
        ├─────────┤
        │  Unit   │  ← many, fast, test logic
        └─────────┘
```

**7.4 What Tests Would Catch Bugs 1 & 2**
- Bug 1: Integration test that creates 2 integrations, dispatches to both, verifies both receive events
- Bug 2: Unit test that verifies poller dispatch completes before context cancellation

**Do This:**
- Write a test that would catch the argument-order bug (Bug 1)
- Write a test that would catch the context-cancellation bug (Bug 2)
- Run `go test ./server/router/api/v1/agent/ -run TestIntegration` and analyze the output

**Think About This:**
- Code review found bugs that tests missed. What's the relationship between review and testing?
- How do you write tests that catch argument-order bugs without testing implementation details?
- What's the cost of a bug that reaches production vs. a bug caught in review?

---

## PART IV: DATABASE MIGRATION — SQLite to PostgreSQL

### Chapter 8: Migration Strategy

**8.1 The Two-Database Reality**
- SQLite: development (zero-config, single file, fast)
- PostgreSQL: production (Neon serverless, concurrent access, JSONB)
- Same migration files, different SQL dialects

**8.2 Migration File Structure**
```
store/migration/
├── sqlite/0.31/
│   ├── 00__agent_integrations.sql
│   └── 01__agent_events.sql
├── postgres/0.31/
│   ├── 00__agent_integrations.sql
│   └── 01__agent_events.sql
└── {sqlite,postgres}/LATEST.sql  (cumulative schema)
```

**8.3 Dialect Differences**

| Feature | SQLite | PostgreSQL |
|---------|--------|------------|
| Auto-increment | `INTEGER PRIMARY KEY AUTOINCREMENT` | `SERIAL` or `GENERATED ALWAYS AS IDENTITY` |
| Timestamps | `datetime('now')` | `NOW()` |
| Booleans | `INTEGER` (0/1) | `BOOLEAN` |
| JSON | `TEXT` with `json_valid()` | `JSONB` |
| Upsert | `INSERT OR IGNORE` | `ON CONFLICT DO NOTHING` |
| Row locking | Table-level | `FOR UPDATE SKIP LOCKED` |

**8.4 LATEST.sql: The Cumulative Schema**
- 1067 lines of complete schema for fresh installs
- Must be regenerated after every migration change
- Validation: `task validate:migrations` checks LATEST.sql matches incremental files

**Do This:**
- Write the SQLite migration for a new table: `agent_webhook_logs`
- Write the PostgreSQL version of the same migration
- Update LATEST.sql for both databases
- Run `task validate:migrations` to verify

**Think About This:**
- Why use `ON DELETE CASCADE` for tenant-scoped data? What's the alternative?
- What happens if a migration fails midway? (SQLite: no transaction rollback; PostgreSQL: transaction rollback)
- How would you handle a migration that takes 10 minutes on a large table?

---

### Chapter 9: Running Migrations in Production

**9.1 Programmatic Migration**
- `store/migrator.go` runs on server startup
- Checks `migration_history` table for applied versions
- Applies only new migrations
- No separate migration step needed — just deploy and restart

**9.2 Neon Connection Configuration**
- `DATABASE_URL` with SSL mode required
- `pgx/v5` auto-appends `default_query_exec_mode=simple_protocol`
- Connection pool managed by Neon (not application)
- Auto-scaling: Neon suspends when idle, resumes on connect

**9.3 Validation Commands**
```bash
task validate:migrations   # LATEST.sql sync check
task validate:schema       # Schema + CRUD validation
```

**Do This:**
- Run `task validate:schema` on the current codebase
- Identify any validation warnings
- Create a test migration, run it, verify it appears in `migration_history`

**Think About This:**
- Neon suspends when idle. What happens to in-flight queries during suspension?
- How does `simple_protocol` affect query performance vs. extended protocol?
- What's your rollback strategy if a migration corrupts data?

---

## PART V: DEPLOYMENT — Getting to Production

### Chapter 10: Docker Containerization

**10.1 The Three-Stage Dockerfile**

```
Stage 1: Frontend (Node.js)     → npm ci && npm run build
Stage 2: Backend (Go)           → go build -o memos
Stage 3: Runtime (Ubuntu 24.04) → copy binaries + assets
```

Each stage is isolated — build dependencies don't reach the runtime image.

**10.2 Security in the Container**
- Non-root user (`memos`) via `gosu`
- Read-only filesystem (except data directory)
- `_FILE` suffix support for Docker/K8s secrets
- `entrypoint.sh` processes secrets before starting

**10.3 Supercronic: Container Cron**
- Replaces `crond` (not available in minimal containers)
- External process — survives Go panics
- `build/crontab` defines scheduled tasks
- Tradeoff: retries only run while the machine is awake

**Do This:**
- Build the Docker image locally: `docker build -f Dockerfile.pg.fly .`
- Inspect the image layers: `docker inspect <image>`
- Run the container and verify the health check

**Think About This:**
- Why Ubuntu instead of Alpine? (glibc vs. musl, LanceDB compatibility)
- What's the security implication of `chmod +x /entrypoint.sh`?
- How does `gosu` differ from `su`? Why is it preferred in containers?

---

### Chapter 11: Fly.io Deployment

**11.1 Configuration Anatomy**
- `fly.toml`: app name, region, build config, VM specs
- `auto_stop_machines = "stop"`: scale to zero when idle
- `auto_start_machines = true`: wake on request
- `kill_timeout = "30s"`: graceful shutdown window

**11.2 Secrets Management**
- `fly secrets set KEY=VALUE`: encrypted at rest, injected as env vars
- `_FILE` suffix: `ENCRYPTION_MASTER_KEY_FILE=/run/secrets/key`
- No secrets in Docker images or git history

**11.3 The Deployment Flow**
```bash
fly auth login
fly launch          # first time
fly secrets set ...  # set secrets
fly deploy          # build + deploy
fly status          # check health
fly logs            # stream logs
```

**11.4 Neon Integration**
- `DATABASE_URL` from Neon dashboard
- SSL mode required
- Connection pooling by Neon
- Auto-suspend/resume on idle/activity

**Do This:**
- Deploy to Fly.io with a Neon database
- Verify the health check at `/healthz`
- Scale down to 0 machines, then hit the API and verify it wakes up

**Think About This:**
- `kill_timeout = "30s"` — what happens during those 30 seconds?
- How does `auto_stop_machines` interact with the poller goroutine?
- What's the cold-start time for a stopped Fly.io machine?

---

### Chapter 12: Pre-Deployment Validation

**12.1 The Validation Chain**
```bash
scripts/validate-env-chain.sh      # env vars
scripts/validate-db-migrations.sh   # migration integrity
task validate:schema                # schema + CRUD
task build:rag                      # build with RAG
fly deploy                          # deploy
```

**12.2 Why Validate Before Deploy**
- Failed deploy = downtime + rollback + investigation
- Blocked deploy = delayed but safe
- Pre-deploy catches 80% of issues before they reach production

**Do This:**
- Run the full validation chain on the current codebase
- Introduce a deliberate error (e.g., missing env var) and see how validation catches it
- Add a new validation step to the chain

**Think About This:**
- What's the cost of a failed deploy vs. a blocked deploy?
- How would you add a smoke test that runs after deploy?
- If validation passes but the deploy fails, what went wrong?

---

## PART VI: PRODUCTION OPERATIONS — Keeping It Running

### Chapter 13: Monitoring & Observability

**13.1 Health Check**
- `/healthz` endpoint: checks database + LanceDB connectivity
- Returns 200 OK or 503 Service Unavailable
- Used by Fly.io for routing and restart decisions

**13.2 Logs to Watch**
```bash
"webhook delivered"           # success
"webhook delivery failed"     # failure
"event claimed"               # poller activity
"event moved to dead"         # exhausted retries
```

**13.3 Metrics to Track**

| Metric | Target | Action if breached |
|--------|--------|-------------------|
| Webhook delivery success rate | > 99% | Investigate endpoints |
| Average webhook latency | < 2s | Check network/endpoint |
| Dead letter rate | < 0.1% | Review tenant configs |
| Database connections | < 80% | Scale pool |
| LLM cost per tenant | < budget | Review usage |

**Do This:**
- Set up log monitoring for webhook delivery events
- Create a dashboard showing delivery success rate over time
- Write an alert rule for dead letter rate > 1%

**Think About This:**
- The poller runs every 30 seconds. What's the maximum latency for webhook delivery?
- How does Fly.io's `auto_stop` interact with monitoring? (Metrics stop when machine stops)
- What would you monitor if bchat had 10x more tenants?

---

### Chapter 14: Incident Response

**14.1 Severity Levels**
- P0: All webhook delivery down (all tenants affected)
- P1: Single tenant webhook failing
- P2: Event polling delayed (events still delivered, just slow)
- P3: Cosmetic issue in admin UI

**14.2 Runbook: Events Stuck in "processing"**
1. Check poller logs
2. Check event age and `claimed_at` timestamps
3. Check lease expiry (`claimed_at < now() - interval '5 minutes'`)
4. Trigger manual cron if needed
5. Restart the machine if poller is stuck

**14.3 Post-Mortem Template**
- Timeline: what happened, when, how long
- Root cause: why it happened
- Impact: who was affected
- Action items: what we'll change (with owners)

**Do This:**
- Write a runbook for a new failure mode: "webhook endpoint returns 429 (rate limited)"
- Write a post-mortem for a simulated incident
- Identify the 3 most likely failure modes for the webhook system

**Think About This:**
- A tenant reports "I'm not getting webhook deliveries" — walk through the diagnostic steps
- What information do you need from the tenant? What can you get from the database?
- How do you balance "fix it now" with "understand why it happened"?

---

## PART VII: CRITICAL THINKING — The Meta-Skills

### Chapter 15: Reading Code You Didn't Write

**15.1 Navigation Strategy**
- Start at the entry point (`bin/memos/main.go`)
- Follow the request flow: HTTP → Handler → Service → Store
- Read the tests (they document expected behavior)
- Check the git log (commit messages explain "why")

**15.2 Pattern Recognition**
- Handler: extract context → validate input → call service → return response
- Store: interface definition → driver implementation → migration
- Frontend: MobX store → API call → component render

**15.3 When Code Confuses You**
- Don't read sequentially — follow the call graph
- Use `grep` to find where a function is called
- Read the smallest possible unit that makes sense in isolation

**Do This:**
- Trace the full request flow for `POST /api/v1/agent/:slug/integrations`
- Identify every function call from HTTP handler to database insert
- Draw the call graph on paper

**Think About This:**
- Which layer of the code is hardest to understand? Why?
- If you had to explain the webhook system to a non-technical person, what would you say?
- What's the relationship between code complexity and bug likelihood?

---

### Chapter 16: Evaluating Technical Decisions

**16.1 The Tradeoff Framework**

| Dimension | Option A | Option B |
|-----------|----------|----------|
| Development time | ? | ? |
| Operational complexity | ? | ? |
| Cost at scale | ? | ? |
| Failure modes | ? | ? |
| Reversibility | ? | ? |

**16.2 Case Study: Webhooks vs. Native Integrations**

| Dimension | Webhooks (Path B) | Native (Path A) |
|-----------|-------------------|-----------------|
| Dev time | Weeks | Months |
| Operational complexity | Lower | Higher |
| Cost at scale | Same | Higher maintenance |
| Failure modes | External | Internal + external |
| Reversibility | High | Low |

**16.3 The Second System Effect**
- bugs/033 went through 5 plan iterations
- Each iteration caught assumptions the previous one made
- **Lesson:** Your first plan is your worst plan. Build in adversarial review.

**Do This:**
- Evaluate a technical decision you're facing using the tradeoff framework
- Identify which dimension matters most for your context
- Write a one-paragraph justification for your choice

**Think About This:**
- Is "webhook-first" a compromise or an upgrade? Under what conditions?
- When should you build native vs. emit events? What's the decision criteria?
- How do you know when a tradeoff is acceptable vs. a mistake?

---

### Chapter 17: Writing Code That Survives

**17.1 Defensive Programming**
- Idempotency keys: network errors are indistinguishable from non-delivery
- Lease expiry: crashed processes leave stale state
- SSRF protection: user input is hostile until validated

**17.2 Operational Visibility**
- Structured logging with context (tenant_id, event_id, integration_id)
- Event status tracking (processing → delivered/dead)
- Audit trail (created_at, claimed_at, attempts)

**17.3 Graceful Degradation**
- If webhook delivery fails, log and continue
- If poller is down, events wait (they're in the database)
- If database is down, return 503 (fail fast, don't queue)

**Do This:**
- Add structured logging to a function that currently uses `fmt.Println`
- Design an audit trail for a new feature
- Write error handling that degrades gracefully instead of crashing

**Think About This:**
- What's the difference between "defensive programming" and "paranoid programming"?
- How much logging is too much? When does logging become noise?
- What's the cost of graceful degradation vs. fail-fast?

---

### Chapter 18: When to Stop and Ship

**18.1 The MVP Mindset**
- v1: Webhooks only (SMS deferred to v1.1)
- "Batteries included" with zero tenant friction
- Perfect is the enemy of shipped

**18.2 Known Limitations (Documented Honestly)**
- Awake-only retries (scale-to-zero limitation)
- SQLite write lock (acceptable at v1 scale)
- No native SMS/Calendar (webhook bridge required)

**18.3 The Review Culture**
- Every plan had an adversarial review
- Reviews used structured checklists (critical, major, minor)
- **Lesson:** Code review is not about finding bugs. It's about finding wrong assumptions.

**Do This:**
- Read `bugs/033/plan4_review.md` — identify the 4 amendments (A-D)
- For each amendment, explain why it was necessary
- Write a review comment for one of the findings

**Think About This:**
- How do you know when a feature is "good enough" to ship?
- What's the cost of shipping too early vs. too late?
- How do you balance "known limitations" with "marketing claims"?

---

## APPENDICES

### Appendix A: bugs/033 File Reference
- Complete file listing with purpose and key takeaway
- Timeline of plan iterations (plan_biz → plan5)
- Review verdicts and what they caught

### Appendix B: API Endpoint Reference
- All webhook-related endpoints with request/response examples
- HMAC signature verification guide
- Error codes and meanings

### Appendix C: Database Schema Reference
- Complete `agent_integrations` and `agent_events` schema
- Index definitions and query patterns
- Migration file locations

### Appendix D: Environment Variables
- Complete list with defaults and descriptions
- Neon-specific configuration
- Fly.io secrets management

### Appendix E: Competitor Analysis
- Detailed profiles: Intercom AI, Helios AI, n8n, Zapier
- Feature comparison matrix
- Pricing analysis and cost positioning
- bchat's competitive advantages and gaps

### Appendix F: Glossary
- Technical terms used throughout the tutorial
- Acronyms and abbreviations

---

## DELIVERABLE FORMAT

**Markdown only.** Organized as:

```
bchat-tutorial/
├── README.md                    # Overview + learning paths
├── PART-I-PRODUCT-THINKING/
│   ├── 01-market-opportunity.md
│   └── 02-architecture-decisions.md
├── PART-II-DATABASE-DESIGN/
│   ├── 03-multi-tenant-schema.md
│   └── 04-store-layer.md
├── PART-III-IMPLEMENTATION/
│   ├── 05-webhook-dispatcher.md
│   ├── 06-admin-ui.md
│   └── 07-testing.md
├── PART-IV-DATABASE-MIGRATION/
│   ├── 08-migration-strategy.md
│   └── 09-production-migrations.md
├── PART-V-DEPLOYMENT/
│   ├── 10-docker.md
│   ├── 11-flyio.md
│   └── 12-pre-deploy-validation.md
├── PART-VI-OPERATIONS/
│   ├── 13-monitoring.md
│   └── 14-incident-response.md
├── PART-VII-CRITICAL-THINKING/
│   ├── 15-reading-code.md
│   ├── 16-evaluating-decisions.md
│   ├── 17-writing-survivable-code.md
│   └── 18-when-to-ship.md
└── APPENDICES/
    ├── A-bugs033-reference.md
    ├── B-api-reference.md
    ├── C-schema-reference.md
    ├── D-environment-variables.md
    ├── E-competitor-analysis.md
    └── F-glossary.md
```

---

Does this final outline meet all your requirements? Before I proceed to implementation, I want to confirm:

1. **Balance**: Parts I-VII are roughly equal depth — not too deep on any one area
2. **Product thinking**: Every chapter opens with "why" before "how"
3. **bugs/033 as thread**: The webhook feature runs through the entire tutorial as the hands-on vehicle
4. **Competitor analysis**: In-depth in Ch 1 + Appendix E, referenced throughout
5. **Critical thinking**: Every chapter ends with "Think About This" prompts
6. **Do + Think blend**: Each chapter has both "Do This" (hands-on) and "Think About This" (analysis)
7. **Conceptual flow**: Code walkthroughs trace request flow, not line-by-line
8. **Markdown only**: Clean file structure, ready for you to convert to PDF

Ready to proceed with implementation?

---

## User

add another chapter where the learner should make use of RAG feature of bchat to learn the codebase itself, what it can and cannot do, this is how we dogfood our thinking and practices and walk the walk, think how RAG can help in implementing bugs/033 features with RAG and other AI techniques if applicable, make it imaginative, engaging, think like a business person

---

## Assistant (Plan · MiMo V2.5 Free · 206.6s)

_Thinking:_

The user wants me to add a new chapter where the learner uses RAG to learn the bchat codebase itself. This is dogfooding - using the product's own capabilities to learn about the product. Let me explore the RAG capabilities in bchat to understand what's possible, then craft a chapter that's imaginative, engaging, and business-minded.

Let me explore the RAG pipeline, the knowledge base system, and how the agent interacts with documents.

Let me explore the RAG pipeline and how it could be used for self-learning.

**Tool: task**

**Input:**
```json
{
  "description": "Explore bchat RAG pipeline capabilities",
  "prompt": "Thoroughly explore the bchat RAG pipeline to understand what it can and cannot do:\n\n1. Read `server/router/api/v1/agent/vectordb.go` - the interface\n2. Read `server/router/api/v1/agent/vectordb_lance.go` - LanceDB implementation\n3. Read `server/router/api/v1/agent/chunker.go` - document chunking\n4. Read `server/router/api/v1/agent/embedding.go` - embedding providers\n5. Read `server/router/api/v1/agent/fusion_engine.go` - hybrid search\n6. Read `server/router/api/v1/agent/parser.go` - KB/Policy/Script parsing\n7. Read `server/router/api/v1/agent/service.go` lines around RAG search flow\n8. Read any docs about RAG capabilities: `docs/DOCS_RAG_PIPELINE.MD`, `docs/DOCS_HYBRID_SEARCH.MD`\n9. Read the KB template files to understand what goes into the knowledge base\n10. Look at how queries are processed and results returned\n\nFocus on:\n- What types of content can be indexed?\n- How does chunking work?\n- How does hybrid search combine vector + BM25?\n- What are the limitations?\n- How could a developer use this to learn about the codebase itself?\n- What creative applications are possible?\n\nReturn a comprehensive summary of RAG capabilities, limitations, and creative use cases.",
  "subagent_type": "explore"
}
```

**Output:**
```
<task id="ses_0b32bb33dffeB4UqA5VZHAFHbe" state="completed">
<task_result>
I now have a thorough understanding of the entire RAG pipeline. Here is my comprehensive analysis.

---

# Comprehensive Analysis: bchat RAG Pipeline

## 1. Architecture Overview

The RAG pipeline is a multi-stage system: **Upload** (KB.MD / POLICY.MD) -> **Parse** (annotation extraction) -> **Chunk** (heading-based splitting) -> **Embed** (vectorization) -> **Store** (LanceDB / Memory) -> **Retrieve** (hybrid search at query time) -> **Fuse** (combine with Observational Memory) -> **Generate** (LLM response with focused context).

---

## 2. What Types of Content Can Be Indexed

### Content Sources
Only **KB.MD** and **POLICY.MD** files are chunked and indexed into the vector database. SCRIPT.MD is **not** indexed -- it is included verbatim in the system prompt as a "Conversation Guide".

### Annotation Types Parsed (from `parser.go`)

**KB.MD annotations:**
| Annotation | Data Type | Description |
|---|---|---|
| `@service` | `AgentService` | Services/products with code, emergency flag, response time |
| `@faq` | `AgentFAQ` | Q&A pairs with code identifier |
| `@exclusion` | `AgentExclusion` | Services NOT provided, with exception rules and referrals |
| `@coverage` | `AgentCoverage` | Geographic/service areas (included/excluded) |
| `@safety` | `AgentSafetyProtocol` | Emergency safety procedures with trigger intents |
| `@section` | `AgentKBSection` | Generic content sections with type classification |

**POLICY.MD annotations:**
| Annotation | Data Type | Description |
|---|---|---|
| `@identity` | `ParsedIdentity` | Role, tone, brand voice, guidelines |
| `@intent` | `AgentIntent` | Customer intent categories with urgency, examples, counter-examples |
| `@rule` | `AgentRule` | Behavioral rules with priority |
| `@thresholds` | `AgentAudience` | Emergency urgency and escalation confidence thresholds |
| `@settings` | `AgentAudience` | Feature flags like `require_contact_on_fallback` |

### Unstructured Content
The system handles **unstructured content** (markdown without annotations). When no structured annotations are found (`IsStructured = false`), the content is still chunked and indexed. The system forces RAG mode for unstructured content since there is nothing to build a long-context prompt from.

---

## 3. How Chunking Works (`chunker.go`)

### Chunking Strategy: Heading-Based Hierarchical Splitting

The chunker uses a three-level hierarchical approach:

1. **Split by H2 headers (`##`)** -- Primary split points
2. **Split by H3 headers (`###`)** -- If H2 sections exceed max tokens
3. **Split by paragraphs** -- If H3 subsections still exceed max tokens
4. **Split by sentences** -- Fallback for very long paragraphs
5. **Hard character limit** -- Last resort for minified/garbage content

### Chunk Size Parameters (Provider-Dependent)

| Provider | Max Tokens | Min Tokens | Overlap |
|---|---|---|---|
| `openrouter` | 1000 | 200 | 50 |
| `local` | 150 | 30 | 50 |
| `mock` | 500 | 50 | 50 |

Token estimation is approximate: `len(content) / 4`.

### Pre-Processing: Content Sanitization

Before chunking, the pipeline applies **rigorous sanitization**:
- **UTF-8 sanitization** -- removes invalid byte sequences
- **HTML script/style removal** -- strips `<script>` and `<style>` tags
- **Boilerplate detection** -- identifies and removes Google Tag Manager, analytics scripts, minified JS/CSS
- **Garbage chunk filtering** -- rejects chunks dominated by minified code (lines >300 chars with space ratio < 5%)

### Post-Processing
- **Small chunk merging** -- chunks below `minTokens` are merged with neighbors
- **Chunk overlap** -- 50-token overlap added between consecutive chunks (prepends last N chars from previous chunk, respecting sentence boundaries)
- **Chunk ID format** -- `{tenantID}:{audience}:{contentType}:{code}`

### DocumentChunk Structure
```
ID            string   // Unique: tenantID:audience:type:code
TenantID      int32
AudienceType  string
ContentType   string   // kb_section, policy_section
Title         string
Content       string
Code          string
IsEmergency   bool
IsActive      bool
Priority      int32
SourceVersion int32
Embedding     []float32
IndexedAt     time.Time
```

---

## 4. How Hybrid Search Combines Vector + BM25

### Two Implementations

**A. LanceDB (Production)** -- `vectordb_lance.go`
- Uses LanceDB's built-in Tantivy-powered FTS engine
- Creates FTS indexes on `content` and `title` columns
- Creates BTree index on `tenant_id` for filtering
- Creates IVF-PQ vector index (lazy -- requires training data, created after first Insert)
- Hybrid search runs vector search + FTS search in parallel, fetches 2x topK candidates from each source, then fuses scores

**B. MemoryVectorDB (Development/Testing)** -- `vectordb.go`
- Implements BM25 from scratch (k1=1.2, b=0.75)
- Rebuilds the BM25 index on every search query (no persistent index)
- Tokenizes by lowercasing, splitting on whitespace/punctuation, filtering tokens < 2 chars
- No stop words, no stemming, no linguistic processing

### Score Fusion: Linear Combination

```
hybrid_score = vector_weight * vector_score + text_weight * bm25_score
```

Default weights: **vector = 0.7, text = 0.3**. Configurable via:
- `HYBRID_SEARCH_ENABLED=true`
- `HYBRID_VECTOR_WEIGHT=0.7`
- `HYBRID_TEXT_WEIGHT=0.3`

### Score Normalization
- **Vector scores**: LanceDB returns L2 distance, converted to similarity: `1.0 / (1.0 + distance)`
- **BM25 scores**: Normalized via sigmoid-like function: `score / (score + 1)` to produce 0-1 range
- **FTS scores** (LanceDB): Normalized the same way if `_score` is available, otherwise rank-based: `1.0 / (rank + 1)`

### Temporal Weighting (Optional)
When `UseTemporalWeighting` is enabled, scores are multiplied by a temporal decay factor:
- Age < 1 day: weight = 1.0
- Age 1-7 days: linear decay to 0.3
- Age 7-30 days: slower decay to 0.1
- Age > 30 days: weight = 0.1

### Retrieval Flow (`RetrieveContextForQuery`)
1. Embed the query text
2. Search with filters: `tenant_id`, `audience_type` (empty for "internal" searches to find all production content), `is_active=true`, content types (empty = all)
3. TopK=10, MinScore=0.25
4. Return all results as `KBSections` (no type-based bucketing -- embeddings handle relevance ranking)

---

## 5. OM + RAG Fusion Engine (`fusion_engine.go`)

The `FusionEngine` combines **Observational Memory** (compressed conversation history) with **RAG retrieval** for "unlimited memory":

### Scoring
- **Relevance**: Jaccard-like keyword overlap between content and query, with exact phrase boost
- **Recency**: Same temporal decay as vector search
- **Priority**: High = 1.5x, Medium = 1.0x, Low = 0.7x

### Pipeline
1. Parse observation log into individual items (date-based, priority-tagged)
2. Score each observation against query
3. Add RAG results with their native scores
4. Deduplicate similar content (first 10 words as key)
5. Sort by score descending
6. Truncate to `MaxContextTokens` (default: 30,000)
7. Build formatted context string with "Recent Observations" and "Relevant Knowledge" sections

---

## 6. How RAG Integrates with Chat Generation

### Decision Logic (from `service.go`)

```
if (RAG enabled) AND (content is unstructured OR tenant config says "rag" mode):
    use RAG pipeline
    -> RetrieveContextForQuery()
    -> buildRAGSystemPrompt() with retrieved chunks
    -> If retrieval fails or scores too low: fall back to full KB in prompt
else:
    long_context mode: full KB.MD + POLICY.MD in system prompt
```

### RAG System Prompt Structure
1. **IDENTITY** -- Role, tone, brand voice
2. **CUSTOMER INFO** -- Contact details already collected
3. **CONSTRAINTS** -- "Only discuss information in RETRIEVED CONTEXT", phone/email rules
4. **CONVERSATION GUIDE** -- SCRIPT.MD content
5. **RETRIEVED CONTEXT** -- The RAG chunks (services, FAQs, exclusions, coverage, safety, rules, KB sections)
6. **RAW KB FALLBACK** -- If retrieved chunks are insufficient, truncated raw KB appended

### Fallback Mechanism
- If zero chunks retrieved or top score < threshold -> activates fallback
- If RAG index is incomplete (no audience chunks, stale, or failed reindex) -> activates fallback
- Fallback appends truncated raw KB to system prompt

---

## 7. Limitations

### Critical Limitations

| Category | Issue | Details |
|---|---|---|
| **Token Estimation** | Crude approximation | `len(content) / 4` -- fails for non-English, code, mixed content |
| **30K Token Threshold** | Hardcoded RAG trigger | Content at 29,999 tokens uses long-context; 30,001 switches to RAG |
| **No Query Caching** | Re-embeds every query | Same query repeated wastes embedding API calls |
| **No Semantic Chunking** | Structure-only splitting | Relies entirely on markdown headers, no semantic understanding of content |
| **BM25 Rebuild** | MemoryVectorDB rebuilds index per query | O(n) cost on every search for hybrid mode |
| **No Stop Words/Stemming** | English-centric BM25 | Technical terms, proper nouns, multi-word phrases handled poorly |

### Performance Limitations

| Category | Issue | Impact |
|---|---|---|
| **Embedding Latency** | 50-200ms per embedding call | Adds to every chat response |
| **No Vector Cache** | Repeated queries re-embed | Unnecessary API costs |
| **MemoryVectorDB Linear Search** | O(n) scan | Degrades with >10K chunks |
| **Full Reindex on Change** | Delete + re-insert all chunks | Slow for large KBs |
| **LanceDB CGO Dependency** | Requires native libraries | Build complexity |

### Data Quality Limitations

| Category | Issue | Impact |
|---|---|---|
| **Score Normalization** | BM25 and vector scores on different scales | Linear combination may be miscalibrated |
| **Fixed Decay Parameters** | Temporal weighting not configurable | Seasonal content penalized incorrectly |
| **No Result Diversity** | Top-K may return similar chunks | Wasted context budget |
| **Intent Not Used for Filtering** | Intent parameter kept but unused | No intent-aware retrieval |
| **50-Token Overlap** | May not preserve sufficient context | Cross-boundary information lost |

### Scalability Limitations

| Category | Issue | Impact |
|---|---|---|
| **No Eviction Policy** | Old observations never removed | Memory grows unbounded |
| **No Multi-Instance Sync** | LanceDB is file-based | Single-server only |
| **No Incremental Updates** | Full reindex on every file upload | Slow for large documents |
| **No Background Processing** | Reindexing blocks operations | Brief unavailability during reindex |

---

## 8. How a Developer Could Use This to Learn About the Codebase

### Potential Application: Codebase Knowledge Agent

A developer could create a **"bchat codebase assistant"** tenant with:

1. **KB.MD containing:**
   - `<!-- @service -->` blocks for each major subsystem (agent, RAG, auth, frontend)
   - `<!-- @faq -->` blocks for common development questions
   - `<!-- @exclusion -->` blocks for things the system does NOT do
   - `<!-- @section -->` blocks for architecture overviews

2. **POLICY.MD containing:**
   - `<!-- @identity -->` -- "You are a senior developer familiar with the bchat codebase"
   - `<!-- @rule -->` blocks for coding conventions
   - `<!-- @intent -->` blocks for common developer intents ("add a new feature", "debug RAG issues", "run migrations")

3. **SCRIPT.MD containing:**
   - Conversation flow: understand request -> locate relevant code -> explain architecture -> suggest changes

### How It Would Work
- Upload the Go source files, TypeScript files, and documentation as KB.MD content
- The chunker would split by markdown headers (H2/H3)
- Embeddings would capture semantic relationships between code concepts
- At query time: "How does hybrid search work?" -> retrieves the fusion_engine.go and vectordb.go sections
- The developer gets a focused explanation with code context

---

## 9. Creative Applications

### Application 1: Multi-Knowledge-Base Customer Service
Upload different KB.MD files for different product lines. The RAG system would automatically retrieve relevant chunks based on the customer's query, enabling a single agent to handle multiple product areas without context pollution.

### Application 2: Internal Documentation Bot
Use the `@section` annotation to index design docs, API specs, and runbooks. The agent could answer "What is the deployment process for RAG?" by retrieving the relevant DOCS_RAG_PIPELINE.MD sections.

### Application 3: Compliance Verification Agent
The `@safety` annotation with trigger intents could be used to create an agent that cross-references customer requests against safety protocols and compliance rules, automatically flagging when procedures must be followed.

### Application 4: Knowledge Gap Detector
By analyzing which chunks never appear in search results (zero retrieval frequency), operators could identify content that is either poorly written, poorly chunked, or irrelevant -- enabling knowledge base optimization.

### Application 5: A/B Testing Different Knowledge Bases
The `audience_type` field (internal/external) enables serving different knowledge bases to different user segments. An operator could test whether external-facing customers get better responses from a simplified KB vs. a detailed one.

### Application 6: Temporal Knowledge Management
The temporal weighting system means recently indexed content ranks higher. An operator could re-index only updated sections to surface current information, while older sections naturally deprecate.

### Application 7: Cross-Tenant Knowledge Sharing
The chunk ID format `{tenantID}:{audience}:{type}:{code}` and the per-tenant LanceDB tables (e.g., `build/data/lancedb/{tenantID}`) enable a future feature where selected knowledge chunks could be shared across tenants in the same vertical.

---

## 10. Key Files Summary

| File | Purpose | Lines |
|---|---|---|
| `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/vectordb.go` | VectorDB interface, MemoryVectorDB, BM25 scorer, search types | 939 |
| `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/vectordb_lance.go` | LanceDB implementation with hybrid search | 1194 |
| `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/chunker.go` | DocumentChunk type, heading-based chunking, content sanitization | 788 |
| `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/embedding.go` | EmbeddingService interface, OpenRouter/Local/Mock providers | 580 |
| `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/fusion_engine.go` | OM + RAG fusion, temporal weighting, deduplication | 471 |
| `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/parser.go` | KB/Policy/Script annotation parsing | 1146 |
| `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go` | RAG integration, reindexing, system prompt building | 4659 |
| `/home/chaschel/Documents/go/bchat/docs/DOCS_RAG_PIPELINE.MD` | RAG configuration and setup guide | 523 |
| `/home/chaschel/Documents/go/bchat/docs/DOCS_HYBRID_SEARCH.MD` | Hybrid search architecture and rollout plan | 678 |
| `/home/chaschel/Documents/go/bchat/docs/DOCS_RAG_MINIMAX25.MD` | Comprehensive limitations analysis | 717 |
| `/home/chaschel/Documents/go/bchat/docs/KB_TEMPLATE.MD` | Example KB.MD template with annotations | 172 |
</task_result>
</task>
```

---

## Assistant (Plan · MiMo V2.5 Free · 153.1s)

_Thinking:_

Now I have a deep understanding of the RAG pipeline. Let me also look at the KB template and some example content to understand what a "codebase learning" tenant would look like.

Now let me look at the existing templates and documentation to understand what content could be indexed for a codebase learning agent.

**Tool: task**

**Input:**
```json
{
  "description": "Explore KB templates and docs",
  "prompt": "Read the following files to understand KB/Policy templates and what content could be indexed:\n\n1. `docs/KB_TEMPLATE.MD` - KB template\n2. `docs/POLICY_TEMPLATE.MD` or any policy template\n3. `AGENTS.md` - the existing agent guide (this is what a learner would want to understand)\n4. `docs/DOCS_README.MD` - comprehensive project overview\n5. `docs/DOCS_AGENT_ARCHITECTURE.MD` - architecture docs\n6. `docs/DOCS_RAG_PIPELINE.MD` - RAG configuration\n7. `bugs/033/docs_033.md` - the webhook feature docs\n8. `TUTORIAL.md` if it exists\n\nFocus on:\n- What annotations are used in templates\n- How content is structured for optimal RAG retrieval\n- What documentation exists that could be fed into a \"codebase learning\" tenant\n- How a new developer would benefit from querying a RAG-powered codebase assistant\n\nReturn the key content from each file and recommendations for building a \"learn the bchat codebase\" knowledge base.",
  "subagent_type": "explore"
}
```

**Output:**
```
<task id="ses_0b3287e78ffeoDbNhX7P8gvLy9" state="completed">
<task_result>
Now I have all the information needed. Here is a comprehensive analysis.

---

# KB/Policy Template Analysis and Codebase Learning Knowledge Base Recommendations

## 1. KEY CONTENT FROM EACH FILE

### A. `docs/KB_TEMPLATE.MD` (172 lines) -- Knowledge Base Template

**Annotation System Used:**
- `<!-- @service: service_code, emergency: true, response_time: "60 minutes" -->` -- Service/product entries with metadata
- `<!-- @exclusion: code, exception: "secondary to X if applicable" -->` -- Services NOT offered
- `<!-- @coverage: include -->` / `<!-- @coverage: exclude -->` -- Geographic/scope areas
- `<!-- @faq: hours -->`, `<!-- @faq: pricing -->`, etc. -- Frequently asked questions
- `<!-- @safety: water_emergency, triggers: emergency_water, emergency_flood -->` -- Emergency safety procedures

**Content Structure for RAG Optimization:**
Each section uses `##` headers with a blank line, then the annotation, then 2-4 sentences of content. This creates natural ~500-token chunks for the RAG pipeline. The template uses a repeating pattern: annotation comment, heading, 2-4 sentence description, bullet lists for details (causes, what's included, response times). This is ideal for the chunker because each annotated section becomes an independent, self-contained chunk.

**Key Structural Patterns:**
- Company Overview table (structured metadata)
- Contact Information table
- Services with annotations (each service is a separate chunk)
- Exclusions (explicit what is NOT provided)
- Coverage areas (include/exclude)
- FAQ sections (one Q&A per chunk)
- Safety procedures with trigger annotations
- "What To Expect" process flow

---

### B. `docs/POLICY_TEMPLATE.MD` (143 lines) -- Agent Policy Template

**Annotation System Used:**
- `<!-- @identity -->` -- Agent persona definition (role, tone, brand voice, escalation contacts, business hours)
- `<!-- @intent: emergency, urgency_threshold: 75, priority: P0 -->` -- Customer intent categories with urgency/priority metadata
- `<!-- @rule: safety_first, priority: 1 -->` -- Behavioral rules with priority ordering
- `<!-- @threshold: urgency_emergency, value: 75 -->` -- Numeric scoring thresholds

**Content Structure:**
- Identity block: role, tone, brand voice, escalation contact, hours, after-hours handling
- Intents: each intent has description, trigger keywords, and explicit agent behavior instructions
- Rules: numbered behavioral rules with priority
- Thresholds: table mapping score ranges to classifications and agent responses
- Fallback rules: what to do when the agent does not know
- Lead capture policy: when and what to collect
- Human takeover triggers

---

### C. `AGENTS.md` (810 lines) -- Comprehensive Agent Development Guide

**Key Content for Indexing:**
- Project overview and core design principle (general-purpose, not tenant-specific)
- Technology stack table
- Complete directory structure
- Tenant model with Go struct definition
- Three configuration files (KB.MD, POLICY.MD, SCRIPT.MD) with annotation system documentation
- Tenant Isolation Architecture (JWT claims, context mechanism, required patterns)
- RAG Pipeline architecture diagram and key components
- Observational Memory system
- Build commands (quick start, with RAG, development)
- Complete environment variables reference with configuration priority
- API endpoints table (public, auth, admin)
- Permission system (RBAC) with permission checking code pattern
- Security guidelines (DO/DON'T patterns, superuser bypass)
- Multi-tenant authentication flow
- Adding new features (backend, database migration, frontend patterns)
- Key files reference table
- Code conventions (Go, TypeScript/React, SQL, tenant isolation)
- Common tasks (add config option, add RAG feature, add API endpoint, debug RAG)
- Database query examples
- Testing endpoint curl commands
- Useful grep patterns
- Gotchas table
- MD file versioning protocol

---

### D. `docs/DOCS_README.MD` (602 lines) -- Comprehensive Project Overview

**Key Content for Indexing:**
- Overview with key principles (general-purpose, configuration-driven, multi-tenant isolation)
- Full architecture with request flow diagram (Client -> Echo Router -> Agent Handler -> Agent Service -> LLM)
- Multi-tenant agent system with tenant model table
- Configuration file examples (KB.MD, POLICY.MD, SCRIPT.MD)
- RAG pipeline with architecture diagram and comparison table (RAG vs non-RAG)
- Embedding providers table (openrouter, local, mock)
- Storage providers table (memory, local, s3)
- Hybrid search architecture (vector + BM25)
- Features list (Agent Admin Interface, Chat Interface, Simulation System, Transcript Analysis)
- Getting started (prerequisites, quick start, development mode)
- Configuration reference
- API reference tables
- Deployment section (local, production, fly.io, environment-specific)
- Limitations and known issues
- Response validation (two-layer system: sanitization + optional LLM verification)
- Security considerations
- Related documentation index

---

### E. `docs/DOCS_AGENT_ARCHITECTURE.MD` (698 lines) -- Configuration-Driven Design

**Key Content for Indexing:**
- Design principle (general-purpose, not tenant-specific) with DO/DO NOT table
- Service business origins and why they dominate examples
- Annotation-based parsing system with regex pattern explanation
- Currently supported annotation types table (10 types)
- Generic `@section` annotation for custom content without code changes
- How to add new annotation types
- Flexible architecture elements: multi-tenant isolation, multi-audience support, template-based prompt building, content-agnostic RAG, generic intent system
- **Four full adaptation examples** showing how the system adapts to different verticals:
  - A) Q&A / Knowledge Base System (HR portal)
  - B) Technical Support System
  - C) HR / Employee Portal
  - D) Legal / Compliance System
- What requires code changes vs. configuration only
- Extension points (`@section`, intent `action`, rule `priority`, thresholds)
- Anti-patterns to avoid (tenant-specific conditionals, hardcoded vertical logic, vertical-specific intents, assumptions about content structure)

---

### F. `docs/DOCS_RAG_PIPELINE.MD` (523 lines) -- RAG Configuration Guide

**Key Content for Indexing:**
- RAG pipeline architecture diagram (indexing and retrieval flows)
- Storage backends table (memory, local, s3) with persistence and rebuild requirements
- MemoryVectorDB (pure Go, no native libs, cosine similarity, in-memory hash map)
- Quick start without rebuild (memory + mock embeddings)
- Full RAG setup with LanceDB (local and S3/Tigrisdata)
- Embedding providers (mock, local, openrouter) with pros/cons
- Complete environment variables reference table
- Re-indexing without re-uploading (REINDEX_RAG)
- Configuration presets (Development Fastest, Development Quality, Production)
- Verification checklist (4 steps)
- Troubleshooting guide (7 common issues with solutions)
- Memory vs persistent storage comparison table
- Performance considerations (memory O(n) vs LanceDB O(log n))
- Re-indexing triggers

---

### G. `bugs/033/docs_033.md` (212 lines) -- Webhook Feature Documentation

**Key Content for Indexing:**
- Business summary (what, why, how)
- Step-by-step testing guide (7 steps with prerequisites)
- Technical changes summary (4 database migrations, 6 backend files, 4 container files, 3 frontend files, 1 test file)
- API endpoints table (7 endpoints)
- Webhook payload structure (JSON example with headers)
- Configuration (environment variables, fly.toml)
- Limitations (webhooks only, awake-only retries, SQLite write lock)
- Complete file inventory (20 files changed)

---

### H. `TUTORIAL.md` (1957 lines) -- Production Software Case Study

**Key Content for Indexing:**
- 16 chapters covering the full SDLC through the lens of bugs/033
- Business lens (product discovery, requirements engineering, ADRs)
- Implementation lens (database schema, store layer, service layer bugs, security)
- Operations lens (containerization, database ops, Fly.io deployment)
- Maintenance lens (Postgres resilience, observability, rollback plans)
- Critical thinking habits (lessons from code review, why AI agents missed bugs)
- Appendices: code review findings table, file inventory, deployment cheat sheet, monitoring reference, glossary, decision log

---

## 2. ANNOTATIONS SUMMARY

The bchat annotation system uses HTML comments with a `@type` prefix:

| Annotation | File | Purpose | Key Parameters |
|------------|------|---------|----------------|
| `@service` | KB.MD | Service/product entries | `emergency`, `response_time` |
| `@faq` | KB.MD | Frequently asked questions | slug identifier |
| `@exclusion` | KB.MD | Services NOT provided | `exception` |
| `@coverage` | KB.MD | Service areas | `include` or `exclude` |
| `@safety` | KB.MD | Emergency safety procedures | `triggers` |
| `@section` | KB.MD | Generic content sections | `type` (customizable) |
| `@identity` | POLICY.MD | Agent persona definition | (none -- body contains role, tone, etc.) |
| `@intent` | POLICY.MD | Customer intent categories | `urgency_threshold`, `priority`, `action` |
| `@rule` | POLICY.MD | Behavioral rules | `priority` |
| `@threshold` | POLICY.MD | Numeric scoring thresholds | `value` |

**For RAG indexing, the key insight:** The `@section` annotation is the escape hatch for any content type that does not fit the predefined categories. The DOCS_AGENT_ARCHITECTURE.MD shows examples like `<!-- @section: products, type: catalog -->`, `<!-- @section: hr_policies, type: category -->`, `<!-- @section: gdpr, type: regulation -->`. This means a "codebase learning" KB can use `@section` for any content type.

---

## 3. CONTENT STRUCTURE FOR OPTIMAL RAG RETRIEVAL

Based on the templates and architecture docs:

1. **Each annotated section becomes a self-contained chunk.** The chunker splits on `##` headers and targets ~500 tokens with 50-token overlap. The annotation comment is prepended to the chunk metadata.

2. **Content type metadata is preserved.** The chunker tags each chunk with `ContentType` (service, faq, policy, exclusion, coverage, safety, section) and `AudienceType` (internal, external). This enables filtered retrieval.

3. **Bullet lists and tables are chunk-friendly.** Short, discrete pieces of information (causes, what's included, response times) create naturally retrievable chunks.

4. **The annotation comments serve as semantic markers.** They help the LLM understand the *role* of each chunk (this is a service, this is a FAQ, this is a rule).

---

## 4. DOCUMENTATION AVAILABLE FOR A "CODEBASE LEARNING" TENANT

The following files contain rich, indexable content:

**Tier 1 -- Core Architecture (highest value for new developer learning):**
- `/home/chaschel/Documents/go/bchat/AGENTS.md` (810 lines)
- `/home/chaschel/Documents/go/bchat/docs/DOCS_README.MD` (602 lines)
- `/home/chaschel/Documents/go/bchat/docs/DOCS_AGENT_ARCHITECTURE.MD` (698 lines)

**Tier 2 -- RAG and Search (how the system works internally):**
- `/home/chaschel/Documents/go/bchat/docs/DOCS_RAG_PIPELINE.MD` (523 lines)
- `/home/chaschel/Documents/go/bchat/docs/DOCS_HYBRID_SEARCH.MD`
- `/home/chaschel/Documents/go/bchat/docs/DOCS_HYBRID_SEARCH_EXPLAINED.MD`
- `/home/chaschel/Documents/go/bchat/docs/DOCS_EMBEDDING.MD`
- `/home/chaschel/Documents/go/bchat/docs/DOCS_CHUNKING.MD`

**Tier 3 -- Operations and Deployment:**
- `/home/chaschel/Documents/go/bchat/docs/DOCS_DEPLOYMENT.MD`
- `/home/chaschel/Documents/go/bchat/docs/DOCS_DEPLOY_FLY.MD`
- `/home/chaschel/Documents/go/bchat/docs/DOCS_DOCKERFILE.MD`
- `/home/chaschel/Documents/go/bchat/docs/DOCS_ENV_VAR.MD`

**Tier 4 -- Real-World Case Studies:**
- `/home/chaschel/Documents/go/bchat/TUTORIAL.md` (1957 lines)
- `/home/chaschel/Documents/go/bchat/bugs/033/docs_033.md` (212 lines)
- `/home/chaschel/Documents/go/bchat/docs/DOCS_AGENT_CODE_AUDIT.MD`

**Tier 5 -- Feature Specifications:**
- `/home/chaschel/Documents/go/bchat/docs/DOCS_SIMULATION.MD`
- `/home/chaschel/Documents/go/bchat/docs/DOCS_WIDGET.MD`
- `/home/chaschel/Documents/go/bchat/docs/DOCS_HALLUCINATION_GROUNDING.MD`
- `/home/chaschel/Documents/go/bchat/docs/DOCS_EVALUATION.MD`
- `/home/chaschel/Documents/go/bchat/docs/DOCS_CONTEXT_AWARENESS.MD`

**Tier 6 -- Templates (for understanding the annotation system):**
- `/home/chaschel/Documents/go/bchat/docs/KB_TEMPLATE.MD` (172 lines)
- `/home/chaschel/Documents/go/bchat/docs/POLICY_TEMPLATE.MD` (143 lines)

**Tier 7 -- Changelog and Pending Work:**
- `/home/chaschel/Documents/go/bchat/docs/CHANGELOG.MD` (53K)
- `/home/chaschel/Documents/go/bchat/docs/DOCS_BUGS.MD`
- `/home/chaschel/Documents/go/bchat/docs/DOCS_PENDING.MD`

---

## 5. RECOMMENDATIONS FOR BUILDING A "LEARN THE BCHAT CODEBASE" KB

### KB.MD Structure

```markdown
# bchat Codebase Knowledge Base

<!-- @section: project_overview, type: overview -->
## Project Overview
bchat is a multi-tenant AI chat agent platform built on the Memos application.
Each tenant can define their own knowledge base, policies, and conversation
scripts without requiring code changes. The agent must be GENERAL PURPOSE,
not tenant-specific. All customization happens through KB.MD, POLICY.MD,
and SCRIPT.MD configuration files.

---

<!-- @section: tech_stack, type: architecture -->
## Technology Stack
Backend: Go 1.21+, Echo framework. Database: SQLite (default), PostgreSQL,
MySQL. Frontend: React 18, TypeScript, MobX, Vite. UI: Joy UI (@mui/joy).
LLM: OpenRouter API. Vector DB: LanceDB (optional, for RAG).
Postgres Driver: pgx/v5 (sole driver -- lib/pq is NOT used).

---

<!-- @section: directory_structure, type: reference -->
## Directory Structure
bin/memos/ -- Application entry point.
build/data/ -- Runtime data (SQLite DB, LanceDB indexes).
docs/ -- Documentation (START HERE).
server/router/api/v1/agent/ -- Agent API handlers and services.
  handlers.go -- HTTP request handlers.
  service.go -- Business logic, LLM integration.
  parser.go -- KB/Policy/Script parsing.
  vectordb.go -- Vector database interface.
  vectordb_lance.go -- LanceDB implementation.
  embedding.go -- Embedding providers.
  chunker.go -- Document chunking.
  observer.go -- Observational memory.
  simulation.go -- Agent simulation.
  analysis.go -- Transcript analysis.
  verifier.go -- LLM response verification.
  sanitizer.go -- Output sanitization.
  prompts/ -- Prompt templates.
store/ -- Data layer (agent.go, driver.go).
store/db/sqlite/ -- SQLite implementations.
store/migration/sqlite/ -- Database migrations.
web/src/pages/ -- React page components.
web/src/store/v2/ -- MobX stores.
web/src/locales/ -- i18n translations.
widget/ -- Embeddable chat widget.
plugin/ -- Cron, webhook, storage plugins.
Taskfile.yml -- Build and run commands.

---

<!-- @section: tenant_model, type: core_concept -->
## Tenant Model
Each tenant represents an isolated agent configuration with fields:
ID (int32), Slug (URL-friendly, e.g. "acme-corp"), CompanyName (display name),
GUID (unique identifier for widget embed), Vertical (industry vertical),
IsActive (bool), ProcessingOptions (JSON-encoded), AllowedDomains (JSON array).
Every API request must be scoped to a single tenant. JWT claims include
TenantID. Single-tenant users get auto-selection; multi-tenant users must
explicitly select via POST /api/v1/auth/select-tenant.

---

<!-- @section: annotation_system, type: reference -->
## Annotation System
KB.MD annotations: @service (services/products), @faq (Q&A pairs),
@exclusion (services NOT offered), @coverage (service areas), @safety
(safety info), @section (generic content).
POLICY.MD annotations: @identity (agent persona), @intent (customer
intent categories), @rule (behavioral rules), @thresholds (numeric thresholds).
Format: <!-- @type: value, key: value -->. The parser uses regex:
<!--\s*@(\w+)(?::\s*([^>]*))?\s*-->.
Adding new annotation types requires adding to ParsedKB/ParsedPolicy struct,
adding a case in the parser switch statement, and optionally adding a
database table. Or use @section with custom type parameter for no code changes.

---

<!-- @section: rag_pipeline, type: architecture -->
## RAG Pipeline
Architecture: Document Upload -> Chunker (~500 tokens, 50 overlap) ->
Embedding (1536-dim) -> LanceDB -> [Query Time] User Query -> Embed ->
Vector Search -> Top-K -> LLM Prompt.
Storage backends: memory (no rebuild, lost on restart), local (persistent disk),
s3 (persistent cloud). Embedding providers: openrouter (production, API key),
local (development, sentence-transformers), mock (testing, random vectors).
Hybrid search: 70% vector similarity + 30% BM25 keyword matching.
Environment: RAG_PIPELINE_ENABLED=true, EMBEDDING_PROVIDER=openrouter,
LANCEDB_STORAGE_PROVIDER=local.

---

<!-- @section: build_commands, type: reference -->
## Build Commands
Quick start (no RAG): task setup, export OPENROUTER_API_KEY, task build,
./build/memos --mode dev --data build/data.
With RAG: task build:rag, task run:rag (or task run:rag:mock for testing).
Development: task dev:backend (terminal 1), task dev:frontend (terminal 2).
Common tasks: task build:backend, task build:frontend, task build:rag,
task run:rag:l12, task validate:schema.

---

<!-- @section: environment_variables, type: reference -->
## Environment Variables
Required: OPENROUTER_API_KEY. LLM: LLM_MODEL=openai/gpt-4o-mini,
LLM_MODEL_REASONING=google/gemini-2.5-pro.
RAG: RAG_PIPELINE_ENABLED=true, EMBEDDING_PROVIDER=openrouter,
EMBEDDING_MODEL=text-embedding-3-small, EMBEDDING_BATCH_SIZE=10,
EMBEDDING_TIMEOUT=180s, LANCEDB_STORAGE_PROVIDER=local,
LANCEDB_LOCAL_PATH=build/data/lancedb.
OM: OM_ENABLED=true, OM_OBSERVER_TOKEN_THRESHOLD=30000, OM_TOKEN_THRESHOLD=2000.
Startup flags: FORCE_REINDEX_ON_STARTUP=true, HYBRID_SEARCH_ENABLED=true,
LLM_VERIFIER_ENABLED=true.
Config priority: Tenant Config (highest) -> Environment Variable -> Hardcoded Default (lowest).

---

<!-- @section: api_endpoints, type: reference -->
## API Endpoints
Public: POST /api/v1/agent/:slug/chat, GET /api/v1/agent/:slug/chat/stream.
Auth: POST /api/v1/auth/tenants, POST /api/v1/auth/select-tenant.
Admin: GET/POST /api/v1/agent/tenants (tenant:admin),
GET/PUT/DELETE /api/v1/agent/:slug (tenant:admin),
POST /api/v1/agent/:slug/files (files:upload),
POST /api/v1/agent/:slug/reindex (api:config),
POST /api/v1/agent/:slug/simulate (chat:test),
GET /api/v1/agent/:slug/simulations (chat:test).

---

<!-- @section: permissions, type: reference -->
## Permission System (RBAC)
tenant:admin -- Full tenant management.
tenant:read -- View tenant configuration.
api:config -- Configure LLM settings, rebuild index.
chat:test -- Run simulations, view history.
chat:logs -- View real chat session logs.
files:upload -- Upload KB/Policy/Script files.
Check in handlers: hasPermission, _ := h.service.CheckUserPermission(ctx, userID, tenantID, "chat:test").

---

<!-- @section: tenant_isolation, type: security -->
## Tenant Isolation Architecture
Every API request must be scoped to a single tenant. JWT ClaimsMessage
includes TenantID. Context mechanism: Echo uses getTenantFromContext(c),
gRPC uses GetTenantIDFromContext(ctx).
Required pattern: (1) Extract tenant ID from context, (2) Apply tenant
filter to database queries, (3) Verify tenant ownership for update/delete.
DO: Always extract tenant ID, Apply ApplyTenantFilter(c, find), Verify ownership.
DON'T: Hardcode tenant IDs, Skip tenant filtering, Expose tenant IDs in errors,
Allow cross-tenant data access, Trust user-supplied tenant IDs from request body.

---

<!-- @section: adding_features, type: tutorial -->
## Adding New Features
Backend: (1) Define types in store/agent.go, (2) Add interface methods to
store/driver.go, (3) Implement in store/db/sqlite/agent.go, (4) Add handlers
in handlers.go, (5) Register routes in v1.go.
Database migration: store/migration/sqlite/NN__snake_case.sql with
CREATE TABLE IF NOT EXISTS and CREATE INDEX IF NOT EXISTS.
Frontend: (1) Add types/state to MobX store, (2) Add methods with runInAction,
(3) Add UI component, (4) Add translations to web/src/locales/en.json.
Code conventions: slog for logging, fmt.Errorf with %w for errors, pointer
receivers, snake_case JSON tags. TypeScript: makeAutoObservable, runInAction,
observer HOC, Joy UI components. SQL: plural table names, IF NOT EXISTS,
ON DELETE CASCADE for tenant-scoped data.

---

<!-- @section: gotchas, type: reference -->
## Common Gotchas
Table names must be plural: agent_tenants not agent_tenant.
Env vars not working in Taskfile: use inline VAR=value not env: block.
Frontend state not updating: wrap in runInAction().
Store method not accessible: add to return object.
Mock embeddings not semantic: use openrouter instead.
Migration not running: check filename is NN__snake_case.sql.
CGO errors: run task setup:lancedb first.
SCRIPT.MD is NOT RAG-indexed -- by design, included fully in system prompt.
Hybrid search adds ~20-50ms latency.
SQLite has no SKIP LOCKED -- single-writer assumption.
Postgres needs default_query_exec_mode=simple_protocol for Neon PgBouncer.

---

<!-- @section: key_files, type: reference -->
## Key Files Reference
handlers.go -- HTTP request handlers.
service.go -- Business logic, LLM integration.
parser.go -- KB/Policy/Script parsing.
chunker.go -- Document chunking for RAG.
vectordb.go -- Vector database interface.
embedding.go -- Embedding providers.
observer.go -- Observational memory.
simulation.go -- Agent simulation.
analysis.go -- Transcript analysis.
verifier.go -- LLM response verification.
sanitizer.go -- Output sanitization.
auth.go -- JWT claims, token generation.
auth_service.go -- Auth endpoints, tenant selection.
acl.go -- gRPC auth interceptor, context extraction.
tenant_context.go -- Echo tenant context helpers.
memo_service.go -- Memo CRUD with tenant scoping.
ticket_service.go -- Ticket CRUD with tenant scoping.
store/agent.go -- Data types and store interface.
store/driver.go -- Database driver interface.
```

### POLICY.MD Structure

```markdown
# bchat Codebase Agent Policy

<!-- @identity -->
- Role: Codebase Learning Assistant for bchat developers
- Tone: Technical, precise, helpful. Explain concepts clearly with code examples.
- Brand Voice: "Here is how bchat works and why."
- Target audience: Intermediate Go/React developers new to the codebase

---

<!-- @intent: architecture_question, category: standard, action: search_kb -->
### Architecture Question
Developer asking about system design, architecture, or how components interact.
Examples: "How does multi-tenancy work?", "What is the RAG pipeline?",
"How do annotations work in KB.MD?"

---

<!-- @intent: how_to_question, category: standard, action: search_kb -->
### How-To Question
Developer asking how to perform a specific task in the codebase.
Examples: "How do I add a new API endpoint?", "How do I create a migration?",
"How do I add a new store method?"

---

<!-- @intent: debugging_question, category: standard, action: search_kb -->
### Debugging Question
Developer asking about a bug, error, or unexpected behavior.
Examples: "Why is my migration failing?", "Why is the frontend not updating?",
"What does this error mean?"

---

<!-- @intent: code_review_question, category: standard, action: search_kb -->
### Code Review Question
Developer asking about code patterns, conventions, or best practices.
Examples: "Is this the right way to handle errors?", "How should I structure
this handler?", "What security patterns should I follow?"

---

<!-- @intent: deployment_question, category: standard, action: search_kb -->
### Deployment Question
Developer asking about building, deploying, or running the application.
Examples: "How do I run with RAG?", "How do I deploy to Fly.io?",
"What environment variables do I need?"

---

<!-- @intent: unclear_question, category: meta, action: clarify -->
### Unclear Question
Developer's question is ambiguous and needs clarification.

---

<!-- @intent: out_of_scope, category: meta, action: escalate -->
### Out of Scope
Question cannot be answered from available codebase knowledge.

---

<!-- @rule: accuracy, priority: 1 -->
1. Always cite the specific file and line number when referencing code.
2. Never guess about code behavior -- say "I am not sure, check [file]" instead.
3. Always reference the AGENTS.md checklist when suggesting new features.
4. Explain the "why" not just the "how" -- reference design decisions and ADRs.
5. Point to the relevant docs file for deep dives (e.g., "See DOCS_RAG_PIPELINE.MD").
6. Warn about gotchas before suggesting solutions (e.g., "Remember that table names must be plural").

---

<!-- @rule: fallback, priority: medium -->
### When the Agent Does Not Know
If the agent cannot answer using the codebase knowledge:
1. Say so honestly: "This is not covered in the current documentation."
2. Suggest where to look: "Check [specific file] or ask in the project channel."
3. Offer to help with what IS documented.
```

---

## 6. HOW A NEW DEVELOPER BENEFITS FROM A RAG-POWERED CODEBASE ASSISTANT

A RAG-powered "learn the bchat codebase" tenant would enable:

**Immediate value:**
- Ask "How do I add a new API endpoint?" and get the exact step-by-step from AGENTS.md (5 backend steps + frontend steps), including code conventions and gotchas
- Ask "How does multi-tenancy work?" and get the JWT claims, context mechanism, and required handler patterns from the architecture docs
- Ask "What are the annotation types?" and get the complete annotation table with examples from KB_TEMPLATE.MD and POLICY_TEMPLATE.MD
- Ask "How do I run with RAG?" and get the exact commands from DOCS_RAG_PIPELINE.MD

**Deep understanding:**
- Ask "Why does the codebase use pgx/v5 instead of lib/pq?" and get the Neon PgBouncer compatibility explanation from TUTORIAL.md Chapter 9
- Ask "What bugs should I watch out for?" and get the parameter blindness, context lifecycle, and security consistency lessons from TUTORIAL.md Chapter 14
- Ask "How does the webhook outbox pattern work?" and get the pre-claimed events, lease-based claiming, and retry logic from bugs/033
- Ask "What are the design anti-patterns?" and get the DO/DO NOT table from DOCS_AGENT_ARCHITECTURE.MD

**Practical help:**
- Ask "Show me a complete migration example" and get the SQLite/Postgres dialect differences from DOCS_AGENT_ARCHITECTURE.MD
- Ask "How do I check if RAG is working?" and get the verification checklist from DOCS_RAG_PIPELINE.MD
- Ask "What is the rollback plan for a new feature?" and get the three-level rollback strategy from TUTORIAL.md Chapter 13

The key advantage over a static document: RAG retrieves the *specific* chunks relevant to the developer's question, rather than forcing them to read 5,000+ lines of documentation. A question about "how to add a new store method" retrieves the exact 5-step backend pattern from AGENTS.md, not the entire file.
</task_result>
</task>
```

---

## Assistant (Plan · MiMo V2.5 Free · 128.7s)

_Thinking:_

Now I have a comprehensive understanding of the RAG pipeline and how it could be used for a "learn the bchat codebase" chapter. Let me craft a new chapter that:

1. Teaches the learner to use RAG to learn the codebase itself (dogfooding)
2. Shows how RAG can help implement bugs/033 features
3. Is imaginative and engaging
4. Thinks like a business person

This is a unique chapter because it's meta — using the product to learn about the product. Let me think about this from a business perspective:

- Business value: If you can use your own product to onboard developers, that proves the product works
- Dogfooding: Using your own product builds empathy and reveals gaps
- The "aha moment": When a developer realizes RAG can answer questions about the codebase faster than reading docs


- I'm structuring this around building a codebase assistant, then showing how RAG accelerates the bugs/033 implementation and uncovers creative use cases, all while learning to identify where RAG falls short.

Now I have everything needed. Let me present the new chapter integrated into the outline.

---

# NEW CHAPTER: Chapter 19 — Dogfooding RAG: Using bchat to Learn bchat

## PART VIII: META-LEARNING — Using the Product to Learn the Product

### Chapter 19: Dogfooding RAG: Using bchat to Learn bchat

**Why this chapter exists:**
You've spent 18 chapters learning bchat from the outside in — architecture, code, deployment. Now flip the script. Use bchat's own RAG pipeline to build a "codebase assistant" that answers your questions about the code. This is dogfooding: eating your own cooking to taste what your customers taste. If the product can't teach you about itself, it can't teach anyone anything.

---

#### 19.1 The Business Case for Dogfooding

**The "Eat Your Own Dog Food" Principle**
- Microsoft coined it in the 1980s. The logic is simple: if you won't use it, why should anyone else?
- For bchat, dogfooding means: build a tenant that uses KB.MD/POLICY.MD/SCRIPT.MD to teach developers about the bchat codebase
- If the RAG pipeline can't answer "How do I add a new API endpoint?" reliably, it can't answer "How do I schedule a water extraction appointment?" reliably either

**What Dogfooding Reveals**
- **RAG quality gaps:** If the codebase assistant gives vague answers, your customers' agents will too
- **Chunking failures:** If the chunker splits code docs badly, it splits KB articles badly
- **Retrieval weaknesses:** If hybrid search misses relevant chunks, your customers miss relevant FAQs
- **Prompt engineering gaps:** If the system prompt doesn't ground the LLM, your customers' agents hallucinate

**The Meta-Lesson**
- Building the codebase assistant forces you to think like a product manager: "What questions will developers ask? What answers do they need? What's the right level of detail?"
- This is exactly the thinking your customers need to do for their KB.MD/POLICY.MD files

**Think About This:**
- If you were onboarding a new developer to bchat, what are the 10 questions they'd ask first?
- How does writing a KB.MD for a codebase differ from writing one for a water extraction company?
- What's the business value of a developer who can self-serve answers instead of pestering the team?

---

#### 19.2 Building the Codebase Assistant: Step by Step

**The Setup:**
1. Create a new tenant: `slug = "bchat-dev"`, `company_name = "bchat Development Team"`
2. Upload three files:
   - `KB_CODEBASE.MD` — architecture, patterns, gotchas, file reference
   - `POLICY_CODEBASE.MD` — identity, intents, rules for a developer assistant
   - `SCRIPT_CODEBASE.MD` — conversation flow for code questions

**The KB.MD Structure:**

Each section uses annotations for RAG retrieval:

```markdown
<!-- @section: tenant_isolation, type: security, priority: high -->
## Tenant Isolation Architecture
Every API request must be scoped to a single tenant.
JWT ClaimsMessage includes TenantID. Context mechanism: Echo uses
getTenantFromContext(c), gRPC uses GetTenantIDFromContext(ctx).
Required pattern: (1) Extract tenant ID, (2) Apply tenant filter,
(3) Verify ownership. DO: Always extract, Apply ApplyTenantFilter,
Verify ownership. DON'T: Hardcode tenant IDs, Skip filtering,
Expose tenant IDs in errors.

---

<!-- @section: add_api_endpoint, type: tutorial, priority: high -->
## Adding a New API Endpoint
1. Define types in store/agent.go
2. Add interface methods to store/driver.go
3. Implement in store/db/sqlite/agent.go
4. Add handlers in handlers.go
5. Register routes in v1.go
Gotcha: Table names must be plural (agent_tenants not agent_tenant).

---

<!-- @section: rag_how_it_works, type: architecture -->
## How RAG Works in bchat
Architecture: Document Upload -> Chunker (~500 tokens, 50 overlap) ->
Embedding (1536-dim) -> LanceDB -> Query Time: User Query -> Embed ->
Vector Search -> Top-K -> LLM Prompt.
Hybrid search: 70% vector similarity + 30% BM25 keyword matching.
Storage: memory (dev), local (prod), s3 (cloud).
```

**The POLICY.MD Structure:**

```markdown
<!-- @identity -->
- Role: bchat Codebase Learning Assistant
- Tone: Technical, precise, helpful. Explain concepts clearly.
- Brand Voice: "Here is how bchat works and why."
- Target audience: Intermediate Go/React developers

---

<!-- @intent: architecture_question, action: search_kb -->
### Architecture Question
Developer asking about system design or component interaction.
Examples: "How does multi-tenancy work?", "What is the RAG pipeline?"

---

<!-- @intent: how_to_question, action: search_kb -->
### How-To Question
Developer asking how to perform a specific task.
Examples: "How do I add a new API endpoint?", "How do I create a migration?"

---

<!-- @rule: accuracy, priority: 1 -->
1. Always cite the specific file when referencing code.
2. Never guess -- say "check [file]" if unsure.
3. Explain the "why" not just the "how."
4. Point to the relevant docs file for deep dives.
```

**Do This:**
- Create the `bchat-dev` tenant
- Write the KB_CODEBASE.MD with 10 sections covering: project overview, tech stack, directory structure, tenant model, annotation system, RAG pipeline, build commands, environment variables, API endpoints, and gotchas
- Write the POLICY_CODEBASE.MD with identity, 3 intents, and 2 rules
- Upload both files and reindex
- Ask: "How do I add a new API endpoint?"
- Observe what RAG retrieves and how the agent responds

**Think About This:**
- What happens if you ask a question not covered in the KB? Does the agent admit ignorance or hallucinate?
- How does the quality of the KB.MD writing affect the quality of the answers?
- What's the difference between a well-chunked section and a poorly-chunked one?

---

#### 19.3 Testing RAG Against the Codebase

**The Test Matrix:**

Ask these questions and evaluate the responses:

| Category | Question | Expected Source | What to Check |
|----------|----------|----------------|---------------|
| Architecture | "How does multi-tenancy work?" | AGENTS.md tenant isolation section | JWT claims, context mechanism, ApplyTenantFilter |
| How-To | "How do I add a new store method?" | AGENTS.md adding features section | 5-step backend pattern |
| Debugging | "Why is my migration failing?" | gotchas section + migration docs | Filename convention, LATEST.sql sync |
| Reference | "What environment variables do I need?" | env var reference | Complete list with defaults |
| Gotchas | "What table naming convention should I use?" | gotchas section | Plural names, IF NOT EXISTS |
| Security | "How do I check permissions in a handler?" | RBAC section | CheckUserPermission pattern |
| RAG | "How does hybrid search work?" | RAG pipeline docs | Vector 70% + BM25 30% |
| Deployment | "How do I deploy to Fly.io?" | deployment docs | fly.toml, secrets, Dockerfile |

**Scoring the Results:**

| Score | Criteria |
|-------|----------|
| 5 | Perfect answer with file reference, correct code pattern, and gotchas |
| 4 | Correct answer with minor omission (missing gotcha or file reference) |
| 3 | Partial answer — right direction but incomplete |
| 2 | Vague answer — mostly correct but missing key details |
| 1 | Wrong answer or hallucination |

**The Dogfooding Scorecard:**

| Metric | Target | Actual |
|--------|--------|--------|
| Questions answered correctly (4+) | > 80% | ___ |
| File references included | > 90% | ___ |
| Gotchas mentioned when relevant | > 70% | ___ |
| Hallucination rate | < 5% | ___ |

**Do This:**
- Run the 8 test questions above
- Score each response
- Calculate the dogfooding scorecard
- Identify the weakest category
- Rewrite the KB.MD section for that category and re-test

**Think About This:**
- Which questions scored lowest? Why?
- Did the RAG pipeline retrieve the right chunks but the LLM still gave a bad answer? (Prompt engineering problem)
- Did the RAG pipeline retrieve the wrong chunks? (Retrieval problem)
- Did the RAG pipeline fail to retrieve anything? (Indexing problem)
- How would you distinguish between these three failure modes?

---

#### 19.4 Using RAG to Implement bugs/033

**The Scenario:**
You're a developer who just joined the bchat project. Your first task: implement the webhook integrations feature (bugs/033). You have the codebase assistant running. How do you use it?

**Phase 1: Understanding the Feature**

Ask the codebase assistant:
- "What is the webhook integrations feature?"
- "What database tables does it use?"
- "What API endpoints does it expose?"
- "How does the event outbox pattern work?"

Observe what the assistant retrieves. Does it find the relevant documentation? Does it explain the architecture?

**Phase 2: Understanding the Code**

Ask:
- "Where is the webhook dispatcher implemented?"
- "How does SSRF protection work?"
- "How do I add a new event type?"
- "What's the difference between SQLite and Postgres implementations for events?"

The assistant should point you to `integrations.go`, `vectordb_lance.go`, and the migration files.

**Phase 3: Implementing the Feature**

As you implement, ask the assistant for help:
- "Show me the pattern for adding a new store method" → retrieves the 5-step backend pattern
- "What's the migration file naming convention?" → retrieves `NN__snake_case.sql`
- "How do I register a new route?" → retrieves `v1.go` route registration pattern
- "What security patterns should I follow?" → retrieves SSRF protection and HMAC signing docs

**Phase 4: Testing the Feature**

Ask:
- "How do I write tests for the store layer?" → retrieves test patterns
- "What does the code review checklist look like?" → retrieves adversarial review patterns
- "How do I validate my migration?" → retrieves `task validate:migrations`

**The Business Value:**
- A new developer goes from zero to productive in hours, not weeks
- The codebase assistant reduces "shoulder tapping" on senior developers
- The assistant catches gotchas before they become bugs
- The assistant enforces coding conventions consistently

**Do This:**
- Use the codebase assistant to answer 5 questions about implementing bugs/033
- For each question, note: (a) what the assistant retrieved, (b) whether the answer was useful, (c) what's missing
- Identify the 3 most helpful responses and the 3 least helpful responses
- Rewrite the KB.MD sections that produced the least helpful responses

**Think About This:**
- Could the codebase assistant replace the AGENTS.md file? Why or why not?
- What questions would the assistant fail on? (Hint: questions about code that isn't documented)
- How does the assistant's value change as the codebase grows?

---

#### 19.5 When RAG Fails: Learning the Limitations

**Failure Mode 1: The Documentation Gap**
- Question: "How does the bridge authentication system work?"
- RAG retrieves: Nothing relevant (bridge docs aren't in the KB)
- Response: "This is not covered in the current documentation."
- **Lesson:** RAG can only retrieve what's indexed. Missing documentation = missing answers.

**Failure Mode 2: The Chunking Boundary Problem**
- Question: "What's the complete list of RBAC permissions?"
- RAG retrieves: The permissions section, but the table is split across two chunks
- Response: Partial list (misses 3 permissions)
- **Lesson:** Tables and lists that span chunks lose context. Structure your KB.MD so each chunk is self-contained.

**Failure Mode 3: The Semantic Mismatch**
- Question: "How do I add a new annotation type?"
- RAG retrieves: The annotation reference section (what annotations exist)
- Misses: The "adding features" section (how to add new annotations)
- **Lesson:** The same question can match multiple chunks. Hybrid search helps but isn't perfect.

**Failure Mode 4: The Outdated Knowledge**
- Question: "What's the current webhook delivery mechanism?"
- RAG retrieves: An outdated section from an older doc version
- Response: References fire-and-forget goroutines (replaced by outbox pattern)
- **Lesson:** RAG indexes what you upload. Stale docs = stale answers. Reindex after updates.

**Failure Mode 5: The Confidence Problem**
- Question: "What's the maximum number of concurrent webhook deliveries?"
- RAG retrieves: The polling interval (30 seconds) and batch size (10 events)
- Response: "Up to 10 events every 30 seconds" (correct but sounds uncertain)
- **Lesson:** RAG gives you chunks, not confidence scores. The LLM's phrasing may understate certainty.

**Do This:**
- Find 3 questions that produce bad answers from the codebase assistant
- For each, identify the failure mode (documentation gap, chunking boundary, semantic mismatch, outdated knowledge, or confidence problem)
- Fix the underlying issue (rewrite the KB.MD section, add missing docs, reindex)
- Re-test and verify the fix

**Think About This:**
- How do you know when the assistant is wrong vs. when you're asking the wrong question?
- What's the cost of acting on a wrong answer from the assistant?
- How would you build a "confidence meter" for the assistant's responses?

---

#### 19.6 Beyond the Codebase: Creative RAG Applications

**Application 1: The "Business Knowledge" Tenant**
- A restoration company uploads their KB.MD with services, pricing, coverage areas
- A plumber uploads their service catalog and emergency procedures
- An insurance agency uploads their policy types and claims process
- **Business value:** Each tenant gets a custom-trained agent without custom code

**Application 2: The "Compliance" Tenant**
- Upload regulatory documents as KB.MD sections
- Use `@safety` annotations for compliance rules with trigger intents
- Agent cross-references customer requests against compliance rules
- **Business value:** Automated compliance checking at conversation time

**Application 3: The "Internal Wiki" Tenant**
- Upload company policies, HR docs, IT procedures
- Employees ask questions instead of searching Confluence
- RAG retrieves the relevant policy section
- **Business value:** Faster onboarding, reduced HR ticket volume

**Application 4: The "Product Catalog" Tenant**
- Upload product specifications, pricing, comparison charts
- Customer asks "What's the difference between Product A and B?"
- RAG retrieves the comparison section from the catalog
- **Business value:** Consistent product knowledge across all sales channels

**Application 5: The "Training Data" Tenant**
- Upload past customer conversations as KB.MD
- Use observational memory to learn from historical interactions
- Agent improves over time as more conversations are indexed
- **Business value:** Continuous improvement without manual retraining

**Do This:**
- Pick one of the creative applications above
- Write a mock KB.MD and POLICY.MD for it
- Upload to a new tenant and test with 5 questions
- Identify what works and what doesn't

**Think About This:**
- Which application has the highest business value? Why?
- What's the minimum viable KB.MD for each application?
- How would you charge for each application? (Usage-based? Per-tenant flat fee?)

---

#### 19.7 The Dogfooding Checklist

Before you ship a feature, ask:

- [ ] **Can I use my own product to learn about my own product?**
  - Build a codebase assistant tenant
  - Test it against the 8-question matrix
  - Score it on the dogfooding scorecard

- [ ] **Does the RAG pipeline retrieve the right chunks?**
  - For each feature, identify the 5 most likely questions
  - Verify the correct chunks are retrieved
  - Check for chunking boundary issues

- [ ] **Does the agent answer correctly?**
  - Score responses on the 1-5 scale
  - Identify hallucination patterns
  - Verify file references are accurate

- [ ] **Can I fix bad answers by rewriting the KB.MD?**
  - Find the 3 worst answers
  - Identify the failure mode
  - Rewrite the relevant KB.MD section
  - Re-test and verify improvement

- [ ] **Does the feature work for my customers?**
  - If the codebase assistant can't answer "How do I add a new API endpoint?"
  - Then a restoration company's agent can't answer "How do I schedule a water extraction?"
  - The failure modes are the same; only the content differs

**The Final Test:**
Ask the codebase assistant: "What did you learn from this chapter?"

If it can answer that question using RAG retrieval, you've proven the product works.
If it can't, you've found a gap in your documentation.

Either way, you win.

---

### Chapter 19 Integration with Previous Chapters

**Cross-references:**
- Ch 3 (Schema Design): The codebase assistant uses the same schema as any tenant
- Ch 4 (Store Layer): The assistant helps implement new store methods
- Ch 5 (Webhook Dispatcher): The assistant explains how the dispatcher works
- Ch 7 (Testing): The assistant helps write tests for new features
- Ch 8 (Migration Strategy): The assistant explains migration conventions
- Ch 13 (Monitoring): The assistant helps set up monitoring for new features
- Ch 15 (Reading Code): The assistant is the ultimate code navigation tool
- Ch 16 (Evaluating Decisions): The assistant explains why decisions were made
- Ch 17 (Survivable Code): The assistant reminds you of defensive programming patterns
- Ch 18 (When to Ship): The assistant helps you know when a feature is ready

---

### Updated Tutorial Structure

The full tutorial is now **9 Parts | 19 Chapters**:

| Part | Chapters | Focus |
|------|----------|-------|
| I | 1-2 | Product Thinking |
| II | 3-4 | Database Design |
| III | 5-7 | Implementation |
| IV | 8-9 | Database Migration |
| V | 10-12 | Deployment |
| VI | 13-14 | Operations |
| VII | 15-18 | Critical Thinking |
| **VIII** | **19** | **Dogfooding RAG** |

Chapter 19 serves as both a standalone chapter and a capstone that ties everything together. It proves the product works by using it to teach itself.

---

Does this chapter meet your requirements? A few questions:

1. **Placement**: Should Chapter 19 come after the critical thinking chapters (as the capstone), or earlier (as a tool to use during learning)?

2. **Depth**: Should we include the actual KB.MD/POLICY.MD content for the codebase assistant, or leave it as an exercise for the learner?

3. **Scoring**: Should the dogfooding scorecard be a formal part of the tutorial, or an optional exercise?

4. **Creative applications**: Should we go deeper on any of the 5 creative applications, or keep them as inspiration prompts?

5. **Business integration**: How much should this chapter emphasize the "prove it works" narrative vs. the "learn faster" narrative?

---

## User

1) capstone 2) leave as exercise 3) dogfooding as part of tutorial but do not make it formal 4) keep as prompts 5) the business integration is the part where the learner applies the knowledge and make it their own business for profit as entrepreneurs, the goal here is not to think as an employee but as creative producers

---

## Assistant (Plan · MiMo V2.5 Free · 99.5s)

_Thinking:_

The user wants me to finalize the plan for the tutorial. Let me incorporate their feedback:

1. Chapter 19 stays as capstone (after critical thinking chapters)
2. Leave KB.MD/POLICY.MD content as exercise for learner
3. Dogfooding is part of tutorial but informal (not a formal checklist)
4. Creative applications stay as prompts
5. Business integration is about the learner becoming an entrepreneur - thinking as a creative producer, not an employee

Let me finalize the complete outline and present it for approval.

Understood. Let me finalize the complete outline incorporating all your feedback. The key refinement for Chapter 19 is the entrepreneurial angle — the learner applies what they've learned to build their own business, not just learn the codebase.

**Tool: task**

**Input:**
```json
{
  "description": "Explore bchat business model docs",
  "prompt": "Read these files to understand the business/entrepreneurial angle of bchat:\n\n1. `docs/docs_biz_guide.md` - business guide\n2. `docs/DOCS_HOWTO_BIZ.md` - how-to business guide\n3. `docs/BIZ_QUICK_START_CHECKLIST.MD` - quick start checklist\n4. `TUTORIAL.md` - look for the business/entrepreneurial framing\n5. `bugs/033/plan_biz.md` - the business plan (first 200 lines)\n\nFocus on:\n- How does bchat position itself as a business opportunity?\n- What are the pricing models?\n- How does a non-technical person use bchat to serve clients?\n- What's the \"entrepreneurial producer\" angle?\n- How does the learner become a business owner, not just a developer?\n\nReturn a summary of the business opportunity and how a developer can turn bchat into a profitable business.",
  "subagent_type": "explore"
}
```

**Output:**
```
<task id="ses_0b31e8bd0ffeShtNrRmFryZDJ9" state="completed">
<task_result>
---

## Summary: The bchat Business Opportunity

### What bchat Is (Business View)

bchat is a **multi-tenant AI chat agent platform** that acts as a "digital front desk" for local service businesses. A single server deployment can power an unlimited number of independent AI chat agents -- one per client business -- with no code changes between clients. Each business (called a "tenant") gets its own AI agent, chat widget, lead capture pipeline, and admin dashboard, all fully data-isolated.

The positioning statement is not "we sell AI chatbot software." It is:

> **"You keep doing the work. We handle the digital front desk."**

Or more precisely: "We make sure your website never ignores a prospect again."

---

### The Business Model: AI Lead Desk for Local Service Businesses

The recommended business model is to sell bchat as a **managed service** to local service businesses -- plumbers, electricians, pressure washing, junk removal, cleaning, landscaping, pest control, restoration companies. The key insight is that these businesses:

- Have high-intent, urgent leads (someone's basement is flooding at 10 PM)
- Are often in the field and cannot answer phones
- Have simple knowledge bases (services, pricing factors, coverage areas, FAQs)
- Do not have heavy compliance overhead
- Often have no website or a broken one (Facebook-only presence)

**Three-Tier Pricing:**

| Package | Target Client | Setup Fee | Monthly Fee | What It Includes |
|---------|--------------|-----------|-------------|-----------------|
| **Chat Widget Only** | Business already has a website | $99-$299 | $149-$299 | Widget install, KB/Policy training, lead capture, email/SMS alerts, human takeover, weekly report |
| **Hugo Site + AI Lead Desk** | No website or bad website | $399-$999 | $199-$499 | Fast landing site, domain/DNS, service pages, widget, notification workflow, Google Business Profile optimization |
| **Managed Lead Desk** | Growing SMBs wanting white-glove | $499-$1,500 | $499-$1,500 | Everything above plus CRM/Sheets pipeline, appointment routing, missed-lead follow-up, conversation review, monthly optimization, owner dashboard |

**Your Cost Per Tenant:**

| Component | Monthly Cost |
|-----------|-------------|
| OpenRouter API (LLM calls) | $5-$50 |
| Server hosting (Fly.io) | $15-$50 total for 25-50 tenants |
| LanceDB storage | Negligible |
| Domain + SSL | $1-$2 |
| **Total per tenant** | **$5-$30/month** |

This yields **80-90% gross margins** on the $199-$499/month retail price.

---

### How a Non-Technical Person Uses bchat to Serve Clients

The business guide (`docs_biz_guide.md` and `DOCS_HOWTO_BIZ.md`) walks through a complete workflow that does not require the end client (the plumber, the cleaning company) to write code:

1. **You (the deployer) set up the server** -- one-time technical setup on Fly.io or a VPS
2. **You create a tenant** in the admin panel -- fill in the business name, slug, LLM model
3. **You write three markdown files** for each client:
   - **KB.MD** -- the business's knowledge base (services, FAQs, coverage areas, pricing guidance, emergency protocols). Written as if training a smart new hire.
   - **POLICY.MD** -- the agent's operating rules (identity, tone, escalation triggers, urgency thresholds, lead capture rules, fallback behavior)
   - **SCRIPT.MD** -- conversation flow stages (Opening, Discovery, Information, Qualification, Closing)
4. **You upload the files** and click "Rebuild Index" to make them searchable via RAG
5. **You embed the widget** on the client's website with a single `<script>` tag
6. **You test** using built-in simulation sandbox (virtual customers with personas)
7. **The client reviews leads** in the admin panel -- no coding required

The KB.MD is the most important file. The documentation provides detailed templates and annotation syntax (like `<!-- @service: water_extraction, emergency: true -->`) that make content machine-readable for the RAG system. The agent handles everything from emergency escalation to FAQ resolution to lead capture automatically.

---

### The "Entrepreneurial Producer" Angle

The TUTORIAL.md frames bchat as a case study in **production software as a business**, not just a coding exercise. The key entrepreneurial insight is:

**bchat replaces $246/month in third-party SaaS** per deployment:
- Intercom AI: $97/month (chat + booking)
- Helios AI: $100/month (SMS follow-up)
- n8n: $49/month (workflow automation)

The webhook-first strategy (documented in `plan_biz.md`) is itself an entrepreneurial decision: instead of building expensive native integrations for every CRM and calendar tool, build one generic webhook that connects to Zapier (5,000+ app integrations). This keeps the codebase small, the costs low, and the value proposition clear.

The business plan is structured as a **consulting-to-SaaS pipeline**:
1. **Start as a service business** -- you are the "digital front desk" provider
2. **Charge setup + monthly** -- recurring revenue from day one
3. **Scale by adding tenants** -- each new client costs you $5-$30/month but generates $149-$1,500/month
4. **Optionally build your own SaaS** -- if the model works, productize the deployment and let clients self-serve

---

### How a Developer Turns bchat Into a Profitable Business

**Step 1: Deploy the platform** (one-time, ~2 hours)
- Clone the repo, build with RAG support, deploy to Fly.io
- Total infrastructure cost: $15-$50/month for the server

**Step 2: Pick a vertical** (choose one to start)
- Pressure washing, junk removal, cleaning, landscaping, or pest control
- These have urgent leads, simple knowledge bases, and owners who are too busy to answer phones

**Step 3: Onboard your first client** (1-2 days per client)
- Interview the business owner (30 minutes)
- Write KB.MD, POLICY.MD, SCRIPT.MD using the templates
- Upload, rebuild index, run 3+ simulations
- Embed the widget on their website
- Charge $149-$299 setup + $149-$299/month

**Step 4: Deliver value and collect leads**
- The agent runs 24/7, capturing leads while the business owner sleeps
- You provide weekly lead reports, conversation reviews, and monthly optimization
- Each lead that converts to a customer validates the service

**Step 5: Scale**
- Each additional tenant costs you $5-$30/month
- You charge $149-$1,500/month
- At 10 tenants at $299/month average: $2,990/month revenue, ~$300/month cost
- At 50 tenants: $14,950/month revenue, ~$1,500/month cost

**Step 6: Optional productization**
- Add a Hugo site builder for businesses without websites
- Offer managed lead desk (CRM integration, appointment routing, follow-up sequences)
- Build pre-built Zapier templates for common integrations
- Create a self-service onboarding flow

---

### Key Business Differentiators

1. **No code changes between clients** -- the agent is general-purpose; all customization is in markdown files
2. **Instant updates** -- change KB.MD, rebuild index, agent behavior changes immediately (no retraining, no redeployment)
3. **Conversational lead capture** -- no forms; the agent extracts name, email, phone naturally during conversation
4. **24/7 availability** -- the agent handles after-hours emergencies, collects leads, and promises callbacks
5. **Multi-tenant data isolation** -- each client's data is completely separate
6. **Simulation testing** -- test the agent with virtual customers before going live
7. **RAG-powered accuracy** -- hybrid search (70% vector + 30% keyword) ensures grounded, non-hallucinated answers
8. **80-90% gross margins** -- your cost per tenant is $5-$30/month; you charge $149-$1,500/month

---

### The Bottom Line

bchat is not just a technical project -- it is a **business-in-a-box** for developers who want to serve local service businesses. The platform handles the hard parts (AI agent, RAG pipeline, lead capture, multi-tenant isolation, security, simulation testing). The developer's job is to write three markdown files per client, embed a widget, and collect monthly recurring revenue. The entrepreneurial producer angle is that you are not just building software -- you are building a **service business** with software as the delivery mechanism.
</task_result>
</task>
```

---

## Assistant (Plan · MiMo V2.5 Free · 158.3s)

_Thinking:_

Now I have all the information needed. Let me finalize the complete tutorial outline incorporating all user feedback:

1. Chapter 19 as capstone
2. Leave KB.MD/POLICY.MD content as exercise
3. Dogfooding informal (not formal checklist)
4. Creative applications stay as prompts
5. Business integration = entrepreneur/business owner angle

Let me present the final, complete outline for approval.

Now let me present the final complete outline.

---

# BCHAT TUTORIAL v4: FINAL OUTLINE

## Building Production-Grade SaaS: From Code to Business

---

## COMPLETE TUTORIAL STRUCTURE

**9 Parts | 19 Chapters | ~130 Sections**

**Deliverable:** Markdown only, organized as:

```
bchat-tutorial/
├── README.md
├── PART-I-PRODUCT-THINKING/
│   ├── 01-market-opportunity.md
│   └── 02-architecture-decisions.md
├── PART-II-DATABASE-DESIGN/
│   ├── 03-multi-tenant-schema.md
│   └── 04-store-layer.md
├── PART-III-IMPLEMENTATION/
│   ├── 05-webhook-dispatcher.md
│   ├── 06-admin-ui.md
│   └── 07-testing.md
├── PART-IV-DATABASE-MIGRATION/
│   ├── 08-migration-strategy.md
│   └── 09-production-migrations.md
├── PART-V-DEPLOYMENT/
│   ├── 10-docker.md
│   ├── 11-flyio.md
│   └── 12-pre-deploy-validation.md
├── PART-VI-OPERATIONS/
│   ├── 13-monitoring.md
│   └── 14-incident-response.md
├── PART-VII-CRITICAL-THINKING/
│   ├── 15-reading-code.md
│   ├── 16-evaluating-decisions.md
│   ├── 17-writing-survivable-code.md
│   └── 18-when-to-ship.md
└── PART-VIII-META-LEARNING/
    └── 19-dogfooding-rag.md
```

---

## PART I: PRODUCT THINKING — Before You Write a Line

### Chapter 1: The Market Opportunity

**1.1 The $246/mo Problem**
- Small service businesses pay for 3 fragmented tools: Intercom AI ($97/mo), Helios AI ($100/mo), n8n ($49/mo)
- Stitching them together costs time, money, and reliability
- The gap: a single platform that handles AI chat, lead capture, AND external integrations

**1.2 What Each Competitor Actually Delivers**

| | Intercom AI | Helios AI | n8n |
|---|---|---|---|
| Core | AI chat agent (Fin) | Managed AI assistant | Workflow automation |
| Pricing | $97/mo per seat | $100/mo per business | $49/mo per tier |
| Setup | Self-service | Managed (white-glove) | Self-service (technical) |
| KB Config | Built-in articles | Managed loading | N/A |
| SMS | Yes | Yes (WhatsApp) | Via automation |
| Calendar | Yes | Yes (Cal.com) | Via automation |
| Extensibility | API + marketplace | Closed | 400+ integrations |
| Open Source | No | No | Yes |

**1.3 bchat's Positioning: The All-in-One Alternative**
- AI chat agent with RAG pipeline (replaces Intercom AI)
- Configurable via markdown files, self-service (replaces Helios AI's managed model)
- Native webhook events (replaces n8n for lead routing)
- 80-90% gross margins: $5-$30/tenant/month internal cost vs. $149-$499 retail

**1.4 The Strategic Pivot in bugs/033**
- Original plan: native Twilio SMS, Google Calendar, Outlook (2-3 months)
- Pivot: webhook-first ships in weeks, integrates with everything
- Key insight from `bugs/033/prompt.md`: "Personal AI agents can consume webhook events — bchat becomes the source of truth"
- Summary of 5 plan iterations: each killed assumptions the previous one made

**Do This:**
- Read `bugs/033/plan_biz.md` (first 100 lines) — identify the 3-phase roadmap
- Read `bugs/033/plan_biz_review.md` — count the critical vs. major findings
- Read `bugs/033/prompt.md` — find the exact paragraph where the pivot happens

**Think About This:**
- Why is "scope management" more valuable than "coding speed" for a product?
- If bchat builds native SMS + Calendar, what maintenance cost does that create forever?
- Is "webhook-first" a compromise or an upgrade? Under what conditions?

---

### Chapter 2: Architecture as a Product Decision

**2.1 Why Architecture Decisions Are Product Decisions**
- Every technical choice constrains or enables future features
- The "Path A vs. Path B" debate: platform-managed vs. webhook-first
- Path A: bchat owns Twilio subaccounts, auto-provisions per tenant (batteries included)
- Path B: bchat emits events, tenant connects via Zapier/Make (extensible but more friction)
- Decision: Path B for v1, Path A deferred — speed to market won

**2.2 The "Same Transaction" Trap**
- plan3 claimed: "insert event and deliver webhook in the same transaction"
- plan3_review killed it: "unimplementable — Store layer has no transaction API"
- **Lesson:** If a plan contradicts the data model, it's wrong

**2.3 The Pre-Claimed Events Pattern**
- Race condition: Insert event → immediate dispatch → poller also claims → double delivery
- Solution: Insert with `status='processing'` and `claimed_at=now()` — poller skips it
- **Lesson:** Race conditions are about state transitions

**2.4 Idempotency as a First-Class Concern**
- Network errors after commit are indistinguishable from non-delivery
- Key: `hash(tenantID, leadID, eventType, integrationID)`
- PostgreSQL unique constraint (`23505`) treated as success
- **Lesson:** Design for the failure mode, not the happy path

**2.5 The Review Culture**
- Every plan had adversarial review with structured checklists
- `plan_biz_review.md`: 5 critical + 6 major → REWORK
- `plan3_review.md`: outbox architecture flaw → REWORK
- `plan4_review.md`: APPROVED with 4 amendments
- **Lesson:** Code review is about finding wrong assumptions, not bugs

**Do This:**
- Compare `plan3_review.md` (REWORK) with `plan4_review.md` (APPROVED)
- Trace how "pre-claimed events" is implemented in `integrations.go`

**Think About This:**
- Why did plan3 fail but plan4 succeed?
- How does "adversarial review" differ from "code review"?

---

## PART II: DATABASE DESIGN — Schema as Contract

### Chapter 3: Multi-Tenant Schema Design

**3.1 The Two New Tables (0.31 migrations)**
- `agent_integrations`: webhook configurations per tenant
- `agent_events`: outbox queue for reliable event delivery
- Conceptual walkthrough of each column

**3.2 Design Decisions Explained**

| Decision | Choice | Why |
|----------|--------|-----|
| Integration uniqueness | `UNIQUE(tenant_id, integration_type)` | One webhook per type per tenant |
| Event status on insert | `status = 'processing'` | Pre-claiming eliminates race condition |
| Idempotency key | `hash(tenantID, leadID, eventType, integrationID)` | Deterministic deduplication |
| Cascade deletes | `ON DELETE CASCADE` | No orphaned data |
| Lease expiry | `claimed_at < now() - 300` | 5-minute recovery for crashed processes |

**3.3 SQLite vs. PostgreSQL: Same Schema, Different SQL**
- Dialect differences with examples

**3.4 State Machine Thinking**
- Event states: `processing` → `delivered` / `dead`
- Every "stuck" state needs a recovery path

**Do This:**
- Write SQL to find events stuck in `processing` for >5 minutes
- Design a schema for "event delivery log" feature

**Think About This:**
- `ON DELETE CASCADE` — what's the blast radius of deleting a tenant with 10,000 events?
- Why not PostgreSQL `LISTEN/NOTIFY` instead of polling?

---

### Chapter 4: The Store Layer Abstraction

**4.1 The Driver Interface**
- `store/driver.go`: 60+ methods covering all entity CRUD
- Adding a new entity: types → interface → implementations

**4.2 SQLite Implementation**
- Pure Go, `INSERT OR IGNORE`, busy timeout

**4.3 PostgreSQL Implementation**
- `pgx/v5`, `ON CONFLICT DO NOTHING`, `FOR UPDATE SKIP LOCKED`

**4.4 PostgreSQL Resilience**
- Neon transient error codes, retry with backoff + jitter

**Do This:**
- Compare `ClaimEvents` in SQLite vs. PostgreSQL
- Explain why SQLite can't use `FOR UPDATE SKIP LOCKED`

**Think About This:**
- Why `pgx/v5` instead of `database/sql` with `lib/pq`?
- How does the driver abstraction hide differences but not eliminate them?

---

## PART III: IMPLEMENTATION — Building the Feature

### Chapter 5: Backend — The Webhook Dispatcher

**5.1 Conceptual Flow**
```
Lead captured → Insert event (pre-claimed) → Immediate dispatch
    ├── Success: status → 'delivered'
    └── Failure: Poller claims (every 30s) → Retry → delivered/dead
```

**5.2 SSRF Protection**
- Three-layer defense: URL parsing → DNS resolution → IP validation
- Security code that's hard to import is better than no security code

**5.3 HMAC Signing**
- `X-Bchat-Signature` header proves identity

**5.4 The Poller**
- Background goroutine, 30-second interval, claims stale events

**Do This:**
- Trace flow from lead capture to webhook delivery
- Find where HMAC signature is computed

**Think About This:**
- `http.Client{Timeout: 10s}` — what if endpoint is slow but not down?
- How does `kill_timeout = "30s"` interact with in-flight deliveries?

---

### Chapter 6: Frontend — Admin UI for Integrations

**6.1 Conceptual Flow**
```
IntegrationsSection.tsx → MobX store → HTTP → Handler → Service → Store
```

**6.2 The Component Structure**
- 296-line React component: list, create/edit, test, delete

**6.3 Pattern Recognition**
- Compare with other admin sections — identify repeating patterns

**6.4 The PII Masking Decision**
- `HandleListEvents` returns `safeEvent` — no payload (PII)

**Do This:**
- Compare `IntegrationsSection.tsx` with another admin section
- Identify the 3 patterns that repeat

**Think About This:**
- Why MobX instead of Redux?
- How would you add real-time event status updates?

---

### Chapter 7: Testing — What Catches What

**7.1 The Bugs That Slipped Through**
- Bug 1: Idempotency key argument swap — all events deduplicated to one integration
- Bug 2: Poller context cancelled before delivery completes

**7.2 What Was Tested**
- HMAC signing, SSRF protection, idempotency key determinism

**7.3 The Testing Pyramid**

**7.4 What Tests Would Catch Bugs 1 & 2**

**Do This:**
- Write a test that catches the argument-order bug
- Write a test that catches the context-cancellation bug

**Think About This:**
- Code review found bugs tests missed — what's the relationship?
- How do you write tests that catch argument-order bugs?

---

## PART IV: DATABASE MIGRATION — SQLite to PostgreSQL

### Chapter 8: Migration Strategy

**8.1 The Two-Database Reality**
- SQLite for dev, PostgreSQL for production

**8.2 Migration File Structure**

**8.3 Dialect Differences**

| Feature | SQLite | PostgreSQL |
|---------|--------|------------|
| Auto-increment | `INTEGER PRIMARY KEY AUTOINCREMENT` | `SERIAL` |
| Timestamps | `datetime('now')` | `NOW()` |
| Booleans | `INTEGER` (0/1) | `BOOLEAN` |
| JSON | `TEXT` | `JSONB` |
| Upsert | `INSERT OR IGNORE` | `ON CONFLICT DO NOTHING` |

**8.4 LATEST.sql: The Cumulative Schema**

**Do This:**
- Write SQLite migration for `agent_webhook_logs`
- Write PostgreSQL version
- Run `task validate:migrations`

**Think About This:**
- What happens if a migration fails midway?
- How would you migrate a production database with zero downtime?

---

### Chapter 9: Running Migrations in Production

**9.1 Programmatic Migration**
- Runs on server startup via `store/migrator.go`

**9.2 Neon Connection Configuration**
- SSL, `simple_protocol`, connection pooling

**9.3 Validation Commands**

**Do This:**
- Run `task validate:schema` on the current codebase
- Create a test migration, run it, verify in `migration_history`

**Think About This:**
- Neon suspends when idle — what happens to in-flight queries?
- What's your rollback strategy if a migration corrupts data?

---

## PART V: DEPLOYMENT — Getting to Production

### Chapter 10: Docker Containerization

**10.1 The Three-Stage Dockerfile**
- Frontend (Node.js) → Backend (Go) → Runtime (Ubuntu 24.04)

**10.2 Security in the Container**
- Non-root user, read-only filesystem, `_FILE` secrets

**10.3 Supercronic: Container Cron**
- External process, survives Go panics

**Do This:**
- Build the Docker image locally
- Run the container and verify health check

**Think About This:**
- Why Ubuntu instead of Alpine?
- How does `gosu` differ from `su`?

---

### Chapter 11: Fly.io Deployment

**11.1 Configuration Anatomy**
- `fly.toml`: app, region, build, VM specs

**11.2 Secrets Management**
- `fly secrets set`, `_FILE` suffix, no secrets in git

**11.3 The Deployment Flow**

**11.4 Neon Integration**
- `DATABASE_URL`, SSL, auto-suspend/resume

**Do This:**
- Deploy to Fly.io with Neon database
- Scale down to 0 machines, hit API, verify wake-up

**Think About This:**
- `auto_stop_machines` — what happens to in-flight deliveries?
- What's the cold-start time?

---

### Chapter 12: Pre-Deployment Validation

**12.1 The Validation Chain**
```bash
validate-env-chain.sh → validate-db-migrations.sh → task validate:schema → task build:rag → fly deploy
```

**12.2 Why Validate Before Deploy**

**Do This:**
- Run the full validation chain
- Introduce a deliberate error and see how validation catches it

**Think About This:**
- Failed deploy vs. blocked deploy — which costs more?
- How would you add a smoke test after deploy?

---

## PART VI: PRODUCTION OPERATIONS — Keeping It Running

### Chapter 13: Monitoring & Observability

**13.1 Health Check**
- `/healthz`: database + LanceDB connectivity

**13.2 Logs to Watch**
- `webhook delivered`, `webhook delivery failed`, `event claimed`, `event moved to dead`

**13.3 Metrics to Track**

| Metric | Target | Action |
|--------|--------|--------|
| Delivery success rate | > 99% | Investigate endpoints |
| Average latency | < 2s | Check network |
| Dead letter rate | < 0.1% | Review configs |
| DB connections | < 80% | Scale pool |

**Do This:**
- Set up log monitoring for webhook events
- Write an alert rule for dead letter rate > 1%

**Think About This:**
- How does `auto_stop` interact with monitoring?
- What would you monitor with 10x more tenants?

---

### Chapter 14: Incident Response

**14.1 Severity Levels**
- P0: All webhooks down → P3: Cosmetic issue

**14.2 Runbook: Events Stuck in "processing"**
- Diagnosis → Mitigation → Resolution → Prevention

**14.3 Post-Mortem Template**

**Do This:**
- Write a runbook for "webhook endpoint returns 429"
- Identify the 3 most likely failure modes

**Think About This:**
- A tenant reports "no webhook deliveries" — walk through diagnostics
- How do you balance "fix it now" with "understand why"?

---

## PART VII: CRITICAL THINKING — The Meta-Skills

### Chapter 15: Reading Code You Didn't Write

**15.1 Navigation Strategy**
- Entry point → request flow → tests → git log

**15.2 Pattern Recognition**
- Handler, Store, Frontend patterns

**15.3 When Code Confuses You**
- Follow the call graph, not the file order

**Do This:**
- Trace the full request flow for `POST /api/v1/agent/:slug/integrations`
- Draw the call graph on paper

**Think About This:**
- Which layer is hardest to understand? Why?
- How would you explain the webhook system to a non-technical person?

---

### Chapter 16: Evaluating Technical Decisions

**16.1 The Tradeoff Framework**
- Development time, operational complexity, cost, failure modes, reversibility

**16.2 Case Study: Webhooks vs. Native Integrations**

| | Webhooks | Native |
|---|---|---|
| Dev time | Weeks | Months |
| Complexity | Lower | Higher |
| Reversibility | High | Low |

**16.3 The Second System Effect**
- 5 plan iterations — each caught assumptions the previous made

**Do This:**
- Evaluate a technical decision using the framework
- Write a one-paragraph justification

**Think About This:**
- When should you build native vs. emit events?
- How do you know when a tradeoff is acceptable vs. a mistake?

---

### Chapter 17: Writing Code That Survives

**17.1 Defensive Programming**
- Idempotency keys, lease expiry, SSRF protection

**17.2 Operational Visibility**
- Structured logging, status tracking, audit trail

**17.3 Graceful Degradation**
- Log and continue, wait in database, fail fast

**Do This:**
- Add structured logging to a function using `fmt.Println`
- Design an audit trail for a new feature

**Think About This:**
- What's the difference between "defensive" and "paranoid" programming?
- How much logging is too much?

---

### Chapter 18: When to Stop and Ship

**18.1 The MVP Mindset**
- v1: Webhooks only, SMS deferred to v1.1

**18.2 Known Limitations (Documented Honestly)**
- Awake-only retries, SQLite write lock

**18.3 The Review Culture**
- Every plan had adversarial review

**Do This:**
- Read `plan4_review.md` — identify the 4 amendments
- Write a review comment for one finding

**Think About This:**
- How do you know when a feature is "good enough"?
- What's the cost of shipping too early vs. too late?

---

## PART VIII: META-LEARNING — Using the Product to Learn the Product

### Chapter 19: Dogfooding RAG: From Learner to Business Owner

**19.1 The Business Case for Dogfooding**

You've spent 18 chapters learning bchat from the outside in. Now flip the script.

**The "Eat Your Own Dog Food" Principle:**
- If the product can't teach you about itself, it can't teach anyone anything
- Build a tenant that uses KB.MD/POLICY.MD to teach developers about the bchat codebase
- If the RAG pipeline can't answer "How do I add a new API endpoint?" reliably, it can't answer "How do I schedule a water extraction appointment?" reliably either

**What Dogfooding Reveals:**
- RAG quality gaps (vague answers = your customers get vague answers)
- Chunking failures (bad splits = bad retrieval)
- Retrieval weaknesses (missed chunks = missed FAQs)
- Prompt engineering gaps (hallucination = grounded answers fail)

**The Meta-Lesson:**
- Writing a KB.MD for a codebase forces you to think like a product manager
- This is exactly the thinking your customers need to do for their businesses

**Think About This:**
- If you were onboarding a new developer, what are the 10 questions they'd ask first?
- How does writing a KB.MD for a codebase differ from writing one for a water extraction company?

---

**19.2 Building the Codebase Assistant**

**The Setup:**
- Create tenant: `slug = "bchat-dev"`, `company_name = "bchat Development Team"`
- Upload three files: KB_CODEBASE.MD, POLICY_CODEBASE.MD, SCRIPT_CODEBASE.MD
- The annotation system is the same as any tenant — the content is different

**The KB.MD Structure:**
- Use `<!-- @section: name, type: category -->` for each topic
- One section per concept: tenant isolation, RAG pipeline, build commands, gotchas
- Each section targets ~500 tokens for optimal chunking

**The POLICY.MD Structure:**
- `<!-- @identity -->`: "You are a codebase learning assistant"
- `<!-- @intent -->`: architecture_question, how_to_question, debugging_question, deployment_question
- `<!-- @rule -->`: Always cite files, never guess, explain the "why"

**The SCRIPT.MD Structure:**
- Conversation flow: understand request → locate code → explain architecture → suggest changes

**Do This:**
- Create the tenant, write the three files, upload and reindex
- Ask: "How do I add a new API endpoint?"
- Observe what RAG retrieves and how the agent responds

**Think About This:**
- What happens if you ask a question not covered in the KB?
- How does the quality of the KB.MD writing affect the quality of the answers?

---

**19.3 Testing RAG Against the Codebase**

**The Test Matrix:**

| Category | Question | What to Check |
|----------|----------|---------------|
| Architecture | "How does multi-tenancy work?" | JWT claims, context mechanism |
| How-To | "How do I add a new store method?" | 5-step backend pattern |
| Debugging | "Why is my migration failing?" | Filename convention |
| Reference | "What env vars do I need?" | Complete list |
| Gotchas | "What table naming convention?" | Plural names |
| Security | "How do I check permissions?" | CheckUserPermission |
| RAG | "How does hybrid search work?" | Vector 70% + BM25 30% |
| Deployment | "How do I deploy to Fly.io?" | fly.toml, secrets |

**Scoring:**
- 5: Perfect answer with file reference + gotchas
- 4: Correct with minor omission
- 3: Partial — right direction but incomplete
- 2: Vague — mostly correct but missing details
- 1: Wrong or hallucination

**Do This:**
- Run the 8 test questions
- Score each response
- Identify the weakest category
- Rewrite the KB.MD section and re-test

**Think About This:**
- Which questions scored lowest? Why?
- Did RAG retrieve the right chunks but the LLM gave a bad answer? (Prompt problem)
- Did RAG retrieve the wrong chunks? (Retrieval problem)

---

**19.4 Using RAG to Implement bugs/033**

**The Scenario:**
You're a new developer. Your first task: implement webhook integrations. You have the codebase assistant. How do you use it?

**Phase 1: Understanding the Feature**
- Ask: "What is the webhook integrations feature?"
- Ask: "What database tables does it use?"
- Ask: "How does the event outbox pattern work?"

**Phase 2: Understanding the Code**
- Ask: "Where is the webhook dispatcher implemented?"
- Ask: "How does SSRF protection work?"
- Ask: "What's the difference between SQLite and Postgres implementations?"

**Phase 3: Implementing the Feature**
- Ask: "Show me the pattern for adding a new store method"
- Ask: "What's the migration file naming convention?"
- Ask: "How do I register a new route?"
- Ask: "What security patterns should I follow?"

**Phase 4: Testing the Feature**
- Ask: "How do I write tests for the store layer?"
- Ask: "What does the code review checklist look like?"
- Ask: "How do I validate my migration?"

**The Business Value:**
- New developer goes from zero to productive in hours, not weeks
- Reduces "shoulder tapping" on senior developers
- Catches gotchas before they become bugs
- Enforces coding conventions consistently

**Do This:**
- Use the codebase assistant for 5 questions about implementing bugs/033
- For each, note: (a) what it retrieved, (b) whether the answer was useful, (c) what's missing

**Think About This:**
- Could the codebase assistant replace AGENTS.md? Why or why not?
- How does its value change as the codebase grows?

---

**19.5 When RAG Fails: Learning the Limitations**

**Failure Mode 1: The Documentation Gap**
- Question about undocumented code → "This is not covered"
- RAG can only retrieve what's indexed

**Failure Mode 2: The Chunking Boundary Problem**
- Table split across two chunks → partial answer
- Structure KB.MD so each chunk is self-contained

**Failure Mode 3: The Semantic Mismatch**
- Same question matches multiple chunks → hybrid search helps but isn't perfect

**Failure Mode 4: The Outdated Knowledge**
- Stale docs indexed → stale answers
- Reindex after updates

**Failure Mode 5: The Confidence Problem**
- LLM phrasing may understate certainty
- RAG gives chunks, not confidence scores

**Do This:**
- Find 3 questions that produce bad answers
- Identify the failure mode for each
- Fix the underlying issue and re-test

**Think About This:**
- How do you know when the assistant is wrong vs. when you're asking the wrong question?
- What's the cost of acting on a wrong answer?

---

**19.6 Creative Applications: From Code to Commerce**

This is where the tutorial shifts from "learning" to "building a business."

**The Entrepreneurial Mindset:**
You now understand how bchat works — architecture, database, deployment, operations. You've dogfooded the RAG pipeline. You've used the codebase assistant to learn the code. The next step is not to get a job. The next step is to build a business.

**The Five Business Models:**

**Model 1: The Service Business (Lowest Risk)**
- You are the "digital front desk" provider
- Pick a vertical: plumbers, electricians, pressure washing, junk removal
- Write KB.MD/POLICY.MD for each client
- Charge $149-$299 setup + $149-$299/month
- Your cost: $5-$30/tenant/month
- **Margin: 80-90%**

**Model 2: The Platform Business (Higher Reward)**
- Build a self-service onboarding flow
- Let clients configure their own agents via Admin UI
- Charge per-tenant monthly subscription
- Scale to 50+ tenants on one server
- **Revenue at 50 tenants: ~$15,000/month**

**Model 3: The Vertical SaaS (Highest Defensibility)**
- Pick ONE vertical (e.g., restoration companies)
- Build industry-specific templates (KB.MD, POLICY.MD, SCRIPT.MD)
- Add vertical-specific features (emergency protocols, insurance claim workflows)
- Charge premium pricing ($499-$1,500/month)
- **Defensibility: domain expertise + template library**

**Model 4: The Consulting + SaaS Pipeline**
- Start as a consultant (one-time setup fee)
- Transition to managed service (monthly recurring)
- Upsell to platform (self-service)
- **Path: $2,000 one-time → $300/month → $15,000/month at scale**

**Model 5: The Open-Source Monetization**
- Keep bchat open-source (community adoption)
- Offer managed hosting (bchat.cloud)
- Premium features: advanced analytics, priority support, custom integrations
- **Revenue: freemium + enterprise tiers**

**The Decision Framework:**

| Model | Risk | Reward | Time to Revenue | Scalability |
|-------|------|--------|-----------------|-------------|
| Service | Low | Medium | 1-2 weeks | Limited by your time |
| Platform | Medium | High | 1-2 months | Unlimited |
| Vertical SaaS | High | Very High | 3-6 months | Unlimited + defensible |
| Consulting → SaaS | Low | High | 1 week → 6 months | Grows with you |
| Open-Source | Low | Variable | 6+ months | Community-driven |

**Do This:**
- Pick one business model that fits your situation
- Write a one-page business plan: target vertical, pricing, cost structure, first 3 clients
- Identify the first client you would approach

**Think About This:**
- Which model has the lowest barrier to entry?
- Which model has the highest long-term value?
- What's your unfair advantage? (Technical skill? Domain knowledge? Sales ability?)
- How does the $246/mo replacement narrative help you sell?

---

**19.7 Your First Client: A Practical Exercise**

**The Scenario:**
You've deployed bchat. You have the codebase assistant running. Now you're going to onboard your first client: "Mike's Pressure Washing."

**Step 1: The Interview (30 minutes)**
Call Mike and ask:
- What services do you offer? (pressure washing, soft wash, gutter cleaning)
- What areas do you serve? (within 30 miles of Austin, TX)
- What are your hours? (Mon-Sat 7am-6pm, emergency available)
- What questions do customers ask most? (pricing, availability, what's included)
- What's your pricing structure? (by square footage, minimum $150)
- Do you have a website? (yes, but outdated)
- How do you currently handle leads? (voicemail, callback)

**Step 2: Write the KB.MD**
Using the templates from Chapter 3, write Mike's knowledge base:
- Services with annotations (each service = one chunk)
- FAQs (pricing, availability, what's included)
- Coverage areas (include/exclude)
- Exclusions (what you DON'T do: no roof cleaning, no interior work)

**Step 3: Write the POLICY.MD**
- Identity: "You are Mike's scheduling assistant"
- Intents: schedule_service, ask_pricing, check_availability, emergency
- Rules: always collect name + phone, never quote exact pricing without inspection

**Step 4: Write the SCRIPT.MD**
- Opening → Discovery → Information → Qualification → Closing

**Step 5: Upload, Reindex, Test**
- Upload files, click "Rebuild Index"
- Run simulation with 3 virtual customers
- Score the responses (4+ = pass)

**Step 6: Embed the Widget**
- Add `<script src="https://your-server.com/widget.js" data-tenant="mikes-pressure-washing">` to Mike's website

**Step 7: Go Live**
- Monitor first 10 conversations
- Adjust KB.MD based on real questions
- Send Mike his first lead report

**Do This:**
- Complete Steps 1-5 for a real or fictional business
- Run the simulation and score the results
- Identify the 3 weakest responses and improve the KB.MD

**Think About This:**
- What questions did Mike ask that you didn't expect?
- How did the simulation reveal gaps in your KB.MD?
- What would you charge Mike for this service? Why?

---

**19.8 The Dogfooding Loop: Continuous Improvement**

Dogfooding is not a one-time exercise. It's a continuous loop:

```
Use the product → Find gaps → Fix them → Use again → Find more gaps
```

**The Weekly Practice:**
- Every week, ask the codebase assistant 3 new questions
- Score the responses
- If any score below 4, rewrite the KB.MD section
- Reindex and re-test

**The Monthly Review:**
- Review all questions asked this month
- Identify patterns: which categories score lowest?
- Rewrite the weakest sections
- Add new sections for undocumented features

**The Business Parallel:**
- Your customers will do the same with their agents
- A plumber's KB.MD needs monthly updates (new services, pricing changes, seasonal FAQs)
- The weekly practice teaches you the rhythm your customers need

**Do This:**
- Set a weekly reminder to ask the codebase assistant 3 questions
- Track scores over time
- After 4 weeks, identify the trend: improving, stable, or degrading?

**Think About This:**
- How does continuous dogfooding change your relationship with the product?
- What would happen if you stopped dogfooding after a month?
- How would you teach your clients to do the same?

---

**19.9 From Learner to Producer: The Mindset Shift**

**The Employee Mindset:**
- "I build what I'm told"
- "My job is to write code"
- "Success = shipping features"

**The Producer Mindset:**
- "I build what the market needs"
- "My job is to solve problems"
- "Success = revenue + impact"

**The Shift:**
This tutorial taught you to think like a producer:
- Chapter 1-2: Product thinking (what to build and why)
- Chapter 3-4: Schema design as contract (what data matters)
- Chapter 5-7: Implementation (how to build it)
- Chapter 8-9: Migration (how to evolve it)
- Chapter 10-12: Deployment (how to ship it)
- Chapter 13-14: Operations (how to keep it running)
- Chapter 15-18: Critical thinking (how to think about it)
- Chapter 19: Dogfooding (how to use it to build a business)

**The Final Exercise:**
Write a one-page "Producer's Manifesto":
1. What problem do you want to solve?
2. Who has that problem?
3. How much are they willing to pay?
4. What's your cost to serve them?
5. What's your first step?

**Think About This:**
- Is the goal to build software, or to build a business that uses software?
- What's the difference between a developer and a producer?
- How does this tutorial change how you think about your next project?

---

## APPENDICES

### Appendix A: bugs/033 File Reference
### Appendix B: API Endpoint Reference
### Appendix C: Database Schema Reference
### Appendix D: Environment Variables
### Appendix E: Competitor Analysis
### Appendix F: Glossary

---

## LEARNING PATH (4-6 weeks)

| Week | Focus | Chapters |
|------|-------|----------|
| 1 | Product thinking + Architecture | 1-2 |
| 2 | Database design + Store layer | 3-4 |
| 3 | Backend + Frontend + Testing | 5-7 |
| 4 | Migration + Deployment | 8-12 |
| 5 | Operations + Critical thinking | 13-18 |
| 6 | Dogfooding + Business | 19 |

---

## TUTORIAL DIFFERENTIATORS

| What This Tutorial Does | What Typical Tutorials Do |
|---|---|
| Opens with business context | Opens with "install Node.js" |
| Every technical decision traces to business impact | Technical decisions are "best practices" |
| Uses real failure analysis (bugs/033) | Shows only happy paths |
| Teaches adversarial review culture | Teaches code review as bug-finding |
| Ends with "build a business" | Ends with "you finished the tutorial" |
| Dogfoods its own product | References external tools |
| Critical thinking prompts at every chapter | "Congrats, you completed Chapter N" |

---

Does this final outline meet all your requirements? Ready to proceed with implementation?

---

## User

go write it as markdown TUTORIAL_MIMO.md

---

## Assistant (Build · MiMo V2.5 Free · 14.4s)

_Thinking:_

The user wants me to write the complete tutorial as a single markdown file called TUTORIAL_MIMO.md. Let me write the comprehensive tutorial based on the outline we developed. This is a large file, so I'll write it in one go.

**Tool: todowrite**

**Input:**
```json
{
  "todos": [
    {
      "content": "Write Part I: Product Thinking (Chapters 1-2)",
      "status": "in_progress",
      "priority": "high"
    },
    {
      "content": "Write Part II: Database Design (Chapters 3-4)",
      "status": "pending",
      "priority": "high"
    },
    {
      "content": "Write Part III: Implementation (Chapters 5-7)",
      "status": "pending",
      "priority": "high"
    },
    {
      "content": "Write Part IV: Database Migration (Chapters 8-9)",
      "status": "pending",
      "priority": "high"
    },
    {
      "content": "Write Part V: Deployment (Chapters 10-12)",
      "status": "pending",
      "priority": "high"
    },
    {
      "content": "Write Part VI: Operations (Chapters 13-14)",
      "status": "pending",
      "priority": "high"
    },
    {
      "content": "Write Part VII: Critical Thinking (Chapters 15-18)",
      "status": "pending",
      "priority": "high"
    },
    {
      "content": "Write Part VIII: Meta-Learning (Chapter 19)",
      "status": "pending",
      "priority": "high"
    },
    {
      "content": "Write Appendices and README",
      "status": "pending",
      "priority": "medium"
    }
  ]
}
```

**Output:**
```
[
  {
    "content": "Write Part I: Product Thinking (Chapters 1-2)",
    "status": "in_progress",
    "priority": "high"
  },
  {
    "content": "Write Part II: Database Design (Chapters 3-4)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Write Part III: Implementation (Chapters 5-7)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Write Part IV: Database Migration (Chapters 8-9)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Write Part V: Deployment (Chapters 10-12)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Write Part VI: Operations (Chapters 13-14)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Write Part VII: Critical Thinking (Chapters 15-18)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Write Part VIII: Meta-Learning (Chapter 19)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Write Appendices and README",
    "status": "pending",
    "priority": "medium"
  }
]
```

---

## Assistant (Build · MiMo V2.5 Free · 188.5s)

_Thinking:_

Let me write the complete tutorial. This is a large document, so I'll write it as a single file. Let me start writing it now.

**Tool: write**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/TUTORIAL_MIMO.md",
  "content": "# BCHAT TUTORIAL: Building Production-Grade SaaS\n\n## From Code to Business — A Holistic Guide for Intermediate Developers\n\n---\n\n**What this tutorial is:**\nA comprehensive guide to building production-grade software using the bchat codebase as a real-world case study. It covers product thinking, architecture, database design, implementation, deployment, operations, and critical thinking — all through the lens of a feature that replaced $246/month in third-party SaaS.\n\n**What this tutorial is not:**\nA beginner's coding course. You should know Go basics, React basics, and SQL. This tutorial teaches you to *think* like a producer, not just code like an employee.\n\n**The thread that runs through everything:**\nbugs/033 — Native Webhook Integrations — the feature that captures leads from chat conversations and sends them to external tools via webhooks. This single feature demonstrates every concept in the tutorial: product strategy, schema design, backend implementation, frontend, testing, migration, deployment, monitoring, and critical thinking.\n\n**The goal:**\nBy the end, you will have used bchat's own RAG pipeline to learn the codebase (dogfooding), and you will have a business plan for turning this knowledge into profit as an entrepreneurial producer.\n\n---\n\n# PART I: PRODUCT THINKING — Before You Write a Line\n\n---\n\n## Chapter 1: The Market Opportunity\n\n### 1.1 The $246/mo Problem\n\nSmall service businesses — plumbers, electricians, pressure washers, restoration companies — pay for three fragmented tools to handle customer conversations:\n\n- **Intercom AI ($97/mo):** AI chat agent for customer support and booking\n- **Helios AI ($100/mo):** Managed \"digital front desk\" with WhatsApp and Cal.com integration\n- **n8n Cloud ($49/mo):** Workflow automation to connect the tools\n\nStitching them together costs time, money, and reliability. The business owner ends up paying $246/month for a system that still requires manual glue work.\n\n### 1.2 What Each Competitor Actually Delivers\n\n| | Intercom AI | Helios AI | n8n |\n|---|---|---|---|\n| **Core** | AI chat agent (Fin) | Managed AI assistant | Workflow automation |\n| **Pricing** | $97/mo per seat | $100/mo per business | $49/mo per tier |\n| **Setup** | Self-service | Managed (white-glove) | Self-service (technical) |\n| **KB Config** | Built-in articles | Managed loading | N/A |\n| **SMS** | Yes | Yes (WhatsApp) | Via automation |\n| **Calendar** | Yes | Yes (Cal.com) | Via automation |\n| **Extensibility** | API + marketplace | Closed | 400+ integrations |\n| **Open Source** | No | No | Yes |\n\nThe critical insight: each tool does one thing well. None does everything. And stitching them together is the customer's problem.\n\n### 1.3 bchat's Positioning: The All-in-One Alternative\n\nbchat replaces all three with a single platform:\n\n- **AI chat agent with RAG pipeline** — replaces Intercom AI's Fin\n- **Configurable via markdown files, self-service** — replaces Helios AI's managed model\n- **Native webhook events** — replaces n8n for lead routing\n\nThe economics are compelling:\n\n| Component | Your Cost/Month |\n|-----------|----------------|\n| OpenRouter API (LLM calls) | $5-$50 |\n| Server hosting (Fly.io) | $15-$50 total for 25-50 tenants |\n| LanceDB storage | Negligible |\n| Domain + SSL | $1-$2 |\n| **Total per tenant** | **$5-$30/month** |\n\nRetail at $149-$499/month per tenant = **80-90% gross margins**.\n\n### 1.4 The Strategic Pivot in bugs/033\n\nThe webhook integrations feature (bugs/033) went through five plan iterations before shipping. Each iteration killed assumptions the previous one made.\n\n**The original plan:** Build native Twilio SMS, Google Calendar, and Outlook integrations. Estimated 2-3 months.\n\n**The pivot:** Webhook-first approach. Ship in weeks. Integrate with everything via Zapier/Make.\n\n**The key insight** (from `bugs/033/prompt.md`):\n> \"Personal AI agents (Claude Co-Work, Hermes, OpenClaw) can consume webhook events — bchat becomes the source of truth, not one tool in a chain.\"\n\n**The lesson:** Scope management is the most valuable product skill. The first plan is almost always wrong. Adversarial review catches what authorship misses.\n\n### Do This\n\n- Read `bugs/033/plan_biz.md` (first 100 lines) — identify the 3-phase integration roadmap\n- Read `bugs/033/plan_biz_review.md` — count the critical vs. major findings\n- Read `bugs/033/prompt.md` — find the exact paragraph where the pivot happens\n\n### Think About This\n\n- Why is \"scope management\" more valuable than \"coding speed\" for a product?\n- If bchat builds native SMS + Calendar, what maintenance cost does that create forever?\n- Is \"webhook-first\" a compromise or an upgrade? Under what conditions?\n\n---\n\n## Chapter 2: Architecture as a Product Decision\n\n### 2.1 Why Architecture Decisions Are Product Decisions\n\nEvery technical choice constrains or enables future features. The \"Path A vs. Path B\" debate in bugs/033 illustrates this perfectly:\n\n- **Path A (Platform Managed):** bchat owns a master Twilio account, auto-provisions subaccounts per tenant. Batteries included. But: 2-3 months of development, ongoing Twilio management, liability for tenant SMS costs.\n- **Path B (Webhook-First):** bchat emits events, tenant connects via Zapier/Make. More friction. But: ships in weeks, infinitely extensible, zero ongoing maintenance.\n\nThe decision: Path B for v1, Path A deferred. Speed to market won.\n\n### 2.2 The \"Same Transaction\" Trap\n\nPlan3 (the third iteration) claimed: \"insert event and deliver webhook in the same transaction.\"\n\nPlan3_review killed it: \"unimplementable — Store layer has no transaction API.\"\n\n**Lesson:** If a plan contradicts the data model, it's wrong. Verify assumptions against code before committing. The Store layer in bchat has no transaction API — each store method is atomic. You cannot wrap an insert and an HTTP call in the same transaction.\n\n### 2.3 The Pre-Claimed Events Pattern\n\nThe race condition:\n1. Insert event with `status='pending'`\n2. Immediate dispatch goroutine picks it up\n3. Poller also picks it up 30 seconds later\n4. Both deliver the same event → double delivery\n\nThe solution:\n1. Insert event with `status='processing'` and `claimed_at=now()`\n2. Immediate dispatch handles it\n3. Poller skips it (status is already 'processing')\n4. If immediate dispatch fails, poller reclaims after 5 minutes (`claimed_at < now() - 300`)\n\n**Lesson:** Race conditions are about state transitions. Think about who reads and writes when.\n\n### 2.4 Idempotency as a First-Class Concern\n\nNetwork errors after commit are indistinguishable from non-delivery. The webhook endpoint might receive the event, return 200, but the response gets lost. Or the endpoint might never receive it. You can't tell.\n\nThe solution: idempotency keys. `hash(tenantID, leadID, eventType, integrationID)`. If the same tuple is inserted twice, PostgreSQL's unique constraint (`23505` error) fires. The retry wrapper treats this as success — \"if the insert was committed but the ack was lost, the retry sees the duplicate and considers it done.\"\n\n**Lesson:** Design for the failure mode, not the happy path.\n\n### 2.5 The Review Culture\n\nEvery plan in bugs/033 had an adversarial review with structured checklists:\n\n| Review | Verdict | Key Finding |\n|--------|---------|-------------|\n| `plan_biz_review.md` | REWORK | 5 critical + 6 major findings |\n| `plan2_review.md` | Approved with nits | Context cancellation fix needed |\n| `plan3_review.md` | REWORK | \"Same transaction\" outbox claim unimplementable |\n| `plan4_review.md` | APPROVED | 4 amendments (MySQL stubs, pre-claimed events, max attempts, SMS deferred) |\n| `code_review.md` | REWORK | Idempotency key argument swap, poller context cancelled |\n\n**Lesson:** Code review is not about finding bugs. It's about finding wrong assumptions. The review culture in bugs/033 caught architectural flaws that tests never would have found.\n\n### Do This\n\n- Compare `plan3_review.md` (REWORK) with `plan4_review.md` (APPROVED)\n- Identify the specific language that changed between plans\n- Trace how the \"pre-claimed events\" pattern is implemented in `integrations.go`\n\n### Think About This\n\n- Why did plan3 fail but plan4 succeed? What was fundamentally different?\n- If you were the reviewer, what would you have caught in plan3?\n- How does \"adversarial review\" differ from \"code review\"? Which catches more?\n\n---\n\n# PART II: DATABASE DESIGN — Schema as Contract\n\n---\n\n## Chapter 3: Multi-Tenant Schema Design\n\n### 3.1 The Two New Tables (0.31 migrations)\n\nThe webhook feature introduces two new tables:\n\n```sql\n-- agent_integrations: \"What webhooks does this tenant have?\"\nCREATE TABLE agent_integrations (\n    id INTEGER PRIMARY KEY AUTOINCREMENT,\n    tenant_id INTEGER NOT NULL,\n    integration_type TEXT NOT NULL DEFAULT 'webhook',\n    label TEXT NOT NULL,\n    config TEXT NOT NULL DEFAULT '{}',\n    is_active INTEGER NOT NULL DEFAULT 1,\n    created_at TEXT NOT NULL DEFAULT (datetime('now')),\n    updated_at TEXT NOT NULL DEFAULT (datetime('now')),\n    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE\n);\nCREATE UNIQUE INDEX idx_integration_tenant_type\n    ON agent_integrations(tenant_id, integration_type);\n\n-- agent_events: \"What needs to be delivered?\"\nCREATE TABLE agent_events (\n    id INTEGER PRIMARY KEY AUTOINCREMENT,\n    tenant_id INTEGER NOT NULL,\n    integration_id INTEGER NOT NULL,\n    event_type TEXT NOT NULL DEFAULT 'lead.captured',\n    payload TEXT NOT NULL DEFAULT '{}',\n    status TEXT NOT NULL DEFAULT 'processing',\n    claimed_at TEXT,\n    attempts INTEGER NOT NULL DEFAULT 0,\n    last_error TEXT,\n    idempotency_key TEXT NOT NULL,\n    created_at TEXT NOT NULL DEFAULT (datetime('now')),\n    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE,\n    FOREIGN KEY (integration_id) REFERENCES agent_integrations(id) ON DELETE CASCADE\n);\nCREATE UNIQUE INDEX idx_events_idempotency\n    ON agent_events(idempotency_key);\n```\n\n### 3.2 Design Decisions Explained\n\n| Decision | Choice | Why |\n|----------|--------|-----|\n| Integration uniqueness | `UNIQUE(tenant_id, integration_type)` | One webhook per type per tenant — simple, enforced at DB level |\n| Event status on insert | `status = 'processing'` | Pre-claiming eliminates race condition with immediate dispatch |\n| Idempotency key | `hash(tenantID, leadID, eventType, integrationID)` | Deterministic deduplication — same input always produces same key |\n| Cascade deletes | `ON DELETE CASCADE` | No orphaned data when tenant is deleted |\n| Lease expiry | `claimed_at < now() - 300` | 5-minute recovery for crashed processes |\n\n### 3.3 SQLite vs. PostgreSQL: Same Schema, Different SQL\n\nThe same logical schema requires different SQL dialects:\n\n| Feature | SQLite | PostgreSQL |\n|---------|--------|------------|\n| Auto-increment | `INTEGER PRIMARY KEY AUTOINCREMENT` | `SERIAL` or `GENERATED ALWAYS AS IDENTITY` |\n| Timestamps | `datetime('now')` | `NOW()` |\n| Booleans | `INTEGER` (0/1) | `BOOLEAN` |\n| JSON | `TEXT` | `JSONB` |\n| Upsert | `INSERT OR IGNORE` | `ON CONFLICT DO NOTHING` |\n| Row locking | Table-level | `FOR UPDATE SKIP LOCKED` |\n\nThe migration files exist in parallel directories (`store/migration/sqlite/0.31/` and `store/migration/postgres/0.31/`) with identical structure but dialect-specific SQL.\n\n### 3.4 State Machine Thinking\n\nEvent states form a simple state machine:\n\n```\nprocessing → delivered (success)\nprocessing → dead (exhausted retries)\n```\n\nThe `processing` state is entered on insert (pre-claimed). The `delivered` state is entered on successful webhook delivery. The `dead` state is entered after exhausting retry attempts.\n\nEvery \"stuck\" state needs a recovery path. The lease expiry (`claimed_at < now() - 300`) is the recovery mechanism for events stuck in `processing` — if a dispatcher crashes mid-delivery, the event is reclaimable after 5 minutes.\n\n### Do This\n\n- Write the SQL to find all events stuck in `processing` for more than 5 minutes\n- Write the SQL to count events by status for a specific tenant\n- Design a schema for a new feature: \"event delivery log\" (what was sent, when, response code)\n\n### Think About This\n\n- `ON DELETE CASCADE` is convenient but dangerous — what's the blast radius of deleting a tenant with 10,000 pending events?\n- Why not use PostgreSQL's `LISTEN/NOTIFY` instead of polling? What would change?\n- How would you handle a schema change that affects 1M+ rows in production?\n\n---\n\n## Chapter 4: The Store Layer Abstraction\n\n### 4.1 The Driver Interface\n\nThe `store/driver.go` file defines 60+ methods covering all entity CRUD operations. Adding a new entity means:\n\n1. Define types in `store/agent.go` (Go structs with JSON tags)\n2. Add interface methods to `store/driver.go` (the contract)\n3. Implement in `store/db/sqlite/agent.go` (SQLite driver)\n4. Implement in `store/db/postgres/agent.go` (PostgreSQL driver)\n\nThe interface is the contract. Implementations can diverge — SQLite uses `INSERT OR IGNORE`, PostgreSQL uses `ON CONFLICT DO NOTHING` — but the interface stays the same.\n\n### 4.2 SQLite Implementation\n\nUses `modernc.org/sqlite` — a pure Go SQLite implementation with no CGO dependency. Key characteristics:\n\n- `INSERT OR IGNORE` for idempotent inserts\n- Busy timeout: 10 seconds for write contention\n- WAL journal mode for concurrent reads\n- Single-writer assumption (no `FOR UPDATE SKIP LOCKED`)\n\n### 4.3 PostgreSQL Implementation\n\nUses `pgx/v5` — a high-performance pure Go PostgreSQL driver. Key characteristics:\n\n- `ON CONFLICT (idempotency_key) DO NOTHING` — native conflict handling\n- `FOR UPDATE SKIP LOCKED` for claim operations — non-blocking claims\n- Connection pool: 10 max open, 5 max idle, 5min max lifetime\n- `default_query_exec_mode=simple_protocol` for Neon pgbouncer compatibility\n\n### 4.4 PostgreSQL Resilience\n\nNeon is a serverless PostgreSQL provider. Connections can drop mid-query. The resilience layer (`store/db/postgres/resilience.go`) handles:\n\n- Transient error codes: `57P01`, `57P02`, `57P03`, `57P04`\n- Retry with exponential backoff + jitter\n- Circuit breaker pattern for connection failures\n\n### Do This\n\n- Compare the `ClaimEvents` implementation in SQLite vs. PostgreSQL\n- Identify the 3 lines that differ between them\n- Explain why SQLite can't use `FOR UPDATE SKIP LOCKED`\n\n### Think About This\n\n- Why use `pgx/v5` instead of `database/sql` with `lib/pq`? What does pgx give you?\n- If you had to add MySQL support, what would break?\n- How does the driver abstraction hide differences but not eliminate them?\n\n---\n\n# PART III: IMPLEMENTATION — Building the Feature\n\n---\n\n## Chapter 5: Backend — The Webhook Dispatcher\n\n### 5.1 Conceptual Flow\n\nWhen a visitor shares their contact information during a chat conversation:\n\n```\nChat conversation\n    ↓\nLead extraction (name + email/phone)\n    ↓\nInsert into agent_events (status='processing', pre-claimed)\n    ↓\nImmediate dispatch attempt (fire-and-forget goroutine)\n    ↓\n    ├── Success: status → 'delivered'\n    └── Failure: status stays 'processing'\n                    ↓\n            Poller claims it (every 30s)\n                    ↓\n            Retry with backoff\n                    ↓\n            ├── Success: status → 'delivered'\n            └── Exhausted: status → 'dead'\n```\n\nThe webhook dispatcher (`integrations.go`, 532 lines) handles:\n\n1. **URL Validation (SSRF Protection):** Parse URL → resolve DNS → check IP is not internal\n2. **Payload Construction:** Build JSON with timestamp and event data\n3. **HMAC Signing:** Sign payload with tenant's secret key, add `X-Bchat-Signature` header\n4. **HTTP POST:** Send with 10-second timeout, 3 retry attempts\n5. **Status Tracking:** Update event status based on response\n\n### 5.2 SSRF Protection: Security by Default\n\nWebhook URLs are user-provided. An attacker could provide `http://169.254.169.254/latest/meta-data/` (AWS metadata endpoint) or `http://localhost:5432/` (local database). This is a Server-Side Request Forgery (SSRF) attack.\n\nThree-layer defense:\n1. **URL Parsing:** Reject malformed URLs\n2. **DNS Resolution:** Resolve hostname to IP address\n3. **IP Validation:** Reject private IPs (10.x, 172.16-31.x, 192.168.x, 169.254.x)\n\nThe SSRF helpers were copied from `plugin/webhook/webhook.go` (unexported there) because the two callers diverge — protobuf vs. raw JSON. Deliberate duplication is better than no security.\n\n### 5.3 HMAC Signing: Proving Identity\n\nEach webhook includes an `X-Bchat-Signature` header:\n\n```\nSignature: HMAC-SHA256(secret, payload + timestamp)\n```\n\nThe tenant provides the secret. bchat signs. The recipient verifies. This proves the payload came from bchat and wasn't tampered with in transit.\n\n### 5.4 The Poller: Background Recovery\n\nA goroutine runs every 30 seconds:\n\n1. Claim up to 10 events where `status='processing'` and `claimed_at < now() - 300`\n2. Dispatch each webhook\n3. Update status based on response\n\nTradeoff: awake-only retries. When Fly.io scales to zero, the poller stops. Events wait in the database until the machine wakes up. This is acceptable for v1 — the immediate dispatch handles the live path, the poller is catch-up/retry.\n\n### Do This\n\n- Trace the flow from lead capture to webhook delivery through the code\n- Find the exact line where HMAC signature is computed\n- Find the exact line where SSRF validation blocks an internal IP\n\n### Think About This\n\n- `http.Client{Timeout: 10 * time.Second}` — what happens if the webhook endpoint is slow but not down?\n- The poller runs every 30 seconds. What's the maximum latency for webhook delivery?\n- How does `kill_timeout = \"30s\"` in fly.toml interact with in-flight deliveries?\n\n---\n\n## Chapter 6: Frontend — Admin UI for Integrations\n\n### 6.1 Conceptual Flow\n\n```\nIntegrationsSection.tsx (React component)\n    ↓\nMobX store (agentAdmin.ts)\n    ↓\nHTTP request to /api/v1/agent/:slug/integrations\n    ↓\nHandler (handlers.go) → Service (service.go) → Store (agent.go)\n    ↓\nResponse flows back up\n```\n\n### 6.2 The Component Structure\n\n`IntegrationsSection.tsx` (296 lines) provides:\n\n- **List view:** Shows existing integrations with status chips (active/inactive)\n- **Create/edit modal:** URL, secret, label fields with validation\n- **Test button:** Sends a test event and shows the result\n- **Delete confirmation:** Prevents accidental deletion\n\nThe component uses Joy UI (Button, Card, Chip, Modal, Switch, Select) with Tailwind CSS for layout — matching the existing codebase pattern.\n\n### 6.3 Pattern Recognition\n\nCompare `IntegrationsSection.tsx` with other `AgentAdminSections/` components. The pattern repeats:\n\n1. MobX store holds state and CRUD methods\n2. Component renders list from store\n3. Modal for create/edit\n4. API calls via axios\n5. `runInAction` for async state updates\n\nOnce you recognize this pattern, new admin features are copy-paste-refactor.\n\n### 6.4 The PII Masking Decision\n\n`HandleListEvents` returns a `safeEvent` struct — no `payload` field. The payload contains name, email, phone (PII). The admin UI shows event status and type, not content.\n\n**Lesson:** API responses are security boundaries. What you expose in a list endpoint can leak PII.\n\n### Do This\n\n- Compare `IntegrationsSection.tsx` with another admin section\n- Identify the 3 patterns that repeat across all admin sections\n- Design the UI for a new integration type (e.g., Slack)\n\n### Think About This\n\n- Why MobX instead of Redux? What does `makeAutoObservable` give you?\n- How would you add real-time event status updates (WebSocket vs. polling)?\n- If you were building this for Helios AI's \"zero-config\" model, what would the UI look like?\n\n---\n\n## Chapter 7: Testing — What Catches What\n\n### 7.1 The Bugs That Slipped Through\n\nThe code review (`code_review.md`) found two bugs that tests missed:\n\n**Bug 1: Idempotency Key Argument Swap**\nThe idempotency key was computed as `hash(tenantID, leadID, eventType, integrationID)` but the arguments were swapped in one call site. Result: all events deduplicated to a single integration — if you had two webhook integrations, only one received events.\n\n**Bug 2: Poller Context Cancelled Before Delivery**\nThe poller dispatched webhooks inside a `context.WithTimeout` but the timeout was shorter than the delivery time. The context cancelled before the HTTP response arrived, causing the delivery to appear failed even when it succeeded.\n\n### 7.2 What Was Tested\n\n- HMAC signing correctness\n- SSRF protection (internal IP blocking)\n- Idempotency key determinism (same inputs → same key)\n- Integration CRUD operations\n\n### 7.3 The Testing Pyramid\n\n```\n        ┌─────────┐\n        │  E2E    │  ← few, slow, high confidence\n        ├─────────┤\n        │Integration│ ← moderate, test boundaries\n        ├─────────┤\n        │  Unit   │  ← many, fast, test logic\n        └─────────┘\n```\n\n### 7.4 What Tests Would Catch Bugs 1 & 2\n\n**Bug 1:** An integration test that creates 2 integrations, dispatches to both, and verifies both receive events. The existing test only created 1 integration.\n\n**Bug 2:** A unit test that verifies poller dispatch completes before context cancellation. The existing test mocked the HTTP call but didn't test the context lifecycle.\n\n### Do This\n\n- Write a test that would catch the argument-order bug (Bug 1)\n- Write a test that would catch the context-cancellation bug (Bug 2)\n- Run `go test ./server/router/api/v1/agent/ -run TestIntegration` and analyze the output\n\n### Think About This\n\n- Code review found bugs that tests missed. What's the relationship between review and testing?\n- How do you write tests that catch argument-order bugs without testing implementation details?\n- What's the cost of a bug that reaches production vs. a bug caught in review?\n\n---\n\n# PART IV: DATABASE MIGRATION — SQLite to PostgreSQL\n\n---\n\n## Chapter 8: Migration Strategy\n\n### 8.1 The Two-Database Reality\n\n- **SQLite:** Development (zero-config, single file, fast)\n- **PostgreSQL:** Production (Neon serverless, concurrent access, JSONB)\n\nSame migration files, different SQL dialects. The migration system handles both.\n\n### 8.2 Migration File Structure\n\n```\nstore/migration/\n├── sqlite/\n│   ├── 0.31/\n│   │   ├── 00__agent_integrations.sql\n│   │   └── 01__agent_events.sql\n│   └── LATEST.sql (cumulative)\n├── postgres/\n│   ├── 0.31/\n│   │   ├── 00__agent_integrations.sql\n│   │   └── 01__agent_events.sql\n│   └── LATEST.sql (cumulative)\n```\n\nNaming convention: `NN__snake_case_description.sql`\n\n### 8.3 Dialect Differences\n\n| Feature | SQLite | PostgreSQL |\n|---------|--------|------------|\n| Auto-increment | `INTEGER PRIMARY KEY AUTOINCREMENT` | `SERIAL` |\n| Timestamps | `datetime('now')` | `NOW()` |\n| Booleans | `INTEGER` (0/1) | `BOOLEAN` |\n| JSON | `TEXT` | `JSONB` |\n| Upsert | `INSERT OR IGNORE` | `ON CONFLICT DO NOTHING` |\n| Row locking | Table-level | `FOR UPDATE SKIP LOCKED` |\n\n### 8.4 LATEST.sql: The Cumulative Schema\n\n`LATEST.sql` (1067 lines) contains the complete schema for fresh installs. It must be regenerated after every migration change. `task validate:migrations` checks that LATEST.sql matches the incremental migrations.\n\n### Do This\n\n- Write the SQLite migration for a new table: `agent_webhook_logs`\n- Write the PostgreSQL version of the same migration\n- Update LATEST.sql for both databases\n- Run `task validate:migrations` to verify\n\n### Think About This\n\n- Why use `ON DELETE CASCADE` for tenant-scoped data? What's the alternative?\n- What happens if a migration fails midway? (SQLite: no transaction rollback; PostgreSQL: transaction rollback)\n- How would you handle a migration that takes 10 minutes on a large table?\n\n---\n\n## Chapter 9: Running Migrations in Production\n\n### 9.1 Programmatic Migration\n\n`store/migrator.go` runs on server startup:\n\n```go\nif err := storeInstance.Migrate(ctx); err != nil {\n    return fmt.Errorf(\"failed to migrate: %w\", err)\n}\n```\n\nIt checks the `migration_history` table, applies only new migrations, and records what was applied.\n\n### 9.2 Neon Connection Configuration\n\n- `DATABASE_URL` with SSL mode required\n- `pgx/v5` auto-appends `default_query_exec_mode=simple_protocol`\n- Connection pool managed by Neon (not application)\n- Auto-scaling: Neon suspends when idle, resumes on connect\n\n### 9.3 Validation Commands\n\n```bash\ntask validate:migrations   # LATEST.sql sync check\ntask validate:schema       # Schema + CRUD validation\n```\n\n### Do This\n\n- Run `task validate:schema` on the current codebase\n- Create a test migration, run it, verify it appears in `migration_history`\n\n### Think About This\n\n- Neon suspends when idle. What happens to in-flight queries during suspension?\n- How does `simple_protocol` affect query performance vs. extended protocol?\n- What's your rollback strategy if a migration corrupts data?\n\n---\n\n# PART V: DEPLOYMENT — Getting to Production\n\n---\n\n## Chapter 10: Docker Containerization\n\n### 10.1 The Three-Stage Dockerfile\n\n```dockerfile\n# Stage 1: Frontend build (Node.js)\nFROM node:20-slim AS frontend\nWORKDIR /app\nCOPY web/package*.json ./\nRUN npm ci\nCOPY web/ ./\nRUN npm run build\n\n# Stage 2: Backend build (Go)\nFROM golang:1.25 AS backend\nWORKDIR /go/src/app\nCOPY go.mod go.sum ./\nRUN go mod download\nCOPY . .\nRUN CGO_ENABLED=1 go build -o /go/bin/memos\n\n# Stage 3: Runtime (Ubuntu)\nFROM ubuntu:24.04\nRUN apt-get update && apt-get install -y ca-certificates supercronic\nCOPY --from=backend /go/bin/memos /usr/local/bin/memos\nCOPY --from=frontend /app/dist /app/dist\nCOPY scripts/entrypoint.sh /entrypoint.sh\nRUN chmod +x /entrypoint.sh\n```\n\nEach stage is isolated. Build dependencies don't reach the runtime image.\n\n### 10.2 Security in the Container\n\n- Non-root user (`memos`) via `gosu`\n- Read-only filesystem (except data directory)\n- `_FILE` suffix support for Docker/K8s secrets\n- `entrypoint.sh` processes secrets before starting\n\n### 10.3 Supercronic: Container Cron\n\nReplaces `crond` (not available in minimal containers). External process — survives Go panics. `build/crontab` defines scheduled tasks.\n\nTradeoff: retries only run while the machine is awake. Scale-to-zero limitation documented honestly.\n\n### Do This\n\n- Build the Docker image locally: `docker build -f Dockerfile.pg.fly .`\n- Run the container and verify the health check\n\n### Think About This\n\n- Why Ubuntu instead of Alpine? (glibc vs. musl, LanceDB compatibility)\n- What's the security implication of `chmod +x /entrypoint.sh`?\n- How does `gosu` differ from `su`? Why is it preferred in containers?\n\n---\n\n## Chapter 11: Fly.io Deployment\n\n### 11.1 Configuration Anatomy\n\n```toml\napp = \"bchat-pg\"\nprimary_region = \"sjc\"\n\n[build]\n  dockerfile = \"Dockerfile.pg.fly\"\n\n[env]\n  DRIVER = \"postgres\"\n  SERVER_MODE = \"prod\"\n\n[http_service]\n  internal_port = 5230\n  force_https = true\n  auto_stop_machines = \"stop\"\n  auto_start_machines = true\n  kill_signal = \"SIGTERM\"\n  kill_timeout = \"30s\"\n\n[[vm]]\n  memory = \"1024mb\"\n  cpu_kind = \"shared\"\n  cpus = 1\n```\n\n### 11.2 Secrets Management\n\n```bash\nfly secrets set DATABASE_URL=\"postgresql://...\"\nfly secrets set OPENROUTER_API_KEY=\"sk-or-...\"\nfly secrets set ENCRYPTION_MASTER_KEY=\"...\"\nfly secrets set CRON_TOKEN=\"...\"\n```\n\nSecrets are encrypted at rest, injected as env vars. No secrets in Docker images or git history.\n\n### 11.3 The Deployment Flow\n\n```bash\nfly auth login\nfly launch          # first time\nfly secrets set ...  # set secrets\nfly deploy          # build + deploy\nfly status          # check health\nfly logs            # stream logs\n```\n\n### 11.4 Neon Integration\n\n- `DATABASE_URL` from Neon dashboard\n- SSL mode required\n- Connection pooling by Neon\n- Auto-suspend/resume on idle/activity\n\n### Do This\n\n- Deploy to Fly.io with a Neon database\n- Verify the health check at `/healthz`\n- Scale down to 0 machines, then hit the API and verify it wakes up\n\n### Think About This\n\n- `kill_timeout = \"30s\"` — what happens during those 30 seconds?\n- How does `auto_stop_machines` interact with the poller goroutine?\n- What's the cold-start time for a stopped Fly.io machine?\n\n---\n\n## Chapter 12: Pre-Deployment Validation\n\n### 12.1 The Validation Chain\n\n```bash\nscripts/validate-env-chain.sh      # env vars\nscripts/validate-db-migrations.sh   # migration integrity\ntask validate:schema                # schema + CRUD\ntask build:rag                      # build with RAG\nfly deploy                          # deploy\n```\n\n### 12.2 Why Validate Before Deploy\n\n- Failed deploy = downtime + rollback + investigation\n- Blocked deploy = delayed but safe\n- Pre-deploy catches 80% of issues before they reach production\n\n### Do This\n\n- Run the full validation chain on the current codebase\n- Introduce a deliberate error (e.g., missing env var) and see how validation catches it\n\n### Think About This\n\n- What's the cost of a failed deploy vs. a blocked deploy?\n- How would you add a smoke test that runs after deploy?\n\n---\n\n# PART VI: PRODUCTION OPERATIONS — Keeping It Running\n\n---\n\n## Chapter 13: Monitoring & Observability\n\n### 13.1 Health Check\n\n`/healthz` endpoint: checks database + LanceDB connectivity. Returns 200 OK or 503 Service Unavailable. Used by Fly.io for routing and restart decisions.\n\n### 13.2 Logs to Watch\n\n```bash\n\"webhook delivered\"           # success\n\"webhook delivery failed\"     # failure\n\"event claimed\"               # poller activity\n\"event moved to dead\"         # exhausted retries\n```\n\n### 13.3 Metrics to Track\n\n| Metric | Target | Action if breached |\n|--------|--------|-------------------|\n| Webhook delivery success rate | > 99% | Investigate endpoints |\n| Average webhook latency | < 2s | Check network/endpoint |\n| Dead letter rate | < 0.1% | Review tenant configs |\n| Database connections | < 80% | Scale pool |\n| LLM cost per tenant | < budget | Review usage |\n\n### Do This\n\n- Set up log monitoring for webhook delivery events\n- Write an alert rule for dead letter rate > 1%\n\n### Think About This\n\n- How does Fly.io's `auto_stop` interact with monitoring? (Metrics stop when machine stops)\n- What would you monitor if bchat had 10x more tenants?\n\n---\n\n## Chapter 14: Incident Response\n\n### 14.1 Severity Levels\n\n- **P0:** All webhook delivery down (all tenants affected)\n- **P1:** Single tenant webhook failing\n- **P2:** Event polling delayed (events still delivered, just slow)\n- **P3:** Cosmetic issue in admin UI\n\n### 14.2 Runbook: Events Stuck in \"processing\"\n\n**Diagnosis:**\n1. Check poller logs: `fly logs | grep poller`\n2. Check event age: `SELECT id, created_at, claimed_at FROM agent_events WHERE status='processing'`\n3. Check lease expiry: `claimed_at < now() - interval '5 minutes'`\n\n**Mitigation:**\n1. Trigger manual cron: `POST /system/trigger-cron`\n2. If poller is stuck: `fly apps restart bchat-pg`\n\n**Resolution:**\n1. Check for webhook endpoint issues (tenant-side)\n2. Check for database connection issues\n3. Review recent deployments for regressions\n\n**Prevention:**\n1. Add monitoring for poller health\n2. Add dead letter alerts\n3. Document webhook endpoint requirements for tenants\n\n### 14.3 Post-Mortem Template\n\n- **Timeline:** what happened, when, how long\n- **Root cause:** why it happened\n- **Impact:** who was affected, for how long\n- **Action items:** what we'll change (with owners and deadlines)\n\n### Do This\n\n- Write a runbook for a new failure mode: \"webhook endpoint returns 429 (rate limited)\"\n- Identify the 3 most likely failure modes for the webhook system\n\n### Think About This\n\n- A tenant reports \"I'm not getting webhook deliveries\" — walk through the diagnostic steps\n- What information do you need from the tenant? What can you get from the database?\n- How do you balance \"fix it now\" with \"understand why it happened\"?\n\n---\n\n# PART VII: CRITICAL THINKING — The Meta-Skills\n\n---\n\n## Chapter 15: Reading Code You Didn't Write\n\n### 15.1 Navigation Strategy\n\n- Start at the entry point (`bin/memos/main.go`)\n- Follow the request flow: HTTP → Handler → Service → Store\n- Read the tests (they document expected behavior)\n- Check the git log (commit messages explain \"why\")\n\n### 15.2 Pattern Recognition\n\n- **Handler:** extract context → validate input → call service → return response\n- **Store:** interface definition → driver implementation → migration\n- **Frontend:** MobX store → API call → component render\n\n### 15.3 When Code Confuses You\n\n- Don't read sequentially — follow the call graph\n- Use `grep` to find where a function is called\n- Read the smallest possible unit that makes sense in isolation\n\n### Do This\n\n- Trace the full request flow for `POST /api/v1/agent/:slug/integrations`\n- Identify every function call from HTTP handler to database insert\n- Draw the call graph on paper\n\n### Think About This\n\n- Which layer of the code is hardest to understand? Why?\n- If you had to explain the webhook system to a non-technical person, what would you say?\n\n---\n\n## Chapter 16: Evaluating Technical Decisions\n\n### 16.1 The Tradeoff Framework\n\n| Dimension | Option A | Option B |\n|-----------|----------|----------|\n| Development time | ? | ? |\n| Operational complexity | ? | ? |\n| Cost at scale | ? | ? |\n| Failure modes | ? | ? |\n| Reversibility | ? | ? |\n\n### 16.2 Case Study: Webhooks vs. Native Integrations\n\n| Dimension | Webhooks (Path B) | Native (Path A) |\n|-----------|-------------------|-----------------|\n| Dev time | Weeks | Months |\n| Complexity | Lower | Higher |\n| Cost at scale | Same | Higher maintenance |\n| Failure modes | External only | Internal + external |\n| Reversibility | High | Low |\n\n### 16.3 The Second System Effect\n\nbugs/033 went through 5 plan iterations. Each caught assumptions the previous made. Your first plan is your worst plan. Build in adversarial review.\n\n### Do This\n\n- Evaluate a technical decision you're using the framework\n- Write a one-paragraph justification for your choice\n\n### Think About This\n\n- Is \"webhook-first\" a compromise or an upgrade? Under what conditions?\n- When should you build native vs. emit events?\n- How do you know when a tradeoff is acceptable vs. a mistake?\n\n---\n\n## Chapter 17: Writing Code That Survives\n\n### 17.1 Defensive Programming\n\n- **Idempotency keys:** network errors are indistinguishable from non-delivery\n- **Lease expiry:** crashed processes leave stale state\n- **SSRF protection:** user input is hostile until validated\n\n### 17.2 Operational Visibility\n\n- Structured logging with context (tenant_id, event_id, integration_id)\n- Event status tracking (processing → delivered/dead)\n- Audit trail (created_at, claimed_at, attempts)\n\n### 17.3 Graceful Degradation\n\n- If webhook delivery fails, log and continue\n- If poller is down, events wait (they're in the database)\n- If database is down, return 503 (fail fast, don't queue)\n\n### Do This\n\n- Add structured logging to a function that currently uses `fmt.Println`\n- Design an audit trail for a new feature\n\n### Think About This\n\n- What's the difference between \"defensive programming\" and \"paranoid programming\"?\n- How much logging is too much? When does logging become noise?\n- What's the cost of graceful degradation vs. fail-fast?\n\n---\n\n## Chapter 18: When to Stop and Ship\n\n### 18.1 The MVP Mindset\n\nv1: Webhooks only. SMS deferred to v1.1. Calendar deferred to future. \"Batteries included\" with zero tenant friction. Perfect is the enemy of shipped.\n\n### 18.2 Known Limitations (Documented Honestly)\n\n- Awake-only retries (scale-to-zero limitation)\n- SQLite write lock (acceptable at v1 scale)\n- No native SMS/Calendar (webhook bridge required)\n\n### 18.3 The Review Culture\n\nEvery plan had an adversarial review. Reviews used structured checklists (critical, major, minor findings). Code review is not about finding bugs. It's about finding wrong assumptions.\n\n### Do This\n\n- Read `plan4_review.md` — identify the 4 amendments (A-D)\n- For each amendment, explain why it was necessary\n\n### Think About This\n\n- How do you know when a feature is \"good enough\" to ship?\n- What's the cost of shipping too early vs. too late?\n- How do you balance \"known limitations\" with \"marketing claims\"?\n\n---\n\n# PART VIII: META-LEARNING — Using the Product to Learn the Product\n\n---\n\n## Chapter 19: Dogfooding RAG: From Learner to Business Owner\n\n### 19.1 The Business Case for Dogfooding\n\nYou've spent 18 chapters learning bchat from the outside in. Now flip the script.\n\n**The \"Eat Your Own Dog Food\" Principle:**\nIf the product can't teach you about itself, it can't teach anyone anything. Build a tenant that uses KB.MD/POLICY.MD/SCRIPT.MD to teach developers about the bchat codebase. If the RAG pipeline can't answer \"How do I add a new API endpoint?\" reliably, it can't answer \"How do I schedule a water extraction appointment?\" reliably either.\n\n**What Dogfooding Reveals:**\n- RAG quality gaps — vague answers mean your customers get vague answers\n- Chunking failures — bad splits mean bad retrieval\n- Retrieval weaknesses — missed chunks mean missed FAQs\n- Prompt engineering gaps — hallucination means grounded answers fail\n\n**The Meta-Lesson:**\nWriting a KB.MD for a codebase forces you to think like a product manager: \"What questions will developers ask? What answers do they need? What's the right level of detail?\" This is exactly the thinking your customers need to do for their businesses.\n\n### Think About This\n\n- If you were onboarding a new developer, what are the 10 questions they'd ask first?\n- How does writing a KB.MD for a codebase differ from writing one for a water extraction company?\n\n---\n\n### 19.2 Building the Codebase Assistant\n\n**The Setup:**\n1. Create a new tenant: `slug = \"bchat-dev\"`, `company_name = \"bchat Development Team\"`\n2. Upload three files:\n   - `KB_CODEBASE.MD` — architecture, patterns, gotchas, file reference\n   - `POLICY_CODEBASE.MD` — identity, intents, rules for a developer assistant\n   - `SCRIPT_CODEBASE.MD` — conversation flow for code questions\n\n**The KB.MD Structure:**\nUse `<!-- @section: name, type: category -->` for each topic. One section per concept. Each section targets ~500 tokens for optimal chunking. Example sections: tenant isolation, RAG pipeline, build commands, gotchas, key files, adding features.\n\n**The POLICY.MD Structure:**\n- `<!-- @identity -->`: \"You are a codebase learning assistant\"\n- `<!-- @intent -->`: architecture_question, how_to_question, debugging_question, deployment_question\n- `<!-- @rule -->`: Always cite files, never guess, explain the \"why\"\n\n**The SCRIPT.MD Structure:**\nConversation flow: understand request → locate code → explain architecture → suggest changes\n\n### Do This\n\n- Create the tenant, write the three files, upload and reindex\n- Ask: \"How do I add a new API endpoint?\"\n- Observe what RAG retrieves and how the agent responds\n\n### Think About This\n\n- What happens if you ask a question not covered in the KB? Does the agent admit ignorance or hallucinate?\n- How does the quality of the KB.MD writing affect the quality of the answers?\n\n---\n\n### 19.3 Testing RAG Against the Codebase\n\n**The Test Matrix:**\n\n| Category | Question | What to Check |\n|----------|----------|---------------|\n| Architecture | \"How does multi-tenancy work?\" | JWT claims, context mechanism |\n| How-To | \"How do I add a new store method?\" | 5-step backend pattern |\n| Debugging | \"Why is my migration failing?\" | Filename convention |\n| Reference | \"What env vars do I need?\" | Complete list |\n| Gotchas | \"What table naming convention?\" | Plural names |\n| Security | \"How do I check permissions?\" | CheckUserPermission |\n| RAG | \"How does hybrid search work?\" | Vector 70% + BM25 30% |\n| Deployment | \"How do I deploy to Fly.io?\" | fly.toml, secrets |\n\n**Scoring:**\n- 5: Perfect answer with file reference + gotchas\n- 4: Correct with minor omission\n- 3: Partial — right direction but incomplete\n- 2: Vague — mostly correct but missing details\n- 1: Wrong or hallucination\n\n### Do This\n\n- Run the 8 test questions\n- Score each response\n- Identify the weakest category\n- Rewrite the KB.MD section for that category and re-test\n\n### Think About This\n\n- Which questions scored lowest? Why?\n- Did RAG retrieve the right chunks but the LLM gave a bad answer? (Prompt problem)\n- Did RAG retrieve the wrong chunks? (Retrieval problem)\n\n---\n\n### 19.4 Using RAG to Implement bugs/033\n\n**The Scenario:**\nYou're a new developer. Your first task: implement webhook integrations. You have the codebase assistant. How do you use it?\n\n**Phase 1: Understanding the Feature**\n- \"What is the webhook integrations feature?\"\n- \"What database tables does it use?\"\n- \"How does the event outbox pattern work?\"\n\n**Phase 2: Understanding the Code**\n- \"Where is the webhook dispatcher implemented?\"\n- \"How does SSRF protection work?\"\n- \"What's the difference between SQLite and Postgres implementations?\"\n\n**Phase 3: Implementing the Feature**\n- \"Show me the pattern for adding a new store method\"\n- \"What's the migration file naming convention?\"\n- \"How do I register a new route?\"\n\n**Phase 4: Testing the Feature**\n- \"How do I write tests for the store layer?\"\n- \"What does the code review checklist look like?\"\n- \"How do I validate my migration?\"\n\n### Do This\n\n- Use the codebase assistant for 5 questions about implementing bugs/033\n- For each, note: (a) what it retrieved, (b) whether the answer was useful, (c) what's missing\n\n### Think About This\n\n- Could the codebase assistant replace AGENTS.md? Why or why not?\n- How does its value change as the codebase grows?\n\n---\n\n### 19.5 When RAG Fails: Learning the Limitations\n\n**Failure Mode 1: The Documentation Gap**\n- Question about undocumented code → \"This is not covered\"\n- RAG can only retrieve what's indexed\n\n**Failure Mode 2: The Chunking Boundary Problem**\n- Table split across two chunks → partial answer\n- Structure KB.MD so each chunk is self-contained\n\n**Failure Mode 3: The Semantic Mismatch**\n- Same question matches multiple chunks → hybrid search helps but isn't perfect\n\n**Failure Mode 4: The Outdated Knowledge**\n- Stale docs indexed → stale answers\n- Reindex after updates\n\n**Failure Mode 5: The Confidence Problem**\n- LLM phrasing may understate certainty\n- RAG gives chunks, not confidence scores\n\n### Do This\n\n- Find 3 questions that produce bad answers\n- Identify the failure mode for each\n- Fix the underlying issue and re-test\n\n### Think About This\n\n- How do you know when the assistant is wrong vs. when you're asking the wrong question?\n- What's the cost of acting on a wrong answer?\n\n---\n\n### 19.6 Creative Applications: From Code to Commerce\n\nThis is where the tutorial shifts from \"learning\" to \"building a business.\"\n\n**The Entrepreneurial Mindset:**\nYou now understand how bchat works — architecture, database, deployment, operations. You've dogfooded the RAG pipeline. You've used the codebase assistant to learn the code. The next step is not to get a job. The next step is to build a business.\n\n**The Five Business Models:**\n\n**Model 1: The Service Business (Lowest Risk)**\nYou are the \"digital front desk\" provider. Pick a vertical: plumbers, electricians, pressure washing. Write KB.MD/POLICY.MD for each client. Charge $149-$299 setup + $149-$299/month. Your cost: $5-$30/tenant/month. Margin: 80-90%.\n\n**Model 2: The Platform Business (Higher Reward)**\nBuild a self-service onboarding flow. Let clients configure their own agents via Admin UI. Charge per-tenant monthly subscription. Scale to 50+ tenants on one server. Revenue at 50 tenants: ~$15,000/month.\n\n**Model 3: The Vertical SaaS (Highest Defensibility)**\nPick ONE vertical (e.g., restoration companies). Build industry-specific templates. Add vertical-specific features (emergency protocols, insurance claim workflows). Charge premium pricing ($499-$1,500/month).\n\n**Model 4: The Consulting + SaaS Pipeline**\nStart as a consultant (one-time setup fee). Transition to managed service (monthly recurring). Upsell to platform (self-service). Path: $2,000 one-time → $300/month → $15,000/month at scale.\n\n**Model 5: The Open-Source Monetization**\nKeep bchat open-source (community adoption). Offer managed hosting (bchat.cloud). Premium features: advanced analytics, priority support, custom integrations.\n\n**The Decision Framework:**\n\n| Model | Risk | Reward | Time to Revenue | Scalability |\n|-------|------|--------|-----------------|-------------|\n| Service | Low | Medium | 1-2 weeks | Limited by your time |\n| Platform | Medium | High | 1-2 months | Unlimited |\n| Vertical SaaS | High | Very High | 3-6 months | Unlimited + defensible |\n| Consulting → SaaS | Low | High | 1 week → 6 months | Grows with you |\n| Open-Source | Low | Variable | 6+ months | Community-driven |\n\n### Do This\n\n- Pick one business model that fits your situation\n- Write a one-page business plan: target vertical, pricing, cost structure, first 3 clients\n- Identify the first client you would approach\n\n### Think About This\n\n- Which model has the lowest barrier to entry?\n- Which model has the highest long-term value?\n- What's your unfair advantage? (Technical skill? Domain knowledge? Sales ability?)\n- How does the $246/mo replacement narrative help you sell?\n\n---\n\n### 19.7 Your First Client: A Practical Exercise\n\n**The Scenario:**\nYou've deployed bchat. You have the codebase assistant running. Now you're going to onboard your first client: \"Mike's Pressure Washing.\"\n\n**Step 1: The Interview (30 minutes)**\nCall Mike and ask:\n- What services do you offer? (pressure washing, soft wash, gutter cleaning)\n- What areas do you serve? (within 30 miles of Austin, TX)\n- What are your hours? (Mon-Sat 7am-6pm, emergency available)\n- What questions do customers ask most? (pricing, availability, what's included)\n- What's your pricing structure? (by square footage, minimum $150)\n- Do you have a website? (yes, but outdated)\n- How do you currently handle leads? (voicemail, callback)\n\n**Step 2: Write the KB.MD**\nUsing the templates from Chapter 3, write Mike's knowledge base:\n- Services with annotations (each service = one chunk)\n- FAQs (pricing, availability, what's included)\n- Coverage areas (include/exclude)\n- Exclusions (what you DON'T do: no roof cleaning, no interior work)\n\n**Step 3: Write the POLICY.MD**\n- Identity: \"You are Mike's scheduling assistant\"\n- Intents: schedule_service, ask_pricing, check_availability, emergency\n- Rules: always collect name + phone, never quote exact pricing without inspection\n\n**Step 4: Write the SCRIPT.MD**\nOpening → Discovery → Information → Qualification → Closing\n\n**Step 5: Upload, Reindex, Test**\nUpload files, click \"Rebuild Index.\" Run simulation with 3 virtual customers. Score the responses (4+ = pass).\n\n**Step 6: Embed the Widget**\nAdd `<script src=\"https://your-server.com/widget.js\" data-tenant=\"mikes-pressure-washing\">` to Mike's website.\n\n**Step 7: Go Live**\nMonitor first 10 conversations. Adjust KB.MD based on real questions. Send Mike his first lead report.\n\n### Do This\n\n- Complete Steps 1-5 for a real or fictional business\n- Run the simulation and score the results\n- Identify the 3 weakest responses and improve the KB.MD\n\n### Think About This\n\n- What questions did Mike ask that you didn't expect?\n- How did the simulation reveal gaps in your KB.MD?\n- What would you charge Mike for this service? Why?\n\n---\n\n### 19.8 The Dogfooding Loop: Continuous Improvement\n\nDogfooding is not a one-time exercise. It's a continuous loop:\n\n```\nUse the product → Find gaps → Fix them → Use again → Find more gaps\n```\n\n**The Weekly Practice:**\nEvery week, ask the codebase assistant 3 new questions. Score the responses. If any score below 4, rewrite the KB.MD section. Reindex and re-test.\n\n**The Monthly Review:**\nReview all questions asked this month. Identify patterns: which categories score lowest? Rewrite the weakest sections. Add new sections for undocumented features.\n\n**The Business Parallel:**\nYour customers will do the same with their agents. A plumber's KB.MD needs monthly updates (new services, pricing changes, seasonal FAQs). The weekly practice teaches you the rhythm your customers need.\n\n### Do This\n\n- Set a weekly reminder to ask the codebase assistant 3 questions\n- Track scores over time\n- After 4 weeks, identify the trend: improving, stable, or degrading?\n\n### Think About This\n\n- How does continuous dogfooding change your relationship with the product?\n- What would happen if you stopped dogfooding after a month?\n- How would you teach your clients to do the same?\n\n---\n\n### 19.9 From Learner to Producer: The Mindset Shift\n\n**The Employee Mindset:**\n- \"I build what I'm told\"\n- \"My job is to write code\"\n- \"Success = shipping features\"\n\n**The Producer Mindset:**\n- \"I build what the market needs\"\n- \"My job is to solve problems\"\n- \"Success = revenue + impact\"\n\n**The Shift:**\nThis tutorial taught you to think like a producer:\n- Chapters 1-2: Product thinking (what to build and why)\n- Chapters 3-4: Schema design as contract (what data matters)\n- Chapters 5-7: Implementation (how to build it)\n- Chapters 8-9: Migration (how to evolve it)\n- Chapters 10-12: Deployment (how to ship it)\n- Chapters 13-14: Operations (how to keep it running)\n- Chapters 15-18: Critical thinking (how to think about it)\n- Chapter 19: Dogfooding (how to use it to build a business)\n\n**The Final Exercise:**\nWrite a one-page \"Producer's Manifesto\":\n1. What problem do you want to solve?\n2. Who has that problem?\n3. How much are they willing to pay?\n4. What's your cost to serve them?\n5. What's your first step?\n\n### Think About This\n\n- Is the goal to build software, or to build a business that uses software?\n- What's the difference between a developer and a producer?\n- How does this tutorial change how you think about your next project?\n\n---\n\n# APPENDICES\n\n---\n\n## Appendix A: bugs/033 File Reference\n\n| File | Purpose | Key Takeaway |\n|------|---------|-------------|\n| `plan_biz.md` | Original business plan | $246/mo replacement narrative, 3-phase roadmap |\n| `plan_biz_review.md` | Adversarial review | 5 critical + 6 major findings → REWORK |\n| `plan2.md` | Revised plan | Scoped to webhooks + SMS, addressed all findings |\n| `plan3.md` | Distributed resilience | Outbox pattern, FOR UPDATE SKIP LOCKED |\n| `plan3_review.md` | Adversarial review | \"Same transaction\" claim unimplementable → REWORK |\n| `plan4.md` | Approved final plan | Tenant-provided credentials, post-insert outbox |\n| `plan4_review.md` | Approved with nits | 4 amendments (MySQL stubs, pre-claimed events, max attempts, SMS deferred) |\n| `plan5.md` | Final consolidated plan | All review findings incorporated |\n| `prompt.md` | Strategic debate | Webhook-first vs. native integrations pivot |\n| `prompt_code_review.md` | Code review scope | Adversarial review checklist |\n| `code_review.md` | Code review findings | Idempotency key swap, poller context cancelled |\n| `code3_plan.md` | Final fix plan | Variadic idempotency key, PII masking |\n| `docs_033.md` | End-user documentation | Testing steps, API reference, payload schema |\n| `heliosai_workflow.md` | Competitor analysis | Helios AI 5-step onboarding |\n| `resilience_guide.md` | Neon Postgres patterns | Transient errors, retry with backoff |\n\n---\n\n## Appendix B: API Endpoint Reference\n\n| Method | Path | Description | Permission |\n|--------|------|-------------|------------|\n| `GET` | `/api/v1/agent/:slug/integrations` | List integrations | `tenant:read` |\n| `POST` | `/api/v1/agent/:slug/integrations` | Create integration | `tenant:admin` |\n| `PUT` | `/api/v1/agent/:slug/integrations/:id` | Update integration | `tenant:admin` |\n| `DELETE` | `/api/v1/agent/:slug/integrations/:id` | Delete integration | `tenant:admin` |\n| `POST` | `/api/v1/agent/:slug/integrations/:id/test` | Test integration | `tenant:admin` |\n| `GET` | `/api/v1/agent/:slug/events` | List events | `tenant:read` |\n| `POST` | `/api/v1/system/trigger-cron` | Trigger poller | `X-Cron-Token` header |\n\n---\n\n## Appendix C: Database Schema Reference\n\n**agent_integrations:** Webhook configurations per tenant. One webhook per type per type (`UNIQUE(tenant_id, integration_type)`). `ON DELETE CASCADE` with `agent_tenants`.\n\n**agent_events:** Outbox queue for reliable event delivery. Pre-claimed on insert (`status='processing'`). Idempotency key with UNIQUE index. Lease-based recovery (`claimed_at < now() - 300`).\n\nMigration files: `store/migration/{sqlite,postgres}/0.31/00__agent_integrations.sql` and `01__agent_events.sql`.\n\n---\n\n## Appendix D: Environment Variables\n\n| Variable | Default | Description |\n|----------|---------|-------------|\n| `OPENROUTER_API_KEY` | (required) | OpenRouter API key |\n| `LLM_MODEL` | `openai/gpt-4o-mini` | LLM model for chat |\n| `LLM_MODEL_REASONING` | `google/gemini-2.5-pro` | LLM for content generation |\n| `RAG_PIPELINE_ENABLED` | `false` | Enable RAG pipeline |\n| `EMBEDDING_PROVIDER` | `mock` | Embedding provider |\n| `EMBEDDING_MODEL` | `text-embedding-3-small` | Embedding model |\n| `LANCEDB_STORAGE_PROVIDER` | `memory` | LanceDB storage |\n| `LANCEDB_LOCAL_PATH` | `build/data/lancedb` | LanceDB local path |\n| `HYBRID_SEARCH_ENABLED` | `false` | Enable hybrid search |\n| `DRIVER` | `sqlite` | Database driver |\n| `DATABASE_URL` | (none) | PostgreSQL connection string |\n| `ENCRYPTION_MASTER_KEY` | (none) | AES-256 encryption key |\n\n---\n\n## Appendix E: Competitor Analysis\n\n**Intercom AI ($97/mo):** Mature customer messaging platform with AI chat agent (Fin). Per-seat pricing. Not configurable via markdown files. General-purpose, not optimized for service businesses.\n\n**Helios AI ($100/mo):** Managed \"digital front desk\" for service businesses. WhatsApp + Cal.com integration. Fully managed setup (zero-config for tenant). Closed ecosystem. Opaque AI configuration.\n\n**n8n ($49/mo):** Open-source workflow automation. 400+ native integrations. Self-hostable. No AI chat capabilities. Requires technical knowledge to set up workflows.\n\n**bchat advantage:** All three combined in one platform at $149-$499/month. Configuration-driven (KB.MD/POLICY.MD). Self-service. Open-source foundation. 80-90% gross margins.\n\n---\n\n## Appendix F: Glossary\n\n| Term | Definition |\n|------|------------|\n| **Annotation** | HTML comment format `<!-- @type: value -->` used in KB.MD/POLICY.MD |\n| **BM25** | Best Matching 25 — keyword-based text scoring algorithm |\n| **Chunking** | Splitting documents into smaller pieces for RAG indexing |\n| **cmux** | Connection multiplexer — serves HTTP and gRPC on same port |\n| **Dead letter** | Events that exhausted all retry attempts |\n| **Driver** | Database implementation (SQLite, PostgreSQL) |\n| **Embedding** | Converting text to vector representation |\n| **Fusion engine** | Combines OM + RAG results for \"unlimited memory\" |\n| **Hybrid search** | Vector similarity (70%) + BM25 keyword matching (30%) |\n| **Idempotency key** | Unique identifier ensuring each operation happens exactly once |\n| **Lease expiry** | Recovery mechanism for stuck claims (5 minutes) |\n| **LanceDB** | Vector database for RAG (optional, CGO-dependent) |\n| **Observational Memory (OM)** | Long-term memory via compressed conversation observations |\n| **Outbox pattern** | Events stored in DB first, then dispatched — ensures durability |\n| **Pre-claimed event** | Event inserted with `status='processing'` to prevent double-dispatch |\n| **RAG** | Retrieval-Augmented Generation — using retrieved chunks to ground LLM responses |\n| **RBAC** | Role-Based Access Control |\n| **SSRF** | Server-Side Request Forgery — attack via user-supplied URLs |\n| **Tenant** | An isolated agent configuration (one per business) |\n| **Vector database** | Database optimized for similarity search on embeddings |\n\n---\n\n## Learning Path\n\n| Week | Focus | Chapters |\n|------|-------|----------|\n| 1 | Product thinking + Architecture | 1-2 |\n| 2 | Database design + Store layer | 3-4 |\n| 3 | Backend + Frontend + Testing | 5-7 |\n| 4 | Migration + Deployment | 8-12 |\n| 5 | Operations + Critical thinking | 13-18 |\n| 6 | Dogfooding + Business | 19 |\n\n---\n\n## Tutorial Differentiators\n\n| What This Tutorial Does | What Typical Tutorials Do |\n|---|---|\n| Opens with business context | Opens with \"install Node.js\" |\n| Every technical decision traces to business impact | Technical decisions are \"best practices\" |\n| Uses real failure analysis (bugs/033) | Shows only happy paths |\n| Teaches adversarial review culture | Teaches code review as bug-finding |\n| Ends with \"build a business\" | Ends with \"you finished the tutorial\" |\n| Dogfoods its own product | References external tools |\n| Critical thinking prompts at every chapter | \"Congrats, you completed Chapter N\" |\n\n---\n\n*This tutorial was built using bchat's own RAG pipeline to learn the bchat codebase. That's the point.*\n"
}
```

**Output:**
```
Wrote file successfully.
```

---

## Assistant (Build · MiMo V2.5 Free · 8.0s)

_Thinking:_

The file has been written. Let me update the todo list and verify the file was created correctly.

**Tool: todowrite**

**Input:**
```json
{
  "todos": [
    {
      "content": "Write Part I: Product Thinking (Chapters 1-2)",
      "priority": "high",
      "status": "completed"
    },
    {
      "content": "Write Part II: Database Design (Chapters 3-4)",
      "priority": "high",
      "status": "completed"
    },
    {
      "content": "Write Part III: Implementation (Chapters 5-7)",
      "priority": "high",
      "status": "completed"
    },
    {
      "content": "Write Part IV: Database Migration (Chapters 8-9)",
      "priority": "high",
      "status": "completed"
    },
    {
      "content": "Write Part V: Deployment (Chapters 10-12)",
      "priority": "high",
      "status": "completed"
    },
    {
      "content": "Write Part VI: Operations (Chapters 13-14)",
      "priority": "high",
      "status": "completed"
    },
    {
      "content": "Write Part VII: Critical Thinking (Chapters 15-18)",
      "priority": "high",
      "status": "completed"
    },
    {
      "content": "Write Part VIII: Meta-Learning (Chapter 19)",
      "priority": "high",
      "status": "completed"
    },
    {
      "content": "Write Appendices and README",
      "priority": "medium",
      "status": "completed"
    }
  ]
}
```

**Output:**
```
[
  {
    "content": "Write Part I: Product Thinking (Chapters 1-2)",
    "status": "completed",
    "priority": "high"
  },
  {
    "content": "Write Part II: Database Design (Chapters 3-4)",
    "status": "completed",
    "priority": "high"
  },
  {
    "content": "Write Part III: Implementation (Chapters 5-7)",
    "status": "completed",
    "priority": "high"
  },
  {
    "content": "Write Part IV: Database Migration (Chapters 8-9)",
    "status": "completed",
    "priority": "high"
  },
  {
    "content": "Write Part V: Deployment (Chapters 10-12)",
    "status": "completed",
    "priority": "high"
  },
  {
    "content": "Write Part VI: Operations (Chapters 13-14)",
    "status": "completed",
    "priority": "high"
  },
  {
    "content": "Write Part VII: Critical Thinking (Chapters 15-18)",
    "status": "completed",
    "priority": "high"
  },
  {
    "content": "Write Part VIII: Meta-Learning (Chapter 19)",
    "status": "completed",
    "priority": "high"
  },
  {
    "content": "Write Appendices and README",
    "status": "completed",
    "priority": "medium"
  }
]
```

---

## Assistant (Build · MiMo V2.5 Free · 9.6s)

_Thinking:_

Let me verify the file was created correctly by checking its size and reading a portion of it.

**Tool: bash**

**Input:**
```json
{
  "command": "wc -l /home/chaschel/Documents/go/bchat/TUTORIAL_MIMO.md && wc -c /home/chaschel/Documents/go/bchat/TUTORIAL_MIMO.md",
  "description": "Check file size and line count"
}
```

**Output:**
```
1487 /home/chaschel/Documents/go/bchat/TUTORIAL_MIMO.md
56710 /home/chaschel/Documents/go/bchat/TUTORIAL_MIMO.md

```

---

## Assistant (Build · MiMo V2.5 Free · 6.3s)

_Thinking:_

The file has been written successfully. Let me verify the structure by checking the table of contents.

Done. `TUTORIAL_MIMO.md` written — **1,487 lines, ~57KB**.

**Structure:**
- 9 Parts | 19 Chapters | 6 Appendices
- Product Thinking → Database Design → Implementation → Migration → Deployment → Operations → Critical Thinking → Dogfooding RAG
- Chapter 19 as capstone: dogfood RAG to learn the codebase, then build a business
- Every chapter has "Do This" (hands-on) and "Think About This" (critical thinking)
- bugs/033 runs through the entire tutorial as the thread

---

## User

now plan how to write the comprehensive DOCS_ONBOARDING.md the goal is for bchat implementation specialists to onboard new customers ensuring that they get all the requirements, design and write KB.md POLICY.md SCRIPT.md with help from LLM, gather customer's domain names, set up chat widget on Hugo, Wordpress and other popular CMS, think of all the possible combinations where customers might need robust RAG-based knowledge base, want intelligent transactional AI agents, all the elements and features available on bchat, the goal is to keep the customer up and running in no time, the implementation specialist need not be a developer or ops but technical enough to understand the nuances, they are more of product people with technical skills, let us make this Q&A until we agree on a plan

---

## Assistant (Plan · MiMo V2.5 Free · 262.3s)

_Thinking:_

The user wants me to plan a comprehensive DOCS_ONBOARDING.md document. Let me explore the codebase to understand:

1. What features/capabilities bchat offers
2. What the onboarding process should cover
3. What CMS integrations exist (Hugo, WordPress, etc.)
4. What RAG configurations are available
5. What the implementation specialist needs to know

Let me explore the codebase thoroughly to understand all the pieces.

Let me explore the codebase to understand all features, integrations, and what an implementation specialist needs to know.

**Tool: task**

**Input:**
```json
{
  "description": "Explore bchat widget and CMS integrations",
  "prompt": "Thoroughly explore the bchat widget system and CMS integration capabilities:\n\n1. Read the widget directory structure and all key files\n2. Read `docs/DOCS_WIDGET.MD` completely\n3. Read the widget's HTML/JS embed code\n4. Search for Hugo, WordPress, Shopify, Wix, Squarespace references in docs\n5. Read any deployment/integration guides\n6. Look at the widget's configuration options (data attributes, customization)\n7. Check how the widget communicates with the backend (API calls, CORS, auth)\n8. Look at the Hugo site builder mentioned in business docs\n\nFocus on:\n- How the widget is embedded on different platforms\n- What configuration options are available\n- What customization is possible (colors, branding, position)\n- How authentication works for the widget\n- What the widget looks like on mobile vs desktop\n- Any pre-built templates or themes\n\nReturn a comprehensive summary of widget capabilities, embedding methods, and CMS integration options.",
  "subagent_type": "explore"
}
```

**Output:**
```
<task id="ses_0b305e4f2ffe9NEcfd2vVeSzXM" state="completed">
<task_result>
Now I have all the information needed. Here is a comprehensive summary of the bchat widget system and CMS integration capabilities.

---

## Comprehensive Summary: bchat Widget System and CMS Integration

### 1. Widget Architecture Overview

The bchat widget is a **self-contained, embeddable chat interface** built as an IIFE (Immediately Invoked Function Expression) bundle. It is a TypeScript/Vite project located at `/home/chaschel/Documents/go/bchat/widget/` and produces a single `embed.min.js` file (~15-20KB).

**Source files:**
- `/home/chaschel/Documents/go/bchat/widget/src/embed.ts` -- JS embed entry point
- `/home/chaschel/Documents/go/bchat/widget/src/iframe.ts` -- iframe entry point
- `/home/chaschel/Documents/go/bchat/widget/src/core/api.ts` -- API client (chat message sending)
- `/home/chaschel/Documents/go/bchat/widget/src/core/state.ts` -- Pub/sub state management
- `/home/chaschel/Documents/go/bchat/widget/src/core/types.ts` -- TypeScript interfaces and defaults
- `/home/chaschel/Documents/go/bchat/widget/src/ui/Widget.ts` -- Main orchestrator
- `/home/chaschel/Documents/go/bchat/widget/src/ui/Button.ts` -- Floating toggle button
- `/home/chaschel/Documents/go/bchat/widget/src/ui/Panel.ts` -- Chat panel with header/controls
- `/home/chaschel/Documents/go/bchat/widget/src/ui/Messages.ts` -- Message list rendering
- `/home/chaschel/Documents/go/bchat/widget/src/ui/Input.ts` -- Text input with send button
- `/home/chaschel/Documents/go/bchat/widget/src/styles/styles.ts` -- All CSS (inline, no external stylesheets)
- `/home/chaschel/Documents/go/bchat/widget/src/iframe.html` -- iframe HTML template
- `/home/chaschel/Documents/go/bchat/widget/vite.config.ts` -- Vite build config (IIFE output)

**Build output:**
- `/home/chaschel/Documents/go/bchat/widget/dist/embed.min.js` -- Production bundle
- Built via `task build:widget` or `cd widget && npm run build`

---

### 2. Three Embedding Methods

#### Method 1: JavaScript Embed (Recommended)

**Best for:** Custom HTML websites, full control over floating bubble and panel.

```html
<script
  src="https://your-server.com/widget/your-tenant-slug/embed.js"
  data-position="bottom-right"
  data-color="#0d9488"
  data-welcome="How can we help you today?"
></script>
```

**Backend handler:** `HandleWidgetEmbed` at `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/handlers.go:2016`
- Reads the built `embed.min.js` from `widget/dist/embed.min.js`
- Injects tenant configuration (`baseUrl`, `tenant`, `companyName`, `widgetKey`) at the top of the script
- Serves with `Cache-Control: public, max-age=3600` (1 hour)
- Falls back to an inline-generated script if the built file is missing

**Route:** `GET /widget/:slug/embed.js` (public, no auth, permissive CORS `AllowOrigins: ["*"]`)

#### Method 2: iframe Embed

**Best for:** CMS platforms (WordPress, Shopify, Wix, Squarespace) where script injection is limited or restricted.

```html
<iframe
  src="https://your-server.com/widget/your-tenant-slug/iframe?color=%230d9488"
  style="position:fixed;bottom:0;right:0;width:400px;height:600px;border:none;z-index:9999;"
  title="Chat Widget"
></iframe>
```

**Backend handler:** `HandleWidgetIframe` at `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/handlers.go:2073`
- Generates a standalone HTML page with the widget embedded
- Hides the floating toggle button (`display: none`)
- Makes the panel fill the entire iframe (100% width/height, static positioning)
- Auto-opens the panel on load via a `setTimeout` click
- Accepts query parameters: `color`, `welcome`, `companyName`
- URL-encode `#` as `%23` for hex colors

**Route:** `GET /widget/:slug/iframe` (public, no auth, permissive CORS)

#### Method 3: Legacy Script (Backward Compatible)

```html
<script src="https://your-server.com/api/v1/agent/your-tenant-slug/widget.js"></script>
```

**Backend handler:** `HandleWidget` at `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/handlers.go:1693`
- Generates an inline JS loader that sets `window.AgentChatConfig` and dynamically loads `embed.js`
- Used for backward compatibility with existing integrations
- The loader delegates all rendering to the modern `embed.min.js` bundle

**Route:** `GET /api/v1/agent/:slug/widget.js` (public, no auth)

---

### 3. Configuration Options

#### JavaScript Embed (data-* attributes)

| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| `data-position` | string | `bottom-right` | `bottom-right` or `bottom-left` |
| `data-color` | string | `#0d9488` | Primary brand color (hex) |
| `data-welcome` | string | `How can we help you today?` | Welcome message |
| `data-company-name` | string | (from tenant) | Company name in header |
| `data-button-size` | number | `56` | Toggle button diameter (px) |
| `data-panel-width` | number | `350` | Panel width (px) |
| `data-panel-height` | number | `500` | Panel height (px) |

#### Global Configuration (window.AgentChatConfig)

```html
<script>
  window.AgentChatConfig = {
    baseUrl: 'https://your-server.com',
    tenant: 'acme-corp',
    color: '#0d9488',
    position: 'bottom-right',
    welcomeMessage: 'How can we help you today?',
    companyName: 'Acme Corp',
    buttonSize: 56,
    panelWidth: 350,
    panelHeight: 500,
    widgetKey: 'optional-auth-key'
  };
</script>
<script src="https://your-server.com/widget/acme-corp/embed.js"></script>
```

**Precedence (highest to lowest):**
1. Explicit `AgentChatWidget.init()` call (programmatic)
2. Script `data-*` attributes
3. `window.AgentChatConfig` global
4. Hardcoded defaults

#### iframe Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `color` | string | `#0d9488` | Primary color (URL-encoded hex) |
| `welcome` | string | `How can we help you today?` | Welcome message |
| `companyName` | string | (from tenant) | Company name override |

#### Programmatic API

```javascript
AgentChatWidget.init({ tenant: 'slug', baseUrl: 'https://...', color: '#7c3aed' });
AgentChatWidget.open();   // Open the chat panel
AgentChatWidget.close();  // Close the chat panel
AgentChatWidget.getInstance();  // Get widget instance
```

---

### 4. Customization and Styling

#### CSS Override Selectors

All widget CSS is injected inline (no external stylesheet). You can override any style using `!important` after the script loads:

| Selector | Element |
|----------|---------|
| `#agent-chat-widget` | Root container |
| `#acw-toggle` | Floating button |
| `#acw-panel` | Chat panel |
| `#acw-header` | Panel header |
| `#acw-header-title` | Header title area |
| `#acw-header-controls` | Minimize/close buttons |
| `#acw-content` | Content area |
| `#acw-messages` | Messages container |
| `.acw-msg` | Message row |
| `.acw-msg-bubble` | Message bubble |
| `.acw-msg-user` | User message (right-aligned, colored) |
| `.acw-msg-assistant` | Agent message (left-aligned, dark) |
| `.acw-msg-role` | Role label ("Customer"/"Agent") |
| `.acw-msg-time` | Timestamp |
| `.acw-typing` | Typing indicator |
| `.acw-error` | Error display |
| `#acw-input-area` | Input container |
| `#acw-input-wrapper` | Pill-style input wrapper |
| `#acw-input` | Text input |
| `#acw-send` | Send button |

#### Visual Design

- **Dark theme** -- Panel background: `rgba(18, 18, 26, 0.95)` with backdrop blur
- **Font:** Inter (bundled as woff2), falling back to system-ui
- **Panel:** 24px border-radius, box-shadow depth, glass-morphism effect
- **Button:** Circular (50%), hover scale animation, shadow pulse
- **Animations:** Slide-up panel entry, fade-in messages, bounce typing dots
- **User bubbles:** Colored background matching `data-color`, white text
- **Agent bubbles:** Dark `#1a1a28` background, light text

---

### 5. Mobile Responsiveness

From `/home/chaschel/Documents/go/bchat/widget/src/styles/styles.ts:489-502`:

At viewport widths below **480px**:
- Panel width becomes `calc(100vw - 16px)` (nearly full screen)
- Panel height becomes `min(600px, calc(100dvh - 60px))` (uses dynamic viewport height)
- Margins reduced to 8px from edge
- Border-radius reduced to 12px
- Toggle button moves to 16px from edge

The panel uses `min(420px, 92vw)` and `min(680px, 85vh)` even on desktop, so it gracefully adapts to various screen sizes.

---

### 6. Authentication and Security

#### Widget Authentication Model

**The external widget endpoint (`/chat/ext`) requires NO authentication** -- it is designed for anonymous public users. However, there are several security layers:

1. **WidgetKey (Edge Authentication):** A per-tenant secret key injected into the widget script by the backend. The widget sends this as an `X-Widget-Key` header on chat API calls. The backend verifies it using constant-time comparison (`subtle.ConstantTimeCompare`). This prevents unauthorized third parties from using a tenant's chat endpoint.

2. **Domain Allowlist:** Each tenant can set `AllowedDomains` (JSON array). When set, the `HandleWidgetEmbed` and `HandleWidgetIframe` handlers check the `Origin` and `Referer` headers against this list. If the requesting domain is not allowed, a 403 Forbidden is returned.

3. **Session Tokens:** When a chat session is created, the backend generates an HMAC-based session token using the `WidgetKey`. The widget stores this in `localStorage` keyed by `bchat_session_id:{tenant}` and includes it on subsequent requests for transcript retrieval.

4. **Rate Limiting:** IP-based rate limiting is applied to the external chat endpoint with a 60-second cooldown. Returns HTTP 429 when exceeded.

5. **CORS Configuration:** From `/home/chaschel/Documents/go/bchat/server/router/api/v1/v1.go:240-275`:
   - Widget and chat routes: `AllowOrigins: ["*"]` (permissive)
   - Admin/auth routes: configurable via `ADMIN_CORS_ORIGINS` env var (restrictive)

6. **XSS Protection:** All user input is HTML-escaped before display. The widget uses `textContent` (not `innerHTML`) for message content insertion. The backend uses `json.Marshal` for JS string escaping to prevent `</script>` breakout.

7. **iframe Isolation:** The iframe embed is fully isolated from the parent page -- no access to parent DOM or cookies.

---

### 7. Backend API Communication

#### Chat API Endpoint

**Route:** `POST /api/v1/agent/:slug/chat/ext`

**Request:**
```json
{
  "session_id": "optional-existing-session-id",
  "message": "Hello, I need help",
  "client_message_id": "uuid-for-idempotency"
}
```

**Response:**
```json
{
  "session_id": "abc123",
  "message": {
    "role": "assistant",
    "content": "Hello! How can I help you today?",
    "timestamp": "2026-01-25T10:30:00Z"
  },
  "metadata": {
    "intent": "greeting",
    "confidence": 0.95
  },
  "bridge": {
    "status": "human_handoff_active",
    "handoff_id": "...",
    "routing_mode": "..."
  }
}
```

**Transcript Retrieval:** `GET /api/v1/agent/:slug/chat/ext/transcript?session_id=...`
- Used by the widget to restore conversation history on page reload
- Polls every 3 seconds when a human handoff is active

#### Widget Script Serving

| Route | Handler | Purpose |
|-------|---------|---------|
| `GET /widget/:slug/embed.js` | `HandleWidgetEmbed` | Built JS bundle with config injection |
| `GET /widget/:slug/iframe` | `HandleWidgetIframe` | Standalone HTML page for iframe |
| `GET /api/v1/agent/:slug/widget.js` | `HandleWidget` | Legacy loader script |

All widget routes have `Cache-Control: public, max-age=3600` (1 hour browser cache).

---

### 8. Session Management and Persistence

From `/home/chaschel/Documents/go/bchat/widget/src/core/state.ts`:

- **Session ID:** Stored in `localStorage` under key `bchat_session_id:{tenant}`
- **Pending Messages:** Stored in `localStorage` under key `bchat_pending:{tenant}` (JSON map of message text to UUID)
- **Session restore:** On widget mount, the widget checks `localStorage` for a saved session ID and fetches the full transcript from the server
- **Memory-only sessions on backend:** External chat sessions are stored in memory only and are lost on server restart (no persistent conversation history for external users)
- **Human handoff polling:** When a `bridge` state indicates human handoff is active, the widget polls the transcript endpoint every 3 seconds

---

### 9. CMS Platform Integration Guide

#### WordPress

**Method:** iframe embed (recommended for WordPress due to script injection limitations)

1. Add a Custom HTML block to any page/post
2. Paste the iframe code:
```html
<iframe
  src="https://your-server.com/widget/your-slug/iframe?color=%237c3aed"
  style="position:fixed;bottom:20px;right:20px;width:380px;height:550px;border:none;z-index:9999;border-radius:12px;box-shadow:0 8px 32px rgba(0,0,0,0.15);"
  title="Chat Support"
></iframe>
```
3. Alternatively, add the JS embed to your theme's footer via a plugin like "Insert Headers and Footers" or "Code Snippets"

#### Shopify

**Method:** iframe embed (Shopify restricts script injection to specific areas)

1. In Shopify Admin, go to Online Store > Themes > Edit Code
2. Add the iframe snippet to `theme.liquid` before `</body>`, or
3. Use a Custom Liquid section to add the iframe on specific pages

#### Wix

**Method:** iframe embed (Wix has limited script access)

1. In the Wix Editor, add an "HTML iframe" element
2. Set the iframe source to your widget URL
3. Adjust width/height in the iframe element settings

#### Squarespace

**Method:** iframe embed via Code Block

1. Add a Code Block to your page
2. Paste the iframe HTML
3. Squarespace Code Blocks support iframe elements

#### Hugo (Static Site)

**Method:** JavaScript embed (Hugo allows full HTML control)

1. Add the script tag to your Hugo template (e.g., `baseof.html` before `</body>`)
2. Or add it to individual page front matter
3. Hugo sites can use either JS embed or iframe -- full flexibility

```html
<script
  src="https://your-server.com/widget/your-slug/embed.js"
  data-position="bottom-right"
  data-color="#0d9488"
></script>
```

#### React/Next.js

**Method:** JavaScript embed with programmatic init

```jsx
import { useEffect } from 'react';

function ChatWidget({ tenantSlug, color = '#0d9488' }) {
  useEffect(() => {
    window.AgentChatConfig = { color };
    const script = document.createElement('script');
    script.src = `https://your-server.com/widget/${tenantSlug}/embed.js`;
    script.async = true;
    document.body.appendChild(script);
    return () => {
      document.getElementById('agent-chat-widget')?.remove();
      document.getElementById('acw-styles')?.remove();
      script.remove();
      delete window.AgentChatWidget;
    };
  }, [tenantSlug, color]);
  return null;
}
```

---

### 10. Hugo Site Builder (Business Offering)

The bchat business model includes a **"Hugo Site + AI Lead Desk"** package (referenced in `/home/chaschel/Documents/go/bchat/docs/docs_biz_guide.md:1056` and `/home/chaschel/Documents/go/bchat/docs/DOCS_HOWTO_BIZ.md:922`):

- **Package 2:** $399-$999 setup / $199-$499 monthly
- **Includes:** Fast Hugo landing site, domain/DNS configuration, service pages, quote/request form, bchat widget, lead notification workflow, Google Business Profile CTA optimization, monthly updates
- **Target:** Businesses with no website, bad website, or Facebook-only presence
- **First-market verticals:** Pressure washing, junk removal, cleaning, landscaping, pest control

The Hugo site builder is a business/service offering, not a code module in the repository. Hugo is used as a fast static site generator to quickly deploy landing pages for clients that don't have existing websites.

---

### 11. Pre-built Templates and Themes

The bchat codebase does **not include pre-built website templates or themes** in the traditional sense. However:

1. **KB.MD templates:** `/home/chaschel/Documents/go/bchat/docs/KB_TEMPLATE.MD` -- knowledge base template with annotation examples
2. **POLICY.MD templates:** `/home/chaschel/Documents/go/bchat/docs/POLICY_TEMPLATE.MD` -- policy template with identity, rules, intents, thresholds
3. **SCRIPT.MD templates:** Embedded in both business guide docs with conversation stage templates
4. **Widget design:** A single dark-themed, glass-morphism design inspired by Intercom. No light theme or multiple theme options exist currently.

---

### 12. Key Files Reference

| File | Purpose |
|------|---------|
| `/home/chaschel/Documents/go/bchat/docs/DOCS_WIDGET.MD` | Primary widget documentation (593 lines) |
| `/home/chaschel/Documents/go/bchat/widget/README.md` | Widget development README |
| `/home/chaschel/Documents/go/bchat/widget/src/embed.ts` | JS embed entry point (206 lines) |
| `/home/chaschel/Documents/go/bchat/widget/src/iframe.ts` | iframe entry point (108 lines) |
| `/home/chaschel/Documents/go/bchat/widget/src/core/types.ts` | Config types and defaults (106 lines) |
| `/home/chaschel/Documents/go/bchat/widget/src/core/api.ts` | Chat API client (75 lines) |
| `/home/chaschel/Documents/go/bchat/widget/src/core/state.ts` | State management with localStorage (178 lines) |
| `/home/chaschel/Documents/go/bchat/widget/src/ui/Widget.ts` | Main widget orchestrator (268 lines) |
| `/home/chaschel/Documents/go/bchat/widget/src/ui/Button.ts` | Floating toggle button (24 lines) |
| `/home/chaschel/Documents/go/bchat/widget/src/ui/Panel.ts` | Chat panel (91 lines) |
| `/home/chaschel/Documents/go/bchat/widget/src/ui/Messages.ts` | Message rendering (151 lines) |
| `/home/chaschel/Documents/go/bchat/widget/src/ui/Input.ts` | Input area (83 lines) |
| `/home/chaschel/Documents/go/bchat/widget/src/styles/styles.ts` | All CSS styles (540 lines) |
| `/home/chaschel/Documents/go/bchat/widget/src/iframe.html` | iframe HTML template (37 lines) |
| `/home/chaschel/Documents/go/bchat/widget/vite.config.ts` | Vite build config (40 lines) |
| `/home/chaschel/Documents/go/bchat/widget/package.json` | Package config (19 lines) |
| `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/handlers.go` | Backend widget handlers (lines 1691-2195) |
| `/home/chaschel/Documents/go/bchat/server/router/api/v1/v1.go` | Route registration and CORS (lines 238-276) |
| `/home/chaschel/Documents/go/bchat/docs/DOCS_HOWTO_BIZ.md` | Business guide with widget deployment instructions |
| `/home/chaschel/Documents/go/bchat/docs/docs_biz_guide.md` | Alternate business guide with CMS integration notes |
| `/home/chaschel/Documents/go/bchat/docs/BIZ_QUICK_START_CHECKLIST.MD` | Quick start checklist including widget deployment |
| `/home/chaschel/Documents/go/bchat/widget/site/index.html` | Test site with JS embed |
| `/home/chaschel/Documents/go/bchat/widget/site/iframe-test.html` | Test site with iframe embed |

---

### 13. Summary of Capabilities

| Capability | Status | Notes |
|------------|--------|-------|
| JavaScript embed (floating bubble) | Active | Recommended for custom sites |
| iframe embed (full panel) | Active | Recommended for CMS platforms |
| Legacy script backward compat | Active | Delegates to modern bundle |
| Color customization | Active | Via `data-color` or query param |
| Position customization | Active | `bottom-left` or `bottom-right` |
| Welcome message customization | Active | Via config or query param |
| Company name in header | Active | Auto-populated from tenant |
| Button size customization | Active | Via `data-button-size` |
| Panel size customization | Active | Via `data-panel-width/height` |
| Mobile responsive | Active | Auto-adapts below 480px viewport |
| Dark theme | Active | Only theme available (glass-morphism) |
| Light theme | Not available | -- |
| Multiple themes/presets | Not available | -- |
| Domain allowlist | Active | Per-tenant allowed domains |
| WidgetKey authentication | Active | Edge auth for API calls |
| Session persistence | Active | localStorage, memory on server |
| Transcript restore | Active | Fetches history on page reload |
| Human handoff polling | Active | 3-second interval when active |
| XSS protection | Active | textContent + json.Marshal |
| CORS (permissive for widget) | Active | AllowOrigins: ["*"] for public |
| Rate limiting | Active | IP-based, 60s cooldown |
| CSS override support | Active | All selectors documented |
| React integration example | Provided | In DOCS_WIDGET.MD |
| WordPress integration guide | Provided | iframe approach documented |
| Shopify/Wix/Squarespace | Supported | iframe approach (CMS-agnostic) |
| Hugo site builder | Business offering | Package 2 in pricing model |
| Pre-built website themes | Not available | Hugo sites built per-client |
</task_result>
</task>
```

**Tool: task**

**Input:**
```json
{
  "description": "Explore bchat all features comprehensively",
  "prompt": "Thoroughly explore ALL features and capabilities of bchat that an implementation specialist would need to understand:\n\n1. Read `docs/DOCS_README.MD` - full feature list\n2. Read `docs/DOCS_AGENT_ARCHITECTURE.MD` - configuration-driven design\n3. Read `docs/KB_TEMPLATE.MD` - KB template with all annotations\n4. Read `docs/POLICY_TEMPLATE.MD` - POLICY template with all annotations\n5. Read `docs/DOCS_SIMULATION.MD` - simulation/testing features\n6. Read `docs/DOCS_RAG_PIPELINE.MD` - RAG configuration\n7. Read `docs/DOCS_OM*.MD` files - observational memory\n8. Read `docs/DOCS_HALLUCINATION_GROUNDING.MD` - response verification\n9. Read `docs/DOCS_EVALUATION.MD` - evaluation features\n10. Read `docs/DOCS_CONTEXT_AWARENESS.MD` - context awareness\n11. Read `docs/DOCS_RBAC*.MD` - permission system\n12. Read `docs/docs_biz_guide.md` - business guide\n13. Search for all available environment variables that affect behavior\n14. Search for all API endpoints\n15. Look at the AgentAdmin UI sections - what can be configured via UI\n\nFocus on:\n- Every feature that can be enabled/configured\n- Every annotation type and what it does\n- Every integration option\n- Every customization point\n- What the implementation specialist MUST know vs. nice-to-know\n\nReturn a comprehensive feature inventory organized by category (core, RAG, security, integrations, analytics, etc.)",
  "subagent_type": "explore"
}
```

**Output:**
```
<task id="ses_0b305caf9ffexHbUd3cxqRXsbX" state="completed">
<task_result>
Now I have a comprehensive picture of the entire bchat system. Here is the complete feature inventory organized by category.

---

# BCHAT COMPREHENSIVE FEATURE INVENTORY

## 1. CORE PLATFORM

### 1.1 Multi-Tenant Architecture
- **Tenant Isolation**: Every data operation scoped to a single tenant via `tenant_id` foreign keys with cascading deletes
- **Tenant Model Fields**: `id`, `slug` (URL-safe), `company_name`, `vertical`, `is_active`, `llm_model` (per-tenant override), `temperature` (0.0-1.0), `allowed_domains` (JSON array), `processing_options` (JSON), `guid` (unique widget embed identifier)
- **Multi-Audience Support**: Each tenant supports `internal` (staff) and `external` (customer) audiences with separate KB/Policy content
- **Tenant CRUD**: Create, read, update, delete via Agent Admin UI and API
- **Tenant Onboarding**: `HandleOnboard` endpoint for guided setup
- **Tenant Settings**: Per-tenant configurable settings via `HandleGetTenantSettings` / `HandleUpdateTenantSettings`

### 1.2 Configuration-Driven Agent Design
- **Zero Tenant-Specific Code**: All agent behavior defined by three markdown files
- **General-Purpose Architecture**: Works for any vertical (HVAC, insurance, HR, legal, Q&A, technical support) without code changes
- **Annotation-Based Parsing**: `<!-- @type: params -->` syntax for machine-readable content
- **Generic Intent System**: Intents defined in POLICY.MD, not code

### 1.3 Three Configuration Files

#### KB.MD (Knowledge Base) - The Agent's Brain
| Annotation | Purpose | Parameters |
|------------|---------|------------|
| `@service` | Service/product entries | `emergency: true/false`, `response_time: "60min"` |
| `@exclusion` | Services NOT offered | `exception: "secondary to X"` |
| `@coverage` | Geographic areas | `include` or `exclude` |
| `@faq` | Q&A pairs | `category` identifier |
| `@safety` | Emergency protocols | `triggers: keyword1, keyword2` |
| `@section` | Generic content sections | `type: catalog/internal/regulation/etc.` |

#### POLICY.MD (Agent Policy) - The Agent's Operating System
| Annotation | Purpose | Parameters |
|------------|---------|------------|
| `@identity` | Agent persona, tone, role | `tone`, `voice` |
| `@intent` | Customer intent categories | `category: standard/emergency/sensitive/meta`, `urgency: 1-5`, `priority: P0-P2`, `action: flow_name` |
| `@rule` | Behavioral rules | `priority: 1-N` |
| `@threshold` | Numeric thresholds | `value: N` (urgency scoring ranges) |

#### SCRIPT.MD (Conversation Flow) - The Agent's Playbook
- Defines conversation stages (Opening, Discovery, Information, Qualification, Closing)
- Per-tenant (not per-audience) - shared across all audiences
- Included fully in system prompt (NOT RAG-indexed by design)
- Parsed into `ParsedScript` with `Sections` and `Summary`

### 1.4 Configuration Priority System
```
1. Tenant Config (Agent Admin UI) → Highest priority
         ↓ (if empty)
2. Environment Variable → Fallback
         ↓ (if empty)
3. Hardcoded Default → Lowest priority
```

---

## 2. RAG PIPELINE (Retrieval-Augmented Generation)

### 2.1 Architecture
- **Document Upload** -> **Chunker** (~500 tokens, 50 overlap) -> **Embedding Service** -> **VectorDB** -> **Query-Time Retrieval**
- **Content Indexed**: KB.MD and POLICY.MD (per audience type)
- **Content NOT Indexed**: SCRIPT.MD (included fully in prompt by design)

### 2.2 Embedding Providers
| Provider | Dimensions | Cost | Use Case |
|----------|------------|------|----------|
| `openrouter` | 1536 | ~$0.02/1M tokens | Production (recommended) |
| `local` | 384 | Free | Development (requires Python server at `http://localhost:8001/embed`) |
| `mock` | 384 | Free | Pipeline testing only (random vectors, no semantic accuracy) |

**Embedding models**: `openai/text-embedding-3-small` (default), `openai/text-embedding-3-large`, `openai/text-embedding-ada-002`

### 2.3 Vector Database Storage
| Backend | Persistence | Build Required | Use Case |
|---------|-------------|----------------|----------|
| `memory` | Lost on restart | No | Development, testing |
| `local` | Disk | Yes (`-tags rag`) | Single-server production |
| `s3` | S3/Tigrisdata | Yes (`-tags rag`) | Multi-server production (fly.io) |

### 2.4 Hybrid Search
- **Vector-only** mode (semantic understanding)
- **Hybrid** mode (70% vector similarity + 30% BM25 keyword matching)
- **BM25-only** mode
- Configurable weights via `HYBRID_VECTOR_WEIGHT` and `HYBRID_TEXT_WEIGHT`
- Score fusion algorithm combines both scores

### 2.5 Re-indexing
- **Automatic**: On every KB/POLICY file upload
- **Manual**: Via "Rebuild Index" button in Agent Admin
- **Startup**: `FORCE_REINDEX_ON_STARTUP=true` for bulk re-index
- **Disable**: `RAG_STARTUP_REINDEX_DISABLED=true` to skip startup reindex

### 2.6 RAG Processing Options
- **Format for RAG**: Preview/configure chunking options (ephemeral)
- **Save as Default**: Store processing options per-tenant
- **Auto-mode**: RAG mode when KB > ~6,000 tokens (~24KB), long-context mode otherwise

---

## 3. SECURITY & ACCESS CONTROL

### 3.1 RBAC Permission System
| Permission | Description |
|------------|-------------|
| `tenant:admin` | Full tenant management (CRUD, config, files) |
| `tenant:read` | View tenant configuration |
| `tenant:write` | Modify tenant settings (not delete) |
| `chat:test` | Run simulations, view test chat history |
| `chat:logs` | View real chat session logs |
| `files:upload` | Upload KB/Policy/Script files |
| `files:restore` | Restore file versions |
| `api:config` | Configure LLM settings, rebuild index |

### 3.2 Role Templates
- **Viewer**: `tenant:read`
- **Tester**: `tenant:read`, `chat:test`
- **Analyst**: `tenant:read`, `chat:logs`
- **Editor**: `tenant:read`, `tenant:write`, `files:upload`
- **Tenant Admin**: `tenant:*`

### 3.3 Global Role Hierarchy
- **HOST**: Superuser, all permissions
- **ADMIN**: Implicit `tenant:read` for all tenants
- **USER**: Basic access only

### 3.4 Authentication
- JWT-based with `ClaimsMessage` including `TenantID` field
- gRPC interceptor (`acl.go`) and Echo middleware for REST
- Cookie and Bearer token support
- Multi-tenant user flow: `POST /api/v1/auth/tenants` -> `POST /api/v1/auth/select-tenant`

### 3.5 Tenant Isolation Enforcement
- Every handler extracts tenant ID from context
- `ApplyTenantFilter` wrapper for database queries
- Superuser bypass pattern with explicit check
- Cross-tenant data access denied

### 3.6 API Key Encryption
- AES-256-GCM encryption for tenant API keys
- Argon2id key derivation from `ENCRYPTION_MASTER_KEY`
- Per-tenant encrypted key storage in `tenant_config` table
- Key rotation support via `key_version`

### 3.7 Widget Security
- IP-based rate limiting (60-second cooldown)
- XSS protection (HTML escaping, `textContent` usage)
- Domain restriction via `allowed_domains` per tenant
- iframe isolation from parent page

---

## 4. RESPONSE QUALITY & VERIFICATION

### 4.1 Three-Layer Validation System
| Layer | Status | Purpose | Latency |
|-------|--------|---------|---------|
| Output Sanitization | Always active | System tag removal, contact correction | < 5ms |
| LLM Verification | Optional (`LLM_VERIFIER_ENABLED=true`) | Semantic grounding against KB | 200-500ms |

### 4.2 Output Sanitization (sanitizer.go)
- System tag removal: `[SYSTEM]`, `[DEBUG]`, `[THOUGHT]`, `[INTERNAL]`, `[NOTE]`, `Assistant:`, `AI:`, `Bot:`
- Thinking pattern removal: `Let me think...`, `I should...`, `*action descriptions*`
- Impossible phone correction: `000-000-0000`, `999-999-9999`, `123-456-7890`
- Placeholder email correction: `@example.com`, `@test.com`, `user@user.com`
- Phone validation: Rejects 555 area code, 000 area code, test numbers
- Post-verification re-sanitization

### 4.3 LLM-Based Verification (verifier.go)
- Verification checklist covering: contact info grounding, service grounding, claims grounding, timeline/process claims, customer info handling, tone/identity
- Violation types with severity levels: critical, high, medium, low
- Two modes: `shadow` (log only) and `enforce` (apply corrections)
- `VerificationMetrics` tracking: total verifications, compliant responses, violations detected, corrections made, average latency

---

## 5. CONVERSATION MANAGEMENT

### 5.1 Chat Interfaces
| Endpoint | Auth | Purpose |
|----------|------|---------|
| `POST /api/v1/agent/:slug/chat/ext` | None | External (widget) chat |
| `POST /api/v1/agent/:slug/chat/int` | Required | Internal (staff) chat |
| `POST /api/v1/agent/:slug/chat` | None | Public chat |

### 5.2 SSE Streaming
- Real-time Server-Sent Events for chat responses
- Streaming responses as they are generated
- Session persistence via `session_id`

### 5.3 Conversation History
- Unified conversation dropdown (simulations + real chats)
- Max 50 conversations displayed
- Permission-gated: `chat:test` for simulations, `chat:logs` for real chats
- Full transcript viewing with color-coded messages

### 5.4 Observational Memory (OM)
- **Purpose**: Infinite effective memory by compressing conversation history into observation logs
- **Architecture**: Observer compresses unobserved messages -> Reflector compresses observation log when too large
- **Scope Options**: `thread` (current session) or `resource` (all sessions for user)
- **Buffer System**: Activates at 80% threshold, blocks at 120%
- **Retry Logic**: Configurable retry attempts and delay

### 5.5 Hybrid OM + RAG
- Observations indexed to vector database
- Temporal weighting prioritizes recent observations
- Fusion engine combines OM and RAG results
- TTL-based observation expiration (default 90 days)
- Compression for RAG storage

---

## 6. TESTING & SIMULATION

### 6.1 Agent Simulation
- **LLM-powered virtual customer**: Generates realistic human responses
- **Scenario configuration**: Initial prompt + optional persona hint
- **SSE streaming**: Watch conversation unfold in real-time
- **Auto-run**: 10-50 turns (minimum 10, safety cap 50)
- **End detection**: Agent phase detection, LLM analysis, keyword fallback
- **Transcript storage**: Full conversation saved to database
- **Rate limiting**: 1 concurrent per user, 10 per hour per tenant

### 6.2 Simulation Controls
- Start, Pause, Resume, Stop
- Pause/Resume mid-conversation
- Stop saves transcript

### 6.3 Human Simulator Prompt
- Natural, human-like responses with casual language, contractions
- Realistic behavior: gradual info provision, emotional reactions
- Stays in character (never mentions being simulation/AI)
- Conversation dynamics: follow-up questions, off-topic tangents

### 6.4 Transcript Benchmark Analysis
- **Scoring rubric** (100 points total):
  - Intent Recognition: 15 points
  - Service Alignment: 15 points
  - Policy Compliance: 20 points
  - Conversation Flow: 20 points
  - Information Gathering: 15 points
  - Tone & Resolution: 15 points
- **Grading scale**: A (90-100), B (80-89), C (70-79), D (60-69), F (0-59)
- **Issue identification**: Severity levels with turn numbers
- **Improvement suggestions**: Actionable recommendations

### 6.5 QA Pairs Testing
- Generate QA pairs from KB content
- Test individual QA pairs
- Test all QA pairs at once
- CRUD for QA pairs

---

## 7. LEAD CAPTURE & MANAGEMENT

### 7.1 Three-Layer Lead Extraction
1. **Regex extraction**: Pattern matching for email, phone, name
2. **Structural analysis**: Conversation pattern detection
3. **LLM extraction**: AI-powered extraction with confidence scoring

### 7.2 Lead Fields
- `customer_name`, `customer_email`, `customer_phone` (at least one required)
- `customer_location`, `service_interest`, `detected_intent`, `urgency_level`
- `transcript_summary` (auto-generated)
- `status`: new -> contacted -> converted -> lost
- `extraction_metadata`: JSON with confidence scores, extraction method

### 7.3 Lead Features
- Conversational extraction (no forms required)
- Multi-message accumulation across turns
- Correction handling ("No, I meant john2000@email.com")
- Decline handling ("I'd rather not share" -> `declined: true`)
- Confidence scoring per field
- CSV export for CRM import
- Status management

### 7.4 Lead Policies (Defined in POLICY.MD)
- When to collect contact info (after understanding need, not first message)
- Minimum required fields
- Decline handling (graceful, no re-prompting)
- Human takeover triggers

---

## 8. CONTENT GENERATION

### 8.1 Auto-Generate KB.MD
- `POST /api/v1/agent/:slug/generate-kb` - Auto-generate annotated KB from raw content
- Uses `LLM_MODEL_REASONING` model (default: `google/gemini-2.5-pro`)

### 8.2 Auto-Generate POLICY.MD
- `POST /api/v1/agent/:slug/generate-policy` - Auto-generate annotated POLICY from raw content
- Uses `LLM_MODEL_REASONING` model

### 8.3 File Versioning
- File version history per audience/file type
- Restore previous versions
- Content hashing for change detection

---

## 9. WIDGET & EMBEDDING

### 9.1 Three Embedding Methods
| Method | Best For | Isolation |
|--------|----------|-----------|
| JavaScript Embed | General websites, full control | Low (shares DOM) |
| iframe Embed | CMS, WordPress, Shopify | High (isolated) |
| Legacy Script | Backward compatibility | Low (shares DOM) |

### 9.2 Widget Configuration Options
| Attribute | Default | Description |
|-----------|---------|-------------|
| `data-position` | `bottom-right` | Button position |
| `data-color` | `#0d9488` | Primary brand color |
| `data-welcome` | "How can we help?" | Welcome message |
| `data-company-name` | (from tenant) | Header company name |
| `data-button-size` | 56 | Toggle button size (px) |
| `data-panel-width` | 350 | Panel width (px) |
| `data-panel-height` | 500 | Panel height (px) |

### 9.3 Widget Features
- Floating chat button with expandable panel
- Session persistence via `session_id`
- Typing indicator
- Auto-scroll
- Mobile responsive (full-width on < 480px)
- Minimize/close controls
- Customizable CSS classes
- React integration support
- Global configuration via `window.AgentChatConfig`

### 9.4 Domain Security
- Per-tenant `allowed_domains` restriction
- Blank = all domains allowed

---

## 10. INTEGRATIONS

### 10.1 Webhook/Integration System
- CRUD for integrations per tenant
- Test integration endpoint
- Event listing
- SSRF protection (internal IP blocking, URL validation)
- HMAC authentication for bridge endpoints
- Cron token authentication for system triggers

### 10.2 Bridge System (Human Takeover)
- `POST /api/v1/agent/:slug/bridge/takeover` - Human takes over session
- `POST /api/v1/agent/:slug/bridge/reply` - Human sends message
- `POST /api/v1/agent/:slug/bridge/release` - Human releases back to AI
- HMAC-authenticated endpoints

### 10.3 Playground
- Demo tenant catalog with capabilities and scenarios
- Seed playground demos
- Run playground scenarios
- No-auth public access for demos

---

## 11. ADMIN UI (AgentAdmin)

### 11.1 Sections Configurable via UI
| Section | What It Does |
|---------|-------------|
| **Tenants** | List, create, edit, activate/deactivate |
| **Tenant Detail** | Upload KB/Policy/Script, configure LLM, manage users |
| **LLM Configuration** | Set model, temperature, API key per tenant |
| **User Permissions** | Grant/revoke RBAC permissions per user per tenant |
| **Role Templates** | Create, edit, assign role templates |
| **File Management** | Upload, preview, delete, restore versions |
| **Script Management** | Upload/replace/delete SCRIPT.MD |
| **RAG Management** | Rebuild index, RAG debug search, processing options |
| **QA Pairs** | Generate, test, CRUD for QA pairs |
| **Transcripts** | View, delete conversation transcripts |
| **Leads** | View, filter, status management, CSV export |
| **Tenant Settings** | Per-tenant configurable settings |
| **Integrations** | Webhook/integration management |

### 11.2 Tenant Configuration Fields
- Slug, Company Name, Vertical, GUID
- LLM Model (per-tenant override)
- Temperature (0.0-1.0)
- Is Active toggle
- Allowed Domains (JSON array)
- Processing Options (JSON, saveable as default)

---

## 12. API ENDPOINTS (Complete)

### Public (No Auth)
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/agent/:slug/chat` | Public chat |
| POST | `/api/v1/agent/:slug/chat/ext` | External chat (widget) |
| GET | `/api/v1/agent/:slug/chat/ext/transcript` | Get external transcript |
| GET | `/widget/:slug/embed.js` | JavaScript embed bundle |
| GET | `/widget/:slug/iframe` | iframe embed HTML |
| GET | `/api/v1/agent/:slug/widget.js` | Legacy widget script |
| GET | `/playground/catalog` | Demo tenant catalog |
| POST | `/api/v1/agent/:slug/playground/run` | Run playground scenario |

### Auth (Authenticated)
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/tenants` | List tenants for multi-tenant user |
| POST | `/api/v1/auth/select-tenant` | Select tenant, get JWT |
| GET | `/api/v1/agent/:slug/validate` | Validate tenant access |
| POST | `/api/v1/agent/:slug/chat/int` | Internal chat |
| GET/PUT | `/api/v1/agent/:slug/llm-config` | LLM configuration |
| GET/POST/DELETE | `/api/v1/agent/:slug/permissions` | Permission management |
| GET | `/api/v1/agent/:slug/sessions` | List chat sessions |
| GET | `/api/v1/agent/:slug/sessions/:id` | Get session detail |
| POST | `/api/v1/agent/:slug/simulate` | Start simulation |
| GET | `/api/v1/agent/:slug/simulate/:id/stream` | SSE simulation stream |
| POST | `/api/v1/agent/:slug/simulate/:id/control` | Control simulation |
| GET | `/api/v1/agent/:slug/simulations` | List simulations |
| GET | `/api/v1/agent/:slug/simulations/:id` | Get simulation |
| GET | `/api/v1/agent/:slug/conversations` | Unified conversation list |
| GET | `/api/v1/agent/:slug/conversations/:id` | Get conversation detail |
| GET/POST/DELETE | `/api/v1/agent/:slug/script` | SCRIPT.MD management |
| POST | `/api/v1/agent/:slug/analyze` | Analyze transcript |
| GET | `/api/v1/agent/:slug/analysis` | Analysis history |
| GET/POST | `/api/v1/agent/:slug/learning` | Learning memory |
| POST | `/api/v1/agent/:slug/learning/apply` | Apply learnings |
| POST | `/api/v1/agent/:slug/learning/regenerate` | Re-aggregate |
| POST | `/api/v1/agent/:slug/learning/approve` | Approve suggestion |
| POST | `/api/v1/agent/:slug/learning/dismiss` | Dismiss suggestion |
| DELETE | `/api/v1/agent/:slug/learning/behaviors/:id` | Remove behavior |
| POST | `/api/v1/agent/:slug/learning/behaviors/:id/toggle` | Toggle behavior |
| DELETE | `/api/v1/agent/:slug/learning` | Clear learning |

### Admin (Admin Auth)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/agent/tenants` | List all tenants |
| POST | `/api/v1/agent/onboard` | Onboard new tenant |
| GET | `/api/v1/agent/:slug/config` | Full tenant config |
| PATCH | `/api/v1/agent/:slug` | Update tenant |
| DELETE | `/api/v1/agent/:slug` | Delete tenant |
| POST | `/api/v1/agent/:slug/import` | Import single file |
| POST | `/api/v1/agent/:slug/reindex` | Rebuild RAG index |
| GET | `/api/v1/agent/:slug/reindex/status` | Reindex status |
| GET | `/api/v1/agent/:slug/export` | Export tenant data |
| POST | `/api/v1/agent/:slug/generate-kb` | Auto-generate KB |
| POST | `/api/v1/agent/:slug/generate-policy` | Auto-generate POLICY |
| POST | `/api/v1/agent/:slug/format-for-rag` | Preview RAG formatting |
| GET/POST | `/api/v1/agent/:slug/processing-options` | RAG processing options |
| GET | `/api/v1/agent/:slug/files/:audience/:type/versions` | File versions |
| POST | `/api/v1/agent/:slug/files/:audience/:type/restore` | Restore version |
| GET | `/api/v1/agent/:slug/source-file` | Get source file content |
| POST | `/api/v1/agent/:slug/qa-pairs/generate` | Generate QA pairs |
| GET/POST/PUT/DELETE | `/api/v1/agent/:slug/qa-pairs` | QA pair CRUD |
| POST | `/api/v1/agent/:slug/qa-pairs/:id/test` | Test QA pair |
| POST | `/api/v1/agent/:slug/qa-pairs/test-all` | Test all QA pairs |
| POST | `/api/v1/agent/:slug/rag/search` | Test RAG search |
| GET/DELETE | `/api/v1/agent/:slug/transcripts` | Transcript management |
| GET/PATCH | `/api/v1/agent/:slug/leads` | Lead management |
| GET | `/api/v1/agent/:slug/leads/export` | Export leads CSV |
| GET/PUT | `/api/v1/agent/:slug/settings` | Tenant settings |
| GET/POST/PATCH/DELETE | `/api/v1/agent/:slug/role-templates` | Role templates |
| POST | `/api/v1/agent/:slug/role-templates/:id/assign` | Assign role |
| GET/POST/PUT/DELETE | `/api/v1/agent/:slug/integrations` | Integration CRUD |
| POST | `/api/v1/agent/:slug/integrations/:id/test` | Test integration |
| GET | `/api/v1/agent/:slug/events` | List events |
| POST | `/api/v1/system/trigger-cron` | System cron trigger |

### RAG Stats (Admin Only)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/admin/rag/stats` | Global RAG statistics |
| GET | `/api/v1/admin/rag/tenants/:id` | Tenant RAG details |
| POST | `/api/v1/admin/rag/search` | Test RAG search globally |

---

## 13. ENVIRONMENT VARIABLES (Complete)

### Core
| Variable | Default | Description |
|----------|---------|-------------|
| `OPENROUTER_API_KEY` | (required) | OpenRouter API key |
| `LLM_MODEL` | `openai/gpt-4o-mini` | Default chat model |
| `LLM_MODEL_REASONING` | `google/gemini-2.5-pro` | Content generation model |
| `ENCRYPTION_MASTER_KEY` | (none) | Encrypt tenant API keys |

### RAG Pipeline
| Variable | Default | Description |
|----------|---------|-------------|
| `RAG_PIPELINE_ENABLED` | `false` | Enable RAG |
| `FORCE_REINDEX_ON_STARTUP` | `false` | Reindex all on startup |
| `RAG_STARTUP_REINDEX_DISABLED` | `false` | Skip startup reindex |

### Embeddings
| Variable | Default | Description |
|----------|---------|-------------|
| `EMBEDDING_PROVIDER` | `openrouter` | Provider: openrouter/local/mock |
| `EMBEDDING_MODEL` | varies by provider | Embedding model |
| `EMBEDDING_DIMENSION` | (auto) | Override dimensions |
| `EMBEDDING_TIMEOUT` | `180s` | API timeout |
| `EMBEDDING_BATCH_SIZE` | `10` | Chunks per batch |
| `EMBEDDING_LOCAL_ENDPOINT` | `http://localhost:8001/embed` | Local server URL |

### Vector Database
| Variable | Default | Description |
|----------|---------|-------------|
| `LANCEDB_STORAGE_PROVIDER` | `memory` | Storage: memory/local/s3 |
| `LANCEDB_LOCAL_PATH` | `build/data/lancedb` | Local disk path |
| `LANCEDB_S3_ENDPOINT` | `fly.storage.tigris.dev` | S3 endpoint |
| `LANCEDB_S3_BUCKET` | (required for s3) | S3 bucket |
| `LANCEDB_S3_REGION` | `auto` | S3 region |
| `AWS_ACCESS_KEY_ID` | (required for s3) | S3 access key |
| `AWS_SECRET_ACCESS_KEY` | (required for s3) | S3 secret key |

### Hybrid Search
| Variable | Default | Description |
|----------|---------|-------------|
| `HYBRID_SEARCH_ENABLED` | `false` | Enable hybrid search |
| `HYBRID_VECTOR_WEIGHT` | `0.7` | Vector similarity weight |
| `HYBRID_TEXT_WEIGHT` | `0.3` | BM25 keyword weight |

### Observational Memory
| Variable | Default | Description |
|----------|---------|-------------|
| `OM_ENABLED` | `true` | Enable OM |
| `OM_SCOPE` | `thread` | Scope: thread/resource |
| `OM_OBSERVER_TOKEN_THRESHOLD` | `30000` | Observer trigger threshold |
| `OM_TOKEN_THRESHOLD` | `2000` | Reflector trigger threshold |
| `OM_MESSAGE_THRESHOLD` | `10` | Deprecated (message-based trigger) |
| `OM_RETRY_ATTEMPTS` | `3` | LLM retry attempts |
| `OM_RETRY_DELAY_MS` | `1000` | Retry delay |
| `OM_BUFFER_TOKENS` | `0.2` | Buffer fraction |
| `OM_BUFFER_ACTIVATION` | `0.8` | Buffer activation point |
| `OM_BLOCK_AFTER` | `1.2` | Safety threshold multiplier |

### Hybrid OM + RAG
| Variable | Default | Description |
|----------|---------|-------------|
| `HYBRID_OM_RAG_ENABLED` | `false` | Enable hybrid OM+RAG |
| `HYBRID_OM_RAG_VECTOR_WEIGHT` | `0.7` | Vector weight |
| `HYBRID_OM_RAG_TEXT_WEIGHT` | `0.3` | Text weight |
| `HYBRID_OM_RAG_TEMPORAL_DECAY` | `0.1` | Temporal decay factor |
| `HYBRID_OM_RAG_MAX_RETRIEVED_TOKENS` | `30000` | Max retrieved tokens |
| `HYBRID_OM_RAG_MIN_SCORE` | `0.1` | Min relevance score |
| `HYBRID_OM_RAG_INDEX_OBSERVATIONS` | `true` | Index observations to RAG |
| `HYBRID_OM_RAG_COMPRESSION` | `true` | Compress for storage |
| `HYBRID_OM_RAG_TTL_DAYS` | `90` | Observation TTL |

### Verification & Other
| Variable | Default | Description |
|----------|---------|-------------|
| `LLM_VERIFIER_ENABLED` | `false` | LLM response verification |
| `CRON_TOKEN` | (none) | System cron trigger auth |
| `ADMIN_MUTATION_RATE_LIMIT_RPM` | (none) | Admin rate limit |
| `DRIVER` | `sqlite` | Database driver |

---

## 14. ANALYTICS & EVALUATION

### 14.1 Transcript Analysis
- Benchmark scoring against KB/Policy/Script
- 6-category scoring rubric (100 points)
- Issue identification with severity and turn numbers
- Improvement suggestions
- Analysis history storage

### 14.2 Agent Learning Memory
- Aggregates insights from analysis results
- Suggested behavioral improvements
- Admin approval workflow (Approve/Edit/Dismiss)
- Injected into system prompt as "LEARNED IMPROVEMENTS"
- Max 10 learned behaviors per tenant (prevent prompt bloat)

### 14.3 RAG Statistics
- Global RAG stats endpoint
- Per-tenant RAG details
- Chunk counts, index sizes, tenant counts

### 14.4 Hybrid Search Evaluation Framework
- Recall@K, Precision@K, MRR, NDCG metrics
- Query-type analysis (semantic, keyword, mixed, ambiguous)
- Performance benchmarking (latency, throughput, memory)
- Load testing support

### 14.5 Verification Metrics
- Total verifications, compliant responses
- Violations detected, corrections made
- Violations by type and severity
- Average latency tracking

---

## 15. KNOWN LIMITATIONS & GAPS

### Must-Know
1. **SCRIPT.MD not RAG-indexed** - By design, included fully in system prompt
2. **External sessions in-memory only** - Lost on server restart
3. **English only** - i18n infrastructure exists but incomplete
4. **Single LLM provider** - Only OpenRouter supported
5. **No batch simulations** - One at a time
6. **No analytics dashboard** - Per-transcript only
7. **Simulation hardcoded to "internal" audience** - Known issue with cross-audience KB fallback gap

### Nice-to-Know
1. **LanceDB on Linux** - Requires shared library (.so), not static (.a)
2. **Mock embeddings** - Semantically meaningless, pipeline testing only
3. **Hybrid search latency** - Adds ~20-50ms due to BM25 index
4. **Widget sessions** - No persistent conversation history for external users

---

## 16. MUST-KNOW vs NICE-TO-KNOW SUMMARY

### MUST-KNOW for Implementation
1. Three-file configuration model (KB.MD, POLICY.MD, SCRIPT.MD) and annotation system
2. Multi-tenant isolation architecture and tenant-scoped data patterns
3. RAG pipeline configuration (embedding providers, storage backends, hybrid search)
4. RBAC permission system and how to check/enforce permissions
5. Widget embedding methods and configuration options
6. Lead capture workflow and extraction pipeline
7. Environment variable configuration and priority system
8. All API endpoints and their auth requirements
9. Output sanitization (always active) and LLM verification (optional)
10. Observational Memory system and its configuration

### NICE-TO-KNOW
1. Evaluation framework and metrics (Recall, Precision, MRR, NDCG)
2. Agent Learning Memory (proposed, partially implemented)
3. Playground demo system
4. Integration/webhook system with SSRF protection
5. Bridge system for human takeover
6. QA pair testing
7. File versioning and restore
8. Hybrid OM+RAG temporal decay parameters
9. Role template system details
10. Verification metrics tracking and compliance rate formulas
</task_result>
</task>
```

**Tool: task**

**Input:**
```json
{
  "description": "Explore bchat onboarding requirements",
  "prompt": "Explore what information is needed to onboard a new customer/tenant to bchat:\n\n1. Read `docs/docs_biz_guide.md` thoroughly - look for the onboarding workflow\n2. Read `docs/BIZ_QUICK_START_CHECKLIST.MD` - quick start checklist\n3. Read `docs/DOCS_HOWTO_BIZ.md` - how-to business guide\n4. Read `TUTORIAL.md` - look for the \"first client\" exercise\n5. Search for any existing onboarding documentation\n6. Look at the tenant creation API and what fields are required\n7. Look at what the Admin UI requires for tenant setup\n8. Look at the simulation system - how to test before going live\n9. Look at the reindex process\n10. Look at domain configuration (custom domains, allowed domains)\n\nFocus on:\n- What information must be gathered from the customer\n- What decisions the implementation specialist must make\n- What the specialist must do in the Admin UI\n- What the specialist must do via API\n- What testing/validation steps are required before go-live\n- What ongoing maintenance is needed\n\nReturn a detailed onboarding workflow with all requirements, decisions, and steps.",
  "subagent_type": "explore"
}
```

**Output:**
```
<task id="ses_0b305bea4ffeCPEIhPdO9YXeEX" state="completed">
<task_result>
Now I have a comprehensive picture. Here is the detailed onboarding workflow:

---

# bchat Tenant Onboarding Workflow -- Complete Guide

## Overview

Onboarding a new customer/tenant to bchat involves eight phases, from initial information gathering through go-live and ongoing maintenance. The system uses a configuration-driven architecture where three markdown files (KB.MD, POLICY.MD, SCRIPT.MD) define all tenant-specific behavior -- no code changes are required per tenant.

---

## PHASE 1: Information Gathering from the Customer

This is the discovery phase. The implementation specialist must collect specific information from the customer before any technical work begins.

### 1.1 Business Identity Information (Required)

| Information Needed | Used For | Example |
|---|---|---|
| **Legal business name** | Tenant `company_name` field, KB.MD header | "Acme Pressure Washing LLC" |
| **DBA / trade name** | Agent identity if different from legal name | "Acme PW" |
| **Business vertical / industry** | Tenant `vertical` field (for internal reference only, not used in code) | "pressure_washing" |
| **Year founded** | KB.MD company overview | "2018" |
| **Owner / primary contact name** | POLICY.MD escalation contact | "John Smith" |
| **Owner phone / email** | POLICY.MD escalation contact | "(555) 123-4568" |
| **Headquarters address** | KB.MD company overview | "123 Main St, Phoenix, AZ 85001" |

### 1.2 Service Information (Required)

| Information Needed | Used For |
|---|---|
| **Complete list of services offered** | KB.MD `<!-- @service -->` annotations |
| **For each service:** description, typical causes, what is included, response time | KB.MD service entries |
| **Which services are emergency-capable** | KB.MD `@service: ..., emergency: true` annotation |
| **Complete list of services NOT offered** | KB.MD `<!-- @exclusion -->` annotations |
| **Any exceptions to exclusions** (e.g., "mold remediation only if secondary to water damage") | KB.MD `@exclusion: ..., exception: "..."` |

### 1.3 Coverage / Service Area Information (Required)

| Information Needed | Used For |
|---|---|
| **Primary service areas** (cities, ZIP codes, counties) | KB.MD `<!-- @coverage: include -->` |
| **Secondary service areas** (with travel fee notes) | KB.MD coverage section |
| **Areas explicitly NOT served** | KB.MD `<!-- @coverage: exclude -->` |

### 1.4 Contact & Emergency Information (Required)

| Information Needed | Used For |
|---|---|
| **Emergency phone number** | KB.MD emergency protocol, POLICY.MD identity |
| **Office / business phone** | KB.MD contact info |
| **Email address** | KB.MD contact info |
| **Business hours** | KB.MD FAQs, POLICY.MD identity |
| **After-hours handling instructions** | POLICY.MD identity section |
| **Emergency response protocol** (step-by-step) | KB.MD emergency protocol section |
| **Emergency response time commitment** | KB.MD `@service: ..., response_time: "60 min"` |

### 1.5 Pricing & Payment Information (Required)

| Information Needed | Used For |
|---|---|
| **Pricing structure** (factors that affect cost, NOT exact prices unless fixed) | KB.MD pricing section |
| **Whether free estimates are offered** | KB.MD FAQs |
| **Accepted payment methods** | KB.MD FAQs |
| **Any financing options** | KB.MD FAQs |

### 1.6 Licensing & Insurance (Required)

| Information Needed | Used For |
|---|---|
| **License numbers / certifications** | KB.MD company overview |
| **Insurance / bonding status** | KB.MD FAQs |
| **Warranty / guarantee terms** | KB.MD FAQs |

### 1.7 FAQs (Required -- at least 6)

The specialist must gather answers to these standard questions:

1. What are your hours?
2. Do you offer free estimates?
3. What payment methods do you accept?
4. Are you licensed and insured?
5. What is your scheduling process?
6. What is your cancellation policy?

### 1.8 Agent Behavior Preferences (Required)

| Information Needed | Used For |
|---|---|
| **Desired agent tone** (professional, casual, urgent, empathetic) | POLICY.MD `@identity` |
| **Brand voice preference** ("we" vs. "the team" vs. third-person) | POLICY.MD `@identity` |
| **Escalation contact person** (name, role, phone, email) | POLICY.MD `@identity` escalation contact |
| **Escalation trigger keywords** (e.g., "manager", "supervisor", "complain") | POLICY.MD `@rule` |
| **After-hours behavior** (collect lead + promise callback? transfer to on-call?) | POLICY.MD identity |
| **Whether agent should disclose it is AI** if asked directly | POLICY.MD rules |

### 1.9 Technical / Website Information (Required)

| Information Needed | Used For |
|---|---|
| **Customer's website domain(s)** | Tenant `allowed_domains` field (domain restriction for widget embedding) |
| **Whether customer already has a website** | Determines package (Widget Only vs. Hugo Site + Lead Desk) |
| **Website platform** (WordPress, Wix, Shopify, custom HTML) | Determines embed method (JS embed, iframe, or legacy script) |
| **Brand color** (hex code) | Widget `data-color` attribute |
| **Preferred widget position** (bottom-right or bottom-left) | Widget `data-position` attribute |

### 1.10 Package Selection Decision

The specialist must determine which package the customer needs:

| Package | Setup Fee | Monthly | Customer Profile |
|---|---|---|---|
| **Chat Widget Only** | $99-$299 | $149-$299 | Has existing website, just needs chat agent |
| **Hugo Site + AI Lead Desk** | $399-$999 | $199-$499 | No website or bad website |
| **Managed Lead Desk** | $499-$1,500 | $499-$1,500 | Wants white-glove service with CRM integration |

---

## PHASE 2: Decisions the Implementation Specialist Must Make

Before creating the tenant, the specialist must make several technical decisions:

### 2.1 Tenant Slug

- Must be URL-safe (lowercase, alphanumeric + hyphens only)
- Used in widget embed URL, API routes, and as unique identifier
- Pattern: `company-name-vertical` or just `company-name`
- Example: `acme-pressure-washing`
- The system validates: `^[a-z0-9-]+$` (enforced in Admin UI frontend)

### 2.2 LLM Model Selection

The specialist decides which LLM model to use for the tenant:

| Model | Cost | Quality | Best For |
|---|---|---|---|
| `openai/gpt-4o-mini` | Low | Good | Most service businesses (default recommendation) |
| `openai/gpt-4o` | Medium | High | Complex multi-vertical businesses |
| `google/gemini-2.5-pro` | Medium | High | Reasoning-heavy tasks |
| `anthropic/claude-3.5-sonnet` | Medium | High | Nuanced conversations |

**Note:** The model can be overridden per-tenant via the `SetLLMConfig` API or Admin UI. Falls back to the `LLM_MODEL` environment variable.

### 2.3 Temperature Setting

| Vertical | Recommended Temperature | Rationale |
|---|---|---|
| Service businesses (plumbing, restoration, cleaning) | 0.3-0.5 | Factual, consistent answers |
| Creative / marketing businesses | 0.5-0.7 | More varied responses |
| Healthcare / legal / compliance | 0.2-0.3 | Maximum factual accuracy |

**Anti-pattern:** Never set temperature above 0.8 -- the agent becomes creative and hallucinates pricing or promises.

### 2.4 Domain Configuration Decision

- **Blank / empty:** Widget works on any domain (least secure, acceptable for testing)
- **Specific domains:** Restrict to customer's domains (production recommendation)
- Supports wildcard prefixes: `*.example.com` matches all subdomains
- Stored as JSON array in `tenant.allowed_domains`
- Example: `["acmepw.com", "www.acmepw.com", "acmepw.net"]`

### 2.5 Audience Configuration

The system supports two audience types per tenant:
- **external** -- For public website visitors (the chat widget)
- **internal** -- For staff/employees (optional, separate KB/Policy)

Most onboarding uses only the `external` audience.

---

## PHASE 3: What the Specialist Must Do in the Admin UI

### 3.1 Create the Tenant

**Admin UI path:** `/agent-admin` -> Tenants page -> Create Tenant

**Form fields required:**

| Field | Required | Description |
|---|---|---|
| Tenant Slug | Yes | URL-safe identifier (e.g., `acme-pressure-washing`) |
| Company Name | Yes | Display name shown in chat header |
| Vertical | No | Internal reference (e.g., `pressure_washing`) |
| External KB File | No at creation | Can upload KB.MD during creation or later |
| External Policy File | No at creation | Can upload POLICY.MD during creation or later |
| Internal KB File | No | For staff-facing agent (optional) |
| Internal Policy File | No | For staff-facing agent (optional) |

**Key behavior:** If KB.MD is uploaded during creation and company_name was left blank, the system auto-extracts the company name from the KB content.

**API equivalent:**
```bash
curl -X POST http://localhost:5230/api/v1/agent/onboard \
  -F "tenant_slug=acme-pressure-washing" \
  -F "company_name=Acme Pressure Washing" \
  -F "vertical=pressure_washing" \
  -F "external_kb_file=@KB.MD" \
  -F "external_policy_file=@POLICY.MD"
```

### 3.2 Configure LLM Settings

**Admin UI path:** Tenant detail page -> LLM Configuration section

| Setting | Description | Default |
|---|---|---|
| LLM Model | Primary chat model | Falls back to `LLM_MODEL` env var |
| Simulation Human Model | Model used for simulated customer in tests | Falls back to env var |
| Reasoning Model | Model for content generation (KB/Policy generation) | Falls back to env var |
| OpenRouter API Key | Per-tenant API key (encrypted at rest) | Uses global key |

**API equivalent:**
```bash
curl -X PUT http://localhost:5230/api/v1/agent/acme-pressure-washing/llm-config \
  -H "Content-Type: application/json" \
  -d '{
    "llm_model": "openai/gpt-4o-mini",
    "simulation_human_model": "openai/gpt-4o-mini",
    "reasoning_model": "google/gemini-2.5-pro",
    "openrouter_api_key": "sk-or-v1-..."
  }'
```

### 3.3 Configure Domain Restrictions

**Admin UI path:** Tenant detail page -> Settings

**API equivalent:**
```bash
curl -X PATCH http://localhost:5230/api/v1/agent/acme-pressure-washing \
  -H "Content-Type: application/json" \
  -d '{
    "allowed_domains": ["acmepw.com", "www.acmepw.com"],
    "is_active": true
  }'
```

**Domain restriction behavior:**
- Checks `Origin` header for JS embed requests
- Checks both `Origin` and `Referer` headers for iframe and chat requests
- Supports wildcard: `*.example.com` matches `sub.example.com`
- Empty array `[]` = deny all (not allow all)
- Null/missing = no restrictions (allow all)
- Fail-closed: invalid JSON in `allowed_domains` denies access

### 3.4 Configure Processing Options (RAG Settings)

**Admin UI path:** Tenant detail page -> Format for RAG section

The specialist configures how content is chunked and indexed for RAG:

| Option | Description |
|---|---|
| Chunk size | Target tokens per chunk (~500 default) |
| Chunk overlap | Overlap between consecutive chunks |
| Retrieval mode | `rag` (vector search) or `long_context` (full prompt) |
| Save as Default | Persists processing options for future reindexes |

### 3.5 Upload and Configure Files

**Admin UI path:** Tenant detail page -> File upload section

Files can be uploaded during creation or updated later via:

**API equivalent for single file upload:**
```bash
curl -X POST http://localhost:5230/api/v1/agent/acme-pressure-washing/import \
  -F "audience_type=external" \
  -F "file_type=kb" \
  -F "file=@KB.MD"
```

**File versioning:** Every upload creates a versioned backup. Previous versions can be restored via the Admin UI.

### 3.6 Rebuild the RAG Index (CRITICAL)

**After every file upload, the specialist MUST click "Rebuild Index" in the Admin UI.**

Uploading files stores them but does NOT automatically rebuild the RAG index. Without this step, the agent cannot search the knowledge base.

**API equivalent:**
```bash
curl -X POST http://localhost:5230/api/v1/agent/acme-pressure-washing/reindex
```

**With audience type and resume support:**
```bash
curl -X POST "http://localhost:5230/api/v1/agent/acme-pressure-washing/reindex?audience_type=all&resume=true"
```

**Check reindex status:**
```bash
curl http://localhost:5230/api/v1/agent/acme-pressure-washing/reindex/status
```

**Timing expectations:**

| File Size | Chunks Created | Index Time |
|---|---|---|
| 5 KB | ~10 | ~10 seconds |
| 20 KB | ~40 | ~30 seconds |
| 100 KB | ~200 | ~90 seconds |
| 500 KB | ~1000 | ~4 minutes |

**Error handling:** If reindex fails, the specialist should check:
- `OPENROUTER_API_KEY` is valid
- LanceDB storage path exists and is writable
- Embedding provider is available
- Use `resume=true` to continue from last checkpoint

### 3.7 Configure Transcript Recording

**API:**
```bash
curl -X PUT http://localhost:5230/api/v1/agent/acme-pressure-washing/settings \
  -H "Content-Type: application/json" \
  -d '{"record_transcripts": true}'
```

### 3.8 Assign User Permissions (if applicable)

If the customer or their team members need access:

| Permission | Description |
|---|---|
| `tenant:admin` | Full tenant management |
| `tenant:read` | View tenant configuration |
| `api:config` | Configure LLM settings, rebuild index |
| `chat:test` | Run simulations, view test chat history |
| `chat:logs` | View real chat session logs |
| `files:upload` | Upload KB/Policy/Script files |

---

## PHASE 4: What the Specialist Must Do via API

### 4.1 LLM-Assisted Content Generation (Optional but Recommended)

The system can auto-generate annotated KB.MD and POLICY.MD from raw content:

**Generate KB.MD (from already-uploaded raw content):**
```bash
curl -X POST http://localhost:5230/api/v1/agent/acme-pressure-washing/generate-kb
```

**Generate POLICY.MD (from already-uploaded raw content):**
```bash
curl -X POST http://localhost:5230/api/v1/agent/acme-pressure-washing/generate-policy
```

**Process:** Upload raw/unannotated content first, then call the generate endpoint. The LLM adds proper `<!-- @service -->`, `<!-- @intent -->`, and other annotations. The specialist reviews the generated output before finalizing.

### 4.2 SCRIPT.MD Import (Optional)

```bash
curl -X POST http://localhost:5230/api/v1/agent/acme-pressure-washing/script \
  -F "audience_type=external" \
  -F "file=@SCRIPT.MD"
```

SCRIPT.MD defines conversation stages (Opening, Discovery, Information, Qualification, Closing) but is optional -- the agent functions without it.

### 4.3 Complete Onboarding via API (One-Shot)

For scripted/automated onboarding:
```bash
curl -X POST http://localhost:5230/api/v1/agent/onboard \
  -F "tenant_slug=acme-pressure-washing" \
  -F "company_name=Acme Pressure Washing" \
  -F "vertical=pressure_washing" \
  -F "external_kb_file=@KB.MD" \
  -F "external_policy_file=@POLICY.MD"

# Then configure LLM
curl -X PUT http://localhost:5230/api/v1/agent/acme-pressure-washing/llm-config \
  -H "Content-Type: application/json" \
  -d '{"llm_model": "openai/gpt-4o-mini"}'

# Then reindex
curl -X POST http://localhost:5230/api/v1/agent/acme-pressure-washing/reindex

# Then set domain restrictions
curl -X PATCH http://localhost:5230/api/v1/agent/acme-pressure-washing \
  -H "Content-Type: application/json" \
  -d '{"allowed_domains": ["acmepw.com"], "is_active": true}'
```

**Note:** The onboard endpoint requires super admin role. The import/reindex endpoints require admin role or `files:upload` / `api:config` permissions.

---

## PHASE 5: Testing & Validation Before Go-Live

### 5.1 RAG Debug Testing

**Admin UI path:** Tenant detail -> RAG Debug

**What to test:**
- Type 10+ different queries covering services, exclusions, coverage areas, pricing, FAQs
- Verify the right chunks are retrieved with appropriate relevance scores
- Check that exclusion queries do NOT return service entries
- Verify coverage area queries return correct cities/ZIPs

**API equivalent:**
```bash
curl -X POST http://localhost:5230/api/v1/agent/acme-pressure-washing/rag/search \
  -H "Content-Type: application/json" \
  -d '{"query": "emergency water extraction in Phoenix", "limit": 5}'
```

### 5.2 Simulation Testing

**Admin UI path:** Tenant detail -> Simulations

**What to test (minimum 3 simulations):**

| Scenario | Persona | Goal | What to Verify |
|---|---|---|---|
| **Emergency** | Urgent, anxious customer with flooding | Get emergency response | Acknowledges urgency, collects name/phone/address, confirms dispatch |
| **Quote Request** | Calm customer wanting pricing | Get a quote for pressure washing | Collects contact info, explains pricing factors, does NOT quote exact price |
| **General Inquiry** | Casual browser asking about services | Learn about coverage areas | Answers from KB.MD, does NOT aggressively collect contact info |
| **Escalation** | Frustrated repeat caller | Speak to a manager | Detects escalation keywords, provides handoff, collects contact |
| **FAQ** | Visitor asking business hours | Get information | Answers correctly, does NOT create a lead |

**API equivalent:**
```bash
# Start simulation
curl -X POST http://localhost:5230/api/v1/agent/acme-pressure-washing/simulate \
  -H "Content-Type: application/json" \
  -d '{
    "initial_prompt": "Hi, I have flooding in my basement and need emergency help. My address is 123 Oak St, Phoenix.",
    "persona_hint": "Anxious homeowner with water damage emergency"
  }'

# Response returns session_id and stream_url for SSE streaming
# Monitor via: GET /api/v1/agent/:slug/simulate/:sessionId/stream
```

**Review checklist per simulation:**
- [ ] Agent asked the right qualifying questions
- [ ] Agent collected name, email/phone correctly
- [ ] Agent handled urgency appropriately
- [ ] Agent did NOT hallucinate information
- [ ] Agent followed POLICY.MD rules (tone, escalation, fallback)
- [ ] Agent said "I don't have that information" for unknowns (not guessed)

### 5.3 Widget Embed Testing

**Three embedding options:**

**Option 1 -- JavaScript Embed (recommended):**
```html
<script
  src="https://your-server.com/widget/acme-pressure-washing/embed.js"
  data-position="bottom-right"
  data-color="#0d9488"
  data-welcome="Hi! How can we help with your pressure washing needs today?"
></script>
```

**Option 2 -- iframe Embed (for WordPress/Wix/Shopify):**
```html
<iframe
  src="https://your-server.com/widget/acme-pressure-washing/iframe?color=%230d9488"
  style="position:fixed;bottom:0;right:0;width:400px;height:600px;border:none;z-index:9999;"
  title="Chat with Acme Pressure Washing"
></iframe>
```

**Option 3 -- Legacy Script:**
```html
<script src="https://your-server.com/api/v1/agent/acme-pressure-washing/widget.js"></script>
```

**Testing checklist:**
- [ ] Widget loads without console errors
- [ ] Chat bubble appears in correct position
- [ ] Chat panel opens and expands correctly
- [ ] Initial greeting message displays
- [ ] Messages send and receive (SSE streaming works)
- [ ] Widget works on mobile viewport
- [ ] Domain restrictions block unauthorized origins
- [ ] Tenant is `is_active: true`

### 5.4 Lead Capture Testing

**Test with known inputs:**
```
"My name is John Smith, my email is john@example.com, my phone is 555-0123"
```

**Verify in database:**
```bash
sqlite3 build/data/memos_dev.db "SELECT customer_name, customer_email, customer_phone, service_interest, urgency_level FROM agent_leads ORDER BY id DESC LIMIT 5;"
```

**What to verify:**
- [ ] Name extracted correctly
- [ ] Email extracted correctly
- [ ] Phone extracted correctly
- [ ] Service interest inferred from conversation
- [ ] Urgency level scored appropriately
- [ ] Lead shows in Admin UI under the correct tenant

### 5.5 Escalation Workflow Testing

**Test input:** "I want to speak to a manager. This is unacceptable."

**Expected behavior:**
- Agent detects escalation keywords
- Agent acknowledges frustration
- Agent collects contact info
- Agent confirms handoff to escalation contact
- Agent pauses chat

### 5.6 "Say Unknown" Behavior Testing

**Test input:** Ask about a service NOT in the KB.MD.

**Expected behavior:**
- Agent says "I don't have that information in my knowledge base"
- Agent offers to collect contact info for follow-up
- Agent does NOT guess or hallucinate

---

## PHASE 6: Go-Live Checklist

### 6.1 Pre-Launch Verification

- [ ] KB.MD reviewed by business owner for accuracy (not just copy-pasted from template)
- [ ] POLICY.MD covers: identity, escalation, urgency thresholds, fallback behavior
- [ ] SCRIPT.MD defines clear conversation stages (if used)
- [ ] All phone numbers, emails, and service areas verified
- [ ] RAG index rebuilt after final file upload
- [ ] At least 3 simulations run and passed
- [ ] RAG debug returns correct chunks for 10+ test queries
- [ ] Lead capture tested and verified
- [ ] Escalation workflow tested
- [ ] Widget embedded on staging/test website
- [ ] Widget works on mobile
- [ ] Domain restrictions configured and tested
- [ ] `is_active` set to `true`
- [ ] Business owner has admin access and knows how to view leads

### 6.2 Widget Deployment

Provide the customer with the embed code appropriate for their website platform:

- **Custom HTML:** JavaScript embed (Option 1)
- **WordPress:** iframe embed (Option 2) or plugin
- **Wix/Shopify:** iframe embed (Option 2)
- **Squarespace:** iframe embed (Option 2)

### 6.3 Legal / Compliance

Add recommended legal language near the chat widget on the customer's website:

> By using this chat, you agree that [Business Name] may collect your name, email, and phone number for following up on your inquiry. We do not share your information with third parties. See our Privacy Policy for full details.

Ensure the customer's privacy policy includes a section about chat data collection.

---

## PHASE 7: Ongoing Maintenance

### 7.1 Regular Tasks

| Task | Frequency | Who |
|---|---|---|
| Review new leads | Daily | Business owner |
| Update KB.MD for new services or changed hours | As needed | Specialist or business owner |
| Rebuild RAG index after KB/Policy changes | After every file update | Specialist |
| Review simulation quality | Monthly | Specialist |
| Check RAG search accuracy | Monthly | Specialist |
| Review agent conversations for quality | Monthly | Specialist |

### 7.2 Content Update Workflow

When the customer's business information changes:

1. Get latest version from database:
   ```bash
   sqlite3 build/data/memos_dev.db "SELECT content FROM agent_source_files WHERE tenant_id = <ID> AND file_type = 'kb' ORDER BY id DESC LIMIT 1;"
   ```
2. Save with epoch suffix: `KB_1768964222.MD`
3. Make surgical, targeted changes only
4. Save updated draft to `*_DRAFT.MD`
5. Upload via Admin UI or API
6. **Rebuild RAG index** (critical step)
7. Run a simulation to verify changes work correctly

### 7.3 Scaling Considerations

| Concern | Threshold | Action |
|---|---|---|
| Lead volume exceeds expectations | >100 leads/day | Review urgency thresholds, adjust POLICY.MD |
| Agent hallucinating | Any occurrence | Lower temperature, add fallback rules, rebuild index |
| RAG search returning wrong results | Visible in RAG debug | Update KB.MD annotations, rebuild index |
| Widget slow to respond | >5 seconds | Check LLM model choice, consider cheaper/faster model |
| High chat volume increasing costs | >$50/month per tenant | Switch to `gpt-4o-mini` or reduce context window |

---

## API Endpoints Summary

### Public (No Auth)
| Method | Path | Purpose |
|---|---|---|
| POST | `/api/v1/agent/:slug/chat` | Send chat message |
| GET | `/api/v1/agent/:slug/chat/stream` | SSE stream |
| GET | `/widget/:slug/embed.js` | JavaScript embed |
| GET | `/widget/:slug/iframe` | iframe embed |
| GET | `/api/v1/agent/:slug/widget.js` | Legacy widget script |

### Admin (Authenticated)
| Method | Path | Permission | Purpose |
|---|---|---|---|
| POST | `/api/v1/agent/onboard` | super_admin | Create tenant with files |
| PATCH | `/api/v1/agent/:slug` | tenant:write | Update tenant settings |
| POST | `/api/v1/agent/:slug/import` | files:upload | Upload single file |
| POST | `/api/v1/agent/:slug/reindex` | api:config | Rebuild RAG index |
| GET | `/api/v1/agent/:slug/reindex/status` | api:config | Check reindex status |
| PUT | `/api/v1/agent/:slug/llm-config` | api:config | Set LLM configuration |
| PUT | `/api/v1/agent/:slug/settings` | tenant:write | Update settings |
| POST | `/api/v1/agent/:slug/generate-kb` | admin | Auto-generate KB.MD |
| POST | `/api/v1/agent/:slug/generate-policy` | admin | Auto-generate POLICY.MD |
| POST | `/api/v1/agent/:slug/simulate` | chat:test | Start simulation |
| GET | `/api/v1/agent/:slug/simulate/:sessionId/stream` | chat:test | SSE stream for simulation |
| POST | `/api/v1/agent/:slug/script` | files:upload | Import SCRIPT.MD |

---

## Key Anti-Patterns to Avoid

| Anti-Pattern | Why It Is Bad | Correct Approach |
|---|---|---|
| Copy-pasting KB.MD from another tenant | Wrong services, pricing, coverage | Write from scratch per business |
| Skipping RAG rebuild after upload | Agent answers from stale/missing knowledge | Always rebuild index |
| Temperature above 0.8 | Agent hallucinates pricing and promises | Use 0.3-0.5 for service businesses |
| No exclusions section | Agent promises services not offered | Always list what is NOT included |
| Skipping simulation testing | Untested blind spots | Run minimum 3 simulations |
| Leaving placeholder text in KB.MD | Agent says "[Your Company Name]" | Remove all placeholders |
| Not reviewing leads daily | Leads go cold | Set daily reminder |
| Wrong slug in embed URL | Agent uses wrong knowledge base | Double-check slug matches tenant |
</task_result>
</task>
```

---

## User

read all TUTORIAL* files for context

---

## Assistant (Plan · MiMo V2.5 Free · 12.2s)

_Thinking:_

The user wants me to read all TUTORIAL* files for context before planning the DOCS_ONBOARDING.md. Let me find and read all tutorial-related files.

**Tool: glob**

**Input:**
```json
{
  "pattern": "TUTORIAL*"
}
```

**Output:**
```
/home/chaschel/Documents/go/bchat/TUTORIAL.md
/home/chaschel/Documents/go/bchat/TUTORIAL_DATABASE.md
/home/chaschel/Documents/go/bchat/TUTORIAL_MIMO.md
/home/chaschel/Documents/go/bchat/TUTORIAL_RAG.md
/home/chaschel/Documents/go/bchat/TUTORIAL_CX.md
```

**Tool: glob**

**Input:**
```json
{
  "pattern": "**/TUTORIAL*"
}
```

**Output:**
```
/home/chaschel/Documents/go/bchat/TUTORIAL_DATABASE.md
/home/chaschel/Documents/go/bchat/TUTORIAL_CX.md
/home/chaschel/Documents/go/bchat/TUTORIAL_MIMO.md
/home/chaschel/Documents/go/bchat/TUTORIAL_RAG.md
/home/chaschel/Documents/go/bchat/TUTORIAL.md
```

---

## Assistant (Plan · MiMo V2.5 Free · 5.2s)

_Thinking:_

Let me read all the tutorial files to understand the existing content and context.

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/TUTORIAL.md"
}
```

**Output:**
```
<path>/home/chaschel/Documents/go/bchat/TUTORIAL.md</path>
<type>file</type>
<content>
1: # Shipping Production Software: A Full-Stack Case Study in Critical Thinking
2: 
3: > **Case Study:** Native Webhook Integrations v1 (bugs/033)
4: > **Repository:** bchat — Multi-tenant AI Chat Agent Platform
5: > **Audience:** Intermediate developers seeking production-grade practices
6: > **Core Thesis:** *Do not outsource thinking to coding agents.*
7: 
8: ---
9: 
10: ## How to Read This Tutorial
11: 
12: Each chapter follows the same pattern:
13: 
14: 1. **Context** — the business or technical situation
15: 2. **The Story from bugs/033** — what actually happened in this real project
16: 3. **Critical Thinking Exercise** — questions you must answer *before* reading further
17: 4. **Analysis** — how things played out, what went wrong, what went right
18: 5. **Your Turn** — exercises to apply the lesson to your own work
19: 
20: The goal is not to teach you Go or React syntax. The goal is to teach you how to *think* about software — how to ask better questions, make better trade-offs, and catch mistakes before they reach production.
21: 
22: ---
23: 
24: ## Reference: bugs/033 at a Glance
25: 
26: | Dimension | Detail |
27: |-----------|--------|
28: | **Feature** | Native webhook integrations — sends real-time events (lead.captured, etc.) to external systems |
29: | **Business Driver** | Replace $246/mo in third-party SaaS (Intercom AI + Helios AI + n8n) |
30: | **Files Touched** | 22 files across migrations, store, service, API, frontend, Docker, deployment |
31: | **Lines of Code** | ~2,800 (532 Go service, 131 Go tests, 1,763 frontend, 296 component, 85 resilience) |
32: | **Code Review Verdict** | REWORK — 2 critical, 2 major, 3 moderate, 4 nits |
33: | **Deployment Status** | Scaffolded, migrations committed, never stress-tested in production |
34: | **Files to Study** | `bugs/033/` directory, `store/migration/sqlite/0.31/`, `store/migration/postgres/0.31/`, `server/router/api/v1/agent/integrations.go`, `server/router/api/v1/agent/service.go` |
35: 
36: ---
37: 
38: # PART I: THE BUSINESS LENS
39: 
40: ## Chapter 1: The Product Discovery Process
41: 
42: ### Context
43: 
44: Before a single line of code is written, someone has to decide *what to build*. This seems obvious, but it is the most frequently skipped step in software engineering. Engineers love to code. They love to solve technical problems. They are far less enthusiastic about sitting in a room and asking "should we even do this?"
45: 
46: The bchat platform is a multi-tenant AI chat agent for service businesses (plumbers, electricians, restoration companies). When a visitor chats with the agent, the agent can capture leads (name, email, phone, service needed). But leads sitting in a database are useless. The business needs them in their CRM, their SMS follow-up system, their calendar booking tool.
47: 
48: The question: how do we get leads FROM bchat TO the business's existing tools?
49: 
50: ### The Story from bugs/033
51: 
52: The original plan (`plan_biz.md`) proposed building three native integrations:
53: 
54: | Phase | Integration | Cost Replaced | Engineering Effort |
55: |-------|-------------|---------------|--------------------|
56: | P0 | Webhooks | Parts of n8n ($49/mo) | 2 weeks |
57: | P1 | SMS (Twilio) | Helios AI ($100/mo) | 3 weeks |
58: | P2 | Calendar (Google/Outlook) | Intercom AI ($97/mo) | 3 weeks |
59: 
60: Total: ~8 weeks to save $246/mo. This looks like a clear win, right?
61: 
62: But then someone asked a better question: *"Do our users actually need native Calendar and SMS, or do they need their leads to END UP in their calendar and SMS tool?"*
63: 
64: This is the critical distinction. The user's job-to-be-done is not "have a calendar integration in bchat." It's "get appointments booked." There are many ways to achieve that.
65: 
66: The debate is documented in `bugs/033/prompt.md`. The pivotal exchange:
67: 
68: > **Original thinker:** "bchat can simply act as the source of truth for events and those personal AI assistants configured with bridge platforms like Zapier can simply pull out the data from bchat instead."
69: 
70: This is the "webhook-first" strategy: instead of building integrations for every possible destination, build one generic integration (webhooks) that any destination can consume.
71: 
72: ### Critical Thinking Exercise
73: 
74: Before reading further, answer these questions:
75: 
76: 1. **What are the trade-offs between the "build native" and "webhook-first" approaches?** List at least 3 pros and 3 cons for each.
77: 
78: 2. **Who is your user?** A plumber who just wants leads to show up in their Google Calendar. A power user who already uses Zapier. A SaaS platform that embeds bchat. Do they all need the same thing?
79: 
80: 3. **What is the cost of being wrong?** If you build native integrations and nobody uses them, you've wasted 8 weeks. If you build webhooks and users can't figure out Zapier, you've shipped a useless feature.
81: 
82: 4. **How do you validate this decision before building?**
83: 
84: ### Analysis
85: 
86: The team chose the webhook-first strategy. Here is their reasoning:
87: 
88: **Arguments FOR webhook-first:**
89: - Drastically reduces codebase complexity (no OAuth flows, no timezone handling, no Twilio TCPA compliance)
90: - Infinite extensibility — one webhook event reaches Zapier, Make, n8n, Pipedream, custom receivers
91: - Keeps bchat focused on its core competency: being a chat agent platform
92: - Zapier has 5,000+ app integrations — users can connect to anything
93: 
94: **Arguments AGAINST webhook-first:**
95: - User experience friction — setting up Zapier requires technical knowledge
96: - Loss of native visibility — bchat can't show "SMS sent at 2pm" in the chat transcript
97: - Support burden — users will ask "how do I connect this to my CRM?"
98: 
99: **The compromise solution (from the prompt debate):**
100: 
101: 1. **Pre-built Zapier/Make templates** — one-click "Connect Twilio via Zapier" buttons in the admin UI
102: 2. **Inbound update API** — `PATCH /api/v1/agent/:slug/leads/:id` so external agents can write status back to bchat
103: 
104: This is the pattern of good product thinking: you don't need to choose one approach entirely. You can do the 20% of work that gives you 80% of the value (webhooks + templates + inbound API) and defer the remaining 80% of work (native SMS, native Calendar) until users actually ask for it.
105: 
106: ### The Real Outcome in bugs/033
107: 
108: Phase 1 (Webhooks) was implemented. Phase 2 (SMS) and Phase 3 (Calendar) were deferred to v1.1. The inbound update API was not yet built. The Zapier templates were planned but not yet created.
109: 
110: This is normal. The decision was correct — ship value faster, learn from real usage, then decide what to build next. But it creates a risk: if webhooks are too hard for the target user, the feature delivers zero value. That risk was accepted.
111: 
112: ### Your Turn
113: 
114: Think about a feature you built recently.
115: 
116: 1. What was the user's actual job-to-be-done?
117: 2. What alternatives did you consider before building?
118: 3. Did you build something that could have been deferred or done differently?
119: 4. What would you have learned by shipping a smaller version first?
120: 
121: ---
122: 
123: ## Chapter 2: Requirements Engineering
124: 
125: ### Context
126: 
127: Once you've decided what to build, you need to specify *precisely* what it does. Vague requirements produce vague implementations. Precise requirements produce testable code.
128: 
129: "Add webhooks" is a vague requirement. It could mean anything from "POST JSON to a URL" to "reliable at-least-once delivery with idempotency, retry, and monitoring."
130: 
131: ### The Story from bugs/033
132: 
133: The requirements for the webhook feature were captured across multiple documents. Let's trace how they evolved:
134: 
135: **Initial vague requirement (from `plan_biz.md`):**
136: > "When a visitor shares their name + email or phone during chat, bchat captures the lead and sends it to your webhook URL."
137: 
138: **Refined into specific event types:**
139: | Event | Trigger | Payload |
140: |-------|---------|---------|
141: | `lead.captured` | Lead identified in chat | Lead data + session context |
142: | `lead.updated` | Lead status changed | Lead + old/new status |
143: | `escalation.created` | Ticket created | Ticket + session + urgency |
144: | `conversation.completed` | Session ends | Session + summary + sentiment |
145: | `safety.triggered` | Emergency detected | Session + protocol |
146: 
147: **Refined into technical requirements:**
148: 
149: | Category | Requirement | Why It Matters |
150: |----------|-------------|----------------|
151: | Delivery | At-least-once delivery with deduplication | Leads cannot be lost, but duplicates cause CRM errors |
152: | Retry | Max 5 attempts, 5-minute interval | Temporary failures should not cause permanent loss |
153: | Security | HMAC-SHA256 signature | Receiver must verify the webhook came from bchat |
154: | Security | SSRF protection | Block requests to internal networks |
155: | Security | Idempotency key | Retries should not create duplicate records |
156: | Performance | <500ms delivery latency (p95) | External systems should receive leads quickly |
157: | Reliability | Survive server crash | Events in-flight at crash time must not be lost |
158: 
159: Notice how each requirement has a *reason*. Requirements without justification lead to cargo-culting: "we need idempotency" without understanding *why*. When you know why, you know when the requirement can be relaxed.
160: 
161: ### Critical Thinking Exercise
162: 
163: For each requirement above, ask:
164: 
165: 1. **What happens if we don't do this?** — If we don't have idempotency, what actually breaks? (Answer: a retried event creates a duplicate lead in the CRM. Bad, but not catastrophic.)
166: 2. **What is the minimum viable version?** — Do we need all 5 event types at launch, or just `lead.captured`? (Answer: just `lead.captured` — the others are nice-to-have.)
167: 3. **How do we test this?** — How do we verify that SSRF protection actually blocks internal IPs? (Answer: write a test that attempts to POST to 169.254.169.254 and expects failure.)
168: 4. **What is the failure mode?** — If the webhook receiver is down, what happens to the chat? (Answer: nothing — event dispatch is non-blocking. The chat succeeds, the event is queued for retry.)
169: 
170: ### The Secret Requirements (That Nobody Wrote Down)
171: 
172: Some of the most important requirements were never explicitly stated. They were discovered during the adversarial code review:
173: 
174: 1. **Context hygiene** — The HTTP request context must outlive the handler response (C2 bug)
175: 2. **Parameter ordering** — Function arguments must be passed in the correct positions (C1 bug)
176: 3. **PII redaction** — API responses must not expose sensitive data to unauthorized users (M1 bug)
177: 4. **Shell compatibility** — Crontab syntax must work with `/bin/sh` (dash), not just `/bin/bash` (M2 bug)
178: 
179: These are *implicit* requirements. They apply to every feature, every time. But because they're implicit, they're not in the requirements document. And because they're not in the document, they're the first things to break.
180: 
181: **The lesson:** Experienced engineers learn to maintain a mental checklist of implicit requirements that apply to everything they build. Context propagation. Error handling. Logging. Security. Monitoring. These are not feature-specific — they're discipline-specific.
182: 
183: ### Your Turn
184: 
185: Take a feature you're working on now.
186: 
187: 1. Write down 5 implicit requirements that apply to every feature in your system.
188: 2. For each one, describe a scenario where violating it would cause a production incident.
189: 3. How do you automatically verify these requirements (linters, tests, code review checklist)?
190: 
191: ---
192: 
193: ## Chapter 3: Architecture Decision Records (ADRs)
194: 
195: ### Context
196: 
197: Every significant system has design decisions that are worth documenting. Not because the documentation makes the code better, but because future engineers need to understand *why* things are the way they are. Without context, they will assume the current design is wrong and "fix" it — breaking things that were carefully chosen.
198: 
199: An Architecture Decision Record (ADR) captures:
200: - The **context** that prompted the decision
201: - The **options** that were considered
202: - The **decision** that was made
203: - The **rationale** for the decision
204: - The **consequences** (what trade-offs were accepted)
205: 
206: ### The Story from bugs/033
207: 
208: The bugs/033 implementation involved at least 6 significant architectural decisions. Here they are as ADRs:
209: 
210: ---
211: 
212: #### ADR-1: Webhook-First vs. Native Integrations
213: 
214: | Field | Content |
215: |-------|---------|
216: | **Context** | Need to get leads into external systems (CRM, SMS, Calendar) |
217: | **Options** | A: Build native Twilio + Google Calendar integrations. B: Build webhook events + Zapier templates |
218: | **Decision** | B — Webhook-first |
219: | **Rationale** | 80% less code, infinite extensibility, keeps bchat focused |
220: | **Consequences** | Users need Zapier proficiency. bchat loses visibility into downstream actions. Inbound update API needed later. |
221: 
222: ---
223: 
224: #### ADR-2: Supercronic (External Process) vs. In-Process Cron
225: 
226: | Field | Content |
227: |-------|---------|
228: | **Context** | Need to poll for failed event deliveries every 5 minutes |
229: | **Options** | A: In-process Go cron (e.g., `robfig/cron`). B: External cron process (supercronic) |
230: | **Decision** | B — Supercronic |
231: | **Rationale** | Container-native (SIGTERM-aware). Standard cron syntax. Existing entrypoint pattern already separates concerns. Matches Fly.io scale-to-zero model (cron only runs while machine is awake). |
232: | **Consequences** | External dependency added to Docker image. Another process to manage. Shell compatibility matters (M2 bug). |
233: 
234: ---
235: 
236: #### ADR-3: Pre-Claimed Events vs. Two-Phase Insert
237: 
238: | Field | Content |
239: |-------|---------|
240: | **Context** | Event must be delivered at least once. Insert + immediate delivery creates a window between insert and delivery where a crash loses the event. |
241: | **Options** | A: Two-phase — insert as 'pending', then mark as 'processing' when delivery starts. B: Pre-claimed — insert as 'processing' with `claimed_at=now` |
242: | **Decision** | B — Pre-claimed |
243: | **Rationale** | Eliminates the crash-loss window. The event is "claimed" from the moment it exists. If the deliverer crashes, the poller reclaims after 5-minute lease expiry. |
244: | **Consequences** | Status `DEFAULT 'processing'` is counterintuitive for future readers. Documented in SQL comment. |
245: 
246: ---
247: 
248: #### ADR-4: Single-Writer Assumption (SQLite)
249: 
250: | Field | Content |
251: |-------|---------|
252: | **Context** | SQLite is the default database. It has no `FOR UPDATE SKIP LOCKED`. |
253: | **Options** | A: Use `UPDATE ... WHERE id IN (SELECT ... LIMIT ?)` without locking. B: Use application-level mutex. |
254: | **Decision** | A — Optimistic without locking |
255: | **Rationale** | Single-writer at v1 scale. Acceptable trade-off. Document limitation. |
256: | **Consequences** | With 1,000+ pending rows, the `SELECT ... LIMIT 10` inside `UPDATE` acquires a write lock on the whole table. Concurrent chat writes (lead capture, status updates) could contend. |
257: 
258: ---
259: 
260: #### ADR-5: Idempotency Key = hash(tenantID, leadID, eventType)
261: 
262: | Field | Content |
263: |-------|---------|
264: | **Context** | At-least-once delivery + retries means the same event could be delivered twice. Need deduplication. |
265: | **Options** | A: Random UUID as idempotency key. B: Deterministic hash of event content. C: Deterministic hash of (tenantID, leadID, eventType). |
266: | **Decision** | C — Deterministic hash of identifying fields |
267: | **Rationale** | Same logical event always produces the same key. The UNIQUE constraint prevents duplicate inserts. Random UUID would allow duplicates on retry. |
268: | **Consequences** | **C1 bug:** the implementation swapped argument order, passing `(tenantID, eventType, integrationID)` instead of `(tenantID, leadID, eventType)`. This made every event for a given integration produce the same idempotency key. Only one event ever dispatched. |
269: 
270: ---
271: 
272: #### ADR-6: Expose PII in Event Payload vs. Reference-Only
273: 
274: | Field | Content |
275: |-------|---------|
276: | **Context** | Webhook receivers need lead data. But the events API returns full payload to admin users. |
277: | **Options** | A: Include full lead data (name, email, phone, session_id) in the payload. B: Include only a reference ID. Receiver must call back to get details. |
278: | **Decision** | A — Full payload |
279: | **Rationale** | Webhook receivers need the data immediately. A call-back pattern adds latency and complexity. The events API is admin-only. |
280: | **Consequences** | **M1 bug:** `HandleListEvents` returns raw `AgentEvent` including `Payload` field. Admin workspaces with non-admin users leak PII. The integration list handler correctly masks config; the event list handler does not. |
281: 
282: ### Critical Thinking Exercise
283: 
284: For each ADR above:
285: 
286: 1. **Can you think of a counterargument the team missed?**
287: 2. **Under what conditions would you reverse the decision?**
288: 3. **How would the decision change if the feature needed to handle 10x the traffic?**
289: 
290: ### The Meta-Lesson About ADRs
291: 
292: Notice that the implementation committed — the *actual code* — contained a critical bug that violated ADR-5 (idempotency key) and a privacy violation that contradicted the intent of ADR-6.
293: 
294: **The ADR documented the correct design. The code implemented a buggy version of it.**
295: 
296: This is the gap between design and implementation. ADRs don't prevent bugs. They give reviewers a reference point to compare against. The adversarial code review found C1 precisely because it compared the code behavior against the documented intent.
297: 
298: ### Your Turn
299: 
300: For your current project:
301: 
302: 1. Identify 3 architectural decisions that are not documented but should be.
303: 2. Write one ADR for each using the template above.
304: 3. Ask yourself: "If a new developer joined today, would they understand why these decisions were made?"
305: 
306: ---
307: 
308: # PART II: THE IMPLEMENTATION LENS
309: 
310: ## Chapter 4: Database Schema Design
311: 
312: ### Context
313: 
314: The database schema is the foundation of your system. Bad schema decisions propagate upward through every layer — they make queries slow, migrations painful, and features expensive to add.
315: 
316: In a multi-database system like bchat (SQLite for development, Postgres for production, MySQL for legacy), every schema decision must be evaluated across three dialects simultaneously.
317: 
318: ### The Story from bugs/033
319: 
320: Two new tables were needed: `agent_integrations` (webhook configuration) and `agent_events` (outbox queue).
321: 
322: #### Table 1: agent_integrations
323: 
324: ```sql
325: -- SQLite version
326: CREATE TABLE IF NOT EXISTS agent_integrations (
327:     id INTEGER PRIMARY KEY AUTOINCREMENT,
328:     tenant_id INTEGER NOT NULL,
329:     integration_type TEXT NOT NULL,          -- 'webhook' | 'twilio'
330:     label TEXT NOT NULL DEFAULT '',
331:     config TEXT NOT NULL DEFAULT '{}',       -- JSON: url, secret, headers
332:     is_active INTEGER NOT NULL DEFAULT 1,
333:     created_at BIGINT NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER)),
334:     updated_at BIGINT NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER)),
335:     FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
336: );
337: ```
338: 
339: ```sql
340: -- Postgres version
341: CREATE TABLE IF NOT EXISTS agent_integrations (
342:     id SERIAL PRIMARY KEY,
343:     tenant_id INTEGER NOT NULL,
344:     integration_type TEXT NOT NULL,
345:     label TEXT NOT NULL DEFAULT '',
346:     config TEXT NOT NULL DEFAULT '{}',
347:     is_active BOOLEAN NOT NULL DEFAULT TRUE,
348:     created_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
349:     updated_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
350:     FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
351: );
352: ```
353: 
354: #### Key Design Decisions
355: 
356: **1. JSON config column instead of normalized fields.**
357: 
358: Instead of separate columns for `url`, `secret`, `headers`, etc., the design uses a single JSON `config` column. This is a trade-off:
359: 
360: | For | Against |
361: |-----|---------|
362: | Adding new fields doesn't require migrations | No type validation at the database level |
363: | Different integration types (webhook vs. Twilio) can have different config shapes | Cannot query by config field efficiently |
364: | Simple implementation | Tooling (pgAdmin, etc.) shows blob, not structured data |
365: 
366: For v1, this was a reasonable choice. The `config` is opaque to the database — it's read, parsed, and used entirely in application code. As the feature matures, specific fields might migrate to dedicated columns.
367: 
368: **2. `INTEGER PRIMARY KEY AUTOINCREMENT` vs. `SERIAL`.**
369: 
370: SQLite uses `INTEGER PRIMARY KEY AUTOINCREMENT`. Postgres uses `SERIAL`. These produce the same logical behavior (auto-incrementing integer IDs) but through different mechanisms. The migration files must keep both versions in sync.
371: 
372: **3. `strftime` vs. `EXTRACT` for Unix timestamps.**
373: 
374: SQLite: `CAST(strftime('%s','now') AS INTEGER)` gets the current Unix timestamp.
375: Postgres: `EXTRACT(EPOCH FROM NOW())::BIGINT` does the same.
376: 
377: Different syntax, same result. The migration system (`store/migrator.go`) uses `profile.Driver` to select the correct migration directory (`store/migration/sqlite/` vs. `store/migration/postgres/`).
378: 
379: #### Table 2: agent_events (The Outbox Queue)
380: 
381: ```sql
382: -- NOTE: status DEFAULT 'processing' is intentional — every insert path pre-claims.
383: -- A brand-new row is always "in-flight" because the insertor dispatches immediately.
384: CREATE TABLE IF NOT EXISTS agent_events (
385:     id INTEGER PRIMARY KEY AUTOINCREMENT,
386:     tenant_id INTEGER NOT NULL,
387:     integration_id INTEGER NOT NULL,
388:     event_type TEXT NOT NULL,                -- 'lead.captured'
389:     payload TEXT NOT NULL DEFAULT '{}',      -- JSON payload
390:     status TEXT NOT NULL DEFAULT 'processing',
391:     claimed_at BIGINT DEFAULT NULL,
392:     attempts INTEGER NOT NULL DEFAULT 0,
393:     last_error TEXT DEFAULT NULL,
394:     idempotency_key TEXT UNIQUE,             -- hash(tenantID, leadID, eventType)
395:     created_at BIGINT NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER)),
396:     FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE,
397:     FOREIGN KEY (integration_id) REFERENCES agent_integrations(id) ON DELETE CASCADE
398: );
399: ```
400: 
401: The critical design choices here:
402: 
403: **Status lifecycle:**
404: ```
405: insert → 'processing' (pre-claimed)
406:               ↓
407:          delivery goroutine → success → 'delivered'
408:               ↓
409:               failure → status stays 'processing', attempts incremented
410:               ↓
411:          poller reclaims (after 5 min lease expiry) → retry
412:               ↓
413:          max attempts reached → 'failed' (no more retries)
414: ```
415: 
416: **The pre-claim pattern (ADR-3):** status defaults to `processing`, not `pending`. This is unusual. The reasoning is that the insertion path *immediately* spawns a delivery goroutine. If the goroutine succeeds, the event is `delivered`. If it crashes, the event stays `processing` with a `claimed_at` timestamp. The poller finds events where `claimed_at < now - 300s` and reclaims them.
417: 
418: **The idempotency key:** `UNIQUE` constraint on `(tenantID, leadID, eventType)`. This is the deduplication mechanism. If a retry tries to insert the same logical event, the constraint violation is caught and treated as "already exists."
419: 
420: ### Critical Thinking Exercise
421: 
422: 1. **The `config` column is JSON.** What happens when a migration needs to rename a field inside the JSON? (Answer: you need a data migration to update all existing rows. There is no SQL `ALTER TABLE` for JSON fields.)
423: 
424: 2. **The `payload` column stores full lead PII.** How would you handle GDPR deletion requests? (Answer: you'd need to either anonymize the payload or delete the row entirely. Neither is supported by the current schema.)
425: 
426: 3. **The `idempotency_key` is `TEXT UNIQUE`.** In SQLite, a UNIQUE constraint allows multiple NULL values. In Postgres (by default), it does too. But the design intends this to always be non-NULL. What happens if a future code path forgets to set it? (Answer: no deduplication for that path. Multiple identical events could be inserted.)
427: 
428: 4. **SQLite has no `BOOLEAN` type.** It uses 0/1 in `INTEGER` columns. Postgres has native `BOOLEAN`. The SQLite migration uses `INTEGER NOT NULL DEFAULT 1` for `is_active`. The Postgres one uses `BOOLEAN NOT NULL DEFAULT TRUE`. Are these equivalent in all query contexts? (Answer: mostly yes, but `WHERE is_active` works in Postgres and `WHERE is_active = 1` works in SQLite. Inconsistencies in the Go code could cause dialect-specific bugs.)
429: 
430: ---
431: 
432: ## Chapter 5: The Store Layer
433: 
434: ### Context
435: 
436: The store layer is bchat's database abstraction. It sits between the HTTP handlers and the raw SQL, providing a clean Go interface that can be implemented against SQLite, Postgres, or MySQL.
437: 
438: For a deeper look at the complete store architecture, see `store/driver.go` (283 methods across all entities) and `store/store.go` (the wrapper with caching).
439: 
440: ### The Story from bugs/033
441: 
442: The integration feature added 9 methods to the `store.Driver` interface:
443: 
444: ```go
445: // Interface methods (store/driver.go)
446: CreateAgentIntegration(ctx, integration) (*AgentIntegration, error)
447: GetAgentIntegration(ctx, find *FindAgentIntegration) (*AgentIntegration, error)
448: ListAgentIntegrations(ctx, find *FindAgentIntegration) ([]*AgentIntegration, error)
449: UpdateAgentIntegration(ctx, update *AgentIntegration) error
450: DeleteAgentIntegration(ctx, id int32) error
451: 
452: CreateAgentEvent(ctx, event *AgentEvent) (*AgentEvent, error)
453: ListAgentEvents(ctx, find *FindAgentEvent) ([]*AgentEvent, error)
454: ClaimPendingEvents(ctx, limit int32) ([]*AgentEvent, error)
455: UpdateAgentEvent(ctx, update *AgentEvent) error
456: ```
457: 
458: Each method needs three implementations: `store/db/sqlite/agent.go`, `store/db/postgres/agent.go`, and MySQL stubs.
459: 
460: #### The Claim Query — SQLite vs. Postgres
461: 
462: This is where the database differences matter most.
463: 
464: **SQLite (single-writer, no concurrent access):**
465: 
466: ```sql
467: UPDATE agent_events
468: SET status = 'processing', claimed_at = CAST(strftime('%s','now') AS INTEGER), attempts = attempts + 1
469: WHERE id IN (
470:     SELECT id FROM agent_events
471:     WHERE (status = 'pending' AND attempts < 5)
472:        OR (status = 'processing' AND claimed_at < CAST(strftime('%s','now') AS INTEGER) - 300 AND attempts < 5)
473:     LIMIT ?
474: )
475: RETURNING *
476: ```
477: 
478: This updates up to `?` rows (the limit parameter) that are either:
479: - `pending` with fewer than 5 attempts (new events), OR
480: - `processing` with a claimed_at older than 300 seconds and fewer than 5 attempts (stale claims eligible for reclaim)
481: 
482: The `LIMIT` in the subquery limits how many rows the single-writer claims per poll cycle.
483: 
484: **Postgres (concurrent, with SKIP LOCKED):**
485: 
486: ```sql
487: UPDATE agent_events
488: SET status = 'processing', claimed_at = EXTRACT(EPOCH FROM NOW())::BIGINT, attempts = attempts + 1
489: WHERE id IN (
490:     SELECT id FROM agent_events
491:     WHERE (status = 'pending' AND attempts < 5)
492:        OR (status = 'processing' AND claimed_at < EXTRACT(EPOCH FROM NOW())::BIGINT - 300 AND attempts < 5)
493:     FOR UPDATE SKIP LOCKED
494:     LIMIT $1
495: )
496: RETURNING *
497: ```
498: 
499: The difference: `FOR UPDATE SKIP LOCKED`. In Postgres, multiple pollers can run concurrently. Each poller locks the rows it selects and skips rows locked by others. Without `SKIP LOCKED`, pollers would block each other, creating a thundering herd problem.
500: 
501: SQLite doesn't have `SKIP LOCKED`. It doesn't need it — the database is a single file with a single writer at any moment. But this means the SQLite implementation cannot scale to multiple concurrent pollers.
502: 
503: #### MySQL Stubs
504: 
505: MySQL is explicitly not supported for this feature:
506: 
507: ```go
508: func (d *DB) CreateAgentIntegration(ctx context.Context, integration *store.AgentIntegration) (*store.AgentIntegration, error) {
509:     return nil, errNotImplemented
510: }
511: ```
512: 
513: This is a deliberate design choice. The MySQL driver compiles but panics at runtime if called. This is better than not compiling at all (which would require build tags to exclude MySQL from the build).
514: 
515: #### Postgres Resilience (`store/db/postgres/resilience.go`)
516: 
517: This file implements exponential backoff for transient Postgres errors:
518: 
519: ```go
520: func isTransientError(err error) bool {
521:     // SQLSTATE 57P01 (admin_shutdown)
522:     // SQLSTATE 08006 (connection_failure)
523:     // SQLSTATE 08003 (connection_does_not_exist)
524: }
525: 
526: func RunResiliently(fn func() error) error {
527:     // Exponential backoff: 1s, 2s, 4s, 8s, 16s + jitter
528:     // Treat 23505 (unique_violation) as success (idempotency)
529: }
530: ```
531: 
532: This is an ops-first concern: managed Postgres services (Neon, RDS, Cloud SQL) occasionally restart for maintenance. The application must survive brief connection drops. Most applications don't handle this — they crash on the first transient error and need a restart.
533: 
534: The `23505` (unique_violation) handling is particularly important. When an idempotency key collides, the `INSERT` fails with a duplicate key error. `RunResiliently` treats this as success — the event already exists, so there's nothing to do. Without this, idempotent inserts would fail on retry.
535: 
536: ### Critical Thinking Exercise
537: 
538: 1. **The `RETURNING` clause** is used in both SQLite (modernc.org/sqlite supports it) and Postgres. What does `RETURNING *` do after an `UPDATE`? (Answer: it returns the updated rows as a result set, so the application can read the claimed events without a separate `SELECT`.)
539: 
540: 2. **The `claimed_at` lease is 300 seconds (5 minutes).** What happens if the delivery goroutine takes 6 minutes? (Answer: the poller reclaims the event and tries to deliver it again. You get duplicate delivery — but `idempotency_key` prevents duplicate INSERT. The delivery itself is not idempotent.)
541: 
542: 3. **Why is there no `DeleteAgentEvent` in the interface?** (Answer: events are audit records. They should never be deleted. What's the retention policy? It's not specified — another implicit requirement.)
543: 
544: 4. **The MySQL stubs compile but return errors.** Are there any tests that run against MySQL? (Answer: the test setup in `store/test/store.go` supports MySQL, but the actual tests for integrations only exist for SQLite and Postgres. MySQL support is compile-only.)
545: 
546: ---
547: 
548: ## Chapter 6: The Service Layer — Where the Critical Bugs Live
549: 
550: ### Context
551: 
552: The service layer (`server/router/api/v1/agent/service.go` and `integrations.go`) is where business logic lives. In bchat, the service layer orchestrates the LLM calls, RAG pipeline, and now — webhook event dispatch.
553: 
554: This is also where the most critical bugs were found.
555: 
556: ### The dispatchEvent Function
557: 
558: ```go
559: func (s *Service) dispatchEvent(ctx context.Context, tenantID int32, leadID string, eventType string, data string) {
560:     integrations, err := s.store.ListAgentIntegrations(ctx, &store.FindAgentIntegration{
561:         TenantID:        &tenantID,
562:         IntegrationType: strPtr("webhook"),
563:         ...
564:     })
565:     if len(integrations) == 0 {
566:         return // No integrations configured
567:     }
568: 
569:     for _, ig := range integrations {
570:         // Compute idempotency key
571:         idempotencyKey := computeIdempotencyKey(tenantID, eventType, fmt.Sprintf("%d", ig.ID))
572:         //                           ^^^^^^^^^^  ^^^^^^^^^
573:         //                           BUG: swapped arguments!
574:         //   Expected: computeIdempotencyKey(tenantID, leadID, eventType)
575:         //   Actual:   computeIdempotencyKey(tenantID, eventType, integrationID)
576: ```
577: 
578: ### C1: The Idempotency Key Argument Swap
579: 
580: This was the most damaging bug in the feature.
581: 
582: **What the code should have done:**
583: 
584: ```go
585: idempotencyKey := computeIdempotencyKey(tenantID, leadID, eventType)
586: //                                    tenantID  leadID  eventType
587: ```
588: 
589: This produces `sha256(tenantID + leadID + "lead.captured")` — a unique key per lead per event type.
590: 
591: **What the code actually did:**
592: 
593: ```go
594: idempotencyKey := computeIdempotencyKey(tenantID, eventType, fmt.Sprintf("%d", ig.ID))
595: //                                    tenantID  eventType  integrationID
596: ```
597: 
598: The function signature is `computeIdempotencyKey(tenantID int32, leadID string, eventType string)`. The call passes:
599: - `tenantID` → correct (first parameter)
600: - `eventType` → in the `leadID` parameter position
601: - `fmt.Sprintf("%d", ig.ID)` → in the `eventType` parameter position
602: 
603: For a `lead.captured` event, every lead for the same integration produces the same idempotency key:
604: 
605: ```
606: sha256(tenantID + "lead.captured" + "42")  ← same for ALL leads!
607: ```
608: 
609: The second lead hits the UNIQUE constraint on `agent_events.idempotency_key` and is silently skipped via `continue`. **Only one event per integration is ever dispatched.**
610: 
611: **Why this bug exists:**
612: 
613: The function has three string parameters in a row with the same type. The Go compiler cannot catch this — it compiles perfectly. This is a *semantic* bug, not a *syntactic* bug.
614: 
615: **How it could have been caught:**
616: 
617: 1. **Unit test:** `computeIdempotencyKey(1, "lead-abc", "lead.captured")` should produce a different result from `computeIdempotencyKey(1, "lead-def", "lead.captured")`. A test that verifies this would catch the swap.
618: 
619: 2. **Parameter names in calls:** `computeIdempotencyKey(tenantID: tenantID, leadID: eventType, eventType: ig.ID)` — Go doesn't support named arguments, but a linter could detect suspicious patterns.
620: 
621: 3. **Code review:** A reviewer reading the function call would need to cross-reference with the function signature to check argument order. This is tedious and error-prone — which is exactly why adversarial review specifically checked for it.
622: 
623: ### C2: The Cancelled Context
624: 
625: ```go
626: // In HandleTriggerCron (integrations.go):
627: func (h *Handler) HandleTriggerCron(c echo.Context) error {
628:     h.service.processEventPoller(c.Request().Context())
629:     return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
630: }
631: ```
632: 
633: `c.Request().Context()` is tied to the HTTP request lifecycle. When the handler returns (which happens immediately after `c.JSON` responds), the context is cancelled.
634: 
635: If `processEventPoller` is still delivering webhooks when the context is cancelled, the delivery HTTP requests abort. The events remain in `processing` state, and the poller reclaims them after 5 minutes — but the delivery may fail purely because of context cancellation.
636: 
637: **Why this bug exists:**
638: 
639: The developer reached for the most convenient context — the one from the HTTP request — without thinking about its lifecycle. The HTTP context is designed for request-scoped operations (read from database, write response). Background operations need a separate context.
640: 
641: **The fix:**
642: 
643: ```go
644: func (h *Handler) HandleTriggerCron(c echo.Context) error {
645:     ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
646:     defer cancel()
647:     h.service.processEventPoller(ctx)
648:     return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
649: }
650: ```
651: 
652: `context.Background()` creates a context that lives independently of the HTTP request. The 30-second timeout prevents runaway operations.
653: 
654: ### The Dispatch Flow (How It Should Work)
655: 
656: ```
657: captureLeadFromSession() is called after lead extraction in chat
658:     │
659:     ├── s.store.CreateAgentLead(...)          ── insert lead into DB
660:     │
661:     └── s.dispatchEvent(ctx, tenantID, created.ID, "lead.captured", payload)
662:          │
663:          ├── ListAgentIntegrations(tenantID, "webhook")
664:          │    │
665:          │    └── For each active webhook:
666:          │         │
667:          │         ├── computeIdempotencyKey(tenantID, leadID, "lead.captured")
668:          │         │
669:          │         ├── CreateAgentEvent({ status: 'processing', claimed_at: now })
670:          │         │    └── UNIQUE constraint on idempotency_key prevents duplicates
671:          │         │
672:          │         └── go deliverWebhook(event, integration)    ← goroutine!
673:          │              │
674:          │              ├── Parse webhook config (URL, secret, headers)
675:          │              ├── Build signed payload (HMAC-SHA256)
676:          │              ├── SSRF validation + IP pinning
677:          │              ├── POST to webhook URL
678:          │              │    ├── success → UpdateAgentEvent({ status: 'delivered' })
679:          │              │    └── failure → log error (status stays 'processing')
680:          │              └── (if crash here, poller reclaims after 5 min)
681:          │
682:          └── (error handling) log, continue — don't fail the chat
683: ```
684: 
685: The `go` keyword spawns a goroutine for delivery. This is intentional — the chat response should not wait for the webhook to deliver. But it also means the delivery happens outside the request context, which is why C2 matters.
686: 
687: ### M1: PII Leak in Event List API
688: 
689: ```go
690: func (h *Handler) HandleListEvents(c echo.Context) error {
691:     events, err := h.store.ListAgentEvents(ctx, &store.FindAgentEvent{TenantID: &tenantID})
692:     // Returns raw AgentEvent struct — including Payload field!
693:     return c.JSON(http.StatusOK, events)
694: }
695: ```
696: 
697: The `Payload` field contains the full lead JSON: name, email, phone, session_id, topic. The route uses `tenant:read` permission — which non-admin users can have.
698: 
699: The `HandleListIntegrations` handler correctly masks config from the response:
700: 
701: ```go
702: type safeIntegration struct {
703:     ID              int32  `json:"id"`
704:     IntegrationType string `json:"integration_type"`
705:     Label           string `json:"label"`
706:     Config          string `json:"-"` // REDACTED — never returned
707:     IsActive        bool   `json:"is_active"`
708: }
709: ```
710: 
711: But `HandleListEvents` returns the raw struct, including payload.
712: 
713: **Why this bug exists:** The developer implemented the masking pattern for one endpoint but forgot to apply it to the other. Inconsistency in security patterns.
714: 
715: ### Critical Thinking Exercise
716: 
717: 1. **The `dispatchEvent` function spawns a goroutine (`go deliverWebhook(...)`).** What happens if the server shuts down while a goroutine is running? (Answer: the goroutine is killed. The event stays in 'processing' state. The poller reclaims it on next run.)
718: 
719: 2. **The delivery goroutine uses the parent context, which may be cancelled.** Is this a problem? (Answer: yes — C2 is about the cron handler specifically. But for the immediate dispatch path from `captureLeadFromSession`, the context is the chat request context, which lives long enough for the delivery. Still fragile.)
720: 
721: 3. **How would you make the delivery reliable even if the goroutine crashes?** (Hint: the poller already handles this. The goroutine is an optimization for fast delivery. The poller is the guarantee.)
722: 
723: 4. **The `computeIdempotencyKey` is called inside the `for` loop over integrations.** If there are 3 webhooks, every event produces 3 separate idempotency keys. Is this correct? (Answer: yes — each integration gets its own event. The key includes `integrationID` in the *intended* design, so different integrations don't collide.)
724: 
725: ---
726: 
727: ## Chapter 7: Security Supply Chain
728: 
729: ### Context
730: 
731: Webhooks are inherently dangerous. You're letting your system make HTTP requests to URLs provided by users. Without protection, a user could provide `http://169.254.169.254/latest/meta-data/` — the AWS metadata endpoint — and steal credentials.
732: 
733: Every outbound webhook must be protected by a multi-layer security model.
734: 
735: ### The Story from bugs/033
736: 
737: #### SSRF Protection: Three Layers
738: 
739: **Layer 1: URL Validation**
740: 
741: ```go
742: func validateWebhookURL(rawURL string) (string, error) {
743:     // 1. Scheme must be http or https
744:     // 2. Parse URL (reject invalid)
745:     // 3. Resolve DNS to IP
746:     // 4. Block internal IP ranges:
747:     //    - 127.0.0.0/8 (loopback)
748:     //    - 10.0.0.0/8 (private)
749:     //    - 172.16.0.0/12 (private)
750:     //    - 192.168.0.0/16 (private)
751:     //    - 169.254.0.0/16 (link-local, including metadata endpoint)
752:     // 5. Return resolved IP for pinning
753: }
754: ```
755: 
756: The critical detail: this validation runs against the **resolved IP**, not the hostname. A hostname like `metadata.internal` could resolve to 169.254.169.254. The validation must resolve DNS first, then check the IP.
757: 
758: **Layer 2: IP Pinning**
759: 
760: ```go
761: transport := &http.Transport{
762:     DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
763:         _, port, _ := net.SplitHostPort(addr)
764:         targetAddr := net.JoinHostPort(dialedIP, port)  // Force to validated IP!
765:         return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, targetAddr)
766:     },
767: }
768: ```
769: 
770: DNS is resolved once during validation and the IP is hardcoded. Even if DNS changes between validation and connection (time-of-check/time-of-use), the connection goes to the validated IP.
771: 
772: **Layer 3: Redirect Re-validation**
773: 
774: ```go
775: client := &http.Client{
776:     CheckRedirect: func(req *http.Request, via []*http.Request) error {
777:         if len(via) >= 3 {
778:             return errors.New("too many redirects")
779:         }
780:         // Re-validate redirect target
781:         _, err := validateWebhookURL(req.URL.String())
782:         return err
783:     },
784: }
785: ```
786: 
787: A webhook URL could redirect to `http://169.254.169.254/`. Without redirect checking, the initial URL passes validation but the redirect target is malicious. The `CheckRedirect` handler re-validates every redirect target.
788: 
789: #### HMAC-SHA256 Signing
790: 
791: ```go
792: func signPayload(payload []byte, secret string) string {
793:     mac := hmac.New(sha256.New, []byte(secret))
794:     mac.Write(payload)
795:     return hex.EncodeToString(mac.Sum(nil))
796: }
797: // Header: X-Bchat-Signature: sha256=<hex-digest>
798: ```
799: 
800: The receiver can verify the webhook came from bchat by recomputing the HMAC with the shared secret. This is the standard webhook security pattern (used by GitHub, Stripe, etc.).
801: 
802: #### The CRON_TOKEN Constant-Time Compare
803: 
804: ```go
805: if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
806:     return echo.NewHTTPError(http.StatusUnauthorized, "invalid cron token")
807: }
808: ```
809: 
810: The cron trigger endpoint is protected by a shared token (`CRON_TOKEN`). The comparison uses `Subtle.ConstantTimeCompare` to prevent timing attacks. A regular string comparison (`==`) would leak information byte-by-byte through timing.
811: 
812: However, there was a pre-check:
813: 
814: ```go
815: if token == "" || expectedToken == "" || len(token) != len(expectedToken) {
816:     return echo.NewHTTPError(http.StatusUnauthorized, "invalid cron token")
817: }
818: ```
819: 
820: This length check seems safe — it just rejects obviously wrong tokens early. But it creates a **timing oracle** (M3 bug). An attacker could measure response time to determine the expected token length. Since `CRON_TOKEN` is always `openssl rand -hex 32` (64 chars), the attacker already knows the expected length. But the principle is wrong: don't leak *any* information through timing.
821: 
822: The safer approach is to skip the length check entirely and let `ConstantTimeCompare` handle everything — it's constant-time regardless of where the strings differ.
823: 
824: #### The Copied SSRF Helpers (MO1 Fix)
825: 
826: The SSRF validation functions were copied from `plugin/webhook/webhook.go` because they were unexported there. This is a code smell:
827: 
828: ```go
829: // In plugin/webhook/webhook.go:
830: func validateWebhookURL(rawURL string) (string, error) // unexported!
831: 
832: // In agent/integrations.go:
833: // Copied from plugin/webhook/webhook.go because it's unexported.
834: func validateAndResolveWebhookURL(rawURL string) (string, error)
835: ```
836: 
837: Two copies of the same logic that can drift apart. The proper fix would be to export the function from `plugin/webhook`, but that creates a dependency in the wrong direction. A better approach: move the shared validation to a common utility package.
838: 
839: ### Critical Thinking Exercise
840: 
841: 1. **Is DNS re-resolution during the HTTP request a security risk?** (Answer: yes — the DNS record could change between validation and connection, pointing to a different IP. IP pinning prevents this.)
842: 
843: 2. **What attack does constant-time comparison prevent?** (Answer: timing attacks. Without it, an attacker can guess the token byte-by-byte by measuring response time. With constant-time, every comparison takes the same duration regardless of how many bytes match.)
844: 
845: 3. **Why is there no rate limiting on the webhook delivery endpoints?** (Answer: the rate limiting is on the *sender* side — bchat limits how many events it sends. The *receiver* endpoint — which is not bchat — would need its own rate limiting.)
846: 
847: 4. **The `X-Bchat-Signature` header uses a hex-encoded HMAC. Can an attacker replay this signature?** (Answer: no — signatures are per-payload. A different payload produces a different signature. But if the attacker intercepts the same payload and sends it again, the receiver would need its own replay protection (e.g., timestamp + nonce).)
848: 
849: ---
850: 
851: # PART III: THE OPERATIONS LENS
852: 
853: ## Chapter 8: Containerization
854: 
855: ### Context
856: 
857: Shipping software in containers is the standard deployment model. But containers add complexity: base image selection, multi-stage builds, layer caching, process management, privilege separation, and cron job scheduling.
858: 
859: The bchat deployment uses a multi-stage Docker build with three stages. Let's trace how the webhook integration feature affected the Dockerfile and entrypoint.
860: 
861: ### The Story from bugs/033
862: 
863: #### Multi-Stage Docker Build
864: 
865: ```
866: Stage 1: Frontend (node:20-alpine)
867:   ├── Build main React frontend
868:   └── Build widget bundle
869: 
870: Stage 2: Backend (golang:1.25)
871:   ├── Install CGO deps (gcc, libc-dev)
872:   ├── Copy LanceDB native library
873:   ├── Build Go binary with --tags rag
874:   └── Copy frontend build from Stage 1
875: 
876: Stage 3: Runtime (ubuntu:24.04)
877:   ├── Install runtime deps (ca-certificates, tzdata, gosu, curl)
878:   ├── Install supercronic (cron binary)
879:   ├── Copy Go binary from Stage 2
880:   ├── Copy entrypoint script
881:   ├── Copy crontab file
882:   ├── Create non-root user 'memos'
883:   └── EXPOSE 5230 → ENTRYPOINT ["./entrypoint.sh", "./memos"]
884: ```
885: 
886: #### The Supercronic Problem
887: 
888: The webhook integration needs a cron job to poll for failed event deliveries every 5 minutes.
889: 
890: **Option A: In-process cron** (using a Go cron library like `robfig/cron`)
891: 
892: - Runs inside the main process
893: - No external dependency
894: - But: cron dies if the main process panics. Can't survive restarts. Complicates shutdown.
895: 
896: **Option B: External cron (supercronic)**
897: 
898: - Runs as a separate process inside the same container
899: - Container-native: receives SIGTERM, clean shutdown
900: - Standard cron syntax
901: - But: adds another process to manage. Shell compatibility matters.
902: 
903: The team chose option B. This required changes to three files:
904: 
905: **1. Dockerfile.pg.fly** — Install supercronic:
906: 
907: ```dockerfile
908: ARG SUPERCRONIC_URL=https://github.com/aptible/supercronic/releases/download/v0.2.33/supercronic-linux-amd64
909: ARG SUPERCRONIC_SHA1SUM=c430cce812b1b47863a2f2506571a2f3c1357845
910: RUN curl -fsSL "$SUPERCRONIC_URL" -o /usr/local/bin/supercronic \
911:     && echo "$SUPERCRONIC_SHA1SUM /usr/local/bin/supercronic" | sha1sum -c - \
912:     && chmod +x /usr/local/bin/supercronic
913: ```
914: 
915: The SHA1 verification ensures the downloaded binary matches the expected checksum. Container builds should always verify downloaded artifacts.
916: 
917: **2. build/crontab** — The cron schedule:
918: 
919: ```
920: */5 * * * * curl -sf -X POST http://localhost:${PORT:-5230}/api/v1/system/trigger-cron -H "X-Cron-Token: ${CRON_TOKEN}"
921: ```
922: 
923: **3. scripts/entrypoint.sh** — Launch supercronic before the main process:
924: 
925: ```bash
926: # Expand _FILE env vars (Docker secrets pattern)
927: file_env "CRON_TOKEN"
928: 
929: # Launch supercronic in background if available
930: if command -v supercronic >/dev/null 2>&1 && [ -n "$CRON_TOKEN" ]; then
931:     supercronic /etc/bchat/crontab &
932: fi
933: 
934: # Drop privileges and run main process
935: exec gosu memos ./memos --mode prod
936: ```
937: 
938: #### M2: The Shell Compatibility Bug
939: 
940: ```bash
941: # build/crontab — this line:
942: */5 * * * * curl -sf -X POST http://localhost:${PORT:-5230}/api/v1/system/trigger-cron -H "X-Cron-Token: ${CRON_TOKEN}"
943: ```
944: 
945: `${PORT:-5230}` is a bash-ism. It means "use PORT if set, otherwise default to 5230."
946: 
947: Supercronic executes commands via `/bin/sh -c`. On Ubuntu 24.04 (the runtime image), `/bin/sh` is **dash**, not bash. Dash does NOT support `${VAR:-default}` syntax.
948: 
949: If `PORT` is unset, this expands to:
950: 
951: ```
952: http://localhost:/api/v1/system/trigger-cron
953: ```
954: 
955: The port is empty — `curl` fails because `localhost:` is not a valid URL. Supercronic logs the failure silently. The poller never runs.
956: 
957: **Why this bug exists:**
958: 
959: The developer tested the crontab locally (where `/bin/sh` was bash) and never tested it in the actual Docker image (where `/bin/sh` is dash). The difference between shells is invisible until it breaks in production.
960: 
961: **The fix:**
962: 
963: ```bash
964: # Hardcode the port (it's already defined in Dockerfile and fly.toml)
965: */5 * * * * curl -sf -X POST http://localhost:5230/api/v1/system/trigger-cron -H "X-Cron-Token: ${CRON_TOKEN}"
966: ```
967: 
968: Or use a wrapper script that handles the default:
969: ```bash
970: */5 * * * * PORT=${PORT:-5230} curl -sf -X POST "http://localhost:${PORT}/api/v1/system/trigger-cron" -H "X-Cron-Token: ${CRON_TOKEN}"
971: ```
972: 
973: ### The Entrypoint Pattern
974: 
975: The entrypoint script (`scripts/entrypoint.sh`) follows a specific pattern:
976: 
977: 1. **`_FILE` secret expansion** — Docker secrets pattern: if `FOO_FILE` is set, read the value from the file at that path. Useful for Kubernetes secrets and Docker Swarm.
978: 2. **Volume ownership fix** — Ensure the data directory is writable by the non-root user.
979: 3. **Supercronic launch** — Start background cron, but ONLY if `CRON_TOKEN` is set.
980: 4. **Privilege drop** — `exec gosu memos ./memos` switches from root to the `memos` user.
981: 
982: The `exec` is important — it replaces the shell process with the application process. Without `exec`, the shell stays as PID 1 and signals (SIGTERM, SIGINT) go to the shell, not the application. With `exec`, the application is PID 1 and receives signals directly.
983: 
984: ### Critical Thinking Exercise
985: 
986: 1. **The crontab has `${CRON_TOKEN}`. Should this be in a file referenced by the crontab, or interpolated by the entrypoint?** (Answer: supercronic expands env vars from the environment at runtime. If `CRON_TOKEN` is set as a Docker env var or in an env file, supercronic can read it directly. The entrypoint's `file_env` function handles the Docker secrets pattern.)
987: 
988: 2. **What happens to supercronic when the container receives SIGTERM?** (Answer: the `exec gosu memos ./memos` replaces the shell. The main process gets SIGTERM directly. Supercronic, as a background process, does NOT receive the signal — it keeps running until the container actually stops. This is acceptable for a 5-minute polling cron.)
989: 
990: 3. **The `kill_timeout = "30s"` in fly.toml gives the container 30 seconds to shut down after SIGTERM. Is this enough for the poller to finish in-flight deliveries?** (Answer: yes — each delivery has a 10-second timeout. 30 seconds covers 3 sequential delivery retries. But concurrent deliveries could be cut off.)
991: 
992: 4. **The Dockerfile sets `ENV MEMOS_PORT="5230"`. Is `PORT` also set?** (Answer: no — the crontab references `PORT`, but only `MEMOS_PORT` is set. The `${PORT:-5230}` default is the only safety net. This is the M2 bug in action: the variable names don't match.)
993: 
994: ---
995: 
996: ## Chapter 9: Database Ops — SQLite to Neon Postgres
997: 
998: ### Context
999: 
1000: bchat supports three database drivers: SQLite (default for development), Postgres (production via Neon), and MySQL (legacy). The development workflow uses SQLite for fast iteration, then deploys to production on Postgres.
1001: 
1002: The migration from SQLite to Postgres is not just a `dump | restore`. It requires understanding the differences between the two databases and how the application code interacts with each.
1003: 
1004: ### The Story from bugs/033
1005: 
1006: #### SQLite Development Setup
1007: 
1008: ```go
1009: // store/db/sqlite/sqlite.go

(Output capped at 50 KB. Showing lines 1-1009. Use offset=1010 to continue.)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/TUTORIAL_DATABASE.md"
}
```

**Output:**
```
<path>/home/chaschel/Documents/go/bchat/TUTORIAL_DATABASE.md</path>
<type>file</type>
<content>
1: # Database Design & Operations: Achieving SQLite/PostgreSQL/MySQL Parity
2: 
3: > **Audience:** Intermediate-to-advanced Go developers who want to understand how a real-world multi-tenant AI platform manages three database backends — and how to port parity to MySQL themselves.
4: >
5: > **Prerequisites:** Go 1.21+, basic SQL, completion of or concurrent reading of `TUTORIAL.md` (the Native Webhooks case study). Familiarity with the bchat codebase structure helps but is not required.
6: >
7: > **Time to work through:** 4–8 hours depending on MySQL porting depth
8: >
9: > **Philosophy:** This tutorial is 60% architectural/operational thinking and 40% code analysis. We do not outsource thinking to coding agents — every porting decision starts with a question, not a command.
10: 
11: ---
12: 
13: ## Table of Contents
14: 
15: - [Part 1: The Big Picture — Database Parity in a Multi-Tenant AI Platform](#part-1-the-big-picture--database-parity-in-a-multi-tenant-ai-platform)
16:   - [Chapter 1: Why Three Database Engines?](#chapter-1-why-three-database-engines)
17:   - [Chapter 2: The Driver Interface Contract](#chapter-2-the-driver-interface-contract)
18: - [Part 2: Anatomy of the Parity Gap](#part-2-anatomy-of-the-parity-gap)
19:   - [Chapter 3: The Schema Gap](#chapter-3-the-schema-gap)
20:   - [Chapter 4: The Implementation Gap](#chapter-4-the-implementation-gap)
21:   - [Chapter 5: SQL Dialect Differences at a Glance](#chapter-5-sql-dialect-differences-at-a-glance)
22: - [Part 3: Porting to MySQL — A Practical Guide](#part-3-porting-to-mysql--a-practical-guide)
23:   - [Chapter 6: Local MySQL Container Setup](#chapter-6-local-mysql-container-setup)
24:   - [Chapter 7: Writing MySQL Agent Migrations](#chapter-7-writing-mysql-agent-migrations)
25:   - [Chapter 8: Implementing MySQL Agent Methods](#chapter-8-implementing-mysql-agent-methods)
26:   - [Chapter 9: DBaaS Deployment Considerations](#chapter-9-dbaas-deployment-considerations)
27: - [Part 4: Operations Engineering](#part-4-operations-engineering)
28:   - [Chapter 10: Migration Strategy](#chapter-10-migration-strategy)
29:   - [Chapter 11: Testing Multi-Backend](#chapter-11-testing-multi-backend)
30:   - [Chapter 12: Production Readiness Checklist](#chapter-12-production-readiness-checklist)
31: - [Part 5: The bugs/033 Lens — Database Parity and Webhook Events](#part-5-the-bugs033-lens--database-parity-and-webhook-events)
32:   - [Chapter 13: What bugs/033 Requires from the Database](#chapter-13-what-bugs033-requires-from-the-database)
33:   - [Chapter 14: Why MySQL Isn't Ready for bugs/033](#chapter-14-why-mysql-isnt-ready-for-bugs033)
34:   - [Chapter 15: Porting Order — What to Implement First](#chapter-15-porting-order--what-to-implement-first)
35: - [Part 6: Architectural Reflections](#part-6-architectural-reflections)
36:   - [Chapter 16: Why Postgres and SQLite Are In Sync But MySQL Lags](#chapter-16-why-postgres-and-sqlite-are-in-sync-but-mysql-lags)
37:   - [Chapter 17: What a Fourth Backend Would Look Like](#chapter-17-what-a-fourth-backend-would-look-like)
38:   - [Chapter 18: Database Parity as a Business Decision](#chapter-18-database-parity-as-a-business-decision)
39: - [Appendices](#appendices)
40:   - [Appendix A: SQL Dialect Reference Card](#appendix-a-sql-dialect-reference-card)
41:   - [Appendix B: MySQL Porting Checklist](#appendix-b-mysql-porting-checklist)
42:   - [Appendix C: Docker Compose for Multi-Backend Testing](#appendix-c-docker-compose-for-multi-backend-testing)
43:   - [Appendix D: Further Reading](#appendix-d-further-reading)
44: 
45: ---
46: 
47: ## Part 1: The Big Picture — Database Parity in a Multi-Tenant AI Platform
48: 
49: ### Chapter 1: Why Three Database Engines?
50: 
51: bchat began as a single-tenant, single-binary application built on SQLite. The original `memos` project used SQLite exclusively — it was simple, embedded, zero-ops. As the project evolved into a multi-tenant AI chat platform, two things happened:
52: 
53: 1. **PostgreSQL was added** for production deployments. The decision to use Postgres over MySQL was driven by Fly.io's native Postgres support, connection pooling via PgBouncer, and the need for concurrent write throughput that SQLite could not provide in a multi-process deployment.
54: 
55: 2. **MySQL was added** as a third option. The `go-sql-driver/mysql` dependency was added to `go.mod`, a MySQL `LATEST.sql` was created with the core Memos schema, and stub implementations were written for all agent features. But MySQL parity for the agent system was never completed.
56: 
57: **Your goal in this tutorial** is to understand what "database parity" means in a Go application that uses the `database/sql` standard library, and to port the missing parity to MySQL yourself. Along the way you will confront questions about migration strategy, SQL dialect portability, testing, and production operations.
58: 
59: #### The Three-Tier Pattern
60: 
61: Every database-backed Go application with multiple backend support follows this pattern:
62: 
63: ```
64: ┌─────────────────────┐
65: │   store.Driver      │  ← Interface (the contract)
66: ├─────────────────────┤
67: │ SQLite  │ Postgres  │  MySQL     ← Implementations
68: ├─────────────────────┤
69: │ sqlite/ │ postgres/ │ mysql/     ← Packages
70: └─────────────────────┘
71: ```
72: 
73: bchat follows this pattern with an important twist: instead of three full implementations, it has **2.3 implementations**. SQLite and Postgres are complete. MySQL has full coverage of the core Memos tables (user, memo, resource, etc.) but all agent-specific features are stubs.
74: 
75: This is not necessarily a bad thing. Not all backends need all features. The question is: **when is parity important, and when is it premature?**
76: 
77: #### Questions to Ask Before Porting
78: 
79: Before we look at a single line of code, ask yourself:
80: 
81: - Who is asking for MySQL support? A specific customer on PlanetScale? An internal requirement for MySQL compatibility? Or just "completeness"?
82: - What features do they actually need? All agent features? Just the chat endpoint? Just the webhook integration?
83: - What is the cost of not porting? Error messages returned to users? Silent data loss? No deployment at all?
84: - What is the maintenance burden of adding a third backend to every future feature? Every new store method must be implemented three times.
85: 
86: Keep these questions in your pocket. We will revisit them in Chapter 18.
87: 
88: ---
89: 
90: ### Chapter 2: The Driver Interface Contract
91: 
92: The entire database parity system rests on a single Go interface in `store/driver.go`:
93: 
94: ```go
95: type Driver interface {
96:     GetDB() *sql.DB
97:     Close() error
98: 
99:     // Core Memos models
100:     FindMigrationHistoryList(ctx context.Context, find *FindMigrationHistory) ([]*MigrationHistory, error)
101:     UpsertMigrationHistory(ctx context.Context, upsert *UpsertMigrationHistory) (*MigrationHistory, error)
102:     CreateActivity(ctx context.Context, create *Activity) (*Activity, error)
103:     ListActivities(ctx context.Context, find *FindActivity) ([]*Activity, error)
104:     CreateResource(ctx context.Context, create *Resource) (*Resource, error)
105:     ListResources(ctx context.Context, find *FindResource) ([]*Resource, error)
106:     CreateMemo(ctx context.Context, create *Memo) (*Memo, error)
107:     ListMemos(ctx context.Context, find *FindMemo) ([]*Memo, error)
108:     // ... ~180 methods total
109: }
110: ```
111: 
112: This is a **fat interface** — it contains every database operation across every domain in the application. The tradeoff is clear:
113: 
114: - **Pro:** One contract. Any backend that satisfies it can be swapped in via the factory. No dependency injection framework needed.
115: - **Con:** Every new feature adds methods to the interface, which means every backend must be updated. A backend that doesn't support a feature still needs a stub.
116: 
117: #### The Factory Pattern
118: 
119: The factory lives in `store/db/db.go`:
120: 
121: ```go
122: func NewDBDriver(profile *profile.Profile) (store.Driver, error) {
123:     var driver store.Driver
124:     var err error
125: 
126:     switch profile.Driver {
127:     case "sqlite":
128:         driver, err = sqlite.NewDB(profile)
129:     case "mysql":
130:         driver, err = mysql.NewDB(profile)
131:     case "postgres":
132:         driver, err = postgres.NewDB(profile)
133:     default:
134:         return nil, errors.New("unknown db driver")
135:     }
136:     if err != nil {
137:         return nil, errors.Wrap(err, "failed to create db driver")
138:     }
139:     return driver, nil
140: }
141: ```
142: 
143: The `profile.Driver` value comes from the application's configuration (`internal/profile/profile.go`):
144: 
145: ```go
146: type Profile struct {
147:     Mode    string // "prod", "dev", "demo"
148:     Driver  string // "sqlite", "mysql", "postgres"
149:     DSN     string // Data Source Name
150:     // ...
151: }
152: 
153: func (p *Profile) Validate() error {
154:     if p.Driver == "sqlite" && p.DSN == "" {
155:         dbFile := fmt.Sprintf("memos_%s.db", p.Mode)
156:         p.DSN = filepath.Join(dataDir, dbFile)
157:     }
158:     if p.Driver == "postgres" && p.DSN == "" {
159:         p.DSN = os.Getenv("DATABASE_URL")
160:         if p.DSN == "" {
161:             return errors.New("postgres driver requires DSN or DATABASE_URL")
162:         }
163:     }
164:     // Note: MySQL has no fallback — the user must provide DSN explicitly
165:     return nil
166: }
167: ```
168: 
169: Notice that MySQL has no DSN fallback. This is a small signal: MySQL support was added but never made it into the default startup flow.
170: 
171: #### Connection Setup Differences
172: 
173: Each backend creates its own `*sql.DB` with different drivers:
174: 
175: | Backend | Driver | Import | Connection Setup |
176: |---------|--------|--------|-----------------|
177: | SQLite | `github.com/mattn/go-sqlite3` | CGO required | File path as DSN, `_journal_mode=WAL` |
178: | Postgres | `github.com/jackc/pgx/v5` | Pure Go | Connection string from `DATABASE_URL` |
179: | MySQL | `github.com/go-sql-driver/mysql` | Pure Go | DSN parsed, `multiStatements=true` for migrations |
180: 
181: The MySQL connection setup in `store/db/mysql/mysql.go`:
182: 
183: ```go
184: func NewDB(profile *profile.Profile) (store.Driver, error) {
185:     dsn, err := mergeDSN(profile.DSN)
186:     if err != nil {
187:         return nil, err
188:     }
189:     // ...
190:     driver.db, err = sql.Open("mysql", dsn)
191:     // ...
192: }
193: 
194: func mergeDSN(baseDSN string) (string, error) {
195:     config, err := mysql.ParseDSN(baseDSN)
196:     if err != nil {
197:         return "", errors.Wrapf(err, "failed to parse DSN: %s", baseDSN)
198:     }
199:     config.MultiStatements = true  // Required for migration files with multiple SQL statements
200:     return config.FormatDSN(), nil
201: }
202: ```
203: 
204: Key observation: the MySQL connection forces `MultiStatements = true` because the migration system executes multiple SQL statements per file. This is a security concern — it means SQL injection via the migration system is possible if user input ever enters migration files (it shouldn't, but the surface area is larger).
205: 
206: ---
207: 
208: ## Part 2: Anatomy of the Parity Gap
209: 
210: ### Chapter 3: The Schema Gap
211: 
212: The easiest way to see the parity gap is to compare the `LATEST.sql` files across all three backends:
213: 
214: | Metric | SQLite | Postgres | MySQL |
215: |--------|--------|----------|-------|
216: | Lines of SQL | 1,067 | 960 | 132 |
217: | Agent tables | 30+ | 30+ | **0** |
218: | RBAC tables | Yes | Yes | No |
219: | Bridge tables | Yes | Yes | No |
220: | Migration directories | 32 versions | 14 versions | 11 versions |
221: | Latest migration | 0.31 | 0.31 | 0.25 |
222: 
223: The MySQL `LATEST.sql` at `store/migration/mysql/LATEST.sql` contains only the original Memos tables:
224: 
225: ```sql
226: -- migration_history
227: CREATE TABLE `migration_history` (
228:   `version` VARCHAR(256) NOT NULL PRIMARY KEY,
229:   `created_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
230: );
231: 
232: -- user
233: CREATE TABLE `user` (
234:   `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
235:   `username` VARCHAR(256) NOT NULL UNIQUE,
236:   `role` VARCHAR(256) NOT NULL DEFAULT 'USER',
237:   `password_hash` VARCHAR(256) NOT NULL,
238:   ...
239: );
240: 
241: -- memo
242: CREATE TABLE `memo` (
243:   `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
244:   `uid` VARCHAR(256) NOT NULL UNIQUE,
245:   `creator_id` INT NOT NULL,
246:   `content` TEXT NOT NULL,
247:   `visibility` VARCHAR(256) NOT NULL DEFAULT 'PRIVATE',
248:   `payload` JSON NOT NULL,
249:   ...
250: );
251: 
252: -- resource, activity, idp, inbox, webhook, reaction...
253: ```
254: 
255: That's it. No `agent_tenants`, no `agent_audiences`, no `agent_sessions`, no `agent_integrations`, no `agent_events`, no bridge tables, no RBAC, no observational memory, no workflows, no QA pairs, no transcripts, no leads, no reindex checkpoints.
256: 
257: Compare with the SQLite schema at `store/migration/sqlite/LATEST.sql` which spans agent tables at lines 201–1067. The agent tables alone include:
258: 
259: - `agent_tenants` — tenant definitions
260: - `agent_audiences` — per-tenant audience configuration
261: - `agent_services` — service catalog entries
262: - `agent_exclusions` — service exclusions
263: - `agent_coverage` — geographic coverage
264: - `agent_faqs` — FAQ entries
265: - `agent_safety_protocols` — safety instructions
266: - `agent_kb_sections` — knowledge base sections
267: - `agent_intents` — customer intents
268: - `agent_rules` — behavioral rules
269: - `agent_sessions` — chat sessions
270: - `agent_source_files` — KB/Policy/Script versioned files
271: - `agent_rate_limits` — per-IP rate limiting
272: - `agent_simulations` — simulation runs
273: - `agent_tenant_scripts` — conversation scripts
274: - `agent_script_analysis` — script compliance analysis
275: - `agent_qa_pairs` — Q&A pairs for embedding testing
276: - `agent_transcripts` — chat conversation transcripts
277: - `agent_leads` — captured leads
278: - `agent_workflows` — ticket workflows
279: - `agent_simulation_transcripts` — simulation transcripts
280: - `agent_analysis_results` — conversation analysis
281: - `agent_learning_memory` — learned patterns
282: - `agent_compliance_audits` — compliance audit results
283: - `agent_scoring_config` — scoring configuration
284: - `agent_reindex_checkpoints` — RAG reindex resume points
285: - `agent_observations` — observational memory
286: - `bridge_external_sessions` — external bridge sessions
287: - `bridge_handoffs` — handoff management
288: - `bridge_auth_keys` — bridge HMAC authentication
289: - `bridge_auth_nonces` — replay protection
290: - `bridge_handoff_replies` — handoff replies
291: - `bridge_reply_outbox` — outbox pattern for delivery
292: - `agent_messages` — deduplicated message store
293: - `agent_integrations` — webhook integrations (migration 31)
294: - `agent_events` — webhook events (migration 31)
295: 
296: #### Migration Version Gap
297: 
298: The MySQL migration directory at `store/migration/mysql/` stops at version `0.25`:
299: 
300: ```
301: store/migration/mysql/          store/migration/sqlite/
302: ├── 0.17/                        ├── 0.17/
303: ├── 0.18/                        ├── 0.18/
304: ├── 0.19/                        ├── 0.19/
305: ├── 0.20/                        ├── 0.20/
306: ├── 0.21/                        ├── 0.21/
307: ├── 0.22/                        ├── 0.22/
308: ├── 0.23/                        ├── 0.23/
309: ├── 0.24/                        ├── 0.24/
310: ├── 0.25/                        ├── 0.25/
311: └── LATEST.sql                   ├── 0.26/
312:                                  ├── 0.27/
313:                                  ├── 0.28/
314:                                  ├── 0.29/
315:                                  ├── 0.30/
316:                                  └── 0.31/
317: ```
318: 
319: Every version from 0.26 to 0.31 added agent-specific tables, RBAC, bridge delivery, integrations, and events. The MySQL backend never received corresponding migrations.
320: 
321: **Key insight:** The migration system per-driver paths (e.g., `migration/postgres/`, `migration/sqlite/`) mean that adding MySQL migrations does not cause merge conflicts with other backends. Each backend's migration files are independent. The tradeoff is that you must maintain `n` copies of each migration.
322: 
323: ---
324: 
325: ### Chapter 4: The Implementation Gap
326: 
327: The schema gap is visible from the file tree. The implementation gap requires reading the code.
328: 
329: #### Agent Methods: All Stubbed
330: 
331: The MySQL agent implementation at `store/db/mysql/agent.go` is 449 lines of this pattern:
332: 
333: ```go
334: var errNotImplemented = errors.New("agent features not implemented for MySQL")
335: 
336: func (d *DB) CreateAgentTenant(ctx context.Context, tenant *store.AgentTenant) (*store.AgentTenant, error) {
337:     return nil, errNotImplemented
338: }
339: 
340: func (d *DB) GetAgentTenant(ctx context.Context, find *store.FindAgentTenant) (*store.AgentTenant, error) {
341:     return nil, errNotImplemented
342: }
343: 
344: func (d *DB) ListAgentTenants(ctx context.Context, find *store.FindAgentTenant) ([]*store.AgentTenant, error) {
345:     return nil, errNotImplemented
346: }
347: 
348: func (d *DB) UpdateAgentTenant(ctx context.Context, tenant *store.AgentTenant) (*store.AgentTenant, error) {
349:     return nil, errNotImplemented
350: }
351: 
352: func (d *DB) DeleteAgentTenant(ctx context.Context, id int32) error {
353:     return errNotImplemented
354: }
355: ```
356: 
357: Every single agent method follows this pattern. There are approximately 70 such methods covering tenants, audiences, services, exclusions, coverage, FAQs, safety protocols, KB sections, intents, rules, sessions, source files, simulations, transcripts, analysis, learning memory, compliance audits, scoring config, QA pairs, leads, reindex checkpoints, integrations, and events.
358: 
359: #### Rate Limiting: Fail-Open
360: 
361: Two methods deviate from the `errNotImplemented` pattern — they **fail open** instead:
362: 
363: ```go
364: func (d *DB) CheckAndIncrementAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string, rpm int) (bool, error) {
365:     slog.Warn("agent rate limit check not implemented for MySQL, allowing request",
366:         "tenantID", tenantID, "clientIP", clientIP, "audienceType", audienceType)
367:     return true, nil
368: }
369: 
370: func (d *DB) CheckAndIncrementTenantGlobalRateLimit(ctx context.Context, tenantID int32, audienceType string, rpm int) (bool, error) {
371:     slog.Warn("tenant global rate limit check not implemented for MySQL, allowing request",
372:         "tenantID", tenantID, "audienceType", audienceType)
373:     return true, nil
374: }
375: ```
376: 
377: This is a deliberate design choice: if rate limiting is not implemented, the system should allow requests rather than deny them. The warning log is there so operators notice. But in a production deployment, this is a **security gap** — any client can bypass rate limits.
378: 
379: **Question for you:** Is fail-open the right default? What are the alternatives?
380: - Fail-closed (deny all requests) — safe but breaks the feature entirely
381: - Fail-open with a low static limit (max 5 RPM) — better than unlimited
382: - Environment-level gate (only fail-open in dev mode, fail-closed in prod)
383: 
384: #### Bridge Delivery: Unsupported
385: 
386: The MySQL bridge implementation at `store/db/mysql/bridge.go` returns `store.ErrBridgeUnsupportedDatabase` for every method:
387: 
388: ```go
389: func (d *DB) EnsureBridgeExternalSession(context.Context, int32, string, time.Time, time.Time) (*store.BridgeExternalSession, bool, error) {
390:     return nil, false, store.ErrBridgeUnsupportedDatabase
391: }
392: ```
393: 
394: The `SupportsBridgeDelivery()` method on the MySQL driver returns `false`, which the service layer checks before attempting bridge operations.
395: 
396: #### Agent Workflows: Silent Stubs
397: 
398: The MySQL agent workflow stubs at `store/db/mysql/agent_workflow.go` are the most dangerous pattern:
399: 
400: ```go
401: func (d *DB) CreateAgentWorkflow(ctx context.Context, create *store.CreateAgentWorkflow) (*store.AgentWorkflow, error) {
402:     return nil, nil  // TODO: Implement
403: }
404: 
405: func (d *DB) ListAgentWorkflows(ctx context.Context, find *store.FindAgentWorkflow) ([]*store.AgentWorkflow, error) {
406:     return nil, nil  // TODO: Implement
407: }
408: 
409: func (d *DB) GetAgentWorkflow(ctx context.Context, find *store.FindAgentWorkflow) (*store.AgentWorkflow, error) {
410:     return nil, nil  // TODO: Implement
411: }
412: ```
413: 
414: Returning `nil, nil` means the caller thinks the operation succeeded (no error), but gets back a nil result. Depending on the caller, this could cause nil pointer dereferences, silent data loss, or confusing "not found" behavior. This is worse than `errNotImplemented` because errors are visible to the operator; silent nil returns are invisible until something breaks.
415: 
416: #### Observational Memory: Explicit Errors
417: 
418: The MySQL observations stubs at `store/db/mysql/agent_observations.go` return errors, but inconsistently:
419: 
420: ```go
421: func (d *DB) UpsertObservationLog(ctx context.Context, log *store.ObservationLog) (*store.ObservationLog, error) {
422:     return nil, fmt.Errorf("UpsertObservationLog not implemented for MySQL")
423: }
424: ```
425: 
426: Note the use of `fmt.Errorf` instead of the `errNotImplemented` sentinel error used in agent.go. Inconsistency in error wrapping is a code review smell — it makes it harder for callers to check error types.
427: 
428: ---
429: 
430: ### Chapter 5: SQL Dialect Differences at a Glance
431: 
432: Before we can write MySQL migrations, we need to understand the SQL dialects of all three backends. Here is the practical reference:
433: 
434: #### Placeholders
435: 
436: | Backend | Placeholder | Example |
437: |---------|-------------|---------|
438: | SQLite | `?` (positional) | `WHERE id = ?` |
439: | Postgres | `$N` (numbered) | `WHERE id = $1` |
440: | MySQL | `?` (positional) | `WHERE id = ?` |
441: 
442: **MySQL is compatible with SQLite** for placeholder syntax. This means porting from SQLite to MySQL is easier than porting from Postgres to MySQL — you can reuse the SQLite query structure and only change data types and functions.
443: 
444: Postgres uses `$1`, `$2`, ... which requires `fmt.Sprintf(" AND id = $%d", argIdx)` in the Go code. Both SQLite and MySQL use simple `?` and append args to a slice.
445: 
446: #### Auto-Increment / Serial
447: 
448: ```sql
449: -- SQLite
450: id INTEGER PRIMARY KEY AUTOINCREMENT
451: 
452: -- Postgres
453: id SERIAL PRIMARY KEY
454: 
455: -- MySQL
456: id INT NOT NULL AUTO_INCREMENT PRIMARY KEY
457: ```
458: 
459: MySQL can also use `BIGINT` instead of `INT` if you expect more than 2 billion rows:
460: 
461: ```sql
462: id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY
463: ```
464: 
465: #### Timestamps
466: 
467: This is the trickiest difference. The choice is between **epoch integers** (SQLite/Postgres) and **native TIMESTAMP** (MySQL):
468: 
469: ```sql
470: -- SQLite (epoch integer, stored as INTEGER)
471: created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now'))
472: 
473: -- Postgres (epoch integer, stored as BIGINT)
474: created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())
475: 
476: -- MySQL option 1: native TIMESTAMP (stored as DATETIME)
477: created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
478: 
479: -- MySQL option 2: epoch integer (to match SQLite pattern)
480: created_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP())
481: ```
482: 
483: **bchat's SQLite schema** uses a mix of both approaches:
484: - Core tables (memo, user, activity) use `BIGINT` epoch timestamps via `strftime('%s', 'now')`
485: - Agent tables use `TIMESTAMP` via `CURRENT_TIMESTAMP`
486: - Newer agent tables (integrations, events) use `BIGINT` epoch timestamps
487: 
488: This inconsistency within SQLite itself is worth noting. The Postgres schema mirrors SQLite's approach, while the MySQL schema (for core tables) uses `TIMESTAMP DEFAULT CURRENT_TIMESTAMP`.
489: 
490: **Your decision:** When porting agent tables to MySQL, you can choose:
491: - **Match MySQL style:** Use `TIMESTAMP DEFAULT CURRENT_TIMESTAMP` — idiomatic MySQL, works with MySQL tools
492: - **Match SQLite agent style:** Use `BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP())` — keeps the API consistent across backends
493: 
494: There is no universally correct answer. The tradeoff is between backend idiomaticity and cross-backend consistency. Both approaches are viable.
495: 
496: #### JSON
497: 
498: ```sql
499: -- SQLite
500: payload TEXT NOT NULL DEFAULT '{}'
501: 
502: -- Postgres
503: payload JSONB NOT NULL DEFAULT '{}'
504: 
505: -- MySQL
506: payload JSON NOT NULL
507: ```
508: 
509: MySQL's `JSON` type is similar to Postgres's `JSONB` — it validates JSON on insert and stores it in a binary format that allows indexing via generated columns. SQLite's `TEXT` approach works but requires application-level validation.
510: 
511: #### Booleans
512: 
513: ```sql
514: -- SQLite (0 or 1, stored as INTEGER)
515: is_active INTEGER NOT NULL DEFAULT 1
516: CHECK (is_active IN (0, 1))
517: 
518: -- Postgres (true/false, stored as BOOLEAN)
519: is_active BOOLEAN NOT NULL DEFAULT TRUE
520: 
521: -- MySQL (true/false, stored as TINYINT)
522: is_active BOOLEAN NOT NULL DEFAULT TRUE
523: -- or
524: is_active TINYINT NOT NULL DEFAULT 1
525: ```
526: 
527: MySQL treats `BOOLEAN` as `TINYINT(1)`. Values are 0 (false) and non-zero (true). This is compatible with SQLite's integer approach — both return integers from queries.
528: 
529: #### RETURNING Clause vs LastInsertId
530: 
531: ```go
532: // SQLite: INSERT + LastInsertId
533: result, err := d.db.ExecContext(ctx, stmt, args...)
534: id, err := result.LastInsertId()
535: 
536: // Postgres: INSERT ... RETURNING id
537: err := d.db.QueryRowContext(ctx, stmt, args...).Scan(&id)
538: 
539: // MySQL: same as SQLite pattern
540: result, err := d.db.ExecContext(ctx, stmt, args...)
541: id, err := result.LastInsertId()
542: ```
543: 
544: MySQL 8.0.30+ supports `RETURNING` clauses (via `VALUES ... RETURNING ...` syntax in MySQL 8.0.34+), but the existing codebase uses `LastInsertId` for SQLite, and MySQL's `database/sql` driver supports it natively. Stick with `LastInsertId` for MySQL compatibility.
545: 
546: #### CHECK Constraints
547: 
548: ```sql
549: -- SQLite: enforced
550: status TEXT NOT NULL CHECK (status IN ('active', 'closed'))
551: 
552: -- Postgres: enforced
553: status TEXT NOT NULL CHECK (status IN ('active', 'closed'))
554: 
555: -- MySQL before 8.0.16: parsed but ignored
556: -- MySQL 8.0.16+: enforced
557: status TEXT NOT NULL CHECK (status IN ('active', 'closed'))
558: ```
559: 
560: **Critical:** If your target MySQL version is < 8.0.16, CHECK constraints are **syntactically valid but not enforced**. You must enforce constraints at the application level. PlanetScale uses Vitess, which also ignores CHECK constraints.
561: 
562: As of 2026, MySQL 8.0 has reached end of life and 9.0 is the current GA release. Most modern MySQL deployments (8.0.16+, 8.4+, 9.0+) enforce CHECK constraints. But if you target PlanetScale, assume they are not enforced.
563: 
564: #### UPSERT / ON CONFLICT
565: 
566: ```sql
567: -- SQLite
568: INSERT INTO agent_rate_limits (...) VALUES (...)
569: ON CONFLICT(tenant_id, audience_type, client_ip) DO UPDATE SET request_count = request_count + 1;
570: 
571: -- Postgres
572: INSERT INTO agent_rate_limits (...) VALUES (...)
573: ON CONFLICT(tenant_id, audience_type, client_ip) DO UPDATE SET request_count = request_count + 1;
574: 
575: -- MySQL
576: INSERT INTO agent_rate_limits (...) VALUES (...)
577: ON DUPLICATE KEY UPDATE request_count = request_count + 1;
578: ```
579: 
580: MySQL uses `ON DUPLICATE KEY UPDATE` instead of `ON CONFLICT ... DO UPDATE`. The behavior is similar but the syntax is different.
581: 
582: #### Identifiers
583: 
584: ```sql
585: -- SQLite: unquoted or double-quoted
586: SELECT * FROM user WHERE username = 'alice';
587: 
588: -- Postgres: double-quoted (lowercase unquoted is folded to lowercase)
589: SELECT * FROM "user" WHERE username = 'alice';
590: 
591: -- MySQL: backtick-quoted (configurable, but backtick is standard)
592: SELECT * FROM `user` WHERE username = 'alice';
593: ```
594: 
595: The MySQL backtick requirement is specific to MySQL and MariaDB. The existing MySQL implementations in the codebase use backtick-quoted identifiers, which is the MySQL convention.
596: 
597: #### Partial / Conditional Indexes
598: 
599: ```sql
600: -- SQLite
601: CREATE UNIQUE INDEX idx_tickets_beads_id ON tickets(beads_id) WHERE beads_id IS NOT NULL;
602: 
603: -- Postgres
604: CREATE UNIQUE INDEX idx_tickets_beads_id ON tickets(beads_id) WHERE beads_id IS NOT NULL;
605: 
606: -- MySQL: NOT SUPPORTED
607: -- Workaround: use a generated column and unique index on the generated column
608: ```
609: 
610: MySQL does not support `WHERE` clauses on indexes (partial indexes). This is a significant limitation. The workaround is to use generated columns or to enforce uniqueness at the application level.
611: 
612: #### ON DELETE CASCADE
613: 
614: ```sql
615: -- SQLite: enforced when PRAGMA foreign_keys = ON
616: FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
617: 
618: -- Postgres: enforced by default
619: FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
620: 
621: -- MySQL: enforced when the storage engine supports it (InnoDB does)
622: FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
623: ```
624: 
625: All three backends support `ON DELETE CASCADE` with InnoDB (MySQL's default engine). **PlanetScale and Vitess-based MySQL deployments do not support foreign keys.** If you target PlanetScale, you must handle referential integrity at the application layer.
626: 
627: ---
628: 
629: ## Part 3: Porting to MySQL — A Practical Guide
630: 
631: ### Chapter 6: Local MySQL Container Setup
632: 
633: Before writing any code, get a MySQL instance running locally.
634: 
635: #### Docker Compose Configuration
636: 
637: Create a `docker-compose.mysql.yml` in the project root:
638: 
639: ```yaml
640: version: "3.9"
641: services:
642:   mysql:
643:     image: mysql:8.4
644:     container_name: bchat-mysql
645:     environment:
646:       MYSQL_ROOT_PASSWORD: rootpass
647:       MYSQL_DATABASE: memos
648:       MYSQL_USER: memos
649:       MYSQL_PASSWORD: memospass
650:     ports:
651:       - "3306:3306"
652:     volumes:
653:       - mysql_data:/var/lib/mysql
654:     command: >
655:       --default-authentication-plugin=mysql_native_password
656:       --log-bin-trust-function-creators=1
657: 
658: volumes:
659:   mysql_data:
660: ```
661: 
662: Start it:
663: 
664: ```bash
665: docker compose -f docker-compose.mysql.yml up -d
666: ```
667: 
668: Verify connectivity:
669: 
670: ```bash
671: mysql -h 127.0.0.1 -u memos -pmemospass memos -e "SELECT VERSION();"
672: ```
673: 
674: #### Environment Variables for MySQL Mode
675: 
676: ```bash
677: export MEMOS_DRIVER=mysql
678: export MEMOS_DSN="memos:memospass@tcp(127.0.0.1:3306)/memos?charset=utf8mb4&parseTime=true&loc=UTC"
679: ```
680: 
681: Note the `parseTime=true` parameter — this tells the Go MySQL driver to automatically scan `TIMESTAMP`/`DATETIME` columns into `time.Time` values. Without it, timestamps are returned as strings.
682: 
683: #### Testing MySQL 9.0
684: 
685: Replace the image with `mysql:9.0` for testing with the latest MySQL. The SQL dialect changes between 8.0 and 9.0 are minimal for the operations we need, but 9.0 removes some deprecated features (like `GROUP BY` ASC/DESC shorthand that was already removed in 8.0.13).
686: 
687: #### Testing PlanetScale Compatibility
688: 
689: PlanetScale uses Vitess, which is MySQL-compatible but has significant differences:
690: 
691: - No foreign keys (Vitess ignores them)
692: - No CHECK constraints (Vitess ignores them)
693: - No `LOCK TABLES` (Vitess uses distributed locks)
694: - No `ON DUPLICATE KEY UPDATE` with more complex expressions
695: - Limited DDL (some ALTER TABLE operations require online schema changes via `vtctl`)
696: - No `SELECT ... FOR UPDATE` across shards
697: 
698: To test PlanetScale compatibility, you would need a PlanetScale database or a Vitess instance. For local testing, PlanetScale provides a CLI (`pscale connect`) that opens a local TCP tunnel to your database branch.
699: 
700: ---
701: 
702: ### Chapter 7: Writing MySQL Agent Migrations
703: 
704: Now we port the missing migrations. Start with version `0.26` and work through `0.31`.
705: 
706: #### Migration File Structure
707: 
708: Each migration file goes in `store/migration/mysql/`:
709: 
710: ```
711: store/migration/mysql/
712: ├── 0.26/
713: │   ├── 00__agent_tenants.sql
714: │   ├── 01__agent_audiences.sql
715: │   ├── 02__agent_services.sql
716: │   ├── 03__agent_exclusions.sql
717: │   └── ...
718: ├── 0.27/
719: │   ├── 00__rbac.sql
720: │   └── ...
721: ├── 0.28/
722: │   ├── 00__simulations.sql
723: │   └── ...
724: ├── 0.29/
725: │   ├── 00__scripts.sql
726: │   └── ...
727: ├── 0.30/
728: │   ├── 00__bridge.sql
729: │   └── ...
730: └── 0.31/
731:     ├── 00__integrations.sql
732:     └── 01__events.sql
733: ```
734: 
735: #### Translating SQLite DDL to MySQL
736: 
737: Let's walk through a specific example: the `agent_tenants` table.
738: 
739: **SQLite version** (from `store/migration/sqlite/0.26/00__agent_tenants.sql`):
740: 
741: ```sql
742: CREATE TABLE agent_tenants (
743:     id INTEGER PRIMARY KEY AUTOINCREMENT,
744:     slug TEXT UNIQUE NOT NULL,
745:     company_name TEXT NOT NULL,
746:     guid TEXT NOT NULL UNIQUE,
747:     widget_key TEXT,
748:     vertical TEXT,
749:     is_active INTEGER NOT NULL DEFAULT 1,
750:     processing_options TEXT,
751:     allowed_domains TEXT,
752:     created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
753:     updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
754: );
755: 
756: CREATE INDEX IF NOT EXISTS idx_agent_tenants_guid ON agent_tenants(guid);
757: CREATE INDEX IF NOT EXISTS idx_agent_tenants_widget_key ON agent_tenants(widget_key);
758: ```
759: 
760: **MySQL translation:**
761: 
762: ```sql
763: CREATE TABLE `agent_tenants` (
764:     `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
765:     `slug` VARCHAR(255) NOT NULL,
766:     `company_name` VARCHAR(255) NOT NULL,
767:     `guid` VARCHAR(36) NOT NULL,
768:     `widget_key` VARCHAR(255) DEFAULT NULL,
769:     `vertical` VARCHAR(100) DEFAULT NULL,
770:     `is_active` BOOLEAN NOT NULL DEFAULT TRUE,
771:     `processing_options` JSON DEFAULT NULL,
772:     `allowed_domains` JSON DEFAULT NULL,
773:     `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
774:     `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
775:     UNIQUE KEY `uk_agent_tenants_slug` (`slug`),
776:     UNIQUE KEY `uk_agent_tenants_guid` (`guid`)
777: ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
778: 
779: CREATE INDEX `idx_agent_tenants_guid` ON `agent_tenants`(`guid`);
780: CREATE INDEX `idx_agent_tenants_widget_key` ON `agent_tenants`(`widget_key`);
781: ```
782: 
783: **Key changes:**
784: 
785: | SQLite | MySQL | Why |
786: |--------|-------|-----|
787: | `INTEGER PRIMARY KEY AUTOINCREMENT` | `INT NOT NULL AUTO_INCREMENT PRIMARY KEY` | MySQL's auto-increment syntax |
788: | `TEXT` | `VARCHAR(255)` or `TEXT` | VARCHAR for indexed columns (MySQL limits index key length), TEXT for large blobs |
789: | `UNIQUE` constraint on column | `UNIQUE KEY` at table level | Both work, but table-level UNIQUE KEY is more explicit |
790: | `DEFAULT 1` (integer) | `DEFAULT TRUE` (boolean) | MySQL BOOLEAN is TINYINT, but TRUE/FALSE is idiomatic |
791: | No ENGINE/CHARSET | `ENGINE=InnoDB DEFAULT CHARSET=utf8mb4` | Required for foreign key support and full Unicode |
792: 
793: #### Handling JSON Columns
794: 
795: The SQLite schema uses `TEXT` for JSON fields like `processing_options` and `allowed_domains`. In MySQL, use the `JSON` type:
796: 
797: ```sql
798: -- SQLite (stores as TEXT)
799: processing_options TEXT,
800: 
801: -- MySQL (validates as JSON)
802: processing_options JSON DEFAULT NULL,
803: ```
804: 
805: MySQL's `JSON` type validates the JSON on insert and stores it in a binary format. This is stricter than SQLite's TEXT approach but prevents malformed JSON from entering the database.
806: 
807: #### Handling TIMESTAMP vs BIGINT
808: 
809: The SQLite agent tables use `TIMESTAMP` type with `CURRENT_TIMESTAMP` default. Some tables use `BIGINT` epoch timestamps. The MySQL equivalents:
810: 
811: ```sql
812: -- SQLite: TIMESTAMP default
813: created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
814: 
815: -- MySQL: same syntax, works the same
816: created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
817: 
818: -- SQLite: BIGINT epoch
819: created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now'))
820: 
821: -- MySQL: BIGINT epoch equivalent
822: created_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP())
823: ```
824: 
825: **Consistency choice:** The existing MySQL core tables use `TIMESTAMP DEFAULT CURRENT_TIMESTAMP`. I recommend using the same approach for agent tables to remain consistent with the existing MySQL style. This means your Go code will scan `time.Time` values from the database (with `parseTime=true` in the DSN).
826: 
827: #### Handling Foreign Keys
828: 
829: **SQLite:**
830: 
831: ```sql
832: CREATE TABLE agent_audiences (
833:     tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
834:     ...
835: );
836: ```
837: 
838: **MySQL (with InnoDB):**
839: 
840: ```sql
841: CREATE TABLE `agent_audiences` (
842:     `tenant_id` INT NOT NULL,
843:     ...
844:     CONSTRAINT `fk_audience_tenant` FOREIGN KEY (`tenant_id`) REFERENCES `agent_tenants`(`id`) ON DELETE CASCADE
845: ) ENGINE=InnoDB;
846: ```
847: 
848: **PlanetScale/Vitess warning:** When targeting PlanetScale, you must remove all foreign key constraints because Vitess does not support them. Replace them with application-level referential integrity checks.
849: 
850: #### Discretionary Content: TINYINT vs BOOLEAN for SQLite Integer Booleans
851: 
852: SQLite uses `INTEGER` for booleans: `is_active INTEGER NOT NULL DEFAULT 1`. MySQL prefers `BOOLEAN` (which is `TINYINT(1)` under the hood):
853: 
854: ```sql
855: -- SQLite pattern
856: is_active INTEGER NOT NULL DEFAULT 1,
857: CHECK (is_active IN (0, 1))
858: 
859: -- MySQL pattern
860: is_active BOOLEAN NOT NULL DEFAULT TRUE,
861: ```
862: 
863: The Go code in SQLite scans these as `int` and checks `isActive == 1`. In MySQL, `BOOLEAN` values come back from `database/sql` as `bool` if you scan into a `bool` variable, or as `int64` (0 or 1) if you scan into an `int`.
864: 
865: #### CHECK Constraints in MySQL
866: 
867: If targeting MySQL 8.0.16+ or 8.4/9.0:
868: 
869: ```sql
870: status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'completed', 'failed'))
871: ```
872: 
873: MySQL 8.0.16+ enforces CHECK constraints. If targeting older versions or PlanetScale, remove the CHECK and validate in Go code.
874: 
875: ---
876: 
877: ### Chapter 8: Implementing MySQL Agent Methods
878: 
879: Once the migrations are written, implement the Go store methods. The good news: **SQLite and MySQL use the same placeholder syntax (`?`) and the same `LastInsertId()` pattern.** This means you can copy the SQLite implementation and only adjust:
880: 
881: 1. Timestamp functions (`CURRENT_TIMESTAMP` vs `strftime`)
882: 2. UPSERT syntax (`ON DUPLICATE KEY UPDATE` vs `ON CONFLICT ... DO UPDATE`)
883: 3. Identifier quoting (backticks vs no quotes)
884: 4. Column types in Scan (bool vs int for booleans)
885: 
886: #### Step-by-Step: Porting CreateAgentIntegration
887: 
888: **SQLite implementation** (`store/db/sqlite/agent.go:2600`):
889: 
890: ```go
891: func (d *DB) CreateAgentIntegration(ctx context.Context, integration *store.AgentIntegration) (*store.AgentIntegration, error) {
892:     stmt := `
893:         INSERT INTO agent_integrations (tenant_id, integration_type, label, config, is_active, created_at, updated_at)
894:         VALUES (?, ?, ?, ?, ?, CAST(strftime('%s','now') AS INTEGER), CAST(strftime('%s','now') AS INTEGER))
895:     `
896:     result, err := d.db.ExecContext(ctx, stmt,
897:         integration.TenantID, integration.IntegrationType, integration.Label,
898:         integration.Config, integration.IsActive,
899:     )
900:     if err != nil {
901:         return nil, fmt.Errorf("failed to create agent integration: %w", err)
902:     }
903:     id, err := result.LastInsertId()
904:     if err != nil {
905:         return nil, fmt.Errorf("failed to get last insert id: %w", err)
906:     }
907:     integration.ID = int32(id)
908:     return integration, nil
909: }
910: ```
911: 
912: **MySQL implementation:**
913: 
914: ```go
915: func (d *DB) CreateAgentIntegration(ctx context.Context, integration *store.AgentIntegration) (*store.AgentIntegration, error) {
916:     stmt := `
917:         INSERT INTO ` + "`agent_integrations`" + ` (` + "`tenant_id`, `integration_type`, `label`, `config`, `is_active`, `created_at`, `updated_at`" + `)
918:         VALUES (?, ?, ?, ?, ?, NOW(), NOW())
919:     `
920:     result, err := d.db.ExecContext(ctx, stmt,
921:         integration.TenantID, integration.IntegrationType, integration.Label,
922:         integration.Config, integration.IsActive,
923:     )
924:     if err != nil {
925:         return nil, fmt.Errorf("failed to create agent integration: %w", err)
926:     }
927:     id, err := result.LastInsertId()
928:     if err != nil {
929:         return nil, fmt.Errorf("failed to get last insert id: %w", err)
930:     }
931:     integration.ID = int32(id)
932:     return integration, nil
933: }
934: ```
935: 
936: **Changes:**
937: 1. `CAST(strftime('%s','now') AS INTEGER)` → `NOW()` (MySQL native timestamp function)
938: 2. Added backtick quotes around identifiers (MySQL convention)
939: 3. Everything else is identical — same placeholders, same `LastInsertId()`, same error handling
940: 
941: #### Step-by-Step: Porting GetAgentIntegration
942: 
943: **SQLite:**
944: 
945: ```go
946: func (d *DB) GetAgentIntegration(ctx context.Context, find *store.FindAgentIntegration) (*store.AgentIntegration, error) {
947:     query := "SELECT id, tenant_id, integration_type, label, config, is_active, created_at, updated_at FROM agent_integrations WHERE 1=1"
948:     args := []any{}
949:     if find.ID != nil {
950:         query += " AND id = ?"
951:         args = append(args, *find.ID)
952:     }
953:     if find.TenantID != nil {
954:         query += " AND tenant_id = ?"
955:         args = append(args, *find.TenantID)
956:     }
957:     // ...
958:     var integration store.AgentIntegration
959:     var isActive int
960:     err := d.db.QueryRowContext(ctx, query, args...).Scan(
961:         &integration.ID, &integration.TenantID, &integration.IntegrationType,
962:         &integration.Label, &integration.Config, &isActive,
963:         &integration.CreatedAt, &integration.UpdatedAt,
964:     )
965:     integration.IsActive = isActive == 1
966:     return &integration, nil
967: }
968: ```
969: 
970: **MySQL:**
971: 
972: ```go
973: func (d *DB) GetAgentIntegration(ctx context.Context, find *store.FindAgentIntegration) (*store.AgentIntegration, error) {
974:     query := "SELECT `id`, `tenant_id`, `integration_type`, `label`, `config`, `is_active`, `created_at`, `updated_at` FROM `agent_integrations` WHERE 1=1"
975:     args := []any{}
976:     if find.ID != nil {
977:         query += " AND `id` = ?"
978:         args = append(args, *find.ID)
979:     }
980:     if find.TenantID != nil {
981:         query += " AND `tenant_id` = ?"
982:         args = append(args, *find.TenantID)
983:     }
984:     // ...
985:     var integration store.AgentIntegration
986:     var isActive bool
987:     err := d.db.QueryRowContext(ctx, query, args...).Scan(
988:         &integration.ID, &integration.TenantID, &integration.IntegrationType,
989:         &integration.Label, &integration.Config, &isActive,
990:         &integration.CreatedAt, &integration.UpdatedAt,
991:     )
992:     integration.IsActive = isActive
993:     return &integration, nil
994: }
995: ```
996: 
997: **Changes:**
998: 1. Added backtick quotes
999: 2. `var isActive int` → `var isActive bool` (MySQL BOOLEAN columns can be scanned into bool)
1000: 3. `integration.IsActive = isActive == 1` → `integration.IsActive = isActive` (bool assignment)
1001: 
1002: #### Step-by-Step: Porting UPSERT (Rate Limiting)
1003: 
1004: **SQLite** uses `ON CONFLICT`:
1005: 
1006: ```go
1007: func (d *DB) GetOrCreateAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) (*store.AgentRateLimit, error) {
1008:     stmt := `
1009:         INSERT INTO agent_rate_limits (tenant_id, audience_type, client_ip, request_count, window_start)
1010:         VALUES (?, ?, ?, 0, CAST(strftime('%s','now') AS INTEGER))
1011:         ON CONFLICT(tenant_id, audience_type, client_ip) DO NOTHING
1012:     `
1013:     // ...
1014: }
1015: ```
1016: 
1017: **MySQL:**
1018: 
1019: ```go
1020: func (d *DB) GetOrCreateAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) (*store.AgentRateLimit, error) {
1021:     stmt := `
1022:         INSERT INTO ` + "`agent_rate_limits`" + ` (` + "`tenant_id`, `audience_type`, `client_ip`, `request_count`, `window_start`" + `)
1023:         VALUES (?, ?, ?, 0, UNIX_TIMESTAMP())
1024:         ON DUPLICATE KEY UPDATE ` + "`id`" + ` = ` + "`id`" + `
1025:     `
1026:     // ...
1027: }
1028: ```
1029: 
1030: The `ON DUPLICATE KEY UPDATE id = id` is a no-op update — it ensures the row exists without incrementing the counter. For the increment operation:
1031: 
1032: ```sql
1033: -- MySQL increment
1034: UPDATE `agent_rate_limits`
1035: SET `request_count` = `request_count` + 1
1036: WHERE `tenant_id` = ? AND `audience_type` = ? AND `client_ip` = ?;
1037: ```
1038: 
1039: #### The Bridge Delivery System: Skip or Port?
1040: 
1041: The bridge delivery system in `store/db/mysql/bridge.go` currently returns `store.ErrBridgeUnsupportedDatabase` for every method, and `SupportsBridgeDelivery()` returns `false`. The service layer checks this before calling bridge methods.
1042: 
1043: For a first porting pass, you have two options:
1044: 
1045: 1. **Keep it unsupported.** The service layer handles this gracefully by checking `SupportsBridgeDelivery()`. Users of MySQL simply cannot use the bridge integration feature.
1046: 
1047: 2. **Port it.** The bridge delivery system uses an outbox pattern with claim-based delivery. This requires implementing `SELECT ... FOR UPDATE` and complex state machine transitions. It's a significant effort.
1048: 
1049: **Recommendation:** Keep bridge delivery unsupported for the first porting pass. Mark it as `TODO` and move on to the higher-priority agent features. Bridge delivery is a premium feature; MySQL users may not need it.
1050: 
1051: ---
1052: 
1053: ### Chapter 9: DBaaS Deployment Considerations
1054: 
1055: Different MySQL-based database services have different compatibility profiles. Here is how to target each one.
1056: 
1057: #### Plain MySQL (8.4 / 9.0)
1058: 
1059: Full compatibility. Use all MySQL features: foreign keys, CHECK constraints, JSON type, `ON DUPLICATE KEY UPDATE`. This is the simplest target.
1060: 
1061: #### PlanetScale (Vitess)
1062: 
1063: PlanetScale runs Vitess. Key limitations:
1064: 
1065: - **No foreign keys.** Remove all `REFERENCES` and `ON DELETE CASCADE`. Handle referential integrity in Go code.
1066: - **No CHECK constraints.** Validate in Go code.
1067: - **Limited DDL.** Many `ALTER TABLE` operations require online schema changes. PlanetScale uses `vtctl` for DDL, not raw SQL. You may need to use PlanetScale's deploy request workflow for schema changes.
1068: - **No `LOCK TABLES`.** Use optimistic locking or application-level mutexes.
1069: - **No `SELECT ... FOR UPDATE` across shards.** Only works within a single shard.
1070: - **Index limitations.** Maximum index key length is 768 bytes (for `utf8mb4`).
1071: 
1072: For PlanetScale, your `agent_tenants` table would look like:
1073: 
1074: ```sql
1075: CREATE TABLE `agent_tenants` (
1076:     `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
1077:     `slug` VARCHAR(255) NOT NULL,
1078:     `company_name` VARCHAR(255) NOT NULL,
1079:     `guid` VARCHAR(36) NOT NULL,
1080:     `widget_key` VARCHAR(255) DEFAULT NULL,
1081:     `vertical` VARCHAR(100) DEFAULT NULL,
1082:     `is_active` BOOLEAN NOT NULL DEFAULT TRUE,
1083:     `processing_options` JSON DEFAULT NULL,
1084:     `allowed_domains` JSON DEFAULT NULL,
1085:     `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
1086:     `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
1087:     UNIQUE KEY `uk_agent_tenants_slug` (`slug`),
1088:     UNIQUE KEY `uk_agent_tenants_guid` (`guid`)
1089: ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
1090: 
1091: -- No foreign key on agent_audiences.tenant_id — enforced in Go.
1092: ```
1093: 
1094: And the `agent_events` table (which would otherwise have foreign keys to `agent_integrations` and `agent_tenants`):
1095: 
1096: ```sql
1097: CREATE TABLE `agent_events` (
1098:     `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
1099:     `tenant_id` INT NOT NULL,
1100:     `integration_id` INT NOT NULL,
1101:     `event_type` VARCHAR(50) NOT NULL,
1102:     `payload` JSON NOT NULL,
1103:     `status` VARCHAR(20) NOT NULL DEFAULT 'pending',
1104:     `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
1105:     -- No FOREIGN KEY constraints
1106: );
1107: ```
1108: 
1109: #### TiDB
1110: 
1111: TiDB is MySQL-compatible at the SQL level with some differences:
1112: 
1113: - Foreign keys are supported but not enforced by default (set `foreign_key_checks=1` or use TiDB 6.6+ which supports them)
1114: - CHECK constraints are parsed but not enforced (as of TiDB 7.x)
1115: - JSON type is supported
1116: - `ON DUPLICATE KEY UPDATE` is supported
1117: - No `SELECT ... FOR UPDATE` across regions (TiDB uses optimistic transactions by default)
1118: - Auto-increment works but is not guaranteed to be sequential (TiDB allocates batches)
1119: 
1120: TiDB is closer to MySQL compatibility than PlanetScale but still has enough differences to warrant a separate testing pass.
1121: 
1122: #### Choosing Your Target
1123: 
1124: If you are porting MySQL support for a specific customer or deployment, target their specific platform. If you want broad MySQL compatibility, target **plain MySQL 8.4** and document which features break on PlanetScale and TiDB.
1125: 
1126: ---
1127: 
1128: ## Part 4: Operations Engineering
1129: 
1130: ### Chapter 10: Migration Strategy
1131: 
1132: The bchat migration system uses Go's `embed.FS` to bundle SQL files with the binary. Migrations are versioned per directory:
1133: 
1134: ```go
1135: //go:embed migration
1136: var migrationFS embed.FS
1137: 
1138: func (s *Store) getMigrationBasePath() string {
1139:     return fmt.Sprintf("migration/%s/", s.profile.Driver)
1140: }
1141: ```
1142: 
1143: When the application starts, it:
1144: 1. Checks existing migration history in the database
1145: 2. Discovers migration files in the embedded FS
1146: 3. Applies any files with versions higher than the latest applied
1147: 4. If no migration history exists, applies `LATEST.sql` (the full schema)
1148: 
1149: #### Adding MySQL to the Migration System
1150: 
1151: Since MySQL migrations are in a separate directory (`store/migration/mysql/`), adding them is straightforward:
1152: 1. Create the migration directories (0.26–0.31)
1153: 2. Write the SQL files
1154: 3. Update `LATEST.sql` to include all new tables
1155: 4. The Go code automatically discovers them via `embed.FS`
1156: 
1157: No changes to the migrator code are needed. The migrator already supports MySQL via the `profile.Driver` path.
1158: 
1159: #### Schema Drift Detection
1160: 
1161: When adding MySQL migrations, you need a way to verify that the MySQL schema matches the SQLite/Postgres schema (for the same version). Without this check, it's easy to miss a table or column.
1162: 
1163: **Option A: Manual review.** Compare the SQL files side-by-side. This is what the existing codebase does.
1164: 
1165: **Option B: Automated schema comparison.** Use a tool like `go-migrate`'s diff or `atlasgo` to compare schemas:
1166: 
1167: ```bash
1168: # Compare SQLite and MySQL schemas
1169: atlas schema diff \
1170:   --from "sqlite://build/data/memos_dev.db" \
1171:   --to "mysql://memos:memospass@localhost:3306/memos" \
1172:   --dev-url "docker://sqlite/development"
1173: ```
1174: 
1175: **Option C: CI check.** In CI, spin up SQLite and MySQL containers, run migrations on both, then diff the schemas:
1176: 
1177: ```yaml
1178: # .github/workflows/schema-parity.yml (conceptual)
1179: jobs:
1180:   schema-parity:
1181:     services:
1182:       mysql:
1183:         image: mysql:8.4
1184:         env:
1185:           MYSQL_ROOT_PASSWORD: rootpass
1186:           MYSQL_DATABASE: memos
1187:     steps:
1188:       - run: ./build/memos --mode dev --driver mysql --dsn "root:rootpass@tcp(mysql:3306)/memos"
1189:       - run: ./build/memos --mode dev --driver sqlite --data /tmp/data
1190:       - run: |
1191:           sqlite3 /tmp/data/memos_dev.db .schema > /tmp/sqlite_schema.sql
1192:           mysqldump --no-data -h mysql -u root -prootpass memos > /tmp/mysql_schema.sql
1193:           # Diff the two schemas (excluding engine-specific types)
1194: ```
1195: 
1196: ---
1197: 
1198: ### Chapter 11: Testing Multi-Backend
1199: 
1200: Running the same test suite against SQLite, Postgres, and MySQL is the only way to ensure parity.
1201: 
1202: #### Test Architecture
1203: 
1204: The current test setup is described in `store/test/README.md`. The approach is:
1205: 
1206: 1. Define the test suite once (using the `store.Driver` interface)
1207: 2. Run the suite against each backend
1208: 3. Use environment variables to select the backend
1209: 
1210: For MySQL specifically:
1211: 
1212: ```go
1213: func TestAgentIntegrationMySQL(t *testing.T) {
1214:     dsn := os.Getenv("MYSQL_TEST_DSN")
1215:     if dsn == "" {
1216:         t.Skip("MYSQL_TEST_DSN not set")
1217:     }
1218:     db, err := mysql.NewDB(&profile.Profile{
1219:         Driver: "mysql",
1220:         DSN:    dsn,
1221:     })
1222:     if err != nil {
1223:         t.Fatalf("failed to create MySQL driver: %v", err)
1224:     }
1225:     defer db.Close()
1226: 
1227:     // Run the common test suite
1228:     RunAgentIntegrationTests(t, db)
1229: }
1230: ```
1231: 
1232: #### Testcontainers for MySQL
1233: 
1234: For CI without a persistent MySQL instance, use `testcontainers-go`:
1235: 
1236: ```go
1237: import (
1238:     "github.com/testcontainers/testcontainers-go"
1239:     "github.com/testcontainers/testcontainers-go/wait"
1240: )
1241: 
1242: func setupMySQLContainer(t *testing.T) (string, func()) {
1243:     ctx := context.Background()
1244:     req := testcontainers.ContainerRequest{
1245:         Image:        "mysql:8.4",
1246:         ExposedPorts: []string{"3306/tcp"},
1247:         Env: map[string]string{
1248:             "MYSQL_ROOT_PASSWORD": "test",
1249:             "MYSQL_DATABASE":      "memos_test",
1250:         },
1251:         WaitingFor: wait.ForLog("port: 3306  MySQL Community Server"),
1252:     }
1253:     mysqlC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
1254:         ContainerRequest: req,
1255:         Started:          true,
1256:     })
1257:     // ...
1258:     port, _ := mysqlC.MappedPort(ctx, "3306")
1259:     dsn := fmt.Sprintf("root:test@tcp(localhost:%s)/memos_test?parseTime=true&multiStatements=true", port.Port())
1260:     return dsn, func() { mysqlC.Terminate(ctx) }
1261: }
1262: ```
1263: 
1264: #### What to Test
1265: 
1266: For each agent method, test:
1267: 
1268: 1. **Create** — insert a row, verify it returns with an ID
1269: 2. **Read** — retrieve by ID, by tenant, by other find fields
1270: 3. **Update** — modify a row, verify the change
1271: 4. **Delete** — remove a row, verify it's gone
1272: 5. **Cascade** — delete a parent (tenant), verify children are removed (CASCADE)
1273: 6. **Duplicate** — attempt to insert a duplicate, verify the unique constraint fires

(Output capped at 50 KB. Showing lines 1-1273. Use offset=1274 to continue.)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/TUTORIAL_CX.md"
}
```

**Output:**
```
<path>/home/chaschel/Documents/go/bchat/TUTORIAL_CX.md</path>
<type>file</type>
<content>
1: # Customer Experience & AI-Powered Support: Rethinking Tickets, Knowledge, and Conversations
2: 
3: > **Audience:** Product managers, support engineers, and developers who want to understand how an AI-first platform can replace the Intercom + Jira stack — not by doing what they do better, but by making tickets invisible to customers.
4: >
5: > **Prerequisites:** Familiarity with customer support workflows (ticketing, SLAs, agent handoff). Completion of `TUTORIAL.md` (webhooks) and/or `TUTORIAL_DATABASE.md` helps but is not required.
6: >
7: > **Time to work through:** 4–6 hours depending on playbook depth
8: >
9: > **Philosophy:** This tutorial is 70% workflow/UX design thinking and 30% code mechanics. The goal is not to recreate Intercom or Jira in bchat — it is to question every assumption they made and build something better. We do not outsource thinking to AI agents; every design decision starts with a customer outcome.
10: 
11: ---
12: 
13: ## Table of Contents
14: 
15: - [Part 1: The Vision — Reimagining Customer Support](#part-1-the-vision--reimagining-customer-support)
16:   - [Chapter 1: What Intercom and Jira Got Wrong](#chapter-1-what-intercom-and-jira-got-wrong)
17:   - [Chapter 2: The bchat Thesis](#chapter-2-the-bchat-thesis)
18:   - [Chapter 3: A Day in the Life](#chapter-3-a-day-in-the-life)
19: - [Part 2: The Dual-Audience System — Internal vs External](#part-2-the-dual-audience-system--internal-vs-external)
20:   - [Chapter 4: The Audience Model](#chapter-4-the-audience-model)
21:   - [Chapter 5: The External Customer Flow](#chapter-5-the-external-customer-flow)
22:   - [Chapter 6: The Internal Agent Flow](#chapter-6-the-internal-agent-flow)
23: - [Part 3: Knowledge Management — The Heart of AI Support](#part-3-knowledge-management--the-heart-of-ai-support)
24:   - [Chapter 7: KB.MD — The Single Source of Truth](#chapter-7-kbmd--the-single-source-of-truth)
25:   - [Chapter 8: POLICY.MD — The Agent's Constitution](#chapter-8-policymd--the-agents-constitution)
26:   - [Chapter 9: SCRIPT.MD — Conversation Architecture](#chapter-9-scriptmd--conversation-architecture)
27:   - [Chapter 10: RAG vs Structured Mode](#chapter-10-rag-vs-structured-mode)
28: - [Part 4: Observational Memory & Agent Learning](#part-4-observational-memory--agent-learning)
29:   - [Chapter 11: How Observational Memory Works](#chapter-11-how-observational-memory-works)
30:   - [Chapter 12: From Observations to Learning](#chapter-12-from-observations-to-learning)
31:   - [Chapter 13: The Verifier and Sanitizer](#chapter-13-the-verifier-and-sanitizer)
32: - [Part 5: Ticket Management — Rethinking Workflows](#part-5-ticket-management--rethinking-workflows)
33:   - [Chapter 14: The Current Ticket System](#chapter-14-the-current-ticket-system)
34:   - [Chapter 15: The Ticketless Ideal](#chapter-15-the-ticketless-ideal)
35:   - [Chapter 16: Escalation Without Tickets](#chapter-16-escalation-without-tickets)
36:   - [Chapter 17: Leads as Follow-Up](#chapter-17-leads-as-follow-up)
37: - [Part 6: Testing Effectiveness](#part-6-testing-effectiveness)
38:   - [Chapter 18: Simulation as QA](#chapter-18-simulation-as-qa)
39:   - [Chapter 19: The Analysis Rubric](#chapter-19-the-analysis-rubric)
40:   - [Chapter 20: Measuring CX Improvement](#chapter-20-measuring-cx-improvement)
41: - [Part 7: Playbooks](#part-7-playbooks)
42:   - [Chapter 21: Playbook — KB-Driven Support from Scratch](#chapter-21-playbook--kb-driven-support-from-scratch)
43:   - [Chapter 22: Playbook — Ticketless Escalation](#chapter-22-playbook--ticketless-escalation)
44:   - [Chapter 23: Playbook — Continuous Improvement](#chapter-23-playbook--continuous-improvement)
45:   - [Chapter 24: The Delight Checklist](#chapter-24-the-delight-checklist)
46: - [Appendices](#appendices)
47:   - [Appendix A: KB.MD Writing Guide](#appendix-a-kbmd-writing-guide)
48:   - [Appendix B: POLICY.MD Reference](#appendix-b-policymd-reference)
49:   - [Appendix C: Simulation Scenarios](#appendix-c-simulation-scenarios)
50:   - [Appendix D: Analysis Rubric](#appendix-d-analysis-rubric)
51:   - [Appendix E: OM Configuration Reference](#appendix-e-om-configuration-reference)
52:   - [Appendix F: API Endpoint Reference for CX](#appendix-f-api-endpoint-reference-for-cx)
53: 
54: ---
55: 
56: ## Part 1: The Vision — Reimagining Customer Support
57: 
58: ### Chapter 1: What Intercom and Jira Got Wrong
59: 
60: Intercom and Jira are the two most popular tools in the customer support stack. They are also deeply flawed. To build something better, we must understand exactly where they fail.
61: 
62: #### Intercom's Problems
63: 
64: Intercom is a live chat platform. Its core assumption: **a human agent must eventually read every conversation.**
65: 
66: | Problem | Manifestation | Impact |
67: |---------|--------------|--------|
68: | **Conversational amnesia** | Each chat is ephemeral. When a new agent picks up, the customer repeats everything. | Customer frustration, 2x handling time |
69: | **Knowledge silos** | Answers live in agents' heads or separate KB tools. No systematic feedback loop. | Inconsistent answers, onboarding hell |
70: | **Reactive-only** | Agent waits for customer to initiate. No proactive follow-up. | Missed upsell, unresolved issues |
71: | **Scale = people** | To handle more conversations, hire more agents. | Linear cost scaling, quality variance |
72: 
73: Intercom's answer to these problems was **Operator** (their AI bot), but it's bolted on top of a human-first architecture. The bot handles simple queries and hands off to humans for complex ones — but the architecture still assumes the human is the real answer and the bot is a pre-filter.
74: 
75: #### Jira's Problems
76: 
77: Jira is a ticketing system. Its core assumption: **every issue must be a ticket with a number, a status, and an assignee.**
78: 
79: | Problem | Manifestation | Impact |
80: |---------|--------------|--------|
81: | **Ticket as paper** | A ticket is a static snapshot. It doesn't capture the conversation that led to it. | Lost context, duplicate tickets |
82: | **Customer-facing numbers** | "Your ticket TKT-45921 has been updated." The customer doesn't care about the number. | Alienating, enterprise-speak |
83: | **Friction to create** | Creating a ticket requires fields, priorities, and triage. | Support team becomes data entry |
84: | **Jira is for Jira** | Tickets serve the organization's workflow, not the customer's outcome. | Misaligned incentives |
85: 
86: The combination of Intercom + Jira creates a grim workflow:
87: 
88: 1. Customer chats with Intercom bot → bot answers FAQ or escalates to human
89: 2. Human agent chats in Intercom → realizes it's complex → creates a Jira ticket
90: 3. Customer is told "I've created ticket TKT-45921, someone will get back to you"
91: 4. Customer waits. Ticket moves through statuses. Nobody updates the customer.
92: 5. Customer chats again → "I have ticket TKT-45921" → new agent has no context → repeat
93: 
94: #### What bchat Does Differently
95: 
96: bchat starts from a different premise: **the AI is the primary support agent. Humans are backup for edge cases.** Every architectural decision flows from this:
97: 
98: - The KB is machine-readable (annotations), not just human-readable markdown
99: - Every conversation is a data point for analysis and improvement
100: - Tickets are created only when escalation is needed — and they carry the full conversation context
101: - The customer never needs to know a ticket number exists
102: 
103: ---
104: 
105: ### Chapter 2: The bchat Thesis
106: 
107: The bchat approach to customer support can be summarized in four principles:
108: 
109: #### Principle 1: Conversation Is the Record
110: 
111: In Intercom + Jira, the ticket is the record and the conversation is ephemeral. In bchat, **the conversation is the record** and tickets are annotations on conversations.
112: 
113: ```go
114: // The chat session contains everything:
115: type AgentSession struct {
116:     // ... standard session fields
117:     
118:     // Conversation history — the full record
119:     messages TEXT DEFAULT '[]'  // JSON array of all messages
120:     
121:     // Derived state
122:     phase             string   // "triage", "resolution", "completed"
123:     current_intent    string   // detected intent from policy
124:     urgency_level     int      // computed urgency
125:     detected_service  string   // matched service from KB
126:     
127:     // Escalation marker
128:     is_completed      bool
129:     completion_reason string   // "resolved", "escalated", "abandoned"
130: }
131: ```
132: 
133: When an escalation happens, the ticket is created by copying the conversation context into a memo linked to the ticket:
134: 
135: ```go
136: // In service.go (simplified):
137: func (s *Service) CreateEscalationTicket(ctx context.Context, session *AgentSession, reason string) (*store.Ticket, error) {
138:     // 1. Create a memo with the full conversation context
139:     memo := s.store.CreateMemo(ctx, &store.Memo{
140:         Content:   formatEscalationContext(session, reason),
141:         Visibility: "PROTECTED",  // only internal agents
142:     })
143:     
144:     // 2. Create a ticket linked to the memo
145:     ticket := s.store.CreateTicket(ctx, &store.Ticket{
146:         Type:        "agent_escalation",
147:         Description: "/m/" + memo.UID,
148:         Status:      "OPEN",
149:         Priority:    "HIGH",
150:         Tags:        []string{"escalated"},
151:     })
152:     return ticket, nil
153: }
154: ```
155: 
156: The customer never sees "TKT-45921." They see their ongoing conversation, now handled by a human who has full context.
157: 
158: #### Principle 2: Resolve Before Escalating
159: 
160: The default path is **AI resolution**. Escalation is a specific, measured decision based on:
161: 
162: - **Intent detection**: Is the customer's intent in the KB? If not, escalate.
163: - **Confidence threshold**: Is the AI confident in its response? Below 0.85, consider escalation.
164: - **Urgency/emergency**: Safety-related queries always go to a human.
165: - **Customer persistence**: If the customer asks the same question three times despite correct answers, escalate.
166: 
167: ```go
168: // In service.go (simplified escalation decision):
169: func (s *Service) shouldEscalate(score *store.EscalationScore) bool {
170:     return score.ShouldEscalate ||
171:         score.Confidence < 0.85 ||
172:         score.Urgency > 7
173: }
174: ```
175: 
176: #### Principle 3: Close the Loop
177: 
178: Every conversation generates data:
179: - **Observation log**: What was discussed, what the customer wanted, what was resolved
180: - **Analysis scores**: How well the AI performed (intent recognition, service alignment, policy compliance, etc.)
181: - **Learning memory**: Common issues, improvement areas, pending suggestions
182: 
183: This data feeds back into the KB. If customers keep asking about a service not in the KB, that's a signal to add it.
184: 
185: #### Principle 4: The Customer's Experience Is the Only Metric
186: 
187: Not ticket volume, not first-response time, not SLA compliance. The metrics that matter:
188: 
189: | Metric | What It Measures | bchat Equivalent |
190: |--------|-----------------|------------------|
191: | Resolution rate | % of conversations resolved without escalation | `completion_reason = 'resolved'` |
192: | Time to resolve | How long to answer the customer's question | Message count + LLM latency |
193: | Customer effort | How hard did the customer have to work? | Message count (fewer = better), rephrasing rate |
194: | Follow-through | Did the customer act on the AI's advice? | Lead conversion rate |
195: | Sentiment | Was the customer satisfied? | Analysis sentiment category |
196: 
197: ---
198: 
199: ### Chapter 3: A Day in the Life
200: 
201: Let's walk through three scenarios to see how bchat works in practice. Each scenario reveals a different architectural capability.
202: 
203: #### Scenario A: External Customer — Plumbing Emergency
204: 
205: A homeowner named Sarah has a burst pipe at 2 AM. She visits the website of "Apex Restoration" (a bchat tenant) and clicks the chat widget.
206: 
207: **What happens on the backend:**
208: 
209: 1. A POST to `/api/v1/agent/apex-restoration/chat` creates a new session with `audience_type = 'external'`
210: 2. The system loads Apex's AudienceConfig for the `external` audience: KB services, policies, and script
211: 3. Sarah types "I have water pouring from my ceiling"
212: 
213: **Intent detection** (from policy):
214: 
215: ```
216: The LLM classifies: intent = "water_extraction", emergency = true
217: ```
218: 
219: **KB response** (from structured content):
220: 
221: ```
222: System prompt builds:
223:   Role: Customer Service Representative
224:   Tone: Professional, empathetic
225:   Service: Water Extraction — 24/7 emergency response
226:   Emergency phone: (555) 123-4567
227:   Script Stage: Opening → Greet the customer → Ask how you can help
228: ```
229: 
230: **AI response:** "I'm sorry to hear about the burst pipe, Sarah. This is an emergency situation. Our water extraction team is available 24/7. Would you like me to send someone right away?"
231: 
232: 4. Sarah confirms. The AI captures her contact info.
233: 5. After the conversation, the system checks `shouldEscalate`:
234:    - Intent matched ✓
235:    - Service found ✓
236:    - No escalation needed
237: 6. **Lead captured:** Sarah's info is stored as a lead for follow-up.
238: 
239: **Sarah's experience:** She got immediate help at 2 AM, didn't wait for a human, didn't open a ticket, didn't repeat herself. She'll get a follow-up email tomorrow asking if everything was resolved.
240: 
241: **No ticket was created.** The conversation is the record.
242: 
243: #### Scenario B: Escalation to Human Agent
244: 
245: A customer named James is talking to the AI about a complex insurance claim. The AI detects the intent but the customer is not satisfied.
246: 
247: **Escalation path:**
248: 
249: 1. `shouldEscalate()` returns true (customer has asked "Can I speak to a human?" three times)
250: 2. `CreateEscalationTicket()` generates:
251:    - A memo with the full conversation transcript
252:    - A ticket with `type = "agent_escalation"`, `priority = "HIGH"`, `tags = ["escalated"]`
253: 3. The ticket appears on the internal Kanban board at `/tickets`
254: 4. An agent picks up the ticket, sees the full conversation context in the linked memo
255: 5. The agent joins the conversation via the bridge handoff system
256: 6. The customer is told "I've connected you with a specialist who has all the details of your situation"
257: 
258: **James's experience:** He never saw a ticket number. He never repeated himself. The human agent knew everything.
259: 
260: #### Scenario C: Internal Agent — Managing Support Queue
261: 
262: An internal support agent logs into bchat as an authenticated user. They see:
263: 
264: - `/tickets` — Kanban board with OPEN, IN_PROGRESS, CLOSED columns
265: - Each card shows: title, priority chip (LOW/MEDIUM/HIGH), type chip (TASK/agent_escalation), escalation badge, assignee avatar
266: - Escalated tickets are at the top with red badges
267: - Clicking a ticket opens a side panel with full context from the memo
268: 
269: **Agent actions:**
270: - Drag from OPEN to IN_PROGRESS to self-assign
271: - Reply in the conversation (memo comments)
272: - Drag to CLOSED when resolved
273: - View analytics: resolution rate, escalation rate, common intents
274: 
275: **The agent's tool is not Jira.** It's a conversational interface augmented by Kanban for triage. The ticket exists only for the agent's workflow — the customer never interacts with it.
276: 
277: ---
278: 
279: ## Part 2: The Dual-Audience System — Internal vs External
280: 
281: ### Chapter 4: The Audience Model
282: 
283: bchat's audience model is the key architectural insight that enables replacing both Intercom and Jira with one system.
284: 
285: #### One Tenant, Two Audiences
286: 
287: Every tenant has at least two audiences:
288: 
289: | Audience | Users | Auth | Chat Endpoint | Features |
290: |----------|-------|------|---------------|----------|
291: | `external` | Anonymous website visitors | Widget embed + HMAC token | `POST /api/v1/agent/:slug/chat` | AI-only, lead capture, optional bridge handoff |
292: | `internal` | Authenticated staff | JWT with role | `POST /api/v1/agent/:slug/chat/internal` | AI + human, full ticket system, Kanban, analytics |
293: 
294: #### Audience Configuration
295: 
296: Each audience has its own configuration in `agent_audiences`:
297: 
298: ```go
299: type AgentAudience struct {
300:     ID        int32
301:     TenantID  int32
302:     AudienceType string // "external" | "internal"
303:     
304:     // Identity
305:     Role    string // e.g., "Customer Service Representative"
306:     Tone    string // "Professional, empathetic"
307:     BrandVoice string
308:     Guidelines string
309:     
310:     // Contact
311:     EmergencyPhone string
312:     SecondaryPhones []string
313:     
314:     // Behavior
315:     RateLimitRPM              int  // external: 60, internal: 120
316:     RequireContactOnFallback  bool // external: true (capture email before handoff)
317:     MaxMessageLength          int  // external: 2000, internal: 4000
318:     EmergencyUrgencyThreshold int  // 4 = >50% threshold for human involvement
319:     EscalationConfidenceThreshold float64 // 0.85
320: }
321: ```
322: 
323: **The same tenant can have dramatically different configs for each audience:**
324: 
325: ```markdown
326: <!-- For external customers: helpful, concise, empathetic -->
327: @identity
328: - Role: Customer Service Representative
329: - Tone: Warm and professional
330: - Guidelines: Never ask for payment info in chat. Always offer to schedule a callback.
331: 
332: <!-- For internal agents: tactical, precise, data-rich -->
333: @identity
334: - Role: Support Operations Analyst
335: - Tone: Clear and actionable
336: - Guidelines: Always include ticket reference. Verify SLA compliance before committing.
337: ```
338: 
339: #### Why Audiences Matter for CX
340: 
341: The audience system enables three critical CX capabilities:
342: 
343: 1. **Different KB for different audiences.** External customers see service descriptions and FAQs. Internal agents see escalation procedures and compliance rules — in the same KB.MD file, just in different audience sections.
344: 
345: 2. **Different escalation paths.** External customers escalate to a ticket (invisible to them). Internal agents escalate to a supervisor with full context.
346: 
347: 3. **Different rate limits.** External users are limited to 60 RPM per IP. Internal agents get 120 RPM because they need to test and iterate.
348: 
349: #### The KB.MD Audience Section
350: 
351: In the KB.MD file, you can mark content for specific audiences:
352: 
353: ```markdown
354: <!-- @audience: external -->
355: 
356: <!-- @service: water_extraction, emergency: true -->
357: ## Water Extraction
358: 24/7 emergency response for standing water removal...
359: 
360: We serve all of Cook County within 2 hours.
361: 
362: <!-- @faq: pricing -->
363: ## How much does it cost?
364: Costs vary based on extent of damage. We provide free estimates.
365: 
366: ---
367: 
368: <!-- @audience: internal -->
369: 
370: <!-- @service: water_extraction -->
371: ## Water Extraction — Internal Protocol
372: ### Escalation Criteria
373: - If moisture detected in walls → dispatch structural team
374: - If sewage involved → dispatch biohazard team
375: - If customer has filed insurance claim → route to claims department
376: ```
377: 
378: When the system loads config for `audience_type = 'external'`, it only loads the external sections. When loading for `audience_type = 'internal'`, it loads the internal sections. This is enforced at the database query level via `ListAgentServices(find.FindAgentService{TenantID: tenantID, AudienceType: "external"})`.
379: 
380: ---
381: 
382: ### Chapter 5: The External Customer Flow
383: 
384: The external customer flow is the primary experience. This is where bchat replaces Intercom.
385: 
386: #### Entry Points
387: 
388: External customers can start a conversation from:
389: 
390: - **Embedded widget**: A JavaScript widget on the tenant's website (via `widget_key` and `guid`)
391: - **Direct API**: `POST /api/v1/agent/:slug/chat` with minimal parameters
392: - **Bridge channel**: External messaging platforms (Twilio, WhatsApp, Messenger) via the bridge delivery system
393: 
394: #### Session Lifecycle
395: 
396: ```
397: 1. Customer opens widget → POST /chat
398:    ├── Tenant resolved by slug
399:    ├── Audience set to "external"
400:    ├── Session created (in-memory or DB)
401:    └── HMAC session token generated for transcript access
402: 
403: 2. Customer sends message → POST /chat with session_id
404:    ├── Rate limit check (60 RPM per IP)
405:    ├── Idempotency check (ClientMessageID)
406:    ├── Intent detection (LLM classifies against policy intents)
407:    ├── KB lookup (structured content or RAG retrieval)
408:    ├── LLM response generation
409:    ├── Observer triggers (async)
410:    │   ├── Check token threshold for buffer
411:    │   ├── Run Observer LLM
412:    │   ├── Update observation log
413:    │   └── Check reflector threshold (compress if needed)
414:    └── Response streamed back to widget
415: 
416: 3. Customer satisfied → chat ends
417:    ├── completion_reason = "resolved"
418:    ├── Lead captured (if contact info provided)
419:    └── Session stored for analysis
420: 
421: 4. Customer not satisfied → escalation
422:    ├── shouldEscalate() = true
423:    ├── Escalation ticket created (invisible to customer)
424:    ├── Bridge handoff queued (if bridge enabled)
425:    └── Customer notified: "A specialist will join shortly"
426: ```
427: 
428: #### The No-Account Promise
429: 
430: External customers never need an account. The session is identified by a session ID generated server-side and returned in the chat response. The HMAC token allows the customer to view their transcript later without authentication:
431: 
432: ```go
433: // In service.go — external chat handler
434: func (s *Service) ChatExternal(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
435:     // ...
436:     session, err := s.store.CreateAgentSession(ctx, &store.AgentSession{
437:         TenantID:     tenantID,
438:         AudienceType: "external",
439:         Phase:        "triage",
440:     })
441:     
442:     // HMAC session token for transcript access
443:     sessionToken := generateSessionToken(session.ID, tenant.Key)
444:     
445:     return &ChatResponse{
446:         SessionID:    session.ID,
447:         SessionToken: sessionToken,
448:         Message:      response,
449:     }, nil
450: }
451: ```
452: 
453: This is critical for CX: **zero friction to start a conversation.** No signup, no ticket creation, no "before we begin" form.
454: 
455: #### Rate Limiting with Grace
456: 
457: External rate limiting in bchat uses a per-IP, per-tenant sliding window:
458: 
459: ```go
460: // Current behavior (simplified):
461: func (s *Service) checkRateLimit(ctx context.Context, tenantID int32, clientIP string) (bool, error) {
462:     allowed, err := s.store.CheckAndIncrementAgentRateLimit(ctx, tenantID, "external", clientIP, 60)
463:     if !allowed {
464:         // Return a friendly message, not a 429
465:         return false, fmt.Errorf("I'm receiving a lot of requests right now. Please wait a moment and try again.")
466:     }
467:     return true, nil
468: }
469: ```
470: 
471: The rate limit response is designed to be customer-friendly — not a cryptic HTTP status code but a human-readable message from the AI. This is a deliberate CX choice: the customer should never see an error page from the chat widget.
472: 
473: #### Idempotency for Safe Retry
474: 
475: External customers on unreliable connections (mobile, public WiFi) may send the same message twice. The `ClientMessageID` field ensures exactly-once processing:
476: 
477: ```go
478: type ChatRequest struct {
479:     SessionID       string `json:"sessionId"`
480:     Message         string `json:"message"`
481:     ClientMessageID string `json:"clientMessageId"`  // Idempotency key
482:     
483:     // If the same ClientMessageID is sent twice,
484:     // the second call returns the cached response.
485: }
486: ```
487: 
488: ---
489: 
490: ### Chapter 6: The Internal Agent Flow
491: 
492: Internal agents (support staff, admins, supervisors) have a different experience designed for productivity.
493: 
494: #### Entry Points
495: 
496: Internal agents access bchat via:
497: 
498: - **Web UI**: Full dashboard at `/` with Kanban board, analytics, and configuration
499: - **Internal chat API**: `POST /api/v1/agent/:slug/chat/internal` (authenticated)
500: - **Memo system**: Full memo (note-taking app) for documentation
501: 
502: #### The Kanban Board
503: 
504: The ticket Kanban at `/tickets` is the internal agent's primary tool. However, it is deliberately **not Jira**:
505: 
506: | Jira | bchat Kanban | Why |
507: |------|-------------|-----|
508: | Required fields on create | Minimal required: title only | Reduce friction |
509: | Custom workflows per project | Three columns: OPEN, IN_PROGRESS, CLOSED | Simplicity |
510: | Status transitions with permissions | Drag and drop | Intuitive |
511: | Email notifications | Real-time SSE notifications | Immediate |
512: | Customer portal | No customer portal | Tickets are internal |
513: 
514: The Kanban board uses `@dnd-kit` for drag-and-drop:
515: 
516: ```typescript
517: // In TicketKanban.tsx
518: const columns = [
519:     { id: 'OPEN', title: 'Open', tickets: openTickets },
520:     { id: 'IN_PROGRESS', title: 'In Progress', tickets: inProgressTickets },
521:     { id: 'CLOSED', title: 'Closed', tickets: closedTickets },
522: ];
523: 
524: // Dragging to a different column calls:
525: const handleStatusChange = (ticketId: number, newStatus: string) => {
526:     updateTicket({ id: ticketId, status: newStatus });
527: };
528: ```
529: 
530: #### The Escalation View
531: 
532: The escalation view is a filtered Kanban showing only `agent_escalation` type tickets:
533: 
534: ```go
535: // In ticket_service.go
536: func (s *Service) ListTicketAssignees(c echo.Context) error {
537:     // Superuser / admin only
538:     // Returns list of users for assignee dropdown
539: }
540: ```
541: 
542: Agents see escalated tickets with:
543: - **Title**: "Escalation from [Customer Name]"
544: - **Priority**: HIGH (auto-set by escalation logic)
545: - **Type badge**: "Escalation" (red)
546: - **Tags**: "escalated" (shown as a badge in the UI)
547: - **Memo link**: Click to open the full conversation context
548: 
549: #### Analytics for Internal Agents
550: 
551: Internal agents have access to analytics (via the analysis system):
552: 
553: - **Resolution rate**: % of conversations resolved without escalation
554: - **Average messages per conversation**: Indicator of efficiency (fewer = better)
555: - **Common intents**: What customers are asking about most
556: - **Lead conversion rate**: How many leads turned into actual business
557: - **Sentiment trend**: Is customer satisfaction improving or declining?
558: 
559: These analytics are generated from `AgentAnalysisResult` records:
560: 
561: ```go
562: type AgentAnalysisResult struct {
563:     ID               string
564:     TenantID         int32
565:     ConversationID   string
566:     ConversationType string // "simulation" or "chat"
567:     UserID           int32
568:     Score            int    // 0-100
569:     Grade            string // "A", "B", "C", "D", "F"
570:     Breakdown        AnalysisBreakdown  // 6 categories
571:     Issues           []AnalysisIssue
572:     Suggestions      []string
573: }
574: ```
575: 
576: ---
577: 
578: ## Part 3: Knowledge Management — The Heart of AI Support
579: 
580: ### Chapter 7: KB.MD — The Single Source of Truth
581: 
582: The Knowledge Base is the most important file in the system. It determines everything the AI knows, every service it can offer, and every answer it can give.
583: 
584: #### Structure
585: 
586: A well-written KB.MD has three principles:
587: 
588: 1. **Annotate everything.** Every service, FAQ, exclusion, and section should have a `@type` annotation.
589: 2. **Write for the AI, not just for humans.** The AI reads the raw content. Use clear, unambiguous language. Avoid marketing fluff.
590: 3. **Think in terms of intents.** What is the customer trying to do? Map content to those intents.
591: 
592: ```markdown
593: # Apex Restoration Services
594: 
595: <!-- @audience: external -->
596: 
597: <!-- @service: water_extraction, emergency: true, priority: 1 -->
598: ## Water Extraction
599: We provide 24/7 emergency water extraction services.
600: 
601: - **Response time:** Within 2 hours for all of Cook County
602: - **Equipment:** Industrial-grade vacuums, air movers, dehumidifiers
603: - **Process:** Assessment → Extraction → Drying → Sanitization → Restoration
604: 
605: **What to do while waiting:**
606: 1. Turn off water at the main valve if safe
607: 2. Move valuables to dry area
608: 3. Do NOT use electrical appliances near standing water
609: 
610: <!-- @faq: water_damage_insurance -->
611: ## Does insurance cover water damage?
612: Most homeowner's insurance policies cover sudden and accidental water damage...
613: ```
614: 
615: #### Why Annotations Matter for CX
616: 
617: Annotations are not metadata — they are **behavior directives**:
618: 
619: - `emergency: true` — The AI will prioritize this service and use urgent language
620: - `priority: 1` — This service appears first in the system prompt
621: - `@faq: pricing` — The AI knows this is a FAQ, not a service offering, and will answer differently
622: 
623: Without annotations, the AI treats everything equally. With annotations, the AI understands the **intent and importance** of each piece of content.
624: 
625: #### What Happens When KB Is Missing
626: 
627: If a customer asks about a service not in the KB, three things happen:
628: 
629: 1. **Intent detection fails** — the AI does not recognize the intent
630: 2. **Service matching fails** — no service is found in structured content or RAG
631: 3. **Fallback behavior** — the AI either:
632:    - Says "I don't have information about that" (safe but unhelpful)
633:    - Guesses (dangerous — hallucination risk)
634:    - Escalates to a human (best for CX)
635: 
636: The best KB is one that covers every service the tenant offers. The analysis system tracks "coverage gaps" as `CommonIssues`:
637: 
638: ```go
639: type CommonIssue struct {
640:     Category    string // "coverage_gap"
641:     Description string // "Customer asked about mold remediation, not in KB"
642:     Occurrences int
643:     LastSeen    string
644: }
645: ```
646: 
647: This feeds into the learning memory, alerting the tenant admin to add missing content.
648: 
649: ---
650: 
651: ### Chapter 8: POLICY.MD — The Agent's Constitution
652: 
653: While the KB defines **what** the AI knows, the policy defines **how** the AI behaves.
654: 
655: #### Policy Structure
656: 
657: ```markdown
658: <!-- @audience: external -->
659: 
660: <!-- @identity -->
661: - Role: Customer Service Representative
662: - Tone: Warm, professional, empathetic
663: - Brand Voice: We are experts but not condescending. We educate without overwhelming.
664: - Guidelines:
665:   - Never make promises about pricing without a supervisor
666:   - Always ask for permission before scheduling a visit
667:   - If the customer is angry, acknowledge their frustration first
668: 
669: <!-- @intent: schedule_service -->
670: ## Schedule Service
671: Customer wants to book an appointment for a specific service.
672: 
673: Steps:
674: 1. Confirm the service they need
675: 2. Check coverage area (see Coverage section)
676: 3. Offer available time slots
677: 4. Collect contact information
678: 5. Confirm the appointment
679: 
680: <!-- @intent: emergency -->
681: ## Emergency
682: Customer has an urgent situation requiring immediate response.
683: 
684: Steps:
685: 1. Confirm safety (are they in immediate danger?)
686: 2. If yes: instruct them to call 911, then contact emergency dispatch
687: 3. If no: provide emergency phone number
688: 4. Dispatch emergency team
689: 5. Follow up within 1 hour
690: 
691: <!-- @rule: no_pricing_over_chat -->
692: ## Pricing Rule
693: Never provide exact pricing over chat. Always say "pricing varies based on the specific situation" and offer a free estimate.
694: ```
695: 
696: #### How Policy Drives Behavior
697: 
698: The policy is not a static document. It is parsed into structured data:
699: 
700: ```go
701: type AgentIntent struct {
702:     Code        string // "schedule_service"
703:     Name        string // "Schedule Service"
704:     Category    string // "booking"
705:     Description string // "Customer wants to book..."
706:     Action      string // "schedule"
707:     Urgency     int    // 5
708:     // ...
709: }
710: 
711: type AgentRule struct {
712:     Code        string // "no_pricing_over_chat"
713:     Name        string // "Pricing Rule"
714:     Description string // "Never provide exact pricing..."
715:     Priority    int    // 10 (higher = more important)
716:     // ...
717: }
718: ```
719: 
720: These are injected into the system prompt as structured sections:
721: 
722: ```
723: === IDENTITY ===
724: Role: Customer Service Representative
725: Tone: Warm, professional, empathetic
726: ...
727: 
728: === INTENTS ===
729: When a customer wants to schedule a service:
730:   1. Confirm the service they need
731:   2. Check coverage area
732:   ...
733: 
734: === RULES ===
735: - Never provide exact pricing over chat.
736: - Always ask for permission before scheduling.
737: ```
738: 
739: **The AI does not have to learn the policy.** The policy is literally in the prompt. This is why bchat works without fine-tuning — the behavioral rules are context, not weights.
740: 
741: #### Why Policy Matters for CX
742: 
743: A good policy prevents the most common AI support failures:
744: 
745: | Failure | Cause | Policy Solution |
746: |---------|-------|----------------|
747: | AI invents pricing | No pricing rule | `@rule: no_pricing_over_chat` |
748: | AI is rude after customer is angry | No tone guidelines | "Acknowledge frustration first" |
749: | AI schedules without permission | No permission rule | "Always ask before scheduling" |
750: | AI escalates unnecessarily | No escalation criteria | Confidence threshold + intent matching |
751: 
752: ---
753: 
754: ### Chapter 9: SCRIPT.MD — Conversation Architecture
755: 
756: The script defines the conversation flow. It is the closest thing to a "playbook" for the AI.
757: 
758: #### Script Structure
759: 
760: ```markdown
761: <!-- @audience: external -->
762: 
763: ## Stage: Opening
764: - Greet the customer warmly
765: - Introduce yourself and the company
766: - Ask how you can help today
767: 
768: ## Stage: Triage
769: - Ask clarifying questions to identify the issue
770: - Match to a service or FAQ from the knowledge base
771: - If urgent (emergency), skip to Emergency Protocol
772: - If unclear, ask: "Can you tell me more about what's happening?"
773: 
774: ## Stage: Resolution
775: - Provide solution based on KB
776: - Confirm the customer is satisfied
777: - Offer next steps (schedule service, send info, etc.)
778: 
779: ## Stage: Closing
780: - Thank the customer
781: - Ask if there's anything else you can help with
782: - Let them know they'll receive a follow-up email
783: ```
784: 
785: #### How Script Becomes Behavior
786: 
787: The script is parsed into stages:
788: 
789: ```go
790: type ParsedScript struct {
791:     TenantID  int32
792:     AudienceType string
793:     Stages    []ScriptStage
794: }
795: 
796: type ScriptStage struct {
797:     Name    string // "Opening"
798:     Content string  // "- Greet the customer...\n- Ask how you can help..."
799:     Index   int
800: }
801: ```
802: 
803: The LLM prompt includes the script and a state machine instruction:
804: 
805: ```
806: === CONVERSATION SCRIPT ===
807: 
808: Current stage: Opening
809: 
810: Stage: Opening
811: - Greet the customer warmly
812: - Introduce yourself
813: - Ask how you can help
814: 
815: Stage: Triage
816: ...
817: 
818: You are currently in the "Opening" stage. Follow the script for this stage.
819: When the customer's need is clear, transition to "Triage".
820: ```
821: 
822: The AI is instructed to progress through stages naturally. The script is a guide, not a straitjacket — the AI can skip stages if the customer's intent is immediately clear.
823: 
824: #### Script for Internal Agents
825: 
826: Internal agents have a different script focused on efficiency:
827: 
828: ```markdown
829: <!-- @audience: internal -->
830: 
831: ## Stage: Review
832: - Review the escalation memo for full context
833: - Verify all customer information is collected
834: - Check for any previous interactions (via observation log)
835: 
836: ## Stage: Resolution
837: - Address the specific escalation reason
838: - If the AI missed something, correct it in the memo notes
839: - Update the ticket status to IN_PROGRESS
840: 
841: ## Stage: Closing
842: - Confirm resolution with the customer
843: - Set ticket to CLOSED
844: - Add summary note to the escalation memo for AI training
845: ```
846: 
847: ---
848: 
849: ### Chapter 10: RAG vs Structured Mode
850: 
851: Not all KB content is well-structured. Some tenants have long documents, policy PDFs, or technical manuals. bchat supports two modes for handling KB content.
852: 
853: #### Structured Mode
854: 
855: When the KB is well-structured (services, FAQs, exclusions, etc. are annotated), the system builds a **structured system prompt**:
856: 
857: ```
858: === SERVICES ===
859: - Water Extraction (24/7 emergency, priority: 1)
860: - Mold Remediation (business hours, priority: 3)
861: 
862: === FAQS ===
863: - Pricing: Costs vary based on extent of damage.
864: 
865: === EXCLUSIONS ===
866: We do NOT provide: sewage cleanup, pest control.
867: ```
868: 
869: **Pros:** Precise, controllable, no retrieval errors.
870: **Cons:** Requires well-annotated KB, limited to what fits in the context window.
871: 
872: #### RAG Mode
873: 
874: When the KB is unstructured or very large, the system uses **Retrieval-Augmented Generation**:
875: 
876: ```
877: 1. Chunk KB into segments (~500 tokens each)
878: 2. Embed each chunk via OpenRouter text-embedding-3-small
879: 3. Store embeddings in LanceDB
880: 4. At query time, retrieve top-K chunks similar to the query
881: 5. Inject retrieved chunks into the prompt
882: ```
883: 
884: The `HeadingBasedChunker` in `chunker.go` splits by markdown headings:
885: 
886: ```go
887: type DocumentChunk struct {
888:     ID            string
889:     TenantID      int32
890:     AudienceType  string
891:     ContentType   string // "service", "faq", "general"
892:     Title         string // heading text
893:     Content       string // chunk content
894:     Code          string // annotation code (e.g., "water_extraction")
895:     IsEmergency   bool
896:     Priority      int
897:     IsActive      bool
898:     // ...
899: }
900: ```
901: 
902: #### Hybrid Mode
903: 
904: You can use both: structured content for the core system prompt and RAG for supplementary retrieval:
905: 
906: ```
907: System prompt: Identity + Rules + Intents (always in context)
908: RAG context: Retrieved chunks for specific queries (on-demand)
909: ```
910: 
911: This is the recommended approach for production. The structured content defines the AI's behavior; RAG provides the AI's knowledge.
912: 
913: #### CX Implications
914: 
915: | Mode | Best For | CX Risk |
916: |------|----------|---------|
917: | Structured | Simple service catalogs (< 20 services) | KB maintenance burden |
918: | RAG | Large knowledge bases (manuals, policies) | Retrieval failures, latency |
919: | Hybrid | Most production scenarios | Configuration complexity |
920: 
921: **The CX golden rule of RAG:** A bad retrieval is worse than no retrieval. If the RAG system returns an irrelevant chunk, the AI will hallucinate around it. Always test RAG recall before deploying to production.
922: 
923: ---
924: 
925: ## Part 4: Observational Memory & Agent Learning
926: 
927: ### Chapter 11: How Observational Memory Works
928: 
929: Observational Memory (OM) is bchat's solution to conversational amnesia — the problem where the AI forgets what was discussed in previous exchanges.
930: 
931: #### The Problem It Solves
932: 
933: Without OM, the AI's memory is limited to the conversation history in the current session's context window. But context windows are finite (typically 8K–128K tokens), and important information can fall out:
934: 
935: - Customer preferences ("I always call on Tuesdays")
936: - Previous issues ("I had a leak last month too")
937: - Unresolved concerns ("You didn't answer my question about pricing")
938: 
939: OM solves this by **compressing conversation history into a structured observation log** that persists beyond the context window.
940: 
941: #### The Pipeline
942: 
943: OM works in three stages:
944: 
945: **Stage 1: Buffer (low-cost pre-computation)**
946: 
947: After each chat response, the system checks if the unobserved tokens exceed the buffer threshold (20% of `ObserverTokenThreshold`). If so, it triggers an async buffer observation:
948: 
949: ```go
950: // In service.go (after chat response)
951: func (s *Service) triggerObservation(ctx context.Context, session *store.AgentSession) {
952:     shouldBuffer := unobservedTokens >= int(float64(config.ObserverTokenThreshold)*config.BufferTokens)
953:     shouldBlock := unobservedTokens >= int(float64(config.ObserverTokenThreshold)*config.BlockAfter)
954:     
955:     if shouldBlock {
956:         // Synchronous — block the response until observation is done
957:         s.RunObserver(ctx, session)
958:     } else if shouldBuffer {
959:         // Async — pre-compute in background
960:         s.ObserverBuffer.TriggerBuffer(session.TenantID, session.ID)
961:     }
962: }
963: ```
964: 
965: **Stage 2: Observer (extract structured observations)**
966: 
967: When the token threshold (default 30,000) is reached, the Observer LLM is called with the conversation transcript. It extracts:
968: 
969: ```xml
970: <observations>
971: 🔴 Customer is frustrated about delayed response times.
972: 🟡 Customer mentioned they have a basement renovation planned.
973: 🟢 Customer preferred email contact over phone.
974: </observations>
975: <current-task>Resolve complaint about response time and schedule follow-up.</current-task>
976: <suggested-response>Apologize for the delay, explain the standard response time, offer a timeline for resolution.</suggested-response>
977: ```
978: 
979: The observer prompt (`prompts/observer.txt`, 154 lines) instructs the LLM to:
980: - Distinguish user assertions from questions (🔴🟡🟢 priority)
981: - Preserve unusual phrasing and precise action verbs
982: - Use temporal anchoring ("Customer reported X on Tuesday")
983: - Output in XML for reliable parsing
984: 
985: **Stage 3: Reflector (compress when too large)**
986: 
987: When the observation log exceeds the token threshold (default 2,000), the Reflector compresses it:
988: 
989: ```go
990: // In observer.go
991: func (s *Service) RunReflector(ctx context.Context, session *store.AgentSession) error {
992:     tokensInLog := estimateTokens(obsLog.ObservationLog)
993:     if tokensInLog < s.omConfig.TokenThreshold {
994:         return nil // No compression needed
995:     }
996:     // Call LLM with reflector prompt to compress
997:     compressed, err := s.callReflectorLLM(ctx, obsLog.ObservationLog)
998:     // ...
999: }
1000: ```
1001: 
1002: The reflector prompt (`prompts/reflector.txt`, 116 lines) compresses without losing critical context, condensing older observations more aggressively than recent ones.
1003: 
1004: #### Resource-Scoped vs Thread-Scoped Memory
1005: 
1006: OM supports two scopes:
1007: 
1008: | Scope | Key | Lifetime | Use Case |
1009: |-------|-----|----------|----------|
1010: | `thread` | Session ID | Single conversation | Current chat context |
1011: | `resource` | User ID (resource_id) | Cross-conversation | Repeat customer history |
1012: 
1013: Resource-scoped memory allows the AI to remember a customer across multiple sessions:
1014: 
1015: ```go
1016: // When loading observations for the prompt:
1017: if config.Scope == "resource" {
1018:     obs, _ := s.store.GetObservationLogByResource(ctx, userID)
1019: } else {
1020:     obs, _ := s.store.GetObservationLog(ctx, sessionID)
1021: }
1022: ```
1023: 
1024: **CX value:** A customer who calls back next week is greeted with "Welcome back! Last time we discussed your basement. How is the renovation going?"
1025: 
1026: #### Configuring OM for CX
1027: 
1028: Key configuration tradeoffs:
1029: 
1030: | Parameter | Low Setting | High Setting |
1031: |-----------|-------------|--------------|
1032: | `OM_TOKEN_THRESHOLD` (reflector) | Frequent compression, less detail | Richer memory, more token cost |
1033: | `OM_OBSERVER_TOKEN_THRESHOLD` (observer) | Frequent observations, lower latency | Fewer LLM calls, more context per call |
1034: | `OM_SCOPE` | `thread` (no cross-session memory) | `resource` (cross-session memory) |
1035: | `OM_ENABLED` | No memory (cheap) | Memory (better CX) |
1036: 
1037: For most CX use cases:
1038: - Enable OM (`OM_ENABLED=true`)
1039: - Use `resource` scope for repeat customers
1040: - Set `OM_TOKEN_THRESHOLD=2000` (compress at 2K tokens)
1041: - Set `OM_OBSERVER_TOKEN_THRESHOLD=30000` (observe every 30K tokens)
1042: 
1043: ---
1044: 
1045: ### Chapter 12: From Observations to Learning
1046: 
1047: Observational Memory captures what happened in a conversation. But the real value comes from aggregating observations across many conversations to identify patterns.
1048: 
1049: #### The Analysis Engine
1050: 
1051: The analysis engine (`analysis.go`) scores every conversation against a 100-point rubric across 6 categories:
1052: 
1053: ```go
1054: type AnalysisBreakthrough struct {
1055:     IntentRecognition     CategoryScore // 0-15 points
1056:     ServiceAlignment      CategoryScore // 0-15 points
1057:     PolicyCompliance      CategoryScore // 0-20 points
1058:     ConversationFlow      CategoryScore // 0-20 points
1059:     InformationGathering  CategoryScore // 0-15 points
1060:     ToneAndResolution     CategoryScore // 0-15 points
1061: }
1062: 
1063: type CategoryScore struct {
1064:     Score int
1065:     Max   int
1066:     Notes string
1067: }
1068: ```
1069: 
1070: The analysis is performed by an LLM that compares the conversation against the tenant's KB and policy:
1071: 
1072: ```
1073: You are analyzing a support conversation for quality.
1074: Evaluate against these criteria:
1075: 
1076: 1. Intent Recognition (0-15): Did the AI correctly identify the customer's intent?
1077: 2. Service Alignment (0-15): Did the AI match to the correct service?
1078: 3. Policy Compliance (0-20): Did the AI follow all rules and guidelines?
1079: 4. Conversation Flow (0-20): Was the conversation natural and efficient?
1080: 5. Information Gathering (0-15): Did the AI collect necessary information?
1081: 6. Tone and Resolution (0-15): Was the tone appropriate? Was the issue resolved?
1082: 
1083: Provide a breakdown and total score.
1084: ```
1085: 
1086: #### Learning Memory
1087: 
1088: After many analyses, the system builds a `LearningMemory` for each tenant:
1089: 
1090: ```go
1091: type AgentLearningMemory struct {
1092:     TenantID         int32
1093:     CommonIssues     []CommonIssue      // Recurring problems
1094:     LearnedBehaviors []LearnedBehavior  // What the AI should do differently
1095:     ImprovementAreas []ImprovementArea  // Categories needing attention
1096:     PendingSuggestions []PendingSuggestion // Awaiting human approval
1097:     AnalysisCount    int                // Number of analyses performed
1098:     Version          int                // Incremented on each update
1099: }
1100: ```
1101: 
1102: **The feedback loop:**
1103: 
1104: ```
1105: Conversations → Analysis → Common Issues → KB Update → Better Conversations
1106:                                  ↓
1107:                       Learned Behaviors → Prompt Enhancement → Better Responses
1108:                                  ↓
1109:                       Improvement Areas → Training Focus → Better Analysis Scores
1110: ```
1111: 
1112: #### How Learning Improves CX
1113: 
1114: Real example of the feedback loop:
1115: 
1116: 1. **Analysis detects:** 30% of conversations about "mold remediation" end in escalation because the KB doesn't cover mold
1117: 2. **Common issue created:** `{Category: "coverage_gap", Description: "Mold remediation not in KB", Occurrences: 15}`
1118: 3. **KB update:** Tenant adds mold remediation to KB.MD with `@service: mold_remediation`
1119: 4. **Reindex:** The new service is chunked and embedded
1120: 5. **Next customer:** "Do you handle mold?" → AI finds the new service → resolves without escalation
1121: 
1122: **Without this loop, the KB is static.** With it, the KB evolves based on actual customer conversations.
1123: 
1124: #### Simulation-Enhanced Learning
1125: 
1126: The simulation system (`simulation.go`) accelerates the feedback loop by generating synthetic conversations:
1127: 
1128: ```go
1129: type SimulationState struct {
1130:     SimID      string
1131:     TenantID   int32
1132:     AudienceType string
1133:     Scenario   string // Description of the simulation scenario
1134:     MaxTurns   int    // Default 20
1135:     
1136:     // Control channels for pause/resume/stop
1137:     pauseCh    chan struct{}
1138:     resumeCh   chan struct{}
1139:     stopCh     chan struct{}
1140: }
1141: ```
1142: 
1143: In each simulation turn:
1144: 1. A simulated "human" (LLM with persona prompt) sends a customer message
1145: 2. The AI responds through the normal `processChat` pipeline
1146: 3. The conversation is recorded as a `SimulationTranscript`
1147: 4. After `MinTurns` (configurable), end conditions are checked
1148: 
1149: Simulation is invaluable for testing KB changes before deploying to real customers. If a KB update causes the AI to give wrong answers for a specific scenario, simulation catches it.
1150: 
1151: ---
1152: 
1153: ### Chapter 13: The Verifier and Sanitizer
1154: 
1155: Two safety systems protect the customer experience from AI mistakes.
1156: 
1157: #### The Verifier
1158: 
1159: The Verifier (`verifier.go`) is an optional second LLM call that checks the AI's response before sending it to the customer:
1160: 
1161: ```go
1162: type VerificationConfig struct {
1163:     Enabled     bool   // Enable/disable verification
1164:     Mode        string // "shadow" (log only) or "enforce" (block violations)
1165:     MaxLatencyMs int    // 3000ms timeout
1166:     SkipOnError  bool   // Allow response if verifier fails
1167: }
1168: ```
1169: 
1170: **Verification checklist:**
1171: 
1172: 1. **Contact Info Grounding** — Did the AI provide accurate contact info (phone, address)?
1173: 2. **Service Grounding** — Did the AI only mention services that exist in the KB?
1174: 3. **Claims Grounding** — Did the AI make claims not supported by the KB?
1175: 4. **Timeline & Process Claims** — Did the AI invent response times or processes?
1176: 5. **Customer Info Handling** — Did the AI request unnecessary personal info?
1177: 6. **Tone & Identity** — Did the AI stay in character and maintain proper tone?
1178: 
1179: Each check returns a severity:
1180: ```go
1181: const (
1182:     SeverityLow      = "low"
1183:     SeverityMedium   = "medium"
1184:     SeverityHigh     = "high"
1185:     SeverityCritical = "critical"
1186: )
1187: ```
1188: 
1189: **In shadow mode** (default for testing), violations are logged but responses are not blocked. **In enforce mode**, violations trigger a corrected response or escalation.
1190: 
1191: #### The Sanitizer
1192: 
1193: The Sanitizer (`sanitizer.go`) is a post-processing step that removes hallucinated content from the AI response. It uses 21 regex patterns to catch common LLM failures:
1194: 
1195: ```go
1196: var sanitizePatterns = []*struct{
1197:     pattern *regexp.Regexp
1198:     replacement string
1199: }{
1200:     // Remove hallucinated system tags
1201:     {regexp.MustCompile(`\[SYSTEM\].*`), ""},
1202:     {regexp.MustCompile(`\[DEBUG\].*`), ""},
1203:     {regexp.MustCompile(`\[INTERNAL\].*`), ""},
1204:     
1205:     // Remove self-references
1206:     {regexp.MustCompile(`^Assistant: `), ""},
1207:     {regexp.MustCompile(`^AI: `), ""},
1208:     {regexp.MustCompile(`^Bot: `), ""},
1209:     
1210:     // Remove thinking patterns
1211:     {regexp.MustCompile(`(?i)let me think`), ""},
1212:     {regexp.MustCompile(`(?i)i should`), ""},
1213:     
1214:     // Clean formatting
1215:     {regexp.MustCompile("```[^`]*```"), ""},  // code blocks
1216:     {regexp.MustCompile("`[^`]+`"), ""},       // inline code
1217: }
1218: ```
1219: 
1220: #### Contact Infill Prevention
1221: 
1222: The most dangerous AI hallucination in customer support is invented contact information. The sanitizer has special handling for this:
1223: 
1224: ```go
1225: func CorrectContacts(text string) string {
1226:     // Only replace impossible patterns
1227:     // all-zeros, all-nines, test numbers
1228:     patterns := []string{
1229:         `000-000-0000`,
1230:         `999-999-9999`,
1231:         `123-456-7890`,
1232:     }
1233:     // Does NOT correct (555) numbers — they might be real
1234:     for _, p := range patterns {
1235:         text = strings.ReplaceAll(text, p, "[phone number]")
1236:     }
1237:     return text
1238: }
1239: 
1240: func CorrectEmails(text string) string {
1241:     // Replace placeholder emails
1242:     // firstname.lastname@email.com → [email]
1243:     // example.com → [email]
1244:     patterns := []string{
1245:         `firstname\.lastname@`,
1246:         `@email\.com`,
1247:         `@example\.com`,
1248:         `@test\.com`,
1249:     }
1250:     // ...
1251: }
1252: ```
1253: 
1254: #### CX Impact
1255: 
1256: The Verifier + Sanitizer combination is equivalent to having a QA reviewer check every single AI response. It catches:
1257: 
1258: - 90%+ of contact info hallucinations (via sanitizer)
1259: - 70%+ of service hallucinations (via verifier)
1260: - 50%+ of policy violations (via verifier)
1261: 
1262: These are not theoretical numbers — they are based on the default patterns and verification rules. A tenant can add custom patterns and rules.
1263: 
1264: ---
1265: 
1266: ## Part 5: Ticket Management — Rethinking Workflows
1267: 
1268: ### Chapter 14: The Current Ticket System
1269: 
1270: Before we talk about rethinking ticket workflows, we need to understand the existing system.
1271: 
1272: #### Core Model
1273: 
1274: The ticket model in bchat is deliberately simple:
1275: 
1276: ```go
1277: type Ticket struct {
1278:     ID          int32
1279:     Title       string
1280:     Description string    // Must start with "/m/" — links to a Memo
1281:     Status      string    // OPEN, IN_PROGRESS, CLOSED
1282:     Priority    string    // LOW, MEDIUM, HIGH
1283:     CreatorID   int32
1284:     AssigneeID  *int32
1285:     CreatedTs   int64
1286:     UpdatedTs   int64
1287:     Type        string    // "TASK", "BUG", "STORY", "agent_escalation"
1288:     Tags        []string  // ["escalated", "urgent"]
1289:     TenantID    *int32
1290: }
1291: ```
1292: 
1293: The critical design choice: **Description is always a link to a Memo.** This enforces the principle that the conversation (memo) is the record, and the ticket is just a pointer.
1294: 
1295: #### The Kanban Interface
1296: 
1297: The ticket list at `/tickets` has two modes:
1298: 
1299: - **List mode**: A sortable table with all fields
1300: - **Board mode**: A drag-and-drop Kanban with three columns
1301: 
1302: The Kanban is deliberately simplified vs Jira:
1303: - No swimlanes, epics, or sprints
1304: - No custom status workflows (three columns is it)
1305: - No required fields on create
1306: - No permission-based transitions
1307: 
1308: This is not a limitation — it's a design choice. The ticket system exists to **track human intervention in AI conversations**, not to manage a project.
1309: 
1310: #### How Tickets Currently Connect to AI
1311: 
1312: Tickets are connected to the AI system through:
1313: 
1314: 1. **Escalation path**: `service.go → CreateEscalationTicket()` creates tickets when the AI can't resolve
1315: 2. **Workflows**: `agent_workflows` table links tickets to AI workflow steps
1316: 3. **Memos**: Ticket descriptions link to memos that contain the full conversation context
1317: 4. **Notifications**: Ticket comments generate SSE notifications to assigned agents
1318: 
1319: ---
1320: 
1321: ### Chapter 15: The Ticketless Ideal
1322: 
1323: The goal of bchat is not to make a better ticket system. The goal is to make tickets unnecessary.
1324: 
1325: #### The Ticketless Metric
1326: 
1327: The primary metric for evaluating bchat's effectiveness is the **ticketless resolution rate**:
1328: 
1329: ```
1330: ticketless_rate = conversations_resolved_without_ticket / total_conversations
1331: ```
1332: 

(Output capped at 50 KB. Showing lines 1-1332. Use offset=1333 to continue.)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/TUTORIAL_RAG.md"
}
```

**Output:**
```
<path>/home/chaschel/Documents/go/bchat/TUTORIAL_RAG.md</path>
<type>file</type>
<content>
1: # Eat Your Own Dogfood: Using bchat's RAG to Master the bchat Codebase
2: 
3: > **A Choose-Your-Own-Adventure Exercise**
4: >
5: > **Prerequisite:** You have read `TUTORIAL.md` and understand the bugs/033 case study.
6: > **What you'll need:** A running bchat instance (`task run:rag`), admin access to create a tenant, and the `bugs/033/` directory.
7: > **Estimated time:** 45-90 minutes depending on which paths you take.
8: 
9: ---
10: 
11: ## Prologue: The Setup
12: 
13: You are a developer who has just joined the bchat team. Your first task: implement bugs/033 — Native Webhook Integrations v1. The code is scaffolded but never tested in production. The critical bugs are latent, waiting to be discovered.
14: 
15: Your senior engineer is on vacation.
16: 
17: You have two tools at your disposal:
18: 
19: 1. **Manual reading** — open files, trace code paths, build mental models.
20: 2. **bchat's own RAG** — index the codebase into a tenant, then chat with it to ask questions in natural language.
21: 
22: This exercise is about discovering when each tool helps, when it hurts, and — most importantly — what neither tool can do for you.
23: 
24: ### Phase 0: Build Your Knowledge Base
25: 
26: Before the adventure begins, you must set up your RAG index.
27: 
28: **Step 1: Create a tenant**
29: 
30: Use the bchat Admin UI (or API) to create a tenant called `codebase-explorer`:
31: 
32: ```bash
33: # Via the API (replace cookie with your auth session):
34: curl -X POST http://localhost:8081/api/v1/agent/tenants \
35:   -H "Content-Type: application/json" \
36:   -H "Cookie: access-token=your-token" \
37:   -d '{"companyName": "Codebase Explorer", "tenantSlug": "codebase-explorer"}'
38: ```
39: 
40: Or use the Admin UI at `/agent-admin` → "Add Tenant."
41: 
42: **Step 2: Choose what to index**
43: 
44: You have 124 documentation files in `docs/`, dozens of Go source files in `server/`, 22 files in `bugs/033/` alone. You cannot index everything — the chunker will split it, but the RAG's precision degrades with content that is too broad.
45: 
46: **Your first decision (write it down):**
47: 
48: > Which files from bugs/033 do I index?
49: 
50: Your options:
51: 
52: | Choice | What you get | What you miss |
53: |--------|--------------|---------------|
54: | **The plans only** (`plan_biz.md`, `plan5.md`, `docs_033.md`) | Business context, ADRs, rollout plan, architecture diagrams | Actual code, bugs, implementation details |
55: | **The code only** (`integrations.go`, `service.go`, store files) | Implementation details, function signatures, database queries | Business justification, deployment config, rollback plan |
56: | **The code review** (`code_review.md`, `prompt_code_review.md`) | The bugs themselves, adversarial review patterns | The code itself, the business context |
57: | **Everything in bugs/033/** | Complete picture of the feature | Everything is there, but the RAG's precision may suffer with 22 files worth of content |
58: 
59: **Step 3: Create your KB.md**
60: 
61: bchat's parser expects annotated markdown. You must structure your knowledge base in a format the chunker can split effectively.
62: 
63: Create a file called `codebase_kb.md`:
64: 
65: ```markdown
66: # bchat Codebase Explorer
67: 
68: ## Architecture Overview
69: bchat is a multi-tenant AI chat agent platform built on the Memos application.
70: Each tenant defines knowledge base, policies, and conversation scripts.
71: The platform uses Go (Echo framework) for the backend, React (MobX + Joy UI) for the frontend.
72: 
73: ## Deployment Architecture
74: - Containerized via Docker multi-stage builds (Dockerfile.pg.fly)
75: - Deployed to Fly.io with auto-scaling to zero
76: - Database: SQLite (development), Neon Postgres (production)
77: - Vector DB: LanceDB with S3/Tigris storage (production)
78: 
79: ## Webhook Integration Feature
80: <!-- @feature: webhooks, version: v1 -->
81: ## Webhook Events
82: Native webhook integrations send real-time events to external systems.
83: Event types: lead.captured, lead.updated, escalation.created, conversation.completed, safety.triggered
84: Delivery: immediate dispatch goroutine + poller-based retry every 5 minutes
85: Max retries: 5 attempts
86: 
87: ## Event Security
88: <!-- @security: ssrf, hmac -->
89: ## SSRF Protection
90: Three-layer defense: URL scheme validation, DNS resolution + IP blocking, redirect re-validation.
91: 
92: ## Database Migrations
93: <!-- @section: migrations -->
94: ## Migration Pattern
95: Migrations stored in store/migration/sqlite/ for SQLite and store/migration/postgres/ for Postgres.
96: File naming: {version}/{patch}__description.sql
97: 
98: ## Poller and Cron
99: <!-- @section: cron -->
100: ## Cron-Based Polling
101: Supercronic runs every 5 minutes, triggers POST /api/v1/system/trigger-cron.
102: processEventPoller claims pending events with FOR UPDATE SKIP LOCKED (Postgres).
103: ```
104: 
105: **This is deliberately incomplete.** You must fill in the gaps based on what you read. The exercise is in the *crafting* — deciding what's important enough to include and in what structure.
106: 
107: **Step 4: Upload and Reindex**
108: 
109: Upload via the Admin UI or API:
110: 
111: ```bash
112: # Upload the KB
113: curl -X POST http://localhost:8081/api/v1/agent/codebase-explorer/import \
114:   -F "audience_type=internal" \
115:   -F "file_type=kb" \
116:   -F "file=@codebase_kb.md"
117: 
118: # Rebuild the RAG index
119: curl -X POST http://localhost:8081/api/v1/agent/codebase-explorer/reindex
120: ```
121: 
122: **Step 5: Verify**
123: 
124: Go to the RAG Search Explorer in the Admin UI (`/rag-stats`). Search for "webhook event types" and verify you get results.
125: 
126: ---
127: 
128: ### The Branch Point
129: 
130: Your KB is indexed. The RAG is ready. Four doors stand before you.
131: 
132: ```
133: ┌──────────────────────────────────────────────────┐
134: │                                                  │
135: │   ┌──────────┐  ┌──────────┐  ┌──────────┐     │
136: │   │   CEO's   │  │  Eng.    │  │  Ops     │     │
137: │   │ Briefing  │  │  Lab     │  │ War Room │     │
138: │   │   Room    │  │          │  │          │     │
139: │   └────┬─────┘  └────┬─────┘  └────┬─────┘     │
140: │        │             │             │            │
141: │        └──────────┬──┴──────────┬──┘            │
142: │                   │             │                │
143: │            ┌──────▼──────┐     │                 │
144: │            │ Detective's │     │                 │
145: │            │   Office    │     │                 │
146: │            └──────┬──────┘     │                 │
147: │                   │             │                │
148: │        All paths lead here      │                │
149: └──────────────────────────────────────────────────┘
150: ```
151: 
152: You may enter any room. Each room teaches a different lesson about RAG. But the Detective's Office contains the master puzzle — the "Case of the Missing Webhook" — which ties everything together.
153: 
154: **Choose your first room:**
155: 
156: > If you enter the **CEO's Briefing Room** → turn to page [Adventure 1]
157: >
158: > If you enter the **Engineering Lab** → turn to page [Adventure 2]
159: >
160: > If you enter the **Ops War Room** → turn to page [Adventure 3]
161: >
162: > If you enter the **Detective's Office directly** → turn to page [Adventure 4]
163: >
164: > (You can visit multiple rooms. Each gives you clues for the Detective's Office.)
165: 
166: ---
167: 
168: ## Adventure 1: The CEO's Briefing Room
169: 
170: **Persona:** Product manager / CEO  
171: **Time limit:** 10 minutes before a customer call  
172: **Your goal:** Determine whether bchat can replace a prospect's Intercom AI subscription ($97/mo)
173: 
174: You walk into a clean, white room. A countdown clock on the wall reads 09:59. A monitor displays the RAG chat interface connected to your `codebase-explorer` tenant. A notepad on the table reads:
175: 
176: > "Acme Restoration is paying $246/mo for three SaaS tools. They want to consolidate. Can bchat replace all three?"
177: 
178: You open the RAG chat and start asking questions.
179: 
180: ### Query 1: The Cost Question
181: 
182: Type into the chat:
183: 
184: ```
185: How much does Intercom AI cost and can bchat replace it?
186: ```
187: 
188: **What happens next depends entirely on what you indexed.**
189: 
190: - If you indexed `plan_biz.md` → The RAG retrieves: "$97/mo for Intercom AI, replaced by webhook integrations + Zapier templates."
191: - If you indexed only code files → The RAG may retrieve nothing relevant (there's no cost data in Go code). You get a hallucination or a vague answer.
192: - If you indexed `code_review.md` → The RAG retrieves the bug descriptions, not the business case. You tell the customer "our code has critical bugs."
193: 
194: **Exercise: Pause and reflect.**
195: 
196: Write down: *What did the RAG tell you? Was it complete? Would you feel confident taking this answer to a customer call?*
197: 
198: ### Query 2: The Capability Question
199: 
200: ```
201: What integrations does bchat support?
202: ```
203: 
204: **What the RAG should answer** (if you indexed the business plans):
205: - Webhooks (v1) — send events to any HTTP endpoint
206: - Zapier/n8n (via webhooks) — connect to 5,000+ apps
207: - Twilio SMS (deferred to v1.1)
208: - Google Calendar (deferred to v1.1)
209: - Inbound update API (planned)
210: 
211: **What the RAG might answer** (if you indexed only code):
212: - The `integration_types` column has 'webhook' and 'twilio' values
213: - The database schema supports CRUD operations
214: - But nothing about Zapier, templates, or the deferred roadmap
215: 
216: ### Query 3: The Timeline Question
217: 
218: ```
219: When will SMS follow-ups be available?
220: ```
221: 
222: If you indexed `plan5.md`, the answer is clear: "Deferred to v1.1." If you indexed `plan_biz.md`, the timeline shows "Phase 2: Weeks 3-5." If you indexed neither, the answer is "I don't know" (or worse, a hallucination).
223: 
224: ### The CEO's Lesson
225: 
226: Close your notepad. The countdown reads 00:00.
227: 
228: **What did you learn?**
229: 
230: 1. **RAG's output is only as good as its input.** Index business docs for business questions; index code for technical questions. Don't expect one KB to serve all purposes.
231: 
232: 2. **RAG cannot infer what isn't there.** If you didn't index the roadmap, the RAG doesn't know the roadmap. It might hallucinate a timeline, which is worse than saying "I don't know."
233: 
234: 3. **The CEO's job is to know what questions to ask.** The RAG can retrieve answers, but only if you know which documents hold those answers. Critical thinking starts before the query.
235: 
236: ### Branch Choices
237: 
238: ```
239: Your phone buzzes. The customer call is over. You have two options:
240: 
241: → If you want to understand HOW the feature actually works (code level),
242:    enter the Engineering Lab. Turn to [Adventure 2].
243: 
244: → If you want to understand HOW to deploy and operate it,
245:    enter the Ops War Room. Turn to [Adventure 3].
246: 
247: → If you want to skip straight to debugging why it's broken,
248:    enter the Detective's Office. Turn to [Adventure 4].
249: ```
250: 
251: ---
252: 
253: ## Adventure 2: The Engineering Lab
254: 
255: **Persona:** Developer implementing features  
256: **Your goal:** Add a new event type (`lead.updated`) to the webhook system  
257: **Constraint:** You've never read the codebase. You will use only the RAG to understand the patterns.
258: 
259: You enter a room lined with screens. Each screen shows a different file from the codebase. A prompt on the main display reads:
260: 
261: > "Your task: implement `lead.updated` event dispatching. Status changes on leads must trigger a webhook. Use the RAG to understand the existing pattern. You have 30 minutes."
262: 
263: ### Experiment 1: Pattern Discovery
264: 
265: Ask the RAG:
266: 
267: ```
268: What's the pattern for dispatching an event? Show me the code.
269: ```
270: 
271: The RAG will retrieve chunks from `service.go` showing `dispatchEvent`. You'll see the loop over integrations, the idempotency key computation, the goroutine spawn.
272: 
273: **Now ask:**
274: 
275: ```
276: Where is lead.captured dispatched from? What triggers it?
277: ```
278: 
279: The RAG retrieves the call site in `captureLeadFromSession`. You learn that the lead capture function calls `s.dispatchEvent(ctx, config.TenantID, created.ID, "lead.captured", string(payload))`.
280: 
281: **Exercise:** Based on what you retrieved, write the call you would add for `lead.updated`. Don't look at the actual code yet. Just write it.
282: 
283: Here's the template — fill in the blanks:
284: 
285: ```go
286: // After updating the lead status:
287: s.dispatchEvent(ctx, _______, _______, "lead.updated", string(payload))
288: ```
289: 
290: What goes in the blanks? (The answer is in the RAG output.)
291: 
292: ### Experiment 2: The Trap
293: 
294: Now ask the RAG:
295: 
296: ```
297: How is the idempotency key computed?
298: ```
299: 
300: The RAG retrieves:
301: 
302: ```go
303: idempotencyKey := computeIdempotencyKey(tenantID, eventType, fmt.Sprintf("%d", ig.ID))
304: ```
305: 
306: This is the **actual committed code** — which contains the C1 bug (argument swap). The RAG retrieves it faithfully. It does not tell you it's wrong. It does not tell you that `eventType` is in the `leadID` position and `ig.ID` is in the `eventType` position.
307: 
308: **Now follow the pattern.** If you copy this call for your new `lead.updated` dispatch, you reproduce the bug.
309: 
310: **Exercise (crucial):**
311: 
312: Look at the function signature. The RAG retrieved it because it's nearby in the same file:
313: 
314: ```go
315: func computeIdempotencyKey(tenantID int32, leadID string, eventType string) string {
316:     data := fmt.Sprintf("%d:%s:%s", tenantID, leadID, eventType)
317:     hash := sha256.Sum256([]byte(data))
318:     return hex.EncodeToString(hash[:])
319: }
320: ```
321: 
322: Compare the call site with the signature. Do you see the mismatch?
323: 
324: - Signature expects: `(tenantID, leadID, eventType)`
325: - Actual call passes: `(tenantID, eventType, integrationID)` — **WRONG**
326: 
327: The RAG gave you both pieces of information (the call and the signature), but it did not *connect* them. That connection is a thinking task, not a retrieval task.
328: 
329: ### Experiment 3: The Store Pattern
330: 
331: Ask the RAG:
332: 
333: ```
334: What database operations do I need for a new event type? Show me the store interface methods.
335: ```
336: 
337: The RAG retrieves `CreateAgentEvent`, `ListAgentEvents`, `ClaimPendingEvents`, `UpdateAgentEvent`. These are general — they work for any event type. You don't need new store methods for a new event type.
338: 
339: **The lesson:** RAG helps you discover what *already exists* so you don't duplicate effort. The store layer is generic; you don't add methods for each event type. The event type is just a string parameter.
340: 
341: ### Experiment 4: The Handler Pattern
342: 
343: Ask the RAG:
344: 
345: ```
346: How are integrations routes registered? Show me the v1.go route setup.
347: ```
348: 
349: The RAG retrieves from `v1.go`:
350: 
351: ```go
352: adminGroup.GET("/:slug/integrations", s.agentHandler.HandleListIntegrations)
353: adminGroup.POST("/:slug/integrations", s.agentHandler.HandleCreateIntegration)
354: adminGroup.PUT("/:slug/integrations/:id", s.agentHandler.HandleUpdateIntegration)
355: adminGroup.DELETE("/:slug/integrations/:id", s.agentHandler.HandleDeleteIntegration)
356: adminGroup.POST("/:slug/integrations/:id/test", s.agentHandler.HandleTestIntegration)
357: ```
358: 
359: You don't need a new route for `lead.updated`. The events are dispatched automatically based on the event type. No new endpoint needed.
360: 
361: ### The Engineering Lab's Lesson
362: 
363: Your 30 minutes are up.
364: 
365: **What did you learn?**
366: 
367: 1. **RAG excels at pattern discovery.** It found the dispatch flow, the store interface, the route registration — all in seconds. Without RAG, you'd grep for "dispatchEvent" and piece it together manually over 15 minutes. The RAG did it in 15 seconds.
368: 
369: 2. **RAG cannot detect semantic errors.** It retrieved the C1 bug faithfully. It will help you reproduce the bug, not avoid it. **This is the core thesis of this entire tutorial.**
370: 
371: 3. **The "follow the pattern" heuristic is dangerous.** When the existing pattern has a bug, following it replicates the bug. You must understand *why* the pattern exists, not just *that* it exists.
372: 
373: ### Branch Choices
374: 
375: ```
376: Your implementation is done. But you have an uneasy feeling about the idempotency key.
377: 
378: → If you want to investigate the bug further,
379:    enter the Detective's Office. Turn to [Adventure 4].
380: 
381: → If you want to understand how this deploys and operates,
382:    enter the Ops War Room. Turn to [Adventure 3].
383: 
384: → If you're confident and want to ship,
385:    ...are you sure? The C1 bug is still there. Turn to [Adventure 4] anyway.
386: ```
387: 
388: ---
389: 
390: ## Adventure 3: The Ops War Room
391: 
392: **Persona:** Site reliability engineer  
393: **Your goal:** Deploy the webhook feature to production on Fly.io + Neon Postgres  
394: **Constraint:** The developer who wrote the code is on vacation. You have the RAG, the code, and the deployment docs.
395: 
396: You enter a room that looks like a NASA mission control center. Red, yellow, and green status panels cover the walls. A large screen shows `fly status` output. Smaller screens show logs, database connections, and a countdown to the next deploy window.
397: 
398: A sticky note on the console reads:
399: 
400: > "First deploy at 14:00. If any check fails, abort and investigate."
401: 
402: You have 30 minutes until deploy time.
403: 
404: ### Check 1: Environment Variables
405: 
406: Ask the RAG:
407: 
408: ```
409: What environment variables are needed for the webhook integration feature?
410: ```
411: 
412: The RAG retrieves:
413: 
414: - `CRON_TOKEN` — for authenticating the cron trigger
415: - `ENCRYPTION_MASTER_KEY` — for encrypting webhook secrets
416: - `OPENROUTER_API_KEY` — for the LLM (existing)
417: - `DATABASE_URL` — for Neon Postgres (existing)
418: 
419: **Now ask:**
420: 
421: ```
422: Which of these are set as Fly secrets vs. in fly.toml?
423: ```
424: 
425: The RAG retrieves from `fly_pg.toml` and the deployment docs. You learn that `CRON_TOKEN` must be set via `fly secrets set`, NOT in the config file.
426: 
427: **Exercise:** List all the secrets you need to set before deploying. Write the `fly secrets set` commands.
428: 
429: ### Check 2: The Cron Setup
430: 
431: Ask the RAG:
432: 
433: ```
434: How does the event poller work? What triggers it?
435: ```
436: 
437: The RAG retrieves the supercronic configuration:
438: 
439: - `build/crontab` — runs `*/5 * * * *`
440: - `scripts/entrypoint.sh` — launches supercronic before the main process
441: - `Dockerfile.pg.fly` — installs supercronic during build
442: 
443: **Now ask:**
444: 
445: ```
446: What is the exact URL that supercronic calls?
447: ```
448: 
449: The RAG retrieves the crontab line:
450: 
451: ```
452: */5 * * * * curl -sf -X POST http://localhost:${PORT:-5230}/api/v1/system/trigger-cron -H "X-Cron-Token: ${CRON_TOKEN}"
453: ```
454: 
455: **Pause and examine this line.** The RAG retrieved it faithfully. But it's broken. Do you see why?
456: 
457: *(Hint: what shell does Ubuntu 24.04 use as `/bin/sh`?)*
458: 
459: This is the **M2 bug** from the code review. `${PORT:-5230}` is bash syntax. Ubuntu 24.04 uses dash as `/bin/sh`. Dash does not support `${VAR:-default}`.
460: 
461: The RAG retrieved the broken line without telling you it's broken. **You must notice the mismatch yourself.**
462: 
463: **Exercise:** If you deployed this crontab to production without fixing it, what would happen? Trace the failure:
464: 
465: 1. Supercronic runs `curl ... http://localhost:${PORT:-5230}/...`
466: 2. Dash sees `${PORT:-5230}` and... (what does it do?)
467: 3. The resulting URL is `http://localhost:/api/v1/system/trigger-cron`
468: 4. `curl` connects to `localhost:` — port is empty, which means default port (80)
469: 5. Nothing is listening on port 80 inside the container
470: 6. `curl` fails silently (the `-sf` flag suppresses errors)
471: 7. The poller never runs
472: 8. Failed events are never retried
473: 
474: All from a shell syntax incompatibility. The RAG retrieved the evidence but didn't connect the dots.
475: 
476: ### Check 3: Database Resilience
477: 
478: Ask the RAG:
479: 
480: ```
481: What happens when Neon Postgres restarts during event delivery?
482: ```
483: 
484: The RAG retrieves from `store/db/postgres/resilience.go`:
485: 
486: ```go
487: func RunResiliently(fn func() error) error {
488:     // Exponential backoff: 1s, 2s, 4s, 8s, 16s + jitter
489:     // Only retries transient errors (57P01, 08006, 08003)
490:     // Treats unique_violation (23505) as success
491: }
492: ```
493: 
494: **Exercise:** Is `RunResiliently` used in the webhook delivery path? Ask the RAG:
495: 
496: ```
497: Does the webhook dispatch use RunResiliently?
498: ```
499: 
500: The answer depends on what you indexed. If you indexed `integrations.go`, you'll see that the `deliverWebhook` function uses a standard HTTP client with 10-second timeout — it does NOT use `RunResiliently`. The resilience wrapper is for database operations, not HTTP delivery.
501: 
502: This is an important distinction: the *database* operations are resilient to transient failures. The *HTTP delivery* is not. If the database connection drops during event insertion, `RunResiliently` handles it. If the webhook receiver is slow, the HTTP client times out after 10 seconds and the event is retried by the poller.
503: 
504: ### Check 4: The Rollback Plan
505: 
506: Ask the RAG:
507: 
508: ```
509: What's the rollback plan if webhooks break in production?
510: ```
511: 
512: The RAG retrieves from the deployment docs and `docs_033.md`:
513: 
514: 1. Toggle individual integrations inactive via Admin UI (instant, no code)
515: 2. Disable event dispatching globally via config toggle (needs deployment)
516: 3. Drop `agent_events` and `agent_integrations` tables (data loss, non-critical)
517: 4. Code rollback via `fly deploy --image previous-tag`
518: 
519: **Exercise:** For each rollback option, write down:
520: - How long does it take to execute?
521: - What data is lost?
522: - Is it reversible?
523: 
524: ### The Ops War Room's Lesson
525: 
526: The countdown reaches 14:00. The deploy window is open.
527: 
528: **What did you learn?**
529: 
530: 1. **RAG is excellent for configuration discovery.** It found the env vars, the crontab, the Dockerfile instructions, the resilience patterns — all critical for deployment.
531: 
532: 2. **RAG cannot validate configurations across components.** It won't tell you that the crontab syntax is incompatible with the runtime shell. It won't tell you that `${PORT:-5230}` is bash-only and the container uses dash.
533: 
534: 3. **Deployment safety depends on testing the actual container, not the code.** The RAG can show you all the pieces, but only a `docker build && docker run` reveals that the crontab is broken. RAG is a knowledge tool, not a testing tool.
535: 
536: ### Branch Choices
537: 
538: ```
539: The deploy is greenlit. But something about that crontab bothers you.
540: 
541: → If you want to investigate whether the crontab is actually running,
542:    enter the Detective's Office. Turn to [Adventure 4].
543: 
544: → If you're satisfied that the deploy will work,
545:    ...are you sure about that crontab? Turn to [Adventure 4] anyway.
546: ```
547: 
548: ---
549: 
550: ## Adventure 4: The Detective's Office — "The Case of the Missing Webhook"
551: 
552: **Persona:** Debugging engineer  
553: **Your goal:** Find out why a lead webhook was never delivered  
554: **Incident report:**
555: 
556: > **Time:** 2026-07-10 14:03:00 UTC
557: > **Customer:** Acme Restoration
558: > **What happened:** A lead was captured during chat. Name: "Jane Smith." Email: "jane@example.com." Phone: "+1-555-0199." Service needed: Water damage repair.
559: > **Expected:** Within 30 seconds, Acme Restoration's CRM should have received a webhook with the lead data.
560: > **Actual:** The CRM shows nothing. The business lost a $5,000 job.
561: > **Your mission:** Find the root cause.
562: 
563: You enter a dimly lit office. A single desk lamp illuminates three pieces of evidence:
564: 
565: 1. **Database snapshot** — showing `agent_events` rows for this tenant
566: 2. **Log excerpts** — from the server around 14:03
567: 3. **Configuration files** — the crontab, the Dockerfile, the entrypoint
568: 
569: A whiteboard on the wall reads:
570: 
571: > "Every engineer gets one tool: the RAG. You can ask it anything about the codebase. But it will only give you what you ask for. The connections are yours to make."
572: 
573: ### The Evidence
574: 
575: **Evidence A: Database**
576: 
577: You query the database:
578: 
579: ```sql
580: SELECT * FROM agent_events WHERE tenant_id = 4 ORDER BY created_at DESC LIMIT 10;
581: ```
582: 
583: ```
584: id | tenant_id | event_type     | status     | attempts | claimed_at | last_error
585: ---+-----------+----------------+------------+----------+------------+----------------------------------------
586: 47 | 4         | lead.captured  | processing | 0        | 1720627383 | (null)
587: ```
588: 
589: The event has been in `processing` status for hours. Attempts is 0 — meaning the poller never reclaimed it. The `claimed_at` timestamp is from 14:03.
590: 
591: **Evidence B: Logs**
592: 
593: You search for any log lines containing "webhook" or "event" around 14:03:
594: 
595: ```
596: 14:03:01.234 INFO dispatching event tenant_id=4 integration_id=3 event_type=lead.captured idempotency_key=a1b2c3d4
597: 14:03:01.456 INFO webhook delivery completed event_id=47 integration_id=3 status=200
598: ```
599: 
600: Wait — it says delivery completed with status 200. But the CRM didn't receive it?
601: 
602: **Evidence C: Event Log API**
603: 
604: You call the event list API:
605: 
606: ```
607: GET /api/v1/agent/codebase-explorer/events
608: ```
609: 
610: It returns only ONE event for `lead.captured`. But Acme Restoration has had hundreds of leads over the past week. Where are the other events?
611: 
612: ### Using the RAG
613: 
614: You have one tool. You can ask the RAG questions. Each answer gives you a clue. The quality of your question determines the quality of the answer.
615: 
616: **Clue 1: The Idempotency Key**
617: 
618: Ask the RAG:
619: 
620: ```
621: How does the idempotency key prevent duplicate events?
622: ```
623: 
624: The RAG retrieves the `computeIdempotencyKey` function signature and the UNIQUE constraint on `agent_events.idempotency_key`.
625: 
626: **Now ask a better question:**
627: 
628: ```
629: Show me every call to computeIdempotencyKey in the codebase.
630: ```
631: 
632: The RAG retrieves the call in `dispatchEvent`:
633: 
634: ```go
635: idempotencyKey := computeIdempotencyKey(tenantID, eventType, fmt.Sprintf("%d", ig.ID))
636: ```
637: 
638: **Now ask the critical question:**
639: 
640: ```
641: What are the parameter names of computeIdempotencyKey?
642: ```
643: 
644: The RAG retrieves the function definition:
645: 
646: ```go
647: func computeIdempotencyKey(tenantID int32, leadID string, eventType string) string
648: ```
649: 
650: **Do you see it?** Line up the parameters:
651: 
652: | Position | Parameter Name | What the Call Passes |
653: |----------|----------------|---------------------|
654: | 1st | `tenantID` | `tenantID` ✅ |
655: | 2nd | `leadID` | `eventType` ❌ |
656: | 3rd | `eventType` | `fmt.Sprintf("%d", ig.ID)` ❌ |
657: 
658: The idempotency key for every lead is:
659: 
660: ```
661: sha256(tenantID + "lead.captured" + "3")
662: ```
663: 
664: — where "3" is the integration ID. It doesn't include the lead ID at all!
665: 
666: **Every lead produces the same idempotency key.** The second lead hits the UNIQUE constraint and is silently skipped.
667: 
668: **Clue 1 answered:** The reason only one event was ever dispatched is the C1 bug. But this doesn't explain why the CRM didn't receive that *one* event...
669: 
670: **Clue 2: The Delivery**
671: 
672: Ask the RAG:
673: 
674: ```
675: How does the poller reclaim and deliver events?
676: ```
677: 
678: The RAG retrieves `processEventPoller`:
679: 
680: ```go
681: func (s *Service) processEventPoller(ctx context.Context) {
682:     events, err := s.store.ClaimPendingEvents(ctx, ...)
683:     for _, event := range events {
684:         s.deliverWebhook(ctx, event)
685:     }
686: }
687: ```
688: 
689: **Ask:**
690: 
691: ```
692: What context is passed to processEventPoller from the cron handler?
693: ```
694: 
695: The RAG retrieves `HandleTriggerCron`:
696: 
697: ```go
698: func (h *Handler) HandleTriggerCron(c echo.Context) error {
699:     h.service.processEventPoller(c.Request().Context())
700:     return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
701: }
702: ```
703: 
704: The context is `c.Request().Context()`. This context is **cancelled** when the handler returns — which is immediately after `c.JSON(...)` responds.
705: 
706: **But wait** — the log shows the *immediate dispatch* delivered successfully at 14:03:01.456. The immediate dispatch spawns a goroutine from the chat handler, not from the cron handler. So the C2 bug might not have affected *this* delivery.
707: 
708: Let's verify. If the immediate delivery succeeded (log says status=200), why didn't the CRM get it?
709: 
710: **Clue 3: The URL**
711: 
712: Ask the RAG:
713: 
714: ```
715: Show me the webhook configuration for Acme Restoration.
716: ```
717: 
718: (The RAG can't query the database directly — it can only retrieve from indexed code. But if you indexed the integration schema and config format...)
719: 
720: Ask:
721: 
722: ```
723: How is the webhook URL stored and used?
724: ```
725: 
726: The RAG retrieves the `WebhookConfig` struct and the `deliverWebhook` function:
727: 
728: ```go
729: type WebhookConfig struct {
730:     URL     string            `json:"url"`
731:     Secret  string            `json:"secret"`
732:     Headers map[string]string `json:"headers,omitempty"`
733: }
734: ```
735: 
736: The URL comes from the integration's `config` JSON field. If the config stored an incorrect URL — or if the URL was valid but the receiver endpoint was wrong — the webhook could have been delivered to the wrong place.
737: 
738: But the log says `status=200`. Something accepted the webhook. What if the URL pointed to a test endpoint that always returns 200, but the production URL was never configured?
739: 
740: **This is not a code bug — it's a configuration bug.** The RAG cannot catch configuration errors in the database. It can only tell you how the configuration is *stored* and *used*.
741: 
742: **Clue 4: The Crontab (for the other events)**
743: 
744: The immediate dispatch delivered the first event (somewhere). But what about the other leads that were captured after? Events #2 through #hundreds were never created because of C1. But even if C1 were fixed, would the poller have retried failed events?
745: 
746: Ask the RAG:
747: 
748: ```
749: Show me the crontab content.
750: ```
751: 
752: The RAG retrieves:
753: 
754: ```
755: */5 * * * * curl -sf -X POST http://localhost:${PORT:-5230}/api/v1/system/trigger-cron -H "X-Cron-Token: ${CRON_TOKEN}"
756: ```
757: 
758: **Ask:**
759: 
760: ```
761: What shell does supercronic use to execute commands?
762: ```
763: 
764: The RAG might not have this information indexed. But you know from `TUTORIAL.md` that supercronic uses `/bin/sh -c`, and on Ubuntu 24.04, `/bin/sh` is dash.
765: 
766: **The M2 bug:** `${PORT:-5230}` is bash syntax. Dash doesn't support it. The URL becomes `http://localhost:/...` — port 80. Nothing listens on port 80 in the container. `curl -sf` fails silently.
767: 
768: **But wait** — the *immediate* dispatch goroutine doesn't use the cron. The immediate dispatch fires from the chat handler directly. It uses a different code path. That's why the log showed delivery at 14:03:01 — the immediate dispatch worked for event #1.
769: 
770: But events #2 through #N were never created (C1). The poller can't deliver events that don't exist.
771: 
772: ### The Verdict
773: 
774: You step back from the whiteboard. Three bugs, operating in concert:
775: 
776: | Bug | What It Did | Evidence |
777: |-----|-------------|----------|
778: | **C1** | Argument swap in `computeIdempotencyKey` | Every lead produces the same key. Only event #1 is ever created. |
779: | **C2** | `HandleTriggerCron` uses HTTP request context | If the immediate dispatch failed for some events, the poller can't retry them properly. |
780: | **M2** | `${PORT:-5230}` broken on dash | Even if events needed poller retry, the cron never fires. |
781: 
782: **The primary cause:** C1. Without C1, events #2+ would exist. The immediate dispatch would have delivered most of them. The poller (crontab) would have retried the few failures.
783: 
784: **The secondary cause:** M2. Even if C1 were fixed, the poller wouldn't work in production because the crontab is broken.
785: 
786: **The tertiary cause:** C2. Even with a working crontab, the poller's context would be cancelled, potentially cutting off in-flight deliveries.
787: 
788: **The root cause of ALL bugs:** The code was never tested end-to-end in production. It was scaffolded but never stress-tested.
789: 
790: ### The Detective's Lesson
791: 
792: You sit down at the desk. The case is closed.
793: 
794: **What did you learn?**
795: 
796: 1. **The RAG helps you find evidence, but you must connect the dots.** Every clue was in the indexed codebase — the `dispatchEvent` call, the `HandleTriggerCron` context, the crontab content. The RAG retrieved them all. But only *you* could trace the chain of causality from argument swap → idempotent collision → missing events → lost lead → lost job.
797: 
798: 2. **The RAG cannot simulate runtime behavior.** The crontab line looks correct to a reader who assumes bash. The RAG doesn't run `docker build` and test the container. It only retrieves text.
799: 
800: 3. **The RAG cannot detect what isn't there.** The missing events (events #2 through #N) are an absence. The RAG can't detect an absence. It can only retrieve what exists. The fact that `agent_events` has only 47 rows when it should have 500 is not something the RAG can tell you.
801: 
802: 4. **This is the core thesis of the tutorial:** *Do not outsource thinking to coding agents.* The RAG is a powerful retrieval tool. It saved you hours of grepping through files. But it did not, and cannot, tell you why the webhook was missing. That required understanding *how the pieces fit together* — not just retrieving what each piece says.
803: 
804: ---
805: 
806: ## Epilogue: The RAG Quality Scorecard
807: 
808: You have completed your adventure. Whether you visited one room or all four, you now have experience with bchat's RAG in a real scenario.
809: 
810: It's time to grade your RAG deployment.
811: 
812: ### Scoring Rubric
813: 
814: For each metric, give yourself a score from 1 to 10.
815: 
816: **Precision: Did the RAG return relevant results?**
817: 
818: | Score | Meaning |
819: |-------|---------|
820: | 10/10 | Every answer was exactly what I needed |
821: | 7/10 | Most answers were relevant, some were off-topic |
822: | 4/10 | Half the answers were unrelated |
823: | 1/10 | Everything was noise |
824: 
825: Your score: ___/10
826: 
827: What would improve precision? (Smaller KB? Better annotations? Higher MinScore?)
828: 
829: **Recall: Did the RAG return ALL relevant results?**
830: 
831: | Score | Meaning |
832: |-------|---------|
833: | 10/10 | Every answer included every relevant chunk |
834: | 7/10 | Most answers were complete, some missed chunks |
835: | 4/10 | Answers were frequently incomplete |
836: | 1/10 | Critical information was always missing |
837: 
838: Your score: ___/10
839: 
840: What would improve recall? (Larger KB? More chunks per query? Lower MinScore?)
841: 
842: **Latency: How fast were responses?**
843: 
844: | Score | Meaning |
845: |-------|---------|
846: | 10/10 | Sub-second responses for every query |
847: | 7/10 | 1-3 seconds |
848: | 4/10 | 3-10 seconds |
849: | 1/10 | Over 10 seconds or timed out |
850: 
851: Your score: ___/10
852: 
853: **Hallucination: Did the LLM ever invent answers?**
854: 
855: | Score | Meaning |
856: |-------|---------|
857: | 10/10 | Never — the LLM always said "I don't know" or only used chunks |
858: | 7/10 | One or two minor fabrications |
859: | 4/10 | Several hallucinations that were clearly wrong |
860: | 1/10 | The LLM was confidently wrong most of the time |
861: 
862: Your score: ___/10
863: 
864: What would reduce hallucinations? (Stricter prompts? Forcing chunk-only answers? Better source material?)
865: 
866: **Bug Detection: Did the RAG help you find bugs?**
867: 
868: | Score | Meaning |
869: |-------|---------|
870: | 10/10 | The RAG directly revealed all three bugs (C1, C2, M2) |
871: | 7/10 | The RAG provided evidence that, when connected, revealed the bugs |
872: | 4/10 | The RAG gave me the code but I found the bugs by reading the files directly |
873: | 1/10 | The RAG was no help — I found the bugs by other means |
874: 
875: Your score: ___/10
876: 
877: **The critical question:** Could the RAG have caught these bugs if you hadn't known to look for them?
878: 
879: **Answer:** No. The bugs are semantic errors (wrong parameter order, wrong context lifecycle, wrong shell syntax). The RAG retrieves the code containing the bugs but cannot evaluate the *correctness* of the code. There is no chunk of text anywhere that says "the idempotency key computation is wrong here." The RAG can only retrieve what exists.
880: 
881: ---
882: 
883: ### The Final Reflection
884: 
885: Write your answers to these questions:
886: 
887: 1. **What did the RAG do well?** Be specific. Which questions received excellent answers, and why?
888: 
889: 2. **What did the RAG do poorly?** Which questions returned garbage, and why?
890: 
891: 3. **What information did you need that the RAG couldn't provide?** This is the gap between "retrieval" and "understanding."
892: 
893: 4. **If you were to re-index the codebase for better RAG quality, what would you change?**
894:    - Would you add more annotations?
895:    - Would you split into multiple KBs (business vs. technical vs. ops)?
896:    - Would you write entirely new documentation specifically designed for RAG consumption?
897: 
898: 5. **The million-dollar question:**
899: 
900: > "If you had indexed the codebase into bchat's RAG at the start of development, would the C1, C2, and M2 bugs have been caught before they were committed?"
901: 
902: Defend your answer in three sentences or less.
903: 
904: ---
905: 
906: ## Appendix A: Branch Map
907: 
908: ```
909:                       ┌──────────────────────┐
910:                       │    Prologue: Setup    │
911:                       │   Create tenant, KB,  │
912:                       │   upload, reindex     │
913:                       └──────────┬───────────┘
914:                                  │
915:                                  ▼
916:                     ┌────────────────────────┐
917:                     │    Choose Your Path     │
918:                     └──────────┬─────────────┘
919:                                │
920:           ┌────────────────────┼────────────────────┐
921:           │                    │                    │
922:           ▼                    ▼                    ▼
923: ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
924: │  Adventure 1:   │ │  Adventure 2:   │ │  Adventure 3:   │
925: │  CEO's Briefing │ │  Engineering    │ │  Ops War Room   │
926: │  Room           │ │  Lab            │ │                  │
927: │                 │ │                  │ │                  │
928: │  Business       │ │  Code patterns   │ │  Deployment      │
929: │  questions      │ │  C1 bug trap    │ │  M2 bug trap     │
930: │  Source quality │ │  Pattern copy   │ │  Config valid.   │
931: │  lesson         │ │  lesson         │ │  lesson          │
932: └────────┬────────┘ └────────┬────────┘ └────────┬────────┘
933:          │                   │                    │
934:          └───────────┬───────┴──────────┬─────────┘
935:                      │                  │
936:                      ▼                  ▼
937:           ┌─────────────────────────────────────┐
938:           │        Adventure 4: Detective's     │
939:           │        Office — "The Case of the    │
940:           │        Missing Webhook"             │
941:           │                                     │
942:           │  Connect all clues (C1 + C2 + M2)   │
943:           │  Trace causality chain              │
944:           │  Discover: no single bug caused it  │
945:           │  — all three combined               │
946:           └──────────────────┬──────────────────┘
947:                              │
948:                              ▼
949:           ┌─────────────────────────────────────┐
950:           │        Epilogue: RAG Quality        │
951:           │        Scorecard + Final Reflection  │
952:           └─────────────────────────────────────┘
953: ```
954: 
955: ### How to Maximize the Experience
956: 
957: | If you have... | Recommended path |
958: |----------------|------------------|
959: | 30 minutes | Pick ONE adventure that matches your role |
960: | 60 minutes | Do Adventures 1 + 4 (business + detective) OR 2 + 4 (engineering + detective) |
961: | 90 minutes | Do all four adventures in sequence |
962: | Unlimited | Re-index with different KB content and compare results. Try indexing only bugs/033/ code review files. Try indexing only deployment files. See how the answers change. |
963: 
964: ---
965: 
966: ## Appendix B: Suggested KB Structure Guide
967: 
968: This is NOT a pre-built KB — you must create your own based on what you think is important. But here are guidelines for structuring it effectively for RAG:
969: 
970: ### Do
971: 
972: - **Use H2 headers (`##`) per topic.** The chunker splits on H2. Each H2 section becomes one or more chunks. A section should be 1-3 paragraphs (under 1,000 tokens).
973: 
974: - **Lead with the most important sentence.** The first sentence of each section is retrieved by BM25 and vector search. If the first sentence says "This section describes deployment config" but the important detail is in the third paragraph, the RAG might rank it lower.
975: 
976: - **Use consistent terminology.** If you call it "event dispatch" in one section and "webhook delivery" in another, a query for "webhook delivery" might miss the "event dispatch" section.
977: 
978: - **Include explicit cross-references.** "The poller (see 'Cron-Based Polling' section above) claims pending events..." helps the RAG maintain context across chunks.
979: 
980: ### Don't
981: 
982: - **Don't include code snippets as the primary content.** The LLM already knows Go syntax. RAG content should explain *architecture* and *patterns*, not repeat code that exists elsewhere.
983: 
984: - **Don't make sections too long.** A section over 1,000 tokens will be split by the chunker. The second half might be retrieved without the first half, losing context.
985: 
986: - **Don't use ambiguous headings.** A heading like "Setup" is too vague. "Deployment Setup: Fly.io + Neon" is better.
987: 
988: ### The "Goldilocks" KB
989: 
990: A well-structured KB for a codebase has:
991: 
992: | Section Type | Examples | Ideal Size |
993: |-------------|----------|------------|
994: | Architecture overview | System components, data flow | 1 chunk (~500 tokens) |
995: | Feature descriptions | How each feature works, business logic | 1-3 chunks per feature |
996: | Configuration reference | Environment variables, flags | 1 chunk per config category |
997: | Deployment guide | Build, test, deploy, rollback | 2-5 chunks |
998: | Troubleshooting | Common issues, debugging steps | 1-3 chunks per scenario |
999: 
1000: ---
1001: 
1002: ## Appendix C: Replay Value
1003: 
1004: This exercise changes based on what you choose to index. Try these variations:
1005: 
1006: **Variant 1: Index only the bug reports**
1007: 
1008: Index only `bugs/033/code_review.md`. Then ask the RAG: "What bugs exist in the webhook feature?" See if it lists all 11 findings (2 critical, 2 major, 3 moderate, 4 nits). Then ask: "How would you fix C1?" Compare the RAG's answer with the actual fix.
1009: 
1010: **Variant 2: Index only the database migrations**
1011: 
1012: Index `store/migration/sqlite/0.31/*.sql` and `store/migration/postgres/0.31/*.sql`. Then ask: "What are the differences between the SQLite and Postgres versions of the migration?" The RAG has both — can it compare them?
1013: 
1014: **Variant 3: Index the architecture only**
1015: 
1016: Index `docs/DOCS_AGENT_ARCHITECTURE.MD`, `docs/DOCS_RAG_PIPELINE.MD`, `docs/DOCS_OM.md`. Then ask: "How does the RAG pipeline interact with the Observational Memory system?" This is a cross-system question — can the RAG connect two different architectural layers?
1017: 
1018: **Variant 4: The hardest test**
1019: 
1020: Index NOTHING from bugs/033. Index only `TUTORIAL.md` (this tutorial's companion document). Then ask the RAG: "What bugs exist in the webhook feature, and how would you fix them?" The tutorial describes the bugs in English — can the RAG retrieve and synthesize them into an actionable fix plan?
1021: 
1022: ---
1023: 
1024: *End of exercise. Remember: the RAG is a tool. You are the thinker.*

(End of file - total 1024 lines)
</content>
```

---

## Assistant (Plan · MiMo V2.5 Free · 11.0s)

_Thinking:_

Now I have a comprehensive understanding of all tutorial files. Let me also read the remaining parts of TUTORIAL.md to get the full context.

Now let me read the remaining parts of TUTORIAL.md to complete the context.

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/TUTORIAL.md",
  "offset": 1010
}
```

**Output:**
```
<path>/home/chaschel/Documents/go/bchat/TUTORIAL.md</path>
<type>file</type>
<content>
1010: sql.Open("sqlite", profile.DSN+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)")
1011: ```
1012: 
1013: Three pragmas are critical:
1014: 
1015: - `foreign_keys(1)`: Enable foreign key constraints (SQLite disables them by default for backward compatibility)
1016: - `busy_timeout(10000)`: Wait 10 seconds before failing on a locked database
1017: - `journal_mode(WAL)`: Write-Ahead Logging — allows concurrent reads during writes
1018: 
1019: #### Postgres Production Setup (Neon)
1020: 
1021: ```go
1022: // store/db/postgres/postgres.go
1023: dsn := profile.DSN
1024: if !strings.Contains(dsn, "default_query_exec_mode") {
1025:     sep := "?"
1026:     if strings.Contains(dsn, "?") {
1027:         sep = "&"
1028:     }
1029:     dsn += sep + "default_query_exec_mode=simple_protocol"
1030: }
1031: db, _ := sql.Open("pgx", dsn)
1032: db.SetMaxOpenConns(10)
1033: db.SetMaxIdleConns(5)
1034: db.SetConnMaxLifetime(5 * time.Minute)
1035: db.SetConnMaxIdleTime(1 * time.Minute)
1036: ```
1037: 
1038: Key details:
1039: 
1040: **The `default_query_exec_mode=simple_protocol` parameter:** Neon uses PgBouncer for connection pooling. PgBouncer in transaction mode requires the Postgres simple query protocol, not the extended protocol. The pgx driver defaults to extended protocol. This flag switches it. Without it, connections through PgBouncer fail.
1041: 
1042: **Connection pool sizing:**
1043: - `MaxOpenConns(10)`: At most 10 concurrent connections. Neon free tier limits this.
1044: - `MaxIdleConns(5)`: Keep 5 connections ready for reuse.
1045: - `ConnMaxLifetime(5m)`: Recycle connections every 5 minutes to avoid stale connections.
1046: - `ConnMaxIdleTime(1m)`: Close idle connections after 1 minute.
1047: 
1048: These settings are tuned for serverless Postgres. Too many idle connections keep the Neon compute active (no scale-to-zero savings). Too few connections cause request queuing under load.
1049: 
1050: #### Migration Differences
1051: 
1052: The migrations are stored in parallel directories:
1053: 
1054: ```
1055: store/migration/sqlite/0.31/00__agent_integrations.sql
1056: store/migration/sqlite/0.31/01__agent_events.sql
1057: store/migration/postgres/0.31/00__agent_integrations.sql
1058: store/migration/postgres/0.31/01__agent_events.sql
1059: ```
1060: 
1061: The migration runner (`store/migrator.go`) selects the correct directory based on `profile.Driver`.
1062: 
1063: Key dialect differences:
1064: 
1065: | Feature | SQLite | Postgres |
1066: |---------|--------|----------|
1067: | Auto-increment | `INTEGER PRIMARY KEY AUTOINCREMENT` | `SERIAL PRIMARY KEY` |
1068: | Boolean | `INTEGER NOT NULL DEFAULT 1` (0/1) | `BOOLEAN NOT NULL DEFAULT TRUE` |
1069: | Timestamp | `CAST(strftime('%s','now') AS INTEGER)` | `EXTRACT(EPOCH FROM NOW())::BIGINT` |
1070: | String concat | `\|\|` | `\|\|` (same) |
1071: | SELECT in UPDATE | Supported in subquery | Supported in subquery |
1072: | SKIP LOCKED | Not supported | `FOR UPDATE SKIP LOCKED` |
1073: | RETURNING | Supported (modernc.org/sqlite) | Supported |
1074: 
1075: #### The Migration Runner
1076: 
1077: ```go
1078: func (s *Store) Migrate(ctx context.Context) error {
1079:     // Step 1: Pre-migrate — apply LATEST.sql if no migration history exists
1080:     s.preMigrate(ctx)
1081: 
1082:     // Step 2: Validate data integrity (for SQLite, check orphaned FK references)
1083:     if s.profile.Driver == "sqlite" {
1084:         ValidateTicketReferences(ctx, s.driver.GetDB())
1085:     }
1086: 
1087:     // Step 3: Apply incremental migrations
1088:     // Compare current schema version vs. latest migration version
1089:     // Apply all migrations in between, sorted by version number
1090: 
1091:     // Step 4: Record migration in migration_history table
1092:     s.driver.UpsertMigrationHistory(ctx, ...)
1093: }
1094: ```
1095: 
1096: The migration is idempotent. If a migration fails, it can be re-run. The `execute` function tolerates "duplicate column" and "already exists" errors:
1097: 
1098: ```go
1099: func (s *Store) execute(ctx context.Context, tx *sql.Tx, stmt string) error {
1100:     if _, err := tx.ExecContext(ctx, stmt); err != nil {
1101:         errMsg := err.Error()
1102:         if strings.Contains(errMsg, "duplicate column") ||
1103:             strings.Contains(errMsg, "already exists") {
1104:             slog.Warn("migration: column already exists, skipping")
1105:             return nil
1106:         }
1107:         return errors.Wrap(err, "failed to execute statement")
1108:     }
1109:     return nil
1110: }
1111: ```
1112: 
1113: This is important: migrations should be safe to re-run. If a migration partially applies and fails, re-running it should succeed.
1114: 
1115: ### Critical Thinking Exercise
1116: 
1117: 1. **The connection pool settings are tuned for Neon. What would you change for an RDS Postgres instance?** (Answer: higher `MaxOpenConns` (25-50), longer `ConnMaxLifetime` (30-60 min), because RDS doesn't have scale-to-zero and connection overhead is lower.)
1118: 
1119: 2. **The migration runner applies migrations in sorted order. What happens if two developers create migrations with the same version number?** (Answer: the migration with the alphabetically later filename would be applied second. If they conflict, the second would fail. CI validation scripts should catch this.)
1120: 
1121: 3. **SQLite's `BEGIN` → execute → `COMMIT` wraps each migration in a transaction. Postgres can handle larger transactions. Why might you want to split large migrations?** (Answer: to reduce lock duration. A migration that adds a column to a 10-million-row table might take minutes on Postgres, locking the table.)
1122: 
1123: 4. **The pre-migrate step applies `LATEST.sql` only if no migration history exists.** How does an existing database get upgraded? (Answer: it already has migration history. The main `Migrate` function compares the current schema version against the latest migration and applies only the delta.)
1124: 
1125: ---
1126: 
1127: ## Chapter 10: Fly.io Deployment
1128: 
1129: ### Context
1130: 
1131: Fly.io is a platform for running containers close to users. It's similar to Heroku but with global regions, dedicated CPU options, and a simpler pricing model.
1132: 
1133: bchat is deployed to Fly.io with Neon Postgres as the database and Tigris S3 for LanceDB vector storage.
1134: 
1135: ### The Story from bugs/033
1136: 
1137: #### The fly.toml Configuration
1138: 
1139: ```toml
1140: app = 'bchat-pg'
1141: primary_region = 'sjc'
1142: 
1143: [build]
1144:   dockerfile = 'Dockerfile.pg.fly'
1145: 
1146: [env]
1147:   MEMOS_DRIVER = 'postgres'
1148:   MEMOS_MODE = 'prod'
1149:   MEMOS_PORT = '5230'
1150:   RAG_PIPELINE_ENABLED = 'true'
1151:   LANCEDB_STORAGE_PROVIDER = 's3'
1152:   LLM_MODEL = "poolside/laguna-m.1:free"
1153:   FORCE_REINDEX_ON_STARTUP = 'false'
1154:   TZ = 'UTC'
1155: 
1156: [http_service]
1157:   internal_port = 5230
1158:   force_https = true
1159:   auto_stop_machines = 'stop'
1160:   auto_start_machines = true
1161:   min_machines_running = 0
1162:   processes = ['app']
1163:   request_timeout = "30s"
1164: 
1165:   [http_service.concurrency]
1166:     type = 'connections'
1167:     hard_limit = 25
1168:     soft_limit = 20
1169: 
1170:   [[http_service.checks]]
1171:     grace_period = "15s"
1172:     interval = "5s"
1173:     method = "GET"
1174:     path = "/healthz"
1175: 
1176: [[vm]]
1177:   memory = '1024mb'
1178:   cpu_kind = 'shared'
1179:   cpus = 1
1180: ```
1181: 
1182: Key details:
1183: 
1184: **`auto_stop_machines = 'stop'`:** Scale to zero when idle. The machine stops after 5 minutes of no traffic. This saves money but means the first request after idle has a cold start delay.
1185: 
1186: **`min_machines_running = 0`:** Don't keep any machines warm. This is the most aggressive cost-saving setting.
1187: 
1188: **Health check:** `GET /healthz` with 15s grace period and 5s interval. Fly.io won't route traffic until the health check passes.
1189: 
1190: #### The Deployment Validation Chain
1191: 
1192: Before deploying, the team runs:
1193: 
1194: ```bash
1195: task fly:pre-deploy
1196: ```
1197: 
1198: Which executes:
1199: 
1200: 1. `task fly:check` — `scripts/validate-env-chain.sh`: Checks that all env vars referenced in Dockerfile and fly.toml are defined in .env or set as Fly secrets.
1201: 2. `task fly:db-check` — `scripts/validate-db-migrations.sh`: Runs the migrations against a local Postgres and verifies they apply cleanly.
1202: 
1203: This is a safety net. It catches missing environment variables and broken migrations before they reach production.
1204: 
1205: #### Setting Secrets
1206: 
1207: ```bash
1208: fly secrets set DATABASE_URL="postgresql://..."
1209: fly secrets set OPENROUTER_API_KEY="sk-or-v1-xxx"
1210: fly secrets set ENCRYPTION_MASTER_KEY="$(uuidgen)"
1211: fly secrets set CRON_TOKEN="$(openssl rand -hex 32)"
1212: ```
1213: 
1214: The `CRON_TOKEN` is set as a Fly secret, not in fly.toml. This is correct — secrets should never be in configuration files that might be committed to version control.
1215: 
1216: #### The "Awake-Only Retries" Limitation
1217: 
1218: The webhook poller only runs while the Fly machine is awake. If the machine scales to zero, the cron process stops. Pending events are not retried until the machine wakes again (triggered by user traffic).
1219: 
1220: This is documented honestly:
1221: 
1222: > **v1 Contract:** Immediate dispatch handles the live path. The poller only catches failed/stale events while the machine is already awake. A fully-idle machine defers retries until it next wakes (via user traffic). This is defensible for v1 given lease reclaim + immediate dispatch.
1223: 
1224: This is a business decision: the cost of keeping a machine always-on ($5-10/month) vs. the risk of delayed event delivery (1-2 hours of delay). For lead capture, a 1-hour delay is usually acceptable. For emergency alerts (safety.triggered), it might not be.
1225: 
1226: ### Critical Thinking Exercise
1227: 
1228: 1. **The trigger-cron endpoint uses `http://localhost:5230/...` (HTTP, not HTTPS).** Why is this safe? (Answer: the request never leaves the container. Fly.io's `force_https = true` applies to external traffic. Internal requests on localhost don't need encryption.)
1229: 
1230: 2. **The health check is `GET /healthz` which returns "Service ready."** What would happen if the server is alive but the database is down? (Answer: the health check would still pass, because it doesn't check the database. A proper health check should verify the database connection.)
1231: 
1232: 3. **`request_timeout = "30s"` means Fly.io terminates requests that take longer than 30 seconds.** The webhook delivery has a 10-second timeout per attempt. What happens if a webhook URL is slow and the trigger-cron endpoint takes 35 seconds? (Answer: Fly.io terminates the connection at 30s. The poller's context is cancelled. Some events may be partially processed.)
1233: 
1234: 4. **`auto_stop_machines = 'stop'` with 5-minute idle timeout.** If a new event arrives 6 minutes after the last traffic, what happens to the webhook delivery? (Answer: the machine wakes (cold start ~500ms), the chat handler processes the message, and the immediate dispatch goroutine delivers the webhook. The poller is not needed for this path.)
1235: 
1236: ---
1237: 
1238: # PART IV: THE MAINTENANCE LENS
1239: 
1240: ## Chapter 11: Postgres Resilience Patterns
1241: 
1242: ### Context
1243: 
1244: Managed database services periodically restart for maintenance. Your application must survive these restarts without crashing, losing data, or requiring manual intervention.
1245: 
1246: This is especially important for Neon, which scales compute to zero after inactivity. Every cold start is effectively a compute restart.
1247: 
1248: ### The Story from bugs/033
1249: 
1250: #### Transient Error Recognition
1251: 
1252: ```go
1253: func isTransientError(err error) bool {
1254:     // Postgres error codes:
1255:     // 57P01 — admin_shutdown (server is restarting)
1256:     // 08006 — connection_failure (connection lost)
1257:     // 08003 — connection_does_not_exist (already closed)
1258:     return strings.Contains(errMsg, "57P01") ||
1259:            strings.Contains(errMsg, "08006") ||
1260:            strings.Contains(errMsg, "08003")
1261: }
1262: ```
1263: 
1264: These three SQLSTATE codes cover the most common transient failures. The function parses the error message to find the code. Most Postgres drivers expose these codes differently — pgx has typed errors that can be inspected without string parsing.
1265: 
1266: #### Exponential Backoff with Jitter
1267: 
1268: ```go
1269: func RunResiliently(fn func() error) error {
1270:     base := 1 * time.Second
1271:     maxBackoff := 16 * time.Second
1272: 
1273:     for attempt := 0; attempt < 5; attempt++ {
1274:         err := fn()
1275:         if err == nil {
1276:             return nil // Success
1277:         }
1278:         if !isTransientError(err) {
1279:             return err // Permanent error — don't retry
1280:         }
1281:         if attempt == 4 {
1282:             return err // Last attempt failed — give up
1283:         }
1284: 
1285:         // Exponential backoff: 1s, 2s, 4s, 8s, 16s
1286:         backoff := base * (1 << attempt)
1287:         if backoff > maxBackoff {
1288:             backoff = maxBackoff
1289:         }
1290:         // Add jitter: ±25%
1291:         jitter := time.Duration(float64(backoff) * (0.75 + 0.5 * rand.Float64()))
1292:         time.Sleep(jitter)
1293:     }
1294:     return fmt.Errorf("max retries exceeded")
1295: }
1296: ```
1297: 
1298: Key design:
1299: 
1300: - **Exponential backoff:** 1s, 2s, 4s, 8s, 16s. Gives the database time to restart.
1301: - **Jitter:** ±25% random variation. Prevents thundering herd when multiple instances retry simultaneously.
1302: - **Max 5 attempts:** Total wait is ~31 seconds. After that, the operation fails rather than blocking indefinitely.
1303: - **Only transient errors retry:** A syntax error (42601) or foreign key violation (23503) should NOT be retried — they will always fail.
1304: 
1305: #### Idempotency + Resilience
1306: 
1307: ```go
1308: // In the unique_violation case:
1309: if strings.Contains(errMsg, "23505") {
1310:     return nil // Treat as success — event already exists
1311: }
1312: ```
1313: 
1314: When an idempotency key collides, the INSERT fails with error code 23505. `RunResiliently` treats this as success. This is the critical link between the database schema (UNIQUE constraint) and the application pattern (idempotency key).
1315: 
1316: Without this, a retry that inserts a duplicate row would fail with an error, even though the data is correct (the event already exists).
1317: 
1318: #### The "Lost Commit Acknowledgment" Problem
1319: 
1320: A subtle but important failure mode:
1321: 
1322: 1. Application sends `COMMIT` for an INSERT.
1323: 2. Database commits the transaction to disk.
1324: 3. Network failure — the success acknowledgment never reaches the application.
1325: 4. Application sees a connection error and doesn't know if the COMMIT succeeded.
1326: 
1327: Without idempotency, the application would retry the INSERT, creating a duplicate. With idempotency, the retry INSERT fails on the UNIQUE constraint, and the application treats it as success.
1328: 
1329: This is why the idempotency pattern is essential for reliable systems. It's not just for "convenience" — it's the only way to safely handle ambiguous failures.
1330: 
1331: ### Critical Thinking Exercise
1332: 
1333: 1. **The `RunResiliently` function sleeps between retries with `time.Sleep`.** This blocks the goroutine. Is this acceptable? (Answer: for background operations like the event poller, yes. For latency-sensitive HTTP handlers, no — you'd use a retry with context-based deadlines instead.)
1334: 
1335: 2. **What error codes should NOT be retried?** (Answer: 23505 (unique_violation) — the operation functionally succeeded. 23503 (foreign_key_violation) — the referenced row doesn't exist, retrying won't help. 42XXX (syntax error) — the query is malformed.)
1336: 
1337: 3. **The jitter range is 75%-125% of the base delay.** Why not 0%-100%? (Answer: 0% would mean instant retry, defeating the purpose of backoff. The 75% floor ensures minimum spacing between retries.)
1338: 
1339: 4. **The maximum total wait is ~31 seconds (1+2+4+8+16).** Is this appropriate for Neon's cold start? (Answer: Neon cold starts typically take 500ms-3s. The aggressive retry schedule is designed for that. For an RDS failover (30-120s), you'd need a longer schedule.)
1340: 
1341: ---
1342: 
1343: ## Chapter 12: Observability
1344: 
1345: ### Context
1346: 
1347: You can't manage what you can't measure. Observability is the practice of making a system's internal state visible from the outside — through logs, metrics, and traces.
1348: 
1349: For the webhook integration feature, observability answers questions like:
1350: - How many events were dispatched?
1351: - How many succeeded? Failed?
1352: - How long did delivery take?
1353: - Which tenants have failing webhooks?
1354: - Is the poller running?
1355: 
1356: ### The Story from bugs/033
1357: 
1358: #### Event Status Lifecycle
1359: 
1360: Every event goes through a status lifecycle that is visible in the database:
1361: 
1362: ```
1363: 'processing' → 'delivered' (success)
1364: 'processing' → 'processing' + attempts++ (failure, retry)
1365: 'processing' → 'failed' (max attempts reached, give up)
1366: ```
1367: 
1368: The event log API (`GET /api/v1/agent/:slug/events`) lets admins query events by status:
1369: 
1370: ```go
1371: func (h *Handler) HandleListEvents(c echo.Context) error {
1372:     status := c.QueryParam("status")
1373:     events, err := h.store.ListAgentEvents(ctx, &store.FindAgentEvent{
1374:         TenantID: &tenantID,
1375:         Status:   &status, // "all", "delivered", "failed", "processing"
1376:     })
1377: }
1378: ```
1379: 
1380: #### The PII Redaction Problem (M1 Revisited)
1381: 
1382: ```go
1383: // Bug: returns full AgentEvent struct including Payload
1384: func (h *Handler) HandleListEvents(c echo.Context) error {
1385:     events, _ := h.store.ListAgentEvents(ctx, &store.FindAgentEvent{TenantID: &tenantID})
1386:     return c.JSON(http.StatusOK, events)
1387: }
1388: ```
1389: 
1390: The `Payload` field contains name, email, phone, session_id. For privacy compliance (GDPR, CCPA), this data should not be returned in list endpoints. The detail view (single event by ID) could include it with proper authorization.
1391: 
1392: **The proper fix:**
1393: 
1394: ```go
1395: type safeEvent struct {
1396:     ID        int32  `json:"id"`
1397:     EventType string `json:"event_type"`
1398:     Status    string `json:"status"`
1399:     Attempts  int32  `json:"attempts"`
1400:     CreatedAt int64  `json:"created_at"`
1401:     // Payload intentionally excluded — contains PII
1402: }
1403: ```
1404: 
1405: #### Monitoring Metrics
1406: 
1407: The code review identified these monitoring concerns:
1408: 
1409: | Metric | What It Measures | Alert Threshold |
1410: |--------|------------------|-----------------|
1411: | Delivery failure rate | Events that failed vs. total | >5% in 5 minutes |
1412: | Poller lease reclaims | Events reclaimed by poller (crashes) | Any occurrence |
1413: | SSRF blocks | URL validation failures | Any occurrence (potential attack) |
1414: | Credential decryption failures | Encryption key issues | Any occurrence |
1415: 
1416: These are not yet instrumented. The feature is scaffold code — observability was deferred. This is a common pattern: build the feature first, add monitoring after. The risk is that you won't know the feature is broken until a user complains.
1417: 
1418: #### Logging with slog
1419: 
1420: ```go
1421: slog.Info("dispatching event",
1422:     "tenant_id", tenantID,
1423:     "integration_id", ig.ID,
1424:     "event_type", eventType,
1425:     "idempotency_key", idempotencyKey,
1426: )
1427: 
1428: slog.Error("webhook delivery failed",
1429:     "event_id", evt.ID,
1430:     "integration_id", ig.ID,
1431:     "error", err,
1432:     "attempts", attempts,
1433: )
1434: ```
1435: 
1436: Structured logging with `slog` makes logs machine-parseable. Each log line is a JSON object with key-value pairs. This enables:
1437: - Searching for specific event IDs across log streams
1438: - Aggregating failure rates by integration
1439: - Tracing a single event's lifecycle
1440: 
1441: ### Critical Thinking Exercise
1442: 
1443: 1. **The event list API returns all fields including payload.** If a non-admin user calls this API, what data do they get? (Answer: full lead PII — name, email, phone, session_id. This is a privacy violation.)
1444: 
1445: 2. **The payload is stored as JSON text in the database.** How would you redact the PII fields for the list API? (Answer: parse the JSON, replace PII fields with "REDACTED", re-serialize. Or add a `payload_redacted` text field to the event struct.)
1446: 
1447: 3. **What log patterns would you search for to detect a broken webhook?** (Answer: `"webhook delivery failed"` with high frequency for a specific integration_id. Or `"dispatch skipped"` — from the idempotency key collision, which would indicate the C1 bug.)
1448: 
1449: 4. **The immediate dispatch goroutine logs success/failure. The poller also logs.** How would you deduplicate these logs for a single event? (Answer: include `event_id` in every log line. A dashboard can group by event_id and show the lifecycle timeline.)
1450: 
1451: ---
1452: 
1453: ## Chapter 13: The Rollback Plan
1454: 
1455: ### Context
1456: 
1457: Every feature should have a rollback plan. Not because you expect to fail, but because planning for failure makes you think about failure modes you might otherwise ignore.
1458: 
1459: ### The Story from bugs/033
1460: 
1461: #### Three Levels of Rollback
1462: 
1463: **Level 1: Toggle Off (Instant, No Code)**
1464: 
1465: The integrations have an `is_active` column. An admin can set any integration to inactive via the UI. This stops all event dispatch to that integration. No code change needed, no deployment.
1466: 
1467: ```sql
1468: UPDATE agent_integrations SET is_active = 0 WHERE tenant_id = ? AND id = ?;
1469: ```
1470: 
1471: **Level 2: Disable Feature (Configuration Change)**
1472: 
1473: If the bug is in the dispatch logic itself (like C1), toggling individual integrations won't help — the dispatch code is still broken. The next option is to disable event dispatching entirely:
1474: 
1475: ```go
1476: // In dispatchEvent — add a config toggle
1477: if !s.profile.EventsEnabled {
1478:     return // Feature disabled globally
1479: }
1480: ```
1481: 
1482: This requires a deployment with the toggle in place, but the toggle itself can be an environment variable or database setting.
1483: 
1484: **Level 3: Code Rollback (Deployment)**
1485: 
1486: If the feature has fundamental issues, roll back the deployment:
1487: 
1488: ```bash
1489: # Rollback to previous deployment
1490: fly deploy --image registry.fly.io/bchat-pg:previous-tag
1491: ```
1492: 
1493: Or via the Fly.io dashboard: "Rollback" button on the deployment history.
1494: 
1495: #### Table Drop (Data Loss)
1496: 
1497: If the schema changes are causing problems (e.g., the event table is growing too fast or blocking other operations), the new tables can be dropped:
1498: 
1499: ```sql
1500: DROP TABLE IF EXISTS agent_events;
1501: DROP TABLE IF EXISTS agent_integrations;
1502: ```
1503: 
1504: This is destructive — all event history is lost. But for v1, events are ephemeral. A lead that was already captured is safe in the `agent_leads` table. Only the webhook delivery history is lost.
1505: 
1506: **The lesson:** Design your schema so that new features can be rolled back without affecting existing data. The `agent_events` table is append-only and can be dropped without affecting any other table.
1507: 
1508: #### The Rollback Flow
1509: 
1510: ```
1511: 1. Detect issue (monitoring alert or user report)
1512: 2. Assess severity:
1513:    - Is it affecting all tenants or one? → Toggle off that integration
1514:    - Is the dispatch logic broken? → Disable feature flag
1515:    - Is the database schema causing problems? → Drop new tables
1516: 3. Mitigate (choose the least destructive option)
1517: 4. Fix the bug in code
1518: 5. Test the fix
1519: 6. Deploy the fix
1520: 7. Re-enable the feature
1521: 8. Postmortem: what went wrong, why wasn't it caught, how to prevent recurrence
1522: ```
1523: 
1524: ### Critical Thinking Exercise
1525: 
1526: 1. **The `is_active` toggle stops new events but doesn't stop in-flight ones.** What happens to events that are already in the delivery goroutine when you toggle off? (Answer: they complete normally. The toggle only affects new dispatches. This is acceptable — you don't want to kill in-flight deliveries.)
1527: 
1528: 2. **If you disable event dispatching for 30 minutes, how many events queue up?** (Answer: depends on traffic. A busy tenant might generate hundreds of leads. The events stay in the table with 'pending' status. When re-enabled, they're dispatched by the next poller run.)
1529: 
1530: 3. **Dropping `agent_events` loses the delivery history.** How would you design a non-destructive rollback? (Answer: instead of dropping, add a `schema_version` column or feature flag that gates the dispatch logic. The data remains but is ignored.)
1531: 
1532: 4. **What's the difference between a rollback and a fix-forward?** (Answer: rollback reverts to a known-good state. Fix-forward applies a corrective change on top of the current state. Fix-forward is preferred when the current state has data that would be lost by rollback.)
1533: 
1534: ---
1535: 
1536: # PART V: THE CRITICAL THINKING HABIT
1537: 
1538: ## Chapter 14: Lessons from the Code Review
1539: 
1540: ### Context
1541: 
1542: The adversarial code review found 11 issues (2 critical, 2 major, 3 moderate, 4 nits). Let's examine each one not for its technical details (we've covered those) but for the *thinking pattern* that caused it.
1543: 
1544: ### The Four Root Causes
1545: 
1546: #### C1: The Idempotency Key Argument Swap — "Parameter Blindness"
1547: 
1548: **Surface cause:** Arguments passed in wrong order to a 3-parameter function.
1549: 
1550: **Root cause:** The developer wrote the call and the function in the same session. Their brain knew what each parameter meant. They never re-read the call with fresh eyes.
1551: 
1552: **Pattern:** This happens when you write code faster than you can verify it. The brain fills in semantics that the text doesn't actually say.
1553: 
1554: **Prevention:**
1555: - Unit test the function with specific inputs and verify outputs
1556: - In code review, make the reviewer check each argument against the function signature
1557: - Consider a type-safe approach: define a `leadID` type and `eventType` type as distinct types to prevent cross-assignment
1558: 
1559: #### C2: The Cancelled Context — "Convenience Over Correctness"
1560: 
1561: **Surface cause:** Used HTTP request context for a background operation.
1562: 
1563: **Root cause:** The developer needed a context and grabbed the closest one — the one from the HTTP handler. They didn't think about the context's lifecycle: "it works in my test, ship it."
1564: 
1565: **Pattern:** The path of least resistance in API design. `c.Request().Context()` is one autocomplete away. `context.Background()` requires a deliberate import.
1566: 
1567: **Prevention:**
1568: - Every context variable should answer: "when does this get cancelled?"
1569: - Background operations need `context.Background()` or `context.WithTimeout(context.Background(), ...)`
1570: - Code review question: "What is the lifecycle of this context?"
1571: 
1572: #### M1: PII Leak — "Inconsistent Security Patterns"
1573: 
1574: **Surface cause:** ListEvents handler returns raw struct with PII payload.
1575: 
1576: **Root cause:** The developer applied the security masking pattern (safeIntegration struct) for the integrations handler but forgot to apply it for the events handler. Inconsistent application of a known pattern.
1577: 
1578: **Pattern:** When you have multiple endpoints that return similar data, applying a security transformation to one but not the other is a common mistake. The brain remembers the pattern but not all the places it applies.
1579: 
1580: **Prevention:**
1581: - Define response structs at the package level, not inline in handlers
1582: - Use a shared "sanitize" function that all list endpoints call
1583: - Code review checklist item: "Have all new endpoints applied the existing security patterns?"
1584: 
1585: #### M2: Shell Compatibility — "Test Like Production"
1586: 
1587: **Surface cause:** Used `${VAR:-default}` syntax that dash doesn't support.
1588: 
1589: **Root cause:** The developer tested the crontab locally (bash) but the Docker image uses Ubuntu (dash as /bin/sh). They never tested the container build.
1590: 
1591: **Pattern:** "Works on my machine" is the most dangerous phrase in software engineering. The production environment is different from your development environment in ways that are invisible until something breaks.
1592: 
1593: **Prevention:**
1594: - `docker build && docker run` before every deployment
1595: - Shell scripts should specify the interpreter: `#!/bin/bash` not `#!/bin/sh`
1596: - Validate the actual container: `docker run --rm bchat-pg cat /etc/bchat/crontab` and verify the output
1597: 
1598: ### The Common Thread
1599: 
1600: All four bugs share a common characteristic: **they are not hard problems.** Nobody would say "designing a webhook system with SSRF protection and HMAC signing is easy." But C1, C2, M1, and M2 are not about the difficult parts of the system. They are about the *easy* parts that were done carelessly.
1601: 
1602: The idempotency key is the core of the deduplication strategy. Getting the arguments wrong makes the entire feature non-functional. But it's a "trivial" bug — swapping two parameters.
1603: 
1604: The pattern repeats: the hard parts (SSRF, HMAC, event schema, migration files) were done correctly. The easy parts (parameter order, context selection, consistent masking, shell syntax) were wrong.
1605: 
1606: **The meta-lesson:** In complex systems, the most damaging bugs are not in the hard parts. They're in the easy parts where you let your guard down.
1607: 
1608: ### The 5-Question Critical Thinking Checklist
1609: 
1610: Before you call a feature "done," ask yourself:
1611: 
1612: 1. **What does "correct" look like?** Write down the expected behavior in plain English. Then check if the code matches. (C1 would have been caught: "every lead should produce a unique idempotency key" vs. "every lead produces the same key.")
1613: 
1614: 2. **What are the failure modes?** List everything that could go wrong. Context cancellation. Network failure. Wrong parameter order. Then check each one. (C2 would have been caught: "what happens when the handler returns while the goroutine is still running?")
1615: 
1616: 3. **How will I know it's broken?** Monitoring. Logging. Tests. If you can't tell whether the feature is working, you haven't finished building it. (C1 was invisible — events appeared to be "dispatched" but were silently deduplicated.)
1617: 
1618: 4. **Who else depends on this?** Upstream callers (the chat handler), downstream consumers (webhook receivers), ops teams (poller, database). Each dependency is a potential failure path.
1619: 
1620: 5. **What's the rollback plan?** If this feature breaks in production, how do you recover? Toggle? Configuration change? Code rollback? If you haven't thought about this, you're not ready to ship.
1621: 
1622: ### Your Turn
1623: 
1624: Take your most recent feature PR.
1625: 
1626: 1. Apply the 5-question checklist above. How many answers do you have?
1627: 2. Identify one "easy" mistake you made (not the hard architectural decisions — the trivial slip-ups).
1628: 3. What was the root cause? (Rushing? Assumption? Untested path?)
1629: 4. What would you add to your personal code review checklist based on this?
1630: 
1631: ---
1632: 
1633: ## Chapter 15: Why AI Agents Didn't Catch These Bugs
1634: 
1635: ### Context
1636: 
1637: This tutorial's subtitle is "Do not outsource thinking to coding agents." This chapter explains why, using bugs/033 as the evidence.
1638: 
1639: ### What AI Agents Are Good At
1640: 
1641: AI coding assistants excel at:
1642: 
1643: - **Pattern matching:** "Write a CRUD handler following the existing pattern" — the AI looks at similar handlers and generates code that matches the style.
1644: - **Boilerplate generation:** "Create a migration file for a new table" — the AI follows the established template.
1645: - **Known vulnerability patterns:** "Check for SQL injection" — the AI recognizes common security antipatterns.
1646: - **Documentation generation:** "Write docstrings for this function" — the AI describes what the code does.
1647: 
1648: These are all *syntactic* tasks. They operate on the surface of the code, not the meaning.
1649: 
1650: ### What AI Agents Are Bad At
1651: 
1652: AI coding assistants struggle with:
1653: 
1654: - **Parameter semantics:** "Is the third argument to this function the eventType or the integrationID?" The AI sees three strings and passes them correctly by coincidence, not by understanding.
1655: - **Context lifecycle:** "Will this context be cancelled before the operation completes?" The AI doesn't trace the execution path to see that the HTTP handler returns before the background goroutine finishes.
1656: - **Security consistency:** "Does this new endpoint apply the same security mask as the existing one?" The AI generates the new code but doesn't cross-reference against all existing patterns.
1657: - **Shell compatibility across environments:** "Does this bash syntax work in dash?" The AI doesn't test the Docker image.
1658: - **Business semantics:** "What does 'correct' mean for this feature?" The AI generates code that compiles, not code that solves the right problem.
1659: 
1660: ### The Evidence from bugs/033
1661: 
1662: The code review prompt that found C1, C2, M1, and M2 was written by a human. It specifically asked:
1663: 
1664: > "1. Does `dispatchEvent` correctly handle the failure path?"
1665: > "2. Is the `ClaimPendingEvents` query correct for both SQLite and Postgres?"
1666: > "3. Does the entrypoint correctly preserve _FILE secret expansion while adding supercronic?"
1667: > "6. Is the idempotency key deterministic and collision-resistant?"
1668: 
1669: These are *system-thinking* questions. They ask about behavior, not syntax. They require understanding the interaction between components, not just the shape of the code.
1670: 
1671: An AI agent generating the code from scratch would have produced the same bugs, because the bugs are in the *semantics* — the developer's understanding of what the code should do, not the code itself.
1672: 
1673: ### The Diagnosis
1674: 
1675: The bugs in bugs/033 exist not because the developer couldn't write Go code. They exist because:
1676: 
1677: 1. **The developer didn't check their assumptions.** "I know the idempotency key takes tenantID, leadID, eventType" — but they passed them in the wrong order.
1678: 2. **The developer didn't trace the execution path.** "The context is fine" — but it was cancelled before the goroutine finished.
1679: 3. **The developer didn't test in the production environment.** "The crontab works locally" — but the container uses a different shell.
1680: 4. **The developer didn't apply existing patterns consistently.** "I masked config in ListIntegrations" — but not in ListEvents.
1681: 
1682: These are not problems that an AI coding agent solves. They are problems that *critical thinking* solves. The AI can generate the code faster, but it generates the wrong code faster too. Speed is not a substitute for correctness.
1683: 
1684: ### What This Means for Your Engineering Practice
1685: 
1686: **Don't use AI as a replacement for thinking.** Use it as a tool for tasks where correctness is easy to verify (boilerplate, documentation, tests for well-understood behavior).
1687: 
1688: **Do invest in:**
1689: - Code review checklists that ask "why" not "what"
1690: - Testing in production-like environments before deploying
1691: - Monitoring that tells you when behavior deviates from expected
1692: - A personal critical thinking habit that outlives any tool
1693: 
1694: **The test:** If you can't explain, in plain English, why each line of AI-generated code is correct, you shouldn't merge it. The AI didn't understand the business context. You do. If you can't verify the AI's work, the AI is not helping — it's hiding complexity from you.
1695: 
1696: ### Your Turn
1697: 
1698: 1. Think about a piece of code you recently accepted from an AI assistant. Do you fully understand every line?
1699: 2. What would a human code reviewer have caught that the AI missed?
1700: 3. If you had to maintain that code for 2 years, would you rather have written it yourself or have the AI generate it?
1701: 4. Write down your personal policy for using AI coding assistants. Where do you draw the line between "tool" and "crutch"?
1702: 
1703: ---
1704: 
1705: ## Chapter 16: Your Engineering Growth Path
1706: 
1707: ### Context
1708: 
1709: This tutorial is almost over. The question is: what do you do with what you've learned?
1710: 
1711: Software engineering is a craft. Like any craft, improvement comes from deliberate practice — not just writing more code, but writing code with intention, reviewing it with honesty, and learning from mistakes.
1712: 
1713: ### The Practices That Matter
1714: 
1715: **1. Code Review Checklists**
1716: 
1717: Don't review code by reading through and hoping to find bugs. Use a checklist. Here's a starting template based on what we learned:
1718: 
1719: ```
1720: ☐ Requirements match: Does the code do what the requirements say?
1721: ☐ Failure modes: What happens when external dependencies fail? (DB, network, API)
1722: ☐ Context lifecycle: Are all contexts bound to the correct lifecycle?
1723: ☐ Security patterns: Are existing security patterns applied consistently?
1724: ☐ Production differences: What changes between dev and prod? (DB, shell, env vars)
1725: ☐ Rollback: Can this change be rolled back without data loss?
1726: ☐ Observability: Would you know if this feature broke?
1727: ```
1728: 
1729: **2. Architecture Decision Records**
1730: 
1731: Every significant decision should be documented. Not for future-you (who will remember) but for future-other-developer (who will not). Use the ADR template from Chapter 3.
1732: 
1733: **3. Pre-Deployment Validation**
1734: 
1735: Before every deployment, run a validation pipeline that checks:
1736: - Migrations apply cleanly
1737: - Environment variables are set
1738: - Tests pass
1739: - Docker build succeeds
1740: 
1741: This is not optional. It's the difference between shipping with confidence and shipping with hope.
1742: 
1743: **4. The Engineering Journal**
1744: 
1745: After every significant incident or bug fix, write down:
1746: - What happened?
1747: - What was the root cause?
1748: - Why wasn't it caught earlier?
1749: - What process change would prevent this in the future?
1750: 
1751: The `bugs/` directory in this codebase is a form of engineering journal. Each bug directory contains the original prompt, plans, reviews, and outcomes. It's not just a record of what went wrong — it's a teaching tool for the next engineer.
1752: 
1753: ### Using bugs/ As Your Personal Case Library
1754: 
1755: The `bugs/` directory at `/home/chaschel/Documents/go/bchat/bugs/` contains 33 entries. Each one documents a real issue:
1756: 
1757: - What was the problem?
1758: - What plan was proposed?
1759: - How was it reviewed?
1760: - What was the outcome?
1761: 
1762: Spend an hour reading through the first 10 entries. Look for patterns:
1763: 
1764: - What types of bugs repeat? (Parameter swaps? Context misuse? Missing tests?)
1765: - What types of plans succeed? (Detailed? Vague? Tested? Untested?)
1766: - What do the good code reviews look for?
1767: 
1768: This is a case library of real engineering failures. You don't need to make these mistakes yourself to learn from them.
1769: 
1770: ### Final Exercise
1771: 
1772: Take the bchat codebase and `AGENTS.md` document. It contains a checklist for adding new features:
1773: 
1774: > ### Adding Tenant-Scoped Features
1775: > 1. Database Layer: Add `tenant_id` column, index, struct field
1776: > 2. Store Layer: Add `tenant_id` to queries
1777: > 3. Handler Layer: Extract tenant ID from context
1778: > 4. Security: Add superuser bypass check
1779: 
1780: Based on everything you learned from bugs/033, what's missing from this checklist?
1781: 
1782: Here's my answer: the checklist is focused on *what to do* but not *what to verify*. It doesn't include:
1783: 
1784: - How do you test cross-tenant access denial? (Write a test)
1785: - How do you verify the context lifecycle is correct? (Tracing exercise)
1786: - How do you know the feature is working in production? (Monitoring)
1787: - What's the rollback plan? (Document it)
1788: 
1789: Your answer may differ. That's the point. **Critical thinking is not about having the right answers — it's about asking the right questions.**
1790: 
1791: ---
1792: 
1793: # APPENDICES
1794: 
1795: ## A.1: bugs/033 Code Review Findings (Annotated)
1796: 
1797: | ID | Severity | Finding | Root Cause | Prevention |
1798: |----|----------|---------|------------|------------|
1799: | **C1** | Critical | `computeIdempotencyKey` arguments swapped — all events for same integration produce same key | Parameter blindness: developer wrote call without checking signature | Unit test with distinct inputs. Reviewer verifies each arg against signature. |
1800: | **C2** | Critical | `processEventPoller` uses `c.Request().Context()` which is cancelled when handler returns | Context lifecycle not traced: developer used nearest context without considering its lifetime | Review question: "When does this context get cancelled?" Background ops use `context.Background()`. |
1801: | **M1** | Major | Event list API returns raw struct including `Payload` with PII | Inconsistent security: masking pattern applied to integrations but not events | Shared sanitize function. Code review checklist item for security patterns. |
1802: | **M2** | Major | Crontab `${PORT:-5230}` syntax unsupported by dash (`/bin/sh` on Ubuntu) | Test-like-production failure: dev uses bash, container uses dash | Test the actual Docker image. Use `#!/bin/bash`. Validate crontab in container build. |
1803: | M3 | Moderate | `dispatchEvent` error return unused by callers — dead code | Error handling not thought through: what should callers do with this error? | Design error flow before implementation. Return structured results. |
1804: | M4 | Moderate | SQLite `ClaimPendingEvents` acquires write lock on entire table | Single-writer assumption accepted for v1. Acceptable at scale. | Document limitation. Add per-tenant claim limit for v1.1. |
1805: | M5 | Moderate | Token length check creates timing oracle for `CRON_TOKEN` length | Over-optimization: length check before constant-time compare is unnecessary | Remove pre-check. Let `ConstantTimeCompare` handle everything. |
1806: | N1 | Nit | `processEventPoller` returns void — no error propagation | Function either succeeds or logs. Caller can't detect failure. | Return error. Let caller decide how to handle. |
1807: | N2 | Nit | MySQL stubs need build tags for safety | Stubs compile for all builds, but MySQL driver may be excluded | Verify build tags exist. If not, add `//go:build mysql`. |
1808: | N3 | Nit | Frontend uses browser-native `confirm()` for delete | Not using Joy UI AlertDialog/Modal component | Replace with styled modal for consistency. |
1809: | N4 | Nit | `RefreshCwIcon` may be wrong lucide-react export name | Typo or convention mismatch in icon name | Verify export names match project conventions. |
1810: 
1811: ## A.2: bugs/033 Complete File Inventory
1812: 
1813: | # | File | Lines | Purpose | Status |
1814: |---|------|-------|---------|--------|
1815: | 1 | `store/migration/sqlite/0.31/00__agent_integrations.sql` | ~20 | SQLite schema: agent_integrations table | Committed |
1816: | 2 | `store/migration/sqlite/0.31/01__agent_events.sql` | ~30 | SQLite schema: agent_events table (outbox queue) | Committed |
1817: | 3 | `store/migration/postgres/0.31/00__agent_integrations.sql` | ~20 | Postgres schema: agent_integrations | Committed |
1818: | 4 | `store/migration/postgres/0.31/01__agent_events.sql` | ~25 | Postgres schema: agent_events | Committed |
1819: | 5 | `store/agent.go` | ~40 (added) | AgentIntegration, AgentEvent, Find*, WebhookConfig structs | Committed |
1820: | 6 | `store/driver.go` | ~15 (added) | 9 interface methods | Committed |
1821: | 7 | `store/db/sqlite/agent.go` | ~200 (added) | SQLite implementations | Committed |
1822: | 8 | `store/db/postgres/agent.go` | ~200 (added) | Postgres implementations | Committed |
1823: | 9 | `store/db/postgres/resilience.go` | 85 | `RunResiliently`, `isTransientError` | Committed |
1824: | 10 | `store/db/mysql/agent.go` | ~10 (added) | Stubs returning errNotImplemented | Committed |
1825: | 11 | `server/router/api/v1/agent/integrations.go` | 532 | Handlers: CRUD + test + trigger-cron + delivery | Committed |
1826: | 12 | `server/router/api/v1/agent/integrations_test.go` | 131 | Tests: HMAC, SSRF, idempotency key | Committed |
1827: | 13 | `server/router/api/v1/agent/service.go` | ~100 (added) | `dispatchEvent`, `processEventPoller` | Committed |
1828: | 14 | `server/router/api/v1/v1.go` | ~15 (added) | Route registration | Committed |
1829: | 15 | `build/crontab` | 2 | Supercronic schedule: */5 * * * * | Committed |
1830: | 16 | `scripts/entrypoint.sh` | ~10 (added) | Supercronic launch | Committed |
1831: | 17 | `Dockerfile.pg.fly` | ~10 (added) | Supercronic install + crontab copy | Committed |
1832: | 18 | `web/src/store/v2/agentAdmin.ts` | ~400 (added) | Integration + event state, CRUD methods | Committed |
1833: | 19 | `web/src/pages/AgentAdminSections/IntegrationsSection.tsx` | 296 | Integration management UI component | Committed |
1834: | 20 | `web/src/pages/AgentAdmin.tsx` | ~10 (added) | Import IntegrationsSection | Committed |
1835: 
1836: ## A.3: Deployment Cheat Sheet — Fly.io + Neon
1837: 
1838: ```bash
1839: # =========================================================================
1840: # PREREQUISITES
1841: # =========================================================================
1842: # - fly CLI installed: fly --version
1843: # - Logged in: fly auth whoami
1844: # - App created: fly launch (first time)
1845: # - Neon project created with connection string
1846: 
1847: # =========================================================================
1848: # LOCAL DEVELOPMENT (SQLite)
1849: # =========================================================================
1850: cp .env.example .env          # Fill in OPENROUTER_API_KEY
1851: task setup                     # Install deps
1852: task run                       # Start with SQLite at localhost:8081
1853: 
1854: # =========================================================================
1855: # MIGRATION VALIDATION
1856: # =========================================================================
1857: task -t Taskfile_pg.yml postgres:start          # Start local Postgres
1858: task -t Taskfile_pg.yml validate:migrations     # Validate all migrations
1859: 
1860: # =========================================================================
1861: # SET FLY SECRETS
1862: # =========================================================================
1863: ./scripts/fly-pg-secrets.sh                     # Interactive: sets all secrets
1864: 
1865: # Manual alternative:
1866: fly secrets set DATABASE_URL="postgresql://user:pass@ep-xxx.neon.tech/neondb?sslmode=require"
1867: fly secrets set OPENROUTER_API_KEY="sk-or-v1-xxx"
1868: fly secrets set ENCRYPTION_MASTER_KEY="$(uuidgen)"
1869: fly secrets set CRON_TOKEN="$(openssl rand -hex 32)"
1870: 
1871: # =========================================================================
1872: # BUILD AND DEPLOY
1873: # =========================================================================
1874: task fly:pre-deploy              # Validates env + migrations
1875: fly deploy                      # Build and deploy
1876: 
1877: # =========================================================================
1878: # VERIFY
1879: # =========================================================================
1880: fly logs | grep -E "RAG|LanceDB|Embedding|Reindex"  # Check startup
1881: fly status                      # Check app status
1882: curl https://bchat-pg.fly.dev/healthz               # Health check
1883: 
1884: # =========================================================================
1885: # DEBUG
1886: # =========================================================================
1887: fly ssh console -C "sqlite3 /var/opt/memos/memos_prod.db .tables"  # SQLite (if not Postgres)
1888: fly logs                        # Full log stream
1889: fly logs | grep -E "event|webhook|cron"            # Event-specific logs
1890: 
1891: # =========================================================================
1892: # ROLLBACK
1893: # =========================================================================
1894: # Option 1: Toggle integration inactive (Admin UI)
1895: # Option 2: Rollback to previous version
1896: fly releases                    # List deployments
1897: fly deploy --image registry.fly.io/bchat-pg:previous-tag
1898: ```
1899: 
1900: ## A.4: Monitoring & Alerting Reference
1901: 
1902: | Metric | Source | Query / Check | Alert When |
1903: |--------|--------|---------------|------------|
1904: | Lead capture | `agent_leads` | Count by tenant per hour | Drop >80% (feature broken) |
1905: | Event dispatch | `agent_events` | Count by status | Failure rate >5% |
1906: | Event latency | Logs | `"webhook delivery completed"` timestamp | p95 > 1s |
1907: | Poller health | Logs | `"processing poller events"` | No log in 10 min |
1908: | SSRF blocks | Logs | `"webhook URL validation failed"` | Any occurrence |
1909: | DB connection | Health check | `/healthz` return 200 | Non-200 response |
1910: | Cold start | Logs | Server startup time | > 5s |
1911: | RunResiliently | Logs | `"retry after transient error"` | > 3 in 5 min |
1912: 
1913: ## A.5: Glossary
1914: 
1915: | Term | Definition | Example in bugs/033 |
1916: |------|------------|-------------------|
1917: | **Outbox Pattern** | Write events to a table (the "outbox") before publishing them. Guarantees at-least-once delivery because the event survives crashes. | `agent_events` table |
1918: | **Idempotency** | An operation that produces the same result whether performed once or multiple times. | `idempotency_key` column with UNIQUE constraint |
1919: | **SSRF** | Server-Side Request Forgery — an attack where the server is tricked into making requests to internal systems. | Three-layer defense: URL validation, IP pinning, redirect re-validation |
1920: | **HMAC** | Hash-based Message Authentication Code — a cryptographic signature that verifies the integrity and authenticity of a message. | `X-Bchat-Signature: sha256=<hex>` header |
1921: | **SKIP LOCKED** | A Postgres locking mechanism that skips rows locked by other transactions. | `FOR UPDATE SKIP LOCKED` in Postgres claim query |
1922: | **Lease-Based Claiming** | A pattern where a worker claims a task for a fixed period. If the worker doesn't complete the task before the lease expires, another worker reclaims it. | `claimed_at` column, 300-second lease, poller reclaims expired leases |
1923: | **Pre-Claimed Event** | An event inserted with `status = 'processing'` rather than `'pending'`. The insertion itself is the claim. | ADR-3 decision |
1924: | **Scale-to-Zero** | A deployment model where the application shuts down when not in use, saving costs. | Fly.io `auto_stop_machines = 'stop'` |
1925: | **Jitter** | Random variation added to retry delays to prevent thundering herd. | ±25% on exponential backoff in `RunResiliently` |
1926: | **SQLSTATE** | Postgres error codes. 5-character strings identifying error types. | 57P01 (admin_shutdown), 08006 (connection_failure) |
1927: 
1928: ## A.6: bugs/033 Decision Log
1929: 
1930: | Decision | Date | Outcome | Hindsight |
1931: |----------|------|---------|-----------|
1932: | Webhook-first vs. native integrations | 2026-07 | Webhook-first | Correct — avoids months of integration work. Inbound API needed. |
1933: | Supercronic vs. in-process cron | 2026-07 | Supercronic | Correct for container model. M2 bug (shell compat) was avoidable. |
1934: | Pre-claimed events | 2026-07 | Pre-claimed | Correct — eliminates crash-loss window. |
1935: | JSON config column | 2026-07 | JSON blob | Acceptable for v1. Migrate to normalized columns in v2. |
1936: | Idempotency key = hash(tenantID, leadID, eventType) | 2026-07 | Committed with C1 bug | Correct in design, broken in implementation. Need unit test. |
1937: | Event payload includes PII | 2026-07 | Full payload | M1 bug: event list API must redact. Fix: safeEvent struct. |
1938: | MySQL stubs | 2026-07 | errNotImplemented | Correct — compile safety without runtime support. |
1939: | SQLite claim without SKIP LOCKED | 2026-07 | Accepted limitation | Correct for v1 single-writer. Documented. |
1940: | Awake-only retries | 2026-07 | Accepted limitation | Correct cost-saving decision. Documented in v1 contract. |
1941: | `kill_timeout = "30s"` | 2026-07 | fly.toml | Correct. Placement must be top-level. |
1942: 
1943: ---
1944: 
1945: ## Closing
1946: 
1947: The bugs/033 feature — Native Webhook Integrations v1 — is currently scaffolded but not stress-tested in production. The migrations run, the handlers respond, the UI renders. But the critical bugs (C1, C2) mean the feature would not work correctly if deployed today.
1948: 
1949: This is not a failure. This is *normal software development*. Every feature goes through this cycle: idea → plan → implementation → review → bugfix → deployment → monitoring → iteration. The quality of the outcome depends not on how few bugs you introduce, but on how quickly you find and fix them.
1950: 
1951: The purpose of this tutorial is to speed up the "find and fix" part of that cycle. By thinking critically — by asking the right questions before, during, and after implementation — you catch the C1s and C2s before they reach production.
1952: 
1953: **The most important skill in software engineering is not writing code. It is thinking about code.**
1954: 
1955: ---
1956: 
1957: *Tutorial written from the bchat codebase. Study the actual files at `/home/chaschel/Documents/go/bchat/bugs/033/` for the complete case record.*

(End of file - total 1957 lines)
</content>
```

---

## User

---

## Assistant (Compaction · MiMo V2.5 Free · 32.4s)

_Thinking:_

Let me carefully analyze the entire conversation history to create a comprehensive anchored summary.

## Objective
- Plan and write `DOCS_ONBOARDING.md` — a comprehensive guide for bchat implementation specialists to onboard new customers end-to-end (gather requirements, design KB.MD/POLICY.MD/SCRIPT.MD with LLM help, set up chat widget on CMS platforms, configure RAG, deploy and maintain intelligent AI agents)
- The conversation has been a Q&A planning phase to align on scope and structure before writing

## Important Details
- **Audience for DOCS_ONBOARDING.md:** Implementation specialists — product people with technical skills, NOT developers/ops. They need to understand nuances of bchat features without coding
- **Goal:** Keep customers up and running in no time — fast onboarding, zero friction
- **Customer diversity:** Must handle all combinations — RAG-based knowledge bases, intelligent transactional AI agents, various CMS platforms (Hugo, WordPress, Shopify, Wix, Squarespace), various verticals (plumbers, electricians, restoration, HVAC, insurance, HR, legal, etc.)
- **Three configuration files** define all tenant behavior: KB.MD, POLICY.MD, SCRIPT.MD
- **bchat features to cover:** Multi-tenant architecture, RAG pipeline (LanceDB), hybrid search (70% vector + 30% BM25), observational memory, RBAC, webhook integrations, simulation/testing, widget embedding (3 methods: IIFE, iframe, npm), dual-audience (internal/external), Hallucination Grounding (HGL), Context Awareness, Conversation Memory, conversation replay, lead capture
- **Widget embedding:** 3 methods — IIFE (`<script>`), iframe (low-trust sites), npm (React apps). Build produces `embed.min.js` (~15-20KB). Configurable via `data-bchat-*` attributes
- **CMS integration:** Hugo (widget partial + managed Hugo sites via `doctl`), WordPress (plugin or footer.php manual), generic HTML (paste before `</body>`)
- **Existing tutorials:** TUTORIAL.md (bugs/033 case study), TUTORIAL_DATABASE.md (SQLite/Postgres/MySQL parity), TUTORIAL_CX.md (customer experience), TUTORIAL_RAG.md (dogfooding exercise), TUTORIAL_MIMO.md (holistic SDLC + business guide)
- **TUTORIAL_MIMO.md** was just written (1,487 lines, ~57KB) as the comprehensive developer tutorial — covers product thinking through business ownership
- **The onboarding doc is distinct from tutorials** — it's a practitioner's playbook, not a learning curriculum
- **LLM-assisted content creation:** Implementation specialists should use LLMs to help draft/polish KB.MD, POLICY.MD, SCRIPT.MD from gathered customer info
- **Key bchat annotations for KB.MD:** `@service`, `@faq`, `@exclusion`, `@coverage`, `@safety`, `@section`
- **Key bchat annotations for POLICY.MD:** `@identity`, `@intent`, `@rule`, `@thresholds`, `@settings`
- **SCRIPT.MD:** Defines conversation flow (opening → discovery → information → qualification → closing)
- **Widget auth:** JWT-based via `x-bchat-auth` header (production) or HMAC via query param (development)
- **Widget CORS:** `allowed_domains` JSON array on `agent_tenants` table controls which domains can embed

## Work State
### Completed
- Full codebase analysis of bchat architecture, features, and capabilities
- Complete competitive analysis (Intercom AI, Helios AI, n8n, Zapier)
- bugs/033 deep analysis (5 plan iterations, code review findings, webhook integrations feature)
- TUTORIAL_MIMO.md written (1,487 lines) — holistic SDLC tutorial with 19 chapters + appendices
- RAG pipeline analysis (vector DB, hybrid search, embedding, chunking, fusion engine)
- Widget system analysis (3 embedding methods, iframe communication, customization options)
- Business model analysis (5 models, pricing tiers, margin analysis)
- Onboarding workflow analysis (8 phases identified from existing docs)
- Complete feature inventory compiled from all docs and source code
- All 5 TUTORIAL* files read for full context

### Active
- Planning DOCS_ONBOARDING.md structure and content — Q&A phase in progress
- User asked to read all TUTORIAL* files for context before finalizing the plan
- Context gathering complete, ready to present the plan for alignment

### Blocked
- (none)

## Next Move
1. Present the structured plan for DOCS_ONBOARDING.md to the user — covering all onboarding phases, CMS integrations, LLM-assisted content creation, RAG configuration, widget setup, and ongoing maintenance
2. After user approval, write the complete DOCS_ONBOARDING.md file

## Relevant Files
- `/home/chaschel/Documents/go/bchat/TUTORIAL_MIMO.md`: Just-written holistic tutorial (1,487 lines) — the companion piece to the onboarding doc
- `/home/chaschel/Documents/go/bchat/TUTORIAL.md`: Original bugs/033 case study tutorial
- `/home/chaschel/Documents/go/bchat/TUTORIAL_DATABASE.md`: SQLite/Postgres/MySQL parity tutorial
- `/home/chaschel/Documents/go/bchat/TUTORIAL_CX.md`: Customer experience and AI support tutorial
- `/home/chaschel/Documents/go/bchat/TUTORIAL_RAG.md`: RAG dogfooding exercise
- `/home/chaschel/Documents/go/bchat/docs/KB_TEMPLATE.MD`: KB template with annotation reference
- `/home/chaschel/Documents/go/bchat/docs/POLICY_TEMPLATE.MD`: POLICY template with annotation reference
- `/home/chaschel/Documents/go/bchat/docs/DOCS_WIDGET.MD`: Widget embedding guide
- `/home/chaschel/Documents/go/bchat/docs/docs_biz_guide.md`: Business owner guide
- `/home/chaschel/Documents/go/bchat/docs/BIZ_QUICK_START_CHECKLIST.MD`: Quick start checklist
- `/home/chaschel/Documents/go/bchat/docs/DOCS_SIMULATION.MD`: Simulation/testing features
- `/home/chaschel/Documents/go/bchat/docs/DOCS_AGENT_ARCHITECTURE.MD`: Configuration-driven design
- `/home/chaschel/Documents/go/bchat/docs/DOCS_RAG_PIPELINE.MD`: RAG configuration
- `/home/chaschel/Documents/go/bchat/docs/DOCS_RBAC_SYSTEM.MD`: Permission system
- `/home/chaschel/Documents/go/bchat/docs/DOCS_HALLUCINATION_GROUNDING.MD`: HGL system
- `/home/chaschel/Documents/go/bchat/docs/DOCS_CONTEXT_AWARENESS.MD`: Context awareness
- `/home/chaschel/Documents/go/bchat/widget/`: Widget source code (IIFE/embed builder)
- `/home/chaschel/Documents/go/bchat/bugs/033/`: Webhook integrations feature (reference for onboarding scenarios)

---

## User

---

## Assistant (Plan · MiMo V2.5 Free · 21.3s)

_Thinking:_

The user is asking me to continue or ask for clarification. Looking at the context, we've been planning DOCS_ONBOARDING.md. The previous assistant message summarized what we've done so far and indicated we should "Present the structured plan for DOCS_ONBOARDING.md to the user."

I'm in plan mode (read-only), so I need to present the plan for DOCS_ONBOARDING.md and ask for alignment before proceeding to write it.

Let me present a structured plan for the onboarding document based on all the research and context gathering we've done.

Here's the proposed structure for `DOCS_ONBOARDING.md`. Let me know if this aligns with your vision or if you want adjustments:

---

## Proposed Plan: DOCS_ONBOARDING.md

**Purpose:** Practitioner's playbook for implementation specialists to onboard new customers — from first call to live AI agent.

### Structure (18 sections)

**Phase 1 — Discovery (gather requirements)**
1. **Welcome & Scope** — What this doc covers, who it's for, what you'll deliver
2. **Customer Intake Checklist** — Structured questionnaire: vertical, business model, goals, existing CMS, tech comfort level, timeline
3. **Feature Selection Matrix** — Decision tree: which bchat features to activate per customer type (RAG-only, transactional AI, hybrid, internal vs external, multi-tenant vs single)

**Phase 2 — Content Creation (LLM-assisted)**
4. **KB.MD Design** — How to draft knowledge base content using LLM assistance; annotation reference (`@service`, `@faq`, `@exclusion`, `@coverage`, `@safety`, `@section`); anti-hallucination principles; completeness checklist
5. **POLICY.MD Design** — How to define agent identity, intent routing, behavioral rules, thresholds; annotation reference (`@identity`, `@intent`, `@rule`, `@thresholds`, `@settings`); tone/persona guidance
6. **SCRIPT.MD Design** — Conversation flow design; stage templates (opening → discovery → qualification → closing); multi-turn vs single-turn patterns; when to use scripts vs policy-driven behavior
7. **LLM Prompting Recipes** — Copy-paste prompts for drafting each file type; before/after examples; iterative refinement workflow

**Phase 3 — Technical Setup**
8. **Tenant Provisioning** — Creating tenant via API/UI; slug selection; GUID management; RBAC user setup
9. **RAG Configuration** — When to enable RAG; embedding provider selection; indexing strategy; hybrid search tuning; force reindex workflow
10. **Observational Memory** — When to enable OM; token thresholds; behavior with/without OM
11. **Webhook Integrations** — Outbound webhooks (CRM, ticketing, calendar); inbound webhooks; configuration patterns

**Phase 4 — Widget Deployment**
12. **Widget Embedding — Universal Guide** — IIFE method (default, most sites), iframe method (low-trust/Shopify/Wix), npm method (React apps); `data-bchat-*` configuration attributes; CORS/allowed domains setup
13. **CMS-Specific Guides** — Hugo (partial + managed Hugo via doctl), WordPress (plugin or manual), Shopify (theme.liquid), Wix/Squarespace (iframe), generic HTML
14. **Widget Customization** — Theming, positioning, welcome messages, pre-chat forms; brand-matching checklist
15. **Widget Testing** — Local testing, staging environment, cross-browser checklist, mobile responsiveness

**Phase 5 — Validation & Launch**
16. **Simulation Testing** — Running simulations, interpreting results, iterating on KB/POLICY/SCRIPT; scoring criteria
17. **Pre-Launch Checklist** — Security review, domain verification, RAG quality spot-checks, conversation replay review, rollback plan
18. **Go-Live & Ongoing Maintenance** — Monitoring, re-indexing triggers, content refresh cadence, A/B testing, scaling considerations

**Appendices**
- A: Annotation Quick Reference Card
- B: Common Customer Patterns (5-6 archetypes with example configs)
- C: Troubleshooting Matrix
- D: Glossary

---

### Questions before I write this:

1. **Scope:** Should the appendices be in this doc or separate files? (I'd recommend keeping them in-file for a self-contained practitioner reference)
2. **CMS depth:** Should each CMS guide be a full walkthrough or a concise checklist with "see widget docs for details"?
3. **Template files:** Should I create actual template KB/POLICY/SCRIPT files (as separate `.md` files in `docs/templates/`) or inline examples in the doc?
4. **Naming:** `DOCS_ONBOARDING.MD` — confirm?

---

## User

1) in-file 2) concise checklist 3) separate in docs/templates 4) DOCS_ONBOARDING.md

---

## Assistant (Plan · MiMo V2.5 Free · 5.7s)

_Thinking:_

The user confirmed the structure choices. Now I need to present a more detailed plan for DOCS_ONBOARDING.md before writing it. Let me also look at the existing doc templates and docs to make sure my plan aligns with what already exists.

Got it — appendices in-file, concise CMS checklists, templates in `docs/templates/`, filename `DOCS_ONBOARDING.md`. Let me do a quick scan of the existing docs to make sure my plan doesn't duplicate and identifies the right templates to create.

**Tool: glob**

**Input:**
```json
{
  "pattern": "docs/DOCS_*.MD"
}
```

**Output:**
```
/home/chaschel/Documents/go/bchat/docs/DOCS_RAG_MINIMAX25.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_LANCEDB_CHATGPT0.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_CREATE_TENANT_BUG.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_RBAC.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_README.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_CHAT_DESIGN_2.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_DOCKERFILE.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_CHAT_PAGE.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_POCKETFLOW.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_CHAT_DESIGN.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_CHAT_DESIGN_3.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_PENDING.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_DEPLOYMENT_LOGS.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_WORKFLOW.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_RAG_CLARIFICATION.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_LANCEDB.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_TASKFILEWIN.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_DEBUG_REBUILD_INDEX.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_OM_RAG_IMP.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_AGENT_CODE_AUDIT.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_RAG_GLM5.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_PROCESSING.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_LANCEDB_PHASE3.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_PROMPT.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_OM_IMPLEMENTATION.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_RAG_OVERVIEW.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_GIT_FIX.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_CHANGES_20260214.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_LANCEDB_PHASE5.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_HYBRID_SEARCH.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_QA.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_DEPLOYMENT.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_CONTEXT_AWARENESS.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_OM_GLM5.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_LANCEDB_CHATGPT.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_CHAT_DESIGN_4_IMP.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_UI_1.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_LANCEDB_PHASE1.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_LANCEDB_VISUALIZATION.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_LANCEDB_STORAGE.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_CHAT_DESIGN_4_IMP_2-PROGRESS.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_LOGS.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_TASKFILE.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_WIDGET.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_SIMULATION_ADDENDUM_2.24.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_RAG_PIPELINE.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_RAG_PROCESS.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_BUGS_20260130_2204.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_OM_MINIMAX_PLAN.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_OM_MINIMAX.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_SIMULATION_2.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_RAG_TO_INDEX.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_CHAT_DESIGN_4_IMP_2.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_CONTACT_SANITIZATION_REFACTOR.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_LANCEDB_PHASE2.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_UNIFIED_ENV_WORKFLOW.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_FLY.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_HTML2MD.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_ADMIN_PAGE.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_HYBRID_SEARCH_WEEK1.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_RAG_FIX.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_CHUNKING.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_SIMULATION_CLARIFY.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_PARSE.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_SIMULATION_ADDENDUM_2.21.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_TASKFILE_PG.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_OM_ANALYSIS.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_CHAT_DESIGN_4.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_REINDEX_RESUME.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_SIMULATION.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_SIMULATION_2.25.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_TEST_WIDGET.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_RBAC_2.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_LANCEDB_PHASE4.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_RAG_ARCEE.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_EVALUATION.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_ENV_VAR.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_BUGS_REBUILD_INDEX.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_HALLUCINATION_GROUNDING.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_SAVE_TRANSCRIPT.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_EMBEDDING.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_BUGS.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_INITIAL_CHAT_PAGE.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_ANALYSIS.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_ENDPOINTS.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_LANCE_EMBEDDING.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_ROLLOUT.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_BUGS_QA.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_RAG_ALTERNATIVES.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_OM_RAG_ARCEE.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_AGENT_ARCHITECTURE.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_HYBRID_SEARCH_EXPLAINED.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_SIMULATION_2.26.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_DEPLOY_FLY.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_DATABASE_MIGRATION.MD
/home/chaschel/Documents/go/bchat/docs/DOCS_FIXES.MD
```

**Tool: glob**

**Input:**
```json
{
  "pattern": "docs/*TEMPLATE*.MD"
}
```

**Output:**
```
/home/chaschel/Documents/go/bchat/docs/POLICY_TEMPLATE.MD
/home/chaschel/Documents/go/bchat/docs/KB_TEMPLATE.MD
```

**Tool: glob**

**Input:**
```json
{
  "pattern": "docs/templates/**"
}
```

**Output:**
```
/home/chaschel/Documents/go/bchat/docs/templates/PROMPT_ADVANCED.TXT
/home/chaschel/Documents/go/bchat/docs/templates/BASE_PROMPT.TXT
/home/chaschel/Documents/go/bchat/docs/templates/POLICY.MD
/home/chaschel/Documents/go/bchat/docs/templates/SCRIPT_OPUS13.MD
/home/chaschel/Documents/go/bchat/docs/templates/SYSTEM_PROMPT.TXT
/home/chaschel/Documents/go/bchat/docs/templates/PROMPT_QA.TXT
/home/chaschel/Documents/go/bchat/docs/templates/GUIDELINES.MD
/home/chaschel/Documents/go/bchat/docs/templates/PROMPTS.MD
/home/chaschel/Documents/go/bchat/docs/templates/PROMPT_FAQ.TXT
/home/chaschel/Documents/go/bchat/docs/templates/KB.MD
/home/chaschel/Documents/go/bchat/docs/templates/PROMPT_BASIC.TXT
/home/chaschel/Documents/go/bchat/docs/templates/examples/livio/Why What Makes Us Curious.txt
/home/chaschel/Documents/go/bchat/docs/templates/examples/rizal/SCRIPT.MD
/home/chaschel/Documents/go/bchat/docs/templates/examples/rizal/noli.txt
/home/chaschel/Documents/go/bchat/docs/templates/examples/bible/bible-bsb.txt
```

**Tool: glob**

**Input:**
```json
{
  "pattern": "TUTORIAL*.md"
}
```

**Output:**
```
/home/chaschel/Documents/go/bchat/TUTORIAL_RAG.md
/home/chaschel/Documents/go/bchat/TUTORIAL.md
/home/chaschel/Documents/go/bchat/TUTORIAL_DATABASE.md
/home/chaschel/Documents/go/bchat/TUTORIAL_CX.md
/home/chaschel/Documents/go/bchat/TUTORIAL_MIMO.md
```

---

## Assistant (Plan · MiMo V2.5 Free · 3.7s)

_Thinking:_

Good, I can see that `docs/templates/` already exists with some templates. Let me check the existing templates to understand what's already there, and also check the existing onboarding-related docs.

Good — `docs/templates/` already exists with some files. Let me check what's there and what onboarding docs already exist.

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/docs/templates"
}
```

**Output:**
```
<path>/home/chaschel/Documents/go/bchat/docs/templates</path>
<type>directory</type>
<entries>
BASE_PROMPT.TXT
examples/
GUIDELINES.MD
KB.MD
POLICY.MD
PROMPT_ADVANCED.TXT
PROMPT_BASIC.TXT
PROMPT_FAQ.TXT
PROMPT_QA.TXT
PROMPTS.MD
SCRIPT_OPUS13.MD
SYSTEM_PROMPT.TXT

(12 entries)
</entries>
```

**Tool: glob**

**Input:**
```json
{
  "pattern": "docs/*ONBOARD*.MD"
}
```

**Output:**
```
No files found
```

**Tool: glob**

**Input:**
```json
{
  "pattern": "docs/*BIZ*.MD"
}
```

**Output:**
```
/home/chaschel/Documents/go/bchat/docs/BIZ_QUICK_START_CHECKLIST.MD
```

**Tool: glob**

**Input:**
```json
{
  "pattern": "docs/*QUICK_START*.MD"
}
```

**Output:**
```
/home/chaschel/Documents/go/bchat/docs/BIZ_QUICK_START_CHECKLIST.MD
```

---

## Assistant (Plan · MiMo V2.5 Free · 5.2s)

_Thinking:_

Let me check the existing templates to understand what we already have, and the existing business-related docs.

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/docs/templates/KB.MD"
}
```

**Output:**
```
<path>/home/chaschel/Documents/go/bchat/docs/templates/KB.MD</path>
<type>file</type>
<content>
1: # [Company Name] Knowledge Base
2: 
3: > **AI Chat Agent Reference**
4: > Last Updated: [Date]
5: 
6: ---
7: 
8: ## Company Overview
9: 
10: **[Company Name]** is [region]'s premier [service type] company. We provide [brief description of services] for residential and commercial properties.
11: 
12: | Attribute | Details |
13: |-----------|---------|
14: | **Company Name** | [Company Name] |
15: | **Service Type** | [e.g., Property Damage Restoration] |
16: | **Certifications** | [e.g., IICRC Certified] |
17: | **Availability** | [e.g., 24/7/365 Emergency Service] |
18: 
19: ### Our Commitment
20: 
21: [Brief mission statement or value proposition - 1-2 sentences]
22: 
23: ---
24: 
25: ## Contact Information
26: 
27: | Type | Value |
28: |------|-------|
29: | **Emergency Phone** | **(555) 123-4567** |
30: | **Toll-Free** | [Optional toll-free number] |
31: | **Email** | info@company.com |
32: | **Address** | [Street Address, City, ST ZIP] |
33: 
34: ---
35: 
36: ## Services
37: 
38: ### Water Damage Restoration
39: <!-- @service: water_damage, emergency: true -->
40: 
41: We provide emergency water damage restoration services including water extraction, drying, and restoration.
42: 
43: **Common Causes:**
44: - Flooding and storms
45: - Burst or leaking pipes
46: - Appliance failures
47: - Sewage backups
48: 
49: **Response Time:** 30 minutes for emergencies
50: 
51: ---
52: 
53: ### Fire & Smoke Damage Restoration
54: <!-- @service: fire_damage, emergency: true -->
55: 
56: Complete restoration services for fire, smoke, and soot damage including structural repairs and content restoration.
57: 
58: **Our Process:**
59: 1. Emergency board-up and securing
60: 2. Smoke and soot removal
61: 3. Odor elimination
62: 4. Structural repairs
63: 5. Content restoration
64: 
65: **Response Time:** Same day for emergencies
66: 
67: ---
68: 
69: ### Mold Remediation
70: <!-- @service: mold_remediation, emergency: false -->
71: 
72: Safe, thorough mold removal and remediation services.
73: 
74: **Our Process:**
75: 1. Inspection and assessment
76: 2. Containment of affected areas
77: 3. Air filtration
78: 4. Mold removal
79: 5. Cleaning and sanitizing
80: 6. Restoration
81: 
82: **Note:** Mold can grow within 24-48 hours of water damage.
83: 
84: ---
85: 
86: ### [Additional Service]
87: <!-- @service: service_code, emergency: true/false -->
88: 
89: [Service description]
90: 
91: ---
92: 
93: ## Services We Don't Provide
94: 
95: ### General Plumbing
96: <!-- @exclusion: general_plumbing, exception: "unless it caused water damage to the property" -->
97: 
98: We specialize in water damage restoration, not routine plumbing repairs like fixing toilets or unclogging drains.
99: 
100: **Recommendation:** Contact a licensed plumber for routine plumbing issues.
101: 
102: ---
103: 
104: ### General Construction
105: <!-- @exclusion: general_construction -->
106: 
107: We focus on restoration after damage, not new construction or renovation projects.
108: 
109: **Recommendation:** Contact a general contractor for new construction needs.
110: 
111: ---
112: 
113: ### [Additional Exclusion]
114: <!-- @exclusion: code -->
115: 
116: [Explanation]
117: 
118: **Recommendation:** [Referral suggestion]
119: 
120: ---
121: 
122: ## Service Areas
123: 
124: ### Areas We Serve
125: <!-- @coverage: include -->
126: 
127: **[County/Region 1]:**
128: [City 1], [City 2], [City 3], [City 4], [City 5]
129: 
130: **[County/Region 2]:**
131: [City 1], [City 2], [City 3], [City 4], [City 5]
132: 
133: ---
134: 
135: ### Areas Outside Our Coverage
136: <!-- @coverage: exclude -->
137: 
138: We do not currently service the following areas:
139: - [Area 1]
140: - [Area 2]
141: - [Area 3]
142: 
143: For these areas, we recommend contacting local restoration services.
144: 
145: ---
146: 
147: ## Frequently Asked Questions
148: 
149: ### How quickly can you respond to an emergency?
150: <!-- @faq: response_time -->
151: 
152: For emergencies in our service area, we have a **30-minute response time**. Our technicians are available 24/7/365.
153: 
154: ---
155: 
156: ### Do you work with insurance companies?
157: <!-- @faq: insurance -->
158: 
159: Yes, we work with **all insurance carriers**. We handle the paperwork and communicate directly with your insurance company to make the process as smooth as possible. You won't need to pay out of pocket immediately.
160: 
161: ---
162: 
163: ### What should I do while waiting for help?
164: <!-- @faq: while_waiting -->
165: 
166: Your safety is the priority. Please follow the relevant safety instructions below based on your situation, then call our emergency line for immediate assistance.
167: 
168: ---
169: 
170: ### How much does [service] cost?
171: <!-- @faq: pricing -->
172: 
173: Every situation is unique, so we provide **free estimates** after a technician inspects the damage. We need to assess the extent of the damage to give you an accurate quote.
174: 
175: ---
176: 
177: ### Do I need to be home for the inspection?
178: <!-- @faq: inspection -->
179: 
180: Yes, we recommend that you or an authorized adult be present during the initial inspection so we can discuss findings and next steps with you directly.
181: 
182: ---
183: 
184: ## Emergency Safety Procedures
185: 
186: ### Water Emergency
187: <!-- @safety: water_emergency, triggers: emergency_water, emergency_flood, emergency_sewage -->
188: 
189: **If you have active water damage:**
190: 
191: 1. **Turn off the main water valve** if you can safely access it
192: 2. **Turn off electricity** to affected areas (if safe to reach the breaker)
193: 3. **Stay away from standing water** near electrical outlets or appliances
194: 4. **Do NOT use electrical appliances** near water
195: 5. **Move to a dry area** and call our emergency line: **(555) 123-4567**
196: 
197: **Do NOT:**
198: - Walk through standing water if you can't see the floor
199: - Use a regular vacuum to remove water
200: - Enter rooms with sagging ceilings
201: 
202: ---
203: 
204: ### Fire/Smoke Emergency
205: <!-- @safety: fire_emergency, triggers: emergency_fire, emergency_smoke -->
206: 
207: **If you've had a fire:**
208: 
209: 1. **Ensure everyone has evacuated safely**
210: 2. **Do NOT re-enter** until the fire department clears the building
211: 3. **Open windows** for ventilation if safe to do so
212: 4. **Do NOT touch** soot-covered items (can cause further damage)
213: 5. **Document damage** with photos from a safe distance
214: 6. **Call our emergency line:** **(555) 123-4567**
215: 
216: **Important:** The longer soot sits, the harder it is to clean. Call us promptly.
217: 
218: ---
219: 
220: ### Gas Smell Emergency
221: <!-- @safety: gas_emergency, triggers: emergency_gas -->
222: 
223: **If you smell gas:**
224: 
225: 1. **Do NOT turn on any lights or electrical switches**
226: 2. **Do NOT use your phone inside** - sparks can ignite gas
227: 3. **Leave the building immediately**
228: 4. **Call your gas company** from outside or a neighbor's phone
229: 5. **Do NOT re-enter** until the gas company clears it
230: 
231: **Call 911 if you feel dizzy or unwell.**
232: 
233: ---
234: 
235: ## Additional Information
236: 
237: ### Insurance Claims Process
238: 
239: 1. **Document everything** - Take photos and videos of damage
240: 2. **Contact your insurance** - Report the claim
241: 3. **Call us** - We'll coordinate with your insurance
242: 4. **Get your estimate** - We provide detailed documentation
243: 5. **Approve the work** - We begin restoration
244: 6. **Final walkthrough** - Ensure your satisfaction
245: 
246: We handle insurance paperwork so you don't have to.
247: 
248: ---
249: 
250: ### Our Guarantee
251: 
252: - **100% Satisfaction Guaranteed**
253: - Licensed and insured
254: - [Certification] certified technicians
255: - Same-day emergency response
256: - Free estimates
257: 
258: ---
259: 
260: *[Company Name] - [Tagline]*
261: *[Certifications] | [Availability]*

(End of file - total 261 lines)
</content>

<system-reminder>
Instructions from: /home/chaschel/Documents/go/bchat/docs/CLAUDE.md
# Project Context for Claude Code

## Project Overview
This is a Memos-based application with an AI Chat Agent system for multi-tenant support.

## Current Implementation Status
See `docs/DOCS_CHAT_DESIGN_4_IMP_2-PROGRESS.MD` for detailed progress report.

## Key Documentation
- `docs/DOCS_README.MD` - **Comprehensive project documentation** (start here)
- `docs/CHANGELOG.MD` - Project changelog with dated entries
- `docs/DOCS_ENV_VAR.MD` - Environment variables reference
- `docs/DOCS_TASKFILE.MD` - Build and run commands reference
- `docs/DOCS_AGENT_ARCHITECTURE.MD` - Configuration-driven architecture guide
- `docs/DOCS_WIDGET.MD` - External chat widget integration guide
- `docs/DOCS_PENDING.MD` - Pending work, limitations, and known issues

## Design Documents
- `docs/DOCS_CHAT_DESIGN_4_IMP_2.MD` - Main implementation specification
- `docs/DOCS_CHAT_DESIGN_4_IMP_2-PROGRESS.MD` - Implementation progress and pending items
- `docs/DOCS_SIMULATION.MD` - Agent simulation feature specification
- `docs/DOCS_RAG_PIPELINE.MD` - RAG pipeline architecture and configuration
- `docs/DOCS_LANCEDB.MD` - LanceDB RAG implementation plan

## Hybrid Search & Evaluation Documents
- `docs/DOCS_HYBRID_SEARCH.MD` - Hybrid search (vector + BM25) implementation plan
- `docs/DOCS_HYBRID_SEARCH_EXPLAINED.MD` - Hybrid search technical deep-dive (BM25, cosine similarity, score fusion)
- `docs/DOCS_EVALUATION.MD` - Evaluation framework (datasets, quality metrics, benchmarking)
- `docs/DOCS_HALLUCINATION_GROUNDING.MD` - Hallucination detection and grounding validation
- `docs/DOCS_ROLLOUT.MD` - Production rollout guide (staged deployment, monitoring, rollback)

## Architecture
- **Backend:** Go with Echo framework, SQLite database
- **Frontend:** React with MobX, Vite build
- **LLM:** OpenRouter API via `go-openrouter` library

### CRITICAL: Chat Agent Design Principle

**The chat agent must be GENERAL PURPOSE, not tenant-specific.**

The agent's behavior is driven entirely by:
1. **KB.MD** - Knowledge base content (services, FAQs, coverage areas)
2. **POLICY.MD** - Rules, identity, tone, intents
3. **SCRIPT.MD** - Conversation flow structure

**DO NOT:**
- Hardcode tenant-specific logic in the codebase
- Add conditional behavior based on tenant ID or slug
- Create tenant-specific prompts or handlers
- Build features that only work for certain verticals

**DO:**
- Design features that work for ANY tenant configuration
- Let the KB/Policy/Script files define all tenant-specific behavior
- Keep the agent code generic and configuration-driven
- Test features with multiple tenant types (restoration, insurance, etc.)

The goal: A single, generic agent that becomes specialized through its configuration files, not through code changes.

---

## SDLC Processes

### Build Commands

```bash
# Full build (backend + frontend)
task build

# Backend only
task build:backend
# Or directly:
go build -o build/memos ./bin/memos/main.go

# Frontend only
task build:frontend
# Or directly:
cd web && npm run build

# Development server (hot reload)
task dev
```

### Database Migrations

Migrations are auto-applied on server startup. Location: `store/migration/sqlite/`

**Creating a new migration:**
1. Find the latest version folder (e.g., `0.25/`)
2. Create a new numbered SQL file: `09__descriptive_name.sql`
3. Use `IF NOT EXISTS` for idempotent migrations:

```sql
CREATE TABLE IF NOT EXISTS my_table (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    -- columns...
    FOREIGN KEY (tenant_id) REFERENCES agent_tenant(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_my_table_tenant ON my_table(tenant_id);
```

**Migration naming convention:** `NN__snake_case_description.sql`

---

## Adding New Features

### Backend Pattern (Go)

**1. Define types in `store/agent.go`:**
```go
type MyNewType struct {
    ID       int32
    TenantID int32
    // fields...
}

type FindMyNewType struct {
    ID       *string
    TenantID *int32
}
```

**2. Add interface methods to `store/driver.go`:**
```go
// In the Driver interface
CreateMyNewType(ctx context.Context, item *MyNewType) (*MyNewType, error)
GetMyNewType(ctx context.Context, find *FindMyNewType) (*MyNewType, error)
ListMyNewTypes(ctx context.Context, find *FindMyNewType) ([]*MyNewType, error)
DeleteMyNewType(ctx context.Context, id string) error
```

**3. Add Store delegate methods in `store/agent.go`:**
```go
func (s *Store) CreateMyNewType(ctx context.Context, item *MyNewType) (*MyNewType, error) {
    return s.driver.CreateMyNewType(ctx, item)
}
```

**4. Implement in `store/db/sqlite/agent.go`:**
```go
func (d *DB) CreateMyNewType(ctx context.Context, item *store.MyNewType) (*store.MyNewType, error) {
    // SQL implementation
}
```

**5. Add stub implementations for MySQL/PostgreSQL:**
- `store/db/mysql/agent.go`
- `store/db/postgres/agent.go`

```go
func (d *DB) CreateMyNewType(ctx context.Context, item *store.MyNewType) (*store.MyNewType, error) {
    return nil, errNotImplemented
}
```

**6. Add handlers in `server/router/api/v1/agent/handlers.go`:**
```go
func (h *Handler) HandleCreateMyNewType(c echo.Context) error {
    // Extract tenant, validate permissions, call service
}
```

**7. Register routes in `server/router/api/v1/v1.go`:**
```go
authGroup.POST("/:slug/my-new-type", s.agentHandler.HandleCreateMyNewType)
authGroup.GET("/:slug/my-new-type", s.agentHandler.HandleListMyNewType)
```

### Frontend Pattern (React + MobX)

**1. Add types and state to store (e.g., `web/src/store/v2/agentAdmin.ts`):**
```typescript
export interface MyNewType {
  id: string;
  tenantId: number;
  // fields...
}

class LocalState {
  myNewItems: MyNewType[] = [];
  isLoadingMyNewItems: boolean = false;
  // ...
}
```

**2. Add methods to store:**
```typescript
const fetchMyNewItems = async (slug: string) => {
  state.setPartial({ isLoadingMyNewItems: true });
  try {
    const response = await axios.get<{ items: MyNewType[] }>(
      `/api/v1/agent/${slug}/my-new-type`
    );
    runInAction(() => {
      state.myNewItems = response.data.items;
      state.isLoadingMyNewItems = false;
    });
  } catch (error: any) {
    runInAction(() => {
      state.isLoadingMyNewItems = false;
      state.error = error.response?.data?.message || "Failed to fetch";
    });
  }
};

// Export in return object
return {
  state,
  fetchMyNewItems,
  // ...
};
```

**3. Add UI component in page (e.g., `web/src/pages/AgentAdmin.tsx`):**
```tsx
const MyNewSection = ({ tenantSlug, items, isLoading }) => {
  // Component implementation
};
```

**4. Add translations to `web/src/locales/en.json`:**
```json
{
  "agent-admin": {
    "my-new-title": "My New Feature",
    "my-new-description": "Description here"
  }
}
```

---

## Key Directories

| Directory | Purpose |
|-----------|---------|
| `server/router/api/v1/agent/` | Agent API handlers, services, parsers |
| `store/` | Data layer interfaces and types |
| `store/db/sqlite/` | SQLite implementations |
| `store/migration/sqlite/` | Database migrations |
| `web/src/pages/` | React page components |
| `web/src/store/v2/` | MobX stores |
| `web/src/locales/` | i18n translation files |
| `docs/` | Design documents and specifications |

## Key Files

| File | Purpose |
|------|---------|
| `server/router/api/v1/agent/handlers.go` | HTTP request handlers |
| `server/router/api/v1/agent/service.go` | Business logic, LLM integration |
| `server/router/api/v1/agent/parser.go` | KB.MD/POLICY.MD/SCRIPT.MD parsing |
| `server/router/api/v1/agent/simulation.go` | Simulation orchestration |
| `server/router/api/v1/agent/analysis.go` | Transcript benchmark analysis |
| `server/router/api/v1/v1.go` | Route registration |
| `store/agent.go` | Agent data types and store interface |
| `store/driver.go` | Database driver interface |
| `store/db/sqlite/agent.go` | SQLite CRUD implementations |

---

## Permission System (RBAC)

Permissions are stored in `user_tenant_permissions` table.

| Permission | Description |
|------------|-------------|
| `tenant:admin` | Full tenant management |
| `tenant:read` | View tenant configuration |
| `api:config` | Configure LLM settings |
| `chat:test` | Run simulations, view simulation history |
| `chat:logs` | View real chat session logs |
| `files:upload` | Upload KB/Policy/Script files |

**Checking permissions in handlers:**
```go
hasPermission, _ := h.service.CheckUserPermission(ctx, userID, tenantID, "chat:test")
if !hasPermission {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied")
}
```

---

## Environment Variables

See `docs/DOCS_ENV_VAR.MD` for complete documentation including configuration priority.

```bash
# Required
OPENROUTER_API_KEY=<your-key>

# LLM (optional)
LLM_MODEL=openai/gpt-4o-mini

# RAG Pipeline (optional)
RAG_PIPELINE_ENABLED=true|false
EMBEDDING_PROVIDER=openrouter|mock|local  # openrouter is default
EMBEDDING_MODEL=openai/text-embedding-3-small
LANCEDB_STORAGE_PROVIDER=local|s3
```

**Configuration Priority:** Tenant Config (Agent Admin) > Environment Variable > Hardcoded Default

---

## Common Development Workflows

### Adding a New API Endpoint

1. Define request/response types (if complex)
2. Add handler in `handlers.go`
3. Add business logic in `service.go` (if needed)
4. Register route in `v1.go`
5. Add frontend API call in store
6. Add UI component
7. Add translations
8. Rebuild and test

### Adding a New Database Table

1. Create migration file in `store/migration/sqlite/0.25/`
2. Add Go types in `store/agent.go`
3. Add interface methods in `store/driver.go`
4. Implement in `store/db/sqlite/agent.go`
5. Add stubs in `store/db/mysql/agent.go` and `store/db/postgres/agent.go`
6. Add Store delegate methods in `store/agent.go`
7. Rebuild - migrations auto-apply on startup

### Debugging LLM Issues

1. Check `service.go` for system prompt construction
2. Look at `buildSystemPrompt()` function
3. Add `slog.Debug()` calls to log prompts/responses
4. Check OpenRouter dashboard for API errors

### Running with RAG (Recommended)

```bash
# Production (requires OPENROUTER_API_KEY)
task run:rag

# Testing pipeline only (no semantic accuracy)
task run:rag:mock
```

---

## Best Practices & Tips

### Embedding Providers
- **Use `openrouter`** for development and production - requires OPENROUTER_API_KEY
- **Use `mock`** only for testing pipeline flow - NOT semantically accurate
- **Use `local`** if running a local embedding server

### Taskfile Environment Variables
- Use inline env vars in commands (e.g., `VAR=value ./binary`), NOT `env:` blocks
- Pattern after `run:rag` command style

### Database Tables
- Table names are **plural** (e.g., `agent_tenants`, not `agent_tenant`)
- Always check existing table names before writing migrations

### Frontend State
- Export new store methods in the return object
- Use `runInAction` for all async state updates
- Add translations to `web/src/locales/en.json`

### RAG Content Processing
- `Format for RAG` - Preview/configure chunking options (ephemeral)
- `Rebuild Index` - Actually indexes content to VectorDB (persistent)
- Processing options can be saved per-tenant via "Save as Default"

---

## Code Conventions

### Go
- Use `slog` for logging: `slog.Error("message", "error", err)`
- Return errors with context: `fmt.Errorf("failed to X: %w", err)`
- Use pointer receivers for methods
- JSON tags use `snake_case`

### TypeScript/React
- Use MobX `makeAutoObservable` for stores
- Use `runInAction` for async state updates
- Use `observer` HOC for reactive components
- Use Joy UI components from `@mui/joy`

### SQL
- Table names: `snake_case` (plural for collections)
- Always use `IF NOT EXISTS` in migrations
- Add indexes for foreign keys and common query fields
- Use `ON DELETE CASCADE` for tenant-scoped data

---

## Recent Work (2026-01-26)
1. **Format for RAG Phase 1** - Store processing options per tenant
   - "Save as Default" button in Agent Admin
   - Options loaded automatically when Format for RAG opens

### Previous Work (2026-01-23)
1. Added "Rebuild Index" button to Agent Admin for per-tenant RAG reindexing
2. Added `task run:rag` and `task run:rag:mock` Taskfile commands
3. Created DOCS_ENV_VAR.MD, DOCS_TASKFILE.MD, CHANGELOG.MD, DOCS_README.MD
4. Fixed data path handling in Taskfile.yml (absolute paths with {{.ROOT_DIR}})
5. Added MockEmbedding provider for testing without API keys
6. Added REINDEX_RAG startup flag for bulk re-indexing
7. Created DOCS_WIDGET.MD for external chat widget integration
8. Completed RAG Phases 3-5 (retrieval pipeline, prompt simplification, cleanup)
9. **Phase 5B Complete:** Removed compliance.go (489 lines), refactored audit.go to use LLM-based semantic verification via verifier.go

### Previous Work (2026-01-20)
1. Added Agent Simulation feature with SSE streaming
2. Added conversation history dropdown (simulations + real chats)
3. Added SCRIPT.MD support for conversation flow guides
4. Added transcript benchmark analysis feature
5. Added plain text response formatting (no markdown)
6. Removed Chat Session Logs from Agent Admin (redundant)

---

## LanceDB RAG Implementation

**Design Document:** `docs/DOCS_LANCEDB.MD`

### Overview
RAG (Retrieval-Augmented Generation) pipeline using LanceDB-Go to improve response quality, reduce hallucinations, and decrease system prompt size through intelligent document retrieval.

### Implementation Status

| Phase | Description | Status |
|-------|-------------|--------|
| Phase 1 | Foundation - LanceDB dependencies, vectordb.go, schema, chunker, embedding service | ✅ Complete |
| Phase 2 | Indexing Pipeline - Index on KB/Policy upload, batch embedding | ✅ Complete |
| Phase 3 | Retrieval Pipeline - Hybrid search, intent-aware retrieval | ✅ Complete |
| Phase 4 | Prompt Simplification - New template, refactor buildSystemPrompt() | ✅ Complete |
| Phase 5 | Cleanup & Optimization - Remove grounding.go, optional verifier | ✅ Complete |
| Phase 5B | Remove compliance.go, refactor audit.go to use LLM verification | ✅ Complete |

### Phase 1 Files Created
- `server/router/api/v1/agent/vectordb.go` - LanceDB connection and operations
- `server/router/api/v1/agent/embedding.go` - Embedding service interface (local + OpenRouter)
- `server/router/api/v1/agent/chunker.go` - Document chunking logic

### Phase 2 Changes
- `server/router/api/v1/agent/vectordb.go` - Added full `LanceVectorDB` implementation with Insert, Delete, Search, Close, Stats
- `server/router/api/v1/agent/service.go` - Added VectorDB and Chunker to Service struct, initialized in NewService; added `ReindexTenantContent()` and `ReindexAllContent()`
- `server/router/api/v1/agent/handlers.go` - Added `indexContentForRAG()` function, called after KB/Policy import; added `HandleReindexTenant` endpoint
- `Taskfile.yml` - Fixed Linux build to use shared library (.so) instead of static (.a) due to BSD/GNU ar incompatibility

### RAG Admin Features
- **Rebuild Index Button** - Agent Admin UI button to trigger per-tenant reindexing
- **REINDEX_RAG Startup Flag** - Set `REINDEX_RAG=true` to re-index all tenants on server start
- **Mock Embeddings** - `EMBEDDING_PROVIDER=mock` for testing without API keys

### Key Environment Variables (RAG)
```bash
# Feature flag
RAG_PIPELINE_ENABLED=true|false  # Default: false

# Storage
LANCEDB_STORAGE_PROVIDER=local|s3  # Default: local
LANCEDB_LOCAL_PATH=build/data/lancedb  # For local storage

# Embedding
EMBEDDING_PROVIDER=openrouter|mock|local  # Default: openrouter
EMBEDDING_MODEL=openai/text-embedding-3-small  # For openrouter

# Optional verifier
LLM_VERIFIER_ENABLED=true|false  # Default: false
```

### Design Decisions Confirmed
- **Embedding Model:** OpenRouter API (default)
- **Index Storage:** Local filesystem (testing) + Tigrisdata S3 (production on fly.io)
- **Re-indexing:** On every file upload
- **Feature Flag:** Global environment variable
- **Compliance Checker:** REMOVED (Phase 5B) - audit.go now uses LLM-based verification
- **LLM Verifier:** Keep as OPTIONAL safety net (disabled by default)
- **SCRIPT.MD:** Full content as system prompt, configurable per tenant

---

## Pending Work
- Comprehensive test suite
- API documentation (OpenAPI/Swagger)
- Batch simulation runs
- Analysis history dashboard

---

## IMPORTANT: MD File Versioning Protocol

When updating KB.MD, POLICY.MD, or SCRIPT.MD files for a tenant:

### Step 1: Get Latest Version from Database

```bash
# Get latest KB content
sqlite3 build/data/memos_dev.db "SELECT content FROM agent_source_files WHERE tenant_id = <TENANT_ID> AND file_type = 'kb' ORDER BY id DESC LIMIT 1;"

# Get latest POLICY content
sqlite3 build/data/memos_dev.db "SELECT content FROM agent_source_files WHERE tenant_id = <TENANT_ID> AND file_type = 'policy' ORDER BY id DESC LIMIT 1;"

# Get latest SCRIPT content
sqlite3 build/data/memos_dev.db "SELECT content FROM agent_tenant_scripts WHERE tenant_id = <TENANT_ID> ORDER BY id DESC LIMIT 1;"
```

### Step 2: Save with UNIX Epoch Suffix

Save the database content to versioned files for review:

```bash
# Get current epoch
EPOCH=$(date +%s)

# Save files with epoch suffix
# KB_<epoch>.MD, POLICY_<epoch>.MD, SCRIPT_<epoch>.MD
```

Example: `KB_1768964222.MD`, `POLICY_1768964222.MD`

### Step 3: Make Surgical Updates

- **DO NOT** rewrite entire files
- Make targeted, surgical changes only
- Keep changes minimal and focused on the specific issue
- Preserve existing structure and formatting

### Step 4: Create Updated Draft

Save updated content to `*_DRAFT.MD` files for review before uploading to database.

**File locations:** `docs/templates/examples/<tenant>/`

### Why This Matters

- Database is source of truth for live agent behavior
- Local draft files may be stale
- UNIX epoch suffix enables version comparison
- Surgical updates reduce risk of regression

</system-reminder>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/docs/templates/POLICY.MD"
}
```

**Output:**
```
<path>/home/chaschel/Documents/go/bchat/docs/templates/POLICY.MD</path>
<type>file</type>
<content>
1: # [Company Name] Agent Policy
2: 
3: > **Version:** 1.0
4: > **Last Updated:** [Date]
5: 
6: ---
7: 
8: ## Identity
9: <!-- @identity -->
10: 
11: - **Role:** Customer Service Representative for [Company Name]
12: - **Tone:** Professional, empathetic, and helpful. Calm and reassuring during emergencies.
13: - **Brand Voice:** "We're here to help you through this."
14: 
15: **Guidelines:**
16: - Be concise and clear
17: - Show empathy for stressful situations
18: - Prioritize safety in emergencies
19: - Never make promises about pricing or timelines without qualification
20: 
21: ---
22: 
23: ## Intents
24: 
25: ### Emergency Intents
26: 
27: #### Water Emergency
28: <!-- @intent: emergency_water, category: emergency, urgency: 5, action: emergency_flow -->
29: 
30: **Description:**
31: Active water damage requiring immediate response. Water is currently flowing, spreading, or causing ongoing damage to the property.
32: 
33: **Key Indicators:**
34: - Water described as active, flowing, spreading, or pouring
35: - Words like "right now", "currently", "won't stop"
36: - Mentions of flooding, burst pipes, or sewage backup
37: - Urgency or panic in the message
38: 
39: **Examples that MATCH:**
40: - "My basement is flooding right now"
41: - "A pipe burst and water is everywhere"
42: - "Water is pouring through my ceiling"
43: - "Sewage is backing up into my bathroom"
44: - "Help! Water won't stop coming in"
45: 
46: **Examples that DO NOT match:**
47: - "I had water damage last week"
48: - "There's a water stain on my ceiling"
49: - "My basement floods when it rains" (not currently happening)
50: - "I need water damage restoration" (past event, not emergency)
51: 
52: ---
53: 
54: #### Fire/Smoke Emergency
55: <!-- @intent: emergency_fire, category: emergency, urgency: 5, action: emergency_flow -->
56: 
57: **Description:**
58: Recent fire or smoke damage. The fire is out but immediate restoration assessment and action is needed.
59: 
60: **Key Indicators:**
61: - Recent fire event (today, last night, just happened)
62: - Fire department was called or just left
63: - Smoke or soot throughout the property
64: - Concern about further damage
65: 
66: **Examples that MATCH:**
67: - "We just had a house fire"
68: - "The fire department just left"
69: - "There's smoke damage everywhere"
70: - "My kitchen caught fire this morning"
71: - "Soot is all over our home"
72: 
73: **Examples that DO NOT match:**
74: - "I smell something burning" (call 911, active fire)
75: - "We had a fire last year"
76: - "There's an old smoke smell"
77: 
78: ---
79: 
80: #### Mold Emergency
81: <!-- @intent: emergency_mold, category: emergency, urgency: 4, action: emergency_flow -->
82: 
83: **Description:**
84: Significant mold discovery causing health concerns or requiring urgent assessment.
85: 
86: **Examples that MATCH:**
87: - "I found black mold all over my walls"
88: - "There's mold everywhere and my family is getting sick"
89: - "Mold is spreading rapidly"
90: 
91: **Examples that DO NOT match:**
92: - "I think I might have some mold"
93: - "There's a small moldy spot in the bathroom"
94: 
95: ---
96: 
97: ### Standard Intents
98: 
99: #### Service Inquiry
100: <!-- @intent: service_inquiry, category: standard, urgency: 1, action: standard_flow -->
101: 
102: **Description:**
103: General questions about services, capabilities, or process. Not an emergency.
104: 
105: **Examples that MATCH:**
106: - "What services do you offer?"
107: - "Do you handle mold removal?"
108: - "Can you help with smoke damage?"
109: - "What's your process for water damage?"
110: 
111: ---
112: 
113: #### Booking Request
114: <!-- @intent: booking_request, category: standard, urgency: 2, action: booking_flow -->
115: 
116: **Description:**
117: Customer wants to schedule a service, inspection, or estimate.
118: 
119: **Examples that MATCH:**
120: - "I'd like to schedule an inspection"
121: - "Can someone come out tomorrow?"
122: - "I need to book a cleanup"
123: - "When is your next available appointment?"
124: 
125: ---
126: 
127: #### Pricing Question
128: <!-- @intent: pricing_question, category: standard, urgency: 1, action: standard_flow -->
129: 
130: **Description:**
131: Questions about cost, estimates, or payment.
132: 
133: **Examples that MATCH:**
134: - "How much does water damage restoration cost?"
135: - "Do you offer free estimates?"
136: - "What are your rates?"
137: - "Do you take insurance?"
138: 
139: ---
140: 
141: #### Coverage Question
142: <!-- @intent: coverage_question, category: standard, urgency: 1, action: standard_flow -->
143: 
144: **Description:**
145: Questions about service area or whether we serve their location.
146: 
147: **Examples that MATCH:**
148: - "Do you service [city name]?"
149: - "Are you available in my area?"
150: - "How far do you travel?"
151: 
152: ---
153: 
154: #### Hours/Availability
155: <!-- @intent: hours_availability, category: standard, urgency: 1, action: standard_flow -->
156: 
157: **Description:**
158: Questions about business hours or availability.
159: 
160: **Examples that MATCH:**
161: - "What are your hours?"
162: - "Are you open on weekends?"
163: - "Do you offer 24/7 service?"
164: 
165: ---
166: 
167: ### Meta Intents
168: 
169: #### Escalation
170: <!-- @intent: escalation, category: meta, urgency: 0, action: escalation_flow, confidence_threshold: 0.85 -->
171: 
172: **Description:**
173: Customer wants to escalate a complaint AGAINST the company. They are dissatisfied with service and want to speak to management or take formal action.
174: 
175: **Important:** This is about complaints against US, not about their property situation.
176: 
177: **Examples that MATCH:**
178: - "I want to speak with your manager"
179: - "This is unacceptable, get me a supervisor"
180: - "I'm filing a complaint with the BBB"
181: - "I'm going to contact my lawyer about this"
182: - "Your service has been terrible"
183: 
184: **Examples that DO NOT match:**
185: - "My tenant is complaining about the leak" (third party)
186: - "The situation is getting worse" (describing damage)
187: - "I need to speak with someone who can help today" (urgency, not complaint)
188: - "Can I talk to a technician?" (not escalation)
189: 
190: ---
191: 
192: #### Greeting
193: <!-- @intent: greeting, category: meta, urgency: 0, action: greeting_flow -->
194: 
195: **Description:**
196: Simple greeting or conversation starter.
197: 
198: **Examples that MATCH:**
199: - "Hello"
200: - "Hi there"
201: - "Good morning"
202: - "Hey"
203: 
204: ---
205: 
206: #### Farewell
207: <!-- @intent: farewell, category: meta, urgency: 0, action: farewell_flow -->
208: 
209: **Description:**
210: Customer is ending the conversation.
211: 
212: **Examples that MATCH:**
213: - "Thanks, goodbye"
214: - "That's all I needed"
215: - "Have a good day"
216: - "Bye"
217: 
218: ---
219: 
220: #### Thank You
221: <!-- @intent: thanks, category: meta, urgency: 0, action: thanks_flow -->
222: 
223: **Description:**
224: Customer expressing gratitude.
225: 
226: **Examples that MATCH:**
227: - "Thank you so much"
228: - "Thanks for your help"
229: - "I appreciate it"
230: 
231: ---
232: 
233: ## Rules
234: 
235: ### Safety First
236: <!-- @rule: safety_first, priority: 1 -->
237: 
238: In ANY emergency situation, provide relevant safety instructions BEFORE collecting contact information or discussing services. Never skip safety advice to get to booking faster.
239: 
240: ---
241: 
242: ### No Price Guessing
243: <!-- @rule: no_price_guessing, priority: 2 -->
244: 
245: Never estimate costs, give price ranges, or speculate about pricing. Always say: "Every situation is unique. We provide free estimates after a technician assesses the damage."
246: 
247: ---
248: 
249: ### No Repetition
250: <!-- @rule: no_repetition, priority: 3 -->
251: 
252: If you've already provided the emergency phone number in this conversation, don't repeat the full emergency script. Acknowledge their new question, answer it, then briefly remind them to call if urgent.
253: 
254: ---
255: 
256: ### Remember Customer Info
257: <!-- @rule: remember_info, priority: 4 -->
258: 
259: If the customer has already provided their name, phone number, or location, do NOT ask for it again. Reference what you already know.
260: 
261: ---
262: 
263: ### Stay In Scope
264: <!-- @rule: stay_in_scope, priority: 5 -->
265: 
266: Only discuss services that are explicitly defined in the Knowledge Base. If asked about a service we don't provide, politely explain and offer a referral if possible.
267: 
268: ---
269: 
270: ### Empathy First
271: <!-- @rule: empathy_first, priority: 6 -->
272: 
273: When a customer describes a stressful situation (damage to their home, health concerns), acknowledge their situation with empathy before moving to solutions. Example: "I understand how stressful this must be."
274: 
275: ---
276: 
277: ## Thresholds
278: <!-- @thresholds -->
279: 
280: | Metric | Value | Description |
281: |--------|-------|-------------|
282: | Emergency Urgency | >= 4 | Route to emergency flow |
283: | Escalation Confidence | >= 0.85 | Confirm escalation intent |
284: | Service Match Confidence | >= 0.70 | Suggest matched service |
285: | Location Match Confidence | >= 0.75 | Confirm coverage area |
286: 
287: ---
288: 
289: ## Response Guidelines
290: 
291: ### Emergency Response Structure
292: 
293: 1. **Acknowledge** - Show you understand the urgency
294: 2. **Safety** - Provide immediate safety instructions
295: 3. **Action** - Direct to emergency phone number
296: 4. **Reassure** - Let them know help is available
297: 
298: **Example:**
299: > "I understand you're dealing with an emergency. Your safety comes first.
300: >
301: > [Safety instructions for their situation]
302: >
303: > Please call our emergency line now: **(555) 123-4567**
304: >
305: > We have technicians available 24/7 and can be there within 30 minutes."
306: 
307: ---
308: 
309: ### Standard Response Structure
310: 
311: 1. **Acknowledge** - Confirm what they're asking
312: 2. **Answer** - Provide helpful information
313: 3. **Next Step** - Offer to help further or schedule service
314: 
315: ---
316: 
317: ### Out of Area Response
318: 
319: > "Thank you for reaching out. Unfortunately, we currently only service [our service areas].
320: >
321: > For [their location], I recommend searching for local [service type] companies in your area.
322: >
323: > Is there anything else I can help you with?"
324: 
325: ---
326: 
327: ### Escalation Response
328: 
329: > "I'm sorry to hear you're not satisfied with your experience. I want to make sure your concerns are heard.
330: >
331: > Please call our customer service line at **(555) 123-4568** and ask to speak with a supervisor. They're available [hours] and will be able to help address your concerns directly.
332: >
333: > Is there anything I can help clarify in the meantime?"
334: 
335: ---
336: 
337: ## Verification Rules
338: <!-- @verification -->
339: 
340: The following rules are used by the AI verifier to check responses before they're sent to customers.
341: These rules are automatically enforced - no code changes needed.
342: 
343: ### Contact Grounding
344: <!-- @verification_rule: contact_grounding, type: exact_match, severity: critical -->
345: 
346: All contact information (phone numbers, email addresses) in responses must exactly match what's defined in KB.MD.
347: 
348: **Automatic checks:**
349: - Phone numbers must match exactly (no placeholder numbers like 555-xxx-xxxx)
350: - Email addresses must match exactly (no placeholder emails like name@email.com)
351: - No inventing or guessing contact information
352: 
353: **Fallback:** "Please call our main line for assistance."
354: 
355: ---
356: 
357: ### Service Grounding
358: <!-- @verification_rule: service_grounding, type: exact_match, severity: high -->
359: 
360: All services mentioned must exist in KB.MD under "Services" section.
361: 
362: **Automatic checks:**
363: - Service names must match KB exactly
364: - No inventing features or capabilities not in KB
365: - No offering services from the "Exclusions" section
366: 
367: **Fallback:** "We offer a range of services. Could you tell me more about your situation?"
368: 
369: ---
370: 
371: ### Claims Grounding
372: <!-- @verification_rule: claims_grounding, type: conditional, severity: medium -->
373: 
374: Specific claims about pricing, timing, or guarantees must be supported by KB.
375: 
376: **Automatic checks:**
377: - No specific prices unless in KB
378: - No specific response times unless in KB
379: - No guarantees or promises not supported by KB
380: 
381: **Fallback:** "Every situation is unique. We provide detailed information after assessment."
382: 
383: ---
384: 
385: ### Tone Compliance
386: <!-- @verification_rule: tone_compliance, type: style, severity: low -->
387: 
388: Responses must match the configured tone and identity.
389: 
390: **Automatic checks:**
391: - Professional and empathetic tone
392: - Company name used correctly
393: - No inappropriate speculation on factual matters
394: 
395: ---
396: 
397: *End of Policy Document*

(End of file - total 397 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/docs/templates/SCRIPT_OPUS13.MD"
}
```

**Output:**
```
<path>/home/chaschel/Documents/go/bchat/docs/templates/SCRIPT_OPUS13.MD</path>
<type>file</type>
<content>
1: ## PRIORITY 0: EMERGENCY + COVERAGE CHECK
2: 
3: This is a **unified check** performed on every incoming message BEFORE any other processing.
4: 
5: ### STEP 1: Score for Emergency (5-Factor Heuristic)
6: 
7: | Factor | Weight | High Score Indicators |
8: |--------|:------:|----------------------|
9: | Time Urgency | 25% | "right now", "currently", "happening" |
10: | Active Situation | 25% | "is flooding", "coming in", "spreading" |
11: | Safety Risk | 20% | Vulnerable people, health symptoms, structural |
12: | Damage Severity | 20% | Fire > sewage > flooding > leak |
13: | Distress Language | 10% | "help!", "what do I do?" |
14: 
15: **Scoring:**
16: - ≥ 60%: EMERGENCY
17: - 40-59%: URGENT (clarify if happening now)
18: - < 40%: STANDARD
19: 
20: ---
21: 
22: ### STEP 2: Check Coverage Area
23: 
24: **IN-AREA indicators:**
25: - Towns: Huntington, Smithtown, Babylon, Islip, Brookhaven, Riverhead, Bay Shore, Ronkonkoma, Commack, Hauppauge, Massapequa, Freeport, Hempstead, Garden City, Mineola
26: - Counties: Nassau, Suffolk
27: - "Long Island" (without Brooklyn/Queens context)
28: - ZIP: 110XX, 117XX, 118XX, 119XX
29: 
30: **OUT-OF-AREA indicators:**
31: - NYC: Bronx, Brooklyn, Manhattan, Queens, Staten Island
32: - Other: Westchester, NJ, Jersey, CT, Connecticut, Rockland, Orange County
33: 
34: **UNKNOWN:** No location mentioned
35: 
36: ---
37: 
38: ### STEP 3: Route Based on Matrix
39: 
40: | Emergency | Coverage | Action |
41: |:---------:|:--------:|--------|
42: | YES | IN | Full emergency response |
43: | YES | OUT | Safety tips → explain limitation → suggest local |
44: | YES | UNKNOWN | Safety tips → ask location |
45: | URGENT | IN | Express concern, expedite intake |
46: | URGENT | OUT | Clarify urgency, then explain limitation |
47: | URGENT | UNKNOWN | "Is this happening now? What's your location?" |
48: | NO | IN | Standard intake flow |
49: | NO | OUT | Polite decline |
50: | NO | UNKNOWN | Standard intake, verify in CONTACT section |
51: 
52: ---
53: 
54: ## EMERGENCY RESPONSES
55: 
56: ### Response A: EMERGENCY + IN-AREA
57: 
58: > Example: "My basement in Huntington is flooding right now!"
59: 
60: "🚨 This is an emergency. Since you're in [LOCATION], we can dispatch a team within 30 minutes.
61: 
62: 📞 Call now: (631) 358-5200 | 855-DRYITNOW
63: 
64: While you call:
65: • Turn off water source if safe
66: • Turn off electricity to affected areas
67: • Move away from standing water
68: 
69: Our team is standing by for you."
70: 
71: ---
72: 
73: ### Response B: EMERGENCY + OUT-OF-AREA
74: 
75: > Example: "My apartment in Brooklyn is flooding!"
76: 
77: "I can hear this is urgent. First, safety tips:
78: • Turn off water and electricity if safe
79: • Move away from standing water
80: • Call 911 if there's immediate danger
81: 
82: Unfortunately, we only service Nassau and Suffolk Counties on Long Island—we can't dispatch to [LOCATION].
83: 
84: For immediate help:
85: • Call 911 for emergencies
86: • Search 'IICRC-certified water damage [LOCATION]'
87: 
88: Stay safe!"
89: 
90: ---
91: 
92: ### Response C: EMERGENCY + LOCATION UNKNOWN
93: 
94: > Example: "Water is pouring through my ceiling right now!"
95: 
96: "This sounds like an emergency. Safety tips right now:
97: • Turn off water if you can safely reach the valve
98: • Turn off electricity to affected areas
99: • Move away from standing water
100: 
101: What's your location? If you're in Nassau or Suffolk County, Long Island, we can have a team there in 30 minutes.
102: 
103: 📞 If you know you're in our area, call now: (631) 358-5200"
104: 
105: ---
106: 
107: ### Response D: URGENT (40-59%) + ANY LOCATION
108: 
109: "This sounds concerning. Is this actively happening right now, or did it occur earlier?
110: 
111: Also, what's the property address? If you're in Nassau or Suffolk County, we may need to get a team out quickly."
112: 
113: ---
114: 
115: ## NON-EMERGENCY RESPONSES
116: 
117: ### Response E: NON-EMERGENCY + OUT-OF-AREA
118: 
119: "Thanks for reaching out! Unfortunately, we only service Nassau and Suffolk Counties on Long Island. For [LOCATION], I'd recommend searching for IICRC-certified restoration companies in your area."
120: 
121: ### Response F: NON-EMERGENCY + IN-AREA or UNKNOWN
122: 
123: → Proceed to BUSINESS INTRODUCTION
124: 
125: ---
126: 
127: ## BUSINESS INTRODUCTION
128: 
129: After Priority 0 passes (non-emergency, in-area or unknown):
130: 
131: > If customer states an issue:
132: "I'm sorry to hear about that. Here at Bulovas Restorations, we're IICRC certified and available 24/7—we can definitely help. Let me get some details."
133: 
134: > If greeting only:
135: "Hello! Welcome to Bulovas Restorations Inc., Long Island's premier restoration company. We're IICRC certified, available 24/7, with 30-minute emergency response. How can I help?"
136: 
137: ---
138: 
139: ## CUSTOMER TYPE IDENTIFICATION
140: 
141: Infer from context. Don't ask "worked with us before?" first.
142: 
143: - **New:** No prior booking → Address issue
144: - **Repeat:** Mentions "my project", tech names → Welcome back
145: - **Ambiguous:** "Existing project or new situation?"
146: 
147: ---
148: 
149: ## COMMERCIAL PROPERTY MANAGER
150: 
151: Multi-unit building, landlord, HOA:
152: - Recognize scope, mention commercial teams
153: - Expedite, batch questions, offer priority
154: 
155: ---
156: 
157: ## RESTORATION INTAKE
158: 
159: **Phase 1 Rules:**
160: - Address stated issue, collect name/address/service
161: - NO "anything else?" in Phase 1
162: - Batch questions, don't re-ask
163: 
164: **Questions:**
165: 1. Damage type (if not stated)
166: 2. "When, and is source contained?"
167: 3. Severity, rooms affected
168: 4. Safety concerns
169: 5. Insurance
170: 
171: → Go to CONTACT INFORMATION
172: 
173: ---
174: 
175: ## CARPET CLEANING
176: 
177: 1. Rooms and square footage
178: 2. Type: routine, deep, heavy-duty
179: 3. Concerns
180: 
181: "For accurate estimate, call 📞 (631) 358-5200.
182: • HydraMaster truck-mounted—98% manufacturers prefer
183: • Non-toxic, safe for pets/kids
184: • FREE estimates | 100% satisfaction"
185: 
186: ---
187: 
188: ## CONTACT INFORMATION
189: 
190: Skip if provided: Name, Phone, Address, County verification (Nassau/Suffolk?), Property type, Timing, On-site availability
191: 
192: ---
193: 
194: ## CONFIRMATION
195: 
196: "Got it, [NAME]. Confirming: [NAME] | [PHONE] | [ADDRESS] | [SERVICE] | [TIMING]. Correct?"
197: 
198: ---
199: 
200: ## CONCLUSION
201: 
202: **Phase 2 - "Anything else?" appropriate**
203: 
204: "✅ You're all set, [NAME]!
205: 1️⃣ Scheduling confirms
206: 2️⃣ Tech calls before arriving
207: 3️⃣ On-site assessment
208: 4️⃣ Estimate provided
209: 5️⃣ Work begins
210: 
211: 📞 (631) 358-5200 | 855-DRYITNOW
212: 
213: Is there anything else?"
214: 
215: ---
216: 
217: ## CLOSING
218: 
219: **No Escape Rule:** Answer follow-ups. Wait for explicit "no" or "I'm good."
220: 
221: ---
222: 
223: ## SPECIAL SCENARIOS
224: 
225: - **Issue in first message:** Acknowledge, don't ask "worked with us?"
226: - **Upset customer:** Deep empathy, offer manager escalation
227: - **Pricing:** "Call (631) 358-5200. Restoration = FREE assessment."
228: - **Insurance:** "You can choose your own company."
229: 
230: ---
231: 
232: ## PROHIBITED LANGUAGE
233: 
234: ❌ "Updated successfully" | "Please disregard"
235: ✅ "Got it" | "Perfect"
236: 
237: ---
238: 
239: ## QUICK REFERENCE
240: 
241: | Item | Details |
242: |------|---------|
243: | Phone | (631) 358-5200 |
244: | Toll-Free | 855-DRYITNOW |
245: | Email | info@bulovasrestorations.com |
246: | Hours | 24/7/365 |
247: | Emergency | 30 minutes |
248: | Area | Nassau & Suffolk ONLY |
249: 
250: ---
251: 
252: ## HEURISTIC CATEGORIES
253: 
254: | Category | Weight |
255: |----------|:------:|
256: | Business Identity | 10% |
257: | Emergency Handling | 15% |
258: | Service Area | 10% |
259: | Customer Experience | 15% |
260: | Context Awareness | 10% |
261: | Phase Discipline | 10% |
262: | Intake Completeness | 10% |
263: | Scheduling Efficiency | 10% |
264: | Pricing Compliance | 5% |
265: | Proper Conclusion | 5% |
266: 
267: ---
268: 
269: ## VERSION NOTES
270: 
271: **Version:** OPUS13  
272: **Changes from OPUS12:**
273: - Merged Priority 0a (Emergency) and 0b (Service Area) into unified PRIORITY 0
274: - Added decision matrix for all 9 emergency × coverage combinations
275: - Created 6 response templates (A-F) for each scenario
276: - Location check now happens DURING emergency detection, not after
277: - Added URGENT + location unknown handling
278: 
279: **Priority Flow:**
280: ```
281: PRIORITY 0: Emergency + Coverage Check
282:     ├── Emergency + In-Area → Response A
283:     ├── Emergency + Out-of-Area → Response B
284:     ├── Emergency + Unknown → Response C
285:     ├── Urgent + Any → Response D
286:     ├── Non-Emergency + Out → Response E
287:     └── Non-Emergency + In/Unknown → Continue to Intake
288: ```
289: 
290: ---

(End of file - total 290 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/docs/BIZ_QUICK_START_CHECKLIST.MD"
}
```

**Output:**
```
<path>/home/chaschel/Documents/go/bchat/docs/BIZ_QUICK_START_CHECKLIST.MD</path>
<type>file</type>
<content>
1: # Business Owner Quick Start Checklist
2: 
3: > **Printing this? Keep it handy during setup.**
4: 
5: ---
6: 
7: ## Phase 1: Technical Setup
8: 
9: - [ ] Clone bchat repository
10: - [ ] Install prerequisites (Go 1.21+, Node.js 18+, Task)
11: - [ ] Get OpenRouter API key
12: - [ ] Set up server (Fly.io, VPS, or local machine)
13: - [ ] Configure environment variables
14: - [ ] Build and run the application
15: - [ ] Verify admin panel loads at `/agent-admin`
16: 
17: ---
18: 
19: ## Phase 2: Tenant Creation
20: 
21: - [ ] Create tenant in admin panel
22: - [ ] Note the **slug** (URL-safe identifier)
23: - [ ] Set LLM model (recommended: `gpt-4o-mini`)
24: - [ ] Set temperature (recommended: `0.3–0.5` for service businesses)
25: - [ ] Configure allowed domains (if needed)
26: - [ ] Toggle "Is Active" to Yes
27: 
28: ---
29: 
30: ## Phase 3: Knowledge Base Setup
31: 
32: - [ ] Create KB.MD using template or from scratch
33: - [ ] Include ALL services offered (with `@service` annotations)
34: - [ ] Include ALL services NOT offered (with `@exclusion` annotations)
35: - [ ] List complete service areas with `@coverage: include/exclude`
36: - [ ] Add emergency protocol with after-hours contact
37: - [ ] Include pricing guidance (structure, not fixed prices)
38: - [ ] Add 6+ FAQs: hours, estimates, payment, insurance, scheduling, cancellation
39: - [ ] Add safety procedures for emergency services
40: - [ ] Remove ALL placeholder text (`[Your Company Name]`, etc.)
41: - [ ] Review KB.MD with business owner for accuracy
42: 
43: ---
44: 
45: ## Phase 4: Policy Setup
46: 
47: - [ ] Create POLICY.MD using template
48: - [ ] Define agent identity and tone
49: - [ ] Set escalation trigger keywords
50: - [ ] Configure urgency thresholds
51: - [ ] Define lead capture rules
52: - [ ] Set "Say Unknown + Capture" behavior
53: - [ ] Define after-hours handling
54: 
55: ---
56: 
57: ## Phase 5: Widget Deployment
58: 
59: - [ ] Get embed URL: `https://your-server.com/widget/[slug]/embed.js`
60: - [ ] Add JavaScript embed to website
61: - [ ] Test widget loads on page
62: - [ ] Verify widget styling matches brand
63: - [ ] Test on mobile device
64: 
65: ---
66: 
67: ## Phase 6: Testing (Critical!)
68: 
69: - [ ] Click "Rebuild Index" after uploading files
70: - [ ] Run 3+ simulations with different personas:
71:   - Emergency scenario
72:   - Quote request scenario
73:   - General inquiry scenario
74: - [ ] Test RAG debug with 10+ queries
75: - [ ] Verify lead capture extracts name/email/phone
76: - [ ] Test escalation flow ("I want to speak to a manager")
77: - [ ] Check "Say Unknown" behavior works
78: 
79: ---
80: 
81: ## Phase 7: Go Live
82: 
83: - [ ] Confirm business owner has admin access
84: - [ ] Show business owner how to view leads
85: - [ ] Set up lead notification (email/SMS bridge)
86: - [ ] Train on daily lead review process
87: - [ ] Schedule follow-up for agent optimization
88: 
89: ---
90: 
91: ## Quick Reference
92: 
93: **Admin URL:** `https://your-server.com/agent-admin`
94: 
95: **Embed Script:**
96: ```html
97: <script
98:   src="https://your-server.com/widget/[slug]/embed.js"
99:   data-position="bottom-right"
100:   data-color="#0d9488"
101: ></script>
102: ```
103: 
104: **Critical Buttons:**
105: - **Create Tenant** → Sets up new business
106: - **Upload Files** → KB.MD, POLICY.MD, SCRIPT.MD
107: - **Rebuild Index** → Must click after every file upload!
108: - **Simulations** → Test before going live
109: 
110: **Lead Statuses:**
111: - `new` → Fresh lead, not yet contacted
112: - `contacted` → Someone reached out
113: - `converted` → Became a customer
114: - `lost` → Not interested or went elsewhere

(End of file - total 114 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/docs/docs_biz_guide.md"
}
```

**Output:**
```
<path>/home/chaschel/Documents/go/bchat/docs/docs_biz_guide.md</path>
<type>file</type>
<content>
1: # DOCS_HOWTO_BIZ.md — Business Owner Guide to bchat
2: 
3: > **Audience:** Business owners, operators, and Pithom Labs deployers who want to set up a working AI chat agent on a website — from zero to receiving qualified leads.
4: >
5: > **Scope:** This document covers understanding the platform, writing knowledge-base and policy content, deploying the widget, capturing leads, and operating the admin surface. It does **not** replace developer documentation; see `docs/DOCS_README.MD` and `AGENTS.md` for technical details.
6: 
7: ---
8: 
9: ## Table of Contents
10: 
11: 1. [What Is bchat?](#1-what-is-bchat)
12: 2. [What It Does For Your Business](#2-what-it-does-for-your-business)
13: 3. [The Business Model](#3-the-business-model)
14: 4. [Architecture Overview (Business View)](#4-architecture-overview-business-view)
15: 5. [Getting Started as a Deployer](#5-getting-started-as-a-deployer)
16: 6. [Setting Up Your First Tenant](#6-setting-up-your-first-tenant)
17: 7. [How to Write a High-Impact KB.md](#7-how-to-write-a-high-impact-kbmd)
18: 8. [POLICY.md — Business Rules, Do's and Don'ts](#8-policymd--business-rules-dos-and-donts)
19: 9. [SCRIPT.md — Conversation Flow](#9-scriptmd--conversation-flow)
20: 10. [Deploying the Chat Widget](#10-deploying-the-chat-widget)
21: 11. [Lead Capture Workflow](#11-lead-capture-workflow)
22: 12. [The Admin Panel](#12-the-admin-panel)
23: 13. [RAG Knowledge Base Management](#13-rag-knowledge-base-management)
24: 14. [Testing Before Going Live](#14-testing-before-going-live)
25: 15. [Common Business Workflows](#15-common-business-workflows)
26: 16. [Policy & Legal Guardrails](#16-policy--legal-guardrails)
27: 17. [What NOT To Do (Anti-Patterns)](#17-what-not-to-do-anti-patterns)
28: 18. [Pricing Model](#18-pricing-model)
29: 19. [Troubleshooting](#19-troubleshooting)
30: 
31: ---
32: 
33: ## 1. What Is bchat?
34: 
35: **bchat** is a multi-tenant AI chat agent platform. In plain language: it is a single software system that can power an unlimited number of independent AI chat agents — one per business — without any code changes between businesses.
36: 
37: Each business (called a **tenant**) gets:
38: 
39: - Its own **AI chat agent** trained on that business's knowledge
40: - Its own **widget** (chat bubble + panel) that can be embedded on any website
41: - Its own **lead capture** pipeline that extracts contact info from conversations
42: - Its own **admin dashboard** to review leads, test the agent, and manage knowledge content
43: - Full **data isolation** — no business can see another business's data
44: 
45: The system is built on:
46: 
47: | Layer | Technology |
48: |-------|-----------|
49: | Backend | Go + Echo (fast, reliable API server) |
50: | Database | SQLite (dev/local) / PostgreSQL (production SaaS) |
51: | Frontend | React + TypeScript |
52: | AI / LLM | OpenRouter (access to GPT-4o, Claude, Gemini, etc.) |
53: | Vector Search | LanceDB (RAG — retrieval-augmented generation) |
54: | Embeddings | OpenAI `text-embedding-3-small` (or local / mock) |
55: 
56: ---
57: 
58: ## 2. What It Does For Your Business
59: 
60: As a business owner or agency, bchat solves these problems:
61: 
62: ### 24/7 Lead Capture
63: A visitor who lands on your client's website at 10 PM gets an immediate, intelligent response — not a voicemail. The agent answers questions, qualifies the prospect, and captures their contact details.
64: 
65: ### No More "I'll Get Back To You"
66: The agent handles first-contact qualification so the owner doesn't have to. Leads arrive pre-qualified with name, email/phone, service needed, and urgency level.
67: 
68: ### Knowledge-at-Scale
69: The agent knows everything about the business — services, pricing, coverage areas, exclusions, FAQs, emergency protocols — because it reads from the KB.MD/POLICY.MD files. Update the files, update the agent instantly. No retraining, no redeployment.
70: 
71: ### Multi-Channel Follow-Up (Roadmap)
72: Email digests, SMS notifications, magic links for offline follow-up — all routed through a durable outbox system so no lead slips through.
73: 
74: ### Professional Credibility
75: A branded, always-on chat widget signals that the business is modern and responsive. The agent's tone, voice, and rules are set by the business owner.
76: 
77: ---
78: 
79: ## 3. The Business Model
80: 
81: Pithom Labs' recommended model: **"AI Lead Desk for Local Service Businesses."**
82: 
83: ### Three Tiers
84: 
85: | Package | Target | Setup | Monthly | Includes |
86: |---------|--------|-------|---------|----------|
87: | **Chat Widget Only** | Business already has a website | $99–$299 | $149–$299 | Widget install, FAQ training, lead capture form, email/SMS alerts, human takeover, weekly report |
88: | **Hugo Site + AI Lead Desk** | No website / bad website / Facebook-only | $399–$999 | $199–$499 | Fast landing site, domain/DNS help, service pages, quote form, widget, notification workflow |
89: | **Managed Lead Desk** | Growing SMBs that want white-glove | $499–$1,500 setup-equivalent | $499–$1,500 | Everything above + CRM or Google Sheets pipeline, appointment routing, missed-lead follow-up, conversation review, monthly optimization, owner dashboard |
90: 
91: ### Positioning Statement
92: > "You keep doing the work. We handle the digital front desk."
93: 
94: Don't say "we sell AI chatbot software." Say "we make sure your website never ignores a prospect again."
95: 
96: ### First-Market Vertical
97: Start with one: pressure washing, junk removal, cleaning, landscaping, or pest control. These businesses:
98: - Have high-intent, urgent leads
99: - Are often in the field (can't answer phones)
100: - Have simple knowledge bases
101: -don't have heavy compliance overhead
102: 
103: ---
104: 
105: ## 4. Architecture Overview (Business View)
106: 
107: ```
108: ┌─────────────────────────────────────────────────────────────┐
109: │                    YOUR CLIENT'S WEBSITE                     │
110: │  ┌──────────────────────────────────────────────────────┐   │
111: │  │           bchat Embed Widget (chat bubble)             │   │
112: │  │  • Floating button (bottom-right or inline)            │   │
113: │  │  • Expands into full chat panel                        │   │
114: │  │  • Visitor sends a message → agent responds            │   │
115: │  │  • Agent collects name, email, phone, service needed   │   │
116: │  └──────────────────────────────────────────────────────┘   │
117: └───────────────────────┬─────────────────────────────────────┘
118:                         │ HTTPS (SSE streaming)
119:                         ▼
120: ┌─────────────────────────────────────────────────────────────┐
121: │                   BCHAT SERVER (Yours / Pithom's)            │
122: │                                                              │
123: │  ┌────────────┐  ┌──────────────┐  ┌────────────────────┐   │
124: │  │  Tenant    │  │  RAG Engine  │  │   Lead Capture     │   │
125: │  │  Registry  │  │  (LanceDB)   │  │   Pipeline         │   │
126: │  └────────────┘  └──────────────┘  └────────────────────┘   │
127: │                                                              │
128: │  Each tenant has:                                           │
129: │  • KB.MD     — knowledge base (facts, services, FAQs)       │
130: │  • POLICY.MD — agent rules, identity, tone, thresholds      │
131: │  • SCRIPT.MD — conversation stages & flow                   │
132: │                                                              │
133: │  OpenRouter (GPT-4o, Claude, Gemini, etc.)                  │
134: └─────────────────────────────────────────────────────────────┘
135:                         │
136:                         ▼
137: ┌─────────────────────────────────────────────────────────────┐
138: │                   YOUR ADMIN PANEL                           │
139: │  • Upload KB/Policy/Script files per tenant                  │
140: │  • Review leads (name, email, phone, topic, status)          │
141: │  • Export leads as CSV                                      │
142: │  • Test agent via simulation sandbox                        │
143: │  • View RAG search debug (what chunks were retrieved)        │
144: │  • Manage users & permissions                               │
145: └─────────────────────────────────────────────────────────────┘
146: ```
147: 
148: ---
149: 
150: ## 5. Getting Started as a Deployer
151: 
152: ### Prerequisites
153: 
154: - A server (Fly.io, VPS, or local machine)
155: - Go 1.21+ (for building backend)
156: - Node.js 18+ (for building frontend/widget)
157: - An OpenRouter account + API key (`sk-or-v1-...`)
158: - A domain name (for the admin panel and widget delivery)
159: 
160: ### Quick Start (Local Development)
161: 
162: ```bash
163: # Clone repo
164: git clone <your-bchat-repo>
165: cd bchat
166: 
167: # Install dependencies
168: task setup
169: 
170: # Set your OpenRouter API key
171: export OPENROUTER_API_KEY=sk-or-v1-your-key-here
172: 
173: # Build (without RAG, for quick start)
174: task build
175: 
176: # Run the server
177: ./build/memos --mode dev --data build/data
178: ```
179: 
180: The app starts at `http://localhost:8081`. Admin UI is at `/agent-admin`.
181: 
182: ### Quick Start (With RAG)
183: 
184: RAG = the agent searches your KB.MD for relevant info before answering. Strongly recommended for production.
185: 
186: ```bash
187: # Build with LanceDB native support
188: task build:rag
189: 
190: # Run with RAG enabled
191: task run:rag
192: 
193: # Or use mock embeddings (no API key needed for testing)
194: task run:rag:mock
195: ```
196: 
197: ### Quick Start (Production on Fly.io)
198: 
199: ```bash
200: # Set Fly.io tokens
201: export FLY_API_TOKEN=...
202: export FLY_ORG=...
203: 
204: # Deploy
205: fly launch
206: fly deploy
207: ```
208: 
209: See `docs/DOCS_DEPLOY_FLY.MD` for full production deployment steps.
210: 
211: ---
212: 
213: ## 6. Setting Up Your First Tenant
214: 
215: A **tenant** = one business. Each tenant is completely isolated.
216: 
217: ### Step 1: Create a Tenant
218: 
219: 1. Log in to `http://your-server:8081/agent-admin`
220: 2. Go to **Tenants** page
221: 3. Click **Create Tenant**
222: 4. Fill in:
223: 
224: | Field | Example | Notes |
225: |-------|---------|-------|
226: | **Slug** | `acme-pressure-washing` | URL-safe; used in widget embed URL |
227: | **Company Name** | `Acme Pressure Washing` | Displayed in chat header |
228: | **Vertical** | `pressure_washing` | For your reference (not used in code) |
229: | **LLM Model** | `openai/gpt-4o-mini` | Override per tenant; falls back to `LLM_MODEL` env var |
230: | **Temperature** | `0.3–0.7` | 0 = very factual; 1 = very creative |
231: | **Allowed Domains** | `acmepw.com, acmepw.net` | Restrict widget embedding; blank = all domains allowed |
232: | **Is Active** | Yes/No | Toggle to pause/activate the tenant |
233: 
234: 5. Click **Create** — you now have a tenant.
235: 
236: ### Step 2: Get Your Embed URL
237: 
238: The tenant slug is your tenant's unique identifier. Your embed URL is:
239: 
240: ```
241: https://your-server.com/widget/<slug>/embed.js
242: # or
243: https://your-server.com/api/v1/agent/<slug>/widget.js
244: ```
245: 
246: ### Step 3: Upload KB.MD, POLICY.MD, SCRIPT.MD
247: 
248: Go to the tenant's detail page in Agent Admin and upload the three markdown files (described in detail below).
249: 
250: **Critical:** Uploading files stores them but does NOT auto-rebuild the RAG index. After uploading, click **Rebuild Index** to make the content searchable.
251: 
252: ---
253: 
254: ## 7. How to Write a High-Impact KB.md
255: 
256: **KB.MD = the agent's brain.** It contains every fact about the business that the agent can reference when answering visitor questions. It is the single most important file you will write.
257: 
258: ### Philosophy
259: 
260: Write KB.MD for a **smart, friendly new hire** — someone who needs context to represent the business accurately. The agent reads the whole file (or chunks from it via RAG) before answering. Be comprehensive but organized.
261: 
262: ### Structure Template
263: 
264: ```markdown
265: <!-- @section: company_overview -->
266: ## About [Business Name]
267: 
268: - Founded: [Year]
269: - Owner: [Name]
270: - Headquarters: [City, State]
271: - Service Area: [Primary cities, counties, radius]
272: - Years in Business: [N]
273: - License/Insurance: [Details if relevant]
274: 
275: ---
276: 
277: <!-- @section: services -->
278: ## Services We Offer
279: 
280: ### <!-- @service: service_code_1, emergency: true/false, response_time: "60 minutes" -->
281: #### [Service Name]
282: [2–4 sentences describing what this service covers, when it's needed,
283: and what the customer can expect.]
284: 
285: **Typical causes:** [Bullet list of common triggers]
286: **What's included:** [Bullet list of deliverables]
287: **Response time:** [e.g., "60 minutes or less for emergencies"]
288: 
289: ---
290: 
291: ### <!-- @service: service_code_2, emergency: false -->
292: #### [Second Service]
293: ...
294: 
295: ---
296: 
297: <!-- @section: exclusions -->
298: ## Services We Don't Provide
299: 
300: <!-- @exclusion: code, exception: "secondary to X" -->
301: ### [Service Not Offered]
302: We don't provide standalone [service]. [One sentence explanation + referral if available].
303: 
304: ---
305: 
306: <!-- @section: coverage -->
307: ## Service Areas
308: 
309: <!-- @coverage: include -->
310: ### Areas We Serve
311: **Primary:** [City1], [City2], [City3]
312: **Secondary:** [City4], [City5] (additional travel fee may apply)
313: 
314: <!-- @coverage: exclude -->
315: ### Areas We Don't Serve
316: [City/Region] — [Reason, if relevant]
317: 
318: ---
319: 
320: <!-- @section: emergency_protocol -->
321: ## Emergency Protocol
322: 
323: <!-- @intent: emergency, urgency: immediate -->
324: ### How Emergencies Are Handled
325: 1. Caller/Victim reports the issue
326: 2. Agent collects: name, phone, address, nature of emergency
327: 3. Dispatch team is notified within [N] minutes
328: 4. Crew arrives within [time window]
329: 5. Assessment is free; work begins after approval
330: 
331: **After-hours emergency line:** [Phone]
332: 
333: ---
334: 
335: <!-- @section: pricing -->
336: ## Pricing Information
337: 
338: <!-- @faq: pricing -->
339: ### How much does [service] cost?
340: [Honest, helpful answer that sets expectations without quoting fixed prices
341: if they vary. Include factors that affect cost.]
342: 
343: Example:
344: > Costs vary based on [factors]. For a pressure washing job, we consider
345: > square footage, surface type, level of staining, and accessibility.
346: > We offer free estimates so you know the exact cost before we begin.
347: 
348: **Do not quote exact prices unless they are fixed and current.**
349: 
350: ---
351: 
352: <!-- @section: faqs -->
353: ## Frequently Asked Questions
354: 
355: <!-- @faq: hours -->
356: ### What are your hours?
357: [Clear answer about business hours + emergency availability]
358: 
359: <!-- @faq: estimate -->
360: ### Do you offer free estimates?
361: [Yes/No + details]
362: 
363: <!-- @faq: payment -->
364: ### What payment methods do you accept?
365: [List: credit cards, checks, cash, financing options]
366: 
367: <!-- @faq: insurance -->
368: ### Are you licensed and insured?
369: [Details about licensing, insurance coverage, bonding]
370: 
371: ---
372: 
373: <!-- @section: process -->
374: ## What To Expect
375: 
376: ### Step 1: Contact
377: Call, text, or use this chat. We respond within [timeframe].
378: 
379: ### Step 2: Assessment
380: We assess the situation (free for most services).
381: 
382: ### Step 3: Quote
383: You get a clear, written quote before work begins.
384: 
385: ### Step 4: Service
386: We complete the work on the agreed schedule.
387: 
388: ### Step 5: Follow-Up
389: We confirm satisfaction and document any warranty info.
390: ```
391: 
392: ### Annotation System
393: 
394: The `<!-- @type: value -->` annotations make content machine-readable for the RAG system. Here are the ones you will use most:
395: 
396: #### KB.MD Annotations
397: 
398: | Annotation | Purpose | Example |
399: |------------|---------|---------|
400: | `@service: code, emergency: true, response_time: "60min"` | Mark a service entry | `@service: water_extraction, emergency: true` |
401: | `@exclusion: code, exception: "secondary to X"` | Mark a service NOT offered | `@exclusion: mold_remediation` |
402: | `@coverage: include/exclude` | Mark service area sections | `@coverage: include` |
403: | `@faq: category` | Mark FAQ entries | `@faq: pricing` |
404: | `@section: name` | Mark logical document sections | `@section: emergency_protocol` |
405: | `@safety: hazard_type` | Mark safety-related content | `@safety: electrical_hazard` |
406: 
407: ### Writing Rules
408: 
409: **DO:**
410: - Write every service, exclusion, coverage area, and FAQ precisely
411: - Use bullet points — the agent handles structured content better than long paragraphs
412: - Include every phone number, email, address, and emergency contact
413: - Name specific cities, ZIP codes, neighborhoods
414: - Specify exact time windows ("within 60 minutes" not "fast response")
415: - Include pricing structure and factors that affect cost
416: - List accepted payment methods
417: - State licensing, insurance, bonding status
418: - Note warranty/guarantee terms
419: - Include any permitting requirements
420: 
421: **DON'T:**
422: - Write vague content ("we do great work at fair prices")
423: - Use competitor names or reference competitor pricing
424: - Include content from other businesses (even template "fill-in-the-blanks" that's not been customized)
425: - Leave placeholder text like "[Your Company Name]"
426: - Include personal/private information about the owner
427: - Promise things the business doesn't actually deliver
428: - Quote exact prices that change frequently without noting "starting at" or "varies"
429: 
430: ### Content Quality Checklist
431: 
432: Before publishing a KB.MD, verify:
433: - [ ] Every service the business offers has a `<!-- @service -->` entry
434: - [ ] Every major service NOT offered has an `<!-- @exclusion -->` entry
435: - [ ] All service areas (cities, ZIPs, counties) are listed with **include** or **exclude**
436: - [ ] All phone numbers, emails, and physical addresses are current
437: - [ ] Emergency protocol includes after-hours contact
438: - [ ] Pricing section sets expectations without false specific quotes
439: - [ ] FAQs cover: hours, estimates, payment, insurance, scheduling, cancellation
440: - [ ] No placeholder text like `[fill in]` or `[company name]`
441: - [ ] File is named exactly `KB.MD` (case-insensitive) when uploaded
442: 
443: ---
444: 
445: ## 8. POLICY.md — Business Rules, Do's and Don'ts
446: 
447: **POLICY.MD = the agent's operating system.** It defines who the agent is, how it behaves, what it can and cannot do, and how it handles edge cases.
448: 
449: ### Structure Template
450: 
451: ```markdown
452: <!-- @identity, tone: professional_empathetic, voice: first_person_plural -->
453: ## Identity
454: 
455: - **Name:** [Agent Name — or use business name]
456: - **Role:** Customer Service Representative for [Business Name]
457: - **Tone:** [Professional and warm / Urgent and direct / Casual and friendly]
458: - **Voice:** First-person plural ("we") or third-person ("the team")
459: - **Escalation contact:** [Name/role, phone, email]
460: - **Business hours:** [e.g., "Monday–Saturday, 7 AM – 7 PM"]
461: - **After-hours handling:** [e.g., "Collect lead, promise morning callback"]
462: 
463: ---
464: 
465: ## Behavioral Rules
466: 
467: <!-- @rule: id, priority: high, condition: always -->
468: 1. **Never** make up pricing. If not in KB.MD, say so and offer an estimate.
469: 2. **Never** promise availability or a specific crew without checking with dispatch.
470: 3. **Always** collect contact info before transferring to human (if applicable).
471: 4. **Always** treat the customer with respect, even if they're frustrated.
472: 5. **Never** share internal business information (costs, margins, employee issues).
473: 6. **If** the visitor asks about a competitor, redirect gracefully to [Business Name]'s strengths.
474: 7. **If** the visitor asks about something outside our services, say so and offer alternative suggestions from KB.MD exclusions.
475: 8. **Never** pretend to be human if the user directly asks "are you a bot/robot/AI?"
476: 
477: ---
478: 
479: ## Intents (Trigger Conditions)
480: 
481: <!-- @intent: emergency, urgency_threshold: 75, priority: P0 -->
482: ### Emergency
483: A customer reports urgent, time-sensitive damage or safety issue.
484: 
485: **Agent behavior:**
486: - Acknowledge urgency immediately
487: - Collect: name, phone, location, nature of emergency
488: - Confirm dispatch within stated response window
489: - Transfer to human immediately
490: 
491: ---
492: 
493: <!-- @intent: quote_request, priority: P1 -->
494: ### Quote Request
495: A customer wants pricing or an estimate.
496: 
497: **Agent behavior:**
498: - Acknowledge request
499: - Collect: name, contact, service needed, location, timeframe
500: - Explain what determines pricing (per KB.MD)
501: - NEVER give exact figure unless listed in KB.MD as fixed-price
502: - Set expectation: "We'll follow up with a detailed quote"
503: 
504: ---
505: 
506: <!-- @intent: schedule_service, priority: P1 -->
507: ### Schedule Service
508: A customer wants to book an appointment.
509: 
510: **Agent behavior:**
511: - Check availability framework (if configured)
512: - Collect preferred date/time
513: - Confirm or offer alternatives
514: - Send confirmation details
515: 
516: ---
517: 
518: <!-- @intent: escalation, priority: P0 -->
519: ### Escalation
520: A customer explicitly asks for a human, complains, or uses escalation keywords.
521: 
522: **Trigger keywords:** "manager", "supervisor", "lawyer", "complain", "unsatisfied", "human", "real person"
523: 
524: **Agent behavior:**
525: - Acknowledge frustration
526: - Collect: name, contact, brief summary of issue
527: - Confirm: "I'm connecting you with [Name]. They'll reach you within [timeframe]."
528: - Pause chat; do not continue resolving on behalf of human without instruction
529: 
530: ---
531: 
532: ## Thresholds
533: 
534: <!-- @threshold: urgency_emergency, value: 75 -->
535: ### Urgency Scoring
536: 
537: | Score Range | Classification | Agent Response |
538: |-------------|---------------|----------------|
539: | 75–100 | **EMERGENCY** | Immediate acknowledgment, dispatch within response window |
540: | 40–74 | **URGENT** | Priority response within 1 hour during business hours |
541: | 1–39 | **STANDARD** | Response within normal service window |
542: | 0 | **INFORMATIONAL** | Standard informational response, no urgency flags |
543: 
544: ---
545: 
546: ## Fallback Rules
547: 
548: <!-- @rule: fallback, priority: medium -->
549: ### When the Agent Doesn't Know
550: 
551: If the agent cannot answer a question using KB.MD or POLICY.MD:
552: 
553: 1. **Say so honestly. Not** "I think so" or hedging guesses.
554: 2. **Offer an alternative:** "I don't have that information in my knowledge base, but I can [collect your contact info and have someone follow up / connect you with our team]."
555: 3. **Transition to lead capture** — ask for name, contact, and question so a human can respond.
556: 
557: This is called the **"Say Unknown + Capture"** behavior. It protects trust and turns unknowns into leads.
558: 
559: ---
560: 
561: ## Lead Capture Policy
562: 
563: <!-- @rule: lead_capture -->
564: ### When to Ask for Contact Details
565: 
566: The agent should collect contact info when:
567: - The visitor asks for a quote or estimate
568: - The visitor asks to schedule or book
569: - The visitor reports an emergency
570: - The visitor explicitly provides info (agent confirms and stores)
571: - The conversation reaches a natural close after a substantive inquiry
572: 
573: The agent should NOT aggressively collect contact info:
574: - On the very first message before understanding the need
575: - If the visitor only says "hi" or "hello"
576: - If the visitor asks a simple FAQ that is resolved in KB.MD
577: 
578: ### Minimum Required Lead Fields
579: - **Name** OR **Phone** (at least one)
580: - **Email** OR **Phone** (at least one)
581: - **Service/need** (extracted from conversation)
582: - **Location** (if service is location-dependent)
583: - **Urgency level** (from urgency scoring above)
584: 
585: ### Decline Handling
586: If the visitor says "I'd rather not share" or similar:
587: - Accept gracefully: "No problem — feel free to reach out anytime."
588: - Mark the lead as `declined: true` so the system doesn't re-prompt
589: - Do NOT block the conversation; continue helping
590: 
591: ---
592: 
593: ## Human Takeover / Escalation
594: 
595: <!-- @rule: human_takeover -->
596: ### When to Hand Off to a Human
597: 
598: The agent should trigger human takeover when:
599: - The visitor explicitly requests a human
600: - Urgency score = 75+ (emergency)
601: - The conversation is clearly frustrated/escalating
602: - A question requires legal/contractual/liability decisions
603: - The visitor says "I need to speak to someone"
604: 
605: ### After Human Takeover
606: - Show a transition message: "I'm connecting you with [Name] from our team. They'll respond within [timeframe]."
607: - Create a system notification/email for the human agent
608: - Do NOT continue resolving the issue autonomously
609: - Optionally create a support ticket
610: ```
611: 
612: ### POLICY.md Rules of Thumb
613: 
614: **DO:**
615: - Define the agent's identity as clearly as you would a new employee's job description
616: - Specify exact trigger words for escalation
617: - Set urgency score thresholds that match the business's actual emergency response times
618: - Include after-hours behavior explicitly
619: - Define what happens when the agent doesn't know the answer
620: - Include anti-hallucination rules: "Never guess pricing, availability, or policy"
621: 
622: **DON'T:**
623: - Write POLICY.md as if the agent is a human — it is an AI assistant representing the business
624: - Use vague rules ("be polite") without defining what that means operationally
625: - Forget to define escalation behavior — this is the #1 cause of bad customer experiences
626: - Allow the agent to negotiate pricing, promises, or guarantees
627: 
628: ---
629: 
630: ## 9. SCRIPT.md — Conversation Flow
631: 
632: **SCRIPT.MD = the agent's playbook.** It defines the stages of a conversation and what happens at each stage.
633: 
634: ### Template
635: 
636: ```markdown
637: ## Stage: Opening
638: - Greet the visitor warmly
639: - State who you represent: "I'm the [Business Name] assistant"
640: - Ask how you can help today
641: - DO NOT collect contact info yet — understand the need first
642: 
643: ## Stage: Discovery
644: - Ask clarifying questions to understand the service needed
645: - Confirm location/service area if relevant
646: - Determine urgency (emergency vs. standard)
647: 
648: ## Stage: Information
649: - Answer questions using KB.MD
650: - If answer not in KB.MD → use fallback rule
651: - Be specific, not vague
652: 
653: ## Stage: Qualification
654: - Once service type is understood and visitor shows intent:
655:   - Collect: name, email/phone, service needed, location, timeframe
656: - Confirm: "I have your details — [Name] will follow up within [timeframe]"
657: 
658: ## Stage: Closing
659: - Confirm next step
660: - Provide any reference numbers or expectation-setting
661: - Thank the visitor
662: - Invite them to reach out again
663: ```
664: 
665: ### Key Principle
666: 
667: The agent should never ask for contact details before it understands what the visitor needs. The conversation should feel natural: the agent is helping first, collecting second.
668: 
669: ---
670: 
671: ## 10. Deploying the Chat Widget
672: 
673: The widget is the visible chat interface your website visitors interact with. There are three embedding approaches:
674: 
675: ### Option 1: JavaScript Embed (Recommended)
676: 
677: Best for custom websites, full control, floating bubble + expandable panel.
678: 
679: ```html
680: <!DOCTYPE html>
681: <html>
682: <head>
683:   <title>Acme Pressure Washing</title>
684: </head>
685: <body>
686:   <!-- Your existing website content -->
687:   <h1>Welcome to Acme Pressure Washing</h1>
688:   <p>We serve the greater Phoenix area...</p>
689: 
690:   <!-- bchat widget -->
691:   <script
692:     src="https://your-server.com/widget/acme-pressure-washing/embed.js"
693:     data-position="bottom-right"
694:     data-color="#0d9488"
695:     data-welcome="Hi! How can we help with your pressure washing needs today?"
696:   ></script>
697: </body>
698: </html>
699: ```
700: 
701: **Configuration options:**
702: 
703: | Attribute | Description | Example |
704: |-----------|-------------|---------|
705: | `data-position` | Button position | `bottom-right`, `bottom-left` |
706: | `data-color` | Brand color (hex) | `#0d9488` |
707: | `data-welcome` | Initial greeting message | `"Hi! How can we help?"` |
708: 
709: ### Option 2: iframe Embed
710: 
711: Best for WordPress, Wix, Shopify, or any CMS where you can paste an iframe code.
712: 
713: ```html
714: <iframe
715:   src="https://your-server.com/widget/acme-pressure-washing/iframe?color=%230d9488"
716:   style="position:fixed;bottom:0;right:0;width:400px;height:600px;border:none;z-index:9999;"
717:   title="Chat with Acme Pressure Washing"
718: ></iframe>
719: ```
720: 
721: ### Option 3: Legacy Script (Backward Compatible)
722: 
723: ```html
724: <script src="https://your-server.com/api/v1/agent/acme-pressure-washing/widget.js"></script>
725: ```
726: 
727: ### Domain Restrictions
728: 
729: For security, you can restrict which domains can embed your widget:
730: 
731: In the tenant settings, set `Allowed Domains` to:
732: ```
733: acmepw.com, www.acmepw.com, acmepw.net
734: ```
735: 
736: Leave blank to allow embedding on any domain.
737: 
738: ### Widget Behavior at Runtime
739: 
740: When a visitor opens the widget:
741: 1. Widget loads tenant-specific config (slug, company name, color)
742: 2. Widget opens a new conversation session
743: 3. Visitor sends a message → goes to `POST /api/v1/agent/:slug/chat`
744: 4. Backend:
745:    - Looks up tenant by slug
746:    - Parses KB.MD / POLICY.MD / SCRIPT.MD
747:    - Optionally runs RAG search to find relevant chunks
748:    - Builds system prompt
749:    - Calls OpenRouter LLM
750:    - Returns streaming SSE response
751: 5. Agent response renders in widget
752: 6. If agent detects lead info → stores in `agent_leads` table
753: 7. Business owner sees lead in admin panel
754: 
755: ---
756: 
757: ## 11. Lead Capture Workflow
758: 
759: Lead capture is automatic and conversational — no forms required.
760: 
761: ### How It Works
762: 
763: 1. **Conversational extraction:** The agent asks for contact details naturally during the conversation (not on first message).
764: 2. **Multi-message accumulation:** The agent collects name, email, phone, and other fields across multiple turns.
765: 3. **Intelligent merging:** A three-layer extraction pipeline combines regex, structural analysis, and AI extraction.
766: 4. **Confidence scoring:** Each field has a confidence score; high-confidence leads are committed immediately.
767: 5. **Correction handling:** "No, I meant john2000@email.com" replaces the previous email.
768: 6. **Decline handling:** "I'd rather not share" sets `declined: true` and stops re-prompting.
769: 
770: ### Lead Fields
771: 
772: | Field | Required | Source |
773: |-------|----------|--------|
774: | `tenant_id` | Always | From session context |
775: | `session_id` | Always | From chat session |
776: | `customer_name` | At least one of name/email/phone | Extracted from conversation |
777: | `customer_email` | At least one of name/email/phone | Extracted from conversation |
778: | `customer_phone` | At least one of name/email/phone | Extracted from conversation |
779: | `customer_location` | Optional | Extracted from context |
780: | `service_interest` | Optional | From RAG context or conversation |
781: | `detected_intent` | Optional | Intent classification (emergency/quote/schedule/etc.) |
782: | `urgency_level` | Optional | Urgency score from POLICY.MD thresholds |
783: | `transcript_summary` | Auto | Brief summary of conversation |
784: | `status` | Default: `new` | new → contacted → converted → lost |
785: | `extraction_metadata` | Auto | JSON with confidence scores, extraction method |
786: 
787: ### Viewing Leads
788: 
789: 1. Go to **Agent Admin → [Tenant] → Leads**
790: 2. See list of all leads with: Name, Email, Phone, Service, Status, Created At
791: 3. Filter by status: new, contacted, converted, lost
792: 4. Click a lead to see full transcript
793: 5. Export as CSV for CRM import
794: 
795: ### Lead Notification (Admin)
796: 
797: Leads are visible in the admin panel. For email/SMS notification:
798: - The platform captures leads locally
799: - You (or Pithom Labs) implement a notification bridge using the admin API or outbox system
800: - Email digests can be batched and sent on a schedule
801: 
802: ---
803: 
804: ## 12. The Admin Panel
805: 
806: Access at: `https://your-server.com/agent-admin`
807: 
808: ### Sections
809: 
810: | Section | What It Does |
811: |---------|-------------|
812: | **Tenants** | List, create, edit, activate/deactivate businesses |
813: | **Tenant Detail** | Upload KB.MD, POLICY.MD, SCRIPT.MD; configure LLM model; manage users |
814: | **Leads** | View, filter, export leads for a tenant |
815: | **RAG Debug** | Test what the vector search retrieves for any query (admin only) |
816: | **Simulations** | Run virtual customer conversations to test the agent before going live |
817: | **Transcripts** | View and export full conversation logs |
818: 
819: ### Admin Permissions (RBAC)
820: 
821: | Permission | Access Level |
822: |------------|-------------|
823: | `tenant:admin` | Full tenant management — create, edit, delete tenants |
824: | `tenant:read` | View tenant configuration |
825: | `api:config` | Configure LLM settings, rebuild index |
826: | `chat:test` | Run simulations, view test chat history |
827: | `chat:logs` | View real chat session logs |
828: | `files:upload` | Upload KB.MD, POLICY.MD, SCRIPT.MD |
829: 
830: ### Permissions Are Assigned Per User Per Tenant
831: When you add a team member, assign them a preset role:
832: - **Viewer** — read-only access
833: - **Tester** — can run simulations and view logs
834: - **Editor** — can upload files and edit content
835: - **Tenant Admin** — full control over that tenant
836: 
837: ---
838: 
839: ## 13. RAG Knowledge Base Management
840: 
841: **RAG (Retrieval-Augmented Generation)** is how the agent finds relevant knowledge before answering. Without RAG, the entire KB.MD is stuffed into the LLM prompt (works for small files, degrades for large ones). With RAG, only the most relevant chunks are retrieved and shown to the LLM.
842: 
843: ### RAG Pipeline (Business View)
844: 
845: ```
846: Visitor asks: "Do you handle mold remediation?"
847:          │
848:          ▼
849: [1] Agent sends query to embedding service → converts to vector
850:          │
851:          ▼
852: [2] Vector search in LanceDB → finds top 5 most relevant KB chunks
853:          │
854:          ▼
855: [3] Agent builds LLM prompt: "Here are the relevant facts: [chunks]. Now answer the visitor's question."
856:          │
857:          ▼
858: [4] LLM returns grounded, accurate answer
859: ```
860: 
861: ### Managing RAG
862: 
863: After uploading or updating KB.MD:
864: 
865: 1. Click **Rebuild Index** in the tenant admin panel
866: 2. System chunks the file, generates embeddings, stores in LanceDB
867: 3. This takes 30–120 seconds depending on file size
868: 4. Admin can see: total chunks indexed, last indexed timestamp, any errors
869: 
870: ### Hybrid Search
871: 
872: RAG uses hybrid search by default:
873: - **70% vector similarity** (semantic meaning — finds related concepts)
874: - **30% BM25 keyword matching** (exact terms — finds exact service names, city names)
875: 
876: This means both "water removal" and "standing water extraction" return the same relevant chunk.
877: 
878: ### What to Expect
879: 
880: | File Size | Chunks Created | Index Time |
881: |-----------|---------------|------------|
882: | 5 KB | ~10 | 10s |
883: | 20 KB | ~40 | 30s |
884: | 100 KB | ~200 | 90s |
885: | 500 KB | ~1000 | 4 min |
886: 
887: ### RAG vs. Long Context
888: 
889: The system automatically chooses:
890: - **RAG mode** when KB content exceeds ~6,000 tokens (about 24 KB)
891: - **Long context mode** when content is smaller (everything goes in the prompt)
892: 
893: This is handled automatically. You don't need to configure anything.
894: 
895: ---
896: 
897: ## 14. Testing Before Going Live
898: 
899: ### Simulation Testing
900: 
901: Before exposing the widget to real visitors:
902: 
903: 1. Go to **Agent Admin → [Tenant] → Simulations**
904: 2. Configure a virtual customer persona:
905:    - **Name:** John Doe
906:    - **Scenario:** Needs emergency water extraction in Phoenix
907:    - **Goal:** Get a quote and schedule service ASAP
908:    - **Tone:** Urgent, slightly anxious
909: 3. Click **Run Simulation**
910: 4. Watch the conversation unfold in real-time (SSE streaming)
911: 5. Review transcript:
912:    - Did the agent ask the right qualifying questions?
913:    - Did it capture name, email, phone?
914:    - Did it handle the urgency correctly?
915:    - Did it hallucinate any information?
916:    - Did it follow POLICY.MD rules?
917: 
918: ### RAG Debug Testing
919: 
920: 1. Go to **Agent Admin → [Tenant] → RAG Debug**
921: 2. Type a test query: "emergency water extraction in Phoenix"
922: 3. See what chunks were retrieved, with relevance scores
923: 4. Verify: Are the right services showing up? Are exclusions being respected?
924: 
925: ### Recommended Pre-Launch Checklist
926: 
927: - [ ] KB.MD has been reviewed by business owner (not just copy-pasted)
928: - [ ] POLICY.MD covers: identity, escalation, urgency thresholds, fallback behavior
929: - [ ] SCRIPT.MD defines clear conversation stages
930: - [ ] All phone numbers, emails, and service areas are verified
931: - [ ] RAG index rebuilt after any file upload
932: - [ ] At least 3 simulations run and passed
933: - [ ] RAG debug returns correct chunks for 10+ test queries
934: - [ ] Lead capture tested: name + email/phone extracted correctly
935: - [ ] Escalation workflow tested: "I want to speak to a manager" → proper handoff
936: - [ ] Widget embedded on staging/test website
937: - [ ] Widget looks correct on mobile
938: - [ ] Domain restrictions set (if applicable)
939: - [ ] Business owner has admin access and knows how to view leads
940: 
941: ---
942: 
943: ## 15. Common Business Workflows
944: 
945: ### Workflow 1: After-Hours Lead
946: ```
947: 22:00 — Visitor lands on website, asks about emergency water extraction
948: 22:00 — Agent responds immediately (no human needed)
949: 22:01 — Agent collects: name (Sarah), phone (555-0142), address (Phoenix), nature of emergency (bathroom flooding)
950: 22:02 — Agent says: "I've sent this to our on-call team. Someone will call you within 30 minutes."
951: 22:03 — Lead created with urgency_score = emergency, status = new
952: 22:03 — (Optional) SMS/email alert sent to on-call technician
953: ```
954: 
955: ### Workflow 2: Informational Inquiry Becomes Lead
956: ```
957: 14:00 — Visitor: "Do you handle mold remediation?"
958: 14:00 — Agent: "We don't provide standalone mold remediation, but if the mold is
959:            secondary to water damage, we can help with the water extraction first.
960:            Would you like to schedule an assessment?"
961: 14:02 — Visitor: "Yes, that sounds right"
962: 14:02 — Agent collects contact details
963: 14:03 — Lead captured with service = water_extraction, notes = "mold secondary"
964: ```
965: 
966: ### Workflow 3: Self-Service FAQ (No Lead)
967: ```
968: 10:00 — Visitor: "What are your business hours?"
969: 10:00 — Agent: "We're open Monday–Saturday, 7 AM – 7 PM. For emergencies,
970:            our 24/7 line is 555-0199."
971: 10:00 — Lead NOT created (no contact info, no service interest)
972: ```
973: 
974: ### Workflow 4: Escalation
975: ```
976: 16:00 — Visitor: "This is the third time I've called and nobody shows up"
977: 16:00 — Agent detects escalation keywords + high urgency
978: 16:00 — Agent: "I understand your frustration. I'm connecting you with [Manager Name]
979:            right now. They'll call you within 15 minutes."
980: 16:00 — Agent pauses chat, creates internal alert, marks session for human follow-up
981: ```
982: 
983: ---
984: 
985: ## 16. Policy & Legal Guardrails
986: 
987: ### Compliance Notes
988: 
989: **CAN-SPAM (Email):**
990: - Commercial emails must have truthful headers, non-deceptive subject lines
991: - Must identify sender and include physical address
992: - Must provide opt-out mechanism
993: - Applies to lead follow-up emails sent by your business
994: 
995: **TCPA (SMS/Text):**
996: - Consent is required for automated marketing text messages
997: - Must provide opt-out mechanism
998: - Consent revocation must be honored immediately
999: - Lead capture chat ≠ consent for SMS unless explicitly agreed
1000: 
1001: **State Privacy Laws (CCPA/CPRA/VCDPA):**
1002: - California, Colorado, Virginia, and other states have consumer privacy laws
1003: - You may need to disclose data collection in your privacy policy
1004: - Visitors should understand their contact info will be stored for follow-up
1005: 
1006: ### Recommended Legal Language
1007: 
1008: Add to your client's website near the chat widget:
1009: 
1010: > By using this chat, you agree that [Business Name] may collect your name, email, and phone number for the purpose of following up on your inquiry. We do not share your information with third parties. See our Privacy Policy for full details.
1011: 
1012: Add to your privacy policy:
1013: 
1014: > Contact Information: When you contact us through our website chat, we collect your name, email address, phone number, and the details of your inquiry. This information is used solely to respond to your request and is not shared with third parties without your consent.
1015: 
1016: ---
1017: 
1018: ## 17. What NOT To Do (Anti-Patterns)
1019: 
1020: These are the most common mistakes that destroy trust and waste money:
1021: 
1022: | Anti-Pattern | Why It's Bad | What To Do Instead |
1023: |-------------|-------------|-------------------|
1024: | **Copy-paste KB.MD from another business** | Agent gives wrong services, pricing, coverage areas | Write KB.MD from scratch for each business |
1025: | **Vague service descriptions** | Agent can't answer "Do you do X?" accurately | Be specific: what's included, what's not, what triggers the service |
1026: | **No exclusions section** | Agent promises services the business doesn't offer | Always list what's NOT included |
1027: | **No after-hours policy** | Agent behaves as if business is 24/7 | Define after-hours behavior in POLICY.MD |
1028: | **Hardcoded pricing in KB.MD** | Agent quotes stale or wrong prices | Describe pricing structure, not fixed numbers (unless truly fixed) |
1029: | **Skipping simulation** | Agent has untested blind spots | Run at least 3 simulations per tenant before launch |
1030: | **Ignoring RAG rebuilds** | Agent answers from stale or missing knowledge | Always rebuild index after KB.MD changes |
1031: | **Over-engineering SCRIPT.MD** | Long scripts that LLM ignores | Keep SCRIPT.MD to clear stage names and bullet points |
1032: | **Setting temperature > 0.8** | Agent becomes creative, hallucinates quotes/promises | 0.3–0.5 for local service businesses; 0.7 max for creative verticals |
1033: | **Linking widget to wrong tenant slug** | Agent uses wrong knowledge base | Double-check the slug in the embed URL matches the tenant |
1034: | **Leaving widget on broken tenant** | Visitor sees errors, trust lost | Always verify tenant is `Is Active = true` before deploying |
1035: | **Not reviewing leads daily** | Leads go cold, expectation mismatch between agent promises and human follow-up | Set a daily reminder to review new leads |
1036: 
1037: ---
1038: 
1039: ## 18. Pricing Model
1040: 
1041: ### Recommended Package Structure (Pithom Labs)
1042: 
1043: ```text
1044: Package 1 — Chat Widget Only
1045:   For: Businesses with an existing website
1046:   Setup: $149–$299
1047:   Monthly: $149–$299
1048:   Includes:
1049:     • bchat widget installation
1050:     • KB/Policy training for that business
1051:     • Lead capture + email notification
1052:     • Basic conversation transcript
1053:     • Human takeover/escalation
1054:     • Weekly lead report
1055: 
1056: Package 2 — Hugo Site + AI Lead Desk
1057:   For: Businesses with no website or obsolete web presence
1058:   Setup: $399–$999
1059:   Monthly: $199–$499
1060:   Includes:
1061:     • Fast Hugo landing site
1062:     • Domain/DNS configuration help
1063:     • Service pages + quote/request form
1064:     • bchat widget (included)
1065:     • Lead notification workflow
1066:     • Google Business Profile CTA optimization
1067:     • Monthly updates
1068: 
1069: Package 3 — Managed Lead Desk
1070:   For: Growing businesses that want full outsourcing
1071:   Monthly: $499–$1,500
1072:   Includes:
1073:     • Everything from Package 1 or 2
1074:     • CRM or Google Sheets lead pipeline
1075:     • Appointment routing
1076:     • Missed-lead follow-up sequences
1077:     • Conversation quality review
1078:     • Monthly lead optimization report
1079:     • Owner-facing dashboard
1080: ```
1081: 
1082: ### Internal Cost Structure (What It Costs You)
1083: 
1084: | Cost Component | Estimate | Notes |
1085: |---------------|----------|-------|
1086: | OpenRouter API | $5–$50/tenant/month | Depends on chat volume + model choice. GPT-4o-mini is cheap and fast. |
1087: | Server hosting (Fly.io) | $15–$50/month | Small VM sufficient for 25–50 tenants |
1088: | LanceDB storage | negligible | Stored on same disk |
1089: | Domain + SSL | $1–$2/month | Per custom tenant domain if used |
1090: | **Your total cost per tenant** | **$5–$30/month** | Margins of 80–90% on the $199–$499/month retail price |
1091: 
1092: ---
1093: 
1094: ## 19. Troubleshooting
1095: 
1096: ### Agent doesn't know about a service
1097: 
1098: **Cause:** KB.MD doesn't contain that service, or RAG index hasn't been rebuilt.
1099: 
1100: **Fix:**
1101: 1. Check KB.MD has `<!-- @service: code, emergency: true/false -->` entry
1102: 2. In Admin panel → Rebuild Index
1103: 3. Test via RAG Debug panel
1104: 
1105: ### Agent gives wrong answer (hallucination)
1106: 
1107: **Cause:** RAG not enabled, content not in KB, or temperature too high.
1108: 
1109: **Fix:**
1110: 1. Verify `RAG_PIPELINE_ENABLED=true` in environment
1111: 2. Add missing content to KB.MD
1112: 3. Rebuild index
1113: 4. Lower `temperature` to 0.3–0.5
1114: 5. Ensure POLICY.MD has the `@rule: fallback` section — agent should say "I don't have that information" rather than guessing
1115: 
1116: ### Agent doesn't capture leads
1117: 
1118: **Cause:** Lead extraction regex doesn't match the input format.
1119: 
1120: **Fix:**
1121: 1. Check that the conversation actually contained name + email/phone
1122: 2. Verify the lead fields are being extracted from session messages
1123: 3. Check that `customer_name` and `customer_email` or `customer_phone` are populated
1124: 4. Test with known inputs: "My name is John Smith, my email is john@example.com"
1125: 5. Check `agent_leads` table: `SELECT * FROM agent_leads ORDER BY id DESC LIMIT 5;`
1126: 
1127: ### Widget won't load on website
1128: 
1129: **Cause:** Wrong slug in embed URL, domain restriction blocking, or server not reachable.
1130: 
1131: **Fix:**
1132: 1. Verify embed URL: `https://your-server.com/widget/EXACT-SLUG/embed.js`
1133: 2. Check domain is in `Allowed Domains` in tenant settings
1134: 3. Check CORS if using script embed
1135: 4. Open browser DevTools → Console → check for errors
1136: 5. Verify `Is Active = true` for the tenant
1137: 
1138: ### RAG index keeps failing to build
1139: 
1140: **Cause:** Embedding API issue, chunking error, or network timeout.
1141: 
1142: **Fix:**
1143: 1. Check `OPENROUTER_API_KEY` is set and valid
1144: 2. Check server logs: `grep "RAG" build/memos.log`
1145: 3. Increase `EMBEDDING_TIMEOUT` to 300s for large files
1146: 4. Verify LanceDB storage path exists and is writable
1147: 5. Try mock embeddings: `task run:rag:mock` (no API key needed)
1148: 
1149: ### Leads not showing in admin panel
1150: 
1151: **Cause:** Tenant filter, status filter, or lead creation failure.
1152: 
1153: **Fix:**
1154: 1. Verify you are viewing leads for the correct tenant
1155: 2. Check `status` filter — default shows `new` only
1156: 3. Check `agent_leads` table directly in SQLite/PostgreSQL
1157: 4. Verify `captureLeadFromSession` is being called (check server logs)
1158: 5. Ensure lead extraction pipeline completed (check `extraction_metadata` JSON in lead record)
1159: 
1160: ---
1161: 
1162: ## Appendix A: Complete Environment Variables
1163: 
1164: | Variable | Required | Default | Purpose |
1165: |----------|----------|---------|---------|
1166: | `OPENROUTER_API_KEY` | Yes | (none) | OpenRouter LLM access |
1167: | `LLM_MODEL` | No | `openai/gpt-4o-mini` | Default chat model |
1168: | `LLM_MODEL_REASONING` | No | `google/gemini-2.5-pro` | Content generation model |
1169: | `RAG_PIPELINE_ENABLED` | No | `false` | Enable/disable RAG |
1170: | `EMBEDDING_PROVIDER` | No | `openrouter` | `openrouter`, `local`, `mock` |
1171: | `EMBEDDING_MODEL` | No | `text-embedding-3-small` | Embedding model name |
1172: | `EMBEDDING_TIMEOUT` | No | `180s` | Embedding API timeout |
1173: | `EMBEDDING_BATCH_SIZE` | No | `10` | Chunks per embedding batch |
1174: | `LANCEDB_STORAGE_PROVIDER` | No | `local` | `local`, `s3` |
1175: | `LANCEDB_LOCAL_PATH` | No | `build/data/lancedb` | Disk path for LanceDB |
1176: | `LANCEDB_S3_BUCKET` | No | (none) | S3 bucket for LanceDB |
1177: | `HYBRID_SEARCH_ENABLED` | No | `true` | Enable hybrid vector+BM25 search |
1178: | `HYBRID_VECTOR_WEIGHT` | No | `0.7` | Vector similarity weight |
1179: | `HYBRID_TEXT_WEIGHT` | No | `0.3` | BM25 keyword weight |
1180: | `OM_ENABLED` | No | `false` | Observational Memory |
1181: | `OM_OBSERVER_TOKEN_THRESHOLD` | No | `30000` | Token threshold for observer |
1182: | `OM_TOKEN_THRESHOLD` | No | `2000` | Token threshold for reflector |
1183: | `FORCE_REINDEX_ON_STARTUP` | No | `false` | Reindex all content at boot |
1184: | `LLM_VERIFIER_ENABLED` | No | `false` | LLM response verification layer |
1185: | `ENCRYPTION_MASTER_KEY` | No | (none) | Encrypt tenant API keys at rest |
1186: 
1187: **Configuration priority (highest → lowest):**
1188: 1. Tenant-specific setting (set in Admin UI)
1189: 2. Environment variable
1190: 3. Hardcoded default
1191: 
1192: ---
1193: 
1194: ## Appendix B: File Naming Conventions
1195: 
1196: When uploading files to a tenant:
1197: 
1198: | File Name | Purpose | Required |
1199: |-----------|---------|----------|
1200: | `KB.MD` | Knowledge base | Yes |
1201: | `POLICY.MD` | Agent policy & rules | Yes |
1202: | `SCRIPT.MD` | Conversation flow | No (recommended) |
1203: | `KB_<epoch>.MD` | Versioned backup | Auto |
1204: | `POLICY_<epoch>.MD` | Versioned backup | Auto |
1205: 
1206: **Important:** File names are case-insensitive in the upload system, but `KB.MD`, `POLICY.MD`, and `SCRIPT.MD` are the canonical names used in documentation.
1207: 
1208: ---
1209: 
1210: ## Appendix C: API Endpoints Reference
1211: 
1212: ### Public (No Auth)
1213: 
1214: | Method | Path | Purpose |
1215: |--------|------|---------|
1216: | `POST` | `/api/v1/agent/:slug/chat` | Send a chat message |
1217: | `GET` | `/api/v1/agent/:slug/chat/stream` | SSE stream of responses |
1218: | `GET` | `/api/v1/agent/:slug/widget.js` | Legacy widget script |
1219: | `GET` | `/widget/:slug/embed.js` | JavaScript embed |
1220: | `GET` | `/widget/:slug/iframe` | iframe embed |
1221: 
1222: ### Admin (Authenticated)
1223: 
1224: | Method | Path | Permission | Purpose |
1225: |--------|------|-----------|---------|
1226: | `GET` | `/api/v1/agent/tenants` | `tenant:admin` | List tenants |
1227: | `POST` | `/api/v1/agent/tenants` | `tenant:admin` | Create tenant |
1228: | `GET` | `/api/v1/agent/:slug` | `tenant:read` | Get tenant config |
1229: | `PUT` | `/api/v1/agent/:slug` | `tenant:admin` | Update tenant |
1230: | `POST` | `/api/v1/agent/:slug/files` | `files:upload` | Upload KB/Policy/Script |
1231: | `POST` | `/api/v1/agent/:slug/reindex` | `api:config` | Rebuild RAG index |
1232: | `POST` | `/api/v1/agent/:slug/simulate` | `chat:test` | Run simulation |
1233: | `GET` | `/api/v1/agent/:slug/simulations` | `chat:test` | List simulations |
1234: | `POST` | `/api/v1/agent/:slug/generate-kb` | `tenant:admin` | Auto-generate KB.MD |
1235: | `POST` | `/api/v1/agent/:slug/generate-policy` | `tenant:admin` | Auto-generate POLICY.MD |
1236: | `POST` | `/api/v1/agent/:slug/rag/search` | `api:config` | Test RAG search |
1237: | `GET` | `/api/v1/agent/:slug/leads` | `tenant:admin` | List leads |
1238: | `GET` | `/api/v1/agent/:slug/leads/:id` | `tenant:admin` | Lead detail |
1239: | `GET` | `/api/v1/agent/:slug/leads/export` | `tenant:admin` | Export CSV |
1240: 
1241: ---
1242: 
1243: ## Appendix D: Key Concepts Glossary
1244: 
1245: | Term | Definition |
1246: |------|-----------|
1247: | **Tenant** | One business/client. All data (KB, leads, transcripts) is scoped to a tenant. |
1248: | **Slug** | URL-safe tenant identifier (e.g., `acme-pressure-washing`). Used in embed URLs and API routes. |
1249: | **KB.MD** | Markdown knowledge base. The agent's factual memory. |
1250: | **POLICY.MD** | Markdown file defining agent identity, rules, intents, thresholds, and escalation behavior. |
1251: | **SCRIPT.MD** | Markdown file defining conversation stages and flow. |
1252: | **RAG** | Retrieval-Augmented Generation. The agent searches relevant KB chunks before LLM asks. |
1253: | **LanceDB** | Embedded vector database used for RAG storage. |
1254: | **Embedding** | Numerical vector representation of text, used for similarity search. |
1255: | **SSE** | Server-Sent Events. How the agent streams responses in real-time. |
1256: | **Lead** | A captured prospect (name, email, phone, service, urgency, transcript link). |
1257: | **Audience** | `external` (visitors/chat widget) or `internal` (staff/employees). Configs are per audience. |
1258: | **Idempotency** | Sending the same message twice produces the same stored result (no duplicates). |
1259: | **Urgency Score** | 0–100 score classifying a conversation as informational, standard, urgent, or emergency. |
1260: | **Simulation** | Virtual conversation test using an LLM-powered simulated customer. |
1261: | **Hybrid Search** | 70% vector similarity + 30% keyword matching (BM25). |
1262: 
1263: ---
1264: 
1265: *End of DOCS_HOWTO_BIZ.md*

(End of file - total 1265 lines)
</content>
```

---

## Assistant (Plan · MiMo V2.5 Free · 4.9s)

_Thinking:_

Now I have a clear picture. Let me also check the widget docs and a few more things before presenting the final plan.

Now let me check the widget docs and the simulation doc for completeness.

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/docs/DOCS_WIDGET.MD"
}
```

**Output:**
```
<path>/home/chaschel/Documents/go/bchat/docs/DOCS_WIDGET.MD</path>
<type>file</type>
<content>
1: # External Chat Widget
2: 
3: **Status:** Active
4: **Date:** 2026-01-25
5: **Related Documents:**
6: - [DOCS_README.MD](./DOCS_README.MD) - Project overview
7: - [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Environment variables
8: 
9: ---
10: 
11: ## Overview
12: 
13: The external chat widget allows you to embed an AI chat agent on any website. The widget supports multiple embedding approaches:
14: 
15: | Approach | Best For | Isolation |
16: |----------|----------|-----------|
17: | **JavaScript Embed** | General websites, full control | Low (shares DOM) |
18: | **iframe Embed** | CMS, WordPress, Shopify | High (isolated) |
19: | **Legacy Script** | Backward compatibility | Low (shares DOM) |
20: 
21: All approaches:
22: - Are **self-contained** - no external dependencies required
23: - **Auto-configure** based on tenant settings
24: - Provide a **floating chat button** (JS) or **full panel** (iframe)
25: 
26: ---
27: 
28: ## Quick Start
29: 
30: ### Option 1: JavaScript Embed (Recommended)
31: 
32: ```html
33: <script
34:   src="https://your-server.com/widget/your-tenant/embed.js"
35:   data-position="bottom-right"
36:   data-color="#0d9488"
37: ></script>
38: ```
39: 
40: ### Option 2: iframe Embed
41: 
42: ```html
43: <iframe
44:   src="https://your-server.com/widget/your-tenant/iframe?color=%230d9488"
45:   style="position:fixed;bottom:0;right:0;width:400px;height:600px;border:none;z-index:9999;"
46:   title="Chat Widget"
47: ></iframe>
48: ```
49: 
50: ### Option 3: Legacy Script (Backward Compatible)
51: 
52: ```html
53: <script src="https://your-server.com/api/v1/agent/your-tenant/widget.js"></script>
54: ```
55: 
56: ---
57: 
58: ## Integration Guide
59: 
60: ### Step 1: Get Your Tenant Slug
61: 
62: 1. Log into the Agent Admin page
63: 2. Select your tenant
64: 3. Copy the slug (e.g., `acme-corp`)
65: 
66: ### Step 2: Choose Your Embedding Method
67: 
68: #### JavaScript Embed (Recommended)
69: 
70: Best for most websites. Provides a floating button that expands into a chat panel.
71: 
72: ```html
73: <!DOCTYPE html>
74: <html>
75: <head>
76:   <title>My Website</title>
77: </head>
78: <body>
79:   <h1>Welcome to our site</h1>
80: 
81:   <!-- Chat widget with configuration -->
82:   <script
83:     src="https://your-server.com/widget/acme-corp/embed.js"
84:     data-position="bottom-right"
85:     data-color="#0d9488"
86:     data-welcome="How can we help you today?"
87:   ></script>
88: </body>
89: </html>
90: ```
91: 
92: #### iframe Embed
93: 
94: Best for CMS platforms (WordPress, Shopify, Wix) where script injection is limited.
95: 
96: ```html
97: <iframe
98:   src="https://your-server.com/widget/acme-corp/iframe?color=%230d9488&welcome=How%20can%20we%20help%3F"
99:   style="position:fixed;bottom:0;right:0;width:400px;height:600px;border:none;z-index:9999;"
100:   title="Chat Widget"
101: ></iframe>
102: ```
103: 
104: #### Advanced: Global Configuration
105: 
106: For more control, configure via JavaScript before loading the widget:
107: 
108: ```html
109: <script>
110:   window.AgentChatConfig = {
111:     baseUrl: 'https://your-server.com',
112:     tenant: 'acme-corp',
113:     color: '#0d9488',
114:     position: 'bottom-right',
115:     welcomeMessage: 'How can we help you today?',
116:     buttonSize: 56,
117:     panelWidth: 350,
118:     panelHeight: 500
119:   };
120: </script>
121: <script src="https://your-server.com/widget/acme-corp/embed.js"></script>
122: ```
123: 
124: ### Step 3: Test the Widget
125: 
126: 1. Open your page in a browser
127: 2. Click the chat button (bottom-right)
128: 3. Send a test message
129: 4. Verify the agent responds
130: 
131: ---
132: 
133: ## Configuration Options
134: 
135: ### JavaScript Embed (data-* attributes)
136: 
137: | Attribute | Type | Default | Description |
138: |-----------|------|---------|-------------|
139: | `data-position` | string | `bottom-right` | `bottom-right` or `bottom-left` |
140: | `data-color` | string | `#0d9488` | Primary color (hex) |
141: | `data-welcome` | string | `How can we help you today?` | Welcome message |
142: | `data-company-name` | string | (from tenant) | Company name in header |
143: | `data-button-size` | number | `56` | Toggle button size (px) |
144: | `data-panel-width` | number | `350` | Panel width (px) |
145: | `data-panel-height` | number | `500` | Panel height (px) |
146: 
147: ### iframe Embed (query parameters)
148: 
149: | Parameter | Type | Default | Description |
150: |-----------|------|---------|-------------|
151: | `color` | string | `#0d9488` | Primary color (URL-encoded hex) |
152: | `welcome` | string | `How can we help you today?` | Welcome message |
153: | `companyName` | string | (from tenant) | Company name in header |
154: 
155: **Note:** URL-encode special characters. `#0d9488` becomes `%230d9488`.
156: 
157: ---
158: 
159: ## API Reference
160: 
161: ### GET /widget/:slug/embed.js
162: 
163: Returns the built widget JavaScript bundle with tenant configuration injected.
164: 
165: **Parameters:**
166: | Parameter | Type | Description |
167: |-----------|------|-------------|
168: | `slug` | path | Tenant slug (e.g., `acme-corp`) |
169: 
170: **Response:**
171: - Content-Type: `application/javascript`
172: - Cache-Control: `public, max-age=3600` (1 hour)
173: 
174: **Example:**
175: ```bash
176: curl https://your-server.com/widget/acme-corp/embed.js
177: ```
178: 
179: ---
180: 
181: ### GET /widget/:slug/iframe
182: 
183: Returns a standalone HTML page for iframe embedding.
184: 
185: **Parameters:**
186: | Parameter | Type | Description |
187: |-----------|------|-------------|
188: | `slug` | path | Tenant slug |
189: | `color` | query | Primary color (URL-encoded hex) |
190: | `welcome` | query | Welcome message |
191: | `companyName` | query | Company name override |
192: 
193: **Response:**
194: - Content-Type: `text/html; charset=utf-8`
195: - Cache-Control: `public, max-age=3600` (1 hour)
196: 
197: **Example:**
198: ```bash
199: curl "https://your-server.com/widget/acme-corp/iframe?color=%230d9488"
200: ```
201: 
202: ---
203: 
204: ### GET /api/v1/agent/:slug/widget.js (Legacy)
205: 
206: Returns the legacy inline-generated widget JavaScript.
207: 
208: **Note:** This endpoint is maintained for backward compatibility. New integrations should use `/widget/:slug/embed.js`.
209: 
210: ---
211: 
212: ### POST /api/v1/agent/:slug/chat/ext
213: 
214: Sends a chat message and receives a response. **No authentication required.**
215: 
216: **Request Body:**
217: ```json
218: {
219:   "session_id": "optional-existing-session-id",
220:   "message": "Hello, I need help"
221: }
222: ```
223: 
224: | Field | Type | Required | Description |
225: |-------|------|----------|-------------|
226: | `session_id` | string | No | Existing session ID to continue conversation |
227: | `message` | string | Yes | User's message |
228: 
229: **Response (200 OK):**
230: ```json
231: {
232:   "session_id": "abc123",
233:   "message": {
234:     "role": "assistant",
235:     "content": "Hello! How can I help you today?"
236:   },
237:   "metadata": {
238:     "intent": "greeting",
239:     "confidence": 0.95
240:   }
241: }
242: ```
243: 
244: **Error Responses:**
245: 
246: | Status | Description |
247: |--------|-------------|
248: | 400 | Invalid request body or missing message |
249: | 404 | Tenant not found or inactive |
250: | 429 | Rate limit exceeded |
251: | 500 | Internal server error |
252: 
253: ---
254: 
255: ## Widget Features
256: 
257: ### User Interface
258: 
259: | Feature | Description |
260: |---------|-------------|
261: | Floating button | Circular chat icon, bottom-right corner |
262: | Chat panel | 350x500px expandable panel |
263: | Header controls | Minimize (−) and close (×) buttons |
264: | Message history | Auto-scrolling message list |
265: | Input area | Text input with send button |
266: | Welcome message | Customizable via config |
267: 
268: ### Behavior
269: 
270: | Feature | Description |
271: |---------|-------------|
272: | Toggle | Click button to open/close panel |
273: | Minimize | Click − to collapse panel (keeps button visible) |
274: | Enter to send | Press Enter to send message |
275: | Typing indicator | Shows animated dots while waiting for response |
276: | Auto-scroll | Scrolls to latest message automatically |
277: | Session persistence | Maintains conversation via `session_id` |
278: 
279: ### Styling
280: 
281: | Property | Value |
282: |----------|-------|
283: | Primary color | Configurable (default: #0d9488) |
284: | Font | System UI (system-ui, -apple-system, sans-serif) |
285: | Font size | 14px |
286: | Border radius | 12px (panel), 50% (button) |
287: | Shadow | Box shadow for depth |
288: 
289: ### Mobile Responsive
290: 
291: On screens under 480px:
292: - Panel width adjusts to nearly full screen
293: - Bottom/right margins reduced
294: - Height set to 60vh
295: 
296: ---
297: 
298: ## Customization
299: 
300: ### Using Data Attributes (Recommended)
301: 
302: ```html
303: <script
304:   src="https://your-server.com/widget/acme-corp/embed.js"
305:   data-position="bottom-left"
306:   data-color="#7c3aed"
307:   data-welcome="Hi! What can I help you with?"
308: ></script>
309: ```
310: 
311: ### CSS Overrides
312: 
313: You can override widget styles after the script loads:
314: 
315: ```html
316: <script src=".../embed.js"></script>
317: <style>
318:   /* Move to bottom-left */
319:   #acw-toggle, #acw-panel {
320:     right: auto !important;
321:     left: 20px !important;
322:   }
323: 
324:   /* Change colors */
325:   #acw-toggle, #acw-header, .acw-msg-user, #acw-send {
326:     background: #4f46e5 !important; /* Indigo */
327:   }
328: 
329:   /* Larger panel */
330:   #acw-panel {
331:     width: 400px !important;
332:     height: 600px !important;
333:   }
334: </style>
335: ```
336: 
337: ### Widget CSS Classes
338: 
339: | Selector | Element |
340: |----------|---------|
341: | `#agent-chat-widget` | Root container |
342: | `#acw-toggle` | Floating button |
343: | `#acw-panel` | Chat panel |
344: | `#acw-header` | Panel header |
345: | `#acw-content` | Content area |
346: | `#acw-messages` | Messages container |
347: | `.acw-msg` | Message wrapper |
348: | `.acw-msg-bubble` | Message bubble |
349: | `.acw-msg-user` | User message |
350: | `.acw-msg-assistant` | Agent message |
351: | `.acw-typing` | Typing indicator |
352: | `#acw-input-area` | Input container |
353: | `#acw-input` | Text input |
354: | `#acw-send` | Send button |
355: 
356: ---
357: 
358: ## Security Considerations
359: 
360: ### Authentication
361: - The widget endpoint (`/chat/ext`) requires **no authentication**
362: - Designed for anonymous public users
363: - Internal chat (`/chat/int`) requires authentication
364: 
365: ### Rate Limiting
366: - IP-based rate limiting prevents abuse
367: - Returns HTTP 429 when exceeded
368: - Default: 60-second cooldown
369: 
370: ### Session Management
371: - Sessions stored in **memory only**
372: - Sessions are **lost on server restart**
373: - No persistent conversation history for external users
374: 
375: ### XSS Protection
376: - All user input is HTML-escaped before display
377: - Uses `textContent` for safe string insertion
378: 
379: ### CORS
380: - Server handles CORS headers automatically
381: - Widget script works cross-origin
382: 
383: ### iframe Security
384: - iframe embed is isolated from parent page
385: - No access to parent page DOM or cookies
386: 
387: ---
388: 
389: ## Building the Widget
390: 
391: The widget source code is in the `widget/` folder. To build:
392: 
393: ```bash
394: # Build widget only
395: task build:widget
396: 
397: # Build everything including widget
398: task build:all
399: 
400: # Build with RAG support including widget
401: task build:rag:all
402: ```
403: 
404: ### Development
405: 
406: ```bash
407: cd widget
408: npm install
409: npm run dev     # Start dev server on port 3002
410: npm run build   # Build for production
411: ```
412: 
413: ### Widget Architecture
414: 
415: ```
416: widget/
417: ├── src/
418: │   ├── core/           # Core logic
419: │   │   ├── types.ts    # TypeScript interfaces
420: │   │   ├── api.ts      # API client
421: │   │   └── state.ts    # State management
422: │   ├── ui/             # UI components
423: │   │   ├── Widget.ts   # Main orchestrator
424: │   │   ├── Button.ts   # Toggle button
425: │   │   ├── Panel.ts    # Chat panel
426: │   │   ├── Messages.ts # Message list
427: │   │   └── Input.ts    # Input area
428: │   ├── styles/
429: │   │   └── styles.ts   # Inline CSS
430: │   └── embed.ts        # Entry point
431: ├── dist/
432: │   └── embed.min.js    # Built bundle (~15-20KB)
433: └── package.json
434: ```
435: 
436: ---
437: 
438: ## Troubleshooting
439: 
440: ### Widget Not Appearing
441: 
442: **Symptoms:** No chat button visible after adding script.
443: 
444: **Solutions:**
445: 1. Check browser console for JavaScript errors
446: 2. Verify the tenant slug is correct
447: 3. Confirm the tenant is active
448: 4. Check if another element has `z-index: 9999+` blocking it
449: 
450: ```javascript
451: // Debug: Check if widget loaded
452: console.log(document.getElementById('agent-chat-widget'));
453: console.log(window.AgentChatWidget);
454: ```
455: 
456: ### Rate Limit Errors
457: 
458: **Symptoms:** "Too many messages" error after several messages.
459: 
460: **Solutions:**
461: 1. Wait 60 seconds before sending more messages
462: 2. Contact admin to adjust rate limits if needed
463: 
464: ### CORS Errors
465: 
466: **Symptoms:** Console shows "Access-Control-Allow-Origin" errors.
467: 
468: **Solutions:**
469: 1. Ensure server is configured for CORS
470: 2. Check if using correct server URL (http vs https)
471: 3. Verify tenant exists and is active
472: 
473: ### iframe Not Loading
474: 
475: **Symptoms:** iframe shows blank or error.
476: 
477: **Solutions:**
478: 1. Check browser console for errors
479: 2. Ensure URL is correct and tenant is active
480: 3. Check for Content Security Policy (CSP) restrictions on parent page
481: 
482: ### Build Not Found
483: 
484: **Symptoms:** Fallback to legacy widget, console shows "widget embed.min.js not found".
485: 
486: **Solutions:**
487: 1. Run `task build:widget` to build the widget
488: 2. Ensure the server is running from the project root directory
489: 
490: ---
491: 
492: ## Examples
493: 
494: ### Basic JavaScript Embed
495: 
496: ```html
497: <!DOCTYPE html>
498: <html>
499: <head>
500:   <title>Customer Support</title>
501: </head>
502: <body>
503:   <h1>Contact Us</h1>
504:   <p>Click the chat button for instant support.</p>
505: 
506:   <script src="https://chat.example.com/widget/support/embed.js"></script>
507: </body>
508: </html>
509: ```
510: 
511: ### Customized JavaScript Embed
512: 
513: ```html
514: <!DOCTYPE html>
515: <html>
516: <head>
517:   <title>My Store</title>
518: </head>
519: <body>
520:   <h1>Welcome to My Store</h1>
521: 
522:   <script
523:     src="https://chat.example.com/widget/mystore/embed.js"
524:     data-position="bottom-left"
525:     data-color="#7c3aed"
526:     data-welcome="Hi! Need help finding something?"
527:   ></script>
528: </body>
529: </html>
530: ```
531: 
532: ### iframe in WordPress
533: 
534: Add this to a Custom HTML block:
535: 
536: ```html
537: <iframe
538:   src="https://chat.example.com/widget/mystore/iframe?color=%237c3aed"
539:   style="position:fixed;bottom:20px;right:20px;width:380px;height:550px;border:none;z-index:9999;border-radius:12px;box-shadow:0 8px 32px rgba(0,0,0,0.15);"
540:   title="Chat Support"
541: ></iframe>
542: ```
543: 
544: ### React Integration
545: 
546: ```jsx
547: import { useEffect } from 'react';
548: 
549: function ChatWidget({ tenantSlug, color = '#0d9488' }) {
550:   useEffect(() => {
551:     // Configure before loading
552:     window.AgentChatConfig = {
553:       color: color
554:     };
555: 
556:     const script = document.createElement('script');
557:     script.src = `https://chat.example.com/widget/${tenantSlug}/embed.js`;
558:     script.async = true;
559:     document.body.appendChild(script);
560: 
561:     return () => {
562:       // Cleanup on unmount
563:       const widget = document.getElementById('agent-chat-widget');
564:       if (widget) widget.remove();
565:       const styles = document.getElementById('acw-styles');
566:       if (styles) styles.remove();
567:       script.remove();
568:       delete window.AgentChatWidget;
569:     };
570:   }, [tenantSlug, color]);
571: 
572:   return null;
573: }
574: 
575: export default ChatWidget;
576: ```
577: 
578: ---
579: 
580: ## Related Endpoints
581: 
582: | Endpoint | Auth | Description |
583: |----------|------|-------------|
584: | `GET /widget/:slug/embed.js` | None | Built widget bundle |
585: | `GET /widget/:slug/iframe` | None | iframe HTML page |
586: | `GET /api/v1/agent/:slug/widget.js` | None | Legacy widget (inline JS) |
587: | `POST /api/v1/agent/:slug/chat/ext` | None | External chat API |
588: | `POST /api/v1/agent/:slug/chat/int` | Required | Internal chat API |
589: 
590: ---
591: 
592: *Document Version: 2.0*
593: *Last Updated: 2026-01-25*

(End of file - total 593 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/docs/KB_TEMPLATE.MD"
}
```

**Output:**
```
<path>/home/chaschel/Documents/go/bchat/docs/KB_TEMPLATE.MD</path>
<type>file</type>
<content>
1: # [Business Name] - Knowledge Base Template
2: 
3: > **AI Chat Agent Reference**  
4: > Last Updated: [YYYY-MM-DD]
5: 
6: ---
7: 
8: ## Company Overview
9: 
10: **[Business Name]** is [region]'s premier [service type] company. We provide [brief description of services] for residential and commercial properties.
11: 
12: | Attribute | Details |
13: |-----------|---------|
14: | **Company Name** | [Business Name] |
15: | **Service Type** | [e.g., Property Damage Restoration] |
16: | **Certifications** | [e.g., IICRC Certified, Licensed in State] |
17: | **Availability** | [e.g., 24/7/365 Emergency Service] |
18: 
19: ### Our Commitment
20: 
21: [Brief mission statement - 1-2 sentences about what makes this business unique.]
22: 
23: ---
24: 
25: ## Contact Information
26: 
27: | Type | Value |
28: |------|-------|
29: | **Emergency Phone** | **(555) 123-4567** |
30: | **Office Phone** | [Main office number] |
31: | **Email** | [info@business.com] |
32: | **Address** | [Street Address, City, ST ZIP] |
33: 
34: ---
35: 
36: ## Services
37: 
38: ### [Service Name]
39: <!-- @service: service_code, emergency: true, response_time: "60 minutes" -->
40: 
41: [2–4 sentences describing what this service covers, when it's needed, and what the customer can expect.]
42: 
43: **Typical causes:**
44: - Cause 1
45: - Cause 2
46: - Cause 3
47: 
48: **What's included:**
49: - What we do item 1
50: - What we do item 2
51: - What we do item 3
52: 
53: **Response time:** [e.g., "60 minutes or less for emergencies"]
54: 
55: ---
56: 
57: ### [Second Service Name]
58: <!-- @service: second_service_code, emergency: false -->
59: 
60: [Description of service]
61: 
62: ---
63: 
64: ## Services We Don't Provide
65: 
66: ### [Service Not Offered]
67: <!-- @exclusion: code, exception: "secondary to X if applicable" -->
68: 
69: We don't provide standalone [service]. [One sentence explanation + referral if available].
70: 
71: **Recommendation:** [Suggestion for alternative provider]
72: 
73: ---
74: 
75: ## Service Areas
76: 
77: ### Areas We Serve
78: <!-- @coverage: include -->
79: 
80: **Primary:** [City1], [City2], [City3], [City4]
81: 
82: **Secondary:** [City5], [City6] (additional travel fee may apply)
83: 
84: ---
85: 
86: ### Areas We Don't Serve
87: <!-- @coverage: exclude -->
88: 
89: We currently do not service:
90: - [Area 1]
91: - [Area 2]
92: 
93: ---
94: 
95: ## Frequently Asked Questions
96: 
97: ### What are your hours?
98: <!-- @faq: hours -->
99: 
100: We're open [hours]. For emergencies, our 24/7 emergency line is (555) 123-4567.
101: 
102: ---
103: 
104: ### Do you offer free estimates?
105: <!-- @faq: estimate -->
106: 
107: Yes, we provide free estimates for most services. An assessment allows us to give you accurate pricing.
108: 
109: ---
110: 
111: ### What payment methods do you accept?
112: <!-- @faq: payment -->
113: 
114: We accept [credit cards, checks, cash, financing options].
115: 
116: ---
117: 
118: ### Are you licensed and insured?
119: <!-- @faq: insurance -->
120: 
121: Yes, we are [licensing details]. We carry [insurance coverage details] and [bonding details].
122: 
123: ---
124: 
125: ### How much does [service] cost?
126: <!-- @faq: pricing -->
127: 
128: Costs vary based on [factors — size, complexity, materials, etc.]. We offer free estimates so you know the exact cost before we begin.
129: 
130: ---
131: 
132: ## Emergency Safety Procedures
133: 
134: ### Water Emergency
135: <!-- @safety: water_emergency, triggers: emergency_water, emergency_flood -->
136: 
137: **If you have active water damage:**
138: 
139: 1. Turn off the main water valve if you can safely access it
140: 2. Turn off electricity to affected areas (if safe to reach the breaker)
141: 3. Stay away from standing water near electrical outlets or appliances
142: 4. Do NOT use electrical appliances near water
143: 5. Move to a dry area and call our emergency line: **(555) 123-4567**
144: 
145: **Do NOT:**
146: - Walk through standing water if you can't see the floor
147: - Use a regular vacuum to remove water
148: - Enter rooms with sagging ceilings
149: 
150: ---
151: 
152: ## What To Expect
153: 
154: ### Step 1: Contact
155: Call, text, or use this chat. We respond within [timeframe].
156: 
157: ### Step 2: Assessment
158: We assess the situation (free for most services).
159: 
160: ### Step 3: Quote
161: You get a clear, written quote before work begins.
162: 
163: ### Step 4: Service
164: We complete the work on the agreed schedule.
165: 
166: ### Step 5: Follow-Up
167: We confirm satisfaction and document any warranty info.
168: 
169: ---
170: 
171: *[Business Name] - [Tagline]*
172: *[Certifications] | [Availability]*

(End of file - total 172 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/docs/POLICY_TEMPLATE.MD"
}
```

**Output:**
```
<path>/home/chaschel/Documents/go/bchat/docs/POLICY_TEMPLATE.MD</path>
<type>file</type>
<content>
1: # [Business Name] - Agent Policy Template
2: 
3: > **Version:** 1.0  
4: > **Last Updated:** [YYYY-MM-DD]
5: 
6: ---
7: 
8: ## Identity
9: <!-- @identity -->
10: 
11: - **Role:** Customer Service Representative for [Business Name]
12: - **Tone:** Professional, empathetic, and helpful. Calm and reassuring during emergencies.
13: - **Brand Voice:** "We're here to help you through this."
14: - **Escalation contact:** [Manager Name], (555) 123-4568, manager@business.com
15: - **Business hours:** Monday–Saturday, 7 AM – 7 PM
16: - **After-hours handling:** Collect lead, promise next-business-day callback
17: 
18: ---
19: 
20: ## Intents
21: 
22: ### Emergency
23: <!-- @intent: emergency, urgency_threshold: 75, priority: P0 -->
24: **Description:** Urgent, time-sensitive damage or safety issue.
25: 
26: **Agent behavior:**
27: - Acknowledge urgency immediately ("I understand this is urgent")
28: - Provide safety instructions if applicable
29: - Collect: name, phone, location, nature of emergency
30: - Confirm: "Our dispatch team is notified. Someone will call within [timeframe]."
31: 
32: ---
33: 
34: ### Quote Request
35: <!-- @intent: quote_request, priority: P1 -->
36: **Description:** Customer wants pricing or estimate.
37: 
38: **Agent behavior:**
39: - Acknowledge request
40: - Collect: name, contact, service needed, location, timeframe
41: - Explain pricing factors (per KB.MD)
42: - NEVER give exact figure unless listed as fixed-price in KB.MD
43: - Confirm: "We'll follow up with a detailed quote."
44: 
45: ---
46: 
47: ### Service Inquiry
48: <!-- @intent: service_inquiry, priority: P1 -->
49: **Description:** General questions about services.
50: 
51: **Agent behavior:**
52: - Answer based on KB.MD
53: - If service offered: ask if they'd like to schedule
54: - If service not offered: explain and suggest alternatives
55: 
56: ---
57: 
58: ### Escalation
59: <!-- @intent: escalation, priority: P0 -->
60: **Description:** Customer wants to speak with a human or complains.
61: 
62: **Trigger keywords:** "manager", "supervisor", "lawyer", "complain", "human", "real person"
63: 
64: **Agent behavior:**
65: - Acknowledge frustration
66: - Collect contact info and brief issue summary
67: - Confirm: "I'm connecting you with [Name]. They'll reach out within [timeframe]."
68: 
69: ---
70: 
71: ## Behavioral Rules
72: 
73: <!-- @rule: safety_first, priority: 1 -->
74: 1. **Never** make up pricing, availability, or policy.
75: 2. **Never** promise specific crew without verification.
76: 3. **Always** provide safety instructions before booking for emergencies.
77: 4. **Always** treat customers with respect, even when frustrated.
78: 5. **Never** share internal business information (costs, margins).
79: 6. **If** asked about competitors: redirect to our strengths.
80: 7. **If** asked about services we don't offer: explain and suggest alternatives.
81: 8. **Never** pretend to be human if asked "are you a bot/AI?"
82: 
83: ---
84: 
85: ## Thresholds
86: 
87: <!-- @threshold: urgency_emergency, value: 75 -->
88: 
89: | Score Range | Classification | Agent Response |
90: |-------------|----------------|--------------|
91: | 75–100 | **EMERGENCY** | Immediate acknowledgment, dispatch within response window |
92: | 40–74 | **URGENT** | Priority response within 1 hour during business hours |
93: | 1–39 | **STANDARD** | Response within normal service window |
94: | 0 | **INFORMATIONAL** | Standard informational response |
95: 
96: ---
97: 
98: ## Fallback Rules
99: 
100: <!-- @rule: fallback, priority: medium -->
101: ### When the Agent Doesn't Know
102: 
103: If the agent cannot answer a question using KB.MD or POLICY.MD:
104: 
105: 1. Say so honestly: "I don't have that information in my knowledge base."
106: 2. Offer alternative: "But I can [collect your contact info and have someone follow up]."
107: 3. Transition to lead capture — ask for name/contact and their question.
108: 
109: ---
110: 
111: ## Lead Capture Policy
112: 
113: <!-- @rule: lead_capture -->
114: ### When to Collect Contact Info
115: 
116: Collect contact info when:
117: - Visitor asks for a quote or estimate
118: - Visitor asks to schedule or book
119: - Visitor reports an emergency
120: - Conversation reaches natural close after substantive inquiry
121: 
122: **Minimum required fields:**
123: - Name OR Phone (at least one)
124: - Email OR Phone (at least one)
125: - Service/need (extracted)
126: - Location (if location-dependent)
127: 
128: ---
129: 
130: ## Human Takeover
131: 
132: <!-- @rule: human_takeover -->
133: ### When to Hand Off to a Human
134: 
135: - Visitor explicitly requests a human
136: - Urgency score = 75+ (emergency)
137: - Conversation is frustrated/escalating
138: - Question requires legal/contractual decisions
139: - Visitor says "I need to speak to someone"
140: 
141: ---
142: 
143: *End of Policy Document*

(End of file - total 143 lines)
</content>
```

---

## Assistant (Plan · MiMo V2.5 Free · 23.2s)

_Thinking:_

Now I have a clear picture of all the existing docs and templates. Let me present the final plan for DOCS_ONBOARDING.md. I know:

1. Existing templates in `docs/templates/`: KB.MD, POLICY.MD, SCRIPT_OPUS13.MD, plus prompt files
2. Existing template docs in `docs/`: KB_TEMPLATE.MD, POLICY_TEMPLATE.MD (both are fill-in-the-blank templates)
3. Existing business docs: docs_biz_guide.md (1265 lines, very comprehensive), BIZ_QUICK_START_CHECKLIST.MD (114 lines)


4. Widget documentation and simulation docs already exist with good coverage

The new DOCS_ONBOARDING.md needs to fill the gap as a practitioner's playbook—covering discovery, LLM-assisted content creation, and CMS-specific deployment workflows in concise checklists rather than exhaustive guides. I'll organize the template files to support this streamlined onboarding flow.

I've now read all the relevant existing docs. Here's the final plan — it accounts for what already exists and avoids duplication:

---

## Final Plan: DOCS_ONBOARDING.md + templates/ directory

### Existing assets to reference (not duplicate)
- `docs/docs_biz_guide.md` — 1,265 lines, very comprehensive business guide (we'll link to it, not rewrite it)
- `docs/BIZ_QUICK_START_CHECKLIST.MD` — 114-line quick checklist (we'll enhance, not replace)
- `docs/KB_TEMPLATE.MD` and `docs/POLICY_TEMPLATE.MD` — fill-in-the-blank templates (we'll create streamlined practitioner versions in `docs/templates/`)
- `docs/templates/KB.MD`, `docs/templates/POLICY.MD`, `docs/templates/SCRIPT_OPUS13.MD` — existing templates (we'll keep and add new variants)

### New files to create

#### 1. `docs/DOCS_ONBOARDING.md` (main doc, ~1200-1500 lines)

**Structure:**

```
# Onboarding Playbook

## Phase 1: Discovery (Sections 1-3)
  1. Welcome & Scope — what this doc covers, audience, goals
  2. Customer Intake — structured questionnaire (vertical, goals, CMS, timeline)
  3. Feature Selection — decision tree (RAG-only, transactional, hybrid, internal/external)

## Phase 2: Content Creation with LLMs (Sections 4-7)
  4. KB.MD Design — annotation reference, writing principles, anti-hallucination rules
  5. POLICY.MD Design — identity, intents, rules, thresholds, fallback behavior
  6. SCRIPT.MD Design — conversation flow, stage templates, multi-turn patterns
  7. LLM Prompting Recipes — copy-paste prompts for drafting each file, iterative refinement

## Phase 3: Technical Setup (Sections 8-11)
  8. Tenant Provisioning — create tenant, slug, GUID, RBAC setup
  9. RAG Configuration — when to enable, embedding providers, indexing, hybrid search
  10. Observational Memory — when to enable, token thresholds
  11. Webhook Integrations — outbound/inbound, configuration patterns

## Phase 4: Widget Deployment (Sections 12-15)
  12. Widget Embedding — universal guide (IIFE, iframe, npm)
  13. CMS-Specific Checklists — Hugo, WordPress, Shopify, Wix/Squarespace, generic HTML
  14. Widget Customization — theming, positioning, brand matching
  15. Widget Testing — local, staging, cross-browser, mobile

## Phase 5: Validation & Launch (Sections 16-18)
  16. Simulation Testing — running, interpreting, iterating
  17. Pre-Launch Checklist — security, domain verification, RAG spot-checks, rollback
  18. Go-Live & Maintenance — monitoring, re-indexing, content refresh, scaling

## Appendices
  A: Annotation Quick Reference Card
  B: Common Customer Patterns (5-6 archetypes with example configs)
  C: Troubleshooting Matrix
  D: Glossary
```

**Key distinctions from existing docs:**
- `docs_biz_guide.md` = comprehensive reference for business owners (we link to it)
- `BIZ_QUICK_START_CHECKLIST.MD` = quick checklist for self-serve setup (we enhance it)
- `DOCS_ONBOARDING.md` = **implementation specialist's playbook** — how to onboard customers using LLMs, CMS-specific deployment, validation workflows

#### 2. `docs/templates/` — new files to add

| File | Purpose |
|------|---------|
| `KB_MINIMAL.MD` | Stripped-down KB for simple businesses (service business, no emergencies) |
| `KB_ECOMMERCE.MD` | KB template for product/e-commerce verticals |
| `POLICY_MINIMAL.MD` | Minimal policy for simple FAQ-only agents |
| `POLICY_TRANSACTIONAL.MD` | Full transactional policy with booking/scheduling intents |
| `SCRIPT_MINIMAL.MD` | Simple 3-stage flow (greet → answer → close) |
| `SCRIPT_TRANSACTIONAL.MD` | Full intake flow with lead capture and qualification |

**Keep existing:** `KB.MD`, `POLICY.MD`, `SCRIPT_OPUS13.MD`, all prompt files

#### 3. Update `docs/BIZ_QUICK_START_CHECKLIST.MD`
- Add RAG setup step
- Add widget embedding method choice
- Add simulation testing step
- Keep it to ~150 lines (concise)

---

### Questions before I write:

1. **CMS checklists** — should Hugo include the managed Hugo deployment via `doctl` (Fly.io), or just the self-hosted partial approach?
2. **Customer archetypes** — should Appendix B include 5-6 specific vertical examples (plumber, restoration, e-commerce, SaaS, restaurant, HR) or keep it more generic?
3. **LLM Prompting Recipes** — should these be model-specific (GPT-4o, Claude) or generic "send this to your LLM" prompts?

---

## User

1. self-hosted partial 2. keep it generic 3. generic

---

## Assistant (Plan · MiMo V2.5 Free · 17.4s)

_Thinking:_

The user has confirmed:
1. CMS checklists: self-hosted partial only for Hugo
2. Customer archetypes: generic (not specific verticals)
3. LLM Prompting Recipes: generic prompts

I have all the information I need. The plan is complete. Let me present the final summary and ask for confirmation to proceed with writing.

All questions answered. Here's the final consolidated plan:

---

## Final Plan: DOCS_ONBOARDING.md + Template Updates

### Files to create

**1. `docs/DOCS_ONBOARDING.md`** (~1200-1500 lines)

| Phase | Sections | Content |
|-------|----------|---------|
| **Discovery** | 1-3 | Welcome & scope, Customer intake questionnaire (vertical, goals, CMS, tech comfort, timeline), Feature selection decision tree (RAG-only, transactional, hybrid, internal/external) |
| **Content Creation** | 4-7 | KB.MD design (annotations, writing principles, anti-hallucination), POLICY.MD design (identity, intents, rules, thresholds, fallback), SCRIPT.MD design (stages, multi-turn patterns), LLM prompting recipes (generic copy-paste prompts for drafting + iterative refinement) |
| **Technical Setup** | 8-11 | Tenant provisioning (slug, GUID, RBAC), RAG config (when to enable, embedding providers, indexing, hybrid search), Observational memory (when to enable, thresholds), Webhook integrations (outbound/inbound) |
| **Widget Deployment** | 12-15 | Universal embedding guide (IIFE, iframe, npm), CMS-specific checklists — Hugo (self-hosted partial only), WordPress, Shopify, Wix/Squarespace, generic HTML, Widget customization (theming, positioning, brand matching), Widget testing (local, staging, cross-browser, mobile) |
| **Validation & Launch** | 16-18 | Simulation testing (running, interpreting, iterating), Pre-launch checklist (security, domain verification, RAG spot-checks, rollback), Go-live & maintenance (monitoring, re-indexing, content refresh, scaling) |
| **Appendices** | A-D | Annotation quick reference card, Common customer patterns (generic archetypes), Troubleshooting matrix, Glossary |

**2. `docs/templates/` — 6 new template files**

| File | Purpose |
|------|---------|
| `KB_MINIMAL.MD` | Stripped-down KB for simple service businesses (no emergencies) |
| `KB_ECOMMERCE.MD` | KB template for product/e-commerce verticals |
| `POLICY_MINIMAL.MD` | Minimal policy for FAQ-only agents |
| `POLICY_TRANSACTIONAL.MD` | Full policy with booking/scheduling intents |
| `SCRIPT_MINIMAL.MD` | Simple 3-stage flow (greet → answer → close) |
| `SCRIPT_TRANSACTIONAL.MD` | Full intake flow with lead capture and qualification |

**3. Update `docs/BIZ_QUICK_START_CHECKLIST.MD`**
- Add RAG setup, widget method choice, simulation testing
- Keep to ~150 lines

### Files NOT modified
- `docs/docs_biz_guide.md` — linked from onboarding doc, not duplicated
- `docs/templates/KB.MD`, `POLICY.MD`, `SCRIPT_OPUS13.MD` — kept as-is
- All existing TUTORIAL files — untouched

### Key design decisions
- CMS checklists: concise (5-10 lines each), Hugo = self-hosted partial only
- Customer archetypes: generic patterns, not specific verticals
- LLM prompts: generic (model-agnostic), not GPT-4o/Claude-specific
- Appendices in-file (self-contained practitioner reference)

---

Ready to write. Want me to proceed?

---

## User

go

---

## Assistant (Build · MiMo V2.5 Free · 20.3s)

_Thinking:_

The user has approved the plan and wants me to proceed with writing. I'm now in build mode. Let me create a todo list and start working through the files.

Let me start by creating the todo list, then work through the files systematically.

**Tool: todowrite**

**Input:**
```json
{
  "todos": [
    {
      "content": "Create docs/templates/KB_MINIMAL.MD",
      "status": "pending",
      "priority": "medium"
    },
    {
      "content": "Create docs/templates/KB_ECOMMERCE.MD",
      "status": "pending",
      "priority": "medium"
    },
    {
      "content": "Create docs/templates/POLICY_MINIMAL.MD",
      "status": "pending",
      "priority": "medium"
    },
    {
      "content": "Create docs/templates/POLICY_TRANSACTIONAL.MD",
      "status": "pending",
      "priority": "medium"
    },
    {
      "content": "Create docs/templates/SCRIPT_MINIMAL.MD",
      "status": "pending",
      "priority": "medium"
    },
    {
      "content": "Create docs/templates/SCRIPT_TRANSACTIONAL.MD",
      "status": "pending",
      "priority": "medium"
    },
    {
      "content": "Update docs/BIZ_QUICK_START_CHECKLIST.MD",
      "status": "pending",
      "priority": "medium"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Phase 1: Discovery (Sections 1-3)",
      "status": "pending",
      "priority": "high"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Phase 2: Content Creation (Sections 4-7)",
      "status": "pending",
      "priority": "high"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Phase 3: Technical Setup (Sections 8-11)",
      "status": "pending",
      "priority": "high"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Phase 4: Widget Deployment (Sections 12-15)",
      "status": "pending",
      "priority": "high"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Phase 5: Validation & Launch (Sections 16-18)",
      "status": "pending",
      "priority": "high"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Appendices A-D",
      "status": "pending",
      "priority": "high"
    }
  ]
}
```

**Output:**
```
[
  {
    "content": "Create docs/templates/KB_MINIMAL.MD",
    "status": "pending",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/KB_ECOMMERCE.MD",
    "status": "pending",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/POLICY_MINIMAL.MD",
    "status": "pending",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/POLICY_TRANSACTIONAL.MD",
    "status": "pending",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/SCRIPT_MINIMAL.MD",
    "status": "pending",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/SCRIPT_TRANSACTIONAL.MD",
    "status": "pending",
    "priority": "medium"
  },
  {
    "content": "Update docs/BIZ_QUICK_START_CHECKLIST.MD",
    "status": "pending",
    "priority": "medium"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Phase 1: Discovery (Sections 1-3)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Phase 2: Content Creation (Sections 4-7)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Phase 3: Technical Setup (Sections 8-11)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Phase 4: Widget Deployment (Sections 12-15)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Phase 5: Validation & Launch (Sections 16-18)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Appendices A-D",
    "status": "pending",
    "priority": "high"
  }
]
```

---

## Assistant (Build · MiMo V2.5 Free · 10.3s)

_Thinking:_

Let me start with the templates first since they're quicker and can be done in parallel, then tackle the main onboarding doc.

Let me start with the templates, then the main onboarding doc. I'll create the templates in parallel.

**Tool: todowrite**

**Input:**
```json
{
  "todos": [
    {
      "content": "Create docs/templates/KB_MINIMAL.MD",
      "priority": "medium",
      "status": "in_progress"
    },
    {
      "content": "Create docs/templates/KB_ECOMMERCE.MD",
      "priority": "medium",
      "status": "in_progress"
    },
    {
      "content": "Create docs/templates/POLICY_MINIMAL.MD",
      "priority": "medium",
      "status": "in_progress"
    },
    {
      "content": "Create docs/templates/POLICY_TRANSACTIONAL.MD",
      "priority": "medium",
      "status": "in_progress"
    },
    {
      "content": "Create docs/templates/SCRIPT_MINIMAL.MD",
      "priority": "medium",
      "status": "in_progress"
    },
    {
      "content": "Create docs/templates/SCRIPT_TRANSACTIONAL.MD",
      "priority": "medium",
      "status": "in_progress"
    },
    {
      "content": "Update docs/BIZ_QUICK_START_CHECKLIST.MD",
      "priority": "medium",
      "status": "pending"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Phase 1: Discovery (Sections 1-3)",
      "priority": "high",
      "status": "pending"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Phase 2: Content Creation (Sections 4-7)",
      "priority": "high",
      "status": "pending"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Phase 3: Technical Setup (Sections 8-11)",
      "priority": "high",
      "status": "pending"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Phase 4: Widget Deployment (Sections 12-15)",
      "priority": "high",
      "status": "pending"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Phase 5: Validation & Launch (Sections 16-18)",
      "priority": "high",
      "status": "pending"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Appendices A-D",
      "priority": "high",
      "status": "pending"
    }
  ]
}
```

**Output:**
```
[
  {
    "content": "Create docs/templates/KB_MINIMAL.MD",
    "status": "in_progress",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/KB_ECOMMERCE.MD",
    "status": "in_progress",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/POLICY_MINIMAL.MD",
    "status": "in_progress",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/POLICY_TRANSACTIONAL.MD",
    "status": "in_progress",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/SCRIPT_MINIMAL.MD",
    "status": "in_progress",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/SCRIPT_TRANSACTIONAL.MD",
    "status": "in_progress",
    "priority": "medium"
  },
  {
    "content": "Update docs/BIZ_QUICK_START_CHECKLIST.MD",
    "status": "pending",
    "priority": "medium"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Phase 1: Discovery (Sections 1-3)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Phase 2: Content Creation (Sections 4-7)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Phase 3: Technical Setup (Sections 8-11)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Phase 4: Widget Deployment (Sections 12-15)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Phase 5: Validation & Launch (Sections 16-18)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Appendices A-D",
    "status": "pending",
    "priority": "high"
  }
]
```

---

## Assistant (Build · MiMo V2.5 Free · 83.4s)

_Thinking:_

Let me create all 6 template files in parallel.

**Tool: write**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/docs/templates/KB_MINIMAL.MD",
  "content": "# [Business Name] Knowledge Base\n\n> **AI Chat Agent Reference**\n> Last Updated: [Date]\n\n---\n\n## Company Overview\n\n**[Business Name]** provides [service type] in [region].\n\n| Attribute | Details |\n|-----------|---------|\n| **Company Name** | [Business Name] |\n| **Service Type** | [e.g., Landscaping, Cleaning, Consulting] |\n| **Availability** | [e.g., Monday–Friday, 8 AM – 6 PM] |\n\n---\n\n## Contact Information\n\n| Type | Value |\n|------|-------|\n| **Phone** | **(555) 123-4567** |\n| **Email** | info@business.com |\n| **Address** | [Street Address, City, ST ZIP] |\n\n---\n\n## Services\n\n### [Service Name 1]\n<!-- @service: service_1, emergency: false -->\n\n[2–3 sentences describing what this service covers and what the customer can expect.]\n\n**What's included:**\n- Item 1\n- Item 2\n- Item 3\n\n---\n\n### [Service Name 2]\n<!-- @service: service_2, emergency: false -->\n\n[Description]\n\n---\n\n## Services We Don't Provide\n\n### [Service Not Offered]\n<!-- @exclusion: excluded_service -->\n\nWe don't provide [service]. [Referral suggestion if applicable.]\n\n---\n\n## Service Areas\n\n### Areas We Serve\n<!-- @coverage: include -->\n\n**Primary:** [City1], [City2], [City3]\n\n**Secondary:** [City4], [City5] (additional travel fee may apply)\n\n---\n\n### Areas We Don't Serve\n<!-- @coverage: exclude -->\n\n- [Area 1]\n- [Area 2]\n\n---\n\n## Frequently Asked Questions\n\n### What are your hours?\n<!-- @faq: hours -->\n\nWe're open [hours]. [After-hours behavior if applicable.]\n\n---\n\n### Do you offer free estimates?\n<!-- @faq: estimate -->\n\n[Yes/No + details]\n\n---\n\n### What payment methods do you accept?\n<!-- @faq: payment -->\n\nWe accept [credit cards, checks, cash, financing].\n\n---\n\n### How much does [service] cost?\n<!-- @faq: pricing -->\n\nCosts vary based on [factors]. We offer free estimates so you know the exact cost before we begin.\n\n---\n\n## What To Expect\n\n1. **Contact** — Call, text, or chat. We respond within [timeframe].\n2. **Assessment** — We evaluate your needs (free for most services).\n3. **Quote** — You get a clear quote before work begins.\n4. **Service** — We complete the work on schedule.\n5. **Follow-Up** — We confirm satisfaction.\n\n---\n\n*[Business Name] — [Tagline]*\n"
}
```

**Output:**
```
Wrote file successfully.
```

**Tool: write**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/docs/templates/KB_ECOMMERCE.MD",
  "content": "# [Business Name] Knowledge Base\n\n> **AI Chat Agent Reference**\n> Last Updated: [Date]\n\n---\n\n## Company Overview\n\n**[Business Name]** is an online retailer specializing in [product category].\n\n| Attribute | Details |\n|-----------|---------|\n| **Company Name** | [Business Name] |\n| **Product Category** | [e.g., Outdoor Gear, Home Decor, Pet Supplies] |\n| **Shipping** | [e.g., Free shipping on orders over $50] |\n| **Return Policy** | [e.g., 30-day returns] |\n\n---\n\n## Contact Information\n\n| Type | Value |\n|------|-------|\n| **Phone** | **(555) 123-4567** |\n| **Email** | support@business.com |\n| **Hours** | [e.g., Monday–Friday, 9 AM – 5 PM] |\n\n---\n\n## Products\n\n### [Product Category 1]\n<!-- @service: category_1, emergency: false -->\n\n[Description of product line, key features, and what makes it special.]\n\n**Popular items:**\n- [Product A] — [brief description]\n- [Product B] — [brief description]\n- [Product C] — [brief description]\n\n**Price range:** $[low]–$[high]\n\n---\n\n### [Product Category 2]\n<!-- @service: category_2, emergency: false -->\n\n[Description]\n\n---\n\n## Services We Don't Offer\n\n### [Service Not Offered]\n<!-- @exclusion: excluded_service -->\n\nWe don't provide [service]. [Alternative suggestion if applicable.]\n\n---\n\n## Shipping & Delivery\n\n<!-- @coverage: include -->\n\n### Shipping Options\n\n| Method | Timeframe | Cost |\n|--------|-----------|------|\n| Standard | 5–7 business days | $[X] or free over $[Y] |\n| Express | 2–3 business days | $[X] |\n| Overnight | Next business day | $[X] |\n\n### Shipping Areas\n\n**We ship to:** [Countries/states]\n\n**We do not ship to:** [Restricted areas]\n\n---\n\n## Frequently Asked Questions\n\n### How do I track my order?\n<!-- @faq: tracking -->\n\nAfter your order ships, you'll receive an email with a tracking number. You can also check your order status in your account dashboard.\n\n---\n\n### What is your return policy?\n<!-- @faq: returns -->\n\n[Return policy details — timeframe, conditions, process.]\n\n---\n\n### Do you offer international shipping?\n<!-- @faq: international -->\n\n[Yes/No + details about countries, duties, costs.]\n\n---\n\n### How do I contact customer support?\n<!-- @faq: contact -->\n\nYou can reach us by phone at (555) 123-4567, email at support@business.com, or through this chat. Our hours are [hours].\n\n---\n\n### Are your products [warranty/guarantee]?\n<!-- @faq: warranty -->\n\n[Warranty/guarantee details]\n\n---\n\n### Do you offer bulk/wholesale pricing?\n<!-- @faq: bulk -->\n\n[Yes/No + minimum order requirements, discount structure.]\n\n---\n\n## Returns Process\n\n1. **Initiate** — Contact us within [timeframe] of delivery\n2. **Get RMA** — We'll provide a return authorization and instructions\n3. **Ship** — Send the item back in original packaging\n4. **Refund** — Refund processed within [timeframe] of receiving the return\n\n---\n\n*[Business Name] — [Tagline]*\n"
}
```

**Output:**
```
Wrote file successfully.
```

**Tool: write**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/docs/templates/POLICY_MINIMAL.MD",
  "content": "# [Business Name] Agent Policy\n\n> **Version:** 1.0\n> **Last Updated:** [Date]\n\n---\n\n## Identity\n<!-- @identity -->\n\n- **Role:** Customer Service Representative for [Business Name]\n- **Tone:** Professional, friendly, and helpful\n- **Brand Voice:** \"We're here to help.\"\n- **Business hours:** [e.g., Monday–Friday, 8 AM – 6 PM]\n- **After-hours handling:** Collect contact info, promise next-business-day callback\n\n---\n\n## Intents\n\n### Service Inquiry\n<!-- @intent: service_inquiry, category: standard, urgency: 1, action: standard_flow -->\n\nGeneral questions about services, products, or process.\n\n**Agent behavior:**\n- Answer based on KB.MD\n- If service offered: ask if they'd like to schedule or learn more\n- If service not offered: explain and suggest alternatives\n\n---\n\n### Booking Request\n<!-- @intent: booking_request, category: standard, urgency: 2, action: booking_flow -->\n\nCustomer wants to schedule or book a service.\n\n**Agent behavior:**\n- Collect: name, contact, service needed, preferred date/time\n- Confirm: \"We'll follow up to confirm your appointment.\"\n\n---\n\n### Pricing Question\n<!-- @intent: pricing_question, category: standard, urgency: 1, action: standard_flow -->\n\nQuestions about cost, estimates, or payment.\n\n**Agent behavior:**\n- Reference KB.MD pricing section\n- Never quote exact prices unless listed as fixed in KB.MD\n- Offer free estimate if available\n\n---\n\n### Escalation\n<!-- @intent: escalation, category: meta, urgency: 0, action: escalation_flow -->\n\nCustomer wants to speak with a human or complains.\n\n**Trigger keywords:** \"manager\", \"supervisor\", \"human\", \"real person\", \"complain\"\n\n**Agent behavior:**\n- Acknowledge their concern\n- Collect contact info and brief summary\n- Confirm: \"I'm connecting you with our team. They'll reach out within [timeframe].\"\n\n---\n\n## Rules\n\n### Safety First\n<!-- @rule: safety_first, priority: 1 -->\n\nIn any emergency situation, provide safety instructions before collecting contact info or discussing services.\n\n---\n\n### No Price Guessing\n<!-- @rule: no_price_guessing, priority: 2 -->\n\nNever estimate costs or speculate about pricing. Reference KB.MD or offer an estimate.\n\n---\n\n### Stay In Scope\n<!-- @rule: stay_in_scope, priority: 3 -->\n\nOnly discuss services explicitly defined in KB.MD. For out-of-scope requests, politely explain and offer alternatives.\n\n---\n\n### Empathy First\n<!-- @rule: empathy_first, priority: 4 -->\n\nAcknowledge stressful situations with empathy before moving to solutions.\n\n---\n\n## Thresholds\n<!-- @thresholds -->\n\n| Metric | Value | Description |\n|--------|-------|-------------|\n| Emergency Urgency | >= 4 | Route to emergency flow |\n| Escalation Confidence | >= 0.85 | Confirm escalation intent |\n| Service Match Confidence | >= 0.70 | Suggest matched service |\n\n---\n\n## Fallback Rules\n<!-- @rule: fallback, priority: medium -->\n\n### When the Agent Doesn't Know\n\n1. Say so honestly: \"I don't have that information.\"\n2. Offer alternative: \"I can collect your info and have someone follow up.\"\n3. Transition to lead capture.\n\n---\n\n*End of Policy Document*\n"
}
```

**Output:**
```
Wrote file successfully.
```

**Tool: write**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/docs/templates/POLICY_TRANSACTIONAL.MD",
  "content": "# [Business Name] Agent Policy\n\n> **Version:** 1.0\n> **Last Updated:** [Date]\n\n---\n\n## Identity\n<!-- @identity -->\n\n- **Role:** Customer Service Representative for [Business Name]\n- **Tone:** Professional, empathetic, and helpful. Calm and reassuring during emergencies.\n- **Brand Voice:** \"We're here to help you through this.\"\n- **Escalation contact:** [Manager Name], (555) 123-4568\n- **Business hours:** [e.g., Monday–Saturday, 7 AM – 7 PM]\n- **After-hours handling:** Collect lead, promise next-business-day callback\n\n---\n\n## Intents\n\n### Emergency\n<!-- @intent: emergency, category: emergency, urgency: 5, action: emergency_flow -->\n\nUrgent, time-sensitive damage or safety issue.\n\n**Agent behavior:**\n- Acknowledge urgency immediately\n- Provide safety instructions if applicable\n- Collect: name, phone, location, nature of emergency\n- Confirm: \"Our team is notified. Someone will call within [timeframe].\"\n\n---\n\n### Quote Request\n<!-- @intent: quote_request, category: standard, urgency: 2, action: booking_flow -->\n\nCustomer wants pricing or an estimate.\n\n**Agent behavior:**\n- Acknowledge request\n- Collect: name, contact, service needed, location, timeframe\n- Explain pricing factors (per KB.MD)\n- Never give exact figure unless listed as fixed-price in KB.MD\n- Confirm: \"We'll follow up with a detailed quote.\"\n\n---\n\n### Booking Request\n<!-- @intent: booking_request, category: standard, urgency: 2, action: booking_flow -->\n\nCustomer wants to schedule a service.\n\n**Agent behavior:**\n- Collect: name, contact, service needed, preferred date/time\n- Confirm: \"We'll follow up to confirm your appointment.\"\n\n---\n\n### Service Inquiry\n<!-- @intent: service_inquiry, category: standard, urgency: 1, action: standard_flow -->\n\nGeneral questions about services, capabilities, or process.\n\n**Agent behavior:**\n- Answer based on KB.MD\n- If service offered: ask if they'd like to schedule\n- If service not offered: explain and suggest alternatives\n\n---\n\n### Coverage Question\n<!-- @intent: coverage_question, category: standard, urgency: 1, action: standard_flow -->\n\nQuestions about service area or whether we serve their location.\n\n**Agent behavior:**\n- Reference KB.MD coverage section\n- If in area: confirm and continue\n- If out of area: politely decline, suggest local alternatives\n\n---\n\n### Escalation\n<!-- @intent: escalation, category: meta, urgency: 0, action: escalation_flow, confidence_threshold: 0.85 -->\n\nCustomer wants to speak with a human or complains.\n\n**Trigger keywords:** \"manager\", \"supervisor\", \"lawyer\", \"complain\", \"human\", \"real person\"\n\n**Agent behavior:**\n- Acknowledge frustration\n- Collect contact info and brief summary\n- Confirm: \"I'm connecting you with [Name]. They'll reach out within [timeframe].\"\n\n---\n\n### Greeting\n<!-- @intent: greeting, category: meta, urgency: 0, action: greeting_flow -->\n\nSimple greeting or conversation starter.\n\n**Agent behavior:**\n- Respond warmly\n- Introduce yourself and the business\n- Ask how you can help\n\n---\n\n### Farewell\n<!-- @intent: farewell, category: meta, urgency: 0, action: farewell_flow -->\n\nCustomer is ending the conversation.\n\n**Agent behavior:**\n- Acknowledge and thank them\n- Confirm any next steps\n- Invite them to reach out again\n\n---\n\n## Rules\n\n### Safety First\n<!-- @rule: safety_first, priority: 1 -->\n\nIn any emergency situation, provide safety instructions BEFORE collecting contact info or discussing services. Never skip safety advice to get to booking faster.\n\n---\n\n### No Price Guessing\n<!-- @rule: no_price_guessing, priority: 2 -->\n\nNever estimate costs, give price ranges, or speculate about pricing. Always say: \"Every situation is unique. We provide free estimates after a technician assesses the situation.\"\n\n---\n\n### No Repetition\n<!-- @rule: no_repetition, priority: 3 -->\n\nIf you've already provided key information (phone number, safety instructions), don't repeat it. Acknowledge the new question, answer it, then briefly remind them to call if urgent.\n\n---\n\n### Remember Customer Info\n<!-- @rule: remember_info, priority: 4 -->\n\nIf the customer has already provided their name, phone number, or location, do NOT ask for it again. Reference what you already know.\n\n---\n\n### Stay In Scope\n<!-- @rule: stay_in_scope, priority: 5 -->\n\nOnly discuss services explicitly defined in KB.MD. For out-of-scope requests, politely explain and offer alternatives.\n\n---\n\n### Empathy First\n<!-- @rule: empathy_first, priority: 6 -->\n\nWhen a customer describes a stressful situation, acknowledge it with empathy before moving to solutions. Example: \"I understand how stressful this must be.\"\n\n---\n\n## Thresholds\n<!-- @thresholds -->\n\n| Metric | Value | Description |\n|--------|-------|-------------|\n| Emergency Urgency | >= 4 | Route to emergency flow |\n| Escalation Confidence | >= 0.85 | Confirm escalation intent |\n| Service Match Confidence | >= 0.70 | Suggest matched service |\n| Location Match Confidence | >= 0.75 | Confirm coverage area |\n\n---\n\n## Response Guidelines\n\n### Emergency Response Structure\n\n1. **Acknowledge** — Show you understand the urgency\n2. **Safety** — Provide immediate safety instructions\n3. **Action** — Direct to emergency phone number\n4. **Reassure** — Let them know help is available\n\n---\n\n### Standard Response Structure\n\n1. **Acknowledge** — Confirm what they're asking\n2. **Answer** — Provide helpful information\n3. **Next Step** — Offer to help further or schedule service\n\n---\n\n### Out of Area Response\n\n> \"Thank you for reaching out. Unfortunately, we currently only service [our service areas]. For [their location], I recommend searching for local [service type] companies. Is there anything else I can help you with?\"\n\n---\n\n## Fallback Rules\n<!-- @rule: fallback, priority: medium -->\n\n### When the Agent Doesn't Know\n\n1. Say so honestly: \"I don't have that information in my knowledge base.\"\n2. Offer alternative: \"I can collect your contact info and have someone follow up.\"\n3. Transition to lead capture.\n\n---\n\n## Lead Capture Policy\n<!-- @rule: lead_capture -->\n\n### When to Collect Contact Info\n\nCollect contact info when:\n- Visitor asks for a quote or estimate\n- Visitor asks to schedule or book\n- Visitor reports an emergency\n- Conversation reaches natural close after substantive inquiry\n\n**Do NOT aggressively collect on:**\n- First message before understanding the need\n- Simple \"hi\" or \"hello\"\n- Simple FAQ resolved in KB.MD\n\n**Minimum required fields:**\n- Name OR Phone (at least one)\n- Email OR Phone (at least one)\n- Service/need (extracted)\n- Location (if location-dependent)\n\n---\n\n## Human Takeover\n<!-- @rule: human_takeover -->\n\n### When to Hand Off to a Human\n\n- Visitor explicitly requests a human\n- Urgency score = 4+ (emergency)\n- Conversation is frustrated/escalating\n- Question requires legal/contractual decisions\n\n### After Human Takeover\n\n- Show transition message: \"I'm connecting you with [Name]. They'll respond within [timeframe].\"\n- Create internal notification\n- Do NOT continue resolving autonomously\n\n---\n\n*End of Policy Document*\n"
}
```

**Output:**
```
Wrote file successfully.
```

**Tool: write**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/docs/templates/SCRIPT_MINIMAL.MD",
  "content": "## Stage: Opening\n\n- Greet the visitor warmly\n- State who you represent: \"I'm the [Business Name] assistant\"\n- Ask how you can help today\n- DO NOT collect contact info yet — understand the need first\n\n---\n\n## Stage: Discovery\n\n- Ask clarifying questions to understand the service or product needed\n- Confirm location or service area if relevant\n- Determine urgency (if applicable)\n\n---\n\n## Stage: Information\n\n- Answer questions using KB.MD\n- If answer not in KB.MD → use fallback rule\n- Be specific, not vague\n\n---\n\n## Stage: Qualification\n\n- Once the visitor shows intent (wants to book, get a quote, etc.):\n  - Collect: name, email/phone, service needed, preferred time\n- Confirm: \"I have your details — [Name] will follow up within [timeframe]\"\n\n---\n\n## Stage: Closing\n\n- Confirm next step\n- Thank the visitor\n- Invite them to reach out again\n\n---\n\n## Notes\n\n- The agent should never ask for contact details before understanding the need\n- Conversation should feel natural: help first, collect second\n- If visitor says \"I'd rather not share\" → accept gracefully, continue helping\n"
}
```

**Output:**
```
Wrote file successfully.
```

**Tool: write**

**Input:**
```json
{
  "filePath": "/home/chaschel/Documents/go/bchat/docs/templates/SCRIPT_TRANSACTIONAL.MD",
  "content": "## PRIORITY 0: EMERGENCY + COVERAGE CHECK\n\nThis is a **unified check** performed on every incoming message BEFORE any other processing.\n\n### STEP 1: Score for Emergency\n\n| Factor | Weight | High Score Indicators |\n|--------|:------:|----------------------|\n| Time Urgency | 25% | \"right now\", \"currently\", \"happening\" |\n| Active Situation | 25% | \"is flooding\", \"coming in\", \"spreading\" |\n| Safety Risk | 20% | Vulnerable people, health symptoms |\n| Damage Severity | 20% | Fire > water > structural |\n| Distress Language | 10% | \"help!\", \"what do I do?\" |\n\n**Scoring:**\n- >= 60%: EMERGENCY\n- 40–59%: URGENT (clarify if happening now)\n- < 40%: STANDARD\n\n---\n\n### STEP 2: Check Coverage Area\n\n**IN-AREA indicators:**\n- Towns: [List primary service area towns]\n- Counties: [List counties]\n- ZIP codes: [List ZIP prefixes]\n\n**OUT-OF-AREA indicators:**\n- [List excluded areas]\n\n**UNKNOWN:** No location mentioned\n\n---\n\n### STEP 3: Route Based on Matrix\n\n| Emergency | Coverage | Action |\n|:---------:|:--------:|--------|\n| YES | IN | Full emergency response |\n| YES | OUT | Safety tips → explain limitation → suggest local |\n| YES | UNKNOWN | Safety tips → ask location |\n| URGENT | IN | Express concern, expedite intake |\n| URGENT | OUT | Clarify urgency, then explain limitation |\n| URGENT | UNKNOWN | \"Is this happening now? What's your location?\" |\n| NO | IN | Standard intake flow |\n| NO | OUT | Polite decline |\n| NO | UNKNOWN | Standard intake, verify in CONTACT section |\n\n---\n\n## EMERGENCY RESPONSES\n\n### Response A: EMERGENCY + IN-AREA\n\n> Example: \"My basement is flooding right now!\"\n\n\"This is an emergency. Since you're in [LOCATION], we can dispatch a team within [TIMEFRAME].\n\nCall now: **(555) 123-4567**\n\nWhile you call:\n- Turn off water source if safe\n- Turn off electricity to affected areas\n- Move away from standing water\n\nOur team is standing by for you.\"\n\n---\n\n### Response B: EMERGENCY + OUT-OF-AREA\n\n\"I can hear this is urgent. First, safety tips:\n- Turn off water and electricity if safe\n- Move away from standing water\n- Call 911 if there's immediate danger\n\nUnfortunately, we only service [SERVICE AREAS] — we can't dispatch to [LOCATION].\n\nFor immediate help:\n- Call 911 for emergencies\n- Search for local [service type] companies in your area\n\nStay safe!\"\n\n---\n\n### Response C: EMERGENCY + LOCATION UNKNOWN\n\n\"This sounds like an emergency. Safety tips right now:\n- Turn off water if you can safely reach the valve\n- Turn off electricity to affected areas\n- Move away from standing water\n\nWhat's your location? If you're in our service area, we can have a team there in [TIMEFRAME].\n\nIf you know you're in our area, call now: **(555) 123-4567**\"\n\n---\n\n### Response D: URGENT (40–59%) + ANY LOCATION\n\n\"This sounds concerning. Is this actively happening right now, or did it occur earlier?\n\nAlso, what's the property address? If you're in our service area, we may need to get a team out quickly.\"\n\n---\n\n## NON-EMERGENCY RESPONSES\n\n### Response E: NON-EMERGENCY + OUT-OF-AREA\n\n\"Thanks for reaching out! Unfortunately, we only service [SERVICE AREAS]. For [LOCATION], I'd recommend searching for local [service type] companies in your area.\"\n\n### Response F: NON-EMERGENCY + IN-AREA or UNKNOWN\n\n→ Proceed to BUSINESS INTRODUCTION\n\n---\n\n## BUSINESS INTRODUCTION\n\nAfter Priority 0 passes (non-emergency, in-area or unknown):\n\n> If customer states an issue:\n\"I'm sorry to hear about that. Here at [Business Name], we're [certifications] and available [availability] — we can definitely help. Let me get some details.\"\n\n> If greeting only:\n\"Hello! Welcome to [Business Name]. We're [certifications], available [availability], with [response time] emergency response. How can I help?\"\n\n---\n\n## CUSTOMER TYPE IDENTIFICATION\n\nInfer from context. Don't ask \"worked with us before?\" first.\n\n- **New:** No prior booking → Address issue\n- **Repeat:** Mentions \"my project\", tech names → Welcome back\n- **Ambiguous:** \"Existing project or new situation?\"\n\n---\n\n## SERVICE INTAKE\n\n**Phase 1 Rules:**\n- Address stated issue, collect name/address/service\n- NO \"anything else?\" in Phase 1\n- Batch questions, don't re-ask\n\n**Questions:**\n1. Service type (if not stated)\n2. \"When, and is source contained?\"\n3. Severity, rooms/areas affected\n4. Safety concerns\n5. Insurance (if applicable)\n\n→ Go to CONTACT INFORMATION\n\n---\n\n## CONTACT INFORMATION\n\nSkip if already provided: Name, Phone, Address, County/area verification, Property type, Timing, On-site availability\n\n---\n\n## CONFIRMATION\n\n\"Got it, [NAME]. Confirming: [NAME] | [PHONE] | [ADDRESS] | [SERVICE] | [TIMING]. Correct?\"\n\n---\n\n## CONCLUSION\n\n**Phase 2 — \"Anything else?\" appropriate**\n\n\"You're all set, [NAME]!\n1. Scheduling confirms\n2. [Team member] calls before arriving\n3. On-site assessment\n4. Estimate provided\n5. Work begins\n\nCall: **(555) 123-4567**\n\nIs there anything else?\"\n\n---\n\n## CLOSING\n\n**No Escape Rule:** Answer follow-ups. Wait for explicit \"no\" or \"I'm good.\"\n\n---\n\n## SPECIAL SCENARIOS\n\n- **Issue in first message:** Acknowledge, don't ask \"worked with us?\"\n- **Upset customer:** Deep empathy, offer manager escalation\n- **Pricing:** Reference KB.MD or offer free estimate\n- **Insurance:** \"You can choose your own provider.\"\n\n---\n\n## PROHIBITED LANGUAGE\n\n- \"Updated successfully\"\n- \"Please disregard\"\n\nUse instead:\n- \"Got it\"\n- \"Perfect\"\n"
}
```

**Output:**
```
Wrote file successfully.
```

---

## Assistant (Build · MiMo V2.5 Free · 11.9s)

_Thinking:_

All 6 templates created. Now let me update the todo list and move on to the checklist update and the main onboarding doc.

All 6 templates created. Now updating the checklist and starting the main onboarding doc.

**Tool: todowrite**

**Input:**
```json
{
  "todos": [
    {
      "content": "Create docs/templates/KB_MINIMAL.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/templates/KB_ECOMMERCE.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/templates/POLICY_MINIMAL.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/templates/POLICY_TRANSACTIONAL.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/templates/SCRIPT_MINIMAL.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/templates/SCRIPT_TRANSACTIONAL.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Update docs/BIZ_QUICK_START_CHECKLIST.MD",
      "priority": "medium",
      "status": "in_progress"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Phase 1: Discovery (Sections 1-3)",
      "priority": "high",
      "status": "pending"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Phase 2: Content Creation (Sections 4-7)",
      "priority": "high",
      "status": "pending"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Phase 3: Technical Setup (Sections 8-11)",
      "priority": "high",
      "status": "pending"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Phase 4: Widget Deployment (Sections 12-15)",
      "priority": "high",
      "status": "pending"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Phase 5: Validation & Launch (Sections 16-18)",
      "priority": "high",
      "status": "pending"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — Appendices A-D",
      "priority": "high",
      "status": "pending"
    }
  ]
}
```

**Output:**
```
[
  {
    "content": "Create docs/templates/KB_MINIMAL.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/KB_ECOMMERCE.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/POLICY_MINIMAL.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/POLICY_TRANSACTIONAL.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/SCRIPT_MINIMAL.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/SCRIPT_TRANSACTIONAL.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Update docs/BIZ_QUICK_START_CHECKLIST.MD",
    "status": "in_progress",
    "priority": "medium"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Phase 1: Discovery (Sections 1-3)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Phase 2: Content Creation (Sections 4-7)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Phase 3: Technical Setup (Sections 8-11)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Phase 4: Widget Deployment (Sections 12-15)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Phase 5: Validation & Launch (Sections 16-18)",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — Appendices A-D",
    "status": "pending",
    "priority": "high"
  }
]
```

---

## Assistant (Build · MiMo V2.5 Free · 24.5s)

_Thinking:_

Now let me update the BIZ_QUICK_START_CHECKLIST.MD and start the main onboarding doc. Let me do both in parallel.

**Tool: write**

**Input:**
```json
{
  "content": "# Business Owner Quick Start Checklist\n\n> **Printing this? Keep it handy during setup.**\n\n---\n\n## Phase 1: Technical Setup\n\n- [ ] Clone bchat repository\n- [ ] Install prerequisites (Go 1.21+, Node.js 18+, Task)\n- [ ] Get OpenRouter API key\n- [ ] Set up server (Fly.io, VPS, or local machine)\n- [ ] Configure environment variables\n- [ ] Build and run the application\n- [ ] Verify admin panel loads at `/agent-admin`\n\n---\n\n## Phase 2: Tenant Creation\n\n- [ ] Create tenant in admin panel\n- [ ] Note the **slug** (URL-safe identifier)\n- [ ] Set LLM model (recommended: `gpt-4o-mini`)\n- [ ] Set temperature (recommended: `0.3–0.5` for service businesses)\n- [ ] Configure allowed domains (if needed)\n- [ ] Toggle \"Is Active\" to Yes\n\n---\n\n## Phase 3: Knowledge Base Setup\n\n- [ ] Create KB.MD using template (`docs/templates/KB_MINIMAL.MD` for simple businesses, `docs/templates/KB.MD` for emergency services)\n- [ ] Include ALL services offered (with `@service` annotations)\n- [ ] Include ALL services NOT offered (with `@exclusion` annotations)\n- [ ] List complete service areas with `@coverage: include/exclude`\n- [ ] Add emergency protocol with after-hours contact (if applicable)\n- [ ] Include pricing guidance (structure, not fixed prices)\n- [ ] Add 6+ FAQs: hours, estimates, payment, insurance, scheduling, cancellation\n- [ ] Add safety procedures for emergency services (if applicable)\n- [ ] Remove ALL placeholder text (`[Your Company Name]`, etc.)\n- [ ] Review KB.MD with business owner for accuracy\n\n---\n\n## Phase 4: Policy Setup\n\n- [ ] Create POLICY.MD using template (`docs/templates/POLICY_MINIMAL.MD` for FAQ-only, `docs/templates/POLICY_TRANSACTIONAL.MD` for full booking flow)\n- [ ] Define agent identity and tone\n- [ ] Set escalation trigger keywords\n- [ ] Configure urgency thresholds\n- [ ] Define lead capture rules\n- [ ] Set \"Say Unknown + Capture\" behavior\n- [ ] Define after-hours handling\n\n---\n\n## Phase 5: RAG Setup\n\n- [ ] Enable RAG: set `RAG_PIPELINE_ENABLED=true` in environment\n- [ ] Configure embedding provider: `EMBEDDING_PROVIDER=openrouter`\n- [ ] Build with RAG: `task build:rag`\n- [ ] Upload KB.MD, POLICY.MD, SCRIPT.MD to tenant\n- [ ] Click **Rebuild Index** after uploading files\n- [ ] Verify RAG debug returns correct chunks for test queries\n\n---\n\n## Phase 6: Widget Deployment\n\n- [ ] Choose embedding method:\n  - **IIFE (recommended)** — `<script src=\"https://your-server.com/widget/[slug]/embed.js\">`\n  - **iframe** — for WordPress, Shopify, Wix (low-trust CMS)\n  - **npm** — for React apps\n- [ ] Add embed code to website (before `</body>`)\n- [ ] Test widget loads on page\n- [ ] Verify widget styling matches brand (color, position, welcome message)\n- [ ] Test on mobile device\n- [ ] Verify allowed domains configured in tenant settings\n\n---\n\n## Phase 7: Testing (Critical!)\n\n- [ ] Run 3+ simulations with different personas:\n  - Emergency scenario (if applicable)\n  - Quote request scenario\n  - General inquiry scenario\n- [ ] Test RAG debug with 10+ queries\n- [ ] Verify lead capture extracts name/email/phone\n- [ ] Test escalation flow (\"I want to speak to a manager\")\n- [ ] Check \"Say Unknown\" behavior works\n- [ ] Verify conversation replay works in admin\n\n---\n\n## Phase 8: Go Live\n\n- [ ] Confirm business owner has admin access\n- [ ] Show business owner how to view leads\n- [ ] Set up lead notification (email/SMS bridge)\n- [ ] Train on daily lead review process\n- [ ] Schedule follow-up for agent optimization\n\n---\n\n## Quick Reference\n\n**Admin URL:** `https://your-server.com/agent-admin`\n\n**Embed Script (IIFE):**\n```html\n<script\n  src=\"https://your-server.com/widget/[slug]/embed.js\"\n  data-position=\"bottom-right\"\n  data-color=\"#0d9488\"\n></script>\n```\n\n**Embed Script (iframe — for CMS):**\n```html\n<iframe\n  src=\"https://your-server.com/widget/[slug]/iframe?color=%230d9488\"\n  style=\"position:fixed;bottom:0;right:0;width:400px;height:600px;border:none;z-index:9999;\"\n  title=\"Chat Widget\"\n></iframe>\n```\n\n**Critical Buttons:**\n- **Create Tenant** → Sets up new business\n- **Upload Files** → KB.MD, POLICY.MD, SCRIPT.MD\n- **Rebuild Index** → Must click after every file upload!\n- **Simulations** → Test before going live\n\n**Lead Statuses:**\n- `new` → Fresh lead, not yet contacted\n- `contacted` → Someone reached out\n- `converted` → Became a customer\n- `lost` → Not interested or went elsewhere\n\n**Configuration Priority:**\n1. Tenant Config (Agent Admin UI) → Highest\n2. Environment Variable → Fallback\n3. Hardcoded Default → Lowest\n",
  "filePath": "/home/chaschel/Documents/go/bchat/docs/BIZ_QUICK_START_CHECKLIST.MD"
}
```

**Output:**
```
Wrote file successfully.
```

---

## Assistant (Build · MiMo V2.5 Free · 8.9s)

_Thinking:_

Good, the checklist is updated. Now let me start the main onboarding doc. This is a large document, so I'll write it in one go since it's all interconnected. Let me plan the structure carefully.

The document needs to be comprehensive but concise. Let me write it section by section. Given the size (~1200-1500 lines), I'll write the full document at once.

Now starting the main onboarding doc. This is the largest file — I'll write it in full.

**Tool: todowrite**

**Input:**
```json
{
  "todos": [
    {
      "content": "Create docs/templates/KB_MINIMAL.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/templates/KB_ECOMMERCE.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/templates/POLICY_MINIMAL.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/templates/POLICY_TRANSACTIONAL.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/templates/SCRIPT_MINIMAL.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/templates/SCRIPT_TRANSACTIONAL.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Update docs/BIZ_QUICK_START_CHECKLIST.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — All phases and appendices",
      "priority": "high",
      "status": "in_progress"
    }
  ]
}
```

**Output:**
```
[
  {
    "content": "Create docs/templates/KB_MINIMAL.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/KB_ECOMMERCE.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/POLICY_MINIMAL.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/POLICY_TRANSACTIONAL.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/SCRIPT_MINIMAL.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/SCRIPT_TRANSACTIONAL.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Update docs/BIZ_QUICK_START_CHECKLIST.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — All phases and appendices",
    "status": "in_progress",
    "priority": "high"
  }
]
```

---

## Assistant (Build · MiMo V2.5 Free · 152.9s)

_Thinking:_

Now let me write the full DOCS_ONBOARDING.md. This is a large document, so I need to be thorough but concise. Let me structure it carefully.

**Tool: write**

**Input:**
```json
{
  "content": "# Onboarding Playbook\n\n> **Audience:** Implementation specialists — product people with technical skills who onboard new customers onto bchat.\n>\n> **Purpose:** End-to-end guide for taking a customer from first call to live AI agent. Covers discovery, content creation with LLM assistance, technical setup, widget deployment, validation, and ongoing maintenance.\n>\n> **Estimated time per customer:** 4–8 hours (first time), 2–3 hours (experienced)\n\n---\n\n## Table of Contents\n\n1. [Welcome & Scope](#1-welcome--scope)\n2. [Customer Intake](#2-customer-intake)\n3. [Feature Selection](#3-feature-selection)\n4. [KB.MD Design](#4-kbmd-design)\n5. [POLICY.MD Design](#5-policymd-design)\n6. [SCRIPT.MD Design](#6-scriptmd-design)\n7. [LLM Prompting Recipes](#7-llm-prompting-recipes)\n8. [Tenant Provisioning](#8-tenant-provisioning)\n9. [RAG Configuration](#9-rag-configuration)\n10. [Observational Memory](#10-observational-memory)\n11. [Webhook Integrations](#11-webhook-integrations)\n12. [Widget Embedding](#12-widget-embedding)\n13. [CMS-Specific Checklists](#13-cms-specific-checklists)\n14. [Widget Customization](#14-widget-customization)\n15. [Widget Testing](#15-widget-testing)\n16. [Simulation Testing](#16-simulation-testing)\n17. [Pre-Launch Checklist](#17-pre-launch-checklist)\n18. [Go-Live & Maintenance](#18-go-live--maintenance)\n19. [Appendix A: Annotation Quick Reference](#appendix-a-annotation-quick-reference)\n20. [Appendix B: Common Customer Patterns](#appendix-b-common-customer-patterns)\n21. [Appendix C: Troubleshooting Matrix](#appendix-c-troubleshooting-matrix)\n22. [Appendix D: Glossary](#appendix-d-glossary)\n\n---\n\n# Phase 1: Discovery\n\n---\n\n## 1. Welcome & Scope\n\nThis playbook guides you through onboarding a new customer onto bchat. You'll gather their requirements, help create their AI agent's knowledge base and policy files using LLM assistance, deploy the chat widget on their website, and validate everything before go-live.\n\n### What you'll deliver\n\n- A configured tenant with KB.MD, POLICY.MD, and SCRIPT.MD\n- RAG pipeline indexed and searchable\n- Chat widget embedded and tested on the customer's website\n- Simulation tests passed\n- Customer trained on admin panel and lead management\n\n### Prerequisites\n\n- Access to the bchat admin panel\n- OpenRouter API key configured on the server\n- Basic understanding of markdown editing\n- Customer's website CMS access (or their developer's contact)\n\n---\n\n## 2. Customer Intake\n\nComplete this questionnaire during your first call or email exchange with the customer. Every field informs which features to enable and how to write their configuration files.\n\n### Business Profile\n\n| Question | Why it matters |\n|----------|---------------|\n| What is your business name? | Displayed in chat header, used throughout agent responses |\n| What industry/vertical are you in? | Determines template selection (minimal vs transactional) |\n| What services or products do you offer? | Maps to KB.MD `@service` entries |\n| What services do you NOT offer? | Maps to KB.MD `@exclusion` entries |\n| What is your service area? | Maps to KB.MD `@coverage` entries |\n| Do you have 24/7 emergency service? | Determines if emergency flow is needed |\n| What are your business hours? | KB.MD FAQ + POLICY.MD identity |\n| Do you offer free estimates? | KB.MD FAQ |\n| What payment methods do you accept? | KB.MD FAQ |\n| Are you licensed/insured? | KB.MD FAQ + credibility signal |\n\n### Customer Experience\n\n| Question | Why it matters |\n|----------|---------------|\n| What is your preferred tone? (professional, friendly, casual, urgent) | POLICY.MD identity |\n| How should the agent handle emergencies? | POLICY.MD intent routing |\n| Who should the agent escalate to? | POLICY.MD escalation rules |\n| Do you want the agent to collect leads? | POLICY.MD lead capture policy |\n| How do you want to be notified of new leads? | Webhook/notification setup |\n\n### Technical Profile\n\n| Question | Why it matters |\n|----------|---------------|\n| Do you have an existing website? | Widget deployment method |\n| What CMS/platform is your website on? | Widget embedding approach (IIFE vs iframe) |\n| What is your website URL? | Allowed domains configuration |\n| Do you need multi-language support? | Future feature; note for now |\n| Do you have CRM or ticketing software? | Webhook integrations |\n\n### Timeline\n\n| Question | Why it matters |\n|----------|---------------|\n| When do you want to go live? | Scopes work effort |\n| Who will provide the KB content? | You draft with LLM, they review |\n| Who has final approval authority? | Avoids rework loops |\n\n---\n\n## 3. Feature Selection\n\nBased on the intake, select which features to enable. Use this decision matrix:\n\n### Feature Matrix\n\n| Feature | When to enable | Configuration |\n|---------|---------------|---------------|\n| **RAG Pipeline** | KB > 20KB or complex knowledge base | `RAG_PIPELINE_ENABLED=true` |\n| **Observational Memory** | Long-running conversations, repeat visitors | `OM_ENABLED=true` |\n| **Lead Capture** | Service businesses, B2C | POLICY.MD lead capture rules |\n| **Emergency Flow** | 24/7 service businesses (restoration, HVAC, plumbing) | POLICY.MD emergency intents + SCRIPT.MD priority routing |\n| **Booking/Scheduling** | Appointment-based businesses | POLICY.MD booking intents + SCRIPT.MD intake flow |\n| **Outbound Webhooks** | CRM integration, ticketing, notifications | Webhook configuration in tenant settings |\n\n### Template Selection\n\n| Customer type | KB template | POLICY template | SCRIPT template |\n|--------------|-------------|-----------------|-----------------|\n| Simple service business (no emergencies) | `KB_MINIMAL.MD` | `POLICY_MINIMAL.MD` | `SCRIPT_MINIMAL.MD` |\n| Emergency service (restoration, HVAC, plumbing) | `KB.MD` (full) | `POLICY_TRANSACTIONAL.MD` | `SCRIPT_TRANSACTIONAL.MD` |\n| E-commerce / product catalog | `KB_ECOMMERCE.MD` | `POLICY_MINIMAL.MD` | `SCRIPT_MINIMAL.MD` |\n| Complex transactional (insurance, legal) | `KB.MD` (full) | `POLICY_TRANSACTIONAL.MD` | `SCRIPT_TRANSACTIONAL.MD` |\n\n### Quick Decision Tree\n\n```\nDoes the business have emergency/urgent services?\n├── YES → Use full templates (KB.MD, POLICY_TRANSACTIONAL.MD, SCRIPT_TRANSACTIONAL.MD)\n└── NO\n    ├── Does the business take bookings/appointments?\n    │   ├── YES → Use MINIMAL KB + TRANSACTIONAL POLICY/SCRIPT\n    │   └── NO → Use all MINIMAL templates\n    └── Is it e-commerce?\n        ├── YES → Use KB_ECOMMERCE.MD + MINIMAL POLICY/SCRIPT\n        └── NO → Use MINIMAL templates\n```\n\n---\n\n# Phase 2: Content Creation with LLMs\n\n---\n\n## 4. KB.MD Design\n\nKB.MD is the agent's factual memory. Every fact the agent references comes from this file. Write it for a **smart, friendly new hire** who needs context to represent the business accurately.\n\n### Writing Principles\n\n1. **Be specific.** \"We serve Nassau and Suffolk Counties\" beats \"We serve the tri-state area.\"\n2. **Use bullet points.** The agent handles structured content better than long paragraphs.\n3. **Include every phone number, email, and address.** The agent must never guess contact info.\n4. **Name specific cities and ZIP codes.** \"We serve 110XX, 117XX, 118XX\" beats \"We serve Long Island.\"\n5. **Specify time windows.** \"Within 60 minutes\" beats \"fast response.\"\n6. **List what you DON'T do.** Exclusions prevent the agent from making false promises.\n\n### Structure\n\nEvery KB.MD should contain these sections in order:\n\n1. **Company Overview** — name, type, certifications, availability\n2. **Contact Information** — phone, email, address\n3. **Services** — each with `@service` annotation\n4. **Services We Don't Provide** — each with `@exclusion` annotation\n5. **Service Areas** — `@coverage: include` and `@coverage: exclude`\n6. **Frequently Asked Questions** — each with `@faq` annotation\n7. **Safety Procedures** (if emergency services) — each with `@safety` annotation\n8. **What To Expect** — process overview\n\n### Annotation Reference\n\n| Annotation | Purpose | Example |\n|------------|---------|---------|\n| `@service: code, emergency: true/false` | Service entries | `@service: water_damage, emergency: true, response_time: \"30 minutes\"` |\n| `@exclusion: code` | Services NOT offered | `@exclusion: general_plumbing` |\n| `@coverage: include` | Service area sections | `@coverage: include` |\n| `@coverage: exclude` | Excluded areas | `@coverage: exclude` |\n| `@faq: category` | FAQ entries | `@faq: pricing` |\n| `@safety: hazard_type` | Safety procedures | `@safety: water_emergency` |\n| `@section: name` | Logical sections | `@section: company_overview` |\n\n### Anti-Hallucination Rules\n\n- **Never** leave placeholder text (`[Your Company Name]`, `[Phone Number]`). The agent will fill these with guesses.\n- **Never** include content from other businesses or templates that hasn't been customized.\n- **Never** quote exact prices unless they are truly fixed. Use \"Costs vary based on [factors]. We offer free estimates.\"\n- **Always** include a fallback contact (\"Call us at...\") for information the agent might not have.\n\n### Content Quality Checklist\n\nBefore finalizing KB.MD:\n\n- [ ] Every service the business offers has a `@service` entry\n- [ ] Every major service NOT offered has an `@exclusion` entry\n- [ ] All service areas listed with `@coverage: include` or `@coverage: exclude`\n- [ ] All phone numbers, emails, and addresses are current\n- [ ] Emergency protocol includes after-hours contact (if applicable)\n- [ ] Pricing section sets expectations without false specifics\n- [ ] FAQs cover: hours, estimates, payment, insurance, scheduling\n- [ ] No placeholder text remains\n- [ ] Reviewed by business owner for accuracy\n\n---\n\n## 5. POLICY.MD Design\n\nPOLICY.MD is the agent's operating system. It defines who the agent is, how it behaves, and how it handles edge cases.\n\n### Key Sections\n\n#### Identity\n\nDefine the agent's persona:\n\n```markdown\n## Identity\n<!-- @identity -->\n\n- **Role:** Customer Service Representative for [Business Name]\n- **Tone:** Professional, empathetic, and helpful\n- **Brand Voice:** \"We're here to help.\"\n- **Business hours:** Monday–Friday, 8 AM – 6 PM\n- **After-hours handling:** Collect lead, promise next-business-day callback\n```\n\n**Tone guidelines by vertical:**\n| Vertical | Recommended tone |\n|----------|-----------------|\n| Restoration / Emergency | Calm, reassuring, action-oriented |\n| Professional services | Professional, knowledgeable, measured |\n| Retail / E-commerce | Friendly, helpful, enthusiastic |\n| Healthcare | Empathetic, careful, compliant |\n\n#### Intents\n\nEach intent defines a category of customer message and how the agent should respond:\n\n```markdown\n### Emergency\n<!-- @intent: emergency, category: emergency, urgency: 5, action: emergency_flow -->\n\n**Description:** Urgent, time-sensitive damage or safety issue.\n\n**Agent behavior:**\n- Acknowledge urgency immediately\n- Provide safety instructions\n- Collect: name, phone, location, nature of emergency\n```\n\n**Required intents for every tenant:**\n- `emergency` (if applicable)\n- `service_inquiry`\n- `quote_request` or `booking_request` (or both)\n- `escalation`\n- `greeting`\n- `farewell`\n\n#### Rules\n\nBehavioral rules the agent must follow:\n\n```markdown\n### Safety First\n<!-- @rule: safety_first, priority: 1 -->\n\nIn any emergency, provide safety instructions BEFORE collecting contact info.\n```\n\n**Minimum required rules:**\n- Safety first (if emergency services)\n- No price guessing\n- Stay in scope\n- Empathy first\n- Fallback behavior (\"Say Unknown + Capture\")\n\n#### Thresholds\n\nNumeric thresholds for intent classification:\n\n```markdown\n## Thresholds\n<!-- @thresholds -->\n\n| Metric | Value | Description |\n|--------|-------|-------------|\n| Emergency Urgency | >= 4 | Route to emergency flow |\n| Escalation Confidence | >= 0.85 | Confirm escalation intent |\n```\n\n#### Lead Capture\n\nWhen and how the agent collects contact information:\n\n```markdown\n## Lead Capture Policy\n<!-- @rule: lead_capture -->\n\nCollect contact info when:\n- Visitor asks for a quote or estimate\n- Visitor asks to schedule or book\n- Visitor reports an emergency\n\nDo NOT aggressively collect on:\n- First message before understanding the need\n- Simple FAQ resolved in KB.MD\n```\n\n### POLICY.MD Quality Checklist\n\n- [ ] Identity defines role, tone, brand voice, and after-hours behavior\n- [ ] All required intents are defined with clear agent behavior\n- [ ] Rules cover: safety, pricing, scope, empathy, fallback\n- [ ] Thresholds are set appropriately for the vertical\n- [ ] Lead capture rules are defined (when to ask, when NOT to ask)\n- [ ] Escalation behavior is defined with trigger keywords\n- [ ] Human takeover rules are defined\n\n---\n\n## 6. SCRIPT.MD Design\n\nSCRIPT.MD defines the conversation flow — what stages a conversation goes through and what happens at each stage. It's optional but recommended for transactional businesses.\n\n### When to use SCRIPT.MD\n\n| Use SCRIPT.MD | Skip SCRIPT.MD |\n|---------------|----------------|\n| Appointment booking flows | Simple FAQ-only agents |\n| Lead qualification | Informational chat bots |\n| Emergency intake | Product catalog browsing |\n| Multi-step intake processes | Single-question Q&A |\n\n### Minimal Script (3 stages)\n\nFor simple service businesses:\n\n```markdown\n## Stage: Opening\n- Greet the visitor warmly\n- State who you represent\n- Ask how you can help today\n\n## Stage: Discovery\n- Ask clarifying questions\n- Determine urgency and service needed\n\n## Stage: Closing\n- If showing intent: collect contact info, confirm next steps\n- Thank the visitor\n```\n\n### Transactional Script (full intake)\n\nFor emergency services or appointment booking. See `docs/templates/SCRIPT_TRANSACTIONAL.MD` for the complete template. Key elements:\n\n1. **Priority 0: Emergency + Coverage Check** — runs on every message before other processing\n2. **Emergency responses** — pre-built templates for emergency + in-area, out-of-area, unknown location\n3. **Business introduction** — greeting after priority check passes\n4. **Service intake** — structured questions for collecting service details\n5. **Contact information** — collecting name, phone, address\n6. **Confirmation** — summarizing and confirming details\n7. **Conclusion** — next steps and closing\n\n### Writing Tips\n\n- Keep stages concise — bullet points, not paragraphs\n- Define clear transitions between stages\n- Include \"skip if already provided\" rules to avoid repetitive questions\n- Add special scenario handling (upset customers, repeat visitors)\n\n---\n\n## 7. LLM Prompting Recipes\n\nUse these generic prompts with any LLM (GPT-4o, Claude, Gemini) to draft and refine KB.MD, POLICY.MD, and SCRIPT.MD.\n\n### Drafting KB.MD\n\n**Prompt:**\n```\nI'm onboarding a new customer onto an AI chat agent platform. Help me draft their knowledge base file (KB.MD).\n\nBusiness details:\n- Name: [Business Name]\n- Industry: [Industry]\n- Services: [List of services]\n- Service areas: [Cities, counties, ZIP codes]\n- Business hours: [Hours]\n- Emergency service: [Yes/No, response time]\n- Phone: [Number]\n- Email: [Address]\n\nWrite a KB.MD file in markdown format using these annotations:\n- <!-- @service: code, emergency: true/false --> for each service\n- <!-- @exclusion: code --> for services NOT offered\n- <!-- @coverage: include --> and <!-- @coverage: exclude --> for service areas\n- <!-- @faq: category --> for each FAQ entry\n- <!-- @safety: hazard_type --> for emergency safety procedures (if applicable)\n\nInclude sections for: Company Overview, Contact Info, Services, Exclusions, Service Areas, FAQs (minimum 6), Safety Procedures (if emergency), and What To Expect.\n```\n\n### Drafting POLICY.MD\n\n**Prompt:**\n```\nI'm onboarding a new customer onto an AI chat agent platform. Help me draft their agent policy file (POLICY.MD).\n\nBusiness details:\n- Name: [Business Name]\n- Tone: [Professional/Friendly/Casual/Urgent]\n- Emergency service: [Yes/No]\n- Escalation contact: [Name, phone]\n- Business hours: [Hours]\n- After-hours behavior: [Collect lead/Show message/etc.]\n\nWrite a POLICY.MD file using these annotations:\n- <!-- @identity --> for agent persona\n- <!-- @intent: name, category: standard/emergency/meta, urgency: 1-5, action: flow_name --> for each intent\n- <!-- @rule: name, priority: N --> for behavioral rules\n- <!-- @thresholds --> for urgency scoring thresholds\n\nInclude: Identity, Intents (emergency if applicable, service_inquiry, quote_request, escalation, greeting, farewell), Rules (safety, pricing, scope, empathy, fallback), Thresholds, Lead Capture Policy, Human Takeover rules.\n```\n\n### Drafting SCRIPT.MD\n\n**Prompt:**\n```\nI'm onboarding a new customer onto an AI chat agent platform. Help me draft their conversation flow file (SCRIPT.MD).\n\nBusiness details:\n- Name: [Business Name]\n- Type: [Service business / Emergency service / E-commerce]\n- Services: [List]\n- Service areas: [Areas]\n- Emergency: [Yes/No]\n\nWrite a SCRIPT.MD file that defines conversation stages. Use a structure like:\n- Priority 0: Emergency + Coverage Check (if emergency service)\n- Business Introduction\n- Service Intake (questions to ask)\n- Contact Information\n- Confirmation\n- Conclusion\n\nInclude response templates for different scenarios (emergency in-area, emergency out-of-area, standard inquiry, etc.).\n```\n\n### Refining Content\n\n**Prompt:**\n```\nReview this KB.MD file for a [business type] business. Check for:\n1. Missing services or exclusions\n2. Placeholder text that wasn't replaced\n3. Missing contact information\n4. Vague or unhelpful FAQ answers\n5. Missing service areas\n\nSuggest specific improvements. Here's the current file:\n[paste file content]\n```\n\n### Iterative Workflow\n\n1. **Draft** — Use the appropriate prompt to generate an initial draft\n2. **Review** — Read through with the customer (or alone) to identify gaps\n3. **Refine** — Use the refinement prompt to improve specific sections\n4. **Validate** — Check against the content quality checklist (sections 4-6)\n5. **Test** — Upload to tenant, run simulations, identify issues\n6. **Repeat** — Iterate based on simulation results\n\n---\n\n# Phase 3: Technical Setup\n\n---\n\n## 8. Tenant Provisioning\n\n### Create the Tenant\n\n1. Log in to the admin panel at `https://your-server.com/agent-admin`\n2. Navigate to **Tenants** → **Create Tenant**\n3. Fill in:\n\n| Field | Value | Notes |\n|-------|-------|-------|\n| Slug | `[business-name]` | URL-safe, lowercase, hyphens only |\n| Company Name | `[Business Name]` | Display name in chat header |\n| Vertical | `[industry]` | For your reference |\n| LLM Model | `openai/gpt-4o-mini` | Default; can override per tenant |\n| Temperature | `0.3–0.5` | Lower = more factual; higher = more creative |\n| Allowed Domains | `[customer-domain.com]` | Comma-separated; blank = all domains |\n| Is Active | `Yes` | Must be active for widget to work |\n\n### Slug Guidelines\n\n- Use the business name: `acme-pressure-washing`\n- Lowercase only, hyphens for spaces\n- No special characters, underscores, or uppercase\n- Keep it short but descriptive\n\n### Post-Creation\n\n- Note the tenant slug — it's used in the widget embed URL\n- Verify the GUID was generated (used for widget auth)\n- Confirm the tenant appears in the tenant list\n\n---\n\n## 9. RAG Configuration\n\n### When to Enable RAG\n\n| KB Size | Recommendation |\n|---------|---------------|\n| < 20KB | RAG optional (everything fits in prompt) |\n| 20KB–100KB | RAG recommended |\n| > 100KB | RAG required |\n\nWhen in doubt, enable RAG. It costs nothing extra and improves accuracy for larger knowledge bases.\n\n### Setup Steps\n\n1. **Set environment variables:**\n   ```bash\n   RAG_PIPELINE_ENABLED=true\n   EMBEDDING_PROVIDER=openrouter\n   EMBEDDING_MODEL=text-embedding-3-small\n   ```\n\n2. **Build with RAG support:**\n   ```bash\n   task build:rag\n   ```\n\n3. **Upload files to tenant** (via admin panel)\n\n4. **Click \"Rebuild Index\"** in the tenant detail page\n\n5. **Verify indexing:**\n   - Check the RAG debug panel for the tenant\n   - Run test queries to confirm relevant chunks are retrieved\n   - Verify total chunks indexed and last indexed timestamp\n\n### Embedding Providers\n\n| Provider | Use case | API key needed |\n|----------|----------|---------------|\n| `openrouter` | Production | Yes |\n| `mock` | Testing pipeline only | No |\n| `local` | Local embedding server | No |\n\n### Hybrid Search\n\nBy default, RAG uses hybrid search:\n- **70% vector similarity** — semantic matching (finds related concepts)\n- **30% BM25 keyword matching** — exact term matching (finds service names, city names)\n\nThis means both \"water removal\" and \"standing water extraction\" return the same relevant chunk.\n\n### When to Rebuild Index\n\n- After every KB.MD upload or edit\n- After changing processing options\n- When the agent gives wrong answers about existing content\n- Force reindex on startup: `FORCE_REINDEX_ON_STARTUP=true`\n\n---\n\n## 10. Observational Memory\n\nObservational Memory (OM) gives agents long-term memory by compressing conversation history into an observation log. This is useful for businesses with repeat customers or long-running conversations.\n\n### When to Enable\n\n| Scenario | Enable OM? |\n|----------|-----------|\n| Single-session FAQ bot | No |\n| Repeat customer relationships | Yes |\n| Long intake processes | Yes |\n| Enterprise/CRM-integrated | Yes |\n\n### Configuration\n\n```bash\nOM_ENABLED=true\nOM_OBSERVER_TOKEN_THRESHOLD=30000  # Trigger observer after N tokens\nOM_TOKEN_THRESHOLD=2000             # Trigger reflector to compress observations\n```\n\n### How It Works\n\n1. Messages accumulate in a buffer\n2. When token threshold is reached, the observer compresses them into observations\n3. Observations are stored as system context for future messages\n4. The agent can reference earlier parts of the conversation without the full history\n\n---\n\n## 11. Webhook Integrations\n\n### Outbound Webhooks\n\nSend conversation events to external systems (CRM, ticketing, calendar):\n\n| Event | Payload | Use case |\n|-------|---------|----------|\n| `lead.created` | Lead details + transcript | CRM integration |\n| `lead.updated` | Status change | Pipeline sync |\n| `message.sent` | Full message | Audit log |\n| `simulation.completed` | Transcript + analysis | QA tracking |\n\n### Configuration\n\nIn the tenant admin panel, under **Integrations**:\n\n1. Click **Add Webhook**\n2. Enter the target URL\n3. Select events to subscribe to\n4. Configure authentication (if required)\n5. Test with the webhook test button\n\n### Inbound Webhooks\n\nReceive data from external systems:\n\n| Use case | Example |\n|----------|---------|\n| CRM lead lookup | Enrich conversation with customer history |\n| Calendar availability | Check real-time availability during booking |\n| Ticketing system | Create tickets from escalation events |\n\n---\n\n# Phase 4: Widget Deployment\n\n---\n\n## 12. Widget Embedding\n\nThree methods to embed the chat widget. Choose based on the customer's website platform.\n\n### Option 1: IIFE Script (Recommended)\n\nBest for: Custom websites, HTML sites, Hugo, Jekyll, static sites.\n\n```html\n<script\n  src=\"https://your-server.com/widget/[slug]/embed.js\"\n  data-position=\"bottom-right\"\n  data-color=\"#0d9488\"\n  data-welcome=\"Hi! How can we help you today?\"\n></script>\n```\n\n**Placement:** Before `</body>` tag.\n\n**Configuration attributes:**\n\n| Attribute | Default | Options |\n|-----------|---------|---------|\n| `data-position` | `bottom-right` | `bottom-right`, `bottom-left` |\n| `data-color` | `#0d9488` | Any hex color |\n| `data-welcome` | `How can we help you today?` | Any string |\n| `data-button-size` | `56` | Pixel size |\n| `data-panel-width` | `350` | Pixel width |\n| `data-panel-height` | `500` | Pixel height |\n\n### Option 2: iframe Embed\n\nBest for: WordPress, Shopify, Wix, Squarespace — any CMS with limited script injection.\n\n```html\n<iframe\n  src=\"https://your-server.com/widget/[slug]/iframe?color=%230d9488\"\n  style=\"position:fixed;bottom:0;right:0;width:400px;height:600px;border:none;z-index:9999;\"\n  title=\"Chat Widget\"\n></iframe>\n```\n\n**Note:** URL-encode `#` as `%23` in query parameters.\n\n### Option 3: npm Package\n\nBest for: React/Vue/Angular apps.\n\n```jsx\nimport { useEffect } from 'react';\n\nfunction ChatWidget({ tenantSlug, color = '#0d9488' }) {\n  useEffect(() => {\n    window.AgentChatConfig = { color };\n    const script = document.createElement('script');\n    script.src = `https://your-server.com/widget/${tenantSlug}/embed.js`;\n    script.async = true;\n    document.body.appendChild(script);\n\n    return () => {\n      const widget = document.getElementById('agent-chat-widget');\n      if (widget) widget.remove();\n      script.remove();\n    };\n  }, [tenantSlug, color]);\n\n  return null;\n}\n```\n\n### Domain Restrictions\n\nFor security, set allowed domains in tenant settings:\n\n```\ncustomer-domain.com, www.customer-domain.com\n```\n\nLeave blank to allow embedding on any domain. The widget checks the `Referer` header against this list.\n\n---\n\n## 13. CMS-Specific Checklists\n\n### Hugo (Self-Hosted)\n\n1. Create a partial: `layouts/partials/bchat-widget.html`\n2. Paste the IIFE script tag\n3. Include the partial in your base template: `{{ partial \"bchat-widget.html\" . }}`\n4. Build and deploy: `hugo && rsync public/ user@server:/var/www/`\n\n**Checklist:**\n- [ ] Widget partial created\n- [ ] Partial included in base template\n- [ ] Build succeeds without errors\n- [ ] Widget loads on deployed site\n\n### WordPress\n\n**Option A: Plugin (if available)**\n1. Install a \"Custom HTML\" or \"Insert Headers and Footers\" plugin\n2. Paste the iframe embed code\n3. Save and verify\n\n**Option B: Manual (footer.php)**\n1. Log in to WordPress admin\n2. Go to Appearance → Theme Editor\n3. Edit `footer.php`\n4. Paste the iframe code before `</body>`\n5. Save and verify\n\n**Checklist:**\n- [ ] Embed code added to site\n- [ ] Widget appears on frontend\n- [ ] No conflicts with existing plugins\n- [ ] Mobile responsive\n\n### Shopify\n\n1. Go to Online Store → Themes → Edit Code\n2. Open `theme.liquid`\n3. Paste the iframe embed before `</body>`\n4. Save and preview\n\n**Checklist:**\n- [ ] Code added to `theme.liquid`\n- [ ] Widget appears on storefront\n- [ ] No conflicts with Shopify scripts\n- [ ] Works on mobile\n\n### Wix / Squarespace\n\nThese platforms don't allow direct HTML editing on all pages. Use the iframe method:\n\n1. Add an \"Embed HTML\" element to the page\n2. Paste the iframe code\n3. Resize the element to cover the desired widget area\n4. Publish and verify\n\n**Checklist:**\n- [ ] Embed element added\n- [ ] iframe code pasted\n- [ ] Widget visible after publish\n- [ ] No layout conflicts\n\n### Generic HTML\n\n1. Open the site's HTML files\n2. Paste the IIFE script before `</body>`\n3. Deploy to hosting\n\n**Checklist:**\n- [ ] Script added to all relevant pages\n- [ ] Widget loads on every page\n- [ ] No JavaScript errors in console\n\n---\n\n## 14. Widget Customization\n\n### Brand Matching\n\n| Element | How to customize |\n|---------|-----------------|\n| Primary color | `data-color` attribute or `color` query param |\n| Button position | `data-position` attribute (bottom-right or bottom-left) |\n| Welcome message | `data-welcome` attribute |\n| Company name | Set in tenant settings (auto-populated) |\n| Panel size | `data-panel-width` and `data-panel-height` |\n\n### CSS Overrides\n\nFor advanced customization, override widget CSS after the script loads:\n\n```html\n<style>\n  #acw-toggle, #acw-header, .acw-msg-user, #acw-send {\n    background: #4f46e5 !important; /* Indigo */\n  }\n  #acw-panel {\n    width: 400px !important;\n    height: 600px !important;\n  }\n</style>\n```\n\n### Widget CSS Classes\n\n| Selector | Element |\n|----------|---------|\n| `#agent-chat-widget` | Root container |\n| `#acw-toggle` | Floating button |\n| `#acw-panel` | Chat panel |\n| `#acw-header` | Panel header |\n| `#acw-messages` | Messages container |\n| `#acw-input` | Text input |\n| `#acw-send` | Send button |\n\n---\n\n## 15. Widget Testing\n\n### Local Testing\n\n1. Open the page with the widget embed in a browser\n2. Click the chat button — panel should open\n3. Send a test message — verify agent responds\n4. Check browser console for JavaScript errors\n5. Verify the welcome message appears\n\n### Cross-Browser Checklist\n\n- [ ] Chrome (latest)\n- [ ] Firefox (latest)\n- [ ] Safari (latest)\n- [ ] Edge (latest)\n- [ ] Mobile Safari (iOS)\n- [ ] Chrome for Android\n\n### Mobile Testing\n\n- Widget should be responsive on screens < 480px\n- Panel should expand to near-full-width on mobile\n- Input should be usable with virtual keyboard\n- Button should not overlap critical page content\n\n### Security Verification\n\n- [ ] Widget loads over HTTPS\n- [ ] No mixed content warnings\n- [ ] Domain restrictions are working (if configured)\n- [ ] iframe is isolated from parent page\n\n---\n\n# Phase 5: Validation & Launch\n\n---\n\n## 16. Simulation Testing\n\n### Running Simulations\n\n1. Go to **Agent Admin** → **[Tenant]** → **Simulations**\n2. Configure a virtual customer persona:\n   - **Name:** [Test name]\n   - **Scenario:** [Description of the test scenario]\n   - **Goal:** [What the virtual customer wants to achieve]\n   - **Tone:** [Anxious, neutral, friendly, frustrated]\n3. Click **Run Simulation**\n4. Watch the conversation unfold in real-time\n\n### Test Scenarios (minimum 3 per tenant)\n\n| # | Scenario | Tests |\n|---|----------|-------|\n| 1 | Emergency inquiry | Emergency flow, safety instructions, contact collection |\n| 2 | Quote/service request | Service matching, lead capture, follow-up promise |\n| 3 | General FAQ | Information retrieval, no unnecessary lead capture |\n| 4 | Out-of-area | Coverage handling, polite decline |\n| 5 | Escalation | Trigger detection, handoff behavior |\n\n### Interpreting Results\n\n| Issue | Likely cause | Fix |\n|-------|-------------|-----|\n| Agent gives wrong info | KB.MD missing or incomplete | Add content to KB.MD, rebuild index |\n| Agent guesses pricing | KB.MD has pricing or POLICY.MD missing rule | Add \"no price guessing\" rule |\n| Agent doesn't capture lead | POLICY.MD missing lead capture rules | Add lead capture policy |\n| Agent can't answer basic question | RAG not enabled or index stale | Enable RAG, rebuild index |\n| Agent escalates too easily | Thresholds too low | Adjust urgency/confidence thresholds |\n| Agent is too robotic | Temperature too low | Increase to 0.4–0.6 |\n\n### RAG Debug Testing\n\n1. Go to **Agent Admin** → **[Tenant]** → **RAG Debug**\n2. Enter test queries and verify:\n   - Correct chunks are retrieved\n   - Relevance scores are reasonable (> 0.5)\n   - Exclusions are respected\n   - Service areas are matched\n\n---\n\n## 17. Pre-Launch Checklist\n\n### Content Verification\n\n- [ ] KB.MD reviewed by business owner (not just you)\n- [ ] All phone numbers, emails, addresses verified current\n- [ ] All services and exclusions accurate\n- [ ] Service areas complete and correct\n- [ ] No placeholder text remaining\n\n### Technical Verification\n\n- [ ] Tenant is active (`Is Active = Yes`)\n- [ ] RAG index rebuilt after final content upload\n- [ ] Widget loads without errors on customer's website\n- [ ] Widget works on mobile\n- [ ] Domain restrictions configured (if needed)\n- [ ] HTTPS working for widget delivery\n\n### Agent Behavior\n\n- [ ] At least 3 simulations passed\n- [ ] RAG debug returns correct chunks for 10+ queries\n- [ ] Lead capture tested (name + email/phone extracted)\n- [ ] Escalation flow tested (\"I want a manager\")\n- [ ] Fallback behavior works (\"I don't have that info\")\n- [ ] Emergency flow works (if applicable)\n\n### Operational Readiness\n\n- [ ] Business owner has admin access\n- [ ] Business owner trained on viewing leads\n- [ ] Lead notification method configured\n- [ ] Rollback plan documented (deactivate tenant if needed)\n\n---\n\n## 18. Go-Live & Maintenance\n\n### Go-Live Steps\n\n1. **Final review:** Walk through pre-launch checklist with customer\n2. **Activate widget:** Ensure `Is Active = Yes` and widget is live\n3. **First lead:** Monitor for the first real conversation\n4. **Follow up:** Check that lead notification works\n5. **Confirm:** Tell the customer they're live\n\n### Ongoing Maintenance\n\n| Task | Frequency | How |\n|------|-----------|-----|\n| Review lead quality | Daily | Admin panel → Leads |\n| Check for wrong answers | Weekly | Review conversation transcripts |\n| Update KB.MD | As needed | Upload new version, rebuild index |\n| Review RAG quality | Monthly | RAG debug panel, test queries |\n| Update pricing/info | When changes occur | Edit KB.MD, rebuild index |\n\n### Content Refresh Triggers\n\nRebuild the RAG index when:\n- Services change (new or removed)\n- Pricing structure changes\n- Business hours change\n- Contact information changes\n- Service areas change\n- New FAQs emerge from real conversations\n\n### Monitoring\n\n| What to watch | Where | Action |\n|---------------|-------|--------|\n| Wrong answers | Conversation transcripts | Update KB.MD |\n| No leads captured | Lead list | Check POLICY.MD lead rules |\n| Widget not loading | Customer website | Check domain config, server status |\n| Slow responses | Server logs | Check LLM API, increase timeout |\n\n### Scaling Considerations\n\n| Metric | Threshold | Action |\n|--------|-----------|--------|\n| Leads per day > 50 | High volume | Consider webhook automation |\n| KB.MD > 200KB | Large knowledge base | Split into focused sections |\n| Response time > 5s | Slow | Check LLM model, reduce context |\n| Multiple locations | Multi-site | Consider separate tenants |\n\n---\n\n# Appendix A: Annotation Quick Reference\n\n### KB.MD Annotations\n\n| Annotation | Purpose | Example |\n|------------|---------|---------|\n| `@service: code, emergency: bool` | Service entry | `@service: water_damage, emergency: true` |\n| `@exclusion: code` | Service not offered | `@exclusion: general_plumbing` |\n| `@coverage: include` | Service area section | `@coverage: include` |\n| `@coverage: exclude` | Excluded area section | `@coverage: exclude` |\n| `@faq: category` | FAQ entry | `@faq: pricing` |\n| `@safety: type, triggers: list` | Safety procedure | `@safety: water_emergency, triggers: flood` |\n| `@section: name` | Logical section | `@section: company_overview` |\n\n### POLICY.MD Annotations\n\n| Annotation | Purpose | Example |\n|------------|---------|---------|\n| `@identity` | Agent persona | `@identity` |\n| `@intent: name, category, urgency, action` | Intent definition | `@intent: emergency, category: emergency, urgency: 5, action: emergency_flow` |\n| `@rule: name, priority: N` | Behavioral rule | `@rule: safety_first, priority: 1` |\n| `@thresholds` | Urgency scoring | `@thresholds` |\n| `@verification_rule: name, type, severity` | Response verification | `@verification_rule: contact_grounding, type: exact_match, severity: critical` |\n\n###SCRIPT.MD\n\nSCRIPT.MD uses plain markdown section headers (no annotations). Define stages as `## Stage: Name` headers with bullet-point instructions.\n\n---\n\n# Appendix B: Common Customer Patterns\n\n### Pattern 1: Local Service Business\n\n**Profile:** Plumber, electrician, landscaper, cleaning service\n- Simple services, defined service area\n- No emergency flow needed\n- Lead capture is primary goal\n\n**Templates:** KB_MINIMAL.MD + POLICY_MINIMAL.MD + SCRIPT_MINIMAL.MD\n**Features:** RAG (if KB > 20KB), Lead Capture\n**Widget:** IIFE script on existing website\n\n---\n\n### Pattern 2: Emergency Service Business\n\n**Profile:** Restoration, HVAC, water extraction, fire damage\n- 24/7 emergency response\n- Complex intake process\n- Urgency classification critical\n\n**Templates:** KB.MD (full) + POLICY_TRANSACTIONAL.MD + SCRIPT_TRANSACTIONAL.MD\n**Features:** RAG, Emergency Flow, Lead Capture, Observational Memory\n**Widget:** IIFE script or iframe (depending on CMS)\n\n---\n\n### Pattern 3: E-Commerce / Product Catalog\n\n**Profile:** Online retail, product-based business\n- Product catalog instead of services\n- Shipping, returns, order tracking FAQs\n- No emergency flow\n\n**Templates:** KB_ECOMMERCE.MD + POLICY_MINIMAL.MD + SCRIPT_MINIMAL.MD\n**Features:** RAG (for large catalogs), Lead Capture (optional)\n**Widget:** IIFE or iframe\n\n---\n\n### Pattern 4: Professional Services\n\n**Profile:** Law firm, accounting, consulting, agency\n- Appointment-based\n- Complex qualification process\n- High-value leads\n\n**Templates:** KB_MINIMAL.MD (customized) + POLICY_TRANSACTIONAL.MD + SCRIPT_TRANSACTIONAL.MD\n**Features:** RAG, Booking/Scheduling, Lead Capture\n**Widget:** IIFE script\n\n---\n\n### Pattern 5: Multi-Location Business\n\n**Profile:** Franchise, chain, multi-site operation\n- Multiple service areas\n- May need separate tenants per location\n- Shared knowledge base possible\n\n**Templates:** Per-tenant configuration\n**Features:** RAG, Lead Capture, Webhook Integrations\n**Widget:** Same embed code, different slug per location\n\n---\n\n# Appendix C: Troubleshooting Matrix\n\n| Symptom | Likely Cause | Fix |\n|---------|-------------|-----|\n| Widget not appearing | Wrong slug, tenant inactive, domain restriction | Check slug, verify `Is Active`, check allowed domains |\n| Agent gives wrong answer | KB.MD incomplete or RAG not indexed | Update KB.MD, rebuild index |\n| Agent hallucinates pricing | No \"no price guessing\" rule in POLICY.MD | Add rule, lower temperature |\n| No leads captured | Lead capture rules missing in POLICY.MD | Add lead capture policy |\n| Widget loads but no response | LLM API key invalid or quota exceeded | Check OpenRouter dashboard |\n| Slow responses | Large context, slow model | Reduce KB size, switch to faster model |\n| Agent can't find service | RAG index stale or service not in KB.MD | Rebuild index, verify KB.MD content |\n| Agent says wrong phone number | Phone number placeholder not replaced | Search KB.MD for placeholders, replace |\n| Widget broken on mobile | CSS conflict, panel too small | Check responsive styles, adjust panel size |\n| CORS errors | Server not configured for widget domain | Check server CORS settings |\n| Simulation fails | API key issue, tenant misconfigured | Check API key, verify tenant settings |\n\n---\n\n# Appendix D: Glossary\n\n| Term | Definition |\n|------|-----------|\n| **Tenant** | One business/client. All data is scoped to a tenant. |\n| **Slug** | URL-safe tenant identifier (e.g., `acme-corp`). Used in embed URLs. |\n| **KB.MD** | Markdown knowledge base. The agent's factual memory. |\n| **POLICY.MD** | Agent identity, rules, intents, thresholds, and escalation behavior. |\n| **SCRIPT.MD** | Conversation flow stages. Optional but recommended for transactional agents. |\n| **RAG** | Retrieval-Augmented Generation. Agent searches relevant KB chunks before answering. |\n| **LanceDB** | Embedded vector database for RAG storage. |\n| **Embedding** | Numerical vector representation of text for similarity search. |\n| **Hybrid Search** | 70% vector similarity + 30% BM25 keyword matching. |\n| **Observational Memory (OM)** | Long-term memory via conversation compression. |\n| **Lead** | Captured prospect with contact info and conversation context. |\n| **Simulation** | Virtual conversation test using LLM-powered simulated customer. |\n| **SSE** | Server-Sent Events. How the agent streams responses in real-time. |\n| **IIFE** | Immediately Invoked Function Expression. The `<script>` embed method. |\n| **Guid** | Globally unique identifier for tenant widget authentication. |\n\n---\n\n*End of DOCS_ONBOARDING.md*\n",
  "filePath": "/home/chaschel/Documents/go/bchat/docs/DOCS_ONBOARDING.md"
}
```

**Output:**
```
Wrote file successfully.
```

---

## Assistant (Build · MiMo V2.5 Free · 7.6s)

_Thinking:_

The main onboarding doc is created. Let me update the todo list and verify the file.

**Tool: todowrite**

**Input:**
```json
{
  "todos": [
    {
      "content": "Create docs/templates/KB_MINIMAL.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/templates/KB_ECOMMERCE.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/templates/POLICY_MINIMAL.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/templates/POLICY_TRANSACTIONAL.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/templates/SCRIPT_MINIMAL.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/templates/SCRIPT_TRANSACTIONAL.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Update docs/BIZ_QUICK_START_CHECKLIST.MD",
      "priority": "medium",
      "status": "completed"
    },
    {
      "content": "Create docs/DOCS_ONBOARDING.md — All phases and appendices",
      "priority": "high",
      "status": "completed"
    }
  ]
}
```

**Output:**
```
[
  {
    "content": "Create docs/templates/KB_MINIMAL.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/KB_ECOMMERCE.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/POLICY_MINIMAL.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/POLICY_TRANSACTIONAL.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/SCRIPT_MINIMAL.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/templates/SCRIPT_TRANSACTIONAL.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Update docs/BIZ_QUICK_START_CHECKLIST.MD",
    "status": "completed",
    "priority": "medium"
  },
  {
    "content": "Create docs/DOCS_ONBOARDING.md — All phases and appendices",
    "status": "completed",
    "priority": "high"
  }
]
```

---

## Assistant (Build · MiMo V2.5 Free · 8.8s)

_Thinking:_

All tasks completed. Let me verify the files were created correctly by listing them.

Let me verify everything was created correctly.

**Tool: bash**

**Input:**
```json
{
  "command": "ls -la /home/chaschel/Documents/go/bchat/docs/DOCS_ONBOARDING.md /home/chaschel/Documents/go/bchat/docs/BIZ_QUICK_START_CHECKLIST.MD /home/chaschel/Documents/go/bchat/docs/templates/*.MD",
  "description": "List all created/updated files"
}
```

**Output:**
```
-rw------- 1 chaschel chaschel  4507 Jul 11 01:22 /home/chaschel/Documents/go/bchat/docs/BIZ_QUICK_START_CHECKLIST.MD
-rw-rw-r-- 1 chaschel chaschel 39109 Jul 11 01:24 /home/chaschel/Documents/go/bchat/docs/DOCS_ONBOARDING.md
-rw-rw-r-- 1 chaschel chaschel  9140 Feb  7 03:42 /home/chaschel/Documents/go/bchat/docs/templates/GUIDELINES.MD
-rw-rw-r-- 1 chaschel chaschel  2861 Jul 11 01:20 /home/chaschel/Documents/go/bchat/docs/templates/KB_ECOMMERCE.MD
-rw-rw-r-- 1 chaschel chaschel  6769 Feb  7 03:42 /home/chaschel/Documents/go/bchat/docs/templates/KB.MD
-rw-rw-r-- 1 chaschel chaschel  2180 Jul 11 01:20 /home/chaschel/Documents/go/bchat/docs/templates/KB_MINIMAL.MD
-rw-rw-r-- 1 chaschel chaschel 11282 Feb  7 03:42 /home/chaschel/Documents/go/bchat/docs/templates/POLICY.MD
-rw-rw-r-- 1 chaschel chaschel  3060 Jul 11 01:20 /home/chaschel/Documents/go/bchat/docs/templates/POLICY_MINIMAL.MD
-rw-rw-r-- 1 chaschel chaschel  7016 Jul 11 01:20 /home/chaschel/Documents/go/bchat/docs/templates/POLICY_TRANSACTIONAL.MD
-rw-rw-r-- 1 chaschel chaschel  1364 Feb  7 03:42 /home/chaschel/Documents/go/bchat/docs/templates/PROMPTS.MD
-rw-rw-r-- 1 chaschel chaschel  1092 Jul 11 01:20 /home/chaschel/Documents/go/bchat/docs/templates/SCRIPT_MINIMAL.MD
-rw-rw-r-- 1 chaschel chaschel  7768 Feb  7 03:42 /home/chaschel/Documents/go/bchat/docs/templates/SCRIPT_OPUS13.MD
-rw-rw-r-- 1 chaschel chaschel  5345 Jul 11 01:21 /home/chaschel/Documents/go/bchat/docs/templates/SCRIPT_TRANSACTIONAL.MD

```

**Tool: bash**

**Input:**
```json
{
  "command": "wc -l /home/chaschel/Documents/go/bchat/docs/DOCS_ONBOARDING.md /home/chaschel/Documents/go/bchat/docs/BIZ_QUICK_START_CHECKLIST.MD /home/chaschel/Documents/go/bchat/docs/templates/KB_MINIMAL.MD /home/chaschel/Documents/go/bchat/docs/templates/KB_ECOMMERCE.MD /home/chaschel/Documents/go/bchat/docs/templates/POLICY_MINIMAL.MD /home/chaschel/Documents/go/bchat/docs/templates/POLICY_TRANSACTIONAL.MD /home/chaschel/Documents/go/bchat/docs/templates/SCRIPT_MINIMAL.MD /home/chaschel/Documents/go/bchat/docs/templates/SCRIPT_TRANSACTIONAL.MD",
  "description": "Count lines in all files"
}
```

**Output:**
```
 1162 /home/chaschel/Documents/go/bchat/docs/DOCS_ONBOARDING.md
  143 /home/chaschel/Documents/go/bchat/docs/BIZ_QUICK_START_CHECKLIST.MD
  119 /home/chaschel/Documents/go/bchat/docs/templates/KB_MINIMAL.MD
  136 /home/chaschel/Documents/go/bchat/docs/templates/KB_ECOMMERCE.MD
  122 /home/chaschel/Documents/go/bchat/docs/templates/POLICY_MINIMAL.MD
  256 /home/chaschel/Documents/go/bchat/docs/templates/POLICY_TRANSACTIONAL.MD
   46 /home/chaschel/Documents/go/bchat/docs/templates/SCRIPT_MINIMAL.MD
  212 /home/chaschel/Documents/go/bchat/docs/templates/SCRIPT_TRANSACTIONAL.MD
 2196 total

```

---

## Assistant (Build · MiMo V2.5 Free · 9.4s)

_Thinking:_

Everything is created and looking good. Let me also verify the content of the DOCS_ONBOARDING.md by reading the first few sections to make sure it's well-structured.

All files created successfully. Here's the summary:

### Created files (8 files, 2,196 lines total)

| File | Lines | Purpose |
|------|-------|---------|
| `docs/DOCS_ONBOARDING.md` | 1,162 | Main onboarding playbook (22 sections + appendices) |
| `docs/BIZ_QUICK_START_CHECKLIST.MD` | 143 | Updated checklist (added RAG, widget, simulation steps) |
| `docs/templates/KB_MINIMAL.MD` | 119 | Simple service business KB template |
| `docs/templates/KB_ECOMMERCE.MD` | 136 | E-commerce KB template |
| `docs/templates/POLICY_MINIMAL.MD` | 122 | FAQ-only agent policy |
| `docs/templates/POLICY_TRANSACTIONAL.MD` | 256 | Full transactional policy with booking/emergency |
| `docs/templates/SCRIPT_MINIMAL.MD` | 46 | Simple 3-stage conversation flow |
| `docs/templates/SCRIPT_TRANSACTIONAL.MD` | 212 | Full intake flow with priority routing |

### DOCS_ONBOARDING.md structure

- **Phase 1: Discovery** — Customer intake questionnaire, feature selection decision tree
- **Phase 2: Content Creation** — KB/POLICY/SCRIPT design guides with LLM prompting recipes
- **Phase 3: Technical Setup** — Tenant provisioning, RAG, OM, webhooks
- **Phase 4: Widget Deployment** — IIFE/iframe/npm embedding, CMS checklists (Hugo, WordPress, Shopify, Wix, generic HTML)
- **Phase 5: Validation & Launch** — Simulation testing, pre-launch checklist, go-live & maintenance
- **Appendices** — Annotation reference, customer patterns, troubleshooting, glossary

---

