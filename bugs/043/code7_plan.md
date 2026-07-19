# Plan: Security Hardening — Code 7 (implement F1–F6 with all rework applied)

**Source:** `bugs/043/code4_summary_review.md` (F1–F6) + `code5_plan_review.md` (rework) + `code6_plan_review.md` (final rework/nits)
**Verdict:** All 6 findings valid. Every technical premise re-verified against current source at plan time. Two rework items from code6 (F4 test call-site count, F2 `failed>0` returns nil) are resolved below; F3 placement nit applied. Plan is implementation-ready.

---

## Verified premises (current source)

- **F1** — `bin/memos/main.go`: top-of-function `os.Remove` at line 333 (keep), O_EXCL-failure `os.Remove` at line 354 (DROP), write-failure partial remove at 360 (keep), last-resort `return key` at line 362 (make FATAL). ✅
- **F2** — `service.go` three decrypt sites: API-key `:278`, bridge-key `:316`, transcript `:362` — all decrypt **only** with `backupSvc` (no primary fallback). `ReEncryptOnStartup` ends `return succeeded, failed, nil` (`:385`) → `err` is nil even when `failed > 0`. `rotateKeysCmd` caller (`main.go:175`) is `if err := svc.ReEncryptOnStartup(ctx); err != nil { return err }` → exits 0 on partial rotation. ✅ (both rework items confirmed)
- **F3** — `getTranscriptSigningSeed` reads `os.Getenv("ENCRYPTION_MASTER_KEY_BACKUP")` at `:1933`; `Service` struct has `encryptionService *crypto.EncryptionService` (no `backupKeyActive` yet). `NewService` block `:92-111` (inside `if p.EncryptionMasterKey != ""`). ✅
- **F4** — `ExtractContactInfoFull` (`lead_llm.go:227`): **2 production** callers (`service.go:4423, 5104`, both have `session`) and **3 test** callers (`lead_extraction_test.go:246, 261, 273`). ✅ (3, not 2)
- **F5** — current `detectPromptInjection` (`:2344-2376`) does NOT contain `"you are a"`, `"system: "`, etc.; re-adding `"system:"`, `"<|im_start"`, `"<|im_end"`, `"### system"` is correct. ✅
- **F6** — SQLite `GetSystemSecret` `(nil,nil)` on not-found (`store/db/sqlite/rbac.go:534-535`); Postgres same (`store/db/postgres/rbac.go:213-215`). `secret == nil && err == nil` is a reliable first-run sentinel. ✅

---

## Task 1 — F1 (HIGH): close O_EXCL-failure `os.Remove` divergence
**File:** `bin/memos/main.go` (`getOrCreateEncryptionKey`, ~323-379)

- Keep top-of-function `os.Remove(keyFile)` (line 333) — we own that file. If the remove fails (permissions), `log.Fatal`/panic with "cannot clear corrupt key file" — do NOT fall through to an O_EXCL that will then fail.
- Loop `for attempt := 0; attempt < 2; attempt++`:
  - `O_EXCL` success → write `key + "\n"`, `f.Close()`, return `key`.
  - write error → `_ = os.Remove(keyFile)` (360, keep), return `key` (best effort).
  - `O_EXCL` failure (peer holds slot):
    - `ReadFile`; if valid (`len >= 16`) → `return k` (adopt).
    - if still empty/short → `continue` (**NO `os.Remove`** — this is the line 354 removal to DROP). The peer finishes writing; a later `ReadFile` adopts it.
- After loop: `ReadFile`; if valid → return it.
- Else → `log.Fatal` / panic "unable to establish a stable encryption key file; tenant secret encryption may be inconsistent" — **never `return key`** (no divergent local key).

## Task 2 — F6 (LOW): never auto-generate salt on secret-load error/timeout
**File:** `service.go` (`NewService`, ~91-111)

- Keep the 15s bounded ctx.
- `secret, err := s.GetSystemSecret(ctx)`:
  - `err == nil && secret != nil` → use it.
  - `err == nil && secret == nil` → first run → `GenerateSalt()` + `UpsertSystemSecret` (keep generating).
  - `err != nil` (incl. 15s `ctx.Err()`) → slog.Error "failed to load system secret for encryption bootstrap"; set `svc.encryptionService = nil`; `return` (NO salt generation). Callers already handle nil (`ReEncryptOnStartup` early-returns; `getTranscriptSigningSeed` returns error).

