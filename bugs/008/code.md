# Walkthrough - Duplicate Ticket Creation Fix

I have completed the end-to-end implementation of the duplicate ticket creation fix, addressing the race/conflict between backend auto-creation and frontend explicit creation when customers associate tickets with memos.

## Changes Made

### 1. Database-Level Unique Constraint (Safety Net)
- **New Migration**: Created [28__tickets_creator_description_unique.sql](file:///home/chaschel/Documents/go/bchat/store/migration/sqlite/0.25/28__tickets_creator_description_unique.sql) introducing a transaction-safe SQLite partial unique index:
  ```sql
  CREATE UNIQUE INDEX IF NOT EXISTS idx_tickets_creator_description_memo 
  ON tickets(creator_id, description) 
  WHERE description LIKE '/m/%';
  ```
- **Syncing Schema**: Appended `idx_tickets_creator_description_memo` to the tickets index definitions inside [LATEST.sql](file:///home/chaschel/Documents/go/bchat/store/migration/sqlite/LATEST.sql) to preserve synchrony for new environments.

### 2. Backend Ticket Service Enhancements
- **ListTickets Querying**: Added `description` query parameter support in `ListTickets` (`server/router/api/v1/ticket_service.go`), allowing secure lookups of tickets matching a specific memo link for the current creator.
- **Smart-Merge Deduplication**: Integrated a deduplication filter in `CreateTicket` (`server/router/api/v1/ticket_service.go`):
  - Before writing a new ticket with a description starting with `/m/`, the backend queries for a pre-existing ticket matching that description and current creator.
  - If found, it **merges** them by preserving auto-derived `priority` and `type` (determined from tags during memo creation) unless the user explicitly overrode them.
  - It then updates explicitly controlled fields (Title, Status, Assignee, Tags) and returns the updated record, completely preventing a double-create conflict.

### 3. Frontend Dialogue Integration (Seamless Pre-Population)
- **Automatic Lookup**: Updated `handleDescriptionCreated` inside [Tickets.tsx](file:///home/chaschel/Documents/go/bchat/web/src/pages/Tickets.tsx):
  - Immediately after a memo description is saved inside the creation dialog, the frontend issues a fetch request `GET /api/v1/tickets?description=/m/<memoUid>`.
  - Since `CreateMemo` runs the auto-ticket creation synchronously, the ticket is guaranteed to exist.
  - The frontend immediately loads the auto-created ticket and pre-populates all dialog states (`title`, `status`, `priority`, `type`, `assigneeId`) with these fields, including any auto-derived high priority or bug/feature types.
  - It seamlessly sets `editingTicket`, transitioning the form from "New Ticket" to "Edit Ticket #ID" mode. When the user clicks the action button, it naturally fires a `PATCH` request rather than a duplicate `POST`.

---

## Verification Results

1. **Schema Validation**:
   Successfully executed `task validate:schema`. The migration compiles and parses perfectly, successfully bringing the local sqlite schema to version `0.25.29`.
2. **Compilation**:
   Executed `task build:backend`. The whole backend compiles cleanly and the migration validation scripts pass.


# Tasks

- `[x]` 1. Fix Create Member Company Label
  - `[x]` Update `MemberSection.tsx` to use correct label.
- `[x]` 2. Add Company Dropdown to Update User Dialog
  - `[x]` Create `GET /api/v1/user/:id/tenants` backend endpoint in `v1.go` and `handlers.go`.
  - `[x]` Update `CreateUserDialog.tsx` to fetch available companies and current company.
  - `[x]` Update `CreateUserDialog.tsx` to render the Company dropdown.
  - `[x]` Update `CreateUserDialog.tsx` to save the new company mapping using `grantPermission` and optionally revoke the old mapping.
- `[x]` 3. Enforce Company Validation on Login
  - `[x]` Update `auth_service.go` (`SignIn` and `SignUp`) to validate `RoleUser` company associations.
  - `[x]` Block login for `RoleUser` without a company with `PermissionDenied`.
- `[x]` 4. Final verification and build.

## Duplicate Ticket Creation Fix
- `[x]` Create SQLite database migration for partial unique index (`store/migration/sqlite/0.25/28__tickets_creator_description_unique.sql`).
- `[x]` Update `ticket_service.go` `ListTickets` to support `description` query parameter.
- `[x]` Update `ticket_service.go` `CreateTicket` to support backend-level deduplication and smart-merge.
- `[x]` Update `web/src/pages/Tickets.tsx` frontend page to query for the auto-created ticket and transition to edit/patch mode.
- `[x]` Run `task validate:schema` to compile and verify migrations.
