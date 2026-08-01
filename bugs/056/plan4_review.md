# Adversarial Plan Review: Plan 4 — Bug 056 Content Type Mismatch Fix (Final)

**Bug/Task:** Plan for `plan4.md`  
**Reviewer:** Kilo (Senior Go Architect)  
**Date:** 2026-08-01  
**Verdict:** REWORK REQUIRED — plan fixes all test issues from Plan 3 and adds correct dual-type tests, but introduces a **critical implementation bug** in the Bug 2 reindex fix that bypasses essential reindex logic (mutexes, versioning, retention).

---

## Executive Summary

Plan 4 correctly addresses all findings from Plan 3:
- Bug 1: Dual-type search `["ticket", "ticket_section"]` in `EscalateTicket` and `InferResolutionForNewTicket`
- Bug 2: `"bug"` handler added to all three reindex functions
- Bug 3: Fixed at correct locations — `handlers.go:4977`, `handlers.go:5107`, `service.go:5031`
- Tests updated to use `"ticket_section"`
- New dual-type test fixed: `TestAskRovo_DualTypeSearchFindsOldAndNewChunks`
- New admin search test added: `TestSearchVectorDB_DualTypeContentTypes`
- Convention documented in `chunker.go`

However, **the Bug 2 reindex implementation is fundamentally incorrect**. The plan proposes direct chunking + insertion (`s.chunker.ChunkMarkdownContent` + `s.vectorDB.Insert`) instead of using the existing `s.ReindexFileVersion` helper. This bypasses critical logic: tenant-level mutex serialization, active version pointer updates, pre-versioned chunk purging, and retention policy enforcement.

| # | Finding | Severity | Action |
|---|---------|----------|--------|
| 1 | Bug 2 fix bypasses `ReindexFileVersion` — misses mutex, versioning, retention | **HIGH** | Rework |
| 2 | `TestSearchVectorDB_DualTypeContentTypes` does not exercise handler code paths | MEDIUM | Document |
| 3 | Plan does not verify Bug 1 dual-type search works in LanceDB `buildFilter` | LOW | Document |

---

## Finding 1: Bug 2 Fix Bypasses `ReindexFileVersion`

**Severity:** HIGH (CRITICAL)

The plan proposes the following Bug 2 fix for `ReindexAllContent`:

```go
if entry, ok := fileMap["bug"]; ok {
    maxChunkTokens := GetMaxChunkTokens(...)
    chunks := s.chunker.ChunkMarkdownContent(entry.content, 0, "internal", "bug", entry.version, maxChunkTokens)
    if len(chunks) > 0 {
        if err := s.vectorDB.Insert(ctx, chunks); err != nil {
            slog.Error("failed to insert bug chunks", "error", err)
        } else {
            slog.Info("indexed bug content", "chunks", len(chunks))
        }
    }
}
```

However, the existing `"kb"` and `"policy"` handlers use `s.ReindexFileVersion(...)`:

```go
if entry, ok := fileMap["kb"]; ok {
    if count, err := s.ReindexFileVersion(tenantCtx, tenant.ID, audience, "kb", entry.version, entry.content, maxChunkTokens); err != nil {
        slog.Warn("failed to reindex kb", "tenantID", tenant.ID, "audience", audience, "error", err)
    } else {
        totalChunks += count
    }
}
```

`ReindexFileVersion` (service.go:563) performs critical logic that the plan's proposed code bypasses:

1. **Empty content check**: `if content == "" { return 0, nil }`
2. **Tenant-level mutex**: `mu := s.getTenantMutex(tenantID); mu.Lock(); defer mu.Unlock()` — serializes reindex operations per tenant to prevent race conditions
3. **Chunking**: `chunks := s.chunker.ChunkMarkdownContent(...)` — same as plan
4. **Pre-versioned chunk purge**: `if len(existing) == 0 { s.vectorDB.PurgePreVersionedChunks(...) }` — cleans up old unversioned data
5. **Insert**: `s.vectorDB.Insert(ctx, chunks)` — same as plan
6. **Active version pointer**: `s.store.UpsertAgentRAGActiveVersion(...)` — updates the active version so `resolveQueryVersion` can find it
7. **Retention policy**: keeps last 5 versions, purges older ones

