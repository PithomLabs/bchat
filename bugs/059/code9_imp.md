# code9.md Implementation Review Checklist

**Purpose:** Checklist for verifying the implementation of code9.md fixes against all findings from the code8_review.md adversarial review.
**Plan source:** `code9.md` (fixes code8_review.md gates)
**Review chain:** code8_review.md → code9.md
**Scope:** 13 steps, 12 files modified/created, ~120 LOC changes + ~350 LOC new tests
**Status:** All steps implemented and verified

---

## 1. Executive Summary

code9.md addresses the code8_review.md findings (R-C1, R-C2, Section 3, N-N1…N-N6) that were raised against code8.md. The implementation:

- **Establishes a map-output contract** for node outputs (Fix 1) with recursive float normalization
- **Adds tolerant node-condition evaluation** (Fix 5b) so runtime eval errors log-and-skip instead of failing workflows
- **Relocates tests to `store/test/`** (R-C1) using the established `NewTestingStore` pattern
- **Applies tenant nullability fix to all 4 DDL files** including CockroachDB (R-C2)
- **Adds pg/crdb parity assertion** (Check 2c in validate-parity.sh)
- **Discovers CEL DynType behavior** — map-vs-string comparison returns `false` gracefully, not an error

---

## 2. Behavioral Contract Changes

### 2.1 Map Output Contract (Fix 1)

After Fix 1, every completed node's state value is **always a `map[string]any`**:

| Input type | Output state value | Example |
|------------|-------------------|---------|
| JSON object | Parsed map (float64→int64 normalized) | `{"found":true}` → `map["found":true]` |
| JSON array | Wrapper `{"output": raw}` | `[1,2,3]` → `map["output":"[1,2,3]"]` |
| Plain string | Wrapper `{"output": raw}` | `"hello"` → `map["output":"hello"]` |
| Invalid JSON | Wrapper `{"output": raw}` | `"not json"` → `map["output":"not json"]` |
| Skipped node | Placeholder `{"output":"", "skipped":true}` | deps-not-met skip |

**Implications for tenant conditions:**
- Field access required: `search_kb.found`, `search_kb.ticket_id`
- Direct string comparison (`search_kb == "logged"`) returns `false` (not error) — CEL DynType handles gracefully
- This is a **contract change** from the raw-string behavior before Fix 1

### 2.2 Node-Condition Tolerance (Fix 5b)

Before Fix 5b, node-`Condition` eval errors were hard failures:
```go
return "", fmt.Errorf("eval condition for %s: %w", node.Name, err)
```

After Fix 5b, runtime eval errors are **logged and treated as not-met** (node skipped):
```go
slog.Warn("condition eval error, treating node as skipped",
    "node", node.Name, "condition", node.Condition, "error", err)
return "", nil
```

This covers:
- Missing-key errors (already tolerated by Fix 2)
- Type mismatch errors (map == string, etc.) — new coverage
- Compile errors (malformed expression) — also log-and-skip (uniform treatment)

### 2.3 CEL DynType Discovery (code8_review Section 3)

code8_review.md Section 3 identified that `search_kb == "logged"` on a map output would cause a CEL "no matching overload" error. Empirical testing revealed:

```
CEL DynType: map == string → returns false (no error)
```

This means the Section 3 contract change is **defensive rather than load-bearing** for the map-vs-string case. The test `TestEvalConditionDynamic_MapStringCompare_ReturnsFalse` documents this behavior.

---

## 3. Changes by File

### 3.1 `server/router/api/v1/agent/execution.go`

**Lines modified:** ~175-191, ~215-224, ~248-266, ~367-431
**Lines added:** ~70 (buildNodeOutput, normalizeNumbers, buildWorkflowOutput rewrite)

| Line | Change | Fix | Verification |
|------|--------|-----|-------------|
| 4 | Added `"strings"` import | Fix 1b | `grep '"strings"' execution.go` |
| 178 | Added `state[nodeName] = map[string]any{"output":"", "skipped":true}` on deps-not-met | Fix 2 (K-4) | Placeholder materializes skipped node in state |
| 190-191 | `state[nodeName] = buildNodeOutput(output)` replaces raw assignment | Fix 1 | `grep 'buildNodeOutput' execution.go` |
| 215-222 | Stop-condition eval errors logged instead of discarded | Fix 5 | `slog.Warn("stop condition eval error..."` present |
| 257-260 | Node-condition eval errors log-and-skip (no hard fail) | Fix 5b | `slog.Warn("condition eval error..."` present |
| 367-406 | `buildNodeOutput` + `normalizeNumbers` helpers | Fix 1 | Recursive float64→int64 normalization |
| 408-431 | `buildWorkflowOutput` unwraps `"output"` key for display | Fix 1b | `strings.Join(parts, "\n")` replaces manual join |

