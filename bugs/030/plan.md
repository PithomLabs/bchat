# Security Remediation Implementation Plan — Phases 0–2

**Based on:** `docs_fable_sec.md` (Security Review, 2026-07-08)
**Scope:** Critical and High severity issues only (Phase 0, 1, 2)
**Status:** 📋 Planning Phase

---

## Executive Summary

This plan addresses **3 Critical** and **7 High** security vulnerabilities across three phases, ordered by impact and dependency. The plan is designed for **interactive refinement** — each section includes recommendations and decision points requiring your approval.

**Total Estimated Effort:** 8–14 dev-days

| Phase | Focus | Effort | Risk if Deferred |
|-------|-------|--------|------------------|
| 0 | Emergency (rotate keys, gate pprof, fix debug) | S (hours) | Immediate compromise risk |
| 1 | Tenant isolation (IDOR, middleware, super-user) | L (3-5d) | Cross-tenant PII breach |
| 2 | Attack surface hardening (path traversal, CSRF, container, SSRF) | M (2-3d) | RCE, CSRF, container escape |

---

## Phase 0 — Emergency (Do Today)

> **Goal:** Eliminate immediate information disclosure and credential exposure
> **Effort:** Small (hours)
> **Can be parallelized:** Yes

---

### H7 — Rotate Credentials

**Problem:** Live credentials (OpenRouter API key, encryption master key, Tigris S3 keys) exist in repo-local `.env` files. The OpenRouter key was previously committed (tripped GitHub Push Protection).

**Current State:**
```
.env → OPENROUTER_API_KEY=sk-or-v1-...
.env → ENCRYPTION_MASTER_KEY=REDACTED-UUID
bugs/026/s3_probe/.env → AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY
```

**Implementation Steps:**
1. **Rotate OpenRouter API key** at https://openrouter.ai/keys
2. **Generate new encryption master key:** `uuidgen` or `openssl rand -hex 32`
3. **Rotate Tigris S3 credentials** in Tigris dashboard
4. **Update `.env` files** with new credentials
5. **Plan re-encryption migration** for stored tenant API keys (backup key path in `crypto/encryption.go` supports a migration window)

**⚠️ DECISION POINT 1: Master Key Rotation Strategy**

The encryption master key protects all tenant API keys. Rotating it requires re-encrypting existing data.

| Option | Description | Effort | Risk |
|--------|-------------|--------|------|
| **A (Recommended)** | Rotate now + schedule migration | Low immediate, M for migration | Keys re-encrypted gradually |
| **B** | Rotate + immediate migration | M | Brief downtime window |
| **C** | Defer rotation, add key versioning | M | Old key still in use |

**My Recommendation:** Option A. Rotate the key immediately (stop the bleeding), then build a migration script that runs as a background job during low-traffic hours. The `crypto/encryption.go` backup-key path already supports dual-key decryption.

---

### C2 — Gate pprof Endpoints

**Problem:** `/debug/pprof/*` endpoints are publicly accessible with no authentication, allowing:
- Process command-line leaks (`/cmdline`)
- Memory/stack dumps with in-flight secrets (`/heap`, `/goroutine`)
- CPU-pegging DoS (`/profile?seconds=30`)

**Current Code:**
```go
// server/profiler/profiler.go:28-42
func (*Profiler) RegisterRoutes(e *echo.Echo) {
    g := e.Group("/debug/pprof")
    // ... unconditional registration
}
```

```go
// server/server.go:57-58
s.profiler = profiler.NewProfiler()
s.profiler.RegisterRoutes(echoServer)  // No mode check!
```

**Implementation Steps:**
1. **Add environment variable check:** `MEMOS_ENABLE_PPROF=false` (default)
2. **Gate registration behind `profile.IsDev()` AND env flag:**
   ```go
   // server/server.go
   if profile.IsDev() || os.Getenv("MEMOS_ENABLE_PPROF") == "true" {
       s.profiler.RegisterRoutes(echoServer)
   }
   ```
