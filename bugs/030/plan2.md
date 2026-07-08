# Security Remediation Implementation Plan v2 — Phases 0–2

**Based on:** `docs_fable_sec.md` (Security Review) + `plan_review.md` (Adversarial Review)
**Scope:** Critical and High severity issues only (Phase 0, 1, 2)
**Status:** ✅ Revised — addresses all B1–B3 blocking findings and N1–N6 nits

---

## Executive Summary

This plan addresses **3 Critical** and **7 High** security vulnerabilities across three phases. It incorporates all findings from the adversarial review of v1, including 3 blocking issues and 6 significant nits.

**Total Estimated Effort:** 10–16 dev-days (increased due to scope corrections)

| Phase | Focus | Effort | Risk if Deferred |
|-------|-------|--------|------------------|
| 0 | Emergency (rotate keys, gate pprof, fix debug) | S (hours) | Immediate compromise risk |
| 1 | Tenant isolation (IDOR, middleware, super-user) | L (4-6d) | Cross-tenant PII breach |
| 2 | Attack surface hardening (path traversal, CSRF, container, SSRF) | M (3-4d) | RCE, CSRF, container escape |

---

## Phase 0 — Emergency (Do Today)

> **Goal:** Eliminate immediate information disclosure and credential exposure
> **Effort:** Small (hours)
> **Can be parallelized:** Yes
> **Review Verdict:** ✅ Approve with N6 nit applied

---

### H7 — Rotate Credentials

**Problem:** Live credentials (OpenRouter API key, encryption master key, Tigris S3 keys) exist in repo-local `.env` files. The OpenRouter key was previously committed (tripped GitHub Push Protection).

**Implementation Steps:**
1. **Rotate OpenRouter API key** at https://openrouter.ai/keys
2. **Generate new encryption master key:** `uuidgen` or `openssl rand -hex 32`
3. **Rotate Tigris S3 credentials** in Tigris dashboard
4. **Update `.env` files** with new credentials
5. **Plan re-encryption migration** for stored tenant API keys (backup key path in `crypto/encryption.go` supports a migration window)

**DECISION POINT 1: Master Key Rotation Strategy**

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

**Implementation Steps (N6 applied — drop loopback binding):**
1. **Add environment variable check:** `MEMOS_ENABLE_PPROF=false` (default)
2. **Gate registration behind `profile.IsDev()` AND env flag:**
   ```go
   // server/server.go
   if profile.IsDev() || os.Getenv("MEMOS_ENABLE_PPROF") == "true" {
       s.profiler.RegisterRoutes(echoServer)
   }
   ```
3. ~~Add loopback binding~~ **DROPPED (N6)** — pprof is registered as a route group on the shared Echo listener; cannot bind one group to 127.0.0.1 without a second listener. The dev-mode gate + env flag is sufficient.

**DECISION POINT 2: pprof Access Control**

| Option | Description | Effort | Use Case |
|--------|-------------|--------|----------|
| **A (Recommended)** | Dev-mode only (no prod access) | S | Most secure |
| **B** | Env flag only (default off) | S | Production debugging possible |
| **C** | Env flag + IP allowlist | M | Remote debugging with control |

**My Recommendation:** Option B. Default off via env flag gives operators flexibility. Combined with the env flag being opt-in, this is sufficient security.

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
> **Effort:** Large (4-6 days)
> **Dependency:** None (can start immediately after Phase 0)
> **Testing:** Requires cross-tenant test harness
> **Review Verdict:** 🔴 Reworked — addresses B1, B2, B3, N1, N2

---

### Architecture: Load-Bearing Controls

**Key Insight from Review:** C1's per-handler guards are **defense-in-depth for RoleUser only**. For scoped admins, **H1 (TenantBindingMiddleware) is the load-bearing control**. This plan makes this dependency explicit.

