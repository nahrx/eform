-- 0020_schema_versions.up.sql
-- Until now a response recorded only its answers, never the instrument those answers
-- were given against. forms.schema is overwritten in place on every save, so editing a
-- live instrument silently rewrote the meaning of every response already collected: an
-- option removed, a field renamed, a question inserted, and the old answers were read
-- back through the new schema with nothing to signal it.
--
-- For a census that runs for months and will certainly be revised mid-field, that is a
-- data integrity problem rather than an inconvenience. This migration keeps a snapshot
-- of every distinct schema a form has had, and pins each response to the one it was
-- actually filled against.

CREATE TABLE IF NOT EXISTS form_schema_versions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    form_id     UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    -- The version string the admin typed, kept for display. It is NOT the identity:
    -- admins forget to bump it, so two different schemas can share one label.
    version     TEXT NOT NULL DEFAULT '',
    schema      JSONB NOT NULL,
    -- Identity is the content. md5 of the JSONB rendered as text: Postgres normalises
    -- key order and whitespace in that cast, so the same instrument always hashes the
    -- same regardless of how the builder happened to serialise it. md5 is used because
    -- it is core Postgres (no pgcrypto), and this is deduplication, not security.
    schema_hash TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Saving a form without touching its questions must not pile up snapshots.
    UNIQUE (form_id, schema_hash)
);

CREATE INDEX IF NOT EXISTS idx_schema_versions_form
    ON form_schema_versions(form_id, created_at DESC);

ALTER TABLE form_responses
    ADD COLUMN IF NOT EXISTS schema_version_id UUID
        REFERENCES form_schema_versions(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_responses_schema_version
    ON form_responses(schema_version_id);

-- Backfill. Every existing form gets one snapshot of the schema it has right now, and
-- every existing response is pinned to it.
--
-- This is a best effort and the limit of it should be stated plainly: the schema being
-- captured is today's, not the one those responses were actually filled against, which
-- was never recorded and cannot be recovered. What the backfill buys is a floor —
-- from here on, further edits can no longer move the ground under this data.
INSERT INTO form_schema_versions (form_id, version, schema, schema_hash)
SELECT f.id, f.version, f.schema, md5(f.schema::text)
FROM forms f
ON CONFLICT (form_id, schema_hash) DO NOTHING;

UPDATE form_responses r
SET schema_version_id = v.id
FROM form_schema_versions v
WHERE v.form_id = r.form_id
  AND r.schema_version_id IS NULL;
