# Code Review: plan_ux.md Implementation (037-UX)

**Files reviewed:** 12 files across backend (Go) and frontend (TypeScript/React)
**Verdict: REWORK REQUIRED — 2 P0 bugs found**

---

## P0: Critical Bugs

### P0-1: `isRebuilding` never reset on skip_reason — button spins forever

**File:** `web/src/pages/AgentAdmin.tsx:304-327`
**Severity:** User-facing infinite spinner

`handleRebuildIndex` sets `setIsRebuilding(true)` at line 304, then enters a `switch (result.skip_reason)` block at line 310. Every skip case (`"long_context"`, `"no_source_files"`, `"pipeline_disabled"`) shows a toast but **never calls `setIsRebuilding(false)`**. The `setIsRebuilding(false)` call exists only in the `else` branch (line 325), which runs when `result.success === false`.

Since `loading={isRebuilding || status === "in_progress"}` (line 1234-1237), the button remains in a spinning/loading state indefinitely after a synchronously skipped reindex.

**Reproduction:**
1. Open evpn tenant (long_context mode)
2. Click "Rebuild Index"
3. ℹ️ toast appears ("Skipped — long_context mode")
4. Button continues spinning forever

**Fix:** Add `setIsRebuilding(false)` after the switch statement in the success branch:

```typescript
if (result.success) {
  switch (result.skip_reason) {
    case "long_context":
      toast(t("agent-admin.reindex-skipped-long-context"), { icon: "ℹ️" });
      break;
    case "no_source_files":
      toast(t("agent-admin.reindex-skipped-no-source"), { icon: "⚠️" });
      break;
    case "pipeline_disabled":
      toast(t("agent-admin.reindex-skipped-pipeline"), { icon: "⚠️" });
      break;
    default:
      toast.success(t("agent-admin.rebuild-index-started"));
  }
  setIsRebuilding(false);  // ← MUST ADD: reset loading state for all skip cases
} else {
  setIsRebuilding(false);
  toast.error(result.error || t("agent-admin.rebuild-index-failed"));
}
```

---

### P0-2: `forceRAG` overrides tenant's explicit `long_context` mode — no index, no answer

**File:** `server/router/api/v1/agent/service.go:2167-2181`
**Severity:** Agent cannot answer KB questions for long_context tenants without structured annotations

The chat decision logic:

```go
forceRAG := !config.HasStructuredContent && s.UseRAGPipeline()
```

When a tenant:
- Has `RetrievalMode = "long_context"` (explicitly configured)
- Has a KB file with unannotated/markdown content (no `@service`, `@faq`, etc. annotations)
- Has RAG pipeline enabled at the server level

**Result:** `HasStructuredContent` is `false` → `forceRAG = true` → RAG mode is forced, **ignoring the tenant's explicit `RetrievalMode`**.

Since the long_context tenant's content was never indexed into the vector DB (reindex was correctly skipped by the handler), RAG retrieval returns nothing. The LLM gets a prompt with constraints like "Only discuss information in RETRIEVED CONTEXT below" but no actual content — producing "the retrieved context does not provide a definition" style responses.

**Root cause chain:**
1. Tenant KB has markdown content without `@faq`/`@service` annotations → `HasStructuredContent = false`
2. `forceRAG = true` bypasses the `RetrievalMode` check (lines 2175-2180 never execute)
3. RAG query returns empty results (no index exists)
4. Fallback triggers but prompt structure is confusing — constraint says "check RETRIEVED CONTEXT below" but `hasRetrievedContent = false`, so no RETRIEVED CONTEXT section is rendered (line 3197)
5. `ragFallbackTokenBudget = 6000` tokens may truncate the raw KB, potentially cutting off the relevant content

**Fix:** The `forceRAG` check must respect an explicit tenant-level `RetrievalMode = "long_context"`:

```go
if s.UseRAGPipeline() {
    tenantConfig, _ := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &config.TenantID})
    if tenantConfig != nil && tenantConfig.RetrievalMode == "rag" {
        useRAG = true
    } else if (tenantConfig == nil || tenantConfig.RetrievalMode == "") && !config.HasStructuredContent {
        // No explicit tenant preference: fall back to RAG for unstructured content
        useRAG = true
    }
}
```

---

## P1: High Severity

### P1-1: `TestCreateFailedCheckpointWritesToStore` tests struct construction, not the function

**File:** `server/router/api/v1/agent/service_reindex_test.go:43-65`

The test constructs a `store.ReindexCheckpoint` literal and checks field values — it **never calls `createFailedCheckpoint`**. The function name suggests it tests the helper, but it only validates a manually-constructed struct. A future refactor could rename or remove `createFailedCheckpoint` and this test would still pass.

```go
// This test constructs a struct manually — it does NOT test
// createFailedCheckpoint(). The function signature could be deleted
// and this test would still pass.
cp := &store.ReindexCheckpoint{
    TenantID:     42,
    Audience:     "internal",
    Status:       "failed",
    ErrorMessage: "RAG pipeline not initialized",
}
// ... field checks on the literal, not on the function's output
```

**Fix:** Either rename the test to `TestReindexCheckpointStructFields` to match what it actually tests, or rewrite to actually call `createFailedCheckpoint` with a mock store that captures the checkpoint.

---

### P1-2: RAG fallback prompt is contradictory when RAG returns nothing

**File:** `server/router/api/v1/agent/service.go:3154-3158, 3197-3203, 2988-2996`

