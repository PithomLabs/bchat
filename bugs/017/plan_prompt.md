# RAG Long-Term Fixes

**Verified root cause:** strict RAG-only prompt + incomplete external audience LanceDB index for `rizal`. Source KB contains the answer; retrieval misses it because indexing stopped at chunk 102/268.

## Goals
- Eliminate the “not in retrieved context” answer pattern for cases where the underlying KB actually contains the answer.
- Make external-audience RAG indexing reliable even when upload HTTP requests are short-lived.
- Align code comments/docs with actual behavior.

## Non-goals
- Do NOT add tenant-specific logic; keep fixes general for any tenant.
- Do NOT change prompt behavior for internal/structured-content tenants.

---

## Tasks

### 1. Add retrieval empty-state fallback to full raw KB
**File:** `server/router/api/v1/agent/service.go`
**Function:** `generateRAGResponse`

When `retrieveContextForQuery` returns an empty/insufficient `RetrievedContext`, and the tenant has `config.RawKB` available, append the raw KB content under a clearly labeled fallback section and let the LLM answer from it.

**Decision points:**
- Trigger when `len(retrieved.KBSections) == 0` OR top score < `MinScore`.
- Caps: truncate `RawKB` to a safe token budget so we do not blow up context.
- Label: `=== UNAVAILABLE IN VECTOR INDEX ===` plus the full `RawKB`. This is intentionally different from normal retrieved context wording so logs show when fallback fired.

### 2. Surface fallback trigger in telemetry
**File:** `server/router/api/v1/agent/service.go`

Log the fallback activation (tenant, session, query, chunks found) so we can measure how often this happens and drive the indexing fix in task 4.

### 3. Fix stuck in_progress checkpoints on upload indexing
**File:** `server/router/api/v1/agent/handlers.go`
**Function:** `indexContentForRAG`

Currently, upload indexing writes `in_progress` then waits for `InsertWithCheckpoint` to finish. If the HTTP request context cancels early, the DB record is left with `status=in_progress` and partial batch count.

Reuse the existing reindex infrastructure (`ReindexTenantContentWithResume`) instead of running indexing inline:

- Remove the inline embedding/insert loop from `indexContentForRAG`.
- After saving source files in `importFiles`, call `ReindexTenantContentWithResume(ctx, tenantID, audienceType, false)`.
- The reindex path already has proper failed/stale-detection, detached checkpoint writes, and resume support.

Outcome: upload-triggered indexing is no longer gated on one HTTP request staying alive; partial failures can be resumed via the existing reindex endpoint.

### 4. Update the stale comment at Handlers reindex
**File:** `server/router/api/v1/agent/handlers.go`  
**Line:** `1086`

Change:
```go
// Note: Only indexes internal audience content. External audience is never indexed.
```
To reflect actual behavior (external IS indexed when explicitly requested).

### 5. Add an incomplete-index alert/check to chat path
**File:** `server/router/api/v1/agent/service.go`
**Function:** `ChatExternal` and/or `LoadConfig`

If the tenant is in RAG mode and its LanceDB chunk count for that audience is `0` or the reindex checkpoint is stale/failed, log a visible warning and enable the raw-KB fallback path from task 1 automatically. This makes the system self-healing: even if indexing is broken, the user gets a useful answer instead of a refusal.

---

## Validation
1. Re-upload the novel KB for tenant `rizal/external`.
2. Verify indexing completes to `268/268` chunks via `GET /api/v1/agent/rizal/reindex/status?audience_type=external`.
3. Ask: “what happened to Maria Clara at the end of the novel?”
4. Confirm the widget returns substantive answer content, not a context-declined refusal.
5. Confirm logs show either vector retrieval success or explicit fallback activation.

---

## Risks / Edge Cases
- Raw KB fallback increases system-prompt token count for large novels. Must enforce the truncation cap.
- Some tenants use full KB in system prompt intentionally; task 1 fallback only activates in RAG mode, so `long_context` tenants are unaffected.
- Reusing `ReindexTenantContentWithResume` from the upload path adds latency to the upload API response. This is acceptable; admin/upload callers already tolerate background work.