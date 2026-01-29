-- Remove target_user_id and metadata columns from audit_logs
DROP INDEX IF EXISTS idx_audit_logs_target_user_id;
ALTER TABLE audit_logs
  DROP COLUMN IF EXISTS target_user_id,
  DROP COLUMN IF EXISTS metadata;
