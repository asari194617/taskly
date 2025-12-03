-- Email suppression table for temporary delivery pause per address

CREATE TABLE IF NOT EXISTS email_suppressions (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  email VARCHAR(255) NOT NULL,
  reason TEXT NULL,
  suspended_until DATETIME NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_email_suppressions_email (email),
  INDEX idx_email_suppressions_until (suspended_until)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
