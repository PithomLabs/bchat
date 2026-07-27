# Plan: Dry Run — Manual Validation Before Full Benchmark

**Goal:** Run 3 questions end-to-end with verbose output so the user can manually inspect every step before committing to the 1-hour full run.

---

## Approach

A separate test function `TestBenchmarkLongMemEvalDryRun` that:

1. Loads both JSON files (same as full benchmark)
2. Picks 3 representative questions:
   - 1 `single_hop` (short session, straightforward)
   - 1 `implicit_preference_v2` (preference/rubric judging)
   - 1 `knowledge_update` (session_old + session_new concatenation)
3. Runs the full pipeline for each: load -> extract -> convert -> OM -> answer -> judge
4. Prints **every intermediate artifact** to stdout and a human-readable report file

---

## Output Verbosity

```
=== Dry Run: Question 1/3 ===
Question ID:     q_034
Question Type:   single_hop
Question:        What did the customer say about their water heater?

Input Turns (10 total, showing all):
  [0] user: "My water heater is making a loud noise"
  [1] agent: "I understand your concern..."
  ...

Observation Log (Raw):
  [INFO] 2024-01-15T10:30:00 | Customer reported water heater noise
  ...

Generated Answer:
  The customer reported their water heater is making a loud noise.

Expected Answer:
  The customer's water heater is making a loud noise and they want it inspected.

Judge Verdict: yes
Judge Prompt Used:
  <prompt text>

  ---
```

---

## Report File

Written to `build/benchmark/dryrun_YYYYMMDD_HHMMSS.txt`. User opens and reviews.

---

## How to Run

```bash
# Explicit gate, same as full benchmark
BENCHMARK_LONGMEMEVAL=true OPENROUTER_API_KEY=sk-or-v1-... \
  go test ./server/router/api/v1/agent/ -run TestBenchmarkLongMemEvalDryRun \
  -v -count=1 -timeout=10m
```

Takes ~3-5 minutes (6 API calls: 3 answer + 3 judge).

---

## Implementation Notes

- Same `loadBenchmarkData`, `extractTurns`, `convertBenchmarkTurns`, `newBenchmarkLLMClient`, `generateAnswer`, `judgeAnswer` as plan9
- No parallelism (sequential, easier to read)
- `t.Log` + `fmt.Printf` for output (both visible with `-v`)
- Separate `writeDryRunReport` for the report file
- Shares all helper code -- if dry run passes, full run is ready with zero new code

---

## Included in plan9 Implementation

The dry run test is ~50 lines and goes in the same file (`benchmark_longmemeval_test.go`). It does NOT need to be removed after the full run -- useful as a fast sanity check for future OM changes.