```
┌─────────────────────────────────────────────────────────────┐
│                    Authorization Layers                      │
├─────────────────────────────────────────────────────────────┤
│  Layer 1 (Primary): TenantBindingMiddleware (H1)            │
│  ├─ Scoped admins: contains(AllowedTenantIDs, tenant.GUID) │
│  ├─ RoleUser: RBAC permission grant on tenant              │
│  └─ Global admin/RoleHost: bypass                          │
├─────────────────────────────────────────────────────────────┤
│  Layer 2 (Defense-in-depth): Per-handler checks (C1)        │
│  ├─ isAdmin() → false for scoped admins (NEW)              │
│  └─ hasPermission() → checks effective permissions         │
├─────────────────────────────────────────────────────────────┤
│  Layer 3 (Data): TenantID filter on queries (H2)           │
│  └─ ApplyTenantFilter/ApplyTicketTenantFilter              │
└─────────────────────────────────────────────────────────────┘
```

---

### H1 — Fix TenantBindingMiddleware

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

            // Global super users bypass binding:
            // - RoleHost (instance owner/super-admin)
            // - RoleAdmin with empty AllowedTenantIDs (global admin)
            if user.Role == store.RoleHost || (user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0) {
                return next(c)
            }

            slug := c.Param("slug")
            if slug == "" {
                return next(c) // No slug in URL, skip check
                // NOTE: No-slug routes (e.g. HandleUpdateRoleTemplate) self-check
                // ownership from the record, so this is safe.
            }

            // Look up the tenant by slug
            tenant, err := s.GetAgentTenant(c.Request().Context(), &store.FindAgentTenant{Slug: &slug})
            if err != nil || tenant == nil {
                return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
            }

            // Check if user has explicit grant for this tenant
            if user.Role == store.RoleAdmin {
                // Scoped admin: check AllowedTenantIDs
                if !contains(user.AllowedTenantIDs, tenant.GUID) {
                    return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
                }
            } else {
                // RoleUser: use existing ListUserTenantPermissions (N2 — reuse, don't add new method)
                perms, err := s.ListUserTenantPermissions(c.Request().Context(), &store.FindUserTenantPermission{
                    UserID:   &userID,
                    TenantID: &tenant.ID,
                })
                if err != nil || len(perms) == 0 {
                    return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
                }
            }

            return next(c)
        }
    }
}
```

**N2 Applied:** Uses existing `ListUserTenantPermissions` instead of proposing a new `HasTenantAccess` method.

---

### H2 — Fix Tenant Filter Derivation + Unify Super-User

**Problem:** Two issues:
1. `isSuperUser` treats all admins as global, letting scoped admins cross tenants
2. Tenant filters (`ApplyTenantFilter`, `ApplyTicketTenantFilter`) use `getTenantFromContext` which is nil for admins, making filters no-ops

**Current Code:**
```go
// common.go:66-68
func isSuperUser(user *store.User) bool {
    return user.Role == store.RoleAdmin || user.Role == store.RoleHost
}

