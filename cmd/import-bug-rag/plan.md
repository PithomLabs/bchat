# Plan: RAG-Based Bug History Insights

**Bug ID:** 051 (extension)
**Date:** 2026-07-30
**Status:** Implemented

---

## 1. Background

### The Problem

bchat is a multi-tenant AI chat agent platform. Over 50 bugs (001–051) have been resolved through an intensive iterative process involving plan → adversarial review → code → adversarial review → signoff cycles. Each bug folder under `bchat/bugs/` contains rich history: root cause analyses, implementation decisions, adversarial findings, and resolution patterns.

However, this knowledge is trapped in local markdown files. When a new ticket is created, the agent cannot learn from past resolutions. The same issues (RAG indexing failures, permission errors, migration conflicts) recur because there is no mechanism for the agent to access historical resolution data.

### The Goal

Build a RAG-based mechanism that draws insights from all files under `bchat/bugs/` and surfaces them as auto-suggested resolutions when a new ticket is created. Test locally with SQLite + LanceDB, then port to CockroachDB for the hackathon demo.

### Scope

Single-tenant demo only. Import bug corpus into the existing active tenant (tenant 19, slug `hackathon-demo`). Cross-tenant abstraction deferred post-hackathon.

---

## 2. Architecture

### Data Flow

```
bchat/bugs/001-051/*.md
        │
        │  cmd/import-bug-rag reads, concatenates per folder
        ▼
AgentSourceFile rows
  tenant_id=19, audience_type="internal", file_type="bug"
        │
        │  ReindexTenantContent(tenant_id=19)
        ▼
LanceDB local: build/data/lancedb/kb_documents_<dim>
CockroachDB: agent_vectors table
        │
        │  SearchQuery{TenantID: 19, ContentTypes: ["bug_section"]}
        ▼
InferResolutionForNewTicket
  search 1: ContentTypes=["ticket"]     → similar tickets
  search 2: ContentTypes=["bug_section"] → bug history
        │
        ▼
ticket.internal_notes = merged suggestion
```

### Key Design Decisions

| Decision | Choice |
|----------|--------|
| Bug content storage | `AgentSourceFile` with `file_type="bug"` under existing active tenant |
| Chunk granularity | One `AgentSourceFile` per bug folder (concatenated markdown) |
| Search trigger | Extend `InferResolutionForNewTicket` — two searches, merge results |
| Local test stack | SQLite + LanceDB local (`task run:rag`) |
| Hackathon stack | CockroachDB + CockroachDB native vector (`-tags cockroach`) |

---

## 3. Implementation

### 3.1 New File: `cmd/import-bug-rag/main.go`

A standalone Go command that imports bug corpus as `AgentSourceFile` entries.

#### Import Flow

```
For each bug folder in bchat/bugs/001-051/:
  1. Read all .md files
  2. Concatenate with headers → raw markdown string
  3. Compute SHA-256 content hash
  4. Check if source file with same (tenant_id, audience_type="internal", file_type="bug", content_hash) exists
  5. If not exists: INSERT INTO agent_source_files
```

#### Key Functions

```go
func importBugRAG(ctx context.Context, db *sql.DB, driver string, tenantID, creatorID int32, bug BugFolder) (created, skipped int, err error)
func buildRawContent(bug BugFolder) string
func sourceFileExists(ctx context.Context, db *sql.DB, driver string, tenantID int32, audienceType, fileType, contentHash string) (bool, error)
func createSourceFile(ctx context.Context, db *sql.DB, driver string, tenantID int32, audienceType, fileType, content, contentHash string) error
```

#### SQL

```sql
-- Existence check
SELECT EXISTS(SELECT 1 FROM agent_source_files
  WHERE tenant_id=? AND audience_type='internal' AND file_type='bug' AND content_hash=?);

-- Insert
INSERT INTO agent_source_files
  (tenant_id, audience_type, file_type, content, content_hash, version)
VALUES (?, 'internal', 'bug', ?, ?, 1);
```

#### Reindex Trigger

After all inserts, restart the server. Bug 004's auto-bootstrap detects empty LanceDB table and reindexes automatically. The import script's output documents this clearly.

#### Idempotency

Deduplicate by `content_hash`. Re-runs skip already-imported folders.

---

### 3.2 Modified File: `server/router/api/v1/agent/service.go`

**Function:** `InferResolutionForNewTicket` (line 5589)

#### Before

Single `SearchQuery` with `ContentTypes: ["ticket"]`. Only searched similar past tickets in the same tenant.

#### After

Two searches, merged results:

```go
// Search 1: similar tickets in the same tenant
ticketResult, ticketErr := vectorDB.Search(ctx, SearchQuery{
    QueryText:    queryText,
    TenantID:     tenantID,
    ContentTypes: []string{"ticket"},
    TopK:         3,
    MinScore:     0.7,
})

// Search 2: relevant bug history
bugResult, bugErr := vectorDB.Search(ctx, SearchQuery{
    QueryText:    queryText,
    TenantID:     tenantID,
    ContentTypes: []string{"bug_section"},
    TopK:         3,
    MinScore:     0.5,
})
```

Both searches are attempted independently. Errors are logged but do not block the other search. Results are merged into `ticket.internal_notes` with two sections:
- "Based on N similar past tickets"
- "Relevant Bug History (N matches)"

If neither search returns results, the function returns early without modifying `internal_notes`.

---

## 4. How to Run

### Prerequisites

