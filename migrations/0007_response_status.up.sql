ALTER TABLE form_responses
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'submitted';

-- Hanya boleh ada satu draf per respondent per form
CREATE UNIQUE INDEX IF NOT EXISTS unq_resp_draft_respondent
  ON form_responses(form_id, respondent_id)
  WHERE status = 'draft' AND respondent_id IS NOT NULL;
