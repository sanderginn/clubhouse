-- Seed default admin user (password: Admin123!)
-- Only inserts if no admin exists
INSERT INTO users (username, email, password_hash, is_admin, approved_at)
SELECT 'admin', 'admin@clubhouse.local', '$2a$12$DeQhCv6P2LDlcVfjudk3JOV4VUIg/FHnTxLh7t4Jvxfduf4Ct4BNO', true, now()
WHERE NOT EXISTS (SELECT 1 FROM users WHERE is_admin = true);
