## Adversarial Code Review — All 5 Findings Addressed

### 1. Prompt conflict with fallback (was High)
**Verdict:** Fixed.

`buildRAGSystemPrompt` now accepts `withFallback bool`. When true:
- Constraint becomes: *"If the RETRIEVED CONTEXT below does not contain an answer, use the RAW KNOWLEDGE BASE FALLBACK section."* (`service.go:2807`)
- Retrieval header becomes: *"Note: If the RETRIEVED CONTEXT below does not contain an answer, use the RAW KNOWLEDGE BASE FALLBACK section."* (`service.go:2851-2852`)
- Non-fallback path preserves the original hard constraint (`service.go:2809`)

`generateRAGResponse` only passes `needsFallback` as the fifth arg, so the relaxed prompt is scoped strictly to fallback cases.

### 2. Incomplete index triggers fallback (was High)
**Verdict:** Fixed.

`checkIncompleteRAGIndex` now returns `bool`. It returns `true` when:
- No chunks exist for the audience (`!hasAudienceChunks`), OR
- Reindex status is `stale_in_progress` or `failed` (`service.go:2730`)

`generateRAGResponse` ORs this into `needsFallback` (`service.go:2634-2635`):
```go
incompleteIndex := s.checkIncompleteRAGIndex(ctx, config, session)
needsFallback = needsFallback || incompleteIndex
```

This guarantees partial indexes force the raw-KB path instead of returning incomplete answers.

### 3. Real token truncation (was Medium)
**Verdict:** Fixed.

`truncateToTokenBudget` now uses `EstimateTokens` for the budget check rather than a raw byte heuristic:
```go
tokens := EstimateTokens(text)
if tokens <= maxTokens {
    return text
}
```
It still uses a byte-level pre-check for an initial slice candidate and then validates with `EstimateTokens(candidate)` (`service.go:2543-2558`). The token-based guard is authoritative.

### 4. Dead code cleanup (was Low)
**Verdict:** Fixed.

`indexContentForRAG` is fully removed from `handlers.go`. The upload path in `importFiles` calls `ReindexTenantContentWithResume(ctx, tenantID, audienceType, false)` directly (`handlers.go:1480`).

### 5. Scores nil-safety (was Low)
**Verdict:** Fixed.

`RetrievedContext.topScore()` helper added in `vectordb.go:802-807`:
```go
func (r *RetrievedContext) topScore() float64 {
    if len(r.Scores) > 0 {
        return r.Scores[0]
    }
    return 0
}
```

`generateRAGResponse` uses `retrieved.topScore()` (`service.go:2623, 2631`).

---

## Remaining Minor Observations

**A. `truncateToTokenBudget` trimming could return empty string + ellipsis**
When `end < 100`, `candidate[:max(0, end-100)]` yields `""`, and the function returns `"..."`. This is acceptable for a best-effort truncator, but worth noting if the raw KB is ever smaller than the budget and `EstimateTokens` thinks it needs trimming.

**B. `checkIncompleteRAGIndex` does a full chunk scan on every chat**
`ListChunks` returns all tenant chunks, and the code iterates to find any with matching `AudienceType`. For tenants with many audiences/chunks, adding a server-side `audience_type` filter or `LIMIT 1` would be more efficient, though correctness is fine.

**C. Constraint language is still slightly ambiguous**
The non-fallback constraint says `- Never invent information, facts, or details not in the context`. When fallback is active, "the context" could be interpreted by some models as referring only to the RETRIEVED CONTEXT section. The current explicit directive overrides this, but tightening the wording to `not in the RETRIEVED CONTEXT or RAW KNOWLEDGE BASE FALLBACK sections` would remove any residual ambiguity.

---

## Summary

| Finding | Status |
|---------|--------|
| Prompt conflict blocks fallback | ✅ Fixed |
| Incomplete index does not force fallback | ✅ Fixed |
| Byte heuristic for token truncation | ✅ Fixed |
| Dead code `indexContentForRAG` | ✅ Fixed |
| `Scores` slice nil-safety | ✅ Fixed |

The implementation is sound. Observed risks (A, B, C) are minor and do not block the stated goals. No functional blockers remain.
