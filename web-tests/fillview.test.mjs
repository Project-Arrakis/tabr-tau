// The fill level of liquid containers (#367): the Inventory table has a Liquid column (amount, capacity and percent; empty; capacity unknown; over capacity), the card header
// counts the containers and how many are not full, and an item that holds no liquid has an empty cell. The base container view shows the same column.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const item = (id, tpl, fill, extra = {}) => ({ id, template_id: tpl, stack_size: 1, quality_level: 1, position_index: id, inventory_id: 1, inventory_type: 0, inventory_name: 'Backpack', durability: null, max_durability: null, augments: [], aug_limit: 0, fill, ...extra });
const INV = {
  items: [
    item(1, 'Literjon_T6', { kind: 'Water', amount: 11000, state: 'partial', capacity: 20000, percent: 55 }),
    item(2, 'Literjon', { kind: 'Water', amount: 1000, state: 'full', capacity: 1000, percent: 100 }),
    item(3, 'Bloodsack_01', { kind: 'Blood', amount: 0, state: 'empty', capacity: 3000, percent: 0 }),
    item(4, 'Bloodsack_03', { kind: 'Blood', amount: 500, state: 'unknown' }),
    item(5, 'Decajon', { kind: 'Water', amount: 12000, state: 'over', capacity: 10000, percent: 120 }),
    item(6, 'Knife', null),
  ],
  inventories: [{ id: 1, inventory_type: 0, max_item_count: 20, max_item_volume: 1000 }], templates: [],
};
async function openInventory(ui) {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
}
const cells = (ui) => [...ui.doc.querySelectorAll('table tbody tr')].map((tr) => [...tr.children].map((td) => td.textContent.trim()));

test('Inventory: the Liquid column shows amount, capacity and percent, empty, over capacity and unknown capacity', async () => {
  const ui = await boot({ get: { '/api/player/inventory': INV } });
  await openInventory(ui);
  const heads = [...ui.doc.querySelectorAll('table thead th')].map((th) => th.textContent.trim());
  const col = heads.findIndex((h) => h.startsWith('Liquid'));
  assert.ok(col >= 0, 'a Liquid column: ' + heads);
  const text = cells(ui).map((r) => r[col]);
  assert.deepEqual(text, ['Water 11,000 / 20,000 (55%)', 'Water 1,000 / 1,000 (100%)', 'Blood empty', 'Blood 500 (capacity unknown)', 'Water 12,000 / 10,000 (120%)', '']);
  ui.dom.window.close();
});

test('Inventory: the header counts the liquid containers and the ones that are not full', async () => {
  const ui = await boot({ get: { '/api/player/inventory': INV } });
  await openInventory(ui);
  const t = ui.doc.querySelector('#invfill').textContent;
  assert.equal(t, '5 liquid containers, 2 not full, 1 of unknown capacity.');
  ui.dom.window.close();
});

test('Inventory: a group with no liquid container has no count and nothing breaks', async () => {
  const ui = await boot({ get: { '/api/player/inventory': { ...INV, items: [item(6, 'Knife', null)] } } });
  await openInventory(ui);
  assert.equal(ui.doc.querySelector('#invfill'), null);
  ui.dom.window.close();
});
