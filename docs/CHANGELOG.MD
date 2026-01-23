# Changelog

All notable changes to the bchat AI Agent System are documented in this file.

---

## 2026-01-23

### Removed
- **compliance.go (Phase 5B)** - Removed regex-based compliance checker
  - Replaced with LLM-based semantic verification in `audit.go`
  - `audit.go` now uses `verifier.go` for service, contact, and exclusion compliance checks
  - Regex patterns cannot understand semantic meaning and had high false positive rates
  - Consistent with RAG philosophy: use LLM grounding, not regex patterns
  - Updated `DOCS_HALLUCINATION_GROUNDING.MD` to reflect Layer 1 removal

### Added
- **Rebuild Index Button** - Added "Rebuild Index" button to Agent Admin page for per-tenant RAG reindexing
  - New `ReindexTenantContent()` function in `service.go`
  - New `HandleReindexTenant` handler at `POST /api/v1/agent/:slug/reindex`
  - Frontend button with loading state and success/error toast notifications
  - Requires ADMIN role or `api:config` permission

- **Taskfile Run Commands** - Added convenience commands for running with RAG enabled
  - `task run:rag` - Run with OpenRouter embeddings (requires `OPENROUTER_API_KEY`)
  - `task run:rag:mock` - Run with mock embeddings (no API key needed)
  - Both commands automatically set `RAG_PIPELINE_ENABLED=true` and library paths

- **DOCS_TASKFILE.MD** - Comprehensive Taskfile commands documentation
  - Quick reference table
  - Detailed command descriptions
  - Common workflows
  - Troubleshooting guide
  - Build and Run matrices

- **DOCS_ENV_VAR.MD** - Environment variables documentation
  - Configuration priority documentation (Tenant Config > Env Var > Default)
  - Embedding provider settings
  - LanceDB storage configuration
  - RAG pipeline feature flags

- **DOCS_WIDGET.MD** - External chat widget integration guide
  - JavaScript embed approach documentation
  - API reference for widget.js and chat/ext endpoints
  - Customization options and CSS override examples
  - Security considerations and troubleshooting
  - React integration example

- **DOCS_PENDING.MD** - Pending work and known limitations tracking
  - Technical limitations and known issues
  - Technical debt and TODOs
  - Security considerations
  - Future enhancement plans

- **DOCS_HYBRID_SEARCH.MD** - Hybrid search implementation plan
  - Vector + BM25 keyword search design
  - LanceDB Tantivy FTS integration
  - Score fusion strategies (linear combination, RRF)
  - Testing strategy with quality metrics
  - Phased implementation plan

- **Hybrid Search Implementation (Week 1)** - Infrastructure for vector + BM25 search
  - Added `BM25Scorer` struct with standard BM25 parameters (k1=1.2, b=0.75)
  - Extended `SearchQuery` with `UseHybridSearch`, `VectorWeight`, `TextWeight` fields
  - Extended `SearchResult` with `SearchMode`, `VectorScores`, `BM25Scores` for debugging
  - Updated `MemoryVectorDB.Search()` to support hybrid search with on-the-fly BM25 indexing
  - Added FTS index creation (`idx_content_fts`, `idx_title_fts`) in LanceDB
  - Implemented `hybridSearch()` method in LanceVectorDB with linear combination score fusion
  - Created `vectordb_test.go` with 17 unit tests for BM25 scoring and hybrid search

- **Hybrid Search Implementation (Week 2)** - Environment configuration and integration
  - Added `HYBRID_SEARCH_ENABLED` environment variable to globally enable hybrid search
  - Added `HYBRID_VECTOR_WEIGHT` and `HYBRID_TEXT_WEIGHT` for configuring score fusion weights
  - Extended `VectorDBConfig` struct with hybrid search configuration fields
  - Added `parseFloatOrDefault()` helper for environment variable parsing
  - Updated `RetrieveContextForQuery()` to accept `HybridSearchOptions`
  - Service now logs hybrid search configuration on startup
  - Created `vectordb_lance_test.go` with LanceDB integration tests (requires `-tags "rag integration"`)
  - Added 3 new unit tests for environment configuration: `TestParseFloatOrDefault`, `TestVectorDBConfig_HybridSearchDefaults`, `TestVectorDBConfig_HybridSearchEnabled`

- **DOCS_HYBRID_SEARCH_EXPLAINED.MD** - Comprehensive hybrid search technical guide
  - Library analogy explaining semantic vs keyword search
  - Deep dive into vector search and cosine similarity
  - BM25 algorithm explanation with TF-IDF foundation
  - Score fusion (linear combination) with worked examples
  - Complete data flow diagram (ASCII art)
  - Configuration guide with tuning recommendations
  - Testing and verification instructions
  - Real-world query examples with score breakdowns
  - Technical reference with file/line numbers
  - Troubleshooting guide

- **DOCS_AGENT_ARCHITECTURE.MD** - Agent architecture flexibility documentation
  - Configuration-driven design principle explanation
  - Service business origins and why examples are service-focused
  - Annotation-based parsing system (`<!-- @type: params -->`)
  - Flexible architecture elements (multi-tenant, multi-audience, RAG)
  - Adaptation examples for Q&A, Technical Support, HR, and Legal systems
  - Configuration vs code changes guide
  - Anti-patterns to avoid

