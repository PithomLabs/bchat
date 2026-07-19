# Plan Review — Code 6 (implement F1–F6 with code5 rework applied)

**Date:** 2026-07-20
**Reviewer:** Senior Go Architect / Application Security Engineer (bchat)
**Plan under review:** `bugs/043/code6_plan.md`
**Upstream:** `bugs/043/code4_summary_review.md` (F1–F6) + `bugs/043/code5_plan_review.md` (rework/nits)

**Verdict: APPROVE WITH REWORK + NITS**

The plan correctly incorporates both rework items from `code5_plan_review.md` (F2 false-resume premise, F1 self-contradiction) and addresses all six findings. I re-verified every technical premise against the CURRENT source. Two rework items remain that will cause either a compile break or a silently-ignored partial rotation; both are small but must be fixed before implementation.

---

## Verification of premises (against current source)

- **F1** — `getOrCreateEncryptionKey` at `bin/memos/main.go:323-379` still has the top-of-function `os.Remove` (line 333) and the O_EXCL-failure `os.Remove` (line 354). Plan's resolution (keep 333, drop 354, last-resort fatal) is correct. ✅
- **F2** — Three decrypt sites confirmed: API-key (`service.go:278`), bridge-key (`service.go:316`), transcript (`service.go:362`). All decrypt **only** with `backupSvc`; no primary fallback. The plan's backup→primary fallback snippet is correct and makes rotation idempotent. ✅ (but see Rework 2 on the caller/return contract)
- **F3** — `getTranscriptSigningSeed` (`service.go:1914-1944`) reads `os.Getenv("ENCRYPTION_MASTER_KEY_BACKUP")` at line 1933; `encryptionService == nil` early-return at 1922. Plan's `Service`-field `backupKeyActive` is correct. ✅ (nit on placement below)
- **F4** — `ExtractContactInfoFull` has exactly **2 production callers** (`service.go:4423`, `service.go:5104`, both with `session` in scope) and **3 test callers** (`lead_extraction_test.go:246, 261, 273`). ⚠️ Plan says "both call sites" (246, 273) — **misses line 261** (`TestExtractContactInfoFull_Declined`). See Rework 1.
- **F5** — CURRENT `detectPromptInjection` (`service.go:2344-2376`) does **NOT** contain `"you are a"`, `"system: "`, `"assistant: "`, `"human: "`, `"### system:"`. So Round 4's removal WAS implemented (my earlier cached read was from a stale line range — the function moved). F5 premise is VALID; re-adding `"system:"`, `"<|im_start"`, `"<|im_end"`, `"### system"` is correct and safe. ✅
- **F6** — SQLite `GetSystemSecret` returns `(nil, nil)` on not-found (`store/db/sqlite/rbac.go:534-535`). Postgres `GetSystemSecret` returns `(nil, nil)` (`store/db/postgres/rbac.go:213-215`). `secret == nil && err == nil` is a reliable first-run sentinel for both drivers. ✅

---

## Rework 1 — F4: test call-site count is wrong (compile break)

**Plan (code6_plan.md:74, 80):** lists `lead_extraction_test.go` call sites as "~246, ~273" and says "Update the two `service.go` callers" plus "both call sites" in tests.

**Reality:** `ExtractContactInfoFull` is called at **three** test sites, not two:
- `lead_extraction_test.go:246` (`TestExtractContactInfoFull`)
- `lead_extraction_test.go:261` (`TestExtractContactInfoFull_Declined`)
- `lead_extraction_test.go:273` (`TestExtractContactInfoFull_Correction`)

Adding a `flagged bool` parameter to `ExtractContactInfoFull` while updating only 246 and 273 will leave line 261 calling it with the old arity → **compile error** in the test package.

**Fix:** Update all three test call sites (246, 261, 273) to pass `false`. The plan's Task 5 verification (code6_plan.md:112) should also assert `ExtractContactInfoFull(..., false)` still compiles/passes.

---

## Rework 2 — F2: partial rotation (`failed > 0`, no ctx cancel) exits 0; remediation never fires

**Plan (code6_plan.md:60-63):** "On `failed > 0` → return `fmt.Errorf("key rotation partially failed: ...")`."

