# AI Suggestion Trigger Flow — How RAG Inference Works in Ticket Creation

**Date:** 2026-08-01
**Status:** Verified — all paths traced with line numbers
**Context:** Bug 052 (per-ticket RAG indexing) + Bug 054 (tenant association fixes)

---

## 1. What Happens When You Create a Ticket

```
User clicks "Create Ticket" in modal
  │
  ├─► POST /api/v1/tickets                          [ticket_service.go:56]
  │     │
  │     ├─ Validate + dedup check                    [lines 86-183]
  │     │   └─ Checks if /m/<uid> already has a ticket
  │     │
  │     ├─ Normal creation path (no existing ticket) [line 185]
  │     │   └─ Store.CreateTicket()
  │     │
  │     └─ Launch goroutine                          [lines 192-213]
  │           │
  │           ├─ IndexTicketContent(..., true)        [line 195]
  │           │   │
  │           │   ├─ Build content blob:              [lines 5729-5739]
  │           │   │   "# {title}\n\n{description}"
  │           │   │
  │           │   ├─ Content-hash dedup               [lines 5740-5763]
  │           │   │   └─ Skip if hash unchanged
  │           │   │
  │           │   ├─ UpsertAgentSourceFile            [line 5766]
  │           │   ├─ ReindexFileVersion               [line 5777]
  │           │   │
  │           │   └─ InferResolutionForNewTicket()    [line 5784]
  │           │       │
  │           │       ├─ Search 1: similar tickets    [lines 5615-5621]
  │           │       │   ContentTypes: ["ticket"]
  │           │       │   TopK: 3, MinScore: 0.7
  │           │       │
  │           │       ├─ Search 2: bug history        [lines 5627-5633]
  │           │       │   ContentTypes: ["bug_section"]
  │           │       │   TopK: 3, MinScore: 0.5
  │           │       │
  │           │       ├─ Build suggestion markdown    [lines 5638-5677]
  │           │       │   "## Suggested Resolution (Auto-generated)"
  │           │       │
  │           │       └─ UpdateTicket(internal_notes) [line 5691]
  │           │
  │           ├─ Re-fetch ticket                      [line 202]
  │           │   └─ Read populated InternalNotes
  │           │
  │           └─ createSystemResolutionComment()      [line 209]
  │               │
  │               ├─ CreateMemo                       [lines 646-652]
  │               │   CreatorID: store.SystemBotID (0)
  │               │   Content: "## AI Suggestion\n\n" + suggestion
  │               │
  │               └─ UpsertMemoRelation               [lines 657-661]
  │                   Type: store.MemoRelationComment
  │
  └─► Return JSON (internalNotes: "")                 [line 217]
        │
        └─ User sees ticket in list (inference still running async)
```

**Key:** The HTTP response returns immediately. Inference runs async in the goroutine.

---

## 2. Dedup Path — No Inference

If a ticket with the same `/m/<uid>` description already exists, the handler merges into it:

```go
// ticket_service.go:134-179
// Smart merge: preserves auto-derived priority/type if user didn't override
// Re-indexes content, but triggerInference=false (line 174)
```

**Why:** Deduped tickets are updates to existing tickets, not new ones. Re-inferring would produce duplicate noise.

---

## 3. All Indexing Triggers

### 3.1 New Ticket Creation → **Inference YES**

| File | Line | Trigger | `triggerInference` |
|------|------|---------|-------------------|
| `ticket_service.go` | 195 | `CreateTicket` (non-dedup) | **`true`** |

**This is the only trigger that produces AI Suggestion comments.**

### 3.2 Ticket Update → Re-index Only

| File | Line | Trigger | `triggerInference` |
|------|------|---------|-------------------|
| `ticket_service.go` | 433 | `UpdateTicket` (edit title/description/status) | `false` |
| `ticket_service.go` | 174 | `CreateTicket` dedup (same memo link) | `false` |

### 3.3 Comment Operations → Re-index Parent Ticket

| File | Line | Trigger | `triggerInference` |
|------|------|---------|-------------------|
| `memo_service.go` | 694 | `CreateMemoComment` (new comment on ticket) | `false` |
| `memo_service.go` | 498 | `UpdateMemo` (edit existing comment on ticket) | `false` |

**Flow for comment triggers:**
```
Comment created/edited
  │
  ├─ Find parent memo of the comment
  ├─ Find ticket with description = /m/<parent_uid>
  ├─ Fetch all comments for that ticket
  └─ IndexTicketContent(..., comments, false)
      └─ Re-indexes content with comments included
         Vector DB updated for future searches
         NO inference triggered
```

---

## 4. Why Only Creation Triggers Inference

