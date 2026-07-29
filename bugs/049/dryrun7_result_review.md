# Dry Run 7 Review: xiaomi/mimo-v2.5 + Mastra OM Parity

**Date:** 2026-07-28
**Depends on:** dryrun7_result.md, mastra.md
**Status:** READY TO IMPLEMENT

---

## Part 1: Benchmark Result Assessment

### Run Configuration

| Parameter | Value |
|-----------|-------|
| Model | xiaomi/mimo-v2.5 (via `BENCHMARK_ANSWER_MODEL`) |
| Test | `TestBenchmarkKnowledgeUpdate` (72 questions) |
| Duration | 61 min (3658s) |
| Dataset | `data_6_session_cache.json` (~14K entries) |

### Results

| Metric | mimo-v2.5 | openrouter/free (baseline) |
|--------|-----------|---------------------------|
| Passed | 53 | 57 |
| Failed | 14 | 13 |
| Skipped (observer timeout) | 3 | — |
| No cache | 2 | 2 |
| **Pass rate** | **75.7%** (53/70) | **81.4%** (57/70) |

### Analysis

1. **`BENCHMARK_ANSWER_MODEL` flag works** — line 3 of dryrun7 output confirms `Answer model: xiaomi/mimo-v2.5`
2. **75.7% pass rate** — 5.7pp below openrouter/free baseline. Decent for a non-English-optimized model on KnowledgeUpdate, the hardest type requiring cross-session synthesis
3. **3 observer timeouts** (`08e075c7`, `c4ea545c`, `0ddfec37`) — OpenRouter routing latency. The observer always uses `openrouter/free` regardless of `BENCHMARK_ANSWER_MODEL`
4. **2 no-cache** (`89941a94`, `07741c45`) — known dataset gaps, same as previous runs
5. **Pass rate context** — openrouter/free routes to strong models like nemotron-3-ultra-550b. mimo-v2.5's 75.7% on the hardest type is a fair comparison point

### Verdict

The `BENCHMARK_ANSWER_MODEL` feature is working as designed. mimo-v2.5 is viable for KnowledgeUpdate testing. The 14 failures likely reflect weaker cross-session reasoning in English, not a pipeline issue.

---

## Part 2: Mastra OM Parity Assessment

### Compliance Matrix

| Mastra Principle | bchat Implementation | Status |
|-----------------|---------------------|--------|
| Two agents (Observer + Reflector) | `RunObserver` + `runReflector` — both LLM calls | ✅ |
| Token-budget triggers | `ObserverTokenThreshold` (30K), `TokenThreshold` (2K) | ✅ |
| Append-only observations | `updatedLog += newObservations` | ✅ |
| Reflector compression | Rewrites observations when token count > threshold | ✅ |
| Emoji priority (🔴🟡🟢) | In observer prompt output format | ✅ |
| Two-level bullet lists | Sub-bullets with `->` prefix for tool sequences | ✅ |
| Date-grouped sections | `Date: Dec 4, 2025` format in prompt | ✅ |
| Current task + suggested response | Parsed from `<current-task>`, `<suggested-response>` XML tags | ✅ |
| Prompt caching structure | `callObserverLLMWithCache` — system (cached) → obs (cached) → new msgs (dynamic) | ✅ |
| Event-based log (not compaction) | Observer produces event-based observations, not summaries | ✅ |

### Gaps

| Gap | Severity | Impact |
|-----|----------|--------|
| **Temporal anchoring** — bchat uses text-embedded dates (`(14:30)` inline) vs Mastra's structured 3-date model (observation date, referenced date, relative date) | Medium | Temporal reasoning is hardest category (Mastra: 85.7% with gpt-4o). Structured dates may improve parsing |
| **Relative date computation** — Mastra computes relative offsets ("2 days from today"), bchat relies on LLM to interpret in prompt | Low-Medium | May affect temporal-reasoning accuracy |
| **Observer prompt framing** — Mastra emphasizes "event-based decision log", bchat's prompt is more general | Low | Minor — output format aligns anyway |

### Extensions Beyond Mastra

| Extension | Description |
|-----------|-------------|
| **Observer Buffer** | Background pre-computation reduces latency by buffering observations before threshold |
| **Hybrid OM + RAG** | Observations indexed to vector DB for retrieval-based augmentation |
| **Scope system** | Thread-scoped (per-session) vs Resource-scoped (per-user cross-session) |
| **Trivial message filtering** | Skips "ok", "thanks" etc. before observation |
| **Retry logic** | Configurable retry attempts for LLM calls |
| **Per-session mutex** | Prevents concurrent observer runs on same session |

### Verdict

**Architecturally compliant with Mastra's OM spec.** The two-agent architecture, token-budget triggers, append-only observations, emoji priorities, date-grouped format, and continuity mechanism are all present and correctly implemented.

No code changes needed for principle compliance. The main divergence (temporal anchoring) is a representation difference, not a missing feature.

---

## Part 3: Recommendations

1. **Test additional models** — mimo-v2.5 at 75.7% is a data point; test 2-3 more models to build a comparison matrix
2. **Temporal anchoring improvement optional** — structured 3-date model could improve temporal-reasoning scores but is not required for OM compliance
3. **No code changes needed** — the `BENCHMARK_ANSWER_MODEL` flag and OM architecture are both working correctly
