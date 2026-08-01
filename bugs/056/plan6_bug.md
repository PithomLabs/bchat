# Plan 6: Bug 056 — "parent memo not found" Error (Final)

**Date:** 2026-08-01  
**Status:** Ready for implementation  
**Revision:** Incorporates all findings from `plan5_bug_review.md`  
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

Same issue: will fail to find memos with NULL `tenant_id`. Additionally, the `ListMemoRelations` call at line 611-614 has **no tenant filter**, which could mix comments across tenants when the parent memo is global/legacy.

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

### Step 13: Check `convertMemoRelationFromStore` for Nil Dereference

`convertMemoRelationFromStore` (`memo_relation_service.go:138-176`) loads relation endpoints with tenant filters derived from the relation itself:

```go
findMemo := &store.FindMemo{ID: &memoRelation.MemoID}
if memoRelation.TenantID != nil {
    findMemo.TenantID = memoRelation.TenantID
}
memo, err := s.Store.GetMemo(ctx, findMemo)
// memo.Content dereferenced later without nil check
```

If a relation has `tenant_id=19` but points to a legacy memo with `tenant_id=NULL`, the ID+tenant lookup returns nil and the converter dereferences nil. This is not fixed by changing the initial UID lookup in `ListMemoRelations`.

### Step 14: Check Relation Delete/List Scoping in `SetMemoRelations` and `ListMemoRelations`

**`SetMemoRelations`** (`memo_relation_service.go:31-35`) deletes existing reference relations using `memo.TenantID`:

```go
if err := s.Store.DeleteMemoRelation(ctx, &store.DeleteMemoRelation{
    MemoID:   &memo.ID,
    Type:     &referenceType,
    TenantID: memo.TenantID,     // nil for legacy memos → unscoped delete
}); err != nil {
    return nil, status.Errorf(codes.Internal, "failed to delete memo relation")
}
```

For a legacy source memo, this is an unscoped delete that can remove relations belonging to other tenants.

**`ListMemoRelations`** (`memo_relation_service.go:101-120`) queries relations using `memo.TenantID`:

```go
tempList, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
    MemoID:     &memo.ID,
    MemoFilter: memoFilter,
    TenantID:   memo.TenantID,   // nil for legacy memos → unscoped list
})
```

For a legacy source memo, this returns relations across all tenants.

### Step 15: Check `GetMemo` Scoped-Admin Behavior

`GetMemo` (`memo_service.go:253-322`) has scoped-admin logic:

```go
if user != nil && !isSuperUser(user) && tenantID == nil {
    allowedTenantIDs := deriveTenantIDsForScopedAdmin(ctx, s.Store, user)
    if allowedTenantIDs != nil {
        if memo.TenantID == nil {
            // Legacy memo with no tenant — scoped admins cannot access global memos
            return nil, status.Errorf(codes.PermissionDenied, "permission denied")
        }
        // Check if memo's tenant is in the allowed list
        // ...
    }
}
```

Scoped admins without tenant context are **denied** access to legacy nil-tenant memos. Any replacement helper must preserve this behavior.

### Step 16: Document `ListMemoComments` Nil-Tenant Comment Memo Behavior

Existing `ListMemoComments` code filters comment relations by request tenant when tenant context exists, and filters each fetched comment memo by the same tenant. That means a tenant-scoped relation pointing to a nil-tenant legacy comment memo will be excluded. This is the safer default for tenant isolation and should be documented.

---

## Root Cause Summary

