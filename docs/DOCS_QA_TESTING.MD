# Proposal: Visual RAG Testing Interface

**Date:** 2026-01-27
**Status:** Proposed
**Priority:** High
**Purpose:** Provide intuitive, visual way to test and debug embedding quality and retrieval accuracy

---

## Problem Statement

Current Q&A testing is a black box:
- User clicks "Test All" → sees "Found" or "Not Found"
- No visibility into:
  - What chunks were retrieved
  - How similar they are to the query
  - Why a match failed
  - How embeddings relate to each other

**Users need to understand:**
1. How their question gets embedded
2. What chunks the search returns
3. How close/far the results are from the expected answer
4. Visual representation of the embedding space

---

## Proposed Solution: RAG Playground

A dedicated "RAG Playground" section in Agent Admin with three components:

### Component 1: Search Explorer

Interactive search interface showing real-time retrieval results.

```
┌─────────────────────────────────────────────────────────────────────────┐
│ 🔍 RAG Search Explorer                                                  │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│ Query: [What are the core beliefs of Iglesia ni Cristo?    ] [Search]  │
│                                                                         │
│ Audience: (•) Internal  ( ) External       Top K: [5 ▼]                │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│ Results (5 chunks in 45ms)                                              │
│                                                                         │
│ ┌─────────────────────────────────────────────────────────────────────┐ │
│ │ #1  Score: 0.89  ████████████████████░░░░  (89% match)              │ │
│ │ Section: Core Doctrines > Fundamental Beliefs                       │ │
│ │ ─────────────────────────────────────────────────────────────────── │ │
│ │ "The Iglesia ni Cristo believes in one God, the Father. Jesus      │ │
│ │ Christ is the Son of God and the mediator between God and man.     │ │
│ │ The Holy Spirit is the power of God, not a separate person..."     │ │
│ │                                                      [View Full ▼]  │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
│                                                                         │
│ ┌─────────────────────────────────────────────────────────────────────┐ │
│ │ #2  Score: 0.76  ████████████████░░░░░░░░  (76% match)              │ │
│ │ Section: History > Founding Principles                              │ │
│ │ ─────────────────────────────────────────────────────────────────── │ │
│ │ "Founded in 1914 by Felix Y. Manalo, the church established its    │ │
│ │ core beliefs based on biblical interpretation..."                   │ │
│ │                                                      [View Full ▼]  │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
│                                                                         │
│ ┌─────────────────────────────────────────────────────────────────────┐ │
│ │ #3  Score: 0.71  ███████████████░░░░░░░░░  (71% match)              │ │
│ │ ...                                                                 │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**Features:**
- Real-time search with score visualization
- Expandable chunk content
- Score bar showing similarity percentage
- Source section breadcrumb
- Latency display

---

### Component 2: Q&A Pair Debugger

Visual comparison of expected vs retrieved content for each Q&A pair.

```
┌─────────────────────────────────────────────────────────────────────────┐
│ 🧪 Q&A Pair Debugger                                      [Test All]   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│ Pair #1: "What are the core beliefs?"                    ✅ FOUND      │
│ ─────────────────────────────────────────────────────────────────────── │
│                                                                         │
│ ┌─────────────────────────┐    ┌─────────────────────────┐             │
│ │ 📝 Expected Answer      │    │ 🔍 Retrieved Chunk #1   │             │
│ │                         │    │                         │             │
│ │ "The INC believes in    │ ←→ │ "The Iglesia ni Cristo  │  Score:    │
│ │ one God, the Father,    │    │ believes in one God,    │  0.89      │
│ │ and Jesus Christ as     │    │ the Father. Jesus       │  ████████░ │
│ │ the Son of God..."      │    │ Christ is the Son..."   │             │
│ │                         │    │                         │             │
│ └─────────────────────────┘    └─────────────────────────┘             │
│                                                                         │
│ Keyword Match: God ✓  Jesus ✓  Christ ✓  Son ✓  Father ✓  (5/5)       │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│ Pair #2: "When was the church founded?"                  ❌ NOT FOUND  │
│ ─────────────────────────────────────────────────────────────────────── │
│                                                                         │
│ ┌─────────────────────────┐    ┌─────────────────────────┐             │
│ │ 📝 Expected Answer      │    │ 🔍 Best Retrieved       │             │
│ │                         │    │                         │             │
│ │ "The INC was founded    │ ✗  │ "The church has grown   │  Score:    │
│ │ on July 27, 1914 by     │    │ to millions of members  │  0.42      │
│ │ Felix Y. Manalo in      │    │ across the world..."    │  ████░░░░░ │
│ │ Punta, Santa Ana..."    │    │                         │             │
│ │                         │    │                         │             │
│ └─────────────────────────┘    └─────────────────────────┘             │
│                                                                         │
│ Keyword Match: 1914 ✗  founded ✗  Manalo ✗  July ✗  (0/4)             │
│                                                                         │
│ 💡 Suggestion: The founding date may not be indexed. Check KB content. │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**Features:**
- Side-by-side comparison of expected vs retrieved
- Keyword highlighting showing what matched
- Visual score bar
- Actionable suggestions for failed pairs
- Drill-down into why a match failed

---

