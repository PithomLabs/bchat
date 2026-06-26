DROP INDEX IF EXISTS idx_bridge_reply_outbox_pending;
CREATE INDEX IF NOT EXISTS idx_bridge_reply_outbox_claimable
ON bridge_reply_outbox(tenant_id, status, claim_expires_at, created_at);
