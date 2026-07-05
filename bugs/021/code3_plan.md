# Implementation Plan: code2_plan Review (code3_plan)

**Date:** 2026-07-06
**Bug:** 021
**Status:** PENDING APPROVAL

---

## Context

The code2_plan_review.md identified that the general memo API (`CreateMemo`, `ListMemos`, `GetMemo`, `UpdateMemo`, `DeleteMemo`) completely ignores tenant isolation. There is no mechanism for the general API to know which tenant the user belongs to. The JWT token doesn't contain tenant_id, and the auth middleware doesn't extract it.

**Root Cause:** The tenant model exists in the database (`user_tenant_permission` table), but the auth layer doesn't propagate it to the API layer.

---

## Decisions Made (Interactive Q&A)

### Q1: How should we integrate tenant selection into the existing sign-in flow?

**Decision: A — Add REST alongside gRPC**
- Keep existing gRPC `SignIn` for single-tenant users (JWT includes that tenant automatically)
- Add REST two-step flow for multi-tenant users: `POST /auth/tenants` → `POST /auth/select-tenant`

### Q2: How should the selection token be implemented?

**Decision: B — Random string in DB**
- Generate a random token, store in `user_access_token` table with 5-minute expiry
- Simple, no schema changes, naturally expires

### Q3: How should we force re-login for existing sessions?

**Decision: A — Delete all tokens on deployment**
- Migration truncates `user_access_token` table
- Clean break, eliminates backward compatibility risk

### Q4: How should the SQL safety net work?

**Decision: B — ApplyTenantFilter wrapper**
- Single function `ApplyTenantFilter(ctx, find)` called in API layer before each DB call
- Clean, testable, no SQL changes needed

### Q5: Implementation scope?

**Decision: All 6 sprints at once**

---

## Scope

### In Scope

1. **Sprint 1: Auth Flow** — Add `TenantID` to JWT, add REST endpoints for tenant selection, update auth middleware
2. **Sprint 2: Infrastructure** — Implement `getTenantFromContext()`, `ApplyTenantFilter`, context helpers
3. **Sprint 3: Memo API** — Fix CreateMemo, ListMemos, GetMemo, UpdateMemo, DeleteMemo with tenant scoping
4. **Sprint 4: Agent & Filters** — Fix fallback ticket, remove tenant_id from CEL filter, SQL safety net
5. **Sprint 5: Frontend** — Tenant selector on login, tenant switch UI, store tenant_id from JWT
6. **Sprint 6: Testing** — Unit/integration tests for tenant context, cross-tenant denial, tenant switching

### Out of Scope

- Proto/protobuf changes (not needed)
- Database schema changes for tenant_id columns (already exist)
- RAG pipeline changes (already tenant-scoped)

---

## Sprint 1: Auth Flow (4-6 hours)

### Files to Modify

| File | Changes |
|------|---------|
| `server/router/api/v1/auth.go` | Add `TenantID *int32` to `ClaimsMessage`, update `generateToken` signature |
| `server/router/api/v1/auth_service.go` | Add `POST /auth/tenants` and `POST /auth/select-tenant` REST endpoints |
| `server/router/api/v1/v1.go` | Register new routes, update `AuthMiddleware` to extract tenant_id |
| `store/migration/sqlite/0.27/01__force_relogin.sql` | Migration to truncate user_access_token |
| `store/migration/postgres/0.27/01__force_relogin.sql` | Migration to truncate user_access_token |

### Changes

1. **ClaimsMessage struct** (`auth.go:31-34`):
```go
type ClaimsMessage struct {
    Name     string  `json:"name"`
    TenantID *int32  `json:"tenant_id,omitempty"`
    jwt.RegisteredClaims
}
```

2. **generateToken function** (`auth.go:42`):
- Add `tenantID *int32` parameter
- Include `TenantID` in `ClaimsMessage` when not nil

3. **GenerateAccessToken** (`auth.go:37`):
- Update signature to accept `tenantID *int32`
- Pass through to `generateToken`

