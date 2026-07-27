# Plan v2: Detail Preservation Tests for bchat's Observational Memory

**Bug/Feature:** 049
**Date:** 2026-07-27
**Status:** Plan v2 (revised per adversarial review)
**File:** `server/router/api/v1/agent/observer_longmemeval_test.go`

---

## Context

bchat's Observational Memory (OM) uses an LLM to compress conversation history into observation logs. The `prompts/observer.txt` prompt instructs the LLM to preserve specific details.

Research in `Desktop/memory/memtest.md` shows Mem0 — which uses similar LLM summarization — **failed all 5 LongMemEval questions** because summarization dropped specific details (playlist names, chess moves). EmergenceMem Simple Fast achieved 79% using RAG on raw turns instead.

**Goal:** Validate that bchat's observer pipeline preserves details that LLM summarization typically loses.

**Benchmark:** LongMemEval (500 questions, 5 memory skills: IE, MR, KU, TR, ABS).

### OM vs RAG Boundary (acknowledged per review)

- **OM's job:** Preserve enough context for coherent conversation continuation
- **RAG's job:** Preserve exact facts for precise retrieval
- **The real test:** Does OM preserve enough detail that downstream systems can retrieve it?

---

## Data Source

**File:** `build/data/longmemeval_s.json`
**Source:** https://huggingface.co/datasets/xiaowu0162/longmemeval-cleaned

### Data Setup

Add a setup step that extracts from `Desktop/memory/LongMemEval-main.zip` if `build/data/longmemeval_s.json` doesn't exist:

```bash
# In Taskfile or test helper
mkdir -p build/data
cd /tmp && unzip -o /home/chaschel/Desktop/memory/LongMemEval-main.zip \
  LongMemEval-main/data/longmemeval_s.json \
  && mv LongMemEval-main/data/longmemeval_s.json /path/to/build/data/
```

If extraction fails, tests use an embedded subset of 5 hand-crafted questions (see Embedded Test Data below).

---

## File Structure

```
server/router/api/v1/agent/
├── observer_longmemeval_test.go  # NEW (renamed from benchmark)
├── observer_test.go              # Existing (unchanged)
├── observer.go                   # Existing (unchanged)
├── om_config.go                  # Existing (unchanged)
├── llm_mock_test.go              # Existing mock infrastructure (reuse)
└── prompts/
    └── observer.txt              # Existing (unchanged)
```

---

## Test Structure

All tests are parameterized where possible. The file uses existing mock infrastructure (`newMockLLMServer`, `withMockLLM`) for Tier 1, and real LLM for Tier 2.

### Test 1: End-to-End Observer Pipeline — Detail Preservation

**Replaces old Tests 1, 4, 5** (prompt text, formatting, XML parsing — all test implementation, not behavior).

**What it tests:** Full `RunObserver` → `callObserverLLM` → `parseXMLTag` → observation output path using mock LLM.

**Setup:**
1. Create a `Service` with test store and tenant (using `newBridgeChatTestService` pattern)
2. Create an agent session with messages containing specific details:
   - User: "I created a playlist on Spotify called Summer Vibes"
   - Assistant: "Great choice! Summer Vibes has some excellent chill tracks."
3. Set up mock LLM to return properly formatted XML with those details preserved
4. Call `RunObserver` on the session

**Assertions:**
- `RunObserver` returns nil error
- Observation log is persisted (via `GetObservationLog`)
- Observation text contains "Summer Vibes" (exact substring match)
- Observation contains both user and assistant contributions
- Current-task and suggested-response are extracted

**Mock output:**
```xml
<observations>
Date: Jan 1, 2025
* 🔴 (14:30) User stated they created playlist "Summer Vibes" on Spotify
* 🟡 (14:31) Assistant confirmed playlist has chill tracks
</observations>
<current-task>None</current-task>
<suggested-response>Acknowledge the playlist.</suggested-response>
```

### Test 2: Reflector Detail Preservation

**NEW — highest production risk per review.**

**What it tests:** When observations exceed `TokenThreshold`, the reflector compresses them. Does compression preserve specific details?

**Setup:**
1. Create a Service with mock LLM
2. Create an observation log with detailed observations (10+ facts including names, dates, entities)
3. Trigger `RunObserver` enough times to exceed `TokenThreshold` (or set threshold low for test)
4. Mock reflector returns compressed output

