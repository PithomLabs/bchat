# Plan 9: Implementation — LongMemEval Benchmark Runner

**Status:** PLAN (ready to implement)
**Depends on:** plan8.md + plan8_answer.md

---

## Overview

Single new file: `server/router/api/v1/agent/benchmark_longmemeval_test.go`

Runs 176 LongMemEval questions through bchat's OM pipeline, generates answers, judges with GPT-4o. Parallelism optional via `BENCHMARK_PARALLEL` env.

---

## Structs

```go
type BenchmarkTurn struct {
    Role      string `json:"role"`
    Content   string `json:"content"`
    HasAnswer *bool  `json:"has_answer,omitempty"`
}

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
    QuestionID string          `json:"question_id"`
    Session    []BenchmarkTurn `json:"session,omitempty"`
    SessionOld []BenchmarkTurn `json:"session_old,omitempty"`
    SessionNew []BenchmarkTurn `json:"session_new,omitempty"`
    Session1   []BenchmarkTurn `json:"session_1,omitempty"`
    Session2   []BenchmarkTurn `json:"session_2,omitempty"`
}

type BenchResult struct {
    QuestionID      string `json:"question_id"`
    QuestionType    string `json:"question_type"`
    Question        string `json:"question"`
    ExpectedAnswer  string `json:"expected_answer"`
    ActualAnswer    string `json:"actual_answer"`
    JudgeVerdict    string `json:"judge_verdict"`    // "yes" / "no" / "skipped"
    ObservationLog  string `json:"observation_log"`
    ModelUsed       string `json:"model_used"`       // actual LLM model from API response
    Error           string `json:"error,omitempty"`
}
```

---

## Functions

### `TestBenchmarkLongMemEval(t *testing.T)`

Entry point. Gate: `BENCHMARK_LONGMEMEVAL=true`.

1. Require `OPENROUTER_API_KEY` set
2. Load benchmark data (`loadBenchmarkData`)
3. Log scope summary (176 questions, skipped types)
4. Run 4-question smoke test (1 per type)
5. Run gold baseline (10 questions)
6. Run full 176-question benchmark (parallel if `BENCHMARK_PARALLEL > 1`, default 3)
7. Aggregate results, save JSON, print summary to stdout

### `loadBenchmarkData()`

Load both JSON files. Build `map[string]CacheEntry` keyed by `question_id`. Return filtered `[]BenchmarkQuestion` (only testable types: single_hop, implicit_preference_v2, knowledge_update).

Paths from env or hardcoded:
```go
questionPath := "/home/chaschel/Desktop/memory/custom_history_data/2_questions/0822_all_500_questions_final_v2.json"
cachePath := "/home/chaschel/Desktop/memory/custom_history_data/6_session_cache/data_6_session_cache.json"
```

### `extractTurns(entry CacheEntry) []BenchmarkTurn`

Priority:
1. `entry.Session` — single_hop, implicit_preference_v2
2. `entry.SessionOld` + `entry.SessionNew` — knowledge_update (concatenate)
3. `entry.Session1` — two_hop (completeness only, not tested)
4. `nil` — no data

### `convertBenchmarkTurns(turns []BenchmarkTurn) []store.AgentMessage`

Loop: `Role`/`Content` map to `store.AgentMessage`. `HasAnswer` ignored (metadata only). Use `time.Now() + i * time.Minute` for sequential timestamps.

### `newBenchmarkLLMClient() *openrouter.Client`

```go
func newBenchmarkLLMClient(t *testing.T) *openrouter.Client {
    t.Helper()
    apiKey := os.Getenv("OPENROUTER_API_KEY")
    if apiKey == "" {
        t.Fatal("OPENROUTER_API_KEY required")
    }
    return newOpenRouterClient(apiKey)
}
```

Reuses existing `newOpenRouterClient` (same package, unexported but accessible from test code).

### `runBenchmarkQuestion(q BenchmarkQuestion, entry CacheEntry) BenchResult`

1. Create resource-scoped service (`newObserverTestService`)
2. Extract turns via `extractTurns`
3. Convert via `convertBenchmarkTurns`
4. Create session via `createTestSession`
5. Set `session.UserID = &userID` (required for resource scope)
6. Call `service.RunObserver`
7. Read `GetObservationLog`
8. If nil/empty → `BenchResult{Status: "skipped"}`
9. Extract observation log text
10. Call `generateAnswer` → actual answer
11. Call `judgeAnswer` → verdict
12. Return `BenchResult`

### `generateAnswer(obsLog, question string) string`

```go
prompt := fmt.Sprintf(`You are an AI assistant answering a question based on your memory of past conversations.

## MEMORY (Observation Log)
%s

## QUESTION
%s

