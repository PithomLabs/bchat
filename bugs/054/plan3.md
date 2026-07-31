# Bug 054 Plan 3: 054b–054d — Auto-Ticket TenantID, Superuser Tenant Override, UI Dropdown, and Hackathon AI Suggestion Surfacing

**Bug IDs:** 054b, 054c, 054d
**Created:** 2026-07-31
**Status:** DRAFT — Awaiting review
**Supersedes:** plan.md (rejected), plan2.md (054a only)
**Scope:** Auto-ticket tenant propagation, superuser REST tenant override, UI tenant dropdown, and hackathon-deliverable AI suggestion surfacing as system comment + internal_notes.

---

## 1. Problem Summary

| ID | Symptom | Root Cause |
|----|---------|------------|
| **054a** | HOST manual ticket has `tenant_id = NULL` | Fixed in plan2 / `auth_service.go` |
| **054b** | Auto-created ticket has `tenant_id = NULL` | `handleAutoTicketCreation` omits `TenantID` |
| **054c** | Superuser cannot create ticket for another tenant via REST | `CreateTicketRequest` lacks `tenantId` |
| **054d** | No tenant selector in web UI ticket modal | `Tickets.tsx` has no tenant dropdown |

**Hackathon deliverable:** When RAG inference finds similar tickets/bug history, surface the suggestion immediately in the UI as a system-authored comment and populate `internal_notes`. Currently `InferResolutionForNewTicket` writes `internal_notes` but returns `void`, so callers cannot react. Across 51 non-NULL-`tenant_id` tickets, zero have `internal_notes` populated because inference finds nothing useful at current thresholds, and even when it does, nothing creates a visible UI artifact.

---

## 2. Fix 054b — Propagate `memo.TenantID` in `handleAutoTicketCreation`

**File:** `server/router/api/v1/memo_service.go`
**Lines:** 1163–1173

**Before:**
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
}
```

**After:**
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

**Scope:** `handleAutoTicketCreation` is only called when `!isSuperUser(user)` (`memo_service.go:117`). HOST/ADMIN do not use this path. This fix is strictly for regular users whose memos auto-create tickets.

**Behavior change:** Previously the ticket’s `tenant_id` was `NULL`, which skipped `IndexTicketContent` and `InferResolutionForNewTicket`. After this fix, the ticket participates in per-tenant RAG indexing and inference like any manually created ticket.

---

## 3. Fix 054c — Add `tenantId` to `CreateTicketRequest` for Superusers

### 3.1 Backend — request struct

**File:** `server/router/api/v1/ticket_service.go`
**Lines:** 32–40

**After:**
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

### 3.2 Backend — handler

**File:** `server/router/api/v1/ticket_service.go`
**Lines:** 83–95

After the existing bind block:

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
    TenantID:    tenantID,
}

// Superuser tenant override
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

### 3.3 Frontend — conditional send

The existing web UI ticket modal already has `isAdmin` state. Send `tenantId` only when `isAdmin && selectedTenantId !== currentTenantId`.

---

## 4. Fix 054d — Tenant Dropdown in Ticket Creation Modal

**File:** `web/src/pages/Tickets.tsx`
**Lines:** ~498–673

Show the dropdown only when the user can switch tenants:

```typescript
const showTenantDropdown =
  userRole === "HOST" ||
  (userRole === "ADMIN" && allowedTenantIds.length > 1);
