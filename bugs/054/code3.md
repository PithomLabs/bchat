# Code Documentation: Bug 054 Plan 3 — 054b–054d + Hackathon AI Suggestion Surfacing

**Bug IDs:** 054b, 054c, 054d
**Date:** 2026-07-31
**Status:** Implemented
**Files Modified:**
- `server/router/api/v1/memo_service.go`
- `server/router/api/v1/ticket_service.go`
- `server/router/api/v1/agent/service.go`
- `web/src/pages/Tickets.tsx`
- `web/src/locales/en.json`

---

## 1. Implementation Summary

| Step | Fix | File | Status |
|------|-----|------|--------|
| 1 | 054b — propagate `memo.TenantID` in `handleAutoTicketCreation` | `memo_service.go` | Done |
| 2 | 054c backend — `TenantID *int32` in `CreateTicketRequest`, superuser-only override | `ticket_service.go` | Done |
| 3 | 054c frontend — include `tenantId` in create-ticket payload for admin dropdown | `Tickets.tsx` | Done |
| 4 | 054d — tenant dropdown in ticket modal + translation | `Tickets.tsx`, `en.json` | Done |
| 5 | Hackathon — `InferResolutionForNewTicket` returns `string` notes | `agent/service.go` | Done |
| 6 | Hackathon — `IndexTicketContent` returns `(chunks, inferred, error)` + updated 5 callers | `agent/service.go`, `ticket_service.go`, `memo_service.go` | Done |
| 7 | Hackathon — post-index goroutine creates system resolution comment | `ticket_service.go` | Done |
| 8 | Hackathon — `createSystemResolutionComment` helper: system memo + comment relation, no re-index | `ticket_service.go` | Done |
| 9 | Hackathon — frontend renders system suggestion comment with amber styling | `Tickets.tsx` | Done |

Build, `go vet`, and `go test ./server/router/api/v1/...` all pass.

---

## 2. Fix 054b — Auto-Ticket Tenant Propagation

**File:** `server/router/api/v1/memo_service.go`
**Lines:** 1163–1173

**Problem:** `handleAutoTicketCreation` constructed tickets without `TenantID`, inserting `NULL` for regular users' auto-created tickets. This skipped both `IndexTicketContent` and `InferResolutionForNewTicket` because both guard on `ticket.TenantID != nil`.

**Fix:** Added `TenantID: memo.TenantID` to the ticket constructor.

```go
ticket := &store.Ticket{
    Title:       title,
    Description: "/m/" + memo.UID,
    Status:      store.TicketStatusOpen,
    Priority:    priority,
    Type:        ticketType,
    Tags:        tags,
    CreatorID:   user.ID,
    CreatedTs:   time.Now().Unix(),
    UpdatedTs:   time.Now().Unix(),
    TenantID:    memo.TenantID,
}
```

**Scope:** Only affects the `!isSuperUser(user)` path at `memo_service.go:117`. HOST/ADMIN never enter this path.

**Behavior change:** Previously auto-created tickets had `tenant_id = NULL`. After this fix, `tenant_id` matches the memo's tenant, enabling RAG indexing and inference for regular users' auto-created tickets.

---

## 3. Fix 054c — Superuser Tenant Override

### 3.1 Backend

**File:** `server/router/api/v1/ticket_service.go`
**Lines:** 32–40, 83–103

**Problem:** `CreateTicketRequest` had no `TenantID` field. Superusers creating tickets via REST could only create tickets in their JWT tenant. There was no way to create a ticket on behalf of another tenant.

**Request struct change:**

```go
type CreateTicketRequest struct {
    Title       string   `json:"title"`
    Description string   `json:"description"`
    Status      string   `json:"status"`
    Priority    string   `json:"priority"`
    Type        string   `json:"type"`
    Tags        []string `json:"tags"`
    AssigneeID  *int32   `json:"assigneeId"`
    TenantID    *int32   `json:"tenantId"` // superuser-only tenant override
}
```

