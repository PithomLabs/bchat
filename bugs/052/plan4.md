# Plan: Per-Ticket RAG Indexing Prototype (Final)

**Bug ID:** 052
**Date:** 2026-07-31
**Status:** Draft — Awaiting Adversarial Review (Round 4)
**Revision:** Based on adversarial review of plan3.md

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

### All Review Findings Addressed

| Round | # | Finding | Severity | Resolution |
|-------|---|---------|----------|------------|
| R1 | 1 | Version inflation on reindex failure | CRITICAL | Content-hash dedup added |
| R1 | 2 | Inference races ahead of indexing | HIGH | Inference chained after indexing |
| R1 | 3 | Silent failures in `getTicketComments` | HIGH | Returns error, logged |
| R1 | 4 | `UpdateMemo` hook unspecified | HIGH | Full pseudo-code added |
| R2 | 1 | TOCTOU race in content-hash dedup | HIGH | Per-ticket mutex added |
| R2 | 2 | Missing tenant ID in find structs | HIGH | TenantID passed in all find structs |
| R2 | 3 | Stale `memo.ID` after UpdateMemo | nit | Documented that ID is immutable |
| R2 | 4 | Multiple COMMENT relations ambiguity | nit | Documented assumption |
| R2 | 5 | CreateTicket dedup path not hooked | nit | Hook added to dedup path |
| R2 | 6 | Validation timing not documented | MEDIUM | Wait/retry loop added |
| R2 | 7 | `GetAgentSourceFile` error swallowed | nit | Error logged |
| R2 | 8 | Inference threshold may block validation | nit | Documented workaround |
| R3 | 1 | Tenant-nil bypass leaks cross-tenant tickets | HIGH | nil guard added |
| R3 | 2 | `ticketIndexMu` unbounded memory growth | MEDIUM | Documented as prototype limitation |
| R3 | 3 | `GetMemo` error swallowed in UpdateMemo | MEDIUM | Debug log added |
| R3 | 4 | Goroutine captures loop variable by ref | MEDIUM | Capture by value |
| R3 | 5 | Dedup path validation incorrect | LOW | Wording fixed |
| R3 | 6 | Step 8/9 validate wrong content | LOW | **INVALID** — same title = same hash |
| R3 | 7 | `sha256Hash` undefined | NIT | Reference `ContentHash` from parser.go:86 |

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
| `ContentHash` | SHA256 hex hash function | `parser.go:86` |

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

### Key Codebase References (Verified)

| Reference | File | Line | Verified |
|-----------|------|------|----------|
| `FindTicket.TenantID` field | `store/ticket.go` | 45 | Yes |
| SQLite nil-TenantID = no filter | `store/db/sqlite/ticket.go` | 75-77 | Yes |
| `FindMemo.TenantID` field | `store/memo.go` | 83 | Yes |
| `FindMemoRelation.TenantID` field | `store/memo_relation.go` | 28 | Yes |
| `ContentHash` function | `server/router/api/v1/agent/parser.go` | 86 | Yes |
| `strPtr` helper | `server/router/api/v1/agent/service.go` | 5513 | Yes |
| `GetTenantIDFromContext` (gRPC) | `server/router/api/v1/acl.go` | 170 | Yes |
| `getTenantFromContext` (Echo) | `server/router/api/v1/tenant_context.go` | 14 | Yes |
| `APIV1Service.agentHandler` | `server/router/api/v1/v1.go` | 53 | Yes |
| `Handler.GetService()` | `server/router/api/v1/agent/handlers.go` | 51 | Yes |
| `sync` import in service.go | `server/router/api/v1/agent/service.go` | 19 | Yes |

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

New public method. Package is `agent` — can call `ContentHash` directly (same package).

