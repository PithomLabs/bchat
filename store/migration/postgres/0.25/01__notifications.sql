CREATE TABLE notifications (
  id SERIAL PRIMARY KEY,
  initiator_id INTEGER NOT NULL,
  receiver_id INTEGER NOT NULL,
  ticket_url TEXT NOT NULL,
  created_ts BIGINT NOT NULL,
  is_read BOOLEAN NOT NULL DEFAULT FALSE
);
