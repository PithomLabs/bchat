# Unified Resource-Authorization Policy Plan

Establish a robust, consistent, and granular resource-authorization policy to manage access to attachments, ticket comments, and unattached files across all user roles (including hosts, admins, ticket owners, commenters, and anonymous users).

## Analysis of the Underlying Problem

The previous plan only patched `GetResourceBinary` and did not resolve the parent-root hierarchy for comment attachments. Comment memos (`MemoRelationComment`) are child nodes linked to a root ticket memo. An attachment uploaded within a comment is associated with that comment's memo ID, not the root ticket. 

To correctly authorize users:
1. **Hierarchy Traversal:** We must recursively trace the parent relation chain (using `memo.ParentID`) to locate the root conversation/ticket memo.
2. **Access Control Inconsistency:** The authorization rules are currently fragmented:
   - `GetResource` leaks metadata without checks.
   - `UpdateResource` allows any authenticated user to rename/modify metadata of any resource.
   - `SetMemoResources` allows linking/detaching resources without ownership or edit checks on the parent memo or the resources.
   - `CreateResource` lets callers link new resources to any arbitrary memo.
   - `ListMemoResources` does not validate access to the parent memo.
   - `DeleteResource` does not support admin overrides unless the admin created the resource.

## Proposed Security Policy Design

### Reusable Traversal & Policy Helper

We will introduce a core verification policy that resolves any resource to its root memo and enforces granular actions (`"read"`, `"write"`, `"delete"`):

```go
// checkResourceAccess checks if the current caller is authorized to perform an action on a resource.
func (s *APIV1Service) checkResourceAccess(ctx context.Context, resource *store.Resource, action string) error {
    user, err := s.GetCurrentUser(ctx)
    if err != nil {
        return status.Errorf(codes.Internal, "failed to get current user: %v", err)
    }

    // 1. Host and Admin (Superusers) always have full access
    if user != nil && isSuperUser(user) {
        return nil
    }

    // 2. Destructive (delete) and write (rename/reassign) actions are restricted to Resource Creator or Superuser
    if action == "delete" || action == "write" {
        if user == nil {
            return status.Errorf(codes.Unauthenticated, "unauthorized access")
        }
        if user.ID != resource.CreatorID {
            return status.Errorf(codes.PermissionDenied, "permission denied")
        }
        return nil
    }

    // 3. For Read action, check resource ownership or memo visibility:
    if user != nil && user.ID == resource.CreatorID {
        return nil
    }

    // Unattached resources are strictly private to creator and superusers
    if resource.MemoID == nil {
        if user == nil {
            return status.Errorf(codes.Unauthenticated, "unauthorized access")
        }
        return status.Errorf(codes.PermissionDenied, "permission denied")
    }

    // Resolve attached memo and its parent hierarchy to locate the root memo
    memo, err := s.Store.GetMemo(ctx, &store.FindMemo{ID: resource.MemoID})
    if err != nil {
        return status.Errorf(codes.Internal, "failed to find memo: %v", err)
    }
    if memo == nil {
        if user == nil {
            return status.Errorf(codes.Unauthenticated, "unauthorized access")
        }
        return status.Errorf(codes.PermissionDenied, "permission denied")
    }

    rootMemo := memo
    for i := 0; i < 5; i++ {
        if rootMemo.ParentID == nil {
            break
        }
        parent, err := s.Store.GetMemo(ctx, &store.FindMemo{ID: rootMemo.ParentID})
        if err != nil || parent == nil {
            break
        }
        rootMemo = parent
    }

    // Apply visibility rules based on the resolved root memo
    switch rootMemo.Visibility {
    case store.Public:
        return nil // Anyone can read public attachments
    case store.Protected:
        if user == nil {
            return status.Errorf(codes.Unauthenticated, "unauthorized access")
        }
        return nil // Any authenticated user can read
    case store.Private:
        if user == nil {
            return status.Errorf(codes.Unauthenticated, "unauthorized access")
        }
        // Root Creator (Ticket Owner) or Commenter (Attached Memo Creator) can read
        if user.ID == rootMemo.CreatorID || user.ID == memo.CreatorID {
            return nil
        }
        return status.Errorf(codes.PermissionDenied, "permission denied")
    }

    return status.Errorf(codes.PermissionDenied, "permission denied")
}
```

---

## Role / Visibility Authorization Matrix

