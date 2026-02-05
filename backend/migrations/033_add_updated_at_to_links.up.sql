-- Add updated_at column to links table for tracking metadata updates
ALTER TABLE links ADD COLUMN updated_at TIMESTAMP;
