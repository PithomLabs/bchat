# Security Remediation — Implementation Documentation

**Date:** 2026-07-09
**Based on:** `docs_fable_sec.md` (Security Review) + `plan_review.md` (Adversarial Review)
**Status:** ✅ Implemented — Phases 0–2 complete, build passes, tests pass

---

## Files Modified

| File | Change | Review Findings Applied |
|------|--------|------------------------|
| `.env` | Replaced live credentials with placeholders | H7 |
| `bugs/026/s3_probe/.env` | Replaced live S3 credentials with placeholders | H7 |
| `server/server.go` | Gated pprof, fixed debug mode, custom error handler | C2, C3, N6 |
| `server/router/api/v1/common.go` | Fixed `isSuperUser` definition | H2 Part A, B3 |
| `server/router/api/v1/tenant_context.go` | Added `deriveTenantIDsForScopedAdmin`, updated filter functions | H2 Part B, B2, M2 |
| `server/router/api/v1/tenant_binding.go` | Enforced binding for all roles (RoleUser, scoped admins) | H1, N2 |
| `server/router/api/v1/agent/handlers.go` | Added `isSuperAdmin`, permission guards to 9 handlers | C1, B1 |
| `server/router/api/v1/agent/permissions.go` | (unchanged — existing constants reused) | N/A |
| `server/router/api/v1/csrf.go` | New file: CSRF middleware with Sec-Fetch-Site | H4, N5, M4 |
| `server/router/api/v1/auth_service.go` | Changed cookie to SameSite=Lax | H4 |
| `server/router/api/v1/v1.go` | Wired CSRF middleware to gwGroup + adminGroup | H4, N5 |
| `server/router/api/v1/resource_service.go` | Added `sanitizeFilename` + containment assertion | H3 |
| `server/router/api/v1/webhook_service.go` | Added URL validation on create/update | H6 |
| `plugin/webhook/webhook.go` | IP-pinned dialer + internal IP blocking at dispatch | H6, N4, M1 |
| `store/agent.go` | Added `GUID` field to `FindAgentTenant` | H2 Part B |
| `store/ticket.go` | Added `TenantIDs` field to `FindTicket` | H2 Part B, M2 |
| `store/memo.go` | Added `TenantIDs` field to `FindMemo` | H2 Part B, M2 |
| `store/db/sqlite/agent.go` | Added GUID filter to `ListAgentTenants` | H2 Part B |
| `store/db/sqlite/ticket.go` | Added TenantIDs filter to `ListTickets` | M2 |
| `store/db/sqlite/memo.go` | Added TenantIDs filter to `ListMemos` | M2 |
| `store/db/postgres/agent.go` | Added GUID filter to `ListAgentTenants` | H2 Part B |
| `store/db/postgres/ticket.go` | Added TenantIDs filter to `ListTickets` | M2 |
| `store/db/postgres/memo.go` | Added TenantIDs filter to `ListMemos` | M2 |
| `Dockerfile.fly` | Non-root user via gosu, entrypoint handles privilege drop | H5, N3, M3 |
| `Dockerfile.s3.fly` | Same as Dockerfile.fly | H5, N3, M3 |
| `scripts/entrypoint.sh` | Added gosu privilege drop (preserved _FILE logic) | H5, N3, M3 |

---

## Implementation Details

### Phase 0 — Emergency

#### H7: Credential Rotation

**What changed:** Replaced live credentials in `.env` files with placeholder values and added rotation instructions.

**Before:**
```
OPENROUTER_API_KEY=
ENCRYPTION_MASTER_KEY=
```

**After:**
```
OPENROUTER_API_KEY=sk-or-v1-REPLACE_ME_AFTER_ROTATION
ENCRYPTION_MASTER_KEY=REPLACE_ME_WITH_UUIDGEN_OUTPUT
```

**Action required:** Operator must rotate keys at OpenRouter dashboard and generate new encryption master key.

---

#### C2: Gate pprof Endpoints

**What changed:** Profiler registration is now conditional on dev mode or explicit env flag.

**Before (`server/server.go:56-59`):**
```go
s.profiler = profiler.NewProfiler()
s.profiler.RegisterRoutes(echoServer)
s.profiler.StartMemoryMonitor(ctx)
```

**After:**
```go
s.profiler = profiler.NewProfiler()
s.profiler.StartMemoryMonitor(ctx)
if profile.IsDev() || os.Getenv("MEMOS_ENABLE_PPROF") == "true" {
    s.profiler.RegisterRoutes(echoServer)
}
```

**N6 applied:** Loopback binding dropped — pprof is a route group on the shared Echo listener; the env flag (default off) is the correct and sufficient control.

---

#### C3: Fix Echo Debug Mode

**What changed:** Debug mode is now conditional; production gets generic error messages.

**Before (`server/server.go:50`):**
```go
echoServer.Debug = true
```

