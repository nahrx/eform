-- 0003_respondents.up.sql
-- Table for public respondents who sign in via Google OAuth.

CREATE TABLE IF NOT EXISTS respondents (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    google_id   TEXT UNIQUE NOT NULL,
    email       TEXT NOT NULL,
    name        TEXT NOT NULL DEFAULT '',
    picture     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Add the respondent_id column to form_responses.
ALTER TABLE form_responses
    ADD COLUMN IF NOT EXISTS respondent_id UUID REFERENCES respondents(id) ON DELETE SET NULL;

-- One response per form per respondent (the basis for the upsert's ON CONFLICT).
CREATE UNIQUE INDEX IF NOT EXISTS idx_responses_resp_form
    ON form_responses(form_id, respondent_id)
    WHERE respondent_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_responses_respondent ON form_responses(respondent_id);
