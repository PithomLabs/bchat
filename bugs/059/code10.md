# code10.md — Revised Fix Plan (fires code9_imp_review.md gates)

**Source:** code9_imp_review.md (DeepSeek adversarial review)
**Date:** 2026-08-05
**Scope:** R-F1 (same-worker carve-out), R-F2 (compile vs runtime error separation), N-1 through N-4 nits
**Status:** Ready for implementation

---

## Fixes

| Step | Fix | Files | Depends On |
|------|-----|-------|------------|
| 1 | R-F1 — same-worker carve-out in sqlite driver | `store/db/sqlite/agent_skill.go` | — |
| 2 | R-F1 — same-worker carve-out in postgres driver | `store/db/postgres/agent_skill.go` | — |
| 3 | R-F1 — add re-claim assertion to sqlite test | `store/test/skill_execution_test.go` | Step 1 |
| 4 | R-F1 — verify postgres test re-claim (already asserted) | `store/test/skill_execution_postgres_test.go` | Step 2 |
| 5 | R-F2 — add CompileError type to evaluator | `server/router/api/v1/agent/evaluator.go` | — |
| 6 | R-F2 — hard-fail on compile, tolerate runtime in executeStep | `server/router/api/v1/agent/execution.go` | Step 5 |
| 7 | R-F2 — add exec_id to node-condition warn | `server/router/api/v1/agent/execution.go` | Step 6 |
| 8 | N-1 — gofmt indentation in service.go | `server/router/api/v1/agent/service.go` | — |
| 9 | N-2 — scope Check 2c grep to skill table blocks | `scripts/validate-parity.sh` | — |
| 10 | N-3 — fix normalizeNumbers rationale doc | `server/router/api/v1/agent/execution.go` | — |
| 11 | N-4 — document output-contract LLM impact | `server/router/api/v1/agent/skill_builtins.go` | — |

---

### R-F1 — Same-worker carve-out (Steps 1-4)

**Problem:** code9.md Fix 4b specified "second claim same worker → success (lease re-entry)" but the driver's WHERE clause has no same-worker carve-out. The postgres test asserts re-claim succeeds and will fail on real PG/CRDB.

**Resolution:** Implement same-worker carve-out in both drivers.

#### Step 1 — sqlite driver (`store/db/sqlite/agent_skill.go:163-168`)

Change the ClaimSkillExecution WHERE clause:

```sql
-- Before:
WHERE id = ?
    AND (status = 'pending' OR (status = 'running' AND claim_expires_at < ?))

-- After:
WHERE id = ?
    AND (status = 'pending'
         OR (status = 'running' AND (claim_expires_at < ? OR claimed_by = ?)))
```

Bind `workerID` as the new parameter (after `now`):

```go
result, err := d.db.ExecContext(ctx, stmt, now, workerID, expiresAt, id, now, workerID)
```

#### Step 2 — postgres driver (`store/db/postgres/agent_skill.go:130-135`)

Same change with positional params:

```sql
-- Before:
WHERE id = $4
    AND (status = 'pending' OR (status = 'running' AND claim_expires_at < $5))

-- After:
WHERE id = $4
    AND (status = 'pending'
         OR (status = 'running' AND (claim_expires_at < $5 OR claimed_by = $6)))
```

Add `workerID` as `$6`:

```go
result, err := d.db.ExecContext(ctx, stmt, now, workerID, expiresAt, id, now, workerID)
```

#### Step 3 — sqlite test (`store/test/skill_execution_test.go`)

Add re-claim assertion after Case 3 claim, before Case 4 stop:

```go
// Re-claim by same worker (lease re-entry) — should succeed
_, err = ts.ClaimSkillExecution(ctx, tenantExec.ID, "worker-1", 60)
require.NoError(t, err)
```

#### Step 4 — postgres test (`store/test/skill_execution_postgres_test.go`)

Already asserts re-claim at line 80-82. No change needed — it will now pass with the driver fix.

**Behavioral note:** A different worker cannot re-claim an execution with an unexpired lease. Only the original claimer can re-claim (lease renewal pattern). This is intentional — prevents a crashed worker's execution from being silently stolen back.

---

### R-F2 — Compile vs runtime error separation (Steps 5-7)

