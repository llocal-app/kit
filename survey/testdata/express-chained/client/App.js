import React, { useEffect, useState } from 'react';
import { Link, Route, Routes, useParams } from 'react-router-dom';

function Orders() {
  const [orders, setOrders] = useState([]);
  useEffect(() => {
    fetch('/api/orders').then((r) => r.json()).then(setOrders);
  }, []);
  return <ul>{orders.map((o) => <li key={o.id}><Link to={`/orders/${o.id}`}>{o.title}</Link></li>)}</ul>;
}

function Order() {
  const { orderId } = useParams();
  const done = () => fetch(`/api/orders/${orderId}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: '{"done":true}' });
  return <button onClick={done}>Mark order {orderId} done</button>;
}

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Orders />} />
      <Route path="/orders/:orderId" element={<Order />} />
    </Routes>
  );
}
