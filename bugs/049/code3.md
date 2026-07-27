# Code 3: Tier 2 Real LLM Detail Preservation Integration

**Bug/Feature:** 049
**Date:** 2026-07-27
**Status:** Implemented
**Plan:** `bugs/049/plan5.md`

---

## Summary

Implements `TestRealLLM_DetailPreservationIntegration` — a Tier 2 test that validates observer and reflector detail preservation using a real LLM. Also adds two test helpers: `convertTurnsToMessages` and `resetObservationLog`.

### Files Changed

| File | Change |
|------|--------|
| `server/.../agent/observer_test_helpers_test.go` | Add `convertTurnsToMessages`, `resetObservationLog` |
| `server/.../agent/observer_longmemeval_test.go` | Add `TestRealLLM_DetailPreservationIntegration`, `fmt` import |

### Test Design

Two-pass pattern per question:
- **Pass 1** (default threshold): observer produces raw observations → assert detail present
- **Pass 2** (threshold=1): reset observation log, re-run observer → reflector fires → assert compressed detail present

---

## Review Prompt

Review the following code for correctness, edge cases, and potential issues. Check for:

### 1. Helper Correctness

**`convertTurnsToMessages`** (observer_test_helpers_test.go):
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

Review checklist:
- [ ] Does `turn["role"]` panic if key missing? (Go map access returns zero value — safe)
- [ ] Are timestamps sequential? (Yes, `i * time.Minute` increments)
- [ ] Does `len(turns)` match expectations for embedded questions? (Yes, 2 turns per session)
- [ ] Is `baseTime` appropriate for tests? (Yes, exact time doesn't matter)

**`resetObservationLog`** (observer_test_helpers_test.go):
```go
func resetObservationLog(t *testing.T, ctx context.Context, ts *store.Store, sessionID string) {
	t.Helper()
	obsLog, err := ts.GetObservationLog(ctx, sessionID)
	require.NoError(t, err)
	require.NotNil(t, obsLog, "observation log must exist before reset")
	obsLog.ObservationLog = ""
	obsLog.LastObservedMsgIndex = -1
	obsLog.TokensInLog = 0
	_, err = ts.UpsertObservationLog(ctx, obsLog)
	require.NoError(t, err, "failed to reset observation log")
}
```

Review checklist:
- [ ] Does `LastObservedMsgIndex = -1` cause `RunObserver` to re-process all messages? (Yes — `len(session.Messages) <= lastIdx+1` becomes `len(session.Messages) <= 0`, false for non-empty sessions)
- [ ] Is `UpsertObservationLog` the correct API for updating? (Yes — it's an upsert, updates existing record by session_id)
- [ ] Does the `require.NotNil` guard prevent nil pointer dereference? (Yes)
- [ ] Should we also reset `CurrentTask` and `SuggestedResponse`? (No — these are set by the observer on each call, old values are overwritten)

### 2. Two-Pass Pattern Logic

**Pass 1 flow:**
```go
for _, sid := range sessionIDs {
    if err := service.RunObserver(ctx, tenant.ID, sid); err != nil {
        t.Logf("Pass 1 API call failed for %s session %s: %v — skipping", q.QuestionType, sid, err)
        pass1OK = false
        break
    }
}
```

**Pass 2 flow:**
```go
setOMEnvAndReload(t, "OM_TOKEN_THRESHOLD", "1")

for _, sid := range sessionIDs {
    resetObservationLog(t, ctx, ts, sid)
}

for _, sid := range sessionIDs {
    if err := service.RunObserver(ctx, tenant.ID, sid); err != nil {
        t.Logf("Pass 2 API call failed for %s session %s: %v — skipping reflector check", q.QuestionType, sid, err)
        return
    }
}
```

Review checklist:
- [ ] Does `setOMEnvAndReload` restore original value on cleanup? (Yes — `t.Cleanup` restores + reloads)
- [ ] Is threshold=1 appropriate? (Yes — guarantees reflector fires on any observation; threshold=200 would never trigger on ~2-turn sessions producing ~60-100 tokens)
- [ ] Does resetting observation log also require resetting in-memory session? (No — `RunObserver` reads `session.Messages` which are unchanged; `LastObservedMsgIndex` is in obsLog, not session)
- [ ] Can pass 2 fail if pass 1 failed? (No — `pass1OK` check returns early before pass 2)
- [ ] Does `t.Failed()` guard before `observerPassed++` correctly skip counting on assertion failure? (Yes)

### 3. Multi-Session Handling

```go
var sessionIDs []string
for i, turns := range q.HaystackSessions {
    msgs := convertTurnsToMessages(turns)
    sid := createTestSession(t, ctx, ts, service, tenant.ID,
        fmt.Sprintf("session-real-%s-%d", q.QuestionType, i), msgs)
    sessionIDs = append(sessionIDs, sid)
}
```

Review checklist:
- [ ] Does each `HaystackSession` get a unique session ID? (Yes — `session-real-{type}-{index}`)
- [ ] Does `createTestSession` persist to both DB and in-memory? (Yes — verified in helpers.go:93-111)
- [ ] Are multi-session questions handled correctly? (Yes — multi-session has 2 inner sessions, each gets its own session ID)
- [ ] Does `RunObserver` process each session independently? (Yes — it takes a single sessionID)
- [ ] Is the detail assertion correct for multi-session? (Yes — checks `found` across all session logs)

### 4. Error Recovery

```go
pass1OK := true
for _, sid := range sessionIDs {
    if err := service.RunObserver(ctx, tenant.ID, sid); err != nil {
        t.Logf("Pass 1 API call failed for %s session %s: %v — skipping", q.QuestionType, sid, err)
        pass1OK = false
        break
    }
}
if !pass1OK {
    return
}
```

Review checklist:
- [ ] Does `return` skip only the current subtest? (Yes — we're inside `t.Run`)
- [ ] Is logging before return correct for debugging? (Yes)
- [ ] Should we use `t.Skip` instead of `return`? (Debatable — `t.Skip` marks as skipped, `return` marks as passed. For API failures, `return` is appropriate since the test didn't fail, it just couldn't run)
- [ ] Is the break correct for multi-session? (Yes — if any session fails, skip the whole question)

### 5. Detail Mapping

```go
detailForType := func(questionType string) string {
    switch questionType {
    case "single-session-user":
        return "Summer Vibes"
    case "single-session-assistant":
        return "27. Kg2 Bd5+"
    case "multi-session":
        return "Korg B1"
    case "knowledge-update":
        return "4 bikes"
    case "temporal-reasoning":
        return "museum"
    default:
        return ""
    }
}
```

Review checklist:
- [ ] Does the `default` case return `""` cause issues? (No — `strings.Contains(obsLog, "")` is always true, but `assert.True(t, found)` would pass. For unknown types, this is acceptable since we only iterate over `embeddedQuestions`)
- [ ] Is "4 bikes" the correct detail for knowledge-update? (Yes — session 2 says "now I have 4 bikes")
- [ ] Is "Korg B1" the correct detail for multi-session? (Yes — session 1 mentions "Korg B1 keyboard")
- [ ] Is "museum" the correct detail for temporal-reasoning? (Yes — session 1 mentions "visited the modern art museum")

### 6. Assertion Correctness

**Detail-preserve path:**
```go
found := false
for _, sid := range sessionIDs {
    obsLog, err := ts.GetObservationLog(ctx, sid)
    require.NoError(t, err)
    if obsLog != nil && strings.Contains(obsLog.ObservationLog, detail) {
        found = true
        break
    }
}
assert.True(t, found, "detail %q must appear in at least one session's raw observations", detail)
```

**Abstention path:**
```go
for _, sid := range sessionIDs {
    obsLog, err := ts.GetObservationLog(ctx, sid)
    require.NoError(t, err)
    require.NotNil(t, obsLog)
    assert.NotContains(t, obsLog.ObservationLog, "30-gallon",
        "raw observations must not hallucinate 30-gallon")
    assert.Contains(t, obsLog.ObservationLog, "10-gallon",
        "raw observations must capture actual tank size")
}
```

Review checklist:
- [ ] Is `strings.Contains` correct for checking detail in observation? (Yes — detail is a substring of the observation)
- [ ] Should we use `assert.Contains` instead of `strings.Contains` + `assert.True`? (Could, but `strings.Contains` gives more control over the "found" logic)
- [ ] Does the abstention path check ALL session logs? (Yes — iterates all sessions)
- [ ] Is `require.NotNil(obsLog)` correct? (Yes — if obsLog is nil, subsequent assertions would panic)

### 7. Edge Cases

- [ ] What happens if `embeddedQuestions` returns 0 questions? (Loop doesn't execute, summary shows 0/0)
- [ ] What happens if a session has 0 messages? (`RunObserver` returns nil — no messages to observe)
- [ ] What happens if `createTestSession` fails? (`require.NoError` stops the subtest)
- [ ] What happens if the real LLM returns empty observations? (Observer persists empty string, assertions fail — expected behavior)
- [ ] What happens if `t.Failed()` is true after pass 1 assertions? (Returns early, doesn't increment `observerPassed`, doesn't run pass 2)

### 8. Concurrency

- [ ] Are tests safe to run in parallel? (No — `setOMEnvAndReload` mutates global OM config. But `t.Run` subtests run sequentially by default in Go, so this is fine)
- [ ] Could two subtests share the same OM config? (Yes — but `setOMEnvAndReload` registers cleanup, so config is restored between subtests)
- [ ] Is the `atomic.Int64` in `newCountingMockLLM` thread-safe? (Yes — but not relevant here since we use real LLM, not mock)

### 9. Cost

- [ ] How many API calls? (6 questions × 2 passes = 12 calls)
- [ ] What's the approximate cost? (~$0.02 with gpt-4o-mini, ~$0.15 with gpt-4o)
- [ ] Is this acceptable for a test? (Yes — it's gated behind `BENCHMARK_REAL_LLM=true`)

### 10. Regression Risk

- [ ] Does this modify any existing test? (No — adds new test and helpers only)
- [ ] Does this change any production code? (No)
- [ ] Does this affect test infrastructure? (No — helpers are test-only)
