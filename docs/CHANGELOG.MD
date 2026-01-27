# Changelog

All notable changes to the bchat AI Agent System are documented in this file.

---

## 2026-01-27

### Added
- **Chat Transcript Recording** - Persist chat conversations to database
  - New `agent_transcripts` table stores chat history with metadata
  - `record_transcripts` tenant setting (default: true) to enable/disable
  - Works for BOTH external (anonymous) AND internal (authenticated) chat
  - GET/DELETE endpoints for transcript management
  - Agent Admin UI section with:
    - Toggle checkbox for recording
    - Transcript list with customer info and intent
    - Full transcript viewer modal
    - Delete individual transcripts
  - Captures: messages, customer name/phone/email, intent, timestamps
  - Upsert logic: creates transcript on first message, updates on each subsequent message

### Fixed
- **Internal Chat Transcripts** - Added transcript saving to `ChatInternal`
  - Previously only `ChatExternal` saved transcripts
  - Now both chat endpoints save transcripts when recording is enabled

- **Fallback Markdown Chunking** - Automatic chunking for unannotated KB content
  - When KB.MD has no `<!-- @section -->` annotations, uses `ChunkMarkdownContent()`
  - Splits by H2 → H3 → paragraphs → sentences (hierarchical fallback)
  - Enables RAG for plain markdown files without manual annotation
  - Logs: `No KB annotations found, using markdown-based chunking`

### Removed
- **Cybertron Embedding Provider** - Removed native Go embedding service
  - Removed `github.com/nlpodyssey/cybertron` dependency
  - Removed `CYBERTRON_MODELS_DIR`, `CYBERTRON_WORKERS` environment variables
  - Removed `task run:rag:cybertron` and `task run:rag:cybertron:quality` commands
  - OpenRouter is now the default embedding provider (`EMBEDDING_PROVIDER=openrouter`)
  - Deleted DOCS_CYBERTRON.MD, DOCS_CYBERTRON_DEBUG.MD, DOCS_CYBERTRON_PARALLEL.MD

- **OpenRouter sentence-transformers Support** - GPU-accelerated MiniLM embeddings
  - `EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L6-v2` via OpenRouter
  - Added `getOpenRouterDimension()` helper for model dimension detection

- **DOCS_LANCEDB_CHATGPT.MD** - lanceEmbeddings vs bchat comparison analysis
  - Architecture comparison (Python vs Go)
  - Chunking strategy analysis
  - Adaptable techniques identification
  - Root cause analysis for unannotated KB query failures

- **DOCS_LANCE_EMBEDDING.MD** - Adapted lanceEmbeddings techniques
  - Fallback chunking implementation details
  - Heading-based splitting algorithm
  - Combined title+content embedding
  - Recommendations for encoding resilience, data inspection, progress streaming

- **DOCS_PROMPT.MD** - System prompt guide for KB formatting
  - How `ChunkMarkdownContent()` algorithm works
  - Optimal markdown structure for RAG
  - Ready-to-use LLM prompts for formatting raw content
  - Token limits by embedding provider
  - Before/after transformation examples

- **DOCS_CYBERTRON_PARALLEL.MD** - Parallel embedding implementation plan
  - Worker pool architecture diagram
  - Expected speedup calculations
  - Thread safety considerations

- **DOCS_CHANGES.MD** - Timestamped changes tracking document
  - Records implementation changes with timestamps
  - Includes problem, solution, and verification for each change

- **Q&A Pairs Testing System** - Embedding quality evaluation framework
  - Generate up to 50 Q&A pairs from KB content using LLM
  - Test retrieval accuracy with Recall@K metrics
  - API endpoints: generate, list, create, update, delete, test
  - Database table: `agent_qa_pairs` with difficulty, category, source tracking
  - DOCS_QA.MD design document

- **Chunk-Based Q&A Generation** - Fixed token limit errors for large KB files
  - Previous: Sent entire KB to LLM (failed on 485K token files, limit 131K)
  - Now: Chunks KB first, samples 25 chunks, generates pairs in batches
  - Each batch: 5 chunks × 10 pairs = fits within token limits
  - Q&A pairs now link to `source_chunk_id` for better retrieval testing
  - Added: `generateQAPairsFromChunks()`, `sampleChunksForQA()`, `generateQAPairsFromBatch()`

