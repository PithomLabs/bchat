# Plan 037-UX — User Experience for By-Design, Gaps, and Bug Communication

**Date:** 2026-07-14
**Status:** Awaiting approval
**Scope:** Backend response enrichment + Frontend messaging + Status transparency

---

## Problem Statement

When a user clicks "Rebuild Index" for a tenant, the system may silently skip the operation,
fail before producing any status, or complete with zero visible feedback. The user has no way
to distinguish between:

- **By-design behavior** (long_context mode doesn't need RAG)
- **Known limitations** (no source files yet, pipeline disabled)
- **Bugs** (goroutine crashed, checkpoint not created)

**Concrete example (evpn tenant):**
1. User clicks "Rebuild Index"
2. Frontend shows `toast.success("Rebuild index started")` — always, regardless of outcome
3. Polling returns `status: "idle"` — frontend hides the progress card
4. User sees nothing — thinks it's broken
5. Reality: tenant is in `long_context` mode (363K tokens, should be RAG) — RAG was never needed

---

## Root Cause Analysis

| Layer | Gap | Impact |
|-------|-----|--------|
| Backend | Long_context skip returns same HTTP 200 shape as normal start | Frontend can't distinguish skip from start |
| Backend | Goroutine failures before checkpoint (G1-G4) produce no status | Frontend sees `idle`, user sees nothing |
| Backend | `content_tokens = 0` for evpn — mode never properly calculated | Tenant stuck in wrong mode |
| Frontend | `handleRebuildIndex` always shows same toast | Misleading feedback for all skip/failure cases |
| Frontend | Progress card hidden when `status === "idle"` | No fallback messaging |
| Frontend | No display of retrieval mode | User doesn't know what mode the tenant is in |

---

## Design Principles

1. **Inform, don't alarm** — By-design behavior gets an info toast, not an error
2. **Explain the "why"** — Every skipped/failed operation gets a reason the user can act on
3. **Progress card is always informative** — Even when idle, show what mode the tenant is in
4. **Backend owns the message** — Frontend renders what the backend tells it; no hardcoded logic

---

## Three Categories of Communication

### Category 1: By-Design Behavior
**Examples:** long_context skip, resume from checkpoint, already up-to-date
**UX:** Info toast (ℹ️) + inline info message in progress card

### Category 2: Known Limitations / Gaps
**Examples:** No source files, pipeline disabled, zero chunks from empty content
**UX:** Warning toast (⚠️) + inline warning in progress card

### Category 3: Bugs / Failures
**Examples:** Goroutine crash, embedding provider misconfigured, checkpoint persist failed
**UX:** Error toast (❌) + red failed status in progress card with error details

---

## Changes

### Change 1: Backend — Enrich POST reindex response with skip_reason

**File:** `server/router/api/v1/agent/handlers.go`
**Location:** Lines 1172-1181 (long_context skip)

**Current:**
```go
return c.JSON(http.StatusOK, map[string]interface{}{
    "success":  true,
    "message":  "Skipped - tenant uses long_context mode (RAG indexing not needed)",
    "chunks":   0,
    "audience": "internal",
})
```

**Proposed:**
```go
return c.JSON(http.StatusOK, map[string]interface{}{
    "success":     true,
    "chunks":      0,
    "audience":    "internal",
    "skip_reason": "long_context",
    "message":     "Skipped — this tenant uses long_context mode. RAG indexing is not needed.",
})
```

**Why:** The `skip_reason` field lets the frontend branch on the specific reason, showing
category-appropriate messaging instead of a generic "started" toast.

---

### Change 2: Backend — Create failed checkpoint for early goroutine failures

**File:** `server/router/api/v1/agent/service.go`
**Location:** Lines 711-718 (pre-checkpoint error paths)

**Current:** Returns `(0, error)` with no checkpoint — frontend sees `idle`.

**Proposed:** Create a `failed` checkpoint before returning:

```go
if s.vectorDB == nil || s.chunker == nil {
    s.createFailedCheckpoint(ctx, tenantID, audienceType, "RAG pipeline not initialized")
    return 0, fmt.Errorf("RAG pipeline not initialized")
}

if _, isNoOp := s.vectorDB.(*NoOpVectorDB); isNoOp {
    s.createFailedCheckpoint(ctx, tenantID, audienceType, "RAG pipeline disabled (using NoOpVectorDB)")
    return 0, fmt.Errorf("RAG pipeline disabled (using NoOpVectorDB)")
}
```

**Add helper:**
```go
func (s *Service) createFailedCheckpoint(ctx context.Context, tenantID int32, audience, msg string) {
    cp := &store.ReindexCheckpoint{
        TenantID:     tenantID,
        Audience:     audience,
        Status:       "failed",
        ErrorMessage: msg,
        StartedAt:    time.Now(),
    }
    checkpointCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if _, err := s.store.UpsertReindexCheckpoint(checkpointCtx, cp); err != nil {
        slog.Warn("failed to create failure checkpoint", "error", err)
    }
}
```

**Why:** The status endpoint always has something to report. No more silent `idle` for
known failure modes. The frontend can poll and show the failed status with the error message.

---

### Change 3: Backend — Enrich status response with retrieval_mode

**File:** `server/router/api/v1/agent/service.go`
**Location:** Lines 517-528 (ReindexStatus struct)

**Current:**
```go
type ReindexStatus struct {
    Status          string `json:"status"`
    CurrentBatch    int    `json:"current_batch,omitempty"`
    TotalBatches    int    `json:"total_batches,omitempty"`
    ProcessedChunks int    `json:"processed_chunks,omitempty"`
    TotalChunks     int    `json:"total_chunks,omitempty"`
    ErrorMessage    string `json:"error,omitempty"`
    LastMessage     string `json:"last_message,omitempty"`
    ErrorBatch      *int   `json:"error_batch,omitempty"`
    CanResume       bool   `json:"can_resume"`
    UpdatedAt       string `json:"updated_at,omitempty"`
}
```

**Proposed:** Add `retrieval_mode` field:
```go
type ReindexStatus struct {
    Status          string `json:"status"`
    CurrentBatch    int    `json:"current_batch,omitempty"`
    TotalBatches    int    `json:"total_batches,omitempty"`
    ProcessedChunks int    `json:"processed_chunks,omitempty"`
    TotalChunks     int    `json:"total_chunks,omitempty"`
    ErrorMessage    string `json:"error,omitempty"`
    LastMessage     string `json:"last_message,omitempty"`
    ErrorBatch      *int   `json:"error_batch,omitempty"`
    CanResume       bool   `json:"can_resume"`
    RetrievalMode   string `json:"retrieval_mode,omitempty"`
    UpdatedAt       string `json:"updated_at,omitempty"`
}
```

**Populate in GetReindexStatus:**
```go
// After resolving checkpoints (before return)...
if tc, err := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &tenantID}); err == nil && tc != nil {
    status.RetrievalMode = tc.RetrievalMode
}
```

**Why:** The frontend needs to know the retrieval mode to show context-appropriate idle
messaging (e.g., "long_context mode — no index needed" vs "RAG mode — upload files first").

---

### Change 4: Frontend store — Return skip_reason and retrieval_mode

**File:** `web/src/store/v2/agentAdmin.ts`

**4a. Update ReindexStatus interface (line 252):**
```typescript
export interface ReindexStatus {
  status: "idle" | "in_progress" | "completed" | "failed";
  current_batch: number;
  total_batches: number;
  processed_chunks: number;
  total_chunks: number;
  error?: string;
  last_message?: string;
  can_resume: boolean;
  retrieval_mode?: string;  // NEW: "long_context", "rag", or ""
}
```

**4b. Update reindexTenant return type (line 963):**
```typescript
const reindexTenant = async (
  slug: string,
  audienceType: string = "all",
): Promise<{ success: boolean; chunks?: number; skip_reason?: string; error?: string }> => {
  ...
  return {
    success: true,
    chunks: response.data.chunks,
    skip_reason: response.data.skip_reason,  // NEW
  };
};
```

---

### Change 5: Frontend — Handle skip_reason in toast

**File:** `web/src/pages/AgentAdmin.tsx`
**Location:** Lines 302-316 (handleRebuildIndex)

**Current:**
```typescript
const handleRebuildIndex = async () => {
  if (!selectedTenant) return;
  setIsRebuilding(true);
  const result = await agentAdminStore.reindexTenant(
    selectedTenant.tenant.slug,
    reindexAudience,
  );
  if (result.success) {
    toast.success(t("agent-admin.rebuild-index-started"));
  } else {
    setIsRebuilding(false);
    toast.error(result.error || t("agent-admin.rebuild-index-failed"));
  }
};
```

**Proposed:**
```typescript
const handleRebuildIndex = async () => {
  if (!selectedTenant) return;
  setIsRebuilding(true);
  const result = await agentAdminStore.reindexTenant(
    selectedTenant.tenant.slug,
    reindexAudience,
  );
  if (result.success) {
    if (result.skip_reason === "long_context") {
      toast(t("agent-admin.reindex-skipped-long-context"), { icon: "ℹ️" });
    } else {
      toast.success(t("agent-admin.rebuild-index-started"));
    }
  } else {
    setIsRebuilding(false);
    toast.error(result.error || t("agent-admin.rebuild-index-failed"));
  }
};
```

**Why:** Category-appropriate toast. long_context skip gets an info toast, not a success toast.

---

### Change 6: Frontend — Show mode-aware idle messaging

**File:** `web/src/pages/AgentAdmin.tsx`
**Location:** Lines 1234-1294 (progress card, after the existing status display)

**Proposed:** Add idle-state messaging AFTER the existing `{status !== "idle" && (...)}` block:

```tsx
{/* Idle state: show mode-aware messaging */}
{agentAdminStore.state.reindexStatus?.status === "idle" && (
  <div className="bg-white dark:bg-zinc-800 rounded-lg p-3 border border-blue-100 dark:border-blue-900/40">
    {agentAdminStore.state.reindexStatus?.retrieval_mode === "long_context" ? (
      <div className="text-xs text-blue-600 dark:text-blue-400">
        ℹ️ {t("agent-admin.reindex-idle-long-context")}
      </div>
    ) : agentAdminStore.state.reindexStatus?.retrieval_mode === "rag" ? (
      <div className="text-xs text-amber-600 dark:text-amber-400">
        ⚠️ {t("agent-admin.reindex-idle-no-index")}
      </div>
    ) : null}
  </div>
)}
```

**Why:** The progress card is always informative. Even when idle, the user knows what mode
the tenant is in and what action (if any) is needed.

---

### Change 7: Frontend — Add translation keys

**File:** `web/src/locales/en.json`
**Location:** After the existing `rebuild-index-hint` line

**Proposed:**
```json
"reindex-skipped-long-context": "Skipped — this tenant uses long_context mode, RAG indexing is not needed",
"reindex-idle-long-context": "This tenant uses long_context mode. The full document is sent to the LLM — RAG indexing is not needed.",
"reindex-idle-no-index": "No RAG index found. Upload KB/Policy files and click Rebuild Index to make content searchable.",
"reindex-failed-pipeline": "RAG pipeline is not available. Check server configuration (RAG_PIPELINE_ENABLED).",
"reindex-failed-no-source": "No source files found. Upload KB or Policy files first."
```

---

## User Experience Matrix

| Scenario | HTTP Status | Toast | Progress Card | Root Cause |
|----------|-------------|-------|---------------|------------|
| long_context skip | 200 OK | ℹ️ "Skipped — long_context mode" | Blue info: "Full document sent to LLM" | By design |
| Normal reindex start | 202 Accepted | ✅ "Reindexing started" | Progress bar + chunk count | By design |
| No source files | 202 Accepted | ✅ "Reindexing started" | Goroutine creates failed checkpoint → red: "No source files" | Limitation |
| Pipeline disabled | 202 Accepted | ✅ "Reindexing started" | Goroutine creates failed checkpoint → red: "RAG pipeline not available" | Limitation |
| Zero chunks from chunking | 202 Accepted | ✅ "Reindexing started" | Checkpoint completed with 0 chunks → green: "0 chunks indexed" | Limitation |
| Goroutine crashes early | 202 Accepted | ✅ "Reindexing started" | Failed checkpoint → red: error message | Bug |
| Embedding provider down | 202 Accepted | ✅ "Reindexing started" | Failed checkpoint → red: "embedding provider unavailable" | Bug |
| Idle, RAG mode, no index | — | — | Amber: "No index found. Upload files first." | Gap |
| Idle, long_context | — | — | Blue info: "Full document mode — no index needed" | By design |

---

## Files to Modify

| File | Changes | Lines Affected |
|------|---------|----------------|
| `server/router/api/v1/agent/handlers.go` | Add `skip_reason` to long_context response | ~1174-1181 |
| `server/router/api/v1/agent/service.go` | Add `createFailedCheckpoint` helper, call for G1-G4, add `retrieval_mode` to `ReindexStatus`, populate in `GetReindexStatus` | ~517-528, ~711-718, ~585-587 |
| `web/src/pages/AgentAdmin.tsx` | Handle `skip_reason` in toast, add idle-state messaging | ~302-316, ~1234-1294 |
| `web/src/store/v2/agentAdmin.ts` | Add `retrieval_mode` to `ReindexStatus`, return `skip_reason` from `reindexTenant` | ~252-261, ~963-985 |
| `web/src/locales/en.json` | Add new translation keys | After line 181 |

---

## Verification

### Step 1: Compile and test
```bash
go build ./...
go test ./server/router/api/v1/agent/... -count=1
```

### Step 2: Manual test — long_context tenant
```bash
task run:rag
# Upload small KB (< 30K tokens)
# Click Rebuild Index
# Expect: ℹ️ toast "Skipped — long_context mode"
# Expect: blue info card in progress area
```

### Step 3: Manual test — RAG tenant
```bash
# Upload large KB (>= 30K tokens)
# Click Rebuild Index
# Expect: ✅ toast "Reindexing started"
# Expect: progress bar → completed
```

### Step 4: Manual test — no source files
```bash
# Create tenant with no files
# Click Rebuild Index
# Expect: ✅ toast "Reindexing started"
# Expect: polling shows failed checkpoint → red error message
```

### Step 5: Manual test — idle state messaging
```bash
# For long_context tenant: expect blue info when idle
# For RAG tenant with no index: expect amber warning when idle
```

---

## Risks

| Risk | Impact | Mitigation |
|------|--------|-----------|
| `createFailedCheckpoint` race with concurrent reindex | Low — uses 5s timeout context, idempotent upsert | Existing per-batch mutex pattern |
| Frontend breaks if `skip_reason` field missing | Low — optional field, frontend checks for it | Optional chaining `result.skip_reason` |
| Translation keys missing in non-English locales | Medium — shows key instead of message | Add keys to all locale files, or fallback to English |
| `retrieval_mode` not populated for old tenants | Low — shows empty string, frontend handles gracefully | Default to showing nothing when mode is unknown |

---

## Rollback

Revert the 5 file changes. No data model changes. No migrations needed.
