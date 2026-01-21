-- Create links table
CREATE TABLE links (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  post_id UUID REFERENCES posts(id),
  comment_id UUID REFERENCES comments(id),
  url TEXT NOT NULL,
  metadata JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT now(),

  CONSTRAINT link_target CHECK (
    (post_id IS NOT NULL AND comment_id IS NULL) OR
    (post_id IS NULL AND comment_id IS NOT NULL)
  )
);

-- Create indexes for links table
CREATE INDEX idx_links_post_id ON links(post_id);
CREATE INDEX idx_links_comment_id ON links(comment_id);
CREATE INDEX idx_links_metadata ON links USING GIN(metadata);
