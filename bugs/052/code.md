# Code Documentation: Per-Ticket RAG Indexing Implementation

**Bug ID:** 052
**Date:** 2026-07-31
**Status:** Implemented — Compiles, Tests Pass

---

## 1. Overview

This implementation adds per-ticket RAG indexing to bchat, enabling the agent to
search previously indexed tickets when suggesting resolutions for new tickets.
The system now indexes ticket content (title + description + comments) into the
vector DB on every create, update, and comment event.

### Files Changed

| File | Lines Added | Lines Removed | Net |
|------|-------------|---------------|-----|
| `server/router/api/v1/agent/service.go` | +92 | -4 | +88 |
| `server/router/api/v1/ticket_service.go` | +80 | -3 | +77 |
| `server/router/api/v1/memo_service.go` | +92 | 0 | +92 |
| **Total** | **+264** | **-7** | **+257** |

---

## 2. Architecture

### Before

```
Ticket Created
  └── InferResolutionForNewTicket (goroutine)
        ├── Search vector DB: ContentTypes=["ticket"]  → EMPTY (nothing indexed)
        └── Search vector DB: ContentTypes=["bug_section"] → bug corpus results
```

### After

```
Ticket Created/Updated/Commented
  └── IndexTicketContent (goroutine)
        ├── Build content: "# Title\n\nDescription\n\n## Comments\n\n---\n\n..."
        ├── Content-hash dedup (per-ticket mutex)
        ├── UpsertAgentSourceFile (file_type="ticket")
        ├── ReindexFileVersion (chunk + embed + vector DB insert)
        └── InferResolutionForNewTicket (chained after indexing completes)
              ├── Search vector DB: ContentTypes=["ticket"]  → NOW RETURNS RESULTS
              └── Search vector DB: ContentTypes=["bug_section"] → bug corpus results
```

---

## 3. Changes by File

### 3.1 `server/router/api/v1/agent/service.go`

#### 3.1.1 Export `reindexFileVersion` → `ReindexFileVersion`

**Line 563:** Renamed unexported `reindexFileVersion` to exported `ReindexFileVersion`.

```go
// Before:
func (s *Service) reindexFileVersion(ctx context.Context, tenantID int32, ...) (int, error) {

// After:
func (s *Service) ReindexFileVersion(ctx context.Context, tenantID int32, ...) (int, error) {
```

**4 callers updated:**
- `service.go:665` — `ReindexAllContent` (kb)
- `service.go:672` — `ReindexAllContent` (policy)
- `service.go:768` — `ReindexTenantContent` (kb)
- `service.go:776` — `ReindexTenantContent` (policy)

#### 3.1.2 Add `ticketIndexMu` (per-ticket mutex)

**Line 5697:** Package-level `sync.Map` prevents TOCTOU races on content-hash dedup.

```go
var ticketIndexMu sync.Map

// Key: fmt.Sprintf("%d:%d", tenantID, ticketID)
// Value: *sync.Mutex
// LIMITATION: Entries never removed. Acceptable for prototype.
```

#### 3.1.3 Add `IndexTicketContent`

**Lines 5704–5776:** New public method on `*Service`. Core logic:

1. **Acquire per-ticket mutex** from `ticketIndexMu` — serializes concurrent
   operations on the same ticket.

2. **Build content blob** — concatenates title, description, and comments:
   ```
   # {title}

   {description}

   ## Comments

   ---

   {comment1}

   ---

   {comment2}
   ```

3. **Content-hash dedup** — calls `ContentHash(content)` (parser.go:86, SHA256 hex),
   then queries `GetAgentSourceFile` with `LatestOnly: true`. If the latest version's
   hash matches, skips upsert and calls `ReindexFileVersion` on the existing version.

4. **Upsert** — calls `UpsertAgentSourceFile` which auto-increments the version number.

5. **Reindex** — calls `ReindexFileVersion` which chunks, embeds, and inserts into
   LanceDB.

6. **Chain inference** — if `triggerInference` is true, calls `InferResolutionForNewTicket`
   after indexing completes (eliminates the race between search and indexing).

**Parameters:**
- `ctx context.Context` — request context
- `tenantID int32` — tenant scope
- `ticket *store.Ticket` — ticket to index
- `comments []*store.Memo` — comment memos (nil for create, populated for update)
- `triggerInference bool` — true for new tickets, false for edits (avoids double-inference)

