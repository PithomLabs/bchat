# Code 4 Review: Address Code 3 Review Findings

**Status:** APPROVED with nits
**Reviewer:** AI Agent
**Date:** 2026-07-27
**Files examined:**
- `bugs/049/code4.md` (plan, 131 lines)
- `server/.../agent/observer_longmemeval_test.go:668-824` (current implementation)
- `bugs/049/plan5.md:59-68` (threshold status)

---

## Verdict: Approved with Nits

All 5 actionable findings have correct, well-specified fixes. The plan5.md revision history (threshold=1 vs 200) is accurately described. Two minor inconsistencies remain.

---

## Validated Claims

| Code3 Finding | Status | Verified |
|---------------|--------|----------|
| #1 Threshold divergence | Plan5 already updated to 1 | ✓ `plan5.md:59-68` now specifies threshold=1 |
| #2 Token reduction | Action specified | ✓ Fix captures `originalTokens` before reset, compares after |
| #3 Pass 2 abstention | Action specified | ✓ Add `Contains("10-gallon")` to pass 2 |
| #4 Log message | Action specified | ✓ Rename "Observer pass" → "Passed" |
| #5 detailForType default | Action specified | ✓ Guard with `t.Fatalf` for unknown types |
| #6-#9 Minor items | Resolved/acceptable | ✓ All correct |

### Plan5.md Revision Confirmed

`plan5.md:59-68` has been updated since code3 was implemented. It now reads:

> Use threshold=1 with `setOMEnvAndReload` (not bare `ReloadOMConfig`). Threshold=1 guarantees the reflector fires on any observation, which is necessary for the two-pass test pattern.

The header still says `OM_TOKEN_THRESHOLD=1 unrealistic` — cosmetic, flagged below.

---

## Nits (2)

### 1. plan5.md Finding #4 header still says "unrealistic"

`plan5.md:59`:
```
#### #4. `OM_TOKEN_THRESHOLD=1` unrealistic — **ACTION REQUIRED**
```

The body text was updated to justify threshold=1, but the header still calls it "unrealistic." If someone reads the header only, they'll be confused about the plan's direction.

**Fix:** Update the header to match the body:
```
#### #4. OM_TOKEN_THRESHOLD lowered to 1 for two-pass test — **ACTION REQUIRED**
```

### 2. Run command regex captures `TestEndToEndObserver_ResourceScope`

Line 130:
```
-run "TestEndToEndObserver|TestReflector_..."
```

The prefix `TestEndToEndObserver` matches both `TestEndToEndObserver_DetailPreservation` and `TestEndToEndObserver_ResourceScope`. This is **harmless** (both are valid Tier 1 tests), but the plan should note this to prevent confusion about why an unlisted test runs.

**Fix (optional):** Specify exact names:
```
-run "TestEndToEndObserver_DetailPreservation|TestEndToEndObserver_ResourceScope|TestReflector_..."
```

Or add a note: "The `TestEndToEndObserver` prefix matches both Tier 1 subtests, which is correct."

---

## Implementation Notes

All 5 fixes have clean insertion points that don't conflict with each other:

| Fix | Insertion Point | Method |
|-----|----------------|--------|
| Threshold comment | Line 769 | Add comment before `setOMEnvAndReload` |
| Token comparison | Lines 766-767 (between `observerPassed++` and reset section) | Record before reset, assert after pass 2 assertions |
| Abstention Contains | Line 792 | Add after existing `NotContains` |
| Log label | Line 815 | Replace "Observer pass" with "Passed" |
| detailForType guard | Line 736 | Insert between `detail := detailForType(...)` and `if q.QuestionType == "abstention"` |

No overlap or merge conflicts between these changes.

---

## Summary

| Severity | Count | Items |
|----------|-------|-------|
| 🔴 Critical | 0 | — |
| 🟡 Significant | 0 | — |
| 🟢 Minor | 2 | Stale header in plan5.md, run command note |
