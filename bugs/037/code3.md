# code3.md — Fixes for code_review.md Findings (incorporated code2_review.md)

**Date:** 2026-07-14
**Status:** Ready for implementation
**Scope:** Fixes to bugs introduced by plan_ux3.md implementation only

---

## Findings In Scope

### P0-1: `isRebuilding` never reset on skip — button spins forever

**File:** `web/src/pages/AgentAdmin.tsx:304-327`

**Fix:** Add `setIsRebuilding(false)` after the switch in the success branch:

```typescript
if (result.success) {
  switch (result.skip_reason) { ... }
  setIsRebuilding(false);  // reset for all skip cases
} else {
  setIsRebuilding(false);
  toast.error(result.error || t("agent-admin.rebuild-index-failed"));
}
```

---

### P1-1: Test doesn't call the function it claims to test

**File:** `server/router/api/v1/agent/service_reindex_test.go:43-65`

**Fix:** Rename `TestCreateFailedCheckpointWritesToStore` → `TestReindexCheckpointStructFields`.
Integration test with mock store deferred — follow-up issue needed.

---

### P2-1: No safety timeout for goroutine spinner

**File:** `web/src/pages/AgentAdmin.tsx`

**Fix:** Add useEffect safety timeout (60s). Does not cancel goroutine — only resets UI spinner.

```typescript
useEffect(() => {
  if (!isRebuilding) return;
  const timeout = setTimeout(() => {
    setIsRebuilding(false);
  }, 60_000);
  return () => clearTimeout(timeout);
}, [isRebuilding]);
```

---

### P2-2: Long_context skip hardcodes `"audience":"internal"`

**File:** `server/router/api/v1/agent/handlers.go`

**Fix:** Move audience resolution before the long_context check. Use the resolved `audienceType` in the skip response.

Order becomes:
1. Get tenant
2. Check admin/permission
3. Resolve audience type ← moved up
4. Check long_context mode
5. Pre-check pipeline
6. Pre-check source files
7. Goroutine spawn

---

### P2-4: Whitespace-only files bypass `totalContentLen == 0` check

**File:** `store/driver.go`, `store/agent.go`, 3 DB files, `handlers.go`

**Fix:** Extend `CountTenantSourceFiles` to return third value `maxTrimmedLen`:

```go
CountTenantSourceFiles(ctx context.Context, tenantID int32) (count int, totalContentLen int, maxTrimmedLen int, err error)
```

SQL:
```sql
SELECT COUNT(*), COALESCE(SUM(LENGTH(content)), 0), MAX(LENGTH(TRIM(content)))
FROM agent_source_files WHERE tenant_id = ?
```

**TRIM limitation (nit #1):** Default TRIM only strips space characters (0x20), not tabs/newlines.
Tab-only whitespace files are an accepted edge case — document in code comment.

**Caller verification (nit #2):** Run `grep -rn "CountTenantSourceFiles"` before implementing.
Expected callers: `handlers.go` (1 site) + store implementations (3 sites).

---

## Verification

```bash
grep -rn "CountTenantSourceFiles" server/ store/ plugin/  # verify callers
go build ./...
go test ./server/router/api/v1/agent/... -count=1
go test ./store/... -count=1
```

Manual test:
1. evpn tenant (long_context) → Rebuild Index → ℹ️ toast + button stops
2. RAG tenant → Rebuild Index → ✅ toast + progress bar
3. No files → Rebuild Index → ⚠️ toast + button stops
4. Pipeline disabled → Rebuild Index → ⚠️ toast + button stops
5. Wait 60s after skip → button not stuck

---

## Risks

| Risk | Mitigation |
|------|-----------|
| P2-4 interface change breaks callers | grep verification before implementing |
| P2-2 reorder changes handler flow | Test all 3 skip paths after reorder |
| P2-1 timeout fires during legitimate reindex | Only resets spinner, goroutine continues independently |
| P1-1 integration test deferred | Note in risks: follow-up issue needed for mock infrastructure |
