# RAG Reindex Resume-from-Error Feature

**Status:** Planned
**Date:** 2026-01-31
**Related Documents:**
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG pipeline architecture
- [DOCS_BUGS_REBUILD_INDEX.MD](./DOCS_BUGS_REBUILD_INDEX.MD) - Rebuild index timeout fix

---

## Problem Statement

When reindexing large KB content, the process can fail mid-way due to transient API errors:

```
INFO Processing batch batch=121 totalBatches=137 chunksInBatch=25 progress=3025/3401
ERROR reindex failed tenantID=6 audience=internal error="failed to insert chunks for audience internal: failed to generate embeddings for batch 121: OpenRouter API error: No successful provider responses."
```

**Current behavior:** The entire reindex must be restarted from batch 1, wasting:
- Time (re-processing 3025 chunks that already succeeded)
- API costs (re-embedding already-indexed content)
- User patience

**Desired behavior:** Resume from the last successful batch (batch 120 in this case).

---

## Root Cause Analysis

The error `OpenRouter API error: No successful provider responses` is a transient failure - the embedding API temporarily couldn't process the request. This can happen due to:
- Rate limiting
- Temporary provider outage
- Network issues
- Provider capacity limits

The current implementation in `vectordb_lance.go` processes batches sequentially but doesn't track progress persistently.

---

## Proposed Solution

### Option A: Checkpoint-Based Resume (Recommended)

Store reindex progress in the database, allowing resume from last successful batch.

**New table: `agent_reindex_checkpoints`**

```sql
CREATE TABLE IF NOT EXISTS agent_reindex_checkpoints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    audience TEXT NOT NULL,
    total_chunks INTEGER NOT NULL,
    processed_chunks INTEGER NOT NULL DEFAULT 0,
    current_batch INTEGER NOT NULL DEFAULT 0,
    total_batches INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'in_progress',  -- in_progress, completed, failed
    error_message TEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE,
    UNIQUE(tenant_id, audience)
);
```

**Workflow:**

```
┌─────────────────────────────────────────────────────────────────┐
│                    Reindex Request                               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Check for existing checkpoint                                   │
│  SELECT * FROM agent_reindex_checkpoints                         │
│  WHERE tenant_id=? AND audience=? AND status='failed'            │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              │                               │
              ▼                               ▼
┌─────────────────────────┐     ┌─────────────────────────────────┐
│  No checkpoint found    │     │  Checkpoint found               │
│  Start from batch 0     │     │  Resume from current_batch      │
└─────────────────────────┘     └─────────────────────────────────┘
              │                               │
              └───────────────┬───────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Process batches                                                 │
│  FOR batch in range(start_batch, total_batches):                 │
│    - Process batch                                               │
│    - UPDATE checkpoint SET current_batch=?, processed_chunks=?   │
│    - On error: SET status='failed', error_message=?              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  On completion: SET status='completed', completed_at=NOW()       │
└─────────────────────────────────────────────────────────────────┘
```

### Option B: Chunk-Level Tracking

Track individual chunks that have been indexed (more granular but higher overhead).

**Pros:** Can resume from exact failure point
**Cons:** More database writes, complex deduplication logic

### Option C: Automatic Retry with Exponential Backoff

Retry failed batches automatically before giving up.

**Pros:** Handles transient failures transparently
**Cons:** Doesn't help if failure persists; still loses progress on hard failures

---

## Recommended Implementation: Option A + C Combined

1. **Automatic retry** (3 attempts with exponential backoff) for transient failures
2. **Checkpoint persistence** for resume after hard failures

---

## Implementation Plan

### Phase 1: Database Schema

**File:** `store/migration/sqlite/0.25/21__reindex_checkpoints.sql`

```sql
-- Reindex checkpoint tracking for resume-from-error support
CREATE TABLE IF NOT EXISTS agent_reindex_checkpoints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    audience TEXT NOT NULL,
    total_chunks INTEGER NOT NULL,
    processed_chunks INTEGER NOT NULL DEFAULT 0,
    current_batch INTEGER NOT NULL DEFAULT 0,
    total_batches INTEGER NOT NULL,
    batch_size INTEGER NOT NULL DEFAULT 25,
    status TEXT NOT NULL DEFAULT 'in_progress',
    error_message TEXT,
    error_batch INTEGER,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_reindex_checkpoint_tenant_audience
ON agent_reindex_checkpoints(tenant_id, audience);
```

