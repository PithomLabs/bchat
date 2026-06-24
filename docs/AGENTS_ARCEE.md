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
| Vector Database | LanceDB (optional, for RAG) |
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
type Tenant struct {
    ID          int32   // Unique identifier
    Slug        string  // URL-friendly (e.g., "acme-corp")
    Name        string  // Display name
    LLMModel    string  // LLM model override
    Temperature float64 // Response temperature (0.0-1.0)
}
```

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

## RAG Pipeline

### Architecture

```
Document Upload → Chunker → Embedding → LanceDB → [Query Time]
                                                      ↓
User Query → Embed → Vector Search → Top-K → LLM Prompt
```

### Key Components

| File | Purpose |
|------|---------|
| [`chunker.go`](server/router/api/v1/agent/chunker.go) | Document chunking (~500 tokens with overlap) |
| [`embedding.go`](server/router/api/v1/agent/embedding.go) | Embedding providers (openrouter, local, mock) |
| [`vectordb.go`](server/router/api/v1/agent/vectordb.go) | Vector database interface |
| [`vectordb_lance.go`](server/router/api/v1/agent/vectordb_lance.go) | LanceDB implementation |

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
LANCEDB_STORAGE_PROVIDER=local
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

## Important Principles

1. **No Tenant-Specific Code** - All behavior comes from configuration files
2. **Generic Features** - Design features that work for ANY tenant type
3. **Configuration-First** - Add options to KB/Policy/Script before code
4. **Multi-Tenant Aware** - All queries must filter by tenant_id
5. **Idempotent Migrations** - Use `IF NOT EXISTS`

---

## Testing

### Manual Testing
```bash
# Start server
task run:rag

# Upload test files
# Navigate to http://localhost:5173/agent-admin

# Test chat
curl -X POST "http://localhost:8081/api/v1/agent/:slug/chat" \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello"}'
```

### RAG Testing
```bash
# Enable verbose logging
RAG_PIPELINE_ENABLED=true LOG_LEVEL=debug task run:rag

# Check embeddings
curl "http://localhost:8081/api/v1/admin/rag/stats"
```

---

## Documentation

Start with these files:
- `docs/DOCS_README.MD` - Comprehensive project overview
- `docs/DOCS_ENV_VAR.MD` - Environment variables
- `docs/DOCS_TASKFILE.MD` - Build commands
- `docs/DOCS_AGENT_ARCHITECTURE.MD` - Configuration-driven design

RAG-specific:
- `docs/DOCS_RAG_PIPELINE.MD` - RAG configuration
- `docs/DOCS_HYBRID_SEARCH.MD` - Hybrid search
- `docs/DOCS_RAG_MINIMAX25.MD` - Limitations analysis

---

## Known Limitations

See `docs/DOCS_RAG_MINIMAX25.MD` for detailed analysis:
- Token threshold hardcoded at 30K
- No dynamic weight adjustment for hybrid search
- Memory storage limited to ~10K chunks
- Binary classification for structured/unstructured content

---

## Getting Help

1. Check `docs/DOCS_README.MD` first
2. Review `CLAUDE.md` for design principles
3. Search docs folder: `ls docs/ | grep <keyword>`
4. Check `CHANGELOG.MD` for recent changes