| User Role / Context | Root Attachment (Root Memo is Private) | Root Attachment (Root Memo is Protected) | Root Attachment (Root Memo is Public) | Comment Attachment (Parent Memo is Private) | Unattached Resource (`MemoID == nil`) | Destructive (Delete) / Write (Rename) |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **Host / Admin** | Read: **Allow**<br>Write/Delete: **Allow** | Read: **Allow**<br>Write/Delete: **Allow** | Read: **Allow**<br>Write/Delete: **Allow** | Read: **Allow**<br>Write/Delete: **Allow** | Read: **Allow**<br>Write/Delete: **Allow** | **Allow** (all resources) |
| **Resource Creator** | Read: **Allow**<br>Write/Delete: **Allow** | Read: **Allow**<br>Write/Delete: **Allow** | Read: **Allow**<br>Write/Delete: **Allow** | Read: **Allow**<br>Write/Delete: **Allow** | Read: **Allow**<br>Write/Delete: **Allow** | **Allow** (own resources) |
| **Ticket Owner (Root Creator)** | Read: **Allow**<br>Write/Delete: Denied | Read: **Allow**<br>Write/Delete: Denied | Read: **Allow**<br>Write/Delete: Denied | Read: **Allow** (via root traversal)<br>Write/Delete: Denied | Read: Denied<br>Write/Delete: Denied | **Denied** |
| **Commenter** | Read: Denied (unless commenter = owner) | Read: **Allow** (any auth user) | Read: **Allow** | Read: **Allow** (creator of comment memo)<br>Write/Delete: **Allow** (creator of resource) | Read: Denied<br>Write/Delete: Denied | **Allow** (only for their own resources) |
| **Unrelated User (Auth)** | Read: **Denied** (PermissionDenied) | Read: **Allow** (Protected visibility) | Read: **Allow** | Read: **Denied** (PermissionDenied) | Read: **Denied** (PermissionDenied) | **Denied** |
| **Anonymous** | Read: **Denied** (Unauthenticated) | Read: **Denied** (Unauthenticated) | Read: **Allow** (Public visibility) | Read: **Denied** (Unauthenticated) | Read: **Denied** (Unauthenticated) | **Denied** |

---

## Proposed Changes

### Component: Server Router V1 API

#### [MODIFY] [resource_service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/resource_service.go)

- Implement `checkResourceAccess` helper method.
- Update `GetResourceBinary`: Replace lines 147-167 with a call to `checkResourceAccess(ctx, resource, "read")`. This covers both attachments and thumbnail operations.
- Update `GetResource`: Add `checkResourceAccess` read check before converting and returning metadata.
- Update `UpdateResource`: Add `checkResourceAccess` write check after retrieving resource metadata.
- Update `DeleteResource`: Fetch resource metadata by UID first, run `checkResourceAccess(ctx, resource, "delete")` instead of filtering by `CreatorID` in SQLite query directly.
- Update `CreateResource`: If `request.Resource.Memo != nil`, check that the user is either the creator of the target memo or a superuser (host/admin) before associating.

#### [MODIFY] [memo_resource_service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/memo_resource_service.go)

- Update `SetMemoResources`:
  - Validate that caller has write/edit permissions to the target memo.
  - Verify that caller is authorized to modify each target resource (`checkResourceAccess(ctx, tempResource, "write")`).
- Update `ListMemoResources`:
  - Traverse the parent hierarchy of the memo to find its root memo.
  - Validate that caller is authorized to read the root memo (similar visibility check as `checkResourceAccess`).

---

## Verification Plan

### Automated Tests

We will create a new comprehensive test suite [resource_service_test.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/resource_service_test.go) using `store/test` to validate the security model.

#### [NEW] [resource_service_test.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/resource_service_test.go)

Test scenarios:
- **TestResourceReadAccessPublicRoot**: Asserts anyone (including anonymous) can read attachments linked to public root memos.
- **TestResourceReadAccessProtectedRoot**: Asserts only authenticated users can read attachments linked to protected root memos.
- **TestResourceReadAccessPrivateRoot**: Asserts only the root ticket owner, resource creator, and host/admin can read attachments on a private root memo.
- **TestResourceReadAccessCommentHierarchy**: Asserts that a ticket owner can access an attachment added by an agent on a private ticket comment, while unrelated users are rejected.
- **TestResourceReadAccessUnattached**: Asserts only resource creators and host/admin can access unattached resources.
- **TestResourceWriteAccess**: Asserts renaming/updating is strictly restricted to resource creators and host/admins.
- **TestResourceDeleteAccess**: Asserts deletion is strictly restricted to resource creators and host/admins.
- **TestSetMemoResourcesVerification**: Asserts users cannot attach resources they don't own to arbitrary memos.

To run the automated tests:
```bash
go test -v ./server/router/api/v1/... -run "TestResource"
```

### Manual Verification

1. Run the server locally using `task run`.
2. Sign in as a regular user `UserA`, create a private ticket (memo UID `xyz`), and upload an attachment `img1.png`.
3. Sign in as another user `UserB` (regular user). Attempt to download `img1.png` via `/file/resources/img1.png`. Verify it returns HTTP 403 `codes.PermissionDenied`.
4. Sign in as an anonymous caller. Attempt to download `img1.png`. Verify it returns HTTP 401 `codes.Unauthenticated`.
5. Sign in as Host or Admin user. Verify downloading `img1.png` succeeds with HTTP 200.
6. Verify support staff can add a comment with an attachment `reply.pdf` and that the ticket owner `UserA` can successfully download `reply.pdf`.

