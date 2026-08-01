# Adversarial Plan Review: Bug 056 — Plan 5 `parent memo not found`

**Bug/Task:** Review of `bugs/056/plan5_bug.md`  
**Reviewer:** Codex, senior Go architect  
**Date:** 2026-08-01  
**Verdict:** REWORK REQUIRED

## Executive Summary

`plan5_bug.md` is much closer than the prior plans. It fixes the Plan 4 compile issues, restores the existing enum conversion helpers, preserves snippet truncation in `convertMemoRelationFromStore`, and adds request-tenant scoping for relation delete/list operations.

The remaining blocker is the proposed `MemoIsAccessible` helper. It is not implementable as written because it calls scoped-admin resolution with a nil store, and it still does not fully match `GetMemo` semantics for scoped admins. There are also a few smaller inconsistencies between planned behavior and tests.

| # | Finding | Severity | Required Action |
|---|---------|----------|-----------------|
| 1 | `MemoIsAccessible` can panic by calling scoped-admin resolver with nil store | HIGH | Pass `ctx` and `*store.Store`, or avoid resolver |
| 2 | `MemoIsAccessible` does not match `GetMemo` scoped-admin behavior | HIGH | Match fully or document stricter behavior |
| 3 | Planned error behavior conflicts with scoped-admin test expectation | MEDIUM | Return/status-plan must be consistent |
| 4 | `RelationTenantID` can still produce unscoped relation mutation | MEDIUM | Restrict or document true unscoped callers |
| 5 | `ListMemoComments` legacy comment behavior is under-specified | MEDIUM | Document exclusion or add compatibility |
| 6 | Test plan is broad enough to sprawl | LOW | Prioritize high-risk cases |

## Findings

### 1. `MemoIsAccessible` Can Panic

**Severity:** HIGH

The proposed helper calls:

```go
allowedTenantIDs := deriveTenantIDsForScopedAdmin(context.Background(), nil, user)
```

That is not safe. `deriveTenantIDsForScopedAdmin` resolves scoped-admin GUIDs by calling:

```go
tenant, err := s.GetAgentTenant(ctx, &store.FindAgentTenant{GUID: &guid})
```

If `user.Role == store.RoleAdmin` and `len(user.AllowedTenantIDs) > 0`, passing `nil` for `s *store.Store` can panic.

Fix the helper signature so it has the dependencies needed to mirror `GetMemo`:

```go
func MemoIsAccessible(ctx context.Context, s *store.Store, memo *store.Memo, user *store.User, tenantID *int32) bool
```

Then call:

```go
allowedTenantIDs := deriveTenantIDsForScopedAdmin(ctx, s, user)
```

Alternatively, do not call `deriveTenantIDsForScopedAdmin` from this helper and explicitly define stricter behavior. But the current helper is not safe.

### 2. `MemoIsAccessible` Does Not Match `GetMemo` for Scoped Admins

**Severity:** HIGH

The plan says `MemoIsAccessible` mirrors `GetMemo`, but it does not.

`GetMemo` allows scoped admins with no tenant context to access tenant-scoped memos when the memo tenant is in their derived allowed tenant list. Plan 5’s helper denies all tenant-scoped memos without an explicit tenant context unless the user is a superuser:

```go
if tenantID != nil && *memo.TenantID == *tenantID {
    return true
}
if user != nil && isSuperUser(user) {
    return true
}
return false
```

That is stricter than `GetMemo`.

The plan needs to choose one:

- Fully mirror `GetMemo`, including scoped-admin allowed-tenant checks.
- Intentionally make relation/comment APIs stricter than `GetMemo`, and update rationale/tests accordingly.

Given the stated goal of preserving existing gRPC memo API behavior, the recommended fix is to mirror `GetMemo`.

### 3. Error Behavior Conflicts With Test Expectation

**Severity:** MEDIUM

Most call sites in the plan do this:

```go
if !MemoIsAccessible(...) {
    return nil, status.Errorf(codes.NotFound, "memo not found")
}
```

But the test plan includes:

```go
// Expect: SetMemoRelations returns codes.PermissionDenied (matching GetMemo behavior)
```

Those cannot both be true. If the plan wants to avoid revealing cross-tenant existence, use `codes.NotFound` consistently and change the test. If the plan wants to mirror `GetMemo`, return `codes.PermissionDenied` for permission failures and test that.

For consistency with the existing `GetMemo` semantics, prefer returning `PermissionDenied` for resolved-but-inaccessible memos in gRPC memo APIs, while ticket-internal helpers can continue returning generic not-found errors.

### 4. `RelationTenantID` Can Still Produce Unscoped Relation Mutation

**Severity:** MEDIUM

The proposed helper is:

```go
func RelationTenantID(ctx context.Context, memo *store.Memo) *int32 {
    if requestTenantID := GetTenantIDFromContext(ctx); requestTenantID != nil {
        return requestTenantID
    }
    return memo.TenantID
}
```

For unscoped requests on legacy memos, this returns nil. In `SetMemoRelations`, that means `DeleteMemoRelation` is unscoped.

This may be acceptable for true superusers, but the plan must make that explicit and enforce it. Non-super unscoped callers should not be able to mutate relations on legacy/global memos with nil tenant scope.

If `MemoIsAccessible` is fixed to mirror `GetMemo`, this is probably safe because scoped admins without tenant context are denied for legacy memos. Add a test that proves non-super unscoped scoped-admin mutation is denied before the unscoped delete can run.

### 5. `ListMemoComments` Legacy Comment Behavior Is Under-Specified

**Severity:** MEDIUM

The plan fixes the parent memo lookup in `ListMemoComments`, then says the rest is unchanged. Existing code filters comment relations by request tenant when tenant context exists, and filters each fetched comment memo by the same tenant.

That means a tenant-scoped relation pointing to a nil-tenant legacy comment memo will be excluded. That may be the safer default, but the plan should document it like it does for nil-tenant relations in `getTicketComments`.

Either:

- explicitly state nil-tenant legacy comment memos are excluded from tenant-scoped `ListMemoComments`; or
- add compatibility handling and tests if they should be visible.

For tenant isolation, exclusion is the safer default.

### 6. Test Plan Should Prioritize High-Risk Cases

**Severity:** LOW

Twenty tests is broad enough that implementation may sprawl. The plan should prioritize the cases that prove the design:

- original ticket regression: nil-tenant parent memo succeeds;
- cross-tenant parent memo rejection;
- tenant-scoped relation delete/list isolation for legacy source memos;
- scoped-admin no-tenant behavior;
- superuser no-tenant behavior;
- converter handles tenant-scoped relation with legacy endpoint and preserves snippet truncation.

The remaining edge cases are useful but secondary.

## Required Plan Rework

Before implementation, update `plan5_bug.md` to:

1. Change `MemoIsAccessible` to accept `ctx context.Context` and `s *store.Store`, or remove scoped-admin resolver usage from it.
2. Decide whether gRPC relation/comment APIs fully mirror `GetMemo`; if yes, implement scoped-admin allowed-tenant behavior and permission-denied semantics.
3. Make status-code expectations consistent across plan text and tests.
4. Explicitly state when unscoped `RelationTenantID == nil` is allowed and prove non-super unscoped callers cannot mutate legacy relations.
5. Document `ListMemoComments` behavior for nil-tenant legacy comment memos under tenant-scoped requests.
6. Trim or prioritize the test plan around the high-risk access-control and relation-isolation cases.

## Revised Verdict

The plan is not approved as written. It is one solid revision away: fix `MemoIsAccessible` so it is safe and semantically aligned with `GetMemo`, then align the status codes and tests with that decision.
