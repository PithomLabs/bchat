# Bug #038: RAG Version Rollback UI + tenant:admin RBAC Fix

**Status:** Plan (awaiting approval)
**Created:** 2026-07-15

---

## 1. Background: LanceDB Versioning Architecture

### Overview

The RAG (Retrieval-Augmented Generation) system uses LanceDB as its vector database. Each tenant's vectors are stored in an isolated LanceDB instance — either a local directory (`build/data/lancedb/<tenantID>/`) or an S3 prefix (`s3://bucket/lancedb/<tenantID>/`).

### Four-Layer Version Tracking

The system tracks RAG content versions across four layers:

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
2. Reindex reads `ListAgentSourceFiles(LatestOnly: true)` → gets version N
3. Chunks are created with `source_version = N` embedded in each row
4. New versioned chunks are **appended** to the existing LanceDB table (old versions coexist)
5. `UpsertAgentRAGActiveVersion` sets the pointer to version N
6. Retention policy: if more than 5 indexed versions exist, oldest ones are deleted via `DeleteByVersion`

### Key Files

| File | Purpose |
|------|---------|
| `server/router/api/v1/agent/vectordb.go:237-253` | Per-tenant storage path resolution (S3 and local) |
| `server/router/api/v1/agent/vectordb_pool.go:15-98` | TenantVectorDBPool — lazy per-tenant LanceDB instances |
| `server/router/api/v1/agent/vectordb_lance.go:29-32` | Table naming by dimension (`kb_documents_<dim>`) |
| `server/router/api/v1/agent/vectordb_lance.go:1167-1169` | Query filtering by `tenant_id` |
| `server/router/api/v1/agent/service.go:291-337` | `reindexFileVersion` — append-only versioned indexing |
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

| Permission | Description | Auto-implies |
|------------|-------------|--------------|
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

### Permission Expansion

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

| Endpoint | Permission Required |
|----------|---------------------|
| `GET /:slug/validate` | ADMIN or `chat:test` |
| `GET /:slug/config` | ADMIN or `tenant:read` |
| `PATCH /:slug` | ADMIN or `tenant:write` |
| `POST /:slug/chat/int` | ADMIN or `chat:test` |
| `POST /:slug/import` | ADMIN or `files:upload` |
| `POST /:slug/files/.../restore` | ADMIN or `files:restore` |
| `GET /:slug/files/.../versions` | ADMIN or `files:restore` or `tenant:read` |
| `PUT /:slug/llm-config` | ADMIN or `api:config` |
| `GET /:slug/sessions` | ADMIN or `chat:logs` |
| `GET/POST /:slug/permissions` | ADMIN or `tenant:admin` |
| `GET /:slug/rag/active-versions` | ADMIN or `api:config` |
| `GET /:slug/rag/indexed-versions` | ADMIN or `api:config` |
| `POST /:slug/rag/active-version` | ADMIN or `api:config` |
| `GET /agent/tenants` | ADMIN only |
| `POST /agent/onboard` | ADMIN only |
| `DELETE /:slug` | ADMIN only |

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

This is the **intended behavior** — the label says "full access" and it should deliver it.

### Security Verification Required

The following admin-only endpoints remain protected by `isAdmin()` (not `hasPermission()`), so they are NOT affected:
- `GET /agent/tenants` — list all tenants
- `POST /agent/onboard` — create new tenant
- `DELETE /:slug` — delete tenant
- `GET /:slug/export` — export tenant data

These are correctly tenant-management operations that a tenant-admin should not perform.

---

## 4. Bug #2: RAG Rollback Has No Frontend UI

### Backend (Complete)

Three endpoints exist and are fully functional:

| Endpoint | Handler | Purpose |
|----------|---------|---------|
| `GET /:slug/rag/active-versions` | `HandleListActiveVersions` (line 6061) | List current active version per (audience, file_type) |
| `GET /:slug/rag/indexed-versions` | `HandleListIndexedVersions` (line 6097) | List all versions present in LanceDB |
| `POST /:slug/rag/active-version` | `HandleSetActiveVersion` (line 5994) | Point active version to any indexed version (instant rollback) |

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

## 5. Fix Plan: tenant:admin Expansion

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
| `HandleSetActiveVersion` | `isAdmin \|\| hasPermission(PermAPIConfig)` | `tenant:admin` users gain rollback access ✓ |

Admin-only endpoints (protected by `isAdmin()` only, no `hasPermission()` fallback):
- `HandleListTenants` — not affected
- `HandleCreateTenant` — not affected
- `HandleDeleteTenant` — not affected

### Step 3: Test Scenarios

1. Create a USER with `tenant:admin` permission
2. Verify they can: upload files, restore versions, test chat, view logs, configure LLM, rollback RAG
3. Verify they CANNOT: create tenants, delete tenants, list all tenants, export tenants
4. Verify HOST/ADMIN roles are unchanged

---

## 6. Fix Plan: RAG Rollback UI (8 Steps)

### Step 1: Write Versioning Documentation

**File:** `docs/bugs/038/docs_lancedb_versioning.md`

Document the 4-layer version tracking system, upload/reindex flows, and version rollback mechanism.

### Step 2: Add `PermTenantAdmin` to Backend Guards

**File:** `server/router/api/v1/agent/handlers.go`

Update 3 handlers to accept `tenant:admin`:

```go
// HandleSetActiveVersion (line 6004)
if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermAPIConfig) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
    return echo.NewHTTPError(http.StatusForbidden, "Admin, api:config, or tenant:admin permission required")
}

// HandleListActiveVersions (line 6071)
// Same change

// HandleListIndexedVersions (line 6107)
// Same change
```

