-- Drop obsolete push_daily_enabled column if it exists
ALTER TABLE users DROP COLUMN IF EXISTS push_daily_enabled;
