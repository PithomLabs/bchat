# Bug 047: SQLite Driver Missing 14 RBAC Methods — Implementation Plan

**Status:** Ready to implement (awaiting go-signal)  
**Created:** 2026-07-23  
**Scope:** Fix `go test ./...` / `go build ./...` failures caused by incomplete SQLite driver  

---

## Executive Summary

The `store.Driver` interface defines **200 methods**. The SQLite driver implements **186**. The 14 missing methods are all RBAC-related (user-tenant permissions, role templates, system secrets). This causes **every package that transitively depends on `store/db/sqlite`** to fail compilation, blocking the entire test suite.

---

## Root Cause Analysis

| Driver | Interface Compliance | Missing Methods |
|--------|---------------------|-----------------|
| PostgreSQL | ✅ 200/200 | 0 |
| MySQL | ✅ 200/200 | 0 |
| **SQLite** | ❌ **186/200** | **14** |

The 14 missing methods were added to `store/driver.go` (the RBAC layer) after the SQLite driver was last updated. The Postgres and MySQL drivers were updated; SQLite was not.

### Missing Methods

**User-Tenant Permissions (7 methods):**
```
CreateUserTenantPermission
GetUserTenantPermission
ListUserTenantPermissions
UpdateUserTenantPermission
DeleteUserTenantPermission
DeleteAllUserTenantPermissions
DeleteExplicitUserTenantPermissions
```

**Tenant Role Templates (5 methods):**
```
CreateTenantRoleTemplate
GetTenantRoleTemplate
ListTenantRoleTemplates
UpdateTenantRoleTemplate
DeleteTenantRoleTemplate
```

**System Secrets (2 methods):**
```
GetSystemSecret
UpsertSystemSecret
```

### Impact Chain

```
store/driver.go (interface)
  → store/db/sqlite/sqlite.go:49 (NewDB returns *DB as Driver)
    → Compilation fails: *DB does not implement Driver
      → ALL downstream packages fail:
         - store/db/sqlite
         - store/test
         - store
         - server/router/api/v1
         - server/router/api/v1/agent
         - internal/bridgeworker
         - bin/memos
```

---

## Table Schemas (Already Exist in SQLite)

### `user_tenant_permission`
```sql
CREATE TABLE user_tenant_permission (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    tenant_id INTEGER NOT NULL,
    permissions TEXT NOT NULL DEFAULT '',
    granted_by INTEGER DEFAULT NULL,
    granted_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    source_template_id INTEGER REFERENCES tenant_role_templates(id) ON DELETE SET NULL,
    UNIQUE(user_id, tenant_id)
);
CREATE INDEX idx_user_tenant_permission_user ON user_tenant_permission(user_id);
CREATE INDEX idx_user_tenant_permission_tenant ON user_tenant_permission(tenant_id);
CREATE INDEX idx_user_tenant_permission_template ON user_tenant_permission(source_template_id);
```

### `tenant_role_templates`
```sql
CREATE TABLE tenant_role_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER DEFAULT NULL,
    name TEXT NOT NULL,
    code TEXT NOT NULL,
    permissions TEXT NOT NULL DEFAULT '[]',
    created_by INTEGER DEFAULT NULL,
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now'))
);
CREATE INDEX idx_tenant_role_templates_tenant ON tenant_role_templates(tenant_id);
CREATE INDEX idx_tenant_role_templates_code ON tenant_role_templates(code);
```

### `system_secret`
```sql
CREATE TABLE system_secret (
    id INTEGER PRIMARY KEY DEFAULT 1,
    encryption_salt BLOB NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    rotated_at BIGINT DEFAULT NULL
);
```

---

## Store Types (Reference)