**After:**
```go
echoServer.Debug = profile.IsDev()

if !profile.IsDev() {
    echoServer.HTTPErrorHandler = func(err error, c echo.Context) {
        slog.Error("request error", "error", err, "path", c.Request().URL.Path, ...)
        _ = c.JSON(http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
    }
}
```

---

### Phase 1 — Tenant Isolation

#### H2 Part A: Fix isSuperUser (B3 applied)

**What changed:** `isSuperUser` now correctly identifies only true super users.

**Before (`common.go:66-68`):**
```go
func isSuperUser(user *store.User) bool {
    return user.Role == store.RoleAdmin || user.Role == store.RoleHost
}
```

**After:**
```go
func isSuperUser(user *store.User) bool {
    return user.Role == store.RoleHost || (user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0)
}
```

**B3 applied:** `RoleHost` (instance owner/super-admin) is always super. Scoped admins (RoleAdmin with non-empty AllowedTenantIDs) are NOT super.

**Impact:** 29 call sites across `resource_service.go`, `memo_service.go`, `memo_relation_service.go`, `ticket_service.go` — all now correctly deny scoped admins cross-tenant access.

---

#### H2 Part B: Fix Tenant Filter Derivation (B2 + M2 applied)

**What changed:** For scoped admins, tenant filters are now derived from `AllowedTenantIDs` instead of being nil no-ops.

**New helper (`tenant_context.go`):**
```go
func deriveTenantIDsForScopedAdmin(ctx context.Context, s *store.Store, user *store.User) []int32 {
    if user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) > 0 {
        var tenantIDs []int32
        for _, guid := range user.AllowedTenantIDs {
            tenant, err := s.GetAgentTenant(ctx, &store.FindAgentTenant{GUID: &guid})
            if err != nil || tenant == nil { continue }
            tenantIDs = append(tenantIDs, tenant.ID)
        }
        return tenantIDs
    }
    return nil // super users see all
}
```

**Updated filter functions:**
```go
func ApplyTenantFilter(c echo.Context, s *store.Store, find *store.FindMemo) {
    tenantID := getTenantFromContext(c)
    if tenantID != nil {
        find.TenantID = tenantID
        return
    }
    user := getUserFromContext(c)
    if user != nil {
        tenantIDs := deriveTenantIDsForScopedAdmin(c.Request().Context(), s, user)
        if tenantIDs != nil {
            find.TenantIDs = tenantIDs
        }
    }
}
```

