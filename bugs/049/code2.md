# Plan: Code Review Fixes for observer_longmemeval_test.go

**Bug/Feature:** 049
**Date:** 2026-07-27
**Status:** Plan (awaiting implementation)
**Review:** `bugs/049/code_review.md`

---

## Critical Issues (all 5)

### C1 + C3. Split test helper to support real LLM

**File:** `observer_test_helpers_test.go`

Replace single `newObserverTestService` with two variants:

```go
// newObserverTestService creates a lean Service without mock LLM.
// Used for Tier 2 tests (BENCHMARK_REAL_LLM=true).
func newObserverTestService(t *testing.T, slug string) (context.Context, *store.Store, *Service, *store.AgentTenant) {
    t.Helper()
    t.Setenv("RAG_PIPELINE_ENABLED", "false")
    ctx := context.Background()
    ts := teststore.NewTestingStore(ctx, t)
    tenant, err := ts.CreateAgentTenant(ctx, &store.AgentTenant{...})
    require.NoError(t, err)
    _, err = ts.CreateAgentAudience(ctx, &store.AgentAudience{...})
    require.NoError(t, err)
    service := NewService(ts, &profile.Profile{Driver: "sqlite", Mode: "prod"})
    return ctx, ts, service, tenant
}

// newObserverTestServiceWithMock creates a Service with a mock LLM server.
// Used for Tier 1 tests (default, no API key needed).
func newObserverTestServiceWithMock(t *testing.T, slug, reply string) (context.Context, *store.Store, *Service, *store.AgentTenant) {
    t.Helper()
    withMockLLM(t, reply)
    return newObserverTestService(t, slug)
}
```

**All existing test calls change from:**
```go
ctx, ts, service, tenant := newObserverTestService(t, "obs-e2e")
withMockLLM(t, mockObservations["specific_name"])
```

**To:**
```go
ctx, ts, service, tenant := newObserverTestServiceWithMock(t, "obs-e2e", mockObservations["specific_name"])
```

**Real LLM test (`TestDetailPreservationByQuestionType`) changes to:**
```go
ctx, ts, service, tenant := newObserverTestService(t, "obs-detail-"+tt.questionType)
// No withMockLLM call — uses real OPENROUTER_API_KEY
```

### C2. Fix embedded test data dead code

**File:** `observer_longmemeval_test.go`

Remove the `t.Skip` call. When full dataset is absent, fall through to embedded data:

```go
func loadLongMemEvalQuestions(t *testing.T) []LongMemEvalQuestion {
    t.Helper()
    // ... try to load from file paths ...
    // ... try zip extraction ...

    // Dataset not found — use embedded subset (always runs, not skipped).
    t.Logf("LongMemEval dataset not found — using embedded subset (5 questions)")
    return embeddedQuestions
}
```

The `TestLongMemEvalDataLoader` subtests will now always run against embedded data, even without the full dataset.

### C4. Fix trivial messages test assertions

**File:** `observer_longmemeval_test.go`

Replace dead code:
```go
obsLog, err := ts.GetObservationLog(ctx, sessionID)
_ = obsLog
_ = err
```

With real assertions:
```go
obsLog, err := ts.GetObservationLog(ctx, sessionID)
if err == nil && obsLog != nil {
    assert.Equal(t, 4, obsLog.LastObservedMsgIndex,
        "all 4 trivial messages should be skipped and index updated")
}
```

### C5. Fix `expectRaw` dead logic

**File:** `observer_longmemeval_test.go`

Add assertion in `TestMalformedLLMOutput` after the existing `require.NoError`:
```go
if tt.expectRaw {
    assert.Contains(t, obsLog.ObservationLog, "User stated something important",
        "fallback should preserve raw output content")
}
```

---

## Significant Issues (2 of 4)

### S1. Reflector test with differentiated mock responses

**File:** `observer_test_helpers_test.go` + `observer_longmemeval_test.go`

Add a call-counting mock helper:
```go
func newCountingMockLLM(t *testing.T, replies ...string) {
    t.Helper()
    var callCount int
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        reply := replies[0] // default
        if callCount < len(replies) {
            reply = replies[callCount]
        }
        callCount++
        // ... encode response ...
    }))
    t.Cleanup(srv.Close)
    t.Setenv("OPENROUTER_API_KEY", "sk-or-v1-mock")
    t.Setenv("OPENROUTER_API_BASE_URL", srv.URL)
}
```

Update `TestReflector_DetailPreservation` to use different responses:
- Call 1 (observer): Returns detailed observations with "Summer Vibes"
- Call 2 (reflector): Returns compressed observations still containing "Summer Vibes"

Assert that the final log is shorter but still contains the detail.

### S2. Add `single-session-assistant` to embedded data

**File:** `observer_longmemeval_test.go`

Add a 6th question to `embeddedQuestions`:
```go
{
    QuestionID:   "embedded_ie_assistant",
    QuestionType: "single-session-assistant",
    Question:     "What did the assistant recommend for my home studio?",
    Answer:       "Korg B1 keyboard",
    HaystackSessions: [][]map[string]string{
        {
            {"role": "user", "content": "I'm setting up a home studio. What keyboard should I get?"},
            {"role": "assistant", "content": "I recommend the Korg B1 — it's an excellent digital piano with weighted keys and great sound quality."},
        },
    },
    HaystackDates: []string{"2025-05-01"},
},
```

Also add `single-session-assistant` to the expected types list in `FilterByType`.

---

## Minor Issues (2 of 5, quick wins)

### S3. Fix zip log message

**File:** `observer_longmemeval_test.go`

Change:
```go
t.Logf("Found zip at %s — extraction not implemented, using embedded subset", zp)
```
To:
```go
t.Logf("Found zip at %s — skipping extraction, using embedded subset (5 questions)", zp)
```

### S4. Update skip message after C1 fix

**File:** `observer_longmemeval_test.go`

Change:
```go
t.Skip("Set BENCHMARK_REAL_LLM=true to run with real LLM (mock mode: testing assertion logic only)")
```
To:
```go
t.Skip("Set BENCHMARK_REAL_LLM=true to test with a real LLM")
```

### M4. Sequential timestamps in mustMakeMessages

**File:** `observer_test_helpers_test.go`

```go
func mustMakeMessages(pairs ...string) []store.AgentMessage {
    msgs := make([]store.AgentMessage, 0, len(pairs)/2)
    baseTime := time.Now()
    for i := 0; i < len(pairs); i += 2 {
        msgs = append(msgs, store.AgentMessage{
            Role:      pairs[i],
            Content:   pairs[i+1],
            Timestamp: baseTime.Add(time.Duration(i/2) * time.Minute),
        })
    }
    return msgs
}
```

### M5. Remove `_ = dbSession`

**File:** `observer_test_helpers_test.go`

Remove the unused `_ = dbSession` line.

---

## Not Addressing (deferred)

| Issue | Rationale |
|-------|-----------|
| M1 (`FormatForObserver` shallow) | Testing field transfer is sufficient for Phase 1 |
| M2 (`encoding/json` unused import) | Will be used once full dataset loading is active |
| M3 (fragile zip paths) | Low risk, can address in follow-up |

---

## Implementation Order

1. `observer_test_helpers_test.go` — Split helpers, add counting mock, fix timestamps, remove dead code
2. `observer_longmemeval_test.go` — Fix embedded data, assertions, reflector test, add missing type, update messages
3. Run tests
4. Verify no regressions
