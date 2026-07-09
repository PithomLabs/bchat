# Code Review: Native Webhook Integrations v1

**Reviewer:** Adversarial  
**Plan:** plan5.md  
**Reviewed against:** prompt_code_review.md  
**Verdict: REWORK**

---

## CRITICAL

### C1. Idempotency key argument swap — all events deduplicated to one per integration

**File:** `server/router/api/v1/agent/service.go:4546`  
**Function:** `dispatchEvent`

```go
idempotencyKey := computeIdempotencyKey(tenantID, eventType, fmt.Sprintf("%d", ig.ID))
```

The function signature is `computeIdempotencyKey(tenantID int32, leadID string, eventType string)`. The call passes:
- `tenantID` → correct
- `eventType` → passed as `leadID` parameter
- `fmt.Sprintf("%d", ig.ID)` → passed as `eventType` parameter

For a `lead.captured` event, every lead produces the same idempotency key `sha256(tenantID + "lead.captured" + integrationID)`. The second lead hits the UNIQUE constraint on `agent_events.idempotency_key` and is silently deduplicated via `continue`. **Only one event per integration is ever dispatched.**

**Fix:** Call must be:
```go
computeIdempotencyKey(tenantID, leadID, eventType)
```
where `leadID` is the actual lead/session identifier (e.g., `created.ID` from `captureLeadFromSession`).

Additionally, `dispatchEvent` currently has no access to the lead ID. The signature needs `leadID string` added, and the caller at `service.go:4407` must pass it.

---

### C2. Poller context cancelled before webhook deliveries complete

**File:** `server/router/api/v1/agent/integrations.go:125`  
**Handler:** `HandleTriggerCron`

```go
h.service.processEventPoller(c.Request().Context())
return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
```

`c.Request().Context()` is cancelled when the HTTP handler returns. The `200 OK` response is sent synchronously, then the function returns, cancelling the context. If `processEventPoller` is still delivering webhooks, the delivery HTTP client receives a cancelled context and aborts.

**Fix:** Use `context.Background()` with a timeout:
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
h.service.processEventPoller(ctx)
```

---

## Major

### M1. Event API leaks PII in payload field

**File:** `server/router/api/v1/agent/integrations.go:332-348`  
**Handler:** `HandleListEvents`

`HandleListEvents` returns the raw `AgentEvent` struct including `Payload` — a JSON blob containing lead PII (name, email, phone, session_id). The route uses `tenant:read` permission, so non-admin users can see full lead data via the events API. The `HandleListIntegrations` handler correctly masks `config` from the response; `HandleListEvents` should do the same for `payload`.

**Fix:** Create a `safeEvent` struct that omits `payload` (or truncates it), and apply the same pattern used for integrations. Document that the payload is intentionally excluded from the list endpoint.

### M2. Crontab `${PORT:-5230}` default syntax incompatible with `/bin/sh` (dash)

**File:** `build/crontab:2`

```
*/5 * * * * curl -sf -X POST http://localhost:${PORT:-5230}/api/v1/system/trigger-cron -H "X-Cron-Token: ${CRON_TOKEN}"
```

Supercronic executes commands via `/bin/sh -c`. On Ubuntu 24.04 (the runtime image), `/bin/sh` is **dash**, which does **not** support `${VAR:-default}` syntax. If `PORT` is unset, this expands to `http://localhost:/api/v1/...` — the port is empty, `curl` fails silently.

