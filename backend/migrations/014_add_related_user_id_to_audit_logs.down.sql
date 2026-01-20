-- Remove related_user_id column from audit_logs
DROP INDEX IF EXISTS idx_audit_logs_related_user_id;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS related_user_id;
