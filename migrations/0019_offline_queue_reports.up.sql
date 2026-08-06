-- 0019_offline_queue_reports.up.sql
-- Devices in offline mode hold answers and photos that exist nowhere else until they
-- are sent. Until now that backlog was visible only on the device itself: if a phone
-- broke or its browser data was cleared, nobody at the office ever knew work had been
-- stranded there. This table is what the devices report into.
--
-- Deliberately metadata only — counts, HTTP statuses, error texts, timestamps. No
-- answer content. Recovering the data still means reaching the user; the point
-- here is to know that you have to.

CREATE TABLE IF NOT EXISTS offline_queue_reports (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    form_id        UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    respondent_id  UUID NOT NULL REFERENCES respondents(id) ON DELETE CASCADE,
    -- A respondent may work from more than one device, and each has its own queue.
    device_id      TEXT NOT NULL,

    pending        INT NOT NULL DEFAULT 0,
    failed         INT NOT NULL DEFAULT 0,
    files          INT NOT NULL DEFAULT 0,
    -- How long the oldest item has been waiting: the number that says whether this is
    -- a blip or a week of lost fieldwork.
    oldest_queued_at TIMESTAMPTZ,
    -- [{kind, status, error, queuedAt, failedAt, attempts}] — capped by the handler.
    items          JSONB NOT NULL DEFAULT '[]',
    user_agent     TEXT NOT NULL DEFAULT '',
    reported_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- One row per device holding its latest state, not a history. The question being
    -- answered is "what is stuck right now", and an append-only log would grow without
    -- bound for something nobody reads back.
    UNIQUE (form_id, respondent_id, device_id)
);

-- The dashboard asks for one form's reports, worst first.
CREATE INDEX IF NOT EXISTS idx_queue_reports_form
    ON offline_queue_reports(form_id, reported_at DESC);

-- A device whose respondent token expired stops reporting altogether, so the age of
-- the newest report is itself a signal. This index serves the staleness scan.
CREATE INDEX IF NOT EXISTS idx_queue_reports_stale
    ON offline_queue_reports(reported_at)
    WHERE pending > 0 OR failed > 0;
