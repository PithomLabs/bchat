 The depth boundary and cycle handling fail closed correctly, and resolveRootMemo has no nil-success path.
  However, the preservation exemption introduces a TOCTOU authorization bypass during concurrent resource
  reassignment.

  Review comment:

  - [P1] Make the preserved-resource update conditional — /home/chaschel/Documents/go/bchat/server/router/api/v1/
    memo_resource_service.go:117-120
    When a foreign-owned resource initially belongs to this memo, the ownership check is skipped, but this later
    update is unconditional. If its owner concurrently moves or unbinds it after validation, the caller can
    reattach it without ActionWrite permission. Use a transaction/lock or an update conditioned on the resource
    still having the validated memo ID.
