## Verdict: **REWORK**

The diagnosis is valuable, but I would **not approve the plan as written** if it mainly does two things: manually strip this one DB row and patch `GetReindexStatus`. The evidence shows two real failures: the KB source contains a large GTM/minified JS block before useful article content, and the UI status path masks the stalled reindex because it queries `audience_type="all"` while checkpoints are stored under concrete audiences like `"internal"` . But the proposed fix still risks being a **one-off data surgery + UI patch**, not a durable RAG ingestion/indexing fix.

## What is good

The plan correctly identifies a likely causal chain:

1. Polluted source content enters the KB.
2. Chunking produces many low-value JS/script chunks.
3. Reindex stalls or fails before useful content is fully indexed.
4. LanceDB contains mostly garbage chunks.
5. RAG retrieval returns irrelevant context.
6. The admin UI reports `"idle"` and hides the failure because the status lookup does not aggregate audience-specific checkpoints .

That is the right class of diagnosis. It is not just “LLM gave bad answer”; it traces the failure through ingestion → chunking → indexing → retrieval → UI observability.

## Blocking issues

### 1. Manual DB sanitization is acceptable only as an emergency repair, not the primary fix

Running a one-off Python script against `build/data/memos_dev.db` may recover the local tenant, but it does not prevent the next polluted upload from breaking RAG again.

The durable fix should be in the ingestion/chunking pipeline:

```text
source import / upload
  → content normalization
  → script/style/tracker boilerplate stripping
  → markdown/article segmentation
  → chunk quality filtering
  → index
  → completeness/quality proof
```

A DB cleanup script can be allowed only after Gemini implements guardrails around it: backup, transaction, exact before/after hashes, idempotency, tenant/file targeting, dry-run mode, and post-reindex proof.

### 2. The plan needs a recovered invariant, not just a fix

I would name the main invariant:

```text
INV_RAG_SOURCE_CONTENT_MUST_BE_CANONICAL_BEFORE_CHUNKING

RAG indexing must operate on canonical knowledge content, not raw scraped page artifacts.
Script/style/tracking/boilerplate blocks must be removed or rejected before chunking and vector indexing.
```

Secondary invariants:

```text
INV_RAG_REINDEX_STATUS_MUST_REFLECT_EFFECTIVE_SCOPE

When the UI asks for audience_type="all", backend status must aggregate or summarize all concrete audience checkpoints rather than looking for a literal "all" checkpoint.
```

```text
INV_RAG_INDEX_COMPLETENESS_MUST_BE_PROVABLE

A completed or healthy RAG index must be backed by checkpoint state, indexed row counts, source-file chunk counts, and retrieval smoke tests over known article queries.
```

### 3. The plan must separate three work types

This should not be one blended patch. Separate them clearly:

| Work type           |            Allowed in this PR? | Notes                                                     |
| ------------------- | -----------------------------: | --------------------------------------------------------- |
| Data repair         |                   Yes, guarded | Tenant/file-specific emergency cleanup only               |
| Runtime bug fix     |                            Yes | `GetReindexStatus` aggregation and status visibility      |
| Ingestion hardening |          Yes, but keep focused | Strip/reject script/style/tracker garbage before chunking |
| Retrieval tuning    | No, unless evidence demands it | Do not start changing ranking, embeddings, or prompts yet |
| UI redesign         |                             No | Only status accuracy / visibility                         |

### 4. The stalled indexing root cause is not fully proven yet

The evidence says the checkpoint stalled near the transition from GTM garbage to useful content. That is strong, but Gemini must still prove whether the failure is caused by:

1. chunk count / payload size,
2. LanceDB insert behavior,
3. embedding API failure,
4. checkpoint resume bug,
5. toxic/minified content causing chunk explosion,
6. or status reporting hiding an actually recoverable job.

Do not let Gemini assume “GTM caused everything” without preserving falsification. The fix should still work even if future pollution is a huge cookie banner, inline app bundle, minified JSON, CSS, SVG sprite, or unrelated boilerplate.

## Required tests

Gemini should add tests for these specific behaviors:

1. **Content sanitizer test**
   Given markdown/html containing useful article text plus `<script>`, GTM, minified JS, `<style>`, and tracker boilerplate, the canonical content passed to chunking must retain article headings/body and remove script garbage.

2. **Chunk quality test**
   Chunks dominated by minified JS or tracker code should be rejected or excluded from indexing.

3. **Reindex status aggregation test**
   If checkpoints exist for `audience_type="internal"` and/or `"external"`, a query for `"all"` must return effective status based on those concrete audiences, not `"idle"` unless all concrete scopes are truly idle.

4. **Resume/reindex completeness test**
   Reindex must be able to resume from a checkpoint and eventually index the useful article chunks after polluted content is removed.

5. **Retrieval smoke test**
   After sanitized reindex, a query like “How are credits calculated?” should retrieve article chunks, not GTM/script chunks.

6. **Negative regression test**
   The sanitizer must not delete legitimate code snippets in docs unless they are detected as raw script/style/tracker boilerplate. This matters for KBs that may legitimately document JavaScript examples.

## Revised Gemini prompt

Use this instead of approving the current plan as-is:

