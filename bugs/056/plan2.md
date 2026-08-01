# Plan 2: Bug 056 — Content Type Mismatch Fix (Revised)

**Date:** 2026-08-01
**Status:** Revised based on plan_review.md findings
**Review:** 8 findings — all addressed
**Related:** Bug 052 (per-ticket RAG indexing), Bug 055 (Ask Rovo E2E test)

---

## Background

During manual E2E testing of the Ask Rovo feature, tickets #172 and #173 were created on tenant 19 (hackathon-demo). Both tickets were successfully indexed into LanceDB (source files 110, 111 exist with correct content hashes), but `InferResolutionForNewTicket` returned empty results — no AI Suggestion comment, no InternalNotes populated.

Investigation revealed a content type mismatch: `ChunkMarkdownContent` stores chunks with `ContentType: fileType + "_section"` (e.g., `"ticket_section"`), but `InferResolutionForNewTicket` searches for the raw file type (e.g., `"ticket"`). The LanceDB filter `content_type IN ('ticket')` never matches rows with `content_type = 'ticket_section'`.

Further audit revealed two additional bugs: bug history search is dead code (no reindex path processes `"bug"` source files), and admin RAG search has the same content type mismatch for KB/Policy.

---

## What Changed from Plan 1

| # | Finding | Severity | Fix Applied |
|---|---------|----------|-------------|
| 1 | Bug 1 fix breaks background embedder path | HIGH | **Fixed** — search accepts both types `["ticket", "ticket_section"]` + update embedder to `"ticket_section"` |
| 2 | Existing tests use `ContentType: "ticket"` | HIGH | **Fixed** — update test seed data to `"ticket_section"` |
| 3 | Root cause (inconsistent `_section` convention) not addressed | HIGH | **Fixed** — add convention comment, update embedder to follow convention |
| 4 | Bug 3 deferred without holistic fix | MEDIUM | **Fixed** — apply same dual-type pattern to Bug 3 |
| 5 | No migration strategy for existing LanceDB data | MEDIUM | **Fixed** — document backward compatibility via dual-type search |
| 6 | Risk assessment mitigation incorrect | MEDIUM | **Fixed** — corrected mitigation text |
| 7 | `buildFilter` line number incorrect | LOW | **Fixed** — corrected reference |
| 8 | No test coverage for `ticket_embedder.go` | LOW | **Documented** — follow-up item |

---

## ContentType Convention

**Standard:** `ChunkMarkdownContent` appends `"_section"` to `fileType` to produce `ContentType`.

| fileType | Stored ContentType | Example |
|----------|-------------------|---------|
| `"kb"` | `"kb_section"` | KB document chunk |
| `"policy"` | `"policy_section"` | Policy document chunk |
| `"ticket"` | `"ticket_section"` | Ticket content chunk |
| `"bug"` | `"bug_section"` | Bug history chunk |
| `"observation"` | `"observation"` | Observation memory (different convention — not from ChunkMarkdownContent) |

All producers MUST follow this convention. Code comment to be added to `chunker.go`.

---

## Bug 1: Ticket Content Type Mismatch (HIGH)

**Root Cause:** `InferResolutionForNewTicket` and `EscalateTicket` search for `"ticket"`, but `IndexTicketContent` → `ChunkMarkdownContent` stores `"ticket_section"`. The background embedder (`ticket_embedder.go`) stores `"ticket"` directly — a different convention.

### Fix

**Step 1: Make search accept both types (backward compatible)**

```go
// service.go:5579 (EscalateTicket)
- ContentTypes: []string{"ticket"},
+ ContentTypes: []string{"ticket", "ticket_section"},

// service.go:5618 (InferResolutionForNewTicket, Search 1)
- ContentTypes: []string{"ticket"},
+ ContentTypes: []string{"ticket", "ticket_section"},
```

**Step 2: Update background embedder to follow convention**

```go
// ticket_embedder.go:106
- ContentType: "ticket",
+ ContentType: "ticket_section",
```

**Step 3: Update tests**

