## Completed (19 issues)

### Phase 1 — Critical Runtime Fixes
- **CRIT-1:** Fixed migration table name `agent_audience` → `agent_audiences`
- **CRIT-2:** Wired `allowed_tenant_ids` through CreateUser/UpdateUser/ListUsers + added to UpdateUser struct
- **CRIT-3:** Added `TenantID` to `DeleteMemoRelation` + updated all 3 callsites
- **CRIT-4:** Added `TenantID: memo.TenantID` to both `ListMemoRelations` calls

### Phase 2 — Security & Memory Fixes
- **CRIT-5:** Fixed session lock leak + orphaned lock cleanup in `cleanup()`
- **CRIT-6:** Added rate limiting to `HandleSelectTenant`
- **HIGH-3:** TenantBindingMiddleware now fails closed (500 on DB error, 403 on nil user)
- **HIGH-4:** Removed `access_token`/`cookie` from select-tenant JSON response
- **HIGH-5:** Added `MaxMessageLength` validation to `ChatInternal`
- **HIGH-6:** Derived HMAC session key via `HMAC(GUID, "session-token-key")`
- **HIGH-8:** Added `</script>` and `</` escaping to `escapeJS`
- **HIGH-9:** Fixed nil TenantID bypass at 3 locations (documented legacy memo behavior)
- **HIGH-10:** Added `TenantID: memo.TenantID` to REFERENCE relation upsert

### Phase 3 — Hardening
- **HIGH-1:** Derived backup salt via `HMAC(primarySalt, "backup-key-salt")`
- **MED-2:** Fixed `Sscanf` into `time.Time` using `int64` temp variable
- **MED-3:** Added bounded eviction (10K max entries) + cleanup goroutine to rate limiter
- **MED-4:** Added `ON CONFLICT DO UPDATE SET tenant_id = excluded.tenant_id` to UpsertMemoRelation
- **MED-5:** Added `sync.Mutex` to Handler struct guarding `ensurePlaygroundDemo`
- **MED-6:** Added `slog.Warn` when backup key fallback succeeds