**Problem:** Fix 5b folds ALL node-condition eval errors into `slog.Warn` + skip. A tenant condition typo (`search_kb.found == treu`) silently skips the node forever. Compile errors indicate real graph bugs and should fail the workflow.

#### Step 5 — add CompileError type (`evaluator.go`)

```go
// CompileError indicates a CEL expression failed to compile (graph bug).
type CompileError struct {
    Expr string
    Err  error
}

func (e *CompileError) Error() string {
    return fmt.Sprintf("cel compile %q: %v", e.Expr, e.Err)
}

func (e *CompileError) Unwrap() error { return e.Err }
```

Update `EvalConditionDynamic` to return `CompileError` for compile failures:

```go
ast, issues := env.Compile(expr)
if issues != nil {
    return nil, &CompileError{Expr: expr, Err: fmt.Errorf("%v", issues)}
}
```

The `env.Program(ast)` error is also a compile-phase error:

```go
prg, err := env.Program(ast)
if err != nil {
    return nil, &CompileError{Expr: expr, Err: err}
}
```

#### Step 6 — hard-fail on compile, tolerate runtime (`execution.go:254-266`)

```go
if node.Condition != "" {
    result, err := EvalConditionDynamic(ctx, node.Condition, celVars, graph)
    if err != nil {
        var compileErr *CompileError
        if errors.As(err, &compileErr) {
            // Compile errors = graph bug → hard fail
            return "", fmt.Errorf("condition compile error for %s: %w", node.Name, err)
        }
        // Runtime eval errors → log and treat as not-met
        slog.Warn("condition eval error, treating node as skipped",
            "exec_id", exec.ID, "node", node.Name,
            "condition", node.Condition, "error", err)
        return "", nil
    }
    if !result.Met {
        slog.Debug("condition not met, skipping", "node", node.Name, "condition", node.Condition)
        return "", nil
    }
}
```

#### Step 7 — exec_id already in warn (Step 6 above)

The `exec_id` is included in the warn log in Step 6. No separate step needed.

---

### N-1 — gofmt indentation (`service.go:3653-3658`)

**Problem:** Lines 3653-3658 have inconsistent indentation (4 tabs vs surrounding 3 tabs).

**Fix:** Run `gofmt -w` on the affected region, or manually fix to match the surrounding block indentation (3 tabs for the `if config.TenantID` block, consistent with the enclosing `if ok {` at line 3648).

The corrected block should be:

```go
            if ok {
                args := make(map[string]string)
                if tc.Function.Arguments != "" {
                    json.Unmarshal([]byte(tc.Function.Arguments), &args)
                }
                // N5-3: inject tenant_id for tenant-aware handlers
                if config.TenantID != 0 {
                    args["tenant_id"] = strconv.Itoa(int(config.TenantID))
                }
                vars := map[string]any{"tenant_id": config.TenantID}
                result, err = h.Execute(ctx, args, vars)
                if err != nil {
                    result = fmt.Sprintf("error: %v", err)
                }
            } else {
```

---

### N-2 — scope Check 2c grep (`validate-parity.sh:316-328`)

**Problem:** Check 2c greps the entire LATEST files for `tenant_id BIGINT DEFAULT NULL` / `INT8 DEFAULT NULL`. A future unrelated line or single-table regression keeps count ≥2 and passes.

**Fix:** Scope the grep to only the `agent_skill_executions` and `agent_skill_logs` table blocks. Use awk to extract lines between `CREATE TABLE ... agent_skill_executions` and the next `CREATE`, then grep within that block:

```bash
# Check 2c: Postgres/Cockroach tenant_id nullability parity (skill tables)
if [ -f "$POSTGRES_LATEST" ] && [ -f "$COCKROACH_LATEST" ]; then
    echo ""
    echo "Check 2c: Postgres/Cockroach tenant_id nullability parity"

    # Extract skill table blocks only
    pg_skill_tables=$(awk '/CREATE TABLE.*agent_skill_executions/,/^CREATE TABLE/' "$POSTGRES_LATEST" | head -n -1)
    pg_null_count=$(echo "$pg_skill_tables" | grep -c 'tenant_id BIGINT DEFAULT NULL' || true)
    cr_skill_tables=$(awk '/CREATE TABLE.*agent_skill_executions/,/^CREATE TABLE/' "$COCKROACH_LATEST" | head -n -1)
    cr_null_count=$(echo "$cr_skill_tables" | grep -c 'tenant_id INT8 DEFAULT NULL' || true)

    if [ "$pg_null_count" -lt 2 ] || [ "$cr_null_count" -lt 2 ]; then
        echo "  FAIL: skill table tenant_id nullability diverged (postgres=$pg_null_count crdb=$cr_null_count, need >=2 each)"
        SCHEMA_ISSUES=1
    else
        echo "  PASS: tenant_id nullability consistent (postgres=$pg_null_count, crdb=$cr_null_count)"
    fi
fi
```

