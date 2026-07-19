# Security Hardening — Code 7 Implementation Summary

**Date:** 2026-07-20
**Source reviews:** `bugs/043/code4_summary_review.md` (F1–F6) + `code5_plan_review.md` + `code6_plan_review.md` + `code7_plan_review.md`
**Status:** All 6 findings (F1–F6) implemented; `go build ./...` clean; `go vet` clean; non-LLM tests pass. Approved by `code7_plan_review.md` (minimum-viable, 3 non-blocking nits applied).

---

## Overview

Code 7 implements the six findings from the Code 4 adversarial review, with every rework item from `code5`/`code6`/`code7` plan reviews incorporated. Findings: 1 HIGH, 2 MEDIUM, 3 LOW. No CRITICAL. All changes are grounded in live source reads at implementation time.

---

## Reconciliation of findings

| ID | Sev | Implemented | Verified in code |
|----|-----|-------------|------------------|
| F1 | HIGH | `getOrCreateEncryptionKey` no longer removes a peer's in-flight key; last-resort is fatal | `bin/memos/main.go:323-388` |
| F2 | MEDIUM | `ReEncryptOnStartup` idempotent (backup→primary decrypt); `failed>0` returns error | `service.go` `decryptForRotation` + return at end |
| F3 | MEDIUM | `Service.backupKeyActive` field gates the transcript-seed backup fallback | `service.go` struct + `NewService` + `getTranscriptSigningSeed` |
| F4 | LOW | `FlaggedInput` threaded into lead-extraction LLM via shared guardrail helper | `lead_llm.go` + `service.go` callers + 3 test sites |
| F5 | LOW | High-precision delimiter patterns re-added to `detectPromptInjection` | `service.go` patterns list |
| F6 | LOW | `NewService` never auto-generates salt on secret-load error/timeout | `service.go` `NewService` block |

---

## Findings Implemented

### F1 — HIGH: Eliminate key-divergence `os.Remove` on O_EXCL failure
**File:** `bin/memos/main.go` (`getOrCreateEncryptionKey`, ~323-388)

- Kept the top-of-function `os.Remove(keyFile)` (line 333) — we own that file; if the remove itself fails it now **panics** with a clear message (fails safe rather than proceeding to a doomed O_EXCL).
- **Removed** the `os.Remove` in the O_EXCL-failure branch (was line 354): a peer's in-flight file is never unlinked. On O_EXCL failure we adopt the peer's key if valid, otherwise `continue` (the peer will finish writing; a later `ReadFile` adopts it).
- The final last-resort branch now **panics** ("refusing to start with a divergent master key") instead of returning a locally generated key. No path returns a key that does not match the on-disk file.

### F2 — MEDIUM: Idempotent/resumable rotation; `failed>0` is fatal
**File:** `server/router/api/v1/agent/service.go` (`ReEncryptOnStartup`)

- Added helper `decryptForRotation(backupSvc, ct, cn)` that tries `backupSvc.Decrypt` first, then `s.encryptionService.Decrypt` (the already-rotated case), returning a clear error only if both fail.
- All three decrypt sites (API key, bridge key, transcript signing key) now use it — making a partial/canceled rotation resumable: a re-run re-decrypts already-rotated tenants under the primary instead of counting them as `failed`.
- End of function: if `failed > 0`, returns a non-nil `fmt.Errorf(...)` with explicit remediation ("do NOT remove ENCRYPTION_MASTER_KEY_BACKUP until a clean re-run reports 0 failures"). `rotateKeysCmd` (`main.go`) already does `if err := svc.ReEncryptOnStartup(ctx); err != nil { return err }`, so a partial rotation now exits non-zero.

### F3 — MEDIUM: `Service`-scoped `backupKeyActive` + INFO log
**File:** `server/router/api/v1/agent/service.go`

- Added `backupKeyActive bool` to the `Service` struct (next to `encryptionService`).
- Set **inside** the `if p.EncryptionMasterKey != ""` block in `NewService`, only when `ENCRYPTION_MASTER_KEY_BACKUP != ""`, accompanied by a single INFO log "key-rotation overlap window active — backup key accepted for decryption".
- `getTranscriptSigningSeed` now gates the backup-key fallback on `s.backupKeyActive` (no per-request `os.Getenv` re-read / TOCTOU). Salt derivation is unchanged (still `crypto.NewEncryptionService(backup, systemSecret.EncryptionSalt)`, consistent with the original seal). Log changed WARN → INFO.

