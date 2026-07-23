# Adversarial Plan Review — plandb.md (SQLite RBAC Gap)

**Reviewer:** Senior Go Architect (automated)
**Date:** 2026-07-23
**Scope:** SQLite driver missing 14 RBAC methods — implementation plan

---

## Verdict: APPROVED with Nits (4 moderate, 7 minor)

The plan is **fundamentally sound** — root cause analysis is correct, translation approach (Postgres → SQLite) is appropriate, and the scope of work is well-defined. No critical blockers. Four moderate issues and several minor nits should be addressed before implementation.

---

## Verified Correct

| Claim | Status | Evidence |
|-------|--------|----------|
| 14 methods missing in SQLite | ✅ | `store/db/sqlite/sqlite.go:49` — compilation fails: `*DB does not implement Driver` |
| All 14 exist in Postgres | ✅ | `store/db/postgres/rbac.go` — all 17 methods (3 TenantConfig + 14 RBAC) |
| Tables exist in SQLite migrations | ✅ | `store/migration/sqlite/0.25/07__rbac_tables.sql`, `0.26/04__tenant_role_templates.sql`, `0.33/01__add_system_secret.sql` |
| `RETURNING` supported by modernc.org/sqlite | ✅ | Already used in same file (`UpsertTenantConfig` line 148), plus `agent.go`, `memo.go` |
| `ON CONFLICT` supported | ✅ | Already used in same file (`UpsertTenantConfig` line 133) |
| No other files need modification | ✅ | Go auto-implements interface via method resolution on `*DB` |
| SQLite supports `RETURNING` with upserts | ✅ | SQLite 3.35+; `modernc.org/sqlite` targets SQLite 3.39+ compatibility |

---

## Moderate Issues (Must Fix Before Implementation)

### M1: Line Count Estimate Is Off by 3-4× (Lines 175, 293)

**Plan says:** `~200 lines`

**Reality:** The existing `store/db/sqlite/rbac.go` already occupies **163 lines** for just 3 methods (GetTenantConfig, UpsertTenantConfig, DeleteTenantConfig). Adding 14 more methods (many longer — `ListTenantRoleTemplates` alone is 60+ lines in Postgres) will produce a file of **~500–700 lines**.

| Method Group | Postgres Lines | Estimated SQLite Lines |
|---|---|---|
| 7 × UserTenantPermission | 108 (avg 15/ea) | ~120 |
| 5 × TenantRoleTemplate | 201 (avg 40/ea) | ~220 |
| 2 × SystemSecret | 47 (avg 23/ea) | ~55 |
| Subtotal new methods | 356 | ~395 |
| Existing 3 methods | — | 163 |
| **Total** | **462** | **~558** |

**Fix:** Update line count estimate to 500–600 lines. Consider splitting into `rbac_permissions.go` + `rbac_templates.go` if file grows unwieldy.

---

### M2: UpsertSystemSecret Should Use `RETURNING`, Not `Exec` (Lines 256-258)

**Plan says:** `INSERT ... ON CONFLICT(id) DO UPDATE SET ...` (no RETURNING needed; use `Exec`)

**Problem:** This is inconsistent with the existing codebase pattern. The same file's `UpsertTenantConfig` (line 150) uses `QueryRowContext` + `RETURNING id` + `.Scan(&config.ID)`. The Postgres reference also uses `RETURNING id` (line 250). Using `Exec` loses the Scan-based error path.

**Impact:** Using `Exec` works functionally (ID is always 1) but creates an inconsistency that will confuse future readers. If the secret.ID ever matters for error checking, the pattern is wrong.

**Fix:** Use `QueryRowContext` + `RETURNING id` + `.Scan(&secret.ID)` matching both the Postgres reference and existing `UpsertTenantConfig` pattern:
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

### M3: Missing Discussion of `-1` Sentinel for Global Role Templates (Lines 229-237)

**Plan says:** Simple nullable tenant_id logic with `?` placeholders

**Reality:** `GetTenantRoleTemplate` and `ListTenantRoleTemplates` in Postgres use a **sentinel value `-1`** to represent "global/system-level template" (tenant_id IS NULL). When `find.TenantID == -1`, the SQL uses `tenant_id IS NULL`. On read, `tenantID.Valid && tenantID.Int32 != -1` guards against exposing the sentinel.

```go
// Postgres GetTenantRoleTemplate (lines 297-308):
if find.ID == nil {
    if find.TenantID != nil {
        if *find.TenantID == -1 {
            where = append(where, "tenant_id IS NULL")  // sentinel!
        } else {
            args = append(args, *find.TenantID)
            where = append(where, fmt.Sprintf("tenant_id=$%d", len(args)))
        }
    } else {
        where = append(where, "tenant_id IS NULL")
    }
}
```

**Impact:** Without this pattern in the plan, an implementer translating naively would produce incorrect queries for global templates.

**Fix:** Add a dedicated section documenting the `-1` sentinel pattern and its read-side guard (`tenantID.Valid && tenantID.Int32 != -1`).

---

### M4: `SourceTemplateID == 0` → IS NULL Logic Missing (Line 201)

**Plan says:** `WHERE id=$N, user_id=$N, tenant_id=$N, source_template_id=$N`

**Reality:** `ListUserTenantPermissions` has a special case (Postgres line 51): when `find.SourceTemplateID == 0`, the query uses `source_template_id IS NULL` instead of `source_template_id = $N`. This filters for **explicit** permissions (not linked to a template).

```go
if find.SourceTemplateID != nil {
    if *find.SourceTemplateID == 0 {
        where = append(where, "source_template_id IS NULL")
    } else {
        args = append(args, *find.SourceTemplateID)
        where = append(where, fmt.Sprintf("source_template_id=$%d", len(args)))
    }
}
```

