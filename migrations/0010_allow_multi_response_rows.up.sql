-- Allows multiple responses per respondent for the same form.
-- The old constraint blocked a second insert for (form_id, respondent_id).
DROP INDEX IF EXISTS idx_responses_resp_form;