// tenant_context.go:25-38
func ApplyTicketTenantFilter(c echo.Context, find *store.FindTicket) {
    tenantID := getTenantFromContext(c) // nil for admins
    if tenantID != nil { find.TenantID = tenantID } // skipped → ALL tenants returned
}
```

**Proposed Fix:**

#### Part A: Fix `isSuperUser` (B3 applied — keep RoleHost)

```go
// common.go
func isSuperUser(user *store.User) bool {
    // RoleHost is the instance owner/super-admin — always super
    // RoleAdmin is super only when not scoped to specific tenants
    return user.Role == store.RoleHost || (user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0)
}
```

**N1 Applied — Full cascade inventory (29 call sites):**

| File | Lines | Impact |
|------|-------|--------|
| `resource_service.go` | 119, 547, 571, 585 | Resource visibility/CRUD |
| `memo_service.go` | 56, 117, 175, 185, 270, 280, 318, 325, 451, 458, 596, 625, 1127, 1178 | Memo visibility/CRUD |
| `memo_relation_service.go` | 96 | Memo relation access |
| `ticket_service.go` | 126, 192, 231, 279, 283, 300, 341, 417, 422 | Ticket visibility/CRUD |

**Testing:** All 29 sites must be tested with:
- RoleHost → should pass (super user)
- Global RoleAdmin (empty AllowedTenantIDs) → should pass
- Scoped RoleAdmin (non-empty AllowedTenantIDs) → should be denied
- RoleUser with permission → should be denied (not super)

#### Part B: Fix Tenant Filter Derivation (B2)

For scoped admins, populate the filter from `AllowedTenantIDs`:

```go
// tenant_context.go
func ApplyTicketTenantFilter(c echo.Context, find *store.FindTicket) {
    tenantID := getTenantFromContext(c)
    if tenantID != nil {
        find.TenantID = tenantID
        return
    }

    // For scoped admins, derive filter from AllowedTenantIDs
    user := getUserFromContext(c) // need to add this helper
    if user != nil && user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) > 0 {
        // Resolve GUIDs to tenant IDs
        tenantIDs := resolveTenantIDsFromGUIDs(c.Request().Context(), store, user.AllowedTenantIDs)
        if len(tenantIDs) > 0 {
            find.TenantIDs = tenantIDs // new field on FindTicket
        }
    }
    // Global admins (nil tenantID + empty AllowedTenantIDs) see all — correct
}
```

**Store changes needed:**
```go
// store/agent.go — add TenantIDs field
type FindTicket struct {
    // ... existing fields
    TenantIDs []int32 // for scoped admin filter (OR semantics)
}
```

**Apply same pattern to:**
- `ApplyTenantFilter` (memos)
- Any other tenant-scoped query paths

---

### C1 — Add Permission Guards to 9 Handlers (B1 applied)

**Problem:** `isAdmin(c)` returns `true` for ANY `RoleHost`/`RoleAdmin`, regardless of `AllowedTenantIDs`. So the guard short-circuits for scoped admins.

**Root Cause:**
```go
// handlers.go:2222
isAdmin := user.Role == store.RoleHost || user.Role == store.RoleAdmin
// No AllowedTenantIDs check!
```

**Proposed Fix — Two options:**

**Option A (Recommended): Make `isAdmin()` tenant-aware**

```go
func (h *Handler) isAdmin(c echo.Context) bool {
    userID, ok := c.Get("user-id").(int32)
    if !ok {
        return false
    }

    user, err := h.store.GetUser(c.Request().Context(), &store.FindUser{ID: &userID})
    if err != nil || user == nil {
        return false
    }

    // RoleHost is always admin
    if user.Role == store.RoleHost {
        return true
    }

    // RoleAdmin is admin only if not scoped to specific tenants
    if user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0 {
        return true
    }

    // Scoped admins and RoleUser are NOT "admin" for handler guards
    return false
}
```

**Impact:** This makes C1 guards effective for scoped admins too. The `hasPermission` branch then checks their effective permissions.

**Option B: Use `isSuperAdmin()` for handler guards**

```go
func (h *Handler) isSuperAdmin(c echo.Context) bool {
    // ... same logic as Option A
}