```text
We are working in the RAG-based Memos app repo, likely /home/chaschel/Documents/go/bchat.

Apply the same workflow discipline as mvp3/arb2: scientific method, root-cause proof, invariant recovery, small PR scope, and no symptom-only patches.

Context:
The Internal Agent for tenant 6 ("browse") is returning poor RAG answers. Diagnostics found that the KB source content includes a very large Google Tag Manager/minified JS block before the useful article content. Reindex checkpoint state appears stalled partway through the polluted content, LanceDB contains mostly garbage chunks, and the Admin UI masks the problem because it queries reindex status with audience_type="all" while checkpoints are stored under concrete audiences such as "internal".

Your task is to produce and implement a focused fix that addresses the underlying class of failure, not only this one tenant/file.

Recovered invariants:
1. INV_RAG_SOURCE_CONTENT_MUST_BE_CANONICAL_BEFORE_CHUNKING:
   RAG indexing must operate on canonical knowledge content, not raw scraped page artifacts. Script/style/tracking/boilerplate blocks must be removed or rejected before chunking and vector indexing.

2. INV_RAG_REINDEX_STATUS_MUST_REFLECT_EFFECTIVE_SCOPE:
   When the UI/API asks for audience_type="all", backend reindex status must aggregate or summarize all concrete audience checkpoints rather than looking for a literal "all" checkpoint.

3. INV_RAG_INDEX_COMPLETENESS_MUST_BE_PROVABLE:
   A healthy/completed RAG index must be provable through checkpoint state, indexed row counts, source chunk counts, and retrieval smoke tests over known article queries.

Scope:
- Allowed:
  - Add canonical content sanitization before markdown chunking / vector indexing.
  - Add chunk-quality rejection for obvious script/style/tracker/minified boilerplate chunks.
  - Fix GetReindexStatus or the equivalent backend status path so audience_type="all" reflects concrete checkpoints.
  - Add an explicitly guarded local repair utility only if necessary, with dry-run, backup, transaction, exact tenant/file targeting, before/after hashes, and idempotency.
  - Add tests proving the invariants above.
- Not allowed:
  - Do not tune LLM prompts, ranking, embeddings, or retrieval heuristics unless the existing evidence directly requires it.
  - Do not rewrite the admin UI beyond showing accurate backend status.
  - Do not hard-code tenant 6, "browse", "KB.MD", "3hsp/index.md", or exact character offsets as runtime logic.
  - Do not make the sanitizer a one-pattern GTM-only regex. It must generalize to script/style/tracker/minified boilerplate classes.

Implementation requirements:
1. Inspect the actual ingestion path:
   - upload/import handler
   - source-file persistence
   - markdown/html processing
   - chunker
   - reindex/resume logic
   - LanceDB insert path
   - RAG search path
   - Admin UI/API status path

2. Prove the failure boundary before coding:
   - Identify exactly where raw polluted content enters chunking.
   - Identify exactly where checkpoint status is stored and read.
   - Identify whether the stalled indexing is due to chunking, embedding, LanceDB insert, resume logic, or hidden failure status.
   - Record before/after evidence in the implementation summary.

3. Implement canonicalization:
   - Remove or reject script/style/tracker blocks before chunking.
   - Preserve useful markdown headings/body.
   - Preserve legitimate documentation code snippets unless they are raw page/script boilerplate.
   - Add a short code-local invariant comment at the dangerous boundary where raw uploaded/scraped content becomes RAG chunks.

4. Implement status aggregation:
   - When audience_type="all", aggregate concrete audience checkpoints for the tenant/file_type.
   - Return a status that cannot mask failed/stalled/in-progress concrete checkpoints as idle.
   - Preserve existing behavior for concrete audience queries.

5. Add tests:
   - Sanitizer removes GTM/script/style/minified boilerplate while preserving article content.
   - Chunking does not produce indexable chunks dominated by script garbage.
   - audience_type="all" status reflects internal/external checkpoint state.
   - Reindex/resume test proves useful article chunks can be indexed after sanitization.
   - Retrieval smoke test proves known article query returns article content, not tracker content.
   - Negative test proves legitimate documentation code snippets are not blindly stripped.

6. Local repair utility, if needed:
   - Put it under scratch/tools or an equivalent non-production location unless the repo already has a maintenance-tool convention.
   - It must support dry-run.
   - It must create a backup before mutation.
   - It must run in a transaction.
   - It must target exact tenant/file IDs.
   - It must print before/after byte length, removed-block summary, and content hash.
   - It must be idempotent.
   - It must not be the only fix.

Verification commands:
- Run targeted Go tests for the agent/RAG packages.
- Run the specific new tests by name.
- If integration tags are required for LanceDB/RAG, document the exact command and environment variables used.
- After local repair/reindex, prove:
  - source content no longer contains tracker/script pollution,
  - checkpoint status is accurate,
  - indexed row count matches expected sanitized chunks,
  - sample RAG query retrieves article chunks.

Implementation summary format:
A. Root cause
B. Failure boundary evidence
C. Recovered invariants
D. Files changed
E. Behavior changed
F. Behavior preserved
G. Tests added/run with exact commands
H. Local repair steps, if any
I. Remaining risks / follow-up
```

## Decision on the two requested approvals

**DB sanitization:** approve only as a guarded emergency repair after the pipeline fix is implemented or at least after the sanitizer logic is reused by the repair script. Do not approve a one-off offset-based deletion.

**Backend `GetReindexStatus` fix:** approve in principle, but require aggregation semantics and tests for `audience_type="all"` so it cannot report `"idle"` while any concrete audience checkpoint is failed, stalled, or in progress.
