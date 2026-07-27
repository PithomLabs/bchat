# Code 4 Implementation Review

**Status:** APPROVED with nits
**Reviewer:** AI Agent
**Date:** 2026-07-27
**File:** `server/.../agent/observer_longmemeval_test.go:668-850`

---

## Verdict: Approved with Nits

All 5 fixes from code4.md are correctly implemented. One ordering bug with `reflectorPassed++`.

---

## Fix Verification

| code4.md Fix | Lines | Status |
|-------------|-------|--------|
| Threshold comment | 771-773 | ✓ Accurately explains plan5_review divergence |
| Token comparison | 776-784 (record), 828-836 (assert) | ✓ Captures before reset, compares after pass 2 |
| Pass 2 abstention `Contains` | 807-808 | ✓ Added `Contains("10-gallon")` to compressed check |
| Log label | 841 | ✓ "Passed" replaces "Observer pass" |
| `detailForType` guard | 736-738 | ✓ `t.Fatalf` on unknown type |

---

## Issues Found

### 🔴 `reflectorPassed++` runs before token assertion (ordering bug)

Lines 824-826 increment `reflectorPassed` BEFORE the token compression assertion at lines 828-836:

```go
if !t.Failed() {
    reflectorPassed++          // line 824-826 — incremented HERE
}

// Assert token compression
for _, sid := range sessionIDs {
    ...
    assert.LessOrEqual(...)    // line 833 — assertion runs AFTER increment
}
```

If detail preservation passes (`t.Failed()=false`) but the token assertion fails, `reflectorPassed` is already incremented. The summary would overcount reflector passes.

Compare with `observerPassed` at line 765-768 which uses an early-return pattern — assertion failures before it prevent the increment entirely.

**Fix:** Move the token assertion block before `reflectorPassed++`:

```go
// Assert token compression (must be before reflectorPassed++)
for _, sid := range sessionIDs {
    obsLog, err := ts.GetObservationLog(ctx, sid)
    require.NoError(t, err)
    if obsLog != nil && originalTokens[sid] > 0 {
        assert.LessOrEqual(t, obsLog.TokensInLog, originalTokens[sid],
            "compressed log should be same size or smaller for session %s", sid)
    }
}

if !t.Failed() {
    reflectorPassed++
}
```

---

## Summary

| Severity | Count | Items |
|----------|-------|-------|
| 🔴 Critical | 1 | `reflectorPassed++` ordering before token assertion |
| 🟡 Significant | 0 | — |
| 🟢 Minor | 0 | — |