## Task 3 — F2 (MEDIUM): idempotent/resumable rotation + `failed>0` is fatal
**Files:** `service.go` (`ReEncryptOnStartup` decrypt sites ~278, ~316, ~transcript loop; return at ~384-385), `bin/memos/main.go` (`rotateKeysCmd` ~175)

- Make all three decrypt sites idempotent (backup first, then primary):
  ```go
  plaintext, dErr := backupSvc.Decrypt(ct, cn)
  if dErr != nil {
      // Already re-encrypted under the primary key on a prior (partial) run.
      plaintext, dErr = s.encryptionService.Decrypt(ct, cn)
      if dErr != nil {
          slog.Error("failed to decrypt tenant secret (neither backup nor primary)", "tenant", tenant.Slug, "error", dErr)
          failed++
          continue
      }
  }
  ```
  Apply to: API-key (`config.OpenRouterAPIKeyEncrypted`), bridge-key (`key.SecretKeyEncrypted`), transcript (`tenant.TranscriptSigningKey`).
- At the end of `ReEncryptOnStartup` (replace `:384-385`):
  ```go
  slog.Info("Re-encryption complete", "succeeded", succeeded, "failed", failed)
  if failed > 0 {
      return succeeded, failed, fmt.Errorf("key rotation partially failed: %d of %d secrets not re-encrypted; tenants still under the backup key remain — do NOT remove ENCRYPTION_MASTER_KEY_BACKUP until a clean re-run reports 0 failures", failed, succeeded+failed)
  }
  return succeeded, failed, nil
  ```
  (Keep the existing `ctx.Err()` early-returns as-is — those already set `err`.) This guarantees `failed > 0` yields a non-nil error → `rotateKeysCmd` exits non-zero and the remediation text fires (Option A from code6 review).
- `rotateKeysCmd` caller: keep `if err := svc.ReEncryptOnStartup(ctx); err != nil { return err }` (now errors on partial). On success path, log INFO "key rotation complete; safe to remove ENCRYPTION_MASTER_KEY_BACKUP".
- Single consolidated overlap log: in `NewService`, when `backupKey != ""`, emit INFO "key-rotation overlap window active — backup key accepted for decryption" once (covers F3 visibility).

## Task 4 — F3 (MEDIUM): `Service`-scoped `backupKeyActive` + INFO log
**Files:** `service.go` (`Service` struct ~70, `NewService` ~92-111, `getTranscriptSigningSeed` ~1914-1944)

- Add `backupKeyActive bool` field to `Service` struct near `encryptionService`.
- In `NewService`, **inside** the `if p.EncryptionMasterKey != ""` block (after building `encryptionService`), set `svc.backupKeyActive = os.Getenv("ENCRYPTION_MASTER_KEY_BACKUP") != ""`. (Consistent with `crypto.NewEncryptionService`, which derives a backup key from the same env; and with `encryptionService != nil` so the flag is never true while the service is unusable.)
- `getTranscriptSigningSeed`: replace `if backup := os.Getenv("ENCRYPTION_MASTER_KEY_BACKUP"); backup != "" {` (`:1933`) with `if s.backupKeyActive {`. Keep the salt-consistent `crypto.NewEncryptionService(backup, systemSecret.EncryptionSalt)` derivation (verified safe). Change the WARN → INFO: "transcript signing key decrypted via backup key (rotation overlap active)".

## Task 5 — F4 (LOW): apply guardrail to lead-extraction LLM
**Files:** `service.go` (callers 4423, 5104), `lead_llm.go` (`ExtractContactInfoFull` 227, `ExtractContactInfoLLMCached` 199, `ExtractContactInfoLLM` 50), `lead_extraction_test.go` (call sites 246, 261, 273)

