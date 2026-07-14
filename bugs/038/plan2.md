# Bug #038: RAG Version Rollback UI + tenant:admin RBAC Fix

**Status:** Plan v2 (reviewed, awaiting approval)
**Created:** 2026-07-15
**Changes from v1:** Incorporates plan_review.md findings (race condition fix, route group correction, HandleReindexTenant coverage, error message updates)

---

## 1. Background: LanceDB Versioning Architecture

### Overview

The RAG (Retrieval-Augmented Generation) system uses LanceDB as its vector database. Each tenant's vectors are stored in an isolated LanceDB instance — either a local directory (`build/data/lancedb/<tenantID>/`) or an S3 prefix (`s3://bucket/lancedb/<tenantID>/`).

### Four-Layer Version Tracking

| Layer | Storage | Purpose |
|-------|---------|---------|
| **Source File Version** | SQLite `agent_source_files` | Every upload creates a new row with `MAX(version) + 1` per `(tenant_id, audience_type, file_type)` |
| **Indexed Source Version** | LanceDB row metadata | Each `DocumentChunk` carries a `source_version` field stored in the `source_version` Arrow column |
| **Active Version Pointer** | SQLite `agent_rag_active_versions` | Points to which indexed version is "current" for query purposes per `(tenant_id, audience_type, file_type)` |
| **Query Version Resolution** | `resolveQueryVersion()` in `service.go:4381-4421` | Resolves which version to query: explicit request > active pointer > latest indexed > nil |

### Upload vs Rebuild Index Flow

**Upload (no auto-reindex):**
1. Admin uploads KB/Policy file
2. `UpsertAgentSourceFile` creates a new row in `agent_source_files` with version N
3. RAG reindex is **NOT** triggered — explicit comment at `handlers.go:1601-1602`
4. Old data remains searchable via the active-version pointer (pointing to version N-1)

**Rebuild Index (manual):**
1. Admin clicks "Rebuild Index"
2. `HandleReindexTenant` spawns a **background goroutine** (`handlers.go:1231`)
3. Reindex reads `ListAgentSourceFiles(LatestOnly: true)` → gets version N
4. Chunks are created with `source_version = N` embedded in each row
5. New versioned chunks are **appended** to the existing LanceDB table (old versions coexist)
6. `UpsertAgentRAGActiveVersion` sets the pointer to version N
7. Retention policy: if more than 5 indexed versions exist, oldest ones are deleted via `DeleteByVersion`

### Key Files

| File | Purpose |
|------|---------|
| `server/router/api/v1/agent/vectordb.go:237-253` | Per-tenant storage path resolution (S3 and local) |
| `server/router/api/v1/agent/vectordb_pool.go:15-98` | TenantVectorDBPool — lazy per-tenant LanceDB instances |
| `server/router/api/v1/agent/vectordb_lance.go:29-32` | Table naming by dimension (`kb_documents_<dim>`) |
| `server/router/api/v1/agent/vectordb_lance.go:1167-1169` | Query filtering by `tenant_id` |
| `server/router/api/v1/agent/service.go:291-337` | `reindexFileVersion` — append-only versioned indexing |
| `server/router/api/v1/agent/service.go:747` | `ReindexTenantContentWithResume` — full reindex flow |
| `server/router/api/v1/agent/service.go:4381-4421` | `resolveQueryVersion` — version selection algorithm |
| `store/agent.go:349-367` | `AgentRAGActiveVersion` struct and filter |
| `store/db/sqlite/agent.go:1244-1327` | Active version CRUD (SQLite) |
| `store/db/postgres/agent.go:1154-1232` | Active version CRUD (Postgres) |
| `store/migration/sqlite/0.31/02__rag_active_versions.sql` | Migration for `agent_rag_active_versions` table |

---

## 2. Background: RBAC System

### Three-Tier Global Roles

| Role | Description | Capabilities |
|------|-------------|--------------|
| **HOST** | Instance owner (first user) | Full system access, all permissions, all tenants |
| **ADMIN** | Administrator | Access admin pages, manage tenants; super user only when `AllowedTenantIDs` is empty |
| **USER** | Regular user | Basic memo functionality; gains access via explicit permission grants |

### Tenant-Level Permissions

Permissions are stored in `user_tenant_permission` table as comma-separated strings per `(user_id, tenant_id)` pair.

