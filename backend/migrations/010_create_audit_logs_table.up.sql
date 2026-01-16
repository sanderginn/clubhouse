-- Create audit_logs table
CREATE TABLE audit_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_user_id UUID NOT NULL REFERENCES users(id),
  action VARCHAR(50) NOT NULL,
  related_post_id UUID REFERENCES posts(id),
  related_comment_id UUID REFERENCES comments(id),
  created_at TIMESTAMP NOT NULL DEFAULT now()
);

-- Create indexes for audit_logs table
CREATE INDEX idx_audit_logs_admin_id ON audit_logs(admin_user_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
