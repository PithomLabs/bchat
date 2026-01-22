-- Add foreign key constraints to tickets table via table recreation
-- This migration supports transaction-safe execution by handling dependent tables manually

-- 1. Backup agent_workflows (dependent table)
CREATE TEMPORARY TABLE agent_workflows_backup AS SELECT * FROM agent_workflows;
DROP TABLE agent_workflows;

-- 2. Backup tickets
CREATE TEMPORARY TABLE tickets_backup AS SELECT * FROM tickets;
DROP TABLE tickets;

-- 3. Recreate tickets with foreign keys
CREATE TABLE tickets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'OPEN',
  priority TEXT NOT NULL DEFAULT 'MEDIUM',
  type TEXT NOT NULL DEFAULT 'TASK',
  tags TEXT NOT NULL DEFAULT '[]',
  creator_id INTEGER NOT NULL,
  assignee_id INTEGER,
  created_ts BIGINT NOT NULL,
  updated_ts BIGINT NOT NULL,
  beads_id TEXT,
  parent_id INTEGER,
  labels TEXT DEFAULT '[]',
  dependencies TEXT DEFAULT '[]',
  discovery_context TEXT,
  closed_reason TEXT,
  issue_type TEXT,
  FOREIGN KEY (creator_id) REFERENCES user(id) ON DELETE CASCADE,
  FOREIGN KEY (assignee_id) REFERENCES user(id) ON DELETE SET NULL,
  FOREIGN KEY (parent_id) REFERENCES tickets(id) ON DELETE CASCADE
);

-- 4. Restore tickets data
INSERT INTO tickets (
  id, title, description, status, priority, 
  creator_id, assignee_id, created_ts, updated_ts, 
  type, tags, beads_id, parent_id, 
  labels, dependencies, discovery_context, closed_reason, issue_type
) 
SELECT 
  id, title, description, status, priority, 
  creator_id, assignee_id, created_ts, updated_ts, 
  type, tags, beads_id, parent_id, 
  labels, dependencies, discovery_context, closed_reason, issue_type
FROM tickets_backup 
ORDER BY id ASC;

DROP TABLE tickets_backup;

-- 5. Recreate indexes for tickets
CREATE INDEX idx_tickets_creator_id ON tickets (creator_id);
CREATE INDEX idx_tickets_status ON tickets (status);
CREATE INDEX idx_tickets_assignee_id ON tickets (assignee_id);
CREATE UNIQUE INDEX idx_tickets_beads_id ON tickets(beads_id) WHERE beads_id IS NOT NULL;

-- 6. Recreate agent_workflows
CREATE TABLE agent_workflows (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id INTEGER NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL,
    agent_name TEXT NOT NULL DEFAULT 'antigravity',
    task_name TEXT,
    task_mode TEXT CHECK(task_mode IN ('PLANNING', 'EXECUTION', 'VERIFICATION')),
    task_status TEXT,
    task_summary TEXT,
    predicted_size INTEGER,
    created_ts INTEGER NOT NULL,
    metadata TEXT DEFAULT '{}'
);

-- 7. Restore agent_workflows data
INSERT INTO agent_workflows SELECT * FROM agent_workflows_backup;
DROP TABLE agent_workflows_backup;

-- 8. Recreate indexes for agent_workflows
CREATE INDEX IF NOT EXISTS idx_workflows_ticket ON agent_workflows(ticket_id);
CREATE INDEX IF NOT EXISTS idx_workflows_session ON agent_workflows(session_id);
CREATE INDEX IF NOT EXISTS idx_workflows_created ON agent_workflows(created_ts);
