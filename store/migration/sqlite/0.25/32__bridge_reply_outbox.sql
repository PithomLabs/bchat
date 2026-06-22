-- bridge_reply_outbox is a durable preparation table only.
-- No consumer, delivery worker, transport adapter, SSE, polling, or ChatExternal delivery
-- is implemented in BRIDGE-OUTBOX-0006.

CREATE UNIQUE INDEX IF NOT EXISTS idx_bridge_handoff_replies_tenant_reply
ON bridge_handoff_replies(tenant_id, reply_id);

CREATE TABLE IF NOT EXISTS bridge_reply_outbox (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  outbox_id TEXT NOT NULL UNIQUE CHECK(length(outbox_id) = 36),

  tenant_id INTEGER NOT NULL,
  session_id TEXT NOT NULL CHECK(length(session_id) > 0),
  handoff_id TEXT NOT NULL CHECK(length(handoff_id) > 0),
  reply_id TEXT NOT NULL CHECK(length(reply_id) = 36),

  status TEXT NOT NULL DEFAULT 'pending' CHECK(status = 'pending'),
  attempt_count INTEGER NOT NULL DEFAULT 0 CHECK(attempt_count = 0),
  created_at INTEGER NOT NULL,

  UNIQUE(tenant_id, reply_id),

  FOREIGN KEY (tenant_id, session_id)
    REFERENCES bridge_external_sessions(tenant_id, session_id)
    ON DELETE CASCADE,

  FOREIGN KEY (tenant_id, handoff_id)
    REFERENCES bridge_handoffs(tenant_id, handoff_id)
    ON DELETE CASCADE,

  FOREIGN KEY (tenant_id, reply_id)
    REFERENCES bridge_handoff_replies(tenant_id, reply_id)
    ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_bridge_reply_outbox_pending ON bridge_reply_outbox(tenant_id, status);
