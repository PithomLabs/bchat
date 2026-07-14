# Plan 037-UX — User Experience for By-Design, Gaps, and Bug Communication

**Date:** 2026-07-14
**Status:** Ready for implementation (incorporated review_ux2.md findings)
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

## Out of Scope (Review Nit #1)

The root cause `content_tokens = 0` (mode never properly calculated) is **out of scope for this UX plan**. The mode calculation fix belongs in `plan_fix.md` (plan2). Without that fix, the UX improvements here are partially undermined — if `retrieval_mode` is wrong (e.g., still "long_context" when it should be "rag"), the mode-aware idle messaging will be misleading rather than informative. Cross-reference: `bugs/037/plan_fix.md`.

---

## Design Principles

1. **Inform, don't alarm** — By-design behavior gets an info toast, not an error
2. **Explain the "why"** — Every skipped/failed operation gets a reason the user can act on
3. **Don't pretend work started when it hasn't** — Detect pre-checkpoint failures synchronously
4. **Progress card is always informative** — Even when idle, show what mode the tenant is in
5. **Backend owns the message** — Frontend renders what the backend tells it; no hardcoded logic

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

## Skip Reason Constants

Backend defines typed constants:

```go
const (
    SkipReasonNone             = ""
    SkipReasonLongContext      = "long_context"
    SkipReasonNoSourceFiles    = "no_source_files"
    SkipReasonPipelineDisabled = "pipeline_disabled"
)
```

Frontend defines a union type:

```typescript
type SkipReason = "long_context" | "no_source_files" | "pipeline_disabled";
```

---

## Changes

### Change 1: Backend — Pre-check failures in handler (synchronous, before goroutine)

**File:** `server/router/api/v1/agent/handlers.go`
**Location:** After line 1181 (long_context skip), before goroutine spawn

**Why:** Don't pretend work started when it hasn't. Detect failures synchronously and return
them as `skip_reason` responses with appropriate HTTP status and toast.

**Define constants** at the top of `handlers.go`:
```go
const (
    SkipReasonNone             = ""
    SkipReasonLongContext      = "long_context"
    SkipReasonNoSourceFiles    = "no_source_files"
    SkipReasonPipelineDisabled = "pipeline_disabled"
)
```

**Add three synchronous pre-checks after the long_context check:**

```go
// Pre-check: pipeline readiness (detectable synchronously)
if !h.service.IsRAGEnabled() {
    return c.JSON(http.StatusOK, map[string]interface{}{
        "success":     true,
        "chunks":      0,
        "skip_reason": SkipReasonPipelineDisabled,
        "message":     "RAG pipeline is not available. Check server configuration.",
    })
}

// Pre-check: source files exist and have content (review nits #2 + #3)
// Uses CountTenantSourceFiles — lightweight COUNT(*) + SUM(LENGTH(content))
// instead of ListAgentSourceFiles which fetches full content blobs.
count, totalContentLen, err := h.store.CountTenantSourceFiles(ctx, tenant.ID)
if err != nil {
    slog.Warn("failed to count source files for pre-check", "tenantID", tenant.ID, "error", err)
    // Don't fail the whole request — let the goroutine handle it
} else if count == 0 || totalContentLen == 0 {
    return c.JSON(http.StatusOK, map[string]interface{}{
        "success":     true,
        "chunks":      0,
        "skip_reason": SkipReasonNoSourceFiles,
        "message":     "No source files found. Upload KB or Policy files first.",
    })
}
```

**Also update the long_context skip to use the constant:**
```go
"skip_reason": SkipReasonLongContext,
```

**Note:** The goroutine still handles the "zero chunks from chunking" case (too expensive to
check synchronously). The `createFailedCheckpoint` helper in Change 2 covers goroutine-level
failures that occur after the goroutine starts but before checkpoint creation.

---

### Change 1b: Backend — Add lightweight `CountTenantSourceFiles` store method

**File:** `store/driver.go` — Add to driver interface:
```go
CountTenantSourceFiles(ctx context.Context, tenantID int32) (count int, totalContentLen int, err error)
```

**File:** `store/agent.go` — Add delegating method:
```go
func (s *Store) CountTenantSourceFiles(ctx context.Context, tenantID int32) (int, int, error) {
    return s.driver.CountTenantSourceFiles(ctx, tenantID)
}
```