When `withFallback = true` but `hasRetrievedContent = false` (RAG returned nothing), the prompt says:

```
- If the RETRIEVED CONTEXT below does not contain an answer, use the RAW KNOWLEDGE BASE FALLBACK section.
```

But the RETRIEVED CONTEXT section is **never rendered** (skipped at line 3197 because `hasRetrievedContent` is false). The LLM receives a directive to check a section that doesn't exist, followed by the RAW KNOWLEDGE BASE FALLBACK. This ambiguity can cause the LLM to ignore the fallback content.

**Fix:** When `hasRetrievedContent` is false, skip the "if RETRIEVED CONTEXT below" instruction entirely:

```go
if hasRetrievedContent {
    if withFallback {
        sb.WriteString("=== RETRIEVED CONTEXT ===\n\n")
        sb.WriteString("Note: If RETRIEVED CONTEXT does not contain the answer, use RAW KNOWLEDGE BASE FALLBACK at the end.\n\n")
    } else {
        sb.WriteString("=== RETRIEVED CONTEXT (Use ONLY this information) ===\n\n")
    }
    // ... render sections ...
} else if withFallback {
    sb.WriteString("=== NO RETRIEVED CONTEXT AVAILABLE ===\n")
    sb.WriteString("Use the RAW KNOWLEDGE BASE FALLBACK section at the end of this prompt to answer the user.\n\n")
}
```

---

### P1-3: `IsRAGEnabled()` vs `UseRAGPipeline()` — inconsistent implementations

**File:** `server/router/api/v1/agent/service.go:2916-2923` and handler pre-checks

The handler pre-check uses `h.service.IsRAGEnabled()` (checks some config), but the chat flow uses `s.UseRAGPipeline()` (checks `s.vectorDB != nil && !isNoOp`). These are different checks. `IsRAGEnabled()` could return true while `UseRAGPipeline()` returns false, or vice versa.

This means the reindex pre-check might block with `SkipReasonPipelineDisabled` while the chat flow still attempts RAG (with nil vectorDB), or reindex passes but chat falls back to long context (inconsistent).

**Fix:** Unify the check. At minimum, `IsRAGEnabled()` should delegate to `UseRAGPipeline()`.

---

## P2: Medium Severity

| # | File | Issue | Fix |
|---|------|-------|-----|
| 1 | `AgentAdmin.tsx:320-321` | `default` case (goroutine) also doesn't call `setIsRebuilding(false)` — relies on polling. No timeout if goroutine hangs | Add a timeout effect that resets `isRebuilding` after N seconds with no status update |
| 2 | `handlers.go:1186` | Long_context skip returns hardcoded `"audience":"internal"`, ignoring query param `audience_type` | Use the requested audience type |
| 3 | `service.go:586-593` | Long_context skip creates no checkpoint → first page load has no status → idle card not rendered | Ensure polling or initial fetch populates status even without a checkpoint |
| 4 | `handlers.go:1205-1214` | Whitespace-only files bypass `totalContentLen == 0` check (whitespace has bytes) | Add a minimum meaningful content length check |
| 5 | `service.go:33` | `ragMinScore = 0.25` is very low for cosine similarity (range 0-1) | Evaluate and adjust threshold; 0.5 is a more common minimum |

---

## P3: Low Severity

| # | File | Issue |
|---|------|-------|
| 1 | `handlers.go:1205` | `countErr` is shadowed — the error path falls through to goroutine but may confuse future readers |
| 2 | `handlers_test.go:48-61` | `TestSkipReasonConstants` validates const values by string equality — tests the compiler, not the logic |
| 3 | `store/db/mysql/agent.go` | MySQL `CountTenantSourceFiles` returns `errNotImplemented` — pre-check silently falls through to goroutine for MySQL |
| 4 | `service.go:2989` | `truncateToTokenBudget` uses byte estimation (`maxTokens * 4`) that can split multi-byte UTF-8 characters, producing broken text |

---

## Bug Explanations

### Why Rebuild Index button keeps circling for evpn tenant

**Root cause:** P0-1. The `handleRebuildIndex` function never resets `isRebuilding` when the response contains a `skip_reason`. The long_context skip returns success with `skip_reason: "long_context"`, but `setIsRebuilding(false)` is only called on the `result.success === false` branch.

### Why agent can't answer "What is ISP throttling" despite long_context mode

**Root cause:** P0-2. The `forceRAG` check (`!config.HasStructuredContent && s.UseRAGPipeline()`) overrides the tenant's explicit `RetrievalMode = "long_context"`. When the evpn KB file has markdown content without structured annotations (`@faq`, `@service` tags), `HasStructuredContent` is false, forcing RAG mode. Since no RAG index was built (reindex was correctly skipped), the vector DB returns nothing, and the agent has no content to answer from.

---

## Summary

| Priority | Count | Key Issues |
|----------|-------|------------|
| P0 | 2 | `isRebuilding` spinner loop; `forceRAG` ignores long_context config |
| P1 | 3 | Fake test; contradictory RAG fallback prompt; inconsistent pipeline check |
| P2 | 5 | No reset timeout; hardcoded audience; stale status on idle; empty content bypass; low minScore |
| P3 | 4 | Shadowed variable; trivial const test; MySQL stub; UTF-8 truncation |

**Verdict: REWORK required.** P0-1 and P0-2 must be fixed before merge. P1 and P2 should be addressed before or shortly after merge.
