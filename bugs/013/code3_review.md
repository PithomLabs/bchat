The process-local mutex does not close cross-instance or DeleteMemo races, and the check/update sequence
  remains non-atomic. Failures can also leave permanently partial resource sets, while the global lock introduces
  substantial cross-tenant contention.

  Full review comments:

  - [P1] Make the authorization check and reassignment atomic — /home/chaschel/Documents/go/bchat/server/router/
    api/v1/memo_resource_service.go:132-145
    In multi-instance deployments, the process-local mutex does not prevent another instance from moving the
    resource between this re-fetch and UpdateResource. A caller who bypassed ActionWrite because the resource
    originally belonged to this memo can therefore overwrite a concurrent reassignment and take it back without
    creator permission; use a transaction, row lock, or conditional update and verify the affected row count.

  - [P1] Prevent stale deletions after concurrent reassignment — /home/chaschel/Documents/go/bchat/server/router/
    api/v1/memo_resource_service.go:112-115
    When another process reassigns a resource after this request lists it, this deletion can remove the resource
    from its new memo. Although MemoID is supplied, every database driver's DeleteResource ignores that field and
    deletes solely by ID, so the mutex provides no protection across application instances; the deletion must
    condition on both ID and the expected memo.

  - [P1] Roll back mutations when a later check fails — /home/chaschel/Documents/go/bchat/server/router/api/v1/
    memo_resource_service.go:110-112
    If any re-fetch, authorization check, or update fails after this deletion loop begins, previously deleted
    resources and earlier updates remain committed while the endpoint reports failure. The new pre-update check
    creates an additional expected failure path after destructive work, so the complete replacement needs one
    database transaction or equivalent rollback behavior.

  - [P1] Coordinate concurrent memo deletion — /home/chaschel/Documents/go/bchat/server/router/api/v1/
    memo_resource_service.go:145-148
    DeleteMemo bypasses this mutex, so after the target memo is loaded it can be deleted concurrently before
    these updates execute. Because the current schemas do not enforce a resource-to-memo foreign key, an incoming
    resource can then be updated to reference the deleted memo; memo existence and resource mutations should be
    protected by the same transaction or conditional write.

  - [P2] Avoid serializing every tenant behind one mutex — /home/chaschel/Documents/go/bchat/server/router/api/
    v1/memo_resource_service.go:21-22
    Every SetMemoResources request in the process now waits on one global lock, including unrelated tenants. The
    critical section contains numerous database calls and DeleteResource, which may perform synchronous
    filesystem or S3 deletion, so one slow request can block all resource reassignment traffic; scope
    synchronization per resource or memo, or rely on database transactions and conditional writes.

