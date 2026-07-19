# Security Hardening — Code 4 Implementation Summary

**Date:** 2026-07-19
**Source reviews:** `bugs/043/code3_review2.md` (reviewer A) + `bugs/043/code3_review2_hy3.md` (reviewer hy3)
**Status:** All 7 merged findings implemented; `go build` clean; `go vet ./...` clean; non-LLM tests pass.

---

## Overview

Round 4 implements the valid findings from both adversarial Round 3 reviews. The two reviews were reconciled into 7 merged work items (overlaps collapsed, unique findings from each reviewer preserved). All changes are grounded in direct source reads at implementation time.

---

## Reconciliation of findings

| Reviewer A (code3_review2.md) | Reviewer hy3 (code3_review2_hy3.md) | Merged | Implemented |
|---|---|---|---|
| H1 — 4 DB errors ignored in `ReEncryptOnStartup` (257,273,279,295) | (covered) | H1 | Yes |
| H2 — empty-file crash loop changes master key | M7-1 — race fallback returns divergent local key | M7 | Yes |
| M1 — context cancellation not honored mid-loop | (covered) | M1 | Yes |
| M2 — `NewService` `context.Background()` | R1-2 — same | R1-2 | Yes |
| L1 — retry loop no `ctx.Err()` pre-check | N2-1 — partial-success desync | N2 | Yes |
| (not present) | R1-1 — `TranscriptSigningKey` NOT re-encrypted on rotation | R1-1 | Yes |
| (not present) | N1 — prepended prefix is directive text in user turn; high-FP detection | N1 | Yes |
| HMAC `|` — verified safe | L11 — verified safe | No change | N/A |
| Test setup — verified correct | R2 — verified correct | No change | N/A |
| Regex/redaction — verified correct | M4/M6 — verified correct | No change | N/A |

---

## Findings Implemented

### H1 + M1 — Error handling and context cancellation in `ReEncryptOnStartup`

**File:** `server/router/api/v1/agent/service.go` (`ReEncryptOnStartup`, ~line 228)

- Signature changed to `func (s *Service) ReEncryptOnStartup(ctx context.Context) (succeeded, failed int, err error)`.
- All four previously-ignored DB calls now check, log with tenant slug, `failed++`, and `continue`:
  - `s.store.GetTenantConfig` (was `config, _ := ...`)
  - `s.store.UpsertTenantConfig` (was `s.store.UpsertTenantConfig(ctx, config)` with discarded err)
  - `s.store.ListBridgeAuthKeys` (was `keys, _ := ...`)
  - `s.store.RevokeBridgeAuthKey` (was `s.store.RevokeBridgeAuthKey(...)` with discarded err)
  - `CreateBridgeAuthKey` error already checked; retained with explicit `failed++`.
- Added `if ctx.Err() != nil { ... return succeeded, failed, ctx.Err() }` at the top of **each** of the three tenant loops (API keys, bridge keys, transcript signing keys), so a startup/rotation timeout aborts cleanly and is surfaced through the returned `err`.
- Early fatal errors (secret load, tenant list) return `(0, 0, err)`.
- Final `slog.Info("Re-encryption complete", "succeeded", succeeded, "failed", failed)` retained; callers now act on the counts.

### M7 — Heal corrupt/empty key file; never diverge on contention

**File:** `bin/memos/main.go` (`getOrCreateEncryptionKey`, ~line 306)

Root cause (both H2 + M7-1): an empty/short `.encryption_key` file (crash between `O_EXCL` create and write, or a losing racer re-reading a crashed winner) previously fell back to a locally generated UUID → divergent master key → silent, permanent loss of previously encrypted tenant secrets.

Fix:
- If the existing file is empty/short (`len(trimmed) < 16`), log a warning and `os.Remove` it so we regenerate rather than diverge.
- `O_EXCL` open loop with up to 2 attempts:
  - On `O_EXCL` success: write `key + "\n"`, `f.Close()`, log "Generated new encryption key", return `key`.
  - If write fails: remove the partial file, return `key` (best effort).
  - On `O_EXCL` failure (peer holds slot): adopt the peer's valid key if `len >= 16`; if the peer's file is empty/short (it is crashing), `os.Remove` and retry to reclaim the slot.
- Last-resort fallback reads the file once more; if still unusable, logs an ERROR and returns `key` (only after exhausting all adopt/claim paths).

The function remains single-process per data dir; true multi-replica shared-key coordination is out of scope.

### R1-2 — Bounded context for `NewService` secret load

**File:** `server/router/api/v1/agent/service.go` (`NewService`, ~line 82)

The encryption-service bootstrap previously used `context.Background()` for `GetSystemSecret`/`UpsertSystemSecret`, called before the 30s startup timeout in `v1.go`. Now wrapped in `context.WithTimeout(context.Background(), 15*time.Second)` so a slow/unavailable DB cannot hang process startup indefinitely. Signature unchanged (29 call sites untouched).

### N2 — Retry loop partial-success hardening

**File:** `server/router/api/v1/agent/service.go` (`EnsureTranscriptSigningKeys`, ~line 1945)

