-- 0023_multi_response_drafts.up.sql
-- Lets a multi-response share hold more than one draft per respondent.
--
-- 0007 added a partial UNIQUE index that allowed exactly one 'draft' row per
-- (form_id, respondent_id). 0022 then added form_shares.allow_new_while_draft, whose
-- whole purpose is to let a respondent keep several responses going at once — but the
-- index still rejected the second draft, so "Save Draft" on a new response failed with
-- a unique violation, which the API reported as "failed to save response".
--
-- The one-draft rule depends on a per-share setting, which a table-level index cannot
-- see, so it now lives in the application (publicSubmit calls HasDraftResponse before
-- creating a new row when the share does not allow new responses while a draft is open).
DROP INDEX IF EXISTS unq_resp_draft_respondent;

-- Same columns, without the uniqueness: the lookup that rule performs stays fast.
CREATE INDEX IF NOT EXISTS idx_resp_draft_respondent
  ON form_responses(form_id, respondent_id)
  WHERE status = 'draft' AND respondent_id IS NOT NULL;
