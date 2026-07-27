# Plan 6 Review: LongMemEval Full Benchmark

**Status:** REWORK needed
**Reviewer:** AI Agent
**Date:** 2026-07-27
**Files examined:**
- `bugs/049/plan6.md` (344 lines)
- Dataset files on disk (questions + session cache)
- `server/.../agent/observer.go` (resource scope logic)
- `server/.../agent/observer_longmemeval_test.go` (existing resource scope test)

---

## Verdict: Rework

The plan's ambition is correct but the data loading strategy is fundamentally broken. The session cache has **three different formats** the plan doesn't account for, and the actual testable question count is **100, not 234**. Additionally, the resource-scoped OM setup is incomplete. These blocking issues must be resolved before implementation.

---

## 🔴 Critical (blocks implementation)

### 1. Session cache has 3 formats — plan only handles 1

**Verified against disk:**

| Format | Keys | Used by | Count | Testable? |
|--------|------|---------|-------|-----------|
| `session_1`/`session_2` | `session_1`, `session_2` | `two_hop` only | 71 questions | No (skipped) |
| `session` | `session` | `single_hop`, `implicit_preference_v2` | 100 questions | **Yes** |
| `session_old`/`session_new` | `session_old`, `session_new` | `knowledge_update` | 76 questions | **Yes (but plan ignores)** |
| *(none)* | — | `assistant_previnfo`, `multi_session_synthesis`, `temp_reasoning_*` | 253 questions | No (no data) |

**Plan6.md line 47 says:**
> 3. Extract `session_1` (list of `{role, content, has_answer}` turns)

This only works for `two_hop` questions (71, all skipped). The 100 single-hop and implicit-preference questions use the `session` key. The 76 knowledge-update questions use `session_old`/`session_new`.

**Evidence (disk verification):**
```
$ python3 -c "import json; ..."
single_hop: 70/70 have session data (format: session key)
assistant_previnfo: 0/56 have session data
knowledge_update: 76/78 have cache entries (format: session_old/session_new)
implicit_preference_v2: 30/30 have session data (format: session key)
```

**Fix:** Update data loading to handle all three formats:
```go
func extractTurnsFromCacheEntry(entry CacheEntry) []Turn {
    // Format 1: session key (single_hop, implicit_preference_v2)
    if entry.Session != nil && len(entry.Session) > 0 {
        return entry.Session
    }
    // Format 2: session_old + session_new (knowledge_update)
    if entry.SessionOld != nil && len(entry.SessionOld) > 0 {
        return append(entry.SessionOld, entry.SessionNew...)
    }
    // Format 3: session_1 (two_hop — skipped, but handle for completeness)
    if entry.Session1 != nil && len(entry.Session1) > 0 {
        return entry.Session1
    }
    return nil
}
```

### 2. Actual testable count is 100, not 234

The plan's question counts table (lines 68-78) is wrong:

| Type | Plan claims | Actually testable | Issue |
|------|------------|------------------|-------|
| single-session-user (single_hop) | 70 | 70 | ✓ |
| single-session-assistant (assistant_previnfo) | 56 | **0** | No cache entries |
| single-session-preference (implicit_preference_v2) | 30 | 30 | ✓ |
| knowledge-update | 78 | **76** (plan says 0 covered) | Wrong — format exists but plan ignores it |
| **Total claimed** | **234** | **176** | — |
| **Single-session claimed** | **234** | **100** | Plan doesn't handle knowledge_update format |

The plan lists knowledge-update as "Yes" covered (line 73) but the data loading code (line 47) only extracts `session_1`, which knowledge_update doesn't have. The "Covered" column is wrong.

**Fix:** Update the questions-in-scope table to reflect actual data availability. knowledge_update should be "Yes (session_old/session_new format)" and assistant_previnfo should be "No (no cache data)".

### 3. Resource-scoped OM requires UserID — plan doesn't set it

**Observer code (`observer.go:93`):**
```go
if config.Scope == OMScopeResource && session.UserID != nil {
    resourceID = fmt.Sprintf("user_%d", *session.UserID)
}
```

If `session.UserID` is nil, `resourceID` stays empty, and the observer falls through to thread-scope behavior. The benchmark would silently run in thread scope, producing meaningless results for the "resource scope" test.

**Existing test (`TestEndToEndObserver_ResourceScope` line 46-52):**
```go
var userID int32 = 42
sessionID := createTestSession(t, ctx, ts, service, tenant.ID, "session-resource-1", ...)
session := service.memorySessions.Get(tenant.ID, sessionID)
session.UserID = &userID
service.memorySessions.Update(session)
```

The test manually sets `session.UserID` after creation. The plan's pipeline (lines 108-110) calls `createTestSession` but never sets UserID.

**Fix:** Add UserID assignment to the pipeline:
```go
sid := createTestSession(t, ctx, ts, service, tenant.ID, "lmeval-"+q.QuestionID, messages)
// Set UserID for resource-scoped OM
memSession := service.memorySessions.Get(tenant.ID, sid)
memSession.UserID = &userID  // use a fixed test user ID
service.memorySessions.Update(memSession)
```