**Spot-check:**
- [ ] `buildNodeOutput("not json")` returns `{"output":"not json"}`
- [ ] `buildNodeOutput('{"count":5}')` returns `{"count":int64(5)}` (not float64)
- [ ] `buildNodeOutput('{"nested":{"x":10}}')` normalizes nested `x` to int64
- [ ] `buildWorkflowOutput({"search_kb":{"output":"logged"}}, {"search_kb":true})` shows `search_kb: logged` (not `map[...]`)
- [ ] Stop-condition eval error produces `slog.Warn` (not swallowed)

### 3.2 `server/router/api/v1/agent/evaluator.go`

**Lines modified:** 137-140

| Line | Change | Fix | Verification |
|------|--------|-----|-------------|
| 140 | Added `strings.Contains(msg, "no such attribute")` | Fix 2 | `grep 'no such attribute' evaluator.go` |

**Spot-check:**
- [ ] `isMissingKeyError` catches all 4 terms: "no such key", "missing variable", "undeclared identifier", "no such attribute"

### 3.3 `server/router/api/v1/agent/service.go`

**Lines modified:** 3654-3658

| Line | Change | Fix | Verification |
|------|--------|-----|-------------|
| 3657 | `h.Execute(ctx, args, vars)` with `vars := map[string]any{"tenant_id": config.TenantID}` | Fix 3 | `grep 'vars := map\[string\]any{"tenant_id"' service.go` |

**Spot-check:**
- [ ] Chat-path `llm_call` passes `vars` (not `nil`) to `Execute`
- [ ] `GenerateFn` receives `int32` tenant_id → resolves to model/key

### 3.4 `server/router/api/v1/agent/handlers.go`

**Lines modified:** 6884-6886

| Line | Change | Fix | Verification |
|------|--------|-----|-------------|
| 6885 | `if limit < 1 { limit = 1 }` after the >200 cap | Fix 7 | `grep 'limit < 1' handlers.go` |

**Spot-check:**
- [ ] `?limit=0` returns 1 row
- [ ] `?limit=-5` returns 1 row
- [ ] `?limit=100` returns 100 rows (unchanged)

### 3.5 DDL Files (4 files)

All 4 files changed `NOT NULL` → `DEFAULT NULL` on `tenant_id` for both tables:

| File | Lines changed | Change |
|------|--------------|--------|
| `store/migration/postgres/0.36/00__add_skill_executions.sql` | 5, 33 | `BIGINT NOT NULL` → `BIGINT DEFAULT NULL` |
| `store/migration/postgres/LATEST.sql` | 1035, 1063 | Same |
| `store/migration/cockroach/0.36/00__add_skill_executions.sql` | 5, 33 | `INT8 NOT NULL` → `INT8 DEFAULT NULL` |
| `store/migration/cockroach/LATEST.sql` | 1035, 1063 | Same |

**Spot-check:**
- [ ] SQLite `INTEGER DEFAULT NULL` — unchanged (already correct)
- [ ] MySQL `INT DEFAULT NULL` — unchanged (already correct)
- [ ] Postgres + Cockroach `DEFAULT NULL` — applied in lockstep

### 3.6 `scripts/validate-parity.sh`

**Lines added:** ~315-328 (Check 2c)

| Line | Change | Fix | Verification |
|------|--------|-----|-------------|
| 315-328 | Check 2c: pg/crdb nullability parity grep | Fix 6b | `task validate:parity` shows Check 2c PASS |

**Spot-check:**
- [ ] `grep -c 'tenant_id BIGINT DEFAULT NULL' postgres/LATEST.sql` returns 2
- [ ] `grep -c 'tenant_id INT8 DEFAULT NULL' cockroach/LATEST.sql` returns 2
- [ ] Future NOT NULL regression on one side fails Check 2c

### 3.7 `server/router/api/v1/agent/evaluator_test.go`

**Lines added:** ~130 (9 new test functions)

| Test | Fix | What it verifies |
|------|-----|-----------------|
| `TestEvalConditionDynamic_WrapperMap_NoFieldAccess` | Fix 4a / N-N1 | Wrapper map with missing field → Met=false, no error |
| `TestEvalConditionDynamic_RawString_NoFieldAccess` | Fix 4a / N-N1 | Raw Go string → field access yields nil → Met=false |
| `TestEvalConditionDynamic_MapStringCompare_ReturnsFalse` | Section 3 | map vs string comparison → Met=false (DynType graceful) |
| `TestBuildNodeOutput_FlatJSON` | Fix 1 | Flat JSON → parsed map with bool/string |
| `TestBuildNodeOutput_NestedJSON` | Fix 1 / N-N3 | Nested JSON → int64 normalization at depth |
| `TestBuildNodeOutput_ArrayJSON` | Fix 1 | JSON array → wrapper map |
| `TestBuildNodeOutput_PlainString` | Fix 1 | Plain string → wrapper map |
| `TestBuildNodeOutput_InvalidJSON` | Fix 1 | Invalid JSON → wrapper map |
| `TestBuildNodeOutput_NumberNormalization` | Fix 1 / N-N3 | Integral float64→int64, fractional stays float64 |

