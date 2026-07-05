# Adversarial Code Review: Memo & Ticket Tenant Isolation

## Summary

**MAJOR SECURITY BREACHES IDENTIFIED** - The tenant isolation implementation has **critical architectural flaws** that allow complete cross-tenant data leakage. Multiple code paths allow any authenticated user to read, create, and delete memos and tickets from other tenants.

## Executive Summary

**CRITICAL VULNERABILITIES:**
- **File:** `server/router/api/v1/memo_service.go` - General memo API compromised
- **File:** `server/router/api/v1/agent/service.go` - Agent escalation handlers vulnerable  
- **Issue:** Missing `getTenantFromContext()` implementation

**UNDERLYING PROBLEM:** The general memo API (`CreateMemo`, `ListMemos`, `GetMemo`) completely ignores tenant isolation - it creates memos without setting `tenant_id`, lists all tenants' memos, and allows reading any memo by UID regardless of tenant.

**ATTACK SCENARIO:** An authenticated user can:
1. POST `/api/v1/memos` to create tenantless memos (bypassing tenant isolation)
2. GET `/api/v1/memos` to see PROTECTED memos from ALL tenants (PII exposure)
3. GET `/api/v1/memos/{id}` to read private memos from any tenant
4. POST to escalation endpoints triggers fallback that leaks tenant ID in plaintext and creates tickets without tenant context

## Critical Findings

### [1]: General Memo API Bypasses All Tenant Controls
**Severity:** CRITICAL  
**File:** `server/router/api/v1/memo_service.go`  
**Lines:** 46-51  
**Description:** `CreateMemo` doesn't set `TenantID` from context, allowing any user to create memos without tenant association.  
**Attack Vector:** Simple POST request to `/api/v1/memos` creates "tenantless" data accessible to all.  
**Fix:** Line 50: Add `tenantID := getTenantFromContext(ctx)` and set `TenantID: tenantID`.

### [2]: `ListMemos` Returns All Tenants' Protected Data
**Severity:** CRITICAL  
**File:** `server/router/api/v1/memo_service.go`  
**Lines:** 126-279  
**Description:** Without tenant filtering, anonymous sees all PUBLIC memos; authenticated see PROTECTED memos from ALL tenants.  
**Attack Vector:** `GET /api/v1/memos` reveals entire system's PROTECTED memo population.  
**Fix:** Add tenant filter after user lookup: `if tenantID := getTenantFromContext(ctx); tenantID != nil { memoFind.TenantID = tenantID }`.

### [3]: `GetMemo` Allows Reading Any Memo Across Tenants
**Severity:** CRITICAL  
**File:** `server/router/api/v1/memo_service.go`  
**Lines:** 236-267  
**Description:** UID-based lookup ignores `tenant_id`, enabling reading of any memo regardless of tenant.  
**Attack Vector:** Guess/find memo UID to access private memos from other tenants.  
**Fix:** Add tenant verification: `if memo.TenantID != nil && *memo.TenantID != *tenantID`.

### [4]: Fallback Creates Tenant-Scoped Ticket & Leaks PII
**Severity:** HIGH  
**File:** `server/router/api/v1/agent/service.go`  
**Lines:** 3802-3845  
**Description:** `createEscalationTicketFallback` creates ticket WITHOUT `TenantID` and exposes tenant ID in plaintext.  
**Attack Vector:** Trigger memo creation failure → creates ticket with customer PII + tenant ID in description.  
**Fix:** Line 3845: Set `TenantID: &tenantID` AND remove tenant ID from description (line 3806).

### [5]: CEL Filter Bypasses Tenant Controls
**Severity:** HIGH  
**File:** `store/db/sqlite/memo_filter.go`  
**Lines:** 61  
**Description:** `tenant_id` can be used in CEL filter to read memos from other tenants.  
**Attack Vector:** Filter query `tenant_id=999` retrieves all tenant 999's protected memos.  
**Fix:** Remove `tenant_id` from valid CEL filter identifiers.

### [6]: UpdateMemo Omits Tenant Verification
**Severity:** MEDIUM  
**File:** `server/router/api/v1/memo_service.go`  
**Lines:** 270-359  
**Description:** `UpdateMemo` doesn't verify new `tenant_id` matches context tenant; allows changing ownership.  
**Attack Vector:** Modify memo's `tenant_id` field to change data ownership.  
**Fix:** After fetching memo, verify `memo.TenantID` against context before updates.

### [7]: DeleteMemo Omits Tenant Controls
**Severity:** MEDIUM  
**File:** `server/router/api/v1/memo_service.go`  
**Lines:** 270-359  
**Description:** Only checks creator/admin, not tenant ownership for deletion.  
**Attack Vector:** Delete memos from other tenants by ID.  
**Fix:** Add tenant verification before deletion.

## Missing Infrastructure

### [8]: Essential `getTenantFromContext` Missing
**Severity:** CRITICAL  
**Description:** Planning expects `getTenantFromContext()` but it's NOT implemented anywhere.  
**Fix Required:** Implement in `server/router/api/v1/tenant_context.go`:
```go
func getTenantFromContext(ctx context.Context) *int32 {
    if tenantID, ok := ctx.Value("tenant_id").(*int32); ok {
        return tenantID
    }
    return nil
}
```
Also need middleware to inject tenant ID into context during authentication.

## Attack Impact

**Immediate exploitation results:**
1. **100% data breach** - All memos/tickets from all tenants exposed
2. **PII violations** - Customer data in other tenants accessible
3. **Account compromise** - Unauthorized access to all tenant data
4. **Escalation leaks** - Private tickets expose PII and tenant IDs
5. **Data destruction** - Delete any tenant's memos

**Data at risk:** Customer PII, session details, escalation tickets, sensitive business data.

## Priority Fix Order

1. **URGENT:** Implement `getTenantFromContext()` and inject into API middleware
2. **URGENT:** Fix `CreateMemo` in memo_service.go line 50
3. **URGENT:** Fix `ListMemos` in memo_service.go lines 126-279  
4. **URGENT:** Fix `GetMemo` in memo_service.go lines 236-267
5. **URGENT:** Fix `createEscalationTicketFallback` in agent/service.go line 3845
6. **URGENT:** Remove `tenant_id` from CEL filters in memo_filter.go
7. **HIGH:** Fix `UpdateMemo` and `DeleteMemo` tenant verification

## Migration Notes

**Safe to implement:** All changes are additive with NULL backward compatibility. Existing memos with NULL `tenant_id` become accessible only to creators/superusers in the new implementation.

**Rollback plan:** Can drop `tenant_id` columns or restore from backup if needed. No breaking changes to API contracts.

**Files requiring modifications (34 files identified):** Store layer, database layer (sqlite/postgres), service layer, handlers, frontend - see original plan documentation.

## TL;DR

**This is not a "plan implementation" - it's fixing a critical security architecture failure.** The entire general memo API needs tenant controls implemented immediately. Without them, bchat's multi-tenant claim is completely false.