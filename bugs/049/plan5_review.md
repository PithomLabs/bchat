# Plan 5 Review: Address Plan 4 Review Findings

**Status:** APPROVED with nits
**Reviewer:** AI Agent
**Date:** 2026-07-27

---

## Verdict: Approved with Nits

Plan 5 correctly validates 7/12 plan4_review findings against the codebase and specifies concrete fixes for the remaining 5. All codebase claims are verified. Six non-blocking nits remain.

---

## Verified Claims (codebase matches plan5)

| Finding | Claim | Code Location | Match |
|---------|-------|--------------|-------|
| #1 | `embeddedQuestions` matches plan4 table | `observer_longmemeval_test.go:305-396` | ✓ All 6 questions match |
| #2 | `ReloadOMConfig` exists | `om_config.go:87` | ✓ |
| #2 | `createTestSession` exists | `observer_test_helpers_test.go:93` | ✓ |
| #6 | Abstention has full session context | `embeddedQuestions[5].HaystackSessions` (lines 388-394) | ✓ 10-gallon tank, no 30-gallon mention |
| #7 | `setOMEnvAndReload` exists with cleanup | `observer_test_helpers_test.go:145` | ✓ |
| #12 | `ObservationLog` is `string` type | `store/agent.go:689` | ✓ |

---

## Action Items Confirmed (5 findings, all with valid fixes)

| Finding | Fix Specified | Assessment |
|---------|--------------|------------|
| #3 Dependency linkage | Document exact prerequisites | Clear. List of C1-C5, S1-S4, M4-M5 with code locations is correct. |
| #4 OM_TOKEN_THRESHOLD | Change to 200 via `setOMEnvAndReload` | Correct. Threshold 200 is realistic. Helper handles cleanup. |
| #5 API error recovery | `t.Run` per subtest with error→skip pattern | Solid. Logs failure, skips that subtest only. |
| #8 Token assertion | `assert.LessOrEqual` instead of `assert.Less` | Correct for short observations. |
| #9 Multi-session doc | Document `createTestSession` called once per inner `HaystackSession` | Accurate mapping per question type. |
| #10 Split pass/fail | Two separate counts, per-type breakdown | Pattern is clear. |
| #11 File extension | Add as new function in existing file | Correct. |

---

## Nits (6, non-blocking)

### 1. `convertTurnsToMessages` doesn't exist in codebase

Line 88 of the code pattern:
```go
msgs := convertTurnsToMessages(turns)
```

This function is undefined — grep returns zero results. The closest existing helper is `mustMakeMessages(t *testing.T, pairs ...string)` in `observer_test_helpers_test.go`, but it takes `string...` pairs, not `[]map[string]string`.

**Fix:** Either add this helper to `observer_test_helpers_test.go`:
```go
func convertTurnsToMessages(turns []map[string]string) []store.AgentMessage {
    msgs := make([]store.AgentMessage, 0, len(turns))
    baseTime := time.Now()
    for i, turn := range turns {
        msgs = append(msgs, store.AgentMessage{
            Role:      turn["role"],
            Content:   turn["content"],
            Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
        })
    }
    return msgs
}
```
Or inline the conversion in the test.

### 2. Detail-to-preserve needs an explicit mapping, not a placeholder

`LongMemEvalQuestion` (line 291-301) has no `DetailToPreserve` field. The plan's code pattern shows:
```go
assert.Contains(t, lastObsLog.ObservationLog, /* detail from question */)
```

But the detail to preserve is not derivable from `q.Answer` (e.g., multi-session `Answer` is "2" but the detail is "Korg B1"). The table in lines 238-245 correctly maps type→detail, but the code has no equivalent.

**Fix:** Use a switch/case on `q.QuestionType`:
```go
var detail string
switch q.QuestionType {
case "single-session-user":
    detail = "Summer Vibes"
case "single-session-assistant":
    detail = "27. Kg2 Bd5+"
case "multi-session":
    detail = "Korg B1"
case "knowledge-update":
    detail = "4 bikes"
case "temporal-reasoning":
    detail = "museum"
case "abstention":
    // special handling: NOT "30-gallon", contains "10-gallon"
}
```

Or add a `DetailToPreserve string` field to the struct and populate it per question.

### 3. Vestigial "Update plan4.md" instruction

Line 44:
> Update plan4.md's "Depends on" from vague to exact.

Plan5.md supersedes plan4.md — there's no need to update plan4.md. This instruction is dead and will confuse anyone reading the plan file.

**Fix:** Remove this sentence or rephrase as "Document exact prerequisites in the test file header comment."

### 4. Reflector assertion target is ambiguous

The flow (lines 229-232):
```
├── setOMEnvAndReload(t, "OM_TOKEN_THRESHOLD", "200")
├── RunObserver again (triggers reflector on existing logs)
├── Assert compressed observations still contain detail
```

For multi-session types (knowledge-update, temporal-reasoning, multi-session), there are 2+ sessions each with their own observation log. The plan doesn't specify **which** session's compressed log to assert on.

**Fix:** Clarify: "Run observer on all sessions again. Assert on the last session's observation log for each question." Or use a different heuristic (e.g., assert all session logs contain the detail).

### 5. Missing model name log

Flow line 213 says "Log model being used" but the code pattern doesn't include it.

**Fix:** Add before the main loop:
```go
model := os.Getenv("LLM_MODEL")
if model == "" {
    model = "openai/gpt-4o-mini"
}
t.Logf("Using LLM model: %s", model)
```

### 6. `defer ts.Close()` not verified

Line 83: `defer ts.Close()` — `ts` is a `*store.Store` from `teststore.NewTestingStore`. Verify that `store.Store` has a `Close()` method, or check that the returned type from `NewTestingStore` exposes one. If not, the call will fail at compile time.

---

## Summary

| Category | Count |
|----------|-------|
| Findings confirmed resolved | 7/12 |
| Action items with valid fixes | 5/5 |
| New nits found | 6 (all minor) |

Plan is ready for implementation after addressing nits 1-6. The core design (shell env passthrough, per-question subtests with error recovery, `setOMEnvAndReload` for threshold) is sound.
