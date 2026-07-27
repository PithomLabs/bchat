# Plan Dry Run Assessment — Adversarial Review

**Reviewed by:** Senior Go Architect (second opinion)
**Date:** 2026-07-28
**Verdict on Assessment:** APPROVED — assessment is thorough and correct. Proceed with modifications as recommended, with 2 advisory notes on gold baseline.

---

## Review Summary

The assessment correctly identifies all review findings (C1, C2, S1, N1, N2) and prescribes appropriate fixes. The cost, time, and output format analyses are sound. One finding (S1 gold baseline) warrants qualification.

| Finding | Assessment Verdict | Adversarial Verdict |
|---------|-------------------|---------------------|
| C1: Abstention not tested | Fix required | ✅ Agree |
| C2: API call count wrong | Fix required | ✅ Agree (with note) |
| S1: No gold baseline | Optional | ⚠️ Agree, but N=1 is too weak |
| N1: No failure semantics | Fix | ✅ Agree |
| N2: Gold threshold missing | Fix | ✅ Agree |

---

## Advisory Notes

### Note 1: Gold baseline N=1 is statistically meaningless

**Location:** Lines 43-50, 131-139, 179-183

The assessment recommends 1 gold baseline question (5 total), but `openrouter/free` has 5-10% per-call variance from non-deterministic model routing. A single pass/fail is noise-dominated:

- **1/1 pass:** Does not distinguish "LLM is reliable" from "LLM got lucky"
- **0/1 fail:** Could be rate-limiting, transient error, or routing to a weaker model — not necessarily a systemic LLM failure

Plan8 uses 10 gold questions with a `≥8/10` threshold for a reason. For the dry run:

- **Option A (recommended):** Skip gold entirely. The 10-question baseline in `TestBenchmarkLongMemEval` is the real gate. The dry run's purpose is manual inspection of artifacts, not LLM reliability diagnosis.
- **Option B:** Use 3 gold questions (one per non-abstention type: single_hop, implicit_preference_v2, knowledge_update). That's enough to catch a broken model without being noise-dominated.

**Impact:** If option B is chosen, total becomes 7 questions, 21 API calls, ~$0.015 cost, ~12-15 min runtime.

---

### Note 2: C2 call count has dual interpretation

**Location:** Lines 36-39 vs Lines 51 (final verdict item 2)

C2 section says "12 API calls: 4 observer + 4 answer + 4 judge (after C1 fix)" — this is the count for C1 alone (4 questions, observer now included).

Final verdict item 2 says "Correct call count to 15 API calls: 5 observer + 5 answer + 5 judge" — this incorporates S1 (gold baseline), making 5 questions.

If S1 is rejected, item 2 in the final verdict is wrong by 3 calls. To avoid confusion, the final verdict should either:
- Decouple: "C2 fix: 12 calls for 4 questions (C1); gold baseline adjusts to 15"
- Or commit to 5 questions unconditionally

Minor clarity issue — the numbers are internally consistent when S1 is accepted.

---

## Minor Issues

### M1: `build/benchmark/` directory precondition

**Location:** Line 172

`build/benchmark/` does not exist in the repo. The implementation must call `os.MkdirAll("build/benchmark", 0755)` before writing the report file. Add to implementation checklist.

### M2: Line count estimate too low

**Location:** Line 200

Assessment estimates ~50 lines for the dry run test. Realistic estimate is ~80-100 lines:

| Section | Lines |
|---------|-------|
| Package, imports, gate check | 10 |
| Load data + pick 5 questions | 15 |
| Loop: print verbose output (×5) | 35 |
| Gold baseline handling | 10 |
| Report file writing | 15 |
| **Total** | **~85** |

### M3: `convertTurnsToMessages` type mismatch not flagged

**Location:** Line 201 (assumes shared helpers)

`convertTurnsToMessages` takes `[]map[string]string` (existing in `observer_test_helpers_test.go`), but plan9 defines `convertBenchmarkTurns` taking `[]BenchmarkTurn` (typed struct). The dry run depends on plan9's new conversion function. If plan9 implementation hasn't landed yet, the dry run can't share helpers. This is a sequencing dependency that should be noted.

---

## Final Verdict

| Decision | Value |
|----------|-------|
| **Proceed?** | Yes |
| **Conditional changes** | None required — assessment is ready |
| **Advisory notes** | 2 (gold baseline N=1 weakness, C2 dual count) |
| **Implementation risk** | Low — ~85 lines, ~$0.01-0.015, ~10-15 min runtime |

The assessment is implementation-ready. The gold baseline advisory is a trade-off decision (cost vs signal), not a correctness issue.
