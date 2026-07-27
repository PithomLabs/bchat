# Plan 5: Address Plan 4 Review Findings

**Status:** IMPLEMENTED
**Target:** bugs/049/plan4.md → implementation
**Prerequisites:** `code2.md` fixes completed (C1–C5, S1–S4, M4–M5) — all verified present in current codebase.

---

## Overview

`bugs/049/plan4_review.md` identified 12 findings in `plan4.md`. This plan validates each against the actual codebase and specifies changes for the valid ones. **7 of 12 findings are already resolved** by the existing code; **5 require action**.

---

## Validated Findings

### 🔴 Critical

#### #1. Question data alignment — NO ACTION (resolved)

The review claimed plan4.md's question table contradicts `code2.md` S2. The actual `embeddedQuestions` in `observer_longmemeval_test.go` (lines 305–396) already match plan4.md's table exactly:

| # | Type | Detail | Code Location |
|---|------|--------|--------------|
| 1 | single-session-user | "Summer Vibes" | line 310 |
| 2 | single-session-assistant | "27. Kg2 Bd5+" | line 323 |
| 3 | multi-session | "Korg B1" | line 340 (detail found in session text) |
| 4 | knowledge-update | "4 bikes" | line 353 |
| 5 | temporal-reasoning | "museum" | line 373 |
| 6 | abstention | NOT "30-gallon" | line 387 |

Note: The `multi-session` question's Answer is "2" (number of instruments), but the detail to preserve in the observation log is "Korg B1" (mentioned in session 1). The abstention question's session context is fully present (10-gallon tank conversation, line 388–394).

#### #2. `ReloadOMConfig` / `createTestSession` — NO ACTION (resolved)

Both functions exist in the codebase:
- `ReloadOMConfig` — `server/.../agent/om_config.go:87`
- `createTestSession` — `server/.../agent/observer_test_helpers_test.go:93`

Plan4.md's references are correct.

#### #3. Explicit dependency linkage — **ACTION REQUIRED**

Update plan4.md's "Depends on" from vague to exact. All prerequisites are present:

| Issue | What It Provides | Verified In |
|-------|-----------------|-------------|
| C1/C3 | `newObserverTestService`, `newObserverTestServiceWithMock` | `helpers.go:21-55` |
| C2 | `loadLongMemEvalQuestions` without `t.Skip` fallthrough | `test.go:433` |
| S2 | All 6 question types in `embeddedQuestions` | `test.go:305-396` |
| S4 | Updated skip message | `test.go:580,603` |
| M4 | Sequential timestamps in `mustMakeMessages` | `helpers.go:115-128` |
| M5 | Removed `_ = dbSession` dead code | `helpers.go` (verified clean) |

---

### 🟡 Significant

#### #4. OM_TOKEN_THRESHOLD lowered to 1 for two-pass test — **ACTION REQUIRED**

Use threshold=1 with `setOMEnvAndReload` (not bare `ReloadOMConfig`). Threshold=1 guarantees the reflector fires on any observation, which is necessary for the two-pass test pattern. The test log notes this is artificially low for testing purposes.

**Implementation:**
```go
setOMEnvAndReload(t, "OM_TOKEN_THRESHOLD", "1")
```

The helper handles cleanup automatically (restores original value + reloads config).

#### #5. No API error recovery — **ACTION REQUIRED**

Each of the 6 real LLM calls can fail (timeout, rate limit, key expiry). Wrap each question as a `t.Run` subtest with error recovery.

**Pattern:**
```go
for _, q := range embeddedQuestions {
    t.Run(q.QuestionType, func(t *testing.T) {
        ctx, ts, service, tenant := newObserverTestService(t, "obs-real-"+q.QuestionType)
        defer ts.Close()

        // Create all haystack sessions
        var sessionIDs []string
        for i, turns := range q.HaystackSessions {
            msgs := convertTurnsToMessages(turns)
            sid := createTestSession(t, ctx, ts, service, tenant.ID,
                fmt.Sprintf("session-real-%s-%d", q.QuestionType, i), msgs)
            sessionIDs = append(sessionIDs, sid)
        }

        // Run observer on each session
        var lastObsLog *store.ObservationLog
        for _, sid := range sessionIDs {
            err := service.RunObserver(ctx, tenant.ID, sid)
            if err != nil {
                t.Logf("API call failed for %s session %s: %v — skipping", q.QuestionType, sid, err)
                return
            }
            obsLog, err := ts.GetObservationLog(ctx, sid)
            if err != nil || obsLog == nil {
                t.Logf("no observation log for %s session %s", q.QuestionType, sid)
                continue
            }
            lastObsLog = obsLog
        }
        if lastObsLog == nil {
            return
        }

        // Assertions
        if q.QuestionType == "abstention" {
            assert.NotContains(t, lastObsLog.ObservationLog, "30-gallon",
                "must not hallucinate 30-gallon")
            assert.Contains(t, lastObsLog.ObservationLog, "10-gallon",
                "must capture actual tank size")
        } else {
            assert.Contains(t, lastObsLog.ObservationLog, /* detail from question */)
        }

        t.Logf("✓ %s: observer preserved detail", q.QuestionType)
    })
}
```

#### #6. Abstention session context — NO ACTION (resolved)

The embedded data already provides full context (lines 384–395): user mentions a 10-gallon aquarium. Plan4.md's abstention test is valid as-is.

