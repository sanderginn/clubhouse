-- Revert audit_logs admin_user_id to required
ALTER TABLE audit_logs
  DROP CONSTRAINT IF EXISTS audit_logs_admin_user_id_fkey;

ALTER TABLE audit_logs
  ALTER COLUMN admin_user_id SET NOT NULL;

ALTER TABLE audit_logs
  ADD CONSTRAINT audit_logs_admin_user_id_fkey
  FOREIGN KEY (admin_user_id)
  REFERENCES users(id);
