-- Manual sync migration to align schema with hand-written migrations
-- Safe conditional changes using information_schema

-- users: drop is_day_shift_active; add updated_at with ON UPDATE
DO BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'is_day_shift_active') THEN
    SET @sql := 'ALTER TABLE users DROP COLUMN is_day_shift_active';
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  END IF;
END;

DO BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'updated_at') THEN
    SET @sql := 'ALTER TABLE users ADD COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP';
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  ELSE
    -- Ensure ON UPDATE clause
    SET @sql := 'ALTER TABLE users MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP';
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  END IF;
END;

-- sessions: user_id NOT NULL
DO BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'sessions' AND COLUMN_NAME = 'user_id' AND IS_NULLABLE = 'YES') THEN
    SET @sql := 'ALTER TABLE sessions MODIFY COLUMN user_id BIGINT UNSIGNED NOT NULL';
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  END IF;
END;

-- tasks: self_check_required boolean not null; deadline DATE; enums for since/until
DO BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tasks' AND COLUMN_NAME = 'self_check_required' AND DATA_TYPE <> 'tinyint') THEN
    SET @sql := 'ALTER TABLE tasks MODIFY COLUMN self_check_required TINYINT(1) NOT NULL DEFAULT 0';
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  ELSEIF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tasks' AND COLUMN_NAME = 'self_check_required') THEN
    SET @sql := 'ALTER TABLE tasks ADD COLUMN self_check_required TINYINT(1) NOT NULL DEFAULT 0 AFTER is_require_hand_in';
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  END IF;
END;

DO BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tasks' AND COLUMN_NAME = 'deadline' AND DATA_TYPE <> 'date') THEN
    SET @sql := 'ALTER TABLE tasks MODIFY COLUMN deadline DATE NOT NULL';
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  END IF;
END;

-- Ensure enums for deadline_since/until
DO BEGIN
  SET @want := "enum('NONE','P1','P2','P3','P4','P5','P6','P7','P8','P9','P10')";
  SET @cur := (SELECT COLUMN_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tasks' AND COLUMN_NAME = 'deadline_since');
  IF @cur IS NULL THEN
    SET @sql := CONCAT('ALTER TABLE tasks ADD COLUMN deadline_since ', @want, ' NOT NULL DEFAULT \"NONE\" AFTER deadline');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  ELSEIF @cur <> @want THEN
    SET @sql := CONCAT('ALTER TABLE tasks MODIFY COLUMN deadline_since ', @want, ' NOT NULL DEFAULT \"NONE\"');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  END IF;
END;

DO BEGIN
  SET @want := "enum('NONE','MORNING','LUNCH','AFTER_SCHOOL','P1','P2','P3','P4','P5','P6','P7','P8','P9','P10')";
  SET @cur := (SELECT COLUMN_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tasks' AND COLUMN_NAME = 'deadline_until');
  IF @cur IS NULL THEN
    SET @sql := CONCAT('ALTER TABLE tasks ADD COLUMN deadline_until ', @want, ' NOT NULL DEFAULT \"NONE\" AFTER deadline_since');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  ELSEIF @cur <> @want THEN
    SET @sql := CONCAT('ALTER TABLE tasks MODIFY COLUMN deadline_until ', @want, ' NOT NULL DEFAULT \"NONE\"');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  END IF;
END;

-- ical_keys: add expires_at DATETIME NULL
DO BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ical_keys' AND COLUMN_NAME = 'expires_at') THEN
    SET @sql := 'ALTER TABLE ical_keys ADD COLUMN expires_at DATETIME NULL AFTER description';
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  END IF;
END;

-- Drop system_settings table
DO BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'system_settings') THEN
    SET @sql := 'DROP TABLE system_settings';
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  END IF;
END;

-- Create web_push_vapid
CREATE TABLE IF NOT EXISTS web_push_vapid (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  public_key VARCHAR(255) NOT NULL,
  private_key VARCHAR(255) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Create web_push_subscriptions
CREATE TABLE IF NOT EXISTS web_push_subscriptions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  endpoint VARCHAR(1024) NOT NULL,
  p256dh VARCHAR(1024) NOT NULL,
  auth VARCHAR(255) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX(user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Create mcps (MCP integration tokens)
CREATE TABLE IF NOT EXISTS mcps (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  hashed_token VARCHAR(255),
  description VARCHAR(255),
  expires_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX(user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Day shift group tables
CREATE TABLE IF NOT EXISTS day_shift_cycle_groups (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  cycle_id BIGINT UNSIGNED NOT NULL,
  position INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX(cycle_id),
  INDEX(position)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS day_shift_cycle_group_members (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  group_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  position INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX(group_id),
  INDEX(user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
