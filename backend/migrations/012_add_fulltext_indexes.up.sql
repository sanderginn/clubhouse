CREATE INDEX idx_posts_content_fts ON posts USING GIN(to_tsvector('english', content));
CREATE INDEX idx_comments_content_fts ON comments USING GIN(to_tsvector('english', content));
CREATE INDEX idx_links_metadata_fts ON links USING GIN(
  to_tsvector(
    'english',
    COALESCE(metadata->>'title','') || ' ' ||
    COALESCE(metadata->>'description','') || ' ' ||
    COALESCE(metadata->>'author','') || ' ' ||
    COALESCE(metadata->>'artist','') || ' ' ||
    COALESCE(metadata->>'provider','') || ' ' ||
    COALESCE(url,'')
  )
);
