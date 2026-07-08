-- 0011_editor_permissions.up.sql
-- Hak akses editor per kuesioner (untuk mengelola form yang ditugaskan)

CREATE TABLE IF NOT EXISTS editor_form_permissions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    editor_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    form_id    UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(editor_id, form_id)
);

CREATE INDEX IF NOT EXISTS idx_efp_editor ON editor_form_permissions(editor_id);
CREATE INDEX IF NOT EXISTS idx_efp_form ON editor_form_permissions(form_id);
