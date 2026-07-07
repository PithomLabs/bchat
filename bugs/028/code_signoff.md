# Code Signoff: bugs/028 Schema Parity & Test Fixes

**Date:** 2026-07-08
**Scope:** SQLite ↔ Postgres schema parity, bugs/027 (UpsertTenantConfig), bugs/028 (4 failing tests), code review findings

---

## 1. Executive Summary

Achieved bidirectional schema parity between SQLite (local dev) and Postgres (production Neon DB) for the bchat multi-tenant AI chat agent platform. Fixed 5 failing Go tests (4 original bugs/028 + 1 code review regression). Applied 4 code review findings from adversarial review.

**Final state:** All tests pass (`go test ./...` clean), build compiles, migration validation passes.

---

## 2. Files Modified

### Schema Files (2)
| File | Changes |
|------|---------|
| `store/migration/postgres/LATEST.sql` | 5 columns, 6 indexes, `ON CONFLICT DO NOTHING` fix, 7 CHECK constraints, 5 defaults, 2 FKs, NOT NULL on `agent_sessions` timestamps |
| `store/migration/sqlite/LATEST.sql` | UNIQUE constraint, NOT NULL + UNIQUE on `guid`, NOT NULL on 20+ columns, CHECK on `audience_type`, FK on 17 tables, NOT NULL on `tenant_config` 5 columns, NOT NULL on `agent_sessions` timestamps |

### Store Layer (2)
| File | Changes |
|------|---------|
| `store/db/postgres/rbac.go` | Added `vector_db_s3_override` to `GetTenantConfig`/`UpsertTenantConfig`, `COALESCE` consistency |
| `store/migration_helper.go` | SQLite-only warning comments |

### Handler (1)
| File | Changes |
|------|---------|
| `server/router/api/v1/agent/handlers.go` | Rewrote `HandleAssignRoleTemplate` to handle existing permissions via UPDATE instead of failing on INSERT |

### Test Files (8)
| File | Changes |
|------|---------|
| `store/test/migrator_test.go` | Fixed stale schema version assertion `0.29.x` → `0.30.x` |
| `store/test/ticket_test.go` | Added SQLite skip guard + `os` import |
| `store/test/bridge_auth_test.go` | Added SQLite skip guard |
| `store/test/bridge_test.go` | Added SQLite skip guard + `os` import |
| `store/test/schema_validation_test.go` | Added SQLite skip guards |
| `store/test/bridge_postgres_cascade_test.go` | **New file** — Postgres FK cascade verification |
| `server/router/api/v1/agent/bridge_endpoints_test.go` | Added SQLite skip guard |
| `server/router/api/v1/agent/bridge_delivery_test.go` | Added SQLite skip guard |
| `server/router/api/v1/agent/bridge_middleware_test.go` | Added SQLite skip guard |
| `server/router/api/v1/agent/role_template_handler_test.go` | Separate admin user, rewrote 2 tests for UNIQUE constraint |

---

## 3. Schema Parity Changes

### 3.1 Postgres LATEST.sql Additions

**Columns added (5):**
- `user.allowed_tenant_ids TEXT DEFAULT '[]'`
- `memo_relation.tenant_id INTEGER DEFAULT NULL`
- `user_tenant_permission.source_template_id INTEGER DEFAULT NULL`
- `tenant_config.admin_mutation_rate_limit_rpm INTEGER NOT NULL DEFAULT 30`
- `tenant_config.vector_db_s3_override TEXT DEFAULT ''`

**Indexes added (6):**
- `idx_agent_tenants_guid` on `agent_tenants(guid)`
- `idx_agent_audiences_tenant` on `agent_audiences(tenant_id, audience_type)`
- `idx_user_tenant_permission_user` on `user_tenant_permission(user_id)`
- `idx_user_tenant_permission_tenant` on `user_tenant_permission(tenant_id)`
- `idx_user_tenant_permission_template` on `user_tenant_permission(source_template_id)` (IF NOT EXISTS)
- `idx_tenant_config_tenant` on `tenant_config(tenant_id)` (IF NOT EXISTS)

**Bug fix:**
- Line 672: `INSERT OR IGNORE` → `INSERT INTO ... ON CONFLICT DO NOTHING`

**CHECK constraints added (7):**
- `agent_tenants.slug` — `CHECK (slug != '')`
- `agent_tenants.guid` — `CHECK (guid != '')`
- `agent_tenants.company_name` — `CHECK (company_name != '')`
- `agent_source_files.file_type` — `CHECK (file_type IN ('kb', 'policy'))`
- `agent_source_files.audience_type` — `CHECK (audience_type IN ('internal', 'external'))`
- `agent_sessions.audience_type` — `CHECK (audience_type IN ('internal', 'external'))`
- `agent_audiences.audience_type` — `CHECK (audience_type IN ('internal', 'external'))`