### F4 — LOW: Apply guardrail to lead-extraction LLM
**Files:** `service.go` (`appendInjectionGuardrail` helper + 2 callers), `lead_llm.go`, `lead_extraction_test.go`

- Extracted `appendInjectionGuardrail(sb *strings.Builder)` from the duplicated blocks in `buildSystemPrompt`/`buildRAGSystemPrompt` (now byte-identical, single source of truth).
- `ExtractContactInfoLLM` / `ExtractContactInfoLLMCached` / `ExtractContactInfoFull` take a `flagged bool` param. When flagged, the extraction system prompt gets the same `=== SECURITY GUARDRAIL ===` block. Cache key includes `flagged` so a guarded prompt isn't served from an unguarded cache entry.
- Both production callers (`service.go:4483, 5164`) pass `session.FlaggedInput`. All **3** test call sites (`lead_extraction_test.go:246, 261, 273`) updated to `false` (closes the code6 compile-break).

### F5 — LOW: Re-add high-precision delimiter detection
**File:** `server/router/api/v1/agent/service.go` (`detectPromptInjection`)

Re-added (without the noisy high-FP substrings): `"system:"`, `"<|im_start"`, `"<|im_end"`, `"### system"`. Kept existing `"<|im_start|>system"`, `"[inst]"`, `"<<sys>>"`, `` "```\nsystem" ``. Comment notes detection is heuristic-only; the system-prompt guardrail (N1) is primary defense.

### F6 — LOW: Never auto-generate salt on secret-load error/timeout
**File:** `server/router/api/v1/agent/service.go` (`NewService`)

- 15s bounded context retained.
- `GetSystemSecret` **error** → `slog.Error` + `encryptionService = nil` + `return` (no salt generation). Callers already handle nil (`ReEncryptOnStartup` early-returns; `getTranscriptSigningSeed` errors).
- `secret == nil && err == nil` (first run) → still generates + stores a salt.
- Verified both SQLite (`store/db/sqlite/rbac.go`) and Postgres (`store/db/postgres/rbac.go`) `GetSystemSecret` return `(nil, nil)` on not-found, so the first-run sentinel is reliable for both drivers.

---

## Files Changed

| File | Findings |
|------|----------|
| `bin/memos/main.go` | F1 |
| `server/router/api/v1/agent/service.go` | F2, F3, F4 (helper + callers), F5, F6 |
| `server/router/api/v1/agent/lead_llm.go` | F4 |
| `server/router/api/v1/agent/lead_extraction_test.go` | F4 (3 call sites) |

No schema migration required (all code-only).

---

## Verification

| Check | Result |
|-------|--------|
| `go build ./...` | Clean |
| `go vet ./bin/memos/... ./server/router/api/v1/... ./store/... ./internal/crypto/...` | Clean |
| `go test ./server/router/api/v1/agent/ -run Transcript` | PASS |
| `go test ./server/router/api/v1/agent/ -run 'TestTranscript|TestExtractContactInfoFull|TestDetectPromptInjection'` | PASS |
| `go test ./server/router/api/v1/agent/ -skip 'Live|ChatExternal|Materialization|UnsupportedDB|Release|Escalation'` | PASS (all non-LLM) |
| LLM-dependent `TestChatExternal*` | FAIL — require `OPENROUTER_API_KEY`; confirmed pre-existing (fails identically on baseline via `git stash`) |

---

## Verified-safe (no change)
- R1-1 salt consistency (backup fallback uses same `systemSecret.EncryptionSalt` as original seal).
- HMAC `|` separator (sessionID validated `[A-Za-z0-9_-]`, excludes `|`).
- N2 `may_be_committed` logging.
- Test encryption setup (`setupTestSigningKey`).
- `simulation.go` inherits `FlaggedInput`/guardrail via `processChat`.

## Residual / out of scope
- Full per-tenant transactional rollback for rotation: deferred (large cross-DB change). Idempotent resume + `failed>0`-is-fatal guardrail + explicit remediation mitigate the realistic operator failure mode.
- Multi-replica distributed key agreement: still out of scope; F1 removes the last intra-process divergence path under single-process-per-datadir.
- F1 now **panics** if a stable key cannot be established (corrupt/unreadable `.encryption_key` blocks startup rather than degrading). Safer tradeoff, but operators must fix the volume/permissions to boot.

---

## Adversarial Code Review Prompt (for Code 8)

