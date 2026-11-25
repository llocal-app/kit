CREATE TABLE IF NOT EXISTS quote_drafts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  body text
);
