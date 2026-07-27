# Plan v3: Detail Preservation Tests for bchat's Observational Memory

**Bug/Feature:** 049
**Date:** 2026-07-27
**Status:** Plan v3 (revised per plan2 review)
**File:** `server/router/api/v1/agent/observer_longmemeval_test.go`

---

## Changes from v2 (per review)

| # | Change | Rationale |
|---|--------|-----------|
| 1 | Replace "Done" → "Addressed" in action items | Not yet implemented |
| 2 | Add `ReloadOMConfig` + `t.Cleanup` pattern for Test 2 | OM config singleton race |
| 3 | Fix Test 3 missing-closing-tag expected → raw fallback | `observer.go:182-185` fallback behavior |
| 4 | Add `"trivial_response"` and `"empty_response"` mock entries | Edge case coverage |
| 5 | `createTestSession` in `observer_test_helpers.go` | Reusable across test files |
| 6 | Skip abstention subtest in mock mode | Mock can't test hallucination |
| 7 | Document thread vs resource scope exclusion | OMScopeResource needs user_id |
| 8 | Leaner test service (no encryption/signing) | Observer doesn't need them |
| 9 | `long_content` subtest mock-only with note | Mock has no token limits |

---

## Implementation Order

1. `observer_test_helpers.go` — lean service + session helper + mock data
2. `observer_longmemeval_test.go` — all 6 tests

## Success Criteria

```
go test -v -run TestObserverLongMemEval ./server/router/api/v1/agent/
```
