## Verdict: **APPROVE WITH NITS**

This is now much closer to the correct mvp3/arb2-style plan. It has moved from “repair this one polluted DB row” to a durable pipeline fix with recovered invariants, status observability, checkpoint persistence, guarded local repair, and tests. The new `INV_RAG_CHECKPOINT_STATE_MUST_PERSIST_ON_CANCEL` is a strong addition because it explains why the system stayed misleadingly stuck instead of becoming visibly failed .

## What is good

The plan now addresses the real failure chain:

```text
raw polluted KB source
→ polluted chunking
→ stalled/cancelled embedding/reindex
→ failed checkpoint write hidden by cancelled ctx
→ UI "all" status incorrectly reports idle
→ RAG retrieves garbage / no useful article chunks
```

The recovered invariants are appropriate:

1. `INV_RAG_SOURCE_CONTENT_MUST_BE_CANONICAL_BEFORE_CHUNKING`
2. `INV_RAG_REINDEX_STATUS_MUST_REFLECT_EFFECTIVE_SCOPE`
3. `INV_RAG_INDEX_COMPLETENESS_MUST_BE_PROVABLE`
4. `INV_RAG_CHECKPOINT_STATE_MUST_PERSIST_ON_CANCEL`

The strongest improvement is that the plan now treats this as a **RAG ingestion integrity problem**, not merely a Tenant 6 data cleanup problem.

## Nits / required refinements before implementation

### 1. Use a bounded detached context, not raw `context.Background()`

The plan says to use `context.Background()` for final failure checkpoint writes. That solves the cancelled-request problem, but raw background context can hang indefinitely if SQLite is locked or storage is slow.

Use a short bounded detached context instead:

```go
checkpointCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

The invariant should be:

```text
Failure/cancel checkpoint writes must detach from the cancelled request context,
but remain bounded by a short timeout.
```

This preserves durability without creating unbounded goroutine/database risk.

### 2. Sanitizer should return diagnostics, not only a string

`CleanRAGSourceContent(content string) string` is a good start, but the implementation should expose what it removed. Otherwise, future debugging becomes guesswork again.

Prefer:

```go
type RAGSanitizeReport struct {
    OriginalBytes       int
    SanitizedBytes      int
    RemovedSections     int
    RemovedScriptBlocks int
    RemovedStyleBlocks  int
    RejectedChunks      int
    Reasons             map[string]int
}

func CleanRAGSourceContent(content string) (string, RAGSanitizeReport)
```

This lets tests and logs prove the sanitizer acted on the right boundary.

### 3. Avoid overly broad `.js` / `.css` section deletion without preserving docs use cases

The plan says `isBoilerplateBlock` skips any section whose path ends with `.js` or `.css`. That may be okay for scraped website artifacts, but in a knowledge-base app, a source could legitimately contain documentation pages named `example.js`, `config.css`, or code-reference files.

Refine the rule:

```text
A .js/.css path is not automatically boilerplate unless it is also raw/minified/tracker-like
or appears under known scraped asset paths.
```

The negative regression test should include a legitimate documented `.js` or `.css` section and prove it is preserved.

### 4. Do not hard-code `3hsp/` into production sanitizer logic

The plan still mentions `strings.Contains(filePathLower, "3hsp/")`. That is useful diagnostic evidence, but production sanitizer logic should not encode the observed artifact path unless it is part of a configurable or clearly classified “scraped asset path” rule.

Better:

```text
Reject by content class first: script/style/minified/tracker signatures.
Use path hints only as supporting evidence, not as a sole deletion reason.
```

Tenant/file/path-specific knowledge belongs in the repair utility, not the general runtime sanitizer.

### 5. Status aggregation precedence needs to distinguish stale `in_progress`

The plan says precedence is:

```text
in_progress > failed > completed > idle
```

That is fine for live state, but this bug includes a permanently stuck `in_progress`. So aggregation should also detect stale progress, for example:

```text
if in_progress and updated_at older than timeout/stale threshold:
    report failed or stale_in_progress
