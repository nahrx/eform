-- 0018_revisions_and_indexes.down.sql
DROP INDEX IF EXISTS idx_responses_answers_gin;
DROP INDEX IF EXISTS idx_drafts_form_time;
DROP INDEX IF EXISTS idx_responses_form_status;
DROP INDEX IF EXISTS idx_responses_form_time;
DROP TABLE IF EXISTS response_revisions;
