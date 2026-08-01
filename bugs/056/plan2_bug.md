# Plan 2: Bug 056 — "parent memo not found" Error (Revised)

**Date:** 2026-08-01
**Status:** Ready for implementation
**Revision:** Addresses all 6 findings from plan_bug_review.md
**Bug:** `createSystemResolutionComment` fails when parent memo has NULL `tenant_id`

---

## Investigation (Background Context)

### Step 1: Read the Error Log

Read `bugs/056/bug.md` — the log shows the full request lifecycle:

```
05:24:04 INFO OK method=/memos.api.v1.MemoService/CreateMemo          ← memo 241 created
05:24:06 INFO CreateTicket handler context_keys=[]
05:24:06 INFO CreateTicket userID userID=1 ok=true
05:24:06 INFO CreateTicket request title="Per-Ticket RAG Indexing Prototype" status=OPEN priority=MEDIUM
05:24:06 INFO CreateTicket validated
05:24:06 INFO CreateTicket success id=174
05:24:06 INFO Creating per-tenant local LanceDB connection tenantID=19 path=build/data/lancedb/19
05:24:06 INFO LanceDB vector database initialized uri=build/data/lancedb/19 provider=local tableName=kb_documents_1536 dimension=1536
05:24:06 INFO Starting batched insert totalChunks=1 batchSize=200 table=kb_documents_1536
05:24:06 INFO Processing batch batch=1 totalBatches=1 chunksInBatch=1 progress=1/1
05:24:07 WARN Failed to create vector index after insert error="...Not enough rows to train PQ..."
05:24:07 INFO Completed batched insert totalChunks=1
05:24:09 INFO inferred resolution for new ticket ticket_id=174 similar_tickets=1 bug_history=0 total=1
05:24:09 ERROR failed to create system resolution comment ticket_id=174 error="parent memo not found: %!w(<nil>)"
```

Key observations:
- Memo created 2 seconds before ticket
- Ticket created successfully (id=174)
- RAG indexing completed (1 chunk inserted, PQ index warning is non-fatal)
- Inference found 1 similar ticket — `InternalNotes` was populated
- **Then** `createSystemResolutionComment` failed

### Step 2: Grep for the Error Source

```bash
grep -rn "parent memo not found" server/router/api/v1/ticket_service.go
```

Found at `ticket_service.go:643`:
```go
return fmt.Errorf("parent memo not found: %w", err)
```

And the caller at `ticket_service.go:210`:
```go
slog.Error("failed to create system resolution comment", "ticket_id", ticket.ID, "error", commentErr)
```

### Step 3: Read the Ticket from SQLite

```bash
sqlite3 build/data/memos_dev.db ".schema tickets"
```

Schema shows `description TEXT NOT NULL DEFAULT ''` and a unique index on `(creator_id, description) WHERE description LIKE '/m/%'`.

```bash
sqlite3 build/data/memos_dev.db "SELECT id, title, description, status, priority, tenant_id, internal_notes FROM tickets WHERE id=174;"
```

Result:
```
174|Per-Ticket RAG Indexing Prototype|/m/MCSNigc5QCrgsycAnTJfih|OPEN|MEDIUM|19|## Suggested Resolution (Auto-generated)
Based on 1 similar past tickets:
### Content (86% match)
# Per-Ticket RAG Indexing Prototype

/m/MCSNigc5QCrgsycAnTJfih
```

Key: `description=/m/MCSNigc5QCrgsycAnTJfih` (links to memo), `tenant_id=19`, `internal_notes` populated by inference.

### Step 4: Read the Parent Memo from SQLite

```bash
sqlite3 build/data/memos_dev.db "SELECT uid, id, creator_id, tenant_id, row_status, visibility FROM memo WHERE uid='MCSNigc5QCrgsycAnTJfih';"
```

Result:
```
MCSNigc5QCrgsycAnTJfih|241|1||NORMAL|PRIVATE
```

**Critical finding:** `tenant_id` is **empty/NULL**. The memo has no tenant association.

