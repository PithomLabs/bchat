# Plan 8: LongMemEval Full Benchmark (Final)

**Bug/Feature:** 049
**Date:** 2026-07-27
**Status:** PLAN (ready for implementation)
**Prerequisites:** code4.md fixes (all implemented and reviewed)
**Previous plans:** plan6.md → plan6_review.md → plan7.md → plan7_review.md

---

## Goal

Answer: **is bchat's OM good enough?**

Run LongMemEval against bchat's observational memory using `openrouter/free` for observer/answer and GPT-4o for judging. One-off validation run.

---

## Design Decisions

| Decision | Choice |
|----------|--------|
| LLM for observer/answer | `openrouter/free` (free model routing — log actual model per call for reproducibility) |
| LLM for judging | `openai/gpt-4o` via OpenRouter |
| Answer generation | Observation log + question → LLM → answer |
| OM scope | Resource-scoped (`OM_SCOPE=resource`) with UserID set |
| Sessions per question | 1 for single_hop/implicit_preference_v2; 2 for knowledge_update (both old+new concatenated) |
| Frequency | One-off manual run |
| Output | stdout summary + `build/benchmark/longmemeval_results_*.json` (final) + `*.jsonl` (crash recovery) |

---

## Dataset

### Files

| File | Path | Size | Use |
|------|------|------|-----|
| Questions | `/home/chaschel/Desktop/memory/custom_history_data/2_questions/0822_all_500_questions_final_v2.json` | 461 KB | Question metadata, types, answers |
| Session cache | `/home/chaschel/Desktop/memory/custom_history_data/6_session_cache/data_6_session_cache.json` | 415 MB | Turn data per question |

### Cache Format (3 variants on disk)

| Key | Used by | Questions | Loading |
|-----|---------|-----------|---------|
| `session` (list of turns) | `single_hop`, `implicit_preference_v2` | 100 | Single session |
| `session_old` + `session_new` (lists of turns) | `knowledge_update` | 76 | Concatenate both |
| `session_1` + `session_2` (lists of turns) | `two_hop` | 71 | Skipped (needs 2+) |
| *(key absent)* | `assistant_previnfo`, `multi_session_synthesis`, `temp_reasoning_*` | 253 | No data available |

### Data Loading Strategy

Load cache into `map[string]CacheEntry` (in-memory index, 415MB → ~5s parse). For each question:

```go
func extractTurns(entry CacheEntry) []Turn {
    // Format 1: session key (single_hop, implicit_preference_v2)
    if entry.Session != nil {
        return entry.Session
    }
    // Format 2: session_old + session_new (knowledge_update)
    if entry.SessionOld != nil {
        return append(entry.SessionOld, entry.SessionNew...)
    }
    // Format 3: session_1 (two_hop — completeness only, not tested)
    if entry.Session1 != nil {
        return entry.Session1
    }
    return nil
}
```

### Question Type Mapping

| LongMemEval Type | bchat OM Type | Sessions | In cache? | Tested? |
|-----------------|---------------|----------|-----------|---------|
| `single_hop` | `single-session-user` | 1 (`session`) | Yes (70/70) | Yes |
| `assistant_previnfo` | `single-session-assistant` | 1 | **No** (0/56) | No |
| `knowledge_update` | `knowledge-update` | 2 (`session_old`+`session_new`) | Yes (76/78) | Yes |
| `implicit_preference_v2` | `single-session-preference` | 1 (`session`) | Yes (30/30) | Yes |
| `two_hop` | `multi-session` | 2 | Yes | No (needs 2+) |
| `multi_session_synthesis` | `multi-session` | 2+ | No | No |
| `temp_reasoning_implicit` | `temporal-reasoning` | 2+ | No | No |
| `temp_reasoning_explicit` | `temporal-reasoning` | 2+ | No | No |

### Questions in Scope

| Type | Non-abs | Abstention | Total | Testable | Format |
|------|---------|------------|-------|----------|--------|
| single-session-user | 64 | 6 | 70 | 70 | `session` |
| single-session-assistant | 56 | 0 | 56 | **0** | No cache data |
| single-session-preference | 30 | 0 | 30 | 30 | `session` |
| knowledge-update | 72 | 6 | 78 | **76** (2 missing cache) | `session_old`+`session_new` |
| multi-session (2-hop) | 65 | 6 | 71 | 0 | Skipped by design |
| multi-session (synthesis) | 56 | 6 | 62 | 0 | No data |
| temporal-reasoning | 127 | 6 | 133 | 0 | No data |
| **Total** | **470** | **30** | **500** | **176** | — |

