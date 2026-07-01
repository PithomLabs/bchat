All 5 findings addressed. Tests pass.

**1. Prompt conflict with fallback (High)**
- `buildRAGSystemPrompt` now accepts `withFallback bool`.
- When `withFallback` is `true`, the constraint line becomes: *"If the RETRIEVED CONTEXT below does not contain an answer, use the RAW KNOWLEDGE BASE FALLBACK section."*
- The `RETRIEVED CONTEXT` header also changes from *"Use ONLY this information"* to a softer note directing the model to the fallback section when needed.

**2. Incomplete index triggers fallback (High)**
- `checkIncompleteRAGIndex` now returns `bool`.
- `needsFallback = needsFallback || s.checkIncompleteRAGIndex(...)` in `generateRAGResponse`.

**3. Real token truncation (Medium)**
- `truncateToTokenBudget` now uses `EstimateTokens` for the truncation budget, with a conservative byte-level pre-check before finalizing the slice.

**4. Dead code cleanup (Low)**
- Removed the unused `indexContentForRAG` no-op from `handlers.go`.

**5. Scores nil-safety (Low)**
- Added `topScore()` helper on `RetrievedContext` in `vectordb.go`.
- Updated `generateRAGResponse` to use `retrieved.topScore()`.

Validation: `go build`, `go vet`, and `go test ./server/router/api/v1/agent/...` all pass.
