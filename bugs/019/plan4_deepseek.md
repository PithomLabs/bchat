## Plan: Fix Issues from `code_review.md`

I've read all 8 files in `/home/chaschel/Documents/go/bchat/bugs/019/`. Here's the full picture and my plan for fixing the issues identified in the adversarial code review.

---

### Summary of the Code Review Findings

The `code_review.md` identifies **9 issues** across the bchat codebase, with one **CRITICAL shipping blocker**:

| Issue | Severity | Status in code_review |
|-------|----------|----------------------|
| **P0-3: Token redaction breaks frontend** | **CRITICAL** | Shipping blocker |
| **P1-8: `requireLLMConfig` not implemented** | HIGH | Not done |
| **P1-10: Error discarded in `convertWorkspaceSettingToStore`** | HIGH | Not done |
| P0-1: Deferred goroutine leaks context | MEDIUM | Bug |
| P1-9: Unparseable tokens not logged | MEDIUM | Missing log |
| P0-4: Missing error log on parse failure | MEDIUM | Missing log |
| P0-1: Error message shows first attempt error | LOW | Bug |
| P0-6: `companyName` parameter unused | LOW | Dead code |
| Migration: Non-standard base64 fallback | LOW | Unnecessary |

---

### How the Plans Evolved

The plans went through **4 iterations** of refinement:

1. **`plan_priority.md`** → Initial prioritization (Tier 1-5)
2. **`plan_deepseek.md`** → First implementation plan (P0-1 through P1-10 + INFRA)
3. **`plan2.md`** → Revised after 1st adversarial review (11 corrections)
4. **`plan3.md`** → Final after 2nd adversarial review (12 corrections)
5. **`plan_implementation.md`** → Final implementation plan (resolves 3 critical findings from plan3_review.md)

The `plan_implementation.md` is the most refined and battle-tested plan. It resolves all critical findings from `plan3_review.md` and `plan3_review_mimo.md`.

---

### My Implementation Plan

I will follow the **Sprint structure from `plan_implementation.md`** with the following modifications based on `plan3_review_mimo.md`:

#### Sprint 1: "Stop the Bleeding" (~2 days)

| # | Item | Files | Key Changes |
|---|------|-------|-------------|
| 1 | **INFRA-11** — Rotate secrets | `.env`, Fly secrets | Rotate BEFORE P0-5 |
| 2 | **INFRA-12** — `fly.toml` fixes | `fly.toml` | `min_machines=1`, `cpu_kind=performance`, `[[checks]]` |
| 3 | **P0-5 verify** — grep "usememos" | Repo-wide | Zero refs remaining |
| 4 | **P0-2** — Migration script | `scripts/migrate-old-tokens/main.go` (new) | Script parses JWT exp and removes >30d tokens |
| 5 | **P0-3 Phase 1** — Token redaction | `user_service.go` | List response has `access_token = ""`; Create response still shows token |

#### Sprint 2: "Prevent Exploitation" (~2 days)

| # | Item | Files | Key Changes |
|---|------|-------|-------------|
| 6 | **P0-1** — Session revocation on password change | `user_service.go` | Retry + honest docs + deferred recovery goroutine |
| 7 | **P0-6** — Fix XSS in `generateWidgetScript` | `handlers.go:1679-1688` | Replace raw concatenation with `json.Marshal`-safe values |
| 8 | **P0-4** — `isDomainAllowed` fail-closed | `handlers.go:1904` | Parse error → `return false` + error log |
| 9 | **P1-7** — Rate limit on missing admin endpoints | `handlers.go` (9 endpoints) | 10 endpoints now call `checkAdminMutationRateLimit` |

#### Sprint 3: "Harden Isolation" (~2 days)

| # | Item | Files | Key Changes |
|---|------|-------|-------------|
| 10 | **P1-8** — `requireLLMConfig` wrapper | `service.go` (lines 2151, 2614) | Chat returns error on broken key; non-critical sites keep soft-fallback |
| 11 | **P1-9** — Access token dedup (iat-sorted max-N=10) | `user_service.go:506-528` | Max 10 tokens per user; oldest evicted |
| 12 | **P1-10** — Error handling fix | `workspace_setting_service.go:103` | `ExtractWorkspaceSettingKeyFromName` error → InvalidArgument |
| 13 | **P0-3 Phase 2** — ID-based token deletion | `user_service.go`, `AccessTokenSection.tsx` | Frontend can delete tokens from list without raw token value |

#### Sprint 4: Removed per `plan3_review_mimo.md` (operational concerns, not security)

---

### Key Design Decisions

1. **P0-3 (Critical shipping blocker):** Two-phase approach. Phase 1 (Sprint 1) redacts tokens in List response. Phase 2 (Sprint 3) adds ID-based deletion so the frontend can delete without the raw token.

2. **P0-1 (Password change + token invalidation):** The plan is honest about the limitation — the password IS committed before token cleanup. Uses retry + deferred goroutine for transient-fault recovery. Returns 500 with clear message if permanent failure.

3. **P1-8 (Tenant API key isolation):** Adds `requireLLMConfig` wrapper that errors hard on decryption failure, applied ONLY to the 2 chat-critical endpoints. The other 7 call sites keep their existing soft-fallback to the global key.

4. **P1-9 (Token dedup):** Uses `slices.SortFunc` (already imported) to sort by JWT `iat` before evicting oldest, rather than `sort.Slice`.

5. **INFRA-13:** Dropped per user directive ("not using nginx").

### Verification

After implementation, I will run:
```bash
go test ./server/router/api/v1/... -count=1 -race
go test ./server/router/api/v1/agent/... -count=1 -race
grep -r '"usememos"' --include="*.go" .
```

---

**Ready for your review.** If this plan looks good, please toggle to ACT mode so I can begin implementing Sprint 1.
