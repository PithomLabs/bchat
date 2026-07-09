# Fix: Test Failure After `delivery_status` CHECK Expansion

**Date**: 2026-07-09
**Bug**: `gotest_fail.md`
**Plan**: `plan_fix_gotest.md`
**Reviews**: `plan_fix_gotest_review_hyk3.md`, `plan_fix_gotest_review_deepseek.md`

---

## Symptom

`go test ./...` failed with:

```
--- FAIL: TestCreateBridgeHandoffReplyIfActiveDeliveryStatusConstraint (0.05s)
    bridge_test.go:578:
        Error: An error is expected but got nil.
```

One failure out of the entire suite.

## Root Cause

Step 5 of `plan2` expanded the `delivery_status` CHECK constraint from:

```sql
CHECK(delivery_status = 'not_delivered')
```

to:

```sql
CHECK(delivery_status IN ('not_delivered', 'delivered', 'failed'))
```

The test `TestCreateBridgeHandoffReplyIfActiveDeliveryStatusConstraint` inserted `delivery_status = 'delivered'` and expected a constraint violation. After the fix, `'delivered'` is valid — no error — test fails.

The test was validating the old, too-restrictive constraint, not our fix.

## Changes

**File**: `store/test/bridge_test.go`

### Edit 1 — Line 575: Test value

```go
// BEFORE:
VALUES ('reply-fail', ?, ?, ?, ?, 'msg-fail', 'some text', 'delivered', ?)

// AFTER:
VALUES ('reply-fail', ?, ?, ?, ?, 'msg-fail', 'some text', 'bogus_status', ?)
```

`'bogus_status'` is not in `('not_delivered', 'delivered', 'failed')`, so the CHECK constraint still rejects it. The test continues to validate that the constraint works.

### Edit 2 — Line 579: Assertion (driver-agnostic)

```go
// BEFORE:
require.Contains(t, err.Error(), "constraint failed")

// AFTER:
require.Contains(t, err.Error(), "constraint")
```

SQLite emits `CHECK constraint failed: ...` (contains `"constraint failed"`).
Postgres emits `new row for relation "..." violates check constraint "..."` (contains `"constraint"` but NOT `"constraint failed"`).

Using `"constraint"` works on both drivers.

## Verification

- `go test ./store/test/... -run TestCreateBridgeHandoffReplyIfActiveDeliveryStatusConstraint -v` — PASS
- `go test ./...` — 0 failures

## Known Follow-Up

3 other tests have the same `"constraint failed"` substring that would fail under Postgres:
- `store/test/bridge_settlement_test.go:404`
- `store/test/bridge_settlement_test.go:418`
- `store/test/bridge_test.go:803`

Same fix applies (`"constraint failed"` → `"constraint"`). Out of scope for this change.
