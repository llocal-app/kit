import React from 'react';

export default function ReportHelp() {
  return (
    <aside>
      <h2>Reports</h2>
      <p>A report only reads. Something like this is turned away:</p>
      <pre>{`CREATE TABLE shop_settings (
  shop_name text PRIMARY KEY,
  opens_at time,
  closes_at time
);`}</pre>
    </aside>
  );
}
