ALTER TABLE users
  DROP COLUMN IF EXISTS totp_secret_encrypted,
  DROP COLUMN IF EXISTS totp_enabled;
