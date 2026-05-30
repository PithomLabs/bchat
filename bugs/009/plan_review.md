## Review Findings

The plan correctly identifies the root cause: existing duplicate tickets with matching `creator_id` and `/m/...` descriptions prevent the unique index from being created. The database has 3 duplicate pairs (IDs 7,8; 4,5; 9,10) all with `creator_id=2`.

**Issue with Proposed SQL**: The DELETE statement in the plan may fail in SQLite due to the restriction on modifying a table while selecting from it in a subquery. SQLite commonly returns "recursive use of aliases" errors for this pattern.

**Recommendations**:
1. The SQL needs to be adjusted to avoid SQLite's self-referential delete limitation. Use a CTE or separate statements.
2. Consider adding `BEGIN TRANSACTION`/`COMMIT` for safety, though SQLite auto-commits each statement by default.

**Approved with nits** - The approach is correct but the SQL implementation needs adjustment.



# Implementation Plan - Fix Migration Constraint Failure

## Goal Description

During startup, the database migration `28__tickets_creator_description_unique.sql` failed with:
`constraint failed: UNIQUE constraint failed: tickets.creator_id, tickets.description (2067)`

### Root Cause Analysis

The existing database contains 3 duplicate ticket pairs (verified):
- IDs 7,8 with creator_id=2 and description `/m/6NfGnW2AwrZMsWfN8V7K6y`
- IDs 4,5 with creator_id=2 and description `/m/8cCkBdFKcdva5zz8tAxcCT`
- IDs 9,10 with creator_id=2 and description `/m/aKADagzazXfQcDoCJNDZjq`

When the migration attempts to create a UNIQUE index on `(creator_id, description)` for `/m/%` patterns, SQLite rejects it because duplicates exist.

### Proposed Fix

Update the SQLite migration file to perform deduplication **before** creating the unique index. Delete older duplicates (keeping highest ID) and let CASCADE delete clean up `agent_workflows` references.

---

## User Review Required

No breaking changes. 3 duplicate ticket records will be deleted, with CASCADE cleanup of workflow references.

---

## Proposed Changes

### Database Layer

#### [MODIFY] store/migration/sqlite/0.25/28__tickets_creator_description_unique.sql

Update the migration file to:
1. Deduplicate existing `/m/...` tickets by keeping the one with the maximum ID for each `(creator_id, description)` pair
2. Create the unique partial index `idx_tickets_creator_description_memo`

```sql
-- 1. Clean up existing duplicate tickets for the same memo link and creator.
-- Keeps the latest ticket (highest ID) and deletes older duplicates.
-- Using CTE to avoid SQLite self-referential delete limitations
WITH duplicates AS (
  SELECT id FROM tickets 
  WHERE description LIKE '/m/%' 
    AND id NOT IN (
      SELECT id FROM (
        SELECT MAX(id) as id
        FROM tickets 
        WHERE description LIKE '/m/%'
        GROUP BY creator_id, description
      )
    )
)
DELETE FROM tickets WHERE id IN (SELECT id FROM duplicates);

-- 2. Prevent duplicate tickets for same memo link (auto-creation + explicit creation)
CREATE UNIQUE INDEX IF NOT EXISTS idx_tickets_creator_description_memo 
ON tickets(creator_id, description) 
WHERE description LIKE '/m/%';
```

---

## Verification Plan

### Automated Tests
- Run `task validate:schema` to ensure the migration passes validation.
- Run `task run:rag` to verify the application boots and migrates successfully.

---

## Review Decision

**APPROVED WITH NITS**

The plan correctly identifies the problem. The proposed SQL uses a CTE pattern which is safer for SQLite's self-referential delete restrictions. The CASCADE foreign key on `agent_workflows.ticket_id` will handle cleanup automatically.