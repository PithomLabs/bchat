# code8.md — Fix Plan for code7.md Review Gaps

**Source:** code7_review.md (DeepSeek adversarial review)
**Date:** 2026-08-05
**Scope:** 2 corrections (C-1, C-2) + 6 nits (N-1 through N-6) folded into code7.md fixes
**Status:** Ready for implementation

---

## Corrections

### C-1 — Fix 4 sqlite tests: `&DB{}` panics (no connection)

**File:** code7.md section 4b, `store/db/sqlite/agent_skill_test.go`
**Problem:** Every test constructs `db := &DB{}`. The unexported `db *sql.DB` field is nil, so `d.db.ExecContext` panics at runtime.
**Fix:** Use the established pattern from `store/test/store.go:85-107`. Create a real DB via `sqlite.NewDB` over `t.TempDir()`, run migrations, then use the driver interface.

```go
package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
	sqlitedb "github.com/usememos/memos/store/db/sqlite"
	"github.com/usememos/memos/store/migration"
)

func setupTestDB(t *testing.T) store.Driver {
	t.Helper()
	dir := t.TempDir()
	dsn := dir + "/test.db"
	p := &profile.Profile{
		Mode:   "prod",
		Data:   dir,
		DSN:    dsn,
		Driver: "sqlite",
	}
	db, err := sqlitedb.NewDB(p)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	m := migration.New(db.GetDB(), p)
	if err := m.Migrate(ctx); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}
```

Each test then starts with `db := setupTestDB(t)` and uses the driver interface methods. The exact migration call may need adjustment depending on the migration package API — the key point is **never use `&DB{}`**.

---

### C-2 — Fix 6 omits CockroachDB DDLs

**Files:**
- `store/migration/cockroach/0.36/00__add_skill_executions.sql`
- `store/migration/cockroach/LATEST.sql`

**Problem:** CRDB is served by the postgres driver (C3-9). Both cockroach DDLs have `tenant_id INT8 NOT NULL` in both tables. Nil-tenant logs/executions violate NOT NULL, same as postgres. `validate:parity` checks names only, so this ships silently.

**Fix:** Apply `NOT NULL` to `DEFAULT NULL` in both cockroach files:

`store/migration/cockroach/0.36/00__add_skill_executions.sql`:
- `agent_skill_executions.tenant_id`: `INT8 NOT NULL` -> `INT8 DEFAULT NULL`
- `agent_skill_logs.tenant_id`: `INT8 NOT NULL` -> `INT8 DEFAULT NULL`

`store/migration/cockroach/LATEST.sql` (lines 1035, 1063):
- Same changes: `INT8 NOT NULL` -> `INT8 DEFAULT NULL`

---

## Nits

### N-1 — Test name mismatch: rename `RawString_NoFieldAccess`

**File:** code7.md section 4a
**Problem:** Test feeds `map[string]any{"output": "logged"}` (a wrapper map), not a raw Go string. Name and comment do not match.
**Fix:** Rename to `TestEvalConditionDynamic_WrapperMap_NoFieldAccess` and update comment to say "Wrapper map with 'output' key — field access on non-existent key returns nil."

---

### N-2 — Fix 4c postgres tests: provide real bodies

**File:** code7.md section 4c
**Problem:** Test bodies are `// ... implementation ...` placeholders.
**Fix:** Provide real test bodies using `skipUnlessIntegration` gating and `DRIVER=postgres` + `DSN` env var. See code7.md section 4c with the correction applied — connect via `postgres.NewDB(profile)`, create -> claim -> complete/stop, assert TIMESTAMPTZ round-trips.

---

### N-3 — JSON float64 vs int coercion for CEL

**File:** code7.md Fix 1 (output contract)
**Problem:** After Fix 1, JSON-parsed node outputs carry numeric fields as `float64`. CEL has no `int == double` overload. A condition like `kb.count == 5` (int literal) will eval-error.
**Fix:** Normalize integer-like float64 to int after unmarshalling. Add this block inside Fix 1, after the `json.Unmarshal` succeeds and the map type assertion passes:

```go
for k, v := range m {
    if f, ok := v.(float64); ok && f == float64(int(f)) {
        m[k] = int(f)
    }
}
```

---

### N-4 — `buildWorkflowOutput` renders `map[...]` after Fix 1

**File:** `server/router/api/v1/agent/execution.go:359-378`
**Problem:** After Fix 1, `state[name]` is a `map[string]any` (wrapper or parsed JSON). `fmt.Sprintf("%s: %v", name, output)` renders as `search_kb: map[found:true ticket_id:T1]` instead of the handler text.
**Fix:** In `buildWorkflowOutput`, unwrap the wrapper map's `"output"` field for display:

