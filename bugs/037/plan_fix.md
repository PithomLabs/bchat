# Plan 037 — Fix: evpn Zero Chunks Investigation

**Date:** 2026-07-14

## Problem Statement
evpn tenant reindex produces zero chunks. No logs visible. DB shows no checkpoint. Three silent failure paths exist in the reindex pipeline.

## Goals
1. Fix silent failure paths with proper logging
2. Determine why tenant 13 goroutine never reached checkpoint creation
3. Clean up stuck checkpoint for tenant 12

## Non-Goals
- Changing the reindex architecture (already done in plan037)
- Changing config clamping or throughput projection (already done in plan2)

---

## Changes

### Change 1: Add Logging to Goroutine Launch

**File:** `handlers.go`
**Location:** Lines 1190-1213

**Current:**
```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            slog.Error("reindex panic recovered", "error", r)
        }
    }()
    h.service.ReindexTenantContentWithResume(context.Background(), tenant.Slug, "")
}()
```

**Target:**
```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            slog.Error("reindex panic recovered", "error", r)
        }
    }()
    slog.Info("reindex goroutine started", "slug", tenant.Slug, "tenant_id", tenant.ID)
    h.service.ReindexTenantContentWithResume(context.Background(), tenant.Slug, "")
    slog.Info("reindex goroutine completed", "slug", tenant.Slug, "tenant_id", tenant.ID)
}()
```

---

### Change 2: Add Logging to Source File Loading

**File:** `service.go`
**Location:** Inside `ReindexTenantContentWithResume`, where source files are loaded

**Current:**
```go
sourceFiles, err := h.store.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
    TenantID: &tenantID,
})
if err != nil {
    return fmt.Errorf("failed to list source files: %w", err)
}
```

**Target:**
```go
sourceFiles, err := h.store.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
    TenantID: &tenantID,
})
if err != nil {
    return fmt.Errorf("failed to list source files: %w", err)
}
slog.Info("reindex: loaded source files",
    "tenant_id", tenantID,
    "count", len(sourceFiles),
    "files", func() []string {
        var files []string
        for _, f := range sourceFiles {
            files = append(files, fmt.Sprintf("%s/%s (v%d, %d bytes)", f.AudienceType, f.FileType, f.Version, len(f.Content)))
        }
        return files
    }(),
)
if len(sourceFiles) == 0 {
    slog.Warn("reindex: no source files found for tenant", "tenant_id", tenantID)
    return nil
}
```

---

### Change 3: Add Logging to Reindex Stages

**File:** `service.go`
**Location:** Inside `ReindexTenantContentWithResume`

**Add logging at:**
1. After source file filtering (what files are being processed)
2. After chunking (how many chunks produced)
3. After embedding (how many vectors inserted)
4. At checkpoint creation/update

**Specifically:**
```go
// After chunking
slog.Info("reindex: chunking complete",
    "tenant_id", tenantID,
    "total_chunks", len(allChunks),
    "garbage_chunks", garbageCount,
)

// After embedding
slog.Info("reindex: embedding batch complete",
    "tenant_id", tenantID,
    "batch", currentBatch,
    "chunks_embedded", len(vectors),
    "chunks_failed", batchFailedCount,
)
```

---

### Change 4: Fix Silent Failure at service.go:805-807

**File:** `service.go`
**Location:** Lines 805-807

**Current:**
```go
if len(allChunks) == 0 {
    return 0, nil
}
```

**Target:**
```go
if len(allChunks) == 0 {
    slog.Warn("reindex: chunking produced zero chunks",
        "tenant_id", tenantID,
        "source_file_count", len(sourceFiles),
    )
    return 0, nil
}
```

---

### Change 5: Clean Up Stuck Checkpoint for Tenant 12

**File:** Run SQL directly or add cleanup logic

**Option A (Manual SQL):**
```sql
UPDATE agent_reindex_checkpoints
SET status = 'failed', error_message = 'Orphaned checkpoint — cleaned up during investigation'
WHERE tenant_id = 12 AND status = 'in_progress';
```

**Option B (Code fix):** Add checkpoint cleanup on startup in `NewService()`:
```go
// Clean up any orphaned in_progress checkpoints from previous runs
if err := h.store.CleanupStaleCheckpoints(ctx); err != nil {
    slog.Warn("failed to cleanup stale checkpoints", "error", err)
}
```

---

## Verification

1. **Build:** `go build ./...`
2. **Run:** `task run:rag`
3. **Trigger:** Click "Rebuild Index" for evpn tenant
4. **Verify:**
   - Log shows "reindex goroutine started"
   - Log shows "loaded source files" with count >= 1
   - Log shows "chunking complete" with chunk count > 0
   - Log shows "embedding batch complete" with vectors > 0
   - DB shows checkpoint created for tenant 13
   - RAG Stats shows non-zero chunks

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Logging too verbose in production | Log noise | Use `slog.Info` for key events only |
| Cleanup of stuck checkpoint loses data | None — orphaned checkpoint is useless | Verified tenant 12 checkpoint is stuck at batch 1493/2588 with no completion |
| Goroutine still fails silently | User still sees zero chunks | Added logging at every stage |

---

## Rollback
Revert the 4 file changes. The stuck checkpoint cleanup is a one-time fix.
