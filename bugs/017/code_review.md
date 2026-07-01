## Adversarial Code Review

**Scope reviewed:** `vectordb.go`, `service.go`, `handlers.go` changes against `.kilo/plans/1782936579779-rag-long-term-fixes.md`.

**Overall verdict:** implementation is structurally correct and passes tests, but it has two functional gaps that will prevent the intended long-term fix from working reliably in production.

---

### 1. High — Prompt conflict: hard constraint contradicts fallback section
**Files:** `service.go:2802` and `service.go:2642-2645`

`buildRAGSystemPrompt` emits:
```
- Only discuss information in RETRIEVED CONTEXT below
```
and later:
```
=== RETRIEVED CONTEXT (Use ONLY this information) ===
```

When fallback activates, `generateRAGResponse` appends:
```
=== RAW KNOWLEDGE BASE FALLBACK (Retrieved chunks were insufficient) ===
[raw KB]
```

The instruction explicitly limits the model to the `RETRIEVED CONTEXT` section, but the fallback is appended outside that named section. Many LLMs will treat the fallback as out-of-scope and refuse or ignore it — reproducing the very failure mode this fix is supposed to solve.

**Recommendation:** Pass a `withFallback bool` into `buildRAGSystemPrompt` and soften the constraint when true, e.g.:
```
- If the RETRIEVED CONTEXT below does not contain an answer, use the RAW KNOWLEDGE BASE FALLBACK section
```
Or remove the hard “only” qualifier entirely for RAG mode.

---

### 2. High — Task 5 incomplete: incomplete index does not force fallback
**File:** `service.go:2698-2737`

`checkIncompleteRAGIndex` only logs a warning. It does NOT influence whether the fallback is used. The actual fallback gate is:
```go
needsFallback := len(retrieved.KBSections) == 0 || topScore < ragMinScore
```

This means: if the index is partially populated (e.g. 102/268 chunks) and the top-k retrieval happens to include a high-scoring chunk from the indexed portion, `needsFallback` is `false` and the user gets an answer drawn from incomplete context. The plan explicitly required “enable the raw-KB fallback path from task 1 automatically” for incomplete indexes.

**Recommendation:** Make `checkIncompleteRAGIndex` return a `bool` (or an enum status) and OR it into `needsFallback`:
```go
needsFallback = needsFallback || s.checkIncompleteRAGIndex(ctx, config, session)
```

Without this, the widget can return short/incomplete answers from a partially indexed novel.

---

### 3. Medium — `truncateToTokenBudget` is a byte heuristic, not real tokenization
**File:** `service.go:2539-2555`

```go
maxBytes := maxTokens * 4
```

This assumes 1 token ≈ 4 bytes, which is roughly true for English ASCII but breaks for CJK, emoji, or dense code. A 6 000-token budget could silently cheapen the fallback window for non-Latin KBs.

**Recommendation:** Use a real token estimator (e.g. `tiktoken` or the server’s existing `EstimateTokens`) to truncate by actual token count rather than bytes.

---

### 4. Low — Dead code: `indexContentForRAG` is now an unused no-op
**File:** `handlers.go:1499-1503`

No caller exists outside its own definition. Keeping it increases surface area and invites future confusion. Either remove it or leave a clear deprecation comment if removal is deferred.

---

### 5. Low — `RetrievedContext.Scores` nil-safety depends on future discipline
**File:** `vectordb.go:855` and `service.go:2618-2621`

Today all `RetrievedContext` values are constructed with `Scores`. But if a future caller does `RetrievedContext{KBSections: chunks}` without initializing `Scores`, `retrieved.Scores[0]` would panic. Consider adding a small helper:
```go
func (r *RetrievedContext) topScore() float64 {
    if len(r.Scores) > 0 {
        return r.Scores[0]
    }
    return 0
}
```
or a constructor that always initializes `Scores`.

---

### 6. Informational — Upload path now blocks on reindex
`importFiles` calls `ReindexTenantContentWithResume` synchronously. Upload API latency now includes embedding + LanceDB insert time. The plan called this acceptable, but for tenants with large KBs this can push the upload request into timeout territory if the HTTP client has a short deadline.

No code change required, but worth monitoring in production logs.

---

## Summary

| Finding | Severity | Type |
|---------|----------|------|
| Prompt constraint blocks fallback usage | High | Functional bug |
| Incomplete index does not force fallback | High | Plan deviation |
| Byte heuristic for token truncation | Medium | Correctness |
| Dead code `indexContentForRAG` | Low | Hygiene |
| `Scores` slice nil-safety | Low | robustness |
| Upload path latency | Info | Trade-off |

The two High findings are blockers for the stated goal. The model will likely continue to refuse answers from the new fallback section because of the hard prompt constraint (Finding 1), and partial indexes will still slip through without forcing the fallback (Finding 2).