```go
// store/rbac.go
type UserTenantPermission struct {
    ID              int32
    UserID          int32
    TenantID        int32
    Permissions     []string  // Comma-separated in DB, split on read
    GrantedBy       *int32
    GrantedAt       time.Time
    SourceTemplateID *int32
}

type FindUserTenantPermission struct {
    ID               *int32
    UserID           *int32
    TenantID         *int32
    SourceTemplateID *int32
}

// store/role_template.go
type TenantRoleTemplate struct {
    ID          int32
    TenantID    *int32
    Name        string
    Code        string
    Permissions []string
    CreatedBy   *int32
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type FindTenantRoleTemplate struct {
    ID       *int32
    TenantID *int32
    Code     *string
    Name     *string
}

// store/rbac.go
type SystemSecret struct {
    ID             int32
    EncryptionSalt []byte
    KeyVersion     int
    CreatedAt      time.Time
    RotatedAt      *time.Time
}
```

---

## Implementation Plan

### File to Create

**`store/db/sqlite/rbac.go`** — Single file, ~200 lines.

Port all 14 methods from `store/db/postgres/rbac.go`, applying these Postgres → SQLite translations:

| Postgres Syntax | SQLite Equivalent |
|----------------|-------------------|
| `$1, $2, $3` | `?, ?, ?` |
| `RETURNING id` | `result.LastInsertId()` or `RETURNING id` (SQLite 3.35+) |
| `::jsonb` cast | Plain text (SQLite stores JSON as text) |
| `NOW()` / `now()` | `strftime('%s', 'now')` or `time.Now().Unix()` in Go |
| `ON CONFLICT ... DO UPDATE ... RETURNING id` | Split into `Exec` + `LastInsertId` |

### Method-by-Method Port

#### 1. User-Tenant Permissions

**`CreateUserTenantPermission`**
- Postgres: `INSERT ... VALUES($1,$2,$3,$4,$5,$6) RETURNING id`
- SQLite: `INSERT ... VALUES(?, ?, ?, ?, ?, ?)` + `LastInsertId()`
- Set `perm.GrantedAt = time.Now()`

**`GetUserTenantPermission`**
- Delegate to `ListUserTenantPermissions` + return first result (same pattern as Postgres)

**`ListUserTenantPermissions`**
- Postgres: `WHERE id=$N, user_id=$N, tenant_id=$N, source_template_id=$N`
- SQLite: Same logic with `?` placeholders
- Parse `permissions` from comma-separated string: `strings.Split(permissions, ",")`
- Handle `sql.NullInt32` for `granted_by` and `source_template_id`

**`UpdateUserTenantPermission`**
- Postgres: `UPDATE ... SET permissions=$1, granted_by=$2, granted_at=$3, source_template_id=$4 WHERE id=$5`
- SQLite: Same with `?` placeholders

**`DeleteUserTenantPermission`**
- Postgres: `DELETE FROM ... WHERE user_id=$1 AND tenant_id=$2 AND id=$3`
- SQLite: Same with `?` placeholders

**`DeleteAllUserTenantPermissions`**
- Postgres: `DELETE FROM ... WHERE user_id=$1 AND tenant_id=$2`
- SQLite: Same with `?` placeholders

**`DeleteExplicitUserTenantPermissions`**
- Postgres: `DELETE FROM ... WHERE user_id=$1 AND tenant_id=$2 AND source_template_id IS NULL`
- SQLite: Same with `?` placeholders

#### 2. Tenant Role Templates

**`CreateTenantRoleTemplate`**
- Postgres: `INSERT ... VALUES($1,$2,$3,$4,$5) RETURNING id, created_at, updated_at`
- SQLite: `INSERT ... VALUES(?, ?, ?, ?, ?)` + `LastInsertId()`
- `permissions`: `json.Marshal(template.Permissions)` → store as text
- Handle nullable `tenant_id` and `created_by` with `nil`/interface{}

**`GetTenantRoleTemplate`**
- Postgres: Complex WHERE with nullable tenant_id (-1 = NULL check)
- SQLite: Same logic with `?` placeholders
- `sql.NullInt32` for `tenant_id` and `created_by`
- `json.Unmarshal` on `permissions` column

