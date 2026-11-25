-- The repair desk's first tables.
CREATE TABLE repairs (
  id serial PRIMARY KEY,
  customer text NOT NULL,
  item text NOT NULL,
  received_on date NOT NULL DEFAULT current_date
);

CREATE TABLE "repairNotes" (
  id serial PRIMARY KEY,
  repair_id integer NOT NULL REFERENCES repairs (id),
  body text NOT NULL
);
