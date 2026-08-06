# Adversarial Plan Review — Bug 060

**Reviewer:** Senior Go Architect & CockroachDB Expert
**Date:** 2026-08-06
**Verdict:** GO-WITH-CHANGES

---

## Executive Summary

The plan correctly identifies the root cause and proposes a minimal, targeted fix. The analysis is thorough and accurate. I recommend proceeding with the fix after addressing the issues below.

---

## 1. Root-Cause Validity ✅ CONFIRMED

**Claim:** `CockroachVectorDB.Search` renders empty `ContentTypes` as `content_type IN ('')`, matching zero rows.

**Verification:**
- `vectordb.go:1081` — `RetrieveContextForQuery` sends `ContentTypes: []string{}` (empty list)
- `vectordb_cockroach.go:388` — `contentTypeFilter := "''"` when list empty
- `vectordb_cockroach.go:401` — SQL: `content_type IN (%s)` becomes `content_type IN ('')`
- `chunker.go:394` — Chunks stored as `ContentType: fileType + "_section"` (e.g., `kb_section`)
- `vectordb_lance.go:1198-1204` — LanceDB correctly omits clause when empty (correct behavior)
- `vectordb.go:544-555` — MemoryVectorDB correctly skips filter when empty

**Conclusion:** Root cause is valid. The empty string literal `''` never matches any stored `content_type` values.

---

## 2. Broken Assumptions ⚠️ PARTIAL

### 2.1 `content_type IN ('')` match-zero ✅
Confirmed. No rows have `content_type = ''`. The predicate is logically false.

### 2.2 Legacy rows with bare `kb` ⚠️
The plan mentions this at §2.6 and §8 but doesn't quantify the risk. Legacy importers may have stored `content_type = 'kb'` (without `_section`). The CRUD methods (`DeleteByVersion`, `PurgePreVersionedChunks`) still use bare `fileType` in their WHERE clauses:

```go
// vectordb_cockroach.go:318
DELETE FROM agent_vectors WHERE tenant_id = $1 AND content_type = $2 AND source_version = $3
```

This means **existing legacy rows (`kb`) may not be cleaned up properly during reindex**. The fix should normalize content_type handling in CRUD methods (Fix 2 in the plan), but this is not blocking Fix 1.

### 2.3 Deployed binary vs repo HEAD
Cannot verify without access to `bchat-crdb.fly.dev` logs. The plan correctly flags this as a risk.

---

## 3. Fix Risk ✅ LOW

### 3.1 Regressions in other backends
The fix is isolated to `vectordb_cockroach.go` (build tag `cockroach`). No changes to LanceDB, PG, or SQLite code. Both build tags (`cockroach` and `rag`) should compile independently.

### 3.2 Tenant isolation
The `tenant_id = $2` filter remains intact. Dropping `content_type IN (...)` when empty does **not** affect tenant isolation — it expands the search to all content types for that tenant, which is the documented intent.

### 3.3 SQL syntax
The fix must produce valid SQL. Two valid approaches:

**Option A (recommended):** Conditionally build the WHERE clause
```go
whereClause := fmt.Sprintf("tenant_id = $2 AND (embedding <=> $1::VECTOR) <= 1 - $4")
if len(query.ContentTypes) > 0 {
    whereClause = fmt.Sprintf("tenant_id = $2 AND content_type IN (%s) AND (embedding <=> $1::VECTOR) <= 1 - $4", contentTypeFilter)
}
```

**Option B:** Use `TRUE` as a no-op filter
```go
contentTypeFilter := "TRUE"
if len(query.ContentTypes) > 0 { ... }
// WHERE tenant_id = $2 AND content_type IN (%s) ...
```

Option A is cleaner (avoids unnecessary `content_type IN (TRUE)` for non-empty lists).

---

## 4. Latent Bugs ⚠️ DEFER-CLASSIFY

### 4.1 CRDB ignores `SourceVersion`/`ActiveOnly`
The plan correctly identifies this. Without version filtering, stale-version mixing can occur. **Classification: MEDIUM risk, defer to follow-up.** The immediate user-visible failure (zero results) is higher priority.

### 4.2 No `audience` column
The plan correctly identifies this. **Classification: LOW risk, defer.** Requires schema migration + reindex.

### 4.3 CRUD content_type mismatch
The plan identifies this as Fix 2. **Classification: HIGH priority.** Without this fix, reindex operations may fail to delete old-version chunks, leading to duplicate data. This should be included in the same PR.

---

## 5. Scope Discipline ✅ CLEAN

The plan correctly limits changes to `vectordb_cockroach.go`. No drift into LanceDB, PG, or SQLite files.

**Exception:** Fix 2 (CRDB CRUD methods) is also in the same file and should be included.

