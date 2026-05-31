# Implementation Plan: Missing Comments Update in Ticket Modal for Admins

## Status: IMPLEMENTATION ALREADY COMPLETE

After reviewing all plan documents and comparing against the actual codebase, the implementation has already been applied. Both backend and frontend fixes are in place.

## Summary of Verified Changes

### Backend: `server/router/api/v1/memo_relation_service.go`
✅ **Lines 78-85**: Superuser bypass is already implemented correctly
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

### Frontend: `web/src/components/MemoEditor/index.tsx`
✅ **Lines 51-54**: Props already defined
✅ **Lines 128-135**: `forceVisibility` priority logic already implemented
✅ **Lines 597-603**: Visibility selector already hidden when `hideVisibilitySelector` is true

### Frontend: `web/src/pages/Tickets.tsx`
✅ **Lines 709-710**: Props already passed for comment editor
```tsx
forceVisibility={Visibility.PROTECTED}
hideVisibilitySelector={true}
```

## Optional Nit Fixes (Non-blocking)

### 1. Eliminate Render Flash in MemoEditor
**File**: `web/src/components/MemoEditor/index.tsx:85-86`

**Current**:
```typescript
useState<State>({
  memoVisibility: defaultVisibility,
```

**Suggested**:
```typescript
useState<State>({
  memoVisibility: forceVisibility || defaultVisibility,
```

This eliminates the one-render window where visibility shows PRIVATE before being overridden to PROTECTED.

### 2. Add Type Guard for `extractMemoUidFromDescription`
**File**: `web/src/pages/Tickets.tsx:42-46`

Add a type guard to ensure the function cannot return null when used in `parentMemoName`, though the existing React gate at line 688 already handles this safely.

## Security Verification

| Check | Result |
|-------|--------|
| Admin reads PRIVATE ticket | Allowed (GetMemo bypass) |
| Admin reads PROTECTED comments | Allowed (ListMemoRelations bypass) |
| Customer reads admin PROTECTED comment | Allowed (PROTECTED matches filter) |
| Other customers access comments | Blocked (cannot reach PRIVATE parent) |

## Recommendation

No further changes required. The implementation is complete and approved.