| Event | Inference? | Rationale |
|-------|-----------|-----------|
| New ticket created | **YES** | Creator gets immediate insights on similar past tickets |
| Ticket edited | NO | Content changed — re-index for searchability, but no new suggestions |
| New comment added | NO | More context — re-index so future tickets can find this one |
| Comment edited | NO | Same as above — index for future, don't re-trigger |

**Design principle:** Inference is a one-time event at ticket creation. It answers "what similar tickets existed when this was created?" Re-triggering on every edit would:
- Waste LLM calls
- Produce duplicate/contradictory suggestions
- Create noisy comment history

The tradeoff: if a ticket's title changes significantly after creation, the initial suggestions may no longer be relevant. This is acceptable — the content is still re-indexed, so *future* tickets will find the updated version.

---

## 5. What the User Sees

### 5.1 AI Suggestion Comment

**Location:** Ticket detail modal → "Comments" section
**File:** `Tickets.tsx:802`

```tsx
const isSystemSuggestion = liveMemo.content.startsWith("## AI Suggestion");
// Renders with amber styling, robot emoji, "AI Suggestion" label
```

**Visibility:** Always visible to anyone who can see the ticket's parent memo.

### 5.2 Internal Notes

**Location:** Ticket detail page → yellow box
**File:** `TicketDetail.tsx:92-98`

**Visibility:** Permission-gated (`ticket_service.go:576-581`):

| User Type | Can See? |
|-----------|----------|
| Superuser (HOST/ADMIN) | YES |
| Ticket creator | YES |
| Assigned user | YES |
| User with `ticket:internal_notes` permission | YES |
| Everyone else | NO |

**Roles with `ticket:internal_notes`:** `analyst`, `editor`, `tenant_admin`

### 5.3 Timing

```
t=0    User clicks "Create Ticket"
t=0    HTTP response returns (internalNotes: "")
t=1-2s Goroutine completes indexing + inference
t=1-2s AI Suggestion comment created (system bot memo)
t=1-2s User opens ticket detail → sees suggestion
```

The suggestion appears **reactively** — `Tickets.tsx:135-140` watches `memoStore.state.stateId` and re-fetches comments.

---

## 6. Content Hash Dedup

`IndexTicketContent` uses content-hash dedup (`service.go:5740-5763`) to avoid creating duplicate versions:

```go
contentHash := ContentHash(content)
existing, _ := s.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{
    TenantID: &tenantID, FileType: strPtr("ticket"), LatestOnly: true,
})
if existing != nil && existing.ContentHash == contentHash {
    // Content unchanged — reindex existing version, don't create new row
    chunks, err = s.ReindexFileVersion(ctx, tenantID, "internal", "ticket", existing.Version, content, 0)
    return
}
```

**Why it matters:** When a comment is added and the ticket is re-indexed, the content blob now includes the comment. The hash changes → new version row created → vector DB updated with new chunks that include the comment text.

---

## 7. System Comment Creation

`createSystemResolutionComment` (`ticket_service.go:632-668`):

```go
func (s *APIV1Service) createSystemResolutionComment(ctx context.Context, tenantID int32, ticket *store.Ticket, suggestion string) error {
    // 1. Find parent memo from description /m/<uid>
    // 2. Create memo with SystemBotID (0)
    memo, err := s.Store.CreateMemo(ctx, &store.Memo{
        CreatorID:  store.SystemBotID,  // 0
        Content:    "## AI Suggestion\n\n" + suggestion,
        Visibility: store.Public,
        TenantID:   &tenantID,
    })
    // 3. Link as COMMENT relation
    _, err = s.Store.UpsertMemoRelation(ctx, &store.MemoRelation{
        MemoID:        memo.ID,
        RelatedMemoID: parentMemo.ID,
        Type:          store.MemoRelationComment,
        TenantID:      &tenantID,
    })
}
```

**System bot:** `store.SystemBotID` is a constant `0` (`store/user.go:32`). The comment appears with a robot emoji in the UI (`Tickets.tsx:814`).

---

## 8. Summary

| Question | Answer |
|----------|--------|
| When does inference fire? | Only on **new ticket creation** (non-dedup) |
| Does adding a comment trigger inference? | **No** — re-indexes content only |
| Does editing a ticket trigger inference? | **No** — re-indexes content only |
| Does editing a comment trigger inference? | **No** — re-indexes parent ticket content only |
| Where does the suggestion appear? | AI Suggestion comment (memo) + InternalNotes field |
| Is the suggestion visible immediately? | Async — appears after goroutine completes (~1-2s) |
| Can all users see it? | AI Suggestion: yes (if memo visible). InternalNotes: permission-gated. |
| What if content doesn't change? | Content-hash dedup skips new version row, re-indexes existing |