**M2 applied:** Added `TenantIDs []int32` field to `FindTicket` and `FindMemo`. Updated SQLite and Postgres drivers to generate `tenant_id IN (...)` clauses. MySQL skipped (doesn't have tenant_id columns — upstream issue).

---

#### H1: Fix TenantBindingMiddleware (N2 applied)

**What changed:** Middleware now enforces tenant binding for ALL authenticated users, not just scoped admins.

**Before:**
```go
if user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0 {
    return next(c) // superuser
}
if user.Role != store.RoleAdmin {
    return next(c) // ANY RoleUser passes!
}
```

**After:**
```go
if user.Role == store.RoleHost || (user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0) {
    return next(c) // super users
}
// ... slug lookup ...
if user.Role == store.RoleAdmin {
    if !contains(user.AllowedTenantIDs, tenant.GUID) {
        return 403
    }
} else {
    // N2: Use existing ListUserTenantPermissions
    perms, err := s.ListUserTenantPermissions(...)
    if err != nil || len(perms) == 0 {
        return 403
    }
}
```

**N2 applied:** Reused existing `ListUserTenantPermissions` store method instead of adding a new `HasTenantAccess` method.

---

#### C1: Add Permission Guards to 9 Handlers (B1 applied)

**What changed:** Added `isSuperAdmin()` helper and permission checks to 9 previously unprotected handlers.

**New helper (`agent/handlers.go`):**
```go
func (h *Handler) isSuperAdmin(c echo.Context) bool {
    user, _ := h.store.GetUser(...)
    if user.Role == store.RoleHost { return true }
    if user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0 { return true }
    return false
}
```

**B1 applied:** `isSuperAdmin()` excludes scoped admins, so the per-handler guard is effective. `isAdmin()` remains unchanged for the 67+ existing call sites.

**Handlers updated:**

| Handler | Permission Required |
|---------|---------------------|
| `HandleListTranscripts` | `chat:logs` |
| `HandleGetTranscript` | `chat:logs` |
| `HandleDeleteTranscript` | `chat:logs` |
| `HandleListLeads` | `tenant:read` |
| `HandleGetLead` | `tenant:read` |
| `HandleUpdateLeadStatus` | `tenant:write` |
| `HandleExportLeads` | `tenant:read` |
| `HandleGetTenantSettings` | `tenant:read` |
| `HandleUpdateTenantSettings` | `tenant:write` |

---

### Phase 2 — Hardening

#### H3: Path Traversal Fix

**What changed:** Filenames are sanitized and resolved paths are checked against the data directory.

**New helper (`resource_service.go`):**
```go
func sanitizeFilename(filename string) string {
    filename = filepath.Base(filename)
    filename = strings.ReplaceAll(filename, "\x00", "")
    if filename == "." || filename == ".." || filename == "" {
        return "unnamed"
    }
    return filename
}
```

**Containment assertion (applied to save + read paths):**
```go
cleanDataDir := filepath.Clean(profile.Data) + string(os.PathSeparator)
if !strings.HasPrefix(filepath.Clean(osPath), cleanDataDir) {
    return errors.Errorf("path traversal detected: %s", osPath)
}
```

---

#### H4: CSRF Fix (N5 + M4 applied)

**What changed:** Cookie changed to `SameSite=Lax`; CSRF middleware added with `Sec-Fetch-Site` header check.

**Cookie (`auth_service.go`):**
```go
// Before: SameSite=None (prod HTTPS) or SameSite=Strict (dev)
// After: Always SameSite=Lax
attrs = append(attrs, "SameSite=Lax")
```

**New CSRF middleware (`csrf.go`):**
- Skips safe methods (GET, HEAD, OPTIONS)
- M4: Skips Bearer/PAT requests (no cookie = no CSRF risk)
- N5: Uses `Sec-Fetch-Site` header when available (more reliable than Origin)
- Falls back to Origin header validation against allowlist

**Wired to:** `gwGroup` (gRPC-web) and `adminGroup` (agent admin routes).

**Config:** `CSRF_ALLOWED_ORIGINS` env var (comma-separated hostnames).

---

#### H5: Non-Root Container (N3 + M3 applied)

**What changed:** Container now drops privileges to non-root user via `gosu`.

**Dockerfile:**
```dockerfile
RUN apt-get install -y ... gosu ...
RUN groupadd -r memos && useradd -r -g memos -d /usr/local/memos -s /sbin/nologin memos
RUN mkdir -p /var/opt/memos && chown memos:memos /var/opt/memos
# No USER directive — entrypoint handles privilege drop
```

**Entrypoint (M3 — preserved _FILE logic):**
```sh
if [ "$(id -u)" = '0' ]; then
    chown -R memos:memos /var/opt/memos 2>/dev/null || true
    exec gosu memos "$@"
fi
exec "$@"
```

---

#### H6: Webhook SSRF Fix (N4 + M1 applied)

**What changed:** URL validation at save time (UX); IP-pinned dialer at dispatch time (security).

**Save-time validation (`webhook_service.go`):**
```go
func validateWebhookURL(rawURL string) error {
    parsed, err := url.Parse(rawURL)
    if parsed.Scheme != "http" && parsed.Scheme != "https" { return error }
    if parsed.Hostname() == "" { return error }
    return nil
}
```

**Dispatch-time enforcement (`plugin/webhook/webhook.go`):**
```go
func validateAndResolveWebhookURL(rawURL string) (string, error) {
    // Resolve DNS once
    addrs, _ := net.LookupHost(hostname)
    for _, addr := range addrs {
        ip := net.ParseIP(addr)
        if isInternalIP(ip) { return error } // blocks 169.254.169.254, 10.x, 192.168.x, etc.
    }
    return dialIP, nil
}

// IP-pinned transport
transport := &http.Transport{
    DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
        targetAddr := net.JoinHostPort(dialIP, port)
        return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, targetAddr)
    },
}
```

**N4 applied:** DNS is resolved once at dispatch; the connection is pinned to the validated IP. No TOCTOU re-resolution.

**M1 applied:** M1 (duplicate `DialContext`) fixed — only one `DialContext` function in the transport.

---

## Verification Checklist

- [x] Build passes (`go build ./...`)
- [x] Tests pass (`go test ./server/router/api/v1/...`)
- [x] H7: Credentials replaced with placeholders
- [x] C2: `/debug/pprof` requires `MEMOS_ENABLE_PPROF=true` in prod
- [x] C3: Error responses in prod show generic messages only
- [x] H2 Part A: `isSuperUser` returns false for scoped admins
- [x] H2 Part B: Tenant filter derived from AllowedTenantIDs for scoped admins
- [x] H1: Middleware denies RoleUser access to unauthorized tenants
- [x] C1: All 9 handlers return 403 for unauthorized access
- [x] H3: Filename `../../../etc/passwd` sanitized to `passwd`
- [x] H4: Cookie uses SameSite=Lax; CSRF middleware wired
- [x] H5: Container uses gosu for privilege drop
- [x] H6: Webhook dispatch uses IP-pinned dialer

---

## Remaining Work (Out of Scope)

- **H7 migration:** Operator must rotate actual credentials and run re-encryption migration
- **MySQL driver:** Doesn't have tenant_id columns (upstream issue) — tenant filtering silently open
- **Backfill:** Pre-existing tainted resource `Reference` rows need migration to sanitize
- **Optional hardening:** Scope RoleAdmin's implicit grants in `ResolveEffectivePermissions` so C1 is self-sufficient
