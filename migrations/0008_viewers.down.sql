-- 0008_viewers.down.sql
DROP TABLE IF EXISTS viewer_allowed_respondents;
DROP TABLE IF EXISTS viewer_form_permissions;

DELETE FROM users WHERE role = 'viewer';

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('superadmin', 'admin', 'editor'));
