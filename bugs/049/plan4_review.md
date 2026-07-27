# Plan 4 Review: Tier 2 Real LLM Detail Preservation Test

**Status:** REWORK needed
**Reviewer:** AI Agent
**Date:** 2026-07-27

---

## Verdict: Rework

The plan addresses a real gap (env loading in tests) and the shell passthrough approach is clean. However, there are blocking issues that must be resolved before implementation.

---

## 🔴 Critical (blocks implementation)

### 1. Question data mismatch with code2.md S2

code2.md issue **S2** adds a 6th embedded question of type `single-session-assistant` with answer "Korg B1 keyboard":

```go
QuestionType: "single-session-assistant",
Answer:       "Korg B1 keyboard",
```

But plan4.md's table (lines 63-71) assigns:

| # | Type | Detail |
|---|------|--------|
| 2 | single-session-assistant | "27. Kg2 Bd5+" |
| 3 | multi-session | "Korg B1" |

These are contradictory. If S2 is applied first, `single-session-assistant` will contain "Korg B1", not "27. Kg2 Bd5+". The plan reuses "Korg B1" for `multi-session`, causing a data collision.

**Fix:** Decide one layout and make both plans agree. Either:
- (A) Change code2.md S2 to use "27. Kg2 Bd5+" as the detail, or
- (B) Change plan4.md to match what code2.md S2 creates.

### 2. Undefined functions referenced

- **`ReloadOMConfig`** (line 44): The codebase has `LoadOMConfig` (reads env vars once) but no `ReloadOMConfig`. The plan either needs to specify implementing this function in `om_config.go`, or use `t.Setenv("OM_TOKEN_THRESHOLD", "1")` before each call (which takes effect automatically if OM config reads from env at call time — needs verification).

- **`createTestSession`**: Not referenced in the existing codebase or code2.md. The plan says "Load haystack sessions into a test session via createTestSession" but doesn't define the signature or behavior. This helper must be either implemented alongside this test or reused from somewhere.

### 3. Missing dependency linkage to code2.md

"Depends on: code2.md fixes (completed)" is too vague. Which code2.md issues are prerequisites?

| Issue | Needed for | Why |
|-------|-----------|-----|
| C1/C3 | `newObserverTestService` | Plan4 test uses this function — doesn't exist without C1/C3 |
| C2 | Embedded questions load | Without C2, `loadLongMemEvalQuestions` skips entirely |
| S2 | 6th question type | Plan4 expects 6 questions including `single-session-assistant` |
| S4 | Skip message | Minor, but skip message format matters for clarity |

If code2.md isn't applied first, `TestRealLLM_DetailPreservationIntegration` won't compile (missing `newObserverTestService`, wrong question count).

---

## 🟡 Significant

### 4. `OM_TOKEN_THRESHOLD=1` produces unrealistic compression test

Setting threshold to 1 means the reflector triggers after every single token of observations. In practice, this would reflect essentially empty observations most of the time. The test won't demonstrate meaningful compression behavior.

**Suggestion:** Use a realistic value like `OM_TOKEN_THRESHOLD=200` (matching the existing default-ish range) after loading meaningful sessions. Or, if the intent is to force compression, show intent explicitly and document that the threshold is artificially low.

### 5. No error handling for real API failures

The test calls a live LLM 12 times. Possible failures:
- Network timeout
- Rate limiting (OpenRouter is aggressive on free keys)
- Invalid or expired API key
- Model unavailable

The plan only checks `OPENROUTER_API_KEY is set`. All 12 calls are unprotected. A single failure (`t.Fatal` from `RunObserver`) kills the whole test instead of logging a warning for that sub-question.

**Suggestion:** Use a helper that wraps each sub-test with a timeout and error recovery:

```go
t.Run(tt.questionType, func(t *testing.T) {
    obsLog, err := runObserverWithTimeout(ctx, service, ...)
    if err != nil {
        t.Logf("API call failed for %s: %v — skipping", tt.questionType, err)
        return
    }
    // assertions...
})
```

### 6. Abstention question has no session context

Line 71: `abstention | Must NOT say "30-gallon"`.

There are no session contents, no haystack data, and no description of what the LLM should be asked. For an abstention test to be meaningful, the test must present information that *could* lead to hallucination but should not. Without the session context, this test scenario cannot be reviewed or implemented correctly.

**Suggestion:** Provide the full abstention test case including:
- Session content (what user said)
- What detail should NOT appear in the observation
- Why the LLM might hallucinate it

### 7. No cleanup for env var mutation

Line 43-44: `Set OM_TOKEN_THRESHOLD=1, ReloadOMConfig` mutates global process state. If `t.Setenv` is not used, this leaks to other tests in the same package.

**Fix:** Use `t.Setenv("OM_TOKEN_THRESHOLD", "1")` which auto-restores on test cleanup, or implement `ReloadOMConfig` to be test-scoped.

---

## 🟢 Minor / Nits

### 8. Token reduction assertion may fail on short input

```go
assert.Less(t, compressedTokens, originalTokens)
```

If the original observation is already very short, compression may not reduce tokens. Use `assert.LessOrEqual` or guard with a minimum size check.

### 9. Multi-session types need multi-session setup

`multi-session` and `temporal-reasoning` imply multiple conversation sessions (separate `HaystackSessions`). The flow diagram shows a single `createTestSession` call per question. Clarify how `createTestSession` handles the `HaystackSessions [][]map[string]string` array — does it create one session per inner array?

### 10. Split pass/fail counts in summary

Line 47: "N/6 passed" is ambiguous — does this count observer passes or reflector passes? Log both:

```
Observer: N/6 passed
Reflector: M/6 passed
```

### 11. No mention of whether `observer_longmemeval_test.go` is new or extended

After code2.md, does this file already exist (with Tier 1 tests) or is plan4.md creating it for the first time? Make explicit: "code2.md creates `observer_longmemeval_test.go`; plan4.md adds `TestRealLLM_DetailPreservationIntegration` to the same file."

### 12. `assert.Contains` on `obsLog.ObservationLog`

What is the type of `ObservationLog`? If it's a string field, `assert.Contains` works. If it's a struct or slice, the assertion may fail at compile time or behave unexpectedly. Confirm the type from the existing `store.AgentObservationLog` struct.

---

## Summary of Required Changes

| # | Severity | Change |
|---|----------|--------|
| 1 | 🔴 | Align question data with code2.md S2 |
| 2 | 🔴 | Define or verify `ReloadOMConfig` / `createTestSession` |
| 3 | 🔴 | List exact code2.md prerequisites |
| 4 | 🟡 | Use realistic `OM_TOKEN_THRESHOLD` value |
| 5 | 🟡 | Add API error recovery per sub-test |
| 6 | 🟡 | Provide full abstention test scenario |
| 7 | 🟡 | Use `t.Setenv` for env mutations |
| 8 | 🟢 | Use `LessOrEqual` for token assertion |
| 9 | 🟢 | Document multi-session setup behavior |
| 10 | 🟢 | Split observer/reflector pass counts |
| 11 | 🟢 | Clarify file creation vs extension |
| 12 | 🟢 | Verify `ObservationLog` type matches `assert.Contains` |

---

*Review generated from bugs/049/plan4.md (105 lines) and bugs/049/code2.md (240 lines).*
