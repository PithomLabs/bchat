# Adversarial Plan Review: Bug 056 — Plan 3 `parent memo not found`

**Bug/Task:** Review of `bugs/056/plan3_bug.md`  
**Reviewer:** Codex, senior Go architect  
**Date:** 2026-08-01  
**Verdict:** REWORK REQUIRED

## Executive Summary

`plan3_bug.md` closes the main scope gap from `plan2_bug.md`: it now covers the ticket helpers, `CreateMemoComment`, `ListMemoComments`, `SetMemoRelations`, and `ListMemoRelations`. The direction is correct: resolve globally unique memo UIDs without a SQL tenant filter, then enforce tenant compatibility in Go.

However, the plan still has implementation-breaking API mistakes and one important access-control mismatch with existing `GetMemo` behavior. It is close, but not safe to implement as written.

| # | Finding | Severity | Required Action |
|---|---------|----------|-----------------|
| 1 | Helper returns true for nil memo | HIGH | Fix helper semantics |
| 2 | Proposed snippets use non-existent proto fields/types | HIGH | Correct API references |
| 3 | Helper conflicts with unscoped superuser/admin behavior | HIGH | Make access rule user-aware or narrow usage |
| 4 | `CreateMemoComment` still has a tenant-filtered post-create lookup | MEDIUM | Document or simplify |
| 5 | `convertMemoRelationFromStore` can still nil-deref after relation filtering | MEDIUM | Add handling or tests |
| 6 | Tests need gRPC status assertions and admin coverage | MEDIUM | Expand tests |

## Findings

### 1. Helper Returns True for Nil Memo

**Severity:** HIGH

The proposed helper comment says nil memo is caller-handled:

```go
//   - it is nil (caller must handle separately), or
```

But the implementation returns true for nil memo:

```go
if memo == nil || memo.TenantID == nil {
    return true
}
```

That is wrong. A nil memo must not be considered accessible. The helper should be:

```go
func MemoBelongsToTenantOrLegacy(memo *store.Memo, tenantID *int32) bool {
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

Callers should still return `NotFound` or nil as appropriate, but the helper itself must be safe.

### 2. Proposed Snippets Use Non-Existent Proto Fields and Return Types

**Severity:** HIGH

Several code snippets in the plan will not compile:

- `SetMemoRelations` currently returns `(*emptypb.Empty, error)`, not `(*v1pb.SetMemoRelationsResponse, error)`.
- Related memo names are accessed as `relation.RelatedMemo.Name`, not `relation.RelatedMemoName`.
- `ListMemoCommentsRequest` has `Name`, not `ParentName`.

The plan should correct these snippets before implementation so the implementer does not have to infer API shape from generated proto code.

### 3. Helper Conflicts with Existing Unscoped Superuser/Admin Behavior

**Severity:** HIGH

The helper proposes:

```go
if tenantID == nil {
    return false // tenant-scoped memo requires explicit tenant context
}
```

That conflicts with `GetMemo`, which explicitly allows superusers to access tenant-scoped memos even when there is no tenant context:

```go
if memo.TenantID != nil {
    if tenantID == nil || *memo.TenantID != *tenantID {
        if !isSuperUser(user) {
            return nil, status.Errorf(codes.PermissionDenied, "permission denied")
        }
    }
}
```

If the new helper is applied to `SetMemoRelations`, `ListMemoRelations`, or `ListMemoComments`, unscoped host/admin workflows may regress. The plan must choose one of these designs:

1. Keep `MemoBelongsToTenantOrLegacy` only for tenant-required internal paths like ticket auto-commenting, and use a separate user-aware access check for gRPC memo APIs.
2. Replace it with a user-aware helper that accepts `user *store.User` and allows superuser bypass consistently with `GetMemo`.
3. Explicitly decide that relation/comment APIs are stricter than `GetMemo`, and add tests proving that this is intended.

The recommended approach is a user-aware helper for gRPC memo APIs and the simpler tenant-only helper for ticket internals.

### 4. `CreateMemoComment` Still Has a Tenant-Filtered Post-Create Lookup

**Severity:** MEDIUM

The plan fixes the parent lookup in `CreateMemoComment`, but leaves the lookup for the newly created comment memo conceptually unchanged:

```go
findMemo2 := &store.FindMemo{UID: &memoUID}
if tenantID := GetTenantIDFromContext(ctx); tenantID != nil {
    findMemo2.TenantID = tenantID
}
memo, err := s.Store.GetMemo(ctx, findMemo2)
```

This is usually fine because `CreateMemo` sets `TenantID` from context, so the new comment should match. But the plan should either:

- document that this lookup remains tenant-filtered intentionally because the memo was just created in the same context; or
- simplify by using the created memo identity directly if feasible.

Leaving this implicit invites another “UID lookup plus tenant filter” regression discussion later.

### 5. `convertMemoRelationFromStore` Can Still Nil-Deref

**Severity:** MEDIUM

`ListMemoRelations` calls `convertMemoRelationFromStore`, and the converter loads both sides of a relation with tenant filters derived from `memoRelation.TenantID`:

```go
findMemo := &store.FindMemo{ID: &memoRelation.MemoID}
if memoRelation.TenantID != nil {
    findMemo.TenantID = memoRelation.TenantID
}
memo, err := s.Store.GetMemo(ctx, findMemo)
// memo.Content dereferenced later
```

If a relation has `tenant_id=19` but points to a legacy memo with `tenant_id=NULL`, the ID+tenant lookup returns nil and the converter can dereference nil. This is not fixed by changing the initial UID lookup.

The plan should either:

- update `convertMemoRelationFromStore` to resolve relation endpoints by ID without tenant filter and apply tenant compatibility checks before conversion; or
- return a clean conversion error when either endpoint is nil; and
- add a test that exercises `ListMemoRelations` with a tenant-scoped relation involving a legacy memo.

### 6. Tests Need Status Code and Admin Coverage

**Severity:** MEDIUM

The proposed tests are directionally good, but they should be more precise:

- For gRPC methods, assert `status.Code(err)` rather than only checking that an error occurred.
- Add coverage for unscoped host/admin behavior if the helper is used by relation/comment APIs.
- Add a `ListMemoRelations` conversion test where a relation is tenant-scoped but one endpoint memo is legacy nil-tenant.
- Keep the ticket tests focused on the original regression: legacy parent memo succeeds, cross-tenant parent memo fails, and nil-tenant comment relations are intentionally excluded from ticket re-indexing.

## Required Plan Rework

Before implementation, update `plan3_bug.md` to:

1. Fix `MemoBelongsToTenantOrLegacy` so `memo == nil` returns false.
2. Correct all proto field and method signature references.
3. Define how unscoped superusers/admins should behave in relation/comment APIs, matching or intentionally differing from `GetMemo`.
4. Document or remove the tenant-filtered post-create lookup in `CreateMemoComment`.
5. Address `convertMemoRelationFromStore` nil handling for tenant-scoped relations pointing at legacy memos.
6. Expand tests with gRPC status-code assertions, admin behavior, and relation conversion coverage.

## Revised Verdict

The plan is not approved as written. It has the right architectural direction and the right functional scope, but the implementer would hit compile errors and potentially regress unscoped admin behavior. After fixing those issues, it should be close to approve-with-nits.