**Note:** `awk ... | head -n -1` drops the trailing `CREATE TABLE` line that awk captures. This is POSIX-compatible (no bash 4+ required), matching the script's existing style.

---

### N-3 — fix normalizeNumbers rationale doc (`execution.go:379-380`)

**Problem:** The comment says "no int == double overload" but cel-go `Int.Equal` actually handles int==double (evaluates, doesn't error). normalizeNumbers is harmless canonicalization, not load-bearing.

**Fix:** Update the comment:

```go
// normalizeNumbers converts integral float64 (including nested) to int64 for
// canonical representation. CEL handles int==double natively, but canonicalizing
// avoids widening surprises in checkpoint serialization and keeps state maps tidy.
func normalizeNumbers(v any) {
```

---

### N-4 — document output-contract LLM impact (`skill_builtins.go`)

**Problem:** The output-contract change also reshapes the `llm_call` prompt context: `LLMHandler.Execute` marshals `vars` into the LLM prompt, so node outputs now appear as `{"output": ...}` / parsed maps instead of raw strings.

**Fix:** Add a comment in `skill_builtins.go` near the vars marshaling (around line 139-143):

```go
// NOTE: Node outputs in vars are now maps (buildNodeOutput contract),
// not raw strings. LLM prompts see {"output":"..."} / {"found":true,...}
// instead of bare text. Tenant scripts comparing on raw node text will
// see the wrapper. This is by design — CEL conditions use field access.
```

---

## Verification

```
go build ./...
go test ./store/... ./server/router/api/v1/agent/...
task validate:migrations
task validate:schema
task validate:parity
```

### Spot-checks:
- [ ] R-F1: sqlite re-claim by same worker succeeds (`TestSkillExecutionRoundTrip` Case 3)
- [ ] R-F1: sqlite re-claim by different worker with unexpired lease fails
- [ ] R-F1: postgres re-claim by same worker succeeds (`TestSkillExecutionRoundTrip_Postgres` Case 3)
- [ ] R-F2: `search_kb.found == treu` (typo) → hard workflow failure with compile error
- [ ] R-F2: `search_kb.nonexistent_field == false` → log-and-skip (runtime missing key)
- [ ] R-F2: node-condition warn includes `exec_id`
- [ ] N-1: `gofmt -l server/router/api/v1/agent/service.go` returns empty (no diffs)
- [ ] N-2: Check 2c grep scoped to skill tables (count exactly 2 per driver)
- [ ] N-3: normalizeNumbers comment updated
- [ ] N-4: skill_builtins.go comment present

---

## Files modified (complete manifest)

| # | File | Change |
|---|------|--------|
| 1 | `store/db/sqlite/agent_skill.go` | R-F1: same-worker carve-out in ClaimSkillExecution WHERE |
| 2 | `store/db/postgres/agent_skill.go` | R-F1: same-worker carve-out in ClaimSkillExecution WHERE |
| 3 | `store/test/skill_execution_test.go` | R-F1: add re-claim assertion in Case 3 |
| 4 | `server/router/api/v1/agent/evaluator.go` | R-F2: CompileError type + return on compile failure |
| 5 | `server/router/api/v1/agent/execution.go` | R-F2: errors.As(CompileError) hard-fail + exec_id in warn; N-3: comment fix |
| 6 | `server/router/api/v1/agent/service.go` | N-1: gofmt indentation |
| 7 | `scripts/validate-parity.sh` | N-2: scope Check 2c grep to skill tables |
| 8 | `server/router/api/v1/agent/skill_builtins.go` | N-4: output-contract LLM impact comment |
