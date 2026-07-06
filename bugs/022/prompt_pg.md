# Coding Agent Prompt: Fix Postgres/MySQL Memo Relation Tenant Isolation + dispatchMemoMentions

### Context

The bugs/022 security fixes added `TenantID` to `memo_relation` operations in SQLite but left Postgres and MySQL drivers unchanged. This creates a tenant isolation breach on those database backends. Additionally, `dispatchMemoMentions` in `memo_service.go` omits `TenantID` on a `ListMemoRelations` call.

### Tasks

#### 1. Fix `store/db/postgres/memo_relation.go`

Read the file, then update three functions:

**`UpsertMemoRelation`**: Add `tenant_id` to the INSERT column list and VALUES. Add `ON CONFLICT (memo_id, related_memo_id, type) DO UPDATE SET tenant_id = EXCLUDED.tenant_id` (or the Postgres equivalent).

**`ListMemoRelations`**: Add `AND mr.tenant_id = $N` to the WHERE clause when `find.TenantID != nil`. Increment the parameter counter accordingly.

**`DeleteMemoRelation`**: Add `AND tenant_id = $N` to the WHERE clause when `delete.TenantID != nil`.

#### 2. Fix `store/db/mysql/memo_relation.go`

Same three functions as Postgres, using MySQL syntax (`?` placeholders, `ON DUPLICATE KEY UPDATE` instead of `ON CONFLICT`).

#### 3. Fix `server/router/api/v1/memo_service.go`

In `dispatchMemoMentions` (around line 854), add `TenantID: memo.TenantID` to the `FindMemoRelation` struct passed to `ListMemoRelations`.

### Reference: SQLite Implementation

The SQLite implementations in `store/db/sqlite/memo_relation.go` are the reference. Copy the pattern:
- `TenantID *int32` field usage in conditionals
- Parameterized query placeholders
- `ON CONFLICT` / `ON DUPLICATE KEY UPDATE` clauses

### Verification

After making changes, run:
```bash
go build ./...
go test ./store/... ./server/... -count=1
```

### Files to Modify

- `store/db/postgres/memo_relation.go`
- `store/db/mysql/memo_relation.go`
- `server/router/api/v1/memo_service.go`
