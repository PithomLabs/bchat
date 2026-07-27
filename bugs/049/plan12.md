# Plan 12: LongMemEval Per-Type Benchmark (Final)

**Date:** 2026-07-28
**Depends on:** plan8.md, plan9.md, plan_dryrun2.md, plan11.md
**Status:** READY TO IMPLEMENT

---

## Current State

All helpers are implemented. Only 5 test functions remain to be written.

### Implemented (benchmark_longmemeval_test.go)
| Component | Lines | Status |
|-----------|-------|--------|
| Structs (BenchmarkQuestion, CacheEntry, BenchResult, DryRunResult) | 20-89 | ✅ |
| Data loading (loadBenchmarkDataDryRun) | 93-134 | ✅ |
| Cache extraction (extractTurnsDryRun, convertBenchmarkTurns) | 137-165 | ✅ |
| LLM calls (generateAnswerDryRun, judgeAnswerDryRun) | 168-247 | ✅ |
| Question picker (pickDryRunQuestions) | 250-300 | ✅ |
| JSONL crash recovery (appendJSONL, loadCompletedFromJSONL, benchmarkJSONLPath, clearBenchmarkJSONLs) | 302-359 | ✅ |
| Filter helpers (filterByType, filterAbstention) | 363-385 | ✅ |
| Dry run output (printDryRunHeader, printDryRunTurns, printDryRunResult, writeDryRunReport) | 388-489 | ✅ |
| Dry run test (TestBenchmarkLongMemEvalDryRun) | 493-641 | ✅ |

### Not Yet Implemented
| Component | Status |
|-----------|--------|
| `runBenchmarkQuestion` — shared per-question pipeline | ❌ |
| `TestBenchmarkSingleHop` — 64 non-abs single_hop questions | ❌ |
| `TestBenchmarkPreference` — 30 implicit_preference_v2 questions | ❌ |
| `TestBenchmarkKnowledgeUpdate` — 72 non-abs knowledge_update questions | ❌ |
| `TestBenchmarkAbstention` — 12 _abs questions (6 from single_hop + 6 from knowledge_update) | ❌ |
| `TestBenchmarkAggregate` — reads all JSONLs, computes summary | ❌ |

---

## Architecture

### Per-Type Test Flow (each test function)

```
1. Gate check (BENCHMARK_LONGMEMEVAL=true, OPENROUTER_API_KEY)
2. Load questions + cache (loadBenchmarkDataDryRun)
3. Filter to type (filterByType or filterAbstention)
4. Load JSONL (loadCompletedFromJSONL) — skip completed questions
5. For each question:
   a. Skip if already in JSONL
   b. Run observer pipeline (newObserverTestService → createTestSession → RunObserver → GetObservationLog)
   c. Generate answer (generateAnswerDryRun — openrouter/free)
   d. Judge answer (judgeAnswerDryRun — openai/gpt-4o)
   e. Append to JSONL (appendJSONL)
   f. Log progress
6. Print summary (pass/fail/skip counts)
```

### Aggregate Test Flow

```
1. Read all 4 JSONL files (single_hop, preference, knowledge_update, abstention)
2. For each type: count pass/fail/skip
3. Deduplicate: abstention questions are a subset of single_hop + knowledge_update
4. Compute unique total = sum(all non-abs types) + abstention count
5. Print per-type and overall summary
```

---

## Implementation: `runBenchmarkQuestion`

Shared helper that encapsulates the observer→answer→judge pipeline for a single question. Returns `BenchResult`.

