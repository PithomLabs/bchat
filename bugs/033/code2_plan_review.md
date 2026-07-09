# Adversarial Review: code2_plan.md

**Review of:** Code Review Fix Plan — Native Webhook Integrations v1  
**Source review:** `code_review.md`  
**Verdict: REWORK**

---

## Issues Found

### P1. C1 fix misses `integrationID` in idempotency key (critical)

The plan says to add `leadID string` to `dispatchEvent` and fix the `computeIdempotencyKey` call. But the current code uses `ig.ID` as a key component (in the wrong position). If the fix just swaps arguments to `computeIdempotencyKey(tenantID, leadID, eventType)`, the `integrationID` falls out of the hash entirely.

**Result:** Two integrations on the same tenant firing for the same `lead.captured` event produce the same idempotency key. The second integration silently skips the event via `continue` on unique violation — same deduplication bug, just scoped differently.

**Required fix:** The idempotency key must include `integrationID`. Either:
- Add `integrationID int32` to `computeIdempotencyKey` signature and fold it into the hash
- Or refactor to a variadic approach:

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

The call site becomes: `computeIdempotencyKey(fmt.Sprintf("%d", tenantID), leadID, eventType, fmt.Sprintf("%d", ig.ID))`

### P2. M2 crontab fix doesn't actually fix anything

Proposed crontab fix:
```
PORT=${PORT:-5230} curl ...
```

This still uses `${PORT:-5230}` — the same POSIX parameter expansion syntax. `${var:-word}` is POSIX.1-2008 and dash supports it. The original review's M2 was a false positive: the real issue would only arise if `PORT` were unset in the cron environment, but `Dockerfile.pg.fly` already sets `ENV MEMOS_PORT=5230`, which is inherited by all child processes including supercronic's shell.

**Fix:** Neither the original crontab line nor the proposed fix has a real bug. The crontab should just use `${PORT}` directly since it's always set. The plan's change is a no-op. Remove from plan or document as informational.

### P3. N1 (dead error return) omitted without explanation

The original review flagged `dispatchEvent`'s error return as dead code (moderate). The plan neither fixes it nor documents it as a wontfix. Should be addressed or explicitly deferred.

### P4. Labeling mismatch between review and plan

| Review | Plan |
|--------|------|
| Major — M1 (PII leak) | Major — M1 |
| Major — M2 (crontab) | Major — M2 |
| Moderate — N1 (dead return) | **Omitted** |
| Moderate — M2 (SQLite lock) | **Omitted** |
| Moderate — M3 (token length) | Moderate — M3 (renumbered) |

Plan reuses `M3` for a different item than the review's ModM3. Cross-referencing between documents is error-prone.

---

## Summary

| Severity | Issue | Action |
|----------|-------|--------|
| **Critical** | P1: C1 fix drops `integrationID` from key | Add `integrationID` to hash |
| Moderate | P2: M2 crontab fix is no-op | Remove or document as optional |
| Moderate | P3: N1 dead error return omitted | Add to plan or mark wontfix |
| Nit | P4: Label mismatch | Align numbering |

P1 is a blocker — the C1 fix as written creates a different but equally broken deduplication behavior where parallel integrations for the same lead collide. Fix P1 and the plan is approved.
