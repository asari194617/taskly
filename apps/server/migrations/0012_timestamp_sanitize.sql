-- Sanitize invalid zero timestamps before altering columns that require NOT NULL/valid defaults

SET @OLD_SQL_MODE = @@sql_mode;
SET SESSION sql_mode=(SELECT REPLACE(@@sql_mode,'NO_ZERO_DATE',''));
SET SESSION sql_mode=(SELECT REPLACE(@@sql_mode,'NO_ZERO_IN_DATE',''));

UPDATE users SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00';
UPDATE tasks SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00';
UPDATE tasks SET updated_at = IFNULL(updated_at, created_at) WHERE updated_at IS NULL OR updated_at = '0000-00-00 00:00:00';
UPDATE api_keys SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00';
UPDATE api_keys SET updated_at = IFNULL(updated_at, created_at) WHERE updated_at IS NULL OR updated_at = '0000-00-00 00:00:00';
UPDATE permissions SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00';
UPDATE permissions SET updated_at = IFNULL(updated_at, created_at) WHERE updated_at IS NULL OR updated_at = '0000-00-00 00:00:00';
UPDATE blocks SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00';
UPDATE blocks SET updated_at = IFNULL(updated_at, created_at) WHERE updated_at IS NULL OR updated_at = '0000-00-00 00:00:00';
UPDATE role_users SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00';
UPDATE passkeys SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00';
UPDATE passkeys SET updated_at = IFNULL(updated_at, created_at) WHERE updated_at IS NULL OR updated_at = '0000-00-00 00:00:00';
UPDATE day_shift_assignments SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00';
UPDATE day_shift_assignments SET updated_at = IFNULL(updated_at, created_at) WHERE updated_at IS NULL OR updated_at = '0000-00-00 00:00:00';
UPDATE web_push_subscriptions SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00';
UPDATE web_push_subscriptions SET updated_at = IFNULL(updated_at, created_at) WHERE updated_at IS NULL OR updated_at = '0000-00-00 00:00:00';

SET SESSION sql_mode=@OLD_SQL_MODE;
