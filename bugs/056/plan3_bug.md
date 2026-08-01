# Plan 3: Bug 056 — "parent memo not found" Error (Final)

**Date:** 2026-08-01  
**Status:** Ready for implementation  
**Revision:** Incorporates all findings from `plan2_bug_review.md`  
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

**`createSystemResolutionComment`** (`ticket_service.go:632-668`):

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

`getTicketComments` (`ticket_service.go:595-630`):

```go
parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{
    UID:       &memoUID,
    TenantID:  ticket.TenantID,    // same bug — filters on tenant_id
})
```

Same issue: will fail to find memos with NULL `tenant_id`. Additionally, the `ListMemoRelations` call at line 620-624 has **no tenant filter**, which could mix comments across tenants when the parent memo is global/legacy.

### Step 9: Check `CreateMemoComment` for Same Bug

`CreateMemoComment` (`memo_service.go:619-662`):

```go
findMemo := &store.FindMemo{UID: &memoUID}
if tenantID := GetTenantIDFromContext(ctx); tenantID != nil {
    findMemo.TenantID = tenantID
}
relatedMemo, err := s.Store.GetMemo(ctx, findMemo)
```

Same class of bug — tenant-scoped context filters out legacy `tenant_id=NULL` memos.

### Step 10: Check `memo_relation_service.go` for Same Bug

**`SetMemoRelations`** (`memo_relation_service.go:16-72`):

```go
// Source memo lookup (lines 21-25)
findMemo := &store.FindMemo{UID: &memoUID}
if tenantID := GetTenantIDFromContext(ctx); tenantID != nil {
    findMemo.TenantID = tenantID
}
memo, err := s.Store.GetMemo(ctx, findMemo)

// Related memo lookup (lines 53-57)
findRelated := &store.FindMemo{UID: &relatedMemoUID}
if tenantID := GetTenantIDFromContext(ctx); tenantID != nil {
    findRelated.TenantID = tenantID
}
relatedMemo, err := s.Store.GetMemo(ctx, findRelated)
```

**`ListMemoRelations`** (`memo_relation_service.go:74-136`):

```go
// Lines 79-83
findMemo := &store.FindMemo{UID: &memoUID}
if tenantID := GetTenantIDFromContext(ctx); tenantID != nil {
    findMemo.TenantID = tenantID
}
memo, err := s.Store.GetMemo(ctx, findMemo)
```

All three paths conditionally apply `TenantID` from context. When context has a tenant, legacy memos (`tenant_id=NULL`) are excluded.

### Step 11: Check `ListMemoComments` for Same Bug

**`ListMemoComments`** (`memo_service.go:741-806`):

```go
// Lines 746-750
findMemo := &store.FindMemo{UID: &memoUID}
if tenantID := GetTenantIDFromContext(ctx); tenantID != nil {
    findMemo.TenantID = tenantID
}
memo, err := s.Store.GetMemo(ctx, findMemo)
```

Same pattern: conditional tenant filter from context excludes legacy memos.

### Step 12: Error Formatting Bug

```go
return fmt.Errorf("parent memo not found: %w", err)
```

When `err == nil` (memo not found, no actual error), `%w(nil)` produces `%!w(<nil>)` in the output. Additionally, the error message leaks the memo UID which is an externally meaningful resource identifier.

---

## Root Cause Summary

| # | Bug | Location | Severity |
|---|-----|----------|----------|
| 1 | `TenantID` filter on UID-based memo lookup — memo with NULL `tenant_id` is never found | `createSystemResolutionComment` (line 638), `getTicketComments` (line 600), `CreateMemoComment` (line 619), `SetMemoRelations` (lines 21, 53), `ListMemoRelations` (line 79), `ListMemoComments` (line 746) | HIGH |
| 2 | `getTicketComments` missing `TenantID` on `ListMemoRelations` — could mix cross-tenant comments | `getTicketComments` (line 620) | HIGH |
| 3 | No ownership validation after UID-only lookup | All six functions | HIGH |
| 4 | Empty `/m/` descriptions produce needless lookups | `createSystemResolutionComment`, `getTicketComments` | MEDIUM |
| 5 | `fmt.Errorf` wraps nil error producing `%!w(<nil>)` | `createSystemResolutionComment` (line 643) | LOW |
| 6 | Error message leaks memo UID | `createSystemResolutionComment` (line 643) | LOW |

---

## Fix

