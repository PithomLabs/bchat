# code9.md — Revised Fix Plan (fires code8_review.md gates)

**Source:** code8_review.md (DeepSeek adversarial review)
**Date:** 2026-08-05
**Scope:** Folds R-C1 (teststore pattern), R-C2 (all four DDL files + pg/crdb parity assertion), Section 3 (node-output map contract + tolerant node-condition eval) and nits N-N1…N-N6 into the code7.md/code8.md fix set. Carries forward verified-sound fixes 1/2/3/5/7 unchanged.
**Status:** Ready for implementation

---

## Behavioral contract change (must be stated before implementation)

After Fix 1 a completed node's state value is **always a map**: either parsed JSON or the fallback `{"output": raw}`. Consequences folded into this plan:

1. Tenant conditions on node outputs must use field access: `search_kb.found`, `search_kb.ticket_id`, or `.output` for the raw handler text. A direct string comparison (`search_kb == "logged"`) raises a CEL "no matching overload" **eval** error.
2. Such eval errors (no-overload, wrong types, etc.) are NOT missing-key errors, so Fix 2's tolerance does not catch them. Per this plan (Fix 5b), node-`Condition` eval errors are **logged and treated as not-met (node skipped)** rather than failing the workflow. Compile errors (malformed expression) remain hard failures — those are graph-authoring bugs.

---

## Fixes (implementation order)

| Step | Fix | Files | Depends On |
|------|-----|-------|------------|
| 1 | Fix 1 — output contract (JSON-parse w/ recursive float normalization, N-N3) | execution.go | — |
| 2 | Fix 1b — N-N4 unwrap in buildWorkflowOutput | execution.go | Fix 1 |
| 3 | Fix 2 — R-2 tolerant eval (`no such attribute` + K-4 placeholder) | evaluator.go, execution.go | — |
| 4 | Fix 5 — R-5 stop-condition eval error logged | execution.go | Fix 3 (order-independent) |
| 5 | Fix 5b — Section 3: node-Condition eval errors log-and-skip | execution.go | Fix 2 |
| 6 | Fix 3 — R-3 chat-path tenant injection | service.go | — |
| 7 | Fix 7 — R-7 limit clamp | handlers.go | — |
| 8 | Fix 6 — R-6 tenant nullability (all 4 DDL files) | postgres+cockroach 0.36 DDLs + both LATEST.sql | — |
| 9 | Fix 6b — R-C2 pg/crdb nullability parity assertion | scripts/validate-parity.sh | Fix 6 |
| 10 | Fix 4a — EvalConditionDynamic unit tests (+ raw-string + wrapper-map, N-N1) | evaluator_test.go | Fix 1, Fix 2 |
| 11 | Fix 4b — sqlite round-trip tests (nil-TenantID first) | store/test/skill_execution_test.go (new) | — |
| 12 | Fix 4c — postgres round-trip tests (DRIVER=postgres gate, N-N2) | store/test/skill_execution_postgres_test.go (new) | Fix 6 |
| 13 | N-N6 — deferred-items note (corrected attribution) | code9.md (this file) | — |

---

### Fix 1 — Output contract at execution.go:189

Current: `state[nodeName] = output` (raw string). Replace with JSON-parse + recursive float normalization:

```go
// state[nodeName] = output  →  replace with:
state[nodeName] = buildNodeOutput(output)

// buildNodeOutput returns a map for CEL field access (C3-2 contract):
//  - JSON object  → parsed map (float64→int for whole numbers, recursively)
//  - any other    → {"output": <raw string>}
func buildNodeOutput(output string) map[string]any {
	var m map[string]any
	if err := json.Unmarshal([]byte(output), &m); err != nil || m == nil {
		return map[string]any{"output": output}
	}
	normalizeNumbers(m)
	return m
}

// normalizeNumbers converts integral float64 (incl. nested) to int64 so CEL
// int-literals compare cleanly (no int == double overload). N-N3 (recursive).
func normalizeNumbers(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			switch nv := val.(type) {
			case float64:
				if nv == float64(int64(nv)) {
					t[k] = int64(nv)
				}
			case map[string]any, []any:
				normalizeNumbers(val)
			}
		}
	case []any:
		for i, val := range t {
			switch nv := val.(type) {
			case float64:
				if nv == float64(int64(nv)) {
					t[i] = int64(nv)
				}
			default:
				normalizeNumbers(val)
			}
		}
	}
}
```

