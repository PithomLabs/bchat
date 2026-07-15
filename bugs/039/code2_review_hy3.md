# Code Review — `memstate` Integration Revision 2 (`code2.md`)

**Subject:** Revision 2 described in `code2.md`, verified against actual code (`server/router/api/v1/agent/service.go`).
**Verdict:** APPROVE WITH REQUIRED FIXES — prior issues addressed, but `code2.md` introduces one real regression (F1) that must be fixed before merge.

## Findings

| # | Severity | Location (code2.md / actual) | Issue | Evidence | Recommendation |
|---|----------|------|-------|----------|----------------|
| F1 | High (regression) | `code2.md:52-66` (revised `getContactState`); actual `service.go:3490`, `3542-3568`, `3571-3593` | The revised `getContactState` sets `state.Name/Phone/Address` but **omits `state.Email = info.Email`**. The current function sets it (`service.go:3490`); `buildSection0`/`buildRAGSection0` render it (lines 3556-3558, 3585-3587). Result: any email extracted by `extractCollectedInfo` silently vanishes from the "CUSTOMER INFO ALREADY PROVIDED" banner, so the agent re-asks for an email the customer already gave. | Diff of code2.md `getContactState` vs actual `service.go:3488-3490`. Banner rendering reads `state.Email`. | Add `state.Email = info.Email` (or prefer a session email field if one is ever added) inside the revised `getContactState`, before the `HasEmailOrPhone` computation. |
| H3 | Open (deferred) | `code2.md:75-77`; actual `service.go:2095, 2126-2137` | Pre-existing `session.Messages` race remains only-documented, not fixed. Concurrent same-session turns (`ClientMessageID` empty) race the unsynchronized `append` (2095) against the `extractLatest*` readers (2128-2135). `SafeMemory`'s mutex covers only `Facts`, not the session slice. | `extractLatest*` range unlocked; `append` unlocked. | Acceptable per stated scope (deferred to docs), but record as a latent bug; ideally serialize per-session or document explicitly. |
| C1 | Fixed | `code2.md:10-12, 117`; `go.mod:45,122` | Local `replace` + missing `go.sum` + `// indirect` mislabel resolved by removing `replace` and running `go mod tidy`. | Pinned pseudo-version `v0.0.0-20260714224641-ff73beb8902f` matches public repo HEAD `ff73beb8902f` (PithomLabs/memstate, Jul 14 2026) → resolves from GitHub. | None required; verify `go.sum` gains a memstate entry and the require becomes direct after tidy. |
| H2 | Fixed (modulo F1) | `code2.md:14-73`; actual `service.go:3479-3496, 2116-2137` | Contradiction confirmed real: `getContactState` uses first-match `extractCollectedInfo` (`service.go:3488`) while Section 0.5a uses newest-first `extractLatestName`. The fix (update `session.CustomerName/Phone/Location` in the memstate block + prefer them in `getContactState`) resolves it. Ordering is safe — `processChat` (sets the fields) runs before `buildSystemPrompt`. `session.CustomerName` is also set first-match at 2116-2124, but the later memstate block (no `== ""` guard) correctly overwrites to latest. | `getContactState` line 3488 (first-match); memstate block 2126-2137. | None beyond F1 (ensure `state.Email` fix doesn't regress the email path). |
| M1 | Fixed | `code2.md:79-89`; actual `service.go:3894-3897, 3920` | Weak name markers `it's`/`this is`/`it is` dropped from `latestNamePatterns[0]`. Sufficient: the existing `isCommonWord` filter (`service.go:3920`) already rejects captured "here"/"good", so no residual false positives (e.g., "I'm here" → "here" is filtered). | Pattern diff; `isCommonWord` contains "here". | None. |
| D1 | Fixed | `code2.md:91-93, 121` | `TestFactsNilByDefault` switched from struct literal to `NewMemorySessionStore` + `GetOrCreate`. Correct — `GetOrCreate` leaves `Facts` nil when `MEMSTATE_ENABLED` is unset. | `GetOrCreate` guards `Facts` on `isMemstateEnabled()` (`service.go:1196`). | Assert `Facts == nil` with the flag unset. |
| D2 | Fixed | `code2.md:95-97, 121`; actual `service.go:3901` | Added `TestExtractLatestPhoneCorrection` with "correct my number to". Supported — `latestPhoneCorrectionPatterns` already includes `(?:correct|change|update)\s+(?:my\s+)?(?:phone|number)\s+to\s+...` (`service.go:3901`). | Correction pattern present. | Assert the captured value equals the "correct my number to X" digits. |
| L1 | Fixed | `code2.md:99-101, 118` | `gofmt -w` on `service.go`. | `gofmt -l` flagged the file previously. | Run `gofmt -w`. |
| L2 | Fixed | `code2.md:103-105, 121` | `TestExtractLatestPhoneExcludesTenant` uses a 10-digit tenant phone so it actually exercises tenant exclusion. | 7-digit "555-9999" never matched the 10-digit pattern. | Use a 10-digit tenant number. |
| L3 | Fixed | `code2.md:107-109, 117` | `// indirect` annotation corrected via `go mod tidy`. | Becomes direct after tidy. | None. |

## Low Observations
- **F2 (redundancy):** The name now appears in both Section 0 (via `session.CustomerName`) and Section 0.5a (via `Facts`). Redundant but harmless; the prompt still designates 0.5a as ground truth.
- **F3 (latest-wins):** The processChat memstate block sets session fields unconditionally (no `== ""` guard) — correct for latest-wins. If `session.Facts` is nil, the first-match value from 2116-2124 persists; consistent, no contradiction.
- **F4 (test assertion):** `TestExtractLatestPhoneCorrection` should assert the captured value equals the "correct my number to X" digits exactly.

## Required Before Merge
1. **F1** — restore `state.Email = info.Email` in the revised `getContactState` (email regression).

## Confirmed Correct / No Action
- `SafeMemory` in `store` (no import cycle); mutex + `recover()` in all methods; deep-copy `Facts()`; per-session instances (no shared-map fatal write).
- Standalone name pattern #3 already excluded; Section 0.5a positioned before OM.
- Four supersession acceptance tests + nil/init/extraction tests present (11 total).
- `validatedCompanyPhone` used (not `tenant.Phone`); tenant phone excluded from captured numbers.
