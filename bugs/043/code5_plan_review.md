# Plan Review — Code 5 (Security Hardening Round 4 Review implementation)

**Date:** 2026-07-20
**Reviewer:** Senior Go Architect / Application Security Engineer (bchat)
**Plan under review:** `bugs/043/code5.md`
**Upstream review:** `bugs/043/code4_summary_review.md` (F1–F6)

**Verdict: APPROVE WITH REWORK + NITS**

The plan correctly targets all six valid findings from `code4_summary_review.md`, and most proposed fixes are sound and verifiable against live source. However, **one finding (F2) is built on a false premise** that makes its resume/idempotency claim incorrect, and **one finding (F1) contains an internal contradiction** that will produce an unbootable or diverging key path if implemented literally. Both must be resolved before implementation. Several smaller nits are listed at the end.

All statements below are backed by direct source reads cited by file:line.

---

## 1. F2 (MEDIUM) — Resume/idempotency claim is INCORRECT (rework required)

**Plan claim (code5.md:50):** *"Confirm re-run is safe: yes, because backup key can still decrypt the rotated ciphertext."*

**This is false.** In `ReEncryptOnStartup` the decryption side always uses `backupSvc` (built from `ENCRYPTION_MASTER_KEY_BACKUP`), not the primary:

- `server/router/api/v1/agent/service.go:278` — `plaintext, dErr := backupSvc.Decrypt(config.OpenRouterAPIKeyEncrypted, config.OpenRouterAPIKeyNonce)`
- On failure: `slog.Error(...); failed++; continue` (lines 279-283) — it does **not** fall back to `s.encryptionService.Decrypt`.

After a *partial* rotation, already-rotated tenants are sealed under the **primary** key. On a re-run, `backupSvc.Decrypt` will FAIL for those tenants → they are counted as `failed` and skipped, never re-processed. The plan's central justification for "re-run is safe" does not hold. The command will exit non-zero on every re-run until the operator manually fixes the already-rotated rows, and the remediation text ("re-run reports 0 failures") is unreachable for exactly the tenants that were successfully rotated first time.

**Required rework:** Specify that the three decrypt sites must attempt `backupSvc.Decrypt` first, then on failure attempt `s.encryptionService.Decrypt` (the already-rotated case), before counting `failed`. Concretely:

```go
plaintext, dErr := backupSvc.Decrypt(ct, cn)
if dErr != nil {
    // Already re-encrypted under the primary key on a prior (partial) run.
    plaintext, dErr = s.encryptionService.Decrypt(ct, cn)
    if dErr != nil {
        slog.Error("failed to decrypt tenant secret (neither backup nor primary)", ...)
        failed++
        continue
    }
}
```

This makes `rotate-keys` genuinely idempotent/resumable, which is the entire value of the F2 scope. Without it, F2's "resume" is an illusion.

**Secondary:** The plan scopes F2 to "caller guardrail + resume + remediation msg" and explicitly defers per-tenant transactional rollback (code5.md:48,101). That scoping is reasonable, but the remediation message must be explicit that a non-zero `failed` after a *clean* re-run means "some tenants are sealed under a key the backup key cannot open" — i.e., the opposite of the plan's stated assumption. Fix the wording so operators are not misled.

---

## 2. F1 (HIGH) — Internal contradiction on the early `os.Remove` (rework/ambiguity)

**Plan contradiction (code5.md:30-31):**
- Line 30: *"Keep the top-of-function `os.Remove` for the locally-discovered empty/short file (line 333) — that one we 'own'."*
- Line 31: *"Simplest: drop the early remove; instead attempt O_EXCL and only on success overwrite."*

These are mutually exclusive. If the early remove (line 333) is dropped, then for a locally-discovered empty/short file the subsequent `O_EXCL` **always fails** (the file still exists), the O_EXCL-failure branch now only adopts-or-retries, and after the 2-attempt loop the last-resort path must no longer `return key` (the plan correctly says replace it with panic). But nothing ever writes the key, so the process **panics on every boot** whenever the file is empty/short. That defeats the "heal corrupt/empty key file" goal from Round 4 (M7).

**Required clarification:** Keep the *local* remove (line 333) because we own that file (we just read it and decided to regenerate). Remove **only** the `os.Remove` inside the O_EXCL-failure branch (line 354), which is the one that can unlink a *peer's* in-flight file. The plan's Task 1 prose should be rewritten to state this unambiguously:

- Top-of-function: if file exists and `len(trimmed) < 16` → `os.Remove` (we own it) → fall through to the O_EXCL write loop.
- O_EXCL-failure branch: read peer file; if valid → `return k`; if still empty/short → `continue` (do NOT remove). The peer will finish writing; a later `ReadFile` adopts it.
- Last-resort (after retries): if still no valid key → `log.Fatal` / `panic`, never `return key`.

This preserves the Round 4 healing behavior while closing the divergence path. Implementation will otherwise be ambiguous and risk either divergence or a boot-loop.

**Added safety (nit, recommended):** Because `os.Remove` can still fail (permissions), the local-remove path should treat a remove failure as fatal rather than silently proceeding to an O_EXCL that will then fail and panic anyway — better to fail with a clear "cannot clear corrupt key file" message.

---

## 3. F3 (MEDIUM) — Flag must be a `Service` field, not package-level (nit)

**Plan (code5.md:56):** *"Compute a package-level (or Service-field) `rotationOverlapActive bool`"*

