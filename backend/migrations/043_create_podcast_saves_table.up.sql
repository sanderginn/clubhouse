CREATE TABLE podcast_saves (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  deleted_at TIMESTAMP
);

CREATE UNIQUE INDEX podcast_saves_user_post_unique
ON podcast_saves(user_id, post_id)
WHERE deleted_at IS NULL;

CREATE INDEX idx_podcast_saves_user_id ON podcast_saves(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_podcast_saves_post_id ON podcast_saves(post_id) WHERE deleted_at IS NULL;
