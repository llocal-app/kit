import express from 'express';
import pg from 'pg';
import { readOnly } from './sql.js';

const pool = new pg.Pool({ connectionString: process.env.DATABASE_URL });
const app = express();

app.use(express.json());

app.get('/api/repairs', async (req, res) => {
  const { rows } = await pool.query('SELECT * FROM repairs ORDER BY received_on DESC');
  res.json(rows);
});

app.post('/api/repairs/:id/notes', async (req, res) => {
  await pool.query('INSERT INTO repair_notes (repair_id, body) VALUES ($1, $2)', [req.params.id, req.body.body]);
  res.status(201).end();
});

app.post('/api/report', async (req, res) => {
  const { rows } = await pool.query(readOnly(req.body.sql));
  res.json(rows);
});

app.listen(process.env.PORT || 3000);
