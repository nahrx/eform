-- 0014_user_language.up.sql — per-account UI language preference (builder & dashboard)
ALTER TABLE users ADD COLUMN IF NOT EXISTS preferred_language TEXT NOT NULL DEFAULT 'id';
