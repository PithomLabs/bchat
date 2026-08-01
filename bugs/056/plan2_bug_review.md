# Adversarial Plan Review: Bug 056 — Revised `parent memo not found` Plan

**Bug/Task:** Review of `bugs/056/plan2_bug.md`  
**Reviewer:** Codex, senior Go architect  
**Date:** 2026-08-01  
**Verdict:** REWORK REQUIRED

## Executive Summary

`plan2_bug.md` is materially better than the first plan. It correctly moves from tenant-filtered UID lookup to UID-only lookup followed by explicit tenant compatibility validation. It also correctly adds the empty `/m/` guard, fixes `%!w(<nil>)`, and recognizes that `getTicketComments` needs tenant-scoped relation reads.

The remaining issue is scope consistency. The plan says it addresses the broader UID+tenant lookup class, but it only updates `CreateMemoComment` and the ticket helpers. There are still related memo relation/comment paths that apply `TenantID` directly to `FindMemo{UID: ...}` and will continue to fail for legacy `tenant_id=NULL` memos in tenant-scoped contexts.

| # | Finding | Severity | Required Action |
|---|---------|----------|-----------------|
| 1 | Expanded bug class is still incomplete | HIGH | Rework |
| 2 | Helper should accept `*int32`, not only `int32` | MEDIUM | Rework |
| 3 | Touched relation/comment handlers need nil memo checks | MEDIUM | Add |
| 4 | Relation tenant filtering needs legacy-relation test coverage | MEDIUM | Add tests |
| 5 | Helper should live in a neutral package-v1 file | LOW | Nit |

## Findings

### 1. Expanded Bug Class Is Still Incomplete

**Severity:** HIGH

The plan expands beyond the original ticket-only failure and includes `CreateMemoComment`, which is good. But the same `FindMemo{UID: ...}` plus context `TenantID` pattern still exists in related gRPC memo relation/comment paths:

- `server/router/api/v1/memo_relation_service.go`
  - `SetMemoRelations`: target memo lookup
  - `SetMemoRelations`: related memo lookup
  - `ListMemoRelations`: target memo lookup
- `server/router/api/v1/memo_service.go`
  - `ListMemoComments`: target memo lookup

These paths will still fail when a tenant-scoped request references a legacy memo whose `tenant_id` is `NULL`.

If `plan2_bug.md` claims to fix the broader UID+tenant lookup class, it must include these handlers or explicitly narrow the scope back to ticket auto-comment creation plus `CreateMemoComment`.

Required behavior for each path should match the new rule:

- resolve memo by UID only;
- allow nil-tenant legacy memo;
- allow same-tenant memo;
- reject non-nil mismatched tenant memo;
- preserve existing visibility/current-user filtering after tenant compatibility passes.

### 2. Helper Should Accept `*int32`

**Severity:** MEDIUM

The proposed helper is:

```go
func memoBelongsToTenantOrLegacy(memo *store.Memo, tenantID int32) bool
```

That works for `createSystemResolutionComment`, which receives a concrete tenant ID, but it is awkward for gRPC paths where `GetTenantIDFromContext(ctx)` returns `*int32`.

Use a pointer-aware helper:

```go
func memoBelongsToTenantOrLegacy(memo *store.Memo, tenantID *int32) bool
```

Recommended semantics:

- return false for `memo == nil`;
- return true when `memo.TenantID == nil`;
- return false when `tenantID == nil` and `memo.TenantID != nil`;
- return `*memo.TenantID == *tenantID` otherwise.

This makes nil-context behavior explicit and avoids each caller inventing its own guard.

### 3. Add Nil Checks in Relation/Comment Handlers

**Severity:** MEDIUM**

Some existing relation handlers dereference memo results without checking for nil after `GetMemo`. When changing those paths, add explicit not-found handling.

Examples:

- `SetMemoRelations` uses `memo.ID` after lookup.
- `SetMemoRelations` uses `relatedMemo.ID` after lookup.
- `ListMemoRelations` uses `memo.ID` after lookup.
- `ListMemoComments` uses `memo.ID` after lookup.

These should return `codes.NotFound` for gRPC handlers when the UID does not resolve or fails the tenant compatibility check. Avoid returning permission details that reveal whether a cross-tenant memo exists.

### 4. Relation Tenant Filtering Needs Compatibility Tests

**Severity:** MEDIUM

Adding `TenantID: ticket.TenantID` to `getTicketComments` is the right default for ticket re-indexing because `createSystemResolutionComment` writes tenant-scoped relations.

But existing data may contain `memo_relation.tenant_id=NULL`, especially from legacy relation creation or direct store tests. The plan should explicitly decide that ticket re-indexing excludes nil-tenant comment relations, or it should include a compatibility path.

For tenant isolation, exclusion is the safer default. Add tests that document it:

- legacy parent memo with tenant 19 relation is included for tenant 19;
- legacy parent memo with tenant 20 relation is excluded for tenant 19;
- legacy parent memo with nil-tenant relation is excluded for tenant 19 unless the implementation intentionally supports legacy relation fallback.

### 5. Helper Location Is Too Ticket-Specific

**Severity:** LOW

The helper is proposed in `ticket_service.go`, but the revised plan uses it from memo service code too. Put it in a neutral package-v1 file such as a small tenant/memo access helper file, or near existing tenant-context helpers, so it does not look ticket-private.

## Required Plan Rework

Before implementation, update `plan2_bug.md` to:

1. Include `SetMemoRelations`, `ListMemoRelations`, and `ListMemoComments`, or explicitly narrow the plan scope and leave them as follow-up.
2. Change `memoBelongsToTenantOrLegacy` to accept `tenantID *int32` and handle `memo == nil`.
3. Add not-found handling after every touched `GetMemo` lookup.
4. Document whether `getTicketComments` intentionally excludes nil-tenant relations and test that behavior.
5. Place the shared helper in a neutral package-v1 location.
6. Expand tests to cover legacy nil-tenant parent memos across ticket helpers, `CreateMemoComment`, memo relations, and memo comment listing if those paths are included.

## Revised Verdict

The revised plan is directionally sound but not approved as written. It fixes the original failure mode while leaving adjacent UID+tenant memo relation paths inconsistent. Once the scope and helper semantics are tightened, this should be approvable with only minor nits.
