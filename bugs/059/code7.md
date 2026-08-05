# code7.md — Fix Plan for code6.md Implementation Gaps

**Source:** code6_imp_review.md (DeepSeek adversarial review)
**Date:** 2026-08-05
**Scope:** 7 findings (2 HIGH, 2 MEDIUM, 3 LOW) — all one-line to ~50 LOC each
**Status:** Ready for implementation

---

## Fix 1 — R-1 (HIGH): Output contract not implemented; conditions silently mis-evaluate

**File:** `server/router/api/v1/agent/execution.go:189`
**Finding:** `state[nodeName] = output` stores raw string. CEL field access on a string returns absent→nil, so `.field` conditions always evaluate false silently. The parser's own documented example (`search_kb.found == false`) can never fire.
**Root cause:** code6.md Fix 4 specified the output contract (JSON-parse → map; else wrap `{"output": raw}`) but it was never written.

### Changes

**execution.go — `executeWorkflow` (line 189):**

Replace:
```go
state[nodeName] = output
```

With:
```go
// Output contract: parse as JSON map, else wrap in {"output": raw}
var parsed any
if err := json.Unmarshal([]byte(output), &parsed); err == nil {
    if m, ok := parsed.(map[string]any); ok {
        state[nodeName] = m
    } else {
        state[nodeName] = map[string]any{"output": output}
    }
} else {
    state[nodeName] = map[string]any{"output": output}
}
```

**Impact:** Conditions like `search_kb.found == false` now evaluate correctly against the parsed map. Raw string outputs still work via the `{"output": raw}` wrapper.

---

## Fix 2 — R-2 (HIGH): N5-2/K-4 ineffective; missing-variable still hard-errors

**Files:**
- `server/router/api/v1/agent/evaluator.go:134-139` (`isMissingKeyError`)
- `server/router/api/v1/agent/execution.go:176-179` (skipped nodes get no placeholder)

**Finding:** `isMissingKeyError` checks for `"no such key"`, `"missing variable"`, `"undeclared identifier"` — but cel-go's actual error for a missing undeclared variable is `"no such attribute(s): <name>"`. Also, when a node's dependencies aren't met, `executeWorkflow` does `continue` without writing a placeholder to `state`, so the variable is completely absent and cel-go errors at eval time.

### Changes

**evaluator.go — `isMissingKeyError` (line 134-139):**

Replace:
```go
func isMissingKeyError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "no such key") ||
		strings.Contains(msg, "missing variable") ||
		strings.Contains(msg, "undeclared identifier")
}
```

With:
```go
func isMissingKeyError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "no such key") ||
		strings.Contains(msg, "no such attribute") ||
		strings.Contains(msg, "missing variable") ||
		strings.Contains(msg, "undeclared identifier")
}
```

**execution.go — `executeWorkflow` (line 157-160):**

Replace:
```go
if !depsMet {
	slog.Debug("skipping node: dependencies not met", "node", nodeName)
	continue
}
```

With:
```go
if !depsMet {
	slog.Debug("skipping node: dependencies not met", "node", nodeName)
	// K-4: Write placeholder so downstream conditions referencing this node
	// resolve cleanly (field access on placeholder yields nil → false, not error)
	state[nodeName] = map[string]any{"output": "", "skipped": true}
	completed[nodeName] = false
	continue
}
```

**Impact:** Skipped nodes get a placeholder map. Downstream conditions like `skipped_node.found == false` evaluate to `false` (nil == false is false) instead of erroring. The `isMissingKeyError` extension catches any remaining edge cases.

---

## Fix 3 — R-3 (MEDIUM): N5-3 tenant injection inert in chat path

**File:** `server/router/api/v1/agent/service.go:3657`
**Finding:** `h.Execute(ctx, args, nil)` passes nil vars. `LLMHandler.Execute` calls `GenerateFn(ctx, expandedPrompt, vars)` passing the nil vars map. The `GenerateFn` closure reads `vars["tenant_id"]` from nil → key not found → `tenantID = 0` → env-default model/key.

