import React, { useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import ReportHelp from './ReportHelp';

function Repairs() {
  const [repairs, setRepairs] = useState([]);
  useEffect(() => {
    fetch('/api/repairs').then((r) => r.json()).then(setRepairs);
  }, []);
  return (
    <main>
      <ul>{repairs.map((r) => <li key={r.id}>{r.customer}: {r.item}</li>)}</ul>
      <ReportHelp />
    </main>
  );
}

createRoot(document.getElementById('root')).render(<Repairs />);