For context, adjacent memos all have `tenant_id=19`:
```bash
sqlite3 build/data/memos_dev.db "SELECT uid, id, creator_id, tenant_id FROM memo ORDER BY id DESC LIMIT 5;"
```
```
MCSNigc5QCrgsycAnTJfih|241|1|      ← NULL (the problem)
J7tff2cUXLWwSMx2eHWKWq|240|1|19
GuVv2MNG8DDgt3xUWiHVNo|239|1|19
EjB4xyrpe5LrGBrRiJPpU5|238|1|19
gKNwG35mXGkeibEeVPxdgx|237|1|19
```

### Step 5: Trace the Code Flow

**`CreateTicket` handler** (`ticket_service.go:65-218`):

1. Binds request, creates `store.Ticket` with `TenantID: getTenantFromContext(c)` (line 97) → gets `19`
2. Stores ticket via `s.Store.CreateTicket(ctx, ticket)` (line 185)
3. Spawns goroutine (lines 192-212):
   ```go
   if s.agentHandler != nil && ticket.TenantID != nil {
       go func() {
           ctx := context.WithoutCancel(ctx)
           _, _, err := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, nil, true)
           // ...
           updated, fetchErr := s.Store.GetTicket(ctx, &store.FindTicket{ID: &ticket.ID})
           if fetchErr != nil || updated == nil || updated.InternalNotes == "" {
               return
           }
           suggestion := updated.InternalNotes
           if commentErr := s.createSystemResolutionComment(ctx, *ticket.TenantID, ticket, suggestion); commentErr != nil {
               slog.Error("failed to create system resolution comment", "ticket_id", ticket.ID, "error", commentErr)
           }
       }()
   }
   ```

**`createSystemResolutionComment`** (`ticket_service.go:632-667`):

```go
func (s *APIV1Service) createSystemResolutionComment(ctx context.Context, tenantID int32, ticket *store.Ticket, suggestion string) error {
    if !strings.HasPrefix(ticket.Description, "/m/") {
        return nil
    }
    memoUID := strings.TrimPrefix(ticket.Description, "/m/")

    parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{
        UID:      &memoUID,        // "MCSNigc5QCrgsycAnTJfih"
        TenantID: &tenantID,       // &19  ← THE PROBLEM
    })
    if err != nil || parentMemo == nil {
        return fmt.Errorf("parent memo not found: %w", err)
    }
    // ... creates comment memo, links to parent via UpsertMemoRelation ...
}
```

### Step 6: Trace the SQL Query

`GetMemo` (`store/memo.go:134`) delegates to `ListMemos`.

`ListMemos` (`store/db/sqlite/memo.go:42`) builds WHERE clauses from `FindMemo` fields:

```go
if v := find.UID; v != nil {
    where, args = append(where, "`memo`.`uid` = ?"), append(args, *v)
}
// ...
if v := find.TenantID; v != nil {
    where, args = append(where, "`memo`.`tenant_id` = ?"), append(args, *v)
}
```

When called with `UID: "MCSNigc5QCrgsycAnTJfih"` and `TenantID: &19`, the SQL becomes:
```sql
SELECT ... FROM memo WHERE memo.uid = 'MCSNigc5QCrgsycAnTJfih' AND memo.tenant_id = 19
```

But the memo has `tenant_id = NULL`. In SQL, `NULL = 19` evaluates to `NULL` (not `TRUE`), so **zero rows match**.

`GetMemo` gets `[]` from `ListMemos` → returns `(nil, nil)`.

### Step 7: Verify the Correct Pattern Exists

`findExistingEscalationTicket` (`service.go:4593`) looks up memos by UID **without** a tenant filter:

```go
memo, err := s.store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
```

This is the correct first step — UID is globally unique, no tenant filter needed. But this pattern alone is insufficient — it must be followed by an ownership check.

### Step 8: Check `getTicketComments` for Same Bug

`getTicketComments` (`ticket_service.go:600-603`):

```go
parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{
    UID:       &memoUID,
    TenantID:  ticket.TenantID,    // same bug — filters on tenant_id
})
```

