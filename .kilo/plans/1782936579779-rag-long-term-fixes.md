Implement the long-term RAG fixes from `.kilo/plans/1782936579779-rag-long-term-fixes.md` in `server/router/api/v1/agent/`.

**Task 1 — Raw KB fallback in RAG path**
In `service.go`, function `generateRAGResponse`: when `retrieved.KBSections` is empty or top score is below `MinScore`, append `config.RawKB` under a clearly labeled fallback section in the system prompt; truncate to a safe token budget; do not affect `long_context` tenants.

**Task 2 — Telemetry**
Log fallback activation with `tenant_slug`, `session_id`, `query`, and `chunks_found`.

**Task 3 — Fix upload indexing**
In `handlers.go`, function `indexContentForRAG`: remove the inline chunk insert/checkpoint loop. After saving source files in `importFiles`, call `ReindexTenantContentWithResume(ctx, tenantID, audienceType, false)`. Keep `indexContentForRAG` as a no-op or remove its body if unused.

**Task 4 — Fix stale comment**
Update the comment at `handlers.go:1086` — external audience is indexed when explicitly requested.

**Task 5 — Incomplete-index safeguard**
In `service.go`, before RAG generation, if tenant is in RAG mode and LanceDB has 0 chunks for the audience or the reindex checkpoint is stale/failed, log a warning and rely on the Task 1 fallback path.

**Constraints**
- No tenant-specific logic.
- Do not change behavior for internal/structured-content tenants.
- Enforce token truncation cap for fallback KB.

**Validation**
Re-upload `rizal` external KB, confirm `reindex/status` reaches `268/268`, ask “what happened to Maria Clara at the end of the novel?” and confirm a substantive answer rather than a retrieved-context refusal.
