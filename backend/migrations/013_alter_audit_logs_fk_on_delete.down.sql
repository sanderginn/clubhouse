-- Revert audit_logs FK constraints back to default (no ON DELETE action)

ALTER TABLE audit_logs
DROP CONSTRAINT IF EXISTS audit_logs_related_post_id_fkey;

ALTER TABLE audit_logs
DROP CONSTRAINT IF EXISTS audit_logs_related_comment_id_fkey;

ALTER TABLE audit_logs
ADD CONSTRAINT audit_logs_related_post_id_fkey
FOREIGN KEY (related_post_id) REFERENCES posts(id);

ALTER TABLE audit_logs
ADD CONSTRAINT audit_logs_related_comment_id_fkey
FOREIGN KEY (related_comment_id) REFERENCES comments(id);
