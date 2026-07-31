# Plan: Per-Ticket RAG Indexing Prototype

**Bug ID:** 052
**Date:** 2026-07-31
**Status:** Draft — Awaiting Adversarial Review

---

## 1. Background

### User Prompt

> under /home/chaschel/Documents/go/bchat/cmd/import-bug-rag read plan.md I want to replicate Jira's Ask Rovo feature where for each ticket created, the system (bchat) using AI will infer from previous similar tickets and suggest possible resolution using RAG, embedding, FTS and the like, is it better to do the RAG indexing per ticket or do it as one big lump operation? note that I am only focusing for now about this RAG indexing operation before the actual ticket inference

### AI Response (Verbatim)

> **Per-ticket indexing is better** for a Jira "Ask Rovo" feature, for these reasons:
>
> | Factor | Per-Ticket | Lump/Batch |
> |--------|-----------|------------|
> | **Freshness** | Instant - suggestions always reflect latest tickets | Stale between batches |
> | **User experience** | No delay on new ticket creation | Users miss suggestions for recently created tickets |
> | **Latency** | ~1-3s added to ticket creation (embedding API call) | N/A until batch runs |
> | **Simplicity** | Direct - one ticket = one `reindexFileVersion` call | Need dirty-flag tracking + periodic sweep |
> | **Existing infra** | Already supported via `reindexFileVersion` at `service.go:563` | Already supported via `ReindexTenantContentWithResume` |
>
> The existing codebase already has the building blocks:
>
> 1. **`reindexFileVersion`** (`service.go:563`) - indexes a single `(tenant, audience, file_type, version)` - perfect for per-ticket
> 2. **`InsertWithCheckpoint`** (`vectordb_lance.go:493`) - handles batch embedding with retry, circuit breaker, and resume
> 3. **Per-tenant LanceDB tables** - isolated, so per-ticket inserts don't block other tenants
>
> ### The catch
>
> The current `ReindexTenantContentWithResume` reads **all** source files for a tenant and reindexes. For per-ticket, you'd want to call `reindexFileVersion` directly for just the new ticket's source file, rather than re-scanning everything.
>
> ### Recommendation
>
> **Hybrid approach:**
>
> 1. **On ticket creation**: Store as `AgentSourceFile` with `file_type="ticket"`, then immediately call `reindexFileVersion` for that single file
> 2. **Periodic safety net**: Every N minutes, a lightweight sweep catches any failures (dirty-flag or version comparison)
> 3. **Bulk import mode**: For initial historical import, use the existing lump `ReindexTenantContentWithResume`

### Design Decisions (Q&A)

| # | Question | Answer |
|---|----------|--------|
| 1 | What content to index per ticket? | Just title + description (no status/priority/labels) |
| 2 | Should ticket edits trigger re-index? | Yes — if ticket OR comments are edited, reindex with version increment |
| 3 | Sync or async indexing? | Async (goroutine) |
| 4 | Which tickets to test with? | Just ticket #120 (`Bug #002: Repair Frontend Dependency Provenance`, tenant 19) |
| 5 | Version retention policy? | Keep last 5 versions (matches existing KB/POLICY retention) |

### Prototype Scope

Single ticket: **Ticket #120** (`Bug #002: Repair Frontend Dependency Provenance`, tenant 19, slug `hackathon-demo`).

---

## 2. Current State Analysis

### What Exists

| Component | Status | Location |
|-----------|--------|----------|
| `InferResolutionForNewTicket` | Implemented, searches `ContentTypes: ["ticket"]` and `["bug_section"]` | `service.go:5589` |
| Called on ticket creation | Yes, as goroutine | `ticket_service.go:166` |
| Ticket vector search | Queries `ContentTypes: ["ticket"]` but **no tickets are indexed** | `service.go:5606-5612` |
| Bug corpus indexing | Works — `file_type="bug"` via `import-bug-rag` | `cmd/import-bug-rag/main.go` |
| `reindexFileVersion` | Indexes single `(tenant, audience, file_type, version)` — **unexported** | `service.go:563` |
| `UpsertAgentSourceFile` | Auto-increments version per `(tenant, audience, file_type)` | `store/db/sqlite/agent.go:1117` |

### The Gap

`InferResolutionForNewTicket` searches for `ContentTypes: ["ticket"]` but **no code ever indexes ticket content into the vector DB**. The search always returns empty. This is the missing piece.

### Data Model: How Tickets Relate to Comments

```
Ticket (tickets table)
  ├── title, description, status, priority, ...
  └── description = "/m/<memo_uid>"  (links to parent memo)

Memo (memo table)
  └── id, uid, content, creator_id, ...

Comment = Memo + memo_relation(type='COMMENT')
  ├── memo_id = comment memo
  ├── related_memo_id = parent memo (the ticket's description memo)
  └── type = 'COMMENT'
```

So to get full ticket content for indexing: title + description + all comment memos.

---

## 3. Implementation Plan

### Step 1: Export `reindexFileVersion`

**File:** `server/router/api/v1/agent/service.go` (line 563)

Rename `reindexFileVersion` → `ReindexFileVersion` (export it). Update all internal callers.

