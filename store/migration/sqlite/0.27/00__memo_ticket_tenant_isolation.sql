-- Add tenant_id to memo table for tenant-scoped visibility
ALTER TABLE memo ADD COLUMN tenant_id INTEGER DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_memo_tenant ON memo(tenant_id);

-- Add tenant_id to tickets table for tenant-scoped access
ALTER TABLE tickets ADD COLUMN tenant_id INTEGER DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_tickets_tenant ON tickets(tenant_id);
