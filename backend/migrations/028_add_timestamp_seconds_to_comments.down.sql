DROP INDEX IF EXISTS idx_comments_timestamp_seconds;
ALTER TABLE comments
  DROP CONSTRAINT IF EXISTS comments_timestamp_seconds_check,
  DROP COLUMN IF EXISTS timestamp_seconds;
