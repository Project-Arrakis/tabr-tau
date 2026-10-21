// The Give queue is kept in local storage across a refresh, and what is read back is checked field by field: storage is not trusted.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const src = fs.readFileSync(path.join(here, '..', 'internal', 'web', 'static', 'app.js'), 'utf8');
const start = src.indexOf('function loadQueue()');
const end = src.indexOf('// ---------- status bar');
assert.ok(start > 0 && end > start, 'the queue helpers are not where the test expects them');

function make(stored) {
  const store = new Map(stored === undefined ? [] : [['giveQueue', stored]]);
  const localStorage = { getItem: (k) => (store.has(k) ? store.get(k) : null), setItem: (k, v) => store.set(k, String(v)), removeItem: (k) => store.delete(k) };
  const fn = new Function('localStorage', `let giveQueue = [];\n${src.slice(start, end)}\nreturn { loadQueue, keepQueue, get q() { return giveQueue; }, set q(v) { giveQueue = v; } };`);
  return { api: fn(localStorage), store };
}

test('a queue is written on change and read back after a reload', () => {
  const a = make();
  a.api.q = [{ template_id: 'IronBar', quantity: 5, quality: 2 }, { template_id: 'Helmet', quantity: 1, quality: 0, augments: ['A1', 'A2'], grade: 3 }];
  a.api.keepQueue();
  const b = make(a.store.get('giveQueue'));
  assert.deepEqual(b.api.loadQueue(), a.api.q);
  a.api.q = [];
  a.api.keepQueue();
  assert.equal(a.store.has('giveQueue'), false, 'an empty queue leaves nothing in storage');
});

test('what storage holds is checked: wrong types, huge numbers, long strings and junk are cut or dropped', () => {
  const junk = JSON.stringify([
    { template_id: 'Ok', quantity: 9e99, quality: 99 },
    { template_id: 7, quantity: 1 },
    { template_id: 'x'.repeat(500), quantity: 1 },
    null, 'text', [],
    { template_id: 'Aug', quantity: 'NaN', quality: -3, augments: ['a', 5], grade: 2 },
    { template_id: 'Aug2', quantity: 2, augments: ['a', 'b', 'c', 'd'] },
    { template_id: '<img src=x onerror=alert(1)>', quantity: 1.7, quality: 1.2 },
  ]);
  const got = make(junk).api.loadQueue();
  assert.deepEqual(got.map((e) => e.template_id), ['Ok', 'Aug', 'Aug2', '<img src=x onerror=alert(1)>']);
  assert.equal(got[0].quantity, 1e6);
  assert.equal(got[0].quality, 5);
  assert.equal(got[1].quantity, 1);
  assert.equal(got[1].quality, 0);
  assert.equal(got[1].augments, undefined, 'augments that are not all text are dropped');
  assert.equal(got[2].augments, undefined, 'more than three augments are dropped');
  assert.equal(got[3].quantity, 1);
  assert.equal(got[3].quality, 1);
  for (const bad of ['not json', '{"a":1}', '"str"', '42']) assert.deepEqual(make(bad).api.loadQueue(), [], bad);
  assert.deepEqual(make().api.loadQueue(), [], 'nothing stored');
});
