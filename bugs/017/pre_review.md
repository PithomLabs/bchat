Verdict: the findings in `pre.md` are accurate. I traced the exact code paths and confirmed the root cause is a combination of strict RAG-only prompting plus an incomplete LanceDB index for the external audience. Below is a concise, code-verified report.

---

## Verified Findings

### 1. Strict RAG system prompt forces “not in retrieved context” refusals
In `buildRAGSystemPrompt` (`server/router/api/v1/agent/service.go:2711-2712`):
```
sb.WriteString("- Only discuss information in RETRIEVED CONTEXT below\n")
sb.WriteString("- Never invent information, facts, or details not in the context\n")
```

And `buildRAGFallback` (`service.go:3083-3105`) injects:
```
- If topic not in retrieved context, politely decline
```

This is exactly why the model responds with “I've checked all the retrieved context available to me…” style answers — it is following the system prompt literally.

### 2. Unstructured (Novel) imports force RAG mode even when retrieval_mode=long_context
At `service.go:1814-1828`:
```go
forceRAG := !config.HasStructuredContent && s.UseRAGPipeline()
if forceRAG {
    useRAG = true // ignores tenant retrieval_mode
}
```

`HasStructuredContent` is computed in `LoadConfig` (`service.go:1395-1397`) from DB tables:
```go
hasStructuredContent := len(services) > 0 || len(faqs) > 0 || len(exclusions) > 0 ||
    len(coverage) > 0 || len(safety) > 0 || len(sections) > 0 ||
    len(intents) > 0 || len(rules) > 0
```

For `rizal`/external the DB query showed all counts are `0`. Also, the simplified import path `importFiles` (`handlers.go:1493`) always returns:
```go
HasStructuredContent: false,
```

So the agent is forced into RAG-only mode regardless of what `tenant_config.retrieval_mode` says.

### 3. External audience IS indexed (comment at line 1086 is outdated)
The inline comment in `HandleReindexTenant` (`handlers.go:1086`) says:
```
// Note: Only indexes internal audience content. External audience is never indexed.
```

But the actual code in `ReindexTenantContentWithResume` (`service.go:720-741`) loads ALL source files and indexes them. The upload import path `indexContentForRAG` (`handlers.go:1479-1620`) also indexes by the specific `audienceType` passed to it (including `external`). The external checkpoint existing at all proves external content can enter LanceDB.

### 4. rizal external RAG index is incomplete
The checkpoint DB state:
```
11|all     |in_progress|268|36 |268|36 |...                   # reindex endpoint partial run
11|external|in_progress|268|102|268|102|$ Indexing batch...    # upload import partial run
```

`indexContentForRAG` uses batching with retries and checkpoints (`handlers.go:1538-1618`). If the HTTP request context is cancelled or times out during upload, the checkpoint stays `in_progress` and only `102` of `268` chunks are persisted. The Maria Clara ending text lives near byte `967191` in a `1047137`-byte file, putting it past chunk `102`, so it is very plausibly missing from LanceDB.

### 5. Source KB DOES contain the answer
Confirmed by direct SQL:
- `instr(...)` found the sentence at byte `967191`
- `substr(...)` extracted the passage ending with: *“Of Maria Clara nothing more is known except that the sepulcher seems to guard her in its bosom…”*

So the data exists in `agent_source_files.content`, but the vector index has not yet embedded/retrieved that chunk for the external audience.

---

## Root Cause Summary

The widget’s poor answer is NOT caused by the contact-collection change, and it is not a generic prompt bug. The causal chain is:

1. `importFiles` → `HasStructuredContent: false` for the `external` audience (novel has no `@service`, `@faq`, `@section` annotations).
2. `ChatExternal` → `forceRAG = true` because `!HasStructuredContent && UseRAGPipeline()` (`service.go:1815`), overriding `retrieval_mode=long_context` in `tenant_config`.
3. `generateRAGResponse` calls `RetrieveContextForQuery` (`vectordb.go:811`) with `audience_type="external"`, hitting LanceDB.
4. LanceDB external index is incomplete (102/268 chunks) because the synchronous upload indexing was interrupted/cancelled and left a stuck checkpoint.
5. The relevant ending chunk is not in the top-k retrieved set.
6. `buildRAGSystemPrompt` instructs the model: *“Only discuss information in RETRIEVED CONTEXT below”*, and `buildRAGFallback` injects: *“If topic not in retrieved context, politely decline.”*
7. LLM faithfully declines with the exact wording the user observed.

---

## Recommended Fixes

| Fix | Location | Notes |
|-----|----------|-------|
| **Resume/complete external indexing** | Re-run reindex for `audience_type=external` or trigger it from Admin UI. | POST `/api/v1/agent/rizal/reindex?audience_type=external&resume=true` should pick up at batch 102. |
| **Fix upload-indexing resilience** | `handlers.go:indexContentForRAG` | When `ctx` cancels, the current code already writes a detached failure checkpoint (~5s timeout), but callers should retry instead of leaving it stuck. Consider a background worker. |
| **Remove outdated comment** | `handlers.go:1086` | The comment claims external is never indexed; it is, and this misleads debugging. |
| **Soft-fallback for incomplete index** | `service.go:1830+` | When `retrieve` returns empty for RAG mode and raw `RawKB` is available, inject a “KB excerpt” fallback instead of a flat refusal. |