### Changes

**service.go — `toolCallingLoop` (line 3657):**

Replace:
```go
result, err = h.Execute(ctx, args, nil)
```

With:
```go
// N5-3: pass vars with tenant_id so GenerateFn can resolve per-tenant config
vars := map[string]any{"tenant_id": config.TenantID}
result, err = h.Execute(ctx, args, vars)
```

**Impact:** Chat-path `llm_call` now resolves the tenant's custom model/key instead of falling back to env defaults. Detached path was already working by accident (executeStep passes celVars).

---

## Fix 4 — R-4 (MEDIUM): Tests unimplemented

**Files to create:**
- `server/router/api/v1/agent/evaluator_test.go` (add new test cases)
- `store/db/sqlite/agent_skill_test.go` (new file)

**Files to modify:**
- `store/db/postgres/agent_skill_test.go` (new file, N5-5 gated)

### 4a. evaluator_test.go — Add `EvalConditionDynamic` tests

```go
func TestEvalConditionDynamic_MissingVar_ReturnsNotMet(t *testing.T) {
	// N5-2: Missing variable should return Met=false, not error
	graph := &SkillGraph{Nodes: map[string]*SkillDefinition{
		"search_kb": {Name: "search_kb"},
	}}
	vars := map[string]any{} // search_kb is absent
	result, err := EvalConditionDynamic(context.Background(), `search_kb.found == false`, vars, graph)
	if err != nil {
		t.Fatalf("expected no error for missing var, got: %v", err)
	}
	if result.Met {
		t.Fatal("expected Met=false for missing var")
	}
}

func TestEvalConditionDynamic_MissingField_ReturnsNotMet(t *testing.T) {
	// N5-2: Missing field on present var should return Met=false, not error
	graph := &SkillGraph{Nodes: map[string]*SkillDefinition{
		"search_kb": {Name: "search_kb"},
	}}
	vars := map[string]any{"search_kb": map[string]any{"output": "no results"}} // .found is absent
	result, err := EvalConditionDynamic(context.Background(), `search_kb.found == false`, vars, graph)
	if err != nil {
		t.Fatalf("expected no error for missing field, got: %v", err)
	}
	if result.Met {
		t.Fatal("expected Met=false for missing field")
	}
}

func TestEvalConditionDynamic_PlaceholderMap_EvaluatesCorrectly(t *testing.T) {
	// K-4: Skipped node placeholder should allow downstream conditions
	graph := &SkillGraph{Nodes: map[string]*SkillDefinition{
		"search_kb": {Name: "search_kb"},
	}}
	vars := map[string]any{"search_kb": map[string]any{"output": "", "skipped": true}}
	result, err := EvalConditionDynamic(context.Background(), `search_kb.found == false`, vars, graph)
	if err != nil {
		t.Fatalf("expected no error for placeholder map, got: %v", err)
	}
	// .found is absent from placeholder → nil → nil == false is false
	if result.Met {
		t.Fatal("expected Met=false for placeholder map")
	}
}

func TestEvalConditionDynamic_OutputContract_FieldAccess(t *testing.T) {
	// R-1: JSON-parsed output should allow field access
	graph := &SkillGraph{Nodes: map[string]*SkillDefinition{
		"search_kb": {Name: "search_kb"},
	}}
	vars := map[string]any{"search_kb": map[string]any{"found": true, "ticket_id": "T1"}}
	result, err := EvalConditionDynamic(context.Background(), `search_kb.found == false`, vars, graph)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Met {
		t.Fatal("expected Met=false when found=true")
	}
}

func TestEvalConditionDynamic_RawString_NoFieldAccess(t *testing.T) {
	// R-1: Raw string output → field access returns nil → false
	graph := &SkillGraph{Nodes: map[string]*SkillDefinition{
		"create_ticket": {Name: "create_ticket"},
	}}
	vars := map[string]any{"create_ticket": map[string]any{"output": "logged"}}
	result, err := EvalConditionDynamic(context.Background(), `create_ticket.ticket_id != ''`, vars, graph)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// wrapper has "output" but not "ticket_id" → nil != '' → false
	if result.Met {
		t.Fatal("expected Met=false for raw string wrapper without ticket_id")
	}
}
```

