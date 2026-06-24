# AGENTS.md - Agent Development Guide

**Purpose:** Comprehensive guide for AI agents working on the bchat codebase.

---

## Project Overview

bchat is a **multi-tenant AI chat agent platform** built on top of the Memos application. Each tenant can define their own knowledge base, policies, and conversation scripts without requiring code changes.

### Key Principle: Configuration-Driven Design

The chat agent must be **GENERAL PURPOSE**, not tenant-specific.

- **DO NOT:** Hardcode tenant-specific logic in the codebase
- **DO:** Design features that work for ANY tenant configuration

### Technology Stack

| Layer | Technology |
|-------|------------|
| Backend | Go 1.21+, Echo framework |
| Database | SQLite (default), PostgreSQL, MySQL |
| Frontend | React 18, TypeScript, MobX, Vite |
| LLM Provider | OpenRouter API |
| Vector DB | LanceDB (optional, for RAG) |

---

## Directory Structure

```
bchat/
├── bin/memos/              # Application entry point
├── build/                  # Build output
│   └── data/               # Runtime data (SQLite DB, LanceDB)
├── docs/                   # Documentation (START HERE)
├── server/
│   └── router/api/v1/
│       └── agent/          # Agent API handlers
│           ├── handlers.go # HTTP request handlers
│           ├── service.go  # Business logic, LLM integration
│           ├── parser.go   # KB/Policy/Script parsing
│           ├── vectordb.go # Vector database operations
│           ├── embedding.go# Embedding providers
│           ├── chunker.go  # Document chunking
│           └── observer.go # Observational memory
├── store/
│   ├── agent.go            # Data types and store interface
│   ├── driver.go           # Database driver interface
│   └── db/sqlite/          # SQLite implementation
├── web/
│   ├── src/                # React source
│   └── dist/               # Built frontend
└── Taskfile.yml            # Build commands
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
- `@service` - Service offerings
- `@faq` - Frequently asked questions
- `@exclusion` - Services not provided
- `@coverage` - Service areas
- `@safety` - Safety information

**Supported Policy annotations:**
- `@identity` - Agent identity
- `@intent` - Customer intents
- `@rule` - Behavioral rules
- `@thresholds` - Decision thresholds

---

## RAG Pipeline

### Architecture

```
Document Upload → Chunker → Embedding → LanceDB → [Query Time]
                                                      ↓
User Query → Embed → Vector Search → Top-K → LLM Prompt
```

### Embedding Providers

| Provider | Dimensions | Use Case | API Key |
|----------|------------|----------|---------|
| openrouter | 1536 | Production | Yes |
| local | 384 | Development | No |
| mock | 1536 | Testing | No |

### Storage Providers

| Provider | Use Case | Configuration |
|----------|----------|---------------|
| memory | Testing | None |
| local | Development | `LANCEDB_LOCAL_PATH` |
| s3 | Production | `LANCEDB_S3_*` variables |

### Hybrid Search

Combines vector similarity (70%) with BM25 keyword matching (30%):
```bash
HYBRID_SEARCH_ENABLED=true
```

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

### Common Tasks
```bash
task build:backend       # Backend only
task build:frontend     # Frontend only
task run:rag:l12        # L12 embeddings via OpenRouter
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
LLM_TEMPERATURE=0.7
```

### RAG Configuration
```bash
RAG_PIPELINE_ENABLED=true
EMBEDDING_PROVIDER=openrouter
EMBEDDING_MODEL=text-embedding-3-small
LANCEDB_STORAGE_PROVIDER=local
LANCEDB_LOCAL_PATH=build/data/lancedb
```

### Startup Flags
```bash
FORCE_REINDEX_ON_STARTUP=true  # Re-index all content
```

---

## API Endpoints

### Public
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/agent/:slug/chat` | Send chat message |
| GET | `/api/v1/agent/:slug/chat/stream` | SSE stream |

### Admin (Authenticated)
| Method | Path | Description |
|--------|------|-------------|
| GET/POST | `/api/v1/agent/tenants` | List/Create tenants |
| GET/PUT/DELETE | `/api/v1/agent/:slug` | CRUD tenant |
| POST | `/api/v1/agent/:slug/files` | Upload KB/Policy/Script |
| POST | `/api/v1/agent/:slug/reindex` | Rebuild RAG index |
| POST | `/api/v1/agent/:slug/simulate` | Run simulation |

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
```

**2. Add interface methods to `store/driver.go`:**
```go
CreateMyNewType(ctx context.Context, item *MyNewType) (*MyNewType, error)
GetMyNewType(ctx context.Context, find *FindMyNewType) (*MyNewType, error)
```

**3. Implement in `store/db/sqlite/agent.go`:**

**4. Add handlers in `server/router/api/v1/agent/handlers.go`:**

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

---

## Key Files Reference

| File | Purpose |
|------|---------|
| `handlers.go` | HTTP request handlers |
| `service.go` | Business logic, LLM integration |
| `parser.go` | KB/Policy/Script parsing |
| `chunker.go` | Document chunking for RAG |
| `vectordb.go` | Vector database interface |
| `embedding.go` | Embedding providers |
| `observer.go` | Observational memory |

---

## Common Tasks

### 1. Add New Tenant Configuration Option
- Update `store/agent.go` with new field
- Add migration in `store/migration/sqlite/`
- Update `parser.go` if needed for parsing
- Update `service.go` to use the new field

### 2. Add New RAG Feature
- Update `chunker.go` for new chunking strategy
- Update `vectordb.go` for new search method
- Update `embedding.go` for new provider
- Add environment variables to config

### 3. Add New API Endpoint
- Add handler in `handlers.go`
- Register route in `v1.go`
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
