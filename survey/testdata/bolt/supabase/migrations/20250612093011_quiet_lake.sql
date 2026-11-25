/*
  # Create quotes table

  1. New Tables
    - `quotes`
      - `id` (uuid, primary key)
      - `user_id` (uuid, the person who made it)
      - `customer`, `scope` (text)
      - `total_cents` (integer)
      - `letter` (text, the drafted letter)
  2. Security
    - Enable RLS on `quotes`
    - People read and change only their own quotes
*/

CREATE TABLE IF NOT EXISTS quotes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES auth.users(id) DEFAULT auth.uid(),
  customer text NOT NULL,
  scope text NOT NULL DEFAULT '',
  total_cents integer NOT NULL DEFAULT 0,
  letter text,
  created_at timestamptz DEFAULT now()
);

ALTER TABLE quotes ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users manage own quotes"
  ON quotes FOR ALL TO authenticated
  USING (auth.uid() = user_id)
  WITH CHECK (auth.uid() = user_id);
