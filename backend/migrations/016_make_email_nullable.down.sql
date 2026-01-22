-- Revert email to required and ensure non-null values
UPDATE users
SET email = CONCAT('restored_', id, '@example.invalid')
WHERE email IS NULL;

ALTER TABLE users
  ALTER COLUMN email SET NOT NULL;