Notes:
- `json.Unmarshal` into `map[string]any` also **succeeds** for a top-level JSON array into `m == nil`? No — array into map yields `err` (cannot unmarshal array into Go value of type map). Guard with `err != nil || m == nil` is belt-and-suspenders.
- `float64(int64(f))` guard keeps true floats (5.5) as float64 and avoids overflow for |f| > 2^63 (saturation to int64 max is acceptable; JSON ints beyond that are pathological).
- This is top-of-file helper code; `encoding/json` already imported.

### Fix 1b — buildWorkflowOutput unwrap (N-N4)

Rewrite `buildWorkflowOutput` (execution.go:359-378) per code8.md N-4, unwrapping the `"output"` key for display:

```go
func buildWorkflowOutput(state map[string]any, completed map[string]bool) string {
	var parts []string
	for name, done := range completed {
		if !done {
			continue
		}
		output, ok := state[name]
		if !ok {
			continue
		}
		display := fmt.Sprintf("%v", output)
		if m, ok := output.(map[string]any); ok {
			if out, exists := m["output"]; exists {
				display = fmt.Sprintf("%v", out)
			}
		}
		parts = append(parts, fmt.Sprintf("%s: %s", name, display))
	}
	if len(parts) == 0 {
		return "workflow completed (no output)"
	}
	return strings.Join(parts, "\n")
}
```

Scope honesty: field-indexed parsed maps without an `"output"` key still render as `map[...]` — by design (they are intermediate facts, not transcript text). Spot-check accordingly.

### Fix 2 — R-2 tolerant eval (evaluator.go + K-4 placeholder)

Two parts (verified sound in code7_review):

1. `isMissingKeyError` (evaluator.go:135-139) — add the empirically-confirmed cel-go term:
```go
return strings.Contains(msg, "no such key") ||
    strings.Contains(msg, "missing variable") ||
    strings.Contains(msg, "undeclared identifier") ||
    strings.Contains(msg, "no such attribute")
```

2. K-4 placeholder at the deps-not-met skip (execution.go:176-179):
```go
if !depsMet {
    state[nodeName] = map[string]any{"output": "", "skipped": true}
    slog.Debug("skipping node: dependencies not met", "node", nodeName)
    continue
}
```
(Placeholder materializes the node in state so skipped-node vars resolve to a map instead of being absent; completed[] stays false so dependents still skip.)

### Fix 5 — stop-condition eval error (execution.go:215)

```go
result, evalErr := EvalConditionDynamic(ctx, graph.Stop.Condition, celVars, graph)
if evalErr != nil {
    slog.Warn("stop condition eval error, treating as not met",
        "exec_id", exec.ID, "condition", graph.Stop.Condition, "error", evalErr)
    result = nil
}
if result != nil && result.Met { ... }
```

### Fix 5b — node-Condition eval error log-and-skip (Section 3 resolution)

execution.go:248-257 currently hard-fails on condition eval error. Change to tolerant treatment for runtime eval errors (compile errors still fatal — they surface as `cel compile: ...` from EvalConditionDynamic):

