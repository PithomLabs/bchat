 The plan identifies the immediate failing check, but its authorization model may be broader than the stated
  requirement. I’m tracing resource ownership, memo/ticket visibility, role definitions, and existing tests
  before judging the proposed bypasses.

• Explored
  └ Read resource_service.go
    Search RoleHost|RoleAdmin|Superuser|superuser|GetResourceBinary|DeleteResource|unauthorized access in server
    Search resource_service|GetResourceBinary|DeleteResource|/file/resources|Resource in v1
    Search type Resource|type Memo struct|Visibility|ResourceID|ResourceId|resource.*memo|memo.*resource in store
    Read memo_resource_service.go, memo_service.go, resource.go, common.go, ticket_service.go, user.go, memo.go,
         acl_config.go, acl.go
    Search ticket|Ticket in ticket_service.go

─────────────────────────────────────────────────────────────────────────────────────────────────────────────────

• A key defect in the proposed reasoning is now clear: an attachment on a ticket comment is linked to the comment
  memo, not directly to the ticket’s root memo. Therefore “memo creator bypass” does not grant the ticket owner
  access to attachments uploaded by another commenter. I’m checking the remaining resource endpoints for
  inconsistent authorization so the review covers the root model, not only the reported download path.

• Explored
  └ Read plan.md, resource_service.go, memo_resource_service.go
    Search SetMemoResources|ListMemoResources|CreateResource|UpdateResource|GetResource\( in proto
    Search CreateMemoComment|MemoRelationComment|ListMemoComments|ListMemoRelations|RelatedMemoID in
    memo_service.go
    Search *test.go in v1
    Search NewAPIV1Service|AuthenticationInterceptor|GetMemo in v1
    Read memo_service.go, acl_config.go, common.go, ticket_service.go

─────────────────────────────────────────────────────────────────────────────────────────────────────────────────

• ## Adversarial review

  The plan should not be approved as written.

  ### High-severity findings

  1. The proposed owner check is incorrect for comment attachments.
     Resources on comments reference the comment memo, whose creator is the commenter—not the ticket owner
     (server/router/api/v1/memo_service.go:496). Therefore, the proposed “parent memo/ticket creator” check in
     bugs/013/plan.md:45 cannot be implemented using only resource.MemoID. The authorization logic must resolve
     comment relations back to the root ticket memo.

  2. It expands “access” into deletion without justification.
     The requirement is to access tickets, conversations, and attachments. The plan additionally grants host/
     admin deletion of other users’ files (bugs/013/plan.md:49). That is a separate destructive permission
     requiring explicit policy and tests.

  3. It does not address the underlying inconsistent authorization model.
     Resource authorization is scattered and currently has serious gaps:
      - GetResource exposes metadata to any authenticated user.
      - UpdateResource allows any authenticated user to rename any resource (server/router/api/v1/
        resource_service.go:205).

      - SetMemoResources can attach, detach, or delete resources without ownership checks (server/router/api/v1/
        memo_resource_service.go:16).

      - ListMemoResources does not validate access to the memo.
      - CreateResource can directly associate a resource with an arbitrary memo without checking memo access.

     Fixing only binary download and deletion preserves contradictory security rules.

  4. The verification plan has no authorization tests.
     A backend build does not validate security behavior. Automated service/API tests are required; manual
     testing only covers one host/admin success case.

  ### Additional issues

  - GetResourceBinary is anonymously reachable (server/router/api/v1/acl_config.go:23). The plan does not define
    behavior for unattached resources, missing parent memos, protected resources, or anonymous callers.

  - Denied authenticated users should receive PermissionDenied, not Unauthenticated.
  - “Fully backwards compatible” (bugs/013/plan.md:32) is unsupported: this changes a security boundary and
    potentially grants destructive access.

  - The plan says it covers “tickets, conversations and attachments,” but only modifies two attachment
    operations.

  ### Required plan revision

  Define one reusable resource-authorization policy that:

  - Resolves a resource to its attached memo and, for comments, to the root memo/ticket.
  - Grants host/admin read access across users.
  - Grants regular users access only when they may view the root conversation.
  - Treats read, rename/reassignment, and deletion as separate permissions.
  - Applies consistently across every resource and memo-resource endpoint.
  - Includes a role/visibility matrix covering host, admin, ticket owner, commenter, unrelated user, and
    anonymous access for root attachments, comment attachments, thumbnails, unattached resources, and destructive
    operations.

