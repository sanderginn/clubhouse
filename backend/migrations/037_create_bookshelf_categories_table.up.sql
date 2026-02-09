CREATE TABLE bookshelf_categories (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name VARCHAR(100) NOT NULL,
  position INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT bookshelf_categories_user_name_unique UNIQUE (user_id, name)
);

CREATE INDEX idx_bookshelf_categories_user_id ON bookshelf_categories(user_id);