### Step 0: Add shared ownership helper

**File:** `server/router/api/v1/tenant_context.go`

Add a pointer-aware helper that enforces the ownership rule consistently across all call sites:

```go
// MemoBelongsToTenantOrLegacy reports whether the memo is accessible from
// the given tenant context. A memo is accessible if:
//   - it is nil (caller must handle separately), or
//   - it has no tenant (legacy/unscoped), or
//   - its tenant matches the requested tenant.
//
// tenantID == nil means no tenant context (e.g., superuser or legacy request).
// In that case, only nil-tenant (legacy) memos are accessible; tenant-scoped
// memos require an explicit tenant match.
func MemoBelongsToTenantOrLegacy(memo *store.Memo, tenantID *int32) bool {
    if memo == nil || memo.TenantID == nil {
        return true // nil memo or legacy/unscoped memo — accessible
    }
    if tenantID == nil {
        return false // tenant-scoped memo requires explicit tenant context
    }
    return *memo.TenantID == *tenantID
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
    if parentMemo == nil || !MemoBelongsToTenantOrLegacy(parentMemo, &tenantID) {
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
    if parentMemo == nil || !MemoBelongsToTenantOrLegacy(parentMemo, ticket.TenantID) {
        return nil, nil
    }

    commentType := store.MemoRelationComment
    relations, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
        RelatedMemoID: &parentMemo.ID,
        Type:          &commentType,
        TenantID:      ticket.TenantID,         // NEW: filter relations by ticket tenant
    })
    if err != nil {
        return nil, fmt.Errorf("failed to list memo relations: %w", err)
    }

    // ... rest unchanged (fetches comment memos by ID, no tenant filter needed
    // because relations are already scoped by TenantID above) ...
}
```

