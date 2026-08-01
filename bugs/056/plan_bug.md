# Plan: Bug 056 — "parent memo not found" Error on Manual Ticket Creation

**Date:** 2026-08-01
**Status:** Ready for implementation
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

This is the correct pattern — UID is globally unique, no tenant filter needed.

### Step 8: Check `getTicketComments` for Same Bug

`getTicketComments` (`ticket_service.go:600-603`):

```go
parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{
    UID:       &memoUID,
    TenantID:  ticket.TenantID,    // same bug — filters on tenant_id
})
```

Same issue: will fail to find memos with NULL `tenant_id`. The error handling is better (returns `nil, nil` instead of wrapping nil), but the lookup is still wrong.

### Step 9: Check How Memo Got NULL tenant_id

`CreateMemo` (`memo_service.go:53`):
```go
if tenantID := GetTenantIDFromContext(ctx); tenantID != nil {
    create.TenantID = tenantID
}
```

The memo only gets a `tenant_id` if the context has one. If the gRPC call was made without tenant context (e.g., superuser session, or different auth flow), the memo gets `NULL`.

This is a separate concern from the immediate bug — the fix should not depend on how the memo was created.

### Step 10: Error Formatting Bug

```go
return fmt.Errorf("parent memo not found: %w", err)
```

When `err == nil` (memo not found, no actual error), `%w(nil)` produces `%!w(<nil>)` in the output. This is a secondary bug — the nil-error case should be handled separately.

---

## Reproduction

1. Create a memo via gRPC/REST (without tenant context → `tenant_id=NULL`)
2. Create a ticket with description `/m/<memo_uid>` (in tenant context → `tenant_id=19`)
3. Ticket creation triggers RAG indexing + inference → populates `InternalNotes`
4. `createSystemResolutionComment` is called → FAILS with `"parent memo not found: %!w(<nil>)"`

---

## Root Cause Summary

Two bugs in `ticket_service.go`:

| # | Bug | Location | Severity |
|---|-----|----------|----------|
| 1 | `TenantID` filter on UID-based memo lookup — memo with NULL tenant_id is never found | `createSystemResolutionComment` (line 638), `getTicketComments` (line 600) | HIGH |
| 2 | `fmt.Errorf` wraps nil error producing `%!w(<nil>)` | `createSystemResolutionComment` (line 643) | LOW |

---

## Affected Code Locations

| Location | Function | Issue |
|----------|----------|-------|
| `ticket_service.go:638-641` | `createSystemResolutionComment` | `TenantID: &tenantID` in `GetMemo` lookup |
| `ticket_service.go:642-643` | `createSystemResolutionComment` | Wraps nil error in `fmt.Errorf` |
| `ticket_service.go:600-603` | `getTicketComments` | `TenantID: ticket.TenantID` in `GetMemo` lookup |

---

## Fix

### Step 1: Remove TenantID from `createSystemResolutionComment` memo lookup

**File:** `server/router/api/v1/ticket_service.go`

```go
// Before (line 638-641):
parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{
    UID:      &memoUID,
    TenantID: &tenantID,
})

// After:
parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{
    UID: &memoUID,
})
```

### Step 2: Fix error handling for nil err

**File:** `server/router/api/v1/ticket_service.go`

```go
// Before (line 642-643):
if err != nil || parentMemo == nil {
    return fmt.Errorf("parent memo not found: %w", err)
}

// After:
if err != nil {
    return fmt.Errorf("parent memo not found: %w", err)
}
if parentMemo == nil {
    return fmt.Errorf("parent memo not found: no memo with uid %s", memoUID)
}
```

### Step 3: Remove TenantID from `getTicketComments` memo lookup

**File:** `server/router/api/v1/ticket_service.go`

```go
// Before (line 600-603):
parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{
    UID:       &memoUID,
    TenantID:  ticket.TenantID,
})

// After:
parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{
    UID: &memoUID,
})
```

---

## Files to Modify