Answer concisely. If the memory does not contain enough information to answer, say "I don't have enough information to answer this question."`, obsLog, question)

// Call openrouter/free with ChatCompletionRequest
// Return response.Choices[0].Message.Content
// Log response.Model to BenchResult.ModelUsed
```

### `judgeAnswer(question, expected, actual, questionType string) string`

Three prompt variants:
- Standard types: simple yes/no correctness
- Preference types: rubric-based correctness (if `questionType == "implicit_preference_v2"`)
- Abstention: check refused to answer (if questionID ends with `_abs`)

```go
var judgePrompt string
switch {
case strings.HasSuffix(questionID, "_abs"):
    judgePrompt = abstentionPrompt
case questionType == "implicit_preference_v2":
    judgePrompt = preferenceRubricPrompt
default:
    judgePrompt = standardPrompt
}

// Call gpt-4o (model: "openai/gpt-4o")
// Return "yes" if eval response contains "yes"
```

### `runGoldBaseline(questions []BenchmarkQuestion, cache map[string]CacheEntry) int`

```go
distribution := map[string]int{
    "single_hop":              3,
    "implicit_preference_v2":  2,
    "knowledge_update":        3,
}
// Pick first N of each type from filtered list
// Also include first abstention question (total = 10)
```

For each:
```go
prompt := fmt.Sprintf("Answer this question with exactly this text: %s\nQuestion: %s",
    q.QuestionContent.Answer, q.QuestionContent.Question)
// Call openrouter/free
// Compare response to expected answer (exact match or GPT-4o judge)
```

### `saveResults(results []BenchResult, goldPassed, goldTotal int)`

Write two files to `build/benchmark/`:
1. `longmemeval_results_YYYYMMDD_HHMMSS.jsonl` — per-question append during run (crash recovery)
2. `longmemeval_results_YYYYMMDD_HHMMSS.json` — final aggregated JSON

---

## Parallelism

```go
parallel := 3 // default
if v := os.Getenv("BENCHMARK_PARALLEL"); v != "" {
    if p, err := strconv.Atoi(v); err == nil && p > 0 {
        parallel = p
    }
}

semaphore := make(chan struct{}, parallel)
results := make([]BenchResult, 0, len(questions))
var mu sync.Mutex

for _, q := range questions {
    semaphore <- struct{}{}
    go func(q BenchmarkQuestion) {
        defer func() { <-semaphore }()
        result := runBenchmarkQuestion(q, cache[q.QuestionID])
        mu.Lock()
        results = append(results, result)
        appendToJSONL(result) // crash recovery
        mu.Unlock()
    }(q)
}
for i := 0; i < parallel; i++ {
    semaphore <- struct{}{} // wait for all goroutines
}
```

---

## Files Changed

| File | Change |
|------|--------|
| `server/.../agent/benchmark_longmemeval_test.go` | NEW — ~400 lines |

No changes to existing files. All dependencies are existing package-internal functions (`newOpenRouterClient`, `newObserverTestService`, `createTestSession`, `convertTurnsToMessages`).

---

## Smoke Test (before full run)

4 questions, 1 per type:
- first `single_hop` question
- first `implicit_preference_v2` question
- first `knowledge_update` question
- first abstention question

If any fails → log the error and abort the full run (pipeline is broken).

---

## Run Command

```bash
# From repo root
BENCHMARK_LONGMEMEVAL=true OPENROUTER_API_KEY=sk-or-v1-... \
  go test ./server/router/api/v1/agent/ -run TestBenchmarkLongMemEval \
  -v -count=1 -timeout=2h

# With parallelism
BENCHMARK_LONGMEMEVAL=true BENCHMARK_PARALLEL=5 \
  OPENROUTER_API_KEY=sk-or-v1-... \
  go test ./server/router/api/v1/agent/ -run TestBenchmarkLongMemEval \
  -v -count=1 -timeout=1h
```

---

## Implementation Order

1. Define structs (`BenchmarkTurn`, `BenchmarkQuestion`, `CacheEntry`, `BenchResult`)
2. `loadBenchmarkData` — load both JSON files, build map index, filter scope
3. `extractTurns` + `convertBenchmarkTurns` — handle all 3 cache formats
4. `newBenchmarkLLMClient` — wrap existing `newOpenRouterClient`
5. `generateAnswer` — prompt + openrouter/free call
6. `judgeAnswer` — 3 prompt variants + GPT-4o call
7. `runBenchmarkQuestion` — full per-question pipeline
8. `runGoldBaseline` — 10 questions with injected answer
9. `saveResults` — JSONL incremental + JSON aggregated
10. `TestBenchmarkLongMemEval` — gate, smoke, parallel orchestration, report