| # | Bug | Location | Severity |
|---|-----|----------|----------|
| 1 | `TenantID` filter on UID-based memo lookup — memo with NULL `tenant_id` is never found | `createSystemResolutionComment` (line 638), `getTicketComments` (line 600), `CreateMemoComment` (line 619), `SetMemoRelations` (lines 21, 53), `ListMemoRelations` (line 79), `ListMemoComments` (line 746) | HIGH |
| 2 | `getTicketComments` missing `TenantID` on `ListMemoRelations` — could mix cross-tenant comments | `getTicketComments` (line 611) | HIGH |
| 3 | No ownership validation after UID-only lookup | All six functions | HIGH |
| 4 | `SetMemoRelations` unscoped delete for legacy source memos can remove cross-tenant relations | `SetMemoRelations` (line 34) | HIGH |
| 5 | `ListMemoRelations` unscoped list for legacy source memos can expose cross-tenant relations | `ListMemoRelations` (lines 101, 116) | HIGH |
| 6 | `convertMemoRelationFromStore` can nil-dereference when relation's tenant-scoped endpoint is a legacy memo | `memo_relation_service.go` converter | MEDIUM |
| 7 | `ListMemoComments` excludes nil-tenant legacy comment memos under tenant-scoped requests | `ListMemoComments` existing behavior | MEDIUM |
| 8 | Empty `/m/` descriptions produce needless lookups | `createSystemResolutionComment`, `getTicketComments` | MEDIUM |
| 9 | `fmt.Errorf` wraps nil error producing `%!w(<nil>)` | `createSystemResolutionComment` (line 643) | LOW |
| 10 | Error message leaks memo UID | `createSystemResolutionComment` (line 643) | LOW |

---

## Fix

### Step 0: Add shared ownership helpers

**File:** `server/router/api/v1/tenant_context.go`

Add three helpers. The first is for ticket-internal paths that have a concrete `tenantID int32` but no user object. The second is for gRPC memo APIs that have both user and tenant context, mirroring `GetMemo` semantics including scoped-admin restrictions. The third derives the effective tenant ID for relation operations.

```go
// MemoBelongsToTenantOrLegacy reports whether the memo is accessible from
// the given tenant context. A memo is accessible if:
//   - it has no tenant (legacy/unscoped), or
//   - its tenant matches the requested tenant.
//
// Returns false for nil memo (caller must handle separately).
// Returns false when tenantID is nil and the memo is tenant-scoped,
// because tenant-scoped memos require an explicit tenant match.
func MemoBelongsToTenantOrLegacy(memo *store.Memo, tenantID *int32) bool {
    if memo == nil {
        return false
    }
    if memo.TenantID == nil {
        return true // legacy/unscoped memo — accessible to all tenants
    }
    if tenantID == nil {
        return false // tenant-scoped memo requires explicit tenant context
    }
    return *memo.TenantID == *tenantID
}

// MemoIsAccessible reports whether the memo is accessible to the given user
// in the given tenant context. A memo is accessible if:
//   - it has no tenant (legacy/unscoped) and the user is not a scoped admin
//     without tenant context, or
//   - its tenant matches the requested tenant, or
//   - the user is a superuser who may access tenant-scoped content without
//     an explicit tenant match.
//
// Returns false for nil memo (caller must handle separately).
// Mirrors the access rules in GetMemo, including scoped-admin restrictions.
func MemoIsAccessible(ctx context.Context, s *store.Store, memo *store.Memo, user *store.User, tenantID *int32) bool {
    if memo == nil {
        return false
    }
    if memo.TenantID == nil {
        // Legacy memo: accessible to all except scoped admins without tenant context.
        if user != nil && !isSuperUser(user) && tenantID == nil {
            allowedTenantIDs := deriveTenantIDsForScopedAdmin(ctx, s, user)
            if allowedTenantIDs != nil {
                return false // scoped admin without tenant context cannot access global memos
            }
        }
        return true
    }
    if tenantID != nil && *memo.TenantID == *tenantID {
        return true
    }
    if user != nil && isSuperUser(user) {
        return true // superuser bypass, matching existing GetMemo behavior
    }
    return false
}

// RelationTenantID returns the tenant ID to use for relation operations
// (delete, upsert, list) given the request context and the source memo.
// The request tenant takes precedence; the memo tenant is used only when
// the request has no tenant context (e.g., unscoped superuser).
//
// Note: for non-super unscoped callers, MemoIsAccessible will have already
// denied access to legacy memos, so reaching this helper with nil tenantID
// implies the caller is a superuser or the memo is tenant-scoped.
func RelationTenantID(ctx context.Context, memo *store.Memo) *int32 {
    if requestTenantID := GetTenantIDFromContext(ctx); requestTenantID != nil {
        return requestTenantID
    }
    return memo.TenantID
}
```

