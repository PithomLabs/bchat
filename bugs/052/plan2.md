# Plan: Per-Ticket RAG Indexing Prototype (Revised)

**Bug ID:** 052
**Date:** 2026-07-31
**Status:** Draft — Awaiting Adversarial Review (Round 2)
**Revision:** Based on adversarial review of plan.md

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
> ### Recommendation
>
> **Hybrid approach:**
>
> 1. **On ticket creation**: Store as `AgentSourceFile` with `file_type="ticket"`, then immediately call `reindexFileVersion` for that single file
> 2. **Periodic safety net**: Every N minutes, a lightweight sweep catches any failures
> 3. **Bulk import mode**: For initial historical import, use the existing lump `ReindexTenantContentWithResume`

### Design Decisions (Q&A)

| # | Question | Answer |
|---|----------|--------|
| 1 | What content to index per ticket? | Just title + description (no status/priority/labels) |
| 2 | Should ticket edits trigger re-index? | Yes — if ticket OR comments are edited, reindex with version increment |
| 3 | Sync or async indexing? | Async (goroutine) |
| 4 | Which tickets to test with? | Just ticket #120 (`Bug #002: Repair Frontend Dependency Provenance`, tenant 19) |
| 5 | Version retention policy? | Keep last 5 versions (matches existing KB/POLICY retention) |

### Adversarial Review Findings Addressed

This revision addresses 4 must-fix findings from the adversarial review:

| # | Finding | Severity | Resolution |
|---|---------|----------|------------|
| 1 | Version inflation on reindex failure (no content-hash dedup) | CRITICAL | Add content-hash check before upsert |
| 2 | Inference races ahead of indexing | HIGH | Chain: index → then infer (not parallel) |
| 3 | Silent failures in `getTicketComments` | HIGH | Return error, log on failure |
| 4 | `UpdateMemo` hook unspecified | HIGH | Full pseudo-code added |

Findings 5-9 (comment churn, no unit tests, deletion orphaning, injection surface, rename coverage) tracked as future work — acceptable for prototype.

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
| `UpsertAgentSourceFile` | **Always increments version** — no content-hash dedup | `store/db/sqlite/agent.go:1117` |

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

---

## 3. Implementation Plan

### Step 1: Export `reindexFileVersion`

**File:** `server/router/api/v1/agent/service.go` (line 563)

Rename `reindexFileVersion` → `ReindexFileVersion`. Update 4 internal callers:
- `service.go:665` (ReindexTenantContent, kb)
- `service.go:672` (ReindexTenantContent, policy)
- `service.go:768` (ReindexTenantContentWithResume, kb)
- `service.go:776` (ReindexTenantContentWithResume, policy)

No test files reference the unexported name (verified via grep).

### Step 2: Create `IndexTicketContent` Helper

**File:** `server/router/api/v1/agent/service.go`

New public method. **Key change from plan.md:** includes content-hash dedup to prevent version bloat (Finding 1), and calls `InferResolutionForNewTicket` after successful indexing (Finding 2).

```go
func (s *Service) IndexTicketContent(ctx context.Context, tenantID int32, ticket *store.Ticket, comments []*store.Memo, triggerInference bool) (int, error) {
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

    // CRITICAL: Check if latest version already has this content hash.
    // UpsertAgentSourceFile always increments version — skip if unchanged.
    existing, _ := s.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{
        TenantID:   &tenantID,
        FileType:   strPtr("ticket"),
        LatestOnly: true,
    })
    if existing != nil && existing.ContentHash == contentHash {
        // Content unchanged — reindex existing version, don't create new row
        chunks, err := s.ReindexFileVersion(ctx, tenantID, "internal", "ticket", existing.Version, content, 0)
        if err != nil {
            return 0, err
        }
        // Optionally trigger inference after re-index
        if triggerInference {
            s.InferResolutionForNewTicket(ctx, ticket)
        }
        return chunks, nil
    }

    // Content changed — upsert creates new version row
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

    chunks, err := s.ReindexFileVersion(ctx, tenantID, "internal", "ticket", sourceFile.Version, content, 0)
    if err != nil {
        return 0, err
    }

    // Chain: index first, then infer (Finding 2 — eliminates race)
    if triggerInference {
        s.InferResolutionForNewTicket(ctx, ticket)
    }

    return chunks, nil
}
```

