-- Create generalized day shift group tables (replacing legacy pairs)
-- Note: This migration creates schema only; data backfill from legacy
-- failures on fresh databases where the legacy table does not exist.

CREATE TABLE IF NOT EXISTS day_shift_cycle_groups (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  cycle_id BIGINT UNSIGNED NOT NULL,
  position INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_dscg_cycle (cycle_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS day_shift_cycle_group_members (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  group_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  position INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_dscgm_group (group_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