**Rationale for three helpers:**
- `MemoBelongsToTenantOrLegacy`: strict tenant-only check for ticket-internal goroutines where no user object exists and superuser access must not be silently granted. Returns generic not-found errors.
- `MemoIsAccessible`: user-aware check for gRPC memo APIs, preserving `GetMemo` semantics including scoped-admin restrictions on legacy memos. Returns `PermissionDenied` for resolved-but-inaccessible memos.
- `RelationTenantID`: ensures relation operations are scoped to the request tenant when present, preventing cross-tenant relation deletion/exposure for legacy source memos. Only safe to return nil for true superusers because `MemoIsAccessible` gates non-super unscoped access first.

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

    comment, err := s.Store.CreateMemo(ctx, &store.Memo{
        // ... create system comment memo ...
    })
    if err != nil {
        return fmt.Errorf("failed to create comment memo: %w", err)
    }

    // Link comment to parent memo. TenantID is set from the ticket so the
    // relation is scoped to the requesting tenant.
    _, err = s.Store.UpsertMemoRelation(ctx, &store.MemoRelation{
        MemoID:        comment.ID,
        RelatedMemoID: parentMemo.ID,
        Type:          store.MemoRelationComment,
        TenantID:      &tenantID,
    })
    if err != nil {
        return fmt.Errorf("failed to create memo relation: %w", err)
    }
    return nil
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

    // ... fetch comment memos by ID (unchanged) ...
    var comments []*store.Memo
    for _, rel := range relations {
        memo, err := s.Store.GetMemo(ctx, &store.FindMemo{ID: &rel.MemoID})
        if err != nil || memo == nil {
            continue
        }
        comments = append(comments, memo)
    }
    return comments, nil
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
    user, err := s.GetCurrentUser(ctx)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to get user")
    }
    if !MemoIsAccessible(ctx, s.Store, relatedMemo, user, GetTenantIDFromContext(ctx)) {
        return nil, status.Errorf(codes.NotFound, "memo not found")
    }

    // Create the comment memo. CreateMemo sets TenantID from context, so the
    // new comment inherits the caller's tenant (or remains nil if no tenant).
    memoComment, err := s.CreateMemo(ctx, &v1pb.CreateMemoRequest{
        Memo: request.Comment,
    })
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to create comment: %v", err)
    }

    // NOTE: CreateMemo returns a *v1pb.Memo which does not expose the int32 ID
    // required by store.MemoRelation. We must re-fetch by UID to obtain memo.ID.
    // This lookup intentionally preserves the existing tenant filter because the
    // comment was just created in the same context and should match.
    commentUID, err := ExtractMemoUIDFromName(memoComment.Name)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "invalid created memo name: %v", err)
    }
    findComment := &store.FindMemo{UID: &commentUID}
    if tenantID := GetTenantIDFromContext(ctx); tenantID != nil {
        findComment.TenantID = tenantID
    }
    commentStoreMemo, err := s.Store.GetMemo(ctx, findComment)
    if err != nil || commentStoreMemo == nil {
        return nil, status.Errorf(codes.Internal, "failed to fetch created comment memo")
    }

    _, err = s.Store.UpsertMemoRelation(ctx, &store.MemoRelation{
        MemoID:        commentStoreMemo.ID,
        RelatedMemoID: relatedMemo.ID,
        Type:          store.MemoRelationComment,
        TenantID:      GetTenantIDFromContext(ctx),
    })
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to create memo relation: %v", err)
    }

    return memoComment, nil
}
```

### Step 4: Fix `SetMemoRelations`

**File:** `server/router/api/v1/memo_relation_service.go`

```go
func (s *APIV1Service) SetMemoRelations(ctx context.Context, request *v1pb.SetMemoRelationsRequest) (*emptypb.Empty, error) {
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
    user, err := s.GetCurrentUser(ctx)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to get user")
    }
    if !MemoIsAccessible(ctx, s.Store, memo, user, GetTenantIDFromContext(ctx)) {
        return nil, status.Errorf(codes.PermissionDenied, "permission denied")
    }

    // For relation operations, prefer the request tenant over the memo tenant.
    // This prevents a tenant-scoped SetMemoRelations call on a legacy memo from
    // deleting or exposing relations belonging to other tenants.
    relationTenantID := RelationTenantID(ctx, memo)

    referenceType := store.MemoRelationReference
    // Delete all reference relations scoped to the effective tenant.
    if err := s.Store.DeleteMemoRelation(ctx, &store.DeleteMemoRelation{
        MemoID:   &memo.ID,
        Type:     &referenceType,
        TenantID: relationTenantID,
    }); err != nil {
        return nil, status.Errorf(codes.Internal, "failed to delete memo relation")
    }

    // Resolve related memos by UID only, then enforce ownership.
    for _, relation := range request.Relations {
        // Ignore reflexive relations.
        if request.Name == relation.RelatedMemo.Name {
            continue
        }
        // Ignore comment relations as there's no need to update a comment's relation.
        // Inserting/Deleting a comment is handled elsewhere.
        if relation.Type == v1pb.MemoRelation_COMMENT {
            continue
        }

        relatedMemoUID, err := ExtractMemoUIDFromName(relation.RelatedMemo.Name)
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
        if !MemoIsAccessible(ctx, s.Store, relatedMemo, user, GetTenantIDFromContext(ctx)) {
            return nil, status.Errorf(codes.PermissionDenied, "permission denied")
        }

        // Upsert relation using int32 IDs from resolved memos.
        _, err = s.Store.UpsertMemoRelation(ctx, &store.MemoRelation{
            MemoID:        memo.ID,
            RelatedMemoID: relatedMemo.ID,
            Type:          convertMemoRelationTypeToStore(relation.Type),
            TenantID:      relationTenantID,
        })
        if err != nil {
            return nil, status.Errorf(codes.Internal, "failed to upsert memo relation: %v", err)
        }
    }

    return &emptypb.Empty{}, nil
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
    user, err := s.GetCurrentUser(ctx)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to get user")
    }
    if !MemoIsAccessible(ctx, s.Store, memo, user, GetTenantIDFromContext(ctx)) {
        return nil, status.Errorf(codes.PermissionDenied, "permission denied")
    }

    // For relation queries, prefer the request tenant over the memo tenant.
    // This prevents a tenant-scoped ListMemoRelations call on a legacy memo from
    // exposing relations belonging to other tenants.
    relationTenantID := RelationTenantID(ctx, memo)

    currentUser := user
    var memoFilter *string
    if currentUser == nil {
        filterStr := `visibility == "PUBLIC"`
        memoFilter = &filterStr
    } else if !isSuperUser(currentUser) {
        filterStr := fmt.Sprintf(`creator_id == %d || visibility in ["PUBLIC", "PROTECTED"]`, currentUser.ID)
        memoFilter = &filterStr
    }
    relationList := []*v1pb.MemoRelation{}
    tempList, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
        MemoID:     &memo.ID,
        MemoFilter: memoFilter,
        TenantID:   relationTenantID,
    })
    if err != nil {
        return nil, err
    }
    for _, raw := range tempList {
        relation, err := s.convertMemoRelationFromStore(ctx, raw)
        if err != nil {
            return nil, status.Errorf(codes.Internal, "failed to convert memo relation")
        }
        relationList = append(relationList, relation)
    }
    tempList, err = s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
        RelatedMemoID: &memo.ID,
        MemoFilter:    memoFilter,
        TenantID:      relationTenantID,
    })
    if err != nil {
        return nil, err
    }
    for _, raw := range tempList {
        relation, err := s.convertMemoRelationFromStore(ctx, raw)
        if err != nil {
            return nil, status.Errorf(codes.Internal, "failed to convert memo relation")
        }
        relationList = append(relationList, relation)
    }

    response := &v1pb.ListMemoRelationsResponse{
        Relations: relationList,
    }
    return response, nil
}
```

### Step 6: Fix `ListMemoComments`

**File:** `server/router/api/v1/memo_service.go`

```go
func (s *APIV1Service) ListMemoComments(ctx context.Context, request *v1pb.ListMemoCommentsRequest) (*v1pb.ListMemoCommentsResponse, error) {
    memoUID, err := ExtractMemoUIDFromName(request.Name)
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
    user, err := s.GetCurrentUser(ctx)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to get user")
    }
    if !MemoIsAccessible(ctx, s.Store, memo, user, GetTenantIDFromContext(ctx)) {
        return nil, status.Errorf(codes.PermissionDenied, "permission denied")
    }

    // Note: existing downstream code filters comment relations by request tenant
    // when tenant context exists, and filters each fetched comment memo by the
    // same tenant. That means nil-tenant legacy comment memos are excluded from
    // tenant-scoped ListMemoComments responses. This is the safer default for
    // tenant isolation.

    // ... rest unchanged (ListMemoRelations by memo.ID, then fetch comment memos) ...
}
```

### Step 7: Fix `convertMemoRelationFromStore`

**File:** `server/router/api/v1/memo_relation_service.go`

Update the converter to remove tenant filters from ID lookups and add nil safety. The `ListMemoRelations` handler already verified the source memo is accessible before calling the converter. Removing the tenant filter from the converter's internal lookups ensures legacy memos are not silently dropped during response construction. Ownership was already enforced at the handler level.

```go
func (s *APIV1Service) convertMemoRelationFromStore(ctx context.Context, memoRelation *store.MemoRelation) (*v1pb.MemoRelation, error) {
    // Resolve source memo by ID only (tenant filter removed to support legacy memos).
    findMemo := &store.FindMemo{ID: &memoRelation.MemoID}
    memo, err := s.Store.GetMemo(ctx, findMemo)
    if err != nil || memo == nil {
        return nil, status.Errorf(codes.Internal, "source memo not found for relation")
    }
    memoSnippet, err := getMemoContentSnippet(memo.Content)
    if err != nil {
        return nil, errors.Wrap(err, "failed to get memo content snippet")
    }

    // Resolve related memo by ID only (tenant filter removed to support legacy memos).
    findRelated := &store.FindMemo{ID: &memoRelation.RelatedMemoID}
    relatedMemo, err := s.Store.GetMemo(ctx, findRelated)
    if err != nil || relatedMemo == nil {
        return nil, status.Errorf(codes.Internal, "related memo not found for relation")
    }
    relatedMemoSnippet, err := getMemoContentSnippet(relatedMemo.Content)
    if err != nil {
        return nil, errors.Wrap(err, "failed to get related memo content snippet")
    }

    return &v1pb.MemoRelation{
        Memo: &v1pb.MemoRelation_Memo{
            Name:    fmt.Sprintf("%s%s", MemoNamePrefix, memo.UID),
            Uid:     memo.UID,
            Snippet: memoSnippet,
        },
        RelatedMemo: &v1pb.MemoRelation_Memo{
            Name:    fmt.Sprintf("%s%s", MemoNamePrefix, relatedMemo.UID),
            Uid:     relatedMemo.UID,
            Snippet: relatedMemoSnippet,
        },
        Type: convertMemoRelationTypeFromStore(memoRelation.Type),
    }, nil
}
```

---

## Files to Modify

| File | Changes |
|------|---------|
| `server/router/api/v1/tenant_context.go` | Add `MemoBelongsToTenantOrLegacy`, `MemoIsAccessible`, and `RelationTenantID` helpers |
| `server/router/api/v1/ticket_service.go` | Fix `createSystemResolutionComment` (UID-only lookup + ownership check + empty UID guard + error wording), fix `getTicketComments` (same + tenant filter on relations) |
| `server/router/api/v1/memo_service.go` | Fix `CreateMemoComment` (UID-only lookup + user-aware ownership check + documented post-create re-fetch), fix `ListMemoComments` (UID-only lookup + user-aware ownership check) |
| `server/router/api/v1/memo_relation_service.go` | Fix `SetMemoRelations` (UID-only lookups + user-aware ownership checks + relation tenant scoping for deletes/upserts + correct enum conversion), fix `ListMemoRelations` (UID-only lookup + user-aware ownership check + relation tenant scoping for list queries), fix `convertMemoRelationFromStore` (remove tenant filter from ID lookups + nil safety + preserve snippet truncation) |

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
    // Expect: ListMemoComments returns codes.PermissionDenied (matching GetMemo behavior)
}
```