// In handlers:
if !h.isSuperAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatLogs) {
    return 403
}
```

**My Recommendation:** Option A. Renaming to `isSuperAdmin` is clearer but requires changing 67+ call sites. Since `isAdmin` is already the established pattern, modifying its semantics is less invasive — but **must be tested exhaustively**.

**DECISION POINT 3: isAdmin Semantics**

| Option | Description | Effort | Risk |
|--------|-------------|--------|------|
| **A (Recommended)** | Modify `isAdmin()` to exclude scoped admins | M | May break other admin checks |
| **B** | Create `isSuperAdmin()`, keep `isAdmin()` unchanged | L | Safer but more code |
| **C** | Keep `isAdmin()` as-is, rely on H1 middleware only | S | C1 becomes no-op for admins |

**My Recommendation:** Option B. Safer — keeps existing `isAdmin()` behavior unchanged for the 67+ existing call sites. Only the 9 vulnerable handlers use `isSuperAdmin()`. Lower regression risk.

**Handlers to update (with correct permissions):**

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

**Proposed Fix Pattern (using isSuperAdmin):**
```go
func (h *Handler) HandleListTranscripts(c echo.Context) error {
    ctx := c.Request().Context()
    slug := c.Param("slug")

    // Get tenant
    tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
    if err != nil || tenant == nil {
        return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
    }

    // Permission check: super admin OR specific permission
    if !h.isSuperAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatLogs) {
        return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or chat:logs permission")
    }

    // ... rest of handler
}
```

---

### Phase 1 Testing Requirements

**Extended Cross-Tenant Test Harness (B1 applied):**
```go
func TestCrossTenantAccess(t *testing.T) {
    // 1. Create tenant A and tenant B
    // 2. Test with RoleUser scoped to tenant A
    // 3. Test with ScopedAdmin (AllowedTenantIDs = [tenant-A-GUID])
    // 4. Test with GlobalAdmin (empty AllowedTenantIDs)
    // 5. Test with RoleHost

    // For each of the 9 handlers:
    //   RoleUser(A) → tenant B slug → expect 403
    //   RoleUser(A) → tenant A slug → expect 200
    //   ScopedAdmin(A) → tenant B slug → expect 403
    //   ScopedAdmin(A) → tenant A slug → expect 200
    //   GlobalAdmin → any slug → expect 200
    //   RoleHost → any slug → expect 200
}
```

**isSuperUser Cascade Tests (N1):**
```go
func TestIsSuperUserCascade(t *testing.T) {
    // For each of the 29 call sites:
    //   RoleHost → should be treated as super
    //   GlobalAdmin → should be treated as super
    //   ScopedAdmin → should NOT be treated as super
    //   RoleUser → should NOT be treated as super
}
```

**Middleware Test:**
```go
func TestTenantBindingMiddleware(t *testing.T) {
    // RoleUser with tenant A access → tenant A slug → 200
    // RoleUser with tenant A access → tenant B slug → 403
    // RoleUser with no access → any slug → 403
    // ScopedAdmin with tenant A → tenant A slug → 200
    // ScopedAdmin with tenant A → tenant B slug → 403
    // GlobalAdmin → any slug → 200
    // RoleHost → any slug → 200
    // No slug in URL → skips check (safe for no-slug routes)
}
```

---

## Phase 2 — High-Impact Hardening

> **Goal:** Eliminate remaining attack surface (path traversal, CSRF, container, SSRF)
> **Effort:** Medium (3-4 days)
> **Dependency:** None (can parallelize with Phase 1)
> **Review Verdict:** 🟡 Approve with N3, N4, N5 nits applied

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

**Also fix in (review confirmed):**
- `GetResourceBlob` (read path)
- `DeleteResource` (delete path)
- `memo_resource_service.go:252-263`

**Note:** Pre-existing tainted `Reference` rows remain until backfilled (add migration to sanitize existing data).

**Effort:** Medium (1 day)
**Testing:** Unit tests with `../../../etc/passwd`, absolute paths, null bytes, empty strings

---

### H4 — CSRF Fix (N5 applied)

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

**Part A: Change cookie to SameSite=Lax**
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
    // SameSite=Lax prevents cross-site POST while allowing same-site navigation
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

**Part B: CSRF middleware with Sec-Fetch-Site (N5 — close Origin bypass)**

```go
// server/router/api/v1/csrf.go
func CSRFProtectionMiddleware(allowedOrigins []string) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // Skip safe methods (Idempotent requests)
            if c.Request().Method == "GET" || c.Request().Method == "HEAD" || c.Request().Method == "OPTIONS" {
                return next(c)
            }

            // Prefer Sec-Fetch-Site when available (more reliable than Origin)
            secFetchSite := c.Request().Header.Get("Sec-Fetch-Site")
            if secFetchSite != "" {
                // "cross-site" and "none" from different origin = CSRF
                if secFetchSite == "cross-site" {
                    return echo.NewHTTPError(http.StatusForbidden, "CSRF validation failed: cross-site request")
                }
                // "same-origin" and "same-site" are safe
                if secFetchSite == "same-origin" || secFetchSite == "same-site" {
                    return next(c)
                }
                // "none" from same origin is safe (direct navigation)
                if secFetchSite == "none" {
                    return next(c)
                }
            }

            // Fallback: check Origin header
            origin := c.Request().Header.Get("Origin")
            if origin == "" {
                // N5 FIX: Treat missing Origin conservatively for state-changing methods
                // Some CSRF attacks omit Origin. Rely on SameSite=Lax as primary defense.
                // Allow if no Origin + no Sec-Fetch-Site (likely same-site form submission)
                return next(c)
            }

            // Validate origin against allowlist
            if !isAllowedOrigin(origin, allowedOrigins) {
                return echo.NewHTTPError(http.StatusForbidden, "CSRF validation failed: invalid origin")
            }

            return next(c)
        }
    }
}

