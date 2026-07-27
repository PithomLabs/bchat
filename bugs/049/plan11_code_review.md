# Plan 11 — Adversarial Code Review

**Reviewed by:** Senior Go Architect
**Date:** 2026-07-28
**File:** `server/router/api/v1/agent/benchmark_longmemeval_test.go`

---

## Verdict

| Severity | Count | Action |
|----------|-------|--------|
| Critical (bug) | 1 | Must fix before running per-type benchmark |
| Significant | 3 | Should fix before running full benchmark |
| Minor | 6 | Fix at convenience |

---

## Critical

### B1: `loadCompletedFromJSONL` only reads first JSONL line

**Location:** Lines 327-342

```go
dec := json.NewDecoder(strings.NewReader(string(data)))
for dec.More() {   // ← BUG: More() works for arrays, not top-level streams
    var r BenchResult
    if err := dec.Decode(&r); err != nil {
        continue
    }
    completed[r.QuestionID] = true
}
```

`dec.More()` is designed for JSON **arrays** — it checks for comma/close-bracket tokens. For JSONL (newline-delimited top-level objects), `More()` returns `false` after the first `Decode` call. The loop processes **exactly one entry** then exits.

**Impact:** Crash recovery is broken. On re-run, only the first question is skipped; all ~87+ others are re-executed. Total waste: ~87 API calls × 3 = 261 calls, ~$0.20 in unnecessary judge calls, ~25 min lost runtime.

**Fix:** Replace with `io.EOF` termination loop (add `"io"` to imports):

```go
for {
    var r BenchResult
    err := dec.Decode(&r)
    if err == io.EOF {
        break
    }
    if err != nil {
        continue
    }
    completed[r.QuestionID] = true
}
```

---

## Significant

### S1: `appendJSONL` silently drops all errors

**Location:** Lines 316-325

```go
f, err := os.OpenFile(...)
if err != nil {
    return   // ← silent: caller thinks write succeeded
}
defer f.Close()
data, _ := json.Marshal(result)  // ← silent: empty bytes written
f.Write(data)                      // ← silent: partial write undetected
```

Three error points ignored. If `build/benchmark/` doesn't exist (it's created by `writeDryRunReport` but not by any benchmark test), disk is full, or permissions are wrong — crash recovery silently does nothing.

**Impact:** False sense of security. User thinks per-question writes provide safety, but a disk issue means all 528 API calls run and produce zero persisted results.

**Fix:** Return `error` from `appendJSONL` and log at call site. At minimum check `f.Write` return value.

---

### S2: `content()` silently swallows marshal/unmarshal errors

**Location:** Lines 40-45

```go
data, _ := json.Marshal(q.QuestionContent)
_ = json.Unmarshal(data, &c)
```

If `QuestionContent` (typed `interface{}`) has unexpected shape, or `Answer` is nil, the method returns a zero-value struct. `AnswerString()` then returns `"<nil>"` (line 60: `Sprintf("%v", v)` with nil), `QuestionString()` returns `""`.

**Impact:** Silent data corruption. A misaligned question format produces empty answers indistinguishable from LLM errors. Debugging this wastes hours.

**Fix:** Return `(questionContent, error)` or at minimum check `c.Question == ""` and `c.Answer == nil` after unmarshal and log a warning.

---

### S3: Per-type benchmark tests not implemented

**Location:** File ends at line 641 with only `TestBenchmarkLongMemEvalDryRun`.

Plan11 specifies 5 test functions that are **missing**:

| Test Function | Status | What's Needed |
|---------------|--------|---------------|
| `TestBenchmarkSingleHop` (64 questions) | ❌ Missing | Full per-type loop with JSONL recovery |
| `TestBenchmarkPreference` (30 questions) | ❌ Missing | Same pattern |
| `TestBenchmarkKnowledgeUpdate` (72 questions) | ❌ Missing | Same pattern |
| `TestBenchmarkAbstention` (12 questions) | ❌ Missing | Same pattern, uses `filterAbstention` |
| `TestBenchmarkAggregate` | ❌ Missing | Reads all JSONL files, prints summary |

The shared helpers ARE defined (line 344 `benchmarkJSONLPath`, line 363 `filterByType`, line 377 `filterAbstention`, line 316 `appendJSONL`, line 327 `loadCompletedFromJSONL`) but are **unused** and **untested** — including the buggy `loadCompletedFromJSONL`.

**Impact:** Plan11 is half-implemented. The dry run validates the pipeline but the actual benchmark can't run.

---

## Minor

| # | Issue | Lines | Details |
|---|-------|-------|---------|
| M1 | Hardcoded paths | 96-97 | `/home/chaschel/Desktop/...` paths are user-specific. Not portable for CI or other developers. Should use env vars (e.g., `BENCHMARK_DATA_DIR`). |
| M2 | `AnswerString()` returns `"<nil>"` for nil answers | 47-62 | `Sprintf("%v", v)` on nil passes `"<nil>"` as the expected answer. Judge compares against this string, causing false fails. |
| M3 | `benchmarkJSONLPath` defined but unused | 344-346 | Dead code until per-type tests are added. Minor now, but must be verified when per-type tests land. |
| M4 | `filterByType` and `filterAbstention` defined but unused | 363-385 | Same as M3. |
| M5 | `DryRunResult` / `BenchResult` duplication | 78-89 vs 304-314 | Two near-identical structs with different fields. `DryRunResult` has `ObservationLog`; `BenchResult` doesn't. Per-type tests use `BenchResult` which loses observation log context in output. Either unify or document the tradeoff. |
| M6 | `content()` is fragile | 40-45 | Marshalling `interface{}` → JSON → unmarshalling to struct. Use `q.QuestionContent.(map[string]any)` with direct field access instead. |

---

## What Still Needs to Be Done

### Before any per-type benchmark can run (blocking):

1. **Fix B1** — `loadCompletedFromJSONL` must loop to `io.EOF`, not rely on `dec.More()`
2. **Fix S1** — `appendJSONL` must surface errors to caller
3. **Fix S2** — `content()` must surface marshalling errors
4. **Implement S3** — Add all 5 per-type test functions from plan11

### Non-blocking improvements:

5. **Add `BENCHMARK_DATA_DIR` env var** — remove hardcoded paths
6. **Fix M2** — handle nil `Answer` in `AnswerString()`
7. **Resolve M5** — decide: unified result struct or keep separate
8. **Either wire up or remove unused functions** (M3, M4) — dead code today, needed tomorrow

### Implementation Order (per S3):

Based on plan11.md implementation order, adjusted for bugs found:

1. Fix B1 — `loadCompletedFromJSONL` loop
2. Fix S1 — `appendJSONL` error handling
3. Fix S2 — `content()` error handling
4. Implement `TestBenchmarkSingleHop` (64 questions) — simplest, proves the pattern
5. Implement `TestBenchmarkPreference` (30 questions) — same pattern
6. Implement `TestBenchmarkKnowledgeUpdate` (72 questions) — verify 2 missing cache handling
7. Implement `TestBenchmarkAbstention` (12 questions) — verify `filterAbstention` works
8. Implement `TestBenchmarkAggregate` — reads all JSONL, computes summary
9. Run full benchmark