3. **Add loopback binding** (even when enabled, bind to `127.0.0.1` only)
4. **Optional: Add IP allowlist middleware** for pprof routes

**⚠️ DECISION POINT 2: pprof Access Control**

| Option | Description | Effort | Use Case |
|--------|-------------|--------|----------|
| **A (Recommended)** | Dev-mode only (no prod access) | S | Most secure |
| **B** | Env flag + loopback binding | S | Production debugging possible |
| **C** | Env flag + IP allowlist | M | Remote debugging with control |

**My Recommendation:** Option A. In production, pprof should never be needed. If you must debug production, use SSH tunneling to the loopback interface instead of exposing endpoints.

---

### C3 — Fix Echo Debug Mode

**Problem:** `echoServer.Debug = true` is hardcoded, causing Echo to serialize internal error messages (file paths, dependency internals, query fragments) in HTTP responses.

**Current Code:**
```go
// server/server.go:50
echoServer.Debug = true  // Unconditional!
```

**Implementation Steps:**
1. **Set debug mode conditionally:**
   ```go
   echoServer.Debug = profile.IsDev()
   ```
2. **Add custom HTTP error handler** for production:
   ```go
   if !profile.IsDev() {
       echoServer.HTTPErrorHandler = func(err error, c echo.Context) {
           // Log details server-side
           slog.Error("request error", "error", err, "path", c.Request().URL.Path)
           // Return generic message to client
           if he, ok := err.(*echo.HTTPError); ok {
               c.JSON(he.Code, map[string]string{"error": "Internal server error"})
           } else {
               c.JSON(500, map[string]string{"error": "Internal server error"})
           }
       }
   }
   ```

**Effort:** Small (< 1 hour)
**Risk:** None — this is strictly more secure

---

## Phase 1 — Critical Tenant Isolation

> **Goal:** Eliminate cross-tenant IDOR and unify authorization model
> **Effort:** Large (3-5 days)
> **Dependency:** None (can start immediately after Phase 0)
> **Testing:** Requires cross-tenant test harness

---

### C1 + H1 + H2 — Unified Tenant Isolation Fix

These three issues are deeply interconnected and should be fixed as a single workstream:

| Issue | Root Cause | Fix |
|-------|------------|-----|
| **C1** | 9 handlers missing `isAdmin`/`hasPermission` checks | Add per-handler guards |
| **H1** | `TenantBindingMiddleware` bypasses all `RoleUser` | Enforce binding for all roles |
| **H2** | `isSuperUser` treats all admins as global | Unify on `RoleAdmin + empty AllowedTenantIDs` |

---

#### H1 — Fix TenantBindingMiddleware

**Problem:** The middleware only enforces tenant binding for `RoleAdmin` with non-empty `AllowedTenantIDs`. All `RoleUser` pass through unchecked.

**Current Code:**
```go
// server/router/api/v1/tenant_binding.go:30-38
if user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0 {
    return next(c) // superuser
}
if user.Role != store.RoleAdmin {
    return next(c) // ANY RoleUser passes!
}
```

**Proposed Fix:**
```go
func TenantBindingMiddleware(s *store.Store) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            userID, ok := c.Get(getUserIDContextKey()).(int32)
            if !ok {
                return next(c) // No user context, let auth middleware handle
            }

            user, err := s.GetUser(c.Request().Context(), &store.FindUser{ID: &userID})
            if err != nil || user == nil {
                return echo.NewHTTPError(http.StatusForbidden, "access denied")
            }

            // Super users (admin with empty AllowedTenantIDs) bypass binding
            if user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0 {
                return next(c)
            }

            slug := c.Param("slug")
            if slug == "" {
                return next(c) // No slug in URL, skip check
            }

            // Look up the tenant by slug
            tenant, err := s.GetAgentTenant(c.Request().Context(), &store.FindAgentTenant{Slug: &slug})
            if err != nil || tenant == nil {
                return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
            }

            // Check if user has explicit grant for this tenant
            // For scoped admins: check AllowedTenantIDs
            // For regular users: check RBAC permission grant
            if user.Role == store.RoleAdmin {
                if !contains(user.AllowedTenantIDs, tenant.GUID) {
                    return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
                }
            } else {
                // For RoleUser/RoleHost: verify they have any permission on this tenant
                hasAccess, err := s.HasTenantAccess(user.ID, tenant.ID)
                if err != nil || !hasAccess {
                    return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
                }
            }

            return next(c)
        }
    }
}
```

