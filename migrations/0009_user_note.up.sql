-- 0009_user_note.up.sql — catatan opsional pada akun user (dipakai untuk akun viewer)
ALTER TABLE users ADD COLUMN IF NOT EXISTS note TEXT;
