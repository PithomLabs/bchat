# Security Hardening Plan v2 Review

## Overview

Plan v2 addresses all 5 issues from v1. Significant improvement most review feedback from `plan_review.md` was incorporated. Two critical bugs remain in the implementation details.

---

## Issue 1: Transcript Token Forgery MISSING DECRYPTION STEP (REWORK)

### Verified claims

| Claim | Status | Evidence |
|-------|--------|----------|
| WidgetKey exposed in widget.js | Confirmed | `handlers.go:1760` `generateWidgetLoaderScript(..., tenant.WidgetKey)` |
| WidgetKey exposed in embed.js | Confirmed | `handlers.go:2104` `window.AgentChatConfig.widgetKey=...` |
| WidgetKey used for HMAC token generation | Confirmed | `service.go:1805` `generateSessionToken(..., tenant.WidgetKey)` |
| WidgetKey used for HMAC token verification | Confirmed | `handlers.go:534` `verifySessionToken(..., tenant.WidgetKey)` |

### Critical Bug: `tenant.TranscriptSigningKey` is encrypted `[]byte`, not `string`

At **plan line 114** (`service.go:1805` area):
```go
sessionToken = generateSessionToken(session.ID, tokenExpiry, tenant.TranscriptSigningKey)
```

`tenant.TranscriptSigningKey` is `[]byte` the **encrypted ciphertext** stored in the DB. `generateSessionToken(..., seed string)` expects a `string` the raw signing key. This causes **two problems**:

1. **Type mismatch:** `[]byte` not equal to `string`. Will not compile.
2. **Wrong data:** Even if cast, it passes the ciphertext, not the actual key. Token verification would use the encrypted blob as the HMAC seed, making all tokens invalid.

The same issue applies at **plan line 182** (`handlers.go:534` area):
```go
expiry, err := verifySessionToken(token, sessionID, expiryStr, tenant.TranscriptSigningKey)
```

Both the token **generation** and **verification** paths need to decrypt the signing key before use.

### Fix

Add a helper method on `Service`:

```go
func (s *Service) getTranscriptSigningSeed(ctx context.Context, tenantID int32) (string, error) {
    tenant, err := s.store.GetAgentTenant(ctx, &store.FindAgentTenant{ID: &tenantID})
    if err != nil || tenant == nil {
        return "", fmt.Errorf("tenant not found: %w", err)
    }
    if len(tenant.TranscriptSigningKey) == 0 || len(tenant.TranscriptSigningKeyNonce) == 0 {
        return "", fmt.Errorf("no transcript signing key for tenant %d", tenantID)
    }
    if s.encryptionService == nil {
        return "", fmt.Errorf("encryption service not initialized")
    }
    return s.encryptionService.Decrypt(tenant.TranscriptSigningKey, tenant.TranscriptSigningKeyNonce)
}
```

Then use it at:
- `service.go:1805`: `seed, _ := s.getTranscriptSigningSeed(ctx, config.TenantID)` then `generateSessionToken(session.ID, tokenExpiry, seed)`
- `handlers.go:534` (after GUID removal): `seed, err := h.service.getTranscriptSigningSeed(ctx, tenant.ID)` then `verifySessionToken(token, sessionID, expiryStr, seed)`

### Other nits

- **Parameter rename is good** (`tenantGUID` to `seed` at lines 104-106).
- **`ensureTranscriptSigningKeys` function** (plan lines 118-159) correctly generates 32-byte keys, hex-encodes, and encrypts.
- **GUID fallback removal** (plan line 169-183) is safe 30-min TTL vs 1-hour grace.
- **Migration column types:** SQLite `BLOB`, Postgres `BYTEA` are correct.

---

## Issue 2: Prompt Injection APPROVED WITH MINOR NIT

### Changes analysis

| Change | Status | Notes |
|--------|--------|-------|
| 2a. Security instruction in system prompt | Correct | Instruction-based, not template insertion |
| 2b. Expanded detection from 8 to ~25 patterns | Correct | Still log-only, no false-positive risk |
| 2c. Improved sanitizer (no XML stripping) | Correct | Removed per v1 review |
| 2d. session_id validation | Redundant | See below |

### Redundant: `session_id` validation

Plan lines 348-356 add a regex-based validation in `HandleChatExternal`. But `store.ValidateExternalSessionID()` already exists at `store/bridge.go:159-164` with the **exact same** pattern `^[A-Za-z0-9_-]{1,128}$`. Reuse it instead of creating a duplicate:

```go
if err := store.ValidateExternalSessionID(req.SessionID); err != nil {
    return echo.NewHTTPError(http.StatusBadRequest, "Invalid session_id format")
}
```

The existing function is also used by the bridge middleware (`handlers.go:236`), so reusing it is consistent.

---

## Issue 3: Open CORS APPROVED

### Changes analysis