#### Test 10: `ListMemoComments` excludes nil-tenant legacy comment memos under tenant-scoped request

```go
func TestListMemoComments_NilTenantCommentExcluded(t *testing.T) {
    // Setup: create parent memo tenant_id=19, create comment memo with tenant_id=NULL,
    //        create comment relation with tenant_id=19 pointing to nil-tenant comment
    // Expect: ListMemoComments in tenant 19 context excludes the nil-tenant comment memo
}
```

### Test file: `server/router/api/v1/memo_relation_service_test.go`

#### Test 11: `SetMemoRelations` with legacy source memo

```go
func TestSetMemoRelations_LegacySourceMemo(t *testing.T) {
    // Setup: create source memo with tenant_id=NULL, related memo with tenant_id=19
    // Expect: SetMemoRelations succeeds
}
```

#### Test 12: `SetMemoRelations` with legacy related memo

```go
func TestSetMemoRelations_LegacyRelatedMemo(t *testing.T) {
    // Setup: create source memo with tenant_id=19, related memo with tenant_id=NULL
    // Expect: SetMemoRelations succeeds
}
```

#### Test 13: `SetMemoRelations` cross-tenant rejection

```go
func TestSetMemoRelations_CrossTenantReject(t *testing.T) {
    // Setup: create source memo with tenant_id=20, related memo with tenant_id=19
    //        request in tenant_id=19 context
    // Expect: SetMemoRelations returns codes.PermissionDenied (matching GetMemo behavior)
}
```

