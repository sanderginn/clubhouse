-- Create cook_logs table
CREATE TABLE cook_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id),
  post_id UUID NOT NULL REFERENCES posts(id),
  rating SMALLINT NOT NULL CHECK (rating >= 1 AND rating <= 5),
  notes TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  updated_at TIMESTAMP,
  deleted_at TIMESTAMP,

  CONSTRAINT unique_cook_log UNIQUE(user_id, post_id)
);

CREATE INDEX idx_cook_logs_user_id ON cook_logs(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_cook_logs_post_id ON cook_logs(post_id) WHERE deleted_at IS NULL;
