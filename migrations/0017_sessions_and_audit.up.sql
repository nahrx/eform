-- 0017_sessions_and_audit.up.sql
-- 1) Session revocation: token_version is bumped whenever an account is deactivated or
--    its password changes, so older JWTs carrying a lower version are rejected.
-- 2) Admin action audit: who did what, when, and from which IP.

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS token_version INT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS activity_logs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    -- The actor's name is stored separately so the trail stays readable even if the account is deleted.
    actor_name   TEXT NOT NULL DEFAULT '',
    actor_role   TEXT NOT NULL DEFAULT '',
    action       TEXT NOT NULL,              -- mis. 'form.delete', 'export.csv'
    target_type  TEXT NOT NULL DEFAULT '',   -- 'form' | 'user' | 'share' | 'permission'
    target_id    TEXT NOT NULL DEFAULT '',
    target_label TEXT NOT NULL DEFAULT '',   -- title/username, so the log stays meaningful
    form_id      UUID REFERENCES forms(id) ON DELETE SET NULL,
    ip           TEXT NOT NULL DEFAULT '',
    detail       TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_alog_time  ON activity_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_alog_actor ON activity_logs(actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_alog_form  ON activity_logs(form_id, created_at DESC);