**Fix:** Use a wrapper that handles the default:
```sh
*/5 * * * * PORT=${PORT:-5230} curl -sf -X POST "http://localhost:${PORT}/api/v1/system/trigger-cron" -H "X-Cron-Token: ${CRON_TOKEN}"
```
Or set `PORT` explicitly in `Dockerfile.pg.fly` and `fly.toml` so the variable is always defined (it already is in both — `MEMOS_PORT=5230`, but `crond` doesn't see env vars from Dockerfile unless exported). The safest fix: hardcode the port in crontab or use a wrapper script.

---

## Moderate

### N1. `dispatchEvent` error return is dead code

**File:** `server/router/api/v1/agent/service.go:4407`

The caller in `captureLeadFromSession` only logs the error:
```go
if err := s.dispatchEvent(ctx, config.TenantID, "lead.captured", string(payload)); err != nil {
    slog.Warn("failed to dispatch lead event", ...)
}
```

Since `dispatchEvent` logs all errors internally (failed integration list, duplicate event, etc.), the returned error is never actionable at the call site. The error return is unused by all callers.

**Fix:** Remove the error return from `dispatchEvent` or add meaningful errors that callers can act on (e.g., return a structured result for monitoring).

### M2. SQLite `ClaimPendingEvents` acquires write lock on entire table

**File:** `store/db/sqlite/agent.go:2710-2720`

The `UPDATE ... WHERE id IN (SELECT ... LIMIT ?)` statement acquires a SQLite write lock (RESERVED) on the entire `agent_events` table. With 1000+ stale rows (e.g., after crash), every 5-min poll cycle grabs 10. A single misbehaving tenant with many pending events could cause lock contention for all concurrent writes (lead capture inserts, status updates).

**Fix:** Acceptable for v1 at expected scale. Document limitation in code comment. Consider adding `tenant_id` filter to `ClaimPendingEvents` or adding per-tenant claim limit in v1.1.

### M3. Token length check leaks CRON_TOKEN length via timing

**File:** `server/router/api/v1/agent/integrations.go:110`

```go
if token == "" || expectedToken == "" || len(token) != len(expectedToken) {
```

The early return on length mismatch reveals the expected token length via timing. Since `CRON_TOKEN` is always `openssl rand -hex 32` (64 chars), the attacker already knows the length. Minimal practical risk. Still, `hmac.Equal` alone would handle mismatch without leaking length.

**Fix:** Remove the length check and let `hmac.Equal` handle everything. Low priority.

---

## Nits

### N4. `processEventPoller` returns void — no error propagation to caller

**File:** `server/router/api/v1/agent/service.go:4596`

`processEventPoller` returns nothing. All errors are logged. The trigger-cron handler always returns `200 OK`. A monitoring system cannot detect poller failure. Add error return and propagate in the handler.

### N5. MySQL stubs compile only if MySQL driver is excluded from build

**File:** `store/db/mysql/agent.go:411-449`

Stubs return `errNotImplemented`. Verify the MySQL driver file has a build exclusion tag (`//go:build mysql`) or is excluded by the build path. If the MySQL driver is always in tree and the interface requires all methods, the stubs will compile — which is correct. No action needed if build tags exist; worth verifying.

### N6. Frontend uses browser-native `confirm()` for delete dialog

**File:** `web/src/pages/AgentAdminSections/IntegrationsSection.tsx:78`

```tsx
if (!confirm(`Delete webhook "${ig.label}"?`)) return;
```

Joy UI provides `AlertDialog` and `Modal` components. The browser-native `confirm()` blocks the event loop and cannot be styled.

### N7. Icon import may fail — `RefreshCwIcon` vs `RefreshCw`

**File:** `web/src/pages/AgentAdminSections/IntegrationsSection.tsx:17`

```tsx
import { PlusIcon, TrashIcon, PlayIcon, RefreshCwIcon } from "lucide-react";
```

`lucide-react` exports `RefreshCw`, not `RefreshCwIcon`. Verify project conventions — the other imports (`PlusIcon`, `TrashIcon`, `PlayIcon`) also have the `Icon` suffix, suggesting a re-export pattern. If `RefreshCwIcon` is not in the re-exports, this will fail at build time.

---

## Summary

| Severity | Count | Key Issues |
|----------|-------|------------|
| Critical | 2 | C1: idempotency key broken (all events deduplicated), C2: poller uses cancelled context |
| Major | 2 | M1: event payload leaks PII, M2: crontab syntax incompatible with dash |
| Moderate | 3 | M3: dead error return, M4: SQLite lock behavior, M5: token length oracle |
| Nits | 4 | N4-N7: error propagation, build tags, UI polish, icon import |

**C1 must be fixed before any deployment.** The idempotency key bug makes the entire feature non-functional for more than one event. C2 causes unreliable webhook delivery under load.