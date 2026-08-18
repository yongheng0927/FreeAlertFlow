-- 0010_user_bootstrap down

DROP INDEX IF EXISTS uq_users_single_bootstrap;

ALTER TABLE users DROP COLUMN IF EXISTS is_bootstrap;