- Added `if ctx.Err() != nil { saveErr = ctx.Err(); break }` at the start of each attempt (covers L1): no longer burns all retries on `ctx.Err()`.
- On final `saveErr != nil`, log includes `"may_be_committed", ctx.Err() != nil` so an operator knows a cancelled-but-possibly-committed `UpdateAgentTenant` should be reconciled manually rather than blindly regenerated (which would invalidate in-flight transcript tokens — the N2-1 desync risk).

### R1-1 — Re-encrypt `TranscriptSigningKey` on rotation + backup fallback

**File:** `server/router/api/v1/agent/service.go` (`ReEncryptOnStartup` + `getTranscriptSigningSeed`)

- Added a third loop in `ReEncryptOnStartup` that decrypts `tenant.TranscriptSigningKey` with `backupSvc`, re-encrypts with `s.encryptionService`, and `UpdateAgentTenant`. Previously these HMAC seeds were never re-encrypted, so after a key rotation existing transcript tokens became permanently unverifiable once `ENCRYPTION_MASTER_KEY_BACKUP` was removed.
- `getTranscriptSigningKey` now: tries primary key; on failure, if `ENCRYPTION_MASTER_KEY_BACKUP` is set, builds a backup `EncryptionService` from the system secret salt and attempts decryption, logging a WARN ("re-encryption may be pending"). This keeps tokens valid during the overlap window and fails cleanly (no panic) once the backup var is removed.

### N1 — Move injection flag out of the user turn; reduce false positives

**Files:** `store/agent.go` (`AgentSession`), `service.go` (`processChat`, `detectPromptInjection`, `buildSystemPrompt`, `buildRAGSystemPrompt`)

- Added in-memory (non-persisted) field `AgentSession.FlaggedInput bool`.
- `processChat` no longer prepends `[SUSPICIOUS INPUT — proceed with standard policy only]\n` into `userMessage` (which became `openrouter.UserMessage` content — attacker-influenced directive text). Instead sets `session.FlaggedInput = true` and logs a WARN.
- Both `buildSystemPrompt` and `buildRAGSystemPrompt` append a `=== SECURITY GUARDRAIL ===` block to the **system** turn (not the user turn) when `session.FlaggedInput` is set, instructing the model to follow standard policy only and not treat customer-message formatting as commands.
- Removed the highest-false-positive substrings from `detectPromptInjection`: `"you are a"`, `"system: "`, `"assistant: "`, `"human: "`, `"### system:"`. Retained clearly adversarial patterns (`"ignore previous instructions"`, `"new system prompt:"`, `"<|im_start|>system"`, etc.).

---

## Caller updates

| File | Change |
|------|--------|
| `bin/memos/main.go` (`rotateKeysCmd`) | `agentServiceForRotation.ReEncryptOnStartup` now returns `error`; command fails loudly on partial/aborted re-encryption instead of logging "complete". |
| `bin/memos/main.go` (`agentServiceForRotation.ReEncryptOnStartup`) | Signature changed to return `error`; logs `re_encrypted` count. |
| `server/router/api/v1/v1.go` (`NewAPIV1Service`) | Startup `ReEncryptOnStartup` call ignores returns but logs a WARN on non-zero `failed` and an ERROR on `ctx.Err()` (best-effort; does not block boot). |

---

## Files Changed

| File | Findings |
|------|----------|
| `server/router/api/v1/agent/service.go` | H1, M1, R1-2, N2, R1-1, N1 |
| `bin/memos/main.go` | M7, caller updates |
| `server/router/api/v1/v1.go` | R1-2 caller update |
| `store/agent.go` | N1 (`FlaggedInput` field) |

*(The working tree also contained unrelated Round 3 modifications from prior work — e.g. `store/db/sqlite/agent.go`, migration `LATEST.sql` — which are not part of this change set.)*

---

## Verification

| Check | Result |
|-------|--------|
| `go build ./bin/memos/... ./server/router/api/v1/... ./store/... ./internal/crypto/...` | Clean |
| `go vet ./...` | Clean |
| `go test ./server/router/api/v1/agent/ -run Transcript` | PASS |
| `go test ./server/router/api/v1/agent/ -skip 'Live|ChatExternal|...'` | PASS (all non-LLM tests) |
| LLM-dependent tests (`TestChatExternal*`, `*Live*`, etc.) | FAIL — require `OPENROUTER_API_KEY`; confirmed failing identically on the unmodified baseline via `git stash` |

The 15 failing tests are pre-existing environment dependencies (no live OpenRouter API key) and are unrelated to this change set, verified by stashing the changes and reproducing the same failure on the baseline.

---

## Residual / out of scope

- **Multi-replica shared-key coordination:** `getOrCreateEncryptionKey` is single-process per data dir; the divergence path is closed for the single-process case, but distributed key agreement is a separate design effort.
- **No schema migrations** required (only an in-memory struct field added).
- **HMAC `|` separator, test encryption setup, pre-compiled regex, tenant-ID redaction:** verified correct in prior reviews; no changes made.

---

