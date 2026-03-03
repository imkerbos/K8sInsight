-- 用户表增加 SSO 相关字段
ALTER TABLE users
  ADD COLUMN auth_source   VARCHAR(16) NOT NULL DEFAULT 'local',
  ADD COLUMN sso_provider  VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN sso_subject   VARCHAR(255) NOT NULL DEFAULT '';

-- SSO 用户唯一约束（同一 provider + subject 只能关联一个用户）
CREATE UNIQUE INDEX idx_users_sso_subject
  ON users(sso_provider, sso_subject)
  WHERE sso_provider != '' AND sso_subject != '';
