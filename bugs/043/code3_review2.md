# Security Hardening — Adversarial Code Review Round 3

**Date:** 2026-07-19  
**Reviewer:** Senior Go Architect / Application Security Engineer  
**Scope:** Changes from `code3_summary.md` (Round 3 hardening implementation)  
**Stance:** Aggressive — assume developer mistakes. Focus on exploitable vulnerabilities.

---

## Executive Summary

The Round 3 hardening fixes several real issues (context propagation, HMAC separators, retry context, key-file races). However, the implementation is **incomplete in critical areas**:

- **2 HIGH** findings: silent partial re-encryption due to ignored DB errors, and an empty-file crash-loop in key generation that silently changes the master key on every restart.
- **2 MEDIUM** findings: context cancellation is not honored inside the `ReEncryptOnStartup` loop, and the new timeout wrapper does not cover `NewService`, which still uses `context.Background()`.
- **0 CRITICAL** findings.

All findings below are backed by exact line references.

---

## Findings

### H1 — Silent partial re-encryption: 4 DB errors still discarded in `ReEncryptOnStartup`

**Severity:** HIGH  
**Location:** `server/router/api/v1/agent/service.go:257, 273, 279, 295`

**Description:**  
Finding H2 in Round 3 added error logging for `GetSystemSecret` and `ListAgentTenants`, but four other database calls inside `ReEncryptOnStartup` still silently discard errors:

```go
// Line 257 — error ignored
config, _ := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &tenant.ID})

// Line 273 — error ignored
s.store.UpsertTenantConfig(ctx, config)

// Line 279 — error ignored
keys, _ := s.store.ListBridgeAuthKeys(ctx, tenant.ID)

// Line 295 — error ignored
s.store.RevokeBridgeAuthKey(ctx, tenant.ID, key.KeyID, time.Now())
```

The final summary log at line 312 only counts explicit `failed++` increments from decrypt/encrypt errors. DB failures do not increment `failed`, do not log the tenant, and do not stop the loop.

**Exploit scenario:**  
During key rotation, the database becomes unavailable or returns an error (e.g., SQLite lock, connection pool exhaustion, transient network error). The loop silently skips re-encryption for the remaining tenants. The operator sees:

```
Re-encryption complete  succeeded=5  failed=0
```

while 3 tenants’ API keys remain encrypted with the old backup key. If the backup key is later revoked or rotated out, those keys are unrecoverable. In a multi-tenant SaaS deployment, this means silent data loss for affected tenants.

**Fix recommendation:**  
Check and log every DB error. Increment `failed` and break/continue explicitly:

```go
config, err := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &tenant.ID})
if err != nil {
    slog.Error("failed to get tenant config for re-encryption", "tenant", tenant.Slug, "error", err)
    failed++
    continue
}
// ...
if err := s.store.UpsertTenantConfig(ctx, config); err != nil {
    slog.Error("failed to upsert tenant config for re-encryption", "tenant", tenant.Slug, "error", err)
    failed++
    continue
}
// ...
keys, err := s.store.ListBridgeAuthKeys(ctx, tenant.ID)
if err != nil {
    slog.Error("failed to list bridge auth keys for re-encryption", "tenant", tenant.Slug, "error", err)
    failed++
    continue
}
// ...
if err := s.store.RevokeBridgeAuthKey(ctx, tenant.ID, key.KeyID, time.Now()); err != nil {
    slog.Error("failed to revoke old bridge auth key", "tenant", tenant.Slug, "key_id", key.KeyID, "error", err)
    failed++
    continue
}
```

---

### H2 — Empty-file crash loop in `getOrCreateEncryptionKey` silently changes the master key

**Severity:** HIGH  
**Location:** `bin/memos/main.go:320-329`

**Description:**  
If the process crashes between `os.OpenFile(..., O_EXCL)` success and `WriteString`, the file is created but empty (0 bytes). On the next startup, the re-read fallback checks `len(k) >= 16`, fails for empty content, logs a warning, and returns a newly generated UUID. The empty file persists, so every subsequent restart generates a new key.

