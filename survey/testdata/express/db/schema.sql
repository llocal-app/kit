CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  email TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  name TEXT
);

CREATE TABLE jobs (
  id SERIAL PRIMARY KEY,
  title TEXT NOT NULL,
  address TEXT,
  starts_on DATE,
  assigned_to INTEGER REFERENCES users (id),
  invoice_id TEXT,
  paid BOOLEAN DEFAULT false
);
