# Plan Review: Bug #038 — RAG Version Rollback UI + tenant:admin RBAC Fix

**Reviewer:** Senior Go Architect  
**Status:** Approved with Nits  
**Date:** 2026-07-15

---

## Verdict: APPROVED WITH NITS

The plan is sound and the two bugs are correctly identified. The proposed fixes are correct in intent. Below are issues that should be addressed before execution.

---

## Critical

### 1. Race Condition: Rollback During Active Reindex

**Severity:** Critical  
**File:** `handlers.go:1231`, `service.go:317`

`HandleReindexTenant` spawns a background goroutine (handlers.go:1231) that calls `reindexFileVersion` → `UpsertAgentRAGActiveVersion` (service.go:317). If a rollback (`HandleSetActiveVersion`) is called concurrently, the reindex goroutine will overwrite the active version pointer back to the newly indexed version, silently undoing the rollback.

There is no mutex, no compare-and-swap, and no advisory lock anywhere in the reindex or rollback path. The `UpsertAgentRAGActiveVersion` uses `ON CONFLICT DO UPDATE` which is atomic at the row level, but the two callers race to set different values.

**Mitigation:** Add a per-tenant mutex (`sync.Map` keyed by `tenantID`) in the service layer, acquired by both `HandleSetActiveVersion` and `reindexFileVersion`. Alternatively, use a database advisory lock (Postgres `pg_advisory_xact_lock`, SQLite `BEGIN IMMEDIATE`).

---

## 2. Redundant `PermTenantAdmin` Guard (High)

**Severity:** High  
**File:** `handlers.go:6004,6071,6107` (plan Step 2)

The plan adds explicit `PermTenantAdmin` checks to 3 handlers. After Bug #1 fix, `tenant:admin` already grants `api:config` via the unconditional return in `containsResolvedPermission`. The explicit check is defense-in-depth but adds no behavioral change.

**Recommendation:** Accept as-is, but update the error messages to mention `tenant:admin` if the explicit check is kept. Also consider adding the same explicit check to `HandleReindexTenant` (handlers.go:1175) for consistency — it uses the same `isAdmin() || hasPermission(PermAPIConfig)` pattern.

---

## 3. Route Group Inaccuracy (Medium)

**Severity:** Medium

The plan's permission table (section 4) and Step 2 imply the RAG rollback endpoints are in the `authGroup`. They are actually registered in the `adminGroup` (v1.go:371-375), which applies `TenantBindingMiddleware` + `CSRFProtectionMiddleware` in addition to `AuthMiddleware`. This is *more* restrictive than the plan suggests, so it's a safe inaccuracy, but the plan should be corrected for accuracy.

---

## 4. Missing `HandleReindexTenant` in Step 2 (Medium)

**Severity:** Medium  
**File:** `handlers.go:1175`

`HandleReindexTenant` also uses `isAdmin() || hasPermission(PermAPIConfig)`. After Bug #1 fix, `tenant:admin` users gain reindex access automatically. The plan's Step 2 only adds `PermTenantAdmin` to the 3 rollback handlers but does not mention `HandleReindexTenant`. For consistency with the defense-in-depth approach, the plan should either:
- Add the explicit `PermTenantAdmin` check to `HandleReindexTenant` as well, or
- Explicitly note that Bug #1 covers it and no change is needed.

---

## 3. Route Group Inaccuracy (Medium)

**Severity:** Medium

The plan's permission table (section 4) and Step 2 imply the RAG rollback endpoints are in the `authGroup`. They are actually registered in the `adminGroup` (v1.go:371-375), which applies `TenantBindingMiddleware` + `CSRFProtectionMiddleware` in addition to `AuthMiddleware`. This is *more* restrictive than the plan suggests, so it's a safe inaccuracy, but the plan should be corrected for accuracy.

---

## 4. Missing `HandleReindexTenant` in Defense-in-Depth (Low)

**Severity:** Low  
**File:** `handlers.go:1175`

`HandleReindexTenant` also uses `isAdmin() || hasPermission(PermAPIConfig)`. After Bug #1 fix, `tenant:admin` → `api:config` → access granted automatically. The plan's Step 2 only adds explicit `PermTenantAdmin` to the 3 rollback handlers. For consistency with the defense-in-depth approach, either add the same explicit check to `HandleReindexTenant` or note that Bug #1 covers it.

---

## 5. Minor Nits

| # | Issue | Severity |
|---|-------|----------|
| 1 | Step 2 error message should be updated to mention `tenant:admin` if the explicit check is kept | Low |
| 2 | `resolveQueryVersion` fallback to latest indexed version (service.go:4410-4418) could mask a rollback failure — consider logging when fallback is used | Low |
| 3 | Frontend `canRollbackVersion` is purely cosmetic (backend enforces independently) — acceptable but worth noting | Low |
| 4 | Plan's Step 4 store method response field names (`activeVersions`, `groups`) correctly match backend camelCase | ✓ |
| 5 | Plan's Step 3 `canRollbackVersion` location (~line 183) matches actual code at AgentAdmin.tsx:183 | ✓ |

---

## Summary

| Category | Verdict |
|----------|---------|
| Bug #1 fix (tenant:admin expansion) | **Approved** — one-line change, correct, well-scoped |
| Bug #2 fix (RAG Rollback UI) | **Approved with nits** — frontend plan is sound, backend route group needs correction |
| Race condition (rollback vs reindex) | **Must fix before execution** — add per-tenant mutex or advisory lock |
| Security boundary analysis | **Clean** — no admin-only endpoints leak through the expansion |
| Tenant isolation | **Clean** — `TenantBindingMiddleware` + handler-level checks provide defense-in-depth |

---

## Execution Order Recommendation

1. Add per-tenant mutex/advisory lock for reindex/rollback serialization
2. Fix `permissions.go:93` (Bug #1 — one-line change)
3. Update error messages in 3 rollback handlers to mention `tenant:admin`
4. Add store methods + state in `agentAdmin.ts`
5. Add `canRollbackVersion` + rollback panel in `AgentAdmin.tsx`
6. Add translations in `en.json`
7. Run lint + typecheck
8. Manual test with HOST, ADMIN, tenant-admin, and USER roles
