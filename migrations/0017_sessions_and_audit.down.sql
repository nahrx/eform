-- 0017_sessions_and_audit.down.sql
DROP TABLE IF EXISTS activity_logs;

ALTER TABLE users
  DROP COLUMN IF EXISTS token_version;