- **RAG Search Explorer** - Visual debugging tool for vector search
  - Interactive search interface in Agent Admin
  - Real-time results with visual similarity score bars (color-coded)
  - Shows matched keywords, content previews, latency
  - Top-K selector (3, 5, 10 results)
  - API endpoint: `POST /api/v1/agent/:slug/rag/search`
  - Frontend: SearchExplorer panel with collapsible UI

- **DOCS_QA_TESTING.MD** - Visual RAG testing proposal document
  - Search Explorer specification
  - Q&A Pair Debugger design (side-by-side comparison)
  - Embedding Visualizer concept (2D t-SNE/UMAP plots)
  - Implementation phases and API designs

- **Batched Reindex for Large Files** - Handles any size KB/POLICY files
  - Previous: All chunks processed at once (memory spikes, API timeouts)
  - Now: Processes chunks in batches of 100 with progress logging
  - Memory efficient: Only one batch in memory at a time
  - API friendly: Smaller embedding requests, less likely to timeout
  - Progress visibility: Server logs show batch progress (e.g., "batch 5/15, 500/1487 chunks")
  - Modified: `vectordb_lance.go` `Insert()` function

- **MiniLM-L12-v2 Taskfile Command** - Higher quality embedding option
  - Added `task run:rag:l12` command for L12 model via OpenRouter
  - Same 384 dimensions as L6 - no reindexing when switching
  - Better semantic quality (12 layers vs 6 layers)
  - Requires `OPENROUTER_API_KEY` environment variable

- **DOCS_RAG_PROCESS.MD** - Deep analysis of chat-to-RAG integration
  - Complete data flow from user question to LLM response
  - Confirms user questions ARE embedded for vector search
  - Documents two modes: RAG (retrieved chunks) vs Long Context (full KB)
  - Intent-driven retrieval mapping table
  - Key code locations with file:line references
  - Verification and debugging guide

- **DOCS_SAVE_TRANSCRIPT.MD** - Chat transcript recording feature plan
  - Persist external chat sessions to database (currently only in-memory)
  - Tenant-level toggle: "Record transcript of chat conversations" (default: enabled)
  - New `agent_transcripts` table with messages, metadata, customer info
  - API endpoints: list, view, delete transcripts
  - Agent Admin UI: checkbox setting + transcripts viewer section
  - Privacy considerations and future enhancements

### Fixed
- **Q&A Test Audience Mismatch** - Fixed "Not Found" on all Q&A tests
  - Bug: `testRetrievalForPair()` searched `"external"` audience
  - Issue: Q&A pairs generated from `"internal"` KB, index built for `"internal"`
  - Fix: Changed search audience to `"internal"` in handlers.go:4514
  - Result: Q&A tests now correctly search the indexed content

- **Q&A Pairs Frontend URL Mismatch** - Fixed 404 errors on Q&A endpoints
  - Bug: Frontend called `/api/v1/admin/agent/:slug/qa-pairs/...`
  - Issue: Backend routes registered at `/api/v1/agent/:slug/qa-pairs/...`
  - Fix: Removed `/admin` from all Q&A API URLs in agentAdmin.ts

- **Q&A Pairs Foreign Key Error** - Fixed table creation SQL error
  - Bug: Migration referenced `agent_tenant` (singular)
  - Issue: Actual table name is `agent_tenants` (plural)
  - Fix: Updated migration to use correct table name

- **BUG-002: Empty Vector Index Creation** - Fixed IVF-PQ index error on empty tables
  - Error: `Creating empty vector indices with train=False is not yet implemented`
  - Root cause: IVF-PQ indexes require training data
  - Solution: Deferred index creation to first `Insert()` operation
  - Added `hasVectorIndex` tracking and `ensureVectorIndex()` method
  - BTree and FTS indexes still created immediately (work on empty tables)

- **Schema-Based Dimension Detection** - Fixed dimension mismatch on empty tables
  - Previous: Queried data to detect dimensions (failed on empty tables)
  - Now: Queries table schema via `db.table.Schema()` (works always)
  - Extracts dimension from `arrow.FixedSizeListType` field
  - Auto-recreates table when dimension mismatch detected