**⚠️ DECISION POINT 3: User Tenant Access Check**

For `RoleUser` (non-admin) users, we need to verify they have access to the tenant in the URL.

| Option | Description | Effort | Complexity |
|--------|-------------|--------|------------|
| **A (Recommended)** | Check RBAC permission grants table | M | Low |
| **B** | Add `AllowedTenantIDs` to all user roles | M | Medium |
| **C** | Add `tenant_id` claim to JWT | L | High |

**My Recommendation:** Option A. The RBAC permission system already tracks `(user_id, tenant_id, permission)` tuples. We can query this table to verify the user has at least one permission grant for the tenant. This is the minimal-change approach.

**New store method needed:**
```go
// store/agent.go
func (s *Store) HasTenantAccess(ctx context.Context, userID int32, tenantID int32) (bool, error) {
    // Query agent_user_permissions WHERE user_id = ? AND tenant_id = ?
    // Return true if any row exists
}
```

---

#### H2 — Unify Super-User Definition

**Problem:** Two conflicting definitions:
- `common.go:66-68`: `isSuperUser = RoleAdmin || RoleHost` (all admins are super)
- `tenant_binding.go:31`: Super = `RoleAdmin && len(AllowedTenantIDs) == 0` (only global admins)

This lets scoped admins cross tenants via `isSuperUser` bypass in tickets/memos.

**Current Code:**
```go
// server/router/api/v1/common.go:66-68
func isSuperUser(user *store.User) bool {
    return user.Role == store.RoleAdmin || user.Role == store.RoleHost
}
```

**Proposed Fix:**
```go
// server/router/api/v1/common.go
func isSuperUser(user *store.User) bool {
    // Only true admins with NO tenant restrictions are super users
    return user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0
}

// Also handle RoleHost (if still used)
func isHostUser(user *store.User) bool {
    return user.Role == store.RoleHost
}
```

**Cascade Changes Required:**
- `ticket_service.go:192,279,341,417` — Update `isSuperUser` calls
- `memo_service.go:270,280,318,325,451,458` — Update `isSuperUser` calls
- All handlers using `isSuperUser` for tenant bypass

**⚠️ DECISION POINT 4: RoleHost Handling**

`RoleHost` appears to be a legacy role. How should we handle it?

| Option | Description | Effort | Risk |
|--------|-------------|--------|------|
| **A (Recommended)** | Treat as regular user (no super-user bypass) | S | May break legacy flows |
| **B** | Keep as super-user | S | Maintains current behavior |
| **C** | Remove RoleHost entirely | M | Requires migration |

**My Recommendation:** Option A. `RoleHost` is not used in the tenant binding middleware and appears to be a legacy concept. Treat it as a regular user to close the bypass gap.

---

#### C1 — Add Permission Guards to 9 Handlers

**Problem:** The following handlers in `adminGroup` have NO `isAdmin`/`hasPermission` checks:

