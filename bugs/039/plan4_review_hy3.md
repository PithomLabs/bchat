# Final Adversarial Plan Review (hy3): memstate Integration — Iteration 4

**Reviewer role:** Senior security & reliability engineer
**Plan reviewed:** `bugs/039/plan4.md`
**Prior reviews:** `plan_review.md`, `plan2_review_hy3.md`, `plan3_review_hy3.md`,
`plan2_review_deepseek.md`, `plan3_review_deepseek.md`
**Goal:** Confirm plan4 is a minimum viable plan the coding agent can implement without a
further review round.
**Verdict:** **APPROVED WITH NITS — READY FOR IMPLEMENTATION.** All prior criticals and highs
are resolved. One Medium correctness issue and a handful of nits remain; all are
implementation-time corrections, not blockers, and do not require another review cycle.

---

## Summary

plan4 faithfully closes every HIGH and MEDIUM from the plan3 reviews:

- **HIGH #1 (faithful extraction):** replaced the naive `extractLatestField` with three
  per-field extractors that scan **user-role, newest-first** and mirror
  `extractCollectedInfo`'s safeguards — name false-positive filter (`isCommonWord`, `len>2`),
  phone via `m[0]` (full number, not the area-code group) with tenant/placeholder exclusion
  and correction-pattern precedence, address via `m[0]` with the `len>10` guard. Patterns are
  hoisted to package level (single source of truth), and the extractors take
  `[]store.AgentMessage` directly (resolves the `msgs` type mismatch and duplication drift).
- **HIGH #2 (supersession):** `NewSafeMemory(cfg ...memstate.Config)` makes the threshold
  tunable, and the raw-token-count test predictions are replaced with measured acceptance
  tests gated before `MEMSTATE_ENABLED=true`.
- **MEDIUM #1:** Step 4 corrected to `service.go` (MemorySessionStore.GetOrCreate,
  lines 1177–1188).
- **MEDIUM #2:** duplicate-fact guard removed entirely (prose and code now agree).
- **MEDIUM #3/#4:** resolved by the HIGH #1 refactor.

The architecture (session-scoped facts, `store`-package `SafeMemory` wrapper with mutex +
recover, default-off flag, Section 0.5a before OM) is sound and unchanged.

### Verification basis (from source)

- `isCommonWord` (`service.go:3830–3851`) filters only a fixed list and returns true if *any*
  word matches; phrases like "sounds good" / "will do" are **not** filtered.
- Name pattern #3 (`service.go:3657`) `^([a-z]{2,}(?:\s+[a-z]{2,})?)$` matches any 1–2 word
  lowercase message.
- `processChat` (`service.go:2087`) computes `validatedCompanyPhone` at 2104 and calls
  `extractCollectedInfo` at 2105; there is no `tenant` variable in scope.
- Session construction: `service.go:1177–1188` (package `agent`, imports `store`) — can call
  `store.NewSafeMemory()` with no cycle.
- OM injection ("SECTION 0.5") begins at `service.go:2666`.
- memstate `topicSim` is IDF-weighted with a `min(denL, denR)` denominator; `Facts()` exposes
  `Current()` state but no raw similarity score.

---

## Findings

### MEDIUM — must fix during implementation (no re-review required)

#### M1. Newest-first + standalone-name pattern can overwrite a correct name
- **Severity:** Medium (correctness regression risk)
- **Regression or new:** New — a side effect of switching to newest-first extraction.
- **Failure scenario:** `extractLatestName` scans newest-first and accepts name pattern #3
  (`service.go:3657`), which matches any 1–2 word lowercase message. `isCommonWord`
  (`service.go:3830`) does not filter phrases like "sounds good", "will do", "talk soon".
  Sequence: turn 1 "my name is John" → name "John"; turn 6 user types "sounds good" →
  `extractLatestName` returns "sounds good" → `Add("Customer name is sounds good")` supersedes
  the correct fact. In the original code this could not happen because name was first-match
  (the greeting won and later messages were ignored); newest-first reverses that protection.
