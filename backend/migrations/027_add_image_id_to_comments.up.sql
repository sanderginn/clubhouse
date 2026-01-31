-- Add optional image reference to comments
ALTER TABLE comments
  ADD COLUMN image_id UUID REFERENCES post_images(id) ON DELETE SET NULL;

CREATE INDEX idx_comments_image_id ON comments(image_id);