**Default values added (5):**
- `agent_tenants.is_active` — `DEFAULT TRUE`
- `agent_tenants.created_at` — `DEFAULT NOW()`
- `agent_tenants.updated_at` — `DEFAULT NOW()`
- `agent_source_files.content_hash` — `DEFAULT ''`
- `agent_source_files.imported_at` — `DEFAULT NOW()`

**FKs added (2):**
- `agent_leads.transcript_id` → `agent_transcripts(id) ON DELETE SET NULL`
- `bridge_handoffs.tenant_id` → `agent_tenants(id) ON DELETE CASCADE`

**NOT NULL added (2):**
- `agent_sessions.created_at NOT NULL DEFAULT NOW()`
- `agent_sessions.updated_at NOT NULL DEFAULT NOW()`

### 3.2 SQLite LATEST.sql Additions

**Constraints added:**
- `user_tenant_permission`: `UNIQUE(user_id, tenant_id)`
- `agent_tenants.guid`: `NOT NULL UNIQUE` (was nullable TEXT)

**NOT NULL added (20+ columns):**
- `agent_tenants.created_at`, `agent_tenants.updated_at`
- `agent_audiences`: `guidelines`, `secondary_phones`, `emergency_urgency_threshold`, `escalation_confidence_threshold`, `rate_limit_rpm`, `require_contact_on_fallback`, `max_message_length`, `updated_at`
- `tenant_config`: `simulation_human_model`, `retrieval_mode`, `content_tokens`, `record_transcripts`, `reasoning_model`
- `agent_sessions.created_at`, `agent_sessions.updated_at`

**CHECK constraint added:**
- `agent_audiences.audience_type`: `CHECK (audience_type IN ('internal', 'external'))`

**FKs added (17):**
- `agent_source_files.tenant_id` → `agent_tenants(id) ON DELETE CASCADE`
- `agent_tenant_scripts.tenant_id` → `agent_tenants(id) ON DELETE CASCADE`
- `agent_analysis_results.tenant_id` → `agent_tenants(id) ON DELETE CASCADE`
- `agent_audiences.tenant_id` → `agent_tenants(id) ON DELETE CASCADE`
- `agent_sessions.tenant_id` → `agent_tenants(id) ON DELETE CASCADE`
- `agent_observation_logs.tenant_id` → `agent_tenants(id) ON DELETE CASCADE`
- `agent_leads.tenant_id` → `agent_tenants(id) ON DELETE CASCADE`
- `agent_lead_transcripts.tenant_id` → `agent_tenants(id) ON DELETE CASCADE`
- `agent_lead_summaries.tenant_id` → `agent_tenants(id) ON DELETE CASCADE`
- `agent_analysis_results.conversation_id` → `agent_sessions(id) ON DELETE CASCADE`
- `agent_observation_logs.session_id` → `agent_sessions(id) ON DELETE CASCADE`
- `agent_lead_transcripts.transcript_id` → `agent_lead_transcripts(id) ON DELETE CASCADE`
- `bridge_external_sessions.tenant_id` → `agent_tenants(id) ON DELETE CASCADE`
- `bridge_auth_keys.tenant_id` → `agent_tenants(id) ON DELETE CASCADE`
- `bridge_handoffs.tenant_id` → `agent_tenants(id) ON DELETE CASCADE`
- `bridge_reply_outbox.tenant_id` → `agent_tenants(id) ON DELETE CASCADE`
- `bridge_handoff_replies.tenant_id` → `agent_tenants(id) ON DELETE CASCADE`

---

## 4. Bug Fixes

### 4.1 bugs/027: UpsertTenantConfig Missing Column

**Root cause:** `vector_db_s3_override` column was missing from Postgres `GetTenantConfig`/`UpsertTenantConfig`, causing "column not found" errors on Postgres.

**Fix:** Added `vector_db_s3_override` to:
- `store/db/postgres/rbac.go` `GetTenantConfig` SELECT query and Scan
- `store/db/postgres/rbac.go` `UpsertTenantConfig` INSERT/ON CONFLICT clause

### 4.2 bugs/028: 4 Failing Tests

#### Test 1: `TestGetCurrentSchemaVersion`

**Failure:** `assertion.go:89: Expected "0.30.0" to contain "0.29."`

**Root cause:** Schema version was bumped to `0.30.0` but test assertion still checked for `0.29.x`.

**Fix:** Changed assertion in `store/test/migrator_test.go` from `"0.29."` to `"0.30."`.

#### Test 2: `TestBridgeExternalSessionUsesAgentTenantsTable`

**Failure:** FK constraint violation — `bridge_external_sessions.tenant_id` referenced `agent_tenants(id)` but no FK was defined.

