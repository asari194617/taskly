-- Manual sync migration to align schema with hand-written migrations
-- Safe conditional changes using information_schema

-- users.updated_at: ensure column exists and ON UPDATE clause
ALTER TABLE users ADD COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;
ALTER TABLE users MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;

-- sessions: user_id NOT NULL
-- sessions.user_id: enforce NOT NULL (ignored if column not present)
ALTER TABLE sessions MODIFY COLUMN user_id BIGINT UNSIGNED NOT NULL;

-- tasks: is_self_check_required boolean not null; deadline DATE; enums for since/until
-- tasks.is_self_check_required: add then enforce type
ALTER TABLE tasks ADD COLUMN is_self_check_required TINYINT(1) NOT NULL DEFAULT 0 AFTER is_require_hand_in;
ALTER TABLE tasks MODIFY COLUMN is_self_check_required TINYINT(1) NOT NULL DEFAULT 0;

-- tasks.deadline: ensure DATE type
ALTER TABLE tasks MODIFY COLUMN deadline DATE NOT NULL;

-- Ensure enums for deadline_since/until
-- tasks.deadline_since: add then ensure enum type
ALTER TABLE tasks ADD COLUMN deadline_since ENUM('NONE','P1','P2','P3','P4','P5','P6','P7','P8','P9','P10') NOT NULL DEFAULT 'NONE' AFTER deadline;
ALTER TABLE tasks MODIFY COLUMN deadline_since ENUM('NONE','P1','P2','P3','P4','P5','P6','P7','P8','P9','P10') NOT NULL DEFAULT 'NONE';

-- tasks.deadline_until: add then ensure enum type
ALTER TABLE tasks ADD COLUMN deadline_until ENUM('NONE','MORNING','LUNCH','AFTER_SCHOOL','P1','P2','P3','P4','P5','P6','P7','P8','P9','P10') NOT NULL DEFAULT 'NONE' AFTER deadline_since;
ALTER TABLE tasks MODIFY COLUMN deadline_until ENUM('NONE','MORNING','LUNCH','AFTER_SCHOOL','P1','P2','P3','P4','P5','P6','P7','P8','P9','P10') NOT NULL DEFAULT 'NONE';

-- ical_keys: add expires_at DATETIME NULL
-- ical_keys.expires_at: add column
ALTER TABLE ical_keys ADD COLUMN expires_at DATETIME NULL AFTER description;

-- Drop system_settings table
-- Drop system_settings table if present (executor ignores unknown table errors)
DROP TABLE system_settings;

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
