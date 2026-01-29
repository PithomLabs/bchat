# Agent Admin Page Documentation

This document describes the Agent Admin page functionality and configuration options.

## Overview

The Agent Admin page (`/agent-admin`) allows administrators and authorized users to manage AI agent tenants, configure LLM settings, upload knowledge base and policy files, and test the RAG pipeline.

## Sections

### 1. Status Toggle

Toggle tenant active/inactive status. Inactive tenants will not respond to chat requests.

### 2. LLM Configuration

Configure the LLM models used by the agent.

| Field | Description | Storage |
|-------|-------------|---------|
| **Model** | Main LLM model for chat responses | `tenant_config.llm_model` |
| **Model for Human in Simulation** | LLM model for simulating human/customer role | `tenant_config.simulation_human_model` |
| **OpenRouter API Key** | Tenant-specific API key (encrypted) | `tenant_config.openrouter_api_key_encrypted` |

**API Key Priority:**
1. Tenant-specific API key (if set in Admin page)
2. Environment variable `OPENROUTER_API_KEY`

### 3. User Permissions

Grant or revoke user access to the tenant. Available permission presets:

| Preset | Permissions |
|--------|------------|
| Viewer | `tenant:read` |
| Tester | `tenant:read`, `chat:test` |
| Analyst | `tenant:read`, `chat:logs` |
| Editor | `tenant:read`, `tenant:write`, `files:upload` |
| Tenant Admin | `tenant:admin` |

### 4. SCRIPT.MD

Upload a conversation flow guide to structure agent interactions.

### 5. External/Internal Configuration

Upload and manage KB.MD and POLICY.MD files for external (customer-facing) and internal (employee-facing) audiences.

**File Upload Buttons:**
- External KB.MD
- External POLICY.MD
- Internal KB.MD
- Internal POLICY.MD

### 6. Auto-Generate & Format Content

Admin-only section for content processing.

| Feature | Description | LLM Used |
|---------|-------------|----------|
| **Reasoning Model** | Configure model for Generate KB/Policy | Stored in `tenant_config.reasoning_model` |
| **Generate KB.MD** | AI-converts raw content to annotated KB format | Uses Reasoning Model |
| **Generate POLICY.MD** | AI-converts raw content to annotated Policy format | Uses Reasoning Model |
| **Format for RAG** | Rule-based content processing (no LLM) | None - pure text processing |

**Reasoning Model Priority:**
1. Tenant-specific model (if set in Admin page)
2. Environment variable `LLM_MODEL_REASONING`
3. Hardcoded default: `google/gemini-2.5-pro`

## Important: File Upload Does NOT Trigger Rebuild Index

**Key Behavior:** Uploading KB.MD or POLICY.MD files does **NOT** automatically rebuild the RAG index.

When you upload a file, the system:
1. Parses and stores the file content
2. Calculates token count
3. Determines retrieval mode (long_context vs RAG) based on content size
4. Invalidates the config cache

**To index content for RAG search, you must explicitly click "Rebuild Index".**

This design is intentional:
- Allows batch uploads without triggering expensive re-indexing
- Gives administrators control over when indexing occurs
- Prevents accidental re-indexing during iterative content updates

### Rebuild Index Button

Located below the External Configuration section:
- Click to re-generate embeddings and rebuild the RAG search index
- Only indexes **internal** audience content (external uses long_context mode)
- Requires `api:config` permission

### Retrieval Mode

The system automatically determines retrieval mode based on content size:

| Mode | Condition | Description |
|------|-----------|-------------|
| `long_context` | Content < threshold tokens | Full KB+Policy sent in system prompt |
| `rag` | Content >= threshold tokens | RAG retrieval for relevant chunks |

The mode is stored in `tenant_config.retrieval_mode` and updated on file upload.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENROUTER_API_KEY` | Global OpenRouter API key | Required |
| `LLM_MODEL_REASONING` | Model for Generate KB/Policy | `google/gemini-2.5-pro` |
| `RAG_PIPELINE_ENABLED` | Enable RAG features | `false` |
| `EMBEDDING_PROVIDER` | Embedding provider | `openrouter` |

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/agent/:slug/config` | Get tenant configuration |
| POST | `/api/v1/agent/:slug/import` | Upload KB/Policy file |
| POST | `/api/v1/agent/:slug/reindex` | Rebuild RAG index |
| GET | `/api/v1/agent/:slug/llm-config` | Get LLM configuration |
| PUT | `/api/v1/agent/:slug/llm-config` | Update LLM configuration |
| POST | `/api/v1/agent/:slug/generate-kb` | Generate annotated KB |
| POST | `/api/v1/agent/:slug/generate-policy` | Generate annotated Policy |
| POST | `/api/v1/agent/:slug/format-for-rag` | Format content for RAG |

## Database Schema

### tenant_config table

| Column | Type | Description |
|--------|------|-------------|
| `tenant_id` | INTEGER | Foreign key to agent_tenants |
| `llm_model` | TEXT | Main chat model |
| `simulation_human_model` | TEXT | Model for simulation human role |
| `reasoning_model` | TEXT | Model for Generate KB/Policy |
| `openrouter_api_key_encrypted` | BLOB | Encrypted API key |
| `openrouter_api_key_nonce` | BLOB | Encryption nonce |
| `retrieval_mode` | TEXT | "long_context" or "rag" |
| `content_tokens` | INTEGER | Estimated token count |
| `record_transcripts` | INTEGER | Whether to record chat transcripts |
