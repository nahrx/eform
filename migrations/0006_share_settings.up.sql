ALTER TABLE form_shares
  ADD COLUMN IF NOT EXISTS multi_response BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS access_mode TEXT NOT NULL DEFAULT 'public';

CREATE TABLE IF NOT EXISTS share_allowed_emails (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  share_id   UUID NOT NULL REFERENCES form_shares(id) ON DELETE CASCADE,
  email      TEXT NOT NULL,
  note       TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(share_id, email)
);

CREATE INDEX IF NOT EXISTS idx_share_allowed_emails_share ON share_allowed_emails(share_id);
