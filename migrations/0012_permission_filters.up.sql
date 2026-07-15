ALTER TABLE viewer_form_permissions
  ADD COLUMN IF NOT EXISTS field_filters JSONB NOT NULL DEFAULT '{}';

ALTER TABLE editor_form_permissions
  ADD COLUMN IF NOT EXISTS field_filters JSONB NOT NULL DEFAULT '{}';
