-- Account closure and the 30-day erasure deadline. Historical closed accounts
-- have no reliable closure timestamp, so they require manual review.
ALTER TABLE users ADD COLUMN deleted_at DATETIME NULL;
CREATE INDEX idx_users_erasure_due ON users (is_deleted, deleted_at);

CREATE TABLE account_erasure_holds (
  user_id BIGINT UNSIGNED PRIMARY KEY,
  reason VARCHAR(255) NOT NULL,
  review_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
