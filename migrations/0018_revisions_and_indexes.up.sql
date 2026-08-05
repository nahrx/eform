-- 0018_revisions_and_indexes.up.sql
-- 1) Jejak perubahan jawaban oleh editor: nilai sebelum & sesudah disimpan utuh,
--    supaya setiap koreksi data bisa ditelusuri dan dibandingkan.
-- 2) Index untuk pola query daftar jawaban yang paling sering dipakai.

CREATE TABLE IF NOT EXISTS response_revisions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    response_id    UUID NOT NULL,
    form_id        UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    -- Nama editor disimpan terpisah agar jejaknya tetap terbaca walau akunnya dihapus.
    editor_id      UUID REFERENCES users(id) ON DELETE SET NULL,
    editor_name    TEXT NOT NULL DEFAULT '',
    answers_before JSONB NOT NULL DEFAULT '{}',
    answers_after  JSONB NOT NULL DEFAULT '{}',
    ip             TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_revisions_response ON response_revisions(response_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_revisions_form     ON response_revisions(form_id, created_at DESC);

-- Daftar jawaban selalu difilter per kuesioner lalu diurutkan waktu kirim; index
-- gabungan membuat urutannya tidak perlu sort terpisah.
CREATE INDEX IF NOT EXISTS idx_responses_form_time   ON form_responses(form_id, submitted_at DESC);
CREATE INDEX IF NOT EXISTS idx_responses_form_status ON form_responses(form_id, status);
CREATE INDEX IF NOT EXISTS idx_drafts_form_time      ON response_drafts(form_id, saved_at DESC);

-- GIN pada answers menyiapkan pencarian isi jawaban (operator @> dan ?).
-- Catatan: filter bentuk `answers->>'x' = $1` TIDAK memakai index ini; kalau ada
-- variabel tertentu yang sering difilter dan datanya sudah besar, tambahkan index
-- ekspresi khusus, mis.
--   CREATE INDEX idx_resp_kabupaten ON form_responses ((answers->>'kabupaten_kota'));
CREATE INDEX IF NOT EXISTS idx_responses_answers_gin ON form_responses USING GIN (answers);