### 3.8 `store/test/skill_execution_test.go` (NEW)

**Lines:** ~180 (8-case round-trip lifecycle)
**Package:** `teststore` (uses `NewTestingStore` pattern)
**Driver:** Default sqlite (DRIVER unset)

| Case | What it verifies |
|------|-----------------|
| 1. Nil-TenantID create | R-6: NULL allowed on sqlite |
| 2. Tenant-filter scoping | CRITICAL-2: cross-tenant isolation |
| 3. Claim lifecycle | Running status, claimed_by set |
| 4. Stop | Stopped status, subsequent claim fails |
| 5. Fail with error message | K-1: ErrorMessage persisted |
| 6. Complete | Completed status, CompletedAt set |
| 7. Log round-trip | Nil tenant log, StartedAt non-zero |
| 8. Stop on pending row | Unclaimed row transitions to stopped |

**Spot-check:**
- [ ] Case 1 proves R-6 necessity (nil TenantID accepted)
- [ ] Case 2 proves tenant isolation (only 1 exec returned per tenant)

### 3.9 `store/test/skill_execution_postgres_test.go` (NEW)

**Lines:** ~200 (same 8 cases, postgres-gated)
**Package:** `teststore`
**Gate:** `getDriverFromEnv() != "postgres"` → skip

| Case | Postgres-specific verification |
|------|-------------------------------|
| 1. Nil-TenantID | Fails red before Fix 6 (NOT NULL), green after |
| 7. Log nil-tenant | TIMESTAMPTZ round-trip (not int64 epoch) |

**Spot-check:**
- [ ] With `DRIVER=postgres DSN=...` → all 8 cases pass
- [ ] Without postgres env → skipped (not failed)

---

## 4. Review Finding Traceability

| code8_review Finding | code9 Fix | Status | Evidence |
|---------------------|-----------|--------|----------|
| R-C1: `setupTestDB` won't compile | Fix 4b/4c: use `store/test/` with `NewTestingStore` | **Fixed** | `skill_execution_test.go` compiles and passes |
| R-C2: Parity assertion dropped | Fix 6b: Check 2c in validate-parity.sh | **Fixed** | `task validate:parity` Check 2c PASS |
| Section 3: map contract unstated | Fix 1 + Fix 5b + behavioral contract doc | **Fixed** | Documented in §2.1; tolerance in Fix 5b |
| N-N1: Raw-string test dropped | Fix 4a: both `_WrapperMap_` AND `_RawString_` tests | **Fixed** | Both tests present and passing |
| N-N2: Invented infra (`SKILL_PG_INTEGRATION_TEST`) | Fix 4c: uses `getDriverFromEnv()` pattern | **Fixed** | `skill_execution_postgres_test.go:15` |
| N-N3: Float normalization top-level only | Fix 1: `normalizeNumbers` is recursive | **Fixed** | `TestBuildNodeOutput_NestedJSON` passes |
| N-N4: buildWorkflowOutput scope | Fix 1b: unwrap correct for wrapper; maps render as `map[...]` | **Accepted** | Spot-check scoped honestly |
| N-N5: `task validate:pg-migrations` nonexistent | Fix verification: uses `./scripts/validate-pg-migrations.sh` | **Fixed** | Verification section corrected |
| N-N6: Deferred attribution wrong | Deferred table added with corrected attributions | **Fixed** | §8 below |

---

## 5. Test Coverage Matrix

### New Tests (code9.md)

| Test file | Test count | Lines | Package |
|-----------|-----------|-------|---------|
| `evaluator_test.go` | 9 | ~130 | `agent` (internal) |
| `skill_execution_test.go` | 1 | ~180 | `teststore` |
| `skill_execution_postgres_test.go` | 1 | ~200 | `teststore` |

### Coverage by scenario

