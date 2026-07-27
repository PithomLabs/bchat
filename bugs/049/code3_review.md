# Code 3 Review: Tier 2 Real LLM Detail Preservation Integration

**Status:** REWORK needed
**Reviewer:** AI Agent
**Date:** 2026-07-27
**Files examined:**
- `bugs/049/code3.md` (review document, 241 lines)
- `server/.../agent/observer_longmemeval_test.go:668-824` (actual implementation)
- `server/.../agent/observer_test_helpers_test.go:130-189` (helpers)

---

## Verdict: Rework

The code is functionally coherent and the two-pass pattern is correct, but there are 2 critical issues (plan divergence + missing assertion) and several significant gaps that were not caught by code3.md's own review checklist.

---

## 🔴 Critical

### 1. Threshold=1 diverges from plan5.md without documentation

**Plan5.md** (Finding #4, fixed): Changed threshold from 1 → 200:
```go
setOMEnvAndReload(t, "OM_TOKEN_THRESHOLD", "200")
```

**Actual code** (`observer_longmemeval_test.go:770`):
```go
setOMEnvAndReload(t, "OM_TOKEN_THRESHOLD", "1")
```

code3.md line 109 claims this is "noted in plan5.md as artificially low for testing" — this is **inaccurate**. Plan5.md explicitly rejected threshold=1 as producing "meaningless compression" and mandated 200.

**The rationale for divergence is valid** — threshold=200 would never trigger the reflector on 2-turn test sessions (~60-100 tokens). But this design decision is undocumented, and a future reader reconciling plan5.md with the code will find a contradiction.

**Fix:** Add a comment at line 770 explaining why threshold=1, not 200:
```go
// Threshold=1 guarantees the reflector fires on small test sessions
// (~2 turns, ~60-100 tokens). Plan5 suggested 200, but that would
// never trigger compression on these sessions.
setOMEnvAndReload(t, "OM_TOKEN_THRESHOLD", "1")
```

Also update code3.md line 109 to accurately describe the divergence.

### 2. No token reduction assertion (missing plan5.md success criterion)

**Plan5.md success criteria, line 254:**
```
assert.LessOrEqual passes for token reduction (not assert.Less)
```

**Plan5_review Finding #8 (action required):**
```go
// Before:
assert.Less(t, compressedTokens, originalTokens)
// After:
assert.LessOrEqual(t, compressedTokens, originalTokens,
    "compressed observation should be same size or smaller")
```

**Actual code:** Lines 794-806 only check detail preservation (`assert.True(t, found, ...)`). There is **zero** token comparison logic anywhere in the implementation.

The test validates that the reflector preserves details, but it never validates that the reflector actually **compresses** (reduces token count). If the reflector fires but produces identical output, the test would pass while the reflector is non-functional.

The `ObservationLog` struct has `TokensInLog int` at `store/agent.go:691` — the data is available. The assertion was simply omitted.

**Fix:** Add token comparison in pass 2. For each session, compare `obsLog.TokensInLog` before and after reset:
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

## 🟡 Significant

### 3. Pass 2 abstention assertion drops `Contains("10-gallon")`

**Pass 1** (lines 743-746) checks both conditions:
```go
assert.NotContains(t, obsLog.ObservationLog, "30-gallon",
    "raw observations must not hallucinate 30-gallon")
assert.Contains(t, obsLog.ObservationLog, "10-gallon",
    "raw observations must capture actual tank size")
```

**Pass 2** (lines 791-793) only checks one:
```go
assert.NotContains(t, obsLog.ObservationLog, "30-gallon",
    "compressed observations must not hallucinate 30-gallon")
// Missing: assert.Contains for "10-gallon"
```

If the reflector drops "10-gallon" while correctly not hallucinating "30-gallon", the test passes despite losing the valid detail. The `Contains("10-gallon")` check should be applied to both raw and compressed observations.

This was not caught by code3.md's review because the checklist (lines 198-209) only showed the pass 1 version.

### 4. Misleading log message at line 815

```go
t.Logf("  Observer pass: %v", !t.Failed())
```

This runs **after pass 2** (line 767-810). If pass 1 passed but pass 2's reflector assertion failed, `t.Failed()` returns `true`. The message prints `"Observer pass: false"` — but it was the **reflector** that failed, not the observer.

**Fix:** Either split into two log lines (before/after each pass) or rephrase:
```go
t.Logf("  Passed: %v", !t.Failed())
```

The pass 1 result is already captured in the `observerPassed` counter and summary at line 822.

### 5. `detailForType` default `""` produces silent pass

code3.md review line 177 acknowledges this:
> `strings.Contains(obsLog, "")` is always true

If a new question type is added to `embeddedQuestions` without updating `detailForType`, the test will silently pass on that question. `return ""` should be `t.Fatalf("unknown question type: %s", questionType)` — but since `detailForType` is a closure and only used inside `t.Run`, this needs adjustment.

**Fix:** Move the switch into the `t.Run` body so it can call `t.Fatalf`:
```go
detail := detailForType(q.QuestionType)
if detail == "" && q.QuestionType != "abstention" {
    t.Fatalf("unknown question type: %q — add to detailForType", q.QuestionType)
}
```

Or refactor to return `(string, bool)` and handle the missing case.

---

## 🟢 Minor

### 6. Inconsistent failure guard patterns

**Pass 1** (lines 762-765): early return pattern
```go
if t.Failed() {
    return
}
observerPassed++
```

**Pass 2** (lines 808-810): conditional increment pattern
```go
if !t.Failed() {
    reflectorPassed++
}
```

These are functionally equivalent but inconsistent. Choose one style.

### 7. `assert.Contains` would provide better diagnostics than `strings.Contains + assert.True`

Lines 753-759 and 799-805:
```go
if obsLog != nil && strings.Contains(obsLog.ObservationLog, detail) {
    found = true
    break
}
// ...
assert.True(t, found, "detail %q must appear...", detail)
```

`assert.Contains` shows the full string and the substring on failure. The custom pattern loses that info. Since the "found across sessions" logic needs the loop, use `assert.Contains` inside the loop:
```go
if obsLog != nil {
    if assert.Contains(t, obsLog.ObservationLog, detail,
        "detail %q must appear in session %s observations", detail, sid) {
        break
    }
}
```

But this has side effects (assertion calls accumulate). The current approach is acceptable — just less diagnostic on failure.

### 8. code3.md's "plan5.md noted as artificially low" is inaccurate

code3.md line 109 says the threshold=1 approach was "noted in plan5.md as artificially low for testing." This is false — plan5.md explicitly rejected threshold=1. The justification should reference the actual constraint (small test sessions), not plan5.md.

### 9. Plan5_review nits not tracked in code3.md

| plan5_review Nit | Addressed? |
|-----------------|------------|
| 1. `convertTurnsToMessages` missing | ✓ Added at `helpers.go:132` |
| 2. Detail mapping needs explicit switch | ✓ `detailForType` at line 688 |
| 3. Vestigial "update plan4.md" | N/A (plan doc, not code) |
| 4. Reflector assertion target ambiguous | ✓ Reset-and-re-run approach resolves this |
| 5. Model name log | ✓ Line 680 |
| 6. `ts.Close()` on `*store.Store` | ✓ Exists at `store/store.go:50` |

All 5 actionable nits are resolved — no regressions.

---

## Summary

| Severity | Count | Items |
|----------|-------|-------|
| 🔴 Critical | 2 | Plan divergence (threshold), missing token reduction assertion |
| 🟡 Significant | 3 | Missing pass 2 abstention `Contains`, misleading log label, silent `""` default |
| 🟢 Minor | 4 | Inconsistent guard patterns, assertion style, inaccurate citation, missing nit tracking |

**Recommendation:** Fix critical items 1-2 before merge. Items 3-5 are significant but low risk in practice. The two-pass pattern, error recovery, and multi-session handling are all correct.
