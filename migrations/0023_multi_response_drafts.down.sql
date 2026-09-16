-- Restores the one-draft-per-respondent index from 0007.
-- Note: this fails if any respondent already holds two or more drafts for the same form
-- (which this migration's .up.sql made possible). Clear those first if you need to roll back.
DROP INDEX IF EXISTS idx_resp_draft_respondent;

CREATE UNIQUE INDEX IF NOT EXISTS unq_resp_draft_respondent
  ON form_responses(form_id, respondent_id)
  WHERE status = 'draft' AND respondent_id IS NOT NULL;
