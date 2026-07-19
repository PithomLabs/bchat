# Plan: Security Hardening Round 3 — Code 4 Implementation

**Source reviews:** `bugs/043/code3_review2.md` (reviewer A) + `bugs/043/code3_review2_hy3.md` (reviewer hy3)
**Goal:** Implement the valid findings from both adversarial reviews, reconcile overlaps, and keep existing passing tests green.

---

## Reconciliation of findings

| Reviewer A (code3_review2.md) | Reviewer hy3 (code3_review2_hy3.md) | Merged action | Valid? |
|---|---|---|---|
| H1 — 4 DB errors ignored in `ReEncryptOnStartup` (lines 257,273,279,295) | (covered) | Fix H1 | **Yes** |
| H2 — empty-file crash loop changes master key | M7-1 — race fallback returns divergent local key | Fix M7: heal empty/short file; never return local key on contention | **Yes** |
| M1 — context cancellation not honored mid-loop | (covered) | Fix M1 | **Yes** |
| M2 — `NewService` uses `context.Background()` | R1-2 — same | Fix R1-2 | **Yes** |
| L1 — retry loop no pre-check of `ctx.Err()` | N2-1 — partial-success desync on ctx cancel | Fix N2: pre-check + harden partial success | **Yes** |
| (not present) | R1-1 — `TranscriptSigningKey` NOT re-encrypted on rotation → permanent token break | Fix R1-1 | **Yes** |
| (not present) | N1 — prepended prefix is directive text inside `user` turn; high FP detection | Fix N1 | **Yes (MEDIUM)** |
| HMAC `|` — verified safe | L11 — verified safe | No change | No |
| Test setup — verified correct | R2 — verified correct | No change | No |
| Regex/redaction — verified correct | M4/M6 — verified correct | No change | No |

**Net work items:** H1, M7 (H2+M7-1 merged), M1, R1-2 (M2), N2 (L1+N2-1 merged), R1-1, N1.

---

## Implementation tasks

### Task 1 — H1: Handle all DB errors in `ReEncryptOnStartup`
**File:** `server/router/api/v1/agent/service.go` (lines ~257, 273, 279, 295)

Replace each discarded error with checked + logged + `failed++` + `continue`/`return`:
- `config, _ := s.store.GetTenantConfig(...)` → check, log `"tenant"`, `failed++`, `continue`.
- `s.store.UpsertTenantConfig(ctx, config)` → check, log, `failed++`, `continue`.
- `keys, _ := s.store.ListBridgeAuthKeys(...)` → check, log, `failed++`, `continue`.
- `s.store.RevokeBridgeAuthKey(...)` → check, log `"key_id"`, `failed++`, `continue`.
- `CreateBridgeAuthKey` (line ~304) already checked; keep, ensure `failed++` on err.

Also change the function signature to return results so callers can fail:
`func (s *Service) ReEncryptOnStartup(ctx context.Context) (succeeded, failed int, err error)`
- On fatal early error (secret/list tenants) return `(0, 0, err)`.
- Accumulate `success`/`failed` and `return success, failed, nil` at end.
- Update callers: `v1.go:70` ignore return (best-effort startup); `main.go:175` check `failed>0` and `return fmt.Errorf("re-encryption partially failed: %d", failed)`.

### Task 2 — M7: Heal corrupt/empty key file; never diverge on contention
**File:** `bin/memos/main.go` (`getOrCreateEncryptionKey`, lines 306-338)

Root cause (both H2 + M7-1): an empty/short `.encryption_key` file (crash between O_EXCL and write, OR losing-instance re-read) currently falls back to a locally generated UUID → divergent key.

Rewrite the contention/empty-file branch:
- On `O_EXCL` success: write + `f.Sync()` + `f.Close()`; on write error, `os.Remove(keyFile)` and return error (do not leave a partial file).
- On `O_EXCL` failure (file exists): re-read. If content `len(trimmed) >= 16` → return it (normal race win). If too short/empty (crashed winner) → `os.Remove(keyFile)` then retry the `O_EXCL` open once; if still failing, return error. **Never** return the local `key`.
- Add a `teststore`-style unit test simulating an empty file: assert the key stabilizes (same value across two calls) and is not the pre-generated UUID.

Note: this function is single-process per pod (`dataDir` is local). The multi-replica shared-DB race is out of scope; the fix still closes the divergence path.

### Task 3 — M1: Honor context cancellation mid-loop
**File:** `server/router/api/v1/agent/service.go` (loops at ~256 and ~278)

After `ListAgentTenants` and at the top of **each** tenant iteration in both loops, add:
```go
if ctx.Err() != nil {
    slog.Warn("re-encryption canceled", "error", ctx.Err())
    return success, failed, ctx.Err()
}
```
This makes a timeout abort cleanly and surface in the returned `err`.

### Task 4 — R1-2: Propagate context into `NewService` secret load
**File:** `server/router/api/v1/agent/service.go` (`NewService` line 82, secret load 92-104); `v1.go:62-68`

