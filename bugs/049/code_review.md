# Adversarial Code/Test Review

**Implementation:** `code.md` → `observer_longmemeval_test.go`, `observer_test_helpers_test.go`, `llm_mock_test.go`
**Reviewer:** Senior Go Engineer
**Status:** REWORK REQUIRED

---

## Critical Issues (must fix before merge)

### C1. Real LLM tests always use mock LLM

`TestDetailPreservationByQuestionType` is supposed to be the real-LLM test (gated by `BENCHMARK_REAL_LLM=true`). But `newObserverTestService` at `observer_test_helpers_test.go:19` hardcodes `withMockLLM(t, "mock-observer-response")`, overriding `OPENROUTER_API_KEY` and `OPENROUTER_API_BASE_URL` to point at the mock server. The test body at line 617 then calls `withMockLLM` again. **Two mock servers** are created per test case. The real LLM path is unreachable regardless of the env var.

The real LLM test (`TestDetailPreservationByQuestionType`) is structurally identical to the mock fallback test (`TestDetailPreservationByQuestionType_MockFallback`) — both test the mock, neither tests the real LLM.

**Fix:** `newObserverTestService` must not call `withMockLLM`. Split into two helpers:
- `newObserverTestService(t, slug)` — no mock (for real LLM)
- `newObserverTestServiceWithMock(t, slug, reply)` — with mock (all current tests)

The real LLM test should use the no-mock variant and rely on the user's `OPENROUTER_API_KEY`.

---

### C2. Embedded test data is dead code

`loadLongMemEvalQuestions` (line 456) calls `t.Skip(...)` when the dataset is absent. `t.Skip` calls `runtime.Goexit()` — the `return embeddedQuestions` on line 457 is **never reached**. All three `TestLongMemEvalDataLoader` subtests (`LoadHaystack`, `FilterByType`, `FormatForObserver`) are skipped. The 5-question embedded subset is defined (line 338–416) but never exercised.

| Scenario | What happens | Should happen |
|----------|-------------|---------------|
| Full dataset present | Loaded and tested | ✅ Correct |
| Full dataset absent | `t.Skip` → subtests skip | ❌ Should test embedded data |

**Fix:** Remove the `t.Skip` call. When the dataset is absent, fall through to `return embeddedQuestions` so the subtests always run.

---

### C3. `withMockLLM` called twice in every test

`newObserverTestService` calls `withMockLLM` internally with throwaway reply `"mock-observer-response"`. Every test then calls `withMockLLM` again with the real mock content. This creates **two mock HTTP servers** per test — the first is orphaned (no client points to it) but consumes resources until `t.Cleanup`.

```
newObserverTestService  ──→  withMockLLM("mock-observer-response")  ──→  server A (never used)
test body               ──→  withMockLLM(mockObservations["..."])   ──→  server B (used)
```

**Fix:** Same as C1 — `newObserverTestService` should not set up a mock. The caller chooses which mock reply to use.

---

### C4. Trivial messages test asserts nothing

`TestEdgeCases/"trivial messages only"` (line 239–261) has no meaningful assertions:

```go
obsLog, err := ts.GetObservationLog(ctx, sessionID)
_ = obsLog   // ← unused
_ = err      // ← unused
```

The only real assertion is `require.NoError(t, err)` from `RunObserver`. The test should verify that `LastObservedMsgIndex` was updated to skip the trivial messages.

**Fix:** Replace dead code with:
```go
obsLog, err := ts.GetObservationLog(ctx, sessionID)
if err == nil && obsLog != nil {
    assert.Equal(t, 4, obsLog.LastObservedMsgIndex,
        "all trivial messages should be skipped")
}
```

---

### C5. `expectRaw` field is dead logic

In `TestMalformedLLMOutput` (line 133–222), each test case sets `expectRaw bool` but it is **never asserted**. The field exists in the struct; only `expectEmpty` is checked. For `missing_closing_tag` where `expectRaw: true`, the fallback in `observer.go:182–185` assigns the entire raw output — the test should verify this.

