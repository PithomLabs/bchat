-- Add tenant_id to memo_relation for comment tenant isolation (SQLite 0.28/00).
-- Backfills from parent memo's tenant_id for existing rows.
ALTER TABLE memo_relation ADD COLUMN IF NOT EXISTS tenant_id INTEGER DEFAULT NULL;

UPDATE memo_relation
SET tenant_id = (
    SELECT m.tenant_id FROM memo m WHERE m.id = memo_relation.memo_id
)
WHERE tenant_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_memo_relation_tenant ON memo_relation(tenant_id);

-- Add allowed_tenant_ids to user for admin tenant binding (SQLite 0.28/02).
-- Null means user can access all tenants; non-null restricts to listed GUIDs.
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS allowed_tenant_ids TEXT DEFAULT NULL;
