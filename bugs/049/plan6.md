# Plan 6: LongMemEval Full Benchmark

**Bug/Feature:** 049
**Date:** 2026-07-27
**Status:** PLAN (not yet implemented)
**Prerequisites:** code4.md fixes (all implemented and reviewed)

---

## Goal

Answer the question: **is bchat's OM good enough?**

Run the full 500-question LongMemEval benchmark against bchat's observational memory pipeline using real LLM calls. This is a one-off validation run, not a CI test.

---

## Design Decisions (settled via Q&A)

| Decision | Choice |
|----------|--------|
| LLM for observer/reflector/answer | `openrouter/free` (free model routing) |
| LLM for judging | GPT-4o (via OpenRouter, ~$0.60 for 500 calls) |
| Answer generation | Observation log + question → LLM → answer (Approach A) |
| OM scope | Resource-scoped (`OM_SCOPE=resource`) |
| Sessions per question | 1 only (first evidence session) |
| Question types tested | single-session types only |
| Question types skipped | multi-session, temporal-reasoning (need 2+ sessions) |
| Frequency | One-off manual run |
| Output | stdout summary + JSON results file |

---

## Dataset

### Files

| File | Path | Size | Use |
|------|------|------|-----|
| Questions | `/home/chaschel/Desktop/memory/custom_history_data/2_questions/0822_all_500_questions_final_v2.json` | 461 KB | Question metadata, types, answers |
| Session cache | `/home/chaschel/Desktop/memory/custom_history_data/6_session_cache/data_6_session_cache.json` | 415 MB | Actual turn data per question |

### Data Loading

1. Load the 500-question JSON (full list)
2. For each question, look up `data_6_session_cache.json` by `question_id` to find matching cache entry
3. Extract `session_1` (list of `{role, content, has_answer}` turns)

### Question Type Mapping

LongMemEval uses different type names than bchat. Mapping:

| LongMemEval Type | bchat OM Type | Sessions | Tested? |
|-----------------|---------------|----------|---------|
| `single_hop` | `single-session-user` | 1 | Yes |
| `assistant_previnfo` | `single-session-assistant` | 1 | Yes |
| `knowledge_update` | `knowledge-update` | 1 (has both old/new in turns) | Yes |
| `implicit_preference_v2` | *(new)* `single-session-preference` | 1 | Yes |
| `two_hop` | `multi-session` | 2+ | No (needs >1 session) |
| `multi_session_synthesis` | `multi-session` | 2+ | No (needs >1 session) |
| `temp_reasoning_implicit` | `temporal-reasoning` | 2+ | No (needs >1 session) |
| `temp_reasoning_explicit` | `temporal-reasoning` | 2+ | No (needs >1 session) |

Abstention questions (question_id ends with `_abs`): tested across all types that include them (30 total). Only single-session abstention questions are tested; multi-session abstention questions are skipped.

### Questions in Scope

| Type | Non-abs | Abstention | Total | Covered |
|------|---------|------------|-------|---------|
| single-session-user | 64 | 6 | 70 | Yes |
| single-session-assistant | 56 | 0 | 56 | Yes |
| single-session-preference | 30 | 0 | 30 | Yes |
| knowledge-update | 72 | 6 | 78 | Yes |
| multi-session (2-hop) | 65 | 6 | 71 | No |
| multi-session (synthesis) | 56 | 6 | 62 | No |
| temporal-reasoning (implicit) | 73 | 0 | 73 | No |
| temporal-reasoning (explicit) | 54 | 6 | 60 | No |
| **Total** | **470** | **30** | **500** | **234 covered** |

---

## Pipeline

### Flow Per Question

```
For each question in scope:
  1. Create resource-scoped test service (OM_SCOPE=resource, OM_ENABLED=true)
  2. Load session_1 from session cache
  3. Create test session via createTestSession
  4. RunObserver → produces observation log
  5. Build answer prompt: observation log + question → openrouter/free → answer
  6. Build judge prompt: question + expected answer + actual answer → GPT-4o → yes/no
  7. Log result
```

### Step Details

**Step 1 — Service setup:**
```go
t.Setenv("OM_SCOPE", "resource")
t.Setenv("OM_ENABLED", "true")
// Use newObserverTestService (no mock)
```

**Step 4 — Observer call uses existing helpers:**
```go
sid := createTestSession(t, ctx, ts, service, tenant.ID, "lmeval-"+q.QuestionID, messages)
err := service.RunObserver(ctx, tenant.ID, sid)
obsLog, err := ts.GetObservationLog(ctx, sid)
```