**Handler change:**

```go
ticket := &store.Ticket{
    Title:       request.Title,
    Description: request.Description,
    Status:      store.TicketStatus(request.Status),
    Priority:    store.TicketPriority(request.Priority),
    Type:        request.Type,
    Tags:        request.Tags,
    CreatorID:   userID,
    AssigneeID:  request.AssigneeID,
    CreatedTs:   time.Now().Unix(),
    UpdatedTs:   time.Now().Unix(),
    TenantID:    getTenantFromContext(c),
}

if request.TenantID != nil {
    if !isSuperUser(user) {
        return echo.NewHTTPError(http.StatusBadRequest, "tenantId is only available to admins")
    }
    tenant, err := s.Store.GetAgentTenant(ctx, &store.FindAgentTenant{ID: request.TenantID})
    if err != nil || tenant == nil {
        return echo.NewHTTPError(http.StatusBadRequest, "invalid tenantId")
    }
    ticket.TenantID = request.TenantID
}
```

**Authorization:** Only `HOST` or unscoped `ADMIN` may set `tenantId`. Regular users get 400 if they send the field. This prevents cross-tenant ticket creation.

**Behavior change:** Superusers can now create tickets in any tenant via REST by sending `tenantId` in the request body.

### 3.2 Frontend

**File:** `web/src/pages/Tickets.tsx`
**Lines:** 110–117, 216–237, 316–325

**State initialization:**

```typescript
// Tenant selector state (admin only)
const [availableTenants, setAvailableTenants] = useState<AgentTenant[]>([]);
const [selectedTenantId, setSelectedTenantId] = useState<number | null>(() => {
    const stored = localStorage.getItem("tenant_id");
    return stored ? Number(stored) : null;
});
```

**Payload construction:**

```typescript
const payload: any = {
    title,
    description: memoUrl,
    status,
    priority,
    type,
    assigneeId: assigneeId || undefined
};

if (isAdmin && selectedTenantId) {
    payload.tenantId = selectedTenantId;
}
```

**Reset form includes tenant reset:**

```typescript
const resetForm = () => {
    setTitle("");
    setStatus("OPEN");
    setPriority("MEDIUM");
    setType("TASK");
    setAssigneeId(null);
    setDescription("");
    setRelatedMemos([]);
    setIsCreatingDescription(false);
    const stored = localStorage.getItem("tenant_id");
    setSelectedTenantId(stored ? Number(stored) : null);
};
```

**Translation:**

```json
"agent-admin": {
  "tenantLabel": "Company"
}
```

---

## 4. Fix 054d — UI Tenant Dropdown

**File:** `web/src/pages/Tickets.tsx`
**Lines:** 110–117, 498–516

**Visibility rule:**

```typescript
{isAdmin && availableTenants.length > 0 && (
    <div>
        <label className="block text-sm font-medium mb-1">Company</label>
        <Select
            value={selectedTenantId || null}
            onChange={(_, val) => setSelectedTenantId(val)}
            placeholder="Select tenant"
        >
            {availableTenants.map((t) => (
                <Option key={t.id} value={t.id}>{t.name}</Option>
            ))}
        </Select>
    </div>
)}
```

**Data source:** `/api/v1/auth/tenants` (POST with empty body), fetched on component mount when `isAdmin` is true.

**Behavior:** Visible only to admin users. Initialized from `localStorage.tenant_id` set during sign-in. Changes apply only to the current ticket creation; they do not mutate the JWT context.

---

## 5. Hackathon Deliverable — AI Suggestion Surfacing

### 5.1 `InferResolutionForNewTicket` Returns Notes

**File:** `server/router/api/v1/agent/service.go`
**Lines:** 5597–5704

**Before:**

```go
func (s *Service) InferResolutionForNewTicket(ctx context.Context, ticket *store.Ticket) {
    // ...
    if !hasResults {
        slog.Info("no similar tickets or bug history found for inference", "ticket_id", ticket.ID)
        return
    }
    // Update ticket's internal_notes
    // ...
}
```

