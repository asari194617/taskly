CREATE TABLE policy_consents (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  terms_version VARCHAR(32) NOT NULL,
  privacy_version VARCHAR(32) NOT NULL,
  security_version VARCHAR(32) NOT NULL,
  display_language VARCHAR(5) NOT NULL,
  accepted_at DATETIME(6) NOT NULL,
  INDEX idx_policy_consents_user_time (user_id, accepted_at),
  CONSTRAINT fk_policy_consents_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
