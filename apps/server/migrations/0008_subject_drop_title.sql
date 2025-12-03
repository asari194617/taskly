-- Drop legacy title column from subjects after ensuring name_ja is populated

-- Final backfill guard (no-op if already filled)
UPDATE subjects
  SET name_ja = title
  WHERE (name_ja IS NULL OR name_ja = '')
    AND title IS NOT NULL AND title <> '';

-- Drop legacy column if exists
ALTER TABLE subjects DROP COLUMN IF EXISTS title;
