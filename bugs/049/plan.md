# Plan: Detail Preservation Tests for bchat's Observational Memory

**Bug/Feature:** 049
**Date:** 2026-07-27
**Status:** Plan (not yet implemented)

---

## Context

bchat's Observational Memory (OM) uses an LLM to compress conversation history into observation logs. The `prompts/observer.txt` prompt instructs the LLM to preserve specific details (names, dates, entity attributes).

Research in `Desktop/memory/memtest.md` shows that Mem0 — which uses a similar LLM summarization approach — **failed all 5 LongMemEval questions** because summarization lost specific details (playlist names, chess moves). Meanwhile, EmergenceMem Simple Fast achieved 79% accuracy using RAG on raw turns instead of summaries.

**Goal:** Validate that bchat's observer prompt preserves the specific details that LLM summarization typically loses.

**Benchmark:** LongMemEval (500 questions, 5 memory skills: IE, MR, KU, TR, ABS).

---

## Approach: Two-Tier Testing

### Tier 1: No LLM (fast, deterministic, always runs)

Tests infrastructure and prompt analysis without API calls.

### Tier 2: Real LLM (opt-in, slower, validates actual behavior)

Gated by `BENCHMARK_REAL_LLM=true`. Uses real OpenRouter API calls.

---

## Data Source

**File:** `build/data/longmemeval_s.json`
**Source:** https://huggingface.co/datasets/xiaowu0162/longmemeval-cleaned
**Size:** ~40 sessions per question, ~115k tokens total
**Format:** 500 questions with haystack_sessions, haystack_dates, question, answer, question_type

Tests skip gracefully if the file is absent.

---

## File Structure

```
server/router/api/v1/agent/
├── observer_benchmark_test.go   # NEW — all benchmark tests
├── observer_test.go             # Existing (unchanged)
├── observer.go                  # Existing (unchanged)
├── om_config.go                 # Existing (unchanged)
└── prompts/
    └── observer.txt             # Existing (unchanged)
```

---

## Tier 1: No LLM Tests

### Test 1: `TestObserverPrompt_PreservesDetails`

Parse `prompts/observer.txt` via `//go:embed`. Assert it contains key preservation instructions:

| Instruction | Why it matters |
|---|---|
| `"PRESERVE UNUSUAL PHRASING"` | Direct instruction to keep exact terms |
| `"quote their exact words"` | Ensures names like "Summer Vibes" survive |
| `"DISTINGUISHING DETAILS"` | Preserves specifics in assistant responses |
| `"who/what/where/when"` | Temporal and entity preservation |
| `"preserve the attributes"` | Entity attributes (employer, location, etc.) |
| `"verbatim"` | Code blocks, formatted text |

### Test 2: `TestLongMemEvalLoader_LoadHaystack`

Parse `longmemeval_s.json` into bchat message format. Verify:
- Correct number of sessions loaded
- Each session has alternating user/assistant turns
- `AgentMessage.Role` is `"user"` or `"assistant"`
- `haystack_dates` aligns with session count

### Test 3: `TestLongMemEvalLoader_FilterByType`

Filter loaded data by `question_type`. Verify each type returns correct items:
- `single-session-user`
- `single-session-assistant`
- `multi-session`
- `temporal-reasoning`
- `knowledge-update`
- `abstention` (question_id ends with `_abs`)

### Test 4: `TestFormatMessagesForObserver`

Take a LongMemEval session, convert to the format `observer.go:146-151` produces:
```
**USER (14:30):**
message content

**ASSISTANT (14:31):**
response content
```
Verify formatting matches.

### Test 5: `TestMockObserver_ParsesOutput`

Mock observer returns canned XML:
```xml
<observations>
Date: Jan 1, 2025
* 🔴 (14:30) User stated they created playlist "Summer Vibes"
</observations>
<current-task>None</current-task>
<suggested-response>Acknowledge the playlist.</suggested-response>
```
Verify `parseXMLTag` extracts observations, current-task, suggested-response correctly.

---

## Tier 2: Real LLM Tests (opt-in)

**Gate:** `if os.Getenv("BENCHMARK_REAL_LLM") != "true" { t.Skip() }`

### Test 6: `TestObserverDetailPreservation_SpecificNames`

- Load `single-session-user` question with a specific name (e.g., playlist "Summer Vibes")
- Feed through real observer
- Assert observation text contains the name

### Test 7: `TestObserverDetailPreservation_ImplicitContext`

- Load question where context is implicit (chess moves without saying "chess")
- Assert notation is preserved

