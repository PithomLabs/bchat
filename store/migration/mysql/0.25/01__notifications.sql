CREATE TABLE notifications (
  id INTEGER PRIMARY KEY AUTO_INCREMENT,
  initiator_id INTEGER NOT NULL,
  receiver_id INTEGER NOT NULL,
  ticket_url TEXT NOT NULL,
  created_ts BIGINT NOT NULL,
  is_read BOOLEAN NOT NULL DEFAULT 0
);
