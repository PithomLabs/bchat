# Adversarial Plan Review: Bug 056 — `parent memo not found` on Manual Ticket Creation

**Bug/Task:** Review of `bugs/056/plan_bug.md`  
**Reviewer:** Codex, senior Go architect  
**Date:** 2026-08-01  
**Verdict:** REWORK REQUIRED

## Executive Summary

The plan correctly identifies the immediate failure: `createSystemResolutionComment` and `getTicketComments` look up a parent memo by globally unique UID while also filtering by `tenant_id`, so legacy or unscoped memos with `tenant_id=NULL` are missed. It also correctly identifies the `%!w(<nil>)` formatting bug.

However, the proposed fix is too permissive. Removing the `TenantID` filter without replacing it with an explicit ownership check weakens tenant isolation. A ticket in tenant A could reference a memo UID from tenant B and the helper would create or read comment relations against that memo. The codebase’s safer pattern is: resolve by UID, then enforce access/ownership in Go.

The bug plan should be expanded before implementation.

| # | Finding | Severity | Required Action |
|---|---------|----------|-----------------|
| 1 | UID-only lookup needs an explicit tenant ownership check | HIGH | Rework |
| 2 | `getTicketComments` must filter relations by ticket tenant | HIGH | Rework |
| 3 | Related `CreateMemoComment` UID+tenant lookup has the same legacy-NULL failure mode | MEDIUM | Expand plan |
| 4 | Empty `/m/` descriptions should short-circuit cleanly | MEDIUM | Add guard |
| 5 | Error handling fix is correct but should avoid noisy UID disclosure | LOW | Adjust wording |
| 6 | Verification needs focused regression tests, not only existing tests | HIGH | Add tests |

## Findings

### 1. UID-only Lookup Needs an Ownership Check

**Severity:** HIGH

The plan changes:

```go
parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
```

That is the right first step because `memo.uid` is globally unique across SQLite, Postgres, and MySQL schemas. But the plan stops there.

After resolving the memo by UID, the ticket helpers must enforce:

- allow `parentMemo.TenantID == nil` for legacy/unscoped memos;
- allow `parentMemo.TenantID != nil && *parentMemo.TenantID == tenantID`;
- reject `parentMemo.TenantID != nil && *parentMemo.TenantID != tenantID`.

Without that check, the more permissive lookup can cross tenant boundaries. The ticket description containing `/m/<uid>` is not sufficient authorization by itself because a UID can be copied or guessed from logs, URLs, exports, or UI state.

### 2. `getTicketComments` Must Filter Relations by Tenant

**Severity:** HIGH

The plan removes the tenant filter from the parent memo lookup but does not add a tenant filter to the relation query:

```go
relations, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
    RelatedMemoID: &parentMemo.ID,
    Type:          &commentType,
})
```

That is unsafe when the parent memo is legacy/global (`tenant_id=NULL`) and multiple tenant tickets reference it. `createSystemResolutionComment` writes comment relations with `TenantID: &tenantID`, so `getTicketComments` should read relations with the same tenant:

```go
TenantID: ticket.TenantID,
```

This keeps updated-ticket re-indexing from mixing comments across tenants.

### 3. Expand Plan to Cover `CreateMemoComment`

**Severity:** MEDIUM

`server/router/api/v1/memo_service.go` has a related pattern:

```go
findMemo := &store.FindMemo{UID: &memoUID}
if tenantID := GetTenantIDFromContext(ctx); tenantID != nil {
    findMemo.TenantID = tenantID
}
relatedMemo, err := s.Store.GetMemo(ctx, findMemo)
```

This can also fail to comment on a legacy `tenant_id=NULL` memo from a tenant-scoped context. The bug plan should be expanded to include this path, using the same pattern:

- resolve target memo by UID only;
- allow nil-tenant legacy memo or same-tenant memo;
- reject mismatched non-nil tenant memo;
- keep newly created comment memo tenant-scoped through `CreateMemo`;
- keep `MemoRelation.TenantID` set from context.

This is not required to fix the exact logged `createSystemResolutionComment` failure, but it is the same class of bug and should be handled while the ownership rule is being standardized.

### 4. Empty `/m/` UID Should Short-Circuit

**Severity:** MEDIUM

Both ticket helpers use:

```go
memoUID := strings.TrimPrefix(ticket.Description, "/m/")
```

They should treat `memoUID == ""` as no valid memo link and return nil. Otherwise `/m/` produces a needless store lookup and, for `createSystemResolutionComment`, a misleading parent-not-found error.

This also matches the defensive guard already present in `findExistingEscalationTicket`.

### 5. Error Handling Fix Is Directionally Correct

**Severity:** LOW

Splitting the error cases is correct:

```go
if err != nil {
    return fmt.Errorf("parent memo lookup failed: %w", err)
}
if parentMemo == nil {
    return fmt.Errorf("parent memo not found")
}
```

Avoid formatting `%w` with nil. Prefer not to include the full memo UID in the error unless needed for debug logs, because memo UIDs are externally meaningful resource identifiers.

### 6. Existing Tests Are Not Enough

**Severity:** HIGH

The plan’s verification runs broad existing tests, but this bug needs direct regression coverage. Add focused tests in `server/router/api/v1/ticket_service_test.go` or a nearby service test file:

1. `createSystemResolutionComment` succeeds when the ticket has `tenant_id=19` and the parent memo has `tenant_id=NULL`.
2. `createSystemResolutionComment` refuses a parent memo with non-nil mismatched tenant ID.
3. `getTicketComments` returns only relations matching the ticket tenant when the parent memo is legacy/global.
4. `getTicketComments` still returns nil, nil for no `/m/` prefix, empty `/m/`, missing memo, and deleted/archived parent behavior as currently expected.
5. `CreateMemoComment` can attach a tenant-scoped comment to a legacy nil-tenant parent memo, while rejecting mismatched non-nil tenant parents.

## Required Plan Rework

Update `bugs/056/plan_bug.md` before implementation with these changes:

1. Replace tenant-filtered UID lookups in ticket helpers with UID-only lookup plus explicit tenant compatibility validation.
2. Add a small shared helper if it keeps the rule consistent, for example `memoBelongsToTenantOrLegacy(memo *store.Memo, tenantID *int32) bool`.
3. Add `memoUID == ""` guards after trimming `/m/`.
4. Add `TenantID: ticket.TenantID` to `ListMemoRelations` in `getTicketComments`.
5. Expand the scope to fix `CreateMemoComment` with the same UID-lookup and ownership rule.
6. Add targeted tests for legacy nil-tenant memo compatibility and cross-tenant rejection.

## Revised Verdict

The plan is not approved as written. The root cause is real and the proposed direction is close, but the implementation must preserve tenant isolation explicitly after the UID-only lookup. Once those ownership checks and regression tests are added, this should become an approve-with-nits plan.
