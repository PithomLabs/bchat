# Adversarial Review: code2.md — Fixes from code.md §6 Checklist

**Review Date:** 2026-07-29
**Source:** bugs/050/code2.md

---

## Verdict: APPROVED WITH NITS — BUT layered on top of code_review.md criticals

The 3 proposed fixes in code2.md are technically correct and should be implemented. However, they address secondary edge-case hardening while 3 critical dataflow bugs from my `code_review.md` remain unfixed. The combined set of 6 bugs (3 critical + 3 high) must all be resolved before the implementation is production-ready.

**Deployment order:**
1. Fix `code_review.md` C-1, C-2, C-3 first (broken core dataflow)
2. Then apply code2.md H-1, H-2, H-4 (edge-case hardening)
3. Then write real integration tests

---

## code2.md Findings Verification

### Correctly Identified Fixes (3)

| Item | Finding | Fix | Status |
|------|---------|-----|--------|
| H-1 | `json.Marshal(nil)` → `"null"`, `null::VECTOR` fails | Check `len(chunk.Embedding) == 0`, pass Go `nil` | ✅ `nil` → SQL `NULL`, `NULL::VECTOR` valid |
| H-2 | Empty `QueryText` + nil `QueryEmbedding` → undefined embed | Early return empty result at top of `Search()` | ✅ Matches `SearchQuery.QueryEmbedding` at `vectordb.go:156` |
| H-4 | No description length limit → memory exhaustion | Add 10K char cap in handler | ✅ Prevents unbounded input |

### Verified PASS Verdicts (12 of 12)

All 12 PASS verdicts are accurate against the current code. Key cross-checks:

| Item | Claim | Verification |
|------|-------|-------------|
| C-1 | Uses `*pgconn.PgError` | `vectordb_cockroach.go:118` — correct import at line 16 |
| C-2 | No `IF NOT EXISTS` on vector index | `vectordb_cockroach.go:109` documents it, lines 115-127 catch `42P07` |
| C-3 | Error messages sanitized | `handlers.go:492`: `"Escalation service unavailable"` |
| C-4 | Tenant scoping enforced | `handlers.go:469-473` extracts tenant; `service.go:5546` sets `TenantID: &tenant.ID` |
| H-3 | Cron uses context timeout | `service.go:196-198`: `context.WithTimeout(context.Background(), 2*time.Minute)` |
| M-1 | SetDB type-asserts | `service.go:160`: `cockroachDB, ok := vectorDB.(*CockroachVectorDB)` (assertion OK, but logic broken — see below) |
| M-2 | Index failure graceful | `vectordb_cockroach.go:115-127`: non-fatal `slog.Warn` on failure |
| M-3 | Docker port 8081 | `Dockerfile.ecs:37` EXPOSE 8081, healthcheck at `localhost:8081` |
| M-4 | Taskfile env var syntax | `Taskfile.yml:241`: inline `VAR=value ./binary` (correct) |
| N-1/N-2/N-3 | Imports, error wrapping, slog | All consistent with codebase conventions |

---

## What code2.md Misses (Critical Blind Spots)

The code.md §6 checklist that code2.md validates against covers error types, tenant scoping, and edge cases. But it **completely misses the core dataflow bugs** from my `code_review.md`:

### C-1 (code_review): Search() missing `tenantID` parameter
**File:** `vectordb_cockroach.go:309`

Not checked by any code.md §6 item. The SQL has `$1`, `$2`, `$3` placeholders but only 2 args are passed. Tenant isolation is broken.

### C-2 (code_review): Search() column/scan mismatch  
**File:** `vectordb_cockroach.go:299-320`

Not checked by any code.md §6 item. SELECT returns 8 columns (including `metadata`), Scan reads 7 (skipping `metadata`, misaligning everything after). Type mismatch between JSONB and int32.

### C-3 (code_review): SetDB unconditional — overwrites separate pool
**File:** `service.go:160-165`

Item **[M-1] PASS** is misleading: it checks "Does SetDB work for SQLite deployments?" and only verifies the type assertion `*CockroachVectorDB` exists (line 160). The assertion is correct, but the **logic is wrong** — `SetDB()` is called unconditionally without checking whether `COCKROACH_DSN` was set. When `COCKROACH_DSN` is explicitly provided, `NewCockroachVectorDB` opens a separate pool, which is immediately orphaned by `SetDB()`.

The code.md §6 checklist item [M-1] should also check the DSN guard:
```go
// What should be checked — missing from both checklist and code2.md:
if vectorDBConfig.CockroachDSN == "" || vectorDBConfig.CockroachDSN == p.DSN {
    cockroachDB.SetDB(s.GetDriver().GetDB())  // Only when reusing existing pool
}
```

### Gap Analysis

| code_review.md finding | code.md §6 checklist coverage | code2.md coverage |
|------------------------|-------------------------------|-------------------|
| C-1: missing tenantID | ❌ Not checked | ❌ Not mentioned |
| C-2: column/scan mismatch | ❌ Not checked | ❌ Not mentioned |
| C-3: SetDB unconditional | ❌ [M-1] checks wrong thing | ❌ Marked PASS incorrectly |
| H-1: nil embedding | ❌ Not checked | ✅ Fixed |
| H-2: empty query | ❌ Not checked | ✅ Fixed |
| H-4: description length | ❌ Not checked | ✅ Fixed |

---

## Implementation Order

| Step | What | Files | Lines | Source |
|------|------|-------|-------|--------|
| 1 | Fix Search() `tenantID` param | `vectordb_cockroach.go` | 309 | code_review.md C-1 |
| 2 | Fix Search() column scan | `vectordb_cockroach.go` | 299-320 | code_review.md C-2 |
| 3 | Fix SetDB DSN guard | `service.go` | 160-165 | code_review.md C-3 |
| 4 | Fix nil embedding → NULL | `vectordb_cockroach.go` | 175-224 | code2.md H-1 |
| 5 | Add empty query guard | `vectordb_cockroach.go` | 273-281 | code2.md H-2 |
| 6 | Add description length limit | `handlers.go` | 486 | code2.md H-4 |
| 7 | Write real integration tests | `ticket_resolution_test.go` | full file | both reviews |

---

## Summary

| Category | Findings in code2.md | Findings not in code2.md | Total to fix |
|----------|----------------------|--------------------------|-------------|
| CRITICAL | 0 | 3 (code_review.md C-1, C-2, C-3) | 3 |
| HIGH | 3 (H-1, H-2, H-4) | 2 (code_review.md H-1, H-2) | 5 |
| MEDIUM | 0 | 5 (code_review.md M-1–M-5) | 5 |
| NIT | 0 | 3 (code_review.md N-1–N-3) | 3 |

code2.md's 3 fixes are correct but incomplete. The implementation must first fix the 3 critical dataflow bugs before applying the hardening in code2.md.
