const express = require('express');

const router = express.Router();
const linesRouter = express.Router({ mergeParams: true });
const orders = [];

router.get('/', (req, res) => res.json(orders));
router.post('/', (req, res) => {
  orders.push(req.body);
  res.status(201).json(req.body);
});
router.get('/:id', (req, res) => res.json(orders[req.params.id] || null));

linesRouter.get('/', (req, res) => res.json([]));
router.use('/:id/lines', linesRouter);

module.exports = router;