**Fix:** Remove the field and the associated dead logic, or add assertions:
```go
if tt.expectRaw {
    assert.Contains(t, obsLog.ObservationLog, "User stated something important")
}
```

---

## Significant Issues

### S1. Reflector test doesn't test real compression

`TestReflector_DetailPreservation` sets `OM_TOKEN_THRESHOLD=1` so the reflector runs. But the mock returns the **same response** for both the observer LLM call and the reflector LLM call. The test asserts `assert.Contains(t, obsLog.ObservationLog, "Summer Vibes")` — this passes trivially because both calls return the same content.

The test verifies that **the pipeline doesn't crash when the reflector runs**. It does NOT verify that the reflector compresses observations while preserving details.

**Fix:** Either:
1. Make the mock return different responses based on request content, OR
2. Create two observation log snapshots (before/after reflection) and verify the second is shorter but still contains key details

### S2. Loader missing `single-session-assistant` type

`TestLongMemEvalDataLoader/FilterByType` lists expected types as:
```go
"single-session-user", "multi-session", "knowledge-update",
"temporal-reasoning", "abstention"
```

Missing: `single-session-assistant`. The LongMemEval dataset has 6 question types. The embedded data also lacks this type.

**Fix:** Add `single-session-assistant` to both the expected types list and the embedded data questions.

### S3. Zip extraction logs but doesn't extract

Line 450–453:
```go
t.Logf("Found zip at %s — extraction not implemented, using embedded subset", zp)
```

"Extraction not implemented" sounds like a TODO. Change the message to "Skipping zip extraction — using embedded subset (5 questions)".

### S4. Real LLM `t.Skip` message contradicts behavior

Line 611:
```
"Set BENCHMARK_REAL_LLM=true to run with real LLM (mock mode: testing assertion logic only)"
```

"run with real LLM" implies the test uses a real LLM, but even when the env var is set, the test uses a mock (see C1). The first clause is misleading. The second clause ("testing assertion logic only") is vague.

**Fix:** After fixing C1, update to:
```
"Set BENCHMARK_REAL_LLM=true to test with a real LLM"
```

---

## Minor Issues

| ID | Issue | Location |
|----|-------|----------|
| M1 | `FormatForObserver` subtest doesn't test the actual format string — only field transfer | line 510 |
| M2 | `encoding/json` imported but only used in the skipped dataset parse path | line 4 |
| M3 | Zip path resolution uses 4 relative paths at different depths — fragile to directory restructuring | line 422–431 |
| M4 | `mustMakeMessages` uses `time.Now()` for all messages — identical timestamps within a batch | line 86 |
| M5 | `_ = dbSession` blank identifier in `createTestSession` — signals unused variable | line 70 |

---

## Real Risk Assessment

| Passes in CI | Breaks in production |
|---|---|
| All 25 tests pass with mock | Real LLM ignores preservation instructions — no test catches this |
| Reflector test passes (same response for both calls) | Real reflector compresses away details — nobody knows |
| Data loader subtests skip | Full dataset format could silently drift without detection |
| Mock returns perfect XML | Real LLM produces novel malformed patterns not in the 5 test cases |

---

## Summary

| Severity | Count | Issues |
|----------|-------|--------|
| **Critical** | 5 | C1, C2, C3, C4, C5 |
| **Significant** | 4 | S1, S2, S3, S4 |
| **Minor** | 5 | M1, M2, M3, M4, M5 |

The **mock LLM override in `newObserverTestService`** (C1) is the highest-priority fix — without it, the Tier 2 tests cannot function as designed. The **embedded data being dead code** (C2) means 30% of the test infrastructure exists but is never exercised. Fix C1–C5 and S1 before merge. S2–S4 and M1–M5 can be addressed in follow-up.
