# Implementation Plan Review — plan2.md (Coding Review Fixes)

**Date:** 2026-07-06
**Reviewer:** opencode
**Status:** Approved with nits

---

## Verdict: Approved with nits

The plan is well-structured — clear phases (P0/P1/P2/P2-deferred), correct prioritization of critical runtime fixes before security hardening, and appropriate deferral of issues requiring separate design discussions. The 6 open questions are answered below.

---

## Open Question Answers

### Q1: CRIT-2 — `AllowedTenantIDs` Storage Format

**Answer: Option A** — Keep `[]string` in Go struct, marshal/unmarshal at SQL boundary in `store/db/sqlite/user.go`.

**Rationale:** Cleaner Go types throughout the codebase. Marshaling is isolated to the store layer. Middleware and handlers work with `[]string` directly without JSON concerns.

### Q2: MED-4 — `UpsertMemoRelation` ON CONFLICT Behavior

**Answer: Option A** — `ON CONFLICT DO UPDATE SET tenant_id = excluded.tenant_id`.

**Rationale:** The function is named `Upsert`. A second call with the same `(memo_id, related_memo_id, type)` should update the `tenant_id` if it was previously NULL (e.g., REFERENCE relations created before the tenant isolation fix).

### Q3: HIGH-7 — CSRF Protection Strategy

**Answer: Option A** — Set `SameSite=Lax` for auth cookies.

**Rationale:** The admin frontend already uses the Authorization header for authenticated requests. The cookie is only set during the initial sign-in flow, where `SameSite=Lax` is sufficient (same-site navigation triggers the cookie). No CSRF token framework needed.

### Q4: MED-1 — Rate Limiting IP Trust

**Answer: Option B** — Make trusted header configurable via env var.

**Rationale:** Different deployments use different reverse proxies (nginx, Cloudflare, fly.io). A configurable header (e.g., `RATE_LIMIT_TRUSTED_HEADER=X-Real-IP`) gives operators flexibility without hardcoding assumptions.

### Q5: HIGH-4 — Frontend Dependency on access_token in JSON

**Answer: Safe to remove.** The frontend does NOT read `access_token` from the JSON response.

**Evidence:** `web/src/components/PasswordSignInForm.tsx:100-120` — the `selectTenant` function:
- On success: never calls `response.json()`, never reads the body
- Stores only `tenant_id` in `localStorage` (for UI display)
- Authenticates via HttpOnly cookie set by the server on the same response

Removing `access_token` and `cookie` from the JSON response is safe.

### Q6: CRIT-5 — Lock Ordering for Session Cleanup

**Answer: Option A** — Acquire `mu` → `locksMu` in `cleanup()`.

**Rationale:** Preserves existing concurrency semantics of `SessionLock` (which acquires `locksMu` independently). Consistent ordering (`mu` always before `locksMu`) prevents deadlock.

---

## Nits

### Nit 1: CRIT-3 fix should also update all `DeleteMemoRelation` callsites

The plan fixes `memo_service.go:460` but there are additional `DeleteMemoRelation` callsites:
- `memo_service.go:476-489` (comment cascade delete)
- `memo_service.go:492-495` (reference cascade delete)

All three should pass `TenantID: memo.TenantID`. The plan only mentions line 460.

### Nit 2: HIGH-9 fix needs to handle legacy memos explicitly

The proposed fix:
```go
if memo.TenantID != nil {
    if tenantID == nil || *memo.TenantID != *tenantID {
        if !isSuperUser(user) {
            return nil, status.Errorf(codes.PermissionDenied, "permission denied")
        }
    }
}
```

This allows access when `memo.TenantID == nil` (legacy memos with no tenant). If the intent is to deny access to legacy tenantless memos for tenant-scoped users, the logic needs an additional branch. If legacy memos should remain globally accessible, this is fine — but worth documenting the decision.

### Nit 3: MED-3 rate limiter eviction should use a bounded map

Adding a cleanup goroutine is good, but the map still has no size limit. An attacker can still fill it faster than cleanup runs. Consider adding a max-entries check (e.g., 10,000 IPs) with LRU eviction, or using a fixed-size ring buffer.

### Nit 4: CRIT-5 cleanup should handle orphaned locks

The cleanup should also iterate `sessionLocks` independently and remove entries whose keys are not in `sessions` (orphaned locks from sessions that were evicted by a different code path or crashed between creation and cleanup).

### Nit 5: HIGH-6 quick mitigation

While the full per-tenant signing key architecture can be deferred, a quick mitigation is available: derive the HMAC key from `HMAC(tenant.GUID, "session-token-key")` instead of using the GUID directly. This is not a full fix but prevents the GUID from being usable as-is for forgery. Consider including this as a sub-task in Phase 2.

---

## Summary

| Phase | Issues | Est. Effort | Status |
|-------|--------|-------------|--------|
| Phase 1 (P0) | CRIT-1, CRIT-2, CRIT-3, CRIT-4 | ~30 min | Approved |
| Phase 2 (P1) | CRIT-5, CRIT-6, HIGH-3 through HIGH-10 | ~2 hours | Approved with nits 1,2,4,5 |
| Phase 3 (P2) | HIGH-1, MED-2 through MED-6 | ~2 hours | Approved with nit 3 |
| Deferred | HIGH-2, HIGH-6, HIGH-7, MED-1, LOW-23/24/25 | Separate design | Approved |

---

## Sign-Off

- [x] Plan reviewed
- [x] Open questions answered
- [x] Approved with nits (no rework needed)
- [ ] Nits addressed during implementation