#### Test 14: `SetMemoRelations` with legacy source memo deletes only request tenant's relations

```go
func TestSetMemoRelations_LegacySourceMemo_RelationIsolation(t *testing.T) {
    // Setup: create legacy source memo, add reference relations from tenant 19 and tenant 20
    //        call SetMemoRelations in tenant 19 context with new relations
    // Expect: tenant 19's old relations are deleted, tenant 20's relations remain
}
```

#### Test 15: `ListMemoRelations` with legacy source memo returns only request tenant's relations

```go
func TestListMemoRelations_LegacySourceMemo_RelationIsolation(t *testing.T) {
    // Setup: create legacy source memo, add reference relations from tenant 19 and tenant 20
    //        call ListMemoRelations in tenant 19 context
    // Expect: only tenant 19's relations are returned
}
```

#### Test 16: `ListMemoRelations` with legacy source memo

```go
func TestListMemoRelations_LegacySourceMemo(t *testing.T) {
    // Setup: create source memo with tenant_id=NULL, add relation
    // Expect: ListMemoRelations succeeds and returns relations
}
```

#### Test 17: `ListMemoRelations` cross-tenant rejection

```go
func TestListMemoRelations_CrossTenantReject(t *testing.T) {
    // Setup: create source memo with tenant_id=20, request in tenant_id=19 context
    // Expect: ListMemoRelations returns codes.PermissionDenied (matching GetMemo behavior)
}
```

