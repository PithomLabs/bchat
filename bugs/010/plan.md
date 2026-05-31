# Fix 403 Error for Ticket Memos (Visibility Permissions)

This plan addresses the issue where internal users (admins and support staff) encounter a 403 Permission Denied error when trying to view the description or comments of a ticket created by a customer.

## Root Cause Analysis

I've deep dived into the codebase and found the exact sequence of events causing the 403 error:

1. **Forced `PRIVATE` Visibility**: When a non-superuser (a customer) creates a ticket, the frontend might request `PUBLIC` visibility, but the backend `CreateMemo` function (in `server/router/api/v1/memo_service.go`) forcefully overrides the visibility to `PRIVATE`:
   ```go
   if !isSuperUser(user) {
       create.Visibility = store.Private
   }
   ```
2. **Missing Admin Bypass on Read**: In the Memo API, functions like `UpdateMemo` and `DeleteMemo` explicitly allow admins to bypass the `PRIVATE` restriction (e.g., `if memo.CreatorID != user.ID && !isSuperUser(user) { return permission_denied }`). However, the **`GetMemo`**, **`ListMemos`**, and **`ListMemoComments`** functions strictly enforce that `PRIVATE` memos can *only* be seen by their creator. They are missing the admin bypass check.
3. **The Result**: When an admin opens a customer's ticket, the `TicketModal` tries to load the underlying memo for the description or comments. Since the memo is `PRIVATE` and the admin is not the creator, the backend strictly rejects the read request with a 403 error.

## User Review Required

Since we are retrofitting the Memos app into a ticket management system where tickets should be visible to "the customer + internal support", we need to agree on how to represent this visibility state. 

I propose two options:

### Option A: Fix `PRIVATE` to mean "Customer + Support" (Recommended)
Instead of adding a new visibility setting, we update the existing `PRIVATE` visibility logic to allow Super Users (Admins/Hosts) to view them. 
* **Why it's better:** It aligns perfectly with your requirement. Memos/Tickets remain completely hidden from other customers, but internal support can view, comment, and resolve them. It also requires no risky database schema or Protobuf changes. Admins can already edit/delete `PRIVATE` memos, so allowing them to *read* them fixes an existing inconsistency.

### Option B: Introduce a new `TICKET_RESTRICTED` Visibility Level
We add a new enum value (e.g., `INTERNAL` or `SUPPORT`) to the Protobuf definitions, update the frontend TypeScript generated files, run DB migrations, and update the backend logic to handle this new state.
* **Why it's worse:** It's a much heavier lift (requires re-compiling protos and managing migrations) and essentially duplicates the behavior we'd get by just fixing the read permissions for `PRIVATE`.

## Open Questions

> [!IMPORTANT]
> **Do you approve going with Option A?** By allowing admins to read `PRIVATE` memos, we fix the 403 error at its root without needing "band-aid" UI fixes or massive database/protobuf changes. This perfectly achieves the "visible only to internal users and the external customer" requirement.

## Proposed Changes (Assuming Option A)

### Backend (Memo API)

#### [MODIFY] `server/router/api/v1/memo_service.go`
- **`GetMemo`**: Update the permission check to allow super-users.
  ```diff
  - if memo.Visibility == store.Private && memo.CreatorID != user.ID {
  + if memo.Visibility == store.Private && memo.CreatorID != user.ID && !isSuperUser(user) {
  ```
- **`ListMemos`**: Update the query filter so that if the requesting user is a super-user, they can fetch `PRIVATE` memos created by others.
- **`ListMemoComments`**: Ensure the internal relation queries don't filter out comments based on `PRIVATE` visibility if the requester is an admin.

## Verification Plan

### Automated/Manual Verification
- Log in as a customer (non-admin) and create a new ticket.
- Switch to an Admin/Host account.
- Open the Tickets board and click on the newly created ticket.
- Verify that the `TicketModal` loads the description (memo) and comments successfully without a 403 error.
- Verify that *other* customers still cannot view the ticket.



Ran command: `ls -R | grep -i memo | grep -i model`
Searched for "Visibility"
Searched for "ticket modal"
Searched for "ticket #11"
Listed directory bugs
Searched for "TicketModal"
Searched for "Visibility"
Viewed Tickets.tsx:1-766
Searched for "403"
Searched for "func (s *MemoService) GetMemo"
Searched for "PermissionDenied"
Viewed memo_service.go:1-800
Searched for "Role"
Searched for "func isSuperUser"
Viewed common.go:1-69
Searched for "/api/v1/tickets"
Searched for "/tickets"
Created implementation_plan.md

I've investigated the root cause of the 403 error you are seeing when attempting to load the memo URL description in the ticket modal. 

The core issue stems from how the system enforces visibility:
1. When a normal user (customer) creates a ticket, the backend forcefully overrides the visibility of the underlying memo to `PRIVATE`.
2. When the frontend later attempts to fetch that memo, the `GetMemo` logic explicitly blocks anyone who isn't the creator from viewing `PRIVATE` memos, *even if they are a super-user/admin*. 

This is an oversight in the read permissions, as admins are already allowed to edit and delete `PRIVATE` memos.

I have created an [Implementation Plan](file:///home/chaschel/.gemini/antigravity-ide/brain/5f08cd34-3bb8-4a9f-a70e-74c54f2fca71/implementation_plan.md) with two proposed options to resolve this permanently. Option A recommends adjusting the `PRIVATE` permissions to allow admins to view them, perfectly achieving your requirement of "visible only to internal support and the external customer" without needing extensive database/schema migrations. 

Please review the plan and let me know if you approve of Option A or if you'd prefer to go the route of adding a completely new visibility setting.