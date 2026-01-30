-- Remove suspended_at from users table
ALTER TABLE users
  DROP COLUMN suspended_at;
