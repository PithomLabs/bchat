# Fix Missing Comments & Enforce Visibility in Ticket Modal

## Root Cause Analysis

I've deep dived into both the frontend React code and the Go backend to understand why comments aren't updating immediately for admins.

**The Expected Flow:**
1. You add a comment in the "Edit Ticket" modal.
2. The frontend saves it and triggers a re-fetch of the parent ticket description memo to get the latest `memo.relations` (comments).
3. The UI updates the "Comments (X)" counter and displays the new comment.

**Why it fails for Admins:**
While we previously fixed `GetMemo` and `ListMemoComments` to allow admins to view `PRIVATE` memos, we missed one crucial endpoint: **`ListMemoRelations`** in `server/router/api/v1/memo_relation_service.go`.

When the backend populates a memo's relations (comments), it builds a SQL query filter (`memoFilter`). For non-admins, this filter is strictly set to:
`creator_id == {current_user_id} || visibility in ["PUBLIC", "PROTECTED"]`

Crucially, the database applies this filter to **BOTH** the comment memo and the parent memo. Because the parent ticket memo is `PRIVATE` and was created by the customer (not the admin), it fails the filter check for the admin. 

As a result, the backend silently returns `0` relations for the ticket description memo. The frontend receives an empty `relations` array, setting the counter to "Comments (0)" and hiding your newly created comment.

## Visibility Setting Recommendation

> **Which setting should we choose for the comment to work?**
> We should choose **`PROTECTED`** for ticket comments.

**Why PROTECTED works perfectly:**
- **Customer Access:** The customer can read the admin's `PROTECTED` replies (if it were `PRIVATE`, the customer wouldn't be able to see the admin's reply).
- **Admin Access:** The admin can read the customer's and their own replies.
- **Privacy:** `PROTECTED` comments do not leak to other logged-in users because (1) comments are strictly excluded from the main Memos feed by default, and (2) other customers cannot access the `PRIVATE` parent ticket memo, meaning they can never fetch its associated comments. 

## Proposed Changes (No Band-Aid Fix)

### 1. Backend: Fix `ListMemoRelations` for Admins
- **File:** `server/router/api/v1/memo_relation_service.go`
- **Change:** We will apply the exact same admin-bypass that we used for `ListMemoComments`. If the requesting user is a super-user (admin/host), we will leave `memoFilter` as `nil`. This guarantees that admins bypass the strict visibility filters, allowing the backend to correctly return the comment relations and fixing the UI reactivity immediately.

### 2. Frontend: Remove Dropdown & Enforce `PROTECTED`
To achieve the goal of removing the visibility dropdown so users don't have to think about it, we will add support to force visibility settings in the `MemoEditor` component and apply it to the Tickets page.

- **File:** `web/src/components/MemoEditor/index.tsx`
  - Add `hideVisibilitySelector?: boolean` and `forceVisibility?: Visibility` to the component `Props`.
  - Update the state initialization logic to prioritize `forceVisibility` over inheriting the parent memo's visibility.
  - Hide the `<VisibilitySelector />` UI element if `hideVisibilitySelector` is true.

- **File:** `web/src/pages/Tickets.tsx`
  - When rendering the `<MemoEditor />` for adding a comment, pass `hideVisibilitySelector={true}` and `forceVisibility={Visibility.PROTECTED}`.

## User Review Required

> [!IMPORTANT]
> The backend change fixes the root cause of the "missing comment" bug, and the frontend changes fulfill your request to lock the visibility to `PROTECTED` and hide the dropdown. Does this comprehensive plan look good to you? Once approved, I will proceed with the code changes.


I will investigate the backend `ListMemoComments` logic to see how it handles a customer trying to read an admin's comment on their ticket.

Viewed memo_service.go:540-570

I will inspect the `MemoEditor` properties and implementation to see how we can hide the visibility selector and enforce `PROTECTED` visibility for ticket comments.

Viewed index.tsx:38-100

I will view the `useAsyncEffect` block in `MemoEditor` to confirm how visibility is initialized and how we can introduce a `forceVisibility` prop to override it correctly for tickets.

Viewed index.tsx:110-140

I will update the implementation plan to address your requirement regarding the visibility dropdown. I've analyzed the visibility modes and `PROTECTED` is the perfect setting for ticket comments, as it allows both the customer and admin to view the conversation while preventing the comments from leaking into the public space.

Edited implementation_plan.md

I have updated the [implementation_plan.md](file:///home/chaschel/.gemini/antigravity-ide/brain/5f08cd34-3bb8-4a9f-a70e-74c54f2fca71/implementation_plan.md) artifact! 

I investigated the implications of the different visibility settings for comments in the context of the ticket management system. **`PROTECTED`** is the ideal setting for ticket comments because:
1. It guarantees that the customer (ticket creator) can see the internal support agent's replies.
2. It remains hidden from the general Memos feed because comments are excluded from global feeds by default, and other customers cannot fetch them since they lack access to the `PRIVATE` ticket description.

To accomplish your goal of removing the dropdown so users don't have to think about it, I've outlined a robust, non-hacky approach to pass a `forceVisibility` parameter into the frontend `MemoEditor` component specifically for tickets.

Please review the revised plan. If it looks good, let me know and I will immediately proceed with executing the code changes!