 ## Review result

  plan2.md is substantially better, but still not implementation-ready.

  ### High-severity issues

  1. Host/admin deletion remains an unjustified permission expansion.
     The requirement is cross-user access, not unrestricted rename/delete. The policy grants full destructive
     access at bugs/013/plan2.md:33. Read and management permissions must be decided separately.

  2. SetMemoResources still permits unauthorized deletion.
     The plan validates only resources supplied in the new request (bugs/013/plan2.md:140). The implementation
     also deletes existing resources omitted from that request. Every removed resource must be authorized before
     deletion, preferably before any mutation to avoid partial updates.

  3. Host/admin still cannot list all resources.
     ListResources remains filtered by the current creator. The original requirement says hosts/admins should
     access attachments from all users, and the Resources UI depends on this endpoint. The plan omits it
     entirely.

  4. Creation authorization must occur before blob storage.
     CreateResource currently writes the blob before resolving the requested memo. Adding authorization at the
     existing memo-resolution location could leave orphaned local/S3 files after denial. Resolve and authorize
     the memo before SaveResourceBlob.

  ### Policy/design problems

  5. The helper is fail-open for unknown actions.
     Any value other than "write" or "delete" receives read behavior. Replace strings with a typed action and an
     exhaustive switch that rejects unknown values.

  6. Root traversal silently accepts incomplete resolution.
     The five-level limit and break on lookup errors (bugs/013/plan2.md:74) can authorize against an intermediate
     memo. Traversal must:
      - Follow ParentID, which is derived from MemoRelationComment.
      - Detect cycles.
      - Fail closed on missing parents or database errors.
      - Avoid an unexplained fixed depth, or return an error when the safety limit is exceeded.

  7. The “unified policy” remains duplicated.
     ListMemoResources proposes a separate “similar visibility check” (bugs/013/plan2.md:143). That will drift.
     Extract shared root-resolution and memo-read authorization helpers used by resource and memo-resource
     endpoints.

  8. Memo write authorization is underspecified.
     “Write/edit permissions to the target memo” must explicitly mean memo creator or host/admin, matching
     UpdateMemo. Otherwise the implementer still has to choose the policy.

  ### Missing verification

  - Test host/admin ListResources.
  - Test resources removed through SetMemoResources.
  - Test atomic rejection when one resource in a batch is unauthorized.
  - Test missing parent, cyclic relation, and excessive hierarchy depth.
  - Test CreateResource against another user’s memo and verify no blob is written.
  - Test invalid authorization actions fail closed.
  - Test Host and Admin separately.
  - The manual URL is incorrect; it should be /file/resources/<uid>/<filename>, not /file/resources/img1.png.

  The plan is close, but findings 1–4 should be resolved before implementation.