```
You are a senior application security engineer performing a thorough adversarial
code review of the security hardening implementation described below (Code 7).
Your job is to find every vulnerability, logic error, and security anti-pattern.
Be aggressive — assume the developer made mistakes. Focus on exploitable issues,
not theoretical ones. Every claim must cite file:line from the CURRENT source.

## Scope

### F1 — getOrCreateEncryptionKey (bin/memos/main.go)
- Top-of-function os.Remove only when we own the file; O_EXCL-failure branch no
  longer removes the peer's file; last-resort panics instead of returning a local key.
- Question: Can two processes on the same data dir STILL end up with different
  keys under any interleaving (e.g., process A removes empty file, process B
  reads empty file and also proceeds; A's O_EXCL wins, B's O_EXCL fails, B
  retries — does B ever adopt A's key or diverge)? Trace every interleaving.
  Is the panic-on-last-resort reachable in normal operation (false positive
  that blocks boot)? Is `panic` the right failure mode for a long-running server
  (vs. returning error to caller)? Does any caller ignore the panic consequence?

### F2 — ReEncryptOnStartup idempotency (service.go)
- decryptForRotation tries backupSvc then s.encryptionService (primary).
- failed>0 now returns a non-nil error.
- Question: Is the backup->primary fallback CORRECT for the transcript-signing-key
  loop specifically (s.encryptionService.Decrypt on a tenant.TranscriptSigningKey
  already rotated under primary)? Any case where a tenant is double-rotated or
  skipped? If ctx is canceled mid-loop, partial rows are persisted under primary
  while the function returns ctx.Err(); on re-run, are those rows correctly
  re-adopted (primary decrypt succeeds) so failed reaches 0? Confirm the remediation
  message is accurate and not misleading.

### F3 — backupKeyActive (service.go)
- Service.backupKeyActive set in NewService inside EncryptionMasterKey!="" block;
  getTranscriptSigningSeed gates fallback on it; INFO log emitted once.
- Question: Is there any code path where backupKeyActive is true but
  encryptionService is nil (or vice versa), causing a missed/incorrect fallback?
  Does agentServiceForRotation (main.go) set the flag consistently with the main
  service? Is the flag ever read before it is set (nil-receiver / zero-value)?

### F4 — lead-extraction guardrail (lead_llm.go, service.go)
- ExtractContactInfoFull/LLMCached/LLM take `flagged bool`; appendInjectionGuardrail
  shared helper; cache key includes flagged.
- Question: Does the guardrail actually reach the extraction LLM, or is it
  appended to a system prompt that is then overwritten/ignored (e.g., the
  `systemPrompt` variable vs. the SystemMessage passed)? Are there OTHER assembly
  sites (simulation.go, playground.go, lead_llm other callers) that still ignore
  FlaggedInput? Is `flagged` ever stale (set on a prior message, not the current)?

### F5 — detectPromptInjection delimiters (service.go)
- Re-added "system:", "<|im_start", "<|im_end", "### system".
- Question: Does adding "system:" create a false positive that breaks legitimate
  transcripts (e.g., a customer pasting "system: ticket #123")? Is "<|im_start"
  a safe substring (catches "<im_start" too)? Any injection form still bypasses
  detection AND the system guardrail?

### F6 — NewService salt bootstrap (service.go)
- GetSystemSecret error -> encryptionService=nil, return (no salt gen).
- Question: If encryptionService is nil, are ALL downstream callers safe (no nil
  deref)? Specifically ReEncryptOnStartup (early return?), getTranscriptSigningSeed
  (early return at encryptionService==nil?), and any handler that assumes a
  non-nil service? Is "first run" correctly distinguished from "DB error" for
  both SQLite and Postgres drivers?

## Attack Vectors to Check
1. Master-key divergence across processes/restart despite F1.
2. Partial key rotation leaving tenants permanently unrecoverable (F2 + R1-1
   backup fallback interaction).
3. Backup-key trust surface / env-injection to decrypt transcript seeds (F3).
4. Any LLM message-assembly path that ignores FlaggedInput (F4).
5. Nil encryptionService after a secret-load failure causing panics or silent
   decryption loss (F6).

## Output Format
For each finding: Severity (CRITICAL/HIGH/MEDIUM/LOW/INFO), Location (file:line),
Description, Exploit scenario, Fix recommendation. Reference exact line numbers
and code snippets. Focus on exploitable vulnerabilities; skip theoretical noise.
```

---

**End of summary.**
