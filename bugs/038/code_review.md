# Code Review: Bug #038 — Implementation

**Reviewer:** Senior Go Architect  
**Status:** Approved with Nits  
**Date:** 2026-07-15

---

## Verdict: APPROVED WITH NITS

The implementation is correct and addresses all three bugs plus the race condition. Below are issues found during review.

---

## Issues

### 1. Dead Function `ContainsPermission` Has Stale Logic (High)

**File:** `server/router/api/v1/agent/permissions.go:63-80`

`ContainsPermission` (exported, lines 64-80) still has the old `strings.HasPrefix(required, "tenant:")` check. The fix was correctly applied only to `containsResolvedPermission` (unexported, line 94). Since `ContainsPermission` is never called anywhere in the codebase, this is not a security issue — but it's dead code that will confuse future readers and could reintroduce the bug if someone copies from it.

**Recommendation:** Update or remove `ContainsPermission` to match the fixed logic. At minimum, add a comment: `// Deprecated: use containsResolvedPermission`.

### 2. Mutex Scope in `reindexFileVersion` Blocks Rollback for Minutes (Medium)

**File:** `server/router/api/v1/agent/service.go:310-313`

The per-tenant mutex is acquired at the start of `reindexFileVersion` and held for the entire function body, including CPU-bound chunking and I/O-bound LanceDB Insert. A reindex of a large tenant can take minutes; during that time, any rollback for the same tenant will block (waiting on `mu.Lock()`) with no feedback to the user.

This is architecturally correct (serialization is the goal), but the UX is poor: the rollback HTTP request will appear to hang with no progress indicator.

**Recommendation:** Accept as-is, but document in the handler that rollback may block. Consider adding a server-sent timeout (e.g., 5s timeout on the mutex with a `"Retry later, reindex in progress"` response) in a follow-up.

### 3. Frontend Fetch Functions Swallow Errors (Low)

**File:** `web/src/store/v2/agentAdmin.ts:1702-1713, 1715-1726`

```typescript
const fetchActiveVersions = async (slug: string) => {
    try {
      // ...
    } catch (error: any) {
      console.error("Failed to fetch active versions:", error); // No error state set
    }
  };
```

`fetchActiveVersions` and `fetchIndexedVersions` only log to console on failure — they don't set `state.error`. If the API call fails (network error, 500), the UI will silently show stale data or an empty state, and the user gets no feedback.

**Recommendation:** Set `state.error` in the catch block:
```typescript
runInAction(() => {
  state.error = "Failed to fetch active versions";
});
```

### 4. Global `isSaving` State Conflict (Low)

**File:** `web/src/store/v2/agentAdmin.ts:1729`

`setActiveVersion` sets `state.isSaving = true` globally. If another operation (e.g., file upload, LLM config save) is in progress simultaneously, the `isSaving` state will be overwritten or prematurely cleared.

**Recommendation:** Accept as-is. The existing codebase uses `isSaving` globally for other operations too (pre-existing pattern). If this causes issues, a future refactor should use per-operation saving states.

### 5. No Loading State for Rollback Panel (Low)

**File:** `web/src/pages/AgentAdmin.tsx:286-291`

```typescript
useEffect(() => {
    if (showRollbackPanel && selectedTenant) {
      agentAdminStore.fetchActiveVersions(...);
      agentAdminStore.fetchIndexedVersions(...);
    }
  }, [showRollbackPanel, selectedTenant?.tenant.slug]);
```

When the panel opens, both fetch calls fire simultaneously. There's no loading indicator — the panel renders immediately, potentially showing stale/empty data until the API responds.

**Recommendation:** Accept as-is for MVP. Add loading state in a follow-up.

### 6. Stale Active Version Pointer Not Handled in UI (Low)

If the active version pointer in SQLite references a version that was pruned from LanceDB by retention, `resolveQueryVersion` falls back to the latest indexed version (service.go:4410-4418). However, the UI will still show the stale pointer from `GET /rag/active-versions` (which reads from SQLite). This creates a mismatch between what the UI reports and what the agent actually queries.

**Recommendation:** Accept as-is. Document in Future Improvements that the backend should cross-validate active versions against LanceDB on read.

---

## Summary

| # | Severity | Issue | Fix Required? |
|---|----------|-------|---------------|
| 1 | High | Dead `ContainsPermission` has stale logic | Optional — update or add deprecation comment |
| 2 | Medium | Mutex blocks rollback during reindex | Document as known UX tradeoff |
| 3 | Low | Frontend fetch errors swallowed | Optional — add error state |
| 4 | Low | Global `isSaving` conflict | Accept (pre-existing pattern) |
| 5 | Low | No loading state in rollback panel | Accept for MVP |
| 6 | Low | Stale active version pointer not validated | Accept; future improvement |

**Security:** No privilege escalation found. Tenant isolation holds. The `PermTenantAdmin` expansion is correct and scoped correctly.
