CREATE TABLE invoices (
  id SERIAL PRIMARY KEY,
  customer TEXT NOT NULL,
  total NUMERIC(10, 2) NOT NULL
);
