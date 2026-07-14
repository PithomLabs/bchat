# Bug #038 — Implementation Record

**Date:** 2026-07-14
**Status:** Implemented, ready for adversarial review

---

## Summary

| Problem | Fix |
|---------|-----|
| Reindex and rollback race condition: concurrent reindex goroutine overwrites rollback pointer | Per-tenant mutex via `sync.Map` serializes reindex + rollback per tenant |
| `tenant:admin` permission only matched `tenant:*` prefix, not `api:*`, `files:*`, `chat:*` | `permissions.go:93` unconditional grant — tenant:admin is a superset of all tenant permissions |
| 4 rollback-related handlers missing `tenant:admin` as an authorized permission | Added `PermTenantAdmin` to guard checks with consistent error messages |
| No frontend UI for version rollback | Collapsible Version Rollback panel with clickable version buttons |

---

## Files Changed

### Backend

| File | Change |
|------|--------|
| `server/router/api/v1/agent/service.go` | Per-tenant mutex field + helper + lock in reindex |
| `server/router/api/v1/agent/permissions.go` | tenant:admin unconditional expansion |
| `server/router/api/v1/agent/handlers.go` | 4 handler guard updates + mutex in HandleSetActiveVersion |

### Frontend

| File | Change |
|------|--------|
| `web/src/store/v2/agentAdmin.ts` | 3 store methods + 2 state fields |
| `web/src/pages/AgentAdmin.tsx` | Permission check + collapsible rollback panel |
| `web/src/locales/en.json` | 6 translation keys |

---

## Detailed Changes

### 1. `server/router/api/v1/agent/service.go`

**Per-tenant mutex field (line 63):**
```go
type Service struct {
    // ... existing fields ...
    vectorDBMu          sync.RWMutex  // Protects vectorDB access
    reindexMu           sync.Map      // per-tenant mutex for reindex/rollback serialization
    observerBuffer      *ObserverBuffer
}
```

**Mutex helper (lines 188-195):**
```go
// getTenantMutex returns a per-tenant mutex for serializing reindex and rollback
// operations. Different tenants can proceed in parallel; operations on the same
// tenant are serialized to prevent the rollback pointer from being overwritten
// by a concurrent reindex goroutine.
func (s *Service) getTenantMutex(tenantID int32) *sync.Mutex {
    val, _ := s.reindexMu.LoadOrStore(tenantID, &sync.Mutex{})
    return val.(*sync.Mutex)
}
```

**Lock in reindexFileVersion (lines 310-313):**
```go
func (s *Service) reindexFileVersion(ctx context.Context, tenantID int32, ...) (int, error) {
    // ... content check ...

    // Serialize reindex and rollback per tenant to prevent rollback pointer overwrite.
    mu := s.getTenantMutex(tenantID)
    mu.Lock()
    defer mu.Unlock()
    chunks := s.chunker.ChunkMarkdownContent(...)
```

### 2. `server/router/api/v1/agent/permissions.go` (line 93-96)

**Before:**
```go
if p.Permission == PermTenantAdmin && strings.HasPrefix(required, "tenant:") {
    return true
}
```

**After:**
```go
// tenant:admin grants all permissions for this tenant (superset of tenant:*, chat:*, files:*, api:*).
if p.Permission == PermTenantAdmin {
    return true
}
```

### 3. `server/router/api/v1/agent/handlers.go`

**HandleReindexTenant (line 1175):**
```go
if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermAPIConfig) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin, api:config, or tenant:admin")
}
```

**HandleSetActiveVersion (lines 6005 + 6028-6030):**
```go
// Permission guard (line 6005):
if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermAPIConfig) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin, api:config, or tenant:admin")
}

// Mutex lock (lines 6028-6030):
mu := h.service.getTenantMutex(tenant.ID)
mu.Lock()
defer mu.Unlock()
```

**HandleListActiveVersions (line 6078):**
```go
if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermAPIConfig) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin, api:config, or tenant:admin")
}
```

**HandleListIndexedVersions (line 6115):**
```go
if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermAPIConfig) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin, api:config, or tenant:admin")
}
```

### 4. `web/src/store/v2/agentAdmin.ts`

**State fields (lines 532-535):**
```typescript
// RAG Version Rollback
activeVersions: Array<{ audience_type: string; file_type: string; version: number }> = [];
indexedVersions: Array<{ audience_type: string; file_type: string; versions: number[] }> = [];
```

**Store methods (lines 1698-1746):**
```typescript
const fetchActiveVersions = async (slug: string) => {
    try {
      const response = await axios.get<{ activeVersions: Array<{ audience_type: string; file_type: string; version: number }> }>(
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
      const response = await axios.get<{ groups: Array<{ audience_type: string; file_type: string; versions: number[] }> }>(
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

### 5. `web/src/pages/AgentAdmin.tsx`

**Permission check (line 183):**
```typescript
const canRollbackVersion = isAdmin || canConfigApi || agentAdminStore.hasPermission("tenant:admin");
```

**State (line 140):**
```typescript
const [showRollbackPanel, setShowRollbackPanel] = useState(false);
```

**Destructured store fields (lines 168-169):**
```typescript
activeVersions,
indexedVersions,
```

**Fetch useEffect (lines 283-288):**
```typescript
useEffect(() => {
    if (showRollbackPanel && selectedTenant) {
      agentAdminStore.fetchActiveVersions(selectedTenant.tenant.slug);
      agentAdminStore.fetchIndexedVersions(selectedTenant.tenant.slug);
    }
  }, [showRollbackPanel, selectedTenant?.tenant.slug]);
