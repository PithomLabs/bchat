# Plan: Fix Duplicate Ticket Creation

## Deep Dive Root Cause

When a user creates a ticket from the frontend with a Memo description, two tickets are created due to a race/redundancy in the design:

1. **Auto-creation (Backend):** When the user clicks "Add description (Create Memo)", the frontend creates a Memo. On the backend, `CreateMemo` automatically calls `handleAutoTicketCreation(ctx, memo, user)` for all non-superusers. This instantly creates a ticket (defaulting to type `SUPPORT`).
2. **Explicit-creation (Frontend):** After the Memo is created, the user returns to the modal, clicks "Create Ticket", and the frontend explicitly calls `POST /api/v1/tickets`. This unconditionally creates a second ticket (defaulting to type `TASK`) with the same Memo description.

## Proposed Changes

We will surgically fix this on the backend by deduplicating ticket creation in the `POST /api/v1/tickets` endpoint. 

### [MODIFY] `server/router/api/v1/ticket_service.go`
In the `CreateTicket` function (around line 98):
- Before calling `s.Store.CreateTicket`, check if the `ticket.Description` starts with `/m/` (indicating a linked memo).
- Use `s.Store.ListTickets(ctx, &store.FindTicket{Description: &ticket.Description})` to check if a ticket for this memo already exists.
- If it does, **update** the existing ticket (using `s.Store.UpdateTicket`) with the explicit fields provided by the user (Title, Type, Priority, Status, etc.) instead of creating a new one.
- Return the updated ticket.

This guarantees that a single Memo will only have one primary ticket representing it, resolving the duplicate row issue cleanly without breaking the auto-creation workflow.

## Verification Plan
1. Create a ticket using the frontend modal, utilizing the "Add description (Create Memo)" button.
2. Verify that only **one** ticket is created in the Tickets table (it should reflect the explicit Type, such as `TASK`, set in the modal).
3. Verify that the ticket is correctly linked to the created memo.


Searched for "FindTicket"
Viewed ticket.go:30-60
Edited implementation_plan.md

I have completed the deep dive into the dual ticket creation issue and have written the root cause analysis along with the proposed backend deduplication fix in the [Implementation Plan](file:///home/chaschel/.gemini/antigravity-ide/brain/fa117fe9-3dbb-4f7d-b621-45dfc6861bb7/implementation_plan.md). 

Please review the plan, and once you approve, I will proceed with the surgical fix.