```go
// ticketIndexMu prevents TOCTOU races on content-hash dedup per ticket.
// Key: fmt.Sprintf("%d:%d", tenantID, ticketID)
// LIMITATION: Entries are never removed. Acceptable for prototype.
// PRODUCTION TODO: Add background cleanup or move dedup into ReindexFileVersion.
var ticketIndexMu sync.Map

func (s *Service) IndexTicketContent(ctx context.Context, tenantID int32, ticket *store.Ticket, comments []*store.Memo, triggerInference bool) (int, error) {
    // Per-ticket mutex to prevent TOCTOU race on content-hash dedup
    muKey := fmt.Sprintf("%d:%d", tenantID, ticket.ID)
    muVal, _ := ticketIndexMu.LoadOrStore(muKey, &sync.Mutex{})
    mu := muVal.(*sync.Mutex)
    mu.Lock()
    defer mu.Unlock()

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
    contentHash := ContentHash(content) // parser.go:86

    // Check if latest version already has this content hash.
    // UpsertAgentSourceFile always increments version — skip if unchanged.
    existing, err := s.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{
        TenantID:   &tenantID,
        FileType:   strPtr("ticket"),
        LatestOnly: true,
    })
    if err != nil {
        slog.Warn("failed to check existing ticket source file, will upsert new version",
            "ticket_id", ticket.ID, "error", err)
    }
    if existing != nil && existing.ContentHash == contentHash {
        // Content unchanged — reindex existing version, don't create new row
        chunks, rerr := s.ReindexFileVersion(ctx, tenantID, "internal", "ticket", existing.Version, content, 0)
        if rerr != nil {
            return 0, rerr
        }
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

    // Chain: index first, then infer (eliminates race)
    if triggerInference {
        s.InferResolutionForNewTicket(ctx, ticket)
    }

    return chunks, nil
}
```

### Step 3: Create `getTicketComments` Helper

**File:** `server/router/api/v1/ticket_service.go`

Returns `([]*store.Memo, error)` — no silent failures.

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

**File:** `server/router/api/v1/ticket_service.go`

Remove independent `InferResolutionForNewTicket` goroutine. Replace with `IndexTicketContent` that chains inference.

**4a. Main creation path** (after line 162, after `s.Store.CreateTicket`):

```go
ticket, err = s.Store.CreateTicket(ctx, ticket)
if err != nil {
    slog.Error("CreateTicket store error", "error", err)
    return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create ticket").SetInternal(err)
}

// Index ticket for RAG, then trigger inference (chained, not parallel)
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

**4b. Deduplication path** (after line 153, after `slog.Info("CreateTicket deduplication success")`):

```go
slog.Info("CreateTicket deduplication success", "id", ticket.ID)

// Index deduplicated ticket for RAG
if s.agentHandler != nil && ticket.TenantID != nil {
    go func() {
        ctx := context.WithoutCancel(ctx)
        _, err := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, nil, false)
        if err != nil {
            slog.Error("failed to index deduplicated ticket for RAG", "ticket_id", ticket.ID, "error", err)
        }
    }()
}

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

**Tenant-nil guard**: `GetTenantIDFromContext(ctx)` must not be nil (gRPC context).

```go
// After UpsertMemoRelation:
go func() {
    ctx := context.WithoutCancel(ctx)
    tenantID := GetTenantIDFromContext(ctx)
    if tenantID == nil {
        return // skip indexing for unscoped requests (R3-1)
    }

    descriptionLink := "/m/" + relatedMemo.UID
    tickets, _ := s.Store.ListTickets(ctx, &store.FindTicket{
        Description: &descriptionLink,
        TenantID:    tenantID,
    })
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

**Tenant-nil guard**, **capture by value**, **debug log for GetMemo error**.

```go
if err = s.Store.UpdateMemo(ctx, update); err != nil {
    return nil, status.Errorf(codes.Internal, "failed to update memo")
}

