# Code Review — `memstate` Session-Scoped Belief Revision

**Subject:** Implementation described in `code.md`, verified against actual code.
**Verdict:** REWORK — one build blocker as-committed + two correctness/concurrency issues.

## Findings

| # | Severity | Location | Issue | Evidence | Recommendation |
|---|----------|----------|-------|----------|----------------|
| C1 | Critical (fix now trivial) | `go.mod:45,122`; `go.sum` | Build non-reproducible: local `replace => /home/chaschel/Documents/go/mnem-main/go`; memstate has **no** `go.sum` entry; marked `// indirect` but directly imported. | `grep memstate go.sum` → 0; replace target is a local path. The pinned pseudo-version `v0.0.0-20260714224641-ff73beb8902f` matches the public repo HEAD `ff73beb8902f` (PithomLabs/memstate, Jul 14 2026), so it resolves from GitHub. | Remove the `replace` line; run `go mod tidy` (fetches the published module, populates `go.sum`, flips the require to direct). Optionally cut a semver tag upstream and pin to it (repo currently has 0 releases). |
| H2 | High | `service.go:2116-2124, 2685, 2692` | Section 0 injects stale `session.CustomerName/Phone/Location` (set only-if-empty, never updated) while Section 0.5a injects the corrected current memstate fact as "ground truth". After John→Jonathan the prompt contradicts itself. | Section 0 at 2685; 0.5a at 2692; fields set only-if-empty at 2116-2124. | When memstate is enabled and holds a current value for a field, reconcile/suppress the stale Section 0 value (update `session.CustomerName` from `extractLatestName`, or gate the Section 0 line when 0.5a covers it). |
| H3 | High | `service.go:2095, 2128-2135` | `extractLatestName/Phone/Address` range over `session.Messages` with **no lock**, racing the unsynchronized `append` at 2095 for concurrent same-session turns (`ClientMessageID` empty). `SafeMemory`'s mutex covers only the memstate maps, not the session slice. | append at 2095; readers at 2128-2135; no `IdempotencyMu` use around them. | Serialize per-session reads/writes of `session.Messages`, or explicitly document the pre-existing race and scope it. |
| M1 | Medium | `service.go:3894-3895` | Weak name markers `it's`/`this is`/`it is` remain in `latestNamePatterns[0]`. Under newest-first, "it's broken" captures "broken" and supersedes a correct name — same class the plan fixed by dropping standalone pattern #3. | `latestNamePatterns[0]` includes `it's|this is|it is`; `isCommonWord` doesn't filter "broken"/"fine". | Drop `it's`/`this is`/`it is` from the latest-name markers; keep `my name is`/`I'm`/`I am`/`call me`. |
| L1 | Low | `service.go:3894-3895` | `gofmt -l` flags the file (mis-indented `latestNamePatterns` var block). code.md claims formatting verification. | `gofmt -l server/router/api/v1/agent/service.go` → flagged. | Run `gofmt -w` on the edited file. |
| L2 | Low | `memstate_test.go` | `TestExtractLatestPhoneExcludesTenant` uses 7-digit "555-9999", which never matches the 10-digit `latestPhonePattern`, so it passes without exercising tenant exclusion. | Phone pattern requires 10 digits; "555-9999" = 7 digits. | Use a 10-digit tenant phone so the test actually validates exclusion. |
| L3 | Low | `service.go:2126-2137` | Per-turn re-add grows history by one fact each turn (no dedup). Bounded by 50-turn cap; only current facts surface. | `Add` called every turn with identical text. | Acceptable; optionally `Current()`-check before `Add`. |
| L4 | Low | `service.go:3888+` | Prompt-injection via extracted facts: acceptable — name/phone/address regexes constrain character classes, so payloads can't reach the prompt. | Patterns are letter/digit/street-word restricted. | No change. |

## Confirmed Correct
- `store/safe_memory.go`: no import cycle (lives in `store` package); `Add/Prompt/Facts` each mutex-locked + `recover()`-guarded; `Facts()` returns a deep copy.
- Per-session `SafeMemory` instances (`store/agent.go:271`, `Facts *SafeMemory`) — memstate internal maps never shared → no fatal concurrent-map write.
- Init gated on enable (`service.go:1196`); `validatedCompanyPhone` used (not `tenant.Phone`).
- Standalone name pattern #3 correctly excluded; Section 0.5a positioned before OM (2692 vs 2701).
- Four supersession acceptance tests present with real assertions on the current-fact set; nil/init and extraction tests present (11 total).

## Required Before Merge
1. **C1** — remove local `replace`, run `go mod tidy`.
2. **H2** — reconcile stale Section 0 vs 0.5a.
3. **H3** — serialize/guard `session.Messages` access (or document).
4. **M1** — drop weak name markers.
