-- Mengizinkan multi-response per respondent untuk form yang sama.
-- Constraint lama ini memblokir insert ke-2 (form_id, respondent_id).
DROP INDEX IF EXISTS idx_responses_resp_form;
