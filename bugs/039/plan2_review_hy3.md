# Adversarial Plan Review (hy3): memstate Integration — Revised Plan

**Reviewer role:** Senior security & reliability engineer
**Plan reviewed:** `bugs/039/plan2.md`
**Prior review:** `bugs/039/plan_review.md` (of `plan.md`)
**Verdict:** **REWORK** — the revision resolves the plan1 criticals but introduces two new
critical defects (an import cycle that fails compilation, and a data path that structurally
defeats belief revision for name/address).

---

## Summary

plan2 correctly addresses the plan1 blockers: cross-session memory is descoped (D1), raw
messages are no longer stored as facts (D2), a mutex wrapper is added (D3), the feature is
default-off (D4), config is standalone (D5), and the dependency is pinned/vendored (D6). The
prompt-header and backward-compatibility concerns are handled well.

However, two of the fixes introduce new critical problems:

- **CR1:** placing the `SafeMemory` type in the `agent` package while referencing it from
  `store.AgentSession` creates a circular import — the code will not compile.
- **CR2:** feeding memstate exclusively from `extractCollectedInfo()` output means the *new*
  value for name and address never reaches memstate (the extractor returns the first match),
  so supersession — the entire point of the feature — never fires for 2 of the 3 fact types.

Both must be resolved before implementation.

### Verification basis (from source)

- `processChat` (service.go:2087) sanitizes input, appends to `session.Messages`
  **unlocked** (2095–2100), then calls `extractCollectedInfo(session.Messages, ...)` (2105)
  and copies values into the session only when the session field is empty (2106–2114).
- `extractCollectedInfo` (service.go:3648):
  - **Name** is captured "only if not already found" (3694) → **first match wins** across
    history.
  - **Address** is captured "only if not already found" (3760) → **first match wins**.
  - **Phone** has explicit correction/override patterns that replace a previously captured
    value (3707–3719); plain phone capture is first-match (3722).
  - Email/phone retraction handling exists (3769–3775).
- `agent` package (service.go) imports `github.com/usememos/memos/store` and uses
  `store.AgentSession`. Therefore `store` cannot import `agent`.
- memstate library (`github.com/PithomLabs/memstate@main`): `Memory` has no internal mutex;
  default `SupersedeThreshold = 0.55` lexical IDF overlap; `Prompt` estimates ~4 chars/token;
  API `New/Add/Prompt/Facts/Fact.Current/Fact.Text` all exist and match the plan.

---

## Findings

### CRITICAL

#### CR1. Import cycle — plan will not compile (new regression)
- **Severity:** Critical
- **Regression or new:** New finding, introduced by D3/Step 2–3.
- **Failure scenario:** Step 2 declares `Facts *SafeMemory` on `store.AgentSession` (package
  `store`, `store/agent.go`). Step 3 defines `SafeMemory` in
  `server/router/api/v1/agent/safe_memory.go` (package `agent`). The `agent` package already
  imports `store` (e.g., `store.AgentSession` throughout service.go). Adding a `store` →
  `agent` reference produces a circular import: `store` → `agent` → `store`. Compilation
  fails.
- **Why plan1 didn't hit this:** plan1 typed the field as `*memstate.Memory` (an external
  package), so `store` → `memstate` had no cycle. Wrapping the type and putting the wrapper
  in `agent` created the cycle.
- **Recommended fix (pick one):**
  1. Define `SafeMemory` in the `store` package (it only needs `sync` + `memstate`), so the
     field type is `*store.SafeMemory` — no cycle.
  2. Keep the field as `*memstate.Memory` in `store` and apply the mutex via a wrapper used
     only at the `agent`-package call sites (store stays as plain memstate).
  3. Type the field as a small interface (or `any`) declared in `store`, implemented by
     `agent.SafeMemory`.
  - Option 1 is cleanest and keeps the wrapper reusable.
- **Disposition:** In this plan (blocking).

#### CR2. Data path defeats belief revision for name & address (new logic flaw)
- **Severity:** Critical
- **Regression or new:** New finding, introduced by D2/Step 5's decision to feed memstate
  *only* from `extractCollectedInfo()` output.
- **Failure scenario:** `extractCollectedInfo` returns the **first** match for name
  (service.go:3694) and address (service.go:3760) across the entire message history. When a
  customer says "My name is John" and later "Actually, Jonathan," `customerInfo.Name` remains
  `"John"` on every turn. Step 5 therefore only ever calls
  `safeFactsAdd(session.Facts, "Customer name is John")`; the new value "Jonathan" never
  enters memstate, so memstate's supersession is never triggered. The same applies to address
  (Rome → Milan never updates). Only **phone** has correction/override logic (3707–3719), so
  belief revision effectively works for phone only.
- **Consequence:** The feature's flagship use case (name/location change) cannot work through
  this path, and the plan's own Testing Strategy #3 and #5 ("Customer location is Rome →
  Milan supersedes"; "verify only current facts appear") will fail — `customerInfo.Address`
  is still "Rome" when the LLM is prompted. This is a correctness gap, not a tuning issue.
- **Recommended fix (pick one, must be decided in-plan):**
  1. Before feeding memstate, extract the **latest** occurrence of each field (scan history
     newest-first, or track per-turn deltas), so the changed value reaches memstate.
  2. Extend `extractCollectedInfo` with name/address correction handling analogous to the
     existing phone-correction patterns, and feed memstate from those.
  3. Feed memstate from a source that captures the newly stated value on the turn it is said
     (e.g., extract from the current `userMessage` only, not the whole history).
  - Whichever path is chosen, add a fixture test proving name John→Jonathan and address
    Rome→Milan supersede end-to-end (extractor → memstate → prompt).
