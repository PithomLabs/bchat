# Code2 Review: Internal Notes + RAG-Based Bug Inference (Post-Review)

**Reviewer:** Senior Go Architect
**Date:** 2026-07-30
**Code:** `code2.md` (Bug 051 — Revised Post-Code-Review)
**Verdict:** **Rework** — 4 of 6 claimed fixes were never applied to the source code. The document describes an aspirational state that doesn't exist.

---

## Changelog Verification

| # | Finding (from `code_review.md`) | code2.md Claims | Actual Source | Status |
|---|-------------------------------|-----------------|---------------|--------|
| 1 | HIGH: Import script hardcodes `creator_id = 1` | `getOrCreateUser()` queries first available user | Still hardcoded `1` at `cmd/import-bugs/main.go:447` | ❌ NOT FIXED |
| 2 | MEDIUM: Context cancellation in inference goroutine | `context.WithoutCancel(ctx)` used | No match for `WithoutCancel` in `server/` tree. Request context still used. | ❌ NOT FIXED |
| 3 | MEDIUM: Two-step create+update documentation | Corrected to single INSERT | Already correct in code.md | ✅ DOC ONLY |
| 4 | NIT: Postgres ListTickets dead code | Lines 58-64 removed | Still present at `store/db/postgres/ticket.go:58-64` | ❌ NOT FIXED |
| 5 | NIT: Inference truncation 500→1000 chars | Increased to 1000 | Still 500 at `server/router/api/v1/agent/service.go:5632` | ❌ NOT FIXED |
| 6 | NIT: Integration test for inference path | Documented as known limitation | Documentation change only | ✅ DOC ONLY |

---

## Issues Found

### CRITICAL: Source Code Does Not Match Documentation

**Severity:** CRITICAL
**Description:** code2.md describes 6 fixes from code_review.md. 4 of these fixes (all source-code changes) were never applied to the actual files. Only 2 documentation-only items were updated.

The document claims:
- A `getOrCreateUser` function exists — it does not exist in `cmd/import-bugs/main.go`
- `context.WithoutCancel(ctx)` is used — no match in the entire server tree
- Postgres dead code was cleaned up — lines 58-64 are unchanged
- Truncation was increased to 1000 — still 500

**Fix:** Either:
1. Implement all 4 changes to match the code2.md description, OR
2. Revert code2.md to reflect the actual current state (reverting to code.md's accuracy)

---

### HIGH: Import Script Still Hardcodes `creator_id = 1`

**File:** `cmd/import-bugs/main.go:447`
**Severity:** HIGH (unchanged from code_review.md)
**Description:** code2.md claims this was fixed with a `getOrCreateUser` function (line 145-156 of code2.md), but the actual file still has:
```go
_, err := db.ExecContext(ctx, query,
    title, description, status, priority,
    1, // system user
    ...
)
```
No `getOrCreateUser` or any user-resolution function exists in the file (454 lines, confirmed via grep).

---

### MEDIUM: No `context.WithoutCancel` Found Anywhere

**File:** `server/router/api/v1/ticket_service.go` (CreateTicket handler)
**Severity:** MEDIUM (unchanged from code_review.md)
**Description:** code2.md says `context.WithoutCancel(ctx)` is used in the goroutine trigger. A grep for `WithoutCancel` across the entire `server/` tree returned zero results. The inference goroutine still receives the request-scoped context.

---

### MEDIUM: Postgres Dead Code Not Removed

**File:** `store/db/postgres/ticket.go:58-64`
**Severity:** MEDIUM (unchanged from code_review.md)
**Description:** code2.md claims "dead code removed." The actual file still has the redundant initialization:
```go
where, args := []string{"1=1"}, []interface{}{}   // line 58
if find.ID != nil {
    where = append(where, "id = $1")               // line 60 — dead
}
where = []string{"1=1"}                            // line 63 — overwrites
args = []interface{}{}                             // line 64 — overwrites
```

---

### MEDIUM: Content Truncation Still at 500

**File:** `server/router/api/v1/agent/service.go:5632`
**Severity:** MEDIUM (unchanged from code_review.md)
**Description:** code2.md claims truncation was increased to 1000 characters. Actual code still uses 500:
```go
if len(content) > 500 {
    content = content[:500] + "..."
}
```

---

## Items Correctly Described

| Item | Status |
|------|--------|
| Import script uses single INSERT (not two-step) | ✅ Already correct in code.md |
| Known limitation: no integration test for inference path | ✅ Documentation change |
| SQLite DSN pragmas (foreign_keys, busy_timeout, WAL) | ✅ Matches `main.go:66` |
| Import script uses `modernc.org/sqlite` | ✅ Matches `main.go:15` |
| Import script uses `pgx/v5/stdlib` | ✅ Matches `main.go:14` |
| `ticketExists` idempotency check | ✅ Matches `main.go:422-432` |
| Empty folder skip | ✅ Matches `main.go:250-252` |
| Phase classification rules | ✅ Matches `main.go:229-248` |
| Status/priority determination | ✅ Matches `main.go:352-383` |
| Internal notes format with key points extraction | ✅ Matches `main.go:304-350` |

---

## Summary

code2.md describes the correct fixes but the source code was never updated. The document is aspirational, not factual. To close this review cycle:

1. **Apply the 4 source changes** (creator_id lookup, context.WithoutCancel, dead code cleanup, 1000-char truncation), OR
2. **Downgrade code2.md** to match the actual implementation (revert to code.md state for those 4 items).

The architectural design and RBAC logic remain sound — the implementation gap is purely in the 4 unapplied fixes.