| Scenario | Test(s) |
|----------|---------|
| EvalConditionDynamic: wrapper map field access | `TestEvalConditionDynamic_WrapperMap_NoFieldAccess` |
| EvalConditionDynamic: raw string field access | `TestEvalConditionDynamic_RawString_NoFieldAccess` |
| EvalConditionDynamic: map vs string comparison | `TestEvalConditionDynamic_MapStringCompare_ReturnsFalse` |
| buildNodeOutput: flat JSON | `TestBuildNodeOutput_FlatJSON` |
| buildNodeOutput: nested JSON int normalization | `TestBuildNodeOutput_NestedJSON`, `TestBuildNodeOutput_NumberNormalization` |
| buildNodeOutput: non-object JSON (array) | `TestBuildNodeOutput_ArrayJSON` |
| buildNodeOutput: non-JSON (string) | `TestBuildNodeOutput_PlainString` |
| buildNodeOutput: invalid JSON | `TestBuildNodeOutput_InvalidJSON` |
| SkillExecution lifecycle (sqlite) | `TestSkillExecutionRoundTrip` (8 cases) |
| SkillExecution lifecycle (postgres) | `TestSkillExecutionRoundTrip_Postgres` (8 cases, gated) |
| Tenant isolation | Case 2 in both test files |
| Nil tenant NULLability | Case 1 in both test files |
| Error message persistence | Case 5 in both test files |
| TIMESTAMPTZ round-trip | Case 6 + timestamp assertions in postgres test |

---

## 6. DDL Changes Summary

### Tenant Nullability

| Driver | Table | Before | After |
|--------|-------|--------|-------|
| SQLite | `agent_skill_executions.tenant_id` | `INTEGER DEFAULT NULL` | No change |
| SQLite | `agent_skill_logs.tenant_id` | `INTEGER DEFAULT NULL` | No change |
| MySQL | `agent_skill_executions.tenant_id` | `INT DEFAULT NULL` | No change |
| MySQL | `agent_skill_logs.tenant_id` | `INT DEFAULT NULL` | No change |
| Postgres | `agent_skill_executions.tenant_id` | `BIGINT NOT NULL` | `BIGINT DEFAULT NULL` |
| Postgres | `agent_skill_logs.tenant_id` | `BIGINT NOT NULL` | `BIGINT DEFAULT NULL` |
| CockroachDB | `agent_skill_executions.tenant_id` | `INT8 NOT NULL` | `INT8 DEFAULT NULL` |
| CockroachDB | `agent_skill_logs.tenant_id` | `INT8 NOT NULL` | `INT8 DEFAULT NULL` |

### Files changed in lockstep

| File | Change |
|------|--------|
| `store/migration/postgres/0.36/00__add_skill_executions.sql` | Lines 5, 33 |
| `store/migration/postgres/LATEST.sql` | Lines 1035, 1063 |
| `store/migration/cockroach/0.36/00__add_skill_executions.sql` | Lines 5, 33 |
| `store/migration/cockroach/LATEST.sql` | Lines 1035, 1063 |

---

## 7. Verification Results

| Gate | Command | Result |
|------|---------|--------|
| Build | `go build ./...` | PASS |
| Agent tests | `go test ./server/router/api/v1/agent/...` | PASS (10.9s) |
| Store tests | `go test ./store/...` | PASS (8.5s) |
| Migration validation | `task validate:migrations` | PASS |
| Schema validation | `task validate:schema` | PASS |
| Parity validation | `task validate:parity` | PASS (incl. Check 2c) |

---

## 8. Deferred Items

| Item | Origin | Status |
|------|--------|--------|
| R-8 | EmitEvent dispatch on stop | Deferred per code6_imp_review.md (INFO; not a regression) |
| D2 | Chat-path durable loop | Deferred per code6.md (plan6 conformance table: DEFERRED) |
| MaxRetries/Timeout honored | retry_count not driving retry | Deferred per code6.md |
| MED-3 | retry/backoff policy | Deferred (code3/4 chain; subsumed by MaxRetries row) |
| MED-4 | Graceful shutdown of worker goroutines | Deferred (code4 chain) |
| LOW-2 | CEL program caching | Deferred (code2 chain; not closed) |
| LOW-4 | Recovery graph JSON dedup | Deferred (code2 chain; not closed) |
| Cadence | 30s ticker | Minor deviation, acceptable per code6.md |

---

## 9. Known Limitations

| Limitation | Impact | Mitigation |
|-----------|--------|------------|
| `buildWorkflowOutput` renders parsed-JSON maps as `map[...]` | Intermediate facts display as raw map | By design — wrapper outputs unwrap correctly; spot-check scoped to wrapper case |
| CEL DynType `map == string` returns `false` | Section 3 contract change is defensive, not load-bearing | Documented; Fix 5b provides tolerance for future CEL behavior changes |
| `normalizeNumbers` saturates at int64 max | Float64 > 2^63 → int64 max | Pathological JSON; acceptable |
| Postgres round-trip tests gated (skip without env) | No CI coverage without `DRIVER=postgres` | Pattern matches existing `agent_lead_postgres_test.go` |
