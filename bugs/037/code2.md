# code2.md — Fixes for code_review.md Findings

**Date:** 2026-07-14
**Status:** Ready for approval — no coding until explicit go-ahead
**Scope:** Fixes to bugs introduced by plan_ux3.md implementation only

---

## Findings In Scope (my bugs)

### P0-1: `isRebuilding` never reset on skip — button spins forever

**File:** `web/src/pages/AgentAdmin.tsx:304-327`
**Severity:** User-facing infinite spinner

`handleRebuildIndex` sets `setIsRebuilding(true)`, then enters a `switch (result.skip_reason)` block. Every skip case shows a toast but **never calls `setIsRebuilding(false)`**. The reset only exists in the `else` branch (failure path).

Since `loading={isRebuilding || status === "in_progress"}`, the button spins indefinitely after a synchronously skipped reindex.

**Fix:**
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
  setIsRebuilding(false);  // ← ADD: reset for all skip cases
} else {
  setIsRebuilding(false);
  toast.error(result.error || t("agent-admin.rebuild-index-failed"));
}
```

---

### P1-1: Test doesn't call the function it claims to test

**File:** `server/router/api/v1/agent/service_reindex_test.go:43-65`

`TestCreateFailedCheckpointWritesToStore` constructs a `store.ReindexCheckpoint` literal and checks its fields — it never calls `createFailedCheckpoint`. A refactor could delete the function and the test would still pass.

**Fix:** Rename to `TestReindexCheckpointStructFields` to match actual behavior. A proper integration test (mock store + call `createFailedCheckpoint`) is deferred — it requires mock infrastructure not present in the codebase.

---

### P2-1: No safety timeout for goroutine spinner

**File:** `web/src/pages/AgentAdmin.tsx`

The `default` case (goroutine started) also doesn't call `setIsRebuilding(false)` — it relies on polling to eventually pick up `status !== "in_progress"`. If the goroutine hangs or the polling endpoint is unreachable, the button spins forever.

**Fix:** Add a `useEffect` safety timeout that resets `isRebuilding` after 60 seconds with no status change:

```typescript
useEffect(() => {
  if (!isRebuilding) return;
  const timeout = setTimeout(() => {
    setIsRebuilding(false);
  }, 60_000); // 60s safety net
  return () => clearTimeout(timeout);
}, [isRebuilding]);
```

**Placement:** After the existing `useEffect` blocks, before the JSX return.

---

### P2-2: Long_context skip hardcodes `"audience":"internal"`

**File:** `server/router/api/v1/agent/handlers.go:1186`

The long_context skip response returns `"audience": "internal"` regardless of the requested `audience_type` query param. This is misleading — the user requested a specific audience but the response says "internal".

**Fix:** Use the requested audience type:
```go
// Before the long_context check, resolve audienceType:
audienceType := c.QueryParam("audience_type")
if audienceType == "" {
    audienceType = "all"
}