**File:** `store/db/sqlite/agent.go` — Add implementation:
```go
func (d *DB) CountTenantSourceFiles(ctx context.Context, tenantID int32) (count int, totalContentLen int, err error) {
    err = d.db.QueryRowContext(ctx,
        `SELECT COUNT(*), COALESCE(SUM(LENGTH(content)), 0)
         FROM agent_source_files
         WHERE tenant_id = ?`, tenantID,
    ).Scan(&count, &totalContentLen)
    return
}
```

**File:** `store/db/postgres/agent.go` — Add implementation:
```go
func (d *DB) CountTenantSourceFiles(ctx context.Context, tenantID int32) (count int, totalContentLen int, err error) {
    err = d.db.QueryRowContext(ctx,
        `SELECT COUNT(*), COALESCE(SUM(LENGTH(content)), 0)
         FROM agent_source_files
         WHERE tenant_id = $1`, tenantID,
    ).Scan(&count, &totalContentLen)
    return
}
```

**File:** `store/db/mysql/agent.go` — Add implementation:
```go
func (d *DB) CountTenantSourceFiles(ctx context.Context, tenantID int32) (count int, totalContentLen int, err error) {
    err = d.db.QueryRowContext(ctx,
        `SELECT COUNT(*), COALESCE(SUM(LENGTH(content)), 0)
         FROM agent_source_files
         WHERE tenant_id = ?`, tenantID,
    ).Scan(&count, &totalContentLen)
    return
}
```

**Why:** `ListAgentSourceFiles` with `LatestOnly: true` still fetches `content` blobs
(confirmed: both SQLite and Postgres SELECT include `content`). For the pre-check we only
need count + total length. A single `COUNT(*) + SUM(LENGTH(content))` query is ~100x
cheaper than fetching full records.

---

### Change 2: Backend — Create failed checkpoint for goroutine-level early failures

**File:** `server/router/api/v1/agent/service.go`
**Location:** Lines 711-718 (pre-checkpoint error paths inside ReindexTenantContentWithResume)

**Why:** Even with synchronous pre-checks, some failures only manifest inside the goroutine
(e.g., vectorDB becomes nil between handler check and goroutine execution, or chunking
produces zero chunks). These need a checkpoint so the status endpoint can surface them.

