# Code Review Fix Plan — Native Webhook Integrations v1

> Source: `code_review.md` (adversarial review)
> Verdict: REWORK — 2 critical, 2 major, 3 moderate, 4 nits

---

## Critical Fixes (must deploy)

### C1: Idempotency key argument swap — all events deduplicated

**Problem:** `computeIdempotencyKey(tenantID, eventType, fmt.Sprintf("%d", ig.ID))` passes `eventType` as `leadID` and `ig.ID` as `eventType`. Every lead produces the same key, causing deduplication.

**Fix:**
1. Add `leadID string` parameter to `dispatchEvent` signature
2. Fix call: `computeIdempotencyKey(tenantID, leadID, eventType)`
3. Update caller in `captureLeadFromSession` to pass `created.ID`

**Files:** `server/router/api/v1/agent/service.go`

### C2: Poller context cancelled before webhook deliveries complete

**Problem:** `c.Request().Context()` is cancelled when HTTP handler returns. Poller deliveries abort.

**Fix:** Use `context.Background()` with 30s timeout:
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
h.service.processEventPoller(ctx)
```

**File:** `server/router/api/v1/agent/integrations.go`

---

## Major Fixes

### M1: Event API leaks PII in payload field

**Problem:** `HandleListEvents` returns raw `AgentEvent` including `Payload` (name, email, phone). `tenant:read` permission allows non-admin access.

**Fix:** Create `safeEvent` struct that omits `Payload`:
```go
type safeEvent struct {
    ID            int32  `json:"id"`
    TenantID      int32  `json:"tenant_id"`
    IntegrationID int32  `json:"integration_id"`
    EventType     string `json:"event_type"`
    Status        string `json:"status"`
    Attempts      int32  `json:"attempts"`
    CreatedAt     int64  `json:"created_at"`
}
```

**File:** `server/router/api/v1/agent/integrations.go`

### M2: Crontab syntax incompatible with dash

**Problem:** `${PORT:-5230}` doesn't work in `/bin/sh` (dash on Ubuntu 24.04). Port expands to empty.

**Fix:** Use explicit variable assignment:
```
*/5 * * * * PORT=${PORT:-5230} curl -sf -X POST "http://localhost:${PORT}/api/v1/system/trigger-cron" -H "X-Cron-Token: ${CRON_TOKEN}"
```

**File:** `build/crontab`

---

## Moderate Fixes

### M3: Token length check leaks timing

**Problem:** `len(token) != len(expectedToken)` reveals expected length via timing.

**Fix:** Remove length check, let `hmac.Equal` handle everything:
```go
if token == "" || expectedToken == "" {
    return c.JSON(http.StatusUnauthorized, ...)
}
if !hmac.Equal([]byte(token), []byte(expectedToken)) {
    return c.JSON(http.StatusUnauthorized, ...)
}
```

**File:** `server/router/api/v1/agent/integrations.go`

---

## Nits (optional)

### N4: processEventPoller returns void
- Add error return, propagate in handler
- Low priority — logging is sufficient for v1

### N6: confirm() usage
- Replace browser-native `confirm()` with Joy UI AlertDialog
- Low priority — functional but not styled

---

## False Positive

### N7: Icon import
- `RefreshCwIcon` is valid (used in `web/src/pages/RagStats.tsx:2`)
- No action needed

---

## Implementation Order

| Fix | Files | Effort |
|-----|-------|--------|
| C1 | service.go | 10 min |
| C2 | integrations.go | 5 min |
| M1 | integrations.go | 15 min |
| M2 | build/crontab | 2 min |
| M3 | integrations.go | 2 min |
| **Total** | | **34 min** |
