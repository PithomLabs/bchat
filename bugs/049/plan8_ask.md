# Plan 8 Implementation — Questions

## 1. LLM client for answer generation and judging

For `generateAnswer` and `judgeAnswer`, I need to make standalone LLM calls to OpenRouter. The agent package's `RunObserver` uses `callObserverLLM` internally (observer prompt only). For generic calls I see two options:

- (A) Reuse the same `callLLM` / `chat` infrastructure the observer uses internally — it's already wired for OpenRouter with the API key from env
- (B) Write a new direct HTTP helper

Which should I use? (A) is preferable if the existing LLM client is package-accessible from test code.

## 2. CacheEntry turn format — `has_answer` field

The session cache turns have keys: `role` (string), `content` (string), `has_answer` (bool/null).

The existing `convertTurnsToMessages` helper (from code3.md) takes `[]map[string]string` — meaning all values must be strings. The `has_answer` bool will be lost or cause a type error.

Options for `CacheEntry` struct:
- (A) `[]map[string]any` — accept any value, handle conversion inline
- (B) `[]map[string]string` — strip `has_answer` during JSON unmarshal  
- (C) A typed `BenchmarkTurn` struct with `Role`, `Content`, `HasAnswer` fields

Which do you prefer? (C) is cleanest but adds a struct. (B) is simplest if we don't need `has_answer` for evidence routing (we don't — we use the cache key format instead).

## 3. Optional parallelism?

Plan says sequential single-threaded. At ~7s per question × 176 = ~20 min best case, likely longer with free tier rate limits.

Do you want optional parallelism (e.g., env var `BENCHMARK_PARALLEL=3`) to speed this up, or strictly sequential?

## 4. Smoke test: 4 or 5 questions?

Step 10 says "5-question smoke test (1 per type)." With 4 testable types (single_hop, preference, knowledge_update, abstention), that's 4. Should I add a 5th from the largest type (single_hop or knowledge_update), or just run 4?
