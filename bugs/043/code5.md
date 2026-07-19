# Plan: Security Hardening Round 4 Review (Code 5) — implement valid findings

**Source:** `bugs/043/code4_summary_review.md` (reviewer A, adversarial review of Code 4)
**Goal:** Implement the valid findings (F1–F6), all of which were verified against live source at planning time. No CRITICAL findings; 1 HIGH, 2 MEDIUM, 3 LOW.

---

## Reconciliation / validity

| ID | Severity | Verified in code? | Implement? |
|----|----------|--------------------|-------------|
| F1 | HIGH | Yes — `main.go:354` `_ = os.Remove(keyFile)` on O_EXCL-failure; `return key` at 378 can diverge | **Yes** |
| F2 | MEDIUM | Yes — no rollback/resume; half-state permanent after backup env removed | **Yes (scoped: caller guardrail + resume + remediation msg)** |
| F3 | MEDIUM | Yes — `service.go:1933` re-reads env per request, silent WARN | **Yes (scoped: startup flag + INFO log)** |
| F4 | LOW | Yes — `lead_llm.go:104` raw `messageContent`, no `FlaggedInput` | **Yes** |
| F5 | LOW | Yes — `system: ` etc. removed → delimiter bypass | **Yes** |
| F6 | LOW | Yes — `service.go:100-108` auto-generates salt on any error/timeout | **Yes** |

Verified-safe (no change): R1-1 salt consistency, HMAC `|` separator, N2 `may_be_committed` logging, test encryption setup, `simulation.go` inherits guardrail.

---

## Task 1 — F1 (HIGH): Eliminate key-divergence `os.Remove` on O_EXCL failure
**File:** `bin/memos/main.go` (`getOrCreateEncryptionKey`, lines 342-369 loop)

Root cause: the O_EXCL-failure branch does `_ = os.Remove(keyFile)` (line 354) which can unlink a peer's in-flight key file, then `continue` and re-create its own → divergent key.

