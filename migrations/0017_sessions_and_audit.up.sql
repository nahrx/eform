-- 0017_sessions_and_audit.up.sql
-- 1) Pencabutan sesi: token_version dinaikkan setiap kali akun dinonaktifkan atau
--    passwordnya diganti, sehingga JWT lama yang membawa versi lebih rendah ditolak.
-- 2) Audit aksi admin: siapa melakukan apa, kapan, dari IP mana.

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS token_version INT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS activity_logs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    -- Nama pelaku disimpan terpisah supaya jejaknya tetap terbaca walau akunnya dihapus.
    actor_name   TEXT NOT NULL DEFAULT '',
    actor_role   TEXT NOT NULL DEFAULT '',
    action       TEXT NOT NULL,              -- mis. 'form.delete', 'export.csv'
    target_type  TEXT NOT NULL DEFAULT '',   -- 'form' | 'user' | 'share' | 'permission'
    target_id    TEXT NOT NULL DEFAULT '',
    target_label TEXT NOT NULL DEFAULT '',   -- judul/username, agar log tetap bermakna
    form_id      UUID REFERENCES forms(id) ON DELETE SET NULL,
    ip           TEXT NOT NULL DEFAULT '',
    detail       TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_alog_time  ON activity_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_alog_actor ON activity_logs(actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_alog_form  ON activity_logs(form_id, created_at DESC);
