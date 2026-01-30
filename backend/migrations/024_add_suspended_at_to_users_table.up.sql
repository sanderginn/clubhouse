-- Add suspended_at to users table
ALTER TABLE users
  ADD COLUMN suspended_at TIMESTAMP;