**Step 5 — Answer generation prompt:**
```
You are an AI assistant answering a question based on your memory of past conversations.

## MEMORY (Observation Log)
{obsLog.ObservationLog}

## QUESTION
{q.QuestionContent.Question}

Answer concisely. If the memory does not contain enough information to answer, say "I don't have enough information to answer this question."
```

Model: `openrouter/free`

**Step 6 — Judge prompt** (LongMemEval's official evaluation template per type):

For standard types:
```
You are evaluating a model's answer. Determine if the answer is correct.
Question: {question}
Correct answer: {expected_answer}
Model answer: {actual_answer}

Is the model's answer correct? Answer "yes" or "no".
```

For abstention:
```
You are evaluating a model's answer. Determine if the model correctly identified the question as unanswerable.
Question: {question}
The correct response is that the question cannot be answered based on available information.
Model answer: {actual_answer}

Did the model correctly refuse to answer? Answer "yes" if it identifies the question as unanswerable or says it doesn't have enough information. Answer "no" if it hallucinated an answer.
```

Model: `openai/gpt-4o` via OpenRouter (this is the one paid call per question)

---

## Report Format

### stdout Summary

```
=== LongMemEval Benchmark Results ===
Date: 2026-07-27
Model (observer/reflector/answer): openrouter/free
Model (judge): openai/gpt-4o

single-session-user:       42/64  (65.6%)
single-session-assistant:  31/56  (55.4%)
single-session-preference: 18/30  (60.0%)
knowledge-update:          48/72  (66.7%)
abstention:                24/30  (80.0%)
--- Overall: 163/252 (64.7%) ---

Skipped (need 2+ sessions): 248
  multi-session (2-hop): 71
  multi-session (synthesis): 62
  temporal-reasoning (implicit): 73
  temporal-reasoning (explicit): 60
```

### JSON Results File

Saved to `build/data/longmemeval_results.json`:

```json
{
  "date": "2026-07-27",
  "config": {
    "passes": 1,
    "observer_model": "openrouter/free",
    "judge_model": "openai/gpt-4o"
  },
  "summary": {
    "total": 234,
    "passed": 163,
    "failed": 71,
    "accuracy": 0.697
  },
  "per_type": {
    "single-session-user": {"total": 64, "passed": 42, "failed": 22, "accuracy": 0.656},
    "single-session-assistant": {"total": 56, "passed": 31, "failed": 25, "accuracy": 0.554},
    "single-session-preference": {"total": 30, "passed": 18, "failed": 12, "accuracy": 0.600},
    "knowledge-update": {"total": 72, "passed": 48, "failed": 24, "accuracy": 0.667},
    "abstention": {"total": 30, "passed": 24, "failed": 6, "accuracy": 0.800}
  },
  "failures": [
    {
      "question_id": "abc123",
      "type": "single-session-user",
      "question": "What is the name of...",
      "expected_answer": "Summer Vibes",
      "actual_answer": "The playlist is called Chill Vibes",
      "judge_verdict": "no",
      "observation_log": "... (truncated)"
    }
  ]
}
```

---

## Run Command

```bash
# From repo root
BENCHMARK_LONGMEMEVAL=true OPENROUTER_API_KEY=sk-or-v1-... go test \
  ./server/router/api/v1/agent/ \
  -run TestBenchmarkLongMemEval \
  -v -count=1 -timeout=2h
```

Expected runtime: ~1 hour on free tier (~2s per observer call + ~2s per answer call + ~1s per judge call = 234 × 5s ≈ 20 minutes at best; free tier rate limits may extend to 1h+).

---

## Files Changed

| File | Change | Lines |
|------|--------|-------|
| `server/.../agent/benchmark_longmemeval_test.go` | NEW — benchmark runner | ~350 |
| `server/.../agent/observer_test_helpers_test.go` | Add `makeMessagesFromTurns` helper if needed | ~10 |

### New file: `benchmark_longmemeval_test.go`

Structure:
```
package agent

func TestBenchmarkLongMemEval(t *testing.T)            // Entry point, gate + orchestration
func loadBenchmarkQuestions(t *testing.T) ([]BenchmarkQuestion, error)  // Load questions + cache
func filterSingleSessionQuestions(questions) []BenchmarkQuestion         // Keep testable only
func runQuestionBenchmark(t *testing.T, q BenchmarkQuestion) BenchResult // Per-question pipeline
func generateAnswer(ctx context.Context, obsLog, question string) string // openrouter/free call
func judgeAnswer(ctx context.Context, q, expected, actual string) bool   // GPT-4o call
func saveBenchmarkResults(results []BenchResult)                         // Write JSON to disk
```

---

## Comparison Baselines

Published SOTA on LongMemEval (from `Desktop/memory/emergence.md`):

| Method | Accuracy | Notes |
|--------|----------|-------|
| "I don't know" (baseline) | 5.8% | Floor |
| Best Guess (no history) | 18.8% | No memory at all |
| Naive RAG | 52.0% | Raw retrieval |
| Full Context GPT-4o | 60-64% | All sessions in context |
| Zep (prior SOTA) | 71.2% | Commercial memory product |
| EmergenceMem Simple Fast | 79.0% | SOTA RAG |
| EmergenceMem Internal | 86.0% | SOTA with training |

**What "good enough" means:** Zep (71.2%) is the direct comparison — another commercial memory product. If bchat OM scores near or above 71%, it's competitive. If below 60%, OM summarization is losing too much detail and the prompts need work.

---

## Success Criteria

- Benchmark runs to completion on all 234 in-scope questions
- JSON results file saved to `build/data/longmemeval_results.json`
- At least 1 question per type passes (pipeline works)
- Failures file includes actual observation log for debugging
- No changes to production code or existing tests

---

## Risks

| Risk | Mitigation |
|------|------------|
| Free tier rate limits throttle the run | Start with `-count=1`, single-threaded. Add retry with backoff if rate limited. |
| GPT-4o judge costs more than expected | Log token usage. Hard cap at $2 max spend. |
| Session cache (415MB) is slow to load | Load once into memory at startup. JSON parsing of 16K entries takes ~2-5s. |
| Resource-scoped OM doesn't accumulate correctly | Test with 1 known question first before full run. |
| Long run loses intermediate results | Write per-question results to file incrementally, not just at the end. |

---

## Adversarial Plan Review Prompt

Before implementing, review this plan against the following specific concerns. For each, identify whether it is **🔴 blocking** (results would be meaningless), **🟡 significant** (must document in report), or **🟢 minor** (note but doesn't affect validity).

### 1. Single-session only — does this answer the question?

The plan tests 234/500 questions with 1 session per question. LongMemEval's SOTA scores (Zep 71%, Emergence 79%) include multi-session and temporal-reasoning types. If bchat scores 65% on single-session types, does that mean it is "good enough"? The comparison is apples-to-oranges.

### 2. Session selection bias

We pick `session_1` from the cache. Not all questions have evidence in `session_1` — some have the answer only in `session_2`. If we guess wrong on 20% of questions, the max possible accuracy drops to 80% regardless of OM quality. How do we verify which session contains the evidence?

### 3. No reflector in the pipeline

The plan runs the observer once per question. The reflector (compression) is never triggered because a single session produces far fewer tokens than `OM_TOKEN_THRESHOLD` (default 2000). The benchmark tests detail preservation in raw observations only, not post-compression. Is that acceptable for answering "is OM good enough"?

### 4. Resource-scoped OM readiness

The plan sets `OM_SCOPE=resource`, but the existing test helpers and observer are primarily tested with `thread` scope. Has resource-scoped OM been validated end-to-end? The `GetObservationLogByResource` method may behave differently from `GetObservationLog` in ways that affect results.

### 5. Answer generation confound

If the answer is wrong, is it because:
- OM lost the detail (memory failure), or
- The answer LLM could not find the detail in the observation (reasoning failure), or
- The answer LLM did not follow instructions (prompt failure)?

The plan treats all failures as OM failures, but the answer LLM adds noise. How do we distinguish these failure modes?

### 6. GPT-4o judge reliability

LongMemEval's official judge uses task-specific prompts per type (e.g., "do not penalize off-by-one errors for days"). The plan uses a generic judge prompt. If the judge is too strict or too lenient, the accuracy numbers shift. Is an unofficial judge acceptable?

### 7. Data loading cost without indexing

The session cache (415MB) has 16K entries. Without an index, a naive scan for 234 matching IDs means processing 16K × 234 = 3.7M comparisons. Building an in-memory `map[string]CacheEntry` avoids this, but the plan does not specify the lookup strategy.

### 8. Free tier model variability

`openrouter/free` routes to different models per request. The observer call may use Llama 3.1 70B while the answer call uses Gemini 2.0 Flash. The observer's detail preservation quality varies by model. Should we lock a specific free model?

### 9. Abstention test significance

30 abstention questions exist across single-session and multi-session types. The plan tests only the single-session ones. If only 6 abstention questions are in scope (from single_hop + knowledge_update), 4-5 correct answers is too few for statistical significance.

### 10. Fair comparison with SOTA

If bchat scores 65% on 234 single-session questions, and Zep scored 71% on all 500 questions, is 65% better or worse? We cannot directly compare. The plan should either run LongMemEval's oracle mode (evidence sessions only) to match the single-session constraint, or adjust baseline numbers to single-session-only for fair comparison.
