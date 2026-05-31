# Plan Review: Missing Comments Update in Ticket Modal for Admins

## Status: APPROVED with minor nits

## Findings

### plan.md (Backend-only) vs plan2.md (Comprehensive)

Both correctly identify the root cause: Missing superuser bypass in `ListMemoRelations`.

### Visibility Analysis: Critical Correction

When `ListMemoRelations` fetches a comment to show in relations, the `memoFilter` applies to the **comment memo being fetched**, NOT the parent.

**For customers to see admin replies:**
- If admin comment is PRIVATE: `creator_id == customer_id || visibility in [PUBLIC, PROTECTED]` → FALSE (admin != customer, PRIVATE not in PUBLIC/PROTECTED) → **Customer CANNOT see it**
- If admin comment is PROTECTED: `creator_id == customer_id || visibility in [PUBLIC, PROTECTED]` → TRUE (PROTECTED is in PUBLIC/PROTECTED) → **Customer CAN see it**

**Therefore plan2.md's PROTECTED recommendation is CORRECT** - it ensures customers can view admin replies on their PRIVATE ticket memos.

### Code Flow Verification

1. `MemoEditor:index.tsx:127-130` already inherits parent visibility when `parentMemoName` is set
2. For tickets, the parent is the PRIVATE ticket memo → comments would inherit PRIVATE
3. This is INCORRECT behavior - customers can't see PRIVATE comments from admins

### Recommendation

**APPROVE plan2.md FULLY:**
1. **Backend fix** - Essential for admins to see any comments at all
2. **`forceVisibility={PROTECTED}`** - Correct! Overrides PRIVATE parent and ensures bidirectional visibility
3. **`hideVisibilitySelector`** - Good UX improvement since visibility is now predetermined

The plan correctly addresses both the technical bug and the visibility requirement for ticket comments.