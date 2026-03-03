DROP INDEX IF EXISTS idx_users_sso_subject;

ALTER TABLE users
  DROP COLUMN IF EXISTS auth_source,
  DROP COLUMN IF EXISTS sso_provider,
  DROP COLUMN IF EXISTS sso_subject;