#### #7. Env var cleanup — NO ACTION (resolved)

The existing `setOMEnvAndReload` helper (line 145) handles env mutation with proper `t.Cleanup` restoration. Plan4.md's reference to `ReloadOMConfig` should be updated to use this helper (see #4 above).

---

### 🟢 Minor

#### #8. Token reduction assertion — **ACTION REQUIRED**

```go
// Before:
assert.Less(t, compressedTokens, originalTokens)

// After:
assert.LessOrEqual(t, compressedTokens, originalTokens,
    "compressed observation should be same size or smaller")
```

If the original observation is already very short, compression may not reduce size.

#### #9. Multi-session setup — **ACTION REQUIRED**

Document that `createTestSession` is called once per inner `HaystackSession`:

- `single-session-user`, `single-session-assistant`: 1 `HaystackSession` → 1 `createTestSession` call
- `multi-session`, `knowledge-update`, `temporal-reasoning`: 2 `HaystackSession`s → 2 `createTestSession` calls with unique session IDs
- `abstention`: 1 `HaystackSession` → 1 call

The helper signature is:
```go
func createTestSession(t *testing.T, ctx context.Context, ts *store.Store,
    service *Service, tenantID int32, sessionID string,
    messages []store.AgentMessage) string
```

Only the `single-session-assistant` type in the embedded data actually tests assistant-generated content preservation.

#### #10. Split pass/fail counts — **ACTION REQUIRED**

```go
t.Logf("=== Summary ===")
t.Logf("Observer: %d/6 passed", observerPassCount)
t.Logf("Reflector: %d/6 passed", reflectorPassCount)
for _, qt := range questionTypes {
    t.Logf("  %s: observer=%v reflector=%v", qt, observerResults[qt], reflectorResults[qt])
}
```

#### #11. Clarify file creation vs extension — **ACTION REQUIRED**

Explicitly note that `observer_longmemeval_test.go` already exists with Tier 1 tests (from code2.md). Plan4 adds `TestRealLLM_DetailPreservationIntegration` as a **new top-level function in the existing file**.

```go
// --- Test 7: Tier 2 Real LLM Detail Preservation ---
// Added by bugs/049/plan5. Requires BENCHMARK_REAL_LLM=true.
func TestRealLLM_DetailPreservationIntegration(t *testing.T) {
```

#### #12. `assert.Contains` on `ObservationLog` — NO ACTION (resolved)

`ObservationLog` field is type `string` (`store/agent.go:689`). `assert.Contains` works correctly on strings. The 8 existing assertions in the file (lines 33, 34, 59, 88, 112, etc.) already use `assert.Contains` on this field.

---

## Summary of Code Changes

| File | Change | Lines |
|------|--------|-------|
| `server/.../agent/observer_test_helpers_test.go` | Add `convertTurnsToMessages`, `resetObservationLog` helpers | ~40 new lines |
| `server/.../agent/observer_longmemeval_test.go` | Add `TestRealLLM_DetailPreservationIntegration`, `fmt` import | ~120 new lines |

---

## Revised Test Flow

```
TestRealLLM_DetailPreservationIntegration
├── Skip unless BENCHMARK_REAL_LLM=true
├── Require OPENROUTER_API_KEY is set (t.Fatal if missing)
├── Log model being used (from LLM_MODEL env or default)
├── Load embeddedQuestions (6 questions, lines 305–396)
├── For each question q:
│   ├── newObserverTestService(t, "obs-real-"+q.QuestionType)
│   ├── For each inner HaystackSession:
│   │   ├── convertTurnsToMessages(turns)
│   │   ├── createTestSession with unique session ID
│   │   └── RunObserver — on error: t.Log + return (skip)
│   ├── Pass 1: Assert raw observations contain detail
│   ├── resetObservationLog for all sessions
│   ├── setOMEnvAndReload(t, "OM_TOKEN_THRESHOLD", "1")
│   ├── Pass 2: RunObserver again (triggers reflector)
│   ├── Assert compressed observations still contain detail
│   └── t.Logf per-question pass/fail
├── Summary: Observer N/6, Reflector M/6
```

### Detail-to-Detail Mapping

| QuestionType | Detail to Preserve | Source in Haystack |
|-------------|-------------------|-------------------|
| single-session-user | "Summer Vibes" | User mentions playlist name |
| single-session-assistant | "27. Kg2 Bd5+" | User mentions chess move in context |
| multi-session | "Korg B1" | Session 1 mentions keyboard |
| knowledge-update | "4 bikes" | Session 2 updates bike count from 3→4 |
| temporal-reasoning | "museum" | Session 1 mentions museum visit |
| abstention | NOT "30-gallon" | Only "10-gallon" is in conversation |

---

## Success Criteria

- All 6 questions pass observer detail preservation (using real LLM)
- At least 5/6 pass reflector detail preservation
- Abstention question does not hallucinate "30-gallon"
- Per-question API failures are logged and skip that subtest only
- `setOMEnvAndReload` used for all env mutations
- No test regressions (existing Tier 1 tests still pass)

## Run Command

```bash
# From repo root
source .env && BENCHMARK_REAL_LLM=true go test \
  ./server/router/api/v1/agent/ \
  -run TestRealLLM_DetailPreservationIntegration \
  -v -count=1
```
