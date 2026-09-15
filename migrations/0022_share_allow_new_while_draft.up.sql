ALTER TABLE form_shares
  ADD COLUMN IF NOT EXISTS allow_new_while_draft BOOLEAN NOT NULL DEFAULT false;