```

Place it above `Title` when visible:

```tsx
{showTenantDropdown && (
  <div>
    <label className="block text-sm font-medium mb-1">Company</label>
    <Select value={selectedTenantId} onChange={(_, val) => setSelectedTenantId(val)}>
      {availableTenants.map(t => (
        <Option key={t.id} value={t.id}>{t.name}</Option>
      ))}
    </Select>
  </div>
)}
```

**Data source:** reuse existing tenant list API. Fetch on component mount.

**Translation:** add `"agent-admin.tickets.tenantLabel": "Company"` to `web/src/locales/en.json`.

---

## 5. Hackathon Deliverable — AI Suggestion Surfacing (Option D)

### 5.1 Backend — make inference result available

**A.** `InferResolutionForNewTicket` returns the generated notes.

**File:** `server/router/api/v1/agent/service.go`
**Lines:** 5597–5704

Change signature:
```go
func (s *Service) InferResolutionForNewTicket(ctx context.Context, ticket *store.Ticket) string {
```

At the end, instead of only logging, return the notes:
```go
return strings.Join(notes, "\n")
```

When `hasResults` is false, log the existing message and return `""`.

**B.** Create system comment after indexing + inference in ticket creation.

**File:** `server/router/api/v1/ticket_service.go`
**Lines:** 177–186

Current block:
```go
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

After:
```go
if s.agentHandler != nil && ticket.TenantID != nil {
    go func() {
        ctx := context.WithoutCancel(ctx)
        _, err := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, nil, true)
        if err != nil {
            slog.Error("failed to index new ticket for RAG", "ticket_id", ticket.ID, "error", err)
            return
        }

        // Fetch updated ticket to read populated internal_notes
        updated, fetchErr := s.Store.GetTicket(ctx, &store.FindTicket{ID: &ticket.ID})
        if fetchErr != nil || updated == nil || updated.InternalNotes == "" {
            return
        }

        // Create system-authored comment with the inferred resolution
        suggestion := updated.InternalNotes
        if err := s.createSystemResolutionComment(ctx, *ticket.TenantID, ticket, suggestion); err != nil {
            slog.Error("failed to create system resolution comment", "ticket_id", ticket.ID, "error", err)
        }
    }()
}
```

**C.** Implement `createSystemResolutionComment` helper.

**File:** `server/router/api/v1/ticket_service.go`
**Location:** new helper near `getTicketComments`

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
        Visibility: store.VisibilityPublic,
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

    // NOTE: intentionally no re-index here. The system comment is the inference
    // OUTPUT, not new input. Re-indexing would create an unnecessary version bump
    // and waste a LanceDB write. When/if the user adds a real comment,
    // CreateMemoComment will re-index at that time.
    return nil
}
```

**D.** `IndexTicketContent` returns whether inference produced notes.

**File:** `server/router/api/v1/agent/service.go`
**Lines:** 5716–5784

At the end of the function, after calling `InferResolutionForNewTicket`, also return a boolean indicating whether notes were produced. Change signature to:

```go
func (s *Service) IndexTicketContent(ctx context.Context, tenantID int32, ticket *store.Ticket, comments []*store.Memo, triggerInference bool) (chunks int, inferred bool, err error)
```

Update the two call sites inside `IndexTicketContent` itself (lines 5756 and 5780) to capture `inferred`. All 5 external callers must also be updated to ignore the new return value: `_, _, err := ...IndexTicketContent(...)`.

| File | Line | Current | Required |
|------|------|---------|----------|
| `ticket_service.go` | 160 | `_, err := ...IndexTicketContent(...)` | `_, _, err := ...IndexTicketContent(...)` |
| `ticket_service.go` | 181 | `_, err := ...IndexTicketContent(...)` | `_, _, err := ...IndexTicketContent(...)` |
| `ticket_service.go` | 388 | `_, idxErr := ...IndexTicketContent(...)` | `_, _, idxErr := ...IndexTicketContent(...)` |
| `memo_service.go` | 498 | `_, idxErr := ...IndexTicketContent(...)` | `_, _, idxErr := ...IndexTicketContent(...)` |
| `memo_service.go` | 694 | `_, idxErr := ...IndexTicketContent(...)` | `_, _, idxErr := ...IndexTicketContent(...)` |

### 5.2 Frontend — render system suggestion comment

`CommentItem` in `Tickets.tsx` already iterates all memo comments. System-authored comments are identified by the `## AI Suggestion\n\n` prefix.

```tsx
const isSystemSuggestion = memo.content.startsWith("## AI Suggestion");

<div className={cx(
  "p-3 rounded-lg text-sm",
  isSystemSuggestion
    ? "bg-amber-50 border border-amber-200 dark:bg-amber-900/20 dark:border-amber-900/30"
    : "bg-white border"
)}>
  {isSystemSuggestion && (
    <div className="flex items-center gap-1 mb-2 font-semibold text-amber-800 dark:text-amber-300">
      <span>🤖</span>
      <span>AI Suggestion</span>
    </div>
  )}
  <div className="whitespace-pre-wrap">{renderedContent}</div>
</div>
```

---

## 6. Implementation Order

| Step | Fix | File | Description |
|------|-----|------|-------------|
| 1 | 054b | `memo_service.go` | Add `TenantID: memo.TenantID` |
| 2 | 054c backend | `ticket_service.go` | Add `TenantID *int32` to `CreateTicketRequest`, superuser-only override |
| 3 | 054c frontend | `Tickets.tsx` | Include `tenantId` in create-ticket payload for admin dropdown |
| 4 | 054d | `Tickets.tsx`, `en.json` | Add tenant dropdown for HOST / multi-tenant ADMIN |
| 5 | Hackathon | `service.go` | `InferResolutionForNewTicket` returns `string` notes |
| 6 | Hackathon | `service.go` | `IndexTicketContent` returns `(chunks, inferred, error)` + update all 5 callers |
| 7 | Hackathon | `ticket_service.go` | After indexing goroutine completes, fetch ticket and call `createSystemResolutionComment` |
| 8 | Hackathon | `ticket_service.go` | New `createSystemResolutionComment` helper: creates system memo, links as comment, no re-index |
| 9 | Hackathon | `Tickets.tsx` | Render system suggestion comment with distinct style |

