-- 0016_api_keys.up.sql
-- API keys per form: external systems pull responses through the /api/v1 endpoints.
-- The scope columns deliberately mirror viewer_form_permissions (0008) so that the
-- same query helper can apply the field/row/respondent restrictions.

CREATE TABLE IF NOT EXISTS form_api_keys (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    form_id            UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    label              TEXT NOT NULL DEFAULT '',
    -- key_prefix is only for identification in the panel & logs; the real key is never
    -- stored — only its SHA-256 hex digest is kept in key_hash.
    key_prefix         TEXT NOT NULL,
    key_hash           TEXT NOT NULL UNIQUE,
    respondent_access  TEXT NOT NULL DEFAULT 'all'
        CHECK (respondent_access IN ('all', 'selected')),
    visible_fields     TEXT[],                            -- NULL = every field
    field_filters      JSONB NOT NULL DEFAULT '{}',       -- fieldName -> required value
    include_respondent BOOLEAN NOT NULL DEFAULT false,    -- false = the respondent's identity is hidden
    allowed_ips        TEXT[],                            -- empty/NULL = any IP; set = IP or CIDR
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

-- Allowed respondents (only applies when respondent_access='selected')
CREATE TABLE IF NOT EXISTS api_key_allowed_respondents (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    permission_id  UUID NOT NULL REFERENCES form_api_keys(id) ON DELETE CASCADE,
    respondent_id  UUID NOT NULL REFERENCES respondents(id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(permission_id, respondent_id)
);
CREATE INDEX IF NOT EXISTS idx_akar_perm ON api_key_allowed_respondents(permission_id);

-- Audit: every /api/v1 call is recorded, including rejected ones.
-- api_key_id is deliberately nullable and key_prefix is stored separately so that attempts
-- with an unknown (or already deleted) key are still recorded.
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
