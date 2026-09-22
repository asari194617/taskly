ALTER TABLE integration_jobs
  ADD INDEX idx_integration_kind_claim (kind, status, available_at, lease_until);
