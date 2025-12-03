-- Add name_ja to subjects, backfill from legacy title, enforce NOT NULL

ALTER TABLE subjects
  ADD COLUMN IF NOT EXISTS name_ja VARCHAR(255) NULL;

-- Backfill from legacy title if present
UPDATE subjects
  SET name_ja = title
  WHERE (name_ja IS NULL OR name_ja = '')
    AND title IS NOT NULL AND title <> '';

-- Enforce NOT NULL after backfill
ALTER TABLE subjects
  MODIFY COLUMN name_ja VARCHAR(255) NOT NULL;
