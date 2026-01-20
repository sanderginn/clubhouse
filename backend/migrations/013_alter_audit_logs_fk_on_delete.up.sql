-- Alter audit_logs FK constraints to allow deletion of referenced posts/comments
-- Drop existing constraints and recreate with ON DELETE SET NULL

ALTER TABLE audit_logs
DROP CONSTRAINT IF EXISTS audit_logs_related_post_id_fkey;

ALTER TABLE audit_logs
DROP CONSTRAINT IF EXISTS audit_logs_related_comment_id_fkey;

ALTER TABLE audit_logs
ADD CONSTRAINT audit_logs_related_post_id_fkey
FOREIGN KEY (related_post_id) REFERENCES posts(id) ON DELETE SET NULL;

ALTER TABLE audit_logs
ADD CONSTRAINT audit_logs_related_comment_id_fkey
FOREIGN KEY (related_comment_id) REFERENCES comments(id) ON DELETE SET NULL;
