-- 0015_editor_respondent_access.down.sql
DROP TABLE IF EXISTS editor_allowed_respondents;

ALTER TABLE editor_form_permissions
  DROP COLUMN IF EXISTS respondent_access;