**Impact:** The plan's simplified description of ListUserTenantPermissions as "same logic with ? placeholders" and "handle sql.NullInt32 for granted_by and source_template_id" misses this critical filtering logic.

**Fix:** Document the `SourceTemplateID == 0` → `IS NULL` sentinel in ListUserTenantPermissions.

---

## Minor Issues (Nits)

### N1: File Action Is "APPEND", Not "CREATE" (Line 173)

**Plan says:** **CREATE** `store/db/sqlite/rbac.go`

**Reality:** The file already exists at 163 lines with 3 methods.

**Fix:** Change "CREATE" to "APPEND TO" or "EXTEND".

### N2: Permissions Serialization — JSON, Not Plain Text (Line 226)

**Plan says:** "store as text" for tenant_role_templates permissions

**Detail:** The `tenant_role_templates.permissions` column is stored as **JSON array** (Postgres uses `::jsonb`), not comma-separated text. `json.Marshal` on write, `json.Unmarshal` on read. This differs from `user_tenant_permission.permissions` which uses comma-separated `strings.Join/strings.Split`.

**Fix:** Clarify that two different serialization strategies are needed:
- `user_tenant_permission.permissions` → comma-separated → `strings.Join/Split`
- `tenant_role_templates.permissions` → JSON array → `json.Marshal/Unmarshal`

### N3: Empty Permissions → Non-Nil Slice Edge Case (Lines 76-80, 345-349)

**Plan says:** `strings.Split(permissions, ",")` for user_tenant_permission

**Edge case not documented:** When the DB value is `""`, Postgres returns `[]string{}` (non-nil empty slice), not `nil`. Same for tenant_role_templates — when permissions JSON is empty, returns `[]string{}`. This ensures callers can safely range over the result.

**Fix:** Add a note that both tables return `[]string{}` (not `nil`) when permissions are empty.

### N4: Timestamp Handling — `now.Unix()` Is the Convention (Multiple Lines)

**Plan says:** Uses `time.Now()` in several method descriptions

**Reference:** Both the Postgres reference and the existing `UpsertTenantConfig` (SQLite, lines 112, 152) use `now.Unix()` for all RBAC timestamps. The `created_at`/`updated_at` columns in tenant_role_templates store `int64` Unix epoch seconds, and `GrantedAt` is stored as `now.Unix()` then read via `time.Unix(grantedAt, 0)`.

**Fix:** Be explicit that RBAC timestamps use `now.Unix()` (int64), not raw `time.Time`. Standardize one pattern throughout.

### N5: `UpdateTenantRoleTemplate` Merge Logic Prevents Clearing Fields (Lines 239-241)

**Plan says:** "Fetch existing, merge changes, UPDATE"

**Detail:** Postgres (lines 430-438) uses non-empty-string guards:
```go
if template.Name != "" { existing.Name = template.Name }
if template.Code != "" { existing.Code = template.Code }
if template.Permissions != nil { existing.Permissions = template.Permissions }
```
This means **you cannot set Name or Code to empty string** via Update — the merge always keeps the old value. This is by design but should be documented.

**Fix:** Add a note that Name/Code cannot be cleared via Update (guarded by `!= ""`).

### N6: `DeleteTenantRoleTemplate` Has TOCTOU Race (Lines 243-245)

**Plan says:** "Check for active assignments first, then DELETE"

**Detail:** Postgres (lines 453-461) does a separate `SELECT COUNT(*)` before `DELETE`. In a concurrent system, a permission could be created between the SELECT and DELETE, leaving orphaned FK references (though the FK has `ON DELETE SET NULL`, so no integrity violation — just stale data).

**Fix:** Not a blocker, but add a note that this is a non-atomic pattern (SELECT then DELETE) that could miss concurrent inserts. For correctness, wrap in a transaction or use a single `DELETE` with a check in the application layer.

### N7: Test Commands Are Speculative (Lines 309-313)

**Plan says:**
```bash
go test ./store/test/ -v -run TestRBAC
go test ./store/test/ -v -run TestSystemSecret
```

**Reality:**
- `TestRBAC` — **does not exist.** No test function with "RBAC" in the name.
- `TestSystemSecret` — **does not exist.** No standalone SystemSecret test.
- `TestTenantRoleTemplateCRUD` — **exists** in `store/test/role_template_test.go`. This is the only RBAC-related test.

**Fix:** Change commands to:
```bash
go test ./store/test/ -v -run TestTenantRoleTemplateCRUD    # Only existing RBAC test
go test ./store/test/ -v -count=1                           # Full test suite
```
And add a note that no standalone UserTenantPermission or SystemSecret tests exist yet.

---

## Coding Convention Recommendations

The existing `store/db/sqlite/rbac.go` uses `[]interface{}{}` for args (legacy convention). The Postgres reference uses `[]any{}` (newer Go 1.21+ convention). The newer SQLite code in `agent.go` uses `[]any{}`. **Recommend using `[]any{}`** for consistency with the rest of the growing codebase.

---

## Summary

| Category | Count | Key Items |
|----------|-------|-----------|
| **Moderate** | 4 | M1: line count estimate (200 → ~550), M2: UpsertSystemSecret should use RETURNING, M3: missing -1 sentinel for role templates, M4: missing SourceTemplateID==0 → IS NULL logic |
| **Minor** | 7 | N1-N7: file action, serialization clarity, empty slice edge case, timestamp convention, merge limitations, TOCTOU race, test commands |
| **Verified Correct** | 6 | missing methods, table schemas, RETURNING support, ON CONFLICT support, no other files needed, modernc/sqlite version |

**Bottom line:** The plan is production-ready after addressing M2, M3, M4, and minor nit corrections. No architectural rework needed.