- **DOCS_EVALUATION.MD** - Hybrid search evaluation guide
  - Evaluation dataset creation with ground truth schema
  - Quality metrics definitions (Recall@K, Precision@K, MRR, NDCG)
  - Performance benchmarking methodology (latency, throughput, memory)
  - Test scenarios for semantic, keyword, and mixed queries
  - Step-by-step evaluation process
  - Interpretation guidelines for tuning weights
  - Future capabilities roadmap (hallucination detection, A/B testing)

- **DOCS_HALLUCINATION_GROUNDING.MD** - Hallucination detection and grounding guide
  - Three-layer validation system (regex, sanitization, LLM verification)
  - Output sanitizer patterns and contact correction
  - LLM-based semantic verification against KB
  - Violation types and severity levels (critical/high/medium/low)
  - Grounding score calculation methodology
  - VerificationMetrics for monitoring
  - Configuration guide for enabling verification
  - Integration points in service.go

- **DOCS_ROLLOUT.MD** - Hybrid search production rollout guide
  - Phase 1: Test tenant enablement with verification checklist
  - Phase 2: Metrics monitoring (quality, latency, errors) with alert thresholds
  - Phase 3: Gradual rollout stages (10% → 25% → 50% → 100%)
  - Rollback procedures (immediate and gradual)
  - Success criteria and go/no-go checklists
  - SQL commands for tenant-level feature flags
  - Emergency response procedures

### Fixed
- **Data Path Doubling** - Fixed `build/build/data` path error in Taskfile.yml
  - Changed `--data build/data` to `--data {{.ROOT_DIR}}/build/data` for absolute paths

---

## 2026-01-22

### Added
- **MockEmbedding Provider** - Added mock embedding implementation for testing
  - Generates deterministic pseudo-random vectors based on text hash
  - No API key required
  - Configured via `EMBEDDING_PROVIDER=mock`

- **REINDEX_RAG Feature** - Added re-indexing capability on startup
  - Set `REINDEX_RAG=true` to re-index all tenant content on server start
  - Useful when switching embedding providers

- **DOCS_RAG_PIPELINE.MD** - RAG pipeline documentation
  - Architecture overview
  - Configuration options
  - Provider setup guides

---

## 2026-01-21

### Added
- **LanceDB RAG Phase 2** - Indexing Pipeline implementation
  - `indexContentForRAG()` function called after KB/Policy file uploads
  - Full `LanceVectorDB` implementation with Insert, Delete, Search, Close, Stats
  - Document chunking with metadata preservation
  - Batch embedding support

- **Dockerfile.fly** - fly.io deployment configuration
  - Multi-stage build with LanceDB native libraries
  - Configurable for S3 storage backend

### Changed
- **Taskfile.yml** - Fixed Linux build configuration
  - Changed from static library (.a) to shared library (.so) for LanceDB
  - BSD/GNU ar incompatibility workaround

---

## 2026-01-20

### Added
- **Agent Simulation Feature** - Full simulation system with SSE streaming
  - Simulate conversations between agent and virtual customer
  - Stream responses in real-time
  - Store transcripts for analysis

- **Conversation History Dropdown** - View past simulations and real chats
  - Unified history view
  - Quick access to transcripts

- **SCRIPT.MD Support** - Conversation flow guide system
  - Define conversation stages and transitions
  - Configure opening scripts
  - Stored separately from RAG (not chunked)

- **Transcript Benchmark Analysis** - Analyze simulation transcripts
  - Score conversations against criteria
  - Identify improvement areas

- **Plain Text Response Formatting** - Removed markdown from agent responses
  - Cleaner output for chat widgets
  - Configurable per tenant

### Removed
- **Chat Session Logs Section** - Removed from Agent Admin page (redundant with conversation history)

---

## 2026-01-19

### Added
- **LanceDB RAG Phase 1** - Foundation implementation
  - `vectordb.go` - LanceDB connection and operations interface
  - `embedding.go` - Embedding service (local + OpenRouter providers)
  - `chunker.go` - Document chunking logic with overlap

- **NoOpVectorDB** - No-operation vector database for non-RAG builds
  - Allows code to compile without LanceDB dependencies
  - Returns empty results for all operations

---

## 2026-01-18

### Added
- **Multi-tenant Agent System** - Core agent infrastructure
  - Tenant management with slug-based routing
  - KB.MD, POLICY.MD, SCRIPT.MD file support
  - Permission-based access control (RBAC)

- **Agent Admin Page** - Full administration interface
  - Tenant list and detail views
  - File upload for KB/Policy/Script
  - LLM configuration (model selection, temperature)
  - User permission management

- **Chat API** - Real-time chat endpoint
  - SSE streaming responses
  - Conversation history tracking
  - Intent extraction and logging

---

## Earlier Development

### Core Platform
- Memos-based application foundation
- SQLite database with auto-migrations
- React frontend with MobX state management
- Go backend with Echo framework
- OpenRouter LLM integration via `go-openrouter`

---

*This changelog follows a date-based format. For detailed implementation specifications, see the corresponding DOCS_*.MD files in the docs/ directory.*
