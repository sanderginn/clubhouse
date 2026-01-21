-- Create reactions table
CREATE TABLE reactions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id),
  post_id UUID REFERENCES posts(id),
  comment_id UUID REFERENCES comments(id),
  emoji VARCHAR(10) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  deleted_at TIMESTAMP,

  CONSTRAINT reaction_target CHECK (
    (post_id IS NOT NULL AND comment_id IS NULL) OR
    (post_id IS NULL AND comment_id IS NOT NULL)
  ),
  CONSTRAINT unique_post_reaction UNIQUE(user_id, post_id, emoji),
  CONSTRAINT unique_comment_reaction UNIQUE(user_id, comment_id, emoji)
);

-- Create indexes for reactions table
CREATE INDEX idx_reactions_user_id ON reactions(user_id);
CREATE INDEX idx_reactions_post_id ON reactions(post_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_reactions_comment_id ON reactions(comment_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_reactions_deleted_at ON reactions(deleted_at);