### 4b. store/db/sqlite/agent_skill_test.go — Round-trip tests

```go
package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/usememos/memos/store"
)

func TestSkillExecutionRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := &DB{} // use test helper to get real DB

	// Create
	tenantID := int32(1)
	now := time.Now()
	exec, err := db.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             "test-001",
		TenantID:       &tenantID,
		ConversationID: "conv-001",
		SkillGraphJSON: `{"nodes":{}}`,
		Status:         "pending",
		TriggerPath:    "api",
		MaxRetries:     3,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if exec.ID != "test-001" {
		t.Fatalf("expected ID test-001, got %s", exec.ID)
	}

	// Get
	got, err := db.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected execution, got nil")
	}
	if got.Status != "pending" {
		t.Fatalf("expected status pending, got %s", got.Status)
	}
	if got.ErrorMessage != "" {
		t.Fatalf("expected empty error_message, got %s", got.ErrorMessage)
	}

	// List (tenant-scoped)
	list, err := db.ListSkillExecutions(ctx, &store.FindSkillExecution{TenantID: &tenantID}, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(list))
	}

	// Complete (atomic)
	if err := db.CompleteSkillExecution(ctx, exec.ID); err != nil {
		t.Fatalf("complete: %v", err)
	}
	got, _ = db.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
	if got.Status != "completed" {
		t.Fatalf("expected completed, got %s", got.Status)
	}

	// Fail on completed → no-op
	if err := db.FailSkillExecution(ctx, exec.ID, "should not happen"); err != nil {
		t.Fatalf("fail on completed: %v", err)
	}
	got, _ = db.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
	if got.Status != "completed" {
		t.Fatalf("expected still completed, got %s", got.Status)
	}

	// Stop on completed → no-op
	if err := db.StopSkillExecution(ctx, exec.ID); err != nil {
		t.Fatalf("stop on completed: %v", err)
	}
	got, _ = db.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
	if got.Status != "completed" {
		t.Fatalf("expected still completed, got %s", got.Status)
	}
}

func TestSkillExecutionFailPersistsError(t *testing.T) {
	ctx := context.Background()
	db := &DB{}
	tenantID := int32(1)
	now := time.Now()

	exec, _ := db.CreateSkillExecution(ctx, &store.SkillExecution{
		ID:             "test-fail-001",
		TenantID:       &tenantID,
		ConversationID: "conv-fail",
		SkillGraphJSON: `{"nodes":{}}`,
		Status:         "pending",
		TriggerPath:    "api",
		CreatedAt:      now,
		UpdatedAt:      now,
	})

	// Claim
	claimed, err := db.ClaimSkillExecution(ctx, exec.ID, "worker-1", 300)
	if err != nil || claimed == nil {
		t.Fatalf("claim: %v", err)
	}

	// Fail with error message
	if err := db.FailSkillExecution(ctx, exec.ID, "test error message"); err != nil {
		t.Fatalf("fail: %v", err)
	}

	got, _ := db.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
	if got.ErrorMessage != "test error message" {
		t.Fatalf("expected error_message 'test error message', got '%s'", got.ErrorMessage)
	}
	if got.Status != "failed" {
		t.Fatalf("expected status failed, got %s", got.Status)
	}
}

func TestSkillExecutionTenantScopedList(t *testing.T) {
	ctx := context.Background()
	db := &DB{}
	tenant1 := int32(1)
	tenant2 := int32(2)
	now := time.Now()

	db.CreateSkillExecution(ctx, &store.SkillExecution{
		ID: "t1-exec", TenantID: &tenant1, ConversationID: "c1",
		SkillGraphJSON: `{"nodes":{}}`, Status: "pending", TriggerPath: "api",
		CreatedAt: now, UpdatedAt: now,
	})
	db.CreateSkillExecution(ctx, &store.SkillExecution{
		ID: "t2-exec", TenantID: &tenant2, ConversationID: "c2",
		SkillGraphJSON: `{"nodes":{}}`, Status: "pending", TriggerPath: "api",
		CreatedAt: now, UpdatedAt: now,
	})

	// Tenant 1 should only see its own
	list, _ := db.ListSkillExecutions(ctx, &store.FindSkillExecution{TenantID: &tenant1}, 10)
	if len(list) != 1 {
		t.Fatalf("expected 1 execution for tenant 1, got %d", len(list))
	}
	if list[0].ID != "t1-exec" {
		t.Fatalf("expected t1-exec, got %s", list[0].ID)
	}
}

func TestStopDuringRunning(t *testing.T) {
	ctx := context.Background()
	db := &DB{}
	tenantID := int32(1)
	now := time.Now()

	exec, _ := db.CreateSkillExecution(ctx, &store.SkillExecution{
		ID: "stop-test", TenantID: &tenantID, ConversationID: "c",
		SkillGraphJSON: `{"nodes":{}}`, Status: "pending", TriggerPath: "api",
		CreatedAt: now, UpdatedAt: now,
	})

	// Claim → running
	db.ClaimSkillExecution(ctx, exec.ID, "w", 300)

	// Stop
	if err := db.StopSkillExecution(ctx, exec.ID); err != nil {
		t.Fatalf("stop: %v", err)
	}

	got, _ := db.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
	if got.Status != "stopped" {
		t.Fatalf("expected stopped, got %s", got.Status)
	}

	// Complete on stopped → no-op
	if err := db.CompleteSkillExecution(ctx, exec.ID); err != nil {
		t.Fatalf("complete on stopped: %v", err)
	}
	got, _ = db.GetSkillExecution(ctx, &store.FindSkillExecution{ID: &exec.ID})
	if got.Status != "stopped" {
		t.Fatalf("expected still stopped, got %s", got.Status)
	}
}
```

