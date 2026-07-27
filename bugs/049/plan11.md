# Plan 11: LongMemEval Per-Type Benchmark Architecture (Revised)

**Date:** 2026-07-28
**Depends on:** plan8.md, plan9.md, plan_dryrun2.md
**Status:** PLAN (ready to implement)
**Revision of:** plan10.md (addressed S1, N1, N2, N3 from review)

---

## Goal

Convert the 176-question LongMemEval benchmark from a single hour-long test into
independent per-type tests with crash recovery, so no single failure loses all results.

---

## Background: OM vs RAG vs Both

Understanding what each system solves is critical for interpreting benchmark results.

### What Each Does

| Aspect | OM | RAG |
|--------|-----|-----|
| Input | Conversation turns (user/assistant messages) | Static documents (KB.MD, POLICY.MD, SCRIPT.MD) |
| Process | Summarizes into structured observation log | Chunks → embeds → vector search → retrieve |
| Output | Bullet points with emoji priority signals | Relevant document chunks |
| Data source | What users actually said | What the tenant configured as knowledge |

### In Isolation

**OM Only:**
| Pros | Cons |
|------|------|
| No vector DB needed | Only captures conversation-derived facts |
| Works with zero configuration | Struggles with cross-session synthesis |
| Compresses long histories efficiently | Misses policy/procedural knowledge |
| Natural language observations are inspectable | Token threshold triggers can miss nuance |
| No embeddings, no indexing latency | No retrieval of static reference material |

**RAG Only:**
| Pros | Cons |
|------|------|
| Fast retrieval at query time | Only knows what's in uploaded docs |
| Scales to large knowledge bases | Can't answer "what did we discuss last time?" |
| Deterministic (same query → same retrieval) | Needs manual doc maintenance |
| Hybrid search (vector + BM25) is robust | No memory of past conversations |
| Works for policy/procedural queries | Irrelevant if question is about conversation history |

### When Used Together

```
User Query
    ↓
RAG retrieval (KB/Policy/Script chunks)
    ↓
OM observation log (conversation history)
    ↓
Combined context → LLM → Answer
```

| Pros | Cons |
|------|------|
| Answers both "what did we discuss" AND "what's our policy" | More infrastructure (vector DB + OM) |
| Complete coverage of tenant knowledge | Higher latency per query |
| Policy-aware responses grounded in conversation | More failure modes |
| OM captures dynamic facts RAG can't index | RAG docs need manual curation |

### Does Good OM Performance Mean RAG is Unnecessary?

**No.** LongMemEval tests conversation recall only — zero policy/procedural questions.
A 90% OM score says nothing about RAG's value. Different question types need different sources:
- *"What did the customer say about pricing?"* → OM
- *"What is our pricing policy?"* → RAG
- *"Based on our policy and what the customer said, what should we offer?"* → Both

The benchmark validates OM as one component of the full system, not a replacement for RAG.

---

## Current Situation

| Metric | Value |
|--------|-------|
| Testable questions | 176 |
| Time per question | ~16s (rate-limited openrouter/free) |
| Single test function estimate | ~47 min |
| Single test function risk | One crash = all results lost |

---

## Architecture: Per-Type Tests with Checkpoints

Split the benchmark into 4 independent test functions — one per question type.
Abstention questions are filtered OUT of SingleHop and KnowledgeUpdate, run only once in TestBenchmarkAbstention.

| Test Function | Questions | Est. Time | Can Run Alone |
|---------------|-----------|-----------|---------------|
| `TestBenchmarkSingleHop` | 64 (non-abs only) | ~17 min | ✅ |
| `TestBenchmarkPreference` | 30 | ~8 min | ✅ |
| `TestBenchmarkKnowledgeUpdate` | 72 (non-abs only, minus 2 missing cache) | ~19 min | ✅ |
| `TestBenchmarkAbstention` | 12 (6 from SingleHop + 6 from KnowledgeUpdate) | ~3 min | ✅ |