**After:**

```go
func (s *Service) InferResolutionForNewTicket(ctx context.Context, ticket *store.Ticket) string {
    // ...
    if !hasResults {
        slog.Info("no similar tickets or bug history found for inference", "ticket_id", ticket.ID)
        return ""
    }
    // Update ticket's internal_notes
    // ...
    return suggestedNotes
}
```

All early returns now return `""` instead of bare `return`. The notes string is returned after successful `UpdateTicket`.

### 5.2 `IndexTicketContent` Returns Inferred Flag

**File:** `server/router/api/v1/agent/service.go`
**Lines:** 5718–5786

**Before:** `(int, error)`
**After:** `(chunks int, inferred bool, err error)`

The function now captures whether inference produced notes:

```go
if triggerInference {
    inferred = s.InferResolutionForNewTicket(ctx, ticket) != ""
}
return chunks, inferred, nil
```

### 5.3 Updated All 5 `IndexTicketContent` Callers

| File | Line | Before | After |
|------|------|--------|-------|
| `ticket_service.go` | 160 | `_, err := ...` | `_, _, err := ...` |
| `ticket_service.go` | 181 | `_, err := ...` | `_, _, err := ...` |
| `ticket_service.go` | 388 | `_, idxErr := ...` | `_, _, idxErr := ...` |
| `memo_service.go` | 498 | `_, idxErr := ...` | `_, _, idxErr := ...` |
| `memo_service.go` | 694 | `_, idxErr := ...` | `_, _, idxErr := ...` |

### 5.4 Post-Index Goroutine Creates System Comment

**File:** `server/router/api/v1/ticket_service.go`
**Lines:** 189–226

After `IndexTicketContent` completes with `triggerInference: true`, the goroutine fetches the updated ticket and creates a system comment if `internal_notes` is populated:

```go
if s.agentHandler != nil && ticket.TenantID != nil {
    go func() {
        ctx := context.WithoutCancel(ctx)
        _, _, err := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, nil, true)
        if err != nil {
            slog.Error("failed to index new ticket for RAG", "ticket_id", ticket.ID, "error", err)
            return
        }

        updated, fetchErr := s.Store.GetTicket(ctx, &store.FindTicket{ID: &ticket.ID})
        if fetchErr != nil || updated == nil || updated.InternalNotes == "" {
            return
        }

        suggestion := updated.InternalNotes
        if commentErr := s.createSystemResolutionComment(ctx, *ticket.TenantID, ticket, suggestion); commentErr != nil {
            slog.Error("failed to create system resolution comment", "ticket_id", ticket.ID, "error", commentErr)
        }
    }()
}
```

**Design decision:** Intentionally no re-index after system comment creation. The comment is the inference OUTPUT, not new input. Re-indexing would create an unnecessary version bump and waste a LanceDB write. When/if the user adds a real comment, `CreateMemoComment` will re-index at that time.

### 5.5 `createSystemResolutionComment` Helper

**File:** `server/router/api/v1/ticket_service.go`
**Lines:** 611–648

```go
func (s *APIV1Service) createSystemResolutionComment(ctx context.Context, tenantID int32, ticket *store.Ticket, suggestion string) error {
    if !strings.HasPrefix(ticket.Description, "/m/") {
        return nil
    }
    memoUID := strings.TrimPrefix(ticket.Description, "/m/")

    parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{
        UID:      &memoUID,
        TenantID: &tenantID,
    })
    if err != nil || parentMemo == nil {
        return fmt.Errorf("parent memo not found: %w", err)
    }

    comment, err := s.Store.CreateMemo(ctx, &store.Memo{
        RowStatus:  store.Normal,
        CreatorID:  store.SystemBotID,
        Content:    "## AI Suggestion\n\n" + suggestion,
        Visibility: store.Public,
        TenantID:   &tenantID,
    })
    if err != nil {
        return fmt.Errorf("failed to create system comment: %w", err)
    }

    _, err = s.Store.UpsertMemoRelation(ctx, &store.MemoRelation{
        MemoID:        comment.ID,
        RelatedMemoID: parentMemo.ID,
        Type:          store.MemoRelationComment,
        TenantID:      &tenantID,
    })
    if err != nil {
        return fmt.Errorf("failed to link system comment: %w", err)
    }

    return nil
}
```

