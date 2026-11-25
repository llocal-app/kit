import assert from 'node:assert/strict';
import { test } from 'node:test';
import { readOnly } from './sql.js';

test('a report reads', () => {
  assert.equal(readOnly('SELECT * FROM repairs'), 'SELECT * FROM repairs');
});

test('a report cannot write', () => {
  assert.throws(() => readOnly('CREATE TABLE scratch (id int, payload text)'));
  assert.throws(() => readOnly('ALTER TABLE repairs ADD COLUMN hidden text'));
});
