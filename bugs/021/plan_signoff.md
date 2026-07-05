# Plan Signoff: Memo & Ticket Tenant Isolation

**Date:** 2026-07-05
**Bug:** 021
**Status:** Approved

---

## Problem Statement

Memos and tickets in bchat lack `tenant_id` fields, causing cross-tenant data leakage:
- PUBLIC memos are globally accessible to anonymous users across all tenants
- PROTECTED memos leak across tenant boundaries to any authenticated user
- Escalation memos containing customer PII are created with `Protected` visibility without tenant scoping
- Ticket deduplication scans ALL tickets globally

The bchat platform is not designed to publish memos to the internet.

---

## Decisions Made

### 1. Visibility Model: Option B

**Decision:** Keep existing PUBLIC/PROTECTED/PRIVATE levels, add `tenant_id` filtering.

**Rationale:** Simplest change. `DisallowPublicVisibility=true` already blocks true public exposure, so PUBLIC effectively becomes "visible within workspace" once tenant filtering is added. No proto enum changes needed.

| Level | After Changes |
|-------|---------------|
| PUBLIC | Blocked by `DisallowPublicVisibility=true` |
| PROTECTED | Visible to authenticated users (tenant-scoped when `tenant_id` set) |
| PRIVATE | Creator + super user only |

### 2. Tickets: Included in Plan

**Decision:** Add `tenant_id` to tickets table, fix dedup query, scope ticket listing by tenant.

**Rationale:** Tickets are the bridge between the agent system and memos. Escalation tickets contain PII and must be tenant-scoped. Completes the isolation story in one go.

### 3. RSS Feeds: Disabled

**Decision:** RSS endpoints return 410 Gone.

**Rationale:** bchat is not a public publishing platform. RSS exposes PUBLIC memos to anonymous users. Disabling eliminates the attack surface.

### 4. Backward Compatibility: NULL tenant_id

**Decision:** Existing memos/tickets get `NULL` for `tenant_id`. They remain accessible to their creators and super users only.

**Rationale:** Safe, non-destructive, no data loss. Existing memos are invisible to tenants but remain accessible to their original creators. New memos get `tenant_id` set automatically.

### 5. Escalation Memos: Keep Protected + Tenant Filtering

**Decision:** Escalation memos retain `Protected` visibility but gain `tenant_id`. Any authenticated user in the SAME tenant can read them.

**Rationale:** Balanced approach. Agents can see their own escalation history within the tenant. Cross-tenant exposure is eliminated by the `tenant_id` filter.

### 6. Frontend Defaults: Change to PROTECTED

**Decision:** Ticket descriptions default to `Visibility.PROTECTED` instead of `Visibility.PUBLIC`.

**Rationale:** Consistent with security model. Even if `DisallowPublicVisibility` blocks PUBLIC, the UI should reflect the intended default.

---

## Scope

### In Scope
- Add `tenant_id` to `memo` and `tickets` tables (SQLite + Postgres)
- Add `TenantID` to Go structs (`Memo`, `FindMemo`, `UpdateMemo`, `Ticket`, `FindTicket`)
- Update all SQL queries to filter by `tenant_id`
- Add `tenant_id` to CEL filter expressions
- Set `TenantID` on agent-created memos and tickets
- Scope ticket deduplication by tenant
- Disable RSS endpoints
- Update frontend ticket defaults

### Out of Scope
- Full tenant-scoped memo listing for the general memo API (requires middleware changes)
- RSS re-enablement with tenant scoping
- Proto enum changes (Option A was rejected)

---

## Approval

| Role | Name | Date | Signoff |
|------|------|------|---------|
| Author | | 2026-07-05 | |
| Reviewer | | | |
