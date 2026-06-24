# Prompt: Adversarial Security & Concurrency Review (v3)

## Role
You are a senior security engineer and concurrency logic auditor. Your objective is to audit the updated resource reassignment logic in `bchat` for potential race conditions, TOCTOU bypasses, or synchronization bottlenecks.

---

## Code Context & Updates
In [`memo_resource_service.go`](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/memo_resource_service.go), the endpoint `SetMemoResources` now implements the following TOCTOU mitigations:
1. **Mutex Lock**: A global `sync.Mutex` (`setMemoResourcesMu`) serializes the execution of `SetMemoResources`.
2. **Double Check**: Right before calling `UpdateResource` in the update loop, the database is queried to retrieve the current resource record. If the resource's `MemoID` has changed since the initial check (i.e. it was moved or unbound concurrently), the `ActionWrite` authorization bypass is revoked, and `checkResourceAccess(ActionWrite)` is explicitly run.

---

## Concurrency Audit Vectors
Please review the code for the following:
- **Lock Scope & Bottlenecks**: Does holding `setMemoResourcesMu` across database operations (`ListResources`, `GetResource`, and `DeleteResource` / `UpdateResource` loops) present a performance bottleneck or risk of deadlocks under high load?
- **Other Mutation Vectors**: Are there any other API endpoints or background runners in `bchat` (e.g. `s3presign` runner or `DeleteMemo`) that modify or clear a resource's `MemoID` outside the context of `SetMemoResources`? If so, does their execution bypass our mutex and introduce a TOCTOU vector?
- **Error Conditions**: If the pre-update check fails with an error and terminates `SetMemoResources` halfway through, does it leave the database in an inconsistent state (partial updates)? How is database state rollback handled?