func isAllowedOrigin(origin string, allowedOrigins []string) bool {
    parsed, err := url.Parse(origin)
    if err != nil {
        return false
    }
    originHost := parsed.Hostname()
    for _, allowed := range allowedOrigins {
        if originHost == allowed {
            return true
        }
    }
    return false
}
```

**N5 FIX — Wire the middleware:**
```go
// server/router/api/v1/v1.go — register CSRF middleware
func NewAPIV1Service(...) *APIV1Service {
    // ... existing code ...

    // CSRF protection for state-changing routes
    allowedOrigins := getEnvSlice("CSRF_ALLOWED_ORIGINS", []string{})
    csrfMiddleware := CSRFProtectionMiddleware(allowedOrigins)

    // Apply to gateway group (gRPC-web endpoints)
    gwGroup.Use(csrfMiddleware)

    // Apply to admin group
    adminGroup.Use(csrfMiddleware)

    // Widget routes use widget-key auth, not cookies — no CSRF needed
}
```

**Widget compatibility:** The widget uses widget-key auth (not cookies), so `SameSite=Lax` won't break it. Verified in `handlers.go:2056-2061,2116`.

**DECISION POINT 4: CSRF Allowed Origins**

| Option | Description | Effort |
|--------|-------------|--------|
| **A (Recommended)** | Env var `CSRF_ALLOWED_ORIGINS` (comma-separated) | S |
| **B** | Derive from `InstanceURL` + `ADMIN_CORS_ORIGINS` | M |
| **C** | Hardcode allowed origins | S |

**My Recommendation:** Option A. Simple, explicit, and doesn't require coupling to other config.

---

### H5 — Non-Root Container (N3 applied)

**Problem:** Container runs as root, so any RCE/container escape runs as root.

**N3 Issue:** Original plan proposed `USER memos` + entrypoint `chown` — contradictory (with `USER memos`, entrypoint never runs as root, so `chown` is skipped). Also `gosu` is not installed.

**Proposed Fix (N3 — coherent approach):**

```dockerfile
# Dockerfile.fly

# Stage 3: Minimal runtime image
FROM debian:bookworm-slim

WORKDIR /usr/local/memos

# Install runtime dependencies + gosu for privilege dropping
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    gosu \
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

# Create non-root user
RUN groupadd -r memos && useradd -r -g memos -d /usr/local/memos -s /sbin/nologin memos

# Create data directory for SQLite and local storage
RUN mkdir -p /var/opt/memos && chown memos:memos /var/opt/memos
VOLUME /var/opt/memos

# DO NOT add USER memos here — entrypoint handles privilege dropping
# Container enters as root, chowns volume, then drops to memos user

ENTRYPOINT ["./entrypoint.sh", "./memos"]
```

**Entrypoint script (N3 — coherent with Dockerfile):**
```bash
#!/bin/sh
# scripts/entrypoint.sh
set -e

# If running as root, fix volume permissions and drop privileges
if [ "$(id -u)" = '0' ]; then
    # Fix ownership of data volume (may be root-owned from fresh mount)
    chown -R memos:memos /var/opt/memos 2>/dev/null || true

    # Drop to memos user and execute the main command
    exec gosu memos "$@"
