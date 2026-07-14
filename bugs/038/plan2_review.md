# Plan v2 Review: Bug #038 — RAG Version Rollback UI + tenant:admin RBAC Fix

**Reviewer:** Senior Go Architect  
**Status:** Approved  
**Date:** 2026-07-15

---

## Verdict: APPROVED — proceed to implementation

Plan v2 addresses all findings from plan_review.md. The plan is minimal, correct, and ready for execution.

---

## Resolved from v1 Review

| Issue | Status |
|-------|--------|
| Race condition (rollback vs reindex) | ✅ Per-tenant mutex added (Step 2) |
| Route group inaccuracy | ✅ Corrected to `adminGroup` throughout |
| HandleReindexTenant not covered | ✅ Added to defense-in-depth (Step 3) |
| Error messages outdated | ✅ Updated to mention `tenant:admin` |

---

## Remaining Nits (Do Not Block)

### 1. Mutex scope in `HandleSetActiveVersion`

The plan locks only around `UpsertAgentRAGActiveVersion`, but `HandleSetActiveVersion` calls `ListIndexedVersions` first (handler.go:6027) to validate the target version exists. If the mutex is held only for the upsert, retention pruning inside `reindexFileVersion` could delete the version between the list and the upsert.

**Impact:** Very narrow window — the version would fail the upsert's existence check, but the mutex should cover `ListIndexedVersions` too for correctness. If the version is pruned between the list and the upsert, the rollback returns 400 ("version not found") which is acceptable. Not a data corruption issue.

### 2. `sync.Map` memory leak

Entries in `reindexMu` are never removed when a tenant is deleted. For expected tenant counts (hundreds), this is negligible.

### 3. Missing `sync` import

The `Service` struct needs `import "sync"` for `sync.Map`. The plan should note this.

---

## Implementation Order

The plan's execution order (Section 9) is correct. No reordering needed.

Proceed to implementation.
