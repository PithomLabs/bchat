# Adversarial Code Review: code2_plan.md (Interactive Q&A)

**Reviewer:** Adversarial security review
**Date:** 2026-07-06
**Input:** code2_plan.md (original plan) + code_review_northmini.md (7 findings)

---

## Interactive Q&A Decisions

### Q1: How should the default tenant be selected at sign-in?

**Problem:** `doSignIn` at `auth_service.go:171-201` checks `len(perms) == 0` but doesn't select a specific tenant. JWT generation at line 183 passes `user.Email` and `user.ID` but no `tenant_id`.

**Options considered:**
- A: First tenant alphabetically — Simple but arbitrary
- B: Most recently used — Requires new DB column
- C: Require explicit selection — Most secure, adds a step
- D: Single-tenant users only — Limits flexibility

**Decision: C — Require explicit selection**
- Sign-in returns list of tenants + short-lived selection token
- User calls POST /api/v1/auth/select-tenant with selection token + tenant_id
- Returns full JWT with tenant_id

---

### Q2: Should the sprints be reordered?

**Problem:** Current plan modifies `GenerateAccessToken` in Sprint 1 (infrastructure) but the sign-in flow needs to be updated FIRST to populate tenant_id. If JWT struct changes before population, every existing token gets `nil` tenant_id.

**Options considered:**
- A: Reorder sprints — Auth flow first, then infrastructure
- B: Keep current order — Accept deployment risk

**Decision: A — Reorder sprints**

**New sprint order:**
1. Sprint 1: Auth flow (sign-in selects tenant, JWT generation includes tenant_id)
2. Sprint 2: Infrastructure (middleware, getTenantFromContext, tenant context injection)
3. Sprint 3: Memo API (fix Create, List, Get, Update, Delete)
4. Sprint 4: Agent & Filters (fix fallback, CEL, SQL safety net)
5. Sprint 5: Frontend (tenant selector, switch-tenant UI)
6. Sprint 6: Testing (unit + integration)

---

### Q3: How should existing sessions be handled?

**Problem:** The plan claims "existing JWT tokens without tenant_id will work (returns nil tenant)." But nil tenant = `ListMemos` returns ALL memos from ALL tenants (the exact vulnerability being fixed).

**Options considered:**
- A: Force re-login — Invalidates all existing tokens, most secure
- B: Graceful degradation with restrictions — nil = workspace-wide read-only
- C: Graceful degradation with warning — nil = full access, log warning

**Decision: A — Force re-login**
- All existing tokens invalidated on deployment
- Users must sign in again to get new JWT with tenant_id
- Causes brief disruption but eliminates the vulnerability completely

---

### Q4: How should the frontend get the tenant list before login?

**Problem:** The /switch-tenant endpoint requires authentication. But before login, the user doesn't have a token yet. They need to know which tenants are available to select from.

**Options considered:**
- A: Unauthenticated endpoint — POST /api/v1/auth/tenants returns tenants + selection token
- B: Two-step sign-in — Sign-in returns partial JWT + tenant list
- C: Embed in sign-in response — Sign-in returns tenant list, no usable JWT

**Decision: A — Unauthenticated endpoint**
- New endpoint: POST /api/v1/auth/tenants (takes username + password, returns tenant list + short-lived selection token)
- Selection token is single-use, expires in 5 minutes
- User then calls POST /api/v1/auth/select-tenant with selection token + tenant_id

---

### Q5: Should agent endpoints use JWT tenant_id or URL slug?

**Problem:** Agent endpoints (`/api/v1/agent/:slug/...`) already extract tenant from URL slug. The JWT will also have tenant_id. These could mismatch if user switches tenants but URL still points to old tenant.

**Options considered:**
- A: URL slug overrides JWT — Agent endpoints always use tenant from URL
- B: JWT must match URL — Verify match, return 403 on mismatch
- C: JWT is primary — Ignore URL slug

**Decision: A — URL slug overrides JWT**
- Agent endpoints continue using URL slug (current behavior, explicit)
- JWT tenant_id is only for the general memo API
- No change to existing agent routing model

---

### Q6: Should we fix CEL filter only, or also SQL layer?

**Problem:** Even if tenant_id is removed from CEL filter, the SQL query in `store/db/sqlite/memo.go:85-87` only adds tenant filter when `find.TenantID != nil`. If API layer doesn't set it, SQL still leaks.

**Options considered:**
- A: Fix both CEL and SQL — Belt and suspenders
- B: CEL only — Trust API layer

**Decision: A — Fix both CEL and SQL**
- Remove tenant_id from CEL filter identifiers
- Add SQL safety net: enforce tenant_id filter at database layer when context tenant is present
- Defense-in-depth approach

---

### Q7: Should UpdateMemo/DeleteMemo have hard tenant checks?

