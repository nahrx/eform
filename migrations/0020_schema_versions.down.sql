-- 0020_schema_versions.down.sql
-- Dropping this discards which instrument each response was filled against, and that
-- information exists nowhere else.
ALTER TABLE form_responses DROP COLUMN IF EXISTS schema_version_id;
DROP TABLE IF EXISTS form_schema_versions;
