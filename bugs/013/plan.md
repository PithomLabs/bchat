## prompt

as bchat host or admin, I should be able to access all tickets, conversations and attachments from all users. as example when ate signed in and attached a file in comment per ticket 14, I was not able to access  it saying {"code":16, "message":"unauthorized access", "details":[]}

write plan to fix this, analyze the underlying problem so it is not a band-aid fix, do not code yet


## plan

# Support Admin Authorization Fix for Ticket Attachments

Provide host/admin and memo owners the ability to access and manage attachments (resources) across all users.

## Analysis of the Underlying Problem

When a user tries to download or view a private attachment (e.g. an image attached to a comment on a ticket), the request is handled by the `GetResourceBinary` endpoint in [resource_service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/resource_service.go). 

The logic checking permissions is currently:
```go
if memo.Visibility == store.Private && user.ID != resource.CreatorID {
    return nil, status.Errorf(codes.Unauthenticated, "unauthorized access")
}
```
This check is overly restrictive because:
1. **No Superuser Bypass:** It fails to allow `RoleHost` or `RoleAdmin` users to bypass the private memo visibility checks, even though they are authorized to see the parent ticket/conversation.
2. **No Memo Creator Bypass:** If a system bot or support agent uploads an attachment to a customer's private ticket comment, the customer (the creator of the memo) is denied access to view or download it because they are not the *resource* creator.

A similar limitation exists in `DeleteResource` where `s.Store.GetResource` is queried filtering by the current user's ID (`CreatorID: &user.ID`). This prevents hosts/admins from deleting or managing attachments from other users.

## User Review Required

> [!NOTE]
> This change is fully backwards compatible and aligned with existing role permissions in `memo_service.go` and `ticket_service.go`. It does not require any breaking database schema changes.

## Proposed Changes

### Resource Service

Modify resource permission checks in `server/router/api/v1/resource_service.go`.

---

#### [MODIFY] [resource_service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/resource_service.go)

- Update `GetResourceBinary` (lines 163-165) to allow resource access if:
  1. The user is a superuser (host/admin).
  2. The user is the creator of the parent memo/ticket.
  3. The user is the creator of the resource.
- Update `DeleteResource` (lines 246-249) to bypass the creator ID constraint if the user is a superuser (host/admin).

## Verification Plan

### Automated Tests
- Build the backend using `task build:backend` to ensure compilation and validation succeed.

### Manual Verification
1. Run the backend locally with `task run`.
2. Sign in as a regular user, create a ticket, and add a comment with an attachment.
3. Sign in as a Host or Admin user.
4. Try to access the ticket comment's attachment using the URL structure `/file/resources/<uid>/<filename>`.
5. Verify the attachment downloads successfully without returning the `"unauthorized access"` error.
