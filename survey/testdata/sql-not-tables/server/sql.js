export function readOnly(sql) {
  if (!/^\s*(select|with)\b/i.test(sql) || sql.includes(';')) throw new Error('Reports can only read.');
  return sql;
}
