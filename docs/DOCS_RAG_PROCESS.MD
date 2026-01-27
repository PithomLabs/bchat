# RAG Pipeline Process - Deep Analysis

**Status:** Active
**Date:** 2026-01-27
**Related Documents:**
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG pipeline configuration
- [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD) - LanceDB implementation
- [DOCS_CYBERTRON.MD](./DOCS_CYBERTRON.MD) - Cybertron embedding provider

---

## Overview

This document provides a deep analysis of how the chat page integrates with the RAG (Retrieval-Augmented Generation) pipeline. It answers the question: **Does the user's question get embedded and used for semantic retrieval?**

**Answer: YES** - When properly configured, the user's question IS embedded and used for vector similarity search to retrieve relevant context before generating a response.

---

## When RAG Is Active

RAG mode is enabled when ALL three conditions are met:

| Condition | Check | Location |
|-----------|-------|----------|
| RAG Pipeline Enabled | `RAG_PIPELINE_ENABLED=true` | Environment variable |
| VectorDB Initialized | `vectorDB != nil && !isNoOp` | service.go:1582-1589 |
| Tenant Retrieval Mode | `tenantConfig.RetrievalMode == "rag"` | service.go:990-992 |

```go
// service.go:987-996
useRAG := false
if s.UseRAGPipeline() {
    tenantConfig, _ := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{
        TenantID: &config.TenantID,
    })
    if tenantConfig != nil && tenantConfig.RetrievalMode == "rag" {
        useRAG = true
    }
}
```

---

## Two Chat Modes

| Mode | System Prompt Content | Token Usage | When Used |
|------|----------------------|-------------|-----------|
| **RAG** | Only retrieved chunks | ~2-5K tokens | `RetrievalMode == "rag"` |
| **Long Context** | Entire KB.MD | ~50-500K tokens | Default, or RAG fallback |

**RAG Mode Benefits:**
- Smaller token usage (lower cost)
- Faster response generation
- More focused context (reduces hallucination)
- Intent-driven retrieval (relevant chunks only)

**Long Context Mode Benefits:**
- Simpler setup (no VectorDB needed)
- Full KB always available
- No retrieval failures possible

---

## Complete Data Flow