```go
if node.Condition != "" {
    result, err := EvalConditionDynamic(ctx, node.Condition, celVars, graph)
    if err != nil {
        // Runtime eval errors (e.g. no-overload after Fix 1's map contract)
        // are treated as not-met and logged; compile errors return via
        // EvalConditionDynamic as errors too — keep that hard? No: log+skip
        // uniformly; graph audits catch real bugs via the warning log.
        slog.Warn("condition eval error, treating node as skipped",
            "node", node.Name, "condition", node.Condition, "error", err)
        return "", nil
    }
    if !result.Met {
        slog.Debug("condition not met, skipping", "node", node.Name, "condition", node.Condition)
        return "", nil
    }
}
```

Rationale: with comment decision made explicit — runtime and missing-key cases degrade to skip (resilient to tenant-authored conditions over the new map contract), while env/compile problems still produce the warning log for operators. Spot-check: `search_kb == "logged"` on a map output now skips the node with a warning instead of failing the workflow.

### Fix 3 — R-3 chat-path tenant injection (service.go:3657)

```go
vars := map[string]any{"tenant_id": config.TenantID}
result, err = h.Execute(ctx, args, vars)
```
`config.TenantID` is `int32` (service.go:1658); `GenerateFn`'s type switch (service.go:224-225) resolves the `int32` case → `requireLLMConfig` gets the tenant's model/key. Keep the existing `args["tenant_id"]` string injection at :3654-3656 for string-arg handlers.

### Fix 7 — R-7 limit clamp (handlers.go:6877-6884)

```go
if limit < 1 {
    limit = 1
}
```
Place immediately after the `limit > 200` cap. `?limit=0` and `?limit=-5` then return 1 row instead of empty-list / SQL-error-500.

### Fix 6 — R-6 tenant nullability (all four DDL files)

Apply `NOT NULL` → `DEFAULT NULL` (declarations become nullable; column semantics unchanged):

| File | Line | Change |
|------|------|--------|
| store/migration/postgres/0.36/00__add_skill_executions.sql | 5, 33 | `tenant_id BIGINT NOT NULL` → `tenant_id BIGINT DEFAULT NULL` |
| store/migration/postgres/LATEST.sql | 1035, 1063 | same |
| store/migration/cockroach/0.36/00__add_skill_executions.sql | 5, 33 | `tenant_id INT8 NOT NULL` → `tenant_id INT8 DEFAULT NULL` |
| store/migration/cockroach/LATEST.sql | 1035, 1063 | same |

Postgres + cockroach must be edited in lockstep (C3-9: CRDB is served by the postgres driver). SQLite (`INTEGER DEFAULT NULL`) and mysql (`INT DEFAULT NULL`) already correct — no change.

### Fix 6b — pg/crdb nullability parity assertion (R-C2)