| Permission | Description | Current Auto-implies |
|------------|-------------|---------------------|
| `tenant:admin` | Full tenant management | `tenant:read`, `tenant:write` (prefix match `tenant:*`) |
| `tenant:read` | View tenant config, stats | — |
| `tenant:write` | Modify tenant settings | — |
| `chat:test` | Test chat functionality | — |
| `chat:logs` | View conversation history | — |
| `files:upload` | Upload KB/Policy files | — |
| `files:restore` | Restore file versions | — |
| `api:config` | Configure LLM settings and API keys | — |
| `*` | Wildcard (HOST only) | All permissions |

### Resolution Algorithm

From `permissions.go:135-181` (`ResolveEffectivePermissions`):

1. **HOST** → returns `*` (wildcard, all permissions)
2. **ADMIN** → returns `tenant:read` + `api:config` (auto-granted for all tenants)
3. **USER** → unions all `user_tenant_permission` rows for the user/tenant

### Current Permission Expansion

From `permissions.go:82-98` (`containsResolvedPermission`):

```go
func containsResolvedPermission(permissions []ResolvedPermission, required string) bool {
    for _, p := range permissions {
        if p.Permission == PermWildcard || p.Permission == required {
            return true
        }
        if strings.HasSuffix(p.Permission, ":*") {
            prefix := strings.TrimSuffix(p.Permission, "*")
            if strings.HasPrefix(required, prefix) {
                return true
            }
        }
        if p.Permission == PermTenantAdmin && strings.HasPrefix(required, "tenant:") {
            return true
        }
    }
    return false
}
```

### Current Enforcement Points

| Endpoint | Route Group | Permission Required |
|----------|-------------|---------------------|
| `GET /:slug/validate` | adminGroup | ADMIN or `chat:test` |
| `GET /:slug/config` | adminGroup | ADMIN or `tenant:read` |
| `PATCH /:slug` | adminGroup | ADMIN or `tenant:write` |
| `POST /:slug/chat/int` | adminGroup | ADMIN or `chat:test` |
| `POST /:slug/import` | adminGroup | ADMIN or `files:upload` |
| `POST /:slug/reindex` | adminGroup | ADMIN or `api:config` |
| `POST /:slug/files/.../restore` | adminGroup | ADMIN or `files:restore` |
| `GET /:slug/files/.../versions` | adminGroup | ADMIN or `files:restore` or `tenant:read` |
| `PUT /:slug/llm-config` | adminGroup | ADMIN or `api:config` |
| `GET /:slug/sessions` | adminGroup | ADMIN or `chat:logs` |
| `GET/POST /:slug/permissions` | adminGroup | ADMIN or `tenant:admin` |
| `GET /:slug/rag/active-versions` | adminGroup | ADMIN or `api:config` |
| `GET /:slug/rag/indexed-versions` | adminGroup | ADMIN or `api:config` |
| `POST /:slug/rag/active-version` | adminGroup | ADMIN or `api:config` |
| `GET /agent/tenants` | adminGroup | ADMIN only |
| `POST /agent/onboard` | adminGroup | ADMIN only |
| `DELETE /:slug` | adminGroup | ADMIN only |

**Note:** All RAG rollback endpoints are in `adminGroup` (v1.go:371-375), which applies `TenantBindingMiddleware` + `CSRFProtectionMiddleware` in addition to `AuthMiddleware`. This provides tenant isolation at the middleware level.

---

## 3. Bug #1: tenant:admin Expansion is Incomplete

### Current Code

`permissions.go:93`:
```go
if p.Permission == PermTenantAdmin && strings.HasPrefix(required, "tenant:") {
    return true
}
```

### Problem

`tenant:admin` only expands to `tenant:*` prefix match. It does **NOT** grant:
- `chat:test`
- `chat:logs`
- `files:upload`
- `files:restore`
- `api:config`

These permissions do not start with `tenant:`, so the prefix check fails.

### Impact

The "Tenant Admin (full access)" preset in the UI (`AgentAdmin.tsx:3764`) maps to `["tenant:admin"]` (`agentAdmin.ts:357`). Users granted this preset see a "full access" label but are denied on:
- File upload/restore
- Chat testing
- Chat logs
- LLM configuration
- RAG index rollback

### Intended Behavior

`tenant:admin` should be a **superset of all permissions available to a tenant**. A tenant-admin is an admin scoped to a single tenant — they should have every permission that tenant offers.

### Fix

Change `permissions.go:93` from:
```go
if p.Permission == PermTenantAdmin && strings.HasPrefix(required, "tenant:") {
    return true
}
```

To:
```go
if p.Permission == PermTenantAdmin {
    return true
}
```

### Behavioral Change

