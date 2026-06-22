CREATE TABLE IF NOT EXISTS bridge_handoff_replies (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  reply_id TEXT NOT NULL UNIQUE CHECK(length(reply_id) > 0 AND length(reply_id) <= 36),
  tenant_id INTEGER NOT NULL,
  session_id TEXT NOT NULL CHECK(length(session_id) > 0),
  handoff_id TEXT NOT NULL CHECK(length(handoff_id) > 0),
  generation INTEGER NOT NULL,
  client_message_id TEXT NOT NULL CHECK(length(client_message_id) > 0 AND length(client_message_id) <= 128),
  text TEXT NOT NULL CHECK(length(text) > 0 AND length(text) <= 2000),
  delivery_status TEXT NOT NULL DEFAULT 'not_delivered' CHECK(delivery_status = 'not_delivered'),
  created_at INTEGER NOT NULL,

  UNIQUE(tenant_id, session_id, handoff_id, client_message_id),
  FOREIGN KEY (tenant_id, handoff_id)
    REFERENCES bridge_handoffs(tenant_id, handoff_id)
    ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, session_id)
    REFERENCES bridge_external_sessions(tenant_id, session_id)
    ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_bridge_handoff_replies_lookup 
ON bridge_handoff_replies(tenant_id, session_id, handoff_id, client_message_id);
