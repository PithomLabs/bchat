# Plan v2: SQLite → PostgreSQL Parity Fixes

**Date:** 2026-07-17
**Goal:** Ensure full functionality of bchat on PostgreSQL (Neon DB / Fly.io) without regression
**Changes from v1:** Incorporates adversarial review findings (see `plan_review.md`)

---

## Analysis Summary

### Schema LATEST.sql: Fully Ported ✅

All **56 tables** are present in both SQLite and PostgreSQL `LATEST.sql` files. All indexes and constraints match. Differences are expected platform idioms (INTEGER→BOOLEAN, BLOB→BYTEA, TEXT→JSONB, TIMESTAMP→TIMESTAMPTZ).

### Driver Code: 7 Bugs Found 🐛

| # | Severity | File | Bug |
|---|----------|------|-----|
| 1 | **HIGH** | `store/db/postgres/bridge_auth.go:19-33` | `CreateBridgeAuthKey` passes `time.Time` to `BIGINT` AND uses `LastInsertId()` — two bugs in one |
| 2 | **MEDIUM** | `store/db/postgres/agent.go:1735` | `GetOrCreateAgentLearningMemory` uses `LastInsertId()` without `RETURNING id` |
| 3 | **MEDIUM** | `store/db/postgres/agent.go:1936` | `GetOrCreateAgentScoringConfig` — same `LastInsertId()` issue |
| 4 | **MEDIUM** | `store/db/postgres/agent.go:1979` | `CreateAgentQAPair` — same `LastInsertId()` issue |
| 5 | **LOW** | `store/db/postgres/agent.go:2501` | `UpsertReindexCheckpoint` — `LastInsertId()` unreliable on upsert conflict path |
| 6 | **LOW** | `store/driver.go` | `CreateAgentWorkflow`, `ListAgentWorkflows`, `GetAgentWorkflow` not in `Driver` interface |
| 7 | **LOW** | `store/migration/postgres/LATEST.sql:186` | `tenant_role_templates.tenant_id` missing CHECK constraint vs SQLite |

---

## Fix Details

### Fix 1: CreateBridgeAuthKey — time.Time + LastInsertId (HIGH)

**File:** `store/db/postgres/bridge_auth.go`

**Two bugs in one function:**
1. Line 23: `key.CreatedAt` / `key.UpdatedAt` passed as `time.Time` to `BIGINT` columns → runtime crash
2. Line 28: `result.LastInsertId()` unreliable on pgx → silent failure

**Current (lines 19-33):**
```go
result, err := d.db.ExecContext(ctx, `
    INSERT INTO bridge_auth_keys (
        tenant_id, key_id, label, secret_key_encrypted, secret_key_nonce, status, created_at, updated_at, last_used_at, revoked_at
    ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULL, NULL)
`, key.TenantID, key.KeyID, labelVal, key.SecretKeyEncrypted, key.SecretKeyNonce, key.Status, key.CreatedAt, key.UpdatedAt)
if err != nil {
    return nil, fmt.Errorf("create bridge auth key: %w", err)
}

id, err := result.LastInsertId()
if err != nil {
    return nil, fmt.Errorf("get bridge auth key insert id: %w", err)
}
key.ID = id
return key, nil
```

**Change to:**
```go
var id int64
err = d.db.QueryRowContext(ctx, `
    INSERT INTO bridge_auth_keys (
        tenant_id, key_id, label, secret_key_encrypted, secret_key_nonce, status, created_at, updated_at, last_used_at, revoked_at
    ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULL, NULL)
    RETURNING id
`, key.TenantID, key.KeyID, labelVal, key.SecretKeyEncrypted, key.SecretKeyNonce, key.Status, key.CreatedAt.Unix(), key.UpdatedAt.Unix()).Scan(&id)
if err != nil {
    return nil, fmt.Errorf("create bridge auth key: %w", err)
}
key.ID = id
return key, nil
```

**Why:**
- `.Unix()` converts `time.Time` to `int64` for `BIGINT` columns (matches SQLite behavior)
- `RETURNING id` + `QueryRowContext` is the idiomatic pgx pattern (same as 40+ other methods in this driver)
- Removes unused `result` variable

---

### Fix 2: GetOrCreateAgentLearningMemory — RETURNING id (MEDIUM)

**File:** `store/db/postgres/agent.go` (lines ~1728-1739)

