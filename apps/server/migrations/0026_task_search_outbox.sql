-- MySQL records every task mutation, including writes by batch jobs outside the API.
CREATE TABLE IF NOT EXISTS task_search_outbox (
  task_id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
  version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  changed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TRIGGER IF NOT EXISTS task_search_after_insert AFTER INSERT ON tasks FOR EACH ROW
  INSERT INTO task_search_outbox (task_id) VALUES (NEW.id)
  ON DUPLICATE KEY UPDATE version = version + 1;

CREATE TRIGGER IF NOT EXISTS task_search_after_update AFTER UPDATE ON tasks FOR EACH ROW
  INSERT INTO task_search_outbox (task_id) VALUES (NEW.id)
  ON DUPLICATE KEY UPDATE version = version + 1;

CREATE TRIGGER IF NOT EXISTS task_search_after_delete AFTER DELETE ON tasks FOR EACH ROW
  INSERT INTO task_search_outbox (task_id) VALUES (OLD.id)
  ON DUPLICATE KEY UPDATE version = version + 1;