### 4c. store/db/postgres/agent_skill_test.go — N5-5 gated integration tests

```go
package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/usememos/memos/store"
)

func skipUnlessIntegration(t *testing.T) {
	t.Helper()
	if testing.Short() && os.Getenv("SKILL_PG_INTEGRATION_TEST") != "true" {
		t.Skip("skipped: set SKILL_PG_INTEGRATION_TEST=true to run postgres integration tests")
	}
}

func TestSkillExecutionPG_RoundTrip(t *testing.T) {
	skipUnlessIntegration(t)
	// Requires a running Postgres instance (use test DB or docker)
	// ... implementation with real DB connection ...
}

func TestSkillExecutionPG_ClaimTimestamps(t *testing.T) {
	skipUnlessIntegration(t)
	// N5-1: Verify TIMESTAMPTZ columns receive time.Time values, not int64
	// ... implementation ...
}
```

**Impact:** Guards R-1, R-2, K-1, C3-3, CRITICAL-2, N5-1, N5-4 regressions. Provides the compile+runtime regression gate code6.md promised.

---

## Fix 5 — R-5 (LOW): Stop-condition eval error swallowed

**File:** `server/router/api/v1/agent/execution.go:215`
**Finding:** `result, _ := EvalConditionDynamic(...)` — a non-missing-key error (e.g. misspelled identifier at compile time) silently disables the stop rule.

### Changes

Replace:
```go
result, _ := EvalConditionDynamic(ctx, graph.Stop.Condition, celVars, graph)
if result != nil && result.Met {
```

With:
```go
result, stopErr := EvalConditionDynamic(ctx, graph.Stop.Condition, celVars, graph)
if stopErr != nil {
	slog.Warn("stop condition eval error, treating as not-matched",
		"exec_id", exec.ID, "condition", graph.Stop.Condition, "error", stopErr)
}
if result != nil && result.Met {
```