**Returns:** `(int, error)` — number of chunks indexed, or error.

---

### 3.2 `server/router/api/v1/ticket_service.go`

#### 3.2.1 Add `fmt` import

**Line 4:** Added `"fmt"` to imports for `fmt.Errorf` in `getTicketComments`.

#### 3.2.2 Hook into ticket creation (main path)

**Lines 174–184:** Replaced the independent `InferResolutionForNewTicket` goroutine
with a single `IndexTicketContent` call that chains inference.

```go
// Before (old code):
if s.agentHandler != nil {
    go s.agentHandler.GetService().InferResolutionForNewTicket(context.WithoutCancel(ctx), ticket)
}

// After (new code):
if s.agentHandler != nil && ticket.TenantID != nil {
    go func() {
        ctx := context.WithoutCancel(ctx)
        _, err := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, nil, true)
        if err != nil {
            slog.Error("failed to index new ticket for RAG", "ticket_id", ticket.ID, "error", err)
        }
    }()
}
```

**Key changes:**
- Added `ticket.TenantID != nil` guard (prevents nil dereference)
- `triggerInference: true` — indexes first, then infers
- Error is logged instead of silently ignored

#### 3.2.3 Hook into ticket creation (dedup path)

**Lines 155–165:** Added indexing hook to the deduplication path (when a ticket
matches an existing one by description + creator).

```go
if s.agentHandler != nil && ticket.TenantID != nil {
    go func() {
        ctx := context.WithoutCancel(ctx)
        _, err := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, nil, false)
        if err != nil {
            slog.Error("failed to index deduplicated ticket for RAG", "ticket_id", ticket.ID, "error", err)
        }
    }()
}
```

**Note:** `triggerInference: false` — deduplicated tickets don't need self-inference.

#### 3.2.4 Hook into ticket update

**Lines 379–395:** After `s.Store.UpdateTicket`, triggers async re-index with comments.

```go
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
```

**Note:** `triggerInference: false` — ticket updates don't trigger self-inference.

#### 3.2.5 Add `getTicketComments`

**Lines 546–584:** New helper that fetches all comment memos for a ticket.

```go
func (s *APIV1Service) getTicketComments(ctx context.Context, ticket *store.Ticket) ([]*store.Memo, error)
```

**Logic:**
1. Check if `ticket.Description` starts with `/m/` (memo link format)
2. Extract memo UID, fetch parent memo via `GetMemo` with `TenantID` scoping
3. Query `ListMemoRelations` for `type=COMMENT` with `RelatedMemoID = parentMemo.ID`
4. Fetch each comment memo via `GetMemo`
5. Return slice of comment memos

**Error handling:** Returns `([]*store.Memo, error)` — callers log warnings on
error and fall back to indexing title+description only.

---

### 3.3 `server/router/api/v1/memo_service.go`

#### 3.3.1 Hook into comment creation (`CreateMemoComment`)

**Lines 654–686:** After `UpsertMemoRelation`, triggers async re-index of the
parent ticket.

```go
if s.agentHandler != nil {
    go func() {
        ctx := context.WithoutCancel(ctx)
        tid := GetTenantIDFromContext(ctx)
        if tid == nil {
            return // skip for unscoped requests
        }
        descriptionLink := "/m/" + relatedMemo.UID
        tickets, _ := s.Store.ListTickets(ctx, &store.FindTicket{
            Description: &descriptionLink,
            TenantID:    tid,
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
}
```

**Guards:**
- `s.agentHandler != nil` — no-op when RAG is disabled
- `tid == nil` — skip for unscoped requests (superusers)
- `ticket.TenantID == nil` — skip if ticket has no tenant

#### 3.3.2 Hook into comment edit (`UpdateMemo`)

**Lines 441–499:** After `s.Store.UpdateMemo`, checks if the updated memo is a
comment on a ticket and triggers re-index.

```go
if s.agentHandler != nil && update.Content != nil {
    if tenantID := GetTenantIDFromContext(ctx); tenantID != nil {
        // Check if this memo is a comment
        commentType := store.MemoRelationComment
        parentRelations, relErr := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
            MemoID: &memo.ID,
            Type:   &commentType,
        })
        if relErr == nil && len(parentRelations) > 0 {
            // Process all parent relations
            for _, rel := range parentRelations {
                // Find parent memo → find ticket → re-index
                // ... (see full code above)
                go func(t store.Ticket) {
                    // Capture by value to prevent loop variable reference bug
                    // ... re-index logic ...
                }(*ticketCopy)
            }
        }
    }
}
```

