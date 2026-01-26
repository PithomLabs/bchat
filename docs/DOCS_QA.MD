# Plan: Q&A Pairs for Embedding Quality Testing

**Date:** 2026-01-27
**Status:** Planned
**Priority:** Medium
**Purpose:** Generate Q&A pairs from KB content to evaluate embedding/retrieval quality

---

## Overview

Create a system to:
1. Generate Q&A pairs from tenant KB content using LLM
2. Store pairs in database for persistence
3. Display in Agent Admin UI for review
4. Use for automated retrieval quality testing

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        KB.MD Content                             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    LLM Q&A Generator                             │
│  "Generate 50 Q&A pairs from this knowledge base content..."    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Database Storage                               │
│                 agent_qa_pairs table                             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Agent Admin UI                                │
│              Q&A Pairs Tab / Accordion                           │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Q: What are your business hours?                        │   │
│  │ A: We are open Monday-Friday 9am-5pm                    │   │
│  │ Source: FAQ Section                                      │   │
│  │ [Test Retrieval] [Edit] [Delete]                        │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

## 1. Database Schema

### New Table: `agent_qa_pairs`

```sql
CREATE TABLE IF NOT EXISTS agent_qa_pairs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    question TEXT NOT NULL,
    expected_answer TEXT NOT NULL,
    source_section TEXT,           -- Which KB section this came from
    source_chunk_id TEXT,          -- Reference to chunk ID in vector DB
    difficulty TEXT DEFAULT 'medium', -- easy, medium, hard
    category TEXT,                 -- faq, service, policy, coverage, etc.
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenant(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_qa_pairs_tenant ON agent_qa_pairs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_qa_pairs_category ON agent_qa_pairs(category);
```

### Migration File

Location: `store/migration/sqlite/0.25/10__qa_pairs.sql`

---

## 2. Q&A Generation

### LLM Prompt for Generation

```
You are a QA test generator. Given the following knowledge base content, generate up to 50 question-answer pairs that could be used to test a RAG system's retrieval quality.

REQUIREMENTS:
1. Questions should be natural language queries a real user might ask
2. Answers should be directly derivable from the KB content
3. Include a mix of:
   - Simple factual questions (easy)
   - Questions requiring multiple facts (medium)
   - Questions with nuanced answers (hard)
4. Categorize each pair (faq, service, policy, coverage, safety, general)
5. Note which section of the KB contains the answer

OUTPUT FORMAT (JSON):
{
  "qa_pairs": [
    {
      "question": "What are your business hours?",
      "answer": "We are open Monday through Friday from 9am to 5pm.",
      "source_section": "Contact Information",
      "difficulty": "easy",
      "category": "faq"
    },
    ...
  ]
}

KB CONTENT:
---
{kb_content}
---

Generate up to 50 diverse Q&A pairs. Output valid JSON only.
```

### Generation Endpoint

```
POST /api/v1/agent/:slug/qa/generate
```

**Request:**
```json
{
  "max_pairs": 50,
  "audience_type": "external"
}
```

**Response:**
```json
{
  "generated": 47,
  "pairs": [
    {
      "id": 1,
      "question": "What services do you offer?",
      "expected_answer": "We offer water damage restoration, fire damage restoration, and mold remediation.",
      "source_section": "Services",
      "difficulty": "easy",
      "category": "service"
    }
  ]
}
```

---

## 3. Storage Implementation

### Go Types (store/agent.go)

```go
type QAPair struct {
    ID             int32     `json:"id"`
    TenantID       int32     `json:"tenant_id"`
    Question       string    `json:"question"`
    ExpectedAnswer string    `json:"expected_answer"`
    SourceSection  string    `json:"source_section,omitempty"`
    SourceChunkID  string    `json:"source_chunk_id,omitempty"`
    Difficulty     string    `json:"difficulty"` // easy, medium, hard
    Category       string    `json:"category"`   // faq, service, policy, etc.
    IsActive       bool      `json:"is_active"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
}

