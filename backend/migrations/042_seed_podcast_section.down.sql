-- No-op rollback by design.
-- This seed migration cannot safely identify which podcast rows were created by the migration.
-- Deleting by type/name risks removing pre-existing user-managed rows and can violate FKs
-- (for example via section_subscriptions). Keep existing data intact on rollback.
SELECT 1;
