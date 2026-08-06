# Adversarial Test-Plan Review — Bug 060 v2

**Reviewer:** Senior Go Architect & CockroachDB Expert
**Date:** 2026-08-06
**Verdict:** GO

---

## Summary

v2 addresses all 5 major and 5 minor findings from v1. The plan is sound and ready for execution.

## Findings

**0 new issues.** All v1 findings correctly applied:

| v1 # | Finding | Disposition | Correct? |
|-------|---------|-------------|----------|
| 1 | Signin returns cookie | Step 2 cookie jar | ✅ |
| 2 | auth/status is POST | Step 2 POST | ✅ |
| 3 | Version resolution | Pre-check + optional sourceVersion | ✅ |
| 4 | Baseline isolation | Step 7 SQL diff | ✅ |
| 5 | ContentType assertion | Relaxed to ≥1 kb_section | ✅ |
| 6 | BCHAT_ vars undocumented | §4 preconditions | ✅ |
| 7 | bcrypt fallback | Go helper in Step 2b | ✅ |
| 8 | RAG_STARTUP_REINDEX_DISABLED | Consumed at service.go:280 | ✅ (correctly rebutted) |
| 9 | Widget key fetch | Step 3b | ✅ |
| 10 | Poll unbounded | 60×5s bounded loop | ✅ |
| 11 | Cost inflated | ~$0.005 | ✅ |
| 12 | Log capture | Redirected to /tmp/bchat.log | ✅ |

## Minor nit

`HandleTestRAGSearch` doesn't set `MinScore` (handler defaults to 0). The plan's "> 0.5 similarity" assertion is enforced by post-hoc check, not the query. This is correct — the search should return all candidates, and the assertion filters.

## Verdict

**GO** — Execute as written.