| Handler | Line | Action | Required Permission |
|---------|------|--------|---------------------|
| `HandleListTranscripts` | :5896 | List transcripts | `chat:logs` |
| `HandleGetTranscript` | :5924 | Get transcript | `chat:logs` |
| `HandleDeleteTranscript` | :5953 | Delete transcript | `chat:logs` |
| `HandleListLeads` | :5984 | List leads | `tenant:read` |
| `HandleGetLead` | :6019 | Get lead | `tenant:read` |
| `HandleUpdateLeadStatus` | :6040 | Update lead | `tenant:write` |
| `HandleExportLeads` | :6076 | Export leads CSV | `tenant:read` |
| `HandleGetTenantSettings` | :6147 | Get settings | `tenant:read` |
| `HandleUpdateTenantSettings` | :6177 | Update settings | `tenant:write` |

**Proposed Fix Pattern:**
```go
func (h *Handler) HandleListTranscripts(c echo.Context) error {
    ctx := c.Request().Context()
    slug := c.Param("slug")

    // Get tenant
    tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
    if err != nil || tenant == nil {
        return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
    }

    // ADD: Permission check
    if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatLogs) {
        return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or chat:logs permission")
    }

    // ... rest of handler
}
```

**⚠️ DECISION POINT 5: Lead Permissions**

The review mentions `PermLeadsRead` and `PermLeadsWrite` but these don't exist in the current permission constants. Should we:

| Option | Description | Effort |
|--------|-------------|--------|
| **A (Recommended)** | Use existing `tenant:read`/`tenant:write` | S |
| **B** | Create new `leads:read`/`leads:write` permissions | M |

**My Recommendation:** Option A. Creating fine-grained lead permissions adds complexity without clear benefit at this stage. Use `tenant:read` for lead read operations and `tenant:write` for lead mutations.

---

### Phase 1 Testing Requirements

**Cross-Tenant Test Harness:**
```go
func TestCrossTenantAccess(t *testing.T) {
    // 1. Create tenant A and tenant B
    // 2. Create UserX with RoleUser, scoped to tenant A
    // 3. For each of the 9 handlers:
    //    - Call with tenant B slug → expect 403
    //    - Call with tenant A slug → expect 200
}
```

**Regression Test for Middleware:**
```go
func TestTenantBindingMiddleware(t *testing.T) {
    // RoleUser with tenant A access → tenant A slug → 200
    // RoleUser with tenant A access → tenant B slug → 403
    // ScopedAdmin with tenant A → tenant A slug → 200
    // ScopedAdmin with tenant A → tenant B slug → 403
    // GlobalAdmin → any slug → 200
}
```

---

## Phase 2 — High-Impact Hardening

> **Goal:** Eliminate remaining attack surface (path traversal, CSRF, container, SSRF)
> **Effort:** Medium (2-3 days)
> **Dependency:** None (can parallelize with Phase 1)

---

### H3 — Path Traversal Fix

**Problem:** User-controlled `Filename` is used in storage path template without sanitization, allowing `../../../etc/cron.d/x` to escape the data directory.

**Current Code:**
```go
// server/router/api/v1/resource_service.go:319
internalPath = replaceFilenameWithPathTemplate(internalPath, create.Filename)
// No filepath.Base() or containment check!
```

**Proposed Fix:**
```go
// server/router/api/v1/resource_service.go

func sanitizeFilename(filename string) string {
    // Strip directory components
    filename = filepath.Base(filename)
    // Remove null bytes
    filename = strings.ReplaceAll(filename, "\x00", "")
    // Reject empty filenames
    if filename == "." || filename == ".." || filename == "" {
        return "unnamed"
    }
    return filename
}

func SaveResourceBlob(ctx context.Context, profile *profile.Profile, stores *store.Store, create *store.Resource) error {
    // ... existing code ...

    if workspaceStorageSetting.StorageType == storepb.WorkspaceStorageSetting_LOCAL {
        // ADD: Sanitize filename
        create.Filename = sanitizeFilename(create.Filename)

        filepathTemplate := "assets/{timestamp}_{filename}"
        // ... existing code ...

        osPath := filepath.FromSlash(internalPath)
        if !filepath.IsAbs(osPath) {
            osPath = filepath.Join(profile.Data, osPath)
        }

        // ADD: Containment assertion
        cleanDataDir := filepath.Clean(profile.Data) + string(os.PathSeparator)
        if !strings.HasPrefix(filepath.Clean(osPath), cleanDataDir) {
            return errors.Errorf("path traversal detected: %s", osPath)
        }

        // ... rest of existing code
    }
}
```

