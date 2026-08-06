# Adversarial Test-Plan Review Prompt — Bug 060

> Usage: give this prompt (plus the files below) to a reviewer agent. The reviewer
> is **read-only** — it analyzes, probes, and reports. It must NOT execute the test
> plan, start servers, or modify files. Output goes to `bugs/060/plan_test_review.md`.

---

You are an adversarial reviewer for the local end-to-end test plan `bugs/060/plan_test.md` (Bug 060 — CRDB vector search returns zero results in chat flow).

Your job is to **break the plan**: find every way it could silently pass, falsely fail, hang, destroy data, cost too much, or be non-reproducible — then decide whether it is safe and meaningful to execute. Assume the plan author is competent but fallible. Do not soften findings; severity is your judgment.

## Inputs (read all)

1. `bugs/060/plan_test.md` — the plan under review
2. `bugs/060/plan.md` — implementation plan (bug context, root cause, fixes, MCP findings)
3. `bugs/060/plan_review.md` — prior adversarial review of the implementation plan
4. Relevant source (verify claims yourself, do not trust the plan):
   - `server/router/api/v1/agent/vectordb_cockroach.go` (the fix: `buildCockroachSearchQuery`)
   - `server/router/api/v1/v1.go` (routes, groups, body limits, middleware order)
   - `server/router/api/v1/agent/handlers.go` (`HandleOnboard`, `HandleImportSingleFile`, `HandleReindexTenant`, `HandleChatExternal`, `HandleTestRAGSearch`)
   - `server/router/api/v1/agent/service.go` (`ReindexTenantContentWithResume`, fallback path ~3733/3751, `SearchVectorDB` ~5228)
   - `scripts/rag_query.sh`, `Taskfile.yml`, `.env.example`, `scripts/crdb-init.sql`

## Attack categories — check every one

### A. Assumption validity
- Every concrete claim in the plan (port 5230, `.env` keys, route paths, response fields, DSNs, table/column names, baseline row counts, tenant ids). Grep the code; run read-only SQL against local CRDB if needed.
- The auth flow: exact `signin` request/response shape (field names — is it `accessToken`?), token expiry, `GET /api/v1/auth/status` semantics, whether the local admin password actually matches `BCHAT_PASS` or the SQL-reset fallback is really needed; the bcrypt-generation snippet (is `bcrypt` importable in the environment? alternative htpasswd?).
- Widget key acquisition: does `GET /:slug/config` actually return `widget_key`? Does `HandleChatExternal` require anything else (session, origin/domain allowlist `allowed_domains` for noli)?

### B. Ordering & dependencies
- Is onboard → reindex → rag/search → chat/ext truly sequential-safe? What does the reindex status endpoint return during/after work; can it race the rag/search call?
- Is reindex really async with a 30-minute window, and what happens if the server is restarted mid-reindex (resume semantics)?
- Does `HandleOnboard` actually ingest the KB synchronously, or does chunking/embedding happen only at reindex? If onboard's `importFiles` is async or partial, does Step 4 still have input?
- Tenant `retrieval_mode` default — could noli get `long_context` (reindex skipped, per handler) and silently invalidate Steps 4–6?

### C. Isolation & idempotency
- Does the plan prove the 5 existing tenants / 88 baseline rows are untouched? (SQL before/after diff.)
- Rerun-ability: if the run is aborted and restarted, does onboard fail on duplicate slug (conflict)? Does the plan handle a half-created `noli`?
- Could the run corrupt shared state: `RAG_STARTUP_REINDEX_DISABLED` in `.env` — is it actually consumed? Auto-bootstrap reindex logic (service.go ~300) — does a non-empty DB skip it?

### D. Auth / CSRF / rate-limit / body traps
- CSRF middleware behavior with Bearer vs cookie; is there any admin route called without Bearer?
- Per-tenant admin mutation rate limits — could onboard/import/reindex 429 mid-run and how does the plan handle it?
- Body limits: confirm `adminGroup` truly has none (1 MB noli.txt); confirm `publicGroup` 16 KB does not bite the chat/ext message.
- Rate limiter for signin (5/min per IP) — does the fallback password-reset + retry risk lockout?

### E. Cost & time bounds
- Embedding: 1 MB novel ≈ how many tokens/chunks/batches at the `.env` `EMBEDDING_BATCH_SIZE`? Worst-case wall time and cost. Is the $0.02 estimate defensible?
- LLM chat call — any runaway (streaming, tool loops)?
- Reindex 30-min timeout — what happens to the `agent_source_files`/`agent_vectors` state if it times out? Is the plan's poll loop bounded?

### F. Pass-criteria rigor (most important)
- **Could the test pass even if the bug were still present?** Trace: with the old buggy query, would rag/search return empty (→ fail, good) or could it return *something* (e.g., fallback to `kb`-only rows, emergency chunks, other tenants leaking, or `IN ('')` silently matching?) that still passes the "non-empty" check?
- Does "all `contentType == kb_section`" actually catch the regression? What does `HandleTestRAGSearch` return when fileType is empty — does it hit the fixed path, or a different path (e.g., BM25/hybrid, per-audience default fileType injection)?
- Chat/ext: can the reply be grounded even when retrieval is empty (fallback path) — would the plan detect that? Is the log-scan for `RAG fallback` robust (exact log string? stderr vs stdout? slog level suppressed?)?
- Are the "mid-novel scene" expectations objective enough, or is this a vibes-check? Define a stricter assertion if possible (e.g., retrieved chunk must not be from the Translator's Introduction; mention specific title fields).

### G. Cross-driver generalization
- What in §5 is still CRDB-coupled despite the claim of reuse? (SQL snippets, DSNs, `agent_vectors` schema existence on sqlite — is the table named/dimensioned the same?)
- Is the API-primary assertion really driver-independent — does `HandleTestRAGSearch` behave identically under LanceDB vs CRDB (score fields, searchMode, result caps)?
- Are the "not run / blocked" rows accurate (verify mysql `errNotImplemented` count; neon's local-task absence)?

### H. Reproducibility
- Fresh machine / fresh clone: what preconditions are unstated (cockroach binary + node already running, `.env` secrets, noli.txt path, `uuidgen`, `python3`+`bcrypt` availability)?
- Which steps depend on the author's specific machine state (already-running CRDB node, PID 4326)?

## Output format

- **Verdict:** `GO` | `GO-WITH-CHANGES` | `NO-GO`
- **Findings:** numbered list, each with: `[SEV: critical|major|minor|nit]` category (A–H), evidence (file:line or command output), and a concrete fix or plan amendment.
- **Criticality rules:** any finding that could make the test pass while the bug exists, destroy data, or exceed ~30 min of run time = critical/major. Unstated preconditions that block a fresh run = major.
- Be blunt. If the plan is sound, say so and justify — do not manufacture findings.

## Constraints

- Read-only. You may run read-only SQL against `postgresql://root@localhost:26257/bchat?sslmode=disable` and read any repo file, but you must NOT start the server, upload files, trigger reindexes, mutate rows, or spend money on embeddings/LLM calls.
