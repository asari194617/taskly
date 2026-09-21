ALTER TABLE notification_type_prefs
  MODIFY notification_type ENUM('TASK','TASK_MORNING','TASK_EVENING','DAYSHIFT') NOT NULL;

ALTER TABLE notifications_queue
  ADD COLUMN event_key VARCHAR(191) NULL,
  ADD COLUMN slot VARCHAR(16) NULL,
  ADD UNIQUE KEY uniq_notification_event (event_key),
  ADD INDEX idx_notification_pending (channel, status, created_at);

CREATE TABLE IF NOT EXISTS notification_deliveries (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  delivery_key VARCHAR(191) NOT NULL,
  channel VARCHAR(32) NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  status ENUM('sending','sent','error') NOT NULL,
  attempts INT UNSIGNED NOT NULL DEFAULT 0,
  error_message TEXT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  sent_at DATETIME NULL,
  UNIQUE KEY uniq_delivery_key (delivery_key),
  INDEX idx_delivery_status (status, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