Same issue: will fail to find memos with NULL `tenant_id`. Additionally, the `ListMemoRelations` call at line 611-614 has **no tenant filter**, which could mix comments across tenants when the parent memo is global/legacy.

### Step 9: Check `CreateMemoComment` for Same Bug

`CreateMemoComment` (`memo_service.go:619-622`):

```go
findMemo := &store.FindMemo{UID: &memoUID}
if tenantID := GetTenantIDFromContext(ctx); tenantID != nil {
    findMemo.TenantID = tenantID
}
relatedMemo, err := s.Store.GetMemo(ctx, findMemo)
```

Same class of bug — tenant-scoped context filters out legacy `tenant_id=NULL` memos.

### Step 10: Error Formatting Bug

```go
return fmt.Errorf("parent memo not found: %w", err)
```

When `err == nil` (memo not found, no actual error), `%w(nil)` produces `%!w(<nil>)` in the output. Additionally, the error message includes the memo UID which is an externally meaningful resource identifier that should not be leaked.

---

## Reproduction

1. Create a memo via gRPC/REST (without tenant context → `tenant_id=NULL`)
2. Create a ticket with description `/m/<memo_uid>` (in tenant context → `tenant_id=19`)
3. Ticket creation triggers RAG indexing + inference → populates `InternalNotes`
4. `createSystemResolutionComment` is called → FAILS with `"parent memo not found: %!w(<nil>)"`

---

## Root Cause Summary

| # | Bug | Location | Severity |
|---|-----|----------|----------|
| 1 | `TenantID` filter on UID-based memo lookup — memo with NULL `tenant_id` is never found | `createSystemResolutionComment` (line 638), `getTicketComments` (line 600), `CreateMemoComment` (line 619) | HIGH |
| 2 | `getTicketComments` missing `TenantID` on `ListMemoRelations` — could mix cross-tenant comments | `getTicketComments` (line 611) | HIGH |
| 3 | No ownership validation after UID-only lookup | All three functions | HIGH |
| 4 | Empty `/m/` descriptions produce needless lookups | `createSystemResolutionComment`, `getTicketComments` | MEDIUM |
| 5 | `fmt.Errorf` wraps nil error producing `%!w(<nil>)` | `createSystemResolutionComment` (line 643) | LOW |
| 6 | Error message leaks memo UID | `createSystemResolutionComment` (line 643) | LOW |

---

## Fix

### Step 0: Add shared ownership helper

**File:** `server/router/api/v1/ticket_service.go`

Add a helper that enforces the ownership rule consistently across all three call sites:

```go
// memoBelongsToTenantOrLegacy reports whether the memo is accessible from
// the given tenant context. A memo is accessible if:
//   - it has no tenant (legacy/unscoped), or
//   - its tenant matches the requested tenant.
func memoBelongsToTenantOrLegacy(memo *store.Memo, tenantID int32) bool {
    if memo.TenantID == nil {
        return true // legacy/unscoped memo — accessible to all
    }
    return *memo.TenantID == tenantID
}
```

### Step 1: Fix `createSystemResolutionComment`

**File:** `server/router/api/v1/ticket_service.go`

```go
func (s *APIV1Service) createSystemResolutionComment(ctx context.Context, tenantID int32, ticket *store.Ticket, suggestion string) error {
    if !strings.HasPrefix(ticket.Description, "/m/") {
        return nil
    }
    memoUID := strings.TrimPrefix(ticket.Description, "/m/")
    if memoUID == "" {                          // NEW: guard against empty UID
        return nil
    }

    // Resolve by UID only (globally unique), then enforce ownership in Go.
    parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{
        UID: &memoUID,
    })
    if err != nil {
        return fmt.Errorf("parent memo lookup failed: %w", err)
    }
    if parentMemo == nil {
        return fmt.Errorf("parent memo not found")
    }
    if !memoBelongsToTenantOrLegacy(parentMemo, tenantID) {  // NEW: ownership check
        return fmt.Errorf("parent memo not found")
    }

    // ... rest unchanged (creates comment memo, links to parent via UpsertMemoRelation) ...
}
```

