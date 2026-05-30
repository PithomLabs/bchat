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

#### [NEW] [28__tickets_creator_description_unique.sql](file:///home/chaschel/Documents/go/bchat/store/migration/sqlite/0.25/28__tickets_creator_description_unique.sql)
Add a database-level partial unique index:
```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_tickets_creator_description_memo 
ON tickets(creator_id, description) 
WHERE description LIKE '/m/%';
```

#### [MODIFY] [LATEST.sql](file:///home/chaschel/Documents/go/bchat/store/migration/sqlite/LATEST.sql)
Append `idx_tickets_creator_description_memo` to the ticket indexes definition to maintain consistency for new environments.

---

### Backend Service (Go)

#### [MODIFY] [ticket_service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/ticket_service.go)
1. **Support description querying in `ListTickets`:**
   Add support to retrieve tickets by `description` query parameter:
   ```go
   if desc := c.QueryParam("description"); desc != "" {
       find.Description = &desc
   }
   ```
2. **Deduplicate & Smart-Merge in `CreateTicket`:**
   Before creating a new ticket, if the description is `/m/...`:
   - Search for an existing ticket using `Description` and `CreatorID = userID`.
   - If found, merge and update:
     - Keep existing `Priority` if the request has the default `"MEDIUM"`, unless it was explicitly changed or doesn't exist.
     - Keep existing `Type` if the request has the default `"TASK"`, unless it was explicitly changed or doesn't exist.
     - Update all other explicitly controlled fields: Title, Status, Assignee, and Tags.
     - Run `Store.UpdateTicket` and return the result.

---

### Frontend Components (React)

#### [MODIFY] [Tickets.tsx](file:///home/chaschel/Documents/go/bchat/web/src/pages/Tickets.tsx)
Update `handleDescriptionCreated` to:
1. Set the description state to `/m/<memoUid>`.
2. Issue a `GET /api/v1/tickets?description=/m/<memoUid>` fetch.
3. If a ticket is returned, immediately transition the dialog state by setting `setEditingTicket(existingTicket)` and pre-populating all form states (`title`, `status`, `priority`, `type`, `assigneeId`) with the values of the existing ticket.
4. When the user clicks the "Create Ticket" (which becomes "Update Ticket") button, it will seamlessly submit a `PATCH` request to update the auto-created ticket rather than posting a duplicate one.

---

## Verification Plan

### Automated Tests
- Run `task validate:schema` to ensure the new migration compiles and builds cleanly.

### Manual Verification
1. Log in as a customer (non-superuser).
2. Go to Tickets and click "Create Ticket".
3. Write a memo description with a tag (e.g. `urgent` or `#bug`).
4. Save the memo inside the dialog.
5. Verify that the form title immediately transitions to "Edit Ticket #ID" and fields like Priority/Type are pre-populated with the auto-derived values (e.g. `HIGH` or `BUG`).
6. Update the Title, select options, and click "Update Ticket".
7. Verify that only **one** row is added to the Tickets table, containing all correct values and correctly linked.
