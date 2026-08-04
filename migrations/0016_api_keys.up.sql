-- 0016_api_keys.up.sql
-- API key per kuesioner: sistem eksternal menarik jawaban lewat endpoint /api/v1.
-- Bentuk kolom cakupannya sengaja sama dengan viewer_form_permissions (0008) supaya
-- pembatasan variabel/baris/responden bisa memakai helper query yang sama.

CREATE TABLE IF NOT EXISTS form_api_keys (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    form_id            UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    label              TEXT NOT NULL DEFAULT '',
    -- key_prefix hanya untuk identifikasi di panel & log; key aslinya tidak pernah
    -- disimpan — yang tersimpan cuma SHA-256 hex-nya di key_hash.
    key_prefix         TEXT NOT NULL,
    key_hash           TEXT NOT NULL UNIQUE,
    respondent_access  TEXT NOT NULL DEFAULT 'all'
        CHECK (respondent_access IN ('all', 'selected')),
    visible_fields     TEXT[],                            -- NULL = semua variabel
    field_filters      JSONB NOT NULL DEFAULT '{}',       -- fieldName -> nilai wajib
    include_respondent BOOLEAN NOT NULL DEFAULT false,    -- false = identitas responden disembunyikan
    allowed_ips        TEXT[],                            -- kosong/NULL = semua IP; isi = IP atau CIDR
    rate_limit_per_min INT NOT NULL DEFAULT 60,
    is_active          BOOLEAN NOT NULL DEFAULT true,
    expires_at         TIMESTAMPTZ,
    last_used_at       TIMESTAMPTZ,
    last_used_ip       TEXT,
    request_count      BIGINT NOT NULL DEFAULT 0,
    created_by         UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_fak_form ON form_api_keys(form_id);

-- Responden yang diizinkan (hanya berlaku jika respondent_access='selected')
CREATE TABLE IF NOT EXISTS api_key_allowed_respondents (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    permission_id  UUID NOT NULL REFERENCES form_api_keys(id) ON DELETE CASCADE,
    respondent_id  UUID NOT NULL REFERENCES respondents(id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(permission_id, respondent_id)
);
CREATE INDEX IF NOT EXISTS idx_akar_perm ON api_key_allowed_respondents(permission_id);

-- Audit: setiap panggilan /api/v1 dicatat, termasuk yang ditolak.
-- api_key_id sengaja nullable dan key_prefix disimpan terpisah supaya percobaan
-- dengan key yang tidak dikenal (atau key yang sudah dihapus) tetap tercatat.
CREATE TABLE IF NOT EXISTS api_access_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id  UUID REFERENCES form_api_keys(id) ON DELETE SET NULL,
    key_prefix  TEXT NOT NULL DEFAULT '',
    form_id     UUID REFERENCES forms(id) ON DELETE SET NULL,
    ip          TEXT NOT NULL DEFAULT '',
    path        TEXT NOT NULL DEFAULT '',
    query       TEXT NOT NULL DEFAULT '',
    status      INT  NOT NULL DEFAULT 0,
    row_count   INT  NOT NULL DEFAULT 0,
    error       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_aal_key  ON api_access_logs(api_key_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_aal_time ON api_access_logs(created_at DESC);
