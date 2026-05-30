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
