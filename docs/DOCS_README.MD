# bchat AI Agent System

**Comprehensive Documentation**

**Status:** Active Development
**Last Updated:** 2026-01-23

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Multi-Tenant Agent System](#multi-tenant-agent-system)
4. [RAG Pipeline](#rag-pipeline)
5. [Features](#features)
6. [Getting Started](#getting-started)
7. [Configuration](#configuration)
8. [API Reference](#api-reference)
9. [Deployment](#deployment)
10. [Limitations](#limitations)
11. [Related Documentation](#related-documentation)

---

## Overview

bchat is a multi-tenant AI chat agent platform built on top of the Memos application. It provides a configurable, general-purpose conversational AI system where each tenant can define their own:

- **Knowledge Base (KB.MD)** - Services, FAQs, product information
- **Policy (POLICY.MD)** - Rules, identity, tone, allowed intents
- **Conversation Script (SCRIPT.MD)** - Conversation flow and stages

The system uses OpenRouter for LLM access and includes an optional RAG (Retrieval-Augmented Generation) pipeline powered by LanceDB for improved response accuracy.

### Key Principles

1. **General-Purpose Agent** - The codebase contains no tenant-specific logic. All customization happens through configuration files.
2. **Configuration-Driven** - Tenant behavior is entirely defined by KB, Policy, and Script files.
3. **Multi-Tenant Isolation** - Each tenant has isolated data, configurations, and chat sessions.

---

## Architecture

### Technology Stack

| Layer | Technology |
|-------|------------|
| **Backend** | Go 1.21+, Echo framework |
| **Database** | SQLite (default), PostgreSQL, MySQL supported |
| **Frontend** | React 18, TypeScript, MobX, Vite |
| **UI Components** | Joy UI (@mui/joy) |
| **LLM Provider** | OpenRouter API |
| **Vector Database** | LanceDB (optional, for RAG) |
| **Embeddings** | OpenRouter text-embedding-3-small, local sentence-transformers, or mock |

### Directory Structure

```
bchat/
├── bin/memos/              # Application entry point
├── build/                  # Build output directory
│   └── data/               # Runtime data (SQLite DB, LanceDB indexes)
├── docs/                   # Documentation
├── server/
│   └── router/api/v1/
│       └── agent/          # Agent API handlers and services
│           ├── handlers.go # HTTP request handlers
│           ├── service.go  # Business logic, LLM integration
│           ├── parser.go   # KB/Policy/Script parsing
│           ├── vectordb.go # LanceDB operations
│           ├── embedding.go# Embedding providers
│           └── chunker.go  # Document chunking
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
└── Taskfile.yml            # Build and run commands
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

---

## Multi-Tenant Agent System

### Tenant Model

Each tenant represents an isolated agent configuration:

| Field | Description |
|-------|-------------|
| `id` | Unique identifier |
| `slug` | URL-friendly identifier (e.g., "acme-corp") |
| `name` | Display name |
| `llm_model` | LLM model override (optional) |
| `temperature` | Response temperature (0.0-1.0) |
| `created_at` | Creation timestamp |

### Configuration Files

#### KB.MD (Knowledge Base)

Contains factual information the agent can reference:

```markdown
# Company Information
- Company Name: Acme Corp
- Founded: 2010
- Headquarters: New York

# Services
## Service A
- Description: ...
- Pricing: ...

# FAQs
## Q: What are your hours?
A: We're open Monday-Friday, 9am-5pm EST.
```

#### POLICY.MD (Agent Policy)

Defines agent behavior, identity, and rules:

```markdown
# Identity
- Name: Acme Assistant
- Role: Customer Service Representative

# Tone
- Professional but friendly
- Empathetic to customer concerns

# Rules
- Never discuss competitor products
- Always verify customer identity before account changes
- Escalate to human for refund requests over $500

# Intents
- GREETING: Customer says hello
- FAQ: General questions
- SUPPORT: Technical issues
- ESCALATE: Request human agent
```

#### SCRIPT.MD (Conversation Flow)

Defines conversation stages and transitions:

```markdown
# Conversation Flow

## Stage: Opening
- Greet the customer
- Ask how you can help today

## Stage: Discovery
- Understand the customer's need
- Ask clarifying questions

## Stage: Resolution
- Provide solution or information
- Confirm customer satisfaction

## Stage: Closing
- Thank the customer
- Offer additional assistance
```

### Permission System (RBAC)

| Permission | Access Level |
|------------|--------------|
| `tenant:admin` | Full tenant management |
| `tenant:read` | View tenant configuration |
| `api:config` | Configure LLM settings, rebuild index |
| `chat:test` | Run simulations, view history |
| `chat:logs` | View real chat session logs |
| `files:upload` | Upload KB/Policy/Script files |

---

## RAG Pipeline

### Overview

The RAG (Retrieval-Augmented Generation) pipeline enhances agent responses by:

1. **Chunking** documents into smaller pieces with overlap
2. **Embedding** chunks into vector representations
3. **Storing** vectors in LanceDB for similarity search
4. **Retrieving** relevant chunks based on user query
5. **Augmenting** the LLM prompt with retrieved context

### Architecture

```
Document Upload (KB.MD, POLICY.MD)
     │
     ▼
Chunker (chunker.go)
     │ Split into ~500 token chunks with 50 token overlap
     ▼
Embedding Service (embedding.go)
     │ Generate 1536-dimension vectors
     ▼
LanceDB (vectordb.go)
     │ Store vectors with metadata
     ▼
[Query Time]
     │
User Query → Embed → Vector Search → Top-K Chunks → LLM Prompt
```

### Embedding Providers

| Provider | Use Case | Dimensions | API Key Required |
|----------|----------|------------|------------------|
| `openrouter` | Production | 1536 | Yes |
| `local` | Development | 384 | No (needs local server) |
| `mock` | Testing | 1536 | No |

### Storage Providers

| Provider | Use Case | Configuration |
|----------|----------|---------------|
| `local` | Development, single-instance | `LANCEDB_LOCAL_PATH` |
| `s3` | Production, fly.io | `LANCEDB_S3_*` variables |

### Hybrid Search

The RAG pipeline supports **hybrid search** combining vector similarity with BM25 keyword matching:

```
Query → Embedding → Vector Search (semantic) ─┐
                                              ├─► Score Fusion → Results
Query → Tokenization → BM25 Search (keyword) ─┘
```

| Mode | Use Case | Configuration |
|------|----------|---------------|
| Vector-only | Semantic understanding | `HYBRID_SEARCH_ENABLED=false` |
| Hybrid | Best of both | `HYBRID_SEARCH_ENABLED=true` |

**Default Weights:** 70% vector similarity, 30% BM25 keyword matching

See [DOCS_HYBRID_SEARCH_EXPLAINED.MD](./DOCS_HYBRID_SEARCH_EXPLAINED.MD) for technical details.

### RAG vs Non-RAG

| Feature | Without RAG | With RAG |
|---------|-------------|----------|
| Build command | `task build` | `task build:rag` |
| Run command | `task dev:backend` | `task run:rag` |
| KB/Policy handling | Full content in prompt | Chunked + retrieved |
| Response quality | Good | Better (grounded) |
| Dependencies | None | LanceDB native libs |

---

## Features

### Agent Admin Interface

Web-based administration for managing tenants:

- **Tenant Management** - Create, edit, delete tenants
- **File Upload** - Upload KB.MD, POLICY.MD, SCRIPT.MD
- **LLM Configuration** - Set model, temperature per tenant
- **User Permissions** - Assign RBAC permissions
- **Rebuild Index** - Trigger RAG re-indexing

### Chat Interface

Real-time chat with streaming responses:

- **SSE Streaming** - Responses stream as they're generated
- **Conversation History** - Full session history preserved
- **Intent Logging** - Extracted intents logged for analysis

### Simulation System

Test agent behavior without real customers:

- **Virtual Customer** - LLM-powered customer simulator
- **Scenario Configuration** - Define customer persona and goals
- **SSE Streaming** - Watch conversation unfold in real-time
- **Transcript Storage** - All simulations saved for review

### Transcript Analysis

Analyze conversation quality:

- **Benchmark Scoring** - Rate conversations against criteria
- **Issue Identification** - Highlight problem areas
- **Improvement Suggestions** - Actionable recommendations

---

## Getting Started

### Prerequisites

- Go 1.21+
- Node.js 18+
- Task (taskfile.dev)
- OpenRouter API key

### Quick Start (No RAG)

```bash
# 1. Install dependencies
task setup

# 2. Set environment
export OPENROUTER_API_KEY=sk-or-v1-xxx

# 3. Build
task build

# 4. Run
./build/memos --mode dev --data build/data
```

### Quick Start (With RAG)

```bash
# 1. Install dependencies
task setup

# 2. Set environment
export OPENROUTER_API_KEY=sk-or-v1-xxx

# 3. Build with RAG support
task build:rag

# 4. Run with RAG enabled
task run:rag

# Or with mock embeddings (no API key needed for embeddings)
task run:rag:mock
```

### Development Mode

```bash
# Terminal 1: Backend
task dev:backend

# Terminal 2: Frontend (hot reload)
task dev:frontend
```

### Access Points

| Service | URL |
|---------|-----|
| Frontend | http://localhost:5173 (dev) or http://localhost:8081 |
| Backend API | http://localhost:8081/api/v1/ |
| Agent Admin | http://localhost:5173/agent-admin |

---

## Configuration

### Environment Variables

See `docs/DOCS_ENV_VAR.MD` for complete reference.

#### Required

```bash
OPENROUTER_API_KEY=sk-or-v1-xxx    # OpenRouter API key
```

#### LLM Configuration

```bash
LLM_MODEL=openai/gpt-4o-mini       # Default LLM model
```

#### RAG Configuration

```bash
RAG_PIPELINE_ENABLED=true          # Enable RAG pipeline
EMBEDDING_PROVIDER=openrouter      # openrouter|local|mock
EMBEDDING_MODEL=text-embedding-3-small
LANCEDB_STORAGE_PROVIDER=local     # local|s3
LANCEDB_LOCAL_PATH=build/data/lancedb
```

#### Startup Flags

```bash
REINDEX_RAG=true                   # Re-index all tenants on startup
```

### Configuration Priority

```
Tenant Config (Agent Admin UI)
         │
         ▼ (if not set)
Environment Variable
         │
         ▼ (if not set)
Hardcoded Default
```

---

## API Reference

### Public Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/agent/:slug/chat` | Send chat message |
| GET | `/api/v1/agent/:slug/chat/stream` | SSE chat stream |

### Admin Endpoints (Authenticated)

| Method | Path | Description | Permission |
|--------|------|-------------|------------|
| GET | `/api/v1/agent/tenants` | List tenants | tenant:read |
| POST | `/api/v1/agent/tenants` | Create tenant | tenant:admin |
| GET | `/api/v1/agent/:slug` | Get tenant details | tenant:read |
| PUT | `/api/v1/agent/:slug` | Update tenant | tenant:admin |
| DELETE | `/api/v1/agent/:slug` | Delete tenant | tenant:admin |
| POST | `/api/v1/agent/:slug/files` | Upload file | files:upload |
| POST | `/api/v1/agent/:slug/reindex` | Rebuild RAG index | api:config |
| POST | `/api/v1/agent/:slug/simulate` | Run simulation | chat:test |
| GET | `/api/v1/agent/:slug/simulations` | List simulations | chat:test |

---

## Deployment

### Local Development

```bash
task build
./build/memos --mode dev --data build/data
```

### Production Build

```bash
task build:rag
```

### fly.io Deployment

See `Dockerfile.fly` for container configuration.

```bash
# Build and deploy
fly deploy

# Set secrets
fly secrets set OPENROUTER_API_KEY=sk-or-v1-xxx
fly secrets set RAG_PIPELINE_ENABLED=true
fly secrets set EMBEDDING_PROVIDER=openrouter
fly secrets set LANCEDB_STORAGE_PROVIDER=s3
```

### Environment-Specific Configuration

| Environment | RAG | Storage | Embeddings |
|-------------|-----|---------|------------|
| Development | Optional | Local | mock or local |
| Staging | Yes | Local | openrouter |
| Production | Yes | S3 | openrouter |

---

## Limitations

### Current Limitations

1. **Single LLM Provider** - Only OpenRouter is supported. No direct OpenAI, Anthropic, or local LLM support.

2. **SQLite Default** - While PostgreSQL and MySQL drivers exist, SQLite is the primary tested database.

3. **No Batch Operations** - Simulations run one at a time. Batch simulation support is planned.

4. **English Only** - i18n infrastructure exists but only English translations are complete.

5. **No Analytics Dashboard** - Analysis results are per-transcript. Historical trends dashboard is planned.

6. **Widget Limitations** - External widget exists but has limited customization (see [DOCS_WIDGET.MD](./DOCS_WIDGET.MD)).

### Known Issues

1. **LanceDB on Linux** - Requires shared library (.so), not static library (.a), due to BSD/GNU ar incompatibility.

2. **Mock Embeddings** - Produce deterministic but semantically meaningless vectors. Only use for pipeline testing.

3. **SCRIPT.MD Not RAG-Indexed** - By design, SCRIPT.MD is included fully in the system prompt, not chunked.

4. **Hybrid Search Latency** - Enabling hybrid search adds ~20-50ms latency due to BM25 index lookup and score fusion.

### Response Validation

The system includes a two-layer validation system for response quality:

| Layer | Status | Description |
|-------|--------|-------------|
| Output Sanitization | Always Active | System tag removal, contact correction |
| LLM Verification | Optional | Semantic grounding against KB |

**Note:** The regex compliance layer was removed in Phase 5B. Audit functionality now uses LLM-based semantic verification instead of regex patterns.

See [DOCS_HALLUCINATION_GROUNDING.MD](./DOCS_HALLUCINATION_GROUNDING.MD) for details.

### Security Considerations

1. **API Key Exposure** - Ensure `OPENROUTER_API_KEY` is never committed to version control.

2. **RBAC Enforcement** - All admin endpoints require authentication and permission checks.

3. **Tenant Isolation** - Each tenant's data is isolated by `tenant_id` foreign keys with cascading deletes.

---

## Related Documentation

### Core Documentation

| Document | Description |
|----------|-------------|
| [CHANGELOG.MD](./CHANGELOG.MD) | Project changelog |
| [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) | Environment variables reference |
| [DOCS_TASKFILE.MD](./DOCS_TASKFILE.MD) | Build commands reference |
| [DOCS_AGENT_ARCHITECTURE.MD](./DOCS_AGENT_ARCHITECTURE.MD) | Configuration-driven architecture guide |

### RAG & Search

| Document | Description |
|----------|-------------|
| [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) | RAG architecture details |
| [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD) | LanceDB implementation plan |
| [DOCS_HYBRID_SEARCH.MD](./DOCS_HYBRID_SEARCH.MD) | Hybrid search implementation plan |
| [DOCS_HYBRID_SEARCH_EXPLAINED.MD](./DOCS_HYBRID_SEARCH_EXPLAINED.MD) | Hybrid search technical deep-dive |

### Quality & Evaluation

| Document | Description |
|----------|-------------|
| [DOCS_EVALUATION.MD](./DOCS_EVALUATION.MD) | Evaluation framework and metrics |
| [DOCS_HALLUCINATION_GROUNDING.MD](./DOCS_HALLUCINATION_GROUNDING.MD) | Hallucination detection and grounding |
| [DOCS_ROLLOUT.MD](./DOCS_ROLLOUT.MD) | Production rollout guide |

### Features

| Document | Description |
|----------|-------------|
| [DOCS_WIDGET.MD](./DOCS_WIDGET.MD) | External chat widget integration |
| [DOCS_SIMULATION.MD](./DOCS_SIMULATION.MD) | Simulation feature specification |

### For Claude Code Users

See `CLAUDE.md` in the project root for development patterns, code conventions, and contribution guidelines.

---

*Document Version: 1.0*
*Last Updated: 2026-01-23*
