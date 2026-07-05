# Implementation Plan: Address Code Review Findings (code2_plan)

**Date:** 2026-07-05
**Bug:** 021
**Status:** Approved

---

## Context

The adversarial code review (`code_review_northmini.md`) identified critical architectural flaws in the tenant isolation implementation. The general memo API (`CreateMemo`, `ListMemos`, `GetMemo`, `UpdateMemo`, `DeleteMemo`) completely ignores tenant isolation — it creates memos without setting `tenant_id`, lists all tenants' memos, and allows reading any memo by UID regardless of tenant.

**Underlying Problem:** The implementation focused on the agent system but left the general memo API completely open. There is no mechanism for the general memo API to know which tenant the user belongs to.

---

## Decisions Made (Interactive Q&A)

### Q1: How should the general memo API determine which tenant the user belongs to?

| Option | Description | Decision |
|--------|-------------|----------|
| **A: JWT claims** | Add tenant_id to JWT claims. Requires re-login but cleanest solution. | **Selected** |
| B: DB lookup per request | Query user_tenant_permission per request. Adds DB overhead. | Rejected |
| C: Tenant ID header | Frontend sends X-Tenant-ID header. Most explicit but changes API contract. | Rejected |
| D: Don't scope general memos | Keep general memos workspace-wide. Only agent memos are tenant-scoped. | Rejected |

**Rationale:** JWT claims provide the cleanest solution. The tenant ID is available in every request without additional DB queries. The frontend can extract it from the token and use it for display/filtering.

### Q2: How should we handle users belonging to multiple tenants?

| Option | Description | Decision |
|--------|-------------|----------|
| **Single tenant at a time** | JWT contains one tenant_id. To switch, re-authenticate or call /switch-tenant. | **Selected** |
| Multiple tenants + header | JWT contains all tenant IDs. Frontend sends X-Tenant-ID header. | Rejected |
| Default to first tenant | JWT contains first tenant. User can't switch without re-login. | Rejected |

**Rationale:** Single tenant at a time is the simplest model. The user operates in one tenant context at a time. Switching is explicit via a dedicated endpoint.

### Q3: How should users switch between tenants?

| Option | Description | Decision |
|--------|-------------|----------|
| **Switch-tenant endpoint** | POST /api/v1/auth/switch-tenant rewrites JWT. Returns new token. | **Selected** |
| Re-login required | User logs out and logs back in. Simplest but bad UX. | Rejected |
| Query param on login | Sign-in includes tenant slug. JWT issued with that tenant. | Rejected |

**Rationale:** A dedicated switch-tenant endpoint provides good UX without requiring full re-authentication. The endpoint verifies the user has access to the target tenant, then issues a new JWT.

### Q4: How should we handle the fallback ticket creation?

| Option | Description | Decision |
|--------|-------------|----------|
| **Fix fallback** | Set TenantID on fallback ticket AND remove plaintext tenant ID from description. | **Selected** |
| Remove fallback entirely | If memo creation fails, escalation fails. No fallback. | Rejected |
| Keep as-is | Accept the risk for backward compatibility. | Rejected |

**Rationale:** The fallback path is a safety net for when memo creation fails. It should still work but with proper tenant scoping and without leaking PII.

### Q5: How should we handle the CEL filter bypass?

| Option | Description | Decision |
|--------|-------------|----------|
| **Remove from CEL** | Remove tenant_id from valid identifiers. Users can't filter by it. | **Selected** |
| Auto-inject tenant_id | If user includes tenant_id, override with their actual tenant. | Rejected |
| Block filter with tenant_id | Reject requests with tenant_id in filter string. | Rejected |

**Rationale:** Removing tenant_id from CEL filter identifiers is the simplest fix. Tenant filtering is done internally by the API layer, not exposed to users.

### Q6: What scope should we address?

| Option | Description | Decision |
|--------|-------------|----------|
| **All 7 findings** | Fix CreateMemo, ListMemos, GetMemo, UpdateMemo, DeleteMemo, fallback ticket, CEL filter. | **Selected** |
| Critical only (1-4) | Fix CreateMemo, ListMemos, GetMemo, fallback ticket. Skip Update/Delete and CEL. | Rejected |
| Critical + infrastructure | Fix 1-4 + CEL filter + middleware. Skip Update/Delete. | Rejected |

**Rationale:** All 7 findings are security issues that need to be addressed. Skipping any of them leaves attack vectors open.

---

## Scope

### In Scope

1. **Sprint 1: Infrastructure** — Implement `getTenantFromContext()`, add `tenant_id` to JWT claims, add `tenantIDContextKey`, update auth middleware to extract tenant ID
2. **Sprint 2: Auth Flow** — Add `/switch-tenant` endpoint, update sign-in to accept tenant slug, update JWT generation to include tenant_id
3. **Sprint 3: Memo API** — Fix `CreateMemo` (set TenantID), `ListMemos` (filter by tenant), `GetMemo` (verify tenant), `UpdateMemo` (verify tenant), `DeleteMemo` (verify tenant)
4. **Sprint 4: Agent & Filters** — Fix `createEscalationTicketFallback` (set TenantID, remove plaintext), remove `tenant_id` from CEL filter identifiers
5. **Sprint 5: Frontend** — Add tenant selector to login, add tenant switch UI, store tenant_id from JWT
6. **Sprint 6: Testing** — Unit tests for tenant context, integration tests for cross-tenant denial, test tenant switching

### Out of Scope

- Proto/protobuf changes (not needed for this fix)
- Database schema changes (tenant_id columns already exist from previous implementation)
- RAG pipeline changes (already tenant-scoped)

---

## Effort Estimates

| Sprint | Effort | Risk |
|--------|--------|------|
| 1: Infrastructure | 2-3 hours | Medium — JWT changes affect all auth |
| 2: Auth Flow | 2-3 hours | Medium — new endpoint + sign-in changes |
| 3: Memo API | 2-3 hours | Medium — multiple handlers to update |
| 4: Agent & Filters | 1-2 hours | Low — targeted fixes |
| 5: Frontend | 2-3 hours | Medium — UI changes |
| 6: Testing | 2-3 hours | Medium — comprehensive coverage |

**Total estimated effort: 1.5-2 days**

---

## Risk Mitigation

1. **Backward compatibility** — Existing JWT tokens without tenant_id will work (returns nil tenant, memos without tenant_id are accessible to creators/superusers)
2. **Rollback** — Can remove tenant_id from JWT claims and revert middleware changes
3. **No breaking API changes** — All existing endpoints continue to work, just with tenant scoping added
4. **Graceful degradation** — If tenant_id is missing from JWT, the API operates in workspace-wide mode (backward compatible)