**Also fix in:**
- `GetResourceBlob` (read path)
- `DeleteResource` (delete path)
- `memo_resource_service.go:252-263`

**Effort:** Medium (1 day)
**Testing:** Unit tests with `../../../etc/passwd`, absolute paths, null bytes

---

### H4 — CSRF Fix

**Problem:** `SameSite=None` cookie auth with no CSRF token allows cross-site state changes.

**Current Code:**
```go
// server/router/api/v1/auth_service.go:314-316
if isHTTPS {
    attrs = append(attrs, "SameSite=None")
    attrs = append(attrs, "Secure")
}
```

**Proposed Fix:**

**⚠️ DECISION POINT 6: CSRF Protection Strategy**

| Option | Description | Effort | Trade-off |
|--------|-------------|--------|-----------|
| **A (Recommended)** | `SameSite=Lax` everywhere | S | May break cross-origin embeds |
| **B** | `SameSite=Lax` + Origin header check | M | More secure, slightly more complex |
| **C** | Double-submit CSRF token | L | Most secure, highest effort |

**My Recommendation:** Option B. `SameSite=Lax` prevents most CSRF attacks. Adding an `Origin`/`Sec-Fetch-Site` allowlist check on state-changing methods (POST, PUT, PATCH, DELETE) provides defense-in-depth without the complexity of CSRF tokens.

**Implementation:**
```go
// server/router/api/v1/auth_service.go
func (*APIV1Service) buildAccessTokenCookie(ctx context.Context, accessToken string, expireTime time.Time) (string, error) {
    attrs := []string{
        fmt.Sprintf("%s=%s", AccessTokenCookieName, accessToken),
        "Path=/",
        "HttpOnly",
    }
    if expireTime.IsZero() {
        attrs = append(attrs, "Expires=Thu, 01 Jan 1970 00:00:00 GMT")
    } else {
        attrs = append(attrs, "Expires="+expireTime.Format(time.RFC1123))
    }

    // CHANGE: Always use Lax (not None)
    attrs = append(attrs, "SameSite=Lax")

    // Only add Secure flag for HTTPS
    md, ok := metadata.FromIncomingContext(ctx)
    if ok {
        var origin string
        for _, v := range md.Get("origin") {
            origin = v
        }
        if strings.HasPrefix(origin, "https://") {
            attrs = append(attrs, "Secure")
        }
    }

    return strings.Join(attrs, "; "), nil
}
```

**Add Origin validation middleware:**
```go
func CSRFProtectionMiddleware() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // Skip safe methods
            if c.Request().Method == "GET" || c.Request().Method == "HEAD" || c.Request().Method == "OPTIONS" {
                return next(c)
            }

            // Check Origin header
            origin := c.Request().Header.Get("Origin")
            if origin == "" {
                // Allow same-site requests without Origin (e.g., form submissions)
                return next(c)
            }

            // Validate origin against allowlist
            if !isAllowedOrigin(origin) {
                return echo.NewHTTPError(http.StatusForbidden, "CSRF validation failed")
            }

            return next(c)
        }
    }
}
```

---

### H5 — Non-Root Container

**Problem:** Container runs as root, so any RCE/container escape runs as root.

**Current Code:**
```dockerfile
# Dockerfile.fly (no USER directive)
ENTRYPOINT ["./entrypoint.sh", "./memos"]
```