### Step 2: Fix `getTicketComments`

**File:** `server/router/api/v1/ticket_service.go`

```go
func (s *APIV1Service) getTicketComments(ctx context.Context, ticket *store.Ticket) ([]*store.Memo, error) {
    if !strings.HasPrefix(ticket.Description, "/m/") {
        return nil, nil
    }
    memoUID := strings.TrimPrefix(ticket.Description, "/m/")
    if memoUID == "" {                          // NEW: guard against empty UID
        return nil, nil
    }

    // Resolve by UID only, then enforce ownership.
    parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{
        UID: &memoUID,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to get parent memo: %w", err)
    }
    if parentMemo == nil {
        return nil, nil
    }
    if ticket.TenantID != nil && !memoBelongsToTenantOrLegacy(parentMemo, *ticket.TenantID) {
        return nil, nil
    }

    commentType := store.MemoRelationComment
    relations, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
        RelatedMemoID: &parentMemo.ID,
        Type:          &commentType,
        TenantID:      ticket.TenantID,         // NEW: filter relations by ticket tenant
    })
    // ... rest unchanged ...
}
```

### Step 3: Fix `CreateMemoComment`

**File:** `server/router/api/v1/memo_service.go`

```go
func (s *APIV1Service) CreateMemoComment(ctx context.Context, request *v1pb.CreateMemoCommentRequest) (*v1pb.Memo, error) {
    memoUID, err := ExtractMemoUIDFromName(request.Name)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid memo name: %v", err)
    }

    // Resolve target memo by UID only, then enforce ownership in Go.
    relatedMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to get memo")
    }
    if relatedMemo == nil {
        return nil, status.Errorf(codes.NotFound, "memo not found")
    }
    if tenantID := GetTenantIDFromContext(ctx); tenantID != nil {
        if !memoBelongsToTenantOrLegacy(relatedMemo, *tenantID) {
            return nil, status.Errorf(codes.NotFound, "memo not found")
        }
    }

    // ... rest of function unchanged ...
}
```

Note: `memoBelongsToTenantOrLegacy` is defined in `ticket_service.go` but accessible within the same package `v1`.

---

## Files to Modify

| File | Changes |
|------|---------|
| `server/router/api/v1/ticket_service.go` | Add `memoBelongsToTenantOrLegacy` helper, fix `createSystemResolutionComment` (ownership check + empty UID guard + error wording), fix `getTicketComments` (ownership check + empty UID guard + tenant filter on relations) |
| `server/router/api/v1/memo_service.go` | Fix `CreateMemoComment` (UID-only lookup + ownership check) |

---

## Tests

**File:** `server/router/api/v1/ticket_service_test.go`

### Test 1: Legacy nil-tenant memo succeeds

```go
func TestCreateSystemResolutionComment_LegacyMemo(t *testing.T) {
    // Setup: create memo with tenant_id=NULL, ticket with tenant_id=19
    // Expect: createSystemResolutionComment succeeds (legacy memo accessible to all)
}
```

### Test 2: Cross-tenant memo is rejected

```go
func TestCreateSystemResolutionComment_CrossTenantReject(t *testing.T) {
    // Setup: create memo with tenant_id=20, ticket with tenant_id=19
    // Expect: createSystemResolutionComment returns error (non-nil mismatched tenant)
}
```

### Test 3: getTicketComments filters relations by tenant

```go
func TestGetTicketComments_TenantScopedRelations(t *testing.T) {
    // Setup: create legacy memo, two tickets from different tenants referencing it,
    //        comment relations written with different TenantIDs
    // Expect: each ticket only sees its own tenant's comments
}
```

### Test 4: Empty /m/ description returns nil

```go
func TestCreateSystemResolutionComment_EmptyUID(t *testing.T) {
    // Setup: ticket with Description="/m/"
    // Expect: returns nil (no error, no lookup)
}
```