### Step 3: Create `getTicketComments` Helper

**File:** `server/router/api/v1/ticket_service.go`

**Key change from plan.md:** returns `([]*store.Memo, error)` instead of swallowing errors (Finding 3).

```go
func (s *APIV1Service) getTicketComments(ctx context.Context, ticket *store.Ticket) ([]*store.Memo, error) {
    if !strings.HasPrefix(ticket.Description, "/m/") {
        return nil, nil
    }
    memoUID := strings.TrimPrefix(ticket.Description, "/m/")
    parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
    if err != nil {
        return nil, fmt.Errorf("failed to get parent memo: %w", err)
    }
    if parentMemo == nil {
        return nil, nil
    }
    commentType := store.MemoRelationComment
    relations, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
        RelatedMemoID: &parentMemo.ID,
        Type:          &commentType,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to list memo relations: %w", err)
    }
    var comments []*store.Memo
    for _, rel := range relations {
        memo, err := s.Store.GetMemo(ctx, &store.FindMemo{ID: &rel.MemoID})
        if err != nil {
            slog.Warn("failed to load comment memo", "memo_id", rel.MemoID, "error", err)
            continue
        }
        if memo != nil {
            comments = append(comments, memo)
        }
    }
    return comments, nil
}
```

### Step 4: Hook into Ticket Creation

**File:** `server/router/api/v1/ticket_service.go` (after line 166)

**Key change from plan.md:** remove the independent `InferResolutionForNewTicket` goroutine. Replace with single `IndexTicketContent` call that chains inference after indexing.

```go
ticket, err = s.Store.CreateTicket(ctx, ticket)
if err != nil {
    slog.Error("CreateTicket store error", "error", err)
    return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create ticket").SetInternal(err)
}

// Index ticket for RAG, then trigger inference (Finding 2 — chained, not parallel)
if s.agentHandler != nil && ticket.TenantID != nil {
    go func() {
        ctx := context.WithoutCancel(ctx)
        _, err := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, nil, true)
        if err != nil {
            slog.Error("failed to index new ticket for RAG", "ticket_id", ticket.ID, "error", err)
        }
    }()
}

slog.Info("CreateTicket success", "id", ticket.ID)
return c.JSON(http.StatusOK, convertTicketFromStore(ticket))
```

### Step 5: Hook into Ticket Update

**File:** `server/router/api/v1/ticket_service.go` (after line 355)

```go
ticket, err := s.Store.UpdateTicket(ctx, update)
if err != nil {
    return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update ticket").SetInternal(err)
}

// Re-index ticket content for RAG
if s.agentHandler != nil && ticket.TenantID != nil {
    go func() {
        ctx := context.WithoutCancel(ctx)
        comments, err := s.getTicketComments(ctx, ticket)
        if err != nil {
            slog.Warn("failed to fetch comments for ticket re-index, indexing title+description only",
                "ticket_id", ticket.ID, "error", err)
        }
        _, idxErr := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, comments, false)
        if idxErr != nil {
            slog.Error("failed to re-index updated ticket for RAG", "ticket_id", ticket.ID, "error", idxErr)
        }
    }()
}

return c.JSON(http.StatusOK, convertTicketFromStore(ticket))
```

### Step 6: Hook into Comment Creation

**File:** `server/router/api/v1/memo_service.go` (after line 594, after `UpsertMemoRelation`)

```go
// After UpsertMemoRelation:
go func() {
    ctx := context.WithoutCancel(ctx)
    descriptionLink := "/m/" + relatedMemo.UID
    tickets, _ := s.Store.ListTickets(ctx, &store.FindTicket{Description: &descriptionLink})
    if len(tickets) == 0 {
        return
    }
    ticket := tickets[0]
    if ticket.TenantID == nil {
        return
    }
    comments, err := s.getTicketComments(ctx, ticket)
    if err != nil {
        slog.Warn("failed to fetch comments for ticket re-index after comment creation",
            "ticket_id", ticket.ID, "error", err)
    }
    _, idxErr := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, comments, false)
    if idxErr != nil {
        slog.Error("failed to re-index ticket after comment creation", "ticket_id", ticket.ID, "error", idxErr)
    }
}()
```