```go
// main.go:320-329
f, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
if err != nil {
    // Another instance won the race — re-read their key
    if data, readErr := os.ReadFile(keyFile); readErr == nil {
        if k := strings.TrimSpace(string(data)); len(k) >= 16 {
            return k
        }
    }
    slog.Warn("failed to create encryption key file", "error", err)
    return key // returns NEW generated key, ignoring the corrupt empty file
}
```

**Exploit scenario:**  
An operator deploys the application in a container. The container crashes during first start after the empty `.encryption_key` file is created. On restart, a new key is generated. All tenant API keys encrypted with the previous key become unrecoverable. In a crash-loop scenario (e.g., OOM, liveness probe failure), the key changes on every restart, causing repeated data loss. The empty file is never healed automatically.

**Fix recommendation:**  
If the file exists but is too short, truncate and overwrite it:

```go
f, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
if err != nil {
    slog.Warn("failed to open encryption key file", "error", err)
    return key
}
defer f.Close()
if _, err := f.WriteString(key + "\n"); err != nil {
    slog.Warn("failed to write encryption key file", "error", err)
}
slog.Info("Generated new encryption key", "file", keyFile)
return key
```

Using `O_TRUNC` instead of `O_EXCL` eliminates the race: the last writer wins, and an empty/corrupt file is always overwritten. If strict atomicity is required, keep `O_EXCL` but add a fallback that truncates the existing file when it is too short.

---

### M1 — `ReEncryptOnStartup` does not honor context cancellation mid-loop

**Severity:** MEDIUM  
**Location:** `server/router/api/v1/agent/service.go:250-310`

**Description:**  
The loop ignores `ctx.Err()` between iterations. If the parent context is canceled (e.g., `v1.go` 30s timeout or `main.go` 60s timeout), DB calls return `ctx.Err()`, but the error is discarded at lines 257 and 279. The loop continues to decrypt with the backup key and attempt re-encryption, delaying shutdown and potentially causing partial updates without logging that the operation was aborted.

**Exploit scenario:**  
An operator triggers key rotation with a custom short timeout. The context expires mid-loop. The function does not stop promptly. Some tenants have their keys re-encrypted, others do not. The final log does not reflect that the operation was aborted by context cancellation, misleading the operator into believing the rotation succeeded.

**Fix recommendation:**  
Add an early-exit check after each DB call:

```go
tenants, err := s.store.ListAgentTenants(ctx, &store.FindAgentTenant{})
if err != nil {
    slog.Error("failed to list tenants for re-encryption", "error", err)
    return
}
for _, tenant := range tenants {
    if ctx.Err() != nil {
        slog.Warn("re-encryption canceled", "error", ctx.Err())
        return
    }
    config, err := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &tenant.ID})
    if err != nil {
        slog.Error("failed to get tenant config", "tenant", tenant.Slug, "error", err)
        failed++
        continue
    }
    // ...
    for _, tenant := range tenants {
        if ctx.Err() != nil {
            slog.Warn("re-encryption canceled during bridge keys", "error", ctx.Err())
            return
        }
        keys, err := s.store.ListBridgeAuthKeys(ctx, tenant.ID)
        if err != nil {
            slog.Error("failed to list bridge auth keys", "tenant", tenant.Slug, "error", err)
            failed++
            continue
        }
        // ...
    }
}
```

---

### M2 — `NewService` `context.Background()` undermines the startup timeout wrapper

**Severity:** MEDIUM  
**Location:** `server/router/api/v1/agent/service.go:93` and `server/router/api/v1/v1.go:62, 66-68`

**Description:**  
`NewService` still uses `context.Background()` for `GetSystemSecret` and `UpsertSystemSecret` at line 93. This is called at `v1.go:62` **before** the 30-second timeout wrapper at lines 66-68. If the database is slow or unavailable, `NewService` can hang indefinitely, making the timeout wrapper unreachable.

