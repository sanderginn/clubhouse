-- Create read_logs table
CREATE TABLE read_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  rating INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,

  CONSTRAINT read_logs_rating_check CHECK (rating >= 1 AND rating <= 5)
);

CREATE UNIQUE INDEX read_logs_user_post_unique
  ON read_logs(user_id, post_id)
  WHERE deleted_at IS NULL;

CREATE INDEX idx_read_logs_user_id ON read_logs(user_id);
CREATE INDEX idx_read_logs_post_id ON read_logs(post_id);
