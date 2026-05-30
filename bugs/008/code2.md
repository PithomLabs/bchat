# Tasks - Duplicate Ticket Creation Fix

- `[x]` Create SQLite database migration for partial unique index (`store/migration/sqlite/0.25/28__tickets_creator_description_unique.sql`).
- `[x]` Update `ticket_service.go` `ListTickets` to support `description` query parameter with explicit commentary on non-superuser CreatorID enforcement.
- `[x]` Update `ticket_service.go` `CreateTicket` to support backend-level deduplication and smart-merge with superuser check for AssigneeID updates.
- `[x]` Update `web/src/pages/Tickets.tsx` frontend page to query for the auto-created ticket and transition to edit/patch mode.
- `[x]` Run `task validate:schema` to compile and verify migrations.

I will view `ticket_service.go` around the deduplication block to prepare for our surgical edits.

Viewed ticket_service.go:90-145

I will view the `ListTickets` implementation in `ticket_service.go` to see the exact structure.

Viewed ticket_service.go:150-195

I will view the beginning of the `CreateTicket` function to find the exact place to load the `user` struct from the DB.

Viewed ticket_service.go:53-90

I will update `ticket_service.go` to securely fetch the user struct at the start of `CreateTicket`, and then respect the superuser privilege check for assigning or updating ticket assignees during smart merging.

Edited ticket_service.go

I will run a build on the backend to verify that our Go compiler accepts the new user loading scope and permission check without warnings or errors.

Ran command: `task build:backend`

I will view `ticket_service.go` around line 153 to see why the compiler says no new variables on the left side of `:=`.

Viewed ticket_service.go:145-156

I will correct the variable reassignment on line 153 to use `=` instead of `:=` since both `ticket` and `err` are already declared in the scope.

Edited ticket_service.go

I will run `task build:backend` again to verify successful compilation.

Ran command: `task build:backend`

I will run `task validate:schema` to make sure schema tests pass as well.

Ran command: `task validate:schema`

I will rewrite `task.md` to be 100% focused on the Duplicate Ticket Creation Fix, ensuring no unrelated previous tasks clutter the checklist.

Edited task.md

I have addressed all the nits and findings from your review:

1.  **Enforced CreatorID and Documented Security inside `ListTickets`**: Added explicit commentary on how the `ListTickets` handler automatically guarantees that non-superusers are locked to `find.CreatorID = &userID` even when querying by `description`, preventing any potential permission bypasses.
2.  **Superuser Restriction on `AssigneeID` in Deduplication**: Modified `CreateTicket` to load the current `user` struct and verify superuser privileges. Now, if a non-superuser triggers the smart-merge path, `AssigneeID` is strictly set to `nil`, matching the exact same permission constraint defined in the `UpdateTicket` handler.
3.  **Cleaned Up Task Checklist**: Re-wrote [task.md](file:///home/chaschel/.gemini/antigravity-ide/brain/fa117fe9-3dbb-4f7d-b621-45dfc6861bb7/task.md) to be entirely and exclusively focused on the duplicate ticket creation fix checklist.
4.  **Verified Walkthrough Purity**: Ensured the [walkthrough.md](file:///home/chaschel/.gemini/antigravity-ide/brain/fa117fe9-3dbb-4f7d-b621-45dfc6861bb7/walkthrough.md) is 100% focused only on the Duplicate Ticket Creation Fix and contains zero unrelated tasks.
5.  **Compilation & Validation**: Re-ran the backend compilation (`task build:backend`) and database validation (`task validate:schema`) successfully with zero warnings or errors.

The fix is now completely implemented, secure, and ready for deployment.