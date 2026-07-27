# Plan 7 Review: LongMemEval Full Benchmark (Revised)

**Status:** APPROVED with nits
**Reviewer:** AI Agent
**Date:** 2026-07-27
**Files examined:**
- `bugs/049/plan7.md` (412 lines)
- `bugs/049/plan6_review.md` (prior review findings)
- Dataset files on disk (questions + session cache)
- `server/.../agent/observer.go` (resource scope logic)
- `server/.../agent/observer_test_helpers_test.go` (existing helpers)

---

## Verdict: Approved with Nits

Plan 7 correctly addresses all 4 critical findings from plan6_review and all 5 significant findings. Data loading handles all 3 cache formats. The 176-question scope is accurate. Resource-scoped OM UserID is fixed. The gold baseline and abstention sample size are documented. Three minor nits remain.

---

## Verified Claims (all match codebase/disk)

| Plan Claim | Verification | Match |
|-----------|--------------|-------|
| Cache format: `session` key for single_hop/implicit_preference_v2 | Disk: single_hop uses `session` key, 12 turns per entry | ✅ |
| Cache format: `session_old`+`session_new` for knowledge_update | Disk: knowledge_update uses both keys, 12 turns each | ✅ |
| Cache format: `session_1`+`session_2` for two_hop | Disk: two_hop uses both keys (71 questions, skipped) | ✅ |
| No cache data for assistant_previnfo | Disk: 0 cache entries for assistant_previnfo question IDs | ✅ |
| 176 testable questions (70+30+76) | Verified: single_hop=70, implicit_preference_v2=30, knowledge_update=76 | ✅ |
| 12 testable abstention (6+6) | Disk: 6 single_hop_abs + 6 knowledge_update_abs = 12 | ✅ |
| `human_valid_label` is bool | Disk: all 500 questions have `bool: True` | ✅ |
| Answer in `question_content.answer` | Disk: all 500 questions have string answer field | ✅ |
| `has_answer` in turns is metadata only | Turn format: `{role, content, has_answer}` — observer reads role+content only | ✅ |
| `GetObservationLogByResource` exists | Code: `store/agent.go:1249`, implemented in sqlite/postgres/mysql | ✅ |
| `TestEndToEndObserver_ResourceScope` validates resource scope | Code: `observer_longmemeval_test.go:40-60` — sets UserID, asserts observation | ✅ |
| `convertTurnsToMessages` exists | Code: `observer_test_helpers_test.go:132` | ✅ |

---

## Plan6 Review Findings — Status

| # | Finding | Plan7 Fix | Verified |
|---|---------|-----------|----------|
| 🔴1 | Cache 3 formats | `extractTurns` handles all 3 | ✅ Correct |
| 🔴2 | Wrong question count (234→176) | Corrected to 176 | ✅ Correct |
| 🔴3 | Missing UserID | `memSession.UserID = &userID` before RunObserver | ✅ Correct |
| 🔴4 | Struct field names | `BenchmarkQuestion` and `CacheEntry` defined with correct JSON tags | ✅ Correct |
| 🟡5 | Single-session scope limitation | Documented in "Fair comparison note" and "Known Limitations" | ✅ Adequate |
| 🟡6 | No reflector | Documented in "Known Limitations" | ✅ Adequate |
| 🟡7 | Session selection bias | Mitigated: knowledge_update uses both sessions | ✅ Correct |
| 🟡8 | Answer confound | Gold baseline: 10 questions with expected answer injected | ✅ Good addition |
| 🟡9 | assistant_previnfo no data | Removed from scope (56 questions) | ✅ Correct |

---

## Nits (3, non-blocking)

### 1. Typo in cache format table

Line 48:
```
| `session_1` + `session_2` (lists of turns) | `two_hop` | 71 | Skiped (needs 2+) |
```

**Fix:** "Skiped" → "Skipped"

### 2. JSON `per_type` double-counts abstention

Lines 280-283:
```json
"single-session-user": {"total": 64, "passed": 42, "abstention": {"total": 6, "passed": 5}},
"knowledge-update": {"total": 72, "passed": 48, "abstention": {"total": 6, "passed": 5}},
"abstention_combined": {"total": 12, "passed": 10}
```

The `total` fields (64, 72) are non-abstention counts. The nested `abstention` objects add 6 each. The `abstention_combined` adds 12 more. A reader summing all `total` fields gets 64+30+72+12=178, not 176.

**Fix:** Either:
- (A) Make `total` include abstention (70, 30, 78) and remove nested `abstention` + `abstention_combined`, or
- (B) Keep non-abstention totals, remove `abstention_combined`, keep nested `abstention` within types

Option (A) is cleaner:
```json
"single-session-user": {"total": 70, "passed": 47},
"single-session-preference": {"total": 30, "passed": 18},
"knowledge-update": {"total": 78, "passed": 53},
"abstention": {"total": 12, "passed": 10}
```

### 3. Stdout summary line 244 says "76 in cache, 6 abstention"

```
knowledge-update:          48/72  (66.7%)  [76 in cache, 6 abstention]
```

The `48/72` uses non-abstention count (72), but 76 questions are in cache (78 total minus 2 missing). The bracket note is confusing — it mixes cache count with abstention count.

**Fix:**
```
knowledge-update:          48/72  (66.7%)  [78 total, 2 missing cache, 6 abstention]
```

---

## Additional Observations (informational)

### Gold baseline question selection

Plan says "Choose 2 questions per type" for the 10-question gold baseline. With 4 testable types (single_hop, implicit_preference_v2, knowledge_update, abstention), that's 8 questions. The remaining 2 should come from somewhere — clarify which types get the extra 2.

### Incremental write format

Risk section mentions `build/benchmark/results_*.jsonl` per-question append, but the main output is `build/benchmark/longmemeval_results_*.json`. These are two different files. Clarify that the JSONL is a safety net for crash recovery, and the final JSON is the authoritative result.

### `openrouter/free` model variability

Plan documents this risk but doesn't mitigate it beyond noting. For reproducibility, consider logging the actual model used per call (available in the API response `model` field). This helps diagnose if a particular model is responsible for failures.

---

## Summary

| Severity | Count | Items |
|----------|-------|-------|
| 🔴 Critical | 0 | — |
| 🟡 Significant | 0 | — |
| 🟢 Minor | 3 | Typo, JSON double-count, summary note |

**Recommendation:** Fix the 3 nits, then implement. The plan is solid — all critical and significant findings from plan6_review are properly addressed. The data loading strategy is correct, the scope is accurately stated, and the gold baseline is a good addition for isolating OM quality from answer LLM noise.
