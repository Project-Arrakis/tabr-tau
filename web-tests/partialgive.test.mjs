// A give puts in as much as fits (#345): the toast says how much went in and why not more, as a warning (not an error) that stays until dismissed; a give that fits nothing is an error
// with the editor's reason; a whole give says "Item added".
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const CATALOG = [{ id: 'IronBar', name: 'Iron Bar', category: 'resources' }];
const INV = { items: [], inventories: [{ id: 1, inventory_type: 0, max_item_count: 5, max_item_volume: 1000 }], templates: [] };
const posts = (ui, p) => ui.posted.filter((x) => x.path === p);

async function openGive(ui, qty) {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  ui.doc.querySelector('#gt').value = 'Iron Bar';
  ui.doc.querySelector('#gq').value = String(qty);
}
const boot_ = (post) => boot({ get: { '/api/catalog/items': CATALOG, '/api/player/inventory': INV }, post });

test('Give item: part of it fitted is a warning with the real amount and the reason, and it stays until dismissed', async () => {
  const ui = await boot_({ '/api/player/give': { ok: true, requested: 500, given: 340, reason: 'the backpack is full', limit: 'slots', outcome: 'written' } });
  await openGive(ui, 500);
  ui.click('[data-act="give"]'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/player/give').map((p) => p.body), [{ template_id: 'IronBar', quantity: 500, quality: 0 }]);
  const t = ui.doc.querySelector('#toast .part');
  assert.ok(t, 'a warning, not an error');
  assert.equal(ui.doc.querySelector('#toast .err'), null);
  assert.ok(t.textContent.includes('Gave 340 of 500 Iron Bar: the backpack is full'), t.textContent);
  assert.ok(t.querySelector('button.x'), 'it can be dismissed');
  assert.equal(t.getAttribute('role'), 'status');
  ui.dom.window.close();
});

test('Give item: a whole give says Item added, and a give that fits nothing shows the editor\'s refusal as an error', async () => {
  let ui = await boot_({ '/api/player/give': { ok: true, requested: 5, given: 5, reason: '', limit: '', outcome: 'written' } });
  await openGive(ui, 5);
  ui.click('[data-act="give"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Item added'));
  assert.equal(ui.doc.querySelector('#toast .part'), null);
  assert.equal(ui.doc.querySelector('#toast .err'), null);
  ui.dom.window.close();

  ui = await boot_({ '/api/player/give': { __fail: 'Nothing was given: the backpack is full.', __code: 'no_room', __status: 409 } });
  await openGive(ui, 5);
  ui.click('[data-act="give"]'); await ui.settle();
  const err = ui.doc.querySelector('#toast .err');
  assert.ok(err && err.textContent.includes('Nothing was given: the backpack is full'));
  assert.ok(!ui.doc.querySelector('#toast').textContent.includes('Item added'));
  ui.dom.window.close();
});
