# Adversarial Plan Review: bugs/049/plan2.md

**Plan:** `bugs/049/plan2.md`
**Reviewer:** Senior Go Architect
**Status:** APPROVED WITH NITS

---

## Summary

Plan v2 is significantly improved. All 9 action items from the v1 review are addressed:
- End-to-end pipeline test replaces the prompt-text/formatting/XML-parsing unit tests
- Reflector detail preservation test added
- Malformed XML test added
- Silent skip replaced with `t.Skip`
- File renamed to `observer_longmemeval_test.go`
- Edge case tests added
- Tier 2 mock fallback added
- OM vs RAG boundary documented
- Embedded data subset + extraction step added

Ready to implement after addressing the remaining concerns below.

---

## 1. Remaining Gaps

### 1a. OM config singleton race (still unaddressed)

`GetOMConfig()` returns a package-level singleton. Tests that set env vars like `OM_TOKEN_THRESHOLD` via `t.Setenv` and run in `t.Parallel` will race. This is especially relevant for Test 2 (reflector), which needs to lower `TokenThreshold` to trigger compression without processing 2000+ real tokens.

**Fix:** Either add `t.Parallel()` = `false` at the file level, or use `ReloadOMConfig()` in each test's setup with `t.Cleanup` to restore defaults.

### 1b. `createTestSession` helper location is unspecified

The plan shows it as a local helper but doesn't specify where it lives. If placed in `observer_longmemeval_test.go`, it won't be reusable by future test files.

**Fix:** Place in a dedicated `observer_test_helpers.go` within the same package. The helper must handle both in-memory session (`service.memorySessions.GetOrCreate()`) and persisted session (`store.CreateAgentSession()`), since `RunObserver` reads from in-memory cache first.

### 1c. Test 6 abstention subtest is a no-op with mock LLM

With a canned mock, the output always returns what you program it to return — "does not hallucinate" is trivially true. The abstention test only has meaning with the real LLM (Tier 2).

**Fix:** Skip the abstention subtest in mock mode (`t.Skip("abstention only meaningful with real LLM")`), or add a mock variant that returns fabricated details and assert the parser filters them out (testing the parser, not the LLM).

---

## 2. Structural Issues

### 2a. "Done" in action items table is misleading

The table says "Done" for all 9 items, but the plan is not implemented. Developers cross-referencing this table against implementation may be confused.

**Fix:** Replace "Done" with "Addressed" or "Fixed in v2".

### 2b. Test 3 "missing closing tag" expected behavior is wrong

The plan says: "Partial extraction or empty (no crash)". But `observer.go:182–185` shows:

```go
newObservations := parseXMLTag(output, "observations")
if newObservations == "" {
    newObservations = output  // fallback to raw output
}
```

When `</observations>` is missing, the regex doesn't match → `parseXMLTag` returns `""` → fallback assigns the entire raw output (including the opening `<observations>` tag).

**Fix:** Correct the expected behavior to: "Raw content preserved verbatim via fallback (no crash)."

### 2c. "long_content" edge case only tests mock path

With a mock LLM (no token limits), a 5000+ char message always passes. With a real LLM, it could hit context window limits.

**Fix:** Specify which mode this subtest runs under. Consider two variants: (a) mock — asserts no truncation, (b) real LLM — asserts graceful token-limit handling.

---

## 3. Implementation Details to Clarify

| Detail | Where | Clarification Needed |
|--------|-------|---------------------|
| `newBridgeChatTestService` sets `EncryptionMasterKey` | Test 1 setup | Observer tests don't need an encryption service (no signing key). A simpler helper without encryption reduces test init time. |
| `setupTestSigningKey` call | Test 1 setup | Not needed for observer tests — adds unnecessary dependency on `bridge_foundation_test.go`. |
| How Test 2 triggers reflector | "set threshold low" | Vague. Use `t.Setenv("OM_TOKEN_THRESHOLD", "1")` + `ReloadOMConfig()` before the test, with `t.Cleanup` to restore defaults. Show this pattern explicitly. |
| `TestFormatForObserver` assertion | Test 5 subtest | Says "just that role and content transfer correctly" but doesn't specify how. Use `strings.Contains` or direct field checks rather than exact format strings. |
| Thread vs Resource scope | Not tested | `RunObserver` has different DB query paths for `OMScopeThread` vs `OMScopeResource`. Test 1 should test both, or document why resource scope is excluded. |
| `newBridgeChatTestService` uses `http.Handler` mocks | Test 1 setup | Observer tests don't need HTTP handlers — they call `RunObserver` directly. A leaner helper avoids pulling in unnecessary HTTP infrastructure. |

---

## 4. Mock Infrastructure Notes

The plan reuses `newMockLLMServer` and `withMockLLM` correctly. One addition: the `mockObservations` map should include edge-case entries:

```go
"trivial_response": `<observations></observations>`,
"empty_response":   "",  // completely empty LLM response
```

This lets Test 4 (edge cases) test what happens when the mock returns trivial or empty output.

---

## 5. Minor Nits

| Nit | Location | Suggestion |
|-----|----------|------------|
| "consolidated" misspelling | Test 6 description | s/consolidated/consolidates |
| `t.Skip("Using embedded subset...")` | Test 5 data handling | Good — self-documenting skip message. Keep as-is. |
| AgentSession requires `AudienceType` | Test 1 session setup | `GetOrCreate` hardcodes `AudienceType: "external"`. Ensure the test tenant has an `external` audience row, or the observer path may fail at DB query time. |
| `MemorySessionStore` TTL | Test 1 session setup | Default TTL is 30 minutes. Fine for unit tests, but add a note if tests take longer than expected. |

---

## 6. Pre-Implementation Checklist

1. Replace "Done" with "Addressed" in the action items table
2. Add `ReloadOMConfig` + `t.Cleanup` pattern to the reflector test section
3. Correct Test 3 missing-closing-tag expected output to "raw fallback"
4. Add edge-case entries to `mockObservations` map
5. Specify `createTestSession` goes in `observer_test_helpers.go`
6. Add abstention subtest behavior for mock mode
7. Document thread vs resource scope testing decision
8. Add OM config singleton race note
