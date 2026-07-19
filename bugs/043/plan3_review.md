# Security Hardening Plan v3 Final Review

## Overview

v3 addresses all feedback from `plan2_review.md`. Three critical blockers are resolved. Two minor nits remain before code.

---

## Bug Fix Verification

| Bug from v2 review | Status | Evidence in v3 |
|---|---|---|
| Missing decryption step for `TranscriptSigningKey` | **FIXED** | Added `getTranscriptSigningSeed()` helper (1c, lines 110-124) |
| `UpdateBridgeAuthKey` does not exist | **FIXED** | Now uses `RevokeBridgeAuthKey` + `CreateBridgeAuthKey` (5b, lines 483-496) |
| `s.systemSecret.EncryptionSalt` does not exist | **FIXED** | Now fetches from DB via `GetSystemSecret()` (5b, line 435) |
| session_id validation redundant | **FIXED** | Now reuses `store.ValidateExternalSessionID()` (2d, line 297) |
| XML tag stripping over-broad | **FIXED** | Removed (2c, line 292) |

---

## Issue-by-Issue Assessment

### Issue 4: No-Expiry Tokens APPROVED

No changes from v2. Clean, scoped fix. `MaxNeverExpireDuration` activated from dead code.

### Issue 1: Transcript Token Forgery APPROVED WITH NITS

**Nit 1: `UpsertAgentTenant` does not exist (line 171)**

Plan calls `s.store.UpsertAgentTenant(ctx, tenant)` but the function is named `UpdateAgentTenant`:

- `store/agent.go:842`: `func (s *Store) UpdateAgentTenant(ctx context.Context, tenant *AgentTenant) (*AgentTenant, error)`

Change line 171 from `s.store.UpsertAgentTenant(ctx, tenant)` to `s.store.UpdateAgentTenant(ctx, tenant)`.

**Nit 2: Method name case mismatch (line 144 vs line 187)**

Function defined at line 144 as lowercase `ensureTranscriptSigningKeys` (unexported), called at line 187 as `EnsureTranscriptSigningKeys` (capital E, exported). Will not compile.

Fix: export the function rename to `EnsureTranscriptSigningKeys` at the definition (line 144). Or lowercase the call site. Exporting is recommended since main.go needs to call it.

### Issue 2: Prompt Injection APPROVED

All changes correct:
- 2a: Instruction-based security boundary (not template insertion)
- 2b: 25-pattern detection, log-only
- 2c: Strips control chars + null bytes only (no XML stripping)
- 2d: Reuses existing `ValidateExternalSessionID()`

### Issue 3: Open CORS APPROVED

All changes correct:
- 3a: `PUBLIC_CORS_ORIGINS` env var with `*` default + deprecation warning
- 3b: Widget group gets permissive `*` CORS separately
- 3c: `filepath.Match` for glob wildcards

No breaking change default remains `*`.

### Issue 5: Key Rotation APPROVED WITH MINOR GAP

Both critical bugs from v2 are fixed:
- `systemSecret` fetched from DB
- Bridge auth key re-encryption uses `RevokeBridgeAuthKey` + `CreateBridgeAuthKey`

**Gap: `ReEncryptOnStartup` has no call site shown.**

The function is defined (5b, lines 422-502) but the plan never shows where it is called from main.go. The startup flow in main.go is:

```go
instanceProfile.EncryptionMasterKey = ...  // line 50
storeInstance := store.New(...)            // line 82
storeInstance.Migrate(ctx)                 // line 83
s, _ := server.NewServer(ctx, ...)         // line 89
```

After `server.NewServer` returns, `agentService` is created inside the server and `encryptionService` is initialized. `ReEncryptOnStartup` should be called after both exist, alongside `EnsureTranscriptSigningKeys` (Issue 1e, line 187):

```go
agentService.EnsureTranscriptSigningKeys(context.Background())
agentService.ReEncryptOnStartup()
```

The plan also needs to ensure `getOrCreateEncryptionKey` (5a, line 397) is called BEFORE `instanceProfile.EncryptionMasterKey` is checked at line 54. The data dir must be resolved first (from `viper.GetString("data")` with fallback to `"./build/data"` in dev mode).

---

## Summary

| Issue | Verdict |
|-------|---------|
| 1. Transcript token forgery | **APPROVED WITH NITS** fix `UpsertAgentTenant` to `UpdateAgentTenant`, fix method case mismatch |
| 2. Prompt injection | **APPROVED** all clean |
| 3. Open CORS | **APPROVED** all clean |
| 4. No-expiry tokens | **APPROVED** all clean |
| 5. Key rotation | **APPROVED WITH MINOR GAP** add `ReEncryptOnStartup` call site in main.go, ensure key file is generated before profile init |

**Overall: APPROVED WITH NITS** all five issues are correctly diagnosed and well-structured. The remaining issues are implementation-level details, not architectural problems.