### Step 7: Hook into Comment Edit (UpdateMemo)

**File:** `server/router/api/v1/memo_service.go` (after line 437, after `s.Store.UpdateMemo`)

**Finding 4 resolution:** Full pseudo-code with guard conditions.

```go
if err = s.Store.UpdateMemo(ctx, update); err != nil {
    return nil, status.Errorf(codes.Internal, "failed to update memo")
}

// Re-index parent ticket if this memo is a comment and content changed
if s.agentHandler != nil && update.Content != nil {
    // Guard 1: Check if this memo is a comment on something
    commentType := store.MemoRelationComment
    parentRelations, relErr := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
        MemoID: &memo.ID,
        Type:   &commentType,
    })
    if relErr == nil && len(parentRelations) > 0 {
        // This memo IS a comment. Find the parent memo.
        parentMemoID := parentRelations[0].RelatedMemoID
        parentMemo, _ := s.Store.GetMemo(ctx, &store.FindMemo{ID: &parentMemoID})
        if parentMemo != nil {
            // Guard 2: Check if parent memo is linked to a ticket
            descriptionLink := "/m/" + parentMemo.UID
            tickets, _ := s.Store.ListTickets(ctx, &store.FindTicket{Description: &descriptionLink})
            if len(tickets) > 0 {
                ticket := tickets[0]
                if ticket.TenantID != nil {
                    go func() {
                        ctx := context.WithoutCancel(ctx)
                        comments, err := s.getTicketComments(ctx, ticket)
                        if err != nil {
                            slog.Warn("failed to fetch comments for ticket re-index after comment edit",
                                "ticket_id", ticket.ID, "error", err)
                        }
                        _, idxErr := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, comments, false)
                        if idxErr != nil {
                            slog.Error("failed to re-index ticket after comment edit",
                                "ticket_id", ticket.ID, "error", idxErr)
                        }
                    }()
                }
            }
        }
    }
}

memo, err = s.Store.GetMemo(ctx, &store.FindMemo{ID: &memo.ID})
// ... rest of existing code
```

### Step 8: Verify with Ticket #120

```bash
# 1. Build and start server
task build:backend && task run:rag

# 2. Trigger indexing for ticket #120 via the reindex endpoint
curl -X POST http://localhost:5230/api/v1/agent/hackathon-demo/reindex

# 3. Check source file was created (version=1)
sqlite3 build/data/memos_dev.db \
  "SELECT id, version, length(content), content_hash FROM agent_source_files WHERE file_type='ticket' AND tenant_id=19"

# 4. Check LanceDB has ticket chunks
ls -la build/data/lancedb/19/

# 5. Test inference: create a NEW ticket (not #120)
curl -X POST http://localhost:5230/api/v1/tickets \
  -H "Content-Type: application/json" \
  -d '{"title":"Frontend build broken","description":"Dependency issue with React","status":"OPEN","priority":"MEDIUM","type":"TASK"}'

# 6. Check internal_notes has suggestions from ticket #120
sqlite3 build/data/memos_dev.db \
  "SELECT substr(internal_notes, 1, 500) FROM tickets ORDER BY id DESC LIMIT 1"

# 7. Edit ticket #120, verify version increments to 2
curl -X PUT http://localhost:5230/api/v1/tickets/120 \
  -H "Content-Type: application/json" \
  -d '{"title":"Bug #002: Repair Frontend Dependency Provenance (updated)"}'

sqlite3 build/data/memos_dev.db \
  "SELECT id, version, content_hash FROM agent_source_files WHERE file_type='ticket' AND tenant_id=19 ORDER BY version DESC"
```

---

## 4. Files Modified