**Current:**
```go
stmt := `
    INSERT INTO agent_learning_memory (
        tenant_id, common_issues, learned_behaviors, improvement_areas,
        pending_suggestions, analysis_count, last_updated, version
    ) VALUES ($1, '[]', '[]', '[]', '[]', 0, $2, 1)
`
now := time.Now()
result, err := d.db.ExecContext(ctx, stmt, tenantID, now)
if err != nil {
    return nil, err
}
id, _ := result.LastInsertId()
```

**Change to:**
```go
stmt := `
    INSERT INTO agent_learning_memory (
        tenant_id, common_issues, learned_behaviors, improvement_areas,
        pending_suggestions, analysis_count, last_updated, version
    ) VALUES ($1, '[]', '[]', '[]', '[]', 0, $2, 1)
    RETURNING id
`
now := time.Now()
var id int64
err := d.db.QueryRowContext(ctx, stmt, tenantID, now).Scan(&id)
if err != nil {
    return nil, err
}
```

**Why:** `database/sql.Result.LastInsertId()` is not reliably supported by pgx via the `database/sql` shim. Using `RETURNING id` with `QueryRowContext` is the idiomatic PostgreSQL pattern and is what the workflow code already does correctly.

---

### Fix 3: GetOrCreateAgentScoringConfig — RETURNING id (MEDIUM)

**File:** `store/db/postgres/agent.go` (lines ~1932-1941)

**Current:**
```go
stmt := `
    INSERT INTO agent_scoring_config (tenant_id, version, config, created_at, updated_at)
    VALUES ($1, $2, $3, $4, $5)
`
result, err := d.db.ExecContext(ctx, stmt, tenantID, "1.0", defaultConfig, now, now)
if err != nil {
    return nil, err
}
id, _ := result.LastInsertId()
```

**Change to:**
```go
stmt := `
    INSERT INTO agent_scoring_config (tenant_id, version, config, created_at, updated_at)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING id
`
var id int64
err := d.db.QueryRowContext(ctx, stmt, tenantID, "1.0", defaultConfig, now, now).Scan(&id)
if err != nil {
    return nil, err
}
```

---

### Fix 4: CreateAgentQAPair — RETURNING id (MEDIUM)

**File:** `store/db/postgres/agent.go` (lines ~1973-1990)

