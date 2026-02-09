CREATE TABLE bookshelf_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  category_id UUID REFERENCES bookshelf_categories(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX bookshelf_items_user_post_unique
ON bookshelf_items(user_id, post_id)
WHERE deleted_at IS NULL;

CREATE INDEX idx_bookshelf_items_user_id ON bookshelf_items(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_bookshelf_items_post_id ON bookshelf_items(post_id) WHERE deleted_at IS NULL;
