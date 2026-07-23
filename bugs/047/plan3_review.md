# Adversarial Plan Review — plan3.md (Final)

**Reviewer:** Senior Go Architect (automated)
**Date:** 2026-07-23
**Scope:** Final adversarial review of plan3.md remediation plan

---

## Verdict: APPROVED (1 minor nit)

All claims in plan3.md are **verified correct** against the actual codebase. The plan has properly incorporated all corrections from plan2_review.md.

---

## Verified Claims

| Claim | Verified? | Evidence |
|-------|-----------|----------|
| Middleware does NOT call `setTenantInContext` | ✅ Confirmed | `tenant_binding.go:69` — just calls `next(c)` |
| 0 agent handlers use `getTenantFromContext` | ✅ Confirmed | Zero results in `server/router/api/v1/agent/` |
| `ApplyAgentTenantFilter` does not exist | ✅ Confirmed | Only exists in plan docs, no `.go` source |
| 82 `GetAgentTenant` calls in handlers.go | ✅ Confirmed | Minor undercount in plan (claims 79, actual 82) |
| N+1 query in selection token lookup | ✅ Confirmed | `auth_service.go:472-499` — 1 query + N per-user queries |
| REST SignIn uses nil tenant | ✅ Confirmed | `auth_service.go:664` — `GenerateAccessToken(..., nil, ...)` |
| 30K threshold is RAG activation trigger | ✅ Confirmed | `chunker.go:71` — `DefaultTokenThreshold = 30000` |
| No mock embedding production guard | ✅ Confirmed | `embedding.go:220-233` — accepts "mock" without validation |
| BM25 normalized via `score/(score+1)` | ✅ Confirmed | `vectordb.go:914-917` and `vectordb_lance.go:1273-1281` |

---

## Nit: P0 Issue #1 Call Count

**Plan says:** 79 handler calls to `h.store.GetAgentTenant`
**Actual:** 82 calls

Minor undercount of 3. Not material to the fix but should be corrected for accuracy.

---

## Key Code Evidence

### TenantBindingMiddleware (tenant_binding.go:52-69)

```go
// After RBAC validation:
if user.Role == store.RoleAdmin {
    if !contains(user.AllowedTenantIDs, tenant.GUID) {
        return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
    }
} else {
    perms, err := s.ListUserTenantPermissions(c.Request().Context(), &store.FindUserTenantPermission{
        UserID:   &userID,
        TenantID: &tenant.ID,
    })
    if err != nil || len(perms) == 0 {
        return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
    }
}

return next(c)  // <-- No setTenantInContext call
```

### Selection Token Scan (auth_service.go:472-499)

```go
users, err := s.Store.ListUsers(ctx, &store.FindUser{})  // Query 1: ALL users
for _, user := range users {
    tokens, err := s.Store.GetUserAccessTokens(ctx, user.ID)  // Query N: per-user
    for _, token := range tokens {
        if token.AccessToken == "selection:"+req.SelectionToken {
            matchedUser = user
            break
        }
    }
}
```

### REST SignIn nil tenant (auth_service.go:664)

```go
accessToken, err := GenerateAccessToken(user.Email, user.ID, nil, expireTime, []byte(s.Secret))
//                                                                            ^^^^ tenant_id = nil ALWAYS
```

---

## Bottom Line

The plan is production-ready. All corrections from prior reviews are properly incorporated. The single nit (79 vs 82 call count) is trivial and does not affect the fix scope.

**Recommendation:** Proceed with P0 implementation.
