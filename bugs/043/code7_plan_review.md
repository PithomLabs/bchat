# Plan Review — Code 7 (final; F1–F6 with all rework applied)

**Date:** 2026-07-20
**Reviewer:** Senior Go Architect / Application Security Engineer (bchat)
**Plan under review:** `bugs/043/code7_plan.md`
**Upstream:** `code4_summary_review.md` (F1–F6) + `code5_plan_review.md` + `code6_plan_review.md`

**Verdict: APPROVE (minimum viable) — with NON-BLOCKING nits**

This is the final review. The plan correctly resolves every rework item from `code6_plan_review.md` and the F3 placement nit, and all six findings (F1–F6) are addressed with premises re-verified against current source. The two items that previously blocked approval are fixed. Remaining notes are minor documentation/placement nits that an implementer can resolve without further review.

---

## Rework from code6 — CONFIRMED RESOLVED

- **F4 (was Rework 1 — compile break):** Plan line 84 now says "Update **ALL 3 test** call sites (246, 261, 273)". The arity regression that would have broken the test build is closed. ✅
- **F2 (was Rework 2 — partial rotation exits 0):** Plan lines 58-65 replace the `return succeeded, failed, nil` (service.go:384-385) with an explicit `if failed > 0 { return ..., fmt.Errorf(...) }`. This is Option A from code6 — `failed > 0` now yields a non-nil error, so `rotateKeysCmd` (`main.go:175) exits non-zero and the remediation text fires. ✅
- **F3 (nit):** `backupKeyActive` is set **inside** the `if p.EncryptionMasterKey != ""` block (line 74), consistent with `encryptionService != nil`. ✅

## Premises re-verified (current source)
- **F1** — top `os.Remove` (333, keep), O_EXCL-failure `os.Remove` (354, DROP), write-error partial remove (360, keep), last-resort `return key` → FATAL. ✅
- **F2** — three decrypt sites (API :278, bridge :316, transcript :362) get backup→primary fallback. ✅
- **F5** — current `detectPromptInjection` (service.go:2344) lacks the removed high-FP patterns; re-adding `"system:"`, `"<|im_start"`, `"<|im_end"`, `"### system"` is correct. ✅
- **F6** — SQLite + Postgres `GetSystemSecret` both return `(nil, nil)` on not-found; first-run sentinel reliable. ✅

---

## NON-BLOCKING nits (fix during implementation, no re-review needed)

1. **F1 line numbers are off by a few.** The plan calls line 362 the "last-resort `return key`" to make FATAL, but in the current source the last-resort `return key` is at **line 378** (line 362 is the *write-error* `return key`, which the plan correctly keeps as best-effort). The INTENT ("make the final fallback FATAL, never return a divergent key") is unambiguous — implementers should target the last-resort branch that currently ends `return key` after the `slog.Error(...)` at ~377. No behavior change needed; just don't be misled by the number.

2. **Overlap log placement (F3/Task 3 line 68).** The single consolidated overlap log should be emitted **inside** the `if p.EncryptionMasterKey != ""` block (where `backupKeyActive` is set), i.e. only when both `EncryptionMasterKey != ""` AND `backupKey != ""`. The plan text says "when `backupKey != ""`" which is ambiguous; guard it with the same `EncryptionMasterKey != ""` condition so we never emit an overlap log (or set the flag) when there is no usable `encryptionService`. This keeps F3's flag and the log perfectly consistent.

3. **F4 guardrail helper extraction.** `appendInjectionGuardrail(sb *strings.Builder)` must reproduce the EXACT text currently emitted by `buildSystemPrompt` (service.go:~3263) and `buildRAGSystemPrompt` (service.go:~3762). Confirm the two existing blocks are byte-identical before extracting; if they differ, unify them in the helper. Also ensure `ExtractContactInfoLLM`'s `systemPrompt` is a mutable `string` variable (not a `const`) so the guardrail can be appended.

---

## Summary

| Item | Status |
|------|--------|
| F4 test call-site count (code6 Rework 1) | ✅ RESOLVED — all 3 sites updated |
| F2 `failed>0` returns nil (code6 Rework 2) | ✅ RESOLVED — non-nil error on `failed>0` |
| F3 `backupKeyActive` placement | ✅ RESOLVED — inside `EncryptionMasterKey != ""` block |
| F1 / F2 / F5 / F6 premises | ✅ Verified against current source |
| Nit 1 (F1 line numbers) | Non-blocking doc fix |
| Nit 2 (overlap log guard) | Non-blocking placement fix |
| Nit 3 (guardrail helper unify) | Non-blocking implementation detail |

**Bottom line:** This is a minimum-viable, implementation-ready plan. All blocking rework from code6 is incorporated; the three nits are cosmetic/placement and do not change security behavior. Approve and proceed to implementation.