4. **POST /api/v1/auth/tenants** (new endpoint):
- Takes username + password (validates credentials)
- Returns list of tenants user has access to + selection token
- Selection token: random 32-byte string, stored in `user_access_token` with 5-min expiry
- Unauthenticated endpoint

5. **POST /api/v1/auth/select-tenant** (new endpoint):
- Takes selection token + tenant_id
- Validates selection token exists and hasn't expired
- Validates user has access to the target tenant
- Returns full JWT with tenant_id
- Sets cookie

6. **AuthMiddleware** (`v1.go:326-402`):
- After extracting `claims.Subject` (user ID), also extract `claims.TenantID`
- Set in context using `tenantIDContextKey`

7. **Force re-login migration**:
```sql
-- SQLite
DELETE FROM user_access_token;

-- PostgreSQL
TRUNCATE TABLE user_access_token RESTART IDENTITY;
```

---

## Sprint 2: Infrastructure (2-3 hours)

### New File

| File | Purpose |
|------|---------|
| `server/router/api/v1/tenant_context.go` | Context helpers for tenant isolation |

### Changes

1. **Context key** (`tenant_context.go`):
```go
package v1

import "context"

type tenantContextKey struct{}

func getTenantFromContext(ctx context.Context) *int32 {
    if v, ok := ctx.Value(tenantContextKey{}).(*int32); ok {
        return v
    }
    return nil
}

func contextWithTenant(ctx context.Context, tenantID *int32) context.Context {
    return context.WithValue(ctx, tenantContextKey{}, tenantID)
}
```

2. **ApplyTenantFilter** (`tenant_context.go`):
```go
func ApplyTenantFilter(ctx context.Context, find *store.FindMemo) {
    tenantID := getTenantFromContext(ctx)
    if tenantID != nil {
        find.TenantID = tenantID
    }
}
```

3. **ApplyTicketTenantFilter** (`tenant_context.go`):
```go
func ApplyTicketTenantFilter(ctx context.Context, find *store.FindTicket) {
    tenantID := getTenantFromContext(ctx)
    if tenantID != nil {
        find.TenantID = tenantID
    }
}
```

---

## Sprint 3: Memo API (4-6 hours)

### File to Modify

| File | Changes |
|------|---------|
| `server/router/api/v1/memo_service.go` | Fix all memo CRUD operations |

### Changes

1. **CreateMemo** (`memo_service.go:40`):
```go
// After getting user, set tenant context
tenantID := getTenantFromContext(ctx)
create.TenantID = tenantID
```

2. **ListMemos** (`memo_service.go:125`):
```go
// After building memoFind, apply tenant filter
ApplyTenantFilter(ctx, memoFind)
```

3. **GetMemo** (`memo_service.go:236`):
```go
// After fetching memo, verify tenant ownership
tenantID := getTenantFromContext(ctx)
if tenantID != nil && memo.TenantID != nil && *memo.TenantID != *tenantID {
    return nil, status.Errorf(codes.PermissionDenied, "permission denied")
}
```

4. **UpdateMemo** (`memo_service.go:270`):
```go
// After fetching memo, verify tenant ownership
tenantID := getTenantFromContext(ctx)
if tenantID != nil && memo.TenantID != nil && *memo.TenantID != *tenantID {
    return nil, status.Errorf(codes.PermissionDenied, "permission denied")
}
```

5. **DeleteMemo** (`memo_service.go:394`):
```go
// After fetching memo, verify tenant ownership
tenantID := getTenantFromContext(ctx)
if tenantID != nil && memo.TenantID != nil && *memo.TenantID != *tenantID {
    return nil, status.Errorf(codes.PermissionDenied, "permission denied")
}
```

---

## Sprint 4: Agent & Filters (2-3 hours)

### Files to Modify