**Problem:** `UpdateMemo` at `memo_service.go:270-359` only checks `memo.CreatorID != user.ID && !isSuperUser(user)`. Doesn't check tenant. `DeleteMemo` has same issue.

**Options considered:**
- A: Hard tenant ownership check — Verify memo.TenantID == context.TenantID
- B: Soft tenant check — Allow if creator OR admin, log warning
- C: Skip tenant check — Trust API layer filtering

**Decision: A — Hard tenant ownership check**
- Before update/delete, verify `memo.TenantID == context.TenantID`
- If mismatch, return 403 even if user is creator of different tenant's memo
- Eliminates cross-tenant data modification

---

### Q8: Should the effort estimate be updated?

**Problem:** Original estimate is 1.5-2 days (6 sprints × 2-3 hours). Realistic breakdown:

| Sprint | Actual Effort |
|--------|--------------|
| 1: Auth flow | 4-6 hours |
| 2: Infrastructure | 2-3 hours |
| 3: Memo API | 4-6 hours |
| 4: Agent & Filters | 2-3 hours |
| 5: Frontend | 4-6 hours |
| 6: Testing | 4-6 hours |
| **Total** | **3-4 days** |

**Decision: Update estimate to 3-4 days**

---

## Additional Findings Not in Original Plan

### Finding A: generateToken Function Signature Change

**File:** `auth.go:42`
```go
func generateToken(username string, userID int32, audience string, expirationTime time.Time, secret []byte) (string, error)
```

This function must be updated to accept `tenantID *int32` parameter. All callers (SignIn, SignUp, RefreshToken) must be updated.

### Finding B: ClaimsMessage Struct Change

**File:** `auth.go:31-34`
```go
type ClaimsMessage struct {
    Name string `json:"name"`
    jwt.RegisteredClaims
}
```

Must add `TenantID *int32` field. This changes the JWT token structure.

### Finding C: AuthMiddleware Must Extract Tenant

**File:** `v1.go:326-402`

The `AuthMiddleware` function currently sets only `userID` in context. Must also extract and set `tenant_id` from JWT claims.

### Finding D: Cookie Must Include Tenant

**File:** `auth_service.go:191`

The `buildAccessTokenCookie` function returns a cookie. If tenant_id is in the JWT, the cookie automatically includes it. But the frontend must also store the tenant_id for display purposes.

---

## Updated Plan Summary

### Sprint 1: Auth Flow (4-6 hours)
- Add `TenantID *int32` to `ClaimsMessage` struct
- Update `generateToken` to accept and include `tenantID`
- Add POST /api/v1/auth/tenants (unauthenticated, returns tenant list + selection token)
- Add POST /api/v1/auth/select-tenant (authenticated, returns full JWT with tenant_id)
- Update sign-in flow to require tenant selection
- Invalidate all existing tokens on deployment

### Sprint 2: Infrastructure (2-3 hours)
- Implement `getTenantFromContext()` in `server/router/api/v1/tenant_context.go`
- Update `AuthMiddleware` to extract tenant_id from JWT and set in context
- Add `tenantIDContextKey` constant

### Sprint 3: Memo API (4-6 hours)
- Fix `CreateMemo` — set TenantID from context
- Fix `ListMemos` — filter by tenant
- Fix `GetMemo` — verify tenant ownership
- Fix `UpdateMemo` — verify tenant ownership before update
- Fix `DeleteMemo` — verify tenant ownership before delete

### Sprint 4: Agent & Filters (2-3 hours)
- Fix `createEscalationTicketFallback` — set TenantID, remove plaintext tenant ID
- Remove `tenant_id` from CEL filter identifiers
- Add SQL safety net for tenant_id filtering

### Sprint 5: Frontend (4-6 hours)
- Add tenant selector to login flow
- Add tenant switch UI
- Store tenant_id from JWT for display
- Update API calls to include tenant context

### Sprint 6: Testing (4-6 hours)
- Unit tests for tenant context injection
- Integration tests for cross-tenant access denial
- Test tenant switching flow
- Test backward compatibility (force re-login)

---

## Risk Mitigation

1. **Force re-login** — Eliminates backward compatibility risk completely
2. **Defense-in-depth** — CEL + SQL safety net ensures tenant filtering even if API layer has bugs
3. **Hard ownership checks** — Update/Delete verify tenant before modification
4. **URL slug for agent** — Preserves existing behavior, no confusion
5. **Explicit tenant selection** — No arbitrary defaults, user always knows their context

---

## Final Verdict: APPROVED WITH SIGNIFICANT REWORK

The plan has the right direction but requires:
1. Sprint reordering (auth flow first)
2. Force re-login for existing sessions
3. Updated effort estimate (3-4 days)
4. Explicit tenant source rules (JWT vs URL slug)
5. SQL safety net in addition to CEL fix
