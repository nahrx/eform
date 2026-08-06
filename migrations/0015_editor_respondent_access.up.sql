-- 0015_editor_respondent_access.up.sql
-- Respondent access for editors (mirrors viewer_form_permissions.respondent_access)

ALTER TABLE editor_form_permissions
  ADD COLUMN IF NOT EXISTS respondent_access TEXT NOT NULL DEFAULT 'all'
    CHECK (respondent_access IN ('all', 'selected'));

-- Allowed respondents (only applies when respondent_access='selected')
CREATE TABLE IF NOT EXISTS editor_allowed_respondents (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    permission_id  UUID NOT NULL REFERENCES editor_form_permissions(id) ON DELETE CASCADE,
    respondent_id  UUID NOT NULL REFERENCES respondents(id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(permission_id, respondent_id)
);
CREATE INDEX IF NOT EXISTS idx_ear_perm ON editor_allowed_respondents(permission_id);
