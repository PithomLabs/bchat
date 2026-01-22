CREATE TABLE tickets (
  id INT NOT NULL AUTO_INCREMENT,
  title TEXT NOT NULL,
  description TEXT NOT NULL,
  status VARCHAR(255) NOT NULL DEFAULT 'OPEN',
  priority VARCHAR(255) NOT NULL DEFAULT 'MEDIUM',
  creator_id INT NOT NULL,
  assignee_id INT,
  created_ts BIGINT NOT NULL,
  updated_ts BIGINT NOT NULL,
  PRIMARY KEY (id),
  INDEX idx_tickets_creator_id (creator_id),
  INDEX idx_tickets_status (status)
);