### 4. Answer prompt field name mismatch

**Plan line 121:**
```go
{q.QuestionContent.Question}
```

**Actual data structure:**
```json
{
  "question_content": {
    "facts": [...],
    "question": "What is the name of...",
    "answer": "Summer Vibes",
    "explanation": "...",
    "unified_date": "..."
  }
}
```

`question_content.question` exists but the Go struct must match the JSON tags. The plan doesn't define the `BenchmarkQuestion` struct fields. If `QuestionContent` is `map[string]interface{}`, then `q.QuestionContent["question"]` works. If it's a typed struct, the field names must match.

**Fix:** Define the struct explicitly:
```go
type BenchmarkQuestion struct {
    QuestionID      string          `json:"question_id"`
    BackgroundID    string          `json:"background_id"`
    QuestionType    string          `json:"question_type"`
    QuestionContent struct {
        Facts       []string `json:"facts"`
        Question    string   `json:"question"`
        Answer      string   `json:"answer"`
        Explanation string   `json:"explanation"`
    } `json:"question_content"`
    HumanValidLabel string          `json:"human_valid_label"`
}
```

---

## 🟡 Significant

### 5. Single-session-only doesn't answer "is OM good enough?"

The plan tests 100/500 questions (20%). LongMemEval's SOTA scores include multi-session and temporal types. If bchat scores 65% on single-hop, that's not comparable to Zep's 71% on all 500.

**Mitigation:** Document this limitation explicitly in the report. Compare only against single-session baselines if available, or note the scope difference.

### 6. No reflector in pipeline

The plan runs observer once per question. With 12 turns producing ~60-100 tokens, the reflector never triggers (threshold=2000). The benchmark tests raw observation quality only.

**Mitigation:** Acceptable for a first pass, but document that reflector compression is untested. Consider a separate reflector benchmark.

### 7. Session selection bias

The plan picks `session_1` (or `session`) from the cache. For knowledge_update, there are `session_old` and `session_new` — which one contains the evidence? Some questions may have evidence in `session_new` only.

**Mitigation:** For knowledge_update, run observer on BOTH `session_old` and `session_new` (two sessions). For single-hop, run on the single `session`.

### 8. Answer generation confound

If the answer is wrong, is it OM failure, answer LLM failure, or prompt failure? The plan treats all failures as OM failures.

**Mitigation:** Add a "gold" baseline: run the answer LLM with the EXPECTED answer (from `human_valid_label`) instead of the observation log. If the gold baseline also fails, the failure is answer LLM noise, not OM.

### 9. `assistant_previnfo` has no cache data

56 questions (11.2% of total) have zero session data in the cache. The plan lists them as "covered" but they cannot be tested.

**Fix:** Remove from scope or find alternative data source.

---

## 🟢 Minor

### 10. Runtime estimate may be optimistic

Plan says "~20 minutes at best" for 234 questions × 5s each. But:
- Only 100 questions are testable
- Free tier rate limits may be aggressive
- 415MB JSON parse time not validated

**Fix:** Add a 5-question smoke test before the full run to validate timing.

### 11. JSON results file path

Plan says `build/data/longmemeval_results.json`. The `build/data/` directory is for SQLite DB and LanceDB. Consider `build/benchmark/` instead.

### 12. Plan lists `makeMessagesFromTurns` helper

But `convertTurnsToMessages` already exists (added in code3.md). The plan should reference the existing helper, not propose a new one.

---

## Corrected Questions-in-Scope Table

| Type | Questions | With Data | Format | Tested? |
|------|-----------|-----------|--------|---------|
| single_hop | 70 | 70 | `session` | Yes |
| assistant_previnfo | 56 | 0 | *(none)* | **No** |
| knowledge_update | 78 | 76 | `session_old`/`session_new` | **Yes (was listed as covered but code doesn't handle format)** |
| implicit_preference_v2 | 30 | 30 | `session` | Yes |
| two_hop | 71 | 71 | `session_1`/`session_2` | No (skipped) |
| multi_session_synthesis | 62 | 0 | *(none)* | No |
| temp_reasoning_implicit | 73 | 0 | *(none)* | No |
| temp_reasoning_explicit | 60 | 0 | *(none)* | No |
| **Total** | **500** | **247** | — | **176 testable** |

---

## Summary

| Severity | Count | Items |
|----------|-------|-------|
| 🔴 Critical | 4 | Cache format mismatch, wrong question count, missing UserID, struct field names |
| 🟡 Significant | 5 | Scope limitation, no reflector, selection bias, confound, missing data |
| 🟢 Minor | 3 | Runtime estimate, file path, helper naming |

**Recommendation:** Fix critical items 1-4 before implementation. The data loading rewrite is the largest change — it affects the entire pipeline structure.
