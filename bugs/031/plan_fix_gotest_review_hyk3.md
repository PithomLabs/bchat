# Adversarial Plan Review: `bugs/031/plan_fix_gotest.md`

**Reviewer:** hyk3
**Input:** `bugs/031/plan_fix_gotest.md` (test fix for `TestCreateBridgeHandoffReplyIfActiveDeliveryStatusConstraint` after the `delivery_status` CHECK expansion)
**Goal:** Minimum-viable, self-contained plan a coding agent can execute without another review round.

---

## Verdict

The plan's **core fix (Change 1) is correct and necessary**, but **Change 2 is wrong** and must be corrected before hand-off. One required fix, plus a related advisory.

---

## What's Correct

- The schema change (Step 5 of `plan2`) **is already applied** — both `store/migration/postgres/LATEST.sql:809` and `store/migration/sqlite/LATEST.sql:868` now use `CHECK(delivery_status IN ('not_delivered', 'delivered', 'failed'))`. Git status confirms both files are modified. The test failure is real.
- `store/test/bridge_test.go:575` does insert `delivery_status='delivered'` and expects an error — the plan's description of the test is accurate.
- **Only one test is affected.** The other two `'delivered'` insertions (`bridge_settlement_test.go:414`, `bridge_test.go:799`) target `bridge_reply_outbox.status` (a separate CHECK), **not** `delivery_status` — independent of this change.
- Production code only ever sets `delivery_status="not_delivered"` (`store/db/sqlite/bridge.go:437,559`, `store/db/postgres/bridge.go:448,566`), so the expanded CHECK breaks no production insert. Safe.

---

## Issue 1 — HIGH (Postgres test runs): assertion substring is SQLite-only

Plan Change 2 states *"No change needed — both Postgres and SQLite error messages contain 'constraint'"*, but the assertion actually checks the literal substring **`"constraint failed"`** (`bridge_test.go:579`):

```go
require.Contains(t, err.Error(), "constraint failed")
```

- **SQLite** CHECK violation: `CHECK constraint failed: bridge_handoff_replies` → contains `"constraint failed"` ✅
- **PostgreSQL** CHECK violation: `new row for relation "bridge_handoff_replies" violates check constraint "bridge_handoff_replies_delivery_status_check"` → contains `"constraint"` but **NOT** `"constraint failed"` ❌

The test driver is **env-selectable** (`getDriverFromEnv()` in `store/test/store.go`), so these tests run against Postgres when `DSN`/driver env is set. On Postgres, `require.Contains(err.Error(), "constraint failed")` **fails** even after Change 1. The plan's premise is factually incorrect.

**Fix:** change the assertion to a driver-agnostic substring:
- `"constraint"` — matches SQLite `"CHECK constraint failed"` and Postgres `"violates check constraint"`.
- or `"CHECK"` — more precise; both drivers emit the word `CHECK` for CHECK violations ( FK errors say `"foreign key constraint"`, which does **not** contain `CHECK`).

So Change 2 should **modify line 579**, not skip it.

---

## Issue 2 — LOW (advisory, cross-cutting): same brittleness elsewhere

The identical `"constraint failed"` substring appears in 3 other tests that would also fail under Postgres:
- `store/test/bridge_settlement_test.go:404`
- `store/test/bridge_settlement_test.go:418`
- `store/test/bridge_test.go:803`

Out of scope for this plan, but a known, broader defect — the same fix (`"constraint"` / `"CHECK"`) applies. Flag for a follow-up.

---

## Issue 3 — Minor: Files Modified list

Should also list `store/test/bridge_test.go:579` (the assertion), not only `:575`.

---

## Revised MVP (2 edits, both in `store/test/bridge_test.go`)

### Edit 1 — line 575 (test value)
```go
// BEFORE:
) VALUES ('reply-fail', ?, ?, ?, ?, 'msg-fail', 'some text', 'delivered', ?)

// AFTER:
) VALUES ('reply-fail', ?, ?, ?, ?, 'msg-fail', 'some text', 'bogus_status', ?)
```
`bogus_status` is a valid `TEXT` not in the allowed CHECK list, so the constraint still rejects it — the test still validates the CHECK.

### Edit 2 — line 579 (assertion)
```go
// BEFORE:
require.Contains(t, err.Error(), "constraint failed")

// AFTER (driver-agnostic):
require.Contains(t, err.Error(), "constraint")
```
(Alternatively `"CHECK"` — also driver-agnostic and more specific.)

---

## Verification
1. `go test ./store/test/... -run TestCreateBridgeHandoffReplyIfActiveDeliveryStatusConstraint -v`
   - Must pass on **SQLite** (default) **and** on **Postgres** (when `DSN` points at a Postgres DB).
2. `go test ./...`
   - Must produce 0 failures.
3. Manual check: confirm `'bogus_status'` is NOT in `('not_delivered', 'delivered', 'failed')`.

---

## Excluded
- Modifying the 3 other `"constraint failed"` assertions (Issue 2) — out of scope; separate follow-up.
- Reverting the `delivery_status` CHECK expansion — not warranted; production only uses `'not_delivered'` and the expanded set is correct business logic for future settlement states.