```go
func buildWorkflowOutput(state map[string]any, completed map[string]bool) string {
    var parts []string
    for name, done := range completed {
        if done {
            if output, ok := state[name]; ok {
                // Unwrap wrapper map for display
                display := fmt.Sprintf("%v", output)
                if m, ok := output.(map[string]any); ok {
                    if out, exists := m["output"]; exists {
                        display = fmt.Sprintf("%v", out)
                    }
                }
                parts = append(parts, fmt.Sprintf("%s: %s", name, display))
            }
        }
    }
    if len(parts) == 0 {
        return "workflow completed (no output)"
    }
    result := ""
    for i, p := range parts {
        if i > 0 {
            result += "\n"
        }
        result += p
    }
    return result
}
```

---

### N-5 — Add validation scripts to verification section

**File:** code7.md Verification section
**Problem:** `validate-migrations.sh` and `validate-pg-migrations` are not listed.
**Fix:** Add to the verification commands:

```
go build ./...
go test ./store/... ./server/router/api/v1/agent/...
task validate:schema
task validate:parity
./scripts/validate-migrations.sh
task validate:pg-migrations
```

---

### N-6 — R-8 (EmitEvent on stop) stays deferred

**File:** code7.md (not mentioned)
**Problem:** R-8 (`EmitEvent` on stop) was intentionally dropped from code6.md. It should be explicitly stated as deferred rather than disappearing.
**Fix:** Add a note at the end of code7.md:

```
## Deferred (Non-Blocking)

| Item | Description | Status |
|------|-------------|--------|
| R-8 | EmitEvent dispatch on stop condition match | Deferred per code6.md (not a regression) |
| D2  | Chat path durability | Deferred per code6.md |
| MED-3 | max_retries/retry_count logic | Deferred per code6.md |
| MED-4 | Graceful shutdown for goroutines | Deferred per code6.md |
| LOW-2 | CEL program caching | Deferred per code6.md |
| LOW-4 | Recovery graph JSON dedup | Deferred per code6.md |
```

---

## Implementation Order

| Step | Fix/Nit | Files | Depends On |
|------|---------|-------|------------|
| 1 | Fix 1 (R-1 output contract) | execution.go | - |
| 2 | N-3 (float64 normalization) | execution.go | Fix 1 |
| 3 | N-4 (buildWorkflowOutput unwrap) | execution.go | Fix 1 |
| 4 | Fix 2 (R-2 tolerant eval) | evaluator.go, execution.go | - |
| 5 | Fix 3 (R-3 chat tenant) | service.go | - |
| 6 | Fix 5 (R-5 stop error) | execution.go | - |
| 7 | Fix 6 (R-6 tenant null) + C-2 (cockroach) | postgres DDL, cockroach DDL, LATEST.sql files | - |
| 8 | Fix 7 (R-7 limit) | handlers.go | - |
| 9 | Fix 4a (EvalConditionDynamic tests) | evaluator_test.go | Fix 1, Fix 2 |
| 10 | Fix 4b (sqlite round-trip) + C-1 (setupTestDB) | agent_skill_test.go (new) | - |
| 11 | Fix 4c (postgres integration) + N-2 | agent_skill_test.go (new) | - |
| 12 | N-1 (rename test) | evaluator_test.go | Fix 4a |
| 13 | N-5 (verification scripts) | code7.md / README | - |
| 14 | N-6 (deferred items note) | code7.md | - |

---

## Verification

```
go build ./...
go test ./store/... ./server/router/api/v1/agent/...
task validate:schema
task validate:parity
./scripts/validate-migrations.sh
task validate:pg-migrations
```

### Spot-checks:
- [ ] Fix 1: `search_kb = {"found":true}` -> `search_kb.found == false` returns `Met=false` (not silently wrong)
- [ ] Fix 1: `search_kb = "logged"` -> `search_kb.found == false` returns `Met=false`
- [ ] Fix 1: `kb.count == 5` works after float64 normalization (N-3)
- [ ] Fix 1: `buildWorkflowOutput` shows handler text, not `map[...]` (N-4)
- [ ] Fix 2: absent var -> `search_kb.found == false` returns `Met=false` (not error)
- [ ] Fix 2: skipped node gets placeholder -> downstream condition evaluates cleanly
- [ ] Fix 3: chat-path `llm_call` resolves tenant model/key
- [ ] Fix 4b: sqlite tests pass with real DB setup (C-1)
- [ ] Fix 4c: postgres tests pass with `SKILL_PG_INTEGRATION_TEST=true` + `DSN`
- [ ] Fix 5: stop-condition eval error is logged (not swallowed)
- [ ] Fix 6: postgres + cockroach DDLs have `DEFAULT NULL` for `tenant_id`
- [ ] Fix 7: `?limit=0` returns 1 row; `?limit=-5` returns 1 row
