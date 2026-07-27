# Plan 4: Tier 2 Real LLM Detail Preservation Test

**Status:** DRAFT
**Depends on:** code2.md fixes (completed)

---

## Goal

Validate whether the observer prompt actually preserves specific details (names, entities, dates) with a real LLM — the same failure mode that killed Mem0 on LongMemEval.

## Problem

The `.env` at repo root contains `OPENROUTER_API_KEY` and `LLM_MODEL`, but `godotenv.Load(".env")` in `store/test/store.go` resolves relative to the test package directory (`server/router/api/v1/agent/`), not the repo root. Tests log `failed to load .env file, but it's ok` — the key is never loaded into the test process.

## Solution: Shell Env Var Passthrough

```bash
source .env && BENCHMARK_REAL_LLM=true go test \
  ./server/router/api/v1/agent/ \
  -run TestRealLLM_DetailPreservationIntegration \
  -v -count=1
```

The test reads `OPENROUTER_API_KEY` and `LLM_MODEL` from the process environment (inherited from the shell). No test code changes needed for env loading.

## Test Design

Add `TestRealLLM_DetailPreservationIntegration` to `observer_longmemeval_test.go`.

### Flow

```
TestRealLLM_DetailPreservationIntegration
  ├── Skip unless BENCHMARK_REAL_LLM=true
  ├── Require OPENROUTER_API_KEY is set (t.Fatal if missing)
  ├── Log model being used (from LLM_MODEL env or default)
  ├── For each of 6 embedded questions:
  │   ├── newObserverTestService(t, "obs-real-<type>") — no mock
  │   ├── Load haystack sessions into a test session via createTestSession
  │   ├── RunObserver (real LLM observer call)
  │   ├── Assert observation contains expected detail
  │   ├── Set OM_TOKEN_THRESHOLD=1, ReloadOMConfig
  │   ├── RunObserver again to trigger reflector (real LLM reflector call)
  │   ├── Assert compressed observation still contains expected detail
  │   └── t.Logf per-question report
  └── Summary log: N/6 passed, per-type breakdown
```

### Per-Question Report Format

```
=== single-session-user ===
Detail: "Summer Vibes"
Observer: * 🔴 (14:30) User stated they created playlist "Summer Vibes" on Spotify
Observer pass: YES
Reflector: * 🔴 User created playlist "Summer Vibes" on Spotify
Reflector pass: YES
Tokens: 38 → 17 (ratio: 0.45)
```

### Embedded Questions (6 total)

| # | Type | Detail to Preserve |
|---|------|--------------------|
| 1 | single-session-user | "Summer Vibes" |
| 2 | single-session-assistant | "27. Kg2 Bd5+" |
| 3 | multi-session | "Korg B1" |
| 4 | knowledge-update | "4 bikes" |
| 5 | temporal-reasoning | "museum" |
| 6 | abstention | Must NOT say "30-gallon" |

### Assertions

- **Observer**: `assert.Contains(t, obsLog.ObservationLog, tt.detail)`
- **Reflector**: `assert.Contains(t, compressedObsLog.ObservationLog, tt.detail)`
- **Abstention**: `assert.NotContains(t, obsLog.ObservationLog, "30-gallon")`
- **Token reduction**: `assert.Less(t, compressedTokens, originalTokens)`

## Cost

- 6 questions × 2 LLM calls = **12 API calls**
- `gpt-4o-mini`: ~$0.02 total
- `gpt-4o`: ~$0.15 total

## Run Command

```bash
# From repo root
source .env && BENCHMARK_REAL_LLM=true go test \
  ./server/router/api/v1/agent/ \
  -run TestRealLLM_DetailPreservationIntegration \
  -v -count=1
```

## Files Changed

- `server/router/api/v1/agent/observer_longmemeval_test.go` — add `TestRealLLM_DetailPreservationIntegration`

## Success Criteria

- All 6 questions pass observer detail preservation
- At least 5/6 pass reflector detail preservation
- Abstention question does not hallucinate "30-gallon"
- No test regressions (existing Tier 1 tests still pass)
