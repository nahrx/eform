-- 0018_revisions_and_indexes.up.sql
-- 1) Trail of answer changes made by editors: the before & after values are stored in full,
--    so every data correction can be traced and compared.
-- 2) Indexes for the most common response-list query patterns.

CREATE TABLE IF NOT EXISTS response_revisions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    response_id    UUID NOT NULL,
    form_id        UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    -- The editor's name is stored separately so the trail stays readable even if the account is deleted.
    editor_id      UUID REFERENCES users(id) ON DELETE SET NULL,
    editor_name    TEXT NOT NULL DEFAULT '',
    answers_before JSONB NOT NULL DEFAULT '{}',
    answers_after  JSONB NOT NULL DEFAULT '{}',
    ip             TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_revisions_response ON response_revisions(response_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_revisions_form     ON response_revisions(form_id, created_at DESC);

-- The response list is always filtered per form and then sorted by submission time; a
-- composite index removes the need for a separate sort step.
CREATE INDEX IF NOT EXISTS idx_responses_form_time   ON form_responses(form_id, submitted_at DESC);
CREATE INDEX IF NOT EXISTS idx_responses_form_status ON form_responses(form_id, status);
CREATE INDEX IF NOT EXISTS idx_drafts_form_time      ON response_drafts(form_id, saved_at DESC);

-- A GIN index on answers enables searching inside the answers (the @> and ? operators).
-- Note: filters of the form `answers->>'x' = $1` do NOT use this index; if a particular
-- field is filtered often and the table has grown large, add a dedicated expression
-- index for it, e.g.
--   CREATE INDEX idx_resp_kabupaten ON form_responses ((answers->>'kabupaten_kota'));
CREATE INDEX IF NOT EXISTS idx_responses_answers_gin ON form_responses USING GIN (answers);