**Guards:**
- `s.agentHandler != nil` — no-op when RAG is disabled
- `update.Content != nil` — only re-index when content actually changed
- `tenantID != nil` — skip for unscoped requests
- `ticketCopy.TenantID == nil` — skip if ticket has no tenant

**Concurrency pattern:** Ticket is captured by value (`*ticketCopy`) in the
goroutine to prevent loop variable reference bugs.

---

## 4. Security Model

### Tenant Isolation

| Location | Guard |
|----------|-------|
| `IndexTicketContent` | Receives `tenantID` parameter, used in all store queries |
| `CreateTicket` hook | `ticket.TenantID != nil` check before calling `IndexTicketContent` |
| `UpdateTicket` hook | `ticket.TenantID != nil` check before calling `IndexTicketContent` |
| `CreateMemoComment` hook | `GetTenantIDFromContext(ctx) != nil` check, `TenantID` in `FindTicket` |
| `UpdateMemo` hook | `GetTenantIDFromContext(ctx) != nil` check, `TenantID` in `FindMemo` and `FindTicket` |
| `getTicketComments` | `TenantID: ticket.TenantID` in `FindMemo` lookup |

### Nil-Tenant Guard Pattern

Every hook that calls store methods with `TenantID` in the find struct first
checks that the tenant ID is non-nil. This prevents cross-tenant data writes
when superusers or unscoped JWTs are in play.

```go
tenantID := GetTenantIDFromContext(ctx)
if tenantID == nil {
    return // skip indexing for unscoped requests
}
```

---

## 5. Concurrency Model

### Per-Tenant Mutex (Existing)

`ReindexFileVersion` acquires a per-tenant mutex (`s.getTenantMutex(tenantID)`)
that serializes all reindex operations within a tenant. This prevents concurrent
LanceDB writes from interfering with each other.

### Per-Ticket Mutex (New)

`IndexTicketContent` acquires a per-ticket mutex from `ticketIndexMu` (sync.Map)
that serializes the content-hash check + upsert sequence. This prevents TOCTOU
races where two goroutines for the same ticket both observe an outdated hash
and both create new versions.

```
Thread A: GetAgentSourceFile → hash=abc
Thread B: GetAgentSourceFile → hash=abc (same, because A hasn't upserted yet)
Thread A: UpsertAgentSourceFile → version=2
Thread B: UpsertAgentSourceFile → version=3 (ORPHANED)

WITH PER-TICKET MUTEX:
Thread A: Lock(ticketMutex) → GetAgentSourceFile → hash=abc → Upsert → version=2 → Unlock
Thread B: Lock(ticketMutex) → GetAgentSourceFile → hash=abc → MATCH → Reindex v2 → Unlock
```

### Goroutine Pattern

All hooks launch background goroutines with `context.WithoutCancel(ctx)` to
ensure the indexing operation survives the HTTP request lifecycle. Errors are
logged but do not propagate to the caller (fire-and-forget).

---

## 6. Content Format

The indexed content blob follows this structure:

```markdown
# {ticket.Title}

{ticket.Description}

## Comments

---

{comment1.Content}

---

{comment2.Content}

---

{comment3.Content}
```

Each chunk produced by `ReindexFileVersion` will split this content using the
existing recursive heading-based chunker (`ChunkMarkdownContent`), which splits
by `##` headers, then `###`, then paragraphs, then sentences.

---

## 7. Version Management

### How Versions Work

| Event | Behavior |
|-------|----------|
| Ticket created | `UpsertAgentSourceFile` → version=1, `ReindexFileVersion` → LanceDB chunks |
| Ticket updated (content changed) | `UpsertAgentSourceFile` → version=2, `ReindexFileVersion` → LanceDB chunks |
| Ticket updated (same content) | Hash matches → skip upsert, `ReindexFileVersion` on existing version |
| Comment added | Full ticket content re-indexed → new version if content changed |
| Comment edited | Full ticket content re-indexed → new version if content changed |

### Retention

