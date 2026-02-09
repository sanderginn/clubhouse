CREATE TABLE watchlist_categories (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  position INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  CONSTRAINT watchlist_categories_user_name_unique UNIQUE (user_id, name)
);

CREATE INDEX idx_watchlist_categories_user_id ON watchlist_categories(user_id);
