# Review of `plan4_deepseek.md`

**Reviewer:** mimo
**Date:** 2026-07-04
**Verdict:** Rework required. The plan has a critical timing flaw and misses several findings.

---

## Critical Flaw: P0-3 Phase 1 Is Already Deployed

The plan's Sprint 1, item 5 says:

> **P0-3 Phase 1** — Token redaction | `user_service.go` | List response has `access_token = ""`

**This is already done.** The code at `user_service.go:417` already has `AccessToken: ""`. The frontend is already broken. The plan reads as if Phase 1 is future work — it is not.

**What the plan should say:**

> **P0-3 EMERGENCY FIX** — Revert token redaction OR hotfix frontend. The current code at `user_service.go:417` returns `AccessToken: ""` which breaks `AccessTokenSection.tsx` (cannot display, copy, or delete tokens). Two options:
> - **Option A (immediate):** Revert line 417 to return the raw token. Defer redaction until Phase 2 (ID-based delete) is ready with frontend changes.
> - **Option B (ship Phase 2 now):** Add `id` field to protobuf, update `DeleteUserAccessToken` to accept `id`, update frontend, then redact. This is 2-3 hours of work, not a Sprint 1 item.

The current plan will cause the implementing agent to "implement" something that's already deployed, discover it's already there, and then move on without fixing the broken frontend.

---

## Issue Not Addressed: P0-1 Deferred Goroutine Context Leak

**Severity:** Medium
**File:** `user_service.go:244-249`

The plan's Sprint 2, item 6 describes the P0-1 fix but does not mention the goroutine leak. The current code:

```go
go func(uid int32) {
    if deferredErr := s.deleteAllUserAccessTokens(ctx, uid); deferredErr != nil {
```

The goroutine captures `ctx` from the request. When the request completes, `ctx` is cancelled. The deferred `deleteAllUserAccessTokens` call will always fail with "context cancelled" — the recovery never executes.

**Fix:** The plan should specify using `context.Background()` in the goroutine:
```go
go func(uid int32) {
    bgCtx := context.Background()
    if deferredErr := s.deleteAllUserAccessTokens(bgCtx, uid); err != nil {
```

---

## Issue Not Addressed: P0-1 Error Message Shows Wrong Error

**Severity:** Low
**File:** `user_service.go:250-252`

```go
return nil, status.Errorf(codes.Internal,
    "password changed but failed to invalidate existing sessions. "+
        "Admin must manually purge tokens via SQL. Error: %v", err)
```

The `%v` formats the original `err` (first attempt), not `retryErr` (the actual failure after retry). The plan should specify changing `err` to `retryErr`.

---

## Issue Not Addressed: P1-9 Unparseable Tokens Not Logged

**Severity:** Medium
**File:** `user_service.go:553-554`

```go
_, _, _ = jwt.NewParser().ParseUnverified(a.AccessToken, aClaims)
```

Errors are silently discarded. The plan describes the max-N dedup but does not mention adding a `slog.Warn` when token parsing fails. Corrupted tokens persist in the DB with no visibility.

---

## P1-10 Line Number Wrong

**Severity:** Low (confusing for implementing agent)

The plan says `workspace_setting_service.go:103`. The actual validation in `SetWorkspaceSetting` is at **line 73** (already fixed). The remaining issue is at **line 107** in `convertWorkspaceSettingToStore` where the error is discarded with `_, _`.

---

## What's Solid

- Sprint structure and ordering is correct.
- P0-3 Phase 2 (ID-based deletion) in Sprint 3 is the right approach.
- P1-8 `requireLLMConfig` wrapper scoped to 2 chat-critical endpoints is correct.
- P1-9 iat-sorted max-N=10 eviction is correct.
- P0-4 includes the error log that was missing.
- INFRA-13 correctly dropped.
- Sprint 4 correctly removed per plan3_review_mimo.md.
- Verification commands are correct.

---

## Summary

| Issue | Severity | Action |
|-------|----------|--------|
| P0-3 Phase 1 already deployed | **Critical** | Rewrite Sprint 1 item 5 as emergency fix (revert or hotfix) |
| P0-1 goroutine context leak | Medium | Add `context.Background()` to goroutine |
| P0-1 error message wrong error var | Low | Change `err` to `retryErr` |
| P1-9 unparseable tokens not logged | Medium | Add `slog.Warn` on parse failure |
| P1-10 wrong line number | Low | Fix to line 107 |

**Bottom line:** The plan must be rewritten to acknowledge that P0-3 Phase 1 is already deployed and the frontend is broken. The implementing agent needs explicit instructions to either revert the redaction or ship Phase 2 immediately — not "implement" something that's already in the code.