```

**Rollback panel JSX (inserted between Rebuild Index section and Widget Embed Code section):**
- Collapsible header with `HistoryIcon` + `ChevronDownIcon`/`ChevronUpIcon`
- Active Versions panel showing `audience_type` + `file_type` badges and version number
- Indexed Versions panel with clickable version buttons per audience/file group
- Active version highlighted in green with checkmark; others are clickable to switch
- Buttons disabled while active version (prevents redundant calls)
- `setActiveVersion` called on click, re-fetches active versions on success

### 6. `web/src/locales/en.json`

```json
"version-rollback-title": "Version Rollback",
"version-rollback-desc": "Switch active versions without reindexing. Changes are instant.",
"active-versions": "Active Versions",
"indexed-versions": "Indexed Versions",
"no-active-versions": "No active versions set. Run a Rebuild Index first.",
"no-indexed-versions": "No indexed versions found."
```

---

## Verification

| Check | Result |
|-------|--------|
| `go vet ./server/...` | Clean |
| `go build ./server/router/api/v1/agent/` | Clean |
| `npx tsc --noEmit` | Pre-existing node_modules type errors only |

---

## Adversarial Code Review Prompt

Copy-paste the prompt below to another LLM for a thorough review:

```
You are performing an adversarial code review of Bug #038: RAG Version Rollback UI + tenant:admin RBAC fix.

Review the following diff for security vulnerabilities, race conditions, logic errors, and UX issues. Be aggressive — find every possible problem.

DIFF SUMMARY:
- service.go: Added `reindexMu sync.Map` field to Service struct, `getTenantMutex()` helper, and mutex lock+defer unlock in `reindexFileVersion()`.
- permissions.go: Changed line 93 from `if p.Permission == PermTenantAdmin && strings.HasPrefix(required, "tenant:")` to unconditional `if p.Permission == PermTenantAdmin`.
- handlers.go: Added `PermTenantAdmin` to 4 handler guards (`HandleReindexTenant`, `HandleSetActiveVersion`, `HandleListActiveVersions`, `HandleListIndexedVersions`). Added mutex lock+defer unlock in `HandleSetActiveVersion`.
- agentAdmin.ts: Added `activeVersions` and `indexedVersions` state fields. Added `fetchActiveVersions`, `fetchIndexedVersions`, `setActiveVersion` store methods.
- AgentAdmin.tsx: Added `canRollbackVersion` check. Added `showRollbackPanel` state. Added fetch useEffect. Added collapsible rollback panel with version buttons.
- en.json: Added 6 translation keys.

CHECK THESE SPECIFIC AREAS:

1. RACE CONDITIONS:
   - Is the per-tenant mutex correctly scoped? Can a reindex goroutine still overwrite the rollback pointer?
   - Is `sync.Map` the right choice here vs a `map[int32]*sync.Mutex` protected by a RWMutex?
   - Does `defer mu.Unlock()` in `reindexFileVersion` correctly release on all code paths including panics?
   - In `HandleSetActiveVersion`, the mutex is acquired AFTER the permission check but BEFORE the version validation + upsert. Is this correct? Could a TOCTOU issue exist between the permission check and the lock acquisition?

2. PERMISSION EXPANSION:
   - Was the old `strings.HasPrefix(required, "tenant:")` check a security issue? Did it mean `tenant:admin` did NOT grant `api:config`, `files:upload`, `chat:test`?
   - After the fix, does `tenant:admin` now grant ALL permissions? Is this the intended behavior per AGENTS.md ("Tenant Admin (full access)" preset)?
   - Could this be a privilege escalation? A user with ONLY `tenant:admin` can now do everything — is that documented and intended?
   - Are there any handlers that check for specific permissions but should NOT be accessible to `tenant:admin`?

3. FRONTEND STATE MANAGEMENT:
   - `fetchActiveVersions` and `fetchIndexedVersions` swallow errors silently (`console.error`). Should they set error state?
   - `setActiveVersion` sets `isSaving` globally. Could this interfere with other save operations?
   - After `setActiveVersion` succeeds, it re-fetches `activeVersions` but not `indexedVersions`. Is this correct?
   - The `showRollbackPanel` state is local. If the user switches tenants, does the panel state persist incorrectly?

4. UI/UX:
   - Version buttons have `disabled={v === activeVersion?.version}` — good for preventing redundant calls, but does the active version still appear clickable (just visually disabled)?
   - The panel auto-fetches on open. What happens if the API is slow? Is there a loading indicator?
   - What happens if there are 0 active versions and 0 indexed versions? Is the empty state clear?
   - Are the version buttons keyboard-navigable? Do they have proper aria labels?

5. EDGE CASES:
   - What if a tenant has NO indexed versions at all? Does the rollback panel show a useful message?
   - What if the active version points to a version that was subsequently deleted from LanceDB? Does the backend handle this gracefully?
   - Can a user set an active version that doesn't exist? The backend validates this, but what about the frontend — does it prevent the UI from showing non-existent versions?
   - What happens if two users try to set different active versions simultaneously? The mutex serializes them, but does the second user see stale data?

6. BACKEND LOGIC:
   - In `HandleSetActiveVersion`, the mutex is held while calling `ListIndexedVersions` and `UpsertAgentRAGActiveVersion`. Could this cause contention if one of these operations is slow?
   - The `UpsertAgentRAGActiveVersion` is an upsert — does it handle the case where the same (audience, file_type) already has an active version?
   - Is the mutex in `HandleSetActiveVersion` redundant with the mutex in `reindexFileVersion`? Or do they protect different critical sections?

Return your findings as a numbered list with severity (Critical/High/Medium/Low) and recommended fix for each.
```
