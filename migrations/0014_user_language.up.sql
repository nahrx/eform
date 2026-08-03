-- 0014_user_language.up.sql — preferensi bahasa UI (builder & dashboard) per akun
ALTER TABLE users ADD COLUMN IF NOT EXISTS preferred_language TEXT NOT NULL DEFAULT 'id';
