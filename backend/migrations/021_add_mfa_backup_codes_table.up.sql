CREATE TABLE IF NOT EXISTS mfa_backup_codes (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code_hash TEXT NOT NULL,
  used_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_mfa_backup_codes_user_id ON mfa_backup_codes (user_id);
CREATE INDEX IF NOT EXISTS idx_mfa_backup_codes_user_unused ON mfa_backup_codes (user_id) WHERE used_at IS NULL;
