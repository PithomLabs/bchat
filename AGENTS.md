# AGENTS.md - AI Agent Development Guide

**Purpose:** Comprehensive guide for AI agents (Claude, Gemini, etc.) working on the bchat codebase.

---

## Project Overview

bchat is a **multi-tenant AI chat agent platform** built on top of the Memos application. Each tenant can define their own knowledge base, policies, and conversation scripts without requiring code changes.

### Core Design Principle

The chat agent must be **GENERAL PURPOSE**, not tenant-specific.

| DO | DO NOT |
|----|--------|
| Design features that work for ANY tenant configuration | Hardcode tenant-specific logic in the codebase |
| Let KB/Policy/Script files define all tenant-specific behavior | Add conditional behavior based on tenant ID or slug |
| Keep the agent code generic and configuration-driven | Create tenant-specific prompts or handlers |
| Test features with multiple tenant types | Build features that only work for certain verticals |

### Technology Stack

| Layer | Technology |
|-------|------------|
| Backend | Go 1.21+, Echo framework |
| Database | SQLite (default), PostgreSQL, MySQL |
| Frontend | React 18, TypeScript, MobX, Vite |
| UI Components | Joy UI (@mui/joy) |
| LLM Provider | OpenRouter API |
| Vector Database | LanceDB (local/S3) or CockroachDB native vector columns |
| Postgres Driver | pgx/v5 (sole driver — `lib/pq` is NOT used and must not be added) |
| Embeddings | OpenRouter text-embedding-3-small, local sentence-transformers, or mock |

---

## Directory Structure

```
bchat/
├── bin/memos/              # Application entry point
├── build/                  # Build output directory
│   └── data/               # Runtime data (SQLite DB, LanceDB indexes)
├── docs/                   # Documentation (START HERE)
├── server/
│   └── router/api/v1/
│       └── agent/          # Agent API handlers and services
│           ├── handlers.go     # HTTP request handlers
│           ├── service.go      # Business logic, LLM integration
│           ├── parser.go       # KB/Policy/Script parsing
│           ├── vectordb.go     # Vector database interface
│           ├── vectordb_lance.go # LanceDB implementation
│           ├── vectordb_cockroach.go # CockroachDB native vector implementation
│           ├── embedding.go    # Embedding providers
│           ├── chunker.go      # Document chunking
│           ├── observer.go     # Observational memory
│           ├── simulation.go   # Agent simulation
│           ├── analysis.go     # Transcript analysis
│           ├── verifier.go     # LLM response verification
│           ├── sanitizer.go    # Output sanitization
│           └── prompts/        # Prompt templates
├── store/
│   ├── agent.go            # Data types and store interface
│   ├── driver.go           # Database driver interface
│   ├── db/sqlite/          # SQLite implementation
│   └── migration/sqlite/   # Database migrations
├── web/
│   ├── src/
│   │   ├── pages/          # React page components
│   │   ├── store/v2/       # MobX stores
│   │   └── locales/        # i18n translations
│   └── dist/               # Built frontend assets
├── widget/                 # Embeddable chat widget
├── plugin/                 # Cron, webhook, storage plugins
└── Taskfile.yml            # Build and run commands
```

---

## Key Concepts

### 1. Tenant Model

Each tenant represents an isolated agent configuration:

```go
type AgentTenant struct {
    ID                int32
    Slug              string      // URL-friendly (e.g., "acme-corp")
    CompanyName       string      // Display name
    GUID              string      // Unique identifier for widget embed
    Vertical          string      // Industry vertical
    IsActive          bool
    ProcessingOptions string      // JSON-encoded processing options
    AllowedDomains    string      // JSON array of allowed domains
}
```

