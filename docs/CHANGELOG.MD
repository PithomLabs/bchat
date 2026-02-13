# Changelog

All notable changes to the bchat AI Agent System are documented in this file.

---

## 2026-02-13

### Added
- **Token-Based Observer Trigger** - Observer now triggers based on token count instead of message count
  - New config option `OM_OBSERVER_TOKEN_THRESHOLD` (default: 30,000 tokens, matching Mastra specification)
  - Consistent triggering regardless of message verbosity (verbose vs terse conversations)
  - Replaces deprecated `OM_MESSAGE_THRESHOLD` for Observer trigger
  - Files: `om_config.go`, `service.go`, `DOCS_ENV_VAR.MD`

- **Current Task & Suggested Response Extraction** - Observer output now parsed for continuity hints
  - Extracts `<current-task>` and `<suggested-response>` XML tags from Observer LLM output
  - Stored in new database columns `current_task` and `suggested_response`
  - Injected into system prompt for agent continuity across sessions
  - Migration: `26__om_task_response.sql`
  - Files: `observer.go`, `service.go`, `store/agent.go`, `agent_observations.go`

- **Retry Logic Integration** - Observer and Reflector LLM calls now use retry logic
  - Uses existing `withRetry()` function with configurable attempts and delay
  - Reduces transient error failures
  - Files: `observer.go:140-145`, `observer.go:184-189`

### Documentation
- **DOCS_OM_GLM5.MD** - Comprehensive gap analysis of OM implementation vs Mastra specification
  - Critical gaps identified: MySQL/PostgreSQL support, token-based trigger, async buffering
  - Feature gaps: current task extraction, resource scope, prompt caching
  - Implementation completeness score: ~65%

- **DOCS_OM_ENHANCEMENT_PLAN.md** - Detailed implementation plan for OM enhancements
  - Phase 1: Token-based trigger (completed)
  - Phase 2: Task/response extraction (completed)
  - Phase 3: Async buffering (planned)
  - Phase 4: Resource scope (planned)
  - Phase 5: Prompt caching (planned)

### Changed
- **LATEST.sql** - Updated agent_observations table schema
  - Added `current_task TEXT` column
  - Added `suggested_response TEXT` column
  - Updated table comment to reflect migration 26 enhancement

### Fixed
- **Unused PromptCache Code** - Removed dead code from observer.go
  - Deleted unused `PromptCache` struct, `promptCache` variable, `hashString()` function
  - Removed unused `sync` import
  - Code was defined but never integrated into LLM calls

---

## 2026-02-06

### Fixed
- **Dimension Mismatch Error Messages** - Clear, actionable error messages when embedding model changes
  - Previous: Cryptic LanceDB error `query dim(4096) doesn't match column dim(384)`
  - Now: Clear message explaining dimension mismatch and directing user to rebuild index
  - Added dimension validation in `Search()` method before querying LanceDB
  - Checks both query embedding dimension and table schema dimension
  - Files: `server/router/api/v1/agent/vectordb_lance.go`

### Added
- **TableName() Method** - Returns LanceDB table name for debugging dimension issues
  - Useful for troubleshooting which dimension-specific table is being queried
  - File: `server/router/api/v1/agent/vectordb_lance.go:735`

- **GetEmbeddingDimension() Method** - Returns current VectorDB embedding dimension
  - Thread-safe accessor for debugging embedding configuration
  - File: `server/router/api/v1/agent/service.go:126`

- **RefreshVectorDB() Method** - Recreates VectorDB with current environment configuration
  - Useful for development when switching embedding providers without restart
  - Closes old VectorDB and creates new one from current env vars
  - File: `server/router/api/v1/agent/service.go:138`

---

## 2026-02-03

### Changed
- **Simplified RAG Chunking** - Replaced complex structured parsing with markdown-based unstructured chunking
  - Disabled KB/Policy parser entirely - no more structured table population
  - `importFiles()` now uses simple markdown chunking instead of `ParseKBWithResult`/`ParsePolicyWithResult`
  - `indexContentForRAG()` always uses `ChunkMarkdownContent()` regardless of content structure
  - Let embeddings and hybrid search handle relevance instead of content type classification
  - Files: `handlers.go`, `chunker.go`, `vectordb.go`

