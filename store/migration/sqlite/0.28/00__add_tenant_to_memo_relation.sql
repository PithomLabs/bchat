-- Add tenant_id to memo_relation for comment tenant isolation (Issue #14)
-- Backfills from parent memo's tenant_id for existing rows.

ALTER TABLE memo_relation ADD COLUMN tenant_id INTEGER DEFAULT NULL;

UPDATE memo_relation
SET tenant_id = (
    SELECT m.tenant_id FROM memo m WHERE m.id = memo_relation.memo_id
)
WHERE tenant_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_memo_relation_tenant ON memo_relation(tenant_id);
