# Code Review Fix Plan v3 — Native Webhook Integrations v1

> Source: `code_review.md` + `code2_plan_review.md`
> Verdict: APPROVED (after P1 fix)

---

## Critical Fixes

### C1: Idempotency key argument swap + missing integrationID

**Problem:**
1. Arguments swapped: `eventType` passed as `leadID`, `ig.ID` passed as `eventType`
2. `integrationID` not included in hash — two integrations on same tenant collide

**Fix:**
1. Refactor `computeIdempotencyKey` to accept variadic components:
```go
func computeIdempotencyKey(components ...string) string {
    h := sha256.New()
    for _, c := range components {
        h.Write([]byte(c))
        h.Write([]byte(":"))
    }
    return hex.EncodeToString(h.Sum(nil))
}
```

2. Add `leadID string` to `dispatchEvent` signature:
```go
func (s *Service) dispatchEvent(ctx context.Context, tenantID int32, leadID string, eventType string, data string) error
```

3. Fix call site in `dispatchEvent`:
```go
idempotencyKey := computeIdempotencyKey(
    fmt.Sprintf("%d", tenantID),
    leadID,
    eventType,
    fmt.Sprintf("%d", ig.ID),
)
```

4. Update caller in `captureLeadFromSession`:
```go
s.dispatchEvent(ctx, config.TenantID, created.ID, "lead.captured", string(payload))
```

**Files:** `server/router/api/v1/agent/service.go`, `server/router/api/v1/agent/integrations.go`

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

Apply same pattern as `HandleListIntegrations`.

**File:** `server/router/api/v1/agent/integrations.go`

### M2: Crontab syntax — FALSE POSITIVE

`${VAR:-default}` is POSIX.1-2008 and dash supports it. Dockerfile sets `MEMOS_PORT=5230` which is inherited by supercronic. **No fix needed.**

---

## Moderate Fixes

### M3: Dead error return from dispatchEvent

**Problem:** `dispatchEvent` returns error, but callers only log it and never act on it.

**Fix:** Remove error return from `dispatchEvent` — change signature to:
```go
func (s *Service) dispatchEvent(ctx context.Context, tenantID int32, leadID string, eventType string, data string)
```

Update caller in `captureLeadFromSession` to remove error check.

**File:** `server/router/api/v1/agent/service.go`

### M4: Token length check leaks timing

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

## Nits

### N1: SQLite lock behavior — DOCUMENT ONLY

`UPDATE ... WHERE id IN (SELECT ... LIMIT ?)` acquires write lock on entire `agent_events` table. Acceptable for v1 at expected scale. Add code comment documenting limitation.

---

## Implementation Order

| Fix | Files | Effort |
|-----|-------|--------|
| C1 | service.go, integrations.go | 15 min |
| M1 | integrations.go | 15 min |
| M3 | service.go | 5 min |
| M4 | integrations.go | 2 min |
| N1 | service.go (comment) | 1 min |
| **Total** | | **38 min** |
