-- Add optional expiry to api_keys and mcps

-- Executor ignores duplicate column errors
ALTER TABLE api_keys
  ADD COLUMN expires_at DATETIME NULL;

ALTER TABLE mcps
  ADD COLUMN expires_at DATETIME NULL;