- Extract `func appendInjectionGuardrail(sb *strings.Builder)` from the duplicated block in `buildSystemPrompt` (~3263) and `buildRAGSystemPrompt` (~3762).
- Add `flagged bool` param to `ExtractContactInfoFull`, thread into `ExtractContactInfoLLMCached` → `ExtractContactInfoLLM`.
- In `ExtractContactInfoLLM`, after building the system prompt, if `flagged` → append `=== SECURITY GUARDRAIL ===` block (via the helper) to the system message content before the `openrouter.UserMessage(...)` extraction target (lead_llm.go:104).
- Update the **2 production** callers to pass `session.FlaggedInput`.
- Update **ALL 3 test** call sites (246, 261, 273) from `ExtractContactInfoFull(ctx, "", messages, "", nil)` → `ExtractContactInfoFull(ctx, "", messages, "", nil, false)`. (Missing line 261 breaks the test build.)

## Task 6 — F5 (LOW): re-add high-precision delimiter detection
**File:** `service.go` (`detectPromptInjection` ~2344-2376)

Add (do NOT re-add high-FP `"you are a"`, `"assistant: "`, `"human: "`):
- `"system:"` — catches `system: ignore previous instructions` (the F5 bypass; also matches `"system prompt:"` substring, acceptable)
- `"<|im_start"` and `"<|im_end"` — prefix match; also covers `"<|im_start|>system"`
- `"### system"` — catches `### system:` too

Keep existing `"<|im_start|>system"`, `"[inst]"`, `"<<sys>>"`, `` "```\nsystem" ``. All via `strings.Contains` (no word-boundary). Comment: detection is heuristic; the system-prompt guardrail (N1) is primary defense.

---

## Affected files
| File | Tasks |
|------|-------|
| `bin/memos/main.go` | F1, F2 (caller) |
| `server/router/api/v1/agent/service.go` | F2, F3, F4 (caller), F5, F6 |
| `server/router/api/v1/agent/lead_llm.go` | F4 |
| `server/router/api/v1/agent/lead_extraction_test.go` | F4 (3 call sites) |

No schema migration (all code-only).

## Verification
1. `go build ./bin/memos/... ./server/router/api/v1/... ./store/... ./internal/crypto/...` — clean.
2. `go vet ./...` — clean (catches the F4 arity regression if any test site missed).
3. `go test ./server/router/api/v1/agent/ -run Transcript` — PASS.
4. `go test ./server/router/api/v1/agent/ -skip 'Live|ChatExternal|Materialization|UnsupportedDB|Release'` — PASS (non-LLM).
5. New/updated tests:
   - **F1:** (a) pre-create empty/short `.encryption_key`, assert single call heals to one stable key; (b) two goroutines calling on same temp dir → assert equal returned keys + consistent on-disk content (no divergence).
   - **F2 resume:** simulate a partial rotation (ctx-cancel mid-loop), then a second `ReEncryptOnStartup` call with fresh ctx + backup key still set → asserts remaining tenants reach `failed == 0` (idempotency), and that an already-rotated (primary-sealed) tenant is reprocessed to `failed == 0` on re-run (core of the original false-premise bug). Assert `ReEncryptOnStartup` returns non-nil error when `failed > 0`.
   - **F4:** `ExtractContactInfoFull(..., false)` compiles + passes; `ExtractContactInfoFull(..., true)` produces a system prompt containing `=== SECURITY GUARDRAIL ===`.
   - **F5:** `detectPromptInjection("system: ignore previous instructions")` → true; `detectPromptInjection("you are a happy customer")` → false (no FP regression).
6. Manual: run `rotate-keys`, kill mid-rotation (ctx timeout) → non-zero exit + corrected remediation text; clean re-run reaches 0 failures (resume works).

## Out of scope / notes
- Full per-tenant transactional rollback for rotation: deferred (large cross-DB change). Idempotent resume + `failed>0`-is-fatal guardrail + explicit remediation mitigate the realistic operator failure mode.
- Multi-replica distributed key agreement: still out of scope; Task 1 removes the last intra-process divergence path under single-process-per-datadir.
- Verified-safe (no change): R1-1 salt consistency, HMAC `|` separator, N2 `may_be_committed` logging, test encryption setup, `simulation.go` inherits guardrail.