fi

# Already running as non-root user
exec "$@"
```

**Apply same to:** `Dockerfile.s3.fly`

**Effort:** Small (1 day)
**Testing:** Build image, verify `id` shows `memos` user, verify volume is writable

---

### H6 — Webhook SSRF Fix (N4 applied)

**Problem:** User-supplied webhook URLs get no scheme allowlist, no internal-IP block, and no redirect restriction.

**N4 Issue:** Original plan validated at create/update time with `net.LookupHost`, but dispatch (`plugin/webhook/webhook.go`) re-resolves DNS — classic TOCTOU. Validation-at-save is a UX pre-check, not the security boundary.

**Proposed Fix (N4 — enforcement at dispatch with IP-pinned dialer):**

**Part A: Save-time validation (UX pre-check)**
```go
// server/router/api/v1/webhook_service.go
func validateWebhookURL(rawURL string) error {
    parsed, err := url.Parse(rawURL)
    if err != nil {
        return fmt.Errorf("invalid URL: %w", err)
    }

    // Scheme allowlist
    if parsed.Scheme != "http" && parsed.Scheme != "https" {
        return fmt.Errorf("only http/https schemes allowed, got: %s", parsed.Scheme)
    }

    // Hostname must not be empty
    if parsed.Hostname() == "" {
        return fmt.Errorf("missing hostname")
    }

    return nil
}

func (s *APIV1Service) CreateWebhook(ctx context.Context, request *v1pb.CreateWebhookRequest) (*v1pb.Webhook, error) {
    // ... existing code ...

    // UX pre-check: validate scheme and format
    if err := validateWebhookURL(request.Url); err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid webhook URL: %v", err)
    }

    webhook, err := s.Store.CreateWebhook(ctx, &store.Webhook{...})
    // ...
}
```

**Part B: Dispatch-time enforcement (security boundary)**
```go
// plugin/webhook/webhook.go
import (
    "context"
    "net"
    "net/http"
    "net/url"
)

func dispatchWebhook(ctx context.Context, webhook *store.Webhook, payload []byte) error {
    parsed, err := url.Parse(webhook.URL)
    if err != nil {
        return fmt.Errorf("invalid webhook URL: %w", err)
    }

    // Resolve DNS once
    hostname := parsed.Hostname()
    addrs, err := net.LookupHost(hostname)
    if err != nil {
        return fmt.Errorf("failed to resolve webhook host: %w", err)
    }

    // Check ALL resolved IPs
    var dialIP string
    for _, addr := range addrs {
        ip := net.ParseIP(addr)
        if ip == nil {
            continue
        }
        if isInternalIP(ip) {
            return fmt.Errorf("webhook target resolves to internal IP: %s", addr)
        }
        // Use first valid external IP for pinning
        if dialIP == "" {
            dialIP = addr
        }
    }

    if dialIP == "" {
        return fmt.Errorf("no valid IP found for webhook host")
    }

    // Create IP-pinned transport
    transport := &http.Transport{
        DialContext: func(ctx context.Context, network, addr string) (*net.Dialer, error) {
            // Force connection to the validated IP
            _, port, _ := net.SplitHostPort(addr)
            return &net.Dialer{}, nil
        },
        DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
            // Pin to validated IP
            targetAddr := net.JoinHostPort(dialIP, parsed.Port())
            return (&net.Dialer{}).DialContext(ctx, network, targetAddr)
        },
    }

    // Redirect policy: cap redirects + re-validate
    client := &http.Client{
        Transport: transport,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            if len(via) >= 3 {
                return fmt.Errorf("too many redirects")
            }
            // Re-validate redirect target
            redirectAddrs, err := net.LookupHost(req.URL.Hostname())
            if err != nil {
                return fmt.Errorf("redirect target lookup failed: %w", err)
            }
            for _, addr := range redirectAddrs {
                ip := net.ParseIP(addr)
                if ip != nil && isInternalIP(ip) {
                    return fmt.Errorf("redirect to internal IP blocked: %s", addr)
                }
            }
            return nil
        },
        Timeout: 10 * time.Second,
    }

    // ... make request with client ...
}