type FindQAPair struct {
    ID       *int32
    TenantID *int32
    Category *string
    IsActive *bool
}
```

### Driver Interface (store/driver.go)

```go
// Q&A Pairs for embedding testing
CreateQAPair(ctx context.Context, pair *QAPair) (*QAPair, error)
ListQAPairs(ctx context.Context, find *FindQAPair) ([]*QAPair, error)
UpdateQAPair(ctx context.Context, pair *QAPair) (*QAPair, error)
DeleteQAPair(ctx context.Context, id int32) error
DeleteQAPairsByTenant(ctx context.Context, tenantID int32) error
```

---

## 4. API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/agent/:slug/qa/generate` | Generate Q&A pairs from KB |
| GET | `/api/v1/agent/:slug/qa` | List all Q&A pairs |
| POST | `/api/v1/agent/:slug/qa` | Create single Q&A pair |
| PUT | `/api/v1/agent/:slug/qa/:id` | Update Q&A pair |
| DELETE | `/api/v1/agent/:slug/qa/:id` | Delete Q&A pair |
| POST | `/api/v1/agent/:slug/qa/:id/test` | Test retrieval for single pair |
| POST | `/api/v1/agent/:slug/qa/test-all` | Test all pairs, return scores |

---

## 5. UI Design

### Location in Agent Admin

Add new section/tab: **"Q&A Test Pairs"** in Agent Admin page

### UI Components

```
┌─────────────────────────────────────────────────────────────────┐
│ Q&A Test Pairs                                    [Generate 50] │
├─────────────────────────────────────────────────────────────────┤
│ Filter: [All ▼] [Easy/Medium/Hard ▼]    Total: 47 pairs        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ #1 [easy] [faq]                                    [⋮ menu] │ │
│ │ Q: What are your business hours?                            │ │
│ │ A: We are open Monday-Friday 9am-5pm                        │ │
│ │ Source: Contact Information                                  │ │
│ │                                                              │ │
│ │ [Test Retrieval]  Result: ✓ Found (score: 0.89)            │ │
│ └─────────────────────────────────────────────────────────────┘ │
│                                                                 │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ #2 [medium] [service]                              [⋮ menu] │ │
│ │ Q: Do you handle mold after water damage?                   │ │
│ │ A: Yes, our water damage restoration includes mold...       │ │
│ │ Source: Services > Water Damage                              │ │
│ │                                                              │ │
│ │ [Test Retrieval]  Result: ✗ Not in top 5 (best: 0.45)      │ │
│ └─────────────────────────────────────────────────────────────┘ │
│                                                                 │
│ ... (scrollable list)                                          │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│ [Test All Pairs]                                                │
│                                                                 │
│ Last Test Results:                                              │
│ ├─ Recall@5: 85% (40/47 found)                                 │
│ ├─ Avg Score: 0.76                                             │
│ └─ Failed: 7 pairs (click to view)                             │
└─────────────────────────────────────────────────────────────────┘
```

### Card Actions Menu (⋮)

- Edit question/answer
- Delete pair
- Mark as inactive
- View source chunk

---

## 6. Retrieval Testing

### Test Single Pair

```go
func (h *Handler) HandleTestQAPair(c echo.Context) error {
    // 1. Get the Q&A pair
    pair := getQAPair(id)

    // 2. Search vector DB with question
    results, err := vectorDB.Search(ctx, SearchQuery{
        QueryText:  pair.Question,
        TenantID:   tenantID,
        TopK:       5,
        ActiveOnly: true,
    })

    // 3. Check if expected answer content appears in results
    found := false
    bestScore := 0.0
    for _, chunk := range results.Chunks {
        if containsAnswer(chunk.Content, pair.ExpectedAnswer) {
            found = true
            bestScore = max(bestScore, results.Scores[i])
        }
    }

    // 4. Return result
    return c.JSON(200, TestResult{
        Found:     found,
        BestScore: bestScore,
        TopChunks: results.Chunks[:min(3, len(results.Chunks))],
    })
}
```

### Test All Pairs Response

```json
{
  "total_pairs": 47,
  "found": 40,
  "not_found": 7,
  "recall_at_5": 0.85,
  "avg_score": 0.76,
  "results": [
    {
      "pair_id": 1,
      "question": "What are your business hours?",
      "found": true,
      "score": 0.89,
      "rank": 1
    },
    {
      "pair_id": 15,
      "question": "Do you handle mold after water damage?",
      "found": false,
      "best_score": 0.45,
      "top_chunk_preview": "Water damage restoration services..."
    }
  ],
  "failed_pairs": [15, 23, 31, 35, 41, 44, 47]
}
```

