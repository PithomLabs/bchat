# Code3 Implementation Review

**Reviewer:** AI Agent
**Date:** 2026-07-29
**Status:** APPROVED WITH NITS — one HIGH new finding

---

## Summary

Implementation is in the working tree (uncommitted). 11 files changed across 4 new + 3 modified source files. All 11 fixes from code3.md are correctly implemented. One new HIGH bug found during adversarial review.

---

## All 11 code3.md Fixes: ✅ Correct

| # | Fix | File | Check | Verdict |
|---|-----|------|-------|---------|
| C-1 | Add tenantID to Search args | `vectordb_cockroach.go:321` | `query.TenantID` passed as `$2`, `query.TopK` as `$3` | ✅ |
| C-2 | Add metadata scan variable | `vectordb_cockroach.go:330-335` | `metadata string` added, 8 scan targets match 8 SELECT cols | ✅ |
| C-3 | Guard SetDB with DSN check | `service.go:160-170` | `vectorDBConfig.CockroachDSN == "" \|\| vectorDBConfig.CockroachDSN == p.DSN` | ✅ |
| H-1 | Fix nil embedding handling | `vectordb_cockroach.go:177-181,201-205` | `len == 0 → nil → SQL NULL` in both Insert + InsertWithCheckpoint | ✅ |
| H-2 | Add empty query guard | `vectordb_cockroach.go:279-286` | Early return with empty result when both empty | ✅ |
| H-3 | Add description length | `handlers.go:487-489` | `> 10000` → 400 error | ✅ |
| H-4 | Fix Docker Go version | `Dockerfile.ecs:5` | `golang:1.26-alpine` (was 1.21) | ✅ |
| H-5 | CGO_ENABLED=0 | `Dockerfile.ecs:8,20-21` | gcc/musl-dev removed, `ENV CGO_ENABLED=0` | ✅ |
| H-6 | Fix region format | `deploy/ccloud/setup.sh:8` | `us-east-1` (was `aws-us-east-1`) | ✅ |
| H-7 | Cache EmbeddingService | `ticket_embedder.go:26-30,41,61,120` | Created once in processPendingTickets, passed as parameter, reused | ✅ |
| M-1 | Implement seed script | `cmd/seed/seed_demo_tickets.go` | main() connects, creates tenant, seeds 10 tickets | ✅ |

---

## New Finding: HIGH

### Search() ignores QueryEmbedding — diverges from LanceDB pattern

**File:** `vectordb_cockroach.go:289`
**Severity:** HIGH — latent (not triggered by current callers, but is a correctness bug)

**Problem:**

`LanceVectorDB.Search()` (`vectordb_lance.go:1108-1119`) uses the correct priority:
```go
if len(query.QueryEmbedding) > 0 {
    queryEmbedding = query.QueryEmbedding
} else if query.QueryText != "" {
    embeddings, err := db.embedSvc.Embed(...)
} else {
    return error
}
```

`CockroachVectorDB.Search()` unconditionally embeds `QueryText` — ignores `QueryEmbedding`.

**Impact:**
- If caller provides `QueryEmbedding` without `QueryText` → embeds empty string → garbage vector
- If caller provides both → `QueryEmbedding` silently ignored

**Not triggered currently** — all callers set `QueryText` only. Latent correctness bug.

**Fix:** Match LanceDB pattern — check `QueryEmbedding` first, fall back to embedding `QueryText`, error if neither.

---

## Nits

1. **MinScore not implemented**: `CockroachVectorDB.Search()` returns all TopK results regardless of `query.MinScore`. Feature gap vs LanceDB.

2. **Integration tests are stubs**: `ticket_resolution_test.go` creates `&Service{}` without a real store — calling `processPendingTickets()` would nil-dereference. Tests do not exercise any code path.

---

## Verdict

**APPROVED WITH NITS** — all 11 code3.md fixes correctly implemented. One HIGH bug (Search ignores QueryEmbedding) found during adversarial review should be fixed before deploy.