**Note on nil-tenant relations:** `ListMemoRelations` filtered by `TenantID: ticket.TenantID` will exclude relations where `tenant_id=NULL`. This is intentional — nil-tenant relations from legacy data are not associated with any tenant and could leak cross-tenant data if included. Tests document this behavior.

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
        if !MemoBelongsToTenantOrLegacy(relatedMemo, tenantID) {
            return nil, status.Errorf(codes.NotFound, "memo not found")
        }
    }

    // ... rest of function unchanged ...
}
```

### Step 4: Fix `SetMemoRelations`

**File:** `server/router/api/v1/memo_relation_service.go`

```go
func (s *APIV1Service) SetMemoRelations(ctx context.Context, request *v1pb.SetMemoRelationsRequest) (*v1pb.SetMemoRelationsResponse, error) {
    memoUID, err := ExtractMemoUIDFromName(request.Name)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid memo name: %v", err)
    }

    // Resolve source memo by UID only, then enforce ownership.
    memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to get memo")
    }
    if memo == nil {
        return nil, status.Errorf(codes.NotFound, "memo not found")
    }
    if tenantID := GetTenantIDFromContext(ctx); tenantID != nil {
        if !MemoBelongsToTenantOrLegacy(memo, tenantID) {
            return nil, status.Errorf(codes.NotFound, "memo not found")
        }
    }

    // Resolve related memos by UID only, then enforce ownership.
    for _, relation := range request.Relations {
        relatedMemoUID, err := ExtractMemoUIDFromName(relation.RelatedMemoName)
        if err != nil {
            return nil, status.Errorf(codes.InvalidArgument, "invalid related memo name: %v", err)
        }

        relatedMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &relatedMemoUID})
        if err != nil {
            return nil, status.Errorf(codes.Internal, "failed to get related memo")
        }
        if relatedMemo == nil {
            return nil, status.Errorf(codes.NotFound, "related memo not found")
        }
        if tenantID := GetTenantIDFromContext(ctx); tenantID != nil {
            if !MemoBelongsToTenantOrLegacy(relatedMemo, tenantID) {
                return nil, status.Errorf(codes.NotFound, "related memo not found")
            }
        }

        // ... upsert relation (use memo.ID and relatedMemo.ID) ...
    }

    // ... rest unchanged ...
}
```

### Step 5: Fix `ListMemoRelations`

**File:** `server/router/api/v1/memo_relation_service.go`

```go
func (s *APIV1Service) ListMemoRelations(ctx context.Context, request *v1pb.ListMemoRelationsRequest) (*v1pb.ListMemoRelationsResponse, error) {
    memoUID, err := ExtractMemoUIDFromName(request.Name)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid memo name: %v", err)
    }

    // Resolve source memo by UID only, then enforce ownership.
    memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to get memo")
    }
    if memo == nil {
        return nil, status.Errorf(codes.NotFound, "memo not found")
    }
    if tenantID := GetTenantIDFromContext(ctx); tenantID != nil {
        if !MemoBelongsToTenantOrLegacy(memo, tenantID) {
            return nil, status.Errorf(codes.NotFound, "memo not found")
        }
    }

    // ... rest unchanged (ListMemoRelations already scoped by memo.ID) ...
}
```

### Step 6: Fix `ListMemoComments`

**File:** `server/router/api/v1/memo_service.go`

```go
func (s *APIV1Service) ListMemoComments(ctx context.Context, request *v1pb.ListMemoCommentsRequest) (*v1pb.ListMemoCommentsResponse, error) {
    memoUID, err := ExtractMemoUIDFromName(request.ParentName)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid memo name: %v", err)
    }

    // Resolve parent memo by UID only, then enforce ownership.
    memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to get memo")
    }
    if memo == nil {
        return nil, status.Errorf(codes.NotFound, "memo not found")
    }
    if tenantID := GetTenantIDFromContext(ctx); tenantID != nil {
        if !MemoBelongsToTenantOrLegacy(memo, tenantID) {
            return nil, status.Errorf(codes.NotFound, "memo not found")
        }
    }

    // ... rest unchanged (ListMemoRelations by memo.ID, then fetch comment memos) ...
}
```

---

## Files to Modify

| File | Changes |
|------|---------|
| `server/router/api/v1/tenant_context.go` | Add `MemoBelongsToTenantOrLegacy` helper (pointer-aware, nil-safe) |
| `server/router/api/v1/ticket_service.go` | Fix `createSystemResolutionComment` (UID-only lookup + ownership check + empty UID guard + error wording), fix `getTicketComments` (same + tenant filter on relations) |
| `server/router/api/v1/memo_service.go` | Fix `CreateMemoComment` (UID-only lookup + ownership check), fix `ListMemoComments` (UID-only lookup + ownership check) |
| `server/router/api/v1/memo_relation_service.go` | Fix `SetMemoRelations` (UID-only lookups + ownership checks for source and related memos), fix `ListMemoRelations` (UID-only lookup + ownership check) |

---

## Tests

### Test file: `server/router/api/v1/ticket_service_test.go`

#### Test 1: Legacy nil-tenant memo succeeds for `createSystemResolutionComment`

```go
func TestCreateSystemResolutionComment_LegacyMemo(t *testing.T) {
    // Setup: create memo with tenant_id=NULL, ticket with tenant_id=19
    // Expect: createSystemResolutionComment succeeds (legacy memo accessible to all)
}
```

#### Test 2: Cross-tenant memo is rejected for `createSystemResolutionComment`

```go
func TestCreateSystemResolutionComment_CrossTenantReject(t *testing.T) {
    // Setup: create memo with tenant_id=20, ticket with tenant_id=19
    // Expect: createSystemResolutionComment returns error (non-nil mismatched tenant)
}
```

#### Test 3: `getTicketComments` filters relations by tenant

```go
func TestGetTicketComments_TenantScopedRelations(t *testing.T) {
    // Setup: create legacy memo, two tickets from different tenants referencing it,
    //        comment relations written with different TenantIDs
    // Expect: each ticket only sees its own tenant's comments; nil-tenant relations excluded
}
```

#### Test 4: Empty `/m/` description returns nil for `createSystemResolutionComment`

```go
func TestCreateSystemResolutionComment_EmptyUID(t *testing.T) {
    // Setup: ticket with Description="/m/"
    // Expect: returns nil (no error, no lookup)
}
```

#### Test 5: `getTicketComments` with empty `/m/` description returns nil

```go
func TestGetTicketComments_EmptyUID(t *testing.T) {
    // Setup: ticket with Description="/m/"
    // Expect: returns (nil, nil)
}
```

### Test file: `server/router/api/v1/memo_service_test.go`

#### Test 6: `CreateMemoComment` with legacy parent memo

```go
func TestCreateMemoComment_LegacyParentMemo(t *testing.T) {
    // Setup: create parent memo with tenant_id=NULL
    // Expect: CreateMemoComment succeeds (legacy memo accessible)
}
```

#### Test 7: `CreateMemoComment` with cross-tenant parent memo is rejected

```go
func TestCreateMemoComment_CrossTenantReject(t *testing.T) {
    // Setup: create parent memo with tenant_id=20, request in tenant_id=19 context
    // Expect: CreateMemoComment returns codes.NotFound
}
```

#### Test 8: `ListMemoComments` with legacy parent memo

```go
func TestListMemoComments_LegacyParentMemo(t *testing.T) {
    // Setup: create parent memo with tenant_id=NULL, add comment relation
    // Expect: ListMemoComments succeeds and returns comments
}
```

#### Test 9: `ListMemoComments` with cross-tenant parent memo is rejected

```go
func TestListMemoComments_CrossTenantReject(t *testing.T) {
    // Setup: create parent memo with tenant_id=20, request in tenant_id=19 context
    // Expect: ListMemoComments returns codes.NotFound
}
```

### Test file: `server/router/api/v1/memo_relation_service_test.go`

#### Test 10: `SetMemoRelations` with legacy source memo

```go
func TestSetMemoRelations_LegacySourceMemo(t *testing.T) {
    // Setup: create source memo with tenant_id=NULL, related memo with tenant_id=19
    // Expect: SetMemoRelations succeeds
}
```

#### Test 11: `SetMemoRelations` with legacy related memo

```go
func TestSetMemoRelations_LegacyRelatedMemo(t *testing.T) {
    // Setup: create source memo with tenant_id=19, related memo with tenant_id=NULL
    // Expect: SetMemoRelations succeeds
}
```

#### Test 12: `SetMemoRelations` cross-tenant rejection

```go
func TestSetMemoRelations_CrossTenantReject(t *testing.T) {
    // Setup: create source memo with tenant_id=20, related memo with tenant_id=19
    //        request in tenant_id=19 context
    // Expect: SetMemoRelations returns codes.NotFound
}
```

#### Test 13: `ListMemoRelations` with legacy source memo

```go
func TestListMemoRelations_LegacySourceMemo(t *testing.T) {
    // Setup: create source memo with tenant_id=NULL, add relation
    // Expect: ListMemoRelations succeeds and returns relations
}
```

#### Test 14: `ListMemoRelations` cross-tenant rejection

```go
func TestListMemoRelations_CrossTenantReject(t *testing.T) {
    // Setup: create source memo with tenant_id=20, request in tenant_id=19 context
    // Expect: ListMemoRelations returns codes.NotFound
}
```

---

## Verification

```bash
# Run new regression tests
go test -v -run TestCreateSystemResolutionComment ./server/router/api/v1/ -count=1
go test -v -run TestGetTicketComments ./server/router/api/v1/ -count=1
go test -v -run TestCreateMemoComment ./server/router/api/v1/ -count=1
go test -v -run TestListMemoComments ./server/router/api/v1/ -count=1
go test -v -run TestSetMemoRelations ./server/router/api/v1/ -count=1
go test -v -run TestListMemoRelations ./server/router/api/v1/ -count=1

