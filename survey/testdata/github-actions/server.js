const express = require('express');
const cron = require('node-cron');

const app = express();
const readings = [];

app.use(express.json());

app.post('/api/readings', (req, res) => {
  readings.push({ freezer: req.body.freezer, celsius: req.body.celsius, at: new Date() });
  res.status(201).end();
});

app.get('/api/readings', (req, res) => res.json(readings));

cron.schedule('0 * * * *', () => {
  readings.splice(0, Math.max(0, readings.length - 1000));
});

app.listen(process.env.PORT || 3000);
