# Adversarial Code Review: Bug 053 — EscalateTicket Missing CreatorID

**Bug ID:** 053
**Reviewer:** Kilo (Senior Go Architect)
**Date:** 2026-07-31
**Verdict:** APPROVED WITH NITS — implementation is correct, secure, and follows existing patterns. No critical or high-severity blockers remain. Four nits tracked for follow-up.

---

## Executive Summary

The implementation fixes all three core defects identified in the plan:

| Defect | Status | Evidence |
|--------|--------|----------|
| Missing `CreatorID` | ✅ Fixed | `service.go:5547` — `CreatorID: creatorID` from JWT |
| Missing `Type` field | ✅ Fixed | `service.go:5550` — `Type: "agent_escalation"` |
| Route never registered | ✅ Fixed | `v1.go:373-374` — `authGroup.POST("/:slug/escalate", ...)` |
| Auth model | ✅ Fixed | `handlers.go:470` — `h.getUserID(c)`, returns 401 if missing |
| Permission check | ✅ Fixed | `handlers.go:482` — `h.hasPermission(c, tenant.ID, PermChatTest)` |
| Priority validation | ✅ Fixed | `handlers.go:503` — rejects non-high/medium/low with 400 |
| Redundant DB lookup | ✅ Fixed | Service accepts `tenantID int32`, no internal `GetAgentTenant` |
| Defensive guard | ✅ Fixed | `service.go:5556` — checks all 4 required fields |
| Port in curl tests | ✅ Fixed | Uses `8081` (dev server default) |

---

## Finding 1: Missing Rate Limiting

**Severity:** MEDIUM (Nit)

`HandleChatInternal` enforces rate limiting (30 RPM per user/IP) at `handlers.go:630-641`. `HandleStartSimulation` also has rate limiting. `HandleEscalateTicket` does not.

Escalation creates a database row and optionally queries the vector DB. A user with `chat:test` permission can call this endpoint in a tight loop and create unlimited tickets/load the vector DB.

**Recommendation:** Track separately. The plan correctly keeps scope tight for this bug. A follow-up should add rate limiting to all state-changing agent endpoints consistently.

---

## Finding 2: No Runnable Unit Test

**Severity:** MEDIUM (Nit)

`TestEscalateTicket` in `ticket_resolution_test.go` is skipped (`t.Skip("Requires real CockroachDB + store")`). The implementation adds no runnable unit test.

With the signature change (`tenantSlug string` → `tenantID int32`, added `creatorID int32`) and handler rewrite, a regression is possible if future edits remove the permission check or `userID` extraction.

**Recommendation:** Track separately. The implementation is acceptable without a new test for a focused bug fix. A follow-up ticket should add a handler-level test with a mock store.

---

## Finding 3: Case-Sensitive Priority in Service

**Severity:** LOW (Nit)

The handler validates priority is one of `"high"`, `"medium"`, `"lower"` (lowercase). The service maps these to `store.TicketPriorityHigh/Medium/Low` (uppercase constants). If the service were called directly with `"HIGH"`, it would silently fall through to MEDIUM.

This is not currently exploitable because only the handler calls the service, but it is a missing defensive case.

**Recommendation:** Either normalize in service with `strings.ToLower(req.Priority)` or add the same validation in service.

---

## Finding 4: Nil Tags Marshals to JSON null

**Severity:** LOW (Nit — pre-existing)

`req.Tags` is `[]string`. When omitted in JSON, `echo.Bind` sets it to `nil`. The store marshals tags with `json.Marshal(create.Tags)` at `store/db/sqlite/ticket.go:13`. `json.Marshal(nil)` returns `"null"`, while the schema default is `'[]'`.

This is pre-existing behavior in the ticket store, not introduced by this change.

**Recommendation:** Track in ticket-store cleanup.

---

## Detailed Analysis

### 1. CORRECTNESS — Service Signature and Tenant Flow

The handler passes `tenant.ID` (int32) directly to the service. The service no longer calls `GetAgentTenant`. This eliminates the third redundant DB query. The tenant ID originates from `TenantBindingMiddleware`, which resolves it from the URL slug and stores it in context. The handler then reads it via `getTenantOrFail`. No path exists where `tenant.ID` could be stale or mismatched — it is set once by trusted middleware and read once by the handler.

### 2. SECURITY — Permission Check

`TenantBindingMiddleware` enforces tenant access. `hasPermission(c, tenant.ID, PermChatTest)` enforces that the user has chat-test rights. These are orthogonal checks: one ensures the user can reach the tenant, the other ensures they have the specific capability. A user could have tenant access but lack `chat:test`, correctly receiving 403.

### 3. SECURITY — JWT User ID

`h.getUserID(c)` reads `c.Get("user-id").(int32)`. The `AuthMiddleware` sets this from the validated JWT. A client cannot manipulate it without a valid token signed by the server secret.

### 4. CONCURRENCY — Vector DB Search

The service reads `vectorDB` under `RLock`, then calls `Search` outside the lock. If the search panics, the mutex was already released, so no deadlock. SQLite serializes ticket creation. No race conditions.

### 5. ERROR HANDLING — Generic 500

The handler returns `"Escalation service unavailable"` for all service errors. The service wraps DB errors with context. This is consistent with sibling endpoints like `HandleChatInternal`. Not a blocker.

### 6. EDGE CASES — Priority Defaults

The handler validates priority is one of `"high"`, `"medium"`, `"low"`. The service maps these to `store.TicketPriorityHigh/Medium/Low` (uppercase constants). If the service were called directly with `"HIGH"`, it would silently fall through to MEDIUM. This is not exploitable because only the handler calls the service, but it is a missing defensive case in the service.

### 7. EDGE CASES — Description `/m/` Prefix

The service prepends `/m/` if missing. `store.Ticket.Validate()` only checks the prefix, not whether the memo exists. This is pre-existing and out of scope.

### 8. BACKWARD COMPATIBILITY

Grep confirms: only `HandleEscalateTicket` calls `EscalateTicket`. No other callers. Response shape (`ticket_id`, `status`, `similar_count`) is unchanged. The only behavior change is `creator_id` now reflects the authenticated user instead of the system user — documented in `code.md` and `plan3.md`.

---

## Final Verdict

**APPROVED WITH NITS.** The implementation is correct, secure, and follows project conventions. No blockers for merge. Track the four nits above as follow-up tickets.