Existing `tenant:admin` users will gain access to:
- File upload (`files:upload`)
- File restore (`files:restore`)
- Chat test (`chat:test`)
- Chat logs (`chat:logs`)
- LLM config (`api:config`)
- RAG rollback (after Bug #2 fix)
- RAG reindex (`HandleReindexTenant`)

This is the **intended behavior** — the label says "full access" and it should deliver it.

### Security Verification

Admin-only endpoints remain protected by `isAdmin()` (not `hasPermission()`), so they are NOT affected:
- `GET /agent/tenants` — list all tenants
- `POST /agent/onboard` — create new tenant
- `DELETE /:slug` — delete tenant
- `GET /:slug/export` — export tenant data

These are correctly tenant-management operations that a tenant-admin should not perform.

---

## 4. Bug #2: RAG Rollback Has No Frontend UI

### Backend (Complete)

Three endpoints exist and are fully functional in `adminGroup`:

| Endpoint | Handler | Permission | Purpose |
|----------|---------|------------|---------|
| `GET /:slug/rag/active-versions` | `HandleListActiveVersions` (line 6061) | ADMIN or `api:config` | List current active version per (audience, file_type) |
| `GET /:slug/rag/indexed-versions` | `HandleListIndexedVersions` (line 6097) | ADMIN or `api:config` | List all versions present in LanceDB |
| `POST /:slug/rag/active-version` | `HandleSetActiveVersion` (line 5994) | ADMIN or `api:config` | Point active version to any indexed version (instant rollback) |

### Frontend (Missing)

Zero store methods, zero UI components call these endpoints. Confirmed by grep:
- `rag/active-version` — 0 matches in `web/src/`
- `rag/indexed-version` — 0 matches in `web/src/`
- `activeVersion` — 0 matches in `web/src/`
- `setActiveVersion` — 0 matches in `web/src/`

### Current Workaround

Admins must use `curl`:
```bash
# List indexed versions
curl -H "Authorization: Bearer <token>" \
  "http://localhost:8081/api/v1/agent/<slug>/rag/indexed-versions"

# Rollback to version 2
curl -X POST -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"audience_type":"internal","file_type":"kb","version":2}' \
  "http://localhost:8081/api/v1/agent/<slug>/rag/active-version"
```

---

## 5. Known Issue: Reindex/Rollback Race Condition

### The Race

`HandleReindexTenant` (`handlers.go:1231`) spawns a background goroutine that calls `reindexFileVersion` → `UpsertAgentRAGActiveVersion` (`service.go:317`). If `HandleSetActiveVersion` (rollback) is called concurrently:

1. Rollback sets active pointer to version 2 (old)
2. Reindex goroutine completes and overwrites active pointer to version 3 (new)
3. **Rollback is silently undone**

No mutex, no CAS, no advisory lock exists in either path.

### Why It Matters

An admin who rolls back while a reindex is running will see the rollback appear to succeed (HTTP 200) but the pointer will be overwritten moments later when the reindex goroutine finishes.

### Fix

Add a per-tenant mutex to serialize reindex and rollback operations. See Section 7, Step 2.

---

## 6. Fix Plan: tenant:admin Expansion

### Step 1: Update `containsResolvedPermission`

**File:** `server/router/api/v1/agent/permissions.go:93`

Change:
```go
if p.Permission == PermTenantAdmin && strings.HasPrefix(required, "tenant:") {
    return true
}
```

To:
```go
if p.Permission == PermTenantAdmin {
    return true
}
```

### Step 2: Verify All Handlers

Confirm that no handler relies on the current broken behavior as a security boundary. Key handlers to verify:

| Handler | Current Guard | After Fix |
|---------|--------------|-----------|
| `HandleImportSingleFile` | `isAdmin \|\| hasPermission(PermFilesUpload)` | `tenant:admin` users gain `files:upload` ✓ |
| `HandleRestoreFileVersion` | `isAdmin \|\| hasPermission(PermFilesRestore)` | `tenant:admin` users gain `files:restore` ✓ |
| `HandleChatInternal` | `isAdmin \|\| hasPermission(PermChatTest)` | `tenant:admin` users gain `chat:test` ✓ |
| `HandleListSessions` | `isAdmin \|\| hasPermission(PermChatLogs)` | `tenant:admin` users gain `chat:logs` ✓ |
| `HandleSetLLMConfig` | `isAdmin \|\| hasPermission(PermAPIConfig)` | `tenant:admin` users gain `api:config` ✓ |
| `HandleReindexTenant` | `isAdmin \|\| hasPermission(PermAPIConfig)` | `tenant:admin` users gain reindex access ✓ |
| `HandleSetActiveVersion` | `isAdmin \|\| hasPermission(PermAPIConfig)` | `tenant:admin` users gain rollback access ✓ |

Admin-only endpoints (protected by `isAdmin()` only, no `hasPermission()` fallback):
- `HandleListTenants` — not affected
- `HandleCreateTenant` — not affected
- `HandleDeleteTenant` — not affected

### Step 3: Test Scenarios

1. Create a USER with `tenant:admin` permission
2. Verify they can: upload files, restore versions, test chat, view logs, configure LLM, reindex, rollback RAG
3. Verify they CANNOT: create tenants, delete tenants, list all tenants, export tenants
4. Verify HOST/ADMIN roles are unchanged

---

## 7. Fix Plan: RAG Rollback UI (10 Steps)

### Step 1: Write Versioning Documentation

**File:** `bugs/038/docs_lancedb_versioning.md`

Document the 4-layer version tracking system, upload/reindex flows, and version rollback mechanism. ✓ Complete.

### Step 2: Add Per-Tenant Mutex for Reindex/Rollback Serialization

**File:** `server/router/api/v1/agent/service.go`

Add a `sync.Map` keyed by `tenantID` holding `*sync.Mutex` values to the `Service` struct:

```go
type Service struct {
    // ... existing fields ...
    reindexMu sync.Map // map[int32]*sync.Mutex
}
```

Add a helper to acquire/release:

```go
func (s *Service) getTenantMutex(tenantID int32) *sync.Mutex {
    val, _ := s.reindexMu.LoadOrStore(tenantID, &sync.Mutex{})
    return val.(*sync.Mutex)
}
```

Acquire in:
- `reindexFileVersion` (`service.go:295`) — lock at start, unlock at end (via defer)
- `HandleSetActiveVersion` (`handlers.go:5996`) — lock before `UpsertAgentRAGActiveVersion`, unlock after

This serializes reindex and rollback per tenant. Different tenants can proceed in parallel.

### Step 3: Add `PermTenantAdmin` to Backend Guards (Defense-in-Depth)

**File:** `server/router/api/v1/agent/handlers.go`

Update 4 handlers to explicitly accept `tenant:admin` (redundant after Bug #1 fix, but makes intent clear):

```go
// HandleSetActiveVersion (line 6004)
if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermAPIConfig) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin, api:config, or tenant:admin")
}

// HandleListActiveVersions (line 6071)
// Same change

// HandleListIndexedVersions (line 6107)
// Same change

// HandleReindexTenant (line 1175)
if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermAPIConfig) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin, api:config, or tenant:admin")
}
```

**Note:** After Bug #1 fix, `tenant:admin` already grants `api:config` via the updated expansion logic. The explicit check is defense-in-depth and makes the permission intent self-documenting.

### Step 4: Add `canRollbackVersion` Permission Check

**File:** `web/src/pages/AgentAdmin.tsx:~183`

Add after `canConfigApi`:
```tsx
const canRollbackVersion = isAdmin || canConfigApi || agentAdminStore.hasPermission("tenant:admin");
```

### Step 5: Add Store Methods

**File:** `web/src/store/v2/agentAdmin.ts`

Add state fields:
```typescript
activeVersions: Array<{audience_type: string, file_type: string, version: number}> = [];
indexedVersions: Array<{audience_type: string, file_type: string, versions: number[]}> = [];
```

Add 3 methods:

```typescript
const fetchActiveVersions = async (slug: string) => {
  try {
    const response = await axios.get<{ activeVersions: Array<{audience_type: string, file_type: string, version: number}> }>(
      `/api/v1/agent/${slug}/rag/active-versions`
    );
    runInAction(() => {
      state.activeVersions = response.data.activeVersions || [];
    });
  } catch (error: any) {
    console.error("Failed to fetch active versions:", error);
  }
};

const fetchIndexedVersions = async (slug: string) => {
  try {
    const response = await axios.get<{ groups: Array<{audience_type: string, file_type: string, versions: number[]}> }>(
      `/api/v1/agent/${slug}/rag/indexed-versions`
    );
    runInAction(() => {
      state.indexedVersions = response.data.groups || [];
    });
  } catch (error: any) {
    console.error("Failed to fetch indexed versions:", error);
  }
};

const setActiveVersion = async (slug: string, audienceType: string, fileType: string, version: number): Promise<boolean> => {
  state.setPartial({ isSaving: true, error: null });
  try {
    await axios.post(`/api/v1/agent/${slug}/rag/active-version`, {
      audience_type: audienceType,
      file_type: fileType,
      version: version,
    });
    runInAction(() => { state.isSaving = false; });
    await fetchActiveVersions(slug);
    return true;
  } catch (error: any) {
    runInAction(() => {
      state.isSaving = false;
      state.error = error.response?.data?.message || "Failed to set active version";
    });
    return false;
  }
};
```

Export in return object.

### Step 6: Add Collapsible Rollback Panel

**File:** `web/src/pages/AgentAdmin.tsx:~1216` (inside Rebuild Index section)

Add after the reindex progress/status area (after line 1333):

- State: `showRollbackPanel` boolean (default false)
- On tenant select + RAG enabled: fetch active and indexed versions
- Render collapsible section with heading "Version Rollback"
- For each (audience, file_type) group:
  - Show current active version badge
  - Show list of indexed versions as chips/radio
  - "Rollback" button calls `setActiveVersion`
  - Toast on success, refresh active versions

### Step 7: Add Translations

**File:** `web/src/locales/en.json`

```json
{
  "agent-admin": {
    "version-rollback": "Version Rollback",
    "version-rollback-desc": "Switch the active RAG index version without re-embedding. Old version data remains in LanceDB until retention cleanup.",
    "active-version": "Active",
    "rollback-to": "Rollback to v{{version}}",
    "rollback-success": "Rolled back to version {{version}}",
    "no-indexed-versions": "No indexed versions available",
    "indexed-versions": "Indexed Versions"
  }
}
```

### Step 8: Run Lint + Typecheck

```bash
# Backend
go vet ./...
go build ./...

# Frontend
cd web && npm run lint
cd web && npx tsc --noEmit
```

### Step 9: Manual Test Scenarios

| Role | Expected Behavior |
|------|-------------------|
| HOST | Sees rollback panel, can rollback any tenant |
| ADMIN (global) | Sees rollback panel, can rollback any tenant |
| ADMIN (scoped) | Sees rollback panel for scoped tenants only |
| USER + `tenant:admin` | Sees rollback panel, can rollback (after Bug #1 fix) |
| USER + `api:config` | Sees rollback panel, can rollback |
| USER (no perms) | Rollback panel not visible |

### Step 10: Verify Race Condition Fix

1. Start a reindex (long-running, use large content)
2. While reindex is running, trigger rollback
3. Verify: rollback succeeds and is NOT overwritten by reindex goroutine
4. Verify: rollback blocks until reindex releases the mutex (or vice versa)

---

## 8. Future Improvements (Out of Scope)

| Improvement | Description |
|-------------|-------------|
| `resolveQueryVersion` fallback logging | Log a warning when falling through to LanceDB fallback (latest indexed) instead of using the active pointer |
| Retention window configurability | Allow admins to configure max retained versions (currently hardcoded to 5) |
| Stale pointer cleanup | Auto-delete active-version pointer entries that point to pruned versions |
| Rollback audit trail | Log who rolled back to which version and when |

---

## 9. Execution Order

```
1.  Write docs_lancedb_versioning.md (companion documentation)
2.  Add per-tenant mutex in Service struct (reindex/rollback serialization)
3.  Fix permissions.go:93 (tenant:admin expansion — one-line change)
4.  Add PermTenantAdmin to 4 handler guards (3 rollback + reindex) with updated error messages
5.  Add store methods + state in agentAdmin.ts
6.  Add canRollbackVersion + rollback panel in AgentAdmin.tsx
7.  Add translations in en.json
8.  Run lint + typecheck
9.  Manual test with HOST, ADMIN, tenant-admin, and USER roles
10. Verify race condition fix
```

---

## 10. Files Changed

| File | Change | Bug |
|------|--------|-----|
| `bugs/038/docs_lancedb_versioning.md` | New — versioning architecture documentation | — |
| `server/router/api/v1/agent/service.go` | New — per-tenant mutex (`sync.Map` of `sync.Mutex`) | Race condition |
| `server/router/api/v1/agent/permissions.go:93` | Fix — unconditional `tenant:admin` grant | Bug #1 |
| `server/router/api/v1/agent/handlers.go:1175` | Fix — add `PermTenantAdmin` guard + error msg | Bug #2 |
| `server/router/api/v1/agent/handlers.go:6004` | Fix — add `PermTenantAdmin` guard + error msg | Bug #2 |
| `server/router/api/v1/agent/handlers.go:6071` | Fix — add `PermTenantAdmin` guard + error msg | Bug #2 |
| `server/router/api/v1/agent/handlers.go:6107` | Fix — add `PermTenantAdmin` guard + error msg | Bug #2 |
| `web/src/store/v2/agentAdmin.ts` | Feature — add 3 store methods + state fields | Bug #2 |
| `web/src/pages/AgentAdmin.tsx` | Feature — add `canRollbackVersion` + rollback panel | Bug #2 |
| `web/src/locales/en.json` | Feature — add translation keys | Bug #2 |