| Change | Status | Notes |
|--------|--------|-------|
| 3a. `PUBLIC_CORS_ORIGINS` env var with `*` default | Correct | Deprecation warning when default used |
| 3b. Separate permissive CORS for widget group | Correct | Widget GET routes use `AllowOrigins: ["*"]` |
| 3c. `filepath.Match` for wildcard support | Correct | Handles `http://localhost:*`, `*.example.com`, etc. |

The default behaviour (`*`) is unchanged not a breaking change.

---

## Issue 4: No-Expiry Tokens APPROVED

### Changes analysis

| Change | Status | Notes |
|--------|--------|-------|
| Default to `AccessTokenDuration` (7 days) | Correct | `user_service.go:456-461` |
| Cap at `MaxNeverExpireDuration` (30 days) | Correct | `auth.go:22` constant activated |

Clean, well-scoped fix. No issues.

---

## Issue 5: Key Rotation NEEDS REFINEMENT (two blockers)

### Verified claims

| Item | Status | Evidence |
|------|--------|----------|
| Encryption init at service.go:85-98 | Only fires when `p.EncryptionMasterKey != ""` | |
| `ListAgentTenants` exists | `store/agent.go:838` | |
| `ListBridgeAuthKeys` exists | `store/bridge.go:298`, signature `(ctx, tenantID int32)` | |
| `UpdateBridgeAuthKey` exists | **DOES NOT EXIST** | |

### Critical Bug 1: `UpdateBridgeAuthKey` does not exist

Plan line 460:
```go
s.store.UpdateBridgeAuthKey(context.Background(), key)
```

This function is not defined anywhere. The only bridge auth update function is `UpdateBridgeAuthKeyLastUsed` which only updates `last_used_at`.

**Fix options:**
1. **Add `UpdateBridgeAuthKey`** to the store interface (`store/driver.go`) and implement in SQLite, Postgres, and MySQL drivers new code, three drivers.
2. **Delete + re-insert** instead of update simpler, no new interface methods.
3. **Skip bridge auth re-encryption** in the initial implementation document as a follow-up.

Option 2 is simplest: delete the old key entry and insert with re-encrypted ciphertext.

### Critical Bug 2: `s.systemSecret.EncryptionSalt` does not exist

Plan line 415:
```go
backupSvc := crypto.NewEncryptionService(backupKey, s.systemSecret.EncryptionSalt)
```

The `Service` struct (`service.go:57-72`) has no `systemSecret` field. The encryption salt is fetched locally inside `NewService` (line 87-96) and used only to create `encryptionService`, then discarded.

**Fix options:**
1. **Add `systemSecret` field to Service struct** and populate it in `NewService`.
2. **Fetch salt from DB** at the start of `ReEncryptOnStartup`.

Option 2 is safer avoids storing unnecessary state:

```go
func (s *Service) ReEncryptOnStartup() {
    if s.encryptionService == nil {
        return
    }
    backupKey := os.Getenv("ENCRYPTION_MASTER_KEY_BACKUP")
    if backupKey == "" {
        return
    }
    secret, _ := s.store.GetSystemSecret(context.Background())
    if secret == nil || len(secret.EncryptionSalt) == 0 {
        slog.Warn("no system secret found, skipping re-encryption")
        return
    }
    backupSvc := crypto.NewEncryptionService(backupKey, secret.EncryptionSalt)
    // ...
}
```

### Other nits

- **Auto-generated key file** (plan 5a) correctly uses UUID + Argon2id derivation with DB-stored salt.
- **CLI `rotate-keys` command** (plan 5c) correctly checks `primaryKey == backupKey`.
- **Re-encryption loops are otherwise correct** tenant config keys and bridge auth keys are properly iterated.

---

## Summary of Required Changes Before Implementation

| # | Issue | Severity | Fix |
|---|-------|----------|-----|
| 1 | `tenant.TranscriptSigningKey` passed as seed without decryption | **BLOCKER** | Add `getTranscriptSigningSeed()` helper; decrypt before use at `service.go:1805` and `handlers.go:534` |
| 2 | `UpdateBridgeAuthKey` does not exist | **BLOCKER** | Use delete+reinsert or add the store method |
| 3 | `s.systemSecret.EncryptionSalt` does not exist | **BLOCKER** | Fetch from DB at start of `ReEncryptOnStartup` |
| 4 | `session_id` validation is redundant | **Minor** | Reuse `store.ValidateExternalSessionID()` instead of new regex |

---

## Decision

| Issue | Verdict |
|-------|---------|
| 1. Transcript token forgery | **REWORK** missing decryption step makes the fix uncompilable |
| 2. Prompt injection | **APPROVED WITH NIT** deduplicate session_id validation |
| 3. Open CORS | **APPROVED** clean fix |
| 4. No-expiry tokens | **APPROVED** clean fix |
| 5. Key rotation | **REWORK** two blockers (`UpdateBridgeAuthKey`, `systemSecret`) |

**Overall: APPROVED WITH REWORK** Issues 3 and 4 are ready. Issues 1, 2 (minor), and 5 need the fixes above before implementation.
