# Adversarial Plan Review: Bug 056 — Plan 4 `parent memo not found`

**Bug/Task:** Review of `bugs/056/plan4_bug.md`  
**Reviewer:** Codex, senior Go architect  
**Date:** 2026-08-01  
**Verdict:** REWORK REQUIRED

## Executive Summary

`plan4_bug.md` fixes several prior issues: it splits ticket-internal tenant checks from user-aware gRPC checks, corrects the earlier proto field mistakes, documents the `CreateMemoComment` post-create lookup, and includes relation converter coverage.

The plan is still not ready. The main remaining problems are in relation handling: the proposed gRPC snippets use the wrong user lookup helper, relation delete/read scoping can leak or delete cross-tenant relations for legacy memos, enum conversion snippets are wrong, and the relation converter replacement changes response semantics by returning full memo content as snippets.

| # | Finding | Severity | Required Action |
|---|---------|----------|-----------------|
| 1 | `getUserFromContext(ctx)` will not compile for gRPC contexts | HIGH | Use `s.GetCurrentUser(ctx)` |
| 2 | `SetMemoRelations` can delete cross-tenant relations for legacy source memos | HIGH | Scope deletes by request tenant when present |
| 3 | `ListMemoRelations` can expose cross-tenant relations for legacy source memos | HIGH | Scope relation reads by request tenant when present |
| 4 | Proposed enum conversions are invalid | HIGH | Use existing conversion helpers |
| 5 | Converter replacement leaks full memo content as snippets | MEDIUM | Preserve snippet behavior |
| 6 | `MemoIsAccessible` is weaker than `GetMemo` for scoped admins | MEDIUM | Match scoped-admin behavior or narrow helper usage |
| 7 | Tests need relation isolation coverage | MEDIUM | Add targeted tests |

## Findings

### 1. `getUserFromContext(ctx)` Will Not Compile

**Severity:** HIGH

The plan uses:

```go
user := getUserFromContext(ctx)
```

inside gRPC handlers such as `CreateMemoComment`, `SetMemoRelations`, `ListMemoRelations`, and `ListMemoComments`.

That does not compile. `getUserFromContext` is defined for Echo contexts:

```go
func getUserFromContext(c echo.Context) *store.User
```

For gRPC `context.Context`, use:

```go
user, err := s.GetCurrentUser(ctx)
if err != nil {
    return nil, status.Errorf(codes.Internal, "failed to get user")
}
```

Then pass `user` to `MemoIsAccessible`.

### 2. `SetMemoRelations` Can Delete Cross-Tenant Relations for Legacy Source Memos

**Severity:** HIGH

The current code deletes existing reference relations with:

```go
TenantID: memo.TenantID,
```

The plan leaves this behavior effectively unchanged. For a legacy source memo where `memo.TenantID == nil`, this becomes an unscoped delete. If multiple tenants have relations against the same legacy memo, a tenant-scoped `SetMemoRelations` call can delete relations belonging to other tenants.

The delete scope should use the request tenant when present:

```go
relationTenantID := GetTenantIDFromContext(ctx)
if relationTenantID == nil {
    relationTenantID = memo.TenantID
}
```

Use `relationTenantID` for both `DeleteMemoRelation` and `UpsertMemoRelation`. This preserves tenant isolation for tenant-scoped relation writes while keeping legacy/unscoped behavior for truly unscoped requests.

### 3. `ListMemoRelations` Can Expose Cross-Tenant Relations for Legacy Source Memos

**Severity:** HIGH

The plan says the rest of `ListMemoRelations` remains unchanged after the source memo lookup. Existing relation queries use:

```go
TenantID: memo.TenantID,
```

For a legacy memo, `memo.TenantID == nil`, so the relation query has no tenant filter and can return relations across tenants. This undermines the plan’s tenant isolation goal.

For tenant-scoped requests against a legacy memo, relation reads should use request tenant context:

```go
relationTenantID := GetTenantIDFromContext(ctx)
if relationTenantID == nil {
    relationTenantID = memo.TenantID
}
```

Use that for both outgoing and incoming relation queries. Tests must cover a legacy source memo with tenant 19 and tenant 20 relations and assert a tenant 19 request only sees tenant 19 relations.

### 4. Proposed Enum Conversions Are Invalid

**Severity:** HIGH

The plan proposes:

```go
Type: store.MemoRelationType(relation.Type),
```

and:

```go
Type: v1pb.MemoRelation_Type(memoRelation.Type),
```

These are not correct. The store relation type is string-like, while the proto relation type is an enum. The codebase already has the correct conversion helpers:

```go
convertMemoRelationTypeToStore(relation.Type)
convertMemoRelationTypeFromStore(memoRelation.Type)
```

The plan should explicitly keep those helpers.

### 5. Converter Replacement Leaks Full Memo Content as Snippets

**Severity:** MEDIUM

The proposed `convertMemoRelationFromStore` replacement returns:

```go
Snippet: memo.Content,
```

The existing converter uses:

```go
memoSnippet, err := getMemoContentSnippet(memo.Content)
```

Returning full content changes API response semantics and can expose more memo content than intended. The converter should keep `getMemoContentSnippet` for both memo endpoints.

### 6. `MemoIsAccessible` Is Weaker Than `GetMemo` for Scoped Admins

**Severity:** MEDIUM

The proposed `MemoIsAccessible` allows any nil-tenant legacy memo:

```go
if memo.TenantID == nil {
    return true
}
```

But `GetMemo` has extra scoped-admin logic: scoped admins with no tenant context are denied access to legacy nil-tenant memos unless they are superusers.

If `MemoIsAccessible` is intended to match `GetMemo`, it needs scoped-admin awareness via `deriveTenantIDsForScopedAdmin`. If it is only intended for contexts where `tenantID` is present, the plan should say that and tests should reflect it.

Recommended rule:

- ticket-internal helper remains tenant-only;
- gRPC helper should mirror `GetMemo` as closely as practical, including superuser bypass and scoped-admin restrictions.

### 7. Tests Need Relation Isolation Coverage

**Severity:** MEDIUM

The test list is broad, but it needs two specific relation isolation cases:

1. `SetMemoRelations` with a legacy source memo and tenant-scoped request must delete only that request tenant’s reference relations, not relations for other tenants.
2. `ListMemoRelations` with a legacy source memo and tenant-scoped request must return only that request tenant’s relations, not relations for other tenants.

Also keep the converter test, but assert snippets are truncated/processed through `getMemoContentSnippet`, not full content.

## Required Plan Rework

Before implementation, update `plan4_bug.md` to:

1. Replace all gRPC `getUserFromContext(ctx)` calls with `s.GetCurrentUser(ctx)` and error handling.
2. Define a `relationTenantID` rule for relation deletes, upserts, and list queries: request tenant first, then memo tenant for unscoped contexts.
3. Preserve existing enum conversion helpers.
4. Preserve `getMemoContentSnippet` in `convertMemoRelationFromStore`.
5. Make `MemoIsAccessible` match `GetMemo` scoped-admin behavior or explicitly constrain where it can be used.
6. Add tests for tenant-scoped relation delete/read isolation on legacy source memos.

## Revised Verdict

The plan is not approved as written. It is close on the original ticket bug, but the expanded relation handling still has correctness and tenant-isolation gaps. After fixing the gRPC user lookup and relation scoping rules, it should be near approve-with-nits.
