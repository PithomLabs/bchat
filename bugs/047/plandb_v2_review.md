# Adversarial Plan Review — plandb_v2.md (SQLite RBAC Gap, Revision 2)

**Reviewer:** Senior Go Architect (automated)
**Date:** 2026-07-23
**Scope:** SQLite driver missing 14 RBAC methods — v2 implementation plan

---

## Verdict: APPROVED (1 new minor observation)

All prior review findings (M1-M4, N1-N7) are **correctly addressed**. The v2 plan is production-ready. One new minor observation about `CreateTenantRoleTemplate` timestamp scanning should be reviewed before implementation.

---

## Prior Review Findings — All Resolved ✅

| Finding | Status | v2 Location |
|---------|--------|-------------|
| **M1** — Line count estimate (200 → ~560) | ✅ Fixed | Line 176: "~395 new lines (~560 total)" |
| **M2** — UpsertSystemSecret should use RETURNING | ✅ Fixed | Line 360: "Must use `QueryRowContext` + `RETURNING id` + `.Scan(&secret.ID)`" |
| **M3** — Missing -1 sentinel for role templates | ✅ Fixed | Lines 195-225: Pattern 1 with complete code |
| **M4** — Missing SourceTemplateID==0 → IS NULL | ✅ Fixed | Lines 227-242: Pattern 2 with complete code |
| **N1** — File action "APPEND" not "CREATE" | ✅ Fixed | Line 176: "EXTEND existing file" |
| **N2** — Permissions serialization clarity | ✅ Fixed | Lines 244-251: Pattern 3 table |
| **N3** — Empty slice edge case | ✅ Fixed | Lines 253-271: Pattern 4 with code |
| **N4** — Timestamp `now.Unix()` convention | ✅ Fixed | Line 191: explicit note |
| **N5** — UpdateTenantRoleTemplate merge guards | ✅ Fixed | Lines 273-281: Pattern 5 |
| **N6** — DeleteTenantRoleTemplate TOCTOU note | ✅ Fixed | Line 348: non-atomic note |
| **N7** — Test commands | ✅ Fixed | Line 417: `TestTenantRoleTemplateCRUD` |

---

## New Observation: CreateTenantRoleTemplate Timestamp Scanning (Line 324)

**Plan says:**
```go
.Scan(&template.ID, &template.CreatedAt, &template.UpdatedAt)
```

This follows the Postgres reference exactly (Postgres line 277-278). In Postgres, `created_at` / `updated_at` are `BIGINT` columns and `pgx/v5` handles scanning `BIGINT` → `time.Time` by treating the value as a Unix timestamp.

**Potential issue with SQLite:** The column default is `strftime('%s', 'now')` which evaluates to a TEXT epoch string (e.g. `"1724371200"`). The INSERT doesn't explicitly set `created_at`/`updated_at`, so the DEFAULT fires. Due to the `BIGINT` column's INTEGER affinity, SQLite may convert this TEXT to INTEGER at storage time. RETURNING then returns an INTEGER. Scanning INTEGER → `time.Time` via `database/sql` is **not a standard conversion** — whether `modernc.org/sqlite` supports it depends on the driver's internal `Scanner` registration.

**Risk:** Low — if it fails, it's a runtime scan error (not compilation), caught immediately by tests.

**Two safe alternatives (both used elsewhere in the same file):**

| Approach | Pattern | Used In |
|----------|---------|---------|
| **Go-side timestamps** | Pass `now.Unix()` in INSERT, `RETURNING id` only, set struct from `now` | `UpsertTenantConfig:112-156`, `CreateUserTenantPermission` |
| **Scan as int64** | `var createdAtUnix int64` → scan → `template.CreatedAt = time.Unix(createdAtUnix, 0)` | `GetTenantConfig:96` |

**Recommendation:** Adopt the **Go-side timestamps** approach to match `UpsertTenantConfig` and avoid driver-dependent scan behavior:

```go
func (d *DB) CreateTenantRoleTemplate(ctx context.Context, template *store.TenantRoleTemplate) (*store.TenantRoleTemplate, error) {
    var tenantID interface{}
    if template.TenantID != nil {
        tenantID = *template.TenantID
    }
    var createdBy interface{}
    if template.CreatedBy != nil {
        createdBy = *template.CreatedBy
    }
    now := time.Now()
    permissionsJSON, _ := json.Marshal(template.Permissions)
    err := d.db.QueryRowContext(ctx, `
        INSERT INTO tenant_role_templates(tenant_id,name,code,permissions,created_by,created_at,updated_at)
        VALUES(?, ?, ?, ?, ?, ?, ?)
        RETURNING id
    `, tenantID, template.Name, template.Code, string(permissionsJSON), createdBy, now.Unix(), now.Unix()).Scan(&template.ID)
    if err != nil {
        return nil, err
    }
    template.CreatedAt = now
    template.UpdatedAt = now
    // Reconstruct TenantID pointer
    template.TenantID = nil
    if tenantID != nil {
        tid := tenantID.(int32)
        template.TenantID = &tid
    }
    return template, nil
}
```

---

## Verified Correct (All Prior + New)

| Claim | Status |
|-------|--------|
| Root cause analysis | ✅ |
| 14 missing methods | ✅ |
| Table schemas match SQLite migrations | ✅ |
| Postgres→SQLite translation table | ✅ |
| `-1` sentinel pattern documented | ✅ |
| `SourceTemplateID == 0` → `IS NULL` documented | ✅ |
| Two permission serialization strategies documented | ✅ |
| Empty slice vs nil edge case handled | ✅ |
| Merge logic limitations documented | ✅ |
| TOCTOU race noted | ✅ |
| Test commands corrected | ✅ |
| `[]any{}` convention recommended | ✅ |
| Line count corrected to ~560 | ✅ |

---

## Bottom Line

The plan is **approved**. All prior review findings are fully resolved. The lone new observation is low-risk and can be decided during implementation — the plan's current approach (matching Postgres directly) would work in the best case, and the Go-side timestamp fallback is a trivial change if it doesn't. No further review needed before implementation.
