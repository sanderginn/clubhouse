-- Allow email to be optional for users
ALTER TABLE users
  ALTER COLUMN email DROP NOT NULL;