**Impact of bypassing `ReindexFileVersion`:**
- **No mutex**: Concurrent reindex calls for the same tenant could race, corrupting the vector DB
- **No active version pointer**: `resolveQueryVersion` won't find the bug chunks because there's no `AgentRAGActiveVersion` row pointing to them. `SearchVectorDB` and admin searches would return empty results.
- **No retention**: Unlimited bug versions accumulate, wasting storage
- **No pre-versioned purge**: Old bug data persists alongside new versioned data

**Required fix:** Use `ReindexFileVersion` for all three reindex functions:

```go
if entry, ok := fileMap["bug"]; ok {
    if count, err := s.ReindexFileVersion(tenantCtx, tenant.ID, audience, "bug", entry.version, entry.content, maxChunkTokens); err != nil {
        slog.Warn("failed to reindex bug", "tenantID", tenant.ID, "audience", audience, "error", err)
    } else {
        totalChunks += count
    }
}
```

This matches the `"kb"` and `"policy"` pattern exactly.

---

## Finding 2: `TestSearchVectorDB_DualTypeContentTypes` Does Not Exercise Handler Paths

**Severity:** MEDIUM

The new test `TestSearchVectorDB_DualTypeContentTypes` calls `svc.SearchVectorDB` directly. This verifies the `SearchVectorDB` fix (Bug 3, `service.go:5031`) but does NOT exercise the two handler-level fixes:

1. `handlers.go:4977` — `HandleTestRAGSearch`
2. `handlers.go:5107` — `HandleTenantRAGSearch`

If a future developer reverts the handler fixes but keeps the `SearchVectorDB` fix, this test would still pass. The originally reported broken endpoints would remain broken.

**Impact:** The Bug 3 fix at the handler level is untested. A regression in those handlers would not be caught.

**Required addition:** Either:
1. Add handler-level tests that exercise `HandleTestRAGSearch` and `HandleTenantRAGSearch` via HTTP requests, OR
2. Document this as a known coverage gap and add it to the follow-up table

**Recommendation:** Add to follow-up table: "Add HTTP-level tests for `HandleTestRAGSearch` and `HandleTenantRAGSearch` to verify dual-type ContentTypes fix."

---

## Finding 3: Plan Does Not Verify Bug 1 Dual-Type Search in LanceDB

**Severity:** LOW

The existing tests use `MemoryVectorDB`. The Bug 1 fix also affects LanceDB's `buildFilter` function (vectordb_lance.go:1179):

```go
if len(query.ContentTypes) > 0 {
    types := make([]string, len(query.ContentTypes))
    for i, ct := range query.ContentTypes {
        types[i] = fmt.Sprintf("'%s'", ct)
    }
    filterParts = append(filterParts, fmt.Sprintf("content_type IN (%s)", strings.Join(types, ", ")))
}
```

This correctly generates `content_type IN ('ticket', 'ticket_section')` for the dual-type search. No fix is needed in LanceDB — the existing code already supports multiple content types.

However, the plan does not explicitly verify this. If someone later modifies `buildFilter` to not support multiple types, the Bug 1 fix would break in LanceDB.

**Recommendation:** Document as low-priority follow-up: "Verify LanceDB `buildFilter` supports multiple ContentTypes in IN clause."

---

## What Is Correct

| Aspect | Status | Notes |
|--------|--------|-------|
| Bug 1 fix locations | CORRECT | `service.go:5579` and `5618` use dual-type search |
| Bug 1 backward compatibility | CORRECT | Old `"ticket"` and new `"ticket_section"` both matched |
| Bug 3 fix locations | CORRECT | `handlers.go:4977`, `handlers.go:5107`, `service.go:5031` |
| Convention documentation | CORRECT | `chunker.go` comment added |
| Migration strategy | CORRECT | Dual-type search handles backward compatibility |
| Test updates | CORRECT | Existing tests use `"ticket_section"` consistently |
| Dual-type test query | CORRECT | Title contains seed keywords → HIGH vector |
| Admin search test | CORRECT | Tests `SearchVectorDB` with both old and new content types |
| Follow-up items | CORRECT | Embedder tests, reindex verification, observation standardization |

