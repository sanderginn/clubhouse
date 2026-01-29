-- Allow audit logs without admin users (e.g. registration/profile updates)
ALTER TABLE audit_logs
  ALTER COLUMN admin_user_id DROP NOT NULL;

ALTER TABLE audit_logs
  DROP CONSTRAINT IF EXISTS audit_logs_admin_user_id_fkey;

ALTER TABLE audit_logs
  ADD CONSTRAINT audit_logs_admin_user_id_fkey
  FOREIGN KEY (admin_user_id)
  REFERENCES users(id)
  ON DELETE SET NULL;