| File | Action | Description |
|------|--------|-------------|
| `server/router/api/v1/agent/service.go` | MODIFY | Export `ReindexFileVersion`, add `IndexTicketContent` with content-hash dedup and inference chaining |
| `server/router/api/v1/ticket_service.go` | MODIFY | Add `getTicketComments` (returns error), hook indexing on create + update, remove independent inference goroutine |
| `server/router/api/v1/memo_service.go` | MODIFY | Hook re-indexing on comment create + edit with `UpdateMemo` guard conditions |

---

## 5. Validation

| Check | Command | Expected |
|-------|---------|----------|
| Compile | `task build:backend` | Clean |
| Ticket #120 indexed | `sqlite3 ... count(*) WHERE file_type='ticket' AND tenant_id=19` | ≥1 |
| Version correct | `sqlite3 ... SELECT version FROM agent_source_files ...` | version=1 (initial) |
| LanceDB chunks exist | `ls build/data/lancedb/19/` | Table files present |
| New ticket gets suggestions | Create ticket → check `internal_notes` | Contains "Suggested Resolution" |
| Edit ticket re-indexes | Update title → check version increments | version=2 |
| Same content no version bump | Edit with same content → check version stays | version=1 (Finding 1) |
| Comment re-indexes | Add comment → check content includes comments | Comment text in indexed content |
| Comment edit re-indexes | Edit comment → check version increments | version=N+1 |

---

## 6. Edge Cases

| Case | Behavior |
|------|----------|
| Ticket has no comments | Index title + description only |
| Ticket has 10+ comments | All comments included in indexed content |
| Embedding API unavailable | Reindex fails gracefully; retry on next edit |
| Same content re-indexed | Content hash matches → skip upsert, reindex existing version (Finding 1) |
| Comment on non-ticket memo | No-op (no ticket found via description link) |
| Multi-tenant isolation | Tenant ID from context; cross-tenant indexing blocked |
| Comment fetch fails | Log warning, index with title+description only (Finding 3) |

---

## 7. Future Work (Post-Prototype)

| Item | Description | Finding |
|------|-------------|---------|
| Comment churn debounce | 2-5s debounce window after last comment before re-indexing | #5 |
| Unit tests | `IndexTicketContent`, `getTicketComments`, hook wiring | #6 |
| Ticket deletion handling | Delete source file + vector chunks when ticket is deleted | #7 |
| Content injection sanitization | Sanitize `internal_notes` before writing (use `sanitizer.go`) | #8 |
| Bulk import mode | Batch indexing for historical ticket migration | — |
| Cross-tenant | Extend to multi-tenant ticket indexing with proper isolation | — |

---

## 8. Rollback

```sql
-- Delete ticket source files
DELETE FROM agent_source_files WHERE file_type='ticket' AND tenant_id=19;

-- Revert code changes
git checkout server/router/api/v1/agent/service.go
git checkout server/router/api/v1/ticket_service.go
git checkout server/router/api/v1/memo_service.go
```

Restart server. `InferResolutionForNewTicket` will continue to search (finding nothing), but no errors will occur.

---

## 9. Adversarial Review Prompt (Round 2)

```
You are an adversarial code reviewer. Review this REVISED implementation plan
for bugs/052 (Per-Ticket RAG Indexing Prototype, Round 2). The original plan
was reviewed and had 4 must-fix findings that have been addressed:

1. Version inflation → content-hash dedup added to IndexTicketContent
2. Inference race → inference now chained AFTER indexing completes
3. Silent failures → getTicketComments returns error, logged on failure
4. UpdateMemo hook → full pseudo-code with guard conditions added

Focus on:

1. CORRECTNESS of the content-hash dedup logic in IndexTicketContent.
   Is the check-then-act pattern safe given the async goroutine model?
   Could two goroutines race on the same ticket and both create versions?

2. Is the inference chaining (call InferResolutionForNewTicket AFTER
   ReindexFileVersion) correct? Does InferResolutionForNewTicket need
   its own context, or is the parent context sufficient?

3. Are the UpdateMemo hook guard conditions complete? What if the memo
   has multiple parent relations (e.g., is a comment on two different memos)?

4. Any remaining issues not addressed from the original review?

Provide severity ratings and recommended fixes.
```
