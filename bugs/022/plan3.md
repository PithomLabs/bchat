# Implementation Plan 3 — Postgres/MySQL ListMemoRelations Fix

**Date:** 2026-07-06
**Source:** `bugs/022/coding2_review.md`
**Status:** Ready for implementation

---

## Background

The `coding2_review.md` adversarial review verified all 19 plan2 fixes. It found one remaining critical gap and two tracked nits.

### What's Already Fixed (Confirmed)

| Function | SQLite | Postgres | MySQL |
|----------|--------|----------|-------|
| `UpsertMemoRelation` — tenant_id in INSERT/UPSERT | ✅ | ✅ | ✅ |
| `ListMemoRelations` — tenant_id in WHERE clause | ✅ | ✅ | ✅ |
| `ListMemoRelations` — tenant_id in SELECT/Scan | ✅ | ❌ **MISSING** | ❌ **MISSING** |
| `DeleteMemoRelation` — tenant_id in WHERE clause | ✅ | ✅ | ✅ |

The WHERE clause filtering works correctly on all three drivers — tenant isolation is enforced at query time. The bug is that the `tenant_id` column is not returned in the result set, so `MemoRelation.TenantID` is always `nil` on Postgres/MySQL. Downstream code that reads `TenantID` from the returned struct (e.g., `convertMemoRelationFromStore` at `memo_relation_service.go:134`) gets `nil`, causing it to skip tenant-scoped memo lookups.

---

## Task 1: Fix Postgres `ListMemoRelations` SELECT/Scan

**File:** `store/db/postgres/memo_relation.go`

### Current Code (lines 79-98)

```go
rows, err := d.db.QueryContext(ctx, `
    SELECT
        memo_id,
        related_memo_id,
        type
    FROM memo_relation
    WHERE `+strings.Join(where, " AND "), args...)
if err != nil {
    return nil, err
}
defer rows.Close()

list := []*store.MemoRelation{}
for rows.Next() {
    memoRelation := &store.MemoRelation{}
    if err := rows.Scan(
        &memoRelation.MemoID,
        &memoRelation.RelatedMemoID,
        &memoRelation.Type,
    ); err != nil {
        return nil, err
    }
    list = append(list, memoRelation)
}
```

### Required Change

1. **SELECT**: Add `tenant_id` column to the SELECT list
2. **Scan**: Add `&memoRelation.TenantID` to the Scan args

### Target Code

```go
rows, err := d.db.QueryContext(ctx, `
    SELECT
        memo_id,
        related_memo_id,
        type,
        tenant_id
    FROM memo_relation
    WHERE `+strings.Join(where, " AND "), args...)
if err != nil {
    return nil, err
}
defer rows.Close()

list := []*store.MemoRelation{}
for rows.Next() {
    memoRelation := &store.MemoRelation{}
    if err := rows.Scan(
        &memoRelation.MemoID,
        &memoRelation.RelatedMemoID,
        &memoRelation.Type,
        &memoRelation.TenantID,
    ); err != nil {
        return nil, err
    }
    list = append(list, memoRelation)
}
```

### Verification

- The `MemoRelation` struct at `store/memo_relation.go` already has `TenantID *int32` — no struct change needed.
- The `tenant_id` column was added to `memo_relation` in migration `0.28/00__add_tenant_to_memo_relation.sql` — the column exists in the schema.
- The WHERE clause already filters on `tenant_id` (lines 55-57) — this is already correct.

---

## Task 2: Fix MySQL `ListMemoRelations` SELECT/Scan

**File:** `store/db/mysql/memo_relation.go`

### Current Code (lines 70-84)

```go
rows, err := d.db.QueryContext(ctx, "SELECT `memo_id`, `related_memo_id`, `type` FROM `memo_relation` WHERE "+strings.Join(where, " AND "), args...)
if err != nil {
    return nil, err
}
defer rows.Close()

list := []*store.MemoRelation{}
for rows.Next() {
    memoRelation := &store.MemoRelation{}
    if err := rows.Scan(
        &memoRelation.MemoID,
        &memoRelation.RelatedMemoID,
        &memoRelation.Type,
    ); err != nil {
        return nil, err
    }
    list = append(list, memoRelation)
}
```

### Required Change

1. **SELECT**: Add `` `tenant_id` `` to the SELECT list
2. **Scan**: Add `&memoRelation.TenantID` to the Scan args

### Target Code

```go
rows, err := d.db.QueryContext(ctx, "SELECT `memo_id`, `related_memo_id`, `type`, `tenant_id` FROM `memo_relation` WHERE "+strings.Join(where, " AND "), args...)
if err != nil {
    return nil, err
}
defer rows.Close()

list := []*store.MemoRelation{}
for rows.Next() {
    memoRelation := &store.MemoRelation{}
    if err := rows.Scan(
        &memoRelation.MemoID,
        &memoRelation.RelatedMemoID,
        &memoRelation.Type,
        &memoRelation.TenantID,
    ); err != nil {
        return nil, err
    }
    list = append(list, memoRelation)
}
```

### Verification

- Same as Postgres: struct already has `TenantID`, column exists in schema, WHERE clause already filters.
- MySQL uses backtick quoting for identifiers and `?` placeholders — consistent with existing style.

---

## Task 3 (Deferred): Expose `allowed_tenant_ids` in UpdateMask

**Status:** DEFERRED — requires proto changes first.

The `allowed_tenant_ids` field does not exist in the gRPC proto definition (`*.proto` files). The store layer supports it (`store/user.go:79`), but there is no API surface to update it after user creation.

**To implement later:**
1. Add `repeated string allowed_tenant_ids = N;` to the `UpdateUserRequest.User` message in the proto
2. Regenerate protobuf Go code
3. Add the `else if field == "allowed_tenant_ids"` case in `user_service.go:222`

This is a feature addition, not a security fix. Defer to a separate task.

---

## Files Modified

| File | Change |
|------|--------|
| `store/db/postgres/memo_relation.go` | Add `tenant_id` to SELECT and Scan in `ListMemoRelations` |
| `store/db/mysql/memo_relation.go` | Add `` `tenant_id` `` to SELECT and Scan in `ListMemoRelations` |

## Verification Commands

```bash
go build ./...
go test ./store/... -count=1
go vet ./...
```

---

## Risk Assessment

- **Low risk**: Only adding a column to an existing SELECT statement. No logic changes, no new queries, no schema changes.
- **Backward compatible**: The `tenant_id` column already exists in the table. Adding it to SELECT is a no-op for existing code that doesn't read it.
- **No SQL injection**: Both drivers use parameterized queries. The `tenant_id` value in WHERE is already parameterized.