**Assertions:**
- After compression, key facts survive:
  - Specific names (e.g., "Summer Vibes")
  - Dates (e.g., "last January")
  - Entity attributes (e.g., "Alex who works at Google")
- Compression ratio is logged

**Mock reflector output:**
```xml
<observations>
Date: Jan 1, 2025
* 🔴 User created playlist "Summer Vibes" on Spotify (chill tracks)
* 🔴 User mentioned friend Alex (works at Google)
* 🟡 Visited museum with Alex last January
</observations>
```

### Test 3: Malformed LLM Output Handling

**NEW — covers real-world LLM output.**

**What it tests:** Parser degrades gracefully when mock returns messy XML.

**Test cases (subtests):**

| Case | Mock Output | Expected |
|------|-------------|----------|
| Missing closing tag | `<observations>text here` | Partial extraction or empty (no crash) |
| Content outside tags | `Here are your observations:\n<observations>text</observations>\nBy the way...` | Extracts content between tags |
| Multiple observation blocks | `<observations>first</observations> <observations>second</observations>` | First block extracted (existing behavior) |
| Extra whitespace | `<observations>\n  \n  * 🔴 detail\n  \n</observations>` | Whitespace trimmed |
| No XML tags at all | `Just plain text observations here` | Falls back to raw output (existing fallback) |
| Empty tags | `<observations></observations>` | Empty string returned |

**Assertions for all cases:**
- `parseXMLTag` does not panic
- `RunObserver` completes without error
- Observation log is persisted (even if empty or partial)

### Test 4: Edge Cases

**NEW — surface area coverage.**

| Subtest | Input | Expected |
|---------|-------|----------|
| Empty session | Session with 0 messages | `RunObserver` returns nil (no-op) |
| Trivial messages only | Session with only "ok", "thanks", "yes" | Observation log skipped, `LastObservedMsgIndex` updated |
| Single message | Session with one user message | One observation generated |
| Long content | Message with 5000+ char code block | Handled without truncation or error |
| Unicode content | Messages with CJK, emoji, accented chars | Preserved in observations |

**Setup:** Each subtest creates its own session with the specified message pattern. Uses mock LLM returning a simple canned observation.

### Test 5: LongMemEval Data Loader

**Replaces old Test 2 and Test 3** (loading and filtering).

**What it tests:** LongMemEval JSON parses correctly and filters work.

**Subtests:**
- `LoadHaystack`: Parse `longmemeval_s.json`, verify session count, turn alternation, date alignment
- `FilterByType`: Filter by each `question_type`, verify correct items returned
- `FormatForObserver`: Convert LongMemEval turns to bchat message format (without testing the exact `fmt.Sprintf` — just that role and content transfer correctly)

**Data handling:** If `build/data/longmemeval_s.json` is absent:
- Check for the zip at `Desktop/memory/LongMemEval-main.zip`
- If found, extract
- If not found, use embedded subset (5 questions) and call `t.Skip("Using embedded subset — full dataset not available")`

### Test 6: Detail Preservation Across Question Types (Parameterized)

**Consolidates old Tests 6–9** (all detail preservation) into one parameterized test.

**What it tests:** Real observer preserves details across the 5 LongMemEval memory skills.

**Gate:** `if os.Getenv("BENCHMARK_REAL_LLM") != "true" { t.Skip("Set BENCHMARK_REAL_LLM=true to run") }`

**Fallback:** If env var not set, run with mock LLM returning canned observations (tests the assertion logic, not the LLM).

**Subtests:**

| Question Type | Detail to Preserve | Session Content |
|---|---|---|
| `single-session-user` | Specific name | "Created playlist called Summer Vibes" |
| `single-session-assistant` | Implicit context | Chess moves "27. Kg2 Bd5+" without saying "chess" |
| `temporal-reasoning` | Temporal anchor | "Visited museum last January" |
| `knowledge-update` | State change | "Had 3 bikes → got a new hybrid" |
| `multi-session` | Cross-session facts | Session 1: Korg B1; Session 3: Yamaha guitar |
| `abstention` | No hallucination | No mention of "30-gallon fish tank" in any session |

**Assertions per subtest:**
- Exact substring match for specific names/numbers
- No hallucinated details (abstention test)
- Both old and new state captured (knowledge update)

---

## Embedded Test Data

For when the full dataset isn't available, embed 5 questions directly in the test file:

```go
var embeddedQuestions = []LongMemEvalQuestion{
    {
        QuestionID:   "embedded_ie_name",
        QuestionType: "single-session-user",
        Question:     "What is the name of the playlist I created on Spotify?",
        Answer:       "Summer Vibes",
        HaystackSessions: [][]map[string]string{
            {
                {"role": "user", "content": "I've been listening to this one playlist on Spotify that I created, called Summer Vibes..."},
                {"role": "assistant", "content": "Summer Vibes sounds like a great playlist for chill vibes!"},
            },
        },
        HaystackDates: []string{"2025-01-15"},
    },
    // ... 4 more covering each question type
}
```

---

## Mock Infrastructure

Reuse existing mock LLM infrastructure:

```go
// From llm_mock_test.go - reuse newMockLLMServer, withMockLLM
// From bridge_foundation_test.go - reuse newBridgeChatTestService
```

Add helper to create test sessions with specific details:

```go
func createTestSession(service *Service, tenantID int32, messages []AgentMessage) string {
    // Create session, populate messages, return session ID
}
```

Add canned mock outputs:

```go
var mockObservations = map[string]string{
    "specific_name": `<observations>
Date: Jan 1, 2025
* 🔴 (14:30) User stated they created playlist "Summer Vibes" on Spotify
</observations>
<current-task>None</current-task>
<suggested-response>Acknowledge the playlist.</suggested-response>`,

    "temporal_anchor": `<observations>
Date: Mar 5, 2025
* 🔴 (14:30) User visited museum with friend Alex (meaning Jan 2025)
</observations>
<current-task>None</current-task>
<suggested-response>Reference the museum visit.</suggested-response>`,

    // ... more
}
```

---

## What We DON'T Build

- GPT-4o judge / accuracy scoring
- Comparison with published baselines
- Hybrid OM+RAG tests (Phase 2)
- Mock LLM server (reuse existing)
- Full LongMemEval benchmark runner (just detail preservation)

---

## Success Criteria

```
go test -v -run TestObserverLongMemEval ./server/router/api/v1/agent/

=== Tier 1 (mock LLM) ===
PASS: TestEndToEndObserver_DetailPreservation
PASS: TestReflector_DetailPreservation
PASS: TestMalformedLLMOutput
PASS: TestEdgeCases/empty_session
PASS: TestEdgeCases/trivial_messages_only
PASS: TestEdgeCases/single_message
PASS: TestEdgeCases/long_content
PASS: TestEdgeCases/unicode_content
PASS: TestLongMemEvalDataLoader/LoadHaystack
PASS: TestLongMemEvalDataLoader/FilterByType
PASS: TestLongMemEvalDataLoader/FormatForObserver
PASS: TestDetailPreservationByQuestionType/single-session-user (mock)
PASS: TestDetailPreservationByQuestionType/multi-session (mock)
PASS: ... (all 6 subtests pass with mock)

=== Tier 2 (BENCHMARK_REAL_LLM=true) ===
PASS: TestDetailPreservationByQuestionType/single-session-user (real LLM)
PASS: TestDetailPreservationByQuestionType/temporal-reasoning (real LLM)
PASS: ... (all 6 subtests pass with real LLM)
```

---

## Action Items (from review)

| # | Change | Status |
|---|--------|--------|
| 1 | Replace Tests 1, 4, 5 with end-to-end pipeline test | Done (Test 1) |
| 2 | Add reflector detail-preservation test | Done (Test 2) |
| 3 | Add malformed XML / noisy output test | Done (Test 3) |
| 4 | Replace silent skip with `t.Skip("msg")` | Done (Test 5) |
| 5 | Rename file to `observer_longmemeval_test.go` | Done |
| 6 | Add edge-case tests | Done (Test 4) |
| 7 | Add "Tier 2 with mock" fallback | Done (Test 6 fallback) |
| 8 | Acknowledge OM vs RAG boundary | Done (Context section) |
| 9 | Add data extraction step or embed subset | Done (Embedded Test Data) |

---

## Next Steps

1. Review plan v2
2. Implement Test 1 (end-to-end pipeline)
3. Implement Test 2 (reflector)
4. Implement Tests 3–5 (malformed, edge cases, data loader)
5. Implement Test 6 (parameterized detail preservation)
6. Run full suite
7. Iterate on observer prompt if details are lost