**Key security property:** Every API request must be scoped to a single tenant. See [Tenant Isolation Architecture](#tenant-isolation-architecture) for implementation details.

### 2. Configuration Files

Each tenant can upload three markdown files:

#### KB.MD (Knowledge Base)
Factual information the agent references:
```markdown
<!-- @service: water_extraction, emergency: true -->
## Water Extraction
24/7 emergency response for standing water removal...

<!-- @faq: pricing -->
## How much does it cost?
Costs vary based on extent of damage...
```

#### POLICY.MD (Agent Policy)
Defines behavior, identity, and rules:
```markdown
<!-- @identity -->
- Role: Customer Service Representative
- Tone: Professional, empathetic

<!-- @intent: schedule_service -->
## Schedule Service
Customer wants to book an appointment...
```

#### SCRIPT.MD (Conversation Flow)
Defines conversation stages:
```markdown
## Stage: Opening
- Greet the customer
- Ask how you can help

## Stage: Resolution
- Provide solution
- Confirm satisfaction
```

### 3. Annotation System

The parser uses a generic annotation format:
```markdown
<!-- @type: value, key: value -->
```

**Supported KB annotations:**
| Annotation | Purpose |
|------------|---------|
| `@service` | Service/product entries |
| `@faq` | Frequently asked questions |
| `@exclusion` | Services NOT provided |
| `@coverage` | Service areas |
| `@safety` | Safety information |
| `@section` | Generic content sections |

**Supported Policy annotations:**
| Annotation | Purpose |
|------------|---------|
| `@identity` | Agent persona |
| `@intent` | Customer intent categories |
| `@rule` | Behavioral rules |
| `@thresholds` | Numeric thresholds |

---

## Tenant Isolation Architecture

Every API request must be scoped to a single tenant. The system uses a dual-context mechanism to extract tenant ID from JWT claims.

### JWT Claims

The `ClaimsMessage` struct includes a `TenantID` field:

```go
type ClaimsMessage struct {
    Username string
    Role     RoleType
    TenantID *int32  // json:"tenant_id,omitempty"
    // ... other fields
}
```

- **Single-tenant users**: JWT contains their tenant ID automatically
- **Multi-tenant users**: Must explicitly select tenant via REST flow (see [Multi-Tenant Auth Flow](#multi-tenant-auth-flow))

### Context Mechanism

Two context implementations exist for different frameworks:

| Framework | Function | Return Type |
|-----------|----------|-------------|
| Echo (HTTP) | `getTenantFromContext(c)` | `*int32` |
| gRPC | `GetTenantIDFromContext(ctx)` | `*int32` |

Both extract tenant ID from context values set by the auth middleware.

### Required Pattern

Every handler that reads/writes tenant-scoped data must:

1. **Extract tenant ID** from context
2. **Apply tenant filter** to database queries
3. **Verify tenant ownership** for update/delete operations

```go
// Echo handler example
func (h *Handler) CreateMemo(c echo.Context) error {
    tenantID := getTenantFromContext(c)
    memo := &store.Memo{
        TenantID: tenantID,
        // ... other fields
    }
    // ...
}
```

For detailed implementation patterns, see [Security Guidelines](#security-guidelines-tenant-isolation).

---

## RAG Pipeline

### Architecture

```
Document Upload → Chunker → Embedding → VectorDB → [Query Time]
                           ↓                        ↓
                    LanceDB (local/S3)    CockroachDB (native VECTOR columns)
                           ↓                        ↓
                    User Query → Embed → Vector Search → Top-K → LLM Prompt
```

### Key Components

| File | Purpose |
|------|---------|
| [`chunker.go`](server/router/api/v1/agent/chunker.go) | Document chunking (~500 tokens with overlap) |
| [`embedding.go`](server/router/api/v1/agent/embedding.go) | Embedding providers (openrouter, local, mock) |
| [`vectordb.go`](server/router/api/v1/agent/vectordb.go) | Vector database interface |
| [`vectordb_lance.go`](server/router/api/v1/agent/vectordb_lance.go) | LanceDB implementation |
| [`vectordb_cockroach.go`](server/router/api/v1/agent/vectordb_cockroach.go) | CockroachDB native vector implementation |

### Embedding Providers

| Provider | Dimensions | Use Case | API Key |
|----------|------------|----------|---------|
| openrouter | 1536 | Production | Yes |
| local | 384 | Development | No |
| mock | 1536 | Testing | No |

### Hybrid Search

Combines vector similarity (70%) with BM25 keyword matching (30%):
```bash
HYBRID_SEARCH_ENABLED=true
```

---

## Observational Memory (OM)

OM gives agents long-term memory by compressing conversation history into an observation log.

| File | Purpose |
|------|---------|
| [`observer.go`](server/router/api/v1/agent/observer.go) | Core observer implementation |
| [`observer_buffer.go`](server/router/api/v1/agent/observer_buffer.go) | Message buffering |
| [`om_config.go`](server/router/api/v1/agent/om_config.go) | Configuration |

Key environment variables:
- `OM_ENABLED=true` - Enable observational memory
- `OM_OBSERVER_TOKEN_THRESHOLD=30000` - Trigger observer after N tokens
- `OM_TOKEN_THRESHOLD=2000` - Trigger reflector to compress observations

---

## Build Commands

### Quick Start (No RAG)
```bash
task setup
export OPENROUTER_API_KEY=sk-or-v1-xxx
task build
./build/memos --mode dev --data build/data
```

### With RAG Support
```bash
task build:rag
task run:rag
# Or with mock embeddings (no API key needed)
task run:rag:mock
```

### Development
```bash
# Terminal 1: Backend
task dev:backend

# Terminal 2: Frontend
task dev:frontend
```

### Common Build Tasks
```bash
task build:backend       # Backend only
task build:frontend      # Frontend only
task build:rag           # Full build with RAG
task run:rag:l12         # L12 embeddings via OpenRouter
task validate:schema     # Validate database schema
```

---

## Environment Variables

### Required
```bash
OPENROUTER_API_KEY=sk-or-v1-xxx
```

### LLM Configuration
```bash
LLM_MODEL=openai/gpt-4o-mini
LLM_MODEL_REASONING=google/gemini-2.5-pro  # For content generation
```

### RAG Configuration
```bash
RAG_PIPELINE_ENABLED=true
EMBEDDING_PROVIDER=openrouter
EMBEDDING_MODEL=text-embedding-3-small
EMBEDDING_BATCH_SIZE=10
EMBEDDING_TIMEOUT=180s
LANCEDB_STORAGE_PROVIDER=local     # local|s3|cockroach
LANCEDB_LOCAL_PATH=build/data/lancedb
```

### Observational Memory
```bash
OM_ENABLED=true
OM_OBSERVER_TOKEN_THRESHOLD=30000
OM_TOKEN_THRESHOLD=2000
```

### Startup Flags
```bash
FORCE_REINDEX_ON_STARTUP=true  # Re-index all content
HYBRID_SEARCH_ENABLED=true     # Enable hybrid search
LLM_VERIFIER_ENABLED=true      # Enable LLM verification
```

### Postgres DSN
```bash
# DATABASE_URL or MEMOS_DSN — pgx/v5 auto-appends default_query_exec_mode=simple_protocol
# for Neon pgbouncer compatibility. No manual DSN tuning needed.
DATABASE_URL="postgresql://user:pass@host/db?sslmode=require"
```

### CockroachDB Configuration
```bash
# Driver selection
MEMOS_DRIVER=cockroach

# Vector storage (replaces LanceDB with native CRDB vector support)
LANCEDB_STORAGE_PROVIDER=cockroach
COCKROACH_DSN="postgresql://user:pass@host:26257/db?sslmode=verify-full"

# RAG pipeline
RAG_PIPELINE_ENABLED=true
EMBEDDING_PROVIDER=openrouter
EMBEDDING_MODEL=openai/text-embedding-3-small
```

**CockroachDB Cloud Basic (Serverless) Notes:**
- First-boot `LATEST.sql` DDL backfills take 25-60 min on Cloud Basic
- `CREATE UNIQUE INDEX` may time out silently, leaving tables without constraints
- The app verifies and repairs missing indexes at startup (`verifyCockroachIndexes`)
- Use Cloud Standard for production to avoid slow DDL
- See `fly_cockroach.toml` for Fly.io deployment config

### Configuration Priority

```
1. Tenant Config (Agent Admin UI) → Highest priority
         ↓ (if empty)
2. Environment Variable → Fallback
         ↓ (if empty)
3. Hardcoded Default → Lowest priority
```

---

## API Endpoints

### Public
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/agent/:slug/chat` | Send chat message |
| GET | `/api/v1/agent/:slug/chat/stream` | SSE stream |

### Auth (Public)
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/tenants` | List tenants for multi-tenant user |
| POST | `/api/v1/auth/select-tenant` | Select tenant, return JWT with tenant_id |

### Admin (Authenticated)
| Method | Path | Description | Permission |
|--------|------|-------------|------------|
| GET/POST | `/api/v1/agent/tenants` | List/Create tenants | tenant:admin |
| GET/PUT/DELETE | `/api/v1/agent/:slug` | CRUD tenant | tenant:admin |
| POST | `/api/v1/agent/:slug/files` | Upload KB/Policy/Script | files:upload |
| POST | `/api/v1/agent/:slug/reindex` | Rebuild RAG index | api:config |
| POST | `/api/v1/agent/:slug/simulate` | Run simulation | chat:test |
| GET | `/api/v1/agent/:slug/simulations` | List simulations | chat:test |

---

## Permission System (RBAC)

| Permission | Description |
|------------|-------------|
| `tenant:admin` | Full tenant management |
| `tenant:read` | View tenant configuration |
| `api:config` | Configure LLM settings, rebuild index |
| `chat:test` | Run simulations, view history |
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

## Security Guidelines (Tenant Isolation)

### DO

- ✅ Always extract tenant ID from context in handlers
- ✅ Apply `ApplyTenantFilter(c, find)` before database queries
- ✅ Verify tenant ownership before update/delete operations
- ✅ Check `!isSuperUser(user)` before tenant ownership checks
- ✅ Use `GetTenantIDFromContext(ctx)` in gRPC handlers
- ✅ Use `getTenantFromContext(c)` in Echo handlers
- ✅ Set tenant ID on create operations for tenant-scoped data

### DON'T

- ❌ Never hardcode tenant IDs in queries
- ❌ Never skip tenant filtering for "convenience"
- ❌ Never expose tenant IDs in error messages
- ❌ Never allow cross-tenant data access (even for superusers without explicit check)
- ❌ Never store PII (like tenant_id) in plaintext descriptions
- ❌ Never trust user-supplied tenant IDs from request body

### Superuser Bypass Pattern

```go
tenantID := getTenantFromContext(c)
if tenantID != nil && item.TenantID != nil && *item.TenantID != *tenantID && !isSuperUser(user) {
    return echo.NewHTTPError(http.StatusForbidden, "permission denied")
}
```

---

## Adding Tenant-Scoped Features

When adding a new tenant-scoped feature, follow this checklist:

### 1. Database Layer
- [ ] Add `tenant_id INTEGER DEFAULT NULL` column (if new table)
- [ ] Add index on `tenant_id`
- [ ] Add `TenantID *int32` to store structs

### 2. Store Layer
- [ ] Add `tenant_id` to SQL queries (INSERT, SELECT, WHERE, UPDATE)
- [ ] Update CEL filter if applicable (remove `tenant_id` from CEL identifiers)

### 3. Handler Layer
- [ ] Extract tenant ID from context
- [ ] Set tenant ID on create operations
- [ ] Apply tenant filter on list operations
- [ ] Verify tenant ownership on update/delete operations

### 4. Security
- [ ] Add superuser bypass check
- [ ] Test cross-tenant access denial
- [ ] Verify no PII leakage in error messages

---

## Multi-Tenant Authentication

### Single-Tenant Users
- gRPC `SignIn` auto-selects the single tenant
- JWT includes `tenant_id`

### Multi-Tenant Users
- gRPC `SignIn` returns error: "multiple tenants found, use /auth/tenants endpoint"
- Must use REST flow:
  1. `POST /api/v1/auth/tenants` → Returns tenant list + selection token
  2. `POST /api/v1/auth/select-tenant` → Returns JWT with `tenant_id`

### REST SignIn — Nil Tenant (By Design)
`POST /api/v1/auth/signin` (REST) always generates a JWT with `tenant_id: nil`, even for single-tenant users. This is **by design** — REST sign-in creates an unscoped session. The tenant must be selected separately via `POST /api/v1/auth/select-tenant`. This ensures the multi-tenant selection flow is consistent regardless of user type.

### Selection Token
- Random 32-byte string stored in `user_access_token`
- 5-minute expiry enforced via timestamp in description
- Single-use (deleted after successful selection)
- **P0 fix:** Token lookup uses O(1) hash-indexed table (`user_access_token_lookup`) instead of N+1 user scan

---

## Adding New Features

### Backend Pattern (Go)

**1. Define types in [`store/agent.go`](store/agent.go):**
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

**2. Add interface methods to [`store/driver.go`](store/driver.go):**
```go
CreateMyNewType(ctx context.Context, item *MyNewType) (*MyNewType, error)
GetMyNewType(ctx context.Context, find *FindMyNewType) (*MyNewType, error)
ListMyNewTypes(ctx context.Context, find *FindMyNewType) ([]*MyNewType, error)
DeleteMyNewType(ctx context.Context, id string) error
```

**3. Implement in `store/db/sqlite/agent.go`:**

**4. Add handlers in [`handlers.go`](server/router/api/v1/agent/handlers.go):**

**5. Register routes in `server/router/api/v1/v1.go`:**

### Database Migrations

**Primary workflow:** `task migrate:new NAME=add_widget_config`

Creates SQLite and Postgres migration file templates. Write SQL for each driver manually.
See `docs/DOCS_DATABASE_MIGRATION_GUIDE.md` and `docs/TYPE_MAPPING.md` for full reference.

Location: `store/migration/sqlite/<version>/` and `store/migration/postgres/<version>/`

Naming: `NN__snake_case_description.sql`

Validation commands:
- `task validate:parity` — cross-driver schema + file-list parity
- `task validate:schema` — schema validation tests
- `./scripts/validate-migrations.sh` — LATEST.sql drift check

### Frontend Pattern (React + MobX)

**1. Add types and state to store (e.g., `web/src/store/v2/agentAdmin.ts`):**
```typescript
export interface MyNewType {
  id: string;
  tenantId: number;
}

class LocalState {
  myNewItems: MyNewType[] = [];
  isLoadingMyNewItems: boolean = false;
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
    });
  } catch (error: any) {
    // Handle error
  }
};
```

**3. Add UI component in page:**

**4. Add translations to `web/src/locales/en.json`:**

---

## Key Files Reference

| File | Purpose |
|------|---------|
| [`handlers.go`](server/router/api/v1/agent/handlers.go) | HTTP request handlers |
| [`service.go`](server/router/api/v1/agent/service.go) | Business logic, LLM integration |
| [`parser.go`](server/router/api/v1/agent/parser.go) | KB/Policy/Script parsing |
| [`chunker.go`](server/router/api/v1/agent/chunker.go) | Document chunking for RAG |
| [`vectordb.go`](server/router/api/v1/agent/vectordb.go) | Vector database interface |
| [`embedding.go`](server/router/api/v1/agent/embedding.go) | Embedding providers |
| [`observer.go`](server/router/api/v1/agent/observer.go) | Observational memory |
| [`simulation.go`](server/router/api/v1/agent/simulation.go) | Agent simulation |
| [`auth.go`](server/router/api/v1/auth.go) | JWT claims, token generation |
| [`auth_service.go`](server/router/api/v1/auth_service.go) | Auth endpoints, tenant selection |
| [`acl.go`](server/router/api/v1/acl.go) | gRPC auth interceptor, context extraction |
| [`tenant_context.go`](server/router/api/v1/tenant_context.go) | Echo tenant context helpers |
| [`memo_service.go`](server/router/api/v1/memo_service.go) | Memo CRUD with tenant scoping |
| [`ticket_service.go`](server/router/api/v1/ticket_service.go) | Ticket CRUD with tenant scoping |
| [`store/agent.go`](store/agent.go) | Data types and store interface |
| [`store/driver.go`](store/driver.go) | Database driver interface |

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

### Tenant Isolation
- Always extract tenant ID from context (never hardcode)
- Apply `ApplyTenantFilter(c, find)` before database queries
- Verify tenant ownership for update/delete operations
- Use wrapper functions (`ApplyTenantFilter`, `ApplyTicketTenantFilter`) for consistency
- Remove `tenant_id` from CEL filter identifiers (SQLite + Postgres)

---

## Common Tasks

### 1. Add New Tenant Configuration Option
- Update [`store/agent.go`](store/agent.go) with new field
- Add migration in `store/migration/sqlite/`
- Update [`parser.go`](server/router/api/v1/agent/parser.go) if needed
- Update [`service.go`](server/router/api/v1/agent/service.go) to use the new field

### 2. Add New RAG Feature
- Update [`chunker.go`](server/router/api/v1/agent/chunker.go) for new chunking strategy
- Update [`vectordb.go`](server/router/api/v1/agent/vectordb.go) for new search method
- Update [`embedding.go`](server/router/api/v1/agent/embedding.go) for new provider
- Add environment variables to config

### 3. Add New API Endpoint
- Add handler in [`handlers.go`](server/router/api/v1/agent/handlers.go)
- Register route in `server/router/api/v1/v1.go`
- Add to frontend in `web/src/`

### 4. Debug RAG Issues
```bash
# Check RAG is enabled
grep "RAG pipeline" build/memos.log

# Check indexing
grep "Indexed content" build/memos.log

# Test retrieval
curl -X POST "http://localhost:8081/api/v1/agent/:slug/rag/search" \
  -H "Content-Type: application/json" \
  -d '{"query": "test"}'
```

---

## Database Queries

```bash
# List tenants
sqlite3 build/data/memos_dev.db "SELECT id, slug, company_name FROM agent_tenants;"

# Get tenant by slug
sqlite3 build/data/memos_dev.db "SELECT * FROM agent_tenants WHERE slug='inc';"

# List source files for tenant
sqlite3 build/data/memos_dev.db "SELECT id, audience_type, file_type, length(content), version FROM agent_source_files WHERE tenant_id=4 ORDER BY file_type, version DESC;"

# Get latest KB content
sqlite3 build/data/memos_dev.db "SELECT content FROM agent_source_files WHERE tenant_id=<ID> AND file_type='kb' ORDER BY id DESC LIMIT 1;"

# Get latest POLICY content
sqlite3 build/data/memos_dev.db "SELECT content FROM agent_source_files WHERE tenant_id=<ID> AND file_type='policy' ORDER BY id DESC LIMIT 1;"

# Get SCRIPT content
sqlite3 build/data/memos_dev.db "SELECT content FROM agent_tenant_scripts WHERE tenant_id=<ID> ORDER BY id DESC LIMIT 1;"

# List all tables
sqlite3 build/data/memos_dev.db ".tables"

# Show table schema
sqlite3 build/data/memos_dev.db ".schema agent_tenants"
```

---

## Testing Endpoints

```bash
# Validate tenant
curl http://localhost:5230/api/v1/agent/inc/validate

# Reindex tenant (needs auth cookie)
curl -X POST http://localhost:5230/api/v1/agent/inc/reindex

# RAG stats
curl http://localhost:5230/api/v1/admin/rag/stats

# Test RAG search
curl -X POST http://localhost:5230/api/v1/admin/rag/search \
  -H "Content-Type: application/json" \
  -d '{"query": "water damage", "tenant_id": 4, "limit": 5}'
```

---

## Useful Grep Patterns

```bash
# Find handler
grep -n "func.*Handle" server/router/api/v1/agent/handlers.go

# Find route registration
grep -n "adminGroup\|authGroup" server/router/api/v1/v1.go

# Find store method
grep -n "func.*Store" store/agent.go

# Find translation key usage
grep -rn "agent-admin.my-key" web/src/
```

---

## Gotchas

| Issue | Solution |
|-------|----------|
| Table name wrong | Use **plural**: `agent_tenants` not `agent_tenant` |
| Env vars not working in Taskfile | Use inline: `VAR=value ./binary` not `env:` block |
| Frontend state not updating | Wrap in `runInAction()` |
| Store method not accessible | Add to return object |
| Mock embeddings not semantic | Use `openrouter` instead |
| Migration not running | Check filename: `NN__snake_case.sql` |
| CGO errors | Run `task setup:lancedb` first |
| CRDB Cloud Basic: DDL backfill timeout | First-boot `LATEST.sql` can take 25-60 min on Cloud Basic (Serverless). `CREATE UNIQUE INDEX` may time out silently, leaving tables without constraints. The app now verifies and repairs missing indexes at startup (`verifyCockroachIndexes` in `store/migrator.go`). Use Cloud Standard for production to avoid slow DDL. |

---

## MD File Versioning Protocol

When updating KB.MD, POLICY.MD, or SCRIPT.MD files for a tenant:

### Step 1: Get Latest Version from Database
```bash
# Get latest KB content
sqlite3 build/data/memos_dev.db "SELECT content FROM agent_source_files WHERE tenant_id = <TENANT_ID> AND file_type = 'kb' ORDER BY id DESC LIMIT 1;"
```

### Step 2: Save with UNIX Epoch Suffix
```bash
EPOCH=$(date +%s)
# Save as KB_<epoch>.MD, POLICY_<epoch>.MD, SCRIPT_<epoch>.MD
```

### Step 3: Make Surgical Updates
- **DO NOT** rewrite entire files
- Make targeted, surgical changes only
- Preserve existing structure and formatting

### Step 4: Create Updated Draft
Save updated content to `*_DRAFT.MD` files for review before uploading.

---

## Known Limitations

See [`docs/DOCS_RAG_MINIMAX25.MD`](docs/DOCS_RAG_MINIMAX25.MD) for detailed analysis:

| Category | Issue | Severity |
|----------|-------|----------|
| Architecture | Token limit hardcoded at 30K | Critical |
| Performance | Embedding provider reliability | High |
| Scalability | Vector database memory limits | High |
| Data Quality | Hybrid search score normalization | Medium |
| Temporal | Fixed decay parameters | Medium |
| Memory | No eviction policy | Medium |

---

## Documentation

Start with these files:
- [`docs/DOCS_README.MD`](docs/DOCS_README.MD) - Comprehensive project overview
- [`docs/DOCS_ENV_VAR.MD`](docs/DOCS_ENV_VAR.MD) - Environment variables
- [`docs/DOCS_TASKFILE.MD`](docs/DOCS_TASKFILE.MD) - Build commands
- [`docs/DOCS_AGENT_ARCHITECTURE.MD`](docs/DOCS_AGENT_ARCHITECTURE.MD) - Configuration-driven design

RAG-specific:
- [`docs/DOCS_RAG_PIPELINE.MD`](docs/DOCS_RAG_PIPELINE.MD) - RAG configuration
- [`docs/DOCS_HYBRID_SEARCH.MD`](docs/DOCS_HYBRID_SEARCH.MD) - Hybrid search
- [`docs/DOCS_RAG_MINIMAX25.MD`](docs/DOCS_RAG_MINIMAX25.MD) - Limitations analysis

---

## Getting Help

1. Check [`docs/DOCS_README.MD`](docs/DOCS_README.MD) first
2. Review [`GEMINI.MD`](GEMINI.MD) for design principles
3. Search docs folder: `ls docs/ | grep <keyword>`
4. Check [`docs/CHANGELOG.MD`](docs/CHANGELOG.MD) for recent changes