// Re-index parent ticket if this memo is a comment and content changed
if s.agentHandler != nil && update.Content != nil {
    tenantID := GetTenantIDFromContext(ctx)
    if tenantID == nil {
        goto afterMemoUpdate // skip indexing for unscoped requests (R3-1)
    }

    // Check if this memo is a comment on something.
    // NOTE: memo.ID is used intentionally — it is immutable after creation.
    commentType := store.MemoRelationComment
    parentRelations, relErr := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
        MemoID: &memo.ID,
        Type:   &commentType,
    })
    if relErr == nil && len(parentRelations) > 0 {
        if len(parentRelations) > 1 {
            slog.Warn("memo has multiple COMMENT relations, indexing all parent tickets",
                "memo_id", memo.ID, "count", len(parentRelations))
        }
        for _, rel := range parentRelations {
            parentMemoID := rel.RelatedMemoID
            parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{
                ID:       &parentMemoID,
                TenantID: tenantID,
            })
            if err != nil {
                slog.Debug("failed to load parent memo for comment-edit reindex, skipping",
                    "memo_id", parentMemoID, "error", err)
                continue
            }
            if parentMemo == nil {
                continue
            }
            descriptionLink := "/m/" + parentMemo.UID
            tickets, _ := s.Store.ListTickets(ctx, &store.FindTicket{
                Description: &descriptionLink,
                TenantID:    tenantID,
            })
            if len(tickets) == 0 {
                continue
            }
            // Capture ticket by value to avoid loop variable reference issue (R3-4)
            ticketCopy := tickets[0]
            if ticketCopy.TenantID == nil {
                continue
            }
            go func(t store.Ticket) {
                ctx := context.WithoutCancel(ctx)
                comments, err := s.getTicketComments(ctx, &t)
                if err != nil {
                    slog.Warn("failed to fetch comments for ticket re-index after comment edit",
                        "ticket_id", t.ID, "error", err)
                }
                _, idxErr := s.agentHandler.GetService().IndexTicketContent(ctx, *t.TenantID, &t, comments, false)
                if idxErr != nil {
                    slog.Error("failed to re-index ticket after comment edit",
                        "ticket_id", t.ID, "error", idxErr)
                }
            }(ticketCopy)
        }
    }
}

afterMemoUpdate:
memo, err = s.Store.GetMemo(ctx, &store.FindMemo{ID: &memo.ID})
// ... rest of existing code
```

**Note on `goto`:** Go does not allow `goto` across variable declarations. Since `memo` is declared before this block and re-fetched after, we use a labeled block instead:

```go
// Re-index parent ticket if this memo is a comment and content changed
reindexBlock:
if s.agentHandler != nil && update.Content != nil {
    tenantID := GetTenantIDFromContext(ctx)
    if tenantID == nil {
        break reindexBlock
    }
    // ... rest of the reindex logic ...
}

memo, err = s.Store.GetMemo(ctx, &store.FindMemo{ID: &memo.ID})
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

# 5. Test inference: create a NEW ticket with similar wording to #120
#    Ticket #120 title: "Bug #002: Repair Frontend Dependency Provenance"
curl -X POST http://localhost:5230/api/v1/tickets \
  -H "Content-Type: application/json" \
  -d '{"title":"Frontend dependency provenance issue","description":"Repair frontend dependency provenance for build","status":"OPEN","priority":"MEDIUM","type":"TASK"}'

# 6. Wait for async indexing + inference to complete
for i in $(seq 1 10); do
  NOTES=$(sqlite3 build/data/memos_dev.db \
    "SELECT substr(internal_notes, 1, 50) FROM tickets ORDER BY id DESC LIMIT 1")
  if [[ "$NOTES" == *"Suggested Resolution"* ]]; then
    echo "Inference complete"
    break
  fi
  sleep 1
done

# 7. Check internal_notes has suggestions from ticket #120
sqlite3 build/data/memos_dev.db \
  "SELECT substr(internal_notes, 1, 500) FROM tickets ORDER BY id DESC LIMIT 1"

# 8. Edit ticket #120, verify version increments to 2
curl -X PUT http://localhost:5230/api/v1/tickets/120 \
  -H "Content-Type: application/json" \
  -d '{"title":"Bug #002: Repair Frontend Dependency Provenance (updated)"}'

sleep 3

sqlite3 build/data/memos_dev.db \
  "SELECT id, version, content_hash FROM agent_source_files WHERE file_type='ticket' AND tenant_id=19 ORDER BY version DESC"

# 9. Edit ticket #120 with SAME content as step 8, verify NO version increment
curl -X PUT http://localhost:5230/api/v1/tickets/120 \
  -H "Content-Type: application/json" \
  -d '{"title":"Bug #002: Repair Frontend Dependency Provenance (updated)"}'

sleep 3

sqlite3 build/data/memos_dev.db \
  "SELECT count(*) FROM agent_source_files WHERE file_type='ticket' AND tenant_id=19"
