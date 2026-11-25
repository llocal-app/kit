import express from 'express';
import * as store from './store';

const router = express.Router();

router.route('/api/orders')
  .get(store.list)
  .post(store.create);

router.route('/api/orders/:orderId')
  // .get(store.read)
  .put(store.update)
  .delete((req, res) => {
    store.remove(req.params.orderId);
    res.status(204).end();
  });

export default router;