- **Increased Chunk Size for OpenRouter** - Optimized for `openai/text-embedding-3-small` (8191 token limit)
  - `GetMaxChunkTokens("openrouter")` increased from 4000 to 7000 tokens (85% of 8191 limit)
  - Reduces chunk count by ~43% for large KB files (from ~11000 to ~6500 chunks)
  - File: `chunker.go:78`

### Removed
- **Deprecated Chunker Functions** - Removed ~620 lines of unused structured chunking code
  - Deleted `ChunkKBContent()` - used structured annotation parsing
  - Deleted `ChunkPolicyContent()` - used structured annotation parsing
  - Deleted `ChunkFromStoreTypes()` - direct database chunking
  - Deleted `ChunkRawContent()` - redundant wrapper around `ChunkMarkdownContent()`
  - Deleted `splitIntoParagraphs()` - helper only used by `ChunkRawContent`
  - Deleted 8 `build*Content()` helper functions (Service, FAQ, Exclusion, Coverage, Safety, KBSection, Rule, Intent)
  - Deleted `GetTextsForEmbedding()` - unused utility function
  - Removed unused `store` import from chunker.go
  - File reduced from ~1188 lines to 569 lines (~52% reduction)
  - File: `chunker.go`

### Fixed
- **UTF-8 Sanitization for LanceDB** - Fixed "Invalid UTF8 sequence" errors during RAG indexing
  - Added `sanitizeUTF8()` function using `strings.ToValidUTF8()` in `chunker.go`
  - Sanitize all 6 string fields in `chunksToArrowRecord()` before Arrow serialization
  - Prevents LanceDB IPC serialization failures from corrupted content
  - Files: `chunker.go:537-545`, `vectordb_lance.go:627-635`

- **LanceDB Table Recreation** - Fixed "Table not found" error when recreating table for dimension change
  - `dropAndRecreateTable()` now handles missing table gracefully
  - Allows switching embedding models without manual intervention
  - File: `vectordb_lance.go:265-272`

### Added
- **Unstructured File Upload Support** - Auto-detection for structured vs unstructured content
  - `ParseKBResult` and `ParsePolicyResult` structs now include `IsStructured` flag and `ParsedCount`
  - `StructuredContentThreshold` constant (2 items) determines structured vs unstructured classification
  - `ChunkRawContent()` method in chunker.go for arbitrary text chunking
  - `HasStructuredContent` field in `AudienceConfig` for runtime mode selection
  - `AudienceInfo` response now includes `KBIsStructured`, `PolicyIsStructured`, `HasStructuredContent` fields
  - Files: `parser.go`, `chunker.go`, `handlers.go`, `service.go`

### Changed
- **Automatic RAG Mode for Unstructured Content** - System now intelligently handles plain text uploads
  - `importFiles()` detects structured vs unstructured content during parsing
  - `processChat()` forces RAG mode when `HasStructuredContent=false`
  - `buildRAGSystemPrompt()` handles minimal identity for unstructured content
  - Enables use case: Upload a novel/document and chat about it without annotation requirements
  - Files: `handlers.go:953-1114`, `service.go:1264-1270`

### Documentation
- **DOCS_RAG_OVERVIEW.MD** - Comprehensive RAG architecture overview document
  - Content types (structured vs unstructured)
  - Auto-detection mechanism and processing flow
  - Configuration options and behavior matrix
  - Verification and troubleshooting guide

---

## 2026-02-01

### Changed
- **Simplified Widget URLs** - Removed GUID requirement from widget embed URLs
  - Widget URLs now use simple slug format: `/widget/bchat/embed.js`
  - Previously used slug-guid format: `/widget/bchat-xxxxxxxx-xxxx/embed.js`
  - Removed GUID suffix from widget header title (was showing "CompanyName-{guid}")
  - GUID field remains in database but is no longer used for widget routing
  - Updated widget-test files to use simple slug format
  - Files: `server/router/api/v1/agent/handlers.go`, `server/router/api/v1/v1.go`, `web/src/pages/AgentAdmin.tsx`, `build/widget-test/static/*.html`

- **LATEST.sql Sync** - Added missing columns from migrations 22-23 to `agent_tenants` table
  - Added `guid TEXT` column for widget URL security
  - Added `allowed_domains TEXT` column for domain allowlisting
  - Added `idx_agent_tenants_guid` index
  - File: `store/migration/sqlite/LATEST.sql`