Fix:
- Remove the `os.Remove` from the O_EXCL-failure branch entirely.
- On O_EXCL failure: read the peer's file. If valid (`len>=16`) → `return k` (adopt). If still empty/short, `continue` to retry the loop (the peer will finish writing; a later `ReadFile` picks it up). Do **not** unlink a file we did not create.
- Keep the top-of-function `os.Remove` for the *locally-discovered* empty/short file (line 333) — that one we "own" because we just read it and decided to regenerate; but to be fully safe, also guard it: only remove if we are about to create. Simplest: drop the early remove; instead attempt O_EXCL and only on success overwrite. Since O_EXCL fails if file exists, the early-read-empty path is naturally handled by the retry loop adopting the peer's key once written.
- Remove the divergent `return key` last-resort (line 378): instead, if we cannot establish a stable key after retries, `panic` or `log.Fatal` — never return a local key that does not match the file. (Process won't start; operator must fix the volume/permissions. This is strictly safer than silent divergence.)
- Net: the only file mutation we ever perform is a successful `O_EXCL`+write that we own. No path unlinks a peer's file.

## Task 2 — F6 (LOW, do with F1 area): Never auto-generate salt on secret-load error/timeout
**File:** `server/router/api/v1/agent/service.go` (`NewService`, lines 91-111)

Current: `if err != nil || secret == nil { GenerateSalt(); UpsertSystemSecret(); }` — this fires on a 15s context timeout too, silently rotating the salt and desyncing all existing ciphertext.

Fix: distinguish "not found (first run)" from "error/timeout":
- Use a sentinel: attempt `GetSystemSecret`; if it returns a *not-found* condition, generate + store salt (first run).
- On any other error (incl. `ctx.Err()`), do **not** generate a salt. Log ERROR, set `svc.encryptionService = nil`, and return. Callers already handle nil (`ReEncryptOnStartup` returns early; `getTranscriptSigningSeed` returns error). This makes a slow DB fail loudly instead of silently disabling encryption.
- Note: `GetSystemSecret` error semantics — check the store impl; if it cannot distinguish not-found from error, add a `GetSystemSecretExists`/sentinel or inspect `err`. Simplest robust approach: try the query; if `secret == nil && err == nil` → first run (generate). If `err != nil` → fatal, no generate.

## Task 3 — F2 (MEDIUM, scoped): Non-atomic rotation guardrail + resume
**Files:** `bin/memos/main.go` (`rotateKeysCmd` ~175), `server/router/api/v1/agent/service.go` (`ReEncryptOnStartup`)

Full per-tenant transactional rollback is a large cross-DB change; scope to the safe, high-value parts:
- `rotateKeysCmd`: after `ReEncryptOnStartup`, if `failed > 0` (or `err != nil`), **return a non-zero error** (already does) AND print an explicit remediation line: "N tenants remain encrypted under the backup key (ENCRYPTION_MASTER_KEY_BACKUP). Do NOT remove that env var until a clean re-run reports 0 failures." Surface `succeeded`/`failed` counts.
- Resume semantics already exist for the API-key, bridge-key, and transcript-signing-key loops: each skips tenants already under the correct key (API/bridge: `len(OpenRouterAPIKeyEncrypted)==0` skip; transcript: `len(TranscriptSigningKey)==0` skip — but after a *partial* rotation the already-rotated tenants still have non-empty ciphertext, so a re-run would re-decrypt them with the backup key successfully and re-encrypt — which is actually idempotent and safe). Confirm re-run is safe: yes, because backup key can still decrypt the rotated ciphertext. Add a comment documenting that `rotate-keys` is safely re-runnable.
- Add a startup INFO log in `NewService`/`ReEncryptOnStartup` when `ENCRYPTION_MASTER_KEY_BACKUP` is set: "key-rotation overlap window active — backup key accepted for decryption". This addresses the F3 silent-downgrade visibility too.

## Task 4 — F3 (MEDIUM, scoped): Gate backup fallback behind startup flag + INFO log
**File:** `server/router/api/v1/agent/service.go` (`getTranscriptSigningSeed` ~1914; `NewService`/`ReEncryptOnStartup`)

- Compute a package-level (or Service-field) `rotationOverlapActive bool` once at startup: set true only when `ENCRYPTION_MASTER_KEY_BACKUP != ""` AND the process is in the rotation path (i.e., `ReEncryptOnStartup` is invoked / `backupSvc` was built). Simplest: set `svc.backupKeyActive = backupKey != ""` inside `NewService` when building `encryptionService` (the `crypto.NewEncryptionService` already derives `backupKey` from the env — expose it or mirror the check).
- `getTranscriptSigningSeed` consults `svc.backupKeyActive` instead of re-reading `os.Getenv` on every token verification. Log at **INFO** (not WARN) "transcript signing key decrypted via backup key (rotation overlap active)".
- This removes the per-request env re-read (TOCTOU/trust-surface) and makes the fallback explicit and startup-scoped. Keep the salt-consistent derivation (verified safe in review).

## Task 5 — F4 (LOW): Apply guardrail to lead-extraction LLM
**Files:** `server/router/api/v1/agent/lead_llm.go` (`ExtractContactInfoFull` ~227, `ExtractContactInfoLLM` ~50), `service.go` callers (4423, 5104)

- Add `flagged bool` param to `ExtractContactInfoFull(ctx, messageContent, conversationHistory, tenantPhone, existingDraft, flagged)` and thread into `ExtractContactInfoLLMCached` → `ExtractContactInfoLLM`.
- In `ExtractContactInfoLLM`, after building `systemPrompt`, if `flagged`, append the same `=== SECURITY GUARDRAIL ===` block (reuse a helper `appendInjectionGuardrail(sb *strings.Builder)` extracted from `buildSystemPrompt`/`buildRAGSystemPrompt` to avoid duplication).
- Update the 2 callers in `service.go` to pass `session.FlaggedInput`. Update `lead_extraction_test.go` call sites (`ExtractContactInfoFull(ctx, "", messages, "", nil)` → add `false`).
- (Optional) centralize the guardrail in one function so all LLM assembly sites use it.

## Task 6 — F5 (LOW): Re-add high-precision delimiter detection
**File:** `server/router/api/v1/agent/service.go` (`detectPromptInjection` ~2344)

- Re-add a small, high-precision set of delimiter-only patterns without the noisy generic phrases that caused false positives:
  - `"system:"` (word-boundary / line-start) — catches `system: ignore previous instructions...`
  - `"<|im_start"` and `"<|im_end"` 
  - `"### system"`
  - keep existing `"<|im_start|>system"`, `"[inst]"`, `"<<sys>>"`, `"```\nsystem"`
- Do NOT re-add `"you are a"`, `"assistant: "`, `"human: "` (high FP). The guardrail-in-system-prompt (N1) remains the primary defense; detection is heuristics-only.
- Use `strings.Contains` as before (keep simple); optionally trim/normalize leading whitespace before match for `system:`.

---

## Affected files
| File | Tasks |
|------|-------|
| `bin/memos/main.go` | F1, F2 (caller), F6-adjacent |
| `server/router/api/v1/agent/service.go` | F2, F3, F4(caller), F5, F6 |
| `server/router/api/v1/agent/lead_llm.go` | F4 |
| `server/router/api/v1/agent/lead_extraction_test.go` | F4 (call-site update) |

## Verification
1. `go build ./bin/memos/... ./server/router/api/v1/... ./store/... ./internal/crypto/...` — clean.
2. `go vet ./...` — clean.
3. `go test ./server/router/api/v1/agent/ -run Transcript` — PASS.
4. `go test ./server/router/api/v1/agent/ -skip 'Live|ChatExternal|Materialization|UnsupportedDB|Release'` — PASS (non-LLM).
5. New/updated tests:
   - F1: unit test for `getOrCreateEncryptionKey` simulating two concurrent processes on the same dir → assert both end up with the **same** key (no divergence); and that an empty pre-existing file is healed to a single stable key across calls.
   - F5: unit test `detectPromptInjection("system: ignore previous instructions")` → true; `detectPromptInjection("you are a happy customer")` → false (no FP regression).
   - F4: test that `ExtractContactInfoFull` with `flagged=true` produces a system prompt containing `=== SECURITY GUARDRAIL ===`.
6. Manual: run `rotate-keys` against a DB, kill mid-rotation (ctx timeout), confirm command exits non-zero with remediation text and that a re-run completes cleanly (resume).

## Out of scope / notes
- Full per-tenant transactional rollback for rotation: deferred (large cross-DB change); the guardrail + resume + remediation messaging mitigate the realistic operator failure mode.
- Multi-replica distributed key agreement: still out of scope (single-process-per-datadir assumption stands; F1 removes the last divergence path within that model).
- No schema migration required (F2 scoped to caller behavior + documentation of re-run safety).