```go
// ticket_rag_inference_test.go — all seed chunks
- ContentType: "ticket",
+ ContentType: "ticket_section",

- ContentType: "bug_section",
  ContentType: "bug_section",  // unchanged — already follows convention
```

### Backward Compatibility

- Old embedder chunks (`ContentType: "ticket"`) are still found by the dual-type search
- New embedder chunks (`ContentType: "ticket_section"`) are also found
- After next full reindex, all chunks will use `"ticket_section"`

---

## Bug 2: Bug History Search Is Dead Code (HIGH)

**Root Cause:** `import-bug-rag` creates `AgentSourceFile` rows with `file_type="bug"`, but all three reindex functions only handle `"kb"` and `"policy"`. Bug content is silently skipped.

### Fix

Add `"bug"` handler to all three reindex functions:

**`ReindexAllContent` (~line 677):**
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

**`ReindexTenantContent` (~line 782):** Same pattern.

**`ReindexTenantContentWithResume` (~line 1175):** Same pattern, inside the resume loop.

### Note

`ChunkMarkdownContent` with `fileType="bug"` produces `ContentType = "bug_section"`. This already matches what `InferResolutionForNewTicket` Search 2 searches for (`ContentTypes: []string{"bug_section"}`). No search query change needed.

---

## Bug 3: Admin RAG Search Content Type Mismatch (MEDIUM)

**Root Cause:** Admin search endpoints pass raw `fileType` values (`"kb"`, `"policy"`) as ContentTypes, but stored values are `"kb_section"`, `"policy_section"`.

### Fix

Apply same dual-type pattern:

```go
// service.go:5031 (SearchVectorDB)
- queryObj.ContentTypes = []string{fileType}
+ queryObj.ContentTypes = []string{fileType, fileType + "_section"}
```

This handles both raw and `_section` suffixed content types. Admin searches will find chunks regardless of which convention was used during indexing.

---

## ContentType Audit — Complete Table (Post-Fix)

| Stored ContentType | Produced by | Searched as | Searched by | Match? |
|-------------------|-------------|-------------|-------------|--------|
| `"kb_section"` | ChunkMarkdownContent | `["kb", "kb_section"]` | SearchVectorDB | YES |
| `"kb_section"` | ChunkMarkdownContent | `""` (all) | RetrieveContextForQuery | YES |
| `"policy_section"` | ChunkMarkdownContent | `["policy", "policy_section"]` | SearchVectorDB | YES |
| `"policy_section"` | ChunkMarkdownContent | `""` (all) | RetrieveContextForQuery | YES |
| `"ticket_section"` | ChunkMarkdownContent | `["ticket", "ticket_section"]` | InferResolution, Escalate | YES |
| `"ticket_section"` | ticket_embedder.go (after fix) | `["ticket", "ticket_section"]` | InferResolution, Escalate | YES |
| `"observation"` | observation_indexer.go | `["observation"]` | SearchObservations | YES |
| `"bug_section"` | ChunkMarkdownContent (after fix) | `["bug_section"]` | InferResolution Search 2 | YES |
| `"cluster"` | ticket_embedder.go | (not searched) | N/A | N/A |

---

## Implementation Plan

### Step 1: Fix Search Queries

**File:** `server/router/api/v1/agent/service.go`

| Line | Current | Fixed |
|------|---------|-------|
| 5031 | `ContentTypes: []string{fileType}` | `ContentTypes: []string{fileType, fileType + "_section"}` |
| 5579 | `ContentTypes: []string{"ticket"}` | `ContentTypes: []string{"ticket", "ticket_section"}` |
| 5618 | `ContentTypes: []string{"ticket"}` | `ContentTypes: []string{"ticket", "ticket_section"}` |

### Step 2: Update Background Embedder

**File:** `server/router/api/v1/agent/ticket_embedder.go`

| Line | Current | Fixed |
|------|---------|-------|
| 106 | `ContentType: "ticket"` | `ContentType: "ticket_section"` |

### Step 3: Add Bug Reindex

**File:** `server/router/api/v1/agent/service.go`

Add `"bug"` handler in three functions:
- `ReindexAllContent` (~line 677)
- `ReindexTenantContent` (~line 782)
- `ReindexTenantContentWithResume` (~line 1175)