```

Do not let a dead checkpoint appear actively healthy forever. At minimum, expose `updated_at`, elapsed time, and current batch progress so the UI can show “stalled” or “in progress, last updated X ago.”

### 6. Repair utility should reuse production sanitizer

The Python utility is okay for local emergency repair, but it must not drift from the Go production sanitizer.

Best options:

1. make the Python tool call a Go helper/CLI that uses `CleanRAGSourceContent`, or
2. keep Python only for DB plumbing but compare against sanitizer-equivalent rules very explicitly.

Otherwise, the repair script may clean Tenant 6 differently from future uploads.

## Root-cause/generalization check

**Pass, with nits.**

This plan now generalizes beyond GTM:

* polluted source artifacts,
* minified scripts/styles,
* request cancellation during long embedding/indexing,
* checkpoint observability masking,
* audience aggregation mismatch,
* and provable index completeness.

It is no longer merely patching the observed symptom. The remaining risk is making the sanitizer too path-specific or too destructive.

## Scope-control notes

Approved scope:

```text
ingestion sanitizer
chunk rejection
checkpoint failure persistence
GetReindexStatus aggregation
guarded local repair
tests
```

Do **not** allow scope expansion into:

```text
LLM prompt tuning
retrieval ranking changes
embedding model changes
admin UI redesign
tenant-specific runtime hacks
offset-based DB surgery
```

## Required implementation tests

These should be non-negotiable:

1. `TestCleanRAGSourceContent_RemovesScriptStyleAndTrackerBoilerplate`
2. `TestCleanRAGSourceContent_PreservesArticleMarkdown`
3. `TestCleanRAGSourceContent_PreservesLegitimateCodeExamples`
4. `TestChunkMarkdownContent_RejectsScriptDominatedChunks`
5. `TestGetReindexStatus_AllAggregatesConcreteAudienceCheckpoints`
6. `TestReindexFailureCheckpointPersistsAfterRequestContextCancel`
7. `TestRAGIndexCompleteness_UsefulArticleChunksReachVectorIndex`
8. Optional but valuable: `TestRAGSearch_ReturnsArticleChunkNotTrackerChunk`

## Narrow Gemini implementation prompt

```text
Proceed with the approved RAG ingestion/status hardening plan, with the following required refinements.

Keep the scope limited to:
- RAG source sanitization before chunking/vector indexing
- script/style/tracker/minified boilerplate rejection
- checkpoint failure persistence after request context cancellation
- GetReindexStatus aggregation for audience_type="all"
- guarded local Tenant 6 repair utility only if needed
- tests proving the recovered invariants

Recovered invariants:
1. INV_RAG_SOURCE_CONTENT_MUST_BE_CANONICAL_BEFORE_CHUNKING
2. INV_RAG_REINDEX_STATUS_MUST_REFLECT_EFFECTIVE_SCOPE
3. INV_RAG_INDEX_COMPLETENESS_MUST_BE_PROVABLE
4. INV_RAG_CHECKPOINT_STATE_MUST_PERSIST_ON_CANCEL

Required refinements:
1. Do not use raw context.Background() for checkpoint persistence. Use a detached but bounded context, e.g. context.WithTimeout(context.Background(), 5*time.Second).
2. Sanitizer must return diagnostics/report data, not only a sanitized string.
3. Do not delete every .js/.css section solely by file extension. Preserve legitimate documentation/code-reference content unless it is raw/minified/tracker-like.
4. Do not hard-code "3hsp/" as a production deletion rule. Tenant/file/path-specific rules may exist only in the local repair tool.
5. Status aggregation must not mask stale in_progress checkpoints forever. Expose or classify stale progress using updated_at/progress timing if the schema supports it.
6. If a repair utility is created, it should reuse or closely mirror the production sanitizer and must support dry-run, backup, transaction, exact tenant/file targeting, before/after hashes, and idempotency.

Required tests:
- TestCleanRAGSourceContent_RemovesScriptStyleAndTrackerBoilerplate
- TestCleanRAGSourceContent_PreservesArticleMarkdown
- TestCleanRAGSourceContent_PreservesLegitimateCodeExamples
- TestChunkMarkdownContent_RejectsScriptDominatedChunks
- TestGetReindexStatus_AllAggregatesConcreteAudienceCheckpoints
- TestReindexFailureCheckpointPersistsAfterRequestContextCancel
- TestRAGIndexCompleteness_UsefulArticleChunksReachVectorIndex
- Add retrieval smoke coverage if feasible without making the test suite brittle.

Implementation summary must include:
A. Root cause
B. Failure boundary evidence
C. Recovered invariants
D. Files changed
E. Behavior changed
F. Behavior preserved
G. Tests added/run with exact commands
H. Local repair steps, if any
I. Remaining risks/follow-up
```

Next step: let Gemini implement with the refinements above. After Gemini returns the implementation summary, we should **not approve directly**; we should hand it to StepFun for narrow code review against this approved plan.
