// The cargo hold card of a vehicle shows the room its parts give (#483): a vehicle with no storage part has no Add item row and says so (rows already in it can still be deleted or cleared),
// a vehicle with a hold reads "N of M slots used" and never "of 0", and a give that fits only in part says how much did.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { proseWords, longestBlock } from './uitext.mjs';

afterEach(closeAll);

const item = (id, pos) => ({ id: 9000 + id, template_id: 'DolomiteRock', stack_size: 500, quality_level: 0, position_index: pos });
const veh = (type, cargo) => ({ vehicles: [{ id: 41, name: `BP_${type}_CHOAM`, type, map: 'HaggaBasin', condition: 60, fuel: 10, x: 1, y: 2, components: [], cargo }], hidden: 0, recovered: [], backups: [] });
const ITEMS = [{ id: 'DolomiteRock', name: 'Carbon Ore', category: 'resources' }];
const FUEL = { total: 1, notFull: 0, unknown: 0, noFuelRecord: 0 };
const NO_HOLD = { inventory_id: 70, slots: 0, volume: 0, used: 0, noHold: true, items: [] };
const SURPLUS = { ...NO_HOLD, used: 10, items: Array.from({ length: 10 }, (_, i) => item(i, i)) };
const BUGGY = { inventory_id: 81, slots: 35, volume: 3500, used: 10, noHold: false, items: Array.from({ length: 10 }, (_, i) => item(i, i)) };

async function open(vehicles, post = {}) {
  const ui = await boot({ get: { '/api/vehicles': vehicles, '/api/vehicles/fuel': FUEL, '/api/catalog/items': ITEMS }, post });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  return ui;
}
const cardText = (ui) => ui.doc.querySelector('#cargoroom').closest('.card').textContent.replace(/\s+/g, ' ');

test('a Sandbike with no storage part says so and has no Add item row or Clear button', async () => {
  const ui = await open(veh('Sandbike', NO_HOLD));
  assert.equal(ui.doc.querySelector('#cargoroom').textContent, 'This vehicle has no cargo hold (no storage part).');
  assert.equal(ui.doc.querySelector('[data-act="giveCargo"]'), null, 'no Add item');
  assert.equal(ui.doc.querySelector('#bq'), null);
  assert.equal(ui.doc.querySelector('[data-act="clearInv"]'), null, 'nothing to clear');
  assert.ok(!cardText(ui).includes('of 0'), cardText(ui));
  ui.dom.window.close();
});

test('the 10 stacks already in a hold with no slots are listed with N items in a hold with no slots, and Delete and Clear this container stay', async () => {
  const ui = await open(veh('Sandbike', SURPLUS));
  const t = cardText(ui);
  assert.ok(t.includes('This vehicle has no cargo hold (no storage part).') && t.includes('10 items in a hold with no slots.'), t);
  assert.ok(!t.includes('of 0'), t);
  assert.equal(ui.doc.querySelector('[data-act="giveCargo"]'), null, 'no Add item');
  assert.equal(ui.doc.querySelectorAll('.card [data-act="delItem"]').length, 10, 'a Delete for each surplus row');
  const clear = ui.doc.querySelector('[data-act="clearInv"]');
  assert.ok(clear && !clear.disabled && clear.dataset.inv === '70' && clear.dataset.all === '10');
  ui.dom.window.close();
});

test('a vehicle with a hold reads N of M slots used and offers Add item for its inventory', async () => {
  const ui = await open(veh('Buggy', BUGGY), { '/api/bases/give': { ok: true, requested: 5000, given: 5000, reason: '', limit: '', volumeChecked: true, volumeNote: '' } });
  assert.equal(ui.doc.querySelector('#cargoroom').textContent, '10 of 35 slots used');
  assert.ok(!cardText(ui).includes('of 0'));
  assert.equal(ui.doc.querySelector('[data-act="giveCargo"]').dataset.id, '81');
  ui.dom.window.close();
});

test('a give that fits only in part says how much, and a refusal from the server is shown as the error it is', async () => {
  let ui = await open(veh('Buggy', BUGGY), { '/api/bases/give': { ok: true, requested: 20000, given: 12500, reason: 'the container is full (volume)', limit: 'volume', volumeChecked: true, volumeNote: '' } });
  ui.doc.querySelector('#bt').value = 'Carbon Ore';
  ui.doc.querySelector('#bq').value = '20000';
  ui.click('[data-act="giveCargo"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/bases/give').map((p) => p.body), [{ inventory_id: 81, template_id: 'DolomiteRock', quantity: 20000 }]);
  const t = ui.doc.querySelector('#toast .part');
  assert.ok(t && t.textContent.includes('Gave 12,500 of 20,000 Carbon Ore: the container is full (volume).'), t && t.textContent);
  ui.dom.window.close();

  // a page that was drawn before the parts changed: the server still refuses
  ui = await open(veh('Buggy', BUGGY), { '/api/bases/give': { __fail: 'This vehicle has no cargo hold (no storage part).', __code: 'no_room', __status: 409 } });
  ui.doc.querySelector('#bt').value = 'Carbon Ore';
  ui.click('[data-act="giveCargo"]'); await ui.settle();
  const err = ui.doc.querySelector('#toast .err');
  assert.ok(err && err.textContent.includes('This vehicle has no cargo hold (no storage part).'), err && err.textContent);
  ui.dom.window.close();
});

test('a hold with no recorded limit shows the item count, as before', async () => {
  const ui = await open(veh('Sandbike', { inventory_id: 70, slots: null, volume: null, used: 1, noHold: false, items: [item(1, 0)] }));
  assert.equal(ui.doc.querySelector('#cargoroom').textContent, '1 item');
  ui.dom.window.close();
});

test('the cards stay inside the words-to-read budget of the text sweep', async () => {
  for (const cargo of [NO_HOLD, SURPLUS, BUGGY]) {
    const ui = await open(veh('Sandbike', cargo));
    const main = ui.doc.querySelector('#main');
    assert.ok(proseWords(main) <= 100, `${proseWords(main)} words`);
    assert.ok(longestBlock(main) <= 45, `${longestBlock(main)} words in a block`);
    ui.dom.window.close();
  }
});