```go
// Before:
func (s *Service) reindexFileVersion(...)

// After:
func (s *Service) ReindexFileVersion(...)
```

Internal callers to update:
- `ReindexTenantContent` (service.go:688)
- `ReindexTenantContentWithResume` (service.go:1020)

### Step 2: Create `IndexTicketContent` Helper

**File:** `server/router/api/v1/agent/service.go`

New public method that builds ticket content and indexes it:

```go
func (s *Service) IndexTicketContent(ctx context.Context, tenantID int32, ticket *store.Ticket, comments []*store.Memo) (int, error) {
    // Build content: title + description + comments
    var sb strings.Builder
    fmt.Fprintf(&sb, "# %s\n\n%s", ticket.Title, ticket.Description)

    if len(comments) > 0 {
        fmt.Fprintf(&sb, "\n\n## Comments\n\n")
        for _, c := range comments {
            fmt.Fprintf(&sb, "---\n\n%s\n\n", c.Content)
        }
    }

    content := sb.String()
    contentHash := sha256Hash(content)

    // Upsert as AgentSourceFile (auto-increments version)
    sourceFile, err := s.store.UpsertAgentSourceFile(ctx, &store.AgentSourceFile{
        TenantID:     tenantID,
        AudienceType: "internal",
        FileType:     "ticket",
        Content:      content,
        ContentHash:  contentHash,
    })
    if err != nil {
        return 0, fmt.Errorf("failed to upsert source file: %w", err)
    }

    // Index this single version into vector DB
    return s.ReindexFileVersion(ctx, tenantID, "internal", "ticket", sourceFile.Version, content, 0)
}
```

### Step 3: Hook into Ticket Creation

**File:** `server/router/api/v1/ticket_service.go` (after line 166)

After `InferResolutionForNewTicket` goroutine, add ticket indexing:

```go
// After InferResolutionForNewTicket goroutine:
if s.agentHandler != nil {
    go func() {
        ctx := context.WithoutCancel(ctx)
        _, err := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, nil)
        if err != nil {
            slog.Error("failed to index new ticket for RAG", "ticket_id", ticket.ID, "error", err)
        }
    }()
}
```

### Step 4: Hook into Ticket Update

**File:** `server/router/api/v1/ticket_service.go` (line 355, after `UpdateTicket`)

After ticket is updated, re-index with comments:

```go
ticket, err := s.Store.UpdateTicket(ctx, update)
if err != nil {
    return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update ticket").SetInternal(err)
}

// Re-index ticket content for RAG
if s.agentHandler != nil && ticket.TenantID != nil {
    go func() {
        ctx := context.WithoutCancel(ctx)
        // Fetch comments for this ticket
        comments, _ := s.getTicketComments(ctx, ticket)
        _, err := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, comments)
        if err != nil {
            slog.Error("failed to re-index updated ticket for RAG", "ticket_id", ticket.ID, "error", err)
        }
    }()
}
```

Add helper to fetch ticket comments:

```go
func (s *APIV1Service) getTicketComments(ctx context.Context, ticket *store.Ticket) ([]*store.Memo, error) {
    if !strings.HasPrefix(ticket.Description, "/m/") {
        return nil, nil
    }
    memoUID := strings.TrimPrefix(ticket.Description, "/m/")
    parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
    if err != nil || parentMemo == nil {
        return nil, nil
    }
    commentType := store.MemoRelationComment
    relations, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
        RelatedMemoID: &parentMemo.ID,
        Type:          &commentType,
    })
    if err != nil {
        return nil, err
    }
    var comments []*store.Memo
    for _, rel := range relations {
        memo, err := s.Store.GetMemo(ctx, &store.FindMemo{ID: &rel.MemoID})
        if err == nil && memo != nil {
            comments = append(comments, memo)
        }
    }
    return comments, nil
}
```

### Step 5: Hook into Comment Creation/Edit

**File:** `server/router/api/v1/memo_service.go`

In `CreateMemoComment` (after line 594, after relation is created), trigger parent ticket re-index:

```go
// After UpsertMemoRelation:
go func() {
    ctx := context.WithoutCancel(ctx)
    // Find parent ticket via description link
    descriptionLink := "/m/" + relatedMemo.UID
    tickets, _ := s.Store.ListTickets(ctx, &store.FindTicket{Description: &descriptionLink})
    if len(tickets) > 0 {
        ticket := tickets[0]
        if ticket.TenantID != nil {
            comments, _ := s.getTicketComments(ctx, ticket)
            s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, comments)
        }
    }
}()
```

In `UpdateMemo` (after memo content is updated), if the memo is a comment on a ticket, re-index the parent ticket.

### Step 6: Verify with Ticket #120