**Unique question count: 64 + 30 + 72 + 12 = 178, minus 2 missing cache = 176**

Each test function:
1. Loads its own data (questions + cache)
2. Filters to its specific type (abstention excluded from SingleHop/KnowledgeUpdate)
3. Runs questions sequentially
4. Writes results to JSONL after each question (crash recovery)
5. Aggregates to JSON at the end
6. Reports pass/fail/skip per question

---

## Implementation Structure

```
benchmark_longmemeval_test.go
├── shared structs (BenchmarkQuestion, CacheEntry, etc.)
├── shared helpers (loadBenchmarkData, extractTurns, etc.)
├── TestBenchmarkSingleHop       ← per-type test (non-abs only)
├── TestBenchmarkPreference      ← per-type test
├── TestBenchmarkKnowledgeUpdate ← per-type test (non-abs only)
├── TestBenchmarkAbstention      ← per-type test (12 questions)
└── TestBenchmarkAggregate       ← reads JSONL, prints summary
```

### Run Commands

```bash
# Full benchmark (all types, sequential)
go test -run "TestBenchmark(SingleHop|Preference|KnowledgeUpdate|Abstention)" -v

# Individual type (for debugging)
go test -run TestBenchmarkSingleHop -v

# Aggregated summary
go test -run TestBenchmarkAggregate -v

# Parallel execution
go test -run TestBenchmarkSingleHop &
go test -run TestBenchmarkPreference &
go test -run TestBenchmarkKnowledgeUpdate &
wait
go test -run TestBenchmarkAbstention
```

### Crash Recovery

Each test writes to its own JSONL file with timestamp to avoid same-day collision:
- `build/benchmark/single_hop_YYYYMMDD_HHMMSS.jsonl`
- `build/benchmark/preference_YYYYMMDD_HHMMSS.jsonl`
- `build/benchmark/knowledge_update_YYYYMMDD_HHMMSS.jsonl`
- `build/benchmark/abstention_YYYYMMDD_HHMMSS.jsonl`

If a test crashes, re-run it — it skips already-completed questions by reading the JSONL.

---

## Shared Helpers

All helpers are reusable from dry run implementation:

| Function | Purpose |
|----------|---------|
| `loadBenchmarkData` | Load questions + cache from JSON files |
| `extractTurns` | Extract conversation turns (3 cache formats) |
| `convertBenchmarkTurns` | BenchmarkTurn → AgentMessage |
| `generateAnswerDryRun` | openrouter/free answer generation |
| `judgeAnswerDryRun` | openai/gpt-4o judge with 3 prompt variants |
| `newBenchmarkLLMClient` | Create openrouter client from env |
| `appendJSONL` | Write result to JSONL (crash recovery) |
| `loadCompletedFromJSONL` | Read JSONL, return completed question IDs |
| `filterByType` | Filter questions by type, excluding abstention from parent types |

---

## Per-Type Test Flow

```go
func TestBenchmarkSingleHop(t *testing.T) {
    if os.Getenv("BENCHMARK_LONGMEMEVAL") != "true" {
        t.Skip("Set BENCHMARK_LONGMEMEVAL=true")
    }
    apiKey := os.Getenv("OPENROUTER_API_KEY")
    require.NotEmpty(t, apiKey)

    questions, cache := loadBenchmarkData(t)
    typeQuestions := filterByType(questions, "single_hop", false) // false = exclude abstention

    jsonlPath := fmt.Sprintf("build/benchmark/single_hop_%s.jsonl", time.Now().Format("20060102_150405"))
    completed := loadCompletedFromJSONL(jsonlPath)

    var results []BenchResult
    for i, q := range typeQuestions {
        if completed[q.QuestionID] {
            t.Logf("[%d/%d] SKIP %s (already completed)", i+1, len(typeQuestions), q.QuestionID)
            continue
        }

        result := runBenchmarkQuestion(t, apiKey, q, cache[q.QuestionID])
        results = append(results, result)
        appendJSONL(jsonlPath, result) // crash recovery

        t.Logf("[%d/%d] %s %s — %s", i+1, len(typeQuestions), q.QuestionID, q.QuestionType, result.JudgeVerdict)
    }

    saveResults(results, "single_hop")
}
```

