-- Fix role_users.created_at defaults and sanitize invalid values

SET @OLD_SQL_MODE = @@sql_mode;
SET SESSION sql_mode=(SELECT REPLACE(@@sql_mode,'NO_ZERO_DATE',''));
SET SESSION sql_mode=(SELECT REPLACE(@@sql_mode,'NO_ZERO_IN_DATE',''));

UPDATE role_users SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00';
ALTER TABLE role_users
  MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP;

SET SESSION sql_mode=@OLD_SQL_MODE;
