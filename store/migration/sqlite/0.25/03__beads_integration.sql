-- Beads Integration Migration
-- Adds beads-specific columns to tickets table and creates agent_workflows table

-- Add beads-specific columns to tickets table
ALTER TABLE tickets ADD COLUMN beads_id TEXT UNIQUE;
ALTER TABLE tickets ADD COLUMN parent_id INTEGER REFERENCES tickets(id);
ALTER TABLE tickets ADD COLUMN labels TEXT DEFAULT '[]';
ALTER TABLE tickets ADD COLUMN dependencies TEXT DEFAULT '[]';
ALTER TABLE tickets ADD COLUMN discovery_context TEXT;
ALTER TABLE tickets ADD COLUMN closed_reason TEXT;
ALTER TABLE tickets ADD COLUMN issue_type TEXT;

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_tickets_beads_id ON tickets(beads_id);
CREATE INDEX IF NOT EXISTS idx_tickets_parent_id ON tickets(parent_id);
CREATE INDEX IF NOT EXISTS idx_tickets_issue_type ON tickets(issue_type);

-- Migrate existing tickets: set defaults
UPDATE tickets SET issue_type = COALESCE(type, 'TASK') WHERE issue_type IS NULL;
UPDATE tickets SET labels = '[]' WHERE labels IS NULL;
UPDATE tickets SET dependencies = '[]' WHERE dependencies IS NULL;

-- Create agent_workflows table for durable storage of agent thoughts/processes
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

-- Create indexes for agent_workflows
CREATE INDEX IF NOT EXISTS idx_workflows_ticket ON agent_workflows(ticket_id);
CREATE INDEX IF NOT EXISTS idx_workflows_session ON agent_workflows(session_id);
CREATE INDEX IF NOT EXISTS idx_workflows_created ON agent_workflows(created_ts);