- **Disposition:** In this plan (blocking).

### MEDIUM

#### M1. Inconsistent panic protection
- **Severity:** Medium
- **Regression or new:** New (partial). plan1 had no protection; plan2 protects `Add` only.
- **Failure scenario:** `safeFactsAdd` wraps `Add` in `recover()` (Step 5), but Step 6 calls
  `session.Facts.Prompt("", 500)` (and any `Facts()` reads) with no recover. A panic there is
  unguarded at the memstate boundary (it would rely on Echo's outer middleware recover).
- **Note:** Because the `SafeMemory` mutex now serializes access, the *fatal* concurrent-map
  case (which `recover()` cannot catch) is prevented; the remaining `recover()` is
  defensive-only. Either protect all wrapper entry points consistently or document that only
  `Add` needs it and why.
- **Recommended fix:** Add the same `recover()` guard inside `SafeMemory.Prompt`/`Facts`
  (centralizing it in the wrapper is cleaner than the free `safeFactsAdd` helper).
- **Disposition:** In this plan.

#### M2. D3's `session.Messages` claim is unfulfilled
- **Severity:** Medium
- **Regression or new:** Carried over; the plan overstates what it fixes.
- **Failure scenario:** D3 states the wrapper "also fixes the pre-existing unsynchronized
  `session.Messages` mutation if wired up properly," but the plan never wires it up.
  `session.Messages`/`MessageCount` are still mutated without a lock (service.go:2095–2100);
  the `SafeMemory` mutex guards `Facts` only. Concurrent same-session turns still race on
  `Messages`.
- **Recommended fix:** Either remove the claim, or actually serialize `processChat` for the
  same session (close the `SessionLock`/`IdempotencyMu` gap noted in the plan1 review). Track
  the `Messages` race as a separate pre-existing bug if out of scope here.
- **Disposition:** Correct the claim in this plan; the `Messages` race fix can be a separate
  follow-up.

### LOW / NITS

#### L1. `safeFactsAdd` cannot be declared inside `processChat`
- **Severity:** Low
- The Step 5 snippet shows `func safeFactsAdd(...)` inside the `processChat()` body. Go does
  not allow nested named function declarations; it must be package-level (or fold the
  `recover()` into the `SafeMemory` wrapper per M1). Cosmetic/illustrative, but should be
  corrected so implementers don't copy it verbatim.
- **Disposition:** In this plan (trivial).

#### L2. `isMemstateEnabled()` testability
- **Severity:** Low
- Reads `os.Getenv` on every call (invoked in `GetOrCreate`, `processChat`, and
  `buildSystemPrompt` — several times per request) and is not overridable in tests without
  mutating process env. Consider caching the value once, or providing a package-level
  override seam for tests (Q6).
- **Disposition:** Optional / separate.

#### L3. Backward compatibility is clean (no action)
- **Severity:** Informational
- With `MEMSTATE_ENABLED` unset (default `false`), initialization (Step 4), fact tracking
  (Step 5), and prompt injection (Step 6) are all gated; behavior matches today. No implicit
  default changes were found (Q7 satisfied).

---

## Status of plan1 issues (resolution check)

| plan1 issue | Status in plan2 |
|-------------|-----------------|
| C1 dependency `v0.1.0` nonexistent | Resolved (D6/Step 1: pseudo-version or vendor) |
| C2 memstate not thread-safe + unserialized `processChat` | Partially resolved: `SafeMemory` mutex serializes `Facts`; the `session.Messages` race is still open (M2) |
| C3 shared cross-session `Memory` race | Resolved by removal (D1 descopes cross-session) |
| C4 prompt injection via raw messages | Resolved (D2 + neutral header, Step 6) |
| H1 unbounded `userMemories` leak | Resolved by removal (D1) |
| H2 cross-session lost on restart | Resolved by removal / explicit Limitation #2 |
| H3 "zero-dependency" false claim | Partially: Overview still calls memstate "deterministic state memory library with belief revision" (no longer claims zero-dependency); vendoring makes deps explicit — acceptable |
| H4 supersession threshold unvalidated | Acknowledged as Limitation #3 + Testing #3; still must be validated before enable |
| M1 extractCollectedInfo "replaced" contradiction | Resolved (Step 5 explicitly keeps it) — but see CR2, keeping it as the sole source breaks name/address revision |
| M2 `json:"-"` rationale | Resolved (Step 2 note corrected) |
| M3 store depends on memstate | Reintroduced worse as CR1 (store now depends on `agent`) |

---

## Required changes before approval

1. **CR1:** Move `SafeMemory` into `store` (or otherwise break the `store` → `agent` import
   cycle). Verify with a compile (`go build ./...`).
2. **CR2:** Change the data path so the *latest* name/address value reaches memstate (extend
   correction handling, scan newest-first, or feed from the current turn), and add an
   end-to-end supersession test for name and address.
3. **M1:** Make panic handling consistent across all `SafeMemory` entry points (or document
   why only `Add` is guarded), centralized in the wrapper.
4. **M2:** Correct or fulfill the D3 claim about `session.Messages`.
5. **L1:** Make `safeFactsAdd` package-level (or fold into the wrapper).
6. Keep the pre-enable gate on H4: require the Rome→Milan-supersedes / billing↔account-does-
   not tests to pass before `MEMSTATE_ENABLED=true` in any environment.

Once CR1 and CR2 are fixed and M1/M2/L1 addressed, this plan is approvable. The remaining
items (L2, H4 tuning) can proceed as normal pre-production validation.