```go
// v1.go:62 — no timeout
agentService := agent.NewService(store, profile)

// v1.go:66 — timeout starts AFTER NewService returns
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
agentService.EnsureTranscriptSigningKeys(ctx)
agentService.ReEncryptOnStartup(ctx)
```

**Exploit scenario:**  
A slow or unavailable database causes `NewService` to block forever on `GetSystemSecret` (line 94). The intended 30-second startup timeout never activates. The process appears hung, health checks fail, and the deployment stalls.

**Fix recommendation:**  
Either:
1. Move `NewService` inside the timeout wrapper, or
2. Pass a `ctx` into `NewService` and use it for secret operations, or
3. Use a short `context.WithTimeout` inside `NewService` itself for the secret fetch.

---

### L1 — Retry loop in `EnsureTranscriptSigningKeys` does not check `ctx.Err()` before each attempt

**Severity:** LOW  
**Location:** `server/router/api/v1/agent/service.go:1884-1904`

**Description:**  
The retry loop only checks `ctx.Err()` after a failed attempt and during backoff. If the context is canceled before the loop starts, the loop still runs all 3 attempts, each immediately returning `ctx.Err()`. This wastes a small amount of time but does not cause incorrect behavior.

**Exploit scenario:**  
None — this is a correctness/efficiency nit, not an exploitable vulnerability.

**Fix recommendation:**  
Add a pre-check:

```go
for attempt := 0; attempt < 3; attempt++ {
    if ctx.Err() != nil {
        saveErr = ctx.Err()
        break
    }
    if _, err := s.store.UpdateAgentTenant(ctx, tenant); err != nil {
        // ...
    }
}
```

---

## Verified Non-Findings

### HMAC `|` separator is unambiguous

**Location:** `server/router/api/v1/agent/service.go:1820, 1832`

`ValidateExternalSessionID` (`store/bridge.go:160`) enforces `^[A-Za-z0-9_-]{1,128}$`, which excludes `|`. The token is hex-encoded, so URL encoding is not an issue. **No vulnerability.**

### Injection prefix wording avoids detection patterns

**Location:** `server/router/api/v1/agent/service.go:2288`

`"[SUSPICIOUS INPUT — proceed with standard policy only]\n"` does not contain any string from `detectPromptInjection` (`service.go:2245-2273`). **No vulnerability.**

### Test encryption setup is correct

**Location:** `server/router/api/v1/agent/bridge_foundation_test.go:21-31, 432-437`

`setupTestSigningKey` uses the service’s initialized `EncryptionService` to encrypt `testSigningSeed`. The master key length (33 chars) is sufficient for Argon2id. `getTranscriptSigningSeed` decrypts successfully. **No vulnerability.**

---

## Summary Table

| ID | Severity | File | Issue |
|----|----------|------|-------|
| H1 | HIGH | `service.go:257,273,279,295` | Silent partial re-encryption due to ignored DB errors |
| H2 | HIGH | `bin/memos/main.go:320-329` | Empty-file crash loop silently changes master key |
| M1 | MEDIUM | `service.go:250-310` | Context cancellation not honored mid-loop |
| M2 | MEDIUM | `service.go:93`, `v1.go:62,66` | `NewService` timeout bypass |
| L1 | LOW | `service.go:1884-1904` | Retry loop does not pre-check context |

---

## Recommended Next Steps

1. **Fix H1 first** — silent partial re-encryption is the most likely production incident.
2. **Fix H2 second** — the empty-file bug can brick encryption in containerized/crash-loop environments.
3. **Fix M1 and M2** — these undermine the timeout guarantees that R1 was meant to provide.
4. Re-run `go build` and `go vet` after fixes.
5. Add a test for `getOrCreateEncryptionKey` that simulates an empty file and asserts the key is stable.

