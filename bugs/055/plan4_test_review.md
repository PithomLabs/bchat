# Adversarial Code Review: Plan 4 — Ask Rovo E2E Test (Final — Implementation Verified)

**Bug/Task:** Plan for `plan4_test.md`  
**Reviewer:** Kilo (Senior Go Architect)  
**Date:** 2026-08-01  
**Verdict:** APPROVED — all 5 prior nits are fixed in the implemented test file, and I verified the claims by running the tests. No critical, high, or medium-severity blockers remain. Two minor observations are documented below.

---

## Executive Summary

I read `plan4_test.md`, reviewed the already-implemented test file `server/router/api/v1/agent/ticket_rag_inference_test.go`, ran the full `TestAskRovo` suite, and re-ran the previously flaky TopK test 5 times consecutively. All 11 tests pass, and `go test -race` reports no data races.

| Check | Result |
|-------|--------|
| TopK flakiness (Finding 1) | **Fixed** — count-based assertion is deterministic; 5/5 runs passed |
| Missing `TICKET_EMBEDDING_ENABLED` guard (Finding 2) | **Fixed** — `t.Setenv` present in `setupAskRovoTest` |
| `createTestMemo` `context.Background()` (Finding 3) | **Fixed** — accepts and uses `ctx` parameter |
| `atomic.AddInt32` return values (Finding 4) | **Fixed** — captured into `slugCounter` and `n` |
| `t.Parallel()` prohibition (Finding 5) | **Fixed** — file-level warning comment present |
| Race detector on Test 9 | **Pass** — no data races |
| TopK determinism | **Pass** — 5 consecutive runs all passed |

---

## What Is Correct

| Aspect | Status | Notes |
|--------|--------|-------|
| `controlledEmbeddingService` | CORRECT | Binary vectors exercise `MinScore`, `TenantID`, and `TopK` filters correctly |
| `setupAskRovoTest` env isolation | CORRECT | `RAG_PIPELINE_ENABLED`, `LANCEDB_STORAGE_PROVIDER`, `TICKET_EMBEDDING_ENABLED` all set |
| `createTestMemo` UID pattern | CORRECT | `test-memo-<tenantID>-<n>` satisfies `base.UIDMatcher` regex |
| Test 1 (happy path) | CORRECT | Seeds, indexes, infers, asserts `InternalNotes` |
| Test 2 (bug sections) | CORRECT | Verifies bug section search independently |
| Test 3 (no results) | CORRECT | Empty result for non-matching query |
| Test 4 (tenant isolation) | CORRECT | Cross-tenant seeds excluded by `TenantID` filter |
| Test 5 (dedup unchanged) | CORRECT | Two index calls, one version row |
| Test 6 (content changed) | CORRECT | Two index calls, two versions |
| Test 7 (comments) | CORRECT | Comment text appears in indexed content blob |
| Test 8 (nil tenant) | CORRECT | No panic, returns empty string |
| Test 9 (concurrent) | CORRECT | Thread-safe error collection, no `testing.T` race |
| Test 10 (MinScore) | CORRECT | Below-threshold query returns empty |
| Test 11 (TopK) | CORRECT | Count-based assertion; deterministic across runs |
| Struct references | CORRECT | All use `store/ticket.go` types |
| Atomic return values | CORRECT | Captured in `setupAskRovoTest` and `createTestMemo` |

---

## Behavioral Verification

I ran the full suite and observed the following in logs:

```
TestAskRovo_InferResolutionFromSimilarTickets:
  inferred resolution for new ticket ticket_id=1 similar_tickets=2 bug_history=0 total=2

TestAskRovo_InferResolutionFromBugSections:
  inferred resolution for new ticket ticket_id=1 similar_tickets=0 bug_history=1 total=1

TestAskRovo_NoResultsReturnsEmpty:
  no similar tickets or bug history found for inference ticket_id=1

TestAskRovo_TenantIsolation:
  no similar tickets or bug history found for inference ticket_id=1

TestAskRovo_TopKLimiting:
  inferred resolution for new ticket ticket_id=1 similar_tickets=3 bug_history=0 total=3
```

TopK limiting is verified: 5 seeds inserted, `similar_tickets=3` returned.

---

## Minor Observations (Not Blockers)

### Observation 1: User Creation Deviates from Plan Reference

**Severity:** LOW (cosmetic)

The plan references `createTestingHostUser` from `store/test/user_test.go:41`. The implemented code uses `ts.CreateUser` directly:

```go
user, err = ts.CreateUser(ctx, &store.User{
    Username: fmt.Sprintf("ask-rovo-user-%d", atomic.LoadInt32(&testTenantIDCounter)),
    Role:     store.RoleHost,
})
```

This is functionally correct — the SQLite schema allows empty strings for `email`, `nickname`, and `password_hash` — and avoids the bcrypt overhead. No action needed.

### Observation 2: `strings.Count(result, "### ")` Could Theoretically Overcount

**Severity:** LOW (theoretical)

If a chunk's `Content` or `Title` contained the literal substring `### `, `strings.Count` would overcount. The current test data does not trigger this. For a hackathon deliverable, this is acceptable. If this test is ever extended to use realistic content, switch to a structured count (e.g., split on `"### "` and count non-empty segments, or parse the markdown headers explicitly).

---

## Final Verdict

**APPROVED.** Plan 4 is implementation-ready. The test suite covers the core Ask Rovo pipeline: indexing, inference, dedup, concurrency, tenant isolation, nil safety, MinScore threshold, and TopK limiting. All 11 tests pass, including under the race detector. The two minor observations above are not blockers.

