# Review: plan_ux2.md (037-UX, revision)

**Verdict: APPROVED WITH NITS**

---

## Previous Review Items — All Resolved ✅

| Review Nit (round 1) | Status | How Addressed |
|---|---|---|
| Misleading success toast for pre-checkpoint failures | ✅ Resolved | Change 1 — synchronous pre-checks return 200 + `skip_reason` instead of 202 + goroutine |
| `skip_reason` is an untyped magic string | ✅ Resolved | Change 1 — typed Go constants + TypeScript union type |
| `retrieval_mode` fetch failures silently swallowed | ✅ Resolved | Change 3 — error is logged, field left empty on failure |
| Unused translation keys | ✅ Resolved | Change 7 note confirms all 5 keys are now referenced |
| No test coverage | ✅ Resolved | Change 8 — 3 handler tests + 2 checkpoint tests proposed |

---

## New Issues (nit-level)

### 1. Root cause `content_tokens = 0` unaddressed

**File context:** Root Cause Analysis table, row 3: `"content_tokens = 0 for evpn — mode never properly calculated"`

The plan's root cause table identifies this as a gap, but the changes never touch it. If `retrieval_mode` is wrong (still "rag" when it should be "long_context"), all mode-aware UX messaging (idle card, skip_reason) will be misleading rather than informative.

**Fix:** Add a note in the **Scope** or **Risks** section that mode calculation is out of scope for this UX plan, and cross-reference `plan_fix.md` as the fix location. Without that fix, the UX improvements here are partially undermined.

---

### 2. Source file query fetches full records, not a count

**Change 1:**
```go
files, _ := h.store.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
    TenantID:   &tenant.ID,
    LatestOnly: true,
})
if len(files) == 0 { ... }
```

`ListAgentSourceFiles` may return full records (including content bytes). For tenants with many files this is more expensive than a `SELECT COUNT(*)`. The plan's risk table notes "Extra DB query" but mischaracterizes it as "single indexed query, fast" — it's a full row fetch, not a count.

**Fix:** Either:
- Confirm that `LatestOnly: true` returns metadata only (no content blob), OR
- Add a lightweight `CountTenantSourceFiles(tenantID) int` method to the store, OR
- Add a comment in the code explaining why this is safe (e.g., `LatestOnly` strips content)

---

### 3. Zero-content files slip past the pre-check

**Change 1:** Checks `len(files) == 0` — but files might exist with zero content length. The check passes, a goroutine spawns, and the user ends up with "0 chunks indexed" — the same misleading "started → nothing" pattern the plan was designed to eliminate.

**Fix:** After the `len(files) == 0` check, add a synchronous check for total content length:
```go
totalContentLen := 0
for _, f := range files {
    totalContentLen += len(f.Content)
}
if totalContentLen == 0 {
    // return skip_reason: "no_source_files" (same message is accurate enough)
}
```

Or at minimum, acknowledge this edge case in the risk table.

---

### 4. No manual test step for pipeline_disabled

**Verification section, steps 2–5:** Covers long_context, RAG, no source files, and idle state — but not the `pipeline_disabled` synchronous skip path.

**Fix:** Add a step 4b:
```bash
# Disable RAG (set RAG_PIPELINE_ENABLED=false)
# Click Rebuild Index
# Expect: ⚠️ toast "Pipeline not configured"
```

---

### 5. `createFailedCheckpoint` context rationale undocumented

**Change 2:**
```go
checkpointCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
```

This uses a fully detached context so the checkpoint write survives parent cancellation — correct intent. But without a comment, a future reader might "fix" it to use the parent context, reintroducing the silent-failure bug. The rationale should be documented inline.

**Fix:** Add a comment:
```go
// Detached context: checkpoint write must persist even if the
// parent request context is cancelled (goroutine may abort).
// 5s timeout prevents a slow DB write from blocking indefinitely.
```

---

## Summary

The revision is high quality — all 5 round-1 nits are properly addressed. The 5 remaining nits above are smaller: 3 documentation/edge-case clarifications, 1 missing test step, and 1 out-of-scope acknowledgement. Address and proceed.