**Root cause:** SQLite LATEST.sql was missing FK on `bridge_external_sessions.tenant_id`.

**Fix:** Added `REFERENCES agent_tenants(id) ON DELETE CASCADE` to `bridge_external_sessions.tenant_id` in SQLite LATEST.sql.

#### Test 3: `TestBridgeAuthKeyTenantCascade`

**Failure:** FK cascade didn't fire — deleting a tenant didn't remove `bridge_auth_keys`.

**Root cause:** SQLite LATEST.sql was missing FK on `bridge_auth_keys.tenant_id`.

**Fix:** Added `REFERENCES agent_tenants(id) ON DELETE CASCADE` to `bridge_auth_keys.tenant_id` in SQLite LATEST.sql.

#### Test 4: `TestSQLiteFKCascadeOnTenantDeletion`

**Failure:** FK cascade didn't fire — deleting a tenant didn't remove `bridge_external_sessions` or `bridge_handoffs`.

**Root cause:** SQLite LATEST.sql was missing FKs on both tables' `tenant_id`.

**Fix:** Added `REFERENCES agent_tenants(id) ON DELETE CASCADE` to both `bridge_external_sessions.tenant_id` and `bridge_handoffs.tenant_id` in SQLite LATEST.sql.

### 4.3 Code Review Finding: TestRoleTemplateEndpoints

**Failure:** `code=500, message=Failed to assign role template`

**Root cause:** The `UNIQUE(user_id, tenant_id)` constraint on `user_tenant_permission` (added in Phase 2a) caused `CreateUserTenantPermission` to fail when a user already had a permission row. The handler's idempotency check only filtered by `SourceTemplateID`, missing the existing row.

**Fix (handler):** Rewrote `HandleAssignRoleTemplate` in `handlers.go:2966-2998`:
```go
// Check for any existing permission for this user+tenant
existingPerm, err := h.store.GetUserTenantPermission(ctx, &store.FindUserTenantPermission{
    UserID:   &req.UserID,
    TenantID: &tenant.ID,
})

if existingPerm != nil {
    // Same template → idempotent
    if existingPerm.SourceTemplateID != nil && *existingPerm.SourceTemplateID == int32(templateID) {
        return c.JSON(http.StatusOK, map[string]interface{}{"success": true, "created": false})
    }
    // Different template → UPDATE existing row
    existingPerm.Permissions = template.Permissions
    existingPerm.SourceTemplateID = intPtr(int32(templateID))
    _, err := h.store.UpdateUserTenantPermission(ctx, existingPerm)
    // ...
}
// No existing → INSERT new
```

**Fix (tests):** Updated `role_template_handler_test.go`:
- Created separate `adminUser` for admin operations (template assignment replaces permissions)
- Rewrote `revoke_preserves_template_assignments` — UNIQUE constraint means 1 row per user+tenant
- Rewrote `grant_deduplicates_orphaned_explicit_rows` — can't create duplicate rows

### 4.4 Code Review Finding: SQLite Skip Guards

**Problem:** Tests using `sqlite_master` or `PRAGMA table_info()` fail on Postgres.

**Fix:** Added skip guards to 7 test files:
```go
if os.Getenv("DRIVER") != "" && os.Getenv("DRIVER") != "sqlite" {
    t.Skip("SQLite-specific test")
}
```

### 4.5 Code Review Finding: Postgres Cascade Test Gap

**Problem:** SQLite skip guards meant no Postgres FK cascade test existed.

**Fix:** Created `store/test/bridge_postgres_cascade_test.go` — verifies FK cascades work on Postgres by creating a session, deleting the tenant, and confirming cascade deletion.

---

## 5. Verification

### Build
```
task build:backend
✓ LATEST.sql is in sync with all migrations
✓ Build succeeds
```

### Tests
```
go test ./store/test/... -count=1
ok   github.com/usememos/memos/store/test    7.634s

go test ./server/router/api/v1/agent/... -count=1
ok   github.com/usememos/memos/server/router/api/v1/agent    5.191s

go test ./...
ok   (all packages pass)
```

### Migration Validation
```
./scripts/validate-migrations.sh
✓ LATEST.sql is in sync with all migrations
```

---

## 6. Remaining Follow-ups (Out of Scope)

| Item | Severity | Notes |
|------|----------|-------|
| `vector_db_s3_override` plaintext storage | Medium | S3 credentials stored unencrypted. Design decision, not code bug. |
| Dual LATEST.sql CI validation | Low | No automated check enforces cross-DB parity. |
| `migration_helper.go` function export | Low | Functions documented as SQLite-only but still exported. |
| `schema_validation_test.go` depth | Low | Only validates column names, not types/constraints. |

---

## 7. Signoff

All 5 failing tests fixed. Build compiles. Migration validation passes. Schema parity achieved between SQLite and Postgres for fresh DB deployments.