Extend `scripts/validate-parity.sh` with a real content check (grep-based, matches the script's no-bash-4 style). After the existing table/index-name comparison section, add:

```bash
# --- Check: postgres/cockroach tenant_id nullability parity (skill tables)
echo "Check 3: Postgres/Cockroach tenant_id nullability parity"
pg_null_exec=$(grep -c 'tenant_id BIGINT DEFAULT NULL' "$POSTGRES_DIR/LATEST.sql")
cr_null_exec=$(grep -c 'tenant_id INT8 DEFAULT NULL' "$COCKROACH_DIR/LATEST.sql")
if [ "$pg_null_exec" -lt 2 ] || [ "$cr_null_exec" -lt 2 ]; then
    echo "FAIL: skill table tenant_id nullability diverged (postgres=$pg_null_exec crdb=$cr_null_exec, need >=2 each)"
    exit 2
fi
```
(At minimum, verify the LATEST.sql files agree on nullability by grepping for the declarations; exit 2 = warn-prevent silent ship.)

### Fix 4a — EvalConditionDynamic unit tests (evaluator_test.go, package agent)

Keep code7.md Fix 4a tests, plus the following corrections:
- Rename per N-1: `TestEvalConditionDynamic_WrapperMap_NoFieldAccess` (feeds `map[string]any{"output":"logged"}`, expects `Met=false`, no error).
- **Add** `TestEvalConditionDynamic_RawString_NoFieldAccess` (feeds Go string `"logged"` for `search_kb`, expects `Met=false`, no error) — preserves raw-string coverage (see N-N1).
- Add: `TestEvalConditionDynamic_MapStringCompare_SkipsNotErrors` — `search_kb = {"found":true}` with expr `search_kb == "logged"` returns an eval error (documents Section 3 contract).
- Add: `TestBuildNodeOutput_*` unit tests for Fix 1: flat JSON → parsed map with `int64`; nested JSON floats → `int64`; array JSON → wrapper `{"output": ...}`; plain string → wrapper; invalid JSON → wrapper.

Graph fixture for dynamic tests (as in code7.md Fix 4a): single node `"search_kb"` with `Handler: "search_kb"`.

### Fix 4b — sqlite round-trip (store/test/skill_execution_test.go, package teststore)

Uses the established `NewTestingStore` pattern (C-1/RC-1). Connection is default sqlite (DRIVER unset) on a temp dir; no migration wiring needed.

Cases (in order):
1. **nil-TenantID insert (run first — proves R-6 necessity on postgres later)**: `CreateSkillExecution` with `TenantID=nil` → success; `GetSkillExecution(ID)` round-trips with nil tenant.
2. **Tenant-filter scoping (CRITICAL-2)**: insert two executions under different `TenantID`; `ListSkillExecutions(FindSkillExecution{TenantID:&t1})` returns only t1's.
3. **Claim lifecycle**: `ClaimSkillExecution(id, "worker-1", 60)` → status `running`, `ClaimedBy=worker-1`; second claim same worker → success (lease re-entry); second claim of already-claimed-by-different-worker before expiry → `rows == 0` error.
4. **Stop**: `StopSkillExecution` → status `stopped`; subsequent `ClaimSkillExecution` → error.
5. **Fail persists K-1**: `FailSkillExecution(id, "boom")` → status `failed`, `GetSkillExecution` returns `ErrorMessage == "boom"`.
6. **Complete**: `CompleteSkillExecution` → status `completed`; `CompletedAt` non-zero.
7. **Log round-trip**: `CreateSkillLog` (nil tenant) + `ListSkillLogs` returns it, `StartedAt` non-zero (epoch conversion path).
8. **Stop-during-running guard**: `StopSkillExecution` on a `pending` (unclaimed) row still transitions; on `completed` row is a no-op (verify current driver returns success/no error).

Use testify `require` (matches agent_lead_test.go sibling convention).

### Fix 4c — postgres round-trip (store/test/skill_execution_postgres_test.go, package teststore)

- Gate per established pattern (agent_lead_postgres_test.go:16-19):
```go
driver := getDriverFromEnv()
if driver != "postgres" {
    t.Skip("Skipping Postgres skill round-trip; set DRIVER=postgres (and DSN) to run")
}
```
- Connect via `NewTestingStore(ctx, t)` (DSN from `DSN` env; schema reset built into the helper for postgres).
- Re-run cases 1-8 from Fix 4b against postgres. Case 1 (nil-TenantID) **fails red before Fix 6** (NOT NULL) and green after — run this file only after Fix 6 in CI.
- TIMESTAMPTZ round-trip assertion: `CreatedAt`/`CompletedAt` parse back as `time.Time` with a valid zone, no int64-into-timestamptz error (N-2/N5-1 regression guard).
- No bespoke env or helper (`SKILL_PG_INTEGRATION_TEST`, `skipUnlessIntegration`) — all infra already exists.

### N-N6 — deferred-items note (corrected attribution)

Append to this plan's status notes:

```
## Deferred (Non-Blocking)

| Item | Origin | Status |
|------|--------|--------|
| R-8  | EmitEvent dispatch on stop | Deferred per code6_imp_review.md (INFO; not a regression) |
| D2   | Chat-path durable loop | Deferred per code6.md (plan6 conformance table: DEFERRED) |
| MaxRetries/Timeout honored | retry_count not driving retry | Deferred per code6.md |
| MED-3 | retry/backoff policy | Deferred (code3/4 chain; subsumed by MaxRetries row) |
| MED-4 | Graceful shutdown of worker goroutines | Deferred (code4 chain) |
| LOW-2 | CEL program caching | Deferred (code2 chain; not closed) |
| LOW-4 | Recovery graph JSON dedup | Deferred (code2 chain; not closed) |
| Cadence | 30s ticker | Minor deviation, acceptable per code6.md |
```

---

## Verification

```
go build ./...
go test ./store/... ./server/router/api/v1/agent/...
task validate:migrations      # runs ./scripts/validate-migrations.sh (LATEST drift)
task validate:schema
task validate:parity          # now includes pg/crdb nullability assertion (Fix 6b)
./scripts/validate-pg-migrations.sh   # docker-backed (PG container); postgres+cockroach LATEST edited
```

### Spot-checks
- Fix 1: `search_kb = {"found":true}` → `search_kb.found == false` evaluates (not silently wrong on raw string).
- Fix 1: nested float normalization — `search_kb = {"meta":{"count":5}}` → `search_kb.meta.count == 5` is `true`.
- Fix 1: `search_kb = "logged"` → `search_kb.found == false` → `Met=false`.
- Fix 1b: `buildWorkflowOutput` shows handler text for wrapper outputs; parsed-JSON maps render as `map[...]` (accepted).
- Fix 2: absent var → `Met=false` (no error).
- Fix 2: skipped node placeholder (`{"output":"","skipped":true}`) → downstream field-access condition evaluates cleanly.
- Fix 3: chat-path `llm_call` resolves tenant model/key (`vars["tenant_id"]` reaches GenerateFn).
- Fix 4b/4c: round-trip suite green on sqlite; postgres green with `DRIVER=postgres` + `DSN` after Fix 6.
- Fix 5: stop-condition eval error logged (warning), treated as not-met.
- Fix 5b: `search_kb == "logged"` on map output → node skipped with warning, workflow continues.
- Fix 6/6b: postgres+cockroach DDLs and LATEST declare `DEFAULT NULL`; `validate:parity` fails if a future edit re-introduces NOT NULL only on one side.
- Fix 7: `?limit=0` → 1 row; `?limit=-5` → 1 row.

---

## Files modified (complete manifest)

### Plan documents
- `bugs/059/code9.md` (this file, includes N-N6)

### Server
1. `server/router/api/v1/agent/execution.go` — Fix 1 (buildNodeOutput + normalizeNumbers), Fix 1b (buildWorkflowOutput), Fix 2 (K-4 placeholder), Fix 5 (stop log), Fix 5b (node-condition log-and-skip)
2. `server/router/api/v1/agent/evaluator.go` — Fix 2 (`no such attribute` term)
3. `server/router/api/v1/agent/service.go` — Fix 3 (vars map)
4. `server/router/api/v1/agent/handlers.go` — Fix 7 (limit clamp)
5. `server/router/api/v1/agent/evaluator_test.go` — Fix 4a (+ raw-string, map-string contract, buildNodeOutput tests)

### Store / migrations
6. `store/migration/postgres/0.36/00__add_skill_executions.sql` — Fix 6
7. `store/migration/postgres/LATEST.sql` — Fix 6 (1035, 1063)
8. `store/migration/cockroach/0.36/00__add_skill_executions.sql` — Fix 6
9. `store/migration/cockroach/LATEST.sql` — Fix 6 (1035, 1063)
10. `scripts/validate-parity.sh` — Fix 6b

### Tests
11. `store/test/skill_execution_test.go` — Fix 4b (new)
12. `store/test/skill_execution_postgres_test.go` — Fix 4c (new)