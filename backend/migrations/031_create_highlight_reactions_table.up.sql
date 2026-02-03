-- Create highlight_reactions table
CREATE TABLE highlight_reactions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id),
  link_id UUID NOT NULL REFERENCES links(id),
  highlight_id TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT now(),

  CONSTRAINT unique_highlight_reaction UNIQUE(user_id, highlight_id)
);

CREATE INDEX idx_highlight_reactions_user_id ON highlight_reactions(user_id);
CREATE INDEX idx_highlight_reactions_link_id ON highlight_reactions(link_id);
CREATE INDEX idx_highlight_reactions_highlight_id ON highlight_reactions(highlight_id);
