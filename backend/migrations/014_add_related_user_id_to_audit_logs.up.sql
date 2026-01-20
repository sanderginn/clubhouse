-- Add related_user_id column to audit_logs for user-related actions (approve, reject)
ALTER TABLE audit_logs ADD COLUMN related_user_id UUID REFERENCES users(id) ON DELETE SET NULL;

-- Create index for the new column
CREATE INDEX idx_audit_logs_related_user_id ON audit_logs(related_user_id);