- **Widget Test URLs** - Updated widget-test to use new `slug-guid` URL format
  - Changed `/widget/rizal/...` to `/widget/rizal-YOUR_GUID_HERE/...`
  - Updated simple.html, iframe.html, and index.html
  - Added note about replacing placeholder with actual tenant GUID
  - Files: `build/widget-test/static/*.html`

### Fixed
- **Widget HTTPS Scheme Detection** - Fixed widget embed.js generating HTTP URLs instead of HTTPS
  - Added `X-Forwarded-Proto` header check for reverse proxy environments (fly.io)
  - Previously, `c.Request().TLS == nil` always returned true behind reverse proxy
  - Fixed in 3 locations: HandleWidget, HandleWidgetEmbed, HandleWidgetIframe
  - File: `server/router/api/v1/agent/handlers.go`

### Added
- **Widget Bundle in Docker Deployment** - Widget now built and included in fly.io deployments
  - Added widget build step to Stage 1 (frontend) in Dockerfiles
  - Added `COPY --from=frontend /widget-build/dist ./widget/dist` to runtime stage
  - Files: `Dockerfile.fly`, `Dockerfile.s3.fly`, `Dockerfile.local.fly`

- **Widget Test Server** - Simple Go server for testing external widget embedding
  - Simulates third-party website embedding the chat widget
  - Test pages: simple embed (JS injection) and iframe wrapper
  - Location: `build/widget-test/`
  - Run: `cd build/widget-test && go run main.go` → http://localhost:8888
  - Documentation: `docs/DOCS_TEST_WIDGET.MD`

- **Tenant GUID for Widget Security** - Added unique GUID to tenant records
  - New `guid` field auto-generated on tenant creation (UUID v4)
  - Existing tenants received GUIDs via migration
  - Widget URLs now use `slug-guid` format: `/widget/rizal-abc123-def456/embed.js`
  - Makes widget URLs harder to guess, adding security through obscurity
  - Migration: `store/migration/sqlite/0.25/22__tenant_guid.sql`
  - Files: `store/agent.go`, `store/db/sqlite/agent.go`, `handlers.go`

- **Optional Domain Allowlisting** - Restrict widget to approved domains only
  - New `allowed_domains` field in tenant table (JSON array)
  - When enabled, widget/API rejects requests from unlisted domains
  - Supports wildcards: `*.example.com` matches any subdomain
  - Checks both `Origin` and `Referer` headers for widget requests
  - Checks `Origin` header for chat API requests
  - UI: Checkbox toggle + domain list textarea in Agent Admin
  - Empty list = no restrictions (default, backwards compatible)
  - Migration: `store/migration/sqlite/0.25/23__tenant_allowed_domains.sql`
  - Files: `handlers.go`, `AgentAdmin.tsx`, `agentAdmin.ts`

---

## 2026-01-31

### Changed
- **Tenant-Scoped Chat Agent Responses** - Agent now only answers queries relevant to current tenant
  - Added SCOPE OF KNOWLEDGE section to system prompt defining tenant boundaries
  - Agent politely declines queries about other businesses or out-of-scope topics
  - Enhanced CRITICAL REMINDER section with tenant-only representation
  - Updated RAG pipeline constraints with tenant scoping rules
  - Prevents hallucination by encouraging honest "I don't have that information" responses
  - File: `server/router/api/v1/agent/service.go` (buildSystemPrompt, buildRAGSystemPrompt)

- **Enhanced `fly:check` Pre-Deployment Validation** - Now validates full environment chain
  - Validates: `.env` -> `Dockerfile` -> `fly.toml` -> `fly secrets`
  - Reads Dockerfile specified in `fly.toml [build].dockerfile` (supports local vs s3)
  - Checks both variable names AND default values match across chain
  - Context-aware: skips `LANCEDB_LOCAL_PATH` for s3 builds, skips S3 vars for local builds
  - Security check: ensures `OPENROUTER_API_KEY` and `ENCRYPTION_MASTER_KEY` are NOT in Dockerfile or fly.toml
  - New script: `scripts/validate-env-chain.sh`
  - Updated documentation in `DOCS_TASKFILE.MD`

