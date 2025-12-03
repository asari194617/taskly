-- 0021_configs.sql
-- Create key-value configs table and seed day_shift_current_index
CREATE TABLE IF NOT EXISTS configs (
  `key` VARCHAR(128) NOT NULL PRIMARY KEY,
  `value` TEXT NULL,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Seed index if missing
INSERT INTO configs(`key`,`value`) SELECT 'day_shift_current_index','0' WHERE NOT EXISTS (SELECT 1 FROM configs WHERE `key`='day_shift_current_index');
