# Security Hardening — Code Review Round 2 Implementation Plan (Adversarial Review)

**Date:** 2026-07-19
**Source:** `bugs/043/code3.md`
**Methodology:** Each proposed change verified against actual codebase at exact line numbers.

---

## Triage Verdict

| ID | Planned Verdict | Adversarial Verdict | Change Required |
|----|-----------------|---------------------|-----------------|
| H2 | APPROVED | **APPROVED** | Clean |
| M4 | APPROVED | **APPROVED** | Clean |
| M6 | APPROVED | **APPROVED** | Clean |
| L11 | APPROVED | **APPROVED** | Clean |
| R1 | REWORK | **APPROVED WITH NIT** | Missing ctx source in rotateKeysCmd |
| M7 | APPROVED | **APPROVED** | Clean |
| N1 | APPROVED WITH NIT | **APPROVED** | Clean |
| N2 | APPROVED WITH NIT | **APPROVED** | Clean |
| R2 | REWORK | **REWORK CONFIRMED** | Nil encService path stores raw bytes that won't decrypt |

**7 approved, 1 nit, 1 rework confirmed.**

---

## CONFIRMED REWORK Finding

### R2: `setupTestSigningKey` nil-encryption path stores undecryptable raw bytes

**Plan claim (code3.md:415-431):**
```go
func setupTestSigningKey(t *testing.T, ctx context.Context, ts *store.Store,
    tenant *store.AgentTenant, service *Service) {
    encService := service.EncryptionService()
    if encService == nil {
        // No encryption service in test — use raw key
        tenant.TranscriptSigningKey = []byte(testSigningSeed)
        tenant.TranscriptSigningKeyNonce = make([]byte, 12)
        ts.UpdateAgentTenant(ctx, tenant)
        return
    }
    ciphertext, nonce, err := encService.Encrypt(testSigningSeed)
    ...
```

**Actual test setup (`bridge_foundation_test.go:418`):**
```go
service := NewService(ts, &profile.Profile{Driver: "sqlite", Mode: "prod"})
```

No `EncryptionMasterKey` in profile → `svc.encryptionService` is nil (`service.go:86`).

**Actual decryption path (`service.go:1827-1838`):**
```go
func (s *Service) getTranscriptSigningSeed(ctx, tenantID) (string, error) {
    tenant, err := s.store.GetAgentTenant(...)
    if len(tenant.TranscriptSigningKey) == 0 || ... { return "", error }
    if s.encryptionService == nil {
        return "", fmt.Errorf("encryption service not initialized")  // ← HIT
    }
    return s.encryptionService.Decrypt(tenant.TranscriptSigningKey, ...)
}
```

**Bug:** The nil-encryption branch stores `[]byte(testSigningSeed)` as `TranscriptSigningKey`. When the handler calls `getTranscriptSigningSeed` to verify the token, `s.encryptionService` is nil → returns error → handler returns **403 Forbidden**. Tests would fail.

**Fix:** The nil-encryption branch is unreachable in a properly-set-up test. Instead of a nil fallback, ensure encryption is always initialized:

```go
// bridge_foundation_test.go:418 — change profile to include master key
service := NewService(ts, &profile.Profile{
    Driver:              "sqlite",
    Mode:                "prod",
    EncryptionMasterKey: "test-encryption-master-key-1234567890ab",
})
```

Then the non-nil path always executes and the nil branch is either removed or replaced with `require.NotNil(t, encService)`. The `setupTestSigningKey` function works correctly when encryption is initialized.

---

## Nits

### N1: R1 — `rotateKeysCmd` already has `ctx` but it's `context.Background()`

**Plan claim (code3.md:206-208):**
```go
// rotate-keys:
svc.ReEncryptOnStartup(ctx)
```

**Actual code (`main.go:153`):**
```go
ctx := context.Background()
```

The `ctx` variable already exists at line 153. The plan should show replacing it with a timed context, not adding a new variable:

```go
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()
```

The `context` package is already imported at `main.go:4`.

---

## Approved Findings (verified correct)

### H2 — Log DB errors (`service.go:232,240`)
**Codebase evidence:** Both use `_` discard pattern. Plan replaces with proper error logging via `slog.Error`. Correct.

### M4 — Pre-compile regex (`service.go:2201,2206`)
**Codebase evidence:** Two `regexp.MustCompile` calls inside function body. Plan moves to package-level vars. Correct.

### M6 — Redact tenant ID (`service.go:1833`)
**Codebase evidence:** `"no transcript signing key for tenant %d"`. Plan removes `%d`. Correct. Tenant slug logged separately at `handlers.go:532`.

### L11 — HMAC separator (`service.go:1806,1818`)
**Codebase evidence:** `mac.Write([]byte(sessionID + expiry...))`. Plan adds `|` separator. 30-min TTL makes breakage acceptable. Correct.

### R1 — Context param (`service.go:222`, `v1.go:66-68`, `main.go:173,188-191`)
**Codebase evidence:**
- `service.go:222`: `func (s *Service) ReEncryptOnStartup()` — no ctx param
- 7 internal `context.Background()` calls (lines 232, 240, 243, 259, 265, 281, 290)
- `v1.go:66-68`: Both hooks use `context.Background()`
- `main.go:173`: `svc.ReEncryptOnStartup()` in rotateKeysCmd
- `main.go:188-191`: `agentServiceForRotation.ReEncryptOnStartup()` wraps the call

Plan correctly adds `ctx` parameter to all three sites. Correct.

### M7 — O_EXCL key file (`main.go:305-323`)
**Codebase evidence:** TOCTOU race between ReadFile and WriteFile. Plan adds `O_EXCL` with re-read fallback. Correct.

### N1 — Prefix wording (`service.go:2257-2259`)
**Codebase evidence:** Current code is log-only. Plan changes prefix from `"[FLAGGED ...]"` to `"[SUSPICIOUS INPUT — proceed with standard policy only]"` — avoids "instruction", "override", "injection". Correct.

### N2 — Context-aware retry (`service.go:1870-1873`)
**Codebase evidence:** Current code has no retry. Plan adds 3-attempt retry with `select` on `ctx.Done()`. Correct.

---

## Summary

| Change | Verdict | Key Action |
|--------|---------|------------|
| R2 (test fix) | **REWORK** | Use `EncryptionMasterKey` in test profile instead of nil-encryption fallback |
| R1 (rotateKeysCmd) | **NIT** | Use existing `ctx` var, replace `context.Background()` with `WithTimeout` |
| H2, M4, M6, L11, M7, N1, N2 | **APPROVED** | All verified correct |