```go
func runBenchmarkQuestion(t *testing.T, apiKey, sessionID string, q BenchmarkQuestion, cacheEntry CacheEntry) BenchResult {
    t.Helper()

    result := BenchResult{
        QuestionID:     q.QuestionID,
        QuestionType:   q.QuestionType,
        Question:       q.QuestionString(),
        ExpectedAnswer: q.AnswerString(),
        Status:         "pending",
    }

    // Extract and validate turns
    turns := extractTurnsDryRun(cacheEntry)
    if turns == nil || len(turns) < 2 {
        result.Status = "skipped"
        result.Error = fmt.Sprintf("insufficient turns: %d", len(turns))
        return result
    }

    // Run observer
    msgs := convertBenchmarkTurns(turns)
    ctx, ts, service, tenant := newObserverTestService(t, sessionID)
    defer ts.Close()

    setOMEnvAndReload(t, "OM_SCOPE", "resource")
    setOMEnvAndReload(t, "OM_ENABLED", "true")

    sid := createTestSession(t, ctx, ts, service, tenant.ID, sessionID, msgs)
    var userID int32 = 999
    memSession := service.memorySessions.Get(tenant.ID, sid)
    memSession.UserID = &userID
    service.memorySessions.Update(memSession)

    if err := service.RunObserver(ctx, tenant.ID, sid); err != nil {
        result.Status = "skipped"
        result.Error = fmt.Sprintf("observer error: %v", err)
        return result
    }

    obsLog, err := ts.GetObservationLog(ctx, sid)
    if err != nil || obsLog == nil || obsLog.ObservationLog == "" {
        result.Status = "skipped"
        result.Error = "no observation log produced"
        return result
    }

    // Generate answer
    answer, model, err := generateAnswerDryRun(t, ctx, apiKey, obsLog.ObservationLog, q.QuestionString())
    if err != nil {
        result.Status = "skipped"
        result.Error = fmt.Sprintf("answer error: %v", err)
        return result
    }
    result.ActualAnswer = answer
    result.ModelUsed = model

    // Judge answer
    verdict, err := judgeAnswerDryRun(t, ctx, apiKey, q.QuestionString(), result.ExpectedAnswer, answer, q.QuestionID, q.QuestionType)
    if err != nil {
        result.Status = "skipped"
        result.Error = fmt.Sprintf("judge error: %v", err)
        return result
    }
    result.JudgeVerdict = verdict
    if verdict == "yes" {
        result.Status = "pass"
    } else {
        result.Status = "fail"
    }

    return result
}
```

---

## Implementation: Per-Type Tests

Each test follows the same pattern. Only the filter and type name differ.

### TestBenchmarkSingleHop

```go
func TestBenchmarkSingleHop(t *testing.T) {
    if os.Getenv("BENCHMARK_LONGMEMEVAL") != "true" {
        t.Skip("Set BENCHMARK_LONGMEMEVAL=true")
    }
    apiKey := os.Getenv("OPENROUTER_API_KEY")
    require.NotEmpty(t, apiKey)

    if os.Getenv("BENCHMARK_FRESH") == "true" {
        clearBenchmarkJSONLs()
    }

    questions, cache := loadBenchmarkDataDryRun(t)
    typeQuestions := filterByType(questions, "single_hop", false) // exclude _abs
    t.Logf("SingleHop: %d questions (non-abs)", len(typeQuestions))

    jsonlPath := benchmarkJSONLPath("single_hop")
    completed := loadCompletedFromJSONL(jsonlPath)

    var results []BenchResult
    for i, q := range typeQuestions {
        if completed[q.QuestionID] {
            t.Logf("[%d/%d] SKIP %s (already completed)", i+1, len(typeQuestions), q.QuestionID)
            continue
        }
        entry, ok := cache[q.QuestionID]
        if !ok {
            t.Logf("[%d/%d] SKIP %s (no cache)", i+1, len(typeQuestions), q.QuestionID)
            continue
        }

        result := runBenchmarkQuestion(t, apiKey, fmt.Sprintf("sh-%s", q.QuestionID), q, entry)
        results = append(results, result)
        appendJSONL(jsonlPath, result)
        t.Logf("[%d/%d] %s — %s", i+1, len(typeQuestions), q.QuestionID, result.JudgeVerdict)
    }

    passed, failed, skipped := countResults(results)
    t.Logf("=== SingleHop: %d passed, %d failed, %d skipped ===", passed, failed, skipped)
}
```

### TestBenchmarkPreference

```go
func TestBenchmarkPreference(t *testing.T) {
    // Same pattern, filterByType(questions, "implicit_preference_v2", false)
    // jsonlPath: benchmarkJSONLPath("preference")
    // session prefix: "pref-"
}
```

### TestBenchmarkKnowledgeUpdate

```go
func TestBenchmarkKnowledgeUpdate(t *testing.T) {
    // Same pattern, filterByType(questions, "knowledge_update", false)
    // jsonlPath: benchmarkJSONLPath("knowledge_update")
    // session prefix: "ku-"
}
```

### TestBenchmarkAbstention

