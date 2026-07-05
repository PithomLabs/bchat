# Adversarial Code Review Prompt: Memo & Ticket Tenant Isolation

**Date:** 2026-07-05
**Bug:** 021
**Purpose:** Prompt for coding agent to perform adversarial code review

---

## Instructions

You are performing an adversarial security code review of the tenant isolation implementation for memos and tickets in the bchat codebase. Your goal is to find vulnerabilities, edge cases, logic errors, and security flaws that the original author may have missed.

**Be aggressive.** Assume the implementation is wrong until proven otherwise. Look for:
- Ways to bypass tenant isolation
- Race conditions
- Missing error handling
- Incomplete data migration
- Cross-tenant data leakage paths
- Regression risks
- Missing test coverage

**Do NOT:**
- Accept code at face value
- Skip files because they "look fine"
- Trust that the author followed the plan correctly
- Ignore defense-in-depth gaps

---

## Context

### What Changed

1. **Database:** Added `tenant_id INTEGER DEFAULT NULL` column to `memo` and `tickets` tables
2. **Store Layer:** Added `TenantID *int32` to Go structs and SQL queries
3. **Agent Service:** Escalation memos and tickets now set `TenantID`
4. **Ticket Dedup:** Now scoped by `TenantID` instead of scanning all tickets
5. **RSS:** Disabled (returns 410 Gone)
6. **Frontend:** Ticket default visibility changed from PUBLIC to PROTECTED

### What Was NOT Changed

- General memo API (`CreateMemo`, `ListMemos`, `GetMemo`) does NOT inject tenant context
- No middleware to inject tenant ID into request context
- No tenant-scoped visibility enforcement for the general memo API
- `DisallowPublicVisibility` workspace setting was NOT enabled by default

---

## Review Checklist

### 1. Tenant Isolation Bypass

- [ ] Can a user create a memo without `tenant_id` set?
- [ ] Can a user read memos from another tenant by manipulating the `uid` or `id`?
- [ ] Does `ListMemos` without a `TenantID` filter return memos from all tenants?
- [ ] Can anonymous users read memos if `DisallowPublicVisibility` is not set?
- [ ] Is there any code path that creates memos without setting `tenant_id`?
- [ ] Are there any SQL queries that don't filter by `tenant_id` when they should?

### 2. Escalation Memo Security

- [ ] Does `CreateEscalationTicket` always set `TenantID` on the memo?
- [ ] Does `CreateEscalationTicket` always set `TenantID` on the ticket?
- [ ] Can the fallback path (`createEscalationTicketFallback`) leak tenant data?
- [ ] Is the fallback ticket creation also tenant-scoped?
- [ ] Does `handleTicketAIResponse` always set `TenantID` on AI reply memos?

### 3. Ticket Deduplication

- [ ] Does `findExistingEscalationTicket` filter by `TenantID`?
- [ ] Can a ticket from Tenant A be matched against Tenant B's session?
- [ ] Is the string matching in `findExistingEscalationTicket` still necessary after adding `TenantID`?
- [ ] Could the `memoUID` lookup in the dedup loop leak cross-tenant data?

### 4. Database Migration

- [ ] Is the migration idempotent (safe to run multiple times)?
- [ ] Does `ALTER TABLE ... ADD COLUMN` fail gracefully if column already exists?
- [ ] Are indexes created with `IF NOT EXISTS`?
- [ ] Will existing rows get `NULL` for `tenant_id` or a default value?
- [ ] Is there a data backfill step for existing escalation memos/tickets?
- [ ] Does the migration handle both SQLite and Postgres correctly?

### 5. Backward Compatibility

- [ ] What happens when `ListMemos` is called without `TenantID` filter?
- [ ] Are existing memos (with `NULL` tenant_id) accessible to their creators?
- [ ] Can super users still see all memos regardless of `tenant_id`?
- [ ] Does the general memo API (non-agent) work correctly with `NULL` tenant_id?

### 6. RSS & Frontend

- [ ] Does the RSS endpoint return 410 for both `/explore/rss.xml` and `/u/:username/rss.xml`?
- [ ] Is the frontend ticket default PROTECTED everywhere (not just one location)?
- [ ] Are there other places in the frontend that default to PUBLIC?

### 7. CEL Filter

- [ ] Can a user filter by `tenant_id` to read memos from another tenant?
- [ ] Is `tenant_id` correctly added to the valid identifiers list?
- [ ] Can a user use `tenant_id` in a filter to bypass visibility checks?

### 8. Concurrency & Race Conditions

- [ ] Can two agents create duplicate escalation tickets simultaneously?
- [ ] Is the ticket dedup loop safe under concurrent access?
- [ ] Can a memo be created with `tenant_id` set to a different tenant than the ticket?

### 9. Missing Changes

- [ ] Does `UpdateMemo` allow changing `tenant_id` after creation?
- [ ] Is there a check to prevent moving a memo to a different tenant?
- [ ] Does `DeleteMemo` respect tenant isolation?
- [ ] Are there any other places that create memos without setting `tenant_id`?

### 10. Test Coverage

- [ ] Are there tests for tenant-scoped memo creation?
- [ ] Are there tests for cross-tenant access denial?
- [ ] Are there tests for the ticket dedup with tenant scopingoping?
- [ ] Are there tests for the RSS 410 response?
- [ ] Are there tests for `NULL` tenant_id backward compatibility?

---

## Specific Code to Examine

### High Priority

| File | Lines | Concern |
|------|-------|---------|
| `server/router/api/v1/memo_service.go` | 40-61 | `CreateMemo` does NOT set `TenantID` — any user can create tenantless memos |
| `server/router/api/v1/memo_service.go` | 125-234 | `ListMemos` does NOT filter by `tenant_id` — returns all memos |
| `server/router/api/v1/memo_service.go` | 236-268 | `GetMemo` does NOT check `tenant_id` — any user can read any memo by UID |
| `server/router/api/v1/agent/service.go` | 3802-3809 | `createEscalationTicketFallback` does NOT set `TenantID` on fallback ticket |
| `store/db/sqlite/memo.go` | 42-210 | `ListMemos` only filters by `tenant_id` when `FindMemo.TenantID` is set — default is no filter |

### Medium Priority

| File | Lines | Concern |
|------|-------|---------|
| `store/memo.go` | 98-108 | `UpdateMemo` has `TenantID` field — can tenant be changed after creation? |
| `store/db/sqlite/memo_filter.go` | 61 | `tenant_id` added to valid identifiers — can users filter by other tenants? |
| `server/router/rss/rss.go` | 46-50 | RSS disabled — but is the route still registered? |

---

## Questions to Answer

1. **What is the attack vector?** How can a malicious user exploit the remaining gaps?
2. **What is the blast radius?** If exploited, what data is exposed?
3. **What is the fix priority?** Which issues are critical vs. nice-to-have?
4. **What tests are missing?** What test cases would catch these issues?

---

## Output Format

Provide your findings in this format:

```markdown
### Finding [NUMBER]: [TITLE]

**Severity:** CRITICAL / HIGH / MEDIUM / LOW
**File:** [file path]
**Lines:** [line numbers]
**Description:** [what is wrong]
**Attack Vector:** [how to exploit]
**Fix:** [what to change]
```

---

## Deadline

Complete this review within 30 minutes. Prioritize critical and high severity issues.

---

## Reference Files

- `bugs/021/plan_signoff.md` — Decision rationale
- `bugs/021/code.md` — Implementation details
- `bugs/021/docs_public.md` — Original investigation