### Added
- **Pre-Deployment Database Migration Check** - `task fly:db-check` command
  - Comprehensive validation that migrations will work correctly on fly.io
  - Step 1: Runs existing `validate:migrations` (LATEST.sql sync check)
  - Step 2: Validates migration file numbering (no gaps, proper sequence)
  - Step 3: Tests LATEST.sql can create a fresh database
  - Step 4: Tests all migrations can apply to an empty database in sequence
  - Step 5: Compares final schema after migrations vs LATEST.sql
  - New script: `scripts/validate-db-migrations.sh`

- **Combined Pre-Deployment Check** - `task fly:pre-deploy` command
  - Runs both `fly:check` (env sync) and `fly:db-check` (database validation)
  - Single command to validate before `fly deploy`
  - Reports overall pass/fail status

### Fixed
- **Migration 03 SQLite UNIQUE Constraint Error** - Fixed "Cannot add a UNIQUE column" error
  - SQLite doesn't support `ALTER TABLE ADD COLUMN ... UNIQUE`
  - Fixed by recreating tickets table with all columns including `beads_id TEXT UNIQUE`
  - Migration now uses proper SQLite table recreation pattern (create new → copy → drop → rename)
  - File: `store/migration/sqlite/0.25/03__beads_integration.sql`

- **Migration 06/12/20 Version Column Ordering** - Fixed "no such column: version" error
  - Migration 12 tried to create index on `version` column before it existed
  - Fixed by adding `version` column in migration 06 when table is recreated
  - Migration 20 converted to no-op (column now added in 06)
  - Files: `store/migration/sqlite/0.25/06__agent_file_versioning.sql`, `20__agent_source_file_version_fix.sql`

- **Q&A Pairs Cross-Tenant Data Leak** - Fixed brief display of previous tenant's Q&A pairs when switching tenants
  - Bug: `qaPairs` and `qaTestResults` state not cleared in `fetchTenantConfig()`
  - Symptom: Q&A pairs from Tenant A briefly visible when switching to Tenant B
  - Fix: Added `qaPairs: []` and `qaTestResults: null` to initial state reset
  - File: `web/src/store/v2/agentAdmin.ts:465-471`
  - Note: Backend SQL queries were already correctly filtered by `tenant_id`

### Added
- **Pre-Deployment Env Sync Check** - `task fly:check` command
  - Compares `.env` variables against `fly.toml [env]` and `fly secrets`
  - Reports missing variables with clear status indicators
  - Run before `fly deploy` to catch configuration drift
  - Updated `Taskfile.yml` with new task

- **EMBEDDING_TIMEOUT Environment Variable** - Configurable embedding API timeout
  - Default: `180s` (3 minutes)
  - Accepts Go duration strings (e.g., `60s`, `3m`, `5m`)
  - Fixes timeout issues with large KB content during "Rebuild Index"
  - Documented in `DOCS_ENV_VAR.MD`

- **RAG Reindex Resume Feature** - Resume reindexing from where errors occurred
  - New `agent_reindex_checkpoints` database table tracks progress
  - Checkpoint-based progress tracking (batch number, processed chunks, status)
  - Exponential backoff retry logic (3 retries, 5s initial delay)
  - Resume API: `POST /api/v1/agent/:slug/reindex?resume=true`
  - Status API: `GET /api/v1/agent/:slug/reindex/status`
  - Handles transient OpenRouter API failures gracefully
  - Design document: `docs/DOCS_REINDEX_RESUME.MD`

### Changed
- **DOCS_ENV_VAR.MD** - Added deployment sync documentation
  - New "Local vs Production Environment Variables" section
  - Pre-deployment sync checklist referencing `task fly:check`
  - Variable placement guide (fly.toml vs fly secrets)
  - Updated to version 1.4

- **DOCS_DEPLOYMENT.MD** - Added `.env` sync workflow
  - "Sync .env to fly.io Before Deployment" section
  - Example output from `task fly:check`
  - Quick sync commands reference
  - Updated to version 1.2

- **DOCS_TASKFILE.MD** - Added `fly:check` task documentation
  - Added to Quick Reference table
  - Full documentation in Utility Commands section
  - Updated to version 1.4

---

## 2026-01-30