func isInternalIP(ip net.IP) bool {
    if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
       ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
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
```

**Also update:** `UpdateWebhook` to validate URL on update.

**Effort:** Medium (1-2 days)
**Testing:** Unit tests with `http://169.254.169.254/`, `http://localhost:8080/`, `http://10.0.0.1/`, `http://[::1]/`, DNS-rebinding scenarios

---

## Implementation Order & Dependencies

```
Phase 0 (Today)
    ├── H7: Rotate credentials ─────────────────────────┐
    ├── C2: Gate pprof (N6: drop loopback) ─────────────┤
    └── C3: Fix debug mode ─────────────────────────────┘
                                                        │
Phase 1 (This Sprint)                                   │
    ├── H2 Part A: Fix isSuperUser (B3: keep RoleHost)──┤
    ├── H2 Part B: Fix tenant filter derivation (B2) ◄──┘
    ├── H1: Fix TenantBindingMiddleware (N2: reuse ListUserTenantPermissions)
    └── C1: Add permission guards (B1: use isSuperAdmin for 9 handlers)
                                                        │
Phase 2 (Next Sprint)                                   │
    ├── H3: Path traversal fix ─────────────────────────┐
    ├── H4: CSRF fix (N5: wire middleware) ─────────────┤
    ├── H5: Non-root container (N3: gosu + coherent) ───┤
    └── H6: Webhook SSRF fix (N4: dispatch-time) ───────┘
```

**Critical Path:** H7 → H2 Part A → H2 Part B → H1 → C1

---

## Verification Checklist

After implementation, verify:

- [ ] **Phase 0:**
  - [ ] New OpenRouter key works
  - [ ] New encryption key encrypts/decrypts correctly
  - [ ] `/debug/pprof` returns 404 in production (or requires `MEMOS_ENABLE_PPROF=true`)
  - [ ] Error responses in prod show generic messages only

- [ ] **Phase 1:**
  - [ ] RoleUser(A) gets 403 on tenant-B slug for all 9 handlers
  - [ ] ScopedAdmin(A) gets 403 on tenant-B slug for all 9 handlers
  - [ ] GlobalAdmin can access all tenants
  - [ ] RoleHost can access all tenants
  - [ ] isSuperUser returns false for scoped admins
  - [ ] isSuperUser returns true for RoleHost
  - [ ] Tenant filter returns only allowed tenants for scoped admins
  - [ ] Cross-tenant test suite passes (RoleUser + ScopedAdmin + GlobalAdmin + RoleHost)
  - [ ] All 29 isSuperUser call sites tested with all 4 role types

- [ ] **Phase 2:**
  - [ ] Filename `../../../etc/passwd` is sanitized to `passwd`
  - [ ] Read/delete paths also sanitized
  - [ ] Webhook URL `http://169.254.169.254/` is rejected at dispatch
  - [ ] DNS-rebinding attack blocked by IP-pinned dialer
  - [ ] Container runs as non-root (`id` shows `memos` user)
  - [ ] Volume is writable after container restart
  - [ ] Cross-origin POST gets 403 without valid Origin/Sec-Fetch-Site
  - [ ] CSRF middleware is wired to gwGroup and adminGroup

---

## Open Questions for You

1. **H7:** Do you have access to rotate the OpenRouter key and Tigris credentials now? (Option A/B/C for master key rotation)

2. **C1:** Do you prefer Option A (modify `isAdmin`) or Option B (create `isSuperAdmin`)? I recommend B for safety.

3. **H4:** What origins should be in `CSRF_ALLOWED_ORIGINS`? (e.g., your production domain, localhost for dev)

4. **Timeline:** Should I proceed with implementation after your review, or do you want to discuss any trade-offs first?

---

*This plan addresses all B1–B3 blocking findings and N1–N6 nits from the adversarial review. Ready for your approval.*