#### Test 18: `convertMemoRelationFromStore` with legacy endpoint memo

```go
func TestConvertMemoRelationFromStore_LegacyEndpoint(t *testing.T) {
    // Setup: create relation with tenant_id=19, source memo tenant_id=19,
    //        related memo tenant_id=NULL (legacy)
    // Expect: convertMemoRelationFromStore succeeds (ID-only lookup finds legacy memo)
    //         and snippets are truncated via getMemoContentSnippet (max 64 chars)
}
```

#### Test 19: Superuser can access tenant-scoped memo via `SetMemoRelations`

```go
func TestSetMemoRelations_SuperuserAccess(t *testing.T) {
    // Setup: create memo with tenant_id=20, authenticate as superuser (no tenant context)
    // Expect: SetMemoRelations succeeds (superuser bypass matches existing GetMemo behavior)
}
```

#### Test 20: Superuser can access tenant-scoped memo via `ListMemoRelations`

```go
func TestListMemoRelations_SuperuserAccess(t *testing.T) {
    // Setup: create source memo with tenant_id=20, add relation, authenticate as superuser
    // Expect: ListMemoRelations succeeds
}
```

#### Test 21: Scoped admin cannot access legacy memo via `SetMemoRelations`

```go
func TestSetMemoRelations_ScopedAdminLegacyDeny(t *testing.T) {
    // Setup: create memo with tenant_id=NULL, authenticate as scoped admin (no tenant context)
    // Expect: SetMemoRelations returns codes.PermissionDenied (matching GetMemo behavior)
}
```