### Component 3: Embedding Visualizer (Advanced)

2D visualization of embedding space using dimensionality reduction.

```
┌─────────────────────────────────────────────────────────────────────────┐
│ 📊 Embedding Space Visualizer                                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│     Legend: ● KB Chunks  ★ Query  ○ Expected Answer                    │
│                                                                         │
│                        ●                                                │
│                    ●       ●                                            │
│                ●               ★ ← Your query                          │
│            ●       ●     ●  ○ ← Expected answer                        │
│                ●           ●                                            │
│        ●           ●   ●                                                │
│            ●   ●       ●                                                │
│                    ●                                                    │
│        ●       ●           ●                                            │
│                                                                         │
│ Nearest chunks to query:                                                │
│ 1. "Core beliefs..." (dist: 0.11)                                      │
│ 2. "Doctrines..." (dist: 0.18)                                         │
│ 3. "History..." (dist: 0.24)                                           │
│                                                                         │
│ Distance to expected answer: 0.15 (close - good embedding!)            │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**Features:**
- t-SNE or UMAP projection of embeddings to 2D
- Interactive: hover to see chunk content
- Shows query position relative to chunks
- Clusters visible = good semantic organization
- Distance metrics displayed

---

## API Endpoints Required

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/:slug/rag/search` | POST | Search with full result details |
| `/:slug/rag/debug-pair/:id` | GET | Debug single Q&A pair with comparison |
| `/:slug/rag/embeddings` | GET | Get embeddings for visualization (sampled) |
| `/:slug/rag/compare` | POST | Compare two texts' embeddings |

### Search Endpoint Response

```json
{
  "query": "What are the core beliefs?",
  "query_tokens": 6,
  "latency_ms": 45,
  "results": [
    {
      "rank": 1,
      "chunk_id": "4:internal:kb:doctrines-001",
      "score": 0.89,
      "title": "Core Doctrines > Fundamental Beliefs",
      "content": "The Iglesia ni Cristo believes in one God...",
      "content_preview": "The Iglesia ni Cristo believes in one God, the Father...",
      "highlight_matches": ["God", "believes", "Christ"]
    }
  ]
}
```

### Debug Pair Endpoint Response

```json
{
  "pair_id": 1,
  "question": "What are the core beliefs?",
  "expected_answer": "The INC believes in one God...",
  "status": "found",
  "best_match": {
    "chunk_id": "4:internal:kb:doctrines-001",
    "score": 0.89,
    "content": "The Iglesia ni Cristo believes in one God...",
    "rank": 1
  },
  "keyword_analysis": {
    "expected_keywords": ["God", "Jesus", "Christ", "Son", "Father"],
    "matched_keywords": ["God", "Jesus", "Christ", "Son", "Father"],
    "match_ratio": 1.0
  },
  "all_results": [...],
  "suggestions": []
}
```

---

## UI Implementation Plan

### Phase 1: Search Explorer (MVP)
- Simple search box with results
- Score bars and chunk previews
- Audience selector

### Phase 2: Q&A Debugger
- Side-by-side comparison view
- Keyword highlighting
- Pass/fail indicators

### Phase 3: Embedding Visualizer
- 2D scatter plot using Chart.js or D3
- Server-side t-SNE/UMAP computation
- Interactive hover tooltips

---

## Files to Create/Modify

### New Files

| File | Purpose |
|------|---------|
| `web/src/components/RAGPlayground.tsx` | Main playground component |
| `web/src/components/SearchExplorer.tsx` | Search interface |
| `web/src/components/QAPairDebugger.tsx` | Q&A comparison view |
| `web/src/components/EmbeddingVisualizer.tsx` | 2D embedding plot |

### Modified Files

| File | Change |
|------|--------|
| `handlers.go` | Add RAG debug endpoints |
| `service.go` | Add search with full details |
| `AgentAdmin.tsx` | Add RAG Playground tab/section |
| `agentAdmin.ts` | Add store methods for new endpoints |
| `en.json` | Add translations |

---

## Immediate Bug Fix Required

Before implementing the playground, fix the "Not Found" bug:

**File:** `handlers.go` line 4514

```go
// CURRENT (wrong):
searchResult, err := h.service.SearchVectorDB(ctx, tenantID, "external", pair.Question, 5)

// FIXED:
searchResult, err := h.service.SearchVectorDB(ctx, tenantID, "internal", pair.Question, 5)
```

---

## Benefits

1. **Transparency** - See exactly what the RAG system is doing
2. **Debugging** - Quickly identify why retrievals fail
3. **Tuning** - Adjust chunk size, embedding model based on visual feedback
4. **Confidence** - Validate embedding quality before production
5. **Education** - Understand how semantic search works

---

## Success Metrics

| Metric | Target |
|--------|--------|
| Time to debug failed retrieval | < 30 seconds |
| Understanding of why match failed | Visual explanation |
| Confidence in embedding quality | Quantified score |

---

## Priority Recommendation

1. **Immediate:** Fix the `"external"` → `"internal"` bug
2. **Short-term:** Implement Search Explorer (most value, simplest)
3. **Medium-term:** Implement Q&A Debugger
4. **Long-term:** Implement Embedding Visualizer

---

*Document created: 2026-01-27*
