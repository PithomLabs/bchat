The implementation breaks supported PostgreSQL/MySQL deployments and bypasses resource storage cleanup. It also
  introduces an unintended memo timestamp mutation.

  Full review comments:

  - [P1] Use database-specific SQL syntax — /home/chaschel/Documents/go/bchat/server/router/api/v1/
    memo_resource_service.go:52-52
    When PostgreSQL is configured, this statement fails because lib/pq requires $1 placeholders; on MySQL,
    assigning Unix integers directly to TIMESTAMP columns also bypasses the required FROM_UNIXTIME conversion.
    Since PostgreSQL and MySQL are supported backends, SetMemoResources is unusable outside SQLite unless these
    transactional operations are implemented per driver/dialect.

  - [P1] Delete backing resource objects — /home/chaschel/Documents/go/bchat/server/router/api/v1/
    memo_resource_service.go:161-163
    When a resource is omitted from the request, this raw SQL deletes only its database row. The previous
    Store.DeleteResource path also removes local files or S3 objects, so every successful removal now leaves the
    backing object orphaned, causing storage leaks and retained user data.

  - [P2] Avoid changing the memo timestamp solely to acquire a lock — /home/chaschel/Documents/go/bchat/server/
    router/api/v1/memo_resource_service.go:51-52
    Every successful resource-set operation now writes the current time into memo.updated_ts, even though no memo
    content changed. This can reorder memos and make them appear newly edited; use a dialect-appropriate row lock
    or a no-op write that does not mutate user-visible metadata.