### Test 5: CreateMemoComment with legacy memo

```go
func TestCreateMemoComment_LegacyParentMemo(t *testing.T) {
    // Setup: create parent memo with tenant_id=NULL
    // Expect: CreateMemoComment succeeds (legacy memo accessible)
}
```

### Verification

```bash
# Run new regression tests
go test -v -run TestCreateSystemResolutionComment ./server/router/api/v1/ -count=1
go test -v -run TestGetTicketComments ./server/router/api/v1/ -count=1
go test -v -run TestCreateMemoComment ./server/router/api/v1/ -count=1

# Run existing tests to ensure no regression
go test -v -run TestAskRovo ./server/router/api/v1/agent/ -count=1
go test -v ./server/router/api/v1/ -count=1
```

---

## Why This Is Not a Band-Aid Fix

1. **Preserves tenant isolation**: Unlike the previous plan that removed the tenant filter entirely, this version resolves by UID then validates ownership in Go. Legacy (NULL tenant) memos are accessible to all, but non-NULL mismatched tenants are rejected.

2. **Shared ownership rule**: A single `memoBelongsToTenantOrLegacy` helper enforces the same logic across all three call sites (`createSystemResolutionComment`, `getTicketComments`, `CreateMemoComment`), preventing future inconsistencies.

3. **Fixes the relations query**: `getTicketComments` now filters `ListMemoRelations` by `ticket.TenantID`, preventing cross-tenant comment mixing when multiple tenants reference the same legacy memo.

4. **Covers the same bug class**: `CreateMemoComment` is fixed with the same pattern, addressing the broader class of UID+tenant lookup failures.

5. **Defensive guards**: Empty `/m/` descriptions are handled cleanly before any store lookup.

6. **No data migration needed**: Purely in the query/logic layer — no schema changes, no reindex, no data backfill.

7. **Regression tests**: Five targeted tests cover legacy memos, cross-tenant rejection, tenant-scoped relations, and edge cases.

---

## Adversarial Review Prompt

```
You are a senior Go architect reviewing a revised bug fix plan for a
"parent memo not found" error. The fix resolves memos by globally unique
UID, then validates ownership with a memoBelongsToTenantOrLegacy helper
that allows nil-tenant legacy memos and rejects non-nil mismatched tenants.
It also adds tenant filtering to ListMemoRelations in getTicketComments
and fixes CreateMemoComment with the same pattern.

Review this revised plan critically. Focus on:

1. CORRECTNESS: Is the ownership validation logic correct? Should legacy
   (nil-tenant) memos truly be accessible from any tenant, or should they
   be restricted to the creator's context? What are the security
   implications of allowing cross-tenant access to legacy memos?

2. COMPLETENESS: Are all three call sites covered? Are there any other
   places in the codebase that look up memos by UID with a TenantID
   filter? Search for FindMemo{UID:} patterns.

3. CONSISTENCY: Does the ownership rule in memoBelongsToTenantOrLegacy
   match the system's broader tenant isolation model? Are there other
   parts of the codebase that handle nil-tenant resources differently?

4. EDGE CASES: What happens if:
   - The memo was deleted but the ticket still references it?
   - The memo UID doesn't exist at all?
   - Multiple tickets from different tenants reference the same legacy memo?
   - The ticket has no description (empty string)?
   - The description starts with /m/ but has no UID after it?
   - GetMemo returns a non-nil error AND nil memo simultaneously?

5. RELATIONS QUERY: Is adding TenantID to ListMemoRelations safe? Could
   there be existing relations written WITHOUT a TenantID that would be
   excluded by this filter? Check the UpsertMemoRelation calls.

6. RISK: What's the worst case if this fix is applied incorrectly? Could
   the ownership check be bypassed? Could the legacy memo access pattern
   be exploited for cross-tenant data exfiltration?

7. TESTING: Are the 5 regression tests sufficient? Should we add tests
   for concurrent ticket creation referencing the same memo, or for the
   RAG re-indexing path that calls getTicketComments?
```
