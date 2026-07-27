# Adversarial Plan Review: Detail Preservation Tests

**Plan:** `bugs/049/plan.md`
**Reviewer:** Senior Go Architect
**Status:** APPROVED WITH NITS

---

## Summary

The plan correctly identifies the core risk — LLM summarization dropping specific details (as documented in `memtest.md`) — and proposes a sensible two-tier structure. However, ~50% of the proposed tests test infrastructure or prompt wording rather than actual observer behavior. The biggest gap is the absence of an end-to-end observer pipeline test using the existing mock LLM infrastructure.

---

## 1. WHAT'S MISSING

### 1a. End-to-end pipeline test (critical)

Tests 1–5 are all unit-level (prompt text, data loading, formatting, XML parsing). None exercise the full `RunObserver` → `callObserverLLM` → `parseXMLTag` → `UpsertObservationLog` path.

The codebase already has `newMockLLMServer` + `withMockLLM` in `llm_mock_test.go` and `newBridgeChatTestService` in `bridge_foundation_test.go`. These can be reused to:
1. Create a `Service` with a test store and tenant
2. Set up an agent session with messages containing a specific name/entity
3. Route LLM calls to a mock returning a properly formatted XML observation
4. Call `RunObserver` and assert the observation log contains the detail

Without this, you test prompt *wording*, not prompt *behavior*.

### 1b. Reflector detail loss (explicitly excluded, highest risk)

The plan says "What we DON'T Build: ..." and excludes reflector testing. In production the reflector compresses observations. If it drops details, observer prompt improvements are moot. This is Phase 1 risk, not Phase 2. At minimum, a single test should feed observations through `runReflector` with a mock LLM and verify details survive.

### 1c. Edge cases

No tests for:
- Session with 0 messages
- Session where all messages are trivial (`"ok"`, `"thanks"`) — should skip gracefully
- Single-message session
- Messages with very long content (code blocks, markdown tables >5000 chars)
- Unicode/non-ASCII content
- Thread scope vs resource scope for observation storage

### 1d. Parallel test safety

`GetOMConfig()` returns a singleton. `TestOMConfig_Defaults` and tests calling `ReloadOMConfig()` can race. The plan must ensure tests are serialized or use `t.Setenv` (safe via `testing.Cleanup`). Existing tests already have this issue, but new tests should not compound it.

### 1e. Malformed LLM output handling

The mock returns perfect XML. Real LLM output is noisy: extra whitespace, missing closing tags, content outside XML structure, multiple `<observations>` blocks, inconsistent emoji. Add a test where the mock returns messy output and verify the parser degrades gracefully.

---

## 2. WHAT'S REDUNDANT

### Test 1 (prompt keyword presence)

Checking for literal strings `"PRESERVE UNUSUAL PHRASING"`, `"verbatim"`, etc. is a self-fulfilling test — it passes because you wrote those words. If someone refines the prompt to be more effective but rephrases these instructions, the test breaks with zero behavioral change. This belongs in a pre-commit hook, not `go test`.

### Test 4 (formatting)

Tests a ~5-line `fmt.Sprintf` loop (`observer.go:146–151`). If someone changes `**%s (%s):**` to `[%s] (%s):`, the test breaks but nothing meaningful changes. Testing implementation details.

| Implementation detail | Brittle? | Tests behavior? |
|---|---|---|
| Format string `**%s (%s):**` | Yes | No |
| Timestamp format `15:04` | Yes | No |
| Role casing `strings.ToUpper(...)` | Yes | No |

### Test 5 (XML parsing)

`parseXMLTag` is already thoroughly tested in `observer_test.go:7–60` (6 cases: simple, multiline, missing, case-insensitive, nested, multiple). A canned-output mock that calls `parseXMLTag` again tests nothing new.

### Test 6 and Test 9 overlap

"Summer Vibes" is both a specific name AND an entity attribute. Can be consolidated into a single parameterized test.

---

## 3. WHAT'S UNTESTABLE

### "Detail preservation" is binary in the plan, continuous in reality

The plan asserts passing as "observation text contains the name." But what if the observation says "user mentioned a playlist" without the actual value? The test passes but the detail is effectively lost. The threshold is underspecified — exact match? substring? semantic equivalence?

### Tier 1 prompt keyword checks validate nothing about LLM behavior

A prompt can contain all the right words and the LLM can still ignore them. The plan's own Open Question #1 acknowledges this. These tests are assertions about a text file, not validations of the system.

---

## 4. WHAT'S FRAGILE

### `TestFormatMessagesForObserver` uses `time.Now()`

