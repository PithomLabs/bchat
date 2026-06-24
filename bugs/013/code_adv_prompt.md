# Prompt: Adversarial Security & Logic Review

## Role
You are a senior security engineer, penetration tester, and adversarial code reviewer. Your objective is to find bugs, bypasses, information leaks, or logic flaws in a newly implemented resource authorization model in a Go (Echo framework) application.

---

## Context & Objectives
In `bchat` (a multi-tenant AI chat platform built on top of Memos), we have refactored the authorization policy for file attachments/resources. 

Here is the security matrix we must enforce:
1. **Access Matrix**:
   - **`ActionRead`**: Superusers (`RoleHost` and `RoleAdmin`) must have global read access across all users (to see attachments in tickets/comments). Resource creators always have read access to their own files. For other users, access to files attached to memos depends on the root memo's visibility (`Public` allows anyone, `Protected` allows authenticated users, and `Private` restricts access to the ticket owner and commenter). Unattached resources (`MemoID == nil`) are private to the creator and superusers.
   - **`ActionWrite` (Rename/Reassign) & `ActionDelete`**: Restrained **strictly** to the resource creator. Superusers (Hosts/Admins) **must be blocked** from modifying or deleting resources they do not own.
2. **Atomicity**:
   - **`SetMemoResources`**: All validation checks must occur before any database mutations (deletions or bindings).
   - **`CreateResource`**: The parent memo check must happen before saving the file blob to S3/local storage.
3. **Cycle-Aware Traversal**:
   - Ticket comments link to parent memos. Traverse links via `ParentID` to evaluate root memo visibility.
   - Guard against circular relations and enforce a strict traversal depth limit of `10`, failing closed upon violation.

---

## Files Under Review
Please audit the following files in the codebase:
- [`server/router/api/v1/resource_service.go`](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/resource_service.go) (contains core helpers `checkResourceAccess`, `checkMemoReadAccess`, `checkMemoWriteAccess`, `resolveRootMemo`)
- [`server/router/api/v1/memo_resource_service.go`](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/memo_resource_service.go) (contains `SetMemoResources` and `ListMemoResources`)
- [`server/router/api/v1/resource_service_test.go`](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/resource_service_test.go) (tests the logic matrix)

---

## Review Angles & Threat Scenarios

Examine the code specifically for the following vulnerability vectors:

### 1. Authorization Bypass Vectors
- **Anonymous Bypasses**: Can a request with `user == nil` (anonymous) somehow trigger a `nil` return in `checkResourceAccess` for a `Private` or `Protected` resource/memo?
- **Host/Admin Action Escalation**: Does any path in `checkResourceAccess` let a Host or Admin perform `ActionWrite` or `ActionDelete` on another user's resource?
- **Root Traversal Logic Bugs**: In `resolveRootMemo`, if a comment chain exists, does `checkMemoReadAccess` verify the user's permission against both the root memo and the child comment memo correctly? What if the comment memo itself is private but the root is public, or vice-versa?

### 2. Failure Handling & Fail-Open Vulnerabilities
- **Trapping Loops/Depth Limits**: If `resolveRootMemo` reaches a cycle or exceeds depth `10`, it returns an error. Does the caller of `resolveRootMemo` properly propagate that error and fail closed, or does it defaults to allowing access?
- **Missing Resource/Memo checks**: If a resource is fetched, but its linked memo is not found (`memo == nil` in database), does the system fail closed?

### 3. Mutational Concurrency & Atomic Race Conditions
- **`SetMemoResources` Race Conditions**: Is it possible to race a request where a resource is bound to a memo we do not own, or delete a resource during update?
- **Double Binding / Hijacking**: Can a malicious user bind a resource they do not own to a memo they do own, or rebind a resource they own to a memo they do not own?

### 4. Information Disclosures & Side Channels
- **Database/Error Leaks**: Does returning `codes.NotFound` vs `codes.PermissionDenied` leak the existence of private memos or resources to unauthorized users?
- **Enclosure/RSS Leakage**: Does the RSS feed generation (`server/router/rss/rss.go`) bypass the updated checks and inadvertently publish links to private or protected attachments?

---

## Deliverables
Provide a structured report detailing:
1. **Critical Findings**: Any paths where security checks can be bypassed or logic can be broken.
2. **Edge Cases**: Non-standard inputs or states that could lead to unexpected behavior.
3. **Recommendations**: Concrete diffs or refactoring proposals for any identified flaws.
