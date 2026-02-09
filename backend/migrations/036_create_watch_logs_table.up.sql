-- Create watch_logs table
CREATE TABLE watch_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  rating INTEGER NOT NULL,
  notes TEXT,
  watched_at TIMESTAMP NOT NULL DEFAULT now(),
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  updated_at TIMESTAMP,
  deleted_at TIMESTAMP,

  CONSTRAINT watch_logs_rating_check CHECK (rating >= 1 AND rating <= 5)
);

CREATE UNIQUE INDEX watch_logs_user_post_unique
  ON watch_logs(user_id, post_id)
  WHERE deleted_at IS NULL;

CREATE INDEX idx_watch_logs_user_id ON watch_logs(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_watch_logs_post_id ON watch_logs(post_id) WHERE deleted_at IS NULL;
