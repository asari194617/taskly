-- Add optional expiry to api_keys and mcps

ALTER TABLE api_keys
  ADD COLUMN IF NOT EXISTS expires_at DATETIME NULL;

ALTER TABLE mcps
  ADD COLUMN IF NOT EXISTS expires_at DATETIME NULL;