Steps 1–2 are independent. Steps 3–4 are coupled. Steps 5–6 are coupled. Step 7 depends on 5–6. Step 8 is new. Step 9 is frontend only.

---

## 7. Files Modified

| File | Steps | Description |
|------|-------|-------------|
| `server/router/api/v1/memo_service.go` | 1 | Add `TenantID: memo.TenantID` |
| `server/router/api/v1/ticket_service.go` | 2, 7, 8 | Request field, post-index goroutine, system comment helper |
| `server/router/api/v1/agent/service.go` | 5, 6 | Inference returns notes; indexing returns inferred flag |
| `web/src/pages/Tickets.tsx` | 3, 4, 9 | Payload includes tenantId; tenant dropdown; system comment style |
| `web/src/locales/en.json` | 4 | Translation key for tenant dropdown label |

---

## 8. Testing Plan

### 8.1 054b — Auto-ticket tenant propagation

| Test | Steps | Expected |
|------|-------|----------|
| Regular user creates memo | Create memo with `#staff` tag via web UI | Ticket row has `tenant_id = <memo tenant_id>` |
| RAG runs | Wait for goroutine / check logs | `IndexTicketContent` called, no `ticket.TenantID == nil` skip |
| Inference runs | Wait for goroutine / check logs | `InferResolutionForNewTicket` not skipped |

### 8.2 054c — Superuser tenant override

| Test | Steps | Expected |
|------|-------|----------|
| HOST creates ticket with `tenantId` | `POST /api/v1/tickets` with `tenantId: <other tenant>` | Ticket belongs to specified tenant |
| Regular user sends `tenantId` | `POST /api/v1/tickets` as non-admin with `tenantId` | HTTP 400 |
| HOST creates ticket without `tenantId` | `POST /api/v1/tickets` with no `tenantId` | Ticket belongs to JWT tenant (no regression) |

### 8.3 054d — UI tenant dropdown

| Test | Steps | Expected |
|------|-------|----------|
| HOST opens new-ticket modal | Open modal as HOST user | Tenant dropdown visible, lists all tenants |
| Scoped ADMIN opens modal | Open modal as scoped ADMIN | Dropdown visible only if `allowedTenantIds.length > 1` |
| Regular user opens modal | Open modal as USER | Dropdown hidden |
| Change tenant, submit | Select different tenant, submit | Ticket created in selected tenant |

### 8.4 Hackathon — AI suggestion surfacing

| Test | Steps | Expected |
|------|-------|----------|
| Ticket with similar matches | Create ticket whose title matches indexed bug history | `internal_notes` populated + system comment appears in thread |
| Ticket with no matches | Create ticket in thin corpus | No system comment. `internal_notes` stays empty. No error. |
| Comment renders | Open ticket detail after suggestion | System comment shows 🤖 AI Suggestion header + amber styling |
| Re-index preserves comments | Trigger reindex for ticket | System comment remains; duplicate not created |

### 8.5 Regression

| Test | Expected |
|------|----------|
| Existing `IndexTicketContent` callers compile and run | No change in behavior |
| `InferResolutionForNewTicket` returns empty on no-match | No comment created |
| Superuser `CreateTicket` without `tenantId` | Works as before, JWT tenant used |
| Non-superuser `CreateTicket` without `tenantId` | Works as before |

---

## 9. Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|-----------|
| System comment duplicates on reindex | None | Helper intentionally skips re-index. Duplicate creation can only happen if the helper itself is called twice. |
| Superuser can create tickets in any tenant | Low | Same as current admin capability manually. `tenantId` is explicitly an override, not a new privilege. |
| `InferResolutionForNewTicket` signature change | Low | Only internal callers. Changed to return `string`. No external API change. |
| Frontend tenant dropdown adds complexity | Low | Visible only to HOST / multi-tenant ADMIN. Reuses existing tenant list API. |
| System comment runs in goroutine and may race | Low | `createSystemResolutionComment` creates memo + relation sequentially. `getTicketComments` reads committed relations. Safe. |
| System comment not searchable via RAG | Low | Intentional for hackathon. Follow-up can batch and re-index on a schedule or on next user comment. |

---

## 10. Open Questions

1. **Dedup for system comments:** should we check whether an AI Suggestion comment already exists before creating one? Recommended: yes, append a lightweight dedup check querying `MemoRelationComment` for system-bot-authored comments on this ticket. Deferrable to follow-up.
2. **MinScore tuning:** ticket search `MinScore=0.7` and bug search `MinScore=0.5` are probably too strict for demo data. Consider lowering to `0.4`/`0.3` or exposing via tenant config. Recommended: track as follow-up; current fix makes inference work when matches exist.