#### Test 22: Scoped admin cannot access legacy memo via `ListMemoRelations`

```go
func TestListMemoRelations_ScopedAdminLegacyDeny(t *testing.T) {
    // Setup: create source memo with tenant_id=NULL, add relation, authenticate as scoped admin
    // Expect: ListMemoRelations returns codes.PermissionDenied
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

2. **Three-tier access control**: `MemoBelongsToTenantOrLegacy` enforces strict tenant matching for ticket-internal paths. `MemoIsAccessible` preserves existing superuser bypass and scoped-admin restrictions for gRPC memo APIs, matching `GetMemo` semantics. `RelationTenantID` ensures relation operations are scoped to the request tenant, preventing cross-tenant relation leakage for legacy memos.

3. **Fixes the relations query**: `getTicketComments` now filters `ListMemoRelations` by `ticket.TenantID`, preventing cross-tenant comment mixing when multiple tenants reference the same legacy memo. Nil-tenant relations are intentionally excluded for security.

4. **Covers the entire bug class**: All six paths that combine UID lookup with tenant filtering are fixed: `createSystemResolutionComment`, `getTicketComments`, `CreateMemoComment`, `ListMemoComments`, `SetMemoRelations`, `ListMemoRelations`.

5. **Fixes relation scoping gaps**: `SetMemoRelations` now uses `RelationTenantID` for both delete and upsert operations, preventing cross-tenant relation deletion for legacy source memos. `ListMemoRelations` uses `RelationTenantID` for both outgoing and incoming relation queries, preventing cross-tenant relation exposure.

6. **Fixes converter nil-dereference**: `convertMemoRelationFromStore` no longer applies tenant filters to its internal ID lookups, preventing nil dereference when a tenant-scoped relation points to a legacy memo endpoint. It preserves `getMemoContentSnippet` truncation behavior.

7. **Defensive guards**: Empty `/m/` descriptions and `/m/`-prefixed empty UIDs are handled cleanly before any store lookup.

8. **No data migration needed**: Purely in the query/logic layer — no schema changes, no reindex, no data backfill.

9. **Regression tests**: Twenty-two targeted tests cover the original ticket regression, cross-tenant rejection, tenant-scoped relations, superuser access, scoped-admin restrictions, relation isolation, nil-tenant relation endpoints, nil-tenant comment memo exclusion, and edge cases across all touched paths.