**Add helper (with documented context rationale — review nit #5):**
```go
func (s *Service) createFailedCheckpoint(ctx context.Context, tenantID int32, audience, msg string) {
    cp := &store.ReindexCheckpoint{
        TenantID:     tenantID,
        Audience:     audience,
        Status:       "failed",
        ErrorMessage: msg,
        StartedAt:    time.Now(),
    }
    // Detached context: checkpoint write must persist even if the
    // parent request context is cancelled (goroutine may abort).
    // 5s timeout prevents a slow DB write from blocking indefinitely.
    checkpointCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if _, err := s.store.UpsertReindexCheckpoint(checkpointCtx, cp); err != nil {
        slog.Warn("failed to create failure checkpoint", "error", err)
    }
}
```

**Call for existing pre-checkpoint paths:**
```go
if s.vectorDB == nil || s.chunker == nil {
    s.createFailedCheckpoint(ctx, tenantID, audienceType, "RAG pipeline not initialized")
    return 0, fmt.Errorf("RAG pipeline not initialized")
}

if _, isNoOp := s.vectorDB.(*NoOpVectorDB); isNoOp {
    s.createFailedCheckpoint(ctx, tenantID, audienceType, "RAG pipeline disabled")
    return 0, fmt.Errorf("RAG pipeline disabled (using NoOpVectorDB)")
}
```

---

### Change 3: Backend — Enrich status response with retrieval_mode

**File:** `server/router/api/v1/agent/service.go`
**Location:** Lines 517-528 (ReindexStatus struct)

**Add field to struct:**
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
    RetrievalMode   string `json:"retrieval_mode,omitempty"` // NEW
    UpdatedAt       string `json:"updated_at,omitempty"`
}
```

**Populate in GetReindexStatus (after resolving checkpoints, before return):**
```go
if tc, err := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &tenantID}); err != nil {
    slog.Warn("failed to get tenant config for status", "tenantID", tenantID, "error", err)
    // Don't fail the whole request — just leave retrieval_mode empty
} else if tc != nil {
    status.RetrievalMode = tc.RetrievalMode
}
```

---

### Change 4: Frontend store — Return skip_reason and retrieval_mode

**File:** `web/src/store/v2/agentAdmin.ts`

**4a. Add SkipReason type (near line 252):**
```typescript
export type SkipReason = "long_context" | "no_source_files" | "pipeline_disabled";
```

**4b. Update ReindexStatus interface (line 252):**
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

**4c. Update reindexTenant return type (line 963):**
```typescript
const reindexTenant = async (
  slug: string,
  audienceType: string = "all",
): Promise<{ success: boolean; chunks?: number; skip_reason?: SkipReason; error?: string }> => {
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

```typescript
const handleRebuildIndex = async () => {
  if (!selectedTenant) return;
  setIsRebuilding(true);
  const result = await agentAdminStore.reindexTenant(
    selectedTenant.tenant.slug,
    reindexAudience,
  );
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
  } else {
    setIsRebuilding(false);
    toast.error(result.error || t("agent-admin.rebuild-index-failed"));
  }
};
```

---

### Change 6: Frontend — Show mode-aware idle messaging

**File:** `web/src/pages/AgentAdmin.tsx`
**Location:** Lines 1234-1294 (progress card, after the existing status display)

Add idle-state messaging AFTER the existing `{status !== "idle" && (...)}` block:

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

---

### Change 7: Frontend — Add translation keys

**File:** `web/src/locales/en.json`
**Location:** After the existing `rebuild-index-hint` line

```json
"reindex-skipped-long-context": "Skipped — this tenant uses long_context mode, RAG indexing is not needed",
"reindex-skipped-no-source": "No files to index. Upload KB or Policy files first.",
"reindex-skipped-pipeline": "RAG pipeline is not configured. Check server settings.",
"reindex-idle-long-context": "This tenant uses long_context mode. The full document is sent to the LLM — RAG indexing is not needed.",
"reindex-idle-no-index": "No RAG index found. Upload KB/Policy files and click Rebuild Index to make content searchable."
```

---

### Change 8: Tests — Unit tests for pre-check and createFailedCheckpoint

**File:** `server/router/api/v1/agent/handlers_test.go`

Add test cases:
- `TestHandleReindex_LongContextSkip`
- `TestHandleReindex_NoSourceFiles`
- `TestHandleReindex_PipelineDisabled`

**File:** `server/router/api/v1/agent/service_reindex_test.go`

Add test cases:
- `TestCreateFailedCheckpoint`
- `TestReindexPipelineDisabled_CreatesCheckpoint`

---

## Files to Modify

| File | Changes |
|------|---------|
| `handlers.go` | Add skip_reason constants, 3 synchronous pre-checks, use constants in long_context skip |
| `service.go` | Add `createFailedCheckpoint` helper, call for G1-G2, add `retrieval_mode` to `ReindexStatus`, populate in `GetReindexStatus` |
| `store/driver.go` | Add `CountTenantSourceFiles` interface method |
| `store/agent.go` | Add `CountTenantSourceFiles` delegating method |
| `store/db/sqlite/agent.go` | Add `CountTenantSourceFiles` implementation |
| `store/db/postgres/agent.go` | Add `CountTenantSourceFiles` implementation |
| `store/db/mysql/agent.go` | Add `CountTenantSourceFiles` implementation |
| `AgentAdmin.tsx` | Handle skip_reason in toast (switch), add idle-state messaging |
| `agentAdmin.ts` | Add SkipReason type, add `retrieval_mode` to ReindexStatus, return skip_reason |
| `en.json` | Add 5 new translation keys |
| `handlers_test.go` | Add 3 pre-check test cases |
| `service_reindex_test.go` | Add 2 checkpoint test cases |

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
# Expect: ⚠️ toast "No files to index" (synchronous, no goroutine)
```

### Step 4b: Manual test — pipeline disabled
```bash
# Disable RAG: set RAG_PIPELINE_ENABLED=false in .env, restart
# Click Rebuild Index
# Expect: ⚠️ toast "Pipeline not configured"
# Re-enable RAG after test
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
| `createFailedCheckpoint` race with concurrent reindex | Low — detached 5s timeout, idempotent upsert | Existing per-batch mutex pattern |
| Frontend breaks if `skip_reason` field missing | Low — optional field, switch handles undefined | `default` case in switch |
| Translation keys missing in non-English locales | Medium — shows key instead of message | Add keys to all locale files, or fallback to English |
| `retrieval_mode` not populated for old tenants | Low — shows empty string, frontend handles gracefully | Default to showing nothing when mode is unknown |
| `content_tokens = 0` not fixed here | Medium — mode-aware UX is partially undermined | Cross-reference `plan_fix.md` for mode calculation fix |

---

## Rollback

Revert all file changes. No data model changes. No migrations needed.
