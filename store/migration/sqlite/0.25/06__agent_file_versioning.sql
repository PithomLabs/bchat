-- Drop unique constraint to allow file versioning
-- SQLite doesn't support ALTER TABLE DROP CONSTRAINT, so we need to recreate the table

-- Create new table without unique constraint
CREATE TABLE agent_source_files_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    file_type TEXT NOT NULL,
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Copy data from old table
INSERT INTO agent_source_files_new (id, tenant_id, audience_type, file_type, content, content_hash, imported_at)
SELECT id, tenant_id, audience_type, file_type, content, content_hash, imported_at FROM agent_source_files;

-- Drop old table
DROP TABLE agent_source_files;

-- Rename new table
ALTER TABLE agent_source_files_new RENAME TO agent_source_files;

-- Create index for efficient lookups (not unique)
CREATE INDEX idx_source_files_lookup ON agent_source_files(tenant_id, audience_type, file_type, imported_at DESC);