```go
func TestBenchmarkAbstention(t *testing.T) {
    // Uses filterAbstention(questions) instead of filterByType
    // jsonlPath: benchmarkJSONLPath("abstention")
    // session prefix: "abs-"
}
```

---

## Implementation: `TestBenchmarkAggregate`

```go
func TestBenchmarkAggregate(t *testing.T) {
    types := []string{"single_hop", "preference", "knowledge_update", "abstention"}
    typeLabels := map[string]string{
        "single_hop":       "SingleHop",
        "preference":       "Preference",
        "knowledge_update": "KnowledgeUpdate",
        "abstention":       "Abstention",
    }

    var allResults []BenchResult
    for _, qType := range types {
        jsonlPath := benchmarkJSONLPath(qType)
        results := readJSONL(jsonlPath)
        passed, failed, skipped := countResults(results)
        label := typeLabels[qType]
        if len(results) > 0 {
            t.Logf("%-20s %d/%d passed (%.1f%%)", label, passed, len(results), 100*float64(passed)/float64(len(results)))
        } else {
            t.Logf("%-20s (no results)", label)
        }
        allResults = append(allResults, results...)
    }

    // Deduplicate: abstention is a subset of single_hop + knowledge_update
    abstentionCount := len(readJSONL(benchmarkJSONLPath("abstention")))
    uniqueTotal := len(allResults) - abstentionCount
    passed, _, _ := countResults(allResults)
    t.Logf("--- Overall: %d/%d unique questions passed (%.1f%%) ---", passed, uniqueTotal, 100*float64(passed)/float64(uniqueTotal))
}
```

### `readJSONL` helper

```go
func readJSONL(path string) []BenchResult {
    var results []BenchResult
    data, err := os.ReadFile(path)
    if err != nil {
        return results
    }
    dec := json.NewDecoder(strings.NewReader(string(data)))
    for dec.More() {
        var r BenchResult
        if err := dec.Decode(&r); err != nil {
            continue
        }
        results = append(results, r)
    }
    return results
}
```

### `countResults` helper

```go
func countResults(results []BenchResult) (passed, failed, skipped int) {
    for _, r := range results {
        switch r.Status {
        case "pass": passed++
        case "fail": failed++
        case "skipped": skipped++
        }
    }
    return
}
```

---

## Dynamic Denominator (from review)

The aggregate test computes the denominator from actual results, not hardcoded 176.

Unique questions = (non-abs single_hop) + (non-abs preference) + (non-abs knowledge_update) + (all abstention)

This handles missing cache entries automatically — if 2 knowledge_update questions lack cache, denominator drops from 176 to 174.

---

## Run Commands

```bash
# Full benchmark (all types, sequential)
cd /home/chaschel/Documents/go/bchat
BENCHMARK_LONGMEMEVAL=true OPENROUTER_API_KEY=<key> \
  go test ./server/router/api/v1/agent/ \
  -run "TestBenchmark(SingleHop|Preference|KnowledgeUpdate|Abstention)" \
  -v -timeout=60m -count=1

# Individual type (for debugging)
BENCHMARK_LONGMEMEVAL=true OPENROUTER_API_KEY=<key> \
  go test ./server/router/api/v1/agent/ \
  -run TestBenchmarkSingleHop \
  -v -timeout=30m -count=1

# Aggregated summary
BENCHMARK_LONGMEMEVAL=true \
  go test ./server/router/api/v1/agent/ \
  -run TestBenchmarkAggregate \
  -v -count=1

# Fresh run (clear existing JSONLs)
BENCHMARK_FRESH=true BENCHMARK_LONGMEMEVAL=true \
  go test ./server/router/api/v1/agent/ \
  -run TestBenchmarkSingleHop \
  -v -timeout=30m -count=1
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

## Implementation Order

1. Add `readJSONL` and `countResults` helpers
2. Add `runBenchmarkQuestion` shared pipeline
3. Implement `TestBenchmarkSingleHop`
4. Implement `TestBenchmarkPreference`
5. Implement `TestBenchmarkKnowledgeUpdate`
6. Implement `TestBenchmarkAbstention`
7. Implement `TestBenchmarkAggregate`
8. Verify compilation (`go vet`)
9. Run dry run first (4 questions), then full benchmark

---

## Files Changed

| File | Change | Lines Added |
|------|--------|-------------|
| `server/.../agent/benchmark_longmemeval_test.go` | Add 5 test functions + 2 helpers | ~150 |