```bash
# Download LanceDB CGO library (one-time)
task setup:lancedb

# Build server with RAG support
task build:backend
```

### Step 1: Import Bug Corpus

```bash
go run ./cmd/import-bug-rag/
```

Expected output:
```
=== Bug History RAG Import ===
Found 51 bug folders

=== Import Complete ===
Created: 50 source files
Skipped: 0 (already exist)
Tenant ID: 19
```

Verify:
```bash
sqlite3 build/data/memos_dev.db \
  "SELECT count(*) FROM agent_source_files WHERE file_type='bug' AND tenant_id=19"
# Expected: 50
```

### Step 2: Start Server with RAG

```bash
task run:rag
```

The server will:
1. Detect that source files exist but LanceDB table is empty
2. Auto-trigger bootstrap reindexing in the background
3. Chunk bug content, generate embeddings, and insert into LanceDB

**Note:** `OPENROUTER_API_KEY` must be set for embeddings to succeed. Without it, the reindex will fail with "embedding provider misconfigured". Set it via:
```bash
export OPENROUTER_API_KEY=sk-or-v1-xxx
task run:rag
```

### Step 3: Test Inference

Create a new ticket via API or UI:

```bash
curl -X POST http://localhost:5230/api/v1/tickets \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Test RAG inference",
    "description": "/m/test",
    "status": "OPEN",
    "priority": "MEDIUM",
    "type": "TASK"
  }'
```

Check `internal_notes`:
```bash
sqlite3 build/data/memos_dev.db \
  "SELECT id, substr(internal_notes, 1, 300) FROM tickets WHERE id=<new_id>"
```

Expected: `internal_notes` contains "Suggested Resolution (Auto-generated)" with sections for similar tickets and bug history.

### Step 4: Verify LanceDB Index

```bash
ls -la build/data/lancedb/
# Expected: tenant directories (e.g., 13/, 14/, 15/, 19/) with LanceDB tables
```

### Idempotency

Re-running the import script is safe:
```bash
go run ./cmd/import-bug-rag/
# Expected: Created: 0, Skipped: 50 (already exist)
```

---

## 5. Files Modified

| File | Action | Description |
|------|--------|-------------|
| `cmd/import-bug-rag/main.go` | NEW | Import bug corpus as AgentSourceFile entries |
| `server/router/api/v1/agent/service.go` | MODIFY | Extend `InferResolutionForNewTicket` for dual search |

---

## 6. Validation

| Check | Command | Expected |
|-------|---------|----------|
| Compile import script | `go build ./cmd/import-bug-rag/` | Clean |
| Compile server | `task build:backend` | Clean |
| Run tests | `go test ./server/router/api/v1/agent/...` | Pass |
| Import corpus | `go run ./cmd/import-bug-rag/` | 50 source files created |
| Verify source files | `sqlite3 ... count(*) WHERE file_type='bug'` | 50 |
| Idempotency | Re-run import | Skips existing |
| Server starts | `task run:rag` | Server on port 8081 |
| Auto-reindex | Server logs | "RAG vector database table is empty... Auto-triggering bootstrap" |
| Test inference | Create ticket → check `internal_notes` | Contains bug history |

---

## 7. Hackathon Demo Flow

1. Show bug corpus import: `go run ./cmd/import-bug-rag/`
2. Restart server, show auto-bootstrap reindexing in logs
3. Create a new ticket that relates to an existing bug
4. Show `internal_notes` auto-populated with:
   - Similar past tickets (from current tenant)
   - Relevant bug history snippets (from bug corpus)
5. Highlight: CockroachDB stores both transactional data AND embeddings — single system of record for agentic memory

### Porting to CockroachDB

```bash
# Build with CockroachDB tag
go build -tags cockroach ./bin/memos/main.go

# Set CockroachDB DSN
export COCKROACH_DSN="postgresql://user:pass@host:26257/db?sslmode=require"

# Run import against CockroachDB
go run ./cmd/import-bug-rag/

# Start server
./build/memos --mode dev --data build/data
```

No code changes needed — `vectordb_cockroach.go` handles vector storage.

---

## 8. Edge Cases

| Case | Behavior |
|------|----------|
| Bug folder has no .md files | Skipped early |
| Import interrupted mid-folder | Partial import; re-run completes remaining |
| Server started without restarting after import | Auto-bootstrap detects new source files and reindexes |
| No similar tickets or bug history found | `internal_notes` left empty; no error |
| Embedding API unavailable | Chunks skipped; reindex fails gracefully; retry on next restart |
| Content hash collision | Extremely unlikely with 64-bit hash; would skip legitimate updates |

---

## 9. Rollback

If the bug-history corpus causes issues:

```sql
-- Delete bug source files
DELETE FROM agent_source_files WHERE file_type='bug' AND tenant_id=19;

-- Revert service.go changes
git checkout server/router/api/v1/agent/service.go
```

Then restart the server. The next auto-bootstrap will reindex remaining source files.

---

## 10. References

- [Bug 051 Plan](../.kilo/plans/1785365112142-import-pipeline-memo-comments.md)
- [Bug 004 Auto-Bootstrap Plan](bugs/004/plan.md)
- [RAG Pipeline Docs](../docs/DOCS_RAG_PIPELINE.MD)
- [LanceDB Docs](../docs/DOCS_LANCEDB.MD)
- [Ticket Service](../server/router/api/v1/ticket_service.go)
- [Import Script](../cmd/import-bugs/main.go)
