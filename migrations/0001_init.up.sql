-- 0001_init.up.sql — skema awal eForm backend
-- Membutuhkan PostgreSQL 13+ (gen_random_uuid tersedia di core).

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT UNIQUE NOT NULL,
    email         TEXT UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'editor' CHECK (role IN ('superadmin', 'admin', 'editor')),
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS forms (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        TEXT UNIQUE NOT NULL,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    schema      JSONB NOT NULL DEFAULT '{}'::jsonb,   -- instrumen v1.1 dari eForm Builder
    status      TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    version     TEXT NOT NULL DEFAULT '1.0.0',
    owner_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_forms_owner ON forms(owner_id);
CREATE INDEX IF NOT EXISTS idx_forms_status ON forms(status);

-- Share publik: tiap kuesioner bisa punya >=1 link share.
CREATE TABLE IF NOT EXISTS form_shares (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    form_id         UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    token           TEXT UNIQUE NOT NULL,
    label           TEXT NOT NULL DEFAULT '',
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    allow_responses BOOLEAN NOT NULL DEFAULT TRUE,
    password_hash   TEXT,                 -- opsional: proteksi password
    expires_at      TIMESTAMPTZ,          -- opsional: kedaluwarsa
    view_count      BIGINT NOT NULL DEFAULT 0,
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_shares_form ON form_shares(form_id);
CREATE INDEX IF NOT EXISTS idx_shares_token ON form_shares(token);

-- Jawaban dari publik via share.
CREATE TABLE IF NOT EXISTS form_responses (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    form_id      UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    share_id     UUID REFERENCES form_shares(id) ON DELETE SET NULL,
    answers      JSONB NOT NULL DEFAULT '{}'::jsonb,
    meta         JSONB NOT NULL DEFAULT '{}'::jsonb,   -- ip, user agent, dll.
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_responses_form ON form_responses(form_id);
CREATE INDEX IF NOT EXISTS idx_responses_share ON form_responses(share_id);
