# Code 3.1 — Critical Fix: `fetchTenants` Wrong Endpoint

**Bug ID:** 054
**Finding:** code3_review.md Finding #1 — CRITICAL
**Status:** NOT FIXED — coding agent said done but this was missed

---

## Problem

`web/src/pages/Tickets.tsx:151-164` — `fetchTenants` sends `POST /api/v1/auth/tenants` with an empty body `{}`. This endpoint (`HandleAuthTenants` at `auth_service.go:365`) requires `username` and `password` in the request body. Sending `{}` binds to empty strings → `GetUser` returns nil → HTTP 401. The tenant dropdown never populates.

## Required Fix

Replace the `fetchTenants` function at `Tickets.tsx:151-164` with:

```typescript
const fetchTenants = async () => {
    try {
        const response = await fetch("/api/v1/agent/tenants");
        if (!response.ok) throw new Error("Failed to fetch tenants");
        const data = await response.json<{ tenants: { id: number; companyName: string; slug: string }[] }>();
        setAvailableTenants(
            (data.tenants || []).map((t) => ({
                id: t.id,
                slug: t.slug,
                companyName: t.companyName,
                guid: "",
                vertical: "",
                isActive: true,
                allowedDomains: [],
                createdAt: "",
                updatedAt: "",
            }))
        );
    } catch (error) {
        console.error("Error loading tenants:", error);
    }
};
```

## Key Changes

1. **Endpoint:** `POST /api/v1/auth/tenants` → `GET /api/v1/agent/tenants`
2. **Method:** POST with empty body → GET (no body needed, cookie auth handles credentials)
3. **Response parsing:** Admin endpoint returns `companyName` not `name`. Map to `AgentTenant` interface fields.
4. **Auth:** Admin endpoint uses cookie auth (already sent by browser). No explicit credentials needed.

## Why This Works

- `GET /api/v1/agent/tenants` is registered under `adminGroup` at `v1.go:391`
- `adminGroup` has `AuthMiddleware` (line 385) — cookie auth is sufficient
- `HandleListTenants` checks `isSuperAdmin(c)` (line 692) — HOST passes this check
- Response format: `{ "tenants": [{ "id", "slug", "companyName", ... }] }`
- `AgentTenant` interface (store/v2/agentAdmin.ts:5) uses `companyName` field

## Verification

After the fix, verify:
1. HOST opens ticket modal → dropdown populates with all tenants
2. Scoped ADMIN opens modal → dropdown shows only allowed tenants
3. Regular USER opens modal → dropdown hidden
4. No console errors in browser dev tools