# Run existing tests to ensure no regression
go test -v ./server/router/api/v1/ -count=1
```

---

## Why This Is Not a Band-Aid Fix

1. **Preserves tenant isolation**: Unlike removing the tenant filter entirely, this version resolves by UID then validates ownership in Go. Legacy (NULL tenant) memos are accessible to all, but non-NULL mismatched tenants are rejected.

2. **Shared ownership rule**: A single `MemoBelongsToTenantOrLegacy` helper in `tenant_context.go` enforces the same logic across all six call sites, preventing future inconsistencies.

3. **Fixes the relations query**: `getTicketComments` now filters `ListMemoRelations` by `ticket.TenantID`, preventing cross-tenant comment mixing when multiple tenants reference the same legacy memo. Nil-tenant relations are intentionally excluded for security.

4. **Covers the entire bug class**: All six paths that combine UID lookup with tenant filtering are fixed: `createSystemResolutionComment`, `getTicketComments`, `CreateMemoComment`, `ListMemoComments`, `SetMemoRelations`, `ListMemoRelations`.

5. **Defensive guards**: Empty `/m/` descriptions and `/m/`-prefixed empty UIDs are handled cleanly before any store lookup.

6. **No data migration needed**: Purely in the query/logic layer — no schema changes, no reindex, no data backfill.

7. **Regression tests**: Fourteen targeted tests cover legacy memos, cross-tenant rejection, tenant-scoped relations, and edge cases across all touched paths.
