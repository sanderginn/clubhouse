CREATE TABLE saved_recipes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id),
  post_id UUID NOT NULL REFERENCES posts(id),
  category VARCHAR(100) NOT NULL DEFAULT 'Uncategorized',
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  deleted_at TIMESTAMP,
  CONSTRAINT unique_saved_recipe UNIQUE(user_id, post_id, category)
);

CREATE INDEX idx_saved_recipes_user_id ON saved_recipes(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_saved_recipes_post_id ON saved_recipes(post_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_saved_recipes_user_category ON saved_recipes(user_id, category) WHERE deleted_at IS NULL;