### Test 8: `TestObserverDetailPreservation_TemporalAnchor`

- Load question with relative time reference ("last January")
- Assert observation includes both message time and referenced time

### Test 9: `TestObserverDetailPreservation_EntityAttributes`

- Load question where entity has attributes ("Alex who works at Google")
- Assert both name and employer appear

### Test 10: `TestObserverDetailPreservation_CrossSession`

- Load two separate sessions with different facts (Korg B1 in session 1, Yamaha guitar in session 3)
- Run observer on both
- Assert both instrument names appear in combined observations

---

## Mock Infrastructure

For Tier 1 tests that need a "fake" observer response:

```go
type mockObserverOutput struct {
    Observations      string
    CurrentTask       string
    SuggestedResponse string
}

var cannedOutputs = map[string]mockObserverOutput{
    "specific_names": {
        Observations:      "* 🔴 (14:30) User stated they created playlist \"Summer Vibes\"",
        CurrentTask:       "None",
        SuggestedResponse: "Acknowledge the playlist.",
    },
    // ... more canned outputs
}
```

Tier 2 uses real `Service.callObserverLLM` with OpenRouter API.

---

## What We DON'T Build

- GPT-4o judge / accuracy scoring
- Comparison with published baselines
- Hybrid OM+RAG tests (Phase 2)
- Mock LLM server — just inline canned outputs

---

## Success Criteria

```
go test -v -run TestObserver ./server/router/api/v1/agent/

=== Tier 1 (no LLM) ===
PASS: TestObserverPrompt_PreservesDetails
PASS: TestLongMemEvalLoader_LoadHaystack
PASS: TestLongMemEvalLoader_FilterByType
PASS: TestFormatMessagesForObserver
PASS: TestMockObserver_ParsesOutput

=== Tier 2 (BENCHMARK_REAL_LLM=true) ===
PASS: TestObserverDetailPreservation_SpecificNames
PASS: TestObserverDetailPreservation_ImplicitContext
PASS: TestObserverDetailPreservation_TemporalAnchor
PASS: TestObserverDetailPreservation_EntityAttributes
PASS: TestObserverDetailPreservation_CrossSession
```

---

## Open Questions for Adversarial Review

1. **Is prompt text analysis sufficient?** The prompt may contain preservation instructions, but does the LLM actually follow them? Tier 2 validates this, but Tier 1 is just checking text exists.

2. **Is LongMemEval_S the right scale?** ~40 sessions / ~115k tokens fits in context. But real bchat sessions may be shorter or longer. Does the benchmark reflect real usage?

3. **Are 5 detail types enough?** We test names, implicit context, temporal anchors, entity attributes, cross-session. Are there other failure modes we're missing?

4. **Is the mock infrastructure too simple?** Inline canned outputs work for unit tests but don't exercise the full observer pipeline. Is that okay for Tier 1?

5. **Does the skip-on-absent-file behavior hide problems?** If `longmemeval_s.json` is missing, tests silently skip. Should CI fail instead?

6. **Are we testing the right thing?** The prompt says "preserve details" — but maybe the real failure mode is that the LLM summarizes correctly but the summary is too compressed for downstream retrieval. Should we test retrieval quality too?

---

## Adversarial Plan Review Prompt

Before implementing, review this plan against the following questions:

```
You are a skeptical senior engineer reviewing this test plan. Be critical.

1. WHAT'S MISSING? What failure modes are not covered by these tests?
2. WHAT'S REDUNDANT? Which tests overlap and could be consolidated?
3. WHAT'S UNTESTABLE? Which assumptions are we making that we can't validate?
4. WHAT'S FRAGILE? Which tests will break on minor changes or be flaky?
5. WHAT'S THE WRONG ABSTRACTION? Are we testing implementation details instead of behavior?
6. WHAT'S THE REAL RISK? If these tests all pass, what could still go wrong in production?
7. IS THE MOCK REALISTIC? Does our mock infrastructure test what we think it tests?
8. DATA QUALITY? Could the LongMemEval data have issues that invalidate results?

For each question, provide a specific, actionable concern. Don't just say "more tests" — say WHICH tests and WHY.
```

---

## Next Steps

1. Review this plan (adversarial review)
2. Adjust based on feedback
3. Implement Tier 1 tests
4. Run Tier 1, verify pass
5. Implement Tier 2 tests (requires OPENROUTER_API_KEY)
6. Run Tier 2, inspect results
7. Iterate on observer prompt if details are lost
