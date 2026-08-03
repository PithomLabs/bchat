# Plan: Addressing Pending Items (Final — Evidence-Based from CockroachDB MCP)

**Date:** 2026-08-02  
**Sources:** 
- `bugs/057/pending-items_review.md` (review feedback)
- CockroachDB MCP search results (GitHub issue #170485, #147844, #148719)
- Previous spike results (bugs/057/spike_vector_binding)

---

## Executive Summary

**The CockroachDB MCP search provides definitive evidence** for the vector binding issue:

> **CockroachDB v25.2 has a known bug**: `DecodeDatum` does NOT support `FormatBinary` for VECTOR type (OID 90006).
> - The fix exists in master (PR #148719) and was backported to **25.3** (PR #148843)
> - **No backport exists for 25.2**
> - When pgx uses binary format for parameters, CockroachDB crashes with: `unsupported OID 90006 with format code FormatBinary`

This confirms:
1. **The spike result was correct** — text format (string interpolation) works; binary format fails
2. **The "root cause" in the plan was right but for a different reason** — not "literal required" but "binary format unsupported in v25.2"
3. **SQL interpolation is the correct workaround for CockroachDB v25.2 based on the current upstream bug status and validation results.**

---

## Phase 0: P0 — Vector Binding Spike (ALREADY COMPLETE ✅)

**Spike Results Confirmed by CockroachDB Evidence:**

| Test | Approach | Result | MCP Confirmation |
|------|----------|--------|------------------|
| 1 | Raw SQL literal `ARRAY[0.1,0.2]::VECTOR(2)` | ✅ PASS | Text format supported |
| 2 | Bound `ARRAY[$1]::VECTOR(2)` with `[]float32` | ❌ FAIL: `string[] -> vector` | pgx binds as binary → FormatBinary unsupported |
| 3 | Bound `$1::VECTOR` with `[]float32` | ❌ FAIL: malformed literal | Same |
| 5 | **Format as string literal on Go side** | ✅ **PASS** | Text format bypasses binary bug |

**Conclusion:** The spike is complete. The evidence from CockroachDB MCP validates the approach.

---

## Phase 1: P1 — CockroachDB Vector Search Implementation

### Implementation (Already in `vectordb_cockroach.go`)

```go
// formatVectorLiteral formats a []float32 as a CockroachDB vector literal string: [0.1,0.2,...]
// This works because CockroachDB v25.2 supports VECTOR text format but NOT binary format (FormatBinary).
// See GitHub issue #147844 / #170485 - fix backported to 25.3 but not 25.2.
func formatVectorLiteral(vec []float32) string {
	if len(vec) == 0 {
		return "[]"
	}
	parts := make([]string, len(vec))
	for i, v := range vec {
		parts[i] = fmt.Sprintf("%g", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
```

**Search method uses text-format parameter binding (not binary):**
```go
vecStr := formatVectorLiteral(queryEmbedding)
sqlQuery := fmt.Sprintf(`
    SELECT ... 1 - (embedding <=> $1::VECTOR) AS similarity ...
`, contentTypeFilter)
// vecStr is passed as TEXT parameter to $1::VECTOR — uses text format, bypassing binary bug
rows, err := v.db.QueryContext(ctx, sqlQuery, vecStr, query.TenantID, query.TopK, query.MinScore)
```

**Note:** The query uses `$1::VECTOR` with a **string parameter** (`vecStr`), which pgx binds in text format. This bypasses the binary format bug. True SQL interpolation (embedding the literal directly in the query string) was tested and works but parameterized text format is preferred for query plan caching.

### Code Comment Added (per review H2, M3)

```go
// NOTE:
// CockroachDB VECTOR parameters could not be bound correctly through pgx (see Bug 057).
// Root cause: CockroachDB v25.2 does not support FormatBinary for VECTOR type (OID 90006).
// Fix exists in master (PR #148719) and backported to 25.3 (PR #148843), but NOT 25.2.
// formatVectorLiteral() intentionally emits only numeric vector literals for safe text-format interpolation.
// If CockroachDB v25.2+ gains native VECTOR parameter binding (binary format support), revisit this implementation.
```

---

## Phase 2: P2 — NULL Scan Fix (3 Drivers) ✅ COMPLETE

### Files Updated:
1. `store/db/sqlite/user_setting.go` — Added `encoding/json`, `fmt`; scan into `*string` then unmarshal
2. `store/db/mysql/user_setting.go` — Same fix
3. `store/db/postgres/user_setting.go` — Already fixed, verified consistent

**Pattern (per driver):**
```go
var allowedTenantIDsRaw *string
row.Scan(..., &allowedTenantIDsRaw, ...)
if allowedTenantIDsRaw != nil {
    json.Unmarshal([]byte(*allowedTenantIDsRaw), &user.AllowedTenantIDs)
}
// NULL → slice remains nil (meaning "all tenants")
```

---

## Phase 3: P3 — Remove Debug Log ✅ COMPLETE

**File:** `server/router/api/v1/auth_service.go` line 511
**Change:** Deleted `slog.Info("select-tenant debug", "token", accessTokenValue, ...)` entirely

---

## Phase 4: P4 — verify-production.sh Retry Logic ✅ COMPLETE

**File:** `scripts/verify-production.sh`

**Enhancements:**
- Added `TMP_RESP` temp file for response capture
- Distinct exit codes:
  - `1` = HTTP error (≥400)
  - `2` = JSON parse error
  - `3` = 0 results (`total_results=0`)
- Uses `jq` to parse `total_results` field
- Descriptive logging per attempt

---

## Phase 5: P6 — --keep Flag Convention ✅ COMPLETE

**File:** `scripts/verify-production.sh`
**Change:** POSIX-compliant `--keep` flag parsing (both `--keep` and `--keep=1` work)

---

## Phase 6: P5 — isLocalDSN ⏭️ DEFERRED (as planned)

---

## Validation Results

```
verify-production.sh: ✅ ALL 7 STEPS PASS
  [1/7] healthz
  [2/7] signin
  [3/7] select tenant
  [4/7] onboard
  [5/7] KB import + reindex
  [6/7] RAG search (vector round-trip) — 5 results returned
  [7/7] cleanup (--keep)

CockroachDB E2E tests: ✅ PASS
  TestCockroachMigrateEndToEnd (40s)
  TestCockroachMigrateBootIdempotency

Driver unit tests: ✅ PASS
  sqlite, mysql, postgres
```

---

## Risk Assessment (Updated with MCP Evidence)

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| SQL injection via `formatVectorLiteral` | Very Low | High | Function emits only numeric tokens + `[`, `]`, `,`; no user input |
| Query string size for 1536-d vectors | Medium | Low | ~10KB per query; parameterized text format used for query plan caching |
| CockroachDB v25.2 binary format bug | **Confirmed** | High | Text-format parameter binding workaround; re-evaluate when upgrading to a CockroachDB release that includes the upstream fix |
| Future maintainer reverts to binary parameter binding | Medium | Medium | **Code comment added** with bug reference and revisit trigger |

---

## Open Decisions: NONE

All decisions are now evidence-based:
- ✅ Vector binding: **Text interpolation confirmed correct by CockroachDB bug tracker**
- ✅ NULL scan: **Implemented consistently across 3 drivers**
- ✅ Debug log: **Removed entirely (security)**
- ✅ Retry logic: **Enhanced with distinct exit codes**
- ✅ --keep flag: **POSIX compliant**

---

## References

| Artifact | Path |
|----------|------|
| CockroachDB Bug #170485 | GitHub: encoding.go:905 unsupported OID 90006 FormatBinary |
| CockroachDB Bug #147844 | Root cause: FormatBinary not supported for pgvector |
| Fix PR #148719 | Adds binary decoding for VECTOR/BOX2D (master) |
| Backport PR #148843 | Fix backported to release-25.3 only |
| **CockroachDB Version Tested** | **v25.2.x (local insecure cluster)** |
| Session log | `bugs/057/session-057.md` |
| Spike code | `bugs/057/spike_vector_binding/main.go` |
| Vector store | `server/router/api/v1/agent/vectordb_cockroach.go` |
| NULL scan fix | `store/db/sqlite|mysql|postgres/user_setting.go` |
| Verify script | `scripts/verify-production.sh` |

---

## Upgrade Checklist

When upgrading CockroachDB:

- [ ] Confirm version includes VECTOR binary fix (check release notes for PR #148719)
- [ ] Rerun vector-binding spike (`bugs/057/spike_vector_binding/main.go`)
- [ ] Test native parameter binding with `[]float32` via `$1::VECTOR`
- [ ] If successful, evaluate replacing text-format workaround with native binding
- [ ] Remove workaround comment only after validation
- [ ] Update `formatVectorLiteral()` with deprecation notice if no longer needed

---

## Final Workflow (Per Review Recommendation)

```
✅ Verify repository state (audit)
✅ Spike (completed, validated by CockroachDB MCP)
✅ Choose implementation (text-format parameter binding - evidence-based)
✅ Patch (all phases implemented)
✅ Validate (all tests PASS)
```

**Status: IMPLEMENTATION COMPLETE AND VALIDATED**