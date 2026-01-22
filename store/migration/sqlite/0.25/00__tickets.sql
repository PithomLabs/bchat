CREATE TABLE tickets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'OPEN',
  priority TEXT NOT NULL DEFAULT 'MEDIUM',
  creator_id INTEGER NOT NULL,
  assignee_id INTEGER,
  created_ts BIGINT NOT NULL,
  updated_ts BIGINT NOT NULL,
  FOREIGN KEY (creator_id) REFERENCES user(id) ON DELETE CASCADE,
  FOREIGN KEY (assignee_id) REFERENCES user(id) ON DELETE SET NULL
);

CREATE INDEX idx_tickets_creator_id ON tickets (creator_id);
CREATE INDEX idx_tickets_status ON tickets (status);
CREATE INDEX idx_tickets_assignee_id ON tickets (assignee_id);
