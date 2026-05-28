## Verdict: **DO NOT APPROVE DIRECTLY — send to StepFun review**

This is an implementation walkthrough, not an independent code review. Under the mvp3/arb2 workflow, the right next move is a **narrow StepFun code review** against the approved plan. The claimed implementation covers the approved scope: sanitizer, garbage-chunk rejection, bounded detached checkpoint writes, `audience_type="all"` status aggregation, stale checkpoint detection, local repair utility, and seven tests passing .

The walkthrough is promising, but StepFun must verify the actual code, because several areas are easy to get subtly wrong:

1. whether sanitizer rules are general and not GTM/tenant/path-specific,
2. whether legitimate `.js` / `.css` documentation is preserved,
3. whether `context.WithTimeout(context.Background(), …)` is used only for final checkpoint persistence and not as a broad cancellation bypass,
4. whether status aggregation handles mixed `internal` / `external` / `all` checkpoint states correctly,
5. whether `stale_in_progress` does not break resume logic or existing callers,
6. whether the Python repair utility is safely scoped and does not become the source of truth,
7. whether tests are meaningful rather than just testing synthetic happy paths.

## StepFun review prompt

````text
You are StepFun Flash acting as a narrow Go code reviewer for the RAG-based Memos app.

Review the completed implementation against the approved RAG Ingestion Integrity and Observability Hardening plan. Do not redesign the system. Do not suggest broad prompt-tuning, embedding, retrieval-ranking, or UI redesign changes. Focus only on correctness, regressions, invariants, and scope control.

Repo:
/home/chaschel/Documents/go/bchat

Implementation summary claims:
- RAG source content is sanitized before chunking/vector indexing.
- `CleanRAGSourceContent(content string) (string, RAGSanitizeReport)` strips script/style/tracker/minified boilerplate.
- `isBoilerplateBlock` preserves legitimate documented code snippets while rejecting minified code blocks.
- `IsGarbageChunk` rejects individual chunks dominated by minified code.
- `ChunkMarkdownContent` integrates sanitizer and garbage-chunk filtering with local invariant comments.
- `GetReindexStatus` aggregates concrete checkpoints when `audience_type="all"` is requested.
- stale `in_progress` checkpoints older than 1 hour are classified as `stale_in_progress`.
- `ReindexTenantContentWithResume` and `indexContentForRAG` persist final failed/completed checkpoints using a detached bounded context.
- Local repair utility `scratch/tools/clean_gtm_kb.py` is scoped to Tenant 6, supports dry-run, backups, transaction, hashes, and idempotency.
- Tests in `rag_sanitizer_test.go` pass.

Files to inspect:
- `server/router/api/v1/agent/chunker.go`
- `server/router/api/v1/agent/service.go`
- `server/router/api/v1/agent/handlers.go`
- `server/router/api/v1/agent/rag_sanitizer_test.go`
- `scratch/tools/clean_gtm_kb.py`
- any related model/types files touched by the implementation
- git diff / status for accidental unrelated changes

Approved recovered invariants:
1. `INV_RAG_SOURCE_CONTENT_MUST_BE_CANONICAL_BEFORE_CHUNKING`
   RAG indexing must operate on canonical knowledge content, not raw scraped page artifacts. Script/style/tracking/minified boilerplate must be removed or rejected before chunking and vector indexing.

2. `INV_RAG_REINDEX_STATUS_MUST_REFLECT_EFFECTIVE_SCOPE`
   When UI/API asks for `audience_type="all"`, backend status must aggregate concrete audience checkpoints instead of looking only for a literal `"all"` checkpoint. It must not report idle while a concrete checkpoint is in progress, failed, or stale.

3. `INV_RAG_INDEX_COMPLETENESS_MUST_BE_PROVABLE`
   A healthy RAG index must be provable by checkpoint state, indexed row counts, source chunk counts, and useful retrieval/index smoke coverage.

4. `INV_RAG_CHECKPOINT_STATE_MUST_PERSIST_ON_CANCEL`
   If request context is cancelled or times out, final failure checkpoint persistence must detach from the cancelled request context but remain bounded by a short timeout.

Review checklist:

A. Sanitizer correctness
- Verify sanitization occurs before chunking/vector indexing on the real ingestion path.
- Verify script/style HTML blocks are stripped.
- Verify minified/tracker boilerplate is rejected by content class, not by tenant/file/path-specific hacks.
- Verify production logic does not hard-code Tenant 6, `3hsp/`, exact GTM offsets, or exact local artifact names.
- Verify `.js` / `.css` sections are not blindly deleted solely by extension if they contain legitimate documentation.
- Verify legitimate markdown article text, headings, and formatted code examples are preserved.
- Verify `RAGSanitizeReport` is accurate enough to support debugging and tests.

