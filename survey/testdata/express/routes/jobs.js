const express = require('express');
const { pool } = require('../db');

const router = express.Router();

router.get('/', async (req, res) => {
  const { rows } = await pool.query('SELECT * FROM jobs WHERE assigned_to = $1 ORDER BY starts_on', [req.user.id]);
  res.json(rows);
});

router.post('/', async (req, res) => {
  const { rows } = await pool.query('INSERT INTO jobs (title, address, starts_on, assigned_to) VALUES ($1, $2, $3, $4) RETURNING *',
    [req.body.title, req.body.address, req.body.starts_on, req.user.id]);
  res.status(201).json(rows[0]);
});

router.put('/:id', async (req, res) => {
  const { rows } = await pool.query('UPDATE jobs SET title = $1 WHERE id = $2 RETURNING *', [req.body.title, req.params.id]);
  res.json(rows[0]);
});

module.exports = router;