```bash
# 1. Build and start server
task build:backend && task run:rag

# 2. Trigger indexing for ticket #120 via the reindex endpoint
curl -X POST http://localhost:5230/api/v1/agent/hackathon-demo/reindex

# 3. Check source file was created
sqlite3 build/data/memos_dev.db \
  "SELECT id, version, length(content) FROM agent_source_files WHERE file_type='ticket' AND tenant_id=19"

# 4. Check LanceDB has ticket chunks
ls -la build/data/lancedb/19/

# 5. Test inference: create a new similar ticket
curl -X POST http://localhost:5230/api/v1/tickets \
  -H "Content-Type: application/json" \
  -d '{"title":"Frontend build broken","description":"Dependency issue with React","status":"OPEN","priority":"MEDIUM","type":"TASK"}'

# 6. Check internal_notes has suggestions
sqlite3 build/data/memos_dev.db \
  "SELECT substr(internal_notes, 1, 500) FROM tickets ORDER BY id DESC LIMIT 1"
```

---

## 4. Files Modified

| File | Action | Description |
|------|--------|-------------|
| `server/router/api/v1/agent/service.go` | MODIFY | Export `ReindexFileVersion`, add `IndexTicketContent` |
| `server/router/api/v1/ticket_service.go` | MODIFY | Hook indexing on create + update |
| `server/router/api/v1/memo_service.go` | MODIFY | Hook re-indexing on comment create/edit |

---

## 5. Validation

| Check | Command | Expected |
|-------|---------|----------|
| Compile | `task build:backend` | Clean |
| Ticket #120 indexed | `sqlite3 ... count(*) WHERE file_type='ticket' AND tenant_id=19` | ≥1 |
| LanceDB chunks exist | `ls build/data/lancedb/19/` | Table files present |
| New ticket gets suggestions | Create ticket → check `internal_notes` | Contains "Suggested Resolution" |
| Edit ticket re-indexes | Update title → check version increments | version=2 |
| Add comment re-indexes | Add comment → check content includes comments | Comment text in indexed content |

---

## 6. Edge Cases

| Case | Behavior |
|------|----------|
| Ticket has no comments | Index title + description only |
| Ticket has 10+ comments | All comments included in indexed content |
| Embedding API unavailable | Reindex fails gracefully; retry on next edit |
| Same content re-indexed | Content hash dedup; version still increments (content may differ slightly) |
| Comment on non-ticket memo | No-op (no ticket found via description link) |
| Multi-tenant isolation | Tenant ID from context; cross-tenant indexing blocked |

---

## 7. Adversarial Review Prompt

The following prompt should be used for adversarial plan review:

```
You are an adversarial code reviewer. Review this implementation plan for bugs/052
(Per-Ticket RAG Indexing Prototype). Focus on:

1. CORRECTNESS: Are there race conditions with async indexing? What if a ticket is
   edited while the previous index operation is still running? The per-tenant mutex
   in reindexFileVersion serializes writes, but is that sufficient?

2. DATA CONSISTENCY: If IndexTicketContent fails after UpsertAgentSourceFile succeeds,
   the vector DB will be stale (old version indexed, new version in SQLite). Is this
   acceptable for the prototype? What's the recovery path?

3. PERFORMANCE: Each ticket creation triggers an embedding API call (~1-3s). For
   bulk ticket imports (e.g., migrating from Jira), this would be O(N) API calls.
   Should we add a batch mode for bulk imports, or is async goroutine sufficient?

4. COMMENT RE-INDEX CHURN: Adding a comment re-indexes the entire ticket content.
   If someone adds 5 comments in rapid succession, we get 5 embedding API calls
   for the same ticket. Should we debounce/dedupe? What's the right strategy?

5. CONTENT FORMAT: We index "# Title\n\nDescription\n\n## Comments\n\n---\n\n..."
   Is this the right chunking boundary? Should comments be separate chunks instead
   of appended to the ticket content?

6. SECURITY: Can a user inject malicious content into the RAG index via ticket
   title/description/comments? Are there injection vectors in the embedding API
   or vector search?

7. OBSERVABILITY: How do we know if indexing succeeded or failed? Should we add
   a status field to tickets or a separate tracking table?

8. TESTABILITY: The plan says "verify with ticket #120" but doesn't specify
   automated tests. What unit/integration tests are needed before shipping?

Provide a severity rating (Critical/High/Medium/Low) for each finding and a
recommended fix or mitigation.
```

---

## 8. Rollback

If the per-ticket indexing causes issues:

```sql
-- Delete ticket source files
DELETE FROM agent_source_files WHERE file_type='ticket' AND tenant_id=19;

-- Revert code changes
git checkout server/router/api/v1/agent/service.go
git checkout server/router/api/v1/ticket_service.go
git checkout server/router/api/v1/memo_service.go
```

Then restart the server. `InferResolutionForNewTicket` will continue to search (finding nothing), but no errors will occur.

---

## 9. Future Work (Post-Prototype)

| Item | Description |
|------|-------------|
| Periodic safety net | Background sweep every N minutes to re-index any missed tickets |
| Bulk import mode | Batch indexing for historical ticket migration |
| Comment chunking | Separate chunks per comment for finer-grained retrieval |
| Metadata fields | Index status, priority, labels as vector DB metadata for filtered search |
| Cross-tenant | Extend to multi-tenant ticket indexing with proper isolation |
| LLM-powered inference | Use indexed tickets to generate resolution suggestions (Bug 051 scope) |
