# Plan: SQLite → PostgreSQL Parity Fixes

**Date:** 2026-07-17
**Goal:** Ensure full functionality of bchat on PostgreSQL (Neon DB / Fly.io) without regression

---

## Analysis Summary

### Schema LATEST.sql: Fully Ported ✅

All **56 tables** are present in both SQLite and PostgreSQL `LATEST.sql` files. All indexes and constraints match. Differences are expected platform idioms (INTEGER→BOOLEAN, BLOB→BYTEA, TEXT→JSONB, TIMESTAMP→TIMESTAMPTZ).

### Driver Code: 7 Bugs Found 🐛

| # | Severity | File | Bug |
|---|----------|------|-----|
| 1 | **HIGH** | `store/db/postgres/bridge_auth.go:23` | `CreateBridgeAuthKey` passes `time.Time` to `BIGINT` columns — runtime crash |
| 2 | **MEDIUM** | `store/db/postgres/agent.go:1735` | `GetOrCreateAgentLearningMemory` uses `LastInsertId()` without `RETURNING id` |
| 3 | **MEDIUM** | `store/db/postgres/agent.go:1936` | `GetOrCreateAgentScoringConfig` — same `LastInsertId()` issue |
| 4 | **MEDIUM** | `store/db/postgres/agent.go:1979` | `CreateAgentQAPair` — same `LastInsertId()` issue |
| 5 | **LOW** | `store/db/postgres/agent.go:2501` | `UpsertReindexCheckpoint` — `LastInsertId()` unreliable on upsert conflict path |
| 6 | **LOW** | `store/driver.go` | `CreateAgentWorkflow`, `ListAgentWorkflows`, `GetAgentWorkflow` not in `Driver` interface |
| 7 | **LOW** | `store/migration/postgres/LATEST.sql:186` | `tenant_role_templates.tenant_id` missing CHECK constraint vs SQLite |

---

## Fix Details

### Fix 1: CreateBridgeAuthKey — time.Time → .Unix() (HIGH)

**File:** `store/db/postgres/bridge_auth.go`

**Current (line 23):**
```go
`, key.TenantID, key.KeyID, labelVal, key.SecretKeyEncrypted, key.SecretKeyNonce, key.Status, key.CreatedAt, key.UpdatedAt)
```

**Change to:**
```go
`, key.TenantID, key.KeyID, labelVal, key.SecretKeyEncrypted, key.SecretKeyNonce, key.Status, key.CreatedAt.Unix(), key.UpdatedAt.Unix())
```

**Why:** The `bridge_auth_keys` table has `created_at BIGINT NOT NULL` and `updated_at BIGINT NOT NULL`. Passing `time.Time` to pgx will attempt to encode it as a PostgreSQL timestamp type, causing a type mismatch error. The SQLite driver correctly uses `.Unix()`.

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

**Why:** `RETURNING id` works with both the INSERT and UPDATE (conflict) paths in PostgreSQL, reliably returning the ID in both cases.

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

**File:** `store/agent_workflow.go` (lines 51-66)

Uncomment the Store wrapper methods:
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

Also remove the TODO comment on line 48.

---

### Fix 7: Add CHECK Constraint to PG LATEST.sql

**File:** `store/migration/postgres/LATEST.sql` (line 186)

**Current:**
```sql
tenant_id INTEGER REFERENCES agent_tenants(id) ON DELETE CASCADE,
```

**Change to:**
```sql
tenant_id INTEGER CHECK (tenant_id IS NULL OR tenant_id >= 1) REFERENCES agent_tenants(id) ON DELETE CASCADE,
```

**Why:** Parity with SQLite. Provides defense-in-depth — if referential integrity is ever deferred or bypassed, this prevents `tenant_id = 0` from being inserted.

---

## Files to Modify

| # | File | Change Type |
|---|------|-------------|
| 1 | `store/db/postgres/bridge_auth.go` | Bug fix: `.Unix()` |
| 2 | `store/db/postgres/agent.go` | Bug fix: 4 `LastInsertId` → `RETURNING id` |
| 3 | `store/driver.go` | Interface: add 3 workflow methods |
| 4 | `store/agent_workflow.go` | Uncomment wrapper methods, remove TODO |
| 5 | `store/migration/postgres/LATEST.sql` | Schema parity: add CHECK constraint |

---

## Verification Steps

1. `go build ./...` — verify compilation
2. `go vet ./...` — static analysis
3. Run existing tests if available
4. Manual smoke test against local PostgreSQL (if available)