---

## filterByType Logic

```go
func filterByType(questions []BenchmarkQuestion, qType string, includeAbs bool) []BenchmarkQuestion {
    var result []BenchmarkQuestion
    for _, q := range questions {
        if q.QuestionType != qType {
            continue
        }
        if !includeAbs && strings.HasSuffix(q.QuestionID, "_abs") {
            continue
        }
        result = append(result, q)
    }
    return result
}
```

---

## JSONL Crash Recovery Format

Each line is a complete JSON object:

```json
{"question_id":"6a1eabeb","question_type":"knowledge_update","question":"What was my personal best time?","expected_answer":"25:50","actual_answer":"25:50","judge_verdict":"yes","model_used":"nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free","status":"pass"}
```

---

## Aggregation

`TestBenchmarkAggregate` reads all JSONL files and prints the final summary.
Denominator uses unique question count (176), not sum of per-type totals.

```
=== LongMemEval Benchmark Results ===
Date: 2026-07-28
Model (answer): openrouter/free (varies per call)
Model (judge): openai/gpt-4o

single-hop:              47/64  (73.4%)
implicit-preference:     18/30  (60.0%)
knowledge-update:        53/72  (73.6%)
abstention:              10/12  (83.3%)

--- Overall: 128/176 (72.7%) ---

Note: Abstention results (12 questions) are a subset of single-hop and knowledge-update.
Gold baseline deferred — run TestBenchmarkLongMemEvalDryRun first if accuracy is unexpectedly low.
```

---

## Gold Baseline Rationale

Gold baseline from plan8 is **deferred to the dry run**. The dry run's 4-question test includes observer + answer + judge, validating the answer LLM's reliability. If the full benchmark shows unexpectedly low accuracy (<50%), run `TestBenchmarkLongMemEvalDryRun` first to diagnose whether the issue is OM or the answer LLM.

This avoids 10 additional paid API calls ($0.021) in the per-type benchmark while maintaining diagnostic capability.

---

## API Calls

| Call | Model | Count | Cost |
|------|-------|-------|------|
| Observer | openrouter/free | 176 | $0 |
| Answer | openrouter/free | 176 | $0 |
| Judge | openai/gpt-4o | 176 | ~$0.39 |
| **Total** | | 528 | **~$0.39** |

---

## Files Changed

| File | Change | Lines |
|------|--------|-------|
| `server/.../agent/benchmark_longmemeval_test.go` | Refactor to per-type tests, add JSONL recovery, add aggregate test | ~300 |

---

## Why Per-Type Tests

| Criterion | Single Test | Per-Type Tests |
|-----------|-------------|----------------|
| Crash resilience | ❌ All lost | ✅ Only current type lost |
| Debuggability | ❌ Run entire 47min | ✅ Run single type (3-20min) |
| Granular metrics | ❌ One summary | ✅ Per-type breakdown |
| Parallelism | ❌ Single process | ✅ Run types simultaneously |
| Code complexity | ✅ Simpler | ⚠️ More functions |
| Maintenance | ✅ One function | ⚠️ Four functions |

The tradeoff is worth it for resilience and debuggability.

---

## Implementation Order

1. Extract shared helpers into package-level functions (already done in dry run)
2. Add `filterByType` helper (exclude abstention from parent types)
3. Implement `TestBenchmarkSingleHop` with JSONL recovery
4. Implement `TestBenchmarkPreference` (same pattern)
5. Implement `TestBenchmarkKnowledgeUpdate` (same pattern)
6. Implement `TestBenchmarkAbstention` (same pattern)
7. Implement `TestBenchmarkAggregate` (read JSONL, print summary)
8. Verify compilation
9. Run dry run first (4 questions), then full benchmark
