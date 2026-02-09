CREATE TABLE watchlist_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  category TEXT NOT NULL DEFAULT 'Uncategorized',
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  deleted_at TIMESTAMP,
  CONSTRAINT watchlist_items_user_post_category_unique
    UNIQUE (user_id, post_id, category)
    WHERE deleted_at IS NULL
);

CREATE INDEX idx_watchlist_items_user_id ON watchlist_items(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_watchlist_items_post_id ON watchlist_items(post_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_watchlist_items_user_category ON watchlist_items(user_id, category) WHERE deleted_at IS NULL;
