ALTER TABLE posts
  ADD COLUMN search_vector tsvector NOT NULL DEFAULT ''::tsvector;

ALTER TABLE comments
  ADD COLUMN search_vector tsvector NOT NULL DEFAULT ''::tsvector;

ALTER TABLE links
  ADD COLUMN search_vector tsvector NOT NULL DEFAULT ''::tsvector;

UPDATE posts
  SET search_vector = to_tsvector('english', COALESCE(content, ''));

UPDATE comments
  SET search_vector = to_tsvector('english', COALESCE(content, ''));

UPDATE links
  SET search_vector = to_tsvector(
    'english',
    COALESCE(metadata->>'title','') || ' ' ||
    COALESCE(metadata->>'description','') || ' ' ||
    COALESCE(metadata->>'author','') || ' ' ||
    COALESCE(metadata->>'artist','') || ' ' ||
    COALESCE(metadata->>'provider','') || ' ' ||
    COALESCE(url,'')
  );

CREATE INDEX idx_posts_search_vector ON posts USING GIN(search_vector);
CREATE INDEX idx_comments_search_vector ON comments USING GIN(search_vector);
CREATE INDEX idx_links_search_vector ON links USING GIN(search_vector);

CREATE OR REPLACE FUNCTION update_posts_search_vector() RETURNS trigger AS $$
BEGIN
  NEW.search_vector := to_tsvector('english', COALESCE(NEW.content, ''));
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION update_comments_search_vector() RETURNS trigger AS $$
BEGIN
  NEW.search_vector := to_tsvector('english', COALESCE(NEW.content, ''));
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION update_links_search_vector() RETURNS trigger AS $$
BEGIN
  NEW.search_vector := to_tsvector(
    'english',
    COALESCE(NEW.metadata->>'title','') || ' ' ||
    COALESCE(NEW.metadata->>'description','') || ' ' ||
    COALESCE(NEW.metadata->>'author','') || ' ' ||
    COALESCE(NEW.metadata->>'artist','') || ' ' ||
    COALESCE(NEW.metadata->>'provider','') || ' ' ||
    COALESCE(NEW.url,'')
  );
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER posts_search_vector_trigger
  BEFORE INSERT OR UPDATE OF content ON posts
  FOR EACH ROW
  EXECUTE FUNCTION update_posts_search_vector();

CREATE TRIGGER comments_search_vector_trigger
  BEFORE INSERT OR UPDATE OF content ON comments
  FOR EACH ROW
  EXECUTE FUNCTION update_comments_search_vector();

CREATE TRIGGER links_search_vector_trigger
  BEFORE INSERT OR UPDATE OF url, metadata ON links
  FOR EACH ROW
  EXECUTE FUNCTION update_links_search_vector();
