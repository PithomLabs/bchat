# Plan: Fix gotest_fail.md — Test Failure After delivery_status CHECK Fix

## Status
- **Date**: 2026-07-09
- **Requester**: User
- **State**: Draft — awaiting approval

---

## Problem

After Step 5 of plan2 (expanding the `delivery_status` CHECK constraint), `go test ./...` produces **one failure**:

```
--- FAIL: TestCreateBridgeHandoffReplyIfActiveDeliveryStatusConstraint (0.05s)
    bridge_test.go:578:
        Error Trace:    bridge_test.go:578
        Error:          An error is expected but got nil.
        Test:           TestCreateBridgeHandoffReplyIfActiveDeliveryStatusConstraint
```

All other tests pass.

---

## Root Cause

### What we changed (Step 5 of plan2)

In both `store/migration/postgres/LATEST.sql:809` and `store/migration/sqlite/LATEST.sql:868`:

```sql
-- BEFORE (too restrictive, blocked legitimate 'delivered' state):
delivery_status TEXT NOT NULL DEFAULT 'not_delivered' CHECK(delivery_status = 'not_delivered')

-- AFTER (correct business logic):
delivery_status TEXT NOT NULL DEFAULT 'not_delivered' CHECK(delivery_status IN ('not_delivered', 'delivered', 'failed'))
```

### Why the test breaks

`TestCreateBridgeHandoffReplyIfActiveDeliveryStatusConstraint` (`store/test/bridge_test.go:567-580`) does this:

```go
_, err := ts.GetDriver().GetDB().ExecContext(ctx, `
    INSERT INTO bridge_handoff_replies (...)
    VALUES (..., 'delivered', ...)
`)
require.Error(t, err)    // ← expects constraint violation
require.Contains(t, err.Error(), "constraint failed")
```

The test inserts `delivery_status = 'delivered'` and **expects** a constraint error. Before our fix, `'delivered'` violated the old `CHECK(delivery_status = 'not_delivered')`. After our fix, `'delivered'` is valid, so the insert succeeds and `require.Error(t, err)` fails.

**The test is testing the old, wrong constraint — not our fix.**

---

## Fix

### Change 1: Update the test value

**File**: `store/test/bridge_test.go:575`

```go
// BEFORE:
VALUES ('reply-fail', ?, ?, ?, ?, 'msg-fail', 'some text', 'delivered', ?)

// AFTER:
VALUES ('reply-fail', ?, ?, ?, ?, 'msg-fail', 'some text', 'bogus_status', ?)
```

Using `'bogus_status'` — a value not in any CHECK constraint — ensures the test still validates that the CHECK constraint works, regardless of what values are allowed.

### Change 2: Update the assertion message (optional, recommended)

```go
// BEFORE:
require.Contains(t, err.Error(), "constraint failed")

// AFTER (no change needed — Postgres and SQLite both include "constraint" in the error message)
```

No change needed — both Postgres and SQLite error messages contain "constraint" when a CHECK is violated.

---

## Why this is correct

| Aspect | Detail |
|--------|--------|
| **Test intent** | Verify that `delivery_status` CHECK constraint rejects invalid values |
| **Old behavior** | `'delivered'` was invalid (too restrictive) |
| **New behavior** | `'delivered'` is valid (correct business logic) |
| **Fix** | Use a truly invalid value (`'bogus_status'`) that the constraint still rejects |
| **Test still validates CHECK** | Yes — `'bogus_status'` is not in `('not_delivered', 'delivered', 'failed')` |
| **Production correctness** | The CHECK constraint works as designed |

---

## Files Modified

| File | Change |
|------|--------|
| `store/test/bridge_test.go:575` | Change `'delivered'` → `'bogus_status'` in test INSERT |

---

## Verification

1. `go test ./store/test/... -run TestCreateBridgeHandoffReplyIfActiveDeliveryStatusConstraint -v`
   - Must pass (error expected, error received)
2. `go test ./...`
   - Must produce 0 failures
3. Manual review: Confirm `'bogus_status'` is NOT in the CHECK constraint's allowed list