**Abstention breakdown:** 6 from single_hop + 6 from knowledge_update = **12 abstention questions testable** (not 30). Too few for statistical significance — treat as directional only.

---

## Pipeline

### Flow

```
For each question in scope:
  1. Load cache entry, extract turns (one of 3 formats)
  2. Create test service with OM_SCOPE=resource, set UserID on session
  3. For each session (1 for single_hop/preference, 2 for knowledge_update):
     a. Create session via createTestSession
     b. Set session.UserID = &userID (required for resource-scoped OM)
     c. RunObserver → produces observation log
  4. Read combined observation log
  5. Generate answer: observation log + question → openrouter/free → answer
  6. Judge answer: question + expected + actual → GPT-4o → yes/no
  7. Append result to JSONL crash recovery file
  8. Log pass/fail to stdout
  9. After all questions: aggregate JSONL → final JSON report
```

### Per-Question Pipeline (Go skeleton)

```go
type BenchmarkQuestion struct {
    QuestionID      string `json:"question_id"`
    QuestionType    string `json:"question_type"`
    QuestionContent struct {
        Facts       []string `json:"facts"`
        Question    string   `json:"question"`
        Answer      string   `json:"answer"`
        Explanation string   `json:"explanation"`
    } `json:"question_content"`
    HumanValidLabel bool   `json:"human_valid_label"`
}

type CacheEntry struct {
    QuestionID string                `json:"question_id"`
    Session    []map[string]any      `json:"session,omitempty"`
    SessionOld []map[string]any      `json:"session_old,omitempty"`
    SessionNew []map[string]any      `json:"session_new,omitempty"`
    Session1   []map[string]any      `json:"session_1,omitempty"`
    Session2   []map[string]any      `json:"session_2,omitempty"`
}
```

**Service setup:**
```go
t.Setenv("OM_SCOPE", "resource")
t.Setenv("OM_ENABLED", "true")
ctx, ts, service, tenant := newObserverTestService(t, "lmeval-"+q.QuestionID)
```

**Observer call with UserID:**
```go
var userID int32 = 999
turns := extractTurns(cacheEntry)
msgs := convertTurnsToMessages(turns)
sid := createTestSession(t, ctx, ts, service, tenant.ID, "lmeval-"+q.QuestionID, msgs)
memSession := service.memorySessions.Get(tenant.ID, sid)
memSession.UserID = &userID
service.memorySessions.Update(memSession)
err := service.RunObserver(ctx, tenant.ID, sid)
// Log the model used (from API response) for reproducibility
```

**Answer generation prompt:**
```
You are an AI assistant answering a question based on your memory of past conversations.

## MEMORY (Observation Log)
{observationLog}

## QUESTION
{question}

Answer concisely. If the memory does not contain enough information to answer, say "I don't have enough information to answer this question."
```

Model: `openrouter/free` (log actual model from response `model` field)

**Judge prompt — standard types:**
```
You are evaluating a model's answer. Determine if the answer is correct.
Question: {question}
Correct answer: {expected_answer}
Model answer: {actual_answer}

Is the model's answer correct? Answer "yes" or "no".
```

**Judge prompt — preference types (rubric-based):**
```
You are evaluating a model's answer on a preference/suggestion question.
Question: {question}
Rubric (expected behavior): {expected_answer}
Model answer: {actual_answer}

Is the model's answer correct based on the rubric? Answer "yes" or "no".
```

**Judge prompt — abstention:**
```
You are evaluating a model's answer. Determine if the model correctly identified the question as unanswerable.
Question: {question}
The correct response is that the question cannot be answered based on available information.
Model answer: {actual_answer}

Did the model correctly refuse to answer? Answer "yes" if it identifies the question as unanswerable or says it doesn't have enough information. Answer "no" if it hallucinated an answer.
```

Model: `openai/gpt-4o` via OpenRouter

**Gold baseline:**
Run 10 questions with the correct answer injected instead of the observation log. If the answer LLM fails on these, the noise is in the answer LLM, not OM.

Distribution: single_hop (3), implicit_preference_v2 (2), knowledge_update (3), abstention (2) = 10. Pick questions randomly from each type.

```go
// Gold check: answer LLM with expected answer injected
if isGoldCheck(q) {
    answerPrompt = fmt.Sprintf(
        "Answer this question with exactly this text: %s\nQuestion: %s",
        q.QuestionContent.Answer, q.QuestionContent.Question)
}
```

