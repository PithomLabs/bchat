# Implementation Plan - Fix Migration Constraint Failure

## Goal Description

During startup, the database migration `28__tickets_creator_description_unique.sql` failed with:
`constraint failed: UNIQUE constraint failed: tickets.creator_id, tickets.description (2067)`

### Root Cause
Because the duplicate ticket creation bug was active previously, existing database instances (like `memos_dev.db`) already contain duplicate tickets with the same `creator_id` and `description` (memo link `/m/...`). 
SQLite does not allow creating a `UNIQUE` index on columns that contain duplicate values.

### Proposed Fix
We will update the SQLite migration file to perform a transactional clean-up/deduplication step **before** creating the unique index. This will delete older duplicates and keep only the latest ticket (highest ID) for each `(creator_id, description)` group, ensuring the migration succeeds seamlessly on existing databases.

---

## User Review Required

No breaking changes are introduced. Unused duplicate ticket records will be deleted, and their references in the `agent_workflows` table will cascade delete cleanly.

---

## Proposed Changes

### Database Layer

#### [MODIFY] [28__tickets_creator_description_unique.sql](file:///home/chaschel/Documents/go/bchat/store/migration/sqlite/0.25/28__tickets_creator_description_unique.sql)
Update the migration file to:
1. Deduplicate existing `/m/...` tickets by keeping the one with the maximum ID for each `(creator_id, description)` pair and deleting the rest.
2. Create the unique partial index `idx_tickets_creator_description_memo`.

```sql
-- 1. Clean up existing duplicate tickets for the same memo link and creator.
-- Keeps the latest ticket (highest ID) and deletes older duplicates.
DELETE FROM tickets 
WHERE description LIKE '/m/%' 
  AND id NOT IN (
    SELECT MAX(id) 
    FROM tickets 
    WHERE description LIKE '/m/%' 
    GROUP BY creator_id, description
  );

-- 2. Prevent duplicate tickets for same memo link (auto-creation + explicit creation)
CREATE UNIQUE INDEX IF NOT EXISTS idx_tickets_creator_description_memo 
ON tickets(creator_id, description) 
WHERE description LIKE '/m/%';
```

---

## Verification Plan

### Automated Tests
- Run `task validate:schema` to ensure the migration passes validation.
- Run `task run:rag` to verify that the application boots and migrates existing databases successfully.



Viewed ticket_service.go:1-43

I will search `LATEST.sql` to find all foreign keys referencing the `tickets` table to ensure that deleting duplicates does not violate foreign key constraints.

Searched for "REFERENCES tickets"

I will update the implementation plan to detail the root cause of the migration failure (existing duplicate records in the database) and outline a clean, robust data-cleanup script inside the migration file before creating the unique constraint.

Edited implementation_plan.md

I will update `task.md` to add the migration hotfix items for tracking.

Edited task.md

I have analyzed the migration error and updated the implementation plan to resolve it.

### Root Cause
Because the duplicate ticket creation bug was previously active in your development environment, your active database (`memos_dev.db`) already contains duplicate tickets with the same `creator_id` and `/m/...` `description`. When the system tries to execute the SQLite migration `28` on startup, it attempts to create a `UNIQUE` index on columns that contain duplicate values, causing SQLite to reject the migration with a `UNIQUE constraint failed` error.

### Proposed Solution
I have designed a revised migration plan inside the [implementation_plan.md](file:///home/chaschel/.gemini/antigravity-ide/brain/fa117fe9-3dbb-4f7d-b621-45dfc6861bb7/implementation_plan.md):
1. **Deduplication Step:** We first execute a `DELETE` query inside migration `28` to clean up existing duplicate tickets for the same memo and creator, keeping only the latest ticket (highest ID). Any references to the deleted tickets in `agent_workflows` will cleanly cascade delete.
2. **Unique Index:** Once the existing duplicates are removed, we safely create the unique index constraint.

Please check the revised plan. Let me know when I have your go-ahead to execute this hotfix!