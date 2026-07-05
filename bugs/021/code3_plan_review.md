# Adversarial Code Review: code3_plan.md (Pre-Coding Review)

**Reviewer:** Adversarial security review
**Date:** 2026-07-06
**Input:** code3_plan.md + code2_plan_review.md (interactive Q&A)
**Status:** APPROVED WITH 4 FIXES REQUIRED

---

## Interactive Q&A Decisions (carried from code2_plan_review.md)

### Q1: How should the default tenant be selected at sign-in?
**Decision: C — Require explicit selection**
- Sign-in returns list of tenants + short-lived selection token
- User calls POST /api/v1/auth/select-tenant with selection token + tenant_id
- Returns full JWT with tenant_id

### Q2: Should the sprints be reordered?
**Decision: A — Reorder sprints**
1. Sprint 1: Auth flow (sign-in selects tenant, JWT generation includes tenant_id)
2. Sprint 2: Infrastructure (middleware, getTenantFromContext, tenant context injection)
3. Sprint 3: Memo API (fix Create, List, Get, Update, Delete)
4. Sprint 4: Agent & Filters (fix fallback, CEL, SQL safety net)
5. Sprint 5: Frontend (tenant selector, switch-tenant UI)
6. Sprint 6: Testing (unit + integration)

### Q3: How should existing sessions be handled?
**Decision: A — Force re-login**
- All existing tokens invalidated on deployment
- Users must sign in again to get new JWT with tenant_id
- Causes brief disruption but eliminates the vulnerability completely

### Q4: How should the frontend get the tenant list before login?
**Decision: A — Unauthenticated endpoint**
- New endpoint: POST /api/v1/auth/tenants (takes username + password, returns tenant list + short-lived selection token)
- Selection token is single-use, expires in 5 minutes
- User then calls POST /api/v1/auth/select-tenant with selection token + tenant_id

### Q5: Should agent endpoints use JWT tenant_id or URL slug?
**Decision: A — URL slug overrides JWT**
- Agent endpoints continue using URL slug (current behavior, explicit)
- JWT tenant_id is only for the general memo API
- No change to existing agent routing model

### Q6: Should we fix CEL filter only, or also SQL layer?
**Decision: A — Fix both CEL and SQL**
- Remove tenant_id from CEL filter identifiers
- Add SQL safety net: enforce tenant_id filter at database layer when context tenant is present
- Defense-in-depth approach

### Q7: Should UpdateMemo/DeleteMemo have hard tenant checks?
**Decision: A — Hard tenant ownership check**
- Before update/delete, verify `memo.TenantID == context.TenantID`
- If mismatch, return 403 even if user is creator of different tenant's memo
- Eliminates cross-tenant data modification

### Q8: Should the effort estimate be updated?
**Decision: Update estimate to 3-4 days**

---

## New Q&A (Pre-Coding Review)

### Q9: What context mechanism should we use for tenant_id?

**Problem:** The plan uses `context.WithValue` for tenant_id, but the current code uses echo's `c.Set()` for user_id. These are different mechanisms.

**Options considered:**
- A: Use echo's c.Set() — Consistent with current user_id extraction
- B: Use context.WithValue — More idiomatic Go but requires changing user_id extraction
- C: Bridge both — Set in echo context, copy to Go context

**Decision: A — Use echo's c.Set()**
- Consistent with current code
- Less invasive
- All handlers already have access to echo.Context

---

### Q10: How should gRPC SignIn handle single-tenant users?

**Problem:** The plan says "Keep existing gRPC SignIn for single-tenant users" but doesn't show how.

**Options considered:**
- A: Auto-select if single tenant — If user has exactly 1 tenant, auto-select it
- B: Reject if not pre-selected — Always require REST two-step flow
- C: Return tenant list in gRPC — Modify SignInResponse

**Decision: A — Auto-select if single tenant**
- If user has exactly 1 tenant permission, auto-select and include in JWT
- If 0 tenants: reject with "user is not associated with any company"
- If >1 tenants: reject with "multiple tenants found, use /auth/tenants endpoint"
- Admin users: no tenant needed (workspace-wide access)

---

### Q11: Should superusers bypass tenant ownership checks?

**Problem:** The plan's tenant checks don't account for superusers. Current code allows superusers to access all memos.

**Options considered:**
- A: Yes, superusers bypass — Add !isSuperUser(user) to tenant check
- B: No, even superusers are scoped — Superusers limited to current tenant
- C: Superusers can opt-in — Pass header to access other tenants

**Decision: A — Yes, superusers bypass**
- Superusers can access all memos across tenants
- Consistent with current behavior
- Add !isSuperUser(user) to GetMemo, UpdateMemo, DeleteMemo tenant checks

---

### Q12: How should the fallback function receive tenantID?

**Problem:** The plan gets tenantID from context, but the agent service has tenantID as a parameter.

**Options considered:**
- A: Pass as parameter — Add tenantID parameter to fallback function
- B: Set in context — Agent handlers set tenantID in context before calling service