**Nil observation handling:**
If RunObserver fails for ALL sessions of a question (no observation log produced), record the question as "skipped" in the results (not as a failure). Log the error. This prevents empty observations from being evaluated as incorrect answers.

```go
if obsLog == nil || obsLog.ObservationLog == "" {
    result.Status = "skipped"
    result.Error = "no observation produced"
    appendToJSONL(result)
    return
}
```

---

## Report Format

### stdout Summary

```
=== LongMemEval Benchmark Results ===
Date: 2026-07-27
Scope: single-session types only (176/500 questions)
Model (observer/answer): openrouter/free (varies per call — logged per question)
Model (judge): openai/gpt-4o

single-session-user:       47/70  (67.1%)  [64 non-abs + 6 abstention]
single-session-preference: 18/30  (60.0%)
knowledge-update:          53/78  (67.9%)  [72 non-abs + 6 abstention; 2 missing cache skipped]
abstention:                10/12  (83.3%)  [directional only — small sample]

--- Overall: 128/190 (67.4%) ---
  (176 testable + 2 missing cache = 178 total;
   12 abstention included in above per-type counts)

Skipped:
  assistant_previnfo:           56 (no cache data)
  multi-session (by design):   133
  temporal-reasoning:          133 (no cache data)
  missing cache entries:         2 (knowledge_update)

Gold baseline (10 questions): 9/10 passed
  → answer LLM noise is minimal (<10%)
```

### JSON Results File

Saved to `build/benchmark/longmemeval_results_YYYYMMDD_HHMMSS.json` (aggregated from per-question JSONL crash recovery file):

```json
{
  "date": "2026-07-27",
  "scope": "single-session only",
  "total_in_scope": 176,
  "config": {
    "observer_model": "openrouter/free",
    "answer_model": "openrouter/free",
    "judge_model": "openai/gpt-4o"
  },
  "summary": {
    "total": 176,
    "passed": 118,
    "failed": 56,
    "skipped": 2,
    "accuracy": 0.670
  },
  "per_type": {
    "single-session-user":  {"total": 70, "passed": 47, "failed": 23, "accuracy": 0.671},
    "single-session-preference": {"total": 30, "passed": 18, "failed": 12, "accuracy": 0.600},
    "knowledge-update":     {"total": 78, "passed": 53, "failed": 23, "accuracy": 0.679},
    "abstention":           {"total": 12, "passed": 10, "failed": 2,  "accuracy": 0.833}
  },
  "gold_baseline": {"total": 10, "passed": 9},
  "skipped": {
    "assistant_previnfo_no_data": 56,
    "multi_session_by_design":      133,
    "temp_reasoning_no_data":       133,
    "missing_cache_entries":          2
  },
  "failures": [
    {
      "question_id": "abc123",
      "type": "single-session-user",
      "question": "What is the name of...",
      "expected_answer": "Summer Vibes",
      "actual_answer": "The playlist is called Chill Vibes",
      "judge_verdict": "no",
      "model_used": "openai/gpt-4o-mini",
      "observation_log": "... (truncated 500 chars)"
    }
  ]
}
```

**Note on counts:** `total` includes abstention questions within each type (e.g., single-session-user 70 = 64 non-abs + 6 abs). An `abstention` top-level key provides the combined total. Summing all `total` fields gives 70+30+78+12=190 = 176 + 12 abstention (already counted in per-type). The `summary.total` (176) is the unique question count, not the sum of per-type totals.

---

## Run Command

```bash
# From repo root
BENCHMARK_LONGMEMEVAL=true OPENROUTER_API_KEY=sk-or-v1-... go test \
  ./server/router/api/v1/agent/ \
  -run TestBenchmarkLongMemEval \
  -v -count=1 -timeout=2h
```

Expected runtime: ~20-40 minutes (176 questions × ~3-5s observer + ~2s answer + ~1s judge ≈ 20 min; free tier rate limits may extend).

---

## Files Changed

| File | Change | Lines |
|------|--------|-------|
| `server/.../agent/benchmark_longmemeval_test.go` | NEW — benchmark runner | ~400 |

