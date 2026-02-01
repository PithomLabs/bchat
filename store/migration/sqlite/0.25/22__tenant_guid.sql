-- Add GUID column to agent_tenants for security
-- Makes tenant identifiers harder to guess

ALTER TABLE agent_tenants ADD COLUMN guid TEXT;

-- Generate UUIDs for existing tenants (SQLite compatible)
UPDATE agent_tenants
SET guid = lower(hex(randomblob(4))) || '-' ||
           lower(hex(randomblob(2))) || '-4' ||
           substr(lower(hex(randomblob(2))),2) || '-' ||
           substr('89ab', abs(random()) % 4 + 1, 1) ||
           substr(lower(hex(randomblob(2))),2) || '-' ||
           lower(hex(randomblob(6)))
WHERE guid IS NULL;

-- Create index for lookups
CREATE INDEX IF NOT EXISTS idx_agent_tenants_guid ON agent_tenants(guid);
