-- 0009_user_note.up.sql — optional note on a user account (used for viewer accounts)
ALTER TABLE users ADD COLUMN IF NOT EXISTS note TEXT;