---

## 6. Test Design ⚠️ NEEDS CLARIFICATION

### 6.1 SQL-shape unit tests
The plan proposes testing:
- `ContentTypes = []` → no `content_type` predicate
- `ContentTypes = ["kb_section","kb"]` → `content_type IN ('kb_section','kb')`

**Issue:** The current `vectordb_cockroach.go` does not have a testable query-builder function — the SQL is built inline in the `Search` method. To make this testable:
1. Extract a `buildSearchQuery(query SearchQuery) (string, []interface{})` helper function
2. Add unit tests that verify the SQL string without a live CRDB

### 6.2 CI without CRDB server
SQL-shape tests can run without CRDB (pure string comparison). Integration tests require CRDB. The plan should clarify which tests are unit vs integration.

### 6.3 Flake risk
Low for SQL-shape tests. Integration tests may be flaky if CRDB is not available.

---

## 7. Deploy ⚠️ MINOR GAPS

### 7.1 Reindex requirement
The plan mentions reindex in verification but should be explicit: **after deploying Fix 1, a full reindex is required** to ensure all chunks are searchable. The fix only affects search, not storage.

### 7.2 Downtime
Zero downtime expected — the fix is a code change, not a schema migration.

### 7.3 Rollback path
Correct: redeploy previous image. The SQL change is additive.

### 7.4 Monitoring
The plan mentions checking server logs for `RAG fallback activated`. Should also monitor:
- `TotalResults` in search responses (should be > 0 after fix)
- Embedding API latency (unchanged)
- CRDB query latency (may improve due to fewer fallback calls)

---

## 8. Edge Cases ⚠️ COVERAGE GAPS

### 8.1 Empty `QueryText` and `QueryEmbedding`
Handled correctly in `Search` (lines 370-377) — returns empty result set.

### 8.2 `TopK <= 0`
Not validated. SQL `LIMIT $3` with `TopK = 0` or negative may cause an error or return no results. Should add validation:
```go
if query.TopK <= 0 {
    query.TopK = 10
}
```

### 8.3 `MinScore` distance/similarity sign confusion
The plan correctly identifies the `<=>` operator as distance. The comparison `1 - distance <= 1 - $4` is equivalent to `distance >= $4`, which correctly filters by minimum similarity. ✅

### 8.4 Legacy `kb` vs `kb_section` in ContentTypes
The plan's Fix 2 addresses this for CRUD methods. For search, when a caller supplies bare `kb`, it won't match `kb_section` rows. The `SearchQuery` should normalize content types to include both variants:
```go
// In RetrieveContextForQuery or Search
normalizedTypes := make([]string, 0, len(query.ContentTypes)*2)
for _, ct := range query.ContentTypes {
    normalizedTypes = append(normalizedTypes, ct)
    if !strings.HasSuffix(ct, "_section") {
        normalizedTypes = append(normalizedTypes, ct+"_section")
    }
}
query.ContentTypes = normalizedTypes
```

This is already done in `service.go:5255` for the file-type search path but not for the general chat path.

### 8.5 SQL injection via ContentTypes
The `ContentTypes` values are interpolated directly into SQL (line 392). While currently safe (internal values only), this is a latent risk. Consider parameterizing or validating against an allowlist.

---

## 9. Missed Anything? ✅ COVERED

- **Widget 404 issue:** Correctly marked out of scope.
- **Fallback token budget:** Correctly identified as a symptom, not root cause.
- **embed.js caching:** Not mentioned, but irrelevant to this fix.

---

## Required Changes Before Implementation

| # | Issue | Severity | Action |
|---|-------|----------|--------|
| 1 | SQL injection via ContentTypes | Low | Add input validation or allowlist check |
| 2 | TopK <= 0 not validated | Low | Add default fallback |
| 3 | Extract query builder for testability | Medium | Refactor `Search` method |
| 4 | Legacy `kb` vs `kb_section` normalization | Medium | Add normalization in `RetrieveContextForQuery` |
| 5 | CRUD methods use bare `fileType` | High | Fix in Fix 2 (same PR) |

---

## Confidence Rating

**Fix 1 alone:** 95% confidence it restores correct answers on bchat-crdb.

**Combined with Fix 2:** 99% confidence. Fix 2 ensures reindex operations properly clean up old-version chunks.

---

## Recommendation

**GO-WITH-CHANGES** — Proceed with implementation after:
1. Adding input validation for `TopK`
2. Including Fix 2 (CRUD content_type normalization) in the same PR
3. Extracting a testable query builder function
4. Adding normalization for bare `kb` → `kb_section` in `RetrieveContextForQuery`

The fix is correct, minimal, and well-isolated. The additional changes are low-risk improvements that prevent future issues.
