Implemented all tasks from `.kilo/plans/1782936579779-rag-long-term-fixes.md`.

**`vectordb.go`**
- Added `Scores []float64` to `RetrievedContext` and populated it from `SearchResult.Scores`.

**`service.go`**
- Added constants `ragMinScore = 0.25` and `ragFallbackTokenBudget = 6000`.
- Added helper `truncateToTokenBudget`.
- Modified `generateRAGResponse` to trigger fallback when `retrieved.KBSections` is empty or the top score is below `ragMinScore`. On fallback:
  - Logs telemetry: `tenant_slug`, `session_id`, `query`, `chunks_found`, `top_score`.
  - Appends `config.RawKB` under a clearly labeled `RAW KNOWLEDGE BASE FALLBACK` section in the system prompt, truncated to the token budget.
- Added `checkIncompleteRAGIndex`: called before generation to warn when LanceDB has 0 chunks for the audience or the reindex checkpoint is `stale_in_progress`/`failed`.

**`handlers.go`**
- Replaced the inline chunk insert/checkpoint loop in `importFiles` with `ReindexTenantContentWithResume(ctx, tenantID, audienceType, false)`.
- Reduced `indexContentForRAG` to a no-op returning `nil`.
- Fixed the stale comment at line 1086 to reflect that external audience is indexed when explicitly requested.

Validation: `go build` and `go vet` pass cleanly; `go test ./server/router/api/v1/agent/...` passes.