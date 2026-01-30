ALTER TABLE admin_config
ADD COLUMN IF NOT EXISTS display_timezone TEXT NOT NULL DEFAULT 'UTC';

UPDATE admin_config
SET display_timezone = 'UTC'
WHERE display_timezone IS NULL OR display_timezone = '';
