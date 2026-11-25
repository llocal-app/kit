const express = require('express');
const orders = require('./routes/orders');

const app = express();
const api = express.Router();

api.get('/status', (req, res) => res.json({ ok: true }));
api.use('/orders', orders);

app.use(express.json());
app.use('/api', api);

app.listen(process.env.PORT || 3000);
