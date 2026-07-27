# Plan 8 Review

**Reviewed by:** OpenCode (adversarial)
**Date:** 2026-07-27
**Verdict:** APPROVED with nits — 0 critical, 0 significant, 3 minor

---

## Prior Review Findings — Status

| Source | Finding | Status |
|--------|---------|--------|
| plan6_review | Cache format mismatch (1 of 3) | ✅ Fixed — all 3 formats handled |
| plan6_review | Wrong question count (234→176) | ✅ Fixed |
| plan6_review | Missing UserID for resource scope | ✅ Fixed |
| plan6_review | No gold baseline | ✅ Fixed — 10-question gold check added |
| plan6_review | Summary format inconsistency | ✅ Fixed |
| plan7_review | Typo "Skiped" | ✅ Fixed → "Skipped" |
| plan7_review | JSON double-counts abstention | ✅ Fixed — per_type uses inclusive totals with note |
| plan7_review | Stdout bracket note confusing | ✅ Fixed |

---

## Minor Findings

### Nit 1: CacheEntry type mismatch with extractTurns

**Location:** Lines 138-145, 56-70

`CacheEntry` struct defines `Session`, `SessionOld`, etc. as `[]map[string]any` (untyped JSON), but `extractTurns` return type is `[]store.AgentMessage`. The skeleton doesn't show the conversion step.

**Impact:** Implementation clarity only — the `mustMakeMessages` helper in `observer_test_helpers_test.go` handles this.

**Fix:** Add one line in skeleton or note: "Convert using `mustMakeMessages` from test helpers."

---

### Nit 2: Two counting conventions in same document

**Location:** Line 259 (stdout) vs Lines 287-289 (JSON summary)

Stdout uses inclusive total: `128/190 (67.4%)` — abstention (12) counted in per-type totals.
JSON summary uses unique total: `summary.total = 176`.

Both are correct but use different denominators. Without a note, implementer may think they disagree.

**Fix:** Add one-line note after JSON summary: "JSON summary.total is unique question count (176); per_type totals are inclusive (190). See stdout for inclusive breakdown."

---

### Nit 3: JSON example values are placeholders

**Location:** Lines 287-289

JSON example shows `passed: 118, failed: 56`. Stdout shows `128/190`. These don't match — they're illustrative, but an implementer may copy-paste.

**Fix:** Either (a) use stdout values in JSON example, or (b) add a comment like `"// example values — actual depends on run"`.

---

## Implementation Readiness

| Criterion | Status |
|-----------|--------|
| Struct definitions complete | ✅ |
| Pipeline steps documented | ✅ |
| All 3 cache formats handled | ✅ |
| UserID fix included | ✅ |
| Gold baseline designed | ✅ |
| Crash recovery (JSONL) designed | ✅ |
| Judge prompt variants documented | ✅ |
| Success criteria defined | ✅ |
| Files changed listed | ✅ |
| Implementation order clear | ✅ |

**Plan is implementation-ready.** All nits are documentation clarity — no logic changes needed.
