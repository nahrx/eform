-- 0003_respondents.up.sql
-- Tabel untuk responden publik yang login via Google OAuth.

CREATE TABLE IF NOT EXISTS respondents (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    google_id   TEXT UNIQUE NOT NULL,
    email       TEXT NOT NULL,
    name        TEXT NOT NULL DEFAULT '',
    picture     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Tambah kolom respondent_id ke form_responses.
ALTER TABLE form_responses
    ADD COLUMN IF NOT EXISTS respondent_id UUID REFERENCES respondents(id) ON DELETE SET NULL;

-- Satu jawaban per form per respondent (basis ON CONFLICT untuk upsert).
CREATE UNIQUE INDEX IF NOT EXISTS idx_responses_resp_form
    ON form_responses(form_id, respondent_id)
    WHERE respondent_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_responses_respondent ON form_responses(respondent_id);