B. Chunk rejection correctness
- Verify `IsGarbageChunk` rejects script-dominated chunks without rejecting normal prose or documentation code examples.
- Verify rejected chunks do not enter vector indexing.
- Verify chunk filtering does not silently drop all content without an observable report or test coverage.

C. Checkpoint persistence correctness
- Verify checkpoint failure persistence uses a detached but bounded context, e.g. `context.WithTimeout(context.Background(), ...)`.
- Verify this detached context is used only for final checkpoint/status persistence after cancellation/error, not to bypass normal request cancellation for long-running work.
- Verify errors from checkpoint persistence are handled/logged and not silently swallowed.
- Verify completed/failed checkpoint state remains internally consistent.

D. Reindex status aggregation correctness
- Verify `GetReindexStatus` preserves existing behavior for concrete audiences.
- Verify `audience_type="all"` aggregates concrete audience checkpoints correctly.
- Verify status precedence is correct for mixed states:
  - active `in_progress` should not be hidden,
  - `failed` should not be hidden,
  - stale `in_progress` should be visible,
  - `completed` should not override failed/stale/in-progress concrete states.
- Verify stale detection uses reliable timestamp fields and does not break resume semantics.
- Verify callers expecting previous status strings will not crash or mis-handle `stale_in_progress`.

E. Local repair utility safety
- Verify `scratch/tools/clean_gtm_kb.py` is not imported into production runtime.
- Verify it is exact-target scoped and cannot accidentally mutate broad tenant/file sets.
- Verify dry-run performs no mutation.
- Verify it creates backups before mutation.
- Verify it uses a transaction.
- Verify it prints before/after length and hashes.
- Verify it is idempotent.
- Verify it does not rely on brittle offset-based deletion.

F. Tests
Inspect `server/router/api/v1/agent/rag_sanitizer_test.go`.
Confirm these behaviors are genuinely tested:
- `TestCleanRAGSourceContent_RemovesScriptStyleAndTrackerBoilerplate`
- `TestCleanRAGSourceContent_PreservesArticleMarkdown`
- `TestCleanRAGSourceContent_PreservesLegitimateCodeExamples`
- `TestChunkMarkdownContent_RejectsScriptDominatedChunks`
- `TestGetReindexStatus_AllAggregatesConcreteAudienceCheckpoints`
- `TestReindexFailureCheckpointPersistsAfterRequestContextCancel`
- `TestRAGIndexCompleteness_UsefulArticleChunksReachVectorIndex`

Check whether tests are too synthetic or merely assert implementation details. Add review notes if missing:
- mixed checkpoint state precedence,
- stale `in_progress`,
- legitimate `.js` / `.css` docs,
- sanitizer report accuracy,
- garbage chunk rejection without over-deletion.

Required commands:
Run or verify equivalent:
```bash
go test -v ./server/router/api/v1/agent/... -run "TestCleanRAGSourceContent|TestChunkMarkdownContent|TestGetReindexStatus|TestReindexFailureCheckpoint|TestRAGIndexCompleteness"
go test ./server/router/api/v1/agent/...
python3 scratch/tools/clean_gtm_kb.py --dry-run
````

Also inspect:

```bash
git status --short
git diff -- server/router/api/v1/agent/chunker.go server/router/api/v1/agent/service.go server/router/api/v1/agent/handlers.go server/router/api/v1/agent/rag_sanitizer_test.go scratch/tools/clean_gtm_kb.py
```

Scope-control rules:

* Do not approve changes to LLM prompts, retrieval ranking, embedding models, unrelated UI redesign, unrelated DB schema changes, or tenant-specific runtime hacks.
* Data repair artifacts/backups may exist locally, but production/runtime code must not depend on them.
* The runtime fix must generalize beyond GTM and Tenant 6.

Output format:
A. Verdict: APPROVE / APPROVE WITH NITS / REWORK
B. Files reviewed
C. Summary of actual changes
D. Invariant verification
E. Blocking issues, if any
F. Nits, if any
G. Tests run and results
H. Scope-control assessment
I. Final recommendation

```

## My current gatekeeping position

The implementation is **review-ready**, not merge-ready. If StepFun confirms the code matches the claims and the tests are not superficial, I would likely move this to **APPROVE or APPROVE WITH NITS**.
```
