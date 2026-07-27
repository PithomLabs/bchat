# Plan 10: LongMemEval Per-Type Benchmark Architecture

**Date:** 2026-07-28
**Depends on:** plan8.md, plan9.md, plan_dryrun2.md
**Status:** PLAN (ready to implement)

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

Split the benchmark into 4 independent test functions — one per question type:

| Test Function | Questions | Est. Time | Can Run Alone |
|---------------|-----------|-----------|---------------|
| `TestBenchmarkSingleHop` | 70 | ~18 min | ✅ |
| `TestBenchmarkPreference` | 30 | ~8 min | ✅ |
| `TestBenchmarkKnowledgeUpdate` | 76 | ~20 min | ✅ |
| `TestBenchmarkAbstention` | 12 | ~3 min | ✅ |

Each test function:
1. Loads its own data (questions + cache)
2. Runs questions sequentially
3. Writes results to JSONL after each question (crash recovery)
4. Aggregates to JSON at the end
5. Reports pass/fail/skip per question

---

## Implementation Structure

```
benchmark_longmemeval_test.go
├── shared structs (BenchmarkQuestion, CacheEntry, etc.)
├── shared helpers (loadBenchmarkData, extractTurns, etc.)
├── TestBenchmarkSingleHop       ← per-type test
├── TestBenchmarkPreference      ← per-type test
├── TestBenchmarkKnowledgeUpdate ← per-type test
├── TestBenchmarkAbstention      ← per-type test
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

Each test writes to its own JSONL file:
- `build/benchmark/single_hop_YYYYMMDD.jsonl`
- `build/benchmark/preference_YYYYMMDD.jsonl`
- `build/benchmark/knowledge_update_YYYYMMDD.jsonl`
- `build/benchmark/abstention_YYYYMMDD.jsonl`

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
| `aggregateJSONL` | Read JSONL, compute pass/fail/skip |

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
    typeQuestions := filterByType(questions, "single_hop")

    jsonlPath := fmt.Sprintf("build/benchmark/single_hop_%s.jsonl", time.Now().Format("20060102"))
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

## JSONL Crash Recovery Format

Each line is a complete JSON object:

```json
{"question_id":"6a1eabeb","question_type":"knowledge_update","question":"What was my personal best time?","expected_answer":"25:50","actual_answer":"25:50","judge_verdict":"yes","model_used":"nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free","status":"pass"}
```

The `loadCompletedFromJSONL` function reads existing results and skips completed questions on re-run.

---

## Aggregation

`TestBenchmarkAggregate` reads all JSONL files and prints the final summary:

```
=== LongMemEval Benchmark Results ===
Date: 2026-07-28
Model (answer): openrouter/free (varies per call)
Model (judge): openai/gpt-4o

single-hop:              47/70  (67.1%)
implicit-preference:     18/30  (60.0%)
knowledge-update:        53/76  (69.7%)
abstention:              10/12  (83.3%)

--- Overall: 128/188 (68.1%) ---
```

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
2. Implement `TestBenchmarkSingleHop` with JSONL recovery
3. Implement `TestBenchmarkPreference` (same pattern)
4. Implement `TestBenchmarkKnowledgeUpdate` (same pattern)
5. Implement `TestBenchmarkAbstention` (same pattern)
6. Implement `TestBenchmarkAggregate` (read JSONL, print summary)
7. Verify compilation
8. Run dry run first (4 questions), then full benchmark
