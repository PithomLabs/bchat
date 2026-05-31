# Duplicate LLM Reply Fix: Root Cause Analysis and Implementation Plan

## Status: ROOT CAUSE IDENTIFIED - AWAITING APPROVAL

## Problem Statement
When an external user adds a comment to a non-escalated ticket, the automated LLM reply comes in twice (duplicate comments with different answers). This is NOT a race condition in the LLM itself — it's a code bug causing the handler to be invoked twice.

## Root Cause Analysis

### Flow: External User Adds Comment to Non-Escalated Ticket

1. **`CreateMemoComment`** is called (`memo_service.go:457`)
2. **Line 471**: Calls `s.CreateMemo(ctx, &v1pb.CreateMemoRequest{Memo: request.Comment})` internally
3. **Inside `CreateMemo (lines 106-110)`**: Because user is NOT a superuser, it triggers AI response:
   ```go
   go s.handleTicketAIResponse(context.Background(), memo.UID, user.ID, memo.Content)
   ```
   This is **FIRST INVOCATION** — but with WRONG context (comment not yet linked to parent)
4. **Back in `CreateMemoComment (lines 525-527)`**: ALSO triggers AI response:
   ```go
   go s.handleTicketAIResponse(context.Background(), memo.UID, user.ID, request.Comment.Content)
   ```
   This is **SECOND INVOCATION** — with correct parent ticket context

### Why Two Different Replies?
Both goroutines make concurrent LLM calls. The first invocation (from `CreateMemo`) may have incomplete parent detection logic since the comment-parent relation hasn't been established yet. Both save their results, creating duplicate AI comment replies.

### Code Evidence
```go
// CreateMemo (line 106-110) - called internally by CreateMemoComment
if !isSuperUser(user) {
    isEscalated := s.handleAutoTicketCreation(ctx, memo, user)  // Returns false for comments
    if !isEscalated {
        go s.handleTicketAIResponse(...)  // TRIGGERS AI for comments - WRONG!
    }
}

// CreateMemoComment (line 525-527)
user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &creatorID})
if err == nil && user != nil && !isSuperUser(user) {
    go s.handleTicketAIResponse(...)  // TRIGGERS AI again - CORRECT context
}
```

## Proposed Fix: Context-Based Skip Flag

Since the comment-parent relation is created AFTER `CreateMemo` returns, we can't detect it from within `CreateMemo`. The cleanest fix is to pass a context value.

### Implementation

**File: `server/router/api/v1/memo_service.go`**

1. **Define context key** (near top of file, after imports):
```go
type contextKey string
const skipTicketAIKey contextKey = "skip-ticket-ai-response"
```

2. **Modify `CreateMemoComment` line 471** to add context with skip flag:
```go
// Create the memo comment first.
if request.Comment.Visibility == v1pb.Visibility_VISIBILITY_UNSPECIFIED {
    request.Comment.Visibility = v1pb.Visibility_PUBLIC
}
ctxWithSkip := context.WithValue(ctx, skipTicketAIKey, true)
memoComment, err := s.CreateMemo(ctxWithSkip, &v1pb.CreateMemoRequest{Memo: request.Comment})
```

3. **Modify `CreateMemo` lines 106-110** to check context:
```go
if !isSuperUser(user) {
    isEscalated := s.handleAutoTicketCreation(ctx, memo, user)
    // Skip if context indicates this is a comment creation (handled by caller)
    skipTicketAI, _ := ctx.Value(skipTicketAIKey).(bool)
    if !isEscalated && !skipTicketAI {
        go s.handleTicketAIResponse(context.Background(), memo.UID, user.ID, memo.Content)
    }
}
```

### Why This Approach?

1. **No proto changes needed** - Uses Go context for internal signaling
2. **Surgical fix** - Only affects the code path where the bug exists
3. **Preserves correct behavior** - Root-level memos still trigger AI response
4. **Self-contained** - All changes in one file
5. **No security concerns** - Context value can't be forged by external API clients

## Verification Checklist

- [ ] `CreateMemo` for root-level memos still triggers AI response (for chat integration)
- [ ] `CreateMemoComment` still triggers AI response for comments
- [ ] No duplicate AI response triggered
- [ ] Existing tests pass
- [ ] No proto regeneration needed

## User Confirmation Required

Please confirm:
1. The context-based skip flag approach is acceptable
2. Any additional edge cases to consider (e.g., webhook-triggered memo creation, batch imports)