**Proposed Fix:**
```dockerfile
# Dockerfile.fly

# Stage 3: Minimal runtime image
FROM debian:bookworm-slim

WORKDIR /usr/local/memos

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

# Copy LanceDB shared library for runtime
COPY --from=backend /usr/local/lib/lancedb/liblancedb_go.so /usr/local/lib/
RUN ldconfig

# Copy application binary and scripts
COPY --from=backend /backend-build/memos .
COPY scripts/entrypoint.sh .
RUN chmod +x entrypoint.sh

# Copy widget bundle for external embeds
COPY --from=frontend /widget-build/dist ./widget/dist

# ADD: Create non-root user
RUN groupadd -r memos && useradd -r -g memos -d /usr/local/memos -s /sbin/nologin memos

# Create data directory for SQLite and local storage
RUN mkdir -p /var/opt/memos && chown memos:memos /var/opt/memos
VOLUME /var/opt/memos

# ADD: Switch to non-root user
USER memos

# ... rest of file
```

**Also fix:** `Dockerfile.s3.fly` (same pattern)

**⚠️ DECISION POINT 7: Volume Permissions**

Fly.io volume mounts may have specific permission requirements.

| Option | Description | Effort | Risk |
|--------|-------------|--------|------|
| **A (Recommended)** | Test with non-root user, adjust if needed | S | May need Fly config changes |
| **B** | Use entrypoint script to chown at runtime | S | Requires root at startup |

**My Recommendation:** Option A. Test the deployment with the non-root user. If Fly volumes have permission issues, we can add a one-time `chown` in the entrypoint script (running as root initially, then dropping privileges).

**Entrypoint script update:**
```bash
#!/bin/sh
# scripts/entrypoint.sh

# If running as root, fix permissions and drop privileges
if [ "$(id -u)" = '0' ]; then
    chown -R memos:memos /var/opt/memos
    exec gosu memos "$@"
fi

exec "$@"
```

---

### H6 — Webhook SSRF Fix

**Problem:** User-supplied webhook URLs get no scheme allowlist, no internal-IP block, and no redirect restriction.

**Current Code:**
```go
// server/router/api/v1/webhook_service.go:24-28
webhook, err := s.Store.CreateWebhook(ctx, &store.Webhook{
    CreatorID: currentUser.ID,
    Name:      request.Name,
    URL:       strings.TrimSpace(request.Url),  // No validation!
})
```

**Proposed Fix:**
```go
// server/router/api/v1/webhook_service.go

import (
    "net"
    "net/url"
)

func validateWebhookURL(rawURL string) error {
    parsed, err := url.Parse(rawURL)
    if err != nil {
        return fmt.Errorf("invalid URL: %w", err)
    }

    // 1. Scheme allowlist
    if parsed.Scheme != "http" && parsed.Scheme != "https" {
        return fmt.Errorf("only http/https schemes allowed, got: %s", parsed.Scheme)
    }

    // 2. Resolve host and check for internal IPs
    hostname := parsed.Hostname()
    if hostname == "" {
        return fmt.Errorf("missing hostname")
    }

    // Resolve DNS
    addrs, err := net.LookupHost(hostname)
    if err != nil {
        return fmt.Errorf("failed to resolve hostname: %w", err)
    }

    for _, addr := range addrs {
        ip := net.ParseIP(addr)
        if ip == nil {
            continue
        }

        // Block internal IPs
        if isInternalIP(ip) {
            return fmt.Errorf("internal/private IPs not allowed: %s", addr)
        }
    }

    return nil
}

func isInternalIP(ip net.IP) bool {
    // Loopback
    if ip.IsLoopback() {
        return true
    }
    // Private ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
    if ip.IsPrivate() {
        return true
    }
    // Link-local
    if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
        return true
    }
    // Unspecified
    if ip.IsUnspecified() {
        return true
    }
    // Cloud metadata IPs
    metadataIPs := []string{"169.254.169.254", "fd00:ec2::254"}
    for _, mIP := range metadataIPs {
        if ip.Equal(net.ParseIP(mIP)) {
            return true
        }
    }
    return false
}

// Update CreateWebhook
func (s *APIV1Service) CreateWebhook(ctx context.Context, request *v1pb.CreateWebhookRequest) (*v1pb.Webhook, error) {
    currentUser, err := s.GetCurrentUser(ctx)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
    }

    // ADD: Validate URL
    if err := validateWebhookURL(request.Url); err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid webhook URL: %v", err)
    }

    webhook, err := s.Store.CreateWebhook(ctx, &store.Webhook{
        CreatorID: currentUser.ID,
        Name:      request.Name,
        URL:       strings.TrimSpace(request.Url),
    })
    // ...
}
```

