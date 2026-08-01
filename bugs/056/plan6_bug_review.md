# Adversarial Plan Review: Bug 056 — Plan 6 MVP Decision

**Bug/Task:** Review of `bugs/056/plan6_bug.md`  
**Reviewer:** Codex, senior Go architect  
**Date:** 2026-08-01  
**Verdict:** APPROVED WITH REQUIRED NITS — implement the MVP, defer broad relation API cleanup

## Executive Summary

Plan 6 is too broad for the immediate production bug. The fastest safe path is to fix the original failing ticket auto-comment flow and the directly related ticket re-index comment fetch path. Do **not** implement the full memo relation/comment API refactor in this pass.

The broad plan is directionally useful, but it pulls in `CreateMemoComment`, `SetMemoRelations`, `ListMemoRelations`, `ListMemoComments`, scoped-admin semantics, relation conversion, visibility/write permissions, and many tests. That is too much blast radius for this bug.

## MVP Scope

Implement only:

1. `createSystemResolutionComment`
   - Resolve parent memo by UID only.
   - Add empty `/m/` UID guard.
   - Allow parent memo when `TenantID == nil` or matches the ticket tenant.
   - Reject non-nil mismatched tenant as `"parent memo not found"`.
   - Split nil-error handling so `%!w(<nil>)` disappears.

2. `getTicketComments`
   - Resolve parent memo by UID only.
   - Add empty `/m/` UID guard.
   - Allow parent memo when `TenantID == nil` or matches `ticket.TenantID`.
   - Filter `ListMemoRelations` by `ticket.TenantID`.
   - Keep comment memo fetch by ID as-is.

3. Add a small unexported helper local to package `v1`, or near the ticket helpers:

```go
func memoBelongsToTenantOrLegacy(memo *store.Memo, tenantID *int32) bool {
    if memo == nil {
        return false
    }
    if memo.TenantID == nil {
        return true
    }
    if tenantID == nil {
        return false
    }
    return *memo.TenantID == *tenantID
}
```

This helper is tenant-compatibility only. Do not use it as a general read/write authorization helper.

## Explicit Deferrals

Defer all broader gRPC memo API changes:

- `CreateMemoComment`
- `SetMemoRelations`
- `ListMemoRelations`
- `ListMemoComments`
- `convertMemoRelationFromStore`
- `MemoIsAccessible`
- `RelationTenantID`

Those paths need a separate access-control design because they mix tenant checks, visibility checks, write permissions, superuser behavior, and scoped-admin behavior. They should not block the original ticket auto-comment fix.

## Required Tests

Add only focused tests for the MVP:

1. `createSystemResolutionComment` succeeds when ticket tenant is `19` and parent memo has `tenant_id=NULL`.
2. `createSystemResolutionComment` rejects when ticket tenant is `19` and parent memo has `tenant_id=20`.
3. `createSystemResolutionComment` returns nil for `Description == "/m/"`.
4. `getTicketComments` finds comments for a legacy nil-tenant parent memo when relations are scoped to `ticket.TenantID`.
5. `getTicketComments` excludes relations for other tenant IDs and nil-tenant relations.

Run:

```bash
go test -v -run 'TestCreateSystemResolutionComment|TestGetTicketComments' ./server/router/api/v1/ -count=1
go test -v ./server/router/api/v1/ -count=1
```

## Required Nits Before Coding

- Update `plan6_bug.md` or implementation notes to state this is an MVP ticket-service-only fix.
- Do not introduce exported broad helpers like `MemoIsAccessible` in this pass.
- Do not alter gRPC memo relation/comment behavior in this pass.
- Keep errors generic; do not include memo UID in returned errors.

## Final Decision

Approved for implementation only under the MVP scope above. The original production bug can be fixed safely with a small ticket-service change and focused tests. Broad memo relation API cleanup should be filed as follow-up work, not bundled into this hotfix.
