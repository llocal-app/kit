const express = require('express');
const digest = require('./digest');

const app = express();
const notes = [];

app.use(express.json());

app.post('/api/notes', (req, res) => {
  notes.push({ text: req.body.text, at: new Date() });
  res.status(201).end();
});

app.get('/api/notes', (req, res) => res.json(notes));

digest.start(notes);
app.listen(process.env.PORT || 3000);
