-- Align users table with application model fields used in auth
ALTER TABLE users
  ADD COLUMN email_code_allowed BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE users
  ADD COLUMN notification_lang VARCHAR(5) NOT NULL DEFAULT 'ja';
ALTER TABLE users
  ADD COLUMN is_registered BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users
  ADD COLUMN is_agreed_policies BOOLEAN NOT NULL DEFAULT FALSE;

-- Login codes for email-based authentication
CREATE TABLE IF NOT EXISTS login_codes (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  email VARCHAR(255) NOT NULL,
  hashed_code VARCHAR(255) NOT NULL,
  expires_at DATETIME NOT NULL,
  consumed_at DATETIME NULL,
  failed_attempts INT NOT NULL DEFAULT 0,
  locked_until DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_login_codes_email (email),
  INDEX idx_login_codes_expires_at (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Ensure sessions table has user_id column used by application
ALTER TABLE sessions
  ADD COLUMN user_id BIGINT UNSIGNED NULL;
ALTER TABLE sessions
  ADD INDEX idx_sessions_user (user_id);
