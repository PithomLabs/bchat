# Final Adversarial Plan Review (hy3): memstate Integration — Iteration 3

**Reviewer role:** Senior security & reliability engineer
**Plan reviewed:** `bugs/039/plan3.md`
**Prior reviews:** `plan_review.md`, `plan2_review_hy3.md`, `plan2_review_deepseek.md`
**Goal of this review:** Confirm plan3 is a *minimum viable plan* the coding agent can begin from.
**Verdict:** **APPROVE WITH REQUIRED FIXES** — plan3 resolves all prior criticals, but two
HIGH correctness issues remain that would let the coding agent produce code that compiles yet
silently does not work. Fix H1, H2, and M1 before coding (or encode them as acceptance gates).

---

## Summary

plan3 correctly closes every critical from the previous round:

- **Import cycle (hy3 #CR1):** resolved by moving `SafeMemory` into the `store` package.
  Verified this is sound — session `GetOrCreate` lives in the `agent` package
  (`server/router/api/v1/agent/service.go:1158`), which already imports `store`, so
  `store.NewSafeMemory()` is callable there with no cycle.
- **Panic consistency (hy3 #M1 / DeepSeek #H1):** `recover()` moved into each `SafeMemory`
  method; standalone `safeFactsAdd` removed.
- **Dead code (DeepSeek #M2/#H2):** `SafeMemory.Facts()` removed.
- **Testable flag (DeepSeek #M1 / hy3 #L2):** `var isMemstateEnabled` seam.
- **D3 `session.Messages` claim (hy3 #M2):** corrected/scoped out.

The remaining problems are all in the **belief-revision data path** — the very thing this
iteration set out to fix (CR2). They are correctness, not compilation, issues, which makes
them dangerous: the build will pass and the feature will appear wired up while doing the wrong
thing.

### Verification basis (from source)

- Session `GetOrCreate`: `service.go:1158`; new session constructed at `service.go:1177–1188`
  (no `Facts` set) — this is the correct init point, **not** `store/bridge.go`.
- `extractCollectedInfo` (`service.go:3648`):
  - Skips non-user messages: `if msg.Role != "user" { continue }` (3687).
  - Name: filtered by `isCommonWord` and `len(name) > 2` (3699); first-match.
  - Phone pattern has **three capture groups** `([2-9]\d{2})[-.\s]?(\d{3})[-.\s]?(\d{4})`
    (3661); tenant-phone and placeholder exclusions (3713, 3726).
  - Address uses `match[0]` (full) with `len(addr) > 10` guard (3762–3763), not `match[1]`.
- OM injection ("SECTION 0.5") begins at `service.go:2666`; inserting 0.5a before it (after
  2664) is correct.
- memstate (`github.com/PithomLabs/memstate@main`): `topicSim` is **IDF-weighted** overlap
  with a `min(denL, denR)` denominator (memory.go); default `SupersedeThreshold = 0.55`
  (options.go); `New(cfg ...Config)` accepts a `Config`.

---

## Findings

### HIGH — must resolve before coding

#### H1. `extractLatestField` discards the safeguards that make extraction correct
- **Severity:** High
- **Regression or new:** New (introduced by the CR2 fix in plan3).
- **Failure scenario:** The helper (plan3 lines 17–27) takes `[]string`, ignores role, and
  returns raw `m[1]`. Compared to `extractCollectedInfo` it loses four safeguards:
  1. **No user-role filter.** `extractCollectedInfo` skips non-user messages
     (`service.go:3687`). The helper scans everything, so an assistant turn that echoes
     customer details ("So your name is John, correct?") is captured as a customer fact.
  2. **Phone returns only the area code.** `phonePattern` has three groups
     (`service.go:3661`), so `m[1]` is the 3-digit area code, not the full number — memstate
     receives `"Customer phone is 555"`. It also drops the tenant-phone/placeholder
     exclusions (3713, 3726), so the tenant's own number can be captured.
  3. **Address is truncated/unguarded.** The real code uses `match[0]` with a `len > 10`
     guard (3762–3763); `m[1]` is just the street token group, and there is no length check.
  4. **Name false positives.** No `isCommonWord`/`len > 2` filter (3699), so filler like
     "ok thanks" can become a name.
- **Recommended fix:** Do not use a generic single-group scanner. Either (a) refactor
  `extractCollectedInfo` to expose a "latest value per field" mode that reuses its existing
  per-field logic (filters, exclusions, `match[0]` for address, role filter), or (b) write
  per-field latest-extractors that mirror those safeguards exactly. Keep the user-role filter.
- **Disposition:** In this plan (blocking for MVP).

#### H2. Supersession is unvalidated and the acceptance table uses the wrong scoring model
- **Severity:** High
- **Regression or new:** Carried-forward risk (prior H4), now with mis-specified test
  expectations.
- **Failure scenario:** The test table (plan3 lines 104–109) predicts outcomes from raw token
  counts ("3/4 tokens shared → overlap > 0.55"). memstate does **not** score that way:
  `topicSim` weights shared tokens by IDF and divides by `min(denL, denR)` (memory.go). The
  distinctive tokens (`john`/`jonathan`, `rome`/`milan`) carry high IDF and inflate the
  denominator, so "Customer name is John" → "…Jonathan" may score **below** the default 0.55
  and **fail to supersede**. If it fails, both facts stay current, the prompt shows both
  values, and the original bug persists — the feature silently does nothing despite the H1/CR2
  extractor fix.
- **Additional gap:** `NewSafeMemory()` calls `memstate.New()` with no `Config`, so
  `SupersedeThreshold` cannot be tuned. There is no lever to make the tests pass if the
  default misfires.
- **Recommended fix:**
  1. Give `NewSafeMemory` a `memstate.Config` (tunable `SupersedeThreshold`; consider
     `Semantic` embedding for topic matching).
  2. Rewrite the test table as **measured acceptance gates**: the tests must print the actual
     `topicSim`/supersession result and the threshold must be tuned until John→Jonathan and
     Rome→Milan supersede while cross-topic pairs do not. Do not assume default 0.55 works.
  3. Make "these four cases pass with the chosen threshold" a hard pre-`MEMSTATE_ENABLED=true`
     gate.
- **Disposition:** In this plan (blocking for MVP — this is the feature's core promise).

### MEDIUM

#### M1. Step 4 points at the wrong file
- **Severity:** Medium
- Step 4 says initialize `SafeMemory` in `store/bridge.go (GetOrCreate)`. Session
  `GetOrCreate` is `MemorySessionStore.GetOrCreate` in
  `server/router/api/v1/agent/service.go:1158`; the new session is built at 1177–1188.
  `store/bridge.go` exists but is unrelated to session creation. (The cycle fix still holds:
  the `agent` package can call `store.NewSafeMemory()`.)
- **Fix:** Correct the file/location to `service.go` and gate init on `isMemstateEnabled()`.
- **Disposition:** In this plan.

#### M2. Contradiction on the duplicate-fact guard
- **Severity:** Medium
- Line 46 says "remove the duplicate-fact guard ('only add if changed')", but the call sites
  (lines 32–40) keep `latestName != session.CustomerName`. Moreover `session.CustomerName` is
  the *first-match* value (set only-if-empty at `service.go:2106`), so the guard compares
  latest-vs-first and re-adds the fact every turn after a change (memstate dedups via
  supersession, so it is idempotent but churns history).
- **Fix:** Choose one design and make prose and code agree — either drop the guard entirely
  and rely on memstate idempotency, or compare against the last value actually added to
  memstate (not `session.CustomerName`).
- **Disposition:** In this plan.

#### M3. `msgs` type/role mismatch is unspecified
- **Severity:** Medium
- Call sites pass `msgs []string`, but `session.Messages` is `[]store.AgentMessage`. The
  conversion and the user-role filter (see H1) must be spelled out so the coding agent does
  not improvise.
- **Fix:** Specify the adapter (extract `Content` from user-role messages) or pass
  `[]store.AgentMessage` into the extractor directly.
- **Disposition:** In this plan.

#### M4. Pattern duplication will drift
- **Severity:** Medium
- Duplicating `namePatterns`/`phonePatterns`/`addressPatterns` from `extractCollectedInfo`
  (plan3 line 44) creates two sources of truth; a future change to one silently diverges.
  Sharing/hoisting the existing patterns also fixes H1's multi-group problem in one place.
- **Fix:** Hoist the existing patterns to package-level and reuse them in both functions.
- **Disposition:** In this plan (folds into H1).

### LOW

#### L1. Per-session facts-history growth
- Re-adding the same fact each turn creates superseded history entries (bounded by turn
  count; only current facts are shown by `Prompt`). Acceptable at session scope; note it.

#### L2. `NewSafeMemory` signature undecided
- Tied to H2: decide whether it takes a `memstate.Config`. If threshold tuning is needed, the
  constructor must accept it.

---

## Status of prior-review issues

| Issue | Source | Status in plan3 |
|-------|--------|-----------------|
| Import cycle (`store → agent → store`) | hy3 #CR1 | Resolved (SafeMemory in `store`) — verified no cycle |
| First-match extractor breaks revision | hy3 #CR2 / DeepSeek #C1 | Addressed in intent; **new H1/H2 gaps remain** in the fix |
| Inconsistent panic protection | hy3 #M1 / DeepSeek #H1 | Resolved (recover in each method) |
| Dead `Facts()` method | DeepSeek #M2/#H2 | Resolved (removed) |
| `isMemstateEnabled` testability | DeepSeek #M1 / hy3 #L2 | Resolved (`var` seam) |
| D3 `session.Messages` claim | hy3 #M2 | Resolved (scoped out) |
| Supersession threshold unvalidated | prior #H4 / DeepSeek #M3 | Partially — test cases added but **mis-modeled** (H2) |
| `safeFactsAdd` nested func | hy3 #L1 | Resolved (folded into wrapper) |

---

## MVP-readiness gate — required before the coding agent starts

1. **H1:** Replace the naive `extractLatestField` with per-field latest extraction that
   preserves role filtering, name false-positive filtering, full-number phone capture +
   tenant/placeholder exclusion, and full-address (`match[0]` + length guard). Prefer
   refactoring/sharing `extractCollectedInfo` over duplicating patterns (also closes M4).
2. **H2:** Make `SafeMemory`/`NewSafeMemory` accept a `memstate.Config`; convert the
   supersession table into measured tests that print actual scores; tune `SupersedeThreshold`
   until John→Jonathan and Rome→Milan supersede and cross-topic pairs do not. Gate
   `MEMSTATE_ENABLED=true` on these passing.
3. **M1:** Fix the Step 4 file/location to `service.go:1177–1188`.
4. **M2/M3:** Resolve the dedup-guard contradiction and specify the `msgs` conversion + role
   filter.

With H1, H2, and M1 addressed (and M2/M3 clarified), plan3 becomes a sound minimum viable
plan. The architecture, packaging, safety wrapper, default-off rollout, and prompt wording are
all correct; the remaining work is making the extraction faithful and proving supersession
actually fires with the real (IDF-based) scoring.