`observer.go:149` uses `time.Now().Format("15:04")` — the timestamp is always the *current* time, not the message timestamp. The test either:
- Passes pre-determined timestamps (doesn't test the real code path), or
- Compares against `time.Now()` (flaky, non-deterministic)

This will be a source of CI flakes.

### LongMemEval data format dependency

The upstream dataset is at `/home/chaschel/Desktop/memory/LongMemEval-main.zip` — not at `build/data/longmemeval_s.json`. If the extraction step is missing or the format changes, tests silently skip. Silent skips are worse than failures: they pass in CI and give false confidence. Use `t.Skip("msg")` at minimum, and consider a CI check that the file exists.

### Test 1 keyword list maintenance burden

If someone adds `"PRESERVE URLS"` to the prompt, they must remember to update the test assertion list. If they forget, the test doesn't fail — the checklist is just incomplete. The test is only as strong as its out-of-sync item list.

---

## 5. WRONG ABSTRACTION

### Testing inputs instead of outputs

Test 1 tests an input (prompt text) instead of a behavior (does the observer output preserve details?). The entire Tier 1 suite should test outputs via mock LLM, not inputs via string matching.

### File naming

`observer_benchmark_test.go` is misleading. These are not `go test -bench` benchmarks. Rename to `observer_longmemeval_test.go`.

---

## 6. REAL RISK (if all tests pass)

| Risk | Severity | Plan covers? |
|------|----------|-------------|
| Reflector drops details | **Critical** | Explicitly excluded |
| LLM model inconsistency (Tier 2 tests one model) | **High** | No mitigation |
| Pipeline integration fails silently | **High** | No end-to-end test |
| Overfitting to LongMemEval patterns | **Medium** | No real-world test |
| Output format drift (model changes XML style) | **Medium** | No malformed-input test |
| Silent skip when data absent | **Medium** | "Skip gracefully" is underspecified |

### If all tests pass but the system fails in production, the most likely cause:

The observer prompt has the right instructions, the LLM produces good observations in isolation, but:
1. The reflector compresses away the details, or
2. The observer pipeline errors are silently swallowed, or
3. The vector DB indexing step fails, or
4. Real conversations don't match LongMemEval patterns.

---

## 7. MOCK REALISM

### Mock returns perfect XML; real LLMs don't

The plan's mock infrastructure shows clean canned outputs. Add a test with:
```
<observations>
some text with no closing tag
```
Or:
```
Here are your observations:

<observations>
* 🔴 (14:30) something
</observations>

By the way, I also noticed...
```
Assert the parser extracts the content without swallowing errors.

### No mock path for Tier 2

Tier 2 is gated by `BENCHMARK_REAL_LLM=true`. If not set, tests skip entirely. Add a "Tier 2 with mock" fallback using `withMockLLM` so the tests always run in CI. The real-LLM path becomes an optional enhancement.

---

## 8. DATA QUALITY

### Extraction prerequisite

The dataset exists at `/home/chaschel/Desktop/memory/LongMemEval-main.zip`. The plan assumes `build/data/longmemeval_s.json` exists but doesn't specify the extraction step. Add a `task` command or script to set up the test data, or embed a small subset (e.g., 5 questions) directly in the test file to avoid the external dependency.

### Benchmark–system mismatch

LongMemEval tests exact-fact retrieval from raw sessions. OM is a *summary* layer — it's designed for compression, not verbatim recall. If OM loses some detail during summarization, that's by design. Precise factual recall should come from the vector DB (RAG), not OM.

The plan should acknowledge this boundary explicitly:
- **OM's job:** preserve enough context for coherent conversation continuation
- **RAG's job:** preserve exact facts for precise retrieval
- **The real test:** does OM preserve enough detail that RAG can retrieve it?

---

## Action Items

| # | Change | Priority | Rationale |
|---|--------|----------|-----------|
| 1 | Replace Tests 1, 4, 5 with a single end-to-end pipeline test using mock LLM | High | Tests actual behavior, not wording |
| 2 | Add reflector detail-preservation test with mock LLM | High | Highest production risk |
| 3 | Add malformed XML / noisy output test | Medium | Covers real-world LLM output |
| 4 | Replace silent skip with `t.Skip("msg")` for absent dataset | Medium | Avoids false CI passes |
| 5 | Rename file to `observer_longmemeval_test.go` | Low | Avoids confusion with go benchmarks |
| 6 | Add edge-case tests (empty, trivial, single-msg) | Low | Surface area coverage |
| 7 | Add "Tier 2 with mock" fallback | Medium | Tests run without API key |
| 8 | Acknowledge OM vs RAG detail-preservation boundary | Medium | Clarifies success criteria |
| 9 | Add data extraction step or embed small test subset | Medium | Eliminates silent-skip scenario |