**Key design choices:**
- Uses `store.SystemBotID` (0) as creator, making it identifiable as a system comment
- Content prefixed with `## AI Suggestion\n\n` for frontend detection
- Uses store-level `CreateMemo` + `UpsertMemoRelation` instead of gRPC `CreateMemoComment` because the goroutine is fire-and-forget and creating a full gRPC request would be overkill
- No dedup check — relies on `triggerInference: true` being used only on new ticket creation

### 5.6 Frontend — System Suggestion Rendering

**File:** `web/src/pages/Tickets.tsx`
**Lines:** 783–815

`CommentItem` detects system suggestions by the `## AI Suggestion` prefix:

```typescript
const isSystemSuggestion = liveMemo.content.startsWith("## AI Suggestion");

return (
    <div className={cx(
        "border rounded-lg p-3",
        isSystemSuggestion
            ? "bg-amber-50 border-amber-200 dark:bg-amber-900/20 dark:border-amber-900/30"
            : "bg-gray-50 dark:bg-zinc-900/50"
    )}>
        <div className="flex justify-between items-center mb-1">
            <div className="flex items-center gap-2">
                {creator && <UserAvatar avatarUrl={creator.avatarUrl as any} className="w-5 h-5" />}
                {isSystemSuggestion && (
                    <span className="font-semibold text-sm text-amber-800 dark:text-amber-300 flex items-center gap-1">
                        <span>🤖</span>
                        <span>AI Suggestion</span>
                    </span>
                )}
                {!isSystemSuggestion && (
                    <span className="font-semibold text-sm">{creator ? (creator.nickname || creator.username) : liveMemo.creator}</span>
                )}
            </div>
            <span className="text-xs text-gray-500">{liveMemo.createTime ? new Date(liveMemo.createTime).toLocaleString() : ""}</span>
        </div>
        <MemoView memo={liveMemo} compact showCreator={false} />
    </div>
);
```

**Behavior:** System suggestions render with amber background/border, 🤖 AI Suggestion header, and no creator avatar/name. Regular comments are unchanged.

---

## 6. Data Flow

### 6.1 New Ticket Creation with Inference

```
User creates ticket via REST or UI
  -> CreateTicket handler
  -> ticket.TenantID set from JWT or superuser override
  -> CreateTicket in store
  -> goroutine:
       IndexTicketContent(ctx, tenantID, ticket, nil, true)
         -> content-hash dedup
         -> UpsertAgentSourceFile + ReindexFileVersion
         -> InferResolutionForNewTicket(ctx, ticket)
              -> vectorDB.Search(tickets, MinScore=0.7)
              -> vectorDB.Search(bug_sections, MinScore=0.5)
              -> if hasResults: UpdateTicket internal_notes, return notes
              -> if no results: return ""
         -> return (chunks, inferred, nil)
       <- if inferred:
          fetch updated ticket
          if internal_notes != "":
              createSystemResolutionComment(ctx, tenantID, ticket, notes)
                -> CreateMemo(SystemBotID, "## AI Suggestion\n\n" + notes)
                -> UpsertMemoRelation(COMMENT)
                -> NO re-index
```

### 6.2 Frontend Polling

```
Tickets.tsx loads
  -> relatedMemos = []
  -> open ticket detail
  -> loadRelatedMemos(ticket)
    -> memoStore.getOrFetchMemoByName(parentMemoName)
    -> ListMemoRelations(COMMENT)
    -> fetch each comment memo
  -> CommentItem for each memo
    -> if content starts with "## AI Suggestion": amber styling
```

