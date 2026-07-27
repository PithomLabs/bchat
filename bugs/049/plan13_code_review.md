# Plan 13 — Adversarial Code Review

**Reviewed by:** Senior Go Architect
**Date:** 2026-07-28
**File:** `server/router/api/v1/agent/benchmark_longmemeval_test.go`
**Assessing:** plan13 fixes applied + readiness for per-type tests (plan12)

---

## Verdict

| Criteria | Verdict |
|----------|---------|
| Plan13 fixes correctly applied | ✅ All correct |
| Ready to implement per-type tests | ✅ **Yes, after fixing S1 + S2 first** |

---

## Plan13 Fixes — Verification

| Fix | Prescribed | Actual Code | Status |
|-----|-----------|-------------|--------|
| F1: `loadCompletedFromJSONL` | `io.EOF` loop (lines 52-60) | Lines 335-345: `for { err := dec.Decode(&r); err == io.EOF { break }; if err != nil { continue } }` | ✅ Correct — flat if-blocks, no `else if` |
| F2: `readJSONL` | `t *testing.T` + `io.EOF` loop + `IsNotExist` logging (lines 90-112) | Lines 349-370: `t *testing.T`, `io.EOF` loop, logs ALL errors | ✅ Correct — improved over plan13 (universal logging, not just `IsNotExist`) |
| F3: Aggregate dedup | Remove subtraction | Not applicable — `TestBenchmarkAggregate` not implemented yet | ✅ Apply when implementing |
| F4: `"io"` import | Add to imports | Line 7: `"io"` present | ✅ |

The implementation is slightly better than plan13 prescribed: flat `if` blocks instead of `if/else if`, and `readJSONL` logs all read errors, not only `IsNotExist`.

---

## Pre-Existing Issues (still present)

These were flagged in `plan11_code_review.md` and remain unfixed. They affect per-type test reliability.

### S1: `appendJSONL` silently drops write errors

**Lines 317-326**

```go
func appendJSONL(path string, result BenchResult) {
    f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil { return }           // ← silent
    defer f.Close()
    data, _ := json.Marshal(result)     // ← silent
    data = append(data, '\n')
    f.Write(data)                        // ← silent
}
```

**Impact on per-type tests:** The per-type test loop (plan12, line 189) calls `appendJSONL` after each question. If the write fails (disk full, permissions, directory missing), the test continues silently. On re-run, `loadCompletedFromJSONL` finds no JSONL and re-executes all questions — 528 wasted API calls, ~$0.39.

**Fix (5 lines):** Return `error` from `appendJSONL` and log at call site in the per-type test:

```go
func appendJSONL(path string, result BenchResult) error {
    f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return fmt.Errorf("open JSONL: %w", err)
    }
    defer f.Close()
    data, err := json.Marshal(result)
    if err != nil {
        return fmt.Errorf("marshal result: %w", err)
    }
    data = append(data, '\n')
    if _, err := f.Write(data); err != nil {
        return fmt.Errorf("write JSONL: %w", err)
    }
    return nil
}
```

Then in per-type test:
```go
if err := appendJSONL(jsonlPath, result); err != nil {
    t.Logf("WARN: failed to write JSONL: %v", err)
}
```

---

### S2: `content()` silently swallows marshal/unmarshal errors

**Lines 41-45, 48-62**

```go
func (q BenchmarkQuestion) content() questionContent {
    var c questionContent
    data, _ := json.Marshal(q.QuestionContent)   // ← silent
    _ = json.Unmarshal(data, &c)                   // ← silent
    return c
}
```

**Impact on per-type tests:** `runBenchmarkQuestion` (plan12) calls `q.QuestionString()` and `q.AnswerString()`. If the `QuestionContent` JSON shape is unexpected, these return empty/`"<nil>"` strings. The judge compares the LLM's answer against `"<nil>"` — a false fail that looks like an OM bug.

**Fix (add validation, 3 lines):**
```go
func (q BenchmarkQuestion) content() questionContent {
    var c questionContent
    data, _ := json.Marshal(q.QuestionContent)
    _ = json.Unmarshal(data, &c)
    if c.Question == "" {
        panic(fmt.Sprintf("question %s: content unmarshal produced empty question", q.QuestionID))
    }
    return c
}
```

Or return error:
```go
func (q BenchmarkQuestion) content() (questionContent, error) { ... }
```

---

## Readiness Summary

| Component | Ready? | Notes |
|-----------|--------|-------|
| Data loading (`loadBenchmarkDataDryRun`) | ✅ | |
| Cache extraction (`extractTurnsDryRun`, `convertBenchmarkTurns`) | ✅ | |
| LLM calls (`generateAnswerDryRun`, `judgeAnswerDryRun`) | ✅ | |
| JSONL helpers (`loadCompletedFromJSONL`, `readJSONL`, `benchmarkJSONLPath`, `clearBenchmarkJSONLs`) | ✅ | Both `io.EOF` loops fixed |
| Filter helpers (`filterByType`, `filterAbstention`) | ✅ | |
| Result helpers (`countResults`) | ✅ | |
| `appendJSONL` — crash recovery writes | ❌ **Fix S1 first** | Silent failures → false sense of security |
| `content()` — question data parsing | ❌ **Fix S2 first** | Silent failures → `"<nil>"` expected answers |
| Per-type test functions | N/A | Not implemented — plan12 scope |

**To unblock:** Fix S1 + S2 before adding the per-type test functions. Both are small, isolated changes (~8 lines total) that prevent hard-to-diagnose failures in the 176-question run.

**Without fixing S1/S2:** The per-type tests will work for the happy path, but a disk issue (S1) or data format shift (S2) will silently produce wrong results, wasting the full benchmark run.
