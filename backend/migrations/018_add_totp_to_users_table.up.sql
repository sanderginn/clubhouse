ALTER TABLE users
  ADD COLUMN totp_secret_encrypted BYTEA,
  ADD COLUMN totp_enabled BOOLEAN NOT NULL DEFAULT false;
