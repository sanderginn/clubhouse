DROP INDEX IF EXISTS idx_comments_image_id;

ALTER TABLE comments
  DROP COLUMN IF EXISTS image_id;
