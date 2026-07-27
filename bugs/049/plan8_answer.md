# Plan 8 — Answers

## 1. LLM client for answer generation and judging

**Answer: (A) — Reuse existing infrastructure.**

Both `newOpenRouterClient(apiKey)` (service.go:58) and `newLLMClient(timeout)` (lead_llm.go:42) are unexported but accessible from test code in the same `agent` package. The openrouter client's `CreateChatCompletion` method is all we need.

```go
// In benchmark_longmemeval_test.go
func newBenchmarkLLMClient() *openrouter.Client {
    apiKey := os.Getenv("OPENROUTER_API_KEY")
    if apiKey == "" {
        // skip test
    }
    return newOpenRouterClient(apiKey)
}
```

Then call `client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{...})` directly. No new HTTP helper needed — the openrouter package already handles retries, timeouts, and the API base URL override (`OPENROUTER_API_BASE_URL` for mocks).

---

## 2. CacheEntry turn format — `has_answer` field

**Answer: (C) — Typed `BenchmarkTurn` struct.**

Reasons:
- `has_answer` is `bool` in JSON — `map[string]string` silently drops it or errors; `map[string]any` requires manual type assertion everywhere
- A typed struct makes `extractTurns` self-documenting and avoids scattered `any`→`string` conversions
- One extra struct (3 lines) is trivial cost for clarity

```go
type BenchmarkTurn struct {
    Role      string `json:"role"`
    Content   string `json:"content"`
    HasAnswer *bool  `json:"has_answer,omitempty"`
}
```

Then `CacheEntry` fields become `[]BenchmarkTurn`, and `extractTurns` returns `[]BenchmarkTurn`. Conversion to `store.AgentMessage` is a simple loop in `convertTurnsToMessages`.

---

## 3. Optional parallelism?

**Answer: Yes, add `BENCHMARK_PARALLEL` env var, default 1 (sequential).**

Reasons:
- 20-40 min sequential is fine for a one-off run, but free tier rate limits may force longer waits — parallelism lets you fill gaps between rate-limited calls
- Keep it simple: a semaphore channel (`make(chan struct{}, parallel)`) gating goroutines
- Each goroutine creates its own `Service` instance (already the case — `newObserverTestService` is per-question)
- Add retry with backoff for 429 responses

Recommended default: `BENCHMARK_PARALLEL=3` (conservative — avoids hammering free tier). Max建议: 5.

---

## 4. Smoke test: 4 or 5 questions?

**Answer: 4 — exactly 1 per testable type.**

The smoke test validates that the pipeline works end-to-end for each type. A 5th question from the same type adds no new signal — if single_hop works once, it works. Run the full 176 after smoke passes.

```go
// Smoke: pick first question of each type
smokeTypes := []string{"single_hop", "implicit_preference_v2", "knowledge_update", "abstention"}
```