---

## 7. Files to Create/Modify

### New Files

| File | Purpose |
|------|---------|
| `store/migration/sqlite/0.25/10__qa_pairs.sql` | Database migration |
| `web/src/components/QAPairsSection.tsx` | UI component |

### Modified Files

| File | Change |
|------|--------|
| `store/agent.go` | Add QAPair types and Store methods |
| `store/driver.go` | Add QAPair interface methods |
| `store/db/sqlite/agent.go` | Implement QAPair CRUD |
| `server/router/api/v1/agent/handlers.go` | Add Q&A handlers |
| `server/router/api/v1/v1.go` | Register Q&A routes |
| `web/src/pages/AgentAdmin.tsx` | Add Q&A section |
| `web/src/store/v2/agentAdmin.ts` | Add Q&A state and actions |
| `web/src/locales/en.json` | Add translations |

---

## 8. Implementation Phases

### Phase 1: Backend (Database + API)
1. Create migration file
2. Add Go types to store/agent.go
3. Implement SQLite CRUD in store/db/sqlite/agent.go
4. Add handlers for CRUD operations
5. Add Q&A generation endpoint (LLM call)

### Phase 2: Testing Logic
1. Implement single pair test endpoint
2. Implement batch test endpoint
3. Add scoring/metrics calculation

### Phase 3: Frontend
1. Create QAPairsSection component
2. Add to Agent Admin page
3. Implement generate, list, test UI
4. Add results visualization

---

## 9. Usage Workflow

1. **Generate**: Click "Generate 50" → LLM analyzes KB → Creates Q&A pairs
2. **Review**: Browse pairs, edit/delete as needed
3. **Test**: Click "Test All Pairs" → System queries each question
4. **Analyze**: View Recall@5, identify failing pairs
5. **Iterate**: Improve KB content or chunking for failed pairs

---

## 10. Verification

### Manual Testing
1. Upload KB.MD with diverse content
2. Generate Q&A pairs
3. Verify pairs are reasonable
4. Run "Test All" and check recall
5. Edit a pair and re-test

### Automated Testing
```go
func TestQAPairGeneration(t *testing.T) {
    // Test that generation returns valid JSON
    // Test that pairs reference actual KB content
}

func TestQAPairRetrieval(t *testing.T) {
    // Test that retrieval finds expected chunks
    // Test scoring accuracy
}
```

---

## 11. Future Enhancements

- **Auto-regenerate** on KB upload
- **Difficulty auto-detection** based on retrieval success
- **Export/Import** Q&A pairs as JSON
- **Compare providers** - test same pairs with Cybertron vs OpenRouter
- **Historical tracking** - track recall over time as KB changes

---

## ADDENDUM: Operational Workflow Guide

**Added:** 2026-01-27
**Status:** Implemented (with known limitations)

### Prerequisites

Before using Q&A Pairs, ensure:

1. **RAG Pipeline is enabled:**
   ```bash
   RAG_PIPELINE_ENABLED=true task run:rag
   ```

2. **Tenant has internal KB content:**
   - Q&A generation reads from `internal` audience KB files
   - Upload KB.MD via Agent Admin with "Internal" audience selected

3. **Index is built:**
   - Click "Rebuild Index" in Agent Admin
   - Verify chunks > 0 in success message

4. **Tenant retrieval mode is NOT `long_context`:**
   - If tenant uses `long_context` mode, RAG indexing is skipped
   - Check: `sqlite3 build/data/memos_dev.db "SELECT retrieval_mode FROM tenant_config WHERE tenant_id = X;"`
   - Fix: `UPDATE tenant_config SET retrieval_mode = 'rag' WHERE tenant_id = X;`

### Step-by-Step Workflow

#### Step 1: Prepare Content

```
Agent Admin → Select Tenant → Upload KB.MD (Internal audience)
```

- KB content should be well-structured markdown with clear sections
- Richer content = better Q&A pairs generated

#### Step 2: Build Vector Index

```
Agent Admin → Click "Rebuild Index"
```

- Wait for success message showing chunk count
- Example: "Successfully reindexed 510 chunks"

#### Step 3: Generate Q&A Pairs

```
Agent Admin → Q&A Pairs section (purple box) → Click "Generate Pairs"
```