```
                              CHAT REQUEST
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│  POST /api/v1/agent/:slug/chat/ext (external)                       │
│  POST /api/v1/agent/:slug/chat/int (internal)                       │
│  handlers.go:92-177                                                 │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│  service.ChatExternal() or service.ChatInternal()                   │
│  service.go:813-902                                                 │
│  ├─ LoadConfig(tenantSlug, audienceType)                           │
│  ├─ CheckRateLimit()                                               │
│  └─ GetOrCreateSession()                                           │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│  service.processChat()                                              │
│  service.go:905-1090                                               │
│  ├─ Add user message to session history                            │
│  ├─ extractCollectedInfo() - name, phone, email                    │
│  ├─ ScoreUserMessage() - urgency/sentiment                         │
│  ├─ classifyIntent() - "service_inquiry", "booking_request", etc.  │
│  └─ evaluatePolicy() - check rules and safety triggers             │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
                    ┌──────────────────────────────┐
                    │  UseRAGPipeline() &&         │
                    │  RetrievalMode == "rag"?     │
                    │  service.go:987-996          │
                    └──────────────────────────────┘
                           │              │
                     YES   │              │  NO
                           ▼              ▼
         ┌─────────────────────┐   ┌─────────────────────┐
         │ generateRAGResponse │   │  generateResponse   │
         │ service.go:1594     │   │  service.go:1100    │
         └─────────────────────┘   └─────────────────────┘
                   │                        │
                   ▼                        ▼
┌─────────────────────────────┐   ┌─────────────────────────────┐
│ RetrieveContextForQuery()   │   │ buildSystemPrompt()         │
│ vectordb.go:637             │   │ service.go:1200             │
│                             │   │ ► FULL KB.MD in prompt      │
│ ► USER QUESTION EMBEDDED    │   │ ► No retrieval needed       │
│ ► Vector + BM25 search      │   └─────────────────────────────┘
│ ► Intent-driven filtering   │
└─────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│  db.Search(SearchQuery)                                             │
│  vectordb_lance.go (LanceDB) or vectordb.go (Memory)               │
│                                                                     │
│  1. embedSvc.Embed([]string{userQuestion})                         │
│     └─ Returns 384D or 768D vector                                 │
│                                                                     │
│  2. Vector similarity search (cosine distance)                     │
│     └─ Find nearest chunks in embedding space                      │
│                                                                     │
│  3. BM25 text search (if hybrid enabled)                           │
│     └─ Keyword relevance scoring                                   │
│                                                                     │
│  4. Score fusion (vector_weight * vec + text_weight * bm25)        │
│     └─ Default: 0.7 vector + 0.3 BM25                              │
│                                                                     │
│  5. Filter by tenantID, audienceType, contentTypes, minScore=0.3   │
│                                                                     │
│  6. Return top-K chunks (3-5 based on intent)                      │
└─────────────────────────────────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│  buildRAGSystemPrompt()                                             │
│  service.go:1680-1892                                               │
│                                                                     │
│  SECTION 1: IDENTITY                                               │
│  └─ Role, company name, tone, brand voice                          │
│                                                                     │
│  SECTION 2: CUSTOMER CONTEXT                                       │
│  └─ Name, phone, email, address from session                       │
│                                                                     │
│  SECTION 3: CONSTRAINTS                                            │
│  └─ "Only discuss services in RETRIEVED CONTEXT below"             │
│                                                                     │
│  SECTION 4: CONVERSATION GUIDE (SCRIPT.MD)                         │
│  └─ Conversation flow structure                                    │
│                                                                     │
│  ► SECTION 5: RETRIEVED CONTEXT ◄ (RAG RESULTS HERE)              │
│  ├─ SERVICES: (retrieved.Services)                                 │
│  ├─ FAQS: (retrieved.FAQs)                                         │
│  ├─ COVERAGE: (retrieved.Coverage)                                 │
│  ├─ NOT PROVIDED: (retrieved.Exclusions)                           │
│  ├─ POLICIES: (retrieved.Rules)                                    │
│  ├─ SAFETY: (retrieved.Safety)                                     │
│  └─ INFORMATION: (retrieved.KBSections)                            │
│                                                                     │
│  SECTION 6: ACTIVE RULES & SAFETY                                  │
│  SECTION 7: EMERGENCY FLAG                                         │
│  SECTION 8: LEARNED BEHAVIORS                                      │
│  SECTION 9: FORMAT INSTRUCTIONS                                    │
└─────────────────────────────────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│  OpenRouter LLM Call                                                │
│  service.go:1655-1660                                               │
│                                                                     │
│  Messages: [                                                        │
│    SystemMessage(ragSystemPrompt),  // With retrieved context      │
│    UserMessage(msg1),               // Conversation history        │
│    AssistantMessage(msg2),          // (last 10 messages)          │
│    ...                                                              │
│  ]                                                                  │
└─────────────────────────────────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Post-Processing                                                    │
│  service.go:1050-1080                                               │
│  ├─ SanitizeResponse()                                             │
│  ├─ CorrectContactsInResponse()                                    │
│  ├─ CorrectEmailsInResponse()                                      │
│  └─ verifyResponseWithLLM() (if LLM_VERIFIER_ENABLED)              │
└─────────────────────────────────────────────────────────────────────┘
                   │
                   ▼
                RESPONSE
```

---

## Intent-Driven Retrieval

The user's classified intent determines WHAT to search for:

| Intent | Content Types Searched | Top-K |
|--------|----------------------|-------|
| `service_inquiry` | service, faq | 5 |
| `booking_request` | service, faq | 5 |
| `coverage_question` | coverage, service | 5 |
| `pricing_question` | faq, service | 3 |
| `emergency` | service, safety | 3 |
| `complaint` | rule, faq | 3 |
| `escalation` | rule, faq | 3 |
| default (unknown) | faq, service, kb_section | 5 |

**Code Location:** `vectordb.go:647-671`

