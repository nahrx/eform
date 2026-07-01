-- 0005_drafts.up.sql
-- Tabel draf server: simpan progres pengisian yang belum final.

CREATE TABLE IF NOT EXISTS response_drafts (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    form_id        UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    share_id       UUID REFERENCES form_shares(id) ON DELETE SET NULL,
    respondent_id  UUID NOT NULL REFERENCES respondents(id) ON DELETE CASCADE,
    answers        JSONB NOT NULL DEFAULT '{}',
    cur_page       INTEGER NOT NULL DEFAULT 0,
    saved_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(form_id, respondent_id)
);

CREATE INDEX IF NOT EXISTS idx_drafts_respondent ON response_drafts(respondent_id);
