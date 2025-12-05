-- Drop obsolete push_daily_enabled column; executor ignores 'unknown column' errors for idempotency
ALTER TABLE users DROP COLUMN push_daily_enabled;
