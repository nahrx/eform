-- 0021_share_row_display.up.sql — which answers a multi-response share shows on each
-- row of the respondent's own list. Empty means fall back to "Response 1, 2, 3…".
-- A text array rather than JSONB: it is only ever a flat list of dataKeys.
ALTER TABLE form_shares
  ADD COLUMN IF NOT EXISTS row_display TEXT[] NOT NULL DEFAULT '{}';
