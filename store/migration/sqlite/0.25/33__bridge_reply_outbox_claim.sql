-- bridge_reply_outbox is a durable preparation table only.
-- The claim layer has no delivery worker, no adapter, no endpoint, and no ChatExternal consumer.

DROP INDEX IF EXISTS idx_bridge_reply_outbox_pending;

CREATE TABLE IF NOT EXISTS bridge_reply_outbox_new (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  outbox_id TEXT NOT NULL UNIQUE CHECK(length(outbox_id) = 36),

  tenant_id INTEGER NOT NULL,
  session_id TEXT NOT NULL CHECK(length(session_id) > 0),
  handoff_id TEXT NOT NULL CHECK(length(handoff_id) > 0),
  reply_id TEXT NOT NULL CHECK(length(reply_id) = 36),

  status TEXT NOT NULL DEFAULT 'pending',
  attempt_count INTEGER NOT NULL DEFAULT 0 CHECK(attempt_count >= 0),
  created_at INTEGER NOT NULL,

  claim_token TEXT UNIQUE CHECK(claim_token IS NULL OR length(claim_token) = 36),
  claimed_by TEXT CHECK(claimed_by IS NULL OR length(claimed_by) BETWEEN 1 AND 128),
  claimed_at INTEGER CHECK(claimed_at IS NULL OR claimed_at > 0),
  claim_expires_at INTEGER CHECK(claim_expires_at IS NULL OR claim_expires_at > 0),

  UNIQUE(tenant_id, reply_id),

  -- Foreign Keys
  FOREIGN KEY (tenant_id, session_id) REFERENCES bridge_external_sessions(tenant_id, session_id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, handoff_id) REFERENCES bridge_handoffs(tenant_id, handoff_id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, reply_id) REFERENCES bridge_handoff_replies(tenant_id, reply_id) ON DELETE CASCADE,

  -- Strict State Machine Constraints
  CHECK(
    (status = 'pending'
      AND claim_token IS NULL
      AND claimed_by IS NULL
      AND claimed_at IS NULL
      AND claim_expires_at IS NULL)
    OR
    (status = 'claimed'
      AND claim_token IS NOT NULL
      AND claimed_by IS NOT NULL
      AND claimed_at IS NOT NULL
      AND claim_expires_at IS NOT NULL
      AND claim_expires_at > claimed_at)
  )
);

INSERT INTO bridge_reply_outbox_new (
    id, outbox_id, tenant_id, session_id, handoff_id, reply_id,
    status, attempt_count, created_at,
    claim_token, claimed_by, claimed_at, claim_expires_at
)
SELECT
    id, outbox_id, tenant_id, session_id, handoff_id, reply_id,
    status, attempt_count, created_at,
    NULL, NULL, NULL, NULL
FROM bridge_reply_outbox;

DROP TABLE bridge_reply_outbox;

ALTER TABLE bridge_reply_outbox_new RENAME TO bridge_reply_outbox;

CREATE INDEX IF NOT EXISTS idx_bridge_reply_outbox_pending
ON bridge_reply_outbox(tenant_id, status);