**Also update:** `UpdateWebhook` to validate URL on update.

**Add redirect restriction at dispatch time:**
```go
// plugin/webhook/webhook.go
client := &http.Client{
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        if len(via) >= 3 {
            return fmt.Errorf("too many redirects")
        }
        // Re-validate redirect target
        return validateWebhookURL(req.URL.String())
    },
    Timeout: 10 * time.Second,
}
```

**Effort:** Medium (1-2 days)
**Testing:** Unit tests with `http://169.254.169.254/`, `http://localhost:8080/`, `http://10.0.0.1/`, `http://[::1]/`

---

## Implementation Order & Dependencies

```
Phase 0 (Today)
    ├── H7: Rotate credentials ─────────────────────────┐
    ├── C2: Gate pprof ─────────────────────────────────┤
    └── C3: Fix debug mode ─────────────────────────────┘
                                                        │
Phase 1 (This Sprint)                                   │
    ├── H2: Unify super-user ───────────────────────────┤
    ├── H1: Fix TenantBindingMiddleware ◄────────────────┘
    └── C1: Add permission guards (depends on H1)
                                                        │
Phase 2 (Next Sprint)                                   │
    ├── H3: Path traversal fix ─────────────────────────┐
    ├── H4: CSRF fix ───────────────────────────────────┤
    ├── H5: Non-root container ─────────────────────────┤
    └── H6: Webhook SSRF fix ───────────────────────────┘
```

**Critical Path:** H7 → H2 → H1 → C1 (Phase 0 must complete before Phase 1 for credential safety)

---

## Verification Checklist

After implementation, verify:

- [ ] **Phase 0:**
  - [ ] New OpenRouter key works
  - [ ] New encryption key encrypts/decrypts correctly
  - [ ] `/debug/pprof` returns 404 in production
  - [ ] Error responses in prod show generic messages only

- [ ] **Phase 1:**
  - [ ] Tenant-A `RoleUser` gets 403 on tenant-B slug for all 9 handlers
  - [ ] Scoped admin gets 403 on non-assigned tenant
  - [ ] Global admin can access all tenants
  - [ ] Cross-tenant test suite passes

- [ ] **Phase 2:**
  - [ ] Filename `../../../etc/passwd` is sanitized to `passwd`
  - [ ] Webhook URL `http://169.254.169.254/` is rejected
  - [ ] Container runs as non-root (`id` shows `memos` user)
  - [ ] Cross-origin POST gets 403 without valid Origin

---

## Open Questions for You

1. **H7:** Do you have access to rotate the OpenRouter key and Tigris credentials now? (Option A/B/C for master key rotation)

2. **H1:** Is there an existing `agent_user_permissions` table we can query for tenant access checks, or do we need to create one?

3. **H2:** Should `RoleHost` be deprecated entirely, or kept as a separate role?

4. **H4:** Do you have cross-origin embedding use cases that require `SameSite=None`? (e.g., widget embedded on customer sites)

5. **H5:** Are you using Fly.io volumes with specific permission requirements?

6. **Timeline:** Should I proceed with implementation after your review, or do you want to discuss any trade-offs first?

---

*This plan is ready for your review. Please confirm or adjust the decisions above before implementation begins.*