**Note:** After Step 5 (Bug #1 fix), `tenant:admin` already grants `api:config` via the updated expansion logic. So the `PermTenantAdmin` check is technically redundant after Bug #1 fix. But adding it explicitly makes the intent clear and provides defense-in-depth.

### Step 3: Add `canRollbackVersion` Permission Check

**File:** `web/src/pages/AgentAdmin.tsx:~183`

Add after `canConfigApi`:
```tsx
const canRollbackVersion = isAdmin || canConfigApi || agentAdminStore.hasPermission("tenant:admin");
```

### Step 4: Add Store Methods

**File:** `web/src/store/v2/agentAdmin.ts`

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

Add state fields:
```typescript
activeVersions: Array<{audience_type: string, file_type: string, version: number}> = [];
indexedVersions: Array<{audience_type: string, file_type: string, versions: number[]}> = [];
```

Export in return object.

### Step 5: Add Collapsible Rollback Panel

**File:** `web/src/pages/AgentAdmin.tsx:~1216` (inside Rebuild Index section)

Add after the reindex progress/status area (after line 1333):

- State: `showRollbackPanel` boolean (default false)
- On tenant select + RAG enabled: fetch active and indexed versions
- Render collapsible section with heading "Version Rollback"
- For each (audience, file_type) group:
  - Show current active version
  - Show list of indexed versions as chips/radio
  - "Rollback" button calls `setActiveVersion`
  - Toast on success, refresh active versions

### Step 6: Add Translations

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

---

## 7. Adversarial Review Prompt

Use this prompt to have a reviewing agent perform a security and correctness audit of the plan:

---

**PROMPT:**

You are a senior security architect reviewing a plan for two changes to the bchat RBAC and RAG systems. Perform a thorough adversarial review.

## Change 1: tenant:admin Expansion

**Current code** (`permissions.go:93`):
```go
if p.Permission == PermTenantAdmin && strings.HasPrefix(required, "tenant:") {
    return true
}
```

**Proposed change:**
```go
if p.Permission == PermTenantAdmin {
    return true
}
```

This means `tenant:admin` grants ALL permissions for a tenant (not just `tenant:*`).

## Change 2: RAG Rollback UI

Add frontend UI for 3 existing backend endpoints that allow instant rollback of the RAG index to a prior version without re-embedding. The rollback simply flips a pointer in SQLite (`agent_rag_active_versions` table).

## Your Review Must Cover:

### 2.1 Permission Escalation
- Does expanding `tenant:admin` to grant all permissions create unintended access to admin-only endpoints?
- Are there any endpoints that check `isAdmin()` but should also check `hasPermission()`?
- Could a `tenant:admin` user escalate to HOST-level access?

### 2.2 Tenant Isolation
- Can a scoped admin (ADMIN with `AllowedTenantIDs`) use the rollback endpoint to affect tenants outside their scope?
- Does the tenant-binding middleware (`tenant_binding.go`) correctly intercept cross-tenant requests?
- Is there a risk that the `tenant_id` from the JWT could be manipulated to target a different tenant's RAG index?

### 2.3 Race Conditions
- What happens if rollback is called during an active reindex?
- Can a concurrent chat query read a half-flipped active-version pointer?
- Is the active-version pointer update atomic?

### 2.4 Data Integrity
- Can rollback fail if the target version was already pruned (beyond the 5-version retention window)?
- Is the backend validation (`ListIndexedVersions` check in `HandleSetActiveVersion`) sufficient to prevent rollback to a non-existent version?
- What happens if the LanceDB table is corrupted or missing?

### 2.5 Frontend Security
- Is the `canRollbackVersion` frontend check purely cosmetic, or does the backend independently enforce the permission?
- Could a malicious user bypass the frontend and call the rollback API directly with a `tenant:admin` JWT?

### 2.6 Consistency
- List all endpoints that check `isAdmin || PermAPIConfig` but do NOT check `PermTenantAdmin`. Are there others that should be updated?
- After the `tenant:admin` expansion fix, is the explicit `PermTenantAdmin` check in the rollback handler redundant? Is redundancy acceptable for defense-in-depth?

Return your findings as a numbered list with severity (Critical/High/Medium/Low) and recommended mitigations.

---

## 8. Execution Order

1. Write `docs_lancedb_versioning.md` (companion documentation)
2. Fix `permissions.go:93` (tenant:admin expansion)
3. Verify handler security (manual review + test)
4. Add `PermTenantAdmin` to 3 rollback handler guards (defense-in-depth)
5. Add store methods in `agentAdmin.ts`
6. Add `canRollbackVersion` check in `AgentAdmin.tsx`
7. Add collapsible rollback panel UI in `AgentAdmin.tsx`
8. Add translations in `en.json`
9. Run lint + typecheck
10. Manual test with HOST, ADMIN, tenant-admin, and USER roles

---

## 9. Files Changed

| File | Change |
|------|--------|
| `docs/bugs/038/docs_lancedb_versioning.md` | New — versioning architecture documentation |
| `server/router/api/v1/agent/permissions.go:93` | Fix — unconditional `tenant:admin` grant |
| `server/router/api/v1/agent/handlers.go:6004` | Fix — add `PermTenantAdmin` guard |
| `server/router/api/v1/agent/handlers.go:6071` | Fix — add `PermTenantAdmin` guard |
| `server/router/api/v1/agent/handlers.go:6107` | Fix — add `PermTenantAdmin` guard |
| `web/src/store/v2/agentAdmin.ts` | Feature — add 3 store methods + state fields |
| `web/src/pages/AgentAdmin.tsx` | Feature — add `canRollbackVersion` + rollback panel |
| `web/src/locales/en.json` | Feature — add translation keys |
