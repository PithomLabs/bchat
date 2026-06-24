# Unified Resource-Authorization Policy Plan (V3)

Establish a robust, consistent, and granular resource-authorization policy to manage access to attachments, ticket comments, and unattached files across all user roles (including hosts, admins, ticket owners, commenters, and anonymous users).

## Analysis of the Underlying Problem & Design Fixes

1. **Write/Delete Isolation:** Read and management permissions are separated. Hosts and admins (superusers) are granted read access across all users, but **rename/write and delete operations are restricted strictly to the resource creator** (and blocked for hosts/admins on resources they do not own).
2. **Atomic SetMemoResources Validation:** Omitted resources in `SetMemoResources` will be deleted. To prevent partial updates, we pre-validate the whole request:
   - Verify caller has write permissions to the target memo.
   - Verify caller has write permissions to all incoming resources.
   - Verify caller has delete permissions to all resources being removed.
   If any check fails, the request is rejected immediately before database mutations occur.
3. **Admin Resource Listing:** `ListResources` currently filters strictly by the current user. It is updated to show all resources to hosts/admins, allowing the Resources UI to work as intended.
4. **Early CreateResource Authorization:** Memo checks are moved to the top of `CreateResource`. Resolving and validating memo write access occurs **before** `SaveResourceBlob` to prevent orphaned assets on local storage or S3 upon authorization failure.
5. **Fail-Closed Enum Actions:** Replaced action strings with a typed action `ResourceAction` and an exhaustive switch rejecting unknown values.
6. **Cycle-Aware Memo Traversal:** Implemented a traversal helper (`resolveRootMemo`) that follows `ParentID`, detects cycles using a visited set, fails closed on db/lookup errors, and errors if the max depth (10) is exceeded.
7. **Unified Reuse of Helpers:** Extracted shared `checkMemoReadAccess` and `checkMemoWriteAccess` helpers, eliminating duplicate visibility checks.

---

## Role / Visibility Authorization Matrix

| User Role / Context | Root Attachment (Root Memo is Private) | Root Attachment (Root Memo is Protected) | Root Attachment (Root Memo is Public) | Comment Attachment (Parent Memo is Private) | Unattached Resource (`MemoID == nil`) | Rename / Reassign (Write) | Destructive (Delete) |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **Host** | Read: **Allow**<br>Write/Delete: Denied | Read: **Allow**<br>Write/Delete: Denied | Read: **Allow**<br>Write/Delete: Denied | Read: **Allow**<br>Write/Delete: Denied | Read: **Allow**<br>Write/Delete: Denied | **Denied** (unless creator) | **Denied** (unless creator) |
| **Admin** | Read: **Allow**<br>Write/Delete: Denied | Read: **Allow**<br>Write/Delete: Denied | Read: **Allow**<br>Write/Delete: Denied | Read: **Allow**<br>Write/Delete: Denied | Read: **Allow**<br>Write/Delete: Denied | **Denied** (unless creator) | **Denied** (unless creator) |
| **Resource Creator** | Read: **Allow**<br>Write/Delete: **Allow** | Read: **Allow**<br>Write/Delete: **Allow** | Read: **Allow**<br>Write/Delete: **Allow** | Read: **Allow**<br>Write/Delete: **Allow** | Read: **Allow**<br>Write/Delete: **Allow** | **Allow** | **Allow** |
| **Ticket Owner (Root Creator)** | Read: **Allow**<br>Write/Delete: Denied | Read: **Allow**<br>Write/Delete: Denied | Read: **Allow**<br>Write/Delete: Denied | Read: **Allow** (via root traversal)<br>Write/Delete: Denied | Read: Denied<br>Write/Delete: Denied | **Denied** | **Denied** |
| **Commenter** | Read: Denied (unless commenter = owner) | Read: **Allow** (any auth user) | Read: **Allow** | Read: **Allow** (creator of comment memo)<br>Write/Delete: Denied (unless creator) | Read: Denied<br>Write/Delete: Denied | **Denied** (unless creator) | **Denied** (unless creator) |
| **Unrelated User (Auth)** | Read: **Denied** (PermissionDenied) | Read: **Allow** (Protected visibility) | Read: **Allow** | Read: **Denied** (PermissionDenied) | Read: **Denied** (PermissionDenied) | **Denied** | **Denied** |
| **Anonymous** | Read: **Denied** (Unauthenticated) | Read: **Denied** (Unauthenticated) | Read: **Allow** (Public visibility) | Read: **Denied** (Unauthenticated) | Read: **Denied** (Unauthenticated) | **Denied** | **Denied** |

---

## Proposed Changes

### Component: Server Router V1 API

