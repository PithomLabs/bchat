# Plan Review: Missing Comments Update in Ticket Modal for Admins

## Status: APPROVED with one clarification needed

## Findings

### Root Cause Analysis: ✅ VALIDATED

The plan correctly identifies the issue in `server/router/api/v1/memo_relation_service.go:ListMemoRelations` (lines 78-83):

```go
var memoFilter string
if currentUser == nil {
    memoFilter = `visibility == "PUBLIC"`
} else {
    memoFilter = fmt.Sprintf(`creator_id == %d || visibility in ["PUBLIC", "PROTECTED"]`, currentUser.ID)
}
```

This differs from `memo_service.go:ListMemoComments` (lines 547-554) which properly handles superuser bypass:

```go
var memoFilter *string
if currentUser == nil {
    filterStr := `visibility == "PUBLIC"`
    memoFilter = &filterStr
} else if !isSuperUser(currentUser) {
    filterStr := fmt.Sprintf(`creator_id == %d || visibility in ["PUBLIC", "PROTECTED"]`, currentUser.ID)
    memoFilter = &filterStr
}
```

### Proposed Fix: ✅ SOUND

The backend change to apply the same superuser bypass pattern is correct and follows existing conventions.

### Pending Clarification

The original prompt also asked: *"which setting should we choose for the comment to work in this ticket management system?"*

Currently `Tickets.tsx` line 709 hardcodes `defaultVisibility={Visibility.PUBLIC}` for all comments. For admin comments on customer PRIVATE tickets, consider inheriting visibility from the parent memo (as `MemoEditor` does at lines 127-130).

**Decision needed:** Should admin comments on PRIVATE ticket memos:
1. Inherit the parent memo's PRIVATE visibility (current recommended approach)
2. Or remain PUBLIC?

Answer: Inherit the parent memo's PRIVATE visibility

## Implementation Steps

1. Modify `server/router/api/v1/memo_relation_service.go:ListMemoRelations`:
   - Change `memoFilter string` to `memoFilter *string`
   - Add `else if !isSuperUser(currentUser)` check
   - Update database query calls to handle pointer

2. (Optional) Update `Tickets.tsx` to inherit parent memo visibility for comments when admin is commenting on a PRIVATE memo
