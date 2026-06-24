# Walkthrough: Unified Resource-Authorization Policy

Successfully implemented and verified the unified resource-authorization policy for `bchat`. All 14 test cases pass correctly, validating security, access matrix enforcement, and fail-closed behaviors.

## Changes Made

### 1. Unified Resource Authorization Engine
- **Files**: [resource_service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/resource_service.go)
- **Implementations**:
  - `ResourceAction`: Created typed enum (`ActionRead`, `ActionWrite`, `ActionDelete`).
  - `resolveRootMemo`: Traverses comment chains using `ParentID` to locate the root memo while avoiding loops (cycle-safe with a visited map) and limiting depth to 10.
  - **[P2 Fix]**: Refactored traversal recursion loop boundary check to check `ParentID == nil` at depth 10, failing only if the chain exceeds 10 edges (allowing exactly 10 parent edges).
  - `checkMemoReadAccess` / `checkMemoWriteAccess`: Centralized read and write visibility checks.
  - `checkResourceAccess`: Replaces individual endpoint authorization code with a single, consistent matrix enforcement.
- **Endpoints updated**:
  - `GetResource` / `GetResourceBinary`: Securely enforces read access.
  - `UpdateResource`: Restricts renaming/assigning strictly to the resource creator (blocked for hosts/admins on resources they don't own).
  - `DeleteResource`: Retreives the metadata first and applies `ActionDelete` check to verify only the resource creator can delete.
  - `CreateResource`: Resolves the target memo and validates write access **before** writing any file blobs to disk/S3 (preventing orphaned resources).
  - `ListResources`: Allows superusers (`RoleHost` and `RoleAdmin`) to list all resources, while regular users are filtered.

### 2. Memo Resource Binding
- **Files**: [memo_resource_service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/memo_resource_service.go)
- **Implementations**:
  - `SetMemoResources`: Pre-validates memo write access, write access on incoming resources, and delete access on removed resources **before** modifying the database (ensuring atomic validation).
  - **[P1 Fix]**: Exempted resources already linked to the target memo from the `ActionWrite` check, enabling updates to the resource list without failing on attachments originally added by other users (like admins).
  - `ListMemoResources`: Checks read access to the memo before listing.

### 3. Comprehensive Verification Tests
- **Files**: [resource_service_test.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/resource_service_test.go)
- **Implementations**:
  - Added 14 test cases covering:
    - Public, Protected, and Private root memo read visibility.
    - Comment hierarchy memo traversal (`ParentID` resolution for comment replies).
    - Unattached resources isolation.
    - Write/Delete access restrictions (denying hosts/admins cross-user mutations).
    - Admin list bypass.
    - Atomic batch set-memo-resources validation.
    - **[New Test]** Preserve existing bound resources created by different users.
    - Pre-blob creation authorization check.
    - **[Updated Test]** Loop/Cycle detection and depth traversal limits (asserts that exactly 10 edges succeed while 11 edges fail).
    - Invalid action error handling.
    - Separating host/admin role cross-user read operations from modification operations.

## Test Results

Executed the test suite package-wide:
```bash
go test -v ./server/router/api/v1 -run "Test(Resource|ListResources|SetMemoResources|CreateResource|CycleAndDepthLimits|InvalidActionFailClosed|HostAndAdminSeparate)"
```

Output:
```
=== RUN   TestResourceReadAccessPublicRoot
--- PASS: TestResourceReadAccessPublicRoot (0.04s)
=== RUN   TestResourceReadAccessProtectedRoot
--- PASS: TestResourceReadAccessProtectedRoot (0.04s)
=== RUN   TestResourceReadAccessPrivateRoot
--- PASS: TestResourceReadAccessPrivateRoot (0.04s)
=== RUN   TestResourceReadAccessCommentHierarchy
--- PASS: TestResourceReadAccessCommentHierarchy (0.04s)
=== RUN   TestResourceReadAccessUnattached
--- PASS: TestResourceReadAccessUnattached (0.03s)
=== RUN   TestResourceWriteAccessRestrictions
--- PASS: TestResourceWriteAccessRestrictions (0.03s)
=== RUN   TestResourceDeleteAccessRestrictions
--- PASS: TestResourceDeleteAccessRestrictions (0.03s)
=== RUN   TestListResourcesAdminAccess
--- PASS: TestListResourcesAdminAccess (0.04s)
=== RUN   TestSetMemoResourcesAtomicRejection
--- PASS: TestSetMemoResourcesAtomicRejection (0.04s)
=== RUN   TestSetMemoResourcesPreserveOtherUserResource
--- PASS: TestSetMemoResourcesPreserveOtherUserResource (0.04s)
=== RUN   TestCreateResourcePreBlobAuthorization
--- PASS: TestCreateResourcePreBlobAuthorization (0.04s)
=== RUN   TestCycleAndDepthLimits
--- PASS: TestCycleAndDepthLimits (0.07s)
=== RUN   TestInvalidActionFailClosed
--- PASS: TestInvalidActionFailClosed (0.04s)
=== RUN   TestHostAndAdminSeparate
--- PASS: TestHostAndAdminSeparate (0.03s)
PASS
ok      github.com/usememos/memos/server/router/api/v1  0.551s
```