**Decision: A — Pass as parameter**
- Agent service already has tenantID as parameter
- More explicit, no hidden state
- Update fallback function signature and call site

---

## Critical Fixes Required

### FIX 1 (CRITICAL): Context Mechanism — Use echo's c.Set()

**Current plan** (`tenant_context.go`): Uses `context.WithValue`
**Required change**: Use echo's `c.Set()` / `c.Get()`.

```go
package v1

import "github.com/labstack/echo/v4"

type tenantContextKey struct{}

func getTenantFromContext(c echo.Context) *int32 {
    if v, ok := c.Get(string(tenantContextKey{})).(*int32); ok {
        return v
    }
    return nil
}

func setTenantInContext(c echo.Context, tenantID *int32) {
    c.Set(string(tenantContextKey{}), tenantID)
}

func ApplyTenantFilter(c echo.Context, find *store.FindMemo) {
    tenantID := getTenantFromContext(c)
    if tenantID != nil {
        find.TenantID = tenantID
    }
}

func ApplyTicketTenantFilter(c echo.Context, find *store.FindTicket) {
    tenantID := getTenantFromContext(c)
    if tenantID != nil {
        find.TenantID = tenantID
    }
}
```

**Impact**: Sprint 2 and Sprint 3 must be updated. All call sites change from `ctx` to `c`.

---

### FIX 2 (CRITICAL): Fallback Function — Pass tenantID as Parameter

**Current plan** (`agent/service.go:3802`):
```go
func (s *Service) createEscalationTicketFallback(ctx context.Context, ticketNumber, ticketType string, customerInfo map[string]string, issue string) (*EscalationTicketInfo, error) {
    tenantID := getTenantFromContext(ctx)  // WRONG
```

**Required change**:
```go
func (s *Service) createEscalationTicketFallback(ctx context.Context, tenantID int32, ticketNumber, ticketType string, customerInfo map[string]string, issue string) (*EscalationTicketInfo, error) {
    ticket.TenantID = &tenantID
```

And call site at line 3760:
```go
return s.createEscalationTicketFallback(ctx, tenantID, ticketNumber, ticketType, customerInfo, issue)
```

**Impact**: Sprint 4 must be updated.

---

### FIX 3 (CRITICAL): Superuser Bypass in Tenant Checks

**Current plan** (Sprint 3):
```go
if tenantID != nil && memo.TenantID != nil && *memo.TenantID != *tenantID {
    return nil, status.Errorf(codes.PermissionDenied, "permission denied")
}
```

**Required change** for GetMemo, UpdateMemo, DeleteMemo:
```go
user, err := s.GetCurrentUser(ctx)
if err != nil { return nil, err }
if tenantID != nil && memo.TenantID != nil && *memo.TenantID != *tenantID && !isSuperUser(user) {
    return nil, status.Errorf(codes.PermissionDenied, "permission denied")
}
```

**Impact**: Sprint 3 must be updated.

---

### FIX 4 (MINOR): gRPC SignIn — Auto-Select Single Tenant

**Current plan** (Q1): *"Keep existing gRPC SignIn for single-tenant users"*

**Required implementation** in `doSignIn` (`auth_service.go:171-201`):
```go
func (s *APIV1Service) doSignIn(ctx context.Context, user *store.User, expireTime time.Time) error {
    if user.Role == store.RoleUser {
        perms, err := s.Store.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{UserID: &user.ID})
        if err != nil { return status.Errorf(codes.Internal, "failed to verify user company association") }
        if len(perms) == 0 {
            return status.Errorf(codes.PermissionDenied, "user is not associated with any company")
        }
        // Auto-select if single tenant
        if len(perms) == 1 {
            tenantID := perms[0].TenantID
            accessToken, err := GenerateAccessToken(user.Email, user.ID, &tenantID, expireTime, []byte(s.Secret))
            // ... rest of flow
        }
        // Multiple tenants: require REST two-step flow
        return status.Errorf(codes.FailedPrecondition, "multiple tenants found, use /auth/tenants endpoint")
    }
    // Admin users: no tenant needed
    accessToken, err := GenerateAccessToken(user.Email, user.ID, nil, expireTime, []byte(s.Secret))
    // ... rest of flow
}
```

**Impact**: Sprint 1 must be updated.

---

## Updated Sprint Details

### Sprint 1: Auth Flow (4-6 hours)

**Files to Modify:**
| File | Changes |
|------|---------|
| `server/router/api/v1/auth.go` | Add TenantID to ClaimsMessage, update generateToken |
| `server/router/api/v1/auth_service.go` | Add /auth/tenants, /auth/select-tenant, update doSignIn |
| `server/router/api/v1/v1.go` | Register new routes, update AuthMiddleware |
| `store/migration/sqlite/0.27/01__force_relogin.sql` | Truncate user_access_token |
| `store/migration/postgres/0.27/01__force_relogin.sql` | Truncate user_access_token |