| File | Changes |
|------|---------|
| `server/router/api/v1/agent/service.go` | Fix `createEscalationTicketFallback` |
| `store/db/sqlite/memo_filter.go` | Remove `tenant_id` from CEL identifiers |
| `store/db/postgres/memo_filter.go` | Remove `tenant_id` from CEL identifiers |

### Changes

1. **createEscalationTicketFallback** (`agent/service.go:3802-3861`):
```go
// Set TenantID from context
tenantID := getTenantFromContext(ctx)
ticket.TenantID = tenantID

// Remove this line (leaks PII):
// description += fmt.Sprintf("Tenant ID: %s\n", customerInfo["tenant_id"])
```

2. **CEL filter** (`memo_filter.go`):
- Remove `tenant_id` from valid CEL filter identifiers
- Users cannot filter by tenant_id (it's applied automatically)

3. **SQL safety net** (`memo_filter.go`):
- In the SQL query builder, always include `WHERE tenant_id = ?` when context has tenant
- Defense-in-depth: even if API layer forgets to call `ApplyTenantFilter`, SQL still filters

---

## Sprint 5: Frontend (4-6 hours)

### Files to Modify

| File | Changes |
|------|---------|
| `web/src/components/PasswordSignInForm.tsx` | Add tenant selection step |
| `web/src/store/v2/user.ts` | Store tenant_id from JWT |
| `web/src/locales/en.json` | Add tenant-related translations |

### Changes

1. **PasswordSignInForm** (`PasswordSignInForm.tsx`):
- After successful sign-in, check if user has multiple tenants
- If multiple: show tenant selector dropdown
- Call `POST /api/v1/auth/select-tenant` with selection token
- Store tenant_id in user store

2. **User store** (`user.ts`):
```typescript
// Add tenant_id to user state
tenantId: number | null = null;

// Extract from JWT after sign-in
const claims = decodeJwt(token);
this.tenantId = claims.tenant_id ?? null;
```

3. **Translations** (`en.json`):
```json
{
  "auth.select-tenant": "Select Company",
  "auth.tenant-required": "Please select a company to continue",
  "auth.switch-tenant": "Switch Company"
}
```

---

## Sprint 6: Testing (4-6 hours)

### New Files

| File | Purpose |
|------|---------|
| `server/router/api/v1/tenant_context_test.go` | Unit tests for context helpers |
| `server/router/api/v1/memo_service_tenant_test.go` | Integration tests for tenant isolation |

### Tests

1. **Unit tests** (`tenant_context_test.go`):
- `TestGetTenantFromContext` — returns nil when not set
- `TestGetTenantFromContext` — returns tenant_id when set
- `TestApplyTenantFilter` — sets TenantID on FindMemo
- `TestApplyTicketTenantFilter` — sets TenantID on FindTicket

2. **Integration tests** (`memo_service_tenant_test.go`):
- `TestCreateMemo_SetsTenantID` — created memo has correct tenant_id
- `TestListMemos_FiltersByTenant` — only returns memos from current tenant
- `TestGetMemo_CrossTenantDenied` — returns 403 for other tenant's memo
- `TestUpdateMemo_CrossTenantDenied` — returns 403 for other tenant's memo
- `TestDeleteMemo_CrossTenantDenied` — returns 403 for other tenant's memo
- `TestTenantSwitch` — user can switch tenants and get new JWT

---

## Migration Strategy

### Force Re-login Migration

**File:** `store/migration/sqlite/0.27/01__force_relogin.sql`
```sql
-- Force re-login by deleting all access tokens
-- Existing JWT tokens without tenant_id will be rejected
DELETE FROM user_access_token;
```

**File:** `store/migration/postgres/0.27/01__force_relogin.sql`
```sql
-- Force re-login by deleting all access tokens
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
3. **Hard ownership checks** — Update/Delete verify tenant before modification
4. **URL slug for agent** — Preserves existing behavior, no confusion
5. **Explicit tenant selection** — No arbitrary defaults, user always knows their context
6. **REST alongside gRPC** — Minimal disruption to existing sign-in flow