**Problem:** The current `ReEncryptOnStartup` returns `(succeeded, failed, nil)` at the end (`service.go:384-385`) — `err` is non-nil ONLY on `ctx.Err()`. A partial rotation with `failed > 0` but NO context cancellation returns `nil` error. `rotateKeysCmd` does `if err := svc.ReEncryptOnStartup(ctx); err != nil { return err }` (main.go:175), so it will **exit 0** on a partial rotation, and the carefully-worded remediation error (code6_plan.md:62) is never constructed or returned.

The plan text "On `failed > 0` → return fmt.Errorf(...)" is ambiguous about WHERE this error is produced. As written, neither `ReEncryptOnStartup` nor `rotateKeysCmd` actually returns it for the `failed > 0 && err == nil` case.

**Fix (choose one, state it explicitly in the plan):**
- **Option A (preferred):** `ReEncryptOnStartup` returns a non-nil error when `failed > 0`, e.g. at the end:
  ```go
  if failed > 0 {
      return succeeded, failed, fmt.Errorf("key rotation partially failed: %d of %d secrets not re-encrypted; tenants still under the backup key remain — do NOT remove ENCRYPTION_MASTER_KEY_BACKUP until a clean re-run reports 0 failures", failed, succeeded+failed)
  }
  return succeeded, failed, nil
  ```
  (Keep `ctx.Err()` returns as-is for the cancel case.)
- **Option B:** `rotateKeysCmd` inspects the returned `failed` count: `if _, failed, rerr := svc.ReEncryptOnStartup(ctx); rerr != nil || failed > 0 { ... }`.

Either way, the plan must specify that `failed > 0` yields a non-zero process exit, otherwise the entire F2 caller-guardrail value is lost.

---

## Nits

- **F3 (nit):** Place `svc.backupKeyActive = os.Getenv("ENCRYPTION_MASTER_KEY_BACKUP") != ""` **inside** the `if p.EncryptionMasterKey != ""` block in `NewService` (service.go:92-111), not after it unconditionally. Today `encryptionService` is only built when `EncryptionMasterKey != ""`, and `getTranscriptSigningSeed` early-returns at line 1922 (`encryptionService == nil`) before consulting the flag — so there is no crash — but an unconditional flag would be misleading (true while `encryptionService` is nil). Keeping them in the same conditional block makes the flag semantically consistent with "a backup key actually exists AND we have a primary service to use it with."
- **F1 (nit, already good):** The plan correctly retains the top `os.Remove` (we own that file) and drops only the O_EXCL-failure `os.Remove`. The "remove-failure → log.Fatal" guard (code6_plan.md:22) is the right call and prevents the boot-loop I flagged in code5. No change needed.
- **F2/F3 (nit):** The single consolidated overlap log (code6_plan.md:64) emitted once in `NewService` when `backupKey != ""` is correct and resolves the duplicate-log nit from code5. Good.
- **Verification F2 resume (code6_plan.md:113):** The idempotency test ("second call completes remaining tenants to 0 failed") is exactly right and directly exercises the backup→primary fallback. Keep it. Consider also asserting that an ALREADY-rotated tenant (primary-sealed) is reprocessed to 0 `failed` on re-run (the core of the original false-premise bug).

---

## Summary

| Item | Finding | Action |
|------|---------|--------|
| F4 | Test call sites: 3, not 2 (line 261 missed) | **REWORK** — update all 3 test call sites |
| F2 | `failed > 0` returns `nil` err → `rotateKeysCmd` exits 0; remediation never fires | **REWORK** — make `failed > 0` produce a non-nil error (Option A preferred) |
| F3 | `backupKeyActive` placement | **NIT** — set inside `EncryptionMasterKey != ""` block |
| F1/F2/F5/F6 | Premises verified against current source | ✅ Correct as written |

**Bottom line:** The plan is fundamentally sound and correctly applies the code5 rework. It is NOT implementation-ready as written — Rework 1 will break the test build and Rework 2 will silently defeat the F2 caller guardrail. Fix those two (the nits are optional polish) and the plan is ready.