**`ListTenantRoleTemplates`**
- Postgres: Same nullable tenant_id logic, ORDER BY code ASC
- SQLite: Same pattern

**`UpdateTenantRoleTemplate`**
- Postgres: Fetch existing, merge changes, UPDATE
- SQLite: Same pattern — fetch, merge, update with `?`

**`DeleteTenantRoleTemplate`**
- Postgres: Check for active assignments first, then DELETE
- SQLite: Same guard logic

#### 3. System Secrets

**`GetSystemSecret`**
- Postgres: `SELECT id, encryption_salt, key_version, created_at, rotated_at FROM system_secret WHERE id = 1`
- SQLite: Same query (both use `id = 1` singleton pattern)
- Handle `sql.ErrNoRows` → return `nil, nil`
- `rotated_at`: `sql.NullInt64` → parse Unix timestamp

**`UpsertSystemSecret`**
- Postgres: `INSERT ... ON CONFLICT(id) DO UPDATE SET ... RETURNING id`
- SQLite: `INSERT ... ON CONFLICT(id) DO UPDATE SET ...` (no RETURNING needed; use `Exec`)
- Set `created_at` and `rotated_at` as Unix timestamps

---

## Translation Reference: Postgres → SQLite

```go
// Postgres pattern:
row := d.db.QueryRowContext(ctx, `
    INSERT INTO table(col1, col2) VALUES($1, $2) RETURNING id
`, val1, val2)
row.Scan(&id)

// SQLite equivalent (Option A — use LastInsertId):
result, err := d.db.ExecContext(ctx, `
    INSERT INTO table(col1, col2) VALUES(?, ?)
`, val1, val2)
id, _ := result.LastInsertId()

// SQLite equivalent (Option B — RETURNING also works in SQLite 3.35+):
row := d.db.QueryRowContext(ctx, `
    INSERT INTO table(col1, col2) VALUES(?, ?) RETURNING id
`, val1, val2)
row.Scan(&id)
```

Both options work. Use `RETURNING` where it simplifies multi-column return (e.g., `CreateTenantRoleTemplate` returns `id, created_at, updated_at`). Use `LastInsertId()` for simple single-column returns.

---

## Files to Modify

| File | Action | Lines |
|------|--------|-------|
| `store/db/sqlite/rbac.go` | **CREATE** | ~200 |
| `store/db/sqlite/sqlite.go` | None (auto-implements) | 0 |
| `store/driver.go` | None (interface already complete) | 0 |

No other files need modification.

---

## Verification Steps

### 1. Compilation
```bash
go build ./...
```
Expected: Clean build (no errors).

### 2. Unit Tests
```bash
go test ./store/test/ -v -run TestRBAC
go test ./store/test/ -v -run TestTenantConfig
go test ./store/test/ -v -run TestSystemSecret
```

### 3. Full Test Suite
```bash
go test ./... -count=1
```
Expected: All packages pass (or only pre-existing failures unrelated to this fix).

### 4. Specific Package Tests
```bash
go test ./server/router/api/v1/agent/ -v -count=1
go test ./server/router/api/v1/ -v -count=1
```

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| SQLite dialect mismatch | Low | High | Postgres version is reference; port is mechanical |
| Timestamp handling difference | Low | Medium | Both use Unix epoch integers; same `time.Unix()` parsing |
| `sql.ErrNoRows` behavior | None | — | Both drivers return same error |
| Missing test coverage | Medium | Low | Existing `store/test/` exercises RBAC paths |
| `ON CONFLICT` syntax differences | Low | Medium | SQLite supports `ON CONFLICT` since 3.24+; use same pattern |

---

## Definition of Done

- [ ] `store/db/sqlite/rbac.go` created with all 14 methods
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes (no pre-existing issues)
- [ ] `go test ./...` passes (or only pre-existing failures)
- [ ] All 3 drivers (sqlite, postgres, mysql) implement 200/200 interface methods