### Changed
- **embedding.go** - Added parallel processing and OpenRouter model support
  - New imports: `runtime`, `strconv`, `sync`
  - Added `CybertronWorkers` to `EmbeddingConfig`
  - Added `workers` field to `CybertronEmbedding`
  - New methods: `embedParallel()`, `embedSequential()`
  - New helper: `getOpenRouterDimension()` for model dimension mapping

- **vectordb_lance.go** - Improved index and schema handling
  - `createIndexes()` now skips IVF-PQ (deferred to insert)
  - Added `ensureVectorIndex()` for lazy index creation
  - Rewrote `getTableEmbeddingDimension()` to use schema instead of data
  - Added `hasVectorIndex` field to track index state

- **handlers.go** - Added fallback chunking support
  - `indexContentForRAG()` now accepts raw content parameters
  - Added `hasKBContent()` and `hasPolicyContent()` helpers
  - Falls back to `ChunkMarkdownContent()` when no annotations found

- **DOCS_BUGS.MD** - Added BUG-002 documentation

---

## 2026-01-26

### Added
- **Cybertron Embedding Provider** - Native Go embedding service using nlpodyssey/cybertron
  - Zero external dependencies (no Python, no API calls)
  - Supports `sentence-transformers/all-MiniLM-L6-v2` (384 dim) and `all-mpnet-base-v2` (768 dim)
  - Auto-downloads models on first use to `~/.cybertron/models/`
  - Configured via `EMBEDDING_PROVIDER=cybertron`

- **Provider-Adaptive Chunk Sizing** - Automatic chunk size based on embedding provider
  - `GetMaxChunkTokens(provider)` returns optimal chunk size per provider
  - OpenRouter: 2000 tokens (8191 token limit)
  - Cybertron: 150 tokens (512 token limit with aggressive subword tokenization)
  - Prevents "sequence too long" errors across all providers

- **LanceDB Dimension Mismatch Auto-Fix** - Automatic table recreation on provider switch
  - Detects embedding dimension mismatch before insert
  - Auto-drops and recreates table with correct schema
  - Logs warning: `Embedding dimension mismatch detected, recreating table`
  - No manual intervention required when switching providers

- **Taskfile Command** - `task run:rag:cybertron` for offline RAG testing
  - Runs with Cybertron embeddings (no API key needed)
  - Sets `EMBEDDING_PROVIDER=cybertron` automatically

- **DOCS_CYBERTRON.MD** - Cybertron embedding implementation guide
  - Model specifications and limitations
  - Configuration options
  - Troubleshooting guide

- **DOCS_CYBERTRON_DEBUG.MD** - Sequence length error debugging documentation
  - Root cause analysis of 512 token limit issues
  - Three-layer fix (reduced MaxChunkTokens, sentence splitting, truncation safety)
  - Token estimation lessons (subword tokenization ~1.9-2.5 chars/token)
  - Provider-adaptive chunking solution

- **DOCS_RAG_ALTERNATIVES.MD** - RAG processing alternatives comparison
  - Provider comparison (OpenRouter vs Cybertron)
  - Chunk size by provider reference table
  - Pre-summarization alternatives (NotebookLM, API, Ollama)
  - Offline fallback strategy

- **DOCS_BUGS.MD** - Bug documentation file
  - BUG-001: LanceDB embedding dimension mismatch

### Fixed
- **Cybertron Sequence Length Errors** - Multiple iterations to fix "input sequence too long" errors
  - Reduced `MaxChunkTokens` from 800 → 400 → 250 → 150
  - Reduced `maxChars` truncation safety from 1800 → 1000 → 750
  - Added `splitBySentences()` for large paragraphs without `\n\n` breaks
  - Key insight: Subword tokenization is 40-60% more aggressive than `len/4` estimate

- **LanceDB Schema Mismatch** - Fixed "Append with different schema" error
  - Error occurred when switching from Cybertron (384 dim) to OpenRouter (1536 dim)
  - Added `getTableEmbeddingDimension()` to detect existing schema
  - Added `dropAndRecreateTable()` for automatic schema migration

### Changed
- **chunker.go** - Now accepts `maxTokens` parameter for provider-adaptive chunking
- **embedding.go** - Added 750 char truncation safety net for Cybertron
- **service.go** - Uses `GetMaxChunkTokens(embeddingProvider)` for chunk sizing
- **vectordb_lance.go** - Added dimension mismatch detection and auto-recreation

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
