# Adversarial Code Review — plan3.md

**Reviewer:** opencode  
**Date:** 2026-07-06  
**Input:** `plan3.md`, current working tree  
**Status:** Approved

---

## Executive Summary

plan3.md accurately describes the remaining critical gap from `coding2_review.md`. However, the implementation described in plan3.md has **already been applied** to the working tree. The review below verifies the existing implementation against the plan's intent.

---

## Plan3.md Assessment

### Diagnosis: ACCURATE

The plan correctly identifies that `store/db/postgres/memo_relation.go` and `store/db/mysql/memo_relation.go` have `tenant_id` in the WHERE clause but not in the SELECT/Scan of `ListMemoRelations`.

### Proposed Fix: CORRECT AND MINIMAL

Both tasks in plan3.md describe the exact change needed:
- Add `tenant_id` to the SELECT column list
- Add `&memoRelation.TenantID` to the Scan args

This is the minimal safe fix. No logic changes, no schema changes, no new queries.

### Verification Steps: ADEQUATE

The `go build ./...`, `go test ./store/... -count=1`, `go vet ./...` commands are appropriate for this change.

---

## Implementation Verification (Current Working Tree)

Both fixes are **already present** in the working tree:

**Postgres** (`store/db/postgres/memo_relation.go`):
- Line 79-86: SELECT includes `tenant_id`
- Line 95-100: Scan includes `&memoRelation.TenantID`

**MySQL** (`store/db/mysql/memo_relation.go`):
- Line 70: SELECT includes `` `tenant_id` ``
- Line 79-84: Scan includes `&memoRelation.TenantID`

The implementation matches plan3.md's "Target Code" exactly.

---

## Downstream Impact Analysis

The missing `tenant_id` projection affects `convertMemoRelationFromStore` in `memo_relation_service.go:138-157`. That function:
```go
findMemo := &store.FindMemo{ID: &memoRelation.MemoID}
if memoRelation.TenantID != nil {
    findMemo.TenantID = memoRelation.TenantID
}
```

When `TenantID` is nil (pre-fix behavior on Postgres/MySQL), `GetMemo` is called without tenant scoping. Since `ListMemos` in both drivers only filters by tenant when `find.TenantID != nil`, this falls back to an unscoped lookup.

With the fix applied, `memoRelation.TenantID` is now populated, and `GetMemo` is correctly scoped to the tenant.

**Note:** This fallback behavior is not a direct data leak because memo IDs use `shortuuid` (high-entropy, effectively unique), but it does violate the tenant isolation invariant and could expose data in edge cases (e.g., ID collision, database corruption, or future changes to ID generation).

---

## Other Findings

### Advisory — Stale plan status

`plan3.md` line 5 says `Status: Ready for implementation`, but the changes are already in the working tree. The status should be updated to `Implemented` or `Committed`.

### Low — Test coverage gap

`store/test/memo_relation_test.go` does not assert on `TenantID` after `ListMemoRelations` or `UpsertMemoRelation`. The fix is correct, but there is no regression test that would catch this bug reoccurring on a new driver.

### Advisory — `plan2_imp_review.md` overstatement persists

The `plan2_imp_review.md` claimed that Postgres/MySQL drivers "completely ignore TenantID in DeleteMemoRelation, UpsertMemoRelation, and ListMemoRelations." The current code shows that `DeleteMemoRelation` and `UpsertMemoRelation` were already fixed during plan2 implementation. Only `ListMemoRelations` was missing. The review doc should be corrected to avoid confusion in future audits.

---

## Verdict

**APPROVED**

The fix described in plan3.md is correct, minimal, and already correctly implemented in the working tree. No additional changes needed.

### Recommended follow-ups (non-blocking)
| # | Action |
|---|--------|
| 1 | Update `plan3.md` status from `Ready for implementation` to `Implemented` |
| 2 | Add test assertion in `store/test/memo_relation_test.go` verifying `TenantID` is populated after `ListMemoRelations` |
| 3 | Correct `plan2_imp_review.md` scope of the Postgres/MySQL driver issue |