---

## Behavioral Correctness Check

### Bug 1: Dual-Type Search
| Step | Expected | Actual |
|------|----------|--------|
| Search with `ContentTypes: ["ticket", "ticket_section"]` | Matches both old and new chunks | CORRECT — MemoryVectorDB and LanceDB both match either type |
| Old embedder chunks (`"ticket"`) | Found | CORRECT |
| New reindex chunks (`"ticket_section"`) | Found | CORRECT |
| Main chat path (`RetrieveContextForQuery`) | Unaffected | CORRECT — uses empty ContentTypes |

### Bug 2: Bug Reindex
| Step | Expected | Actual |
|------|----------|--------|
| Plan's proposed code | Direct chunking + insertion | **INCORRECT** — bypasses `ReindexFileVersion` |
| Correct implementation | `ReindexFileVersion` with `"bug"` fileType | Would handle mutex, versioning, retention |

### Bug 3: Admin Search
| Step | Expected | Actual |
|------|----------|--------|
| `HandleTestRAGSearch` with `FileType="kb"` | Finds both `"kb"` and `"kb_section"` chunks | CORRECT — dual-type pattern applied |
| `HandleTenantRAGSearch` with `FileType="policy"` | Finds both `"policy"` and `"policy_section"` chunks | CORRECT — dual-type pattern applied |
| `HandleRAGSearch` with `FileType="kb"` | Finds both `"kb"` and `"kb_section"` chunks | CORRECT — `SearchVectorDB` fixed |
| Test coverage | Handlers tested via HTTP | **MISSING** — only `SearchVectorDB` tested directly |

---

## Recommended Rework

### 1. Fix Bug 2 Reindex Implementation (Finding 1)

Replace the plan's direct chunking + insertion with `ReindexFileVersion` calls:

```go
// ReindexAllContent (~line 677)
if entry, ok := fileMap["bug"]; ok {
    if count, err := s.ReindexFileVersion(tenantCtx, tenant.ID, audience, "bug", entry.version, entry.content, maxChunkTokens); err != nil {
        slog.Warn("failed to reindex bug", "tenantID", tenant.ID, "audience", audience, "error", err)
    } else {
        totalChunks += count
    }
}

// ReindexTenantContent (~line 782): same pattern
// ReindexTenantContentWithResume (~line 1175): same pattern
```

This ensures:
- Tenant mutex serialization
- Active version pointer update
- Pre-versioned chunk purge
- Retention policy enforcement

### 2. Add Handler-Level Test Coverage (Finding 2)

Add to follow-up table:
> Add HTTP-level tests for `HandleTestRAGSearch` and `HandleTenantRAGSearch` to verify dual-type ContentTypes fix at handler level.

### 3. Document LanceDB `buildFilter` Verification (Finding 3)

Add to follow-up table:
> Verify LanceDB `buildFilter` supports multiple ContentTypes in IN clause (already works, but not explicitly tested).

---

## Final Verdict

**REWORK REQUIRED.** Plan 4 correctly fixes all test issues from Plan 3 and adds comprehensive test coverage for the dual-type search pattern. The Bug 1 and Bug 3 fixes are at the correct locations and use the correct pattern.

However, **the Bug 2 reindex implementation is fundamentally incorrect**. The plan proposes direct chunking + insertion instead of using the existing `ReindexFileVersion` helper. This bypasses critical logic: tenant-level mutex serialization, active version pointer updates, pre-versioned chunk purging, and retention policy enforcement. Without `ReindexFileVersion`, bug chunks would be inserted but would not be found by `resolveQueryVersion` (no active version pointer), and concurrent reindex operations could race.

**Required fix:** Replace the direct chunking + insertion code with `s.ReindexFileVersion(...)` calls, matching the existing `"kb"` and `"policy"` pattern exactly.

After this fix, the plan will be implementation-ready.
