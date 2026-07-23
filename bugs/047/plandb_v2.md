# Bug 047: SQLite Driver Missing 14 RBAC Methods — Implementation Plan v2

**Status:** Ready to implement (awaiting go-signal)  
**Created:** 2026-07-23  
**Updated:** 2026-07-23 (incorporating plandb_review.md findings)  
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

### File to Modify

**`store/db/sqlite/rbac.go`** — **EXTEND** existing file (currently 163 lines, ~395 lines of new code added, total ~560 lines).

> **Note (M1):** The file already exists with 3 methods (`GetTenantConfig`, `UpsertTenantConfig`, `DeleteTenantConfig`). The 14 new methods are appended after the existing ones.

Port all 14 methods from `store/db/postgres/rbac.go`, applying these Postgres → SQLite translations:

| Postgres Syntax | SQLite Equivalent |
|----------------|-------------------|
| `$1, $2, $3` | `?, ?, ?` |
| `RETURNING id` | `RETURNING id` (SQLite 3.35+; `modernc.org/sqlite` targets 3.39+) |
| `::jsonb` cast | Plain text (SQLite stores JSON as text) |
| `NOW()` / `now()` | `time.Now().Unix()` in Go (consistent with existing `UpsertTenantConfig` pattern) |
| `ON CONFLICT ... DO UPDATE ... RETURNING id` | `ON CONFLICT ... DO UPDATE SET ... RETURNING id` (same syntax) |
| `[]interface{}` | `[]any{}` (use Go 1.21+ convention, consistent with newer SQLite code in `agent.go`) |

> **Note (N4):** All RBAC timestamps use `now.Unix()` (int64), not raw `time.Time`. This is the convention used by both Postgres reference and existing `UpsertTenantConfig` (line 112).

### Critical Patterns (Must Follow)

#### Pattern 1: `-1` Sentinel for Global Role Templates (M3)

`GetTenantRoleTemplate` and `ListTenantRoleTemplates` use a **sentinel value `-1`** to represent "global/system-level template" (where `tenant_id IS NULL`). This is NOT a typo.

**Write side (building WHERE clause):**
```go
if find.TenantID != nil {
    if *find.TenantID == -1 {
        where = append(where, "tenant_id IS NULL")  // sentinel!
    } else {
        args = append(args, *find.TenantID)
        where = append(where, "tenant_id = ?")
    }
} else {
    where = append(where, "tenant_id IS NULL")
}
```

**Read side (scanning results):**
```go
var tenantID sql.NullInt32
// ... scan tenantID ...
if tenantID.Valid && tenantID.Int32 != -1 {
    tid := tenantID.Int32
    template.TenantID = &tid
}
// If tenantID.Valid && tenantID.Int32 == -1, leave template.TenantID as nil
// If !tenantID.Valid, leave template.TenantID as nil
```

This pattern prevents exposing the `-1` sentinel to callers. The Postgres reference (lines 297-308, 337-340, 401-404) all use this exact logic.

#### Pattern 2: `SourceTemplateID == 0` → IS NULL (M4)

`ListUserTenantPermissions` has a special case: when `find.SourceTemplateID` is non-nil and equals `0`, the query uses `source_template_id IS NULL` instead of `source_template_id = ?`. This filters for **explicit** permissions (not linked to a template).

```go
if find.SourceTemplateID != nil {
    if *find.SourceTemplateID == 0 {
        where = append(where, "source_template_id IS NULL")
    } else {
        args = append(args, *find.SourceTemplateID)
        where = append(where, "source_template_id = ?")
    }
}
```

Without this, `FindUserTenantPermission{SourceTemplateID: intPtr(0)}` would return 0 results instead of all explicit (non-template) permissions.

#### Pattern 3: Permissions Serialization (N2)

Two different serialization strategies are used:

| Table | Column | Serialization | Read | Write |
|-------|--------|---------------|------|-------|
| `user_tenant_permission` | `permissions` | Comma-separated string | `strings.Split(permissions, ",")` | `strings.Join(perms, ",")` |
| `tenant_role_templates` | `permissions` | JSON array | `json.Unmarshal(permissions, &perms)` | `json.Marshal(perms)` |

#### Pattern 4: Empty Permissions → Non-Nil Slice (N3)

When the DB value is empty, return `[]string{}` (non-nil empty slice), not `nil`. This ensures callers can safely range over the result.

```go
// user_tenant_permission: empty string → non-nil empty slice
if permissions == "" {
    perm.Permissions = []string{}
} else {
    perm.Permissions = strings.Split(permissions, ",")
}

// tenant_role_templates: empty/missing JSON → non-nil empty slice
if len(permissionsJSON) > 0 {
    json.Unmarshal(permissionsJSON, &template.Permissions)
} else {
    template.Permissions = []string{}
}
```

#### Pattern 5: UpdateTenantRoleTemplate Merge Logic (N5)

`UpdateTenantRoleTemplate` uses non-empty-string guards. This means **Name and Code cannot be cleared** via Update — the merge always keeps the old value. This is by design (matches Postgres reference, lines 430-438):

```go
if template.Name != "" { existing.Name = template.Name }
if template.Code != "" { existing.Code = template.Code }
if template.Permissions != nil { existing.Permissions = template.Permissions }
```

---

### Method-by-Method Port

#### 1. User-Tenant Permissions

