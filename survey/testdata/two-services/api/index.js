const express = require('express');
const Database = require('better-sqlite3');

const db = new Database('estimates.db');
db.exec('CREATE TABLE IF NOT EXISTS estimates (id INTEGER PRIMARY KEY, customer TEXT, total REAL)');

const app = express();
app.use(express.json());

app.get('/api/estimates', (req, res) => res.json(db.prepare('SELECT * FROM estimates').all()));
app.post('/api/estimates', async (req, res) => {
  const priced = await fetch(`${process.env.PRICER_URL}/price`, { method: 'POST', body: JSON.stringify(req.body) }).then((r) => r.json());
  db.prepare('INSERT INTO estimates (customer, total) VALUES (?, ?)').run(req.body.customer, priced.total);
  res.status(201).json(priced);
});

app.listen(process.env.PORT || 3000);
