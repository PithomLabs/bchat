The principal authorization checks pass their focused tests, but valid attachment updates can be blocked and
  traversal rejects a permitted boundary-depth hierarchy.

  Full review comments:

  - [P1] Skip write checks for resources already bound here — /home/chaschel/Documents/go/bchat/server/router/
    api/v1/memo_resource_service.go:92-94
    When a memo already contains a resource created by another user, including that unchanged resource in
    SetMemoResources fails ActionWrite. This can occur when an admin attaches their own file to a user's ticket;
    afterward the ticket owner cannot preserve it while updating the attachment list. Only resources whose MemoID
    differs from the target memo require reassignment authorization.

  - [P2] Allow traversal at exactly the depth limit — /home/chaschel/Documents/go/bchat/server/router/api/v1/
    resource_service.go:501-503
    A chain containing exactly 10 parent edges is rejected even when the tenth parent is the root, because the
    loop exits immediately after loading it without checking ParentID again. The requirement is to fail when
    traversal exceeds 10, so valid depth-10 comment chains incorrectly fail closed.

