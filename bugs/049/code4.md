# Code 4: Address Code 3 Review Findings

**Bug/Feature:** 049
**Date:** 2026-07-27
**Status:** PLAN (awaiting implementation)
**Review:** `bugs/049/code3_review.md`

---

## Summary

`bugs/049/code3_review.md` identified 9 findings in the `TestRealLLM_DetailPreservationIntegration` implementation. This plan validates each against the actual codebase and specifies changes for valid ones. **5 require action** (2 critical, 3 significant); **4 are resolved or acceptable as-is**.

---

## Validated Findings

### 🔴 Critical

#### #1. Threshold=1 divergence from plan5.md — **ACTION REQUIRED**

**Current state:** `plan5.md` already updated to threshold=1 during implementation. However, `code3.md` line 109 still says "noted in plan5.md as artificially low for testing" which is inaccurate — plan5.md originally mandated 200, then was updated to 1. The review correctly identifies that a future reader reconciling plan5.md with plan5_review.md (which says threshold=200) will be confused.

**Fix:** Add a comment at `observer_longmemeval_test.go:770` explaining the rationale:
```go
// Threshold=1 guarantees the reflector fires on small test sessions
// (~2 turns, ~60-100 tokens). Plan5_review suggested 200, but that
// would never trigger compression on these sessions.
setOMEnvAndReload(t, "OM_TOKEN_THRESHOLD", "1")
```

Also fix `code3.md` line 109 to accurately describe the divergence.

#### #2. No token reduction assertion — **ACTION REQUIRED**

**Current state:** The test validates detail preservation but never validates that the reflector actually **compresses** (reduces token count). `ObservationLog.TokensInLog` is available at `store/agent.go:691`.

**Fix:** Record token counts before reset, compare after pass 2:
```go
// Before reset in pass 2, record original token counts
originalTokens := make(map[string]int, len(sessionIDs))
for _, sid := range sessionIDs {
    obsLog, err := ts.GetObservationLog(ctx, sid)
    require.NoError(t, err)
    if obsLog != nil {
        originalTokens[sid] = obsLog.TokensInLog
    }
}
// ... reset and re-run ...
// After pass 2, assert compression
for _, sid := range sessionIDs {
    obsLog, err := ts.GetObservationLog(ctx, sid)
    require.NoError(t, err)
    if obsLog != nil && originalTokens[sid] > 0 {
        assert.LessOrEqual(t, obsLog.TokensInLog, originalTokens[sid],
            "compressed log should be same size or smaller for session %s", sid)
    }
}
```

---

### 🟡 Significant

#### #3. Pass 2 abstention drops `Contains("10-gallon")` — **ACTION REQUIRED**

**Current state:** Pass 1 (line 745) checks both `NotContains("30-gallon")` and `Contains("10-gallon")`. Pass 2 (line 791) only checks `NotContains("30-gallon")`.

**Fix:** Add `Contains("10-gallon")` to pass 2 abstention assertions:
```go
assert.Contains(t, obsLog.ObservationLog, "10-gallon",
    "compressed observations must capture actual tank size")
```

#### #4. Misleading log message — **ACTION REQUIRED**

**Current state:** Line 815 says `"Observer pass: %v"` but runs after pass 2 (reflector). If pass 1 passed but reflector failed, it prints "Observer pass: false".

**Fix:** Rename to `"Passed: %v"`:
```go
t.Logf("  Passed: %v", !t.Failed())
```

#### #5. `detailForType` default `""` produces silent pass — **ACTION REQUIRED**

**Current state:** If a new question type is added without updating `detailForType`, `strings.Contains(obsLog, "")` is always true and the test silently passes.

**Fix:** Add guard after `detailForType` call:
```go
detail := detailForType(q.QuestionType)
if detail == "" && q.QuestionType != "abstention" {
    t.Fatalf("unknown question type: %q — add to detailForType", q.QuestionType)
}
```

---

### 🟢 Minor (resolved or acceptable)

| # | Finding | Verdict | Action |
|---|---------|---------|--------|
| 6 | Inconsistent failure guard patterns | Acceptable — both work | No change |
| 7 | `assert.Contains` vs `strings.Contains` | Acceptable — current approach is fine | No change |
| 8 | code3.md inaccurate citation | Fix line 109 in code3.md | Update code3.md |
| 9 | Plan5_review nits not tracked | Already resolved — all 5 nits addressed | No change |

---

## Implementation Changes

| File | Change |
|------|--------|
| `server/.../agent/observer_longmemeval_test.go` | Add threshold comment, token comparison, pass 2 abstention `Contains`, fix log label, add `detailForType` guard |
| `bugs/049/code3.md` | Fix inaccurate citation at line 109 |

---

## Success Criteria

- Token reduction assertion passes (compressed ≤ original)
- Pass 2 abstention checks both `NotContains("30-gallon")` and `Contains("10-gallon")`
- Log says "Passed" not "Observer pass"
- Unknown question types cause `t.Fatalf` instead of silent pass
- code3.md accurately describes threshold=1 rationale
- No test regressions

## Run Command

```bash
go test ./server/router/api/v1/agent/ -run "TestEndToEndObserver|TestReflector_DetailPreservation|TestReflector_PreservesMultipleDetails|TestMalformedLLMOutput|TestEdgeCases|TestLongMemEvalDataLoader|TestDetailPreservationByQuestionType_MockFallback" -v -count=1
```
