-- Add optional timestamp_seconds to comments
ALTER TABLE comments
  ADD COLUMN timestamp_seconds INTEGER,
  ADD CONSTRAINT comments_timestamp_seconds_check
    CHECK (timestamp_seconds IS NULL OR (timestamp_seconds >= 0 AND timestamp_seconds <= 21600));

CREATE INDEX idx_comments_timestamp_seconds ON comments(timestamp_seconds);