**What happens internally:**
1. Backend fetches KB content for tenant (internal audience)
2. Sends KB to LLM with generation prompt
3. LLM returns JSON with Q&A pairs
4. Pairs stored in `agent_qa_pairs` table
5. UI displays generated pairs

**Expected output:** Modal opens showing generated pairs with questions, answers, categories, and difficulty levels.

#### Step 4: Test Retrieval Quality

```
Q&A Pairs Modal → Click "Test All"
```

**What happens internally:**
1. For each Q&A pair:
   - Uses the question as a vector search query
   - Retrieves top-K chunks from vector DB
   - Checks if expected answer appears in retrieved chunks
2. Calculates metrics:
   - **Recall@K**: % of pairs where answer was found in top-K results
   - **Avg Score**: Average similarity score for found answers

#### Step 5: Analyze Results

Review the test results:
- **High Recall (>80%)**: Embeddings are working well
- **Low Recall (<60%)**: Consider:
  - Different embedding model
  - Better chunking strategy
  - KB content improvements

### Current API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/agent/:slug/qa-pairs/generate` | POST | Generate pairs from KB |
| `/api/v1/agent/:slug/qa-pairs` | GET | List all pairs |
| `/api/v1/agent/:slug/qa-pairs` | POST | Create single pair |
| `/api/v1/agent/:slug/qa-pairs/:id` | PUT | Update pair |
| `/api/v1/agent/:slug/qa-pairs/:id` | DELETE | Delete pair |
| `/api/v1/agent/:slug/qa-pairs/test-all` | POST | Test all pairs |

### Known Limitations (Current Implementation)

| Limitation | Description | Workaround |
|------------|-------------|------------|
| Internal audience only | Q&A pairs generated from internal KB only | Upload content as internal |
| No audience selector in UI | Cannot choose external/internal in UI | Defaults to internal |
| No individual pair testing | UI only has "Test All", no single pair test | Use API directly |
| No pair editing in UI | Cannot edit questions/answers in UI | Delete and regenerate |
| No filtering/sorting | Cannot filter pairs by category/difficulty | Future enhancement |
| No export/import | Cannot backup or share Q&A pairs | Use direct DB queries |

### Troubleshooting

#### "Generate Pairs" does nothing / shows error

**Cause:** No KB content for the specified audience type

**Solution:**
1. Check tenant has internal KB:
   ```bash
   sqlite3 build/data/memos_dev.db "SELECT id, length(content) FROM agent_source_files WHERE tenant_id=X AND audience_type='internal' AND file_type='kb';"
   ```
2. If empty, upload KB.MD with Internal audience in Agent Admin

#### "0 chunks indexed" on Rebuild Index

**Cause:** Tenant uses `long_context` retrieval mode

**Solution:**
```bash
sqlite3 build/data/memos_dev.db "UPDATE tenant_config SET retrieval_mode='rag' WHERE tenant_id=X;"
```

#### Q&A pairs exist but "Test All" fails

**Cause:** Vector index not built or dimension mismatch

**Solution:**
1. Delete LanceDB: `rm -rf build/data/lancedb/`
2. Restart server
3. Click "Rebuild Index"
4. Try "Test All" again

### Database Schema Reference

```sql
-- Table: agent_qa_pairs
CREATE TABLE agent_qa_pairs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    question TEXT NOT NULL,
    expected_answer TEXT NOT NULL,
    source_section TEXT,        -- KB section containing answer
    source_chunk_id TEXT,       -- Vector DB chunk ID (if linked)
    difficulty TEXT DEFAULT 'medium',  -- easy, medium, hard
    category TEXT,              -- faq, service, policy, etc.
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);
```

### Quick Verification Commands

```bash
# Check Q&A pairs for a tenant
sqlite3 build/data/memos_dev.db "SELECT id, question, difficulty, category FROM agent_qa_pairs WHERE tenant_id=X LIMIT 5;"

# Count pairs by tenant
sqlite3 build/data/memos_dev.db "SELECT tenant_id, COUNT(*) FROM agent_qa_pairs GROUP BY tenant_id;"

# Delete all pairs for a tenant (to regenerate fresh)
sqlite3 build/data/memos_dev.db "DELETE FROM agent_qa_pairs WHERE tenant_id=X;"
```