**Changes:**
1. ClaimsMessage struct: Add `TenantID *int32 json:"tenant_id,omitempty"`
2. generateToken: Add tenantID *int32 parameter
3. GenerateAccessToken: Add tenantID *int32 parameter
4. POST /api/v1/auth/tenants: Unauthenticated, returns tenant list + selection token
5. POST /api/v1/auth/select-tenant: Returns full JWT with tenant_id
6. AuthMiddleware: Extract tenant_id from JWT, set via c.Set()
7. doSignIn: Auto-select single tenant, reject multi-tenant in gRPC

---

### Sprint 2: Infrastructure (2-3 hours)

**New File:**
| File | Purpose |
|------|---------|
| `server/router/api/v1/tenant_context.go` | Echo-based context helpers |

**Changes:**
1. tenantContextKey struct (echo-based)
2. getTenantFromContext(c echo.Context) — uses c.Get()
3. setTenantInContext(c echo.Context, tenantID *int32) — uses c.Set()
4. ApplyTenantFilter(c echo.Context, find *store.FindMemo)
5. ApplyTicketTenantFilter(c echo.Context, find *store.FindTicket)

---

### Sprint 3: Memo API (4-6 hours)

**File to Modify:**
| File | Changes |
|------|---------|
| `server/router/api/v1/memo_service.go` | Fix all memo CRUD operations |

**Changes:**
1. CreateMemo: Set TenantID from context
2. ListMemos: Apply tenant filter
3. GetMemo: Verify tenant ownership + superuser bypass
4. UpdateMemo: Verify tenant ownership + superuser bypass
5. DeleteMemo: Verify tenant ownership + superuser bypass

---

### Sprint 4: Agent & Filters (2-3 hours)

**Files to Modify:**
| File | Changes |
|------|---------|
| `server/router/api/v1/agent/service.go` | Fix createEscalationTicketFallback |
| `store/db/sqlite/memo_filter.go` | Remove tenant_id from CEL identifiers |
| `store/db/postgres/memo_filter.go` | Remove tenant_id from CEL identifiers |

**Changes:**
1. createEscalationTicketFallback: Pass tenantID as parameter, remove PII leak
2. CEL filter: Remove tenant_id from valid identifiers
3. SQL safety net: Already handled by ApplyTenantFilter setting find.TenantID

---

### Sprint 5: Frontend (4-6 hours)

**Files to Modify:**
| File | Changes |
|------|---------|
| `web/src/components/PasswordSignInForm.tsx` | Add tenant selection step |
| `web/src/store/v2/user.ts` | Store tenant_id from JWT |
| `web/src/locales/en.json` | Add translations |

---

### Sprint 6: Testing (4-6 hours)

**New Files:**
| File | Purpose |
|------|---------|
| `server/router/api/v1/tenant_context_test.go` | Unit tests for context helpers |
| `server/router/api/v1/memo_service_tenant_test.go` | Integration tests for tenant isolation |

---

## Migration Strategy

### Force Re-login Migration

**SQLite** (`store/migration/sqlite/0.27/01__force_relogin.sql`):
```sql
DELETE FROM user_access_token;
```

**PostgreSQL** (`store/migration/postgres/0.27/01__force_relogin.sql`):
```sql
TRUNCATE TABLE user_access_token RESTART IDENTITY;
```

---

## Effort Summary

| Sprint | Effort | Risk |
|--------|--------|------|
| 1: Auth Flow | 4-6 hours | Medium — JWT changes affect all auth |
| 2: Infrastructure | 2-3 hours | Low — new file, no existing code changes |
| 3: Memo API | 4-6 hours | Medium — multiple handlers to update |
| 4: Agent & Filters | 2-3 hours | Low — targeted fixes |
| 5: Frontend | 4-6 hours | Medium — UI changes |
| 6: Testing | 4-6 hours | Low — new test files |
| **Total** | **20-30 hours** | |

---

## Risk Mitigation

1. **Force re-login** — Eliminates backward compatibility risk completely
2. **Defense-in-depth** — CEL + SQL safety net ensures tenant filtering even if API layer has bugs
3. **Hard ownership checks** — Update/Delete verify tenant before modification + superuser bypass
4. **URL slug for agent** — Preserves existing behavior, no confusion
5. **Explicit tenant selection** — No arbitrary defaults, user always knows their context
6. **REST alongside gRPC** — Minimal disruption to existing sign-in flow
7. **Echo-based context** — Consistent with current codebase patterns

---

## Final Verdict: APPROVED WITH 4 FIXES REQUIRED

The plan is sound and addresses all major issues. The 4 required fixes are:
1. Use echo's c.Set() instead of context.WithValue
2. Pass tenantID as parameter to fallback function
3. Add superuser bypass to tenant checks
4. Auto-select single tenant in gRPC SignIn

**Ready for coding after these fixes are applied to code3_plan.md.**