# Expected: still 2 rows (version=1 and version=2), not 3
```

---

## 4. Files Modified

| File | Action | Description |
|------|--------|-------------|
| `server/router/api/v1/agent/service.go` | MODIFY | Export `ReindexFileVersion`, add `IndexTicketContent` with per-ticket mutex, content-hash dedup, inference chaining |
| `server/router/api/v1/ticket_service.go` | MODIFY | Add `getTicketComments` (returns error), hook indexing on create + dedup path + update, tenant-nil guard on ticket.TenantID |
| `server/router/api/v1/memo_service.go` | MODIFY | Hook re-indexing on comment create + edit with tenant-nil guard, tenant ID isolation, capture-by-value goroutine, debug log for GetMemo error |

---

## 5. Validation

| Check | Command | Expected |
|-------|---------|----------|
| Compile | `task build:backend` | Clean |
| Ticket #120 indexed | `sqlite3 ... count(*) WHERE file_type='ticket' AND tenant_id=19` | ≥1 |
| Version correct | `sqlite3 ... SELECT version FROM agent_source_files ...` | version=1 (initial) |
| LanceDB chunks exist | `ls build/data/lancedb/19/` | Table files present |
| New ticket gets suggestions | Create ticket → wait → check `internal_notes` | Contains "Suggested Resolution" |
| Edit ticket re-indexes | Update title → check version increments | version=2 |
| Same content no version bump | Edit with same content → check row count stays | 2 rows (not 3) |
| Comment re-indexes | Add comment → check content includes comments | Comment text in indexed content |
| Comment edit re-indexes | Edit comment → check version increments | version=N+1 |
| Dedup path indexes | Create duplicate ticket → check source file exists | Existing version reused if hash matches |

---

## 6. Edge Cases

| Case | Behavior |
|------|----------|
| Ticket has no comments | Index title + description only |
| Ticket has 10+ comments | All comments included in indexed content |
| Embedding API unavailable | Reindex fails gracefully; retry on next edit |
| Same content re-indexed | Content hash matches → skip upsert, reindex existing version |
| Comment on non-ticket memo | No-op (no ticket found via description link) |
| Multi-tenant isolation | TenantID passed in all find structs; nil guard skips unscoped requests |
| Comment fetch fails | Log warning, index with title+description only |
| TOCTOU race on dedup | Per-ticket mutex serializes check+upsert |
| Multiple COMMENT relations | Log warning, index all parent tickets |
| CreateTicket dedup path | Indexing triggered for deduplicated tickets |
| Validation timing | Wait/retry loop checks for inference completion |
| TenantID nil (superuser) | Skip indexing — no cross-tenant data write |
| Goroutine loop variable | Capture ticket by value to prevent reference bugs |

---

## 7. Future Work (Post-Prototype)

| Item | Description | Finding |
|------|-------------|---------|
| sync.Map cleanup | Background goroutine removes entries older than N minutes | R3-2 |
| Comment churn debounce | 2-5s debounce window after last comment before re-indexing | R1-5 |
| Unit tests | `IndexTicketContent`, `getTicketComments`, hook wiring | R1-6 |
| Ticket deletion handling | Delete source file + vector chunks when ticket is deleted | R1-7 |
| Content injection sanitization | Sanitize `internal_notes` before writing (use `sanitizer.go`) | R1-8 |
| Bulk import mode | Batch indexing for historical ticket migration | — |

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

## 9. Adversarial Review Prompt (Round 4)

```
You are an adversarial code reviewer. Review this REVISED implementation plan
for bugs/052 (Per-Ticket RAG Indexing Prototype, Round 4). The plan has been
through 3 prior reviews with all must-fix findings addressed:

Round 1: Version inflation, inference race, silent failures, UpdateMemo hook
Round 2: TOCTOU race, missing tenant ID, stale memo.ID, multiple COMMENT
         relations, dedup path, validation timing, GetAgentSourceFile error,
         inference threshold
Round 3: Tenant-nil bypass, sync.Map growth, GetMemo error swallow, goroutine
         loop variable capture, dedup validation wording, sha256Hash reference

All findings are now resolved. Focus on:

1. Does the code in Steps 4-7 compile against the actual codebase types?
   Verify: FindTicket has TenantID, FindMemo has TenantID, FindMemoRelation
   has TenantID, MemoRelationComment is a valid constant.

2. Is the labeled block (reindexBlock: break reindexBlock) correct Go
   syntax for skipping the reindex logic when tenantID is nil?

3. Is the goroutine capture-by-value pattern correct? Does passing
   store.Ticket by value (not pointer) cause issues with getTicketComments
   which expects *store.Ticket?

4. Any remaining issues?
```
