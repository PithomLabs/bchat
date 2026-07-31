# Code 3.2 — TypeScript Errors in `fetchTenants` Fix

**Bug ID:** 054
**Finding:** code3.1_review.md follow-up — 3 TypeScript compile errors
**Status:** NOT FIXED — coding agent's fix introduced type errors

---

## Errors

```
src/pages/Tickets.tsx(155,46): error TS2558: Expected 0 type arguments, but got 1.
src/pages/Tickets.tsx(157,43): error TS7006: Parameter 't' implicitly has an 'any' type.
src/pages/Tickets.tsx(575,80): error TS2339: Property 'name' does not exist on type 'AgentTenant'.
```

## Root Cause

The coding agent's fix at `Tickets.tsx:151-166` has 3 issues:

1. **Line 155:** `response.json<{ ... }>()` — `Response.json()` doesn't accept type parameters in the standard Fetch API
2. **Line 160:** `name: t.companyName` — creates `{ id, slug, name }` but `AgentTenant` interface requires `companyName` (not `name`)
3. **Line 575:** `t.name` — dropdown references `name` which doesn't exist on `AgentTenant`

## Required Fix

Replace `fetchTenants` at `Tickets.tsx:151-166` with:

```typescript
const fetchTenants = async () => {
    try {
        const response = await fetch("/api/v1/agent/tenants");
        if (!response.ok) throw new Error("Failed to fetch tenants");
        const data = await response.json();
        setAvailableTenants(data.tenants || []);
    } catch (error) {
        console.error("Error loading tenants:", error);
    }
};
```

**Also update the dropdown at line 575:**

```typescript
<Option key={t.id} value={t.id}>{t.companyName}</Option>
```

## Why This Works

- `GET /api/v1/agent/tenants` returns `{ "tenants": [{ "id", "slug", "companyName", ... }] }`
- `HandleListTenants` (handlers.go:688) returns fields matching `AgentTenant` interface exactly: `id`, `slug`, `companyName`, `guid`, `vertical`, `isActive`, `allowedDomains`, `createdAt`, `updatedAt`
- No mapping needed — the API response matches `AgentTenant[]` directly
- `response.json()` without type parameters returns `any`, which is assigned to `AgentTenant[]` via the state setter

## Files to Modify

1. `web/src/pages/Tickets.tsx:151-166` — replace `fetchTenants` function
2. `web/src/pages/Tickets.tsx:575` — change `t.name` to `t.companyName`

## Verification

After fix, run:
```bash
cd web && npx tsc --noEmit 2>&1 | grep "Tickets.tsx"
```

Should return no errors.
