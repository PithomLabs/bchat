# Plan 101: Disable Auto RAG Reindex on Startup

**Status:** Implementation-ready
**Date:** 2026-07-07
**Location:** `server/router/api/v1/agent/service.go` (`NewService`, lines ~132-168)

## Goal
Add a single, reversible runtime switch to prevent the agent service from automatically
reindexing RAG content on startup. When enabled, **both** startup reindex paths are skipped:
1. The explicit `FORCE_REINDEX_ON_STARTUP=true` full reindex (after 2s).
2. The "auto-bootstrap" reindex (after 5s) that triggers when the vector DB is empty but
   SQLite source files exist.

Manual admin reindex (`POST /api/v1/agent/:slug/reindex` → `ReindexAllContent`) continues to
work unchanged. No database migration is required.

## Decisions (confirmed with user)
| # | Decision | Choice |
|---|----------|--------|
| 1 | Scope | Disable BOTH startup paths (explicit + auto-bootstrap) |
| 2 | Mechanism | New env var `RAG_STARTUP_REINDEX_DISABLED`; when `"true"` skip both; non-destructive/reversible |
| 3 | Empty-DB warning | When disabled AND RAG enabled AND vector DB empty AND source files exist → log `WARN` pointing to manual reindex |

## Current behavior (unchanged unless flag set)
- `FORCE_REINDEX_ON_STARTUP == "true"` → `go` routine sleeps 2s, calls `ReindexAllContent`.
- Otherwise → `go` routine sleeps 5s; if `IsRAGEnabled()` and `GetVectorDB().Stats().TotalChunks == 0`
  and `ListAgentSourceFiles(LatestOnly:true)` returns >0 rows → `ReindexAllContent`.
- `ReindexAllContent` is also reachable from admin endpoints (manual), independent of startup logic.

## Implementation
In `NewService`, wrap the existing two branches (lines 132-168) as follows:

```go
// Startup RAG reindex control.
// RAG_STARTUP_REINDEX_DISABLED=true skips ALL automatic startup reindexing
// (explicit FORCE_REINDEX_ON_STARTUP path AND empty-DB auto-bootstrap).
// Manual admin reindex endpoints remain fully functional.
if os.Getenv("RAG_STARTUP_REINDEX_DISABLED") == "true" {
    if svc.IsRAGEnabled() {
        // Best-effort: warn operators that the vector store will stay empty
        // until they trigger a manual reindex.
        if stats, serr := svc.GetVectorDB().Stats(context.Background()); serr == nil && stats.TotalChunks == 0 {
            if files, ferr := s.ListAgentSourceFiles(context.Background(), &store.FindAgentSourceFile{LatestOnly: true}); ferr == nil && len(files) > 0 {
                slog.Warn("Startup RAG reindex disabled (RAG_STARTUP_REINDEX_DISABLED=true) and vector DB is empty; run manual admin reindex (POST /api/v1/agent/:slug/reindex) to populate it",
                    "sourceFilesCount", len(files))
            }
        }
    }
} else if os.Getenv("FORCE_REINDEX_ON_STARTUP") == "true" {
    go func() {
        time.Sleep(2 * time.Second)
        if err := svc.ReindexAllContent(context.Background()); err != nil {
            slog.Error("Failed to reindex RAG content on startup", "error", err)
        }
    }()
} else {
    go func() {
        time.Sleep(5 * time.Second)
        ctx := context.Background()
        if svc.IsRAGEnabled() {
            stats, err := svc.GetVectorDB().Stats(ctx)
            if err == nil && stats.TotalChunks == 0 {
                files, err := s.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{LatestOnly: true})
                if err == nil && len(files) > 0 {
                    slog.Info("RAG vector database table is empty but source files exist. Auto-triggering bootstrap reindexing in the background...", "sourceFilesCount", len(files))
                    if err := svc.ReindexAllContent(ctx); err != nil {
                        slog.Error("Failed to auto-bootstrap RAG content reindexing", "error", err)
                    }
                }
            }
        }
    }()
}
```

Notes for the implementing agent:
- The warning block calls `Stats`/`ListAgentSourceFiles` inline (not in a `go` routine). These are
  fast read-only calls; this keeps the WARN deterministic at startup. Acceptable because it only runs
  when the flag is set.
- Keep the existing variable/method names (`ReindexAllContent`, `IsRAGEnabled`, `GetVectorDB`,
  `ListAgentSourceFiles`) — verified present in `service.go`.
- No other startup reindex triggers exist (confirmed via grep: only `service.go:134/138/161/162`
  reference these; `ReindexAllContent` at `:272` is the callable impl).

## Files touched
- `server/router/api/v1/agent/service.go` (replace lines ~132-168 with the block above)

## Rollout / migration
- **Code-only change.** No DB migration, no schema change.
- Set the flag out-of-band, matching repo convention of not baking config into the image:
  - Fly: `fly secrets set RAG_STARTUP_REINDEX_DISABLED=true` (or add `RAG_STARTUP_REINDEX_DISABLED = "true"`
    under `[env]` in `fly.toml`).
  - Local: `RAG_STARTUP_REINDEX_DISABLED=true task run:rag` (or export in `.env`).
- Redeploy/restart normally. On next boot the startup reindex goroutines are not scheduled.

## Validation
1. **Disabled path:** run with `RAG_STARTUP_REINDEX_DISABLED=true`. Confirm startup logs do NOT contain
   `"Starting RAG reindex"` or `"Auto-triggering bootstrap reindexing"`. If vector DB is empty with
   source files present, confirm the new `WARN` line appears.
2. **Manual reindex still works:** call `POST /api/v1/agent/:slug/reindex` and confirm chunks get indexed
   and RAG search returns results.
3. **Backward compatibility (default):** run with the var unset/`"false"`. Confirm behavior is identical to
   today (auto-bootstrap triggers when vector DB empty; `FORCE_REINDEX_ON_STARTUP=true` still forces a full
   reindex).

## Risks / caveats
- With the flag on and the vector store empty, RAG search returns no results until an operator runs a manual
  reindex. The `WARN` log is the mitigation; documented here so it is not mistaken for a bug.
- Operational: after a deploy that wipes the vector store (e.g., switching Tigris bucket / per-tenant prefix
  as in plan 024), remember to trigger a manual reindex — auto-bootstrap will no longer do it.
- Low sensitivity flag; safe to put in `fly.toml [env]` or as a secret.

## Out of scope
- Changing `FORCE_REINDEX_ON_STARTUP` semantics.
- Removing manual reindex endpoints.
- Any database migration.
