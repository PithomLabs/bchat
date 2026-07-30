# Plan3 Review: Internal Notes + RAG-Based Bug Inference (Final)

**Reviewer:** Senior Go Architect
**Date:** 2026-07-30
**Plan:** `plan3.md` (Bug 051 — Post-Plan2-Review)
**Verdict:** **Approved with Nits** — All 18 prior findings are resolved. Two minor items remain.

---

## Changelog Verification

| # | Finding (from `plan2_review.md`) | Status | Evidence |
|---|-------------------------------|--------|----------|
| N1 | Agent Service Call Chain | **Fixed** ✅ | `go s.agentHandler.GetService().inferResolutionForNewTicket(...)` with reference to `memo_service.go:1225` |
| N2 | `HasPermission` Omitted from Step 6 | **Fixed** ✅ | Step 6 updated: "Permission constant + presets + `HasPermission` helper" |
| N3 | Import/CreateTicket `internal_notes` Conflict | **Fixed** ✅ | Two-step process: create with `""`, immediately UpdateTicket to `"Pending summary..."` |
| N4 | Raw SQL vs Existing Search API | **Fixed** ✅ | Uses `vectorDB.Search(ctx, SearchQuery{...})` matching `service.go:5568` |
| N5 | Import Interrupt Resilience | **Fixed** ✅ | Re-run detection + manual cleanup SQL documented |

---

## New Findings (Nits)

### NA. Postgres Parameter Wording (Line 234)

The plan states:
```
- Change `$11` to `$12` for `tenant_id`, add `$12` for `internal_notes`
```

Two parameters cannot both be `$12`. If `internal_notes` is added **after** `tenant_id` in the INSERT column list, `tenant_id` stays at `$11` and `internal_notes` becomes `$12`. The wording implies `tenant_id` moved, which is incorrect.

**Action:** Correct to: "Add `$12` for `internal_notes` after `tenant_id`'s `$11`".

### NB. Missing `vectorDBMu` Lock in Inference Function (Line 557)

The inference function accesses `s.vectorDB` directly:
```go
result, err := s.vectorDB.Search(ctx, SearchQuery{...})
```

The existing pattern at `service.go:5561` guards with the read mutex:
```go
s.vectorDBMu.RLock()
vectorDB := s.vectorDB
s.vectorDBMu.RUnlock()
// ... use vectorDB ...
```

Since `s.vectorDB` can be swapped during reindex (see `service.go:542`), direct access without locking is a data race.

**Action:** Wrap `s.vectorDB` access with `s.vectorDBMu.RLock()/RUnlock()`.

---

## Overall Assessment

**Approved with Nits.** The plan is comprehensive, all prior issues are addressed, and the remaining two items are trivially fixable during implementation. Ready for coding.