#### [MODIFY] [resource_service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/resource_service.go)

- Define `ResourceAction` type and constants (`ActionRead`, `ActionWrite`, `ActionDelete`).
- Implement `resolveRootMemo`: Cyclic-safe memo hierarchy traversal following `ParentID` to root memo.
- Implement `checkMemoReadAccess`: Verifies read authorization on a memo (and its comment descendants) using root memo visibility.
- Implement `checkMemoWriteAccess`: Verifies write authorization on a memo (creator or host/admin).
- Implement `checkResourceAccess`: Enforces the above matrix using typed actions and fail-closed switch logic.
- Update `GetResourceBinary`: Replaces checks with `checkResourceAccess(ctx, resource, ActionRead)`.
- Update `GetResource`: Add `checkResourceAccess(ctx, resource, ActionRead)`.
- Update `UpdateResource`: Add `checkResourceAccess(ctx, resource, ActionWrite)`.
- Update `DeleteResource`: Retrieve resource metadata by UID first, run `checkResourceAccess(ctx, resource, ActionDelete)`.
- Update `CreateResource`: Extract and resolve memo link at the top. Run `checkMemoWriteAccess` **before** storage operations to avoid orphaned local/S3 files.
- Update `ListResources`: If user is Host or Admin, list all resources. Otherwise, filter by `CreatorID: &user.ID`.

#### [MODIFY] [memo_resource_service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/memo_resource_service.go)

- Update `SetMemoResources`:
  - Verify caller has write permissions to target memo (`checkMemoWriteAccess`).
  - Pre-validate write access to all incoming resources.
  - Pre-validate delete access to all omitted resources being deleted.
  - Fail atomically before any DB mutations occur.
- Update `ListMemoResources`: Use `checkMemoReadAccess` to validate access to the parent memo.

---

## Verification Plan

### Automated Tests

We will create a new comprehensive test suite [resource_service_test.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/resource_service_test.go) using `store/test`.

#### [NEW] [resource_service_test.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/resource_service_test.go)

Test cases:
1. **TestResourceReadAccessPublicRoot**: Anyone (including anonymous) can read attachments linked to public root memos.
2. **TestResourceReadAccessProtectedRoot**: Only authenticated users can read attachments linked to protected root memos.
3. **TestResourceReadAccessPrivateRoot**: Only the root ticket owner, resource creator, host, and admin can read attachments on a private root memo.
4. **TestResourceReadAccessCommentHierarchy**: Asserts that a ticket owner can access an attachment added by an agent on a private ticket comment, while unrelated users are rejected.
5. **TestResourceReadAccessUnattached**: Only resource creators, host, and admin can access unattached resources.
6. **TestResourceWriteAccessRestrictions**: Asserts renaming/updating is strictly restricted to resource creators (blocked for unrelated users, host, and admin).
7. **TestResourceDeleteAccessRestrictions**: Asserts deletion is strictly restricted to resource creators (blocked for unrelated users, host, and admin).
8. **TestListResourcesAdminAccess**: Asserts Host and Admin list all resources, while regular users only list their own.
9. **TestSetMemoResourcesAtomicRejection**: Asserts atomic fail-closed behavior when a batch has an unauthorized write or delete.
10. **TestCreateResourcePreBlobAuthorization**: Asserts that memo authorization is checked first and no local/S3 blob is stored if verification fails.
11. **TestCycleAndDepthLimits**: Asserts that cyclic parent relations or excessive hierarchy depths are detected and failed closed with an error.
12. **TestInvalidActionFailClosed**: Asserts helper rejects invalid/unknown action codes.
13. **TestHostAndAdminSeparate**: Verifies Host and Admin roles behave identically regarding cross-user read operations but are restricted from managing other users' resources.

To run automated tests:
```bash
go test -v ./server/router/api/v1/... -run "TestResource"
```

### Manual Verification

1. Run the server locally using `task run`.
2. Sign in as `UserA`, create a private ticket (memo UID `xyz`), and upload an attachment `img1.png`.
3. Sign in as `UserB` (regular user). Attempt to download the attachment via `/file/resources/<uid>/<filename>`. Verify it returns HTTP 403 `codes.PermissionDenied`.
4. Sign in as an anonymous caller. Attempt to download the attachment. Verify it returns HTTP 401 `codes.Unauthenticated`.
5. Sign in as Host or Admin user. Verify downloading `img1.png` succeeds with HTTP 200.
6. Attempt to delete `img1.png` while logged in as a Host or Admin. Verify it is rejected with HTTP 403 `codes.PermissionDenied`.
7. Verify that support staff can add a comment with an attachment `reply.pdf` and that the ticket owner `UserA` can successfully download `reply.pdf`.

