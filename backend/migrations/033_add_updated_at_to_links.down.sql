-- Remove updated_at column from links table
ALTER TABLE links DROP COLUMN IF EXISTS updated_at;
