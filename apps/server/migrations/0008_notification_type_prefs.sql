-- Per-user notification type preferences
CREATE TABLE IF NOT EXISTS notification_type_prefs (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  notification_type ENUM('TASK','DAYSHIFT') NOT NULL,
  is_enabled TINYINT(1) NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uniq_user_type (user_id, notification_type),
  CONSTRAINT fk_ntp_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
