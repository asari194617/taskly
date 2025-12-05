-- Add language preferences to users
ALTER TABLE users
  ADD COLUMN display_lang VARCHAR(5) NOT NULL DEFAULT 'ja' AFTER is_deleted,
  ADD COLUMN notification_lang VARCHAR(5) NOT NULL DEFAULT 'ja' AFTER display_lang;