```go
func RetrieveContextForQuery(...) {
    var contentTypes []string
    var topK int

    switch intent {
    case "service_inquiry", "booking_request":
        contentTypes = []string{"service", "faq"}
        topK = 5
    case "coverage_question":
        contentTypes = []string{"coverage", "service"}
        topK = 5
    // ... more cases
    }
}
```

---

## Key Code Locations

| Function | File:Line | Purpose |
|----------|-----------|---------|
| `HandleChatExternal()` | handlers.go:92-130 | External chat endpoint |
| `HandleChatInternal()` | handlers.go:135-177 | Internal chat endpoint |
| `ChatExternal()` | service.go:813-842 | External chat service |
| `ChatInternal()` | service.go:845-902 | Internal chat service |
| `processChat()` | service.go:905-1090 | Main chat orchestration |
| `UseRAGPipeline()` | service.go:1582-1589 | Check if RAG available |
| `generateRAGResponse()` | service.go:1594-1676 | RAG generation path |
| `generateResponse()` | service.go:1100-1200 | Long context generation path |
| `RetrieveContextForQuery()` | vectordb.go:637-719 | Intent-driven retrieval |
| `buildRAGSystemPrompt()` | service.go:1680-1892 | RAG prompt with retrieved context |
| `buildSystemPrompt()` | service.go:1200-1400 | Long context prompt with full KB |
| `Search()` | vectordb_lance.go | LanceDB vector search |
| `Embed()` | embedding.go | Generate embeddings |

---

## Configuration Reference

### Environment Variables

```bash
# Required for RAG
RAG_PIPELINE_ENABLED=true

# Storage (pick one)
LANCEDB_STORAGE_PROVIDER=local        # Local filesystem
LANCEDB_LOCAL_PATH=build/data/lancedb
# OR
LANCEDB_STORAGE_PROVIDER=s3           # S3-compatible storage
S3_BUCKET=my-bucket
S3_ENDPOINT=https://...

# Embedding Provider (pick one)
EMBEDDING_PROVIDER=cybertron          # Native Go, recommended
EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L6-v2
# OR
EMBEDDING_PROVIDER=openrouter         # API-based
OPENROUTER_API_KEY=sk-or-v1-xxx
EMBEDDING_MODEL=openai/text-embedding-3-small

# Hybrid Search (optional)
HYBRID_SEARCH_ENABLED=true
HYBRID_VECTOR_WEIGHT=0.7
HYBRID_TEXT_WEIGHT=0.3
```

### Tenant Configuration

Set in Agent Admin UI or database:

```sql
-- Check current retrieval mode
SELECT tenant_id, retrieval_mode FROM tenant_config WHERE tenant_id = ?;

-- Set RAG mode
UPDATE tenant_config SET retrieval_mode = 'rag' WHERE tenant_id = ?;

-- Set Long Context mode (default)
UPDATE tenant_config SET retrieval_mode = 'long_context' WHERE tenant_id = ?;
```

---

## Verification & Debugging

### How to Verify RAG Is Active

**1. Check Server Startup Logs:**
```
INFO Embedding service initialized provider=cybertron dimension=384
INFO VectorDB initialized storage=local path=build/data/lancedb
INFO RAG pipeline enabled
```

**2. Check Chat Logs During Request:**
```
INFO Processing chat useRAG=true tenantID=1 intent=service_inquiry
INFO RetrieveContextForQuery query="what services do you offer" topK=5
INFO Search completed results=5 searchMode=hybrid latencyMs=45
INFO Using RAG system prompt sections=9 retrievedChunks=5
```

**3. Check Tenant Config:**
```bash
sqlite3 build/data/memos_dev.db \
  "SELECT retrieval_mode FROM tenant_config WHERE tenant_id = 1;"
# Should return: rag
```

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| RAG not used | `RetrievalMode != "rag"` | Set retrieval mode in Agent Admin |
| 0 results | No indexed content | Click "Rebuild Index" in Agent Admin |
| Wrong results | Dimension mismatch | Delete LanceDB folder, rebuild index |
| Slow retrieval | No vector index | Ensure 256+ chunks before building index |

