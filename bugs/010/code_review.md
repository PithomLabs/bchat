Based on my review of the codebase:

**Plan Review: APPROVED with nits**

The core fix for `GetMemo` is already implemented correctly (line 249 includes `!isSuperUser(user)` check). However, I found a potential issue:

**Findings:**
1. **`GetMemo` (line 249)**: ✅ Correctly allows superusers to view PRIVATE memos
2. **`ListMemoComments` (lines 547-554)**: ⚠️ **Potential issue** - The filter `creator_id == %d || visibility in ["PUBLIC", "PROTECTED"]` excludes PRIVATE memos from non-admin users, but when `memoFilter` is nil for superusers, this should work. However, the approach differs from GetMemo.
3. **AI comments (line 1116)**: ⚠️ AI auto-replies use `PRIVATE` visibility, which will now be viewable by admins due to the GetMemo fix, but other non-admin users won't see them in comment lists.

**Nit issues to address:**
1. **AI comments should use PROTECTED visibility** instead of PRIVATE (line 1116) - This aligns with the "internal support can see" requirement since PROTECTED is visible to all logged-in users but not the public
2. **VisibilitySelector.tsx** doesn't enforce admin-only visibility selection - This is fine per the plan (Option A uses existing visibility)

**Recommendation:** Change the AI response memo visibility from `store.Private` to `store.Protected` so that support staff (logged-in users) can also see the comments, while still keeping them hidden from public users.