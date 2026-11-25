const orders = [];

export const list = (req, res) => res.json(orders);

export const create = (req, res) => {
  const order = { id: String(orders.length + 1), title: req.body.title, done: false };
  orders.push(order);
  res.status(201).json(order);
};

export const update = (req, res) => {
  const order = orders.find((o) => o.id === req.params.orderId);
  Object.assign(order, req.body);
  res.json(order);
};

export const remove = (id) => {
  orders.splice(orders.findIndex((o) => o.id === id), 1);
};
