-- 0003_respondents.down.sql
DROP INDEX IF EXISTS idx_responses_resp_form;
DROP INDEX IF EXISTS idx_responses_respondent;
ALTER TABLE form_responses DROP COLUMN IF EXISTS respondent_id;
DROP TABLE IF EXISTS respondents;
