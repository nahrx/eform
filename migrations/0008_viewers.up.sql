-- 0008_viewers.up.sql — hak akses viewer

-- Tambah role 'viewer' ke constraint users
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('superadmin', 'admin', 'editor', 'viewer'));

-- Hak akses viewer per kuesioner
CREATE TABLE IF NOT EXISTS viewer_form_permissions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    viewer_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    form_id           UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    respondent_access TEXT NOT NULL DEFAULT 'all'
        CHECK (respondent_access IN ('all', 'selected')),
    visible_fields    TEXT[],   -- NULL = semua field terlihat
    created_by        UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(viewer_id, form_id)
);
CREATE INDEX IF NOT EXISTS idx_vfp_viewer ON viewer_form_permissions(viewer_id);
CREATE INDEX IF NOT EXISTS idx_vfp_form   ON viewer_form_permissions(form_id);

-- Responden yang diizinkan (hanya berlaku jika respondent_access='selected')
CREATE TABLE IF NOT EXISTS viewer_allowed_respondents (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    permission_id  UUID NOT NULL REFERENCES viewer_form_permissions(id) ON DELETE CASCADE,
    respondent_id  UUID NOT NULL REFERENCES respondents(id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(permission_id, respondent_id)
);
CREATE INDEX IF NOT EXISTS idx_var_perm ON viewer_allowed_respondents(permission_id);