- Add `ctx context.Context` param to `NewService`, or create a short `context.WithTimeout` inside for the `GetSystemSecret`/`UpsertSystemSecret` calls (lines 93-103). Simplest: inside `NewService`, wrap the secret fetch in a 10s timeout so a slow DB cannot hang startup indefinitely.
- Keep `v1.go:62` call but ensure the 30s timeout at `v1.go:66` still wraps `EnsureTranscriptSigningKeys`/`ReEncryptOnStartup`. Update any other `NewService` call sites (grep: all callers) to pass a context.

### Task 5 — N2: Harden retry loop partial-success
**File:** `server/router/api/v1/agent/service.go` (`EnsureTranscriptSigningKeys`, lines ~1884-1908)

- Add `ctx.Err()` pre-check at the start of each attempt (covers L1).
- After the loop, if `saveErr != nil` due to `ctx.Err()`, do **not** let the next startup regenerate the key blindly. Log WARN with tenant slug and a clear "manual reconcile may be required" message. Keep `continue` (skip) rather than aborting the whole migration.
- Keep existing backoff; total (700ms) is well under parent timeout — no change needed there.

### Task 6 — R1-1: Re-encrypt `TranscriptSigningKey` on rotation
**File:** `server/router/api/v1/agent/service.go` (`ReEncryptOnStartup`, add to the tenant loop)

After the API-key loop, for each tenant with `TranscriptSigningKey` set:
```go
if len(tenant.TranscriptSigningKey) > 0 {
    pt, derr := backupSvc.Decrypt(tenant.TranscriptSigningKey, tenant.TranscriptSigningKeyNonce)
    if derr != nil { failed++; continue }
    ct, nonce, eerr := s.encryptionService.Encrypt(pt)
    if eerr != nil { failed++; continue }
    tenant.TranscriptSigningKey = ct
    tenant.TranscriptSigningKeyNonce = nonce
    if _, uerr := s.store.UpdateAgentTenant(ctx, tenant); uerr != nil {
        slog.Error("failed to re-encrypt transcript signing key", "tenant", tenant.Slug, "error", uerr)
        failed++; continue
    }
    success++
}
```
Also add a fallback in `getTranscriptSigningSeed` (lines 1840-1853): if primary decrypt fails, try `backupSvc` when `ENCRYPTION_MASTER_KEY_BACKUP` is set, with a WARN log. This keeps existing tokens valid during the overlap window and fails cleanly (not panic) once the backup var is removed.

Add a test: encrypt a seed under backup key, rotate to primary, assert `getTranscriptSigningSeed` still decrypts (backup fallback) while backup env set, and returns error after removal.

### Task 7 — N1: Stop prepending directive text into the user turn; reduce false positives
**File:** `server/router/api/v1/agent/service.go` (`processChat` 2286-2288; `detectPromptInjection` 2243-2279)

- Keep detection as logging-only (per its doc comment) but **do not** concatenate instruction-like text into `userMessage` that later becomes `openrouter.UserMessage` (line 2746 / 3328). Instead, record the flag on the session (e.g., `session.Flagged = true` or a field) and have `buildSystemPrompt`/`buildRAGSystemPrompt` emit a single system-level guardrail note — keeping the marker in the **system** prompt, not the user turn.
- Remove the highest-false-positive substrings from `detectPromptInjection` that appear in legitimate transcripts: `"system: "`, `"assistant: "`, `"human: "`, `"you are a"`, `"you will now"` (keep clearly adversarial ones: "ignore previous instructions", "disregard your instructions", "new system prompt:", "<|im_start|>system", etc.).
- Ensure the marker wording avoids detection trigger words (already does: no instruction/override/role/system/injection/prompt).

---

## Affected files
| File | Tasks |
|------|-------|
| `server/router/api/v1/agent/service.go` | H1, M1, R1-2, N2, R1-1, N1 |
| `bin/memos/main.go` | M7 (H2+M7-1) |
| `server/router/api/v1/v1.go` | R1-2 (call-site) |
| `server/router/api/v1/agent/bridge_foundation_test.go` | add M7 unit test + R1-1 rotation test |
| `server/router/api/v1/agent/bridge_delivery_test.go` | adjust if N1 changes session flag/field shape |

## Verification
1. `go build ./bin/memos/main.go` — clean.
2. `go vet ./bin/memos/... ./server/router/api/v1/... ./store/...` — clean.
3. `validate-migrations.sh` — LATEST.sql in sync (no schema change needed).
4. New/updated tests:
   - M7: empty `.encryption_key` → key stabilizes across calls, no divergence.
   - R1-1: pre-rotation transcript token verifies post-rotation (backup fallback) and fails cleanly after backup removed.
   - N1: flagged message stored as session flag, not concatenated into user content (assert `session.Messages` user content has no `[SUSPICIOUS INPUT` prefix).
5. Existing `Transcript*` and `TestBChatLiveHumanReplyAppearsInVisitorTranscript` — must still PASS.
6. Manual: run `rotateKeysCmd` with a backup key, assert exit 0 only when `failed==0`; simulate 30s timeout to confirm clean abort log.

## Out of scope / notes
- Multi-replica shared-data-dir key race (M7) is mitigated for the single-process case; true distributed coordination is a separate design effort.
- No schema migrations required.
- The `|` HMAC separator and test-encryption setup are verified correct — no changes.
