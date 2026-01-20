DROP TRIGGER IF EXISTS posts_search_vector_trigger ON posts;
DROP TRIGGER IF EXISTS comments_search_vector_trigger ON comments;
DROP TRIGGER IF EXISTS links_search_vector_trigger ON links;

DROP FUNCTION IF EXISTS update_posts_search_vector();
DROP FUNCTION IF EXISTS update_comments_search_vector();
DROP FUNCTION IF EXISTS update_links_search_vector();

DROP INDEX IF EXISTS idx_posts_search_vector;
DROP INDEX IF EXISTS idx_comments_search_vector;
DROP INDEX IF EXISTS idx_links_search_vector;

ALTER TABLE posts DROP COLUMN IF EXISTS search_vector;
ALTER TABLE comments DROP COLUMN IF EXISTS search_vector;
ALTER TABLE links DROP COLUMN IF EXISTS search_vector;