- **Recommended fix:** For the latest-name extractor, drop standalone pattern #3, **or** use
  it only as a fallback when no explicit-marker match ("my name is / I'm / call me / X here /
  X speaking") exists anywhere in history. The explicit patterns are safe under newest-first.
- **Disposition:** Fix inline while implementing HIGH #1.

### NITS — fix inline

#### N1. Wrong variable at the call site
- Plan4 line 94 passes `tenant.Phone`, but `processChat` has no `tenant` in scope. Use the
  existing `validatedCompanyPhone` computed at `service.go:2104` when calling
  `extractLatestPhone(session.Messages, validatedCompanyPhone)`.

#### N2. The tuned threshold must be plumbed into the init call, not just the constructor
- `NewSafeMemory(cfg ...memstate.Config)` is correct, but Step 4 initializes with
  `store.NewSafeMemory()` (default 0.55). If the acceptance tests require tuning (e.g., 0.45),
  the chosen `memstate.Config` must actually be passed at the GetOrCreate init site
  (`store.NewSafeMemory(memstate.Config{SupersedeThreshold: ...})`) and the value documented.

#### N3. Acceptance tests need real assertions; scores aren't publicly observable
- The skeleton (plan4 lines 122–147) only has comments. Assert on the **current-fact set**,
  e.g. exactly one `Current()` fact equal to "Customer name is Jonathan" for the supersede
  cases, and two current facts for the cross-topic cases. memstate exposes no raw `topicSim`
  via its public API, so "print actual scores" is not feasible — assert on outcomes, not
  scores. If numeric visibility is truly needed, it requires a memstate change (out of scope).

#### N4. Residual (acceptance-gated) risk — single global threshold vs all four cases
- Lowering the threshold to make John→Jonathan supersede also raises false-supersession risk
  for the cross-topic pairs (tests 3–4). The current fact phrasing already mitigates this: the
  field name acts as a topic key (same-field facts share 3 tokens: "customer/name/is";
  cross-field share ~2: "customer/is"). Keep all four cases as hard gates. If no single
  threshold passes all four, retain the field-prefix design and tune — do not remove it.
- **Also initialize Facts only when enabled** to avoid allocating an unused wrapper when
  `MEMSTATE_ENABLED` is off (Step 4 text says "when enabled"; ensure the code guards it).

#### L1. Per-turn re-add grows superseded history (informational)
- With the guard removed, an unchanged fact is re-added every turn, creating one superseded
  history entry per turn. Only current facts are surfaced by `Prompt`, and growth is bounded
  by session length, so this is acceptable at session scope. No action required.

---

## Status of prior-review issues

| Issue | Source | Status in plan4 |
|-------|--------|-----------------|
| Import cycle (`store → agent → store`) | hy3 #CR1 | Resolved (SafeMemory in `store`) — verified |
| First-match extractor breaks revision | hy3 #CR2 | Resolved (per-field newest-first extractors) — see M1 nuance |
| Naive `extractLatestField` loses safeguards | hy3 #H1 | Resolved (faithful extractors) |
| Supersession mis-modeled / unvalidated | hy3 #H2 | Resolved (tunable config + measured acceptance gates) |
| Wrong file in Step 4 | hy3 #M1 | Resolved (service.go:1177–1188) |
| Dedup-guard contradiction | hy3 #M2 | Resolved (guard removed) |
| `msgs` type/role mismatch | hy3 #M3 | Resolved (`[]store.AgentMessage`, role-filtered) |
| Pattern duplication drift | hy3 #M4 | Resolved (hoisted to package level) |
| Inconsistent panic protection | prior | Resolved (recover in each method) |
| Dead `Facts()` / testable flag | prior | `Facts()` retained for tests (used by acceptance tests); `var isMemstateEnabled` seam present |

---

## Implementation punch list (apply while coding — no further review needed)

1. **M1:** In the latest-name extractor, exclude standalone pattern #3 (or make it a fallback
   only when no explicit-marker name exists).
2. **N1:** Pass `validatedCompanyPhone` (service.go:2104), not `tenant.Phone`.
3. **N2:** Plumb the tuned `memstate.Config` into the GetOrCreate init call; document the
   chosen threshold.
4. **N3:** Write real assertions on the current-fact set (not on scores).
5. **N4:** Keep all four supersession cases as hard gates before enabling; initialize `Facts`
   only when `isMemstateEnabled()`.

With these applied, plan4 is a sound minimum viable plan and implementation may proceed.