**Current:**
```go
stmt := `
    INSERT INTO agent_qa_pairs (
        tenant_id, question, expected_answer, source_section, source_chunk_id,
        difficulty, category, is_active, created_at, updated_at
    ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
`
result, err := d.db.ExecContext(ctx, stmt,
    pair.TenantID, pair.Question, pair.ExpectedAnswer, pair.SourceSection, pair.SourceChunkID,
    pair.Difficulty, pair.Category, pair.IsActive, now, now,
)
if err != nil {
    return nil, err
}
id, err := result.LastInsertId()
if err != nil {
    return nil, err
}
pair.ID = int32(id)
```

**Change to:**
```go
stmt := `
    INSERT INTO agent_qa_pairs (
        tenant_id, question, expected_answer, source_section, source_chunk_id,
        difficulty, category, is_active, created_at, updated_at
    ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    RETURNING id
`
var id int64
err := d.db.QueryRowContext(ctx, stmt,
    pair.TenantID, pair.Question, pair.ExpectedAnswer, pair.SourceSection, pair.SourceChunkID,
    pair.Difficulty, pair.Category, pair.IsActive, now, now,
).Scan(&id)
if err != nil {
    return nil, err
}
pair.ID = int32(id)
```

---

### Fix 5: UpsertReindexCheckpoint — RETURNING id (LOW)

**File:** `store/db/postgres/agent.go` (lines ~2481-2517)

**Behavioral change:** The current code only fetches the ID for new inserts (`if checkpoint.ID == 0`). The new code always populates `checkpoint.ID` via `RETURNING id`, which is an improvement — it guarantees the ID is always correct, whether the path was INSERT or UPDATE (conflict).

**Current:**
```go
stmt := `
    INSERT INTO agent_reindex_checkpoints (...)
    VALUES (...)
    ON CONFLICT(tenant_id, audience, file_type, version) DO UPDATE SET ...
`
result, err := d.db.ExecContext(ctx, stmt, ...)
if err != nil {
    return nil, fmt.Errorf("failed to upsert reindex checkpoint: %w", err)
}
if checkpoint.ID == 0 {
    id, err := result.LastInsertId()
    if err == nil {
        checkpoint.ID = int32(id)
    }
}
```

**Change to:**
```go
stmt := `
    INSERT INTO agent_reindex_checkpoints (...)
    VALUES (...)
    ON CONFLICT(tenant_id, audience, file_type, version) DO UPDATE SET ...
    RETURNING id
`
err := d.db.QueryRowContext(ctx, stmt, ...).Scan(&checkpoint.ID)
if err != nil {
    return nil, fmt.Errorf("failed to upsert reindex checkpoint: %w", err)
}
```

**Why:** `RETURNING id` works with both the INSERT and UPDATE (conflict) paths in PostgreSQL, reliably returning the ID in both cases. The error wrapping remains accurate since a `Scan` failure from `RETURNING` effectively means the upsert failed.

---

### Fix 6: Add Workflow Methods to Driver Interface

**File:** `store/driver.go` (insert after line 262, before the Observation Log section)

Add:
```go
// Agent Workflow model related methods.
CreateAgentWorkflow(ctx context.Context, create *CreateAgentWorkflow) (*AgentWorkflow, error)
ListAgentWorkflows(ctx context.Context, find *FindAgentWorkflow) ([]*AgentWorkflow, error)
GetAgentWorkflow(ctx context.Context, find *FindAgentWorkflow) (*AgentWorkflow, error)
```

**File:** `store/agent_workflow.go` (lines 48-66)

Remove TODO comment (line 48) and uncomment the Store wrapper methods:
```go
func (s *Store) CreateAgentWorkflow(ctx context.Context, create *CreateAgentWorkflow) (*AgentWorkflow, error) {
    return s.driver.CreateAgentWorkflow(ctx, create)
}

func (s *Store) ListAgentWorkflows(ctx context.Context, find *FindAgentWorkflow) ([]*AgentWorkflow, error) {
    return s.driver.ListAgentWorkflows(ctx, find)
}

func (s *Store) GetAgentWorkflow(ctx context.Context, find *FindAgentWorkflow) (*AgentWorkflow, error) {
    return s.driver.GetAgentWorkflow(ctx, find)
}
```

**Note:** MySQL stubs (`store/db/mysql/agent_workflow.go`) already exist and return `nil, nil`. Adding these to the Driver interface won't break compilation, but if anyone switches to MySQL, workflow calls will silently return nil. Consider returning an explicit "not implemented" error in the future.

---

### Fix 7: Add CHECK Constraint — Migration + LATEST.sql

**Two files to modify (not just LATEST.sql):**

#### 7a. Migration file for existing databases

**New file:** `store/migration/postgres/0.30/04__add_tenant_id_check_to_role_templates.sql`

```sql
ALTER TABLE tenant_role_templates
  ADD CONSTRAINT chk_tenant_role_templates_tenant_id
  CHECK (tenant_id IS NULL OR tenant_id >= 1);
```

**Why:** Modifying only `LATEST.sql` affects fresh installs. Existing PostgreSQL databases (e.g., Neon on Fly.io, the stated deployment target) will not receive the CHECK constraint without a migration. This migration adds it to existing databases.

#### 7b. LATEST.sql update

**File:** `store/migration/postgres/LATEST.sql` (line 186)

**Current:**
```sql
tenant_id INTEGER REFERENCES agent_tenants(id) ON DELETE CASCADE,
```

**Change to:**
```sql
tenant_id INTEGER CHECK (tenant_id IS NULL OR tenant_id >= 1) REFERENCES agent_tenants(id) ON DELETE CASCADE,
```

**Why:** Ensures fresh installs get the CHECK constraint from the start.

---

## Files to Modify

| # | File | Change Type |
|---|------|-------------|
| 1 | `store/db/postgres/bridge_auth.go` | Bug fix: `.Unix()` + `RETURNING id` |
| 2 | `store/db/postgres/agent.go` | Bug fix: 4 `LastInsertId` → `RETURNING id` |
| 3 | `store/driver.go` | Interface: add 3 workflow methods |
| 4 | `store/agent_workflow.go` | Uncomment wrapper methods, remove TODO |
| 5 | `store/migration/postgres/0.30/04__add_tenant_id_check_to_role_templates.sql` | New migration file |
| 6 | `store/migration/postgres/LATEST.sql` | Schema parity: add CHECK constraint |

---

## Verification Steps

1. `go build ./...` — verify compilation
2. `go vet ./...` — static analysis
3. Run existing tests if available
4. Manual smoke test against local PostgreSQL (if available)