### Added
- **fly.io Deployment Documentation** - Comprehensive deployment guide
  - `docs/DOCS_DEPLOYMENT.MD` - Step-by-step fly.io deployment procedures
  - Architecture diagrams (local vs fly.io)
  - Configuration mapping table (local `task run:rag:l12` → fly.io)
  - First-time setup instructions (flyctl, Tigrisdata, secrets, volumes)
  - Verification checklist and troubleshooting guide
  - Redeployment commands reference
  - Addendum explaining Docker build + migration flow

- **Database Migration Documentation** - Deep dive into migration system
  - `docs/DOCS_DATABASE_MIGRATION.MD` - How `go:embed` works with migrations
  - Flow diagrams for new vs existing databases
  - LATEST.sql vs individual migrations explanation
  - Root cause analysis of fly.io schema issues
  - Best practices for adding migrations

- **Migration Validation Script** - Build-time sync check
  - `scripts/validate-migrations.sh` - Validates LATEST.sql against migrations
  - Checks all `CREATE TABLE` statements exist in LATEST.sql
  - Checks all `ALTER TABLE ADD COLUMN` statements exist in LATEST.sql
  - Ignores temporary tables (`_new`, `_old`, `_temp`)
  - Integrated into Taskfile as dependency for `build:backend` and `build:backend:rag`

- **Ubuntu-based Dockerfiles for fly.io**
  - `Dockerfile.local.fly` - Ubuntu 24.04 with local volume storage for LanceDB
  - `Dockerfile.s3.fly` - Ubuntu 24.04 with Tigrisdata S3 storage for LanceDB
  - Both use `sentence-transformers/all-minilm-l12-v2` embedding model

- **fly.toml Templates**
  - `fly.local.toml` - Configuration for local volume storage
  - `fly.s3.toml` - Configuration for Tigrisdata S3 storage
  - Updated `fly.toml` to use local storage by default

- **Deployment Setup Script**
  - `~/Desktop/b.sh` - Interactive setup script for fly.io prerequisites
  - Checks flyctl installation and authentication
  - Prompts for storage type (local vs S3)
  - Creates Tigrisdata bucket (S3 mode)
  - Sets required secrets (OPENROUTER_API_KEY, ENCRYPTION_MASTER_KEY)
  - Creates fly volume for SQLite

### Fixed
- **LATEST.sql Out of Sync** - Updated with all missing tables and columns
  - Added `processing_options` column to `agent_tenants` table
  - Fixed `notification` → `notifications` table name
  - Added missing columns to `tickets` table (beads_id, parent_id, labels, etc.)
  - Added `agent_workflows` table
  - Added `agent_simulation_transcripts` table
  - Added `agent_analysis_results` table
  - Added `agent_learning_memory` table
  - Added `agent_compliance_audits` table
  - Added `agent_scoring_config` table
  - Added `user_tenant_permission` table
  - Added `tenant_config` table with all columns
  - Added `system_secret` table
  - Added `agent_simulations` table
  - Added `agent_qa_pairs` table
  - Added `agent_transcripts` table
  - Updated `agent_tenant_scripts` with summary and version columns

- **Missing axios Dependency** - Added axios to `web/package.json`
  - Frontend build was failing with "failed to resolve import axios"
  - Added `"axios": "^1.7.7"` to dependencies

### Changed
- **Taskfile.yml** - Added migration validation
  - New task: `validate:migrations`
  - Added as dependency to `build:backend` and `build:backend:rag`
  - Builds now fail if LATEST.sql is out of sync with migrations

---

## 2026-01-29

### Added
- **Tenant-Specific API Key** - OpenRouter API key textbox in Agent Admin
  - Encrypted storage using AES-256 with `ENCRYPTION_MASTER_KEY`
  - Show/hide toggle for security
  - Overrides environment variable when set (tenant config > env var > default)

- **Reasoning Model Configuration** - Per-tenant reasoning model setting
  - New textbox in Auto-Generate section for Generate KB/Policy operations
  - Default: `google/gemini-2.5-pro` (from `LLM_MODEL_REASONING` env var)
  - Database migration: `19__reasoning_model.sql`

- **Unified Environment Workflow** - Secure environment variable management
  - `.env.example` template with all variables documented
  - `.env` file support via Taskfile (auto-sourced on `task run*`)
  - `_FILE` suffix support in Docker entrypoint for all secrets
  - `docs/DOCS_UNIFIED_ENV_WORKFLOW.MD` comprehensive guide