| File | Changes |
|------|---------|
| `server/router/api/v1/ticket_service.go` | Remove `TenantID` from 2 memo lookups, fix error handling |

---

## Why This Is Not a Band-Aid Fix

1. **Correct architectural principle**: Memo UIDs are globally unique identifiers. Filtering by tenant on a UID lookup is redundant and incorrect — it assumes the memo was created in the same tenant context as the ticket, which is not guaranteed.

2. **Follows existing correct pattern**: `findExistingEscalationTicket` (service.go:4593) already uses UID-only lookup without tenant filter. This fix aligns `createSystemResolutionComment` and `getTicketComments` with the correct pattern.

3. **No data migration needed**: The fix is purely in the query layer — no schema changes, no reindex, no data backfill.

4. **No backward compatibility concerns**: Removing the tenant filter makes the lookup MORE permissive (finds memos regardless of tenant_id), not less. Old and new tickets both work.

5. **Fixes both read and write paths**: `getTicketComments` (read) and `createSystemResolutionComment` (write) are both fixed.

---

## Verification

```bash
# Run existing tests
go test -v -run TestAskRovo ./server/router/api/v1/agent/ -count=1
go test -v -run TestCreateTicket ./server/router/api/v1/ -count=1

# Manual verification: create ticket with description /m/<existing_memo_uid>
# Should succeed without "parent memo not found" error
```

---

## Follow-Up Considerations

| Item | Description | Priority |
|------|-------------|----------|
| Why did memo 241 get `tenant_id=NULL`? | Memo creation may have occurred without tenant context. Investigate `GetTenantIDFromContext` for the gRPC path. | LOW — not blocking this fix |
| Cross-tenant memo links | If a ticket references a memo from a different tenant, the system will now find it. This is correct behavior — the `/m/` link is an explicit authorization. | N/A — by design |
| Error logging improvement | The `%!w(<nil>)` formatting is ugly in logs. After the fix, errors will be clean strings. | DONE — included in fix |

---

## Adversarial Review Prompt

```
You are a senior Go architect reviewing a bug fix plan for a "parent memo
not found" error that occurs when a ticket is created manually with a
/m/<uid> description linking to a memo that has NULL tenant_id. The fix
removes TenantID from two GetMemo lookups (createSystemResolutionComment
and getTicketComments) since memo UIDs are globally unique and tenant
filtering on a UID lookup is redundant and incorrect. A secondary fix
handles nil error wrapping that produces %!w(<nil>) in logs.

Review this plan critically. Focus on:

1. CORRECTNESS: Is removing the TenantID filter safe? Memo UIDs are
   globally unique UUIDs — but could removing the filter enable cross-tenant
   data access in any scenario? Consider: the ticket already references
   the memo via /m/<uid> in its description — is that implicit
   authorization sufficient?

2. COMPLETENESS: Are there other places in the codebase that look up
   memos by UID with a TenantID filter that might have the same bug?
   Search for all FindMemo{UID: ...} patterns with TenantID.

3. CONSISTENCY: Does this fix align with the existing correct pattern in
   findExistingEscalationTicket (service.go:4593)? Are there any other
   callers of createSystemResolutionComment or getTicketComments?

4. EDGE CASES: What happens if:
   - The memo was deleted but the ticket still references it?
   - The memo UID doesn't exist at all?
   - Multiple tickets reference the same memo?
   - The ticket has no description (empty string)?
   - The description starts with /m/ but has no UID after it?

5. ERROR HANDLING: Is the split error handling (separate nil-error and
   nil-memo checks) correct? Could GetMemo return a non-nil error AND
   nil memo simultaneously?

6. RISK: What's the worst case if this fix is applied incorrectly?
   Could the more permissive lookup cause issues with memo caching,
   indexing, or any other system that assumes tenant-scoped memos?

7. TESTING: The plan only runs existing tests. Should we add a new test
   that specifically exercises the cross-tenant memo lookup scenario?
```
