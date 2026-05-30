The revised plan is solid and addresses my earlier concerns. Here are my findings:

## Review of Revised Plan

### ✅ Strengths
1. **Frontend-first approach solves race conditions**: By querying for the auto-created ticket after memo creation and transitioning to edit mode, the frontend eliminates duplicate submissions entirely
2. **Smart merge preserves auto-derived fields**: The backend logic correctly handles preserving priority/type from auto-ticket when user uses defaults
3. **Database constraint as safety net**: The unique index prevents duplicates even if frontend/backend logic fails
4. **Permission scoping**: Correctly restricts lookups to current user's tickets

### ⚠️ Nits/Clarifications Needed
1. **SQLite unique index syntax**: The `WHERE description LIKE '/m/%'` clause in SQLite needs double quotes or proper escaping. Should be:
   ```sql
   CREATE UNIQUE INDEX idx_tickets_creator_description_memo ON tickets(creator_id, description) WHERE description LIKE '/m/%';
   ```

2. **ListTickets handler**: The plan mentions adding description query param support, but this already exists via `Description` in `FindTicket` - just needs to be wired in `ListTickets` handler

3. **Index placement**: The unique index should go in a new migration file (e.g., `28__tickets_creator_description_unique.sql`) in the `0.25` folder, and LATEST.sql already gets regenerated, so no manual append needed

4. **PostgreSQL/MySQL variants**: Will need equivalent migrations for other databases

### Recommendation
**Approved with nits** - The plan is well-structured and addresses the core issue. Ready for implementation with the minor syntax adjustments noted above.


# Implementation Plan - Fix Duplicate Ticket Creation

## Goal Description

Resolve the duplicate ticket creation issue when a customer creates a ticket with a description using the "Add description (Create Memo)" button. Currently, this action triggers two duplicate tickets:
1. **Auto-creation (Backend):** When the customer creates a memo, `CreateMemo` synchronously triggers `handleAutoTicketCreation`, creating a `SUPPORT` ticket (with auto-derived type/priority if tags like `#bug`, `#high`, `#urgent` are present).
2. **Explicit-creation (Frontend):** When the user clicks the "Create Ticket" form button, the frontend submits a `POST /api/v1/tickets` request, creating a second `TASK` ticket.

We will implement a seamless, bulletproof end-to-end deduplication approach:
1. **Frontend Integration:** When a memo is successfully created inside the dialog, the frontend immediately queries the backend for the auto-created ticket. If found, it populates the dialog with the existing ticket's fields (preserving auto-derived type and priority in the UI) and transitions the dialog to edit/update mode (`PATCH /api/v1/tickets/:id`).
2. **Backend Deduplication & Smart-Merge:** In the `CreateTicket` endpoint (`POST /api/v1/tickets`), before creating a new ticket with a description starting with `/m/`, we check if a ticket for this description and current user already exists. If found, we smart-merge: we preserve the auto-derived `priority` and `type` unless the user explicitly overrode them, and update explicitly controlled fields (Title, Status, Assignee, etc.).
3. **Database unique constraint:** A partial unique index `idx_tickets_creator_description_memo` on `(creator_id, description)` for descriptions matching `/m/%` ensures strong database-level consistency.

---

## User Review Required

We have fully integrated your recommendations:
- **Field preservation:** Auto-derived fields (such as high priority or bug/feature type) are fetched in the frontend and displayed in the form, and also smart-merged in the backend to ensure they are never lost.
- **Permission checks:** Both frontend query and backend deduplication strictly scope ticket lookups to the current authenticated `CreatorID`.
- **Zero race condition:** `CreateMemo` runs `handleAutoTicketCreation` synchronously, meaning the ticket is fully committed before the frontend query is executed.

---

## Proposed Changes

### Database Layer

#### [NEW] `store/migration/sqlite/0.25/28__tickets_creator_description_unique.sql`
Add a database-level partial unique index:
```sql
-- Prevent duplicate tickets for same memo link (auto-creation + explicit creation)
CREATE UNIQUE INDEX IF NOT EXISTS idx_tickets_creator_description_memo 
ON tickets(creator_id, description) 
WHERE description LIKE '/m/%';
```

Note: LATEST.sql is auto-regenerated during build, no manual edit needed.

---

### Backend Service (Go)

#### [MODIFY] `server/router/api/v1/ticket_service.go`

**In `ListTickets` (around line 120):**
```go
if desc := c.QueryParam("description"); desc != "" {
    find.Description = &desc
}
```

**In `CreateTicket` (around line 85):**
Before `s.Store.CreateTicket(ctx, ticket)`, add deduplication logic:
```go
// Check for existing ticket with same memo description (auto-creation de-duplication)
if strings.HasPrefix(ticket.Description, "/m/") {
    existingList, err := s.Store.ListTickets(ctx, &store.FindTicket{
        Description: &ticket.Description,
        CreatorID:   &userID,
    })
    if err == nil && len(existingList) > 0 {
        existing := existingList[0]
        
        // Smart merge: preserve auto-derived values if user didn't override
        if ticket.Priority == store.TicketPriorityMedium {
            // User used default, keep auto-derived priority
            ticket.Priority = existing.Priority
        }
        if ticket.Type == "" || ticket.Type == "TASK" {
            // User used default, keep auto-derived type
            ticket.Type = existing.Type
        }
        
        // Update the existing ticket
        update := &store.UpdateTicket{
            ID:          existing.ID,
            Title:       &ticket.Title,
            Description: &ticket.Description,
            Status:      &ticket.Status,
            Priority:    &ticket.Priority,
            Type:        &ticket.Type,
            Tags:        ticket.Tags,
            AssigneeID:  ticket.AssigneeID,
        }
        now := time.Now().Unix()
        update.UpdatedTs = &now
        
        ticket, err = s.Store.UpdateTicket(ctx, update)
        // ... return updated ticket
    }
}
```

---

### Frontend Components (React)

#### [MODIFY] `web/src/pages/Tickets.tsx`

**Update `handleDescriptionCreated` (around line 261):**
```typescript
const handleDescriptionCreated = async (memoName: string) => {
    const memoUid = memoName.split("/").pop();
    if (memoUid) {
        setDescription(`/m/${memoUid}`);
        
        // Query for auto-created ticket
        const response = await fetch(`/api/v1/tickets?description=/m/${memoUid}`);
        if (response.ok) {
            const data = await response.json();
            if (data && data.length > 0) {
                const existingTicket = data[0];
                setEditingTicket(existingTicket);
                setTitle(existingTicket.title);
                setStatus(existingTicket.status);
                setPriority(existingTicket.priority);
                setType(existingTicket.type || "TASK");
                setAssigneeId(existingTicket.assigneeId || null);
            }
        }
    }
    setIsCreatingDescription(false);
};
```

---

## Verification Plan

### Automated Tests
- Run `task validate:schema` to ensure the new migration compiles

### Manual Verification
1. Log in as a customer (non-superuser)
2. Go to Tickets and click "Create Ticket"
3. Click "Add description (Create Memo)" and create a memo with `#urgent` tag
4. Verify form transitions to "Edit Ticket #ID" with Priority=High pre-filled
5. Click "Update Ticket"
6. Verify only **one** row in the Tickets table with correct values