### Debug Logging

Add to service.go for detailed RAG debugging:

```go
slog.Debug("RAG decision",
    "useRAGPipeline", s.UseRAGPipeline(),
    "retrievalMode", tenantConfig.RetrievalMode,
    "useRAG", useRAG)

slog.Debug("RAG retrieval",
    "query", userMessage,
    "intent", classification.PrimaryIntent,
    "contentTypes", contentTypes,
    "topK", topK)
```

---

## Embedding Flow Detail

When a user sends a message, here's exactly how it gets embedded:

```
User: "What water damage restoration services do you offer?"
                    │
                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  RetrieveContextForQuery(query="What water damage...")              │
│  vectordb.go:637                                                    │
└─────────────────────────────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  SearchQuery{                                                       │
│    QueryText: "What water damage restoration services do you offer?"│
│    TenantID: 1,                                                     │
│    AudienceType: "external",                                        │
│    ContentTypes: ["service", "faq"],  // From intent               │
│    TopK: 5,                                                         │
│    MinScore: 0.3,                                                   │
│    UseHybridSearch: true,                                           │
│    VectorWeight: 0.7,                                               │
│    TextWeight: 0.3                                                  │
│  }                                                                  │
└─────────────────────────────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  db.Search(searchQuery)                                             │
│  vectordb_lance.go                                                  │
│                                                                     │
│  Step 1: Embed the query                                           │
│  ────────────────────────                                          │
│  embeddings, _ := db.embedSvc.Embed([]string{query})               │
│  queryVector := embeddings[0]  // [0.023, -0.156, 0.089, ...]      │
│                                // 384 dimensions                    │
│                                                                     │
│  Step 2: Vector search (cosine similarity)                         │
│  ──────────────────────────────────────────                        │
│  SELECT * FROM rag_chunks                                          │
│  WHERE tenant_id = 1 AND audience_type = 'external'                │
│  ORDER BY cosine_distance(embedding, queryVector) ASC              │
│  LIMIT 10                                                          │
│                                                                     │
│  Step 3: BM25 text search                                          │
│  ────────────────────────                                          │
│  Full-text search on content field                                 │
│  Keywords: "water", "damage", "restoration", "services"            │
│                                                                     │
│  Step 4: Score fusion                                              │
│  ───────────────────                                               │
│  finalScore = 0.7 * vectorScore + 0.3 * bm25Score                  │
│                                                                     │
│  Step 5: Filter and rank                                           │
│  ──────────────────────                                            │
│  Keep only: score >= 0.3, contentType in ["service", "faq"]        │
│  Return top 5 by finalScore                                        │
└─────────────────────────────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  SearchResult{                                                      │
│    Chunks: [                                                        │
│      {ID: "chunk-123", Title: "Water Damage Restoration",          │
│       Content: "We offer 24/7 emergency water extraction...",       │
│       Score: 0.87, ContentType: "service"},                        │
│      {ID: "chunk-456", Title: "FAQ: Water Damage Process",         │
│       Content: "Our water damage restoration process...",           │
│       Score: 0.82, ContentType: "faq"},                            │
│      ...                                                            │
│    ],                                                               │
│    SearchMode: "hybrid"                                            │
│  }                                                                  │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Summary

The bchat RAG pipeline provides a complete semantic search capability:

1. **User questions ARE embedded** - The query text is converted to a vector using the configured embedding provider (Cybertron, OpenRouter, etc.)

2. **Vector similarity search** - The query embedding is compared against all indexed document chunks using cosine distance

3. **Intent-driven filtering** - The classified user intent determines which content types to search and how many results to return

4. **Hybrid search** - Optional combination of vector similarity (semantic) and BM25 (keyword) for better accuracy

5. **Context injection** - Retrieved chunks are formatted and injected into Section 5 of the RAG system prompt

6. **Fallback mechanism** - If RAG fails or returns no results, automatically falls back to long context mode

The implementation is production-ready with multiple safeguards including minimum relevance thresholds, batched processing for large files, and automatic fallback on retrieval failure.

---

*Document Version: 1.0*
*Last Updated: 2026-01-27*