`ReindexFileVersion` enforces a retention policy: keep last 5 indexed versions
per `(tenant, audience, fileType)`. Older versions are deleted from LanceDB.

### Idempotency

The content-hash dedup ensures that no-op edits (saving without changes) do not
create new version rows in `agent_source_files` or new chunks in LanceDB.

---

## 8. Error Handling

| Error | Handling |
|-------|----------|
| `GetAgentSourceFile` fails | Log warning, proceed to upsert (creates new version) |
| `UpsertAgentSourceFile` fails | Return error to caller |
| `ReindexFileVersion` fails | Return error to caller |
| `getTicketComments` fails | Log warning, index with title+description only |
| `GetMemo` fails in UpdateMemo hook | Log debug, skip this parent relation |
| Embedding API unavailable | `ReindexFileVersion` fails gracefully; retry on next edit |
| TenantID is nil | Skip indexing entirely (no cross-tenant writes) |
| `agentHandler` is nil | Skip indexing entirely (RAG disabled) |

---

## 9. Validation

### Build

```bash
go build ./...
# Expected: clean
```

### Tests

```bash
go test ./server/router/api/v1/agent/... -count=1
go test ./server/router/api/v1/... -count=1
# Expected: PASS
```

### Manual Verification (Ticket #120)

```bash
# 1. Start server with RAG
task run:rag

# 2. Reindex tenant
curl -X POST http://localhost:5230/api/v1/agent/hackathon-demo/reindex

# 3. Verify source file created
sqlite3 build/data/memos_dev.db \
  "SELECT id, version, length(content), content_hash FROM agent_source_files WHERE file_type='ticket' AND tenant_id=19"

# 4. Create similar ticket, wait for inference
curl -X POST http://localhost:5230/api/v1/tickets \
  -H "Content-Type: application/json" \
  -d '{"title":"Frontend dependency provenance issue","description":"Repair frontend dependency provenance for build","status":"OPEN","priority":"MEDIUM","type":"TASK"}'

# 5. Wait and check
for i in $(seq 1 10); do
  NOTES=$(sqlite3 build/data/memos_dev.db \
    "SELECT substr(internal_notes, 1, 50) FROM tickets ORDER BY id DESC LIMIT 1")
  if [[ "$NOTES" == *"Suggested Resolution"* ]]; then
    echo "Inference complete"; break
  fi
  sleep 1
done

sqlite3 build/data/memos_dev.db \
  "SELECT substr(internal_notes, 1, 500) FROM tickets ORDER BY id DESC LIMIT 1"
```

---

## 10. Rollback

```sql
DELETE FROM agent_source_files WHERE file_type='ticket' AND tenant_id=19;
```

```bash
git checkout server/router/api/v1/agent/service.go
git checkout server/router/api/v1/ticket_service.go
git checkout server/router/api/v1/memo_service.go
```

Restart server.

---

## 11. Adversarial Code Review Prompt

```
You are an adversarial code reviewer performing a final code review of the
Per-Ticket RAG Indexing implementation (bugs/052). The code has been implemented
and compiles cleanly. Review the following three files:

1. server/router/api/v1/agent/service.go — IndexTicketContent, ReindexFileVersion
2. server/router/api/v1/ticket_service.go — getTicketComments, CreateTicket hooks, UpdateTicket hook
3. server/router/api/v1/memo_service.go — CreateMemoComment hook, UpdateMemo hook

Focus on:

1. CORRECTNESS: Does the content-hash dedup logic handle all edge cases?
   What happens if UpsertAgentSourceFile fails after the hash check passes?
   Is the per-ticket mutex correctly scoped?

2. SECURITY: Are all tenant isolation guards correct? Can a nil TenantID
   slip through any path? Are there injection vectors in the content blob?

3. CONCURRENCY: Is the goroutine capture-by-value pattern correct in the
   UpdateMemo hook? Could the `ticketCopy` dereference panic?

4. ERROR HANDLING: Are all error paths properly handled? Are there any
   silent failures that should be surfaced?

5. PERFORMANCE: Does the per-ticket mutex create contention? Could the
   sync.Map grow unboundedly in production?

6. EDGE CASES: What happens when a ticket has 100+ comments? What if
   the embedding API times out mid-reindex?

Provide severity ratings (Critical/High/Medium/Low) for each finding.
```
