-- Ensure migrations.applied_at has valid non-zero values and default

SET @OLD_SQL_MODE = @@sql_mode;
SET SESSION sql_mode=(SELECT REPLACE(@@sql_mode,'NO_ZERO_DATE',''));
SET SESSION sql_mode=(SELECT REPLACE(@@sql_mode,'NO_ZERO_IN_DATE',''));

UPDATE migrations SET applied_at = NOW() WHERE applied_at IS NULL OR applied_at = '0000-00-00 00:00:00';
ALTER TABLE migrations
  MODIFY COLUMN applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP;

SET SESSION sql_mode=@OLD_SQL_MODE;
