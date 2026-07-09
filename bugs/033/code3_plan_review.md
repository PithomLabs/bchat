# Adversarial Review: code3_plan.md

**Review of:** Code Review Fix Plan v3 — Native Webhook Integrations v1  
**Source reviews:** `code_review.md` + `code2_plan_review.md`  
**Verdict: APPROVED with nits**

---

## What Changed from code2_plan

| Issue | code2_plan | code3_plan | Status |
|-------|------------|------------|--------|
| C1: idempotency key | `computeIdempotencyKey(tenantID, leadID, eventType)` — drops integrationID | Variadic `computeIdempotencyKey(...)` — includes all components | **Fixed** |
| C2: poller context | Listed as critical fix | Dropped (false positive — poller is synchronous, context valid during blocking call) | **Correct** |
| M2: crontab | Listed as fix | Marked FALSE POSITIVE with POSIX justification | **Correct** |
| M3: dead error return | Omitted | Added as M3 — removes error return from `dispatchEvent` | **Fixed** |
| Labeling | Mismatched with review | Clean sequential M1, M3, M4 | **Fixed** |

---

## Nit

### N1. M3 fix doesn't specify how internal error returns become logs

The plan changes `dispatchEvent` from `func ... error` to `func ... data string)`. But `dispatchEvent` currently has three `return fmt.Errorf(...)` paths:
1. `failed to list integrations` (line ~4533)
2. `no integrations configured` returns `nil` (line ~4536) — fine, no change needed
3. `CreateAgentEvent` failure: logs via `slog.Warn` then `continue` — no return, fine

Path 1 currently returns an error that bubbles up to `captureLeadFromSession`. With the void signature, this must become an `slog.Error(...)` call.

**Recommendation:** In the plan's implementation description, add a note that the `ListAgentIntegrations` error path becomes:
```go
if err != nil {
    slog.Error("failed to list integrations", "tenant_id", tenantID, "error", err)
    return
}
```

---

## Summary

| Item | Result |
|------|--------|
| All code2_plan_review issues | **Resolved** |
| C2 dropped | **Correct** (false positive) |
| Remaining gap | M3 internal error paths need slog conversion (minor) |

Plan is sound and ready for implementation.
