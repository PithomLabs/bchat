# Widget Tenant Scoping Analysis

**Date:** 2026-02-01
**Purpose:** Verify tenant identification mechanism in chat widget

---

## Summary

The widget **already uses slug-based identification** throughout. The GUID was only used for:
1. URL path obfuscation (`/widget/slug-guid/embed.js`) - now removed
2. CompanyName display (`CompanyName-GUID`) - now removed

The simplification changes made are correct and align with the existing architecture.

---

## How Tenant Identification Works

### 1. Widget Configuration Sources (Priority Order)

| Source | Example | Priority |
|--------|---------|----------|
| Script data attributes | `data-tenant="acme"` | Highest |
| Global window object | `window.AgentChatConfig.tenant` | Medium |
| URL path extraction | `/widget/acme/embed.js` → `acme` | Lowest |

**Code:** `widget/src/embed.ts:58-107`

```typescript
// Extract tenant from URL path: /widget/{tenant}/embed.js
const urlMatch = currentScript.src.match(/\/widget\/([^/]+)\/embed/);
if (urlMatch) {
  config.tenant = urlMatch[1];
  config.baseUrl = url.origin;
}
```

### 2. API Calls Use Slug Only

**Widget API call:** `widget/src/core/api.ts:11`
```typescript
const url = `${config.baseUrl}/api/v1/agent/${config.tenant}/chat/ext`;
```

**Backend handler:** `server/router/api/v1/agent/handlers.go:91-148`
```go
// POST /api/v1/agent/:slug/chat/ext
slug := c.Param("slug")
tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
```

### 3. Widget Embed Serving

**Route:** `GET /widget/:slug/embed.js`

**Handler:** `handlers.go:1579-1636`
- Validates tenant by slug
- Checks domain allowlist
- Injects config and serves bundle

---

## Security Mechanisms

| Mechanism | Location | Description |
|-----------|----------|-------------|
| Domain Allowlist | `handlers.go:1539-1577` | Restricts which domains can embed widget |
| Rate Limiting | `handlers.go:91-148` | IP-based throttling per tenant |
| Tenant Validation | All handlers | Checks tenant exists and is active |
| XSS Protection | `widget/src/ui/Messages.ts:139` | HTML escapes user input |

---

## Files Involved

| File | Purpose |
|------|---------|
| `widget/src/embed.ts` | Entry point, config parsing |
| `widget/src/core/api.ts` | API client with tenant-scoped endpoints |
| `widget/src/core/types.ts` | WidgetConfig interface |
| `widget/src/ui/Widget.ts` | Main widget class |
| `server/router/api/v1/agent/handlers.go` | Backend handlers |
| `server/router/api/v1/v1.go` | Route registration |

---

## Changes Made (2026-02-01)

Simplified widget URLs to use slug only:

### Backend (`handlers.go`)

| Function | Change |
|----------|--------|
| `findTenantBySlugGuid` | Renamed to `findTenantBySlug`, matches by slug only |
| `HandleWidgetEmbed` | Uses `:slug` param, removed GUID from companyName injection |
| `HandleWidgetIframe` | Uses `:slug` param, removed GUID from companyName |
| `generateIframeHTML` | Removed slugGuid parameter |

**Key fix:** Removed GUID suffix from widget header title:
```go
// Before (lines 1620-1624)
combinedName := tenant.CompanyName
if tenant.GUID != "" {
    combinedName = fmt.Sprintf("%s-%s", tenant.CompanyName, tenant.GUID)
}

// After
// Just use tenant.CompanyName directly
```

### Routes (`v1.go`)

```go
// Before
widgetGroup.GET("/:slugGuid/embed.js", ...)
widgetGroup.GET("/:slugGuid/iframe", ...)

// After
widgetGroup.GET("/:slug/embed.js", ...)
widgetGroup.GET("/:slug/iframe", ...)
```

### Frontend (`AgentAdmin.tsx`)

Simplified embed code display to use just slug:
```html
<script src="https://bchat.fly.dev/widget/bchat/embed.js"></script>
```

---

## Conclusion

The widget system was already designed around slug-based tenant identification. The GUID was an unnecessary layer that:
1. Made URLs longer and harder to read
2. Required database GUID population
3. Added complexity without security benefit (slug is already unique)

The simplification removes this complexity while maintaining all security mechanisms (domain allowlist, rate limiting, tenant validation).

---

## Verification Checklist

- [x] Backend compiles successfully
- [x] Frontend builds successfully
- [ ] Widget loads at `/widget/{slug}/embed.js`
- [ ] Chat API works at `/api/v1/agent/{slug}/chat/ext`
- [ ] Domain allowlist still enforced
- [ ] Rate limiting still works