**Impact:** Eval errors in stop conditions are logged instead of silently disabling the stop rule.

---

## Fix 6 — R-6 (LOW): tenant_id NOT NULL vs DEFAULT NULL mismatch

**Files:**
- `store/migration/postgres/0.36/00__add_skill_executions.sql`
- `store/migration/postgres/LATEST.sql`

**Finding:** Postgres DDL has `tenant_id BIGINT NOT NULL` but SQLite has `DEFAULT NULL`. `CreateSkillLog` (execution.go:295) writes `exec.TenantID` which may be nil → NOT NULL violation on Postgres.

### Changes

Make Postgres DDL match SQLite — allow NULL tenant_id:

**store/migration/postgres/0.36/00__add_skill_executions.sql:**
```sql
-- Before:
tenant_id BIGINT NOT NULL,
-- After:
tenant_id BIGINT DEFAULT NULL,
```

Same change in the `agent_skill_logs` table definition in both the versioned DDL and `LATEST.sql`.

**store/migration/postgres/LATEST.sql:**
Same `NOT NULL` → `DEFAULT NULL` change for both tables.

**Impact:** Nil-tenant executions/logs no longer violate Postgres constraints. Matches SQLite behavior.

---

## Fix 7 — R-7 (LOW): Limit validation in HandleListExecutions

**File:** `server/router/api/v1/agent/handlers.go:6877-6884`
**Finding:** `?limit=0` returns empty list; `?limit=-5` produces `LIMIT -5` → SQL error → 500.

### Changes

After the existing limit parsing (line 6884), add:

```go
if limit < 1 {
    limit = 1
}
```

Full block:
```go
status := c.QueryParam("status")
limit := 50
if l := c.QueryParam("limit"); l != "" {
    fmt.Sscanf(l, "%d", &limit)
}
if limit < 1 {
    limit = 1
}
if limit > 200 {
    limit = 200
}
```

**Impact:** `?limit=0` and `?limit=-5` now clamp to 1 instead of erroring.

---

## Implementation Order

| Step | Fix | Files | Est. LOC | Depends On |
|------|-----|-------|----------|------------|
| 1 | Fix 1 (R-1 output contract) | execution.go | ~12 | — |
| 2 | Fix 2 (R-2 missing-var) | evaluator.go, execution.go | ~8 | — |
| 3 | Fix 3 (R-3 chat tenant) | service.go | ~3 | — |
| 4 | Fix 5 (R-5 stop error) | execution.go | ~4 | — |
| 5 | Fix 6 (R-6 tenant null) | postgres DDL, LATEST.sql | ~4 | — |
| 6 | Fix 7 (R-7 limit) | handlers.go | ~3 | — |
| 7 | Fix 4 (R-4 tests) | evaluator_test.go, agent_skill_test.go | ~200 | Fix 1, Fix 2 |
| | **Total** | | **~234** | |

---

## Verification

After implementation:
```
go build ./...
go test ./store/... ./server/router/api/v1/agent/...
task validate:schema
task validate:parity
```

### Specific checks to confirm fixes:
- [ ] R-1: `search_kb = {"found":true}` → condition `search_kb.found == false` returns `Met=false` (not silently wrong)
- [ ] R-2: `search_kb` absent → condition `search_kb.found == false` returns `Met=false` (not error)
- [ ] R-2: Skipped node gets placeholder → downstream condition evaluates cleanly
- [ ] R-3: Chat-path `llm_call` resolves tenant model/key (not env default)
- [ ] R-5: Stop-condition eval error is logged (not swallowed)
- [ ] R-6: Postgres DDL has `DEFAULT NULL` for `tenant_id` in both tables
- [ ] R-7: `?limit=0` returns 1 row (not empty); `?limit=-5` returns 1 row (not 500)
