-- Add moderation support: is_hidden on tasks, flagged_contents and user_reports tables

-- Executor ignores duplicate column errors
ALTER TABLE tasks
  ADD COLUMN is_hidden BOOLEAN NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS flagged_contents (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  task_id BIGINT UNSIGNED NOT NULL,
  category VARCHAR(64) NOT NULL,
  severity VARCHAR(16) NOT NULL,
  score DECIMAL(5,3) NULL,
  threat BOOLEAN NOT NULL DEFAULT FALSE,
  reason TEXT NULL,
  model VARCHAR(64) NULL,
  metadata JSON NULL,
  snapshot JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_flagged_contents_task_id (task_id),
  INDEX idx_flagged_contents_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_reports (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  reporter_user_id BIGINT UNSIGNED NULL,
  task_id BIGINT UNSIGNED NOT NULL,
  reason VARCHAR(255) NOT NULL,
  comment TEXT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_user_reports_task_id (task_id),
  INDEX idx_user_reports_reporter (reporter_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