- **DOCS_ADMIN_PAGE.MD** - Agent Admin page documentation
  - Feature descriptions for all sections
  - File upload behavior (no auto-reindex)
  - API endpoint reference

### Changed
- **Taskfile.yml** - All run tasks now source `.env` file if present
  - `task run`, `task run:rag`, `task run:rag:l12` auto-load environment
  - Pattern: `set -a && source .env && set +a`

- **scripts/entrypoint.sh** - Extended `_FILE` suffix support
  - Now supports: `MEMOS_DSN_FILE`, `OPENROUTER_API_KEY_FILE`, `ENCRYPTION_MASTER_KEY_FILE`, `AWS_ACCESS_KEY_ID_FILE`, `AWS_SECRET_ACCESS_KEY_FILE`

- **.gitignore** - Added comprehensive .env patterns
  - `.env`, `.env.local`, `.env.*.local`, `.env.development`, `.env.test`, `.env.production`

### Removed
- **Endpoints Section** - Removed from Agent Admin page
  - Displayed 3 static endpoint URLs (unused, cluttered UI)

---

## 2026-01-28

### Changed
- **Chat Widget Premium Redesign** - Complete visual overhaul with Intercom-style design
  - Gradient header with curved bottom wave effect
  - Pill-style input with rounded wrapper and circular send button
  - Asymmetric chat bubbles (user: bottom-right tail, assistant: bottom-left tail)
  - Smooth animations (slide-up panel, fade-in messages, bouncing typing dots)
  - Focus states with brand color ring on input
  - Premium shadows throughout
  - Full dark mode support (Facebook Messenger-inspired)
  - Updated files:
    - `widget/src/styles/styles.ts` - Complete CSS redesign
    - `widget/src/ui/Input.ts` - Added wrapper div for pill-style input
    - `widget/src/ui/Panel.ts` - Updated header structure
    - `widget/src/ui/Messages.ts` - Simplified message structure

### Added

- **Widget Embed Code Generator** - New section in Agent Admin
  - Color picker for widget branding
  - Position toggle (bottom-right/bottom-left)
  - Welcome message customization
  - Live preview panel showing widget appearance
  - Generated embed code with copy button
  - Auto-fills tenant slug and company name

### Fixed
- **Widget Dark Mode Text Color** - Message text invisible in dark mode
  - Bug: `.acw-msg-user .acw-msg-bubble` and `.acw-msg-assistant .acw-msg-bubble` set `color: #1c1e21`
  - Issue: Higher specificity overrode dark mode `.acw-msg-bubble { color: #ffffff }`
  - Fix: Removed redundant color declarations (lines 265-271 in styles.ts)
  - Result: Base `.acw-msg-bubble` color now properly overridden by dark mode media query

- **Duplicate RAG Indexing on File Upload** - KB files were indexed twice
  - Bug: `indexContentForRAG()` ran during import, then `ReindexTenantContent()` ran in background goroutine
  - Impact: 2x processing time, 2x embedding API costs
  - Fix: Removed redundant background reindex call from `HandleImportFiles()`
  - Added `DOCS_LOGS.MD` with detailed log analysis

### Changed
- **External Chat Widget Now Uses Internal Audience** - Cleaner content for customers
  - `ChatExternal()` now loads `internal` audience config instead of `external`
  - Rationale: "internal" = cleaned/production-ready, "external" = draft/staging
  - Both customers (via widget) and admins (via testing) now use same cleaned content
  - Rate limiting still tracked as "external" to protect public API

### Removed
- **Legacy `/api/v1/chat` Endpoint** - Dead code cleanup
  - Deleted `server/router/api/v1/chat_service.go` (117 lines)
  - Removed `RegisterChatRoutes()` call from `v1.go`
  - This generic Memos chat endpoint was unused (all chat uses tenant-aware agent endpoints)

- **Chat Sidebar Icon** - Consolidated navigation
  - Removed "Chat" link from sidebar navigation
  - Users should use "Internal Agent" page instead (same functionality)
  - `/chat` route still exists but is hidden from navigation

### Added
- **DOCS_LOGS.MD** - Log analysis documentation for duplicate indexing bug
- **DOCS_ENDPOINTS.MD** - Chat endpoint analysis and design documentation
  - Documents `/chat/ext` and `/chat/int` endpoints
  - Explains audience design (draft vs published content)
  - Records changes made and rationale

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
