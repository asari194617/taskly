-- Add indexes for GraphQL auth and subject listing hot paths

ALTER TABLE sessions
  ADD INDEX idx_sessions_hashed_token (hashed_token);

ALTER TABLE blocks
  ADD INDEX idx_blocks_user_id_expires_at (user_id, expires_at);

ALTER TABLE role_users
  ADD INDEX idx_role_users_user_id_role (user_id, role);

ALTER TABLE permissions
  ADD INDEX idx_permissions_user_target_method (user_id, target, method);

ALTER TABLE subjects
  ADD INDEX idx_subjects_expires_at_id (expires_at, id);