**Update:** `store/migration/sqlite/LATEST.sql` - Add table definition

### Phase 2: Store Layer

**File:** `store/agent.go` - Add types

```go
type ReindexCheckpoint struct {
    ID              int32
    TenantID        int32
    Audience        string
    TotalChunks     int32
    ProcessedChunks int32
    CurrentBatch    int32
    TotalBatches    int32
    BatchSize       int32
    Status          string  // "in_progress", "completed", "failed"
    ErrorMessage    string
    ErrorBatch      *int32
    StartedAt       time.Time
    UpdatedAt       time.Time
    CompletedAt     *time.Time
}

type FindReindexCheckpoint struct {
    TenantID *int32
    Audience *string
    Status   *string
}
```

**File:** `store/driver.go` - Add interface methods

```go
UpsertReindexCheckpoint(ctx context.Context, checkpoint *ReindexCheckpoint) (*ReindexCheckpoint, error)
GetReindexCheckpoint(ctx context.Context, find *FindReindexCheckpoint) (*ReindexCheckpoint, error)
DeleteReindexCheckpoint(ctx context.Context, tenantID int32, audience string) error
```

**File:** `store/db/sqlite/agent.go` - Implement methods

### Phase 3: VectorDB Integration

**File:** `server/router/api/v1/agent/vectordb_lance.go`

Modify `Insert()` to support:
1. Starting from a specific batch offset
2. Updating checkpoint after each successful batch
3. Automatic retry with exponential backoff

```go
type InsertOptions struct {
    StartBatch     int
    CheckpointFunc func(batch, processed int) error  // Called after each batch
    MaxRetries     int
    RetryDelay     time.Duration
}

func (db *LanceVectorDB) InsertWithResume(ctx context.Context, docs []Document, opts InsertOptions) error {
    // ... implementation
}
```

### Phase 4: Service Layer

**File:** `server/router/api/v1/agent/service.go`

Modify `ReindexTenantContent()`:

```go
func (s *Service) ReindexTenantContent(ctx context.Context, tenantID int32, resume bool) error {
    // 1. Check for existing failed checkpoint if resume=true
    // 2. Create or update checkpoint
    // 3. Call InsertWithResume with checkpoint callback
    // 4. Mark checkpoint complete or failed
}
```

### Phase 5: API Endpoint

**File:** `server/router/api/v1/agent/handlers.go`

Modify `HandleReindexTenant`:

```go
// POST /api/v1/agent/:slug/reindex?resume=true
func (h *Handler) HandleReindexTenant(c echo.Context) error {
    resume := c.QueryParam("resume") == "true"
    // ...
}
```

### Phase 6: Frontend UI

**File:** `web/src/store/v2/agentAdmin.ts`

Add checkpoint status tracking:

```typescript
interface ReindexStatus {
    status: 'idle' | 'in_progress' | 'failed' | 'completed';
    progress?: {
        currentBatch: number;
        totalBatches: number;
        processedChunks: number;
        totalChunks: number;
    };
    error?: string;
    canResume: boolean;
}
```

**File:** `web/src/pages/AgentAdmin.tsx`

Update Rebuild Index button:
- Show progress bar during indexing
- Show "Resume" button when status is 'failed'
- Display error message and failed batch number

---

## API Changes

### Reindex Endpoint

```
POST /api/v1/agent/:slug/reindex
Query params:
  - resume=true  (optional) Resume from last failed checkpoint
  - force=true   (optional) Delete checkpoint and start fresh

Response (during progress):
{
    "status": "in_progress",
    "progress": {
        "currentBatch": 121,
        "totalBatches": 137,
        "processedChunks": 3025,
        "totalChunks": 3401
    }
}

Response (on failure):
{
    "status": "failed",
    "error": "OpenRouter API error: No successful provider responses.",
    "errorBatch": 121,
    "canResume": true
}
```

