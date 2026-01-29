-- Add target_user_id and metadata columns to audit_logs
ALTER TABLE audit_logs
  ADD COLUMN target_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN metadata JSONB;

-- Create index for filtering by target_user_id
CREATE INDEX idx_audit_logs_target_user_id ON audit_logs(target_user_id);