// Then in the long_context skip response:
"audience": audienceType,
```

**Note:** This requires moving the audience resolution BEFORE the long_context check (currently it's after). The order becomes:
1. Get tenant
2. Check admin/permission
3. Resolve audience type
4. Check long_context mode (use resolved audience)
5. Pre-check pipeline
6. Pre-check source files
7. Goroutine spawn

---

### P2-4: Whitespace-only files bypass `totalContentLen == 0` check

**File:** `server/router/api/v1/agent/handlers.go:1205-1214`

Files containing only whitespace (spaces, newlines, tabs) have `len(content) > 0` but no meaningful content. The pre-check passes, a goroutine spawns, chunking produces 0 chunks, and the user sees "Reindexing started" → nothing.

**Fix:** Add a `strings.TrimSpace` check:
```go
} else if count == 0 || totalContentLen == 0 || isAllWhitespace(files) {
```

With helper:
```go
func isAllWhitespace(files []*store.AgentSourceFile) bool {
    for _, f := range files {
        if strings.TrimSpace(f.Content) != "" {
            return false
        }
    }
    return true
}
```

**Note:** This requires importing `strings` and `store` in `handlers.go` (both already imported). The `isAllWhitespace` function iterates over the files from `CountTenantSourceFiles` — but wait, `CountTenantSourceFiles` only returns count and total length, not the files themselves.

**Revised approach:** Extend `CountTenantSourceFiles` to also return a `hasNonWhitespace` bool, OR add a separate query. The cheapest approach is to add a `MAX(LENGTH(TRIM(content)))` check:

```sql
SELECT COUNT(*), COALESCE(SUM(LENGTH(content)), 0), MAX(LENGTH(TRIM(content)))
FROM agent_source_files WHERE tenant_id = ?
```

If `maxTrimmedLen == 0`, all files are whitespace-only.

**Update all 3 DB implementations** (SQLite, Postgres, MySQL) to return a third value:
```go
CountTenantSourceFiles(ctx context.Context, tenantID int32) (count int, totalContentLen int, maxTrimmedLen int, err error)
```

---

## Findings Out of Scope (pre-existing, not my bugs)

| Finding | Why Out of Scope |
|---------|-----------------|
| P0-2: `forceRAG` overrides long_context | Chat flow bug, not reindex flow. Different handler, different code path. |
| P1-2: Contradictory RAG fallback prompt | Pre-existing prompt construction in chat flow. |
| P1-3: `IsRAGEnabled` vs `UseRAGPipeline` | Design inconsistency predating this change. |
| P2-3: No status on first page load | By design — idle means no checkpoint. The new idle card addresses this. |
| P2-5: `ragMinScore` too low | Pre-existing threshold, not related to reindex UX. |
| P3-1: Shadowed `countErr` | Intentional fallthrough — error logged, goroutine handles it. |
| P3-2: Const test trivial | Low priority, can rename in follow-up. |
| P3-3: MySQL stub | Consistent with all other MySQL agent methods. |
| P3-4: UTF-8 truncation | Pre-existing in `truncateToTokenBudget`. |

---

## Implementation Order

| Step | Change | Files | Risk |
|------|--------|-------|------|
| 1 | P0-1: Reset `isRebuilding` on skip | `AgentAdmin.tsx` | Low — single line add |
| 2 | P2-1: Add 60s safety timeout | `AgentAdmin.tsx` | Low — new useEffect |
| 3 | P2-2: Move audience resolution before long_context check | `handlers.go` | Medium — reorders handler logic |
| 4 | P2-4: Extend `CountTenantSourceFiles` with `maxTrimmedLen` | `driver.go`, `agent.go`, `sqlite/agent.go`, `postgres/agent.go`, `mysql/agent.go`, `handlers.go` | Medium — changes interface + 5 files |
| 5 | P1-1: Rename test | `service_reindex_test.go` | Low — rename only |

---

## Verification

After each step:
```bash
go build ./...
go test ./server/router/api/v1/agent/... -count=1
```

Manual test after all steps:
1. Open evpn tenant (long_context) → Click Rebuild Index → expect ℹ️ toast + button stops spinning
2. Open RAG tenant → Click Rebuild Index → expect ✅ toast + progress bar
3. Create tenant with no files → Click Rebuild Index → expect ⚠️ toast + button stops spinning
4. Set `RAG_PIPELINE_ENABLED=false` → Click Rebuild Index → expect ⚠️ toast + button stops spinning
5. Wait 60s after any skip → verify button is not stuck

---

## Risks

| Risk | Impact | Mitigation |
|------|--------|-----------|
| P2-2 reordering changes handler flow | Low — audience is only used in response, not in pre-checks | Test all 3 skip paths after reorder |
| P2-4 third return value changes interface | Medium — all 3 DB implementations must be updated | MySQL returns `errNotImplemented`, safe to add third return |
| P2-1 60s timeout may fire during legitimate long reindex | Low — timeout only resets the spinner, not the goroutine | Goroutine continues independently; status polling still works |

---

## Rollback

Revert the 6 file changes. No data model changes. No migrations needed.