### Reindex Status Endpoint (New)

```
GET /api/v1/agent/:slug/reindex/status

Response:
{
    "status": "failed",
    "progress": {
        "currentBatch": 120,
        "totalBatches": 137,
        "processedChunks": 3000,
        "totalChunks": 3401
    },
    "error": "OpenRouter API error: No successful provider responses.",
    "errorBatch": 121,
    "startedAt": "2026-01-31T00:00:00Z",
    "updatedAt": "2026-01-31T00:09:49Z",
    "canResume": true
}
```

---

## Retry Logic

Before persisting failure, attempt automatic retry:

```go
const (
    maxRetries   = 3
    initialDelay = 5 * time.Second
    maxDelay     = 60 * time.Second
)

func retryWithBackoff(fn func() error) error {
    var lastErr error
    delay := initialDelay

    for attempt := 0; attempt < maxRetries; attempt++ {
        if err := fn(); err != nil {
            lastErr = err
            slog.Warn("batch failed, retrying",
                "attempt", attempt+1,
                "maxRetries", maxRetries,
                "delay", delay,
                "error", err)
            time.Sleep(delay)
            delay = min(delay*2, maxDelay)
            continue
        }
        return nil
    }
    return lastErr
}
```

---

## UI Mockup

```
┌─────────────────────────────────────────────────────────────────┐
│  RAG Index                                                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Status: ❌ Failed at batch 121/137                              │
│                                                                  │
│  ████████████████████████░░░░░░  88% (3000/3401 chunks)         │
│                                                                  │
│  Error: OpenRouter API error: No successful provider responses.  │
│                                                                  │
│  [Resume Indexing]  [Start Fresh]                                │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Files to Modify

| File | Changes |
|------|---------|
| `store/migration/sqlite/0.25/21__reindex_checkpoints.sql` | New migration |
| `store/migration/sqlite/LATEST.sql` | Add table |
| `store/agent.go` | Add types and store methods |
| `store/driver.go` | Add interface methods |
| `store/db/sqlite/agent.go` | Implement CRUD |
| `store/db/mysql/agent.go` | Add stubs |
| `store/db/postgres/agent.go` | Add stubs |
| `server/router/api/v1/agent/vectordb_lance.go` | Add InsertWithResume |
| `server/router/api/v1/agent/service.go` | Update ReindexTenantContent |
| `server/router/api/v1/agent/handlers.go` | Update endpoint, add status endpoint |
| `server/router/api/v1/v1.go` | Register new status endpoint |
| `web/src/store/v2/agentAdmin.ts` | Add checkpoint state |
| `web/src/pages/AgentAdmin.tsx` | Add Resume button, progress UI |

---

## Testing

1. **Unit tests:**
   - Checkpoint CRUD operations
   - Retry logic with mock failures
   - Resume from specific batch

2. **Integration tests:**
   - Start reindex, simulate failure, verify checkpoint saved
   - Resume reindex, verify starts from correct batch
   - Complete reindex, verify checkpoint cleaned up

3. **Manual testing:**
   - Trigger reindex on large KB
   - Kill server mid-process
   - Restart and resume
   - Verify no duplicate chunks

---

## Verification

After implementation:

```bash
# 1. Start reindex
curl -X POST http://localhost:5230/api/v1/agent/test-tenant/reindex

# 2. Check status (while running or after failure)
curl http://localhost:5230/api/v1/agent/test-tenant/reindex/status

# 3. Resume after failure
curl -X POST "http://localhost:5230/api/v1/agent/test-tenant/reindex?resume=true"

# 4. Force fresh start
curl -X POST "http://localhost:5230/api/v1/agent/test-tenant/reindex?force=true"
```

---

## Future Enhancements

1. **SSE Progress Streaming** - Real-time progress updates to frontend
2. **Batch-level checkpointing** - Store which specific chunks in a batch succeeded
3. **Parallel batch processing** - Process multiple batches concurrently with checkpointing
4. **Scheduled retries** - Automatically retry failed reindexes after delay

---

*Document Version: 1.0*
*Created: 2026-01-31*