**`CreateUserTenantPermission`**
- Postgres: `INSERT ... VALUES($1,$2,$3,$4,$5,$6) RETURNING id`
- SQLite: `INSERT ... VALUES(?, ?, ?, ?, ?, ?) RETURNING id` + `.Scan(&perm.ID)`
- Use `now := time.Now()` and `perm.GrantedAt = now` (not `now.Unix()` for the struct field — the struct stores `time.Time`, the DB stores `int64`)

**`GetUserTenantPermission`**
- Delegate to `ListUserTenantPermissions` + return first result (same pattern as Postgres, line 28)

**`ListUserTenantPermissions`**
- Postgres: `WHERE id=$N, user_id=$N, tenant_id=$N, source_template_id=$N`
- SQLite: Same logic with `?` placeholders
- **Critical (M4):** Apply `SourceTemplateID == 0` → `IS NULL` sentinel
- Parse `permissions` from comma-separated string
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
- SQLite: `INSERT ... VALUES(?, ?, ?, ?, ?) RETURNING id, created_at, updated_at` + `.Scan(&template.ID, &template.CreatedAt, &template.UpdatedAt)`
- `permissions`: `json.Marshal(template.Permissions)` → store as text
- Handle nullable `tenant_id` and `created_by` with `nil`/interface{}

**`GetTenantRoleTemplate`**
- Postgres: Complex WHERE with nullable tenant_id (-1 = NULL check)
- SQLite: Same logic with `?` placeholders
- **Critical (M3):** Apply `-1` sentinel pattern for global templates
- `sql.NullInt32` for `tenant_id` and `created_by`
- `json.Unmarshal` on `permissions` column

**`ListTenantRoleTemplates`**
- Postgres: Same nullable tenant_id logic, ORDER BY code ASC
- SQLite: Same pattern
- **Critical (M3):** Apply `-1` sentinel pattern

**`UpdateTenantRoleTemplate`**
- Postgres: Fetch existing, merge changes, UPDATE
- SQLite: Same pattern — fetch, merge, update with `?`
- **Note (N5):** Name/Code cannot be cleared via Update (guarded by `!= ""`)

**`DeleteTenantRoleTemplate`**
- Postgres: Check for active assignments first, then DELETE
- SQLite: Same guard logic
- **Note (N6):** Non-atomic pattern (SELECT then DELETE) — acceptable for this use case since the FK has `ON DELETE SET NULL`

#### 3. System Secrets

**`GetSystemSecret`**
- Postgres: `SELECT id, encryption_salt, key_version, created_at, rotated_at FROM system_secret WHERE id = 1`
- SQLite: Same query (both use `id = 1` singleton pattern)
- Handle `sql.ErrNoRows` → return `nil, nil`
- `rotated_at`: `sql.NullInt64` → parse Unix timestamp

**`UpsertSystemSecret`**
- Postgres: `INSERT ... ON CONFLICT(id) DO UPDATE SET ... RETURNING id`
- SQLite: **Must use `QueryRowContext` + `RETURNING id` + `.Scan(&secret.ID)`** (M2) — consistent with existing `UpsertTenantConfig` pattern (line 148) and Postgres reference (line 250)
- Set `created_at` and `rotated_at` as `now.Unix()` (N4)

```sql
INSERT INTO system_secret (id, encryption_salt, key_version, created_at)
VALUES (1, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    encryption_salt = excluded.encryption_salt,
    key_version = excluded.key_version,
    rotated_at = ?
RETURNING id
```

---

## Translation Reference: Postgres → SQLite

```go
// Postgres pattern:
row := d.db.QueryRowContext(ctx, `
    INSERT INTO table(col1, col2) VALUES($1, $2) RETURNING id
`, val1, val2)
row.Scan(&id)

// SQLite equivalent (RETURNING — preferred for consistency):
row := d.db.QueryRowContext(ctx, `
    INSERT INTO table(col1, col2) VALUES(?, ?) RETURNING id
`, val1, val2)
row.Scan(&id)
```

Both Postgres and SQLite support `RETURNING` in this codebase (`modernc.org/sqlite` targets 3.39+). Use `RETURNING` consistently — it matches the existing `UpsertTenantConfig` pattern and avoids losing Scan-based error paths.

---

## Files to Modify

| File | Action | Lines |
|------|--------|-------|
| `store/db/sqlite/rbac.go` | **EXTEND** (append 14 methods after existing 3) | ~395 new lines (~560 total) |
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

### 2. Existing RBAC Test
```bash
go test ./store/test/ -v -run TestTenantRoleTemplateCRUD -count=1
```
> **Note (N7):** `TestRBAC` and `TestSystemSecret` do not exist. `TestTenantRoleTemplateCRUD` is the only existing RBAC-related test. No standalone `UserTenantPermission` or `SystemSecret` tests exist yet.

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
| Missing test coverage | Medium | Low | `TestTenantRoleTemplateCRUD` exercises most RBAC paths |
| `ON CONFLICT` syntax differences | Low | Medium | Already used in same file (`UpsertTenantConfig` line 133) |
| `-1` sentinel not implemented | Medium | High | Documented in Pattern 1 above; must follow exactly |
| `SourceTemplateID == 0` not implemented | Medium | High | Documented in Pattern 2 above; must follow exactly |

---

## Definition of Done

- [ ] `store/db/sqlite/rbac.go` extended with all 14 methods
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes (no pre-existing issues)
- [ ] `go test ./store/test/ -v -run TestTenantRoleTemplateCRUD -count=1` passes
- [ ] `go test ./... -count=1` passes (or only pre-existing failures)
- [ ] All 3 drivers (sqlite, postgres, mysql) implement 200/200 interface methods
