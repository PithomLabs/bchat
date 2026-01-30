-- Beads Integration Migration
-- Adds beads-specific columns to tickets table and creates agent_workflows table
-- NOTE: SQLite doesn't support UNIQUE constraint in ALTER TABLE ADD COLUMN
-- So we recreate the table with all columns, copy data, and swap

-- Create new table with all columns including beads columns
CREATE TABLE tickets_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'OPEN',
    priority TEXT NOT NULL DEFAULT 'MEDIUM',
    creator_id INTEGER NOT NULL,
    assignee_id INTEGER,
    created_ts BIGINT NOT NULL,
    updated_ts BIGINT NOT NULL,
    type TEXT NOT NULL DEFAULT 'TASK',
    tags TEXT NOT NULL DEFAULT '[]',
    beads_id TEXT UNIQUE,
    parent_id INTEGER,
    labels TEXT DEFAULT '[]',
    dependencies TEXT DEFAULT '[]',
    discovery_context TEXT,
    closed_reason TEXT,
    issue_type TEXT,
    FOREIGN KEY (creator_id) REFERENCES user(id) ON DELETE CASCADE,
    FOREIGN KEY (assignee_id) REFERENCES user(id) ON DELETE SET NULL,
    FOREIGN KEY (parent_id) REFERENCES tickets_new(id)
);

-- Copy existing data from old table
INSERT INTO tickets_new (id, title, description, status, priority, creator_id, assignee_id, created_ts, updated_ts, type, tags, issue_type)
SELECT id, title, description, status, priority, creator_id, assignee_id, created_ts, updated_ts,
       COALESCE(type, 'TASK'), COALESCE(tags, '[]'), COALESCE(type, 'TASK')
FROM tickets;

-- Drop old table
DROP TABLE tickets;

-- Rename new table to original name
ALTER TABLE tickets_new RENAME TO tickets;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_tickets_creator_id ON tickets(creator_id);
CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets(status);
CREATE INDEX IF NOT EXISTS idx_tickets_assignee_id ON tickets(assignee_id);
CREATE INDEX IF NOT EXISTS idx_tickets_beads_id ON tickets(beads_id);
CREATE INDEX IF NOT EXISTS idx_tickets_parent_id ON tickets(parent_id);
CREATE INDEX IF NOT EXISTS idx_tickets_issue_type ON tickets(issue_type);

-- Set default values for new columns
UPDATE tickets SET labels = '[]' WHERE labels IS NULL;
UPDATE tickets SET dependencies = '[]' WHERE dependencies IS NULL;

-- Create agent_workflows table for durable storage of agent thoughts/processes
CREATE TABLE IF NOT EXISTS agent_workflows (
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

-- Create indexes for agent_workflows
CREATE INDEX IF NOT EXISTS idx_workflows_ticket ON agent_workflows(ticket_id);
CREATE INDEX IF NOT EXISTS idx_workflows_session ON agent_workflows(session_id);
CREATE INDEX IF NOT EXISTS idx_workflows_created ON agent_workflows(created_ts);