A **package-level** variable is unsafe: `NewService` is called for the main service *and* for `agentServiceForRotation` (bin/memos/main.go:182-194), and could be called with different master/backup keys in tests or future multi-tenant tooling. A shared global would let one `Service`'s backup-key state leak into another's `getTranscriptSigningSeed`. Use a `Service`-scoped field (e.g., `svc.backupKeyActive`) set in `NewService` when `EncryptionMasterKey != "" && backupKey != ""`. This also removes any concurrency concern.

Otherwise F3 is sound: replacing the per-request `os.Getenv("ENCRYPTION_MASTER_KEY_BACKUP")` (service.go:1933) with the startup-scoped flag removes the TOCTOU/trust-surface and the silent WARN→INFO change is appropriate.

---

## 4. F6 (LOW) — Sentinel approach is correct; verify store semantics (verified)

**Plan (code5.md:40-43):** generate salt only on first-run (`secret == nil && err == nil`), else fatal.

Verified against `store/db/sqlite/rbac.go:534-535`: `GetSystemSecret` returns `(nil, nil)` on `sql.ErrNoRows`. So `secret == nil && err == nil` is a reliable first-run sentinel across SQLite. (Postgres impl at `store/db/postgres/rbac.go:213+` should be checked for the same `(nil,nil)` contract during implementation, but the SQLite path is the dev/default and is correct.) The fix correctly stops the 15s-timeout silent salt regeneration. **Approved as written**, with the note to confirm the Postgres driver returns `(nil,nil)` on not-found too.

---

## 5. F4 (LOW) — Call sites have `session` in scope (verified, approved with nit)

Verified:
- `service.go:4423` is inside `refreshLeadFromSession(ctx, config, session *store.AgentSession)` — `session` is a parameter.
- `service.go:5104` is inside `captureLeadFromSession(ctx, config, session *store.AgentSession)` — `session` is a parameter.

So threading `session.FlaggedInput` into `ExtractContactInfoFull` is straightforward. **Approved.**

**Nit:** The plan threads a `flagged bool` param (code5.md:63). Prefer passing the whole `session` or `session.FlaggedInput` explicitly; ensure `lead_extraction_test.go` call sites (the two `ExtractContactInfoFull(ctx, "", messages, "", nil)` at lines 246/273) are updated to pass `false`. Also extract `appendInjectionGuardrail(sb)` from `buildSystemPrompt`/`buildRAGSystemPrompt` (service.go:3263, 3762) to avoid duplicating the guardrail text — the plan mentions this (code5.md:64) and it is the right call.

---

## 6. F5 (LOW) — Re-add delimiter patterns (approved with nit)

**Plan (code5.md:71-77):** re-add `"system:"`, `"<|im_start"`, `"<|im_end"`, `"### system"`; keep existing `"<|im_start|>system"`, `"[inst]"`, `"<<sys>>"`, `"```\nsystem"`.

Verified the existing list at `service.go:2245-2273` already contains `"<|im_start|>system"`, `"[inst]"`, `"<<sys>>"`, `"```\nsystem"`. Adding the shorter delimiters is a strict improvement and addresses the F5 bypass (`"system: ignore previous instructions"` currently passes undetected because `"system: "` was removed). **Approved.**

**Nit:** Specify the exact strings to add to avoid ambiguity, and note they are `strings.Contains` (no word-boundary). Recommend:
- `"system:"` (catches `system: ` and `system:` delimiters; also matches existing `"system prompt:"` substring — acceptable, more precise)
- `"<|im_start"` and `"<|im_end"` (prefix match; catches the existing `<|im_start|>system` too)
- `"### system"` (catches `### system:` as well)

Do **not** re-add `"you are a"`, `"assistant: "`, `"human: "` (high FP, per plan). Good.

---

## 7. Cross-cutting nits

- **Duplicated overlap log:** The "key-rotation overlap window active" log is described in both Task 3 (code5.md:51) and Task 4 (code5.md:57). Consolidate into a single startup log emitted once in `NewService` (or `ReEncryptOnStartup` only when `backupKey != ""`), so we don't emit it from two places with divergent wording.
- **Verification step F1 (code5.md:95):** The proposed unit test ("two concurrent processes on the same dir → assert both end up with the same key") is the right test but is hard to express as a pure unit test because `getOrCreateEncryptionKey` uses `os` directly with no injection point. Recommend a test that (a) pre-creates an empty/short file and asserts a single run heals it to one stable key, and (b) simulates the O_EXCL race by spawning two goroutines that both call the function on a temp dir and asserting equality of returned keys + on-disk content. If direct subprocess simulation is needed, a small helper that exercises the adopt-path is acceptable.
- **No schema migration needed** — confirmed; F2/F3/F4/F5/F6 are code-only. Good.

---

## Summary

| Finding | Plan status | Action |
|---------|-------------|--------|
| F2 | Incorrect resume premise | **REWORK** — add primary-key fallback decrypt on backup-decrypt failure |
| F1 | Internal contradiction on early `os.Remove` | **REWORK** — keep local remove, drop only O_EXCL-failure-branch remove; clarify |
| F3 | Package-level flag risk | **NIT** — use `Service` field |
| F6 | Sentinel approach | Verified correct (check Postgres not-found contract) |
| F4 | Call-site `session` scope | Verified; **NIT** — update test call sites, extract guardrail helper |
| F5 | Re-add delimiters | **NIT** — specify exact strings |
| Cross-cutting | Dup log / test design | **NIT** |

**Bottom line:** The plan is on the right track and addresses all six findings, but must not be implemented as written — F2's idempotency is broken and F1 is self-contradictory. Resolve the two rework items (and ideally the F3 field nit) and the plan is implementation-ready.
