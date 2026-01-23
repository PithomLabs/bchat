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

## Design Documents
- `docs/DOCS_CHAT_DESIGN_4_IMP_2.MD` - Main implementation specification
- `docs/DOCS_CHAT_DESIGN_4_IMP_2-PROGRESS.MD` - Implementation progress and pending items
- `docs/DOCS_SIMULATION.MD` - Agent simulation feature specification
- `docs/DOCS_RAG_PIPELINE.MD` - RAG pipeline architecture and configuration
- `docs/DOCS_LANCEDB.MD` - LanceDB RAG implementation plan
- `docs/DOCS_LANCEDB_PHASE1.MD` - Phase 1: Foundation (complete)
- `docs/DOCS_LANCEDB_PHASE2.MD` - Phase 2: Indexing Pipeline (complete)

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
EMBEDDING_PROVIDER=local|openrouter|mock
EMBEDDING_MODEL=text-embedding-3-small
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

## Recent Work (2026-01-23)
1. Added "Rebuild Index" button to Agent Admin for per-tenant RAG reindexing
2. Added `task run:rag` and `task run:rag:mock` Taskfile commands
3. Created DOCS_ENV_VAR.MD, DOCS_TASKFILE.MD, CHANGELOG.MD, DOCS_README.MD
4. Fixed data path handling in Taskfile.yml (absolute paths with {{.ROOT_DIR}})
5. Added MockEmbedding provider for testing without API keys
6. Added REINDEX_RAG startup flag for bulk re-indexing

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
| Phase 3 | Retrieval Pipeline - Hybrid search, intent-aware retrieval | 🔶 Pending |
| Phase 4 | Prompt Simplification - New template, refactor buildSystemPrompt() | 🔶 Pending |
| Phase 5 | Cleanup & Optimization - Remove compliance.go, grounding.go, optional verifier | 🔶 Pending |

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
EMBEDDING_PROVIDER=local|openrouter  # Default: local
EMBEDDING_MODEL=all-MiniLM-L6-v2  # For local provider

# Optional verifier
LLM_VERIFIER_ENABLED=true|false  # Default: false
```

### Design Decisions Confirmed
- **Embedding Model:** Local (testing) + OpenRouter API (production)
- **Index Storage:** Local filesystem (testing) + Tigrisdata S3 (production on fly.io)
- **Re-indexing:** On every file upload
- **Feature Flag:** Global environment variable
- **Compliance Checker:** Will be REMOVED (regex compliance)
- **LLM Verifier:** Keep as OPTIONAL safety net (disabled by default)
- **SCRIPT.MD:** Full content as system prompt, configurable per tenant

---

## Pending Work
- External chat widget React component
- Comprehensive test suite
- API documentation (OpenAPI/Swagger)
- Batch simulation runs
- Analysis history dashboard
- LanceDB RAG Phase 3-5 implementation (retrieval, prompt simplification, cleanup)

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