## Adversarial Code Review Prompt (for Code 5)

```
You are a senior application security engineer performing a thorough adversarial
code review of the security hardening implementation described below (Code 4).
Your job is to find every vulnerability, logic error, and security anti-pattern.
Be aggressive — assume the developer made mistakes. Focus on exploitable issues,
not theoretical ones.

## Scope

Review these changes for security flaws:

### H1 + M1 — ReEncryptOnStartup error handling & context (service.go)
- Signature: ReEncryptOnStartup(ctx) (succeeded, failed int, err error)
- All 4 previously-ignored DB calls now checked/logged/failed++
- ctx.Err() checked at top of each of the 3 tenant loops (API keys, bridge keys,
  transcript signing keys); returns (succeeded, failed, ctx.Err()) on cancel
- Question: Can a ctx-cancelled mid-loop leave some tenants re-encrypted and
  others not, with the caller treating a returned err as "abort" while secrets
  are already mutated? Is the returned (succeeded, failed) tuple trustworthy
  after a cancel? Does the caller (rotateKeysCmd) roll back, or leave a
  half-rotated state?

### M7 — getOrCreateEncryptionKey (bin/memos/main.go)
- Empty/short file is removed and regenerated; O_EXCL retry loop (2 attempts)
- Losing racer adopts peer's valid key; reclaims peer's empty/short file
- Question: Is there still any path where two instances end up with DIFFERENT
  keys? What if os.Remove fails (permission)? What if the peer writes a key
  between our Remove and our O_EXCL? Is the final "last resort" branch still
  able to return a divergent local key? Does the function ever return without
  having persisted a key that matches what it returns?

### R1-2 — NewService bounded context (service.go)
- Secret load wrapped in context.WithTimeout(15s)
- Question: If the 15s secret load times out, is the error handled or does
  encryptionService stay nil and cause downstream nil-pointer / silent disable?
  Does the 15s bound interact badly with the 30s startup timeout in v1.go?

### N2 — EnsureTranscriptSigningKeys retry (service.go)
- ctx.Err() pre-check added; "may_be_committed" logged on cancel
- Question: If UpdateAgentTenant committed server-side but returned ctx.Err()
  to the client, the next startup sees the key already present (len>0) and
  skips — is that actually safe, or could it leave a tenant with a key sealed
  under the WRONG key version? Is "may_be_committed" just logging, with no
  reconciliation path?

### R1-1 — TranscriptSigningKey re-encryption + backup fallback (service.go)
- Third loop re-encrypts tenant.TranscriptSigningKey under new primary
- getTranscriptSigningSeed: try primary, then backup key from
  ENCRYPTION_MASTER_KEY_BACKUP + system secret salt
- Question: Does the backup fallback in getTranscriptSigningSeed create a
  TOCTOU or a silent downgrade? If an attacker can influence
  ENCRYPTION_MASTER_KEY_BACKUP at runtime, can they decrypt transcript seeds?
  Is the backup key's Argon2 salt derivation consistent with
  crypto.NewEncryptionService (which derives a SEPARATE backup salt via HMAC)?
  Does building a second EncryptionService here double-derive or mismatch the
  salt used during original encryption? Is there an infinite fallback loop risk?

### N1 — Injection flag moved to system prompt (store/agent.go, service.go)
- AgentSession.FlaggedInput (in-memory) replaces prepended user-turn prefix
- buildSystemPrompt / buildRAGSystemPrompt append === SECURITY GUARDRAIL ===
  to the system turn when FlaggedInput
- detectPromptInjection had 5 high-FP patterns removed
- Question: Does moving the guardrail to the system prompt actually reduce
  injection risk, or does it just shift attacker-influenced text? Can the
  guardrail itself be neutralized if the user message contains text that makes
  the model "prioritize the customer"? Are there OTHER assembly sites that
  build openrouter messages and do NOT consult FlaggedInput (e.g. lead_llm.go,
  simulation.go, playground.go)? Do the removed detection patterns open a
  bypass (e.g. "system: " is now NOT flagged — can an attacker use it)?

## Attack Vectors to Check

1. Key-rotation half-state: can a cancelled/partial rotation be exploited to
   leave some tenant secrets under the old (backup) key while the operator
   believes rotation succeeded?
2. Master-key divergence across replicas despite M7 fix — enumerate every
   remaining divergence path.
3. Backup-key runtime injection / salt mismatch in R1-1 fallback.
4. Any openrouter message-assembly path that ignores FlaggedInput, allowing the
   removed user-turn prefix protection to be silently lost.
5. Context-propagation gaps: any remaining context.Background() inside the
   changed functions, or callers that ignore the new (succeeded, failed, err).

## Output Format

For each finding, provide:
- Severity: CRITICAL / HIGH / MEDIUM / LOW / INFO
- Location: file:line
- Description: what the vulnerability is
- Exploit scenario: how an attacker would exploit it
- Fix recommendation: specific code change

Be specific. Reference exact line numbers and code snippets. Don't waste time on
theoretical issues — focus on exploitable vulnerabilities.
```

---

**End of summary.**
