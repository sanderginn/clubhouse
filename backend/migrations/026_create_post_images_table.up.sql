-- Create post_images table
CREATE TABLE post_images (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  image_url TEXT NOT NULL,
  position INTEGER NOT NULL,
  caption TEXT,
  alt_text TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_post_images_post_position ON post_images(post_id, position);
CREATE INDEX idx_post_images_post_id ON post_images(post_id);