### Step 4: Add Convention Comment

**File:** `server/router/api/v1/agent/chunker.go`

Add comment at `ChunkMarkdownContent` function:
```go
// ContentType convention: fileType + "_section".
// Example: fileType="kb" → ContentType="kb_section"
// All producers of DocumentChunk MUST follow this convention.
// See also: ticket_embedder.go, observation_indexer.go
```

### Step 5: Update Tests

**File:** `server/router/api/v1/agent/ticket_rag_inference_test.go`

Change all `ContentType: "ticket"` in seed data to `ContentType: "ticket_section"`.

### Step 6: Run Tests

```bash
go test -v -run TestAskRovo ./server/router/api/v1/agent/ -count=1
go test -v -race -run TestAskRovo ./server/router/api/v1/agent/ -count=1
```

### Step 7: Manual Verification

1. Reindex tenant 19
2. Create ticket #172 (per trigger052.md)
3. Create ticket #173 (per trigger_052.md)
4. Check for AI Suggestion comment on ticket #173

---

## Migration Strategy

**Existing LanceDB data:**
- Chunks from `ticket_embedder.go` with `ContentType: "ticket"` — still found by dual-type search
- Chunks from `IndexTicketContent` with `ContentType: "ticket_section"` — found by dual-type search
- After full reindex, all chunks will use `"ticket_section"` (embedder updated)

**No data migration required.** The dual-type search handles backward compatibility automatically.

---

## Files to Modify

| File | Changes |
|------|---------|
| `server/router/api/v1/agent/service.go` | Fix search ContentTypes (Bug 1, Bug 3), add bug reindex (Bug 2) |
| `server/router/api/v1/agent/ticket_embedder.go` | Update ContentType to `"ticket_section"` |
| `server/router/api/v1/agent/chunker.go` | Add convention comment |
| `server/router/api/v1/agent/ticket_rag_inference_test.go` | Update test seed ContentType |

**No new files. No schema changes. No frontend changes.**

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Dual-type search adds slight overhead | None | Single额外的字符串比较 — negligible |
| Embedder stores `"ticket_section"` but old chunks have `"ticket"` | None | Dual-type search handles both |
| Bug reindex creates many chunks for large bug corpus | Low | Retention policy keeps last 5 versions |
| Observation indexer uses different convention (`"observation"`) | None | Already searched correctly — not affected |
| Main chat RAG path breaks | None | `RetrieveContextForQuery` uses empty ContentTypes filter — unaffected |

---

## Follow-Up Work

| Item | Description | Priority |
|------|-------------|----------|
| Add tests for `ticket_embedder.go` | Verify ContentType consistency with inference path | MEDIUM |
| Standardize observation indexer | Consider renaming `"observation"` to `"observation_section"` for full convention compliance | LOW |
| Admin search UI update | Update admin RAG search UI to show content type with `_section` suffix | LOW |

---

## Adversarial Review Prompt

```
You are a senior Go architect reviewing a revised bug fix plan for a content
type mismatch in a RAG pipeline. The original plan proposed changing search
from "ticket" to "ticket_section" but this broke the background embedder path.

The revised plan uses dual-type search (["ticket", "ticket_section"]) for
backward compatibility, updates the embedder to use "ticket_section", and
adds bug content to the reindex loop.

Review this revised plan critically. Focus on:

1. BACKWARD COMPATIBILITY: Does the dual-type search correctly handle both
   old ("ticket") and new ("ticket_section") chunks? Are there edge cases?

2. CONVENTION: Is the "_section" suffix convention clearly documented?
   Will future developers follow it?

3. BUG 2: Is adding "bug" to the reindex loop sufficient? Are there any
   other paths that need updating?

4. BUG 3: Is the admin search fix correct? Does it handle all file types?

5. TESTS: Are the test updates sufficient? Do we need new tests for the
   dual-type search?

6. MIGRATION: Is the migration strategy (no data migration needed) correct?
   Will old chunks be found after the fix?

7. RISK: What's the worst case if this fix is applied incorrectly?

8. SCOPE: Is the scope appropriate? Should we include more or less?
```