Structure:
```
package agent

func TestBenchmarkLongMemEval(t *testing.T)
func loadBenchmarkData(t *testing.T) (questions []BenchmarkQuestion, cache map[string]CacheEntry)
func extractTurns(entry CacheEntry) []store.AgentMessage
func runBenchmarkQuestion(t *testing.T, q BenchmarkQuestion, entry CacheEntry) BenchResult
func generateAnswer(ctx context.Context, obsLog, question string) string       // openrouter/free
func judgeAnswer(ctx context.Context, q, expected, actual, qType string) bool  // GPT-4o
func runGoldBaseline(t *testing.T, questions []BenchmarkQuestion, cache map[string]CacheEntry) int
func saveResults(results []BenchResult, goldPassed int)
```

Reuses existing helpers: `convertTurnsToMessages`, `createTestSession`, `newObserverTestService`, `setOMEnvAndReload`.

---

## Comparison Baselines

Published SOTA on LongMemEval (from `Desktop/memory/emergence.md`):

| Method | Accuracy | Notes | Comparable? |
|--------|----------|-------|-------------|
| "I don't know" (baseline) | 5.8% | Floor | Yes |
| Best Guess (no history) | 18.8% | No memory | Yes |
| Naive RAG | 52.0% | Raw retrieval | Partial (RAG on full data vs OM on 1 session) |
| Full Context GPT-4o | 60-64% | All sessions | Partial (more context, but no OM summarization loss) |
| Zep (prior SOTA) | 71.2% | Commercial memory | **Partial** — Zep tested on all 500, we test 176 single-session only |
| EmergenceMem | 79-86% | SOTA RAG | Partial — RAG with full sessions |

**Fair comparison note:** Published SOTA scores include multi-session and temporal-reasoning types. This benchmark covers 176 single-session questions only. A score of ~67% on single-session is NOT directly comparable to Zep's 71% on all 500. The best reference points are "I don't know" (5.8%) and "Best Guess" (18.8%) — if bchat scores significantly above these, the OM pipeline is working. Full comparison requires expanding to all 500 questions with multi-session support.

---

## Success Criteria

- Benchmark runs to completion on all 176 testable questions
- JSON results saved to `build/benchmark/longmemeval_results_*.json`
- Per-question JSONL crash recovery file written incrementally
- Gold baseline passes ≥8/10 (answer LLM noise ≤20%)
- At least 1 question per type passes (pipeline works end-to-end)
- Failures include observation log excerpt and model name for debugging
- Nil observation logs recorded as "skipped" rather than "failed"
- No changes to production code or existing tests

---

## Known Limitations (documented)

| Limitation | Impact |
|------------|--------|
| 176/500 questions tested | Results apply to single-session types only |
| No assistant_previnfo data | 56 questions untestable |
| No reflector compression | Tests raw observation quality only |
| Session selection bias | Single session may miss evidence (mitigated for knowledge_update by using both sessions) |
| Model variability | `openrouter/free` routes to different models per call — log actual model per question for diagnosis |
| Small abstention sample | 12 questions — treat accuracy as directional |
| Generic judge prompt | Not LongMemEval's official type-specific prompts (preference rubric handled separately) |

---

## Risks

| Risk | Mitigation |
|------|------------|
| Free tier rate limits throttle the run | Sequential single-threaded. Add retry with backoff. |
| GPT-4o judge costs > $2 | Hard cap. ~$0.20-0.40 for 176 calls. |
| Session cache 415MB slow to parse | Load once, parse into `map[string]CacheEntry`. ~5s. |
| Resource-scoped OM silently uses thread scope | Validate UserID is set before RunObserver. Existing `TestEndToEndObserver_ResourceScope` confirms pattern. |
| Long run loses intermediate results | Write per-question JSONL (`*.jsonl` — append mode) for crash recovery. Final JSON aggregated at end. |
| Nil observation log evaluated as failure | Record as "skipped" instead, with error field. |

---

## Implementation Order

1. Define `BenchmarkQuestion` and `CacheEntry` structs with correct JSON tags
2. Implement `loadBenchmarkData` (questions + cache with map indexing)
3. Implement `extractTurns` (3 cache formats, priority: session > session_old > session_1)
4. Implement `runBenchmarkQuestion` with UserID fix and nil obs handling
5. Implement `generateAnswer` (openrouter/free call, log `model` field from response)
6. Implement `judgeAnswer` (GPT-4o call with 3 prompt variants for standard/preference/abstention)
7. Implement `runGoldBaseline` (10 questions: 3 single_hop, 2 preference, 3 knowledge_update, 2 abstention)
8. Implement `saveResults` (per-question JSONL append + final JSON aggregation)
9. Wire into `TestBenchmarkLongMemEval`
10. Run 5-question smoke test first (1 per type: single_hop, preference, knowledge_update, abstention)
11. Full 176-question run