The system comment appears in `relatedMemos` on the next poll/fetch cycle after the goroutine commits it to SQLite. No websocket push is required.

---

## 7. Behavioral Contracts

| Scenario | Behavior |
|----------|----------|
| Regular user creates memo with `#staff` | Auto-ticket gets `tenant_id = memo.TenantID`; RAG + inference run |
| HOST creates ticket via REST without `tenantId` | Ticket uses JWT tenant (054a fix) |
| HOST creates ticket via REST with `tenantId` | Ticket uses specified tenant |
| Regular user sends `tenantId` | HTTP 400 |
| HOST opens ticket modal | Tenant dropdown visible |
| Scoped ADMIN opens ticket modal | Dropdown visible if `allowedTenantIds.length > 1` |
| Regular user opens ticket modal | Dropdown hidden |
| Inference finds matches | `internal_notes` populated + 🤖 system comment created |
| Inference finds no matches | No system comment. `internal_notes` stays empty. Log emitted. |
| User adds real comment after system comment | `CreateMemoComment` re-indexes ticket (existing behavior) |
| Reindex triggered manually | `IndexTicketContent` dedup prevents duplicate source file version. System comment unaffected. |

---

## 8. Risk Register

| Risk | Impact | Mitigation |
|------|--------|-----------|
| System comment duplicates on rapid reindex | Low | Only triggered on `triggerInference: true` path. Future paths should add dedup if needed. |
| Superuser creates ticket in wrong tenant | Low | Explicit `tenantId` override; user must intentionally select target tenant. |
| Inference writes empty notes | None | `InferResolutionForNewTicket` returns `""` when `hasResults` is false; no comment created. |
| Frontend `fetchTenants` fails | Low | Catch logged; dropdown stays hidden; ticket creation still works with JWT tenant. |
| System comment not immediately visible | Low | Comment is created in goroutine after indexing. Frontend polls on ticket open; delay is sub-second. |
| `localStorage.tenant_id` stale after tenant switch | Low | `resetForm` re-reads from localStorage. Switching tenant in UI updates dropdown only, not JWT. |

---

## 9. Testing Evidence

| Check | Result |
|-------|--------|
| `go build ./server/router/api/v1/` | Pass |
| `go build ./...` | Pass |
| `go test ./server/router/api/v1/...` | Pass (2.984s + 8.868s) |
| `go vet ./server/router/api/v1/` | Pass |

---

## 10. Adversarial Code Review Prompt

Please conduct an adversarial code review of the Bug 054 Plan 3 implementation documented above. Focus on:

1. **Correctness:** Do the code snippets match the actual diff? Are there logic errors, race conditions, or missing edge cases in the implementation?
2. **Security:** Does the superuser tenant override properly prevent cross-tenant data leakage? Is the system comment creation secure (no injection, proper tenant scoping)?
3. **Performance:** Does the post-index goroutine + ticket fetch + comment creation introduce any deadlocks, livelocks, or resource leaks? Is the `ticketIndexMu` usage still correct after the signature change?
4. **API surface:** Does changing `InferResolutionForNewTicket` from `void` to `string` and `IndexTicketContent` from `(int, error)` to `(int, bool, error)` break any external callers or plugins?
5. **Frontend:** Does the tenant dropdown logic correctly handle all role combinations (HOST, scoped ADMIN, regular USER)? Is the `localStorage.tenant_id` fallback safe when the value is missing or malformed?
6. **Edge cases:** What happens when `ticket.Description` does not start with `/m/`? When `CreateMemo` fails after the parent memo is found? When `UpsertMemoRelation` fails after the comment is created? Are these handled gracefully?
7. **Regression:** Does the `IndexTicketContent` signature change break the dedup logic? Do the 5 updated callers correctly ignore the new return value?

Return a verdict of APPROVED, APPROVED WITH NITS, or REJECT WITH REWORK, with specific line-number references for any issues found.
